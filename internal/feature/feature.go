// Package feature manages feature lifecycle states and per-feature expiring leases
// with monotonic fencing tokens backed by the ledger.
package feature

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
)

// Sentinel errors returned by feature package methods.
var (
	ErrInvalidParentRef  = errors.New("feature: invalid parent ref")
	ErrInvalidTransition = errors.New("feature: invalid status transition")
	ErrFeatureImmutable  = errors.New("feature: base sha and parent ref are immutable")
	ErrFeatureNotFound   = errors.New("feature: feature not found")
	ErrFeatureIDRequired = errors.New("feature: id is required")
	ErrBaseSHARequired   = errors.New("feature: base sha is required")
	ErrLeaseHeld         = errors.New("feature: lease currently held by another active owner")
	ErrLeaseNotFound     = errors.New("feature: lease not found")
	ErrStaleLease        = errors.New("feature: lease is stale or fence superseded")
	ErrLeaseExpired      = errors.New("feature: lease has expired")
	ErrFeatureIDMissing  = errors.New("feature: feature id is required")
	ErrOwnerMissing      = errors.New("feature: owner is required")
)

const timestampFormat = "2006-01-02T15:04:05.000000000Z"

func formatTimestamp(t time.Time) string {
	return t.UTC().Format(timestampFormat)
}

func parseTimestamp(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}

// Status represents the lifecycle status of a feature.
type Status string

