package agyhooks_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/agyhooks"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	runCmd(t, dir, "git", "init")
	runCmd(t, dir, "git", "config", "user.name", "Test User")
	runCmd(t, dir, "git", "config", "user.email", "test@example.com")
	runCmd(t, dir, "git", "commit", "--allow-empty", "-m", "init")
}

func runCmd(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s in %s failed: %v\nOutput: %s", name, strings.Join(args, " "), dir, err, string(out))
	}
	return string(out)
}

func splitPOSIX(t *testing.T, command string) []string {
	t.Helper()
	cmd := exec.Command("sh", "-c", `eval "set -- $1"; for a in "$@"; do printf "%s\0" "$a"; done`, "sh", command)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("POSIX shell split failed for command %q: %v", command, err)
	}
	if len(out) == 0 {
		return nil
	}
	parts := bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0})
	var result []string
	for _, p := range parts {
		result = append(result, string(p))
	}
	return result
}

func TestInstall_SchemaAndShellSplit(t *testing.T) {
	ctx := context.Background()
	worktree := filepath.Join(t.TempDir(), "work dir 'with quote'")
	initGitRepo(t, worktree)

	binary := filepath.Join(worktree, "bin 'test'", "lucind-ai")
	stateDir := filepath.Join(worktree, "state dir 'test'")
	resultPath := filepath.Join(worktree, "results 'test'", "result.json")

	opts := agyhooks.Options{
		Binary:       binary,
		StateDir:     stateDir,
		ResultPath:   resultPath,
		MaxContinues: 2,
	}

	if err := agyhooks.Install(ctx, worktree, opts); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	hooksPath := filepath.Join(worktree, ".agents", "hooks.json")
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("read hooks.json: %v", err)
	}

	var parsed map[string]struct {
		Stop []struct {
			Type    string `json:"type"`
			Command string `json:"command"`
			Timeout int    `json:"timeout"`
		} `json:"Stop"`
	}

	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal hooks.json: %v", err)
	}

	if len(parsed) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(parsed))
	}

	hook, ok := parsed["lucind-lane"]
	if !ok {
		t.Fatalf("expected hook named 'lucind-lane', got keys: %v", parsed)
	}

	if len(hook.Stop) != 1 {
		t.Fatalf("expected 1 Stop handler, got %d", len(hook.Stop))
	}

	handler := hook.Stop[0]
	if handler.Type != "command" {
		t.Errorf("handler.Type = %q, want %q", handler.Type, "command")
	}
	if handler.Timeout != 30 {
		t.Errorf("handler.Timeout = %d, want 30", handler.Timeout)
	}

	tokens := splitPOSIX(t, handler.Command)
	expectedTokens := []string{
		binary,
		"hook",
		"stop",
		"--state-dir",
		stateDir,
		"--result",
		resultPath,
		"--max-continues",
		"2",
	}

	if len(tokens) != len(expectedTokens) {
		t.Fatalf("split tokens count = %d, want %d\nGot: %v\nWant: %v", len(tokens), len(expectedTokens), tokens, expectedTokens)
	}

	for i := range expectedTokens {
		if tokens[i] != expectedTokens[i] {
			t.Errorf("token[%d] = %q, want %q", i, tokens[i], expectedTokens[i])
		}
	}
}

