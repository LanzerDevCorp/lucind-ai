package skillroots_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/skillroots"
)

func createSkillFile(t *testing.T, rootDir, skillName, content string, mode os.FileMode) string {
	t.Helper()
	skillDir := filepath.Join(rootDir, skillName)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("failed to create skill dir %s: %v", skillDir, err)
	}
	skillPath := filepath.Join(skillDir, skillroots.SkillFileName)
	if err := os.WriteFile(skillPath, []byte(content), mode); err != nil {
		t.Fatalf("failed to write %s: %v", skillPath, err)
	}
	return skillPath
}

func TestResolveRoots(t *testing.T) {
	tempDir := t.TempDir()
	root1 := filepath.Join(tempDir, "root1")
	root2 := filepath.Join(tempDir, "root2")

	// Skill A exists in root1 and root2 with different content -> root1 should win (first match).
	pathA1 := createSkillFile(t, root1, "skill-a", "# Skill A from Root 1\n", 0o644)
	_ = createSkillFile(t, root2, "skill-a", "# Skill A from Root 2\n", 0o644)

	// Skill B exists only in root2 -> root2 should resolve.
	pathB2 := createSkillFile(t, root2, "skill-b", "# Skill B from Root 2\n", 0o644)

	roots := []string{root1, root2}
	resolver := skillroots.NewResolver(roots)

	// Case 1: First-match priority.
	resolvedA, err := resolver.Resolve("skill-a")
	if err != nil {
		t.Fatalf("unexpected error resolving skill-a: %v", err)
	}
	if resolvedA != pathA1 {
		t.Errorf("ordered search failed: expected %s (root1), got %s", pathA1, resolvedA)
	}

	// Case 2: Multi-root fallback to second root.
	resolvedB, err := resolver.Resolve("skill-b")
	if err != nil {
		t.Fatalf("unexpected error resolving skill-b: %v", err)
	}
	if resolvedB != pathB2 {
		t.Errorf("multi-root search failed: expected %s (root2), got %s", pathB2, resolvedB)
	}

	// Case 3: Top-level Resolve helper function.
	resolvedHelper, err := skillroots.Resolve(roots, "skill-b")
	if err != nil {
		t.Fatalf("unexpected error from top-level Resolve: %v", err)
	}
	if resolvedHelper != pathB2 {
		t.Errorf("top-level Resolve mismatch: expected %s, got %s", pathB2, resolvedHelper)
	}
}

func TestResolveRoots_TildeExpansion(t *testing.T) {
	fakeHome := t.TempDir()
	claudeSkillsDir := filepath.Join(fakeHome, ".claude", "skills")
	skillPath := createSkillFile(t, claudeSkillsDir, "sdd-propose", "# sdd-propose skill\n", 0o644)

	// Configure root with tilde: ~/.claude/skills
	roots := []string{"~/.claude/skills"}
	resolver := skillroots.NewResolverWithHome(roots, fakeHome)

	resolved, err := resolver.Resolve("sdd-propose")
	if err != nil {
		t.Fatalf("failed to resolve tilde-expanded skill: %v", err)
	}
	if resolved != skillPath {
		t.Errorf("tilde expansion failed: expected %s, got %s", skillPath, resolved)
	}

	// Direct ExpandTilde test
	expanded, err := skillroots.ExpandTilde("~/.claude/skills")
	if err != nil {
		t.Fatalf("ExpandTilde failed: %v", err)
	}
	if strings.HasPrefix(expanded, "~") {
		t.Errorf("ExpandTilde did not expand leading tilde: %s", expanded)
	}

	// Test ExpandTildeWithHome for bare "~" and non-tilde paths
	if skillroots.ExpandTildeWithHome("~", "/home/user") != "/home/user" {
		t.Errorf("expected bare ~ to expand to /home/user")
	}
	if skillroots.ExpandTildeWithHome("/var/skills", "/home/user") != "/var/skills" {
		t.Errorf("expected /var/skills to remain unchanged")
	}
}

