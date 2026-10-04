package dispatch_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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
	var lastSplitCwd string
	runner.handlers["pane split"] = func(args []string) ([]byte, error) {
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "--cwd" {
				lastSplitCwd = args[i+1]
			}
		}
		return []byte(fmt.Sprintf(`{"result": {"pane": {"pane_id": %q}}}`, newPaneID)), nil
	}
	runner.handlers["pane get"] = func(args []string) ([]byte, error) {
		return []byte(fmt.Sprintf(`{"result": {"pane": {"pane_id": %q, "cwd": %q}}}`, newPaneID, lastSplitCwd)), nil
	}
	runner.handlers["pane close"] = func(args []string) ([]byte, error) {
		return []byte(`{"result": {"closed": true}}`), nil
	}
	runner.handlers["agent start"] = func(args []string) ([]byte, error) {
		return []byte(`{"result": {"started": true}}`), nil
	}
	runner.handlers["agent wait"] = func(args []string) ([]byte, error) {
		return []byte(`{"result": {"status": "idle"}}`), nil
	}
	runner.handlers["agent prompt"] = func(args []string) ([]byte, error) {
		return []byte(`{"result": {"submitted": true}}`), nil
	}
	return runner
}

func wantPromptTail() []string {
	return []string{"--wait", "--until", "working", "--until", "blocked", "--timeout", "30000"}
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
		Cwd:    repoDir,
		Allow:  []string{"internal/**", "cmd/**"},
		Model:  "gemini-3.8-flash-high",
		Brief:  "# Implement Feature A\nPlease implement feature A carefully.",
		Detach: true,
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
	if savedLane.Model != "gemini-3.8-flash-high" {
		t.Errorf("savedLane.Model = %q, want \"gemini-3.8-flash-high\"", savedLane.Model)
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
	if !strings.Contains(briefContent, "- This lane requires no verification command.") {
		t.Errorf("brief.md missing no verification command line: %s", briefContent)
	}
	if strings.Contains(briefContent, "attest run") {
		t.Errorf("brief.md should not contain attest run: %s", briefContent)
	}
	if !strings.Contains(briefContent, "Do not edit outside the allowed globs.") {
		t.Errorf("brief.md missing edit constraint: %s", briefContent)
	}

	// 3. Verify herdr runner command sequence:
	// layout -> split -> pane get -> agent start -> agent wait (idle) -> agent prompt
	calls := runner.Calls()
	if len(calls) != 6 {
		t.Fatalf("expected 6 herdr calls, got %d: %v", len(calls), calls)
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

	// pane get
	if !reflect.DeepEqual(calls[2], []string{"pane", "get", "w1:pLane1"}) {
		t.Errorf("call 2 = %v, want pane get w1:pLane1", calls[2])
	}

	// agent start lane-<last 4 of id> --kind agy --pane <pane_id> -- --dangerously-skip-permissions --model <model>
	startArgs := calls[3]
	last4 := out.Lane[len(out.Lane)-4:]
	wantAgentName := "lane-" + last4
	if startArgs[0] != "agent" || startArgs[1] != "start" || startArgs[2] != wantAgentName {
		t.Errorf("call 3 = %v, want agent start %s", startArgs, wantAgentName)
	}
	if !containsSlice(startArgs, []string{"--kind", "agy", "--pane", "w1:pLane1"}) {
		t.Errorf("call 3 missing kind agy and pane: %v", startArgs)
	}
	if !containsSlice(startArgs, []string{"--", "--dangerously-skip-permissions", "--model", "gemini-3.8-flash-high"}) {
		t.Errorf("call 3 missing agent options: %v", startArgs)
	}

	// agent wait <pane_id> --until idle --timeout 60000
	if !reflect.DeepEqual(calls[4], []string{"agent", "wait", "w1:pLane1", "--until", "idle", "--timeout", "60000"}) {
		t.Errorf("call 4 = %v, want agent wait until idle", calls[4])
	}

	// agent prompt <pane_id> "Read and follow <abs brief.md>" --wait --until working --until blocked
	promptArgs := calls[5]
	if promptArgs[0] != "agent" || promptArgs[1] != "prompt" || promptArgs[2] != "w1:pLane1" {
		t.Errorf("call 5 = %v, want agent prompt w1:pLane1", promptArgs)
	}
	absBriefPath, _ := filepath.Abs(briefPath)
	wantPrompt := fmt.Sprintf("Read and follow %s", absBriefPath)
	if promptArgs[3] != wantPrompt {
		t.Errorf("call 5 prompt text = %q, want %q", promptArgs[3], wantPrompt)
	}
	if !reflect.DeepEqual(promptArgs[4:], wantPromptTail()) {
		t.Errorf("prompt flags = %v, want %v", promptArgs[4:], wantPromptTail())
	}
}

func newLaneRunnerWithPrompt(t *testing.T, prompt func(args []string) ([]byte, error), pane string) *fakeHerdrRunner {
	t.Helper()
	runner := setupFakeRunnerForNewLane(t, "w1:pLane1")
	runner.handlers["agent prompt"] = prompt
	runner.handlers["pane read"] = func(args []string) ([]byte, error) {
		return []byte(pane), nil
	}
	return runner
}

func countCalls(calls [][]string, a, b string) int {
	n := 0
	for _, c := range calls {
		if len(c) >= 2 && c[0] == a && c[1] == b {
			n++
		}
	}
	return n
}

func dispatchNew(t *testing.T, runner *fakeHerdrRunner) error {
	t.Helper()
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)
	_, _, err := dispatch.Dispatch(context.Background(), dispatch.Options{
		Cwd: repoDir, Allow: []string{"x/**"}, Model: "gemini-3.8-flash-high", Brief: "b", Detach: true,
	}, runner)
	return err
}

func TestDispatch_NewLane_PromptStall_PromptAbsent_ResendsOnce(t *testing.T) {
	n := 0
	runner := newLaneRunnerWithPrompt(t, func(args []string) ([]byte, error) {
		n++
		if n == 1 {
			return []byte(`agent_prompt_stalled`), errors.New("exit 1")
		}
		return []byte(`{"result": {"submitted": true}}`), nil
	}, "idle agy screen without the brief")
	if err := dispatchNew(t, runner); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	calls := runner.Calls()
	if got := countCalls(calls, "agent", "prompt"); got != 2 {
		t.Errorf("prompt calls = %d, want 2", got)
	}
	var read []string
	for _, c := range calls {
		if len(c) >= 2 && c[0] == "pane" && c[1] == "read" {
			read = c
		}
	}
	want := []string{"pane", "read", "w1:pLane1", "--source", "recent-unwrapped", "--lines", "40"}
	if !reflect.DeepEqual(read, want) {
		t.Errorf("pane read = %v, want %v", read, want)
	}
}

func TestDispatch_NewLane_PromptStall_PromptPresent_NoResend(t *testing.T) {
	runner := newLaneRunnerWithPrompt(t, func(args []string) ([]byte, error) {
		return []byte(`agent_prompt_stalled`), errors.New("exit 1")
	}, "> Read and follow /x/.lucind/lanes/L/brief.md\nworking")
	// the pane must contain the brief path as dispatched; compute via the fake
	runner.handlers["pane read"] = func(args []string) ([]byte, error) {
		for _, c := range runner.Calls() {
			if len(c) >= 4 && c[0] == "agent" && c[1] == "prompt" {
				return []byte("> " + c[3] + "\nworking"), nil
			}
		}
		return nil, nil
	}
	if err := dispatchNew(t, runner); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := countCalls(runner.Calls(), "agent", "prompt"); got != 1 {
		t.Errorf("prompt calls = %d, want 1 (no resend)", got)
	}
}

func TestDispatch_NewLane_ResendFails_ReturnsError(t *testing.T) {
	runner := newLaneRunnerWithPrompt(t, func(args []string) ([]byte, error) {
		return []byte(`agent_prompt_stalled`), errors.New("exit 1")
	}, "nothing here")
	if err := dispatchNew(t, runner); err == nil {
		t.Fatal("expected error when the resend also fails")
	}
	if got := countCalls(runner.Calls(), "agent", "prompt"); got != 2 {
		t.Errorf("prompt calls = %d, want 2", got)
	}
}

func TestDispatch_NewLane_ReadinessWaitFails(t *testing.T) {
	runner := setupFakeRunnerForNewLane(t, "w1:pLane1")
	runner.handlers["agent wait"] = func(args []string) ([]byte, error) {
		return []byte("timeout"), errors.New("exit 1")
	}
	if err := dispatchNew(t, runner); err == nil || !strings.Contains(err.Error(), "agent wait") {
		t.Fatalf("expected agent wait error, got %v", err)
	}
	if got := countCalls(runner.Calls(), "agent", "prompt"); got != 0 {
		t.Errorf("prompt must not be sent when agy never became ready")
	}
}

func TestDispatch_Continuation(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	// Create initial lane
	createdLane, err := lane.Create(context.Background(), repoDir, []string{"pkg/**"}, "gemini-3.8-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	createdLane.PaneID = "w1:pExisting"
	createdLane.Status = lane.StatusFailed
	createdLane.Retries = 2
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
	if !reflect.DeepEqual(calls[0][4:], wantPromptTail()) {
		t.Errorf("continuation prompt flags = %v, want %v", calls[0][4:], wantPromptTail())
	}

	// A continuation re-arms the lane: running again with a fresh retry budget.
	reloaded, err := lane.Load(repoDir, createdLane.ID)
	if err != nil {
		t.Fatalf("lane.Load failed: %v", err)
	}
	if reloaded.Status != lane.StatusRunning || reloaded.Retries != 0 {
		t.Errorf("continued lane status=%s retries=%d, want running/0", reloaded.Status, reloaded.Retries)
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
			l, err := lane.Create(context.Background(), repoDir, []string{"pkg/**"}, "gemini-3.8-flash-high")
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

func TestConstructBrief(t *testing.T) {
	laneID := "20261004-120000-abcd"
	allow := []string{"internal/**", "cmd/**"}
	absResultPath := "/workspace/.lucind/lanes/20261004-120000-abcd/result.json"

	t.Run("zero checks", func(t *testing.T) {
		brief := dispatch.ConstructBrief("Brief description", laneID, allow, absResultPath, nil)
		if !strings.Contains(brief, "Brief description") {
			t.Errorf("expected brief to contain user description, got: %s", brief)
		}
		if !strings.Contains(brief, fmt.Sprintf("- Lane ID: %s", laneID)) {
			t.Errorf("expected brief to contain lane ID, got: %s", brief)
		}
		if !strings.Contains(brief, "- Allowed globs:\n  - internal/**\n  - cmd/**") {
			t.Errorf("expected brief to contain allowed globs, got: %s", brief)
		}
		if !strings.Contains(brief, fmt.Sprintf("- Write your result envelope to `%s` following the result schema.", absResultPath)) {
			t.Errorf("expected brief to contain result envelope path, got: %s", brief)
		}
		if !strings.Contains(brief, "- This lane requires no verification command.") {
			t.Errorf("expected brief to indicate no verification command, got: %s", brief)
		}
		if strings.Contains(brief, "attest run") {
			t.Errorf("brief should not contain attest run, got: %s", brief)
		}
		if !strings.HasSuffix(strings.TrimSpace(brief), "- Do not edit outside the allowed globs.") {
			t.Errorf("brief should end with edit constraint bullet, got: %s", brief)
		}
	})

	t.Run("one check", func(t *testing.T) {
		checks := []string{"go test ./..."}
		brief := dispatch.ConstructBrief("Brief description", laneID, allow, absResultPath, checks)
		wantAttest := "- As the final verification run exactly `lucind-ai attest run -- sh -c 'go test ./...'` and do not edit files afterwards."
		if !strings.Contains(brief, wantAttest) {
			t.Errorf("brief missing expected attest line: %q in: %s", wantAttest, brief)
		}
		if strings.Contains(brief, "- This lane requires no verification command.") {
			t.Errorf("brief should not contain no verification command note: %s", brief)
		}
		if !strings.HasSuffix(strings.TrimSpace(brief), "- Do not edit outside the allowed globs.") {
			t.Errorf("brief should end with edit constraint bullet, got: %s", brief)
		}
	})

	t.Run("multiple checks", func(t *testing.T) {
		checks := []string{"go test ./...", "golangci-lint run"}
		brief := dispatch.ConstructBrief("Brief description", laneID, allow, absResultPath, checks)
		wantAttest1 := "- As the final verification run exactly `lucind-ai attest run -- sh -c 'go test ./...'` and do not edit files afterwards."
		wantAttest2 := "- As the final verification run exactly `lucind-ai attest run -- sh -c 'golangci-lint run'` and do not edit files afterwards."
		if !strings.Contains(brief, wantAttest1) {
			t.Errorf("brief missing expected attest line 1: %q in: %s", wantAttest1, brief)
		}
		if !strings.Contains(brief, wantAttest2) {
			t.Errorf("brief missing expected attest line 2: %q in: %s", wantAttest2, brief)
		}
		idx1 := strings.Index(brief, wantAttest1)
		idx2 := strings.Index(brief, wantAttest2)
		if idx1 >= idx2 {
			t.Errorf("expected attest lines in order, got idx1=%d idx2=%d", idx1, idx2)
		}
		if strings.Contains(brief, "- This lane requires no verification command.") {
			t.Errorf("brief should not contain no verification command note: %s", brief)
		}
		if !strings.HasSuffix(strings.TrimSpace(brief), "- Do not edit outside the allowed globs.") {
			t.Errorf("brief should end with edit constraint bullet, got: %s", brief)
		}
	})

	t.Run("quoting with single quotes and and-operator", func(t *testing.T) {
		check := `echo 'hello' && test`
		brief := dispatch.ConstructBrief("Brief description", laneID, allow, absResultPath, []string{check})
		wantAttest := `- As the final verification run exactly ` + "`" + `lucind-ai attest run -- sh -c 'echo '\''hello'\'' && test'` + "`" + ` and do not edit files afterwards.`
		if !strings.Contains(brief, wantAttest) {
			t.Errorf("brief missing properly quoted attest line: %q in: %s", wantAttest, brief)
		}
	})
}

func TestDispatchChecks(t *testing.T) {
	t.Run("NewLaneWithChecks", func(t *testing.T) {
		t.Setenv("HERDR_ENV", "1")
		repoDir := t.TempDir()
		initGitRepo(t, repoDir)

		runner := setupFakeRunnerForNewLane(t, "w1:pLaneChecks")

		opts := dispatch.Options{
			Cwd:    repoDir,
			Allow:  []string{"internal/**"},
			Model:  "gemini-3.8-flash-high",
			Brief:  "New lane with checks",
			Checks: []string{"go test ./...", "golangci-lint run"},
			Detach: true,
		}

		out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
		if err != nil {
			t.Fatalf("unexpected dispatch error: %v", err)
		}
		if exitCode != 0 {
			t.Fatalf("exitCode = %d, want 0", exitCode)
		}

		savedLane, err := lane.Load(repoDir, out.Lane)
		if err != nil {
			t.Fatalf("lane.Load failed: %v", err)
		}
		if !reflect.DeepEqual(savedLane.Checks, opts.Checks) {
			t.Errorf("savedLane.Checks = %v, want %v", savedLane.Checks, opts.Checks)
		}

		briefPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "brief.md")
		briefBytes, err := os.ReadFile(briefPath)
		if err != nil {
			t.Fatalf("read brief.md failed: %v", err)
		}
		briefContent := string(briefBytes)
		wantAttest1 := "- As the final verification run exactly `lucind-ai attest run -- sh -c 'go test ./...'` and do not edit files afterwards."
		wantAttest2 := "- As the final verification run exactly `lucind-ai attest run -- sh -c 'golangci-lint run'` and do not edit files afterwards."
		if !strings.Contains(briefContent, wantAttest1) {
			t.Errorf("brief.md missing attest line 1: %s", briefContent)
		}
		if !strings.Contains(briefContent, wantAttest2) {
			t.Errorf("brief.md missing attest line 2: %s", briefContent)
		}
		if strings.Contains(briefContent, "- This lane requires no verification command.") {
			t.Errorf("brief.md should not contain no verification command note: %s", briefContent)
		}
	})

	t.Run("ContinuationWithoutChecks", func(t *testing.T) {
		t.Setenv("HERDR_ENV", "1")
		repoDir := t.TempDir()
		initGitRepo(t, repoDir)

		originalChecks := []string{"go test ./..."}
		createdLane, err := lane.Create(context.Background(), repoDir, []string{"pkg/**"}, "gemini-3.8-flash-high", originalChecks...)
		if err != nil {
			t.Fatalf("lane.Create failed: %v", err)
		}
		createdLane.PaneID = "w1:pCont1"
		createdLane.Status = lane.StatusFailed
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
			Brief:  "Resume without overriding checks",
			Checks: nil,
			Detach: true,
		}

		out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
		if err != nil {
			t.Fatalf("unexpected dispatch error: %v", err)
		}
		if exitCode != 0 {
			t.Fatalf("exitCode = %d, want 0", exitCode)
		}

		reloaded, err := lane.Load(repoDir, out.Lane)
		if err != nil {
			t.Fatalf("lane.Load failed: %v", err)
		}
		if !reflect.DeepEqual(reloaded.Checks, originalChecks) {
			t.Errorf("reloaded.Checks = %v, want original %v", reloaded.Checks, originalChecks)
		}

		briefPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "brief.md")
		briefBytes, err := os.ReadFile(briefPath)
		if err != nil {
			t.Fatalf("read brief.md failed: %v", err)
		}
		wantAttest := "- As the final verification run exactly `lucind-ai attest run -- sh -c 'go test ./...'` and do not edit files afterwards."
		if !strings.Contains(string(briefBytes), wantAttest) {
			t.Errorf("brief.md did not contain preserved check: %s", string(briefBytes))
		}
	})

	t.Run("ContinuationWithReplacementChecks", func(t *testing.T) {
		t.Setenv("HERDR_ENV", "1")
		repoDir := t.TempDir()
		initGitRepo(t, repoDir)

		originalChecks := []string{"go test ./..."}
		createdLane, err := lane.Create(context.Background(), repoDir, []string{"pkg/**"}, "gemini-3.8-flash-high", originalChecks...)
		if err != nil {
			t.Fatalf("lane.Create failed: %v", err)
		}
		createdLane.PaneID = "w1:pCont2"
		createdLane.Status = lane.StatusFailed
		if err := createdLane.Save(repoDir); err != nil {
			t.Fatalf("lane.Save failed: %v", err)
		}

		runner := newFakeHerdrRunner()
		runner.handlers["agent prompt"] = func(args []string) ([]byte, error) {
			return []byte(`{"result": {"submitted": true}}`), nil
		}

		replacementChecks := []string{"golangci-lint run", "go test -race ./..."}
		opts := dispatch.Options{
			Cwd:    repoDir,
			LaneID: createdLane.ID,
			Brief:  "Resume with replacement checks",
			Checks: replacementChecks,
			Detach: true,
		}

		out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
		if err != nil {
			t.Fatalf("unexpected dispatch error: %v", err)
		}
		if exitCode != 0 {
			t.Fatalf("exitCode = %d, want 0", exitCode)
		}

		reloaded, err := lane.Load(repoDir, out.Lane)
		if err != nil {
			t.Fatalf("lane.Load failed: %v", err)
		}
		if !reflect.DeepEqual(reloaded.Checks, replacementChecks) {
			t.Errorf("reloaded.Checks = %v, want replacement %v", reloaded.Checks, replacementChecks)
		}

		briefPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "brief.md")
		briefBytes, err := os.ReadFile(briefPath)
		if err != nil {
			t.Fatalf("read brief.md failed: %v", err)
		}
		briefContent := string(briefBytes)
		wantAttest1 := "- As the final verification run exactly `lucind-ai attest run -- sh -c 'golangci-lint run'` and do not edit files afterwards."
		wantAttest2 := "- As the final verification run exactly `lucind-ai attest run -- sh -c 'go test -race ./...'` and do not edit files afterwards."
		if !strings.Contains(briefContent, wantAttest1) {
			t.Errorf("brief.md missing replacement attest 1: %s", briefContent)
		}
		if !strings.Contains(briefContent, wantAttest2) {
			t.Errorf("brief.md missing replacement attest 2: %s", briefContent)
		}
		if strings.Contains(briefContent, "- As the final verification run exactly `lucind-ai attest run -- sh -c 'go test ./...'`") {
			t.Errorf("brief.md should not contain old check: %s", briefContent)
		}
	})

	t.Run("RejectEmptyOrWhitespaceChecks", func(t *testing.T) {
		t.Setenv("HERDR_ENV", "1")
		repoDir := t.TempDir()
		initGitRepo(t, repoDir)

		badChecksList := [][]string{
			{""},
			{"   "},
			{"\t\n"},
			{"go test ./...", "  "},
		}

		for _, badChecks := range badChecksList {
			opts := dispatch.Options{
				Cwd:    repoDir,
				Allow:  []string{"pkg/**"},
				Brief:  "Should fail",
				Checks: badChecks,
				Detach: true,
			}
			_, exitCode, err := dispatch.Dispatch(context.Background(), opts, nil)
			if exitCode != 1 {
				t.Errorf("expected exitCode 1 for checks %v, got %d", badChecks, exitCode)
			}
			if err == nil || !strings.Contains(err.Error(), "check command cannot be empty") {
				t.Errorf("expected 'check command cannot be empty' for checks %v, got %v", badChecks, err)
			}
		}
	})
}

