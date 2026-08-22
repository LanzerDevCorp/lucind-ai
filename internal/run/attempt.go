package run

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/feature"
	"github.com/LanzerDevCorp/lucind-ai/internal/integrate"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/overlap"
	"github.com/LanzerDevCorp/lucind-ai/internal/reconcile"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

// Sentinel errors returned by attempt operations.
var (
	ErrAttemptIDRequired      = errors.New("run: attempt id is required")
	ErrAttemptNotFound        = errors.New("run: attempt not found")
	ErrIdempotencyKeyRequired = errors.New("run: idempotency key is required")
	ErrRefMismatch            = errors.New("run: parent ref sha mismatch during recovery")
)

// AttemptStatus represents the lifecycle state of a durable integration attempt.
type AttemptStatus string

const (
	AttemptStatusRecorded   AttemptStatus = "recorded"
	AttemptStatusLeased     AttemptStatus = "leased"
	AttemptStatusCombining  AttemptStatus = "combining"
	AttemptStatusChecking   AttemptStatus = "checking"
	AttemptStatusCASPending AttemptStatus = "cas_pending"
	AttemptStatusPromoted   AttemptStatus = "promoted"
	AttemptStatusBlocked    AttemptStatus = "blocked"
	AttemptStatusFailed     AttemptStatus = "failed"
	AttemptStatusStale      AttemptStatus = "stale"
)

// Valid reports whether s is a valid attempt status in the schema.
func (s AttemptStatus) Valid() bool {
	switch s {
	case AttemptStatusRecorded, AttemptStatusLeased, AttemptStatusCombining,
		AttemptStatusChecking, AttemptStatusCASPending, AttemptStatusPromoted,
		AttemptStatusBlocked, AttemptStatusFailed, AttemptStatusStale:
		return true
	default:
		return false
	}
}

// Terminal reports whether s is a terminal attempt status.
func (s AttemptStatus) Terminal() bool {
	switch s {
	case AttemptStatusPromoted, AttemptStatusBlocked, AttemptStatusFailed, AttemptStatusStale:
		return true
	default:
		return false
	}
}

