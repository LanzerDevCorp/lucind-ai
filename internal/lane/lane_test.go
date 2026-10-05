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

	// Uniqueness. The suffix has only 16 random bits, so IDs generated within
	// the same second can collide (birthday problem); use a distinct second per
	// ID to keep the test deterministic.
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		id, err := lane.GenerateIDAt(fixedTime.Add(time.Duration(i) * time.Second))
		if err != nil {
			t.Fatalf("GenerateIDAt failed on iteration %d: %v", i, err)
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

	l := lane.Lane{ID: id, Turn: 1}
	wantResultPath := filepath.Join(wantDir, "result-1.json")
	if got := lane.ResultFilePath(root, l); got != wantResultPath {
		t.Errorf("ResultFilePath = %q, want %q", got, wantResultPath)
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
		Evidence: []lane.CheckEvidence{
			{
				Check:       "go test ./...",
				Attestation: &attestationPath,
				CheckLog:    nil,
			},
		},
		CreatedAt: time.Now().UTC(),
	}

	if err := lane.WriteReceipt(tempDir, laneID, receipt); err != nil {
		t.Fatalf("WriteReceipt failed: %v", err)
	}

	// Verify raw JSON serialization: attestation present, check_log omitted due to omitempty
	data, err := os.ReadFile(lane.ReceiptPath(tempDir, laneID))
	if err != nil {
		t.Fatalf("ReadFile receipt failed: %v", err)
	}
	var raw struct {
		Evidence []map[string]any `json:"evidence"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal receipt failed: %v", err)
	}
	if len(raw.Evidence) != 1 {
		t.Fatalf("raw evidence len = %d, want 1", len(raw.Evidence))
	}
	if raw.Evidence[0]["check"] != "go test ./..." {
		t.Errorf("raw check = %v, want %q", raw.Evidence[0]["check"], "go test ./...")
	}
	if raw.Evidence[0]["attestation"] != attestationPath {
		t.Errorf("raw attestation = %v, want %q", raw.Evidence[0]["attestation"], attestationPath)
	}
	if _, exists := raw.Evidence[0]["check_log"]; exists {
		t.Errorf("raw check_log should be omitted when nil, but exists: %v", raw.Evidence[0]["check_log"])
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
	if len(loaded.Evidence) != 1 {
		t.Fatalf("loaded.Evidence len = %d, want 1", len(loaded.Evidence))
	}
	if loaded.Evidence[0].Check != "go test ./..." {
		t.Errorf("loaded.Evidence[0].Check = %q, want %q", loaded.Evidence[0].Check, "go test ./...")
	}
	if loaded.Evidence[0].Attestation == nil || *loaded.Evidence[0].Attestation != attestationPath {
		t.Errorf("loaded.Evidence[0].Attestation = %v, want %q", loaded.Evidence[0].Attestation, attestationPath)
	}
	if loaded.Evidence[0].CheckLog != nil {
		t.Errorf("loaded.Evidence[0].CheckLog = %v, want nil", loaded.Evidence[0].CheckLog)
	}

	// Test check_log without attestation serializes with check_log and omits attestation
	checkLogPath := "/path/to/check.log"
	logReceipt := lane.Receipt{
		Version:      1,
		Lane:         laneID,
		FinalTree:    "finaltree789",
		BaseTree:     "basetree123",
		ChangedFiles: []string{"main.go"},
		Verdict:      lane.VerdictAccepted,
		Reasons:      []string{},
		Evidence: []lane.CheckEvidence{
			{
				Check:       "sh lucind-checks.sh",
				Attestation: nil,
				CheckLog:    &checkLogPath,
			},
		},
		CreatedAt: time.Now().UTC(),
	}
	if err := lane.WriteReceipt(tempDir, laneID, logReceipt); err != nil {
		t.Fatalf("WriteReceipt logReceipt failed: %v", err)
	}
	logData, err := os.ReadFile(lane.ReceiptPath(tempDir, laneID))
	if err != nil {
		t.Fatalf("ReadFile log receipt failed: %v", err)
	}
	var logRaw struct {
		Evidence []map[string]any `json:"evidence"`
	}
	if err := json.Unmarshal(logData, &logRaw); err != nil {
		t.Fatalf("Unmarshal log receipt failed: %v", err)
	}
	if len(logRaw.Evidence) != 1 {
		t.Fatalf("log raw evidence len = %d, want 1", len(logRaw.Evidence))
	}
	if logRaw.Evidence[0]["check_log"] != checkLogPath {
		t.Errorf("raw check_log = %v, want %q", logRaw.Evidence[0]["check_log"], checkLogPath)
	}
	if _, exists := logRaw.Evidence[0]["attestation"]; exists {
		t.Errorf("raw attestation should be omitted when nil, but exists: %v", logRaw.Evidence[0]["attestation"])
	}

	// Test nil Evidence serializes as empty slice [] in JSON
	nilEvidenceReceipt := lane.Receipt{
		Version:      1,
		Lane:         laneID,
		FinalTree:    "finaltree456",
		BaseTree:     "basetree123",
		ChangedFiles: []string{"forbidden/file.go"},
		Verdict:      lane.VerdictRejected,
		Reasons:      []string{"changed file not in allowlist"},
		Evidence:     nil,
		CreatedAt:    time.Now().UTC(),
	}
	if err := lane.WriteReceipt(tempDir, laneID, nilEvidenceReceipt); err != nil {
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
	evidenceSlice, ok := rejectRaw["evidence"].([]any)
	if !ok {
		t.Fatalf("evidence is not an array: %T (%v)", rejectRaw["evidence"], rejectRaw["evidence"])
	}
	if len(evidenceSlice) != 0 {
		t.Errorf("expected empty array [] for evidence in JSON, got len %d", len(evidenceSlice))
	}

	loadedReject, err := lane.LoadReceipt(tempDir, laneID)
	if err != nil {
		t.Fatalf("LoadReceipt rejected failed: %v", err)
	}
	if len(loadedReject.Evidence) != 0 {
		t.Errorf("loadedReject.Evidence len = %d, want 0", len(loadedReject.Evidence))
	}
}

func TestWriteReceipt_RoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	laneID := "20261003-215144-rtrip"

	attestationPath := "/path/to/attest.json"
	checkLogPath := "/path/to/check.log"

	receipt := lane.Receipt{
		Version:      1,
		Lane:         laneID,
		FinalTree:    "tree-final",
		BaseTree:     "tree-base",
		ChangedFiles: []string{"foo.go", "bar.go"},
		Verdict:      lane.VerdictAccepted,
		Reasons:      []string{"reason 1"},
		Evidence: []lane.CheckEvidence{
			{
				Check:       "check 1",
				Attestation: &attestationPath,
			},
			{
				Check:    "check 2",
				CheckLog: &checkLogPath,
			},
		},
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
	}

	if err := lane.WriteReceipt(tempDir, laneID, receipt); err != nil {
		t.Fatalf("WriteReceipt failed: %v", err)
	}

	loaded, err := lane.LoadReceipt(tempDir, laneID)
	if err != nil {
		t.Fatalf("LoadReceipt failed: %v", err)
	}

	if loaded.Lane != receipt.Lane {
		t.Errorf("Lane = %q, want %q", loaded.Lane, receipt.Lane)
	}
	if loaded.FinalTree != receipt.FinalTree {
		t.Errorf("FinalTree = %q, want %q", loaded.FinalTree, receipt.FinalTree)
	}
	if loaded.Verdict != receipt.Verdict {
		t.Errorf("Verdict = %q, want %q", loaded.Verdict, receipt.Verdict)
	}
	if len(loaded.Evidence) != 2 {
		t.Fatalf("len(Evidence) = %d, want 2", len(loaded.Evidence))
	}
	if loaded.Evidence[0].Check != "check 1" || *loaded.Evidence[0].Attestation != attestationPath || loaded.Evidence[0].CheckLog != nil {
		t.Errorf("Evidence[0] mismatch: %+v", loaded.Evidence[0])
	}
	if loaded.Evidence[1].Check != "check 2" || *loaded.Evidence[1].CheckLog != checkLogPath || loaded.Evidence[1].Attestation != nil {
		t.Errorf("Evidence[1] mismatch: %+v", loaded.Evidence[1])
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

func TestMarkStopped(t *testing.T) {
	validDoneJSON := `{
		"lane_id": "test-pkt",
		"status": "done",
		"summary": "Completed successfully",
		"hard_stops": []
	}`
	validFailedJSON := `{
		"lane_id": "test-pkt",
		"status": "failed",
		"summary": "Execution failed",
		"hard_stops": []
	}`
	schemaInvalidJSON := `{
		"lane_id": "test-pkt",
		"status": "done"
	}` // missing required hard_stops

	t.Run("valid result.json with status done marks lane done", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, dir)
		l, err := lane.Create(context.Background(), dir, nil, "gemini-3.8-flash-high")
		if err != nil {
			t.Fatalf("Create lane: %v", err)
		}
		resPath := lane.ResultFilePath(dir, l)
		if err := os.WriteFile(resPath, []byte(validDoneJSON), 0644); err != nil {
			t.Fatalf("write result.json: %v", err)
		}

		st, err := lane.MarkStopped(dir, l.ID)
		if err != nil {
			t.Fatalf("MarkStopped error = %v, want nil", err)
		}
		if st != lane.StatusDone {
			t.Errorf("status = %v, want %v", st, lane.StatusDone)
		}

		loaded, err := lane.Load(dir, l.ID)
		if err != nil {
			t.Fatalf("Load lane: %v", err)
		}
		if loaded.Status != lane.StatusDone {
			t.Errorf("loaded.Status = %v, want %v", loaded.Status, lane.StatusDone)
		}
	})

	t.Run("missing result.json marks lane failed", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, dir)
		l, err := lane.Create(context.Background(), dir, nil, "gemini-3.8-flash-high")
		if err != nil {
			t.Fatalf("Create lane: %v", err)
		}

		st, err := lane.MarkStopped(dir, l.ID)
		if err != nil {
			t.Fatalf("MarkStopped error = %v, want nil", err)
		}
		if st != lane.StatusFailed {
			t.Errorf("status = %v, want %v", st, lane.StatusFailed)
		}

		loaded, err := lane.Load(dir, l.ID)
		if err != nil {
			t.Fatalf("Load lane: %v", err)
		}
		if loaded.Status != lane.StatusFailed {
			t.Errorf("loaded.Status = %v, want %v", loaded.Status, lane.StatusFailed)
		}
	})

	t.Run("schema-invalid result.json marks lane failed", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, dir)
		l, err := lane.Create(context.Background(), dir, nil, "gemini-3.8-flash-high")
		if err != nil {
			t.Fatalf("Create lane: %v", err)
		}
		resPath := lane.ResultFilePath(dir, l)
		if err := os.WriteFile(resPath, []byte(schemaInvalidJSON), 0644); err != nil {
			t.Fatalf("write result.json: %v", err)
		}

		st, err := lane.MarkStopped(dir, l.ID)
		if err != nil {
			t.Fatalf("MarkStopped error = %v, want nil", err)
		}
		if st != lane.StatusFailed {
			t.Errorf("status = %v, want %v", st, lane.StatusFailed)
		}

		loaded, err := lane.Load(dir, l.ID)
		if err != nil {
			t.Fatalf("Load lane: %v", err)
		}
		if loaded.Status != lane.StatusFailed {
			t.Errorf("loaded.Status = %v, want %v", loaded.Status, lane.StatusFailed)
		}
	})

	t.Run("result.json with status != done marks lane failed", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, dir)
		l, err := lane.Create(context.Background(), dir, nil, "gemini-3.8-flash-high")
		if err != nil {
			t.Fatalf("Create lane: %v", err)
		}
		resPath := lane.ResultFilePath(dir, l)
		if err := os.WriteFile(resPath, []byte(validFailedJSON), 0644); err != nil {
			t.Fatalf("write result.json: %v", err)
		}

		st, err := lane.MarkStopped(dir, l.ID)
		if err != nil {
			t.Fatalf("MarkStopped error = %v, want nil", err)
		}
		if st != lane.StatusFailed {
			t.Errorf("status = %v, want %v", st, lane.StatusFailed)
		}

		loaded, err := lane.Load(dir, l.ID)
		if err != nil {
			t.Fatalf("Load lane: %v", err)
		}
		if loaded.Status != lane.StatusFailed {
			t.Errorf("loaded.Status = %v, want %v", loaded.Status, lane.StatusFailed)
		}
	})

	t.Run("lane load error returns empty status and error", func(t *testing.T) {
		dir := t.TempDir()
		st, err := lane.MarkStopped(dir, "non-existent-lane")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if st != lane.Status("") {
			t.Errorf("status = %v, want empty", st)
		}
	})

	t.Run("turn 2 with only result-1.json on disk marks lane failed", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, dir)
		l, err := lane.Create(context.Background(), dir, nil, "gemini-3.8-flash-high")
		if err != nil {
			t.Fatalf("Create lane: %v", err)
		}
		// Write result-1.json
		r1Path := filepath.Join(lane.LaneDir(dir, l.ID), "result-1.json")
		if err := os.WriteFile(r1Path, []byte(validDoneJSON), 0644); err != nil {
			t.Fatalf("write result-1.json: %v", err)
		}
		// Bump lane to turn 2 and save
		l.Turn = 2
		if err := lane.Save(dir, l); err != nil {
			t.Fatalf("Save lane: %v", err)
		}

		st, err := lane.MarkStopped(dir, l.ID)
		if err != nil {
			t.Fatalf("MarkStopped error = %v, want nil", err)
		}
		if st != lane.StatusFailed {
			t.Errorf("status = %v, want %v", st, lane.StatusFailed)
		}

		loaded, err := lane.Load(dir, l.ID)
		if err != nil {
			t.Fatalf("Load lane: %v", err)
		}
		if loaded.Status != lane.StatusFailed {
			t.Errorf("loaded.Status = %v, want %v", loaded.Status, lane.StatusFailed)
		}
	})

	t.Run("turn 1 with only result.json on disk marks lane failed", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, dir)
		l, err := lane.Create(context.Background(), dir, nil, "gemini-3.8-flash-high")
		if err != nil {
			t.Fatalf("Create lane: %v", err)
		}
		legacyPath := filepath.Join(lane.LaneDir(dir, l.ID), "result.json")
		if err := os.WriteFile(legacyPath, []byte(validDoneJSON), 0644); err != nil {
			t.Fatalf("write result.json: %v", err)
		}

		st, err := lane.MarkStopped(dir, l.ID)
		if err != nil {
			t.Fatalf("MarkStopped error = %v, want nil", err)
		}
		if st != lane.StatusFailed {
			t.Errorf("status = %v, want %v", st, lane.StatusFailed)
		}

		loaded, err := lane.Load(dir, l.ID)
		if err != nil {
			t.Fatalf("Load lane: %v", err)
		}
		if loaded.Status != lane.StatusFailed {
			t.Errorf("loaded.Status = %v, want %v", loaded.Status, lane.StatusFailed)
		}
	})

	t.Run("turn 2 with valid done result-2.json marks lane done", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, dir)
		l, err := lane.Create(context.Background(), dir, nil, "gemini-3.8-flash-high")
		if err != nil {
			t.Fatalf("Create lane: %v", err)
		}
		l.Turn = 2
		if err := lane.Save(dir, l); err != nil {
			t.Fatalf("Save lane: %v", err)
		}
		r2Path := lane.ResultFilePath(dir, l)
		if err := os.WriteFile(r2Path, []byte(validDoneJSON), 0644); err != nil {
			t.Fatalf("write result-2.json: %v", err)
		}

		st, err := lane.MarkStopped(dir, l.ID)
		if err != nil {
			t.Fatalf("MarkStopped error = %v, want nil", err)
		}
		if st != lane.StatusDone {
			t.Errorf("status = %v, want %v", st, lane.StatusDone)
		}

		loaded, err := lane.Load(dir, l.ID)
		if err != nil {
			t.Fatalf("Load lane: %v", err)
		}
		if loaded.Status != lane.StatusDone {
			t.Errorf("loaded.Status = %v, want %v", loaded.Status, lane.StatusDone)
		}
	})

	t.Run("legacy lane with turn 0 and valid result.json marks lane done", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, dir)
		l, err := lane.Create(context.Background(), dir, nil, "gemini-3.8-flash-high")
		if err != nil {
			t.Fatalf("Create lane: %v", err)
		}
		l.Turn = 0
		if err := lane.Save(dir, l); err != nil {
			t.Fatalf("Save lane: %v", err)
		}
		resPath := lane.ResultFilePath(dir, l)
		if err := os.WriteFile(resPath, []byte(validDoneJSON), 0644); err != nil {
			t.Fatalf("write result.json: %v", err)
		}

		st, err := lane.MarkStopped(dir, l.ID)
		if err != nil {
			t.Fatalf("MarkStopped error = %v, want nil", err)
		}
		if st != lane.StatusDone {
			t.Errorf("status = %v, want %v", st, lane.StatusDone)
		}

		loaded, err := lane.Load(dir, l.ID)
		if err != nil {
			t.Fatalf("Load lane: %v", err)
		}
		if loaded.Status != lane.StatusDone {
			t.Errorf("loaded.Status = %v, want %v", loaded.Status, lane.StatusDone)
		}
	})
}

func TestCheckCommand(t *testing.T) {
	cmd := "go test ./..."
	want := "sh -c go test ./..."
	got := lane.CheckCommand(cmd)
	if got != want {
		t.Errorf("CheckCommand(%q) = %q, want %q", cmd, got, want)
	}
}

func TestLaneChecksJSONSerialization(t *testing.T) {
	t.Run("with checks", func(t *testing.T) {
		ln := lane.Lane{
			Version: 1,
			ID:      "20261003-215144-chk1",
			Checks:  []string{"go test ./...", "go vet ./..."},
		}
		data, err := json.Marshal(ln)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("Unmarshal raw failed: %v", err)
		}
		rawChecks, ok := raw["checks"].([]any)
		if !ok {
			t.Fatalf("expected raw checks to be []any, got %T: %v", raw["checks"], raw["checks"])
		}
		if len(rawChecks) != 2 || rawChecks[0] != "go test ./..." || rawChecks[1] != "go vet ./..." {
			t.Errorf("unexpected checks array in json: %v", rawChecks)
		}

		var deserialized lane.Lane
		if err := json.Unmarshal(data, &deserialized); err != nil {
			t.Fatalf("Unmarshal into Lane failed: %v", err)
		}
		if len(deserialized.Checks) != 2 || deserialized.Checks[0] != "go test ./..." || deserialized.Checks[1] != "go vet ./..." {
			t.Errorf("deserialized.Checks = %v, want %v", deserialized.Checks, ln.Checks)
		}
	})

	t.Run("without checks", func(t *testing.T) {
		ln := lane.Lane{
			Version: 1,
			ID:      "20261003-215144-chk0",
		}
		data, err := json.Marshal(ln)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("Unmarshal raw failed: %v", err)
		}
		if _, exists := raw["checks"]; exists {
			t.Errorf("expected 'checks' field to be omitted from JSON when empty/nil, but found: %v", raw["checks"])
		}

		var deserialized lane.Lane
		if err := json.Unmarshal(data, &deserialized); err != nil {
			t.Fatalf("Unmarshal into Lane failed: %v", err)
		}
		if len(deserialized.Checks) != 0 {
			t.Errorf("deserialized.Checks = %v, want empty", deserialized.Checks)
		}
	})
}

func TestLaneLastStopAtAndContinuesJSON(t *testing.T) {
	t.Run("without fields unmarshals and loads with zero values", func(t *testing.T) {
		jsonWithout := `{
			"version": 1,
			"id": "20261003-215144-zero",
			"cwd": "/test",
			"base_tree": "abc",
			"allow": [],
			"model": "m",
			"pane_id": "",
			"status": "running",
			"retries": 0,
			"created_at": "2026-10-03T21:51:44Z",
			"updated_at": "2026-10-03T21:51:44Z"
		}`
		var ln lane.Lane
		if err := json.Unmarshal([]byte(jsonWithout), &ln); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if ln.LastStopAt != nil {
			t.Errorf("ln.LastStopAt = %v, want nil", ln.LastStopAt)
		}
		if ln.Continues != 0 {
			t.Errorf("ln.Continues = %d, want 0", ln.Continues)
		}

		tempDir := t.TempDir()
		laneDir := lane.LaneDir(tempDir, ln.ID)
		if err := os.MkdirAll(laneDir, 0755); err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}
		if err := os.WriteFile(lane.LanePath(tempDir, ln.ID), []byte(jsonWithout), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		loaded, err := lane.Load(tempDir, ln.ID)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if loaded.LastStopAt != nil {
			t.Errorf("loaded.LastStopAt = %v, want nil", loaded.LastStopAt)
		}
		if loaded.Continues != 0 {
			t.Errorf("loaded.Continues = %d, want 0", loaded.Continues)
		}
	})

	t.Run("when fields are set Save and Load preserve their values", func(t *testing.T) {
		tempDir := t.TempDir()
		stopTime := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
		ln := lane.Lane{
			Version:    1,
			ID:         "20261003-215144-set1",
			Status:     lane.StatusDone,
			LastStopAt: &stopTime,
			Continues:  3,
		}
		if err := lane.Save(tempDir, ln); err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		loaded, err := lane.Load(tempDir, ln.ID)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if loaded.LastStopAt == nil {
			t.Fatal("loaded.LastStopAt is nil, want non-nil")
		}
		if !loaded.LastStopAt.Equal(stopTime) {
			t.Errorf("loaded.LastStopAt = %v, want %v", loaded.LastStopAt, stopTime)
		}
		if loaded.Continues != 3 {
			t.Errorf("loaded.Continues = %d, want 3", loaded.Continues)
		}
	})

	t.Run("when fields are zero marshaling and Save omit them from JSON", func(t *testing.T) {
		ln := lane.Lane{
			Version:    1,
			ID:         "20261003-215144-omit",
			Status:     lane.StatusRunning,
			LastStopAt: nil,
			Continues:  0,
		}
		data, err := json.Marshal(ln)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("Unmarshal raw failed: %v", err)
		}
		if _, exists := raw["last_stop_at"]; exists {
			t.Errorf("expected 'last_stop_at' to be omitted from JSON when nil, got %v", raw["last_stop_at"])
		}
		if _, exists := raw["continues"]; exists {
			t.Errorf("expected 'continues' to be omitted from JSON when 0, got %v", raw["continues"])
		}

		tempDir := t.TempDir()
		if err := lane.Save(tempDir, ln); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
		savedData, err := os.ReadFile(lane.LanePath(tempDir, ln.ID))
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}
		var savedRaw map[string]any
		if err := json.Unmarshal(savedData, &savedRaw); err != nil {
			t.Fatalf("Unmarshal savedRaw failed: %v", err)
		}
		if _, exists := savedRaw["last_stop_at"]; exists {
			t.Errorf("expected 'last_stop_at' omitted in saved JSON, got %v", savedRaw["last_stop_at"])
		}
		if _, exists := savedRaw["continues"]; exists {
			t.Errorf("expected 'continues' omitted in saved JSON, got %v", savedRaw["continues"])
		}
	})
}

func TestLaneCreateWithChecks(t *testing.T) {
	ctx := context.Background()

	t.Run("with checks", func(t *testing.T) {
		repoDir := t.TempDir()
		initGitRepo(t, repoDir)

		checks := []string{"go test ./...", "go vet ./..."}
		ln, err := lane.Create(ctx, repoDir, []string{"*"}, "test-model", checks...)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		if len(ln.Checks) != len(checks) {
			t.Fatalf("ln.Checks len = %d, want %d", len(ln.Checks), len(checks))
		}
		for i, c := range checks {
			if ln.Checks[i] != c {
				t.Errorf("ln.Checks[%d] = %q, want %q", i, ln.Checks[i], c)
			}
		}

		// Verify persisted lane.json
		loaded, err := lane.Load(repoDir, ln.ID)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if len(loaded.Checks) != len(checks) {
			t.Fatalf("loaded.Checks len = %d, want %d", len(loaded.Checks), len(checks))
		}
		for i, c := range checks {
			if loaded.Checks[i] != c {
				t.Errorf("loaded.Checks[%d] = %q, want %q", i, loaded.Checks[i], c)
			}
		}

		// Verify JSON file contains checks
		data, err := os.ReadFile(lane.LanePath(repoDir, ln.ID))
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("Unmarshal raw failed: %v", err)
		}
		if _, exists := raw["checks"]; !exists {
			t.Errorf("expected 'checks' key in lane.json when checks provided")
		}
	})

	t.Run("without checks", func(t *testing.T) {
		repoDir := t.TempDir()
		initGitRepo(t, repoDir)

		ln, err := lane.Create(ctx, repoDir, []string{"*"}, "test-model")
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		if len(ln.Checks) != 0 {
			t.Errorf("ln.Checks = %v, want empty/nil", ln.Checks)
		}

		loaded, err := lane.Load(repoDir, ln.ID)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if len(loaded.Checks) != 0 {
			t.Errorf("loaded.Checks = %v, want empty/nil", loaded.Checks)
		}

		data, err := os.ReadFile(lane.LanePath(repoDir, ln.ID))
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("Unmarshal raw failed: %v", err)
		}
		if _, exists := raw["checks"]; exists {
			t.Errorf("expected 'checks' key omitted in lane.json when no checks provided, got: %v", raw["checks"])
		}
	})
}

func TestResultFileName(t *testing.T) {
	tests := []struct {
		turn int
		want string
	}{
		{-1, "result.json"},
		{0, "result.json"},
		{1, "result-1.json"},
		{7, "result-7.json"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := lane.ResultFileName(tt.turn)
			if got != tt.want {
				t.Errorf("ResultFileName(%d) = %q, want %q", tt.turn, got, tt.want)
			}
		})
	}
}

func TestResultFilePath(t *testing.T) {
	root := "/test/repo"
	id := "20261003-215144-a1b2"

	tests := []struct {
		name string
		lane lane.Lane
		want string
	}{
		{
			name: "legacy turn 0",
			lane: lane.Lane{ID: id, Turn: 0},
			want: filepath.Join(root, ".lucind", "lanes", id, "result.json"),
		},
		{
			name: "negative turn",
			lane: lane.Lane{ID: id, Turn: -1},
			want: filepath.Join(root, ".lucind", "lanes", id, "result.json"),
		},
		{
			name: "turn 1",
			lane: lane.Lane{ID: id, Turn: 1},
			want: filepath.Join(root, ".lucind", "lanes", id, "result-1.json"),
		},
		{
			name: "turn 7",
			lane: lane.Lane{ID: id, Turn: 7},
			want: filepath.Join(root, ".lucind", "lanes", id, "result-7.json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lane.ResultFilePath(root, tt.lane)
			if got != tt.want {
				t.Errorf("ResultFilePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLaneTurn(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	ln, err := lane.Create(ctx, repoDir, []string{"*"}, "test-model")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if ln.Turn != 1 {
		t.Fatalf("ln.Turn = %d, want 1", ln.Turn)
	}

	loaded, err := lane.Load(repoDir, ln.ID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Turn != 1 {
		t.Errorf("loaded.Turn = %d, want 1", loaded.Turn)
	}

	// Turn survives Save/Load
	loaded.Turn = 3
	if err := lane.Save(repoDir, loaded); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	reloaded, err := lane.Load(repoDir, ln.ID)
	if err != nil {
		t.Fatalf("Load after save failed: %v", err)
	}
	if reloaded.Turn != 3 {
		t.Errorf("reloaded.Turn = %d, want 3", reloaded.Turn)
	}

	// Legacy lane without turn unmarshals with Turn == 0
	legacyJSON := `{
		"version": 1,
		"id": "20261003-215144-old0",
		"cwd": "/test",
		"base_tree": "abc",
		"allow": [],
		"model": "m",
		"status": "running"
	}`
	var legacy lane.Lane
	if err := json.Unmarshal([]byte(legacyJSON), &legacy); err != nil {
		t.Fatalf("Unmarshal legacy JSON failed: %v", err)
	}
	if legacy.Turn != 0 {
		t.Errorf("legacy.Turn = %d, want 0", legacy.Turn)
	}
}

func TestSkillsFileName(t *testing.T) {
	tests := []struct {
		turn int
		want string
	}{
		{-1, "skills.json"},
		{0, "skills.json"},
		{1, "skills-1.json"},
		{2, "skills-2.json"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := lane.SkillsFileName(tt.turn)
			if got != tt.want {
				t.Errorf("SkillsFileName(%d) = %q, want %q", tt.turn, got, tt.want)
			}
		})
	}
}

func TestSkillsFilePath(t *testing.T) {
	root := "/test/repo"
	id := "20261003-215144-a1b2"

	tests := []struct {
		name string
		lane lane.Lane
		want string
	}{
		{
			name: "legacy turn 0",
			lane: lane.Lane{ID: id, Turn: 0},
			want: filepath.Join(root, ".lucind", "lanes", id, "skills.json"),
		},
		{
			name: "negative turn",
			lane: lane.Lane{ID: id, Turn: -1},
			want: filepath.Join(root, ".lucind", "lanes", id, "skills.json"),
		},
		{
			name: "turn 1",
			lane: lane.Lane{ID: id, Turn: 1},
			want: filepath.Join(root, ".lucind", "lanes", id, "skills-1.json"),
		},
		{
			name: "turn 2",
			lane: lane.Lane{ID: id, Turn: 2},
			want: filepath.Join(root, ".lucind", "lanes", id, "skills-2.json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lane.SkillsFilePath(root, tt.lane)
			if got != tt.want {
				t.Errorf("SkillsFilePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAtomicWriteJSON(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "sub", "test.json")

	data := map[string]string{"key": "value"}
	if err := lane.AtomicWriteJSON(targetFile, data); err != nil {
		t.Fatalf("AtomicWriteJSON failed: %v", err)
	}

	content, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	var readBack map[string]string
	if err := json.Unmarshal(content, &readBack); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if readBack["key"] != "value" {
		t.Errorf("readBack[key] = %q, want value", readBack["key"])
	}
}