const (
	StatusCreated  Status = "created"
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

// Valid reports whether s is a valid feature status.
func (s Status) Valid() bool {
	switch s {
	case StatusCreated, StatusActive, StatusDisabled:
		return true
	default:
		return false
	}
}

// Feature represents one row of the features table.
type Feature struct {
	ID                string
	ParentRef         string
	BaseSHA           string
	ExpectedParentSHA string
	Status            Status
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Lease represents one row of the feature_leases table: a per-feature expiring
// lease with a monotonic fencing token for distributed mutation serialization.
type Lease struct {
	FeatureID  string
	Owner      string
	Fence      int64
	ExpiresAt  time.Time
	AcquiredAt time.Time
	UpdatedAt  time.Time
}

// Valid reports whether the lease is held and unexpired at the given time.
func (l Lease) Valid(now time.Time) bool {
	return now.Before(l.ExpiresAt)
}

// Service provides feature lifecycle and lease operations backed by the ledger.
type Service struct {
	ledger *ledger.Ledger
}

// NewService constructs a new feature Service handle.
func NewService(l *ledger.Ledger) *Service {
	return &Service{ledger: l}
}

// ValidateParentRef ensures parent ref is non-empty, not main, and not in the
// Lucind temp namespace. It is exported so a caller can reject an unusable
// parent before doing any work, using the same rule Create enforces.
func ValidateParentRef(parentRef string) error {
	trimmed := strings.TrimSpace(parentRef)
	if trimmed == "" {
		return ErrInvalidParentRef
	}
	if trimmed == "main" || trimmed == "refs/heads/main" {
		return ErrInvalidParentRef
	}
	if strings.HasPrefix(trimmed, "lucind/") || strings.HasPrefix(trimmed, "refs/heads/lucind/") {
		return ErrInvalidParentRef
	}
	return nil
}

// Create records a new feature, transitioning created -> active and emitting audit events.
// If the feature already exists and is active, its parentRef and baseSHA are immutable:
// identical values return the existing feature idempotently, and differing values
// return ErrFeatureImmutable. This protects any attempt already recorded against that
// anchor from having its base silently moved out from under it.
//
// If the feature already exists but has been disabled (see Disable), Create instead
// reactivates it under the given parentRef/baseSHA -- which may differ from the
// disabled feature's original anchor. This is the supported "retire and recreate"
// path: an active feature's anchor cannot be re-pointed in place, but once a feature
// has been explicitly retired via "lucind-ai feature disable", its ID is free to be
// re-anchored to a corrected base and reused rather than orphaned forever.
func (s *Service) Create(ctx context.Context, id, parentRef, baseSHA string, expectedParentSHA ...string) (Feature, error) {
	if strings.TrimSpace(id) == "" {
		return Feature{}, ErrFeatureIDRequired
	}
	if strings.TrimSpace(baseSHA) == "" {
		return Feature{}, ErrBaseSHARequired
	}
	if err := ValidateParentRef(parentRef); err != nil {
		return Feature{}, err
	}

	expParentSHA := ""
	if len(expectedParentSHA) > 0 {
		expParentSHA = expectedParentSHA[0]
	}

	// Check if already exists
	existing, err := s.Get(ctx, id)
	if err == nil {
		if existing.Status == StatusDisabled {
			return s.reactivateDisabled(ctx, existing, parentRef, baseSHA, expParentSHA)
		}
		if existing.ParentRef != parentRef || existing.BaseSHA != baseSHA {
			return Feature{}, ErrFeatureImmutable
		}
		return existing, nil
	} else if !errors.Is(err, ErrFeatureNotFound) {
		return Feature{}, err
	}

	now := time.Now().UTC()
	nowStr := formatTimestamp(now)

	// Atomic write for created state
	err = s.ledger.WriteWithAudit(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO features (id, parent_ref, base_sha, expected_parent_sha, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, parentRef, baseSHA, expParentSHA, string(StatusCreated), nowStr, nowStr,
		)
		return err
	}, ledger.IntegrationEvent{
		FeatureID: id,
		Type:      "feature_created",
		Detail:    fmt.Sprintf("parent_ref=%s base_sha=%s", parentRef, baseSHA),
		At:        now,
	})
	if err != nil {
		return Feature{}, fmt.Errorf("feature: record created state: %w", err)
	}

	// Atomic write for active state
	activatedAt := time.Now().UTC()
	activatedAtStr := formatTimestamp(activatedAt)
	err = s.ledger.WriteWithAudit(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			UPDATE features SET status = ?, updated_at = ? WHERE id = ?`,
			string(StatusActive), activatedAtStr, id,
		)
		return err
	}, ledger.IntegrationEvent{
		FeatureID: id,
		Type:      "feature_activated",
		Detail:    "status=active",
		At:        activatedAt,
	})
	if err != nil {
		return Feature{}, fmt.Errorf("feature: record active state: %w", err)
	}

	return Feature{
		ID:                id,
		ParentRef:         parentRef,
		BaseSHA:           baseSHA,
		ExpectedParentSHA: expParentSHA,
		Status:            StatusActive,
		CreatedAt:         now,
		UpdatedAt:         activatedAt,
	}, nil
}

// reactivateDisabled re-anchors a previously disabled feature to a (possibly new)
// parentRef/baseSHA and transitions it back to active, freeing the ID for reuse
// instead of leaving it permanently blocked by ErrFeatureImmutable. It is only
// reachable from Create when the existing row's status is StatusDisabled.
func (s *Service) reactivateDisabled(ctx context.Context, existing Feature, parentRef, baseSHA, expParentSHA string) (Feature, error) {
	now := time.Now().UTC()
	nowStr := formatTimestamp(now)

	err := s.ledger.WriteWithAudit(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			UPDATE features SET parent_ref = ?, base_sha = ?, expected_parent_sha = ?, status = ?, updated_at = ?
			WHERE id = ?`,
			parentRef, baseSHA, expParentSHA, string(StatusActive), nowStr, existing.ID,
		)
		return err
	}, ledger.IntegrationEvent{
		FeatureID: existing.ID,
		Type:      "feature_reactivated",
		Detail:    fmt.Sprintf("parent_ref=%s base_sha=%s (previous_base_sha=%s)", parentRef, baseSHA, existing.BaseSHA),
		At:        now,
	})
	if err != nil {
		return Feature{}, fmt.Errorf("feature: reactivate disabled feature %q: %w", existing.ID, err)
	}

	return Feature{
		ID:                existing.ID,
		ParentRef:         parentRef,
		BaseSHA:           baseSHA,
		ExpectedParentSHA: expParentSHA,
		Status:            StatusActive,
		CreatedAt:         existing.CreatedAt,
		UpdatedAt:         now,
	}, nil
}

