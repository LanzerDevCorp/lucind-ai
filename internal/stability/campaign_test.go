package stability_test

import (
	"errors"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/stability"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability/store"
)

func runFullPassingTrial(t *testing.T, sm *stability.StateMachine) {
	t.Helper()
	stages := []stability.TrialState{
		stability.TrialDispatching,
		stability.TrialAwaitingDefectAssessment,
		stability.TrialAwaitingRemediationApproval,
		stability.TrialFixDispatched,
		stability.TrialCrashInjected,
		stability.TrialLeaseWait,
		stability.TrialReclaimed,
		stability.TrialPromoted,
		stability.TrialEvidenceCaptured,
		stability.TrialCleanedUp,
	}

	for _, stage := range stages {
		if err := sm.AdvanceTrial(stage); err != nil {
			t.Fatalf("AdvanceTrial(%q) failed: %v", stage, err)
		}
	}

	if err := sm.RecordTrialOutcome(stability.TrialPassed); err != nil {
		t.Fatalf("RecordTrialOutcome(TrialPassed) failed: %v", err)
	}
}

func TestCampaignStatesMatchStoreStatus(t *testing.T) {
	// Assert the 4 persisted campaign states are identical to store.Status values.
	storeStatuses := []store.Status{
		store.StatusRunning,
		store.StatusFailed,
		store.StatusBlockedCleanup,
		store.StatusPassed,
	}

	campaignStates := []stability.CampaignState{
		stability.CampaignRunning,
		stability.CampaignFailed,
		stability.CampaignBlockedCleanup,
		stability.CampaignPassed,
	}

	if len(storeStatuses) != len(campaignStates) {
		t.Fatalf("status count mismatch: store has %d, campaign has %d", len(storeStatuses), len(campaignStates))
	}

	for i, ss := range storeStatuses {
		cs := campaignStates[i]
		if string(ss) != string(cs) {
			t.Errorf("state mismatch at %d: store.Status = %q, CampaignState = %q", i, ss, cs)
		}
		gotStoreStatus, err := cs.StoreStatus()
		if err != nil {
			t.Errorf("cs.StoreStatus() failed for %q: %v", cs, err)
		}
		if gotStoreStatus != ss {
			t.Errorf("cs.StoreStatus() = %q, want %q", gotStoreStatus, ss)
		}
	}

	// Assert Persistable() behavior
	for _, cs := range campaignStates {
		if !cs.Persistable() {
			t.Errorf("cs.Persistable() = false for %q, want true", cs)
		}
	}
	if stability.CampaignPreflight.Persistable() {
		t.Errorf("CampaignPreflight.Persistable() = true, want false")
	}

	// Assert preflight is in-memory only and cannot be converted to store.Status.
	if _, err := stability.CampaignPreflight.StoreStatus(); err == nil {
		t.Errorf("CampaignPreflight.StoreStatus() returned nil error, want error because preflight is unpersisted")
	} else if !errors.Is(err, stability.ErrPreflightNotPersisted) {
		t.Errorf("CampaignPreflight.StoreStatus() error = %v, want %v", err, stability.ErrPreflightNotPersisted)
	}

	// Assert invalid campaign state is rejected.
	invalidState := stability.CampaignState("invalid_state")
	if invalidState.Valid() {
		t.Errorf("invalidState.Valid() = true, want false")
	}
	if invalidState.Persistable() {
		t.Errorf("invalidState.Persistable() = true, want false")
	}
	if _, err := invalidState.StoreStatus(); err == nil {
		t.Errorf("invalidState.StoreStatus() returned nil error, want error")
	}
}

