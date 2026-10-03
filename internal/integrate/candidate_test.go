package integrate_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/integrate"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/overlap"
	"github.com/LanzerDevCorp/lucind-ai/internal/reconcile"
)

// setupReconciliationRepo initializes a git repo with two feature branches and a shared base commit.
func setupReconciliationRepo(t *testing.T, checkScriptContent string, targetContent, sourceContent string) (repoRoot, targetSHA, sourceSHA string) {
	t.Helper()

	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "integrate-test@example.com")
	runGit(t, root, "config", "user.name", "integrate-test")

	// Base commit with shared file and checks script
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("line 1\nline 2\nline 3\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if checkScriptContent != "" {
		if err := os.WriteFile(filepath.Join(root, "lucind-checks.sh"), []byte(checkScriptContent), 0o755); err != nil {
			t.Fatalf("WriteFile checks error = %v", err)
		}
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "base commit")
	baseSHA := runGit(t, root, "rev-parse", "HEAD")

	// Target parent branch: feature-target
	runGit(t, root, "checkout", "-b", "feature-target")
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte(targetContent), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, root, "add", "file.txt")
	runGit(t, root, "commit", "-m", "feature-target commit")
	targetSHA = runGit(t, root, "rev-parse", "HEAD")

	// Source parent branch: feature-source off base
	runGit(t, root, "checkout", baseSHA)
	runGit(t, root, "checkout", "-b", "feature-source")
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte(sourceContent), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, root, "add", "file.txt")
	runGit(t, root, "commit", "-m", "feature-source commit")
	sourceSHA = runGit(t, root, "rev-parse", "HEAD")

	// Switch back to main
	runGit(t, root, "checkout", "main")

	return root, targetSHA, sourceSHA
}

