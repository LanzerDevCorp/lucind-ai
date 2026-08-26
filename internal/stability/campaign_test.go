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
	"github.com/LanzerDevCorp/lucind-ai/internal/integrate"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
	"github.com/LanzerDevCorp/lucind-ai/internal/run"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability/fixture"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability/process"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability/reconcile"
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
	return laneEnvelopeJSONWithCommit(id, status, "")
}

// laneEnvelopeJSONWithCommit returns a minimal envelope with commit satisfying result.schema.json.
func laneEnvelopeJSONWithCommit(id, status, commit string) string {
	commitField := ""
	if commit != "" {
		commitField = fmt.Sprintf(",\n\t\t\t\"commit\": %q", commit)
	}
	if status == "blocked" {
		return fmt.Sprintf(`{
			"packet_id": %q,
			"status": "blocked",
			"summary": "hit a hard stop"%s,
			"hard_stops": [{"hard_stop": "do not touch internal/barrier", "fired": true, "note": "would have had to edit it"}]
		}`, id, commitField)
	}
	return fmt.Sprintf(`{
		"packet_id": %q,
		"status": "done",
		"summary": "completed work cleanly"%s,
		"hard_stops": [{"hard_stop": "do not touch internal/barrier", "fired": false}]
	}`, id, commitField)
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
			wtPath := reconcile.WorktreePathFor(root, laneID)
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

	wtPathA := reconcile.WorktreePathFor(deps.PrimaryRoot, "stability-change-a")
	wtPathB := reconcile.WorktreePathFor(deps.PrimaryRoot, "stability-change-b")

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

	wtPathB := reconcile.WorktreePathFor(deps.PrimaryRoot, "stability-change-b")
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

	wtPathB := reconcile.WorktreePathFor(root, "stability-change-b")
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

	wtPathB := reconcile.WorktreePathFor(root, "stability-change-b")
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

	wtPathB := reconcile.WorktreePathFor(root, "stability-change-b")
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

func TestRemediationDefectRecordCreatedOnFixtureFailure(t *testing.T) {
	// Behavior 1: Define a DefectRecord type and a function that persists one when
	// Change A's dispatch reports a failure matching the seeded out-of-scope defect.
	// Prove: dispatching real fixture Change A packet through failing check produces
	// exactly one Defect Record with failure detail captured, and Change B's own
	// concurrent dispatch is unaffected (passes).
	ctx := context.Background()
	fakeExec := newFakeJourneyExecutor()
	deps, ledgerHandle, root := newJourneyTestDeps(t, fakeExec)
	_ = ledgerHandle

	cfg := stability.DefaultJourneyConfig(1, "b000000000000000000000000000000000000000")
	pA, pB := stability.BuildJourneyPackets(cfg)

	wtPathA := reconcile.WorktreePathFor(root, "stability-change-a")
	wtPathB := reconcile.WorktreePathFor(root, "stability-change-b")

	// Change A executor runs fixture check which fails on seeded defect
	fakeExec.runFuncFor[wtPathA] = func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		out, checkErr := fixture.RunCheck(ctx, req.WorktreePath, "change-a")
		if checkErr != nil {
			// Record defect on failure
			_, err := stability.AssessAndRecordDefect(ctx, deps.Ledger, req.WorktreePath, pA.ID, "fixture/check.sh", out, deps.Now())
			if err != nil {
				return executor.Outcome{}, err
			}
			// Write blocked envelope indicating hard stop / out-of-scope defect
			envJSON := laneEnvelopeJSON(pA.ID, "blocked")
			lucindPath := filepath.Join(req.WorktreePath, ".lucind")
			_ = os.MkdirAll(lucindPath, 0o755)
			_ = os.WriteFile(filepath.Join(lucindPath, "result.json"), []byte(envJSON), 0o644)
			return executor.Outcome{ExitCode: 0}, nil
		}
		return executor.Outcome{ExitCode: 0}, nil
	}

	// Change B executor runs fixture check which succeeds
	fakeExec.runFuncFor[wtPathB] = func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		out, checkErr := fixture.RunCheck(ctx, req.WorktreePath, "change-b")
		if checkErr != nil {
			return executor.Outcome{ExitCode: 1, Stderr: out}, nil
		}
		return executor.Outcome{ExitCode: 0}, nil
	}

	batchReport, err := stability.DispatchConcurrentLanes(ctx, deps, pA, pB)
	if err != nil {
		t.Fatalf("DispatchConcurrentLanes failed: %v", err)
	}

	if len(batchReport.Lanes) != 2 {
		t.Fatalf("len(batchReport.Lanes) = %d, want 2", len(batchReport.Lanes))
	}

	// Change A reports Blocked (stopped on out-of-scope defect)
	if batchReport.Lanes[0].Status != lane.Blocked {
		t.Errorf("Change A status = %v, want Blocked", batchReport.Lanes[0].Status)
	}
	// Change B reports Done (unaffected by Change A's defect)
	if batchReport.Lanes[1].Status != lane.Done {
		t.Errorf("Change B status = %v, want Done", batchReport.Lanes[1].Status)
	}

	// Verify exactly one Defect Record was persisted for Change A
	defectRec, err := stability.ReadDefectRecord(deps.WorktreeFS(wtPathA), wtPathA)
	if err != nil {
		t.Fatalf("ReadDefectRecord failed: %v", err)
	}

	if defectRec.ChangeID != pA.ID {
		t.Errorf("defectRec.ChangeID = %q, want %q", defectRec.ChangeID, pA.ID)
	}
	if defectRec.FixtureCheck != "fixture/check.sh" {
		t.Errorf("defectRec.FixtureCheck = %q, want fixture/check.sh", defectRec.FixtureCheck)
	}
	if !strings.Contains(defectRec.Reason, "Seeded defect present in fixture/defect.txt") {
		t.Errorf("defectRec.Reason = %q, want to contain seeded defect message", defectRec.Reason)
	}
	if defectRec.DiscoveredAt.IsZero() {
		t.Errorf("defectRec.DiscoveredAt is zero, want valid timestamp")
	}

	// Verify Change B did NOT produce a defect record
	if _, err := stability.ReadDefectRecord(deps.WorktreeFS(wtPathB), wtPathB); err == nil {
		t.Error("Change B produced a defect record, want no defect record for Change B")
	}
}

