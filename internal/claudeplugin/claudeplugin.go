package claudeplugin

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	claudecode "github.com/LanzerDevCorp/lucind-ai/plugin/claude-code"
)

// SkillDir returns the path to the lucind Claude skill directory for the given home directory.
func SkillDir(home string) string {
	return filepath.Join(home, ".claude", "skills", "lucind")
}

// Install installs the Claude skill into ~/.claude/skills/lucind using VariantManual.
func Install() (string, error) {
	return InstallVariant(VariantManual)
}

// InstallVariant installs the Claude skill with the specified variant into ~/.claude/skills/lucind.
func InstallVariant(v Variant) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return InstallHomeVariant(home, v)
}

// InstallHome installs the Claude skill into <home>/.claude/skills/lucind using VariantManual.
func InstallHome(home string) (string, error) {
	return InstallHomeVariant(home, VariantManual)
}

// InstallHomeVariant installs the Claude skill with the specified variant into <home>/.claude/skills/lucind.
func InstallHomeVariant(home string, v Variant) (string, error) {
	dir := SkillDir(home)
	if err := installTo(dir, v); err != nil {
		return "", err
	}
	return dir, nil
}

// InstallDir installs the Claude skill into targetDir using VariantManual.
func InstallDir(targetDir string) error {
	return InstallDirVariant(targetDir, VariantManual)
}

// InstallDirVariant installs the Claude skill with the specified variant into targetDir.
func InstallDirVariant(targetDir string, v Variant) error {
	return installTo(targetDir, v)
}

func installTo(targetDir string, v Variant) error {
	targetDir = filepath.Clean(targetDir)

	// Only targetDir itself is replaced when it is a symlink (the old Makefile linked it into
	// the repository), so we never write through it. Parent directories such as ~/.claude may
	// be symlinks that belong to the user's own layout (for example a dotfiles manager); they
	// are left alone and written through.
	if fi, err := os.Lstat(targetDir); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(targetDir); err != nil {
			return fmt.Errorf("remove symlink %s: %w", targetDir, err)
		}
	}

	// Remove targetDir if it already exists (ensures clean directory and idempotency).
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("remove previous skill directory: %w", err)
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create skill directory: %w", err)
	}

	sub, err := fs.Sub(claudecode.Skills, "skills/lucind")
	if err != nil {
		return fmt.Errorf("open embedded skill tree: %w", err)
	}

	err = fs.WalkDir(sub, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		destPath := filepath.Join(targetDir, filepath.FromSlash(p))
		if p == "." {
			return nil
		}
		if d.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}
		data, err := fs.ReadFile(sub, p)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", p, err)
		}
		if filepath.Base(p) == "SKILL.md" {
			rendered, err := RenderSkill(data, v)
			if err != nil {
				return fmt.Errorf("render %s: %w", p, err)
			}
			data = rendered
		}
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", filepath.Dir(destPath), err)
		}
		perm := os.FileMode(0o644)
		if info, err := d.Info(); err == nil && info.Mode()&0111 != 0 {
			perm = 0o755
		}
		return os.WriteFile(destPath, data, perm)
	})
	if err != nil {
		return fmt.Errorf("write skill files: %w", err)
	}

	return nil
}
