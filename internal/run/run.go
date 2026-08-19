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

// outputTruncatedDetail is the ledger event detail recorded when a
// dispatch's stdout/stderr capture was truncated (see
// executor.Outcome.OutputTruncated). It is deliberately phrased as a
// diagnosis note, not a failure: truncation says nothing about whether the
// lane's work succeeded -- the result envelope on disk decides that,
// independently -- only that a person debugging this lane from the ledger
// should know the captured output they are looking at may be incomplete.
const outputTruncatedDetail = "dispatch output capture truncated: captured stdout/stderr may be incomplete for diagnosis (lane status is unaffected)"

// streamDetailCap bounds how much of a dispatch's captured stdout or
// stderr is ever written into a ledger note, per stream. A captured
// stream from an agent CLI can run to megabytes -- a ledger row holding
// all of it would be its own defect (unbounded row growth in a database
// meant to stay small and queryable) -- so this is a deliberately small
// cap, generous enough to carry a real diagnostic (a stack trace, a final
// error message, a few lines of context) without turning the ledger into
// a log store.
//
// The cap applies independently to each stream, kept at the same 4096
// bytes it was before stdout capture was added, rather than being halved
// to a shared 2048-byte budget across the two: a diagnosis note now caps
// out at up to two capped chunks (worst case ~8KiB plus labels), which is
// still small next to "unbounded" and preserves the one property that
// actually matters here -- whichever stream carries the real diagnosis
// (agy's failures land on stdout, not stderr; see diagnosisDetail) gets
// the full 4096-byte budget on its own, never squeezed by an empty or
// irrelevant sibling stream.
const streamDetailCap = 4096

// noStreamDetail is recorded in place of a captured stream when a
// non-success dispatch produced none on it, so an empty capture reads as
// "nothing was written," never as a truncated note that merely looks
// empty.
const noStreamDetail = "(none captured)"

// streamTruncatedMarker is appended to a stream detail that was cut down
// to streamDetailCap, so a reader of the ledger can tell at a glance that
// what they are looking at is a clipped tail, not the stream's complete
// content.
const streamTruncatedMarker = "...[truncated, showing last %d of %d bytes]"

// diagnosisDetail formats the reason a lane failed together with its
// captured stderr AND stdout, each independently bounded by
// streamDetailCap, for a single ledger note (see EventLaneNote). reason
// is decideStatus's own explanation (e.g. "dispatch exited 1" or
// "dispatch timed out"); stderr and stdout are the dispatch's raw
// captured streams.
//
// Both streams are captured, not stderr alone: agy -- the only executor
// this binary currently dispatches -- was observed, reproducing the
// incident that motivated this change, to report its failure as
// structured JSON on STDOUT (e.g. {"status":"ERROR","error":"timeout
// waiting for response",...}) while STDERR was empty. Stderr alone would
// have recorded nothing at all for that exact incident. The two streams
// are labelled distinctly and truncated independently, since one may be
// empty while the other carries the whole diagnosis -- precisely what was
// observed.
//
// The **tail** of each stream is kept, not the head: a process that dies
// reports why in its last output -- the final panic, the last error line
// a CLI prints before exiting -- not its first. Keeping the head would
// systematically discard exactly the part of the capture most likely to
// explain the failure.
func diagnosisDetail(reason, stderr, stdout string) string {
	return fmt.Sprintf("%s\nstderr: %s\nstdout: %s", reason, formatStreamDetail(stderr), formatStreamDetail(stdout))
}

// formatStreamDetail bounds and labels one captured stream (stdout or
// stderr) for inclusion in a diagnosisDetail note -- see its doc comment
// for the tail-keeping and per-stream-cap rationale.
func formatStreamDetail(stream string) string {
	if stream == "" {
		return noStreamDetail
	}

	if len(stream) <= streamDetailCap {
		return stream
	}

	tail := stream[len(stream)-streamDetailCap:]
	marker := fmt.Sprintf(streamTruncatedMarker, streamDetailCap, len(stream))
	return fmt.Sprintf("%s\n%s", tail, marker)
}

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
	// LaneTimeout is the wall-clock budget ExecuteBatch grants each lane,
	// independently -- see ExecuteBatch's doc comment. It has no effect on
	// a direct Execute call: Execute always runs within whatever ctx its
	// caller hands it, exactly as before. Zero means "no per-lane deadline
	// applied by ExecuteBatch," which is what every Execute test in this
	// package already relies on and what a plain context.Context without a
	// deadline continues to mean.
	LaneTimeout time.Duration

	CombineTree        func(ctx context.Context, primaryRoot, runID string, branches []string) (worktreePath, branchName string, err error)
	RunChecks          func(ctx context.Context, worktreePath string) (passed bool, output string, err error)
	PromoteTarget      func(ctx context.Context, primaryRoot, integrationBranch string) error
	DiscardCombined    func(ctx context.Context, primaryRoot, worktreePath, branchName string) error
	RemoveLaneWorktree func(ctx context.Context, primaryRoot, worktreePath, branch string) error
}

