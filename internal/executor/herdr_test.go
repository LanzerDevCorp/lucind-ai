package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// setupFakeAgy creates an executable fake agy binary in a temp directory.
func setupFakeAgy(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	binPath := filepath.Join(dir, "agy")
	script := `#!/bin/sh
if [ -n "$FAKE_AGY_DUMP_PROMPT" ]; then
  prev=""
  for arg in "$@"; do
    if [ "$prev" = "--print" ]; then
      printf "%s" "$arg" > "$FAKE_AGY_DUMP_PROMPT"
      break
    fi
    prev="$arg"
  done
fi

if [ -n "$FAKE_AGY_DUMP_ARGS" ]; then
  for arg in "$@"; do
    printf "%s\n" "$arg" >> "$FAKE_AGY_DUMP_ARGS"
  done
fi

if [ -n "$FAKE_AGY_STDOUT" ]; then
  cat "$FAKE_AGY_STDOUT"
elif [ -n "$FAKE_AGY_STDOUT_RAW" ]; then
  printf "%s" "$FAKE_AGY_STDOUT_RAW"
fi

if [ -n "$FAKE_AGY_STDERR" ]; then
  printf "%s" "$FAKE_AGY_STDERR" >&2
fi

exit "${FAKE_AGY_EXIT_CODE:-0}"
`
	if err := os.WriteFile(binPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake agy: %v", err)
	}
	return dir
}

// herdrCall records an invocation of the fake herdr CLI.
type herdrCall struct {
	args []string
}

// fakeHerdr records calls and executes commands when asked.
type fakeHerdr struct {
	mu           sync.Mutex
	calls        []herdrCall
	openResult   string
	openErr      error
	fakeAgyDir   string
	waitDelay    time.Duration
	waitOutErr   error
	waitOutFunc  func(ctx context.Context, pane string, regex string) ([]byte, error)
	typedCmds    []string
	sentKeysCall []string
}

