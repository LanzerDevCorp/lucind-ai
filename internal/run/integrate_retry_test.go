package run_test

import (
	"context"
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/overlap"
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

// TestRetryFeatureTargetRecoversLaneOwnTargetPastFirstWave reproduces the
// documented multi-wave-chaining pattern (each wave's packet declares the
// feature's actual current tip as its own base_sha/expected_parent_sha, and
// that CAS-promotes correctly without ever touching the feature row's own,
// immutable base_sha/expected_parent_sha columns) and then proves the
// regression: once wave 2's own integration attempt is blocked for an
// unrelated reason (a real overlap conflict, not a CAS mismatch) and must be
// retried, RetryFeatureTarget recovers wave 2's own recorded expected value
// -- not the feature row's wave-1-only immutable anchor -- and the retried
// attempt promotes.
func TestRetryFeatureTargetRecoversLaneOwnTargetPastFirstWave(t *testing.T) {
	spies := &gateSpies{}
	deps, l, fx := newFeatureIntegrateDeps(t, spies)

	feat, err := fx.featSvc.Create(context.Background(), "feat-multiwave",
		"refs/heads/feature-multiwave", "base-sha-common", "expected-wave1")
	if err != nil {
		t.Fatalf("featSvc.Create() error = %v", err)
	}

	// currentRef simulates the parent ref's real current tip, exactly as
	// performCASPromotion's own pre-CAS ref read would observe it.
	currentRef := "expected-wave1"
	spies.refSHAFunc = func(ctx context.Context, primaryRoot, ref string) (string, error) {
		return currentRef, nil
	}

	// ---- Wave 1: dispatched and promoted, advancing the ref past the
	// feature's own immutable anchor. ----
	deps.RunID = "run-wave1"
	batch1 := batchWith(deps.RunID, []string{"lane-w1"}, true)
	_, att1, err := run.IntegrateFeature(context.Background(), deps, batch1, run.AttemptRequest{
		ID:                "att-w1",
		FeatureID:         feat.ID,
		ParentRef:         feat.ParentRef,
		BaseSHA:           feat.BaseSHA,
		ExpectedParentSHA: "expected-wave1",
		IdempotencyKey:    "att-w1",
	})
	if err != nil {
		t.Fatalf("wave 1 IntegrateFeature() error = %v", err)
	}
	if att1.Status != run.AttemptStatusPromoted {
		t.Fatalf("wave 1 att.Status = %q, want promoted (failure: %s)", att1.Status, att1.FailureReason)
	}
	wave1Tip := att1.CandidateSHA
	if wave1Tip == "" || wave1Tip == "expected-wave1" {
		t.Fatalf("wave1Tip = %q, want a distinct advanced tip", wave1Tip)
	}
	currentRef = wave1Tip

	// A second, unrelated active feature is required for the overlap gate to
	// evaluate anything at all (see evaluateOverlapGate: it only compares
	// against other active features, and does nothing if none exist).
	if _, err := fx.featSvc.Create(context.Background(), "feat-other",
		"refs/heads/feature-other", "base-sha-common", "expected-other"); err != nil {
		t.Fatalf("featSvc.Create(feat-other) error = %v", err)
	}

	// ---- Wave 2: a fresh packet declares the actual current tip (wave1Tip)
	// as its own base_sha/expected_parent_sha -- the documented multi-wave
	// chaining pattern. The lane itself reaches "done" and, exactly as
	// run.Execute does, records its own dispatch-time target in
	// LaneMetadata. ----
	deps.RunID = "run-wave2"
	registerTestLane(t, l, deps.RunID, "lane-w2", lane.Blocked, "/wt/lane-w2", true)
	if err := l.UpdateLaneMetadata(context.Background(), ledger.LaneMetadata{
		RunID:             deps.RunID,
		LaneID:            "lane-w2",
		Feature:           feat.ID,
		ParentRef:         feat.ParentRef,
		BaseSHA:           wave1Tip,
		ExpectedParentSHA: wave1Tip,
	}, time.Now()); err != nil {
		t.Fatalf("UpdateLaneMetadata() error = %v", err)
	}
	deps.WorktreeFS = withWorktreeFS(map[string]string{"/wt/lane-w2": doneEnvelopeJSON})

	// Wave 2's own integration attempt is blocked for an unrelated reason (a
	// real predicted-conflict overlap against feat-other) -- exactly the
	// "ID appears in reverted_ids" scenario integrate retry recovers from.
	spies.evaluateOverlapFunc = func(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...overlap.EvaluateOption) (*overlap.Evidence, error) {
		return &overlap.Evidence{
			Version: "v1", BaseSHA: baseSHA, FeatureASHA: shaA, FeatureBSHA: shaB,
			Class:     overlap.ClassRequired,
			Rationale: []string{"predicted Git merge conflict detected in file.go"},
			Signals:   overlap.Signals{PredictedConflict: true, ConflictPaths: []string{"file.go"}},
		}, nil
	}
	batch2 := batchWith(deps.RunID, []string{"lane-w2"}, true)
	_, att2, err := run.IntegrateFeature(context.Background(), deps, batch2, run.AttemptRequest{
		ID:                "att-w2",
		FeatureID:         feat.ID,
		ParentRef:         feat.ParentRef,
		BaseSHA:           wave1Tip,
		ExpectedParentSHA: wave1Tip,
		IdempotencyKey:    "att-w2",
	})
	if err != nil {
		t.Fatalf("wave 2 IntegrateFeature() error = %v", err)
	}
	if att2.Status != run.AttemptStatusBlocked {
		t.Fatalf("wave 2 att.Status = %q, want blocked (an unrelated overlap block, not a CAS failure)", att2.Status)
	}

	// The unrelated overlap block is now cleared -- e.g. via "lucind-ai
	// reconcile approve"/"resolve" -- and the run is retried.
	spies.evaluateOverlapFunc = nil

	rebuilt, err := run.RebuildBatchForRetry(context.Background(), deps, deps.RunID, nil)
	if err != nil {
		t.Fatalf("RebuildBatchForRetry() error = %v", err)
	}
	if len(rebuilt.Outcome.Integrate) != 1 || rebuilt.Outcome.Integrate[0] != "lane-w2" {
		t.Fatalf("rebuilt.Outcome.Integrate = %v, want [lane-w2]", rebuilt.Outcome.Integrate)
	}

	target, err := run.RetryFeatureTarget(context.Background(), deps, feat, rebuilt.Outcome.Integrate)
	if err != nil {
		t.Fatalf("RetryFeatureTarget() error = %v", err)
	}

	// This is the defect being guarded against: the feature row's own
	// ExpectedParentSHA is set once at Create and never updated, so on its
	// own it still names wave 1's now-stale anchor.
	if feat.ExpectedParentSHA != "expected-wave1" {
		t.Fatalf("test setup invariant broken: feat.ExpectedParentSHA = %q, want unchanged %q", feat.ExpectedParentSHA, "expected-wave1")
	}
	if target.ExpectedParentSHA != wave1Tip {
		t.Fatalf("RetryFeatureTarget().ExpectedParentSHA = %q, want the lane's own recorded target %q (not the feature row's wave-1-only anchor %q)",
			target.ExpectedParentSHA, wave1Tip, feat.ExpectedParentSHA)
	}
	if target.BaseSHA != wave1Tip {
		t.Errorf("RetryFeatureTarget().BaseSHA = %q, want %q", target.BaseSHA, wave1Tip)
	}

	_, att3, err := run.IntegrateFeature(context.Background(), deps, rebuilt, run.AttemptRequest{
		ID:                "att-w2-retry",
		FeatureID:         target.FeatureID,
		ParentRef:         target.ParentRef,
		BaseSHA:           target.BaseSHA,
		ExpectedParentSHA: target.ExpectedParentSHA,
		IdempotencyKey:    "att-w2-retry",
	})
	if err != nil {
		t.Fatalf("retry IntegrateFeature() error = %v", err)
	}
	if att3.Status != run.AttemptStatusPromoted {
		t.Fatalf("retry att.Status = %q, want promoted (failure: %s) -- integrate retry must use wave 2's own expected value, not the feature's immutable anchor", att3.Status, att3.FailureReason)
	}
}
