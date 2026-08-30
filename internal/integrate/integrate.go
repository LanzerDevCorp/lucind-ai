// Package integrate merges completed lane branches into a combined tree,
// runs verification checks against it, and promotes clean results into the
// primary repository's checked-out branch.
package integrate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/resolve"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

const (
	checkPolicyVersion    = "lucind-check:v1"
	defaultCheckTimeout   = 20 * time.Minute
	checkTerminationGrace = 250 * time.Millisecond
	maxCheckOutput        = 64 << 10
)

var checkEnvironmentKeys = []string{
	"PATH", "HOME", "TMPDIR", "XDG_CACHE_HOME", "GOCACHE", "GOMODCACHE", "GOPATH",
	"GOENV", "GOFLAGS", "GOTOOLCHAIN", "CGO_ENABLED", "CC", "CXX", "LANG", "LC_ALL", "TZ",
}

// ErrMergeConflict is returned by Combine when a branch cannot be merged
// cleanly into the combined tree.
var ErrMergeConflict = errors.New("integrate: merge conflict")

// ErrPrimaryRootDirty is returned by Promote when the primary repository
// root has uncommitted changes.
var ErrPrimaryRootDirty = errors.New("integrate: primary root has uncommitted changes")

// ErrStaleCAS is returned by PromoteCAS when the parent ref's current SHA
// does not match the expected SHA (compare-and-swap mismatch).
var ErrStaleCAS = errors.New("integrate: stale expected sha in cas promotion")

// ErrInvalidParentRef is returned by PromoteCAS when the parent ref is invalid,
// equals main, or belongs to a Lucind temporary branch namespace.
var ErrInvalidParentRef = errors.New("integrate: invalid parent ref")

// ErrEmptySHA is returned by PromoteCAS when candidateSHA or expectedSHA is empty.
var ErrEmptySHA = errors.New("integrate: candidate sha and expected sha must not be empty")

// Combine creates a temporary linked worktree for runID off primaryRoot,
// merges each branch in branches in order via "git merge --no-ff", and
// returns the worktree path and branch name on success.
// parentRef and baseSHA are the feature's declared target, and are empty
// for a legacy dispatch -- empty means "no start point", which branches
// the integration worktree from the primary checkout's current HEAD,
// exactly as before feature targets existed. A feature batch must instead
// start at its own base_sha, or the combined tree is built on top of
// whatever primaryRoot happens to have checked out rather than the
// feature's actual parent.
// If any merge fails, Combine aborts the merge, removes the worktree,
// deletes the created integration branch, and returns ErrMergeConflict
// wrapped with the failing branch name and git's output.
// The caller never has to clean up after a failed Combine call.
func Combine(ctx context.Context, primaryRoot, runID, parentRef, baseSHA string, branches []string) (worktreePath, branchName string, err error) {
	return combine(ctx, primaryRoot, runID, parentRef, baseSHA, branches, resolve.RealInvoker)
}

