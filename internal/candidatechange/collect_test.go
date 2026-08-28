package candidatechange_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/candidatechange"
)

func TestCollectCanonicalCommittedChangesAndCopyScope(t *testing.T) {
	root, base := seedRepo(t)
	write(t, root, "created.txt", "new\n")
	write(t, root, "modified.txt", "changed\n")
	if err := os.Remove(filepath.Join(root, "deleted.txt")); err != nil {
		t.Fatal(err)
	}
	git(t, root, "mv", "rename-source.txt", "rename-dest.txt")
	data, err := os.ReadFile(filepath.Join(root, "copy-source.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "copy-dest.txt"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "--all")
	git(t, root, "commit", "-m", "candidate")
	candidate := gitOut(t, root, "rev-parse", "HEAD")

	got, err := candidatechange.Collect(context.Background(), candidatechange.Request{Root: root, BaseCommit: base, CandidateCommit: candidate})
	if err != nil {
		t.Fatal(err)
	}
	want := []candidatechange.Change{
		{Change: candidatechange.Created, Path: "copy-dest.txt", SourcePath: "copy-source.txt"},
		{Change: candidatechange.Created, Path: "created.txt"},
		{Change: candidatechange.Deleted, Path: "deleted.txt"},
		{Change: candidatechange.Modified, Path: "modified.txt"},
		{Change: candidatechange.Created, Path: "rename-dest.txt"},
		{Change: candidatechange.Deleted, Path: "rename-source.txt"},
	}
	// A detected copy has its own canonical kind and shape.
	want[0].Change = candidatechange.Copied
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Collect() = %#v, want %#v", got, want)
	}
	if outside := candidatechange.OutOfScope(got, []string{"copy-dest.txt", "created.txt", "deleted.txt", "modified.txt", "rename-dest.txt", "rename-source.txt"}); !reflect.DeepEqual(outside, []string{"copy-source.txt"}) {
		t.Fatalf("OutOfScope() = %v, want copy source", outside)
	}
}

func TestCollectFourWayUnionAndCanonicalRootSelectors(t *testing.T) {
	root, base := seedRepo(t)
	write(t, root, "modified.txt", "commit-a\n")
	git(t, root, "commit", "-am", "commit-a")
	candidate := gitOut(t, root, "rev-parse", "HEAD")
	committed, err := candidatechange.Collect(context.Background(), candidatechange.Request{Root: root, BaseCommit: base, CandidateCommit: candidate})
	if err != nil || !contains(committed, candidatechange.Change{Change: candidatechange.Modified, Path: "modified.txt"}) {
		t.Fatalf("commit -a candidate = %v, %v", committed, err)
	}
	write(t, root, "modified.txt", "unstaged\n")
	write(t, root, "staged.txt", "staged\n")
	git(t, root, "add", "staged.txt")
	write(t, root, "untracked.txt", "untracked\n")

	got, err := candidatechange.Collect(context.Background(), candidatechange.Request{Root: root, BaseCommit: base, CandidateCommit: candidate, IncludeWorktree: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []candidatechange.Change{{Change: candidatechange.Modified, Path: "modified.txt"}, {Change: candidatechange.Created, Path: "staged.txt"}, {Change: candidatechange.Created, Path: "untracked.txt"}} {
		if !contains(got, want) {
			t.Errorf("four-way union %v missing %v", got, want)
		}
	}

	symlink := filepath.Join(t.TempDir(), "repo-link")
	if err := os.Symlink(root, symlink); err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	relative, _ := filepath.Rel(cwd, root)
	for _, selector := range []string{root, relative, symlink} {
		clean, err := candidatechange.Collect(context.Background(), candidatechange.Request{Root: selector, BaseCommit: base, CandidateCommit: candidate})
		if err != nil || !contains(clean, candidatechange.Change{Change: candidatechange.Modified, Path: "modified.txt"}) {
			t.Fatalf("selector %q: %v, %v", selector, clean, err)
		}
	}
	git(t, root, "read-tree", "--empty")
	emptyIndex, err := candidatechange.Collect(context.Background(), candidatechange.Request{Root: root, BaseCommit: base, CandidateCommit: candidate, IncludeWorktree: true})
	if err != nil || !contains(emptyIndex, candidatechange.Change{Change: candidatechange.Deleted, Path: "modified.txt"}) {
		t.Fatalf("empty-index union = %v, %v", emptyIndex, err)
	}
	if _, err := candidatechange.Collect(context.Background(), candidatechange.Request{Root: filepath.Join(root, ".git"), BaseCommit: base, CandidateCommit: candidate}); err == nil {
		t.Fatal("mismatched repository selector accepted")
	}
}

func seedRepo(t *testing.T) (string, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repo;touch argv-injection")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "-b", "main")
	git(t, root, "config", "user.name", "test")
	git(t, root, "config", "user.email", "test@example.com")
	for _, p := range []string{"modified.txt", "deleted.txt", "rename-source.txt"} {
		write(t, root, p, p+"\n")
	}
	write(t, root, "copy-source.txt", strings.Repeat("copy source line\n", 20))
	git(t, root, "add", "--all")
	git(t, root, "commit", "-m", "base")
	return root, gitOut(t, root, "rev-parse", "HEAD")
}

func contains(changes []candidatechange.Change, want candidatechange.Change) bool {
	for _, got := range changes {
		if got == want {
			return true
		}
	}
	return false
}
func write(t *testing.T, root, path, value string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, path), []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}
func git(t *testing.T, root string, args ...string) { t.Helper(); _ = gitOut(t, root, args...) }
func gitOut(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
