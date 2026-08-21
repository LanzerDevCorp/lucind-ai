// Package overlap classifies two features' parent ranges as informational, warning,
// or reconciliation-required using deterministic Git-diff evidence.
package overlap

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrNoMergeBase        = errors.New("overlap: no merge base found")
	ErrMultipleMergeBases = errors.New("overlap: multiple merge bases found")
)

// Class represents the overlap classification outcome.
type Class string

const (
	ClassRequired      Class = "required"
	ClassWarning       Class = "warning"
	ClassInformational Class = "informational"
)

// Valid reports whether c is a valid classification class.
func (c Class) Valid() bool {
	switch c {
	case ClassRequired, ClassWarning, ClassInformational:
		return true
	default:
		return false
	}
}

// ChangeLabel represents a category of path modification.
type ChangeLabel string

const (
	LabelRenameDelete     ChangeLabel = "rename/delete"
	LabelBinary           ChangeLabel = "binary"
	LabelModeOnly         ChangeLabel = "mode-only"
	LabelGenerated        ChangeLabel = "generated"
	LabelSymlinkSubmodule ChangeLabel = "symlink/submodule"
	LabelExecutable       ChangeLabel = "executable"
)

// PathChange represents a normalized file change with metadata and categorization labels.
type PathChange struct {
	Path         string        `json:"path"`
	OldPath      string        `json:"old_path,omitempty"`
	Status       string        `json:"status"`
	OldMode      string        `json:"old_mode,omitempty"`
	NewMode      string        `json:"new_mode,omitempty"`
	AddedLines   int           `json:"added_lines"`
	DeletedLines int           `json:"deleted_lines"`
	Binary       bool          `json:"binary"`
	Labels       []ChangeLabel `json:"labels"`
}

// Metrics holds line and path ratio metrics between two features' changes.
type Metrics struct {
	SharedPaths    []string `json:"shared_paths"`
	PathsA         []string `json:"paths_a"`
	PathsB         []string `json:"paths_b"`
	TotalLinesA    int      `json:"total_lines_a"`
	TotalLinesB    int      `json:"total_lines_b"`
	SharedLinesA   int      `json:"shared_lines_a"`
	SharedLinesB   int      `json:"shared_lines_b"`
	HotspotWeightA float64  `json:"hotspot_weight_a"`
	HotspotWeightB float64  `json:"hotspot_weight_b"`
	HotspotWeight  float64  `json:"hotspot_weight"`
	PathRatioA     float64  `json:"path_ratio_a"`
	PathRatioB     float64  `json:"path_ratio_b"`
}

// Thresholds holds configurable numeric thresholds for overlap classification.
type Thresholds struct {
	HotspotRequired float64 `json:"hotspot_required"`
	HotspotWarning  float64 `json:"hotspot_warning"`
	NearbyHunkLines int     `json:"nearby_hunk_lines"`
}

// DefaultThresholds returns the default classification thresholds per design.md.
func DefaultThresholds() Thresholds {
	return Thresholds{
		HotspotRequired: 0.50,
		HotspotWarning:  0.20,
		NearbyHunkLines: 3,
	}
}

// Hunk represents a line range affected in base and new files.
type Hunk struct {
	OldStart int `json:"old_start"`
	OldLines int `json:"old_lines"`
	NewStart int `json:"new_start"`
	NewLines int `json:"new_lines"`
}

// HunkAnalysis records hunk comparison results for a single shared file.
type HunkAnalysis struct {
	Path         string `json:"path"`
	Intersecting bool   `json:"intersecting"`
	Nearby       bool   `json:"nearby"`
	MinDistance  int    `json:"min_distance"`
	HunksA       []Hunk `json:"hunks_a"`
	HunksB       []Hunk `json:"hunks_b"`
}

