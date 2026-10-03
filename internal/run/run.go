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
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/candidatechange"
	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/overlap"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
	"github.com/LanzerDevCorp/lucind-ai/internal/skillset"
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

// ErrMissingFeatureTarget marks a packet that omits required target fields
// (feature, parent_ref, base_sha, expected_parent_sha) and does not declare
// explicit legacy mode.
var ErrMissingFeatureTarget = errors.New("run: packet is missing required target fields (feature, parent_ref, base_sha, expected_parent_sha) without explicit legacy mode")

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

const (
	progressBufferSize    = 32
	progressBatchSize     = 32
	progressFlushInterval = 250 * time.Millisecond
)

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

// DefaultMaxParallelLanes is the default limit on concurrently running lanes in ExecuteBatch.
const DefaultMaxParallelLanes = 3

// Deps is everything Execute needs from the outside world. Every field is
// injected so the whole flow is testable without git, without a real agent
// and without the network.
type Deps struct {
	RunID       string // supplied by the caller, never generated here, so tests are deterministic
	PrimaryRoot string // the primary repository root
	Ledger      *ledger.Ledger
	// LookupExecutor resolves an executor by the packet's own Executor name
	// at dispatch time, one call per lane -- not once per batch.
	LookupExecutor func(name string) (executor.Executor, error)
	// CreateWorktree creates the lane's worktree. parentRef and baseSHA are
	// the packet's declared feature target, and are empty for a legacy
	// dispatch -- empty means "no start point", which branches the lane from
	// the primary checkout's HEAD, exactly as before feature targets existed.
	// A feature lane must instead start at its own base_sha, or its candidate
	// is built on a tree the feature's parent never contained.
	CreateWorktree func(ctx context.Context, primaryRoot, laneID, parentRef, baseSHA string) (worktree.Worktree, error)
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
	// MaxParallelLanes is the maximum number of lanes ExecuteBatch runs concurrently.
	// Zero means the default (3); a negative value is treated as the default as well;
	// values above the number of packets are harmless.
	MaxParallelLanes int
	// AppendProgressBatch is an optional test seam. Production uses Ledger's
	// atomic batch append when this is nil.
	AppendProgressBatch func(context.Context, []ledger.LaneProgress) error
	// ResolveCandidateIdentity freezes full commit/tree object IDs for a done lane.
	// Production defaults to Git; tests inject deterministic identities.
	ResolveCandidateIdentity func(ctx context.Context, primaryRoot, worktreePath, baseSHA string) (CandidateIdentity, error)
	// CollectCandidateChanges freezes canonical WU2 change classifications for
	// compiled authoring evidence. Production defaults to candidatechange.Collect.
	CollectCandidateChanges func(context.Context, candidatechange.Request) ([]candidatechange.Change, error)

	HasUniqueLaneCommits func(ctx context.Context, worktreePath, baseSHA string) (bool, error)
	PorcelainEmpty       func(ctx context.Context, worktreePath string) (bool, error)

	// CombineTree creates the integration worktree and merges branches into
	// it. parentRef and baseSHA are the packet's declared feature target,
	// and are empty for a legacy dispatch -- empty means "no start point",
	// which branches the integration worktree from the primary checkout's
	// current HEAD, exactly as before feature targets existed. A feature
	// batch must instead start at its own base_sha, or the combined tree
	// is built on top of whatever primaryRoot happens to have checked out
	// rather than the feature's actual parent.
	CombineTree        func(ctx context.Context, primaryRoot, runID, parentRef, baseSHA string, branches []string) (worktreePath, branchName string, err error)
	RunChecks          func(ctx context.Context, worktreePath string) (passed bool, output string, err error)
	PromoteTarget      func(ctx context.Context, primaryRoot, integrationBranch string) error
	DiscardCombined    func(ctx context.Context, primaryRoot, worktreePath, branchName string) error
	RemoveLaneWorktree func(ctx context.Context, primaryRoot, worktreePath, branch string) error
	// PersistEnvelope durably records one integrated lane's full result
	// envelope in the primary repository before its worktree is removed
	// -- see completeIntegration. Without this, Envelope.Findings (every
	// structured qualitative-judgment citation a read-only verify lane
	// produces) is permanently lost the moment the lane integrates, since
	// RemoveLaneWorktree deletes the only copy of .lucind/result.json.
	PersistEnvelope func(ctx context.Context, primaryRoot, laneID string, envelope *result.Envelope) error

	// Attempt state machine and Git/CAS hooks
	GitRunner           worktree.GitRunner
	PromoteCAS          func(ctx context.Context, primaryRoot, parentRef, candidateSHA, expectedSHA string) error
	ResolveRefSHA       func(ctx context.Context, primaryRoot, ref string) (string, error)
	ResolveCandidateSHA func(ctx context.Context, primaryRoot, worktreePath, branch string) (string, error)
	EvaluateOverlap     func(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...overlap.EvaluateOption) (*overlap.Evidence, error)
	// IsAncestorSHA reports whether ancestorSHA is an ancestor of (already
	// contained within) descendantSHA. evaluateOverlapGate uses it to guard
	// reuse of a previously approved+integrated reconciliation candidate: the
	// candidate is only safe to promote when this attempt's own feature tip is
	// still an ancestor of it. Without this guard, a candidate resolved for an
	// EARLIER round -- already consumed by an earlier promotion, or simply
	// stale relative to real new work landed since -- could be reused for a
	// LATER attempt with genuinely different content, CAS'ing the branch
	// backward and silently discarding everything since. nil means "not
	// wired to verify," which preserves prior behavior (reuse allowed)
	// unchanged; production always wires a real implementation (see
	// cmd/lucind-ai/cli.go's productionDeps).
	IsAncestorSHA   func(ctx context.Context, primaryRoot, ancestorSHA, descendantSHA string) (bool, error)
	FeatureLeaseTTL time.Duration
	// Dispatcher commit seams
	RunAttested         func(ctx context.Context, worktreePath, cmd string) (exitCode int, output string, err error)
	HasValidAttestation func(ctx context.Context, repoRoot, cmd, expectedTreeHash string) (bool, error)
	PreCommitGate       func(ctx context.Context, worktreePath string, p packet.Packet) (lane.Status, string)
	GitCommit           func(ctx context.Context, worktreePath, message string) error

	// RenewInterval controls how often driveAttemptFromLeased renews the
	// feature lease while checkFunc (integrate.Check) runs during the
	// CHECKING phase -- see the lease-renewal loop there. Zero means the
	// interval is derived from FeatureLeaseTTL (leaseTTL/3, floored at
	// 1 second). This exists mainly as a test injection point: a test can
	// set it to a couple of milliseconds to observe renewal deterministically
	// without waiting out a real multi-second lease TTL.
	RenewInterval time.Duration
}

