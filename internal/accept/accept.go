// Package accept produces immutable mechanical evidence for a frozen lane candidate.
// It never promotes a candidate, mutates refs, or represents its receipt as semantic approval.
package accept

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing/fstest"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/LanzerDevCorp/lucind-ai/internal/candidatechange"
	"github.com/LanzerDevCorp/lucind-ai/internal/integrate"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
	"github.com/LanzerDevCorp/lucind-ai/internal/skillset"
)

const ownerMarkerName = ".lucind-accept-owner.json"

// AcceptanceRequest deliberately contains no refs or caller-supplied identity.
type AcceptanceRequest struct {
	RunID, LaneID string
}

// Binding and AcceptanceReceipt are the public verifier contract.
type Binding = ledger.AcceptanceBinding
type AcceptanceReceipt = ledger.AcceptanceReceipt

// Verifier owns identity loading, isolated checks, fenced cleanup, and receipt persistence.
type Verifier struct {
	primaryRoot   string
	ledger        *ledger.Ledger
	loadCandidate func(context.Context, string, string) (ledger.LaneCandidate, error)
	check         func(context.Context, string) (bool, string, error)
	now           func() time.Time
	newID         func() string
}

func NewVerifier(primaryRoot string, l *ledger.Ledger) *Verifier {
	v := &Verifier{primaryRoot: primaryRoot, ledger: l, check: integrate.Check, now: time.Now, newID: uuid.NewString}
	v.loadCandidate = l.GetLaneCandidate
	return v
}

// Verify validates frozen evidence before cache lookup, checks the candidate in owned isolation,
// cleans that isolation, and only then atomically persists a receipt.
func (v *Verifier) Verify(ctx context.Context, req AcceptanceRequest) (AcceptanceReceipt, error) {
	if v == nil || v.ledger == nil || v.loadCandidate == nil || strings.TrimSpace(req.RunID) == "" || strings.TrimSpace(req.LaneID) == "" {
		return AcceptanceReceipt{}, errors.New("accept: run and lane are required")
	}
	root, err := canonicalRoot(v.primaryRoot)
	if err != nil {
		return AcceptanceReceipt{}, err
	}
	candidate, err := v.loadCandidate(ctx, req.RunID, req.LaneID)
	if err != nil {
		return AcceptanceReceipt{}, fmt.Errorf("accept: load frozen candidate: %w", err)
	}
	if candidate.RunID != req.RunID || candidate.LaneID != req.LaneID || candidate.PacketID == "" || candidate.PacketDigest == "" {
		return AcceptanceReceipt{}, errors.New("accept: frozen lane identity mismatch")
	}
	candidateRoot, err := canonicalRoot(candidate.PrimaryRoot)
	if err != nil || candidateRoot != root {
		return AcceptanceReceipt{}, errors.New("accept: candidate repository root mismatch")
	}
	if err := v.validateObjects(ctx, root, candidate); err != nil {
		return AcceptanceReceipt{}, err
	}
	if candidate.AuthoringEvidenceVersion == ledger.AuthoringEvidenceVersion {
		evidence, err := ledger.DecodeAuthoringEvidence(candidate.AuthoringEvidenceVersion, candidate.AuthoringEvidenceJSON, candidate.AuthoringEvidenceHash)
		if err != nil {
			return AcceptanceReceipt{}, fmt.Errorf("accept: invalid authoring evidence: %w", err)
		}
		metadata, err := v.ledger.GetLaneMetadata(ctx, candidate.RunID, candidate.LaneID)
		if err != nil {
			return AcceptanceReceipt{}, fmt.Errorf("accept: load frozen target metadata: %w", err)
		}
		if err := validateTypedTargetBinding(evidence.Binding, metadata); err != nil {
			return AcceptanceReceipt{}, err
		}
	}
	if err := validateResultAndScope(ctx, root, candidate); err != nil {
		return AcceptanceReceipt{}, err
	}
	binding, err := v.binding(candidate)
	if err != nil {
		return AcceptanceReceipt{}, err
	}
	bindingHash := bindingHash(binding)
	if cached, err := v.ledger.FindAcceptanceReceipt(ctx, bindingHash); err == nil {
		if cached.Binding != binding || cached.ResultHash != candidate.ResultHash {
			return AcceptanceReceipt{}, ledger.ErrAcceptanceBindingMismatch
		}
		return cached, nil
	} else if !errors.Is(err, ledger.ErrAcceptanceReceiptNotFound) {
		return AcceptanceReceipt{}, err
	}

	id := v.newID()
	isolation := v.isolationPath(req.LaneID, id)
	marker := ownerMarker{Root: root, Path: isolation, Candidate: candidate.CandidateCommit, Token: id}
	if err := createOwnedIsolation(ctx, root, isolation, candidate, marker); err != nil {
		return AcceptanceReceipt{}, err
	}
	version, timeout, _, err := integrate.CheckPolicySnapshot()
	if err != nil {
		_ = cleanupOwnedIsolation(context.WithoutCancel(ctx), root, isolation, marker)
		return AcceptanceReceipt{}, err
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	passed, output, checkErr := v.check(checkCtx, isolation)
	cancel()
	cleanupErr := cleanupOwnedIsolation(context.WithoutCancel(ctx), root, isolation, marker)
	if cleanupErr != nil {
		return AcceptanceReceipt{}, fmt.Errorf("accept: cleanup failed: %w", cleanupErr)
	}
	if checkErr != nil {
		return AcceptanceReceipt{}, fmt.Errorf("accept: checks could not execute: %w", checkErr)
	}
	if !passed {
		return AcceptanceReceipt{}, fmt.Errorf("accept: required mechanical checks failed: %s", strings.TrimSpace(output))
	}
	receipt := AcceptanceReceipt{
		ReceiptID: id, BindingHash: bindingHash, Binding: binding, ResultHash: candidate.ResultHash,
		ChecksHash: hashValues("checks:v1", version, output), Cleanup: "removed", CreatedAt: v.now().UTC(),
	}
	stored, err := v.ledger.InsertAcceptanceReceipt(ctx, receipt)
	if err != nil {
		return AcceptanceReceipt{}, fmt.Errorf("accept: persist receipt: %w", err)
	}
	return stored, nil
}

func canonicalRoot(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", errors.New("accept: repository root must be absolute")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("accept: resolve repository root: %w", err)
	}
	return filepath.Clean(resolved), nil
}