// Signals records all deterministic signals computed from Git diffs.
type Signals struct {
	PredictedConflict     bool           `json:"predicted_conflict"`
	ConflictPaths         []string       `json:"conflict_paths,omitempty"`
	RenameDeleteCollision bool           `json:"rename_delete_collision"`
	CollisionPaths        []string       `json:"collision_paths,omitempty"`
	SharedBinary          bool           `json:"shared_binary"`
	BinaryPaths           []string       `json:"binary_paths,omitempty"`
	IntersectingHunks     bool           `json:"intersecting_hunks"`
	NearbyHunks           bool           `json:"nearby_hunks"`
	MinHunkDistance       int            `json:"min_hunk_distance"`
	SharedDisjointPaths   bool           `json:"shared_disjoint_paths"`
	HotspotWeightA        float64        `json:"hotspot_weight_a"`
	HotspotWeightB        float64        `json:"hotspot_weight_b"`
	HotspotWeight         float64        `json:"hotspot_weight"`
	PathRatioA            float64        `json:"path_ratio_a"`
	PathRatioB            float64        `json:"path_ratio_b"`
	SharedPaths           []string       `json:"shared_paths"`
	PathsA                []string       `json:"paths_a"`
	PathsB                []string       `json:"paths_b"`
	TotalLinesA           int            `json:"total_lines_a"`
	TotalLinesB           int            `json:"total_lines_b"`
	SharedLinesA          int            `json:"shared_lines_a"`
	SharedLinesB          int            `json:"shared_lines_b"`
	HunkAnalyses          []HunkAnalysis `json:"hunk_analyses,omitempty"`
}

// StructuralEvidence represents best-effort supplementary structural signals.
type StructuralEvidence struct {
	Available  bool     `json:"available"`
	Status     string   `json:"status"` // "available", "unavailable", "stale"
	Omitted    bool     `json:"omitted"`
	Disclosure string   `json:"disclosure"`
	Symbols    []string `json:"symbols,omitempty"`
}

// StructuralProvider provides structural code evidence.
type StructuralProvider interface {
	GetStructuralEvidence(ctx context.Context, repoDir, baseSHA, shaA, shaB string) (StructuralEvidence, error)
}

// StubStructuralProvider is the default provider that reports structural evidence unavailable.
type StubStructuralProvider struct{}

// GetStructuralEvidence reports structural evidence as unavailable and discloses the omission.
func (s StubStructuralProvider) GetStructuralEvidence(ctx context.Context, repoDir, baseSHA, shaA, shaB string) (StructuralEvidence, error) {
	return StructuralEvidence{
		Available:  false,
		Status:     "unavailable",
		Omitted:    true,
		Disclosure: "structural evidence (CodeGraph) is unavailable; omitted from classification",
	}, nil
}

// Evidence is the complete, self-contained record of overlap evaluation.
type Evidence struct {
	Version     string             `json:"version"`
	Hash        string             `json:"hash"`
	BaseSHA     string             `json:"base_sha"`
	FeatureASHA string             `json:"feature_a_sha"`
	FeatureBSHA string             `json:"feature_b_sha"`
	Class       Class              `json:"class"`
	Rationale   []string           `json:"rationale"`
	Signals     Signals            `json:"signals"`
	ChangesA    []PathChange       `json:"changes_a"`
	ChangesB    []PathChange       `json:"changes_b"`
	Thresholds  Thresholds         `json:"thresholds"`
	Structural  StructuralEvidence `json:"structural"`
	CreatedAt   time.Time          `json:"created_at"`
}

// JSON serializes the evidence to a formatted JSON string.
func (e *Evidence) JSON() (string, error) {
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return "", fmt.Errorf("overlap: marshal evidence json: %w", err)
	}
	return string(b), nil
}

