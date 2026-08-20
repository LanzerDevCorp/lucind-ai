// Package worktree creates and identifies linked git worktrees for lanes.
//
// A linked git worktree is distinguished from a primary repository root by
// one filesystem fact: a primary repository root has ".git" as a
// directory, while a linked worktree has ".git" as a file containing a
// "gitdir:" pointer back to the primary repository's internal worktree
// metadata. IsLinkedWorktree exists to close a deferred requirement: "no
// lane-state database is created inside a worktree's .lucind/".
// internal/ledgerpath.Validate only checks path containment against a
// primary repository root and cannot tell a worktree apart from a
// repository root by path shape alone (a worktree can live anywhere,
// including outside any "-worktrees" naming convention). This package adds
// the identity check ledgerpath deliberately does not have.
package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrEmptyLaneID is returned by Create when laneID is empty. A lane ID
// names both the branch and the worktree directory, so an empty one has
// nowhere to go.
var ErrEmptyLaneID = errors.New("worktree: lane ID must not be empty")

// ErrWorktreeExists is returned by Create when the target worktree path
// already exists. Create never overwrites or reuses an existing directory.
var ErrWorktreeExists = errors.New("worktree: target worktree path already exists")

// ErrEmptyBaseSHA is returned by HasUniqueCommits when baseSHA is empty.
var ErrEmptyBaseSHA = errors.New("worktree: base SHA must not be empty")

// worktreesDirSuffix names the sibling directory that holds every lane's
// linked worktree for a given primary repository.
const worktreesDirSuffix = "-worktrees"

// BranchFor returns the branch name for a lane: "lucind/" + laneID.
func BranchFor(laneID string) string {
	return "lucind/" + laneID
}

// pathFor returns the target worktree path for laneID off primaryRoot:
// "<parent-of-primaryRoot>/<basename-of-primaryRoot>-worktrees/<laneID>".
// This is a hard project rule, never a temp directory, because each
// worktree needs its own CodeGraph index rooted at a stable, discoverable
// location alongside the primary repository.
func pathFor(primaryRoot, laneID string) string {
	parent := filepath.Dir(primaryRoot)
	base := filepath.Base(primaryRoot)
	return filepath.Join(parent, base+worktreesDirSuffix, laneID)
}

// Worktree is a created linked git worktree for one lane.
type Worktree struct {
	Path    string // absolute path to the worktree directory
	Branch  string // the branch checked out in it
	BaseSHA string // hex SHA of this worktree's HEAD immediately after Create
}

// Create adds a linked git worktree for laneID off primaryRoot.
func Create(ctx context.Context, primaryRoot, laneID string) (Worktree, error) {
	if laneID == "" {
		return Worktree{}, ErrEmptyLaneID
	}

	path := pathFor(primaryRoot, laneID)
	if _, err := os.Stat(path); err == nil {
		return Worktree{}, ErrWorktreeExists
	}

	branch := BranchFor(laneID)

	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", branch, path)
	cmd.Dir = primaryRoot
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Worktree{}, fmt.Errorf("worktree: git worktree add failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	headCmd := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "HEAD")
	var headStderr strings.Builder
	headCmd.Stderr = &headStderr
	headOut, err := headCmd.Output()
	if err != nil {
		_ = Remove(ctx, primaryRoot, path)
		return Worktree{}, fmt.Errorf("worktree: git rev-parse HEAD failed: %w: %s", err, strings.TrimSpace(headStderr.String()))
	}
	baseSHA := strings.TrimSpace(string(headOut))

	return Worktree{Path: path, Branch: branch, BaseSHA: baseSHA}, nil
}

// Remove removes a linked git worktree at path off primaryRoot.
func Remove(ctx context.Context, primaryRoot, path string) error {
	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", "--force", path)
	cmd.Dir = primaryRoot
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("worktree: git worktree remove failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return nil
}

// DeleteBranch forces deletion of branch in primaryRoot.
func DeleteBranch(ctx context.Context, primaryRoot, branch string) error {
	cmd := exec.CommandContext(ctx, "git", "branch", "-D", branch)
	cmd.Dir = primaryRoot
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("worktree: git branch -D failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return nil
}

// IsLinkedWorktree reports whether path is the root of a linked git
// worktree, determined solely by the shape of its ".git" entry: a
// directory means path is a primary repository root (not a linked
// worktree), and a file whose contents start with "gitdir:" means it is.
// Any other shape — no ".git" at all, a path that does not exist, or a
// ".git" file that does not carry a "gitdir:" pointer — is not trusted and
// reports false.
func IsLinkedWorktree(path string) bool {
	dotGit := filepath.Join(path, ".git")

	info, err := os.Lstat(dotGit)
	if err != nil || info.IsDir() {
		return false
	}

	data, err := os.ReadFile(dotGit)
	if err != nil {
		return false
	}

	return strings.HasPrefix(string(data), "gitdir:")
}

// HasUniqueCommits reports whether the linked worktree at worktreePath has
// unique commits not present in baseSHA's history, determined by whether
// worktree HEAD differs from git merge-base HEAD <baseSHA>.
func HasUniqueCommits(ctx context.Context, worktreePath, baseSHA string) (bool, error) {
	if strings.TrimSpace(baseSHA) == "" {
		return false, ErrEmptyBaseSHA
	}

	wtHeadCmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "rev-parse", "HEAD")
	var wtStderr strings.Builder
	wtHeadCmd.Stderr = &wtStderr
	wtHeadOut, err := wtHeadCmd.Output()
	if err != nil {
		return false, fmt.Errorf("worktree: git rev-parse HEAD in worktree failed: %w: %s", err, strings.TrimSpace(wtStderr.String()))
	}
	wtHead := strings.TrimSpace(string(wtHeadOut))

	mergeBaseCmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "merge-base", "HEAD", baseSHA)
	var mergeBaseStderr strings.Builder
	mergeBaseCmd.Stderr = &mergeBaseStderr
	mergeBaseOut, err := mergeBaseCmd.Output()
	if err != nil {
		return false, fmt.Errorf("worktree: git merge-base failed: %w: %s", err, strings.TrimSpace(mergeBaseStderr.String()))
	}
	mergeBase := strings.TrimSpace(string(mergeBaseOut))

	return wtHead != mergeBase, nil
}

// PorcelainEmpty reports whether git status --porcelain in the linked worktree
// at worktreePath produces no output, using git's default ignore rules.
func PorcelainEmpty(ctx context.Context, worktreePath string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "status", "--porcelain")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("worktree: git status failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return len(strings.TrimSpace(string(out))) == 0, nil
}


