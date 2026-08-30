package run

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/LanzerDevCorp/lucind-ai/internal/barrier"
	"github.com/LanzerDevCorp/lucind-ai/internal/feature"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
)

// ErrNoRetryCandidates is returned by RebuildBatchForRetry when no lane in
// runID qualifies for a retry: a lane only qualifies when its worktree is
// still preserved on disk and its own persisted result envelope reports
// status "done". A lane that never finished its own work (genuinely
// blocked, deviated, or failed on its own) is never included.
var ErrNoRetryCandidates = errors.New("run: no retryable lanes found for this run (a lane must have a preserved worktree and a \"done\" result envelope)")

// ErrLaneNotRetryable is returned by RebuildBatchForRetry when an
// explicitly requested lane ID does not qualify for retry -- e.g. its
// worktree was already cleaned up, or its own result envelope was never
// "done".
type ErrLaneNotRetryable struct {
	LaneID string
	Reason string
}

func (e *ErrLaneNotRetryable) Error() string {
	return fmt.Sprintf("run: lane %q is not retryable: %s", e.LaneID, e.Reason)
}

// RebuildBatchForRetry reconstructs a BatchReport for runID directly from
// durable state -- the ledger's lane rows and each lane's own preserved
// worktree -- without redispatching anything through an executor.
//
// This is what decouples "integrate" from "dispatch": when Integrate or
// IntegrateFeature revert a batch because the combine/check/promote step
// itself failed (e.g. the base was red, unrelated to the lanes'
// own work), every reverted lane's worktree, branch, and on-disk result
// envelope are left exactly as the lane itself left them -- see
// revertLanes and completeIntegration. RebuildBatchForRetry reads that
// preserved state back into the same BatchReport shape ExecuteBatch would
// have produced, so a caller can feed it straight back into Integrate or
// IntegrateFeature once the base is fixed, instead of burning a fresh AI
// dispatch to regenerate work that already reached "done" and already
// passed its own done criteria.
//
// If laneIDs is non-empty, only those lanes are considered, and every one
// of them must qualify or RebuildBatchForRetry fails closed with
// *ErrLaneNotRetryable naming which lane and why -- an explicit request
// is never silently dropped. If laneIDs is empty, every qualifying lane
// for runID is included automatically, and a lane that does not qualify
// is skipped rather than failing the whole rebuild.
func RebuildBatchForRetry(ctx context.Context, deps Deps, runID string, laneIDs []string) (BatchReport, error) {
	rows, err := deps.Ledger.Lanes(ctx, runID)
	if err != nil {
		return BatchReport{}, fmt.Errorf("run: rebuild batch for retry: list lanes: %w", err)
	}

	byID := make(map[string]ledger.Lane, len(rows))
	for _, row := range rows {
		byID[row.LaneID] = row
	}

	explicit := len(laneIDs) > 0
	candidates := laneIDs
	if !explicit {
		candidates = make([]string, 0, len(rows))
		for _, row := range rows {
			candidates = append(candidates, row.LaneID)
		}
		sort.Strings(candidates)
	}

	var (
		reports []Report
		include []string
	)
	for _, id := range candidates {
		row, ok := byID[id]
		if !ok {
			if explicit {
				return BatchReport{}, &ErrLaneNotRetryable{LaneID: id, Reason: fmt.Sprintf("no such lane in run %q", runID)}
			}
			continue
		}
		if !row.WorktreePreserved || row.WorktreePath == "" {
			if explicit {
				return BatchReport{}, &ErrLaneNotRetryable{LaneID: id, Reason: "worktree is not preserved (already cleaned up, or the lane was never reverted)"}
			}
			continue
		}

		fsys := deps.WorktreeFS(row.WorktreePath)
		envelope, envErr := result.Read(fsys, resultEnvelopePath)
		if envErr != nil {
			if explicit {
				return BatchReport{}, &ErrLaneNotRetryable{LaneID: id, Reason: fmt.Sprintf("result envelope unreadable: %v", envErr)}
			}
			continue
		}
		if envelope.LaneStatus() != lane.Done {
			if explicit {
				return BatchReport{}, &ErrLaneNotRetryable{LaneID: id, Reason: fmt.Sprintf("result envelope status is %q, not \"done\"", envelope.Status)}
			}
			continue
		}

		reports = append(reports, Report{
			LaneID:   id,
			Status:   lane.Done,
			Worktree: row.WorktreePath,
			Envelope: &envelope,
		})
		include = append(include, id)
	}

	if len(include) == 0 {
		return BatchReport{}, ErrNoRetryCandidates
	}

	return BatchReport{
		RunID:    runID,
		Lanes:    reports,
		Released: true,
		Outcome:  barrier.Outcome{Released: true, Integrate: include},
	}, nil
}

