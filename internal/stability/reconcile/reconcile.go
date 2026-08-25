// Package reconcile implements crash inspection, fail-closed resume decisions,
// and idempotent abort cleanup for native stability campaigns.
//
// Authority: openspec/changes/native-stability-campaign/design.md Decision 5
// ("Domain Separation between Stability and Branch Reconcile").
// This package is strictly separate from internal/reconcile, which manages
// git branch-merge-conflict reconciliation requests, candidates, and direction approvals.
package reconcile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LanzerDevCorp/lucind-ai/internal/stability/process"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability/store"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

// Sentinel errors returned by reconcile methods.
var (
	ErrCannotResume               = errors.New("stability/reconcile: campaign cannot be resumed")
	ErrCleanupBlocked             = errors.New("stability/reconcile: cleanup blocked by unremovable residue")
	ErrEvidencePreservationFailed = errors.New("stability/reconcile: evidence preservation failed before cleanup")
	ErrCampaignNotFound           = errors.New("stability/reconcile: campaign not found")
)

// Decision represents the fail-closed resume evaluation outcome.
type Decision string

const (
	// DecisionSafe indicates that state is consistent, unambiguous, and safe to resume.
	DecisionSafe Decision = "safe"
	// DecisionFailClosed indicates that state has discrepancy or ambiguity; resumption is prohibited.
	DecisionFailClosed Decision = "fail_closed"
)

// DefaultStabilityLanes names the default ephemeral lane IDs used in stability campaign trials.
var DefaultStabilityLanes = []string{
	"stability-change-a",
	"stability-change-b",
	"stability-fix-a",
}

// WorktreePathFor computes the linked worktree path for laneID off primaryRoot:
// "<parent-of-primaryRoot>/<basename-of-primaryRoot>-worktrees/<laneID>".
func WorktreePathFor(primaryRoot, laneID string) string {
	parent := filepath.Dir(primaryRoot)
	base := filepath.Base(primaryRoot)
	return filepath.Join(parent, base+"-worktrees", laneID)
}

// WorktreeInspection records the on-disk and git state of one campaign lane's worktree.
type WorktreeInspection struct {
	LaneID           string `json:"lane_id"`
	Path             string `json:"path"`
	Branch           string `json:"branch"`
	Exists           bool   `json:"exists"`
	IsLinkedWorktree bool   `json:"is_linked_worktree"`
	Clean            bool   `json:"clean"`
	Error            string `json:"error,omitempty"`
}

// ProcessGroupInspection records the supervisor audit state of one process group.
type ProcessGroupInspection struct {
	PGID         int    `json:"pgid"`
	Survivors    []int  `json:"survivors,omitempty"`
	HasSurvivors bool   `json:"has_survivors"`
	Error        string `json:"error,omitempty"`
}

// InspectionReport holds the read-only inspection findings for a stability campaign.
type InspectionReport struct {
	Campaign      store.Campaign           `json:"campaign"`
	Worktrees     []WorktreeInspection     `json:"worktrees"`
	ProcessGroups []ProcessGroupInspection `json:"process_groups"`
	IsTerminal    bool                     `json:"is_terminal"`
	Ambiguities   []string                 `json:"ambiguities,omitempty"`
	Decision      Decision                 `json:"decision"`
	Reason        string                   `json:"reason"`
}

// InspectParams configures a read-only campaign inspection.
type InspectParams struct {
	Store         *store.Store
	CampaignID    string
	Campaign      *store.Campaign
	PrimaryRoot   string
	LaneIDs       []string
	ExpectedLanes []string
	ProcessGroups []int
	Runner        worktree.GitRunner
	Auditor       func(pgid int) ([]int, error)
}

