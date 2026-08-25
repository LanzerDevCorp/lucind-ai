package fixture_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/overlap"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability/fixture"
)

func TestFixtureDeterministicSyntheticPackets(t *testing.T) {
	// Test Change A Packet
	packetA := fixture.ChangeAPacket()
	if packetA.ID != "stability-change-a" {
		t.Errorf("ChangeAPacket ID = %q, want stability-change-a", packetA.ID)
	}
	if packetA.Executor != "agy" {
		t.Errorf("ChangeAPacket Executor = %q, want agy", packetA.Executor)
	}
	if packetA.Model != "gemini-3.7-flash-high" {
		t.Errorf("ChangeAPacket Model = %q, want gemini-3.7-flash-high", packetA.Model)
	}
	if len(packetA.AllowedPaths) != 1 || packetA.AllowedPaths[0] != "fixture/change_a.txt" {
		t.Errorf("ChangeAPacket AllowedPaths = %v, want [fixture/change_a.txt]", packetA.AllowedPaths)
	}

	parsedA, err := packet.Parse(strings.NewReader(fixture.ChangeAPacketMarkdown()))
	if err != nil {
		t.Fatalf("Parse(ChangeAPacketMarkdown) error = %v", err)
	}
	if parsedA.ID != packetA.ID || parsedA.Model != packetA.Model || parsedA.Executor != packetA.Executor {
		t.Errorf("parsedA = %+v, want %+v", parsedA, packetA)
	}

	// Test Change B Packet
	packetB := fixture.ChangeBPacket()
	if packetB.ID != "stability-change-b" {
		t.Errorf("ChangeBPacket ID = %q, want stability-change-b", packetB.ID)
	}
	if packetB.Executor != "agy" {
		t.Errorf("ChangeBPacket Executor = %q, want agy", packetB.Executor)
	}
	if packetB.Model != "gemini-3.7-flash-high" {
		t.Errorf("ChangeBPacket Model = %q, want gemini-3.7-flash-high", packetB.Model)
	}
	if len(packetB.AllowedPaths) != 1 || packetB.AllowedPaths[0] != "fixture/change_b.txt" {
		t.Errorf("ChangeBPacket AllowedPaths = %v, want [fixture/change_b.txt]", packetB.AllowedPaths)
	}

	parsedB, err := packet.Parse(strings.NewReader(fixture.ChangeBPacketMarkdown()))
	if err != nil {
		t.Fatalf("Parse(ChangeBPacketMarkdown) error = %v", err)
	}
	if parsedB.ID != packetB.ID || parsedB.Model != packetB.Model || parsedB.Executor != packetB.Executor {
		t.Errorf("parsedB = %+v, want %+v", parsedB, packetB)
	}

	// Test Fix Change Packet
	packetFix := fixture.FixChangePacket()
	if packetFix.ID != "stability-fix-a" {
		t.Errorf("FixChangePacket ID = %q, want stability-fix-a", packetFix.ID)
	}
	if packetFix.Executor != "agy" {
		t.Errorf("FixChangePacket Executor = %q, want agy", packetFix.Executor)
	}
	if packetFix.Model != "gemini-3.7-flash-high" {
		t.Errorf("FixChangePacket Model = %q, want gemini-3.7-flash-high", packetFix.Model)
	}
	if len(packetFix.AllowedPaths) != 1 || packetFix.AllowedPaths[0] != "fixture/defect.txt" {
		t.Errorf("FixChangePacket AllowedPaths = %v, want [fixture/defect.txt]", packetFix.AllowedPaths)
	}

	parsedFix, err := packet.Parse(strings.NewReader(fixture.FixChangePacketMarkdown()))
	if err != nil {
		t.Fatalf("Parse(FixChangePacketMarkdown) error = %v", err)
	}
	if parsedFix.ID != packetFix.ID || parsedFix.Model != packetFix.Model || parsedFix.Executor != packetFix.Executor {
		t.Errorf("parsedFix = %+v, want %+v", parsedFix, packetFix)
	}
}

