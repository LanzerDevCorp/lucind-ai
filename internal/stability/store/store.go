// Package store provides SQLite/WAL durable authority for native stability campaigns
// isolated under the Git common directory.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
	"modernc.org/sqlite"
)

// GitRunner matches worktree.GitRunner interface for running git commands.
type GitRunner = worktree.GitRunner

// Relative stability store path under git common directory.
const stabilityStoreRelPath = "lucind-ai/stability/v1/stability.db"

const (
	wantJournalMode = "wal"
	wantBusyTimeout = 5000
)

// Status represents the lifecycle status of a stability campaign.
type Status string

const (
	StatusRunning        Status = "running"
	StatusFailed         Status = "failed"
	StatusBlockedCleanup Status = "blocked_cleanup"
	StatusPassed         Status = "passed"
)

// Errors returned by Store.
var (
	ErrPragmaNotApplied      = errors.New("stability/store: required SQLite pragma was not applied")
	ErrCampaignActive        = errors.New("stability/store: an active campaign is already in progress")
	ErrCampaignNotFound      = errors.New("stability/store: campaign not found")
	ErrTrialProgressNotFound = errors.New("stability/store: trial progress not found")
	ErrInvalidStatus         = errors.New("stability/store: invalid campaign status")
)

// Campaign represents a persisted stability campaign record.
type Campaign struct {
	ID           string     `json:"id"`
	CandidateSHA string     `json:"candidate_sha"`
	Status       Status     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	ClosedAt     *time.Time `json:"closed_at,omitempty"`
}

// TrialProgress represents a persisted trial-progress record.
type TrialProgress struct {
	CampaignID  string    `json:"campaign_id"`
	TrialNumber int       `json:"trial_number"`
	Stage       string    `json:"stage"`
	PGIDA       *int      `json:"pgid_a,omitempty"`
	PGIDB       *int      `json:"pgid_b,omitempty"`
	PGIDFix     *int      `json:"pgid_fix,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Store represents a SQLite-backed stability campaign authority store.
type Store struct {
	db *sql.DB
}

// DB returns the underlying sql.DB connection pool.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close closes the underlying database pool.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// ResolvePath resolves the authority store path under the repository's git common directory.
func ResolvePath(ctx context.Context, runner GitRunner, repoDir string) (string, error) {
	if runner == nil {
		runner = worktree.DefaultGitRunner
	}
	out, err := runner.Run(ctx, repoDir, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("stability/store: resolve git common dir: %w", err)
	}

	commonDir := strings.TrimSpace(string(out))
	if commonDir == "" {
		return "", errors.New("stability/store: empty git common dir")
	}

	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(repoDir, commonDir)
	}
	commonDir = filepath.Clean(commonDir)

	return filepath.Join(commonDir, filepath.FromSlash(stabilityStoreRelPath)), nil
}

// Open resolves the stability store path under the repository root's git common directory
// and opens the SQLite authority store.
func Open(ctx context.Context, runner GitRunner, repoDir string) (*Store, error) {
	dbPath, err := ResolvePath(ctx, runner, repoDir)
	if err != nil {
		return nil, err
	}
	return OpenAtPath(ctx, dbPath)
}

// OpenAtPath opens the SQLite authority store at an explicit filesystem path.
func OpenAtPath(ctx context.Context, dbPath string) (*Store, error) {
	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("stability/store: create parent directory: %w", err)
		}
	}

	dsn := "file:" + dbPath +
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("stability/store: open: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("stability/store: ping: %w", err)
	}

	if err := checkPragmas(ctx, db); err != nil {
		db.Close()
		return nil, err
	}

	// >1 so the concurrency guarantee this package provides is actually exercised.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(0)

	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("stability/store: migrate: %w", err)
	}

	return &Store{db: db}, nil
}

func checkPragmas(ctx context.Context, db *sql.DB) error {
	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		return fmt.Errorf("stability/store: read journal_mode pragma: %w", err)
	}
	if strings.ToLower(journalMode) != wantJournalMode {
		return fmt.Errorf("%w: journal_mode=%s, want %s", ErrPragmaNotApplied, journalMode, wantJournalMode)
	}

	var busyTimeout int
	if err := db.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		return fmt.Errorf("stability/store: read busy_timeout pragma: %w", err)
	}
	if busyTimeout != wantBusyTimeout {
		return fmt.Errorf("%w: busy_timeout=%d, want %d", ErrPragmaNotApplied, busyTimeout, wantBusyTimeout)
	}

	return nil
}

const schemaDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS campaigns (
  id TEXT PRIMARY KEY,
  candidate_sha TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('running', 'failed', 'blocked_cleanup', 'passed')),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  closed_at TEXT
) STRICT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_campaigns_single_active ON campaigns(status) WHERE status = 'running';

CREATE TABLE IF NOT EXISTS trial_progress (
  campaign_id TEXT NOT NULL,
  trial_number INTEGER NOT NULL,
  stage TEXT NOT NULL,
  pgid_a INTEGER,
  pgid_b INTEGER,
  pgid_fix INTEGER,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (campaign_id, trial_number)
) STRICT;
`

func migrate(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, schemaDDL); err != nil {
		return err
	}

	var currentVersion int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&currentVersion); err != nil {
		return err
	}

	if currentVersion < 1 {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, 1, now); err != nil {
			return err
		}
	}

	if currentVersion < 2 {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, 2, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func isConstraintViolation(err error) bool {
	if err == nil {
		return false
	}
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		// SQLITE_CONSTRAINT (19) or primary/unique constraints
		if sqliteErr.Code()&0xff == 19 {
			return true
		}
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "constraint failed")
}