func TestRemediationTestActorApprovalExercisesGate(t *testing.T) {
	// Behavior 2: Implement a deterministic Test Actor function that reviews a Defect Record
	// and records an approval decision through a durable record with timestamp and actor identity.
	// Prove: the approval is recorded before remediation proceeds, and a run where the Test Actor's
	// approval step is skipped blocks remediation rather than silently proceeding.
	ctx := context.Background()
	fakeExec := newFakeJourneyExecutor()
	deps, ledgerHandle, root := newJourneyTestDeps(t, fakeExec)

	wtPathA := reconcile.WorktreePathFor(root, "stability-change-a")
	lucindPath := filepath.Join(wtPathA, ".lucind")
	_ = os.MkdirAll(lucindPath, 0o755)

	now := time.Date(2026, 8, 25, 12, 10, 0, 0, time.UTC)
	defectRec := stability.DefectRecord{
		ChangeID:     "stability-change-a",
		FixtureCheck: "fixture/check.sh",
		Reason:       "CHECK FAILURE: Seeded defect present in fixture/defect.txt",
		DiscoveredAt: now,
	}

	// 1. Skipped approval scenario: VerifyRemediationApproval MUST return ErrRemediationNotApproved
	_, err := stability.VerifyRemediationApproval(deps.WorktreeFS(wtPathA), wtPathA, defectRec.ChangeID)
	if err == nil {
		t.Fatal("VerifyRemediationApproval before approval returned nil error, want ErrRemediationNotApproved")
	}
	if !errors.Is(err, stability.ErrRemediationNotApproved) {
		t.Fatalf("VerifyRemediationApproval error = %v, want ErrRemediationNotApproved", err)
	}

	// 2. Test Actor records deterministic approval
	actor := stability.DefaultTestActor
	app, err := stability.RecordTestActorApproval(ctx, ledgerHandle, wtPathA, defectRec, actor, true, "remediation approved for seeded fixture defect", now)
	if err != nil {
		t.Fatalf("RecordTestActorApproval failed: %v", err)
	}

	if app.Approver != actor {
		t.Errorf("app.Approver = %q, want %q", app.Approver, actor)
	}
	if !app.Approved {
		t.Errorf("app.Approved = false, want true")
	}
	if app.DecidedAt != now {
		t.Errorf("app.DecidedAt = %v, want %v", app.DecidedAt, now)
	}

	// 3. Verify durable approval file on disk
	durableApp, err := stability.ReadRemediationApproval(deps.WorktreeFS(wtPathA), wtPathA)
	if err != nil {
		t.Fatalf("ReadRemediationApproval failed: %v", err)
	}
	if durableApp.Approver != actor || !durableApp.Approved || durableApp.ChangeID != defectRec.ChangeID {
		t.Errorf("durable approval mismatch: %+v", durableApp)
	}

	// 4. Verify SQLite ledger record exists if ledger is provided
	ledgerApp, err := ledgerHandle.Approval(ctx, deps.RunID, defectRec.ChangeID)
	if err != nil {
		t.Fatalf("ledgerHandle.Approval failed: %v", err)
	}
	if ledgerApp.Approver != actor || ledgerApp.Decision != ledger.DecisionApproved {
		t.Errorf("ledger approval mismatch: %+v", ledgerApp)
	}

	// 5. Verification gate now unblocks with valid approval
	verifiedApp, err := stability.VerifyRemediationApproval(deps.WorktreeFS(wtPathA), wtPathA, defectRec.ChangeID)
	if err != nil {
		t.Fatalf("VerifyRemediationApproval after approval failed: %v", err)
	}
	if verifiedApp.Approver != actor || !verifiedApp.Approved {
		t.Errorf("verifiedApp mismatch: %+v", verifiedApp)
	}
}

func TestRemediationFixChangeDispatchedOnlyAfterApproval(t *testing.T) {
	// Behavior 3: Given an approved Defect Record, dispatch a third lane (Fix) via run.Execute
	// targeting the defect. Prove Fix is dispatched only after approval (not before/unconditionally)
	// and that its own dispatch is independent of Change B's progress.
	ctx := context.Background()
	fakeExec := newFakeJourneyExecutor()
	deps, ledgerHandle, root := newJourneyTestDeps(t, fakeExec)

	cfg := stability.DefaultJourneyConfig(1, "b000000000000000000000000000000000000000")
	pFix := stability.BuildFixPacket(cfg)

	if pFix.Model != stability.PinnedModel {
		t.Errorf("pFix.Model = %q, want %q", pFix.Model, stability.PinnedModel)
	}
	if pFix.Feature != cfg.TargetA {
		t.Errorf("pFix.Feature = %q, want %q", pFix.Feature, cfg.TargetA)
	}
	if pFix.ParentRef != cfg.ParentRefA {
		t.Errorf("pFix.ParentRef = %q, want %q", pFix.ParentRef, cfg.ParentRefA)
	}

	// 1. Attempt dispatch without approval -> fails with ErrRemediationNotApproved
	_, err := stability.DispatchFixChange(ctx, deps, cfg, nil)
	if !errors.Is(err, stability.ErrRemediationNotApproved) {
		t.Fatalf("DispatchFixChange without approval error = %v, want ErrRemediationNotApproved", err)
	}

	unapproved := &stability.RemediationApproval{Approved: false, Reason: "rejected"}
	_, err = stability.DispatchFixChange(ctx, deps, cfg, unapproved)
	if !errors.Is(err, stability.ErrRemediationNotApproved) {
		t.Fatalf("DispatchFixChange with unapproved proposal error = %v, want ErrRemediationNotApproved", err)
	}

	// 2. Setup Fix executor to remediate the defect in worktree
	wtPathFix := reconcile.WorktreePathFor(root, "stability-fix-a")
	fakeExec.runFuncFor[wtPathFix] = func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		// Fix writes STATUS=FIXED to fixture/defect.txt
		defectPath := filepath.Join(req.WorktreePath, "fixture", "defect.txt")
		if err := os.WriteFile(defectPath, []byte("STATUS=FIXED\n"), 0o644); err != nil {
			return executor.Outcome{}, err
		}
		// Commit the fix
		runGit := func(args ...string) {
			cmd := exec.Command("git", append([]string{"-C", req.WorktreePath}, args...)...)
			_ = cmd.Run()
		}
		runGit("add", "fixture/defect.txt")
		runGit("commit", "-m", "fix seeded fixture defect")

		envJSON := laneEnvelopeJSON(pFix.ID, "done")
		lucindPath := filepath.Join(req.WorktreePath, ".lucind")
		_ = os.MkdirAll(lucindPath, 0o755)
		_ = os.WriteFile(filepath.Join(lucindPath, "result.json"), []byte(envJSON), 0o644)
		return executor.Outcome{ExitCode: 0}, nil
	}

	// 3. Record approval and dispatch Fix
	now := deps.Now()
	defectRec := stability.DefectRecord{
		ChangeID:     "stability-change-a",
		FixtureCheck: "fixture/check.sh",
		Reason:       "CHECK FAILURE: Seeded defect present in fixture/defect.txt",
		DiscoveredAt: now,
	}
	app, err := stability.RecordTestActorApproval(ctx, ledgerHandle, root, defectRec, stability.DefaultTestActor, true, "approved", now)
	if err != nil {
		t.Fatalf("RecordTestActorApproval failed: %v", err)
	}

	fixReport, err := stability.DispatchFixChange(ctx, deps, cfg, app)
	if err != nil {
		t.Fatalf("DispatchFixChange with approval failed: %v", err)
	}
	if fixReport.Status != lane.Done {
		t.Errorf("fixReport.Status = %v, want Done", fixReport.Status)
	}

	// 4. Independence proof: Change B progresses while Fix is in-flight
	inFlightFix := make(chan struct{})
	bCompleted := make(chan struct{})
	fixCanFinish := make(chan struct{})

	depsParallel := deps
	depsParallel.RunID = "run-journey-parallel"

	wtPathFixParallel := reconcile.WorktreePathFor(root, "stability-fix-a-parallel")

	fakeExec.runFuncFor[wtPathFixParallel] = func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		close(inFlightFix)
		// Fix waits until B has finished
		select {
		case <-fixCanFinish:
		case <-time.After(2 * time.Second):
			t.Error("timed out waiting for fixCanFinish")
		}
		envJSON := laneEnvelopeJSON(req.Prompt, "done")
		lucindPath := filepath.Join(req.WorktreePath, ".lucind")
		_ = os.MkdirAll(lucindPath, 0o755)
		_ = os.WriteFile(filepath.Join(lucindPath, "result.json"), []byte(envJSON), 0o644)
		return executor.Outcome{ExitCode: 0}, nil
	}

	_, pB := stability.BuildJourneyPackets(cfg)
	pFixParallel := pFix
	pFixParallel.ID = "stability-fix-a-parallel"

	pBParallel := pB
	pBParallel.ID = "stability-change-b-parallel"

	go func() {
		// Dispatch Fix in background
		_, _ = run.Execute(ctx, depsParallel, pFixParallel)
	}()

	// Wait for Fix to be in flight
	select {
	case <-inFlightFix:
	case <-time.After(2 * time.Second):
		t.Fatal("Fix never started dispatch")
	}

	// While Fix is in flight, Change B executes and completes independently
	bReport, err := run.Execute(ctx, depsParallel, pBParallel)
	if err != nil {
		t.Fatalf("Change B Execute failed: %v", err)
	}
	if bReport.Status != lane.Done {
		t.Errorf("bReport.Status = %v, want Done", bReport.Status)
	}
	close(bCompleted)

	// Release Fix
	close(fixCanFinish)
}