// Activate transitions a created feature to active state.
func (s *Service) Activate(ctx context.Context, id string) error {
	feat, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	if feat.Status == StatusDisabled {
		return ErrInvalidTransition
	}
	if feat.Status == StatusActive {
		return nil
	}

	now := time.Now().UTC()
	return s.ledger.WriteWithAudit(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE features SET status = ?, updated_at = ? WHERE id = ?`,
			string(StatusActive), formatTimestamp(now), id)
		return err
	}, ledger.IntegrationEvent{
		FeatureID: id,
		Type:      "feature_activated",
		Detail:    "status=active",
		At:        now,
	})
}

// Disable transitions an active feature to disabled state. A disabled feature is
// excluded from Ledger.ActiveFeatures (and therefore from the overlap gate's active
// set), and its ID becomes eligible for re-anchoring via Create -- see
// reactivateDisabled.
func (s *Service) Disable(ctx context.Context, id string) error {
	feat, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	if feat.Status == StatusDisabled {
		return nil
	}

	now := time.Now().UTC()
	return s.ledger.WriteWithAudit(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE features SET status = ?, updated_at = ? WHERE id = ?`,
			string(StatusDisabled), formatTimestamp(now), id)
		return err
	}, ledger.IntegrationEvent{
		FeatureID: id,
		Type:      "feature_disabled",
		Detail:    "status=disabled",
		At:        now,
	})
}

// Get retrieves a feature row by ID.
func (s *Service) Get(ctx context.Context, id string) (Feature, error) {
	var (
		f         Feature
		status    string
		createdAt string
		updatedAt string
	)

	err := s.ledger.DB().QueryRowContext(ctx, `
		SELECT id, parent_ref, base_sha, expected_parent_sha, status, created_at, updated_at
		FROM features WHERE id = ?`, id,
	).Scan(
		&f.ID, &f.ParentRef, &f.BaseSHA, &f.ExpectedParentSHA, &status, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Feature{}, ErrFeatureNotFound
		}
		return Feature{}, fmt.Errorf("feature: query get %q: %w", id, err)
	}

	f.Status = Status(status)
	if t, err := parseTimestamp(createdAt); err == nil {
		f.CreatedAt = t
	}
	if t, err := parseTimestamp(updatedAt); err == nil {
		f.UpdatedAt = t
	}

	return f, nil
}