func TestFixtureCheckScriptDefectAndPass(t *testing.T) {
	tempDir := t.TempDir()
	if err := fixture.MaterializeFixtures(tempDir); err != nil {
		t.Fatalf("MaterializeFixtures() error = %v", err)
	}

	ctx := context.Background()

	// 1. Change B check passes initially despite seeded defect.
	outB, errB := fixture.RunCheck(ctx, tempDir, "change-b")
	if errB != nil {
		t.Fatalf("RunCheck(change-b) error = %v, out = %q, want nil (pass)", errB, outB)
	}
	if !strings.Contains(outB, "CHECK SUCCESS") {
		t.Errorf("RunCheck(change-b) output = %q, want CHECK SUCCESS", outB)
	}

	// 2. Change A check fails due to seeded defect in fixture/defect.txt.
	outA, errA := fixture.RunCheck(ctx, tempDir, "change-a")
	if errA == nil {
		t.Fatalf("RunCheck(change-a) error = nil, want check failure on seeded defect; out = %q", outA)
	}
	if !strings.Contains(outA, "CHECK FAILURE: Seeded defect") {
		t.Errorf("RunCheck(change-a) output = %q, want CHECK FAILURE message", outA)
	}

	// 3. Unknown target returns error.
	_, errUnknown := fixture.RunCheck(ctx, tempDir, "unknown")
	if errUnknown == nil {
		t.Error("RunCheck(unknown) error = nil, want error")
	}

	// 4. Remediate defect in fixture/defect.txt.
	defectPath := filepath.Join(tempDir, "fixture", "defect.txt")
	if err := os.WriteFile(defectPath, []byte("STATUS=FIXED\nERROR=NONE\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(defect.txt) error = %v", err)
	}

	// 5. Change A check now passes after remediation.
	outAFixed, errAFixed := fixture.RunCheck(ctx, tempDir, "change-a")
	if errAFixed != nil {
		t.Fatalf("RunCheck(change-a) after fix error = %v, out = %q, want nil (pass)", errAFixed, outAFixed)
	}
	if !strings.Contains(outAFixed, "CHECK SUCCESS") {
		t.Errorf("RunCheck(change-a) after fix output = %q, want CHECK SUCCESS", outAFixed)
	}
}

