package run_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/barrier"
	"github.com/LanzerDevCorp/lucind-ai/internal/feature"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/overlap"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
	"github.com/LanzerDevCorp/lucind-ai/internal/run"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

// featurePacket builds a packet that names a full feature target, which is
// what makes a batch route to the durable attempt state machine instead of
// the legacy ff-merge integration.
func featurePacket(id, featureID string) packet.Packet {
	return packet.Packet{
		ID:                id,
		Executor:          "agy",
		RoutedBy:          "test",
		Feature:           featureID,
		ParentRef:         "refs/heads/feature-" + featureID,
		BaseSHA:           "base-sha-common",
		ExpectedParentSHA: "expected-sha-refs/heads/feature-" + featureID,
	}
}

func legacyPacket(id string) packet.Packet {
	return packet.Packet{
		ID:                id,
		Executor:          "agy",
		RoutedBy:          "test",
		LegacyMain:        true,
		ExpectedParentSHA: "main-sha",
	}
}

func TestFeatureTargetLegacyBatchNamesNoFeature(t *testing.T) {
	_, ok, err := run.FeatureTarget([]packet.Packet{legacyPacket("a"), legacyPacket("b")})
	if err != nil {
		t.Fatalf("FeatureTarget(legacy batch) error = %v, want nil", err)
	}
	if ok {
		t.Errorf("FeatureTarget(legacy batch) ok = true, want false: a legacy batch promotes through Integrate, not an attempt")
	}
}

func TestFeatureTargetHomogeneousBatchNamesTheFeature(t *testing.T) {
	target, ok, err := run.FeatureTarget([]packet.Packet{
		featurePacket("a", "feat-alpha"),
		featurePacket("b", "feat-alpha"),
	})
	if err != nil {
		t.Fatalf("FeatureTarget(homogeneous batch) error = %v, want nil", err)
	}
	if !ok {
		t.Fatalf("FeatureTarget(homogeneous batch) ok = false, want true")
	}
	if target.FeatureID != "feat-alpha" {
		t.Errorf("FeatureID = %q, want %q", target.FeatureID, "feat-alpha")
	}
	if target.ParentRef != "refs/heads/feature-feat-alpha" {
		t.Errorf("ParentRef = %q, want %q", target.ParentRef, "refs/heads/feature-feat-alpha")
	}
	if target.BaseSHA != "base-sha-common" {
		t.Errorf("BaseSHA = %q, want %q", target.BaseSHA, "base-sha-common")
	}
	if target.ExpectedParentSHA != "expected-sha-refs/heads/feature-feat-alpha" {
		t.Errorf("ExpectedParentSHA = %q, want %q", target.ExpectedParentSHA, "expected-sha-refs/heads/feature-feat-alpha")
	}
}

// A batch promotes onto exactly one parent ref. Two features in one batch has
// no correct answer -- promoting onto either one silently drops the other's
// lanes onto a parent their author never named -- so it is rejected before
// any lane dispatches rather than resolved by picking the first.
func TestFeatureTargetMixedFeaturesRejected(t *testing.T) {
	tests := []struct {
		name string
		ps   []packet.Packet
	}{
		{
			name: "two different features",
			ps:   []packet.Packet{featurePacket("a", "feat-alpha"), featurePacket("b", "feat-beta")},
		},
		{
			name: "legacy mixed with a feature target",
			ps:   []packet.Packet{legacyPacket("a"), featurePacket("b", "feat-alpha")},
		},
		{
			name: "same feature, divergent expected parent sha",
			ps: func() []packet.Packet {
				p := featurePacket("b", "feat-alpha")
				p.ExpectedParentSHA = "some-other-sha"
				return []packet.Packet{featurePacket("a", "feat-alpha"), p}
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := run.FeatureTarget(tt.ps)
			if !errors.Is(err, run.ErrMixedFeatureTargets) {
				t.Errorf("FeatureTarget() error = %v, want ErrMixedFeatureTargets", err)
			}
		})
	}
}

