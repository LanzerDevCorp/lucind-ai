// Package stability provides a pure, in-memory state machine for native stability
// campaigns and sequential trials. It has no clock, no timer, and no I/O: it
// evaluates transitions and timeout budgets based solely on the states and elapsed
// durations it is handed, and it never spawns goroutines or performs side effects.
package stability

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/feature"
	"github.com/LanzerDevCorp/lucind-ai/internal/integrate"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
	"github.com/LanzerDevCorp/lucind-ai/internal/run"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability/fixture"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability/process"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability/store"
)

// TargetConsecutivePasses is the number of sequential successful trials
// required for campaign verification eligibility.
const TargetConsecutivePasses = 3

// Execution timeout budgets.
const (
	DispatchTimeout = 10 * time.Minute
	TrialTimeout    = 45 * time.Minute
	CampaignTimeout = 135 * time.Minute
)

// Budgets holds the pure execution timeout limits.
type Budgets struct {
	Dispatch time.Duration
	Trial    time.Duration
	Campaign time.Duration
}

// DefaultBudgets specifies the standard stability campaign timeout budgets.
var DefaultBudgets = Budgets{
	Dispatch: DispatchTimeout,
	Trial:    TrialTimeout,
	Campaign: CampaignTimeout,
}

// CampaignState represents in-memory campaign lifecycle states.
type CampaignState string

const (
	CampaignPreflight      CampaignState = "preflight"
	CampaignRunning        CampaignState = CampaignState(store.StatusRunning)
	CampaignFailed         CampaignState = CampaignState(store.StatusFailed)
	CampaignBlockedCleanup CampaignState = CampaignState(store.StatusBlockedCleanup)
	CampaignPassed         CampaignState = CampaignState(store.StatusPassed)
)

// TrialState represents the discrete, closed stages of a single Trial.
type TrialState string

const (
	TrialAdmitted                    TrialState = "admitted"
	TrialDispatching                 TrialState = "dispatching"
	TrialAwaitingDefectAssessment    TrialState = "awaiting_defect_assessment"
	TrialAwaitingRemediationApproval TrialState = "awaiting_remediation_approval"
	TrialFixDispatched               TrialState = "fix_dispatched"
	TrialCrashInjected               TrialState = "crash_injected"
	TrialLeaseWait                   TrialState = "lease_wait"
	TrialReclaimed                   TrialState = "reclaimed"
	TrialPromoted                    TrialState = "promoted"
	TrialEvidenceCaptured            TrialState = "evidence_captured"
	TrialCleanedUp                   TrialState = "cleaned_up"
	TrialPassed                      TrialState = "passed"
	TrialFailed                      TrialState = "failed"
)

// Errors returned by state evaluations and transitions.
var (
	ErrInvalidCampaignState      = errors.New("stability: invalid campaign state")
	ErrInvalidCampaignTransition = errors.New("stability: illegal campaign state transition")
	ErrPreflightNotPersisted     = errors.New("stability: preflight state cannot be persisted to store")
	ErrInvalidTrialState         = errors.New("stability: invalid trial state")
	ErrInvalidTrialTransition    = errors.New("stability: illegal trial state transition")
	ErrCampaignNotRunning        = errors.New("stability: campaign is not running")
	ErrTrialAlreadyActive        = errors.New("stability: active trial is still running")
	ErrCampaignCompleted         = errors.New("stability: campaign has reached terminal state")
)

// Valid reports whether the campaign state is recognized.
func (s CampaignState) Valid() bool {
	switch s {
	case CampaignPreflight, CampaignRunning, CampaignFailed, CampaignBlockedCleanup, CampaignPassed:
		return true
	default:
		return false
	}
}

// Terminal reports whether the campaign state represents a closed terminal outcome.
func (s CampaignState) Terminal() bool {
	switch s {
	case CampaignFailed, CampaignBlockedCleanup, CampaignPassed:
		return true
	default:
		return false
	}
}

// Persistable reports whether the state corresponds to a persistent store.Status.
func (s CampaignState) Persistable() bool {
	switch s {
	case CampaignRunning, CampaignFailed, CampaignBlockedCleanup, CampaignPassed:
		return true
	default:
		return false
	}
}

// StoreStatus maps the in-memory CampaignState to the corresponding persistent store.Status.
// Preflight is strictly in-memory and returns ErrPreflightNotPersisted.
func (s CampaignState) StoreStatus() (store.Status, error) {
	switch s {
	case CampaignRunning:
		return store.StatusRunning, nil
	case CampaignFailed:
		return store.StatusFailed, nil
	case CampaignBlockedCleanup:
		return store.StatusBlockedCleanup, nil
	case CampaignPassed:
		return store.StatusPassed, nil
	case CampaignPreflight:
		return "", ErrPreflightNotPersisted
	default:
		return "", ErrInvalidCampaignState
	}
}

// Valid reports whether the trial state is recognized.
func (s TrialState) Valid() bool {
	switch s {
	case TrialAdmitted,
		TrialDispatching,
		TrialAwaitingDefectAssessment,
		TrialAwaitingRemediationApproval,
		TrialFixDispatched,
		TrialCrashInjected,
		TrialLeaseWait,
		TrialReclaimed,
		TrialPromoted,
		TrialEvidenceCaptured,
		TrialCleanedUp,
		TrialPassed,
		TrialFailed:
		return true
	default:
		return false
	}
}

