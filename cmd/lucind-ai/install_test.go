package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/agyplugin"
	"github.com/LanzerDevCorp/lucind-ai/internal/claudeplugin"
)

type fakeInstallAgy struct {
	calls [][]string
	errOn string
	err   error
}

func (f *fakeInstallAgy) Run(_ context.Context, args ...string) ([]byte, error) {
	f.calls = append(f.calls, args)
	if f.err != nil {
		if f.errOn == "" {
			return nil, f.err
		}
		if len(args) > 2 {
			if f.errOn == "lucind" && strings.HasSuffix(args[2], "lucind") {
				return nil, f.err
			}
			if f.errOn == "lucind-roles" && strings.HasSuffix(args[2], "lucind-roles") {
				return nil, f.err
			}
		}
	}
	if len(args) > 1 && args[1] == "list" {
		return []byte(`{"imports":[]}`), nil
	}
	if len(args) > 2 && strings.HasSuffix(args[2], "lucind-roles") {
		return []byte("[ok] lucind-roles\n  agents : 1 processed\n"), nil
	}
	return []byte("[ok] lucind\n  hooks : 1 processed\n"), nil
}

func TestInstall_AllThreeStepsInOrder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	fake := &fakeInstallAgy{}
	origAgy := pluginAgy
	pluginAgy = fake
	defer func() { pluginAgy = origAgy }()

	origClaude := claudeInstall
	defer func() { claudeInstall = origClaude }()
	claudeInstall = claudeplugin.Install

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"install"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d, want 0; stderr=%s", code, stderr.String())
	}

	out := stdout.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines of output, got %d:\n%s", len(lines), out)
	}

	idxClaude := strings.Index(out, "installed claude skill into ")
	idxLucind := strings.Index(out, "installed lucind via agy plugin install (staged at ")
	idxRoles := strings.Index(out, "installed lucind-roles via agy plugin install (staged at ")

	if idxClaude == -1 {
		t.Errorf("stdout missing claude skill install line: %s", out)
	}
	if idxLucind == -1 {
		t.Errorf("stdout missing lucind install line: %s", out)
	}
	if idxRoles == -1 {
		t.Errorf("stdout missing lucind-roles install line: %s", out)
	}

	if !(idxClaude < idxLucind && idxLucind < idxRoles) {
		t.Errorf("expected outputs in order (claude, lucind, lucind-roles); got indices %d, %d, %d", idxClaude, idxLucind, idxRoles)
	}

	skillFile := filepath.Join(home, ".claude", "skills", "lucind", "SKILL.md")
	if _, err := os.Stat(skillFile); err != nil {
		t.Errorf("claude skill file not found at %s: %v", skillFile, err)
	}

	root, err := agyplugin.StagingRoot()
	if err != nil {
		t.Fatal(err)
	}
	hooksFile := filepath.Join(root, "lucind", "hooks.json")
	if _, err := os.Stat(hooksFile); err != nil {
		t.Errorf("lucind hooks.json not found at %s: %v", hooksFile, err)
	}
	rolesFile := filepath.Join(root, "lucind-roles", "agents", "worker.md")
	if _, err := os.Stat(rolesFile); err != nil {
		t.Errorf("lucind-roles agent file not found at %s: %v", rolesFile, err)
	}
}

