package serve_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/feature"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/overlap"
	"github.com/LanzerDevCorp/lucind-ai/internal/reconcile"
	"github.com/LanzerDevCorp/lucind-ai/internal/serve"
)

func openModelLedger(t *testing.T) *ledger.Ledger {
	t.Helper()
	l, err := ledger.Open(context.Background(), t.TempDir())
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
	hash, err := ev.ComputeHash()
	if err != nil {
		panic(err)
	}
	ev.Hash = hash
	return ev
}

func assertTimeEqual(t *testing.T, got, want time.Time, field string) {
	t.Helper()
	if !got.UTC().Equal(want.UTC()) {
		t.Errorf("%s = %v, want %v", field, got.UTC(), want.UTC())
	}
}

// TestStatusRoundTripFromWriteAPIs proves the DTO/list/get layer reads feature,
// attempt, lease, overlap evidence, and reconciliation request/candidate state
// from the ledger as plain structs after those rows are written through
// feature/reconcile/overlap APIs (not raw SQL, not git, not shell).
func TestStatusRoundTripFromWriteAPIs(t *testing.T) {
	ctx := context.Background()
	l := openModelLedger(t)

	featSvc := feature.NewService(l)
	srcFeat, err := featSvc.Create(ctx, "feat-source", "refs/heads/feat-source", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err != nil {
		t.Fatalf("feature.Create source: %v", err)
	}
	tgtFeat, err := featSvc.Create(ctx, "feat-target", "refs/heads/feat-target", "cccccccccccccccccccccccccccccccccccccccc", "dddddddddddddddddddddddddddddddddddddddd")
	if err != nil {
		t.Fatalf("feature.Create target: %v", err)
	}

	leaseTTL := 15 * time.Minute
	gotLease, err := featSvc.AcquireLease(ctx, tgtFeat.ID, "worker-dto", leaseTTL)
	if err != nil {
		t.Fatalf("AcquireLease: %v", err)
	}

	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	recSvc := reconcile.NewService(l, reconcile.WithClock(func() time.Time { return now }))
	ev := sampleRequiredEvidence()
	req, err := recSvc.CreateRequest(ctx, reconcile.CreateRequestParams{
		ID:            "req-dto-1",
		FeatureID:     tgtFeat.ID,
		SourceFeature: srcFeat.ID,
		SourceParent:  srcFeat.ParentRef,
		TargetFeature: tgtFeat.ID,
		TargetParent:  tgtFeat.ParentRef,
		SourceSHA:     ev.FeatureASHA,
		TargetSHA:     ev.FeatureBSHA,
		Evidence:      ev,
		TTL:           15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}

	approved, cand, err := recSvc.Approve(ctx, reconcile.ApproveParams{
		RequestID:         req.ID,
		SourceFeature:     srcFeat.ID,
		TargetFeature:     tgtFeat.ID,
		ExpectedSourceSHA: ev.FeatureASHA,
		ExpectedTargetSHA: ev.FeatureBSHA,
		Actor:             "local:reviewer",
		AllowedPaths:      []string{"pkg/service.go"},
		CandidateID:       "cand-dto-1",
	})
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}

	m := serve.NewModel(l)

	features, err := m.ListFeatures(ctx)
	if err != nil {
		t.Fatalf("ListFeatures: %v", err)
	}
	if len(features) != 2 {
		t.Fatalf("ListFeatures len = %d, want 2", len(features))
	}

	gotSrc, err := m.GetFeature(ctx, srcFeat.ID)
	if err != nil {
		t.Fatalf("GetFeature source: %v", err)
	}
	gotTgt, err := m.GetFeature(ctx, tgtFeat.ID)
	if err != nil {
		t.Fatalf("GetFeature target: %v", err)
	}

	assertFeatureRoundTrip(t, gotSrc, srcFeat)
	assertFeatureRoundTrip(t, gotTgt, tgtFeat)

	leases, err := m.ListLeases(ctx)
	if err != nil {
		t.Fatalf("ListLeases: %v", err)
	}
	if len(leases) != 1 {
		t.Fatalf("ListLeases len = %d, want 1", len(leases))
	}
	leaseDTO, err := m.GetLease(ctx, tgtFeat.ID)
	if err != nil {
		t.Fatalf("GetLease: %v", err)
	}
	if leaseDTO.FeatureID != gotLease.FeatureID {
		t.Errorf("lease.FeatureID = %q, want %q", leaseDTO.FeatureID, gotLease.FeatureID)
	}
	if leaseDTO.Owner != gotLease.Owner {
		t.Errorf("lease.Owner = %q, want %q", leaseDTO.Owner, gotLease.Owner)
	}
	if leaseDTO.Fence != gotLease.Fence {
		t.Errorf("lease.Fence = %d, want %d", leaseDTO.Fence, gotLease.Fence)
	}
	assertTimeEqual(t, leaseDTO.ExpiresAt, gotLease.ExpiresAt, "lease.ExpiresAt")
	assertTimeEqual(t, leaseDTO.AcquiredAt, gotLease.AcquiredAt, "lease.AcquiredAt")
	assertTimeEqual(t, leaseDTO.UpdatedAt, gotLease.UpdatedAt, "lease.UpdatedAt")
	if leases[0].FeatureID != leaseDTO.FeatureID || leases[0].Owner != leaseDTO.Owner || leases[0].Fence != leaseDTO.Fence {
		t.Errorf("ListLeases[0] = %+v, want GetLease %+v", leases[0], leaseDTO)
	}

	attempts, err := m.ListAttempts(ctx, tgtFeat.ID)
	if err != nil {
		t.Fatalf("ListAttempts: %v", err)
	}
	if attempts == nil {
		t.Fatal("ListAttempts returned nil slice, want empty non-nil")
	}
	if len(attempts) != 0 {
		t.Errorf("ListAttempts len = %d, want 0 (no attempt write API in feature/reconcile/overlap)", len(attempts))
	}
	if _, err := m.GetAttempt(ctx, "missing-attempt"); err == nil {
		t.Error("GetAttempt(missing) error = nil, want not found")
	}

	overlapList, err := m.ListOverlapEvidence(ctx, tgtFeat.ID)
	if err != nil {
		t.Fatalf("ListOverlapEvidence: %v", err)
	}
	if len(overlapList) != 1 {
		t.Fatalf("ListOverlapEvidence len = %d, want 1", len(overlapList))
	}
	ov, err := m.GetOverlapEvidence(ctx, tgtFeat.ID, ev.Hash)
	if err != nil {
		t.Fatalf("GetOverlapEvidence: %v", err)
	}
	if ov.FeatureID != tgtFeat.ID {
		t.Errorf("overlap.FeatureID = %q, want %q", ov.FeatureID, tgtFeat.ID)
	}
	if ov.Version != ev.Version {
		t.Errorf("overlap.Version = %q, want %q", ov.Version, ev.Version)
	}
	if ov.EvidenceHash != ev.Hash {
		t.Errorf("overlap.EvidenceHash = %q, want %q", ov.EvidenceHash, ev.Hash)
	}
	if ov.EvidenceClass != string(overlap.ClassRequired) {
		t.Errorf("overlap.EvidenceClass = %q, want %q", ov.EvidenceClass, overlap.ClassRequired)
	}
	if ov.EvidenceJSON == "" {
		t.Fatal("overlap.EvidenceJSON is empty")
	}
	var parsed overlap.Evidence
	if err := json.Unmarshal([]byte(ov.EvidenceJSON), &parsed); err != nil {
		t.Fatalf("unmarshal EvidenceJSON: %v", err)
	}
	if parsed.Version != ev.Version || parsed.Hash != ev.Hash || parsed.Class != ev.Class {
		t.Errorf("parsed evidence version/hash/class = %q/%q/%q, want %q/%q/%q", parsed.Version, parsed.Hash, parsed.Class, ev.Version, ev.Hash, ev.Class)
	}
	if parsed.BaseSHA != ev.BaseSHA || parsed.FeatureASHA != ev.FeatureASHA || parsed.FeatureBSHA != ev.FeatureBSHA {
		t.Errorf("parsed evidence SHAs = %q/%q/%q, want %q/%q/%q", parsed.BaseSHA, parsed.FeatureASHA, parsed.FeatureBSHA, ev.BaseSHA, ev.FeatureASHA, ev.FeatureBSHA)
	}
	if overlapList[0].EvidenceHash != ov.EvidenceHash || overlapList[0].EvidenceJSON != ov.EvidenceJSON {
		t.Errorf("ListOverlapEvidence[0] hash/json mismatch vs GetOverlapEvidence")
	}

	reqs, err := m.ListReconciliationRequests(ctx, tgtFeat.ID)
	if err != nil {
		t.Fatalf("ListReconciliationRequests: %v", err)
	}
	if len(reqs) != 1 {
		t.Fatalf("ListReconciliationRequests len = %d, want 1", len(reqs))
	}
	gotReq, err := m.GetReconciliationRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetReconciliationRequest: %v", err)
	}
	if gotReq.ID != approved.ID {
		t.Errorf("request.ID = %q, want %q", gotReq.ID, approved.ID)
	}
	if gotReq.FeatureID != approved.FeatureID {
		t.Errorf("request.FeatureID = %q, want %q", gotReq.FeatureID, approved.FeatureID)
	}
	if gotReq.Direction != approved.Direction {
		t.Errorf("request.Direction = %q, want %q", gotReq.Direction, approved.Direction)
	}
	if gotReq.Status != string(approved.Status) {
		t.Errorf("request.Status = %q, want %q", gotReq.Status, approved.Status)
	}
	if gotReq.Actor != approved.Actor {
		t.Errorf("request.Actor = %q, want %q", gotReq.Actor, approved.Actor)
	}
	if gotReq.EvidenceVersion != approved.EvidenceVersion {
		t.Errorf("request.EvidenceVersion = %q, want %q", gotReq.EvidenceVersion, approved.EvidenceVersion)
	}
	if gotReq.EvidenceHash != approved.EvidenceHash {
		t.Errorf("request.EvidenceHash = %q, want %q", gotReq.EvidenceHash, approved.EvidenceHash)
	}
	if gotReq.SourceSHA != approved.SourceSHA {
		t.Errorf("request.SourceSHA = %q, want %q", gotReq.SourceSHA, approved.SourceSHA)
	}
	if gotReq.TargetSHA != approved.TargetSHA {
		t.Errorf("request.TargetSHA = %q, want %q", gotReq.TargetSHA, approved.TargetSHA)
	}
	if gotReq.ExpiresAt == nil || !gotReq.ExpiresAt.Equal(approved.ExpiresAt) {
		t.Errorf("request.ExpiresAt = %v, want %v", gotReq.ExpiresAt, approved.ExpiresAt)
	}
	assertTimeEqual(t, gotReq.CreatedAt, approved.CreatedAt, "request.CreatedAt")
	assertTimeEqual(t, gotReq.UpdatedAt, approved.UpdatedAt, "request.UpdatedAt")

	cands, err := m.ListReconciliationCandidates(ctx, req.ID)
	if err != nil {
		t.Fatalf("ListReconciliationCandidates: %v", err)
	}
	if len(cands) != 1 {
		t.Fatalf("ListReconciliationCandidates len = %d, want 1", len(cands))
	}
	gotCand, err := m.GetReconciliationCandidate(ctx, cand.ID)
	if err != nil {
		t.Fatalf("GetReconciliationCandidate: %v", err)
	}
	if gotCand.ID != cand.ID {
		t.Errorf("candidate.ID = %q, want %q", gotCand.ID, cand.ID)
	}
	if gotCand.RequestID != cand.RequestID {
		t.Errorf("candidate.RequestID = %q, want %q", gotCand.RequestID, cand.RequestID)
	}
	if gotCand.Status != string(cand.Status) {
		t.Errorf("candidate.Status = %q, want %q", gotCand.Status, cand.Status)
	}
	if len(gotCand.AllowedPaths) != 1 || gotCand.AllowedPaths[0] != "pkg/service.go" {
		t.Errorf("candidate.AllowedPaths = %v, want [pkg/service.go]", gotCand.AllowedPaths)
	}
	if gotCand.Model != cand.Model {
		t.Errorf("candidate.Model = %q, want %q", gotCand.Model, cand.Model)
	}
	if gotCand.Config != cand.Config {
		t.Errorf("candidate.Config = %q, want %q", gotCand.Config, cand.Config)
	}
	if gotCand.Output != cand.Output {
		t.Errorf("candidate.Output = %q, want %q", gotCand.Output, cand.Output)
	}
	if gotCand.Checks != cand.Checks {
		t.Errorf("candidate.Checks = %q, want %q", gotCand.Checks, cand.Checks)
	}
	if gotCand.CandidateSHA != cand.CandidateSHA {
		t.Errorf("candidate.CandidateSHA = %q, want %q", gotCand.CandidateSHA, cand.CandidateSHA)
	}
	if gotCand.FailureReason != cand.FailureReason {
		t.Errorf("candidate.FailureReason = %q, want %q", gotCand.FailureReason, cand.FailureReason)
	}
	assertTimeEqual(t, gotCand.CreatedAt, cand.CreatedAt, "candidate.CreatedAt")
	assertTimeEqual(t, gotCand.UpdatedAt, cand.UpdatedAt, "candidate.UpdatedAt")

	if len(gotReq.Candidates) != 1 || gotReq.Candidates[0].ID != gotCand.ID {
		t.Errorf("GetReconciliationRequest.Candidates = %+v, want candidate %q", gotReq.Candidates, gotCand.ID)
	}

	events, err := m.ListAuditEvents(ctx, tgtFeat.ID)
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}
	types := map[string]int{}
	for _, e := range events {
		types[e.Type]++
		if e.FeatureID != tgtFeat.ID {
			t.Errorf("audit event %+v FeatureID = %q, want %q", e, e.FeatureID, tgtFeat.ID)
		}
		if e.At.IsZero() {
			t.Errorf("audit event %s has zero timestamp", e.Type)
		}
		if e.ID == 0 {
			t.Errorf("audit event %s has zero ID", e.Type)
		}
	}
	for _, wantType := range []string{"feature_created", "feature_activated", "reconciliation_request_created", "reconciliation_approved"} {
		if types[wantType] == 0 {
			t.Errorf("audit events missing type %q (got %v)", wantType, types)
		}
	}
	if len(gotReq.Audit) == 0 {
		t.Error("GetReconciliationRequest.Audit is empty, want request audit history")
	}
}