// Terminal reports whether the trial outcome is final (passed or failed).
func (s TrialState) Terminal() bool {
	return s == TrialPassed || s == TrialFailed
}

// ParseTrialState parses a string into a valid TrialState, supporting both
// snake_case and kebab-case representations.
func ParseTrialState(str string) (TrialState, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(str), "-", "_")
	candidate := TrialState(normalized)
	if candidate.Valid() {
		return candidate, nil
	}
	return "", fmt.Errorf("%w: %q", ErrInvalidTrialState, str)
}

// ValidateCampaignTransition validates whether transitioning from -> to is legal.
func ValidateCampaignTransition(from, to CampaignState) error {
	if !from.Valid() || !to.Valid() {
		return ErrInvalidCampaignState
	}

	switch from {
	case CampaignPreflight:
		if to == CampaignRunning || to == CampaignFailed {
			return nil
		}
	case CampaignRunning:
		if to == CampaignPassed || to == CampaignFailed || to == CampaignBlockedCleanup {
			return nil
		}
	case CampaignBlockedCleanup:
		if to == CampaignFailed {
			return nil
		}
	case CampaignPassed, CampaignFailed:
		// Terminal states cannot transition to anything
		return fmt.Errorf("%w: cannot transition from terminal state %q to %q", ErrInvalidCampaignTransition, from, to)
	}

	return fmt.Errorf("%w: invalid transition from %q to %q", ErrInvalidCampaignTransition, from, to)
}

// ValidateTrialTransition validates whether transitioning from -> to is legal.
func ValidateTrialTransition(from, to TrialState) error {
	if !from.Valid() || !to.Valid() {
		return ErrInvalidTrialState
	}

	// Terminal states cannot transition to anything (strict zero retries)
	if from.Terminal() {
		return fmt.Errorf("%w: cannot transition from terminal trial state %q to %q", ErrInvalidTrialTransition, from, to)
	}

	// Any non-terminal state may transition to TrialFailed upon failure/timeout
	if to == TrialFailed {
		return nil
	}

	// Happy path progression
	switch from {
	case TrialAdmitted:
		if to == TrialDispatching {
			return nil
		}
	case TrialDispatching:
		if to == TrialAwaitingDefectAssessment {
			return nil
		}
	case TrialAwaitingDefectAssessment:
		if to == TrialAwaitingRemediationApproval {
			return nil
		}
	case TrialAwaitingRemediationApproval:
		if to == TrialFixDispatched {
			return nil
		}
	case TrialFixDispatched:
		if to == TrialCrashInjected {
			return nil
		}
	case TrialCrashInjected:
		if to == TrialLeaseWait {
			return nil
		}
	case TrialLeaseWait:
		if to == TrialReclaimed {
			return nil
		}
	case TrialReclaimed:
		if to == TrialPromoted {
			return nil
		}
	case TrialPromoted:
		if to == TrialEvidenceCaptured {
			return nil
		}
	case TrialEvidenceCaptured:
		if to == TrialCleanedUp {
			return nil
		}
	case TrialCleanedUp:
		if to == TrialPassed {
			return nil
		}
	}

	return fmt.Errorf("%w: invalid transition from %q to %q", ErrInvalidTrialTransition, from, to)
}

// IsBudgetExceeded reports whether elapsed duration strictly exceeds the budget limit.
func IsBudgetExceeded(elapsed, limit time.Duration) bool {
	return elapsed > limit
}

// CheckDispatchBudget reports whether the dispatch duration exceeded the budget.
func CheckDispatchBudget(elapsed, budget time.Duration) bool {
	return IsBudgetExceeded(elapsed, budget)
}

// CheckTrialBudget reports whether the trial duration exceeded the budget.
func CheckTrialBudget(elapsed, budget time.Duration) bool {
	return IsBudgetExceeded(elapsed, budget)
}

// CheckCampaignBudget reports whether the campaign duration exceeded the budget.
func CheckCampaignBudget(elapsed, budget time.Duration) bool {
	return IsBudgetExceeded(elapsed, budget)
}

// StateMachine coordinates the pure in-memory state of a Campaign and its Trials.
// It has no clock, timer, or I/O.
type StateMachine struct {
	state             CampaignState
	consecutivePasses int
	currentTrial      int
	activeTrialState  TrialState
	hasActiveTrial    bool
}

// NewStateMachine constructs a StateMachine initialized in CampaignPreflight.
func NewStateMachine() *StateMachine {
	return &StateMachine{
		state: CampaignPreflight,
	}
}

// State returns the current CampaignState.
func (sm *StateMachine) State() CampaignState {
	return sm.state
}

// ConsecutivePasses returns the current count of consecutive successful trials (0..3).
func (sm *StateMachine) ConsecutivePasses() int {
	return sm.consecutivePasses
}

// CurrentTrial returns the active or most recent trial number (1-based).
func (sm *StateMachine) CurrentTrial() int {
	return sm.currentTrial
}

