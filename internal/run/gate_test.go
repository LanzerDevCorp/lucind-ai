package run_test

import (
	"context"
	"io/fs"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/feature"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/overlap"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
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
	refSHAFunc func(ctx context.Context, primaryRoot, ref string) (string, error)
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
		CreateWorktree: func(ctx context.Context, primaryRoot, laneID string) (worktree.Worktree, error) {
			return worktree.Worktree{Path: primaryRoot + "/wt/" + laneID, Branch: "lucind/" + laneID, BaseSHA: "base-sha-common"}, nil
		},
		CombineTree: func(ctx context.Context, primaryRoot, runID string, branches []string) (string, string, error) {
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
