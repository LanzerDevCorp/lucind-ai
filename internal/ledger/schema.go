package ledger

import (
	"context"
	"database/sql"
	"time"
)

// schemaVersion is the migration version this schema represents.
const schemaVersion = 3

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

	return tx.Commit()
}
