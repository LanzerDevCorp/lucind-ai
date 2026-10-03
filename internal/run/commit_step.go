package run

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/LanzerDevCorp/lucind-ai/internal/attest"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
)

// dispatcherCommit performs the verification under attestation and Conventional Commit
// on behalf of the worker for packets declaring commit_message.
func dispatcherCommit(ctx context.Context, deps Deps, worktreePath, baseSHA string, p packet.Packet) (lane.Status, string) {
	// a. The worker must not have committed: worktree HEAD must equal baseSHA
	headSHA, err := resolveWorktreeHead(ctx, worktreePath)
	if err != nil {
		return lane.Blocked, fmt.Sprintf("resolve worktree HEAD: %v", err)
	}
	if headSHA != baseSHA {
		return lane.Deviated, "worker committed in a dispatcher-commit packet"
	}

	// b. There must be something to commit (non-empty change set)
	hasChanges, err := checkWorktreeChanges(ctx, worktreePath)
	if err != nil {
		return lane.Blocked, fmt.Sprintf("check worktree changes: %v", err)
	}
	if !hasChanges {
		return lane.Failed, "nothing to commit"
	}

	// c. Verification under attestation. The tree must be identical before and after
	// the verification commands: a command that edits files would otherwise get its
	// own side effects attested and committed.
	treeBefore, err := attest.TreeHash(ctx, worktreePath)
	if err != nil {
		return lane.Blocked, fmt.Sprintf("compute tree hash before verification: %v", err)
	}
	runAttested := deps.RunAttested
	if runAttested == nil {
		runAttested = defaultRunAttested
	}
	for _, cmd := range p.Verification {
		exitCode, err := runAttested(ctx, worktreePath, cmd)
		if err != nil {
			return lane.Blocked, fmt.Sprintf("run verification %s: %v", cmd, err)
		}
		if exitCode != 0 {
			return lane.Failed, fmt.Sprintf("verification failed: %s (exit %d)", cmd, exitCode)
		}
	}

	treeHash, err := attest.TreeHash(ctx, worktreePath)
	if err != nil {
		return lane.Blocked, fmt.Sprintf("compute tree hash: %v", err)
	}
	if treeHash != treeBefore {
		return lane.Failed, "verification changed the worktree (tree hash differs before and after); verification commands must not modify files"
	}

	hasValidAttestation := deps.HasValidAttestation
	if hasValidAttestation == nil {
		hasValidAttestation = defaultHasValidAttestation
	}
	for _, cmd := range p.Verification {
		valid, err := hasValidAttestation(ctx, worktreePath, cmd, treeHash)
		if err != nil {
			return lane.Blocked, fmt.Sprintf("check attestation for %s: %v", cmd, err)
		}
		if !valid {
			return lane.Blocked, fmt.Sprintf("missing attestation for: %s", cmd)
		}
	}

	// Optional pre-commit gate (e.g. judges in T10)
	if deps.PreCommitGate != nil {
		gateStatus, gateReason := deps.PreCommitGate(ctx, worktreePath, p)
		if gateStatus != lane.Done {
			return gateStatus, gateReason
		}
	}

	// d. Commit
	gitCommit := deps.GitCommit
	if gitCommit == nil {
		gitCommit = defaultGitCommit
	}
	if err := gitCommit(ctx, worktreePath, p.CommitMessage); err != nil {
		return lane.Blocked, fmt.Sprintf("git commit: %v", err)
	}

	// e. Success
	return lane.Done, ""
}

func defaultRunAttested(ctx context.Context, worktreePath, cmd string) (int, error) {
	entry, err := attest.RunAndRecord(ctx, worktreePath, []string{"sh", "-c", cmd}, cmd, nil, os.Stderr, os.Stderr)
	if err != nil {
		return 0, err
	}
	return entry.ExitCode, nil
}

func defaultHasValidAttestation(ctx context.Context, repoRoot, cmd, expectedTreeHash string) (bool, error) {
	return attest.HasValidAttestation(ctx, repoRoot, cmd, expectedTreeHash, nil, "")
}

func resolveWorktreeHead(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func checkWorktreeChanges(ctx context.Context, dir string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "status", "--porcelain", "--", ".", ":(exclude).lucind")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("git status: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}

func defaultGitCommit(ctx context.Context, worktreePath, message string) error {
	// Stage everything, then unstage the .lucind result files. `git add` with an
	// explicit exclude pathspec errors when .lucind is gitignored, so unstaging
	// afterwards works for both the ignored and the untracked case.
	cmdAdd := exec.CommandContext(ctx, "git", "-C", worktreePath, "add", "-A")
	if out, err := cmdAdd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %w: %s", err, strings.TrimSpace(string(out)))
	}
	cmdUnstage := exec.CommandContext(ctx, "git", "-C", worktreePath, "reset", "-q", "--", ".lucind")
	if out, err := cmdUnstage.CombinedOutput(); err != nil {
		return fmt.Errorf("git reset .lucind: %w: %s", err, strings.TrimSpace(string(out)))
	}

	var args []string
	args = append(args, "-C", worktreePath)
	if !hasConfiguredGitIdentity(ctx, worktreePath) {
		args = append(args, "-c", "user.name=lucind-ai", "-c", "user.email=lucind-ai@localhost")
	}
	// --no-verify: the attested verification above replaces repository hooks, and a
	// worker-controlled hook path (or a commit-msg hook adding trailers) must not run
	// with the dispatcher's authority.
	args = append(args, "commit", "--no-verify", "-m", message)

	cmdCommit := exec.CommandContext(ctx, "git", args...)
	if out, err := cmdCommit.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func hasConfiguredGitIdentity(ctx context.Context, dir string) bool {
	nameCmd := exec.CommandContext(ctx, "git", "-C", dir, "config", "user.name")
	nameOut, nameErr := nameCmd.Output()
	if nameErr != nil || strings.TrimSpace(string(nameOut)) == "" {
		return false
	}
	emailCmd := exec.CommandContext(ctx, "git", "-C", dir, "config", "user.email")
	emailOut, emailErr := emailCmd.Output()
	if emailErr != nil || strings.TrimSpace(string(emailOut)) == "" {
		return false
	}
	return true
}
