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

// writeOpencodeStub writes an executable shell script into t.TempDir() and returns
// its absolute path. Tests use this instead of invoking the real opencode
// binary, so they never spend real quota and never touch the network.
func writeOpencodeStub(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode-stub.sh")
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("WriteFile(stub) error = %v", err)
	}
	return path
}

func TestOpencodeRunExitZero(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeOpencodeStub(t, "#!/bin/sh\nexit 0\n")
	worktree := t.TempDir()

	o := executor.Opencode{Binary: stub}
	outcome, err := o.Run(context.Background(), executor.Request{
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

func TestOpencodeRunNonZeroExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeOpencodeStub(t, "#!/bin/sh\nexit 17\n")
	worktree := t.TempDir()

	o := executor.Opencode{Binary: stub}
	outcome, err := o.Run(context.Background(), executor.Request{
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

func TestOpencodeRunCapturesStderr(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeOpencodeStub(t, "#!/bin/sh\necho 'boom: something went wrong' 1>&2\nexit 1\n")
	worktree := t.TempDir()

	o := executor.Opencode{Binary: stub}
	outcome, err := o.Run(context.Background(), executor.Request{
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

func TestOpencodeRunCapturesStdout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeOpencodeStub(t, "#!/bin/sh\necho '{\"type\":\"result\",\"subtype\":\"success\"}'\nexit 0\n")
	worktree := t.TempDir()

	o := executor.Opencode{Binary: stub}
	outcome, err := o.Run(context.Background(), executor.Request{
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

func TestOpencodeRunUsesWorktreeAsWorkingDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	pwdFile := filepath.Join(t.TempDir(), "pwd.txt")
	script := fmt.Sprintf("#!/bin/sh\npwd > \"%s\"\n", pwdFile)
	stub := writeOpencodeStub(t, script)
	worktree := t.TempDir()

	o := executor.Opencode{Binary: stub}
	if _, err := o.Run(context.Background(), executor.Request{
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

func TestOpencodeRunTimesOutWhenDeadlineIsShorterThanStub(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeOpencodeStub(t, "#!/bin/sh\nsleep 5\n")
	worktree := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	o := executor.Opencode{Binary: stub}
	outcome, err := o.Run(ctx, executor.Request{
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

func TestOpencodeRunGrandchildHoldingPipesExitZeroReportsOutputTruncated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeOpencodeStub(t, "#!/bin/sh\n( sleep 5 ) &\nexit 0\n")
	worktree := t.TempDir()

	o := executor.Opencode{Binary: stub, WaitDelay: 50 * time.Millisecond}
	outcome, err := o.Run(context.Background(), executor.Request{
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

func TestOpencodeRunGrandchildHoldingPipesNonZeroExitStillReportsRealExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeOpencodeStub(t, "#!/bin/sh\n( sleep 5 ) &\nexit 17\n")
	worktree := t.TempDir()

	o := executor.Opencode{Binary: stub, WaitDelay: 50 * time.Millisecond}
	outcome, err := o.Run(context.Background(), executor.Request{
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

func TestOpencodeRunBinaryDoesNotExist(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	missing := filepath.Join(t.TempDir(), "does-not-exist")
	worktree := t.TempDir()

	o := executor.Opencode{Binary: missing}
	_, err := o.Run(context.Background(), executor.Request{
		Prompt:       "do the thing",
		WorktreePath: worktree,
	})
	if err == nil {
		t.Fatal("Run() error = nil, want a real error for a missing binary")
	}
}

// captureOpencodeArgv runs Opencode against a stub that records its argv, one flag per
// line, and returns the recorded argv.
func captureOpencodeArgv(t *testing.T, req executor.Request) []string {
	t.Helper()

	argvFile := filepath.Join(t.TempDir(), "argv.txt")
	script := fmt.Sprintf("#!/bin/sh\nfor a in \"$@\"; do echo \"$a\" >> \"%s\"; done\nexit 0\n", argvFile)
	stub := writeOpencodeStub(t, script)

	o := executor.Opencode{Binary: stub}
	if _, err := o.Run(context.Background(), req); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	raw, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("ReadFile(argv.txt) error = %v", err)
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

func TestOpencodeRunAlwaysPassesMandatoryFlags(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureOpencodeArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
	})

	if len(argv) == 0 || argv[0] != "do the thing" {
		t.Errorf("argv[0] = %v, want the prompt as the first positional argument %q", argv, "do the thing")
	}
	for _, want := range []string{"--auto"} {
		if !containsFlag(argv, want) {
			t.Errorf("argv = %v, want it to contain %q", argv, want)
		}
	}
	if got, ok := flagValue(argv, "--format"); !ok || got != "json" {
		t.Errorf("--format = (%q, %v), want (%q, true)", got, ok, "json")
	}
}

func TestOpencodeKnownModelsIsGptSolOnly(t *testing.T) {
	o := executor.Opencode{}
	known := o.KnownModels()
	if len(known) != 1 || known[0] != o.DefaultModel() {
		t.Errorf("KnownModels() = %v, want exactly [%q] -- opencode must never escalate to another provider's model", known, o.DefaultModel())
	}
}

func TestOpencodeKnownModelsExcludesOtherProviderFamilies(t *testing.T) {
	o := executor.Opencode{}
	known := o.KnownModels()
	for _, m := range known {
		if strings.HasPrefix(m, "gemini-") {
			t.Errorf("KnownModels() = %v, want no gemini- model -- that is agy's provider family, not opencode's", known)
		}
		if strings.HasPrefix(m, "cursor-") {
			t.Errorf("KnownModels() = %v, want no cursor- model -- that is cursor-agent's provider family, not opencode's", known)
		}
	}
}

func TestOpencodeRunOmitsModelFlagWhenEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureOpencodeArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
	})
	if containsFlag(argv, "--model") {
		t.Errorf("argv = %v, want no --model flag when Request.Model is empty", argv)
	}
}

func TestOpencodeRunIncludesModelFlagWhenSet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureOpencodeArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
		Model:        "openai/gpt-5.6-sol",
	})

	if got, ok := flagValue(argv, "--model"); !ok || got != "openai/gpt-5.6-sol" {
		t.Errorf("--model = (%q, %v), want (%q, true)", got, ok, "openai/gpt-5.6-sol")
	}
}

func TestOpencodeRunOmitsDirFlagWhenWorktreePathEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureOpencodeArgv(t, executor.Request{
		Prompt: "do the thing",
	})
	if containsFlag(argv, "--dir") {
		t.Errorf("argv = %v, want no --dir flag when Request.WorktreePath is empty", argv)
	}
}

func TestOpencodeRunIncludesDirFlagWithWorktreePath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	worktree := t.TempDir()
	argv := captureOpencodeArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: worktree,
	})

	if got, ok := flagValue(argv, "--dir"); !ok || got != worktree {
		t.Errorf("--dir = (%q, %v), want (%q, true)", got, ok, worktree)
	}
}

func TestOpencodeRunNeverPassesJSONSchemaFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	schemaPath := filepath.Join(t.TempDir(), "result.schema.json")
	argv := captureOpencodeArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
		SchemaPath:   schemaPath,
	})

	if containsFlag(argv, "--json-schema") || containsFlag(argv, "--schema") {
		t.Errorf("argv = %v, want no json schema flag when SchemaPath is provided to Opencode", argv)
	}
}

func TestOpencodeRunOmitsAgentFlagWhenEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureOpencodeArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
	})
	if containsFlag(argv, "--agent") {
		t.Errorf("argv = %v, want no --agent flag when Request.Agent is empty", argv)
	}
}

func TestOpencodeRunIncludesAgentFlagWhenSet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureOpencodeArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
		Agent:        "lucind-dag",
	})

	if got, ok := flagValue(argv, "--agent"); !ok || got != "lucind-dag" {
		t.Errorf("--agent = (%q, %v), want (%q, true)", got, ok, "lucind-dag")
	}
}