func assertFeatureRoundTrip(t *testing.T, got serve.Feature, want feature.Feature) {
	t.Helper()
	if got.ID != want.ID {
		t.Errorf("feature.ID = %q, want %q", got.ID, want.ID)
	}
	if got.ParentRef != want.ParentRef {
		t.Errorf("feature.ParentRef = %q, want %q", got.ParentRef, want.ParentRef)
	}
	if got.BaseSHA != want.BaseSHA {
		t.Errorf("feature.BaseSHA = %q, want %q", got.BaseSHA, want.BaseSHA)
	}
	if got.ExpectedParentSHA != want.ExpectedParentSHA {
		t.Errorf("feature.ExpectedParentSHA = %q, want %q", got.ExpectedParentSHA, want.ExpectedParentSHA)
	}
	if got.Status != string(want.Status) {
		t.Errorf("feature.Status = %q, want %q", got.Status, want.Status)
	}
	assertTimeEqual(t, got.CreatedAt, want.CreatedAt, "feature.CreatedAt")
	assertTimeEqual(t, got.UpdatedAt, want.UpdatedAt, "feature.UpdatedAt")
}

func TestReconciliationObservableStatus(t *testing.T) {
	t.Run("decline", testReconciliationDeclinePath)
	t.Run("expiry", testReconciliationExpiryPath)
	t.Run("successful-candidate", testReconciliationSuccessfulCandidatePath)
}