func TestResolveRoots_MissingSkillDiagnostics(t *testing.T) {
	tempDir := t.TempDir()
	root1 := filepath.Join(tempDir, "root1")
	root2 := filepath.Join(tempDir, "root2")
	if err := os.MkdirAll(root1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root2, 0o755); err != nil {
		t.Fatal(err)
	}

	roots := []string{root1, root2}
	resolver := skillroots.NewResolver(roots)

	_, err := resolver.Resolve("non-existent-skill")
	if err == nil {
		t.Fatal("expected error for non-existent skill, got nil")
	}

	// Verify fail-closed error semantics
	if !errors.Is(err, skillroots.ErrSkillNotFound) {
		t.Errorf("expected error to wrap ErrSkillNotFound, got %v", err)
	}

	var notFoundErr *skillroots.SkillNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected error of type *SkillNotFoundError, got %T: %v", err, err)
	}

	if notFoundErr.Skill != "non-existent-skill" {
		t.Errorf("expected missing skill %q, got %q", "non-existent-skill", notFoundErr.Skill)
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "non-existent-skill") {
		t.Errorf("error message should cite missing skill: %s", errStr)
	}
	if !strings.Contains(errStr, root1) || !strings.Contains(errStr, root2) {
		t.Errorf("error message should cite searched roots: %s", errStr)
	}
}

func TestResolveRoots_EmptyRootsAndEmptySkill(t *testing.T) {
	resolver := skillroots.NewResolver([]string{})

	// Resolving in empty roots returns SkillNotFoundError
	_, err := resolver.Resolve("some-skill")
	if err == nil {
		t.Fatal("expected error when resolving with empty roots, got nil")
	}
	if !errors.Is(err, skillroots.ErrSkillNotFound) {
		t.Errorf("expected ErrSkillNotFound, got %v", err)
	}

	// Empty skill name returns ErrEmptySkillName
	_, err = resolver.Resolve("")
	if !errors.Is(err, skillroots.ErrEmptySkillName) {
		t.Errorf("expected ErrEmptySkillName, got %v", err)
	}
}

func TestResolveRoots_DirectoryInsteadOfFile(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "skills")
	skillDir := filepath.Join(root, "invalid-skill")
	// Make SKILL.md a directory instead of a regular file
	skillMdAsDir := filepath.Join(skillDir, skillroots.SkillFileName)
	if err := os.MkdirAll(skillMdAsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	resolver := skillroots.NewResolver([]string{root})
	_, err := resolver.Resolve("invalid-skill")
	if err == nil {
		t.Fatal("expected error when SKILL.md is a directory, got nil")
	}
	if !errors.Is(err, skillroots.ErrSkillNotFound) {
		t.Errorf("expected ErrSkillNotFound, got %v", err)
	}
}

func TestResolveRootsLoadsSkillMarkdownAsData(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "skills")
	trapFile := filepath.Join(tempDir, "trap_executed.txt")

	// Create a SKILL.md that looks like an executable script designed to create a trap file if executed.
	scriptContent := "#!/bin/sh\ntouch " + trapFile + "\nexit 0\n"
	skillPath := createSkillFile(t, root, "executable-skill", scriptContent, 0o755)

	// Ensure the trap file does not exist.
	_ = os.Remove(trapFile)

	resolver := skillroots.NewResolver([]string{root})
	resolvedPath, err := resolver.Resolve("executable-skill")
	if err != nil {
		t.Fatalf("unexpected resolution error: %v", err)
	}
	if resolvedPath != skillPath {
		t.Errorf("expected path %s, got %s", skillPath, resolvedPath)
	}

	// Read the resolved file content to verify data-only loading.
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		t.Fatalf("failed to read skill file as data: %v", err)
	}
	if string(data) != scriptContent {
		t.Errorf("expected skill content %q, got %q", scriptContent, string(data))
	}

	// Verify that the executable file was NEVER executed.
	if _, err := os.Stat(trapFile); err == nil {
		t.Fatalf("CRITICAL: SKILL.md was executed instead of being treated strictly as data! Trap file %s exists.", trapFile)
	}
}

