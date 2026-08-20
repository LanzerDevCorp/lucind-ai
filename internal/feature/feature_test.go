package feature

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
)

func newTestService(t *testing.T) (*Service, *ledger.Ledger) {
	t.Helper()
	root := t.TempDir()
	l, err := ledger.Open(context.Background(), root)
	if err != nil {
		t.Fatalf("open test ledger: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	return NewService(l), l
}

func TestFeatureLifecycleCreateActivateDisable(t *testing.T) {
	ctx := context.Background()
	svc, l := newTestService(t)

	feat, err := svc.Create(ctx, "feat-lifecycle", "refs/heads/feature-x", "sha1234")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if feat.ID != "feat-lifecycle" {
		t.Errorf("feat.ID = %q, want feat-lifecycle", feat.ID)
	}
	if feat.ParentRef != "refs/heads/feature-x" {
		t.Errorf("feat.ParentRef = %q, want refs/heads/feature-x", feat.ParentRef)
	}
	if feat.BaseSHA != "sha1234" {
		t.Errorf("feat.BaseSHA = %q, want sha1234", feat.BaseSHA)
	}
	if feat.Status != StatusActive {
		t.Errorf("feat.Status = %v, want %v", feat.Status, StatusActive)
	}

	// Disable transitions active -> disabled
	if err := svc.Disable(ctx, "feat-lifecycle"); err != nil {
		t.Fatalf("Disable failed: %v", err)
	}

	disabledFeat, err := svc.Get(ctx, "feat-lifecycle")
	if err != nil {
		t.Fatalf("Get after Disable: %v", err)
	}
	if disabledFeat.Status != StatusDisabled {
		t.Errorf("disabledFeat.Status = %v, want %v", disabledFeat.Status, StatusDisabled)
	}

	// Verify audit trail in integration_events
	events, err := l.IntegrationEvents(ctx, "feat-lifecycle")
	if err != nil {
		t.Fatalf("IntegrationEvents: %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("expected at least 2 audit events, got %d", len(events))
	}
	var eventTypes []string
	for _, e := range events {
		eventTypes = append(eventTypes, e.Type)
	}
	// We expect feature_created / feature_activated, and feature_disabled
	hasCreated := false
	hasActivated := false
	hasDisabled := false
	for _, typ := range eventTypes {
		if typ == "feature_created" {
			hasCreated = true
		}
		if typ == "feature_activated" {
			hasActivated = true
		}
		if typ == "feature_disabled" {
			hasDisabled = true
		}
	}
	if !hasCreated || !hasActivated || !hasDisabled {
		t.Errorf("eventTypes = %v, want created, activated, and disabled events", eventTypes)
	}
}

func TestFeatureInvalidTransitions(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)

	_, err := svc.Create(ctx, "feat-trans", "refs/heads/feature-trans", "sha123")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Disable(ctx, "feat-trans"); err != nil {
		t.Fatalf("Disable: %v", err)
	}

	// Transition disabled -> active must be rejected with ErrInvalidTransition
	err = svc.Activate(ctx, "feat-trans")
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("Activate on disabled feature returned %v, want ErrInvalidTransition", err)
	}

	// Non-existent feature operations must return ErrFeatureNotFound
	err = svc.Activate(ctx, "nonexistent")
	if !errors.Is(err, ErrFeatureNotFound) {
		t.Fatalf("Activate on nonexistent feature returned %v, want ErrFeatureNotFound", err)
	}
	err = svc.Disable(ctx, "nonexistent")
	if !errors.Is(err, ErrFeatureNotFound) {
		t.Fatalf("Disable on nonexistent feature returned %v, want ErrFeatureNotFound", err)
	}
}

func TestFeatureBaseSHAAndParentRefImmutable(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)

	_, err := svc.Create(ctx, "feat-immut", "refs/heads/feature-a", "sha-initial")
	if err != nil {
		t.Fatalf("initial Create: %v", err)
	}

	// Changing parent ref for same feature ID must fail
	_, err = svc.Create(ctx, "feat-immut", "refs/heads/feature-b", "sha-initial")
	if !errors.Is(err, ErrFeatureImmutable) {
		t.Fatalf("Create with changed parentRef returned %v, want ErrFeatureImmutable", err)
	}

	// Changing base SHA for same feature ID must fail
	_, err = svc.Create(ctx, "feat-immut", "refs/heads/feature-a", "sha-different")
	if !errors.Is(err, ErrFeatureImmutable) {
		t.Fatalf("Create with changed baseSHA returned %v, want ErrFeatureImmutable", err)
	}

	// Same ID, same parentRef, same baseSHA succeeds idempotently
	feat, err := svc.Create(ctx, "feat-immut", "refs/heads/feature-a", "sha-initial")
	if err != nil {
		t.Fatalf("idempotent Create failed: %v", err)
	}
	if feat.ID != "feat-immut" || feat.ParentRef != "refs/heads/feature-a" || feat.BaseSHA != "sha-initial" {
		t.Errorf("unexpected feature returned: %+v", feat)
	}
}