func validateTypedTargetBinding(encoded json.RawMessage, metadata ledger.LaneMetadata) error {
	var binding struct {
		Kind              *string `json:"kind"`
		Feature           *string `json:"feature"`
		ParentRef         *string `json:"parent_ref"`
		BaseSHA           *string `json:"base_sha"`
		ExpectedParentSHA *string `json:"expected_parent_sha"`
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&binding); err != nil {
		return errors.New("accept: authored target binding is invalid")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("accept: authored target binding has trailing data")
	}
	if binding.Kind == nil || binding.ParentRef == nil || binding.ExpectedParentSHA == nil {
		return errors.New("accept: authored target binding is incomplete")
	}

	switch *binding.Kind {
	case "feature":
		if binding.Feature == nil || binding.BaseSHA == nil || metadata.Feature == "" || metadata.ParentRef == "" || metadata.BaseSHA == "" || metadata.ExpectedParentSHA == "" ||
			*binding.Feature != metadata.Feature || *binding.ParentRef != metadata.ParentRef || *binding.BaseSHA != metadata.BaseSHA || *binding.ExpectedParentSHA != metadata.ExpectedParentSHA {
			return errors.New("accept: feature target binding mismatch")
		}
	case "legacy-main":
		if binding.Feature != nil || binding.BaseSHA != nil || (*binding.ParentRef != "refs/heads/main" && *binding.ParentRef != "main") ||
			(metadata.ParentRef != "" && metadata.ParentRef != "main" && metadata.ParentRef != "refs/heads/main") || *binding.ExpectedParentSHA != metadata.ExpectedParentSHA {
			return errors.New("accept: legacy target binding mismatch")
		}
	default:
		return errors.New("accept: authored target binding kind is invalid")
	}
	return nil
}

func (v *Verifier) validateObjects(ctx context.Context, root string, c ledger.LaneCandidate) error {
	tests := []struct{ rev, want, label string }{
		{c.BaseCommit + "^{commit}", c.BaseCommit, "base commit"},
		{c.BaseCommit + "^{tree}", c.BaseTree, "base tree"},
		{c.CandidateCommit + "^{commit}", c.CandidateCommit, "candidate commit"},
		{c.CandidateCommit + "^{tree}", c.CandidateTree, "candidate tree"},
	}
	for _, test := range tests {
		got, err := gitOutput(ctx, root, "rev-parse", "--verify", test.rev)
		if err != nil || got != test.want {
			return fmt.Errorf("accept: %s identity mismatch", test.label)
		}
	}
	return nil
}

