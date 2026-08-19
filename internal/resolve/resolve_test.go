package resolve_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/resolve"
)

// initConflictedRepo creates a throwaway git repo with two branches that conflict on conflictFiles.
// It leaves the repo in a conflicted merge state.
func initConflictedRepo(t *testing.T, conflictFiles map[string][2]string) string {
	t.Helper()

	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "resolve-test@example.com")
	runGit(t, root, "config", "user.name", "resolve-test")

	// Base commit with all initial files
	for path := range conflictFiles {
		fullPath := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("MkdirAll error = %v", err)
		}
		if err := os.WriteFile(fullPath, []byte("base line\n"), 0o644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}
		runGit(t, root, "add", path)
	}
	runGit(t, root, "commit", "-m", "base commit")

	// Branch A
	runGit(t, root, "checkout", "-b", "branch-a")
	for path, contents := range conflictFiles {
		fullPath := filepath.Join(root, path)
		if err := os.WriteFile(fullPath, []byte(contents[0]), 0o644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}
		runGit(t, root, "add", path)
	}
	runGit(t, root, "commit", "-m", "branch a commit")

	// Branch B off main
	runGit(t, root, "checkout", "main")
	runGit(t, root, "checkout", "-b", "branch-b")
	for path, contents := range conflictFiles {
		fullPath := filepath.Join(root, path)
		if err := os.WriteFile(fullPath, []byte(contents[1]), 0o644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}
		runGit(t, root, "add", path)
	}
	runGit(t, root, "commit", "-m", "branch b commit")

	// Attempt merge branch-a into branch-b to create conflict
	cmd := exec.Command("git", "merge", "--no-ff", "branch-a")
	cmd.Dir = root
	_ = cmd.Run()

	return root
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", append([]string{
		"-c", "user.email=resolve-test@example.com",
		"-c", "user.name=resolve-test",
	}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v error = %v, output = %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestResolveSizeBoundExceeded(t *testing.T) {
	// Create lines exceeding 400 lines in conflict
	var linesA, linesB strings.Builder
	for i := 0; i < 210; i++ {
		linesA.WriteString(fmt.Sprintf("line a %d\n", i))
		linesB.WriteString(fmt.Sprintf("line b %d\n", i))
	}

	repo := initConflictedRepo(t, map[string][2]string{
		"large_conflict.txt": {linesA.String(), linesB.String()},
	})

	invokerCalled := false
	fakeInvoker := func(ctx context.Context, worktreePath, prompt string) (string, error) {
		invokerCalled = true
		panic("fake invoker must not be called when conflict exceeds MaxConflictLines")
	}

	resolved, out, err := resolve.Resolve(context.Background(), repo, fakeInvoker)
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if resolved {
		t.Fatalf("Resolve() resolved = true, want false")
	}
	if invokerCalled {
		t.Errorf("fake invoker was called, want not called")
	}
	if !strings.Contains(out, "conflict exceeds 400-line bound") {
		t.Errorf("Resolve() output = %q, want bound message", out)
	}
}

func TestResolveHappyPath(t *testing.T) {
	repo := initConflictedRepo(t, map[string][2]string{
		"file1.txt": {"branch a file 1\n", "branch b file 1\n"},
		"file2.txt": {"branch a file 2\n", "branch b file 2\n"},
	})

	fakeInvoker := func(ctx context.Context, worktreePath, prompt string) (string, error) {
		if err := os.WriteFile(filepath.Join(worktreePath, "file1.txt"), []byte("resolved file 1\n"), 0o644); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(worktreePath, "file2.txt"), []byte("resolved file 2\n"), 0o644); err != nil {
			return "", err
		}
		return "resolved both files", nil
	}

	resolved, out, err := resolve.Resolve(context.Background(), repo, fakeInvoker)
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if !resolved {
		t.Fatalf("Resolve() resolved = false, want true; output = %s", out)
	}
	if out != "resolved both files" {
		t.Errorf("Resolve() output = %q, want %q", out, "resolved both files")
	}

	// Assert git status is clean
	status := runGit(t, repo, "status", "--porcelain")
	if status != "" {
		t.Errorf("git status --porcelain = %q, want empty", status)
	}

	// Assert git log shows merge commit (2 parents)
	parents := runGit(t, repo, "log", "-1", "--format=%P")
	parentList := strings.Fields(parents)
	if len(parentList) < 2 {
		t.Errorf("git log -1 parents = %v, want at least 2 parents for merge commit", parentList)
	}

	// Assert git diff --diff-filter=U is empty
	unmerged := runGit(t, repo, "diff", "--name-only", "--diff-filter=U")
	if unmerged != "" {
		t.Errorf("git diff --diff-filter=U = %q, want empty", unmerged)
	}
}

