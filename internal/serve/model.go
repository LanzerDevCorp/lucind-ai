package serve

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/barrier"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
)

// Model is the shell-free status and audit query surface for feature-parent
// integration state. It reads the ledger and returns JSON-facing structs for a
// future localhost UI; it does not run git or shell commands.
type Model struct {
	ledger *ledger.Ledger
}

// NewModel constructs a status/audit query Model backed by l.
func NewModel(l *ledger.Ledger) *Model {
	return &Model{ledger: l}
}

// Feature is the JSON payload for one features row.
type Feature struct {
	ID                string    `json:"id"`
	ParentRef         string    `json:"parent_ref"`
	BaseSHA           string    `json:"base_sha"`
	ExpectedParentSHA string    `json:"expected_parent_sha"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// Attempt is the JSON payload for one integration_attempts row.
type Attempt struct {
	ID             string    `json:"id"`
	FeatureID      string    `json:"feature_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	Status         string    `json:"status"`
	Owner          string    `json:"owner"`
	Fence          int64     `json:"fence"`
	CandidateSHA   string    `json:"candidate_sha"`
	FailureReason  string    `json:"failure_reason"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Lease is the JSON payload for one feature_leases row.
type Lease struct {
	FeatureID  string    `json:"feature_id"`
	Owner      string    `json:"owner"`
	Fence      int64     `json:"fence"`
	ExpiresAt  time.Time `json:"expires_at"`
	AcquiredAt time.Time `json:"acquired_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// OverlapEvidence is the JSON payload for one overlap_evidence row.
type OverlapEvidence struct {
	ID            int64     `json:"id"`
	FeatureID     string    `json:"feature_id"`
	Version       string    `json:"version"`
	EvidenceHash  string    `json:"evidence_hash"`
	EvidenceClass string    `json:"evidence_class"`
	EvidenceJSON  string    `json:"evidence_json"`
	CreatedAt     time.Time `json:"created_at"`
}

// ReconciliationRequest is the JSON payload for one reconciliation request and
// its observable candidate and audit history.
type ReconciliationRequest struct {
	ID              string                    `json:"id"`
	FeatureID       string                    `json:"feature_id"`
	Direction       string                    `json:"direction"`
	Status          string                    `json:"status"`
	Actor           string                    `json:"actor"`
	EvidenceVersion string                    `json:"evidence_version"`
	EvidenceHash    string                    `json:"evidence_hash"`
	SourceSHA       string                    `json:"source_sha"`
	TargetSHA       string                    `json:"target_sha"`
	ExpiresAt       *time.Time                `json:"expires_at,omitempty"`
	CreatedAt       time.Time                 `json:"created_at"`
	UpdatedAt       time.Time                 `json:"updated_at"`
	Candidates      []ReconciliationCandidate `json:"candidates"`
	Audit           []AuditEvent              `json:"audit"`
	CheckOutcomes   string                    `json:"check_outcomes"`
	Failures        string                    `json:"failures"`
	CASResult       CASResult                 `json:"cas_result"`
}

// CASResult is the observable compare-and-swap outcome for a reconciliation record.
type CASResult struct {
	Outcome       string `json:"outcome"`
	CandidateSHA  string `json:"candidate_sha,omitempty"`
	FailureReason string `json:"failure_reason,omitempty"`
}

// ReconciliationCandidate is the JSON payload for one reconciliation_candidates row.
type ReconciliationCandidate struct {
	ID            string    `json:"id"`
	RequestID     string    `json:"request_id"`
	Status        string    `json:"status"`
	AllowedPaths  []string  `json:"allowed_paths"`
	Model         string    `json:"model"`
	Config        string    `json:"config"`
	Output        string    `json:"output"`
	Checks        string    `json:"checks"`
	CandidateSHA  string    `json:"candidate_sha"`
	FailureReason string    `json:"failure_reason"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// AuditEvent is the JSON payload for one integration_events row.
type AuditEvent struct {
	ID        int64     `json:"id"`
	FeatureID string    `json:"feature_id"`
	AttemptID string    `json:"attempt_id,omitempty"`
	Type      string    `json:"type"`
	Detail    string    `json:"detail"`
	At        time.Time `json:"at"`
}

// LaneStatusCounts is a stable JSON rollup of the six lane states.
type LaneStatusCounts struct {
	Pending  int `json:"pending"`
	Running  int `json:"running"`
	Done     int `json:"done"`
	Blocked  int `json:"blocked"`
	Deviated int `json:"deviated"`
	Failed   int `json:"failed"`
}

// RunSummary is the JSON payload for one run and its derived lane/approval counts.
type RunSummary struct {
	RunID            string           `json:"run_id"`
	FeatureID        string           `json:"feature_id"`
	Status           string           `json:"status"`
	TargetRef        string           `json:"target_ref"`
	LaneCount        int              `json:"lane_count"`
	StartedAt        time.Time        `json:"started_at"`
	EndedAt          *time.Time       `json:"ended_at"`
	LaneStatusCounts LaneStatusCounts `json:"lane_status_counts"`
	PendingApprovals int              `json:"pending_approvals"`
}

// Lane is the JSON payload for one lane plus its audited dispatch metadata.
type Lane struct {
	RunID             string     `json:"run_id"`
	LaneID            string     `json:"lane_id"`
	PacketID          string     `json:"packet_id"`
	Executor          string     `json:"executor"`
	RoutingCondition  string     `json:"routing_condition"`
	Status            string     `json:"status"`
	WorktreePath      string     `json:"worktree_path"`
	WorktreePreserved bool       `json:"worktree_preserved"`
	Attempt           int        `json:"attempt"`
	StartedAt         *time.Time `json:"started_at"`
	EndedAt           *time.Time `json:"ended_at"`
	Model             string     `json:"model"`
	Agent             string     `json:"agent"`
	Feature           string     `json:"feature"`
	SDDPhase          string     `json:"sdd_phase"`
	FanoutGroup       string     `json:"fanout_group"`
	Change            string     `json:"change"`
	AllowedPaths      []string   `json:"allowed_paths"`
	Dependencies      []string   `json:"dependencies"`
	BodyDigest        string     `json:"body_digest"`
}

// LaneProgress is one JSON-facing cursor-tail item.
type LaneProgress struct {
	RunID   string    `json:"run_id"`
	LaneID  string    `json:"lane_id"`
	Seq     int64     `json:"seq"`
	Message string    `json:"message"`
	At      time.Time `json:"at"`
}

// BatchOutcome is the JSON-facing barrier result shared by a run's batch lanes.
type BatchOutcome struct {
	Released  bool     `json:"released"`
	Integrate []string `json:"integrate"`
	Preserve  []string `json:"preserve"`
}

// BatchLane adds the latest diagnostic note and derived barrier outcome to Lane.
type BatchLane struct {
	Lane
	Note    string       `json:"note"`
	Outcome BatchOutcome `json:"outcome"`
}

// SDDFlow groups lanes by run and audited SDD dispatch context.
type SDDFlow struct {
	RunID       string   `json:"run_id"`
	Change      string   `json:"change"`
	SDDPhase    string   `json:"sdd_phase"`
	FanoutGroup string   `json:"fanout_group"`
	Status      string   `json:"status"`
	LaneCount   int      `json:"lane_count"`
	LaneIDs     []string `json:"lane_ids"`
}

// Overview is the derived top-level run, lane, approval, and flow rollup.
type Overview struct {
	Status           string           `json:"status"`
	RunCount         int              `json:"run_count"`
	ActiveRunCount   int              `json:"active_run_count"`
	LaneCount        int              `json:"lane_count"`
	LaneStatusCounts LaneStatusCounts `json:"lane_status_counts"`
	PendingApprovals int              `json:"pending_approvals"`
	FlowCount        int              `json:"flow_count"`
}

// GetRun returns one run summary. Unknown run IDs preserve ledger.ErrRunUnknown.
func (m *Model) GetRun(ctx context.Context, runID string) (RunSummary, error) {
	run, err := m.ledger.GetRun(ctx, runID)
	if err != nil {
		return RunSummary{}, fmt.Errorf("serve: get run: %w", err)
	}
	return m.summarizeRun(ctx, run)
}

// GetRunSummary is the aggregate-oriented name for GetRun.
func (m *Model) GetRunSummary(ctx context.Context, runID string) (RunSummary, error) {
	return m.GetRun(ctx, runID)
}

// ListRuns returns run summaries in the ledger's newest-first stable order.
func (m *Model) ListRuns(ctx context.Context) ([]RunSummary, error) {
	runs, err := m.ledger.ListRuns(ctx)
	if err != nil {
		return nil, fmt.Errorf("serve: list runs: %w", err)
	}
	out := make([]RunSummary, 0, len(runs))
	for _, run := range runs {
		summary, err := m.summarizeRun(ctx, run)
		if err != nil {
			return nil, err
		}
		out = append(out, summary)
	}
	return out, nil
}

func (m *Model) summarizeRun(ctx context.Context, run ledger.Run) (RunSummary, error) {
	lanes, err := m.ListLanes(ctx, run.RunID)
	if err != nil {
		return RunSummary{}, err
	}
	approvals, err := m.ledger.Approvals(ctx, run.RunID)
	if err != nil {
		return RunSummary{}, fmt.Errorf("serve: list run approvals: %w", err)
	}
	summary := RunSummary{
		RunID: run.RunID, FeatureID: run.FeatureID, Status: run.Status,
		TargetRef: run.TargetRef, LaneCount: run.LaneCount,
		StartedAt: run.StartedAt, EndedAt: run.EndedAt,
	}
	for _, ln := range lanes {
		addLaneStatus(&summary.LaneStatusCounts, lane.Status(ln.Status))
	}
	for _, approval := range approvals {
		if approval.Decision == ledger.DecisionPending {
			summary.PendingApprovals++
		}
	}
	return summary, nil
}

// ListLanes returns a run's lanes ordered by lane_id with non-nil metadata lists.
func (m *Model) ListLanes(ctx context.Context, runID string) ([]Lane, error) {
	rows, err := m.ledger.Lanes(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("serve: list lanes: %w", err)
	}
	out := make([]Lane, 0, len(rows))
	for _, row := range rows {
		metadata, err := m.ledger.GetLaneMetadata(ctx, row.RunID, row.LaneID)
		if err != nil {
			return nil, fmt.Errorf("serve: get lane metadata: %w", err)
		}
		out = append(out, laneDTO(row, metadata))
	}
	return out, nil
}

// GetLane returns one lane or ledger.ErrLaneUnknown for an unknown identity.
func (m *Model) GetLane(ctx context.Context, runID, laneID string) (Lane, error) {
	metadata, err := m.ledger.GetLaneMetadata(ctx, runID, laneID)
	if err != nil {
		return Lane{}, fmt.Errorf("serve: get lane: %w", err)
	}
	rows, err := m.ledger.Lanes(ctx, runID)
	if err != nil {
		return Lane{}, fmt.Errorf("serve: list lanes: %w", err)
	}
	for _, row := range rows {
		if row.LaneID == laneID {
			return laneDTO(row, metadata), nil
		}
	}
	return Lane{}, ledger.ErrLaneUnknown
}

func laneDTO(row ledger.Lane, metadata ledger.LaneMetadata) Lane {
	return Lane{
		RunID: row.RunID, LaneID: row.LaneID, PacketID: row.PacketID,
		Executor: row.Executor, RoutingCondition: row.RoutingCondition, Status: string(row.Status),
		WorktreePath: row.WorktreePath, WorktreePreserved: row.WorktreePreserved,
		Attempt: row.Attempt, StartedAt: row.StartedAt, EndedAt: row.EndedAt,
		Model: metadata.Model, Agent: metadata.Agent, Feature: metadata.Feature,
		SDDPhase: metadata.SDDPhase, FanoutGroup: metadata.FanoutGroup, Change: metadata.Change,
		AllowedPaths: nonNilStrings(metadata.AllowedPaths), Dependencies: nonNilStrings(metadata.Dependencies),
		BodyDigest: metadata.BodyDigest,
	}
}

// GetLaneProgress returns seq > afterSeq in ascending cursor order.
func (m *Model) GetLaneProgress(ctx context.Context, runID, laneID string, afterSeq int64) ([]LaneProgress, error) {
	rows, err := m.ledger.GetProgressAfter(ctx, runID, laneID, afterSeq)
	if err != nil {
		return nil, fmt.Errorf("serve: get lane progress: %w", err)
	}
	out := make([]LaneProgress, 0, len(rows))
	for _, row := range rows {
		out = append(out, LaneProgress{RunID: row.RunID, LaneID: row.LaneID, Seq: row.Seq, Message: row.Message, At: row.At})
	}
	return out, nil
}

// ListBatchLanes derives latest lane notes and the run's barrier outcome.
func (m *Model) ListBatchLanes(ctx context.Context, runID string) ([]BatchLane, error) {
	lanes, err := m.ListLanes(ctx, runID)
	if err != nil {
		return nil, err
	}
	events, err := m.ledger.Events(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("serve: list lane events: %w", err)
	}
	notes := make(map[string]string)
	for _, event := range events {
		if event.Type == ledger.EventLaneNote && !strings.HasPrefix(event.Detail, "lane_metadata:v1:") {
			notes[event.LaneID] = event.Detail
		}
	}
	expected := make([]string, 0, len(lanes))
	states := make([]lane.State, 0, len(lanes))
	for _, ln := range lanes {
		expected = append(expected, ln.LaneID)
		states = append(states, lane.State{LaneID: ln.LaneID, Status: lane.Status(ln.Status)})
	}
	result := barrier.Evaluate(expected, states)
	outcome := BatchOutcome{Released: result.Released, Integrate: nonNilStrings(result.Integrate), Preserve: nonNilStrings(result.Preserve)}
	out := make([]BatchLane, 0, len(lanes))
	for _, ln := range lanes {
		out = append(out, BatchLane{Lane: ln, Note: notes[ln.LaneID], Outcome: outcome})
	}
	return out, nil
}

// ListSDDFlows groups lanes by run and audited change/phase/fanout metadata.
func (m *Model) ListSDDFlows(ctx context.Context) ([]SDDFlow, error) {
	runs, err := m.ledger.ListRuns(ctx)
	if err != nil {
		return nil, fmt.Errorf("serve: list flow runs: %w", err)
	}
	type flowKey struct{ runID, change, phase, fanout string }
	out := make([]SDDFlow, 0)
	indexes := make(map[flowKey]int)
	statuses := make(map[flowKey][]lane.Status)
	for _, run := range runs {
		lanes, err := m.ListLanes(ctx, run.RunID)
		if err != nil {
			return nil, err
		}
		for _, ln := range lanes {
			key := flowKey{run.RunID, ln.Change, ln.SDDPhase, ln.FanoutGroup}
			index, ok := indexes[key]
			if !ok {
				index = len(out)
				indexes[key] = index
				out = append(out, SDDFlow{RunID: key.runID, Change: key.change, SDDPhase: key.phase, FanoutGroup: key.fanout, LaneIDs: []string{}})
			}
			out[index].LaneCount++
			out[index].LaneIDs = append(out[index].LaneIDs, ln.LaneID)
			statuses[key] = append(statuses[key], lane.Status(ln.Status))
		}
	}
	for key, index := range indexes {
		out[index].Status = rollupStatus(statuses[key])
	}
	return out, nil
}

// GetOverview composes existing run and flow methods into one aggregate payload.
func (m *Model) GetOverview(ctx context.Context) (Overview, error) {
	runs, err := m.ListRuns(ctx)
	if err != nil {
		return Overview{}, err
	}
	flows, err := m.ListSDDFlows(ctx)
	if err != nil {
		return Overview{}, err
	}
	overview := Overview{RunCount: len(runs), FlowCount: len(flows)}
	for _, run := range runs {
		if run.Status == string(lane.Running) {
			overview.ActiveRunCount++
		}
		overview.LaneCount += run.LaneStatusCounts.total()
		overview.LaneStatusCounts.add(run.LaneStatusCounts)
		overview.PendingApprovals += run.PendingApprovals
	}
	overview.Status = statusFromCounts(overview.LaneStatusCounts)
	return overview, nil
}

func addLaneStatus(counts *LaneStatusCounts, status lane.Status) {
	switch status {
	case lane.Pending:
		counts.Pending++
	case lane.Running:
		counts.Running++
	case lane.Done:
		counts.Done++
	case lane.Blocked:
		counts.Blocked++
	case lane.Deviated:
		counts.Deviated++
	case lane.Failed:
		counts.Failed++
	}
}

func (c *LaneStatusCounts) add(other LaneStatusCounts) {
	c.Pending += other.Pending
	c.Running += other.Running
	c.Done += other.Done
	c.Blocked += other.Blocked
	c.Deviated += other.Deviated
	c.Failed += other.Failed
}

func (c LaneStatusCounts) total() int {
	return c.Pending + c.Running + c.Done + c.Blocked + c.Deviated + c.Failed
}

func rollupStatus(statuses []lane.Status) string {
	var counts LaneStatusCounts
	for _, status := range statuses {
		addLaneStatus(&counts, status)
	}
	return statusFromCounts(counts)
}

func statusFromCounts(counts LaneStatusCounts) string {
	switch {
	case counts.Failed > 0:
		return string(lane.Failed)
	case counts.Deviated > 0:
		return string(lane.Deviated)
	case counts.Blocked > 0:
		return string(lane.Blocked)
	case counts.Running > 0:
		return string(lane.Running)
	case counts.Pending > 0:
		return string(lane.Pending)
	case counts.Done > 0:
		return string(lane.Done)
	default:
		return "empty"
	}
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// ListFeatures returns every feature row ordered by created_at.
func (m *Model) ListFeatures(ctx context.Context) ([]Feature, error) {
	rows, err := m.ledger.DB().QueryContext(ctx, `
		SELECT id, parent_ref, base_sha, expected_parent_sha, status, created_at, updated_at
		FROM features ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("serve: list features: %w", err)
	}
	defer rows.Close()

	out := []Feature{}
	for rows.Next() {
		f, err := scanFeature(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("serve: iterate features: %w", err)
	}
	return out, nil
}

// GetFeature returns the feature row with the given id.
func (m *Model) GetFeature(ctx context.Context, id string) (Feature, error) {
	row := m.ledger.DB().QueryRowContext(ctx, `
		SELECT id, parent_ref, base_sha, expected_parent_sha, status, created_at, updated_at
		FROM features WHERE id = ?`, id)
	f, err := scanFeature(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Feature{}, fmt.Errorf("serve: feature %q: %w", id, err)
		}
		return Feature{}, err
	}
	return f, nil
}