// Report is the outcome of running exactly one lane through Execute.
// Execute itself does not own a barrier -- see ExecuteBatch, which is the
// sole owner of the barrier for every lane in a batch and carries the
// release/outcome result on BatchReport instead. A Report with a terminal
// Status is not, by itself, evidence of anything about the batch it may be
// part of.
type Report struct {
	LaneID   string
	Status   lane.Status
	Worktree string
	Envelope *result.Envelope // nil when the lane produced no readable envelope
	// OutputCaptureIncomplete is true when the dispatch's captured
	// stdout/stderr may be missing trailing output -- see
	// executor.Outcome.OutputTruncated for the mechanism (a grandchild
	// process holding the pipes open past the executor's own wait
	// delay). It carries no judgment about whether the lane's work
	// succeeded: Status is decided from the on-disk result envelope
	// alone (see decideStatus), never from captured stdout/stderr, and
	// this flag is independent of it. Its only purpose is to warn
	// whoever reads this report that if they go looking at the
	// dispatch's captured output to diagnose something, they may not be
	// looking at the whole picture.
	OutputCaptureIncomplete bool
	// Diagnosis carries the same reason-plus-streams text recorded as the
	// lane's ledger note (see diagnosisDetail) for any of the three
	// non-success dispatch outcomes -- non-zero exit, timeout, or an
	// exit-0 dispatch whose envelope could not be read. It is empty for a
	// lane whose terminal status came from a readable envelope, since
	// that already explains itself through Envelope. Diagnosis exists so
	// a caller that never touches the ledger -- e.g. cmd/lucind-ai's
	// printReport -- can still see why a lane failed. It is not, by
	// itself, proof of what specifically went wrong inside the
	// dispatched process, only the captured (and possibly truncated)
	// tail of what that process wrote to stderr and stdout before its
	// outcome was decided -- both streams, since agy has been observed
	// reporting its own failures as structured JSON on stdout rather than
	// stderr (see diagnosisDetail).
	Diagnosis string
}

