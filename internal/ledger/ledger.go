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
	// lane_id) pair that has no row in the lanes table or approvals table.
	ErrLaneUnknown = errors.New("ledger: lane not found")

	// ErrAlreadyDecided is returned when Decide is called on an approval
	// row that has already been decided (decision != 'pending').
	ErrAlreadyDecided = errors.New("ledger: approval already decided")

	// ErrPragmaNotApplied is returned by Open when a required pragma
	// (journal_mode=wal or busy_timeout=5000) did not take effect on the
	// opened connection. A post-open PRAGMA Exec only reaches one
	// arbitrary pooled connection, so pragmas are set via the DSN and
	// then read back to convert a driver-specific DSN-syntax assumption
	// into a loud, first-cycle failure instead of a silent SQLITE_BUSY
	// later.
	ErrPragmaNotApplied = errors.New("ledger: pragma did not take effect")

	// ErrReconciliationRequestNotFound is returned when a reconciliation request is not found.
	ErrReconciliationRequestNotFound = errors.New("ledger: reconciliation request not found")

	// ErrReconciliationCandidateNotFound is returned when a reconciliation candidate is not found.
	ErrReconciliationCandidateNotFound = errors.New("ledger: reconciliation candidate not found")

	// ErrOverlapEvidenceNotFound is returned when overlap evidence is not found.
	ErrOverlapEvidenceNotFound = errors.New("ledger: overlap evidence not found")
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

