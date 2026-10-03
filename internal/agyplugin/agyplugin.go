// Package agyplugin embeds the lucind agy plugin (manifest, hooks, rules,
// skills) and installs it into the agy CLI plugin directory.
package agyplugin

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Name is the plugin directory and manifest name.
const Name = "lucind"

const binPlaceholder = "__LUCIND_BIN__"

//go:embed assets
var assets embed.FS

// DefaultRoot returns the agy CLI global plugins directory.
func DefaultRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, ".gemini", "antigravity-cli", "plugins"), nil
}

// shellQuote wraps s in single quotes for a POSIX shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// renderHooks fills the hooks template with the shell-quoted binary path,
// JSON-escaped so the result stays valid JSON.
func renderHooks(tmpl []byte, bin string) ([]byte, error) {
	enc, err := json.Marshal(shellQuote(bin))
	if err != nil {
		return nil, err
	}
	// enc includes the surrounding quotes; the template already has them.
	inner := string(enc[1 : len(enc)-1])
	out := strings.ReplaceAll(string(tmpl), binPlaceholder, inner)
	if !json.Valid([]byte(out)) {
		return nil, errors.New("rendered hooks.json is not valid JSON")
	}
	return []byte(out), nil
}

// Install writes the plugin to <root>/lucind, replacing any previous
// install, with hook commands pointing at the absolute binary path bin.
// It returns the plugin directory.
func Install(root, bin string) (string, error) {
	if !filepath.IsAbs(bin) {
		return "", fmt.Errorf("binary path must be absolute, got %q", bin)
	}
	dir := filepath.Join(root, Name)
	if err := os.RemoveAll(dir); err != nil {
		return "", fmt.Errorf("remove previous plugin: %w", err)
	}
	err := fs.WalkDir(assets, "assets", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(p, "assets"), "/")
		if rel == "" {
			return os.MkdirAll(dir, 0o755)
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dir, filepath.FromSlash(rel)), 0o755)
		}
		data, err := assets.ReadFile(p)
		if err != nil {
			return err
		}
		if path.Base(rel) == "hooks.json.tmpl" {
			if data, err = renderHooks(data, bin); err != nil {
				return err
			}
			rel = path.Join(path.Dir(rel), "hooks.json")
		}
		return os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), data, 0o644)
	})
	if err != nil {
		return "", fmt.Errorf("write plugin: %w", err)
	}
	return dir, nil
}

var (
	ansi       = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	hooksCount = regexp.MustCompile(`hooks\s*:\s*(\d+) processed`)
)

// checkValidateOutput accepts `agy plugin validate` output only when it
// reports at least one processed hook and no error line.
func checkValidateOutput(out string) error {
	out = ansi.ReplaceAllString(out, "")
	if strings.Contains(out, "[error]") || strings.Contains(out, "[fail") {
		return fmt.Errorf("agy plugin validate reported errors:\n%s", out)
	}
	m := hooksCount.FindStringSubmatch(out)
	if m == nil {
		return fmt.Errorf("agy plugin validate did not report hooks:\n%s", out)
	}
	if n, _ := strconv.Atoi(m[1]); n < 1 {
		return fmt.Errorf("agy plugin validate processed no hooks:\n%s", out)
	}
	return nil
}

// Validate runs `agy plugin validate <dir>` when agy is on PATH and fails
// unless it reports the hooks. It returns false (and no error) when agy is
// not installed.
func Validate(ctx context.Context, dir string) (bool, error) {
	agy, err := exec.LookPath("agy")
	if err != nil {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, agy, "plugin", "validate", dir).CombinedOutput()
	if err != nil {
		return true, fmt.Errorf("agy plugin validate: %w\n%s", err, out)
	}
	return true, checkValidateOutput(string(out))
}
