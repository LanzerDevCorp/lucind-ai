package worktree_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

func TestIsLinkedWorktree(t *testing.T) {
	t.Run("dot git as a directory is a primary repo root, not a linked worktree", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
			t.Fatalf("Mkdir(.git) error = %v", err)
		}

		if got := worktree.IsLinkedWorktree(root); got {
			t.Errorf("IsLinkedWorktree(%q) = true, want false", root)
		}
	})

	t.Run("dot git as a file with a gitdir pointer is a linked worktree", func(t *testing.T) {
		root := t.TempDir()
		content := "gitdir: /home/user/repo/.git/worktrees/lane1\n"
		if err := os.WriteFile(filepath.Join(root, ".git"), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(.git) error = %v", err)
		}

		if got := worktree.IsLinkedWorktree(root); !got {
			t.Errorf("IsLinkedWorktree(%q) = false, want true", root)
		}
	})

	t.Run("dot git as a file without a gitdir pointer is not trusted", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, ".git"), []byte("not a gitdir pointer\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(.git) error = %v", err)
		}

		if got := worktree.IsLinkedWorktree(root); got {
			t.Errorf("IsLinkedWorktree(%q) = true, want false", root)
		}
	})

	t.Run("no dot git entry at all is not a worktree", func(t *testing.T) {
		root := t.TempDir()

		if got := worktree.IsLinkedWorktree(root); got {
			t.Errorf("IsLinkedWorktree(%q) = true, want false", root)
		}
	})

	t.Run("path that does not exist is not a worktree", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "does-not-exist")

		if got := worktree.IsLinkedWorktree(root); got {
			t.Errorf("IsLinkedWorktree(%q) = true, want false", root)
		}
	})
}

func TestCreateRejectsEmptyLaneID(t *testing.T) {
	primaryRoot := t.TempDir()

	_, err := worktree.Create(context.Background(), primaryRoot, "")
	if !errors.Is(err, worktree.ErrEmptyLaneID) {
		t.Fatalf("Create() error = %v, want %v", err, worktree.ErrEmptyLaneID)
	}
}

func TestCreateRejectsExistingWorktreePath(t *testing.T) {
	parent := t.TempDir()
	primaryRoot := filepath.Join(parent, "repo")
	if err := os.Mkdir(primaryRoot, 0o755); err != nil {
		t.Fatalf("Mkdir(primaryRoot) error = %v", err)
	}

	target := filepath.Join(parent, "repo-worktrees", "fix-auth")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll(target) error = %v", err)
	}

	_, err := worktree.Create(context.Background(), primaryRoot, "fix-auth")
	if !errors.Is(err, worktree.ErrWorktreeExists) {
		t.Fatalf("Create() error = %v, want %v", err, worktree.ErrWorktreeExists)
	}
}

// initRepo creates a throwaway git repository in t.TempDir() with one
// commit, so "git worktree add" has a HEAD to branch from. It works on a
// machine with no configured git identity by passing user.email/user.name
// directly to each command.
func initRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	runGit(t, root, "init")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md) error = %v", err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "seed commit")

	return root
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{
		"-c", "user.email=worktree-test@example.com",
		"-c", "user.name=worktree-test",
	}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v error = %v, output = %s", args, err, out)
	}
}

func TestCreateAddsLinkedWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	wt, err := worktree.Create(context.Background(), primaryRoot, "fix-auth")
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	wantPath := filepath.Join(filepath.Dir(primaryRoot), filepath.Base(primaryRoot)+"-worktrees", "fix-auth")
	if wt.Path != wantPath {
		t.Errorf("Path = %q, want %q", wt.Path, wantPath)
	}
	if wt.Branch != "lucind/fix-auth" {
		t.Errorf("Branch = %q, want %q", wt.Branch, "lucind/fix-auth")
	}

	if !worktree.IsLinkedWorktree(wt.Path) {
		t.Errorf("IsLinkedWorktree(%q) = false, want true", wt.Path)
	}

	branchOut := bytes.Buffer{}
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = wt.Path
	cmd.Stdout = &branchOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("git rev-parse error = %v", err)
	}
	gotBranch := strings.TrimSpace(branchOut.String())
	if gotBranch != "lucind/fix-auth" {
		t.Errorf("checked out branch = %q, want %q", gotBranch, "lucind/fix-auth")
	}
}

func TestCreateRecordsBaseSHA(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	wt, err := worktree.Create(context.Background(), primaryRoot, "fix-auth")
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	headOut := bytes.Buffer{}
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = wt.Path
	cmd.Stdout = &headOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("git rev-parse error = %v", err)
	}
	wantSHA := strings.TrimSpace(headOut.String())

	if wt.BaseSHA == "" {
		t.Errorf("BaseSHA is empty, want %q", wantSHA)
	}
	if wt.BaseSHA != wantSHA {
		t.Errorf("BaseSHA = %q, want %q", wt.BaseSHA, wantSHA)
	}
}

