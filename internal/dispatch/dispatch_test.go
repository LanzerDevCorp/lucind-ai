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
	"time"

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
		Prompt: "# Implement Feature A\nPlease implement feature A carefully.",
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
	if !strings.HasSuffix(out.ResultPath, "result-1.json") {
		t.Errorf("out.ResultPath = %q, want ending with result-1.json", out.ResultPath)
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

	// 2. Verify prompt.md content and absence of brief.md
	promptPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "prompt.md")
	promptBytes, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read prompt.md failed: %v", err)
	}
	promptContent := string(promptBytes)
	wantMarker := fmt.Sprintf("lucind-lane: %s turn: 1", out.Lane)
	if !strings.HasPrefix(promptContent, wantMarker) {
		t.Errorf("prompt.md missing marker at start: %s", promptContent)
	}
	if !strings.Contains(promptContent, "# Implement Feature A") {
		t.Errorf("prompt.md missing user prompt content: %s", promptContent)
	}
	if !strings.Contains(promptContent, fmt.Sprintf("- Lane ID: %s", out.Lane)) {
		t.Errorf("prompt.md missing Lane ID: %s", promptContent)
	}
	if !strings.Contains(promptContent, "- Allowed globs:\n  - internal/**\n  - cmd/**") {
		t.Errorf("prompt.md missing Allowed globs: %s", promptContent)
	}
	resultPath := lane.ResultFilePath(repoDir, savedLane)
	absResultPath, _ := filepath.Abs(resultPath)
	if !strings.HasSuffix(absResultPath, "result-1.json") {
		t.Errorf("absResultPath = %q, want ending with result-1.json", absResultPath)
	}
	if !strings.Contains(promptContent, fmt.Sprintf("Write your result envelope to `%s` following the result schema.", absResultPath)) {
		t.Errorf("prompt.md missing result path: %s", promptContent)
	}
	if out.ResultPath != absResultPath {
		t.Errorf("out.ResultPath = %q, want %q", out.ResultPath, absResultPath)
	}
	if !strings.Contains(promptContent, "- This lane requires no verification command.") {
		t.Errorf("prompt.md missing no verification command line: %s", promptContent)
	}
	if strings.Contains(promptContent, "attest run") {
		t.Errorf("prompt.md should not contain attest run: %s", promptContent)
	}
	if !strings.Contains(promptContent, "Do not edit outside the allowed globs.") {
		t.Errorf("prompt.md missing edit constraint: %s", promptContent)
	}
	briefPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "brief.md")
	if _, err := os.Stat(briefPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("brief.md should not exist, stat err = %v", err)
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

	// agent prompt <pane_id> <promptContent> --wait --until working --until blocked
	promptArgs := calls[5]
	if promptArgs[0] != "agent" || promptArgs[1] != "prompt" || promptArgs[2] != "w1:pLane1" {
		t.Errorf("call 5 = %v, want agent prompt w1:pLane1", promptArgs)
	}
	if promptArgs[3] != promptContent {
		t.Errorf("call 5 prompt text = %q, want %q", promptArgs[3], promptContent)
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
		Cwd: repoDir, Allow: []string{"x/**"}, Model: "gemini-3.8-flash-high", Prompt: "b", Detach: true,
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
	}, "idle agy screen without the prompt marker")
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
	}, "idle")
	// The pane contains only the lane marker line, not brief.md.
	runner.handlers["pane read"] = func(args []string) ([]byte, error) {
		for _, c := range runner.Calls() {
			if len(c) >= 2 && c[0] == "pane" && c[1] == "split" {
				for _, arg := range c {
					if strings.HasPrefix(arg, "LUCIND_LANE=") {
						id := strings.TrimPrefix(arg, "LUCIND_LANE=")
						return []byte(fmt.Sprintf("> lucind-lane: %s turn: 1\nworking", id)), nil
					}
				}
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
		Prompt: "Continue fixing issue 42",
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
	if !strings.HasSuffix(out.ResultPath, "result-2.json") {
		t.Errorf("out.ResultPath = %q, want ending with result-2.json", out.ResultPath)
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
	if reloaded.Turn != 2 {
		t.Errorf("continued lane turn=%d, want 2", reloaded.Turn)
	}

	// Verify prompt.md updated
	promptPath := filepath.Join(lane.LaneDir(repoDir, createdLane.ID), "prompt.md")
	promptBytes, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read prompt.md failed: %v", err)
	}
	promptContent := string(promptBytes)
	if !strings.Contains(promptContent, "Continue fixing issue 42") {
		t.Errorf("prompt.md did not contain continuation prompt: %s", promptContent)
	}
	wantResultPath, _ := filepath.Abs(lane.ResultFilePath(repoDir, reloaded))
	if !strings.Contains(promptContent, wantResultPath) {
		t.Errorf("prompt.md did not contain turn 2 result path %s: %s", wantResultPath, promptContent)
	}
	if calls[0][3] != promptContent {
		t.Errorf("continuation prompt text = %q, want %q", calls[0][3], promptContent)
	}
	briefPath := filepath.Join(lane.LaneDir(repoDir, createdLane.ID), "brief.md")
	if _, err := os.Stat(briefPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("brief.md should not exist, stat err = %v", err)
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
				Prompt: "Do more work",
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

func TestConstructPrompt(t *testing.T) {
	laneID := "20261004-120000-abcd"
	turn := 1
	allow := []string{"internal/**", "cmd/**"}
	absResultPath := "/workspace/.lucind/lanes/20261004-120000-abcd/result-1.json"

	t.Run("marker line first, skills section moved up, footer last", func(t *testing.T) {
		userPrompt := "# Feature Title\n\n## Skills to load before work\n/path/to/skill1/SKILL.md\n/path/to/skill2/SKILL.md\n\n## Goal\nImplement feature."
		got := dispatch.ConstructPrompt(userPrompt, laneID, turn, allow, absResultPath, nil)

		wantMarker := fmt.Sprintf("lucind-lane: %s turn: %d", laneID, turn)
		wantSkills := "## Skills to load before work\n/path/to/skill1/SKILL.md\n/path/to/skill2/SKILL.md"
		wantRest := "# Feature Title\n\n## Goal\nImplement feature."

		if !strings.HasPrefix(got, wantMarker) {
			t.Fatalf("prompt must start with lane marker %q; got:\n%s", wantMarker, got)
		}

		lines := strings.Split(got, "\n")
		if lines[0] != wantMarker {
			t.Errorf("line 0 = %q, want %q", lines[0], wantMarker)
		}

		idxMarker := strings.Index(got, wantMarker)
		idxSkills := strings.Index(got, wantSkills)
		idxRest := strings.Index(got, wantRest)
		idxFooter := strings.Index(got, "## Lane Contract")

		if idxSkills == -1 || idxRest == -1 || idxFooter == -1 {
			t.Fatalf("missing required sections in prompt:\n%s", got)
		}
		if idxMarker >= idxSkills || idxSkills >= idxRest || idxRest >= idxFooter {
			t.Errorf("expected order: marker < skills < rest < footer; got idxMarker=%d idxSkills=%d idxRest=%d idxFooter=%d",
				idxMarker, idxSkills, idxRest, idxFooter)
		}
	})

	t.Run("no skills section keeps marker first and prompt before footer", func(t *testing.T) {
		userPrompt := "# Feature Title\n\n## Goal\nImplement feature."
		got := dispatch.ConstructPrompt(userPrompt, laneID, turn, allow, absResultPath, nil)

		wantMarker := fmt.Sprintf("lucind-lane: %s turn: %d", laneID, turn)
		if !strings.HasPrefix(got, wantMarker) {
			t.Fatalf("prompt must start with lane marker %q; got:\n%s", wantMarker, got)
		}
		if strings.Contains(got, "## Skills to load before work") {
			t.Errorf("prompt should not have skills section when none provided; got:\n%s", got)
		}
		if !strings.Contains(got, userPrompt) {
			t.Errorf("prompt missing user prompt content; got:\n%s", got)
		}
		if !strings.Contains(got, "## Lane Contract") {
			t.Errorf("prompt missing footer; got:\n%s", got)
		}
	})

	t.Run("zero checks", func(t *testing.T) {
		prompt := dispatch.ConstructPrompt("Prompt description", laneID, turn, allow, absResultPath, nil)
		if !strings.Contains(prompt, "Prompt description") {
			t.Errorf("expected prompt to contain user description, got: %s", prompt)
		}
		if !strings.Contains(prompt, fmt.Sprintf("- Lane ID: %s", laneID)) {
			t.Errorf("expected prompt to contain lane ID, got: %s", prompt)
		}
		if !strings.Contains(prompt, "- Allowed globs:\n  - internal/**\n  - cmd/**") {
			t.Errorf("expected prompt to contain allowed globs, got: %s", prompt)
		}
		if !strings.Contains(prompt, fmt.Sprintf("- Write your result envelope to `%s` following the result schema.", absResultPath)) {
			t.Errorf("expected prompt to contain result envelope path, got: %s", prompt)
		}
		if !strings.Contains(prompt, "- This lane requires no verification command.") {
			t.Errorf("expected prompt to indicate no verification command, got: %s", prompt)
		}
		if strings.Contains(prompt, "attest run") {
			t.Errorf("prompt should not contain attest run, got: %s", prompt)
		}
		if strings.Contains(prompt, "As the final verification") {
			t.Errorf("prompt should not contain verification intro line, got: %s", prompt)
		}
		if !strings.HasSuffix(strings.TrimSpace(prompt), "- Do not edit outside the allowed globs.") {
			t.Errorf("prompt should end with edit constraint bullet, got: %s", prompt)
		}
	})

	t.Run("one check", func(t *testing.T) {
		checks := []string{"go test ./..."}
		prompt := dispatch.ConstructPrompt("Prompt description", laneID, turn, allow, absResultPath, checks)
		wantIntro := "- As the final verification, after your last edit, run these commands exactly as listed and do not edit files afterwards:\n"
		wantAttest := "  - `lucind-ai attest run -- sh -c 'go test ./...'`\n"
		if !strings.Contains(prompt, wantIntro) {
			t.Errorf("prompt missing expected intro line: %q in: %s", wantIntro, prompt)
		}
		if strings.Count(prompt, wantIntro) != 1 {
			t.Errorf("prompt should contain intro line exactly once, got %d", strings.Count(prompt, wantIntro))
		}
		if !strings.Contains(prompt, wantAttest) {
			t.Errorf("prompt missing expected attest line: %q in: %s", wantAttest, prompt)
		}
		if strings.Contains(prompt, "- This lane requires no verification command.") {
			t.Errorf("prompt should not contain no verification command note: %s", prompt)
		}
		if !strings.HasSuffix(strings.TrimSpace(prompt), "- Do not edit outside the allowed globs.") {
			t.Errorf("prompt should end with edit constraint bullet, got: %s", prompt)
		}
	})

	t.Run("multiple checks", func(t *testing.T) {
		checks := []string{"go test ./...", "golangci-lint run"}
		prompt := dispatch.ConstructPrompt("Prompt description", laneID, turn, allow, absResultPath, checks)
		wantIntro := "- As the final verification, after your last edit, run these commands exactly as listed and do not edit files afterwards:\n"
		wantAttest1 := "  - `lucind-ai attest run -- sh -c 'go test ./...'`\n"
		wantAttest2 := "  - `lucind-ai attest run -- sh -c 'golangci-lint run'`\n"
		if !strings.Contains(prompt, wantIntro) {
			t.Errorf("prompt missing expected intro line: %q in: %s", wantIntro, prompt)
		}
		if strings.Count(prompt, wantIntro) != 1 {
			t.Errorf("prompt should contain intro line exactly once, got %d", strings.Count(prompt, wantIntro))
		}
		if !strings.Contains(prompt, wantAttest1) {
			t.Errorf("prompt missing expected attest line 1: %q in: %s", wantAttest1, prompt)
		}
		if !strings.Contains(prompt, wantAttest2) {
			t.Errorf("prompt missing expected attest line 2: %q in: %s", wantAttest2, prompt)
		}
		idxIntro := strings.Index(prompt, wantIntro)
		idx1 := strings.Index(prompt, wantAttest1)
		idx2 := strings.Index(prompt, wantAttest2)
		if idxIntro >= idx1 || idx1 >= idx2 {
			t.Errorf("expected intro before attest1 before attest2, got idxIntro=%d idx1=%d idx2=%d", idxIntro, idx1, idx2)
		}
		if strings.Contains(prompt, "- This lane requires no verification command.") {
			t.Errorf("prompt should not contain no verification command note: %s", prompt)
		}
		if !strings.HasSuffix(strings.TrimSpace(prompt), "- Do not edit outside the allowed globs.") {
			t.Errorf("prompt should end with edit constraint bullet, got: %s", prompt)
		}
	})

	t.Run("quoting with single quotes and and-operator", func(t *testing.T) {
		check := `echo 'hello' && test`
		prompt := dispatch.ConstructPrompt("Prompt description", laneID, turn, allow, absResultPath, []string{check})
		wantAttest := "  - `lucind-ai attest run -- sh -c 'echo '\\''hello'\\'' && test'`\n"
		if !strings.Contains(prompt, wantAttest) {
			t.Errorf("prompt missing properly quoted attest line: %q in: %s", wantAttest, prompt)
		}
	})

	t.Run("three checks", func(t *testing.T) {
		checks := []string{"cmd1", "cmd2", "cmd3"}
		prompt := dispatch.ConstructPrompt("Prompt description", laneID, turn, allow, absResultPath, checks)
		wantIntro := "- As the final verification, after your last edit, run these commands exactly as listed and do not edit files afterwards:\n"
		if strings.Count(prompt, wantIntro) != 1 {
			t.Errorf("prompt should contain intro line exactly once, got %d", strings.Count(prompt, wantIntro))
		}
		for _, c := range checks {
			wantItem := fmt.Sprintf("  - `lucind-ai attest run -- sh -c '%s'`\n", c)
			if !strings.Contains(prompt, wantItem) {
				t.Errorf("prompt missing item %q: %s", wantItem, prompt)
			}
		}
	})
}

func TestConstructPrompt_LayoutEdgeCases(t *testing.T) {
	laneID := "20261004-120000-abcd"
	turn := 1
	allow := []string{"internal/**", "cmd/**"}
	absResultPath := "/workspace/.lucind/lanes/20261004-120000-abcd/result-1.json"
	wantMarker := fmt.Sprintf("lucind-lane: %s turn: %d", laneID, turn)
	wantFooterHeader := "---\n## Lane Contract"
	wantFooterSuffix := "- Do not edit outside the allowed globs."

	tests := []struct {
		name       string
		userPrompt string
		assert     func(t *testing.T, got string)
	}{
		{
			name: "skills section at the very end of the prompt",
			userPrompt: "# Feature Title\n\nFeature description.\n\n" +
				"## Skills to load before work\n/path/to/skill1/SKILL.md\n/path/to/skill2/SKILL.md",
			assert: func(t *testing.T, got string) {
				wantSkills := "## Skills to load before work\n/path/to/skill1/SKILL.md\n/path/to/skill2/SKILL.md"
				wantRest := "# Feature Title\n\nFeature description."

				idxMarker := strings.Index(got, wantMarker)
				idxSkills := strings.Index(got, wantSkills)
				idxRest := strings.Index(got, wantRest)
				idxFooter := strings.Index(got, wantFooterHeader)

				if idxSkills == -1 || idxRest == -1 || idxFooter == -1 {
					t.Fatalf("missing required sections in prompt:\n%s", got)
				}
				if idxMarker >= idxSkills || idxSkills >= idxRest || idxRest >= idxFooter {
					t.Errorf("expected order: marker < skills < rest < footer; got idxMarker=%d idxSkills=%d idxRest=%d idxFooter=%d",
						idxMarker, idxSkills, idxRest, idxFooter)
				}
			},
		},
		{
			name: "a blank line inside the section ends it",
			userPrompt: "# Feature Title\n\n" +
				"## Skills to load before work\n/path/to/skill1/SKILL.md\n\n/path/to/skill2/SKILL.md\n\n" +
				"## Goal\nImplement feature.",
			assert: func(t *testing.T, got string) {
				// The blank line inside the skills section causes extractSkillsSection to end
				// the skills section after skill1. Skill2 remains in the rest of the prompt.
				wantSkills := "## Skills to load before work\n/path/to/skill1/SKILL.md"
				wantRest := "# Feature Title\n\n/path/to/skill2/SKILL.md\n\n## Goal\nImplement feature."

				idxMarker := strings.Index(got, wantMarker)
				idxSkills := strings.Index(got, wantSkills)
				idxRest := strings.Index(got, wantRest)
				idxFooter := strings.Index(got, wantFooterHeader)

				if idxSkills == -1 || idxRest == -1 || idxFooter == -1 {
					t.Fatalf("missing required sections in prompt:\n%s", got)
				}
				if idxMarker >= idxSkills || idxSkills >= idxRest || idxRest >= idxFooter {
					t.Errorf("expected order: marker < skills < rest < footer; got idxMarker=%d idxSkills=%d idxRest=%d idxFooter=%d",
						idxMarker, idxSkills, idxRest, idxFooter)
				}
			},
		},
		{
			name: "CRLF line endings and trailing spaces on the heading line",
			userPrompt: "# Feature Title\r\n\r\n" +
				"## Skills to load before work   \r\n/path/to/skill1/SKILL.md\r\n\r\n" +
				"## Goal\r\nImplement feature.",
			assert: func(t *testing.T, got string) {
				wantSkillPath := "/path/to/skill1/SKILL.md"
				wantTitle := "# Feature Title"
				wantGoal := "## Goal"

				idxMarker := strings.Index(got, wantMarker)
				idxSkillsHeading := strings.Index(got, "## Skills to load before work")
				idxSkillPath := strings.Index(got, wantSkillPath)
				idxTitle := strings.Index(got, wantTitle)
				idxGoal := strings.Index(got, wantGoal)
				idxFooter := strings.Index(got, wantFooterHeader)

				if idxSkillsHeading == -1 || idxSkillPath == -1 || idxTitle == -1 || idxGoal == -1 || idxFooter == -1 {
					t.Fatalf("missing required sections in prompt:\n%s", got)
				}
				if idxMarker >= idxSkillsHeading || idxSkillsHeading >= idxTitle || idxTitle >= idxGoal || idxGoal >= idxFooter {
					t.Errorf("expected order: marker < skills < title < goal < footer; got idxMarker=%d idxSkills=%d idxTitle=%d idxGoal=%d idxFooter=%d",
						idxMarker, idxSkillsHeading, idxTitle, idxGoal, idxFooter)
				}
			},
		},
		{
			name: "similar heading is not treated as skills section",
			userPrompt: "# Feature Title\n\n" +
				"## Skills to load\n/path/to/skill1/SKILL.md\n\n" +
				"## Goal\nImplement feature.",
			assert: func(t *testing.T, got string) {
				// "## Skills to load" must not be extracted or moved to the top.
				// The user prompt remains in its original order under rest.
				wantSimilar := "## Skills to load\n/path/to/skill1/SKILL.md"
				wantTitle := "# Feature Title"
				wantGoal := "## Goal\nImplement feature."

				if strings.Contains(got, "## Skills to load before work") {
					t.Errorf("prompt should not contain canonical skills heading; got:\n%s", got)
				}

				idxMarker := strings.Index(got, wantMarker)
				idxTitle := strings.Index(got, wantTitle)
				idxSimilar := strings.Index(got, wantSimilar)
				idxGoal := strings.Index(got, wantGoal)
				idxFooter := strings.Index(got, wantFooterHeader)

				if idxTitle == -1 || idxSimilar == -1 || idxGoal == -1 || idxFooter == -1 {
					t.Fatalf("missing required sections in prompt:\n%s", got)
				}
				if idxMarker >= idxTitle || idxTitle >= idxSimilar || idxSimilar >= idxGoal || idxGoal >= idxFooter {
					t.Errorf("expected order: marker < title < similar < goal < footer; got idxMarker=%d idxTitle=%d idxSimilar=%d idxGoal=%d idxFooter=%d",
						idxMarker, idxTitle, idxSimilar, idxGoal, idxFooter)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dispatch.ConstructPrompt(tt.userPrompt, laneID, turn, allow, absResultPath, nil)

			// In every case the marker line is line 1.
			lines := strings.Split(got, "\n")
			if len(lines) == 0 || lines[0] != wantMarker {
				t.Fatalf("line 0 = %q, want marker %q", lines[0], wantMarker)
			}

			// In every case the footer is last.
			trimmedGot := strings.TrimSpace(got)
			if !strings.HasSuffix(trimmedGot, wantFooterSuffix) {
				t.Errorf("prompt should end with %q; got suffix:\n%s", wantFooterSuffix, trimmedGot)
			}
			if !strings.Contains(got, wantFooterHeader) {
				t.Fatalf("prompt missing footer header %q in:\n%s", wantFooterHeader, got)
			}

			tt.assert(t, got)
		})
	}
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
			Prompt: "New lane with checks",
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

		promptPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "prompt.md")
		promptBytes, err := os.ReadFile(promptPath)
		if err != nil {
			t.Fatalf("read prompt.md failed: %v", err)
		}
		promptContent := string(promptBytes)
		wantIntro := "- As the final verification, after your last edit, run these commands exactly as listed and do not edit files afterwards:\n"
		wantAttest1 := "  - `lucind-ai attest run -- sh -c 'go test ./...'`\n"
		wantAttest2 := "  - `lucind-ai attest run -- sh -c 'golangci-lint run'`\n"
		if !strings.Contains(promptContent, wantIntro) {
			t.Errorf("prompt.md missing intro line: %s", promptContent)
		}
		if strings.Count(promptContent, wantIntro) != 1 {
			t.Errorf("prompt.md should have intro line exactly once, got %d", strings.Count(promptContent, wantIntro))
		}
		if !strings.Contains(promptContent, wantAttest1) {
			t.Errorf("prompt.md missing attest line 1: %s", promptContent)
		}
		if !strings.Contains(promptContent, wantAttest2) {
			t.Errorf("prompt.md missing attest line 2: %s", promptContent)
		}
		if strings.Contains(promptContent, "- This lane requires no verification command.") {
			t.Errorf("prompt.md should not contain no verification command note: %s", promptContent)
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
			Prompt: "Resume without overriding checks",
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

		promptPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "prompt.md")
		promptBytes, err := os.ReadFile(promptPath)
		if err != nil {
			t.Fatalf("read prompt.md failed: %v", err)
		}
		wantIntro := "- As the final verification, after your last edit, run these commands exactly as listed and do not edit files afterwards:\n"
		wantAttest := "  - `lucind-ai attest run -- sh -c 'go test ./...'`\n"
		if !strings.Contains(string(promptBytes), wantIntro) {
			t.Errorf("prompt.md did not contain intro line: %s", string(promptBytes))
		}
		if !strings.Contains(string(promptBytes), wantAttest) {
			t.Errorf("prompt.md did not contain preserved check: %s", string(promptBytes))
		}
	})

	// A follow-up turn is a fresh start for the Stop hook: the retry budget, the total
	// continue counter and the last counted stop must not leak from the previous turn.
	t.Run("ContinuationResetsStopCounters", func(t *testing.T) {
		t.Setenv("HERDR_ENV", "1")
		repoDir := t.TempDir()
		initGitRepo(t, repoDir)

		createdLane, err := lane.Create(context.Background(), repoDir, []string{"pkg/**"}, "gemini-3.8-flash-high")
		if err != nil {
			t.Fatalf("lane.Create failed: %v", err)
		}
		lastStop := time.Now().UTC()
		createdLane.PaneID = "w1:pCont2"
		createdLane.Status = lane.StatusFailed
		createdLane.Retries = 2
		createdLane.Continues = 7
		createdLane.LastStopAt = &lastStop
		if err := createdLane.Save(repoDir); err != nil {
			t.Fatalf("lane.Save failed: %v", err)
		}

		runner := newFakeHerdrRunner()
		runner.handlers["agent prompt"] = func(args []string) ([]byte, error) {
			return []byte(`{"result": {"submitted": true}}`), nil
		}

		out, exitCode, err := dispatch.Dispatch(context.Background(), dispatch.Options{
			Cwd:    repoDir,
			LaneID: createdLane.ID,
			Prompt: "Follow-up turn",
			Detach: true,
		}, runner)
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
		if reloaded.Status != lane.StatusRunning {
			t.Errorf("Status = %q, want %q", reloaded.Status, lane.StatusRunning)
		}
		if reloaded.Retries != 0 {
			t.Errorf("Retries = %d, want 0", reloaded.Retries)
		}
		if reloaded.Continues != 0 {
			t.Errorf("Continues = %d, want 0", reloaded.Continues)
		}
		if reloaded.LastStopAt != nil {
			t.Errorf("LastStopAt = %v, want nil", reloaded.LastStopAt)
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
			Prompt: "Resume with replacement checks",
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

		promptPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "prompt.md")
		promptBytes, err := os.ReadFile(promptPath)
		if err != nil {
			t.Fatalf("read prompt.md failed: %v", err)
		}
		promptContent := string(promptBytes)
		wantIntro := "- As the final verification, after your last edit, run these commands exactly as listed and do not edit files afterwards:\n"
		wantAttest1 := "  - `lucind-ai attest run -- sh -c 'golangci-lint run'`\n"
		wantAttest2 := "  - `lucind-ai attest run -- sh -c 'go test -race ./...'`\n"
		if !strings.Contains(promptContent, wantIntro) {
			t.Errorf("prompt.md missing intro line: %s", promptContent)
		}
		if !strings.Contains(promptContent, wantAttest1) {
			t.Errorf("prompt.md missing replacement attest 1: %s", promptContent)
		}
		if !strings.Contains(promptContent, wantAttest2) {
			t.Errorf("prompt.md missing replacement attest 2: %s", promptContent)
		}
		if strings.Contains(promptContent, "go test ./...") {
			t.Errorf("prompt.md should not contain old check: %s", promptContent)
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
				Prompt: "Should fail",
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

func TestDispatch_ContinuationAllow(t *testing.T) {
	continueLane := func(t *testing.T, originalAllow, continuationAllow []string) lane.Lane {
		t.Helper()
		t.Setenv("HERDR_ENV", "1")
		repoDir := t.TempDir()
		initGitRepo(t, repoDir)

		createdLane, err := lane.Create(context.Background(), repoDir, originalAllow, "gemini-3.8-flash-high")
		if err != nil {
			t.Fatalf("lane.Create failed: %v", err)
		}
		createdLane.PaneID = "w1:pCont3"
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
			Prompt: "Resume",
			Allow:  continuationAllow,
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
		return reloaded
	}

	t.Run("ReplacesAllowAndPersistsIt", func(t *testing.T) {
		want := []string{"pkg/**", "cmd/hook.go"}
		reloaded := continueLane(t, []string{"pkg/**"}, want)
		if !reflect.DeepEqual(reloaded.Allow, want) {
			t.Errorf("reloaded.Allow = %v, want %v (the hook and accept read lane.json)", reloaded.Allow, want)
		}
	})

	t.Run("KeepsAllowWhenOmitted", func(t *testing.T) {
		want := []string{"pkg/**"}
		reloaded := continueLane(t, want, nil)
		if !reflect.DeepEqual(reloaded.Allow, want) {
			t.Errorf("reloaded.Allow = %v, want unchanged %v", reloaded.Allow, want)
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
		Prompt: "test relative cwd reaches split pane as abs",
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
		Prompt: "test output cwd absolute",
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
		Prompt: "test cwd mismatch",
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
		Prompt: "test pane get failure",
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
		Prompt: "test matching cwd proceeds",
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
		Prompt: "test symlink matching",
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

func TestDispatch_Continuation_RenamesResultToPrevResult(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	createdLane, err := lane.Create(context.Background(), repoDir, []string{"pkg/**"}, "gemini-3.8-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	createdLane.PaneID = "w1:pContRename"
	createdLane.Status = lane.StatusFailed
	createdLane.Turn = 0
	if err := createdLane.Save(repoDir); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	laneDir := lane.LaneDir(repoDir, createdLane.ID)
	resultPath := filepath.Join(laneDir, "result.json")
	prevPath := filepath.Join(laneDir, "result.prev.json")
	if err := os.WriteFile(resultPath, []byte(`{"status": "old"}`), 0644); err != nil {
		t.Fatalf("write result.json failed: %v", err)
	}

	runner := newFakeHerdrRunner()
	runner.handlers["agent prompt"] = func(args []string) ([]byte, error) {
		return []byte(`{"result": {"submitted": true}}`), nil
	}

	opts := dispatch.Options{
		Cwd:    repoDir,
		LaneID: createdLane.ID,
		Prompt: "Resume and rename result",
		Detach: true,
	}

	out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}

	reloaded, err := lane.Load(repoDir, createdLane.ID)
	if err != nil {
		t.Fatalf("lane.Load failed: %v", err)
	}
	if reloaded.Turn != 1 {
		t.Errorf("reloaded.Turn = %d, want 1", reloaded.Turn)
	}
	if !strings.HasSuffix(out.ResultPath, "result-1.json") {
		t.Errorf("out.ResultPath = %q, want ending with result-1.json", out.ResultPath)
	}

	promptPath := filepath.Join(laneDir, "prompt.md")
	promptBytes, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read prompt.md failed: %v", err)
	}
	wantResultPath, _ := filepath.Abs(lane.ResultFilePath(repoDir, reloaded))
	if !strings.Contains(string(promptBytes), wantResultPath) {
		t.Errorf("prompt.md did not contain result path %s: %s", wantResultPath, string(promptBytes))
	}
	briefPath := filepath.Join(laneDir, "brief.md")
	if _, err := os.Stat(briefPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("brief.md should not exist, stat err = %v", err)
	}

	if _, err := os.Stat(resultPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("result.json should not exist, err=%v", err)
	}
	prevBytes, err := os.ReadFile(prevPath)
	if err != nil {
		t.Fatalf("read result.prev.json failed: %v", err)
	}
	if string(prevBytes) != `{"status": "old"}` {
		t.Errorf("result.prev.json content = %q, want %q", string(prevBytes), `{"status": "old"}`)
	}
}

func TestDispatch_Continuation_ReplacesOlderPrevResult(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	createdLane, err := lane.Create(context.Background(), repoDir, []string{"pkg/**"}, "gemini-3.8-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	createdLane.PaneID = "w1:pContReplace"
	createdLane.Status = lane.StatusFailed
	createdLane.Turn = 0
	if err := createdLane.Save(repoDir); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	laneDir := lane.LaneDir(repoDir, createdLane.ID)
	resultPath := filepath.Join(laneDir, "result.json")
	prevPath := filepath.Join(laneDir, "result.prev.json")
	if err := os.WriteFile(prevPath, []byte(`{"status": "ancient"}`), 0644); err != nil {
		t.Fatalf("write result.prev.json failed: %v", err)
	}
	if err := os.WriteFile(resultPath, []byte(`{"status": "previous"}`), 0644); err != nil {
		t.Fatalf("write result.json failed: %v", err)
	}

	runner := newFakeHerdrRunner()
	runner.handlers["agent prompt"] = func(args []string) ([]byte, error) {
		return []byte(`{"result": {"submitted": true}}`), nil
	}

	opts := dispatch.Options{
		Cwd:    repoDir,
		LaneID: createdLane.ID,
		Prompt: "Resume and replace older prev result",
		Detach: true,
	}

	_, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}

	if _, err := os.Stat(resultPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("result.json should not exist, err=%v", err)
	}
	prevBytes, err := os.ReadFile(prevPath)
	if err != nil {
		t.Fatalf("read result.prev.json failed: %v", err)
	}
	if string(prevBytes) != `{"status": "previous"}` {
		t.Errorf("result.prev.json content = %q, want %q", string(prevBytes), `{"status": "previous"}`)
	}
}

func TestDispatch_Continuation_ToleratesMissingResult(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	createdLane, err := lane.Create(context.Background(), repoDir, []string{"pkg/**"}, "gemini-3.8-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	createdLane.PaneID = "w1:pContMissing"
	createdLane.Status = lane.StatusFailed
	createdLane.Turn = 0
	if err := createdLane.Save(repoDir); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	laneDir := lane.LaneDir(repoDir, createdLane.ID)
	resultPath := filepath.Join(laneDir, "result.json")
	prevPath := filepath.Join(laneDir, "result.prev.json")

	runner := newFakeHerdrRunner()
	runner.handlers["agent prompt"] = func(args []string) ([]byte, error) {
		return []byte(`{"result": {"submitted": true}}`), nil
	}

	opts := dispatch.Options{
		Cwd:    repoDir,
		LaneID: createdLane.ID,
		Prompt: "Resume with no result.json",
		Detach: true,
	}

	_, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}

	if _, err := os.Stat(resultPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("result.json should not exist, err=%v", err)
	}
	if _, err := os.Stat(prevPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("result.prev.json should not exist, err=%v", err)
	}
}

func TestDispatch_NewLane_DoesNotTouchOrCreatePrevResult(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	runner := setupFakeRunnerForNewLane(t, "w1:pNewNoPrev")

	opts := dispatch.Options{
		Cwd:    repoDir,
		Allow:  []string{"*"},
		Prompt: "New lane should not have prev result",
		Detach: true,
	}

	out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}

	laneDir := lane.LaneDir(repoDir, out.Lane)
	prevPath := filepath.Join(laneDir, "result.prev.json")
	if _, err := os.Stat(prevPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("result.prev.json should not exist for new lane, err=%v", err)
	}
}

func TestDispatch_Continuation_RenameErrorReturned(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	createdLane, err := lane.Create(context.Background(), repoDir, []string{"pkg/**"}, "gemini-3.8-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	createdLane.PaneID = "w1:pContErr"
	createdLane.Status = lane.StatusFailed
	createdLane.Turn = 0
	if err := createdLane.Save(repoDir); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	laneDir := lane.LaneDir(repoDir, createdLane.ID)
	resultPath := filepath.Join(laneDir, "result.json")
	prevPath := filepath.Join(laneDir, "result.prev.json")
	if err := os.WriteFile(resultPath, []byte(`{"status": "old"}`), 0644); err != nil {
		t.Fatalf("write result.json failed: %v", err)
	}
	if err := os.Mkdir(prevPath, 0755); err != nil {
		t.Fatalf("mkdir result.prev.json failed: %v", err)
	}

	runner := newFakeHerdrRunner()
	opts := dispatch.Options{
		Cwd:    repoDir,
		LaneID: createdLane.ID,
		Prompt: "Resume with rename error",
		Detach: true,
	}

	_, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err == nil {
		t.Fatal("expected error when rename fails, got nil")
	}
	if exitCode != 1 {
		t.Errorf("exitCode = %d, want 1", exitCode)
	}
	if !strings.Contains(err.Error(), "rename previous result:") {
		t.Errorf("expected error to contain 'rename previous result:', got: %v", err)
	}
}

func TestDispatch_Continuation_Turn1Lane_KeepsHistoryAndIncrementsTurn(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	createdLane, err := lane.Create(context.Background(), repoDir, []string{"pkg/**"}, "gemini-3.8-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	createdLane.PaneID = "w1:pContTurn1"
	createdLane.Status = lane.StatusFailed
	if err := createdLane.Save(repoDir); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	laneDir := lane.LaneDir(repoDir, createdLane.ID)
	result1Path := filepath.Join(laneDir, "result-1.json")
	prevPath := filepath.Join(laneDir, "result.prev.json")
	if err := os.WriteFile(result1Path, []byte(`{"status": "done", "turn": 1}`), 0644); err != nil {
		t.Fatalf("write result-1.json failed: %v", err)
	}

	runner := newFakeHerdrRunner()
	runner.handlers["agent prompt"] = func(args []string) ([]byte, error) {
		return []byte(`{"result": {"submitted": true}}`), nil
	}

	opts := dispatch.Options{
		Cwd:    repoDir,
		LaneID: createdLane.ID,
		Prompt: "Continue with turn 2",
		Detach: true,
	}

	out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}

	// Turn becomes 2 in lane.json
	reloaded, err := lane.Load(repoDir, createdLane.ID)
	if err != nil {
		t.Fatalf("lane.Load failed: %v", err)
	}
	if reloaded.Turn != 2 {
		t.Errorf("reloaded.Turn = %d, want 2", reloaded.Turn)
	}

	// Output.ResultPath ends with result-2.json
	if !strings.HasSuffix(out.ResultPath, "result-2.json") {
		t.Errorf("out.ResultPath = %q, want ending with result-2.json", out.ResultPath)
	}

	// prompt footer points to result-2.json
	promptPath := filepath.Join(laneDir, "prompt.md")
	promptBytes, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read prompt.md failed: %v", err)
	}
	wantResultPath, _ := filepath.Abs(filepath.Join(laneDir, "result-2.json"))
	wantPromptLine := fmt.Sprintf("Write your result envelope to `%s` following the result schema.", wantResultPath)
	if !strings.Contains(string(promptBytes), wantPromptLine) {
		t.Errorf("prompt.md did not contain %q, got: %s", wantPromptLine, string(promptBytes))
	}
	briefPath := filepath.Join(laneDir, "brief.md")
	if _, err := os.Stat(briefPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("brief.md should not exist, stat err = %v", err)
	}

	// result-1.json is untouched
	r1Bytes, err := os.ReadFile(result1Path)
	if err != nil {
		t.Fatalf("result-1.json missing: %v", err)
	}
	if string(r1Bytes) != `{"status": "done", "turn": 1}` {
		t.Errorf("result-1.json content changed: %s", string(r1Bytes))
	}

	// no result.prev.json is created
	if _, err := os.Stat(prevPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("result.prev.json should not exist, err=%v", err)
	}
}

func TestDispatch_Continuation_Turn2Lane_IncrementsToTurn3(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	createdLane, err := lane.Create(context.Background(), repoDir, []string{"pkg/**"}, "gemini-3.8-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	createdLane.PaneID = "w1:pContTurn2"
	createdLane.Status = lane.StatusFailed
	createdLane.Turn = 2
	if err := createdLane.Save(repoDir); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	laneDir := lane.LaneDir(repoDir, createdLane.ID)
	result1Path := filepath.Join(laneDir, "result-1.json")
	result2Path := filepath.Join(laneDir, "result-2.json")
	prevPath := filepath.Join(laneDir, "result.prev.json")
	_ = os.WriteFile(result1Path, []byte(`{"turn": 1}`), 0644)
	_ = os.WriteFile(result2Path, []byte(`{"turn": 2}`), 0644)

	runner := newFakeHerdrRunner()
	runner.handlers["agent prompt"] = func(args []string) ([]byte, error) {
		return []byte(`{"result": {"submitted": true}}`), nil
	}

	opts := dispatch.Options{
		Cwd:    repoDir,
		LaneID: createdLane.ID,
		Prompt: "Continue with turn 3",
		Detach: true,
	}

	out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}

	reloaded, err := lane.Load(repoDir, createdLane.ID)
	if err != nil {
		t.Fatalf("lane.Load failed: %v", err)
	}
	if reloaded.Turn != 3 {
		t.Errorf("reloaded.Turn = %d, want 3", reloaded.Turn)
	}
	if !strings.HasSuffix(out.ResultPath, "result-3.json") {
		t.Errorf("out.ResultPath = %q, want ending with result-3.json", out.ResultPath)
	}
	if _, err := os.Stat(prevPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("result.prev.json should not exist, err=%v", err)
	}
	if _, err := os.Stat(result1Path); err != nil {
		t.Errorf("result-1.json should still exist: %v", err)
	}
	if _, err := os.Stat(result2Path); err != nil {
		t.Errorf("result-2.json should still exist: %v", err)
	}
}
