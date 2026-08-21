package dag

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EmitPacketContent formats the frontmatter and body for a single packet Node.
// The frontmatter includes id, executor, routed_by, model (if set), and a single-line
// JSON array for allowed_paths. The body is the Markdown at body_path verbatim.
func EmitPacketContent(node Node, baseDir string) (string, error) {
	fullBodyPath := filepath.Join(baseDir, node.BodyPath)
	bodyBytes, err := os.ReadFile(fullBodyPath)
	if err != nil {
		return "", fmt.Errorf("dag: failed to read body_path for %q: %w", node.ID, err)
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("id: %s\n", node.ID))
	b.WriteString(fmt.Sprintf("executor: %s\n", node.Executor))
	b.WriteString(fmt.Sprintf("routed_by: %s\n", node.RoutedBy))
	if node.Model != "" {
		b.WriteString(fmt.Sprintf("model: %s\n", node.Model))
	}
	if node.Feature != "" {
		b.WriteString(fmt.Sprintf("feature: %s\n", node.Feature))
	}
	if node.ParentRef != "" {
		b.WriteString(fmt.Sprintf("parent_ref: %s\n", node.ParentRef))
	}
	if node.BaseSHA != "" {
		b.WriteString(fmt.Sprintf("base_sha: %s\n", node.BaseSHA))
	}
	if node.ExpectedParentSHA != "" {
		b.WriteString(fmt.Sprintf("expected_parent_sha: %s\n", node.ExpectedParentSHA))
	}
	if node.LegacyMain {
		b.WriteString("legacy_main: true\n")
	}
	pathsJSON, err := json.Marshal(node.AllowedPaths)
	if err != nil {
		return "", fmt.Errorf("dag: failed to marshal allowed_paths for %q: %w", node.ID, err)
	}
	b.WriteString(fmt.Sprintf("allowed_paths: %s\n", string(pathsJSON)))
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimLeft(string(bodyBytes), "\n"))

	return b.String(), nil
}

// Emit generates and writes the .md packet files for all nodes in d into outDir.
func Emit(d DAG, baseDir string, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("dag: failed to create outDir %s: %w", outDir, err)
	}

	for _, node := range d.Packets {
		content, err := EmitPacketContent(node, baseDir)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(outDir, node.ID+".md")
		if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("dag: failed to write packet file %s: %w", targetPath, err)
		}
	}

	return nil
}