// Attempt represents one row of the integration_attempts table: a durable record
// of an attempt to integrate and promote changes for a feature onto a parent ref.
type Attempt struct {
	ID             string        `json:"id"`
	FeatureID      string        `json:"feature_id"`
	IdempotencyKey string        `json:"idempotency_key"`
	Status         AttemptStatus `json:"status"`
	Owner          string        `json:"owner"`
	Fence          int64         `json:"fence"`
	CandidateSHA   string        `json:"candidate_sha"`
	FailureReason  string        `json:"failure_reason"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

// AttemptRequest carries inputs required to start or resolve an integration attempt.
type AttemptRequest struct {
	ID                string
	FeatureID         string
	ParentRef         string
	BaseSHA           string
	ExpectedParentSHA string
	IdempotencyKey    string
	Owner             string
	Branches          []string
}

const timestampFormat = "2006-01-02T15:04:05.000000000Z"

func formatAttemptTimestamp(t time.Time) string {
	return t.UTC().Format(timestampFormat)
}

func parseAttemptTimestamp(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

func updateNow(deps Deps) time.Time {
	if deps.Now != nil {
		return deps.Now().UTC()
	}
	return time.Now().UTC()
}

// GetAttempt retrieves an attempt row by attempt ID from the ledger.
func GetAttempt(ctx context.Context, l *ledger.Ledger, attemptID string) (Attempt, error) {
	return getAttempt(ctx, l.DB(), attemptID)
}

// GetAttemptByIdempotencyKey retrieves an attempt row by (feature_id, idempotency_key).
func GetAttemptByIdempotencyKey(ctx context.Context, l *ledger.Ledger, featureID, idempotencyKey string) (Attempt, error) {
	return getAttemptByIdempotencyKey(ctx, l.DB(), featureID, idempotencyKey)
}

func getAttempt(ctx context.Context, db *sql.DB, id string) (Attempt, error) {
	var (
		att       Attempt
		status    string
		createdAt string
		updatedAt string
	)
	err := db.QueryRowContext(ctx, `
		SELECT id, feature_id, idempotency_key, status, owner, fence, candidate_sha, failure_reason, created_at, updated_at
		FROM integration_attempts WHERE id = ?`, id,
	).Scan(
		&att.ID, &att.FeatureID, &att.IdempotencyKey, &status, &att.Owner, &att.Fence,
		&att.CandidateSHA, &att.FailureReason, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Attempt{}, ErrAttemptNotFound
		}
		return Attempt{}, fmt.Errorf("run: query attempt %q: %w", id, err)
	}
	att.Status = AttemptStatus(status)
	att.CreatedAt, _ = parseAttemptTimestamp(createdAt)
	att.UpdatedAt, _ = parseAttemptTimestamp(updatedAt)
	return att, nil
}

func getAttemptByIdempotencyKey(ctx context.Context, db *sql.DB, featureID, idempotencyKey string) (Attempt, error) {
	var (
		att       Attempt
		status    string
		createdAt string
		updatedAt string
	)
	err := db.QueryRowContext(ctx, `
		SELECT id, feature_id, idempotency_key, status, owner, fence, candidate_sha, failure_reason, created_at, updated_at
		FROM integration_attempts WHERE feature_id = ? AND idempotency_key = ?`, featureID, idempotencyKey,
	).Scan(
		&att.ID, &att.FeatureID, &att.IdempotencyKey, &status, &att.Owner, &att.Fence,
		&att.CandidateSHA, &att.FailureReason, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Attempt{}, ErrAttemptNotFound
		}
		return Attempt{}, fmt.Errorf("run: query attempt by idempotency key (%q, %q): %w", featureID, idempotencyKey, err)
	}
	att.Status = AttemptStatus(status)
	att.CreatedAt, _ = parseAttemptTimestamp(createdAt)
	att.UpdatedAt, _ = parseAttemptTimestamp(updatedAt)
	return att, nil
}

func insertAttemptWithAudit(ctx context.Context, l *ledger.Ledger, att Attempt, evtDetail string) error {
	nowStr := formatAttemptTimestamp(att.CreatedAt)
	updStr := formatAttemptTimestamp(att.UpdatedAt)
	return l.WriteWithAudit(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO integration_attempts (id, feature_id, idempotency_key, status, owner, fence, candidate_sha, failure_reason, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			att.ID, att.FeatureID, att.IdempotencyKey, string(att.Status), att.Owner, att.Fence,
			att.CandidateSHA, att.FailureReason, nowStr, updStr,
		)
		return err
	}, ledger.IntegrationEvent{
		FeatureID: att.FeatureID,
		AttemptID: att.ID,
		Type:      "attempt_recorded",
		Detail:    evtDetail,
		At:        att.CreatedAt,
	})
}

func updateAttemptWithAudit(ctx context.Context, l *ledger.Ledger, att Attempt, evtType, evtDetail string) error {
	updStr := formatAttemptTimestamp(att.UpdatedAt)
	return l.WriteWithAudit(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			UPDATE integration_attempts
			SET status = ?, owner = ?, fence = ?, candidate_sha = ?, failure_reason = ?, updated_at = ?
			WHERE id = ?`,
			string(att.Status), att.Owner, att.Fence, att.CandidateSHA, att.FailureReason, updStr, att.ID,
		)
		return err
	}, ledger.IntegrationEvent{
		FeatureID: att.FeatureID,
		AttemptID: att.ID,
		Type:      evtType,
		Detail:    evtDetail,
		At:        att.UpdatedAt,
	})
}