func TestDispatch_Behavior1_RelativeCwd_ReachesSplitPaneAsAbs(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)
	t.Chdir(repoDir)

	runner := setupFakeRunnerForNewLane(t, "w1:pRel")

	opts := dispatch.Options{
		Cwd:    ".",
		Allow:  []string{"*"},
		Brief:  "test relative cwd reaches split pane as abs",
		Detach: true,
	}

	_, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}

	var splitCwd string
	for _, call := range runner.Calls() {
		if len(call) >= 2 && call[0] == "pane" && call[1] == "split" {
			for i := 0; i < len(call)-1; i++ {
				if call[i] == "--cwd" {
					splitCwd = call[i+1]
					break
				}
			}
		}
	}
	if splitCwd == "" {
		t.Fatal("pane split was not called or missing --cwd")
	}
	if !filepath.IsAbs(splitCwd) {
		t.Errorf("pane split cwd is not absolute: %q", splitCwd)
	}
	expectedAbs, _ := filepath.Abs(".")
	if splitCwd != expectedAbs {
		t.Errorf("pane split cwd = %q, want %q", splitCwd, expectedAbs)
	}
}

func TestDispatch_Behavior2_OutputCwd_IsAbsolute(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)
	t.Chdir(repoDir)

	runner := setupFakeRunnerForNewLane(t, "w1:pOutAbs")

	opts := dispatch.Options{
		Cwd:    ".",
		Allow:  []string{"*"},
		Brief:  "test output cwd absolute",
		Detach: true,
	}

	out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}
	if !filepath.IsAbs(out.Cwd) {
		t.Errorf("out.Cwd is not absolute: %q", out.Cwd)
	}
	expectedAbs, _ := filepath.Abs(".")
	if out.Cwd != expectedAbs {
		t.Errorf("out.Cwd = %q, want %q", out.Cwd, expectedAbs)
	}
}