func TestResolvePartialFailureLeavesMarkers(t *testing.T) {
	repo := initConflictedRepo(t, map[string][2]string{
		"file1.txt": {"branch a file 1\n", "branch b file 1\n"},
		"file2.txt": {"branch a file 2\n", "branch b file 2\n"},
	})

	headBefore := runGit(t, repo, "rev-parse", "HEAD")

	fakeInvoker := func(ctx context.Context, worktreePath, prompt string) (string, error) {
		// Clean file1 but leave file2 conflicted
		if err := os.WriteFile(filepath.Join(worktreePath, "file1.txt"), []byte("resolved file 1\n"), 0o644); err != nil {
			return "", err
		}
		return "partial resolution", nil
	}

	resolved, out, err := resolve.Resolve(context.Background(), repo, fakeInvoker)
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if resolved {
		t.Fatalf("Resolve() resolved = true, want false")
	}
	if out != "partial resolution" {
		t.Errorf("Resolve() output = %q, want %q", out, "partial resolution")
	}

	// Assert nothing was committed
	headAfter := runGit(t, repo, "rev-parse", "HEAD")
	if headAfter != headBefore {
		t.Errorf("HEAD changed from %q to %q, want unchanged", headBefore, headAfter)
	}

	// Assert unmerged files still present in git status
	status := runGit(t, repo, "status", "--porcelain")
	if !strings.Contains(status, "file2.txt") {
		t.Errorf("git status --porcelain = %q, want to contain unresolved file2.txt", status)
	}
}

func TestResolveInvokerError(t *testing.T) {
	repo := initConflictedRepo(t, map[string][2]string{
		"file1.txt": {"branch a file 1\n", "branch b file 1\n"},
	})

	headBefore := runGit(t, repo, "rev-parse", "HEAD")

	fakeInvoker := func(ctx context.Context, worktreePath, prompt string) (string, error) {
		return "something failed", errors.New("invoker failed")
	}

	resolved, out, err := resolve.Resolve(context.Background(), repo, fakeInvoker)
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if resolved {
		t.Fatalf("Resolve() resolved = true, want false")
	}
	if out != "something failed" {
		t.Errorf("Resolve() output = %q, want %q", out, "something failed")
	}

	headAfter := runGit(t, repo, "rev-parse", "HEAD")
	if headAfter != headBefore {
		t.Errorf("HEAD changed from %q to %q, want unchanged", headBefore, headAfter)
	}
}

func TestResolvePromptStructuralContract(t *testing.T) {
	repo := initConflictedRepo(t, map[string][2]string{
		"alpha.txt": {"alpha version a\n", "alpha version b\n"},
		"beta.txt":  {"beta version a\n", "beta version b\n"},
	})

	var capturedPrompt string
	fakeInvoker := func(ctx context.Context, worktreePath, prompt string) (string, error) {
		capturedPrompt = prompt
		// Clean files so resolve succeeds
		_ = os.WriteFile(filepath.Join(worktreePath, "alpha.txt"), []byte("resolved alpha\n"), 0o644)
		_ = os.WriteFile(filepath.Join(worktreePath, "beta.txt"), []byte("resolved beta\n"), 0o644)
		return "done", nil
	}

	resolved, _, err := resolve.Resolve(context.Background(), repo, fakeInvoker)
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if !resolved {
		t.Fatalf("Resolve() resolved = false, want true")
	}

	// Verify prompt contains paths and conflict markers / content
	if !strings.Contains(capturedPrompt, "alpha.txt") {
		t.Errorf("prompt missing path alpha.txt")
	}
	if !strings.Contains(capturedPrompt, "beta.txt") {
		t.Errorf("prompt missing path beta.txt")
	}
	if !strings.Contains(capturedPrompt, "<<<<<<<") || !strings.Contains(capturedPrompt, "=======") || !strings.Contains(capturedPrompt, ">>>>>>>") {
		t.Errorf("prompt missing conflict markers")
	}
	if !strings.Contains(capturedPrompt, "alpha version a") || !strings.Contains(capturedPrompt, "alpha version b") {
		t.Errorf("prompt missing alpha conflicted content")
	}
	if !strings.Contains(capturedPrompt, "beta version a") || !strings.Contains(capturedPrompt, "beta version b") {
		t.Errorf("prompt missing beta conflicted content")
	}
}

func TestResolveAppliesTimeout(t *testing.T) {
	repo := initConflictedRepo(t, map[string][2]string{
		"file1.txt": {"branch a\n", "branch b\n"},
	})

	var hadDeadline bool
	var remaining time.Duration

	fakeInvoker := func(ctx context.Context, worktreePath, prompt string) (string, error) {
		deadline, ok := ctx.Deadline()
		hadDeadline = ok
		if ok {
			remaining = time.Until(deadline)
		}
		_ = os.WriteFile(filepath.Join(worktreePath, "file1.txt"), []byte("resolved\n"), 0o644)
		return "done", nil
	}

	_, _, err := resolve.Resolve(context.Background(), repo, fakeInvoker)
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}

	if !hadDeadline {
		t.Errorf("invoker context had no deadline, want 5-minute timeout")
	}
	if remaining <= 4*time.Minute || remaining > 5*time.Minute {
		t.Errorf("remaining timeout = %v, want ~5 minutes", remaining)
	}
}

func TestResolveNothingConflicted(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "resolve-test@example.com")
	runGit(t, root, "config", "user.name", "resolve-test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "initial")

	invokerCalled := false
	fakeInvoker := func(ctx context.Context, worktreePath, prompt string) (string, error) {
		invokerCalled = true
		return "", nil
	}

	resolved, out, err := resolve.Resolve(context.Background(), root, fakeInvoker)
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if !resolved {
		t.Errorf("Resolve() resolved = false, want true")
	}
	if out != "nothing to resolve" {
		t.Errorf("Resolve() output = %q, want %q", out, "nothing to resolve")
	}
	if invokerCalled {
		t.Errorf("fake invoker called when nothing conflicted")
	}
}