func validateResultAndScope(ctx context.Context, root string, c ledger.LaneCandidate) error {
	if hashValues("result:v1", c.ResultJSON) != c.ResultHash {
		return errors.New("accept: frozen result hash mismatch")
	}
	envelope, err := result.Read(fstest.MapFS{c.ResultPath: {Data: []byte(c.ResultJSON)}}, c.ResultPath)
	if err != nil {
		return fmt.Errorf("accept: invalid frozen result: %w", err)
	}
	if envelope.Status != "done" || envelope.PacketID != c.PacketID {
		return errors.New("accept: result status or packet identity mismatch")
	}
	for _, stop := range envelope.HardStops {
		if stop.Fired {
			return fmt.Errorf("accept: hard stop fired: %s", stop.HardStop)
		}
	}
	for _, criterion := range envelope.DoneCriteria {
		if !criterion.Met {
			return fmt.Errorf("accept: done criterion unmet: %s", criterion.Criterion)
		}
	}
	if len(envelope.ExternalChanges) != 0 {
		return errors.New("accept: external changes cannot be mechanically accepted")
	}
	actual, err := candidatechange.Collect(ctx, candidatechange.Request{Root: root, BaseCommit: c.BaseCommit, CandidateCommit: c.CandidateCommit})
	if err != nil {
		return fmt.Errorf("accept: inspect frozen diff: %w", err)
	}
	if c.AuthoringEvidenceVersion == ledger.AuthoringEvidenceVersion {
		return validateVersionedEvidence(c, envelope, actual)
	}
	actualPaths := make([]string, 0, len(actual)*2)
	for _, change := range actual {
		actualPaths = append(actualPaths, change.SourcePath, change.Path)
	}
	declared := make([]string, 0, len(envelope.FilesChanged))
	for _, change := range envelope.FilesChanged {
		declared = append(declared, change.SourcePath, change.Path)
	}
	actualPaths = sortedUnique(actualPaths)
	declared = sortedUnique(declared)
	if strings.Join(actualPaths, "\x00") != strings.Join(declared, "\x00") {
		return fmt.Errorf("accept: result files do not exactly match frozen diff: actual=%v declared=%v", actualPaths, declared)
	}
	if outside := candidatechange.OutOfScope(actual, c.AllowedPaths); len(outside) > 0 {
		return fmt.Errorf("accept: out-of-scope changes %v", outside)
	}
	return nil
}

