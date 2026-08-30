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

// writeClaudeStub writes an executable shell script into t.TempDir() and
// returns its absolute path. Tests use this instead of invoking the real
// claude binary, so they never spend real quota and never touch the network.
func writeClaudeStub(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "claude-stub.sh")
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("WriteFile(stub) error = %v", err)
	}
	return path
}

func captureClaudeArgv(t *testing.T, req executor.Request) []string {
	t.Helper()

	argvFile := filepath.Join(t.TempDir(), "argv.txt")
	script := fmt.Sprintf("#!/bin/sh\nfor a in \"$@\"; do echo \"$a\" >> \"%s\"; done\nexit 0\n", argvFile)
	stub := writeClaudeStub(t, script)

	c := executor.Claude{Binary: stub}
	if _, err := c.Run(context.Background(), req); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	raw, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("ReadFile(argv.txt) error = %v", err)
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

func TestClaudeDefaultModelIsOpus(t *testing.T) {
	c := executor.Claude{}
	if got, want := c.DefaultModel(), "claude-opus-5"; got != want {
		t.Errorf("DefaultModel() = %q, want %q", got, want)
	}
}

func TestClaudeKnownModelsIsOpusOnly(t *testing.T) {
	c := executor.Claude{}
	known := c.KnownModels()
	if len(known) != 1 || known[0] != c.DefaultModel() {
		t.Errorf("KnownModels() = %v, want exactly [%q] -- claude must never escalate to another provider's model", known, c.DefaultModel())
	}
}

// TestClaudeKnownModelsExcludesOtherProviderFamilies mirrors the identical
// guard on every other executor: an executor exists to reach exactly one
// provider family, so a copy-pasted model string from a sibling executor
// must never silently run -- and bill -- here.
func TestClaudeKnownModelsExcludesOtherProviderFamilies(t *testing.T) {
	c := executor.Claude{}
	for _, m := range c.KnownModels() {
		if strings.HasPrefix(m, "gemini-") {
			t.Errorf("KnownModels() = %v, want no gemini- model -- that is agy's provider family, not claude's", c.KnownModels())
		}
		if strings.HasPrefix(m, "cursor-") {
			t.Errorf("KnownModels() = %v, want no cursor- model -- that is cursor-agent's provider family, not claude's", c.KnownModels())
		}
		if strings.HasPrefix(m, "openai/") {
			t.Errorf("KnownModels() = %v, want no openai/ model -- that is opencode's provider family, not claude's", c.KnownModels())
		}
	}
}

func TestClaudeRunExitZero(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeClaudeStub(t, "#!/bin/sh\nexit 0\n")
	c := executor.Claude{Binary: stub}
	outcome, err := c.Run(context.Background(), executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
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

func TestClaudeRunNonZeroExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeClaudeStub(t, "#!/bin/sh\nexit 17\n")
	c := executor.Claude{Binary: stub}
	outcome, err := c.Run(context.Background(), executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if outcome.ExitCode != 17 {
		t.Errorf("ExitCode = %d, want 17", outcome.ExitCode)
	}
}

func TestClaudeRunCapturesStderr(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeClaudeStub(t, "#!/bin/sh\necho 'boom: something went wrong' 1>&2\nexit 1\n")
	c := executor.Claude{Binary: stub}
	outcome, err := c.Run(context.Background(), executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	const want = "boom: something went wrong\n"
	if outcome.Stderr != want {
		t.Errorf("Stderr = %q, want %q", outcome.Stderr, want)
	}
}

func TestClaudeRunSendsHeadlessFlagSet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureClaudeArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
	})

	if !containsFlag(argv, "--print") {
		t.Errorf("argv = %v, want --print -- without it claude starts an interactive session and the lane hangs", argv)
	}
	if got, ok := flagValue(argv, "--output-format"); !ok || got != "json" {
		t.Errorf("argv = %v, want --output-format json, got %q (ok=%v)", argv, got, ok)
	}
	if got, ok := flagValue(argv, "--permission-mode"); !ok || got != "acceptEdits" {
		t.Errorf("argv = %v, want --permission-mode acceptEdits, got %q (ok=%v)", argv, got, ok)
	}
	if !containsFlag(argv, "--dangerously-skip-permissions") {
		t.Errorf("argv = %v, want --dangerously-skip-permissions -- without it a headless run stalls on an interactive permission prompt", argv)
	}
}

// TestClaudeRunPassesPromptAfterDoubleDash pins the one argv detail that is
// not cosmetic. claude's --print is a boolean flag, so unlike agy the prompt
// cannot ride as its value and must be positional. A packet body is Markdown
// and may legitimately begin with "-" (a bullet list), which claude's own
// parser would read as an unknown option and reject before dispatching --
// verified against the real CLI, which answers `--definitely-not-a-flag`
// with "unknown option" bare, but treats it as a prompt behind "--".
func TestClaudeRunPassesPromptAfterDoubleDash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	const prompt = "- resolve the conflict"
	argv := captureClaudeArgv(t, executor.Request{
		Prompt:       prompt,
		WorktreePath: t.TempDir(),
	})

	if len(argv) < 2 {
		t.Fatalf("argv = %v, want at least a separator and a prompt", argv)
	}
	if got := argv[len(argv)-1]; got != prompt {
		t.Errorf("argv[last] = %q, want the prompt %q -- the prompt must be the final positional", got, prompt)
	}
	if got := argv[len(argv)-2]; got != "--" {
		t.Errorf("argv[last-1] = %q, want %q -- a Markdown body starting with '-' is parsed as a flag without it", got, "--")
	}
}

