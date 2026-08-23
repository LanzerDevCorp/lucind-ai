package ledger

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
)

func TestRegisterAndGetRun(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()
	startedAt := time.Date(2026, 8, 22, 14, 0, 0, 0, time.FixedZone("west", -7*60*60))
	want := Run{
		RunID:     "run-1",
		FeatureID: "feature-1",
		Status:    "running",
		TargetRef: "refs/heads/main",
		LaneCount: 3,
		StartedAt: startedAt,
	}

	if err := l.RegisterRun(ctx, want); err != nil {
		t.Fatalf("RegisterRun: %v", err)
	}
	got, err := l.GetRun(ctx, want.RunID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}

	if got.RunID != want.RunID || got.FeatureID != want.FeatureID || got.Status != want.Status ||
		got.TargetRef != want.TargetRef || got.LaneCount != want.LaneCount {
		t.Fatalf("GetRun = %+v, want fields from %+v", got, want)
	}
	if !got.StartedAt.Equal(startedAt) || got.StartedAt.Location() != time.UTC {
		t.Errorf("StartedAt = %v, want UTC instant %v", got.StartedAt, startedAt.UTC())
	}
	if got.EndedAt != nil {
		t.Errorf("EndedAt = %v, want nil for running run", got.EndedAt)
	}
}

func TestRegisterRunRejectsDuplicateRunID(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()
	run := Run{
		RunID:     "run-duplicate",
		FeatureID: "feature-1",
		Status:    "running",
		TargetRef: "main",
		LaneCount: 1,
		StartedAt: time.Date(2026, 8, 22, 15, 0, 0, 0, time.UTC),
	}

	if err := l.RegisterRun(ctx, run); err != nil {
		t.Fatalf("first RegisterRun: %v", err)
	}
	if err := l.RegisterRun(ctx, run); err == nil {
		t.Fatal("duplicate RegisterRun = nil error, want primary-key error")
	}

	runs, err := l.ListRuns(ctx)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("ListRuns after duplicate registration = %d rows, want 1", len(runs))
	}
}

func TestUpdateRunStatusFinishesRun(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()
	startedAt := time.Date(2026, 8, 22, 16, 0, 0, 0, time.UTC)
	endedAt := startedAt.Add(5 * time.Minute)
	if err := l.RegisterRun(ctx, Run{
		RunID:     "run-finish",
		FeatureID: "feature-1",
		Status:    "running",
		TargetRef: "main",
		LaneCount: 2,
		StartedAt: startedAt,
	}); err != nil {
		t.Fatalf("RegisterRun: %v", err)
	}

	if err := l.UpdateRunStatus(ctx, "run-finish", "done", endedAt); err != nil {
		t.Fatalf("UpdateRunStatus: %v", err)
	}
	got, err := l.GetRun(ctx, "run-finish")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Status != "done" {
		t.Errorf("Status = %q, want done", got.Status)
	}
	if got.EndedAt == nil || !got.EndedAt.Equal(endedAt) {
		t.Errorf("EndedAt = %v, want %v", got.EndedAt, endedAt)
	}
}

func TestRunMethodsReportUnknownRun(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()

	if _, err := l.GetRun(ctx, "missing"); !errors.Is(err, ErrRunUnknown) {
		t.Errorf("GetRun error = %v, want ErrRunUnknown", err)
	}
	if err := l.UpdateRunStatus(ctx, "missing", "failed", time.Now()); !errors.Is(err, ErrRunUnknown) {
		t.Errorf("UpdateRunStatus error = %v, want ErrRunUnknown", err)
	}
}

func TestListRunsOrdersNewestFirst(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()
	base := time.Date(2026, 8, 22, 17, 0, 0, 0, time.UTC)
	for _, run := range []Run{
		{RunID: "older", FeatureID: "feature-1", Status: "done", TargetRef: "main", LaneCount: 1, StartedAt: base},
		{RunID: "newer", FeatureID: "feature-2", Status: "running", TargetRef: "dev", LaneCount: 2, StartedAt: base.Add(time.Minute)},
	} {
		if err := l.RegisterRun(ctx, run); err != nil {
			t.Fatalf("RegisterRun(%s): %v", run.RunID, err)
		}
	}

	runs, err := l.ListRuns(ctx)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("ListRuns returned %d rows, want 2", len(runs))
	}
	if runs[0].RunID != "newer" || runs[1].RunID != "older" {
		t.Fatalf("ListRuns IDs = %v, want [newer older]", []string{runs[0].RunID, runs[1].RunID})
	}
}

// TestRunIDsByRecentEventOrdersNewestFirstAcrossRuns exercises the fallback
// serve.buildServerState uses to find run identity when the runs table has
// no matching row: events.id is a global autoincrement, so grouping by
// run_id and ordering by each run's own most recent event id recovers a
// "newest first" run window purely from data that is always written
// (lucind-ai run stamps every event with its run id whether or not the run
// itself is registered).
func TestRunIDsByRecentEventOrdersNewestFirstAcrossRuns(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()
	base := time.Date(2026, 8, 22, 18, 0, 0, 0, time.UTC)

	if err := l.AppendEvent(ctx, Event{RunID: "run-a", Type: EventLaneNote, Detail: "a1", At: base}); err != nil {
		t.Fatalf("AppendEvent(a1): %v", err)
	}
	if err := l.AppendEvent(ctx, Event{RunID: "run-b", Type: EventLaneNote, Detail: "b1", At: base.Add(time.Second)}); err != nil {
		t.Fatalf("AppendEvent(b1): %v", err)
	}
	if err := l.AppendEvent(ctx, Event{RunID: "run-a", Type: EventLaneNote, Detail: "a2", At: base.Add(2 * time.Second)}); err != nil {
		t.Fatalf("AppendEvent(a2): %v", err)
	}

	ids, err := l.RunIDsByRecentEvent(ctx)
	if err != nil {
		t.Fatalf("RunIDsByRecentEvent: %v", err)
	}
	if len(ids) != 2 || ids[0] != "run-a" || ids[1] != "run-b" {
		t.Fatalf("RunIDsByRecentEvent = %v, want [run-a run-b] (run-a's latest event id is the newest overall)", ids)
	}
}

// TestDistinctLaneRunIDsCoversRunsWithLanesButNoEvents guards the residual
// case RunIDsByRecentEvent cannot see: a run whose lanes were registered but
// which never (yet) produced a lifecycle event.
func TestDistinctLaneRunIDsCoversRunsWithLanesButNoEvents(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()

	if err := l.RegisterLane(ctx, Lane{
		RunID: "run-lanes-only", LaneID: "lane-a", PacketID: "packet-a",
		Executor: "agy", RoutingCondition: "primary", Status: lane.Pending,
	}); err != nil {
		t.Fatalf("RegisterLane: %v", err)
	}

	ids, err := l.DistinctLaneRunIDs(ctx)
	if err != nil {
		t.Fatalf("DistinctLaneRunIDs: %v", err)
	}
	if len(ids) != 1 || ids[0] != "run-lanes-only" {
		t.Fatalf("DistinctLaneRunIDs = %v, want [run-lanes-only]", ids)
	}
}
