package run_test

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
	"github.com/LanzerDevCorp/lucind-ai/internal/run"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

// batchPacket builds a valid packet for the given lane ID, matching
// testPacket's shape (executor "agy", a non-empty routed_by, matching the
// ledger schema's CHECK constraint and packet.Parse's own requirements).
func batchPacket(id string) packet.Packet {
	return packet.Packet{
		ID:                id,
		Executor:          "agy",
		RoutedBy:          "touches config, Tier A audit mandatory",
		Feature:           "feat-" + id,
		ParentRef:         "refs/heads/main",
		BaseSHA:           "b000000000000000000000000000000000000000",
		ExpectedParentSHA: "b000000000000000000000000000000000000000",
		Body:              "do the thing",
	}
}

// laneEnvelopeJSON returns a minimal envelope satisfying result.schema.json
// for the given lane ID and status ("done" or "blocked").
func laneEnvelopeJSON(id, status string) string {
	if status == "blocked" {
		return fmt.Sprintf(`{
			"packet_id": %q,
			"status": "blocked",
			"summary": "hit a hard stop",
			"hard_stops": [{"hard_stop": "do not touch internal/barrier", "fired": true, "note": "would have had to edit it"}]
		}`, id)
	}
	return fmt.Sprintf(`{
		"packet_id": %q,
		"status": "done",
		"summary": "did the thing",
		"hard_stops": [{"hard_stop": "do not touch internal/barrier", "fired": false}]
	}`, id)
}

// batchFakeExecutor is a per-lane-aware test double: it looks up the
// programmed outcome by worktree path (each lane gets a distinct worktree
// path in these tests) so different lanes can return different outcomes,
// and it records observable start/end ordering so concurrency can be
// proven without relying on wall-clock timing.
type batchFakeExecutor struct {
	mu sync.Mutex

	// outcomeFor maps a worktree path to the outcome/err that lane's Run
	// call should return.
	outcomeFor map[string]executor.Outcome
	errFor     map[string]error

	// delay, if set for a worktree path, is how long Run sleeps before
	// returning for that lane -- used only to create a genuine ordering
	// (slow lane started, then fast lane both started and finished) that
	// the trace below then verifies by recorded events, not by asserting
	// on wall-clock durations.
	delayFor map[string]time.Duration

	// blockOn, if set for a worktree path, makes Run wait on that channel
	// (in addition to any delay above) before returning -- used to prove
	// concurrency deterministically via synchronization rather than
	// timing: see TestExecuteBatchRunsLanesConcurrentlyNotSequentially.
	blockOn map[string]<-chan struct{}

	// closeOnReturn, if set for a worktree path, is closed right before
	// Run returns for that lane -- the counterpart to blockOn above.
	closeOnReturn map[string]chan struct{}

	trace []string // "start:<path>" / "end:<path>", in actual call order
}

func newBatchFakeExecutor() *batchFakeExecutor {
	return &batchFakeExecutor{
		outcomeFor:    map[string]executor.Outcome{},
		errFor:        map[string]error{},
		delayFor:      map[string]time.Duration{},
		blockOn:       map[string]<-chan struct{}{},
		closeOnReturn: map[string]chan struct{}{},
	}
}

func (f *batchFakeExecutor) Run(ctx context.Context, req executor.Request) (executor.Outcome, error) {
	f.mu.Lock()
	f.trace = append(f.trace, "start:"+req.WorktreePath)
	delay := f.delayFor[req.WorktreePath]
	blockOn := f.blockOn[req.WorktreePath]
	f.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
		}
	}
	if blockOn != nil {
		select {
		case <-blockOn:
		case <-ctx.Done():
		}
	}

	f.mu.Lock()
	f.trace = append(f.trace, "end:"+req.WorktreePath)
	closeCh := f.closeOnReturn[req.WorktreePath]
	outcome := f.outcomeFor[req.WorktreePath]
	errv, hasErr := f.errFor[req.WorktreePath]
	f.mu.Unlock()

	if closeCh != nil {
		close(closeCh)
	}

	// Mirror executor.Agy.Run's own real contract (see agy.go): a
	// dispatch whose context hit its deadline is reported as a graceful
	// TimedOut outcome with a nil error, never as a Go error.
	if ctx.Err() == context.DeadlineExceeded {
		return executor.Outcome{TimedOut: true}, nil
	}

	if hasErr {
		return executor.Outcome{}, errv
	}
	return outcome, nil
}

func (f *batchFakeExecutor) DefaultModel() string {
	return "stub-default"
}

func (f *batchFakeExecutor) KnownModels() []string {
	return []string{"stub-default"}
}

// newBatchTestDeps builds run.Deps wired to a real on-disk ledger, one
// distinct stub worktree per lane (keyed by lane ID, so tests can control
// each lane's fake executor outcome and result envelope independently), and
// a pinned clock. failWorktreeFor, if non-nil, names lane IDs whose
// CreateWorktree call should fail instead of succeeding.
func newBatchTestDeps(t *testing.T, worktreeRoot func(laneID string) string, envelopeFor func(laneID string) []byte, execEnv executor.Executor, failWorktreeFor map[string]bool) run.Deps {
	t.Helper()

	l, err := ledger.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("ledger.Open() error = %v", err)
	}
	t.Cleanup(func() { l.Close() })

	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)

	return run.Deps{
		RunID:  "run-1",
		Ledger: l,
		LookupExecutor: func(name string) (executor.Executor, error) {
			return execEnv, nil
		},
		CreateWorktree: func(_ context.Context, _, laneID string) (worktree.Worktree, error) {
			if failWorktreeFor[laneID] {
				return worktree.Worktree{}, fmt.Errorf("git worktree add: boom for %s", laneID)
			}
			wtPath := worktreeRoot(laneID)
			var baseSHA string
			if wtPath != "" {
				cmd := exec.Command("git", "-C", wtPath, "rev-parse", "HEAD")
				if out, err := cmd.Output(); err == nil {
					baseSHA = strings.TrimSpace(string(out))
				}
			}
			return worktree.Worktree{Path: wtPath, Branch: "lucind/" + laneID, BaseSHA: baseSHA}, nil
		},
		WorktreeFS: func(path string) fs.FS {
			// Derive the lane ID from the worktree path suffix, which
			// worktreeRoot always sets to the lane ID itself in these
			// tests.
			laneID := path[strings.LastIndex(path, "/")+1:]
			data := envelopeFor(laneID)
			if data == nil {
				return fstest.MapFS{}
			}
			return fstest.MapFS{".lucind/result.json": {Data: data}}
		},
		Now: func() time.Time { return now },
		HasUniqueLaneCommits: func(context.Context, string, string) (bool, error) {
			return true, nil
		},
		PorcelainEmpty: func(context.Context, string) (bool, error) {
			return true, nil
		},
		PersistEnvelope: func(context.Context, string, string, *result.Envelope) error {
			return nil
		},
	}
}

