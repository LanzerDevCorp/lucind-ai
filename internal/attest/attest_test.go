package attest_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/attest"
)

func TestKeyCreationAndLoading(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "attest.key")

	// First use: key is generated with mode 0600 and 32 bytes
	key1, err := attest.LoadOrCreateKey(keyPath)
	if err != nil {
		t.Fatalf("LoadOrCreateKey failed on first use: %v", err)
	}
	if len(key1) != 32 {
		t.Fatalf("expected 32 random bytes, got %d", len(key1))
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("expected key file mode 0600, got %04o", perm)
	}

	// Subsequent use: reads existing key
	key2, err := attest.LoadOrCreateKey(keyPath)
	if err != nil {
		t.Fatalf("LoadOrCreateKey failed on second use: %v", err)
	}
	if string(key1) != string(key2) {
		t.Fatalf("expected key to be stable across reads")
	}

	// Invalid key length error
	badPath := filepath.Join(tempDir, "bad.key")
	if err := os.WriteFile(badPath, []byte("short"), 0600); err != nil {
		t.Fatalf("write bad key: %v", err)
	}
	if _, err := attest.LoadOrCreateKey(badPath); err == nil {
		t.Fatalf("expected error loading key with invalid length, got nil")
	}
}

func TestMACGenerationAndVerification(t *testing.T) {
	key := []byte("01234567890123456789012345678901") // 32 bytes
	entry := attest.Entry{
		Version:    1,
		RepoID:     "deadbeefcafe0123456789abcdef0123456789abcdef0123456789abcdef0123",
		Command:    "go test ./...",
		ExitCode:   0,
		TreeHash:   "4b825dc642cb6eb9a060e54bf8d69288fbee4904",
		StartedAt:  "2026-10-03T00:00:00Z",
		FinishedAt: "2026-10-03T00:00:01Z",
	}

	mac := attest.ComputeMAC(entry, key)
	if mac == "" {
		t.Fatalf("expected non-empty MAC")
	}
	entry.MAC = mac

	if !attest.VerifyMAC(entry, key) {
		t.Fatalf("expected MAC to verify successfully")
	}

	// Tampered command
	tamperedCommand := entry
	tamperedCommand.Command = "go test ./cmd/..."
	if attest.VerifyMAC(tamperedCommand, key) {
		t.Fatalf("expected tampered command to fail MAC verification")
	}

	// Tampered tree hash
	tamperedTree := entry
	tamperedTree.TreeHash = "1111111111111111111111111111111111111111"
	if attest.VerifyMAC(tamperedTree, key) {
		t.Fatalf("expected tampered tree hash to fail MAC verification")
	}

	// Tampered exit code
	tamperedExit := entry
	tamperedExit.ExitCode = 1
	if attest.VerifyMAC(tamperedExit, key) {
		t.Fatalf("expected tampered exit code to fail MAC verification")
	}

	// Wrong key
	wrongKey := []byte("wrongkeywrongkeywrongkeywrongkey")
	if attest.VerifyMAC(entry, wrongKey) {
		t.Fatalf("expected wrong key to fail MAC verification")
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Tester",
			"GIT_AUTHOR_EMAIL=tester@example.com",
			"GIT_COMMITTER_NAME=Tester",
			"GIT_COMMITTER_EMAIL=tester@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v\nOutput: %s", strings.Join(args, " "), err, string(out))
		}
	}
	run("init")
	if err := os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatalf("write initial.txt: %v", err)
	}
	run("add", "initial.txt")
	run("commit", "-m", "initial commit")
}

