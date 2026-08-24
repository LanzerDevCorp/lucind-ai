package run

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/barrier"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
)

// BatchReport is the outcome of running every packet in a batch through
// ExecuteBatch. Unlike Report, which says nothing about any barrier,
// BatchReport is where a batch's barrier result lives: Released and
// Outcome describe the single barrier shared by every lane in Lanes.
type BatchReport struct {
	RunID string
	// Lanes holds one Report per input packet, in input order --
	// regardless of the order in which lanes actually completed. See
	// ExecuteBatch's doc comment for why order is preserved this way.
	Lanes    []Report
	Released bool
	Outcome  barrier.Outcome
}

// ExecuteBatch runs every packet in ps as its own lane, concurrently, and
// folds every lane's terminal status through one barrier shared by the
// whole batch -- the join Execute used to fake with a barrier of one lane,
// which could never actually fail to release.
//
// The governing rule is: a lane that ends badly lets the batch finish, and
// everything is preserved. Concretely:
//
//   - Lanes never cancel each other. ExecuteBatch uses sync.WaitGroup, not
//     errgroup's cancelling context: one lane failing, blocking, or timing
//     out never cancels, shortens, or skips another lane.
//   - Each lane gets its own deadline, derived independently from ctx via
//     Deps.LaneTimeout (when non-zero), rather than one deadline shared
//     across the whole batch -- a slow lane never eats into a sibling's
//     clock.
//   - A lane that never starts is still a lane. If its worktree creation
//     or ledger registration fails before Execute would otherwise have
//     registered it, ExecuteBatch registers it itself and drives it to
//     lane.Failed, so it is still one of the barrier's expected lanes --
//     otherwise the barrier would wait forever for a lane that never
//     existed in the ledger.
//   - Every worktree is preserved, including done ones. Integration (and
//     therefore worktree removal for integrated lanes) is a later slice;
//     nothing here ever removes a worktree.
//
// Validation happens before any side effect: an empty ps is an error (via
// barrier.New's own ErrNoLanes), and duplicate lane IDs are an error (via
// barrier.New's own duplicate check) -- barrier.New is built first and is
// the sole authority on both, so this package never reimplements either
// check.
//
// ExecuteBatch's own returned error is reserved for that up-front
// validation failure. Once validation passes, ExecuteBatch always returns
// a nil error: a lane's own failure is absorbed into its Report and the
// batch's Outcome, never turned into ExecuteBatch's error, because (per
// the governing rule above) one bad lane must never stop the batch from
// completing.
func ExecuteBatch(ctx context.Context, deps Deps, ps []packet.Packet) (BatchReport, error) {
	ids := make([]string, len(ps))
	for i, p := range ps {
		ids[i] = p.ID
	}

	// barrier.New is the sole authority on "at least one lane" and "no
	// duplicate lane IDs" -- built first, before any worktree or ledger
	// write, so both checks reject a bad batch with zero side effects.
	b, err := barrier.New(ids)
	if err != nil {
		return BatchReport{}, fmt.Errorf("run: build batch barrier: %w", err)
	}

	reports := make([]Report, len(ps))
	var wg sync.WaitGroup
	for i, p := range ps {
		wg.Add(1)
		go func(i int, p packet.Packet) {
			defer wg.Done()
			reports[i] = runOneLane(ctx, deps, p, b)
		}(i, p)
	}
	wg.Wait()

	outcome := b.Outcome()

	if outcome.Released {
		// Best-effort: the barrier has genuinely released and every
		// lane's own terminal status is already durably recorded by that
		// lane's own Execute (or runOneLane's ensureLaneFailed) call, so
		// losing only this run-scoped summary event must never turn an
		// otherwise-successful batch into an error.
		_ = deps.Ledger.AppendEvent(ctx, ledger.Event{
			RunID:  deps.RunID,
			Type:   ledger.EventBarrierReleased,
			Detail: fmt.Sprintf("batch released: integrate=%d preserve=%d", len(outcome.Integrate), len(outcome.Preserve)),
			At:     deps.Now(),
		})
	}

	return BatchReport{
		RunID:    deps.RunID,
		Lanes:    reports,
		Released: outcome.Released,
		Outcome:  outcome,
	}, nil
}

