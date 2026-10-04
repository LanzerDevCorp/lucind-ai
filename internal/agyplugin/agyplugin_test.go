package agyplugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstall_WritesPluginTree(t *testing.T) {
	root := t.TempDir()
	dir, err := Install(root, "/opt/lucind bin/lucind-ai")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if want := filepath.Join(root, "lucind"); dir != want {
		t.Fatalf("dir = %s, want %s", dir, want)
	}
	for _, rel := range []string{"plugin.json", "hooks.json", "rules/lucind-lane.md", "skills/lucind-result/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}
	var manifest struct{ Name string }
	data, _ := os.ReadFile(filepath.Join(dir, "plugin.json"))
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.Name != "lucind" {
		t.Fatalf("plugin.json name = %q, err %v", manifest.Name, err)
	}
	rule, _ := os.ReadFile(filepath.Join(dir, "rules/lucind-lane.md"))
	if !strings.Contains(string(rule), "trigger: model_decision") {
		t.Errorf("rule must be model_decision-triggered")
	}
}

func TestInstall_HooksUseAbsoluteBinary(t *testing.T) {
	root := t.TempDir()
	// A path with a space and a quote must survive both shell quoting and JSON.
	bin := "/opt/lu cind/it's/lucind-ai"
	dir, err := Install(root, bin)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var hooks map[string]struct {
		PreToolUse []struct {
			Matcher string
			Hooks   []struct{ Type, Command string }
		}
		Stop []struct{ Type, Command string }
	}
	if err := json.Unmarshal(data, &hooks); err != nil {
		t.Fatalf("hooks.json does not parse: %v\n%s", err, data)
	}
	g, ok := hooks["lucind-hooks"]
	if !ok || len(g.PreToolUse) != 1 || len(g.Stop) != 1 {
		t.Fatalf("unexpected hooks shape: %s", data)
	}
	quoted := `'/opt/lu cind/it'\''s/lucind-ai'`
	if got := g.PreToolUse[0].Hooks[0].Command; got != quoted+" hook pre-tool-use" || g.PreToolUse[0].Matcher != "*" || g.PreToolUse[0].Hooks[0].Type != "command" {
		t.Errorf("pre-tool-use command = %q", got)
	}
	if got := g.Stop[0].Command; got != quoted+" hook stop" || g.Stop[0].Type != "command" {
		t.Errorf("stop command = %q", got)
	}
	if strings.Contains(string(data), "__LUCIND_BIN__") {
		t.Errorf("placeholder left in hooks.json")
	}
}

func TestInstall_OverwritesAndRemovesStaleFiles(t *testing.T) {
	root := t.TempDir()
	dir, err := Install(root, "/a/lucind-ai")
	if err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dir, "rules", "old.md")
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hooks.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(root, "/b/lucind-ai"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale file survived reinstall: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "hooks.json"))
	if !strings.Contains(string(data), "/b/lucind-ai") {
		t.Errorf("hooks.json not refreshed: %s", data)
	}
}

func TestInstall_RejectsRelativeBinary(t *testing.T) {
	if _, err := Install(t.TempDir(), "lucind-ai"); err == nil {
		t.Fatal("expected error for relative binary path")
	}
}

func TestValidateOutput(t *testing.T) {
	ok := "[ok]    lucind\n  ✔ hooks       : 1 processed\n  ✔ skills      : 1 processed\n"
	if err := checkValidateOutput(ok); err != nil {
		t.Errorf("good output rejected: %v", err)
	}
	for name, out := range map[string]string{
		"no hooks":    "[ok]    lucind\n  - hooks       : skipped (not found)\n",
		"zero hooks":  "  ✔ hooks       : 0 processed\n",
		"error shown": "[error] lucind\n  ✔ hooks       : 1 processed\n",
	} {
		if err := checkValidateOutput(out); err == nil {
			t.Errorf("%s: expected rejection", name)
		}
	}
}