// CreateCampaign initializes a new active campaign record in "running" status.
// If an unclosed campaign is already running, ErrCampaignActive is returned.
func (s *Store) CreateCampaign(ctx context.Context, id, candidateSHA string) (Campaign, error) {
	if strings.TrimSpace(id) == "" {
		return Campaign{}, errors.New("stability/store: campaign ID must not be empty")
	}
	if strings.TrimSpace(candidateSHA) == "" {
		return Campaign{}, errors.New("stability/store: candidate SHA must not be empty")
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339Nano)

	// Conditional insert: ensure no unclosed campaign with status = 'running' exists
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO campaigns (id, candidate_sha, status, created_at, updated_at)
		SELECT ?, ?, 'running', ?, ?
		WHERE NOT EXISTS (SELECT 1 FROM campaigns WHERE status = 'running')`,
		id, candidateSHA, nowStr, nowStr,
	)
	if err != nil {
		if isConstraintViolation(err) {
			return Campaign{}, ErrCampaignActive
		}
		return Campaign{}, fmt.Errorf("stability/store: create campaign: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return Campaign{}, fmt.Errorf("stability/store: rows affected: %w", err)
	}
	if affected == 0 {
		return Campaign{}, ErrCampaignActive
	}

	return Campaign{
		ID:           id,
		CandidateSHA: candidateSHA,
		Status:       StatusRunning,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// GetCampaign fetches a campaign record by ID.
func (s *Store) GetCampaign(ctx context.Context, id string) (Campaign, error) {
	var (
		c         Campaign
		statusStr string
		createdAt string
		updatedAt string
		closedAt  sql.NullString
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT id, candidate_sha, status, created_at, updated_at, closed_at
		FROM campaigns WHERE id = ?`, id,
	).Scan(&c.ID, &c.CandidateSHA, &statusStr, &createdAt, &updatedAt, &closedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Campaign{}, ErrCampaignNotFound
	}
	if err != nil {
		return Campaign{}, fmt.Errorf("stability/store: get campaign: %w", err)
	}

	c.Status = Status(statusStr)
	c.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	c.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	if closedAt.Valid && closedAt.String != "" {
		t, err := time.Parse(time.RFC3339Nano, closedAt.String)
		if err == nil {
			c.ClosedAt = &t
		}
	}
	return c, nil
}

