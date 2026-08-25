package executor_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
)

// writeStub writes an executable shell script into t.TempDir() and returns
// its absolute path. Tests use this instead of invoking the real agy
// binary, so they never spend real quota and never touch the network.
func writeStub(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "agy-stub.sh")
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("WriteFile(stub) error = %v", err)
	}
	return path
}

func TestRunExitZero(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeStub(t, "#!/bin/sh\nexit 0\n")
	worktree := t.TempDir()

	a := executor.Agy{Binary: stub}
	outcome, err := a.Run(context.Background(), executor.Request{
		Prompt:       "do the thing",
		WorktreePath: worktree,
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if outcome.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", outcome.ExitCode)
	}
	if outcome.TimedOut {
		t.Errorf("TimedOut = true, want false")
	}
}

func TestRunNonZeroExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeStub(t, "#!/bin/sh\nexit 17\n")
	worktree := t.TempDir()

	a := executor.Agy{Binary: stub}
	outcome, err := a.Run(context.Background(), executor.Request{
		Prompt:       "do the thing",
		WorktreePath: worktree,
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if outcome.ExitCode != 17 {
		t.Errorf("ExitCode = %d, want 17", outcome.ExitCode)
	}
}

func TestRunCapturesStderr(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeStub(t, "#!/bin/sh\necho 'boom: something went wrong' 1>&2\nexit 1\n")
	worktree := t.TempDir()

	a := executor.Agy{Binary: stub}
	outcome, err := a.Run(context.Background(), executor.Request{
		Prompt:       "do the thing",
		WorktreePath: worktree,
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	const want = "boom: something went wrong\n"
	if outcome.Stderr != want {
		t.Errorf("Stderr = %q, want %q", outcome.Stderr, want)
	}
}

func TestRunUsesWorktreeAsWorkingDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	pwdFile := filepath.Join(t.TempDir(), "pwd.txt")
	script := fmt.Sprintf("#!/bin/sh\npwd > \"%s\"\n", pwdFile)
	stub := writeStub(t, script)
	worktree := t.TempDir()

	a := executor.Agy{Binary: stub}
	if _, err := a.Run(context.Background(), executor.Request{
		Prompt:       "do the thing",
		WorktreePath: worktree,
	}); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	got, err := os.ReadFile(pwdFile)
	if err != nil {
		t.Fatalf("ReadFile(pwd.txt) error = %v", err)
	}

	wantDir, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		t.Fatalf("EvalSymlinks(worktree) error = %v", err)
	}
	gotDir, err := filepath.EvalSymlinks(strings.TrimSpace(string(got)))
	if err != nil {
		t.Fatalf("EvalSymlinks(pwd output) error = %v", err)
	}

	if gotDir != wantDir {
		t.Errorf("child working directory = %q, want %q", gotDir, wantDir)
	}
}

func TestRunTimesOutWhenDeadlineIsShorterThanStub(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	// The stub sleeps far longer than the context deadline below. It must
	// not be allowed to make this test hang.
	stub := writeStub(t, "#!/bin/sh\nsleep 5\n")
	worktree := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	a := executor.Agy{Binary: stub}
	outcome, err := a.Run(ctx, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: worktree,
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if !outcome.TimedOut {
		t.Errorf("TimedOut = false, want true")
	}
}

// TestRunGrandchildHoldingPipesExitZeroReportsOutputTruncated is the test
// that would have caught the run-time defect where a successful dispatch
// was discarded as an error: the direct child exits immediately and
// cleanly with status 0, but backgrounds a grandchild that keeps the
// inherited stdout/stderr pipes open well past WaitDelay -- exactly the
// shape of a real agy dispatch spawning an MCP server subprocess. Run must
// report the real exit code and a truncation warning, never an error and
// never a discarded outcome.
func TestRunGrandchildHoldingPipesExitZeroReportsOutputTruncated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeStub(t, "#!/bin/sh\n( sleep 5 ) &\nexit 0\n")
	worktree := t.TempDir()

	a := executor.Agy{Binary: stub, WaitDelay: 50 * time.Millisecond}
	outcome, err := a.Run(context.Background(), executor.Request{
		Prompt:       "do the thing",
		WorktreePath: worktree,
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if outcome.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", outcome.ExitCode)
	}
	if outcome.TimedOut {
		t.Errorf("TimedOut = true, want false")
	}
	if !outcome.OutputTruncated {
		t.Errorf("OutputTruncated = false, want true")
	}
}

// TestRunGrandchildHoldingPipesNonZeroExitStillReportsRealExitCode proves
// the same grandchild-holds-the-pipes shape still surfaces the real,
// non-zero exit code rather than being swallowed into an error.
func TestRunGrandchildHoldingPipesNonZeroExitStillReportsRealExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeStub(t, "#!/bin/sh\n( sleep 5 ) &\nexit 17\n")
	worktree := t.TempDir()

	a := executor.Agy{Binary: stub, WaitDelay: 50 * time.Millisecond}
	outcome, err := a.Run(context.Background(), executor.Request{
		Prompt:       "do the thing",
		WorktreePath: worktree,
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if outcome.ExitCode != 17 {
		t.Errorf("ExitCode = %d, want 17", outcome.ExitCode)
	}
	if outcome.TimedOut {
		t.Errorf("TimedOut = true, want false")
	}
}

func TestRunBinaryDoesNotExist(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	missing := filepath.Join(t.TempDir(), "does-not-exist")
	worktree := t.TempDir()

	a := executor.Agy{Binary: missing}
	_, err := a.Run(context.Background(), executor.Request{
		Prompt:       "do the thing",
		WorktreePath: worktree,
	})
	if err == nil {
		t.Fatal("Run() error = nil, want a real error for a missing binary")
	}
}

func TestRunPassesPrintTimeoutAboveContextDeadline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argvFile := filepath.Join(t.TempDir(), "argv.txt")
	// $0 is the script path itself; the rest are the flags Run passed.
	script := fmt.Sprintf("#!/bin/sh\nfor a in \"$@\"; do echo \"$a\" >> \"%s\"; done\nexit 0\n", argvFile)
	stub := writeStub(t, script)
	worktree := t.TempDir()

	const ctxBudget = 30 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), ctxBudget)
	defer cancel()

	a := executor.Agy{Binary: stub}
	if _, err := a.Run(ctx, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: worktree,
	}); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	raw, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("ReadFile(argv.txt) error = %v", err)
	}
	argv := strings.Split(strings.TrimSpace(string(raw)), "\n")

	idx := -1
	for i, a := range argv {
		if a == "--print-timeout" {
			idx = i
			break
		}
	}
	if idx == -1 || idx+1 >= len(argv) {
		t.Fatalf("argv = %v, want a --print-timeout flag followed by a value", argv)
	}

	got, err := time.ParseDuration(argv[idx+1])
	if err != nil {
		t.Fatalf("--print-timeout value %q is not a valid duration: %v", argv[idx+1], err)
	}
	if got <= ctxBudget {
		t.Errorf("--print-timeout = %v, want strictly greater than context budget %v", got, ctxBudget)
	}
}