func TestFeatureCreateRejectsMainAndLucindNamespaces(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)

	invalidRefs := []struct {
		name string
		ref  string
	}{
		{"literal main", "main"},
		{"refs heads main", "refs/heads/main"},
		{"lucind prefix", "lucind/lane-1"},
		{"refs heads lucind prefix", "refs/heads/lucind/lane-1"},
		{"empty ref", ""},
	}

	for _, tt := range invalidRefs {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Create(ctx, "feat-"+filepath.Base(tt.name), tt.ref, "sha123")
			if !errors.Is(err, ErrInvalidParentRef) {
				t.Fatalf("Create with ref %q error = %v, want ErrInvalidParentRef", tt.ref, err)
			}
		})
	}
}

func TestLeaseAcquisitionAndMonotonicFence(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)

	// 1. Initial acquisition on non-existent lease -> fence 1
	l1, err := svc.AcquireLease(ctx, "feat-lease-1", "worker-1", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("AcquireLease 1 failed: %v", err)
	}
	if l1.Fence != 1 {
		t.Errorf("l1.Fence = %d, want 1", l1.Fence)
	}
	if l1.Owner != "worker-1" {
		t.Errorf("l1.Owner = %q, want worker-1", l1.Owner)
	}

	// 2. Active lease cannot be acquired by another worker
	_, err = svc.AcquireLease(ctx, "feat-lease-1", "worker-2", 100*time.Millisecond)
	if !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("AcquireLease while active returned %v, want ErrLeaseHeld", err)
	}

	// 3. Wait for lease to expire
	time.Sleep(120 * time.Millisecond)

	// 4. Acquisition of expired lease succeeds and increments fence monotonically (1 -> 2)
	l2, err := svc.AcquireLease(ctx, "feat-lease-1", "worker-2", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("AcquireLease 2 failed: %v", err)
	}
	if l2.Fence != 2 {
		t.Errorf("l2.Fence = %d, want 2", l2.Fence)
	}
	if l2.Owner != "worker-2" {
		t.Errorf("l2.Owner = %q, want worker-2", l2.Owner)
	}
}

func TestLeaseValidationAndStaleMutationRejection(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)

	// Worker 1 acquires lease (fence 1) with short TTL
	l1, err := svc.AcquireLease(ctx, "feat-lease-stale", "worker-1", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("AcquireLease worker-1: %v", err)
	}

	// Validate valid lease
	if err := svc.ValidateLease(ctx, "feat-lease-stale", l1.Owner, l1.Fence); err != nil {
		t.Fatalf("ValidateLease valid token failed: %v", err)
	}

	// Wait for expiration
	time.Sleep(60 * time.Millisecond)

	// Validate expired lease returns ErrLeaseExpired
	err = svc.ValidateLease(ctx, "feat-lease-stale", l1.Owner, l1.Fence)
	if !errors.Is(err, ErrLeaseExpired) {
		t.Fatalf("ValidateLease expired token returned %v, want ErrLeaseExpired", err)
	}

	// Worker 2 acquires lease (fence 2)
	l2, err := svc.AcquireLease(ctx, "feat-lease-stale", "worker-2", 1*time.Second)
	if err != nil {
		t.Fatalf("AcquireLease worker-2: %v", err)
	}
	if l2.Fence != 2 {
		t.Fatalf("l2.Fence = %d, want 2", l2.Fence)
	}

	// Worker 1 attempts mutation with superseded fence 1 -> rejected with ErrStaleLease
	err = svc.ValidateLease(ctx, "feat-lease-stale", "worker-1", l1.Fence)
	if !errors.Is(err, ErrStaleLease) {
		t.Fatalf("ValidateLease superseded fence returned %v, want ErrStaleLease", err)
	}

	// Worker 1 attempts renewal with superseded fence 1 -> rejected with ErrStaleLease
	_, err = svc.RenewLease(ctx, "feat-lease-stale", "worker-1", l1.Fence, 1*time.Second)
	if !errors.Is(err, ErrStaleLease) {
		t.Fatalf("RenewLease superseded fence returned %v, want ErrStaleLease", err)
	}

	// Worker 1 attempts release with superseded fence 1 -> rejected with ErrStaleLease
	err = svc.ReleaseLease(ctx, "feat-lease-stale", "worker-1", l1.Fence)
	if !errors.Is(err, ErrStaleLease) {
		t.Fatalf("ReleaseLease superseded fence returned %v, want ErrStaleLease", err)
	}

	// Worker 2 renewal succeeds
	renewed, err := svc.RenewLease(ctx, "feat-lease-stale", "worker-2", l2.Fence, 2*time.Second)
	if err != nil {
		t.Fatalf("RenewLease worker-2 failed: %v", err)
	}
	if renewed.Fence != 2 || renewed.Owner != "worker-2" {
		t.Errorf("unexpected renewed lease: %+v", renewed)
	}

	// Worker 2 release succeeds
	if err := svc.ReleaseLease(ctx, "feat-lease-stale", "worker-2", l2.Fence); err != nil {
		t.Fatalf("ReleaseLease worker-2 failed: %v", err)
	}

	// After release, immediate acquire by worker 3 succeeds with fence 3
	l3, err := svc.AcquireLease(ctx, "feat-lease-stale", "worker-3", 1*time.Second)
	if err != nil {
		t.Fatalf("AcquireLease worker-3 after release failed: %v", err)
	}
	if l3.Fence != 3 {
		t.Errorf("l3.Fence after release = %d, want 3", l3.Fence)
	}
}