// Event types this package produces.
const (
	EventRunStarted        = "run_started"
	EventLaneRegistered    = "lane_registered"
	EventLaneStatusChanged = "lane_status_changed"
	EventLaneNote          = "lane_note"
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

func formatNullableTimestamp(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

// Decision represents the status of an approval request.
type Decision string

const (
	DecisionPending  Decision = "pending"
	DecisionApproved Decision = "approved"
	DecisionRejected Decision = "rejected"
	DecisionTimedOut Decision = "timed_out"
)

// Valid reports whether d is one of the four defined decision values.
func (d Decision) Valid() bool {
	switch d {
	case DecisionPending, DecisionApproved, DecisionRejected, DecisionTimedOut:
		return true
	default:
		return false
	}
}

// Approval represents one row of the approvals table: a per-lane human approval
// decision record.
type Approval struct {
	RunID               string
	LaneID              string
	PacketID            string
	Approver            string
	Evidence            string
	Decision            Decision
	DefectSurfacedLater bool
	RequestedAt         time.Time
	DecidedAt           *time.Time
}

// RequestApproval records a new pending approval row for a lane.
func (l *Ledger) RequestApproval(ctx context.Context, app Approval) error {
	if app.Decision == "" {
		app.Decision = DecisionPending
	}
	if !app.Decision.Valid() {
		return fmt.Errorf("ledger: invalid approval decision %q", app.Decision)
	}
	if app.RequestedAt.IsZero() {
		app.RequestedAt = time.Now().UTC()
	}

	_, err := l.db.ExecContext(ctx, `
		INSERT INTO approvals (
			run_id, lane_id, packet_id, approver, decision, evidence,
			defect_surfaced_later, requested_at, decided_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		app.RunID, app.LaneID, app.PacketID, app.Approver, string(app.Decision),
		app.Evidence, boolToInt(app.DefectSurfacedLater),
		app.RequestedAt.UTC().Format(time.RFC3339),
		formatNullableTimestamp(app.DecidedAt),
	)
	if err != nil {
		return fmt.Errorf("ledger: request approval: %w", err)
	}
	return nil
}

// Decide records a decision (approved, rejected, timed_out) on a lane's approval row.
func (l *Ledger) Decide(ctx context.Context, runID, laneID, approver string, d Decision) error {
	if !d.Valid() {
		return fmt.Errorf("ledger: invalid approval decision %q", d)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := l.db.ExecContext(ctx, `
		UPDATE approvals
		SET decision = ?, approver = ?, decided_at = ?
		WHERE run_id = ? AND lane_id = ? AND decision = 'pending'`,
		string(d), approver, now, runID, laneID,
	)
	if err != nil {
		return fmt.Errorf("ledger: decide approval: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("ledger: read rows affected: %w", err)
	}
	if affected == 0 {
		if _, err := l.Approval(ctx, runID, laneID); err != nil {
			return err
		}
		return ErrAlreadyDecided
	}
	return nil
}

// MarkDefectSurfaced flags or unflags that a defect later surfaced for an approved lane.
func (l *Ledger) MarkDefectSurfaced(ctx context.Context, runID, laneID string, surfaced bool) error {
	res, err := l.db.ExecContext(ctx, `
		UPDATE approvals
		SET defect_surfaced_later = ?
		WHERE run_id = ? AND lane_id = ?`,
		boolToInt(surfaced), runID, laneID,
	)
	if err != nil {
		return fmt.Errorf("ledger: mark defect surfaced: %w", err)
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

// Approval returns the approval record for the given (runID, laneID).
func (l *Ledger) Approval(ctx context.Context, runID, laneID string) (Approval, error) {
	var (
		app            Approval
		dec            string
		defectSurfaced int
		reqAt          string
		decAt          sql.NullString
	)
	err := l.db.QueryRowContext(ctx, `
		SELECT run_id, lane_id, packet_id, approver, decision, evidence,
		       defect_surfaced_later, requested_at, decided_at
		FROM approvals
		WHERE run_id = ? AND lane_id = ?`,
		runID, laneID,
	).Scan(
		&app.RunID, &app.LaneID, &app.PacketID, &app.Approver, &dec,
		&app.Evidence, &defectSurfaced, &reqAt, &decAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Approval{}, ErrLaneUnknown
		}
		return Approval{}, fmt.Errorf("ledger: query approval: %w", err)
	}

	app.Decision = Decision(dec)
	app.DefectSurfacedLater = defectSurfaced != 0
	parsedReq, err := time.Parse(time.RFC3339, reqAt)
	if err != nil {
		return Approval{}, fmt.Errorf("ledger: parse requested_at %q: %w", reqAt, err)
	}
	app.RequestedAt = parsedReq
	if t, err := parseNullableTimestamp(decAt); err != nil {
		return Approval{}, err
	} else {
		app.DecidedAt = t
	}
	return app, nil
}

// PendingApprovals returns all pending approval rows ordered by requested_at.
func (l *Ledger) PendingApprovals(ctx context.Context) ([]Approval, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT run_id, lane_id, packet_id, approver, decision, evidence,
		       defect_surfaced_later, requested_at, decided_at
		FROM approvals
		WHERE decision = 'pending'
		ORDER BY requested_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("ledger: query pending approvals: %w", err)
	}
	defer rows.Close()
	return scanApprovals(rows)
}

// Approvals returns all approvals for a specific runID ordered by requested_at.
func (l *Ledger) Approvals(ctx context.Context, runID string) ([]Approval, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT run_id, lane_id, packet_id, approver, decision, evidence,
		       defect_surfaced_later, requested_at, decided_at
		FROM approvals
		WHERE run_id = ?
		ORDER BY requested_at ASC`, runID)
	if err != nil {
		return nil, fmt.Errorf("ledger: query approvals: %w", err)
	}
	defer rows.Close()
	return scanApprovals(rows)
}

func scanApprovals(rows *sql.Rows) ([]Approval, error) {
	var out []Approval
	for rows.Next() {
		var (
			app            Approval
			dec            string
			defectSurfaced int
			reqAt          string
			decAt          sql.NullString
		)
		if err := rows.Scan(
			&app.RunID, &app.LaneID, &app.PacketID, &app.Approver, &dec,
			&app.Evidence, &defectSurfaced, &reqAt, &decAt,
		); err != nil {
			return nil, fmt.Errorf("ledger: scan approval row: %w", err)
		}
		app.Decision = Decision(dec)
		app.DefectSurfacedLater = defectSurfaced != 0
		parsedReq, err := time.Parse(time.RFC3339, reqAt)
		if err != nil {
			return nil, fmt.Errorf("ledger: parse requested_at %q: %w", reqAt, err)
		}
		app.RequestedAt = parsedReq
		if t, err := parseNullableTimestamp(decAt); err != nil {
			return nil, err
		} else {
			app.DecidedAt = t
		}
		out = append(out, app)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ledger: iterate approval rows: %w", err)
	}
	return out, nil
}

// WaitDecision polls the approvals table every 250ms until a non-pending decision is reached
// or ctx is cancelled. If ctx is cancelled/times out while pending, it records timed_out.
func (l *Ledger) WaitDecision(ctx context.Context, runID, laneID string) (Approval, error) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		app, err := l.Approval(ctx, runID, laneID)
		if err != nil {
			return Approval{}, err
		}
		if app.Decision != DecisionPending {
			return app, nil
		}

		select {
		case <-ctx.Done():
			_ = l.Decide(context.Background(), runID, laneID, app.Approver, DecisionTimedOut)
			app, _ = l.Approval(context.Background(), runID, laneID)
			return app, ctx.Err()
		case <-ticker.C:
		}
	}
}