// ExecuteAttempt runs the durable attempt state machine:
// recorded -> leased -> combining -> checking -> cas_pending -> promoted.
// Any non-terminal state may become blocked, failed, or stale.
// Terminal replays return the stored attempt result without invoking any side effects.
func ExecuteAttempt(ctx context.Context, deps Deps, req AttemptRequest) (Attempt, error) {
	if strings.TrimSpace(req.ID) == "" {
		return Attempt{}, ErrAttemptIDRequired
	}
	if strings.TrimSpace(req.FeatureID) == "" {
		return Attempt{}, ErrMissingFeatureTarget
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return Attempt{}, ErrIdempotencyKeyRequired
	}
	if strings.TrimSpace(req.Owner) == "" {
		req.Owner = "run"
	}

	now := updateNow(deps)

	// Check if already exists by ID (replay)
	existing, err := getAttempt(ctx, deps.Ledger.DB(), req.ID)
	if err == nil {
		if existing.Status.Terminal() {
			return existing, nil
		}
		// Non-terminal: recover/resume
		return recoverAttemptInternal(ctx, deps, existing)
	} else if !errors.Is(err, ErrAttemptNotFound) {
		return Attempt{}, err
	}

	// Check if already exists by (feature_id, idempotency_key) (idempotent duplication resolution)
	existingKey, err := getAttemptByIdempotencyKey(ctx, deps.Ledger.DB(), req.FeatureID, req.IdempotencyKey)
	if err == nil {
		if existingKey.Status.Terminal() {
			return existingKey, nil
		}
		// Non-terminal: recover/resume
		return recoverAttemptInternal(ctx, deps, existingKey)
	} else if !errors.Is(err, ErrAttemptNotFound) {
		return Attempt{}, err
	}

	// Resolve feature
	featSvc := feature.NewService(deps.Ledger)
	feat, err := featSvc.Get(ctx, req.FeatureID)
	if errors.Is(err, feature.ErrFeatureNotFound) {
		if req.ParentRef != "" && req.BaseSHA != "" {
			feat, err = featSvc.Create(ctx, req.FeatureID, req.ParentRef, req.BaseSHA, req.ExpectedParentSHA)
			if err != nil {
				return Attempt{}, fmt.Errorf("run: create feature %q: %w", req.FeatureID, err)
			}
		} else {
			return Attempt{}, fmt.Errorf("run: feature %q not found and missing parent_ref/base_sha: %w", req.FeatureID, err)
		}
	} else if err != nil {
		return Attempt{}, fmt.Errorf("run: get feature %q: %w", req.FeatureID, err)
	}

	if req.ParentRef == "" {
		req.ParentRef = feat.ParentRef
	}
	if req.BaseSHA == "" {
		req.BaseSHA = feat.BaseSHA
	}
	if req.ExpectedParentSHA == "" {
		req.ExpectedParentSHA = feat.ExpectedParentSHA
	}

	// 1. RECORDED
	att := Attempt{
		ID:             req.ID,
		FeatureID:      req.FeatureID,
		IdempotencyKey: req.IdempotencyKey,
		Status:         AttemptStatusRecorded,
		Owner:          req.Owner,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := insertAttemptWithAudit(ctx, deps.Ledger, att, fmt.Sprintf("parent_ref=%s base_sha=%s expected_parent_sha=%s", req.ParentRef, req.BaseSHA, req.ExpectedParentSHA)); err != nil {
		return Attempt{}, fmt.Errorf("run: record attempt: %w", err)
	}

	// 2. LEASE ACQUISITION -> LEASED
	leaseTTL := deps.FeatureLeaseTTL
	if leaseTTL <= 0 {
		leaseTTL = 30 * time.Second
	}
	lease, err := featSvc.AcquireLease(ctx, req.FeatureID, req.Owner, leaseTTL)
	if err != nil {
		now = updateNow(deps)
		att.UpdatedAt = now
		if errors.Is(err, feature.ErrLeaseHeld) {
			att.Status = AttemptStatusBlocked
			att.FailureReason = fmt.Sprintf("lease held by another owner: %v", err)
			_ = updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_blocked", att.FailureReason)
			return att, nil
		}
		att.Status = AttemptStatusFailed
		att.FailureReason = fmt.Sprintf("lease acquisition failed: %v", err)
		_ = updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_failed", att.FailureReason)
		return att, nil
	}

	now = updateNow(deps)
	att.Status = AttemptStatusLeased
	att.Fence = lease.Fence
	att.Owner = lease.Owner
	att.UpdatedAt = now
	if err := updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_leased", fmt.Sprintf("fence=%d owner=%s", att.Fence, att.Owner)); err != nil {
		return att, fmt.Errorf("run: update leased status: %w", err)
	}

	return driveAttemptFromLeased(ctx, deps, att, featSvc, req)
}

