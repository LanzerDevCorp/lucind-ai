// Package skillcontent computes and verifies the content hash of the
// shipped Claude Code plugin skill tree
// (plugin/claude-code/skills/lucind-ai/**) against the version and hash
// recorded in internal/packet/testdata/skill_content_hash.txt.
//
// This logic used to run inside go test (internal/packet's
// TestPluginVersionGuardsSkillContent), which forced a plugin.json /
// marketplace.json version bump in the SAME commit as any skill-tree edit,
// on pain of failing go test ./... / lucind-ai check. With 2+ concurrently
// active isolated feature branches each independently touching the skill
// tree, that made the shared version field a mandatory, auto-bumped race:
// every branch bumped the same field to stay green, which in turn forced
// lucind-ai's own overlap-required reconciliation gate between every pair of
// them -- a gate with no support for resolving 3+ simultaneous overlaps in
// one retry pass. See the "fix(packet)" commit that introduced this package
// for the full incident writeup.
//
// The fix: a version bump is now always a deliberate, human-run action (see
// `make bump-plugin-version`), never a side effect of a content edit. The
// hash-vs-content check itself still exists -- as a human-run `make
// verify-plugin-content` check, typically run right before actually
// publishing/releasing the plugin -- but it is no longer wired into go test
// or `make install`.
//
// plugin.json and marketplace.json staying in lockstep with each other is a
// separate, still-blocking go test check
// (internal/packet/packet_test.go's TestPluginVersionGuardsSkillContent);
// this package does not duplicate it.
package skillcontent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Repo-root-relative paths this package reads and writes. Callers resolve
// them against a repoRoot (typically the current working directory when run
// via `make verify-plugin-content` / `make bump-plugin-version` from the
// repository root).
const (
	PluginManifestRelPath      = "plugin/claude-code/.claude-plugin/plugin.json"
	MarketplaceManifestRelPath = ".claude-plugin/marketplace.json"
	SkillDirRelPath            = "plugin/claude-code/skills/lucind-ai"
	HashRecordRelPath          = "internal/packet/testdata/skill_content_hash.txt"
	MarketplacePluginName      = "lucind-ai"
)

// PluginManifest mirrors the fields read from plugin.json.
type PluginManifest struct {
	Version string `json:"version"`
}

// MarketplaceManifest mirrors the fields read from marketplace.json.
type MarketplaceManifest struct {
	Plugins []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"plugins"`
}

// HashDir computes a deterministic SHA-256 over every regular file under
// dir, folding in each file's dir-relative, forward-slash path so a rename
// or an added/removed file changes the hash exactly as much as an edited
// one. Directory traversal order is the lexical order filepath.WalkDir
// already guarantees, so two runs over identical content always produce the
// identical hash.
func HashDir(dir string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		h.Write([]byte(filepath.ToSlash(rel)))
		h.Write([]byte{0})
		h.Write(data)
		h.Write([]byte{0})
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("hash skill content under %s: %w", dir, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ReadPluginVersion reads plugin.json's version field.
func ReadPluginVersion(repoRoot string) (string, error) {
	path := filepath.Join(repoRoot, PluginManifestRelPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("ReadFile(%s): %w", path, err)
	}
	var pm PluginManifest
	if err := json.Unmarshal(data, &pm); err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	if pm.Version == "" {
		return "", fmt.Errorf("%s: version is empty", path)
	}
	return pm.Version, nil
}

// ReadMarketplaceVersion reads marketplace.json's version for the
// MarketplacePluginName plugin entry.
func ReadMarketplaceVersion(repoRoot string) (string, error) {
	path := filepath.Join(repoRoot, MarketplaceManifestRelPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("ReadFile(%s): %w", path, err)
	}
	var mm MarketplaceManifest
	if err := json.Unmarshal(data, &mm); err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	for _, p := range mm.Plugins {
		if p.Name == MarketplacePluginName {
			if p.Version == "" {
				return "", fmt.Errorf("%s: plugin %q has an empty version", path, MarketplacePluginName)
			}
			return p.Version, nil
		}
	}
	return "", fmt.Errorf("%s: no plugin named %q in the plugins array", path, MarketplacePluginName)
}

// ReadRecordedHash reads HashRecordRelPath's "version: X" / "sha256: Y"
// lines.
func ReadRecordedHash(repoRoot string) (version, sum string, err error) {
	path := filepath.Join(repoRoot, HashRecordRelPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("ReadFile(%s): %w -- this file records the plugin version and skill-content hash `make verify-plugin-content` checks against; it must exist and be committed", path, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "version:"):
			version = strings.TrimSpace(strings.TrimPrefix(line, "version:"))
		case strings.HasPrefix(line, "sha256:"):
			sum = strings.TrimSpace(strings.TrimPrefix(line, "sha256:"))
		}
	}
	if version == "" || sum == "" {
		return "", "", fmt.Errorf("%s missing a \"version: X\" or \"sha256: Y\" line", path)
	}
	return version, sum, nil
}

// Verify recomputes the skill tree's content hash and compares it (and the
// version it is recorded against) to what HashRecordRelPath declares and
// what plugin.json currently declares. It returns a descriptive error
// naming exactly what drifted; it never bumps or rewrites anything -- see
// Bump for that.
func Verify(repoRoot string) error {
	pluginVersion, err := ReadPluginVersion(repoRoot)
	if err != nil {
		return err
	}

	recordedVersion, recordedSum, err := ReadRecordedHash(repoRoot)
	if err != nil {
		return err
	}

	if recordedVersion != pluginVersion {
		return fmt.Errorf("%s records version %q but plugin.json declares %q -- run `make bump-plugin-version` to bring them back in sync", HashRecordRelPath, recordedVersion, pluginVersion)
	}

	skillDir := filepath.Join(repoRoot, SkillDirRelPath)
	actualSum, err := HashDir(skillDir)
	if err != nil {
		return err
	}
	if actualSum != recordedSum {
		return fmt.Errorf("skill content changed; run `make bump-plugin-version` to bump plugin.json + marketplace.json and update the recorded hash in %s (want sha256 %s for the current %s tree, found sha256 %s recorded for version %q)", HashRecordRelPath, actualSum, SkillDirRelPath, recordedSum, recordedVersion)
	}
	return nil
}

// versionFieldPattern matches a top-level `"version": "X"` JSON field on its
// own line, capturing the quoted value so Bump can replace it in place
// without reformatting the rest of the file (encoding/json round-tripping
// would reorder keys and normalize indentation, producing a much noisier
// diff than a single field's value changing).
var versionFieldPattern = regexp.MustCompile(`("version"\s*:\s*)"[^"]*"`)

