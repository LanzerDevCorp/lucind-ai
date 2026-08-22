package dag

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EmitPacketContent formats the frontmatter and body for a single packet Node.
// The frontmatter includes id, executor, routed_by, model (if set), agent (if
// set), a single-line JSON array for allowed_paths, and a single-line JSON
// array for read_only_paths (if non-empty). The body is the Markdown at
// body_path, with any leading YAML frontmatter block of its own stripped --
// see stripLeadingFrontmatter's doc comment for why that defends against a
// real dispatch failure, not just a style nit.
//
// read_only_paths is deliberately NOT emitted under the key "read_only":
// internal/packet.Packet already reserves that exact frontmatter key for an
// unrelated, pre-existing boolean ("this whole packet must produce no
// commits") parsed strictly as the literal string "true" or "false"
// (internal/packet/packet.go's ErrInvalidReadOnly). Emitting the Strict-TDD
// path list under the same key would make packet.Parse reject every packet
// split from a node that declares it.
func EmitPacketContent(node Node, baseDir string) (string, error) {
	fullBodyPath := filepath.Join(baseDir, node.BodyPath)
	bodyBytes, err := os.ReadFile(fullBodyPath)
	if err != nil {
		return "", fmt.Errorf("dag: failed to read body_path for %q: %w", node.ID, err)
	}
	bodyBytes = stripLeadingFrontmatter(bodyBytes)

	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("id: %s\n", node.ID))
	b.WriteString(fmt.Sprintf("executor: %s\n", node.Executor))
	b.WriteString(fmt.Sprintf("routed_by: %s\n", node.RoutedBy))
	if node.Model != "" {
		b.WriteString(fmt.Sprintf("model: %s\n", node.Model))
	}
	if node.Agent != "" {
		b.WriteString(fmt.Sprintf("agent: %s\n", node.Agent))
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
	if len(node.ReadOnly) > 0 {
		readOnlyJSON, err := json.Marshal(node.ReadOnly)
		if err != nil {
			return "", fmt.Errorf("dag: failed to marshal read_only_paths for %q: %w", node.ID, err)
		}
		b.WriteString(fmt.Sprintf("read_only_paths: %s\n", string(readOnlyJSON)))
	}
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimLeft(string(bodyBytes), "\n"))

	return b.String(), nil
}

// stripLeadingFrontmatter removes one leading "---\n...\n---\n" YAML block
// from body, if present, and trims the blank line(s) that follow it.
//
// Emit always writes its own frontmatter ahead of the body (built from the
// Node's fields, carrying the live SHAs a plan author cannot know in
// advance). If body_path's own content also opens with a frontmatter block
// -- e.g. because it was authored as a complete standalone packet rather
// than pure body text, or because outDir and body_path's directory are the
// same and a prior split already prepended one -- the result is two stacked
// frontmatter blocks. internal/packet.Parse only consumes the first one, so
// the second becomes literal body text starting with "---": passed as an
// executor's prompt, that leading "-" is indistinguishable from a CLI flag
// to an argv parser expecting a flag or a positional message (confirmed
// against opencode's yargs-based CLI, which prints its own --help and exits
// 1 instead of dispatching). Stripping any such block here makes Emit
// idempotent regardless of how body_path was authored or whether it has
// already been split once before.
func stripLeadingFrontmatter(body []byte) []byte {
	trimmed := strings.TrimLeft(string(body), "\n")
	if !strings.HasPrefix(trimmed, "---\n") && trimmed != "---" {
		return body
	}
	rest := strings.TrimPrefix(trimmed, "---\n")
	closing := strings.Index(rest, "\n---")
	if closing == -1 {
		return body
	}
	after := rest[closing+len("\n---"):]
	after = strings.TrimPrefix(after, "\n")
	return []byte(strings.TrimLeft(after, "\n"))
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