// CandidateIdentity is Git's immutable object identity for a lane and its base.
type CandidateIdentity struct {
	BaseCommit, BaseTree, CandidateCommit, CandidateTree string
}

// Report is the outcome of running exactly one lane through Execute.
// Execute itself does not own a barrier -- see ExecuteBatch, which is the
// sole owner of the barrier for every lane in a batch and carries the
// release/outcome result on BatchReport instead. A Report with a terminal
// Status is not, by itself, evidence of anything about the batch it may be
// part of.
type Report struct {
	LaneID string
	Status lane.Status
	// Worktree is the directory Execute created for this lane. It is
	// empty when admission rejected the packet (or CreateWorktree itself
	// failed): nothing exists on disk. After CreateWorktree succeeds it
	// is the real path even if Execute later returns an error — so a
	// caller that never queries the ledger, including printReport's
	// `worktree:` line, can tell the two failures apart.
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
	// Diagnosis carries diagnostic text also recorded as lane_note events.
	// It covers non-success dispatch outcomes and best-effort progress
	// persistence failures. Progress failures never change Status, which
	// remains decided by the result envelope. Diagnosis exists so
	// a caller that never touches the ledger -- e.g. cmd/lucind-ai's
	// printReport -- can still see why a lane failed. It is not, by
	// itself, proof of what specifically went wrong inside the
	// dispatched process, only the captured (and possibly truncated)
	// tail of what that process wrote to stderr and stdout before its
	// outcome was decided -- both streams, since agy has been observed
	// reporting its own failures as structured JSON on stdout rather than
	// stderr (see diagnosisDetail).
	Diagnosis string
	// Attempts is the total number of executor runs dispatched for this lane.
	Attempts int
}

// Execute runs one packet end to end: create its worktree, register it in
// the ledger, dispatch it through Executor, decide its terminal status from
// what actually happened, and persist that status. It does not touch a
// barrier at all -- that is ExecuteBatch's job, since a barrier is a join
// over every lane in a batch, not a concept a single lane can own by
// itself.
//
// validatePacketAdmission verifies that a packet carries an explicit feature target
// (feature, parent_ref, base_sha, expected_parent_sha) or declares explicit legacy mode
// (legacy_main: true with expected_parent_sha), resolving the effective parent ref.
func validatePacketAdmission(p *packet.Packet) error {
	if p.LegacyMain {
		if p.ExpectedParentSHA == "" {
			return ErrMissingFeatureTarget
		}
		if p.ParentRef == "" {
			p.ParentRef = "main"
		}
		return nil
	}

	if p.Feature == "" || p.ParentRef == "" || p.BaseSHA == "" || p.ExpectedParentSHA == "" {
		return ErrMissingFeatureTarget
	}
	return nil
}

