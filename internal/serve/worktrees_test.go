package serve_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/serve"
)

func TestListWorktreesReportsLiveAndStaleEntriesWithoutRepositoryMutation(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	ctx := context.Background()
	primaryRoot := initWorktreeStatusRepo(t)
	livePath := filepath.Join(t.TempDir(), "live-lane")
	stalePath := filepath.Join(t.TempDir(), "stale-lane")
	addWorktree(t, primaryRoot, livePath, "live-lane")
	addWorktree(t, primaryRoot, stalePath, "stale-lane")

	payload := []byte("worktree payload")
	if err := os.WriteFile(filepath.Join(livePath, "payload.bin"), payload, 0o644); err != nil {
		t.Fatalf("write live payload: %v", err)
	}
	if err := os.RemoveAll(stalePath); err != nil {
		t.Fatalf("remove stale worktree directory: %v", err)
	}

	l, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		t.Fatalf("ledger.Open: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })
	registerWorktreeStatusLanes(t, l, livePath, stalePath)

	wantLiveBytes := directoryBytes(t, livePath)
	beforeStatus := runGitForWorktreeStatus(t, primaryRoot, "status", "--porcelain=v1", "--untracked-files=all")
	beforeWorktrees := runGitForWorktreeStatus(t, primaryRoot, "worktree", "list", "--porcelain")

	got, err := serve.NewModel(l).ListWorktrees(ctx)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}

	afterStatus := runGitForWorktreeStatus(t, primaryRoot, "status", "--porcelain=v1", "--untracked-files=all")
	afterWorktrees := runGitForWorktreeStatus(t, primaryRoot, "worktree", "list", "--porcelain")
	if beforeStatus != afterStatus {
		t.Errorf("repository status changed:\nbefore:\n%s\nafter:\n%s", beforeStatus, afterStatus)
	}
	if beforeWorktrees != afterWorktrees {
		t.Errorf("worktree metadata changed:\nbefore:\n%s\nafter:\n%s", beforeWorktrees, afterWorktrees)
	}

	if len(got) != 2 {
		t.Fatalf("ListWorktrees returned %d entries, want 2: %+v", len(got), got)
	}

	live := got[0]
	if live.Path != livePath || live.Branch != "lucind/live-lane" || live.LaneID != "live-lane" {
		t.Errorf("live association = %+v, want path %q, branch %q, lane %q", live, livePath, "lucind/live-lane", "live-lane")
	}
	if live.Stale {
		t.Errorf("live.Stale = true, want false")
	}
	if live.DiskBytes != wantLiveBytes {
		t.Errorf("live.DiskBytes = %d, want %d", live.DiskBytes, wantLiveBytes)
	}

	stale := got[1]
	if stale.Path != stalePath || stale.Branch != "lucind/stale-lane" || stale.LaneID != "stale-lane" {
		t.Errorf("stale association = %+v, want path %q, branch %q, lane %q", stale, stalePath, "lucind/stale-lane", "stale-lane")
	}
	if !stale.Stale {
		t.Errorf("stale.Stale = false, want true")
	}
	if stale.DiskBytes != 0 {
		t.Errorf("stale.DiskBytes = %d, want 0", stale.DiskBytes)
	}
}

func initWorktreeStatusRepo(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "primary")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatalf("mkdir primary repository: %v", err)
	}
	runGitForWorktreeStatus(t, root, "init", "-b", "main")
	runGitForWorktreeStatus(t, root, "config", "user.email", "test@example.com")
	runGitForWorktreeStatus(t, root, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatalf("write repository fixture: %v", err)
	}
	runGitForWorktreeStatus(t, root, "add", "README.md")
	runGitForWorktreeStatus(t, root, "commit", "-m", "test fixture")
	return root
}

func addWorktree(t *testing.T, primaryRoot, path, laneID string) {
	t.Helper()
	runGitForWorktreeStatus(t, primaryRoot, "worktree", "add", "-b", "lucind/"+laneID, path)
}

func registerWorktreeStatusLanes(t *testing.T, l *ledger.Ledger, livePath, stalePath string) {
	t.Helper()
	ctx := context.Background()
	if err := l.RegisterRun(ctx, ledger.Run{
		RunID:     "run-worktrees",
		Status:    "running",
		TargetRef: "refs/heads/main",
		LaneCount: 2,
		StartedAt: time.Now(),
	}); err != nil {
		t.Fatalf("RegisterRun: %v", err)
	}
	for _, ln := range []ledger.Lane{
		{RunID: "run-worktrees", LaneID: "live-lane", PacketID: "live-packet", Executor: "agy", RoutingCondition: "live fixture", Status: lane.Running, WorktreePath: livePath},
		{RunID: "run-worktrees", LaneID: "stale-lane", PacketID: "stale-packet", Executor: "agy", RoutingCondition: "stale fixture", Status: lane.Blocked, WorktreePath: stalePath, WorktreePreserved: true},
	} {
		if err := l.RegisterLane(ctx, ln); err != nil {
			t.Fatalf("RegisterLane(%s): %v", ln.LaneID, err)
		}
	}
}

func directoryBytes(t *testing.T, root string) int64 {
	t.Helper()
	var total int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		t.Fatalf("measure directory %q: %v", root, err)
	}
	return total
}

func runGitForWorktreeStatus(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %q: %v\n%s", args, dir, err, out)
	}
	return string(out)
}
