package dag

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// Split is the mechanical consumer for apply-dag.yaml:
// 1. Parses the sidecar DAG artifact at dagPath (verifying schema and body_path existence).
// 2. Validates semantic rules (unique IDs, non-empty allowed_paths).
// 3. Groups packets into waves via Kahn's algorithm and verifies global overlap via ValidateGlobalOverlap (transitive reachability) before Emit.
// 4. Emits one packet file per node into outDir with generated frontmatter and verbatim body.
// 5. Prints one copy-pasteable "lucind-ai run" command per wave to stdout in dependency order.
//
// Stdout is the wave plan. A waves.json (or any other plan file) must not be written to disk.
func Split(dagPath, outDir string, stdout io.Writer) error {
	d, err := Parse(dagPath)
	if err != nil {
		return err
	}

	waves, err := Waves(d)
	if err != nil {
		return err
	}

	baseDir := filepath.Dir(dagPath)
	if err := Emit(d, baseDir, outDir); err != nil {
		return err
	}

	for _, wave := range waves {
		var flags []string
		for _, node := range wave {
			packetPath := filepath.Join(outDir, node.ID+".md")
			flags = append(flags, "--packet", packetPath)
		}
		if _, err := fmt.Fprintf(stdout, "lucind-ai run %s\n", strings.Join(flags, " ")); err != nil {
			return fmt.Errorf("dag: failed to write wave command to stdout: %w", err)
		}
	}

	return nil
}