func TestTreeHash(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	indexPath := filepath.Join(repoDir, ".git", "index")
	initialIndexBytes, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read index: %v", err)
	}

	hash1, err := attest.TreeHash(ctx, repoDir)
	if err != nil {
		t.Fatalf("TreeHash failed: %v", err)
	}
	if len(hash1) == 0 {
		t.Fatalf("expected non-empty tree hash")
	}

	// Check that real .git/index was not touched
	afterIndexBytes, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read index after hash1: %v", err)
	}
	if string(initialIndexBytes) != string(afterIndexBytes) {
		t.Fatalf("real .git/index was modified by TreeHash")
	}

	// Running TreeHash again without changes yields identical hash
	hash2, err := attest.TreeHash(ctx, repoDir)
	if err != nil {
		t.Fatalf("TreeHash second run failed: %v", err)
	}
	if hash1 != hash2 {
		t.Fatalf("expected hash1 == hash2, got %q vs %q", hash1, hash2)
	}

	// Untracked (non-ignored) file changes tree hash
	untrackedFile := filepath.Join(repoDir, "untracked.txt")
	if err := os.WriteFile(untrackedFile, []byte("untracked content\n"), 0644); err != nil {
		t.Fatalf("write untracked: %v", err)
	}
	hashUntracked, err := attest.TreeHash(ctx, repoDir)
	if err != nil {
		t.Fatalf("TreeHash failed with untracked file: %v", err)
	}
	if hashUntracked == hash1 {
		t.Fatalf("expected tree hash to change with untracked file")
	}

	// Deleting untracked file returns to hash1
	if err := os.Remove(untrackedFile); err != nil {
		t.Fatalf("remove untracked: %v", err)
	}
	hashBack, err := attest.TreeHash(ctx, repoDir)
	if err != nil {
		t.Fatalf("TreeHash failed after remove untracked: %v", err)
	}
	if hashBack != hash1 {
		t.Fatalf("expected tree hash to revert to hash1, got %q", hashBack)
	}

	// Modifying tracked file changes tree hash
	if err := os.WriteFile(filepath.Join(repoDir, "initial.txt"), []byte("modified\n"), 0644); err != nil {
		t.Fatalf("modify tracked: %v", err)
	}
	hashMod, err := attest.TreeHash(ctx, repoDir)
	if err != nil {
		t.Fatalf("TreeHash failed with modified tracked file: %v", err)
	}
	if hashMod == hash1 {
		t.Fatalf("expected tree hash to change with modified tracked file")
	}

	// Real .git/index was still not modified
	finalIndexBytes, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read index final: %v", err)
	}
	if string(initialIndexBytes) != string(finalIndexBytes) {
		t.Fatalf("real .git/index was modified during tree hash operations")
	}
}