func TestLoadConfig(t *testing.T) {
	tempDir := t.TempDir()

	// Case 1: Valid config with roots map.
	validYaml := "roots:\n  - ~/.claude/skills\n  - .agents/skills\n"
	validConfigPath := filepath.Join(tempDir, "skill-roots.yaml")
	if err := os.WriteFile(validConfigPath, []byte(validYaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := skillroots.LoadConfig(validConfigPath)
	if err != nil {
		t.Fatalf("unexpected error loading valid config: %v", err)
	}
	if len(cfg.Roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(cfg.Roots))
	}
	if cfg.Roots[0] != "~/.claude/skills" || cfg.Roots[1] != ".agents/skills" {
		t.Errorf("unexpected roots parsed: %v", cfg.Roots)
	}

	// Case 2: Config with raw sequence format.
	seqYaml := "- ~/.claude/skills\n- .agents/skills\n"
	seqConfigPath := filepath.Join(tempDir, "skill-roots-seq.yaml")
	if err := os.WriteFile(seqConfigPath, []byte(seqYaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgSeq, err := skillroots.LoadConfig(seqConfigPath)
	if err != nil {
		t.Fatalf("unexpected error loading seq config: %v", err)
	}
	if len(cfgSeq.Roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(cfgSeq.Roots))
	}

	// Case 3: Missing config file fails closed.
	_, err = skillroots.LoadConfig(filepath.Join(tempDir, "does-not-exist.yaml"))
	if err == nil {
		t.Fatal("expected error loading missing config file, got nil")
	}
	if !errors.Is(err, skillroots.ErrMissingConfig) {
		t.Errorf("expected ErrMissingConfig, got %v", err)
	}

	// Case 4: Invalid YAML syntax fails closed.
	invalidYamlPath := filepath.Join(tempDir, "invalid.yaml")
	if err := os.WriteFile(invalidYamlPath, []byte("roots: [unclosed list"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = skillroots.LoadConfig(invalidYamlPath)
	if err == nil {
		t.Fatal("expected error loading malformed YAML, got nil")
	}
	if !errors.Is(err, skillroots.ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestResolveAll(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "skills")

	path1 := createSkillFile(t, root, "skill-1", "# Skill 1\n", 0o644)
	path2 := createSkillFile(t, root, "skill-2", "# Skill 2\n", 0o644)

	resolver := skillroots.NewResolver([]string{root})

	// Success case
	resolvedMap, err := resolver.ResolveAll([]string{"skill-1", "skill-2"})
	if err != nil {
		t.Fatalf("unexpected error in ResolveAll: %v", err)
	}
	if len(resolvedMap) != 2 {
		t.Fatalf("expected 2 resolved skills, got %d", len(resolvedMap))
	}
	if resolvedMap["skill-1"] != path1 || resolvedMap["skill-2"] != path2 {
		t.Errorf("unexpected resolve map: %v", resolvedMap)
	}

	// Ordered paths slice
	paths, err := resolver.ResolvePaths([]string{"skill-2", "skill-1"})
	if err != nil {
		t.Fatalf("unexpected error in ResolvePaths: %v", err)
	}
	if len(paths) != 2 || paths[0] != path2 || paths[1] != path1 {
		t.Errorf("unexpected ordered paths: %v", paths)
	}

	// Fail closed if any skill is missing
	_, err = resolver.ResolveAll([]string{"skill-1", "missing-skill"})
	if err == nil {
		t.Fatal("expected error when any skill is missing, got nil")
	}
	if !errors.Is(err, skillroots.ErrSkillNotFound) {
		t.Errorf("expected ErrSkillNotFound, got %v", err)
	}

	// ResolvePaths failure case
	_, err = resolver.ResolvePaths([]string{"missing-skill", "skill-1"})
	if err == nil {
		t.Fatal("expected error from ResolvePaths when skill is missing, got nil")
	}
	if !errors.Is(err, skillroots.ErrSkillNotFound) {
		t.Errorf("expected ErrSkillNotFound, got %v", err)
	}
}

func TestResolveFromConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "skills")
	path1 := createSkillFile(t, root, "skill-from-cfg", "# Skill Config\n", 0o644)

	configPath := filepath.Join(tempDir, "skill-roots.yaml")
	yamlContent := "roots:\n  - " + root + "\n"
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Test ResolveFromConfig
	p, err := skillroots.ResolveFromConfig(configPath, "skill-from-cfg")
	if err != nil {
		t.Fatalf("ResolveFromConfig failed: %v", err)
	}
	if p != path1 {
		t.Errorf("expected %s, got %s", path1, p)
	}

	// Test ResolveAllFromConfig
	allMap, err := skillroots.ResolveAllFromConfig(configPath, []string{"skill-from-cfg"})
	if err != nil {
		t.Fatalf("ResolveAllFromConfig failed: %v", err)
	}
	if allMap["skill-from-cfg"] != path1 {
		t.Errorf("expected %s, got %s", path1, allMap["skill-from-cfg"])
	}

	// Test ResolvePathsFromConfig
	paths, err := skillroots.ResolvePathsFromConfig(configPath, []string{"skill-from-cfg"})
	if err != nil {
		t.Fatalf("ResolvePathsFromConfig failed: %v", err)
	}
	if len(paths) != 1 || paths[0] != path1 {
		t.Errorf("expected [%s], got %v", path1, paths)
	}
}
