package ledger

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
)

func TestAppendProgressBatchAndCursorTail(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()
	at := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)

	batch := []LaneProgress{
		{RunID: "run-1", LaneID: "lane-a", Seq: 3, Message: "third", At: at.Add(2 * time.Second), TotalTokens: 30, CostUSD: 0.03, ToolCalls: 3},
		{RunID: "run-1", LaneID: "lane-a", Seq: 1, Message: "first", At: at, TotalTokens: 10, CostUSD: 0.01, ToolCalls: 1},
		{RunID: "run-1", LaneID: "lane-a", Seq: 2, Message: "second", At: at.Add(time.Second), TotalTokens: 20, CostUSD: 0.02, ToolCalls: 2},
	}
	if err := l.AppendProgressBatch(ctx, batch); err != nil {
		t.Fatalf("AppendProgressBatch: %v", err)
	}

	got, err := l.GetProgressAfter(ctx, "run-1", "lane-a", 1)
	if err != nil {
		t.Fatalf("GetProgressAfter: %v", err)
	}
	if len(got) != 2 || got[0].Seq != 2 || got[0].Message != "second" ||
		!got[0].At.Equal(at.Add(time.Second)) || got[1].Seq != 3 || got[1].Message != "third" {
		t.Fatalf("tail = %+v, want ordered second and third messages", got)
	}
	if got[0].TotalTokens != 20 || got[0].CostUSD != 0.02 || got[0].ToolCalls != 2 ||
		got[1].TotalTokens != 30 || got[1].CostUSD != 0.03 || got[1].ToolCalls != 3 {
		t.Fatalf("usage tail = %+v, want tokens/cost/tools 20/0.02/2 then 30/0.03/3", got)
	}

	empty, err := l.GetProgressAfter(ctx, "run-1", "lane-a", 3)
	if err != nil {
		t.Fatalf("GetProgressAfter at latest sequence: %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("GetProgressAfter at latest sequence = %#v, want non-nil empty slice", empty)
	}

	var eventCount int
	if err := l.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events`).Scan(&eventCount); err != nil || eventCount != 0 {
		t.Fatalf("events count = %d, error = %v; want 0, nil", eventCount, err)
	}
}

func TestAppendProgressSequenceValidationAndAtomicity(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()
	at := time.Date(2026, 8, 22, 11, 0, 0, 0, time.UTC)

	if err := l.AppendProgress(ctx, LaneProgress{
		RunID: "run-1", LaneID: "lane-a", Seq: 5, Message: "explicit", At: at,
	}); err != nil {
		t.Fatalf("AppendProgress explicit sequence: %v", err)
	}
	if err := l.AppendProgressBatch(ctx, []LaneProgress{
		{RunID: "run-1", LaneID: "lane-a", Message: "automatic-1", At: at.Add(time.Second)},
		{RunID: "run-1", LaneID: "lane-a", Message: "automatic-2", At: at.Add(2 * time.Second)},
	}); err != nil {
		t.Fatalf("AppendProgressBatch automatic sequences: %v", err)
	}

	got, err := l.GetProgressAfter(ctx, "run-1", "lane-a", 5)
	if err != nil {
		t.Fatalf("GetProgressAfter automatic sequences: %v", err)
	}
	if len(got) != 2 || got[0].Seq != 6 || got[1].Seq != 7 {
		t.Fatalf("automatic sequences = %+v, want 6 then 7", got)
	}

	invalid := []struct {
		name     string
		progress LaneProgress
	}{
		{"missing run ID", LaneProgress{LaneID: "lane-a", Seq: 1, Message: "message", At: at}},
		{"empty message", LaneProgress{RunID: "run-1", LaneID: "lane-a", Seq: 1, At: at}},
		{"negative sequence", LaneProgress{RunID: "run-1", LaneID: "lane-a", Seq: -1, Message: "message", At: at}},
		{"zero timestamp", LaneProgress{RunID: "run-1", LaneID: "lane-a", Seq: 1, Message: "message"}},
		{"negative tokens", LaneProgress{RunID: "run-1", LaneID: "lane-a", Seq: 1, Message: "message", At: at, TotalTokens: -1}},
		{"negative cost", LaneProgress{RunID: "run-1", LaneID: "lane-a", Seq: 1, Message: "message", At: at, CostUSD: -0.01}},
		{"negative tool calls", LaneProgress{RunID: "run-1", LaneID: "lane-a", Seq: 1, Message: "message", At: at, ToolCalls: -1}},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			err := l.AppendProgress(ctx, tt.progress)
			if !errors.Is(err, ErrInvalidProgress) {
				t.Fatalf("AppendProgress error = %v, want ErrInvalidProgress", err)
			}
		})
	}

	err = l.AppendProgressBatch(ctx, []LaneProgress{
		{RunID: "run-1", LaneID: "lane-a", Seq: 8, Message: "must roll back", At: at},
		{RunID: "run-1", LaneID: "lane-a", Seq: 5, Message: "duplicate", At: at},
	})
	if err == nil {
		t.Fatal("AppendProgressBatch duplicate sequence = nil error, want primary-key error")
	}
	rolledBack, err := l.GetProgressAfter(ctx, "run-1", "lane-a", 7)
	if err != nil {
		t.Fatalf("GetProgressAfter rolled-back batch: %v", err)
	}
	if len(rolledBack) != 0 {
		t.Fatalf("rolled-back batch left rows: %+v", rolledBack)
	}
}

func TestPruneProgressIsolated(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()
	cutoff := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)

	if _, err := l.db.ExecContext(ctx, `
		INSERT INTO runs (run_id, feature_id, status, target_ref, lane_count, started_at)
		VALUES ('run-1', 'feature-1', 'running', 'main', 1, ?)`,
		cutoff.Add(-time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if err := l.RegisterLane(ctx, Lane{
		RunID: "run-1", LaneID: "lane-a", PacketID: "packet-1", Executor: "agy",
		RoutingCondition: "progress retention", Status: lane.Running,
	}); err != nil {
		t.Fatalf("RegisterLane: %v", err)
	}
	if err := l.RequestApproval(ctx, Approval{
		RunID: "run-1", LaneID: "lane-a", PacketID: "packet-1", RequestedAt: cutoff,
	}); err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}
	if err := l.AppendEvent(ctx, Event{
		RunID: "run-1", LaneID: "lane-a", Type: EventLaneNote, Detail: "keep", At: cutoff,
	}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	if err := l.AppendProgressBatch(ctx, []LaneProgress{
		{RunID: "run-1", LaneID: "lane-a", Seq: 1, Message: "expired", At: cutoff.Add(-time.Second)},
		{RunID: "run-1", LaneID: "lane-a", Seq: 2, Message: "at cutoff", At: cutoff},
		{RunID: "run-1", LaneID: "lane-a", Seq: 3, Message: "new", At: cutoff.Add(time.Second)},
	}); err != nil {
		t.Fatalf("AppendProgressBatch: %v", err)
	}

	pruned, err := l.PruneProgress(ctx, cutoff.Add(-time.Hour))
	if err != nil || pruned != 0 {
		t.Fatalf("PruneProgress before all rows = (%d, %v), want (0, nil)", pruned, err)
	}
	pruned, err = l.PruneProgress(ctx, cutoff)
	if err != nil || pruned != 1 {
		t.Fatalf("PruneProgress at cutoff = (%d, %v), want (1, nil)", pruned, err)
	}

	remaining, err := l.GetProgressAfter(ctx, "run-1", "lane-a", 0)
	if err != nil {
		t.Fatalf("GetProgressAfter after prune: %v", err)
	}
	if len(remaining) != 2 || remaining[0].Seq != 2 || remaining[1].Seq != 3 {
		t.Fatalf("remaining progress = %+v, want sequences 2 and 3", remaining)
	}
	for _, table := range []string{"runs", "lanes", "events", "approvals"} {
		var count int
		if err := l.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&count); err != nil || count != 1 {
			t.Errorf("%s count = %d, error = %v; want 1, nil", table, count, err)
		}
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := l.PruneProgress(context.Background(), time.Now()); err == nil {
		t.Fatal("PruneProgress on closed database = nil error, want non-nil error")
	}
}

func TestAppendProgressBestEffortReportsWithoutInterruptingCapture(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()
	at := time.Date(2026, 8, 22, 13, 0, 0, 0, time.UTC)
	l.AppendProgressBestEffort(ctx, LaneProgress{
		RunID: "run-1", LaneID: "lane-a", Seq: 1, Message: "captured", At: at,
	}, func(err error) { t.Fatalf("successful best-effort append reported error: %v", err) })

	var reported []error
	l.AppendProgressBatchBestEffort(ctx, []LaneProgress{
		{RunID: "run-1", LaneID: "lane-a", Seq: 2, Message: "must roll back", At: at},
		{RunID: "", LaneID: "lane-a", Seq: 2, Message: "invalid", At: at},
	}, func(err error) {
		reported = append(reported, err)
	})
	if len(reported) != 1 || !errors.Is(reported[0], ErrInvalidProgress) {
		t.Fatalf("reported errors = %v, want one ErrInvalidProgress", reported)
	}

	got, err := l.GetProgressAfter(ctx, "run-1", "lane-a", 0)
	if err != nil {
		t.Fatalf("GetProgressAfter failed best-effort batch: %v", err)
	}
	if len(got) != 1 || got[0].Message != "captured" {
		t.Fatalf("best-effort batch was not atomic: %+v", got)
	}
}

func TestConcurrentProgressAndSetStatus(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()
	const writes = 12
	if err := l.RegisterLane(ctx, Lane{
		RunID: "run-concurrent", LaneID: "lane-a", PacketID: "packet-1", Executor: "agy",
		RoutingCondition: "concurrent progress", Status: lane.Running,
	}); err != nil {
		t.Fatalf("RegisterLane: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, writes*2)
	for i := 0; i < writes; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := l.AppendProgress(ctx, LaneProgress{
				RunID: "run-concurrent", LaneID: "lane-a", Message: fmt.Sprintf("chunk-%d", i),
				At: time.Date(2026, 8, 22, 14, 0, i, 0, time.UTC),
			}); err != nil {
				errs <- fmt.Errorf("AppendProgress(%d): %w", i, err)
			}
			if err := l.SetStatus(ctx, "run-concurrent", "lane-a", lane.Running, time.Now()); err != nil {
				errs <- fmt.Errorf("SetStatus(%d): %w", i, err)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent operation: %v", err)
	}

	got, err := l.GetProgressAfter(ctx, "run-concurrent", "lane-a", 0)
	if err != nil {
		t.Fatalf("GetProgressAfter: %v", err)
	}
	if len(got) != writes {
		t.Fatalf("progress row count = %d, want %d", len(got), writes)
	}
	for i, progress := range got {
		if progress.Seq != int64(i+1) {
			t.Fatalf("progress[%d].Seq = %d, want %d", i, progress.Seq, i+1)
		}
	}
}
