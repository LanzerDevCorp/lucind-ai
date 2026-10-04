package dispatch_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/dispatch"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Tester",
			"GIT_AUTHOR_EMAIL=tester@example.com",
			"GIT_COMMITTER_NAME=Tester",
			"GIT_COMMITTER_EMAIL=tester@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v\nOutput: %s", strings.Join(args, " "), err, string(out))
		}
	}
	run("init")
	if err := os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatalf("write initial.txt: %v", err)
	}
	run("add", "initial.txt")
	run("commit", "-m", "initial commit")
}

func setupFakeRunnerForNewLane(t *testing.T, newPaneID string) *fakeHerdrRunner {
	t.Helper()
	runner := newFakeHerdrRunner()
	runner.handlers["pane layout"] = func(args []string) ([]byte, error) {
		return []byte(`{
			"result": {
				"layout": {
					"panes": [
						{"pane_id": "w1:p0", "focused": true, "rect": {"width": 120, "height": 40}}
					]
				}
			}
		}`), nil
	}
	runner.handlers["pane split"] = func(args []string) ([]byte, error) {
		return []byte(fmt.Sprintf(`{"result": {"pane": {"pane_id": %q}}}`, newPaneID)), nil
	}
	runner.handlers["agent start"] = func(args []string) ([]byte, error) {
		return []byte(`{"result": {"started": true}}`), nil
	}
	runner.handlers["agent prompt"] = func(args []string) ([]byte, error) {
		return []byte(`{"result": {"submitted": true}}`), nil
	}
	return runner
}

func TestDispatch_HerdrEnvMissing(t *testing.T) {
	t.Setenv("HERDR_ENV", "")
	_, _, err := dispatch.Dispatch(context.Background(), dispatch.Options{}, nil)
	if err == nil {
		t.Fatal("expected error when HERDR_ENV is missing, got nil")
	}
	if !strings.Contains(err.Error(), "herdr is the only supported runtime") {
		t.Errorf("error %q should mention herdr runtime requirement", err.Error())
	}
}

func TestDispatch_QuotaRefusal(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	dir := t.TempDir()
	initGitRepo(t, dir)

	dispatch.SetEnsureAgyQuotaForTesting(func(ctx context.Context, minQuota float64) error {
		return errors.New("insufficient quota")
	})
	defer dispatch.ResetEnsureAgyQuotaForTesting()

	_, _, err := dispatch.Dispatch(context.Background(), dispatch.Options{
		Cwd:      dir,
		MinQuota: 0.20,
	}, nil)
	if err == nil {
		t.Fatal("expected error when quota refused, got nil")
	}
	if !strings.Contains(err.Error(), "insufficient quota") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestDispatch_NewLane_HappyPath(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	runner := setupFakeRunnerForNewLane(t, "w1:pLane1")

	opts := dispatch.Options{
		Cwd:      repoDir,
		Allow:    []string{"internal/**", "cmd/**"},
		Model:    "gemini-3.7-flash-high",
		Brief:    "# Implement Feature A\nPlease implement feature A carefully.",
		Detach:   true,
	}

	out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}
	if out.PaneID != "w1:pLane1" {
		t.Errorf("out.PaneID = %q, want \"w1:pLane1\"", out.PaneID)
	}
	if out.Status != "running" {
		t.Errorf("out.Status = %q, want \"running\"", out.Status)
	}
	if out.Lane == "" {
		t.Fatal("out.Lane ID should not be empty")
	}

	// 1. Verify lane saved with pane ID
	savedLane, err := lane.Load(repoDir, out.Lane)
	if err != nil {
		t.Fatalf("load lane failed: %v", err)
	}
	if savedLane.PaneID != "w1:pLane1" {
		t.Errorf("savedLane.PaneID = %q, want \"w1:pLane1\"", savedLane.PaneID)
	}
	if savedLane.Model != "gemini-3.7-flash-high" {
		t.Errorf("savedLane.Model = %q, want \"gemini-3.7-flash-high\"", savedLane.Model)
	}

	// 2. Verify brief.md content
	briefPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "brief.md")
	briefBytes, err := os.ReadFile(briefPath)
	if err != nil {
		t.Fatalf("read brief.md failed: %v", err)
	}
	briefContent := string(briefBytes)
	if !strings.Contains(briefContent, "# Implement Feature A") {
		t.Errorf("brief.md missing user brief content: %s", briefContent)
	}
	if !strings.Contains(briefContent, fmt.Sprintf("- Lane ID: %s", out.Lane)) {
		t.Errorf("brief.md missing Lane ID: %s", briefContent)
	}
	if !strings.Contains(briefContent, "- Allowed globs:\n  - internal/**\n  - cmd/**") {
		t.Errorf("brief.md missing Allowed globs: %s", briefContent)
	}
	resultPath := lane.ResultPath(repoDir, out.Lane)
	absResultPath, _ := filepath.Abs(resultPath)
	if !strings.Contains(briefContent, fmt.Sprintf("Write your result envelope to `%s` following the result schema.", absResultPath)) {
		t.Errorf("brief.md missing result path: %s", briefContent)
	}
	if !strings.Contains(briefContent, "As the final verification run exactly `lucind-ai attest run -- sh lucind-checks.sh`") {
		t.Errorf("brief.md missing verification instruction: %s", briefContent)
	}
	if !strings.Contains(briefContent, "Do not edit outside the allowed globs.") {
		t.Errorf("brief.md missing edit constraint: %s", briefContent)
	}

	// 3. Verify herdr runner command sequence:
	// layout -> split -> agent start -> agent prompt
	calls := runner.Calls()
	if len(calls) != 4 {
		t.Fatalf("expected 4 herdr calls, got %d: %v", len(calls), calls)
	}

	// layout
	if calls[0][0] != "pane" || calls[0][1] != "layout" {
		t.Errorf("call 0 = %v, want pane layout", calls[0])
	}
	// split
	splitArgs := calls[1]
	if splitArgs[0] != "pane" || splitArgs[1] != "split" {
		t.Errorf("call 1 = %v, want pane split", splitArgs)
	}
	if !containsSlice(splitArgs, []string{"--env", "LUCIND_LANE=" + out.Lane}) {
		t.Errorf("call 1 missing --env LUCIND_LANE=%s: %v", out.Lane, splitArgs)
	}
	if !containsSlice(splitArgs, []string{"--no-focus"}) {
		t.Errorf("call 1 missing --no-focus: %v", splitArgs)
	}

	// agent start lane-<last 4 of id> --kind agy --pane <pane_id> -- --dangerously-skip-permissions --model <model>
	startArgs := calls[2]
	last4 := out.Lane[len(out.Lane)-4:]
	wantAgentName := "lane-" + last4
	if startArgs[0] != "agent" || startArgs[1] != "start" || startArgs[2] != wantAgentName {
		t.Errorf("call 2 = %v, want agent start %s", startArgs, wantAgentName)
	}
	if !containsSlice(startArgs, []string{"--kind", "agy", "--pane", "w1:pLane1"}) {
		t.Errorf("call 2 missing kind agy and pane: %v", startArgs)
	}
	if !containsSlice(startArgs, []string{"--", "--dangerously-skip-permissions", "--model", "gemini-3.7-flash-high"}) {
		t.Errorf("call 2 missing agent options: %v", startArgs)
	}

	// agent prompt <pane_id> "Read and follow <abs brief.md>" (without --wait)
	promptArgs := calls[3]
	if promptArgs[0] != "agent" || promptArgs[1] != "prompt" || promptArgs[2] != "w1:pLane1" {
		t.Errorf("call 3 = %v, want agent prompt w1:pLane1", promptArgs)
	}
	absBriefPath, _ := filepath.Abs(briefPath)
	wantPrompt := fmt.Sprintf("Read and follow %s", absBriefPath)
	if promptArgs[3] != wantPrompt {
		t.Errorf("call 3 prompt text = %q, want %q", promptArgs[3], wantPrompt)
	}
	for _, arg := range promptArgs {
		if arg == "--wait" {
			t.Errorf("agent prompt must NOT include --wait: %v", promptArgs)
		}
	}
}

