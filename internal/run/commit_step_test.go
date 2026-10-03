package run_test

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/run"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

func initRealGitRepo(t *testing.T, dir string) string {
	t.Helper()
	runGit := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Tester",
			"GIT_AUTHOR_EMAIL=tester@example.com",
			"GIT_COMMITTER_NAME=Tester",
			"GIT_COMMITTER_EMAIL=tester@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v: %s", strings.Join(args, " "), err, string(out))
		}
		return strings.TrimSpace(string(out))
	}

	runGit("init", "-b", "main")
	runGit("config", "user.name", "Tester")
	runGit("config", "user.email", "tester@example.com")
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".lucind/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".gitignore", "seed.txt")
	runGit("commit", "-m", "seed")
	return runGit("rev-parse", "HEAD")
}

const testDoneEnvelopeJSON = `{"packet_id":"lane-a","status":"done","summary":"ok","hard_stops":[{"hard_stop":"stop","fired":false}]}`

func TestDispatcherCommitWorkerAlreadyCommittedDeviated(t *testing.T) {
	wtDir := t.TempDir()
	baseSHA := initRealGitRepo(t, wtDir)

	// Worker already made a commit
	if err := os.WriteFile(filepath.Join(wtDir, "work.txt"), []byte("work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", wtDir, "add", "work.txt")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	cmdCommit := exec.Command("git", "-C", wtDir, "commit", "-m", "worker commit")
	if err := cmdCommit.Run(); err != nil {
		t.Fatal(err)
	}

	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtDir, func(string) fs.FS {
		return fstest.MapFS{resultEnvelopePathForTest(): {Data: []byte(testDoneEnvelopeJSON)}}
	}, fe, baseSHA)

	p := testPacket()
	p.CommitMessage = "feat: add feature"
	p.Verification = []string{"echo pass"}

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if report.Status != lane.Deviated {
		t.Fatalf("report.Status = %v, want %v", report.Status, lane.Deviated)
	}
	if !strings.Contains(report.Diagnosis, "worker committed in a dispatcher-commit packet") {
		t.Fatalf("expected diagnosis to contain %q, got %q", "worker committed in a dispatcher-commit packet", report.Diagnosis)
	}
}

func TestDispatcherCommitNothingToCommitFailed(t *testing.T) {
	wtDir := t.TempDir()
	baseSHA := initRealGitRepo(t, wtDir)

	// Clean tree (only .lucind may exist)
	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtDir, func(string) fs.FS {
		return fstest.MapFS{resultEnvelopePathForTest(): {Data: []byte(testDoneEnvelopeJSON)}}
	}, fe, baseSHA)

	p := testPacket()
	p.CommitMessage = "feat: add feature"
	p.Verification = []string{"echo pass"}

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if report.Status != lane.Failed {
		t.Fatalf("report.Status = %v, want %v", report.Status, lane.Failed)
	}
	if !strings.Contains(report.Diagnosis, "nothing to commit") {
		t.Fatalf("expected diagnosis to contain %q, got %q", "nothing to commit", report.Diagnosis)
	}
}

func TestDispatcherCommitFailingVerificationFailed(t *testing.T) {
	wtDir := t.TempDir()
	baseSHA := initRealGitRepo(t, wtDir)

	// Worktree has an uncommitted file
	if err := os.WriteFile(filepath.Join(wtDir, "change.txt"), []byte("change\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtDir, func(string) fs.FS {
		return fstest.MapFS{resultEnvelopePathForTest(): {Data: []byte(testDoneEnvelopeJSON)}}
	}, fe, baseSHA)

	deps.RunAttested = func(ctx context.Context, dir, cmd string) (int, string, error) {
		return 42, "verification error log", nil // verification command failed with exit 42
	}

	p := testPacket()
	p.CommitMessage = "feat: add feature"
	p.Verification = []string{"go test ./..."}

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if report.Status != lane.Failed {
		t.Fatalf("report.Status = %v, want %v", report.Status, lane.Failed)
	}
	wantReason := "verification failed: go test ./... (exit 42)"
	if !strings.Contains(report.Diagnosis, wantReason) {
		t.Fatalf("expected diagnosis to contain %q, got %q", wantReason, report.Diagnosis)
	}
}