// Inspect performs a read-only inspection of a campaign's persisted and on-disk/process state.
// It does not delete resources, kill processes, or mutate the store.
func Inspect(ctx context.Context, params InspectParams) (*InspectionReport, error) {
	var camp store.Campaign
	if params.Campaign != nil {
		camp = *params.Campaign
	} else if params.Store != nil {
		if params.CampaignID != "" {
			var err error
			camp, err = params.Store.GetCampaign(ctx, params.CampaignID)
			if err != nil {
				return nil, fmt.Errorf("stability/reconcile: get campaign %q: %w", params.CampaignID, err)
			}
		} else {
			var err error
			camp, err = params.Store.GetActiveCampaign(ctx)
			if err != nil {
				return nil, fmt.Errorf("stability/reconcile: get active campaign: %w", err)
			}
		}
	} else {
		return nil, errors.New("stability/reconcile: either Store or Campaign must be provided to Inspect")
	}

	isTerminal := camp.Status == store.StatusFailed ||
		camp.Status == store.StatusBlockedCleanup ||
		camp.Status == store.StatusPassed

	var ambiguities []string
	if isTerminal {
		ambiguities = append(ambiguities, fmt.Sprintf("campaign is in terminal or non-resumable status %q", camp.Status))
	}

	// Determine lanes to inspect
	lanesToInspect := params.LaneIDs
	if len(lanesToInspect) == 0 {
		if len(params.ExpectedLanes) > 0 {
			lanesToInspect = params.ExpectedLanes
		} else {
			lanesToInspect = DefaultStabilityLanes
		}
	}

	expectedMap := make(map[string]bool)
	for _, l := range params.ExpectedLanes {
		expectedMap[l] = true
	}

	var worktrees []WorktreeInspection
	for _, lane := range lanesToInspect {
		wtPath := WorktreePathFor(params.PrimaryRoot, lane)
		branch := worktree.BranchFor(lane)

		wi := WorktreeInspection{
			LaneID: lane,
			Path:   wtPath,
			Branch: branch,
		}

		info, err := os.Stat(wtPath)
		if err == nil && info.IsDir() {
			wi.Exists = true
			if worktree.IsLinkedWorktree(wtPath) {
				wi.IsLinkedWorktree = true
				clean, cleanErr := worktree.PorcelainEmpty(ctx, wtPath)
				if cleanErr != nil {
					wi.Clean = false
					wi.Error = cleanErr.Error()
					ambiguities = append(ambiguities, fmt.Sprintf("failed to check working tree cleanliness at %s: %v", wtPath, cleanErr))
				} else {
					wi.Clean = clean
					if !clean {
						ambiguities = append(ambiguities, fmt.Sprintf("worktree at %s has uncommitted modifications", wtPath))
					}
				}
			} else {
				wi.IsLinkedWorktree = false
				wi.Clean = false
				ambiguities = append(ambiguities, fmt.Sprintf("worktree directory %s exists but is not a valid linked git worktree", wtPath))
			}
		} else if err != nil && !os.IsNotExist(err) {
			wi.Error = err.Error()
			ambiguities = append(ambiguities, fmt.Sprintf("failed to stat worktree path %s: %v", wtPath, err))
		} else {
			// Not exists
			wi.Exists = false
			if expectedMap[lane] {
				ambiguities = append(ambiguities, fmt.Sprintf("expected worktree lane %s is missing from disk at %s", lane, wtPath))
			}
		}

		worktrees = append(worktrees, wi)
	}

	// Audit process groups
	auditor := params.Auditor
	if auditor == nil {
		auditor = process.AuditSurvivors
	}

	var procGroups []ProcessGroupInspection
	for _, pgid := range params.ProcessGroups {
		pgi := ProcessGroupInspection{
			PGID: pgid,
		}
		survivors, err := auditor(pgid)
		if err != nil {
			pgi.Error = err.Error()
			ambiguities = append(ambiguities, fmt.Sprintf("failed to audit process group %d: %v", pgid, err))
		} else {
			pgi.Survivors = survivors
			pgi.HasSurvivors = len(survivors) > 0
			if pgi.HasSurvivors {
				ambiguities = append(ambiguities, fmt.Sprintf("process group %d has %d live surviving process(es): %v", pgid, len(survivors), survivors))
			}
		}
		procGroups = append(procGroups, pgi)
	}

	report := &InspectionReport{
		Campaign:      camp,
		Worktrees:     worktrees,
		ProcessGroups: procGroups,
		IsTerminal:    isTerminal,
		Ambiguities:   ambiguities,
	}

	report.Decision, report.Reason = DecideResume(report)
	return report, nil
}

