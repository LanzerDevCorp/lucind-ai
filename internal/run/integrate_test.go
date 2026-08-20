package run_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/barrier"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
	"github.com/LanzerDevCorp/lucind-ai/internal/run"
)

type integrateRecorder struct {
	mu sync.Mutex

	combineCalls []struct {
		PrimaryRoot string
		RunID       string
		Branches    []string
	}
	combineFunc      func(primaryRoot, runID string, branches []string) (string, string, error)
	combineRetPath   string
	combineRetBranch string
	combineErr       error

	checkCalls     []string
	checkFunc      func(worktreePath string) (bool, string, error)
	checkRetPassed bool
	checkRetOutput string
	checkErr       error

	promoteCalls []struct {
		PrimaryRoot       string
		IntegrationBranch string
	}
	promoteFunc func(primaryRoot, integrationBranch string) error
	promoteErr  error

	discardCalls []struct {
		PrimaryRoot  string
		WorktreePath string
		BranchName   string
	}
	discardFunc func(primaryRoot, worktreePath, branchName string) error
	discardErr  error

	removeCalls []struct {
		PrimaryRoot  string
		WorktreePath string
		Branch       string
	}
	removeFunc func(primaryRoot, worktreePath, branch string) error
	removeErr  error

	persistCalls []struct {
		PrimaryRoot string
		LaneID      string
		Envelope    *result.Envelope
	}
}

func newIntegrateTestDeps(t *testing.T, rec *integrateRecorder) (run.Deps, *ledger.Ledger) {
	t.Helper()

	l, err := ledger.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("ledger.Open() error = %v", err)
	}
	t.Cleanup(func() { l.Close() })

	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)

	deps := run.Deps{
		RunID:       "run-1",
		PrimaryRoot: "/primary/root",
		Ledger:      l,
		Now:         func() time.Time { return now },
		CombineTree: func(_ context.Context, primaryRoot, runID string, branches []string) (string, string, error) {
			rec.mu.Lock()
			rec.combineCalls = append(rec.combineCalls, struct {
				PrimaryRoot string
				RunID       string
				Branches    []string
			}{PrimaryRoot: primaryRoot, RunID: runID, Branches: branches})
			if rec.combineFunc != nil {
				fn := rec.combineFunc
				rec.mu.Unlock()
				return fn(primaryRoot, runID, branches)
			}
			path := rec.combineRetPath
			branch := rec.combineRetBranch
			err := rec.combineErr
			rec.mu.Unlock()
			return path, branch, err
		},
		RunChecks: func(_ context.Context, worktreePath string) (bool, string, error) {
			rec.mu.Lock()
			rec.checkCalls = append(rec.checkCalls, worktreePath)
			if rec.checkFunc != nil {
				fn := rec.checkFunc
				rec.mu.Unlock()
				return fn(worktreePath)
			}
			passed := rec.checkRetPassed
			out := rec.checkRetOutput
			err := rec.checkErr
			rec.mu.Unlock()
			return passed, out, err
		},
		PromoteTarget: func(_ context.Context, primaryRoot, integrationBranch string) error {
			rec.mu.Lock()
			rec.promoteCalls = append(rec.promoteCalls, struct {
				PrimaryRoot       string
				IntegrationBranch string
			}{PrimaryRoot: primaryRoot, IntegrationBranch: integrationBranch})
			if rec.promoteFunc != nil {
				fn := rec.promoteFunc
				rec.mu.Unlock()
				return fn(primaryRoot, integrationBranch)
			}
			err := rec.promoteErr
			rec.mu.Unlock()
			return err
		},
		DiscardCombined: func(_ context.Context, primaryRoot, worktreePath, branchName string) error {
			rec.mu.Lock()
			rec.discardCalls = append(rec.discardCalls, struct {
				PrimaryRoot  string
				WorktreePath string
				BranchName   string
			}{PrimaryRoot: primaryRoot, WorktreePath: worktreePath, BranchName: branchName})
			if rec.discardFunc != nil {
				fn := rec.discardFunc
				rec.mu.Unlock()
				return fn(primaryRoot, worktreePath, branchName)
			}
			err := rec.discardErr
			rec.mu.Unlock()
			return err
		},
		RemoveLaneWorktree: func(_ context.Context, primaryRoot, worktreePath, branch string) error {
			rec.mu.Lock()
			rec.removeCalls = append(rec.removeCalls, struct {
				PrimaryRoot  string
				WorktreePath string
				Branch       string
			}{PrimaryRoot: primaryRoot, WorktreePath: worktreePath, Branch: branch})
			if rec.removeFunc != nil {
				fn := rec.removeFunc
				rec.mu.Unlock()
				return fn(primaryRoot, worktreePath, branch)
			}
			err := rec.removeErr
			rec.mu.Unlock()
			return err
		},
		PersistEnvelope: func(_ context.Context, primaryRoot, laneID string, envelope *result.Envelope) error {
			rec.mu.Lock()
			rec.persistCalls = append(rec.persistCalls, struct {
				PrimaryRoot string
				LaneID      string
				Envelope    *result.Envelope
			}{PrimaryRoot: primaryRoot, LaneID: laneID, Envelope: envelope})
			rec.mu.Unlock()
			return nil
		},
	}

	return deps, l
}

// registerLane registers a lane in the ledger with initial status and worktree_preserved flag.
func registerTestLane(t *testing.T, l *ledger.Ledger, runID, laneID string, st lane.Status, wtPath string, preserved bool) {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	if err := l.RegisterLane(ctx, ledger.Lane{
		RunID:            runID,
		LaneID:           laneID,
		PacketID:         laneID,
		Executor:         "agy",
		RoutingCondition: "test condition",
		Status:           st,
		WorktreePath:     wtPath,
	}); err != nil {
		t.Fatalf("RegisterLane(%s) error = %v", laneID, err)
	}
	if err := l.SetWorktreePreserved(ctx, runID, laneID, preserved); err != nil {
		t.Fatalf("SetWorktreePreserved(%s) error = %v", laneID, err)
	}
	if err := l.SetStatus(ctx, runID, laneID, st, now); err != nil {
		t.Fatalf("SetStatus(%s) error = %v", laneID, err)
	}
}

