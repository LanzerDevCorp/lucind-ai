package reconcile

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/overlap"
)

func openTestLedger(t *testing.T) *ledger.Ledger {
	t.Helper()
	root := t.TempDir()
	l, err := ledger.Open(context.Background(), root)
	if err != nil {
		t.Fatalf("ledger.Open: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

func sampleRequiredEvidence() *overlap.Evidence {
	now := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	ev := &overlap.Evidence{
		Version:     "v1",
		BaseSHA:     "base111111111111111111111111111111111111",
		FeatureASHA: "src222222222222222222222222222222222222",
		FeatureBSHA: "tgt333333333333333333333333333333333333",
		Class:       overlap.ClassRequired,
		Rationale:   []string{"predicted Git merge conflict detected by merge-tree"},
		Signals: overlap.Signals{
			PredictedConflict: true,
			ConflictPaths:     []string{"pkg/service.go"},
			SharedPaths:       []string{"pkg/service.go"},
		},
		ChangesA: []overlap.PathChange{
			{Path: "pkg/service.go", AddedLines: 10, DeletedLines: 2},
		},
		ChangesB: []overlap.PathChange{
			{Path: "pkg/service.go", AddedLines: 5, DeletedLines: 1},
		},
		Thresholds: overlap.DefaultThresholds(),
		CreatedAt:  now,
	}
	hash, _ := ev.ComputeHash()
	ev.Hash = hash
	return ev
}

// TestCreateRequestFromRequiredOverlapDisplaysExactFields proves Done Criterion 3:
// A request created from a required-overlap evidence record displays exactly source
// feature/parent, target feature/parent, the evidence snapshot, and the expected
// source and target SHAs at request time -- nothing else, and no aggregate/approve-all shape.
func TestCreateRequestFromRequiredOverlapDisplaysExactFields(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	svc := NewService(l, WithClock(func() time.Time { return now }))

	ev := sampleRequiredEvidence()

	params := CreateRequestParams{
		ID:            "req-rec-1",
		FeatureID:     "feature-target",
		SourceFeature: "feature-source",
		SourceParent:  "refs/heads/feature-source",
		TargetFeature: "feature-target",
		TargetParent:  "refs/heads/feature-target",
		SourceSHA:     ev.FeatureASHA,
		TargetSHA:     ev.FeatureBSHA,
		Evidence:      ev,
		TTL:           15 * time.Minute,
	}

	req, err := svc.CreateRequest(ctx, params)
	if err != nil {
		t.Fatalf("CreateRequest failed: %v", err)
	}

	// Verify exact fields returned on request
	if req.ID != "req-rec-1" {
		t.Errorf("req.ID = %q, want req-rec-1", req.ID)
	}
	if req.SourceFeature != "feature-source" {
		t.Errorf("req.SourceFeature = %q, want feature-source", req.SourceFeature)
	}
	if req.SourceParent != "refs/heads/feature-source" {
		t.Errorf("req.SourceParent = %q, want refs/heads/feature-source", req.SourceParent)
	}
	if req.TargetFeature != "feature-target" {
		t.Errorf("req.TargetFeature = %q, want feature-target", req.TargetFeature)
	}
	if req.TargetParent != "refs/heads/feature-target" {
		t.Errorf("req.TargetParent = %q, want refs/heads/feature-target", req.TargetParent)
	}
	if req.SourceSHA != ev.FeatureASHA {
		t.Errorf("req.SourceSHA = %q, want %q", req.SourceSHA, ev.FeatureASHA)
	}
	if req.TargetSHA != ev.FeatureBSHA {
		t.Errorf("req.TargetSHA = %q, want %q", req.TargetSHA, ev.FeatureBSHA)
	}
	if req.Status != RequestStatusAwaiting {
		t.Errorf("req.Status = %v, want %v", req.Status, RequestStatusAwaiting)
	}
	if req.Evidence == nil || req.Evidence.Hash != ev.Hash {
		t.Errorf("req.Evidence snapshot mismatch: %+v", req.Evidence)
	}
	if !req.ExpiresAt.Equal(now.Add(15 * time.Minute)) {
		t.Errorf("req.ExpiresAt = %v, want %v", req.ExpiresAt, now.Add(15*time.Minute))
	}

	// Verify durable retrieval through GetRequest returns exact same fields and evidence snapshot
	retrieved, err := svc.GetRequest(ctx, "req-rec-1")
	if err != nil {
		t.Fatalf("GetRequest failed: %v", err)
	}
	if retrieved.ID != req.ID ||
		retrieved.SourceFeature != req.SourceFeature ||
		retrieved.SourceParent != req.SourceParent ||
		retrieved.TargetFeature != req.TargetFeature ||
		retrieved.TargetParent != req.TargetParent ||
		retrieved.SourceSHA != req.SourceSHA ||
		retrieved.TargetSHA != req.TargetSHA ||
		retrieved.Status != req.Status ||
		retrieved.EvidenceHash != req.EvidenceHash ||
		retrieved.Evidence == nil ||
		retrieved.Evidence.Hash != ev.Hash {
		t.Fatalf("GetRequest returned unexpected state: %+v", retrieved)
	}

	// Audit trail verification
	events, err := l.IntegrationEvents(ctx, "feature-target")
	if err != nil {
		t.Fatalf("IntegrationEvents failed: %v", err)
	}
	if len(events) != 1 || events[0].Type != "reconciliation_request_created" {
		t.Fatalf("unexpected audit events: %+v", events)
	}
}

// TestCreateRequestRejectsNonRequiredOverlapEvidence proves that requests can only
// be created from required-overlap evidence.
func TestCreateRequestRejectsNonRequiredOverlapEvidence(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)
	svc := NewService(l)

	// 1. Warning class evidence must be rejected
	warnEv := sampleRequiredEvidence()
	warnEv.Class = overlap.ClassWarning

	_, err := svc.CreateRequest(ctx, CreateRequestParams{
		ID:            "req-warn",
		SourceFeature: "feat-a",
		TargetFeature: "feat-b",
		Evidence:      warnEv,
	})
	if !errors.Is(err, ErrRequiredOverlapOnly) {
		t.Errorf("CreateRequest with warning class error = %v, want ErrRequiredOverlapOnly", err)
	}

	// 2. Informational class evidence must be rejected
	infoEv := sampleRequiredEvidence()
	infoEv.Class = overlap.ClassInformational

	_, err = svc.CreateRequest(ctx, CreateRequestParams{
		ID:            "req-info",
		SourceFeature: "feat-a",
		TargetFeature: "feat-b",
		Evidence:      infoEv,
	})
	if !errors.Is(err, ErrRequiredOverlapOnly) {
		t.Errorf("CreateRequest with informational class error = %v, want ErrRequiredOverlapOnly", err)
	}

	// 3. Nil evidence must be rejected
	_, err = svc.CreateRequest(ctx, CreateRequestParams{
		ID:            "req-nil",
		SourceFeature: "feat-a",
		TargetFeature: "feat-b",
		Evidence:      nil,
	})
	if !errors.Is(err, ErrEvidenceNil) {
		t.Errorf("CreateRequest with nil evidence error = %v, want ErrEvidenceNil", err)
	}
}

// TestApproveBindsDirectionAndCreatesExactlyOneCandidate proves Done Criterion 4:
// Approving a source-to-target direction before expiry binds exactly that direction
// and the evidence snapshot present when approved, creating exactly one
// candidate_running candidate record (per design.md's "approval creates exactly one ... candidate"),
// not zero, not two.
func TestApproveBindsDirectionAndCreatesExactlyOneCandidate(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	now := time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC)
	svc := NewService(l, WithClock(func() time.Time { return now }))

	ev := sampleRequiredEvidence()
	req, err := svc.CreateRequest(ctx, CreateRequestParams{
		ID:            "req-app-1",
		FeatureID:     "feature-target",
		SourceFeature: "feature-source",
		SourceParent:  "refs/heads/feature-source",
		TargetFeature: "feature-target",
		TargetParent:  "refs/heads/feature-target",
		SourceSHA:     ev.FeatureASHA,
		TargetSHA:     ev.FeatureBSHA,
		Evidence:      ev,
		TTL:           10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("CreateRequest failed: %v", err)
	}

	appTime := now.Add(2 * time.Minute)
	svcApp := NewService(l, WithClock(func() time.Time { return appTime }))

	approvedReq, cand, err := svcApp.Approve(ctx, ApproveParams{
		RequestID:         req.ID,
		SourceFeature:     "feature-source",
		TargetFeature:     "feature-target",
		ExpectedSourceSHA: ev.FeatureASHA,
		ExpectedTargetSHA: ev.FeatureBSHA,
		Actor:             "local:testuser",
		AllowedPaths:      []string{"pkg/service.go"},
		CandidateID:       "cand-1",
	})
	if err != nil {
		t.Fatalf("Approve failed: %v", err)
	}

	// Verify approved request state
	if approvedReq.Status != RequestStatusApproved {
		t.Errorf("approvedReq.Status = %v, want %v", approvedReq.Status, RequestStatusApproved)
	}
	if approvedReq.Actor != "local:testuser" {
		t.Errorf("approvedReq.Actor = %q, want local:testuser", approvedReq.Actor)
	}
	if approvedReq.Direction != "feature-source -> feature-target" {
		t.Errorf("approvedReq.Direction = %q, want 'feature-source -> feature-target'", approvedReq.Direction)
	}
	if approvedReq.EvidenceHash != ev.Hash {
		t.Errorf("approvedReq.EvidenceHash = %q, want %q", approvedReq.EvidenceHash, ev.Hash)
	}

	// Verify candidate record
	if cand.ID != "cand-1" {
		t.Errorf("cand.ID = %q, want cand-1", cand.ID)
	}
	if cand.RequestID != req.ID {
		t.Errorf("cand.RequestID = %q, want %q", cand.RequestID, req.ID)
	}
	if cand.Status != CandidateStatusRunning {
		t.Errorf("cand.Status = %v, want %v", cand.Status, CandidateStatusRunning)
	}
	if cand.Model != "sonnet" {
		t.Errorf("cand.Model = %q, want sonnet", cand.Model)
	}
	if len(cand.AllowedPaths) != 1 || cand.AllowedPaths[0] != "pkg/service.go" {
		t.Errorf("cand.AllowedPaths = %v, want [pkg/service.go]", cand.AllowedPaths)
	}

	// Verify exactly ONE candidate exists in ledger (not zero, not two)
	candidates, err := l.ReconciliationCandidates(ctx, req.ID)
	if err != nil {
		t.Fatalf("ReconciliationCandidates: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected exactly 1 candidate row, got %d", len(candidates))
	}
	if candidates[0].ID != "cand-1" || candidates[0].Status != string(CandidateStatusRunning) {
		t.Fatalf("candidate row in ledger mismatch: %+v", candidates[0])
	}

	// Attempting to approve an already-approved request must fail
	_, _, err = svcApp.Approve(ctx, ApproveParams{
		RequestID:         req.ID,
		SourceFeature:     "feature-source",
		TargetFeature:     "feature-target",
		ExpectedSourceSHA: ev.FeatureASHA,
		ExpectedTargetSHA: ev.FeatureBSHA,
		Actor:             "local:testuser",
	})
	if !errors.Is(err, ErrRequestNotAwaiting) {
		t.Errorf("second Approve error = %v, want ErrRequestNotAwaiting", err)
	}

	// Audit trail verification
	events, err := l.IntegrationEvents(ctx, "feature-target")
	if err != nil {
		t.Fatalf("IntegrationEvents: %v", err)
	}
	var approvedEvents []ledger.IntegrationEvent
	for _, e := range events {
		if e.Type == "reconciliation_approved" {
			approvedEvents = append(approvedEvents, e)
		}
	}
	if len(approvedEvents) != 1 {
		t.Fatalf("expected 1 reconciliation_approved audit event, got %d", len(approvedEvents))
	}
}

// TestDeclineTransitionsToTerminalStateWithoutCandidate proves Done Criterion 5 (decline):
// Decline transitions the request to 'declined' state without creating a candidate.
func TestDeclineTransitionsToTerminalStateWithoutCandidate(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	now := time.Date(2026, 8, 20, 15, 0, 0, 0, time.UTC)
	svc := NewService(l, WithClock(func() time.Time { return now }))

	ev := sampleRequiredEvidence()
	req, err := svc.CreateRequest(ctx, CreateRequestParams{
		ID:            "req-dec-1",
		FeatureID:     "feature-target",
		SourceFeature: "feature-source",
		SourceParent:  "refs/heads/feature-source",
		TargetFeature: "feature-target",
		TargetParent:  "refs/heads/feature-target",
		SourceSHA:     ev.FeatureASHA,
		TargetSHA:     ev.FeatureBSHA,
		Evidence:      ev,
	})
	if err != nil {
		t.Fatalf("CreateRequest failed: %v", err)
	}

	decTime := now.Add(1 * time.Minute)
	svcDec := NewService(l, WithClock(func() time.Time { return decTime }))

	declinedReq, err := svcDec.Decline(ctx, req.ID, "local:reviewer", "direction deferred for manual review")
	if err != nil {
		t.Fatalf("Decline failed: %v", err)
	}

	if declinedReq.Status != RequestStatusDeclined {
		t.Errorf("declinedReq.Status = %v, want %v", declinedReq.Status, RequestStatusDeclined)
	}
	if declinedReq.Actor != "local:reviewer" {
		t.Errorf("declinedReq.Actor = %q, want local:reviewer", declinedReq.Actor)
	}

	// Verify NO candidate row was created
	candidates, err := l.ReconciliationCandidates(ctx, req.ID)
	if err != nil {
		t.Fatalf("ReconciliationCandidates: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected 0 candidate rows after decline, got %d", len(candidates))
	}

	// Late approve or second decline attempt must fail
	_, _, err = svcDec.Approve(ctx, ApproveParams{
		RequestID:         req.ID,
		SourceFeature:     "feature-source",
		TargetFeature:     "feature-target",
		ExpectedSourceSHA: ev.FeatureASHA,
		ExpectedTargetSHA: ev.FeatureBSHA,
		Actor:             "local:reviewer",
	})
	if !errors.Is(err, ErrRequestNotAwaiting) {
		t.Errorf("Approve after decline error = %v, want ErrRequestNotAwaiting", err)
	}

	// Audit trail verification
	events, err := l.IntegrationEvents(ctx, "feature-target")
	if err != nil {
		t.Fatalf("IntegrationEvents: %v", err)
	}
	var decEvents []ledger.IntegrationEvent
	for _, e := range events {
		if e.Type == "reconciliation_declined" {
			decEvents = append(decEvents, e)
		}
	}
	if len(decEvents) != 1 {
		t.Fatalf("expected 1 reconciliation_declined audit event, got %d", len(decEvents))
	}
}

// TestCancelTransitionsToTerminalStateWithoutCandidate proves Done Criterion 5 (cancel):
// Cancellation transitions the request to 'cancelled' state without creating a candidate.
func TestCancelTransitionsToTerminalStateWithoutCandidate(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	now := time.Date(2026, 8, 20, 16, 0, 0, 0, time.UTC)
	svc := NewService(l, WithClock(func() time.Time { return now }))

	ev := sampleRequiredEvidence()
	req, err := svc.CreateRequest(ctx, CreateRequestParams{
		ID:            "req-canc-1",
		FeatureID:     "feature-target",
		SourceFeature: "feature-source",
		SourceParent:  "refs/heads/feature-source",
		TargetFeature: "feature-target",
		TargetParent:  "refs/heads/feature-target",
		SourceSHA:     ev.FeatureASHA,
		TargetSHA:     ev.FeatureBSHA,
		Evidence:      ev,
	})
	if err != nil {
		t.Fatalf("CreateRequest failed: %v", err)
	}

	cancTime := now.Add(1 * time.Minute)
	svcCanc := NewService(l, WithClock(func() time.Time { return cancTime }))

	cancelledReq, err := svcCanc.Cancel(ctx, req.ID, "local:operator", "superseded by feature abort")
	if err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	if cancelledReq.Status != RequestStatusCancelled {
		t.Errorf("cancelledReq.Status = %v, want %v", cancelledReq.Status, RequestStatusCancelled)
	}
	if cancelledReq.Actor != "local:operator" {
		t.Errorf("cancelledReq.Actor = %q, want local:operator", cancelledReq.Actor)
	}

	// Verify NO candidate row was created
	candidates, err := l.ReconciliationCandidates(ctx, req.ID)
	if err != nil {
		t.Fatalf("ReconciliationCandidates: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected 0 candidate rows after cancel, got %d", len(candidates))
	}

	// Late approve or second cancel attempt must fail
	_, _, err = svcCanc.Approve(ctx, ApproveParams{
		RequestID:         req.ID,
		SourceFeature:     "feature-source",
		TargetFeature:     "feature-target",
		ExpectedSourceSHA: ev.FeatureASHA,
		ExpectedTargetSHA: ev.FeatureBSHA,
		Actor:             "local:operator",
	})
	if !errors.Is(err, ErrRequestNotAwaiting) {
		t.Errorf("Approve after cancel error = %v, want ErrRequestNotAwaiting", err)
	}

	// Audit trail verification
	events, err := l.IntegrationEvents(ctx, "feature-target")
	if err != nil {
		t.Fatalf("IntegrationEvents: %v", err)
	}
	var cancEvents []ledger.IntegrationEvent
	for _, e := range events {
		if e.Type == "reconciliation_cancelled" {
			cancEvents = append(cancEvents, e)
		}
	}
	if len(cancEvents) != 1 {
		t.Fatalf("expected 1 reconciliation_cancelled audit event, got %d", len(cancEvents))
	}
}

// TestApproveRejectsExpiredRequest proves Done Criterion 6 (expiry):
// An expired request rejects a late approval attempt -- "old authority MUST be rejected".
func TestApproveRejectsExpiredRequest(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	now := time.Date(2026, 8, 20, 17, 0, 0, 0, time.UTC)
	svc := NewService(l, WithClock(func() time.Time { return now }))

	ev := sampleRequiredEvidence()
	req, err := svc.CreateRequest(ctx, CreateRequestParams{
		ID:            "req-exp-1",
		FeatureID:     "feature-target",
		SourceFeature: "feature-source",
		SourceParent:  "refs/heads/feature-source",
		TargetFeature: "feature-target",
		TargetParent:  "refs/heads/feature-target",
		SourceSHA:     ev.FeatureASHA,
		TargetSHA:     ev.FeatureBSHA,
		Evidence:      ev,
		TTL:           5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("CreateRequest failed: %v", err)
	}

	// Advance time past expiry (6 minutes later)
	lateTime := now.Add(6 * time.Minute)
	svcLate := NewService(l, WithClock(func() time.Time { return lateTime }))

	_, _, err = svcLate.Approve(ctx, ApproveParams{
		RequestID:         req.ID,
		SourceFeature:     "feature-source",
		TargetFeature:     "feature-target",
		ExpectedSourceSHA: ev.FeatureASHA,
		ExpectedTargetSHA: ev.FeatureBSHA,
		Actor:             "local:testuser",
	})
	if !errors.Is(err, ErrRequestExpired) {
		t.Errorf("Approve on expired request error = %v, want ErrRequestExpired", err)
	}

	// Verify no candidate row was created
	candidates, err := l.ReconciliationCandidates(ctx, req.ID)
	if err != nil {
		t.Fatalf("ReconciliationCandidates: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected 0 candidate rows, got %d", len(candidates))
	}

	// Verify GetRequest reflects expired status
	lateReq, err := svcLate.GetRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetRequest: %v", err)
	}
	if lateReq.Status != RequestStatusExpired {
		t.Errorf("lateReq.Status = %v, want %v", lateReq.Status, RequestStatusExpired)
	}
}

// TestApproveRejectsStaleExpectedSHAs proves Done Criterion 6 (stale SHAs):
// A request whose expected source/target SHA has since changed rejects an approval attempt.
func TestApproveRejectsStaleExpectedSHAs(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	now := time.Date(2026, 8, 20, 18, 0, 0, 0, time.UTC)
	svc := NewService(l, WithClock(func() time.Time { return now }))

	ev := sampleRequiredEvidence()
	req, err := svc.CreateRequest(ctx, CreateRequestParams{
		ID:            "req-sha-1",
		FeatureID:     "feature-target",
		SourceFeature: "feature-source",
		SourceParent:  "refs/heads/feature-source",
		TargetFeature: "feature-target",
		TargetParent:  "refs/heads/feature-target",
		SourceSHA:     ev.FeatureASHA,
		TargetSHA:     ev.FeatureBSHA,
		Evidence:      ev,
		TTL:           15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("CreateRequest failed: %v", err)
	}

	// 1. Mismatched Source SHA
	_, _, err = svc.Approve(ctx, ApproveParams{
		RequestID:         req.ID,
		SourceFeature:     "feature-source",
		TargetFeature:     "feature-target",
		ExpectedSourceSHA: "stale_source_sha_999999999999999999999999",
		ExpectedTargetSHA: ev.FeatureBSHA,
		Actor:             "local:testuser",
	})
	if !errors.Is(err, ErrStaleExpectedSHA) {
		t.Errorf("Approve with mismatched source SHA error = %v, want ErrStaleExpectedSHA", err)
	}

	// 2. Mismatched Target SHA
	_, _, err = svc.Approve(ctx, ApproveParams{
		RequestID:         req.ID,
		SourceFeature:     "feature-source",
		TargetFeature:     "feature-target",
		ExpectedSourceSHA: ev.FeatureASHA,
		ExpectedTargetSHA: "stale_target_sha_888888888888888888888888",
		Actor:             "local:testuser",
	})
	if !errors.Is(err, ErrStaleExpectedSHA) {
		t.Errorf("Approve with mismatched target SHA error = %v, want ErrStaleExpectedSHA", err)
	}

	// Verify no candidate row was created
	candidates, err := l.ReconciliationCandidates(ctx, req.ID)
	if err != nil {
		t.Fatalf("ReconciliationCandidates: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected 0 candidate rows, got %d", len(candidates))
	}
}

// TestApproveRejectsInvalidDirection proves that approving an un-offered direction is rejected.
func TestApproveRejectsInvalidDirection(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	now := time.Date(2026, 8, 20, 19, 0, 0, 0, time.UTC)
	svc := NewService(l, WithClock(func() time.Time { return now }))

	ev := sampleRequiredEvidence()
	req, err := svc.CreateRequest(ctx, CreateRequestParams{
		ID:            "req-dir-1",
		FeatureID:     "feature-target",
		SourceFeature: "feature-source",
		SourceParent:  "refs/heads/feature-source",
		TargetFeature: "feature-target",
		TargetParent:  "refs/heads/feature-target",
		SourceSHA:     ev.FeatureASHA,
		TargetSHA:     ev.FeatureBSHA,
		Evidence:      ev,
	})
	if err != nil {
		t.Fatalf("CreateRequest failed: %v", err)
	}

	// Attempt to approve wrong direction (e.g. target -> source or third feature)
	_, _, err = svc.Approve(ctx, ApproveParams{
		RequestID:         req.ID,
		SourceFeature:     "feature-target",
		TargetFeature:     "feature-source",
		ExpectedSourceSHA: ev.FeatureASHA,
		ExpectedTargetSHA: ev.FeatureBSHA,
		Actor:             "local:testuser",
	})
	if !errors.Is(err, ErrInvalidDirection) {
		t.Errorf("Approve with reversed direction error = %v, want ErrInvalidDirection", err)
	}

	// Verify no candidate was created
	candidates, err := l.ReconciliationCandidates(ctx, req.ID)
	if err != nil {
		t.Fatalf("ReconciliationCandidates: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected 0 candidates, got %d", len(candidates))
	}
}

// TestRenewRecomputesFreshEvidenceViaOverlap proves Done Criterion 7:
// Renewal of an expired/stale request recomputes evidence via internal/overlap rather than
// reusing the stale snapshot -- asserting the renewed request's evidence differs from (or is
// freshly recomputed against) the original, i.e. renewal genuinely calls the classification
// function again rather than copying the old row.
func TestRenewRecomputesFreshEvidenceViaOverlap(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	now := time.Date(2026, 8, 20, 20, 0, 0, 0, time.UTC)
	evalCalls := 0

	ev1 := sampleRequiredEvidence()
	ev1.FeatureASHA = "sha_old_source_11111111111111111111111"
	ev1.FeatureBSHA = "sha_old_target_22222222222222222222222"
	ev1.Hash, _ = ev1.ComputeHash()

	ev2 := sampleRequiredEvidence()
	ev2.FeatureASHA = "sha_fresh_source_3333333333333333333333"
	ev2.FeatureBSHA = "sha_fresh_target_4444444444444444444444"
	ev2.Signals.ConflictPaths = []string{"pkg/service.go", "pkg/new_conflict.go"}
	ev2.Rationale = []string{"predicted Git merge conflict detected by merge-tree in pkg/service.go, pkg/new_conflict.go"}
	ev2.Hash, _ = ev2.ComputeHash()

	fakeEvaluator := func(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...overlap.EvaluateOption) (*overlap.Evidence, error) {
		evalCalls++
		if shaA == "sha_fresh_source_3333333333333333333333" && shaB == "sha_fresh_target_4444444444444444444444" {
			return ev2, nil
		}
		return ev1, nil
	}

	svc := NewService(l, WithClock(func() time.Time { return now }), WithOverlapEvaluator(fakeEvaluator))

	// Create initial request with ev1
	req1, err := svc.CreateRequest(ctx, CreateRequestParams{
		ID:            "req-initial-1",
		FeatureID:     "feature-target",
		SourceFeature: "feature-source",
		SourceParent:  "refs/heads/feature-source",
		TargetFeature: "feature-target",
		TargetParent:  "refs/heads/feature-target",
		SourceSHA:     ev1.FeatureASHA,
		TargetSHA:     ev1.FeatureBSHA,
		Evidence:      ev1,
		TTL:           5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}

	// Advance time past expiry
	renewTime := now.Add(10 * time.Minute)
	svcRenew := NewService(l, WithClock(func() time.Time { return renewTime }), WithOverlapEvaluator(fakeEvaluator))

	renewParams := RenewParams{
		OldRequestID:     req1.ID,
		RepoDir:          t.TempDir(),
		BaseSHA:          ev1.BaseSHA,
		CurrentSourceSHA: "sha_fresh_source_3333333333333333333333",
		CurrentTargetSHA: "sha_fresh_target_4444444444444444444444",
		TTL:              15 * time.Minute,
		NewRequestID:     "req-renewed-2",
	}

	renewedReq, err := svcRenew.Renew(ctx, renewParams)
	if err != nil {
		t.Fatalf("Renew failed: %v", err)
	}

	// Assert the overlap evaluator was genuinely invoked
	if evalCalls != 1 {
		t.Errorf("overlap evaluator called %d times, want 1", evalCalls)
	}

	// Assert renewed request has fresh evidence and new SHAs
	if renewedReq.ID != "req-renewed-2" {
		t.Errorf("renewedReq.ID = %q, want req-renewed-2", renewedReq.ID)
	}
	if renewedReq.SourceSHA != "sha_fresh_source_3333333333333333333333" {
		t.Errorf("renewedReq.SourceSHA = %q, want sha_fresh_source_3333333333333333333333", renewedReq.SourceSHA)
	}
	if renewedReq.TargetSHA != "sha_fresh_target_4444444444444444444444" {
		t.Errorf("renewedReq.TargetSHA = %q, want sha_fresh_target_4444444444444444444444", renewedReq.TargetSHA)
	}
	if renewedReq.EvidenceHash == req1.EvidenceHash {
		t.Errorf("renewedReq.EvidenceHash (%s) must differ from old request (%s)", renewedReq.EvidenceHash, req1.EvidenceHash)
	}
	if renewedReq.Evidence == nil || renewedReq.Evidence.Hash != ev2.Hash {
		t.Errorf("renewedReq.Evidence mismatch: %+v", renewedReq.Evidence)
	}
	if renewedReq.Status != RequestStatusAwaiting {
		t.Errorf("renewedReq.Status = %v, want %v", renewedReq.Status, RequestStatusAwaiting)
	}
	if !renewedReq.ExpiresAt.Equal(renewTime.Add(15 * time.Minute)) {
		t.Errorf("renewedReq.ExpiresAt = %v, want %v", renewedReq.ExpiresAt, renewTime.Add(15*time.Minute))
	}

	// Verify old request in DB is marked expired
	oldRow, err := l.ReconciliationRequest(ctx, req1.ID)
	if err != nil {
		t.Fatalf("ReconciliationRequest(oldReq): %v", err)
	}
	if oldRow.Status != string(RequestStatusExpired) {
		t.Errorf("oldRow.Status in DB = %q, want expired", oldRow.Status)
	}

	// Audit trail verification
	events, err := l.IntegrationEvents(ctx, "feature-target")
	if err != nil {
		t.Fatalf("IntegrationEvents: %v", err)
	}
	var renewEvents []ledger.IntegrationEvent
	for _, e := range events {
		if e.Type == "reconciliation_renewed" {
			renewEvents = append(renewEvents, e)
		}
	}
	if len(renewEvents) != 1 {
		t.Fatalf("expected 1 reconciliation_renewed audit event, got %d", len(renewEvents))
	}
}

// TestCandidateLifecycleTransitionsAndQueries proves candidate state transitions:
// candidate_running -> integrated | failed | stale with atomic audit logging.
func TestCandidateLifecycleTransitionsAndQueries(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	now := time.Date(2026, 8, 20, 21, 0, 0, 0, time.UTC)
	svc := NewService(l, WithClock(func() time.Time { return now }))

	ev := sampleRequiredEvidence()
	req, err := svc.CreateRequest(ctx, CreateRequestParams{
		ID:            "req-cand-test",
		FeatureID:     "feature-target",
		SourceFeature: "feature-source",
		SourceParent:  "refs/heads/feature-source",
		TargetFeature: "feature-target",
		TargetParent:  "refs/heads/feature-target",
		SourceSHA:     ev.FeatureASHA,
		TargetSHA:     ev.FeatureBSHA,
		Evidence:      ev,
	})
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}

	approvedReq, cand, err := svc.Approve(ctx, ApproveParams{
		RequestID:         req.ID,
		SourceFeature:     "feature-source",
		TargetFeature:     "feature-target",
		ExpectedSourceSHA: ev.FeatureASHA,
		ExpectedTargetSHA: ev.FeatureBSHA,
		Actor:             "local:testuser",
		AllowedPaths:      []string{"pkg/service.go"},
		CandidateID:       "cand-life-1",
	})
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if approvedReq.Status != RequestStatusApproved || cand.ID != "cand-life-1" || cand.Status != CandidateStatusRunning {
		t.Fatalf("Approve returned unexpected request/candidate: req=%+v, cand=%+v", approvedReq, cand)
	}

	// 1. Query candidate by ID
	gotCand, err := svc.GetCandidate(ctx, "cand-life-1")
	if err != nil {
		t.Fatalf("GetCandidate: %v", err)
	}
	if gotCand.ID != "cand-life-1" || gotCand.Status != CandidateStatusRunning {
		t.Errorf("gotCand = %+v, want cand-life-1 with status candidate_running", gotCand)
	}

	// 2. Query candidate by RequestID
	gotByReq, err := svc.GetCandidateByRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetCandidateByRequest: %v", err)
	}
	if gotByReq.ID != "cand-life-1" {
		t.Errorf("gotByReq.ID = %q, want cand-life-1", gotByReq.ID)
	}

	// 3. Query candidates list by RequestID
	list, err := svc.CandidatesByRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("CandidatesByRequest: %v", err)
	}
	if len(list) != 1 || list[0].ID != "cand-life-1" {
		t.Fatalf("CandidatesByRequest returned %+v, want 1 item", list)
	}

	// 4. Transition candidate_running -> integrated
	updTime := now.Add(5 * time.Minute)
	svcUpd := NewService(l, WithClock(func() time.Time { return updTime }))

	integratedCand, err := svcUpd.UpdateCandidateStatus(ctx, "cand-life-1", CandidateStatusIntegrated, "sha_cand_promoted", "")
	if err != nil {
		t.Fatalf("UpdateCandidateStatus(integrated): %v", err)
	}
	if integratedCand.Status != CandidateStatusIntegrated {
		t.Errorf("integratedCand.Status = %v, want %v", integratedCand.Status, CandidateStatusIntegrated)
	}
	if integratedCand.CandidateSHA != "sha_cand_promoted" {
		t.Errorf("integratedCand.CandidateSHA = %q, want sha_cand_promoted", integratedCand.CandidateSHA)
	}

	// Verify persistence in ledger
	candFromDB, err := svcUpd.GetCandidate(ctx, "cand-life-1")
	if err != nil {
		t.Fatalf("GetCandidate after update: %v", err)
	}
	if candFromDB.Status != CandidateStatusIntegrated || candFromDB.CandidateSHA != "sha_cand_promoted" {
		t.Fatalf("candFromDB = %+v, want integrated with sha", candFromDB)
	}

	// Audit trail verification
	events, err := l.IntegrationEvents(ctx, "feature-target")
	if err != nil {
		t.Fatalf("IntegrationEvents: %v", err)
	}
	var updEvents []ledger.IntegrationEvent
	for _, e := range events {
		if e.Type == "reconciliation_candidate_updated" {
			updEvents = append(updEvents, e)
		}
	}
	if len(updEvents) != 1 {
		t.Fatalf("expected 1 reconciliation_candidate_updated audit event, got %d", len(updEvents))
	}
}

// TestCandidateInvalidStatusTransitions proves invalid transitions from terminal candidate states are rejected.
func TestCandidateInvalidStatusTransitions(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	svc := NewService(l)
	ev := sampleRequiredEvidence()

	// Test Running -> Failed
	req1, _ := svc.CreateRequest(ctx, CreateRequestParams{
		ID:            "req-fail",
		SourceFeature: "feat-a",
		TargetFeature: "feat-b",
		Evidence:      ev,
	})
	_, _, _ = svc.Approve(ctx, ApproveParams{
		RequestID:     req1.ID,
		SourceFeature: "feat-a",
		TargetFeature: "feat-b",
		Actor:         "local:user",
		CandidateID:   "cand-fail",
	})
	failedCand, err := svc.UpdateCandidateStatus(ctx, "cand-fail", CandidateStatusFailed, "", "checks failed")
	if err != nil {
		t.Fatalf("UpdateCandidateStatus(failed): %v", err)
	}
	if failedCand.Status != CandidateStatusFailed || failedCand.FailureReason != "checks failed" {
		t.Errorf("failedCand mismatch: %+v", failedCand)
	}

	// Terminal state cannot transition to running or integrated
	_, err = svc.UpdateCandidateStatus(ctx, "cand-fail", CandidateStatusRunning, "", "")
	if !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("transition from failed to running error = %v, want ErrInvalidTransition", err)
	}
	_, err = svc.UpdateCandidateStatus(ctx, "cand-fail", CandidateStatusIntegrated, "sha", "")
	if !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("transition from failed to integrated error = %v, want ErrInvalidTransition", err)
	}

	// Test Running -> Stale
	req2, _ := svc.CreateRequest(ctx, CreateRequestParams{
		ID:            "req-stale",
		SourceFeature: "feat-a",
		TargetFeature: "feat-b",
		Evidence:      ev,
	})
	_, _, _ = svc.Approve(ctx, ApproveParams{
		RequestID:     req2.ID,
		SourceFeature: "feat-a",
		TargetFeature: "feat-b",
		Actor:         "local:user",
		CandidateID:   "cand-stale",
	})
	staleCand, err := svc.UpdateCandidateStatus(ctx, "cand-stale", CandidateStatusStale, "", "parent ref advanced concurrently")
	if err != nil {
		t.Fatalf("UpdateCandidateStatus(stale): %v", err)
	}
	if staleCand.Status != CandidateStatusStale {
		t.Errorf("staleCand mismatch: %+v", staleCand)
	}

	// Terminal state stale cannot transition to integrated
	_, err = svc.UpdateCandidateStatus(ctx, "cand-stale", CandidateStatusIntegrated, "sha", "")
	if !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("transition from stale to integrated error = %v, want ErrInvalidTransition", err)
	}
}