// ActiveTrialState returns the current trial state if active.
func (sm *StateMachine) ActiveTrialState() TrialState {
	return sm.activeTrialState
}

// HasActiveTrial reports whether a trial is currently in-flight.
func (sm *StateMachine) HasActiveTrial() bool {
	return sm.hasActiveTrial
}

// IsEligibleForTerminalVerification reports whether 3 consecutive trials have passed.
func (sm *StateMachine) IsEligibleForTerminalVerification() bool {
	return sm.consecutivePasses >= TargetConsecutivePasses
}

// IsComplete reports whether the campaign has reached a terminal state.
func (sm *StateMachine) IsComplete() bool {
	return sm.state.Terminal()
}

// Start transitions the campaign from CampaignPreflight to CampaignRunning.
func (sm *StateMachine) Start() error {
	if err := ValidateCampaignTransition(sm.state, CampaignRunning); err != nil {
		return err
	}
	sm.state = CampaignRunning
	return nil
}

// TransitionCampaign transitions the campaign to the desired next state.
func (sm *StateMachine) TransitionCampaign(to CampaignState) error {
	if err := ValidateCampaignTransition(sm.state, to); err != nil {
		return err
	}
	sm.state = to
	return nil
}

// StartNextTrial starts the next trial, transitioning into TrialAdmitted.
// It fails if the campaign is not running, if a previous trial is still active,
// or if the campaign has already succeeded or failed.
func (sm *StateMachine) StartNextTrial() (int, error) {
	if sm.state != CampaignRunning {
		return 0, fmt.Errorf("%w: current state is %q", ErrCampaignNotRunning, sm.state)
	}
	if sm.hasActiveTrial {
		return 0, ErrTrialAlreadyActive
	}
	if sm.consecutivePasses >= TargetConsecutivePasses {
		return 0, ErrCampaignCompleted
	}

	sm.currentTrial++
	sm.activeTrialState = TrialAdmitted
	sm.hasActiveTrial = true
	return sm.currentTrial, nil
}

// AdvanceTrial advances the active trial to the specified next state.
func (sm *StateMachine) AdvanceTrial(to TrialState) error {
	if !sm.hasActiveTrial {
		return errors.New("stability: no active trial to advance")
	}
	if err := ValidateTrialTransition(sm.activeTrialState, to); err != nil {
		return err
	}
	sm.activeTrialState = to
	if to.Terminal() {
		sm.hasActiveTrial = false
	}
	return nil
}

// RecordTrialOutcome records the terminal outcome (TrialPassed or TrialFailed) of the active trial.
// On pass, the consecutive-pass count is incremented. If count reaches 3, Campaign transitions to CampaignPassed.
// On fail (or any failure/timeout), consecutive-pass count is IMMEDIATELY reset to 0, and Campaign transitions to CampaignFailed.
func (sm *StateMachine) RecordTrialOutcome(outcome TrialState) error {
	if !sm.hasActiveTrial {
		return errors.New("stability: no active trial to record outcome for")
	}
	if !outcome.Terminal() {
		return fmt.Errorf("stability: outcome %q is not a terminal trial state", outcome)
	}
	if err := ValidateTrialTransition(sm.activeTrialState, outcome); err != nil {
		return err
	}

	sm.activeTrialState = outcome
	sm.hasActiveTrial = false

	if outcome == TrialPassed {
		sm.consecutivePasses++
		if sm.consecutivePasses >= TargetConsecutivePasses {
			sm.state = CampaignPassed
		}
	} else {
		// Zero-retry reset: any failure resets count to zero immediately and fails campaign
		sm.consecutivePasses = 0
		sm.state = CampaignFailed
	}

	return nil
}

// DefaultLeaseTTL is the standard 10-second lease TTL for accelerated crash recovery.
const DefaultLeaseTTL = 10 * time.Second

// PinnedModel is the strict model required for stability dispatches.
const PinnedModel = "gemini-3.7-flash-high"

// JourneyConfig defines the execution parameters for a concurrent Trial journey.
type JourneyConfig struct {
	TargetA           string
	TargetB           string
	ParentRefA        string
	ParentRefB        string
	BaseSHA           string
	ExpectedParentSHA string
	OwnerA            string
	OwnerB            string
	ReplacementOwnerB string
	LeaseTTL          time.Duration
}

// DefaultJourneyConfig creates a default JourneyConfig for a given trial number and base SHA.
func DefaultJourneyConfig(trialNum int, baseSHA string) JourneyConfig {
	return JourneyConfig{
		TargetA:           fmt.Sprintf("stability-trial-%d-target-a", trialNum),
		TargetB:           fmt.Sprintf("stability-trial-%d-target-b", trialNum),
		ParentRefA:        fmt.Sprintf("refs/heads/stability-trial-%d-a", trialNum),
		ParentRefB:        fmt.Sprintf("refs/heads/stability-trial-%d-b", trialNum),
		BaseSHA:           baseSHA,
		ExpectedParentSHA: baseSHA,
		OwnerA:            fmt.Sprintf("orchestrator-trial-%d-a", trialNum),
		OwnerB:            fmt.Sprintf("orchestrator-trial-%d-b", trialNum),
		ReplacementOwnerB: fmt.Sprintf("orchestrator-trial-%d-b-reclaim", trialNum),
		LeaseTTL:          DefaultLeaseTTL,
	}
}