// Execute returns a non-nil error only when the flow itself could not
// complete — worktree creation failed, a ledger write failed, or the
// executor never ran the dispatch at all. A lane that ran and ended
// blocked, deviated, or failed is not one of those: it is a successful
// Execute call carrying a Report that says so.
func Execute(ctx context.Context, deps Deps, p packet.Packet) (Report, error) {
	report := Report{LaneID: p.ID}
	if err := validatePacketAdmission(&p); err != nil {
		return report, fmt.Errorf("run: admit lane %q: admission rejected, no worktree created: %w", p.ID, err)
	}

	now := deps.Now()

	// validatePacketAdmission has already defaulted a legacy packet's
	// ParentRef to "main", which is not a usable worktree start point, so
	// legacy lanes pass neither field and keep the no-start-point behavior.
	var laneParentRef, laneBaseSHA string
	if !p.LegacyMain {
		laneParentRef, laneBaseSHA = p.ParentRef, p.BaseSHA
	}
	wt, err := deps.CreateWorktree(ctx, deps.PrimaryRoot, p.ID, laneParentRef, laneBaseSHA)
	if err != nil {
		return report, fmt.Errorf("run: create worktree for lane %q: %w", p.ID, err)
	}
	report.Worktree = wt.Path

	schemaPath, err := writeResultSchema(wt.Path)
	if err != nil {
		return report, fmt.Errorf("run: write result schema for lane %q: %w", p.ID, err)
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
		return report, fmt.Errorf("run: register lane %q: %w", p.ID, err)
	}
	if err := deps.Ledger.UpdateLaneMetadata(ctx, ledger.LaneMetadata{
		RunID:        deps.RunID,
		LaneID:       p.ID,
		Model:        p.Model,
		Agent:        p.Agent,
		SDDPhase:     p.SDDPhase,
		FanoutGroup:  p.FanoutGroup,
		LaneRole:     p.LaneRole,
		ReadOnly:     p.ReadOnly,
		Feature:      p.Feature,
		Skill:        p.Skill,
		PacketPath:   p.Path,
		AllowedPaths: p.AllowedPaths,
		// Captured verbatim from this packet's own dispatch-time target
		// fields (empty for a legacy dispatch), not the feature row -- see
		// ledger.LaneMetadata's doc comment and RetryFeatureTarget.
		ParentRef:         p.ParentRef,
		BaseSHA:           p.BaseSHA,
		ExpectedParentSHA: p.ExpectedParentSHA,
	}, now); err != nil {
		cause := fmt.Errorf("run: update lane metadata for %q: %w", p.ID, err)
		return report, recordLaneFailure(ctx, deps, p.ID, now, cause)
	}
	if err := deps.Ledger.AppendEvent(ctx, ledger.Event{
		RunID:  deps.RunID,
		LaneID: p.ID,
		Type:   ledger.EventLaneRegistered,
		Detail: routingCondition,
		At:     now,
	}); err != nil {
		cause := fmt.Errorf("run: append lane_registered event for %q: %w", p.ID, err)
		return report, recordLaneFailure(ctx, deps, p.ID, now, cause)
	}
	if err := deps.Ledger.SetStatus(ctx, deps.RunID, p.ID, lane.Running, now); err != nil {
		cause := fmt.Errorf("run: set lane %q running: %w", p.ID, err)
		return report, recordLaneFailure(ctx, deps, p.ID, now, cause)
	}

	plan := buildAttemptPlan(p)
	isLoop := p.MaxIterations > 1 || len(p.Escalation) > 0

	persistCtx := context.WithoutCancel(ctx)

	var (
		attemptsRun        int
		lastStatus         lane.Status
		lastEnvelope       *result.Envelope
		lastReason         string
		lastOutcome        executor.Outcome
		lastVRes           verificationResult
		lastProgressErrors []error
		prevRungIndex      = 0
	)

	for attemptIdx, attempt := range plan {
		if attemptIdx > 0 && ctx.Err() != nil {
			break
		}

		exec, err := deps.LookupExecutor(attempt.ExecutorName)
		if err != nil {
			cause := fmt.Errorf("run: resolve executor %q for lane %q: %w", attempt.ExecutorName, p.ID, err)
			report.Attempts = attemptsRun
			return report, recordLaneFailure(persistCtx, deps, p.ID, now, cause)
		}

		model := attempt.Model
		if model == "" {
			model = exec.DefaultModel()
		}

		if attemptIdx > 0 {
			var noteDetail string
			if attempt.RungIndex != prevRungIndex {
				noteDetail = fmt.Sprintf("attempt %d/%d rung %d escalated %s/%s after: %s",
					attemptIdx+1, len(plan), attempt.RungIndex, attempt.ExecutorName, model, lastReason)
			} else {
				noteDetail = fmt.Sprintf("attempt %d/%d rung %d %s/%s after: %s",
					attemptIdx+1, len(plan), attempt.RungIndex, attempt.ExecutorName, model, lastReason)
			}
			if err := deps.Ledger.AppendEvent(persistCtx, ledger.Event{
				RunID:  deps.RunID,
				LaneID: p.ID,
				Type:   ledger.EventLaneNote,
				Detail: noteDetail,
				At:     deps.Now(),
			}); err != nil {
				cause := fmt.Errorf("run: append retry note event for %q: %w", p.ID, err)
				report.Attempts = attemptsRun
				return report, recordLaneFailure(persistCtx, deps, p.ID, now, cause)
			}
		}
		prevRungIndex = attempt.RungIndex

		prompt := p.Body
		if attemptIdx > 0 {
			prompt = formatFeedback(p.Body, attemptIdx, lastVRes.FailedCommand, lastVRes.ExitCode, lastVRes.Output)
			// A stale envelope from the previous attempt must never be mistaken for
			// this attempt's result if the worker writes none.
			_ = os.Remove(filepath.Join(wt.Path, resultEnvelopePath))
		}
		// lastVRes describes only the attempt that just ran; clear it so a later
		// non-verification failure is not reclassified as loop exhaustion.
		lastVRes = verificationResult{}

		progress := make(chan executor.ProgressEvent, progressBufferSize)
		progressDone := make(chan []error, 1)
		appendProgressBatch := deps.AppendProgressBatch
		if appendProgressBatch == nil {
			appendProgressBatch = deps.Ledger.AppendProgressBatch
		}
		go func() {
			progressDone <- writeLaneProgress(context.WithoutCancel(ctx), appendProgressBatch, deps.RunID, p.ID, progress)
		}()

		attemptsRun++
		outcome, err := exec.Run(ctx, executor.Request{
			Prompt:         prompt,
			WorktreePath:   wt.Path,
			Model:          model,
			Agent:          p.Agent,
			ReadOnlyPaths:  append([]string(nil), p.ReadOnlyPaths...),
			RequiredSkills: append([]string(nil), p.RequiredSkills...),
			SchemaPath:     schemaPath,
			Progress:       progress,
		})
		close(progress)
		progressErrors := <-progressDone

		lastOutcome = outcome
		lastProgressErrors = progressErrors

		if err != nil {
			cause := fmt.Errorf("run: dispatch lane %q: %w", p.ID, err)
			report.Attempts = attemptsRun
			return report, recordLaneFailure(persistCtx, deps, p.ID, now, cause)
		}

		status, envelope, reason := decideStatus(deps, wt.Path, outcome)
		lastStatus = status
		lastEnvelope = envelope
		lastReason = reason

		if status != lane.Done {
			break
		}

		if len(p.AllowedPaths) > 0 {
			status, reason = enforceAllowedPaths(ctx, deps, wt.Path, wt.BaseSHA, p)
			if status != lane.Done {
				lastStatus = status
				lastReason = reason
				break
			}
		}

		status, reason = enforceRequiredSkills(p, envelope)
		if status != lane.Done {
			lastStatus = status
			lastReason = reason
			break
		}

		if isLoop {
			vRes := dispatcherVerify(ctx, deps, wt.Path, wt.BaseSHA, p)
			lastVRes = vRes
			if vRes.Status != lane.Done {
				lastStatus = vRes.Status
				lastReason = vRes.Reason
				if vRes.Retryable && attemptIdx+1 < len(plan) {
					continue
				}
				break
			}
			if p.CommitMessage != "" {
				cStatus, cReason := dispatcherCommitOnly(ctx, deps, wt.Path, p)
				if cStatus != lane.Done {
					lastStatus = cStatus
					lastReason = cReason
					break
				}
			}
			lastStatus = lane.Done
			lastReason = ""
			break
		} else {
			if p.CommitMessage != "" {
				status, reason = dispatcherCommit(ctx, deps, wt.Path, wt.BaseSHA, p)
				lastStatus = status
				lastReason = reason
			}
			break
		}
	}

	if lastStatus == lane.Done {
		status, reason := enforceCompletionMode(ctx, deps, wt.Path, wt.BaseSHA, p)
		if status != lane.Done {
			lastStatus = status
			lastReason = reason
		}
	} else if isLoop && lastVRes.Retryable && attemptsRun == len(plan) {
		if len(p.Escalation) == 0 {
			lastStatus = lane.Failed
			lastReason = fmt.Sprintf("write/test/fix loop exhausted after %d attempts: %s", attemptsRun, lastReason)
		} else {
			lastStatus = lane.Blocked
			lastReason = fmt.Sprintf("escalation ladder exhausted after %d attempts: %s", attemptsRun, lastReason)
		}
	}

	progressDiagnosis := reportProgressErrors(persistCtx, deps, p.ID, now, lastProgressErrors)
	report.Diagnosis = progressDiagnosis

	var diagnosis string
	if lastReason != "" {
		diagnosis = diagnosisDetail(lastReason, lastOutcome.Stderr, lastOutcome.Stdout)
		if err := deps.Ledger.AppendEvent(persistCtx, ledger.Event{
			RunID:  deps.RunID,
			LaneID: p.ID,
			Type:   ledger.EventLaneNote,
			Detail: diagnosis,
			At:     now,
		}); err != nil {
			cause := fmt.Errorf("run: append reason event for %q: %w", p.ID, err)
			return report, recordLaneFailure(persistCtx, deps, p.ID, now, cause)
		}
	}
	diagnosis = joinDiagnostics(diagnosis, progressDiagnosis)

	var terminalErr error
	if lastStatus == lane.Done {
		terminalErr = setDoneCandidate(persistCtx, deps, p, wt.Path, wt.BaseSHA, lastEnvelope, now)
	} else {
		terminalErr = deps.Ledger.SetStatus(persistCtx, deps.RunID, p.ID, lastStatus, now)
	}
	if terminalErr != nil {
		cause := fmt.Errorf("run: set lane %q terminal status: %w", p.ID, terminalErr)
		if lastStatus == lane.Done {
			cause = fmt.Errorf("run: set lane %q terminal status: freeze done candidate: %w", p.ID, terminalErr)
		}
		return report, recordLaneFailure(persistCtx, deps, p.ID, now, cause)
	}

	if lastOutcome.OutputTruncated {
		if err := deps.Ledger.AppendEvent(persistCtx, ledger.Event{
			RunID:  deps.RunID,
			LaneID: p.ID,
			Type:   ledger.EventLaneNote,
			Detail: outputTruncatedDetail,
			At:     now,
		}); err != nil {
			cause := fmt.Errorf("run: append output-truncated event for %q: %w", p.ID, err)
			return report, recordLaneFailure(persistCtx, deps, p.ID, now, cause)
		}
	}

	return Report{
		LaneID:                  p.ID,
		Status:                  lastStatus,
		Worktree:                wt.Path,
		Envelope:                lastEnvelope,
		OutputCaptureIncomplete: lastOutcome.OutputTruncated,
		Diagnosis:               diagnosis,
		Attempts:                attemptsRun,
	}, nil
}

