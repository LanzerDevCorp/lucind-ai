package run_test

import (
	"context"
	"io/fs"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/feature"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/overlap"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/reconcile"
	"github.com/LanzerDevCorp/lucind-ai/internal/run"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

type gateSpies struct {
	mu sync.Mutex

	evaluateOverlapCalls []struct {
		RepoDir string
		BaseSHA string
		SHAA    string
		SHAB    string
	}
	evaluateOverlapFunc func(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...overlap.EvaluateOption) (*overlap.Evidence, error)

	promoteCASCalls []struct {
		ParentRef    string
		CandidateSHA string
		ExpectedSHA  string
	}
	refSHAFunc     func(ctx context.Context, primaryRoot, ref string) (string, error)
	isAncestorFunc func(ctx context.Context, primaryRoot, ancestorSHA, descendantSHA string) (bool, error)
}

func newGateTestDeps(t *testing.T, spies *gateSpies) (run.Deps, *ledger.Ledger, *feature.Service) {
	t.Helper()

	l, err := ledger.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("ledger.Open() error = %v", err)
	}
	t.Cleanup(func() { l.Close() })

	featSvc := feature.NewService(l)
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

	deps := run.Deps{
		RunID:       "run-gate-1",
		PrimaryRoot: t.TempDir(),
		Ledger:      l,
		Now:         func() time.Time { return now },
		CreateWorktree: func(ctx context.Context, primaryRoot, laneID, parentRef, baseSHA string) (worktree.Worktree, error) {
			return worktree.Worktree{Path: primaryRoot + "/wt/" + laneID, Branch: "lucind/" + laneID, BaseSHA: "base-sha-common"}, nil
		},
		CombineTree: func(ctx context.Context, primaryRoot, runID, parentRef, baseSHA string, branches []string) (string, string, error) {
			return primaryRoot + "/integrate-" + runID, "integrate-" + runID, nil
		},
		RunChecks: func(ctx context.Context, worktreePath string) (bool, string, error) {
			return true, "all checks passed", nil
		},
		PromoteCAS: func(ctx context.Context, primaryRoot, parentRef, candidateSHA, expectedSHA string) error {
			spies.mu.Lock()
			spies.promoteCASCalls = append(spies.promoteCASCalls, struct {
				ParentRef    string
				CandidateSHA string
				ExpectedSHA  string
			}{ParentRef: parentRef, CandidateSHA: candidateSHA, ExpectedSHA: expectedSHA})
			spies.mu.Unlock()
			return nil
		},
		ResolveRefSHA: func(ctx context.Context, primaryRoot, ref string) (string, error) {
			spies.mu.Lock()
			fn := spies.refSHAFunc
			spies.mu.Unlock()
			if fn != nil {
				return fn(ctx, primaryRoot, ref)
			}
			return "expected-sha-" + ref, nil
		},
		ResolveCandidateSHA: func(ctx context.Context, primaryRoot, worktreePath, branch string) (string, error) {
			return "candidate-sha-" + branch, nil
		},
		// Default true: every existing gate test predates this guard and
		// implicitly assumes a matched resolution is always still valid to
		// reuse. Only a test that specifically exercises the guard overrides
		// isAncestorFunc to return false for its own scenario.
		IsAncestorSHA: func(ctx context.Context, primaryRoot, ancestorSHA, descendantSHA string) (bool, error) {
			spies.mu.Lock()
			fn := spies.isAncestorFunc
			spies.mu.Unlock()
			if fn != nil {
				return fn(ctx, primaryRoot, ancestorSHA, descendantSHA)
			}
			return true, nil
		},
		DiscardCombined: func(ctx context.Context, primaryRoot, worktreePath, branchName string) error {
			return nil
		},
		EvaluateOverlap: func(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...overlap.EvaluateOption) (*overlap.Evidence, error) {
			spies.mu.Lock()
			spies.evaluateOverlapCalls = append(spies.evaluateOverlapCalls, struct {
				RepoDir string
				BaseSHA string
				SHAA    string
				SHAB    string
			}{RepoDir: repoDir, BaseSHA: baseSHA, SHAA: shaA, SHAB: shaB})
			fn := spies.evaluateOverlapFunc
			spies.mu.Unlock()
			if fn != nil {
				return fn(ctx, repoDir, baseSHA, shaA, shaB, opts...)
			}
			return &overlap.Evidence{
				Version:     "v1",
				BaseSHA:     baseSHA,
				FeatureASHA: shaA,
				FeatureBSHA: shaB,
				Class:       overlap.ClassInformational,
			}, nil
		},
		FeatureLeaseTTL: 30 * time.Second,
	}

	return deps, l, featSvc
}

// 1. RED: Two-parent integration scenario where internal/overlap classifies the pair as reconciliation-required:
// both parents' promotion attempts are blocked (land in blocked, not promoted, not failed) while brand-new lane
// admission/dispatch for either feature still succeeds.
func TestRequiredOverlapGateBlocksBothParentsWhileAdmissionSucceeds(t *testing.T) {
	spies := &gateSpies{
		evaluateOverlapFunc: func(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...overlap.EvaluateOption) (*overlap.Evidence, error) {
			return &overlap.Evidence{
				Version:     "v1",
				BaseSHA:     baseSHA,
				FeatureASHA: shaA,
				FeatureBSHA: shaB,
				Class:       overlap.ClassRequired,
				Rationale:   []string{"predicted Git merge conflict detected in file.go"},
				Signals: overlap.Signals{
					PredictedConflict: true,
					ConflictPaths:     []string{"file.go"},
				},
				CreatedAt: time.Now().UTC(),
			}, nil
		},
	}
	deps, l, featSvc := newGateTestDeps(t, spies)

	// Create feature A and feature B
	featA, err := featSvc.Create(context.Background(), "feat-alpha", "refs/heads/feature-alpha", "base-sha-common", "expected-sha-refs/heads/feature-alpha")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-alpha) error = %v", err)
	}
	featB, err := featSvc.Create(context.Background(), "feat-beta", "refs/heads/feature-beta", "base-sha-common", "expected-sha-refs/heads/feature-beta")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-beta) error = %v", err)
	}

	// Attempt promotion for Feature A
	reqA := run.AttemptRequest{
		ID:                "att-alpha-1",
		FeatureID:         featA.ID,
		ParentRef:         featA.ParentRef,
		BaseSHA:           featA.BaseSHA,
		ExpectedParentSHA: featA.ExpectedParentSHA,
		IdempotencyKey:    "idem-alpha-1",
		Owner:             "owner-alpha",
		Branches:          []string{"lucind/lane-alpha-1"},
	}
	resA, err := run.ExecuteAttempt(context.Background(), deps, reqA)
	if err != nil {
		t.Fatalf("ExecuteAttempt(Feature A) error = %v", err)
	}
	if resA.Status != run.AttemptStatusBlocked {
		t.Fatalf("resA.Status = %v, want %v (blocked on required overlap, not promoted, not failed)", resA.Status, run.AttemptStatusBlocked)
	}

	// Attempt promotion for Feature B
	reqB := run.AttemptRequest{
		ID:                "att-beta-1",
		FeatureID:         featB.ID,
		ParentRef:         featB.ParentRef,
		BaseSHA:           featB.BaseSHA,
		ExpectedParentSHA: featB.ExpectedParentSHA,
		IdempotencyKey:    "idem-beta-1",
		Owner:             "owner-beta",
		Branches:          []string{"lucind/lane-beta-1"},
	}
	resB, err := run.ExecuteAttempt(context.Background(), deps, reqB)
	if err != nil {
		t.Fatalf("ExecuteAttempt(Feature B) error = %v", err)
	}
	if resB.Status != run.AttemptStatusBlocked {
		t.Fatalf("resB.Status = %v, want %v (blocked on required overlap, not promoted, not failed)", resB.Status, run.AttemptStatusBlocked)
	}

	// Verify CAS promotion was never invoked for either feature
	if len(spies.promoteCASCalls) != 0 {
		t.Fatalf("PromoteCAS called %d times, want 0 when blocked by required overlap", len(spies.promoteCASCalls))
	}

	// Brand-new lane admission / dispatch for either feature must still succeed (gate is at promotion, not admission)
	fsys := func(string) fs.FS {
		return fstest.MapFS{
			"result.json": &fstest.MapFile{Data: []byte(`{"packet_id":"lane-new-a","status":"done","summary":"ok","done_criteria":[{"criterion":"c1","met":true,"evidence":"e1"}],"hard_stops":[{"hard_stop":"hs1","fired":false}]}`)},
		}
	}
	exec := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	execDeps := newTestDeps(t, t.TempDir(), fsys, exec)
	execDeps.Ledger = l
	execDeps.HasUniqueLaneCommits = func(ctx context.Context, worktreePath, baseSHA string) (bool, error) { return true, nil }
	execDeps.PorcelainEmpty = func(ctx context.Context, worktreePath string) (bool, error) { return true, nil }

	newPktA := packet.Packet{
		ID:                "lane-new-a",
		Executor:          "agy",
		RoutedBy:          "test-route",
		Feature:           featA.ID,
		ParentRef:         featA.ParentRef,
		BaseSHA:           featA.BaseSHA,
		ExpectedParentSHA: featA.ExpectedParentSHA,
		Body:              "prompt",
	}
	reportA, err := run.Execute(context.Background(), execDeps, newPktA)
	if err != nil {
		t.Fatalf("run.Execute(new lane for Feature A) error = %v, want admission to succeed", err)
	}
	if reportA.LaneID != "lane-new-a" {
		t.Errorf("reportA.LaneID = %v, want lane-new-a", reportA.LaneID)
	}
}

