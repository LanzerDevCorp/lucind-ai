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
	"path/filepath"
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

// Check runs the project's verification suite against worktreePath by
// executing lucind-checks.sh at the root of worktreePath via sh.
// If lucind-checks.sh does not exist, Check returns passed=false, an
// explanatory message, and err=nil.
// If the script exits non-zero, Check returns passed=false, the combined output, and err=nil.
// If the script exits zero, Check returns passed=true, the combined output, and err=nil.
// Non-nil err is reserved for command execution failures (e.g. cancelled context or command cannot be started).
func Check(ctx context.Context, worktreePath string) (passed bool, output string, err error) {
	scriptPath := filepath.Join(worktreePath, "lucind-checks.sh")
	if _, statErr := os.Stat(scriptPath); errors.Is(statErr, os.ErrNotExist) || os.IsNotExist(statErr) {
		return false, "no lucind-checks.sh found at the project root; this project has not defined its verification checks yet", nil
	} else if statErr != nil {
		return false, "", fmt.Errorf("integrate: stat checks script: %w", statErr)
	}

	cmd := exec.CommandContext(ctx, "sh", scriptPath)
	cmd.Dir = worktreePath
	out, cmdErr := cmd.CombinedOutput()
	if cmdErr != nil {
		var exitErr *exec.ExitError
		if errors.As(cmdErr, &exitErr) {
			return false, string(out), nil
		}
		return false, string(out), fmt.Errorf("integrate: run checks script failed to execute: %w", cmdErr)
	}

	return true, string(out), nil
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