func (f *fakeHerdr) cmd(ctx context.Context, args ...string) ([]byte, error) {
	f.mu.Lock()
	f.calls = append(f.calls, herdrCall{args: append([]string(nil), args...)})
	f.mu.Unlock()

	if len(args) >= 2 && args[0] == "worktree" && args[1] == "open" {
		if f.openErr != nil {
			return []byte(f.openResult), f.openErr
		}
		if f.openResult != "" {
			return []byte(f.openResult), nil
		}
		res := map[string]any{
			"result": map[string]any{
				"already_open": false,
				"workspace": map[string]any{
					"workspace_id": "w1",
				},
				"root_pane": map[string]any{
					"pane_id": "w1:p1",
				},
			},
		}
		b, _ := json.Marshal(res)
		return b, nil
	}

	if len(args) >= 3 && args[0] == "pane" && args[1] == "run" {
		cmdStr := args[3]
		f.mu.Lock()
		f.typedCmds = append(f.typedCmds, cmdStr)
		f.mu.Unlock()

		// Execute cmdStr using sh -c in a subprocess with PATH pointing to fakeAgyDir first.
		cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
		if f.fakeAgyDir != "" {
			cmd.Env = append(os.Environ(), "PATH="+f.fakeAgyDir+":"+os.Getenv("PATH"))
		}
		out, err := cmd.CombinedOutput()
		return out, err
	}

	if len(args) >= 2 && args[0] == "pane" && args[1] == "wait-output" {
		if f.waitOutFunc != nil {
			pane := args[2]
			var regex string
			for i, a := range args {
				if a == "--regex" && i+1 < len(args) {
					regex = args[i+1]
					break
				}
			}
			return f.waitOutFunc(ctx, pane, regex)
		}
		if f.waitDelay > 0 {
			select {
			case <-time.After(f.waitDelay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		if f.waitOutErr != nil {
			return nil, f.waitOutErr
		}
		return []byte("LUCIND_EXIT_sentinel=0\n"), nil
	}

	if len(args) >= 2 && args[0] == "pane" && args[1] == "send-keys" {
		f.mu.Lock()
		f.sentKeysCall = append(f.sentKeysCall, args...)
		f.mu.Unlock()
		return nil, nil
	}

	return nil, nil
}

func setupTestDirs(t *testing.T) (worktreeDir, repoDir, stateBaseDir string) {
	t.Helper()
	root := t.TempDir()
	worktreeDir = filepath.Join(root, "worktree")
	repoDir = filepath.Join(root, "repo")
	stateBaseDir = filepath.Join(root, "state")
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", stateBaseDir)
	return worktreeDir, repoDir, stateBaseDir
}

func TestHerdrAgySuccessJSON(t *testing.T) {
	worktreeDir, repoDir, stateBaseDir := setupTestDirs(t)
	fakeAgyDir := setupFakeAgy(t)

	jsonPayload := `{"status":"SUCCESS","response":"operation completed"}`
	t.Setenv("FAKE_AGY_STDOUT_RAW", jsonPayload)
	t.Setenv("FAKE_AGY_EXIT_CODE", "0")

	fHerdr := &fakeHerdr{fakeAgyDir: fakeAgyDir}
	h := HerdrAgy{
		cmd: fHerdr.cmd,
		gitCommonDir: func(ctx context.Context, wt string) (string, error) {
			return filepath.Join(repoDir, ".git"), nil
		},
	}

	req := Request{
		Prompt:        "hello world",
		WorktreePath:  worktreeDir,
		ReadOnlyPaths: []string{"read/only/path.txt"},
	}

	outcome, err := h.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("HerdrAgy.Run failed: %v", err)
	}

	if outcome.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", outcome.ExitCode)
	}
	if outcome.TimedOut {
		t.Errorf("TimedOut = true, want false")
	}
	if outcome.Stdout != jsonPayload {
		t.Errorf("Stdout = %q, want %q", outcome.Stdout, jsonPayload)
	}

	// Verify state dir was cleaned up on success
	entries, _ := os.ReadDir(filepath.Join(stateBaseDir, "lucind-ai", "herdr"))
	if len(entries) != 0 {
		t.Errorf("expected state dir to be removed on success, found %d entries", len(entries))
	}

	// Verify both --cwd and --path were passed to worktree open
	var openCall *herdrCall
	for _, call := range fHerdr.calls {
		if len(call.args) >= 2 && call.args[0] == "worktree" && call.args[1] == "open" {
			c := call
			openCall = &c
			break
		}
	}
	if openCall == nil {
		t.Fatal("worktree open was not called")
	}
	var hasCwd, hasPath bool
	for i, a := range openCall.args {
		if a == "--cwd" && i+1 < len(openCall.args) && openCall.args[i+1] == repoDir {
			hasCwd = true
		}
		if a == "--path" && i+1 < len(openCall.args) && openCall.args[i+1] == worktreeDir {
			hasPath = true
		}
	}
	if !hasCwd || !hasPath {
		t.Errorf("worktree open args %v missing --cwd %q or --path %q", openCall.args, repoDir, worktreeDir)
	}

	// Verify sentinel text does NOT appear in the typed command line
	if len(fHerdr.typedCmds) == 0 {
		t.Fatal("no typed commands recorded")
	}
	typed := fHerdr.typedCmds[0]
	if strings.Contains(typed, "LUCIND_EXIT_") {
		t.Errorf("typed command %q contains sentinel text LUCIND_EXIT_", typed)
	}
}

func TestHerdrAgySuccessStreamJSONWithProgress(t *testing.T) {
	worktreeDir, repoDir, _ := setupTestDirs(t)
	fakeAgyDir := setupFakeAgy(t)

	var streamBuf bytes.Buffer
	streamBuf.WriteString(`{"event":"init","init":{"model":"gemini-3.7-flash-high"}}` + "\n")
	streamBuf.WriteString(`{"event":"step_update","step_update":{"state":"ACTIVE","step_type":"tool","tool_name":"write_to_file","tool_info":{"name":"write_to_file","parameters":{"TargetFile":"main.go"}}}}` + "\n")
	streamBuf.WriteString(`{"event":"step_update","step_update":{"state":"DONE","step_type":"tool","tool_name":"write_to_file","tool_info":{"name":"write_to_file","parameters":{"TargetFile":"main.go"}}}}` + "\n")
	streamBuf.WriteString(`{"event":"result","result":{"status":"SUCCESS","usage":{"total_tokens":42}}}` + "\n")

	t.Setenv("FAKE_AGY_STDOUT_RAW", streamBuf.String())
	t.Setenv("FAKE_AGY_EXIT_CODE", "0")

	fHerdr := &fakeHerdr{fakeAgyDir: fakeAgyDir}
	h := HerdrAgy{
		PollInterval: 10 * time.Millisecond,
		cmd:          fHerdr.cmd,
		gitCommonDir: func(ctx context.Context, wt string) (string, error) {
			return filepath.Join(repoDir, ".git"), nil
		},
	}

	progressCh := make(chan ProgressEvent, 20)
	req := Request{
		Prompt:       "stream prompt",
		WorktreePath: worktreeDir,
		Progress:     progressCh,
	}

	outcome, err := h.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("HerdrAgy.Run failed: %v", err)
	}

	if outcome.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", outcome.ExitCode)
	}

	// Verify terminal result payload
	var resultObj map[string]any
	if err := json.Unmarshal([]byte(outcome.Stdout), &resultObj); err != nil {
		t.Fatalf("failed to parse terminal result stdout %q: %v", outcome.Stdout, err)
	}
	if resultObj["status"] != "SUCCESS" {
		t.Errorf("terminal result status = %v, want SUCCESS", resultObj["status"])
	}

	// Verify progress events were emitted
	close(progressCh)
	var events []ProgressEvent
	for ev := range progressCh {
		events = append(events, ev)
	}
	if len(events) == 0 {
		t.Errorf("expected progress events to be emitted, got 0")
	}
}

