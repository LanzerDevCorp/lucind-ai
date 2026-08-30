package run_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/integrate"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/run"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

// realIsolationDeps wires run.Deps to the REAL git-backed integrate/worktree
// primitives -- the same functions cmd/lucind-ai/cli.go's productionDeps
// wires in production -- instead of the spies every other test in this
// package uses. This is deliberate: every other attempt_test.go test proves
// that ExecuteAttempt *passes* req.ParentRef/req.BaseSHA through to
// CombineTree, but a spy that just echoes back whatever it was given can
// never catch a bug in what CombineTree's real implementation *does* with
// those values (internal/integrate/integrate.go's combine, which is the
// function that actually decides whether the combined worktree starts at the
// feature's parent or at primaryRoot's current checkout). Only real git,
// exercised end-to-end, proves candidate isolation.
func realIsolationDeps(t *testing.T, primaryRoot string, now time.Time) run.Deps {
	t.Helper()

	l, err := ledger.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("ledger.Open() error = %v", err)
	}
	t.Cleanup(func() { l.Close() })

	return run.Deps{
		PrimaryRoot: primaryRoot,
		Ledger:      l,
		Now:         func() time.Time { return now },
		CombineTree: integrate.Combine,
		RunChecks: func(ctx context.Context, worktreePath string) (bool, string, error) {
			return true, "ok", nil
		},
		PromoteCAS: integrate.PromoteCAS,
		ResolveRefSHA: func(ctx context.Context, primaryRoot, ref string) (string, error) {
			return worktree.ResolveCommitSHA(ctx, worktree.DefaultGitRunner, primaryRoot, worktree.CanonicalizeRef(ref))
		},
		ResolveCandidateSHA: func(ctx context.Context, primaryRoot, worktreePath, branch string) (string, error) {
			return worktree.ResolveCommitSHA(ctx, worktree.DefaultGitRunner, worktreePath, "HEAD")
		},
		DiscardCombined: func(ctx context.Context, primaryRoot, path, branch string) error {
			if err := worktree.Remove(ctx, primaryRoot, path, true); err != nil {
				return err
			}
			return worktree.DeleteBranch(ctx, primaryRoot, branch)
		},
		FeatureLeaseTTL: 30 * time.Second,
	}
}

func writeAndCommit(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", name, err)
	}
	runGit(t, dir, "add", name)
	runGit(t, dir, "commit", "-m", msg)
}

// integrateWorktreePath replicates internal/worktree's unexported pathFor
// convention (sibling "<primary>-worktrees/<laneID>" directory) so the test
// can locate and clean up a combined worktree that a blocked/stale attempt
// deliberately leaves on disk as preserved evidence.
func integrateWorktreePath(primaryRoot, attemptID string) string {
	parent := filepath.Dir(primaryRoot)
	base := filepath.Base(primaryRoot)
	return filepath.Join(parent, base+"-worktrees", "integrate-"+attemptID)
}

// currentBranch returns primaryRoot's current checked-out branch name.
// initGitRepo (run_test.go) does not pin the initial branch name, so tests
// that need to return to the seed branch must discover it rather than
// hardcode "main" (this repo's own git config defaults to "master").
func currentBranch(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse --abbrev-ref HEAD error = %v", err)
	}
	return strings.TrimSpace(string(out))
}

func treeContains(t *testing.T, primaryRoot, commitSHA, path string) bool {
	t.Helper()
	out, err := exec.Command("git", "-C", primaryRoot, "ls-tree", "-r", "--name-only", commitSHA).Output()
	if err != nil {
		t.Fatalf("git ls-tree error = %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if line == path {
			return true
		}
	}
	return false
}

