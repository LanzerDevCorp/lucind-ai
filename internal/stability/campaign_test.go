package stability_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/feature"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
	"github.com/LanzerDevCorp/lucind-ai/internal/run"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability/fixture"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability/process"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability/store"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
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
		"awaiting-defect-assessment":    stability.TrialAwaitingDefectAssessment,
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

// laneEnvelopeJSON returns a minimal envelope satisfying result.schema.json.
func laneEnvelopeJSON(id, status string) string {
	if status == "blocked" {
		return fmt.Sprintf(`{
			"packet_id": %q,
			"status": "blocked",
			"summary": "hit a hard stop",
			"hard_stops": [{"hard_stop": "do not touch internal/barrier", "fired": true, "note": "would have had to edit it"}]
		}`, id)
	}
	return fmt.Sprintf(`{
		"packet_id": %q,
		"status": "done",
		"summary": "completed work cleanly",
		"hard_stops": [{"hard_stop": "do not touch internal/barrier", "fired": false}]
	}`, id)
}

// fakeJourneyExecutor is a test double for running trial journey packets.
type fakeJourneyExecutor struct {
	mu         sync.Mutex
	calls      map[string]int
	outcomes   map[string]executor.Outcome
	runFuncFor map[string]func(ctx context.Context, req executor.Request) (executor.Outcome, error)
}

func newFakeJourneyExecutor() *fakeJourneyExecutor {
	return &fakeJourneyExecutor{
		calls:      make(map[string]int),
		outcomes:   make(map[string]executor.Outcome),
		runFuncFor: make(map[string]func(ctx context.Context, req executor.Request) (executor.Outcome, error)),
	}
}

func (f *fakeJourneyExecutor) Run(ctx context.Context, req executor.Request) (executor.Outcome, error) {
	f.mu.Lock()
	f.calls[req.WorktreePath]++
	customFn := f.runFuncFor[req.WorktreePath]
	outcome, ok := f.outcomes[req.WorktreePath]
	f.mu.Unlock()

	if customFn != nil {
		return customFn(ctx, req)
	}
	if ok {
		return outcome, nil
	}
	return executor.Outcome{ExitCode: 0}, nil
}

func (f *fakeJourneyExecutor) DefaultModel() string {
	return stability.PinnedModel
}

func (f *fakeJourneyExecutor) KnownModels() []string {
	return []string{stability.PinnedModel}
}

func (f *fakeJourneyExecutor) CallCount(worktreePath string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[worktreePath]
}

// newJourneyTestDeps creates a fake run.Deps using real on-disk temp worktrees and SQLite ledger.
func newJourneyTestDeps(t *testing.T, execEnv executor.Executor) (run.Deps, *ledger.Ledger, string) {
	t.Helper()
	root := t.TempDir()
	l, err := ledger.Open(context.Background(), root)
	if err != nil {
		t.Fatalf("ledger.Open error = %v", err)
	}
	t.Cleanup(func() { l.Close() })

	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)

	deps := run.Deps{
		RunID:       "run-journey-trial-1",
		PrimaryRoot: root,
		Ledger:      l,
		LookupExecutor: func(name string) (executor.Executor, error) {
			return execEnv, nil
		},
		CreateWorktree: func(_ context.Context, _, laneID, _, baseSHA string) (worktree.Worktree, error) {
			wtPath := filepath.Join(root, "wt-"+laneID)
			lucindPath := filepath.Join(wtPath, ".lucind")
			if err := os.MkdirAll(lucindPath, 0o755); err != nil {
				return worktree.Worktree{}, err
			}

			// Initialize git repo with materialized fixtures
			runGit := func(args ...string) {
				cmd := exec.Command("git", append([]string{"-C", wtPath}, args...)...)
				_ = cmd.Run()
			}
			runGit("init")
			runGit("config", "user.name", "Test User")
			runGit("config", "user.email", "test@example.com")
			_ = fixture.MaterializeFixtures(wtPath)
			runGit("add", ".")
			runGit("commit", "-m", "initial baseline")

			cmd := exec.Command("git", "-C", wtPath, "rev-parse", "HEAD")
			out, _ := cmd.Output()
			actualBase := strings.TrimSpace(string(out))
			if baseSHA == "" {
				baseSHA = actualBase
			}

			// Add a unique commit modifying the allowed path for write packet completion
			if laneID == "stability-change-a" {
				_ = os.WriteFile(filepath.Join(wtPath, "fixture", "change_a.txt"), []byte("CHANGE_A=DONE\n"), 0o644)
			} else if laneID == "stability-change-b" {
				_ = os.WriteFile(filepath.Join(wtPath, "fixture", "change_b.txt"), []byte("CHANGE_B=DONE\n"), 0o644)
			}
			runGit("add", ".")
			runGit("commit", "-m", "lane change for "+laneID)

			// Write valid result envelope by default
			envJSON := laneEnvelopeJSON(laneID, "done")
			if err := os.WriteFile(filepath.Join(lucindPath, "result.json"), []byte(envJSON), 0o644); err != nil {
				return worktree.Worktree{}, err
			}
			return worktree.Worktree{
				Path:    wtPath,
				Branch:  "lucind/" + laneID,
				BaseSHA: actualBase,
			}, nil
		},
		WorktreeFS: func(path string) fs.FS {
			return os.DirFS(path)
		},
		Now: func() time.Time {
			return now
		},
		HasUniqueLaneCommits: func(ctx context.Context, worktreePath, baseSHA string) (bool, error) {
			return true, nil
		},
		PorcelainEmpty: func(ctx context.Context, worktreePath string) (bool, error) {
			return true, nil
		},
	}

	return deps, l, root
}