// BuildJourneyPackets constructs Change A and Change B packets from fixture templates,
// injecting feature targets, parent refs, base SHAs, and pinned model.
func BuildJourneyPackets(cfg JourneyConfig) (packet.Packet, packet.Packet) {
	pA := fixture.ChangeAPacket()
	pA.Feature = cfg.TargetA
	pA.ParentRef = cfg.ParentRefA
	pA.BaseSHA = cfg.BaseSHA
	pA.ExpectedParentSHA = cfg.ExpectedParentSHA
	pA.Model = PinnedModel

	pB := fixture.ChangeBPacket()
	pB.Feature = cfg.TargetB
	pB.ParentRef = cfg.ParentRefB
	pB.BaseSHA = cfg.BaseSHA
	pB.ExpectedParentSHA = cfg.ExpectedParentSHA
	pB.Model = PinnedModel

	return pA, pB
}

// DispatchConcurrentLanes dispatches Change A and Change B concurrently through the existing
// internal/run.ExecuteBatch primitive.
func DispatchConcurrentLanes(ctx context.Context, deps run.Deps, pA, pB packet.Packet) (run.BatchReport, error) {
	return run.ExecuteBatch(ctx, deps, []packet.Packet{pA, pB})
}

// AcquireTargetLeasesConcurrently acquires Ownership Leases for both Target A and Target B
// concurrently using feature.Service, ensuring both orchestrators hold active leases.
func AcquireTargetLeasesConcurrently(ctx context.Context, featSvc *feature.Service, cfg JourneyConfig) (feature.Lease, feature.Lease, error) {
	ttl := cfg.LeaseTTL
	if ttl <= 0 {
		ttl = DefaultLeaseTTL
	}

	var (
		leaseA, leaseB feature.Lease
		errA, errB     error
		wg             sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		leaseA, errA = featSvc.AcquireLease(ctx, cfg.TargetA, cfg.OwnerA, ttl)
	}()
	go func() {
		defer wg.Done()
		leaseB, errB = featSvc.AcquireLease(ctx, cfg.TargetB, cfg.OwnerB, ttl)
	}()
	wg.Wait()

	if errA != nil {
		return leaseA, leaseB, fmt.Errorf("stability: acquire lease for target A (%s): %w", cfg.TargetA, errA)
	}
	if errB != nil {
		return leaseA, leaseB, fmt.Errorf("stability: acquire lease for target B (%s): %w", cfg.TargetB, errB)
	}

	return leaseA, leaseB, nil
}

// AdoptResultEnvelope reads and returns the already-persisted result envelope from the worktree.
func AdoptResultEnvelope(fsys fs.FS, worktreePath string) (result.Envelope, error) {
	envelope, err := result.Read(fsys, ".lucind/result.json")
	if err != nil {
		return result.Envelope{}, fmt.Errorf("stability: read persisted result envelope in %s: %w", worktreePath, err)
	}
	return envelope, nil
}

// VerifyZeroSurvivors checks that no surviving descendant processes remain in pgid.
func VerifyZeroSurvivors(pgid int) error {
	return process.VerifyZeroSurvivors(pgid)
}

// ReclaimTargetLease re-acquires the lease for a target after expiry, verifying monotonic fence increment.
func ReclaimTargetLease(ctx context.Context, featSvc *feature.Service, target, newOwner string, initialFence int64, ttl time.Duration) (feature.Lease, error) {
	if ttl <= 0 {
		ttl = DefaultLeaseTTL
	}
	reclaimed, err := featSvc.AcquireLease(ctx, target, newOwner, ttl)
	if err != nil {
		return feature.Lease{}, fmt.Errorf("stability: reclaim lease for target %s: %w", target, err)
	}
	if reclaimed.Fence <= initialFence {
		return reclaimed, fmt.Errorf("stability: reclaimed lease fence %d is not strictly greater than initial fence %d", reclaimed.Fence, initialFence)
	}
	return reclaimed, nil
}

// RecoverCrashedChangeB orchestrates the full recovery of crashed Change B:
// 1. Verifies zero surviving processes in Change B's process group (if pgid > 1).
// 2. Reclaims Change B's feature lease with incremented fence.
// 3. Adopts Change B's already-persisted result envelope without redispatch.
func RecoverCrashedChangeB(
	ctx context.Context,
	featSvc *feature.Service,
	fsys fs.FS,
	worktreePath string,
	target string,
	newOwner string,
	initialFence int64,
	pgid int,
	ttl time.Duration,
) (feature.Lease, result.Envelope, error) {
	if pgid > 1 {
		if err := VerifyZeroSurvivors(pgid); err != nil {
			return feature.Lease{}, result.Envelope{}, fmt.Errorf("stability: zero-survivor audit failed: %w", err)
		}
	}

	reclaimed, err := ReclaimTargetLease(ctx, featSvc, target, newOwner, initialFence, ttl)
	if err != nil {
		return feature.Lease{}, result.Envelope{}, err
	}

	env, err := AdoptResultEnvelope(fsys, worktreePath)
	if err != nil {
		return reclaimed, result.Envelope{}, err
	}

	return reclaimed, env, nil
}