func validateVersionedEvidence(c ledger.LaneCandidate, envelope result.Envelope, actual []candidatechange.Change) error {
	evidence, err := ledger.DecodeAuthoringEvidence(c.AuthoringEvidenceVersion, c.AuthoringEvidenceJSON, c.AuthoringEvidenceHash)
	if err != nil {
		return fmt.Errorf("accept: invalid authoring evidence: %w", err)
	}
	if evidence.PacketDigest != c.PacketDigest || evidence.BaseCommit != c.BaseCommit || evidence.BaseTree != c.BaseTree ||
		evidence.CandidateCommit != c.CandidateCommit || evidence.CandidateTree != c.CandidateTree || evidence.ResultPath != c.ResultPath || evidence.ResultHash != c.ResultHash {
		return errors.New("accept: authoring evidence identity mismatch")
	}
	if evidence.AuthoringMode != "versioned" || evidence.ContractVersion != "packet-author/v1" || evidence.ResultPath != ".lucind/result.json" || evidence.ResultSchema != ".lucind/result.schema.json" {
		return errors.New("accept: unsupported authoring evidence contract")
	}
	var contract struct {
		Version        string   `json:"version"`
		Mode           string   `json:"mode"`
		LaneRole       string   `json:"lane_role"`
		RequiredSkills []string `json:"required_skills"`
		WritePaths     []string `json:"write_paths"`
		ReadOnlyPaths  []string `json:"read_only_paths"`
		DoneCriteria   []string `json:"done_criteria"`
		HardStops      []string `json:"hard_stops"`
		Result         struct {
			Path   string `json:"path"`
			Schema string `json:"schema"`
		} `json:"result"`
	}
	var binding struct {
		Kind    string `json:"kind"`
		BaseSHA string `json:"base_sha"`
	}
	if json.Unmarshal(evidence.Contract, &contract) != nil || json.Unmarshal(evidence.Binding, &binding) != nil || contract.Version != evidence.ContractVersion ||
		contract.Mode != evidence.Mode || (contract.LaneRole != "" && !skillset.IsValidLaneRole(contract.LaneRole)) || !reflect.DeepEqual(contract.WritePaths, evidence.WritePaths) || !reflect.DeepEqual(contract.ReadOnlyPaths, evidence.ReadOnlyPaths) ||
		!reflect.DeepEqual(contract.DoneCriteria, evidence.DoneCriteria) || !reflect.DeepEqual(contract.HardStops, evidence.HardStops) || contract.Result.Path != evidence.ResultPath || contract.Result.Schema != evidence.ResultSchema ||
		(binding.Kind != "feature" && binding.Kind != "legacy-main") || (binding.Kind == "feature" && binding.BaseSHA != evidence.BaseCommit) {
		return errors.New("accept: authored contract or binding integrity mismatch")
	}
	if !reflect.DeepEqual(actual, evidence.Changes) {
		return errors.New("accept: canonical changes differ from frozen evidence")
	}
	criteria := make([]string, len(envelope.DoneCriteria))
	for i := range envelope.DoneCriteria {
		criteria[i] = envelope.DoneCriteria[i].Criterion
	}
	stops := make([]string, len(envelope.HardStops))
	for i := range envelope.HardStops {
		stops[i] = envelope.HardStops[i].HardStop
	}
	declared := make([]candidatechange.Change, len(envelope.FilesChanged))
	for i, change := range envelope.FilesChanged {
		declared[i] = candidatechange.Change{Change: candidatechange.Kind(change.Change), SourcePath: change.SourcePath, Path: change.Path}
	}
	if !reflect.DeepEqual(criteria, evidence.DoneCriteria) || !reflect.DeepEqual(stops, evidence.HardStops) || !reflect.DeepEqual(declared, actual) {
		return errors.New("accept: result does not exactly correspond to authored evidence")
	}
	if len(contract.RequiredSkills) > 0 {
		loaded := make(map[string]bool, len(envelope.SkillsLoaded))
		for _, s := range envelope.SkillsLoaded {
			loaded[strings.TrimSpace(s)] = true
			loaded[canonicalSkillName(s)] = true
		}
		for _, req := range contract.RequiredSkills {
			reqName := strings.TrimSpace(req)
			if !loaded[reqName] && !loaded[canonicalSkillName(reqName)] {
				return errors.New("accept: result does not declare required skills")
			}
		}
	}
	if evidence.Mode == "write" && (evidence.CommitObligation != "required" || envelope.Commit != c.CandidateCommit) {
		return errors.New("accept: write commit mismatch")
	}
	if evidence.Mode == "read-only" && (evidence.CommitObligation != "forbidden" || envelope.Commit != "" || len(actual) != 0) {
		return errors.New("accept: read-only result reports commit or changes")
	}
	if evidence.Mode != "write" && evidence.Mode != "read-only" {
		return errors.New("accept: invalid authored mode")
	}
	if outside := candidatechange.OutOfScope(actual, c.AllowedPaths); len(outside) > 0 {
		return fmt.Errorf("accept: out-of-scope changes %v", outside)
	}
	return nil
}

func (v *Verifier) binding(c ledger.LaneCandidate) (Binding, error) {
	version, timeout, env, err := integrate.CheckPolicySnapshot()
	if err != nil {
		return Binding{}, err
	}
	script, err := gitOutputBytes(context.Background(), c.PrimaryRoot, "show", c.CandidateCommit+":lucind-checks.sh")
	if err != nil {
		return Binding{}, errors.New("accept: candidate has no root lucind-checks.sh")
	}
	bindingVersion, contractVersion, evidenceVersion := "binding:v1", ledger.LegacyAuthoringVersion, c.AuthoringEvidenceVersion
	if evidenceVersion == "" {
		evidenceVersion = ledger.LegacyAuthoringVersion
	}
	if evidenceVersion == ledger.AuthoringEvidenceVersion {
		evidence, decodeErr := ledger.DecodeAuthoringEvidence(evidenceVersion, c.AuthoringEvidenceJSON, c.AuthoringEvidenceHash)
		if decodeErr != nil {
			return Binding{}, decodeErr
		}
		bindingVersion, contractVersion = "binding:v2", evidence.ContractVersion
	}
	return Binding{
		RunID: c.RunID, LaneID: c.LaneID, PacketID: c.PacketID, PacketDigest: c.PacketDigest,
		BaseCommit: c.BaseCommit, BaseTree: c.BaseTree, CandidateCommit: c.CandidateCommit, CandidateTree: c.CandidateTree,
		AllowedPathsHash: hashValues(append([]string{"allowed-paths:v1"}, c.AllowedPaths...)...),
		CheckPolicyHash:  hashValues("check-policy:v1", version, timeout.String(), string(script)),
		EnvironmentHash:  hashValues(append([]string{"environment:v1"}, env...)...),
		BindingVersion:   bindingVersion, ContractVersion: contractVersion,
		AuthoringEvidenceVersion: evidenceVersion, AuthoringEvidenceHash: c.AuthoringEvidenceHash,
	}, nil
}