func TestCampaignTrialStatesClosedSet(t *testing.T) {
	expectedStates := []stability.TrialState{
		stability.TrialAdmitted,
		stability.TrialDispatching,
		stability.TrialAwaitingDefectAssessment,
		stability.TrialAwaitingRemediationApproval,
		stability.TrialFixDispatched,
		stability.TrialCrashInjected,
		stability.TrialLeaseWait,
		stability.TrialReclaimed,
		stability.TrialPromoted,
		stability.TrialEvidenceCaptured,
		stability.TrialCleanedUp,
		stability.TrialPassed,
		stability.TrialFailed,
	}

	for _, s := range expectedStates {
		if !s.Valid() {
			t.Errorf("state %q reported as invalid", s)
		}
		parsed, err := stability.ParseTrialState(string(s))
		if err != nil {
			t.Errorf("ParseTrialState(%q) failed: %v", s, err)
		}
		if parsed != s {
			t.Errorf("ParseTrialState(%q) = %q, want %q", s, parsed, s)
		}
	}

	// Verify kebab-case alias parsing
	kebabCases := map[string]stability.TrialState{
		"awaiting-defect-assessment":     stability.TrialAwaitingDefectAssessment,
		"awaiting-remediation-approval": stability.TrialAwaitingRemediationApproval,
		"fix-dispatched":                stability.TrialFixDispatched,
		"crash-injected":                stability.TrialCrashInjected,
		"lease-wait":                    stability.TrialLeaseWait,
		"evidence-captured":             stability.TrialEvidenceCaptured,
		"cleaned-up":                    stability.TrialCleanedUp,
	}
	for input, want := range kebabCases {
		got, err := stability.ParseTrialState(input)
		if err != nil {
			t.Errorf("ParseTrialState(%q) error: %v", input, err)
		}
		if got != want {
			t.Errorf("ParseTrialState(%q) = %q, want %q", input, got, want)
		}
	}

	// Unrecognized state strings must be rejected
	invalidStates := []string{"", "unknown", "running", "retry", "pending", "awaiting_approval"}
	for _, inv := range invalidStates {
		if _, err := stability.ParseTrialState(inv); err == nil {
			t.Errorf("ParseTrialState(%q) returned nil error, want error", inv)
		}
		if stability.TrialState(inv).Valid() {
			t.Errorf("TrialState(%q).Valid() = true, want false", inv)
		}
	}

	// Terminal check
	if !stability.TrialPassed.Terminal() {
		t.Errorf("TrialPassed.Terminal() = false, want true")
	}
	if !stability.TrialFailed.Terminal() {
		t.Errorf("TrialFailed.Terminal() = false, want true")
	}
	if stability.TrialAdmitted.Terminal() {
		t.Errorf("TrialAdmitted.Terminal() = true, want false")
	}
	if stability.TrialCleanedUp.Terminal() {
		t.Errorf("TrialCleanedUp.Terminal() = true, want false")
	}
}

