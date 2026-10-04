package lane_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
)

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

func TestLaneIDFormat(t *testing.T) {
	// D3: lane id = YYYYMMDD-HHMMSS-<4 lowercase hex> (UTC), e.g. 20261003-215144-a1b2
	id, err := lane.GenerateID()
	if err != nil {
		t.Fatalf("GenerateID failed: %v", err)
	}

	pattern := regexp.MustCompile(`^[0-9]{8}-[0-9]{6}-[0-9a-f]{4}$`)
	if !pattern.MatchString(id) {
		t.Fatalf("GenerateID() = %q, does not match expected format YYYYMMDD-HHMMSS-<4 lowercase hex>", id)
	}

	if !lane.ValidateID(id) {
		t.Errorf("ValidateID(%q) = false, want true", id)
	}

	// Test deterministic timestamp generation
	fixedTime := time.Date(2026, 10, 3, 21, 51, 44, 0, time.UTC)
	fixedID, err := lane.GenerateIDAt(fixedTime)
	if err != nil {
		t.Fatalf("GenerateIDAt failed: %v", err)
	}
	if !strings.HasPrefix(fixedID, "20261003-215144-") {
		t.Errorf("GenerateIDAt(%v) = %q, want prefix '20261003-215144-'", fixedTime, fixedID)
	}
	if len(fixedID) != 20 {
		t.Errorf("GenerateIDAt length = %d, want 20", len(fixedID))
	}

	// ValidateID rejection cases
	invalidIDs := []string{
		"",
		"20261003-215144-A1B2",  // uppercase hex
		"20261003-215144-123",   // 3 hex digits
		"20261003-215144-12345", // 5 hex digits
		"20261003215144-a1b2",   // missing first dash
		"20261003-215144_a1b2",  // underscore instead of dash
		"not-a-lane-id",
	}
	for _, inv := range invalidIDs {
		if lane.ValidateID(inv) {
			t.Errorf("ValidateID(%q) = true, want false", inv)
		}
	}

	// Uniqueness
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		id, err := lane.GenerateID()
		if err != nil {
			t.Fatalf("GenerateID failed on iteration %d: %v", i, err)
		}
		if seen[id] {
			t.Fatalf("duplicate lane ID generated: %q", id)
		}
		seen[id] = true
	}
}

func TestPathHelpers(t *testing.T) {
	root := "/test/repo"
	id := "20261003-215144-a1b2"

	wantDir := filepath.Join(root, ".lucind", "lanes", id)
	if got := lane.LaneDir(root, id); got != wantDir {
		t.Errorf("LaneDir = %q, want %q", got, wantDir)
	}

	wantLanePath := filepath.Join(wantDir, "lane.json")
	if got := lane.LanePath(root, id); got != wantLanePath {
		t.Errorf("LanePath = %q, want %q", got, wantLanePath)
	}

	wantReceiptPath := filepath.Join(wantDir, "receipt.json")
	if got := lane.ReceiptPath(root, id); got != wantReceiptPath {
		t.Errorf("ReceiptPath = %q, want %q", got, wantReceiptPath)
	}

	wantResultPath := filepath.Join(wantDir, "result.json")
	if got := lane.ResultPath(root, id); got != wantResultPath {
		t.Errorf("ResultPath = %q, want %q", got, wantResultPath)
	}
}