type featureIntegrateFixtures struct {
	removedWorktrees []string
	featSvc          *feature.Service
}

// A packet naming a feature whose parent is main is not a feature dispatch --
// that is exactly what legacy mode is for -- and feature.Create refuses it. It
// must be refused before the lanes run, not after their quota is spent.
func TestFeatureTargetRejectsUnusableParentRef(t *testing.T) {
	for _, ref := range []string{"refs/heads/main", "main", "lucind/lane-1", "refs/heads/lucind/lane-1", ""} {
		t.Run(ref, func(t *testing.T) {
			p := featurePacket("a", "feat-alpha")
			p.ParentRef = ref
			_, _, err := run.FeatureTarget([]packet.Packet{p})
			if !errors.Is(err, feature.ErrInvalidParentRef) {
				t.Errorf("FeatureTarget(parent_ref=%q) error = %v, want feature.ErrInvalidParentRef", ref, err)
			}
		})
	}
}

// A packet that declares no dispatch target at all is the reusable-template
// shape: the orchestrator is meant to supply the target at run time. Dispatched
// without it, every lane would be rejected by admission one at a time, after
// its worktree existed. FeatureTarget names both exits up front instead.
func TestFeatureTargetRejectsPacketWithNoDeclaredTarget(t *testing.T) {
	p := packet.Packet{ID: "a", Executor: "agy", RoutedBy: "test"}

	_, _, err := run.FeatureTarget([]packet.Packet{p})
	if !errors.Is(err, run.ErrMissingFeatureTarget) {
		t.Fatalf("FeatureTarget(target-less packet) error = %v, want ErrMissingFeatureTarget", err)
	}
	if !strings.Contains(err.Error(), "--legacy-main") {
		t.Errorf("error = %q, want it to name the --legacy-main exit", err)
	}
}

func newFeatureIntegrateDeps(t *testing.T, spies *gateSpies) (run.Deps, *ledger.Ledger, *featureIntegrateFixtures) {
	t.Helper()

	deps, l, featSvc := newGateTestDeps(t, spies)
	fx := &featureIntegrateFixtures{featSvc: featSvc}

	deps.PersistEnvelope = func(ctx context.Context, primaryRoot, laneID string, env *result.Envelope) error {
		return nil
	}
	deps.RemoveLaneWorktree = func(ctx context.Context, primaryRoot, worktreePath, branch string) error {
		fx.removedWorktrees = append(fx.removedWorktrees, worktreePath)
		return nil
	}

	return deps, l, fx
}

func batchWith(runID string, laneIDs []string, released bool) run.BatchReport {
	lanes := make([]run.Report, 0, len(laneIDs))
	for _, id := range laneIDs {
		lanes = append(lanes, run.Report{
			LaneID:   id,
			Status:   lane.Done,
			Worktree: "/wt/" + id,
		})
	}
	return run.BatchReport{
		RunID:    runID,
		Lanes:    lanes,
		Released: released,
		Outcome:  barrier.Outcome{Integrate: laneIDs},
	}
}

func TestIntegrateFeatureNothingToIntegrate(t *testing.T) {
	deps, _, _ := newFeatureIntegrateDeps(t, &gateSpies{})

	rep, att, err := run.IntegrateFeature(context.Background(), deps, run.BatchReport{RunID: deps.RunID}, run.AttemptRequest{})
	if err != nil {
		t.Fatalf("IntegrateFeature() error = %v", err)
	}
	if rep.Attempted {
		t.Errorf("Attempted = true, want false when the barrier never released")
	}
	if att.ID != "" {
		t.Errorf("att.ID = %q, want empty: no attempt should be recorded for an empty batch", att.ID)
	}
}