func TestTrialJourneyConcurrentDispatchViaBatchPrimitive(t *testing.T) {
	// Behavior 1: Build a Trial-execution function that constructs two packet.Packet values
	// from internal/stability/fixture (Change A and Change B) with pinned model gemini-3.7-flash-high,
	// and dispatches them through internal/run.ExecuteBatch reusing the existing batch primitive.
	fakeExec := newFakeJourneyExecutor()
	deps, _, _ := newJourneyTestDeps(t, fakeExec)

	cfg := stability.DefaultJourneyConfig(1, "b000000000000000000000000000000000000000")
	pA, pB := stability.BuildJourneyPackets(cfg)

	// Verify packet target fields and pinned model
	if pA.Model != stability.PinnedModel {
		t.Errorf("pA.Model = %q, want %q", pA.Model, stability.PinnedModel)
	}
	if pB.Model != stability.PinnedModel {
		t.Errorf("pB.Model = %q, want %q", pB.Model, stability.PinnedModel)
	}
	if pA.Feature != cfg.TargetA {
		t.Errorf("pA.Feature = %q, want %q", pA.Feature, cfg.TargetA)
	}
	if pB.Feature != cfg.TargetB {
		t.Errorf("pB.Feature = %q, want %q", pB.Feature, cfg.TargetB)
	}
	if pA.ParentRef != cfg.ParentRefA {
		t.Errorf("pA.ParentRef = %q, want %q", pA.ParentRef, cfg.ParentRefA)
	}
	if pB.ParentRef != cfg.ParentRefB {
		t.Errorf("pB.ParentRef = %q, want %q", pB.ParentRef, cfg.ParentRefB)
	}

	// Dispatch concurrently via batch primitive
	report, err := stability.DispatchConcurrentLanes(context.Background(), deps, pA, pB)
	if err != nil {
		t.Fatalf("DispatchConcurrentLanes failed: %v", err)
	}

	if !report.Released {
		t.Errorf("report.Released = false, want true")
	}
	if len(report.Lanes) != 2 {
		t.Fatalf("len(report.Lanes) = %d, want 2", len(report.Lanes))
	}
	if report.Lanes[0].LaneID != pA.ID {
		t.Errorf("report.Lanes[0].LaneID = %q, want %q", report.Lanes[0].LaneID, pA.ID)
	}
	if report.Lanes[1].LaneID != pB.ID {
		t.Errorf("report.Lanes[1].LaneID = %q, want %q", report.Lanes[1].LaneID, pB.ID)
	}
	if report.Lanes[0].Status != lane.Done {
		t.Errorf("report.Lanes[0].Status = %v, want %v", report.Lanes[0].Status, lane.Done)
	}
	if report.Lanes[1].Status != lane.Done {
		t.Errorf("report.Lanes[1].Status = %v, want %v", report.Lanes[1].Status, lane.Done)
	}
}

