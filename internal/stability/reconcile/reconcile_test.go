package reconcile_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/stability/reconcile"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability/store"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

// setupGitRepo creates a temporary git repository with an initial commit.
func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Stability Test")
	runGit(t, repoDir, "config", "user.email", "stability@example.com")

	filePath := filepath.Join(repoDir, "init.txt")
	if err := os.WriteFile(filePath, []byte("init\n"), 0o644); err != nil {
		t.Fatalf("write init.txt: %v", err)
	}
	runGit(t, repoDir, "add", "init.txt")
	runGit(t, repoDir, "commit", "-m", "initial commit")

	return repoDir
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s failed: %v\nOutput: %s", strings.Join(args, " "), dir, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func setupStore(t *testing.T) *store.Store {
	t.Helper()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "stability.db")
	s, err := store.OpenAtPath(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestInspectCleanState(t *testing.T) {
	ctx := context.Background()
	repoDir := setupGitRepo(t)
	s := setupStore(t)

	camp, err := s.CreateCampaign(ctx, "camp-clean-1", "deadbeef00000000000000000000000000000001")
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}

	// Create linked worktree for lane A
	laneA := "stability-change-a"
	wt, err := worktree.Create(ctx, repoDir, laneA)
	if err != nil {
		t.Fatalf("create worktree %s: %v", laneA, err)
	}
	t.Cleanup(func() { _ = worktree.Cleanup(ctx, repoDir, laneA) })

	report, err := reconcile.Inspect(ctx, reconcile.InspectParams{
		Store:         s,
		CampaignID:    camp.ID,
		PrimaryRoot:   repoDir,
		LaneIDs:       []string{laneA},
		ExpectedLanes: []string{laneA},
		ProcessGroups: []int{},
	})
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}

	if report.Campaign.ID != camp.ID {
		t.Errorf("Campaign ID = %q, want %q", report.Campaign.ID, camp.ID)
	}
	if report.IsTerminal {
		t.Errorf("IsTerminal = true, want false")
	}
	if len(report.Worktrees) != 1 {
		t.Fatalf("len(Worktrees) = %d, want 1", len(report.Worktrees))
	}
	if !report.Worktrees[0].Exists || !report.Worktrees[0].IsLinkedWorktree || !report.Worktrees[0].Clean {
		t.Errorf("Worktree[0] state unexpected: %+v", report.Worktrees[0])
	}
	if report.Worktrees[0].Path != wt.Path {
		t.Errorf("Worktree path = %q, want %q", report.Worktrees[0].Path, wt.Path)
	}

	decision, reason := reconcile.DecideResume(report)
	if decision != reconcile.DecisionSafe {
		t.Errorf("DecideResume = %q (reason: %s), want %q", decision, reason, reconcile.DecisionSafe)
	}
}

func TestInspectTerminalCampaignCannotResume(t *testing.T) {
	ctx := context.Background()
	repoDir := setupGitRepo(t)

	terminalStatuses := []store.Status{
		store.StatusFailed,
		store.StatusBlockedCleanup,
		store.StatusPassed,
	}

	for _, st := range terminalStatuses {
		t.Run(string(st), func(t *testing.T) {
			s := setupStore(t)
			campID := fmt.Sprintf("camp-term-%s", st)
			camp, err := s.CreateCampaign(ctx, campID, "deadbeef00000000000000000000000000000002")
			if err != nil {
				t.Fatalf("create campaign: %v", err)
			}
			if err := s.UpdateCampaignStatus(ctx, camp.ID, st); err != nil {
				t.Fatalf("update status to %s: %v", st, err)
			}

			report, err := reconcile.Inspect(ctx, reconcile.InspectParams{
				Store:       s,
				CampaignID:  camp.ID,
				PrimaryRoot: repoDir,
				LaneIDs:     []string{"stability-change-a"},
			})
			if err != nil {
				t.Fatalf("Inspect failed: %v", err)
			}

			if !report.IsTerminal {
				t.Errorf("IsTerminal = false for status %s, want true", st)
			}

			decision, reason := reconcile.DecideResume(report)
			if decision != reconcile.DecisionFailClosed {
				t.Errorf("DecideResume = %q, want %q (reason: %s)", decision, reconcile.DecisionFailClosed, reason)
			}
		})
	}
}