// The promoted path must land exactly where the legacy path lands: envelopes
// persisted, lane worktrees removed, report Passed with every lane listed as
// integrated. Reporting a promotion while leaving lane worktrees on disk would
// leak a worktree per lane per run.
func TestIntegrateFeaturePromotedCompletesIntegration(t *testing.T) {
	deps, l, fx := newFeatureIntegrateDeps(t, &gateSpies{})

	feat, err := fx.featSvc.Create(context.Background(), "feat-alpha", "refs/heads/feature-alpha", "base-sha-common", "expected-sha-refs/heads/feature-alpha")
	if err != nil {
		t.Fatalf("featSvc.Create() error = %v", err)
	}

	batch := batchWith(deps.RunID, []string{"lane-1", "lane-2"}, true)
	req := run.AttemptRequest{
		ID:                "att-1",
		FeatureID:         feat.ID,
		ParentRef:         feat.ParentRef,
		BaseSHA:           feat.BaseSHA,
		ExpectedParentSHA: feat.ExpectedParentSHA,
		IdempotencyKey:    deps.RunID,
		Owner:             "run",
	}

	rep, att, err := run.IntegrateFeature(context.Background(), deps, batch, req)
	if err != nil {
		t.Fatalf("IntegrateFeature() error = %v", err)
	}
	if att.Status != run.AttemptStatusPromoted {
		t.Fatalf("att.Status = %q, want %q (failure reason: %s)", att.Status, run.AttemptStatusPromoted, att.FailureReason)
	}
	if !rep.Attempted || !rep.Passed {
		t.Errorf("Attempted/Passed = %v/%v, want true/true", rep.Attempted, rep.Passed)
	}
	if len(rep.Integrated) != 2 {
		t.Errorf("Integrated = %v, want both lanes", rep.Integrated)
	}
	if len(rep.Reverted) != 0 {
		t.Errorf("Reverted = %v, want none", rep.Reverted)
	}
	if len(fx.removedWorktrees) != 2 {
		t.Errorf("removed lane worktrees = %v, want one per integrated lane", fx.removedWorktrees)
	}

	// The attempt is durable: it is the row `lucind-ai feature recover
	// --attempt <id>` resumes, and until this wiring existed nothing wrote one.
	stored, err := run.GetAttempt(context.Background(), l, "att-1")
	if err != nil {
		t.Fatalf("GetAttempt() error = %v", err)
	}
	if stored.Status != run.AttemptStatusPromoted {
		t.Errorf("stored attempt status = %q, want %q", stored.Status, run.AttemptStatusPromoted)
	}
}