// bumpVersionField rewrites the first `"version": "..."` occurrence in data
// to newVersion, preserving all other formatting. It returns an error if no
// such field is found.
func bumpVersionField(data []byte, newVersion string) ([]byte, error) {
	if !versionFieldPattern.Match(data) {
		return nil, fmt.Errorf("no \"version\": \"...\" field found")
	}
	replaced := false
	out := versionFieldPattern.ReplaceAllFunc(data, func(match []byte) []byte {
		if replaced {
			return match
		}
		replaced = true
		sub := versionFieldPattern.FindSubmatch(match)
		return []byte(string(sub[1]) + `"` + newVersion + `"`)
	})
	return out, nil
}

// Bump rewrites plugin.json's and marketplace.json's version fields to
// newVersion and regenerates HashRecordRelPath's recorded version+hash for
// the CURRENT skill tree content. It is the only place a version bump
// should originate from now (see `make bump-plugin-version`); nothing under
// go test / `make install` calls this.
func Bump(repoRoot, newVersion string) error {
	if strings.TrimSpace(newVersion) == "" {
		return fmt.Errorf("newVersion must not be empty")
	}

	pluginPath := filepath.Join(repoRoot, PluginManifestRelPath)
	pluginData, err := os.ReadFile(pluginPath)
	if err != nil {
		return fmt.Errorf("ReadFile(%s): %w", pluginPath, err)
	}
	newPluginData, err := bumpVersionField(pluginData, newVersion)
	if err != nil {
		return fmt.Errorf("%s: %w", pluginPath, err)
	}

	marketplacePath := filepath.Join(repoRoot, MarketplaceManifestRelPath)
	marketplaceData, err := os.ReadFile(marketplacePath)
	if err != nil {
		return fmt.Errorf("ReadFile(%s): %w", marketplacePath, err)
	}
	newMarketplaceData, err := bumpVersionField(marketplaceData, newVersion)
	if err != nil {
		return fmt.Errorf("%s: %w", marketplacePath, err)
	}

	skillDir := filepath.Join(repoRoot, SkillDirRelPath)
	newSum, err := HashDir(skillDir)
	if err != nil {
		return err
	}

	if err := os.WriteFile(pluginPath, newPluginData, 0o644); err != nil {
		return fmt.Errorf("WriteFile(%s): %w", pluginPath, err)
	}
	if err := os.WriteFile(marketplacePath, newMarketplaceData, 0o644); err != nil {
		return fmt.Errorf("WriteFile(%s): %w", marketplacePath, err)
	}

	hashRecordPath := filepath.Join(repoRoot, HashRecordRelPath)
	if err := os.MkdirAll(filepath.Dir(hashRecordPath), 0o755); err != nil {
		return fmt.Errorf("MkdirAll(%s): %w", filepath.Dir(hashRecordPath), err)
	}
	record := fmt.Sprintf(
		"# Recorded content hash for %s/**, checked\n"+
			"# by `make verify-plugin-content` (internal/skillcontent.Verify).\n"+
			"#\n"+
			"# Regenerate BOTH lines below with `make bump-plugin-version` -- a deliberate,\n"+
			"# human-run action, never an automatic side effect of a skill-tree content edit.\n"+
			"version: %s\n"+
			"sha256: %s\n",
		SkillDirRelPath, newVersion, newSum,
	)
	if err := os.WriteFile(hashRecordPath, []byte(record), 0o644); err != nil {
		return fmt.Errorf("WriteFile(%s): %w", hashRecordPath, err)
	}

	return nil
}