func TestInspectSurvivingProcessesFailClosed(t *testing.T) {
	ctx := context.Background()
	repoDir := setupGitRepo(t)
	s := setupStore(t)

	camp, err := s.CreateCampaign(ctx, "camp-proc-1", "deadbeef00000000000000000000000000000003")
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}

	fakeAuditor := func(pgid int) ([]int, error) {
		if pgid == 12345 {
			return []int{12345, 12346}, nil
		}
		return nil, nil
	}

	report, err := reconcile.Inspect(ctx, reconcile.InspectParams{
		Store:         s,
		CampaignID:    camp.ID,
		PrimaryRoot:   repoDir,
		ProcessGroups: []int{12345},
		Auditor:       fakeAuditor,
	})
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}

	if len(report.ProcessGroups) != 1 {
		t.Fatalf("len(ProcessGroups) = %d, want 1", len(report.ProcessGroups))
	}
	if !report.ProcessGroups[0].HasSurvivors {
		t.Errorf("HasSurvivors = false, want true")
	}

	decision, reason := reconcile.DecideResume(report)
	if decision != reconcile.DecisionFailClosed {
		t.Errorf("DecideResume = %q, want %q", decision, reconcile.DecisionFailClosed)
	}
	if !strings.Contains(reason, "surviving") && !strings.Contains(reason, "survivor") {
		t.Errorf("Reason %q does not mention survivors", reason)
	}
}

// TestFailClosedOnAmbiguousState proves that deliberately constructed ambiguous states
// (missing expected worktree, dirty uncommitted modifications, non-linked worktree directory)
// always resolve to fail-closed cannot resume, never to a guess.
func TestFailClosedOnAmbiguousState(t *testing.T) {
	ctx := context.Background()
	repoDir := setupGitRepo(t)

	t.Run("MissingExpectedWorktreeFailsClosed", func(t *testing.T) {
		s := setupStore(t)
		camp, err := s.CreateCampaign(ctx, "camp-ambig-1", "deadbeef00000000000000000000000000000004")
		if err != nil {
			t.Fatalf("create campaign: %v", err)
		}

		// Expected lane does not exist on disk
		report, err := reconcile.Inspect(ctx, reconcile.InspectParams{
			Store:         s,
			CampaignID:    camp.ID,
			PrimaryRoot:   repoDir,
			ExpectedLanes: []string{"stability-change-a"},
		})
		if err != nil {
			t.Fatalf("Inspect failed: %v", err)
		}

		decision, reason := reconcile.DecideResume(report)
		if decision != reconcile.DecisionFailClosed {
			t.Fatalf("DecideResume = %q, want fail_closed for missing expected worktree (reason: %s)", decision, reason)
		}
		if len(report.Ambiguities) == 0 {
			t.Errorf("Expected ambiguities to be recorded for missing expected worktree")
		}
	})

	t.Run("DirtyWorktreeFailsClosed", func(t *testing.T) {
		s := setupStore(t)
		camp, err := s.CreateCampaign(ctx, "camp-ambig-2", "deadbeef00000000000000000000000000000005")
		if err != nil {
			t.Fatalf("create campaign: %v", err)
		}

		lane := "stability-change-b"
		wt, err := worktree.Create(ctx, repoDir, lane)
		if err != nil {
			t.Fatalf("create worktree: %v", err)
		}
		t.Cleanup(func() { _ = worktree.Cleanup(ctx, repoDir, lane) })

		// Modify a file in the worktree to make it dirty
		dirtyFile := filepath.Join(wt.Path, "untracked.txt")
		if err := os.WriteFile(dirtyFile, []byte("uncommitted change\n"), 0o644); err != nil {
			t.Fatalf("write dirty file: %v", err)
		}

		report, err := reconcile.Inspect(ctx, reconcile.InspectParams{
			Store:         s,
			CampaignID:    camp.ID,
			PrimaryRoot:   repoDir,
			LaneIDs:       []string{lane},
			ExpectedLanes: []string{lane},
		})
		if err != nil {
			t.Fatalf("Inspect failed: %v", err)
		}

		decision, reason := reconcile.DecideResume(report)
		if decision != reconcile.DecisionFailClosed {
			t.Fatalf("DecideResume = %q, want fail_closed for dirty worktree (reason: %s)", decision, reason)
		}
	})

	t.Run("CorruptOrNonLinkedDirectoryFailsClosed", func(t *testing.T) {
		s := setupStore(t)
		camp, err := s.CreateCampaign(ctx, "camp-ambig-3", "deadbeef00000000000000000000000000000006")
		if err != nil {
			t.Fatalf("create campaign: %v", err)
		}

		lane := "stability-fix-a"
		wtPath := reconcile.WorktreePathFor(repoDir, lane)
		if err := os.MkdirAll(wtPath, 0o755); err != nil {
			t.Fatalf("mkdir fake worktree: %v", err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(wtPath) })

		report, err := reconcile.Inspect(ctx, reconcile.InspectParams{
			Store:         s,
			CampaignID:    camp.ID,
			PrimaryRoot:   repoDir,
			LaneIDs:       []string{lane},
			ExpectedLanes: []string{lane},
		})
		if err != nil {
			t.Fatalf("Inspect failed: %v", err)
		}

		decision, reason := reconcile.DecideResume(report)
		if decision != reconcile.DecisionFailClosed {
			t.Fatalf("DecideResume = %q, want fail_closed for non-linked directory (reason: %s)", decision, reason)
		}
	})
}

