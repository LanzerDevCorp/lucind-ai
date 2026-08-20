package ledger

import (
	"context"
	"database/sql"
	"time"
)

// schemaVersion is the migration version this schema represents.
const schemaVersion = 4

const schemaMigrationsDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL) STRICT;
`

const schemaDDL = `
CREATE TABLE IF NOT EXISTS lanes (
  run_id             TEXT    NOT NULL,
  lane_id            TEXT    NOT NULL,
  packet_id          TEXT    NOT NULL,
  executor           TEXT    NOT NULL CHECK (executor IN ('agy','cursor-agent','human')),
  routing_condition  TEXT    NOT NULL CHECK (length(trim(routing_condition)) > 0),
  status             TEXT    NOT NULL CHECK (status IN
                       ('pending','running','done','blocked','deviated','failed')),
  worktree_path      TEXT    NOT NULL DEFAULT '',
  worktree_preserved INTEGER NOT NULL DEFAULT 0 CHECK (worktree_preserved IN (0,1)),
  attempt            INTEGER NOT NULL DEFAULT 1 CHECK (attempt >= 1),
  started_at         TEXT,
  ended_at           TEXT,
  PRIMARY KEY (run_id, lane_id)
) STRICT;

CREATE TABLE IF NOT EXISTS events (
  id      INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id  TEXT NOT NULL,
  lane_id TEXT,
  type    TEXT NOT NULL CHECK (type IN ('run_started','lane_registered',
            'lane_status_changed','lane_note','barrier_released','run_ended')),
  detail  TEXT NOT NULL DEFAULT '',
  at      TEXT NOT NULL
) STRICT;
CREATE INDEX IF NOT EXISTS idx_events_run ON events(run_id, id);

CREATE TABLE IF NOT EXISTS approvals (
  run_id                TEXT    NOT NULL,
  lane_id               TEXT    NOT NULL,
  packet_id             TEXT    NOT NULL,
  approver              TEXT    NOT NULL DEFAULT '',
  decision              TEXT    NOT NULL CHECK (decision IN ('pending','approved','rejected','timed_out')),
  evidence              TEXT    NOT NULL DEFAULT '',
  defect_surfaced_later INTEGER NOT NULL DEFAULT 0 CHECK (defect_surfaced_later IN (0,1)),
  requested_at          TEXT    NOT NULL,
  decided_at            TEXT,
  PRIMARY KEY (run_id, lane_id)
) STRICT;
`

const migrateV1ToV2DDL = `
CREATE TABLE events_new (
  id      INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id  TEXT NOT NULL,
  lane_id TEXT,
  type    TEXT NOT NULL CHECK (type IN ('run_started','lane_registered',
            'lane_status_changed','lane_note','barrier_released','run_ended')),
  detail  TEXT NOT NULL DEFAULT '',
  at      TEXT NOT NULL
) STRICT;

INSERT INTO events_new (id, run_id, lane_id, type, detail, at)
SELECT id, run_id, lane_id, type, detail, at FROM events ORDER BY id;

DROP TABLE events;

ALTER TABLE events_new RENAME TO events;

