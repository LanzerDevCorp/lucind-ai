package dag_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/dag"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
)

// TestSplitTwoPacketOneEdgeEmitsOrderedWaveCommands (Phase 7.1) is the
// end-to-end seam for dag.Split: a two-node apply-dag.yaml with one
// depends_on edge must emit two packet files and exactly two stdout wave
// lines in dependency order, each round-tripping through packet.Parse with
// a non-empty AllowedPaths.
func TestSplitTwoPacketOneEdgeEmitsOrderedWaveCommands(t *testing.T) {
	tempDir := t.TempDir()
	bodiesDir := filepath.Join(tempDir, "bodies")
	if err := os.MkdirAll(bodiesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	rootBody := "## Goal\n\nRoot packet work that the leaf depends on.\n"
	leafBody := "## Goal\n\nLeaf packet work that runs after the root.\n"
	if err := os.WriteFile(filepath.Join(bodiesDir, "apply-root.md"), []byte(rootBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bodiesDir, "apply-leaf.md"), []byte(leafBody), 0o644); err != nil {
		t.Fatal(err)
	}

	yamlContent := `change: apply-dag-dispatch
packets:
  - id: apply-root
    executor: agy
    routed_by: root has no dependencies
    allowed_paths:
      - internal/root/
    depends_on: []
    body_path: bodies/apply-root.md
  - id: apply-leaf
    executor: agy
    routed_by: leaf depends on root
    allowed_paths:
      - internal/leaf/
    depends_on:
      - apply-root
    body_path: bodies/apply-leaf.md
`
	dagPath := filepath.Join(tempDir, "apply-dag.yaml")
	if err := os.WriteFile(dagPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(tempDir, "packets")
	var stdout bytes.Buffer
	if err := dag.Split(dagPath, outDir, &stdout); err != nil {
		t.Fatalf("Split() error = %v, want nil", err)
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("ReadDir(%s) error = %v", outDir, err)
	}
	var packetFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			packetFiles = append(packetFiles, e.Name())
		}
	}
	if len(packetFiles) != 2 {
		t.Fatalf("packet files in outDir = %v, want exactly 2", packetFiles)
	}

	// Task 7.2: stdout alone is the wave plan; split must never write waves.json.
	if _, err := os.Stat(filepath.Join(outDir, "waves.json")); !os.IsNotExist(err) {
		t.Fatalf("waves.json must not exist under outDir; Stat error = %v", err)
	}

	rootPath := filepath.Join(outDir, "apply-root.md")
	leafPath := filepath.Join(outDir, "apply-leaf.md")
	wantStdout := "lucind-ai run --packet " + rootPath + "\n" +
		"lucind-ai run --packet " + leafPath + "\n"
	if got := stdout.String(); got != wantStdout {
		t.Fatalf("stdout = %q, want exactly two ordered wave lines %q", got, wantStdout)
	}

	for _, path := range []string{rootPath, leafPath} {
		f, err := os.Open(path)
		if err != nil {
			t.Fatalf("Open(%s) error = %v", path, err)
		}
		p, err := packet.Parse(f)
		f.Close()
		if err != nil {
			t.Fatalf("packet.Parse(%s) error = %v, want a valid packet", path, err)
		}
		if len(p.AllowedPaths) == 0 {
			t.Errorf("packet.Parse(%s).AllowedPaths is empty, want non-empty", path)
		}
	}
}
