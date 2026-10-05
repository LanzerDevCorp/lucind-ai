package lane

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/attest"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
)

// Lane represents a managed execution lane.
type Lane struct {
	Version    int        `json:"version"`
	ID         string     `json:"id"`
	Cwd        string     `json:"cwd"`
	BaseTree   string     `json:"base_tree"`
	Allow      []string   `json:"allow"`
	Checks     []string   `json:"checks,omitempty"`
	Model      string     `json:"model"`
	PaneID     string     `json:"pane_id"`
	Status     Status     `json:"status"`
	Retries    int        `json:"retries"`
	Turn       int        `json:"turn"`
	LastStopAt *time.Time `json:"last_stop_at,omitempty"`
	Continues  int        `json:"continues,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// CheckEvidence holds an attestation or check log for a single check in a completed lane receipt.
type CheckEvidence struct {
	Check       string  `json:"check"`
	Attestation *string `json:"attestation,omitempty"`
	CheckLog    *string `json:"check_log,omitempty"`
}

// Evidence is an alias for CheckEvidence.
type Evidence = CheckEvidence

// Receipt records the final outcome and verification evidence for a lane.
type Receipt struct {
	Version      int             `json:"version"`
	Lane         string          `json:"lane"`
	FinalTree    string          `json:"final_tree"`
	BaseTree     string          `json:"base_tree"`
	ChangedFiles []string        `json:"changed_files"`
	Verdict      string          `json:"verdict"`
	Reasons      []string        `json:"reasons"`
	Evidence     []CheckEvidence `json:"evidence"`
	CreatedAt    time.Time       `json:"created_at"`
}

const (
	VerdictAccepted = "accepted"
	VerdictRejected = "rejected"
)

var laneIDRegex = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}-[0-9a-f]{4}$`)

// LaneDir returns the directory path for a given lane under root.
func LaneDir(root, id string) string {
	return filepath.Join(root, ".lucind", "lanes", id)
}

// LanePath returns the path to lane.json for a given lane under root.
func LanePath(root, id string) string {
	return filepath.Join(LaneDir(root, id), "lane.json")
}

// ReceiptPath returns the path to receipt.json for a given lane under root.
func ReceiptPath(root, id string) string {
	return filepath.Join(LaneDir(root, id), "receipt.json")
}

// ResultFileName returns the result filename for the given turn.
// For turn <= 0 (legacy lanes), it returns "result.json", otherwise "result-<turn>.json".
func ResultFileName(turn int) string {
	if turn <= 0 {
		return "result.json"
	}
	return fmt.Sprintf("result-%d.json", turn)
}

// ResultFilePath returns the result file path for the current turn of the given lane under root.
func ResultFilePath(root string, l Lane) string {
	return filepath.Join(LaneDir(root, l.ID), ResultFileName(l.Turn))
}

// SkillsFileName returns the skills filename for the given turn.
// For turn <= 0 (legacy lanes), it returns "skills.json", otherwise "skills-<turn>.json".
func SkillsFileName(turn int) string {
	if turn <= 0 {
		return "skills.json"
	}
	return fmt.Sprintf("skills-%d.json", turn)
}

// SkillsFilePath returns the skills file path for the current turn of the given lane under root.
func SkillsFilePath(root string, l Lane) string {
	return filepath.Join(LaneDir(root, l.ID), SkillsFileName(l.Turn))
}

// CheckCommand returns the canonical attested command string for a check.
func CheckCommand(check string) string {
	return "sh -c " + check
}

// GenerateID generates a new lane ID in the format YYYYMMDD-HHMMSS-<4 lowercase hex> (UTC).
func GenerateID() (string, error) {
	return GenerateIDAt(time.Now())
}

// GenerateIDAt generates a new lane ID for a given timestamp.
func GenerateIDAt(t time.Time) (string, error) {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate random lane id suffix: %w", err)
	}
	return fmt.Sprintf("%s-%04x", t.UTC().Format("20060102-150405"), b), nil
}

// ValidateID reports whether id matches the valid lane ID format.
func ValidateID(id string) bool {
	if !laneIDRegex.MatchString(id) {
		return false
	}
	datePart := id[:15]
	_, err := time.Parse("20060102-150405", datePart)
	return err == nil
}

// AtomicWriteJSON serializes v as formatted JSON and writes it atomically to destPath
// using a temporary file in the target directory followed by a rename.
func AtomicWriteJSON(destPath string, v any) error {
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	data = append(data, '\n')

	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpName := tmpFile.Name()

	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

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

	if err := os.Rename(tmpName, destPath); err != nil {
		return fmt.Errorf("rename %s to %s: %w", tmpName, destPath, err)
	}

	cleanup = false
	return nil
}