CREATE INDEX IF NOT EXISTS idx_events_run ON events(run_id, id);
`

const migrateV2ToV3DDL = `
CREATE TABLE IF NOT EXISTS approvals (
  run_id                TEXT    NOT NULL,
  lane_id               TEXT    NOT NULL,
  packet_id             TEXT    NOT NULL,
  approver              TEXT    NOT NULL DEFAULT '',
  decision              TEXT    NOT NULL CHECK (decision IN ('pending','approved','rejected','timed_out')),
  evidence              TEXT    NOT NULL DEFAULT '',
  defect_surfaced_later INTEGER NOT NULL DEFAULT 0 CHECK (defect_surfaced_later IN (0,1)),
  requested_at          TEXT    NOT NULL,
  decided_at            TEXT,
  PRIMARY KEY (run_id, lane_id)
) STRICT;
`

const migrateV3ToV4DDL = `
CREATE TABLE IF NOT EXISTS features (
  id                  TEXT PRIMARY KEY,
  parent_ref          TEXT NOT NULL,
  base_sha            TEXT NOT NULL,
  expected_parent_sha TEXT NOT NULL DEFAULT '',
  status              TEXT NOT NULL CHECK (status IN ('created','active','disabled')),
  created_at          TEXT NOT NULL,
  updated_at          TEXT NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS integration_attempts (
  id              TEXT PRIMARY KEY,
  feature_id      TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  status          TEXT NOT NULL CHECK (status IN
                    ('recorded','leased','combining','checking','cas_pending',
                     'promoted','blocked','failed','stale')),
  owner           TEXT NOT NULL DEFAULT '',
  fence           INTEGER NOT NULL DEFAULT 0,
  candidate_sha   TEXT NOT NULL DEFAULT '',
  failure_reason  TEXT NOT NULL DEFAULT '',
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL,
  UNIQUE (feature_id, idempotency_key)
) STRICT;

CREATE TABLE IF NOT EXISTS feature_leases (
  feature_id   TEXT PRIMARY KEY,
  owner        TEXT NOT NULL,
  fence        INTEGER NOT NULL DEFAULT 0 CHECK (fence >= 0),
  expires_at   TEXT NOT NULL,
  acquired_at  TEXT NOT NULL,
  updated_at   TEXT NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS overlap_evidence (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  feature_id     TEXT NOT NULL,
  version        TEXT NOT NULL,
  evidence_hash  TEXT NOT NULL,
  evidence_class TEXT NOT NULL CHECK (evidence_class IN ('required','warning','informational')),
  evidence_json  TEXT NOT NULL,
  created_at     TEXT NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS reconciliation_requests (
  id               TEXT PRIMARY KEY,
  feature_id       TEXT NOT NULL,
  direction        TEXT NOT NULL,
  status           TEXT NOT NULL CHECK (status IN ('awaiting','approved','declined','cancelled','expired')),
  actor            TEXT NOT NULL DEFAULT '',
  evidence_version TEXT NOT NULL DEFAULT '',
  evidence_hash    TEXT NOT NULL DEFAULT '',
  source_sha       TEXT NOT NULL DEFAULT '',
  target_sha       TEXT NOT NULL DEFAULT '',
  expires_at       TEXT,
  created_at       TEXT NOT NULL,
  updated_at       TEXT NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS reconciliation_candidates (
  id             TEXT PRIMARY KEY,
  request_id     TEXT NOT NULL,
  status         TEXT NOT NULL CHECK (status IN ('candidate_running','integrated','failed','stale')),
  allowed_paths  TEXT NOT NULL DEFAULT '',
  model          TEXT NOT NULL DEFAULT '',
  config         TEXT NOT NULL DEFAULT '',
  output         TEXT NOT NULL DEFAULT '',
  checks         TEXT NOT NULL DEFAULT '',
  candidate_sha  TEXT NOT NULL DEFAULT '',
  failure_reason TEXT NOT NULL DEFAULT '',
  created_at     TEXT NOT NULL,
  updated_at     TEXT NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS integration_events (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  feature_id TEXT NOT NULL,
  attempt_id TEXT,
  type       TEXT NOT NULL,
  detail     TEXT NOT NULL DEFAULT '',
  at         TEXT NOT NULL
) STRICT;
CREATE INDEX IF NOT EXISTS idx_integration_events_feature ON integration_events(feature_id, id);
`

// migrate applies the schema inside one transaction and records the
// migration version. It is idempotent: re-running it against an already
// migrated database (e.g. a second Open on the same file) is a safe no-op.
func migrate(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op once Commit succeeds

	if _, err := tx.ExecContext(ctx, schemaMigrationsDDL); err != nil {
		return err
	}

	var currentVersion int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&currentVersion); err != nil {
		return err
	}

	if currentVersion < 1 {
		if _, err := tx.ExecContext(ctx, schemaDDL); err != nil {
			return err
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?), (?, ?), (?, ?)`,
			1, now, 2, now, 3, now,
		); err != nil {
			return err
		}
		currentVersion = 3
	}

	if currentVersion < 2 {
		if _, err := tx.ExecContext(ctx, migrateV1ToV2DDL); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			2, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			return err
		}
		currentVersion = 2
	}

	if currentVersion < 3 {
		if _, err := tx.ExecContext(ctx, migrateV2ToV3DDL); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			3, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			return err
		}
		currentVersion = 3
	}

	if currentVersion < 4 {
		if _, err := tx.ExecContext(ctx, migrateV3ToV4DDL); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			4, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			return err
		}
		currentVersion = 4
	}

	return tx.Commit()
}
