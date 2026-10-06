package agyplugin

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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
		PreInvocation []struct{ Type, Command string }
		Stop          []struct{ Type, Command string }
	}
	if err := json.Unmarshal(data, &hooks); err != nil {
		t.Fatalf("hooks.json does not parse: %v\n%s", err, data)
	}
	g, ok := hooks["lucind-hooks"]
	if !ok || len(g.PreToolUse) != 1 || len(g.PreInvocation) != 1 || len(g.Stop) != 1 {
		t.Fatalf("unexpected hooks shape: %s", data)
	}
	quoted := `'/opt/lu cind/it'\''s/lucind-ai'`
	if got := g.PreToolUse[0].Hooks[0].Command; got != quoted+" hook pre-tool-use" || g.PreToolUse[0].Matcher != "*" || g.PreToolUse[0].Hooks[0].Type != "command" {
		t.Errorf("pre-tool-use command = %q", got)
	}
	if got := g.PreInvocation[0].Command; got != quoted+" hook pre-invocation" || g.PreInvocation[0].Type != "command" {
		t.Errorf("pre-invocation command = %q", got)
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

type fakeAgy struct {
	calls   [][]string
	list    string
	install string
	err     map[string]error
}

func (f *fakeAgy) Run(ctx context.Context, args ...string) ([]byte, error) {
	f.calls = append(f.calls, args)
	key := strings.Join(args[:2], " ")
	if e := f.err[key]; e != nil {
		return []byte("boom"), e
	}
	switch key {
	case "plugin list":
		return []byte(f.list), nil
	case "plugin install":
		return []byte(f.install), nil
	}
	return []byte("ok"), nil
}

const goodInstall = "\x1b[32m[ok]\x1b[0m lucind\n  \u2714 hooks       : 1 processed\n"

func TestSetup_RegistersViaAgyPluginInstall(t *testing.T) {
	staging, obsolete := t.TempDir(), filepath.Join(t.TempDir(), "lucind")
	if err := os.MkdirAll(obsolete, 0o755); err != nil {
		t.Fatal(err)
	}
	agy := &fakeAgy{list: `{"imports":[{"name":"other"}]}`, install: goodInstall}
	dir, err := Setup(context.Background(), Options{StagingRoot: staging, Bin: "/a/lucind-ai", Agy: agy, ObsoleteDir: obsolete})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if want := filepath.Join(staging, "lucind"); dir != want {
		t.Fatalf("dir = %s, want %s", dir, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "hooks.json")); err != nil {
		t.Errorf("staging not rendered: %v", err)
	}
	want := [][]string{{"plugin", "list"}, {"plugin", "install", dir}}
	if !reflect.DeepEqual(agy.calls, want) {
		t.Errorf("agy calls = %v, want %v", agy.calls, want)
	}
	if _, err := os.Stat(obsolete); !os.IsNotExist(err) {
		t.Errorf("obsolete unloaded copy not removed: %v", err)
	}
}

func TestSetup_NoImportedPluginsPlainText(t *testing.T) {
	agy := &fakeAgy{list: "No imported plugins.\n", install: goodInstall}
	dir, err := Setup(context.Background(), Options{StagingRoot: t.TempDir(), Bin: "/a/lucind-ai", Agy: agy})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	want := [][]string{{"plugin", "list"}, {"plugin", "install", dir}}
	if !reflect.DeepEqual(agy.calls, want) {
		t.Errorf("agy calls = %v, want %v", agy.calls, want)
	}
}

func TestSetup_UninstallsExistingImportFirst(t *testing.T) {
	agy := &fakeAgy{list: `{"imports":[{"name":"lucind"},{"name":"x"}]}`, install: goodInstall}
	dir, err := Setup(context.Background(), Options{StagingRoot: t.TempDir(), Bin: "/a/lucind-ai", Agy: agy})
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"plugin", "list"}, {"plugin", "uninstall", "lucind"}, {"plugin", "install", dir}}
	if !reflect.DeepEqual(agy.calls, want) {
		t.Errorf("agy calls = %v, want %v", agy.calls, want)
	}
}

func TestSetup_FailsWhenAgyMissing(t *testing.T) {
	agy := &fakeAgy{err: map[string]error{"plugin list": ErrAgyNotFound}}
	_, err := Setup(context.Background(), Options{StagingRoot: t.TempDir(), Bin: "/a/lucind-ai", Agy: agy})
	if err == nil || !strings.Contains(err.Error(), "agy") {
		t.Fatalf("expected clear agy error, got %v", err)
	}
}

func TestSetup_KeepsObsoleteWhenInstallFails(t *testing.T) {
	obsolete := filepath.Join(t.TempDir(), "lucind")
	_ = os.MkdirAll(obsolete, 0o755)
	agy := &fakeAgy{list: `{}`, install: "[error] lucind\n", err: nil}
	if _, err := Setup(context.Background(), Options{StagingRoot: t.TempDir(), Bin: "/a/lucind-ai", Agy: agy, ObsoleteDir: obsolete}); err == nil {
		t.Fatal("expected failure on [error] output")
	}
	if _, err := os.Stat(obsolete); err != nil {
		t.Errorf("obsolete removed despite failure: %v", err)
	}
	agy = &fakeAgy{list: `{}`, err: map[string]error{"plugin install": errors.New("exit 1")}}
	if _, err := Setup(context.Background(), Options{StagingRoot: t.TempDir(), Bin: "/a/lucind-ai", Agy: agy}); err == nil {
		t.Fatal("expected failure on agy exit error")
	}
}