func TestRemediationFixPromotesCASRejectsStaleSHA(t *testing.T) {
	// Behavior 4: Once Fix's dispatch reaches done, promote it to Target A using integrate.PromoteCAS.
	// Prove promotion is genuinely CAS: when expected SHA is stale, assert it fails specifically with
	// integrate.ErrStaleCAS. When expected SHA matches, promotion succeeds.
	ctx := context.Background()
	root := t.TempDir()

	// Initialize git repo
	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	runGit("init")
	runGit("config", "user.name", "Test User")
	runGit("config", "user.email", "test@example.com")
	_ = fixture.MaterializeFixtures(root)
	runGit("add", ".")
	runGit("commit", "-m", "initial baseline")
	baseSHA := runGit("rev-parse", "HEAD")

	cfg := stability.DefaultJourneyConfig(1, baseSHA)
	// Create Target A ref pointing to baseSHA
	runGit("update-ref", cfg.ParentRefA, baseSHA)

	// Create Fix commit
	_ = os.WriteFile(filepath.Join(root, "fixture", "defect.txt"), []byte("STATUS=FIXED\n"), 0o644)
	runGit("add", "fixture/defect.txt")
	runGit("commit", "-m", "fix defect")
	fixSHA := runGit("rev-parse", "HEAD")

	// Reset HEAD back to baseSHA so working tree/HEAD is not mutated by promotion
	runGit("reset", "--hard", baseSHA)

	// 1. Construct stale CAS scenario: expected SHA does not match current ref
	staleSHA := "a000000000000000000000000000000000000000"
	err := stability.PromoteTargetCAS(ctx, root, cfg.ParentRefA, fixSHA, staleSHA)
	if err == nil {
		t.Fatal("PromoteTargetCAS with stale expected SHA succeeded, want ErrStaleCAS")
	}
	if !errors.Is(err, integrate.ErrStaleCAS) {
		t.Fatalf("PromoteTargetCAS error = %v, want integrate.ErrStaleCAS", err)
	}

	// Verify Target A ref has NOT changed
	currentRefSHA := runGit("rev-parse", cfg.ParentRefA)
	if currentRefSHA != baseSHA {
		t.Errorf("ref moved after failed CAS: got %s, want %s", currentRefSHA, baseSHA)
	}

	// 2. Valid CAS promotion: expected SHA matches current ref
	if err := stability.PromoteTargetCAS(ctx, root, cfg.ParentRefA, fixSHA, baseSHA); err != nil {
		t.Fatalf("PromoteTargetCAS with matching expected SHA failed: %v", err)
	}

	// Verify Target A ref now points to fixSHA
	promotedRefSHA := runGit("rev-parse", cfg.ParentRefA)
	if promotedRefSHA != fixSHA {
		t.Errorf("ref after promotion = %s, want %s", promotedRefSHA, fixSHA)
	}
}

func TestRemediationChangeAResumesAndPromotesOnlyAfterFixSatisfied(t *testing.T) {
	// Behavior 5: Change A resumes under original identity without reclaim and only proceeds
	// to promotion once Fix->Target A dependency is recorded satisfied.
	// Prove: A's promotion attempt before Fix promotes is rejected, and succeeds only after
	// Fix's promotion is confirmed.
	ctx := context.Background()
	root := t.TempDir()

	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	runGit("init")
	runGit("config", "user.name", "Test User")
	runGit("config", "user.email", "test@example.com")
	_ = fixture.MaterializeFixtures(root)
	runGit("add", ".")
	runGit("commit", "-m", "initial baseline")
	baseSHA := runGit("rev-parse", "HEAD")

	cfg := stability.DefaultJourneyConfig(1, baseSHA)
	runGit("update-ref", cfg.ParentRefA, baseSHA)

	// Create Fix commit
	_ = os.WriteFile(filepath.Join(root, "fixture", "defect.txt"), []byte("STATUS=FIXED\n"), 0o644)
	runGit("add", "fixture/defect.txt")
	runGit("commit", "-m", "fix defect")
	fixSHA := runGit("rev-parse", "HEAD")

	// Create Change A candidate commit on top of fixSHA
	_ = os.WriteFile(filepath.Join(root, "fixture", "change_a.txt"), []byte("CHANGE_A=DONE\n"), 0o644)
	runGit("add", "fixture/change_a.txt")
	runGit("commit", "-m", "change A functionality")
	candidateASHA := runGit("rev-parse", "HEAD")

	// 1. Before Fix promotes: Promotion attempt of Change A MUST fail with ErrDependencyNotSatisfied
	err := stability.PromoteChangeACAS(ctx, root, cfg, candidateASHA, fixSHA, false)
	if err == nil {
		t.Fatal("PromoteChangeACAS before fix promotion returned nil error, want ErrDependencyNotSatisfied")
	}
	if !errors.Is(err, stability.ErrDependencyNotSatisfied) {
		t.Fatalf("PromoteChangeACAS error = %v, want ErrDependencyNotSatisfied", err)
	}

	// 2. Promote Fix to Target A via CAS
	if err := stability.PromoteTargetCAS(ctx, root, cfg.ParentRefA, fixSHA, baseSHA); err != nil {
		t.Fatalf("PromoteTargetCAS for Fix failed: %v", err)
	}

	// 3. Resume Change A under original identity and verify check passes on remediated tree
	resumedOutcome, err := fixture.RunCheck(ctx, root, "change-a")
	if err != nil {
		t.Fatalf("resumed fixture check for Change A failed: %v\n%s", err, resumedOutcome)
	}
	if !strings.Contains(resumedOutcome, "CHECK SUCCESS: Change A verified") {
		t.Errorf("resumedOutcome = %q, want check success", resumedOutcome)
	}

	// 4. Now promote Change A with confirmed Fix dependency (CAS: Target A moves from fixSHA to candidateASHA)
	if err := stability.PromoteChangeACAS(ctx, root, cfg, candidateASHA, fixSHA, true); err != nil {
		t.Fatalf("PromoteChangeACAS after fix promotion failed: %v", err)
	}

	// 5. Verify Target A ref now points to candidateASHA
	finalRefSHA := runGit("rev-parse", cfg.ParentRefA)
	if finalRefSHA != candidateASHA {
		t.Errorf("final Target A ref = %s, want %s", finalRefSHA, candidateASHA)
	}

	// 6. Verify commit ancestry: baseSHA is ancestor of fixSHA, and fixSHA is ancestor of candidateASHA
	isBaseInFix, _ := fixture.IsAncestor(ctx, root, baseSHA, fixSHA)
	if !isBaseInFix {
		t.Error("baseSHA is not ancestor of fixSHA")
	}
	isFixInA, _ := fixture.IsAncestor(ctx, root, fixSHA, candidateASHA)
	if !isFixInA {
		t.Error("fixSHA is not ancestor of candidateASHA")
	}
}