// Create finds the git toplevel for cwd, computes base tree hash using attest.TreeHash,
// generates a lane id, initializes a Lane with Version: 1, Status: "running", timestamps,
// saves to .lucind/lanes/<id>/lane.json, and returns the Lane.
func Create(ctx context.Context, cwd string, allow []string, model string, checks ...string) (Lane, error) {
	if cwd == "" {
		cwd = "."
	}
	root, err := attest.RepoToplevel(ctx, cwd)
	if err != nil {
		return Lane{}, fmt.Errorf("resolve repo toplevel for %s: %w", cwd, err)
	}

	baseTree, err := attest.TreeHash(ctx, root)
	if err != nil {
		return Lane{}, fmt.Errorf("compute base tree hash for %s: %w", root, err)
	}

	id, err := GenerateID()
	if err != nil {
		return Lane{}, fmt.Errorf("generate lane id: %w", err)
	}

	now := time.Now().UTC()
	if allow == nil {
		allow = []string{}
	}

	var laneChecks []string
	if len(checks) > 0 {
		laneChecks = checks
	}

	lane := Lane{
		Version:   1,
		ID:        id,
		Cwd:       cwd,
		BaseTree:  baseTree,
		Allow:     allow,
		Checks:    laneChecks,
		Model:     model,
		PaneID:    "",
		Status:    StatusRunning,
		Retries:   0,
		Turn:      1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := Save(root, lane); err != nil {
		return Lane{}, fmt.Errorf("save lane: %w", err)
	}

	return lane, nil
}

// Load loads a Lane from root/.lucind/lanes/<id>/lane.json.
func Load(root, id string) (Lane, error) {
	path := LanePath(root, id)
	data, err := os.ReadFile(path)
	if err != nil {
		return Lane{}, fmt.Errorf("read lane %s: %w", id, err)
	}
	var lane Lane
	if err := json.Unmarshal(data, &lane); err != nil {
		return Lane{}, fmt.Errorf("unmarshal lane %s: %w", id, err)
	}
	return lane, nil
}

// Save atomically writes a Lane to root/.lucind/lanes/<id>/lane.json.
func Save(root string, lane Lane) error {
	if lane.ID == "" {
		return errors.New("lane id cannot be empty")
	}
	if lane.Version == 0 {
		lane.Version = 1
	}
	if lane.Allow == nil {
		lane.Allow = []string{}
	}
	now := time.Now().UTC()
	if lane.CreatedAt.IsZero() {
		lane.CreatedAt = now
	}
	if lane.UpdatedAt.IsZero() {
		lane.UpdatedAt = now
	}
	path := LanePath(root, lane.ID)
	return AtomicWriteJSON(path, lane)
}

// Save atomically writes the receiver Lane to root.
func (l *Lane) Save(root string) error {
	return Save(root, *l)
}

// WriteReceipt atomically writes a Receipt to root/.lucind/lanes/<id>/receipt.json.
func WriteReceipt(root, id string, receipt Receipt) error {
	if id == "" {
		return errors.New("lane id cannot be empty")
	}
	if receipt.Version == 0 {
		receipt.Version = 1
	}
	if receipt.Lane == "" {
		receipt.Lane = id
	}
	if receipt.ChangedFiles == nil {
		receipt.ChangedFiles = []string{}
	}
	if receipt.Reasons == nil {
		receipt.Reasons = []string{}
	}
	if receipt.Evidence == nil {
		receipt.Evidence = []CheckEvidence{}
	}
	if receipt.CreatedAt.IsZero() {
		receipt.CreatedAt = time.Now().UTC()
	}
	path := ReceiptPath(root, id)
	return AtomicWriteJSON(path, receipt)
}

// LoadReceipt loads a Receipt from root/.lucind/lanes/<id>/receipt.json.
func LoadReceipt(root, id string) (Receipt, error) {
	path := ReceiptPath(root, id)
	data, err := os.ReadFile(path)
	if err != nil {
		return Receipt{}, fmt.Errorf("read receipt %s: %w", id, err)
	}
	var receipt Receipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return Receipt{}, fmt.Errorf("unmarshal receipt %s: %w", id, err)
	}
	return receipt, nil
}

// MarkStopped marks a stopped lane as done or failed based on current turn result validation.
// It loads the lane from root, validates the current turn's result file using result.Read,
// updates lane status and UpdatedAt timestamp, saves the lane, and returns the final status.
func MarkStopped(root, id string) (Status, error) {
	lane, err := Load(root, id)
	if err != nil {
		return Status(""), err
	}

	laneDir := LaneDir(root, id)
	env, err := result.Read(os.DirFS(laneDir), ResultFileName(lane.Turn))

	finalStatus := StatusDone
	if err != nil || env.Status != "done" {
		finalStatus = StatusFailed
	}

	lane.Status = finalStatus
	lane.UpdatedAt = time.Now().UTC()

	if err := Save(root, lane); err != nil {
		return Status(""), fmt.Errorf("save lane: %w", err)
	}

	return finalStatus, nil
}
