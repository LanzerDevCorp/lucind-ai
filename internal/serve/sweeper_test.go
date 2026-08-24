//go:build linux

package serve_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/serve"
)

func setupRunningLane(t *testing.T, pid int) (*ledger.Ledger, string, string) {
	t.Helper()
	ctx := context.Background()
	l, err := ledger.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("ledger.Open: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	now := time.Now().UTC()
	runID, laneID := "run-sweep", "lane-sweep"
	if err := l.RegisterRun(ctx, ledger.Run{
		RunID: runID, FeatureID: "f1", Status: "running", TargetRef: "main",
		LaneCount: 1, StartedAt: now, PID: pid,
	}); err != nil {
		t.Fatalf("RegisterRun: %v", err)
	}
	if err := l.RegisterLane(ctx, ledger.Lane{
		RunID: runID, LaneID: laneID, PacketID: "pkt", Executor: "agy",
		RoutingCondition: "primary", Status: lane.Running,
	}); err != nil {
		t.Fatalf("RegisterLane: %v", err)
	}
	return l, runID, laneID
}

func laneStatus(t *testing.T, l *ledger.Ledger, runID, laneID string) lane.Status {
	t.Helper()
	lanes, err := l.Lanes(context.Background(), runID)
	if err != nil {
		t.Fatalf("Lanes: %v", err)
	}
	for _, ln := range lanes {
		if ln.LaneID == laneID {
			return ln.Status
		}
	}
	t.Fatalf("lane %s not found", laneID)
	return ""
}

func hasOrphanNote(t *testing.T, l *ledger.Ledger, runID, laneID string) bool {
	t.Helper()
	events, err := l.Events(context.Background(), runID)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	for _, e := range events {
		if e.Type == ledger.EventLaneNote && e.LaneID == laneID &&
			e.Detail == "orphaned: driving process no longer running" {
			return true
		}
	}
	return false
}

func runOneSweep(t *testing.T, l *ledger.Ledger) {
	t.Helper()
	sweeper := serve.NewSweeper(l, serve.SweeperConfig{})
	if err := sweeper.SweepOnce(context.Background()); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
}

func TestSweeper_LivePIDRetained(t *testing.T) {
	l, runID, laneID := setupRunningLane(t, os.Getpid())
	runOneSweep(t, l)
	if got := laneStatus(t, l, runID, laneID); got != lane.Running {
		t.Fatalf("status = %s, want running", got)
	}
	if hasOrphanNote(t, l, runID, laneID) {
		t.Fatal("unexpected orphan note for live PID")
	}
}

func TestSweeper_DeadPIDReconciled(t *testing.T) {
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	pid := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	l, runID, laneID := setupRunningLane(t, pid)
	runOneSweep(t, l)
	if got := laneStatus(t, l, runID, laneID); got != lane.Failed {
		t.Fatalf("status = %s, want failed", got)
	}
	if !hasOrphanNote(t, l, runID, laneID) {
		t.Fatal("missing orphan EventLaneNote")
	}
}

func TestSweeper_ZeroPIDIgnored(t *testing.T) {
	l, runID, laneID := setupRunningLane(t, 0)
	runOneSweep(t, l)
	if got := laneStatus(t, l, runID, laneID); got != lane.Running {
		t.Fatalf("status = %s, want running", got)
	}
	if hasOrphanNote(t, l, runID, laneID) {
		t.Fatal("unexpected orphan note for pid 0")
	}
}

func TestSweeper_RecycledPIDAndEPERM(t *testing.T) {
	// PID 1 typically yields EPERM for non-root kill(0); treat as alive.
	proc, err := os.FindProcess(1)
	if err != nil {
		t.Fatalf("FindProcess(1): %v", err)
	}
	err = proc.Signal(syscall.Signal(0))
	if err != nil && !errors.Is(err, syscall.EPERM) {
		t.Skipf("PID 1 probe err = %v; need EPERM or nil for this case", err)
	}
	l, runID, laneID := setupRunningLane(t, 1)
	runOneSweep(t, l)
	if got := laneStatus(t, l, runID, laneID); got != lane.Running {
		t.Fatalf("EPERM/live init PID status = %s, want running", got)
	}

	// Recycled live PID (this test process) is accepted with no second identity check.
	l2, runID2, laneID2 := setupRunningLane(t, os.Getpid())
	runOneSweep(t, l2)
	if got := laneStatus(t, l2, runID2, laneID2); got != lane.Running {
		t.Fatalf("recycled live PID status = %s, want running", got)
	}
}
