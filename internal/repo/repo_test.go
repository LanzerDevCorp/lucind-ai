package repo_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/repo"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func commitAll(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "commit")
}

func TestTreeHash(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	writeFile(t, filepath.Join(dir, "a.txt"), "one\n")
	writeFile(t, filepath.Join(dir, ".gitignore"), "ignored.txt\n")
	commitAll(t, dir)

	clean, err := repo.TreeHash(ctx, dir)
	if err != nil {
		t.Fatalf("TreeHash: %v", err)
	}
	if want := runGit(t, dir, "rev-parse", "HEAD^{tree}"); clean != want {
		t.Fatalf("clean tree hash = %s, want HEAD tree %s", clean, want)
	}

	writeFile(t, filepath.Join(dir, "a.txt"), "two\n")
	tracked, err := repo.TreeHash(ctx, dir)
	if err != nil {
		t.Fatalf("TreeHash tracked: %v", err)
	}
	if tracked == clean {
		t.Fatal("tracked change did not change the tree hash")
	}

	writeFile(t, filepath.Join(dir, "new.txt"), "new\n")
	untracked, err := repo.TreeHash(ctx, dir)
	if err != nil {
		t.Fatalf("TreeHash untracked: %v", err)
	}
	if untracked == tracked {
		t.Fatal("untracked file was not included in the tree hash")
	}

	writeFile(t, filepath.Join(dir, "ignored.txt"), "ignored\n")
	ignored, err := repo.TreeHash(ctx, dir)
	if err != nil {
		t.Fatalf("TreeHash ignored: %v", err)
	}
	if ignored != untracked {
		t.Fatal("ignored file changed the tree hash")
	}

	if status := runGit(t, dir, "diff", "--cached", "--name-only"); status != "" {
		t.Fatalf("real index was touched: %q", status)
	}
}

func TestTreeHashWithoutHead(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	writeFile(t, filepath.Join(dir, "a.txt"), "one\n")

	got, err := repo.TreeHash(ctx, dir)
	if err != nil {
		t.Fatalf("TreeHash without HEAD: %v", err)
	}
	if len(got) != 40 {
		t.Fatalf("tree hash = %q, want 40 hex chars", got)
	}
	runGit(t, dir, "add", "-A")
	if want := runGit(t, dir, "write-tree"); got != want {
		t.Fatalf("tree hash = %s, want %s", got, want)
	}
}

func TestChangedFiles(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	writeFile(t, filepath.Join(dir, "a.txt"), "one\n")
	writeFile(t, filepath.Join(dir, "b.txt"), "one\n")
	commitAll(t, dir)

	base, err := repo.TreeHash(ctx, dir)
	if err != nil {
		t.Fatalf("TreeHash base: %v", err)
	}
	writeFile(t, filepath.Join(dir, "a.txt"), "two\n")
	writeFile(t, filepath.Join(dir, "sub", "c.txt"), "new\n")
	final, err := repo.TreeHash(ctx, dir)
	if err != nil {
		t.Fatalf("TreeHash final: %v", err)
	}

	got, err := repo.ChangedFiles(ctx, dir, base, final)
	if err != nil {
		t.Fatalf("ChangedFiles: %v", err)
	}
	if want := []string{"a.txt", "sub/c.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ChangedFiles = %v, want %v", got, want)
	}

	same, err := repo.ChangedFiles(ctx, dir, base, base)
	if err != nil {
		t.Fatalf("ChangedFiles same: %v", err)
	}
	if len(same) != 0 {
		t.Fatalf("ChangedFiles same = %v, want empty", same)
	}
}

func TestChangedFilesInvalidTree(t *testing.T) {
	dir := initRepo(t)
	_, err := repo.ChangedFiles(context.Background(), dir, "deadbeef", "cafebabe")
	if err == nil || !strings.Contains(err.Error(), "git diff deadbeef..cafebabe:") {
		t.Fatalf("error = %v, want git diff message", err)
	}
}

func TestToplevelFromSubdirectory(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	sub := filepath.Join(dir, "x", "y")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	got, err := repo.Toplevel(ctx, sub)
	if err != nil {
		t.Fatalf("Toplevel: %v", err)
	}
	if got != want {
		t.Fatalf("Toplevel = %s, want %s", got, want)
	}
}

func TestCommonDirResolvesSymlinkedDir(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	viaReal, err := repo.CommonDir(ctx, dir)
	if err != nil {
		t.Fatalf("CommonDir real: %v", err)
	}
	viaLink, err := repo.CommonDir(ctx, link)
	if err != nil {
		t.Fatalf("CommonDir link: %v", err)
	}
	if viaReal != viaLink {
		t.Fatalf("CommonDir real = %s, link = %s", viaReal, viaLink)
	}
}

func TestHeadSHA(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	if _, err := repo.HeadSHA(ctx, dir); err == nil {
		t.Fatal("HeadSHA without commits: expected error")
	}
	writeFile(t, filepath.Join(dir, "a.txt"), "one\n")
	commitAll(t, dir)
	got, err := repo.HeadSHA(ctx, dir)
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}
	if want := runGit(t, dir, "rev-parse", "HEAD"); got != want {
		t.Fatalf("HeadSHA = %s, want %s", got, want)
	}
}