func TestConcurrentLeaseAcquisition(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	// Part (a): Cross-feature concurrency: acquisitions for distinct features do not block or interfere
	const numFeatures = 8
	var wgFeatures sync.WaitGroup
	featureErrs := make(chan error, numFeatures)

	for i := 0; i < numFeatures; i++ {
		wgFeatures.Add(1)
		featID := fmt.Sprintf("feat-concurrent-%d", i)
		owner := fmt.Sprintf("worker-%d", i)
		go func(fID, own string) {
			defer wgFeatures.Done()
			lHandle, err := ledger.Open(ctx, root)
			if err != nil {
				featureErrs <- fmt.Errorf("open ledger handle: %w", err)
				return
			}
			defer lHandle.Close()
			svcHandle := NewService(lHandle)

			lease, err := svcHandle.AcquireLease(ctx, fID, own, 5*time.Second)
			if err != nil {
				featureErrs <- fmt.Errorf("acquire distinct feature %s: %w", fID, err)
				return
			}
			if lease.Fence != 1 || lease.Owner != own {
				featureErrs <- fmt.Errorf("unexpected lease for %s: %+v", fID, lease)
				return
			}
		}(featID, owner)
	}

	wgFeatures.Wait()
	close(featureErrs)
	for err := range featureErrs {
		t.Errorf("cross-feature acquisition error: %v", err)
	}

	// Part (b): Same-feature concurrency: concurrent attempts for the SAME feature serialize correctly
	// Exactly one wins initially; losers receive ErrLeaseHeld; no double grant, no lost update.
	const numCompetitors = 8
	const sharedFeature = "feat-shared-contention"
	var wgSame sync.WaitGroup
	winners := make(chan Lease, numCompetitors)
	losers := make(chan error, numCompetitors)

	for i := 0; i < numCompetitors; i++ {
		wgSame.Add(1)
		owner := fmt.Sprintf("competitor-%d", i)
		go func(own string) {
			defer wgSame.Done()
			lHandle, err := ledger.Open(ctx, root)
			if err != nil {
				losers <- fmt.Errorf("open ledger: %w", err)
				return
			}
			defer lHandle.Close()
			svcHandle := NewService(lHandle)

			lease, err := svcHandle.AcquireLease(ctx, sharedFeature, own, 5*time.Second)
			if err != nil {
				losers <- err
				return
			}
			winners <- lease
		}(owner)
	}

	wgSame.Wait()
	close(winners)
	close(losers)

	var wonLeases []Lease
	for l := range winners {
		wonLeases = append(wonLeases, l)
	}
	if len(wonLeases) != 1 {
		t.Fatalf("expected exactly 1 winner for concurrent initial acquisition, got %d (%+v)", len(wonLeases), wonLeases)
	}
	if wonLeases[0].Fence != 1 {
		t.Errorf("winning fence = %d, want 1", wonLeases[0].Fence)
	}

	var heldCount int
	for err := range losers {
		if errors.Is(err, ErrLeaseHeld) {
			heldCount++
		} else {
			t.Errorf("unexpected loser error: %v", err)
		}
	}
	if heldCount != numCompetitors-1 {
		t.Errorf("heldCount = %d, want %d", heldCount, numCompetitors-1)
	}
}