func setDoneCandidate(ctx context.Context, deps Deps, p packet.Packet, worktreePath, recordedBaseSHA string, envelope *result.Envelope, at time.Time) error {
	resolve := deps.ResolveCandidateIdentity
	if resolve == nil {
		resolve = ResolveCandidateIdentityFromGit
	}
	identity, err := resolve(ctx, deps.PrimaryRoot, worktreePath, recordedBaseSHA)
	if err != nil {
		return err
	}
	if identity.BaseCommit == "" || identity.BaseTree == "" || identity.CandidateCommit == "" || identity.CandidateTree == "" {
		return errors.New("candidate identity is incomplete")
	}
	paths, err := normalizeAllowedPaths(p.AllowedPaths)
	if err != nil {
		return err
	}
	resultJSON, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("encode frozen result: %w", err)
	}
	resultHash := versionedHash("result:v1", string(resultJSON))
	digest := packetDigest(p, paths)
	candidate := ledger.LaneCandidate{
		RunID: deps.RunID, LaneID: p.ID, PacketID: p.ID, PacketDigest: digest,
		PrimaryRoot: deps.PrimaryRoot, WorktreePath: worktreePath,
		BaseCommit: identity.BaseCommit, BaseTree: identity.BaseTree,
		CandidateCommit: identity.CandidateCommit, CandidateTree: identity.CandidateTree,
		AllowedPaths: paths, ResultPath: resultEnvelopePath, ResultJSON: string(resultJSON),
		ResultHash: resultHash, RecordedAt: at,
	}
	if p.Authoring != nil {
		var contract struct {
			Version       string                        `json:"version"`
			Mode          string                        `json:"mode"`
			CommitMessage string                        `json:"commit_message,omitempty"`
			WritePaths    []string                      `json:"write_paths"`
			ReadOnlyPaths []string                      `json:"read_only_paths"`
			DoneCriteria  []string                      `json:"done_criteria"`
			HardStops     []string                      `json:"hard_stops"`
			Result        struct{ Path, Schema string } `json:"result"`
		}
		if err := json.Unmarshal(p.Authoring.ContractJSON, &contract); err != nil {
			return fmt.Errorf("decode compiled authoring contract: %w", err)
		}
		collect := deps.CollectCandidateChanges
		if collect == nil {
			collect = candidatechange.Collect
		}
		changes, err := collect(ctx, candidatechange.Request{Root: worktreePath, BaseCommit: identity.BaseCommit, CandidateCommit: identity.CandidateCommit})
		if err != nil {
			return fmt.Errorf("collect compiled candidate changes: %w", err)
		}
		commit := "required"
		if contract.Mode == "read-only" {
			commit = "forbidden"
		} else if contract.CommitMessage != "" || p.CommitMessage != "" {
			commit = "dispatcher"
		}
		digest = p.Authoring.Digest
		encoded, hash, err := ledger.FreezeAuthoringEvidence(ledger.AuthoringEvidence{
			PacketDigest: digest, AuthoringMode: "versioned", ContractVersion: contract.Version,
			Contract: append(json.RawMessage(nil), p.Authoring.ContractJSON...), Binding: append(json.RawMessage(nil), p.Authoring.BindingJSON...),
			Mode: contract.Mode, CommitObligation: commit, WritePaths: contract.WritePaths, ReadOnlyPaths: contract.ReadOnlyPaths,
			DoneCriteria: contract.DoneCriteria, HardStops: contract.HardStops, ResultPath: contract.Result.Path, ResultSchema: contract.Result.Schema,
			BaseCommit: identity.BaseCommit, BaseTree: identity.BaseTree, CandidateCommit: identity.CandidateCommit, CandidateTree: identity.CandidateTree,
			Changes: changes, ResultHash: resultHash,
		})
		if err != nil {
			return fmt.Errorf("freeze compiled authoring evidence: %w", err)
		}
		candidate.PacketDigest = digest
		candidate.AuthoringEvidenceVersion = ledger.AuthoringEvidenceVersion
		candidate.AuthoringEvidenceJSON = encoded
		candidate.AuthoringEvidenceHash = hash
	}
	return deps.Ledger.SetDoneCandidate(ctx, candidate)
}