// TestAbortIdempotentDoubleCall proves that calling Abort twice in a row against
// the same campaign is an idempotent no-op success on the second call.
func TestAbortIdempotentDoubleCall(t *testing.T) {
	ctx := context.Background()
	repoDir := setupGitRepo(t)
	s := setupStore(t)

	camp, err := s.CreateCampaign(ctx, "camp-abort-idem", "deadbeef00000000000000000000000000000007")
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}

	lane := "stability-change-a"
	_, err = worktree.Create(ctx, repoDir, lane)
	if err != nil {
		t.Fatalf("create worktree: %v", err)
	}

	// First abort call: cleans worktree, deletes branch, transitions store status to failed
	res1, err := reconcile.Abort(ctx, reconcile.AbortParams{
		Store:       s,
		CampaignID:  camp.ID,
		PrimaryRoot: repoDir,
		LaneIDs:     []string{lane},
	})
	if err != nil {
		t.Fatalf("first Abort call failed: %v", err)
	}
	if res1.FinalStatus != store.StatusFailed {
		t.Errorf("res1.FinalStatus = %s, want %s", res1.FinalStatus, store.StatusFailed)
	}
	if len(res1.CleanedWorktrees) != 1 || res1.CleanedWorktrees[0] != lane {
		t.Errorf("res1.CleanedWorktrees = %v, want [%s]", res1.CleanedWorktrees, lane)
	}

	// Verify worktree is gone
	wtPath := reconcile.WorktreePathFor(repoDir, lane)
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("worktree path %s still exists on disk after first abort", wtPath)
	}

	// Second abort call: should be an idempotent no-op success
	res2, err := reconcile.Abort(ctx, reconcile.AbortParams{
		Store:       s,
		CampaignID:  camp.ID,
		PrimaryRoot: repoDir,
		LaneIDs:     []string{lane},
	})
	if err != nil {
		t.Fatalf("second Abort call failed: %v", err)
	}
	if !res2.NoOp {
		t.Errorf("res2.NoOp = false, want true for idempotent retry on already-cleaned terminal campaign")
	}
	if res2.FinalStatus != store.StatusFailed {
		t.Errorf("res2.FinalStatus = %s, want %s", res2.FinalStatus, store.StatusFailed)
	}
}

