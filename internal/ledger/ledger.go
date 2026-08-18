// Package ledger is the SQLite-backed lane ledger: a durable,
// concurrency-safe record of lane state and the routing decision behind it,
// owned solely by the primary repository. It imports internal/lane for the
// shared status vocabulary, internal/ledgerpath to derive the single ledger
// location, and modernc.org/sqlite as its pure-Go driver; it never imports
// internal/barrier.
//
// Open takes a primary repository root, never a database path, so the
// "Single ledger location" requirement's first scenario — the database
// exists under "<primary-repo>/.lucind/" — is closed by the API shape
// itself: there is no parameter through which a caller could open the
// ledger anywhere else. The requirement's second scenario — no lane-state
// database is ever created inside a lane's worktree — needs distinguishing
// a worktree from a repository root, which needs git awareness this slice
// does not have. That remains explicitly deferred to the dispatch slice
// (see internal/ledgerpath's package doc); Open does not attempt to detect
// or reject a worktree path passed as primaryRoot.
package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"modernc.org/sqlite" // registers the "sqlite" database/sql driver

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledgerpath"
)

// Sentinel errors returned by Ledger methods.
var (
	// ErrRoutingConditionMissing is returned when a lane insert carries a
	// NULL or empty/whitespace-only routing_condition. SQLite reports this
	// as two distinct constraint failures (NOT NULL for a NULL value,
	// CHECK for empty/whitespace); both map to this one sentinel.
	ErrRoutingConditionMissing = errors.New("ledger: routing condition is required")

	// ErrLaneUnknown is returned when an operation targets a (run_id,
	// lane_id) pair that has no row in the lanes table.
	ErrLaneUnknown = errors.New("ledger: lane not found")

	// ErrPragmaNotApplied is returned by Open when a required pragma
	// (journal_mode=wal or busy_timeout=5000) did not take effect on the
	// opened connection. A post-open PRAGMA Exec only reaches one
	// arbitrary pooled connection, so pragmas are set via the DSN and
	// then read back to convert a driver-specific DSN-syntax assumption
	// into a loud, first-cycle failure instead of a silent SQLITE_BUSY
	// later.
	ErrPragmaNotApplied = errors.New("ledger: pragma did not take effect")
)

const (
	wantJournalMode = "wal"
	wantBusyTimeout = 5000
)

// Ledger is a handle to the lane ledger database.
type Ledger struct {
	db *sql.DB
}

// Open opens (creating if necessary) the SQLite ledger database under
// primaryRoot's .lucind directory. It derives the exact database path via
// ledgerpath.Resolve(primaryRoot) — it does not accept a database path from
// the caller, so pointing the ledger at a location outside
// "<primaryRoot>/.lucind/" is not possible through this API. Open applies
// pragmas via the connection DSN so every pooled connection gets them,
// asserts the pragmas actually took effect, sizes the connection pool for
// real concurrency (never 1 — a pool of 1 would make the concurrency
// guarantee tautological), and migrates the schema. Open is idempotent: it
// is safe to call again with the same primaryRoot.
func Open(ctx context.Context, primaryRoot string) (*Ledger, error) {
	return openAtPath(ctx, ledgerpath.Resolve(primaryRoot))
}

// openAtPath does the actual work of opening a SQLite database at an
// explicit path. It is unexported: the public Open never exposes a
// caller-supplied path, only openAtPath (and this package's own tests, for
// exercising failure paths such as ":memory:" that ledgerpath.Resolve could
// never produce) can reach an arbitrary path.
func openAtPath(ctx context.Context, dbPath string) (*Ledger, error) {
	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("ledger: create parent directory: %w", err)
		}
	}

	dsn := "file:" + dbPath +
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("ledger: open: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ledger: ping: %w", err)
	}

	if err := checkPragmas(ctx, db); err != nil {
		db.Close()
		return nil, err
	}

	// >1 so the concurrency guarantee this package exists to provide is
	// actually exercised, not made true by construction.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(0)

	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("ledger: migrate: %w", err)
	}

	return &Ledger{db: db}, nil
}

// checkPragmas reads back the pragmas Open requested via the DSN and fails
// loudly if either did not take effect on this connection.
func checkPragmas(ctx context.Context, db *sql.DB) error {
	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		return fmt.Errorf("ledger: read journal_mode pragma: %w", err)
	}
	if journalMode != wantJournalMode {
		return fmt.Errorf("%w: journal_mode=%s, want %s", ErrPragmaNotApplied, journalMode, wantJournalMode)
	}

	var busyTimeout int
	if err := db.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		return fmt.Errorf("ledger: read busy_timeout pragma: %w", err)
	}
	if busyTimeout != wantBusyTimeout {
		return fmt.Errorf("%w: busy_timeout=%d, want %d", ErrPragmaNotApplied, busyTimeout, wantBusyTimeout)
	}

	return nil
}

// Close releases the underlying database connection pool.
func (l *Ledger) Close() error {
	return l.db.Close()
}

