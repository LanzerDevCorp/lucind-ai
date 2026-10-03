package executor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeInteractiveAgy emulates `agy -i`: it fires the Stop hook it finds in .agents/hooks.json
// (as the real agy does at the end of a turn), then stays open until a C-c arrives.
func fakeInteractiveAgy(t *testing.T) (binDir, ctrlCFile string) {
	t.Helper()
	binDir = t.TempDir()
	ctrlCFile = filepath.Join(binDir, "ctrlc")
	script := `#!/bin/sh
if [ -n "$FAKE_AGY_CRASH" ]; then exit "$FAKE_AGY_CRASH"; fi
if [ -n "$FAKE_AGY_DUMP_ARGS" ]; then for a in "$@"; do printf '%s\n' "$a" >> "$FAKE_AGY_DUMP_ARGS"; done; fi
if [ -z "$FAKE_AGY_NO_HOOK" ]; then
  cmd=$(sed -n 's/.*"command": "\(.*\)",$/\1/p' .agents/hooks.json)
  echo '{"fullyIdle":true,"terminationReason":"NO_TOOL_CALL"}' | sh -c "$cmd" > /dev/null
fi
while [ ! -f "$FAKE_AGY_CTRLC_FILE" ]; do sleep 0.02; done
exit 130
`
	if err := os.WriteFile(filepath.Join(binDir, "agy"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return binDir, ctrlCFile
}

// fakeHookBinary stands in for the lucind-ai binary: `hook stop --state-dir D ...` writes done.json.
func fakeHookBinary(t *testing.T, status string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "lucind-ai")
	script := `#!/bin/sh
while [ $# -gt 0 ]; do
  if [ "$1" = "--state-dir" ]; then dir="$2"; fi
  shift
done
printf '{"status":"` + status + `","terminationReason":"NO_TOOL_CALL","error":"boom"}' > "$dir/done.json"
echo '{}'
`
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

type interactiveHarness struct {
	h        HerdrAgy
	f        *fakeHerdr
	worktree string
	settings string
	repoDir  string
}

func newInteractiveHarness(t *testing.T, hookStatus string) *interactiveHarness {
	t.Helper()
	worktreeDir, repoDir, _ := setupTestDirs(t)
	// agyhooks keeps its file out of git, so the lane worktree must be a real git checkout.
	if out, err := exec.Command("git", "-C", worktreeDir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	binDir, ctrlC := fakeInteractiveAgy(t)
	t.Setenv("FAKE_AGY_CTRLC_FILE", ctrlC)
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	settings := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(settings, []byte(`{"trustedWorkspaces":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	f := &fakeHerdr{fakeAgyDir: binDir, asyncRun: true}
	f.onKeys = func(args []string) {
		if len(args) >= 4 && args[3] == "C-c" {
			_ = os.WriteFile(ctrlC, nil, 0o600)
		}
	}
	h := HerdrAgy{
		Interactive:       true,
		HookBinary:        fakeHookBinary(t, hookStatus),
		TrustSettingsPath: settings,
		Grace:             300 * time.Millisecond,
		PollInterval:      20 * time.Millisecond,
		cmd:               f.cmd,
		gitCommonDir: func(context.Context, string) (string, error) {
			return filepath.Join(repoDir, ".git"), nil
		},
	}
	return &interactiveHarness{h: h, f: f, worktree: worktreeDir, settings: settings, repoDir: repoDir}
}

// realWaitOutput makes the fake pane wait-output block until ctx ends (the sentinel is only
// produced once agy exits, which the fake does after a C-c).
func (ih *interactiveHarness) blockingWait() {
	ih.f.waitOutFunc = func(ctx context.Context, pane, regex string) ([]byte, error) {
		for {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(20 * time.Millisecond):
				// the run.sh sentinel is printed into the pane by sh; the fake cannot read panes,
				// so report it once agy has been interrupted.
				ih.f.mu.Lock()
				n := len(ih.f.sentKeysCall)
				ih.f.mu.Unlock()
				if n > 0 {
					return []byte("LUCIND_EXIT_sentinel=130\n"), nil
				}
			}
		}
	}
}

func TestHerdrAgyInteractiveFinishesOnStopHook(t *testing.T) {
	ih := newInteractiveHarness(t, "valid")
	ih.blockingWait()
	dump := filepath.Join(t.TempDir(), "args")
	t.Setenv("FAKE_AGY_DUMP_ARGS", dump)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	outcome, err := ih.h.Run(ctx, Request{Prompt: "do the thing", WorktreePath: ih.worktree, Model: "gemini-3.8-flash-high"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if outcome.ExitCode != 0 || outcome.TimedOut {
		t.Fatalf("outcome = %+v, want exit 0 and not timed out", outcome)
	}

	args, _ := os.ReadFile(dump)
	for _, want := range []string{"-i", "do the thing", "--mode", "accept-edits", "--dangerously-skip-permissions", "--model", "gemini-3.8-flash-high"} {
		if !strings.Contains(string(args), want+"\n") {
			t.Errorf("agy args missing %q:\n%s", want, args)
		}
	}
	for _, banned := range []string{"--print\n", "--output-format\n"} {
		if strings.Contains(string(args), banned) {
			t.Errorf("interactive run must not pass %q:\n%s", strings.TrimSpace(banned), args)
		}
	}
	if n := len(herdrCallsMatching(ih.f, "pane", "send-keys")); n < 2 {
		t.Errorf("send-keys calls = %d, want at least 2 (C-c twice to end the session)", n)
	}
	if _, err := os.Stat(filepath.Join(ih.worktree, ".agents", "hooks.json")); !os.IsNotExist(err) {
		t.Errorf("hooks.json must be removed after the lane (stat err = %v)", err)
	}
	trusted, _ := os.ReadFile(ih.settings)
	if strings.Contains(string(trusted), filepath.Base(ih.worktree)) {
		t.Errorf("worktree trust must be removed after the lane: %s", trusted)
	}
	entries, _ := os.ReadDir(filepath.Join(os.Getenv("XDG_STATE_HOME"), "lucind-ai", "herdr"))
	if len(entries) != 0 {
		t.Errorf("state dir must be removed on success, found %d entries", len(entries))
	}
}

func TestHerdrAgyInteractiveTrustsWorktreeWhileRunning(t *testing.T) {
	ih := newInteractiveHarness(t, "valid")
	ih.blockingWait()
	var during string
	ih.f.onKeys = func(args []string) {
		b, _ := os.ReadFile(ih.settings)
		if during == "" {
			during = string(b)
		}
		if len(args) >= 4 && args[3] == "C-c" {
			_ = os.WriteFile(os.Getenv("FAKE_AGY_CTRLC_FILE"), nil, 0o600)
		}
	}
	if _, err := ih.h.Run(context.Background(), Request{Prompt: "p", WorktreePath: ih.worktree}); err != nil {
		t.Fatal(err)
	}
	resolved, _ := filepath.EvalSymlinks(ih.worktree)
	if !strings.Contains(during, resolved) {
		t.Errorf("worktree %s was not trusted while agy ran: %s", resolved, during)
	}
}

func TestHerdrAgyInteractiveHookErrorIsAFailure(t *testing.T) {
	ih := newInteractiveHarness(t, "hook_error")
	ih.blockingWait()
	outcome, err := ih.h.Run(context.Background(), Request{Prompt: "p", WorktreePath: ih.worktree})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode == 0 || !strings.Contains(outcome.Stderr, "boom") {
		t.Fatalf("outcome = %+v, want non-zero exit carrying the hook error", outcome)
	}
}

func TestHerdrAgyInteractiveInvalidEnvelopeStillExitsZero(t *testing.T) {
	ih := newInteractiveHarness(t, "invalid")
	ih.blockingWait()
	outcome, err := ih.h.Run(context.Background(), Request{Prompt: "p", WorktreePath: ih.worktree})
	if err != nil || outcome.ExitCode != 0 {
		t.Fatalf("outcome = %+v err = %v: the dispatcher, not the executor, demotes an unreadable envelope", outcome, err)
	}
}

func TestHerdrAgyInteractiveAgyCrashBeforeStopReportsItsExitCode(t *testing.T) {
	ih := newInteractiveHarness(t, "valid")
	t.Setenv("FAKE_AGY_CRASH", "3")
	// the pane sentinel is what reveals the crash
	ih.f.waitOutFunc = func(ctx context.Context, pane, regex string) ([]byte, error) {
		time.Sleep(100 * time.Millisecond)
		return []byte("LUCIND_EXIT_sentinel=3\n"), nil
	}
	outcome, err := ih.h.Run(context.Background(), Request{Prompt: "p", WorktreePath: ih.worktree})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode != 3 {
		t.Fatalf("ExitCode = %d, want 3", outcome.ExitCode)
	}
}

func TestHerdrAgyInteractiveTimeoutInterruptsAndHardStops(t *testing.T) {
	ih := newInteractiveHarness(t, "valid")
	t.Setenv("FAKE_AGY_NO_HOOK", "1")
	t.Setenv("FAKE_AGY_CTRLC_FILE", filepath.Join(t.TempDir(), "never")) // agy ignores C-c
	ih.f.onKeys = nil
	ih.f.waitOutFunc = func(ctx context.Context, pane, regex string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	outcome, err := ih.h.Run(ctx, Request{Prompt: "p", WorktreePath: ih.worktree})
	if err != nil || !outcome.TimedOut {
		t.Fatalf("outcome = %+v err = %v, want timed out", outcome, err)
	}
	if len(ih.f.closedWS) != 1 || ih.f.closedWS[0] != "w1" {
		t.Errorf("closed workspaces = %v, want the one this executor opened", ih.f.closedWS)
	}
}

func TestHerdrAgyInteractiveRefusesToOverwriteExistingHooks(t *testing.T) {
	ih := newInteractiveHarness(t, "valid")
	if err := os.MkdirAll(filepath.Join(ih.worktree, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ih.worktree, ".agents", "hooks.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ih.h.Run(context.Background(), Request{Prompt: "p", WorktreePath: ih.worktree}); err == nil {
		t.Fatal("Run must fail instead of overwriting a repo's hooks.json")
	}
	if calls := herdrCallsMatching(ih.f, "pane", "run"); len(calls) != 0 {
		t.Errorf("nothing may run in a pane after the install failed: %v", calls)
	}
	if b, _ := os.ReadFile(filepath.Join(ih.worktree, ".agents", "hooks.json")); string(b) != "{}" {
		t.Errorf("existing hooks.json was modified: %s", b)
	}
}
