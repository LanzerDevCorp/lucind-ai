// Package run is the composition root: the one place where packet,
// worktree, executor, result, ledger, and barrier meet. Every one of those
// packages is proven and fully tested in isolation, but nothing in this
// repository has ever called ledger.Open in anger, dispatched a real
// worktree through a real executor, and fed the result back into a
// barrier. Execute is that wiring, made explicit and testable without
// git, without a real agent, and without the network — every dependency
// it needs from the outside world arrives through Deps, injected by the
// caller.
package run

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/barrier"
	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

// ErrEnvelopeUnreadable marks a lane whose dispatch exited 0 but whose
// result envelope could not be read or failed schema validation. Exit 0
// only proves the child process ran to completion — see executor.Status —
// never that the requested work succeeded, and the only thing that can
// promote a lane past that is a trustworthy envelope. This project exists
// because a packet once reported success while a hard stop had fired, so
// an unreadable envelope forces the lane to lane.Blocked rather than being
// swallowed: the underlying error is recorded in a ledger event so the
// reason survives even though Execute itself still returns successfully.
var ErrEnvelopeUnreadable = errors.New("run: dispatch exited 0 but the result envelope could not be read or is schema-invalid")

// lucindDir is the directory, relative to a worktree's root, that carries
// the schema agy validates against and the envelope it writes back.
const lucindDir = ".lucind"

// resultSchemaFileName is the file Execute writes so a dispatched agy can
// read it via --json-schema.
const resultSchemaFileName = "result.schema.json"

// resultEnvelopePath is where Execute expects the dispatched agent to have
// left its result envelope, relative to the worktree's fs.FS root. fs.FS
// paths are always forward-slash-separated regardless of host OS (see
// io/fs), so this is a string literal, not a filepath.Join.
const resultEnvelopePath = lucindDir + "/result.json"

// DefaultModel is the model a dispatch runs on when its packet names none.
// packet.Model is optional, and packet.Parse deliberately leaves it empty
// rather than filling in this default itself: Parse's job is to reflect
// frontmatter literally, not to inject runtime policy that can change
// independently of the packet format. Execute is the composition root
// where that policy is applied, so the default lives here — in exactly
// one place — rather than duplicated between packet and run, which is how
// the two would drift. See docs/prd.md and
// plugin/claude-code/skills/lucind-ai/references/runtime.md, both of which
// name this as the project default.
const DefaultModel = "gemini-3.7-flash-high"

// Deps is everything Execute needs from the outside world. Every field is
// injected so the whole flow is testable without git, without a real agent
// and without the network.
type Deps struct {
	RunID          string // supplied by the caller, never generated here, so tests are deterministic
	PrimaryRoot    string // the primary repository root
	Ledger         *ledger.Ledger
	Executor       executor.Executor
	CreateWorktree func(ctx context.Context, primaryRoot, laneID string) (worktree.Worktree, error)
	WorktreeFS     func(path string) fs.FS // opens a worktree for reading its result envelope
	Now            func() time.Time        // injected clock; tests pin it
}

// Report is the outcome of running exactly one lane through Execute.
type Report struct {
	LaneID   string
	Status   lane.Status
	Worktree string
	Envelope *result.Envelope // nil when the lane produced no readable envelope
	Released bool
	Outcome  barrier.Outcome
}