func TestExecuteBatchAllDoneReleasesAndIntegratesAll(t *testing.T) {
	root := t.TempDir()
	ids := []string{"lane-a", "lane-b", "lane-c"}
	fe := newBatchFakeExecutor()
	for _, id := range ids {
		fe.outcomeFor[root+"/"+id] = executor.Outcome{ExitCode: 0}
	}
	deps := newBatchTestDeps(t, func(id string) string { return root + "/" + id }, func(id string) []byte {
		return []byte(laneEnvelopeJSON(id, "done"))
	}, fe, nil)

	var ps []packet.Packet
	for _, id := range ids {
		ps = append(ps, batchPacket(id))
	}

	report, err := run.ExecuteBatch(context.Background(), deps, ps)
	if err != nil {
		t.Fatalf("ExecuteBatch() error = %v, want nil", err)
	}

	if !report.Released {
		t.Errorf("report.Released = false, want true")
	}
	if len(report.Lanes) != 3 {
		t.Fatalf("len(report.Lanes) = %d, want 3", len(report.Lanes))
	}
	if got := len(report.Outcome.Integrate); got != 3 {
		t.Errorf("len(report.Outcome.Integrate) = %d, want 3; Integrate = %v", got, report.Outcome.Integrate)
	}
	if len(report.Outcome.Preserve) != 0 {
		t.Errorf("report.Outcome.Preserve = %v, want empty", report.Outcome.Preserve)
	}

	// The ledger, not the Report, is the source of truth.
	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 3 {
		t.Fatalf("LaneStates() = %+v, want 3 lanes", states)
	}
	for _, s := range states {
		if s.Status != lane.Done {
			t.Errorf("lane %s status = %v, want done", s.LaneID, s.Status)
		}
	}
}

// TestExecuteBatchOneBlockedStillCompletesAndPreservesOnlyThatLane proves
// the governing rule: a lane that ends badly lets the batch finish, and
// everything is preserved -- the blocked lane must neither cancel nor skip
// the two done lanes.
func TestExecuteBatchOneBlockedStillCompletesAndPreservesOnlyThatLane(t *testing.T) {
	root := t.TempDir()
	fe := newBatchFakeExecutor()
	fe.outcomeFor[root+"/lane-blocked"] = executor.Outcome{ExitCode: 0}
	fe.outcomeFor[root+"/lane-b"] = executor.Outcome{ExitCode: 0}
	fe.outcomeFor[root+"/lane-c"] = executor.Outcome{ExitCode: 0}

	deps := newBatchTestDeps(t, func(id string) string { return root + "/" + id }, func(id string) []byte {
		if id == "lane-blocked" {
			return []byte(laneEnvelopeJSON(id, "blocked"))
		}
		return []byte(laneEnvelopeJSON(id, "done"))
	}, fe, nil)

	ps := []packet.Packet{batchPacket("lane-blocked"), batchPacket("lane-b"), batchPacket("lane-c")}

	report, err := run.ExecuteBatch(context.Background(), deps, ps)
	if err != nil {
		t.Fatalf("ExecuteBatch() error = %v, want nil", err)
	}

	if !report.Released {
		t.Errorf("report.Released = false, want true")
	}
	if got := len(report.Outcome.Preserve); got != 1 || report.Outcome.Preserve[0] != "lane-blocked" {
		t.Errorf("report.Outcome.Preserve = %v, want [lane-blocked]", report.Outcome.Preserve)
	}
	if got := len(report.Outcome.Integrate); got != 2 {
		t.Errorf("len(report.Outcome.Integrate) = %d, want 2; got %v", got, report.Outcome.Integrate)
	}

	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	byID := map[string]lane.Status{}
	for _, s := range states {
		byID[s.LaneID] = s.Status
	}
	if byID["lane-blocked"] != lane.Blocked {
		t.Errorf("lane-blocked status = %v, want blocked", byID["lane-blocked"])
	}
	if byID["lane-b"] != lane.Done {
		t.Errorf("lane-b status = %v, want done -- must not have been cancelled or skipped by lane-blocked", byID["lane-b"])
	}
	if byID["lane-c"] != lane.Done {
		t.Errorf("lane-c status = %v, want done -- must not have been cancelled or skipped by lane-blocked", byID["lane-c"])
	}
}

