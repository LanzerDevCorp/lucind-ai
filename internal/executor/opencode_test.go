package executor_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

// TestOpencodeRunAlwaysPassesRunSubcommandFirst is a regression test for a
// real defect: opencode's default bare command starts the interactive TUI
// (`opencode [project]`), not a dispatch -- only `opencode run [message..]`
// accepts --model/--agent/--format/--dir/--auto (verified against
// `opencode run --help`, v1.18.19). Omitting "run" makes every one of those
// flags unknown to the default command, which exits 1 printing top-level
// usage before any real dispatch happens. This asserts the subcommand's
// exact position, not merely that flags are present somewhere in argv --
// the prior version of this test asserted argv[0] was the prompt itself,
// which is what let the missing-subcommand bug ship unnoticed.
func TestOpencodeRunAlwaysPassesRunSubcommandFirst(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureOpencodeArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
	})

	if len(argv) == 0 || argv[0] != "run" {
		t.Fatalf("argv[0] = %v, want the \"run\" subcommand first", argv)
	}
	if len(argv) < 2 || argv[1] != "do the thing" {
		t.Errorf("argv[1] = %v, want the prompt as run's first positional argument %q", argv, "do the thing")
	}
}

func TestOpencodeRunAlwaysPassesMandatoryFlags(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	argv := captureOpencodeArgv(t, executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
	})

	for _, want := range []string{"--auto"} {
		if !containsFlag(argv, want) {
			t.Errorf("argv = %v, want it to contain %q", argv, want)
		}
	}
	if got, ok := flagValue(argv, "--format"); !ok || got != "json" {
		t.Errorf("--format = (%q, %v), want (%q, true)", got, ok, "json")
	}
}

// TestOpencodeKnownModelsIncludesSolAndLuna pins the model registry to
// exactly the two verified openai/gpt-5.6 variants Opencode may dispatch:
// its own default, sol, and luna -- the model the fan-out synthesizer is
// slated to move to once its prompt is tuned (see opencode.go's
// KnownModels doc comment). Both were verified against the installed real
// CLI's own model list (`opencode models`, 1.18.21).
//
// Adding luna is not a relaxation of the invariant this test used to pin
// under its old name (TestOpencodeKnownModelsIsGptSolOnly, "exactly one
// entry"): the real invariant, per KnownModels' doc comment on the shared
// Executor interface (executor.go) and
// TestEveryExecutorOwnsExactlyOneProviderFamily
// (cmd/lucind-ai/cli_test.go), is provider-family exclusivity -- no two
// executors may share a model string -- never a cap on how many models one
// executor may run.
func TestOpencodeKnownModelsIncludesSolAndLuna(t *testing.T) {
	o := executor.Opencode{}
	known := o.KnownModels()
	want := []string{"openai/gpt-5.6-sol", "openai/gpt-5.6-luna"}
	if !reflect.DeepEqual(known, want) {
		t.Errorf("KnownModels() = %v, want %v", known, want)
	}
	if o.DefaultModel() != "openai/gpt-5.6-sol" {
		t.Errorf("DefaultModel() = %q, want %q -- adding luna as selectable must not change the default", o.DefaultModel(), "openai/gpt-5.6-sol")
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

// TestOpencodeRunDetectsSilentAgentFallbackAsFailure reproduces opencode's
// real, verified behavior when a requested --agent name cannot run
// directly (its mode is "subagent", not "primary" or "all"): the process
// exits 0 but silently substitutes its own default agent, warning only on
// stderr. Run must not let that read as success -- see
// agentFallbackWarning's doc comment in opencode.go.
func TestOpencodeRunDetectsSilentAgentFallbackAsFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeOpencodeStub(t, "#!/bin/sh\n"+
		`echo 'agent "lucind-dag" is a subagent, not a primary agent. Falling back to default agent' 1>&2`+"\n"+
		"exit 0\n")

	o := executor.Opencode{Binary: stub}
	outcome, err := o.Run(context.Background(), executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
		Agent:        "lucind-dag",
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if outcome.ExitCode == 0 {
		t.Errorf("ExitCode = 0, want non-zero: a silent agent-fallback warning on stderr must not read as success")
	}
}

// TestOpencodeRunIgnoresFallbackWarningTextWhenNoAgentWasRequested proves
// the detection in TestOpencodeRunDetectsSilentAgentFallbackAsFailure is
// scoped to Request.Agent being set -- the same stderr text with no agent
// requested (nothing to have fallen back from) must not force a failure.
func TestOpencodeRunIgnoresFallbackWarningTextWhenNoAgentWasRequested(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	stub := writeOpencodeStub(t, "#!/bin/sh\n"+
		`echo 'agent "lucind-dag" is a subagent, not a primary agent. Falling back to default agent' 1>&2`+"\n"+
		"exit 0\n")

	o := executor.Opencode{Binary: stub}
	outcome, err := o.Run(context.Background(), executor.Request{
		Prompt:       "do the thing",
		WorktreePath: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if outcome.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0: no Agent was requested, so this text carries no meaning here", outcome.ExitCode)
	}
}
