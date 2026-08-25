package store_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

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