func TestCampaignTrialTransitions(t *testing.T) {
	// Happy path transitions sequence
	happyPath := []struct {
		from stability.TrialState
		to   stability.TrialState
	}{
		{stability.TrialAdmitted, stability.TrialDispatching},
		{stability.TrialDispatching, stability.TrialAwaitingDefectAssessment},
		{stability.TrialAwaitingDefectAssessment, stability.TrialAwaitingRemediationApproval},
		{stability.TrialAwaitingRemediationApproval, stability.TrialFixDispatched},
		{stability.TrialFixDispatched, stability.TrialCrashInjected},
		{stability.TrialCrashInjected, stability.TrialLeaseWait},
		{stability.TrialLeaseWait, stability.TrialReclaimed},
		{stability.TrialReclaimed, stability.TrialPromoted},
		{stability.TrialPromoted, stability.TrialEvidenceCaptured},
		{stability.TrialEvidenceCaptured, stability.TrialCleanedUp},
		{stability.TrialCleanedUp, stability.TrialPassed},
	}

	for _, step := range happyPath {
		if err := stability.ValidateTrialTransition(step.from, step.to); err != nil {
			t.Errorf("ValidateTrialTransition(%q, %q) unexpected error: %v", step.from, step.to, err)
		}
	}

	// Every non-terminal state may transition to TrialFailed
	nonTerminalStates := []stability.TrialState{
		stability.TrialAdmitted,
		stability.TrialDispatching,
		stability.TrialAwaitingDefectAssessment,
		stability.TrialAwaitingRemediationApproval,
		stability.TrialFixDispatched,
		stability.TrialCrashInjected,
		stability.TrialLeaseWait,
		stability.TrialReclaimed,
		stability.TrialPromoted,
		stability.TrialEvidenceCaptured,
		stability.TrialCleanedUp,
	}
	for _, s := range nonTerminalStates {
		if err := stability.ValidateTrialTransition(s, stability.TrialFailed); err != nil {
			t.Errorf("ValidateTrialTransition(%q, TrialFailed) unexpected error: %v", s, err)
		}
	}

	// Illegal transitions must be rejected
	illegalTransitions := []struct {
		from stability.TrialState
		to   stability.TrialState
		desc string
	}{
		{stability.TrialAdmitted, stability.TrialPromoted, "jump from admitted to promoted"},
		{stability.TrialAdmitted, stability.TrialPassed, "jump from admitted to passed"},
		{stability.TrialPromoted, stability.TrialDispatching, "backwards transition"},
		{stability.TrialCleanedUp, stability.TrialAdmitted, "backwards to admitted"},
		{stability.TrialPassed, stability.TrialAdmitted, "transition from terminal passed"},
		{stability.TrialPassed, stability.TrialFailed, "transition passed to failed"},
		{stability.TrialFailed, stability.TrialPassed, "transition failed to passed"},
		{stability.TrialFailed, stability.TrialDispatching, "retry from failed to dispatching"},
		{stability.TrialFailed, stability.TrialAdmitted, "retry from failed to admitted"},
	}

	for _, tt := range illegalTransitions {
		err := stability.ValidateTrialTransition(tt.from, tt.to)
		if err == nil {
			t.Errorf("ValidateTrialTransition(%q, %q) [%s] = nil error, want error", tt.from, tt.to, tt.desc)
		} else if !errors.Is(err, stability.ErrInvalidTrialTransition) {
			t.Errorf("ValidateTrialTransition(%q, %q) [%s] error = %v, want %v", tt.from, tt.to, tt.desc, err, stability.ErrInvalidTrialTransition)
		}
	}

	// Invalid states must return ErrInvalidTrialState
	if err := stability.ValidateTrialTransition(stability.TrialState("invalid"), stability.TrialDispatching); !errors.Is(err, stability.ErrInvalidTrialState) {
		t.Errorf("ValidateTrialTransition(invalid, to) error = %v, want ErrInvalidTrialState", err)
	}
	if err := stability.ValidateTrialTransition(stability.TrialAdmitted, stability.TrialState("invalid")); !errors.Is(err, stability.ErrInvalidTrialState) {
		t.Errorf("ValidateTrialTransition(from, invalid) error = %v, want ErrInvalidTrialState", err)
	}
}

func TestCampaignTransitions(t *testing.T) {
	legalTransitions := []struct {
		from stability.CampaignState
		to   stability.CampaignState
	}{
		{stability.CampaignPreflight, stability.CampaignRunning},
		{stability.CampaignPreflight, stability.CampaignFailed},
		{stability.CampaignRunning, stability.CampaignPassed},
		{stability.CampaignRunning, stability.CampaignFailed},
		{stability.CampaignRunning, stability.CampaignBlockedCleanup},
		{stability.CampaignBlockedCleanup, stability.CampaignFailed},
	}

	for _, tt := range legalTransitions {
		if err := stability.ValidateCampaignTransition(tt.from, tt.to); err != nil {
			t.Errorf("ValidateCampaignTransition(%q, %q) unexpected error: %v", tt.from, tt.to, err)
		}
	}

	illegalTransitions := []struct {
		from stability.CampaignState
		to   stability.CampaignState
		desc string
	}{
		{stability.CampaignPreflight, stability.CampaignPassed, "preflight straight to passed"},
		{stability.CampaignPreflight, stability.CampaignBlockedCleanup, "preflight to blocked_cleanup"},
		{stability.CampaignPassed, stability.CampaignRunning, "passed back to running"},
		{stability.CampaignPassed, stability.CampaignFailed, "passed to failed"},
		{stability.CampaignFailed, stability.CampaignRunning, "failed to running (retry)"},
		{stability.CampaignFailed, stability.CampaignPassed, "failed to passed"},
		{stability.CampaignBlockedCleanup, stability.CampaignRunning, "blocked_cleanup to running"},
		{stability.CampaignBlockedCleanup, stability.CampaignPassed, "blocked_cleanup to passed"},
	}

	for _, tt := range illegalTransitions {
		err := stability.ValidateCampaignTransition(tt.from, tt.to)
		if err == nil {
			t.Errorf("ValidateCampaignTransition(%q, %q) [%s] = nil error, want error", tt.from, tt.to, tt.desc)
		} else if !errors.Is(err, stability.ErrInvalidCampaignTransition) {
			t.Errorf("ValidateCampaignTransition(%q, %q) [%s] error = %v, want %v", tt.from, tt.to, tt.desc, err, stability.ErrInvalidCampaignTransition)
		}
	}

	// Invalid states must return ErrInvalidCampaignState
	if err := stability.ValidateCampaignTransition(stability.CampaignState("invalid"), stability.CampaignRunning); !errors.Is(err, stability.ErrInvalidCampaignState) {
		t.Errorf("ValidateCampaignTransition(invalid, to) error = %v, want ErrInvalidCampaignState", err)
	}
	if err := stability.ValidateCampaignTransition(stability.CampaignPreflight, stability.CampaignState("invalid")); !errors.Is(err, stability.ErrInvalidCampaignState) {
		t.Errorf("ValidateCampaignTransition(from, invalid) error = %v, want ErrInvalidCampaignState", err)
	}
}