func TestHerdrAgyNonZeroExitSurfacesExitCodeAndStderr(t *testing.T) {
	worktreeDir, repoDir, stateBaseDir := setupTestDirs(t)
	fakeAgyDir := setupFakeAgy(t)

	t.Setenv("FAKE_AGY_EXIT_CODE", "42")
	t.Setenv("FAKE_AGY_STDERR", "fatal syntax error on line 12\n")

	fHerdr := &fakeHerdr{fakeAgyDir: fakeAgyDir}
	h := HerdrAgy{
		cmd: fHerdr.cmd,
		gitCommonDir: func(ctx context.Context, wt string) (string, error) {
			return filepath.Join(repoDir, ".git"), nil
		},
	}

	req := Request{
		Prompt:       "fail prompt",
		WorktreePath: worktreeDir,
	}

	outcome, err := h.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("expected non-zero exit to be an Outcome, not a Go error: %v", err)
	}

	if outcome.ExitCode != 42 {
		t.Errorf("ExitCode = %d, want 42", outcome.ExitCode)
	}
	if !strings.Contains(outcome.Stderr, "fatal syntax error on line 12") {
		t.Errorf("Stderr = %q, want it to contain error message", outcome.Stderr)
	}

	// Verify state dir was preserved on failure for debugging
	entries, _ := os.ReadDir(filepath.Join(stateBaseDir, "lucind-ai", "herdr"))
	if len(entries) == 0 {
		t.Errorf("expected state dir to be preserved on failure, but found none")
	}
}