func TestExportedHelpersAndEdgeCases(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)

	// Status.Valid
	if !StatusCreated.Valid() || !StatusActive.Valid() || !StatusDisabled.Valid() {
		t.Error("expected Status constants to be valid")
	}
	if Status("unknown").Valid() {
		t.Error("expected bogus status to be invalid")
	}

	// Create parameter validation
	if _, err := svc.Create(ctx, "", "refs/heads/foo", "sha1"); !errors.Is(err, ErrFeatureIDRequired) {
		t.Errorf("Create with empty ID returned %v, want ErrFeatureIDRequired", err)
	}
	if _, err := svc.Create(ctx, "id", "refs/heads/foo", ""); !errors.Is(err, ErrBaseSHARequired) {
		t.Errorf("Create with empty baseSHA returned %v, want ErrBaseSHARequired", err)
	}

	// Lease parameter validation
	if _, err := svc.AcquireLease(ctx, "", "worker", time.Minute); !errors.Is(err, ErrFeatureIDMissing) {
		t.Errorf("AcquireLease empty featureID returned %v, want ErrFeatureIDMissing", err)
	}
	if _, err := svc.AcquireLease(ctx, "feat", "", time.Minute); !errors.Is(err, ErrOwnerMissing) {
		t.Errorf("AcquireLease empty owner returned %v, want ErrOwnerMissing", err)
	}
	if _, err := svc.RenewLease(ctx, "", "worker", 1, time.Minute); !errors.Is(err, ErrFeatureIDMissing) {
		t.Errorf("RenewLease empty featureID returned %v, want ErrFeatureIDMissing", err)
	}
	if _, err := svc.RenewLease(ctx, "feat", "", 1, time.Minute); !errors.Is(err, ErrOwnerMissing) {
		t.Errorf("RenewLease empty owner returned %v, want ErrOwnerMissing", err)
	}
	if err := svc.ReleaseLease(ctx, "", "worker", 1); !errors.Is(err, ErrFeatureIDMissing) {
		t.Errorf("ReleaseLease empty featureID returned %v, want ErrFeatureIDMissing", err)
	}
	if err := svc.ReleaseLease(ctx, "feat", "", 1); !errors.Is(err, ErrOwnerMissing) {
		t.Errorf("ReleaseLease empty owner returned %v, want ErrOwnerMissing", err)
	}

	// Nonexistent lease operations return ErrLeaseNotFound
	if _, err := svc.RenewLease(ctx, "nonexistent", "worker", 1, time.Minute); !errors.Is(err, ErrLeaseNotFound) {
		t.Errorf("RenewLease nonexistent returned %v, want ErrLeaseNotFound", err)
	}
	if err := svc.ReleaseLease(ctx, "nonexistent", "worker", 1); !errors.Is(err, ErrLeaseNotFound) {
		t.Errorf("ReleaseLease nonexistent returned %v, want ErrLeaseNotFound", err)
	}
	if err := svc.ValidateLease(ctx, "nonexistent", "worker", 1); !errors.Is(err, ErrLeaseNotFound) {
		t.Errorf("ValidateLease nonexistent returned %v, want ErrLeaseNotFound", err)
	}
	if _, err := svc.GetLease(ctx, "nonexistent"); !errors.Is(err, ErrLeaseNotFound) {
		t.Errorf("GetLease nonexistent returned %v, want ErrLeaseNotFound", err)
	}

	// GetLease and Lease.Valid
	l, err := svc.AcquireLease(ctx, "feat-get-lease", "worker-get", time.Hour)
	if err != nil {
		t.Fatalf("AcquireLease: %v", err)
	}
	gotL, err := svc.GetLease(ctx, "feat-get-lease")
	if err != nil {
		t.Fatalf("GetLease: %v", err)
	}
	if gotL.Fence != l.Fence || gotL.Owner != l.Owner {
		t.Errorf("gotL = %+v, want %+v", gotL, l)
	}
	if !gotL.Valid(time.Now().UTC()) {
		t.Error("expected gotL to be valid now")
	}
	if gotL.Valid(time.Now().UTC().Add(2 * time.Hour)) {
		t.Error("expected gotL to be invalid in 2 hours")
	}
}
