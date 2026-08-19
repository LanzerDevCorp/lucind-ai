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

// worktreesDirSuffix names the sibling directory that holds every lane's
// linked worktree for a given primary repository.
const worktreesDirSuffix = "-worktrees"

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
	Path   string // absolute path to the worktree directory
	Branch string // the branch checked out in it
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

	branch := "lucind/" + laneID

	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", branch, path)
	cmd.Dir = primaryRoot
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Worktree{}, fmt.Errorf("worktree: git worktree add failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return Worktree{Path: path, Branch: branch}, nil
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