// ListAttempts returns integration attempts for featureID ordered by created_at.
func (m *Model) ListAttempts(ctx context.Context, featureID string) ([]Attempt, error) {
	rows, err := m.ledger.DB().QueryContext(ctx, `
		SELECT id, feature_id, idempotency_key, status, owner, fence, candidate_sha, failure_reason, created_at, updated_at
		FROM integration_attempts WHERE feature_id = ? ORDER BY created_at ASC`, featureID)
	if err != nil {
		return nil, fmt.Errorf("serve: list attempts: %w", err)
	}
	defer rows.Close()

	out := []Attempt{}
	for rows.Next() {
		a, err := scanAttempt(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("serve: iterate attempts: %w", err)
	}
	return out, nil
}

// GetAttempt returns the integration attempt with the given id.
func (m *Model) GetAttempt(ctx context.Context, id string) (Attempt, error) {
	row := m.ledger.DB().QueryRowContext(ctx, `
		SELECT id, feature_id, idempotency_key, status, owner, fence, candidate_sha, failure_reason, created_at, updated_at
		FROM integration_attempts WHERE id = ?`, id)
	a, err := scanAttempt(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Attempt{}, fmt.Errorf("serve: attempt %q: %w", id, err)
		}
		return Attempt{}, err
	}
	return a, nil
}

