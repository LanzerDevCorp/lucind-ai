// Package attest provides deterministic, tamper-evident test attestation.
package attest

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/repo"
)

// Reason constants for attest verify failure.
const (
	ReasonNoEntry     = "no entry"
	ReasonTreeChanged = "tree changed"
	ReasonTestsFailed = "tests failed"
	ReasonBadMAC      = "bad mac"
)

const maxEntryFileSize = 64 * 1024 // 64 KB

// Entry represents a signed test attestation record.
type Entry struct {
	Version    int    `json:"version"`
	RepoID     string `json:"repo_id"`
	Command    string `json:"command"`
	ExitCode   int    `json:"exit_code"`
	TreeHash   string `json:"tree_hash"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
	MAC        string `json:"mac"`
}

// CanonicalPayload returns the canonical byte representation of the entry fields
// used for HMAC generation and verification.
func CanonicalPayload(e Entry) []byte {
	var buf bytes.Buffer
	writeString := func(s string) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(s)))
		buf.Write(size[:])
		buf.WriteString(s)
	}
	writeInt64 := func(n int64) {
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], uint64(n))
		buf.Write(b[:])
	}

	writeString("attest:v1")
	writeInt64(int64(e.Version))
	writeString(e.RepoID)
	writeString(e.Command)
	writeInt64(int64(e.ExitCode))
	writeString(e.TreeHash)
	writeString(e.StartedAt)
	writeString(e.FinishedAt)
	return buf.Bytes()
}

// ComputeMAC computes the HMAC-SHA256 hex string over the canonical encoding of the entry.
func ComputeMAC(e Entry, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(CanonicalPayload(e))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyMAC checks whether the entry's MAC matches the computed HMAC-SHA256 using the key.
func VerifyMAC(e Entry, key []byte) bool {
	expectedMAC := ComputeMAC(e, key)
	gotBytes, err1 := hex.DecodeString(e.MAC)
	wantBytes, err2 := hex.DecodeString(expectedMAC)
	if err1 != nil || err2 != nil {
		return false
	}
	return hmac.Equal(gotBytes, wantBytes)
}

// ResolveKeyPath resolves the path to the attestation key file:
// $XDG_CONFIG_HOME/lucind-ai/attest.key, fallback ~/.config/lucind-ai/attest.key.
func ResolveKeyPath() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "lucind-ai", "attest.key"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, ".config", "lucind-ai", "attest.key"), nil
}

// LoadOrCreateKey loads a 32-byte secret key from path, generating and storing
// 32 random bytes with mode 0600 on first use if it does not exist.
// The key is written in full to a private temp file and published with os.Link,
// which fails with EEXIST if another process won the race. Readers therefore only
// ever observe a complete 32-byte key, and a creator that dies mid-write leaves
// no partial key under the final name.
func LoadOrCreateKey(path string) ([]byte, error) {
	if path == "" {
		var err error
		path, err = ResolveKeyPath()
		if err != nil {
			return nil, err
		}
	}

	data, err := os.ReadFile(path)
	if err == nil {
		return checkKeyLength(data)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read key file: %w", err)
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate random key: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create key directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".attest.key.tmp-*")
	if err != nil {
		return nil, fmt.Errorf("create temp key file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("chmod temp key file: %w", err)
	}
	if _, err := tmp.Write(key); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("write temp key file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("close temp key file: %w", err)
	}

	if err := os.Link(tmpName, path); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("publish key file: %w", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read key file: %w", err)
		}
		return checkKeyLength(data)
	}
	return key, nil
}

func checkKeyLength(data []byte) ([]byte, error) {
	if len(data) != 32 {
		return nil, fmt.Errorf("invalid key file length: expected 32 bytes, got %d", len(data))
	}
	return data, nil
}

// RepoID returns the sha256 hex string of the absolute repository git common directory path.
func RepoID(commonDir string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(commonDir)))
	return hex.EncodeToString(sum[:])
}

// FindValidAttestation looks for a valid attestation entry matching command and expectedTreeHash signed with key.
// If key is empty, it loads the key via LoadOrCreateKey("").
// If logDir is empty, it resolves logDir via RepoCommonDir and ResolveStateDir for repoRoot.
// It returns (entry, path, true, nil) if a valid attestation matching command, expectedTreeHash, exit code 0,
// and valid MAC is found, or (Entry{}, "", false, nil) if no valid attestation exists.
func FindValidAttestation(ctx context.Context, repoRoot, command, expectedTreeHash string, key []byte, logDir string) (Entry, string, bool, error) {
	if len(key) == 0 {
		var err error
		key, err = LoadOrCreateKey("")
		if err != nil {
			return Entry{}, "", false, fmt.Errorf("load attestation key: %w", err)
		}
	}
	var wantRepoID string
	if logDir == "" {
		commonDir, err := repo.CommonDir(ctx, repoRoot)
		if err != nil {
			return Entry{}, "", false, fmt.Errorf("resolve repo common dir: %w", err)
		}
		wantRepoID = RepoID(commonDir)
		logDir, err = ResolveStateDir(wantRepoID)
		if err != nil {
			return Entry{}, "", false, fmt.Errorf("resolve log dir: %w", err)
		}
	}

	items, err := readEntriesWithPaths(logDir)
	if err != nil {
		return Entry{}, "", false, fmt.Errorf("read entries: %w", err)
	}

	for _, item := range items {
		e := item.Entry
		if wantRepoID != "" && e.RepoID != wantRepoID {
			continue
		}
		if e.Command == command && e.TreeHash == expectedTreeHash && e.ExitCode == 0 {
			if VerifyMAC(e, key) {
				return e, item.Path, true, nil
			}
		}
	}
	return Entry{}, "", false, nil
}

// HasValidAttestation checks if logDir contains a valid attestation entry matching command
// and expectedTreeHash signed with key.
// If key is empty, it loads the key via LoadOrCreateKey("").
// If logDir is empty, it resolves logDir via RepoCommonDir and ResolveStateDir for repoRoot.
// It returns (true, nil) if a valid attestation matching command, expectedTreeHash, exit code 0,
// and valid MAC is found, or (false, nil) if no valid attestation exists.
func HasValidAttestation(ctx context.Context, repoRoot, command, expectedTreeHash string, key []byte, logDir string) (bool, error) {
	_, _, found, err := FindValidAttestation(ctx, repoRoot, command, expectedTreeHash, key, logDir)
	return found, err
}

// ResolveStateDir resolves the directory for attestation logs for repoID:
// $XDG_STATE_HOME/lucind-ai/attestations/<repo_id>/, fallback ~/.local/state/lucind-ai/attestations/<repo_id>/.
func ResolveStateDir(repoID string) (string, error) {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "lucind-ai", "attestations", repoID), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, ".local", "state", "lucind-ai", "attestations", repoID), nil
}

// WriteEntry writes an attestation entry atomically to dir, naming it
// <finished_at unix nanos>-<tree_hash[:12]>.json and setting permissions to 0444.
func WriteEntry(dir string, e Entry) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create log directory: %w", err)
	}

	t, err := parseTime(e.FinishedAt)
	if err != nil {
		t = time.Now().UTC()
	}
	nanos := t.UnixNano()
	prefixLen := 12
	if len(e.TreeHash) < 12 {
		prefixLen = len(e.TreeHash)
	}
	filename := fmt.Sprintf("%d-%s.json", nanos, e.TreeHash[:prefixLen])
	finalPath := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal entry: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, "attest-entry-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temp log file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("write temp log file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("close temp log file: %w", err)
	}

	if err := os.Chmod(tmpName, 0444); err != nil {
		return "", fmt.Errorf("chmod temp log file: %w", err)
	}
	if err := os.Rename(tmpName, finalPath); err != nil {
		return "", fmt.Errorf("rename log file: %w", err)
	}

	return finalPath, nil
}

// EntryWithPath associates an Entry with its path on disk.
type EntryWithPath struct {
	Entry Entry
	Path  string
}

func readEntriesWithPaths(dir string) ([]EntryWithPath, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read log directory: %w", err)
	}

	cleanDir := filepath.Clean(dir)
	var result []EntryWithPath
	for _, de := range entries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}

		filePath := filepath.Join(cleanDir, name)
		cleanPath := filepath.Clean(filePath)
		rel, err := filepath.Rel(cleanDir, cleanPath)
		if err != nil || strings.HasPrefix(rel, "..") || filepath.Base(filePath) != name {
			continue
		}

		info, err := de.Info()
		if err != nil {
			continue
		}
		if info.Size() > maxEntryFileSize || info.Size() == 0 {
			continue
		}

		f, err := os.Open(cleanPath)
		if err != nil {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(f, maxEntryFileSize+1))
		_ = f.Close()
		if err != nil || int64(len(data)) > maxEntryFileSize {
			continue
		}

		var entry Entry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}
		if entry.Version == 0 || entry.Command == "" || entry.TreeHash == "" || entry.MAC == "" {
			continue
		}
		result = append(result, EntryWithPath{Entry: entry, Path: cleanPath})
	}

	sort.Slice(result, func(i, j int) bool {
		ti, erri := parseTime(result[i].Entry.FinishedAt)
		tj, errj := parseTime(result[j].Entry.FinishedAt)
		if erri == nil && errj == nil {
			return ti.After(tj)
		}
		return result[i].Entry.FinishedAt > result[j].Entry.FinishedAt
	})

	return result, nil
}

// ReadEntries reads all valid attestation entry JSON files from dir, rejecting
// path traversal and oversized files, and ignoring unparseable files.
// Returned entries are sorted descending by FinishedAt.
func ReadEntries(dir string) ([]Entry, error) {
	items, err := readEntriesWithPaths(dir)
	if err != nil {
		return nil, err
	}
	result := make([]Entry, len(items))
	for i, it := range items {
		result[i] = it.Entry
	}
	return result, nil
}

// Verify verifies an attestation for command in repoRoot.
// Returns "", nil on success.
// On failure, returns reason, nil where reason is one of:
// ReasonNoEntry ("no entry"), ReasonTreeChanged ("tree changed"),
// ReasonTestsFailed ("tests failed"), ReasonBadMAC ("bad mac").
// Other non-verification errors (e.g. git failure) return "", err.
func Verify(ctx context.Context, repoRoot, command string, key []byte, logDir string) (string, error) {
	currentTreeHash, err := repo.TreeHash(ctx, repoRoot)
	if err != nil {
		return "", fmt.Errorf("tree hash: %w", err)
	}

	entries, err := ReadEntries(logDir)
	if err != nil {
		return "", fmt.Errorf("read entries: %w", err)
	}

	var matchingCommand []Entry
	for _, e := range entries {
		if e.Command == command {
			matchingCommand = append(matchingCommand, e)
		}
	}

	if len(matchingCommand) == 0 {
		return ReasonNoEntry, nil
	}

	// Check if any entry matches current tree hash
	var matchedTree *Entry
	for i := range matchingCommand {
		if matchingCommand[i].TreeHash == currentTreeHash {
			matchedTree = &matchingCommand[i]
			break
		}
	}

	if matchedTree != nil {
		if !VerifyMAC(*matchedTree, key) {
			return ReasonBadMAC, nil
		}
		if matchedTree.ExitCode != 0 {
			return ReasonTestsFailed, nil
		}
		return "", nil // Success!
	}

	// No entry matches the current tree hash; check latest entry for this command
	latest := matchingCommand[0]
	if !VerifyMAC(latest, key) {
		return ReasonBadMAC, nil
	}
	return ReasonTreeChanged, nil
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

// RunAndRecord executes argv in dir, computes the TreeHash of the repository toplevel,
// builds and MACs an attestation entry, and writes it to the attestation log directory.
// Non-zero command exits are recorded in Entry.ExitCode and do not return a Go error.
func RunAndRecord(ctx context.Context, dir string, argv []string, commandString string, stdin io.Reader, stdout, stderr io.Writer) (Entry, error) {
	if len(argv) == 0 {
		return Entry{}, errors.New("command is required")
	}

	toplevel, err := repo.Toplevel(ctx, dir)
	if err != nil {
		return Entry{}, fmt.Errorf("resolve repository toplevel: %w", err)
	}
	commonDir, err := repo.CommonDir(ctx, dir)
	if err != nil {
		return Entry{}, fmt.Errorf("resolve repository common dir: %w", err)
	}
	repoID := RepoID(commonDir)

	key, err := LoadOrCreateKey("")
	if err != nil {
		return Entry{}, fmt.Errorf("load attestation key: %w", err)
	}

	logDir, err := ResolveStateDir(repoID)
	if err != nil {
		return Entry{}, fmt.Errorf("resolve attestation log dir: %w", err)
	}

	if commandString == "" {
		commandString = strings.Join(argv, " ")
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	startedAt := time.Now().UTC()
	runErr := cmd.Run()
	finishedAt := time.Now().UTC()

	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return Entry{}, fmt.Errorf("run command: %w", runErr)
		}
	}

	treeHash, err := repo.TreeHash(ctx, toplevel)
	if err != nil {
		return Entry{}, fmt.Errorf("compute tree hash: %w", err)
	}

	entry := Entry{
		Version:    1,
		RepoID:     repoID,
		Command:    commandString,
		ExitCode:   exitCode,
		TreeHash:   treeHash,
		StartedAt:  startedAt.Format(time.RFC3339Nano),
		FinishedAt: finishedAt.Format(time.RFC3339Nano),
	}
	entry.MAC = ComputeMAC(entry, key)

	if _, err := WriteEntry(logDir, entry); err != nil {
		return Entry{}, fmt.Errorf("write attestation entry: %w", err)
	}

	return entry, nil
}
