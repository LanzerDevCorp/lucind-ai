// Package repo owns every git call about the repository and its trees.
package repo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Toplevel returns the absolute git repository top-level directory for dir.
func Toplevel(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	toplevel := strings.TrimRight(string(out), "\r\n")
	if !filepath.IsAbs(toplevel) {
		abs, err := filepath.Abs(toplevel)
		if err != nil {
			return "", fmt.Errorf("resolve repo toplevel path %q: %w", toplevel, err)
		}
		toplevel = abs
	}
	return filepath.Clean(toplevel), nil
}

// CommonDir returns the absolute git common directory for dir.
// For a primary repository, this is the .git directory.
// For a linked worktree, this is the main repository's .git directory.
func CommonDir(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--git-common-dir")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --git-common-dir: %w", err)
	}
	commonDir := strings.TrimRight(string(out), "\r\n")
	if !filepath.IsAbs(commonDir) {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return "", fmt.Errorf("resolve dir path %q: %w", dir, err)
		}
		// git resolves the relative path against the real directory, not a symlinked alias.
		realDir, err := filepath.EvalSymlinks(absDir)
		if err != nil {
			return "", fmt.Errorf("resolve symlinks for %q: %w", absDir, err)
		}
		commonDir = filepath.Join(realDir, commonDir)
	}
	return filepath.Clean(commonDir), nil
}

// HeadSHA returns the commit SHA that HEAD points to in the repository containing dir.
func HeadSHA(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// TreeHash computes a git tree hash of the current working tree including uncommitted
// and untracked, non-ignored files, without touching the real git index.
func TreeHash(ctx context.Context, repoRoot string) (string, error) {
	tmpFile, err := os.CreateTemp("", "lucind-attest-index-*")
	if err != nil {
		return "", fmt.Errorf("create temp index: %w", err)
	}
	tmpIndexPath := tmpFile.Name()
	_ = tmpFile.Close()
	_ = os.Remove(tmpIndexPath)
	defer func() { _ = os.Remove(tmpIndexPath) }()

	env := append(os.Environ(), "GIT_INDEX_FILE="+tmpIndexPath)

	// Seed temporary index with git read-tree HEAD if HEAD exists
	checkHead := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "--verify", "HEAD")
	if err := checkHead.Run(); err == nil {
		cmdRead := exec.CommandContext(ctx, "git", "-C", repoRoot, "read-tree", "HEAD")
		cmdRead.Env = env
		if out, err := cmdRead.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git read-tree HEAD: %w: %s", err, strings.TrimSpace(string(out)))
		}
	}

	cmdAdd := exec.CommandContext(ctx, "git", "-C", repoRoot, "add", "-A")
	cmdAdd.Env = env
	if out, err := cmdAdd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add -A: %w: %s", err, strings.TrimSpace(string(out)))
	}

	cmdWrite := exec.CommandContext(ctx, "git", "-C", repoRoot, "write-tree")
	cmdWrite.Env = env
	out, err := cmdWrite.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git write-tree: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// ChangedFiles lists the paths that differ between the trees base and final.
// It returns an empty, non-nil slice when nothing changed.
func ChangedFiles(ctx context.Context, repoRoot, base, final string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "diff", "--name-only", base, final)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git diff %s..%s: %w: %s", base, final, err, strings.TrimSpace(string(out)))
	}
	files := []string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}
