package ledger

import (
	"context"
	"database/sql"
	"time"
)

// schemaVersion is the only migration this slice defines. schema_migrations
// exists so a future slice can add columns without guesswork.
const schemaVersion = 1

const schemaDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL) STRICT;

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
            'lane_status_changed','barrier_released','run_ended')),
  detail  TEXT NOT NULL DEFAULT '',
  at      TEXT NOT NULL
) STRICT;
CREATE INDEX IF NOT EXISTS idx_events_run ON events(run_id, id);
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

	if _, err := tx.ExecContext(ctx, schemaDDL); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
		schemaVersion, time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		return err
	}

	return tx.Commit()
}