// 2. RED: The same scenario but classified as warning -- a parent that otherwise satisfies every other promotion
// precondition proceeds to promoted; the warning is recorded/visible (query it back) but does not gate.
func TestOverlapGateWarningRecordsEvidenceWithoutBlockingPromotion(t *testing.T) {
	spies := &gateSpies{
		evaluateOverlapFunc: func(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...overlap.EvaluateOption) (*overlap.Evidence, error) {
			ev := &overlap.Evidence{
				Version:     "v1",
				BaseSHA:     baseSHA,
				FeatureASHA: shaA,
				FeatureBSHA: shaB,
				Class:       overlap.ClassWarning,
				Rationale:   []string{"shared disjoint path(s) modified across both features: shared.go"},
				Signals: overlap.Signals{
					SharedDisjointPaths: true,
					SharedPaths:         []string{"shared.go"},
				},
				CreatedAt: time.Now().UTC(),
			}
			h, _ := ev.ComputeHash()
			ev.Hash = h
			return ev, nil
		},
	}
	deps, l, featSvc := newGateTestDeps(t, spies)

	featA, err := featSvc.Create(context.Background(), "feat-warn-a", "refs/heads/feature-warn-a", "base-sha-common", "expected-sha-refs/heads/feature-warn-a")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-warn-a) error = %v", err)
	}
	_, err = featSvc.Create(context.Background(), "feat-warn-b", "refs/heads/feature-warn-b", "base-sha-common", "expected-sha-refs/heads/feature-warn-b")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-warn-b) error = %v", err)
	}

	reqA := run.AttemptRequest{
		ID:                "att-warn-a-1",
		FeatureID:         featA.ID,
		ParentRef:         featA.ParentRef,
		BaseSHA:           featA.BaseSHA,
		ExpectedParentSHA: featA.ExpectedParentSHA,
		IdempotencyKey:    "idem-warn-a-1",
		Owner:             "owner-warn-a",
		Branches:          []string{"lucind/lane-warn-a-1"},
	}
	resA, err := run.ExecuteAttempt(context.Background(), deps, reqA)
	if err != nil {
		t.Fatalf("ExecuteAttempt() error = %v", err)
	}

	// Warning must proceed to promoted
	if resA.Status != run.AttemptStatusPromoted {
		t.Fatalf("resA.Status = %v, want %v (warning does not gate promotion)", resA.Status, run.AttemptStatusPromoted)
	}
	if len(spies.promoteCASCalls) != 1 {
		t.Fatalf("PromoteCAS called %d times, want 1", len(spies.promoteCASCalls))
	}

	// Warning evidence must be recorded and visible in the ledger
	var count int
	err = l.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM overlap_evidence WHERE feature_id = ? AND evidence_class = 'warning'`, featA.ID).Scan(&count)
	if err != nil {
		t.Fatalf("query overlap_evidence: %v", err)
	}
	if count == 0 {
		t.Errorf("overlap_evidence warning count = 0, want at least 1 recorded warning evidence")
	}
}

// 3. RED: Informational classification -- promotion proceeds exactly as if no overlap evidence existed at all.
func TestOverlapGateInformationalProceedsAsNoOverlap(t *testing.T) {
	spies := &gateSpies{
		evaluateOverlapFunc: func(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...overlap.EvaluateOption) (*overlap.Evidence, error) {
			ev := &overlap.Evidence{
				Version:     "v1",
				BaseSHA:     baseSHA,
				FeatureASHA: shaA,
				FeatureBSHA: shaB,
				Class:       overlap.ClassInformational,
				Rationale:   []string{"disjoint changes with no shared hotspots"},
				CreatedAt:   time.Now().UTC(),
			}
			h, _ := ev.ComputeHash()
			ev.Hash = h
			return ev, nil
		},
	}
	deps, _, featSvc := newGateTestDeps(t, spies)

	featA, err := featSvc.Create(context.Background(), "feat-info-a", "refs/heads/feature-info-a", "base-sha-common", "expected-sha-refs/heads/feature-info-a")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-info-a) error = %v", err)
	}
	_, err = featSvc.Create(context.Background(), "feat-info-b", "refs/heads/feature-info-b", "base-sha-common", "expected-sha-refs/heads/feature-info-b")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-info-b) error = %v", err)
	}

	reqA := run.AttemptRequest{
		ID:                "att-info-a-1",
		FeatureID:         featA.ID,
		ParentRef:         featA.ParentRef,
		BaseSHA:           featA.BaseSHA,
		ExpectedParentSHA: featA.ExpectedParentSHA,
		IdempotencyKey:    "idem-info-a-1",
		Owner:             "owner-info-a",
		Branches:          []string{"lucind/lane-info-a-1"},
	}
	resA, err := run.ExecuteAttempt(context.Background(), deps, reqA)
	if err != nil {
		t.Fatalf("ExecuteAttempt() error = %v", err)
	}

	if resA.Status != run.AttemptStatusPromoted {
		t.Fatalf("resA.Status = %v, want %v", resA.Status, run.AttemptStatusPromoted)
	}
	if len(spies.promoteCASCalls) != 1 {
		t.Fatalf("PromoteCAS called %d times, want 1", len(spies.promoteCASCalls))
	}
}

// 4. RED: A required-overlap block creates exactly one reconciliation_request row per blocked overlap pair --
// not one per blocked parent, not a duplicate on retry of the same blocked promotion attempt.
func TestRequiredOverlapCreatesExactlyOneReconciliationRequestPerPair(t *testing.T) {
	spies := &gateSpies{
		evaluateOverlapFunc: func(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...overlap.EvaluateOption) (*overlap.Evidence, error) {
			ev := &overlap.Evidence{
				Version:     "v1",
				BaseSHA:     baseSHA,
				FeatureASHA: shaA,
				FeatureBSHA: shaB,
				Class:       overlap.ClassRequired,
				Rationale:   []string{"predicted Git merge conflict detected"},
				Signals: overlap.Signals{
					PredictedConflict: true,
					ConflictPaths:     []string{"conflict.go"},
				},
				CreatedAt: time.Now().UTC(),
			}
			h, _ := ev.ComputeHash()
			ev.Hash = h
			return ev, nil
		},
	}
	deps, l, featSvc := newGateTestDeps(t, spies)

	featA, err := featSvc.Create(context.Background(), "feat-dedup-a", "refs/heads/feature-dedup-a", "base-sha-common", "expected-sha-refs/heads/feature-dedup-a")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-dedup-a) error = %v", err)
	}
	featB, err := featSvc.Create(context.Background(), "feat-dedup-b", "refs/heads/feature-dedup-b", "base-sha-common", "expected-sha-refs/heads/feature-dedup-b")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-dedup-b) error = %v", err)
	}

	// Step 1: Feature A attempts promotion and is blocked
	reqA := run.AttemptRequest{
		ID:                "att-dedup-a-1",
		FeatureID:         featA.ID,
		ParentRef:         featA.ParentRef,
		BaseSHA:           featA.BaseSHA,
		ExpectedParentSHA: featA.ExpectedParentSHA,
		IdempotencyKey:    "idem-dedup-a-1",
		Owner:             "owner-dedup-a",
		Branches:          []string{"lucind/lane-dedup-a-1"},
	}
	resA1, err := run.ExecuteAttempt(context.Background(), deps, reqA)
	if err != nil {
		t.Fatalf("ExecuteAttempt(A1) error = %v", err)
	}
	if resA1.Status != run.AttemptStatusBlocked {
		t.Fatalf("resA1.Status = %v, want %v", resA1.Status, run.AttemptStatusBlocked)
	}

	// Verify exactly 1 reconciliation_request row exists
	var count1 int
	err = l.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM reconciliation_requests`).Scan(&count1)
	if err != nil {
		t.Fatalf("query reconciliation_requests count: %v", err)
	}
	if count1 != 1 {
		t.Fatalf("reconciliation_requests count after A1 = %d, want exactly 1", count1)
	}

	// Step 2: Feature B attempts promotion and is blocked
	reqB := run.AttemptRequest{
		ID:                "att-dedup-b-1",
		FeatureID:         featB.ID,
		ParentRef:         featB.ParentRef,
		BaseSHA:           featB.BaseSHA,
		ExpectedParentSHA: featB.ExpectedParentSHA,
		IdempotencyKey:    "idem-dedup-b-1",
		Owner:             "owner-dedup-b",
		Branches:          []string{"lucind/lane-dedup-b-1"},
	}
	resB, err := run.ExecuteAttempt(context.Background(), deps, reqB)
	if err != nil {
		t.Fatalf("ExecuteAttempt(B) error = %v", err)
	}
	if resB.Status != run.AttemptStatusBlocked {
		t.Fatalf("resB.Status = %v, want %v", resB.Status, run.AttemptStatusBlocked)
	}

	// Verify still exactly 1 reconciliation_request row exists (NOT one per blocked parent)
	var count2 int
	err = l.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM reconciliation_requests`).Scan(&count2)
	if err != nil {
		t.Fatalf("query reconciliation_requests count: %v", err)
	}
	if count2 != 1 {
		t.Fatalf("reconciliation_requests count after B = %d, want still exactly 1 (not one per parent)", count2)
	}

	// Step 3: Feature A retries its promotion attempt with a new attempt ID / key
	reqA2 := run.AttemptRequest{
		ID:                "att-dedup-a-2",
		FeatureID:         featA.ID,
		ParentRef:         featA.ParentRef,
		BaseSHA:           featA.BaseSHA,
		ExpectedParentSHA: featA.ExpectedParentSHA,
		IdempotencyKey:    "idem-dedup-a-2",
		Owner:             "owner-dedup-a-retry",
		Branches:          []string{"lucind/lane-dedup-a-1"},
	}
	resA2, err := run.ExecuteAttempt(context.Background(), deps, reqA2)
	if err != nil {
		t.Fatalf("ExecuteAttempt(A2 retry) error = %v", err)
	}
	if resA2.Status != run.AttemptStatusBlocked {
		t.Fatalf("resA2.Status = %v, want %v", resA2.Status, run.AttemptStatusBlocked)
	}

	// Verify still exactly 1 reconciliation_request row exists (no duplicate on retry)
	var count3 int
	err = l.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM reconciliation_requests`).Scan(&count3)
	if err != nil {
		t.Fatalf("query reconciliation_requests count: %v", err)
	}
	if count3 != 1 {
		t.Fatalf("reconciliation_requests count after A2 retry = %d, want still exactly 1 (no duplicate on retry)", count3)
	}

	// Verify the request details in the database
	var (
		reqID     string
		featID    string
		direction string
		status    string
		sourceSHA string
		targetSHA string
		evHash    string
	)
	err = l.DB().QueryRowContext(context.Background(), `
		SELECT id, feature_id, direction, status, source_sha, target_sha, evidence_hash
		FROM reconciliation_requests LIMIT 1`,
	).Scan(&reqID, &featID, &direction, &status, &sourceSHA, &targetSHA, &evHash)
	if err != nil {
		t.Fatalf("query single reconciliation_request: %v", err)
	}
	if status != "awaiting" {
		t.Errorf("request status = %q, want awaiting", status)
	}
	if evHash == "" {
		t.Errorf("request evidence_hash is empty")
	}
	if sourceSHA == "" || targetSHA == "" {
		t.Errorf("source_sha (%q) or target_sha (%q) is empty", sourceSHA, targetSHA)
	}
}