func driveAttemptFromLeased(ctx context.Context, deps Deps, att Attempt, featSvc *feature.Service, req AttemptRequest) (Attempt, error) {
	// 3. COMBINING
	now := updateNow(deps)
	att.Status = AttemptStatusCombining
	att.UpdatedAt = now
	if err := updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_combining", fmt.Sprintf("branches=%v", req.Branches)); err != nil {
		return att, fmt.Errorf("run: update combining status: %w", err)
	}

	if err := featSvc.ValidateLease(ctx, att.FeatureID, att.Owner, att.Fence); err != nil {
		now = updateNow(deps)
		att.Status = AttemptStatusStale
		att.FailureReason = fmt.Sprintf("lease lost before combine: %v", err)
		att.UpdatedAt = now
		_ = updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_stale", att.FailureReason)
		return att, nil
	}

	combineFunc := deps.CombineTree
	if combineFunc == nil {
		combineFunc = integrate.Combine
	}
	wtPath, branchName, err := combineFunc(ctx, deps.PrimaryRoot, req.ID, req.Branches)
	if err != nil {
		now = updateNow(deps)
		att.Status = AttemptStatusFailed
		att.FailureReason = fmt.Sprintf("combine failed: %v", err)
		att.UpdatedAt = now
		_ = updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_failed", att.FailureReason)
		return att, nil
	}

	// 4. CHECKING
	now = updateNow(deps)
	att.Status = AttemptStatusChecking
	att.UpdatedAt = now
	if err := updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_checking", fmt.Sprintf("worktree=%s", wtPath)); err != nil {
		return att, fmt.Errorf("run: update checking status: %w", err)
	}

	if err := featSvc.ValidateLease(ctx, att.FeatureID, att.Owner, att.Fence); err != nil {
		now = updateNow(deps)
		att.Status = AttemptStatusStale
		att.FailureReason = fmt.Sprintf("lease lost before checks: %v", err)
		att.UpdatedAt = now
		_ = updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_stale", att.FailureReason)
		return att, nil
	}

	checkFunc := deps.RunChecks
	if checkFunc == nil {
		checkFunc = integrate.Check
	}
	passed, output, err := checkFunc(ctx, wtPath)
	if err != nil || !passed {
		reason := output
		if err != nil {
			reason = err.Error()
		}
		if reason == "" {
			reason = "verification checks failed"
		}
		now = updateNow(deps)
		att.Status = AttemptStatusFailed
		att.FailureReason = fmt.Sprintf("checks failed: %s", reason)
		att.UpdatedAt = now
		_ = updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_failed", att.FailureReason)
		return att, nil
	}

	// 5. CAS PENDING
	var candidateSHA string
	if deps.ResolveCandidateSHA != nil {
		candidateSHA, err = deps.ResolveCandidateSHA(ctx, deps.PrimaryRoot, wtPath, branchName)
	} else if deps.GitRunner != nil {
		out, rErr := deps.GitRunner.Run(ctx, wtPath, "rev-parse", "HEAD")
		if rErr == nil {
			candidateSHA = strings.TrimSpace(string(out))
		}
	}
	if candidateSHA == "" {
		candidateSHA = branchName
	}

	now = updateNow(deps)
	att.Status = AttemptStatusCASPending
	att.CandidateSHA = candidateSHA
	att.UpdatedAt = now
	if err := updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_cas_pending", fmt.Sprintf("candidate_sha=%s", candidateSHA)); err != nil {
		return att, fmt.Errorf("run: update cas_pending status: %w", err)
	}

	if err := featSvc.ValidateLease(ctx, att.FeatureID, att.Owner, att.Fence); err != nil {
		now = updateNow(deps)
		att.Status = AttemptStatusStale
		att.FailureReason = fmt.Sprintf("lease lost before CAS: %v", err)
		att.UpdatedAt = now
		_ = updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_stale", att.FailureReason)
		return att, nil
	}

	// 5b. REQUIRED OVERLAP PROMOTION GATE
	blocked, gatedAtt, err := evaluateOverlapGate(ctx, deps, att, featSvc, req.ParentRef, req.BaseSHA)
	if err != nil {
		return att, fmt.Errorf("run: evaluate overlap gate: %w", err)
	}
	if blocked {
		return gatedAtt, nil
	}

	// gatedAtt, not att: a cleared required-overlap block (a human-resolved reconciliation
	// candidate) overrides CandidateSHA inside the gate to the resolved commit -- promoting att
	// unchanged here would silently discard that override and promote this attempt's own raw
	// combined tree instead.
	// 6. CAS PROMOTION
	return performCASPromotion(ctx, deps, gatedAtt, featSvc, req.ParentRef, req.ExpectedParentSHA, wtPath, branchName)
}