// TrialJourneyResult holds the outcome and artifacts of a Trial concurrent journey.
type TrialJourneyResult struct {
	BatchReport      run.BatchReport
	LeaseA           feature.Lease
	InitialLeaseB    feature.Lease
	ReclaimedLeaseB  feature.Lease
	AdoptedEnvelopeB result.Envelope
	DefectRecord     *DefectRecord
	Approval         *RemediationApproval
	FixReport        *run.Report
}

// ChangeBEnvelopeWatchTimeout bounds how long ExecuteTrialJourneyLive waits for
// Change B's real dispatch to persist its result envelope before giving up on
// injecting a live kill (falling back to letting the dispatch finish naturally).
// Comfortably larger than the fixture's own sleep-5-after-write window
// (internal/stability/fixture/fixture.go's ChangeBPacket) so a real dispatch has
// margin to actually reach that point.
const ChangeBEnvelopeWatchTimeout = 15 * time.Second

// ExecuteTrialJourneyLive runs the concurrent trial journey with live process
// group tracking and abrupt termination upon envelope persistence.
func ExecuteTrialJourneyLive(
	ctx context.Context,
	sm *StateMachine,
	deps run.Deps,
	featSvc *feature.Service,
	cfg JourneyConfig,
) (*TrialJourneyResult, error) {
	if !sm.HasActiveTrial() {
		if _, err := sm.StartNextTrial(); err != nil {
			return nil, fmt.Errorf("stability: start next trial: %w", err)
		}
	}

	// 1. Acquire preflight ownership leases concurrently
	leaseA, leaseB, err := AcquireTargetLeasesConcurrently(ctx, featSvc, cfg)
	if err != nil {
		_ = sm.RecordTrialOutcome(TrialFailed)
		return nil, err
	}

	// 2. Advance to dispatching and run ExecuteBatch
	if err := sm.AdvanceTrial(TrialDispatching); err != nil {
		_ = sm.RecordTrialOutcome(TrialFailed)
		return nil, err
	}

	pA, pB := BuildJourneyPackets(cfg)
	worktreeBPath := filepath.Join(deps.PrimaryRoot, "wt-"+pB.ID)

	pidBChan := make(chan int, 1)
	liveDeps := deps
	liveDeps.Setpgid = true
	liveDeps.OnProcessStart = func(laneID string, pid int) {
		if laneID == pB.ID {
			select {
			case pidBChan <- pid:
			default:
			}
		}
	}

	type dispatchResult struct {
		report run.BatchReport
		err    error
	}
	dispatchDone := make(chan dispatchResult, 1)
	go func() {
		rep, dErr := DispatchConcurrentLanes(ctx, liveDeps, pA, pB)
		dispatchDone <- dispatchResult{report: rep, err: dErr}
	}()

	var (
		capturedPGID     int
		dispatchRes      dispatchResult
		dispatchFinished bool
	)

	watchTimer := time.NewTimer(ChangeBEnvelopeWatchTimeout)
	defer watchTimer.Stop()

	select {
	case <-ctx.Done():
	case <-watchTimer.C:
	case res := <-dispatchDone:
		dispatchRes = res
		dispatchFinished = true
	case pid := <-pidBChan:
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()

	pollLoop:
		for {
			select {
			case <-ctx.Done():
				break pollLoop
			case <-watchTimer.C:
				break pollLoop
			case res := <-dispatchDone:
				dispatchRes = res
				dispatchFinished = true
				break pollLoop
			case <-ticker.C:
				env, readErr := result.Read(deps.WorktreeFS(worktreeBPath), ".lucind/result.json")
				if readErr == nil && env.PacketID == pB.ID {
					_ = process.KillGroup(pid)
					capturedPGID = pid
					break pollLoop
				}
			}
		}
	}

	if !dispatchFinished {
		select {
		case <-ctx.Done():
			dispatchRes = <-dispatchDone
		case res := <-dispatchDone:
			dispatchRes = res
		}
	}

	if dispatchRes.err != nil {
		_ = sm.RecordTrialOutcome(TrialFailed)
		return nil, fmt.Errorf("stability: dispatch batch: %w", dispatchRes.err)
	}

	return continueTrialAfterDispatch(
		ctx, sm, deps, featSvc, cfg, dispatchRes.report, pA, pB,
		leaseA, leaseB, capturedPGID, worktreeBPath,
	)
}

