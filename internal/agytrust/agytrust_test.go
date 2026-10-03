package agytrust

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func write(t *testing.T, dir, content string, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	return p
}

func readTrusted(t *testing.T, p string) []string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Trusted []string `json:"trustedWorkspaces"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("settings not valid JSON: %v\n%s", err, b)
	}
	return doc.Trusted
}

func TestAddPreservesOtherKeysEntriesAndMode(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, `{"statusLine":{"type":"command","command":"x"},"trustedWorkspaces":["/a"]}`, 0o600)
	ws := filepath.Join(dir, "lane")
	if err := os.Mkdir(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Add(p, ws); err != nil {
		t.Fatalf("Add: %v", err)
	}
	got := readTrusted(t, p)
	resolved, _ := filepath.EvalSymlinks(ws)
	if len(got) != 2 || got[0] != "/a" || got[1] != resolved {
		t.Fatalf("trustedWorkspaces = %v, want [/a %s]", got, resolved)
	}
	b, _ := os.ReadFile(p)
	var doc map[string]json.RawMessage
	_ = json.Unmarshal(b, &doc)
	var compact bytes.Buffer
	if err := json.Compact(&compact, doc["statusLine"]); err != nil || compact.String() != `{"type":"command","command":"x"}` {
		t.Errorf("statusLine not preserved: %s (%v)", doc["statusLine"], err)
	}
	if info, _ := os.Stat(p); info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestAddAlreadyTrustedDoesNotRewriteFile(t *testing.T) {
	dir := t.TempDir()
	ws := filepath.Join(dir, "lane")
	_ = os.Mkdir(ws, 0o755)
	resolved, _ := filepath.EvalSymlinks(ws)
	orig := fmt.Sprintf("{\n \"trustedWorkspaces\": [%q]\n}", resolved)
	p := write(t, dir, orig, 0o600)
	if err := Add(p, ws); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != orig {
		t.Errorf("file rewritten although already trusted:\n%s", b)
	}
}

func TestAddCreatesMissingFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	ws := filepath.Join(dir, "lane")
	_ = os.Mkdir(ws, 0o755)
	if err := Add(p, ws); err != nil {
		t.Fatal(err)
	}
	if got := readTrusted(t, p); len(got) != 1 {
		t.Fatalf("trusted = %v", got)
	}
	if info, _ := os.Stat(p); info.Mode().Perm() != 0o600 {
		t.Errorf("new file mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestRefusesToRewriteWhatItCannotParse(t *testing.T) {
	for name, content := range map[string]string{
		"invalid json":     `{"trustedWorkspaces": [`,
		"not an object":    `["x"]`,
		"wrong type":       `{"trustedWorkspaces":"nope"}`,
		"non-string entry": `{"trustedWorkspaces":[1]}`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			p := write(t, dir, content, 0o600)
			if err := Add(p, dir); err == nil {
				t.Fatal("Add succeeded on unparseable settings")
			}
			if err := Remove(p, dir); err == nil {
				t.Fatal("Remove succeeded on unparseable settings")
			}
			if b, _ := os.ReadFile(p); string(b) != content {
				t.Errorf("file modified: %s", b)
			}
		})
	}
}

func TestRejectsRelativePath(t *testing.T) {
	p := write(t, t.TempDir(), `{}`, 0o600)
	if err := Add(p, "relative/dir"); err == nil {
		t.Fatal("relative path accepted")
	}
}

func TestRemoveOnlyDropsThatEntry(t *testing.T) {
	dir := t.TempDir()
	ws := filepath.Join(dir, "lane")
	_ = os.Mkdir(ws, 0o755)
	resolved, _ := filepath.EvalSymlinks(ws)
	p := write(t, dir, fmt.Sprintf(`{"trustedWorkspaces":["/a",%q,"/b"],"k":1}`, resolved), 0o600)
	if err := Remove(p, ws); err != nil {
		t.Fatal(err)
	}
	got := readTrusted(t, p)
	if len(got) != 2 || got[0] != "/a" || got[1] != "/b" {
		t.Fatalf("trusted = %v", got)
	}
	if err := Remove(p, ws); err != nil {
		t.Fatalf("removing an absent entry must be a no-op: %v", err)
	}
	if err := Remove(filepath.Join(dir, "missing.json"), ws); err != nil {
		t.Fatalf("removing from a missing file must be a no-op: %v", err)
	}
}

func TestConcurrentAddsKeepEveryEntry(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, `{"trustedWorkspaces":[]}`, 0o600)
	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		ws := filepath.Join(dir, fmt.Sprintf("lane-%d", i))
		_ = os.Mkdir(ws, 0o755)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := Add(p, ws); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("Add: %v", err)
	}
	if got := readTrusted(t, p); len(got) != n {
		t.Fatalf("trusted has %d entries, want %d: %v", len(got), n, got)
	}
}
