package store_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/ledgerpath"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability/store"
)

type mockGitRunner struct {
	commonDir string
	err       error
}

func (m *mockGitRunner) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []byte(m.commonDir + "\n"), nil
}

func TestPreflightResolvesGitCommonDirAuthority(t *testing.T) {
	ctx := context.Background()
	repoDir := "/path/to/repo/worktree-1"
	expectedCommonDir := "/path/to/repo/.git"

	runner := &mockGitRunner{commonDir: expectedCommonDir}

	resolved, err := store.ResolvePath(ctx, runner, repoDir)
	if err != nil {
		t.Fatalf("ResolvePath failed: %v", err)
	}

	expectedPath := filepath.Join(expectedCommonDir, "lucind-ai", "stability", "v1", "stability.db")
	if resolved != expectedPath {
		t.Errorf("ResolvePath = %q, want %q", resolved, expectedPath)
	}
}

func TestCommonDirResolutionDiffersFromPrimaryRoot(t *testing.T) {
	ctx := context.Background()
	primaryRoot := "/path/to/repo"
	worktreeDir := "/path/to/repo-worktrees/lane-1"
	gitCommonDir := "/path/to/repo/.git"

	runner := &mockGitRunner{commonDir: gitCommonDir}

	storePathFromWorktree, err := store.ResolvePath(ctx, runner, worktreeDir)
	if err != nil {
		t.Fatalf("ResolvePath from worktree failed: %v", err)
	}

	storePathFromPrimary, err := store.ResolvePath(ctx, runner, primaryRoot)
	if err != nil {
		t.Fatalf("ResolvePath from primary root failed: %v", err)
	}

	primaryLedgerPath := ledgerpath.Resolve(primaryRoot)

	// 1. Assert store paths from worktree and primary resolve to the identical common dir path
	if storePathFromWorktree != storePathFromPrimary {
		t.Errorf("storePathFromWorktree (%q) != storePathFromPrimary (%q)", storePathFromWorktree, storePathFromPrimary)
	}

	// 2. Assert store path NEVER equals primary ledger path (<primaryRoot>/.lucind/lucind.db)
	if storePathFromPrimary == primaryLedgerPath {
		t.Errorf("store path (%q) must not equal primary ledger path (%q)", storePathFromPrimary, primaryLedgerPath)
	}

	// 3. Assert store path is under <git-common-dir>/lucind-ai/stability/v1/
	wantPrefix := filepath.Join(gitCommonDir, "lucind-ai", "stability", "v1")
	if !strings.HasPrefix(storePathFromPrimary, wantPrefix) {
		t.Errorf("store path %q does not have expected prefix %q", storePathFromPrimary, wantPrefix)
	}
}

func TestStoreOpenWALAndPragmas(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "lucind-ai", "stability", "v1", "stability.db")

	s, err := store.OpenAtPath(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenAtPath failed: %v", err)
	}
	defer s.Close()

	// Verify WAL journal mode
	var journalMode string
	if err := s.DB().QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if strings.ToLower(journalMode) != "wal" {
		t.Errorf("journal_mode = %q, want %q", journalMode, "wal")
	}

	// Verify foreign_keys
	var foreignKeys int
	if err := s.DB().QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Errorf("foreign_keys = %d, want 1", foreignKeys)
	}

	// Verify busy_timeout
	var busyTimeout int
	if err := s.DB().QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("query busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("busy_timeout = %d, want 5000", busyTimeout)
	}
}

