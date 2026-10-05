package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/claudeplugin"
	"github.com/LanzerDevCorp/lucind-ai/internal/dispatch"
	"github.com/LanzerDevCorp/lucind-ai/internal/skillselect"
	"github.com/LanzerDevCorp/lucind-ai/internal/userconfig"
)

type resetEnv struct {
	home        string
	variant     claudeplugin.Variant
	installed   int
	prompted    int
	answer      string
	promptError error
}

// setupReset isolates the config directory and stubs every side effect of install: a stored key
// ("old-stored-key") exists, stdin is a terminal and the prompt returns env.answer.
func setupReset(t *testing.T, answer string) *resetEnv {
	t.Helper()
	env := &resetEnv{home: t.TempDir(), answer: answer}
	t.Setenv("HOME", env.home)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("TYPESAFE_API_KEY", "")

	if err := userconfig.WriteKey("TYPESAFE_API_KEY", "old-stored-key"); err != nil {
		t.Fatalf("seed stored key: %v", err)
	}

	origAgy, origInstall, origTerm, origRead := pluginAgy, claudeInstall, isStdinTerminal, readSecretKey
	pluginAgy = &fakeInstallAgy{}
	claudeInstall = func(v claudeplugin.Variant) (string, error) {
		env.installed++
		env.variant = v
		return filepath.Join(env.home, ".claude", "skills", "lucind"), nil
	}
	isStdinTerminal = func() bool { return true }
	readSecretKey = func(io.Writer) (string, error) {
		env.prompted++
		return env.answer, env.promptError
	}
	t.Cleanup(func() {
		pluginAgy, claudeInstall, isStdinTerminal, readSecretKey = origAgy, origInstall, origTerm, origRead
	})
	return env
}

func storedKey(t *testing.T) string {
	t.Helper()
	key, err := userconfig.ReadKey("TYPESAFE_API_KEY")
	if err != nil {
		t.Fatalf("read stored key: %v", err)
	}
	return key
}