// TestExecuteBatchWorktreeCreationFailureStillRegistersLaneAsFailed proves
// requirement 3: a lane that never starts (its CreateWorktree call fails)
// is still registered in the ledger and driven to lane.Failed, so the
// batch's barrier still has a complete expected set and the batch itself
// still completes with the other lanes still running.
func TestExecuteBatchWorktreeCreationFailureStillRegistersLaneAsFailed(t *testing.T) {
	root := t.TempDir()
	fe := newBatchFakeExecutor()
	fe.outcomeFor[root+"/lane-b"] = executor.Outcome{ExitCode: 0}

	deps := newBatchTestDeps(t, func(id string) string { return root + "/" + id }, func(id string) []byte {
		return []byte(laneEnvelopeJSON(id, "done"))
	}, fe, map[string]bool{"lane-fails-to-start": true})

	ps := []packet.Packet{batchPacket("lane-fails-to-start"), batchPacket("lane-b")}

	report, err := run.ExecuteBatch(context.Background(), deps, ps)
	if err != nil {
		t.Fatalf("ExecuteBatch() error = %v, want nil (a lane failing to start must not fail the whole batch)", err)
	}
	if !report.Released {
		t.Errorf("report.Released = false, want true")
	}
	if len(report.Lanes) != 2 {
		t.Fatalf("len(report.Lanes) = %d, want 2", len(report.Lanes))
	}

	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("LaneStates() = %+v, want 2 lanes (the never-started lane must still be registered)", states)
	}
	byID := map[string]lane.Status{}
	for _, s := range states {
		byID[s.LaneID] = s.Status
	}
	if byID["lane-fails-to-start"] != lane.Failed {
		t.Errorf("lane-fails-to-start status = %v, want failed", byID["lane-fails-to-start"])
	}
	if byID["lane-b"] != lane.Done {
		t.Errorf("lane-b status = %v, want done -- must still have run", byID["lane-b"])
	}

	if got := len(report.Outcome.Preserve); got != 1 || report.Outcome.Preserve[0] != "lane-fails-to-start" {
		t.Errorf("report.Outcome.Preserve = %v, want [lane-fails-to-start]", report.Outcome.Preserve)
	}
}

// TestExecuteBatchRecordingFailureKeepsBothCausesOnReport proves that when
// ensureLaneFailed itself fails — here because it retries RegisterLane with
// the same unadmitted executor that caused Execute to fail — the operator
// still learns both causes. lucind-ai run prints each lane through
// printReport, which already emits Report.Diagnosis under the failure
// banner; that is the surface a run invocation is actually read on. A test
// that only asserted lane.Failed would re-pin today's behavior: the lane
// already reaches failed, and Diagnosis stays empty.
func TestExecuteBatchRecordingFailureKeepsBothCausesOnReport(t *testing.T) {
	root := t.TempDir()
	fe := newBatchFakeExecutor()
	fe.outcomeFor[root+"/lane-ok"] = executor.Outcome{ExitCode: 0}

	deps := newBatchTestDeps(t, func(id string) string { return root + "/" + id }, func(id string) []byte {
		return []byte(laneEnvelopeJSON(id, "done"))
	}, fe, nil)

	bad := batchPacket("lane-unadmitted")
	bad.Executor = "not-an-executor"
	ok := batchPacket("lane-ok")

	report, err := run.ExecuteBatch(context.Background(), deps, []packet.Packet{bad, ok})
	if err != nil {
		t.Fatalf("ExecuteBatch() error = %v, want nil (recording failure must not fail the batch)", err)
	}
	if len(report.Lanes) != 2 {
		t.Fatalf("len(report.Lanes) = %d, want 2", len(report.Lanes))
	}

	got := report.Lanes[0]
	if got.LaneID != "lane-unadmitted" {
		t.Fatalf("Lanes[0].LaneID = %q, want lane-unadmitted", got.LaneID)
	}
	if got.Status != lane.Failed {
		t.Errorf("status = %v, want failed", got.Status)
	}
	if got.Diagnosis == "" {
		t.Fatal("Diagnosis is empty: both the original lane failure and the recording failure are invisible to the operator")
	}
	if !strings.Contains(got.Diagnosis, "not-an-executor") {
		t.Errorf("Diagnosis %q does not name the original unadmitted executor", got.Diagnosis)
	}
	if !strings.Contains(got.Diagnosis, "additionally") {
		t.Errorf("Diagnosis %q does not mention the recording failure", got.Diagnosis)
	}

	if report.Lanes[1].Status != lane.Done {
		t.Errorf("lane-ok status = %v, want done -- must still have run", report.Lanes[1].Status)
	}
}

// TestExecuteBatchPostWorktreeFailureReportCarriesWorktree proves the
// operator-facing lane report names the directory when Execute created a
// worktree and then failed later — here at RegisterLane, the observed
// incident, not at admission. printReport already prints Report.Worktree
// on the `worktree:` line a human scans mid-batch; today that line is
// blank, same as an admission rejection, so the operator cannot tell
// whether to open a directory. Status failed is true of both paths and
// proves nothing.
func TestExecuteBatchPostWorktreeFailureReportCarriesWorktree(t *testing.T) {
	root := t.TempDir()
	wantPath := root + "/lane-post-wt"
	createCalls := 0
	fe := newBatchFakeExecutor()
	deps := newBatchTestDeps(t, func(id string) string { return root + "/" + id }, func(id string) []byte {
		return []byte(laneEnvelopeJSON(id, "done"))
	}, fe, nil)
	innerCreate := deps.CreateWorktree
	deps.CreateWorktree = func(ctx context.Context, primaryRoot, laneID string) (worktree.Worktree, error) {
		createCalls++
		return innerCreate(ctx, primaryRoot, laneID)
	}

	p := batchPacket("lane-post-wt")
	p.Executor = "not-an-executor"

	report, err := run.ExecuteBatch(context.Background(), deps, []packet.Packet{p})
	if err != nil {
		t.Fatalf("ExecuteBatch() error = %v, want nil", err)
	}
	if len(report.Lanes) != 1 {
		t.Fatalf("len(report.Lanes) = %d, want 1", len(report.Lanes))
	}
	if createCalls == 0 {
		t.Fatal("CreateWorktree was not called: this must fail after a worktree exists, not at admission")
	}

	got := report.Lanes[0]
	if got.Status != lane.Failed {
		t.Errorf("status = %v, want failed", got.Status)
	}
	if got.Worktree != wantPath {
		t.Errorf("Worktree = %q, want %q — the created path must appear on the report so the operator can open it", got.Worktree, wantPath)
	}
	if strings.Contains(got.Diagnosis, "admit lane") || strings.Contains(got.Diagnosis, "no worktree created") {
		t.Errorf("Diagnosis = %q, this path must not be an admission rejection", got.Diagnosis)
	}
}