// captureArgv runs Agy against a stub that records its argv, one flag per
// line, and returns the recorded argv.
func captureArgv(t *testing.T, req executor.Request) []string {
	t.Helper()

	argvFile := filepath.Join(t.TempDir(), "argv.txt")
	script := fmt.Sprintf("#!/bin/sh\nfor a in \"$@\"; do echo \"$a\" >> \"%s\"; done\nexit 0\n", argvFile)
	stub := writeStub(t, script)

	a := executor.Agy{Binary: stub}
	if _, err := a.Run(context.Background(), req); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	raw, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("ReadFile(argv.txt) error = %v", err)
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

func containsFlag(argv []string, flag string) bool {
	for _, a := range argv {
		if a == flag {
			return true
		}
	}
	return false
}

// flagValue returns the value immediately following flag in argv, and
// whether flag was found at all with a value after it.
func flagValue(argv []string, flag string) (string, bool) {
	for i, a := range argv {
		if a == flag {
			if i+1 >= len(argv) {
				return "", false
			}
			return argv[i+1], true
		}
	}
	return "", false
}

func TestRunOmitsModelFlagWhenEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
	})
	if containsFlag(argv, "--model") {
		t.Errorf("argv = %v, want no --model flag when Request.Model is empty", argv)
	}
}

func TestAgyKnownModelsIncludesDefaultModel(t *testing.T) {
	a := executor.Agy{}
	known := a.KnownModels()
	if len(known) == 0 {
		t.Fatalf("KnownModels() = %v, want at least one model", known)
	}
	found := false
	for _, m := range known {
		if m == a.DefaultModel() {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("KnownModels() = %v, want it to include DefaultModel() %q", known, a.DefaultModel())
	}
}

func TestRunIncludesModelFlagWhenSet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
		Model:        "gemini-3.7-flash-high",
	})

	idx := -1
	for i, a := range argv {
		if a == "--model" {
			idx = i
			break
		}
	}
	if idx == -1 || idx+1 >= len(argv) {
		t.Fatalf("argv = %v, want a --model flag followed by a value", argv)
	}
	if argv[idx+1] != "gemini-3.7-flash-high" {
		t.Errorf("--model value = %q, want %q", argv[idx+1], "gemini-3.7-flash-high")
	}
}

