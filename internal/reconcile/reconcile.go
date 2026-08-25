// Package reconcile manages exact source-to-target direction approval for
// reconciliation records, expiring requests, and bounded candidate lifecycles.
package reconcile

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/overlap"
)

// Sentinel errors returned by reconcile package methods.
var (
	ErrRequestNotFound        = errors.New("reconcile: reconciliation request not found")
	ErrCandidateNotFound      = errors.New("reconcile: reconciliation candidate not found")
	ErrRequiredOverlapOnly    = errors.New("reconcile: reconciliation request requires class 'required' overlap evidence")
	ErrRequestNotAwaiting     = errors.New("reconcile: request is not awaiting decision")
	ErrRequestExpired         = errors.New("reconcile: reconciliation request has expired")
	ErrStaleExpectedSHA       = errors.New("reconcile: expected source or target sha has changed (stale authority)")
	ErrInvalidDirection       = errors.New("reconcile: invalid direction for request")
	ErrEmptyActor             = errors.New("reconcile: actor is required")
	ErrCandidateAlreadyExists = errors.New("reconcile: candidate already exists for request")
	ErrInvalidTransition      = errors.New("reconcile: invalid status transition")
	ErrEvidenceNil            = errors.New("reconcile: evidence is required")
	ErrMissingFeature         = errors.New("reconcile: source and target features are required")
)

// RequestStatus represents the lifecycle status of a reconciliation request.
type RequestStatus string

const (
	RequestStatusAwaiting  RequestStatus = "awaiting"
	RequestStatusApproved  RequestStatus = "approved"
	RequestStatusDeclined  RequestStatus = "declined"
	RequestStatusCancelled RequestStatus = "cancelled"
	RequestStatusExpired   RequestStatus = "expired"
)

// Valid reports whether s is a valid request status.
func (s RequestStatus) Valid() bool {
	switch s {
	case RequestStatusAwaiting, RequestStatusApproved, RequestStatusDeclined, RequestStatusCancelled, RequestStatusExpired:
		return true
	default:
		return false
	}
}

// CandidateStatus represents the lifecycle status of an approved candidate.
type CandidateStatus string

const (
	CandidateStatusRunning    CandidateStatus = "candidate_running"
	CandidateStatusIntegrated CandidateStatus = "integrated"
	CandidateStatusFailed     CandidateStatus = "failed"
	CandidateStatusStale      CandidateStatus = "stale"
)

// Valid reports whether s is a valid candidate status.
func (s CandidateStatus) Valid() bool {
	switch s {
	case CandidateStatusRunning, CandidateStatusIntegrated, CandidateStatusFailed, CandidateStatusStale:
		return true
	default:
		return false
	}
}