// Execute runs one packet end to end: create its worktree, register it in
// the ledger, dispatch it through Executor, decide its terminal status from
// what actually happened, and persist that status. It does not touch a
// barrier at all -- that is ExecuteBatch's job, since a barrier is a join
// over every lane in a batch, not a concept a single lane can own by
// itself.
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
		cause := fmt.Errorf("run: append lane_registered event for %q: %w", p.ID, err)
		return Report{}, recordLaneFailure(ctx, deps, p.ID, now, cause)
	}
	if err := deps.Ledger.SetStatus(ctx, deps.RunID, p.ID, lane.Running, now); err != nil {
		cause := fmt.Errorf("run: set lane %q running: %w", p.ID, err)
		return Report{}, recordLaneFailure(ctx, deps, p.ID, now, cause)
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

	// persistCtx carries ctx's values onward without its deadline or
	// cancellation, for every ledger write from here on. A caller-owned
	// per-lane deadline (see ExecuteBatch's Deps.LaneTimeout) is meant to
	// bound the dispatch itself -- "a lane killed on its ceiling returns
	// blocked with the worktree preserved" (see docs/prd.md section 7) --
	// not the bookkeeping that records that outcome. Using the
	// already-expired ctx here would make persisting a timed-out lane's
	// own lane.Blocked status fail with context.DeadlineExceeded, turning
	// a graceful timeout into a spurious lane.Failed. See
	// executor.Agy.Run's own doc comment: a dispatch that hit ctx's
	// deadline returns Outcome{TimedOut: true} with a nil error
	// precisely so the caller can still finish recording it normally.
	persistCtx := context.WithoutCancel(ctx)

	if err != nil {
		// executor.Executor.Run returns a non-nil error only for the
		// genuine never-ran case (see executor.Agy.Run's doc comment) --
		// a real infrastructure failure in our own binary, distinct from
		// a dispatch that ran and produced a bad outcome. The lane has
		// already been registered and marked running at this point, so
		// leaving it there would report a lane as running forever when
		// nothing is running; recordLaneFailure closes that gap.
		cause := fmt.Errorf("run: dispatch lane %q: %w", p.ID, err)
		return Report{}, recordLaneFailure(persistCtx, deps, p.ID, now, cause)
	}

	status, envelope, reason := decideStatus(deps, wt.Path, outcome)

	// diagnosis is empty exactly when reason is empty (the envelope
	// decided a terminal status cleanly): reason is decideStatus's own
	// explanation for a non-zero exit, a timeout, or an unreadable
	// envelope, and it is worth nothing on its own without the captured
	// stderr and stdout that a person would otherwise have to open the
	// ledger's SQLite file to go looking for.
	var diagnosis string
	if reason != "" {
		diagnosis = diagnosisDetail(reason, outcome.Stderr, outcome.Stdout)
		if err := deps.Ledger.AppendEvent(persistCtx, ledger.Event{
			RunID:  deps.RunID,
			LaneID: p.ID,
			Type:   ledger.EventLaneNote,
			Detail: diagnosis,
			At:     now,
		}); err != nil {
			cause := fmt.Errorf("run: append reason event for %q: %w", p.ID, err)
			return Report{}, recordLaneFailure(persistCtx, deps, p.ID, now, cause)
		}
	}

	if err := deps.Ledger.SetStatus(persistCtx, deps.RunID, p.ID, status, now); err != nil {
		cause := fmt.Errorf("run: set lane %q terminal status: %w", p.ID, err)
		return Report{}, recordLaneFailure(persistCtx, deps, p.ID, now, cause)
	}

	// A truncated capture is recorded as its own ledger event so the
	// fact survives the run, not only the one process that printed it.
	// It is appended under ledger.EventLaneNote as a diagnostic annotation.
	if outcome.OutputTruncated {
		if err := deps.Ledger.AppendEvent(persistCtx, ledger.Event{
			RunID:  deps.RunID,
			LaneID: p.ID,
			Type:   ledger.EventLaneNote,
			Detail: outputTruncatedDetail,
			At:     now,
		}); err != nil {
			cause := fmt.Errorf("run: append output-truncated event for %q: %w", p.ID, err)
			return Report{}, recordLaneFailure(persistCtx, deps, p.ID, now, cause)
		}
	}

	return Report{
		LaneID:                  p.ID,
		Status:                  status,
		Worktree:                wt.Path,
		Envelope:                envelope,
		OutputCaptureIncomplete: outcome.OutputTruncated,
		Diagnosis:               diagnosis,
	}, nil
}

// recordLaneFailure is the last line of defense against a lane row that
// silently stays lane.Running (or lane.Pending) forever. Once RegisterLane
// has succeeded for a lane, every later error path in Execute must route
// through this function before returning, so the ledger never lies about a
// lane still being in flight when Execute itself has already given up on
// it. cause is the error that aborted Execute; it is recorded as a
// lane_note event's reason and the lane is set to lane.Failed --
// distinct from lane.Blocked, which means a decision is needed, because
// this is a technical failure in our own binary, not something the
// dispatched work produced. See internal/lane/status.go.
//
// If persisting that failure itself fails, cause is not discarded: it is
// still returned, wrapped together with the write failure, so a caller
// never sees a ledger-write error in place of the real reason Execute
// aborted.
func recordLaneFailure(ctx context.Context, deps Deps, laneID string, now time.Time, cause error) error {
	if err := deps.Ledger.AppendEvent(ctx, ledger.Event{
		RunID:  deps.RunID,
		LaneID: laneID,
		Type:   ledger.EventLaneNote,
		Detail: cause.Error(),
		At:     now,
	}); err != nil {
		return fmt.Errorf("%w (additionally, failed to record the failure reason in the ledger: %v)", cause, err)
	}

	if err := deps.Ledger.SetStatus(ctx, deps.RunID, laneID, lane.Failed, now); err != nil {
		return fmt.Errorf("%w (additionally, failed to persist lane.Failed status in the ledger: %v)", cause, err)
	}

	return cause
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