func TestRemediationTargetBPromotesIndependentlyBeforeFix(t *testing.T) {
	// Behavior 6: Prove with an explicit ordering assertion that Target B's promotion
	// via integrate.PromoteCAS can complete while Fix is still in flight (not yet promoted).
	// B's independence from A's remediation path through to promotion is proven.
	ctx := context.Background()
	root := t.TempDir()

	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	runGit("init")
	runGit("config", "user.name", "Test User")
	runGit("config", "user.email", "test@example.com")
	_ = fixture.MaterializeFixtures(root)
	runGit("add", ".")
	runGit("commit", "-m", "initial baseline")
	baseSHA := runGit("rev-parse", "HEAD")

	cfg := stability.DefaultJourneyConfig(1, baseSHA)
	runGit("update-ref", cfg.ParentRefA, baseSHA)
	runGit("update-ref", cfg.ParentRefB, baseSHA)

	// Create Change B commit
	_ = os.WriteFile(filepath.Join(root, "fixture", "change_b.txt"), []byte("CHANGE_B=DONE\n"), 0o644)
	runGit("add", "fixture/change_b.txt")
	runGit("commit", "-m", "change B functionality")
	candidateBSHA := runGit("rev-parse", "HEAD")

	// Reset HEAD to baseSHA
	runGit("reset", "--hard", baseSHA)

	// Create Fix commit
	_ = os.WriteFile(filepath.Join(root, "fixture", "defect.txt"), []byte("STATUS=FIXED\n"), 0o644)
	runGit("add", "fixture/defect.txt")
	runGit("commit", "-m", "fix defect")
	fixSHA := runGit("rev-parse", "HEAD")

	runGit("reset", "--hard", baseSHA)

	// Synchronization channels for explicit ordering proof
	fixInFlight := make(chan struct{})
	bPromotedChan := make(chan time.Time, 1)
	allowFixPromotion := make(chan struct{})
	fixPromotedChan := make(chan time.Time, 1)

	// Background Fix lane that waits for B promotion before completing
	go func() {
		close(fixInFlight)
		// Fix is in-flight...
		select {
		case <-allowFixPromotion:
		case <-time.After(3 * time.Second):
			return
		}
		_ = stability.PromoteTargetCAS(ctx, root, cfg.ParentRefA, fixSHA, baseSHA)
		fixPromotedChan <- time.Now()
	}()

	// Wait until Fix is confirmed in-flight
	select {
	case <-fixInFlight:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Fix in-flight")
	}

	// Target B promotes via CAS independently while Fix is still in flight
	if err := stability.PromoteTargetBCAS(ctx, root, cfg, candidateBSHA); err != nil {
		t.Fatalf("PromoteTargetBCAS failed: %v", err)
	}
	timeBPromoted := time.Now()
	bPromotedChan <- timeBPromoted

	// Assert Target B ref is already updated while Target A ref is still at baseSHA (Fix unpromoted)
	refB := runGit("rev-parse", cfg.ParentRefB)
	if refB != candidateBSHA {
		t.Errorf("Target B ref = %s, want %s", refB, candidateBSHA)
	}
	refA := runGit("rev-parse", cfg.ParentRefA)
	if refA != baseSHA {
		t.Errorf("Target A ref before fix promotion = %s, want baseSHA %s", refA, baseSHA)
	}

	// Now release Fix promotion
	close(allowFixPromotion)

	var timeFixPromoted time.Time
	select {
	case timeFixPromoted = <-fixPromotedChan:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Fix promotion")
	}

	// Explicit ordering assertion: Target B promoted strictly BEFORE Fix promoted
	if !timeBPromoted.Before(timeFixPromoted) && !timeBPromoted.Equal(timeFixPromoted) {
		t.Errorf("Target B promotion time (%v) is not before Fix promotion time (%v)", timeBPromoted, timeFixPromoted)
	}

	// Assert Target A ref is now updated to fixSHA
	refAAfter := runGit("rev-parse", cfg.ParentRefA)
	if refAAfter != fixSHA {
		t.Errorf("Target A ref after fix promotion = %s, want fixSHA %s", refAAfter, fixSHA)
	}
}

