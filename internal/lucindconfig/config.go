package lucindconfig

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConfigFileName is the tracked repository configuration file name.
const ConfigFileName = "lucind.yaml"

// Config represents the tracked repository configuration in lucind.yaml.
type Config struct {
	SkillBudget *int                `yaml:"skill_budget,omitempty"`
	Skills      map[string][]string `yaml:"skills,omitempty"`
}

// StackSkills returns a defensive copy of the configured stack skill names for the given role,
// or nil if no stack skills are configured for that role.
func (c Config) StackSkills(role string) []string {
	if c.Skills == nil {
		return nil
	}
	skills, ok := c.Skills[role]
	if !ok || len(skills) == 0 {
		return nil
	}
	out := make([]string, len(skills))
	copy(out, skills)
	return out
}

// Parse decodes YAML data from an io.Reader into a Config using yaml.NewDecoder
// with KnownFields(true) to reject unrecognized fields. An empty reader or EOF
// yields an empty Config and nil error.
func Parse(r io.Reader) (Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		if errors.Is(err, io.EOF) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("lucindconfig: failed to parse config: %w", err)
	}
	return cfg, nil
}

// ParseBytes decodes YAML data from a byte slice into a Config using KnownFields(true).
func ParseBytes(data []byte) (Config, error) {
	return Parse(bytes.NewReader(data))
}

// LoadFile reads and decodes the YAML config file from the specified path using KnownFields(true).
func LoadFile(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("lucindconfig: failed to open %s: %w", path, err)
	}
	defer f.Close()

	cfg, err := Parse(f)
	if err != nil {
		return Config{}, fmt.Errorf("lucindconfig: invalid config in %s: %w", path, err)
	}
	return cfg, nil
}

// Load loads the repository configuration from lucind.yaml in the specified directory or file path.
// If lucind.yaml does not exist, Load returns an empty Config and nil error (zero-config default).
func Load(pathOrDir string) (Config, error) {
	path := pathOrDir
	if filepath.Base(pathOrDir) != ConfigFileName {
		path = filepath.Join(pathOrDir, ConfigFileName)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, err
	}
	return cfg, nil
}