func TestInstall_FailingStepStopsSequence(t *testing.T) {
	t.Run("claude skill fails", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		fake := &fakeInstallAgy{}
		origAgy := pluginAgy
		pluginAgy = fake
		defer func() { pluginAgy = origAgy }()

		origClaude := claudeInstall
		defer func() { claudeInstall = origClaude }()
		claudeInstall = func() (string, error) {
			return "", errors.New("simulated disk full")
		}

		var stdout, stderr bytes.Buffer
		code := runInstall(context.Background(), nil, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("code=%d, want 1", code)
		}

		if !strings.Contains(stderr.String(), "lucind-ai: install claude skill: simulated disk full") {
			t.Errorf("stderr does not name the failed step: %s", stderr.String())
		}
		if strings.Contains(stdout.String(), "installed lucind via agy plugin install") {
			t.Errorf("stdout unexpectedly contains lucind install line: %s", stdout.String())
		}
		if strings.Contains(stdout.String(), "installed lucind-roles via agy plugin install") {
			t.Errorf("stdout unexpectedly contains lucind-roles install line: %s", stdout.String())
		}
		if len(fake.calls) != 0 {
			t.Errorf("expected 0 agy calls, got %v", fake.calls)
		}
	})

	t.Run("lucind plugin fails", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		fake := &fakeInstallAgy{
			errOn: "lucind",
			err:   errors.New("simulated lucind agy failure"),
		}
		origAgy := pluginAgy
		pluginAgy = fake
		defer func() { pluginAgy = origAgy }()

		origClaude := claudeInstall
		defer func() { claudeInstall = origClaude }()
		claudeInstall = claudeplugin.Install

		var stdout, stderr bytes.Buffer
		code := runInstall(context.Background(), nil, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("code=%d, want 1", code)
		}

		if !strings.Contains(stdout.String(), "installed claude skill into ") {
			t.Errorf("stdout missing claude skill success: %s", stdout.String())
		}
		if !strings.Contains(stderr.String(), "lucind-ai: install lucind:") || !strings.Contains(stderr.String(), "simulated lucind agy failure") {
			t.Errorf("stderr does not name the failed step: %s", stderr.String())
		}
		if strings.Contains(stdout.String(), "installed lucind-roles via agy plugin install") {
			t.Errorf("stdout unexpectedly contains lucind-roles install line: %s", stdout.String())
		}

		skillFile := filepath.Join(home, ".claude", "skills", "lucind", "SKILL.md")
		if _, err := os.Stat(skillFile); err != nil {
			t.Errorf("claude skill should stay installed after subsequent failure: %v", err)
		}

		for _, call := range fake.calls {
			if len(call) > 2 && strings.HasSuffix(call[2], "lucind-roles") {
				t.Errorf("agy install called for lucind-roles despite prior failure: %v", call)
			}
		}
	})

	t.Run("lucind-roles plugin fails", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		fake := &fakeInstallAgy{
			errOn: "lucind-roles",
			err:   errors.New("simulated roles agy failure"),
		}
		origAgy := pluginAgy
		pluginAgy = fake
		defer func() { pluginAgy = origAgy }()

		origClaude := claudeInstall
		defer func() { claudeInstall = origClaude }()
		claudeInstall = claudeplugin.Install

		var stdout, stderr bytes.Buffer
		code := runInstall(context.Background(), nil, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("code=%d, want 1", code)
		}

		if !strings.Contains(stdout.String(), "installed claude skill into ") {
			t.Errorf("stdout missing claude skill success: %s", stdout.String())
		}
		if !strings.Contains(stdout.String(), "installed lucind via agy plugin install (staged at ") {
			t.Errorf("stdout missing lucind success: %s", stdout.String())
		}
		if !strings.Contains(stderr.String(), "lucind-ai: install lucind-roles:") || !strings.Contains(stderr.String(), "simulated roles agy failure") {
			t.Errorf("stderr does not name the failed step: %s", stderr.String())
		}

		skillFile := filepath.Join(home, ".claude", "skills", "lucind", "SKILL.md")
		if _, err := os.Stat(skillFile); err != nil {
			t.Errorf("claude skill should stay installed: %v", err)
		}
		root, err := agyplugin.StagingRoot()
		if err != nil {
			t.Fatal(err)
		}
		hooksFile := filepath.Join(root, "lucind", "hooks.json")
		if _, err := os.Stat(hooksFile); err != nil {
			t.Errorf("lucind hooks.json should stay installed: %v", err)
		}
	})
}

func TestInstall_AgyMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	fake := &fakeInstallAgy{
		err: agyplugin.ErrAgyNotFound,
	}
	origAgy := pluginAgy
	pluginAgy = fake
	defer func() { pluginAgy = origAgy }()

	origClaude := claudeInstall
	defer func() { claudeInstall = origClaude }()
	claudeInstall = claudeplugin.Install

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"install"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code=%d, want 1", code)
	}

	if !strings.Contains(stdout.String(), "installed claude skill into ") {
		t.Errorf("stdout missing claude skill install line: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "lucind-ai: install lucind:") {
		t.Errorf("stderr does not name install lucind step: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "agy") {
		t.Errorf("stderr should mention agy when agy is missing: %s", stderr.String())
	}

	skillFile := filepath.Join(home, ".claude", "skills", "lucind", "SKILL.md")
	if _, err := os.Stat(skillFile); err != nil {
		t.Errorf("claude skill should be installed even when agy is missing: %v", err)
	}

	for _, call := range fake.calls {
		if len(call) > 2 && strings.HasSuffix(call[2], "lucind-roles") {
			t.Errorf("agy install called for lucind-roles despite prior failure: %v", call)
		}
	}
}

func TestInstall_FlagsAndArguments(t *testing.T) {
	invalidCases := [][]string{
		{"--unexpected"},
		{"arg1"},
		{"--dir", "/tmp"},
		{"--help", "extra"},
		{"-h", "extra"},
	}

	for _, args := range invalidCases {
		t.Run("invalid_"+strings.Join(args, "_"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runInstall(context.Background(), args, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("code=%d, want 1 for args %v", code, args)
			}
			if !strings.Contains(stderr.String(), "usage: lucind-ai install\n") {
				t.Errorf("stderr missing usage line for args %v: %s", args, stderr.String())
			}

			// Also verify through CLI run dispatch
			stdout.Reset()
			stderr.Reset()
			cliArgs := append([]string{"install"}, args...)
			code = run(context.Background(), cliArgs, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("cli run code=%d, want 1 for args %v", code, cliArgs)
			}
			if !strings.Contains(stderr.String(), "usage: lucind-ai install\n") {
				t.Errorf("cli run stderr missing usage line for args %v: %s", cliArgs, stderr.String())
			}
		})
	}

	helpCases := [][]string{
		{"--help"},
		{"-help"},
		{"-h"},
		{"help"},
	}

	for _, args := range helpCases {
		t.Run("help_"+strings.Join(args, "_"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runInstall(context.Background(), args, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("code=%d, want 0 for args %v", code, args)
			}
			if !strings.Contains(stdout.String(), "usage: lucind-ai install\n") {
				t.Errorf("stdout missing usage line for args %v: %s", args, stdout.String())
			}

			// Also verify through CLI run dispatch
			stdout.Reset()
			stderr.Reset()
			cliArgs := append([]string{"install"}, args...)
			code = run(context.Background(), cliArgs, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("cli run code=%d, want 0 for args %v", code, cliArgs)
			}
			if !strings.Contains(stdout.String(), "usage: lucind-ai install\n") {
				t.Errorf("cli run stdout missing usage line for args %v: %s", cliArgs, stdout.String())
			}
		})
	}
}