func TestHerdrAgyTimeoutSendsCtrlCAndReturnsTimedOut(t *testing.T) {
	worktreeDir, repoDir, stateBaseDir := setupTestDirs(t)
	fakeAgyDir := setupFakeAgy(t)

	var fHerdr *fakeHerdr
	fHerdr = &fakeHerdr{
		fakeAgyDir: fakeAgyDir,
		waitOutFunc: func(ctx context.Context, pane string, regex string) ([]byte, error) {
			fHerdr.mu.Lock()
			hasCtrlC := len(fHerdr.sentKeysCall) > 0
			fHerdr.mu.Unlock()
			if hasCtrlC {
				return []byte("LUCIND_EXIT_sentinel=130\n"), nil
			}
			// Simulate initial wait timeout
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	h := HerdrAgy{
		cmd: fHerdr.cmd,
		gitCommonDir: func(ctx context.Context, wt string) (string, error) {
			return filepath.Join(repoDir, ".git"), nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	req := Request{
		Prompt:       "timeout prompt",
		WorktreePath: worktreeDir,
	}

	outcome, err := h.Run(ctx, req)
	if err != nil {
		t.Fatalf("expected timeout to return Outcome, not Go error: %v", err)
	}

	if !outcome.TimedOut {
		t.Errorf("TimedOut = false, want true")
	}

	// Verify send-keys C-c was called
	var sentCtrlC bool
	fHerdr.mu.Lock()
	for _, call := range fHerdr.calls {
		if len(call.args) >= 4 && call.args[0] == "pane" && call.args[1] == "send-keys" && call.args[3] == "C-c" {
			sentCtrlC = true
			break
		}
	}
	fHerdr.mu.Unlock()

	if !sentCtrlC {
		t.Errorf("expected herdr pane send-keys <pane> C-c to be called on timeout")
	}

	// Verify state dir is kept on timeout
	entries, _ := os.ReadDir(filepath.Join(stateBaseDir, "lucind-ai", "herdr"))
	if len(entries) == 0 {
		t.Errorf("expected state dir to be preserved on timeout, but found none")
	}
}

func TestHerdrAgyPromptNoShellInjection(t *testing.T) {
	worktreeDir, repoDir, _ := setupTestDirs(t)
	fakeAgyDir := setupFakeAgy(t)

	dumpFile := filepath.Join(t.TempDir(), "dumped_prompt.txt")
	t.Setenv("FAKE_AGY_DUMP_PROMPT", dumpFile)
	t.Setenv("FAKE_AGY_EXIT_CODE", "0")

	fHerdr := &fakeHerdr{fakeAgyDir: fakeAgyDir}
	h := HerdrAgy{
		cmd: fHerdr.cmd,
		gitCommonDir: func(ctx context.Context, wt string) (string, error) {
			return filepath.Join(repoDir, ".git"), nil
		},
	}

	trickyPrompt := "Line 1: single quote ' and double quote \" and `backticks`\n" +
		"Line 2: $(echo injected_cmd) and variable ${NOT_EXPANDED}\n" +
		"Line 3: '\\'\\'' trailing slashes and quotes\n" +
		"Line 4: ; rm -rf / ; done"

	req := Request{
		Prompt:       trickyPrompt,
		WorktreePath: worktreeDir,
	}

	outcome, err := h.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("HerdrAgy.Run failed: %v", err)
	}
	if outcome.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", outcome.ExitCode)
	}

	dumped, err := os.ReadFile(dumpFile)
	if err != nil {
		t.Fatalf("failed to read dumped prompt: %v", err)
	}

	if string(dumped) != trickyPrompt {
		t.Errorf("received prompt differs from input prompt!\nGot:\n%s\nWant:\n%s", string(dumped), trickyPrompt)
	}
}

func TestHerdrAgyAlreadyOpenReusesPane(t *testing.T) {
	worktreeDir, repoDir, _ := setupTestDirs(t)
	fakeAgyDir := setupFakeAgy(t)

	openResp := `{"result":{"already_open":true,"workspace":{"workspace_id":"w99"},"root_pane":{"pane_id":"w99:p4"}}}`
	fHerdr := &fakeHerdr{
		fakeAgyDir: fakeAgyDir,
		openResult: openResp,
	}
	h := HerdrAgy{
		cmd: fHerdr.cmd,
		gitCommonDir: func(ctx context.Context, wt string) (string, error) {
			return filepath.Join(repoDir, ".git"), nil
		},
	}

	req := Request{
		Prompt:       "already open prompt",
		WorktreePath: worktreeDir,
	}

	outcome, err := h.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("HerdrAgy.Run failed: %v", err)
	}
	if outcome.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", outcome.ExitCode)
	}

	// Verify pane run was executed against w99:p4
	var runPane string
	for _, call := range fHerdr.calls {
		if len(call.args) >= 3 && call.args[0] == "pane" && call.args[1] == "run" {
			runPane = call.args[2]
			break
		}
	}
	if runPane != "w99:p4" {
		t.Errorf("pane run target = %q, want w99:p4", runPane)
	}
}