func TestDispatcherCommitMissingAttestationBlocked(t *testing.T) {
	wtDir := t.TempDir()
	baseSHA := initRealGitRepo(t, wtDir)

	if err := os.WriteFile(filepath.Join(wtDir, "change.txt"), []byte("change\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtDir, func(string) fs.FS {
		return fstest.MapFS{resultEnvelopePathForTest(): {Data: []byte(testDoneEnvelopeJSON)}}
	}, fe, baseSHA)

	deps.RunAttested = func(ctx context.Context, dir, cmd string) (int, string, error) {
		return 0, "", nil
	}
	deps.HasValidAttestation = func(ctx context.Context, repoRoot, cmd, expectedTreeHash string) (bool, error) {
		return false, nil // no valid attestation for current tree
	}

	p := testPacket()
	p.CommitMessage = "feat: add feature"
	p.Verification = []string{"go test ./..."}

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if report.Status != lane.Blocked {
		t.Fatalf("report.Status = %v, want %v", report.Status, lane.Blocked)
	}
	wantReason := "missing attestation for: go test ./..."
	if !strings.Contains(report.Diagnosis, wantReason) {
		t.Fatalf("expected diagnosis to contain %q, got %q", wantReason, report.Diagnosis)
	}
}

func TestDispatcherCommitPreCommitGateBlocks(t *testing.T) {
	wtDir := t.TempDir()
	baseSHA := initRealGitRepo(t, wtDir)

	if err := os.WriteFile(filepath.Join(wtDir, "change.txt"), []byte("change\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtDir, func(string) fs.FS {
		return fstest.MapFS{resultEnvelopePathForTest(): {Data: []byte(testDoneEnvelopeJSON)}}
	}, fe, baseSHA)

	deps.RunAttested = func(ctx context.Context, dir, cmd string) (int, string, error) {
		return 0, "", nil
	}
	deps.HasValidAttestation = func(ctx context.Context, repoRoot, cmd, expectedTreeHash string) (bool, error) {
		return true, nil
	}
	gateCalled := false
	deps.PreCommitGate = func(ctx context.Context, worktreePath string, p packet.Packet) (lane.Status, string) {
		gateCalled = true
		return lane.Blocked, "judge review rejected candidate"
	}

	p := testPacket()
	p.CommitMessage = "feat: add feature"
	p.Verification = []string{"go test ./..."}

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !gateCalled {
		t.Fatal("expected PreCommitGate to be called")
	}
	if report.Status != lane.Blocked {
		t.Fatalf("report.Status = %v, want %v", report.Status, lane.Blocked)
	}
	if !strings.Contains(report.Diagnosis, "judge review rejected candidate") {
		t.Fatalf("expected diagnosis to contain gate rejection, got %q", report.Diagnosis)
	}

	// Verify no commit was made
	headCmd := exec.Command("git", "-C", wtDir, "rev-parse", "HEAD")
	headOut, err := headCmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(headOut)) != baseSHA {
		t.Fatalf("commit was made despite gate blocking! HEAD = %s, baseSHA = %s", string(headOut), baseSHA)
	}
}