// ApproverRate calculates the wrong-approval rate for a given approver:
// flagged defects / approved count. If approved count is 0, returns 0.0.
func (l *Ledger) ApproverRate(ctx context.Context, approver string) (float64, error) {
	var totalApproved, flagged int
	err := l.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(defect_surfaced_later), 0)
		FROM approvals
		WHERE approver = ? AND decision = 'approved'`,
		approver,
	).Scan(&totalApproved, &flagged)
	if err != nil {
		return 0, fmt.Errorf("ledger: query approver rate: %w", err)
	}
	if totalApproved == 0 {
		return 0.0, nil
	}
	return float64(flagged) / float64(totalApproved), nil
}

// DB returns the underlying *sql.DB connection pool.
func (l *Ledger) DB() *sql.DB {
	return l.db
}

// IntegrationEvent is one row of the integration_events table: an append-only
// audit record of feature and reconciliation lifecycle events.
type IntegrationEvent struct {
	ID        int64
	FeatureID string
	AttemptID string
	Type      string
	Detail    string
	At        time.Time
}

// WriteWithAudit atomically executes an optional state mutation function and appends
// an IntegrationEvent in the same SQLite transaction. If either the mutation or
// the audit append fails, the transaction is rolled back.
func (l *Ledger) WriteWithAudit(ctx context.Context, mutate func(tx *sql.Tx) error, evt IntegrationEvent) error {
	if evt.FeatureID == "" {
		return errors.New("ledger: integration event feature_id is required")
	}
	if evt.Type == "" {
		return errors.New("ledger: integration event type is required")
	}

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ledger: begin write-with-audit tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once Commit succeeds

	if mutate != nil {
		if err := mutate(tx); err != nil {
			return err
		}
	}

	var attemptID sql.NullString
	if evt.AttemptID != "" {
		attemptID = sql.NullString{String: evt.AttemptID, Valid: true}
	}

	at := evt.At
	if at.IsZero() {
		at = time.Now().UTC()
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO integration_events (feature_id, attempt_id, type, detail, at) VALUES (?, ?, ?, ?, ?)`,
		evt.FeatureID, attemptID, evt.Type, evt.Detail, at.UTC().Format(time.RFC3339),
	); err != nil {
		return fmt.Errorf("ledger: insert integration event: %w", err)
	}

	return tx.Commit()
}

