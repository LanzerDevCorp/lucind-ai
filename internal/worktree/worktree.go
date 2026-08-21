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

// ErrInvalidParentRef is returned when parentRef is empty, equals main,
// resides in a Lucind temporary branch namespace, or fails git check-ref-format.
var ErrInvalidParentRef = errors.New("worktree: invalid parent ref")

// ErrInvalidBaseSHA is returned when baseSHA does not resolve to a valid commit
// or fails ancestry validation.
var ErrInvalidBaseSHA = errors.New("worktree: invalid base SHA")

// GitRunner is an interface for running git commands in a specified working directory.
type GitRunner interface {
	Run(ctx context.Context, dir string, args ...string) ([]byte, error)
}

type execGitRunner struct{}

func (execGitRunner) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		errStr := strings.TrimSpace(stderr.String())
		if errStr != "" {
			return out, fmt.Errorf("%w: %s", err, errStr)
		}
		return out, err
	}
	return out, nil
}

// DefaultGitRunner executes git commands directly via exec.CommandContext.
var DefaultGitRunner GitRunner = execGitRunner{}

// worktreesDirSuffix names the sibling directory that holds every lane's
// linked worktree for a given primary repository.
const worktreesDirSuffix = "-worktrees"

// BranchFor returns the branch name for a lane: "lucind/" + laneID.
func BranchFor(laneID string) string {
	return "lucind/" + laneID
}

// CanonicalizeRef normalizes a branch or ref name to have a "refs/heads/" prefix.
func CanonicalizeRef(ref string) string {
	trimmed := strings.TrimSpace(ref)
	if strings.HasPrefix(trimmed, "refs/heads/") {
		return trimmed
	}
	return "refs/heads/" + trimmed
}

// ValidateParentRef validates a parent ref string per Git and Overlap Contracts.
// It verifies that ref is non-empty, not "main" or "refs/heads/main", not in the
// "lucind/*" or "refs/heads/lucind/*" namespace, and satisfies git check-ref-format --branch.
func ValidateParentRef(ctx context.Context, runner GitRunner, parentRef string) error {
	return validateParentRef(ctx, runner, parentRef)
}

func validateParentRef(ctx context.Context, runner GitRunner, parentRef string) error {
	trimmed := strings.TrimSpace(parentRef)
	if trimmed == "" {
		return ErrInvalidParentRef
	}
	if trimmed == "main" || trimmed == "refs/heads/main" {
		return ErrInvalidParentRef
	}
	if trimmed == "lucind" || trimmed == "refs/heads/lucind" ||
		strings.HasPrefix(trimmed, "lucind/") || strings.HasPrefix(trimmed, "refs/heads/lucind/") {
		return ErrInvalidParentRef
	}

	if runner == nil {
		runner = DefaultGitRunner
	}

	// git check-ref-format --branch checks that the ref is a valid branch name.
	if _, err := runner.Run(ctx, "", "check-ref-format", "--branch", trimmed); err != nil {
		return ErrInvalidParentRef
	}

	return nil
}

// ResolveCommitSHA resolves a revision specifier to a full commit SHA in primaryRoot
// by validating that rev^{commit} exists and resolves to a commit.
func ResolveCommitSHA(ctx context.Context, runner GitRunner, primaryRoot, rev string) (string, error) {
	return resolveCommitSHA(ctx, runner, primaryRoot, rev)
}