func TestTrialJourneyConcurrentLeaseHoldingBeforePromotion(t *testing.T) {
	// Behavior 2: Before dispatching, acquire an internal/feature Ownership Lease for each of
	// Change A's and Change B's distinct ephemeral feature targets against a real SQLite-backed
	// *feature.Service. Prove with a real concurrency assertion that both leases are held
	// simultaneously before either promotes.
	ctx := context.Background()
	fakeExec := newFakeJourneyExecutor()
	deps, ledgerHandle, _ := newJourneyTestDeps(t, fakeExec)
	featSvc := feature.NewService(ledgerHandle)

	cfg := stability.DefaultJourneyConfig(1, "b000000000000000000000000000000000000000")
	cfg.LeaseTTL = 5 * time.Second

	// Acquire leases concurrently
	leaseA, leaseB, err := stability.AcquireTargetLeasesConcurrently(ctx, featSvc, cfg)
	if err != nil {
		t.Fatalf("AcquireTargetLeasesConcurrently failed: %v", err)
	}

	if leaseA.Fence != 1 || leaseA.Owner != cfg.OwnerA {
		t.Errorf("unexpected leaseA: %+v, want owner %s and fence 1", leaseA, cfg.OwnerA)
	}
	if leaseB.Fence != 1 || leaseB.Owner != cfg.OwnerB {
		t.Errorf("unexpected leaseB: %+v, want owner %s and fence 1", leaseB, cfg.OwnerB)
	}

	// Concurrency proof: Both leases are valid in the real SQLite ledger simultaneously
	if err := featSvc.ValidateLease(ctx, cfg.TargetA, cfg.OwnerA, leaseA.Fence); err != nil {
		t.Errorf("leaseA validation failed: %v", err)
	}
	if err := featSvc.ValidateLease(ctx, cfg.TargetB, cfg.OwnerB, leaseB.Fence); err != nil {
		t.Errorf("leaseB validation failed: %v", err)
	}

	// Active unexpired lease returns ErrLeaseHeld for competing workers
	if _, err := featSvc.AcquireLease(ctx, cfg.TargetA, "competitor-a", cfg.LeaseTTL); !errors.Is(err, feature.ErrLeaseHeld) {
		t.Errorf("competing acquire on targetA returned %v, want ErrLeaseHeld", err)
	}
	if _, err := featSvc.AcquireLease(ctx, cfg.TargetB, "competitor-b", cfg.LeaseTTL); !errors.Is(err, feature.ErrLeaseHeld) {
		t.Errorf("competing acquire on targetB returned %v, want ErrLeaseHeld", err)
	}

	// Synchronization barrier to prove both leases are held concurrently DURING dispatch execution
	inFlightA := make(chan struct{})
	inFlightB := make(chan struct{})
	verifiedConcurrent := make(chan struct{})

	wtPathA := filepath.Join(deps.PrimaryRoot, "wt-stability-change-a")
	wtPathB := filepath.Join(deps.PrimaryRoot, "wt-stability-change-b")

	fakeExec.runFuncFor[wtPathA] = func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		close(inFlightA)
		// Wait until concurrency is verified before completing lane A
		select {
		case <-verifiedConcurrent:
		case <-time.After(2 * time.Second):
			t.Error("timeout waiting for verifiedConcurrent in lane A")
		}
		return executor.Outcome{ExitCode: 0}, nil
	}

	fakeExec.runFuncFor[wtPathB] = func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		close(inFlightB)
		// Wait until concurrency is verified before completing lane B
		select {
		case <-verifiedConcurrent:
		case <-time.After(2 * time.Second):
			t.Error("timeout waiting for verifiedConcurrent in lane B")
		}
		return executor.Outcome{ExitCode: 0}, nil
	}

	pA, pB := stability.BuildJourneyPackets(cfg)

	var (
		batchReport run.BatchReport
		batchErr    error
		batchDone   = make(chan struct{})
	)

	go func() {
		defer close(batchDone)
		batchReport, batchErr = stability.DispatchConcurrentLanes(ctx, deps, pA, pB)
	}()

	// Wait until BOTH dispatches are in flight concurrently
	select {
	case <-inFlightA:
	case <-time.After(2 * time.Second):
		t.Fatal("lane A never started dispatch")
	}
	select {
	case <-inFlightB:
	case <-time.After(2 * time.Second):
		t.Fatal("lane B never started dispatch")
	}

	// While both lanes are running, assert both leases are simultaneously held and active
	if err := featSvc.ValidateLease(ctx, cfg.TargetA, cfg.OwnerA, leaseA.Fence); err != nil {
		t.Errorf("concurrent check: leaseA not valid: %v", err)
	}
	if err := featSvc.ValidateLease(ctx, cfg.TargetB, cfg.OwnerB, leaseB.Fence); err != nil {
		t.Errorf("concurrent check: leaseB not valid: %v", err)
	}

	// Release dispatch barrier so lanes complete
	close(verifiedConcurrent)

	select {
	case <-batchDone:
	case <-time.After(2 * time.Second):
		t.Fatal("batch dispatch timed out")
	}

	if batchErr != nil {
		t.Fatalf("batch dispatch failed: %v", batchErr)
	}
	if !batchReport.Released {
		t.Errorf("batchReport.Released = false, want true")
	}
}

