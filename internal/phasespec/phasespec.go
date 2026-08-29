package phasespec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SchemaNameConstant is the expected schemaName in gentle-ai sdd-status JSON.
const SchemaNameConstant = "gentle-ai.sdd-status"

var (
	// ErrMalformedStatus is returned when sdd-status JSON is invalid, malformed, or missing required fields.
	ErrMalformedStatus = errors.New("phasespec: malformed status JSON")
	// ErrCLIExecution is returned when the external CLI status command fails.
	ErrCLIExecution = errors.New("phasespec: CLI command execution failed")
	// ErrPrematureSynthesis is returned when synthesis is attempted before all required lenses are accepted and merged.
	ErrPrematureSynthesis = errors.New("phasespec: cannot start synthesis before all required lenses are accepted and merged")
	// ErrInvalidPhase is returned when an unrecognized or invalid phase is specified.
	ErrInvalidPhase = errors.New("phasespec: invalid or unrecognized phase")
	// ErrInvalidChange is returned when a change name is empty or contains forbidden path elements.
	ErrInvalidChange = errors.New("phasespec: invalid change name")
	// ErrForbiddenPath is returned when an artifact path attempts to escape the allowed openspec directory.
	ErrForbiddenPath = errors.New("phasespec: path escapes permitted openspec directory")
)

// PlanningHome represents the planning home object in sdd-status.
type PlanningHome struct {
	Mode string `json:"mode"`
	Path string `json:"path"`
}

// TaskProgress represents task progress counters.
type TaskProgress struct {
	Total       int  `json:"total"`
	Completed   int  `json:"completed"`
	Pending     int  `json:"pending"`
	AllComplete bool `json:"allComplete"`
}

// ActionContext represents action context and allowed edit roots.
type ActionContext struct {
	Mode             string   `json:"mode"`
	WorkspaceRoot    string   `json:"workspaceRoot"`
	AllowedEditRoots []string `json:"allowedEditRoots"`
}

// Relationships represents change relationships.
type Relationships struct {
	DependsOn               []string `json:"dependsOn"`
	Supersedes              []string `json:"supersedes"`
	Amends                  []string `json:"amends"`
	ConflictsWith           []string `json:"conflictsWith"`
	SameDomainActiveChanges []string `json:"sameDomainActiveChanges"`
}

// RemediationState represents active remediation status.
type RemediationState struct {
	Required               bool   `json:"required"`
	Complete               bool   `json:"complete"`
	FailedEvidenceRevision string `json:"failedEvidenceRevision"`
	LineageId              string `json:"lineageId"`
	Generation             int    `json:"generation"`
	FixBatch               int    `json:"fixBatch"`
	Reason                 string `json:"reason"`
}

// Status represents the parsed gentle-ai sdd-status JSON payload.
type Status struct {
	SchemaName       string              `json:"schemaName"`
	SchemaVersion    int                 `json:"schemaVersion"`
	ChangeName       string              `json:"changeName"`
	ArtifactStore    string              `json:"artifactStore,omitempty"`
	PlanningHome     PlanningHome        `json:"planningHome,omitempty"`
	ChangeRoot       string              `json:"changeRoot,omitempty"`
	ArtifactPaths    map[string][]string `json:"artifactPaths,omitempty"`
	ContextFiles     map[string][]string `json:"contextFiles,omitempty"`
	Artifacts        map[string]string   `json:"artifacts,omitempty"`
	TaskProgress     TaskProgress        `json:"taskProgress,omitempty"`
	Dependencies     map[string]string   `json:"dependencies,omitempty"`
	ApplyState       string              `json:"applyState,omitempty"`
	ActionContext    ActionContext       `json:"actionContext,omitempty"`
	Relationships    Relationships       `json:"relationships,omitempty"`
	RemediationState RemediationState    `json:"remediationState,omitempty"`
	NextRecommended  string              `json:"nextRecommended,omitempty"`
	BlockedReasons   []string            `json:"blockedReasons,omitempty"`
}