func TestWriteAndReadEntries(t *testing.T) {
	logDir := t.TempDir()
	now := time.Now().UTC()
	entry1 := attest.Entry{
		Version:    1,
		RepoID:     "abcde12345",
		Command:    "go test ./...",
		ExitCode:   0,
		TreeHash:   "4b825dc642cb6eb9a060e54bf8d69288fbee4904",
		StartedAt:  now.Add(-2 * time.Second).Format(time.RFC3339Nano),
		FinishedAt: now.Add(-1 * time.Second).Format(time.RFC3339Nano),
		MAC:        "abcdef123456",
	}

	path1, err := attest.WriteEntry(logDir, entry1)
	if err != nil {
		t.Fatalf("WriteEntry failed: %v", err)
	}

	info, err := os.Stat(path1)
	if err != nil {
		t.Fatalf("stat written entry: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0444 {
		t.Fatalf("expected log entry file mode 0444, got %04o", perm)
	}

	// Verify filename structure: <finished_at unix nanos>-<tree_hash[:12]>.json
	base := filepath.Base(path1)
	if !strings.HasSuffix(base, "-4b825dc642cb.json") {
		t.Fatalf("expected filename to end with tree_hash[:12].json, got %q", base)
	}

	// Write second entry
	entry2 := attest.Entry{
		Version:    1,
		RepoID:     "abcde12345",
		Command:    "go test ./cmd/...",
		ExitCode:   1,
		TreeHash:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		StartedAt:  now.Add(-500 * time.Millisecond).Format(time.RFC3339Nano),
		FinishedAt: now.Format(time.RFC3339Nano),
		MAC:        "9876543210fe",
	}
	_, err = attest.WriteEntry(logDir, entry2)
	if err != nil {
		t.Fatalf("WriteEntry 2 failed: %v", err)
	}

	// Add unparseable and oversized files in logDir to ensure they are ignored
	badJSON := filepath.Join(logDir, "12345-bad.json")
	if err := os.WriteFile(badJSON, []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("write bad JSON: %v", err)
	}

	oversized := filepath.Join(logDir, "99999-oversized.json")
	bigData := make([]byte, 70*1024)
	if err := os.WriteFile(oversized, bigData, 0644); err != nil {
		t.Fatalf("write oversized file: %v", err)
	}

	nonJSON := filepath.Join(logDir, "other.txt")
	if err := os.WriteFile(nonJSON, []byte("hello"), 0644); err != nil {
		t.Fatalf("write non-JSON file: %v", err)
	}

	entries, err := attest.ReadEntries(logDir)
	if err != nil {
		t.Fatalf("ReadEntries failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 valid entries, got %d", len(entries))
	}
	// Sorted descending by FinishedAt: entry2 first, then entry1
	if entries[0].Command != entry2.Command || entries[1].Command != entry1.Command {
		t.Fatalf("entries not properly sorted by finished_at descending")
	}
}

func TestVerifyLogic(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	treeHash, err := attest.TreeHash(ctx, repoDir)
	if err != nil {
		t.Fatalf("TreeHash failed: %v", err)
	}

	repoID := attest.RepoID(repoDir)
	key := []byte("01234567890123456789012345678901")
	logDir := t.TempDir()

	cmdName := "go test ./..."

	// 1. No entry yet
	reason, err := attest.Verify(ctx, repoDir, cmdName, key, logDir)
	if err != nil {
		t.Fatalf("Verify unexpected error: %v", err)
	}
	if reason != attest.ReasonNoEntry {
		t.Fatalf("expected %q, got %q", attest.ReasonNoEntry, reason)
	}

	// Record a passing entry
	entry := attest.Entry{
		Version:    1,
		RepoID:     repoID,
		Command:    cmdName,
		ExitCode:   0,
		TreeHash:   treeHash,
		StartedAt:  time.Now().Add(-1 * time.Second).Format(time.RFC3339Nano),
		FinishedAt: time.Now().Format(time.RFC3339Nano),
	}
	entry.MAC = attest.ComputeMAC(entry, key)
	if _, err := attest.WriteEntry(logDir, entry); err != nil {
		t.Fatalf("WriteEntry failed: %v", err)
	}

	// 2. Passing entry verifies successfully
	reason, err = attest.Verify(ctx, repoDir, cmdName, key, logDir)
	if err != nil {
		t.Fatalf("Verify unexpected error: %v", err)
	}
	if reason != "" {
		t.Fatalf("expected success (reason=\"\"), got %q", reason)
	}

	// 3. Edit file in repo -> tree changed
	modFile := filepath.Join(repoDir, "initial.txt")
	if err := os.WriteFile(modFile, []byte("changed!\n"), 0644); err != nil {
		t.Fatalf("write modFile: %v", err)
	}
	reason, err = attest.Verify(ctx, repoDir, cmdName, key, logDir)
	if err != nil {
		t.Fatalf("Verify unexpected error: %v", err)
	}
	if reason != attest.ReasonTreeChanged {
		t.Fatalf("expected %q, got %q", attest.ReasonTreeChanged, reason)
	}

	// Revert file change
	if err := os.WriteFile(modFile, []byte("hello\n"), 0644); err != nil {
		t.Fatalf("revert modFile: %v", err)
	}

	// 4. Failed command entry (exit code 1)
	logDirFailed := t.TempDir()
	entryFailed := attest.Entry{
		Version:    1,
		RepoID:     repoID,
		Command:    cmdName,
		ExitCode:   1,
		TreeHash:   treeHash,
		StartedAt:  time.Now().Add(-1 * time.Second).Format(time.RFC3339Nano),
		FinishedAt: time.Now().Format(time.RFC3339Nano),
	}
	entryFailed.MAC = attest.ComputeMAC(entryFailed, key)
	if _, err := attest.WriteEntry(logDirFailed, entryFailed); err != nil {
		t.Fatalf("WriteEntry failed: %v", err)
	}
	reason, err = attest.Verify(ctx, repoDir, cmdName, key, logDirFailed)
	if err != nil {
		t.Fatalf("Verify unexpected error: %v", err)
	}
	if reason != attest.ReasonTestsFailed {
		t.Fatalf("expected %q, got %q", attest.ReasonTestsFailed, reason)
	}

	// 5. Tampered entry (bad mac)
	logDirBadMAC := t.TempDir()
	entryBadMAC := entry
	entryBadMAC.MAC = "deadbeef00000000000000000000000000000000000000000000000000000000"
	if _, err := attest.WriteEntry(logDirBadMAC, entryBadMAC); err != nil {
		t.Fatalf("WriteEntry failed: %v", err)
	}
	reason, err = attest.Verify(ctx, repoDir, cmdName, key, logDirBadMAC)
	if err != nil {
		t.Fatalf("Verify unexpected error: %v", err)
	}
	if reason != attest.ReasonBadMAC {
		t.Fatalf("expected %q, got %q", attest.ReasonBadMAC, reason)
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		os.Exit(2)
	}
	switch args[0] {
	case "create-key":
		keyPath := os.Getenv("ATTEST_KEY_PATH")
		key, err := attest.LoadOrCreateKey(keyPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "LoadOrCreateKey failed: %v\n", err)
			os.Exit(1)
		}
		if len(key) != 32 {
			fmt.Fprintf(os.Stderr, "invalid key length: %d\n", len(key))
			os.Exit(3)
		}
		os.Stdout.Write(key)
		os.Exit(0)
	default:
		os.Exit(2)
	}
}

