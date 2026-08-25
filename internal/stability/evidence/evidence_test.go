package evidence_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/stability/evidence"
)

func TestEvidenceBoundedSanitizationAndHashing(t *testing.T) {
	// 1. Test empty stream
	emptyEv := evidence.SanitizeStream("")
	if emptyEv.Algorithm != "sha256" {
		t.Errorf("Algorithm = %q, want sha256", emptyEv.Algorithm)
	}
	emptyHash := sha256.Sum256([]byte(""))
	if emptyEv.Digest != hex.EncodeToString(emptyHash[:]) {
		t.Errorf("Digest = %q, want %q", emptyEv.Digest, hex.EncodeToString(emptyHash[:]))
	}
	if emptyEv.RawBytes != 0 {
		t.Errorf("RawBytes = %d, want 0", emptyEv.RawBytes)
	}
	if emptyEv.Sanitized != evidence.NoStreamDetail {
		t.Errorf("Sanitized = %q, want %q", emptyEv.Sanitized, evidence.NoStreamDetail)
	}

	// 2. Test stream with absolute paths
	rawWithPaths := "Error occurred in /home/user/project/src/main.go at line 42; temporary file at /tmp/test-1234/out.log"
	evWithPaths := evidence.SanitizeStream(rawWithPaths, "/home/user/project")
	if strings.Contains(evWithPaths.Sanitized, "/home/user/project") {
		t.Errorf("Sanitized output contains unstripped absolute path: %q", evWithPaths.Sanitized)
	}
	if strings.Contains(evWithPaths.Sanitized, "/tmp/test-1234") {
		t.Errorf("Sanitized output contains unstripped temp path: %q", evWithPaths.Sanitized)
	}
	rawHash := sha256.Sum256([]byte(rawWithPaths))
	if evWithPaths.Digest != hex.EncodeToString(rawHash[:]) {
		t.Errorf("Digest = %q, want %q", evWithPaths.Digest, hex.EncodeToString(rawHash[:]))
	}
	if evWithPaths.RawBytes != len(rawWithPaths) {
		t.Errorf("RawBytes = %d, want %d", evWithPaths.RawBytes, len(rawWithPaths))
	}

	// 3. Test stream exceeding 4096 bytes
	largeStream := strings.Repeat("A", 5000) + "FINAL_DIAGNOSTIC_MESSAGE"
	largeEv := evidence.SanitizeStream(largeStream)
	if largeEv.RawBytes != len(largeStream) {
		t.Errorf("RawBytes = %d, want %d", largeEv.RawBytes, len(largeStream))
	}
	expectedHash := sha256.Sum256([]byte(largeStream))
	if largeEv.Digest != hex.EncodeToString(expectedHash[:]) {
		t.Errorf("Digest = %q, want %q", largeEv.Digest, hex.EncodeToString(expectedHash[:]))
	}
	if !strings.Contains(largeEv.Sanitized, "FINAL_DIAGNOSTIC_MESSAGE") {
		t.Errorf("Sanitized output should keep the tail, but did not contain final message: %q", largeEv.Sanitized)
	}
	if !strings.Contains(largeEv.Sanitized, "[truncated, showing last 4096 of") {
		t.Errorf("Sanitized output missing truncation marker: %q", largeEv.Sanitized)
	}

	// 4. Test SanitizeDiagnostics for both stdout and stderr
	diag := evidence.SanitizeDiagnostics("stdout log /var/log/app.log", "stderr error", "/var/log")
	if diag.Stdout.RawBytes == 0 || diag.Stderr.RawBytes == 0 {
		t.Errorf("DiagnosticEvidence has empty fields: %+v", diag)
	}
}

func TestStripPathsAndFormatStreamDirect(t *testing.T) {
	// Test StripPaths directly
	stripped := evidence.StripPaths("File located at /var/data/project/file.txt", "/var/data")
	if strings.Contains(stripped, "/var/data") {
		t.Errorf("StripPaths failed to strip base path: %q", stripped)
	}

	// Test FormatStream directly
	empty := evidence.FormatStream("")
	if empty != evidence.NoStreamDetail {
		t.Errorf("FormatStream empty = %q, want %q", empty, evidence.NoStreamDetail)
	}

	short := evidence.FormatStream("short output")
	if short != "short output" {
		t.Errorf("FormatStream short = %q, want short output", short)
	}

	long := evidence.FormatStream(strings.Repeat("B", 5000))
	if !strings.Contains(long, "...[truncated, showing last 4096 of 5000 bytes]") {
		t.Errorf("FormatStream long missing truncated marker: %q", long)
	}

	// Test HashPayload directly
	algo, digest := evidence.HashPayload([]byte("hello world"))
	if algo != evidence.HashAlgorithm {
		t.Errorf("HashPayload algo = %q, want %q", algo, evidence.HashAlgorithm)
	}
	wantDigest := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if digest != wantDigest {
		t.Errorf("HashPayload digest = %q, want %q", digest, wantDigest)
	}
}