// ParseStatus decodes and validates sdd-status JSON.
// It fails closed on malformed syntax, trailing tokens, wrong schemaName, missing changeName, or non-positive schemaVersion.
func ParseStatus(data []byte) (*Status, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("%w: empty input", ErrMalformedStatus)
	}

	dec := json.NewDecoder(bytes.NewReader(trimmed))
	var st Status
	if err := dec.Decode(&st); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedStatus, err)
	}

	if dec.More() {
		return nil, fmt.Errorf("%w: multiple JSON values detected", ErrMalformedStatus)
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("%w: trailing data after JSON", ErrMalformedStatus)
	}

	if st.SchemaName != SchemaNameConstant {
		return nil, fmt.Errorf("%w: invalid schemaName %q (expected %q)", ErrMalformedStatus, st.SchemaName, SchemaNameConstant)
	}
	if st.SchemaVersion <= 0 {
		return nil, fmt.Errorf("%w: invalid schemaVersion %d", ErrMalformedStatus, st.SchemaVersion)
	}
	if strings.TrimSpace(st.ChangeName) == "" {
		return nil, fmt.Errorf("%w: missing changeName", ErrMalformedStatus)
	}

	return &st, nil
}

// LensState represents the acceptance and merge state of a planning lens.
type LensState struct {
	ID       string `json:"id"`
	Accepted bool   `json:"accepted"`
	Merged   bool   `json:"merged"`
}

// DefaultRequiredLenses defines the standard three lenses for SDD planning phases.
var DefaultRequiredLenses = []string{"lens-a", "lens-b", "lens-c"}

// CheckSynthesisEligibility checks whether all required lenses are accepted and merged.
func CheckSynthesisEligibility(phase string, requiredLenses []string, lensStates map[string]LensState) error {
	if len(requiredLenses) == 0 {
		requiredLenses = DefaultRequiredLenses
	}
	for _, req := range requiredLenses {
		state, exists := lensStates[req]
		if !exists {
			return fmt.Errorf("%w: lens %q is missing", ErrPrematureSynthesis, req)
		}
		if !state.Accepted {
			return fmt.Errorf("%w: lens %q is not accepted", ErrPrematureSynthesis, req)
		}
		if !state.Merged {
			return fmt.Errorf("%w: lens %q is not merged", ErrPrematureSynthesis, req)
		}
	}
	return nil
}

// CanonicalArtifactFilename maps a phase token to its canonical artifact filename.
func CanonicalArtifactFilename(phase string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "explore":
		return "explore.md", nil
	case "propose", "proposal":
		return "proposal.md", nil
	case "spec", "specs":
		return "spec.md", nil
	case "design":
		return "design.md", nil
	case "tasks":
		return "tasks.md", nil
	case "apply":
		return "apply-progress.md", nil
	case "verify":
		return "verify-report.md", nil
	case "archive":
		return "archive-report.md", nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidPhase, phase)
	}
}

// CanonicalArtifactPath returns the repository-relative path openspec/changes/<change>/<canonical-filename>.
func CanonicalArtifactPath(change, phase string) (string, error) {
	change = strings.TrimSpace(change)
	if change == "" || strings.Contains(change, "..") || strings.ContainsAny(change, "/\\") {
		return "", fmt.Errorf("%w: %q", ErrInvalidChange, change)
	}
	filename, err := CanonicalArtifactFilename(phase)
	if err != nil {
		return "", err
	}
	return filepath.Join("openspec", "changes", change, filename), nil
}

// WriteCanonicalArtifact writes the canonical artifact to openspec/changes/<change>/<canonical-filename> in workspaceRoot.
// It fails closed if the path escapes the allowed openspec/changes/<change> directory.
func WriteCanonicalArtifact(workspaceRoot, change, phase string, content []byte) (string, error) {
	relPath, err := CanonicalArtifactPath(change, phase)
	if err != nil {
		return "", err
	}

	cleanRel := filepath.Clean(relPath)
	expectedPrefix := filepath.Join("openspec", "changes", change)
	if !strings.HasPrefix(cleanRel, expectedPrefix) {
		return "", fmt.Errorf("%w: %q is not within %q", ErrForbiddenPath, cleanRel, expectedPrefix)
	}

	fullPath := filepath.Join(workspaceRoot, cleanRel)
	cleanRoot := filepath.Clean(workspaceRoot)
	cleanFull := filepath.Clean(fullPath)

	relToRoot, err := filepath.Rel(cleanRoot, cleanFull)
	if err != nil || strings.HasPrefix(relToRoot, "..") || relToRoot == "." {
		return "", fmt.Errorf("%w: %q escapes workspace root %q", ErrForbiddenPath, cleanFull, cleanRoot)
	}

	dir := filepath.Dir(cleanFull)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("phasespec: failed to create directories: %w", err)
	}

	if err := os.WriteFile(cleanFull, content, 0644); err != nil {
		return "", fmt.Errorf("phasespec: failed to write artifact file: %w", err)
	}

	return cleanRel, nil
}

// StatusQuerier queries gentle-ai sdd-status for a change.
type StatusQuerier interface {
	QueryStatus(ctx context.Context, changeName string) ([]byte, error)
}

