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

	if err := worktree.Remove(context.Background(), primaryRoot, wt.Path, false); err != nil {
		t.Fatalf("Remove() error = %v, want nil", err)
	}

	if worktree.IsLinkedWorktree(wt.Path) {
		t.Errorf("IsLinkedWorktree(%q) = true, want false after removal", wt.Path)
	}

	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Errorf("os.Stat(%q) err = %v, want os.IsNotExist", wt.Path, err)
	}
}

func TestRemoveDirtyFailsClosedWithoutForce(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	t.Run("staged changes", func(t *testing.T) {
		primaryRoot := initRepo(t)
		wt, err := worktree.Create(context.Background(), primaryRoot, "lane-staged")
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		stagedFile := filepath.Join(wt.Path, "staged.txt")
		if err := os.WriteFile(stagedFile, []byte("staged content\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		runGit(t, wt.Path, "add", "staged.txt")

		err = worktree.Remove(context.Background(), primaryRoot, wt.Path, false)
		if !errors.Is(err, worktree.ErrWorktreeDirty) {
			t.Fatalf("Remove(force=false) error = %v, want %v", err, worktree.ErrWorktreeDirty)
		}

		if !worktree.IsLinkedWorktree(wt.Path) {
			t.Errorf("worktree was removed despite being dirty")
		}
		if _, err := os.Stat(stagedFile); err != nil {
			t.Errorf("staged file was deleted: %v", err)
		}
	})

	t.Run("unstaged changes", func(t *testing.T) {
		primaryRoot := initRepo(t)
		wt, err := worktree.Create(context.Background(), primaryRoot, "lane-unstaged")
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		trackedFile := filepath.Join(wt.Path, "tracked.txt")
		if err := os.WriteFile(trackedFile, []byte("initial\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		runGit(t, wt.Path, "add", "tracked.txt")
		runGit(t, wt.Path, "commit", "-m", "add tracked.txt")

		if err := os.WriteFile(trackedFile, []byte("modified\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		err = worktree.Remove(context.Background(), primaryRoot, wt.Path, false)
		if !errors.Is(err, worktree.ErrWorktreeDirty) {
			t.Fatalf("Remove(force=false) error = %v, want %v", err, worktree.ErrWorktreeDirty)
		}

		if !worktree.IsLinkedWorktree(wt.Path) {
			t.Errorf("worktree was removed despite being dirty")
		}
		data, err := os.ReadFile(trackedFile)
		if err != nil || string(data) != "modified\n" {
			t.Errorf("tracked file content corrupted or missing: %v, data=%q", err, string(data))
		}
	})

	t.Run("untracked files", func(t *testing.T) {
		primaryRoot := initRepo(t)
		wt, err := worktree.Create(context.Background(), primaryRoot, "lane-untracked")
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		untrackedFile := filepath.Join(wt.Path, "untracked.txt")
		if err := os.WriteFile(untrackedFile, []byte("untracked content\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		err = worktree.Remove(context.Background(), primaryRoot, wt.Path, false)
		if !errors.Is(err, worktree.ErrWorktreeDirty) {
			t.Fatalf("Remove(force=false) error = %v, want %v", err, worktree.ErrWorktreeDirty)
		}

		if !worktree.IsLinkedWorktree(wt.Path) {
			t.Errorf("worktree was removed despite being dirty")
		}
		if _, err := os.Stat(untrackedFile); err != nil {
			t.Errorf("untracked file was deleted: %v", err)
		}
	})
}

func TestRemoveDirtySucceedsWithForce(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)
	wt, err := worktree.Create(context.Background(), primaryRoot, "lane-force")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	untrackedFile := filepath.Join(wt.Path, "untracked.txt")
	if err := os.WriteFile(untrackedFile, []byte("untracked content\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := worktree.Remove(context.Background(), primaryRoot, wt.Path, true); err != nil {
		t.Fatalf("Remove(force=true) error = %v, want nil", err)
	}

	if worktree.IsLinkedWorktree(wt.Path) {
		t.Errorf("IsLinkedWorktree(%q) = true, want false after forced removal", wt.Path)
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

	err := worktree.Remove(context.Background(), primaryRoot, filepath.Join(primaryRoot, "non-existent"), true)
	if err == nil {
		t.Fatalf("Remove() error = nil, want non-nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not a git repository") && !strings.Contains(strings.ToLower(err.Error()), "fatal") {
		t.Errorf("Remove() error = %q, want it to contain git's stderr", err.Error())
	}
}

func TestRemoveInvalidPathFailsCleanly(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := t.TempDir()

	err := worktree.Remove(context.Background(), primaryRoot, filepath.Join(primaryRoot, "non-existent"), false)
	if err == nil {
		t.Fatalf("Remove(force=false) on invalid path error = nil, want non-nil")
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

	err = worktree.Remove(ctx, primaryRoot, wt.Path, false)
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

	if err := worktree.Remove(context.Background(), primaryRoot, wt.Path, false); err != nil {
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

	if err := worktree.Remove(context.Background(), primaryRoot, wt.Path, false); err != nil {
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

func TestCreateWithParentExplicitSHA(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	// Seed commit is the initial commit
	seedSHAOut := bytes.Buffer{}
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = primaryRoot
	cmd.Stdout = &seedSHAOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("git rev-parse HEAD error = %v", err)
	}
	seedSHA := strings.TrimSpace(seedSHAOut.String())

	// Advance main with a second commit
	if err := os.WriteFile(filepath.Join(primaryRoot, "file2.txt"), []byte("second commit\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(file2.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "file2.txt")
	runGit(t, primaryRoot, "commit", "-m", "second commit on main")

	mainHeadOut := bytes.Buffer{}
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = primaryRoot
	cmd.Stdout = &mainHeadOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("git rev-parse HEAD error = %v", err)
	}
	mainHeadSHA := strings.TrimSpace(mainHeadOut.String())

	if seedSHA == mainHeadSHA {
		t.Fatalf("seedSHA (%s) unexpectedly equals mainHeadSHA (%s)", seedSHA, mainHeadSHA)
	}

	// Create feature parent branch pointing at seedSHA
	runGit(t, primaryRoot, "branch", "feature-alpha", seedSHA)

	// Create worktree with explicit parent and base SHA pointing to seedSHA
	wt, err := worktree.CreateWithParent(context.Background(), primaryRoot, "lane-explicit", "refs/heads/feature-alpha", seedSHA)
	if err != nil {
		t.Fatalf("CreateWithParent() error = %v, want nil", err)
	}

	if wt.BaseSHA != seedSHA {
		t.Errorf("BaseSHA = %q, want %q", wt.BaseSHA, seedSHA)
	}

	// Verify via git -C <path> rev-parse HEAD that the worktree HEAD equals seedSHA, not mainHeadSHA
	wtHeadOut := bytes.Buffer{}
	cmd = exec.Command("git", "-C", wt.Path, "rev-parse", "HEAD")
	cmd.Stdout = &wtHeadOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("git -C wt.Path rev-parse HEAD error = %v", err)
	}
	wtHeadSHA := strings.TrimSpace(wtHeadOut.String())

	if wtHeadSHA != seedSHA {
		t.Errorf("worktree HEAD = %q, want explicit seedSHA %q (not %q)", wtHeadSHA, seedSHA, mainHeadSHA)
	}

	// Verify legacy Create still starts at primaryRoot checkout (mainHeadSHA)
	legacyWT, err := worktree.Create(context.Background(), primaryRoot, "lane-legacy")
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if legacyWT.BaseSHA != mainHeadSHA {
		t.Errorf("legacy Create BaseSHA = %q, want mainHeadSHA %q", legacyWT.BaseSHA, mainHeadSHA)
	}
}

func TestCreateWithParentRejectsInvalidParentRef(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)
	seedSHA := strings.TrimSpace(runGitOutput(t, primaryRoot, "rev-parse", "HEAD"))

	testCases := []struct {
		name      string
		parentRef string
	}{
		{name: "empty parent ref", parentRef: ""},
		{name: "whitespace only", parentRef: "   "},
		{name: "bare main", parentRef: "main"},
		{name: "canonical main", parentRef: "refs/heads/main"},
		{name: "bare lucind", parentRef: "lucind"},
		{name: "canonical lucind", parentRef: "refs/heads/lucind"},
		{name: "lucind temp namespace branch", parentRef: "lucind/lane-123"},
		{name: "canonical lucind temp namespace", parentRef: "refs/heads/lucind/lane-123"},
		{name: "double dot in ref", parentRef: "feature..invalid"},
		{name: "space in ref", parentRef: "feature name"},
		{name: "starts with dash", parentRef: "-bad-ref"},
		{name: "tilde in ref", parentRef: "feature~1"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			laneID := "lane-" + strings.ReplaceAll(tc.name, " ", "-")
			_, err := worktree.CreateWithParent(context.Background(), primaryRoot, laneID, tc.parentRef, seedSHA)
			if err == nil {
				t.Fatalf("CreateWithParent(parentRef=%q) error = nil, want ErrInvalidParentRef", tc.parentRef)
			}
			if !errors.Is(err, worktree.ErrInvalidParentRef) {
				t.Errorf("CreateWithParent(parentRef=%q) error = %v, want errors.Is(..., worktree.ErrInvalidParentRef)", tc.parentRef, err)
			}

			// Ensure no worktree path was created
			expectedPath := filepath.Join(filepath.Dir(primaryRoot), filepath.Base(primaryRoot)+"-worktrees", laneID)
			if _, statErr := os.Stat(expectedPath); statErr == nil {
				t.Errorf("worktree directory %q was created despite invalid parent ref", expectedPath)
			}
		})
	}
}

func TestCreateWithParentRejectsInvalidBaseSHA(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	// Create a blob in git object database so we have a valid object that is NOT a commit
	blobSHA := strings.TrimSpace(runGitOutput(t, primaryRoot, "hash-object", "-w", "README.md"))

	testCases := []struct {
		name    string
		baseSHA string
	}{
		{name: "empty base sha", baseSHA: ""},
		{name: "whitespace base sha", baseSHA: "   "},
		{name: "non existent sha", baseSHA: "0123456789abcdef0123456789abcdef01234567"},
		{name: "blob object not a commit", baseSHA: blobSHA},
		{name: "garbage sha string", baseSHA: "not-a-sha-at-all"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			laneID := "lane-sha-" + strings.ReplaceAll(tc.name, " ", "-")
			_, err := worktree.CreateWithParent(context.Background(), primaryRoot, laneID, "refs/heads/feature-alpha", tc.baseSHA)
			if err == nil {
				t.Fatalf("CreateWithParent(baseSHA=%q) error = nil, want ErrInvalidBaseSHA", tc.baseSHA)
			}
			if !errors.Is(err, worktree.ErrInvalidBaseSHA) {
				t.Errorf("CreateWithParent(baseSHA=%q) error = %v, want errors.Is(..., worktree.ErrInvalidBaseSHA)", tc.baseSHA, err)
			}

			// Ensure no worktree path was created
			expectedPath := filepath.Join(filepath.Dir(primaryRoot), filepath.Base(primaryRoot)+"-worktrees", laneID)
			if _, statErr := os.Stat(expectedPath); statErr == nil {
				t.Errorf("worktree directory %q was created despite invalid base SHA", expectedPath)
			}
		})
	}
}

func TestCreateWithParentAncestryCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	initialBranch := strings.TrimSpace(runGitOutput(t, primaryRoot, "rev-parse", "--abbrev-ref", "HEAD"))

	// Commit 1 (seed)
	seedSHA := strings.TrimSpace(runGitOutput(t, primaryRoot, "rev-parse", "HEAD"))

	// Commit 2 on initial branch
	if err := os.WriteFile(filepath.Join(primaryRoot, "file2.txt"), []byte("commit 2\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(file2.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "file2.txt")
	runGit(t, primaryRoot, "commit", "-m", "commit 2")
	commit2SHA := strings.TrimSpace(runGitOutput(t, primaryRoot, "rev-parse", "HEAD"))

	// Branch feature-parent points to commit 2
	runGit(t, primaryRoot, "branch", "feature-parent", commit2SHA)

	// An unrelated orphan branch
	runGit(t, primaryRoot, "checkout", "--orphan", "orphan-branch")
	if err := os.WriteFile(filepath.Join(primaryRoot, "orphan.txt"), []byte("orphan content\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(orphan.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "orphan.txt")
	runGit(t, primaryRoot, "commit", "-m", "orphan commit")
	orphanSHA := strings.TrimSpace(runGitOutput(t, primaryRoot, "rev-parse", "HEAD"))

	// Return to initial branch
	runGit(t, primaryRoot, "checkout", initialBranch)

	// 1. seedSHA is an ancestor of feature-parent (at commit2) -> should succeed
	wt, err := worktree.CreateWithParent(context.Background(), primaryRoot, "lane-anc-ok", "refs/heads/feature-parent", seedSHA)
	if err != nil {
		t.Fatalf("CreateWithParent() ancestor error = %v, want nil", err)
	}
	if wt.BaseSHA != seedSHA {
		t.Errorf("BaseSHA = %q, want %q", wt.BaseSHA, seedSHA)
	}

	// 2. orphanSHA is NOT an ancestor of feature-parent -> should fail
	_, err = worktree.CreateWithParent(context.Background(), primaryRoot, "lane-anc-fail", "refs/heads/feature-parent", orphanSHA)
	if err == nil {
		t.Fatalf("CreateWithParent() non-ancestor error = nil, want ErrInvalidBaseSHA")
	}
	if !errors.Is(err, worktree.ErrInvalidBaseSHA) {
		t.Errorf("CreateWithParent() non-ancestor error = %v, want errors.Is(..., worktree.ErrInvalidBaseSHA)", err)
	}
}

func TestValidateParentRef(t *testing.T) {
	ctx := context.Background()

	validRefs := []string{
		"feature/auth",
		"refs/heads/feature/auth",
		"feature-123",
		"refs/heads/feature-123",
		"user/jane/work",
	}
	for _, ref := range validRefs {
		if err := worktree.ValidateParentRef(ctx, nil, ref); err != nil {
			t.Errorf("ValidateParentRef(%q) err = %v, want nil", ref, err)
		}
	}

	invalidRefs := []string{
		"",
		"   ",
		"main",
		"refs/heads/main",
		"lucind",
		"refs/heads/lucind",
		"lucind/lane-1",
		"refs/heads/lucind/lane-1",
		"feature..invalid",
		"feature space",
		"-bad",
	}
	for _, ref := range invalidRefs {
		if err := worktree.ValidateParentRef(ctx, nil, ref); !errors.Is(err, worktree.ErrInvalidParentRef) {
			t.Errorf("ValidateParentRef(%q) err = %v, want ErrInvalidParentRef", ref, err)
		}
	}
}

func TestCanonicalizeRef(t *testing.T) {
	if got := worktree.CanonicalizeRef("feature/auth"); got != "refs/heads/feature/auth" {
		t.Errorf("CanonicalizeRef(feature/auth) = %q, want refs/heads/feature/auth", got)
	}
	if got := worktree.CanonicalizeRef("refs/heads/feature/auth"); got != "refs/heads/feature/auth" {
		t.Errorf("CanonicalizeRef(refs/heads/feature/auth) = %q, want refs/heads/feature/auth", got)
	}
}

func TestResolveCommitSHA(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)
	seedSHA := strings.TrimSpace(runGitOutput(t, primaryRoot, "rev-parse", "HEAD"))

	// Valid commit
	sha, err := worktree.ResolveCommitSHA(context.Background(), nil, primaryRoot, seedSHA)
	if err != nil {
		t.Fatalf("ResolveCommitSHA(seedSHA) error = %v, want nil", err)
	}
	if sha != seedSHA {
		t.Errorf("ResolveCommitSHA(seedSHA) = %q, want %q", sha, seedSHA)
	}

	// Short SHA
	shortSHA := seedSHA[:7]
	sha, err = worktree.ResolveCommitSHA(context.Background(), nil, primaryRoot, shortSHA)
	if err != nil {
		t.Fatalf("ResolveCommitSHA(shortSHA) error = %v, want nil", err)
	}
	if sha != seedSHA {
		t.Errorf("ResolveCommitSHA(shortSHA) = %q, want %q", sha, seedSHA)
	}

	// Invalid SHA
	_, err = worktree.ResolveCommitSHA(context.Background(), nil, primaryRoot, "0000000000000000000000000000000000000000")
	if !errors.Is(err, worktree.ErrInvalidBaseSHA) {
		t.Errorf("ResolveCommitSHA(invalid) err = %v, want ErrInvalidBaseSHA", err)
	}
}

func TestGitRunnerSeam(t *testing.T) {
	fakeCalled := false
	fakeRunner := &mockGitRunner{
		runFn: func(ctx context.Context, dir string, args ...string) ([]byte, error) {
			fakeCalled = true
			if len(args) > 0 && args[0] == "check-ref-format" {
				return []byte("feature-test\n"), nil
			}
			if len(args) > 0 && args[0] == "rev-parse" {
				return []byte("1111222233334444555566667777888899990000\n"), nil
			}
			if len(args) > 0 && args[0] == "merge-base" {
				return []byte(""), nil
			}
			if len(args) > 0 && args[0] == "worktree" && args[1] == "add" {
				return []byte(""), nil
			}
			return []byte("1111222233334444555566667777888899990000\n"), nil
		},
	}

	primaryRoot := t.TempDir()
	wt, err := worktree.CreateWithRunner(context.Background(), fakeRunner, primaryRoot, "lane-mock", "refs/heads/feature-test", "1111222233334444555566667777888899990000")
	if err != nil {
		t.Fatalf("CreateWithRunner() error = %v", err)
	}
	if !fakeCalled {
		t.Errorf("fake runner was not invoked")
	}
	if wt.BaseSHA != "1111222233334444555566667777888899990000" {
		t.Errorf("wt.BaseSHA = %q, want mocked SHA", wt.BaseSHA)
	}
}

type mockGitRunner struct {
	runFn func(ctx context.Context, dir string, args ...string) ([]byte, error)
}

func (m *mockGitRunner) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if m.runFn != nil {
		return m.runFn(ctx, dir, args...)
	}
	return nil, nil
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", append([]string{
		"-c", "user.email=worktree-test@example.com",
		"-c", "user.name=worktree-test",
	}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v error = %v, output = %s", args, err, out)
	}
	return string(out)
}

func TestCleanupRemovesExistingWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	wt, err := worktree.Create(context.Background(), primaryRoot, "fix-auth")
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	if !worktree.IsLinkedWorktree(wt.Path) {
		t.Fatalf("IsLinkedWorktree(%q) = false, want true before cleanup", wt.Path)
	}

	if err := worktree.Cleanup(context.Background(), primaryRoot, "fix-auth", false); err != nil {
		t.Fatalf("Cleanup() error = %v, want nil", err)
	}

	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Errorf("os.Stat(%q) err = %v, want os.IsNotExist", wt.Path, err)
	}
}

func TestCleanupDirtyFailsClosedWithoutForce(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	t.Run("staged changes", func(t *testing.T) {
		primaryRoot := initRepo(t)
		wt, err := worktree.Create(context.Background(), primaryRoot, "lane-staged")
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		stagedFile := filepath.Join(wt.Path, "staged.txt")
		if err := os.WriteFile(stagedFile, []byte("staged content\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		runGit(t, wt.Path, "add", "staged.txt")

		err = worktree.Cleanup(context.Background(), primaryRoot, "lane-staged", false)
		if !errors.Is(err, worktree.ErrWorktreeDirty) {
			t.Fatalf("Cleanup(force=false) error = %v, want %v", err, worktree.ErrWorktreeDirty)
		}

		if !worktree.IsLinkedWorktree(wt.Path) {
			t.Errorf("worktree was removed despite being dirty")
		}
		if _, err := os.Stat(stagedFile); err != nil {
			t.Errorf("staged file was deleted: %v", err)
		}
	})

	t.Run("unstaged changes", func(t *testing.T) {
		primaryRoot := initRepo(t)
		wt, err := worktree.Create(context.Background(), primaryRoot, "lane-unstaged")
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		trackedFile := filepath.Join(wt.Path, "tracked.txt")
		if err := os.WriteFile(trackedFile, []byte("initial\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		runGit(t, wt.Path, "add", "tracked.txt")
		runGit(t, wt.Path, "commit", "-m", "add tracked.txt")

		if err := os.WriteFile(trackedFile, []byte("modified\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		err = worktree.Cleanup(context.Background(), primaryRoot, "lane-unstaged", false)
		if !errors.Is(err, worktree.ErrWorktreeDirty) {
			t.Fatalf("Cleanup(force=false) error = %v, want %v", err, worktree.ErrWorktreeDirty)
		}

		if !worktree.IsLinkedWorktree(wt.Path) {
			t.Errorf("worktree was removed despite being dirty")
		}
	})

	t.Run("untracked files", func(t *testing.T) {
		primaryRoot := initRepo(t)
		wt, err := worktree.Create(context.Background(), primaryRoot, "lane-untracked")
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		untrackedFile := filepath.Join(wt.Path, "untracked.txt")
		if err := os.WriteFile(untrackedFile, []byte("untracked content\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		err = worktree.Cleanup(context.Background(), primaryRoot, "lane-untracked", false)
		if !errors.Is(err, worktree.ErrWorktreeDirty) {
			t.Fatalf("Cleanup(force=false) error = %v, want %v", err, worktree.ErrWorktreeDirty)
		}

		if !worktree.IsLinkedWorktree(wt.Path) {
			t.Errorf("worktree was removed despite being dirty")
		}
	})
}

func TestCleanupDirtySucceedsWithForce(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)
	wt, err := worktree.Create(context.Background(), primaryRoot, "lane-force")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	untrackedFile := filepath.Join(wt.Path, "untracked.txt")
	if err := os.WriteFile(untrackedFile, []byte("untracked content\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := worktree.Cleanup(context.Background(), primaryRoot, "lane-force", true); err != nil {
		t.Fatalf("Cleanup(force=true) error = %v, want nil", err)
	}

	if worktree.IsLinkedWorktree(wt.Path) {
		t.Errorf("IsLinkedWorktree(%q) = true, want false after forced cleanup", wt.Path)
	}
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Errorf("os.Stat(%q) err = %v, want os.IsNotExist", wt.Path, err)
	}
}

func TestCleanupOnLaneWithNoWorktreeIsNoOp(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	if err := worktree.Cleanup(context.Background(), primaryRoot, "never-created", false); err != nil {
		t.Fatalf("Cleanup(force=false) on lane with no worktree error = %v, want nil", err)
	}

	if err := worktree.Cleanup(context.Background(), primaryRoot, "never-created", true); err != nil {
		t.Fatalf("Cleanup(force=true) on lane with no worktree error = %v, want nil", err)
	}
}