// SQLite structured extended error codes. SQLite reports a missing
// routing_condition two different ways depending on how it is missing: a
// NULL value fails NOT NULL (1299); an empty or whitespace-only value fails
// the CHECK (275). Both are mapped to ErrRoutingConditionMissing below,
// using the driver's structured code — never a message substring.
const (
	sqliteConstraintNotNull = 1299 // SQLITE_CONSTRAINT_NOTNULL
	sqliteConstraintCheck   = 275  // SQLITE_CONSTRAINT_CHECK
)

// Lane is one row of the lanes table: identity, routing decision, current
// status, worktree bookkeeping, and timestamps for one lane of one run.
type Lane struct {
	RunID             string
	LaneID            string
	PacketID          string
	Executor          string
	RoutingCondition  string
	Status            lane.Status
	WorktreePath      string
	WorktreePreserved bool
	Attempt           int
	StartedAt         *time.Time
	EndedAt           *time.Time
}

// RegisterLane inserts a new lane row. Status is validated in Go before any
// write; the routing_condition constraint is enforced by the schema and its
// two possible SQLite failure shapes are both mapped to
// ErrRoutingConditionMissing. A zero Attempt defaults to 1, matching the
// schema's own default.
func (l *Ledger) RegisterLane(ctx context.Context, ln Lane) error {
	if !ln.Status.Valid() {
		return fmt.Errorf("ledger: invalid lane status %q", ln.Status)
	}

	attempt := ln.Attempt
	if attempt == 0 {
		attempt = 1
	}

	_, err := l.db.ExecContext(ctx, `
		INSERT INTO lanes (
			run_id, lane_id, packet_id, executor, routing_condition, status,
			worktree_path, worktree_preserved, attempt
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ln.RunID, ln.LaneID, ln.PacketID, ln.Executor, ln.RoutingCondition, string(ln.Status),
		ln.WorktreePath, boolToInt(ln.WorktreePreserved), attempt,
	)
	if err != nil {
		return mapLaneConstraintError(err)
	}

	return nil
}

// Lanes returns every lane row for the given run, ordered by lane_id.
func (l *Ledger) Lanes(ctx context.Context, runID string) ([]Lane, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT run_id, lane_id, packet_id, executor, routing_condition, status,
		       worktree_path, worktree_preserved, attempt, started_at, ended_at
		FROM lanes WHERE run_id = ? ORDER BY lane_id`, runID)
	if err != nil {
		return nil, fmt.Errorf("ledger: query lanes: %w", err)
	}
	defer rows.Close()

	var out []Lane
	for rows.Next() {
		var (
			ln                 Lane
			status             string
			worktreePreserved  int
			startedAt, endedAt sql.NullString
		)
		if err := rows.Scan(
			&ln.RunID, &ln.LaneID, &ln.PacketID, &ln.Executor, &ln.RoutingCondition, &status,
			&ln.WorktreePath, &worktreePreserved, &ln.Attempt, &startedAt, &endedAt,
		); err != nil {
			return nil, fmt.Errorf("ledger: scan lane row: %w", err)
		}

		ln.Status = lane.Status(status)
		ln.WorktreePreserved = worktreePreserved != 0
		if t, err := parseNullableTimestamp(startedAt); err != nil {
			return nil, err
		} else {
			ln.StartedAt = t
		}
		if t, err := parseNullableTimestamp(endedAt); err != nil {
			return nil, err
		} else {
			ln.EndedAt = t
		}

		out = append(out, ln)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ledger: iterate lane rows: %w", err)
	}

	return out, nil
}

// LaneStates returns the current lane.State for every lane of the given
// run. Its output is the ONLY type that crosses the ledger -> barrier
// boundary; barrier.Evaluate never imports this package, so the seam is
// proven by feeding this output to barrier.Evaluate directly (see
// TestLaneStatesFeedsBarrierEvaluate).
func (l *Ledger) LaneStates(ctx context.Context, runID string) ([]lane.State, error) {
	rows, err := l.db.QueryContext(ctx,
		`SELECT lane_id, status FROM lanes WHERE run_id = ? ORDER BY lane_id`, runID)
	if err != nil {
		return nil, fmt.Errorf("ledger: query lane states: %w", err)
	}
	defer rows.Close()

	var out []lane.State
	for rows.Next() {
		var id, status string
		if err := rows.Scan(&id, &status); err != nil {
			return nil, fmt.Errorf("ledger: scan lane state: %w", err)
		}
		out = append(out, lane.State{LaneID: id, Status: lane.Status(status)})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ledger: iterate lane states: %w", err)
	}

	return out, nil
}