// ComputeHash computes the SHA-256 digest of the canonical evidence JSON payload (excluding Hash).
func (e *Evidence) ComputeHash() (string, error) {
	type payload Evidence
	clone := *e
	clone.Hash = ""
	b, err := json.Marshal(payload(clone))
	if err != nil {
		return "", fmt.Errorf("overlap: marshal payload for hash: %w", err)
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

// RawCapture holds the raw outputs captured from Git plumbing commands.
type RawCapture struct {
	BaseSHA           string
	FeatureASHA       string
	FeatureBSHA       string
	NameStatusA       string
	NameStatusB       string
	NumstatA          string
	NumstatB          string
	DiffU0A           string
	DiffU0B           string
	RawDiffA          string
	RawDiffB          string
	MergeTreeOut      string
	MergeTreeConflict bool
}

// FindUniqueMergeBase finds the single common merge base between two commit SHAs.
// Returns ErrNoMergeBase if no merge base exists, or ErrMultipleMergeBases if more than one exists.
func FindUniqueMergeBase(ctx context.Context, repoDir, shaA, shaB string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "merge-base", "--all", shaA, shaB)
	cmd.Dir = repoDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if strings.TrimSpace(stderr.String()) != "" {
			return "", fmt.Errorf("overlap: git merge-base failed: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return "", ErrNoMergeBase
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	var bases []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			bases = append(bases, trimmed)
		}
	}

	if len(bases) == 0 {
		return "", ErrNoMergeBase
	}
	if len(bases) > 1 {
		return "", fmt.Errorf("%w: found %d merge bases (%s)", ErrMultipleMergeBases, len(bases), strings.Join(bases, ", "))
	}

	return bases[0], nil
}

// CaptureRaw executes git diff plumbing commands between baseSHA and the two feature SHAs.
func CaptureRaw(ctx context.Context, repoDir, baseSHA, shaA, shaB string) (*RawCapture, error) {
	runGit := func(args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = repoDir
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("overlap: git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
		}
		return stdout.String(), nil
	}

	nameStatusA, err := runGit("diff", "--find-renames", "--name-status", "-z", baseSHA, shaA)
	if err != nil {
		return nil, fmt.Errorf("capture name-status A: %w", err)
	}

	nameStatusB, err := runGit("diff", "--find-renames", "--name-status", "-z", baseSHA, shaB)
	if err != nil {
		return nil, fmt.Errorf("capture name-status B: %w", err)
	}

	numstatA, err := runGit("diff", "--numstat", "-z", baseSHA, shaA)
	if err != nil {
		return nil, fmt.Errorf("capture numstat A: %w", err)
	}

	numstatB, err := runGit("diff", "--numstat", "-z", baseSHA, shaB)
	if err != nil {
		return nil, fmt.Errorf("capture numstat B: %w", err)
	}

	diffU0A, err := runGit("diff", "-U0", "--find-renames", baseSHA, shaA)
	if err != nil {
		return nil, fmt.Errorf("capture diff -U0 A: %w", err)
	}

	diffU0B, err := runGit("diff", "-U0", "--find-renames", baseSHA, shaB)
	if err != nil {
		return nil, fmt.Errorf("capture diff -U0 B: %w", err)
	}

	rawDiffA, err := runGit("diff", "--raw", "--find-renames", "-z", baseSHA, shaA)
	if err != nil {
		return nil, fmt.Errorf("capture raw diff A: %w", err)
	}

	rawDiffB, err := runGit("diff", "--raw", "--find-renames", "-z", baseSHA, shaB)
	if err != nil {
		return nil, fmt.Errorf("capture raw diff B: %w", err)
	}

	// git merge-tree --write-tree --merge-base=<baseSHA> <shaA> <shaB>
	mtCmd := exec.CommandContext(ctx, "git", "merge-tree", "--write-tree", "--merge-base="+baseSHA, shaA, shaB)
	mtCmd.Dir = repoDir
	var mtOut bytes.Buffer
	mtCmd.Stdout = &mtOut
	mtCmd.Stderr = &mtOut

	mergeTreeConflict := false
	if mtErr := mtCmd.Run(); mtErr != nil {
		var exitErr *exec.ExitError
		if errors.As(mtErr, &exitErr) && exitErr.ExitCode() == 1 {
			mergeTreeConflict = true
		} else {
			return nil, fmt.Errorf("overlap: git merge-tree failed: %w: %s", mtErr, mtOut.String())
		}
	}

	return &RawCapture{
		BaseSHA:           baseSHA,
		FeatureASHA:       shaA,
		FeatureBSHA:       shaB,
		NameStatusA:       nameStatusA,
		NameStatusB:       nameStatusB,
		NumstatA:          numstatA,
		NumstatB:          numstatB,
		DiffU0A:           diffU0A,
		DiffU0B:           diffU0B,
		RawDiffA:          rawDiffA,
		RawDiffB:          rawDiffB,
		MergeTreeOut:      mtOut.String(),
		MergeTreeConflict: mergeTreeConflict,
	}, nil
}

// NormalizeChanges normalizes and sorts path changes from raw git diff outputs.
func NormalizeChanges(ctx context.Context, repoDir, baseSHA, commitSHA string, raw *RawCapture, isFeatureA bool) ([]PathChange, error) {
	rawDiff := raw.RawDiffA
	numstat := raw.NumstatA
	diffU0 := raw.DiffU0A
	if !isFeatureA {
		rawDiff = raw.RawDiffB
		numstat = raw.NumstatB
		diffU0 = raw.DiffU0B
	}

	changesMap := make(map[string]*PathChange)

	// 1. Parse rawDiff (-z)
	rawTokens := strings.Split(rawDiff, "\x00")
	for i := 0; i < len(rawTokens); i++ {
		tok := strings.TrimSpace(rawTokens[i])
		if tok == "" || !strings.HasPrefix(tok, ":") {
			continue
		}
		// Format: :oldMode newMode oldSHA newSHA status
		fields := strings.Fields(tok)
		if len(fields) < 5 {
			continue
		}
		oldMode := strings.TrimPrefix(fields[0], ":")
		newMode := fields[1]
		status := fields[4]

		var path, oldPath string
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			if i+2 < len(rawTokens) {
				oldPath = rawTokens[i+1]
				path = rawTokens[i+2]
				i += 2
			}
		} else {
			if i+1 < len(rawTokens) {
				path = rawTokens[i+1]
				i++
			}
		}

		if path == "" {
			continue
		}

		pc := changesMap[path]
		if pc == nil {
			pc = &PathChange{Path: path}
			changesMap[path] = pc
		}
		pc.OldPath = oldPath
		pc.Status = status
		pc.OldMode = oldMode
		pc.NewMode = newMode
	}

	// 2. Parse numstat (-z)
	numTokens := strings.Split(numstat, "\x00")
	for i := 0; i < len(numTokens); i++ {
		tok := numTokens[i]
		if strings.TrimSpace(tok) == "" {
			continue
		}
		parts := strings.Split(tok, "\t")
		if len(parts) < 3 {
			continue
		}
		addStr := parts[0]
		delStr := parts[1]
		path := parts[2]

		var oldPath string
		if path == "" && i+2 < len(numTokens) {
			oldPath = numTokens[i+1]
			path = numTokens[i+2]
			i += 2
		}

		if path == "" {
			continue
		}

		pc := changesMap[path]
		if pc == nil {
			pc = &PathChange{Path: path}
			changesMap[path] = pc
		}
		if oldPath != "" && pc.OldPath == "" {
			pc.OldPath = oldPath
		}

		if addStr == "-" || delStr == "-" {
			pc.Binary = true
		} else {
			if a, err := strconv.Atoi(addStr); err == nil {
				pc.AddedLines = a
			}
			if d, err := strconv.Atoi(delStr); err == nil {
				pc.DeletedLines = d
			}
		}
	}

	// 3. Detect labels and finalize
	var result []PathChange
	for _, pc := range changesMap {
		labelsMap := make(map[ChangeLabel]bool)

		// Rename/delete
		if strings.HasPrefix(pc.Status, "R") || strings.HasPrefix(pc.Status, "C") || pc.Status == "D" || (pc.OldPath != "" && pc.OldPath != pc.Path) {
			labelsMap[LabelRenameDelete] = true
		}

		// Binary
		if pc.Binary {
			labelsMap[LabelBinary] = true
		}

		// Mode-only
		if pc.OldMode != pc.NewMode && pc.OldMode != "" && pc.NewMode != "" && pc.AddedLines == 0 && pc.DeletedLines == 0 && !pc.Binary {
			labelsMap[LabelModeOnly] = true
		}

		// Executable
		if strings.HasSuffix(pc.NewMode, "755") || strings.HasSuffix(pc.OldMode, "755") {
			labelsMap[LabelExecutable] = true
		}

		// Symlink / Submodule
		if pc.OldMode == "120000" || pc.NewMode == "120000" || pc.OldMode == "160000" || pc.NewMode == "160000" {
			labelsMap[LabelSymlinkSubmodule] = true
		}

		// Generated
		if isGeneratedFile(ctx, repoDir, commitSHA, pc.Path, pc.Status, diffU0) {
			labelsMap[LabelGenerated] = true
		}

		var labels []ChangeLabel
		for l := range labelsMap {
			labels = append(labels, l)
		}
		sort.Slice(labels, func(i, j int) bool { return labels[i] < labels[j] })
		pc.Labels = labels

		result = append(result, *pc)
	}

	// Sort changes by Path
	sort.Slice(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})

	return result, nil
}