func TestRequestStatusValid(t *testing.T) {
	tests := []struct {
		status RequestStatus
		valid  bool
	}{
		{RequestStatusAwaiting, true},
		{RequestStatusApproved, true},
		{RequestStatusDeclined, true},
		{RequestStatusCancelled, true},
		{RequestStatusExpired, true},
		{RequestStatus("invalid"), false},
		{RequestStatus(""), false},
	}
	for _, tt := range tests {
		if got := tt.status.Valid(); got != tt.valid {
			t.Errorf("RequestStatus(%q).Valid() = %v, want %v", tt.status, got, tt.valid)
		}
	}
}

func TestCandidateStatusValid(t *testing.T) {
	tests := []struct {
		status CandidateStatus
		valid  bool
	}{
		{CandidateStatusRunning, true},
		{CandidateStatusIntegrated, true},
		{CandidateStatusFailed, true},
		{CandidateStatusStale, true},
		{CandidateStatus("invalid"), false},
		{CandidateStatus(""), false},
	}
	for _, tt := range tests {
		if got := tt.status.Valid(); got != tt.valid {
			t.Errorf("CandidateStatus(%q).Valid() = %v, want %v", tt.status, got, tt.valid)
		}
	}
}

func TestFormatAndParseDirection(t *testing.T) {
	// With parent refs
	dir := FormatDirection("feat-a", "refs/heads/feat-a", "feat-b", "refs/heads/feat-b")
	want := "feat-a (refs/heads/feat-a) -> feat-b (refs/heads/feat-b)"
	if dir != want {
		t.Errorf("FormatDirection = %q, want %q", dir, want)
	}
	srcFeat, srcParent, tgtFeat, tgtParent := ParseDirection(dir)
	if srcFeat != "feat-a" || srcParent != "refs/heads/feat-a" || tgtFeat != "feat-b" || tgtParent != "refs/heads/feat-b" {
		t.Errorf("ParseDirection(%q) = (%q, %q, %q, %q)", dir, srcFeat, srcParent, tgtFeat, tgtParent)
	}

	// Without parent refs
	dirSimple := FormatDirection("feat-a", "", "feat-b", "")
	wantSimple := "feat-a -> feat-b"
	if dirSimple != wantSimple {
		t.Errorf("FormatDirection = %q, want %q", dirSimple, wantSimple)
	}
	srcFeat, srcParent, tgtFeat, tgtParent = ParseDirection(dirSimple)
	if srcFeat != "feat-a" || srcParent != "" || tgtFeat != "feat-b" || tgtParent != "" {
		t.Errorf("ParseDirection(%q) = (%q, %q, %q, %q)", dirSimple, srcFeat, srcParent, tgtFeat, tgtParent)
	}
}