func TestLoadOrCreateKey_Race(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "race.key")

	const numProcs = 8
	type result struct {
		key []byte
		err error
	}
	results := make([]result, numProcs)
	var wg sync.WaitGroup
	wg.Add(numProcs)

	for i := 0; i < numProcs; i++ {
		idx := i
		go func() {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", "create-key")
			cmd.Env = append(os.Environ(),
				"GO_WANT_HELPER_PROCESS=1",
				"ATTEST_KEY_PATH="+keyPath,
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				results[idx] = result{err: fmt.Errorf("proc %d failed: %w: %s", idx, err, string(out))}
				return
			}
			results[idx] = result{key: out}
		}()
	}
	wg.Wait()

	for i, res := range results {
		if res.err != nil {
			t.Fatalf("process %d failed: %v", i, res.err)
		}
		if len(res.key) != 32 {
			t.Fatalf("process %d expected 32-byte key, got %d", i, len(res.key))
		}
		if !bytes.Equal(res.key, results[0].key) {
			t.Fatalf("process %d key does not match process 0 key", i)
		}
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}
	if info.Size() != 32 {
		t.Fatalf("expected key file size 32, got %d", info.Size())
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("expected key file mode 0600, got %04o", perm)
	}
}

func TestWriteEntryModeBeforeRename(t *testing.T) {
	logDir := t.TempDir()
	now := time.Now().UTC()
	entry := attest.Entry{
		Version:    1,
		RepoID:     "repoid123",
		Command:    "go test ./...",
		ExitCode:   0,
		TreeHash:   "4b825dc642cb6eb9a060e54bf8d69288fbee4904",
		StartedAt:  now.Add(-1 * time.Second).Format(time.RFC3339Nano),
		FinishedAt: now.Format(time.RFC3339Nano),
		MAC:        "abcdef123456",
	}

	finalPath, err := attest.WriteEntry(logDir, entry)
	if err != nil {
		t.Fatalf("WriteEntry failed: %v", err)
	}

	// Verify final file is mode 0444
	info, err := os.Stat(finalPath)
	if err != nil {
		t.Fatalf("stat final file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0444 {
		t.Fatalf("expected mode 0444, got %04o", perm)
	}

	// Verify no temporary files remain
	entries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("readdir logDir: %v", err)
	}
	for _, de := range entries {
		if strings.HasSuffix(de.Name(), ".tmp") {
			t.Errorf("leftover temporary file found: %s", de.Name())
		}
	}
}