// RetryFeatureTarget derives the AttemptRequest target fields to use when
// retrying a feature-targeted batch's integration, preferring each included
// lane's own dispatch-time target -- captured in that lane's LaneMetadata by
// Execute -- over the feature row's immutable ledger anchor (feat.BaseSHA /
// feat.ExpectedParentSHA).
//
// This matters for any feature past its first wave: the documented
// multi-wave-chaining pattern lets each dispatch declare the feature's
// actual current tip as its own base_sha/expected_parent_sha (see
// FeatureTarget), which the durable attempt machine uses directly for CAS
// promotion (see driveAttemptFromLeased/performCASPromotion) without ever
// requiring the feature row's own base_sha/expected_parent_sha columns to
// change -- those columns are set once at feature.Service.Create and never
// updated again. Reusing feat.ExpectedParentSHA at retry time therefore
// compares the current ref against wave 1's anchor forever, which is stale
// (and fails CAS) for every wave after the first.
//
// laneIDs is the batch's own included lane set (BatchReport.Outcome.Integrate
// from RebuildBatchForRetry). Every lane that recorded a target in its
// LaneMetadata must agree, or RetryFeatureTarget fails closed with
// ErrMixedFeatureTargets -- the same rule FeatureTarget enforces at original
// dispatch time, since one batch promotes onto exactly one parent. A lane
// that predates this capture (no recorded target fields) is skipped rather
// than treated as a real disagreement, so retrying a batch dispatched before
// this fix still falls back to feat's own anchor, unchanged from before.
func RetryFeatureTarget(ctx context.Context, deps Deps, feat feature.Feature, laneIDs []string) (AttemptRequest, error) {
	target := AttemptRequest{
		FeatureID:         feat.ID,
		ParentRef:         feat.ParentRef,
		BaseSHA:           feat.BaseSHA,
		ExpectedParentSHA: feat.ExpectedParentSHA,
	}

	var recovered bool
	for _, id := range laneIDs {
		meta, err := deps.Ledger.GetLaneMetadata(ctx, deps.RunID, id)
		if err != nil {
			if errors.Is(err, ledger.ErrLaneUnknown) {
				continue
			}
			return AttemptRequest{}, fmt.Errorf("run: get lane metadata for %q: %w", id, err)
		}
		if meta.ParentRef == "" && meta.BaseSHA == "" && meta.ExpectedParentSHA == "" {
			// Predates per-lane target capture: no recorded target to prefer.
			continue
		}

		laneTarget := AttemptRequest{
			FeatureID:         feat.ID,
			ParentRef:         meta.ParentRef,
			BaseSHA:           meta.BaseSHA,
			ExpectedParentSHA: meta.ExpectedParentSHA,
		}
		if !recovered {
			target = laneTarget
			recovered = true
			continue
		}
		if laneTarget.ParentRef != target.ParentRef ||
			laneTarget.BaseSHA != target.BaseSHA ||
			laneTarget.ExpectedParentSHA != target.ExpectedParentSHA {
			return AttemptRequest{}, ErrMixedFeatureTargets
		}
	}

	return target, nil
}