func isGeneratedFile(ctx context.Context, repoDir, commitSHA, path, status, diffU0 string) bool {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".pb.go") ||
		strings.HasSuffix(lower, ".gen.go") ||
		strings.HasSuffix(lower, "_gen.go") ||
		strings.Contains(lower, ".generated.") ||
		strings.HasSuffix(lower, ".min.js") ||
		strings.HasSuffix(lower, ".min.css") {
		return true
	}

	if status == "D" {
		return false
	}

	if repoDir != "" && commitSHA != "" {
		cmd := exec.CommandContext(ctx, "git", "show", commitSHA+":"+path)
		cmd.Dir = repoDir
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil {
			content := out.String()
			if len(content) > 4096 {
				content = content[:4096]
			}
			if strings.Contains(content, "Code generated by") ||
				strings.Contains(content, "DO NOT EDIT") ||
				strings.Contains(content, "@generated") ||
				strings.Contains(content, "AUTO-GENERATED") {
				return true
			}
		}
	}

	return false
}

// ComputeMetrics computes line/path ratios and hotspot weights between normalized changes of feature A and feature B.
func ComputeMetrics(changesA, changesB []PathChange) Metrics {
	mapA := make(map[string]int)
	var pathsA []string
	totalLinesA := 0
	for _, c := range changesA {
		pathsA = append(pathsA, c.Path)
		lines := c.AddedLines + c.DeletedLines
		mapA[c.Path] = lines
		totalLinesA += lines
	}

	mapB := make(map[string]int)
	var pathsB []string
	totalLinesB := 0
	for _, c := range changesB {
		pathsB = append(pathsB, c.Path)
		lines := c.AddedLines + c.DeletedLines
		mapB[c.Path] = lines
		totalLinesB += lines
	}

	var sharedPaths []string
	sharedLinesA := 0
	sharedLinesB := 0
	for p, linesA := range mapA {
		if linesB, ok := mapB[p]; ok {
			sharedPaths = append(sharedPaths, p)
			sharedLinesA += linesA
			sharedLinesB += linesB
		}
	}
	sort.Strings(sharedPaths)

	hotspotA := 0.0
	if totalLinesA > 0 {
		hotspotA = float64(sharedLinesA) / float64(totalLinesA)
	}

	hotspotB := 0.0
	if totalLinesB > 0 {
		hotspotB = float64(sharedLinesB) / float64(totalLinesB)
	}

	hotspot := hotspotA
	if hotspotB > hotspot {
		hotspot = hotspotB
	}

	pathRatioA := 0.0
	if len(pathsA) > 0 {
		pathRatioA = float64(len(sharedPaths)) / float64(len(pathsA))
	}

	pathRatioB := 0.0
	if len(pathsB) > 0 {
		pathRatioB = float64(len(sharedPaths)) / float64(len(pathsB))
	}

	return Metrics{
		SharedPaths:    sharedPaths,
		PathsA:         pathsA,
		PathsB:         pathsB,
		TotalLinesA:    totalLinesA,
		TotalLinesB:    totalLinesB,
		SharedLinesA:   sharedLinesA,
		SharedLinesB:   sharedLinesB,
		HotspotWeightA: hotspotA,
		HotspotWeightB: hotspotB,
		HotspotWeight:  hotspot,
		PathRatioA:     pathRatioA,
		PathRatioB:     pathRatioB,
	}
}

