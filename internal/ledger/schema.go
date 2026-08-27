package ledger

import (
	"context"
	"database/sql"
	"time"
)

// schemaVersion is the migration version this schema represents.
const schemaVersion = 9

const schemaMigrationsDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL) STRICT;
`

const schemaDDL = `
CREATE TABLE IF NOT EXISTS lanes (
  run_id             TEXT    NOT NULL,
  lane_id            TEXT    NOT NULL,
  packet_id          TEXT    NOT NULL,
  executor           TEXT    NOT NULL CHECK (executor IN ('agy','cursor-agent','human','opencode')),
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

// migrateV4ToV5DDL adds "opencode" to the lanes.executor CHECK constraint.
// SQLite cannot ALTER a CHECK constraint in place, and lanes is a STRICT
// table, so this follows the same create-copy-drop-rename shape as
// migrateV1ToV2DDL's events table rebuild: create lanes_new with the wider
// CHECK, copy every row and column verbatim (preserving PRIMARY KEY
// identity), drop the old table, rename the new one into place. No other
// column changes; this migration exists solely to admit the third
// executor -- see cmd/lucind-ai/cli.go's supportedExecutors.
const migrateV4ToV5DDL = `
CREATE TABLE lanes_new (
  run_id             TEXT    NOT NULL,
  lane_id            TEXT    NOT NULL,
  packet_id          TEXT    NOT NULL,
  executor           TEXT    NOT NULL CHECK (executor IN ('agy','cursor-agent','human','opencode')),
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

INSERT INTO lanes_new (
  run_id, lane_id, packet_id, executor, routing_condition, status,
  worktree_path, worktree_preserved, attempt, started_at, ended_at
)
SELECT
  run_id, lane_id, packet_id, executor, routing_condition, status,
  worktree_path, worktree_preserved, attempt, started_at, ended_at
FROM lanes ORDER BY run_id, lane_id;

DROP TABLE lanes;

ALTER TABLE lanes_new RENAME TO lanes;
`

// migrateV5ToV6DDL adds the durable run and progress stores, lane dispatch
// metadata, and the run_status_changed event type. The lanes and events
// tables use the established create-copy-drop-rename pattern because SQLite
// cannot widen a STRICT table's shape or CHECK constraint in place.
const migrateV5ToV6DDL = `
CREATE TABLE runs (
  run_id     TEXT    PRIMARY KEY,
  feature_id TEXT    NOT NULL,
  status     TEXT    NOT NULL,
  target_ref TEXT    NOT NULL,
  lane_count INTEGER NOT NULL CHECK (lane_count >= 0),
  started_at TEXT    NOT NULL,
  ended_at   TEXT
) STRICT;

CREATE TABLE lanes_new (
  run_id             TEXT    NOT NULL,
  lane_id            TEXT    NOT NULL,
  packet_id          TEXT    NOT NULL,
  executor           TEXT    NOT NULL CHECK (executor IN ('agy','cursor-agent','human','opencode')),
  routing_condition  TEXT    NOT NULL CHECK (length(trim(routing_condition)) > 0),
  status             TEXT    NOT NULL CHECK (status IN
                       ('pending','running','done','blocked','deviated','failed')),
  worktree_path      TEXT    NOT NULL DEFAULT '',
  worktree_preserved INTEGER NOT NULL DEFAULT 0 CHECK (worktree_preserved IN (0,1)),
  attempt            INTEGER NOT NULL DEFAULT 1 CHECK (attempt >= 1),
  started_at         TEXT,
  ended_at           TEXT,
  model              TEXT,
  agent              TEXT,
  feature            TEXT,
  PRIMARY KEY (run_id, lane_id)
) STRICT;

INSERT INTO lanes_new (
  run_id, lane_id, packet_id, executor, routing_condition, status,
  worktree_path, worktree_preserved, attempt, started_at, ended_at
)
SELECT
  run_id, lane_id, packet_id, executor, routing_condition, status,
  worktree_path, worktree_preserved, attempt, started_at, ended_at
FROM lanes ORDER BY run_id, lane_id;

DROP TABLE lanes;

ALTER TABLE lanes_new RENAME TO lanes;

CREATE TABLE IF NOT EXISTS events (
  id      INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id  TEXT NOT NULL,
  lane_id TEXT,
  type    TEXT NOT NULL CHECK (type IN ('run_started','lane_registered',
            'lane_status_changed','lane_note','barrier_released','run_ended')),
  detail  TEXT NOT NULL DEFAULT '',
  at      TEXT NOT NULL
) STRICT;

CREATE TABLE events_new (
  id      INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id  TEXT NOT NULL,
  lane_id TEXT,
  type    TEXT NOT NULL CHECK (type IN ('run_started','lane_registered',
            'lane_status_changed','lane_note','barrier_released','run_ended',
            'run_status_changed')),
  detail  TEXT NOT NULL DEFAULT '',
  at      TEXT NOT NULL
) STRICT;

INSERT INTO events_new (id, run_id, lane_id, type, detail, at)
SELECT id, run_id, lane_id, type, detail, at FROM events ORDER BY id;

DROP TABLE events;

ALTER TABLE events_new RENAME TO events;

CREATE INDEX IF NOT EXISTS idx_events_run ON events(run_id, id);

CREATE TABLE lane_progress (
  run_id  TEXT    NOT NULL,
  lane_id TEXT    NOT NULL,
  seq     INTEGER NOT NULL,
  message TEXT    NOT NULL,
  at      TEXT    NOT NULL,
  PRIMARY KEY (run_id, lane_id, seq)
) STRICT;
CREATE INDEX IF NOT EXISTS idx_lane_progress_run_lane_seq
  ON lane_progress(run_id, lane_id, seq);
`

const migrateV6ToV7DDL = `
CREATE TABLE runs_new (
  run_id     TEXT    PRIMARY KEY,
  feature_id TEXT    NOT NULL,
  status     TEXT    NOT NULL,
  target_ref TEXT    NOT NULL,
  lane_count INTEGER NOT NULL CHECK (lane_count >= 0),
  started_at TEXT    NOT NULL,
  ended_at   TEXT,
  pid        INTEGER NOT NULL DEFAULT 0 CHECK (pid >= 0)
) STRICT;

INSERT INTO runs_new (
  run_id, feature_id, status, target_ref, lane_count, started_at, ended_at
)
SELECT
  run_id, feature_id, status, target_ref, lane_count, started_at, ended_at
FROM runs ORDER BY started_at, run_id;

DROP TABLE runs;

ALTER TABLE runs_new RENAME TO runs;

CREATE TABLE lane_progress_new (
  run_id       TEXT    NOT NULL,
  lane_id      TEXT    NOT NULL,
  seq          INTEGER NOT NULL,
  message      TEXT    NOT NULL,
  at           TEXT    NOT NULL,
  total_tokens INTEGER NOT NULL DEFAULT 0 CHECK (total_tokens >= 0),
  cost_usd     REAL    NOT NULL DEFAULT 0.0 CHECK (cost_usd >= 0.0),
  tool_calls   INTEGER NOT NULL DEFAULT 0 CHECK (tool_calls >= 0),
  PRIMARY KEY (run_id, lane_id, seq)
) STRICT;

INSERT INTO lane_progress_new (
  run_id, lane_id, seq, message, at
)
SELECT
  run_id, lane_id, seq, message, at
FROM lane_progress ORDER BY run_id, lane_id, seq;

DROP TABLE lane_progress;

ALTER TABLE lane_progress_new RENAME TO lane_progress;

CREATE INDEX IF NOT EXISTS idx_lane_progress_run_lane_seq
  ON lane_progress(run_id, lane_id, seq);
`

const migrateV7ToV8DDL = `
CREATE TABLE IF NOT EXISTS defect_records (
  id              TEXT PRIMARY KEY,
  feature_id      TEXT NOT NULL,
  run_id          TEXT NOT NULL DEFAULT '',
  lane_id         TEXT NOT NULL DEFAULT '',
  error_signature TEXT NOT NULL,
  evidence        TEXT NOT NULL DEFAULT '',
  disposition     TEXT NOT NULL CHECK (disposition IN ('recorded','repaired','declined','deferred')),
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL
) STRICT;
CREATE INDEX IF NOT EXISTS idx_defect_records_feature ON defect_records(feature_id, id);
`

const migrateV8ToV9DDL = `
CREATE TABLE lane_candidates (
  run_id          TEXT NOT NULL,
  lane_id         TEXT NOT NULL,
  packet_id       TEXT NOT NULL,
  packet_digest   TEXT NOT NULL,
  primary_root    TEXT NOT NULL,
  worktree_path   TEXT NOT NULL,
  base_commit     TEXT NOT NULL,
  base_tree       TEXT NOT NULL,
  candidate_commit TEXT NOT NULL,
  candidate_tree  TEXT NOT NULL,
  allowed_paths   TEXT NOT NULL,
  result_path     TEXT NOT NULL,
  result_json     TEXT NOT NULL,
  result_hash     TEXT NOT NULL,
  recorded_at     TEXT NOT NULL,
  PRIMARY KEY (run_id, lane_id)
) STRICT;

CREATE TABLE acceptance_receipts (
  receipt_id       TEXT PRIMARY KEY,
  binding_hash     TEXT NOT NULL UNIQUE,
  run_id           TEXT NOT NULL,
  lane_id          TEXT NOT NULL,
  packet_id        TEXT NOT NULL,
  packet_digest    TEXT NOT NULL,
  base_commit      TEXT NOT NULL,
  base_tree        TEXT NOT NULL,
  candidate_commit TEXT NOT NULL,
  candidate_tree   TEXT NOT NULL,
  allowed_paths_hash TEXT NOT NULL,
  check_policy_hash TEXT NOT NULL,
  environment_hash TEXT NOT NULL,
  result_hash      TEXT NOT NULL,
  checks_hash      TEXT NOT NULL,
  cleanup          TEXT NOT NULL,
  created_at       TEXT NOT NULL
) STRICT;

CREATE TRIGGER lane_candidates_no_update BEFORE UPDATE ON lane_candidates
BEGIN SELECT RAISE(ABORT, 'lane_candidates are immutable'); END;
CREATE TRIGGER lane_candidates_no_delete BEFORE DELETE ON lane_candidates
BEGIN SELECT RAISE(ABORT, 'lane_candidates are immutable'); END;
CREATE TRIGGER acceptance_receipts_no_update BEFORE UPDATE ON acceptance_receipts
BEGIN SELECT RAISE(ABORT, 'acceptance_receipts are immutable'); END;
CREATE TRIGGER acceptance_receipts_no_delete BEFORE DELETE ON acceptance_receipts
BEGIN SELECT RAISE(ABORT, 'acceptance_receipts are immutable'); END;
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

	if currentVersion < 5 {
		if _, err := tx.ExecContext(ctx, migrateV4ToV5DDL); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			5, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			return err
		}
		currentVersion = 5
	}

	if currentVersion < 6 {
		if _, err := tx.ExecContext(ctx, migrateV5ToV6DDL); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			6, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			return err
		}
		currentVersion = 6
	}

	if currentVersion < 7 {
		if _, err := tx.ExecContext(ctx, migrateV6ToV7DDL); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			7, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			return err
		}
		currentVersion = 7
	}

	if currentVersion < 8 {
		if _, err := tx.ExecContext(ctx, migrateV7ToV8DDL); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			8, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			return err
		}
		currentVersion = 8
	}

	if currentVersion < 9 {
		if _, err := tx.ExecContext(ctx, migrateV8ToV9DDL); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			9, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			return err
		}
		currentVersion = 9
	}

	return tx.Commit()
}
