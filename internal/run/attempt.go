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

// startLeaseRenewal begins a background loop that periodically renews the
// feature lease at renewInterval while checkFunc (integrate.Check) runs
// during the CHECKING phase of driveAttemptFromLeased. checkFunc is a
// synchronous call that can run arbitrarily long, and the lease acquired at
// the start of the attempt is a fixed-TTL lock that nothing else proves
// liveness against during that window -- without this, a genuinely
// still-working attempt whose checks simply take a while gets wrongly killed
// once leaseTTL elapses.
//
// Renewal errors are deliberately ignored here: this loop is not the
// authoritative gate on lease loss -- the featSvc.ValidateLease call
// immediately after checkFunc returns is -- so a transient renewal failure
// must never abort an in-flight check. A genuine loss of the lease (another
// owner took it) still surfaces correctly through that post-check
// ValidateLease call, since renewal cannot succeed once owner/fence no
// longer match.
//
// The returned stop function cancels the loop and blocks until it has
// exited, so the caller can rely on the goroutine being gone the moment
// stop returns. It must be called exactly once, unconditionally, as soon as
// checkFunc returns (success, failure, or error alike) to avoid leaking it.
func startLeaseRenewal(ctx context.Context, featSvc *feature.Service, att Attempt, leaseTTL, renewInterval time.Duration) (stop func()) {
	renewCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	go func() {
		defer close(done)
		ticker := time.NewTicker(renewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-renewCtx.Done():
				return
			case <-ticker.C:
				_, _ = featSvc.RenewLease(renewCtx, att.FeatureID, att.Owner, att.Fence, leaseTTL)
			}
		}
	}()

	return func() {
		cancel()
		<-done
	}
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
	wtPath, branchName, err := combineFunc(ctx, deps.PrimaryRoot, req.ID, req.ParentRef, req.BaseSHA, req.Branches)
	if err != nil {
		now = updateNow(deps)
		att.Status = AttemptStatusFailed
		att.FailureReason = fmt.Sprintf("combine failed: %v", err)
		att.UpdatedAt = now
		_ = updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_failed", att.FailureReason)
		// A definitively failed attempt is terminal and will never resume; holding its
		// lease until FeatureLeaseTTL naturally expires would block every other attempt
		// on this feature -- a fresh redispatch or a "lucind-ai integrate retry" of the
		// already-completed lane branches -- for no reason, since nothing is still using
		// it. Release it now so the feature is immediately available again.
		_ = featSvc.ReleaseLease(ctx, att.FeatureID, att.Owner, att.Fence)
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

	checkLeaseTTL := deps.FeatureLeaseTTL
	if checkLeaseTTL <= 0 {
		checkLeaseTTL = 30 * time.Second
	}
	renewInterval := deps.RenewInterval
	if renewInterval <= 0 {
		renewInterval = checkLeaseTTL / 3
		if renewInterval < time.Second {
			renewInterval = time.Second
		}
	}
	stopLeaseRenewal := startLeaseRenewal(ctx, featSvc, att, checkLeaseTTL, renewInterval)
	passed, output, err := checkFunc(ctx, wtPath)
	stopLeaseRenewal()
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
		// See the combine-failure branch above: this attempt is terminal, so its lease
		// is released immediately rather than held until FeatureLeaseTTL expires.
		_ = featSvc.ReleaseLease(ctx, att.FeatureID, att.Owner, att.Fence)
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
		stale := errors.Is(err, integrate.ErrStaleCAS)
		if stale {
			att.Status = AttemptStatusStale
		} else {
			att.Status = AttemptStatusFailed
		}
		att.FailureReason = fmt.Sprintf("promotion CAS failed: %v", err)
		att.UpdatedAt = now
		_ = updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_failed", att.FailureReason)
		if !stale {
			// See driveAttemptFromLeased's combine/checks failure branches: a
			// non-stale CAS failure is terminal, so its lease is released
			// immediately rather than held until FeatureLeaseTTL expires. A
			// stale result is left untouched here: the parent ref already moved
			// out from under this attempt, so its lease may already be invalid
			// or superseded, and ReleaseLease's own owner/fence check makes it
			// a safe no-op either way.
			_ = featSvc.ReleaseLease(ctx, att.FeatureID, att.Owner, att.Fence)
		}
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

// resolveOwnCurrentSHA resolves parentRef's current tip using the same
// multi-tier fallback evaluateOverlapGate already uses for otherFeat's tip.
func resolveOwnCurrentSHA(ctx context.Context, deps Deps, parentRef string) string {
	if deps.ResolveRefSHA != nil {
		sha, _ := deps.ResolveRefSHA(ctx, deps.PrimaryRoot, parentRef)
		return sha
	}
	if deps.GitRunner != nil {
		canonicalRef := worktree.CanonicalizeRef(parentRef)
		out, err := deps.GitRunner.Run(ctx, deps.PrimaryRoot, "rev-parse", "--verify", canonicalRef+"^{commit}")
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	sha, _ := worktree.ResolveCommitSHA(ctx, worktree.DefaultGitRunner, deps.PrimaryRoot, parentRef)
	return sha
}

// ownResolutionStillValid guards reuse of a previously approved+integrated
// reconciliation candidateSHA: this attempt's own feature (at parentRef) must
// still have candidateSHA as an ancestor -- i.e. candidateSHA must already
// contain, not predate, this attempt's own current state. Without this, a
// candidate resolved for an EARLIER round (already consumed by an earlier
// promotion, or simply stale relative to real new work landed on the branch
// since) could be reused for a LATER attempt with genuinely different
// content, CAS'ing the branch backward and silently discarding everything
// landed since -- see evaluateOverlapGate's reuse check below.
//
// deps.IsAncestorSHA is nil in every test double that does not opt in, which
// deliberately preserves prior behavior (reuse allowed) unchanged for them;
// only cmd/lucind-ai/cli.go's productionDeps and a test that specifically
// exercises this guard need to wire it.
func ownResolutionStillValid(ctx context.Context, deps Deps, parentRef, candidateSHA string) bool {
	if deps.IsAncestorSHA == nil {
		return true
	}
	ownCurrentSHA := resolveOwnCurrentSHA(ctx, deps, parentRef)
	if strings.TrimSpace(ownCurrentSHA) == "" {
		return false
	}
	ok, err := deps.IsAncestorSHA(ctx, deps.PrimaryRoot, ownCurrentSHA, candidateSHA)
	if err != nil {
		return false
	}
	return ok
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

	// originalCandidateSHA is att's real candidate before any conflict in this pass resolves.
	// Every evalFunc call below must compare against this value, never att.CandidateSHA directly:
	// a resolved conflict's override is accumulated separately (resolvedOverrideSHA /
	// resolvedAgainst) and only ever applied to att.CandidateSHA after the loop, so a later
	// comparison in the same pass can never see an earlier resolution's SHA by mistake.
	originalCandidateSHA := att.CandidateSHA
	var (
		resolvedOverrideSHA   string   // the resolved candidate SHA, valid only when exactly one conflict resolves
		resolvedAgainst       []string // otherFeat.ID for every conflict that resolved in this pass
		hasUnresolvedRequired bool     // true once any required overlap in this pass is found unresolved
		unresolvedBlockReason string   // block reason for the first unresolved required overlap encountered

		// hasUnresolvedRequired/unresolvedBlockReason exist so that an unresolved required
		// overlap does NOT short-circuit-block before every other active feature has been
		// evaluated. The old behavior returned immediately on the first unresolved required
		// overlap encountered, in ActiveFeatures() iteration order -- which meant a genuinely
		// resolved conflict later (or earlier) in the same list could be silently discarded
		// for this pass whenever 2+ other features simultaneously had a required overlap and
		// not all of them were resolved yet. That made the documented recovery path --
		// "resolve and promote sequentially, one feature pair at a time" -- impossible to
		// actually execute for 3+ simultaneously conflicting features: any partial resolution
		// was either never reached (unresolved one hit first) or, once every conflict was
		// resolved, rejected by the N-way block below. Deferring the block decision until
		// after the full loop lets exactly-one-resolved passes succeed regardless of where in
		// the list the still-unresolved features fall.
	)

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

		ev, err := evalFunc(ctx, deps.PrimaryRoot, commonBase, originalCandidateSHA, otherSHA)
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
			// resolution still applies regardless of this retry's own SHA. If otherFeat has
			// since promoted again, its tip differs from what the request recorded and this
			// falls through to blocking again, same as an unresolved overlap.
			//
			// The override is accumulated here, never applied to att.CandidateSHA directly:
			// doing so in place would corrupt originalCandidateSHA's meaning for any later
			// otherFeat comparison in this same pass, and would silently discard every
			// resolution but the last if two or more conflicts resolve simultaneously -- see
			// the post-loop accumulation below.
			//
			// ownResolutionStillValid guards against reusing a candidate resolved for an
			// EARLIER round of this same feature's own work: matchedOtherSHA==otherSHA alone
			// only proves the OTHER side hasn't moved, never that THIS attempt's own content
			// is still what the resolution was registered against. A later attempt with
			// genuinely new content (a fresh dispatch, not a mere retry) must not silently
			// adopt a stale resolved SHA that predates it -- doing so would CAS the branch
			// backward, discarding real work landed since.
			// blockReason distinguishes *why* this overlap is still blocking, rather than
			// reporting the same generic message for a genuinely fresh conflict, an
			// approved-but-not-yet-resolved request, a resolution registered against a
			// stale tip for otherFeat, an unreadable/un-integrated candidate, and a
			// resolution that predates this attempt's own current content. Before this,
			// every one of those distinct states surfaced as the identical
			// "promotion blocked: reconciliation-required overlap with feature X", making
			// "you haven't resolved this yet" indistinguishable from "you resolved it
			// against the wrong SHA" -- the single biggest time sink in diagnosing a
			// non-converging reconcile/retry cycle.
			blockReason := fmt.Sprintf("promotion blocked: reconciliation-required overlap with feature %s", otherFeat.ID)

			if matched != nil {
				switch {
				case matched.Status != string(reconcile.RequestStatusApproved):
					blockReason = fmt.Sprintf(
						"promotion blocked: reconciliation-required overlap with feature %s (request %s exists but is not yet approved -- status %q; run reconcile approve, then reconcile resolve)",
						otherFeat.ID, matched.ID, matched.Status,
					)

				case matchedOtherSHA != otherSHA:
					blockReason = fmt.Sprintf(
						"promotion blocked: reconciliation-required overlap with feature %s (request %s is approved but was registered against a stale tip for %s -- recorded %s, current %s; run reconcile renew against the current tip, then reconcile approve/resolve again)",
						otherFeat.ID, matched.ID, otherFeat.ID, matchedOtherSHA, otherSHA,
					)

				default:
					cand, cErr := deps.Ledger.ReconciliationCandidateByRequest(ctx, matched.ID)
					switch {
					case cErr != nil:
						blockReason = fmt.Sprintf(
							"promotion blocked: reconciliation-required overlap with feature %s (request %s is approved but its candidate could not be read: %v)",
							otherFeat.ID, matched.ID, cErr,
						)

					case cand.Status != string(reconcile.CandidateStatusIntegrated) || cand.CandidateSHA == "":
						blockReason = fmt.Sprintf(
							"promotion blocked: reconciliation-required overlap with feature %s (request %s is approved but not yet resolved -- candidate %s status is %q; run reconcile resolve)",
							otherFeat.ID, matched.ID, cand.ID, cand.Status,
						)

					case !ownResolutionStillValid(ctx, deps, parentRef, cand.CandidateSHA):
						blockReason = fmt.Sprintf(
							"promotion blocked: reconciliation-required overlap with feature %s (request %s's resolved candidate %s predates this attempt's own current content; resolve again against the current state)",
							otherFeat.ID, matched.ID, cand.CandidateSHA,
						)

					default:
						// This is the reuse path evaluateOverlapGate's older comment above
						// describes: matchedOtherSHA==otherSHA proves the OTHER side hasn't
						// moved, and ownResolutionStillValid proves this attempt's OWN content
						// is still what the resolution was registered against. Adopt the
						// resolved candidate and skip blocking for this otherFeat entirely.
						resolvedOverrideSHA = cand.CandidateSHA
						resolvedAgainst = append(resolvedAgainst, otherFeat.ID)
						continue
					}
				}
			}

			if matched == nil {
				_, err = reconcileSvc.CreateRequest(ctx, reconcile.CreateRequestParams{
					FeatureID:     att.FeatureID,
					SourceFeature: att.FeatureID,
					SourceParent:  parentRef,
					TargetFeature: otherFeat.ID,
					TargetParent:  otherFeat.ParentRef,
					SourceSHA:     originalCandidateSHA,
					TargetSHA:     otherSHA,
					Evidence:      ev,
					TTL:           15 * time.Minute,
				})
				if err != nil {
					return false, att, fmt.Errorf("create reconciliation request: %w", err)
				}
			}

			// Do NOT block-and-return here: an unresolved required overlap with THIS
			// otherFeat must not prevent evaluating the remaining active features, some of
			// which may be resolved and need to accumulate into resolvedAgainst below. Record
			// the first unresolved reason encountered and keep looping; the actual block (or
			// promotion, if this turns out to be the only unresolved one) is decided once,
			// after every active feature has been evaluated.
			if !hasUnresolvedRequired {
				hasUnresolvedRequired = true
				unresolvedBlockReason = blockReason
			}
			continue

		case overlap.ClassInformational:
			// Informational evidence does not block
		}
	}

	if hasUnresolvedRequired {
		// At least one required overlap is still unresolved: block on it regardless of how
		// many OTHER overlaps resolved in this same pass. Any resolutions accumulated above
		// are simply not applied this round -- they remain valid (per ownResolutionStillValid)
		// and will be picked up again, together with fewer outstanding unresolved conflicts,
		// on the next retry once this blocking feature is also resolved.
		now := updateNow(deps)
		att.Status = AttemptStatusBlocked
		att.FailureReason = unresolvedBlockReason
		att.CandidateSHA = originalCandidateSHA
		att.UpdatedAt = now
		if err := updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_blocked", att.FailureReason); err != nil {
			return false, att, fmt.Errorf("update attempt blocked status: %w", err)
		}
		_ = featSvc.ReleaseLease(ctx, att.FeatureID, att.Owner, att.Fence)
		return true, att, nil
	}

	switch len(resolvedAgainst) {
	case 0:
		// Untouched happy path: no required conflict was resolved in this pass.
		return false, att, nil

	case 1:
		// Exactly one resolution: promote using the human-resolved candidate, same as before
		// this fix -- just driven by post-loop accumulation instead of in-loop mutation.
		att.CandidateSHA = resolvedOverrideSHA
		return false, att, nil

	default:
		// Two or more required conflicts resolved simultaneously: a full N-way merge-of-merges
		// is out of scope here. Each resolution is an independent merge commit; there is no
		// mechanism to combine N of them into one candidate, and silently picking one would
		// promote a candidate that drops every other human resolution. Block explicitly instead
		// of guessing, and release the lease so the conflicts can be resolved and promoted
		// sequentially, one feature pair at a time.
		now := updateNow(deps)
		att.Status = AttemptStatusBlocked
		att.FailureReason = fmt.Sprintf(
			"promotion blocked: N-way reconciliation not supported (%d resolved required overlaps: %s); resolve and promote sequentially, one feature pair at a time",
			len(resolvedAgainst), strings.Join(resolvedAgainst, ", "),
		)
		att.CandidateSHA = originalCandidateSHA
		att.UpdatedAt = now
		if err := updateAttemptWithAudit(ctx, deps.Ledger, att, "attempt_blocked", att.FailureReason); err != nil {
			return false, att, fmt.Errorf("update attempt blocked status: %w", err)
		}
		_ = featSvc.ReleaseLease(ctx, att.FeatureID, att.Owner, att.Fence)
		return true, att, nil
	}
}
