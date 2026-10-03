package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func createTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v: %s", dir, err, string(out))
	}
	return dir
}

func TestCLIRulesInitAndGenerate(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	repo := createTempGitRepo(t)
	ctx := context.Background()

	// 1. rules init
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"rules", "init", "--root", repo}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("rules init exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	rulesFile := filepath.Join(repo, "lucind-rules.md")
	if _, err := os.Stat(rulesFile); err != nil {
		t.Fatalf("lucind-rules.md not created: %v", err)
	}

	// 2. rules generate
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"rules", "generate", "--root", repo}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("rules generate exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "CLAUDE.md: written") ||
		!strings.Contains(out, "GEMINI.md: written") ||
		!strings.Contains(out, "AGENTS.md: written") {
		t.Errorf("stdout missing expected 'written' outputs:\n%s", out)
	}

	// 3. second generate => unchanged
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"rules", "generate", "--root", repo}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("second rules generate exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	out = stdout.String()
	if !strings.Contains(out, "CLAUDE.md: unchanged") ||
		!strings.Contains(out, "GEMINI.md: unchanged") ||
		!strings.Contains(out, "AGENTS.md: unchanged") {
		t.Errorf("stdout missing expected 'unchanged' outputs:\n%s", out)
	}

	// 4. check mode when up to date => exit 0
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"rules", "generate", "--root", repo, "--check"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("rules generate --check exit code = %d, want 0; stderr: %s", code, stderr.String())
	}

	// 5. edit source => check mode reports stale => exit 1
	if err := os.WriteFile(rulesFile, append([]byte("<!-- lucind:rules audience=all -->\n# Updated\n"), []byte("Extra content\n")...), 0644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"rules", "generate", "--root", repo, "--check"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("rules generate --check on stale files exit code = %d, want 1", code)
	}
	out = stdout.String()
	if !strings.Contains(out, "stale") {
		t.Errorf("stdout missing 'stale':\n%s", out)
	}
}

func TestCLIRulesHandWrittenSkipped(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	repo := createTempGitRepo(t)
	ctx := context.Background()

	// Init source
	var stdout, stderr bytes.Buffer
	if code := run(ctx, []string{"rules", "init", "--root", repo}, &stdout, &stderr); code != 0 {
		t.Fatalf("rules init failed: %s", stderr.String())
	}

	// Create hand-written CLAUDE.md
	claudePath := filepath.Join(repo, "CLAUDE.md")
	handWritten := "# Custom hand-written rules\n"
	if err := os.WriteFile(claudePath, []byte(handWritten), 0644); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	code := run(ctx, []string{"rules", "generate", "--root", repo}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("rules generate exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "CLAUDE.md: skipped_hand_written") {
		t.Errorf("stdout missing skipped_hand_written for CLAUDE.md:\n%s", out)
	}
	if !strings.Contains(out, "GEMINI.md: written") || !strings.Contains(out, "AGENTS.md: written") {
		t.Errorf("stdout missing written for GEMINI.md or AGENTS.md:\n%s", out)
	}

	// Check CLAUDE.md was untouched
	content, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != handWritten {
		t.Errorf("hand-written CLAUDE.md was overwritten!")
	}
}

func TestCLIRulesBadFlags(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"rules", "init", "--bad-flag"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("rules init bad flag exit code = %d, want 1", code)
	}

	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"rules", "generate", "--bad-flag"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("rules generate bad flag exit code = %d, want 1", code)
	}

	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"rules", "unknown-command"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("rules unknown-command exit code = %d, want 1", code)
	}
}

func TestCLIRulesOutsideRepoWithoutRoot(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	nonRepoDir := t.TempDir()
	ctx := context.Background()

	// Switch working directory to non-git directory for this test
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(nonRepoDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(origWd)
	}()

	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"rules", "init"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("rules init outside repo without --root exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "specify --root") && !strings.Contains(stderr.String(), "not in a git repository") {
		t.Errorf("expected stderr to mention git repository / specify --root, got: %s", stderr.String())
	}
}
