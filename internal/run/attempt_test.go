package run_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/feature"
	"github.com/LanzerDevCorp/lucind-ai/internal/integrate"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/run"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

type attemptSpies struct {
	mu sync.Mutex

	createWorktreeCalls []string
	combineCalls        []string
	checkCalls          []string
	promoteCASCalls     []struct {
		ParentRef    string
		CandidateSHA string
		ExpectedSHA  string
	}

	combineFunc func(ctx context.Context, primaryRoot, runID string, branches []string) (string, string, error)
	checkFunc   func(ctx context.Context, worktreePath string) (bool, string, error)
	promoteFunc func(ctx context.Context, primaryRoot, parentRef, candidateSHA, expectedSHA string) error
	refSHAFunc  func(ctx context.Context, primaryRoot, ref string) (string, error)
}

func newAttemptTestDeps(t *testing.T, spies *attemptSpies) (run.Deps, *ledger.Ledger, *feature.Service) {
	t.Helper()

	l, err := ledger.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("ledger.Open() error = %v", err)
	}
	t.Cleanup(func() { l.Close() })

	featSvc := feature.NewService(l)
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

	deps := run.Deps{
		RunID:       "run-test-1",
		PrimaryRoot: t.TempDir(),
		Ledger:      l,
		Now:         func() time.Time { return now },
		CreateWorktree: func(ctx context.Context, primaryRoot, laneID string) (worktree.Worktree, error) {
			spies.mu.Lock()
			spies.createWorktreeCalls = append(spies.createWorktreeCalls, laneID)
			spies.mu.Unlock()
			return worktree.Worktree{Path: primaryRoot + "/wt/" + laneID, Branch: "lucind/" + laneID, BaseSHA: "base-sha-1"}, nil
		},
		CombineTree: func(ctx context.Context, primaryRoot, runID string, branches []string) (string, string, error) {
			spies.mu.Lock()
			spies.combineCalls = append(spies.combineCalls, runID)
			fn := spies.combineFunc
			spies.mu.Unlock()
			if fn != nil {
				return fn(ctx, primaryRoot, runID, branches)
			}
			return primaryRoot + "/integrate-" + runID, "integrate-" + runID, nil
		},
		RunChecks: func(ctx context.Context, worktreePath string) (bool, string, error) {
			spies.mu.Lock()
			spies.checkCalls = append(spies.checkCalls, worktreePath)
			fn := spies.checkFunc
			spies.mu.Unlock()
			if fn != nil {
				return fn(ctx, worktreePath)
			}
			return true, "all checks passed", nil
		},
		PromoteCAS: func(ctx context.Context, primaryRoot, parentRef, candidateSHA, expectedSHA string) error {
			spies.mu.Lock()
			spies.promoteCASCalls = append(spies.promoteCASCalls, struct {
				ParentRef    string
				CandidateSHA string
				ExpectedSHA  string
			}{ParentRef: parentRef, CandidateSHA: candidateSHA, ExpectedSHA: expectedSHA})
			fn := spies.promoteFunc
			spies.mu.Unlock()
			if fn != nil {
				return fn(ctx, primaryRoot, parentRef, candidateSHA, expectedSHA)
			}
			return nil
		},
		ResolveRefSHA: func(ctx context.Context, primaryRoot, ref string) (string, error) {
			spies.mu.Lock()
			fn := spies.refSHAFunc
			spies.mu.Unlock()
			if fn != nil {
				return fn(ctx, primaryRoot, ref)
			}
			return "expected-parent-sha-1", nil
		},
		ResolveCandidateSHA: func(ctx context.Context, primaryRoot, worktreePath, branch string) (string, error) {
			return "candidate-sha-1", nil
		},
		DiscardCombined: func(ctx context.Context, primaryRoot, worktreePath, branchName string) error {
			return nil
		},
		FeatureLeaseTTL: 30 * time.Second,
	}

	return deps, l, featSvc
}