// Request represents one reconciliation request record.
type Request struct {
	ID              string            `json:"id"`
	FeatureID       string            `json:"feature_id"`
	SourceFeature   string            `json:"source_feature"`
	SourceParent    string            `json:"source_parent"`
	TargetFeature   string            `json:"target_feature"`
	TargetParent    string            `json:"target_parent"`
	Direction       string            `json:"direction"`
	Status          RequestStatus     `json:"status"`
	Actor           string            `json:"actor,omitempty"`
	EvidenceVersion string            `json:"evidence_version"`
	EvidenceHash    string            `json:"evidence_hash"`
	SourceSHA       string            `json:"source_sha"`
	TargetSHA       string            `json:"target_sha"`
	Evidence        *overlap.Evidence `json:"evidence,omitempty"`
	ExpiresAt       time.Time         `json:"expires_at"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// Candidate represents one candidate record authorized by direction approval.
type Candidate struct {
	ID            string          `json:"id"`
	RequestID     string          `json:"request_id"`
	Status        CandidateStatus `json:"status"`
	AllowedPaths  []string        `json:"allowed_paths"`
	Model         string          `json:"model"`
	Config        string          `json:"config,omitempty"`
	Output        string          `json:"output,omitempty"`
	Checks        string          `json:"checks,omitempty"`
	CandidateSHA  string          `json:"candidate_sha,omitempty"`
	FailureReason string          `json:"failure_reason,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// OverlapEvaluator is the signature for computing overlap evidence.
type OverlapEvaluator func(ctx context.Context, repoDir, baseSHA, shaA, shaB string, opts ...overlap.EvaluateOption) (*overlap.Evidence, error)

// Service provides reconciliation request and candidate lifecycle operations backed by the ledger.
type Service struct {
	ledger    *ledger.Ledger
	clock     func() time.Time
	idSource  func() string
	evaluator OverlapEvaluator
}

// ServiceOption configures a Service.
type ServiceOption func(*Service)

// WithClock sets a custom time source for the Service.
func WithClock(clock func() time.Time) ServiceOption {
	return func(s *Service) {
		s.clock = clock
	}
}

// WithIDSource sets a custom ID generator for the Service.
func WithIDSource(idSource func() string) ServiceOption {
	return func(s *Service) {
		s.idSource = idSource
	}
}

// WithOverlapEvaluator sets a custom overlap evaluation function for the Service.
func WithOverlapEvaluator(evaluator OverlapEvaluator) ServiceOption {
	return func(s *Service) {
		s.evaluator = evaluator
	}
}

func defaultUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// NewService constructs a new reconcile Service.
func NewService(l *ledger.Ledger, opts ...ServiceOption) *Service {
	s := &Service{
		ledger:    l,
		clock:     func() time.Time { return time.Now().UTC() },
		idSource:  defaultUUID,
		evaluator: overlap.Evaluate,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// FormatDirection formats source and target features and parent refs into a direction string.
func FormatDirection(sourceFeature, sourceParent, targetFeature, targetParent string) string {
	if sourceParent != "" && targetParent != "" {
		return fmt.Sprintf("%s (%s) -> %s (%s)", sourceFeature, sourceParent, targetFeature, targetParent)
	}
	return fmt.Sprintf("%s -> %s", sourceFeature, targetFeature)
}

// ParseDirection parses a direction string into source and target components.
func ParseDirection(dir string) (sourceFeature, sourceParent, targetFeature, targetParent string) {
	parts := strings.Split(dir, " -> ")
	if len(parts) != 2 {
		return dir, "", "", ""
	}
	parsePart := func(p string) (string, string) {
		p = strings.TrimSpace(p)
		if idx := strings.Index(p, " ("); idx != -1 && strings.HasSuffix(p, ")") {
			feat := p[:idx]
			parent := p[idx+2 : len(p)-1]
			return feat, parent
		}
		return p, ""
	}
	sourceFeature, sourceParent = parsePart(parts[0])
	targetFeature, targetParent = parsePart(parts[1])
	return
}

// CreateRequestParams holds parameters for creating a reconciliation request.
type CreateRequestParams struct {
	ID            string
	FeatureID     string
	SourceFeature string
	SourceParent  string
	TargetFeature string
	TargetParent  string
	SourceSHA     string
	TargetSHA     string
	Evidence      *overlap.Evidence
	TTL           time.Duration
}

// CreateRequest creates a new reconciliation request from required-overlap evidence.
func (s *Service) CreateRequest(ctx context.Context, params CreateRequestParams) (Request, error) {
	if params.Evidence == nil {
		return Request{}, ErrEvidenceNil
	}
	if params.Evidence.Class != overlap.ClassRequired {
		return Request{}, ErrRequiredOverlapOnly
	}
	if strings.TrimSpace(params.SourceFeature) == "" || strings.TrimSpace(params.TargetFeature) == "" {
		return Request{}, ErrMissingFeature
	}

	id := params.ID
	if id == "" {
		id = s.idSource()
	}

	featureID := params.FeatureID
	if featureID == "" {
		featureID = params.TargetFeature
	}

	sourceSHA := params.SourceSHA
	if sourceSHA == "" {
		sourceSHA = params.Evidence.FeatureASHA
	}

	targetSHA := params.TargetSHA
	if targetSHA == "" {
		targetSHA = params.Evidence.FeatureBSHA
	}

	ttl := params.TTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}

	now := s.clock()
	expiresAt := now.Add(ttl)

	// Persist overlap evidence
	evJSON, err := params.Evidence.JSON()
	if err != nil {
		return Request{}, fmt.Errorf("reconcile: marshal evidence: %w", err)
	}

	evHash := params.Evidence.Hash
	if evHash == "" {
		evHash, err = params.Evidence.ComputeHash()
		if err != nil {
			return Request{}, fmt.Errorf("reconcile: compute evidence hash: %w", err)
		}
	}

	_, err = s.ledger.InsertOverlapEvidence(ctx, ledger.OverlapEvidenceRow{
		FeatureID:     featureID,
		Version:       params.Evidence.Version,
		EvidenceHash:  evHash,
		EvidenceClass: string(params.Evidence.Class),
		EvidenceJSON:  evJSON,
		CreatedAt:     now,
	})
	if err != nil {
		return Request{}, fmt.Errorf("reconcile: save overlap evidence: %w", err)
	}

	dir := FormatDirection(params.SourceFeature, params.SourceParent, params.TargetFeature, params.TargetParent)

	reqRow := ledger.ReconciliationRequestRow{
		ID:              id,
		FeatureID:       featureID,
		Direction:       dir,
		Status:          string(RequestStatusAwaiting),
		Actor:           "",
		EvidenceVersion: params.Evidence.Version,
		EvidenceHash:    evHash,
		SourceSHA:       sourceSHA,
		TargetSHA:       targetSHA,
		ExpiresAt:       &expiresAt,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	err = s.ledger.WriteWithAudit(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO reconciliation_requests (
				id, feature_id, direction, status, actor, evidence_version,
				evidence_hash, source_sha, target_sha, expires_at, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			reqRow.ID, reqRow.FeatureID, reqRow.Direction, reqRow.Status, reqRow.Actor,
			reqRow.EvidenceVersion, reqRow.EvidenceHash, reqRow.SourceSHA, reqRow.TargetSHA,
			reqRow.ExpiresAt.UTC().Format(time.RFC3339),
			now.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339),
		)
		return err
	}, ledger.IntegrationEvent{
		FeatureID: featureID,
		Type:      "reconciliation_request_created",
		Detail:    fmt.Sprintf("id=%s direction=%s expires_at=%s", id, dir, expiresAt.Format(time.RFC3339)),
		At:        now,
	})
	if err != nil {
		return Request{}, fmt.Errorf("reconcile: write request with audit: %w", err)
	}

	return Request{
		ID:              id,
		FeatureID:       featureID,
		SourceFeature:   params.SourceFeature,
		SourceParent:    params.SourceParent,
		TargetFeature:   params.TargetFeature,
		TargetParent:    params.TargetParent,
		Direction:       dir,
		Status:          RequestStatusAwaiting,
		EvidenceVersion: params.Evidence.Version,
		EvidenceHash:    evHash,
		SourceSHA:       sourceSHA,
		TargetSHA:       targetSHA,
		Evidence:        params.Evidence,
		ExpiresAt:       expiresAt,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

// GetRequest retrieves a reconciliation request by ID, loading and attaching its evidence snapshot.
func (s *Service) GetRequest(ctx context.Context, id string) (Request, error) {
	row, err := s.ledger.ReconciliationRequest(ctx, id)
	if err != nil {
		if errors.Is(err, ledger.ErrReconciliationRequestNotFound) {
			return Request{}, ErrRequestNotFound
		}
		return Request{}, fmt.Errorf("reconcile: query request: %w", err)
	}

	sourceFeature, sourceParent, targetFeature, targetParent := ParseDirection(row.Direction)

	var ev *overlap.Evidence
	if row.EvidenceHash != "" {
		evRow, err := s.ledger.OverlapEvidenceByHash(ctx, row.EvidenceHash)
		if err == nil {
			var parsed overlap.Evidence
			if err := json.Unmarshal([]byte(evRow.EvidenceJSON), &parsed); err == nil {
				ev = &parsed
			}
		}
	}

	status := RequestStatus(row.Status)
	now := s.clock()
	if status == RequestStatusAwaiting && row.ExpiresAt != nil && !now.Before(*row.ExpiresAt) {
		status = RequestStatusExpired
	}

	var exp time.Time
	if row.ExpiresAt != nil {
		exp = *row.ExpiresAt
	}

	return Request{
		ID:              row.ID,
		FeatureID:       row.FeatureID,
		SourceFeature:   sourceFeature,
		SourceParent:    sourceParent,
		TargetFeature:   targetFeature,
		TargetParent:    targetParent,
		Direction:       row.Direction,
		Status:          status,
		Actor:           row.Actor,
		EvidenceVersion: row.EvidenceVersion,
		EvidenceHash:    row.EvidenceHash,
		SourceSHA:       row.SourceSHA,
		TargetSHA:       row.TargetSHA,
		Evidence:        ev,
		ExpiresAt:       exp,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}, nil
}

// ApproveParams holds parameters for approving a reconciliation direction.
type ApproveParams struct {
	RequestID         string
	SourceFeature     string
	TargetFeature     string
	ExpectedSourceSHA string
	ExpectedTargetSHA string
	Actor             string
	AllowedPaths      []string
	CandidateID       string
}

// Approve approves an exact source-to-target direction for an unexpired reconciliation request,
// binding the direction, expected SHAs, actor, and evidence snapshot, and creating exactly one candidate.
func (s *Service) Approve(ctx context.Context, params ApproveParams) (Request, Candidate, error) {
	req, err := s.GetRequest(ctx, params.RequestID)
	if err != nil {
		return Request{}, Candidate{}, err
	}

	now := s.clock()
	if req.Status == RequestStatusExpired || (!req.ExpiresAt.IsZero() && !now.Before(req.ExpiresAt)) {
		return Request{}, Candidate{}, ErrRequestExpired
	}

	if req.Status != RequestStatusAwaiting {
		return Request{}, Candidate{}, ErrRequestNotAwaiting
	}

	if strings.TrimSpace(params.Actor) == "" {
		return Request{}, Candidate{}, ErrEmptyActor
	}

	if (params.ExpectedSourceSHA != "" && params.ExpectedSourceSHA != req.SourceSHA) ||
		(params.ExpectedTargetSHA != "" && params.ExpectedTargetSHA != req.TargetSHA) {
		return Request{}, Candidate{}, ErrStaleExpectedSHA
	}

	if (params.SourceFeature != "" && params.SourceFeature != req.SourceFeature) ||
		(params.TargetFeature != "" && params.TargetFeature != req.TargetFeature) {
		return Request{}, Candidate{}, ErrInvalidDirection
	}

	// Verify no candidate exists for this request
	_, err = s.ledger.ReconciliationCandidateByRequest(ctx, req.ID)
	if err == nil {
		return Request{}, Candidate{}, ErrCandidateAlreadyExists
	} else if !errors.Is(err, ledger.ErrReconciliationCandidateNotFound) {
		return Request{}, Candidate{}, fmt.Errorf("reconcile: check existing candidate: %w", err)
	}

	candID := params.CandidateID
	if candID == "" {
		candID = s.idSource()
	}

	allowedPaths := params.AllowedPaths
	if len(allowedPaths) == 0 && req.Evidence != nil {
		if len(req.Evidence.Signals.ConflictPaths) > 0 {
			allowedPaths = req.Evidence.Signals.ConflictPaths
		} else if len(req.Evidence.Signals.SharedPaths) > 0 {
			allowedPaths = req.Evidence.Signals.SharedPaths
		}
	}
	allowedPathsStr := strings.Join(allowedPaths, ",")

	approvedDir := fmt.Sprintf("%s -> %s", req.SourceFeature, req.TargetFeature)

	candRow := ledger.ReconciliationCandidateRow{
		ID:            candID,
		RequestID:     req.ID,
		Status:        string(CandidateStatusRunning),
		AllowedPaths:  allowedPathsStr,
		Model:         "sonnet",
		Config:        "",
		Output:        "",
		Checks:        "",
		CandidateSHA:  "",
		FailureReason: "",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	err = s.ledger.WriteWithAudit(ctx, func(tx *sql.Tx) error {
		// Update request
		res, err := tx.ExecContext(ctx, `
			UPDATE reconciliation_requests
			SET status = ?, direction = ?, actor = ?, updated_at = ?
			WHERE id = ? AND status = 'awaiting'`,
			string(RequestStatusApproved), approvedDir, params.Actor, now.UTC().Format(time.RFC3339), req.ID,
		)
		if err != nil {
			return fmt.Errorf("reconcile: update request status: %w", err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("reconcile: read affected rows: %w", err)
		}
		if affected == 0 {
			return ErrRequestNotAwaiting
		}

		// Insert candidate
		_, err = tx.ExecContext(ctx, `
			INSERT INTO reconciliation_candidates (
				id, request_id, status, allowed_paths, model, config,
				output, checks, candidate_sha, failure_reason, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			candRow.ID, candRow.RequestID, candRow.Status, candRow.AllowedPaths, candRow.Model,
			candRow.Config, candRow.Output, candRow.Checks, candRow.CandidateSHA, candRow.FailureReason,
			now.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339),
		)
		if err != nil {
			return fmt.Errorf("reconcile: insert candidate: %w", err)
		}
		return nil
	}, ledger.IntegrationEvent{
		FeatureID: req.FeatureID,
		Type:      "reconciliation_approved",
		Detail:    fmt.Sprintf("request_id=%s direction=%s actor=%s candidate_id=%s", req.ID, approvedDir, params.Actor, candID),
		At:        now,
	})
	if err != nil {
		if errors.Is(err, ErrRequestNotAwaiting) {
			return Request{}, Candidate{}, ErrRequestNotAwaiting
		}
		return Request{}, Candidate{}, fmt.Errorf("reconcile: write approval with audit: %w", err)
	}

	req.Status = RequestStatusApproved
	req.Direction = approvedDir
	req.Actor = params.Actor
	req.UpdatedAt = now

	return req, Candidate{
		ID:           candID,
		RequestID:    req.ID,
		Status:       CandidateStatusRunning,
		AllowedPaths: allowedPaths,
		Model:        "sonnet",
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// Decline records a decline decision on an unexpired awaiting reconciliation request without creating a candidate.
func (s *Service) Decline(ctx context.Context, requestID, actor, reason string) (Request, error) {
	req, err := s.GetRequest(ctx, requestID)
	if err != nil {
		return Request{}, err
	}

	now := s.clock()
	if req.Status == RequestStatusExpired || (!req.ExpiresAt.IsZero() && !now.Before(req.ExpiresAt)) {
		return Request{}, ErrRequestExpired
	}

	if req.Status != RequestStatusAwaiting {
		return Request{}, ErrRequestNotAwaiting
	}

	if strings.TrimSpace(actor) == "" {
		return Request{}, ErrEmptyActor
	}

	err = s.ledger.WriteWithAudit(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE reconciliation_requests
			SET status = ?, actor = ?, updated_at = ?
			WHERE id = ? AND status = 'awaiting'`,
			string(RequestStatusDeclined), actor, now.UTC().Format(time.RFC3339), req.ID,
		)
		if err != nil {
			return fmt.Errorf("reconcile: update request status to declined: %w", err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("reconcile: read affected rows: %w", err)
		}
		if affected == 0 {
			return ErrRequestNotAwaiting
		}
		return nil
	}, ledger.IntegrationEvent{
		FeatureID: req.FeatureID,
		Type:      "reconciliation_declined",
		Detail:    fmt.Sprintf("request_id=%s actor=%s reason=%s", req.ID, actor, reason),
		At:        now,
	})
	if err != nil {
		if errors.Is(err, ErrRequestNotAwaiting) {
			return Request{}, ErrRequestNotAwaiting
		}
		return Request{}, fmt.Errorf("reconcile: write decline with audit: %w", err)
	}

	req.Status = RequestStatusDeclined
	req.Actor = actor
	req.UpdatedAt = now

	return req, nil
}

// Cancel records a cancellation on an unexpired awaiting reconciliation request without creating a candidate.
func (s *Service) Cancel(ctx context.Context, requestID, actor, reason string) (Request, error) {
	req, err := s.GetRequest(ctx, requestID)
	if err != nil {
		return Request{}, err
	}

	now := s.clock()
	if req.Status == RequestStatusExpired || (!req.ExpiresAt.IsZero() && !now.Before(req.ExpiresAt)) {
		return Request{}, ErrRequestExpired
	}

	if req.Status != RequestStatusAwaiting {
		return Request{}, ErrRequestNotAwaiting
	}

	if strings.TrimSpace(actor) == "" {
		return Request{}, ErrEmptyActor
	}

	err = s.ledger.WriteWithAudit(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE reconciliation_requests
			SET status = ?, actor = ?, updated_at = ?
			WHERE id = ? AND status = 'awaiting'`,
			string(RequestStatusCancelled), actor, now.UTC().Format(time.RFC3339), req.ID,
		)
		if err != nil {
			return fmt.Errorf("reconcile: update request status to cancelled: %w", err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("reconcile: read affected rows: %w", err)
		}
		if affected == 0 {
			return ErrRequestNotAwaiting
		}
		return nil
	}, ledger.IntegrationEvent{
		FeatureID: req.FeatureID,
		Type:      "reconciliation_cancelled",
		Detail:    fmt.Sprintf("request_id=%s actor=%s reason=%s", req.ID, actor, reason),
		At:        now,
	})
	if err != nil {
		if errors.Is(err, ErrRequestNotAwaiting) {
			return Request{}, ErrRequestNotAwaiting
		}
		return Request{}, fmt.Errorf("reconcile: write cancel with audit: %w", err)
	}

	req.Status = RequestStatusCancelled
	req.Actor = actor
	req.UpdatedAt = now

	return req, nil
}

// RenewParams holds parameters for renewing an expired or stale reconciliation request.
type RenewParams struct {
	OldRequestID     string
	RepoDir          string
	BaseSHA          string
	CurrentSourceSHA string
	CurrentTargetSHA string
	TTL              time.Duration
	NewRequestID     string
}

// Renew recomputes fresh overlap evidence via overlap evaluation and creates a new awaiting reconciliation request,
// marking the old request expired.
func (s *Service) Renew(ctx context.Context, params RenewParams) (Request, error) {
	oldReq, err := s.GetRequest(ctx, params.OldRequestID)
	if err != nil {
		return Request{}, err
	}

	baseSHA := params.BaseSHA
	if baseSHA == "" && oldReq.Evidence != nil {
		baseSHA = oldReq.Evidence.BaseSHA
	}

	sourceSHA := params.CurrentSourceSHA
	if sourceSHA == "" {
		sourceSHA = oldReq.SourceSHA
	}

	targetSHA := params.CurrentTargetSHA
	if targetSHA == "" {
		targetSHA = oldReq.TargetSHA
	}

	now := s.clock()

	// Call the overlap classification evaluator to recompute fresh evidence
	freshEv, err := s.evaluator(ctx, params.RepoDir, baseSHA, sourceSHA, targetSHA, overlap.WithClock(func() time.Time { return now }))
	if err != nil {
		return Request{}, fmt.Errorf("reconcile: evaluate fresh overlap evidence: %w", err)
	}

	if freshEv.Class != overlap.ClassRequired {
		return Request{}, ErrRequiredOverlapOnly
	}

	newID := params.NewRequestID
	if newID == "" {
		newID = s.idSource()
	}

	ttl := params.TTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	expiresAt := now.Add(ttl)

	evJSON, err := freshEv.JSON()
	if err != nil {
		return Request{}, fmt.Errorf("reconcile: marshal fresh evidence: %w", err)
	}

	evHash := freshEv.Hash
	if evHash == "" {
		evHash, err = freshEv.ComputeHash()
		if err != nil {
			return Request{}, fmt.Errorf("reconcile: compute fresh evidence hash: %w", err)
		}
	}

	_, err = s.ledger.InsertOverlapEvidence(ctx, ledger.OverlapEvidenceRow{
		FeatureID:     oldReq.FeatureID,
		Version:       freshEv.Version,
		EvidenceHash:  evHash,
		EvidenceClass: string(freshEv.Class),
		EvidenceJSON:  evJSON,
		CreatedAt:     now,
	})
	if err != nil {
		return Request{}, fmt.Errorf("reconcile: save fresh overlap evidence: %w", err)
	}

	dir := FormatDirection(oldReq.SourceFeature, oldReq.SourceParent, oldReq.TargetFeature, oldReq.TargetParent)

	newReqRow := ledger.ReconciliationRequestRow{
		ID:              newID,
		FeatureID:       oldReq.FeatureID,
		Direction:       dir,
		Status:          string(RequestStatusAwaiting),
		Actor:           "",
		EvidenceVersion: freshEv.Version,
		EvidenceHash:    evHash,
		SourceSHA:       sourceSHA,
		TargetSHA:       targetSHA,
		ExpiresAt:       &expiresAt,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	err = s.ledger.WriteWithAudit(ctx, func(tx *sql.Tx) error {
		// Mark old request expired if it was awaiting OR already approved. A
		// renewed request supersedes the old one either way: an awaiting
		// request that never got a decision, or an approved request whose
		// underlying SHAs have since gone stale (e.g. the other feature
		// promoted again after approval, before a candidate was resolved).
		// Leaving a stale *approved* request active would let it stay
		// simultaneously "approved" alongside the freshly renewed request
		// for the same direction; evaluateOverlapGate's overlap-request
		// lookup takes the first created_at match for a direction, so the
		// older, stale-SHA request would keep shadowing the new one's
		// resolved candidate forever, permanently reproducing the same
		// reconciliation-required block after every retry.
		_, err := tx.ExecContext(ctx, `
			UPDATE reconciliation_requests
			SET status = ?, updated_at = ?
			WHERE id = ? AND status IN ('awaiting', 'approved')`,
			string(RequestStatusExpired), now.UTC().Format(time.RFC3339), oldReq.ID,
		)
		if err != nil {
			return fmt.Errorf("reconcile: expire old request: %w", err)
		}

		// Insert new request
		_, err = tx.ExecContext(ctx, `
			INSERT INTO reconciliation_requests (
				id, feature_id, direction, status, actor, evidence_version,
				evidence_hash, source_sha, target_sha, expires_at, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			newReqRow.ID, newReqRow.FeatureID, newReqRow.Direction, newReqRow.Status, newReqRow.Actor,
			newReqRow.EvidenceVersion, newReqRow.EvidenceHash, newReqRow.SourceSHA, newReqRow.TargetSHA,
			newReqRow.ExpiresAt.UTC().Format(time.RFC3339),
			now.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339),
		)
		if err != nil {
			return fmt.Errorf("reconcile: insert renewed request: %w", err)
		}
		return nil
	}, ledger.IntegrationEvent{
		FeatureID: oldReq.FeatureID,
		Type:      "reconciliation_renewed",
		Detail:    fmt.Sprintf("old_request_id=%s new_request_id=%s evidence_hash=%s", oldReq.ID, newID, evHash),
		At:        now,
	})
	if err != nil {
		return Request{}, fmt.Errorf("reconcile: write renewal with audit: %w", err)
	}

	return Request{
		ID:              newID,
		FeatureID:       oldReq.FeatureID,
		SourceFeature:   oldReq.SourceFeature,
		SourceParent:    oldReq.SourceParent,
		TargetFeature:   oldReq.TargetFeature,
		TargetParent:    oldReq.TargetParent,
		Direction:       dir,
		Status:          RequestStatusAwaiting,
		EvidenceVersion: freshEv.Version,
		EvidenceHash:    evHash,
		SourceSHA:       sourceSHA,
		TargetSHA:       targetSHA,
		Evidence:        freshEv,
		ExpiresAt:       expiresAt,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

// GetCandidate retrieves a candidate by candidate ID.
func (s *Service) GetCandidate(ctx context.Context, id string) (Candidate, error) {
	row, err := s.ledger.ReconciliationCandidate(ctx, id)
	if err != nil {
		if errors.Is(err, ledger.ErrReconciliationCandidateNotFound) {
			return Candidate{}, ErrCandidateNotFound
		}
		return Candidate{}, fmt.Errorf("reconcile: query candidate: %w", err)
	}
	return toCandidate(row), nil
}

// GetCandidateByRequest retrieves the candidate associated with a reconciliation request ID.
func (s *Service) GetCandidateByRequest(ctx context.Context, requestID string) (Candidate, error) {
	row, err := s.ledger.ReconciliationCandidateByRequest(ctx, requestID)
	if err != nil {
		if errors.Is(err, ledger.ErrReconciliationCandidateNotFound) {
			return Candidate{}, ErrCandidateNotFound
		}
		return Candidate{}, fmt.Errorf("reconcile: query candidate by request: %w", err)
	}
	return toCandidate(row), nil
}

// CandidatesByRequest retrieves all candidate records for a reconciliation request ID.
func (s *Service) CandidatesByRequest(ctx context.Context, requestID string) ([]Candidate, error) {
	rows, err := s.ledger.ReconciliationCandidates(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("reconcile: query candidates: %w", err)
	}
	var out []Candidate
	for _, r := range rows {
		out = append(out, toCandidate(r))
	}
	return out, nil
}

// UpdateCandidateStatus transitions a candidate's status (candidate_running -> integrated|failed|stale)
// and updates candidate_sha or failure_reason with atomic audit logging.
func (s *Service) UpdateCandidateStatus(ctx context.Context, candidateID string, status CandidateStatus, candidateSHA, failureReason string) (Candidate, error) {
	if !status.Valid() {
		return Candidate{}, fmt.Errorf("reconcile: invalid candidate status %q", status)
	}

	cand, err := s.GetCandidate(ctx, candidateID)
	if err != nil {
		return Candidate{}, err
	}

	// Validate transition: only candidate_running can transition to a terminal state
	if cand.Status != CandidateStatusRunning {
		return Candidate{}, ErrInvalidTransition
	}

	now := s.clock()

	req, err := s.GetRequest(ctx, cand.RequestID)
	featureID := ""
	if err == nil {
		featureID = req.FeatureID
	}

	err = s.ledger.WriteWithAudit(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE reconciliation_candidates
			SET status = ?, candidate_sha = ?, failure_reason = ?, updated_at = ?
			WHERE id = ? AND status = 'candidate_running'`,
			string(status), candidateSHA, failureReason, now.UTC().Format(time.RFC3339), cand.ID,
		)
		if err != nil {
			return fmt.Errorf("reconcile: update candidate: %w", err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("reconcile: read rows affected: %w", err)
		}
		if affected == 0 {
			return ErrInvalidTransition
		}
		return nil
	}, ledger.IntegrationEvent{
		FeatureID: featureID,
		Type:      "reconciliation_candidate_updated",
		Detail:    fmt.Sprintf("candidate_id=%s status=%s candidate_sha=%s failure_reason=%s", cand.ID, status, candidateSHA, failureReason),
		At:        now,
	})
	if err != nil {
		if errors.Is(err, ErrInvalidTransition) {
			return Candidate{}, ErrInvalidTransition
		}
		return Candidate{}, fmt.Errorf("reconcile: write candidate update with audit: %w", err)
	}

	cand.Status = status
	cand.CandidateSHA = candidateSHA
	cand.FailureReason = failureReason
	cand.UpdatedAt = now

	return cand, nil
}

// UpdateCandidateOutput persists JSON into Candidate.Output without changing
// status or CandidateSHA. UpdateCandidateStatus SQL does not touch output.
func (s *Service) UpdateCandidateOutput(ctx context.Context, candidateID, output string) (Candidate, error) {
	cand, err := s.GetCandidate(ctx, candidateID)
	if err != nil {
		return Candidate{}, err
	}

	now := s.clock()
	row := ledger.ReconciliationCandidateRow{
		ID:            cand.ID,
		RequestID:     cand.RequestID,
		Status:        string(cand.Status),
		AllowedPaths:  strings.Join(cand.AllowedPaths, ","),
		Model:         cand.Model,
		Config:        cand.Config,
		Output:        output,
		Checks:        cand.Checks,
		CandidateSHA:  cand.CandidateSHA,
		FailureReason: cand.FailureReason,
		CreatedAt:     cand.CreatedAt,
		UpdatedAt:     now,
	}
	if err := s.ledger.UpdateReconciliationCandidate(ctx, row); err != nil {
		if errors.Is(err, ledger.ErrReconciliationCandidateNotFound) {
			return Candidate{}, ErrCandidateNotFound
		}
		return Candidate{}, fmt.Errorf("reconcile: update candidate output: %w", err)
	}

	updated, err := s.GetCandidate(ctx, candidateID)
	if err != nil {
		return Candidate{}, err
	}
	return updated, nil
}

func toCandidate(r ledger.ReconciliationCandidateRow) Candidate {
	var allowedPaths []string
	if r.AllowedPaths != "" {
		for _, p := range strings.Split(r.AllowedPaths, ",") {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				allowedPaths = append(allowedPaths, trimmed)
			}
		}
	}
	return Candidate{
		ID:            r.ID,
		RequestID:     r.RequestID,
		Status:        CandidateStatus(r.Status),
		AllowedPaths:  allowedPaths,
		Model:         r.Model,
		Config:        r.Config,
		Output:        r.Output,
		Checks:        r.Checks,
		CandidateSHA:  r.CandidateSHA,
		FailureReason: r.FailureReason,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
}
