// Package agyplugin embeds the lucind agy plugin (manifest, hooks, rules,
// skills) and registers it with the agy CLI via `agy plugin install`.
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

// StagingRoot returns the directory the plugin is rendered into before
// being registered with `agy plugin install`.
func StagingRoot() (string, error) {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "lucind-ai", "agy-plugin"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "lucind-ai", "agy-plugin"), nil
}

// ObsoleteDir returns the old drop-in plugin location. agy validates but
// never loads a plugin placed there, so it is removed on install.
func ObsoleteDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, ".gemini", "antigravity-cli", "plugins", Name), nil
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

// Install renders the plugin into <root>/lucind, replacing any previous
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

// ErrAgyNotFound is returned by an Agy runner when agy is not on PATH.
var ErrAgyNotFound = errors.New("agy not found on PATH")

// Agy runs the agy CLI. It is a seam so tests never call the real agy.
type Agy interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

// ExecAgy runs the real agy binary found on PATH.
type ExecAgy struct{}

// Run executes `agy <args...>` and returns its combined output.
func (ExecAgy) Run(ctx context.Context, args ...string) ([]byte, error) {
	agy, err := exec.LookPath("agy")
	if err != nil {
		return nil, ErrAgyNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, agy, args...).CombinedOutput()
}

// checkInstallOutput accepts `agy plugin install` output only when it
// reports at least one processed hook and no error line.
func checkInstallOutput(out string) error {
	out = ansi.ReplaceAllString(out, "")
	if strings.Contains(out, "[error]") || strings.Contains(out, "[fail") {
		return fmt.Errorf("agy plugin install reported errors:\n%s", out)
	}
	m := hooksCount.FindStringSubmatch(out)
	if m == nil {
		return fmt.Errorf("agy plugin install did not report hooks:\n%s", out)
	}
	if n, _ := strconv.Atoi(m[1]); n < 1 {
		return fmt.Errorf("agy plugin install processed no hooks:\n%s", out)
	}
	return nil
}

// Options configures Setup.
type Options struct {
	StagingRoot string // where the plugin is rendered
	Bin         string // absolute path of the lucind-ai binary
	Agy         Agy
	ObsoleteDir string // old unloaded copy to remove after success; "" skips
}

func agyErr(op string, out []byte, err error) error {
	if errors.Is(err, ErrAgyNotFound) {
		return errors.New("agy is required on PATH to register the plugin (agy plugin install)")
	}
	return fmt.Errorf("agy %s: %w\n%s", op, err, out)
}

// Setup renders the plugin into the staging root and registers it with
// `agy plugin install`, replacing any previous registration. It returns the
// staging plugin directory.
func Setup(ctx context.Context, o Options) (string, error) {
	dir, err := Install(o.StagingRoot, o.Bin)
	if err != nil {
		return "", err
	}
	out, err := o.Agy.Run(ctx, "plugin", "list")
	if err != nil {
		return "", agyErr("plugin list", out, err)
	}
	var listed struct {
		Imports []struct {
			Name string `json:"name"`
		} `json:"imports"`
	}
	if err := json.Unmarshal([]byte(ansi.ReplaceAllString(string(out), "")), &listed); err != nil {
		return "", fmt.Errorf("parse agy plugin list output: %w\n%s", err, out)
	}
	for _, imp := range listed.Imports {
		if imp.Name == Name {
			if out, err := o.Agy.Run(ctx, "plugin", "uninstall", Name); err != nil {
				return "", agyErr("plugin uninstall", out, err)
			}
			break
		}
	}
	out, err = o.Agy.Run(ctx, "plugin", "install", dir)
	if err != nil {
		return "", agyErr("plugin install", out, err)
	}
	if err := checkInstallOutput(string(out)); err != nil {
		return "", err
	}
	if o.ObsoleteDir != "" {
		if err := os.RemoveAll(o.ObsoleteDir); err != nil {
			return "", fmt.Errorf("remove obsolete plugin copy: %w", err)
		}
	}
	return dir, nil
}