func TestStoreSingleActiveGateAndCommonDirResolution(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "lucind-ai", "stability", "v1", "stability.db")

	s, err := store.OpenAtPath(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenAtPath failed: %v", err)
	}
	defer s.Close()

	// 1. Initially no active campaign
	_, err = s.GetActiveCampaign(ctx)
	if !errors.Is(err, store.ErrCampaignNotFound) {
		t.Fatalf("GetActiveCampaign on empty db returned %v, want ErrCampaignNotFound", err)
	}

	// 2. Start campaign 1
	c1, err := s.CreateCampaign(ctx, "camp-1", "abc1234")
	if err != nil {
		t.Fatalf("CreateCampaign 1 failed: %v", err)
	}
	if c1.ID != "camp-1" || c1.Status != store.StatusRunning {
		t.Errorf("c1 unexpected: %+v", c1)
	}

	// 3. Active campaign is c1
	active, err := s.GetActiveCampaign(ctx)
	if err != nil {
		t.Fatalf("GetActiveCampaign failed: %v", err)
	}
	if active.ID != "camp-1" {
		t.Errorf("active campaign ID = %q, want camp-1", active.ID)
	}

	// 4. Attempt to start campaign 2 while campaign 1 is unclosed -> must fail with ErrCampaignActive
	_, err = s.CreateCampaign(ctx, "camp-2", "def5678")
	if !errors.Is(err, store.ErrCampaignActive) {
		t.Fatalf("CreateCampaign 2 returned %v, want ErrCampaignActive", err)
	}

	// 5. Close campaign 1 with passed status
	if err := s.UpdateCampaignStatus(ctx, "camp-1", store.StatusPassed); err != nil {
		t.Fatalf("UpdateCampaignStatus failed: %v", err)
	}

	// 6. Active campaign now returns ErrCampaignNotFound
	_, err = s.GetActiveCampaign(ctx)
	if !errors.Is(err, store.ErrCampaignNotFound) {
		t.Fatalf("GetActiveCampaign after completion returned %v, want ErrCampaignNotFound", err)
	}

	// 7. Start campaign 3 now succeeds
	c3, err := s.CreateCampaign(ctx, "camp-3", "9876fed")
	if err != nil {
		t.Fatalf("CreateCampaign 3 failed: %v", err)
	}
	if c3.ID != "camp-3" || c3.Status != store.StatusRunning {
		t.Errorf("c3 unexpected: %+v", c3)
	}
}

func TestStoreConcurrentCampaignRejectionRace(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "lucind-ai", "stability", "v1", "stability.db")

	s, err := store.OpenAtPath(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenAtPath failed: %v", err)
	}
	defer s.Close()

	const racers = 10
	startGate := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(racers)

	type raceResult struct {
		campaign store.Campaign
		err      error
	}
	results := make([]raceResult, racers)

	for i := 0; i < racers; i++ {
		go func(idx int) {
			defer wg.Done()
			<-startGate
			campID := fmt.Sprintf("camp-race-%d", idx)
			c, rErr := s.CreateCampaign(ctx, campID, "sha-race")
			results[idx] = raceResult{campaign: c, err: rErr}
		}(i)
	}

	// Unleash all goroutines simultaneously
	close(startGate)
	wg.Wait()

	successCount := 0
	activeErrCount := 0
	for _, res := range results {
		if res.err == nil {
			successCount++
		} else if errors.Is(res.err, store.ErrCampaignActive) {
			activeErrCount++
		} else {
			t.Errorf("unexpected error in concurrent race: %v", res.err)
		}
	}

	if successCount != 1 {
		t.Fatalf("expected exactly 1 successful campaign creation, got %d", successCount)
	}
	if activeErrCount != racers-1 {
		t.Fatalf("expected %d ErrCampaignActive errors, got %d", racers-1, activeErrCount)
	}
}

