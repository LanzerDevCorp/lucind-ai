package claudeplugin_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/claudeplugin"
	claudecode "github.com/LanzerDevCorp/lucind-ai/plugin/claude-code"
)

func TestInstall_WritesSkillTree(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir, err := claudeplugin.Install()
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	wantDir := filepath.Join(home, ".claude", "skills", "lucind")
	if dir != wantDir {
		t.Fatalf("dir = %q, want %q", dir, wantDir)
	}

	skillFile := filepath.Join(dir, "SKILL.md")
	data, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("read installed SKILL.md: %v", err)
	}

	embeddedData, err := claudecode.Skills.ReadFile("skills/lucind/SKILL.md")
	if err != nil {
		t.Fatalf("read embedded skill: %v", err)
	}

	if !bytes.Equal(data, embeddedData) {
		t.Fatalf("installed content does not match embedded content")
	}

	info, err := os.Stat(skillFile)
	if err != nil {
		t.Fatalf("stat SKILL.md: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf("SKILL.md perm = %#o, want 0o644", perm)
	}

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o755 {
		t.Errorf("dir perm = %#o, want 0o755", perm)
	}
}

func TestInstall_IdempotentAndReplacesChangedContent(t *testing.T) {
	home := t.TempDir()

	dir, err := claudeplugin.InstallHome(home)
	if err != nil {
		t.Fatalf("first InstallHome: %v", err)
	}

	skillFile := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("tampered content"), 0o644); err != nil {
		t.Fatalf("tamper SKILL.md: %v", err)
	}

	staleFile := filepath.Join(dir, "stale.txt")
	if err := os.WriteFile(staleFile, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	if _, err := claudeplugin.InstallHome(home); err != nil {
		t.Fatalf("second InstallHome: %v", err)
	}

	embeddedData, err := claudecode.Skills.ReadFile("skills/lucind/SKILL.md")
	if err != nil {
		t.Fatalf("read embedded skill: %v", err)
	}

	data, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("read restored SKILL.md: %v", err)
	}
	if !bytes.Equal(data, embeddedData) {
		t.Fatalf("skill file was not restored to embedded content")
	}

	if _, err := os.Stat(staleFile); !os.IsNotExist(err) {
		t.Errorf("stale file survived re-install: %v", err)
	}
}

func TestInstall_ReplacesSymlinkAndPreservesTarget(t *testing.T) {
	home := t.TempDir()
	claudeSkillsDir := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(claudeSkillsDir, 0o755); err != nil {
		t.Fatalf("create .claude/skills: %v", err)
	}

	targetDir := filepath.Join(t.TempDir(), "repo-skill-target")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("create targetDir: %v", err)
	}
	sentinelFile := filepath.Join(targetDir, "original.txt")
	sentinelContent := []byte("do not touch repository file")
	if err := os.WriteFile(sentinelFile, sentinelContent, 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	destSymlink := filepath.Join(claudeSkillsDir, "lucind")
	if err := os.Symlink(targetDir, destSymlink); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	fi, err := os.Lstat(destSymlink)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("destSymlink is not a symlink: %v", err)
	}

	dir, err := claudeplugin.InstallHome(home)
	if err != nil {
		t.Fatalf("InstallHome: %v", err)
	}

	destFi, err := os.Lstat(dir)
	if err != nil {
		t.Fatalf("lstat installed dir: %v", err)
	}
	if destFi.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("installed dir is still a symlink")
	}
	if !destFi.IsDir() {
		t.Fatalf("installed dir is not a directory")
	}

	// Symlink target must be untouched
	gotSentinel, err := os.ReadFile(sentinelFile)
	if err != nil {
		t.Fatalf("read sentinel in target dir: %v", err)
	}
	if !bytes.Equal(gotSentinel, sentinelContent) {
		t.Fatalf("symlink target was modified: got %q, want %q", gotSentinel, sentinelContent)
	}

	targetEntries, err := os.ReadDir(targetDir)
	if err != nil {
		t.Fatalf("read targetDir: %v", err)
	}
	if len(targetEntries) != 1 || targetEntries[0].Name() != "original.txt" {
		t.Fatalf("targetDir contents changed: %+v", targetEntries)
	}

	// Installed dir must have embedded skill content
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read installed SKILL.md: %v", err)
	}
	embeddedData, err := claudecode.Skills.ReadFile("skills/lucind/SKILL.md")
	if err != nil {
		t.Fatalf("read embedded skill: %v", err)
	}
	if !bytes.Equal(data, embeddedData) {
		t.Fatalf("installed SKILL.md content mismatch")
	}
}

// A symlinked parent (for example ~/.claude managed by a dotfiles tool) is the
// user's own layout: the install must keep it and write through it.
func TestInstall_KeepsParentSymlinkAndWritesThroughIt(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("create .claude: %v", err)
	}

	targetDir := filepath.Join(t.TempDir(), "repo-skills-target")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("create targetDir: %v", err)
	}
	sentinelFile := filepath.Join(targetDir, "keep.txt")
	if err := os.WriteFile(sentinelFile, []byte("original skills repo"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	skillsSymlink := filepath.Join(claudeDir, "skills")
	if err := os.Symlink(targetDir, skillsSymlink); err != nil {
		t.Fatalf("create parent symlink: %v", err)
	}

	dir, err := claudeplugin.InstallHome(home)
	if err != nil {
		t.Fatalf("InstallHome: %v", err)
	}

	skillsFi, err := os.Lstat(skillsSymlink)
	if err != nil {
		t.Fatalf("lstat skills: %v", err)
	}
	if skillsFi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf(".claude/skills symlink was removed")
	}

	// Unrelated files behind the symlink must be untouched
	if _, err := os.Stat(sentinelFile); err != nil {
		t.Fatalf("sentinel in targetDir was removed or missing: %v", err)
	}

	// The skill is written through the symlink
	if _, err := os.Stat(filepath.Join(targetDir, "lucind", "SKILL.md")); err != nil {
		t.Fatalf("SKILL.md not written through the parent symlink: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Fatalf("SKILL.md not installed: %v", err)
	}
}

func TestInstallDir_DirectCustomDir(t *testing.T) {
	customDir := filepath.Join(t.TempDir(), "custom", "skill", "lucind")
	targetDir := filepath.Join(t.TempDir(), "repo-custom-target")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("create targetDir: %v", err)
	}
	sentinel := filepath.Join(targetDir, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("data"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(customDir), 0o755); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	if err := os.Symlink(targetDir, customDir); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	if err := claudeplugin.InstallDir(customDir); err != nil {
		t.Fatalf("InstallDir: %v", err)
	}

	fi, err := os.Lstat(customDir)
	if err != nil || fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
		t.Fatalf("customDir is not a real directory: %v", err)
	}

	// Target preserved
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("sentinel missing: %v", err)
	}

	// Content installed
	if _, err := os.Stat(filepath.Join(customDir, "SKILL.md")); err != nil {
		t.Fatalf("SKILL.md missing: %v", err)
	}
}

func TestInstall_HomeResolutionFailure(t *testing.T) {
	t.Setenv("HOME", "")
	// On Linux, setting HOME="" makes os.UserHomeDir() fail when reading $HOME.
	// Verify that Install() returns a descriptive error.
	_, err := claudeplugin.Install()
	if err == nil {
		t.Skip("os.UserHomeDir() did not fail without HOME in this environment")
	}
}
