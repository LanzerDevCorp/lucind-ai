package run

import (
	"context"
	"errors"
	"fmt"

	"github.com/LanzerDevCorp/lucind-ai/internal/feature"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

// ErrMixedFeatureTargets marks a batch whose packets do not agree on where the
// batch promotes. One batch produces one combined tree and promotes it once, so
// there is exactly one correct parent -- picking the first packet's target
// would silently land the other packets' lanes on a parent nobody named.
var ErrMixedFeatureTargets = errors.New("run: batch mixes integration targets; every packet in a batch must name the same feature target, or all must declare legacy mode")

// FeatureTarget derives the single feature target a batch promotes onto, from
// the target fields its packets declare.
//
// The returned AttemptRequest carries only the four target fields; the caller
// supplies ID, IdempotencyKey, Owner, and Branches. ok is false for a legacy
// batch, which promotes through Integrate's ff-merge into the primary
// checkout rather than through the attempt state machine.
func FeatureTarget(ps []packet.Packet) (AttemptRequest, bool, error) {
	if len(ps) == 0 {
		return AttemptRequest{}, false, nil
	}

	legacy := ps[0].LegacyMain
	target := AttemptRequest{
		FeatureID:         ps[0].Feature,
		ParentRef:         ps[0].ParentRef,
		BaseSHA:           ps[0].BaseSHA,
		ExpectedParentSHA: ps[0].ExpectedParentSHA,
	}

	for _, p := range ps[1:] {
		if p.LegacyMain != legacy {
			return AttemptRequest{}, false, ErrMixedFeatureTargets
		}
		if legacy {
			continue
		}
		if p.Feature != target.FeatureID ||
			p.ParentRef != target.ParentRef ||
			p.BaseSHA != target.BaseSHA ||
			p.ExpectedParentSHA != target.ExpectedParentSHA {
			return AttemptRequest{}, false, ErrMixedFeatureTargets
		}
	}

	if legacy {
		return AttemptRequest{}, false, nil
	}

	// A packet declaring nothing at all is the reusable-template shape: the
	// orchestrator supplies the target at dispatch. Reaching here with it
	// still empty means no target was ever supplied, and every lane is about
	// to be rejected by admission one at a time, each after its worktree
	// already exists. Name both exits once, before any of that.
	if target.FeatureID == "" && target.ParentRef == "" && target.BaseSHA == "" && target.ExpectedParentSHA == "" {
		return AttemptRequest{}, false, fmt.Errorf("%w: dispatch with --legacy-main and --expected-parent-sha to target main, or name feature, parent_ref, base_sha and expected_parent_sha in the packet", ErrMissingFeatureTarget)
	}

	// feature.Create refuses main and the lucind/ lane namespace as a parent,
	// so a batch naming one of those has no reachable promotion target. Caught
	// here rather than at promotion time: by then every lane has already run
	// and spent its quota on work that cannot land. A change targeting main is
	// a legacy dispatch -- `--legacy-main` with an expected SHA -- not a
	// feature whose parent happens to be main.
	if err := feature.ValidateParentRef(target.ParentRef); err != nil {
		return AttemptRequest{}, false, fmt.Errorf("run: packet %q names feature %q with parent_ref %q: %w (a batch targeting main is a legacy dispatch: use --legacy-main with --expected-parent-sha)", ps[0].ID, target.FeatureID, target.ParentRef, err)
	}

	return target, true, nil
}

// IntegrateFeature integrates a completed batch onto a feature parent through
// the durable attempt state machine, and is the feature-targeted counterpart
// to Integrate.
//
// Three differences from Integrate are deliberate and visible to the caller:
//
//   - Promotion is a compare-and-swap on the feature's parent ref
//     (ExecuteAttempt -> performCASPromotion), so it never checks out, merges
//     into, or otherwise mutates the primary repository's working tree. Which
//     branch the primary checkout happens to be on is irrelevant here.
//   - The attempt holds the feature's lease for its whole duration, so a
//     second concurrent attempt on the same feature is blocked rather than
//     racing, and the cross-feature overlap gate runs before promotion.
//   - There is no bisection. A failing combined tree fails the whole attempt;
//     Integrate's clean-subset isolation has no equivalent here, because the
//     attempt is one durable transaction against one parent ref.
//
// The terminal Attempt is returned alongside the report so the caller can name
// it: a non-terminal attempt left by an interrupted process is what
// "lucind-ai feature recover --attempt <id>" resumes.
func IntegrateFeature(ctx context.Context, deps Deps, batch BatchReport, req AttemptRequest) (IntegrateReport, Attempt, error) {
	if !batch.Released || len(batch.Outcome.Integrate) == 0 {
		return IntegrateReport{RunID: deps.RunID}, Attempt{}, nil
	}

	branches := make([]string, len(batch.Outcome.Integrate))
	for i, id := range batch.Outcome.Integrate {
		branches[i] = worktree.BranchFor(id)
	}
	req.Branches = branches

	att, err := ExecuteAttempt(ctx, deps, req)
	if err != nil {
		return IntegrateReport{RunID: deps.RunID, Attempted: true}, att, err
	}

	now := updateNow(deps)

	if att.Status == AttemptStatusPromoted {
		rep, cErr := completeIntegration(ctx, deps, batch, batch.Outcome.Integrate, nil, now)
		return rep, att, cErr
	}

	// Blocked, failed, and stale all land here. Each is a real outcome rather
	// than a crash, so the lanes are demoted with the attempt's own reason and
	// their worktrees are preserved for inspection -- the same contract
	// Integrate's revert path offers.
	reason := att.FailureReason
	if reason == "" {
		reason = fmt.Sprintf("integration attempt %s ended in status %q", att.ID, att.Status)
	}
	revertLanes(ctx, deps, batch.Outcome.Integrate, reason, now)

	return IntegrateReport{
		RunID:     deps.RunID,
		Attempted: true,
		Passed:    false,
		Reverted:  append([]string(nil), batch.Outcome.Integrate...),
		Reason:    reason,
	}, att, nil
}