func setupLedgerAndService(t *testing.T, repoRoot string) (*ledger.Ledger, *reconcile.Service) {
	t.Helper()
	l, err := ledger.Open(context.Background(), repoRoot)
	if err != nil {
		t.Fatalf("ledger.Open error = %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })
	svc := reconcile.NewService(l)
	return l, svc
}

func createApprovedCandidate(t *testing.T, ctx context.Context, svc *reconcile.Service, targetSHA, sourceSHA string, allowedPaths []string) (reconcile.Request, reconcile.Candidate) {
	t.Helper()

	ev := &overlap.Evidence{
		Version:     "v1",
		Class:       overlap.ClassRequired,
		BaseSHA:     "0000000000000000000000000000000000000000",
		FeatureASHA: sourceSHA,
		FeatureBSHA: targetSHA,
		CreatedAt:   time.Now().UTC(),
		Signals: overlap.Signals{
			PredictedConflict: true,
			ConflictPaths:     []string{"file.txt"},
		},
	}

	req, err := svc.CreateRequest(ctx, reconcile.CreateRequestParams{
		FeatureID:     "feature-target",
		SourceFeature: "feature-source",
		SourceParent:  "refs/heads/feature-source",
		TargetFeature: "feature-target",
		TargetParent:  "refs/heads/feature-target",
		SourceSHA:     sourceSHA,
		TargetSHA:     targetSHA,
		Evidence:      ev,
		TTL:           15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("CreateRequest error = %v", err)
	}

	req, cand, err := svc.Approve(ctx, reconcile.ApproveParams{
		RequestID:         req.ID,
		SourceFeature:     "feature-source",
		TargetFeature:     "feature-target",
		ExpectedSourceSHA: sourceSHA,
		ExpectedTargetSHA: targetSHA,
		Actor:             "local:test-user",
		AllowedPaths:      allowedPaths,
	})
	if err != nil {
		t.Fatalf("Approve error = %v", err)
	}

	return req, cand
}

func TestCandidateResolverHappyPathCASPromotion(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	ctx := context.Background()
	checksScript := "#!/bin/sh\nexit 0\n"
	repoRoot, targetSHA, sourceSHA := setupReconciliationRepo(t, checksScript, "target line 1\nline 2\nline 3\n", "line 1\nline 2\nsource line 3\n")
	_, svc := setupLedgerAndService(t, repoRoot)

	req, cand := createApprovedCandidate(t, ctx, svc, targetSHA, sourceSHA, []string{"file.txt"})

	result, err := integrate.ResolveAndPromoteCandidate(ctx, integrate.CandidateParams{
		PrimaryRoot:       repoRoot,
		CandidateID:       cand.ID,
		RequestID:         req.ID,
		SourceRef:         "refs/heads/feature-source",
		ExpectedSourceSHA: sourceSHA,
		TargetRef:         "refs/heads/feature-target",
		ExpectedTargetSHA: targetSHA,
		AllowedPaths:      cand.AllowedPaths,
		ReconcileService:  svc,
	})
	if err != nil {
		t.Fatalf("ResolveAndPromoteCandidate() error = %v, want nil", err)
	}

	if !result.Promoted {
		t.Fatalf("result.Promoted = false, want true; failure_reason = %s", result.FailureReason)
	}
	if result.Status != reconcile.CandidateStatusIntegrated {
		t.Errorf("result.Status = %q, want %q", result.Status, reconcile.CandidateStatusIntegrated)
	}
	if result.CandidateSHA == "" {
		t.Errorf("result.CandidateSHA is empty, want non-empty commit SHA")
	}

	// Verify target parent ref advanced to CandidateSHA via CAS
	targetHead := runGit(t, repoRoot, "rev-parse", "refs/heads/feature-target")
	if targetHead != result.CandidateSHA {
		t.Errorf("refs/heads/feature-target = %q, want %q", targetHead, result.CandidateSHA)
	}

	// Verify source parent ref is untouched
	sourceHead := runGit(t, repoRoot, "rev-parse", "refs/heads/feature-source")
	if sourceHead != sourceSHA {
		t.Errorf("refs/heads/feature-source moved from %q to %q, want untouched", sourceSHA, sourceHead)
	}

	// Verify candidate record in service / ledger is integrated
	updatedCand, err := svc.GetCandidate(ctx, cand.ID)
	if err != nil {
		t.Fatalf("GetCandidate error = %v", err)
	}
	if updatedCand.Status != reconcile.CandidateStatusIntegrated {
		t.Errorf("stored candidate status = %q, want %q", updatedCand.Status, reconcile.CandidateStatusIntegrated)
	}
	if updatedCand.CandidateSHA != result.CandidateSHA {
		t.Errorf("stored candidate SHA = %q, want %q", updatedCand.CandidateSHA, result.CandidateSHA)
	}
}

func TestCandidateResolverChecksFailingBlocksPromotionAndPreservesEvidence(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	ctx := context.Background()
	// Deliberately failing lucind-checks.sh
	checksScript := "#!/bin/sh\necho 'mandatory checks test failure' >&2\nexit 1\n"
	repoRoot, targetSHA, sourceSHA := setupReconciliationRepo(t, checksScript, "target line 1\nline 2\nline 3\n", "line 1\nline 2\nsource line 3\n")
	_, svc := setupLedgerAndService(t, repoRoot)

	req, cand := createApprovedCandidate(t, ctx, svc, targetSHA, sourceSHA, []string{"file.txt"})

	result, err := integrate.ResolveAndPromoteCandidate(ctx, integrate.CandidateParams{
		PrimaryRoot:       repoRoot,
		CandidateID:       cand.ID,
		RequestID:         req.ID,
		SourceRef:         "refs/heads/feature-source",
		ExpectedSourceSHA: sourceSHA,
		TargetRef:         "refs/heads/feature-target",
		ExpectedTargetSHA: targetSHA,
		AllowedPaths:      cand.AllowedPaths,
		ReconcileService:  svc,
	})
	if err != nil {
		t.Fatalf("ResolveAndPromoteCandidate() error = %v, want nil", err)
	}

	if result.Promoted {
		t.Fatalf("result.Promoted = true, want false when checks fail")
	}
	if result.Status != reconcile.CandidateStatusFailed {
		t.Errorf("result.Status = %q, want %q", result.Status, reconcile.CandidateStatusFailed)
	}
	if !strings.Contains(result.FailureReason, "mandatory checks failed") && !strings.Contains(result.FailureReason, "mandatory checks test failure") {
		t.Errorf("result.FailureReason = %q, want mention of check failure", result.FailureReason)
	}

	// Verify target parent ref was NOT updated
	targetHead := runGit(t, repoRoot, "rev-parse", "refs/heads/feature-target")
	if targetHead != targetSHA {
		t.Errorf("target ref mutated to %q, want %q", targetHead, targetSHA)
	}

	// Verify worktree and evidence are preserved (no cleanup on failure)
	if result.WorktreePath == "" {
		t.Fatalf("result.WorktreePath is empty")
	}
	if _, err := os.Stat(result.WorktreePath); err != nil {
		t.Errorf("worktree was deleted, want preserved for inspection: %v", err)
	}
}

func TestCandidateResolverConflictFailsCandidate(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	ctx := context.Background()
	checksScript := "#!/bin/sh\nexit 0\n"
	repoRoot, targetSHA, sourceSHA := setupReconciliationRepo(t, checksScript, "target line\n", "source line\n")
	_, svc := setupLedgerAndService(t, repoRoot)

	req, cand := createApprovedCandidate(t, ctx, svc, targetSHA, sourceSHA, []string{"file.txt"})

	result, err := integrate.ResolveAndPromoteCandidate(ctx, integrate.CandidateParams{
		PrimaryRoot:       repoRoot,
		CandidateID:       cand.ID,
		RequestID:         req.ID,
		SourceRef:         "refs/heads/feature-source",
		ExpectedSourceSHA: sourceSHA,
		TargetRef:         "refs/heads/feature-target",
		ExpectedTargetSHA: targetSHA,
		AllowedPaths:      cand.AllowedPaths,
		ReconcileService:  svc,
	})
	if err != nil {
		t.Fatalf("ResolveAndPromoteCandidate() error = %v, want nil", err)
	}

	if result.Promoted {
		t.Fatalf("result.Promoted = true, want false on conflict")
	}
	if result.Status != reconcile.CandidateStatusFailed {
		t.Errorf("result.Status = %q, want %q", result.Status, reconcile.CandidateStatusFailed)
	}
	if !strings.Contains(result.FailureReason, "CONFLICT") && !strings.Contains(result.FailureReason, "Automatic merge failed") {
		t.Errorf("result.FailureReason = %q, want conflict message", result.FailureReason)
	}

	// Verify target parent ref was NOT updated
	targetHead := runGit(t, repoRoot, "rev-parse", "refs/heads/feature-target")
	if targetHead != targetSHA {
		t.Errorf("target ref mutated to %q, want %q", targetHead, targetSHA)
	}
}


func TestCandidateResolverStaleExpectedRefsFailsClosed(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	ctx := context.Background()
	checksScript := "#!/bin/sh\nexit 0\n"

	// Case 1: Source ref has changed since approval
	t.Run("stale source ref", func(t *testing.T) {
		repoRoot, targetSHA, sourceSHA := setupReconciliationRepo(t, checksScript, "target line\n", "source line\n")
		_, svc := setupLedgerAndService(t, repoRoot)
		req, cand := createApprovedCandidate(t, ctx, svc, targetSHA, sourceSHA, []string{"file.txt"})

		// Advance feature-source out from under us
		runGit(t, repoRoot, "checkout", "feature-source")
		if err := os.WriteFile(filepath.Join(repoRoot, "other.txt"), []byte("other\n"), 0o644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}
		runGit(t, repoRoot, "add", "other.txt")
		runGit(t, repoRoot, "commit", "-m", "advance source")
		runGit(t, repoRoot, "checkout", "main")

		result, err := integrate.ResolveAndPromoteCandidate(ctx, integrate.CandidateParams{
			PrimaryRoot:       repoRoot,
			CandidateID:       cand.ID,
			RequestID:         req.ID,
			SourceRef:         "refs/heads/feature-source",
			ExpectedSourceSHA: sourceSHA, // stale SHA!
			TargetRef:         "refs/heads/feature-target",
			ExpectedTargetSHA: targetSHA,
			AllowedPaths:      cand.AllowedPaths,
			ReconcileService:  svc,
		})
		if err != nil {
			t.Fatalf("ResolveAndPromoteCandidate() error = %v, want nil", err)
		}

		if result.Promoted {
			t.Fatalf("result.Promoted = true, want false")
		}
		if result.Status != reconcile.CandidateStatusStale {
			t.Errorf("result.Status = %q, want %q", result.Status, reconcile.CandidateStatusStale)
		}
		if !strings.Contains(result.FailureReason, "source ref") {
			t.Errorf("result.FailureReason = %q, want mention of source ref", result.FailureReason)
		}
	})

	// Case 2: Target ref has changed since approval
	t.Run("stale target ref", func(t *testing.T) {
		repoRoot, targetSHA, sourceSHA := setupReconciliationRepo(t, checksScript, "target line\n", "source line\n")
		_, svc := setupLedgerAndService(t, repoRoot)
		req, cand := createApprovedCandidate(t, ctx, svc, targetSHA, sourceSHA, []string{"file.txt"})

		// Advance feature-target out from under us
		runGit(t, repoRoot, "checkout", "feature-target")
		if err := os.WriteFile(filepath.Join(repoRoot, "other2.txt"), []byte("other2\n"), 0o644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}
		runGit(t, repoRoot, "add", "other2.txt")
		runGit(t, repoRoot, "commit", "-m", "advance target")
		runGit(t, repoRoot, "checkout", "main")

		result, err := integrate.ResolveAndPromoteCandidate(ctx, integrate.CandidateParams{
			PrimaryRoot:       repoRoot,
			CandidateID:       cand.ID,
			RequestID:         req.ID,
			SourceRef:         "refs/heads/feature-source",
			ExpectedSourceSHA: sourceSHA,
			TargetRef:         "refs/heads/feature-target",
			ExpectedTargetSHA: targetSHA, // stale SHA!
			AllowedPaths:      cand.AllowedPaths,
			ReconcileService:  svc,
		})
		if err != nil {
			t.Fatalf("ResolveAndPromoteCandidate() error = %v, want nil", err)
		}

		if result.Promoted {
			t.Fatalf("result.Promoted = true, want false")
		}
		if result.Status != reconcile.CandidateStatusStale {
			t.Errorf("result.Status = %q, want %q", result.Status, reconcile.CandidateStatusStale)
		}
		if !strings.Contains(result.FailureReason, "target ref") {
			t.Errorf("result.FailureReason = %q, want mention of target ref", result.FailureReason)
		}
	})
}