// DecideResume evaluates an InspectionReport and returns whether resumption is safe
// or must fail closed. Ambiguity always resolves to fail_closed.
func DecideResume(report *InspectionReport) (Decision, string) {
	if report == nil {
		return DecisionFailClosed, "inspection report is nil"
	}

	if report.Campaign.Status != store.StatusRunning {
		return DecisionFailClosed, fmt.Sprintf("campaign %s is in non-running status %q", report.Campaign.ID, report.Campaign.Status)
	}

	if report.IsTerminal {
		return DecisionFailClosed, fmt.Sprintf("campaign %s is in terminal status", report.Campaign.ID)
	}

	for _, pg := range report.ProcessGroups {
		if pg.Error != "" {
			return DecisionFailClosed, fmt.Sprintf("process group %d inspection error: %s", pg.PGID, pg.Error)
		}
		if pg.HasSurvivors {
			return DecisionFailClosed, fmt.Sprintf("process group %d has surviving processes: %v", pg.PGID, pg.Survivors)
		}
	}

	for _, wt := range report.Worktrees {
		if wt.Error != "" {
			return DecisionFailClosed, fmt.Sprintf("worktree for lane %s inspection error: %s", wt.LaneID, wt.Error)
		}
		if wt.Exists && !wt.IsLinkedWorktree {
			return DecisionFailClosed, fmt.Sprintf("worktree directory for lane %s is not a linked git worktree", wt.LaneID)
		}
		if wt.Exists && !wt.Clean {
			return DecisionFailClosed, fmt.Sprintf("worktree for lane %s has uncommitted modifications", wt.LaneID)
		}
	}

	if len(report.Ambiguities) > 0 {
		return DecisionFailClosed, fmt.Sprintf("ambiguity detected: %s", strings.Join(report.Ambiguities, "; "))
	}

	return DecisionSafe, "state is consistent and safe to resume"
}

// AbortParams configures campaign abort cleanup.
type AbortParams struct {
	Store            *store.Store
	CampaignID       string
	Campaign         *store.Campaign
	PrimaryRoot      string
	LaneIDs          []string
	ProcessGroups    []int
	Runner           worktree.GitRunner
	Killer           func(pgid int) error
	Auditor          func(pgid int) ([]int, error)
	PrePurgeEvidence func(ctx context.Context) error
}

// AbortResult summarizes the actions taken during abort cleanup.
type AbortResult struct {
	CampaignID       string           `json:"campaign_id"`
	InitialStatus    store.Status     `json:"initial_status"`
	FinalStatus      store.Status     `json:"final_status"`
	CleanedWorktrees []string         `json:"cleaned_worktrees,omitempty"`
	SkippedWorktrees []string         `json:"skipped_worktrees,omitempty"`
	FailedWorktrees  map[string]error `json:"failed_worktrees,omitempty"`
	CleanedBranches  []string         `json:"cleaned_branches,omitempty"`
	SkippedBranches  []string         `json:"skipped_branches,omitempty"`
	FailedBranches   map[string]error `json:"failed_branches,omitempty"`
	KilledProcesses  []int            `json:"killed_processes,omitempty"`
	FailedProcesses  map[int]error    `json:"failed_processes,omitempty"`
	NoOp             bool             `json:"no_op"`
}

