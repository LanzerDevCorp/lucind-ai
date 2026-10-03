// Package agytrust registers lane worktrees as trusted workspaces for the Antigravity CLI.
//
// Interactive `agy` blocks on a "do you trust this folder" prompt for every folder it has not
// seen, and trust is stored per exact path (a trusted parent does not cover its children). The
// file is the owner's global agy configuration, so every write is conservative: the file is
// parsed first, unknown keys are preserved, a file that cannot be parsed is never rewritten,
// and the write is atomic.
package agytrust

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const trustedKey = "trustedWorkspaces"

// DefaultSettingsPath is the Antigravity CLI settings file.
func DefaultSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("agytrust: resolve home directory: %w", err)
	}
	return filepath.Join(home, ".gemini", "antigravity-cli", "settings.json"), nil
}

// Add trusts workspace. It is a no-op (the file is not rewritten) when already trusted.
func Add(settingsPath, workspace string) error {
	return update(settingsPath, workspace, true)
}

// Remove drops workspace from the trusted list. A missing file or entry is a no-op.
func Remove(settingsPath, workspace string) error {
	return update(settingsPath, workspace, false)
}

func canonical(workspace string) (string, error) {
	if !filepath.IsAbs(workspace) {
		return "", fmt.Errorf("agytrust: workspace %q must be an absolute path", workspace)
	}
	clean := filepath.Clean(workspace)
	if resolved, err := filepath.EvalSymlinks(clean); err == nil {
		return resolved, nil
	}
	return clean, nil
}

func update(settingsPath, workspace string, add bool) error {
	path, err := canonical(workspace)
	if err != nil {
		return err
	}

	unlock, err := lock(settingsPath)
	if err != nil {
		return err
	}
	defer unlock()

	raw, mode, err := readSettings(settingsPath)
	if errors.Is(err, os.ErrNotExist) {
		if !add {
			return nil
		}
		raw, mode = nil, 0o600
	} else if err != nil {
		return err
	}

	doc := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &doc); err != nil || doc == nil {
			return fmt.Errorf("agytrust: %s is not a JSON object; refusing to modify it", settingsPath)
		}
	}
	var trusted []string
	if v, ok := doc[trustedKey]; ok {
		if err := json.Unmarshal(v, &trusted); err != nil {
			return fmt.Errorf("agytrust: %s: %s is not a list of strings; refusing to modify it", settingsPath, trustedKey)
		}
	}

	present := -1
	for i, t := range trusted {
		if t == path {
			present = i
			break
		}
	}
	switch {
	case add && present >= 0, !add && present < 0:
		return nil
	case add:
		trusted = append(trusted, path)
	default:
		trusted = append(trusted[:present], trusted[present+1:]...)
	}
	if trusted == nil {
		trusted = []string{}
	}

	encoded, err := json.Marshal(trusted)
	if err != nil {
		return err
	}
	doc[trustedKey] = encoded
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(settingsPath, append(out, '\n'), mode)
}

func readSettings(path string) ([]byte, os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	return raw, info.Mode().Perm(), nil
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("agytrust: create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".settings-*.tmp")
	if err != nil {
		return fmt.Errorf("agytrust: temp file: %w", err)
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// lock serializes concurrent lucind-ai processes with an advisory lock next to the settings file.
func lock(settingsPath string) (func(), error) {
	dir := filepath.Dir(settingsPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("agytrust: create %s: %w", dir, err)
	}
	f, err := os.OpenFile(settingsPath+".lucind.lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("agytrust: open lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("agytrust: lock: %w", err)
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
