package executor_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
