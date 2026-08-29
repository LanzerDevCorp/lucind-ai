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

// LensesField represents a map of lens ID to LensState that unmarshals flexibly from a JSON object or array.
type LensesField map[string]LensState

// UnmarshalJSON unmarshals a JSON object or array into a LensesField map.
func (lf *LensesField) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	if trimmed[0] == '{' {
		var m map[string]LensState
		if err := json.Unmarshal(trimmed, &m); err != nil {
			return err
		}
		for k, v := range m {
			if v.ID == "" {
				v.ID = k
				m[k] = v
			}
		}
		*lf = m
		return nil
	}
	if trimmed[0] == '[' {
		var list []LensState
		if err := json.Unmarshal(trimmed, &list); err != nil {
			return err
		}
		m := make(map[string]LensState, len(list))
		for _, item := range list {
			m[item.ID] = item
		}
		*lf = m
		return nil
	}
	return fmt.Errorf("invalid lenses format")
}

// Status represents the parsed gentle-ai sdd-status JSON payload.
type Status struct {
	SchemaName       string                 `json:"schemaName"`
	SchemaVersion    int                    `json:"schemaVersion"`
	ChangeName       string                 `json:"changeName"`
	ArtifactStore    string                 `json:"artifactStore,omitempty"`
	PlanningHome     PlanningHome           `json:"planningHome,omitempty"`
	ChangeRoot       string                 `json:"changeRoot,omitempty"`
	ArtifactPaths    map[string][]string    `json:"artifactPaths,omitempty"`
	ContextFiles     map[string][]string    `json:"contextFiles,omitempty"`
	Artifacts        map[string]string      `json:"artifacts,omitempty"`
	TaskProgress     TaskProgress           `json:"taskProgress,omitempty"`
	Dependencies     map[string]string      `json:"dependencies,omitempty"`
	ApplyState       string                 `json:"applyState,omitempty"`
	ActionContext    ActionContext          `json:"actionContext,omitempty"`
	Relationships    Relationships          `json:"relationships,omitempty"`
	RemediationState RemediationState       `json:"remediationState,omitempty"`
	NextRecommended  string                 `json:"nextRecommended,omitempty"`
	BlockedReasons   []string               `json:"blockedReasons,omitempty"`
	Lenses           LensesField            `json:"lenses,omitempty"`
	LensStates       LensesField            `json:"lensStates,omitempty"`
	PhaseLenses      map[string]LensesField `json:"phaseLenses,omitempty"`
}