// Classify classifies overlap into required, warning, or informational based on deterministic signals and thresholds.
func Classify(signals Signals, thresholds Thresholds) (Class, []string) {
	var rationales []string

	// Required triggers
	if signals.PredictedConflict {
		if len(signals.ConflictPaths) > 0 {
			rationales = append(rationales, fmt.Sprintf("predicted Git merge conflict detected by merge-tree in %s", strings.Join(signals.ConflictPaths, ", ")))
		} else {
			rationales = append(rationales, "predicted Git merge conflict detected by merge-tree")
		}
	}
	if signals.RenameDeleteCollision {
		if len(signals.CollisionPaths) > 0 {
			rationales = append(rationales, fmt.Sprintf("rename/delete collision detected on %s", strings.Join(signals.CollisionPaths, ", ")))
		} else {
			rationales = append(rationales, "rename/delete collision detected")
		}
	}
	if signals.SharedBinary {
		if len(signals.BinaryPaths) > 0 {
			rationales = append(rationales, fmt.Sprintf("shared binary file modified in both features: %s", strings.Join(signals.BinaryPaths, ", ")))
		} else {
			rationales = append(rationales, "shared binary file modified in both features")
		}
	}
	if signals.IntersectingHunks {
		rationales = append(rationales, "intersecting diff hunks detected in shared path(s)")
	}
	if signals.NearbyHunks {
		rationales = append(rationales, fmt.Sprintf("nearby diff hunks within %d lines detected in shared path(s) (min distance: %d lines)", thresholds.NearbyHunkLines, signals.MinHunkDistance))
	}
	if signals.HotspotWeight >= thresholds.HotspotRequired {
		rationales = append(rationales, fmt.Sprintf("hotspot weight %.3f meets or exceeds required threshold %.2f", signals.HotspotWeight, thresholds.HotspotRequired))
	}

	if len(rationales) > 0 {
		return ClassRequired, rationales
	}

	// Warning triggers
	if signals.SharedDisjointPaths || len(signals.SharedPaths) > 0 {
		rationales = append(rationales, fmt.Sprintf("shared disjoint path(s) modified across both features: %s", strings.Join(signals.SharedPaths, ", ")))
	}
	if signals.HotspotWeight >= thresholds.HotspotWarning {
		rationales = append(rationales, fmt.Sprintf("hotspot weight %.3f meets or exceeds warning threshold %.2f", signals.HotspotWeight, thresholds.HotspotWarning))
	}

	if len(rationales) > 0 {
		return ClassWarning, rationales
	}

	// Informational
	return ClassInformational, []string{
		fmt.Sprintf("disjoint changes with no shared hotspots or nearby hunks (hotspot weight: %.3f)", signals.HotspotWeight),
	}
}