func TestDispatcherCommitSuccess(t *testing.T) {
	wtDir := t.TempDir()
	baseSHA := initRealGitRepo(t, wtDir)

	// Worker created a file and .lucind directory
	if err := os.WriteFile(filepath.Join(wtDir, "change.txt"), []byte("change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lucindDir := filepath.Join(wtDir, ".lucind")
	if err := os.MkdirAll(lucindDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lucindDir, "result.json"), []byte(testDoneEnvelopeJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtDir, func(string) fs.FS {
		return os.DirFS(wtDir)
	}, fe, baseSHA)
	deps.PrimaryRoot = wtDir
	deps.HasUniqueLaneCommits = func(ctx context.Context, worktreePath, base string) (bool, error) {
		return worktree.HasUniqueCommits(ctx, worktreePath, base)
	}
	deps.PorcelainEmpty = func(ctx context.Context, worktreePath string) (bool, error) {
		return worktree.PorcelainEmpty(ctx, worktreePath)
	}
	deps.ResolveCandidateIdentity = run.ResolveCandidateIdentityFromGit

	deps.RunAttested = func(ctx context.Context, dir, cmd string) (int, string, error) {
		return 0, "", nil
	}
	deps.HasValidAttestation = func(ctx context.Context, repoRoot, cmd, expectedTreeHash string) (bool, error) {
		return true, nil
	}

	p := testPacket()
	p.CommitMessage = "feat(core): implement dispatcher commit"
	p.Verification = []string{"echo pass"}

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if report.Status != lane.Done {
		t.Fatalf("report.Status = %v, want %v; diagnosis = %s", report.Status, lane.Done, report.Diagnosis)
	}

	// Verify exactly one new commit was made
	cmdCount := exec.Command("git", "-C", wtDir, "rev-list", "--count", baseSHA+"..HEAD")
	countOut, err := cmdCount.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(countOut)) != "1" {
		t.Fatalf("expected exactly 1 commit, got %s", string(countOut))
	}

	// Verify commit message
	cmdMsg := exec.Command("git", "-C", wtDir, "log", "-1", "--format=%B")
	msgOut, err := cmdMsg.Output()
	if err != nil {
		t.Fatal(err)
	}
	commitMsg := strings.TrimSpace(string(msgOut))
	if commitMsg != "feat(core): implement dispatcher commit" {
		t.Fatalf("commit message = %q, want %q", commitMsg, "feat(core): implement dispatcher commit")
	}

	// Verify no trailers
	if strings.Contains(strings.ToLower(commitMsg), "co-authored-by") || strings.Contains(strings.ToLower(commitMsg), "generated with") {
		t.Fatalf("commit message contains forbidden trailers: %q", commitMsg)
	}

	// Verify .lucind/ was NOT committed
	cmdShow := exec.Command("git", "-C", wtDir, "show", "--name-only", "--format=")
	showOut, err := cmdShow.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range strings.Split(string(showOut), "\n") {
		if strings.HasPrefix(f, ".lucind") {
			t.Fatalf(".lucind file committed: %s", f)
		}
	}
}

func TestDispatcherCommitDoesNotCommitUntrackedLucindDir(t *testing.T) {
	wtDir := t.TempDir()
	initRealGitRepo(t, wtDir)
	// Stop ignoring .lucind so it is an untracked (not ignored) directory.
	if err := os.WriteFile(filepath.Join(wtDir, ".gitignore"), []byte("\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", ".gitignore"}, {"commit", "-q", "--amend", "--no-edit"}} {
		if out, err := exec.Command("git", append([]string{"-C", wtDir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	headOut, err := exec.Command("git", "-C", wtDir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	baseSHA := strings.TrimSpace(string(headOut))

	// Worker created a file and .lucind directory
	if err := os.WriteFile(filepath.Join(wtDir, "change.txt"), []byte("change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lucindDir := filepath.Join(wtDir, ".lucind")
	if err := os.MkdirAll(lucindDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lucindDir, "result.json"), []byte(testDoneEnvelopeJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtDir, func(string) fs.FS {
		return os.DirFS(wtDir)
	}, fe, baseSHA)
	deps.PrimaryRoot = wtDir
	deps.HasUniqueLaneCommits = func(ctx context.Context, worktreePath, base string) (bool, error) {
		return worktree.HasUniqueCommits(ctx, worktreePath, base)
	}
	deps.PorcelainEmpty = func(ctx context.Context, worktreePath string) (bool, error) {
		return worktree.PorcelainEmpty(ctx, worktreePath)
	}
	deps.ResolveCandidateIdentity = run.ResolveCandidateIdentityFromGit

	deps.RunAttested = func(ctx context.Context, dir, cmd string) (int, string, error) {
		return 0, "", nil
	}
	deps.HasValidAttestation = func(ctx context.Context, repoRoot, cmd, expectedTreeHash string) (bool, error) {
		return true, nil
	}

	p := testPacket()
	p.CommitMessage = "feat(core): implement dispatcher commit"
	p.Verification = []string{"echo pass"}

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	// With .lucind untracked and not ignored, the pre-existing completion check
	// (clean working tree) fails the lane; what matters here is that the
	// dispatcher itself never stages the .lucind result files.
	if report.Status != lane.Failed || !strings.Contains(report.Diagnosis, "uncommitted changes") {
		t.Fatalf("report.Status = %v (diagnosis %q), want failed on the uncommitted-changes completion check", report.Status, report.Diagnosis)
	}

	// Verify exactly one new commit was made
	cmdCount := exec.Command("git", "-C", wtDir, "rev-list", "--count", baseSHA+"..HEAD")
	countOut, err := cmdCount.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(countOut)) != "1" {
		t.Fatalf("expected exactly 1 commit, got %s", string(countOut))
	}

	// Verify commit message
	cmdMsg := exec.Command("git", "-C", wtDir, "log", "-1", "--format=%B")
	msgOut, err := cmdMsg.Output()
	if err != nil {
		t.Fatal(err)
	}
	commitMsg := strings.TrimSpace(string(msgOut))
	if commitMsg != "feat(core): implement dispatcher commit" {
		t.Fatalf("commit message = %q, want %q", commitMsg, "feat(core): implement dispatcher commit")
	}

	// Verify no trailers
	if strings.Contains(strings.ToLower(commitMsg), "co-authored-by") || strings.Contains(strings.ToLower(commitMsg), "generated with") {
		t.Fatalf("commit message contains forbidden trailers: %q", commitMsg)
	}

	// Verify .lucind/ was NOT committed
	cmdShow := exec.Command("git", "-C", wtDir, "show", "--name-only", "--format=")
	showOut, err := cmdShow.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range strings.Split(string(showOut), "\n") {
		if strings.HasPrefix(f, ".lucind") {
			t.Fatalf(".lucind file committed: %s", f)
		}
	}
}

func TestDispatcherCommitIdentityFallback(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	wtDir := t.TempDir()
	// Init git without identity
	cmdInit := exec.Command("git", "-C", wtDir, "init", "-b", "main")
	if err := cmdInit.Run(); err != nil {
		t.Fatal(err)
	}
	// Commit initial with explicit -c
	if err := os.WriteFile(filepath.Join(wtDir, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmdAdd := exec.Command("git", "-C", wtDir, "add", "seed.txt")
	if err := cmdAdd.Run(); err != nil {
		t.Fatal(err)
	}
	cmdCommit := exec.Command("git", "-C", wtDir, "-c", "user.name=Init", "-c", "user.email=init@test.com", "commit", "-m", "init")
	if err := cmdCommit.Run(); err != nil {
		t.Fatal(err)
	}
	cmdRev := exec.Command("git", "-C", wtDir, "rev-parse", "HEAD")
	baseOut, err := cmdRev.Output()
	if err != nil {
		t.Fatal(err)
	}
	baseSHA := strings.TrimSpace(string(baseOut))

	if err := os.WriteFile(filepath.Join(wtDir, "change.txt"), []byte("change\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fe := &fakeExecutor{outcome: executor.Outcome{ExitCode: 0}}
	deps := newTestDeps(t, wtDir, func(string) fs.FS {
		return fstest.MapFS{resultEnvelopePathForTest(): {Data: []byte(testDoneEnvelopeJSON)}}
	}, fe, baseSHA)
	deps.RunAttested = func(ctx context.Context, dir, cmd string) (int, string, error) { return 0, "", nil }
	deps.HasValidAttestation = func(ctx context.Context, repoRoot, cmd, expectedTreeHash string) (bool, error) { return true, nil }

	p := testPacket()
	p.CommitMessage = "fix: fallback identity"
	p.Verification = []string{"echo pass"}

	report, err := run.Execute(context.Background(), deps, p)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if report.Status != lane.Done {
		t.Fatalf("report.Status = %v, want %v; diagnosis = %s", report.Status, lane.Done, report.Diagnosis)
	}

	// Verify author of the commit is fallback lucind-ai
	cmdAuthor := exec.Command("git", "-C", wtDir, "log", "-1", "--format=%an <%ae>")
	authorOut, err := cmdAuthor.Output()
	if err != nil {
		t.Fatal(err)
	}
	author := strings.TrimSpace(string(authorOut))
	if author != "lucind-ai <lucind-ai@localhost>" {
		t.Fatalf("expected author 'lucind-ai <lucind-ai@localhost>', got %q", author)
	}
}

func TestCommitMessageChangesPacketDigest(t *testing.T) {
	p := testPacket()
	baseDigest := run.PacketDigest(p, []string{"internal/run"})

	p.CommitMessage = "feat: add something"
	newDigest := run.PacketDigest(p, []string{"internal/run"})

	if newDigest == baseDigest {
		t.Fatalf("expected digest to change when CommitMessage is set, both are %q", baseDigest)
	}
}

func TestMaxIterationsChangesPacketDigest(t *testing.T) {
	p := testPacket()
	baseDigest := run.PacketDigest(p, []string{"internal/run"})

	p.MaxIterations = 1
	if d := run.PacketDigest(p, []string{"internal/run"}); d != baseDigest {
		t.Fatalf("expected max_iterations 1 not to change digest: got %q, want %q", d, baseDigest)
	}

	p.MaxIterations = 2
	if d := run.PacketDigest(p, []string{"internal/run"}); d == baseDigest {
		t.Fatalf("expected max_iterations 2 to change digest: both %q", d)
	}
}

func TestEscalationChangesPacketDigest(t *testing.T) {
	p := testPacket()
	baseDigest := run.PacketDigest(p, []string{"internal/run"})

	p.Escalation = []packet.EscalationRung{{Executor: "herdr-agy"}}
	if d := run.PacketDigest(p, []string{"internal/run"}); d == baseDigest {
		t.Fatalf("expected escalation to change digest: both %q", d)
	}
}

func TestDispatcherCommitVerificationThatModifiesTreeFailsClosed(t *testing.T) {
	wtDir := t.TempDir()
	baseSHA := initRealGitRepo(t, wtDir)
	if err := os.WriteFile(filepath.Join(wtDir, "change.txt"), []byte("change\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := run.Deps{
		RunAttested: func(ctx context.Context, dir, cmd string) (int, string, error) {
			// A "verification" that edits the tree after the worker finished.
			return 0, "", os.WriteFile(filepath.Join(dir, "sneaky.txt"), []byte("injected\n"), 0o644)
		},
		HasValidAttestation: func(ctx context.Context, repoRoot, cmd, tree string) (bool, error) { return true, nil },
	}
	p := testPacket()
	p.CommitMessage = "feat: x"
	p.Verification = []string{"echo ok"}

	status, reason := run.DispatcherCommitForTest(context.Background(), deps, wtDir, baseSHA, p)
	if status != lane.Failed || !strings.Contains(reason, "changed the worktree") {
		t.Fatalf("status=%v reason=%q, want failed with a tree-changed reason", status, reason)
	}
	out, err := exec.Command("git", "-C", wtDir, "rev-list", "--count", baseSHA+"..HEAD").Output()
	if err != nil || strings.TrimSpace(string(out)) != "0" {
		t.Fatalf("no commit may be made, rev-list=%q err=%v", out, err)
	}
}

func TestDispatcherCommitSkipsRepositoryHooks(t *testing.T) {
	wtDir := t.TempDir()
	baseSHA := initRealGitRepo(t, wtDir)
	hook := filepath.Join(wtDir, ".git", "hooks", "commit-msg")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nprintf '\\nCo-Authored-By: Someone <x@y.z>\\n' >> \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtDir, "change.txt"), []byte("change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := run.Deps{
		RunAttested:         func(ctx context.Context, dir, cmd string) (int, string, error) { return 0, "", nil },
		HasValidAttestation: func(ctx context.Context, repoRoot, cmd, tree string) (bool, error) { return true, nil },
	}
	p := testPacket()
	p.CommitMessage = "feat: hooks must not run"
	p.Verification = []string{"echo ok"}

	status, reason := run.DispatcherCommitForTest(context.Background(), deps, wtDir, baseSHA, p)
	if status != lane.Done {
		t.Fatalf("status=%v reason=%q", status, reason)
	}
	msg, err := exec.Command("git", "-C", wtDir, "log", "-1", "--format=%B").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(msg)), "co-authored-by") {
		t.Fatalf("a commit-msg hook ran and injected a trailer: %q", msg)
	}
}