// TestExecuteBatchAdmissionRejectionReportSaysNoWorktree proves the other
// half of the same scan: when validatePacketAdmission rejects the packet
// before CreateWorktree, the report's worktree line stays empty AND
// Diagnosis says so in words. printReport already prints Diagnosis under
// the failure banner; without that sentence an empty `worktree:` line is
// the same blank the post-worktree path used to print. Status failed is
// true of both and proves nothing.
func TestExecuteBatchAdmissionRejectionReportSaysNoWorktree(t *testing.T) {
	root := t.TempDir()
	createCalls := 0
	fe := newBatchFakeExecutor()
	deps := newBatchTestDeps(t, func(id string) string { return root + "/" + id }, func(id string) []byte {
		return []byte(laneEnvelopeJSON(id, "done"))
	}, fe, nil)
	innerCreate := deps.CreateWorktree
	deps.CreateWorktree = func(ctx context.Context, primaryRoot, laneID string) (worktree.Worktree, error) {
		createCalls++
		return innerCreate(ctx, primaryRoot, laneID)
	}

	p := packet.Packet{
		ID:       "lane-admit",
		Executor: "agy",
		RoutedBy: "touches config, Tier A audit mandatory",
		Body:     "do the thing",
	}

	report, err := run.ExecuteBatch(context.Background(), deps, []packet.Packet{p})
	if err != nil {
		t.Fatalf("ExecuteBatch() error = %v, want nil", err)
	}
	if len(report.Lanes) != 1 {
		t.Fatalf("len(report.Lanes) = %d, want 1", len(report.Lanes))
	}
	if createCalls != 0 {
		t.Fatalf("CreateWorktree was called %d time(s), want 0: admission must fail before a worktree exists", createCalls)
	}

	got := report.Lanes[0]
	if got.Status != lane.Failed {
		t.Errorf("status = %v, want failed", got.Status)
	}
	if got.Worktree != "" {
		t.Errorf("Worktree = %q, want empty — no directory was created", got.Worktree)
	}
	if !strings.Contains(got.Diagnosis, "admission rejected, no worktree created") {
		t.Errorf("Diagnosis = %q, want it to tell the operator admission rejected the packet and no worktree exists", got.Diagnosis)
	}
}