// runOneLane drives exactly one lane of a batch: it derives that lane's own
// deadline (independent of every other lane's), runs it through Execute,
// makes sure a lane that never even got registered still ends up in the
// ledger as lane.Failed, and observes its terminal status into the shared
// batch barrier exactly once.
func runOneLane(ctx context.Context, deps Deps, p packet.Packet, b *barrier.Barrier) Report {
	laneCtx := ctx
	if deps.LaneTimeout > 0 {
		var cancel context.CancelFunc
		laneCtx, cancel = context.WithTimeout(ctx, deps.LaneTimeout)
		defer cancel()
	}

	report, err := Execute(laneCtx, deps, p)
	if err != nil {
		now := deps.Now()
		// Best-effort: even if ensureLaneFailed itself cannot write to the
		// ledger, the lane still must be observed into the barrier as
		// failed below, or the barrier would wait forever for a lane that
		// can never report in. The recording error is not returned — that
		// would fail the batch — but both causes go on Report.Diagnosis,
		// and Execute's Worktree is kept: printReport prints both, and an
		// empty vs non-empty `worktree:` line is how a human scanning
		// stdout tells admission rejection from a directory that exists.
		recErr := ensureLaneFailed(ctx, deps, p, now, err)
		diagnosis := err.Error()
		if recErr != nil {
			diagnosis = fmt.Sprintf("%s (additionally, failed to record the lane failure in the ledger: %v)", err, recErr)
		}
		report = Report{LaneID: p.ID, Status: lane.Failed, Worktree: report.Worktree, Diagnosis: diagnosis}
	}

	// Observe is safe for concurrent use (see barrier.Barrier.Observe) and
	// write-once per lane, so every lane's terminal status folds into the
	// shared batch barrier exactly once regardless of goroutine
	// interleaving. Every lane ID passed here came from the same ps this
	// batch's barrier was built from, so ErrUnexpectedLane can never fire.
	_ = b.Observe(lane.State{LaneID: p.ID, Status: report.Status})

	return report
}

// ensureLaneFailed covers the "a lane that never starts is still a lane"
// requirement: Execute's own recordLaneFailure already registers and fails
// a lane for every failure that happens after RegisterLane succeeds, but
// three earlier failure points -- CreateWorktree, writing the embedded
// result schema, and RegisterLane itself -- leave no row in the ledger at
// all (see Execute's doc comment and its own tests). ensureLaneFailed
// checks whether the lane is already registered and, only if it is not,
// registers it itself and drives it straight to lane.Failed, so it is
// never missing from the batch's ledger or from the barrier's expected
// set.
func ensureLaneFailed(ctx context.Context, deps Deps, p packet.Packet, now time.Time, cause error) error {
	lanes, err := deps.Ledger.Lanes(ctx, deps.RunID)
	if err != nil {
		return fmt.Errorf("run: check lane %q registration before failing it: %w", p.ID, err)
	}
	for _, ln := range lanes {
		if ln.LaneID == p.ID {
			// Already registered -- Execute's own recordLaneFailure got
			// there first and already drove it to lane.Failed.
			return nil
		}
	}

	// routingCondition mirrors Execute's own use of p.RoutedBy, never
	// p.Executor -- see Execute's doc comment on routingCondition for why.
	routingCondition := p.RoutedBy

	if err := deps.Ledger.RegisterLane(ctx, ledger.Lane{
		RunID:            deps.RunID,
		LaneID:           p.ID,
		PacketID:         p.ID,
		Executor:         p.Executor,
		RoutingCondition: routingCondition,
		Status:           lane.Pending,
	}); err != nil {
		return fmt.Errorf("run: register never-started lane %q: %w", p.ID, err)
	}
	if err := deps.Ledger.UpdateLaneMetadata(ctx, ledger.LaneMetadata{
		RunID:        deps.RunID,
		LaneID:       p.ID,
		Model:        p.Model,
		Agent:        p.Agent,
		SDDPhase:     p.SDDPhase,
		FanoutGroup:  p.FanoutGroup,
		Feature:      p.Feature,
		Skill:        p.Skill,
		PacketPath:   p.Path,
		AllowedPaths: p.AllowedPaths,
	}, now); err != nil {
		return fmt.Errorf("run: update lane metadata for never-started lane %q: %w", p.ID, err)
	}
	if err := deps.Ledger.AppendEvent(ctx, ledger.Event{
		RunID:  deps.RunID,
		LaneID: p.ID,
		Type:   ledger.EventLaneRegistered,
		Detail: routingCondition,
		At:     now,
	}); err != nil {
		return fmt.Errorf("run: append lane_registered event for never-started lane %q: %w", p.ID, err)
	}
	if err := deps.Ledger.AppendEvent(ctx, ledger.Event{
		RunID:  deps.RunID,
		LaneID: p.ID,
		Type:   ledger.EventLaneNote,
		Detail: cause.Error(),
		At:     now,
	}); err != nil {
		return fmt.Errorf("run: append failure-reason event for never-started lane %q: %w", p.ID, err)
	}
	if err := deps.Ledger.SetStatus(ctx, deps.RunID, p.ID, lane.Failed, now); err != nil {
		return fmt.Errorf("run: set never-started lane %q failed: %w", p.ID, err)
	}

	return nil
}