func TestAttemptReplayTerminalReturnsStoredResultWithoutSpies(t *testing.T) {
	terminalStatuses := []run.AttemptStatus{
		run.AttemptStatusPromoted,
		run.AttemptStatusFailed,
		run.AttemptStatusBlocked,
		run.AttemptStatusStale,
	}

	for _, st := range terminalStatuses {
		t.Run(string(st), func(t *testing.T) {
			spies := &attemptSpies{}
			deps, l, featSvc := newAttemptTestDeps(t, spies)

			featID := "feat-replay-" + string(st)
			_, err := featSvc.Create(context.Background(), featID, "refs/heads/feature-1", "base-sha-1", "expected-parent-sha-1")
			if err != nil {
				t.Fatalf("featSvc.Create() error = %v", err)
			}

			attemptID := "attempt-replay-" + string(st)
			now := time.Now().UTC().Format(time.RFC3339Nano)
			_, err = l.DB().ExecContext(context.Background(), `
				INSERT INTO integration_attempts (id, feature_id, idempotency_key, status, owner, fence, candidate_sha, failure_reason, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				attemptID, featID, "idem-"+string(st), string(st), "test-owner", 1, "candidate-sha-1", "reason-"+string(st), now, now,
			)
			if err != nil {
				t.Fatalf("insert existing terminal attempt: %v", err)
			}

			req := run.AttemptRequest{
				ID:                attemptID,
				FeatureID:         featID,
				ParentRef:         "refs/heads/feature-1",
				BaseSHA:           "base-sha-1",
				ExpectedParentSHA: "expected-parent-sha-1",
				IdempotencyKey:    "idem-" + string(st),
				Owner:             "new-caller",
				Branches:          []string{"lucind/lane-1"},
			}

			res, err := run.ExecuteAttempt(context.Background(), deps, req)
			if err != nil {
				t.Fatalf("ExecuteAttempt() error = %v", err)
			}

			if res.Status != st {
				t.Fatalf("res.Status = %v, want %v", res.Status, st)
			}
			if res.ID != attemptID {
				t.Errorf("res.ID = %v, want %v", res.ID, attemptID)
			}

			// Spies must assert 0 calls across all 4 functions
			if len(spies.createWorktreeCalls) != 0 {
				t.Errorf("CreateWorktree called %d times, want 0", len(spies.createWorktreeCalls))
			}
			if len(spies.combineCalls) != 0 {
				t.Errorf("CombineTree called %d times, want 0", len(spies.combineCalls))
			}
			if len(spies.checkCalls) != 0 {
				t.Errorf("RunChecks called %d times, want 0", len(spies.checkCalls))
			}
			if len(spies.promoteCASCalls) != 0 {
				t.Errorf("PromoteCAS called %d times, want 0", len(spies.promoteCASCalls))
			}
		})
	}
}

func TestAttemptIdempotencyUniqueFeatureAndKey(t *testing.T) {
	spies := &attemptSpies{}
	deps, _, featSvc := newAttemptTestDeps(t, spies)

	featID := "feat-idempotency-1"
	_, err := featSvc.Create(context.Background(), featID, "refs/heads/feature-1", "base-sha-1", "expected-parent-sha-1")
	if err != nil {
		t.Fatalf("featSvc.Create() error = %v", err)
	}

	req1 := run.AttemptRequest{
		ID:                "att-uuid-1",
		FeatureID:         featID,
		ParentRef:         "refs/heads/feature-1",
		BaseSHA:           "base-sha-1",
		ExpectedParentSHA: "expected-parent-sha-1",
		IdempotencyKey:    "key-shared-1",
		Owner:             "owner-1",
		Branches:          []string{"lucind/lane-1"},
	}

	res1, err := run.ExecuteAttempt(context.Background(), deps, req1)
	if err != nil {
		t.Fatalf("ExecuteAttempt(req1) error = %v", err)
	}
	if res1.Status != run.AttemptStatusPromoted {
		t.Fatalf("res1.Status = %v, want %v", res1.Status, run.AttemptStatusPromoted)
	}

	// Second attempt with same (feature_id, idempotency_key) but different attempt ID
	req2 := run.AttemptRequest{
		ID:                "att-uuid-2",
		FeatureID:         featID,
		ParentRef:         "refs/heads/feature-1",
		BaseSHA:           "base-sha-1",
		ExpectedParentSHA: "expected-parent-sha-1",
		IdempotencyKey:    "key-shared-1",
		Owner:             "owner-2",
		Branches:          []string{"lucind/lane-1"},
	}

	res2, err := run.ExecuteAttempt(context.Background(), deps, req2)
	if err != nil {
		t.Fatalf("ExecuteAttempt(req2) error = %v", err)
	}

	// Must resolve to existing attempt and return its stored result without a second promotion
	if res2.ID != "att-uuid-1" {
		t.Errorf("res2.ID = %v, want att-uuid-1 (resolved to existing attempt)", res2.ID)
	}
	if res2.Status != run.AttemptStatusPromoted {
		t.Errorf("res2.Status = %v, want %v", res2.Status, run.AttemptStatusPromoted)
	}
	if len(spies.promoteCASCalls) != 1 {
		t.Errorf("PromoteCAS called %d times, want exactly 1", len(spies.promoteCASCalls))
	}
}

func TestAttemptStateTransitionSequenceAndFailures(t *testing.T) {
	t.Run("successful full sequence", func(t *testing.T) {
		spies := &attemptSpies{}
		deps, l, featSvc := newAttemptTestDeps(t, spies)

		featID := "feat-seq-success"
		_, err := featSvc.Create(context.Background(), featID, "refs/heads/feature-seq", "base-sha-1", "expected-parent-sha-1")
		if err != nil {
			t.Fatalf("featSvc.Create() error = %v", err)
		}

		attID := "att-seq-1"
		req := run.AttemptRequest{
			ID:                attID,
			FeatureID:         featID,
			ParentRef:         "refs/heads/feature-seq",
			BaseSHA:           "base-sha-1",
			ExpectedParentSHA: "expected-parent-sha-1",
			IdempotencyKey:    "key-seq-1",
			Owner:             "owner-seq",
			Branches:          []string{"lucind/lane-1"},
		}

		res, err := run.ExecuteAttempt(context.Background(), deps, req)
		if err != nil {
			t.Fatalf("ExecuteAttempt() error = %v", err)
		}
		if res.Status != run.AttemptStatusPromoted {
			t.Fatalf("res.Status = %v, want promoted", res.Status)
		}

		events, err := l.IntegrationEvents(context.Background(), featID)
		if err != nil {
			t.Fatalf("IntegrationEvents() error = %v", err)
		}

		expectedEventTypes := []string{
			"feature_created",
			"feature_activated",
			"attempt_recorded",
			"attempt_leased",
			"attempt_combining",
			"attempt_checking",
			"attempt_cas_pending",
			"attempt_promoted",
		}
		var actualEventTypes []string
		for _, e := range events {
			actualEventTypes = append(actualEventTypes, e.Type)
		}

		if len(actualEventTypes) != len(expectedEventTypes) {
			t.Fatalf("event count = %d, want %d. Events: %v", len(actualEventTypes), len(expectedEventTypes), actualEventTypes)
		}
		for i, want := range expectedEventTypes {
			if actualEventTypes[i] != want {
				t.Errorf("event[%d] = %v, want %v", i, actualEventTypes[i], want)
			}
		}
	})

	t.Run("failure during lease acquisition", func(t *testing.T) {
		spies := &attemptSpies{}
		deps, _, featSvc := newAttemptTestDeps(t, spies)

		featID := "feat-fail-lease"
		_, err := featSvc.Create(context.Background(), featID, "refs/heads/feature-lease", "base-sha-1", "expected-parent-sha-1")
		if err != nil {
			t.Fatalf("featSvc.Create() error = %v", err)
		}

		// Hold lease by another owner
		_, err = featSvc.AcquireLease(context.Background(), featID, "other-owner", 1*time.Hour)
		if err != nil {
			t.Fatalf("AcquireLease() error = %v", err)
		}

		attID := "att-fail-lease-1"
		req := run.AttemptRequest{
			ID:                attID,
			FeatureID:         featID,
			ParentRef:         "refs/heads/feature-lease",
			BaseSHA:           "base-sha-1",
			ExpectedParentSHA: "expected-parent-sha-1",
			IdempotencyKey:    "key-fail-lease",
			Owner:             "current-owner",
			Branches:          []string{"lucind/lane-1"},
		}

		res, err := run.ExecuteAttempt(context.Background(), deps, req)
		if err != nil {
			t.Fatalf("ExecuteAttempt() unexpected hard error = %v", err)
		}
		if res.Status != run.AttemptStatusBlocked {
			t.Errorf("res.Status = %v, want %v", res.Status, run.AttemptStatusBlocked)
		}
		if res.FailureReason == "" {
			t.Errorf("res.FailureReason is empty, want detail about lease failure")
		}
	})

	t.Run("failure during combine stage", func(t *testing.T) {
		spies := &attemptSpies{
			combineFunc: func(ctx context.Context, primaryRoot, runID string, branches []string) (string, string, error) {
				return "", "", fmt.Errorf("%w: conflict in file.go", integrate.ErrMergeConflict)
			},
		}
		deps, _, featSvc := newAttemptTestDeps(t, spies)

		featID := "feat-fail-combine"
		_, err := featSvc.Create(context.Background(), featID, "refs/heads/feature-comb", "base-sha-1", "expected-parent-sha-1")
		if err != nil {
			t.Fatalf("featSvc.Create() error = %v", err)
		}

		attID := "att-fail-comb-1"
		req := run.AttemptRequest{
			ID:                attID,
			FeatureID:         featID,
			ParentRef:         "refs/heads/feature-comb",
			BaseSHA:           "base-sha-1",
			ExpectedParentSHA: "expected-parent-sha-1",
			IdempotencyKey:    "key-fail-comb",
			Owner:             "owner-comb",
			Branches:          []string{"lucind/lane-1"},
		}

		res, err := run.ExecuteAttempt(context.Background(), deps, req)
		if err != nil {
			t.Fatalf("ExecuteAttempt() unexpected hard error = %v", err)
		}
		if res.Status != run.AttemptStatusFailed {
			t.Errorf("res.Status = %v, want %v", res.Status, run.AttemptStatusFailed)
		}
		if res.FailureReason == "" || !errors.Is(errors.New(res.FailureReason), integrate.ErrMergeConflict) && len(res.FailureReason) < 10 {
			t.Errorf("res.FailureReason = %q, want detail about combine merge conflict", res.FailureReason)
		}
	})

	t.Run("failure during check stage", func(t *testing.T) {
		spies := &attemptSpies{
			checkFunc: func(ctx context.Context, worktreePath string) (bool, string, error) {
				return false, "tests failed on line 42", nil
			},
		}
		deps, _, featSvc := newAttemptTestDeps(t, spies)

		featID := "feat-fail-check"
		_, err := featSvc.Create(context.Background(), featID, "refs/heads/feature-chk", "base-sha-1", "expected-parent-sha-1")
		if err != nil {
			t.Fatalf("featSvc.Create() error = %v", err)
		}

		attID := "att-fail-chk-1"
		req := run.AttemptRequest{
			ID:                attID,
			FeatureID:         featID,
			ParentRef:         "refs/heads/feature-chk",
			BaseSHA:           "base-sha-1",
			ExpectedParentSHA: "expected-parent-sha-1",
			IdempotencyKey:    "key-fail-chk",
			Owner:             "owner-chk",
			Branches:          []string{"lucind/lane-1"},
		}

		res, err := run.ExecuteAttempt(context.Background(), deps, req)
		if err != nil {
			t.Fatalf("ExecuteAttempt() unexpected hard error = %v", err)
		}
		if res.Status != run.AttemptStatusFailed && res.Status != run.AttemptStatusBlocked {
			t.Errorf("res.Status = %v, want failed or blocked", res.Status)
		}
		if res.FailureReason == "" {
			t.Errorf("res.FailureReason is empty, want detail about check failure")
		}
	})

	t.Run("failure during CAS stage", func(t *testing.T) {
		spies := &attemptSpies{
			promoteFunc: func(ctx context.Context, primaryRoot, parentRef, candidateSHA, expectedSHA string) error {
				return integrate.ErrStaleCAS
			},
		}
		deps, _, featSvc := newAttemptTestDeps(t, spies)

		featID := "feat-fail-cas"
		_, err := featSvc.Create(context.Background(), featID, "refs/heads/feature-cas", "base-sha-1", "expected-parent-sha-1")
		if err != nil {
			t.Fatalf("featSvc.Create() error = %v", err)
		}

		attID := "att-fail-cas-1"
		req := run.AttemptRequest{
			ID:                attID,
			FeatureID:         featID,
			ParentRef:         "refs/heads/feature-cas",
			BaseSHA:           "base-sha-1",
			ExpectedParentSHA: "expected-parent-sha-1",
			IdempotencyKey:    "key-fail-cas",
			Owner:             "owner-cas",
			Branches:          []string{"lucind/lane-1"},
		}

		res, err := run.ExecuteAttempt(context.Background(), deps, req)
		if err != nil {
			t.Fatalf("ExecuteAttempt() unexpected hard error = %v", err)
		}
		if res.Status != run.AttemptStatusStale {
			t.Errorf("res.Status = %v, want %v", res.Status, run.AttemptStatusStale)
		}
		if res.FailureReason == "" {
			t.Errorf("res.FailureReason is empty, want detail about stale CAS failure")
		}
	})
}

func TestAttemptInterruptionAndRecoveryRefMatch(t *testing.T) {
	spies := &attemptSpies{}
	deps, l, featSvc := newAttemptTestDeps(t, spies)

	featID := "feat-interrupted-match"
	_, err := featSvc.Create(context.Background(), featID, "refs/heads/feature-interrupted", "base-sha-1", "expected-parent-sha-1")
	if err != nil {
		t.Fatalf("featSvc.Create() error = %v", err)
	}

	attID := "att-interrupted-1"
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = l.DB().ExecContext(context.Background(), `
		INSERT INTO integration_attempts (id, feature_id, idempotency_key, status, owner, fence, candidate_sha, failure_reason, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		attID, featID, "key-interrupted-1", string(run.AttemptStatusCombining), "crashed-owner", 1, "candidate-sha-1", "", now, now,
	)
	if err != nil {
		t.Fatalf("insert interrupted attempt: %v", err)
	}

	// Ref in git matches recorded expected parent SHA
	spies.refSHAFunc = func(ctx context.Context, primaryRoot, ref string) (string, error) {
		return "expected-parent-sha-1", nil
	}

	res, err := run.RecoverAttempt(context.Background(), deps, attID)
	if err != nil {
		t.Fatalf("RecoverAttempt() error = %v", err)
	}

	if res.Status != run.AttemptStatusPromoted {
		t.Fatalf("res.Status = %v, want %v", res.Status, run.AttemptStatusPromoted)
	}
	if len(spies.promoteCASCalls) != 1 {
		t.Errorf("PromoteCAS called %d times, want 1", len(spies.promoteCASCalls))
	}
}

func TestAttemptInterruptionAndRecoveryRefMismatchFailsClosed(t *testing.T) {
	discardCalled := false
	spies := &attemptSpies{}
	deps, l, featSvc := newAttemptTestDeps(t, spies)
	deps.DiscardCombined = func(ctx context.Context, primaryRoot, worktreePath, branchName string) error {
		discardCalled = true
		return nil
	}

	featID := "feat-interrupted-mismatch"
	_, err := featSvc.Create(context.Background(), featID, "refs/heads/feature-mismatch", "base-sha-1", "expected-parent-sha-1")
	if err != nil {
		t.Fatalf("featSvc.Create() error = %v", err)
	}

	attID := "att-interrupted-mismatch-1"
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = l.DB().ExecContext(context.Background(), `
		INSERT INTO integration_attempts (id, feature_id, idempotency_key, status, owner, fence, candidate_sha, failure_reason, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		attID, featID, "key-mismatch-1", string(run.AttemptStatusCombining), "crashed-owner", 1, "candidate-sha-1", "", now, now,
	)
	if err != nil {
		t.Fatalf("insert interrupted attempt: %v", err)
	}

	// Ref in git differs from recorded expected parent SHA!
	spies.refSHAFunc = func(ctx context.Context, primaryRoot, ref string) (string, error) {
		return "diverged-parent-sha-999", nil
	}

	res, err := run.RecoverAttempt(context.Background(), deps, attID)
	if err != nil {
		t.Fatalf("RecoverAttempt() unexpected hard error = %v", err)
	}

	if res.Status != run.AttemptStatusBlocked && res.Status != run.AttemptStatusStale {
		t.Errorf("res.Status = %v, want blocked or stale", res.Status)
	}
	if res.FailureReason == "" {
		t.Errorf("res.FailureReason is empty, want detail about ref mismatch")
	}

	// MUST NOT perform CAS promotion
	if len(spies.promoteCASCalls) != 0 {
		t.Errorf("PromoteCAS called %d times on ref mismatch, want 0", len(spies.promoteCASCalls))
	}
	// MUST NOT clean up worktree (fails closed preserving artifacts)
	if discardCalled {
		t.Errorf("DiscardCombined was called on ref mismatch, but worktree must be preserved")
	}
}

func TestAttemptBisectionPromotionFaultRecovery(t *testing.T) {
	t.Run("fault after CAS boundary: ref already candidate_sha, finalize without second CAS", func(t *testing.T) {
		spies := &attemptSpies{}
		deps, l, featSvc := newAttemptTestDeps(t, spies)

		featID := "feat-fault-post-cas"
		_, err := featSvc.Create(context.Background(), featID, "refs/heads/feature-postcas", "base-sha-1", "expected-parent-sha-1")
		if err != nil {
			t.Fatalf("featSvc.Create() error = %v", err)
		}

		attID := "att-fault-post-cas-1"
		now := time.Now().UTC().Format(time.RFC3339Nano)
		_, err = l.DB().ExecContext(context.Background(), `
			INSERT INTO integration_attempts (id, feature_id, idempotency_key, status, owner, fence, candidate_sha, failure_reason, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			attID, featID, "key-fault-post-cas", string(run.AttemptStatusCASPending), "crashed-owner", 1, "candidate-sha-verified", "", now, now,
		)
		if err != nil {
			t.Fatalf("insert cas_pending attempt: %v", err)
		}

		// Git ref has already advanced to candidate_sha (CAS succeeded before crash)
		spies.refSHAFunc = func(ctx context.Context, primaryRoot, ref string) (string, error) {
			return "candidate-sha-verified", nil
		}

		res, err := run.RecoverAttempt(context.Background(), deps, attID)
		if err != nil {
			t.Fatalf("RecoverAttempt() error = %v", err)
		}

		if res.Status != run.AttemptStatusPromoted {
			t.Fatalf("res.Status = %v, want promoted", res.Status)
		}
		// Recovery must NOT re-attempt CAS when ref is already at candidate_sha
		if len(spies.promoteCASCalls) != 0 {
			t.Errorf("PromoteCAS called %d times during post-CAS recovery, want 0", len(spies.promoteCASCalls))
		}
	})

	t.Run("fault before CAS boundary: ref is expected_parent_sha, execute CAS", func(t *testing.T) {
		spies := &attemptSpies{}
		deps, l, featSvc := newAttemptTestDeps(t, spies)

		featID := "feat-fault-pre-cas"
		_, err := featSvc.Create(context.Background(), featID, "refs/heads/feature-precas", "base-sha-1", "expected-parent-sha-1")
		if err != nil {
			t.Fatalf("featSvc.Create() error = %v", err)
		}

		attID := "att-fault-pre-cas-1"
		now := time.Now().UTC().Format(time.RFC3339Nano)
		_, err = l.DB().ExecContext(context.Background(), `
			INSERT INTO integration_attempts (id, feature_id, idempotency_key, status, owner, fence, candidate_sha, failure_reason, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			attID, featID, "key-fault-pre-cas", string(run.AttemptStatusCASPending), "crashed-owner", 1, "candidate-sha-topromote", "", now, now,
		)
		if err != nil {
			t.Fatalf("insert cas_pending attempt: %v", err)
		}

		// Git ref is still at expected_parent_sha (CAS definitely never ran)
		spies.refSHAFunc = func(ctx context.Context, primaryRoot, ref string) (string, error) {
			return "expected-parent-sha-1", nil
		}

		res, err := run.RecoverAttempt(context.Background(), deps, attID)
		if err != nil {
			t.Fatalf("RecoverAttempt() error = %v", err)
		}

		if res.Status != run.AttemptStatusPromoted {
			t.Fatalf("res.Status = %v, want promoted", res.Status)
		}
		// Recovery MUST execute CAS because ref was still expected_parent_sha
		if len(spies.promoteCASCalls) != 1 {
			t.Errorf("PromoteCAS called %d times during pre-CAS recovery, want 1", len(spies.promoteCASCalls))
		}
		if spies.promoteCASCalls[0].CandidateSHA != "candidate-sha-topromote" {
			t.Errorf("PromoteCAS CandidateSHA = %v, want candidate-sha-topromote", spies.promoteCASCalls[0].CandidateSHA)
		}
	})
}

func TestAttemptStatusValidAndTerminal(t *testing.T) {
	statuses := []struct {
		status   run.AttemptStatus
		valid    bool
		terminal bool
	}{
		{run.AttemptStatusRecorded, true, false},
		{run.AttemptStatusLeased, true, false},
		{run.AttemptStatusCombining, true, false},
		{run.AttemptStatusChecking, true, false},
		{run.AttemptStatusCASPending, true, false},
		{run.AttemptStatusPromoted, true, true},
		{run.AttemptStatusBlocked, true, true},
		{run.AttemptStatusFailed, true, true},
		{run.AttemptStatusStale, true, true},
		{run.AttemptStatus("unknown"), false, false},
	}

	for _, tt := range statuses {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.Valid(); got != tt.valid {
				t.Errorf("status.Valid() = %v, want %v", got, tt.valid)
			}
			if got := tt.status.Terminal(); got != tt.terminal {
				t.Errorf("status.Terminal() = %v, want %v", got, tt.terminal)
			}
		})
	}
}

func TestGetAttemptQueries(t *testing.T) {
	spies := &attemptSpies{}
	deps, l, featSvc := newAttemptTestDeps(t, spies)

	featID := "feat-query-test"
	_, err := featSvc.Create(context.Background(), featID, "refs/heads/feature-query", "base-sha-1", "expected-parent-sha-1")
	if err != nil {
		t.Fatalf("featSvc.Create() error = %v", err)
	}

	req := run.AttemptRequest{
		ID:                "att-query-1",
		FeatureID:         featID,
		ParentRef:         "refs/heads/feature-query",
		BaseSHA:           "base-sha-1",
		ExpectedParentSHA: "expected-parent-sha-1",
		IdempotencyKey:    "key-query-1",
		Owner:             "owner-query",
		Branches:          []string{"lucind/lane-1"},
	}

	_, err = run.ExecuteAttempt(context.Background(), deps, req)
	if err != nil {
		t.Fatalf("ExecuteAttempt() error = %v", err)
	}

	gotAtt, err := run.GetAttempt(context.Background(), l, "att-query-1")
	if err != nil {
		t.Fatalf("GetAttempt() error = %v", err)
	}
	if gotAtt.ID != "att-query-1" || gotAtt.Status != run.AttemptStatusPromoted {
		t.Errorf("GetAttempt() got %+v", gotAtt)
	}

	gotKey, err := run.GetAttemptByIdempotencyKey(context.Background(), l, featID, "key-query-1")
	if err != nil {
		t.Fatalf("GetAttemptByIdempotencyKey() error = %v", err)
	}
	if gotKey.ID != "att-query-1" || gotKey.IdempotencyKey != "key-query-1" {
		t.Errorf("GetAttemptByIdempotencyKey() got %+v", gotKey)
	}

	_, err = run.GetAttempt(context.Background(), l, "non-existent")
	if !errors.Is(err, run.ErrAttemptNotFound) {
		t.Errorf("GetAttempt(non-existent) error = %v, want ErrAttemptNotFound", err)
	}

	_, err = run.GetAttemptByIdempotencyKey(context.Background(), l, featID, "non-existent")
	if !errors.Is(err, run.ErrAttemptNotFound) {
		t.Errorf("GetAttemptByIdempotencyKey(non-existent) error = %v, want ErrAttemptNotFound", err)
	}
}

func TestAttemptValidationSentinels(t *testing.T) {
	spies := &attemptSpies{}
	deps, _, _ := newAttemptTestDeps(t, spies)

	// Missing Attempt ID
	_, err := run.ExecuteAttempt(context.Background(), deps, run.AttemptRequest{
		FeatureID:      "f1",
		IdempotencyKey: "k1",
	})
	if !errors.Is(err, run.ErrAttemptIDRequired) {
		t.Errorf("ExecuteAttempt() missing ID error = %v, want ErrAttemptIDRequired", err)
	}

	// Missing Feature ID
	_, err = run.ExecuteAttempt(context.Background(), deps, run.AttemptRequest{
		ID:             "a1",
		IdempotencyKey: "k1",
	})
	if !errors.Is(err, run.ErrMissingFeatureTarget) {
		t.Errorf("ExecuteAttempt() missing FeatureID error = %v, want ErrMissingFeatureTarget", err)
	}

	// Missing Idempotency Key
	_, err = run.ExecuteAttempt(context.Background(), deps, run.AttemptRequest{
		ID:        "a1",
		FeatureID: "f1",
	})
	if !errors.Is(err, run.ErrIdempotencyKeyRequired) {
		t.Errorf("ExecuteAttempt() missing IdempotencyKey error = %v, want ErrIdempotencyKeyRequired", err)
	}

	// Recover with empty attempt ID
	_, err = run.RecoverAttempt(context.Background(), deps, "")
	if !errors.Is(err, run.ErrAttemptIDRequired) {
		t.Errorf("RecoverAttempt(\"\") error = %v, want ErrAttemptIDRequired", err)
	}
}