// 5. RED: A required-overlap block, once its reconciliation request is approved and its
// candidate registered as integrated with a resolved SHA (the human-in-the-loop path — see
// `lucind-ai reconcile candidate resolve`), no longer blocks a retried attempt: the retry
// promotes using the candidate's resolved SHA instead of the attempt's own raw candidate SHA,
// and no duplicate reconciliation_request is created.
func TestApprovedIntegratedCandidateUnblocksPromotion(t *testing.T) {
	// Fixed, not time.Now(): the evidence hash must be identical across the initial blocked
	// attempt and the retry for the resolved candidate to match by evidence_hash -- real
	// overlap evaluation is deterministic on unchanged content; a mock using wall-clock time
	// in CreatedAt would (wrongly) produce a different hash every call.
	evidenceCreatedAt := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	evidenceHash := ""
	spies := &gateSpies{
		evaluateOverlapFunc: func(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...overlap.EvaluateOption) (*overlap.Evidence, error) {
			ev := &overlap.Evidence{
				Version:     "v1",
				BaseSHA:     baseSHA,
				FeatureASHA: shaA,
				FeatureBSHA: shaB,
				Class:       overlap.ClassRequired,
				Rationale:   []string{"predicted Git merge conflict detected in shared.go"},
				Signals: overlap.Signals{
					PredictedConflict: true,
					ConflictPaths:     []string{"shared.go"},
				},
				CreatedAt: evidenceCreatedAt,
			}
			h, _ := ev.ComputeHash()
			ev.Hash = h
			evidenceHash = h
			return ev, nil
		},
	}
	deps, l, featSvc := newGateTestDeps(t, spies)

	featA, err := featSvc.Create(context.Background(), "feat-resolve-a", "refs/heads/feature-resolve-a", "base-sha-common", "expected-sha-refs/heads/feature-resolve-a")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-resolve-a) error = %v", err)
	}
	_, err = featSvc.Create(context.Background(), "feat-resolve-b", "refs/heads/feature-resolve-b", "base-sha-common", "expected-sha-refs/heads/feature-resolve-b")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-resolve-b) error = %v", err)
	}

	// Step 1: Feature A attempts promotion and is blocked, creating a reconciliation request.
	reqA1 := run.AttemptRequest{
		ID:                "att-resolve-a-1",
		FeatureID:         featA.ID,
		ParentRef:         featA.ParentRef,
		BaseSHA:           featA.BaseSHA,
		ExpectedParentSHA: featA.ExpectedParentSHA,
		IdempotencyKey:    "idem-resolve-a-1",
		Owner:             "owner-resolve-a",
		Branches:          []string{"lucind/lane-resolve-a-1"},
	}
	resA1, err := run.ExecuteAttempt(context.Background(), deps, reqA1)
	if err != nil {
		t.Fatalf("ExecuteAttempt(A1) error = %v", err)
	}
	if resA1.Status != run.AttemptStatusBlocked {
		t.Fatalf("resA1.Status = %v, want %v", resA1.Status, run.AttemptStatusBlocked)
	}

	var reqID string
	if err := l.DB().QueryRowContext(context.Background(), `SELECT id FROM reconciliation_requests LIMIT 1`).Scan(&reqID); err != nil {
		t.Fatalf("query reconciliation_request id: %v", err)
	}

	// Step 2: Approve the request, then register a resolved candidate SHA -- the human-in-the-loop
	// path this test exists to cover. Uses the same fixed clock as deps.Now so the request's
	// short TTL (15 minutes from a fixed point in the past) never looks expired mid-test.
	reconcileSvc := reconcile.NewService(l, reconcile.WithClock(deps.Now))
	_, _, err = reconcileSvc.Approve(context.Background(), reconcile.ApproveParams{
		RequestID:     reqID,
		SourceFeature: "feat-resolve-a",
		TargetFeature: "feat-resolve-b",
		Actor:         "test-actor",
	})
	if err != nil {
		t.Fatalf("reconcileSvc.Approve() error = %v", err)
	}

	cand, err := l.ReconciliationCandidateByRequest(context.Background(), reqID)
	if err != nil {
		t.Fatalf("ReconciliationCandidateByRequest() error = %v", err)
	}
	const resolvedSHA = "resolved-sha-by-human"
	if _, err := reconcileSvc.UpdateCandidateStatus(context.Background(), cand.ID, reconcile.CandidateStatusIntegrated, resolvedSHA, ""); err != nil {
		t.Fatalf("reconcileSvc.UpdateCandidateStatus() error = %v", err)
	}

	if evidenceHash == "" {
		t.Fatal("evidenceHash never captured by evaluateOverlapFunc")
	}

	// Step 3: Feature A retries promotion with a new attempt ID. The resolved candidate must
	// clear the block and its SHA must be what gets promoted.
	reqA2 := run.AttemptRequest{
		ID:                "att-resolve-a-2",
		FeatureID:         featA.ID,
		ParentRef:         featA.ParentRef,
		BaseSHA:           featA.BaseSHA,
		ExpectedParentSHA: featA.ExpectedParentSHA,
		IdempotencyKey:    "idem-resolve-a-2",
		Owner:             "owner-resolve-a-retry",
		Branches:          []string{"lucind/lane-resolve-a-1"},
	}
	resA2, err := run.ExecuteAttempt(context.Background(), deps, reqA2)
	if err != nil {
		t.Fatalf("ExecuteAttempt(A2 retry) error = %v", err)
	}
	if resA2.Status != run.AttemptStatusPromoted {
		t.Fatalf("resA2.Status = %v, want %v (resolved candidate must unblock promotion)", resA2.Status, run.AttemptStatusPromoted)
	}
	if resA2.CandidateSHA != resolvedSHA {
		t.Errorf("resA2.CandidateSHA = %q, want %q (must promote the human-resolved SHA, not the raw combined tree)", resA2.CandidateSHA, resolvedSHA)
	}

	if len(spies.promoteCASCalls) != 1 {
		t.Fatalf("PromoteCAS called %d times, want 1", len(spies.promoteCASCalls))
	}
	if spies.promoteCASCalls[0].CandidateSHA != resolvedSHA {
		t.Errorf("PromoteCAS candidate_sha = %q, want %q", spies.promoteCASCalls[0].CandidateSHA, resolvedSHA)
	}

	// No duplicate reconciliation_request must have been created on the cleared retry.
	var count int
	if err := l.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM reconciliation_requests`).Scan(&count); err != nil {
		t.Fatalf("query reconciliation_requests count: %v", err)
	}
	if count != 1 {
		t.Errorf("reconciliation_requests count = %d, want still exactly 1", count)
	}
}

// TestStaleResolvedCandidateBehindOwnCurrentTipIsRejected proves the data-loss
// regression this guard exists to close: a reconciliation candidate approved
// and resolved for an EARLIER round of feat-stale-a's own work (e.g. already
// consumed by an earlier promotion, with real new commits landed on the
// branch since) must NOT be silently reused for a LATER attempt just because
// the OTHER feature's tip still matches. matchedOtherSHA==otherSHA alone only
// proves the other side hasn't moved; it says nothing about whether THIS
// attempt's own content is still what the resolution was registered against.
// Reusing a stale resolution here would CAS the branch backward, discarding
// every real commit since -- exactly the reported bug (a "promoted" attempt
// that silently landed the wrong, older content).
func TestStaleResolvedCandidateBehindOwnCurrentTipIsRejected(t *testing.T) {
	spies := &gateSpies{
		evaluateOverlapFunc: func(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...overlap.EvaluateOption) (*overlap.Evidence, error) {
			ev := &overlap.Evidence{
				Version:     "v1",
				BaseSHA:     baseSHA,
				FeatureASHA: shaA,
				FeatureBSHA: shaB,
				Class:       overlap.ClassRequired,
				Rationale:   []string{"predicted Git merge conflict detected in shared.go"},
				Signals: overlap.Signals{
					PredictedConflict: true,
					ConflictPaths:     []string{"shared.go"},
				},
				CreatedAt: time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC),
			}
			h, _ := ev.ComputeHash()
			ev.Hash = h
			return ev, nil
		},
	}
	deps, l, featSvc := newGateTestDeps(t, spies)

	featA, err := featSvc.Create(context.Background(), "feat-stale-a", "refs/heads/feature-stale-a", "base-sha-common", "expected-sha-refs/heads/feature-stale-a")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-stale-a) error = %v", err)
	}
	featB, err := featSvc.Create(context.Background(), "feat-stale-b", "refs/heads/feature-stale-b", "base-sha-common", "expected-sha-refs/heads/feature-stale-b")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-stale-b) error = %v", err)
	}

	// Round 1: feat-stale-a is blocked against feat-stale-b, approved, and
	// resolved to a candidate -- exactly TestApprovedIntegratedCandidateUnblocksPromotion's setup.
	reqA1 := run.AttemptRequest{
		ID:                "att-stale-a-1",
		FeatureID:         featA.ID,
		ParentRef:         featA.ParentRef,
		BaseSHA:           featA.BaseSHA,
		ExpectedParentSHA: featA.ExpectedParentSHA,
		IdempotencyKey:    "idem-stale-a-1",
		Owner:             "owner-stale-a",
		Branches:          []string{"lucind/lane-stale-a-1"},
	}
	resA1, err := run.ExecuteAttempt(context.Background(), deps, reqA1)
	if err != nil {
		t.Fatalf("ExecuteAttempt(A1) error = %v", err)
	}
	if resA1.Status != run.AttemptStatusBlocked {
		t.Fatalf("resA1.Status = %v, want %v", resA1.Status, run.AttemptStatusBlocked)
	}

	var reqID string
	if err := l.DB().QueryRowContext(context.Background(), `SELECT id FROM reconciliation_requests LIMIT 1`).Scan(&reqID); err != nil {
		t.Fatalf("query reconciliation_request id: %v", err)
	}

	reconcileSvc := reconcile.NewService(l, reconcile.WithClock(deps.Now))
	_, _, err = reconcileSvc.Approve(context.Background(), reconcile.ApproveParams{
		RequestID:     reqID,
		SourceFeature: featA.ID,
		TargetFeature: featB.ID,
		Actor:         "test-actor",
	})
	if err != nil {
		t.Fatalf("reconcileSvc.Approve() error = %v", err)
	}
	cand, err := l.ReconciliationCandidateByRequest(context.Background(), reqID)
	if err != nil {
		t.Fatalf("ReconciliationCandidateByRequest() error = %v", err)
	}
	const round1ResolvedSHA = "resolved-sha-round1"
	if _, err := reconcileSvc.UpdateCandidateStatus(context.Background(), cand.ID, reconcile.CandidateStatusIntegrated, round1ResolvedSHA, ""); err != nil {
		t.Fatalf("reconcileSvc.UpdateCandidateStatus() error = %v", err)
	}

	// The request stays "approved" (this is exactly the real bug: nothing
	// ever transitions it away). feat-stale-a's own branch has since moved
	// PAST round1ResolvedSHA -- e.g. an earlier promotion already consumed
	// it, and further real commits landed on top. Simulate that by making
	// ResolveRefSHA report a newer current tip for feat-stale-a's own parent
	// ref from now on, and wiring IsAncestorSHA to reflect the real git
	// relationship: round1ResolvedSHA is NOT an ancestor of the new tip (the
	// new tip descends from it, not the other way around).
	const newerOwnTip = "own-tip-after-round1-promotion-plus-more-commits"
	spies.refSHAFunc = func(ctx context.Context, primaryRoot, ref string) (string, error) {
		if ref == featA.ParentRef {
			return newerOwnTip, nil
		}
		return "expected-sha-" + ref, nil
	}
	spies.isAncestorFunc = func(ctx context.Context, primaryRoot, ancestorSHA, descendantSHA string) (bool, error) {
		if ancestorSHA == newerOwnTip && descendantSHA == round1ResolvedSHA {
			return false, nil
		}
		return true, nil
	}

	// Round 2: a genuinely new dispatch for feat-stale-a (a distinct Lane
	// branch, not a mere retry of round 1's own lane) hits the SAME required
	// overlap against feat-stale-b, whose tip has not moved.
	reqA2 := run.AttemptRequest{
		ID:        "att-stale-a-2",
		FeatureID: featA.ID,
		ParentRef: featA.ParentRef,
		BaseSHA:   featA.BaseSHA,
		// Matches newerOwnTip, exactly like a real feature-targeted retry
		// that re-reads the feature's current parent_ref/base_sha from the
		// ledger at retry time (recovery-reconciliation.md): the ref really
		// is where the attempt expects it. The pre-existing CAS staleness
		// check (currentSHA != expectedParentSHA) must not be what blocks
		// this attempt -- only ownResolutionStillValid's content guard
		// should.
		ExpectedParentSHA: newerOwnTip,
		IdempotencyKey:    "idem-stale-a-2",
		Owner:             "owner-stale-a-2",
		Branches:          []string{"lucind/lane-stale-a-2"},
	}
	resA2, err := run.ExecuteAttempt(context.Background(), deps, reqA2)
	if err != nil {
		t.Fatalf("ExecuteAttempt(A2) error = %v", err)
	}

	if resA2.Status == run.AttemptStatusPromoted && resA2.CandidateSHA == round1ResolvedSHA {
		t.Fatalf("resA2 silently promoted the stale round-1 resolved candidate %q (status=%v) -- this is the reported data-loss regression: the branch would CAS backward, discarding every real commit since", round1ResolvedSHA, resA2.Status)
	}
	for _, c := range spies.promoteCASCalls {
		if c.CandidateSHA == round1ResolvedSHA {
			t.Fatalf("PromoteCAS called with stale round-1 resolved candidate_sha %q", round1ResolvedSHA)
		}
	}
	if resA2.Status != run.AttemptStatusBlocked {
		t.Errorf("resA2.Status = %v, want %v (own tip has moved past the resolved candidate; the stale resolution must not silently unblock promotion)", resA2.Status, run.AttemptStatusBlocked)
	}
}

// 6. Regression: a single required conflict with no matching resolution must still block
// immediately with the existing per-conflict reason, unchanged by the post-loop accumulation
// refactor.
func TestUnresolvedRequiredOverlapBlocksWithReconciliationReason(t *testing.T) {
	spies := &gateSpies{
		evaluateOverlapFunc: func(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...overlap.EvaluateOption) (*overlap.Evidence, error) {
			ev := &overlap.Evidence{
				Version:     "v1",
				BaseSHA:     baseSHA,
				FeatureASHA: shaA,
				FeatureBSHA: shaB,
				Class:       overlap.ClassRequired,
				Rationale:   []string{"predicted Git merge conflict detected"},
				CreatedAt:   time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC),
			}
			h, _ := ev.ComputeHash()
			ev.Hash = h
			return ev, nil
		},
	}
	deps, _, featSvc := newGateTestDeps(t, spies)

	featA, err := featSvc.Create(context.Background(), "feat-unresolved-a", "refs/heads/feature-unresolved-a", "base-sha-common", "expected-sha-refs/heads/feature-unresolved-a")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-unresolved-a) error = %v", err)
	}
	featB, err := featSvc.Create(context.Background(), "feat-unresolved-b", "refs/heads/feature-unresolved-b", "base-sha-common", "expected-sha-refs/heads/feature-unresolved-b")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-unresolved-b) error = %v", err)
	}

	req := run.AttemptRequest{
		ID:                "att-unresolved-a-1",
		FeatureID:         featA.ID,
		ParentRef:         featA.ParentRef,
		BaseSHA:           featA.BaseSHA,
		ExpectedParentSHA: featA.ExpectedParentSHA,
		IdempotencyKey:    "idem-unresolved-a-1",
		Owner:             "owner-unresolved-a",
		Branches:          []string{"lucind/lane-unresolved-a-1"},
	}
	res, err := run.ExecuteAttempt(context.Background(), deps, req)
	if err != nil {
		t.Fatalf("ExecuteAttempt() error = %v", err)
	}
	if res.Status != run.AttemptStatusBlocked {
		t.Fatalf("res.Status = %v, want %v", res.Status, run.AttemptStatusBlocked)
	}
	wantReason := "promotion blocked: reconciliation-required overlap with feature " + featB.ID
	if res.FailureReason != wantReason {
		t.Errorf("res.FailureReason = %q, want %q", res.FailureReason, wantReason)
	}
}

// 7. RED: Two required-overlap conflicts against two different other features, where only one is
// already resolved (approved request + integrated candidate) and the other is still unresolved.
// The unresolved conflict must still block immediately, exactly as an unresolved conflict always
// has -- but critically, evalFunc must be called with the SAME original candidate SHA for BOTH
// comparisons, never a SHA mutated by the first (resolved) conflict's override. Before this fix,
// evaluateOverlapGate mutated att.CandidateSHA in place as soon as the first conflict resolved,
// so every later comparison in the same pass was silently evaluated against the wrong SHA.
func TestTwoRequiredConflictsOneResolvedOneUnresolvedUsesOriginalCandidateSHA(t *testing.T) {
	type shaCall struct {
		ShaA string
		ShaB string
	}
	var calls []shaCall

	spies := &gateSpies{}
	deps, l, featSvc := newGateTestDeps(t, spies)

	requiredEvalFunc := func(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...overlap.EvaluateOption) (*overlap.Evidence, error) {
		ev := &overlap.Evidence{
			Version:     "v1",
			BaseSHA:     baseSHA,
			FeatureASHA: shaA,
			FeatureBSHA: shaB,
			Class:       overlap.ClassRequired,
			Rationale:   []string{"predicted Git merge conflict detected"},
			Signals: overlap.Signals{
				PredictedConflict: true,
				ConflictPaths:     []string{"conflict.go"},
			},
			CreatedAt: time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC),
		}
		h, _ := ev.ComputeHash()
		ev.Hash = h
		return ev, nil
	}
	spies.evaluateOverlapFunc = requiredEvalFunc

	featMain, err := featSvc.Create(context.Background(), "feat-nway-main", "refs/heads/feature-nway-main", "base-sha-common", "expected-sha-refs/heads/feature-nway-main")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-nway-main) error = %v", err)
	}
	featResolved, err := featSvc.Create(context.Background(), "feat-nway-resolved", "refs/heads/feature-nway-resolved", "base-sha-common", "expected-sha-refs/heads/feature-nway-resolved")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-nway-resolved) error = %v", err)
	}

	// Step 1: block feat-nway-main against feat-nway-resolved alone (the unresolved feature does
	// not exist yet), creating the reconciliation request that gets resolved below.
	setupReq := run.AttemptRequest{
		ID:                "att-nway-setup-1",
		FeatureID:         featMain.ID,
		ParentRef:         featMain.ParentRef,
		BaseSHA:           featMain.BaseSHA,
		ExpectedParentSHA: featMain.ExpectedParentSHA,
		IdempotencyKey:    "idem-nway-setup-1",
		Owner:             "owner-nway-setup",
		Branches:          []string{"lucind/lane-nway-setup-1"},
	}
	setupRes, err := run.ExecuteAttempt(context.Background(), deps, setupReq)
	if err != nil {
		t.Fatalf("ExecuteAttempt(setup) error = %v", err)
	}
	if setupRes.Status != run.AttemptStatusBlocked {
		t.Fatalf("setupRes.Status = %v, want %v", setupRes.Status, run.AttemptStatusBlocked)
	}

	var reqID string
	if err := l.DB().QueryRowContext(context.Background(), `SELECT id FROM reconciliation_requests LIMIT 1`).Scan(&reqID); err != nil {
		t.Fatalf("query reconciliation_request id: %v", err)
	}

	reconcileSvc := reconcile.NewService(l, reconcile.WithClock(deps.Now))
	_, _, err = reconcileSvc.Approve(context.Background(), reconcile.ApproveParams{
		RequestID:     reqID,
		SourceFeature: featMain.ID,
		TargetFeature: featResolved.ID,
		Actor:         "test-actor",
	})
	if err != nil {
		t.Fatalf("reconcileSvc.Approve() error = %v", err)
	}
	cand, err := l.ReconciliationCandidateByRequest(context.Background(), reqID)
	if err != nil {
		t.Fatalf("ReconciliationCandidateByRequest() error = %v", err)
	}
	const resolvedSHA = "resolved-sha-nway-a"
	if _, err := reconcileSvc.UpdateCandidateStatus(context.Background(), cand.ID, reconcile.CandidateStatusIntegrated, resolvedSHA, ""); err != nil {
		t.Fatalf("reconcileSvc.UpdateCandidateStatus() error = %v", err)
	}

	// Step 2: bring in the second, still-unresolved conflict, and record every evalFunc call made
	// during the retry that must see both features.
	featUnresolved, err := featSvc.Create(context.Background(), "feat-nway-unresolved", "refs/heads/feature-nway-unresolved", "base-sha-common", "expected-sha-refs/heads/feature-nway-unresolved")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-nway-unresolved) error = %v", err)
	}

	spies.evaluateOverlapFunc = func(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...overlap.EvaluateOption) (*overlap.Evidence, error) {
		calls = append(calls, shaCall{ShaA: shaA, ShaB: shaB})
		return requiredEvalFunc(ctx, repoDir, baseSHA, shaA, shaB, opts...)
	}

	retryReq := run.AttemptRequest{
		ID:                "att-nway-retry-1",
		FeatureID:         featMain.ID,
		ParentRef:         featMain.ParentRef,
		BaseSHA:           featMain.BaseSHA,
		ExpectedParentSHA: featMain.ExpectedParentSHA,
		IdempotencyKey:    "idem-nway-retry-1",
		Owner:             "owner-nway-retry",
		Branches:          []string{"lucind/lane-nway-retry-1"},
	}
	retryRes, err := run.ExecuteAttempt(context.Background(), deps, retryReq)
	if err != nil {
		t.Fatalf("ExecuteAttempt(retry) error = %v", err)
	}
	if retryRes.Status != run.AttemptStatusBlocked {
		t.Fatalf("retryRes.Status = %v, want %v (the unresolved conflict must still block)", retryRes.Status, run.AttemptStatusBlocked)
	}
	if !strings.Contains(retryRes.FailureReason, featUnresolved.ID) {
		t.Errorf("retryRes.FailureReason = %q, want it to mention %q (the still-unresolved feature)", retryRes.FailureReason, featUnresolved.ID)
	}

	if len(calls) != 2 {
		t.Fatalf("evalFunc called %d times, want 2 (one per other active feature)", len(calls))
	}
	expectedSHA := "candidate-sha-integrate-att-nway-retry-1"
	for i, c := range calls {
		if c.ShaA != expectedSHA {
			t.Errorf("call[%d].ShaA = %q, want %q (the retry's own original candidate SHA, not a SHA mutated by resolving the other conflict)", i, c.ShaA, expectedSHA)
		}
	}
	if calls[0].ShaA != calls[1].ShaA {
		t.Errorf("call[0].ShaA = %q != call[1].ShaA = %q, want identical original SHA for both comparisons", calls[0].ShaA, calls[1].ShaA)
	}

	var reqCount int
	if err := l.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM reconciliation_requests`).Scan(&reqCount); err != nil {
		t.Fatalf("query reconciliation_requests count: %v", err)
	}
	if reqCount != 2 {
		t.Errorf("reconciliation_requests count = %d, want 2 (one resolved, one newly created for the unresolved conflict)", reqCount)
	}
}