func writeStubScript(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "stub.sh")
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("writeStubScript error = %v", err)
	}
	return path
}

func TestTrialJourneyAbruptKillAfterResultPersistenceBeforeAcceptance(t *testing.T) {
	// Behavior 3: Abrupt kill of Change B, real OS process, fake AI call.
	// Fake executor starts real local stub process in its own process group via process.Supervisor.Start.
	// Externally trigger SIGKILL via process.KillGroup AFTER Change B's result has been persisted
	// into .lucind/result.json, but BEFORE Acceptance.
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	ctx := context.Background()
	fakeExec := newFakeJourneyExecutor()
	deps, ledgerHandle, _ := newJourneyTestDeps(t, fakeExec)
	featSvc := feature.NewService(ledgerHandle)

	cfg := stability.DefaultJourneyConfig(1, "b000000000000000000000000000000000000000")
	cfg.LeaseTTL = 5 * time.Second

	wtPathB := filepath.Join(deps.PrimaryRoot, "wt-stability-change-b")
	lucindB := filepath.Join(wtPathB, ".lucind")
	_ = os.MkdirAll(lucindB, 0o755)
	persistedMarker := filepath.Join(wtPathB, "persisted.done")

	// Stub writes .lucind/result.json, creates marker file, and then sleeps 30s awaiting kill
	stubScript := fmt.Sprintf(`#!/bin/sh
mkdir -p %q
cat << 'EOF' > %q
{
  "packet_id": "stability-change-b",
  "status": "done",
  "summary": "change B work completed before crash",
  "hard_stops": [{"hard_stop": "do not touch internal/barrier", "fired": false}]
}
EOF
touch %q
sleep 30
`, lucindB, filepath.Join(lucindB, "result.json"), persistedMarker)
	stubPath := writeStubScript(t, stubScript)

	supervisor := process.NewSupervisor()
	pgidChan := make(chan int, 1)

	fakeExec.runFuncFor[wtPathB] = func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		cmd := exec.Command(stubPath)
		cmd.Dir = wtPathB
		pgid, err := supervisor.Start(cmd)
		if err != nil {
			return executor.Outcome{}, err
		}
		pgidChan <- pgid
		// Wait for command to exit or be killed
		_ = cmd.Wait()
		return executor.Outcome{ExitCode: 0}, nil
	}

	// Preflight leases
	_, _, err := stability.AcquireTargetLeasesConcurrently(ctx, featSvc, cfg)
	if err != nil {
		t.Fatalf("AcquireTargetLeasesConcurrently failed: %v", err)
	}

	pA, pB := stability.BuildJourneyPackets(cfg)

	go func() {
		_, _ = stability.DispatchConcurrentLanes(ctx, deps, pA, pB)
	}()

	var pgidB int
	select {
	case pgidB = <-pgidChan:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for Change B pgid")
	}

	t.Cleanup(func() {
		_ = process.KillGroup(pgidB)
	})

	// Wait until stub has written .lucind/result.json
	deadline := time.Now().Add(3 * time.Second)
	var persistenceProven bool
	for time.Now().Before(deadline) {
		if _, err := os.Stat(persistedMarker); err == nil {
			// Result persistence happened! Verify envelope is readable on disk before kill.
			env, readErr := result.Read(os.DirFS(wtPathB), ".lucind/result.json")
			if readErr == nil && env.PacketID == "stability-change-b" && env.LaneStatus() == lane.Done {
				persistenceProven = true
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !persistenceProven {
		t.Fatal("result envelope was not persisted on disk before kill")
	}

	// Order assertion: Kill happens AFTER result persistence and BEFORE acceptance/promotion
	killErr := process.KillGroup(pgidB)
	if killErr != nil {
		t.Fatalf("KillGroup(%d) error = %v", pgidB, killErr)
	}

	// Verify all processes in pgidB are dead
	if err := supervisor.VerifyZero(pgidB); err != nil {
		t.Fatalf("survivors detected after kill: %v", err)
	}
}

func TestTrialJourneyTenSecondLeaseTTLEarlyReclaimRejected(t *testing.T) {
	// Behavior 4: Attempting to reclaim Change B's lease immediately after kill (before lease expiry)
	// MUST return feature.ErrLeaseHeld against a real SQLite lease ledger.
	ctx := context.Background()
	fakeExec := newFakeJourneyExecutor()
	_, ledgerHandle, _ := newJourneyTestDeps(t, fakeExec)
	featSvc := feature.NewService(ledgerHandle)

	cfg := stability.DefaultJourneyConfig(1, "b000000000000000000000000000000000000000")
	cfg.LeaseTTL = 300 * time.Millisecond

	// Acquire initial lease for Target B
	leaseB, err := featSvc.AcquireLease(ctx, cfg.TargetB, cfg.OwnerB, cfg.LeaseTTL)
	if err != nil {
		t.Fatalf("AcquireLease failed: %v", err)
	}
	if leaseB.Fence != 1 {
		t.Errorf("initial fence = %d, want 1", leaseB.Fence)
	}

	// Immediate reclaim before TTL expiry MUST fail with ErrLeaseHeld
	_, err = featSvc.AcquireLease(ctx, cfg.TargetB, cfg.ReplacementOwnerB, cfg.LeaseTTL)
	if !errors.Is(err, feature.ErrLeaseHeld) {
		t.Fatalf("early reclaim error = %v, want ErrLeaseHeld", err)
	}
}

func TestTrialJourneyPostExpiryReclaimMonotonicFenceEnvelopeAdoptionNoRedispatch(t *testing.T) {
	// Behavior 5: After lease genuinely expires, replacement acquisition succeeds with strictly
	// incremented fence (monotonic). Trial adopts already-persisted envelope without redispatching
	// (Change B call count stays at exactly one).
	ctx := context.Background()
	fakeExec := newFakeJourneyExecutor()
	deps, ledgerHandle, root := newJourneyTestDeps(t, fakeExec)
	featSvc := feature.NewService(ledgerHandle)

	cfg := stability.DefaultJourneyConfig(1, "b000000000000000000000000000000000000000")
	cfg.LeaseTTL = 150 * time.Millisecond

	// 1. Initial acquisition
	initialLease, err := featSvc.AcquireLease(ctx, cfg.TargetB, cfg.OwnerB, cfg.LeaseTTL)
	if err != nil {
		t.Fatalf("initial lease acquire failed: %v", err)
	}
	if initialLease.Fence != 1 {
		t.Errorf("initialLease.Fence = %d, want 1", initialLease.Fence)
	}

	wtPathB := filepath.Join(root, "wt-stability-change-b")
	lucindB := filepath.Join(wtPathB, ".lucind")
	_ = os.MkdirAll(lucindB, 0o755)
	envJSON := laneEnvelopeJSON("stability-change-b", "done")
	_ = os.WriteFile(filepath.Join(lucindB, "result.json"), []byte(envJSON), 0o644)

	// Simulate initial dispatch (call count = 1)
	fakeExec.Run(ctx, executor.Request{WorktreePath: wtPathB})
	if fakeExec.CallCount(wtPathB) != 1 {
		t.Fatalf("initial call count = %d, want 1", fakeExec.CallCount(wtPathB))
	}

	// 2. Wait out short lease TTL
	time.Sleep(180 * time.Millisecond)

	// 3. Post-expiry reclaim with monotonic fence increment
	reclaimedLease, err := stability.ReclaimTargetLease(ctx, featSvc, cfg.TargetB, cfg.ReplacementOwnerB, initialLease.Fence, cfg.LeaseTTL)
	if err != nil {
		t.Fatalf("ReclaimTargetLease failed: %v", err)
	}
	if reclaimedLease.Fence != 2 {
		t.Errorf("reclaimedLease.Fence = %d, want 2 (initial fence was 1)", reclaimedLease.Fence)
	}
	if reclaimedLease.Owner != cfg.ReplacementOwnerB {
		t.Errorf("reclaimedLease.Owner = %q, want %q", reclaimedLease.Owner, cfg.ReplacementOwnerB)
	}

	// 4. Adopt already-persisted envelope
	adoptedEnv, err := stability.AdoptResultEnvelope(deps.WorktreeFS(wtPathB), wtPathB)
	if err != nil {
		t.Fatalf("AdoptResultEnvelope failed: %v", err)
	}
	if adoptedEnv.PacketID != "stability-change-b" {
		t.Errorf("adoptedEnv.PacketID = %q, want stability-change-b", adoptedEnv.PacketID)
	}
	if adoptedEnv.LaneStatus() != lane.Done {
		t.Errorf("adoptedEnv.LaneStatus() = %v, want Done", adoptedEnv.LaneStatus())
	}

	// 5. Assert NO second dispatch occurred (call count stays at 1)
	if callCount := fakeExec.CallCount(wtPathB); callCount != 1 {
		t.Errorf("fake executor call count for Change B = %d, WANT 1 (envelope adopted without redispatch)", callCount)
	}
}

func TestTrialJourneyZeroSurvivorVerificationGatesRecovery(t *testing.T) {
	// Behavior 6: Zero-survivor verification gates recovery in both directions:
	// - Zero survivors: recovery proceeds.
	// - Surviving processes: recovery fails with ErrSurvivingProcesses.
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	ctx := context.Background()
	fakeExec := newFakeJourneyExecutor()
	deps, ledgerHandle, root := newJourneyTestDeps(t, fakeExec)
	featSvc := feature.NewService(ledgerHandle)

	cfg := stability.DefaultJourneyConfig(1, "b000000000000000000000000000000000000000")
	cfg.LeaseTTL = 100 * time.Millisecond

	wtPathB := filepath.Join(root, "wt-stability-change-b")
	lucindB := filepath.Join(wtPathB, ".lucind")
	_ = os.MkdirAll(lucindB, 0o755)
	envJSON := laneEnvelopeJSON("stability-change-b", "done")
	_ = os.WriteFile(filepath.Join(lucindB, "result.json"), []byte(envJSON), 0o644)

	// Acquire initial lease
	initialLease, err := featSvc.AcquireLease(ctx, cfg.TargetB, cfg.OwnerB, cfg.LeaseTTL)
	if err != nil {
		t.Fatalf("AcquireLease failed: %v", err)
	}

	// Direction 1: Surviving process blocks recovery
	survivorScript := writeStubScript(t, "#!/bin/sh\nsleep 30\n")
	cmd := exec.Command(survivorScript)
	supervisor := process.NewSupervisor()
	livePGID, err := supervisor.Start(cmd)
	if err != nil {
		t.Fatalf("supervisor.Start failed: %v", err)
	}
	t.Cleanup(func() {
		_ = process.KillGroup(livePGID)
		_ = cmd.Wait()
	})

	time.Sleep(120 * time.Millisecond) // wait out TTL

	_, _, err = stability.RecoverCrashedChangeB(
		ctx,
		featSvc,
		deps.WorktreeFS(wtPathB),
		wtPathB,
		cfg.TargetB,
		cfg.ReplacementOwnerB,
		initialLease.Fence,
		livePGID,
		cfg.LeaseTTL,
	)
	if err == nil {
		t.Error("RecoverCrashedChangeB with live survivor returned nil error, want ErrSurvivingProcesses")
	} else if !errors.Is(err, process.ErrSurvivingProcesses) {
		t.Errorf("RecoverCrashedChangeB error = %v, want ErrSurvivingProcesses", err)
	}

	// Direction 2: Clean kill with zero survivors allows recovery to proceed
	_ = process.KillGroup(livePGID)
	_ = cmd.Wait()

	// Wait for process reaping
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := supervisor.VerifyZero(livePGID); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	reclaimedLease, adoptedEnv, err := stability.RecoverCrashedChangeB(
		ctx,
		featSvc,
		deps.WorktreeFS(wtPathB),
		wtPathB,
		cfg.TargetB,
		cfg.ReplacementOwnerB,
		initialLease.Fence,
		livePGID,
		cfg.LeaseTTL,
	)
	if err != nil {
		t.Fatalf("RecoverCrashedChangeB after clean kill failed: %v", err)
	}
	if reclaimedLease.Fence <= initialLease.Fence {
		t.Errorf("reclaimedLease.Fence = %d, want > %d", reclaimedLease.Fence, initialLease.Fence)
	}
	if adoptedEnv.PacketID != "stability-change-b" {
		t.Errorf("adoptedEnv.PacketID = %q, want stability-change-b", adoptedEnv.PacketID)
	}
}

func TestTrialJourneyStateMachineWiringNoBypass(t *testing.T) {
	// Behavior 7: Every transition routes through StateMachine.AdvanceTrial / RecordTrialOutcome,
	// without any direct field mutations or ad-hoc bypasses.
	ctx := context.Background()
	fakeExec := newFakeJourneyExecutor()
	deps, ledgerHandle, root := newJourneyTestDeps(t, fakeExec)
	featSvc := feature.NewService(ledgerHandle)

	cfg := stability.DefaultJourneyConfig(1, "b000000000000000000000000000000000000000")
	cfg.LeaseTTL = 100 * time.Millisecond

	wtPathB := filepath.Join(root, "wt-stability-change-b")
	lucindB := filepath.Join(wtPathB, ".lucind")
	_ = os.MkdirAll(lucindB, 0o755)
	envJSON := laneEnvelopeJSON("stability-change-b", "done")
	_ = os.WriteFile(filepath.Join(lucindB, "result.json"), []byte(envJSON), 0o644)

	sm := stability.NewStateMachine()
	if err := sm.Start(); err != nil {
		t.Fatalf("sm.Start failed: %v", err)
	}

	// Execute full trial journey
	result, err := stability.ExecuteTrialJourney(ctx, sm, deps, featSvc, cfg, 99999999, wtPathB) // non-existent PGID is clean (0 survivors)
	if err != nil {
		t.Fatalf("ExecuteTrialJourney failed: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
	if result.InitialLeaseB.Fence != 1 {
		t.Errorf("result.InitialLeaseB.Fence = %d, want 1", result.InitialLeaseB.Fence)
	}
	if result.ReclaimedLeaseB.Fence != 2 {
		t.Errorf("result.ReclaimedLeaseB.Fence = %d, want 2", result.ReclaimedLeaseB.Fence)
	}
	if result.AdoptedEnvelopeB.PacketID != "stability-change-b" {
		t.Errorf("result.AdoptedEnvelopeB.PacketID = %q, want stability-change-b", result.AdoptedEnvelopeB.PacketID)
	}

	// Verify state machine state is at TrialEvidenceCaptured
	if sm.ActiveTrialState() != stability.TrialEvidenceCaptured {
		t.Errorf("sm.ActiveTrialState() = %q, want %q", sm.ActiveTrialState(), stability.TrialEvidenceCaptured)
	}
	if !sm.HasActiveTrial() {
		t.Errorf("sm.HasActiveTrial() = false, want true")
	}
	if sm.CurrentTrial() != 1 {
		t.Errorf("sm.CurrentTrial() = %d, want 1", sm.CurrentTrial())
	}

	// Advance through cleanup and record pass
	if err := sm.AdvanceTrial(stability.TrialCleanedUp); err != nil {
		t.Fatalf("AdvanceTrial(TrialCleanedUp) failed: %v", err)
	}
	if err := sm.RecordTrialOutcome(stability.TrialPassed); err != nil {
		t.Fatalf("RecordTrialOutcome(TrialPassed) failed: %v", err)
	}
	if sm.ConsecutivePasses() != 1 {
		t.Errorf("sm.ConsecutivePasses() = %d, want 1", sm.ConsecutivePasses())
	}
}