func TestDispatch_Behavior3_CwdMismatch_ClosesPaneAndErrors(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	runner := setupFakeRunnerForNewLane(t, "w1:pMismatch")
	wrongDir := filepath.Join(repoDir, "other_dir")
	runner.handlers["pane get"] = func(args []string) ([]byte, error) {
		return []byte(fmt.Sprintf(`{"result": {"pane": {"pane_id": "w1:pMismatch", "cwd": %q}}}`, wrongDir)), nil
	}

	opts := dispatch.Options{
		Cwd:    repoDir,
		Allow:  []string{"*"},
		Brief:  "test cwd mismatch",
		Detach: true,
	}

	_, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err == nil {
		t.Fatal("expected error on cwd mismatch, got nil")
	}
	if exitCode != 1 {
		t.Errorf("exitCode = %d, want 1", exitCode)
	}
	absRepoDir, _ := filepath.Abs(repoDir)
	if !strings.Contains(err.Error(), absRepoDir) || !strings.Contains(err.Error(), wrongDir) {
		t.Errorf("error %q should contain expected %q and actual %q", err.Error(), absRepoDir, wrongDir)
	}
	if !strings.Contains(err.Error(), "pane cwd mismatch:") {
		t.Errorf("error %q should mention 'pane cwd mismatch:'", err.Error())
	}

	calls := runner.Calls()
	closed := false
	for _, c := range calls {
		if len(c) >= 3 && c[0] == "pane" && c[1] == "close" && c[2] == "w1:pMismatch" {
			closed = true
			break
		}
	}
	if !closed {
		t.Errorf("expected pane close w1:pMismatch to be called, calls: %v", calls)
	}

	if countCalls(calls, "agent", "start") != 0 {
		t.Errorf("agent start should not be called on cwd mismatch")
	}
}