func performCASPromotion(ctx context.Context, deps Deps, att Attempt, featSvc *feature.Service, parentRef, expectedParentSHA, wtPath, branchName string) (Attempt, error) {
	// Verify current ref matches expected SHA before calling PromoteCAS
	var currentSHA string
	var err error
	if deps.ResolveRefSHA != nil {
		currentSHA, err = deps.ResolveRefSHA(ctx, deps.PrimaryRoot, parentRef)
	} else if deps.GitRunner != nil {
		canonicalRef := worktree.CanonicalizeRef(parentRef)
		out, rErr := deps.GitRunner.Run(ctx, deps.PrimaryRoot, "rev-parse", "--verify", canonicalRef+"^{commit}")
		if rErr == nil {
			currentSHA = strings.TrimSpace(string(out))
		} else {
			err = rErr
		}
	} else {
		currentSHA, err = worktree.ResolveCommitSHA(ctx, worktree.DefaultGitRunner, deps.PrimaryRoot, parentRef)
	}

	if err != nil || currentSHA != expectedParentSHA {
		now := updateNow(deps)
		att.Status = AttemptStatusStale
		att.FailureReason = fmt.Sprintf("stale parent ref before CAS: expected %s, got %s", expectedParentSHA, currentSHA)
		att.UpdatedAt = now
		_ = updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_stale", att.FailureReason)
		return att, nil
	}

	promoteCASFunc := deps.PromoteCAS
	if promoteCASFunc == nil {
		promoteCASFunc = integrate.PromoteCAS
	}

	if err := promoteCASFunc(ctx, deps.PrimaryRoot, parentRef, att.CandidateSHA, expectedParentSHA); err != nil {
		now := updateNow(deps)
		if errors.Is(err, integrate.ErrStaleCAS) {
			att.Status = AttemptStatusStale
		} else {
			att.Status = AttemptStatusFailed
		}
		att.FailureReason = fmt.Sprintf("promotion CAS failed: %v", err)
		att.UpdatedAt = now
		_ = updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_failed", att.FailureReason)
		return att, nil
	}

	// CAS Succeeded!
	now := updateNow(deps)
	att.Status = AttemptStatusPromoted
	att.UpdatedAt = now
	if err := updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_promoted", fmt.Sprintf("parent_ref=%s candidate_sha=%s", parentRef, att.CandidateSHA)); err != nil {
		return att, fmt.Errorf("run: update promoted status: %w", err)
	}

	// Release lease
	_ = featSvc.ReleaseLease(ctx, att.FeatureID, att.Owner, att.Fence)

	// Clean up combined worktree only after committed promotion
	if deps.DiscardCombined != nil && wtPath != "" {
		_ = deps.DiscardCombined(ctx, deps.PrimaryRoot, wtPath, branchName)
	}

	return att, nil
}