// 8. RED: Two required-overlap conflicts against two different other features, BOTH already
// resolved (approved request + integrated candidate). A single resolution can be safely promoted
// (see TestApprovedIntegratedCandidateUnblocksPromotion), but combining two independent
// merge-commit resolutions into one candidate is out of scope for this fix -- silently picking
// one and discarding the other would promote a candidate that drops one of the two human
// resolutions. The attempt must instead block explicitly with a clear N-way reason, leave
// CandidateSHA unchanged from the attempt's own original candidate, and release its lease so a
// human can resolve and promote sequentially, one pair at a time.
func TestTwoRequiredConflictsBothResolvedBlocksNWayReconciliation(t *testing.T) {
	spies := &gateSpies{}
	deps, l, featSvc := newGateTestDeps(t, spies)

	requiredEvalFunc := func(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...overlap.EvaluateOption) (*overlap.Evidence, error) {
		ev := &overlap.Evidence{
			Version:     "v1",
			BaseSHA:     baseSHA,
			FeatureASHA: shaA,
			FeatureBSHA: shaB,
			Class:       overlap.ClassRequired,
			Rationale:   []string{"predicted Git merge conflict detected"},
			Signals: overlap.Signals{
				PredictedConflict: true,
				ConflictPaths:     []string{"conflict.go"},
			},
			CreatedAt: time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC),
		}
		h, _ := ev.ComputeHash()
		ev.Hash = h
		return ev, nil
	}
	spies.evaluateOverlapFunc = requiredEvalFunc

	featMain, err := featSvc.Create(context.Background(), "feat-nway2-main", "refs/heads/feature-nway2-main", "base-sha-common", "expected-sha-refs/heads/feature-nway2-main")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-nway2-main) error = %v", err)
	}

	reconcileSvc := reconcile.NewService(l, reconcile.WithClock(deps.Now))

	// resolveOneConflict runs one blocking attempt against otherFeatID (the only other active
	// feature the first time it is called against a fresh pair), then approves and integrates a
	// resolved candidate for it, mirroring the human-in-the-loop path from
	// TestApprovedIntegratedCandidateUnblocksPromotion.
	resolveOneConflict := func(otherFeatID, attemptID, resolvedSHA string) {
		t.Helper()

		req := run.AttemptRequest{
			ID:                attemptID,
			FeatureID:         featMain.ID,
			ParentRef:         featMain.ParentRef,
			BaseSHA:           featMain.BaseSHA,
			ExpectedParentSHA: featMain.ExpectedParentSHA,
			IdempotencyKey:    "idem-" + attemptID,
			Owner:             "owner-" + attemptID,
			Branches:          []string{"lucind/lane-" + attemptID},
		}
		res, err := run.ExecuteAttempt(context.Background(), deps, req)
		if err != nil {
			t.Fatalf("ExecuteAttempt(%s) error = %v", attemptID, err)
		}
		if res.Status != run.AttemptStatusBlocked {
			t.Fatalf("ExecuteAttempt(%s).Status = %v, want %v", attemptID, res.Status, run.AttemptStatusBlocked)
		}

		existing, err := l.AllReconciliationRequests(context.Background())
		if err != nil {
			t.Fatalf("AllReconciliationRequests() error = %v", err)
		}
		var reqID string
		for _, r := range existing {
			src, _, tgt, _ := reconcile.ParseDirection(r.Direction)
			if (src == featMain.ID && tgt == otherFeatID) || (src == otherFeatID && tgt == featMain.ID) {
				if r.Status == string(reconcile.RequestStatusAwaiting) {
					reqID = r.ID
					break
				}
			}
		}
		if reqID == "" {
			t.Fatalf("no awaiting reconciliation_request found for %s", otherFeatID)
		}

		if _, _, err := reconcileSvc.Approve(context.Background(), reconcile.ApproveParams{
			RequestID:     reqID,
			SourceFeature: featMain.ID,
			TargetFeature: otherFeatID,
			Actor:         "test-actor",
		}); err != nil {
			t.Fatalf("reconcileSvc.Approve(%s) error = %v", otherFeatID, err)
		}
		cand, err := l.ReconciliationCandidateByRequest(context.Background(), reqID)
		if err != nil {
			t.Fatalf("ReconciliationCandidateByRequest(%s) error = %v", otherFeatID, err)
		}
		if _, err := reconcileSvc.UpdateCandidateStatus(context.Background(), cand.ID, reconcile.CandidateStatusIntegrated, resolvedSHA, ""); err != nil {
			t.Fatalf("reconcileSvc.UpdateCandidateStatus(%s) error = %v", otherFeatID, err)
		}
	}

	featB, err := featSvc.Create(context.Background(), "feat-nway2-b", "refs/heads/feature-nway2-b", "base-sha-common", "expected-sha-refs/heads/feature-nway2-b")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-nway2-b) error = %v", err)
	}
	resolveOneConflict(featB.ID, "att-nway2-setup-b", "resolved-sha-nway2-b")

	featC, err := featSvc.Create(context.Background(), "feat-nway2-c", "refs/heads/feature-nway2-c", "base-sha-common", "expected-sha-refs/heads/feature-nway2-c")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-nway2-c) error = %v", err)
	}
	resolveOneConflict(featC.ID, "att-nway2-setup-c", "resolved-sha-nway2-c")

	// Both conflicts are now resolved. The real test attempt must not silently promote either
	// resolution alone.
	finalReq := run.AttemptRequest{
		ID:                "att-nway2-final",
		FeatureID:         featMain.ID,
		ParentRef:         featMain.ParentRef,
		BaseSHA:           featMain.BaseSHA,
		ExpectedParentSHA: featMain.ExpectedParentSHA,
		IdempotencyKey:    "idem-nway2-final",
		Owner:             "owner-nway2-final",
		Branches:          []string{"lucind/lane-nway2-final"},
	}
	finalRes, err := run.ExecuteAttempt(context.Background(), deps, finalReq)
	if err != nil {
		t.Fatalf("ExecuteAttempt(final) error = %v", err)
	}

	if finalRes.Status != run.AttemptStatusBlocked {
		t.Fatalf("finalRes.Status = %v, want %v (N-way resolution must block, not silently promote one side)", finalRes.Status, run.AttemptStatusBlocked)
	}
	if !strings.Contains(finalRes.FailureReason, featB.ID) || !strings.Contains(finalRes.FailureReason, featC.ID) {
		t.Errorf("finalRes.FailureReason = %q, want it to mention both %q and %q", finalRes.FailureReason, featB.ID, featC.ID)
	}

	expectedOriginalSHA := "candidate-sha-integrate-att-nway2-final"
	if finalRes.CandidateSHA != expectedOriginalSHA {
		t.Errorf("finalRes.CandidateSHA = %q, want %q (unchanged from the attempt's own original candidate -- neither resolution silently applied)", finalRes.CandidateSHA, expectedOriginalSHA)
	}
	if finalRes.CandidateSHA == "resolved-sha-nway2-b" || finalRes.CandidateSHA == "resolved-sha-nway2-c" {
		t.Errorf("finalRes.CandidateSHA = %q, must not silently adopt either single resolution", finalRes.CandidateSHA)
	}

	// Lease must have been released so a human can resolve and promote sequentially.
	if _, err := featSvc.AcquireLease(context.Background(), featMain.ID, "probe-owner", time.Second); err != nil {
		t.Errorf("AcquireLease() after N-way block error = %v, want lease to have been released", err)
	}
}