func TestIntegrateNothingToIntegrateIsNoOp(t *testing.T) {
	rec := &integrateRecorder{}
	deps, l := newIntegrateTestDeps(t, rec)

	registerTestLane(t, l, "run-1", "lane-blocked", lane.Blocked, "/wt/lane-blocked", true)

	batch := run.BatchReport{
		RunID:    "run-1",
		Released: true,
		Outcome: barrier.Outcome{
			Released:  true,
			Integrate: nil,
			Preserve:  []string{"lane-blocked"},
		},
		Lanes: []run.Report{
			{LaneID: "lane-blocked", Status: lane.Blocked, Worktree: "/wt/lane-blocked"},
		},
	}

	report, err := run.Integrate(context.Background(), deps, batch)
	if err != nil {
		t.Fatalf("Integrate() error = %v, want nil", err)
	}

	if report.RunID != "run-1" {
		t.Errorf("report.RunID = %q, want %q", report.RunID, "run-1")
	}
	if report.Attempted {
		t.Errorf("report.Attempted = true, want false")
	}
	if report.Passed {
		t.Errorf("report.Passed = true, want false")
	}
	if len(report.Integrated) != 0 {
		t.Errorf("report.Integrated = %v, want empty", report.Integrated)
	}
	if len(report.Reverted) != 0 {
		t.Errorf("report.Reverted = %v, want empty", report.Reverted)
	}

	if len(rec.combineCalls) != 0 {
		t.Errorf("CombineTree was called %d times, want 0", len(rec.combineCalls))
	}
	if len(rec.checkCalls) != 0 {
		t.Errorf("RunChecks was called %d times, want 0", len(rec.checkCalls))
	}
	if len(rec.promoteCalls) != 0 {
		t.Errorf("PromoteTarget was called %d times, want 0", len(rec.promoteCalls))
	}
	if len(rec.discardCalls) != 0 {
		t.Errorf("DiscardCombined was called %d times, want 0", len(rec.discardCalls))
	}
	if len(rec.removeCalls) != 0 {
		t.Errorf("RemoveLaneWorktree was called %d times, want 0", len(rec.removeCalls))
	}
}

func TestIntegrateUnreleasedBatchIsNoOp(t *testing.T) {
	rec := &integrateRecorder{}
	deps, _ := newIntegrateTestDeps(t, rec)

	batch := run.BatchReport{
		RunID:    "run-1",
		Released: false,
		Outcome: barrier.Outcome{
			Released:  false,
			Integrate: []string{"lane-a"},
			Preserve:  []string{"lane-b"},
		},
	}

	report, err := run.Integrate(context.Background(), deps, batch)
	if err != nil {
		t.Fatalf("Integrate() error = %v, want nil", err)
	}
	if report.Attempted {
		t.Errorf("report.Attempted = true, want false for unreleased batch")
	}
	if len(rec.combineCalls) != 0 {
		t.Errorf("CombineTree was called %d times, want 0", len(rec.combineCalls))
	}
}

