// Package accept produces immutable mechanical evidence for a frozen lane candidate.
// It never promotes a candidate, mutates refs, or represents its receipt as semantic approval.
package accept

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing/fstest"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/LanzerDevCorp/lucind-ai/internal/integrate"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
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
	actualRaw, err := gitOutputBytes(ctx, root, "diff", "--name-only", "-z", c.BaseCommit, c.CandidateCommit, "--")
	if err != nil {
		return fmt.Errorf("accept: inspect frozen diff: %w", err)
	}
	actual := splitNUL(actualRaw)
	declared := make([]string, 0, len(envelope.FilesChanged))
	for _, change := range envelope.FilesChanged {
		declared = append(declared, change.Path)
	}
	actual = sortedUnique(actual)
	declared = sortedUnique(declared)
	if strings.Join(actual, "\x00") != strings.Join(declared, "\x00") {
		return fmt.Errorf("accept: result files do not exactly match frozen diff: actual=%v declared=%v", actual, declared)
	}
	for _, path := range actual {
		if !packet.PathInScope(path, c.AllowedPaths) {
			return fmt.Errorf("accept: out-of-scope change %q", path)
		}
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
	return Binding{
		RunID: c.RunID, LaneID: c.LaneID, PacketID: c.PacketID, PacketDigest: c.PacketDigest,
		BaseCommit: c.BaseCommit, BaseTree: c.BaseTree, CandidateCommit: c.CandidateCommit, CandidateTree: c.CandidateTree,
		AllowedPathsHash: hashValues(append([]string{"allowed-paths:v1"}, c.AllowedPaths...)...),
		CheckPolicyHash:  hashValues("check-policy:v1", version, timeout.String(), string(script)),
		EnvironmentHash:  hashValues(append([]string{"environment:v1"}, env...)...),
	}, nil
}

func bindingHash(b Binding) string {
	return hashValues("binding:v1", b.RunID, b.LaneID, b.PacketID, b.PacketDigest, b.BaseCommit, b.BaseTree,
		b.CandidateCommit, b.CandidateTree, b.AllowedPathsHash, b.CheckPolicyHash, b.EnvironmentHash)
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
