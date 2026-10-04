package dispatch_test

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/dispatch"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
)

func TestWait_HerdrEnvMissing(t *testing.T) {
	t.Setenv("HERDR_ENV", "")
	_, _, err := dispatch.Wait(context.Background(), "", "20261003-120000-abcd", time.Second, nil)
	if err == nil {
		t.Fatal("expected error when HERDR_ENV is missing, got nil")
	}
	if !strings.Contains(err.Error(), "herdr is the only supported runtime") {
		t.Errorf("error %q should mention herdr runtime requirement", err.Error())
	}
}

func TestWait_Done(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(context.Background(), repoDir, []string{"**"}, "gemini-3.7-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	l.PaneID = "w1:pWait1"
	if err := l.Save(repoDir); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	var pollCount int32
	dispatch.SetSleepForTesting(func(d time.Duration) {
		count := atomic.AddInt32(&pollCount, 1)
		if count >= 2 {
			current, _ := lane.Load(repoDir, l.ID)
			current.Status = lane.StatusDone
			_ = current.Save(repoDir)
		}
	}, 1*time.Millisecond)
	defer dispatch.ResetSleepForTesting()

	runner := newFakeHerdrRunner()
	out, exitCode, err := dispatch.Wait(context.Background(), repoDir, l.ID, 5*time.Second, runner)
	if err != nil {
		t.Fatalf("unexpected Wait error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}
	if out.Status != "done" {
		t.Errorf("out.Status = %q, want \"done\"", out.Status)
	}
	if out.Lane != l.ID {
		t.Errorf("out.Lane = %q, want %q", out.Lane, l.ID)
	}
	if out.PaneID != "w1:pWait1" {
		t.Errorf("out.PaneID = %q, want \"w1:pWait1\"", out.PaneID)
	}
	if out.ResultPath == "" {
		t.Error("out.ResultPath should not be empty")
	}
}

func TestWait_Failed(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(context.Background(), repoDir, []string{"**"}, "gemini-3.7-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	l.PaneID = "w1:pWait2"
	if err := l.Save(repoDir); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	var pollCount int32
	dispatch.SetSleepForTesting(func(d time.Duration) {
		count := atomic.AddInt32(&pollCount, 1)
		if count >= 2 {
			current, _ := lane.Load(repoDir, l.ID)
			current.Status = lane.StatusFailed
			_ = current.Save(repoDir)
		}
	}, 1*time.Millisecond)
	defer dispatch.ResetSleepForTesting()

	runner := newFakeHerdrRunner()
	out, exitCode, err := dispatch.Wait(context.Background(), repoDir, l.ID, 5*time.Second, runner)
	if err != nil {
		t.Fatalf("unexpected Wait error: %v", err)
	}
	if exitCode != 3 {
		t.Errorf("exitCode = %d, want 3 for failed lane", exitCode)
	}
	if out.Status != "failed" {
		t.Errorf("out.Status = %q, want \"failed\"", out.Status)
	}
}

func TestWait_Timeout(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(context.Background(), repoDir, []string{"**"}, "gemini-3.7-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	l.PaneID = "w1:pWait3"
	if err := l.Save(repoDir); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	// Fake sleep advances time or triggers timeout
	ctx, cancel := context.WithCancel(context.Background())
	var pollCount int32
	dispatch.SetSleepForTesting(func(d time.Duration) {
		count := atomic.AddInt32(&pollCount, 1)
		if count >= 3 {
			cancel()
		}
	}, 1*time.Millisecond)
	defer dispatch.ResetSleepForTesting()

	runner := newFakeHerdrRunner()
	out, exitCode, err := dispatch.Wait(ctx, repoDir, l.ID, 1*time.Hour, runner)
	if err != nil {
		t.Fatalf("unexpected Wait error: %v", err)
	}
	if exitCode != 4 {
		t.Errorf("exitCode = %d, want 4 for timeout", exitCode)
	}
	if out.Status != "timeout" {
		t.Errorf("out.Status = %q, want \"timeout\"", out.Status)
	}

	// Verify lane.json status updated to timeout
	reloaded, err := lane.Load(repoDir, l.ID)
	if err != nil {
		t.Fatalf("lane.Load failed: %v", err)
	}
	if reloaded.Status != lane.StatusTimeout {
		t.Errorf("reloaded.Status = %q, want %q", reloaded.Status, lane.StatusTimeout)
	}

	// Verify NO runner calls were made (specifically no close or kill)
	calls := runner.Calls()
	for _, call := range calls {
		joined := strings.Join(call, " ")
		if strings.Contains(joined, "close") || strings.Contains(joined, "kill") {
			t.Errorf("forbidden command on timeout: %s", joined)
		}
	}
}

func TestDispatch_BlockingWait_TimeoutNeverClosesPanes(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	runner := setupFakeRunnerForNewLane(t, "w1:pBlocking")

	ctx, cancel := context.WithCancel(context.Background())
	var pollCount int32
	dispatch.SetSleepForTesting(func(d time.Duration) {
		count := atomic.AddInt32(&pollCount, 1)
		if count >= 2 {
			cancel()
		}
	}, 1*time.Millisecond)
	defer dispatch.ResetSleepForTesting()

	opts := dispatch.Options{
		Cwd:     repoDir,
		Brief:   "Test timeout",
		Detach:  false,
		Timeout: 10 * time.Minute,
	}

	out, exitCode, err := dispatch.Dispatch(ctx, opts, runner)
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if exitCode != 4 {
		t.Errorf("exitCode = %d, want 4", exitCode)
	}
	if out.Status != "timeout" {
		t.Errorf("out.Status = %q, want \"timeout\"", out.Status)
	}

	calls := runner.Calls()
	for _, call := range calls {
		joined := strings.Join(call, " ")
		if strings.Contains(joined, "close") || strings.Contains(joined, "kill") {
			t.Errorf("forbidden command during blocking wait timeout: %s", joined)
		}
	}
}
