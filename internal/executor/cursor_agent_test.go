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

// writeCursorStub writes an executable shell script into t.TempDir() and returns
// its absolute path. Tests use this instead of invoking the real cursor-agent
// binary, so they never spend real quota and never touch the network.
func writeCursorStub(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "cursor-agent-stub.sh")
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("WriteFile(stub) error = %v", err)
	}
	return path
}

func TestCursorAgentRunExitZero(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeCursorStub(t, "#!/bin/sh\nexit 0\n")
	worktree := t.TempDir()

	c := executor.CursorAgent{Binary: stub}
	outcome, err := c.Run(context.Background(), executor.Request{
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

func TestCursorAgentRunNonZeroExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeCursorStub(t, "#!/bin/sh\nexit 17\n")
	worktree := t.TempDir()

	c := executor.CursorAgent{Binary: stub}
	outcome, err := c.Run(context.Background(), executor.Request{
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

func TestCursorAgentRunCapturesStderr(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeCursorStub(t, "#!/bin/sh\necho 'boom: something went wrong' 1>&2\nexit 1\n")
	worktree := t.TempDir()

	c := executor.CursorAgent{Binary: stub}
	outcome, err := c.Run(context.Background(), executor.Request{
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

func TestCursorAgentRunCapturesStdout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeCursorStub(t, "#!/bin/sh\necho '{\"type\":\"result\",\"subtype\":\"success\"}'\nexit 0\n")
	worktree := t.TempDir()

	c := executor.CursorAgent{Binary: stub}
	outcome, err := c.Run(context.Background(), executor.Request{
		Prompt:       "do the thing",
		WorktreePath: worktree,
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	const want = "{\"type\":\"result\",\"subtype\":\"success\"}\n"
	if outcome.Stdout != want {
		t.Errorf("Stdout = %q, want %q", outcome.Stdout, want)
	}
}

func TestCursorAgentRunUsesWorktreeAsWorkingDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	pwdFile := filepath.Join(t.TempDir(), "pwd.txt")
	script := fmt.Sprintf("#!/bin/sh\npwd > \"%s\"\n", pwdFile)
	stub := writeCursorStub(t, script)
	worktree := t.TempDir()

	c := executor.CursorAgent{Binary: stub}
	if _, err := c.Run(context.Background(), executor.Request{
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

func TestCursorAgentRunTimesOutWhenDeadlineIsShorterThanStub(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeCursorStub(t, "#!/bin/sh\nsleep 5\n")
	worktree := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	c := executor.CursorAgent{Binary: stub}
	outcome, err := c.Run(ctx, executor.Request{
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

func TestCursorAgentRunGrandchildHoldingPipesExitZeroReportsOutputTruncated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeCursorStub(t, "#!/bin/sh\n( sleep 5 ) &\nexit 0\n")
	worktree := t.TempDir()

	c := executor.CursorAgent{Binary: stub, WaitDelay: 50 * time.Millisecond}
	outcome, err := c.Run(context.Background(), executor.Request{
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

func TestCursorAgentRunGrandchildHoldingPipesNonZeroExitStillReportsRealExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeCursorStub(t, "#!/bin/sh\n( sleep 5 ) &\nexit 17\n")
	worktree := t.TempDir()

	c := executor.CursorAgent{Binary: stub, WaitDelay: 50 * time.Millisecond}
	outcome, err := c.Run(context.Background(), executor.Request{
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

func TestCursorAgentRunBinaryDoesNotExist(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	missing := filepath.Join(t.TempDir(), "does-not-exist")
	worktree := t.TempDir()

	c := executor.CursorAgent{Binary: missing}
	_, err := c.Run(context.Background(), executor.Request{
		Prompt:       "do the thing",
		WorktreePath: worktree,
	})
	if err == nil {
		t.Fatal("Run() error = nil, want a real error for a missing binary")
	}
}

// captureCursorArgv runs CursorAgent against a stub that records its argv, one flag per
// line, and returns the recorded argv.
func captureCursorArgv(t *testing.T, req executor.Request) []string {
	t.Helper()

	argvFile := filepath.Join(t.TempDir(), "argv.txt")
	script := fmt.Sprintf("#!/bin/sh\nfor a in \"$@\"; do echo \"$a\" >> \"%s\"; done\nexit 0\n", argvFile)
	stub := writeCursorStub(t, script)

	c := executor.CursorAgent{Binary: stub}
	if _, err := c.Run(context.Background(), req); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	raw, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("ReadFile(argv.txt) error = %v", err)
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

func TestCursorAgentRunAlwaysPassesMandatoryFlags(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureCursorArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
	})

	for _, want := range []string{"--trust", "--force", "--approve-mcps"} {
		if !containsFlag(argv, want) {
			t.Errorf("argv = %v, want it to contain %q", argv, want)
		}
	}

	if got, ok := flagValue(argv, "--print"); !ok || got != "do the thing" {
		t.Errorf("--print = (%q, %v), want (%q, true)", got, ok, "do the thing")
	}
	if got, ok := flagValue(argv, "--output-format"); !ok || got != "json" {
		t.Errorf("--output-format = (%q, %v), want (%q, true)", got, ok, "json")
	}
}

func TestCursorAgentKnownModelsIncludesDefaultAndTestedEscalation(t *testing.T) {
	c := executor.CursorAgent{}
	known := c.KnownModels()
	if len(known) == 0 {
		t.Fatalf("KnownModels() = %v, want at least one model", known)
	}
	wantDefault, wantEscalation := false, false
	for _, m := range known {
		if m == c.DefaultModel() {
			wantDefault = true
		}
		if m == "claude-opus-4-8-high" {
			wantEscalation = true
		}
	}
	if !wantDefault {
		t.Errorf("KnownModels() = %v, want it to include DefaultModel() %q", known, c.DefaultModel())
	}
	if !wantEscalation {
		t.Errorf("KnownModels() = %v, want it to include the deliberately-tested escalation model %q (see TestCursorAgentRunIncludesModelFlagWhenSet)", known, "claude-opus-4-8-high")
	}
}

func TestCursorAgentKnownModelsExcludesOtherProviderFamilies(t *testing.T) {
	c := executor.CursorAgent{}
	known := c.KnownModels()
	for _, m := range known {
		if strings.HasPrefix(m, "gemini-") {
			t.Errorf("KnownModels() = %v, want no gemini- model -- that is agy's provider family, not cursor-agent's; this is the exact mismatch that silently billed against Cursor's Other Models quota", known)
		}
	}
}

func TestCursorAgentRunOmitsModelFlagWhenEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureCursorArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
	})
	if containsFlag(argv, "--model") {
		t.Errorf("argv = %v, want no --model flag when Request.Model is empty", argv)
	}
}

func TestCursorAgentRunIncludesModelFlagWhenSet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureCursorArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
		Model:        "claude-opus-4-8-high",
	})

	if got, ok := flagValue(argv, "--model"); !ok || got != "claude-opus-4-8-high" {
		t.Errorf("--model = (%q, %v), want (%q, true)", got, ok, "claude-opus-4-8-high")
	}
}

func TestCursorAgentRunOmitsWorkspaceFlagWhenWorktreePathEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureCursorArgv(t, executor.Request{
		Prompt: "do the thing",
	})
	if containsFlag(argv, "--workspace") {
		t.Errorf("argv = %v, want no --workspace flag when Request.WorktreePath is empty", argv)
	}
}

func TestCursorAgentRunIncludesWorkspaceFlagWithWorktreePath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	worktree := t.TempDir()
	argv := captureCursorArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: worktree,
	})

	if got, ok := flagValue(argv, "--workspace"); !ok || got != worktree {
		t.Errorf("--workspace = (%q, %v), want (%q, true)", got, ok, worktree)
	}
}

func TestCursorAgentRunNeverPassesJSONSchemaFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	schemaPath := filepath.Join(t.TempDir(), "result.schema.json")
	argv := captureCursorArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
		SchemaPath:   schemaPath,
	})

	if containsFlag(argv, "--json-schema") || containsFlag(argv, "--schema") {
		t.Errorf("argv = %v, want no json schema flag when SchemaPath is provided to CursorAgent", argv)
	}
}