// RecoverAttempt recovers an interrupted or crashed attempt.
// It verifies recorded expected vs. current refs before resuming,
// finalizing CAS if it went through or executing CAS if ref matches,
// and fails closed (blocked) preserving all worktrees/artifacts on mismatch.
func RecoverAttempt(ctx context.Context, deps Deps, attemptID string) (Attempt, error) {
	if strings.TrimSpace(attemptID) == "" {
		return Attempt{}, ErrAttemptIDRequired
	}

	att, err := getAttempt(ctx, deps.Ledger.DB(), attemptID)
	if err != nil {
		return Attempt{}, err
	}

	if att.Status.Terminal() {
		return att, nil
	}

	return recoverAttemptInternal(ctx, deps, att)
}

func recoverAttemptInternal(ctx context.Context, deps Deps, att Attempt) (Attempt, error) {
	featSvc := feature.NewService(deps.Ledger)
	feat, err := featSvc.Get(ctx, att.FeatureID)
	if err != nil {
		return Attempt{}, fmt.Errorf("run: recover attempt get feature %q: %w", att.FeatureID, err)
	}

	// Check current ref
	var currentSHA string
	if deps.ResolveRefSHA != nil {
		currentSHA, err = deps.ResolveRefSHA(ctx, deps.PrimaryRoot, feat.ParentRef)
	} else if deps.GitRunner != nil {
		canonicalRef := worktree.CanonicalizeRef(feat.ParentRef)
		out, rErr := deps.GitRunner.Run(ctx, deps.PrimaryRoot, "rev-parse", "--verify", canonicalRef+"^{commit}")
		if rErr == nil {
			currentSHA = strings.TrimSpace(string(out))
		} else {
			err = rErr
		}
	} else {
		currentSHA, err = worktree.ResolveCommitSHA(ctx, worktree.DefaultGitRunner, deps.PrimaryRoot, feat.ParentRef)
	}

	// Scenario 1: Post-CAS recovery. If status was cas_pending and ref is already candidate_sha,
	// CAS succeeded before crash. Finalize to promoted without second promotion!
	if att.Status == AttemptStatusCASPending && att.CandidateSHA != "" && currentSHA == att.CandidateSHA {
		now := updateNow(deps)
		att.Status = AttemptStatusPromoted
		att.UpdatedAt = now
		if err := updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_promoted", fmt.Sprintf("post-crash finalization: parent_ref=%s candidate_sha=%s", feat.ParentRef, att.CandidateSHA)); err != nil {
			return att, fmt.Errorf("run: finalize recovery promoted: %w", err)
		}
		_ = featSvc.ReleaseLease(ctx, feat.ID, att.Owner, att.Fence)
		return att, nil
	}

	// Scenario 2: Ref still matches expected parent SHA
	if err == nil && currentSHA == feat.ExpectedParentSHA {
		// Re-acquire / validate lease
		leaseTTL := deps.FeatureLeaseTTL
		if leaseTTL <= 0 {
			leaseTTL = 30 * time.Second
		}
		lease, lErr := featSvc.AcquireLease(ctx, feat.ID, att.Owner, leaseTTL)
		if lErr != nil {
			now := updateNow(deps)
			att.Status = AttemptStatusBlocked
			att.FailureReason = fmt.Sprintf("recovery blocked: lease acquisition failed: %v", lErr)
			att.UpdatedAt = now
			_ = updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_blocked", att.FailureReason)
			return att, nil
		}
		att.Fence = lease.Fence
		att.Owner = lease.Owner

		if att.Status == AttemptStatusCASPending {
			// CAS definitely never ran! Check gate and execute CAS
			blocked, gatedAtt, err := evaluateOverlapGate(ctx, deps, att, featSvc, feat.ParentRef, feat.BaseSHA)
			if err != nil {
				return att, fmt.Errorf("run: recovery evaluate overlap gate: %w", err)
			}
			if blocked {
				return gatedAtt, nil
			}
			// gatedAtt, not att -- see the same note in driveAttemptFromLeased.
			return performCASPromotion(ctx, deps, gatedAtt, featSvc, feat.ParentRef, feat.ExpectedParentSHA, "", "")
		}

		// Replay/drive from recorded inputs
		req := AttemptRequest{
			ID:                att.ID,
			FeatureID:         att.FeatureID,
			ParentRef:         feat.ParentRef,
			BaseSHA:           feat.BaseSHA,
			ExpectedParentSHA: feat.ExpectedParentSHA,
			IdempotencyKey:    att.IdempotencyKey,
			Owner:             att.Owner,
			Branches:          []string{worktree.BranchFor(att.ID)},
		}
		return driveAttemptFromLeased(ctx, deps, att, featSvc, req)
	}

	// Scenario 3: Ref mismatch or missing ref! Fail closed, remain blocked, preserve all artifacts.
	now := updateNow(deps)
	att.Status = AttemptStatusBlocked
	att.FailureReason = fmt.Sprintf("recovery ref mismatch: expected %s, got %s", feat.ExpectedParentSHA, currentSHA)
	att.UpdatedAt = now
	_ = updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_blocked", att.FailureReason)
	return att, nil
}