// TestRunAlwaysPassesMandatoryFlags asserts every flag runtime.md documents
// as mandatory for a non-interactive dispatch is present on a plain Run,
// regardless of what the Request does or doesn't set. --dangerously-skip-
// permissions in particular is a live blocker, not a nicety: without it a
// headless run stalls on an interactive permission prompt and the lane
// dies on the wall clock for no reason.
func TestRunAlwaysPassesMandatoryFlags(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
	})

	for _, want := range []string{"--dangerously-skip-permissions"} {
		if !containsFlag(argv, want) {
			t.Errorf("argv = %v, want it to contain %q", argv, want)
		}
	}

	if got, ok := flagValue(argv, "--output-format"); !ok || got != "json" {
		t.Errorf("--output-format = (%q, %v), want (%q, true)", got, ok, "json")
	}
	if got, ok := flagValue(argv, "--mode"); !ok || got != "accept-edits" {
		t.Errorf("--mode = (%q, %v), want (%q, true)", got, ok, "accept-edits")
	}
}

func TestRunOmitsJSONSchemaFlagWhenSchemaPathEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
	})
	if containsFlag(argv, "--json-schema") {
		t.Errorf("argv = %v, want no --json-schema flag when Request.SchemaPath is empty", argv)
	}
}

func TestRunIncludesJSONSchemaFlagWhenSchemaPathSet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	schemaPath := filepath.Join(t.TempDir(), "result.schema.json")
	argv := captureArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
		SchemaPath:   schemaPath,
	})

	if got, ok := flagValue(argv, "--json-schema"); !ok || got != schemaPath {
		t.Errorf("--json-schema = (%q, %v), want (%q, true)", got, ok, schemaPath)
	}
}

func TestRunOmitsAddDirFlagWhenWorktreePathEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureArgv(t, executor.Request{
		Prompt: "do the thing",
	})
	if containsFlag(argv, "--add-dir") {
		t.Errorf("argv = %v, want no --add-dir flag when Request.WorktreePath is empty", argv)
	}
}

func TestRunIncludesAddDirFlagWithWorktreePath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	worktree := t.TempDir()
	argv := captureArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: worktree,
	})

	if got, ok := flagValue(argv, "--add-dir"); !ok || got != worktree {
		t.Errorf("--add-dir = (%q, %v), want (%q, true)", got, ok, worktree)
	}
}

