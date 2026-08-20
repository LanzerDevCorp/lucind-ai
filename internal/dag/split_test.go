package dag_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/dag"
)

func TestSplit_TwoWaveDAGSuccess(t *testing.T) {
	tempDir := t.TempDir()
	bodiesDir := filepath.Join(tempDir, "bodies")
	if err := os.MkdirAll(bodiesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"apply-ledger", "apply-serve", "apply-run"} {
		body := "# Goal\n\nGoal for " + name + "\n\n## Done criteria\n\n- [ ] Done\n"
		if err := os.WriteFile(filepath.Join(bodiesDir, name+".md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	yamlContent := `change: apply-dag-dispatch
packets:
  - id: apply-ledger
    executor: agy
    routed_by: schema and CRUD isolated from HTTP
    allowed_paths:
      - internal/ledger/
    depends_on: []
    body_path: bodies/apply-ledger.md
  - id: apply-serve
    executor: cursor-agent
    routed_by: HTTP isolated after ledger exists
    allowed_paths:
      - internal/serve/
    depends_on:
      - apply-ledger
    body_path: bodies/apply-serve.md
  - id: apply-run
    executor: agy
    routed_by: run logic isolated after ledger exists
    allowed_paths:
      - internal/run/
    depends_on:
      - apply-ledger
    body_path: bodies/apply-run.md
`
	dagPath := filepath.Join(tempDir, "apply-dag.yaml")
	if err := os.WriteFile(dagPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(tempDir, "packets")
	var stdout bytes.Buffer

	if err := dag.Split(dagPath, outDir, &stdout); err != nil {
		t.Fatalf("Split failed unexpectedly: %v", err)
	}

	// Verify emitted files
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("failed to read outDir: %v", err)
	}

	fileNames := make([]string, len(entries))
	for i, e := range entries {
		fileNames[i] = e.Name()
	}
	expectedFiles := []string{"apply-ledger.md", "apply-run.md", "apply-serve.md"}
	for _, expected := range expectedFiles {
		found := false
		for _, fn := range fileNames {
			if fn == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected emitted file %s not found in %v", expected, fileNames)
		}
	}

	// No waves.json or any extra plan file written
	for _, fn := range fileNames {
		if strings.HasSuffix(fn, ".json") {
			t.Errorf("forbidden JSON plan file %s was written to outDir", fn)
		}
	}

	// Verify stdout lines
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 wave lines on stdout, got %d:\n%s", len(lines), stdout.String())
	}

	expectedLine1 := "lucind-ai run --packet " + filepath.Join(outDir, "apply-ledger.md")
	if lines[0] != expectedLine1 {
		t.Errorf("wave line 1 mismatch:\ngot:  %q\nwant: %q", lines[0], expectedLine1)
	}

	expectedLine2 := "lucind-ai run --packet " + filepath.Join(outDir, "apply-serve.md") + " --packet " + filepath.Join(outDir, "apply-run.md")
	if lines[1] != expectedLine2 {
		t.Errorf("wave line 2 mismatch:\ngot:  %q\nwant: %q", lines[1], expectedLine2)
	}
}

func TestSplit_FailuresWriteNoPacketFiles(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "duplicate id",
			yaml: `change: test
packets:
  - id: dup
    executor: agy
    routed_by: test
    allowed_paths: [internal/a/]
    depends_on: []
    body_path: bodies/dup.md
  - id: dup
    executor: agy
    routed_by: test
    allowed_paths: [internal/b/]
    depends_on: []
    body_path: bodies/dup.md`,
		},
		{
			name: "cycle",
			yaml: `change: test
packets:
  - id: p1
    executor: agy
    routed_by: test
    allowed_paths: [internal/a/]
    depends_on: [p2]
    body_path: bodies/p1.md
  - id: p2
    executor: agy
    routed_by: test
    allowed_paths: [internal/b/]
    depends_on: [p1]
    body_path: bodies/p2.md`,
		},
		{
			name: "same wave overlap without edge",
			yaml: `change: test
packets:
  - id: p1
    executor: agy
    routed_by: test
    allowed_paths: [internal/foo/]
    depends_on: []
    body_path: bodies/p1.md
  - id: p2
    executor: agy
    routed_by: test
    allowed_paths: [internal/foo/bar.go]
    depends_on: []
    body_path: bodies/p2.md`,
		},
		{
			name: "empty allowed_paths",
			yaml: `change: test
packets:
  - id: p1
    executor: agy
    routed_by: test
    allowed_paths: []
    depends_on: []
    body_path: bodies/p1.md`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			bodiesDir := filepath.Join(tempDir, "bodies")
			if err := os.MkdirAll(bodiesDir, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"dup", "p1", "p2"} {
				_ = os.WriteFile(filepath.Join(bodiesDir, name+".md"), []byte("# Goal"), 0o644)
			}

			dagPath := filepath.Join(tempDir, "apply-dag.yaml")
			if err := os.WriteFile(dagPath, []byte(tt.yaml), 0o644); err != nil {
				t.Fatal(err)
			}

			outDir := filepath.Join(tempDir, "packets")
			var stdout bytes.Buffer

			err := dag.Split(dagPath, outDir, &stdout)
			if err == nil {
				t.Fatalf("expected Split to return error for %s, got nil", tt.name)
			}

			// Verify no packet files written
			if entries, err := os.ReadDir(outDir); err == nil && len(entries) > 0 {
				t.Fatalf("expected no files in outDir on error, found %d files", len(entries))
			}
		})
	}
}
