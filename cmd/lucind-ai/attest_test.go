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

func initTestGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Tester",
			"GIT_AUTHOR_EMAIL=tester@example.com",
			"GIT_COMMITTER_NAME=Tester",
			"GIT_COMMITTER_EMAIL=tester@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v\nOutput: %s", strings.Join(args, " "), err, string(out))
		}
	}
	run("init")
	if err := os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write code.go: %v", err)
	}
	run("add", "code.go")
	run("commit", "-m", "init commit")
}

func TestCLIAttestRunAndVerify(t *testing.T) {
	repoDir := t.TempDir()
	initTestGitRepo(t, repoDir)

	configDir := t.TempDir()
	stateDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)
	t.Setenv("XDG_STATE_HOME", stateDir)

	// Change working directory to repoDir for testing
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir repoDir: %v", err)
	}
	defer func() {
		_ = os.Chdir(origWd)
	}()

	ctx := context.Background()

	// 1. Verify before run -> "no entry"
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"attest", "verify", "--command", "echo pass"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected verify without entry to exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "no entry") {
		t.Fatalf("expected stderr to contain %q, got %q", "no entry", stderr.String())
	}

	// 2. Run passing command: echo pass
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"attest", "run", "--", "echo", "pass"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected attest run to exit 0, got %d. stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "pass") {
		t.Fatalf("expected stdout to contain command output 'pass', got %q", stdout.String())
	}

	// Key file should have been created with mode 0600
	keyPath := filepath.Join(configDir, "lucind-ai", "attest.key")
	keyInfo, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}
	if keyInfo.Mode().Perm() != 0600 {
		t.Fatalf("expected key file mode 0600, got %04o", keyInfo.Mode().Perm())
	}

	// 3. Verify passing command -> exit 0
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"attest", "verify", "--command", "echo pass"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected verify to exit 0, got %d. stderr: %s", code, stderr.String())
	}

	// 4. Edit repository -> verify fails with "tree changed"
	modFile := filepath.Join(repoDir, "newfile.txt")
	if err := os.WriteFile(modFile, []byte("untracked file\n"), 0644); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"attest", "verify", "--command", "echo pass"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected verify after edit to exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "tree changed") {
		t.Fatalf("expected stderr to contain %q, got %q", "tree changed", stderr.String())
	}

	// Remove untracked file -> verify passes again
	if err := os.Remove(modFile); err != nil {
		t.Fatalf("remove modFile: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"attest", "verify", "--command", "echo pass"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected verify after revert to exit 0, got %d. stderr: %s", code, stderr.String())
	}

	// 4b. Deletion of tracked file -> verify fails with "tree changed"
	trackedFile := filepath.Join(repoDir, "code.go")
	origContent, err := os.ReadFile(trackedFile)
	if err != nil {
		t.Fatalf("read tracked file: %v", err)
	}
	if err := os.Remove(trackedFile); err != nil {
		t.Fatalf("remove tracked file: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"attest", "verify", "--command", "echo pass"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected verify after file deletion to exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "tree changed") {
		t.Fatalf("expected stderr to contain %q, got %q", "tree changed", stderr.String())
	}
	// Restore tracked file -> verify passes again
	if err := os.WriteFile(trackedFile, origContent, 0644); err != nil {
		t.Fatalf("restore tracked file: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"attest", "verify", "--command", "echo pass"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected verify after restoring file to exit 0, got %d. stderr: %s", code, stderr.String())
	}

	// 5. Run failing command: sh -c "exit 42"
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"attest", "run", "--", "sh", "-c", "exit 42"}, &stdout, &stderr)
	if code != 42 {
		t.Fatalf("expected attest run to exit with command code 42, got %d", code)
	}

	// Verify failing command -> "tests failed"
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"attest", "verify", "--command", "sh -c exit 42"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected verify for failed command to exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "tests failed") {
		t.Fatalf("expected stderr to contain %q, got %q", "tests failed", stderr.String())
	}

	// 6. Bad MAC: find entry file and mutate MAC
	// Entries are under stateDir/lucind-ai/attestations/<repo_id>/*.json
	pattern := filepath.Join(stateDir, "lucind-ai", "attestations", "*", "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil || len(files) == 0 {
		t.Fatalf("glob entries failed: files=%v, err=%v", files, err)
	}
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read entry %s: %v", f, err)
		}
		if strings.Contains(string(content), "echo pass") {
			// Tamper MAC
			tampered := strings.Replace(string(content), `"mac": "`, `"mac": "badbeef`, 1)
			_ = os.Chmod(f, 0644)
			if err := os.WriteFile(f, []byte(tampered), 0644); err != nil {
				t.Fatalf("write tampered entry: %v", err)
			}
			_ = os.Chmod(f, 0444)
			break
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"attest", "verify", "--command", "echo pass"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected verify with tampered entry to exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "bad mac") {
		t.Fatalf("expected stderr to contain %q, got %q", "bad mac", stderr.String())
	}

	// 7. Check no secrets are printed or logged
	rawKey, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	if bytes.Contains(stdout.Bytes(), rawKey) || bytes.Contains(stderr.Bytes(), rawKey) {
		t.Fatalf("secret key leaked in stdout or stderr")
	}

	// 8. Check that real .git/index was never modified by attest run or verify
	cmdStatus := exec.Command("git", "status", "--porcelain")
	statusOut, err := cmdStatus.Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	if string(statusOut) != "" {
		t.Fatalf("expected git working tree/index to remain clean, got:\n%s", string(statusOut))
	}
}

func TestCLIAttestUsageAndErrors(t *testing.T) {
	ctx := context.Background()
	var stdout, stderr bytes.Buffer

	// Missing subcommand
	code := run(ctx, []string{"attest"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "usage: lucind-ai attest") {
		t.Fatalf("expected usage message, got %q", stderr.String())
	}

	// Unknown subcommand
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"attest", "unknown"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown subcommand") {
		t.Fatalf("expected unknown subcommand error, got %q", stderr.String())
	}

	// attest run without command
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"attest", "run"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}

	// attest verify without --command
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"attest", "verify"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
}

func TestCLIAttestWorktreeSharing(t *testing.T) {
	repoDir := t.TempDir()
	initTestGitRepo(t, repoDir)

	worktreeDir := filepath.Join(t.TempDir(), "lane-wt")
	cmd := exec.Command("git", "-C", repoDir, "worktree", "add", worktreeDir, "HEAD")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add failed: %v: %s", err, string(out))
	}

	configDir := t.TempDir()
	stateDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)
	t.Setenv("XDG_STATE_HOME", stateDir)

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(origWd)
	}()

	ctx := context.Background()

	// 1. Run attest in the worktree
	if err := os.Chdir(worktreeDir); err != nil {
		t.Fatalf("chdir worktreeDir: %v", err)
	}
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"attest", "run", "--", "echo", "worktree-ok"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("attest run in worktree failed: exit %d, stderr: %s", code, stderr.String())
	}

	// 2. Verify in the primary repo (shares same RepoID via RepoCommonDir)
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir repoDir: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"attest", "verify", "--command", "echo worktree-ok"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected verify in primary repo to succeed for worktree attestation, got %d, stderr: %s", code, stderr.String())
	}
}
