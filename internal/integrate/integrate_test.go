package integrate_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/integrate"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

// initRepo creates a throwaway git repository in t.TempDir() with one
// commit and local user identity configured.
func initRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "integrate-test@example.com")
	runGit(t, root, "config", "user.name", "integrate-test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md) error = %v", err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "seed commit")

	return root
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", append([]string{
		"-c", "user.email=integrate-test@example.com",
		"-c", "user.name=integrate-test",
	}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v error = %v, output = %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestCombineHappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	// Create lane 1 branch with file1.txt
	runGit(t, primaryRoot, "checkout", "-b", "lucind/lane-1")
	if err := os.WriteFile(filepath.Join(primaryRoot, "file1.txt"), []byte("lane 1 content\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(file1.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "file1.txt")
	runGit(t, primaryRoot, "commit", "-m", "lane 1 commit")

	// Create lane 2 branch off main with file2.txt
	runGit(t, primaryRoot, "checkout", "main")
	runGit(t, primaryRoot, "checkout", "-b", "lucind/lane-2")
	if err := os.WriteFile(filepath.Join(primaryRoot, "file2.txt"), []byte("lane 2 content\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(file2.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "file2.txt")
	runGit(t, primaryRoot, "commit", "-m", "lane 2 commit")

	// Return to main
	runGit(t, primaryRoot, "checkout", "main")

	wtPath, branchName, err := integrate.Combine(context.Background(), primaryRoot, "run-happy", []string{"lucind/lane-1", "lucind/lane-2"})
	if err != nil {
		t.Fatalf("Combine() error = %v, want nil", err)
	}
	defer func() {
		_ = worktree.Remove(context.Background(), primaryRoot, wtPath)
		_ = worktree.DeleteBranch(context.Background(), primaryRoot, branchName)
	}()

	wantBranch := "lucind/integrate-run-happy"
	if branchName != wantBranch {
		t.Errorf("branchName = %q, want %q", branchName, wantBranch)
	}

	if !worktree.IsLinkedWorktree(wtPath) {
		t.Errorf("IsLinkedWorktree(%q) = false, want true", wtPath)
	}

	// Verify both files are present in combined worktree
	f1Data, err := os.ReadFile(filepath.Join(wtPath, "file1.txt"))
	if err != nil {
		t.Fatalf("ReadFile(file1.txt) error = %v", err)
	}
	if string(f1Data) != "lane 1 content\n" {
		t.Errorf("file1.txt content = %q, want %q", string(f1Data), "lane 1 content\n")
	}

	f2Data, err := os.ReadFile(filepath.Join(wtPath, "file2.txt"))
	if err != nil {
		t.Fatalf("ReadFile(file2.txt) error = %v", err)
	}
	if string(f2Data) != "lane 2 content\n" {
		t.Errorf("file2.txt content = %q, want %q", string(f2Data), "lane 2 content\n")
	}
}

func TestCombineErrMergeConflictSentinel(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	// Base file
	if err := os.WriteFile(filepath.Join(primaryRoot, "conflict.txt"), []byte("base line\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(conflict.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "conflict.txt")
	runGit(t, primaryRoot, "commit", "-m", "add conflict.txt")

	// Branch 1 modifies conflict.txt
	runGit(t, primaryRoot, "checkout", "-b", "lucind/lane-c1")
	if err := os.WriteFile(filepath.Join(primaryRoot, "conflict.txt"), []byte("change A\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(conflict.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "conflict.txt")
	runGit(t, primaryRoot, "commit", "-m", "lane c1 commit")

	// Branch 2 modifies conflict.txt differently
	runGit(t, primaryRoot, "checkout", "main")
	runGit(t, primaryRoot, "checkout", "-b", "lucind/lane-c2")
	if err := os.WriteFile(filepath.Join(primaryRoot, "conflict.txt"), []byte("change B\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(conflict.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "conflict.txt")
	runGit(t, primaryRoot, "commit", "-m", "lane c2 commit")

	// Return to main
	runGit(t, primaryRoot, "checkout", "main")

	neverResolves := func(ctx context.Context, worktreePath, prompt string) (string, error) {
		return "", errors.New("cannot resolve")
	}

	_, _, err := integrate.CombineWithInvoker(context.Background(), primaryRoot, "run-sentinel", []string{"lucind/lane-c1", "lucind/lane-c2"}, neverResolves)
	if err == nil {
		t.Fatal("Combine() error = nil, want non-nil ErrMergeConflict")
	}
	if !errors.Is(err, integrate.ErrMergeConflict) {
		t.Fatalf("Combine() error = %v, want errors.Is(..., integrate.ErrMergeConflict)", err)
	}
}

func TestCombineConflictCleansUp(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	// Base file
	if err := os.WriteFile(filepath.Join(primaryRoot, "conflict.txt"), []byte("base line\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(conflict.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "conflict.txt")
	runGit(t, primaryRoot, "commit", "-m", "add conflict.txt")

	// Branch A
	runGit(t, primaryRoot, "checkout", "-b", "lucind/lane-ca")
	if err := os.WriteFile(filepath.Join(primaryRoot, "conflict.txt"), []byte("change A\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(conflict.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "conflict.txt")
	runGit(t, primaryRoot, "commit", "-m", "lane ca commit")

	// Branch B
	runGit(t, primaryRoot, "checkout", "main")
	runGit(t, primaryRoot, "checkout", "-b", "lucind/lane-cb")
	if err := os.WriteFile(filepath.Join(primaryRoot, "conflict.txt"), []byte("change B\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(conflict.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "conflict.txt")
	runGit(t, primaryRoot, "commit", "-m", "lane cb commit")

	// Return to main
	runGit(t, primaryRoot, "checkout", "main")

	runID := "run-cleanup-check"
	expectedWorktreePath := filepath.Join(filepath.Dir(primaryRoot), filepath.Base(primaryRoot)+"-worktrees", "integrate-"+runID)
	expectedBranchName := "lucind/integrate-" + runID

	neverResolves := func(ctx context.Context, worktreePath, prompt string) (string, error) {
		return "", errors.New("cannot resolve")
	}

	_, _, err := integrate.CombineWithInvoker(context.Background(), primaryRoot, runID, []string{"lucind/lane-ca", "lucind/lane-cb"}, neverResolves)
	if err == nil {
		t.Fatal("Combine() error = nil, want ErrMergeConflict")
	}
	if !errors.Is(err, integrate.ErrMergeConflict) {
		t.Errorf("Combine() error = %v, want errors.Is(..., integrate.ErrMergeConflict)", err)
	}

	// Verify worktree is no longer linked and cleaned up
	if worktree.IsLinkedWorktree(expectedWorktreePath) {
		t.Errorf("IsLinkedWorktree(%q) = true, want false after failed Combine", expectedWorktreePath)
	}

	// Verify branch was deleted
	branchListOut := runGit(t, primaryRoot, "branch", "--list", expectedBranchName)
	if strings.TrimSpace(branchListOut) != "" {
		t.Errorf("git branch --list %q = %q, want empty after failed Combine", expectedBranchName, branchListOut)
	}
}

func TestCombineConflictResolved(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	// Base file
	if err := os.WriteFile(filepath.Join(primaryRoot, "conflict.txt"), []byte("base line\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(conflict.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "conflict.txt")
	runGit(t, primaryRoot, "commit", "-m", "add conflict.txt")

	// Branch A
	runGit(t, primaryRoot, "checkout", "-b", "lucind/lane-res-a")
	if err := os.WriteFile(filepath.Join(primaryRoot, "conflict.txt"), []byte("change A\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(conflict.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "conflict.txt")
	runGit(t, primaryRoot, "commit", "-m", "lane res a commit")

	// Branch B
	runGit(t, primaryRoot, "checkout", "main")
	runGit(t, primaryRoot, "checkout", "-b", "lucind/lane-res-b")
	if err := os.WriteFile(filepath.Join(primaryRoot, "conflict.txt"), []byte("change B\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(conflict.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "conflict.txt")
	runGit(t, primaryRoot, "commit", "-m", "lane res b commit")

	// Return to main
	runGit(t, primaryRoot, "checkout", "main")

	runID := "run-resolve-success"
	fakeInvoker := func(ctx context.Context, worktreePath, prompt string) (string, error) {
		if err := os.WriteFile(filepath.Join(worktreePath, "conflict.txt"), []byte("merged A and B content\n"), 0o644); err != nil {
			return "", err
		}
		return "resolved", nil
	}

	wtPath, branchName, err := integrate.CombineWithInvoker(context.Background(), primaryRoot, runID, []string{"lucind/lane-res-a", "lucind/lane-res-b"}, fakeInvoker)
	if err != nil {
		t.Fatalf("CombineWithInvoker() error = %v, want nil", err)
	}
	defer func() {
		_ = worktree.Remove(context.Background(), primaryRoot, wtPath)
		_ = worktree.DeleteBranch(context.Background(), primaryRoot, branchName)
	}()

	resolvedData, err := os.ReadFile(filepath.Join(wtPath, "conflict.txt"))
	if err != nil {
		t.Fatalf("ReadFile(conflict.txt) error = %v", err)
	}
	if string(resolvedData) != "merged A and B content\n" {
		t.Errorf("conflict.txt content = %q, want %q", string(resolvedData), "merged A and B content\n")
	}
}

func TestCombineConflictResolutionFailsCleansUp(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	// Base file
	if err := os.WriteFile(filepath.Join(primaryRoot, "conflict.txt"), []byte("base line\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(conflict.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "conflict.txt")
	runGit(t, primaryRoot, "commit", "-m", "add conflict.txt")

	// Branch A
	runGit(t, primaryRoot, "checkout", "-b", "lucind/lane-fail-a")
	if err := os.WriteFile(filepath.Join(primaryRoot, "conflict.txt"), []byte("change A\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(conflict.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "conflict.txt")
	runGit(t, primaryRoot, "commit", "-m", "lane fail a commit")

	// Branch B
	runGit(t, primaryRoot, "checkout", "main")
	runGit(t, primaryRoot, "checkout", "-b", "lucind/lane-fail-b")
	if err := os.WriteFile(filepath.Join(primaryRoot, "conflict.txt"), []byte("change B\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(conflict.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "conflict.txt")
	runGit(t, primaryRoot, "commit", "-m", "lane fail b commit")

	// Return to main
	runGit(t, primaryRoot, "checkout", "main")

	runID := "run-fail-cleanup"
	expectedWorktreePath := filepath.Join(filepath.Dir(primaryRoot), filepath.Base(primaryRoot)+"-worktrees", "integrate-"+runID)
	expectedBranchName := "lucind/integrate-" + runID

	invokerCalled := false
	fakeInvoker := func(ctx context.Context, worktreePath, prompt string) (string, error) {
		invokerCalled = true
		return "failed to resolve", errors.New("resolution error")
	}

	_, _, err := integrate.CombineWithInvoker(context.Background(), primaryRoot, runID, []string{"lucind/lane-fail-a", "lucind/lane-fail-b"}, fakeInvoker)
	if err == nil {
		t.Fatal("CombineWithInvoker() error = nil, want ErrMergeConflict")
	}
	if !invokerCalled {
		t.Errorf("fake invoker was not called")
	}
	if !errors.Is(err, integrate.ErrMergeConflict) {
		t.Errorf("CombineWithInvoker() error = %v, want errors.Is(..., integrate.ErrMergeConflict)", err)
	}

	// Verify worktree is no longer linked and cleaned up
	if worktree.IsLinkedWorktree(expectedWorktreePath) {
		t.Errorf("IsLinkedWorktree(%q) = true, want false after failed combine", expectedWorktreePath)
	}

	// Verify branch was deleted
	branchListOut := runGit(t, primaryRoot, "branch", "--list", expectedBranchName)
	if strings.TrimSpace(branchListOut) != "" {
		t.Errorf("git branch --list %q = %q, want empty after failed combine", expectedBranchName, branchListOut)
	}
}

func TestCombineWorktreeCreateError(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	// Invalid primary root that does not exist
	nonExistentRoot := filepath.Join(t.TempDir(), "non-existent")
	_, _, err := integrate.Combine(context.Background(), nonExistentRoot, "run-err", []string{"branch1"})
	if err == nil {
		t.Fatal("Combine() error = nil, want non-nil when Create fails")
	}
}

func TestCheckAbsentScript(t *testing.T) {
	dir := t.TempDir()
	passed, out, err := integrate.Check(context.Background(), dir)
	if err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	if passed {
		t.Fatalf("Check() passed = true, want false")
	}
	if !strings.Contains(out, "no lucind-checks.sh found") {
		t.Errorf("Check() output = %q, want explanatory message about missing lucind-checks.sh", out)
	}
}

func TestCheckScriptPasses(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to sh")
	}

	dir := t.TempDir()
	scriptContent := "#!/bin/sh\nset -e\necho \"CHECKS_PASSED_OK\"\n"
	if err := os.WriteFile(filepath.Join(dir, "lucind-checks.sh"), []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("WriteFile(lucind-checks.sh) error = %v", err)
	}

	passed, out, err := integrate.Check(context.Background(), dir)
	if err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	if !passed {
		t.Fatalf("Check() passed = false, want true; output = %s", out)
	}
	if !strings.Contains(out, "CHECKS_PASSED_OK") {
		t.Errorf("Check() output = %q, want %q", out, "CHECKS_PASSED_OK")
	}
}

func TestCheckScriptFails(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to sh")
	}

	dir := t.TempDir()
	scriptContent := "#!/bin/sh\necho \"intentional check failure marker\"\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "lucind-checks.sh"), []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("WriteFile(lucind-checks.sh) error = %v", err)
	}

	passed, out, err := integrate.Check(context.Background(), dir)
	if err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	if passed {
		t.Fatalf("Check() passed = true, want false; output = %s", out)
	}
	if !strings.Contains(out, "intentional check failure marker") {
		t.Errorf("Check() output = %q, want to contain failure marker", out)
	}
}

func TestCheckExecutionError(t *testing.T) {
	dir := t.TempDir()
	scriptContent := "#!/bin/sh\necho OK\n"
	if err := os.WriteFile(filepath.Join(dir, "lucind-checks.sh"), []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("WriteFile(lucind-checks.sh) error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := integrate.Check(ctx, dir)
	if err == nil {
		t.Fatal("Check() error = nil, want execution error for cancelled context")
	}
}

func TestPromoteHappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	// Create integration branch with a new commit
	runGit(t, primaryRoot, "checkout", "-b", "lucind/integrate-promo")
	if err := os.WriteFile(filepath.Join(primaryRoot, "promoted.txt"), []byte("promoted content\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(promoted.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "promoted.txt")
	runGit(t, primaryRoot, "commit", "-m", "commit on integrate branch")

	integrationCommit := runGit(t, primaryRoot, "rev-parse", "lucind/integrate-promo")

	// Switch back to main
	runGit(t, primaryRoot, "checkout", "main")

	err := integrate.Promote(context.Background(), primaryRoot, "lucind/integrate-promo")
	if err != nil {
		t.Fatalf("Promote() error = %v, want nil", err)
	}

	mainCommit := runGit(t, primaryRoot, "rev-parse", "HEAD")
	if mainCommit != integrationCommit {
		t.Errorf("main commit = %q, want %q", mainCommit, integrationCommit)
	}

	promotedData, err := os.ReadFile(filepath.Join(primaryRoot, "promoted.txt"))
	if err != nil {
		t.Fatalf("ReadFile(promoted.txt) error = %v", err)
	}
	if string(promotedData) != "promoted content\n" {
		t.Errorf("promoted.txt content = %q, want %q", string(promotedData), "promoted content\n")
	}
}

func TestPromoteDirtyTree(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	// Create integration branch with a new commit
	runGit(t, primaryRoot, "checkout", "-b", "lucind/integrate-dirty")
	if err := os.WriteFile(filepath.Join(primaryRoot, "promoted2.txt"), []byte("promoted content 2\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(promoted2.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "promoted2.txt")
	runGit(t, primaryRoot, "commit", "-m", "commit on integrate branch 2")

	// Switch back to main
	runGit(t, primaryRoot, "checkout", "main")
	headBefore := runGit(t, primaryRoot, "rev-parse", "HEAD")

	// Dirty the primary root
	if err := os.WriteFile(filepath.Join(primaryRoot, "dirty.txt"), []byte("uncommitted\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(dirty.txt) error = %v", err)
	}

	err := integrate.Promote(context.Background(), primaryRoot, "lucind/integrate-dirty")
	if err == nil {
		t.Fatal("Promote() error = nil, want ErrPrimaryRootDirty")
	}
	if !errors.Is(err, integrate.ErrPrimaryRootDirty) {
		t.Fatalf("Promote() error = %v, want errors.Is(..., integrate.ErrPrimaryRootDirty)", err)
	}

	headAfter := runGit(t, primaryRoot, "rev-parse", "HEAD")
	if headAfter != headBefore {
		t.Errorf("HEAD moved from %q to %q despite dirty tree", headBefore, headAfter)
	}
}

func TestPromoteMergeFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)

	// Create branch A with a commit
	runGit(t, primaryRoot, "checkout", "-b", "lucind/integrate-divergent")
	if err := os.WriteFile(filepath.Join(primaryRoot, "divergent.txt"), []byte("div\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(divergent.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "divergent.txt")
	runGit(t, primaryRoot, "commit", "-m", "divergent commit")

	// Switch back to main and make a commit on main so ff is impossible
	runGit(t, primaryRoot, "checkout", "main")
	if err := os.WriteFile(filepath.Join(primaryRoot, "main_change.txt"), []byte("main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(main_change.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "main_change.txt")
	runGit(t, primaryRoot, "commit", "-m", "main commit")

	err := integrate.Promote(context.Background(), primaryRoot, "lucind/integrate-divergent")
	if err == nil {
		t.Fatal("Promote() error = nil, want error on non-ff merge")
	}
	if !strings.Contains(err.Error(), "git merge --ff-only failed") {
		t.Errorf("Promote() error = %q, want it to wrap git merge --ff-only failure", err)
	}
}

func TestPromoteCASHappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)
	shaA := runGit(t, primaryRoot, "rev-parse", "HEAD")

	// Create feature branch pointing at shaA
	runGit(t, primaryRoot, "branch", "feature-alpha", shaA)

	// Create candidate commit on a temporary branch
	runGit(t, primaryRoot, "checkout", "-b", "temp-candidate")
	if err := os.WriteFile(filepath.Join(primaryRoot, "feature_file.txt"), []byte("feature content\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, primaryRoot, "add", "feature_file.txt")
	runGit(t, primaryRoot, "commit", "-m", "feature commit")
	shaCandidate := runGit(t, primaryRoot, "rev-parse", "HEAD")

	// Switch back to main
	runGit(t, primaryRoot, "checkout", "main")
	mainHeadBefore := runGit(t, primaryRoot, "rev-parse", "HEAD")
	mainSymRefBefore := runGit(t, primaryRoot, "symbolic-ref", "HEAD")

	// Promote using CAS
	err := integrate.PromoteCAS(context.Background(), primaryRoot, "refs/heads/feature-alpha", shaCandidate, shaA)
	if err != nil {
		t.Fatalf("PromoteCAS() error = %v, want nil", err)
	}

	// Verify feature-alpha updated to candidate
	featureAlphaHead := runGit(t, primaryRoot, "rev-parse", "refs/heads/feature-alpha")
	if featureAlphaHead != shaCandidate {
		t.Errorf("feature-alpha HEAD = %q, want %q", featureAlphaHead, shaCandidate)
	}

	// Verify primaryRoot HEAD and symbolic-ref were not mutated
	mainHeadAfter := runGit(t, primaryRoot, "rev-parse", "HEAD")
	mainSymRefAfter := runGit(t, primaryRoot, "symbolic-ref", "HEAD")
	if mainHeadAfter != mainHeadBefore {
		t.Errorf("primaryRoot HEAD moved from %q to %q", mainHeadBefore, mainHeadAfter)
	}
	if mainSymRefAfter != mainSymRefBefore {
		t.Errorf("primaryRoot symbolic-ref changed from %q to %q", mainSymRefBefore, mainSymRefAfter)
	}
}

func TestPromoteCASStaleExpectedSHAFails(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)
	shaA := runGit(t, primaryRoot, "rev-parse", "HEAD")

	// Create feature branch pointing at shaA
	runGit(t, primaryRoot, "branch", "feature-alpha", shaA)

	// Create candidate commit
	runGit(t, primaryRoot, "checkout", "-b", "candidate-branch")
	if err := os.WriteFile(filepath.Join(primaryRoot, "candidate.txt"), []byte("candidate\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, primaryRoot, "add", "candidate.txt")
	runGit(t, primaryRoot, "commit", "-m", "candidate commit")
	shaCandidate := runGit(t, primaryRoot, "rev-parse", "HEAD")

	// Switch back to main
	runGit(t, primaryRoot, "checkout", "main")

	// Advance feature-alpha out from under us to an advanced commit
	runGit(t, primaryRoot, "checkout", "-b", "advanced-branch")
	if err := os.WriteFile(filepath.Join(primaryRoot, "advanced.txt"), []byte("advanced\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, primaryRoot, "add", "advanced.txt")
	runGit(t, primaryRoot, "commit", "-m", "advanced commit")
	shaAdvanced := runGit(t, primaryRoot, "rev-parse", "HEAD")

	runGit(t, primaryRoot, "checkout", "main")
	runGit(t, primaryRoot, "update-ref", "refs/heads/feature-alpha", shaAdvanced, shaA)

	// Attempt CAS promotion with stale expected SHA (shaA instead of shaAdvanced)
	err := integrate.PromoteCAS(context.Background(), primaryRoot, "refs/heads/feature-alpha", shaCandidate, shaA)
	if err == nil {
		t.Fatalf("PromoteCAS() with stale expected SHA error = nil, want non-nil ErrStaleCAS")
	}
	if !errors.Is(err, integrate.ErrStaleCAS) {
		t.Errorf("PromoteCAS() error = %v, want errors.Is(..., integrate.ErrStaleCAS)", err)
	}

	// Verify feature-alpha remains unchanged at shaAdvanced
	featureAlphaHead := runGit(t, primaryRoot, "rev-parse", "refs/heads/feature-alpha")
	if featureAlphaHead != shaAdvanced {
		t.Errorf("feature-alpha ref was mutated to %q, want %q", featureAlphaHead, shaAdvanced)
	}
}

func TestPromoteCASDoesNotMutateWorkingTreeOrPrimaryCheckout(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)
	shaA := runGit(t, primaryRoot, "rev-parse", "HEAD")

	// Create feature branch
	runGit(t, primaryRoot, "branch", "feature-beta", shaA)

	// Create candidate commit
	runGit(t, primaryRoot, "checkout", "-b", "cand-b")
	if err := os.WriteFile(filepath.Join(primaryRoot, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, primaryRoot, "add", "b.txt")
	runGit(t, primaryRoot, "commit", "-m", "cand b")
	shaCandidate := runGit(t, primaryRoot, "rev-parse", "HEAD")

	// Switch to main and dirty the working tree with an uncommitted change
	runGit(t, primaryRoot, "checkout", "main")
	dirtyFilePath := filepath.Join(primaryRoot, "uncommitted_work.txt")
	if err := os.WriteFile(dirtyFilePath, []byte("uncommitted working tree data\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	headBefore := runGit(t, primaryRoot, "rev-parse", "HEAD")
	symRefBefore := runGit(t, primaryRoot, "symbolic-ref", "HEAD")

	// Call PromoteCAS - should succeed without checking out or merging anything in primaryRoot
	err := integrate.PromoteCAS(context.Background(), primaryRoot, "feature-beta", shaCandidate, shaA)
	if err != nil {
		t.Fatalf("PromoteCAS() error = %v, want nil", err)
	}

	// Verify working tree file is still untouched
	data, err := os.ReadFile(dirtyFilePath)
	if err != nil {
		t.Fatalf("ReadFile(uncommitted_work.txt) error = %v", err)
	}
	if string(data) != "uncommitted working tree data\n" {
		t.Errorf("dirty file content modified = %q", string(data))
	}

	// Verify primaryRoot HEAD & ref are unchanged
	headAfter := runGit(t, primaryRoot, "rev-parse", "HEAD")
	symRefAfter := runGit(t, primaryRoot, "symbolic-ref", "HEAD")
	if headAfter != headBefore {
		t.Errorf("HEAD changed: %q -> %q", headBefore, headAfter)
	}
	if symRefAfter != symRefBefore {
		t.Errorf("symbolic-ref changed: %q -> %q", symRefBefore, symRefAfter)
	}

	// Verify the feature ref was indeed updated
	featHead := runGit(t, primaryRoot, "rev-parse", "refs/heads/feature-beta")
	if featHead != shaCandidate {
		t.Errorf("feature-beta ref = %q, want %q", featHead, shaCandidate)
	}
}

func TestPromoteCASRejectsInvalidParentRef(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)
	shaA := runGit(t, primaryRoot, "rev-parse", "HEAD")

	testCases := []struct {
		name      string
		parentRef string
	}{
		{name: "empty parent ref", parentRef: ""},
		{name: "whitespace parent ref", parentRef: "   "},
		{name: "bare main", parentRef: "main"},
		{name: "canonical main", parentRef: "refs/heads/main"},
		{name: "bare lucind", parentRef: "lucind"},
		{name: "canonical lucind", parentRef: "refs/heads/lucind"},
		{name: "lucind temp namespace", parentRef: "lucind/lane-123"},
		{name: "canonical lucind temp namespace", parentRef: "refs/heads/lucind/lane-123"},
		{name: "double dot in ref", parentRef: "feature..invalid"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := integrate.PromoteCAS(context.Background(), primaryRoot, tc.parentRef, shaA, shaA)
			if err == nil {
				t.Fatalf("PromoteCAS(parentRef=%q) error = nil, want ErrInvalidParentRef", tc.parentRef)
			}
			if !errors.Is(err, integrate.ErrInvalidParentRef) {
				t.Errorf("PromoteCAS(parentRef=%q) error = %v, want errors.Is(..., integrate.ErrInvalidParentRef)", tc.parentRef, err)
			}
		})
	}
}

func TestPromoteCASRejectsEmptySHAs(t *testing.T) {
	primaryRoot := t.TempDir()

	err := integrate.PromoteCAS(context.Background(), primaryRoot, "refs/heads/feature-alpha", "", "1111222233334444555566667777888899990000")
	if !errors.Is(err, integrate.ErrEmptySHA) {
		t.Errorf("PromoteCAS with empty candidate error = %v, want ErrEmptySHA", err)
	}

	err = integrate.PromoteCAS(context.Background(), primaryRoot, "refs/heads/feature-alpha", "1111222233334444555566667777888899990000", "")
	if !errors.Is(err, integrate.ErrEmptySHA) {
		t.Errorf("PromoteCAS with empty expected error = %v, want ErrEmptySHA", err)
	}
}

func TestPromoteCASWithRunnerSeam(t *testing.T) {
	fakeCalled := false
	fakeRunner := &mockGitRunner{
		runFn: func(ctx context.Context, dir string, args ...string) ([]byte, error) {
			fakeCalled = true
			if len(args) > 0 && args[0] == "check-ref-format" {
				return []byte("feature-test\n"), nil
			}
			if len(args) > 0 && args[0] == "update-ref" {
				return []byte(""), nil
			}
			return nil, nil
		},
	}

	primaryRoot := t.TempDir()
	err := integrate.PromoteCASWithRunner(context.Background(), fakeRunner, primaryRoot, "refs/heads/feature-test", "2222222222222222222222222222222222222222", "1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatalf("PromoteCASWithRunner error = %v", err)
	}
	if !fakeCalled {
		t.Errorf("fake runner was not invoked")
	}
}

func TestPromoteRefAlias(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)
	shaA := runGit(t, primaryRoot, "rev-parse", "HEAD")

	runGit(t, primaryRoot, "branch", "feature-alias", shaA)

	runGit(t, primaryRoot, "checkout", "-b", "alias-cand")
	if err := os.WriteFile(filepath.Join(primaryRoot, "alias.txt"), []byte("alias\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, primaryRoot, "add", "alias.txt")
	runGit(t, primaryRoot, "commit", "-m", "alias commit")
	shaCandidate := runGit(t, primaryRoot, "rev-parse", "HEAD")

	runGit(t, primaryRoot, "checkout", "main")

	err := integrate.PromoteRef(context.Background(), primaryRoot, "refs/heads/feature-alias", shaCandidate, shaA)
	if err != nil {
		t.Fatalf("PromoteRef() error = %v, want nil", err)
	}

	featureHead := runGit(t, primaryRoot, "rev-parse", "refs/heads/feature-alias")
	if featureHead != shaCandidate {
		t.Errorf("feature-alias ref = %q, want %q", featureHead, shaCandidate)
	}
}

type mockGitRunner struct {
	runFn func(ctx context.Context, dir string, args ...string) ([]byte, error)
}

func (m *mockGitRunner) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if m.runFn != nil {
		return m.runFn(ctx, dir, args...)
	}
	return nil, nil
}