func TestIntegrateGreenPath(t *testing.T) {
	rec := &integrateRecorder{
		combineRetPath:   "/wt/integrate-run-1",
		combineRetBranch: "lucind/integrate-run-1",
		checkRetPassed:   true,
		checkRetOutput:   "PASS: all tests passed",
	}
	deps, l := newIntegrateTestDeps(t, rec)

	registerTestLane(t, l, "run-1", "lane-a", lane.Done, "/wt/lane-a", true)
	registerTestLane(t, l, "run-1", "lane-b", lane.Done, "/wt/lane-b", true)

	batch := run.BatchReport{
		RunID:    "run-1",
		Released: true,
		Outcome: barrier.Outcome{
			Released:  true,
			Integrate: []string{"lane-a", "lane-b"},
		},
		Lanes: []run.Report{
			{LaneID: "lane-a", Status: lane.Done, Worktree: "/wt/lane-a"},
			{LaneID: "lane-b", Status: lane.Done, Worktree: "/wt/lane-b"},
		},
	}

	report, err := run.Integrate(context.Background(), deps, batch)
	if err != nil {
		t.Fatalf("Integrate() error = %v, want nil", err)
	}

	if !report.Attempted {
		t.Errorf("report.Attempted = false, want true")
	}
	if !report.Passed {
		t.Errorf("report.Passed = false, want true")
	}
	if len(report.Integrated) != 2 || report.Integrated[0] != "lane-a" || report.Integrated[1] != "lane-b" {
		t.Errorf("report.Integrated = %v, want [lane-a, lane-b]", report.Integrated)
	}
	if len(report.Reverted) != 0 {
		t.Errorf("report.Reverted = %v, want empty", report.Reverted)
	}

	// Assert CombineTree called with correct branches
	if len(rec.combineCalls) != 1 {
		t.Fatalf("CombineTree calls = %d, want 1", len(rec.combineCalls))
	}
	gotBranches := rec.combineCalls[0].Branches
	if len(gotBranches) != 2 || gotBranches[0] != "lucind/lane-a" || gotBranches[1] != "lucind/lane-b" {
		t.Errorf("CombineTree branches = %v, want [lucind/lane-a, lucind/lane-b]", gotBranches)
	}

	// Assert RunChecks called on combined worktree path
	if len(rec.checkCalls) != 1 || rec.checkCalls[0] != "/wt/integrate-run-1" {
		t.Errorf("RunChecks calls = %v, want [/wt/integrate-run-1]", rec.checkCalls)
	}

	// Assert PromoteTarget called with the exact branch name CombineTree returned
	if len(rec.promoteCalls) != 1 || rec.promoteCalls[0].IntegrationBranch != "lucind/integrate-run-1" {
		t.Errorf("PromoteTarget calls = %+v, want [lucind/integrate-run-1]", rec.promoteCalls)
	}

	// Assert DiscardCombined called with combined worktree path and branch
	if len(rec.discardCalls) != 1 || rec.discardCalls[0].WorktreePath != "/wt/integrate-run-1" || rec.discardCalls[0].BranchName != "lucind/integrate-run-1" {
		t.Errorf("DiscardCombined calls = %+v, want [/wt/integrate-run-1, lucind/integrate-run-1]", rec.discardCalls)
	}

	// Assert RemoveLaneWorktree called once per integrated lane with its own path and branch
	if len(rec.removeCalls) != 2 {
		t.Fatalf("RemoveLaneWorktree calls = %d, want 2", len(rec.removeCalls))
	}
	if rec.removeCalls[0].WorktreePath != "/wt/lane-a" || rec.removeCalls[0].Branch != "lucind/lane-a" {
		t.Errorf("removeCalls[0] = %+v, want /wt/lane-a, lucind/lane-a", rec.removeCalls[0])
	}
	if rec.removeCalls[1].WorktreePath != "/wt/lane-b" || rec.removeCalls[1].Branch != "lucind/lane-b" {
		t.Errorf("removeCalls[1] = %+v, want /wt/lane-b, lucind/lane-b", rec.removeCalls[1])
	}

	// Assert ledger states: status stays Done, worktree_preserved is false
	lanes, err := l.Lanes(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("Lanes() error = %v", err)
	}
	for _, ln := range lanes {
		if ln.Status != lane.Done {
			t.Errorf("lane %s status = %v, want done (never overwritten)", ln.LaneID, ln.Status)
		}
		if ln.WorktreePreserved {
			t.Errorf("lane %s worktree_preserved = true, want false", ln.LaneID)
		}
	}

	// Assert summary event
	events, err := l.Events(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	var foundSummary bool
	for _, ev := range events {
		if ev.LaneID == "" && ev.Type == ledger.EventLaneNote {
			foundSummary = true
			break
		}
	}
	if !foundSummary {
		t.Errorf("Events() did not include run-scoped EventLaneNote summary event: %+v", events)
	}
}

func TestCompleteIntegrationPersistsEnvelopeForEveryIntegratedLane(t *testing.T) {
	rec := &integrateRecorder{
		combineRetPath:   "/wt/integrate-run-1",
		combineRetBranch: "lucind/integrate-run-1",
		checkRetPassed:   true,
	}
	deps, l := newIntegrateTestDeps(t, rec)

	registerTestLane(t, l, "run-1", "lane-a", lane.Done, "/wt/lane-a", true)
	registerTestLane(t, l, "run-1", "lane-b", lane.Done, "/wt/lane-b", true)

	envA := &result.Envelope{
		PacketID: "lane-a",
		Status:   "done",
		Summary:  "lane a summary",
		Findings: []result.Finding{{
			Finding:  "finding A",
			Evidence: "file-a.go:10",
			Affects:  "verify A",
		}},
	}
	envB := &result.Envelope{
		PacketID: "lane-b",
		Status:   "done",
		Summary:  "lane b summary",
		Findings: []result.Finding{{
			Finding:  "finding B",
			Evidence: "file-b.go:20",
			Affects:  "verify B",
		}},
	}

	batch := run.BatchReport{
		RunID:    "run-1",
		Released: true,
		Outcome: barrier.Outcome{
			Released:  true,
			Integrate: []string{"lane-a", "lane-b"},
		},
		Lanes: []run.Report{
			{LaneID: "lane-a", Status: lane.Done, Worktree: "/wt/lane-a", Envelope: envA},
			{LaneID: "lane-b", Status: lane.Done, Worktree: "/wt/lane-b", Envelope: envB},
		},
	}

	report, err := run.Integrate(context.Background(), deps, batch)
	if err != nil {
		t.Fatalf("Integrate() error = %v, want nil", err)
	}
	if len(report.Integrated) != 2 || report.Integrated[0] != "lane-a" || report.Integrated[1] != "lane-b" {
		t.Fatalf("report.Integrated = %v, want [lane-a, lane-b]", report.Integrated)
	}

	if len(rec.persistCalls) != 2 {
		t.Fatalf("PersistEnvelope calls = %d, want 2 (once per integrated lane)", len(rec.persistCalls))
	}
	if rec.persistCalls[0].LaneID != "lane-a" || rec.persistCalls[0].Envelope != envA {
		t.Errorf("persistCalls[0] LaneID=%s Envelope=%p, want lane-a and envA (%p)", rec.persistCalls[0].LaneID, rec.persistCalls[0].Envelope, envA)
	}
	if rec.persistCalls[1].LaneID != "lane-b" || rec.persistCalls[1].Envelope != envB {
		t.Errorf("persistCalls[1] LaneID=%s Envelope=%p, want lane-b and envB (%p)", rec.persistCalls[1].LaneID, rec.persistCalls[1].Envelope, envB)
	}
	if rec.persistCalls[0].Envelope == nil || rec.persistCalls[1].Envelope == nil {
		t.Errorf("PersistEnvelope received a nil envelope")
	}
	if rec.persistCalls[0].Envelope == envB {
		t.Errorf("persistCalls[0] received lane-b's envelope, want lane-a's")
	}
	if rec.persistCalls[1].Envelope == envA {
		t.Errorf("persistCalls[1] received lane-a's envelope, want lane-b's")
	}

	if len(rec.removeCalls) != len(rec.persistCalls) {
		t.Errorf("PersistEnvelope calls = %d, RemoveLaneWorktree calls = %d, want one-for-one with integrateIDs", len(rec.persistCalls), len(rec.removeCalls))
	}
	for i, id := range report.Integrated {
		if rec.persistCalls[i].LaneID != id {
			t.Errorf("persistCalls[%d].LaneID = %s, want %s (integrateIDs order)", i, rec.persistCalls[i].LaneID, id)
		}
	}
}

func TestIntegrateMergeConflictRedPath(t *testing.T) {
	rec := &integrateRecorder{
		combineErr: errors.New("integrate: merge conflict: branch lucind/lane-b: CONFLICT in foo.go"),
	}
	deps, l := newIntegrateTestDeps(t, rec)

	registerTestLane(t, l, "run-1", "lane-a", lane.Done, "/wt/lane-a", false)
	registerTestLane(t, l, "run-1", "lane-b", lane.Done, "/wt/lane-b", false)

	batch := run.BatchReport{
		RunID:    "run-1",
		Released: true,
		Outcome: barrier.Outcome{
			Released:  true,
			Integrate: []string{"lane-a", "lane-b"},
		},
		Lanes: []run.Report{
			{LaneID: "lane-a", Status: lane.Done, Worktree: "/wt/lane-a"},
			{LaneID: "lane-b", Status: lane.Done, Worktree: "/wt/lane-b"},
		},
	}

	report, err := run.Integrate(context.Background(), deps, batch)
	if err != nil {
		t.Fatalf("Integrate() error = %v, want nil", err)
	}

	if !report.Attempted {
		t.Errorf("report.Attempted = false, want true")
	}
	if report.Passed {
		t.Errorf("report.Passed = true, want false")
	}
	if len(report.Integrated) != 0 {
		t.Errorf("report.Integrated = %v, want empty", report.Integrated)
	}
	if len(report.Reverted) != 2 || report.Reverted[0] != "lane-a" || report.Reverted[1] != "lane-b" {
		t.Errorf("report.Reverted = %v, want [lane-a, lane-b]", report.Reverted)
	}
	wantErr := "bisection found no viable subset: " + rec.combineErr.Error()
	if report.Reason != wantErr {
		t.Errorf("report.Reason = %q, want %q", report.Reason, wantErr)
	}

	// Assert RunChecks and PromoteTarget are never called
	if len(rec.checkCalls) != 0 {
		t.Errorf("RunChecks called %d times, want 0", len(rec.checkCalls))
	}
	if len(rec.promoteCalls) != 0 {
		t.Errorf("PromoteTarget called %d times, want 0", len(rec.promoteCalls))
	}
	if len(rec.removeCalls) != 0 {
		t.Errorf("RemoveLaneWorktree called %d times, want 0", len(rec.removeCalls))
	}

	// Assert every offered lane is now lane.Blocked with worktree_preserved = true in the ledger
	lanes, err := l.Lanes(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("Lanes() error = %v", err)
	}
	for _, ln := range lanes {
		if ln.Status != lane.Blocked {
			t.Errorf("lane %s status = %v, want blocked", ln.LaneID, ln.Status)
		}
		if !ln.WorktreePreserved {
			t.Errorf("lane %s worktree_preserved = false, want true", ln.LaneID)
		}
	}

	// Assert per-lane and run-scoped notes in events
	events, err := l.Events(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	laneNotes := map[string]string{}
	var runSummary string
	for _, ev := range events {
		if ev.Type == ledger.EventLaneNote {
			if ev.LaneID == "" {
				runSummary = ev.Detail
			} else {
				laneNotes[ev.LaneID] = ev.Detail
			}
		}
	}
	if laneNotes["lane-a"] != wantErr {
		t.Errorf("lane-a note = %q, want %q", laneNotes["lane-a"], wantErr)
	}
	if laneNotes["lane-b"] != wantErr {
		t.Errorf("lane-b note = %q, want %q", laneNotes["lane-b"], wantErr)
	}
	if runSummary == "" {
		t.Errorf("run-scoped summary event missing from events: %+v", events)
	}
}

func TestIntegrateFailingChecksRedPath(t *testing.T) {
	rec := &integrateRecorder{
		combineRetPath:   "/wt/integrate-run-1",
		combineRetBranch: "lucind/integrate-run-1",
		checkRetPassed:   false,
		checkRetOutput:   "--- FAIL: TestFoo (0.01s)\n    foo_test.go:42: failure",
	}
	deps, l := newIntegrateTestDeps(t, rec)

	registerTestLane(t, l, "run-1", "lane-a", lane.Done, "/wt/lane-a", false)
	registerTestLane(t, l, "run-1", "lane-b", lane.Done, "/wt/lane-b", false)

	batch := run.BatchReport{
		RunID:    "run-1",
		Released: true,
		Outcome: barrier.Outcome{
			Released:  true,
			Integrate: []string{"lane-a", "lane-b"},
		},
		Lanes: []run.Report{
			{LaneID: "lane-a", Status: lane.Done, Worktree: "/wt/lane-a"},
			{LaneID: "lane-b", Status: lane.Done, Worktree: "/wt/lane-b"},
		},
	}

	report, err := run.Integrate(context.Background(), deps, batch)
	if err != nil {
		t.Fatalf("Integrate() error = %v, want nil", err)
	}

	if !report.Attempted {
		t.Errorf("report.Attempted = false, want true")
	}
	if report.Passed {
		t.Errorf("report.Passed = true, want false")
	}
	if len(report.Integrated) != 0 {
		t.Errorf("report.Integrated = %v, want empty", report.Integrated)
	}
	if len(report.Reverted) != 2 || report.Reverted[0] != "lane-a" || report.Reverted[1] != "lane-b" {
		t.Errorf("report.Reverted = %v, want [lane-a, lane-b]", report.Reverted)
	}
	wantErr := "bisection found no viable subset: " + rec.checkRetOutput
	if report.Reason != wantErr {
		t.Errorf("report.Reason = %q, want %q", report.Reason, wantErr)
	}

	// Assert DiscardCombined was called for the full batch and each bisection attempt (3 total)
	if len(rec.discardCalls) != 3 || rec.discardCalls[0].WorktreePath != "/wt/integrate-run-1" || rec.discardCalls[0].BranchName != "lucind/integrate-run-1" {
		t.Errorf("DiscardCombined calls = %+v, want 3 calls with [/wt/integrate-run-1, lucind/integrate-run-1]", rec.discardCalls)
	}

	// Assert PromoteTarget and RemoveLaneWorktree are never called
	if len(rec.promoteCalls) != 0 {
		t.Errorf("PromoteTarget called %d times, want 0", len(rec.promoteCalls))
	}
	if len(rec.removeCalls) != 0 {
		t.Errorf("RemoveLaneWorktree called %d times, want 0", len(rec.removeCalls))
	}

	// Assert every offered lane reverts
	lanes, err := l.Lanes(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("Lanes() error = %v", err)
	}
	for _, ln := range lanes {
		if ln.Status != lane.Blocked {
			t.Errorf("lane %s status = %v, want blocked", ln.LaneID, ln.Status)
		}
		if !ln.WorktreePreserved {
			t.Errorf("lane %s worktree_preserved = false, want true", ln.LaneID)
		}
	}
}

func TestIntegrateChecksExecutionErrorRedPath(t *testing.T) {
	rec := &integrateRecorder{
		combineRetPath:   "/wt/integrate-run-1",
		combineRetBranch: "lucind/integrate-run-1",
		checkErr:         errors.New("integrate: go test failed to execute: exec: not found"),
	}
	deps, l := newIntegrateTestDeps(t, rec)

	registerTestLane(t, l, "run-1", "lane-a", lane.Done, "/wt/lane-a", false)

	batch := run.BatchReport{
		RunID:    "run-1",
		Released: true,
		Outcome: barrier.Outcome{
			Released:  true,
			Integrate: []string{"lane-a"},
		},
		Lanes: []run.Report{
			{LaneID: "lane-a", Status: lane.Done, Worktree: "/wt/lane-a"},
		},
	}

	report, err := run.Integrate(context.Background(), deps, batch)
	if err != nil {
		t.Fatalf("Integrate() error = %v, want nil", err)
	}

	if report.Passed {
		t.Errorf("report.Passed = true, want false")
	}
	wantErr := "bisection found no viable subset: " + rec.checkErr.Error()
	if report.Reason != wantErr {
		t.Errorf("report.Reason = %q, want %q", report.Reason, wantErr)
	}
	if len(rec.discardCalls) != 1 {
		t.Errorf("DiscardCombined called %d times, want 1", len(rec.discardCalls))
	}
}

func TestIntegratePromoteFailureRedPath(t *testing.T) {
	rec := &integrateRecorder{
		combineRetPath:   "/wt/integrate-run-1",
		combineRetBranch: "lucind/integrate-run-1",
		checkRetPassed:   true,
		checkRetOutput:   "PASS",
		promoteErr:       errors.New("integrate: primary root has uncommitted changes"),
	}
	deps, l := newIntegrateTestDeps(t, rec)

	registerTestLane(t, l, "run-1", "lane-a", lane.Done, "/wt/lane-a", false)
	registerTestLane(t, l, "run-1", "lane-b", lane.Done, "/wt/lane-b", false)

	batch := run.BatchReport{
		RunID:    "run-1",
		Released: true,
		Outcome: barrier.Outcome{
			Released:  true,
			Integrate: []string{"lane-a", "lane-b"},
		},
		Lanes: []run.Report{
			{LaneID: "lane-a", Status: lane.Done, Worktree: "/wt/lane-a"},
			{LaneID: "lane-b", Status: lane.Done, Worktree: "/wt/lane-b"},
		},
	}

	report, err := run.Integrate(context.Background(), deps, batch)
	if err != nil {
		t.Fatalf("Integrate() error = %v, want nil", err)
	}

	if !report.Attempted {
		t.Errorf("report.Attempted = false, want true")
	}
	if report.Passed {
		t.Errorf("report.Passed = true, want false")
	}
	if len(report.Integrated) != 0 {
		t.Errorf("report.Integrated = %v, want empty", report.Integrated)
	}
	if len(report.Reverted) != 2 || report.Reverted[0] != "lane-a" || report.Reverted[1] != "lane-b" {
		t.Errorf("report.Reverted = %v, want [lane-a, lane-b]", report.Reverted)
	}
	if report.Reason != rec.promoteErr.Error() {
		t.Errorf("report.Reason = %q, want %q", report.Reason, rec.promoteErr.Error())
	}

	// Assert DiscardCombined was called
	if len(rec.discardCalls) != 1 || rec.discardCalls[0].WorktreePath != "/wt/integrate-run-1" || rec.discardCalls[0].BranchName != "lucind/integrate-run-1" {
		t.Errorf("DiscardCombined calls = %+v, want [/wt/integrate-run-1, lucind/integrate-run-1]", rec.discardCalls)
	}

	// Assert RemoveLaneWorktree was never called
	if len(rec.removeCalls) != 0 {
		t.Errorf("RemoveLaneWorktree called %d times, want 0", len(rec.removeCalls))
	}

	// Assert every offered lane reverts in the ledger
	lanes, err := l.Lanes(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("Lanes() error = %v", err)
	}
	for _, ln := range lanes {
		if ln.Status != lane.Blocked {
			t.Errorf("lane %s status = %v, want blocked", ln.LaneID, ln.Status)
		}
		if !ln.WorktreePreserved {
			t.Errorf("lane %s worktree_preserved = false, want true", ln.LaneID)
		}
	}
}

func TestIntegratePartialBatchNeverTouchesPreservedLanes(t *testing.T) {
	rec := &integrateRecorder{
		combineRetPath:   "/wt/integrate-run-1",
		combineRetBranch: "lucind/integrate-run-1",
		checkRetPassed:   true,
		checkRetOutput:   "PASS",
	}
	deps, l := newIntegrateTestDeps(t, rec)

	registerTestLane(t, l, "run-1", "lane-done", lane.Done, "/wt/lane-done", true)
	registerTestLane(t, l, "run-1", "lane-blocked", lane.Blocked, "/wt/lane-blocked", true)

	batch := run.BatchReport{
		RunID:    "run-1",
		Released: true,
		Outcome: barrier.Outcome{
			Released:  true,
			Integrate: []string{"lane-done"},
			Preserve:  []string{"lane-blocked"},
		},
		Lanes: []run.Report{
			{LaneID: "lane-done", Status: lane.Done, Worktree: "/wt/lane-done"},
			{LaneID: "lane-blocked", Status: lane.Blocked, Worktree: "/wt/lane-blocked"},
		},
	}

	report, err := run.Integrate(context.Background(), deps, batch)
	if err != nil {
		t.Fatalf("Integrate() error = %v, want nil", err)
	}

	if !report.Passed {
		t.Errorf("report.Passed = false, want true")
	}
	if len(report.Integrated) != 1 || report.Integrated[0] != "lane-done" {
		t.Errorf("report.Integrated = %v, want [lane-done]", report.Integrated)
	}

	// Assert branches passed to CombineTree names only the done lane
	if len(rec.combineCalls) != 1 {
		t.Fatalf("CombineTree calls = %d, want 1", len(rec.combineCalls))
	}
	if len(rec.combineCalls[0].Branches) != 1 || rec.combineCalls[0].Branches[0] != "lucind/lane-done" {
		t.Errorf("CombineTree branches = %v, want [lucind/lane-done]", rec.combineCalls[0].Branches)
	}

	// Assert RemoveLaneWorktree called only for lane-done
	if len(rec.removeCalls) != 1 || rec.removeCalls[0].WorktreePath != "/wt/lane-done" || rec.removeCalls[0].Branch != "lucind/lane-done" {
		t.Errorf("RemoveLaneWorktree calls = %+v, want [/wt/lane-done, lucind/lane-done]", rec.removeCalls)
	}

	// Assert blocked lane's status/worktree-preserved are untouched
	lanes, err := l.Lanes(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("Lanes() error = %v", err)
	}
	byID := map[string]ledger.Lane{}
	for _, ln := range lanes {
		byID[ln.LaneID] = ln
	}

	if byID["lane-blocked"].Status != lane.Blocked {
		t.Errorf("lane-blocked status = %v, want blocked (untouched)", byID["lane-blocked"].Status)
	}
	if !byID["lane-blocked"].WorktreePreserved {
		t.Errorf("lane-blocked worktree_preserved = false, want true (untouched)")
	}

	if byID["lane-done"].Status != lane.Done {
		t.Errorf("lane-done status = %v, want done", byID["lane-done"].Status)
	}
	if byID["lane-done"].WorktreePreserved {
		t.Errorf("lane-done worktree_preserved = true, want false (cleaned up)")
	}
}

func TestBisectPureInteractionFallback(t *testing.T) {
	rec := &integrateRecorder{
		combineRetPath:   "/wt/test",
		combineRetBranch: "lucind/test",
		checkRetPassed:   true,
		checkRetOutput:   "PASS",
	}
	deps, _ := newIntegrateTestDeps(t, rec)

	intLanes, revLanes := run.Bisect(context.Background(), deps, []string{"lane-a", "lane-b"})
	if len(intLanes) != 0 {
		t.Errorf("integrate lanes = %v, want nil", intLanes)
	}
	if len(revLanes) != 2 || revLanes[0] != "lane-a" || revLanes[1] != "lane-b" {
		t.Errorf("revert lanes = %v, want [lane-a, lane-b]", revLanes)
	}
}

func TestBisectSingleCulpritIsolation(t *testing.T) {
	rec := &integrateRecorder{
		combineRetPath:   "/wt/test",
		combineRetBranch: "lucind/test",
	}
	rec.checkFunc = func(worktreePath string) (bool, string, error) {
		lastCall := rec.combineCalls[len(rec.combineCalls)-1]
		for _, b := range lastCall.Branches {
			if b == "lucind/lane-b" {
				return false, "FAIL: lane-b broken", nil
			}
		}
		return true, "PASS", nil
	}
	deps, _ := newIntegrateTestDeps(t, rec)

	intLanes, revLanes := run.Bisect(context.Background(), deps, []string{"lane-a", "lane-b", "lane-c"})
	if len(intLanes) != 2 || intLanes[0] != "lane-a" || intLanes[1] != "lane-c" {
		t.Errorf("integrate lanes = %v, want [lane-a, lane-c]", intLanes)
	}
	if len(revLanes) != 1 || revLanes[0] != "lane-b" {
		t.Errorf("revert lanes = %v, want [lane-b]", revLanes)
	}
}

func TestBisectRightGreenLeftRed(t *testing.T) {
	rec := &integrateRecorder{
		combineRetPath:   "/wt/test",
		combineRetBranch: "lucind/test",
	}
	rec.checkFunc = func(worktreePath string) (bool, string, error) {
		lastCall := rec.combineCalls[len(rec.combineCalls)-1]
		for _, b := range lastCall.Branches {
			if b == "lucind/lane-a" {
				return false, "FAIL: lane-a broken", nil
			}
		}
		return true, "PASS", nil
	}
	deps, _ := newIntegrateTestDeps(t, rec)

	intLanes, revLanes := run.Bisect(context.Background(), deps, []string{"lane-a", "lane-b", "lane-c"})
	if len(intLanes) != 2 || intLanes[0] != "lane-b" || intLanes[1] != "lane-c" {
		t.Errorf("integrate lanes = %v, want [lane-b, lane-c]", intLanes)
	}
	if len(revLanes) != 1 || revLanes[0] != "lane-a" {
		t.Errorf("revert lanes = %v, want [lane-a]", revLanes)
	}
}

func TestBisectBothHalvesRedUnionGreen(t *testing.T) {
	rec := &integrateRecorder{
		combineRetPath:   "/wt/test",
		combineRetBranch: "lucind/test",
	}
	rec.checkFunc = func(worktreePath string) (bool, string, error) {
		lastCall := rec.combineCalls[len(rec.combineCalls)-1]
		for _, b := range lastCall.Branches {
			if b == "lucind/lane-b" || b == "lucind/lane-c" {
				return false, "FAIL", nil
			}
		}
		return true, "PASS", nil
	}
	deps, _ := newIntegrateTestDeps(t, rec)

	intLanes, revLanes := run.Bisect(context.Background(), deps, []string{"lane-a", "lane-b", "lane-c", "lane-d"})
	if len(intLanes) != 2 || intLanes[0] != "lane-a" || intLanes[1] != "lane-d" {
		t.Errorf("integrate lanes = %v, want [lane-a, lane-d]", intLanes)
	}
	if len(revLanes) != 2 || revLanes[0] != "lane-b" || revLanes[1] != "lane-c" {
		t.Errorf("revert lanes = %v, want [lane-b, lane-c]", revLanes)
	}
}

func TestBisectBothHalvesRedUnionRed(t *testing.T) {
	rec := &integrateRecorder{
		combineRetPath:   "/wt/test",
		combineRetBranch: "lucind/test",
	}
	rec.checkFunc = func(worktreePath string) (bool, string, error) {
		lastCall := rec.combineCalls[len(rec.combineCalls)-1]
		hasA := false
		hasD := false
		for _, b := range lastCall.Branches {
			if b == "lucind/lane-b" || b == "lucind/lane-c" {
				return false, "FAIL: bad single lane", nil
			}
			if b == "lucind/lane-a" {
				hasA = true
			}
			if b == "lucind/lane-d" {
				hasD = true
			}
		}
		if hasA && hasD {
			return false, "FAIL: lane-a and lane-d interaction", nil
		}
		return true, "PASS", nil
	}
	deps, _ := newIntegrateTestDeps(t, rec)

	intLanes, revLanes := run.Bisect(context.Background(), deps, []string{"lane-a", "lane-b", "lane-c", "lane-d"})
	if len(intLanes) != 0 {
		t.Errorf("integrate lanes = %v, want nil", intLanes)
	}
	if len(revLanes) != 4 {
		t.Errorf("revert lanes = %v, want 4 lanes", revLanes)
	}
}

func TestIntegratePromotesBisectedSubsetAndRevertsRest(t *testing.T) {
	rec := &integrateRecorder{
		combineRetPath:   "/wt/integrate-run-1",
		combineRetBranch: "lucind/integrate-run-1",
	}
	rec.checkFunc = func(worktreePath string) (bool, string, error) {
		lastCall := rec.combineCalls[len(rec.combineCalls)-1]
		for _, b := range lastCall.Branches {
			if b == "lucind/lane-b" {
				return false, "FAIL: lane-b fails", nil
			}
		}
		return true, "PASS", nil
	}
	deps, l := newIntegrateTestDeps(t, rec)

	registerTestLane(t, l, "run-1", "lane-a", lane.Done, "/wt/lane-a", true)
	registerTestLane(t, l, "run-1", "lane-b", lane.Done, "/wt/lane-b", true)
	registerTestLane(t, l, "run-1", "lane-c", lane.Done, "/wt/lane-c", true)

	batch := run.BatchReport{
		RunID:    "run-1",
		Released: true,
		Outcome: barrier.Outcome{
			Released:  true,
			Integrate: []string{"lane-a", "lane-b", "lane-c"},
		},
		Lanes: []run.Report{
			{LaneID: "lane-a", Status: lane.Done, Worktree: "/wt/lane-a"},
			{LaneID: "lane-b", Status: lane.Done, Worktree: "/wt/lane-b"},
			{LaneID: "lane-c", Status: lane.Done, Worktree: "/wt/lane-c"},
		},
	}

	report, err := run.Integrate(context.Background(), deps, batch)
	if err != nil {
		t.Fatalf("Integrate() error = %v", err)
	}

	if !report.Attempted {
		t.Errorf("report.Attempted = false, want true")
	}
	if !report.Passed {
		t.Errorf("report.Passed = false, want true")
	}

	// Assert partition
	if len(report.Integrated) != 2 || report.Integrated[0] != "lane-a" || report.Integrated[1] != "lane-c" {
		t.Errorf("report.Integrated = %v, want [lane-a, lane-c]", report.Integrated)
	}
	if len(report.Reverted) != 1 || report.Reverted[0] != "lane-b" {
		t.Errorf("report.Reverted = %v, want [lane-b]", report.Reverted)
	}

	// Assert only integrated lanes had their worktrees removed
	if len(rec.removeCalls) != 2 {
		t.Fatalf("RemoveLaneWorktree calls = %d, want 2", len(rec.removeCalls))
	}
	if rec.removeCalls[0].WorktreePath != "/wt/lane-a" || rec.removeCalls[0].Branch != "lucind/lane-a" {
		t.Errorf("removeCalls[0] = %+v, want /wt/lane-a, lucind/lane-a", rec.removeCalls[0])
	}
	if rec.removeCalls[1].WorktreePath != "/wt/lane-c" || rec.removeCalls[1].Branch != "lucind/lane-c" {
		t.Errorf("removeCalls[1] = %+v, want /wt/lane-c, lucind/lane-c", rec.removeCalls[1])
	}

	// Assert ledger states
	lanes, err := l.Lanes(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("Lanes() error = %v", err)
	}
	byID := map[string]ledger.Lane{}
	for _, ln := range lanes {
		byID[ln.LaneID] = ln
	}

	if byID["lane-a"].Status != lane.Done || byID["lane-a"].WorktreePreserved {
		t.Errorf("lane-a = %+v, want Done and WorktreePreserved=false", byID["lane-a"])
	}
	if byID["lane-c"].Status != lane.Done || byID["lane-c"].WorktreePreserved {
		t.Errorf("lane-c = %+v, want Done and WorktreePreserved=false", byID["lane-c"])
	}
	if byID["lane-b"].Status != lane.Blocked || !byID["lane-b"].WorktreePreserved {
		t.Errorf("lane-b = %+v, want Blocked and WorktreePreserved=true", byID["lane-b"])
	}
}

func TestIntegrateBisectionFindsNothingFullyReverts(t *testing.T) {
	rec := &integrateRecorder{
		combineRetPath:   "/wt/integrate-run-1",
		combineRetBranch: "lucind/integrate-run-1",
		checkRetPassed:   false,
		checkRetOutput:   "FAIL: everything fails",
	}
	deps, l := newIntegrateTestDeps(t, rec)

	registerTestLane(t, l, "run-1", "lane-a", lane.Done, "/wt/lane-a", false)
	registerTestLane(t, l, "run-1", "lane-b", lane.Done, "/wt/lane-b", false)

	batch := run.BatchReport{
		RunID:    "run-1",
		Released: true,
		Outcome: barrier.Outcome{
			Released:  true,
			Integrate: []string{"lane-a", "lane-b"},
		},
		Lanes: []run.Report{
			{LaneID: "lane-a", Status: lane.Done, Worktree: "/wt/lane-a"},
			{LaneID: "lane-b", Status: lane.Done, Worktree: "/wt/lane-b"},
		},
	}

	report, err := run.Integrate(context.Background(), deps, batch)
	if err != nil {
		t.Fatalf("Integrate() error = %v", err)
	}

	if report.Passed {
		t.Errorf("report.Passed = true, want false")
	}
	if len(report.Integrated) != 0 {
		t.Errorf("report.Integrated = %v, want empty", report.Integrated)
	}
	if len(report.Reverted) != 2 || report.Reverted[0] != "lane-a" || report.Reverted[1] != "lane-b" {
		t.Errorf("report.Reverted = %v, want [lane-a, lane-b]", report.Reverted)
	}

	lanes, err := l.Lanes(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("Lanes() error = %v", err)
	}
	for _, ln := range lanes {
		if ln.Status != lane.Blocked {
			t.Errorf("lane %s status = %v, want Blocked", ln.LaneID, ln.Status)
		}
		if !ln.WorktreePreserved {
			t.Errorf("lane %s WorktreePreserved = false, want true", ln.LaneID)
		}
	}
}

func TestIntegratePromoteFailureDoesNotTriggerBisection(t *testing.T) {
	rec := &integrateRecorder{
		combineRetPath:   "/wt/integrate-run-1",
		combineRetBranch: "lucind/integrate-run-1",
		checkRetPassed:   true,
		checkRetOutput:   "PASS",
		promoteErr:       errors.New("promote failed: dirty tree"),
	}
	deps, l := newIntegrateTestDeps(t, rec)

	registerTestLane(t, l, "run-1", "lane-a", lane.Done, "/wt/lane-a", false)
	registerTestLane(t, l, "run-1", "lane-b", lane.Done, "/wt/lane-b", false)

	batch := run.BatchReport{
		RunID:    "run-1",
		Released: true,
		Outcome: barrier.Outcome{
			Released:  true,
			Integrate: []string{"lane-a", "lane-b"},
		},
		Lanes: []run.Report{
			{LaneID: "lane-a", Status: lane.Done, Worktree: "/wt/lane-a"},
			{LaneID: "lane-b", Status: lane.Done, Worktree: "/wt/lane-b"},
		},
	}

	report, err := run.Integrate(context.Background(), deps, batch)
	if err != nil {
		t.Fatalf("Integrate() error = %v", err)
	}

	if report.Passed {
		t.Errorf("report.Passed = true, want false")
	}
	if len(rec.combineCalls) != 1 {
		t.Errorf("CombineTree calls = %d, want 1 (bisection must not be triggered)", len(rec.combineCalls))
	}
	if len(report.Integrated) != 0 {
		t.Errorf("report.Integrated = %v, want empty", report.Integrated)
	}
	if len(report.Reverted) != 2 || report.Reverted[0] != "lane-a" || report.Reverted[1] != "lane-b" {
		t.Errorf("report.Reverted = %v, want [lane-a, lane-b]", report.Reverted)
	}
}

func TestIntegrateFinalReverifyFallback(t *testing.T) {
	rec := &integrateRecorder{
		combineRetPath:   "/wt/integrate-run-1",
		combineRetBranch: "lucind/integrate-run-1",
		promoteFunc: func(primaryRoot, integrationBranch string) error {
			return errors.New("promote failed on reverify")
		},
	}
	rec.checkFunc = func(worktreePath string) (bool, string, error) {
		lastCall := rec.combineCalls[len(rec.combineCalls)-1]
		for _, b := range lastCall.Branches {
			if b == "lucind/lane-b" {
				return false, "FAIL: lane-b fails", nil
			}
		}
		return true, "PASS", nil
	}
	deps, l := newIntegrateTestDeps(t, rec)

	registerTestLane(t, l, "run-1", "lane-a", lane.Done, "/wt/lane-a", false)
	registerTestLane(t, l, "run-1", "lane-b", lane.Done, "/wt/lane-b", false)
	registerTestLane(t, l, "run-1", "lane-c", lane.Done, "/wt/lane-c", false)

	batch := run.BatchReport{
		RunID:    "run-1",
		Released: true,
		Outcome: barrier.Outcome{
			Released:  true,
			Integrate: []string{"lane-a", "lane-b", "lane-c"},
		},
		Lanes: []run.Report{
			{LaneID: "lane-a", Status: lane.Done, Worktree: "/wt/lane-a"},
			{LaneID: "lane-b", Status: lane.Done, Worktree: "/wt/lane-b"},
			{LaneID: "lane-c", Status: lane.Done, Worktree: "/wt/lane-c"},
		},
	}

	report, err := run.Integrate(context.Background(), deps, batch)
	if err != nil {
		t.Fatalf("Integrate() error = %v", err)
	}

	if report.Passed {
		t.Errorf("report.Passed = true, want false")
	}
	if len(report.Integrated) != 0 {
		t.Errorf("report.Integrated = %v, want empty", report.Integrated)
	}
	// All original lanes in batch.Outcome.Integrate must be reverted
	if len(report.Reverted) != 3 || report.Reverted[0] != "lane-a" || report.Reverted[1] != "lane-b" || report.Reverted[2] != "lane-c" {
		t.Errorf("report.Reverted = %v, want [lane-a, lane-b, lane-c]", report.Reverted)
	}

	lanes, err := l.Lanes(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("Lanes() error = %v", err)
	}
	for _, ln := range lanes {
		if ln.Status != lane.Blocked {
			t.Errorf("lane %s status = %v, want Blocked", ln.LaneID, ln.Status)
		}
		if !ln.WorktreePreserved {
			t.Errorf("lane %s WorktreePreserved = false, want true", ln.LaneID)
		}
	}
}

func TestTryCombineDiscardsWorktree(t *testing.T) {
	t.Run("on success", func(t *testing.T) {
		rec := &integrateRecorder{
			combineRetPath:   "/wt/try-combine-1",
			combineRetBranch: "lucind/try-combine-1",
			checkRetPassed:   true,
		}
		deps, _ := newIntegrateTestDeps(t, rec)
		ok := run.TryCombine(context.Background(), deps, []string{"lane-a"})
		if !ok {
			t.Errorf("TryCombine() = false, want true")
		}
		if len(rec.discardCalls) != 1 || rec.discardCalls[0].WorktreePath != "/wt/try-combine-1" {
			t.Errorf("DiscardCombined calls = %+v, want 1 call for /wt/try-combine-1", rec.discardCalls)
		}
	})

	t.Run("on check failure", func(t *testing.T) {
		rec := &integrateRecorder{
			combineRetPath:   "/wt/try-combine-2",
			combineRetBranch: "lucind/try-combine-2",
			checkRetPassed:   false,
		}
		deps, _ := newIntegrateTestDeps(t, rec)
		ok := run.TryCombine(context.Background(), deps, []string{"lane-a"})
		if ok {
			t.Errorf("TryCombine() = true, want false")
		}
		if len(rec.discardCalls) != 1 || rec.discardCalls[0].WorktreePath != "/wt/try-combine-2" {
			t.Errorf("DiscardCombined calls = %+v, want 1 call for /wt/try-combine-2", rec.discardCalls)
		}
	})

	t.Run("on combine failure", func(t *testing.T) {
		rec := &integrateRecorder{
			combineErr: errors.New("combine error"),
		}
		deps, _ := newIntegrateTestDeps(t, rec)
		ok := run.TryCombine(context.Background(), deps, []string{"lane-a"})
		if ok {
			t.Errorf("TryCombine() = true, want false")
		}
		if len(rec.discardCalls) != 0 {
			t.Errorf("DiscardCombined calls = %+v, want 0 calls on combine error", rec.discardCalls)
		}
	})
}

// TestIntegratePassedDoneLanesBranchesPassedToCombineTreeWithoutReadOnlyFilter
// verifies that Integrate passes every released Done lane's branch to
// CombineTree without filtering out read-only lanes: a read-only lane that
// passed completion mode has zero unique commits, so merging its branch is
// already a correct no-op (Design Decision 3).
func TestIntegratePassedDoneLanesBranchesPassedToCombineTreeWithoutReadOnlyFilter(t *testing.T) {
	rec := &integrateRecorder{
		combineRetPath:   "/wt/integrate-run-1",
		combineRetBranch: "lucind/integrate-run-1",
		checkRetPassed:   true,
		checkRetOutput:   "PASS: all tests passed",
	}
	deps, l := newIntegrateTestDeps(t, rec)

	registerTestLane(t, l, "run-1", "lane-write", lane.Done, "/wt/lane-write", true)
	registerTestLane(t, l, "run-1", "lane-readonly", lane.Done, "/wt/lane-readonly", true)

	batch := run.BatchReport{
		RunID:    "run-1",
		Released: true,
		Outcome: barrier.Outcome{
			Released:  true,
			Integrate: []string{"lane-write", "lane-readonly"},
		},
		Lanes: []run.Report{
			{LaneID: "lane-write", Status: lane.Done, Worktree: "/wt/lane-write"},
			{LaneID: "lane-readonly", Status: lane.Done, Worktree: "/wt/lane-readonly"},
		},
	}

	report, err := run.Integrate(context.Background(), deps, batch)
	if err != nil {
		t.Fatalf("Integrate() error = %v, want nil", err)
	}

	if !report.Passed {
		t.Errorf("report.Passed = false, want true")
	}

	if len(rec.combineCalls) != 1 {
		t.Fatalf("CombineTree calls = %d, want 1", len(rec.combineCalls))
	}
	gotBranches := rec.combineCalls[0].Branches
	if len(gotBranches) != 2 || gotBranches[0] != "lucind/lane-write" || gotBranches[1] != "lucind/lane-readonly" {
		t.Errorf("CombineTree branches = %v, want [lucind/lane-write, lucind/lane-readonly]", gotBranches)
	}
}