// PruneIntegrationEvents deletes all integration_events rows where at < cutoff.
// It returns the number of deleted rows.
func (l *Ledger) PruneIntegrationEvents(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := l.db.ExecContext(ctx,
		`DELETE FROM integration_events WHERE at < ?`,
		cutoff.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("ledger: prune integration events: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("ledger: read rows affected: %w", err)
	}
	return affected, nil
}

// IntegrationEvents returns all integration_events for the given featureID ordered by id.
func (l *Ledger) IntegrationEvents(ctx context.Context, featureID string) ([]IntegrationEvent, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT id, feature_id, attempt_id, type, detail, at
		FROM integration_events WHERE feature_id = ? ORDER BY id`, featureID)
	if err != nil {
		return nil, fmt.Errorf("ledger: query integration events: %w", err)
	}
	defer rows.Close()

	var out []IntegrationEvent
	for rows.Next() {
		var (
			e         IntegrationEvent
			attemptID sql.NullString
			at        string
		)
		if err := rows.Scan(&e.ID, &e.FeatureID, &attemptID, &e.Type, &e.Detail, &at); err != nil {
			return nil, fmt.Errorf("ledger: scan integration event row: %w", err)
		}
		if attemptID.Valid {
			e.AttemptID = attemptID.String
		}
		parsed, err := time.Parse(time.RFC3339, at)
		if err != nil {
			return nil, fmt.Errorf("ledger: parse integration event timestamp %q: %w", at, err)
		}
		e.At = parsed

		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ledger: iterate integration event rows: %w", err)
	}

	return out, nil
}

// OverlapEvidenceRow represents one row of the overlap_evidence table.
type OverlapEvidenceRow struct {
	ID            int64
	FeatureID     string
	Version       string
	EvidenceHash  string
	EvidenceClass string
	EvidenceJSON  string
	CreatedAt     time.Time
}

// InsertOverlapEvidence inserts a new overlap_evidence row and returns its generated ID.
func (l *Ledger) InsertOverlapEvidence(ctx context.Context, row OverlapEvidenceRow) (int64, error) {
	at := row.CreatedAt
	if at.IsZero() {
		at = time.Now().UTC()
	}
	res, err := l.db.ExecContext(ctx, `
		INSERT INTO overlap_evidence (feature_id, version, evidence_hash, evidence_class, evidence_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		row.FeatureID, row.Version, row.EvidenceHash, row.EvidenceClass, row.EvidenceJSON, at.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("ledger: insert overlap evidence: %w", err)
	}
	return res.LastInsertId()
}

// OverlapEvidence retrieves an overlap_evidence row by featureID and evidenceHash.
func (l *Ledger) OverlapEvidence(ctx context.Context, featureID, evidenceHash string) (OverlapEvidenceRow, error) {
	var (
		row       OverlapEvidenceRow
		createdAt string
	)
	err := l.db.QueryRowContext(ctx, `
		SELECT id, feature_id, version, evidence_hash, evidence_class, evidence_json, created_at
		FROM overlap_evidence WHERE feature_id = ? AND evidence_hash = ? ORDER BY id DESC LIMIT 1`,
		featureID, evidenceHash,
	).Scan(&row.ID, &row.FeatureID, &row.Version, &row.EvidenceHash, &row.EvidenceClass, &row.EvidenceJSON, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OverlapEvidenceRow{}, ErrOverlapEvidenceNotFound
		}
		return OverlapEvidenceRow{}, fmt.Errorf("ledger: query overlap evidence: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return OverlapEvidenceRow{}, fmt.Errorf("ledger: parse overlap evidence created_at %q: %w", createdAt, err)
	}
	row.CreatedAt = parsed
	return row, nil
}

// OverlapEvidenceByHash retrieves an overlap_evidence row by evidenceHash across any feature.
func (l *Ledger) OverlapEvidenceByHash(ctx context.Context, evidenceHash string) (OverlapEvidenceRow, error) {
	var (
		row       OverlapEvidenceRow
		createdAt string
	)
	err := l.db.QueryRowContext(ctx, `
		SELECT id, feature_id, version, evidence_hash, evidence_class, evidence_json, created_at
		FROM overlap_evidence WHERE evidence_hash = ? ORDER BY id DESC LIMIT 1`,
		evidenceHash,
	).Scan(&row.ID, &row.FeatureID, &row.Version, &row.EvidenceHash, &row.EvidenceClass, &row.EvidenceJSON, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OverlapEvidenceRow{}, ErrOverlapEvidenceNotFound
		}
		return OverlapEvidenceRow{}, fmt.Errorf("ledger: query overlap evidence by hash: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return OverlapEvidenceRow{}, fmt.Errorf("ledger: parse overlap evidence created_at %q: %w", createdAt, err)
	}
	row.CreatedAt = parsed
	return row, nil
}

// ReconciliationRequestRow represents one row of the reconciliation_requests table.
type ReconciliationRequestRow struct {
	ID              string
	FeatureID       string
	Direction       string
	Status          string
	Actor           string
	EvidenceVersion string
	EvidenceHash    string
	SourceSHA       string
	TargetSHA       string
	ExpiresAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// InsertReconciliationRequest inserts a new reconciliation_requests row.
func (l *Ledger) InsertReconciliationRequest(ctx context.Context, req ReconciliationRequestRow) error {
	cAt := req.CreatedAt
	if cAt.IsZero() {
		cAt = time.Now().UTC()
	}
	uAt := req.UpdatedAt
	if uAt.IsZero() {
		uAt = cAt
	}
	_, err := l.db.ExecContext(ctx, `
		INSERT INTO reconciliation_requests (
			id, feature_id, direction, status, actor, evidence_version,
			evidence_hash, source_sha, target_sha, expires_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.ID, req.FeatureID, req.Direction, req.Status, req.Actor, req.EvidenceVersion,
		req.EvidenceHash, req.SourceSHA, req.TargetSHA, formatNullableTimestamp(req.ExpiresAt),
		cAt.UTC().Format(time.RFC3339), uAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("ledger: insert reconciliation request: %w", err)
	}
	return nil
}

// ReconciliationRequest retrieves a single reconciliation_requests row by ID.
func (l *Ledger) ReconciliationRequest(ctx context.Context, id string) (ReconciliationRequestRow, error) {
	var (
		req                  ReconciliationRequestRow
		expiresAt            sql.NullString
		createdAt, updatedAt string
	)
	err := l.db.QueryRowContext(ctx, `
		SELECT id, feature_id, direction, status, actor, evidence_version,
		       evidence_hash, source_sha, target_sha, expires_at, created_at, updated_at
		FROM reconciliation_requests WHERE id = ?`, id,
	).Scan(
		&req.ID, &req.FeatureID, &req.Direction, &req.Status, &req.Actor, &req.EvidenceVersion,
		&req.EvidenceHash, &req.SourceSHA, &req.TargetSHA, &expiresAt, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ReconciliationRequestRow{}, ErrReconciliationRequestNotFound
		}
		return ReconciliationRequestRow{}, fmt.Errorf("ledger: query reconciliation request %q: %w", id, err)
	}
	if t, err := parseNullableTimestamp(expiresAt); err != nil {
		return ReconciliationRequestRow{}, err
	} else {
		req.ExpiresAt = t
	}
	if t, err := time.Parse(time.RFC3339, createdAt); err != nil {
		return ReconciliationRequestRow{}, fmt.Errorf("ledger: parse request created_at %q: %w", createdAt, err)
	} else {
		req.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339, updatedAt); err != nil {
		return ReconciliationRequestRow{}, fmt.Errorf("ledger: parse request updated_at %q: %w", updatedAt, err)
	} else {
		req.UpdatedAt = t
	}
	return req, nil
}

// ReconciliationRequests retrieves all reconciliation_requests rows for a featureID ordered by created_at.
func (l *Ledger) ReconciliationRequests(ctx context.Context, featureID string) ([]ReconciliationRequestRow, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT id, feature_id, direction, status, actor, evidence_version,
		       evidence_hash, source_sha, target_sha, expires_at, created_at, updated_at
		FROM reconciliation_requests WHERE feature_id = ? ORDER BY created_at ASC`, featureID,
	)
	if err != nil {
		return nil, fmt.Errorf("ledger: query reconciliation requests: %w", err)
	}
	defer rows.Close()

	var out []ReconciliationRequestRow
	for rows.Next() {
		var (
			req                  ReconciliationRequestRow
			expiresAt            sql.NullString
			createdAt, updatedAt string
		)
		if err := rows.Scan(
			&req.ID, &req.FeatureID, &req.Direction, &req.Status, &req.Actor, &req.EvidenceVersion,
			&req.EvidenceHash, &req.SourceSHA, &req.TargetSHA, &expiresAt, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("ledger: scan reconciliation request row: %w", err)
		}
		if t, err := parseNullableTimestamp(expiresAt); err != nil {
			return nil, err
		} else {
			req.ExpiresAt = t
		}
		if t, err := time.Parse(time.RFC3339, createdAt); err != nil {
			return nil, fmt.Errorf("ledger: parse request created_at %q: %w", createdAt, err)
		} else {
			req.CreatedAt = t
		}
		if t, err := time.Parse(time.RFC3339, updatedAt); err != nil {
			return nil, fmt.Errorf("ledger: parse request updated_at %q: %w", updatedAt, err)
		} else {
			req.UpdatedAt = t
		}
		out = append(out, req)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ledger: iterate reconciliation request rows: %w", err)
	}
	return out, nil
}

// UpdateReconciliationRequest updates mutable fields of an existing reconciliation_requests row.
func (l *Ledger) UpdateReconciliationRequest(ctx context.Context, req ReconciliationRequestRow) error {
	uAt := req.UpdatedAt
	if uAt.IsZero() {
		uAt = time.Now().UTC()
	}
	res, err := l.db.ExecContext(ctx, `
		UPDATE reconciliation_requests
		SET direction = ?, status = ?, actor = ?, evidence_version = ?,
		    evidence_hash = ?, source_sha = ?, target_sha = ?, expires_at = ?, updated_at = ?
		WHERE id = ?`,
		req.Direction, req.Status, req.Actor, req.EvidenceVersion,
		req.EvidenceHash, req.SourceSHA, req.TargetSHA, formatNullableTimestamp(req.ExpiresAt),
		uAt.UTC().Format(time.RFC3339), req.ID,
	)
	if err != nil {
		return fmt.Errorf("ledger: update reconciliation request: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("ledger: read rows affected: %w", err)
	}
	if affected == 0 {
		return ErrReconciliationRequestNotFound
	}
	return nil
}

// ReconciliationCandidateRow represents one row of the reconciliation_candidates table.
type ReconciliationCandidateRow struct {
	ID            string
	RequestID     string
	Status        string
	AllowedPaths  string
	Model         string
	Config        string
	Output        string
	Checks        string
	CandidateSHA  string
	FailureReason string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// InsertReconciliationCandidate inserts a new reconciliation_candidates row.
func (l *Ledger) InsertReconciliationCandidate(ctx context.Context, cand ReconciliationCandidateRow) error {
	cAt := cand.CreatedAt
	if cAt.IsZero() {
		cAt = time.Now().UTC()
	}
	uAt := cand.UpdatedAt
	if uAt.IsZero() {
		uAt = cAt
	}
	_, err := l.db.ExecContext(ctx, `
		INSERT INTO reconciliation_candidates (
			id, request_id, status, allowed_paths, model, config,
			output, checks, candidate_sha, failure_reason, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cand.ID, cand.RequestID, cand.Status, cand.AllowedPaths, cand.Model, cand.Config,
		cand.Output, cand.Checks, cand.CandidateSHA, cand.FailureReason,
		cAt.UTC().Format(time.RFC3339), uAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("ledger: insert reconciliation candidate: %w", err)
	}
	return nil
}

// ReconciliationCandidate retrieves a reconciliation_candidates row by ID.
func (l *Ledger) ReconciliationCandidate(ctx context.Context, id string) (ReconciliationCandidateRow, error) {
	var (
		cand                 ReconciliationCandidateRow
		createdAt, updatedAt string
	)
	err := l.db.QueryRowContext(ctx, `
		SELECT id, request_id, status, allowed_paths, model, config,
		       output, checks, candidate_sha, failure_reason, created_at, updated_at
		FROM reconciliation_candidates WHERE id = ?`, id,
	).Scan(
		&cand.ID, &cand.RequestID, &cand.Status, &cand.AllowedPaths, &cand.Model, &cand.Config,
		&cand.Output, &cand.Checks, &cand.CandidateSHA, &cand.FailureReason, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ReconciliationCandidateRow{}, ErrReconciliationCandidateNotFound
		}
		return ReconciliationCandidateRow{}, fmt.Errorf("ledger: query reconciliation candidate %q: %w", id, err)
	}
	if t, err := time.Parse(time.RFC3339, createdAt); err != nil {
		return ReconciliationCandidateRow{}, fmt.Errorf("ledger: parse candidate created_at %q: %w", createdAt, err)
	} else {
		cand.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339, updatedAt); err != nil {
		return ReconciliationCandidateRow{}, fmt.Errorf("ledger: parse candidate updated_at %q: %w", updatedAt, err)
	} else {
		cand.UpdatedAt = t
	}
	return cand, nil
}

// ReconciliationCandidateByRequest retrieves the most recent reconciliation_candidates row for a given requestID.
func (l *Ledger) ReconciliationCandidateByRequest(ctx context.Context, requestID string) (ReconciliationCandidateRow, error) {
	var (
		cand                 ReconciliationCandidateRow
		createdAt, updatedAt string
	)
	err := l.db.QueryRowContext(ctx, `
		SELECT id, request_id, status, allowed_paths, model, config,
		       output, checks, candidate_sha, failure_reason, created_at, updated_at
		FROM reconciliation_candidates WHERE request_id = ? ORDER BY created_at DESC LIMIT 1`, requestID,
	).Scan(
		&cand.ID, &cand.RequestID, &cand.Status, &cand.AllowedPaths, &cand.Model, &cand.Config,
		&cand.Output, &cand.Checks, &cand.CandidateSHA, &cand.FailureReason, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ReconciliationCandidateRow{}, ErrReconciliationCandidateNotFound
		}
		return ReconciliationCandidateRow{}, fmt.Errorf("ledger: query reconciliation candidate for request %q: %w", requestID, err)
	}
	if t, err := time.Parse(time.RFC3339, createdAt); err != nil {
		return ReconciliationCandidateRow{}, fmt.Errorf("ledger: parse candidate created_at %q: %w", createdAt, err)
	} else {
		cand.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339, updatedAt); err != nil {
		return ReconciliationCandidateRow{}, fmt.Errorf("ledger: parse candidate updated_at %q: %w", updatedAt, err)
	} else {
		cand.UpdatedAt = t
	}
	return cand, nil
}

// ReconciliationCandidates retrieves all reconciliation_candidates rows for a requestID ordered by created_at.
func (l *Ledger) ReconciliationCandidates(ctx context.Context, requestID string) ([]ReconciliationCandidateRow, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT id, request_id, status, allowed_paths, model, config,
		       output, checks, candidate_sha, failure_reason, created_at, updated_at
		FROM reconciliation_candidates WHERE request_id = ? ORDER BY created_at ASC`, requestID,
	)
	if err != nil {
		return nil, fmt.Errorf("ledger: query reconciliation candidates: %w", err)
	}
	defer rows.Close()

	var out []ReconciliationCandidateRow
	for rows.Next() {
		var (
			cand                 ReconciliationCandidateRow
			createdAt, updatedAt string
		)
		if err := rows.Scan(
			&cand.ID, &cand.RequestID, &cand.Status, &cand.AllowedPaths, &cand.Model, &cand.Config,
			&cand.Output, &cand.Checks, &cand.CandidateSHA, &cand.FailureReason, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("ledger: scan reconciliation candidate row: %w", err)
		}
		if t, err := time.Parse(time.RFC3339, createdAt); err != nil {
			return nil, fmt.Errorf("ledger: parse candidate created_at %q: %w", createdAt, err)
		} else {
			cand.CreatedAt = t
		}
		if t, err := time.Parse(time.RFC3339, updatedAt); err != nil {
			return nil, fmt.Errorf("ledger: parse candidate updated_at %q: %w", updatedAt, err)
		} else {
			cand.UpdatedAt = t
		}
		out = append(out, cand)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ledger: iterate reconciliation candidate rows: %w", err)
	}
	return out, nil
}

// UpdateReconciliationCandidate updates mutable fields of an existing reconciliation_candidates row.
func (l *Ledger) UpdateReconciliationCandidate(ctx context.Context, cand ReconciliationCandidateRow) error {
	uAt := cand.UpdatedAt
	if uAt.IsZero() {
		uAt = time.Now().UTC()
	}
	res, err := l.db.ExecContext(ctx, `
		UPDATE reconciliation_candidates
		SET status = ?, allowed_paths = ?, model = ?, config = ?,
		    output = ?, checks = ?, candidate_sha = ?, failure_reason = ?, updated_at = ?
		WHERE id = ?`,
		cand.Status, cand.AllowedPaths, cand.Model, cand.Config,
		cand.Output, cand.Checks, cand.CandidateSHA, cand.FailureReason,
		uAt.UTC().Format(time.RFC3339), cand.ID,
	)
	if err != nil {
		return fmt.Errorf("ledger: update reconciliation candidate: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("ledger: read rows affected: %w", err)
	}
	if affected == 0 {
		return ErrReconciliationCandidateNotFound
	}
	return nil
}
