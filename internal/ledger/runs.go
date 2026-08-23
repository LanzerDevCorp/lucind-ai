package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrRunUnknown is returned when an operation targets a run_id that has no
// row in the runs table.
var ErrRunUnknown = errors.New("ledger: run not found")

// Run is one row of the runs table.
type Run struct {
	RunID     string
	FeatureID string
	Status    string
	TargetRef string
	LaneCount int
	StartedAt time.Time
	EndedAt   *time.Time
}

// RegisterRun inserts a run row. A duplicate RunID is returned as the
// underlying SQLite primary-key error so callers can observe the rejected
// duplicate registration.
func (l *Ledger) RegisterRun(ctx context.Context, run Run) error {
	_, err := l.db.ExecContext(ctx, `
		INSERT INTO runs (
			run_id, feature_id, status, target_ref, lane_count, started_at, ended_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		run.RunID, run.FeatureID, run.Status, run.TargetRef, run.LaneCount,
		run.StartedAt.UTC().Format(time.RFC3339), formatNullableTimestamp(run.EndedAt),
	)
	if err != nil {
		return fmt.Errorf("ledger: register run: %w", err)
	}
	return nil
}

// UpdateRunStatus records a run's terminal status and completion timestamp.
func (l *Ledger) UpdateRunStatus(ctx context.Context, runID, status string, endedAt time.Time) error {
	res, err := l.db.ExecContext(ctx,
		`UPDATE runs SET status = ?, ended_at = ? WHERE run_id = ?`,
		status, endedAt.UTC().Format(time.RFC3339), runID,
	)
	if err != nil {
		return fmt.Errorf("ledger: update run status: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("ledger: read run rows affected: %w", err)
	}
	if affected == 0 {
		return ErrRunUnknown
	}
	return nil
}

// GetRun returns the run identified by runID.
func (l *Ledger) GetRun(ctx context.Context, runID string) (Run, error) {
	row := l.db.QueryRowContext(ctx, `
		SELECT run_id, feature_id, status, target_ref, lane_count, started_at, ended_at
		FROM runs WHERE run_id = ?`, runID)

	run, err := scanRun(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, ErrRunUnknown
	}
	if err != nil {
		return Run{}, fmt.Errorf("ledger: get run: %w", err)
	}
	return run, nil
}

// ListRuns returns all runs ordered newest first, with run_id as a stable
// tie-breaker.
func (l *Ledger) ListRuns(ctx context.Context) ([]Run, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT run_id, feature_id, status, target_ref, lane_count, started_at, ended_at
		FROM runs ORDER BY started_at DESC, run_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("ledger: list runs: %w", err)
	}
	defer rows.Close()

	var runs []Run
	for rows.Next() {
		run, err := scanRun(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("ledger: scan run: %w", err)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ledger: iterate runs: %w", err)
	}
	return runs, nil
}

// RunIDsByRecentEvent returns every run_id present in the events table,
// ordered by that run's own most recent event id descending (newest first).
//
// This exists so a caller can recover a run window even when the runs table
// has no matching row for a run_id that events (and lanes) already carry --
// lucind-ai run only started calling RegisterRun recently, and nothing ever
// backfills the runs table for a ledger dispatched before that. events.id is
// a global autoincrement across the whole table (not scoped per run), so
// grouping by run_id and taking MAX(id) is a correct, cheap "most recently
// active" signal without touching the runs table at all.
func (l *Ledger) RunIDsByRecentEvent(ctx context.Context) ([]string, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT run_id
		FROM events
		WHERE run_id IS NOT NULL AND run_id != ''
		GROUP BY run_id
		ORDER BY MAX(id) DESC`)
	if err != nil {
		return nil, fmt.Errorf("ledger: run ids by recent event: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("ledger: scan run id by recent event: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ledger: iterate run ids by recent event: %w", err)
	}
	return ids, nil
}

// DistinctLaneRunIDs returns every run_id present in the lanes table, in no
// particular recency order (lanes carries no ordering signal comparable to
// events.id). It exists to cover the residual case RunIDsByRecentEvent
// cannot: a run whose lanes were registered but which has not (yet, or
// ever) produced a lifecycle event of its own.
func (l *Ledger) DistinctLaneRunIDs(ctx context.Context) ([]string, error) {
	rows, err := l.db.QueryContext(ctx, `SELECT DISTINCT run_id FROM lanes ORDER BY run_id`)
	if err != nil {
		return nil, fmt.Errorf("ledger: distinct lane run ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("ledger: scan distinct lane run id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ledger: iterate distinct lane run ids: %w", err)
	}
	return ids, nil
}

func scanRun(scan func(...any) error) (Run, error) {
	var (
		run       Run
		startedAt string
		endedAt   sql.NullString
	)
	if err := scan(
		&run.RunID, &run.FeatureID, &run.Status, &run.TargetRef, &run.LaneCount,
		&startedAt, &endedAt,
	); err != nil {
		return Run{}, err
	}

	parsedStartedAt, err := time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return Run{}, fmt.Errorf("parse started_at %q: %w", startedAt, err)
	}
	run.StartedAt = parsedStartedAt
	run.EndedAt, err = parseNullableTimestamp(endedAt)
	if err != nil {
		return Run{}, err
	}
	return run, nil
}