// ResolveCandidateIdentityFromGit reads complete object IDs for terminal identity persistence.
func ResolveCandidateIdentityFromGit(ctx context.Context, primaryRoot, worktreePath, baseSHA string) (CandidateIdentity, error) {
	resolve := func(dir, rev string) (string, error) {
		out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--verify", rev).CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git rev-parse %s: %w: %s", rev, err, strings.TrimSpace(string(out)))
		}
		return strings.TrimSpace(string(out)), nil
	}
	baseCommit, err := resolve(primaryRoot, baseSHA+"^{commit}")
	if err != nil {
		return CandidateIdentity{}, err
	}
	baseTree, err := resolve(primaryRoot, baseCommit+"^{tree}")
	if err != nil {
		return CandidateIdentity{}, err
	}
	candidateCommit, err := resolve(worktreePath, "HEAD^{commit}")
	if err != nil {
		return CandidateIdentity{}, err
	}
	candidateTree, err := resolve(worktreePath, candidateCommit+"^{tree}")
	return CandidateIdentity{baseCommit, baseTree, candidateCommit, candidateTree}, err
}

func normalizeAllowedPaths(paths []string) ([]string, error) {
	seen := make(map[string]struct{}, len(paths))
	for _, raw := range paths {
		if raw == "" || filepath.IsAbs(raw) || strings.Contains(raw, `\`) {
			return nil, fmt.Errorf("invalid allowed path %q", raw)
		}
		clean := filepath.ToSlash(filepath.Clean(raw))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
			return nil, fmt.Errorf("invalid allowed path %q", raw)
		}
		seen[strings.TrimSuffix(clean, "/")] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for path := range seen {
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}

func packetDigest(p packet.Packet, paths []string) string {
	parts := []string{"packet:v1", p.ID, p.Executor, p.RoutedBy, p.Model, p.Agent,
		fmt.Sprint(p.ReadOnly), p.Feature, p.ParentRef, p.BaseSHA, p.ExpectedParentSHA,
		fmt.Sprint(p.LegacyMain), p.SDDPhase, p.FanoutGroup, p.Skill, skillset.DigestBody(p.Body)}
	parts = append(parts, paths...)
	parts = append(parts, p.ReadOnlyPaths...)
	parts = append(parts, p.LaneRole)

	requiredSkills := append([]string(nil), p.RequiredSkills...)
	sort.Strings(requiredSkills)
	parts = append(parts, strconv.Itoa(len(requiredSkills)))
	parts = append(parts, requiredSkills...)

	adhocSkills := append([]string(nil), p.AdhocSkills...)
	sort.Strings(adhocSkills)
	parts = append(parts, strconv.Itoa(len(adhocSkills)))
	parts = append(parts, adhocSkills...)

	if p.Route != "" {
		parts = append(parts, "route:"+p.Route)
	}
	if p.RouteEvidence != "" {
		parts = append(parts, "route_evidence:"+p.RouteEvidence)
	}
	if p.NamedSkillsOnly {
		parts = append(parts, "named_skills_only")
	}
	if len(p.Verification) > 0 {
		raw, _ := json.Marshal(p.Verification)
		parts = append(parts, "verification:"+string(raw))
	}
	if len(p.KnownEnvironmentalFailures) > 0 {
		raw, _ := json.Marshal(p.KnownEnvironmentalFailures)
		parts = append(parts, "known_env_failures:"+string(raw))
	}
	if p.CommitMessage != "" {
		parts = append(parts, "commit_message:"+p.CommitMessage)
	}
	if p.MaxIterations > 1 {
		parts = append(parts, fmt.Sprintf("max_iterations:%d", p.MaxIterations))
	}
	if len(p.Escalation) > 0 {
		raw, _ := json.Marshal(p.Escalation)
		parts = append(parts, "escalation:"+string(raw))
	}

	return versionedHash(parts...)
}

func versionedHash(parts ...string) string {
	h := sha256.New()
	var size [8]byte
	for _, part := range parts {
		binary.BigEndian.PutUint64(size[:], uint64(len(part)))
		_, _ = h.Write(size[:])
		_, _ = h.Write([]byte(part))
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}

func writeLaneProgress(
	ctx context.Context,
	appendBatch func(context.Context, []ledger.LaneProgress) error,
	runID, laneID string,
	progress <-chan executor.ProgressEvent,
) []error {
	ticker := time.NewTicker(progressFlushInterval)
	defer ticker.Stop()

	batch := make([]ledger.LaneProgress, 0, progressBatchSize)
	var writeErrors []error
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := appendBatch(ctx, batch); err != nil {
			writeErrors = append(writeErrors, err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case event, ok := <-progress:
			if !ok {
				flush()
				return writeErrors
			}
			batch = append(batch, ledger.LaneProgress{
				RunID: runID, LaneID: laneID, Message: event.Message, At: event.At,
				TotalTokens: event.TotalTokens, CostUSD: event.CostUSD, ToolCalls: event.ToolCalls,
			})
			if len(batch) == progressBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func reportProgressErrors(ctx context.Context, deps Deps, laneID string, at time.Time, writeErrors []error) string {
	if len(writeErrors) == 0 {
		return ""
	}

	messages := make([]string, len(writeErrors))
	for i, err := range writeErrors {
		messages[i] = err.Error()
	}
	detail := fmt.Sprintf("progress persistence failed for lane %q: %s", laneID, strings.Join(messages, "; "))
	if err := deps.Ledger.AppendEvent(ctx, ledger.Event{
		RunID: deps.RunID, LaneID: laneID, Type: ledger.EventLaneNote, Detail: detail, At: at,
	}); err != nil {
		return fmt.Sprintf("%s (additionally, failed to record the progress error in the ledger: %v)", detail, err)
	}
	return detail
}

func joinDiagnostics(parts ...string) string {
	var nonEmpty []string
	for _, part := range parts {
		if part != "" {
			nonEmpty = append(nonEmpty, part)
		}
	}
	return strings.Join(nonEmpty, "\n")
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
		for _, hs := range envelope.HardStops {
			if hs.Fired {
				return lane.Blocked, &envelope, "hard stop fired: " + hs.HardStop
			}
		}
		if envelope.Status == "interaction_required" {
			question := ""
			if envelope.Interaction != nil {
				question = envelope.Interaction.Question
			}
			return lane.Blocked, &envelope, "interaction required: " + question
		}
		return st, &envelope, ""
	}
}

// enforceAllowedPaths inspects the actual git diff of the worktree against
// its recorded BaseSHA using a four-way diff union (committed-since-base,
// unstaged, staged, and untracked). If any changed path is outside p.AllowedPaths,
// the lane is demoted to lane.Deviated with a reason listing the offending
// paths. If any git command fails, or if baseSHA is empty, the lane becomes
// lane.Blocked with a diagnostic reason.
func enforceAllowedPaths(ctx context.Context, deps Deps, worktreePath, baseSHA string, p packet.Packet) (lane.Status, string) {
	if strings.TrimSpace(baseSHA) == "" {
		return lane.Blocked, "worktree missing recorded base SHA"
	}

	changes, err := candidatechange.Collect(ctx, candidatechange.Request{
		Root:            worktreePath,
		BaseCommit:      baseSHA,
		CandidateCommit: "HEAD",
		IncludeWorktree: true,
	})
	if err != nil {
		return lane.Blocked, fmt.Sprintf("collect actual git diff in worktree failed: %v", err)
	}

	offending := candidatechange.OutOfScope(changes, p.AllowedPaths)

	if len(offending) > 0 {
		return lane.Deviated, fmt.Sprintf("actual diff touched paths outside declared allowed_paths: %s", strings.Join(offending, ", "))
	}

	return lane.Done, ""
}

// enforceRequiredSkills verifies that all skills declared in p.RequiredSkills
// are present in the result envelope's SkillsLoaded. If any required skill is
// omitted, the lane is demoted to lane.Deviated with an explanatory reason.
func enforceRequiredSkills(p packet.Packet, envelope *result.Envelope) (lane.Status, string) {
	if len(p.RequiredSkills) == 0 {
		return lane.Done, ""
	}
	if envelope == nil {
		return lane.Deviated, "result envelope is missing required skills declaration"
	}
	loaded := make(map[string]bool, len(envelope.SkillsLoaded))
	for _, s := range envelope.SkillsLoaded {
		loaded[strings.TrimSpace(s)] = true
		loaded[canonicalSkillName(s)] = true
	}
	var missing []string
	for _, req := range p.RequiredSkills {
		reqName := strings.TrimSpace(req)
		if !loaded[reqName] && !loaded[canonicalSkillName(reqName)] {
			missing = append(missing, req)
		}
	}
	if len(missing) > 0 {
		return lane.Deviated, fmt.Sprintf("result envelope skills_loaded omitted required skills: %s", strings.Join(missing, ", "))
	}
	return lane.Done, ""
}

func canonicalSkillName(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, "\\", "/")
	if strings.HasSuffix(s, "/SKILL.md") {
		s = strings.TrimSuffix(s, "/SKILL.md")
		if idx := strings.LastIndex(s, "/"); idx >= 0 {
			return s[idx+1:]
		}
		return s
	}
	if idx := strings.LastIndex(s, "/"); idx >= 0 {
		return s[idx+1:]
	}
	return s
}

// enforceCompletionMode verifies real git state after decideStatus mapped
// an envelope to lane.Done. A write packet requires at least one unique
// commit and a clean working tree; a read-only packet requires no unique
// commits and a clean working tree. If git inspection fails or git state
// violates the declared completion mode, the lane becomes lane.Failed with
// an explanatory reason.
func enforceCompletionMode(ctx context.Context, deps Deps, worktreePath, baseSHA string, p packet.Packet) (lane.Status, string) {
	hasCommits, err := deps.HasUniqueLaneCommits(ctx, worktreePath, baseSHA)
	if err != nil {
		return lane.Failed, fmt.Sprintf("check unique lane commits: %v", err)
	}

	porcelainEmpty, err := deps.PorcelainEmpty(ctx, worktreePath)
	if err != nil {
		return lane.Failed, fmt.Sprintf("check porcelain status: %v", err)
	}

	if !p.ReadOnly {
		if !hasCommits {
			return lane.Failed, "write packet completed without unique commits on lane branch"
		}
		if !porcelainEmpty {
			return lane.Failed, "write packet completed with uncommitted changes in worktree"
		}
		return lane.Done, ""
	}

	if hasCommits {
		return lane.Failed, "read-only packet completed with unique commits on lane branch"
	}
	if !porcelainEmpty {
		return lane.Failed, "read-only packet completed with uncommitted changes in worktree"
	}
	return lane.Done, ""
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
