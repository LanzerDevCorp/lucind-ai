package skillcontent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newFixtureRepo builds a minimal fake repo tree under t.TempDir() with
// plugin.json, marketplace.json, a small skill dir, and no recorded hash
// file yet -- exactly enough for Bump/Verify to operate on the well-known
// relative paths.
func newFixtureRepo(t *testing.T) (repoRoot string) {
	t.Helper()
	repoRoot = t.TempDir()

	mustWrite := func(rel, content string) {
		t.Helper()
		path := filepath.Join(repoRoot, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", path, err)
		}
	}

	mustWrite(PluginManifestRelPath, `{
  "name": "lucind-ai",
  "version": "1.0.0",
  "description": "test fixture"
}
`)
	mustWrite(MarketplaceManifestRelPath, `{
  "name": "lucind-ai",
  "plugins": [
    {
      "name": "lucind-ai",
      "version": "1.0.0",
      "description": "test fixture"
    }
  ]
}
`)
	mustWrite(filepath.Join(SkillDirRelPath, "SKILL.md"), "# hello\n")
	mustWrite(filepath.Join(SkillDirRelPath, "references", "core", "domain.md"), "domain content v1\n")

	return repoRoot
}

func TestBumpThenVerifyRoundTrips(t *testing.T) {
	repoRoot := newFixtureRepo(t)

	if err := Bump(repoRoot, "1.1.0"); err != nil {
		t.Fatalf("Bump: %v", err)
	}

	if err := Verify(repoRoot); err != nil {
		t.Fatalf("Verify after Bump: %v", err)
	}

	pv, err := ReadPluginVersion(repoRoot)
	if err != nil {
		t.Fatalf("ReadPluginVersion: %v", err)
	}
	if pv != "1.1.0" {
		t.Errorf("ReadPluginVersion = %q, want %q", pv, "1.1.0")
	}

	mv, err := ReadMarketplaceVersion(repoRoot)
	if err != nil {
		t.Fatalf("ReadMarketplaceVersion: %v", err)
	}
	if mv != "1.1.0" {
		t.Errorf("ReadMarketplaceVersion = %q, want %q", mv, "1.1.0")
	}
}

// TestBumpPreservesUnrelatedManifestFields proves Bump does a targeted
// in-place field replacement rather than an encoding/json round-trip, which
// would silently reorder keys and re-indent the whole file for a one-field
// change.
func TestBumpPreservesUnrelatedManifestFields(t *testing.T) {
	repoRoot := newFixtureRepo(t)

	before, err := os.ReadFile(filepath.Join(repoRoot, PluginManifestRelPath))
	if err != nil {
		t.Fatalf("ReadFile before Bump: %v", err)
	}

	if err := Bump(repoRoot, "1.1.0"); err != nil {
		t.Fatalf("Bump: %v", err)
	}

	after, err := os.ReadFile(filepath.Join(repoRoot, PluginManifestRelPath))
	if err != nil {
		t.Fatalf("ReadFile after Bump: %v", err)
	}

	if !strings.Contains(string(after), `"name": "lucind-ai"`) {
		t.Errorf("plugin.json after Bump lost the unrelated \"name\" field:\n%s", after)
	}
	if !strings.Contains(string(after), `"description": "test fixture"`) {
		t.Errorf("plugin.json after Bump lost the unrelated \"description\" field:\n%s", after)
	}
	if got, want := string(before), string(after); strings.Count(got, "\n") != strings.Count(want, "\n") {
		t.Errorf("Bump changed the line count of plugin.json (before %d lines, after %d lines); want only the version field's value to change", strings.Count(got, "\n"), strings.Count(want, "\n"))
	}
}

