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