// GetActiveCampaign returns the currently running campaign, or ErrCampaignNotFound if none is active.
func (s *Store) GetActiveCampaign(ctx context.Context) (Campaign, error) {
	var (
		c         Campaign
		statusStr string
		createdAt string
		updatedAt string
		closedAt  sql.NullString
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT id, candidate_sha, status, created_at, updated_at, closed_at
		FROM campaigns WHERE status = 'running' LIMIT 1`,
	).Scan(&c.ID, &c.CandidateSHA, &statusStr, &createdAt, &updatedAt, &closedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Campaign{}, ErrCampaignNotFound
	}
	if err != nil {
		return Campaign{}, fmt.Errorf("stability/store: get active campaign: %w", err)
	}

	c.Status = Status(statusStr)
	c.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	c.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	if closedAt.Valid && closedAt.String != "" {
		t, err := time.Parse(time.RFC3339Nano, closedAt.String)
		if err == nil {
			c.ClosedAt = &t
		}
	}
	return c, nil
}

// UpdateCampaignStatus updates the status of a campaign record. If transitioning
// to a terminal status (failed, blocked_cleanup, passed), closed_at is set.
func (s *Store) UpdateCampaignStatus(ctx context.Context, id string, status Status) error {
	switch status {
	case StatusRunning, StatusFailed, StatusBlockedCleanup, StatusPassed:
	default:
		return ErrInvalidStatus
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339Nano)

	var closedAtStr sql.NullString
	if status != StatusRunning {
		closedAtStr = sql.NullString{String: nowStr, Valid: true}
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE campaigns
		SET status = ?, updated_at = ?, closed_at = ?
		WHERE id = ?`,
		string(status), nowStr, closedAtStr, id,
	)
	if err != nil {
		if isConstraintViolation(err) {
			return ErrCampaignActive
		}
		return fmt.Errorf("stability/store: update campaign status: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("stability/store: rows affected: %w", err)
	}
	if affected == 0 {
		return ErrCampaignNotFound
	}
	return nil
}

// UpsertTrialStage inserts or updates the active stage for a campaign trial.
func (s *Store) UpsertTrialStage(ctx context.Context, campaignID string, trialNumber int, stage string) error {
	if strings.TrimSpace(campaignID) == "" {
		return errors.New("stability/store: campaign ID must not be empty")
	}
	if trialNumber <= 0 {
		return errors.New("stability/store: trial number must be > 0")
	}
	if strings.TrimSpace(stage) == "" {
		return errors.New("stability/store: stage must not be empty")
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO trial_progress (campaign_id, trial_number, stage, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(campaign_id, trial_number) DO UPDATE SET stage = excluded.stage, updated_at = excluded.updated_at`,
		campaignID, trialNumber, stage, now, now,
	)
	if err != nil {
		return fmt.Errorf("stability/store: upsert trial stage: %w", err)
	}
	return nil
}

// UpdateTrialPGID inserts or updates the process group ID for a specific lane in a trial.
// lane must be "a", "b", or "fix".
func (s *Store) UpdateTrialPGID(ctx context.Context, campaignID string, trialNumber int, lane string, pgid int) error {
	if strings.TrimSpace(campaignID) == "" {
		return errors.New("stability/store: campaign ID must not be empty")
	}
	if trialNumber <= 0 {
		return errors.New("stability/store: trial number must be > 0")
	}

	var col string
	switch lane {
	case "a":
		col = "pgid_a"
	case "b":
		col = "pgid_b"
	case "fix":
		col = "pgid_fix"
	default:
		return fmt.Errorf("stability/store: invalid lane %q (must be 'a', 'b', or 'fix')", lane)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	query := fmt.Sprintf(`
		INSERT INTO trial_progress (campaign_id, trial_number, stage, %s, created_at, updated_at)
		VALUES (?, ?, '', ?, ?, ?)
		ON CONFLICT(campaign_id, trial_number) DO UPDATE SET %s = excluded.%s, updated_at = excluded.updated_at`,
		col, col, col,
	)
	_, err := s.db.ExecContext(ctx, query, campaignID, trialNumber, pgid, now, now)
	if err != nil {
		return fmt.Errorf("stability/store: update trial pgid: %w", err)
	}
	return nil
}

// GetTrialProgress returns the trial progress record with the highest trial number for the campaign.
// If no record exists, ErrTrialProgressNotFound is returned.
func (s *Store) GetTrialProgress(ctx context.Context, campaignID string) (TrialProgress, error) {
	var (
		tp        TrialProgress
		pgidA     sql.NullInt64
		pgidB     sql.NullInt64
		pgidFix   sql.NullInt64
		createdAt string
		updatedAt string
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT campaign_id, trial_number, stage, pgid_a, pgid_b, pgid_fix, created_at, updated_at
		FROM trial_progress
		WHERE campaign_id = ?
		ORDER BY trial_number DESC
		LIMIT 1`, campaignID,
	).Scan(&tp.CampaignID, &tp.TrialNumber, &tp.Stage, &pgidA, &pgidB, &pgidFix, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return TrialProgress{}, ErrTrialProgressNotFound
	}
	if err != nil {
		return TrialProgress{}, fmt.Errorf("stability/store: get trial progress: %w", err)
	}

	if pgidA.Valid {
		v := int(pgidA.Int64)
		tp.PGIDA = &v
	}
	if pgidB.Valid {
		v := int(pgidB.Int64)
		tp.PGIDB = &v
	}
	if pgidFix.Valid {
		v := int(pgidFix.Int64)
		tp.PGIDFix = &v
	}
	tp.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	tp.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return tp, nil
}