// ExecuteTrialJourney runs the concurrent journey for Changes A and B, coordinating
// with the StateMachine through all required stage transitions.
func ExecuteTrialJourney(
	ctx context.Context,
	sm *StateMachine,
	deps run.Deps,
	featSvc *feature.Service,
	cfg JourneyConfig,
	pgidB int,
	worktreeBPath string,
) (*TrialJourneyResult, error) {
	if !sm.HasActiveTrial() {
		if _, err := sm.StartNextTrial(); err != nil {
			return nil, fmt.Errorf("stability: start next trial: %w", err)
		}
	}

	// 1. Acquire preflight ownership leases concurrently
	leaseA, leaseB, err := AcquireTargetLeasesConcurrently(ctx, featSvc, cfg)
	if err != nil {
		_ = sm.RecordTrialOutcome(TrialFailed)
		return nil, err
	}

	// 2. Advance to dispatching and run ExecuteBatch
	if err := sm.AdvanceTrial(TrialDispatching); err != nil {
		_ = sm.RecordTrialOutcome(TrialFailed)
		return nil, err
	}

	pA, pB := BuildJourneyPackets(cfg)
	batchReport, err := DispatchConcurrentLanes(ctx, deps, pA, pB)
	if err != nil {
		_ = sm.RecordTrialOutcome(TrialFailed)
		return nil, fmt.Errorf("stability: dispatch batch: %w", err)
	}

	return continueTrialAfterDispatch(
		ctx, sm, deps, featSvc, cfg, batchReport, pA, pB,
		leaseA, leaseB, pgidB, worktreeBPath,
	)
}

func continueTrialAfterDispatch(
	ctx context.Context,
	sm *StateMachine,
	deps run.Deps,
	featSvc *feature.Service,
	cfg JourneyConfig,
	batchReport run.BatchReport,
	pA, pB packet.Packet,
	leaseA, leaseB feature.Lease,
	pgidB int,
	worktreeBPath string,
) (*TrialJourneyResult, error) {
	// 3. Progress to defect assessment
	if err := sm.AdvanceTrial(TrialAwaitingDefectAssessment); err != nil {
		_ = sm.RecordTrialOutcome(TrialFailed)
		return nil, err
	}

	wtPathA := filepath.Join(deps.PrimaryRoot, "wt-"+pA.ID)
	defectRec, _ := ReadDefectRecord(deps.WorktreeFS(wtPathA), wtPathA)
	var defectPtr *DefectRecord
	if defectRec.ChangeID != "" {
		defectPtr = &defectRec
	} else if len(batchReport.Lanes) > 0 && batchReport.Lanes[0].Status == lane.Blocked {
		d, err := AssessAndRecordDefect(ctx, deps.Ledger, wtPathA, pA.ID, "fixture/check.sh", "CHECK FAILURE: Seeded defect present in fixture/defect.txt", deps.Now())
		if err == nil {
			defectPtr = d
		}
	}

	// 4. Progress to remediation approval
	if err := sm.AdvanceTrial(TrialAwaitingRemediationApproval); err != nil {
		_ = sm.RecordTrialOutcome(TrialFailed)
		return nil, err
	}

	var approvalPtr *RemediationApproval
	if defectPtr != nil {
		app, err := RecordTestActorApproval(ctx, deps.Ledger, wtPathA, *defectPtr, DefaultTestActor, true, "remediation approved for fixture defect", deps.Now())
		if err != nil {
			_ = sm.RecordTrialOutcome(TrialFailed)
			return nil, err
		}
		approvalPtr = app
	}

	// 5. Progress to fix dispatched
	if err := sm.AdvanceTrial(TrialFixDispatched); err != nil {
		_ = sm.RecordTrialOutcome(TrialFailed)
		return nil, err
	}

	var fixReportPtr *run.Report
	if approvalPtr != nil && approvalPtr.Approved {
		rep, err := DispatchFixChange(ctx, deps, cfg, approvalPtr)
		if err == nil {
			fixReportPtr = &rep
		}
	}

	// 6. Crash injection stage
	if err := sm.AdvanceTrial(TrialCrashInjected); err != nil {
		_ = sm.RecordTrialOutcome(TrialFailed)
		return nil, err
	}

	// 7. Lease wait stage
	if err := sm.AdvanceTrial(TrialLeaseWait); err != nil {
		_ = sm.RecordTrialOutcome(TrialFailed)
		return nil, err
	}

	// In TrialLeaseWait, wait for Change B's lease TTL to expire if still held
	now := time.Now().UTC()
	if now.Before(leaseB.ExpiresAt) {
		time.Sleep(leaseB.ExpiresAt.Sub(now) + 10*time.Millisecond)
	}

	// 8. Recover Change B (zero survivors check -> lease reclaim -> envelope adoption)
	reclaimedB, adoptedEnvB, err := RecoverCrashedChangeB(
		ctx,
		featSvc,
		deps.WorktreeFS(worktreeBPath),
		worktreeBPath,
		cfg.TargetB,
		cfg.ReplacementOwnerB,
		leaseB.Fence,
		pgidB,
		cfg.LeaseTTL,
	)
	if err != nil {
		_ = sm.RecordTrialOutcome(TrialFailed)
		return nil, err
	}

	if err := sm.AdvanceTrial(TrialReclaimed); err != nil {
		_ = sm.RecordTrialOutcome(TrialFailed)
		return nil, err
	}

	// 9. Promote stage
	if err := sm.AdvanceTrial(TrialPromoted); err != nil {
		_ = sm.RecordTrialOutcome(TrialFailed)
		return nil, err
	}

	// 10. Evidence capture stage
	if err := sm.AdvanceTrial(TrialEvidenceCaptured); err != nil {
		_ = sm.RecordTrialOutcome(TrialFailed)
		return nil, err
	}

	return &TrialJourneyResult{
		BatchReport:      batchReport,
		LeaseA:           leaseA,
		InitialLeaseB:    leaseB,
		ReclaimedLeaseB:  reclaimedB,
		AdoptedEnvelopeB: adoptedEnvB,
		DefectRecord:     defectPtr,
		Approval:         approvalPtr,
		FixReport:        fixReportPtr,
	}, nil
}