func TestWithIDSourceCustomGenerator(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	counter := 0
	customIDGen := func() string {
		counter++
		return "custom-id-" + string(rune('0'+counter))
	}

	svc := NewService(l, WithIDSource(customIDGen))
	ev := sampleRequiredEvidence()

	req, err := svc.CreateRequest(ctx, CreateRequestParams{
		SourceFeature: "feat-a",
		TargetFeature: "feat-b",
		Evidence:      ev,
	})
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}
	if req.ID != "custom-id-1" {
		t.Errorf("req.ID = %q, want custom-id-1", req.ID)
	}

	_, cand, err := svc.Approve(ctx, ApproveParams{
		RequestID:     req.ID,
		SourceFeature: "feat-a",
		TargetFeature: "feat-b",
		Actor:         "local:test",
	})
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if cand.ID != "custom-id-2" {
		t.Errorf("cand.ID = %q, want custom-id-2", cand.ID)
	}
}

func TestCreateRequestValidatesRequiredFeatureIDs(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)
	svc := NewService(l)
	ev := sampleRequiredEvidence()

	// Missing SourceFeature
	_, err := svc.CreateRequest(ctx, CreateRequestParams{
		SourceFeature: "",
		TargetFeature: "feat-b",
		Evidence:      ev,
	})
	if !errors.Is(err, ErrMissingFeature) {
		t.Errorf("CreateRequest missing source error = %v, want ErrMissingFeature", err)
	}

	// Missing TargetFeature
	_, err = svc.CreateRequest(ctx, CreateRequestParams{
		SourceFeature: "feat-a",
		TargetFeature: "",
		Evidence:      ev,
	})
	if !errors.Is(err, ErrMissingFeature) {
		t.Errorf("CreateRequest missing target error = %v, want ErrMissingFeature", err)
	}
}