func TestDispatch_Behavior4_PaneGetFailure_ClosesPaneAndErrors(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	runner := setupFakeRunnerForNewLane(t, "w1:pGetFail")
	runner.handlers["pane get"] = func(args []string) ([]byte, error) {
		return []byte("pane get failed"), errors.New("exit 1")
	}

	opts := dispatch.Options{
		Cwd:    repoDir,
		Allow:  []string{"*"},
		Brief:  "test pane get failure",
		Detach: true,
	}

	_, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err == nil {
		t.Fatal("expected error on pane get failure, got nil")
	}
	if exitCode != 1 {
		t.Errorf("exitCode = %d, want 1", exitCode)
	}
	if !strings.Contains(err.Error(), "verify pane cwd:") {
		t.Errorf("error %q should mention 'verify pane cwd:'", err.Error())
	}

	calls := runner.Calls()
	closed := false
	for _, c := range calls {
		if len(c) >= 3 && c[0] == "pane" && c[1] == "close" && c[2] == "w1:pGetFail" {
			closed = true
			break
		}
	}
	if !closed {
		t.Errorf("expected pane close w1:pGetFail to be called, calls: %v", calls)
	}

	if countCalls(calls, "agent", "start") != 0 {
		t.Errorf("agent start should not be called when pane get fails")
	}
}