// TestExecuteBatchRunsLanesConcurrentlyNotSequentially proves lanes run
// concurrently by deterministic synchronization, never by wall-clock timing
// (sleeping and hoping the scheduler interleaves the way you expect is
// exactly the flakiness the task warns against). lane-slow's dispatch is
// made to block until lane-fast's dispatch has fully returned. If
// ExecuteBatch ran lanes sequentially -- fully executing lane-slow (the
// first packet) before even starting lane-fast -- lane-fast's Run would
// never be called, the channel lane-slow is blocked on would never close,
// and the whole call would hang forever. The only way this test can
// complete is if lane-fast's goroutine actually got to run (and finish)
// while lane-slow's own dispatch was still in flight, which is only
// possible if the two lanes run in separate, unserialized goroutines.
func TestExecuteBatchRunsLanesConcurrentlyNotSequentially(t *testing.T) {
	root := t.TempDir()
	fe := newBatchFakeExecutor()
	slowPath := root + "/lane-slow"
	fastPath := root + "/lane-fast"
	fe.outcomeFor[slowPath] = executor.Outcome{ExitCode: 0}
	fe.outcomeFor[fastPath] = executor.Outcome{ExitCode: 0}

	fastDone := make(chan struct{})
	fe.blockOn[slowPath] = fastDone
	fe.closeOnReturn[fastPath] = fastDone

	deps := newBatchTestDeps(t, func(id string) string { return root + "/" + id }, func(id string) []byte {
		return []byte(laneEnvelopeJSON(id, "done"))
	}, fe, nil)

	ps := []packet.Packet{batchPacket("lane-slow"), batchPacket("lane-fast")}

	type outcome struct {
		report run.BatchReport
		err    error
	}
	resultCh := make(chan outcome, 1)
	go func() {
		r, err := run.ExecuteBatch(context.Background(), deps, ps)
		resultCh <- outcome{report: r, err: err}
	}()

	select {
	case res := <-resultCh:
		if res.err != nil {
			t.Fatalf("ExecuteBatch() error = %v, want nil", res.err)
		}
		byID := map[string]lane.Status{}
		for _, r := range res.report.Lanes {
			byID[r.LaneID] = r.Status
		}
		if byID["lane-slow"] != lane.Done || byID["lane-fast"] != lane.Done {
			t.Errorf("report.Lanes = %+v, want both lane-slow and lane-fast done", res.report.Lanes)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ExecuteBatch did not complete within 3s: lane-slow's dispatch was blocked waiting for lane-fast's dispatch to finish, which only happens if the two lanes run concurrently -- this hang means they ran sequentially instead")
	}
}

// TestExecuteBatchLanesComeBackInInputOrder proves Lanes preserves input
// order regardless of completion order -- the slow lane is listed first in
// the input even though the fast lane finishes first.
func TestExecuteBatchLanesComeBackInInputOrder(t *testing.T) {
	root := t.TempDir()
	fe := newBatchFakeExecutor()
	slowPath := root + "/lane-slow"
	fastPath := root + "/lane-fast"
	fe.outcomeFor[slowPath] = executor.Outcome{ExitCode: 0}
	fe.outcomeFor[fastPath] = executor.Outcome{ExitCode: 0}
	fe.delayFor[slowPath] = 100 * time.Millisecond

	deps := newBatchTestDeps(t, func(id string) string { return root + "/" + id }, func(id string) []byte {
		return []byte(laneEnvelopeJSON(id, "done"))
	}, fe, nil)

	ps := []packet.Packet{batchPacket("lane-slow"), batchPacket("lane-fast")}

	report, err := run.ExecuteBatch(context.Background(), deps, ps)
	if err != nil {
		t.Fatalf("ExecuteBatch() error = %v, want nil", err)
	}

	if len(report.Lanes) != 2 {
		t.Fatalf("len(report.Lanes) = %d, want 2", len(report.Lanes))
	}
	if report.Lanes[0].LaneID != "lane-slow" || report.Lanes[1].LaneID != "lane-fast" {
		t.Errorf("report.Lanes = [%s, %s], want [lane-slow, lane-fast] (input order)", report.Lanes[0].LaneID, report.Lanes[1].LaneID)
	}
}

// TestExecuteBatchConcurrentLedgerWritesDoNotErrorOrLoseData proves the
// real ledger (WAL + busy_timeout + a >1 connection pool) tolerates N lanes
// writing concurrently: every lane's registration and terminal status
// arrives intact.
func TestExecuteBatchConcurrentLedgerWritesDoNotErrorOrLoseData(t *testing.T) {
	root := t.TempDir()
	const n = 8
	var ids []string
	for i := 0; i < n; i++ {
		ids = append(ids, fmt.Sprintf("lane-%02d", i))
	}

	fe := newBatchFakeExecutor()
	for _, id := range ids {
		fe.outcomeFor[root+"/"+id] = executor.Outcome{ExitCode: 0}
	}

	deps := newBatchTestDeps(t, func(id string) string { return root + "/" + id }, func(id string) []byte {
		return []byte(laneEnvelopeJSON(id, "done"))
	}, fe, nil)

	var ps []packet.Packet
	for _, id := range ids {
		ps = append(ps, batchPacket(id))
	}

	report, err := run.ExecuteBatch(context.Background(), deps, ps)
	if err != nil {
		t.Fatalf("ExecuteBatch() error = %v, want nil", err)
	}
	if len(report.Lanes) != n {
		t.Fatalf("len(report.Lanes) = %d, want %d", len(report.Lanes), n)
	}

	states, err := deps.Ledger.LaneStates(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LaneStates() error = %v", err)
	}
	if len(states) != n {
		t.Fatalf("LaneStates() = %+v, want %d lanes", states, n)
	}
	for _, s := range states {
		if s.Status != lane.Done {
			t.Errorf("lane %s status = %v, want done", s.LaneID, s.Status)
		}
	}
}

// TestExecuteBatchRejectsEmptyBatchBeforeAnySideEffect proves an empty
// packet slice is rejected before any worktree or ledger write.
func TestExecuteBatchRejectsEmptyBatchBeforeAnySideEffect(t *testing.T) {
	var createCalls int32
	l, err := ledger.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("ledger.Open() error = %v", err)
	}
	t.Cleanup(func() { l.Close() })

	deps := run.Deps{
		RunID:  "run-1",
		Ledger: l,
		LookupExecutor: func(string) (executor.Executor, error) {
			return newBatchFakeExecutor(), nil
		},
		CreateWorktree: func(context.Context, string, string) (worktree.Worktree, error) {
			atomic.AddInt32(&createCalls, 1)
			return worktree.Worktree{}, nil
		},
		WorktreeFS: func(string) fs.FS { return fstest.MapFS{} },
		Now:        func() time.Time { return time.Now() },
		PersistEnvelope: func(context.Context, string, string, *result.Envelope) error {
			return nil
		},
	}

	_, err = run.ExecuteBatch(context.Background(), deps, nil)
	if err == nil {
		t.Fatal("ExecuteBatch(nil) error = nil, want non-nil for an empty batch")
	}
	if atomic.LoadInt32(&createCalls) != 0 {
		t.Errorf("CreateWorktree was called %d time(s), want 0 for an empty batch", createCalls)
	}

	states, statesErr := deps.Ledger.LaneStates(context.Background(), "run-1")
	if statesErr != nil {
		t.Fatalf("LaneStates() error = %v", statesErr)
	}
	if len(states) != 0 {
		t.Errorf("LaneStates() = %+v, want none", states)
	}
}

// TestExecuteBatchRejectsDuplicateLaneIDsBeforeAnySideEffect proves
// duplicate lane IDs are rejected up front, via barrier.New acting as the
// sole authority on that check, before any worktree or ledger write.
func TestExecuteBatchRejectsDuplicateLaneIDsBeforeAnySideEffect(t *testing.T) {
	var createCalls int32
	l, err := ledger.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("ledger.Open() error = %v", err)
	}
	t.Cleanup(func() { l.Close() })

	deps := run.Deps{
		RunID:  "run-1",
		Ledger: l,
		LookupExecutor: func(string) (executor.Executor, error) {
			return newBatchFakeExecutor(), nil
		},
		CreateWorktree: func(context.Context, string, string) (worktree.Worktree, error) {
			atomic.AddInt32(&createCalls, 1)
			return worktree.Worktree{}, nil
		},
		WorktreeFS: func(string) fs.FS { return fstest.MapFS{} },
		Now:        func() time.Time { return time.Now() },
		PersistEnvelope: func(context.Context, string, string, *result.Envelope) error {
			return nil
		},
	}

	ps := []packet.Packet{batchPacket("lane-dup"), batchPacket("lane-dup")}

	_, err = run.ExecuteBatch(context.Background(), deps, ps)
	if err == nil {
		t.Fatal("ExecuteBatch() error = nil, want non-nil for duplicate lane IDs")
	}
	if atomic.LoadInt32(&createCalls) != 0 {
		t.Errorf("CreateWorktree was called %d time(s), want 0 when duplicate lane IDs are rejected", createCalls)
	}

	states, statesErr := deps.Ledger.LaneStates(context.Background(), "run-1")
	if statesErr != nil {
		t.Fatalf("LaneStates() error = %v", statesErr)
	}
	if len(states) != 0 {
		t.Errorf("LaneStates() = %+v, want none", states)
	}
}

