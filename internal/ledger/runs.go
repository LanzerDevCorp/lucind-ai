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
