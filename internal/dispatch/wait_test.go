package dispatch_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

	l, err := lane.Create(context.Background(), repoDir, []string{"**"}, "gemini-3.8-flash-high")
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
			writeResultEnvelope(t, repoDir, l.ID, "done")
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
	if !strings.HasSuffix(out.ResultPath, "result-1.json") {
		t.Errorf("out.ResultPath = %q, want ending with result-1.json", out.ResultPath)
	}
}

func writeTurnResultEnvelope(t *testing.T, repoDir, laneID string, turn int, status string) {
	t.Helper()
	dir := lane.LaneDir(repoDir, laneID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir lane dir failed: %v", err)
	}
	content := fmt.Sprintf(`{
		"lane_id": %q,
		"status": %q,
		"summary": "Lane result summary",
		"hard_stops": []
	}`, laneID, status)
	filename := lane.ResultFileName(turn)
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644); err != nil {
		t.Fatalf("write %s failed: %v", filename, err)
	}
}

func writeResultEnvelope(t *testing.T, repoDir, laneID, status string) {
	t.Helper()
	l, err := lane.Load(repoDir, laneID)
	turn := 1
	if err == nil {
		turn = l.Turn
	}
	writeTurnResultEnvelope(t, repoDir, laneID, turn, status)
}

func TestWait_Failed(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(context.Background(), repoDir, []string{"**"}, "gemini-3.8-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	l.PaneID = "w1:pWait2"
	if err := l.Save(repoDir); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	dispatch.SetExhaustionGraceForTesting(20 * time.Millisecond)
	defer dispatch.ResetExhaustionGraceForTesting()

	var pollCount int32
	dispatch.SetSleepForTesting(func(d time.Duration) {
		count := atomic.AddInt32(&pollCount, 1)
		if count >= 2 {
			current, _ := lane.Load(repoDir, l.ID)
			current.Status = lane.StatusFailed
			_ = current.Save(repoDir)
			writeResultEnvelope(t, repoDir, l.ID, "failed")
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
	if !strings.HasSuffix(out.ResultPath, "result-1.json") {
		t.Errorf("out.ResultPath = %q, want ending with result-1.json", out.ResultPath)
	}
}

func TestWait_Timeout(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(context.Background(), repoDir, []string{"**"}, "gemini-3.8-flash-high")
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

func TestWait_Failed_ValidDoneEnvelope(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(context.Background(), repoDir, []string{"**"}, "gemini-3.8-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	l.PaneID = "w1:pWaitDoneEnv"
	l.Status = lane.StatusFailed
	if err := l.Save(repoDir); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	writeResultEnvelope(t, repoDir, l.ID, "done")

	dispatch.SetExhaustionGraceForTesting(20 * time.Millisecond)
	defer dispatch.ResetExhaustionGraceForTesting()

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
	if !strings.HasSuffix(out.ResultPath, "result-1.json") {
		t.Errorf("out.ResultPath = %q, want ending with result-1.json", out.ResultPath)
	}

	reloaded, err := lane.Load(repoDir, l.ID)
	if err != nil {
		t.Fatalf("lane.Load failed: %v", err)
	}
	if reloaded.Status != lane.StatusDone {
		t.Errorf("reloaded.Status = %q, want %q", reloaded.Status, lane.StatusDone)
	}
}

func TestWait_Failed_ValidNonDoneEnvelope(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(context.Background(), repoDir, []string{"**"}, "gemini-3.8-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	l.PaneID = "w1:pWaitNonDoneEnv"
	l.Status = lane.StatusFailed
	if err := l.Save(repoDir); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	writeResultEnvelope(t, repoDir, l.ID, "failed")

	dispatch.SetExhaustionGraceForTesting(20 * time.Millisecond)
	defer dispatch.ResetExhaustionGraceForTesting()

	var pollCount int32
	dispatch.SetSleepForTesting(func(d time.Duration) {
		atomic.AddInt32(&pollCount, 1)
	}, 1*time.Millisecond)
	defer dispatch.ResetSleepForTesting()

	runner := newFakeHerdrRunner()
	out, exitCode, err := dispatch.Wait(context.Background(), repoDir, l.ID, 5*time.Second, runner)
	if err != nil {
		t.Fatalf("unexpected Wait error: %v", err)
	}
	if exitCode != 3 {
		t.Errorf("exitCode = %d, want 3", exitCode)
	}
	if out.Status != "failed" {
		t.Errorf("out.Status = %q, want \"failed\"", out.Status)
	}
	if !strings.HasSuffix(out.ResultPath, "result-1.json") {
		t.Errorf("out.ResultPath = %q, want ending with result-1.json", out.ResultPath)
	}
	if pollCount != 0 {
		t.Errorf("pollCount = %d, want 0 (should return immediately)", pollCount)
	}
}

func TestWait_Failed_NoEnvelope_EnvelopeAppears(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(context.Background(), repoDir, []string{"**"}, "gemini-3.8-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	l.PaneID = "w1:pWaitLateEnv"
	l.Status = lane.StatusFailed
	if err := l.Save(repoDir); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	dispatch.SetExhaustionGraceForTesting(5 * time.Minute)
	defer dispatch.ResetExhaustionGraceForTesting()

	var pollCount int32
	dispatch.SetSleepForTesting(func(d time.Duration) {
		count := atomic.AddInt32(&pollCount, 1)
		if count >= 2 {
			writeResultEnvelope(t, repoDir, l.ID, "done")
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
	if !strings.HasSuffix(out.ResultPath, "result-1.json") {
		t.Errorf("out.ResultPath = %q, want ending with result-1.json", out.ResultPath)
	}
	if pollCount < 2 {
		t.Errorf("pollCount = %d, want >= 2", pollCount)
	}

	reloaded, err := lane.Load(repoDir, l.ID)
	if err != nil {
		t.Fatalf("lane.Load failed: %v", err)
	}
	if reloaded.Status != lane.StatusDone {
		t.Errorf("reloaded.Status = %q, want %q", reloaded.Status, lane.StatusDone)
	}
}

func TestWait_Failed_NoEnvelope_GraceExpires(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(context.Background(), repoDir, []string{"**"}, "gemini-3.8-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	l.PaneID = "w1:pWaitGraceExpires"
	l.Status = lane.StatusFailed
	if err := l.Save(repoDir); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	dispatch.SetExhaustionGraceForTesting(10 * time.Millisecond)
	defer dispatch.ResetExhaustionGraceForTesting()

	var pollCount int32
	dispatch.SetSleepForTesting(func(d time.Duration) {
		atomic.AddInt32(&pollCount, 1)
		time.Sleep(3 * time.Millisecond)
	}, 1*time.Millisecond)
	defer dispatch.ResetSleepForTesting()

	runner := newFakeHerdrRunner()
	out, exitCode, err := dispatch.Wait(context.Background(), repoDir, l.ID, 5*time.Second, runner)
	if err != nil {
		t.Fatalf("unexpected Wait error: %v", err)
	}
	if exitCode != 3 {
		t.Errorf("exitCode = %d, want 3", exitCode)
	}
	if out.Status != "failed" {
		t.Errorf("out.Status = %q, want \"failed\"", out.Status)
	}
	if pollCount < 2 {
		t.Errorf("pollCount = %d, want >= 2 (should poll during grace)", pollCount)
	}
}

func TestWait_Failed_NoEnvelope_DeadlineCapsGrace(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(context.Background(), repoDir, []string{"**"}, "gemini-3.8-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	l.PaneID = "w1:pWaitDeadlineCaps"
	l.Status = lane.StatusFailed
	if err := l.Save(repoDir); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	// Grace is large (10 minutes), but overall wait timeout is short (5 milliseconds)
	dispatch.SetExhaustionGraceForTesting(10 * time.Minute)
	defer dispatch.ResetExhaustionGraceForTesting()

	var pollCount int32
	dispatch.SetSleepForTesting(func(d time.Duration) {
		atomic.AddInt32(&pollCount, 1)
		time.Sleep(2 * time.Millisecond)
	}, 1*time.Millisecond)
	defer dispatch.ResetSleepForTesting()

	runner := newFakeHerdrRunner()
	out, exitCode, err := dispatch.Wait(context.Background(), repoDir, l.ID, 5*time.Millisecond, runner)
	if err != nil {
		t.Fatalf("unexpected Wait error: %v", err)
	}
	if exitCode != 4 {
		t.Errorf("exitCode = %d, want 4 for timeout", exitCode)
	}
	if out.Status != "timeout" {
		t.Errorf("out.Status = %q, want \"timeout\"", out.Status)
	}

	reloaded, err := lane.Load(repoDir, l.ID)
	if err != nil {
		t.Fatalf("lane.Load failed: %v", err)
	}
	if reloaded.Status != lane.StatusTimeout {
		t.Errorf("reloaded.Status = %q, want %q", reloaded.Status, lane.StatusTimeout)
	}
}

func TestWait_Done_Turn2_MissingTurnResult_ReportsFailed(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(context.Background(), repoDir, []string{"**"}, "gemini-3.8-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	l.PaneID = "w1:pWaitTurn2DoneMissing"
	l.Turn = 2
	l.Status = lane.StatusDone
	if err := l.Save(repoDir); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	// Only turn 1 result is written; turn 2 result is missing
	writeTurnResultEnvelope(t, repoDir, l.ID, 1, "done")

	runner := newFakeHerdrRunner()
	out, exitCode, err := dispatch.Wait(context.Background(), repoDir, l.ID, 5*time.Second, runner)
	if err != nil {
		t.Fatalf("unexpected Wait error: %v", err)
	}
	if exitCode != 3 {
		t.Errorf("exitCode = %d, want 3", exitCode)
	}
	if out.Status != "failed" {
		t.Errorf("out.Status = %q, want \"failed\"", out.Status)
	}
	if !strings.HasSuffix(out.ResultPath, "result-2.json") {
		t.Errorf("out.ResultPath = %q, want ending with result-2.json", out.ResultPath)
	}
}

func TestWait_Done_Turn2_ValidDoneEnvelope(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(context.Background(), repoDir, []string{"**"}, "gemini-3.8-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	l.PaneID = "w1:pWaitTurn2DoneValid"
	l.Turn = 2
	l.Status = lane.StatusDone
	if err := l.Save(repoDir); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	writeTurnResultEnvelope(t, repoDir, l.ID, 2, "done")

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
	if !strings.HasSuffix(out.ResultPath, "result-2.json") {
		t.Errorf("out.ResultPath = %q, want ending with result-2.json", out.ResultPath)
	}
}

func TestWait_Failed_Turn2_OnlyTurn1Envelope_ReportsFailed(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(context.Background(), repoDir, []string{"**"}, "gemini-3.8-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	l.PaneID = "w1:pWaitTurn2FailedOnlyTurn1"
	l.Turn = 2
	l.Status = lane.StatusFailed
	if err := l.Save(repoDir); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	// Only turn 1 has a done envelope; turn 2 result is missing
	writeTurnResultEnvelope(t, repoDir, l.ID, 1, "done")

	dispatch.SetExhaustionGraceForTesting(10 * time.Millisecond)
	defer dispatch.ResetExhaustionGraceForTesting()

	var pollCount int32
	dispatch.SetSleepForTesting(func(d time.Duration) {
		atomic.AddInt32(&pollCount, 1)
		time.Sleep(3 * time.Millisecond)
	}, 1*time.Millisecond)
	defer dispatch.ResetSleepForTesting()

	runner := newFakeHerdrRunner()
	out, exitCode, err := dispatch.Wait(context.Background(), repoDir, l.ID, 5*time.Second, runner)
	if err != nil {
		t.Fatalf("unexpected Wait error: %v", err)
	}
	if exitCode != 3 {
		t.Errorf("exitCode = %d, want 3", exitCode)
	}
	if out.Status != "failed" {
		t.Errorf("out.Status = %q, want \"failed\"", out.Status)
	}
	if !strings.HasSuffix(out.ResultPath, "result-2.json") {
		t.Errorf("out.ResultPath = %q, want ending with result-2.json", out.ResultPath)
	}
}

func TestWait_Failed_Turn2_ValidDoneEnvelope_Recovers(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(context.Background(), repoDir, []string{"**"}, "gemini-3.8-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	l.PaneID = "w1:pWaitTurn2FailedDoneEnv"
	l.Turn = 2
	l.Status = lane.StatusFailed
	if err := l.Save(repoDir); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	writeTurnResultEnvelope(t, repoDir, l.ID, 2, "done")

	dispatch.SetExhaustionGraceForTesting(10 * time.Millisecond)
	defer dispatch.ResetExhaustionGraceForTesting()

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
	if !strings.HasSuffix(out.ResultPath, "result-2.json") {
		t.Errorf("out.ResultPath = %q, want ending with result-2.json", out.ResultPath)
	}

	reloaded, err := lane.Load(repoDir, l.ID)
	if err != nil {
		t.Fatalf("lane.Load failed: %v", err)
	}
	if reloaded.Status != lane.StatusDone {
		t.Errorf("reloaded.Status = %q, want %q", reloaded.Status, lane.StatusDone)
	}
}