// evaluateOverlapGate evaluates deterministic overlap signals between the current feature attempt
// and all other active features. Required overlap creates an awaiting reconciliation request (if none exists)
// and blocks promotion; warning overlap records evidence without blocking; informational overlap is a no-op.
func evaluateOverlapGate(ctx context.Context, deps Deps, att Attempt, featSvc *feature.Service, parentRef, baseSHA string) (bool, Attempt, error) {
	activeFeatures, err := deps.Ledger.ActiveFeatures(ctx)
	if err != nil {
		return false, att, fmt.Errorf("query active features for overlap gate: %w", err)
	}

	evalFunc := deps.EvaluateOverlap
	if evalFunc == nil {
		evalFunc = overlap.Evaluate
	}

	reconcileSvc := reconcile.NewService(deps.Ledger, reconcile.WithClock(func() time.Time { return updateNow(deps) }))

	for _, otherFeat := range activeFeatures {
		if otherFeat.ID == att.FeatureID {
			continue
		}

		var otherSHA string
		if deps.ResolveRefSHA != nil {
			otherSHA, _ = deps.ResolveRefSHA(ctx, deps.PrimaryRoot, otherFeat.ParentRef)
		} else if deps.GitRunner != nil {
			canonicalRef := worktree.CanonicalizeRef(otherFeat.ParentRef)
			out, rErr := deps.GitRunner.Run(ctx, deps.PrimaryRoot, "rev-parse", "--verify", canonicalRef+"^{commit}")
			if rErr == nil {
				otherSHA = strings.TrimSpace(string(out))
			}
		} else {
			otherSHA, _ = worktree.ResolveCommitSHA(ctx, worktree.DefaultGitRunner, deps.PrimaryRoot, otherFeat.ParentRef)
		}
		if otherSHA == "" {
			otherSHA = otherFeat.ExpectedParentSHA
		}
		if otherSHA == "" {
			otherSHA = otherFeat.BaseSHA
		}
		if otherSHA == "" {
			continue
		}

		commonBase := ""
		if baseSHA != "" && baseSHA == otherFeat.BaseSHA {
			commonBase = baseSHA
		}

		ev, err := evalFunc(ctx, deps.PrimaryRoot, commonBase, att.CandidateSHA, otherSHA)
		if err != nil {
			if errors.Is(err, overlap.ErrNoMergeBase) {
				continue
			}
			return false, att, fmt.Errorf("evaluate overlap between %s and %s: %w", att.FeatureID, otherFeat.ID, err)
		}
		if ev == nil {
			continue
		}

		evJSON, err := ev.JSON()
		if err != nil {
			return false, att, fmt.Errorf("marshal overlap evidence: %w", err)
		}
		evHash := ev.Hash
		if evHash == "" {
			evHash, err = ev.ComputeHash()
			if err != nil {
				return false, att, fmt.Errorf("compute overlap evidence hash: %w", err)
			}
		}

		switch ev.Class {
		case overlap.ClassWarning:
			_, _ = deps.Ledger.InsertOverlapEvidence(ctx, ledger.OverlapEvidenceRow{
				FeatureID:     att.FeatureID,
				Version:       ev.Version,
				EvidenceHash:  evHash,
				EvidenceClass: string(ev.Class),
				EvidenceJSON:  evJSON,
				CreatedAt:     updateNow(deps),
			})

		case overlap.ClassRequired:
			existingRequests, err := deps.Ledger.AllReconciliationRequests(ctx)
			if err != nil {
				return false, att, fmt.Errorf("query existing reconciliation requests: %w", err)
			}

			var (
				matched         *ledger.ReconciliationRequestRow
				matchedOtherSHA string // whichever of matched's stored SHAs corresponds to otherFeat, by orientation
			)
			for i := range existingRequests {
				reqRow := existingRequests[i]
				if reqRow.Status != string(reconcile.RequestStatusAwaiting) && reqRow.Status != string(reconcile.RequestStatusApproved) {
					continue
				}
				src, _, tgt, _ := reconcile.ParseDirection(reqRow.Direction)
				if src == att.FeatureID && tgt == otherFeat.ID {
					matched = &existingRequests[i]
					matchedOtherSHA = reqRow.TargetSHA
					break
				}
				if src == otherFeat.ID && tgt == att.FeatureID {
					matched = &existingRequests[i]
					matchedOtherSHA = reqRow.SourceSHA
					break
				}
			}

			// A resolved candidate for THIS exact overlap clears the block: a human already
			// produced a reconciled commit and registered it via `lucind-ai reconcile candidate
			// resolve`. Matched on the OTHER feature's current tip, not the full evidence hash:
			// this attempt's own candidate SHA is fresh every retry (a new commit, new
			// timestamp, same content), so requiring the whole evidence to match byte-for-byte
			// would never match on a real retry. What must not have moved is the other side of
			// the conflict -- if otherFeat's tip is still what the human resolved against, their
			// resolution still applies regardless of this retry's own SHA (which gets replaced
			// by it below). If otherFeat has since promoted again, its tip differs from what the
			// request recorded and this falls through to blocking again, same as an unresolved
			// overlap.
			if matched != nil && matched.Status == string(reconcile.RequestStatusApproved) && matchedOtherSHA == otherSHA {
				cand, cErr := deps.Ledger.ReconciliationCandidateByRequest(ctx, matched.ID)
				if cErr == nil && cand.Status == string(reconcile.CandidateStatusIntegrated) && cand.CandidateSHA != "" {
					att.CandidateSHA = cand.CandidateSHA
					continue
				}
			}

			if matched == nil {
				_, err = reconcileSvc.CreateRequest(ctx, reconcile.CreateRequestParams{
					FeatureID:     att.FeatureID,
					SourceFeature: att.FeatureID,
					SourceParent:  parentRef,
					TargetFeature: otherFeat.ID,
					TargetParent:  otherFeat.ParentRef,
					SourceSHA:     att.CandidateSHA,
					TargetSHA:     otherSHA,
					Evidence:      ev,
					TTL:           15 * time.Minute,
				})
				if err != nil {
					return false, att, fmt.Errorf("create reconciliation request: %w", err)
				}
			}

			now := updateNow(deps)
			att.Status = AttemptStatusBlocked
			att.FailureReason = fmt.Sprintf("promotion blocked: reconciliation-required overlap with feature %s", otherFeat.ID)
			att.UpdatedAt = now
			if err := updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_blocked", att.FailureReason); err != nil {
				return false, att, fmt.Errorf("update attempt blocked status: %w", err)
			}
			_ = featSvc.ReleaseLease(ctx, att.FeatureID, att.Owner, att.Fence)
			return true, att, nil

		case overlap.ClassInformational:
			// Informational evidence does not block
		}
	}

	return false, att, nil
}