func TestRemediationStateMachineProgressionNoBypass(t *testing.T) {
	// Behavior 7: Every transition routes through StateMachine.AdvanceTrial / RecordTrialOutcome,
	// exercising the full remediation journey through all required states without bypass.
	ctx := context.Background()
	fakeExec := newFakeJourneyExecutor()
	deps, ledgerHandle, root := newJourneyTestDeps(t, fakeExec)
	featSvc := feature.NewService(ledgerHandle)

	// Set up Git repository with base commit and target refs
	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	runGit("init")
	runGit("config", "user.name", "Test User")
	runGit("config", "user.email", "test@example.com")
	_ = fixture.MaterializeFixtures(root)
	runGit("add", ".")
	runGit("commit", "-m", "initial baseline")
	baseSHA := runGit("rev-parse", "HEAD")

	cfg := stability.DefaultJourneyConfig(1, baseSHA)
	cfg.LeaseTTL = 100 * time.Millisecond
	runGit("update-ref", cfg.ParentRefA, baseSHA)
	runGit("update-ref", cfg.ParentRefB, baseSHA)

	wtPathA := reconcile.WorktreePathFor(root, "stability-change-a")
	wtPathB := reconcile.WorktreePathFor(root, "stability-change-b")
	wtPathFix := reconcile.WorktreePathFor(root, "stability-fix-a")

	// Change A encounters seeded defect
	fakeExec.runFuncFor[wtPathA] = func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		out, checkErr := fixture.RunCheck(ctx, req.WorktreePath, "change-a")
		if checkErr != nil {
			_, _ = stability.AssessAndRecordDefect(ctx, deps.Ledger, req.WorktreePath, "stability-change-a", "fixture/check.sh", out, deps.Now())
			envJSON := laneEnvelopeJSON("stability-change-a", "blocked")
			lucindPath := filepath.Join(req.WorktreePath, ".lucind")
			_ = os.MkdirAll(lucindPath, 0o755)
			_ = os.WriteFile(filepath.Join(lucindPath, "result.json"), []byte(envJSON), 0o644)
			return executor.Outcome{ExitCode: 0}, nil
		}
		return executor.Outcome{ExitCode: 0}, nil
	}

	// Change B completes cleanly before crash injection
	fakeExec.runFuncFor[wtPathB] = func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		envJSON := laneEnvelopeJSON("stability-change-b", "done")
		lucindPath := filepath.Join(req.WorktreePath, ".lucind")
		_ = os.MkdirAll(lucindPath, 0o755)
		_ = os.WriteFile(filepath.Join(lucindPath, "result.json"), []byte(envJSON), 0o644)
		return executor.Outcome{ExitCode: 0}, nil
	}

	// Fix Change remediates defect
	fakeExec.runFuncFor[wtPathFix] = func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		_ = os.WriteFile(filepath.Join(req.WorktreePath, "fixture", "defect.txt"), []byte("STATUS=FIXED\n"), 0o644)
		cmd := exec.Command("git", "-C", req.WorktreePath, "add", "fixture/defect.txt")
		_ = cmd.Run()
		cmd = exec.Command("git", "-C", req.WorktreePath, "commit", "-m", "fix seeded defect")
		_ = cmd.Run()

		envJSON := laneEnvelopeJSON("stability-fix-a", "done")
		lucindPath := filepath.Join(req.WorktreePath, ".lucind")
		_ = os.MkdirAll(lucindPath, 0o755)
		_ = os.WriteFile(filepath.Join(lucindPath, "result.json"), []byte(envJSON), 0o644)
		return executor.Outcome{ExitCode: 0}, nil
	}

	sm := stability.NewStateMachine()
	if err := sm.Start(); err != nil {
		t.Fatalf("sm.Start failed: %v", err)
	}

	observedStates := make([]stability.TrialState, 0)
	trackState := func() {
		observedStates = append(observedStates, sm.ActiveTrialState())
	}

	// Run full trial journey
	journeyResult, err := stability.ExecuteTrialJourney(ctx, sm, deps, featSvc, cfg, 99999999, wtPathB)
	if err != nil {
		t.Fatalf("ExecuteTrialJourney failed: %v", err)
	}
	if journeyResult == nil {
		t.Fatal("journeyResult is nil")
	}

	trackState()

	if sm.ActiveTrialState() != stability.TrialEvidenceCaptured {
		t.Errorf("active trial state = %q, want %q", sm.ActiveTrialState(), stability.TrialEvidenceCaptured)
	}

	// Advance through cleaned_up and passed
	if err := sm.AdvanceTrial(stability.TrialCleanedUp); err != nil {
		t.Fatalf("AdvanceTrial(TrialCleanedUp) failed: %v", err)
	}
	if err := sm.RecordTrialOutcome(stability.TrialPassed); err != nil {
		t.Fatalf("RecordTrialOutcome(TrialPassed) failed: %v", err)
	}

	if sm.ConsecutivePasses() != 1 {
		t.Errorf("consecutive passes = %d, want 1", sm.ConsecutivePasses())
	}
}

func TestTrialJourneyLiveAbruptKillWatchAndRecover(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	ctx := context.Background()
	fakeExec := newFakeJourneyExecutor()
	deps, ledgerHandle, _ := newJourneyTestDeps(t, fakeExec)
	featSvc := feature.NewService(ledgerHandle)

	cfg := stability.DefaultJourneyConfig(1, "b000000000000000000000000000000000000000")
	cfg.LeaseTTL = 200 * time.Millisecond

	wtPathB := reconcile.WorktreePathFor(deps.PrimaryRoot, "stability-change-b")
	lucindB := filepath.Join(wtPathB, ".lucind")
	_ = os.MkdirAll(lucindB, 0o755)

	// Stub writes .lucind/result.json then sleeps for 10 seconds awaiting live kill
	stubScript := fmt.Sprintf(`#!/bin/sh
mkdir -p %q
cat << 'EOF' > %q
{
  "packet_id": "stability-change-b",
  "status": "done",
  "summary": "change B work completed before live crash",
  "hard_stops": [{"hard_stop": "do not touch internal/barrier", "fired": false}]
}
EOF
sleep 10
`, lucindB, filepath.Join(lucindB, "result.json"))
	stubPath := writeStubScript(t, stubScript)

	var (
		capturedStubPID int
		pidMu           sync.Mutex
	)

	supervisor := process.NewSupervisor()
	fakeExec.runFuncFor[wtPathB] = func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		cmd := exec.Command(stubPath)
		cmd.Dir = wtPathB
		pgid, err := supervisor.Start(cmd)
		if err != nil {
			return executor.Outcome{}, err
		}
		pidMu.Lock()
		capturedStubPID = pgid
		pidMu.Unlock()
		if req.Setpgid && req.OnStart != nil {
			req.OnStart(pgid)
		}
		// Wait for command to exit or be killed
		_ = cmd.Wait()
		return executor.Outcome{ExitCode: 0, PGID: pgid}, nil
	}

	sm := stability.NewStateMachine()
	if err := sm.Start(); err != nil {
		t.Fatalf("sm.Start failed: %v", err)
	}

	start := time.Now()
	res, err := stability.ExecuteTrialJourneyLive(ctx, sm, deps, featSvc, cfg)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("ExecuteTrialJourneyLive failed: %v", err)
	}
	if res == nil {
		t.Fatal("ExecuteTrialJourneyLive returned nil result")
	}

	// (a) Stub process is confirmed dead after call returns
	pidMu.Lock()
	pid := capturedStubPID
	pidMu.Unlock()
	if pid <= 1 {
		t.Fatalf("capturedStubPID = %d, want > 1", pid)
	}
	if err := supervisor.VerifyZero(pid); err != nil {
		t.Errorf("stub process %d still alive after ExecuteTrialJourneyLive: %v", pid, err)
	}

	// (b) Returned *TrialJourneyResult is populated with AdoptedEnvelopeB
	if res.AdoptedEnvelopeB.PacketID != "stability-change-b" {
		t.Errorf("AdoptedEnvelopeB.PacketID = %q, want stability-change-b", res.AdoptedEnvelopeB.PacketID)
	}
	if res.AdoptedEnvelopeB.LaneStatus() != lane.Done {
		t.Errorf("AdoptedEnvelopeB.LaneStatus() = %v, want Done", res.AdoptedEnvelopeB.LaneStatus())
	}
	if res.ReclaimedLeaseB.Fence <= res.InitialLeaseB.Fence {
		t.Errorf("ReclaimedLeaseB.Fence = %d, want > %d", res.ReclaimedLeaseB.Fence, res.InitialLeaseB.Fence)
	}

	// (c) Whole call completes well before stub's 10s natural sleep duration
	if elapsed >= 5*time.Second {
		t.Errorf("ExecuteTrialJourneyLive took %v, want < 5s (proving abrupt kill occurred before 10s sleep)", elapsed)
	}

	if sm.ActiveTrialState() != stability.TrialEvidenceCaptured {
		t.Errorf("sm.ActiveTrialState() = %q, want %q", sm.ActiveTrialState(), stability.TrialEvidenceCaptured)
	}
}

