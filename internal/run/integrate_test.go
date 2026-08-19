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
	"github.com/LanzerDevCorp/lucind-ai/internal/run"
)

type integrateRecorder struct {
	mu sync.Mutex

	combineCalls []struct {
		PrimaryRoot string
		RunID       string
		Branches    []string
	}
	combineRetPath   string
	combineRetBranch string
	combineErr       error

	checkCalls     []string
	checkRetPassed bool
	checkRetOutput string
	checkErr       error

	promoteCalls []struct {
		PrimaryRoot       string
		IntegrationBranch string
	}
	promoteErr error

	discardCalls []struct {
		PrimaryRoot  string
		WorktreePath string
		BranchName   string
	}
	discardErr error

	removeCalls []struct {
		PrimaryRoot  string
		WorktreePath string
		Branch       string
	}
	removeErr error
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
			path := rec.combineRetPath
			branch := rec.combineRetBranch
			err := rec.combineErr
			rec.mu.Unlock()
			return path, branch, err
		},
		RunChecks: func(_ context.Context, worktreePath string) (bool, string, error) {
			rec.mu.Lock()
			rec.checkCalls = append(rec.checkCalls, worktreePath)
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
			err := rec.removeErr
			rec.mu.Unlock()
			return err
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
	if report.Reason != rec.combineErr.Error() {
		t.Errorf("report.Reason = %q, want %q", report.Reason, rec.combineErr.Error())
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
	if laneNotes["lane-a"] != rec.combineErr.Error() {
		t.Errorf("lane-a note = %q, want %q", laneNotes["lane-a"], rec.combineErr.Error())
	}
	if laneNotes["lane-b"] != rec.combineErr.Error() {
		t.Errorf("lane-b note = %q, want %q", laneNotes["lane-b"], rec.combineErr.Error())
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
	if report.Reason != rec.checkRetOutput {
		t.Errorf("report.Reason = %q, want %q", report.Reason, rec.checkRetOutput)
	}

	// Assert DiscardCombined was called with CombineTree's returned path/branch
	if len(rec.discardCalls) != 1 || rec.discardCalls[0].WorktreePath != "/wt/integrate-run-1" || rec.discardCalls[0].BranchName != "lucind/integrate-run-1" {
		t.Errorf("DiscardCombined calls = %+v, want [/wt/integrate-run-1, lucind/integrate-run-1]", rec.discardCalls)
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
	if report.Reason != rec.checkErr.Error() {
		t.Errorf("report.Reason = %q, want %q", report.Reason, rec.checkErr.Error())
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