// DefectRecord describes a detected out-of-scope fixture defect.
type DefectRecord struct {
	ChangeID     string    `json:"change_id"`
	FixtureCheck string    `json:"fixture_check"`
	Reason       string    `json:"reason"`
	DiscoveredAt time.Time `json:"discovered_at"`
}

// PersistDefectRecord writes the DefectRecord to .lucind/defect_record.json in worktreePath.
func PersistDefectRecord(worktreePath string, record DefectRecord) error {
	lucindDir := filepath.Join(worktreePath, ".lucind")
	if err := os.MkdirAll(lucindDir, 0o755); err != nil {
		return fmt.Errorf("stability: create .lucind directory in %s: %w", worktreePath, err)
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("stability: marshal defect record: %w", err)
	}
	target := filepath.Join(lucindDir, "defect_record.json")
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return fmt.Errorf("stability: write defect record to %s: %w", target, err)
	}
	return nil
}

// ReadDefectRecord reads and parses a DefectRecord from .lucind/defect_record.json in worktreePath.
func ReadDefectRecord(fsys fs.FS, worktreePath string) (DefectRecord, error) {
	var data []byte
	var err error
	if fsys != nil {
		data, err = fs.ReadFile(fsys, ".lucind/defect_record.json")
	} else {
		data, err = os.ReadFile(filepath.Join(worktreePath, ".lucind", "defect_record.json"))
	}
	if err != nil {
		return DefectRecord{}, fmt.Errorf("stability: read defect record in %s: %w", worktreePath, err)
	}
	var record DefectRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return DefectRecord{}, fmt.Errorf("stability: unmarshal defect record in %s: %w", worktreePath, err)
	}
	return record, nil
}

// AssessAndRecordDefect creates and persists a DefectRecord when a fixture check fails.
func AssessAndRecordDefect(ctx context.Context, l *ledger.Ledger, worktreePath, changeID, checkName, failureOutput string, now time.Time) (*DefectRecord, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	record := DefectRecord{
		ChangeID:     changeID,
		FixtureCheck: checkName,
		Reason:       strings.TrimSpace(failureOutput),
		DiscoveredAt: now,
	}
	if err := PersistDefectRecord(worktreePath, record); err != nil {
		return nil, err
	}
	if l != nil {
		_ = l.AppendEvent(ctx, ledger.Event{
			RunID:  "",
			LaneID: changeID,
			Type:   ledger.EventLaneNote,
			Detail: fmt.Sprintf("defect_recorded: %s", record.Reason),
			At:     now,
		})
	}
	return &record, nil
}

// DefaultTestActor is the canonical deterministic actor name for automated stability remediation decisions.
const DefaultTestActor = "stability-test-actor"

// ErrRemediationNotApproved is returned when remediation dispatch is attempted without prior Test Actor approval.
var ErrRemediationNotApproved = errors.New("stability: remediation proposal has not been approved by Test Actor")

// RemediationApproval represents a deterministic, durable decision on a DefectRecord.
type RemediationApproval struct {
	ChangeID  string    `json:"change_id"`
	Approver  string    `json:"approver"`
	Approved  bool      `json:"approved"`
	Reason    string    `json:"reason"`
	DecidedAt time.Time `json:"decided_at"`
}

// PersistRemediationApproval writes the RemediationApproval to .lucind/remediation_approval.json in worktreePath.
func PersistRemediationApproval(worktreePath string, approval RemediationApproval) error {
	lucindDir := filepath.Join(worktreePath, ".lucind")
	if err := os.MkdirAll(lucindDir, 0o755); err != nil {
		return fmt.Errorf("stability: create .lucind directory in %s: %w", worktreePath, err)
	}
	data, err := json.MarshalIndent(approval, "", "  ")
	if err != nil {
		return fmt.Errorf("stability: marshal remediation approval: %w", err)
	}
	target := filepath.Join(lucindDir, "remediation_approval.json")
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return fmt.Errorf("stability: write remediation approval to %s: %w", target, err)
	}
	return nil
}

// ReadRemediationApproval reads and parses a RemediationApproval from .lucind/remediation_approval.json in worktreePath.
func ReadRemediationApproval(fsys fs.FS, worktreePath string) (RemediationApproval, error) {
	var data []byte
	var err error
	if fsys != nil {
		data, err = fs.ReadFile(fsys, ".lucind/remediation_approval.json")
	} else {
		data, err = os.ReadFile(filepath.Join(worktreePath, ".lucind", "remediation_approval.json"))
	}
	if err != nil {
		return RemediationApproval{}, fmt.Errorf("stability: read remediation approval in %s: %w", worktreePath, err)
	}
	var approval RemediationApproval
	if err := json.Unmarshal(data, &approval); err != nil {
		return RemediationApproval{}, fmt.Errorf("stability: unmarshal remediation approval in %s: %w", worktreePath, err)
	}
	return approval, nil
}

