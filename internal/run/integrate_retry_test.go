package run_test

import (
	"context"
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/run"
)

// blockedEnvelopeJSON (a lane that genuinely could not finish its own
// work, as opposed to one that reached "done" and was only reverted
// because the batch-level integration step failed) is defined once in
// run_test.go and reused here.

// withWorktreeFS returns a WorktreeFS func backed by the given map of
// worktree path -> envelope JSON content (or "" for a path with no
// readable envelope at all, e.g. because the worktree was already cleaned
// up).
func withWorktreeFS(envelopeByPath map[string]string) func(string) fs.FS {
	return func(path string) fs.FS {
		content, ok := envelopeByPath[path]
		if !ok || content == "" {
			return fstest.MapFS{}
		}
		return fstest.MapFS{resultEnvelopePathForTest(): {Data: []byte(content)}}
	}
}

// TestRebuildBatchForRetryAutoSelectsOnlyDoneRevertedLanes proves that with
// no explicit lane list, RebuildBatchForRetry reconstructs a batch from
// exactly the lanes that both (a) have a preserved worktree and (b) have a
// "done" result envelope on disk -- the "reached done, only reverted
// because the batch-level check failed" case Defect B describes -- while
// silently excluding a genuinely blocked lane and a lane whose worktree was
// already cleaned up.
func TestRebuildBatchForRetryAutoSelectsOnlyDoneRevertedLanes(t *testing.T) {
	rec := &integrateRecorder{}
	deps, l := newIntegrateTestDeps(t, rec)
	runID := deps.RunID

	registerTestLane(t, l, runID, "lane-done-1", lane.Blocked, "/wt/lane-done-1", true)
	registerTestLane(t, l, runID, "lane-done-2", lane.Blocked, "/wt/lane-done-2", true)
	registerTestLane(t, l, runID, "lane-genuinely-blocked", lane.Blocked, "/wt/lane-genuinely-blocked", true)
	registerTestLane(t, l, runID, "lane-cleaned-up", lane.Blocked, "", false)

	deps.WorktreeFS = withWorktreeFS(map[string]string{
		"/wt/lane-done-1":            doneEnvelopeJSON,
		"/wt/lane-done-2":            doneEnvelopeJSON,
		"/wt/lane-genuinely-blocked": blockedEnvelopeJSON,
	})

	batch, err := run.RebuildBatchForRetry(context.Background(), deps, runID, nil)
	if err != nil {
		t.Fatalf("RebuildBatchForRetry() error = %v", err)
	}

	if !batch.Released {
		t.Errorf("batch.Released = false, want true")
	}
	want := map[string]bool{"lane-done-1": true, "lane-done-2": true}
	if len(batch.Outcome.Integrate) != len(want) {
		t.Fatalf("batch.Outcome.Integrate = %v, want exactly %v", batch.Outcome.Integrate, want)
	}
	for _, id := range batch.Outcome.Integrate {
		if !want[id] {
			t.Errorf("batch.Outcome.Integrate contains unexpected lane %q", id)
		}
	}
	if len(batch.Lanes) != 2 {
		t.Fatalf("len(batch.Lanes) = %d, want 2", len(batch.Lanes))
	}
	for _, r := range batch.Lanes {
		if r.Status != lane.Done {
			t.Errorf("lane %q Report.Status = %v, want done", r.LaneID, r.Status)
		}
		if r.Envelope == nil {
			t.Errorf("lane %q Report.Envelope = nil, want the rebuilt envelope", r.LaneID)
		}
	}
}

// TestRebuildBatchForRetryExplicitLaneMustQualify proves that when the
// caller names specific lane IDs, every one of them must qualify or the
// whole rebuild fails closed naming the disqualifying lane -- an operator
// hand-picking lanes never gets a silently smaller batch than they asked
// for.
func TestRebuildBatchForRetryExplicitLaneMustQualify(t *testing.T) {
	rec := &integrateRecorder{}
	deps, l := newIntegrateTestDeps(t, rec)
	runID := deps.RunID

	registerTestLane(t, l, runID, "lane-done", lane.Blocked, "/wt/lane-done", true)
	registerTestLane(t, l, runID, "lane-genuinely-blocked", lane.Blocked, "/wt/lane-genuinely-blocked", true)

	deps.WorktreeFS = withWorktreeFS(map[string]string{
		"/wt/lane-done":              doneEnvelopeJSON,
		"/wt/lane-genuinely-blocked": blockedEnvelopeJSON,
	})

	t.Run("explicit done lane succeeds", func(t *testing.T) {
		batch, err := run.RebuildBatchForRetry(context.Background(), deps, runID, []string{"lane-done"})
		if err != nil {
			t.Fatalf("RebuildBatchForRetry() error = %v", err)
		}
		if len(batch.Outcome.Integrate) != 1 || batch.Outcome.Integrate[0] != "lane-done" {
			t.Errorf("batch.Outcome.Integrate = %v, want [lane-done]", batch.Outcome.Integrate)
		}
	})

	t.Run("explicit non-done lane fails closed", func(t *testing.T) {
		_, err := run.RebuildBatchForRetry(context.Background(), deps, runID, []string{"lane-genuinely-blocked"})
		var notRetryable *run.ErrLaneNotRetryable
		if !errors.As(err, &notRetryable) {
			t.Fatalf("RebuildBatchForRetry() error = %v, want *run.ErrLaneNotRetryable", err)
		}
		if notRetryable.LaneID != "lane-genuinely-blocked" {
			t.Errorf("notRetryable.LaneID = %q, want lane-genuinely-blocked", notRetryable.LaneID)
		}
	})

	t.Run("explicit unknown lane fails closed", func(t *testing.T) {
		_, err := run.RebuildBatchForRetry(context.Background(), deps, runID, []string{"lane-does-not-exist"})
		var notRetryable *run.ErrLaneNotRetryable
		if !errors.As(err, &notRetryable) {
			t.Fatalf("RebuildBatchForRetry() error = %v, want *run.ErrLaneNotRetryable", err)
		}
	})
}

// TestRebuildBatchForRetryNoCandidatesIsErrNoRetryCandidates proves the
// zero-eligible-lanes case is reported as a distinct, checkable sentinel
// rather than an empty, silently-successful batch.
func TestRebuildBatchForRetryNoCandidatesIsErrNoRetryCandidates(t *testing.T) {
	rec := &integrateRecorder{}
	deps, l := newIntegrateTestDeps(t, rec)
	runID := deps.RunID

	registerTestLane(t, l, runID, "lane-genuinely-blocked", lane.Blocked, "/wt/lane-genuinely-blocked", true)
	deps.WorktreeFS = withWorktreeFS(map[string]string{
		"/wt/lane-genuinely-blocked": blockedEnvelopeJSON,
	})

	_, err := run.RebuildBatchForRetry(context.Background(), deps, runID, nil)
	if !errors.Is(err, run.ErrNoRetryCandidates) {
		t.Fatalf("RebuildBatchForRetry() error = %v, want ErrNoRetryCandidates", err)
	}
}