func TestApproveDeclineCancelRejectEmptyActor(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)
	svc := NewService(l)
	ev := sampleRequiredEvidence()

	req, err := svc.CreateRequest(ctx, CreateRequestParams{
		ID:            "req-empty-actor",
		SourceFeature: "feat-a",
		TargetFeature: "feat-b",
		Evidence:      ev,
	})
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}

	// Approve with empty actor
	_, _, err = svc.Approve(ctx, ApproveParams{
		RequestID:     req.ID,
		SourceFeature: "feat-a",
		TargetFeature: "feat-b",
		Actor:         "   ",
	})
	if !errors.Is(err, ErrEmptyActor) {
		t.Errorf("Approve with empty actor error = %v, want ErrEmptyActor", err)
	}

	// Decline with empty actor
	_, err = svc.Decline(ctx, req.ID, "   ", "reason")
	if !errors.Is(err, ErrEmptyActor) {
		t.Errorf("Decline with empty actor error = %v, want ErrEmptyActor", err)
	}

	// Cancel with empty actor
	_, err = svc.Cancel(ctx, req.ID, "   ", "reason")
	if !errors.Is(err, ErrEmptyActor) {
		t.Errorf("Cancel with empty actor error = %v, want ErrEmptyActor", err)
	}
}

func TestApproveCandidateAlreadyExists(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)
	svc := NewService(l)
	ev := sampleRequiredEvidence()

	req, _ := svc.CreateRequest(ctx, CreateRequestParams{
		ID:            "req-dup-cand",
		SourceFeature: "feat-a",
		TargetFeature: "feat-b",
		Evidence:      ev,
	})

	// Pre-insert a candidate row in ledger for this request to simulate candidate race
	_ = l.InsertReconciliationCandidate(ctx, ledger.ReconciliationCandidateRow{
		ID:        "cand-existing",
		RequestID: req.ID,
		Status:    string(CandidateStatusRunning),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})

	_, _, err := svc.Approve(ctx, ApproveParams{
		RequestID:     req.ID,
		SourceFeature: "feat-a",
		TargetFeature: "feat-b",
		Actor:         "local:user",
	})
	if !errors.Is(err, ErrCandidateAlreadyExists) {
		t.Errorf("Approve when candidate exists error = %v, want ErrCandidateAlreadyExists", err)
	}
}