func TestCheckInstallOutput(t *testing.T) {
	if err := checkInstallOutput(goodInstall); err != nil {
		t.Errorf("good output rejected: %v", err)
	}
	for name, out := range map[string]string{
		"no hooks":    "[ok]    lucind\n  - hooks       : skipped (not found)\n",
		"zero hooks":  "  \u2714 hooks       : 0 processed\n",
		"error shown": "[error] lucind\n  \u2714 hooks       : 1 processed\n",
	} {
		if err := checkInstallOutput(out); err == nil {
			t.Errorf("%s: expected rejection", name)
		}
	}
}

func TestStagingRoot_HonorsXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/x/data")
	got, err := StagingRoot()
	if err != nil || got != "/x/data/lucind-ai/agy-plugin" {
		t.Errorf("StagingRoot = %q, %v", got, err)
	}
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "/h")
	got, _ = StagingRoot()
	if got != "/h/.local/share/lucind-ai/agy-plugin" {
		t.Errorf("fallback StagingRoot = %q", got)
	}
}

func TestInstallRoles_WritesPluginTree(t *testing.T) {
	root := t.TempDir()
	dir, err := InstallRoles(root)
	if err != nil {
		t.Fatalf("InstallRoles: %v", err)
	}
	if want := filepath.Join(root, RolesName); dir != want {
		t.Fatalf("dir = %s, want %s", dir, want)
	}
	for _, rel := range []string{"plugin.json", "agents/worker.md"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}
	var manifest struct{ Name string }
	data, _ := os.ReadFile(filepath.Join(dir, "plugin.json"))
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.Name != RolesName {
		t.Fatalf("plugin.json name = %q, err %v", manifest.Name, err)
	}
	worker, _ := os.ReadFile(filepath.Join(dir, "agents/worker.md"))
	if len(worker) == 0 {
		t.Errorf("agents/worker.md is empty")
	}
}

func TestInstallRoles_OverwritesAndRemovesStaleFiles(t *testing.T) {
	root := t.TempDir()
	dir, err := InstallRoles(root)
	if err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dir, "agents", "old.md")
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallRoles(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale file survived reinstall: %v", err)
	}
}

func TestCheckRolesInstallOutput(t *testing.T) {
	good := "\x1b[32m[ok]\x1b[0m lucind-roles\n  \u2714 agents      : 1 processed\n"
	if err := checkRolesInstallOutput(good); err != nil {
		t.Errorf("good output rejected: %v", err)
	}
	for name, out := range map[string]string{
		"no agents":   "[ok]    lucind-roles\n  - agents      : skipped (not found)\n",
		"zero agents": "  \u2714 agents      : 0 processed\n",
		"error shown": "[error] lucind-roles\n  \u2714 agents      : 1 processed\n",
		"fail shown":  "[fail]  lucind-roles\n  \u2714 agents      : 1 processed\n",
	} {
		if err := checkRolesInstallOutput(out); err == nil {
			t.Errorf("%s: expected rejection", name)
		}
	}
}

func TestSetupRoles_RegistersViaAgyPluginInstall(t *testing.T) {
	staging := t.TempDir()
	goodRolesInstall := "\x1b[32m[ok]\x1b[0m lucind-roles\n  \u2714 agents      : 1 processed\n"
	agy := &fakeAgy{list: `{"imports":[{"name":"lucind"}]}`, install: goodRolesInstall}
	dir, err := SetupRoles(context.Background(), Options{StagingRoot: staging, Agy: agy})
	if err != nil {
		t.Fatalf("SetupRoles: %v", err)
	}
	if want := filepath.Join(staging, RolesName); dir != want {
		t.Fatalf("dir = %s, want %s", dir, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "agents", "worker.md")); err != nil {
		t.Errorf("staging not rendered: %v", err)
	}
	want := [][]string{{"plugin", "list"}, {"plugin", "install", dir}}
	if !reflect.DeepEqual(agy.calls, want) {
		t.Errorf("agy calls = %v, want %v", agy.calls, want)
	}
}

func TestSetupRoles_UninstallsExistingImportFirst(t *testing.T) {
	staging := t.TempDir()
	goodRolesInstall := "\x1b[32m[ok]\x1b[0m lucind-roles\n  \u2714 agents      : 1 processed\n"
	agy := &fakeAgy{list: `{"imports":[{"name":"lucind-roles"},{"name":"other"}]}`, install: goodRolesInstall}
	dir, err := SetupRoles(context.Background(), Options{StagingRoot: staging, Agy: agy})
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"plugin", "list"}, {"plugin", "uninstall", RolesName}, {"plugin", "install", dir}}
	if !reflect.DeepEqual(agy.calls, want) {
		t.Errorf("agy calls = %v, want %v", agy.calls, want)
	}
}

func TestSetupRoles_FailsWhenAgyMissing(t *testing.T) {
	agy := &fakeAgy{err: map[string]error{"plugin list": ErrAgyNotFound}}
	_, err := SetupRoles(context.Background(), Options{StagingRoot: t.TempDir(), Agy: agy})
	if err == nil || !strings.Contains(err.Error(), "agy") {
		t.Fatalf("expected clear agy error, got %v", err)
	}
}

func TestSetupRoles_FailsWhenInstallReportsError(t *testing.T) {
	agy := &fakeAgy{list: `{}`, install: "[error] lucind-roles\n", err: nil}
	if _, err := SetupRoles(context.Background(), Options{StagingRoot: t.TempDir(), Agy: agy}); err == nil {
		t.Fatal("expected failure on [error] output")
	}
	agy = &fakeAgy{list: `{}`, err: map[string]error{"plugin install": errors.New("exit 1")}}
	if _, err := SetupRoles(context.Background(), Options{StagingRoot: t.TempDir(), Agy: agy}); err == nil {
		t.Fatal("expected failure on agy exit error")
	}
}