// TestFeatureAttemptCandidateIsolatedFromUnrelatedPrimaryCheckout proves,
// against real git, that a feature-targeted integration candidate is built
// solely from the feature's declared immutable parent revision -- never from
// whatever the primary checkout happens to have checked out -- and that two
// features with different parents build and promote independently.
//
// Regression target: internal/integrate/integrate.go's combine used to call
// worktree.Create(primaryRoot, "integrate-"+runID), which branches the
// combined worktree from primaryRoot's *current checkout* rather than the
// feature's declared parent. An unrelated branch checked out in the primary
// workspace (simulated here as "change-c") would then leak into every
// feature's combined candidate.
func TestFeatureAttemptCandidateIsolatedFromUnrelatedPrimaryCheckout(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initGitRepo(t)
	seedBranch := currentBranch(t, primaryRoot)

	// An unrelated Change C, currently checked out in the primary workspace
	// for the remainder of this test -- exactly the scenario the fix targets.
	runGit(t, primaryRoot, "checkout", "-b", "change-c")
	writeAndCommit(t, primaryRoot, "c_only.txt", "change C\n", "change C commit")

	// Change A's declared immutable parent and lane.
	runGit(t, primaryRoot, "checkout", seedBranch)
	runGit(t, primaryRoot, "checkout", "-b", "parent-a")
	writeAndCommit(t, primaryRoot, "a_parent.txt", "parent A\n", "parent A commit")
	baseA := gitRevParse(t, primaryRoot, "HEAD")

	runGit(t, primaryRoot, "checkout", "-b", "lucind/lane-a1")
	writeAndCommit(t, primaryRoot, "a_lane1.txt", "lane a1\n", "lane a1 commit")

	// Change B's declared immutable parent and lane -- a sibling of A's
	// parent, not an ancestor of it or of change-c.
	runGit(t, primaryRoot, "checkout", seedBranch)
	runGit(t, primaryRoot, "checkout", "-b", "parent-b")
	writeAndCommit(t, primaryRoot, "b_parent.txt", "parent B\n", "parent B commit")
	baseB := gitRevParse(t, primaryRoot, "HEAD")

	runGit(t, primaryRoot, "checkout", "-b", "lucind/lane-b1")
	writeAndCommit(t, primaryRoot, "b_lane1.txt", "lane b1\n", "lane b1 commit")

	// A second lane for A, used later to prove a stale target still fails.
	runGit(t, primaryRoot, "checkout", "parent-a")
	runGit(t, primaryRoot, "checkout", "-b", "lucind/lane-a2")
	writeAndCommit(t, primaryRoot, "a_lane2.txt", "lane a2\n", "lane a2 commit")

	// Leave the primary checkout on the unrelated change for every attempt
	// below -- nothing in this test ever checks it back out.
	runGit(t, primaryRoot, "checkout", "change-c")
	primaryCheckoutBefore := gitRevParse(t, primaryRoot, "HEAD")

	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

	runAttempt := func(t *testing.T, featureID, parentRef, baseSHA, expectedParentSHA, attemptID string, branches []string) run.Attempt {
		t.Helper()
		deps := realIsolationDeps(t, primaryRoot, now)
		req := run.AttemptRequest{
			ID:                attemptID,
			FeatureID:         featureID,
			ParentRef:         parentRef,
			BaseSHA:           baseSHA,
			ExpectedParentSHA: expectedParentSHA,
			IdempotencyKey:    "idem-" + attemptID,
			Owner:             "test-owner",
			Branches:          branches,
		}
		att, err := run.ExecuteAttempt(context.Background(), deps, req)
		if err != nil {
			t.Fatalf("ExecuteAttempt(%s) error = %v", attemptID, err)
		}
		return att
	}

	// Change A's attempt: combined candidate must be parent A + lane a1,
	// nothing else.
	attA1 := runAttempt(t, "feat-a", "refs/heads/parent-a", baseA, baseA, "att-a-1", []string{"lucind/lane-a1"})
	if attA1.Status != run.AttemptStatusPromoted {
		t.Fatalf("attA1.Status = %v, want %v (reason: %s)", attA1.Status, run.AttemptStatusPromoted, attA1.FailureReason)
	}

	// Change B's attempt, built and promoted independently.
	attB1 := runAttempt(t, "feat-b", "refs/heads/parent-b", baseB, baseB, "att-b-1", []string{"lucind/lane-b1"})
	if attB1.Status != run.AttemptStatusPromoted {
		t.Fatalf("attB1.Status = %v, want %v (reason: %s)", attB1.Status, run.AttemptStatusPromoted, attB1.FailureReason)
	}

	newParentA := gitRevParse(t, primaryRoot, "refs/heads/parent-a")
	newParentB := gitRevParse(t, primaryRoot, "refs/heads/parent-b")

	if newParentA != attA1.CandidateSHA {
		t.Errorf("refs/heads/parent-a tip = %s, want promoted candidate %s", newParentA, attA1.CandidateSHA)
	}
	if newParentB != attB1.CandidateSHA {
		t.Errorf("refs/heads/parent-b tip = %s, want promoted candidate %s", newParentB, attB1.CandidateSHA)
	}

	// Change A's candidate contains parent A + lane a1, and nothing from B
	// or from the unrelated primary checkout.
	for _, want := range []string{"README.md", "a_parent.txt", "a_lane1.txt"} {
		if !treeContains(t, primaryRoot, newParentA, want) {
			t.Errorf("parent-a candidate missing %q", want)
		}
	}
	for _, unwanted := range []string{"b_parent.txt", "b_lane1.txt", "c_only.txt", "a_lane2.txt"} {
		if treeContains(t, primaryRoot, newParentA, unwanted) {
			t.Errorf("parent-a candidate contains %q, want absent (contamination)", unwanted)
		}
	}

	// Change B's candidate contains parent B + lane b1, and nothing from A
	// or from the unrelated primary checkout.
	for _, want := range []string{"README.md", "b_parent.txt", "b_lane1.txt"} {
		if !treeContains(t, primaryRoot, newParentB, want) {
			t.Errorf("parent-b candidate missing %q", want)
		}
	}
	for _, unwanted := range []string{"a_parent.txt", "a_lane1.txt", "c_only.txt", "a_lane2.txt"} {
		if treeContains(t, primaryRoot, newParentB, unwanted) {
			t.Errorf("parent-b candidate contains %q, want absent (contamination)", unwanted)
		}
	}

	// Candidate construction never checked out or mutated the primary
	// workspace: it is still sitting exactly where the test left it, on the
	// unrelated change.
	if got := gitRevParse(t, primaryRoot, "HEAD"); got != primaryCheckoutBefore {
		t.Errorf("primary checkout HEAD = %s, want unchanged %s", got, primaryCheckoutBefore)
	}
	if got := currentBranch(t, primaryRoot); got != "change-c" {
		t.Errorf("primary checkout branch = %q, want %q (never switched)", got, "change-c")
	}
	if _, err := os.Stat(filepath.Join(primaryRoot, "a_parent.txt")); !os.IsNotExist(err) {
		t.Errorf("primary working tree contains a_parent.txt, want absent: worktree add must not touch it")
	}

	// A stale target still fails through the existing CAS path: parent-a has
	// moved (attA1 promoted past baseA), so a second attempt that still
	// expects the old baseA must be rejected, and the ref left untouched.
	attA2 := runAttempt(t, "feat-a", "refs/heads/parent-a", baseA, baseA, "att-a-2", []string{"lucind/lane-a2"})
	if attA2.Status != run.AttemptStatusStale {
		t.Fatalf("attA2.Status = %v, want %v (reason: %s)", attA2.Status, run.AttemptStatusStale, attA2.FailureReason)
	}
	if !strings.Contains(attA2.FailureReason, "stale") {
		t.Errorf("attA2.FailureReason = %q, want it to mention staleness", attA2.FailureReason)
	}
	if got := gitRevParse(t, primaryRoot, "refs/heads/parent-a"); got != newParentA {
		t.Errorf("refs/heads/parent-a tip = %s after stale attempt, want unchanged %s", got, newParentA)
	}

	// The stale attempt's combined worktree is preserved for inspection
	// rather than silently discarded -- clean it up so the test leaves
	// nothing behind.
	leftoverPath := integrateWorktreePath(primaryRoot, "att-a-2")
	if _, err := os.Stat(leftoverPath); err != nil {
		t.Errorf("stale attempt's combined worktree missing at %s, want preserved: %v", leftoverPath, err)
	} else {
		_ = worktree.Remove(context.Background(), primaryRoot, leftoverPath, true)
		_ = worktree.DeleteBranch(context.Background(), primaryRoot, "integrate-att-a-2")
	}
}