func TestStoreCreateLoadSave(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	allow := []string{"cmd/**", "internal/**"}
	model := "gemini-3.8-flash"

	ln, err := lane.Create(ctx, repoDir, allow, model)
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}

	if ln.Version != 1 {
		t.Errorf("lane.Version = %d, want 1", ln.Version)
	}
	if !lane.ValidateID(ln.ID) {
		t.Errorf("lane.ID %q is invalid", ln.ID)
	}
	if ln.BaseTree == "" {
		t.Errorf("lane.BaseTree is empty")
	}
	if ln.Model != model {
		t.Errorf("lane.Model = %q, want %q", ln.Model, model)
	}
	if ln.Status != lane.StatusRunning {
		t.Errorf("lane.Status = %q, want %q", ln.Status, lane.StatusRunning)
	}
	if ln.Retries != 0 {
		t.Errorf("lane.Retries = %d, want 0", ln.Retries)
	}
	if ln.CreatedAt.IsZero() || ln.UpdatedAt.IsZero() {
		t.Errorf("lane timestamps must not be zero: created=%v updated=%v", ln.CreatedAt, ln.UpdatedAt)
	}

	// Verify file exists on disk
	laneFile := lane.LanePath(repoDir, ln.ID)
	if _, err := os.Stat(laneFile); err != nil {
		t.Fatalf("lane.json stat error: %v", err)
	}

	// Load lane back
	loaded, err := lane.Load(repoDir, ln.ID)
	if err != nil {
		t.Fatalf("lane.Load failed: %v", err)
	}
	if loaded.ID != ln.ID {
		t.Errorf("loaded.ID = %q, want %q", loaded.ID, ln.ID)
	}
	if loaded.BaseTree != ln.BaseTree {
		t.Errorf("loaded.BaseTree = %q, want %q", loaded.BaseTree, ln.BaseTree)
	}
	if loaded.Status != lane.StatusRunning {
		t.Errorf("loaded.Status = %q, want %q", loaded.Status, lane.StatusRunning)
	}
	if len(loaded.Allow) != len(allow) {
		t.Fatalf("loaded.Allow length = %d, want %d", len(loaded.Allow), len(allow))
	}

	// Save modified lane
	loaded.Status = lane.StatusDone
	loaded.Retries = 2
	loaded.PaneID = "pane-123"
	if err := lane.Save(repoDir, loaded); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	reloaded, err := lane.Load(repoDir, ln.ID)
	if err != nil {
		t.Fatalf("lane.Load after save failed: %v", err)
	}
	if reloaded.Status != lane.StatusDone {
		t.Errorf("reloaded.Status = %q, want %q", reloaded.Status, lane.StatusDone)
	}
	if reloaded.Retries != 2 {
		t.Errorf("reloaded.Retries = %d, want 2", reloaded.Retries)
	}
	if reloaded.PaneID != "pane-123" {
		t.Errorf("reloaded.PaneID = %q, want %q", reloaded.PaneID, "pane-123")
	}
}

func TestStoreSubdirectoryCwd(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	subDir := filepath.Join(repoDir, "a", "b", "c")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	ln, err := lane.Create(ctx, subDir, []string{"*"}, "test-model")
	if err != nil {
		t.Fatalf("Create from subDir failed: %v", err)
	}

	// Lane must be created in repoRoot/.lucind/lanes/<id>/lane.json
	expectedLaneFile := filepath.Join(repoDir, ".lucind", "lanes", ln.ID, "lane.json")
	if _, err := os.Stat(expectedLaneFile); err != nil {
		t.Fatalf("lane.json not found at repo root path: %v", err)
	}

	loaded, err := lane.Load(repoDir, ln.ID)
	if err != nil {
		t.Fatalf("Load from repoDir failed: %v", err)
	}
	if loaded.ID != ln.ID {
		t.Errorf("loaded.ID = %q, want %q", loaded.ID, ln.ID)
	}
}

func TestStoreNilAllowSerializedAsArray(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	ln, err := lane.Create(ctx, repoDir, nil, "model")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	data, err := os.ReadFile(lane.LanePath(repoDir, ln.ID))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	allowVal, ok := raw["allow"].([]any)
	if !ok || allowVal == nil {
		t.Errorf("expected allow to serialize as non-nil [] array in JSON, got %v", raw["allow"])
	}
}

func TestStoreReceiverSave(t *testing.T) {
	tempDir := t.TempDir()
	id := "20261003-215144-recv"

	ln := lane.Lane{
		Version: 1,
		ID:      id,
		Status:  lane.StatusRunning,
	}

	if err := ln.Save(tempDir); err != nil {
		t.Fatalf("ln.Save failed: %v", err)
	}

	loaded, err := lane.Load(tempDir, id)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.ID != id {
		t.Errorf("loaded.ID = %q, want %q", loaded.ID, id)
	}
}

func TestStoreErrorHandling(t *testing.T) {
	tempDir := t.TempDir()

	// Load non-existent lane
	if _, err := lane.Load(tempDir, "nonexistent"); err == nil {
		t.Errorf("Load on nonexistent lane should return error")
	}

	// LoadReceipt non-existent receipt
	if _, err := lane.LoadReceipt(tempDir, "nonexistent"); err == nil {
		t.Errorf("LoadReceipt on nonexistent receipt should return error")
	}

	// Save with empty ID
	if err := lane.Save(tempDir, lane.Lane{}); err == nil {
		t.Errorf("Save with empty ID should return error")
	}

	// WriteReceipt with empty ID
	if err := lane.WriteReceipt(tempDir, "", lane.Receipt{}); err == nil {
		t.Errorf("WriteReceipt with empty ID should return error")
	}
}