// TestAbortBlockedCleanupOnGenuineUnremovableResidue proves that when a worktree genuinely
// cannot be removed (locked via git worktree lock), Abort transitions the campaign to blocked_cleanup.
func TestAbortBlockedCleanupOnGenuineUnremovableResidue(t *testing.T) {
	ctx := context.Background()
	repoDir := setupGitRepo(t)
	s := setupStore(t)

	camp, err := s.CreateCampaign(ctx, "camp-blocked-1", "deadbeef00000000000000000000000000000008")
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}

	lane := "stability-change-b"
	wt, err := worktree.Create(ctx, repoDir, lane)
	if err != nil {
		t.Fatalf("create worktree: %v", err)
	}
	t.Cleanup(func() {
		exec.Command("git", "-C", repoDir, "worktree", "unlock", wt.Path).Run() //nolint:errcheck
		_ = worktree.Cleanup(ctx, repoDir, lane)
	})

	// Lock the worktree genuinely using git worktree lock
	runGit(t, repoDir, "worktree", "lock", wt.Path, "--reason", "simulated unremovable lock")

	res, err := reconcile.Abort(ctx, reconcile.AbortParams{
		Store:       s,
		CampaignID:  camp.ID,
		PrimaryRoot: repoDir,
		LaneIDs:     []string{lane},
	})
	if err == nil {
		t.Fatalf("Abort on locked worktree expected error, got nil")
	}

	if res.FinalStatus != store.StatusBlockedCleanup {
		t.Errorf("res.FinalStatus = %s, want %s", res.FinalStatus, store.StatusBlockedCleanup)
	}
	if len(res.FailedWorktrees) != 1 {
		t.Errorf("len(FailedWorktrees) = %d, want 1", len(res.FailedWorktrees))
	}

	// Verify store status was updated to blocked_cleanup
	storedCamp, err := s.GetCampaign(ctx, camp.ID)
	if err != nil {
		t.Fatalf("get campaign from store: %v", err)
	}
	if storedCamp.Status != store.StatusBlockedCleanup {
		t.Errorf("storedCamp.Status = %s, want %s", storedCamp.Status, store.StatusBlockedCleanup)
	}
}