// ExtractSignals derives deterministic signals from raw captures and normalized changes.
func ExtractSignals(raw *RawCapture, changesA, changesB []PathChange, thresholds Thresholds) Signals {
	metrics := ComputeMetrics(changesA, changesB)

	// 1. Conflict extraction from merge-tree
	predictedConflict := false
	var conflictPaths []string
	if raw != nil {
		predictedConflict = raw.MergeTreeConflict
		conflictPaths = extractConflictPaths(raw.MergeTreeOut)
	}

	// 2. Rename/delete collision
	mapA := make(map[string]PathChange)
	for _, c := range changesA {
		mapA[c.Path] = c
		if c.OldPath != "" {
			mapA[c.OldPath] = c
		}
	}
	mapB := make(map[string]PathChange)
	for _, c := range changesB {
		mapB[c.Path] = c
		if c.OldPath != "" {
			mapB[c.OldPath] = c
		}
	}

	collisionMap := make(map[string]bool)
	for _, cA := range changesA {
		isRenameA := cA.OldPath != "" && cA.OldPath != cA.Path
		isDeleteA := cA.Status == "D"
		if isRenameA {
			if _, ok := mapB[cA.OldPath]; ok {
				collisionMap[cA.OldPath] = true
			}
		}
		if isDeleteA {
			if _, ok := mapB[cA.Path]; ok {
				collisionMap[cA.Path] = true
			}
		}
	}
	for _, cB := range changesB {
		isRenameB := cB.OldPath != "" && cB.OldPath != cB.Path
		isDeleteB := cB.Status == "D"
		if isRenameB {
			if _, ok := mapA[cB.OldPath]; ok {
				collisionMap[cB.OldPath] = true
			}
		}
		if isDeleteB {
			if _, ok := mapA[cB.Path]; ok {
				collisionMap[cB.Path] = true
			}
		}
	}

	var collisionPaths []string
	for p := range collisionMap {
		collisionPaths = append(collisionPaths, p)
	}
	sort.Strings(collisionPaths)
	renameDeleteCollision := len(collisionPaths) > 0

	// 3. Shared binaries
	var binaryPaths []string
	binaryMapA := make(map[string]bool)
	for _, c := range changesA {
		if c.Binary {
			binaryMapA[c.Path] = true
		}
	}
	for _, c := range changesB {
		if c.Binary && binaryMapA[c.Path] {
			binaryPaths = append(binaryPaths, c.Path)
		}
	}
	sort.Strings(binaryPaths)
	sharedBinary := len(binaryPaths) > 0

	// 4. Hunk analysis
	var diffU0A, diffU0B string
	if raw != nil {
		diffU0A = raw.DiffU0A
		diffU0B = raw.DiffU0B
	}
	hunkAnalyses, intersectingHunks, nearbyHunks, minDistance := AnalyzeHunks(diffU0A, diffU0B, metrics.SharedPaths, thresholds.NearbyHunkLines)

	// 5. Shared disjoint paths
	sharedDisjointPaths := len(metrics.SharedPaths) > 0 && !intersectingHunks && !nearbyHunks && !sharedBinary && !predictedConflict && !renameDeleteCollision

	return Signals{
		PredictedConflict:     predictedConflict,
		ConflictPaths:         conflictPaths,
		RenameDeleteCollision: renameDeleteCollision,
		CollisionPaths:        collisionPaths,
		SharedBinary:          sharedBinary,
		BinaryPaths:           binaryPaths,
		IntersectingHunks:     intersectingHunks,
		NearbyHunks:           nearbyHunks,
		MinHunkDistance:       minDistance,
		SharedDisjointPaths:   sharedDisjointPaths,
		HotspotWeightA:        metrics.HotspotWeightA,
		HotspotWeightB:        metrics.HotspotWeightB,
		HotspotWeight:         metrics.HotspotWeight,
		PathRatioA:            metrics.PathRatioA,
		PathRatioB:            metrics.PathRatioB,
		SharedPaths:           metrics.SharedPaths,
		PathsA:                metrics.PathsA,
		PathsB:                metrics.PathsB,
		TotalLinesA:           metrics.TotalLinesA,
		TotalLinesB:           metrics.TotalLinesB,
		SharedLinesA:          metrics.SharedLinesA,
		SharedLinesB:          metrics.SharedLinesB,
		HunkAnalyses:          hunkAnalyses,
	}
}

