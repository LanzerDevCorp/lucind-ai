package resolve_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/resolve"
)

func TestScanConflictMarkers(t *testing.T) {
	dir := t.TempDir()

	// Clean file
	cleanFile := filepath.Join(dir, "clean.txt")
	if err := os.WriteFile(cleanFile, []byte("clean content\nno markers here\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	hasMarkers, markerFiles, err := resolve.ScanConflictMarkers(dir)
	if err != nil {
		t.Fatalf("ScanConflictMarkers() error = %v", err)
	}
	if hasMarkers || len(markerFiles) > 0 {
		t.Errorf("ScanConflictMarkers() hasMarkers = %v, files = %v; want false, empty", hasMarkers, markerFiles)
	}

	// Add file with marker
	dirtyFile := filepath.Join(dir, "dirty.txt")
	if err := os.WriteFile(dirtyFile, []byte("line 1\n<<<<<<< HEAD\nleft\n=======\nright\n>>>>>>> other\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	hasMarkers, markerFiles, err = resolve.ScanConflictMarkers(dir)
	if err != nil {
		t.Fatalf("ScanConflictMarkers() error = %v", err)
	}
	if !hasMarkers {
		t.Errorf("ScanConflictMarkers() hasMarkers = false, want true")
	}
	if len(markerFiles) != 1 || markerFiles[0] != "dirty.txt" {
		t.Errorf("ScanConflictMarkers() files = %v, want [dirty.txt]", markerFiles)
	}
}

func TestEnforceAllowedPaths(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "resolve-test@example.com")
	runGit(t, root, "config", "user.name", "resolve-test")

	// Base commit with file1.txt and file2.txt
	if err := os.WriteFile(filepath.Join(root, "file1.txt"), []byte("base 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "file2.txt"), []byte("base 2\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "base commit")
	baseSHA := runGit(t, root, "rev-parse", "HEAD")

	// In-scope edit to file1.txt
	if err := os.WriteFile(filepath.Join(root, "file1.txt"), []byte("modified 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	offending, err := resolve.EnforceAllowedPaths(context.Background(), root, baseSHA, []string{"file1.txt"})
	if err != nil {
		t.Fatalf("EnforceAllowedPaths() error = %v, want nil", err)
	}
	if len(offending) > 0 {
		t.Errorf("EnforceAllowedPaths() offending = %v, want empty", offending)
	}

	// Out-of-scope edit to file2.txt
	if err := os.WriteFile(filepath.Join(root, "file2.txt"), []byte("modified 2\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	offending, err = resolve.EnforceAllowedPaths(context.Background(), root, baseSHA, []string{"file1.txt"})
	if err == nil {
		t.Fatalf("EnforceAllowedPaths() error = nil, want error")
	}
	if len(offending) != 1 || offending[0] != "file2.txt" {
		t.Errorf("EnforceAllowedPaths() offending = %v, want [file2.txt]", offending)
	}
}

func TestScanConflictMarkers_DocumentationAndScriptPaths(t *testing.T) {
	dir := t.TempDir()

	docsDir := filepath.Join(dir, "docs")
	if err := os.Mkdir(docsDir, 0o755); err != nil {
		t.Fatalf("Mkdir(docs) error = %v", err)
	}
	readme := filepath.Join(docsDir, "README.md")
	if err := os.WriteFile(readme, []byte("# doc\n<<<<<<< HEAD\nleft\n=======\nright\n>>>>>>> other\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md) error = %v", err)
	}

	sentinel := filepath.Join(dir, "script.sh.ran")
	script := "#!/bin/sh\necho executed > \"" + sentinel + "\"\n<<<<<<< HEAD\nleft\n=======\nright\n>>>>>>> other\n"
	scriptPath := filepath.Join(dir, "script.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(script.sh) error = %v", err)
	}

	binary := []byte("<<<<<<< HEAD\nleft\n=======\nright\n>>>>>>> other\n")
	binary = append([]byte{0x00, 0x01, 0xff}, binary...)
	if err := os.WriteFile(filepath.Join(dir, "blob.bin"), binary, 0o644); err != nil {
		t.Fatalf("WriteFile(blob.bin) error = %v", err)
	}

	hasMarkers, markerFiles, err := resolve.ScanConflictMarkers(dir)
	if err != nil {
		t.Fatalf("ScanConflictMarkers() error = %v", err)
	}
	if !hasMarkers {
		t.Fatalf("ScanConflictMarkers() hasMarkers = false, want true (docs and scripts are scanned as text)")
	}
	got := map[string]bool{}
	for _, f := range markerFiles {
		got[f] = true
	}
	if !got["docs/README.md"] {
		t.Errorf("markerFiles = %v, want docs/README.md scanned as text", markerFiles)
	}
	if !got["script.sh"] {
		t.Errorf("markerFiles = %v, want script.sh scanned as text", markerFiles)
	}
	if got["blob.bin"] {
		t.Errorf("markerFiles = %v, want blob.bin skipped as NUL binary", markerFiles)
	}
	if _, err := os.Stat(sentinel); err == nil {
		t.Fatalf("script.sh was executed (sentinel %s exists); ScanConflictMarkers must never exec", sentinel)
	}
}

func TestEnforceAllowedPaths_ExplicitWorktreeCwd(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "resolve-test@example.com")
	runGit(t, repo, "config", "user.name", "resolve-test")
	if err := os.WriteFile(filepath.Join(repo, "in-scope.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "base")
	baseSHA := runGit(t, repo, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(repo, "in-scope.txt"), []byte("edited\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	elsewhere := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(elsewhere); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	offending, err := resolve.EnforceAllowedPaths(context.Background(), repo, baseSHA, []string{"in-scope.txt"})
	if err != nil {
		t.Fatalf("EnforceAllowedPaths() error = %v, want nil (git -C worktreePath, not process cwd)", err)
	}
	if len(offending) > 0 {
		t.Errorf("offending = %v, want empty", offending)
	}
}

func TestEnforceAllowedPaths_StagedAndUntrackedDisjoint(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "resolve-test@example.com")
	runGit(t, repo, "config", "user.name", "resolve-test")
	for _, name := range []string{"in-scope.txt", "unstaged.txt"} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte("base "+name+"\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "base")
	baseSHA := runGit(t, repo, "rev-parse", "HEAD")

	// 1. committed since base
	if err := os.WriteFile(filepath.Join(repo, "committed.txt"), []byte("committed after base\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(committed.txt) error = %v", err)
	}
	runGit(t, repo, "add", "committed.txt")
	runGit(t, repo, "commit", "-m", "committed since base")

	// 2. unstaged
	if err := os.WriteFile(filepath.Join(repo, "unstaged.txt"), []byte("unstaged edit\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(unstaged.txt) error = %v", err)
	}

	// 3. staged
	if err := os.WriteFile(filepath.Join(repo, "staged.txt"), []byte("staged new file\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(staged.txt) error = %v", err)
	}
	runGit(t, repo, "add", "staged.txt")

	// 4. untracked
	if err := os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("untracked\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(untracked.txt) error = %v", err)
	}

	offending, err := resolve.EnforceAllowedPaths(context.Background(), repo, baseSHA, []string{"in-scope.txt"})
	if err == nil {
		t.Fatalf("EnforceAllowedPaths() error = nil, want ErrOutOfScopeEdits")
	}
	if !errors.Is(err, resolve.ErrOutOfScopeEdits) {
		t.Errorf("error = %v, want ErrOutOfScopeEdits", err)
	}
	got := map[string]bool{}
	for _, p := range offending {
		got[p] = true
	}
	for _, want := range []string{"committed.txt", "unstaged.txt", "staged.txt", "untracked.txt"} {
		if !got[want] {
			t.Errorf("offending = %v, want %s in 4-way union vs base_sha", offending, want)
		}
	}
}

func TestResolveCandidateMergeSizeBoundExceeded(t *testing.T) {
	var linesA, linesB strings.Builder
	for i := 0; i < 210; i++ {
		linesA.WriteString(fmt.Sprintf("line a %d\n", i))
		linesB.WriteString(fmt.Sprintf("line b %d\n", i))
	}

	repo := initConflictedRepo(t, map[string][2]string{
		"large_conflict.txt": {linesA.String(), linesB.String()},
	})
	baseSHA := runGit(t, repo, "rev-parse", "HEAD")

	invokerCalled := false
	fakeInvoker := func(ctx context.Context, worktreePath, prompt string) (string, error) {
		invokerCalled = true
		panic("fake invoker must not be called when conflict exceeds bound")
	}

	outcome, err := resolve.ResolveCandidateMerge(context.Background(), resolve.CandidateOptions{
		WorktreePath:     repo,
		BaseSHA:          baseSHA,
		AllowedPaths:     []string{"large_conflict.txt"},
		Invoker:          fakeInvoker,
		MaxConflictLines: 400,
	})
	if err != nil {
		t.Fatalf("ResolveCandidateMerge() error = %v, want nil", err)
	}
	if outcome.Resolved {
		t.Errorf("outcome.Resolved = true, want false")
	}
	if invokerCalled {
		t.Errorf("fake invoker was called, want not called")
	}
	if !strings.Contains(outcome.FailureReason, "conflict exceeds 400-line bound") {
		t.Errorf("outcome.FailureReason = %q, want bound message", outcome.FailureReason)
	}
}

func TestResolveCandidateMergeTimeout(t *testing.T) {
	repo := initConflictedRepo(t, map[string][2]string{
		"file1.txt": {"branch a\n", "branch b\n"},
	})
	baseSHA := runGit(t, repo, "rev-parse", "HEAD")

	fakeInvoker := func(ctx context.Context, worktreePath, prompt string) (string, error) {
		return "timeout occurred", context.DeadlineExceeded
	}

	outcome, err := resolve.ResolveCandidateMerge(context.Background(), resolve.CandidateOptions{
		WorktreePath: repo,
		BaseSHA:      baseSHA,
		AllowedPaths: []string{"file1.txt"},
		Invoker:      fakeInvoker,
		Timeout:      50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("ResolveCandidateMerge() error = %v, want nil", err)
	}
	if outcome.Resolved {
		t.Errorf("outcome.Resolved = true, want false")
	}
	if !strings.Contains(outcome.FailureReason, "deadline exceeded") && !strings.Contains(outcome.FailureReason, "timeout") {
		t.Errorf("outcome.FailureReason = %q, want timeout/deadline exceeded", outcome.FailureReason)
	}
}

func TestResolveCandidateMergeLeftoverMarkersBlock(t *testing.T) {
	repo := initConflictedRepo(t, map[string][2]string{
		"file1.txt": {"branch a\n", "branch b\n"},
	})
	baseSHA := runGit(t, repo, "rev-parse", "HEAD")

	fakeInvoker := func(ctx context.Context, worktreePath, prompt string) (string, error) {
		// Write partial resolution that leaves markers behind
		if err := os.WriteFile(filepath.Join(worktreePath, "file1.txt"), []byte("<<<<<<< HEAD\npartial\n=======\n"), 0o644); err != nil {
			return "", err
		}
		return "done partial", nil
	}

	outcome, err := resolve.ResolveCandidateMerge(context.Background(), resolve.CandidateOptions{
		WorktreePath: repo,
		BaseSHA:      baseSHA,
		AllowedPaths: []string{"file1.txt"},
		Invoker:      fakeInvoker,
	})
	if err != nil {
		t.Fatalf("ResolveCandidateMerge() error = %v, want nil", err)
	}
	if outcome.Resolved {
		t.Errorf("outcome.Resolved = true, want false")
	}
	if !strings.Contains(outcome.FailureReason, "conflict markers remain") {
		t.Errorf("outcome.FailureReason = %q, want marker error", outcome.FailureReason)
	}
}

func TestResolveCandidateMergeOutOfScopeEditsBlock(t *testing.T) {
	repo := initConflictedRepo(t, map[string][2]string{
		"file1.txt": {"branch a\n", "branch b\n"},
	})
	baseSHA := runGit(t, repo, "rev-parse", "HEAD")

	fakeInvoker := func(ctx context.Context, worktreePath, prompt string) (string, error) {
		// Resolve file1.txt
		if err := os.WriteFile(filepath.Join(worktreePath, "file1.txt"), []byte("resolved 1\n"), 0o644); err != nil {
			return "", err
		}
		// Also touch an out-of-scope file
		if err := os.WriteFile(filepath.Join(worktreePath, "unapproved.txt"), []byte("leakage\n"), 0o644); err != nil {
			return "", err
		}
		return "done with leakage", nil
	}

	outcome, err := resolve.ResolveCandidateMerge(context.Background(), resolve.CandidateOptions{
		WorktreePath: repo,
		BaseSHA:      baseSHA,
		AllowedPaths: []string{"file1.txt"},
		Invoker:      fakeInvoker,
	})
	if err != nil {
		t.Fatalf("ResolveCandidateMerge() error = %v, want nil", err)
	}
	if outcome.Resolved {
		t.Errorf("outcome.Resolved = true, want false")
	}
	if !strings.Contains(outcome.FailureReason, "outside declared allowed_paths") {
		t.Errorf("outcome.FailureReason = %q, want out-of-scope error", outcome.FailureReason)
	}
}

func TestResolveCandidateMergeSemanticAmbiguityBlocks(t *testing.T) {
	repo := initConflictedRepo(t, map[string][2]string{
		"pricing.go": {
			"func Price() int { return 100 /* tier A discount rule */ }\n",
			"func Price() int { return 200 /* enterprise override rule */ }\n",
		},
	})
	baseSHA := runGit(t, repo, "rev-parse", "HEAD")

	fakeInvoker := func(ctx context.Context, worktreePath, prompt string) (string, error) {
		return "ambiguous pricing business rule: cannot determine whether Tier A or Enterprise takes precedence without product owner input",
			errors.New("semantic ambiguity: incompatible business logic requires human decision")
	}

	outcome, err := resolve.ResolveCandidateMerge(context.Background(), resolve.CandidateOptions{
		WorktreePath: repo,
		BaseSHA:      baseSHA,
		AllowedPaths: []string{"pricing.go"},
		Invoker:      fakeInvoker,
	})
	if err != nil {
		t.Fatalf("ResolveCandidateMerge() error = %v, want nil", err)
	}
	if outcome.Resolved {
		t.Errorf("outcome.Resolved = true, want false")
	}
	if !strings.Contains(outcome.FailureReason, "semantic ambiguity") {
		t.Errorf("outcome.FailureReason = %q, want semantic ambiguity failure", outcome.FailureReason)
	}
}