// newRequiredOverlapEvidence builds a ClassRequired overlap.Evidence with a fixed CreatedAt so its
// hash is deterministic across calls -- shared by the N-way-deadlock regression tests below.
func newRequiredOverlapEvidence(baseSHA, shaA, shaB string) *overlap.Evidence {
	ev := &overlap.Evidence{
		Version:     "v1",
		BaseSHA:     baseSHA,
		FeatureASHA: shaA,
		FeatureBSHA: shaB,
		Class:       overlap.ClassRequired,
		Rationale:   []string{"predicted Git merge conflict detected"},
		Signals: overlap.Signals{
			PredictedConflict: true,
			ConflictPaths:     []string{"conflict.go"},
		},
		CreatedAt: time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC),
	}
	h, _ := ev.ComputeHash()
	ev.Hash = h
	return ev
}

// setupResolvedConflictDirect registers an already-approved, already-integrated reconciliation
// resolution between featMain and otherFeat directly through reconcile.Service, without routing
// through run.ExecuteAttempt. Unlike the resolveOneConflict helper used by
// TestTwoRequiredConflictsBothResolvedBlocksNWayReconciliation, this does not require otherFeat to
// be the ONLY other active feature at setup time: it never runs an attempt, so it cannot be
// affected by (and cannot affect) any other already-active, still-unresolved feature. That
// isolation is exactly what the order-independence regression test below needs: it must be able to
// set up a resolved conflict for a feature that already coexists with other unresolved active
// features, or set one up before those other features even exist, without either path itself
// tripping the overlap gate.
//
// otherCurrentSHA must equal exactly what evaluateOverlapGate will itself resolve for otherFeat's
// tip (the gateSpies default ResolveRefSHA stub returns "expected-sha-"+ref for any ref), so the
// matchedOtherSHA == otherSHA check inside evaluateOverlapGate's reuse path succeeds.
func setupResolvedConflictDirect(t *testing.T, l *ledger.Ledger, reconcileSvc *reconcile.Service, featMain, otherFeat feature.Feature, otherCurrentSHA, resolvedSHA string) {
	t.Helper()

	ev := newRequiredOverlapEvidence("base-sha-common", "placeholder-source-sha", otherCurrentSHA)

	req, err := reconcileSvc.CreateRequest(context.Background(), reconcile.CreateRequestParams{
		FeatureID:     featMain.ID,
		SourceFeature: featMain.ID,
		SourceParent:  featMain.ParentRef,
		TargetFeature: otherFeat.ID,
		TargetParent:  otherFeat.ParentRef,
		SourceSHA:     "placeholder-source-sha",
		TargetSHA:     otherCurrentSHA,
		Evidence:      ev,
		TTL:           15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("reconcileSvc.CreateRequest(%s, %s) error = %v", featMain.ID, otherFeat.ID, err)
	}

	if _, _, err := reconcileSvc.Approve(context.Background(), reconcile.ApproveParams{
		RequestID:     req.ID,
		SourceFeature: featMain.ID,
		TargetFeature: otherFeat.ID,
		Actor:         "test-actor",
	}); err != nil {
		t.Fatalf("reconcileSvc.Approve(%s, %s) error = %v", featMain.ID, otherFeat.ID, err)
	}

	cand, err := l.ReconciliationCandidateByRequest(context.Background(), req.ID)
	if err != nil {
		t.Fatalf("ReconciliationCandidateByRequest(%s) error = %v", otherFeat.ID, err)
	}
	if _, err := reconcileSvc.UpdateCandidateStatus(context.Background(), cand.ID, reconcile.CandidateStatusIntegrated, resolvedSHA, ""); err != nil {
		t.Fatalf("reconcileSvc.UpdateCandidateStatus(%s) error = %v", otherFeat.ID, err)
	}
}

// TestThreeRequiredConflictsOneResolvedTwoUnresolvedOrderIndependent is the regression test for the
// N-way deadlock: with 3 simultaneously active features all in required overlap with att's feature,
// one resolved and two still unresolved, the attempt must block citing an unresolved feature --
// never silently succeed, and never mis-fire the "N-way reconciliation not supported" block (which
// only applies when the loop resolves 2+ conflicts in a single pass, not when 1 resolves and others
// remain unresolved).
//
// Critically, this must hold regardless of where in ActiveFeatures()' iteration order (created_at
// ASC) the resolved feature falls. Before this fix, evaluateOverlapGate returned immediately
// (`return true, att, nil`) on the FIRST unresolved required overlap it encountered, without ever
// evaluating the remaining active features. So if the resolved feature happened to sort BEFORE both
// unresolved ones, the old code still (by luck) evaluated it before hitting the first unresolved
// one and returning -- but if the resolved feature sorted AFTER even one unresolved feature, it was
// never reached at all, and its resolution was silently discarded for the pass. Both subtests below
// assert evalFunc was called exactly 3 times (once per other active feature): the old code called
// it only 1 or 2 times depending on ordering, because it returned before the loop completed. That
// call count, not just the block message, is what structurally distinguishes the fix from the bug:
// the message alone happens to read identically in both the old buggy and new fixed behavior for
// this exact pair of orderings, since either way the first unresolved feature encountered is the
// same (unresolved-1 sorts before unresolved-2 in both subtests) -- only the fixed code proves it
// got there by actually completing the loop.
func TestThreeRequiredConflictsOneResolvedTwoUnresolvedOrderIndependent(t *testing.T) {
	runCase := func(t *testing.T, resolvedFirst bool) {
		spies := &gateSpies{}
		deps, l, featSvc := newGateTestDeps(t, spies)
		reconcileSvc := reconcile.NewService(l, reconcile.WithClock(deps.Now))

		featMain, err := featSvc.Create(context.Background(), "feat-order-main", "refs/heads/feature-order-main-"+t.Name(), "base-sha-common", "expected-sha-refs/heads/feature-order-main-"+t.Name())
		if err != nil {
			t.Fatalf("featSvc.Create(feat-order-main) error = %v", err)
		}

		createOther := func(suffix string) feature.Feature {
			f, err := featSvc.Create(context.Background(), "feat-order-"+suffix, "refs/heads/feature-order-"+suffix+"-"+t.Name(), "base-sha-common", "expected-sha-refs/heads/feature-order-"+suffix+"-"+t.Name())
			if err != nil {
				t.Fatalf("featSvc.Create(feat-order-%s) error = %v", suffix, err)
			}
			return f
		}

		var featResolved, featUnresolved1, featUnresolved2 feature.Feature
		if resolvedFirst {
			featResolved = createOther("resolved")
			setupResolvedConflictDirect(t, l, reconcileSvc, featMain, featResolved, "expected-sha-"+featResolved.ParentRef, "resolved-sha-order")
			featUnresolved1 = createOther("unresolved-1")
			featUnresolved2 = createOther("unresolved-2")
		} else {
			featUnresolved1 = createOther("unresolved-1")
			featUnresolved2 = createOther("unresolved-2")
			featResolved = createOther("resolved")
			setupResolvedConflictDirect(t, l, reconcileSvc, featMain, featResolved, "expected-sha-"+featResolved.ParentRef, "resolved-sha-order")
		}

		type shaCall struct{ ShaB string }
		var calls []shaCall
		spies.evaluateOverlapFunc = func(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...overlap.EvaluateOption) (*overlap.Evidence, error) {
			calls = append(calls, shaCall{ShaB: shaB})
			return newRequiredOverlapEvidence(baseSHA, shaA, shaB), nil
		}

		req := run.AttemptRequest{
			ID:                "att-order-final-" + t.Name(),
			FeatureID:         featMain.ID,
			ParentRef:         featMain.ParentRef,
			BaseSHA:           featMain.BaseSHA,
			ExpectedParentSHA: featMain.ExpectedParentSHA,
			IdempotencyKey:    "idem-order-final-" + t.Name(),
			Owner:             "owner-order-final",
			Branches:          []string{"lucind/lane-order-final-" + t.Name()},
		}
		res, err := run.ExecuteAttempt(context.Background(), deps, req)
		if err != nil {
			t.Fatalf("ExecuteAttempt() error = %v", err)
		}

		if res.Status != run.AttemptStatusBlocked {
			t.Fatalf("res.Status = %v, want %v (an unresolved required overlap must still block, regardless of the resolved feature's position)", res.Status, run.AttemptStatusBlocked)
		}
		wantReason := "promotion blocked: reconciliation-required overlap with feature " + featUnresolved1.ID
		if res.FailureReason != wantReason {
			t.Errorf("res.FailureReason = %q, want %q (unresolved-1 sorts before unresolved-2 in both orderings, so it must be cited either way)", res.FailureReason, wantReason)
		}
		if strings.Contains(res.FailureReason, featResolved.ID) {
			t.Errorf("res.FailureReason = %q, must not cite the already-resolved feature %q", res.FailureReason, featResolved.ID)
		}

		if len(calls) != 3 {
			t.Fatalf("evalFunc called %d times, want 3 (one per other active feature -- the loop must not short-circuit on the first unresolved overlap)", len(calls))
		}

		expectedOriginalSHA := "candidate-sha-integrate-" + req.ID
		if res.CandidateSHA != expectedOriginalSHA {
			t.Errorf("res.CandidateSHA = %q, want %q (a blocked pass must not silently adopt the resolved override)", res.CandidateSHA, expectedOriginalSHA)
		}

		// One reconciliation_request already existed for featResolved (created during setup); a
		// fresh awaiting request must now also exist for EACH unresolved feature (proving the loop
		// reached and recorded both, not just the one it cited in FailureReason).
		allReqs, err := l.AllReconciliationRequests(context.Background())
		if err != nil {
			t.Fatalf("AllReconciliationRequests() error = %v", err)
		}
		if len(allReqs) != 3 {
			t.Fatalf("reconciliation_requests count = %d, want 3 (1 resolved + 2 newly created, one per unresolved feature)", len(allReqs))
		}
		var sawUnresolved2 bool
		for _, r := range allReqs {
			src, _, tgt, _ := reconcile.ParseDirection(r.Direction)
			if (src == featUnresolved2.ID || tgt == featUnresolved2.ID) && r.Status == string(reconcile.RequestStatusAwaiting) {
				sawUnresolved2 = true
			}
		}
		if !sawUnresolved2 {
			t.Errorf("no awaiting reconciliation_request found for %s, want the loop to have created one even though it was not the first unresolved feature cited", featUnresolved2.ID)
		}
	}

	t.Run("resolved_first", func(t *testing.T) { runCase(t, true) })
	t.Run("resolved_last", func(t *testing.T) { runCase(t, false) })
}

// TestThreeActiveFeaturesOneRemainingRequiredOverlapResolvedPromotes proves the fix's actual
// convergence path: once every OTHER required overlap has been resolved or no longer applies (e.g.
// content diverged, or an earlier sequential round already promoted past it), and exactly ONE
// required overlap remains for this pass -- already resolved -- promotion succeeds using that
// resolution, exactly as it would with only one other active feature. Two additional active
// features are present but produce no required overlap at all (overlap.ClassInformational),
// modeling "already promoted past in an earlier round" -- their mere presence in ActiveFeatures()
// must not affect the single-resolution promotion path.
func TestThreeActiveFeaturesOneRemainingRequiredOverlapResolvedPromotes(t *testing.T) {
	spies := &gateSpies{}
	deps, l, featSvc := newGateTestDeps(t, spies)
	reconcileSvc := reconcile.NewService(l, reconcile.WithClock(deps.Now))

	featMain, err := featSvc.Create(context.Background(), "feat-conv-main", "refs/heads/feature-conv-main", "base-sha-common", "expected-sha-refs/heads/feature-conv-main")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-conv-main) error = %v", err)
	}
	featClearA, err := featSvc.Create(context.Background(), "feat-conv-clear-a", "refs/heads/feature-conv-clear-a", "base-sha-common", "expected-sha-refs/heads/feature-conv-clear-a")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-conv-clear-a) error = %v", err)
	}
	featClearB, err := featSvc.Create(context.Background(), "feat-conv-clear-b", "refs/heads/feature-conv-clear-b", "base-sha-common", "expected-sha-refs/heads/feature-conv-clear-b")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-conv-clear-b) error = %v", err)
	}
	featConflict, err := featSvc.Create(context.Background(), "feat-conv-conflict", "refs/heads/feature-conv-conflict", "base-sha-common", "expected-sha-refs/heads/feature-conv-conflict")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-conv-conflict) error = %v", err)
	}

	const resolvedSHA = "resolved-sha-conv"
	setupResolvedConflictDirect(t, l, reconcileSvc, featMain, featConflict, "expected-sha-"+featConflict.ParentRef, resolvedSHA)

	spies.evaluateOverlapFunc = func(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...overlap.EvaluateOption) (*overlap.Evidence, error) {
		switch shaB {
		case "expected-sha-" + featConflict.ParentRef:
			return newRequiredOverlapEvidence(baseSHA, shaA, shaB), nil
		default:
			// featClearA and featClearB: no longer conflicting at all -- the "already promoted
			// past in an earlier round" case.
			return &overlap.Evidence{Version: "v1", BaseSHA: baseSHA, FeatureASHA: shaA, FeatureBSHA: shaB, Class: overlap.ClassInformational}, nil
		}
	}

	req := run.AttemptRequest{
		ID:                "att-conv-final",
		FeatureID:         featMain.ID,
		ParentRef:         featMain.ParentRef,
		BaseSHA:           featMain.BaseSHA,
		ExpectedParentSHA: featMain.ExpectedParentSHA,
		IdempotencyKey:    "idem-conv-final",
		Owner:             "owner-conv-final",
		Branches:          []string{"lucind/lane-conv-final"},
	}
	res, err := run.ExecuteAttempt(context.Background(), deps, req)
	if err != nil {
		t.Fatalf("ExecuteAttempt() error = %v", err)
	}

	if res.Status != run.AttemptStatusPromoted {
		t.Fatalf("res.Status = %v, want %v (the one remaining resolved required overlap must not be blocked by the other two clear features)", res.Status, run.AttemptStatusPromoted)
	}
	if res.CandidateSHA != resolvedSHA {
		t.Errorf("res.CandidateSHA = %q, want %q", res.CandidateSHA, resolvedSHA)
	}
	if len(spies.promoteCASCalls) != 1 || spies.promoteCASCalls[0].CandidateSHA != resolvedSHA {
		t.Errorf("promoteCASCalls = %+v, want exactly one call with candidate_sha %q", spies.promoteCASCalls, resolvedSHA)
	}

	_ = featClearA
	_ = featClearB
}