func testReconciliationDeclinePath(t *testing.T) {
	ctx := context.Background()
	l := openModelLedger(t)
	now := time.Date(2026, 8, 20, 15, 0, 0, 0, time.UTC)
	recSvc := reconcile.NewService(l, reconcile.WithClock(func() time.Time { return now }))
	ev := sampleRequiredEvidence()

	req, err := recSvc.CreateRequest(ctx, reconcile.CreateRequestParams{
		ID:            "req-decline",
		FeatureID:     "feat-target",
		SourceFeature: "feat-source",
		SourceParent:  "refs/heads/feat-source",
		TargetFeature: "feat-target",
		TargetParent:  "refs/heads/feat-target",
		SourceSHA:     ev.FeatureASHA,
		TargetSHA:     ev.FeatureBSHA,
		Evidence:      ev,
		TTL:           15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}

	decTime := now.Add(time.Minute)
	recSvc = reconcile.NewService(l, reconcile.WithClock(func() time.Time { return decTime }))
	declined, err := recSvc.Decline(ctx, req.ID, "local:reviewer", "direction deferred for manual review")
	if err != nil {
		t.Fatalf("Decline: %v", err)
	}

	got, err := serve.NewModel(l).GetReconciliationRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetReconciliationRequest: %v", err)
	}
	assertObservableCore(t, got, declined.Actor, declined.Direction, declined.EvidenceVersion, declined.EvidenceHash, declined.SourceSHA, declined.TargetSHA, declined.CreatedAt, declined.UpdatedAt)
	if got.Status != string(reconcile.RequestStatusDeclined) {
		t.Errorf("Status = %q, want declined", got.Status)
	}
	if got.Actor != "local:reviewer" {
		t.Errorf("Actor = %q, want local:reviewer", got.Actor)
	}
	if len(got.Candidates) != 0 {
		t.Errorf("Candidates = %+v, want none after decline", got.Candidates)
	}
	if got.CheckOutcomes != "" {
		t.Errorf("CheckOutcomes = %q, want empty after decline", got.CheckOutcomes)
	}
	if !strings.Contains(got.Failures, "direction deferred for manual review") {
		t.Errorf("Failures = %q, want decline reason", got.Failures)
	}
	if got.CASResult.Outcome != "not_attempted" {
		t.Errorf("CASResult.Outcome = %q, want not_attempted", got.CASResult.Outcome)
	}
	if got.CASResult.CandidateSHA != "" || got.CASResult.FailureReason != "" {
		t.Errorf("CASResult = %+v, want empty sha/failure on decline", got.CASResult)
	}
}

