package dag

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var (
	ErrMissingChange    = errors.New("dag: missing change name")
	ErrMissingPackets   = errors.New("dag: missing or empty packets list")
	ErrMissingID        = errors.New("dag: packet missing id")
	ErrMissingExecutor  = errors.New("dag: packet missing executor")
	ErrMissingRoutedBy  = errors.New("dag: packet missing routed_by")
	ErrMissingBodyPath  = errors.New("dag: packet missing body_path")
)

// Node is one packet declaration inside apply-dag.yaml.
type Node struct {
	ID                string   `yaml:"id"`
	Executor          string   `yaml:"executor"`
	RoutedBy          string   `yaml:"routed_by"`
	Model             string   `yaml:"model,omitempty"`
	Feature           string   `yaml:"feature,omitempty"`
	ParentRef         string   `yaml:"parent_ref,omitempty"`
	BaseSHA           string   `yaml:"base_sha,omitempty"`
	ExpectedParentSHA string   `yaml:"expected_parent_sha,omitempty"`
	LegacyMain        bool     `yaml:"legacy_main,omitempty"`
	AllowedPaths      []string `yaml:"allowed_paths"`
	DependsOn         []string `yaml:"depends_on"`
	BodyPath          string   `yaml:"body_path"`
}

// DAG represents the top-level apply-dag.yaml sidecar structure.
type DAG struct {
	Change  string `yaml:"change"`
	Packets []Node `yaml:"packets"`
}

// Parse reads and unmarshals an apply-dag.yaml sidecar file, checking required fields
// and ensuring that all referenced body_path files exist on disk relative to the sidecar file.
func Parse(path string) (DAG, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DAG{}, fmt.Errorf("dag: failed to read sidecar %s: %w", path, err)
	}

	var d DAG
	if err := yaml.Unmarshal(data, &d); err != nil {
		return DAG{}, fmt.Errorf("dag: invalid YAML in %s: %w", path, err)
	}

	if d.Change == "" {
		return DAG{}, ErrMissingChange
	}
	if len(d.Packets) == 0 {
		return DAG{}, ErrMissingPackets
	}

	baseDir := filepath.Dir(path)
	for i, p := range d.Packets {
		if p.ID == "" {
			return DAG{}, fmt.Errorf("%w at index %d", ErrMissingID, i)
		}
		if p.Executor == "" {
			return DAG{}, fmt.Errorf("%w for packet %q", ErrMissingExecutor, p.ID)
		}
		if p.RoutedBy == "" {
			return DAG{}, fmt.Errorf("%w for packet %q", ErrMissingRoutedBy, p.ID)
		}
		if p.BodyPath == "" {
			return DAG{}, fmt.Errorf("%w for packet %q", ErrMissingBodyPath, p.ID)
		}

		fullBodyPath := filepath.Join(baseDir, p.BodyPath)
		fi, err := os.Stat(fullBodyPath)
		if err != nil {
			return DAG{}, fmt.Errorf("dag: packet %q body_path %q does not exist: %w", p.ID, p.BodyPath, err)
		}
		if fi.IsDir() {
			return DAG{}, fmt.Errorf("dag: packet %q body_path %q is a directory", p.ID, p.BodyPath)
		}
	}

	return d, nil
}