func TestRepoID_WorktreeConsistency(t *testing.T) {
	ctx := context.Background()
	primaryDir := t.TempDir()
	initGitRepo(t, primaryDir)

	worktreeDir := filepath.Join(t.TempDir(), "wt")
	cmd := exec.Command("git", "-C", primaryDir, "worktree", "add", worktreeDir, "HEAD")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add failed: %v: %s", err, string(out))
	}

	otherDir := t.TempDir()
	initGitRepo(t, otherDir)

	commonPrimary, err := attest.RepoCommonDir(ctx, primaryDir)
	if err != nil {
		t.Fatalf("RepoCommonDir(primary) error: %v", err)
	}
	commonWorktree, err := attest.RepoCommonDir(ctx, worktreeDir)
	if err != nil {
		t.Fatalf("RepoCommonDir(worktree) error: %v", err)
	}
	commonOther, err := attest.RepoCommonDir(ctx, otherDir)
	if err != nil {
		t.Fatalf("RepoCommonDir(other) error: %v", err)
	}

	idPrimary := attest.RepoID(commonPrimary)
	idWorktree := attest.RepoID(commonWorktree)
	idOther := attest.RepoID(commonOther)

	if idPrimary != idWorktree {
		t.Fatalf("expected RepoID to match between primary and worktree: %q vs %q", idPrimary, idWorktree)
	}
	if idPrimary == idOther {
		t.Fatalf("expected RepoID to differ for different repository: %q", idPrimary)
	}
}

func TestHasValidAttestation(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	treeHash, err := attest.TreeHash(ctx, repoDir)
	if err != nil {
		t.Fatalf("TreeHash failed: %v", err)
	}

	logDir := t.TempDir()
	key := []byte("01234567890123456789012345678901")
	command := "sh lucind-checks.sh"

	// 1. No attestation yet
	valid, err := attest.HasValidAttestation(ctx, repoDir, command, treeHash, key, logDir)
	if err != nil {
		t.Fatalf("HasValidAttestation error: %v", err)
	}
	if valid {
		t.Fatalf("expected valid=false for empty logDir")
	}

	// Write passing entry
	entry := attest.Entry{
		Version:    1,
		RepoID:     "somerepoid",
		Command:    command,
		ExitCode:   0,
		TreeHash:   treeHash,
		StartedAt:  time.Now().Add(-1 * time.Second).Format(time.RFC3339Nano),
		FinishedAt: time.Now().Format(time.RFC3339Nano),
	}
	entry.MAC = attest.ComputeMAC(entry, key)
	if _, err := attest.WriteEntry(logDir, entry); err != nil {
		t.Fatalf("WriteEntry failed: %v", err)
	}

	// 2. Passing entry matches
	valid, err = attest.HasValidAttestation(ctx, repoDir, command, treeHash, key, logDir)
	if err != nil {
		t.Fatalf("HasValidAttestation error: %v", err)
	}
	if !valid {
		t.Fatalf("expected valid=true for matching entry")
	}

	// 3. Different command
	valid, err = attest.HasValidAttestation(ctx, repoDir, "sh other.sh", treeHash, key, logDir)
	if err != nil {
		t.Fatalf("HasValidAttestation error: %v", err)
	}
	if valid {
		t.Fatalf("expected valid=false for different command")
	}

	// 4. Different tree hash
	valid, err = attest.HasValidAttestation(ctx, repoDir, command, "0000000000000000000000000000000000000000", key, logDir)
	if err != nil {
		t.Fatalf("HasValidAttestation error: %v", err)
	}
	if valid {
		t.Fatalf("expected valid=false for different tree hash")
	}

	// 5. Tampered MAC
	tamperedLogDir := t.TempDir()
	tamperedEntry := entry
	tamperedEntry.MAC = "deadbeef"
	if _, err := attest.WriteEntry(tamperedLogDir, tamperedEntry); err != nil {
		t.Fatalf("WriteEntry failed: %v", err)
	}
	valid, err = attest.HasValidAttestation(ctx, repoDir, command, treeHash, key, tamperedLogDir)
	if err != nil {
		t.Fatalf("HasValidAttestation error: %v", err)
	}
	if valid {
		t.Fatalf("expected valid=false for tampered MAC")
	}

	// 6. Failing exit code
	failingLogDir := t.TempDir()
	failingEntry := entry
	failingEntry.ExitCode = 1
	failingEntry.MAC = attest.ComputeMAC(failingEntry, key)
	if _, err := attest.WriteEntry(failingLogDir, failingEntry); err != nil {
		t.Fatalf("WriteEntry failed: %v", err)
	}
	valid, err = attest.HasValidAttestation(ctx, repoDir, command, treeHash, key, failingLogDir)
	if err != nil {
		t.Fatalf("HasValidAttestation error: %v", err)
	}
	if valid {
		t.Fatalf("expected valid=false for failing entry")
	}
}