func TestCreateWrapsGitFailureWithStderr(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	// primaryRoot is not a git repository at all, so "git worktree add"
	// fails immediately with a stderr message we can assert on.
	primaryRoot := t.TempDir()

	_, err := worktree.Create(context.Background(), primaryRoot, "fix-auth")
	if err == nil {
		t.Fatalf("Create() error = nil, want non-nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not a git repository") {
		t.Errorf("Create() error = %q, want it to contain git's stderr (e.g. %q)", err.Error(), "not a git repository")
	}
}

func TestCreateHonoursCancelledContext(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := worktree.Create(ctx, primaryRoot, "fix-auth")
	if err == nil {
		t.Fatalf("Create() error = nil, want non-nil for an already-cancelled context")
	}

	target := filepath.Join(filepath.Dir(primaryRoot), filepath.Base(primaryRoot)+"-worktrees", "fix-auth")
	if _, statErr := os.Stat(target); statErr == nil {
		t.Errorf("worktree path %q was created despite a cancelled context", target)
	}
}

func TestBranchFor(t *testing.T) {
	got := worktree.BranchFor("fix-auth")
	want := "lucind/fix-auth"
	if got != want {
		t.Errorf("BranchFor(%q) = %q, want %q", "fix-auth", got, want)
	}
}

func TestRemoveRemovesLinkedWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	wt, err := worktree.Create(context.Background(), primaryRoot, "fix-auth")
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	if !worktree.IsLinkedWorktree(wt.Path) {
		t.Fatalf("IsLinkedWorktree(%q) = false, want true before removal", wt.Path)
	}

	if err := worktree.Remove(context.Background(), primaryRoot, wt.Path); err != nil {
		t.Fatalf("Remove() error = %v, want nil", err)
	}

	if worktree.IsLinkedWorktree(wt.Path) {
		t.Errorf("IsLinkedWorktree(%q) = true, want false after removal", wt.Path)
	}

	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Errorf("os.Stat(%q) err = %v, want os.IsNotExist", wt.Path, err)
	}
}

func TestRemoveWrapsGitFailureWithStderr(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := t.TempDir()

	err := worktree.Remove(context.Background(), primaryRoot, filepath.Join(primaryRoot, "non-existent"))
	if err == nil {
		t.Fatalf("Remove() error = nil, want non-nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not a git repository") && !strings.Contains(strings.ToLower(err.Error()), "fatal") {
		t.Errorf("Remove() error = %q, want it to contain git's stderr", err.Error())
	}
}

func TestRemoveHonoursCancelledContext(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	wt, err := worktree.Create(context.Background(), primaryRoot, "fix-auth")
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = worktree.Remove(ctx, primaryRoot, wt.Path)
	if err == nil {
		t.Fatalf("Remove() error = nil, want non-nil for an already-cancelled context")
	}

	if !worktree.IsLinkedWorktree(wt.Path) {
		t.Errorf("worktree path %q was removed despite a cancelled context", wt.Path)
	}
}

func TestDeleteBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	wt, err := worktree.Create(context.Background(), primaryRoot, "fix-auth")
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	if err := worktree.Remove(context.Background(), primaryRoot, wt.Path); err != nil {
		t.Fatalf("Remove() error = %v, want nil", err)
	}

	if err := worktree.DeleteBranch(context.Background(), primaryRoot, wt.Branch); err != nil {
		t.Fatalf("DeleteBranch() error = %v, want nil", err)
	}

	var branchListOut bytes.Buffer
	cmd := exec.Command("git", "branch", "--list", wt.Branch)
	cmd.Dir = primaryRoot
	cmd.Stdout = &branchListOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("git branch --list error = %v", err)
	}

	if strings.TrimSpace(branchListOut.String()) != "" {
		t.Errorf("git branch --list %q = %q, want empty", wt.Branch, branchListOut.String())
	}
}