// TestExecuteBatchAppliesPerLaneDeadlineIndependently proves a slow lane's
// dispatch, bounded by Deps.LaneTimeout, does not consume another lane's
// clock. lane-slow's fake dispatch never returns on its own -- it blocks on
// a channel that is never closed, so the only thing that can ever end it is
// its own per-lane deadline firing -- while lane-fast has no delay or block
// of any kind and so is never at risk of being slowed by lane-slow's
// deadline. The only wall-clock dependency left is that
// Deps.LaneTimeout must actually elapse for lane-slow to end at all, which
// is inherent to testing a real time.Duration-based deadline; there is no
// timing race between the two lanes for a scheduler hiccup to flip.
func TestExecuteBatchAppliesPerLaneDeadlineIndependently(t *testing.T) {
	root := t.TempDir()
	fe := newBatchFakeExecutor()
	slowPath := root + "/lane-slow"
	fastPath := root + "/lane-fast"
	fe.outcomeFor[fastPath] = executor.Outcome{ExitCode: 0}
	fe.outcomeFor[slowPath] = executor.Outcome{ExitCode: 0}
	// never is a channel that is never closed, so lane-slow's fake Run
	// call can only return via its ctx.Done() branch -- i.e. only once
	// its own per-lane deadline fires.
	never := make(chan struct{})
	fe.blockOn[slowPath] = never

	deps := newBatchTestDeps(t, func(id string) string { return root + "/" + id }, func(id string) []byte {
		return []byte(laneEnvelopeJSON(id, "done"))
	}, fe, nil)
	deps.LaneTimeout = 200 * time.Millisecond

	ps := []packet.Packet{batchPacket("lane-slow"), batchPacket("lane-fast")}

	report, err := run.ExecuteBatch(context.Background(), deps, ps)
	if err != nil {
		t.Fatalf("ExecuteBatch() error = %v, want nil", err)
	}

	byID := map[string]lane.Status{}
	for _, r := range report.Lanes {
		byID[r.LaneID] = r.Status
	}
	if byID["lane-fast"] != lane.Done {
		t.Errorf("lane-fast status = %v, want done -- must not be affected by lane-slow's deadline", byID["lane-fast"])
	}
	if byID["lane-slow"] != lane.Blocked {
		t.Errorf("lane-slow status = %v, want blocked (timed out on its own per-lane deadline)", byID["lane-slow"])
	}
}

// TestExecuteBatchPreservesEveryWorktreeIncludingDone proves requirement 4:
// nothing in ExecuteBatch removes a worktree, even for lanes that reached
// lane.Done. Since worktree removal would be an actual filesystem
// operation this test would need to observe, and CreateWorktree here is a
// pure stub returning an in-memory path, the strongest available proof at
// this seam is behavioral: the report and the ledger both still carry each
// done lane's worktree path afterward, i.e. nothing scrubbed it.
func TestExecuteBatchPreservesEveryWorktreeIncludingDone(t *testing.T) {
	root := t.TempDir()
	ids := []string{"lane-a", "lane-b"}
	fe := newBatchFakeExecutor()
	for _, id := range ids {
		fe.outcomeFor[root+"/"+id] = executor.Outcome{ExitCode: 0}
	}
	deps := newBatchTestDeps(t, func(id string) string { return root + "/" + id }, func(id string) []byte {
		return []byte(laneEnvelopeJSON(id, "done"))
	}, fe, nil)

	var ps []packet.Packet
	for _, id := range ids {
		ps = append(ps, batchPacket(id))
	}

	report, err := run.ExecuteBatch(context.Background(), deps, ps)
	if err != nil {
		t.Fatalf("ExecuteBatch() error = %v, want nil", err)
	}

	for _, r := range report.Lanes {
		if r.Worktree == "" {
			t.Errorf("lane %s report carries an empty Worktree path, want it preserved", r.LaneID)
		}
	}

	lanes, err := deps.Ledger.Lanes(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("Lanes() error = %v", err)
	}
	for _, ln := range lanes {
		if ln.WorktreePath == "" {
			t.Errorf("ledger lane %s carries an empty worktree_path, want it preserved", ln.LaneID)
		}
	}
}

type recordingFakeExecutor struct {
	mu       sync.Mutex
	requests []executor.Request
}

func (r *recordingFakeExecutor) Run(_ context.Context, req executor.Request) (executor.Outcome, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, req)
	return executor.Outcome{ExitCode: 0}, nil
}

func (r *recordingFakeExecutor) DefaultModel() string {
	return "stub-default"
}

func (r *recordingFakeExecutor) KnownModels() []string {
	return []string{"stub-default"}
}

