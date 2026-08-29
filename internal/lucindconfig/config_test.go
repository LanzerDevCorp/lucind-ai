package lucindconfig_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/lucindconfig"
)

func TestLoadConfig(t *testing.T) {
	t.Run("valid role stacks and optional budget", func(t *testing.T) {
		rawYAML := `
skill_budget: 4
skills:
  apply:
    - go-testing
    - lucind-apply
  verify:
    - lucind-verify
  lens:
    - cognitive-doc-design
  synthesis:
    - cognitive-doc-design
`
		cfg, err := lucindconfig.ParseBytes([]byte(rawYAML))
		if err != nil {
			t.Fatalf("ParseBytes returned unexpected error: %v", err)
		}

		if cfg.SkillBudget == nil || *cfg.SkillBudget != 4 {
			t.Fatalf("expected SkillBudget 4, got %v", cfg.SkillBudget)
		}

		expectedSkills := map[string][]string{
			"apply":     {"go-testing", "lucind-apply"},
			"verify":    {"lucind-verify"},
			"lens":      {"cognitive-doc-design"},
			"synthesis": {"cognitive-doc-design"},
		}

		if !reflect.DeepEqual(cfg.Skills, expectedSkills) {
			t.Fatalf("expected skills %v, got %v", expectedSkills, cfg.Skills)
		}

		// Verify StackSkills helper
		applySkills := cfg.StackSkills("apply")
		if !reflect.DeepEqual(applySkills, []string{"go-testing", "lucind-apply"}) {
			t.Errorf("StackSkills(apply) = %v, want %v", applySkills, []string{"go-testing", "lucind-apply"})
		}

		verifySkills := cfg.StackSkills("verify")
		if !reflect.DeepEqual(verifySkills, []string{"lucind-verify"}) {
			t.Errorf("StackSkills(verify) = %v, want %v", verifySkills, []string{"lucind-verify"})
		}

		unknownRole := cfg.StackSkills("nonexistent")
		if unknownRole != nil {
			t.Errorf("StackSkills(nonexistent) = %v, want nil", unknownRole)
		}
	})

	t.Run("omitted optional skill budget", func(t *testing.T) {
		rawYAML := `
skills:
  apply:
    - go-testing
`
		cfg, err := lucindconfig.ParseBytes([]byte(rawYAML))
		if err != nil {
			t.Fatalf("ParseBytes returned unexpected error: %v", err)
		}

		if cfg.SkillBudget != nil {
			t.Fatalf("expected nil SkillBudget when omitted, got %v", *cfg.SkillBudget)
		}

		applySkills := cfg.StackSkills("apply")
		if !reflect.DeepEqual(applySkills, []string{"go-testing"}) {
			t.Errorf("StackSkills(apply) = %v, want %v", applySkills, []string{"go-testing"})
		}
	})

	t.Run("zero skill budget is preserved", func(t *testing.T) {
		rawYAML := `
skill_budget: 0
skills: {}
`
		cfg, err := lucindconfig.ParseBytes([]byte(rawYAML))
		if err != nil {
			t.Fatalf("ParseBytes returned unexpected error: %v", err)
		}

		if cfg.SkillBudget == nil || *cfg.SkillBudget != 0 {
			t.Fatalf("expected SkillBudget 0, got %v", cfg.SkillBudget)
		}
	})

	t.Run("malformed YAML fails", func(t *testing.T) {
		rawYAML := `
skills:
  apply: [unclosed list
`
		_, err := lucindconfig.ParseBytes([]byte(rawYAML))
		if err == nil {
			t.Fatal("expected error parsing malformed YAML, got nil")
		}
	})

	t.Run("unknown top-level key rejected via KnownFields", func(t *testing.T) {
		rawYAML := `
skill_budget: 3
unknown_key: true
skills:
  apply:
    - go-testing
`
		_, err := lucindconfig.ParseBytes([]byte(rawYAML))
		if err == nil {
			t.Fatal("expected error rejecting unknown top-level key, got nil")
		}
		if !strings.Contains(err.Error(), "unknown_key") && !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected error message to indicate unknown field, got: %v", err)
		}
	})

	t.Run("unrelated configuration key rejected via KnownFields", func(t *testing.T) {
		rawYAML := `
skill_roots:
  - ~/.claude/skills
skills:
  apply:
    - go-testing
`
		_, err := lucindconfig.ParseBytes([]byte(rawYAML))
		if err == nil {
			t.Fatal("expected error rejecting skill_roots in lucind.yaml, got nil")
		}
	})

	t.Run("empty and comment-only YAML returns empty Config without error", func(t *testing.T) {
		for _, rawYAML := range []string{"", "   \n\n", "# Only comments\n# in this file\n"} {
			cfg, err := lucindconfig.ParseBytes([]byte(rawYAML))
			if err != nil {
				t.Fatalf("unexpected error for empty/comment YAML %q: %v", rawYAML, err)
			}
			if cfg.SkillBudget != nil {
				t.Errorf("expected nil SkillBudget for empty YAML, got %v", *cfg.SkillBudget)
			}
			if len(cfg.Skills) != 0 {
				t.Errorf("expected empty Skills for empty YAML, got %v", cfg.Skills)
			}
		}
	})

	t.Run("file loader Load and LoadFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, lucindconfig.ConfigFileName)

		content := `
skill_budget: 5
skills:
  apply:
    - go-testing
`
		if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		// Test LoadFile
		cfg1, err := lucindconfig.LoadFile(configPath)
		if err != nil {
			t.Fatalf("LoadFile failed: %v", err)
		}
		if cfg1.SkillBudget == nil || *cfg1.SkillBudget != 5 {
			t.Errorf("LoadFile SkillBudget = %v, want 5", cfg1.SkillBudget)
		}

		// Test Load from directory
		cfg2, err := lucindconfig.Load(tmpDir)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if cfg2.SkillBudget == nil || *cfg2.SkillBudget != 5 {
			t.Errorf("Load SkillBudget = %v, want 5", cfg2.SkillBudget)
		}

		// Test Load from direct file path
		cfg3, err := lucindconfig.Load(configPath)
		if err != nil {
			t.Fatalf("Load(configPath) failed: %v", err)
		}
		if cfg3.SkillBudget == nil || *cfg3.SkillBudget != 5 {
			t.Errorf("Load(configPath) SkillBudget = %v, want 5", cfg3.SkillBudget)
		}
	})

	t.Run("missing file returns empty config on Load and error on LoadFile", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Load on directory with no lucind.yaml returns empty config, no error (zero-config)
		cfg, err := lucindconfig.Load(tmpDir)
		if err != nil {
			t.Fatalf("Load on directory with no config returned error: %v", err)
		}
		if cfg.SkillBudget != nil {
			t.Errorf("expected nil SkillBudget, got %v", *cfg.SkillBudget)
		}
		if len(cfg.Skills) != 0 {
			t.Errorf("expected empty Skills, got %v", cfg.Skills)
		}

		// LoadFile on non-existent path returns error
		_, err = lucindconfig.LoadFile(filepath.Join(tmpDir, "lucind.yaml"))
		if err == nil {
			t.Fatal("expected error from LoadFile on missing file, got nil")
		}
	})

	t.Run("StackSkills returns defensive copy", func(t *testing.T) {
		rawYAML := `
skills:
  apply:
    - go-testing
    - lucind-apply
`
		cfg, err := lucindconfig.ParseBytes([]byte(rawYAML))
		if err != nil {
			t.Fatalf("ParseBytes error: %v", err)
		}

		skills1 := cfg.StackSkills("apply")
		skills1[0] = "mutated-skill"

		skills2 := cfg.StackSkills("apply")
		if skills2[0] != "go-testing" {
			t.Fatalf("StackSkills did not return a defensive copy; mutated value was %q", skills2[0])
		}
	})

	t.Run("root repository lucind.yaml loads cleanly", func(t *testing.T) {
		// Root lucind.yaml is two levels up from internal/lucindconfig
		rootPath := filepath.Join("..", "..", lucindconfig.ConfigFileName)
		cfg, err := lucindconfig.LoadFile(rootPath)
		if err != nil {
			t.Fatalf("failed to load root repository %s: %v", lucindconfig.ConfigFileName, err)
		}
		if cfg.SkillBudget == nil || *cfg.SkillBudget != 3 {
			t.Errorf("root %s SkillBudget = %v, want 3", lucindconfig.ConfigFileName, cfg.SkillBudget)
		}
	})
}
