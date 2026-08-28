package packetauthor_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/packetauthor"
)

func TestAdmitManualCompatibilityFixturesPreserveExactBytes(t *testing.T) {
	tests := []struct {
		name     string
		readOnly bool
	}{
		{name: "write", readOnly: false},
		{name: "read-only", readOnly: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join("testdata", "compatibility", tt.name+".md"))
			if err != nil {
				t.Fatal(err)
			}
			artifact, err := packetauthor.AdmitManual(packetauthor.ManualPacket{
				Body: body, RouteIntent: "apply", ReadOnly: tt.readOnly,
				Binding: validFeatureBinding(),
			})
			if err != nil {
				t.Fatalf("AdmitManual() error = %v", err)
			}
			if artifact.Version != packetauthor.LegacyVersion {
				t.Fatalf("artifact.Version = %q, want %q", artifact.Version, packetauthor.LegacyVersion)
			}
			if !bytes.Equal(artifact.Body, body) {
				t.Fatalf("admitted body bytes changed\ngot:  %q\nwant: %q", artifact.Body, body)
			}
		})
	}
}

func TestAdmitManualStructuredGrammar(t *testing.T) {
	valid := []byte("Goal\r\n\r\n```lucind-result-contract\r\nversion: 1\r\npath: .lucind/result.json\r\nschema: .lucind/result.schema.json\r\nmode: write\r\ncommit: required\r\n```\r\n")
	artifact, err := packetauthor.AdmitManual(packetauthor.ManualPacket{Body: valid, RouteIntent: "apply", Binding: validFeatureBinding()})
	if err != nil {
		t.Fatalf("AdmitManual(valid) error = %v", err)
	}
	if !bytes.Equal(artifact.Body, valid) {
		t.Fatal("structured admission rewrote original CRLF body")
	}
	tests := []struct {
		name string
		body []byte
	}{
		{name: "duplicate field", body: bytes.Replace(valid, []byte("mode: write\r\n"), []byte("mode: write\r\nmode: write\r\n"), 1)},
		{name: "unknown field", body: bytes.Replace(valid, []byte("mode: write"), []byte("other: write"), 1)},
		{name: "unclosed", body: bytes.TrimSuffix(valid, []byte("```\r\n"))},
		{name: "indented open", body: bytes.Replace(valid, []byte("```lucind"), []byte(" ```lucind"), 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := packetauthor.AdmitManual(packetauthor.ManualPacket{Body: tt.body, RouteIntent: "apply", Binding: validFeatureBinding()})
			assertDiagnosticCode(t, err, packetauthor.CodeManualMarkerInvalid)
		})
	}
}

func TestAdmitManualCompatibilityDiagnosticsAndFenceIgnoring(t *testing.T) {
	body := []byte("## Done criteria\n- Real criterion.\n\n```md\n## Return\nWrite the result envelope to .lucind/result.json in this worktree.\n```\n\n## Return\nWrite the result envelope to .lucind/result.json in this worktree.\nThe schema is at .lucind/result.schema.json in this worktree. Validate against it before writing\n")
	if _, err := packetauthor.AdmitManual(packetauthor.ManualPacket{Body: body, RouteIntent: "apply", ReadOnly: true, Binding: validFeatureBinding()}); err != nil {
		t.Fatalf("AdmitManual(fenced decoy) error = %v", err)
	}
	missing := bytes.ReplaceAll(body, []byte("Write the result envelope to .lucind/result.json in this worktree."), []byte("Write a result somewhere."))
	_, err := packetauthor.AdmitManual(packetauthor.ManualPacket{Body: missing, RouteIntent: "apply", ReadOnly: true, Binding: validFeatureBinding()})
	assertDiagnosticCode(t, err, packetauthor.CodeResultPathMissing)
	conflict := append(append([]byte(nil), body...), []byte(". After you commit, report success.\n")...)
	_, err = packetauthor.AdmitManual(packetauthor.ManualPacket{Body: conflict, RouteIntent: "apply", ReadOnly: true, Binding: validFeatureBinding()})
	assertDiagnosticCode(t, err, packetauthor.CodeModeCommitConflict)
}