// AcquireLease acquires or re-acquires a per-feature expiring lease with a monotonic fencing token.
// If no lease row exists, fence starts at 1. If an existing lease has expired, fence is incremented.
// If an active unexpired lease is held, ErrLeaseHeld is returned.
func (s *Service) AcquireLease(ctx context.Context, featureID, owner string, ttl time.Duration) (Lease, error) {
	if strings.TrimSpace(featureID) == "" {
		return Lease{}, ErrFeatureIDMissing
	}
	if strings.TrimSpace(owner) == "" {
		return Lease{}, ErrOwnerMissing
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}

	now := time.Now().UTC()
	nowStr := formatTimestamp(now)
	expiresAt := now.Add(ttl)
	expiresAtStr := formatTimestamp(expiresAt)

	// Conditional insert or update if absent or expired
	res, err := s.ledger.DB().ExecContext(ctx, `
		INSERT INTO feature_leases (feature_id, owner, fence, expires_at, acquired_at, updated_at)
		VALUES (?, ?, 1, ?, ?, ?)
		ON CONFLICT(feature_id) DO UPDATE SET
			owner = excluded.owner,
			fence = feature_leases.fence + 1,
			expires_at = excluded.expires_at,
			acquired_at = excluded.acquired_at,
			updated_at = excluded.updated_at
		WHERE feature_leases.expires_at <= excluded.acquired_at`,
		featureID, owner, expiresAtStr, nowStr, nowStr,
	)
	if err != nil {
		return Lease{}, fmt.Errorf("feature: acquire lease: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return Lease{}, fmt.Errorf("feature: read rows affected: %w", err)
	}
	if affected == 0 {
		return Lease{}, ErrLeaseHeld
	}

	// Fetch current fence
	var (
		fence       int64
		acqAtStr    string
		updAtStr    string
		expAtStr    string
		actualOwner string
	)
	err = s.ledger.DB().QueryRowContext(ctx, `
		SELECT owner, fence, expires_at, acquired_at, updated_at
		FROM feature_leases WHERE feature_id = ?`, featureID,
	).Scan(&actualOwner, &fence, &expAtStr, &acqAtStr, &updAtStr)
	if err != nil {
		return Lease{}, fmt.Errorf("feature: query acquired lease: %w", err)
	}

	parsedExp, _ := parseTimestamp(expAtStr)
	parsedAcq, _ := parseTimestamp(acqAtStr)
	parsedUpd, _ := parseTimestamp(updAtStr)

	return Lease{
		FeatureID:  featureID,
		Owner:      actualOwner,
		Fence:      fence,
		ExpiresAt:  parsedExp,
		AcquiredAt: parsedAcq,
		UpdatedAt:  parsedUpd,
	}, nil
}

// RenewLease extends the expiration of an existing active lease if (owner, fence) matches and the lease is unexpired.
func (s *Service) RenewLease(ctx context.Context, featureID, owner string, fence int64, ttl time.Duration) (Lease, error) {
	if strings.TrimSpace(featureID) == "" {
		return Lease{}, ErrFeatureIDMissing
	}
	if strings.TrimSpace(owner) == "" {
		return Lease{}, ErrOwnerMissing
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}

	now := time.Now().UTC()
	nowStr := formatTimestamp(now)
	newExpiresAt := now.Add(ttl)
	newExpiresAtStr := formatTimestamp(newExpiresAt)

	res, err := s.ledger.DB().ExecContext(ctx, `
		UPDATE feature_leases
		SET expires_at = ?, updated_at = ?
		WHERE feature_id = ? AND owner = ? AND fence = ? AND expires_at > ?`,
		newExpiresAtStr, nowStr, featureID, owner, fence, nowStr,
	)
	if err != nil {
		return Lease{}, fmt.Errorf("feature: renew lease: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return Lease{}, fmt.Errorf("feature: read rows affected: %w", err)
	}

	if affected == 0 {
		// Diagnose failure reason
		var (
			dbOwner      string
			dbFence      int64
			dbExpiresStr string
		)
		err := s.ledger.DB().QueryRowContext(ctx, `
			SELECT owner, fence, expires_at FROM feature_leases WHERE feature_id = ?`, featureID,
		).Scan(&dbOwner, &dbFence, &dbExpiresStr)
		if errors.Is(err, sql.ErrNoRows) {
			return Lease{}, ErrLeaseNotFound
		} else if err != nil {
			return Lease{}, fmt.Errorf("feature: query lease after failed renew: %w", err)
		}

		if dbOwner != owner || dbFence != fence {
			return Lease{}, ErrStaleLease
		}
		dbExp, _ := parseTimestamp(dbExpiresStr)
		if !now.Before(dbExp) {
			return Lease{}, ErrLeaseExpired
		}
		return Lease{}, ErrStaleLease
	}

	var acqAtStr string
	_ = s.ledger.DB().QueryRowContext(ctx, `SELECT acquired_at FROM feature_leases WHERE feature_id = ?`, featureID).Scan(&acqAtStr)
	acqAt, _ := parseTimestamp(acqAtStr)

	return Lease{
		FeatureID:  featureID,
		Owner:      owner,
		Fence:      fence,
		ExpiresAt:  newExpiresAt,
		AcquiredAt: acqAt,
		UpdatedAt:  now,
	}, nil
}

// ReleaseLease explicitly expires an active lease held by (owner, fence), allowing immediate re-acquisition with an incremented fence.
func (s *Service) ReleaseLease(ctx context.Context, featureID, owner string, fence int64) error {
	if strings.TrimSpace(featureID) == "" {
		return ErrFeatureIDMissing
	}
	if strings.TrimSpace(owner) == "" {
		return ErrOwnerMissing
	}

	now := time.Now().UTC()
	nowStr := formatTimestamp(now)
	pastStr := formatTimestamp(time.Unix(0, 0).UTC())

	res, err := s.ledger.DB().ExecContext(ctx, `
		UPDATE feature_leases
		SET expires_at = ?, updated_at = ?
		WHERE feature_id = ? AND owner = ? AND fence = ?`,
		pastStr, nowStr, featureID, owner, fence,
	)
	if err != nil {
		return fmt.Errorf("feature: release lease: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("feature: read rows affected: %w", err)
	}

	if affected == 0 {
		var (
			dbOwner string
			dbFence int64
		)
		err := s.ledger.DB().QueryRowContext(ctx, `
			SELECT owner, fence FROM feature_leases WHERE feature_id = ?`, featureID,
		).Scan(&dbOwner, &dbFence)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrLeaseNotFound
		} else if err != nil {
			return fmt.Errorf("feature: query lease after failed release: %w", err)
		}
		if dbOwner != owner || dbFence != fence {
			return ErrStaleLease
		}
		return ErrStaleLease
	}

	return nil
}

// ValidateLease verifies that (owner, fence) is the current active, unexpired leaseholder.
// Returns ErrLeaseNotFound if no lease exists, ErrStaleLease if owner/fence does not match,
// or ErrLeaseExpired if the lease has expired.
func (s *Service) ValidateLease(ctx context.Context, featureID, owner string, fence int64) error {
	var (
		dbOwner      string
		dbFence      int64
		expiresAtStr string
	)

	err := s.ledger.DB().QueryRowContext(ctx, `
		SELECT owner, fence, expires_at
		FROM feature_leases WHERE feature_id = ?`, featureID,
	).Scan(&dbOwner, &dbFence, &expiresAtStr)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrLeaseNotFound
	} else if err != nil {
		return fmt.Errorf("feature: validate lease query: %w", err)
	}

	if dbOwner != owner || dbFence != fence {
		return ErrStaleLease
	}

	expiresAt, err := parseTimestamp(expiresAtStr)
	if err != nil {
		return fmt.Errorf("feature: parse lease expiry %q: %w", expiresAtStr, err)
	}

	if !time.Now().UTC().Before(expiresAt) {
		return ErrLeaseExpired
	}

	return nil
}

// GetLease retrieves the current lease row for a feature ID.
func (s *Service) GetLease(ctx context.Context, featureID string) (Lease, error) {
	var (
		l            Lease
		expiresAtStr string
		acquiredAt   string
		updatedAt    string
	)

	err := s.ledger.DB().QueryRowContext(ctx, `
		SELECT feature_id, owner, fence, expires_at, acquired_at, updated_at
		FROM feature_leases WHERE feature_id = ?`, featureID,
	).Scan(&l.FeatureID, &l.Owner, &l.Fence, &expiresAtStr, &acquiredAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Lease{}, ErrLeaseNotFound
	} else if err != nil {
		return Lease{}, fmt.Errorf("feature: query get lease: %w", err)
	}

	if t, err := parseTimestamp(expiresAtStr); err == nil {
		l.ExpiresAt = t
	}
	if t, err := parseTimestamp(acquiredAt); err == nil {
		l.AcquiredAt = t
	}
	if t, err := parseTimestamp(updatedAt); err == nil {
		l.UpdatedAt = t
	}

	return l, nil
}