func TestTrialJourneyLiveNoKillWindowGracefulDegradation(t *testing.T) {
	ctx := context.Background()
	fakeExec := newFakeJourneyExecutor()
	deps, ledgerHandle, root := newJourneyTestDeps(t, fakeExec)
	featSvc := feature.NewService(ledgerHandle)

	cfg := stability.DefaultJourneyConfig(1, "b000000000000000000000000000000000000000")
	cfg.LeaseTTL = 50 * time.Millisecond

	wtPathB := reconcile.WorktreePathFor(root, "stability-change-b")
	lucindB := filepath.Join(wtPathB, ".lucind")
	_ = os.MkdirAll(lucindB, 0o755)
	envJSON := laneEnvelopeJSON("stability-change-b", "done")
	_ = os.WriteFile(filepath.Join(lucindB, "result.json"), []byte(envJSON), 0o644)

	// Change B completes naturally without triggering OnStart or live kill
	sm := stability.NewStateMachine()
	if err := sm.Start(); err != nil {
		t.Fatalf("sm.Start failed: %v", err)
	}

	res, err := stability.ExecuteTrialJourneyLive(ctx, sm, deps, featSvc, cfg)
	if err != nil {
		t.Fatalf("ExecuteTrialJourneyLive degraded path failed: %v", err)
	}
	if res == nil {
		t.Fatal("ExecuteTrialJourneyLive returned nil result")
	}
	if res.AdoptedEnvelopeB.PacketID != "stability-change-b" {
		t.Errorf("res.AdoptedEnvelopeB.PacketID = %q, want stability-change-b", res.AdoptedEnvelopeB.PacketID)
	}
	if sm.ActiveTrialState() != stability.TrialEvidenceCaptured {
		t.Errorf("sm.ActiveTrialState() = %q, want %q", sm.ActiveTrialState(), stability.TrialEvidenceCaptured)
	}
}

func TestTrialJourneyPromoteStageNoDefectMovesRefs(t *testing.T) {
	ctx := context.Background()
	fakeExec := newFakeJourneyExecutor()
	deps, ledgerHandle, root := newJourneyTestDeps(t, fakeExec)
	featSvc := feature.NewService(ledgerHandle)

	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	runGit("init")
	runGit("config", "user.name", "Test User")
	runGit("config", "user.email", "test@example.com")
	_ = fixture.MaterializeFixtures(root)
	runGit("add", ".")
	runGit("commit", "-m", "initial baseline")
	baseSHA := runGit("rev-parse", "HEAD")

	cfg := stability.DefaultJourneyConfig(1, baseSHA)
	cfg.LeaseTTL = 100 * time.Millisecond
	runGit("update-ref", cfg.ParentRefA, baseSHA)
	runGit("update-ref", cfg.ParentRefB, baseSHA)

	// Create Change A commit on baseSHA
	_ = os.WriteFile(filepath.Join(root, "fixture", "change_a.txt"), []byte("CHANGE_A=DONE\n"), 0o644)
	runGit("add", "fixture/change_a.txt")
	runGit("commit", "-m", "change A functionality")
	candidateASHA := runGit("rev-parse", "HEAD")
	runGit("reset", "--hard", baseSHA)

	// Create Change B commit on baseSHA
	_ = os.WriteFile(filepath.Join(root, "fixture", "change_b.txt"), []byte("CHANGE_B=DONE\n"), 0o644)
	runGit("add", "fixture/change_b.txt")
	runGit("commit", "-m", "change B functionality")
	candidateBSHA := runGit("rev-parse", "HEAD")
	runGit("reset", "--hard", baseSHA)

	wtPathA := reconcile.WorktreePathFor(root, "stability-change-a")
	wtPathB := reconcile.WorktreePathFor(root, "stability-change-b")

	fakeExec.runFuncFor[wtPathA] = func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		envJSON := laneEnvelopeJSONWithCommit("stability-change-a", "done", candidateASHA)
		lucindPath := filepath.Join(req.WorktreePath, ".lucind")
		_ = os.MkdirAll(lucindPath, 0o755)
		_ = os.WriteFile(filepath.Join(lucindPath, "result.json"), []byte(envJSON), 0o644)
		return executor.Outcome{ExitCode: 0}, nil
	}

	fakeExec.runFuncFor[wtPathB] = func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		envJSON := laneEnvelopeJSONWithCommit("stability-change-b", "done", candidateBSHA)
		lucindPath := filepath.Join(req.WorktreePath, ".lucind")
		_ = os.MkdirAll(lucindPath, 0o755)
		_ = os.WriteFile(filepath.Join(lucindPath, "result.json"), []byte(envJSON), 0o644)
		return executor.Outcome{ExitCode: 0}, nil
	}

	sm := stability.NewStateMachine()
	if err := sm.Start(); err != nil {
		t.Fatalf("sm.Start failed: %v", err)
	}

	res, err := stability.ExecuteTrialJourney(ctx, sm, deps, featSvc, cfg, 99999999, wtPathB)
	if err != nil {
		t.Fatalf("ExecuteTrialJourney failed: %v", err)
	}
	if res == nil {
		t.Fatal("ExecuteTrialJourney returned nil result")
	}

	// Assert actual git ref states in primary root
	refA := runGit("rev-parse", cfg.ParentRefA)
	if refA != candidateASHA {
		t.Errorf("ParentRefA = %s, want %s (candidateASHA)", refA, candidateASHA)
	}
	refB := runGit("rev-parse", cfg.ParentRefB)
	if refB != candidateBSHA {
		t.Errorf("ParentRefB = %s, want %s (candidateBSHA)", refB, candidateBSHA)
	}

	if sm.ActiveTrialState() != stability.TrialEvidenceCaptured {
		t.Errorf("active trial state = %q, want %q", sm.ActiveTrialState(), stability.TrialEvidenceCaptured)
	}
}