func testReconciliationExpiryPath(t *testing.T) {
	ctx := context.Background()
	l := openModelLedger(t)
	past := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	recSvc := reconcile.NewService(l, reconcile.WithClock(func() time.Time { return past }))
	ev := sampleRequiredEvidence()

	req, err := recSvc.CreateRequest(ctx, reconcile.CreateRequestParams{
		ID:            "req-expiry",
		FeatureID:     "feat-target",
		SourceFeature: "feat-source",
		SourceParent:  "refs/heads/feat-source",
		TargetFeature: "feat-target",
		TargetParent:  "refs/heads/feat-target",
		SourceSHA:     ev.FeatureASHA,
		TargetSHA:     ev.FeatureBSHA,
		Evidence:      ev,
		TTL:           time.Minute,
	})
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}

	got, err := serve.NewModel(l).GetReconciliationRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetReconciliationRequest: %v", err)
	}
	assertObservableCore(t, got, req.Actor, req.Direction, req.EvidenceVersion, req.EvidenceHash, req.SourceSHA, req.TargetSHA, req.CreatedAt, req.UpdatedAt)
	if got.Status != string(reconcile.RequestStatusExpired) {
		t.Errorf("Status = %q, want expired", got.Status)
	}
	if got.Actor != "" {
		t.Errorf("Actor = %q, want empty on expiry", got.Actor)
	}
	if len(got.Candidates) != 0 {
		t.Errorf("Candidates = %+v, want none on expiry", got.Candidates)
	}
	if got.CheckOutcomes != "" {
		t.Errorf("CheckOutcomes = %q, want empty on expiry", got.CheckOutcomes)
	}
	if got.Failures != "expired" {
		t.Errorf("Failures = %q, want expired", got.Failures)
	}
	if got.CASResult.Outcome != "not_attempted" {
		t.Errorf("CASResult.Outcome = %q, want not_attempted", got.CASResult.Outcome)
	}
}