func bindingHash(b Binding) string {
	return hashValues(b.BindingVersion, b.RunID, b.LaneID, b.PacketID, b.PacketDigest, b.BaseCommit, b.BaseTree,
		b.CandidateCommit, b.CandidateTree, b.AllowedPathsHash, b.CheckPolicyHash, b.EnvironmentHash,
		b.ContractVersion, b.AuthoringEvidenceVersion, b.AuthoringEvidenceHash)
}

func hashValues(values ...string) string {
	h := sha256.New()
	var size [8]byte
	for _, value := range values {
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = h.Write(size[:])
		_, _ = h.Write([]byte(value))
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}

type ownerMarker struct{ Root, Path, Candidate, Token string }

func (v *Verifier) isolationPath(laneID, token string) string {
	clean := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, laneID)
	return filepath.Join(filepath.Dir(v.primaryRoot), filepath.Base(v.primaryRoot)+"-worktrees", "accept-"+clean+"-"+token)
}

func createOwnedIsolation(ctx context.Context, root, path string, c ledger.LaneCandidate, marker ownerMarker) error {
	if _, err := os.Lstat(path); err == nil || !os.IsNotExist(err) {
		return errors.New("accept: isolation path already exists and is not verifier-owned")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := gitOutput(ctx, root, "worktree", "add", "--detach", path, c.CandidateCommit); err != nil {
		return fmt.Errorf("accept: create detached isolation: %w", err)
	}
	data, _ := json.Marshal(marker)
	if err := os.WriteFile(filepath.Join(path, ownerMarkerName), data, 0o600); err != nil {
		return fmt.Errorf("accept: write ownership marker: %w", err)
	}
	top, topErr := gitOutput(ctx, path, "rev-parse", "--show-toplevel")
	head, headErr := gitOutput(ctx, path, "rev-parse", "HEAD")
	_, attachedErr := gitOutput(ctx, path, "symbolic-ref", "-q", "HEAD")
	status, statusErr := gitOutput(ctx, path, "status", "--porcelain", "--untracked-files=no")
	if topErr != nil || headErr != nil || statusErr != nil || filepath.Clean(top) != filepath.Clean(path) || head != c.CandidateCommit || attachedErr == nil || status != "" {
		return errors.New("accept: isolation is not the clean detached frozen candidate")
	}
	return nil
}

func cleanupOwnedIsolation(ctx context.Context, root, path string, want ownerMarker) error {
	data, err := os.ReadFile(filepath.Join(path, ownerMarkerName))
	if err != nil {
		return errors.New("ownership marker missing; foreign worktree preserved")
	}
	var got ownerMarker
	if json.Unmarshal(data, &got) != nil || got != want {
		return errors.New("ownership marker mismatch; foreign worktree preserved")
	}
	head, err := gitOutput(ctx, path, "rev-parse", "HEAD")
	if err != nil || head != want.Candidate {
		return errors.New("owned worktree HEAD mismatch; worktree preserved")
	}
	if _, err := gitOutput(ctx, root, "worktree", "remove", "--force", path); err != nil {
		return err
	}
	_, _ = gitOutput(ctx, root, "worktree", "prune")
	return nil
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := gitOutputBytes(ctx, dir, args...)
	return strings.TrimSpace(string(out)), err
}

func gitOutputBytes(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func splitNUL(data []byte) []string {
	parts := strings.Split(string(data), "\x00")
	out := parts[:0]
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func sortedUnique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != "" {
			seen[value] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func canonicalSkillName(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, "\\", "/")
	if strings.HasSuffix(s, "/SKILL.md") {
		s = strings.TrimSuffix(s, "/SKILL.md")
		if idx := strings.LastIndex(s, "/"); idx >= 0 {
			return s[idx+1:]
		}
		return s
	}
	if idx := strings.LastIndex(s, "/"); idx >= 0 {
		return s[idx+1:]
	}
	return s
}