func TestStoreOpenWithGitRunner(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	gitCommonDir := filepath.Join(tmpDir, "primary.git")
	runner := &mockGitRunner{commonDir: gitCommonDir}

	s, err := store.Open(ctx, runner, tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer s.Close()

	c, err := s.CreateCampaign(ctx, "camp-open", "sha123")
	if err != nil {
		t.Fatalf("CreateCampaign failed: %v", err)
	}

	fetched, err := s.GetCampaign(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetCampaign failed: %v", err)
	}
	if fetched.ID != c.ID || fetched.CandidateSHA != c.CandidateSHA || fetched.Status != store.StatusRunning {
		t.Errorf("fetched mismatch: %+v", fetched)
	}
}

func TestStoreTrialProgressUpsertAndGet(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "lucind-ai", "stability", "v1", "stability.db")

	s, err := store.OpenAtPath(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenAtPath failed: %v", err)
	}
	defer s.Close()

	// 1. GetTrialProgress on non-existent campaign returns ErrTrialProgressNotFound
	_, err = s.GetTrialProgress(ctx, "nonexistent-campaign")
	if !errors.Is(err, store.ErrTrialProgressNotFound) {
		t.Fatalf("GetTrialProgress for nonexistent campaign returned %v, want ErrTrialProgressNotFound", err)
	}

	// 2. UpsertTrialStage for trial 1: initial insert
	if err := s.UpsertTrialStage(ctx, "camp-1", 1, "admitted"); err != nil {
		t.Fatalf("UpsertTrialStage(camp-1, 1, admitted) failed: %v", err)
	}

	p1, err := s.GetTrialProgress(ctx, "camp-1")
	if err != nil {
		t.Fatalf("GetTrialProgress(camp-1) failed: %v", err)
	}
	if p1.CampaignID != "camp-1" || p1.TrialNumber != 1 || p1.Stage != "admitted" {
		t.Errorf("p1 mismatch: %+v", p1)
	}
	if p1.PGIDA != nil || p1.PGIDB != nil || p1.PGIDFix != nil {
		t.Errorf("expected nil PGIDs, got A=%v, B=%v, Fix=%v", p1.PGIDA, p1.PGIDB, p1.PGIDFix)
	}
	if p1.CreatedAt.IsZero() || p1.UpdatedAt.IsZero() {
		t.Errorf("timestamps not populated: created=%v, updated=%v", p1.CreatedAt, p1.UpdatedAt)
	}

	// 3. UpsertTrialStage for trial 1: update stage and verify updated_at advances
	time.Sleep(5 * time.Millisecond)
	if err := s.UpsertTrialStage(ctx, "camp-1", 1, "dispatching"); err != nil {
		t.Fatalf("UpsertTrialStage(camp-1, 1, dispatching) failed: %v", err)
	}

	p1Updated, err := s.GetTrialProgress(ctx, "camp-1")
	if err != nil {
		t.Fatalf("GetTrialProgress(camp-1) failed: %v", err)
	}
	if p1Updated.Stage != "dispatching" {
		t.Errorf("p1Updated.Stage = %q, want 'dispatching'", p1Updated.Stage)
	}
	if p1Updated.UpdatedAt.Before(p1.UpdatedAt) || p1Updated.UpdatedAt.Equal(p1.UpdatedAt) {
		t.Errorf("p1Updated.UpdatedAt (%v) did not advance after p1.UpdatedAt (%v)", p1Updated.UpdatedAt, p1.UpdatedAt)
	}
	if !p1Updated.CreatedAt.Equal(p1.CreatedAt) {
		t.Errorf("p1Updated.CreatedAt (%v) changed from original (%v)", p1Updated.CreatedAt, p1.CreatedAt)
	}

	// 4. UpdateTrialPGID: lane "b"
	if err := s.UpdateTrialPGID(ctx, "camp-1", 1, "b", 12345); err != nil {
		t.Fatalf("UpdateTrialPGID(camp-1, 1, b, 12345) failed: %v", err)
	}

	p1WithB, err := s.GetTrialProgress(ctx, "camp-1")
	if err != nil {
		t.Fatalf("GetTrialProgress(camp-1) failed: %v", err)
	}
	if p1WithB.PGIDB == nil || *p1WithB.PGIDB != 12345 {
		t.Errorf("p1WithB.PGIDB = %v, want 12345", p1WithB.PGIDB)
	}
	if p1WithB.PGIDA != nil || p1WithB.PGIDFix != nil {
		t.Errorf("PGIDA or PGIDFix unexpectedly set: A=%v, Fix=%v", p1WithB.PGIDA, p1WithB.PGIDFix)
	}
	if p1WithB.Stage != "dispatching" {
		t.Errorf("stage corrupted during UpdateTrialPGID: got %q, want dispatching", p1WithB.Stage)
	}

	// 5. UpdateTrialPGID: lanes "a" and "fix"
	if err := s.UpdateTrialPGID(ctx, "camp-1", 1, "a", 11111); err != nil {
		t.Fatalf("UpdateTrialPGID(camp-1, 1, a, 11111) failed: %v", err)
	}
	if err := s.UpdateTrialPGID(ctx, "camp-1", 1, "fix", 22222); err != nil {
		t.Fatalf("UpdateTrialPGID(camp-1, 1, fix, 22222) failed: %v", err)
	}

	p1AllLanes, err := s.GetTrialProgress(ctx, "camp-1")
	if err != nil {
		t.Fatalf("GetTrialProgress(camp-1) failed: %v", err)
	}
	if p1AllLanes.PGIDA == nil || *p1AllLanes.PGIDA != 11111 {
		t.Errorf("p1AllLanes.PGIDA = %v, want 11111", p1AllLanes.PGIDA)
	}
	if p1AllLanes.PGIDB == nil || *p1AllLanes.PGIDB != 12345 {
		t.Errorf("p1AllLanes.PGIDB = %v, want 12345", p1AllLanes.PGIDB)
	}
	if p1AllLanes.PGIDFix == nil || *p1AllLanes.PGIDFix != 22222 {
		t.Errorf("p1AllLanes.PGIDFix = %v, want 22222", p1AllLanes.PGIDFix)
	}

	// 6. UpdateTrialPGID: invalid lane returns error
	if err := s.UpdateTrialPGID(ctx, "camp-1", 1, "invalid_lane", 9999); err == nil {
		t.Errorf("UpdateTrialPGID with invalid lane returned nil error, want error")
	}

	// 7. UpdateTrialPGID before UpsertTrialStage on new trial 2 (defensive upsert)
	if err := s.UpdateTrialPGID(ctx, "camp-1", 2, "b", 54321); err != nil {
		t.Fatalf("UpdateTrialPGID on new trial 2 failed: %v", err)
	}
	p2Initial, err := s.GetTrialProgress(ctx, "camp-1")
	if err != nil {
		t.Fatalf("GetTrialProgress(camp-1) for trial 2 failed: %v", err)
	}
	if p2Initial.TrialNumber != 2 {
		t.Errorf("GetTrialProgress did not return highest trial number (2), got %d", p2Initial.TrialNumber)
	}
	if p2Initial.PGIDB == nil || *p2Initial.PGIDB != 54321 {
		t.Errorf("p2Initial.PGIDB = %v, want 54321", p2Initial.PGIDB)
	}

	// Now UpsertTrialStage on trial 2 should preserve PGIDB
	if err := s.UpsertTrialStage(ctx, "camp-1", 2, "dispatching"); err != nil {
		t.Fatalf("UpsertTrialStage on trial 2 failed: %v", err)
	}
	p2Updated, err := s.GetTrialProgress(ctx, "camp-1")
	if err != nil {
		t.Fatalf("GetTrialProgress(camp-1) after upsert stage failed: %v", err)
	}
	if p2Updated.Stage != "dispatching" {
		t.Errorf("p2Updated.Stage = %q, want dispatching", p2Updated.Stage)
	}
	if p2Updated.PGIDB == nil || *p2Updated.PGIDB != 54321 {
		t.Errorf("p2Updated.PGIDB = %v, want 54321 (preserved)", p2Updated.PGIDB)
	}
}

func TestStoreMigrationCreatesTrialProgressTable(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "lucind-ai", "stability", "v1", "stability.db")

	s, err := store.OpenAtPath(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenAtPath failed: %v", err)
	}
	defer s.Close()

	var version int
	if err := s.DB().QueryRowContext(ctx, "SELECT MAX(version) FROM schema_migrations").Scan(&version); err != nil {
		t.Fatalf("query schema_migrations version: %v", err)
	}
	if version != 2 {
		t.Errorf("schema_migrations MAX(version) = %d, want 2", version)
	}
}
