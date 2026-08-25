// Package stability provides a pure, in-memory state machine for native stability
// campaigns and sequential trials. It has no clock, no timer, and no I/O: it
// evaluates transitions and timeout budgets based solely on the states and elapsed
// durations it is handed, and it never spawns goroutines or performs side effects.
package stability

import (
	"errors"
	"fmt"
	"strings"
	"time"

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
	ErrInvalidTrialTransition   = errors.New("stability: illegal trial state transition")
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
	state                CampaignState
	consecutivePasses    int
	currentTrial         int
	activeTrialState     TrialState
	hasActiveTrial       bool
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