func testReconciliationSuccessfulCandidatePath(t *testing.T) {
	ctx := context.Background()
	l := openModelLedger(t)
	now := time.Date(2026, 8, 20, 16, 0, 0, 0, time.UTC)
	recSvc := reconcile.NewService(l, reconcile.WithClock(func() time.Time { return now }))
	ev := sampleRequiredEvidence()

	req, err := recSvc.CreateRequest(ctx, reconcile.CreateRequestParams{
		ID:            "req-success",
		FeatureID:     "feat-target",
		SourceFeature: "feat-source",
		SourceParent:  "refs/heads/feat-source",
		TargetFeature: "feat-target",
		TargetParent:  "refs/heads/feat-target",
		SourceSHA:     ev.FeatureASHA,
		TargetSHA:     ev.FeatureBSHA,
		Evidence:      ev,
		TTL:           15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}

	approved, _, err := recSvc.Approve(ctx, reconcile.ApproveParams{
		RequestID:         req.ID,
		SourceFeature:     "feat-source",
		TargetFeature:     "feat-target",
		ExpectedSourceSHA: ev.FeatureASHA,
		ExpectedTargetSHA: ev.FeatureBSHA,
		Actor:             "local:reviewer",
		AllowedPaths:      []string{"pkg/service.go"},
		CandidateID:       "cand-success",
	})
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}

	updTime := now.Add(5 * time.Minute)
	recSvc = reconcile.NewService(l, reconcile.WithClock(func() time.Time { return updTime }))
	const promotedSHA = "sha_cas_promoted_aaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	_, err = recSvc.UpdateCandidateStatus(ctx, "cand-success", reconcile.CandidateStatusIntegrated, promotedSHA, "")
	if err != nil {
		t.Fatalf("UpdateCandidateStatus: %v", err)
	}

	got, err := serve.NewModel(l).GetReconciliationRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetReconciliationRequest: %v", err)
	}
	assertObservableCore(t, got, approved.Actor, approved.Direction, approved.EvidenceVersion, approved.EvidenceHash, approved.SourceSHA, approved.TargetSHA, approved.CreatedAt, approved.UpdatedAt)
	if got.Status != string(reconcile.RequestStatusApproved) {
		t.Errorf("Status = %q, want approved", got.Status)
	}
	if got.Actor != "local:reviewer" {
		t.Errorf("Actor = %q, want local:reviewer", got.Actor)
	}
	if len(got.Candidates) != 1 {
		t.Fatalf("Candidates len = %d, want 1", len(got.Candidates))
	}
	cand := got.Candidates[0]
	if cand.Status != string(reconcile.CandidateStatusIntegrated) {
		t.Errorf("candidate.Status = %q, want integrated", cand.Status)
	}
	if cand.CandidateSHA != promotedSHA {
		t.Errorf("candidate.CandidateSHA = %q, want %q", cand.CandidateSHA, promotedSHA)
	}
	if cand.Checks != "" {
		t.Errorf("candidate.Checks = %q, want empty (no check output recorded on this path)", cand.Checks)
	}
	if got.CheckOutcomes != cand.Checks {
		t.Errorf("CheckOutcomes = %q, want candidate checks %q", got.CheckOutcomes, cand.Checks)
	}
	if got.Failures != "" {
		t.Errorf("Failures = %q, want empty on successful candidate", got.Failures)
	}
	if got.CASResult.Outcome != "promoted" {
		t.Errorf("CASResult.Outcome = %q, want promoted", got.CASResult.Outcome)
	}
	if got.CASResult.CandidateSHA != promotedSHA {
		t.Errorf("CASResult.CandidateSHA = %q, want %q", got.CASResult.CandidateSHA, promotedSHA)
	}
	if got.CASResult.FailureReason != "" {
		t.Errorf("CASResult.FailureReason = %q, want empty", got.CASResult.FailureReason)
	}
}

func assertObservableCore(t *testing.T, got serve.ReconciliationRequest, actor, direction, evidenceVersion, evidenceHash, sourceSHA, targetSHA string, createdAt, updatedAt time.Time) {
	t.Helper()
	if got.Actor != actor {
		t.Errorf("Actor = %q, want %q", got.Actor, actor)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Errorf("timestamps zero: created=%v updated=%v", got.CreatedAt, got.UpdatedAt)
	}
	assertTimeEqual(t, got.CreatedAt, createdAt, "CreatedAt")
	assertTimeEqual(t, got.UpdatedAt, updatedAt, "UpdatedAt")
	if got.EvidenceVersion != evidenceVersion {
		t.Errorf("EvidenceVersion = %q, want %q", got.EvidenceVersion, evidenceVersion)
	}
	if got.EvidenceHash != evidenceHash {
		t.Errorf("EvidenceHash = %q, want %q", got.EvidenceHash, evidenceHash)
	}
	if got.Direction != direction {
		t.Errorf("Direction = %q, want %q", got.Direction, direction)
	}
	if got.SourceSHA != sourceSHA {
		t.Errorf("SourceSHA = %q, want %q", got.SourceSHA, sourceSHA)
	}
	if got.TargetSHA != targetSHA {
		t.Errorf("TargetSHA = %q, want %q", got.TargetSHA, targetSHA)
	}
}