// combine is Combine's implementation, parameterized over an Invoker for
// conflict resolution. See Combine's doc comment for the meaning of
// parentRef and baseSHA.
func combine(ctx context.Context, primaryRoot, runID, parentRef, baseSHA string, branches []string, invoke resolve.Invoker) (worktreePath, branchName string, err error) {
	wt, err := worktree.CreateWithParent(ctx, primaryRoot, "integrate-"+runID, parentRef, baseSHA)
	if err != nil {
		return "", "", fmt.Errorf("integrate: combine create worktree: %w", err)
	}

	for _, branch := range branches {
		// A clean "git merge --no-ff" otherwise produces byte-identical trees
		// and parents across repeated retries of the exact same, unchanged
		// lane branches, but the resulting merge commit is still
		// non-deterministic across retries for two independent reasons:
		//
		//  1. Its author/committer dates default to the current wall-clock
		//     time, which differs on every invocation.
		//  2. Its auto-generated message is "Merge branch '<branch>' into
		//     <current-branch>" -- and the current branch here is always
		//     "integrate-"+runID (see worktree.CreateWithParent above),
		//     freshly named per invocation, so the message text itself
		//     differs on every retry even when merging the exact same
		//     branch.
		//
		// Either alone changes the merge commit's SHA even though nothing
		// about its content did, which defeats "lucind-ai integrate retry":
		// a reconciliation resolution registered against one retry's
		// candidate_sha can never carry forward to the next retry's
		// freshly-regenerated, unrelated candidate_sha. Pinning the dates to
		// branch's own tip commit date (immutable for a preserved "done"
		// lane branch) and the message to a fixed, runID-independent string
		// makes the merge commit -- and therefore candidate_sha --
		// deterministic across retries as long as the merged branches
		// themselves have not changed.
		cmd := exec.CommandContext(ctx, "git", "merge", "--no-ff", "-m", "Merge branch '"+branch+"'", branch)
		cmd.Dir = wt.Path
		if commitDate, dateErr := branchCommitDate(ctx, wt.Path, branch); dateErr == nil && commitDate != "" {
			cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+commitDate, "GIT_COMMITTER_DATE="+commitDate)
		}
		out, mergeErr := cmd.CombinedOutput()
		if mergeErr != nil {
			resolved, _, resolveErr := resolve.Resolve(ctx, wt.Path, invoke)
			if resolveErr == nil && resolved {
				continue
			}

			abortCmd := exec.CommandContext(ctx, "git", "merge", "--abort")
			abortCmd.Dir = wt.Path
			_ = abortCmd.Run()

			_ = worktree.Remove(ctx, primaryRoot, wt.Path, true)
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

// branchCommitDate returns branch's tip commit date in strict ISO 8601
// (git's %cI format), suitable for GIT_AUTHOR_DATE/GIT_COMMITTER_DATE. It
// is used to pin a merge commit's dates to a value derived from the merged
// content itself instead of the current wall-clock time, so that combining
// the exact same unchanged branches on a later retry reproduces the exact
// same merge commit SHA.
func branchCommitDate(ctx context.Context, worktreePath, branch string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "log", "-1", "--format=%cI", branch)
	cmd.Dir = worktreePath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("integrate: resolve commit date for branch %s: %w", branch, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Check runs the project's verification suite against worktreePath by
// executing lucind-checks.sh at the root of worktreePath via sh.
// If lucind-checks.sh does not exist, Check returns passed=false, an
// explanatory message, and err=nil.
// If the script exits non-zero, Check returns passed=false, the combined output, and err=nil.
// If the script exits zero, Check returns passed=true, the combined output, and err=nil.
// Non-nil err is reserved for command execution failures (e.g. cancelled context or command cannot be started).
func Check(ctx context.Context, worktreePath string) (passed bool, output string, err error) {
	if !filepath.IsAbs(worktreePath) {
		return false, "", errors.New("integrate: checks root must be absolute")
	}
	ownedRoot, err := filepath.EvalSymlinks(worktreePath)
	if err != nil {
		return false, "", fmt.Errorf("integrate: resolve checks root: %w", err)
	}
	ownedRoot, err = filepath.Abs(ownedRoot)
	if err != nil {
		return false, "", fmt.Errorf("integrate: canonicalize checks root: %w", err)
	}
	scriptPath := filepath.Join(ownedRoot, "lucind-checks.sh")
	if _, statErr := os.Stat(scriptPath); errors.Is(statErr, os.ErrNotExist) || os.IsNotExist(statErr) {
		return false, "no lucind-checks.sh found at the project root; this project has not defined its verification checks yet", nil
	} else if statErr != nil {
		return false, "", fmt.Errorf("integrate: stat checks script: %w", statErr)
	}

	env, envErr := checkEnvironment()
	if envErr != nil {
		return false, "", envErr
	}
	if ctx.Err() != nil {
		return false, "", fmt.Errorf("integrate: checks context ended before start: %w", ctx.Err())
	}
	checkCtx := ctx
	cancel := func() {}
	if _, ok := ctx.Deadline(); !ok {
		checkCtx, cancel = context.WithTimeout(ctx, defaultCheckTimeout)
	}
	defer cancel()

	cmd := exec.Command("sh", scriptPath)
	cmd.Dir = ownedRoot
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var out limitedBuffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if startErr := cmd.Start(); startErr != nil {
		return false, out.String(), fmt.Errorf("integrate: start checks script: %w", startErr)
	}
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()

	var cmdErr error
	select {
	case cmdErr = <-wait:
	case <-checkCtx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		timer := time.NewTimer(checkTerminationGrace)
		select {
		case cmdErr = <-wait:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			cmdErr = <-wait
		}
		return false, out.String(), nil
	}
	if cmdErr != nil {
		var exitErr *exec.ExitError
		if errors.As(cmdErr, &exitErr) {
			return false, out.String(), nil
		}
		return false, out.String(), fmt.Errorf("integrate: wait checks script: %w", cmdErr)
	}

	return true, out.String(), nil
}

// CheckPolicySnapshot returns the ordered inputs hashed into acceptance policy/environment identity.
func CheckPolicySnapshot() (string, time.Duration, []string, error) {
	env, err := checkEnvironment()
	return checkPolicyVersion, defaultCheckTimeout, env, err
}

func checkEnvironment() ([]string, error) {
	env := make([]string, 0, len(checkEnvironmentKeys))
	for _, key := range checkEnvironmentKeys {
		value, ok := os.LookupEnv(key)
		if key == "PATH" && (!ok || value == "") {
			return nil, errors.New("integrate: required check environment PATH is missing")
		}
		if ok {
			env = append(env, key+"="+value)
		}
	}
	return env, nil
}

type limitedBuffer struct {
	mu        sync.Mutex
	b         bytes.Buffer
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	written := len(p)
	remaining := maxCheckOutput - b.b.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
			b.truncated = true
		}
		_, _ = b.b.Write(p)
	} else {
		b.truncated = true
	}
	return written, nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := b.b.String()
	if b.truncated {
		out += "\n[output truncated]\n"
	}
	return out
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

// PromoteCAS atomically advances parentRef to candidateSHA in primaryRoot if and only if
// parentRef currently points to expectedSHA, using git update-ref.
// It does not check out, merge into, or otherwise mutate primaryRoot's working tree or HEAD branch.
func PromoteCAS(ctx context.Context, primaryRoot, parentRef, candidateSHA, expectedSHA string) error {
	return PromoteCASWithRunner(ctx, worktree.DefaultGitRunner, primaryRoot, parentRef, candidateSHA, expectedSHA)
}

// PromoteRef is an alias for PromoteCAS.
var PromoteRef = PromoteCAS

// PromoteCASWithRunner performs compare-and-swap promotion using the injected GitRunner.
func PromoteCASWithRunner(ctx context.Context, runner worktree.GitRunner, primaryRoot, parentRef, candidateSHA, expectedSHA string) error {
	if runner == nil {
		runner = worktree.DefaultGitRunner
	}

	if err := worktree.ValidateParentRef(ctx, runner, parentRef); err != nil {
		return ErrInvalidParentRef
	}

	candidate := strings.TrimSpace(candidateSHA)
	expected := strings.TrimSpace(expectedSHA)
	if candidate == "" || expected == "" {
		return ErrEmptySHA
	}

	canonicalRef := worktree.CanonicalizeRef(parentRef)

	if _, err := runner.Run(ctx, primaryRoot, "update-ref", canonicalRef, candidate, expected); err != nil {
		return fmt.Errorf("%w: %w", ErrStaleCAS, err)
	}

	return nil
}