// TestExecuteBatchDispatchesDifferentExecutorsPerPacket proves that a batch
// containing packets with different executor names dispatches each lane through
// the specific executor resolved for that lane, without cross-lane pollution.
func TestExecuteBatchDispatchesDifferentExecutorsPerPacket(t *testing.T) {
	root := t.TempDir()
	agyExec := &recordingFakeExecutor{}
	cursorExec := &recordingFakeExecutor{}

	p1 := packet.Packet{
		ID:                "lane-agy",
		Executor:          "agy",
		RoutedBy:          "rule agy",
		Feature:           "feat-lane-agy",
		ParentRef:         "refs/heads/main",
		BaseSHA:           "b000000000000000000000000000000000000000",
		ExpectedParentSHA: "b000000000000000000000000000000000000000",
		Body:              "prompt for agy",
	}
	p2 := packet.Packet{
		ID:                "lane-cursor",
		Executor:          "cursor-agent",
		RoutedBy:          "rule cursor",
		Feature:           "feat-lane-cursor",
		ParentRef:         "refs/heads/main",
		BaseSHA:           "b000000000000000000000000000000000000000",
		ExpectedParentSHA: "b000000000000000000000000000000000000000",
		Body:              "prompt for cursor",
	}

	deps := newBatchTestDeps(t, func(id string) string {
		return root + "/" + id
	}, func(id string) []byte {
		return []byte(laneEnvelopeJSON(id, "done"))
	}, nil, nil)

	deps.LookupExecutor = func(name string) (executor.Executor, error) {
		switch name {
		case "agy":
			return agyExec, nil
		case "cursor-agent":
			return cursorExec, nil
		default:
			return nil, fmt.Errorf("unexpected executor name %q", name)
		}
	}

	report, err := run.ExecuteBatch(context.Background(), deps, []packet.Packet{p1, p2})
	if err != nil {
		t.Fatalf("ExecuteBatch() error = %v, want nil", err)
	}
	if !report.Released {
		t.Errorf("report.Released = false, want true")
	}

	// Verify agy executor saw only lane-agy's prompt and worktree
	agyExec.mu.Lock()
	agyReqs := append([]executor.Request(nil), agyExec.requests...)
	agyExec.mu.Unlock()

	if len(agyReqs) != 1 {
		t.Fatalf("agyExec saw %d requests, want 1", len(agyReqs))
	}
	if agyReqs[0].Prompt != "prompt for agy" {
		t.Errorf("agyExec prompt = %q, want %q", agyReqs[0].Prompt, "prompt for agy")
	}
	if wantPath := root + "/lane-agy"; agyReqs[0].WorktreePath != wantPath {
		t.Errorf("agyExec worktree = %q, want %q", agyReqs[0].WorktreePath, wantPath)
	}

	// Verify cursor executor saw only lane-cursor's prompt and worktree
	cursorExec.mu.Lock()
	cursorReqs := append([]executor.Request(nil), cursorExec.requests...)
	cursorExec.mu.Unlock()

	if len(cursorReqs) != 1 {
		t.Fatalf("cursorExec saw %d requests, want 1", len(cursorReqs))
	}
	if cursorReqs[0].Prompt != "prompt for cursor" {
		t.Errorf("cursorExec prompt = %q, want %q", cursorReqs[0].Prompt, "prompt for cursor")
	}
	if wantPath := root + "/lane-cursor"; cursorReqs[0].WorktreePath != wantPath {
		t.Errorf("cursorExec worktree = %q, want %q", cursorReqs[0].WorktreePath, wantPath)
	}
}

func runBatchGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{
		"-c", "user.email=batch-test@example.com",
		"-c", "user.name=batch-test",
	}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v error = %v, output = %s", args, err, out)
	}
}