// Execute runs one packet end to end: create its worktree, register it in
// the ledger, dispatch it through Executor, decide its terminal status from
// what actually happened, persist that status, and fold it through a
// single-lane barrier.
//
// Execute returns a non-nil error only when the flow itself could not
// complete — worktree creation failed, a ledger write failed, or the
// executor never ran the dispatch at all. A lane that ran and ended
// blocked, deviated, or failed is not one of those: it is a successful
// Execute call carrying a Report that says so.
func Execute(ctx context.Context, deps Deps, p packet.Packet) (Report, error) {
	now := deps.Now()

	wt, err := deps.CreateWorktree(ctx, deps.PrimaryRoot, p.ID)
	if err != nil {
		return Report{}, fmt.Errorf("run: create worktree for lane %q: %w", p.ID, err)
	}

	schemaPath, err := writeResultSchema(wt.Path)
	if err != nil {
		return Report{}, fmt.Errorf("run: write result schema for lane %q: %w", p.ID, err)
	}

	// routingCondition is the reason this lane was routed to its
	// executor — p.RoutedBy, never p.Executor. The executor is the
	// outcome of a routing decision, not its reason; recording it as the
	// condition would be implicit routing, which the skill forbids.
	// packet.Parse already guarantees this is non-empty via
	// ErrMissingRoutedBy, which also satisfies the ledger's
	// NOT NULL/non-empty constraint on routing_condition.
	routingCondition := p.RoutedBy

	if err := deps.Ledger.RegisterLane(ctx, ledger.Lane{
		RunID:            deps.RunID,
		LaneID:           p.ID,
		PacketID:         p.ID,
		Executor:         p.Executor,
		RoutingCondition: routingCondition,
		Status:           lane.Pending,
		WorktreePath:     wt.Path,
	}); err != nil {
		return Report{}, fmt.Errorf("run: register lane %q: %w", p.ID, err)
	}
	if err := deps.Ledger.AppendEvent(ctx, ledger.Event{
		RunID:  deps.RunID,
		LaneID: p.ID,
		Type:   ledger.EventLaneRegistered,
		Detail: routingCondition,
		At:     now,
	}); err != nil {
		return Report{}, fmt.Errorf("run: append lane_registered event for %q: %w", p.ID, err)
	}
	if err := deps.Ledger.SetStatus(ctx, deps.RunID, p.ID, lane.Running, now); err != nil {
		return Report{}, fmt.Errorf("run: set lane %q running: %w", p.ID, err)
	}

	// model falls back to DefaultModel when the packet names none — see
	// DefaultModel's doc comment for why that fallback lives here rather
	// than in packet.Parse.
	model := p.Model
	if model == "" {
		model = DefaultModel
	}

	outcome, err := deps.Executor.Run(ctx, executor.Request{
		Prompt:       p.Body,
		WorktreePath: wt.Path,
		Model:        model,
		SchemaPath:   schemaPath,
	})
	if err != nil {
		return Report{}, fmt.Errorf("run: dispatch lane %q: %w", p.ID, err)
	}

	status, envelope, reason := decideStatus(deps, wt.Path, outcome)

	if reason != "" {
		if err := deps.Ledger.AppendEvent(ctx, ledger.Event{
			RunID:  deps.RunID,
			LaneID: p.ID,
			Type:   ledger.EventLaneStatusChanged,
			Detail: reason,
			At:     now,
		}); err != nil {
			return Report{}, fmt.Errorf("run: append reason event for %q: %w", p.ID, err)
		}
	}

	if err := deps.Ledger.SetStatus(ctx, deps.RunID, p.ID, status, now); err != nil {
		return Report{}, fmt.Errorf("run: set lane %q terminal status: %w", p.ID, err)
	}

	b, err := barrier.New([]string{p.ID})
	if err != nil {
		return Report{}, fmt.Errorf("run: build barrier for lane %q: %w", p.ID, err)
	}
	if err := b.Observe(lane.State{LaneID: p.ID, Status: status}); err != nil {
		return Report{}, fmt.Errorf("run: observe lane %q into barrier: %w", p.ID, err)
	}
	bOutcome := b.Outcome()

	if bOutcome.Released {
		if err := deps.Ledger.AppendEvent(ctx, ledger.Event{
			RunID:  deps.RunID,
			LaneID: p.ID,
			Type:   ledger.EventBarrierReleased,
			Detail: string(status),
			At:     now,
		}); err != nil {
			return Report{}, fmt.Errorf("run: append barrier_released event for %q: %w", p.ID, err)
		}
	}

	return Report{
		LaneID:   p.ID,
		Status:   status,
		Worktree: wt.Path,
		Envelope: envelope,
		Released: bOutcome.Released,
		Outcome:  bOutcome,
	}, nil
}

// decideStatus implements step 5 of the flow: a timed-out or non-zero-exit
// dispatch never has its envelope read at all, and an exit-0 dispatch whose
// envelope cannot be read or fails schema validation is lane.Blocked, never
// lane.Done — reading result.json is the only way a lane is allowed to
// reach Done.
func decideStatus(deps Deps, worktreePath string, outcome executor.Outcome) (lane.Status, *result.Envelope, string) {
	switch executor.Status(outcome) {
	case lane.Blocked:
		if outcome.TimedOut {
			return lane.Blocked, nil, "dispatch timed out"
		}
		return lane.Blocked, nil, fmt.Sprintf("dispatch exited %d", outcome.ExitCode)
	default:
		// executor.Status only ever returns lane.Blocked or lane.Running
		// (see internal/executor/status.go); lane.Running means exit 0,
		// which is the only case where reading the envelope is warranted.
		fsys := deps.WorktreeFS(worktreePath)
		envelope, err := result.Read(fsys, resultEnvelopePath)
		if err != nil {
			return lane.Blocked, nil, fmt.Errorf("%w: %v", ErrEnvelopeUnreadable, err).Error()
		}
		st := envelope.LaneStatus()
		if st == "" {
			// result.Read already validated against the schema's status
			// enum, so this is unreachable in practice; it is handled
			// anyway rather than ever returning an empty lane.Status.
			return lane.Blocked, nil, ErrEnvelopeUnreadable.Error()
		}
		return st, &envelope, ""
	}
}

// writeResultSchema writes the embedded result schema into worktreePath at
// .lucind/result.schema.json, creating .lucind if needed, so a dispatched
// agy can read it via --json-schema. It returns the schema's absolute path
// for use as executor.Request.SchemaPath.
func writeResultSchema(worktreePath string) (string, error) {
	dir := filepath.Join(worktreePath, lucindDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}

	path := filepath.Join(dir, resultSchemaFileName)
	if err := os.WriteFile(path, result.SchemaJSON(), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}

	return path, nil
}