func TestEvidenceSanitizationAndCanonicalReceiptRFC8785(t *testing.T) {
	// 1. Prove 4096-byte cap, path stripping, raw payload hashing
	raw := "/home/lanzer/secret/path/run.log: " + strings.Repeat("X", 5000) + "DONE"
	ev := evidence.SanitizeStream(raw, "/home/lanzer/secret/path")
	if ev.Algorithm != "sha256" {
		t.Errorf("Algorithm = %q, want sha256", ev.Algorithm)
	}
	if strings.Contains(ev.Sanitized, "/home/lanzer/secret/path") {
		t.Errorf("Sanitized contains secret path: %q", ev.Sanitized)
	}
	if !strings.Contains(ev.Sanitized, "DONE") {
		t.Errorf("Sanitized missing tail message: %q", ev.Sanitized)
	}

	// 2. Build receipt
	r := evidence.Receipt{
		ReceiptID:    "rcpt-12345",
		CandidateSHA: "a1b2c3d4e5f6",
		BuildVersion: "v1.2.3-test",
		Verdict:      "passed",
		CreatedAt:    "2026-08-25T12:00:00Z",
		FixtureDigests: map[string]string{
			"z_fixture": "hash_z",
			"a_fixture": "hash_a",
			"m_fixture": "hash_m",
		},
		Trials: []evidence.TrialRecord{
			{
				TrialNumber: 1,
				Verdict:     "passed",
				Diagnostics: evidence.SanitizeDiagnostics("out1", "err1"),
			},
		},
	}

	canonicalBytes, err := r.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON failed: %v", err)
	}

	// Assert no whitespace outside strings
	canonicalStr := string(canonicalBytes)
	if strings.Contains(canonicalStr, " : ") || strings.Contains(canonicalStr, "\n") || strings.Contains(canonicalStr, ", ") {
		t.Errorf("Canonical JSON has whitespace formatting: %s", canonicalStr)
	}

	// Assert deterministic key ordering (e.g. build_version before candidate_sha, a_fixture before m_fixture before z_fixture)
	expectedSubstrings := []string{
		`"build_version":"v1.2.3-test"`,
		`"candidate_sha":"a1b2c3d4e5f6"`,
		`"created_at":"2026-08-25T12:00:00Z"`,
		`"fixture_digests":{"a_fixture":"hash_a","m_fixture":"hash_m","z_fixture":"hash_z"}`,
		`"receipt_id":"rcpt-12345"`,
		`"verdict":"passed"`,
	}
	for _, sub := range expectedSubstrings {
		if !strings.Contains(canonicalStr, sub) {
			t.Errorf("Canonical JSON missing expected substring %q in %s", sub, canonicalStr)
		}
	}
}

func TestReceiptCanonicalJSONDeterminism(t *testing.T) {
	// Construct map with fixture entries inserted in different orders
	map1 := map[string]string{"b": "2", "a": "1", "c": "3"}
	map2 := map[string]string{"c": "3", "b": "2", "a": "1"}

	r1 := evidence.Receipt{
		ReceiptID:      "rcpt-1",
		CandidateSHA:   "sha1",
		BuildVersion:   "v1",
		Verdict:        "passed",
		CreatedAt:      "2026-08-25T00:00:00Z",
		FixtureDigests: map1,
	}

	r2 := evidence.Receipt{
		ReceiptID:      "rcpt-1",
		CandidateSHA:   "sha1",
		BuildVersion:   "v1",
		Verdict:        "passed",
		CreatedAt:      "2026-08-25T00:00:00Z",
		FixtureDigests: map2,
	}

	b1, err := r1.CanonicalJSON()
	if err != nil {
		t.Fatalf("r1.CanonicalJSON failed: %v", err)
	}

	b2, err := r2.CanonicalJSON()
	if err != nil {
		t.Fatalf("r2.CanonicalJSON failed: %v", err)
	}

	// Assert byte-for-byte equality across independent encodes
	if string(b1) != string(b2) {
		t.Errorf("Canonical JSON mismatch:\n b1: %s\n b2: %s", string(b1), string(b2))
	}

	// Test MarshalCanonical on generic maps
	m1Bytes, err := evidence.MarshalCanonical(map1)
	if err != nil {
		t.Fatalf("MarshalCanonical map1: %v", err)
	}
	m2Bytes, err := evidence.MarshalCanonical(map2)
	if err != nil {
		t.Fatalf("MarshalCanonical map2: %v", err)
	}
	if string(m1Bytes) != `{"a":"1","b":"2","c":"3"}` {
		t.Errorf("MarshalCanonical map = %s, want %s", string(m1Bytes), `{"a":"1","b":"2","c":"3"}`)
	}
	if string(m1Bytes) != string(m2Bytes) {
		t.Errorf("MarshalCanonical determinism mismatch: %s != %s", string(m1Bytes), string(m2Bytes))
	}
}

func TestWriteReceiptRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	receiptPath := tmpDir + "/sub/receipt.json"

	r := evidence.Receipt{
		ReceiptID:    "rcpt-disk",
		CandidateSHA: "sha-disk",
		BuildVersion: "v1.0.0",
		Verdict:      "passed",
		CreatedAt:    "2026-08-25T10:00:00Z",
	}

	if err := evidence.WriteReceipt(receiptPath, r); err != nil {
		t.Fatalf("WriteReceipt failed: %v", err)
	}

	b, err := r.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON: %v", err)
	}

	// Read from disk and verify byte equality
	diskBytes, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if string(diskBytes) != string(b) {
		t.Errorf("Disk receipt does not match canonical bytes: %s != %s", string(diskBytes), string(b))
	}
}