func TestCampaignZeroRetryGuarantee(t *testing.T) {
	// A failed trial cannot transition to any state (no automatic retry or restart)
	retryStates := []stability.TrialState{
		stability.TrialAdmitted,
		stability.TrialDispatching,
		stability.TrialAwaitingDefectAssessment,
		stability.TrialAwaitingRemediationApproval,
		stability.TrialFixDispatched,
		stability.TrialCrashInjected,
		stability.TrialLeaseWait,
		stability.TrialReclaimed,
		stability.TrialPromoted,
		stability.TrialEvidenceCaptured,
		stability.TrialCleanedUp,
		stability.TrialPassed,
		stability.TrialFailed,
	}

	for _, target := range retryStates {
		err := stability.ValidateTrialTransition(stability.TrialFailed, target)
		if err == nil {
			t.Errorf("ValidateTrialTransition(TrialFailed, %q) = nil, want rejection of retry", target)
		}
		if !errors.Is(err, stability.ErrInvalidTrialTransition) {
			t.Errorf("ValidateTrialTransition(TrialFailed, %q) error = %v, want %v", target, err, stability.ErrInvalidTrialTransition)
		}
	}

	// In state machine, attempting to retry a trial slot after failure is rejected
	sm := stability.NewStateMachine()
	if err := sm.Start(); err != nil {
		t.Fatalf("sm.Start() failed: %v", err)
	}

	// Start Trial 1
	trialNum, err := sm.StartNextTrial()
	if err != nil {
		t.Fatalf("StartNextTrial failed: %v", err)
	}
	if trialNum != 1 {
		t.Fatalf("trialNum = %d, want 1", trialNum)
	}

	// Fail Trial 1
	if err := sm.RecordTrialOutcome(stability.TrialFailed); err != nil {
		t.Fatalf("RecordTrialOutcome(TrialFailed) failed: %v", err)
	}

	// Campaign must be failed and consecutive passes must be 0
	if sm.State() != stability.CampaignFailed {
		t.Errorf("sm.State() = %q, want %q", sm.State(), stability.CampaignFailed)
	}
	if sm.ConsecutivePasses() != 0 {
		t.Errorf("sm.ConsecutivePasses() = %d, want 0", sm.ConsecutivePasses())
	}

	// Attempting to restart or retry after failure must fail
	if _, err := sm.StartNextTrial(); err == nil {
		t.Error("sm.StartNextTrial() after failure returned nil error, want error rejecting retry")
	}
}

