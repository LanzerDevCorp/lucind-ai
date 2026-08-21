package integrate

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/reconcile"
	"github.com/LanzerDevCorp/lucind-ai/internal/resolve"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

// CandidateParams holds inputs for resolving and promoting an approved reconciliation candidate.
type CandidateParams struct {
	PrimaryRoot       string
	CandidateID       string
	RequestID         string
	SourceRef         string
	ExpectedSourceSHA string
	TargetRef         string
	ExpectedTargetSHA string
	AllowedPaths      []string
	Invoker           resolve.Invoker
	Timeout           time.Duration
	MaxConflictLines  int
	Runner            worktree.GitRunner
	ReconcileService  *reconcile.Service
	CheckFn           func(ctx context.Context, worktreePath string) (bool, string, error)
}

// CandidateResult represents the final outcome of resolving and promoting a candidate.
type CandidateResult struct {
	CandidateID   string
	Status        reconcile.CandidateStatus
	CandidateSHA  string
	FailureReason string
	Output        string
	Checks        string
	Promoted      bool
	WorktreePath  string
}

// ResolveAndPromoteCandidate executes the bounded Sonnet resolver against one approved direction candidate,
// runs mandatory checks (lucind-checks.sh), and advances the target parent ref by CAS only when all gates pass.
// Fails closed and preserves candidate worktree/evidence on any safety violation.
func ResolveAndPromoteCandidate(ctx context.Context, params CandidateParams) (CandidateResult, error) {
	runner := params.Runner
	if runner == nil {
		runner = worktree.DefaultGitRunner
	}

	checkFn := params.CheckFn
	if checkFn == nil {
		checkFn = Check
	}

	timeout := params.Timeout
	if timeout <= 0 {
		timeout = resolve.DefaultTimeout
	}

	maxConflictLines := params.MaxConflictLines
	if maxConflictLines <= 0 {
		maxConflictLines = resolve.MaxConflictLines
	}

	invoker := params.Invoker
	if invoker == nil {
		invoker = resolve.RealInvoker
	}

	// Pre-flight check: validate expected source and target refs have not changed
	currentSourceSHA, err := worktree.ResolveCommitSHA(ctx, runner, params.PrimaryRoot, params.SourceRef)
	if err != nil || currentSourceSHA != params.ExpectedSourceSHA {
		failureReason := fmt.Sprintf("expected source ref %s changed: expected %s, got %s", params.SourceRef, params.ExpectedSourceSHA, currentSourceSHA)
		if params.ReconcileService != nil && params.CandidateID != "" {
			_, _ = params.ReconcileService.UpdateCandidateStatus(ctx, params.CandidateID, reconcile.CandidateStatusStale, "", failureReason)
		}
		return CandidateResult{
			CandidateID:   params.CandidateID,
			Status:        reconcile.CandidateStatusStale,
			FailureReason: failureReason,
		}, nil
	}

	currentTargetSHA, err := worktree.ResolveCommitSHA(ctx, runner, params.PrimaryRoot, params.TargetRef)
	if err != nil || currentTargetSHA != params.ExpectedTargetSHA {
		failureReason := fmt.Sprintf("expected target ref %s changed: expected %s, got %s", params.TargetRef, params.ExpectedTargetSHA, currentTargetSHA)
		if params.ReconcileService != nil && params.CandidateID != "" {
			_, _ = params.ReconcileService.UpdateCandidateStatus(ctx, params.CandidateID, reconcile.CandidateStatusStale, "", failureReason)
		}
		return CandidateResult{
			CandidateID:   params.CandidateID,
			Status:        reconcile.CandidateStatusStale,
			FailureReason: failureReason,
		}, nil
	}

	// Create candidate linked worktree starting at expectedTargetSHA
	laneID := "reconcile-cand-" + params.CandidateID
	wt, err := worktree.CreateWithRunner(ctx, runner, params.PrimaryRoot, laneID, params.TargetRef, params.ExpectedTargetSHA)
	if err != nil {
		failureReason := fmt.Sprintf("create candidate worktree: %v", err)
		if params.ReconcileService != nil && params.CandidateID != "" {
			_, _ = params.ReconcileService.UpdateCandidateStatus(ctx, params.CandidateID, reconcile.CandidateStatusFailed, "", failureReason)
		}
		return CandidateResult{
			CandidateID:   params.CandidateID,
			Status:        reconcile.CandidateStatusFailed,
			FailureReason: failureReason,
		}, err
	}

	// Merge source into target in the candidate worktree
	mergeCmd := exec.CommandContext(ctx, "git", "-C", wt.Path, "merge", "--no-ff", params.ExpectedSourceSHA)
	mergeOut, mergeErr := mergeCmd.CombinedOutput()

	var resolveOutcome resolve.CandidateOutcome
	if mergeErr != nil {
		// Conflicted merge; execute bounded candidate resolver
		resolveOutcome, err = resolve.ResolveCandidateMerge(ctx, resolve.CandidateOptions{
			WorktreePath:     wt.Path,
			BaseSHA:          params.ExpectedTargetSHA,
			AllowedPaths:     params.AllowedPaths,
			Invoker:          invoker,
			Timeout:          timeout,
			MaxConflictLines: maxConflictLines,
		})
		if err != nil || !resolveOutcome.Resolved {
			failureReason := resolveOutcome.FailureReason
			if failureReason == "" && err != nil {
				failureReason = err.Error()
			}
			if failureReason == "" {
				failureReason = strings.TrimSpace(string(mergeOut))
			}
			if params.ReconcileService != nil && params.CandidateID != "" {
				_, _ = params.ReconcileService.UpdateCandidateStatus(ctx, params.CandidateID, reconcile.CandidateStatusFailed, "", failureReason)
			}
			// Preserve worktree and evidence for inspection on failure
			return CandidateResult{
				CandidateID:   params.CandidateID,
				Status:        reconcile.CandidateStatusFailed,
				Output:        resolveOutcome.Output,
				FailureReason: failureReason,
				WorktreePath:  wt.Path,
			}, nil
		}
	} else {
		// Clean merge; still enforce no markers and allowed_paths scope
		if hasMarkers, markerFiles, _ := resolve.ScanConflictMarkers(wt.Path); hasMarkers {
			failureReason := fmt.Sprintf("conflict markers remain in worktree: %s", strings.Join(markerFiles, ", "))
			if params.ReconcileService != nil && params.CandidateID != "" {
				_, _ = params.ReconcileService.UpdateCandidateStatus(ctx, params.CandidateID, reconcile.CandidateStatusFailed, "", failureReason)
			}
			return CandidateResult{
				CandidateID:   params.CandidateID,
				Status:        reconcile.CandidateStatusFailed,
				FailureReason: failureReason,
				WorktreePath:  wt.Path,
			}, nil
		}
		if offending, _ := resolve.EnforceAllowedPaths(ctx, wt.Path, params.ExpectedTargetSHA, params.AllowedPaths); len(offending) > 0 {
			failureReason := fmt.Sprintf("actual diff touched paths outside declared allowed_paths: %s", strings.Join(offending, ", "))
			if params.ReconcileService != nil && params.CandidateID != "" {
				_, _ = params.ReconcileService.UpdateCandidateStatus(ctx, params.CandidateID, reconcile.CandidateStatusFailed, "", failureReason)
			}
			return CandidateResult{
				CandidateID:   params.CandidateID,
				Status:        reconcile.CandidateStatusFailed,
				FailureReason: failureReason,
				WorktreePath:  wt.Path,
			}, nil
		}
	}

	// Run mandatory checks
	passed, checkOut, checkErr := checkFn(ctx, wt.Path)
	if !passed || checkErr != nil {
		failureReason := fmt.Sprintf("mandatory checks failed: %s", strings.TrimSpace(checkOut))
		if checkErr != nil && strings.TrimSpace(checkOut) == "" {
			failureReason = fmt.Sprintf("mandatory checks failed: %v", checkErr)
		}
		if params.ReconcileService != nil && params.CandidateID != "" {
			_, _ = params.ReconcileService.UpdateCandidateStatus(ctx, params.CandidateID, reconcile.CandidateStatusFailed, "", failureReason)
		}
		// Preserve candidate worktree for inspection on failure
		return CandidateResult{
			CandidateID:   params.CandidateID,
			Status:        reconcile.CandidateStatusFailed,
			Output:        resolveOutcome.Output,
			Checks:        checkOut,
			FailureReason: failureReason,
			WorktreePath:  wt.Path,
		}, nil
	}

	// Resolve candidate commit SHA
	candSHA, err := worktree.ResolveCommitSHA(ctx, runner, wt.Path, "HEAD")
	if err != nil {
		failureReason := fmt.Sprintf("resolve candidate commit SHA: %v", err)
		if params.ReconcileService != nil && params.CandidateID != "" {
			_, _ = params.ReconcileService.UpdateCandidateStatus(ctx, params.CandidateID, reconcile.CandidateStatusFailed, "", failureReason)
		}
		return CandidateResult{
			CandidateID:   params.CandidateID,
			Status:        reconcile.CandidateStatusFailed,
			FailureReason: failureReason,
			WorktreePath:  wt.Path,
		}, nil
	}

	// Re-validate both expected refs right before CAS promotion
	curSrc, _ := worktree.ResolveCommitSHA(ctx, runner, params.PrimaryRoot, params.SourceRef)
	curTgt, _ := worktree.ResolveCommitSHA(ctx, runner, params.PrimaryRoot, params.TargetRef)
	if curSrc != params.ExpectedSourceSHA {
		failureReason := fmt.Sprintf("expected source ref %s changed before CAS: expected %s, got %s", params.SourceRef, params.ExpectedSourceSHA, curSrc)
		if params.ReconcileService != nil && params.CandidateID != "" {
			_, _ = params.ReconcileService.UpdateCandidateStatus(ctx, params.CandidateID, reconcile.CandidateStatusStale, "", failureReason)
		}
		return CandidateResult{
			CandidateID:   params.CandidateID,
			Status:        reconcile.CandidateStatusStale,
			FailureReason: failureReason,
			WorktreePath:  wt.Path,
		}, nil
	}
	if curTgt != params.ExpectedTargetSHA {
		failureReason := fmt.Sprintf("expected target ref %s changed before CAS: expected %s, got %s", params.TargetRef, params.ExpectedTargetSHA, curTgt)
		if params.ReconcileService != nil && params.CandidateID != "" {
			_, _ = params.ReconcileService.UpdateCandidateStatus(ctx, params.CandidateID, reconcile.CandidateStatusStale, "", failureReason)
		}
		return CandidateResult{
			CandidateID:   params.CandidateID,
			Status:        reconcile.CandidateStatusStale,
			FailureReason: failureReason,
			WorktreePath:  wt.Path,
		}, nil
	}

	// Execute compare-and-swap (CAS) promotion
	if err := PromoteCASWithRunner(ctx, runner, params.PrimaryRoot, params.TargetRef, candSHA, params.ExpectedTargetSHA); err != nil {
		failureReason := fmt.Sprintf("CAS promotion failed: %v", err)
		if params.ReconcileService != nil && params.CandidateID != "" {
			_, _ = params.ReconcileService.UpdateCandidateStatus(ctx, params.CandidateID, reconcile.CandidateStatusStale, "", failureReason)
		}
		return CandidateResult{
			CandidateID:   params.CandidateID,
			Status:        reconcile.CandidateStatusStale,
			FailureReason: failureReason,
			WorktreePath:  wt.Path,
		}, nil
	}

	// Promotion succeeded! Update candidate to integrated status
	if params.ReconcileService != nil && params.CandidateID != "" {
		_, _ = params.ReconcileService.UpdateCandidateStatus(ctx, params.CandidateID, reconcile.CandidateStatusIntegrated, candSHA, "")
	}

	// Cleanup worktree and branch only after successful promotion
	_ = worktree.Remove(ctx, params.PrimaryRoot, wt.Path)
	_ = worktree.DeleteBranch(ctx, params.PrimaryRoot, wt.Branch)

	return CandidateResult{
		CandidateID:  params.CandidateID,
		Status:       reconcile.CandidateStatusIntegrated,
		CandidateSHA: candSHA,
		Output:       resolveOutcome.Output,
		Checks:       checkOut,
		Promoted:     true,
		WorktreePath: wt.Path,
	}, nil
}