func TestInstall_ResetKey_ReplacesTheStoredKey(t *testing.T) {
	env := setupReset(t, "brand-new-key")

	var stdout, stderr bytes.Buffer
	code := runInstall(context.Background(), []string{"--reset-key", "--no-claude-md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if env.prompted != 1 {
		t.Errorf("prompt calls = %d, want 1", env.prompted)
	}
	if got := storedKey(t); got != "brand-new-key" {
		t.Errorf("stored key = %q, want the new one", got)
	}
	if env.variant != claudeplugin.VariantAuto {
		t.Errorf("variant = %q, want auto", env.variant)
	}
	if !strings.Contains(stdout.String(), "stored the new TYPESAFE_API_KEY in ") {
		t.Errorf("stdout lacks the stored confirmation: %q", stdout.String())
	}
	for _, secret := range []string{"brand-new-key", "old-stored-key"} {
		if strings.Contains(stdout.String()+stderr.String(), secret) {
			t.Errorf("a key value leaked into the output: %q", secret)
		}
	}
}

func TestInstall_ResetKey_WithoutTerminalFailsBeforeInstalling(t *testing.T) {
	env := setupReset(t, "brand-new-key")
	isStdinTerminal = func() bool { return false }

	var stdout, stderr bytes.Buffer
	code := runInstall(context.Background(), []string{"--reset-key"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "--reset-key needs a terminal") {
		t.Errorf("stderr lacks the terminal requirement: %q", stderr.String())
	}
	if env.installed != 0 || env.prompted != 0 {
		t.Errorf("nothing may run: installed=%d prompted=%d", env.installed, env.prompted)
	}
	if got := storedKey(t); got != "old-stored-key" {
		t.Errorf("stored key changed to %q", got)
	}
}

func TestInstall_ResetKey_EmptyAnswerKeepsTheCurrentKey(t *testing.T) {
	env := setupReset(t, "")

	var stdout, stderr bytes.Buffer
	code := runInstall(context.Background(), []string{"--reset-key", "--no-claude-md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if got := storedKey(t); got != "old-stored-key" {
		t.Errorf("stored key = %q, want it untouched", got)
	}
	if env.variant != claudeplugin.VariantAuto {
		t.Errorf("variant = %q, want auto (the old key still resolves)", env.variant)
	}
	if strings.Contains(stdout.String(), "stored the new") {
		t.Errorf("must not claim a new key was stored: %q", stdout.String())
	}
}

func TestInstall_ResetKey_WarnsWhenTheEnvironmentOverridesTheFile(t *testing.T) {
	setupReset(t, "brand-new-key")
	t.Setenv("TYPESAFE_API_KEY", "key-from-the-environment")

	var stdout, stderr bytes.Buffer
	code := runInstall(context.Background(), []string{"--reset-key", "--no-claude-md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if got := storedKey(t); got != "brand-new-key" {
		t.Errorf("stored key = %q, want the new one", got)
	}
	if !strings.Contains(stderr.String(), "environment variable") || !strings.Contains(stderr.String(), "overrides") {
		t.Errorf("stderr must warn that the environment variable overrides the file: %q", stderr.String())
	}
	if strings.Contains(stdout.String()+stderr.String(), "key-from-the-environment") {
		t.Errorf("the environment key leaked into the output")
	}
}

func TestInstall_PlainInstallNeverReplacesAnExistingKey(t *testing.T) {
	env := setupReset(t, "brand-new-key")

	var stdout, stderr bytes.Buffer
	code := runInstall(context.Background(), []string{"--no-claude-md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if env.prompted != 0 {
		t.Errorf("prompt calls = %d, want 0 without --reset-key", env.prompted)
	}
	if got := storedKey(t); got != "old-stored-key" {
		t.Errorf("stored key = %q, want it untouched", got)
	}
}

func TestInstall_FlagsCombineInAnyOrderAndUnknownOnesFail(t *testing.T) {
	if !strings.Contains(installUsage, "--reset-key") {
		t.Errorf("installUsage lacks --reset-key: %q", installUsage)
	}
	for _, args := range [][]string{{"--reset-key", "--no-claude-md"}, {"--no-claude-md", "--reset-key"}} {
		setupReset(t, "brand-new-key")
		var stdout, stderr bytes.Buffer
		if code := runInstall(context.Background(), args, &stdout, &stderr); code != 0 {
			t.Errorf("args %v: exit code = %d, want 0; stderr: %s", args, code, stderr.String())
		}
	}
	setupReset(t, "brand-new-key")
	var stdout, stderr bytes.Buffer
	if code := runInstall(context.Background(), []string{"--reset-key", "--bogus"}, &stdout, &stderr); code != 1 {
		t.Errorf("unknown flag: exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Errorf("unknown flag must print the usage: %q", stderr.String())
	}
}

func TestDispatch_AutoSkillsUnavailable_RejectedKeyHint(t *testing.T) {
	tests := []struct {
		name         string
		cause        error
		wantRejected bool
	}{
		{"401 points to reset-key", &skillselect.HTTPError{StatusCode: 401, Body: "authentication_error"}, true},
		{"server error does not blame the key", &skillselect.HTTPError{StatusCode: 500}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoDir := initRepo(t)
			promptFile := filepath.Join(repoDir, "task_prompt.md")
			if err := os.WriteFile(promptFile, []byte("Implement feature X"), 0o644); err != nil {
				t.Fatal(err)
			}
			orig := dispatchRun
			defer func() { dispatchRun = orig }()
			dispatchRun = func(context.Context, dispatch.Options, dispatch.HerdrRunner) (dispatch.Output, int, error) {
				return dispatch.Output{}, 5, &dispatch.AutoSkillsUnavailableError{Cause: tt.cause}
			}

			var stdout, stderr bytes.Buffer
			code := run(context.Background(), []string{"dispatch", "--cwd", repoDir, "--allow", "src/**", "--prompt", promptFile, "--auto-skills"}, &stdout, &stderr)
			if code != 5 {
				t.Fatalf("exit code = %d, want 5", code)
			}
			out := stderr.String()
			if got := strings.Contains(out, "--reset-key"); got != tt.wantRejected {
				t.Errorf("mentions --reset-key = %v, want %v; stderr: %s", got, tt.wantRejected, out)
			}
			if strings.Contains(out, "to store the key run lucind-ai install") {
				t.Errorf("the missing-key hint must not appear for %s: %s", fmt.Sprint(tt.cause), out)
			}
		})
	}
}