func extractConflictPaths(mergeTreeOut string) []string {
	var paths []string
	seen := make(map[string]bool)
	lines := strings.Split(mergeTreeOut, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "CONFLICT") {
			if idx := strings.Index(l, " in "); idx != -1 {
				p := strings.TrimSpace(l[idx+4:])
				if p != "" && !seen[p] {
					seen[p] = true
					paths = append(paths, p)
				}
			}
		}
	}
	sort.Strings(paths)
	return paths
}

// AnalyzeHunks parses zero-context diffs and compares hunk ranges for shared paths in base coordinates.
func AnalyzeHunks(diffU0A, diffU0B string, sharedPaths []string, nearbyThreshold int) ([]HunkAnalysis, bool, bool, int) {
	if len(sharedPaths) == 0 {
		return nil, false, false, -1
	}

	hunksMapA := parseDiffHunks(diffU0A)
	hunksMapB := parseDiffHunks(diffU0B)

	var analyses []HunkAnalysis
	globalIntersecting := false
	globalNearby := false
	globalMinDistance := -1

	for _, p := range sharedPaths {
		haList := hunksMapA[p]
		hbList := hunksMapB[p]

		if len(haList) == 0 || len(hbList) == 0 {
			continue
		}

		intersecting := false
		nearby := false
		minDist := -1

		for _, ha := range haList {
			endA := ha.OldStart
			if ha.OldLines > 0 {
				endA = ha.OldStart + ha.OldLines - 1
			}

			for _, hb := range hbList {
				endB := hb.OldStart
				if hb.OldLines > 0 {
					endB = hb.OldStart + hb.OldLines - 1
				}

				// Check overlap in base lines
				if ha.OldStart <= endB && hb.OldStart <= endA {
					intersecting = true
					nearby = true
					if minDist == -1 || 0 < minDist {
						minDist = 0
					}
				} else {
					var d int
					if endA < hb.OldStart {
						d = hb.OldStart - endA - 1
					} else {
						d = ha.OldStart - endB - 1
					}
					if minDist == -1 || d < minDist {
						minDist = d
					}
					if d <= nearbyThreshold {
						nearby = true
					}
				}
			}
		}

		if intersecting {
			globalIntersecting = true
		}
		if nearby {
			globalNearby = true
		}
		if minDist != -1 {
			if globalMinDistance == -1 || minDist < globalMinDistance {
				globalMinDistance = minDist
			}
		}

		analyses = append(analyses, HunkAnalysis{
			Path:         p,
			Intersecting: intersecting,
			Nearby:       nearby,
			MinDistance:  minDist,
			HunksA:       haList,
			HunksB:       hbList,
		})
	}

	return analyses, globalIntersecting, globalNearby, globalMinDistance
}

func parseDiffHunks(diffU0 string) map[string][]Hunk {
	result := make(map[string][]Hunk)
	if diffU0 == "" {
		return result
	}

	lines := strings.Split(diffU0, "\n")
	var currentPath string

	for _, l := range lines {
		if strings.HasPrefix(l, "diff --git ") {
			fields := strings.Fields(l)
			if len(fields) >= 4 {
				currentPath = strings.TrimPrefix(fields[3], "b/")
			}
		} else if strings.HasPrefix(l, "+++ b/") {
			currentPath = strings.TrimPrefix(l, "+++ b/")
		} else if strings.HasPrefix(l, "@@ ") && currentPath != "" {
			if h, ok := parseHunkHeader(l); ok {
				result[currentPath] = append(result[currentPath], h)
			}
		}
	}

	return result
}