func TestClaudeRunIncludesModelFlagWhenSet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureClaudeArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
		Model:        "claude-opus-5",
	})
	if got, ok := flagValue(argv, "--model"); !ok || got != "claude-opus-5" {
		t.Errorf("argv = %v, want --model claude-opus-5, got %q (ok=%v)", argv, got, ok)
	}
}

func TestClaudeRunOmitsModelFlagWhenEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureClaudeArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
	})
	if containsFlag(argv, "--model") {
		t.Errorf("argv = %v, want no --model flag when Request.Model is empty", argv)
	}
}

func TestClaudeRunIncludesAddDirForWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	worktree := t.TempDir()
	argv := captureClaudeArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: worktree,
	})
	if got, ok := flagValue(argv, "--add-dir"); !ok || got != worktree {
		t.Errorf("argv = %v, want --add-dir %q, got %q (ok=%v)", argv, worktree, got, ok)
	}
}

func TestClaudeRunIncludesJSONSchemaWhenSet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	schema := filepath.Join(t.TempDir(), "result.schema.json")
	argv := captureClaudeArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
		SchemaPath:   schema,
	})
	if got, ok := flagValue(argv, "--json-schema"); !ok || got != schema {
		t.Errorf("argv = %v, want --json-schema %q, got %q (ok=%v)", argv, schema, got, ok)
	}
}

func TestClaudeRunOmitsJSONSchemaWhenEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureClaudeArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
	})
	if containsFlag(argv, "--json-schema") {
		t.Errorf("argv = %v, want no --json-schema flag when Request.SchemaPath is empty", argv)
	}
}

// TestClaudeRunNeverSendsPrintTimeout is a regression guard, not a style
// preference. agy exposes --print-timeout and Agy.Run sets it above the
// context deadline; claude has no such flag (verified against `claude
// --help`), and claude rejects unknown options outright, so copying agy's
// line here would make every single dispatch exit 1 before running.
// The context remains the only clock, exactly as for cursor-agent and
// opencode.
func TestClaudeRunNeverSendsPrintTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	argvFile := filepath.Join(t.TempDir(), "argv.txt")
	script := fmt.Sprintf("#!/bin/sh\nfor a in \"$@\"; do echo \"$a\" >> \"%s\"; done\nexit 0\n", argvFile)
	stub := writeClaudeStub(t, script)

	c := executor.Claude{Binary: stub}
	if _, err := c.Run(ctx, executor.Request{Prompt: "do the thing", WorktreePath: t.TempDir()}); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	raw, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("ReadFile(argv.txt) error = %v", err)
	}
	argv := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if containsFlag(argv, "--print-timeout") {
		t.Errorf("argv = %v, want no --print-timeout -- claude has no such flag and rejects unknown options", argv)
	}
}

func TestClaudeRunTimesOut(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeClaudeStub(t, "#!/bin/sh\nsleep 30\n")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	c := executor.Claude{Binary: stub, WaitDelay: 50 * time.Millisecond}
	outcome, err := c.Run(ctx, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if !outcome.TimedOut {
		t.Errorf("TimedOut = false, want true -- ctx is the only clock claude dispatches under")
	}
}

func TestClaudeRunReturnsErrorWhenBinaryMissing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	c := executor.Claude{Binary: filepath.Join(t.TempDir(), "does-not-exist")}
	if _, err := c.Run(context.Background(), executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
	}); err == nil {
		t.Errorf("Run() error = nil, want a real error -- a process that never ran is not an Outcome")
	}
}