func TestRenewRejectsNonRequiredFreshEvidence(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	ev1 := sampleRequiredEvidence()
	svc := NewService(l)

	req1, _ := svc.CreateRequest(ctx, CreateRequestParams{
		ID:            "req-renew-info",
		SourceFeature: "feat-a",
		TargetFeature: "feat-b",
		Evidence:      ev1,
	})

	// Evaluator returns informational evidence (conflict resolved)
	infoEv := sampleRequiredEvidence()
	infoEv.Class = overlap.ClassInformational
	infoEv.Hash, _ = infoEv.ComputeHash()

	svcInfo := NewService(l, WithOverlapEvaluator(func(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...overlap.EvaluateOption) (*overlap.Evidence, error) {
		return infoEv, nil
	}))

	_, err := svcInfo.Renew(ctx, RenewParams{
		OldRequestID: req1.ID,
	})
	if !errors.Is(err, ErrRequiredOverlapOnly) {
		t.Errorf("Renew with informational fresh evidence error = %v, want ErrRequiredOverlapOnly", err)
	}
}

func TestGetRequestAndCandidateNotFound(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)
	svc := NewService(l)

	// GetRequest nonexistent
	_, err := svc.GetRequest(ctx, "nonexistent-req")
	if !errors.Is(err, ErrRequestNotFound) {
		t.Errorf("GetRequest(nonexistent) error = %v, want ErrRequestNotFound", err)
	}

	// GetCandidate nonexistent
	_, err = svc.GetCandidate(ctx, "nonexistent-cand")
	if !errors.Is(err, ErrCandidateNotFound) {
		t.Errorf("GetCandidate(nonexistent) error = %v, want ErrCandidateNotFound", err)
	}

	// GetCandidateByRequest nonexistent
	_, err = svc.GetCandidateByRequest(ctx, "nonexistent-req")
	if !errors.Is(err, ErrCandidateNotFound) {
		t.Errorf("GetCandidateByRequest(nonexistent) error = %v, want ErrCandidateNotFound", err)
	}

	// Decline / Cancel nonexistent
	_, err = svc.Decline(ctx, "nonexistent-req", "local:user", "reason")
	if !errors.Is(err, ErrRequestNotFound) {
		t.Errorf("Decline(nonexistent) error = %v, want ErrRequestNotFound", err)
	}
	_, err = svc.Cancel(ctx, "nonexistent-req", "local:user", "reason")
	if !errors.Is(err, ErrRequestNotFound) {
		t.Errorf("Cancel(nonexistent) error = %v, want ErrRequestNotFound", err)
	}
}