func TestModelRunLaneAndProgressJSONContract(t *testing.T) {
	ctx := context.Background()
	l := openModelLedger(t)
	started := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	ended := started.Add(10 * time.Minute)
	for _, run := range []ledger.Run{
		{RunID: "run-old", FeatureID: "feature-old", Status: "done", TargetRef: "refs/heads/old", StartedAt: started.Add(-time.Hour)},
		{RunID: "run-1", FeatureID: "feature-1", Status: "running", TargetRef: "refs/heads/feature-1", LaneCount: 2, StartedAt: started, EndedAt: &ended},
	} {
		if err := l.RegisterRun(ctx, run); err != nil {
			t.Fatalf("RegisterRun(%s): %v", run.RunID, err)
		}
	}
	for _, ln := range []ledger.Lane{
		{RunID: "run-1", LaneID: "lane-b", PacketID: "packet-b", Executor: "agy", RoutingCondition: "fallback", Status: lane.Done, WorktreePath: "/tmp/lane-b", WorktreePreserved: true, Attempt: 2},
		{RunID: "run-1", LaneID: "lane-a", PacketID: "packet-a", Executor: "opencode", RoutingCondition: "primary", Status: lane.Running},
	} {
		if err := l.RegisterLane(ctx, ln); err != nil {
			t.Fatalf("RegisterLane(%s): %v", ln.LaneID, err)
		}
	}
	metadata := ledger.LaneMetadata{
		RunID: "run-1", LaneID: "lane-a", Model: "gpt-5.6", Agent: "builder",
		SDDPhase: "apply", FanoutGroup: "serve", Change: "control-room",
		Feature: "feature-1", Skill: "lucind-apply", PacketPath: ".lucind/packets/lane-a.md",
		AllowedPaths: []string{"internal/serve/model.go"},
		Dependencies: []string{"ledger-progress"}, BodyDigest: "sha256:abc",
	}
	if err := l.UpdateLaneMetadata(ctx, metadata, started); err != nil {
		t.Fatalf("UpdateLaneMetadata: %v", err)
	}
	if err := l.RequestApproval(ctx, ledger.Approval{RunID: "run-1", LaneID: "lane-a", PacketID: "packet-a", RequestedAt: started}); err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}
	for _, seq := range []int64{3, 1, 2} {
		progress := ledger.LaneProgress{
			RunID: "run-1", LaneID: "lane-a", Seq: seq, Message: fmt.Sprintf("chunk-%d", seq),
			At: started.Add(time.Duration(seq) * time.Second),
		}
		if seq == 2 {
			progress.TotalTokens = 1000
			progress.CostUSD = 0.05
			progress.ToolCalls = 6
		}
		if seq == 3 {
			progress.TotalTokens = 2000
			progress.CostUSD = 0.10
			progress.ToolCalls = 12
		}
		if err := l.AppendProgress(ctx, progress); err != nil {
			t.Fatalf("AppendProgress(%d): %v", seq, err)
		}
	}

	m := serve.NewModel(l)
	run, err := m.GetRun(ctx, "run-1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	assertJSON(t, run, `{"run_id":"run-1","feature_id":"feature-1","status":"running","target_ref":"refs/heads/feature-1","lane_count":2,"started_at":"2026-08-22T10:00:00Z","ended_at":"2026-08-22T10:10:00Z","lane_status_counts":{"pending":0,"running":1,"done":1,"blocked":0,"deviated":0,"failed":0},"pending_approvals":1}`)
	if summary, err := m.GetRunSummary(ctx, "run-1"); err != nil || !reflect.DeepEqual(summary, run) {
		t.Fatalf("GetRunSummary = %+v, %v; want GetRun result", summary, err)
	}

	runs, err := m.ListRuns(ctx)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 2 || runs[0].RunID != "run-1" || runs[1].RunID != "run-old" {
		t.Fatalf("ListRuns order = %+v, want run-1 then run-old", runs)
	}

	lanes, err := m.ListLanes(ctx, "run-1")
	if err != nil {
		t.Fatalf("ListLanes: %v", err)
	}
	if len(lanes) != 2 || lanes[0].LaneID != "lane-a" || lanes[1].LaneID != "lane-b" {
		t.Fatalf("ListLanes order = %+v, want lane-a then lane-b", lanes)
	}
	assertJSON(t, lanes[0], `{"run_id":"run-1","lane_id":"lane-a","packet_id":"packet-a","executor":"opencode","routing_condition":"primary","status":"running","worktree_path":"","worktree_preserved":false,"attempt":1,"started_at":null,"ended_at":null,"model":"gpt-5.6","agent":"builder","feature":"feature-1","sdd_phase":"apply","fanout_group":"serve","change":"control-room","skill":"lucind-apply","packet_path":".lucind/packets/lane-a.md","allowed_paths":["internal/serve/model.go"],"dependencies":["ledger-progress"],"body_digest":"sha256:abc"}`)
	if got, err := m.GetLane(ctx, "run-1", "lane-a"); err != nil || got.LaneID != lanes[0].LaneID {
		t.Fatalf("GetLane = %+v, %v; want lane-a", got, err)
	}
	if _, err := m.GetLane(ctx, "run-1", "missing"); !errors.Is(err, ledger.ErrLaneUnknown) {
		t.Fatalf("GetLane(missing) error = %v, want ledger.ErrLaneUnknown", err)
	}

	progress, err := m.GetLaneProgress(ctx, "run-1", "lane-a", 1)
	if err != nil {
		t.Fatalf("GetLaneProgress: %v", err)
	}
	// Lane StartedAt is unset by RegisterLane; tool_rate uses the 1s floor:
	// 6 / (1/60) = 360, 12 / (1/60) = 720.
	assertJSON(t, progress, `[{"run_id":"run-1","lane_id":"lane-a","seq":2,"message":"chunk-2","at":"2026-08-22T10:00:02Z","total_tokens":1000,"cost_usd":0.05,"tool_calls":6,"tool_rate":360},{"run_id":"run-1","lane_id":"lane-a","seq":3,"message":"chunk-3","at":"2026-08-22T10:00:03Z","total_tokens":2000,"cost_usd":0.1,"tool_calls":12,"tool_rate":720}]`)

	if _, err := m.GetRun(ctx, "missing"); !errors.Is(err, ledger.ErrRunUnknown) {
		t.Fatalf("GetRun(missing) error = %v, want ledger.ErrRunUnknown", err)
	}
}

