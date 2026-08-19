// Package integrate merges completed lane branches into a combined tree,
// runs verification checks against it, and promotes clean results into the
// primary repository's checked-out branch.
package integrate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/LanzerDevCorp/lucind-ai/internal/resolve"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

// ErrMergeConflict is returned by Combine when a branch cannot be merged
// cleanly into the combined tree.
var ErrMergeConflict = errors.New("integrate: merge conflict")

// ErrPrimaryRootDirty is returned by Promote when the primary repository
// root has uncommitted changes.
var ErrPrimaryRootDirty = errors.New("integrate: primary root has uncommitted changes")

// Combine creates a temporary linked worktree for runID off primaryRoot,
// merges each branch in branches in order via "git merge --no-ff", and
// returns the worktree path and branch name on success.
// If any merge fails, Combine aborts the merge, removes the worktree,
// deletes the created integration branch, and returns ErrMergeConflict
// wrapped with the failing branch name and git's output.
// The caller never has to clean up after a failed Combine call.
func Combine(ctx context.Context, primaryRoot, runID string, branches []string) (worktreePath, branchName string, err error) {
	return combine(ctx, primaryRoot, runID, branches, resolve.RealInvoker)
}

func combine(ctx context.Context, primaryRoot, runID string, branches []string, invoke resolve.Invoker) (worktreePath, branchName string, err error) {
	wt, err := worktree.Create(ctx, primaryRoot, "integrate-"+runID)
	if err != nil {
		return "", "", fmt.Errorf("integrate: combine create worktree: %w", err)
	}

	for _, branch := range branches {
		cmd := exec.CommandContext(ctx, "git", "merge", "--no-ff", branch)
		cmd.Dir = wt.Path
		out, mergeErr := cmd.CombinedOutput()
		if mergeErr != nil {
			resolved, _, resolveErr := resolve.Resolve(ctx, wt.Path, invoke)
			if resolveErr == nil && resolved {
				continue
			}

			abortCmd := exec.CommandContext(ctx, "git", "merge", "--abort")
			abortCmd.Dir = wt.Path
			_ = abortCmd.Run()

			_ = worktree.Remove(ctx, primaryRoot, wt.Path)
			_ = worktree.DeleteBranch(ctx, primaryRoot, wt.Branch)

			errMsg := strings.TrimSpace(string(out))
			if errMsg == "" {
				errMsg = mergeErr.Error()
			}
			return "", "", fmt.Errorf("%w: branch %s: %s", ErrMergeConflict, branch, errMsg)
		}
	}

	return wt.Path, wt.Branch, nil
}

// Check runs the project's verification suite against worktreePath.
// It first runs "CGO_ENABLED=0 go build ./...". If that fails, it returns
// passed=false, output=combined build output, and err=nil.
// If the build succeeds, it runs "go test ./... -race -count=1".
// If the test fails, it returns passed=false, output=combined test output, and err=nil.
// If both succeed, it returns passed=true, output=combined test output, and err=nil.
// Non-nil err is reserved for command execution failures (e.g. executable not found).
func Check(ctx context.Context, worktreePath string) (passed bool, output string, err error) {
	buildCmd := exec.CommandContext(ctx, "go", "build", "./...")
	buildCmd.Dir = worktreePath
	buildCmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	buildOut, buildErr := buildCmd.CombinedOutput()
	if buildErr != nil {
		var exitErr *exec.ExitError
		if errors.As(buildErr, &exitErr) {
			return false, string(buildOut), nil
		}
		return false, string(buildOut), fmt.Errorf("integrate: go build failed to execute: %w", buildErr)
	}

	testCmd := exec.CommandContext(ctx, "go", "test", "./...", "-race", "-count=1")
	testCmd.Dir = worktreePath
	testOut, testErr := testCmd.CombinedOutput()
	if testErr != nil {
		var exitErr *exec.ExitError
		if errors.As(testErr, &exitErr) {
			return false, string(testOut), nil
		}
		return false, string(testOut), fmt.Errorf("integrate: go test failed to execute: %w", testErr)
	}

	return true, string(testOut), nil
}

// Promote fast-forwards primaryRoot to integrationBranch if primaryRoot
// is clean. If primaryRoot has uncommitted changes according to
// "git status --porcelain", Promote returns ErrPrimaryRootDirty without
// executing any merge.
func Promote(ctx context.Context, primaryRoot, integrationBranch string) error {
	statusCmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	statusCmd.Dir = primaryRoot
	var statusStderr strings.Builder
	statusCmd.Stderr = &statusStderr
	statusOut, err := statusCmd.Output()
	if err != nil {
		return fmt.Errorf("integrate: git status failed: %w: %s", err, strings.TrimSpace(statusStderr.String()))
	}
	if len(strings.TrimSpace(string(statusOut))) > 0 {
		return ErrPrimaryRootDirty
	}

	mergeCmd := exec.CommandContext(ctx, "git", "merge", "--ff-only", integrationBranch)
	mergeCmd.Dir = primaryRoot
	var mergeStderr strings.Builder
	mergeCmd.Stderr = &mergeStderr
	if err := mergeCmd.Run(); err != nil {
		return fmt.Errorf("integrate: git merge --ff-only failed: %w: %s", err, strings.TrimSpace(mergeStderr.String()))
	}

	return nil
}