// TestUpdateCandidateOutputOnly proves triage JSON lands in Candidate.Output
// without changing status or CandidateSHA. UpdateCandidateStatus SQL does not
// touch output, so this path must go through a dedicated output-only update.
func TestUpdateCandidateOutputOnly(t *testing.T) {
	ctx := context.Background()
	l := openTestLedger(t)

	now := time.Date(2026, 8, 23, 18, 0, 0, 0, time.UTC)
	svc := NewService(l, WithClock(func() time.Time { return now }))

	ev := sampleRequiredEvidence()
	req, err := svc.CreateRequest(ctx, CreateRequestParams{
		ID:            "req-output-only",
		FeatureID:     "feature-target",
		SourceFeature: "feature-source",
		SourceParent:  "refs/heads/feature-source",
		TargetFeature: "feature-target",
		TargetParent:  "refs/heads/feature-target",
		SourceSHA:     ev.FeatureASHA,
		TargetSHA:     ev.FeatureBSHA,
		Evidence:      ev,
	})
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}

	_, cand, err := svc.Approve(ctx, ApproveParams{
		RequestID:     req.ID,
		SourceFeature: "feature-source",
		TargetFeature: "feature-target",
		Actor:         "local:testuser",
		CandidateID:   "cand-output-only",
	})
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if cand.Status != CandidateStatusRunning {
		t.Fatalf("cand.Status = %v, want %v", cand.Status, CandidateStatusRunning)
	}
	if cand.CandidateSHA != "" {
		t.Fatalf("cand.CandidateSHA = %q, want empty before output persist", cand.CandidateSHA)
	}
	if cand.Output != "" {
		t.Fatalf("cand.Output = %q, want empty before output persist", cand.Output)
	}

	payload := `{"cause_summary":"business hunk","risk_band":"high","proposed_sha":"abc123","verify_budget":"~4 min: ./lucind-checks.sh"}`
	updTime := now.Add(2 * time.Minute)
	svcUpd := NewService(l, WithClock(func() time.Time { return updTime }))

	got, err := svcUpd.UpdateCandidateOutput(ctx, cand.ID, payload)
	if err != nil {
		t.Fatalf("UpdateCandidateOutput: %v", err)
	}
	if got.Output != payload {
		t.Errorf("got.Output = %q, want %q", got.Output, payload)
	}
	if got.Status != CandidateStatusRunning {
		t.Errorf("got.Status = %v, want still %v", got.Status, CandidateStatusRunning)
	}
	if got.CandidateSHA != "" {
		t.Errorf("got.CandidateSHA = %q, want empty (output-only must not write CandidateSHA)", got.CandidateSHA)
	}

	fromDB, err := svcUpd.GetCandidate(ctx, cand.ID)
	if err != nil {
		t.Fatalf("GetCandidate after output persist: %v", err)
	}
	if fromDB.Output != payload {
		t.Errorf("fromDB.Output = %q, want persisted payload", fromDB.Output)
	}
	if fromDB.Status != CandidateStatusRunning {
		t.Errorf("fromDB.Status = %v, want still %v", fromDB.Status, CandidateStatusRunning)
	}
	if fromDB.CandidateSHA != "" {
		t.Errorf("fromDB.CandidateSHA = %q, want empty", fromDB.CandidateSHA)
	}

	_, err = svcUpd.UpdateCandidateOutput(ctx, "does-not-exist", payload)
	if !errors.Is(err, ErrCandidateNotFound) {
		t.Errorf("UpdateCandidateOutput(missing) error = %v, want ErrCandidateNotFound", err)
	}
}
