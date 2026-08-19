package run

import (
	"context"
	"fmt"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
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
// Governing rule: green integrates everything and removes only the worktrees
// that integrated; red integrates nothing and every offered lane returns to
// lane.Blocked with its worktree preserved.
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
		// Best-effort: discard the combined worktree before reverting lanes.
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

	laneWorktrees := make(map[string]string, len(batch.Lanes))
	for _, l := range batch.Lanes {
		laneWorktrees[l.LaneID] = l.Worktree
	}

	for _, id := range batch.Outcome.Integrate {
		wtPath := laneWorktrees[id]
		// Best-effort: clean up the integrated lane's worktree and branch, and
		// update worktree_preserved in the ledger.
		_ = deps.RemoveLaneWorktree(ctx, deps.PrimaryRoot, wtPath, worktree.BranchFor(id))
		_ = deps.Ledger.SetWorktreePreserved(ctx, deps.RunID, id, false)
	}

	// Best-effort: append one run-scoped summary event.
	_ = deps.Ledger.AppendEvent(ctx, ledger.Event{
		RunID:  deps.RunID,
		Type:   ledger.EventLaneNote,
		Detail: fmt.Sprintf("batch integrated: %d lane(s) integrated", len(batch.Outcome.Integrate)),
		At:     now,
	})

	return IntegrateReport{
		RunID:      deps.RunID,
		Attempted:  true,
		Passed:     true,
		Integrated: batch.Outcome.Integrate,
	}, nil
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