func parseHunkHeader(line string) (Hunk, bool) {
	if !strings.HasPrefix(line, "@@ -") {
		return Hunk{}, false
	}
	endIdx := strings.Index(line[3:], " @@")
	if endIdx == -1 {
		return Hunk{}, false
	}
	content := line[4 : 3+endIdx]
	parts := strings.Split(content, " +")
	if len(parts) != 2 {
		return Hunk{}, false
	}

	oldParts := strings.Split(parts[0], ",")
	oldStart, err := strconv.Atoi(oldParts[0])
	if err != nil {
		return Hunk{}, false
	}
	oldLines := 1
	if len(oldParts) > 1 {
		oldLines, _ = strconv.Atoi(oldParts[1])
	}

	newParts := strings.Split(parts[1], ",")
	newStart, err := strconv.Atoi(newParts[0])
	if err != nil {
		return Hunk{}, false
	}
	newLines := 1
	if len(newParts) > 1 {
		newLines, _ = strconv.Atoi(newParts[1])
	}

	return Hunk{
		OldStart: oldStart,
		OldLines: oldLines,
		NewStart: newStart,
		NewLines: newLines,
	}, true
}

// EvaluateOptions holds optional configuration for Evaluate.
type EvaluateOptions struct {
	Thresholds         Thresholds
	StructuralProvider StructuralProvider
	Clock              func() time.Time
}

// EvaluateOption configures Evaluate behavior.
type EvaluateOption func(*EvaluateOptions)

// WithThresholds sets custom thresholds for Evaluate.
func WithThresholds(t Thresholds) EvaluateOption {
	return func(o *EvaluateOptions) {
		o.Thresholds = t
	}
}

// WithStructuralProvider sets a custom StructuralProvider for Evaluate.
func WithStructuralProvider(p StructuralProvider) EvaluateOption {
	return func(o *EvaluateOptions) {
		o.StructuralProvider = p
	}
}

// WithClock sets a custom clock for timestamping Evidence.
func WithClock(clock func() time.Time) EvaluateOption {
	return func(o *EvaluateOptions) {
		o.Clock = clock
	}
}

// Evaluate runs the full overlap classification between feature A and feature B.
func Evaluate(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...EvaluateOption) (*Evidence, error) {
	options := EvaluateOptions{
		Thresholds:         DefaultThresholds(),
		StructuralProvider: StubStructuralProvider{},
		Clock:              func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		opt(&options)
	}

	if baseSHA == "" {
		var err error
		baseSHA, err = FindUniqueMergeBase(ctx, repoDir, shaA, shaB)
		if err != nil {
			return nil, fmt.Errorf("overlap: find merge base: %w", err)
		}
	}

	raw, err := CaptureRaw(ctx, repoDir, baseSHA, shaA, shaB)
	if err != nil {
		return nil, fmt.Errorf("overlap: capture raw diffs: %w", err)
	}

	changesA, err := NormalizeChanges(ctx, repoDir, baseSHA, shaA, raw, true)
	if err != nil {
		return nil, fmt.Errorf("overlap: normalize changes A: %w", err)
	}

	changesB, err := NormalizeChanges(ctx, repoDir, baseSHA, shaB, raw, false)
	if err != nil {
		return nil, fmt.Errorf("overlap: normalize changes B: %w", err)
	}

	signals := ExtractSignals(raw, changesA, changesB, options.Thresholds)

	structural, err := options.StructuralProvider.GetStructuralEvidence(ctx, repoDir, baseSHA, shaA, shaB)
	if err != nil {
		// Degradation: fallback to unavailable structural evidence
		structural = StructuralEvidence{
			Available:  false,
			Status:     "unavailable",
			Omitted:    true,
			Disclosure: fmt.Sprintf("structural evidence provider failed (%v); omitted from classification", err),
		}
	}

	class, rationale := Classify(signals, options.Thresholds)

	ev := &Evidence{
		Version:     "v1",
		BaseSHA:     baseSHA,
		FeatureASHA: shaA,
		FeatureBSHA: shaB,
		Class:       class,
		Rationale:   rationale,
		Signals:     signals,
		ChangesA:    changesA,
		ChangesB:    changesB,
		Thresholds:  options.Thresholds,
		Structural:  structural,
		CreatedAt:   options.Clock(),
	}

	hash, err := ev.ComputeHash()
	if err != nil {
		return nil, fmt.Errorf("overlap: compute evidence hash: %w", err)
	}
	ev.Hash = hash

	return ev, nil
}