func resolveCommitSHA(ctx context.Context, runner GitRunner, primaryRoot, rev string) (string, error) {
	trimmed := strings.TrimSpace(rev)
	if trimmed == "" {
		return "", ErrInvalidBaseSHA
	}
	if runner == nil {
		runner = DefaultGitRunner
	}

	out, err := runner.Run(ctx, primaryRoot, "rev-parse", "--verify", trimmed+"^{commit}")
	if err != nil {
		return "", ErrInvalidBaseSHA
	}
	sha := strings.TrimSpace(string(out))
	if sha == "" {
		return "", ErrInvalidBaseSHA
	}
	return sha, nil
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

// Create adds a linked git worktree for laneID off primaryRoot using the current primaryRoot checkout.
func Create(ctx context.Context, primaryRoot, laneID string) (Worktree, error) {
	return createWithRunner(ctx, DefaultGitRunner, primaryRoot, laneID, "", "")
}

// CreateWithParent adds a linked git worktree for laneID off primaryRoot starting at baseSHA
// after validating parentRef and baseSHA per Git and Overlap Contracts.
func CreateWithParent(ctx context.Context, primaryRoot, laneID, parentRef, baseSHA string) (Worktree, error) {
	return createWithRunner(ctx, DefaultGitRunner, primaryRoot, laneID, parentRef, baseSHA)
}

// CreateWithRunner creates a linked worktree using the provided GitRunner.
func CreateWithRunner(ctx context.Context, runner GitRunner, primaryRoot, laneID, parentRef, baseSHA string) (Worktree, error) {
	return createWithRunner(ctx, runner, primaryRoot, laneID, parentRef, baseSHA)
}

func createWithRunner(ctx context.Context, runner GitRunner, primaryRoot, laneID, parentRef, baseSHA string) (Worktree, error) {
	if laneID == "" {
		return Worktree{}, ErrEmptyLaneID
	}
	if runner == nil {
		runner = DefaultGitRunner
	}

	var resolvedSHA string
	if parentRef != "" || baseSHA != "" {
		if err := validateParentRef(ctx, runner, parentRef); err != nil {
			return Worktree{}, err
		}
		var err error
		resolvedSHA, err = resolveCommitSHA(ctx, runner, primaryRoot, baseSHA)
		if err != nil {
			return Worktree{}, err
		}

		// If parentRef already exists in primaryRoot, verify that resolvedSHA is an ancestor of parentRef.
		canonicalParent := CanonicalizeRef(parentRef)
		if _, err := runner.Run(ctx, primaryRoot, "rev-parse", "--verify", canonicalParent+"^{commit}"); err == nil {
			if _, err := runner.Run(ctx, primaryRoot, "merge-base", "--is-ancestor", resolvedSHA, canonicalParent); err != nil {
				return Worktree{}, ErrInvalidBaseSHA
			}
		}
	}

	path := pathFor(primaryRoot, laneID)
	if _, err := os.Stat(path); err == nil {
		return Worktree{}, ErrWorktreeExists
	}

	branch := BranchFor(laneID)

	var addArgs []string
	if resolvedSHA != "" {
		addArgs = []string{"worktree", "add", "-b", branch, path, resolvedSHA}
	} else {
		addArgs = []string{"worktree", "add", "-b", branch, path}
	}

	if _, err := runner.Run(ctx, primaryRoot, addArgs...); err != nil {
		return Worktree{}, fmt.Errorf("worktree: git worktree add failed: %w", err)
	}

	headOut, err := runner.Run(ctx, path, "rev-parse", "HEAD")
	if err != nil {
		_ = Remove(ctx, primaryRoot, path)
		return Worktree{}, fmt.Errorf("worktree: git rev-parse HEAD failed: %w", err)
	}
	recordedSHA := strings.TrimSpace(string(headOut))

	return Worktree{Path: path, Branch: branch, BaseSHA: recordedSHA}, nil
}

// Remove removes a linked git worktree at path off primaryRoot.
func Remove(ctx context.Context, primaryRoot, path string) error {
	if _, err := DefaultGitRunner.Run(ctx, primaryRoot, "worktree", "remove", "--force", path); err != nil {
		return fmt.Errorf("worktree: git worktree remove failed: %w", err)
	}
	return nil
}

// DeleteBranch forces deletion of branch in primaryRoot.
func DeleteBranch(ctx context.Context, primaryRoot, branch string) error {
	if _, err := DefaultGitRunner.Run(ctx, primaryRoot, "branch", "-D", branch); err != nil {
		return fmt.Errorf("worktree: git branch -D failed: %w", err)
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

	wtHeadOut, err := DefaultGitRunner.Run(ctx, worktreePath, "rev-parse", "HEAD")
	if err != nil {
		return false, fmt.Errorf("worktree: git rev-parse HEAD in worktree failed: %w", err)
	}
	wtHead := strings.TrimSpace(string(wtHeadOut))

	mergeBaseOut, err := DefaultGitRunner.Run(ctx, worktreePath, "merge-base", "HEAD", baseSHA)
	if err != nil {
		return false, fmt.Errorf("worktree: git merge-base failed: %w", err)
	}
	mergeBase := strings.TrimSpace(string(mergeBaseOut))

	return wtHead != mergeBase, nil
}

// PorcelainEmpty reports whether git status --porcelain in the linked worktree
// at worktreePath produces no output, using git's default ignore rules.
func PorcelainEmpty(ctx context.Context, worktreePath string) (bool, error) {
	out, err := DefaultGitRunner.Run(ctx, worktreePath, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("worktree: git status failed: %w", err)
	}
	return len(strings.TrimSpace(string(out))) == 0, nil
}