// Abort idempotently cleans up all processes, worktrees, and branches belonging to a Campaign.
// If any cleanup step encounters unremovable residue, the Campaign transitions to store.StatusBlockedCleanup.
// If all cleanup steps succeed, the Campaign transitions to store.StatusFailed.
// Calling Abort repeatedly on an already-cleaned campaign is an idempotent no-op success.
func Abort(ctx context.Context, params AbortParams) (*AbortResult, error) {
	var camp store.Campaign
	if params.Campaign != nil {
		camp = *params.Campaign
	} else if params.Store != nil {
		if params.CampaignID != "" {
			var err error
			camp, err = params.Store.GetCampaign(ctx, params.CampaignID)
			if err != nil {
				return nil, fmt.Errorf("stability/reconcile: get campaign %q: %w", params.CampaignID, err)
			}
		} else {
			var err error
			camp, err = params.Store.GetActiveCampaign(ctx)
			if err != nil {
				return nil, fmt.Errorf("stability/reconcile: get active campaign: %w", err)
			}
		}
	} else {
		return nil, errors.New("stability/reconcile: either Store or Campaign must be provided to Abort")
	}

	res := &AbortResult{
		CampaignID:      camp.ID,
		InitialStatus:   camp.Status,
		FinalStatus:     camp.Status,
		FailedWorktrees: make(map[string]error),
		FailedBranches:  make(map[string]error),
		FailedProcesses: make(map[int]error),
	}

	// Q74: Preserve evidence before deleting infrastructure if hook provided.
	if params.PrePurgeEvidence != nil {
		if err := params.PrePurgeEvidence(ctx); err != nil {
			if params.Store != nil && camp.Status != store.StatusBlockedCleanup {
				_ = params.Store.UpdateCampaignStatus(ctx, camp.ID, store.StatusBlockedCleanup)
			}
			res.FinalStatus = store.StatusBlockedCleanup
			return res, fmt.Errorf("%w: %v", ErrEvidencePreservationFailed, err)
		}
	}

	lanesToClean := params.LaneIDs
	if len(lanesToClean) == 0 {
		lanesToClean = DefaultStabilityLanes
	}

	// Check if already terminal with zero residue on disk/process table (idempotent no-op check)
	if camp.Status == store.StatusFailed || camp.Status == store.StatusPassed {
		hasResidue := false
		for _, lane := range lanesToClean {
			wtPath := WorktreePathFor(params.PrimaryRoot, lane)
			if _, err := os.Stat(wtPath); err == nil {
				hasResidue = true
				break
			}
		}
		if !hasResidue && len(params.ProcessGroups) == 0 {
			res.NoOp = true
			res.FinalStatus = camp.Status
			return res, nil
		}
	}

	// 1. Terminate process groups
	killer := params.Killer
	if killer == nil {
		killer = process.KillGroup
	}
	auditor := params.Auditor
	if auditor == nil {
		auditor = process.AuditSurvivors
	}

	for _, pgid := range params.ProcessGroups {
		if err := killer(pgid); err != nil {
			res.FailedProcesses[pgid] = err
			continue
		}
		survivors, err := auditor(pgid)
		if err != nil {
			res.FailedProcesses[pgid] = err
		} else if len(survivors) > 0 {
			res.FailedProcesses[pgid] = fmt.Errorf("process group %d has %d survivor(s): %v", pgid, len(survivors), survivors)
		} else {
			res.KilledProcesses = append(res.KilledProcesses, pgid)
		}
	}

	// 2. Remove worktrees and delete branches
	for _, lane := range lanesToClean {
		wtPath := WorktreePathFor(params.PrimaryRoot, lane)
		branch := worktree.BranchFor(lane)

		// Check if worktree exists on disk
		if _, err := os.Stat(wtPath); err == nil {
			// Worktree exists on disk: attempt removal
			if err := worktree.Remove(ctx, params.PrimaryRoot, wtPath); err != nil {
				res.FailedWorktrees[lane] = err
			} else {
				res.CleanedWorktrees = append(res.CleanedWorktrees, lane)
			}
		} else if os.IsNotExist(err) {
			// Already cleaned / not present
			res.SkippedWorktrees = append(res.SkippedWorktrees, lane)
		} else {
			res.FailedWorktrees[lane] = fmt.Errorf("stat %s: %w", wtPath, err)
		}

		// Only attempt branch deletion if worktree removal didn't fail
		if _, failed := res.FailedWorktrees[lane]; !failed {
			if err := worktree.DeleteBranch(ctx, params.PrimaryRoot, branch); err != nil {
				errStr := strings.ToLower(err.Error())
				if strings.Contains(errStr, "not found") || strings.Contains(errStr, "not a valid branch name") {
					// Branch already deleted or never created
					res.SkippedBranches = append(res.SkippedBranches, branch)
				} else {
					res.FailedBranches[branch] = err
				}
			} else {
				res.CleanedBranches = append(res.CleanedBranches, branch)
			}
		}
	}

	// 3. Evaluate results and update store status
	hasFailures := len(res.FailedWorktrees) > 0 || len(res.FailedBranches) > 0 || len(res.FailedProcesses) > 0

	if hasFailures {
		res.FinalStatus = store.StatusBlockedCleanup
		if params.Store != nil && camp.Status != store.StatusBlockedCleanup {
			if err := params.Store.UpdateCampaignStatus(ctx, camp.ID, store.StatusBlockedCleanup); err != nil {
				return res, fmt.Errorf("stability/reconcile: update status to %s: %w", store.StatusBlockedCleanup, err)
			}
		}
		return res, fmt.Errorf("%w: failed to purge all resources (%d worktrees, %d branches, %d processes failed)",
			ErrCleanupBlocked, len(res.FailedWorktrees), len(res.FailedBranches), len(res.FailedProcesses))
	}

	// All resources cleaned successfully
	res.FinalStatus = store.StatusFailed
	if params.Store != nil && camp.Status != store.StatusFailed {
		if err := params.Store.UpdateCampaignStatus(ctx, camp.ID, store.StatusFailed); err != nil {
			return res, fmt.Errorf("stability/reconcile: update status to %s: %w", store.StatusFailed, err)
		}
	}

	return res, nil
}