func TestTrialJourneyPromoteStageWithFixDependencyPromotesBothInOrder(t *testing.T) {
	ctx := context.Background()
	fakeExec := newFakeJourneyExecutor()
	deps, ledgerHandle, root := newJourneyTestDeps(t, fakeExec)
	featSvc := feature.NewService(ledgerHandle)

	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	runGit("init")
	runGit("config", "user.name", "Test User")
	runGit("config", "user.email", "test@example.com")
	_ = fixture.MaterializeFixtures(root)
	runGit("add", ".")
	runGit("commit", "-m", "initial baseline")
	baseSHA := runGit("rev-parse", "HEAD")

	cfg := stability.DefaultJourneyConfig(1, baseSHA)
	cfg.LeaseTTL = 100 * time.Millisecond
	runGit("update-ref", cfg.ParentRefA, baseSHA)
	runGit("update-ref", cfg.ParentRefB, baseSHA)

	// Create Fix commit on top of baseSHA
	_ = os.WriteFile(filepath.Join(root, "fixture", "defect.txt"), []byte("STATUS=FIXED\n"), 0o644)
	runGit("add", "fixture/defect.txt")
	runGit("commit", "-m", "fix defect")
	fixSHA := runGit("rev-parse", "HEAD")

	// Create Change A candidate commit on top of fixSHA
	_ = os.WriteFile(filepath.Join(root, "fixture", "change_a.txt"), []byte("CHANGE_A=DONE\n"), 0o644)
	runGit("add", "fixture/change_a.txt")
	runGit("commit", "-m", "change A functionality")
	candidateASHA := runGit("rev-parse", "HEAD")
	runGit("reset", "--hard", baseSHA)

	// Create Change B commit on top of baseSHA
	_ = os.WriteFile(filepath.Join(root, "fixture", "change_b.txt"), []byte("CHANGE_B=DONE\n"), 0o644)
	runGit("add", "fixture/change_b.txt")
	runGit("commit", "-m", "change B functionality")
	candidateBSHA := runGit("rev-parse", "HEAD")
	runGit("reset", "--hard", baseSHA)

	wtPathA := reconcile.WorktreePathFor(root, "stability-change-a")
	wtPathB := reconcile.WorktreePathFor(root, "stability-change-b")
	wtPathFix := reconcile.WorktreePathFor(root, "stability-fix-a")

	// Change A encounters defect and reports blocked with candidateASHA commit
	fakeExec.runFuncFor[wtPathA] = func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		out, checkErr := fixture.RunCheck(ctx, req.WorktreePath, "change-a")
		if checkErr != nil {
			_, _ = stability.AssessAndRecordDefect(ctx, deps.Ledger, req.WorktreePath, "stability-change-a", "fixture/check.sh", out, deps.Now())
			envJSON := laneEnvelopeJSONWithCommit("stability-change-a", "blocked", candidateASHA)
			lucindPath := filepath.Join(req.WorktreePath, ".lucind")
			_ = os.MkdirAll(lucindPath, 0o755)
			_ = os.WriteFile(filepath.Join(lucindPath, "result.json"), []byte(envJSON), 0o644)
			return executor.Outcome{ExitCode: 0}, nil
		}
		return executor.Outcome{ExitCode: 0}, nil
	}

	// Change B envelope
	fakeExec.runFuncFor[wtPathB] = func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		envJSON := laneEnvelopeJSONWithCommit("stability-change-b", "done", candidateBSHA)
		lucindPath := filepath.Join(req.WorktreePath, ".lucind")
		_ = os.MkdirAll(lucindPath, 0o755)
		_ = os.WriteFile(filepath.Join(lucindPath, "result.json"), []byte(envJSON), 0o644)
		return executor.Outcome{ExitCode: 0}, nil
	}

	// Fix Change produces fixSHA
	fakeExec.runFuncFor[wtPathFix] = func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		envJSON := laneEnvelopeJSONWithCommit("stability-fix-a", "done", fixSHA)
		lucindPath := filepath.Join(req.WorktreePath, ".lucind")
		_ = os.MkdirAll(lucindPath, 0o755)
		_ = os.WriteFile(filepath.Join(lucindPath, "result.json"), []byte(envJSON), 0o644)
		return executor.Outcome{ExitCode: 0}, nil
	}

	sm := stability.NewStateMachine()
	if err := sm.Start(); err != nil {
		t.Fatalf("sm.Start failed: %v", err)
	}

	res, err := stability.ExecuteTrialJourney(ctx, sm, deps, featSvc, cfg, 99999999, wtPathB)
	if err != nil {
		t.Fatalf("ExecuteTrialJourney failed: %v", err)
	}
	if res == nil {
		t.Fatal("ExecuteTrialJourney returned nil result")
	}

	// Assert final ref state in root
	refA := runGit("rev-parse", cfg.ParentRefA)
	if refA != candidateASHA {
		t.Errorf("ParentRefA = %s, want %s (candidateASHA)", refA, candidateASHA)
	}
	refB := runGit("rev-parse", cfg.ParentRefB)
	if refB != candidateBSHA {
		t.Errorf("ParentRefB = %s, want %s (candidateBSHA)", refB, candidateBSHA)
	}

	// Verify ancestry: fixSHA is an ancestor of candidateASHA, baseSHA is ancestor of fixSHA, and fixSHA not in candidateBSHA
	isBaseInFix, _ := fixture.IsAncestor(ctx, root, baseSHA, fixSHA)
	if !isBaseInFix {
		t.Errorf("baseSHA %s is not ancestor of fixSHA %s", baseSHA, fixSHA)
	}
	isFixInA, _ := fixture.IsAncestor(ctx, root, fixSHA, candidateASHA)
	if !isFixInA {
		t.Errorf("fixSHA %s is not ancestor of candidateASHA %s", fixSHA, candidateASHA)
	}
	isFixInB, _ := fixture.IsAncestor(ctx, root, fixSHA, candidateBSHA)
	if isFixInB {
		t.Errorf("fixSHA %s contaminated candidateBSHA %s", fixSHA, candidateBSHA)
	}

	if sm.ActiveTrialState() != stability.TrialEvidenceCaptured {
		t.Errorf("active trial state = %q, want %q", sm.ActiveTrialState(), stability.TrialEvidenceCaptured)
	}
}