// CLIStatusQuerier queries sdd-status using the gentle-ai CLI.
type CLIStatusQuerier struct {
	Executable string
	WorkDir    string
}

// QueryStatus executes the sdd-status command and returns the JSON output.
func (q *CLIStatusQuerier) QueryStatus(ctx context.Context, changeName string) ([]byte, error) {
	exe := q.Executable
	if exe == "" {
		exe = "gentle-ai"
	}
	args := []string{"sdd-status", "--json"}
	if changeName != "" {
		args = []string{"sdd-status", changeName, "--json"}
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	if q.WorkDir != "" {
		cmd.Dir = q.WorkDir
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCLIExecution, err)
	}
	return out, nil
}

// Adapter coordinates SDD status inspection, lens gating, and canonical artifact generation.
type Adapter struct {
	Querier       StatusQuerier
	WorkspaceRoot string
}

// NewAdapter creates a new phasespec Adapter.
func NewAdapter(querier StatusQuerier, workspaceRoot string) *Adapter {
	return &Adapter{
		Querier:       querier,
		WorkspaceRoot: workspaceRoot,
	}
}

// SynthesizeRequest contains parameters for phase synthesis.
type SynthesizeRequest struct {
	ChangeName     string               `json:"changeName"`
	Phase          string               `json:"phase"`
	RequiredLenses []string             `json:"requiredLenses,omitempty"`
	LensStates     map[string]LensState `json:"lensStates,omitempty"`
	Content        []byte               `json:"content"`
	Force          bool                 `json:"force,omitempty"`
}

// SynthesizeResult contains the result of a synthesis request.
type SynthesizeResult struct {
	ChangeName   string  `json:"changeName"`
	Phase        string  `json:"phase"`
	ArtifactPath string  `json:"artifactPath"`
	Written      bool    `json:"written"`
	Status       *Status `json:"status,omitempty"`
}

// Synthesize executes the synthesis sequencing checks and writes the canonical phase artifact when permitted.
func (a *Adapter) Synthesize(ctx context.Context, req SynthesizeRequest) (*SynthesizeResult, error) {
	if strings.TrimSpace(req.ChangeName) == "" || strings.Contains(req.ChangeName, "..") || strings.ContainsAny(req.ChangeName, "/\\") {
		return nil, fmt.Errorf("%w: %q", ErrInvalidChange, req.ChangeName)
	}
	if _, err := CanonicalArtifactFilename(req.Phase); err != nil {
		return nil, err
	}

	var status *Status
	if a.Querier != nil {
		raw, err := a.Querier.QueryStatus(ctx, req.ChangeName)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCLIExecution, err)
		}
		st, err := ParseStatus(raw)
		if err != nil {
			return nil, err
		}
		status = st

		// Check if phase is already complete
		if !req.Force && isPhaseComplete(st, req.Phase) {
			relPath, _ := CanonicalArtifactPath(req.ChangeName, req.Phase)
			return &SynthesizeResult{
				ChangeName:   req.ChangeName,
				Phase:        req.Phase,
				ArtifactPath: relPath,
				Written:      false,
				Status:       st,
			}, nil
		}
	}

	// Verify lenses eligibility
	if err := CheckSynthesisEligibility(req.Phase, req.RequiredLenses, req.LensStates); err != nil {
		return nil, err
	}

	// Write canonical artifact
	relPath, err := WriteCanonicalArtifact(a.WorkspaceRoot, req.ChangeName, req.Phase, req.Content)
	if err != nil {
		return nil, err
	}

	return &SynthesizeResult{
		ChangeName:   req.ChangeName,
		Phase:        req.Phase,
		ArtifactPath: relPath,
		Written:      true,
		Status:       status,
	}, nil
}

func isPhaseComplete(st *Status, phase string) bool {
	if st == nil {
		return false
	}
	normPhase := strings.ToLower(strings.TrimSpace(phase))
	switch normPhase {
	case "propose", "proposal":
		if st.Artifacts["proposal"] == "done" || st.Dependencies["proposal"] == "all_done" {
			return true
		}
	case "spec", "specs":
		if st.Artifacts["specs"] == "done" || st.Dependencies["specs"] == "all_done" {
			return true
		}
	case "design":
		if st.Artifacts["design"] == "done" || st.Dependencies["design"] == "all_done" {
			return true
		}
	case "tasks":
		if st.Artifacts["tasks"] == "done" || st.Dependencies["tasks"] == "all_done" {
			return true
		}
	default:
		if st.Artifacts[normPhase] == "done" {
			return true
		}
	}
	return false
}