// TestThreeActiveFeaturesTwoResolvedZeroUnresolvedStillBlocksNWay proves the N-way safety limit is
// unchanged by this fix: with 0 unresolved required overlaps and 2 simultaneously resolved ones,
// the attempt still blocks explicitly rather than silently promoting either resolution -- even in
// the presence of a third, entirely non-conflicting active feature.
func TestThreeActiveFeaturesTwoResolvedZeroUnresolvedStillBlocksNWay(t *testing.T) {
	spies := &gateSpies{}
	deps, l, featSvc := newGateTestDeps(t, spies)
	reconcileSvc := reconcile.NewService(l, reconcile.WithClock(deps.Now))

	featMain, err := featSvc.Create(context.Background(), "feat-nway3-main", "refs/heads/feature-nway3-main", "base-sha-common", "expected-sha-refs/heads/feature-nway3-main")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-nway3-main) error = %v", err)
	}
	featClear, err := featSvc.Create(context.Background(), "feat-nway3-clear", "refs/heads/feature-nway3-clear", "base-sha-common", "expected-sha-refs/heads/feature-nway3-clear")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-nway3-clear) error = %v", err)
	}
	featB, err := featSvc.Create(context.Background(), "feat-nway3-b", "refs/heads/feature-nway3-b", "base-sha-common", "expected-sha-refs/heads/feature-nway3-b")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-nway3-b) error = %v", err)
	}
	featC, err := featSvc.Create(context.Background(), "feat-nway3-c", "refs/heads/feature-nway3-c", "base-sha-common", "expected-sha-refs/heads/feature-nway3-c")
	if err != nil {
		t.Fatalf("featSvc.Create(feat-nway3-c) error = %v", err)
	}

	setupResolvedConflictDirect(t, l, reconcileSvc, featMain, featB, "expected-sha-"+featB.ParentRef, "resolved-sha-nway3-b")
	setupResolvedConflictDirect(t, l, reconcileSvc, featMain, featC, "expected-sha-"+featC.ParentRef, "resolved-sha-nway3-c")

	spies.evaluateOverlapFunc = func(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...overlap.EvaluateOption) (*overlap.Evidence, error) {
		switch shaB {
		case "expected-sha-" + featB.ParentRef, "expected-sha-" + featC.ParentRef:
			return newRequiredOverlapEvidence(baseSHA, shaA, shaB), nil
		default:
			return &overlap.Evidence{Version: "v1", BaseSHA: baseSHA, FeatureASHA: shaA, FeatureBSHA: shaB, Class: overlap.ClassInformational}, nil
		}
	}

	req := run.AttemptRequest{
		ID:                "att-nway3-final",
		FeatureID:         featMain.ID,
		ParentRef:         featMain.ParentRef,
		BaseSHA:           featMain.BaseSHA,
		ExpectedParentSHA: featMain.ExpectedParentSHA,
		IdempotencyKey:    "idem-nway3-final",
		Owner:             "owner-nway3-final",
		Branches:          []string{"lucind/lane-nway3-final"},
	}
	res, err := run.ExecuteAttempt(context.Background(), deps, req)
	if err != nil {
		t.Fatalf("ExecuteAttempt() error = %v", err)
	}

	if res.Status != run.AttemptStatusBlocked {
		t.Fatalf("res.Status = %v, want %v (2 simultaneously resolved required overlaps must still block, unchanged)", res.Status, run.AttemptStatusBlocked)
	}
	if !strings.Contains(res.FailureReason, "N-way reconciliation not supported") {
		t.Errorf("res.FailureReason = %q, want it to mention the N-way block", res.FailureReason)
	}
	if !strings.Contains(res.FailureReason, featB.ID) || !strings.Contains(res.FailureReason, featC.ID) {
		t.Errorf("res.FailureReason = %q, want it to mention both %q and %q", res.FailureReason, featB.ID, featC.ID)
	}

	expectedOriginalSHA := "candidate-sha-integrate-att-nway3-final"
	if res.CandidateSHA != expectedOriginalSHA {
		t.Errorf("res.CandidateSHA = %q, want %q (unchanged -- neither resolution silently applied)", res.CandidateSHA, expectedOriginalSHA)
	}
	if len(spies.promoteCASCalls) != 0 {
		t.Errorf("promoteCASCalls = %+v, want none", spies.promoteCASCalls)
	}

	_ = featClear
}