func TestDispatch_Continuation(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	// Create initial lane
	createdLane, err := lane.Create(context.Background(), repoDir, []string{"pkg/**"}, "gemini-3.7-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	createdLane.PaneID = "w1:pExisting"
	createdLane.Status = lane.StatusRunning
	if err := createdLane.Save(repoDir); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	runner := newFakeHerdrRunner()
	runner.handlers["agent prompt"] = func(args []string) ([]byte, error) {
		return []byte(`{"result": {"submitted": true}}`), nil
	}

	opts := dispatch.Options{
		Cwd:    repoDir,
		LaneID: createdLane.ID,
		Brief:  "Continue fixing issue 42",
		Detach: true,
	}

	out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}
	if out.PaneID != "w1:pExisting" {
		t.Errorf("out.PaneID = %q, want \"w1:pExisting\"", out.PaneID)
	}

	// Verify only agent prompt called (no split, no start)
	calls := runner.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected exactly 1 call, got %d: %v", len(calls), calls)
	}
	if calls[0][0] != "agent" || calls[0][1] != "prompt" || calls[0][2] != "w1:pExisting" {
		t.Errorf("continuation call = %v, want agent prompt w1:pExisting", calls[0])
	}
	for _, arg := range calls[0] {
		if arg == "--wait" {
			t.Errorf("agent prompt must NOT include --wait: %v", calls[0])
		}
	}

	// Verify brief.md updated
	briefPath := filepath.Join(lane.LaneDir(repoDir, createdLane.ID), "brief.md")
	briefBytes, err := os.ReadFile(briefPath)
	if err != nil {
		t.Fatalf("read brief.md failed: %v", err)
	}
	if !strings.Contains(string(briefBytes), "Continue fixing issue 42") {
		t.Errorf("brief.md did not contain continuation brief: %s", string(briefBytes))
	}
}

func TestDispatch_Continuation_AcceptedOrRejectedErrors(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	for _, status := range []lane.Status{lane.StatusAccepted, lane.StatusRejected} {
		t.Run(string(status), func(t *testing.T) {
			l, err := lane.Create(context.Background(), repoDir, []string{"pkg/**"}, "gemini-3.7-flash-high")
			if err != nil {
				t.Fatalf("lane.Create failed: %v", err)
			}
			l.Status = status
			if err := l.Save(repoDir); err != nil {
				t.Fatalf("lane.Save failed: %v", err)
			}

			_, _, err = dispatch.Dispatch(context.Background(), dispatch.Options{
				Cwd:    repoDir,
				LaneID: l.ID,
				Brief:  "Do more work",
			}, nil)
			if err == nil {
				t.Fatalf("expected error when resuming %s lane, got nil", status)
			}
		})
	}
}

func containsSlice(haystack []string, needle []string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