// TestAbortRetryFromBlockedCleanupSkipsCleanedResources proves that retrying Abort from
// blocked_cleanup skips already-cleaned resources and only retries pending residue, transitioning to failed upon resolution.
func TestAbortRetryFromBlockedCleanupSkipsCleanedResources(t *testing.T) {
	ctx := context.Background()
	repoDir := setupGitRepo(t)
	s := setupStore(t)

	camp, err := s.CreateCampaign(ctx, "camp-retry-1", "deadbeef00000000000000000000000000000009")
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}

	lane1 := "stability-change-a"
	lane2 := "stability-change-b"

	_, err = worktree.Create(ctx, repoDir, lane1)
	if err != nil {
		t.Fatalf("create worktree %s: %v", lane1, err)
	}
	wt2, err := worktree.Create(ctx, repoDir, lane2)
	if err != nil {
		t.Fatalf("create worktree %s: %v", lane2, err)
	}
	t.Cleanup(func() {
		exec.Command("git", "-C", repoDir, "worktree", "unlock", wt2.Path).Run() //nolint:errcheck
		_ = worktree.Cleanup(ctx, repoDir, lane1)
		_ = worktree.Cleanup(ctx, repoDir, lane2)
	})

	// Lock only lane2
	runGit(t, repoDir, "worktree", "lock", wt2.Path, "--reason", "simulated unremovable lock")

	// First abort call: lane1 cleaned, lane2 fails, status -> blocked_cleanup
	res1, err := reconcile.Abort(ctx, reconcile.AbortParams{
		Store:       s,
		CampaignID:  camp.ID,
		PrimaryRoot: repoDir,
		LaneIDs:     []string{lane1, lane2},
	})
	if err == nil {
		t.Fatalf("first Abort expected error due to locked lane2, got nil")
	}
	if res1.FinalStatus != store.StatusBlockedCleanup {
		t.Errorf("res1.FinalStatus = %s, want %s", res1.FinalStatus, store.StatusBlockedCleanup)
	}
	if len(res1.CleanedWorktrees) != 1 || res1.CleanedWorktrees[0] != lane1 {
		t.Errorf("res1.CleanedWorktrees = %v, want [%s]", res1.CleanedWorktrees, lane1)
	}
	if len(res1.FailedWorktrees) != 1 {
		t.Errorf("len(res1.FailedWorktrees) = %d, want 1", len(res1.FailedWorktrees))
	}

	// Unlock lane2 so retry can succeed
	runGit(t, repoDir, "worktree", "unlock", wt2.Path)

	// Second abort call: retry from blocked_cleanup
	res2, err := reconcile.Abort(ctx, reconcile.AbortParams{
		Store:       s,
		CampaignID:  camp.ID,
		PrimaryRoot: repoDir,
		LaneIDs:     []string{lane1, lane2},
	})
	if err != nil {
		t.Fatalf("second Abort (retry) failed: %v", err)
	}
	if res2.FinalStatus != store.StatusFailed {
		t.Errorf("res2.FinalStatus = %s, want %s", res2.FinalStatus, store.StatusFailed)
	}

	// Verify lane1 was skipped (already cleaned) and lane2 was newly cleaned
	if len(res2.SkippedWorktrees) != 1 || res2.SkippedWorktrees[0] != lane1 {
		t.Errorf("res2.SkippedWorktrees = %v, want [%s]", res2.SkippedWorktrees, lane1)
	}
	if len(res2.CleanedWorktrees) != 1 || res2.CleanedWorktrees[0] != lane2 {
		t.Errorf("res2.CleanedWorktrees = %v, want [%s]", res2.CleanedWorktrees, lane2)
	}

	// Final store check
	finalCamp, err := s.GetCampaign(ctx, camp.ID)
	if err != nil {
		t.Fatalf("get campaign from store: %v", err)
	}
	if finalCamp.Status != store.StatusFailed {
		t.Errorf("finalCamp.Status = %s, want %s", finalCamp.Status, store.StatusFailed)
	}
}

func TestAbortEvidencePreservationFailureBlocksCleanup(t *testing.T) {
	ctx := context.Background()
	repoDir := setupGitRepo(t)
	s := setupStore(t)

	camp, err := s.CreateCampaign(ctx, "camp-ev-1", "deadbeef00000000000000000000000000000010")
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}

	lane := "stability-change-a"
	wt, err := worktree.Create(ctx, repoDir, lane)
	if err != nil {
		t.Fatalf("create worktree: %v", err)
	}
	t.Cleanup(func() { _ = worktree.Cleanup(ctx, repoDir, lane) })

	failEvidence := func(ctx context.Context) error {
		return fmt.Errorf("simulated disk full while archiving evidence")
	}

	res, err := reconcile.Abort(ctx, reconcile.AbortParams{
		Store:            s,
		CampaignID:       camp.ID,
		PrimaryRoot:      repoDir,
		LaneIDs:          []string{lane},
		PrePurgeEvidence: failEvidence,
	})
	if err == nil {
		t.Fatalf("Abort expected error from failed PrePurgeEvidence, got nil")
	}

	if res.FinalStatus != store.StatusBlockedCleanup {
		t.Errorf("res.FinalStatus = %s, want %s", res.FinalStatus, store.StatusBlockedCleanup)
	}

	// Verify infrastructure was NOT deleted
	if _, err := os.Stat(wt.Path); err != nil {
		t.Errorf("worktree %s was deleted despite evidence preservation failure", wt.Path)
	}
}