// AppendEvent records a standalone event row. Callers use it for the
// events this package's own methods do not already emit as a side effect
// (SetStatus emits lane_status_changed itself, in the same transaction as
// its UPDATE) — e.g. run_started, run_ended, lane_registered,
// barrier_released. An empty e.LaneID is stored as SQL NULL, marking a
// run-scoped event.
func (l *Ledger) AppendEvent(ctx context.Context, e Event) error {
	var laneID sql.NullString
	if e.LaneID != "" {
		laneID = sql.NullString{String: e.LaneID, Valid: true}
	}

	_, err := l.db.ExecContext(ctx,
		`INSERT INTO events (run_id, lane_id, type, detail, at) VALUES (?, ?, ?, ?, ?)`,
		e.RunID, laneID, e.Type, e.Detail, e.At.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("ledger: append event: %w", err)
	}

	return nil
}

// SetWorktreePreserved updates whether a lane's worktree was preserved
// (e.g. because the lane's terminal status was not done). It fails with
// ErrLaneUnknown if no row exists for (runID, laneID).
func (l *Ledger) SetWorktreePreserved(ctx context.Context, runID, laneID string, preserved bool) error {
	res, err := l.db.ExecContext(ctx,
		`UPDATE lanes SET worktree_preserved = ? WHERE run_id = ? AND lane_id = ?`,
		boolToInt(preserved), runID, laneID,
	)
	if err != nil {
		return fmt.Errorf("ledger: update worktree_preserved: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("ledger: read rows affected: %w", err)
	}
	if affected == 0 {
		return ErrLaneUnknown
	}

	return nil
}

// mapLaneConstraintError converts a SQLite constraint failure on a lanes
// insert into ErrRoutingConditionMissing when the failure's structured code
// matches a NOT NULL or CHECK violation. RegisterLane pre-validates status
// in Go, so within this package's accepted field set the only constraint a
// well-formed call can still trip is routing_condition.
func mapLaneConstraintError(err error) error {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		switch sqliteErr.Code() {
		case sqliteConstraintNotNull, sqliteConstraintCheck:
			return ErrRoutingConditionMissing
		}
	}
	return fmt.Errorf("ledger: register lane: %w", err)
}

// Event is one row of the events table: an append-only record of something
// that happened during a run. LaneID is empty for run-scoped events (stored
// as SQL NULL), matching the schema comment "NULL for run-scoped events".
type Event struct {
	ID     int64
	RunID  string
	LaneID string
	Type   string
	Detail string
	At     time.Time
}

// Event types this slice actually produces. No column or type exists here
// for a producer that doesn't exist yet (worker removal, call duration) —
// see design decision "Event types are exactly the five this slice can
// emit".
const (
	EventRunStarted        = "run_started"
	EventLaneRegistered    = "lane_registered"
	EventLaneStatusChanged = "lane_status_changed"
	EventBarrierReleased   = "barrier_released"
	EventRunEnded          = "run_ended"
)

// SetStatus updates a lane's status and appends the corresponding
// lane_status_changed event in the same transaction — a status change
// without its event is not a representable state. It fails with
// ErrLaneUnknown if no row exists for (runID, laneID).
func (l *Ledger) SetStatus(ctx context.Context, runID, laneID string, s lane.Status, at time.Time) error {
	if !s.Valid() {
		return fmt.Errorf("ledger: invalid lane status %q", s)
	}

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ledger: begin set-status tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once Commit succeeds

	res, err := tx.ExecContext(ctx,
		`UPDATE lanes SET status = ? WHERE run_id = ? AND lane_id = ?`,
		string(s), runID, laneID,
	)
	if err != nil {
		return fmt.Errorf("ledger: update lane status: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("ledger: read rows affected: %w", err)
	}
	if affected == 0 {
		return ErrLaneUnknown
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO events (run_id, lane_id, type, detail, at) VALUES (?, ?, ?, ?, ?)`,
		runID, laneID, EventLaneStatusChanged, string(s), at.UTC().Format(time.RFC3339),
	); err != nil {
		return fmt.Errorf("ledger: insert status-change event: %w", err)
	}

	return tx.Commit()
}

// Events returns every event row for the given run, ordered by insertion
// (id) so that sequential transitions read back in the order they happened.
func (l *Ledger) Events(ctx context.Context, runID string) ([]Event, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT id, run_id, lane_id, type, detail, at
		FROM events WHERE run_id = ? ORDER BY id`, runID)
	if err != nil {
		return nil, fmt.Errorf("ledger: query events: %w", err)
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var (
			e      Event
			laneID sql.NullString
			at     string
		)
		if err := rows.Scan(&e.ID, &e.RunID, &laneID, &e.Type, &e.Detail, &at); err != nil {
			return nil, fmt.Errorf("ledger: scan event row: %w", err)
		}
		if laneID.Valid {
			e.LaneID = laneID.String
		}
		parsed, err := time.Parse(time.RFC3339, at)
		if err != nil {
			return nil, fmt.Errorf("ledger: parse event timestamp %q: %w", at, err)
		}
		e.At = parsed

		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ledger: iterate event rows: %w", err)
	}

	return out, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func parseNullableTimestamp(v sql.NullString) (*time.Time, error) {
	if !v.Valid || v.String == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, v.String)
	if err != nil {
		return nil, fmt.Errorf("ledger: parse timestamp %q: %w", v.String, err)
	}
	return &t, nil
}