func TestFixtureTreeHashDeterministic(t *testing.T) {
	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()

	if err := fixture.MaterializeFixtures(tempDir1); err != nil {
		t.Fatalf("MaterializeFixtures(1) error = %v", err)
	}
	if err := fixture.MaterializeFixtures(tempDir2); err != nil {
		t.Fatalf("MaterializeFixtures(2) error = %v", err)
	}

	hash1, err := fixture.ComputeFixtureTreeHash(tempDir1)
	if err != nil {
		t.Fatalf("ComputeFixtureTreeHash(1) error = %v", err)
	}
	hash2, err := fixture.ComputeFixtureTreeHash(tempDir2)
	if err != nil {
		t.Fatalf("ComputeFixtureTreeHash(2) error = %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("hash1 (%s) != hash2 (%s), want identical deterministic hashes", hash1, hash2)
	}

	// Modifying a file should change the hash.
	defectPath := filepath.Join(tempDir1, "fixture", "defect.txt")
	if err := os.WriteFile(defectPath, []byte("STATUS=FIXED\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(defect.txt) error = %v", err)
	}

	hashModified, err := fixture.ComputeFixtureTreeHash(tempDir1)
	if err != nil {
		t.Fatalf("ComputeFixtureTreeHash(modified) error = %v", err)
	}
	if hashModified == hash1 {
		t.Errorf("hashModified = %s, want different hash after modification", hashModified)
	}

	// Missing file in ComputeTreeHash returns error.
	_, errMissing := fixture.ComputeTreeHash(tempDir1, []string{"nonexistent.txt"})
	if errMissing == nil {
		t.Error("ComputeTreeHash on nonexistent file error = nil, want error")
	}
}

// TestFixtureConcurrentJourneyDefectAndAncestryIsolation proves ancestry
// verification against real Git commits in a temporary repository.
func TestFixtureConcurrentJourneyDefectAndAncestryIsolation(t *testing.T) {
	repoDir := t.TempDir()

	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v\nOutput:\n%s", strings.Join(args, " "), err, string(out))
		}
		return strings.TrimSpace(string(out))
	}

	// Initialize temp git repo.
	runGit("init")
	runGit("config", "user.name", "Stability Test")
	runGit("config", "user.email", "stability@example.com")

	// 1. Initial base commit.
	baseFile := filepath.Join(repoDir, "base.txt")
	if err := os.WriteFile(baseFile, []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "base.txt")
	runGit("commit", "-m", "initial base commit")
	baseSHA := runGit("rev-parse", "HEAD")

	// 2. Create Target A branch with Fix commit and Change A commit.
	runGit("checkout", "-b", "target-a")
	fixFile := filepath.Join(repoDir, "fix.txt")
	if err := os.WriteFile(fixFile, []byte("fix\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "fix.txt")
	runGit("commit", "-m", "fix defect")
	fixSHA := runGit("rev-parse", "HEAD")

	changeAFile := filepath.Join(repoDir, "change_a.txt")
	if err := os.WriteFile(changeAFile, []byte("change a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "change_a.txt")
	runGit("commit", "-m", "change a implementation")
	targetASHA := runGit("rev-parse", "HEAD")

	// 3. Create Target B branch branched off baseSHA with only Change B commit.
	runGit("checkout", "-b", "target-b", baseSHA)
	changeBFile := filepath.Join(repoDir, "change_b.txt")
	if err := os.WriteFile(changeBFile, []byte("change b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "change_b.txt")
	runGit("commit", "-m", "change b implementation")
	targetBSHA := runGit("rev-parse", "HEAD")

	ctx := context.Background()

	// Direct IsAncestor checks
	isAnc, err := fixture.IsAncestor(ctx, repoDir, baseSHA, targetASHA)
	if err != nil || !isAnc {
		t.Errorf("IsAncestor(base, targetA) = (%v, %v), want (true, nil)", isAnc, err)
	}
	isAnc, err = fixture.IsAncestor(ctx, repoDir, targetASHA, baseSHA)
	if err != nil || isAnc {
		t.Errorf("IsAncestor(targetA, base) = (%v, %v), want (false, nil)", isAnc, err)
	}

	// 4. Verify valid isolated ancestry:
	// - Base is ancestor of Fix, Target A, and Target B
	// - Fix is ancestor of Target A, but NOT Target B
	// - Target A and Target B are independent
	// - Merge base between Target A and Target B is baseSHA
	err = fixture.VerifyAncestryIsolation(ctx, repoDir, baseSHA, targetASHA, targetBSHA, fixSHA)
	if err != nil {
		t.Fatalf("VerifyAncestryIsolation() error = %v, want nil", err)
	}

	// 5. Test contaminated Target B (e.g. Target B merges Fix commit).
	runGit("checkout", "target-b")
	runGit("merge", "--no-ff", "-m", "merge fix into target b", fixSHA)
	contaminatedBSHA := runGit("rev-parse", "HEAD")

	contamErr := fixture.VerifyAncestryIsolation(ctx, repoDir, baseSHA, targetASHA, contaminatedBSHA, fixSHA)
	if contamErr == nil {
		t.Fatal("VerifyAncestryIsolation() on contaminated Target B = nil, want error")
	}
	if !errors.Is(contamErr, fixture.ErrContaminatedTarget) && !errors.Is(contamErr, fixture.ErrAncestryViolation) {
		t.Errorf("contamErr = %v, want ErrContaminatedTarget or ErrAncestryViolation", contamErr)
	}

	// 6. Test invalid base SHA.
	invalidBaseErr := fixture.VerifyAncestryIsolation(ctx, repoDir, targetASHA, baseSHA, targetBSHA, fixSHA)
	if invalidBaseErr == nil {
		t.Fatal("VerifyAncestryIsolation() with invalid base = nil, want error")
	}

	// 7. Test disjoint repository / no merge base.
	disjointRepoDir := t.TempDir()
	runGitDisjoint := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = disjointRepoDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v\nOutput:\n%s", strings.Join(args, " "), err, string(out))
		}
		return strings.TrimSpace(string(out))
	}
	runGitDisjoint("init")
	runGitDisjoint("config", "user.name", "Stability Test")
	runGitDisjoint("config", "user.email", "stability@example.com")
	f1 := filepath.Join(disjointRepoDir, "f1.txt")
	_ = os.WriteFile(f1, []byte("f1\n"), 0o644)
	runGitDisjoint("add", "f1.txt")
	runGitDisjoint("commit", "-m", "commit 1")
	sha1 := runGitDisjoint("rev-parse", "HEAD")

	runGitDisjoint("checkout", "--orphan", "orphan-branch")
	f2 := filepath.Join(disjointRepoDir, "f2.txt")
	_ = os.WriteFile(f2, []byte("f2\n"), 0o644)
	runGitDisjoint("add", "f2.txt")
	runGitDisjoint("commit", "-m", "orphan commit")
	sha2 := runGitDisjoint("rev-parse", "HEAD")

	errNoBase := fixture.VerifyAncestryIsolation(ctx, disjointRepoDir, sha1, sha1, sha2, "")
	if !errors.Is(errNoBase, overlap.ErrNoMergeBase) && !errors.Is(errNoBase, fixture.ErrAncestryViolation) {
		t.Errorf("disjoint history error = %v, want ErrNoMergeBase or ErrAncestryViolation", errNoBase)
	}
}