func TestHerdrAgyHerdrJSONErrorBecomesWrappedGoError(t *testing.T) {
	worktreeDir, repoDir, _ := setupTestDirs(t)

	errResp := `{"error":{"code":"worktree_not_found","message":"worktree path not found"},"id":"cli:worktree:open"}`
	fHerdr := &fakeHerdr{
		openResult: errResp,
		openErr:    errors.New("exit status 1"),
	}
	h := HerdrAgy{
		cmd: fHerdr.cmd,
		gitCommonDir: func(ctx context.Context, wt string) (string, error) {
			return filepath.Join(repoDir, ".git"), nil
		},
	}

	req := Request{
		Prompt:       "err prompt",
		WorktreePath: worktreeDir,
	}

	_, err := h.Run(context.Background(), req)
	if err == nil {
		t.Fatal("expected Go error for worktree_not_found, got nil")
	}

	if !strings.Contains(err.Error(), "worktree_not_found") || !strings.Contains(err.Error(), "worktree path not found") {
		t.Errorf("error %q must carry the herdr code and message", err.Error())
	}
}

func TestHerdrAgyWaitOutputNonTimeoutErrorIsAGoError(t *testing.T) {
	worktreeDir, repoDir, _ := setupTestDirs(t)
	fakeAgyDir := setupFakeAgy(t)

	fHerdr := &fakeHerdr{
		fakeAgyDir: fakeAgyDir,
		waitOutFunc: func(ctx context.Context, pane string, regex string) ([]byte, error) {
			return []byte(`{"error":{"code":"pane_not_found","message":"pane w1:p1 not found"},"id":"cli:pane:wait-output"}`), errors.New("exit status 1")
		},
	}
	h := HerdrAgy{
		cmd: fHerdr.cmd,
		gitCommonDir: func(ctx context.Context, wt string) (string, error) {
			return filepath.Join(repoDir, ".git"), nil
		},
	}

	outcome, err := h.Run(context.Background(), Request{Prompt: "p", WorktreePath: worktreeDir})
	if err == nil {
		t.Fatalf("expected a Go error for pane_not_found, got outcome %+v", outcome)
	}
	if outcome.TimedOut {
		t.Error("a non-timeout wait failure must not be reported as TimedOut")
	}
	if !strings.Contains(err.Error(), "pane_not_found") {
		t.Errorf("error %q does not carry the herdr code", err.Error())
	}
	for _, call := range fHerdr.calls {
		if len(call.args) >= 2 && call.args[0] == "pane" && call.args[1] == "send-keys" {
			t.Error("must not send C-c for a non-timeout wait failure")
		}
	}
}

