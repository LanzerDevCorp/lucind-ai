package modes

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LanzerDevCorp/lucind-ai/internal/userconfig"
)

// Roles.
const (
	RoleExplorer = "explorer"
	RoleWriter   = "writer"
	RoleVerifier = "verifier"
)

// Executors.
const (
	ExecutorNative = "native"
	ExecutorLane   = "lane"
	ExecutorCursor = "cursor"
)

// ErrNoConfig is returned when the modes configuration file does not exist.
var ErrNoConfig = errors.New("modes config not found")

// RoleConfig defines the executor and model used for a role.
type RoleConfig struct {
	Executor string `json:"executor"`
	Model    string `json:"model"`
}

// Route represents the resolved execution route for a role.
type Route struct {
	Mode     string
	Role     string
	Executor string
	Model    string
}

// Config represents the modes configuration.
type Config struct {
	Active string
	modes  map[string]map[string]RoleConfig
}

type rawConfig struct {
	Active string                           `json:"active"`
	Modes  map[string]map[string]RoleConfig `json:"modes"`
}

// MarshalJSON marshals Config to JSON.
func (c *Config) MarshalJSON() ([]byte, error) {
	modesMap := c.modes
	if modesMap == nil {
		modesMap = make(map[string]map[string]RoleConfig)
	}
	return json.Marshal(rawConfig{
		Active: c.Active,
		Modes:  modesMap,
	})
}

// UnmarshalJSON unmarshals Config from JSON.
func (c *Config) UnmarshalJSON(data []byte) error {
	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.Active = raw.Active
	c.modes = raw.Modes
	return nil
}

// Path returns the path to the user's modes.json file inside userconfig.Dir().
func Path() (string, error) {
	dir, err := userconfig.Dir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(dir, "modes.json"), nil
}

// Roles returns a copy of all supported role names.
func Roles() []string {
	return []string{RoleExplorer, RoleWriter, RoleVerifier}
}

func isValidRole(role string) bool {
	switch role {
	case RoleExplorer, RoleWriter, RoleVerifier:
		return true
	default:
		return false
	}
}

// Load loads and validates the modes configuration from the given path.
// If the file does not exist, it returns an error wrapping ErrNoConfig.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read config %s: %w", path, ErrNoConfig)
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	if raw.Modes == nil {
		raw.Modes = make(map[string]map[string]RoleConfig)
	}

	modeNames := make([]string, 0, len(raw.Modes))
	for m := range raw.Modes {
		modeNames = append(modeNames, m)
	}
	sort.Strings(modeNames)

	for _, mName := range modeNames {
		roleMap := raw.Modes[mName]
		roleNames := make([]string, 0, len(roleMap))
		for r := range roleMap {
			roleNames = append(roleNames, r)
		}
		sort.Strings(roleNames)

		for _, rName := range roleNames {
			if !isValidRole(rName) {
				return nil, fmt.Errorf("mode %q: unknown role %q", mName, rName)
			}
			entry := roleMap[rName]
			switch entry.Executor {
			case ExecutorNative, ExecutorLane:
				// valid
			case ExecutorCursor:
				return nil, fmt.Errorf("mode %q role %q: executor %q is reserved and not implemented yet", mName, rName, entry.Executor)
			default:
				return nil, fmt.Errorf("mode %q role %q: unknown executor %q", mName, rName, entry.Executor)
			}
			if strings.TrimSpace(entry.Model) == "" {
				return nil, fmt.Errorf("mode %q role %q: empty model", mName, rName)
			}
		}
	}

	if raw.Active != "" {
		if _, ok := raw.Modes[raw.Active]; !ok {
			return nil, fmt.Errorf("active mode %q does not exist", raw.Active)
		}
	}

	return &Config{
		Active: raw.Active,
		modes:  raw.Modes,
	}, nil
}

// Mode returns the role configurations for the given mode name, or false if not found.
func (c *Config) Mode(name string) (map[string]RoleConfig, bool) {
	m, ok := c.modes[name]
	if !ok {
		return nil, false
	}
	cp := make(map[string]RoleConfig, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp, true
}

// Modes returns a sorted list of all mode names.
func (c *Config) Modes() []string {
	names := make([]string, 0, len(c.modes))
	for name := range c.modes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ActiveName returns the effective active mode name, checking project override first.
// If projectDir has a .lucind/mode file, its content overrides the config active mode.
// It returns an error if an override names an unknown mode or if neither active mode nor override is set.
func (c *Config) ActiveName(projectDir string) (string, bool, error) {
	if projectDir != "" {
		overridePath := filepath.Join(projectDir, ".lucind", "mode")
		data, err := os.ReadFile(overridePath)
		if err == nil {
			override := strings.TrimSpace(string(data))
			if override == "" {
				return "", false, fmt.Errorf("project override in %s is empty", overridePath)
			}
			if _, ok := c.modes[override]; !ok {
				return "", false, fmt.Errorf("project override mode %q in %s not defined", override, overridePath)
			}
			return override, true, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", false, fmt.Errorf("read project override %s: %w", overridePath, err)
		}
	}

	if c.Active != "" {
		if _, ok := c.modes[c.Active]; !ok {
			return "", false, fmt.Errorf("active mode %q not defined", c.Active)
		}
		return c.Active, false, nil
	}

	return "", false, errors.New("no active mode set")
}

// Resolve resolves the route for the given role in the effective active mode.
// It returns an error if the role is unknown or not defined in the active mode.
func (c *Config) Resolve(projectDir, role string) (Route, error) {
	modeName, _, err := c.ActiveName(projectDir)
	if err != nil {
		return Route{}, err
	}
	if !isValidRole(role) {
		return Route{}, fmt.Errorf("mode %q: unknown role %q", modeName, role)
	}
	modeMap, ok := c.modes[modeName]
	if !ok {
		return Route{}, fmt.Errorf("mode %q not defined", modeName)
	}
	roleCfg, ok := modeMap[role]
	if !ok {
		return Route{}, fmt.Errorf("mode %q does not define role %q", modeName, role)
	}
	return Route{
		Mode:     modeName,
		Role:     role,
		Executor: roleCfg.Executor,
		Model:    roleCfg.Model,
	}, nil
}

// SetActive validates that the mode name exists and rewrites the config file with the new active mode.
// The file is written atomically with 2-space indentation and 0o600 permissions.
func SetActive(path, name string) error {
	cfg, err := Load(path)
	if err != nil {
		return err
	}

	if _, ok := cfg.modes[name]; !ok {
		return fmt.Errorf("mode %q does not exist", name)
	}

	cfg.Active = name

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, "modes.*.json")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpName := tmpFile.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if err := tmpFile.Chmod(0o600); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp file to %s: %w", path, err)
	}
	_ = os.Chmod(path, 0o600)

	return nil
}