func TestCampaignSequentialThreeTrialProgression(t *testing.T) {
	t.Run("three consecutive passes achieve passed campaign", func(t *testing.T) {
		sm := stability.NewStateMachine()
		if sm.State() != stability.CampaignPreflight {
			t.Fatalf("initial state = %q, want %q", sm.State(), stability.CampaignPreflight)
		}
		if sm.ConsecutivePasses() != 0 {
			t.Fatalf("initial consecutive passes = %d, want 0", sm.ConsecutivePasses())
		}

		if err := sm.Start(); err != nil {
			t.Fatalf("sm.Start() failed: %v", err)
		}
		if sm.State() != stability.CampaignRunning {
			t.Fatalf("state after start = %q, want %q", sm.State(), stability.CampaignRunning)
		}

		// Trial 1
		t1, err := sm.StartNextTrial()
		if err != nil {
			t.Fatalf("StartNextTrial 1 failed: %v", err)
		}
		if t1 != 1 {
			t.Fatalf("t1 = %d, want 1", t1)
		}
		runFullPassingTrial(t, sm)
		if sm.ConsecutivePasses() != 1 {
			t.Errorf("consecutive passes after T1 = %d, want 1", sm.ConsecutivePasses())
		}
		if sm.State() != stability.CampaignRunning {
			t.Errorf("state after T1 = %q, want %q", sm.State(), stability.CampaignRunning)
		}
		if sm.IsEligibleForTerminalVerification() {
			t.Errorf("eligible for terminal verification after 1 pass = true, want false")
		}

		// Trial 2
		t2, err := sm.StartNextTrial()
		if err != nil {
			t.Fatalf("StartNextTrial 2 failed: %v", err)
		}
		if t2 != 2 {
			t.Fatalf("t2 = %d, want 2", t2)
		}
		runFullPassingTrial(t, sm)
		if sm.ConsecutivePasses() != 2 {
			t.Errorf("consecutive passes after T2 = %d, want 2", sm.ConsecutivePasses())
		}
		if sm.State() != stability.CampaignRunning {
			t.Errorf("state after T2 = %q, want %q", sm.State(), stability.CampaignRunning)
		}
		if sm.IsEligibleForTerminalVerification() {
			t.Errorf("eligible for terminal verification after 2 passes = true, want false")
		}

		// Trial 3
		t3, err := sm.StartNextTrial()
		if err != nil {
			t.Fatalf("StartNextTrial 3 failed: %v", err)
		}
		if t3 != 3 {
			t.Fatalf("t3 = %d, want 3", t3)
		}
		runFullPassingTrial(t, sm)
		if sm.ConsecutivePasses() != 3 {
			t.Errorf("consecutive passes after T3 = %d, want 3", sm.ConsecutivePasses())
		}
		if !sm.IsEligibleForTerminalVerification() {
			t.Errorf("eligible for terminal verification after 3 passes = false, want true")
		}
		if sm.State() != stability.CampaignPassed {
			t.Errorf("state after 3 passes = %q, want %q", sm.State(), stability.CampaignPassed)
		}

		// Cannot start trial after campaign passed
		if _, err := sm.StartNextTrial(); err == nil {
			t.Error("StartNextTrial() after campaign passed returned nil error, want error")
		}
	})

	t.Run("pass pass fail resets consecutive counter to zero and fails campaign", func(t *testing.T) {
		sm := stability.NewStateMachine()
		if err := sm.Start(); err != nil {
			t.Fatalf("sm.Start() failed: %v", err)
		}

		// Trial 1: pass
		if _, err := sm.StartNextTrial(); err != nil {
			t.Fatalf("StartNextTrial 1 failed: %v", err)
		}
		runFullPassingTrial(t, sm)
		if sm.ConsecutivePasses() != 1 {
			t.Fatalf("passes = %d, want 1", sm.ConsecutivePasses())
		}

		// Trial 2: pass
		if _, err := sm.StartNextTrial(); err != nil {
			t.Fatalf("StartNextTrial 2 failed: %v", err)
		}
		runFullPassingTrial(t, sm)
		if sm.ConsecutivePasses() != 2 {
			t.Fatalf("passes = %d, want 2", sm.ConsecutivePasses())
		}

		// Trial 3: fail mid-journey (e.g. at crash_injected)
		if _, err := sm.StartNextTrial(); err != nil {
			t.Fatalf("StartNextTrial 3 failed: %v", err)
		}
		if err := sm.AdvanceTrial(stability.TrialDispatching); err != nil {
			t.Fatalf("AdvanceTrial dispatching: %v", err)
		}
		if err := sm.AdvanceTrial(stability.TrialAwaitingDefectAssessment); err != nil {
			t.Fatalf("AdvanceTrial defect: %v", err)
		}
		if err := sm.RecordTrialOutcome(stability.TrialFailed); err != nil {
			t.Fatalf("RecordTrialOutcome 3: %v", err)
		}

		// MUST reset to 0, NOT stay at 2
		if got := sm.ConsecutivePasses(); got != 0 {
			t.Fatalf("ConsecutivePasses after fail = %d, WANT 0 (must reset to zero immediately)", got)
		}
		if sm.State() != stability.CampaignFailed {
			t.Errorf("Campaign state = %q, want %q", sm.State(), stability.CampaignFailed)
		}
		if sm.IsEligibleForTerminalVerification() {
			t.Errorf("IsEligibleForTerminalVerification = true, want false")
		}

		// Campaign does NOT proceed to any further trial
		if _, err := sm.StartNextTrial(); err == nil {
			t.Error("StartNextTrial after campaign failure returned nil error, want error")
		}
	})

	t.Run("failure at Trial 2 resets count to 0 and stops campaign", func(t *testing.T) {
		sm := stability.NewStateMachine()
		if err := sm.Start(); err != nil {
			t.Fatalf("sm.Start() failed: %v", err)
		}

		// Trial 1: pass
		if _, err := sm.StartNextTrial(); err != nil {
			t.Fatalf("StartNextTrial 1 failed: %v", err)
		}
		runFullPassingTrial(t, sm)
		if sm.ConsecutivePasses() != 1 {
			t.Fatalf("passes = %d, want 1", sm.ConsecutivePasses())
		}

		// Trial 2: fail
		if _, err := sm.StartNextTrial(); err != nil {
			t.Fatalf("StartNextTrial 2 failed: %v", err)
		}
		if err := sm.RecordTrialOutcome(stability.TrialFailed); err != nil {
			t.Fatalf("RecordTrialOutcome 2: %v", err)
		}

		// Reset to 0 immediately
		if got := sm.ConsecutivePasses(); got != 0 {
			t.Fatalf("ConsecutivePasses after fail at T2 = %d, want 0", got)
		}
		// Must not proceed to Trial 3 attempt
		if _, err := sm.StartNextTrial(); err == nil {
			t.Error("StartNextTrial for Trial 3 succeeded after Trial 2 failure, want error")
		}
	})

	t.Run("state machine helper methods and transitions", func(t *testing.T) {
		sm := stability.NewStateMachine()
		if sm.IsComplete() {
			t.Errorf("sm.IsComplete() = true before start, want false")
		}
		if sm.HasActiveTrial() {
			t.Errorf("sm.HasActiveTrial() = true before start, want false")
		}
		if sm.CurrentTrial() != 0 {
			t.Errorf("sm.CurrentTrial() = %d, want 0", sm.CurrentTrial())
		}
		if sm.ActiveTrialState() != "" {
			t.Errorf("sm.ActiveTrialState() = %q, want empty", sm.ActiveTrialState())
		}

		// Cannot start trial before campaign is running
		if _, err := sm.StartNextTrial(); err == nil {
			t.Error("StartNextTrial before start returned nil error, want error")
		}

		// Cannot advance trial when no trial is active
		if err := sm.AdvanceTrial(stability.TrialDispatching); err == nil {
			t.Error("AdvanceTrial with no active trial returned nil error, want error")
		}

		// Cannot record trial outcome when no trial is active
		if err := sm.RecordTrialOutcome(stability.TrialPassed); err == nil {
			t.Error("RecordTrialOutcome with no active trial returned nil error, want error")
		}

		// Start campaign
		if err := sm.Start(); err != nil {
			t.Fatalf("sm.Start() failed: %v", err)
		}

		// Start Trial 1
		t1, err := sm.StartNextTrial()
		if err != nil {
			t.Fatalf("StartNextTrial failed: %v", err)
		}
		if t1 != 1 || sm.CurrentTrial() != 1 {
			t.Errorf("CurrentTrial = %d, want 1", sm.CurrentTrial())
		}
		if !sm.HasActiveTrial() {
			t.Errorf("HasActiveTrial = false, want true")
		}
		if sm.ActiveTrialState() != stability.TrialAdmitted {
			t.Errorf("ActiveTrialState = %q, want %q", sm.ActiveTrialState(), stability.TrialAdmitted)
		}

		// Cannot start another trial while Trial 1 is active
		if _, err := sm.StartNextTrial(); err == nil {
			t.Error("StartNextTrial while trial is active returned nil error, want error")
		}

		// Cannot record non-terminal state as trial outcome
		if err := sm.RecordTrialOutcome(stability.TrialDispatching); err == nil {
			t.Error("RecordTrialOutcome(TrialDispatching) returned nil error, want error")
		}

		// Cannot record TrialPassed from TrialAdmitted without going through stages
		if err := sm.RecordTrialOutcome(stability.TrialPassed); err == nil {
			t.Error("RecordTrialOutcome(TrialPassed) from TrialAdmitted returned nil error, want transition error")
		}

		// Advance through happy path
		if err := sm.AdvanceTrial(stability.TrialDispatching); err != nil {
			t.Fatalf("AdvanceTrial dispatching failed: %v", err)
		}
		if sm.ActiveTrialState() != stability.TrialDispatching {
			t.Errorf("ActiveTrialState = %q, want %q", sm.ActiveTrialState(), stability.TrialDispatching)
		}

		// Invalid advance transition rejected
		if err := sm.AdvanceTrial(stability.TrialCleanedUp); err == nil {
			t.Error("AdvanceTrial with illegal jump returned nil error, want error")
		}

		// Start() on running state machine rejected
		if err := sm.Start(); err == nil {
			t.Error("Start() on running state machine returned nil error, want error")
		}

		// Invalid campaign transition rejected
		if err := sm.TransitionCampaign(stability.CampaignState("invalid")); err == nil {
			t.Error("TransitionCampaign with invalid state returned nil error, want error")
		}

		// Direct transition of campaign to blocked_cleanup
		if err := sm.TransitionCampaign(stability.CampaignBlockedCleanup); err != nil {
			t.Fatalf("TransitionCampaign(CampaignBlockedCleanup) failed: %v", err)
		}
		if sm.State() != stability.CampaignBlockedCleanup {
			t.Errorf("State = %q, want %q", sm.State(), stability.CampaignBlockedCleanup)
		}
		if !sm.IsComplete() {
			t.Errorf("IsComplete() = false for blocked_cleanup, want true")
		}

		// From blocked_cleanup, transition to failed
		if err := sm.TransitionCampaign(stability.CampaignFailed); err != nil {
			t.Fatalf("TransitionCampaign(CampaignFailed) from blocked_cleanup failed: %v", err)
		}
		if sm.State() != stability.CampaignFailed {
			t.Errorf("State = %q, want %q", sm.State(), stability.CampaignFailed)
		}
		if !sm.IsComplete() {
			t.Errorf("IsComplete() = false for failed, want true")
		}
	})
}

