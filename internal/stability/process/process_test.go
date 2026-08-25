package process_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/stability/process"
)

func writeTestStub(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "stub.sh")
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("WriteFile(stub.sh) error = %v", err)
	}
	return path
}

func TestKillGroupRejectsInvalidPGID(t *testing.T) {
	for _, invalidPGID := range []int{0, 1, -1, -42} {
		err := process.KillGroup(invalidPGID)
		if !errors.Is(err, process.ErrInvalidPGID) {
			t.Errorf("KillGroup(%d) error = %v, want ErrInvalidPGID", invalidPGID, err)
		}
	}
}

func TestAuditSurvivorsRejectsInvalidPGID(t *testing.T) {
	for _, invalidPGID := range []int{0, 1, -1, -42} {
		_, err := process.AuditSurvivors(invalidPGID)
		if !errors.Is(err, process.ErrInvalidPGID) {
			t.Errorf("AuditSurvivors(%d) error = %v, want ErrInvalidPGID", invalidPGID, err)
		}
	}
}

// TestProcessGroupKillAndProcSurvivorAudit proves that an externally-triggered
// SIGKILL to -pgid terminates all descendant processes in the group (parent +
// backgrounded grandchild), and that /proc survivor audit detects zero survivors.
// The kill signal is delivered from an independent external goroutine, without
// canceling the context of the dispatch goroutine.
func TestProcessGroupKillAndProcSurvivorAudit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	tempDir := t.TempDir()
	grandchildPidFile := filepath.Join(tempDir, "grandchild.pid")

	// Stub backgrounds a grandchild that writes its PID and sleeps 30s,
	// while parent sleeps 30s.
	script := fmt.Sprintf("#!/bin/sh\n( sleep 30 ) &\necho $! > %q\nsleep 30\n", grandchildPidFile)
	stub := writeTestStub(t, script)

	dispatchCtx, dispatchCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer dispatchCancel()

	cmd := exec.CommandContext(dispatchCtx, stub)
	cmd.Dir = tempDir

	supervisor := process.NewSupervisor()
	pgid, err := supervisor.Start(cmd)
	if err != nil {
		t.Fatalf("supervisor.Start() error = %v", err)
	}

	// Wait for grandchild PID file to be written.
	var grandchildPid int
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, readErr := os.ReadFile(grandchildPidFile)
		if readErr == nil {
			pidStr := strings.TrimSpace(string(data))
			if p, parseErr := strconv.Atoi(pidStr); parseErr == nil && p > 1 {
				grandchildPid = p
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if grandchildPid == 0 {
		t.Fatalf("grandchild PID was not recorded in %s", grandchildPidFile)
	}

	// Register cleanup in case test fails early.
	t.Cleanup(func() {
		_ = process.KillGroup(pgid)
		_ = syscall.Kill(grandchildPid, syscall.SIGKILL)
	})

	// 1. Audit before kill: should detect live processes in the group.
	survivorsBefore, err := supervisor.Audit(pgid)
	if err != nil {
		t.Fatalf("supervisor.Audit() error before kill = %v", err)
	}
	if len(survivorsBefore) == 0 {
		t.Errorf("Audit() before kill found 0 survivors, want >= 1 (grandchild PID %d)", grandchildPid)
	}

	// 2. Trigger abrupt kill from an external goroutine with independent context.
	killDone := make(chan error, 1)
	go func() {
		// External kill action: completely independent of dispatchCtx.
		killErr := supervisor.Kill(pgid)
		killDone <- killErr
	}()

	select {
	case killErr := <-killDone:
		if killErr != nil {
			t.Fatalf("supervisor.Kill() from external goroutine error = %v", killErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("external supervisor.Kill() timed out")
	}

	// Wait for direct parent process to exit and be reaped.
	_ = cmd.Wait()

	// 3. Verify zero survivors in /proc.
	verifyDeadline := time.Now().Add(2 * time.Second)
	var verifyErr error
	for time.Now().Before(verifyDeadline) {
		verifyErr = supervisor.VerifyZero(pgid)
		if verifyErr == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if verifyErr != nil {
		t.Fatalf("VerifyZero() failed after kill: %v", verifyErr)
	}

	// 4. Verify specific descendant PIDs are dead via signal 0 probe.
	if err := syscall.Kill(grandchildPid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Errorf("grandchild pid %d liveness probe = %v, want ESRCH (dead)", grandchildPid, err)
	}
	if err := syscall.Kill(pgid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Errorf("parent pgid %d liveness probe = %v, want ESRCH (dead)", pgid, err)
	}

	// Dispatch context must still be un-canceled at this point to prove
	// termination was external and not triggered by dispatchCtx cancellation.
	if dispatchCtx.Err() != nil {
		t.Errorf("dispatchCtx was canceled (%v); want dispatchCtx to remain active", dispatchCtx.Err())
	}
}

func TestVerifyZeroSurvivorsReturnsErrorWhenProcessesLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	tempDir := t.TempDir()
	script := "#!/bin/sh\nsleep 30\n"
	stub := writeTestStub(t, script)

	cmd := exec.Command(stub)
	cmd.Dir = tempDir

	supervisor := process.NewSupervisor()
	pgid, err := supervisor.Start(cmd)
	if err != nil {
		t.Fatalf("supervisor.Start() error = %v", err)
	}
	defer func() {
		_ = supervisor.Kill(pgid)
		_ = cmd.Wait()
	}()

	// While process is running, VerifyZeroSurvivors must return ErrSurvivingProcesses.
	err = supervisor.VerifyZero(pgid)
	if !errors.Is(err, process.ErrSurvivingProcesses) {
		t.Errorf("VerifyZero() error = %v, want ErrSurvivingProcesses", err)
	}
}