func TestAtomicWrite(t *testing.T) {
	tempDir := t.TempDir()
	id := "20261003-215144-atom"

	ln := lane.Lane{
		Version:   1,
		ID:        id,
		Cwd:       tempDir,
		BaseTree:  "abc1234",
		Allow:     []string{"*"},
		Model:     "test-model",
		Status:    lane.StatusRunning,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := lane.Save(tempDir, ln); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify lane dir has no left-over temporary files (.tmp-*)
	laneDir := lane.LaneDir(tempDir, id)
	entries, err := os.ReadDir(laneDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tmp-") {
			t.Errorf("found leftover temp file in lane dir: %s", entry.Name())
		}
	}

	// Verify content is valid formatted JSON
	data, err := os.ReadFile(lane.LanePath(tempDir, id))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("lane.json is not valid JSON: %v", err)
	}
	if raw["version"] != float64(1) {
		t.Errorf("version = %v, want 1", raw["version"])
	}
	if raw["id"] != id {
		t.Errorf("id = %v, want %s", raw["id"], id)
	}
}

func TestReceiptWriting(t *testing.T) {
	tempDir := t.TempDir()
	laneID := "20261003-215144-rcpt"

	attestationPath := "/path/to/attestation.json"
	receipt := lane.Receipt{
		Version:      1,
		Lane:         laneID,
		FinalTree:    "finaltree123",
		BaseTree:     "basetree123",
		ChangedFiles: []string{"cmd/lucind-ai/main.go"},
		Verdict:      lane.VerdictAccepted,
		Reasons:      []string{"all checks passed"},
		Evidence: lane.Evidence{
			Attestation: &attestationPath,
			CheckLog:    nil,
		},
		CreatedAt: time.Now().UTC(),
	}

	if err := lane.WriteReceipt(tempDir, laneID, receipt); err != nil {
		t.Fatalf("WriteReceipt failed: %v", err)
	}

	// Verify raw JSON serialization of null vs string pointers
	data, err := os.ReadFile(lane.ReceiptPath(tempDir, laneID))
	if err != nil {
		t.Fatalf("ReadFile receipt failed: %v", err)
	}
	var raw struct {
		Evidence struct {
			Attestation *string `json:"attestation"`
			CheckLog    *string `json:"check_log"`
		} `json:"evidence"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal receipt failed: %v", err)
	}
	if raw.Evidence.Attestation == nil || *raw.Evidence.Attestation != attestationPath {
		t.Errorf("raw attestation = %v, want %q", raw.Evidence.Attestation, attestationPath)
	}
	if raw.Evidence.CheckLog != nil {
		t.Errorf("raw check_log = %v, want null (nil)", raw.Evidence.CheckLog)
	}

	// Load back using LoadReceipt
	loaded, err := lane.LoadReceipt(tempDir, laneID)
	if err != nil {
		t.Fatalf("LoadReceipt failed: %v", err)
	}
	if loaded.Verdict != lane.VerdictAccepted {
		t.Errorf("loaded.Verdict = %q, want %q", loaded.Verdict, lane.VerdictAccepted)
	}
	if loaded.FinalTree != receipt.FinalTree {
		t.Errorf("loaded.FinalTree = %q, want %q", loaded.FinalTree, receipt.FinalTree)
	}
	if loaded.Evidence.Attestation == nil || *loaded.Evidence.Attestation != attestationPath {
		t.Errorf("loaded.Evidence.Attestation = %v, want %q", loaded.Evidence.Attestation, attestationPath)
	}
	if loaded.Evidence.CheckLog != nil {
		t.Errorf("loaded.Evidence.CheckLog = %v, want nil", loaded.Evidence.CheckLog)
	}

	// Test rejected receipt with both pointers nil
	rejectReceipt := lane.Receipt{
		Version:      1,
		Lane:         laneID,
		FinalTree:    "finaltree456",
		BaseTree:     "basetree123",
		ChangedFiles: []string{"forbidden/file.go"},
		Verdict:      lane.VerdictRejected,
		Reasons:      []string{"changed file not in allowlist"},
		Evidence: lane.Evidence{
			Attestation: nil,
			CheckLog:    nil,
		},
		CreatedAt: time.Now().UTC(),
	}
	if err := lane.WriteReceipt(tempDir, laneID, rejectReceipt); err != nil {
		t.Fatalf("WriteReceipt rejected failed: %v", err)
	}

	rejectData, err := os.ReadFile(lane.ReceiptPath(tempDir, laneID))
	if err != nil {
		t.Fatalf("ReadFile rejected receipt failed: %v", err)
	}
	var rejectRaw map[string]any
	if err := json.Unmarshal(rejectData, &rejectRaw); err != nil {
		t.Fatalf("Unmarshal reject failed: %v", err)
	}
	evidenceRaw, ok := rejectRaw["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("evidence is not an object: %v", rejectRaw["evidence"])
	}
	if evidenceRaw["attestation"] != nil {
		t.Errorf("expected attestation to be null in JSON, got %v", evidenceRaw["attestation"])
	}
	if evidenceRaw["check_log"] != nil {
		t.Errorf("expected check_log to be null in JSON, got %v", evidenceRaw["check_log"])
	}
}

func TestGlobMatching(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		// cmd/**
		{"cmd/**", "cmd/lucind-ai/main.go", true},
		{"cmd/**", "cmd/a.go", true},
		{"cmd/**", "cmd/sub/dir/file.go", true},
		{"cmd/**", "cmd", true},
		{"cmd/**", "other/cmd/a.go", false},
		{"cmd/**", "cmd2/a.go", false},

		// internal/**
		{"internal/**", "internal/lane/lane.go", true},
		{"internal/**", "internal/a.go", true},
		{"internal/**", "pkg/internal/a.go", false},

		// Makefile exact match
		{"Makefile", "Makefile", true},
		{"Makefile", "./Makefile", true},
		{"Makefile", "cmd/Makefile", false},
		{"Makefile", "Makefile.bak", false},

		// *.go single directory wildcards
		{"*.go", "foo.go", true},
		{"*.go", "main.go", true},
		{"*.go", "cmd/foo.go", false},

		// **/*.go
		{"**/*.go", "foo.go", true},
		{"**/*.go", "cmd/foo.go", true},
		{"**/*.go", "a/b/c/foo.go", true},
		{"**/*.go", "foo.txt", false},

		// a/**/b
		{"a/**/b", "a/b", true},
		{"a/**/b", "a/x/b", true},
		{"a/**/b", "a/x/y/b", true},
		{"a/**/b", "a/x/y/c", false},

		// ** alone matches all
		{"**", "anything", true},
		{"**", "a/b/c/d.go", true},
		{"**", "Makefile", true},

		// Normalize paths with ./
		{"cmd/**", "./cmd/a.go", true},

		// Wildcard ?
		{"file?.txt", "file1.txt", true},
		{"file?.txt", "fileA.txt", true},
		{"file?.txt", "file12.txt", false},

		// Character classes
		{"file[0-9].txt", "file5.txt", true},
		{"file[0-9].txt", "fileA.txt", false},
		{"file[!0-9].txt", "fileA.txt", true},
		{"file[!0-9].txt", "file5.txt", false},

		// Empty strings
		{"", "", true},
		{"", "foo", false},
		{"foo", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.path, func(t *testing.T) {
			got := lane.Match(tt.pattern, tt.path)
			if got != tt.want {
				t.Errorf("Match(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}

	// Test MatchAny
	patterns := []string{"cmd/**", "internal/**", "Makefile"}
	if !lane.MatchAny(patterns, "cmd/lucind-ai/main.go") {
		t.Errorf("MatchAny should match cmd/lucind-ai/main.go")
	}
	if !lane.MatchAny(patterns, "internal/lane/lane.go") {
		t.Errorf("MatchAny should match internal/lane/lane.go")
	}
	if !lane.MatchAny(patterns, "Makefile") {
		t.Errorf("MatchAny should match Makefile")
	}
	if lane.MatchAny(patterns, "docs/README.md") {
		t.Errorf("MatchAny should not match docs/README.md")
	}
	if lane.MatchAny(nil, "Makefile") {
		t.Errorf("MatchAny on nil patterns should return false")
	}
}