func TestDeleteBranchWrapsGitFailureWithStderr(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := t.TempDir()

	err := worktree.DeleteBranch(context.Background(), primaryRoot, "lucind/non-existent")
	if err == nil {
		t.Fatalf("DeleteBranch() error = nil, want non-nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not a git repository") && !strings.Contains(strings.ToLower(err.Error()), "fatal") && !strings.Contains(strings.ToLower(err.Error()), "error") {
		t.Errorf("DeleteBranch() error = %q, want it to contain git's stderr", err.Error())
	}
}

func TestDeleteBranchHonoursCancelledContext(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	wt, err := worktree.Create(context.Background(), primaryRoot, "fix-auth")
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	if err := worktree.Remove(context.Background(), primaryRoot, wt.Path); err != nil {
		t.Fatalf("Remove() error = %v, want nil", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = worktree.DeleteBranch(ctx, primaryRoot, wt.Branch)
	if err == nil {
		t.Fatalf("DeleteBranch() error = nil, want non-nil for an already-cancelled context")
	}

	var branchListOut bytes.Buffer
	cmd := exec.Command("git", "branch", "--list", wt.Branch)
	cmd.Dir = primaryRoot
	cmd.Stdout = &branchListOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("git branch --list error = %v", err)
	}

	if strings.TrimSpace(branchListOut.String()) == "" {
		t.Errorf("branch %q was deleted despite a cancelled context", wt.Branch)
	}
}

func TestHasUniqueCommits(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	wt, err := worktree.Create(context.Background(), primaryRoot, "lane1")
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	// Fresh worktree: no unique commits.
	hasCommits, err := worktree.HasUniqueCommits(context.Background(), wt.Path, wt.BaseSHA)
	if err != nil {
		t.Fatalf("HasUniqueCommits() error = %v, want nil", err)
	}
	if hasCommits {
		t.Errorf("HasUniqueCommits() = true, want false for fresh worktree")
	}

	// Commit on the lane branch.
	laneFile := filepath.Join(wt.Path, "feature.txt")
	if err := os.WriteFile(laneFile, []byte("lane work\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(feature.txt) error = %v", err)
	}
	runGit(t, wt.Path, "add", "feature.txt")
	runGit(t, wt.Path, "commit", "-m", "lane commit")

	// Now unique commits are present.
	hasCommits, err = worktree.HasUniqueCommits(context.Background(), wt.Path, wt.BaseSHA)
	if err != nil {
		t.Fatalf("HasUniqueCommits() error = %v, want nil", err)
	}
	if !hasCommits {
		t.Errorf("HasUniqueCommits() = false, want true after commit on lane branch")
	}
}

func TestHasUniqueCommitsUsesRecordedBaseSHANotLivePrimaryHEAD(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	wt, err := worktree.Create(context.Background(), primaryRoot, "lane1")
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	// Advance primary HEAD with an unrelated commit
	if err := os.WriteFile(filepath.Join(primaryRoot, "primary.txt"), []byte("primary work\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(primary.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "primary.txt")
	runGit(t, primaryRoot, "commit", "-m", "primary advance")

	// (a) fresh lane still reports no unique commits against recorded baseSHA
	hasCommits, err := worktree.HasUniqueCommits(context.Background(), wt.Path, wt.BaseSHA)
	if err != nil {
		t.Fatalf("HasUniqueCommits() error = %v, want nil", err)
	}
	if hasCommits {
		t.Errorf("HasUniqueCommits() = true, want false against recorded BaseSHA for fresh lane")
	}

	// (b) after a lane commit, unique commits is true against recorded baseSHA even though primary moved
	laneFile := filepath.Join(wt.Path, "feature.txt")
	if err := os.WriteFile(laneFile, []byte("lane work\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(feature.txt) error = %v", err)
	}
	runGit(t, wt.Path, "add", "feature.txt")
	runGit(t, wt.Path, "commit", "-m", "lane commit")

	hasCommits, err = worktree.HasUniqueCommits(context.Background(), wt.Path, wt.BaseSHA)
	if err != nil {
		t.Fatalf("HasUniqueCommits() error = %v, want nil", err)
	}
	if !hasCommits {
		t.Errorf("HasUniqueCommits() = false, want true after lane commit")
	}
}

func TestHasUniqueCommitsRejectsEmptyBaseSHA(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)
	wt, err := worktree.Create(context.Background(), primaryRoot, "lane1")
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	_, err = worktree.HasUniqueCommits(context.Background(), wt.Path, "")
	if err == nil {
		t.Fatalf("HasUniqueCommits with empty baseSHA error = nil, want non-nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "base sha") {
		t.Errorf("HasUniqueCommits error = %q, want it to name missing base SHA", err.Error())
	}
}

func TestHasUniqueCommitsHonoursCancelledContext(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	wt, err := worktree.Create(context.Background(), primaryRoot, "lane1")
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = worktree.HasUniqueCommits(ctx, wt.Path, wt.BaseSHA)
	if err == nil {
		t.Fatalf("HasUniqueCommits() error = nil, want non-nil for an already-cancelled context")
	}
}

func TestHasUniqueCommitsWrapsGitFailureWithStderr(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	wtDir := t.TempDir()

	_, err := worktree.HasUniqueCommits(context.Background(), wtDir, "dummy-base-sha-1234567890abcdef")
	if err == nil {
		t.Fatalf("HasUniqueCommits() error = nil, want non-nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not a git repository") && !strings.Contains(strings.ToLower(err.Error()), "fatal") && !strings.Contains(strings.ToLower(err.Error()), "error") {
		t.Errorf("HasUniqueCommits() error = %q, want git stderr", err.Error())
	}
}

func TestPorcelainEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	// Add .gitignore in primary repo ignoring .lucind/ and commit it.
	gitignorePath := filepath.Join(primaryRoot, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(".lucind/\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(.gitignore) error = %v", err)
	}
	runGit(t, primaryRoot, "add", ".gitignore")
	runGit(t, primaryRoot, "commit", "-m", "ignore .lucind/")

	wt, err := worktree.Create(context.Background(), primaryRoot, "lane1")
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	// Fresh worktree: porcelain is empty.
	empty, err := worktree.PorcelainEmpty(context.Background(), wt.Path)
	if err != nil {
		t.Fatalf("PorcelainEmpty() error = %v, want nil", err)
	}
	if !empty {
		t.Errorf("PorcelainEmpty() = false, want true for fresh worktree")
	}

	// Writing only .lucind/result.json in a repo ignoring .lucind/ leaves porcelain empty.
	lucindDir := filepath.Join(wt.Path, ".lucind")
	if err := os.MkdirAll(lucindDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(.lucind) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(lucindDir, "result.json"), []byte(`{"status":"done"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(result.json) error = %v", err)
	}

	empty, err = worktree.PorcelainEmpty(context.Background(), wt.Path)
	if err != nil {
		t.Fatalf("PorcelainEmpty() error = %v, want nil", err)
	}
	if !empty {
		t.Errorf("PorcelainEmpty() = false, want true when only ignored .lucind/result.json is present")
	}

	// Adding an untracked non-ignored file makes PorcelainEmpty false.
	untrackedFile := filepath.Join(wt.Path, "untracked.txt")
	if err := os.WriteFile(untrackedFile, []byte("untracked content\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(untracked.txt) error = %v", err)
	}

	empty, err = worktree.PorcelainEmpty(context.Background(), wt.Path)
	if err != nil {
		t.Fatalf("PorcelainEmpty() error = %v, want nil", err)
	}
	if empty {
		t.Errorf("PorcelainEmpty() = true, want false when untracked non-ignored file is present")
	}
}

func TestPorcelainEmptyHonoursCancelledContext(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	wt, err := worktree.Create(context.Background(), primaryRoot, "lane1")
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = worktree.PorcelainEmpty(ctx, wt.Path)
	if err == nil {
		t.Fatalf("PorcelainEmpty() error = nil, want non-nil for an already-cancelled context")
	}
}

func TestPorcelainEmptyWrapsGitFailureWithStderr(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	wtDir := t.TempDir()

	_, err := worktree.PorcelainEmpty(context.Background(), wtDir)
	if err == nil {
		t.Fatalf("PorcelainEmpty() error = nil, want non-nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not a git repository") && !strings.Contains(strings.ToLower(err.Error()), "fatal") && !strings.Contains(strings.ToLower(err.Error()), "error") {
		t.Errorf("PorcelainEmpty() error = %q, want git stderr", err.Error())
	}
}

// TestLinkedWorktreeInheritsCommittedMechanicalLog (Task 2.3 & 2.4) proves that a log file
// committed to the primary repository branch is automatically inherited into newly created linked
// worktrees via git branch inheritance, requiring zero custom file copying.
func TestLinkedWorktreeInheritsCommittedMechanicalLog(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	logDir := filepath.Join(primaryRoot, "openspec", "changes", "verify-test")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}

	sampleContent := "=== lucind-ai mechanical check ===\n" +
		"Git Commit SHA: 1234567890abcdef1234567890abcdef12345678\n" +
		"Command: lucind-checks.sh\n" +
		"Duration: 1.5s\n" +
		"Exit Code: 0\n" +
		"==================================\n" +
		"PASS: all suites clean\n"

	logPath := filepath.Join(logDir, "verify-mechanical.log")
	if err := os.WriteFile(logPath, []byte(sampleContent), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	runGit(t, primaryRoot, "add", "openspec/changes/verify-test/verify-mechanical.log")
	runGit(t, primaryRoot, "commit", "-m", "chore: record mechanical check")

	wt, err := worktree.Create(context.Background(), primaryRoot, "verify-lane-test")
	if err != nil {
		t.Fatalf("worktree.Create() error = %v", err)
	}

	wtLogPath := filepath.Join(wt.Path, "openspec", "changes", "verify-test", "verify-mechanical.log")
	wtLogBytes, err := os.ReadFile(wtLogPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) in linked worktree error = %v", wtLogPath, err)
	}

	if string(wtLogBytes) != sampleContent {
		t.Fatalf("linked worktree log content = %q, want %q", string(wtLogBytes), sampleContent)
	}
}