func TestDispatch_Behavior5_MatchingCwd_Proceeds(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	runner := setupFakeRunnerForNewLane(t, "w1:pMatch")

	opts := dispatch.Options{
		Cwd:    repoDir,
		Allow:  []string{"*"},
		Brief:  "test matching cwd proceeds",
		Detach: true,
	}

	out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}
	if out.PaneID != "w1:pMatch" {
		t.Errorf("out.PaneID = %q, want \"w1:pMatch\"", out.PaneID)
	}

	calls := runner.Calls()
	getCalls := 0
	for _, c := range calls {
		if len(c) >= 3 && c[0] == "pane" && c[1] == "get" && c[2] == "w1:pMatch" {
			getCalls++
		}
	}
	if getCalls != 1 {
		t.Errorf("expected 1 pane get call, got %d", getCalls)
	}

	if countCalls(calls, "pane", "close") != 0 {
		t.Errorf("pane close should not be called when cwd matches")
	}

	if countCalls(calls, "agent", "start") != 1 {
		t.Errorf("agent start should be called when cwd matches")
	}
}

func TestDispatch_Behavior5_MatchingCwd_Symlink_Proceeds(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	symlinkDir := filepath.Join(t.TempDir(), "symlink_dir")
	if err := os.Symlink(repoDir, symlinkDir); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	runner := setupFakeRunnerForNewLane(t, "w1:pSym")
	runner.handlers["pane get"] = func(args []string) ([]byte, error) {
		return []byte(fmt.Sprintf(`{"result": {"pane": {"pane_id": "w1:pSym", "cwd": %q}}}`, repoDir)), nil
	}

	opts := dispatch.Options{
		Cwd:    symlinkDir,
		Allow:  []string{"*"},
		Brief:  "test symlink matching",
		Detach: true,
	}

	out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("unexpected error on symlink match: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}
	if out.PaneID != "w1:pSym" {
		t.Errorf("out.PaneID = %q, want \"w1:pSym\"", out.PaneID)
	}
}
