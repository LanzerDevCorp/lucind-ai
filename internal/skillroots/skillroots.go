package skillroots

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultConfigRelPath is the default relative path to the machine-local roots configuration.
const DefaultConfigRelPath = ".lucind/skill-roots.yaml"

// SkillFileName is the canonical entry point file name within a skill directory.
const SkillFileName = "SKILL.md"

var (
	// ErrMissingConfig is returned when the skill-roots config file does not exist.
	ErrMissingConfig = errors.New("skillroots: config file does not exist")
	// ErrNoRootsConfigured is returned when no search roots are configured in skill-roots.yaml.
	ErrNoRootsConfigured = errors.New("skillroots: no search roots configured")
	// ErrInvalidConfig is returned when the config file contains malformed YAML.
	ErrInvalidConfig = errors.New("skillroots: invalid YAML in config")
	// ErrSkillNotFound is returned when a skill cannot be located in any configured root.
	ErrSkillNotFound = errors.New("skillroots: skill not found")
	// ErrEmptySkillName is returned when an empty skill name is provided.
	ErrEmptySkillName = errors.New("skillroots: empty skill name")
)

// Config represents the schema of .lucind/skill-roots.yaml.
type Config struct {
	Roots []string `yaml:"roots"`
}

// SkillNotFoundError provides detailed diagnostic information when a skill is not found.
type SkillNotFoundError struct {
	Skill         string
	Roots         []string
	SearchedPaths []string
}

func (e *SkillNotFoundError) Error() string {
	if len(e.Roots) == 0 {
		return fmt.Sprintf("skillroots: skill %q not found: no search roots configured", e.Skill)
	}
	return fmt.Sprintf("skillroots: skill %q not found in configured roots: %s (searched: %s)",
		e.Skill,
		strings.Join(e.Roots, ", "),
		strings.Join(e.SearchedPaths, ", "),
	)
}

func (e *SkillNotFoundError) Unwrap() error {
	return ErrSkillNotFound
}

// LoadConfig reads and parses a skill-roots configuration YAML file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrMissingConfig, path)
		}
		return nil, fmt.Errorf("skillroots: failed to read config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		// Try unmarshaling as a raw sequence of root strings
		var roots []string
		if errSeq := yaml.Unmarshal(data, &roots); errSeq == nil && len(roots) > 0 {
			return &Config{Roots: roots}, nil
		}
		return nil, fmt.Errorf("%w in %s: %v", ErrInvalidConfig, path, err)
	}

	if len(cfg.Roots) == 0 {
		var roots []string
		if errSeq := yaml.Unmarshal(data, &roots); errSeq == nil && len(roots) > 0 {
			cfg.Roots = roots
		}
	}

	return &cfg, nil
}

// ExpandTilde expands a leading tilde (~) in a path using the user's home directory.
func ExpandTilde(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.Getenv("HOME")
	}
	if home == "" {
		return "", fmt.Errorf("skillroots: cannot expand tilde in %q: user home directory not found", path)
	}
	return ExpandTildeWithHome(path, home), nil
}

// ExpandTildeWithHome expands a leading tilde (~) in a path using the provided home directory.
func ExpandTildeWithHome(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		return filepath.Join(home, path[2:])
	}
	return path
}

// Resolver resolves skill names to SKILL.md filesystem paths across configured roots.
type Resolver struct {
	roots   []string
	homeDir string
}

// NewResolver creates a Resolver for the given search roots.
func NewResolver(roots []string) *Resolver {
	return &Resolver{
		roots: append([]string(nil), roots...),
	}
}

// NewResolverWithHome creates a Resolver with an explicit home directory override for testing.
func NewResolverWithHome(roots []string, homeDir string) *Resolver {
	return &Resolver{
		roots:   append([]string(nil), roots...),
		homeDir: homeDir,
	}
}

// Roots returns a copy of the configured search roots.
func (r *Resolver) Roots() []string {
	return append([]string(nil), r.roots...)
}

func (r *Resolver) expandRoot(root string) (string, error) {
	if r.homeDir != "" {
		return ExpandTildeWithHome(root, r.homeDir), nil
	}
	return ExpandTilde(root)
}

// Resolve locates the canonical SKILL.md for skill across configured roots in order.
func (r *Resolver) Resolve(skill string) (string, error) {
	if strings.TrimSpace(skill) == "" {
		return "", ErrEmptySkillName
	}
	if len(r.roots) == 0 {
		return "", &SkillNotFoundError{
			Skill: skill,
			Roots: nil,
		}
	}

	searchedPaths := make([]string, 0, len(r.roots))
	for _, root := range r.roots {
		expandedRoot, err := r.expandRoot(root)
		if err != nil {
			return "", err
		}
		candidate := filepath.Join(expandedRoot, skill, SkillFileName)
		searchedPaths = append(searchedPaths, candidate)

		fi, err := os.Stat(candidate)
		if err == nil && !fi.IsDir() {
			return candidate, nil
		}
	}

	return "", &SkillNotFoundError{
		Skill:         skill,
		Roots:         r.Roots(),
		SearchedPaths: searchedPaths,
	}
}

// ResolveAll resolves every skill in skills, returning a map of skill name to SKILL.md path.
// It fails closed on the first unresolvable skill.
func (r *Resolver) ResolveAll(skills []string) (map[string]string, error) {
	results := make(map[string]string, len(skills))
	for _, skill := range skills {
		path, err := r.Resolve(skill)
		if err != nil {
			return nil, err
		}
		results[skill] = path
	}
	return results, nil
}

// ResolvePaths resolves every skill in skills, returning a slice of SKILL.md paths
// corresponding in order to the input skills. It fails closed on the first unresolvable skill.
func (r *Resolver) ResolvePaths(skills []string) ([]string, error) {
	paths := make([]string, 0, len(skills))
	for _, skill := range skills {
		path, err := r.Resolve(skill)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// Resolve resolves a skill across the provided roots.
func Resolve(roots []string, skill string) (string, error) {
	return NewResolver(roots).Resolve(skill)
}

// ResolveAll resolves multiple skills across the provided roots.
func ResolveAll(roots []string, skills []string) (map[string]string, error) {
	return NewResolver(roots).ResolveAll(skills)
}

// ResolvePaths resolves multiple skills across the provided roots, returning paths in order.
func ResolvePaths(roots []string, skills []string) ([]string, error) {
	return NewResolver(roots).ResolvePaths(skills)
}

// ResolveFromConfig loads roots from configPath and resolves skill.
func ResolveFromConfig(configPath string, skill string) (string, error) {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return "", err
	}
	return Resolve(cfg.Roots, skill)
}

// ResolveAllFromConfig loads roots from configPath and resolves multiple skills.
func ResolveAllFromConfig(configPath string, skills []string) (map[string]string, error) {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}
	return ResolveAll(cfg.Roots, skills)
}

// ResolvePathsFromConfig loads roots from configPath and resolves multiple skills to paths.
func ResolvePathsFromConfig(configPath string, skills []string) ([]string, error) {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}
	return ResolvePaths(cfg.Roots, skills)
}