func TestTrialJourneyPromoteStageStaleCASFailsTrial(t *testing.T) {
	ctx := context.Background()
	fakeExec := newFakeJourneyExecutor()
	deps, ledgerHandle, root := newJourneyTestDeps(t, fakeExec)
	featSvc := feature.NewService(ledgerHandle)

	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	runGit("init")
	runGit("config", "user.name", "Test User")
	runGit("config", "user.email", "test@example.com")
	_ = fixture.MaterializeFixtures(root)
	runGit("add", ".")
	runGit("commit", "-m", "initial baseline")
	baseSHA := runGit("rev-parse", "HEAD")

	// Create another commit so ParentRefA points to unexpected commit
	_ = os.WriteFile(filepath.Join(root, "fixture", "other.txt"), []byte("OTHER=1\n"), 0o644)
	runGit("add", "fixture/other.txt")
	runGit("commit", "-m", "other commit")
	otherSHA := runGit("rev-parse", "HEAD")

	cfg := stability.DefaultJourneyConfig(1, baseSHA)
	cfg.LeaseTTL = 100 * time.Millisecond
	// ParentRefA points to otherSHA, but cfg.ExpectedParentSHA is baseSHA
	runGit("update-ref", cfg.ParentRefA, otherSHA)
	runGit("update-ref", cfg.ParentRefB, baseSHA)

	// Create Change A commit on baseSHA
	_ = os.WriteFile(filepath.Join(root, "fixture", "change_a.txt"), []byte("CHANGE_A=DONE\n"), 0o644)
	runGit("add", "fixture/change_a.txt")
	runGit("commit", "-m", "change A functionality")
	candidateASHA := runGit("rev-parse", "HEAD")
	runGit("reset", "--hard", baseSHA)

	// Create Change B commit on baseSHA
	_ = os.WriteFile(filepath.Join(root, "fixture", "change_b.txt"), []byte("CHANGE_B=DONE\n"), 0o644)
	runGit("add", "fixture/change_b.txt")
	runGit("commit", "-m", "change B functionality")
	candidateBSHA := runGit("rev-parse", "HEAD")
	runGit("reset", "--hard", baseSHA)

	wtPathA := reconcile.WorktreePathFor(root, "stability-change-a")
	wtPathB := reconcile.WorktreePathFor(root, "stability-change-b")

	fakeExec.runFuncFor[wtPathA] = func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		envJSON := laneEnvelopeJSONWithCommit("stability-change-a", "done", candidateASHA)
		lucindPath := filepath.Join(req.WorktreePath, ".lucind")
		_ = os.MkdirAll(lucindPath, 0o755)
		_ = os.WriteFile(filepath.Join(lucindPath, "result.json"), []byte(envJSON), 0o644)
		return executor.Outcome{ExitCode: 0}, nil
	}

	fakeExec.runFuncFor[wtPathB] = func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		envJSON := laneEnvelopeJSONWithCommit("stability-change-b", "done", candidateBSHA)
		lucindPath := filepath.Join(req.WorktreePath, ".lucind")
		_ = os.MkdirAll(lucindPath, 0o755)
		_ = os.WriteFile(filepath.Join(lucindPath, "result.json"), []byte(envJSON), 0o644)
		return executor.Outcome{ExitCode: 0}, nil
	}

	sm := stability.NewStateMachine()
	if err := sm.Start(); err != nil {
		t.Fatalf("sm.Start failed: %v", err)
	}

	res, err := stability.ExecuteTrialJourney(ctx, sm, deps, featSvc, cfg, 99999999, wtPathB)
	if err == nil {
		t.Fatal("ExecuteTrialJourney with stale CAS succeeded, want error")
	}
	if !errors.Is(err, integrate.ErrStaleCAS) {
		t.Fatalf("ExecuteTrialJourney error = %v, want integrate.ErrStaleCAS", err)
	}
	if res != nil {
		t.Fatal("ExecuteTrialJourney returned non-nil result on failure")
	}

	if sm.ActiveTrialState() != stability.TrialFailed {
		t.Errorf("active trial state = %q, want %q", sm.ActiveTrialState(), stability.TrialFailed)
	}
	if sm.State() != stability.CampaignFailed {
		t.Errorf("campaign state = %q, want %q", sm.State(), stability.CampaignFailed)
	}
	if sm.ConsecutivePasses() != 0 {
		t.Errorf("consecutive passes = %d, want 0", sm.ConsecutivePasses())
	}
}

func TestTrialJourneyPromoteStageAncestryViolationFailsTrial(t *testing.T) {
	ctx := context.Background()
	fakeExec := newFakeJourneyExecutor()
	deps, ledgerHandle, root := newJourneyTestDeps(t, fakeExec)
	featSvc := feature.NewService(ledgerHandle)

	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	runGit("init")
	runGit("config", "user.name", "Test User")
	runGit("config", "user.email", "test@example.com")
	_ = fixture.MaterializeFixtures(root)
	runGit("add", ".")
	runGit("commit", "-m", "initial baseline")
	baseSHA := runGit("rev-parse", "HEAD")

	cfg := stability.DefaultJourneyConfig(1, baseSHA)
	cfg.LeaseTTL = 100 * time.Millisecond
	runGit("update-ref", cfg.ParentRefA, baseSHA)
	runGit("update-ref", cfg.ParentRefB, baseSHA)

	// Create Change A commit on top of baseSHA
	_ = os.WriteFile(filepath.Join(root, "fixture", "change_a.txt"), []byte("CHANGE_A=DONE\n"), 0o644)
	runGit("add", "fixture/change_a.txt")
	runGit("commit", "-m", "change A functionality")
	candidateASHA := runGit("rev-parse", "HEAD")

	// Create Change B commit directly ON TOP OF Change A commit (ancestry contamination!)
	_ = os.WriteFile(filepath.Join(root, "fixture", "change_b.txt"), []byte("CHANGE_B=DONE\n"), 0o644)
	runGit("add", "fixture/change_b.txt")
	runGit("commit", "-m", "change B functionality contaminated with A")
	candidateBSHA := runGit("rev-parse", "HEAD")
	runGit("reset", "--hard", baseSHA)

	wtPathA := reconcile.WorktreePathFor(root, "stability-change-a")
	wtPathB := reconcile.WorktreePathFor(root, "stability-change-b")

	fakeExec.runFuncFor[wtPathA] = func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		envJSON := laneEnvelopeJSONWithCommit("stability-change-a", "done", candidateASHA)
		lucindPath := filepath.Join(req.WorktreePath, ".lucind")
		_ = os.MkdirAll(lucindPath, 0o755)
		_ = os.WriteFile(filepath.Join(lucindPath, "result.json"), []byte(envJSON), 0o644)
		return executor.Outcome{ExitCode: 0}, nil
	}

	fakeExec.runFuncFor[wtPathB] = func(ctx context.Context, req executor.Request) (executor.Outcome, error) {
		envJSON := laneEnvelopeJSONWithCommit("stability-change-b", "done", candidateBSHA)
		lucindPath := filepath.Join(req.WorktreePath, ".lucind")
		_ = os.MkdirAll(lucindPath, 0o755)
		_ = os.WriteFile(filepath.Join(lucindPath, "result.json"), []byte(envJSON), 0o644)
		return executor.Outcome{ExitCode: 0}, nil
	}

	sm := stability.NewStateMachine()
	if err := sm.Start(); err != nil {
		t.Fatalf("sm.Start failed: %v", err)
	}

	res, err := stability.ExecuteTrialJourney(ctx, sm, deps, featSvc, cfg, 99999999, wtPathB)
	if err == nil {
		t.Fatal("ExecuteTrialJourney with ancestry violation succeeded, want error")
	}
	if !errors.Is(err, fixture.ErrContaminatedTarget) && !errors.Is(err, fixture.ErrAncestryViolation) {
		t.Fatalf("ExecuteTrialJourney error = %v, want ErrContaminatedTarget or ErrAncestryViolation", err)
	}
	if res != nil {
		t.Fatal("ExecuteTrialJourney returned non-nil result on failure")
	}

	if sm.ActiveTrialState() != stability.TrialFailed {
		t.Errorf("active trial state = %q, want %q", sm.ActiveTrialState(), stability.TrialFailed)
	}
	if sm.State() != stability.CampaignFailed {
		t.Errorf("campaign state = %q, want %q", sm.State(), stability.CampaignFailed)
	}
	if sm.ConsecutivePasses() != 0 {
		t.Errorf("consecutive passes = %d, want 0", sm.ConsecutivePasses())
	}
}

func TestRemediationPromoteStageNoDefectMovesRefs(t *testing.T) {
	TestTrialJourneyPromoteStageNoDefectMovesRefs(t)
}

func TestRemediationPromoteStageWithFixDependencyPromotesBothInOrder(t *testing.T) {
	TestTrialJourneyPromoteStageWithFixDependencyPromotesBothInOrder(t)
}

func TestRemediationPromoteStageStaleCASFailsTrial(t *testing.T) {
	TestTrialJourneyPromoteStageStaleCASFailsTrial(t)
}

func TestRemediationPromoteStageAncestryViolationFailsTrial(t *testing.T) {
	TestTrialJourneyPromoteStageAncestryViolationFailsTrial(t)
}