// ListLeases returns every feature lease.
func (m *Model) ListLeases(ctx context.Context) ([]Lease, error) {
	rows, err := m.ledger.DB().QueryContext(ctx, `
		SELECT feature_id, owner, fence, expires_at, acquired_at, updated_at
		FROM feature_leases ORDER BY acquired_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("serve: list leases: %w", err)
	}
	defer rows.Close()

	out := []Lease{}
	for rows.Next() {
		lease, err := scanLease(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, lease)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("serve: iterate leases: %w", err)
	}
	return out, nil
}

// GetLease returns the lease for featureID.
func (m *Model) GetLease(ctx context.Context, featureID string) (Lease, error) {
	row := m.ledger.DB().QueryRowContext(ctx, `
		SELECT feature_id, owner, fence, expires_at, acquired_at, updated_at
		FROM feature_leases WHERE feature_id = ?`, featureID)
	lease, err := scanLease(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Lease{}, fmt.Errorf("serve: lease %q: %w", featureID, err)
		}
		return Lease{}, err
	}
	return lease, nil
}

// ListOverlapEvidence returns overlap evidence rows for featureID ordered by id.
func (m *Model) ListOverlapEvidence(ctx context.Context, featureID string) ([]OverlapEvidence, error) {
	rows, err := m.ledger.DB().QueryContext(ctx, `
		SELECT id, feature_id, version, evidence_hash, evidence_class, evidence_json, created_at
		FROM overlap_evidence WHERE feature_id = ? ORDER BY id ASC`, featureID)
	if err != nil {
		return nil, fmt.Errorf("serve: list overlap evidence: %w", err)
	}
	defer rows.Close()

	out := []OverlapEvidence{}
	for rows.Next() {
		ov, err := scanOverlap(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ov)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("serve: iterate overlap evidence: %w", err)
	}
	return out, nil
}

// GetOverlapEvidence returns the overlap evidence row for featureID and hash.
func (m *Model) GetOverlapEvidence(ctx context.Context, featureID, evidenceHash string) (OverlapEvidence, error) {
	row, err := m.ledger.OverlapEvidence(ctx, featureID, evidenceHash)
	if err != nil {
		return OverlapEvidence{}, err
	}
	return overlapFromRow(row), nil
}

// ListReconciliationRequests returns reconciliation requests for featureID.
func (m *Model) ListReconciliationRequests(ctx context.Context, featureID string) ([]ReconciliationRequest, error) {
	rows, err := m.ledger.ReconciliationRequests(ctx, featureID)
	if err != nil {
		return nil, err
	}
	out := make([]ReconciliationRequest, 0, len(rows))
	for _, row := range rows {
		req, err := m.assembleRequest(ctx, row)
		if err != nil {
			return nil, err
		}
		out = append(out, req)
	}
	return out, nil
}

// GetReconciliationRequest returns one reconciliation request with candidates and audit.
func (m *Model) GetReconciliationRequest(ctx context.Context, id string) (ReconciliationRequest, error) {
	row, err := m.ledger.ReconciliationRequest(ctx, id)
	if err != nil {
		return ReconciliationRequest{}, err
	}
	return m.assembleRequest(ctx, row)
}

// ListReconciliationCandidates returns candidates for requestID ordered by created_at.
func (m *Model) ListReconciliationCandidates(ctx context.Context, requestID string) ([]ReconciliationCandidate, error) {
	rows, err := m.ledger.ReconciliationCandidates(ctx, requestID)
	if err != nil {
		return nil, err
	}
	out := make([]ReconciliationCandidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, candidateFromRow(row))
	}
	return out, nil
}

// GetReconciliationCandidate returns the candidate with the given id.
func (m *Model) GetReconciliationCandidate(ctx context.Context, id string) (ReconciliationCandidate, error) {
	row, err := m.ledger.ReconciliationCandidate(ctx, id)
	if err != nil {
		return ReconciliationCandidate{}, err
	}
	return candidateFromRow(row), nil
}

// ListAuditEvents returns integration audit events for featureID ordered by id.
func (m *Model) ListAuditEvents(ctx context.Context, featureID string) ([]AuditEvent, error) {
	rows, err := m.ledger.IntegrationEvents(ctx, featureID)
	if err != nil {
		return nil, err
	}
	out := make([]AuditEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, AuditEvent{
			ID:        row.ID,
			FeatureID: row.FeatureID,
			AttemptID: row.AttemptID,
			Type:      row.Type,
			Detail:    row.Detail,
			At:        row.At,
		})
	}
	return out, nil
}

func (m *Model) assembleRequest(ctx context.Context, row ledger.ReconciliationRequestRow) (ReconciliationRequest, error) {
	cands, err := m.ListReconciliationCandidates(ctx, row.ID)
	if err != nil {
		return ReconciliationRequest{}, err
	}
	audit, err := m.ListAuditEvents(ctx, row.FeatureID)
	if err != nil {
		return ReconciliationRequest{}, err
	}
	status := row.Status
	if status == "awaiting" && row.ExpiresAt != nil && !time.Now().UTC().Before(*row.ExpiresAt) {
		status = "expired"
	}
	req := ReconciliationRequest{
		ID:              row.ID,
		FeatureID:       row.FeatureID,
		Direction:       row.Direction,
		Status:          status,
		Actor:           row.Actor,
		EvidenceVersion: row.EvidenceVersion,
		EvidenceHash:    row.EvidenceHash,
		SourceSHA:       row.SourceSHA,
		TargetSHA:       row.TargetSHA,
		ExpiresAt:       row.ExpiresAt,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		Candidates:      cands,
		Audit:           audit,
	}
	if req.Candidates == nil {
		req.Candidates = []ReconciliationCandidate{}
	}
	if req.Audit == nil {
		req.Audit = []AuditEvent{}
	}
	req.CheckOutcomes = joinCheckOutcomes(req.Candidates)
	req.Failures = deriveFailures(req.Status, req.Candidates, req.Audit)
	req.CASResult = deriveCASResult(req.Candidates)
	return req, nil
}

func joinCheckOutcomes(cands []ReconciliationCandidate) string {
	parts := make([]string, 0, len(cands))
	for _, c := range cands {
		if c.Checks != "" {
			parts = append(parts, c.Checks)
		}
	}
	return strings.Join(parts, "\n")
}

func deriveFailures(status string, cands []ReconciliationCandidate, audit []AuditEvent) string {
	parts := make([]string, 0)
	for _, c := range cands {
		if c.FailureReason != "" {
			parts = append(parts, c.FailureReason)
		}
	}
	if status == "declined" {
		for _, e := range audit {
			if e.Type != "reconciliation_declined" {
				continue
			}
			if reason := parseDetailReason(e.Detail); reason != "" {
				parts = append(parts, reason)
			} else if e.Detail != "" {
				parts = append(parts, e.Detail)
			}
		}
	}
	if status == "expired" && len(parts) == 0 {
		return "expired"
	}
	return strings.Join(parts, "; ")
}

func parseDetailReason(detail string) string {
	const prefix = "reason="
	if i := strings.Index(detail, prefix); i >= 0 {
		return detail[i+len(prefix):]
	}
	return ""
}

func deriveCASResult(cands []ReconciliationCandidate) CASResult {
	for _, c := range cands {
		switch c.Status {
		case "integrated":
			return CASResult{Outcome: "promoted", CandidateSHA: c.CandidateSHA}
		case "failed":
			return CASResult{Outcome: "failed", FailureReason: c.FailureReason, CandidateSHA: c.CandidateSHA}
		}
	}
	return CASResult{Outcome: "not_attempted"}
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanFeature(s rowScanner) (Feature, error) {
	var (
		f                    Feature
		createdAt, updatedAt string
	)
	if err := s.Scan(&f.ID, &f.ParentRef, &f.BaseSHA, &f.ExpectedParentSHA, &f.Status, &createdAt, &updatedAt); err != nil {
		return Feature{}, err
	}
	var err error
	if f.CreatedAt, err = parseTime(createdAt); err != nil {
		return Feature{}, fmt.Errorf("serve: parse feature created_at %q: %w", createdAt, err)
	}
	if f.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return Feature{}, fmt.Errorf("serve: parse feature updated_at %q: %w", updatedAt, err)
	}
	return f, nil
}

func scanAttempt(s rowScanner) (Attempt, error) {
	var (
		a                    Attempt
		createdAt, updatedAt string
	)
	if err := s.Scan(&a.ID, &a.FeatureID, &a.IdempotencyKey, &a.Status, &a.Owner, &a.Fence, &a.CandidateSHA, &a.FailureReason, &createdAt, &updatedAt); err != nil {
		return Attempt{}, err
	}
	var err error
	if a.CreatedAt, err = parseTime(createdAt); err != nil {
		return Attempt{}, fmt.Errorf("serve: parse attempt created_at %q: %w", createdAt, err)
	}
	if a.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return Attempt{}, fmt.Errorf("serve: parse attempt updated_at %q: %w", updatedAt, err)
	}
	return a, nil
}

func scanLease(s rowScanner) (Lease, error) {
	var (
		lease                            Lease
		expiresAt, acquiredAt, updatedAt string
	)
	if err := s.Scan(&lease.FeatureID, &lease.Owner, &lease.Fence, &expiresAt, &acquiredAt, &updatedAt); err != nil {
		return Lease{}, err
	}
	var err error
	if lease.ExpiresAt, err = parseTime(expiresAt); err != nil {
		return Lease{}, fmt.Errorf("serve: parse lease expires_at %q: %w", expiresAt, err)
	}
	if lease.AcquiredAt, err = parseTime(acquiredAt); err != nil {
		return Lease{}, fmt.Errorf("serve: parse lease acquired_at %q: %w", acquiredAt, err)
	}
	if lease.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return Lease{}, fmt.Errorf("serve: parse lease updated_at %q: %w", updatedAt, err)
	}
	return lease, nil
}

func scanOverlap(s rowScanner) (OverlapEvidence, error) {
	var (
		ov        OverlapEvidence
		createdAt string
	)
	if err := s.Scan(&ov.ID, &ov.FeatureID, &ov.Version, &ov.EvidenceHash, &ov.EvidenceClass, &ov.EvidenceJSON, &createdAt); err != nil {
		return OverlapEvidence{}, err
	}
	t, err := parseTime(createdAt)
	if err != nil {
		return OverlapEvidence{}, fmt.Errorf("serve: parse overlap created_at %q: %w", createdAt, err)
	}
	ov.CreatedAt = t
	return ov, nil
}

func overlapFromRow(row ledger.OverlapEvidenceRow) OverlapEvidence {
	return OverlapEvidence{
		ID:            row.ID,
		FeatureID:     row.FeatureID,
		Version:       row.Version,
		EvidenceHash:  row.EvidenceHash,
		EvidenceClass: row.EvidenceClass,
		EvidenceJSON:  row.EvidenceJSON,
		CreatedAt:     row.CreatedAt,
	}
}

func candidateFromRow(row ledger.ReconciliationCandidateRow) ReconciliationCandidate {
	return ReconciliationCandidate{
		ID:            row.ID,
		RequestID:     row.RequestID,
		Status:        row.Status,
		AllowedPaths:  splitCSV(row.AllowedPaths),
		Model:         row.Model,
		Config:        row.Config,
		Output:        row.Output,
		Checks:        row.Checks,
		CandidateSHA:  row.CandidateSHA,
		FailureReason: row.FailureReason,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}