func TestInstall_DefaultMaxContinues(t *testing.T) {
	ctx := context.Background()
	worktree := filepath.Join(t.TempDir(), "worktree")
	initGitRepo(t, worktree)

	opts := agyhooks.Options{
		Binary:       filepath.Join(worktree, "lucind-ai"),
		StateDir:     filepath.Join(worktree, "state"),
		ResultPath:   filepath.Join(worktree, "result.json"),
		MaxContinues: 0, // 0 => 2
	}

	if err := agyhooks.Install(ctx, worktree, opts); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	hooksPath := filepath.Join(worktree, ".agents", "hooks.json")
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("read hooks.json: %v", err)
	}

	var parsed map[string]struct {
		Stop []struct {
			Command string `json:"command"`
		} `json:"Stop"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	tokens := splitPOSIX(t, parsed["lucind-lane"].Stop[0].Command)
	if tokens[len(tokens)-1] != "2" {
		t.Fatalf("expected default max-continues 2, got %q", tokens[len(tokens)-1])
	}
}

func TestInstall_RefusesExistingHooksJSON(t *testing.T) {
	ctx := context.Background()
	worktree := filepath.Join(t.TempDir(), "worktree")
	initGitRepo(t, worktree)

	agentsDir := filepath.Join(worktree, ".agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	hooksPath := filepath.Join(agentsDir, "hooks.json")
	existingContent := []byte(`{"custom":{}}`)
	if err := os.WriteFile(hooksPath, existingContent, 0644); err != nil {
		t.Fatal(err)
	}

	opts := agyhooks.Options{
		Binary:       filepath.Join(worktree, "lucind-ai"),
		StateDir:     filepath.Join(worktree, "state"),
		ResultPath:   filepath.Join(worktree, "result.json"),
		MaxContinues: 2,
	}

	err := agyhooks.Install(ctx, worktree, opts)
	if err == nil {
		t.Fatalf("expected error when hooks.json already exists, got nil")
	}

	// Verify original file was preserved
	content, _ := os.ReadFile(hooksPath)
	if string(content) != string(existingContent) {
		t.Fatalf("existing hooks.json was overwritten")
	}
}

func TestInstall_RefusesRelativePaths(t *testing.T) {
	ctx := context.Background()
	worktree := filepath.Join(t.TempDir(), "worktree")
	initGitRepo(t, worktree)

	validAbs := filepath.Join(worktree, "file")

	tests := []struct {
		name string
		opts agyhooks.Options
	}{
		{
			name: "relative binary",
			opts: agyhooks.Options{
				Binary:     "bin/lucind-ai",
				StateDir:   validAbs,
				ResultPath: validAbs,
			},
		},
		{
			name: "relative state dir",
			opts: agyhooks.Options{
				Binary:     validAbs,
				StateDir:   "state",
				ResultPath: validAbs,
			},
		},
		{
			name: "relative result path",
			opts: agyhooks.Options{
				Binary:     validAbs,
				StateDir:   validAbs,
				ResultPath: "result.json",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := agyhooks.Install(ctx, worktree, tc.opts)
			if err == nil {
				t.Fatalf("expected error for relative path in %s, got nil", tc.name)
			}
		})
	}
}

func TestInstall_GitExcludeAndStatus(t *testing.T) {
	ctx := context.Background()
	repoDir := filepath.Join(t.TempDir(), "repo")
	initGitRepo(t, repoDir)

	opts := agyhooks.Options{
		Binary:       filepath.Join(repoDir, "lucind-ai"),
		StateDir:     filepath.Join(repoDir, "state"),
		ResultPath:   filepath.Join(repoDir, "result.json"),
		MaxContinues: 2,
	}

	if err := agyhooks.Install(ctx, repoDir, opts); err != nil {
		t.Fatalf("first Install() failed: %v", err)
	}

	// Verify git status does not show .agents/hooks.json
	statusOut := runCmd(t, repoDir, "git", "status", "--porcelain")
	if strings.Contains(statusOut, ".agents/hooks.json") {
		t.Fatalf("git status --porcelain shows .agents/hooks.json: %s", statusOut)
	}

	// Remove hooks.json and install again to test exclude deduplication
	hooksFile := filepath.Join(repoDir, ".agents", "hooks.json")
	if err := os.Remove(hooksFile); err != nil {
		t.Fatal(err)
	}

	if err := agyhooks.Install(ctx, repoDir, opts); err != nil {
		t.Fatalf("second Install() failed: %v", err)
	}

	// Verify info/exclude has /.agents/hooks.json exactly once
	excludePathOut := runCmd(t, repoDir, "git", "rev-parse", "--git-path", "info/exclude")
	excludePath := strings.TrimSpace(excludePathOut)
	if !filepath.IsAbs(excludePath) {
		excludePath = filepath.Join(repoDir, excludePath)
	}

	excludeData, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatalf("read exclude file: %v", err)
	}

	lines := strings.Split(string(excludeData), "\n")
	count := 0
	for _, l := range lines {
		if strings.TrimSpace(l) == "/.agents/hooks.json" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected line /.agents/hooks.json exactly once in %s, found %d times\nContent:\n%s", excludePath, count, string(excludeData))
	}
}

func TestDoneHelpers(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")

	// 1. ReadDone on non-existent file
	done, exists, err := agyhooks.ReadDone(stateDir)
	if err != nil {
		t.Fatalf("ReadDone error on missing file: %v", err)
	}
	if exists {
		t.Fatalf("ReadDone reported file exists when it does not")
	}

	// 2. DonePath
	expectedPath := filepath.Join(stateDir, "done.json")
	if p := agyhooks.DonePath(stateDir); p != expectedPath {
		t.Fatalf("DonePath = %q, want %q", p, expectedPath)
	}

	// 3. WriteDone & ReadDone
	orig := agyhooks.Done{
		Status:            "valid",
		TerminationReason: "NO_TOOL_CALL",
		At:                "2026-10-03T18:00:00Z",
	}
	if err := agyhooks.WriteDone(stateDir, orig); err != nil {
		t.Fatalf("WriteDone failed: %v", err)
	}

	done, exists, err = agyhooks.ReadDone(stateDir)
	if err != nil {
		t.Fatalf("ReadDone failed: %v", err)
	}
	if !exists {
		t.Fatalf("ReadDone reported file does not exist")
	}
	if done.Status != orig.Status || done.TerminationReason != orig.TerminationReason || done.At != orig.At {
		t.Fatalf("ReadDone got %+v, want %+v", done, orig)
	}
}

func gitWorktree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	return dir
}

func TestInstallRulesWritesExcludedFilesAndRefusesOverwrite(t *testing.T) {
	wt := gitWorktree(t)
	files := map[string][]byte{"lucind-a.md": []byte("---\ntrigger: always_on\n---\nA\n"), "lucind-b.md": []byte("---\ntrigger: always_on\n---\nB\n")}
	if err := agyhooks.InstallRules(context.Background(), wt, files); err != nil {
		t.Fatalf("InstallRules: %v", err)
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join(wt, ".agents", "rules", name))
		if err != nil || string(got) != string(want) {
			t.Fatalf("%s = %q, %v", name, got, err)
		}
	}
	if out, _ := exec.Command("git", "-C", wt, "status", "--porcelain").Output(); strings.Contains(string(out), ".agents") {
		t.Errorf("rule files must be excluded from git status, got:\n%s", out)
	}

	agyhooks.RemoveRules(wt, files)
	if _, err := os.Stat(filepath.Join(wt, ".agents")); !os.IsNotExist(err) {
		t.Errorf(".agents must be removed when it ends up empty (err = %v)", err)
	}

	wt2 := gitWorktree(t)
	if err := os.MkdirAll(filepath.Join(wt2, ".agents", "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	mine := filepath.Join(wt2, ".agents", "rules", "lucind-a.md")
	if err := os.WriteFile(mine, []byte("user rule"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := agyhooks.InstallRules(context.Background(), wt2, files); err == nil {
		t.Fatal("InstallRules must refuse to overwrite an existing rule file")
	}
	if b, _ := os.ReadFile(mine); string(b) != "user rule" {
		t.Errorf("existing rule file was modified: %s", b)
	}
	if _, err := os.Stat(filepath.Join(wt2, ".agents", "rules", "lucind-b.md")); !os.IsNotExist(err) {
		t.Errorf("a refused install must not leave partial files (err = %v)", err)
	}
}

func TestInstallRulesRejectsUnsafeNames(t *testing.T) {
	wt := gitWorktree(t)
	for _, name := range []string{"../escape.md", "sub/dir.md", "other.md", "lucind-x.txt"} {
		if err := agyhooks.InstallRules(context.Background(), wt, map[string][]byte{name: []byte("x")}); err == nil {
			t.Errorf("name %q must be rejected", name)
		}
	}
}