func TestModelDerivedFlowBatchAndOverview(t *testing.T) {
	ctx := context.Background()
	l := openModelLedger(t)
	at := time.Date(2026, 8, 22, 11, 0, 0, 0, time.UTC)
	if err := l.RegisterRun(ctx, ledger.Run{RunID: "run-flow", Status: "running", LaneCount: 2, StartedAt: at}); err != nil {
		t.Fatalf("RegisterRun: %v", err)
	}
	for _, ln := range []ledger.Lane{
		{RunID: "run-flow", LaneID: "lane-a", PacketID: "a", Executor: "agy", RoutingCondition: "a", Status: lane.Done},
		{RunID: "run-flow", LaneID: "lane-b", PacketID: "b", Executor: "agy", RoutingCondition: "b", Status: lane.Blocked, WorktreePreserved: true},
	} {
		if err := l.RegisterLane(ctx, ln); err != nil {
			t.Fatalf("RegisterLane: %v", err)
		}
		if err := l.UpdateLaneMetadata(ctx, ledger.LaneMetadata{RunID: ln.RunID, LaneID: ln.LaneID, Change: "control-room", SDDPhase: "apply", FanoutGroup: "serve"}, at); err != nil {
			t.Fatalf("UpdateLaneMetadata: %v", err)
		}
	}
	if err := l.AppendEvent(ctx, ledger.Event{RunID: "run-flow", LaneID: "lane-b", Type: ledger.EventLaneNote, Detail: "blocked by schema", At: at}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	m := serve.NewModel(l)
	flows, err := m.ListSDDFlows(ctx)
	if err != nil {
		t.Fatalf("ListSDDFlows: %v", err)
	}
	assertJSON(t, flows, `[{"run_id":"run-flow","change":"control-room","sdd_phase":"apply","fanout_group":"serve","status":"blocked","lane_count":2,"lane_ids":["lane-a","lane-b"]}]`)
	overview, err := m.GetOverview(ctx)
	if err != nil {
		t.Fatalf("GetOverview: %v", err)
	}
	assertJSON(t, overview, `{"status":"blocked","run_count":1,"active_run_count":1,"lane_count":2,"lane_status_counts":{"pending":0,"running":0,"done":1,"blocked":1,"deviated":0,"failed":0},"pending_approvals":0,"flow_count":1}`)
	batch, err := m.ListBatchLanes(ctx, "run-flow")
	if err != nil {
		t.Fatalf("ListBatchLanes: %v", err)
	}
	if len(batch) != 2 || batch[1].Note != "blocked by schema" || !batch[1].Outcome.Released {
		t.Fatalf("ListBatchLanes = %+v, want latest note and released outcome", batch)
	}
	assertJSON(t, batch[1].Outcome, `{"released":true,"integrate":["lane-a"],"preserve":["lane-b"]}`)
}

func TestModelNewListsAreNonNilAndDatabaseErrorsRemainObservable(t *testing.T) {
	ctx := context.Background()
	l := openModelLedger(t)
	m := serve.NewModel(l)
	for name, query := range map[string]func() (any, error){
		"runs":     func() (any, error) { return m.ListRuns(ctx) },
		"lanes":    func() (any, error) { return m.ListLanes(ctx, "missing") },
		"progress": func() (any, error) { return m.GetLaneProgress(ctx, "missing", "missing", 0) },
		"flows":    func() (any, error) { return m.ListSDDFlows(ctx) },
		"batch":    func() (any, error) { return m.ListBatchLanes(ctx, "missing") },
	} {
		got, err := query()
		if err != nil {
			t.Fatalf("%s empty query: %v", name, err)
		}
		assertJSON(t, got, `[]`)
	}
	overview, err := m.GetOverview(ctx)
	if err != nil {
		t.Fatalf("GetOverview empty query: %v", err)
	}
	assertJSON(t, overview, `{"status":"empty","run_count":0,"active_run_count":0,"lane_count":0,"lane_status_counts":{"pending":0,"running":0,"done":0,"blocked":0,"deviated":0,"failed":0},"pending_approvals":0,"flow_count":0}`)
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := m.ListRuns(ctx); err == nil {
		t.Fatal("ListRuns on closed DB error = nil, want observable database error")
	}
}

func assertJSON(t *testing.T, value any, want string) {
	t.Helper()
	got, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(got) != want {
		t.Errorf("JSON = %s\nwant = %s", got, want)
	}
}

func TestBatchLanesRoundTrip(t *testing.T) {
	ctx := context.Background()
	l := openModelLedger(t)
	at := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	runID := "run-batch-roundtrip"

	if err := l.RegisterRun(ctx, ledger.Run{
		RunID:     runID,
		FeatureID: "feature-batch",
		Status:    "running",
		TargetRef: "refs/heads/main",
		LaneCount: 3,
		StartedAt: at,
	}); err != nil {
		t.Fatalf("RegisterRun: %v", err)
	}

	lanes := []ledger.Lane{
		{
			RunID:             runID,
			LaneID:            "lane-1-done",
			PacketID:          "pkt-done",
			Executor:          "agy",
			RoutingCondition:  "primary",
			Status:            lane.Done,
			WorktreePath:      "/tmp/worktrees/lane-done",
			WorktreePreserved: false,
			Attempt:           1,
		},
		{
			RunID:             runID,
			LaneID:            "lane-2-demoted",
			PacketID:          "pkt-demoted",
			Executor:          "opencode",
			RoutingCondition:  "fallback",
			Status:            lane.Blocked,
			WorktreePath:      "/tmp/worktrees/lane-demoted",
			WorktreePreserved: true,
			Attempt:           2,
		},
		{
			RunID:             runID,
			LaneID:            "lane-3-failed",
			PacketID:          "pkt-failed",
			Executor:          "cursor-agent",
			RoutingCondition:  "primary",
			Status:            lane.Failed,
			WorktreePath:      "/tmp/worktrees/lane-failed",
			WorktreePreserved: true,
			Attempt:           1,
		},
	}

	for _, ln := range lanes {
		if err := l.RegisterLane(ctx, ln); err != nil {
			t.Fatalf("RegisterLane(%s): %v", ln.LaneID, err)
		}
		if err := l.UpdateLaneMetadata(ctx, ledger.LaneMetadata{
			RunID:        ln.RunID,
			LaneID:       ln.LaneID,
			Change:       "api-gaps",
			SDDPhase:     "apply",
			FanoutGroup:  "serve",
			Model:        "gpt-5.6",
			Agent:        "builder",
			Feature:      "feature-batch",
			AllowedPaths: []string{"internal/serve"},
		}, at); err != nil {
			t.Fatalf("UpdateLaneMetadata(%s): %v", ln.LaneID, err)
		}
	}

	demotionNote := "demoted: lock conflict at wave boundary"
	if err := l.AppendEvent(ctx, ledger.Event{
		RunID:  runID,
		LaneID: "lane-2-demoted",
		Type:   ledger.EventLaneNote,
		Detail: demotionNote,
		At:     at.Add(time.Minute),
	}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	m := serve.NewModel(l)
	batchLanes, err := m.ListBatchLanes(ctx, runID)
	if err != nil {
		t.Fatalf("ListBatchLanes: %v", err)
	}
	if len(batchLanes) != 3 {
		t.Fatalf("ListBatchLanes len = %d, want 3", len(batchLanes))
	}

	// 1. Status mapping assertions
	if batchLanes[0].Status != string(lane.Done) {
		t.Errorf("lane[0] status = %q, want %q", batchLanes[0].Status, lane.Done)
	}
	if batchLanes[1].Status != string(lane.Blocked) {
		t.Errorf("lane[1] status = %q, want %q", batchLanes[1].Status, lane.Blocked)
	}
	if batchLanes[2].Status != string(lane.Failed) {
		t.Errorf("lane[2] status = %q, want %q", batchLanes[2].Status, lane.Failed)
	}

	// 2. Worktree preservation assertions
	if batchLanes[0].WorktreePreserved {
		t.Errorf("lane[0] WorktreePreserved = true, want false")
	}
	if batchLanes[0].WorktreePath != "/tmp/worktrees/lane-done" {
		t.Errorf("lane[0] WorktreePath = %q, want /tmp/worktrees/lane-done", batchLanes[0].WorktreePath)
	}
	if !batchLanes[1].WorktreePreserved {
		t.Errorf("lane[1] WorktreePreserved = false, want true")
	}
	if batchLanes[1].WorktreePath != "/tmp/worktrees/lane-demoted" {
		t.Errorf("lane[1] WorktreePath = %q, want /tmp/worktrees/lane-demoted", batchLanes[1].WorktreePath)
	}
	if !batchLanes[2].WorktreePreserved {
		t.Errorf("lane[2] WorktreePreserved = false, want true")
	}
	if batchLanes[2].WorktreePath != "/tmp/worktrees/lane-failed" {
		t.Errorf("lane[2] WorktreePath = %q, want /tmp/worktrees/lane-failed", batchLanes[2].WorktreePath)
	}

	// 3. Demotion note assertion
	if batchLanes[0].Note != "" {
		t.Errorf("lane[0] Note = %q, want empty", batchLanes[0].Note)
	}
	if batchLanes[1].Note != demotionNote {
		t.Errorf("lane[1] Note = %q, want %q", batchLanes[1].Note, demotionNote)
	}
	if batchLanes[2].Note != "" {
		t.Errorf("lane[2] Note = %q, want empty", batchLanes[2].Note)
	}

	// 4. Barrier outcome assertions
	for i, bl := range batchLanes {
		if !bl.Outcome.Released {
			t.Errorf("lane[%d] Outcome.Released = false, want true (all lanes terminal)", i)
		}
		if len(bl.Outcome.Integrate) != 1 || bl.Outcome.Integrate[0] != "lane-1-done" {
			t.Errorf("lane[%d] Outcome.Integrate = %v, want [lane-1-done]", i, bl.Outcome.Integrate)
		}
		if len(bl.Outcome.Preserve) != 2 || bl.Outcome.Preserve[0] != "lane-2-demoted" || bl.Outcome.Preserve[1] != "lane-3-failed" {
			t.Errorf("lane[%d] Outcome.Preserve = %v, want [lane-2-demoted, lane-3-failed]", i, bl.Outcome.Preserve)
		}
	}
}

func TestModelSourceDoesNotShellOut(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	srcPath := filepath.Join(filepath.Dir(thisFile), "model.go")
	src, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("read model.go: %v", err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, srcPath, src, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse model.go: %v", err)
	}
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		switch path {
		case "os/exec", "os", "os/exec/internal":
			t.Errorf("model.go imports %q; DTO layer must not shell out or touch the filesystem", path)
		}
		if strings.Contains(path, "exec") {
			t.Errorf("model.go imports %q; DTO layer must not depend on exec", path)
		}
	}
	body := string(src)
	if strings.Contains(body, "*exec.Cmd") || strings.Contains(body, "exec.Command") {
		t.Error("model.go references exec.Cmd; DTO layer must not shell out")
	}
	if strings.Contains(body, `"git"`) || strings.Contains(body, `[]string{"git"`) {
		t.Error("model.go invokes git; DTO layer must not call git")
	}
}
