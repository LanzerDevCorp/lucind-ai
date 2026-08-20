package run

import (
	"context"
	"fmt"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

// IntegrateReport is the outcome of running Integrate on a completed batch.
type IntegrateReport struct {
	RunID      string
	Attempted  bool     // false when there was nothing to integrate
	Passed     bool     // meaningful only when Attempted is true
	Integrated []string // lane IDs promoted into primaryRoot; their worktrees were removed
	Reverted   []string // lane IDs returned to lane.Blocked; worktrees preserved
	Reason     string   // populated when Passed is false
}

// Integrate merges completed lane branches into a combined tree, runs
// verification checks against it, and promotes clean results into the
// primary repository.
//
// When the full batch fails (merge conflict or failing checks), it isolates
// the clean subset via bisection, promotes that subset, and reverts only
// the remaining lanes.
func Integrate(ctx context.Context, deps Deps, batch BatchReport) (IntegrateReport, error) {
	if !batch.Released || len(batch.Outcome.Integrate) == 0 {
		return IntegrateReport{
			RunID: deps.RunID,
		}, nil
	}

	now := deps.Now()

	branches := make([]string, len(batch.Outcome.Integrate))
	for i, id := range batch.Outcome.Integrate {
		branches[i] = worktree.BranchFor(id)
	}

	worktreePath, branchName, err := deps.CombineTree(ctx, deps.PrimaryRoot, deps.RunID, branches)
	if err != nil {
		return handleRedBatch(ctx, deps, batch, err.Error(), now)
	}

	passed, output, checkErr := deps.RunChecks(ctx, worktreePath)
	if checkErr != nil || !passed {
		// Best-effort: discard the combined worktree before bisecting.
		_ = deps.DiscardCombined(ctx, deps.PrimaryRoot, worktreePath, branchName)

		reason := output
		if checkErr != nil {
			reason = checkErr.Error()
		}
		return handleRedBatch(ctx, deps, batch, reason, now)
	}

	if err := deps.PromoteTarget(ctx, deps.PrimaryRoot, branchName); err != nil {
		// Best-effort: discard the combined worktree before reverting lanes.
		_ = deps.DiscardCombined(ctx, deps.PrimaryRoot, worktreePath, branchName)

		revertLanes(ctx, deps, batch.Outcome.Integrate, err.Error(), now)
		return IntegrateReport{
			RunID:     deps.RunID,
			Attempted: true,
			Passed:    false,
			Reverted:  batch.Outcome.Integrate,
			Reason:    err.Error(),
		}, nil
	}

	// Best-effort: the combined worktree and branch are now redundant since
	// primaryRoot has the same tip.
	_ = deps.DiscardCombined(ctx, deps.PrimaryRoot, worktreePath, branchName)

	return completeIntegration(ctx, deps, batch, batch.Outcome.Integrate, nil, now)
}

func handleRedBatch(ctx context.Context, deps Deps, batch BatchReport, triggerReason string, now time.Time) (IntegrateReport, error) {
	integrateIDs, revertIDs := bisect(ctx, deps, batch.Outcome.Integrate)
	if len(integrateIDs) == 0 {
		revertReason := "bisection found no viable subset"
		if triggerReason != "" {
			revertReason = fmt.Sprintf("bisection found no viable subset: %s", triggerReason)
		}
		revertLanes(ctx, deps, batch.Outcome.Integrate, revertReason, now)
		return IntegrateReport{
			RunID:     deps.RunID,
			Attempted: true,
			Passed:    false,
			Reverted:  batch.Outcome.Integrate,
			Reason:    revertReason,
		}, nil
	}

	branches := make([]string, len(integrateIDs))
	for i, id := range integrateIDs {
		branches[i] = worktree.BranchFor(id)
	}

	worktreePath, branchName, err := deps.CombineTree(ctx, deps.PrimaryRoot, deps.RunID, branches)
	if err != nil {
		revertLanes(ctx, deps, batch.Outcome.Integrate, err.Error(), now)
		return IntegrateReport{
			RunID:     deps.RunID,
			Attempted: true,
			Passed:    false,
			Reverted:  batch.Outcome.Integrate,
			Reason:    err.Error(),
		}, nil
	}

	passed, output, checkErr := deps.RunChecks(ctx, worktreePath)
	if checkErr != nil || !passed {
		_ = deps.DiscardCombined(ctx, deps.PrimaryRoot, worktreePath, branchName)
		reason := output
		if checkErr != nil {
			reason = checkErr.Error()
		}
		revertLanes(ctx, deps, batch.Outcome.Integrate, reason, now)
		return IntegrateReport{
			RunID:     deps.RunID,
			Attempted: true,
			Passed:    false,
			Reverted:  batch.Outcome.Integrate,
			Reason:    reason,
		}, nil
	}

	if err := deps.PromoteTarget(ctx, deps.PrimaryRoot, branchName); err != nil {
		_ = deps.DiscardCombined(ctx, deps.PrimaryRoot, worktreePath, branchName)
		revertLanes(ctx, deps, batch.Outcome.Integrate, err.Error(), now)
		return IntegrateReport{
			RunID:     deps.RunID,
			Attempted: true,
			Passed:    false,
			Reverted:  batch.Outcome.Integrate,
			Reason:    err.Error(),
		}, nil
	}

	_ = deps.DiscardCombined(ctx, deps.PrimaryRoot, worktreePath, branchName)

	return completeIntegration(ctx, deps, batch, integrateIDs, revertIDs, now)
}

func completeIntegration(ctx context.Context, deps Deps, batch BatchReport, integrateIDs, revertIDs []string, now time.Time) (IntegrateReport, error) {
	laneWorktrees := make(map[string]string, len(batch.Lanes))
	laneEnvelopes := make(map[string]*result.Envelope, len(batch.Lanes))
	for _, l := range batch.Lanes {
		laneWorktrees[l.LaneID] = l.Worktree
		laneEnvelopes[l.LaneID] = l.Envelope
	}

	for _, id := range integrateIDs {
		wtPath := laneWorktrees[id]
		_ = deps.PersistEnvelope(ctx, deps.PrimaryRoot, id, laneEnvelopes[id])
		_ = deps.RemoveLaneWorktree(ctx, deps.PrimaryRoot, wtPath, worktree.BranchFor(id))
		_ = deps.Ledger.SetWorktreePreserved(ctx, deps.RunID, id, false)
	}

	if len(revertIDs) > 0 {
		revertLanes(ctx, deps, revertIDs, "bisected out of batch", now)
	}

	_ = deps.Ledger.AppendEvent(ctx, ledger.Event{
		RunID:  deps.RunID,
		Type:   ledger.EventLaneNote,
		Detail: fmt.Sprintf("batch integrated: %d lane(s) integrated", len(integrateIDs)),
		At:     now,
	})

	return IntegrateReport{
		RunID:      deps.RunID,
		Attempted:  true,
		Passed:     true,
		Integrated: integrateIDs,
		Reverted:   revertIDs,
	}, nil
}

// bisect recursively partitions lanes to find the subset that cleanly combines and passes checks.
func bisect(ctx context.Context, deps Deps, lanes []string) (integrate, revert []string) {
	if len(lanes) == 1 {
		return nil, lanes
	}

	mid := len(lanes) / 2
	left := lanes[:mid]
	right := lanes[mid:]

	leftGreen := tryCombine(ctx, deps, left)
	rightGreen := tryCombine(ctx, deps, right)

	if leftGreen && rightGreen {
		return nil, lanes
	}

	if leftGreen && !rightGreen {
		rInt, rRev := bisect(ctx, deps, right)
		if len(rInt) == 0 {
			return left, rRev
		}
		candidate := make([]string, 0, len(left)+len(rInt))
		candidate = append(candidate, left...)
		candidate = append(candidate, rInt...)
		if tryCombine(ctx, deps, candidate) {
			return candidate, rRev
		}
		return nil, lanes
	}

	if !leftGreen && rightGreen {
		lInt, lRev := bisect(ctx, deps, left)
		if len(lInt) == 0 {
			return right, lRev
		}
		candidate := make([]string, 0, len(lInt)+len(right))
		candidate = append(candidate, lInt...)
		candidate = append(candidate, right...)
		if tryCombine(ctx, deps, candidate) {
			return candidate, lRev
		}
		return nil, lanes
	}

	// Both red
	lInt, lRev := bisect(ctx, deps, left)
	rInt, rRev := bisect(ctx, deps, right)

	revert = make([]string, 0, len(lRev)+len(rRev))
	revert = append(revert, lRev...)
	revert = append(revert, rRev...)

	if len(lInt) == 0 || len(rInt) == 0 {
		integrate = make([]string, 0, len(lInt)+len(rInt))
		integrate = append(integrate, lInt...)
		integrate = append(integrate, rInt...)
		return integrate, revert
	}

	candidate := make([]string, 0, len(lInt)+len(rInt))
	candidate = append(candidate, lInt...)
	candidate = append(candidate, rInt...)
	if tryCombine(ctx, deps, candidate) {
		return candidate, revert
	}
	return nil, lanes
}

// tryCombine tests whether a subset of lane IDs can combine cleanly and pass checks.
// It always cleans up the combined worktree on completion.
func tryCombine(ctx context.Context, deps Deps, laneIDs []string) bool {
	branches := make([]string, len(laneIDs))
	for i, id := range laneIDs {
		branches[i] = worktree.BranchFor(id)
	}

	worktreePath, branchName, err := deps.CombineTree(ctx, deps.PrimaryRoot, deps.RunID, branches)
	if err != nil {
		return false
	}
	defer func() {
		_ = deps.DiscardCombined(ctx, deps.PrimaryRoot, worktreePath, branchName)
	}()

	passed, _, checkErr := deps.RunChecks(ctx, worktreePath)
	if checkErr != nil || !passed {
		return false
	}
	return true
}

// revertLanes demotes every lane in laneIDs to lane.Blocked, marks its worktree
// preserved, and appends diagnostic lane and run notes.
func revertLanes(ctx context.Context, deps Deps, laneIDs []string, reason string, now time.Time) {
	for _, id := range laneIDs {
		// Best-effort: demote lane to Blocked, preserve worktree, and record reason.
		_ = deps.Ledger.SetStatus(ctx, deps.RunID, id, lane.Blocked, now)
		_ = deps.Ledger.SetWorktreePreserved(ctx, deps.RunID, id, true)
		_ = deps.Ledger.AppendEvent(ctx, ledger.Event{
			RunID:  deps.RunID,
			LaneID: id,
			Type:   ledger.EventLaneNote,
			Detail: reason,
			At:     now,
		})
	}

	// Best-effort: append one run-scoped summary event.
	_ = deps.Ledger.AppendEvent(ctx, ledger.Event{
		RunID:  deps.RunID,
		Type:   ledger.EventLaneNote,
		Detail: fmt.Sprintf("integration failed: %s", reason),
		At:     now,
	})
}