func TestCampaignTimeoutBudgetsPure(t *testing.T) {
	// 10 minutes per dispatch, 45 minutes per Trial, 135 minutes per Campaign
	if stability.DispatchTimeout != 10*time.Minute {
		t.Errorf("DispatchTimeout = %v, want %v", stability.DispatchTimeout, 10*time.Minute)
	}
	if stability.TrialTimeout != 45*time.Minute {
		t.Errorf("TrialTimeout = %v, want %v", stability.TrialTimeout, 45*time.Minute)
	}
	if stability.CampaignTimeout != 135*time.Minute {
		t.Errorf("CampaignTimeout = %v, want %v", stability.CampaignTimeout, 135*time.Minute)
	}

	budgets := stability.DefaultBudgets
	if budgets.Dispatch != 10*time.Minute || budgets.Trial != 45*time.Minute || budgets.Campaign != 135*time.Minute {
		t.Errorf("DefaultBudgets mismatch: %+v", budgets)
	}

	tests := []struct {
		name         string
		limit        time.Duration
		elapsed      time.Duration
		wantExceeded bool
	}{
		// Dispatch budget edge cases (10m)
		{
			name:         "dispatch: zero elapsed",
			limit:        10 * time.Minute,
			elapsed:      0,
			wantExceeded: false,
		},
		{
			name:         "dispatch: one nanosecond under limit",
			limit:        10 * time.Minute,
			elapsed:      10*time.Minute - time.Nanosecond,
			wantExceeded: false,
		},
		{
			name:         "dispatch: exactly at limit",
			limit:        10 * time.Minute,
			elapsed:      10 * time.Minute,
			wantExceeded: false,
		},
		{
			name:         "dispatch: one nanosecond over limit",
			limit:        10 * time.Minute,
			elapsed:      10*time.Minute + time.Nanosecond,
			wantExceeded: true,
		},
		{
			name:         "dispatch: well over limit",
			limit:        10 * time.Minute,
			elapsed:      15 * time.Minute,
			wantExceeded: true,
		},

		// Trial budget edge cases (45m)
		{
			name:         "trial: zero elapsed",
			limit:        45 * time.Minute,
			elapsed:      0,
			wantExceeded: false,
		},
		{
			name:         "trial: one nanosecond under limit",
			limit:        45 * time.Minute,
			elapsed:      45*time.Minute - time.Nanosecond,
			wantExceeded: false,
		},
		{
			name:         "trial: exactly at limit",
			limit:        45 * time.Minute,
			elapsed:      45 * time.Minute,
			wantExceeded: false,
		},
		{
			name:         "trial: one nanosecond over limit",
			limit:        45 * time.Minute,
			elapsed:      45*time.Minute + time.Nanosecond,
			wantExceeded: true,
		},
		{
			name:         "trial: well over limit",
			limit:        45 * time.Minute,
			elapsed:      50 * time.Minute,
			wantExceeded: true,
		},

		// Campaign budget edge cases (135m)
		{
			name:         "campaign: zero elapsed",
			limit:        135 * time.Minute,
			elapsed:      0,
			wantExceeded: false,
		},
		{
			name:         "campaign: one nanosecond under limit",
			limit:        135 * time.Minute,
			elapsed:      135*time.Minute - time.Nanosecond,
			wantExceeded: false,
		},
		{
			name:         "campaign: exactly at limit",
			limit:        135 * time.Minute,
			elapsed:      135 * time.Minute,
			wantExceeded: false,
		},
		{
			name:         "campaign: one nanosecond over limit",
			limit:        135 * time.Minute,
			elapsed:      135*time.Minute + time.Nanosecond,
			wantExceeded: true,
		},
		{
			name:         "campaign: well over limit",
			limit:        135 * time.Minute,
			elapsed:      140 * time.Minute,
			wantExceeded: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stability.IsBudgetExceeded(tt.elapsed, tt.limit)
			if got != tt.wantExceeded {
				t.Errorf("IsBudgetExceeded(%v, %v) = %v, want %v", tt.elapsed, tt.limit, got, tt.wantExceeded)
			}
		})
	}

	// Verify CheckDispatchBudget, CheckTrialBudget, CheckCampaignBudget helpers
	if stability.CheckDispatchBudget(10*time.Minute, budgets.Dispatch) {
		t.Errorf("CheckDispatchBudget at limit reported true, want false")
	}
	if !stability.CheckDispatchBudget(10*time.Minute+time.Second, budgets.Dispatch) {
		t.Errorf("CheckDispatchBudget over limit reported false, want true")
	}

	if stability.CheckTrialBudget(45*time.Minute, budgets.Trial) {
		t.Errorf("CheckTrialBudget at limit reported true, want false")
	}
	if !stability.CheckTrialBudget(45*time.Minute+time.Second, budgets.Trial) {
		t.Errorf("CheckTrialBudget over limit reported false, want true")
	}

	if stability.CheckCampaignBudget(135*time.Minute, budgets.Campaign) {
		t.Errorf("CheckCampaignBudget at limit reported true, want false")
	}
	if !stability.CheckCampaignBudget(135*time.Minute+time.Second, budgets.Campaign) {
		t.Errorf("CheckCampaignBudget over limit reported false, want true")
	}
}