// GetLensStates returns the map of lens states for the given phase (or top-level lenses).
func (st *Status) GetLensStates(phase ...string) map[string]LensState {
	if st == nil {
		return nil
	}
	result := make(map[string]LensState)
	if len(st.Lenses) > 0 {
		for k, v := range st.Lenses {
			result[k] = v
		}
	}
	if len(st.LensStates) > 0 {
		for k, v := range st.LensStates {
			result[k] = v
		}
	}
	if len(phase) > 0 && len(st.PhaseLenses) > 0 {
		normPhase := strings.ToLower(strings.TrimSpace(phase[0]))
		if pLenses, ok := st.PhaseLenses[normPhase]; ok {
			for k, v := range pLenses {
				result[k] = v
			}
		}
	}
	return result
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
		return "propose.md", nil
	case "spec", "specs":
		return "spec.md", nil
	case "design":
		return "design.md", nil
	case "tasks":
		return "tasks.md", nil
	case "apply":
		return "apply.md", nil
	case "verify":
		return "verify.md", nil
	case "remediate":
		return "remediate.md", nil
	case "archive":
		return "archive.md", nil
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

// DispatchFunc is a function type for triggering synthesis lane dispatch.
type DispatchFunc func(ctx context.Context, changeName, phase string) error

// Adapter coordinates SDD status inspection, lens gating, and canonical artifact generation.
type Adapter struct {
	Querier       StatusQuerier
	WorkspaceRoot string
	Dispatcher    DispatchFunc
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
	Content        []byte               `json:"content,omitempty"`
	Force          bool                 `json:"force,omitempty"`
}

// SynthesizeResult contains the result of a synthesis request.
type SynthesizeResult struct {
	ChangeName   string  `json:"changeName"`
	Phase        string  `json:"phase"`
	ArtifactPath string  `json:"artifactPath"`
	Written      bool    `json:"written"`
	Dispatched   bool    `json:"dispatched,omitempty"`
	Status       *Status `json:"status,omitempty"`
}

// Synthesize executes the synthesis sequencing checks and writes the canonical phase artifact or dispatches synthesis lane when permitted.
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

		// Check if phase is already complete (status done + artifact exists on disk)
		if !req.Force && isPhaseComplete(a.WorkspaceRoot, req.ChangeName, st, req.Phase) {
			relPath, _ := CanonicalArtifactPath(req.ChangeName, req.Phase)
			return &SynthesizeResult{
				ChangeName:   req.ChangeName,
				Phase:        req.Phase,
				ArtifactPath: relPath,
				Written:      false,
				Dispatched:   false,
				Status:       st,
			}, nil
		}
	}

	lensStates := req.LensStates
	if len(lensStates) == 0 && status != nil {
		lensStates = status.GetLensStates(req.Phase)
	}

	// Verify lenses eligibility
	if err := CheckSynthesisEligibility(req.Phase, req.RequiredLenses, lensStates); err != nil {
		return nil, err
	}

	relPath, err := CanonicalArtifactPath(req.ChangeName, req.Phase)
	if err != nil {
		return nil, err
	}

	// If content is provided, write the canonical artifact directly
	if len(req.Content) > 0 {
		writtenPath, err := WriteCanonicalArtifact(a.WorkspaceRoot, req.ChangeName, req.Phase, req.Content)
		if err != nil {
			return nil, err
		}

		return &SynthesizeResult{
			ChangeName:   req.ChangeName,
			Phase:        req.Phase,
			ArtifactPath: writtenPath,
			Written:      true,
			Dispatched:   false,
			Status:       status,
		}, nil
	}

	// When content is not provided, trigger synthesis lane dispatch if a dispatcher is configured
	dispatched := false
	if a.Dispatcher != nil {
		if err := a.Dispatcher(ctx, req.ChangeName, req.Phase); err != nil {
			return nil, fmt.Errorf("phasespec: dispatch synthesis lane: %w", err)
		}
		dispatched = true
	}

	return &SynthesizeResult{
		ChangeName:   req.ChangeName,
		Phase:        req.Phase,
		ArtifactPath: relPath,
		Written:      false,
		Dispatched:   dispatched,
		Status:       status,
	}, nil
}

func isPhaseComplete(workspaceRoot, change string, st *Status, phase string) bool {
	if st == nil {
		return false
	}
	normPhase := strings.ToLower(strings.TrimSpace(phase))
	statusDone := false
	switch normPhase {
	case "propose", "proposal":
		if st.Artifacts["proposal"] == "done" || st.Artifacts["propose"] == "done" || st.Dependencies["proposal"] == "all_done" || st.Dependencies["propose"] == "all_done" {
			statusDone = true
		}
	case "spec", "specs":
		if st.Artifacts["specs"] == "done" || st.Artifacts["spec"] == "done" || st.Dependencies["specs"] == "all_done" || st.Dependencies["spec"] == "all_done" {
			statusDone = true
		}
	case "design":
		if st.Artifacts["design"] == "done" || st.Dependencies["design"] == "all_done" {
			statusDone = true
		}
	case "tasks":
		if st.Artifacts["tasks"] == "done" || st.Dependencies["tasks"] == "all_done" {
			statusDone = true
		}
	default:
		if st.Artifacts[normPhase] == "done" || st.Dependencies[normPhase] == "all_done" {
			statusDone = true
		}
	}
	if !statusDone {
		return false
	}

	relPath, err := CanonicalArtifactPath(change, phase)
	if err != nil {
		return false
	}
	fullPath := filepath.Join(workspaceRoot, relPath)
	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		return false
	}
	return true
}