// RecordTestActorApproval records a deterministic approval decision for a DefectRecord.
func RecordTestActorApproval(
	ctx context.Context,
	l *ledger.Ledger,
	worktreePath string,
	defect DefectRecord,
	actor string,
	approved bool,
	reason string,
	now time.Time,
) (*RemediationApproval, error) {
	if actor == "" {
		actor = DefaultTestActor
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	app := RemediationApproval{
		ChangeID:  defect.ChangeID,
		Approver:  actor,
		Approved:  approved,
		Reason:    reason,
		DecidedAt: now,
	}

	if err := PersistRemediationApproval(worktreePath, app); err != nil {
		return nil, err
	}

	if l != nil {
		decision := ledger.DecisionApproved
		if !approved {
			decision = ledger.DecisionRejected
		}
		// Register or update approval in SQLite ledger
		ledgerApp := ledger.Approval{
			RunID:       "run-journey-trial-1",
			LaneID:      defect.ChangeID,
			PacketID:    defect.ChangeID,
			Approver:    actor,
			Decision:    decision,
			Evidence:    defect.Reason,
			RequestedAt: defect.DiscoveredAt,
			DecidedAt:   &now,
		}
		_ = l.RequestApproval(ctx, ledgerApp)
		_ = l.AppendEvent(ctx, ledger.Event{
			RunID:  "run-journey-trial-1",
			LaneID: defect.ChangeID,
			Type:   "remediation_approval_decided",
			Detail: fmt.Sprintf("actor=%s decision=%s reason=%s", actor, decision, reason),
			At:     now,
		})
	}

	return &app, nil
}

// VerifyRemediationApproval checks that a valid, approved RemediationApproval exists for changeID.
func VerifyRemediationApproval(fsys fs.FS, worktreePath string, changeID string) (*RemediationApproval, error) {
	app, err := ReadRemediationApproval(fsys, worktreePath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRemediationNotApproved, err)
	}
	if !app.Approved {
		return &app, fmt.Errorf("%w: remediation for %s was rejected: %s", ErrRemediationNotApproved, changeID, app.Reason)
	}
	if app.Approver == "" {
		return &app, fmt.Errorf("%w: missing approver identity", ErrRemediationNotApproved)
	}
	if app.DecidedAt.IsZero() {
		return &app, fmt.Errorf("%w: missing decision timestamp", ErrRemediationNotApproved)
	}
	return &app, nil
}

// PromoteTargetCAS promotes candidateSHA to parentRef using CAS semantics via integrate.PromoteCAS.
func PromoteTargetCAS(ctx context.Context, primaryRoot, parentRef, candidateSHA, expectedSHA string) error {
	return integrate.PromoteCAS(ctx, primaryRoot, parentRef, candidateSHA, expectedSHA)
}

// BuildFixPacket constructs a Fix Change packet from fixture templates,
// injecting feature Target A, ParentRefA, BaseSHA, and pinned model.
func BuildFixPacket(cfg JourneyConfig) packet.Packet {
	pFix := fixture.FixChangePacket()
	pFix.Feature = cfg.TargetA
	pFix.ParentRef = cfg.ParentRefA
	pFix.BaseSHA = cfg.BaseSHA
	pFix.ExpectedParentSHA = cfg.ExpectedParentSHA
	pFix.Model = PinnedModel
	return pFix
}

// DispatchFixChange dispatches the Fix Change lane through run.Execute, gated on prior approval.
func DispatchFixChange(ctx context.Context, deps run.Deps, cfg JourneyConfig, approval *RemediationApproval) (run.Report, error) {
	if approval == nil || !approval.Approved {
		return run.Report{}, fmt.Errorf("%w: cannot dispatch fix change without valid approval", ErrRemediationNotApproved)
	}
	pFix := BuildFixPacket(cfg)
	return run.Execute(ctx, deps, pFix)
}

// ErrDependencyNotSatisfied is returned when Change A promotion is attempted before Fix promotion is confirmed.
var ErrDependencyNotSatisfied = errors.New("stability: remediation fix dependency is not satisfied")

// PromoteChangeACAS promotes Change A's candidate commit to Target A using CAS semantics,
// enforcing that Fix promotion to Target A has been confirmed satisfied first.
func PromoteChangeACAS(ctx context.Context, primaryRoot string, cfg JourneyConfig, candidateASHA, fixSHA string, fixPromoted bool) error {
	if !fixPromoted {
		return fmt.Errorf("%w: fix change has not been promoted to target A (%s)", ErrDependencyNotSatisfied, cfg.TargetA)
	}
	return integrate.PromoteCAS(ctx, primaryRoot, cfg.ParentRefA, candidateASHA, fixSHA)
}

// PromoteTargetBCAS promotes Change B's candidate commit to Target B using CAS semantics.
func PromoteTargetBCAS(ctx context.Context, primaryRoot string, cfg JourneyConfig, candidateBSHA string) error {
	return integrate.PromoteCAS(ctx, primaryRoot, cfg.ParentRefB, candidateBSHA, cfg.ExpectedParentSHA)
}