func TestExecuteBatchOutOfScopeUntrackedFileDeviatedExcludedFromIntegrate(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := t.TempDir()
	runBatchGit(t, primaryRoot, "init")
	if err := os.WriteFile(filepath.Join(primaryRoot, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md) error = %v", err)
	}
	runBatchGit(t, primaryRoot, "add", "README.md")
	runBatchGit(t, primaryRoot, "commit", "-m", "seed commit")

	wtPathA := t.TempDir()
	runBatchGit(t, primaryRoot, "worktree", "add", "-b", "lucind/lane-a", wtPathA)

	wtPathB := t.TempDir()
	runBatchGit(t, primaryRoot, "worktree", "add", "-b", "lucind/lane-b", wtPathB)

	// lane-a has untracked file out of scope: internal/serve/server.go
	serveDir := filepath.Join(wtPathA, "internal", "serve")
	if err := os.MkdirAll(serveDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(serveDir, "server.go"), []byte("package serve\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	// lane-b has in-scope committed file: internal/serve/server.go
	serveDirB := filepath.Join(wtPathB, "internal", "serve")
	if err := os.MkdirAll(serveDirB, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(serveDirB, "server.go"), []byte("package serve\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runBatchGit(t, wtPathB, "add", "internal/serve/server.go")
	runBatchGit(t, wtPathB, "commit", "-m", "add server.go")

	fe := newBatchFakeExecutor()
	fe.outcomeFor[wtPathA] = executor.Outcome{ExitCode: 0}
	fe.outcomeFor[wtPathB] = executor.Outcome{ExitCode: 0}

	wtMap := map[string]string{"lane-a": wtPathA, "lane-b": wtPathB}
	deps := newBatchTestDeps(t, func(laneID string) string {
		return wtMap[laneID]
	}, func(laneID string) []byte {
		return []byte(laneEnvelopeJSON(laneID, "done"))
	}, fe, nil)
	deps.PrimaryRoot = primaryRoot

	p1 := batchPacket("lane-a")
	p1.AllowedPaths = []string{"internal/ledger/"}

	p2 := batchPacket("lane-b")
	p2.AllowedPaths = []string{"internal/serve/"}

	report, err := run.ExecuteBatch(context.Background(), deps, []packet.Packet{p1, p2})
	if err != nil {
		t.Fatalf("ExecuteBatch() error = %v, want nil", err)
	}

	if !report.Released {
		t.Errorf("report.Released = false, want true")
	}

	if len(report.Outcome.Integrate) != 1 || report.Outcome.Integrate[0] != "lane-b" {
		t.Errorf("report.Outcome.Integrate = %v, want [lane-b]", report.Outcome.Integrate)
	}
	if len(report.Outcome.Preserve) != 1 || report.Outcome.Preserve[0] != "lane-a" {
		t.Errorf("report.Outcome.Preserve = %v, want [lane-a]", report.Outcome.Preserve)
	}

	for _, l := range report.Lanes {
		if l.LaneID == "lane-a" && l.Status != lane.Deviated {
			t.Errorf("lane-a status = %v, want lane.Deviated", l.Status)
		}
		if l.LaneID == "lane-b" && l.Status != lane.Done {
			t.Errorf("lane-b status = %v, want lane.Done", l.Status)
		}
	}
}

func TestExecuteBatchBarrierStaysIdleWhileOneLaneWaitsForApproval(t *testing.T) {
	root := t.TempDir()
	p1 := batchPacket("lane-fast")
	p2 := batchPacket("lane-wait")

	fe := newBatchFakeExecutor()
	fe.outcomeFor[root+"/lane-fast"] = executor.Outcome{ExitCode: 0}
	fe.outcomeFor[root+"/lane-wait"] = executor.Outcome{ExitCode: 0}

	deps := newBatchTestDeps(t, func(id string) string {
		return root + "/" + id
	}, func(id string) []byte {
		return []byte(laneEnvelopeJSON(id, "done"))
	}, fe, nil)
	deps.ApprovalTimeout = 3 * time.Second

	// In a background goroutine, handle approvals
	go func() {
		var fastDecided, waitDecided bool
		for !fastDecided || !waitDecided {
			time.Sleep(20 * time.Millisecond)
			if !fastDecided {
				appFast, err1 := deps.Ledger.Approval(context.Background(), deps.RunID, "lane-fast")
				if err1 == nil && appFast.Decision == ledger.DecisionPending {
					if err := deps.Ledger.Decide(context.Background(), deps.RunID, "lane-fast", "alice", ledger.DecisionApproved); err == nil {
						fastDecided = true
					}
				}
			}
			if !waitDecided {
				appWait, err2 := deps.Ledger.Approval(context.Background(), deps.RunID, "lane-wait")
				if err2 == nil && appWait.Decision == ledger.DecisionPending {
					time.Sleep(50 * time.Millisecond)
					if err := deps.Ledger.Decide(context.Background(), deps.RunID, "lane-wait", "alice", ledger.DecisionApproved); err == nil {
						waitDecided = true
					}
				}
			}
		}
	}()

	report, err := run.ExecuteBatch(context.Background(), deps, []packet.Packet{p1, p2})
	if err != nil {
		t.Fatalf("ExecuteBatch() error = %v", err)
	}
	if !report.Released {
		t.Errorf("report.Released = %v, want true", report.Released)
	}
	if len(report.Lanes) != 2 {
		t.Fatalf("len(report.Lanes) = %d, want 2", len(report.Lanes))
	}
	for _, r := range report.Lanes {
		if r.Status != lane.Done {
			t.Errorf("lane %s status = %v, want %v", r.LaneID, r.Status, lane.Done)
		}
	}
}

func TestExecuteBatchBarrierObservesBlockedWhenApprovalRejected(t *testing.T) {
	root := t.TempDir()
	p1 := batchPacket("lane-fast")
	p2 := batchPacket("lane-rej")

	fe := newBatchFakeExecutor()
	fe.outcomeFor[root+"/lane-fast"] = executor.Outcome{ExitCode: 0}
	fe.outcomeFor[root+"/lane-rej"] = executor.Outcome{ExitCode: 0}

	deps := newBatchTestDeps(t, func(id string) string {
		return root + "/" + id
	}, func(id string) []byte {
		return []byte(laneEnvelopeJSON(id, "done"))
	}, fe, nil)
	deps.ApprovalTimeout = 3 * time.Second

	go func() {
		var fastDecided, rejDecided bool
		for !fastDecided || !rejDecided {
			time.Sleep(20 * time.Millisecond)
			if !fastDecided {
				appFast, err1 := deps.Ledger.Approval(context.Background(), deps.RunID, "lane-fast")
				if err1 == nil && appFast.Decision == ledger.DecisionPending {
					if err := deps.Ledger.Decide(context.Background(), deps.RunID, "lane-fast", "alice", ledger.DecisionApproved); err == nil {
						fastDecided = true
					}
				}
			}
			if !rejDecided {
				appRej, err2 := deps.Ledger.Approval(context.Background(), deps.RunID, "lane-rej")
				if err2 == nil && appRej.Decision == ledger.DecisionPending {
					if err := deps.Ledger.Decide(context.Background(), deps.RunID, "lane-rej", "bob", ledger.DecisionRejected); err == nil {
						rejDecided = true
					}
				}
			}
		}
	}()

	report, err := run.ExecuteBatch(context.Background(), deps, []packet.Packet{p1, p2})
	if err != nil {
		t.Fatalf("ExecuteBatch() error = %v", err)
	}
	if !report.Released {
		t.Errorf("report.Released = false, want true (all lanes reached terminal status)")
	}
	if len(report.Outcome.Integrate) != 1 || report.Outcome.Integrate[0] != "lane-fast" {
		t.Errorf("report.Outcome.Integrate = %+v, want [lane-fast]", report.Outcome.Integrate)
	}
	if len(report.Outcome.Preserve) != 1 || report.Outcome.Preserve[0] != "lane-rej" {
		t.Errorf("report.Outcome.Preserve = %+v, want [lane-rej]", report.Outcome.Preserve)
	}
	for _, r := range report.Lanes {
		if r.LaneID == "lane-rej" && r.Status != lane.Blocked {
			t.Errorf("lane-rej status = %v, want lane.Blocked", r.Status)
		}
	}
}