// A required-overlap block is not a crash and not a success: the lanes must be
// demoted in the ledger with the block as their reason, so the next reader sees
// why nothing was promoted instead of finding lanes still marked done.
func TestIntegrateFeatureBlockedRevertsLanes(t *testing.T) {
	spies := &gateSpies{
		evaluateOverlapFunc: func(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...overlap.EvaluateOption) (*overlap.Evidence, error) {
			return &overlap.Evidence{
				Version:     "v1",
				BaseSHA:     baseSHA,
				FeatureASHA: shaA,
				FeatureBSHA: shaB,
				Class:       overlap.ClassRequired,
				Rationale:   []string{"predicted Git merge conflict detected in file.go"},
				Signals: overlap.Signals{
					PredictedConflict: true,
					ConflictPaths:     []string{"file.go"},
				},
			}, nil
		},
	}
	deps, l, fx := newFeatureIntegrateDeps(t, spies)

	featA, err := fx.featSvc.Create(context.Background(), "feat-alpha", "refs/heads/feature-alpha", "base-sha-common", "expected-sha-refs/heads/feature-alpha")
	if err != nil {
		t.Fatalf("featSvc.Create(alpha) error = %v", err)
	}
	if _, err := fx.featSvc.Create(context.Background(), "feat-beta", "refs/heads/feature-beta", "base-sha-common", "expected-sha-refs/heads/feature-beta"); err != nil {
		t.Fatalf("featSvc.Create(beta) error = %v", err)
	}

	if err := l.RegisterLane(context.Background(), ledger.Lane{
		RunID: deps.RunID, LaneID: "lane-1", PacketID: "lane-1",
		Executor: "agy", RoutingCondition: "test", Status: lane.Done,
	}); err != nil {
		t.Fatalf("RegisterLane() error = %v", err)
	}

	batch := batchWith(deps.RunID, []string{"lane-1"}, true)
	rep, att, err := run.IntegrateFeature(context.Background(), deps, batch, run.AttemptRequest{
		ID:                "att-blocked-1",
		FeatureID:         featA.ID,
		ParentRef:         featA.ParentRef,
		BaseSHA:           featA.BaseSHA,
		ExpectedParentSHA: featA.ExpectedParentSHA,
		IdempotencyKey:    deps.RunID,
	})
	if err != nil {
		t.Fatalf("IntegrateFeature() error = %v", err)
	}
	if att.Status != run.AttemptStatusBlocked {
		t.Fatalf("att.Status = %q, want %q", att.Status, run.AttemptStatusBlocked)
	}
	if rep.Passed {
		t.Errorf("Passed = true, want false on a blocked attempt")
	}
	if len(rep.Reverted) != 1 || rep.Reverted[0] != "lane-1" {
		t.Errorf("Reverted = %v, want [lane-1]", rep.Reverted)
	}
	if rep.Reason == "" {
		t.Errorf("Reason is empty, want the attempt failure reason")
	}
	if len(fx.removedWorktrees) != 0 {
		t.Errorf("removed lane worktrees = %v, want none preserved-for-inspection lanes removed", fx.removedWorktrees)
	}

	var status string
	if err := l.DB().QueryRowContext(context.Background(),
		`SELECT status FROM lanes WHERE run_id = ? AND lane_id = ?`, deps.RunID, "lane-1").Scan(&status); err != nil {
		t.Fatalf("query lane status: %v", err)
	}
	if status != string(lane.Blocked) {
		t.Errorf("lane status = %q, want %q", status, lane.Blocked)
	}
}

// A feature lane must start from the feature's own base, not from whatever the
// primary checkout has checked out. Promoting a candidate built on an
// unrelated base onto the feature's parent ref is a silent wrong-base merge:
// the CAS succeeds and the tree is not what anyone asked for.
func TestExecuteCreatesFeatureWorktreeFromPacketBase(t *testing.T) {
	tests := []struct {
		name          string
		pkt           packet.Packet
		wantParentRef string
		wantBaseSHA   string
	}{
		{
			name:          "feature lane carries its parent and base",
			pkt:           featurePacket("lane-1", "feat-alpha"),
			wantParentRef: "refs/heads/feature-feat-alpha",
			wantBaseSHA:   "base-sha-common",
		},
		{
			// Legacy dispatch keeps today's exact behavior: no start point, so
			// git branches the lane from the primary checkout's HEAD.
			name:          "legacy lane carries neither",
			pkt:           legacyPacket("lane-1"),
			wantParentRef: "",
			wantBaseSHA:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps, _, _ := newFeatureIntegrateDeps(t, &gateSpies{})

			var gotParentRef, gotBaseSHA string
			deps.CreateWorktree = func(ctx context.Context, primaryRoot, laneID, parentRef, baseSHA string) (worktree.Worktree, error) {
				gotParentRef, gotBaseSHA = parentRef, baseSHA
				return worktree.Worktree{}, errStopAfterWorktree
			}

			_, _ = run.Execute(context.Background(), deps, tt.pkt)

			if gotParentRef != tt.wantParentRef {
				t.Errorf("CreateWorktree parentRef = %q, want %q", gotParentRef, tt.wantParentRef)
			}
			if gotBaseSHA != tt.wantBaseSHA {
				t.Errorf("CreateWorktree baseSHA = %q, want %q", gotBaseSHA, tt.wantBaseSHA)
			}
		})
	}
}

// errStopAfterWorktree ends Execute right after the call under test, so the
// assertion does not depend on anything downstream of worktree creation.
var errStopAfterWorktree = errors.New("stop after worktree creation")