// TestVerifyFailsWhenSkillContentDriftsWithoutBump reproduces the defect
// this package fixes at the unit level: editing the skill tree without
// running `make bump-plugin-version` must be caught by `make
// verify-plugin-content`, but must NOT be something go test / lucind-ai
// check enforces automatically.
func TestVerifyFailsWhenSkillContentDriftsWithoutBump(t *testing.T) {
	repoRoot := newFixtureRepo(t)

	if err := Bump(repoRoot, "1.0.0"); err != nil {
		t.Fatalf("Bump (seed recorded hash): %v", err)
	}
	if err := Verify(repoRoot); err != nil {
		t.Fatalf("Verify immediately after Bump should pass: %v", err)
	}

	// Edit skill content without bumping -- exactly what an isolated feature
	// branch does today, and what must no longer fail go test.
	domainPath := filepath.Join(repoRoot, SkillDirRelPath, "references", "core", "domain.md")
	if err := os.WriteFile(domainPath, []byte("domain content v2 -- edited\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := Verify(repoRoot)
	if err == nil {
		t.Fatal("Verify after unbumped skill content edit = nil error, want a drift error")
	}
	if !strings.Contains(err.Error(), "skill content changed") {
		t.Errorf("Verify error = %q, want it to explicitly call out skill content drift", err)
	}
}

// TestBumpFixesDriftDetectedByVerify proves the make bump-plugin-version ->
// make verify-plugin-content sequence a human runs actually converges: after
// a content edit trips Verify, re-running Bump against the current tree
// makes Verify pass again.
func TestBumpFixesDriftDetectedByVerify(t *testing.T) {
	repoRoot := newFixtureRepo(t)

	if err := Bump(repoRoot, "1.0.0"); err != nil {
		t.Fatalf("Bump (seed): %v", err)
	}

	domainPath := filepath.Join(repoRoot, SkillDirRelPath, "references", "core", "domain.md")
	if err := os.WriteFile(domainPath, []byte("domain content v2 -- edited\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := Verify(repoRoot); err == nil {
		t.Fatal("Verify before re-Bump = nil error, want a drift error to set up this test")
	}

	if err := Bump(repoRoot, "1.0.1"); err != nil {
		t.Fatalf("Bump (fix drift): %v", err)
	}

	if err := Verify(repoRoot); err != nil {
		t.Fatalf("Verify after re-Bump: %v", err)
	}
}

func TestHashDirIsOrderAndPathSensitive(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	write := func(dir, rel, content string) {
		t.Helper()
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	write(dirA, "a.md", "same content")
	write(dirB, "b.md", "same content")

	sumA, err := HashDir(dirA)
	if err != nil {
		t.Fatalf("HashDir(dirA): %v", err)
	}
	sumB, err := HashDir(dirB)
	if err != nil {
		t.Fatalf("HashDir(dirB): %v", err)
	}
	if sumA == sumB {
		t.Errorf("HashDir produced identical hashes for differently-named files with identical content: %q", sumA)
	}

	sumAAgain, err := HashDir(dirA)
	if err != nil {
		t.Fatalf("HashDir(dirA) again: %v", err)
	}
	if sumA != sumAAgain {
		t.Errorf("HashDir(dirA) not deterministic across runs: %q vs %q", sumA, sumAAgain)
	}
}

func TestVerifySkillCopyDetectsByteDrift(t *testing.T) {
	repoRoot := t.TempDir()
	canonical := filepath.Join(repoRoot, SkillDirRelPath)
	copy := filepath.Join(repoRoot, OpenCodeSkillDirRelPath)
	for _, tree := range []string{canonical, copy} {
		if err := os.MkdirAll(tree, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tree, "SKILL.md"), []byte("same\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := VerifySkillCopy(repoRoot); err != nil {
		t.Fatalf("VerifySkillCopy identical trees: %v", err)
	}
	if err := os.WriteFile(filepath.Join(copy, "SKILL.md"), []byte("drift\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifySkillCopy(repoRoot); err == nil || !strings.Contains(err.Error(), "drifted") {
		t.Fatalf("VerifySkillCopy drift error = %v", err)
	}
}
