package ledger

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrInvalidProgress identifies a progress message that cannot be persisted.
var ErrInvalidProgress = errors.New("ledger: invalid progress")

// LaneProgress is one sequenced progress message emitted by a lane.
type LaneProgress struct {
	RunID, LaneID string
	Seq           int64
	Message       string
	At            time.Time
	TotalTokens   int64
	CostUSD       float64
	ToolCalls     int64
}

// ProgressErrorReporter observes a best-effort append failure.
type ProgressErrorReporter func(error)

// AppendProgress appends one progress message.
func (l *Ledger) AppendProgress(ctx context.Context, progress LaneProgress) error {
	return l.AppendProgressBatch(ctx, []LaneProgress{progress})
}

// AppendProgressBestEffort reports errors without interrupting capture.
func (l *Ledger) AppendProgressBestEffort(ctx context.Context, progress LaneProgress, report ProgressErrorReporter) {
	l.AppendProgressBatchBestEffort(ctx, []LaneProgress{progress}, report)
}

// AppendProgressBatch atomically appends a batch of progress messages.
func (l *Ledger) AppendProgressBatch(ctx context.Context, batch []LaneProgress) error {
	if len(batch) == 0 {
		return nil
	}
	for _, progress := range batch {
		if err := validateProgress(progress); err != nil {
			return err
		}
	}

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ledger: begin append-progress tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once Commit succeeds

	for _, progress := range batch {
		var err error
		if progress.Seq == 0 {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO lane_progress (run_id, lane_id, seq, message, at, total_tokens, cost_usd, tool_calls)
				SELECT ?, ?, COALESCE(MAX(seq), 0) + 1, ?, ?, ?, ?, ?
				FROM lane_progress WHERE run_id = ? AND lane_id = ?`,
				progress.RunID, progress.LaneID, progress.Message,
				progress.At.UTC().Format(time.RFC3339),
				progress.TotalTokens, progress.CostUSD, progress.ToolCalls,
				progress.RunID, progress.LaneID,
			)
		} else {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO lane_progress (run_id, lane_id, seq, message, at, total_tokens, cost_usd, tool_calls)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				progress.RunID, progress.LaneID, progress.Seq, progress.Message,
				progress.At.UTC().Format(time.RFC3339),
				progress.TotalTokens, progress.CostUSD, progress.ToolCalls,
			)
		}
		if err != nil {
			return fmt.Errorf("ledger: append progress: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ledger: commit append-progress tx: %w", err)
	}
	return nil
}

// AppendProgressBatchBestEffort reports an atomic batch failure when configured.
func (l *Ledger) AppendProgressBatchBestEffort(ctx context.Context, batch []LaneProgress, report ProgressErrorReporter) {
	if err := l.AppendProgressBatch(ctx, batch); err != nil && report != nil {
		report(err)
	}
}
func validateProgress(progress LaneProgress) error {
	switch {
	case strings.TrimSpace(progress.RunID) == "":
		return fmt.Errorf("%w: run_id is required", ErrInvalidProgress)
	case strings.TrimSpace(progress.LaneID) == "":
		return fmt.Errorf("%w: lane_id is required", ErrInvalidProgress)
	case progress.Message == "":
		return fmt.Errorf("%w: message is required", ErrInvalidProgress)
	case progress.Seq < 0:
		return fmt.Errorf("%w: sequence must not be negative", ErrInvalidProgress)
	case progress.At.IsZero():
		return fmt.Errorf("%w: timestamp is required", ErrInvalidProgress)
	case progress.TotalTokens < 0:
		return fmt.Errorf("%w: total_tokens must not be negative", ErrInvalidProgress)
	case progress.CostUSD < 0:
		return fmt.Errorf("%w: cost_usd must not be negative", ErrInvalidProgress)
	case progress.ToolCalls < 0:
		return fmt.Errorf("%w: tool_calls must not be negative", ErrInvalidProgress)
	default:
		return nil
	}
}

// GetProgressAfter returns seq > afterSeq in ascending sequence order.
func (l *Ledger) GetProgressAfter(ctx context.Context, runID, laneID string, afterSeq int64) ([]LaneProgress, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT run_id, lane_id, seq, message, at, total_tokens, cost_usd, tool_calls
		FROM lane_progress
		WHERE run_id = ? AND lane_id = ? AND seq > ?
		ORDER BY seq ASC`, runID, laneID, afterSeq)
	if err != nil {
		return nil, fmt.Errorf("ledger: query progress: %w", err)
	}
	defer rows.Close()

	out := make([]LaneProgress, 0)
	for rows.Next() {
		var progress LaneProgress
		var at string
		if err := rows.Scan(&progress.RunID, &progress.LaneID, &progress.Seq, &progress.Message, &at,
			&progress.TotalTokens, &progress.CostUSD, &progress.ToolCalls); err != nil {
			return nil, fmt.Errorf("ledger: scan progress row: %w", err)
		}
		progress.At, err = time.Parse(time.RFC3339, at)
		if err != nil {
			return nil, fmt.Errorf("ledger: parse progress timestamp %q: %w", at, err)
		}
		out = append(out, progress)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ledger: iterate progress rows: %w", err)
	}
	return out, nil
}

// PruneProgress deletes progress older than cutoff and returns the row count.
func (l *Ledger) PruneProgress(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := l.db.ExecContext(ctx,
		`DELETE FROM lane_progress WHERE at < ?`,
		cutoff.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("ledger: prune progress: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("ledger: read pruned progress rows: %w", err)
	}
	return affected, nil
}