func TestLoadOrCreateKey_PartialKeyNeverPublished(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg", "attest.key")
	key, err := attest.LoadOrCreateKey(path)
	if err != nil {
		t.Fatalf("LoadOrCreateKey error: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "attest.key" {
		t.Fatalf("expected only the final key file, got %v", entries)
	}
	again, err := attest.LoadOrCreateKey(path)
	if err != nil || !bytes.Equal(key, again) {
		t.Fatalf("second load must return the same key, err=%v", err)
	}
}

func TestLoadOrCreateKey_RejectsShortKeyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "attest.key")
	if err := os.WriteFile(path, []byte("short"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := attest.LoadOrCreateKey(path); err == nil {
		t.Fatal("expected an error for a key file that is not 32 bytes")
	}
}

func TestRepoCommonDir_ResolvesSymlinkedDir(t *testing.T) {
	ctx := context.Background()
	real := t.TempDir()
	initGitRepo(t, real)
	link := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	viaReal, err := attest.RepoCommonDir(ctx, real)
	if err != nil {
		t.Fatal(err)
	}
	viaLink, err := attest.RepoCommonDir(ctx, link)
	if err != nil {
		t.Fatal(err)
	}
	if viaReal != viaLink {
		t.Fatalf("symlinked dir must resolve to the same common dir: %q vs %q", viaReal, viaLink)
	}
}

func TestHasValidAttestation_RejectsForeignRepoID(t *testing.T) {
	ctx := context.Background()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	repo := t.TempDir()
	initGitRepo(t, repo)
	commonDir, err := attest.RepoCommonDir(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	logDir, err := attest.ResolveStateDir(attest.RepoID(commonDir))
	if err != nil {
		t.Fatal(err)
	}
	key, err := attest.LoadOrCreateKey("")
	if err != nil {
		t.Fatal(err)
	}
	e := attest.Entry{
		Version: 1, RepoID: "someone-elses-repo", Command: "sh lucind-checks.sh", ExitCode: 0,
		TreeHash: "abc", StartedAt: time.Now().UTC().Format(time.RFC3339Nano), FinishedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	e.MAC = attest.ComputeMAC(e, key)
	if _, err := attest.WriteEntry(logDir, e); err != nil {
		t.Fatal(err)
	}
	ok, err := attest.HasValidAttestation(ctx, repo, "sh lucind-checks.sh", "abc", key, "")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("an entry carrying another repository's RepoID must not count as an attestation")
	}
}