func TestRunCapturesStdout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeStub(t, "#!/bin/sh\necho '{\"status\":\"done\"}'\nexit 0\n")
	worktree := t.TempDir()

	a := executor.Agy{Binary: stub}
	outcome, err := a.Run(context.Background(), executor.Request{
		Prompt:       "do the thing",
		WorktreePath: worktree,
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	const want = "{\"status\":\"done\"}\n"
	if outcome.Stdout != want {
		t.Errorf("Stdout = %q, want %q", outcome.Stdout, want)
	}
}

// TestAgyLinuxSetpgidSysProcAttr asserts that when Setpgid is true, Agy.Run
// configures the child in its own process group and context cancellation signals
// the entire process group, ensuring no grandchild or descendant processes survive.
func TestAgyLinuxSetpgidSysProcAttr(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	// The stub backgrounds a grandchild that sleeps 10s and writes its PID to pidFile,
	// then the parent stub sleeps 10s.
	script := fmt.Sprintf("#!/bin/sh\n( sleep 10 ) &\necho $! > \"%s\"\nsleep 10\n", pidFile)
	stub := writeStub(t, script)
	worktree := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	a := executor.Agy{Binary: stub, WaitDelay: 100 * time.Millisecond}
	outcome, err := a.Run(ctx, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: worktree,
		Setpgid:      true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if !outcome.TimedOut {
		t.Errorf("TimedOut = false, want true")
	}

	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("ReadFile(grandchild.pid) error = %v", err)
	}
	pidStr := strings.TrimSpace(string(raw))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		t.Fatalf("invalid grandchild pid %q: %v", pidStr, err)
	}

	// Ensure cleanup in case grandchild survived (e.g. before the fix).
	t.Cleanup(func() {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	})

	// Probe liveness of the grandchild PID. It must be dead (syscall.ESRCH).
	dead := false
	probeDeadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(probeDeadline) {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			dead = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !dead {
		t.Fatalf("grandchild process %d is still alive after context cancellation with Setpgid: true", pid)
	}
}

// TestAgyLinuxSetpgidPGIDCaptured asserts that when Setpgid is true,
// Outcome.PGID is populated with the real PID of the child process (which is
// the process group leader), and is 0 when Setpgid is false.
func TestAgyLinuxSetpgidPGIDCaptured(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	t.Run("SetpgidTruePopulatesPGID", func(t *testing.T) {
		pidFile := filepath.Join(t.TempDir(), "child.pid")
		script := fmt.Sprintf("#!/bin/sh\necho $$ > \"%s\"\nexit 0\n", pidFile)
		stub := writeStub(t, script)
		worktree := t.TempDir()

		a := executor.Agy{Binary: stub}
		outcome, err := a.Run(context.Background(), executor.Request{
			Prompt:       "do the thing",
			WorktreePath: worktree,
			Setpgid:      true,
		})
		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if outcome.ExitCode != 0 {
			t.Fatalf("ExitCode = %d, want 0", outcome.ExitCode)
		}

		raw, err := os.ReadFile(pidFile)
		if err != nil {
			t.Fatalf("ReadFile(child.pid) error = %v", err)
		}
		expectedPID, err := strconv.Atoi(strings.TrimSpace(string(raw)))
		if err != nil {
			t.Fatalf("invalid child pid %q: %v", string(raw), err)
		}
		if expectedPID <= 0 {
			t.Fatalf("expectedPID = %d, want > 0", expectedPID)
		}

		if outcome.PGID != expectedPID {
			t.Errorf("outcome.PGID = %d, want %d", outcome.PGID, expectedPID)
		}
	})

	t.Run("SetpgidFalseLeavesPGIDZero", func(t *testing.T) {
		stub := writeStub(t, "#!/bin/sh\nexit 0\n")
		worktree := t.TempDir()

		a := executor.Agy{Binary: stub}
		outcome, err := a.Run(context.Background(), executor.Request{
			Prompt:       "do the thing",
			WorktreePath: worktree,
			Setpgid:      false,
		})
		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if outcome.ExitCode != 0 {
			t.Fatalf("ExitCode = %d, want 0", outcome.ExitCode)
		}
		if outcome.PGID != 0 {
			t.Errorf("outcome.PGID = %d, want 0 when Setpgid is false", outcome.PGID)
		}
	})
}

// TestAgyLinuxSetpgidOnStartFiresMidFlight asserts that when Setpgid is true and
// OnStart is provided, Agy.Run invokes OnStart synchronously with the real OS PID
// while the child process is actively running (mid-flight), before waiting for exit.
func TestAgyLinuxSetpgidOnStartFiresMidFlight(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	t.Run("SetpgidTrueCallsOnStartMidFlight", func(t *testing.T) {
		pidFile := filepath.Join(t.TempDir(), "child.pid")
		script := fmt.Sprintf("#!/bin/sh\necho $$ > \"%s\"\nsleep 3\nexit 0\n", pidFile)
		stub := writeStub(t, script)
		worktree := t.TempDir()

		a := executor.Agy{Binary: stub}
		pidChan := make(chan int, 1)
		runDone := make(chan struct{})

		go func() {
			defer close(runDone)
			outcome, err := a.Run(context.Background(), executor.Request{
				Prompt:       "do the thing",
				WorktreePath: worktree,
				Setpgid:      true,
				OnStart: func(pid int) {
					pidChan <- pid
				},
			})
			if err != nil {
				t.Errorf("Run() error = %v, want nil", err)
			}
			if outcome.ExitCode != 0 {
				t.Errorf("ExitCode = %d, want 0", outcome.ExitCode)
			}
		}()

		select {
		case pid := <-pidChan:
			if pid <= 0 {
				t.Fatalf("OnStart pid = %d, want > 0", pid)
			}
			// Confirm the process is alive right now while the 3-second sleep stub is running.
			if err := syscall.Kill(pid, 0); err != nil {
				t.Errorf("syscall.Kill(%d, 0) = %v, want nil (process alive mid-flight)", pid, err)
			}
			// Verify runDone has not closed yet (confirms callback fired before Run completed).
			select {
			case <-runDone:
				t.Fatal("runDone closed prematurely: OnStart did not fire before process exit")
			default:
			}
		case <-time.After(1500 * time.Millisecond):
			t.Fatal("timed out waiting for OnStart to fire mid-flight")
		}

		<-runDone
	})

	t.Run("SetpgidFalseDoesNotCallOnStart", func(t *testing.T) {
		stub := writeStub(t, "#!/bin/sh\nexit 0\n")
		worktree := t.TempDir()

		called := false
		a := executor.Agy{Binary: stub}
		outcome, err := a.Run(context.Background(), executor.Request{
			Prompt:       "do the thing",
			WorktreePath: worktree,
			Setpgid:      false,
			OnStart: func(pid int) {
				called = true
			},
		})
		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if outcome.ExitCode != 0 {
			t.Fatalf("ExitCode = %d, want 0", outcome.ExitCode)
		}
		if called {
			t.Errorf("OnStart was called when Setpgid is false, want not called")
		}
	})
}