func TestHerdrAgyWaitOutputHerdrTimeoutCodeIsTimedOut(t *testing.T) {
	worktreeDir, repoDir, _ := setupTestDirs(t)
	fakeAgyDir := setupFakeAgy(t)

	var fHerdr *fakeHerdr
	fHerdr = &fakeHerdr{
		fakeAgyDir: fakeAgyDir,
		waitOutFunc: func(ctx context.Context, pane string, regex string) ([]byte, error) {
			fHerdr.mu.Lock()
			hasCtrlC := len(fHerdr.sentKeysCall) > 0
			fHerdr.mu.Unlock()
			if hasCtrlC {
				return []byte("LUCIND_EXIT_sentinel=130\n"), nil
			}
			return []byte(`{"error":{"code":"timeout","message":"timed out waiting for output match"},"id":"cli:pane:wait-output"}`), errors.New("exit status 1")
		},
	}
	h := HerdrAgy{
		cmd: fHerdr.cmd,
		gitCommonDir: func(ctx context.Context, wt string) (string, error) {
			return filepath.Join(repoDir, ".git"), nil
		},
	}

	outcome, err := h.Run(context.Background(), Request{Prompt: "p", WorktreePath: worktreeDir})
	if err != nil {
		t.Fatalf("a herdr timeout must return an Outcome, got error: %v", err)
	}
	if !outcome.TimedOut {
		t.Error("TimedOut = false, want true for herdr code timeout")
	}
}

func TestHerdrAgyModelDelegation(t *testing.T) {
	h := HerdrAgy{}
	agy := Agy{}

	if h.DefaultModel() != agy.DefaultModel() {
		t.Errorf("DefaultModel() = %q, want %q", h.DefaultModel(), agy.DefaultModel())
	}

	hKnown := h.KnownModels()
	agyKnown := agy.KnownModels()
	if len(hKnown) != len(agyKnown) {
		t.Fatalf("KnownModels() len = %d, want %d", len(hKnown), len(agyKnown))
	}
	for i := range hKnown {
		if hKnown[i] != agyKnown[i] {
			t.Errorf("KnownModels()[%d] = %q, want %q", i, hKnown[i], agyKnown[i])
		}
	}
}

func TestHerdrAgyFailedCdSkipsAgyAndReportsFailure(t *testing.T) {
	worktreeDir, repoDir, _ := setupTestDirs(t)
	fakeAgyDir := setupFakeAgy(t)
	marker := filepath.Join(t.TempDir(), "agy-ran")
	// A fake agy that records that it ran at all.
	if err := os.WriteFile(filepath.Join(fakeAgyDir, "agy"), []byte("#!/bin/sh\ntouch '"+marker+"'\necho '{}'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(worktreeDir); err != nil {
		t.Fatal(err)
	}

	fHerdr := &fakeHerdr{fakeAgyDir: fakeAgyDir}
	h := HerdrAgy{
		cmd: fHerdr.cmd,
		gitCommonDir: func(ctx context.Context, wt string) (string, error) {
			return filepath.Join(repoDir, ".git"), nil
		},
	}
	outcome, err := h.Run(context.Background(), Request{Prompt: "p", WorktreePath: worktreeDir})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if outcome.ExitCode == 0 {
		t.Fatal("a failed cd must not report success")
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("agy must not run when the worktree cannot be entered")
	}
}

func TestHerdrAgyUnparseableExitCodeIsAGoError(t *testing.T) {
	worktreeDir, repoDir, stateBaseDir := setupTestDirs(t)
	fakeAgyDir := setupFakeAgy(t)

	fHerdr := &fakeHerdr{
		fakeAgyDir: fakeAgyDir,
		waitOutFunc: func(ctx context.Context, pane string, regex string) ([]byte, error) {
			matches, _ := filepath.Glob(filepath.Join(stateBaseDir, "lucind-ai", "herdr", "run-*", "exit.code"))
			for _, m := range matches {
				_ = os.WriteFile(m, []byte("garbage"), 0o600)
			}
			return []byte("LUCIND_EXIT_sentinel=0\n"), nil
		},
	}
	h := HerdrAgy{
		cmd: fHerdr.cmd,
		gitCommonDir: func(ctx context.Context, wt string) (string, error) {
			return filepath.Join(repoDir, ".git"), nil
		},
	}
	_, err := h.Run(context.Background(), Request{Prompt: "p", WorktreePath: worktreeDir})
	if err == nil || !strings.Contains(err.Error(), "exit.code") {
		t.Fatalf("expected an exit.code parse error, got %v", err)
	}
}
