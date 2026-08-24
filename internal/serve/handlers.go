package serve

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
)

// ServerState represents the JSON payload returned to the UI or API clients.
//
// static/app.js takes all of its rendered data from this single object (see
// refreshState/renderState in app.js); SSE only tells the client when to
// re-fetch it. Every field below corresponds to a top-level key app.js reads
// directly or through its asArray(state, name, altName, ...) fallback helper.
// Where app.js accepts more than one name for the same concept, exactly one
// canonical name is emitted here -- never both -- since asArray already
// falls back through the alternatives and emitting duplicates would double
// the very payload these bounds exist to keep small.
type ServerState struct {
	Approver        string            `json:"approver"`
	ApproverRate    float64           `json:"approver_rate"`
	OpencodeCommand string            `json:"opencode_command"`
	Approvals       []ledger.Approval `json:"approvals"`
	// ServerTime is this server's clock at the moment the payload was built.
	// The console's lease countdowns are differences against expires_at, and
	// a viewer whose machine clock is off would otherwise read a live lease as
	// expired (or the reverse) with nothing on screen to explain it. The client
	// measures its offset from this field and anchors every countdown to it.
	ServerTime time.Time `json:"server_time"`

	// Runs is the newest-first window of run summaries (see stateMaxRuns).
	Runs []RunSummary `json:"runs"`
	// Lanes is every lane belonging to a run inside the Runs window (see
	// stateMaxLanes). app.js correlates each lane back to Runs by run_id.
	Lanes []Lane `json:"lanes"`
	// Events is the newest-first window of lifecycle events (the same
	// `events` table the SSE hub tails) drawn only from runs inside the Runs
	// window (see stateMaxEvents). Canonical name for app.js's
	// events/run_events alias pair.
	Events []ledger.Event `json:"events"`
	// LaneProgress is the newest-first window of lane progress rows for the
	// handful of most-recently-started runs (see stateMaxProgressRuns and
	// stateMaxLaneProgress). Canonical name for app.js's progress/lane_progress
	// alias pair.
	LaneProgress []LaneProgress `json:"lane_progress"`
	// IntegrationEvents is the newest-first window of feature audit events
	// drawn from the Features window (see stateMaxIntegrationEvents).
	// Canonical name for app.js's integration_events/audit_events alias pair.
	IntegrationEvents []AuditEvent `json:"integration_events"`
	// IntegrationAttempts is the newest-first window of integration attempts
	// drawn from the Features window (see stateMaxIntegrationAttempts).
	// Canonical name for app.js's attempts/integration_attempts alias pair.
	IntegrationAttempts []Attempt `json:"integration_attempts"`
	// Features is the newest-first window of feature rows (see
	// stateMaxFeatures). app.js merges this with its feature_swimlanes alias
	// to build the feature swimlane view; since both sources are unioned
	// (not a strict first-non-empty fallback), emitting Features alone
	// already supplies that view in full.
	Features []Feature `json:"features"`
	// FeatureLeases is every feature lease, most-recently-updated first (see
	// stateMaxFeatureLeases). Canonical name for app.js's
	// leases/feature_leases alias pair.
	FeatureLeases []Lease `json:"feature_leases"`
	// OverlapEvidence is the newest-first window of overlap evidence drawn
	// from the Features window (see stateMaxOverlapEvidence). Canonical name
	// for app.js's overlap_evidence/overlaps alias pair.
	OverlapEvidence []OverlapEvidence `json:"overlap_evidence"`
	// ReconciliationRequests is the newest-first window of reconciliation
	// requests (each already carrying its own candidates and audit trail)
	// drawn from the Features window (see stateMaxReconciliationRequests).
	// Canonical name for app.js's reconciliations/reconciliation_requests
	// alias pair.
	ReconciliationRequests []ReconciliationRequest `json:"reconciliation_requests"`
	// SDDFlows is the run-recency-ordered window of derived SDD flow rollups
	// (see stateMaxSDDFlows).
	SDDFlows []SDDFlow `json:"sdd_flows"`
}

// Bounds applied when assembling /api/state. static/app.js polls this
// endpoint every 2 seconds while SSE is down, and re-fetches it on every
// streamed record while SSE is healthy (see connectSSE in app.js), so its
// cost must stay flat as a ledger accumulates history -- not grow with total
// lane/event/progress/feature counts. Every collection below applies a
// "most recent N, newest first" window instead of serializing full history.
const (
	// stateMaxRuns bounds how many of the newest runs (Model.ListRuns is
	// already newest-first) are surfaced. Fifty is generous headroom over a
	// single active dispatch plus its recent predecessors for context, while
	// keeping the run-scoped query fan-out below fixed regardless of how many
	// runs a long-lived project has accumulated.
	stateMaxRuns = 50

	// stateMaxLanes caps the lanes drawn from the run window above. Lane
	// count is naturally bounded by the fanout of those runs, but this
	// ceiling protects against one unusually large fanout dominating the
	// payload; lanes from the newest runs are kept.
	stateMaxLanes = 500

	// stateMaxEvents bounds the lifecycle events drawn from the run window
	// above. events.id is a global autoincrement (see hub.go), so sorting by
	// id descending is a cheap, deterministic "newest first" with no
	// timestamp parsing.
	stateMaxEvents = 200

	// stateMaxProgressRuns further restricts which runs' lanes get a lane
	// progress lookup. GetLaneProgress has no bulk cross-lane query, so
	// fetching it is one query per lane; running that fan-out over the full
	// stateMaxLanes window would mean hundreds of queries every poll for a
	// project with heavy fanout. Progress is only useful for the currently
	// active work anyway, so it is restricted to the handful of
	// most-recently-started runs.
	stateMaxProgressRuns = 5

	// stateMaxLaneProgress bounds the merged progress rows collected from
	// the stateMaxProgressRuns window, newest first by their "at" timestamp.
	stateMaxLaneProgress = 200

	// stateMaxFeatures bounds the features list itself. Features are created
	// far less often than runs (one per integration feature branch, not per
	// dispatch), so this is deliberately generous relative to today's usage;
	// older features age out of every feature-scoped collection below with
	// them, keeping the story consistent.
	stateMaxFeatures = 50

	// stateMaxIntegrationEvents, stateMaxIntegrationAttempts,
	// stateMaxOverlapEvidence, and stateMaxReconciliationRequests bound the
	// feature-scoped collections, each computed only over the stateMaxFeatures
	// window above.
	stateMaxIntegrationEvents      = 200
	stateMaxIntegrationAttempts    = 200
	stateMaxOverlapEvidence        = 200
	stateMaxReconciliationRequests = 200

	// stateMaxFeatureLeases bounds the leases list. A lease is keyed one per
	// feature today, so this is a defensive ceiling rather than an expected
	// truncation point.
	stateMaxFeatureLeases = 200

	// stateMaxSDDFlows bounds the derived SDD flow rollup returned to the
	// client. Unlike Model.ListSDDFlows (which rediscovers its own run list
	// from the runs table internally, and so is exactly as blind to a
	// runs-table-less ledger as everything else here was), this rollup is
	// computed inline from the same resolveRunWindow lanes already fetched
	// above, so it is naturally scoped to that same window before this
	// ceiling is even applied.
	stateMaxSDDFlows = 200
)

type decideRequest struct {
	Decision  string `json:"decision"`
	Approver  string `json:"approver"`
	Approvals any    `json:"approvals,omitempty"`
	Decisions any    `json:"decisions,omitempty"`
	Lanes     any    `json:"lanes,omitempty"`
}

type defectRequest struct {
	Defect bool `json:"defect"`
}

// HandlerConfig adds optional live telemetry and dispatch control without
// changing the read-only behavior of existing NewHandler callers.
type HandlerConfig struct {
	Hub            *Hub
	EnableDispatch bool
	DispatchToken  string
}

// NewHandler creates a new HTTP handler that serves the approvals UI and API.
func NewHandler(m *Model, defaultApprover string, opencodeCmd string) http.Handler {
	return NewHandlerWithConfig(m, defaultApprover, opencodeCmd, HandlerConfig{})
}

// NewHandlerWithConfig creates a handler with optional SSE and dispatch
// control. Dispatch remains disabled unless explicitly enabled with a token.
func NewHandlerWithConfig(m *Model, defaultApprover string, opencodeCmd string, config HandlerConfig) http.Handler {
	mux := http.NewServeMux()
	model := m
	l := m.Ledger()

	mux.HandleFunc("/api/features", func(w http.ResponseWriter, r *http.Request) {
		if !requireGET(w, r) {
			return
		}
		value, err := model.ListFeatures(r.Context())
		writeModelResponse(w, value, err)
	})
	mux.HandleFunc("/api/features/", func(w http.ResponseWriter, r *http.Request) {
		handleFeatureRead(w, r, model)
	})
	mux.HandleFunc("/api/attempts/", func(w http.ResponseWriter, r *http.Request) {
		if !requireGET(w, r) {
			return
		}
		id, ok := singlePathID(r.URL.Path, "/api/attempts/")
		if !ok {
			writeJSONError(w, http.StatusNotFound, "not found")
			return
		}
		value, err := model.GetAttempt(r.Context(), id)
		writeModelResponse(w, value, err)
	})
	mux.HandleFunc("/api/leases", func(w http.ResponseWriter, r *http.Request) {
		if !requireGET(w, r) {
			return
		}
		value, err := model.ListLeases(r.Context())
		writeModelResponse(w, value, err)
	})
	mux.HandleFunc("/api/reconciliations/", func(w http.ResponseWriter, r *http.Request) {
		handleReconciliationRead(w, r, model)
	})
	mux.HandleFunc("/api/candidates/", func(w http.ResponseWriter, r *http.Request) {
		if !requireGET(w, r) {
			return
		}
		id, ok := singlePathID(r.URL.Path, "/api/candidates/")
		if !ok {
			writeJSONError(w, http.StatusNotFound, "not found")
			return
		}
		value, err := model.GetReconciliationCandidate(r.Context(), id)
		writeModelResponse(w, value, err)
	})
	mux.HandleFunc("/api/approvals", func(w http.ResponseWriter, r *http.Request) {
		if !requireGET(w, r) {
			return
		}
		approvals, err := l.PendingApprovals(r.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "query failed")
			return
		}
		if approvals == nil {
			approvals = []ledger.Approval{}
		}
		writeJSON(w, http.StatusOK, approvals)
	})
	mux.HandleFunc("/api/batch/", func(w http.ResponseWriter, r *http.Request) {
		handleBatchRead(w, r, model)
	})
	mux.HandleFunc("/api/stream", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if config.Hub == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "telemetry stream unavailable")
			return
		}
		config.Hub.ServeHTTP(w, r)
	})
	mux.HandleFunc("/api/packets/", func(w http.ResponseWriter, r *http.Request) {
		handlePacketBody(w, r, l)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			// Try serving static asset
			filePath := strings.TrimPrefix(r.URL.Path, "/")
			if data, err := staticFS.ReadFile("static/" + filePath); err == nil {
				if strings.HasSuffix(filePath, ".js") {
					w.Header().Set("Content-Type", "application/javascript")
				} else if strings.HasSuffix(filePath, ".css") {
					w.Header().Set("Content-Type", "text/css")
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(data)
				return
			}
			http.NotFound(w, r)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// If client requests JSON
		if strings.Contains(r.Header.Get("Accept"), "application/json") || r.URL.Query().Get("format") == "json" {
			serveStateJSON(w, r, model, defaultApprover, opencodeCmd)
			return
		}

		// Serve index.html
		data, err := staticFS.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})

	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		serveStateJSON(w, r, model, defaultApprover, opencodeCmd)
	})

	mux.HandleFunc("/approvals/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/approvals/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 2 {
			http.Error(w, "invalid approval path", http.StatusBadRequest)
			return
		}

		runID := parts[0]
		laneID := parts[1]

		// Check for /approvals/{runID}/{laneID}/defect
		if len(parts) == 3 && parts[2] == "defect" {
			if !authorizeDispatch(w, r, config) {
				return
			}
			handleDefect(w, r, l, runID, laneID)
			return
		}

		if len(parts) > 2 {
			http.Error(w, "invalid approval path", http.StatusBadRequest)
			return
		}

		if !authorizeDispatch(w, r, config) {
			return
		}
		handleDecide(w, r, l, defaultApprover, runID, laneID)
	})

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeJSONError(w, http.StatusNotFound, "not found")
	})

	return mux
}

func handlePacketBody(w http.ResponseWriter, r *http.Request, l *ledger.Ledger) {
	if !requireGET(w, r) {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/packets/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		writeJSONError(w, http.StatusNotFound, "not found")
		return
	}
	runID, laneID := parts[0], parts[1]
	metadata, err := l.GetLaneMetadata(r.Context(), runID, laneID)
	if err != nil {
		if errors.Is(err, ledger.ErrLaneUnknown) {
			writeJSONError(w, http.StatusNotFound, "not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if metadata.PacketPath == "" {
		writeJSONError(w, http.StatusNotFound, "not found")
		return
	}
	data, err := os.ReadFile(metadata.PacketPath)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "not found")
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func handleFeatureRead(w http.ResponseWriter, r *http.Request, model *Model) {
	if !requireGET(w, r) {
		return
	}
	parts := pathParts(strings.TrimPrefix(r.URL.Path, "/api/features/"))
	if len(parts) == 0 {
		writeJSONError(w, http.StatusNotFound, "not found")
		return
	}

	var value any
	var err error
	switch {
	case len(parts) == 1:
		value, err = model.GetFeature(r.Context(), parts[0])
	case len(parts) == 2 && parts[1] == "attempts":
		value, err = model.ListAttempts(r.Context(), parts[0])
	case len(parts) == 2 && parts[1] == "lease":
		value, err = model.GetLease(r.Context(), parts[0])
	case len(parts) == 2 && parts[1] == "overlap":
		value, err = model.ListOverlapEvidence(r.Context(), parts[0])
	case len(parts) == 3 && parts[1] == "overlap":
		value, err = model.GetOverlapEvidence(r.Context(), parts[0], parts[2])
	case len(parts) == 2 && parts[1] == "reconciliations":
		value, err = model.ListReconciliationRequests(r.Context(), parts[0])
	case len(parts) == 2 && parts[1] == "events":
		value, err = model.ListAuditEvents(r.Context(), parts[0])
	default:
		writeJSONError(w, http.StatusNotFound, "not found")
		return
	}
	writeModelResponse(w, value, err)
}

func handleReconciliationRead(w http.ResponseWriter, r *http.Request, model *Model) {
	if !requireGET(w, r) {
		return
	}
	parts := pathParts(strings.TrimPrefix(r.URL.Path, "/api/reconciliations/"))
	var value any
	var err error
	switch {
	case len(parts) == 1:
		value, err = model.GetReconciliationRequest(r.Context(), parts[0])
	case len(parts) == 2 && parts[1] == "candidates":
		value, err = model.ListReconciliationCandidates(r.Context(), parts[0])
	default:
		writeJSONError(w, http.StatusNotFound, "not found")
		return
	}
	writeModelResponse(w, value, err)
}

func handleBatchRead(w http.ResponseWriter, r *http.Request, model *Model) {
	if !requireGET(w, r) {
		return
	}
	parts := pathParts(strings.TrimPrefix(r.URL.Path, "/api/batch/"))
	if len(parts) != 2 || parts[1] != "lanes" || parts[0] == "" {
		writeJSONError(w, http.StatusNotFound, "not found")
		return
	}
	if _, err := model.GetRun(r.Context(), parts[0]); err != nil {
		if isModelNotFound(err) {
			writeJSONError(w, http.StatusNotFound, "not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "query failed")
		return
	}
	value, err := model.ListBatchLanes(r.Context(), parts[0])
	if err == nil && value == nil {
		value = []BatchLane{}
	}
	writeModelResponse(w, value, err)
}

func requireGET(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	return false
}

func singlePathID(path, prefix string) (string, bool) {
	parts := pathParts(strings.TrimPrefix(path, prefix))
	if len(parts) != 1 || parts[0] == "" {
		return "", false
	}
	return parts[0], true
}

func pathParts(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func writeModelResponse(w http.ResponseWriter, value any, err error) {
	if err != nil {
		if isModelNotFound(err) {
			writeJSONError(w, http.StatusNotFound, "not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "query failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func isModelNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows) ||
		errors.Is(err, ledger.ErrRunUnknown) ||
		errors.Is(err, ledger.ErrLaneUnknown) ||
		errors.Is(err, ledger.ErrOverlapEvidenceNotFound) ||
		errors.Is(err, ledger.ErrReconciliationRequestNotFound) ||
		errors.Is(err, ledger.ErrReconciliationCandidateNotFound)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func authorizeDispatch(w http.ResponseWriter, r *http.Request, config HandlerConfig) bool {
	if !config.EnableDispatch || config.DispatchToken == "" {
		writeJSONError(w, http.StatusForbidden, "dispatch control disabled")
		return false
	}
	if !requestIsSameOrigin(r) {
		writeJSONError(w, http.StatusForbidden, "same-origin request required")
		return false
	}
	if !validBearerToken(r.Header.Get("Authorization"), config.DispatchToken) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="lucind-ai-control"`)
		writeJSONError(w, http.StatusUnauthorized, "invalid control token")
		return false
	}
	return true
}

func requestIsSameOrigin(r *http.Request) bool {
	origin, err := url.Parse(r.Header.Get("Origin"))
	if err != nil || origin.Scheme == "" || origin.Host == "" || origin.User != nil ||
		(origin.Path != "" && origin.Path != "/") || origin.RawQuery != "" || origin.Fragment != "" {
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return strings.EqualFold(origin.Scheme, scheme) && strings.EqualFold(origin.Host, r.Host)
}

func validBearerToken(header, expected string) bool {
	scheme, token, ok := strings.Cut(strings.TrimSpace(header), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return false
	}
	providedHash := sha256.Sum256([]byte(token))
	expectedHash := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(providedHash[:], expectedHash[:]) == 1
}

func serveStateJSON(w http.ResponseWriter, r *http.Request, model *Model, defaultApprover string, opencodeCmd string) {
	state, err := buildServerState(r.Context(), model, defaultApprover, opencodeCmd)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to build state: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(state)
}

// buildServerState assembles the full /api/state payload from model, bounded
// per the stateMax* constants documented above ServerState. See those
// constants for why each collection is windowed the way it is.
func buildServerState(ctx context.Context, model *Model, defaultApprover, opencodeCmd string) (ServerState, error) {
	l := model.Ledger()

	approvals, err := l.PendingApprovals(ctx)
	if err != nil {
		return ServerState{}, fmt.Errorf("query approvals: %w", err)
	}
	if approvals == nil {
		approvals = []ledger.Approval{}
	}

	rate, err := l.ApproverRate(ctx, defaultApprover)
	if err != nil {
		return ServerState{}, fmt.Errorf("query approver rate: %w", err)
	}

	runIDs, realRuns, err := resolveRunWindow(ctx, model)
	if err != nil {
		return ServerState{}, err
	}
	if len(runIDs) > stateMaxRuns {
		runIDs = runIDs[:stateMaxRuns]
	}
	progressRunIDs := make(map[string]bool, stateMaxProgressRuns)
	for i, runID := range runIDs {
		if i >= stateMaxProgressRuns {
			break
		}
		progressRunIDs[runID] = true
	}

	runs := make([]RunSummary, 0, len(runIDs))
	lanes := []Lane{}
	events := []ledger.Event{}
	sddFlows := []SDDFlow{}
	type flowKey struct{ runID, change, phase, fanout string }
	flowIndex := make(map[flowKey]int)
	flowStatuses := make(map[flowKey][]lane.Status)
	for _, runID := range runIDs {
		runLanes, err := model.ListLanes(ctx, runID)
		if err != nil {
			return ServerState{}, fmt.Errorf("list lanes for run %q: %w", runID, err)
		}
		lanes = append(lanes, runLanes...)

		runEvents, err := l.Events(ctx, runID)
		if err != nil {
			return ServerState{}, fmt.Errorf("list events for run %q: %w", runID, err)
		}
		events = append(events, runEvents...)

		if real, ok := realRuns[runID]; ok {
			runs = append(runs, real)
		} else {
			synthetic, err := synthesizeRunSummary(ctx, l, runID, runLanes, runEvents)
			if err != nil {
				return ServerState{}, fmt.Errorf("synthesize run summary for %q: %w", runID, err)
			}
			runs = append(runs, synthetic)
		}

		// Derived inline from the same lanes fetched above, rather than via
		// Model.ListSDDFlows: that method rediscovers its own run list from
		// the runs table internally (see its doc comment), so it is exactly
		// as blind to a runs-table-less ledger as everything else here was.
		for _, ln := range runLanes {
			key := flowKey{runID, ln.Change, ln.SDDPhase, ln.FanoutGroup}
			idx, ok := flowIndex[key]
			if !ok {
				idx = len(sddFlows)
				flowIndex[key] = idx
				sddFlows = append(sddFlows, SDDFlow{RunID: runID, Change: ln.Change, SDDPhase: ln.SDDPhase, FanoutGroup: ln.FanoutGroup, LaneIDs: []string{}})
			}
			sddFlows[idx].LaneCount++
			sddFlows[idx].LaneIDs = append(sddFlows[idx].LaneIDs, ln.LaneID)
			flowStatuses[key] = append(flowStatuses[key], lane.Status(ln.Status))
		}
	}
	for key, idx := range flowIndex {
		sddFlows[idx].Status = rollupStatus(flowStatuses[key])
	}
	if len(lanes) > stateMaxLanes {
		lanes = lanes[:stateMaxLanes]
	}
	sort.Slice(events, func(i, j int) bool { return events[i].ID > events[j].ID })
	if len(events) > stateMaxEvents {
		events = events[:stateMaxEvents]
	}
	if len(sddFlows) > stateMaxSDDFlows {
		sddFlows = sddFlows[:stateMaxSDDFlows]
	}

	laneProgress := []LaneProgress{}
	for _, ln := range lanes {
		if !progressRunIDs[ln.RunID] {
			continue
		}
		progress, err := model.GetLaneProgress(ctx, ln.RunID, ln.LaneID, 0)
		if err != nil {
			return ServerState{}, fmt.Errorf("list lane progress for %s/%s: %w", ln.RunID, ln.LaneID, err)
		}
		laneProgress = append(laneProgress, progress...)
	}
	sort.Slice(laneProgress, func(i, j int) bool { return laneProgress[i].At.After(laneProgress[j].At) })
	if len(laneProgress) > stateMaxLaneProgress {
		laneProgress = laneProgress[:stateMaxLaneProgress]
	}

	allFeatures, err := model.ListFeatures(ctx)
	if err != nil {
		return ServerState{}, fmt.Errorf("list features: %w", err)
	}
	sort.Slice(allFeatures, func(i, j int) bool { return allFeatures[i].CreatedAt.After(allFeatures[j].CreatedAt) })
	features := allFeatures
	if len(features) > stateMaxFeatures {
		features = features[:stateMaxFeatures]
	}

	attempts := []Attempt{}
	overlaps := []OverlapEvidence{}
	reconciliations := []ReconciliationRequest{}
	integrationEvents := []AuditEvent{}
	for _, feature := range features {
		featureAttempts, err := model.ListAttempts(ctx, feature.ID)
		if err != nil {
			return ServerState{}, fmt.Errorf("list attempts for feature %q: %w", feature.ID, err)
		}
		attempts = append(attempts, featureAttempts...)

		featureOverlaps, err := model.ListOverlapEvidence(ctx, feature.ID)
		if err != nil {
			return ServerState{}, fmt.Errorf("list overlap evidence for feature %q: %w", feature.ID, err)
		}
		overlaps = append(overlaps, featureOverlaps...)

		featureReconciliations, err := model.ListReconciliationRequests(ctx, feature.ID)
		if err != nil {
			return ServerState{}, fmt.Errorf("list reconciliation requests for feature %q: %w", feature.ID, err)
		}
		reconciliations = append(reconciliations, featureReconciliations...)

		featureEvents, err := model.ListAuditEvents(ctx, feature.ID)
		if err != nil {
			return ServerState{}, fmt.Errorf("list audit events for feature %q: %w", feature.ID, err)
		}
		integrationEvents = append(integrationEvents, featureEvents...)
	}
	sort.Slice(attempts, func(i, j int) bool { return attempts[i].CreatedAt.After(attempts[j].CreatedAt) })
	if len(attempts) > stateMaxIntegrationAttempts {
		attempts = attempts[:stateMaxIntegrationAttempts]
	}
	sort.Slice(overlaps, func(i, j int) bool { return overlaps[i].ID > overlaps[j].ID })
	if len(overlaps) > stateMaxOverlapEvidence {
		overlaps = overlaps[:stateMaxOverlapEvidence]
	}
	sort.Slice(reconciliations, func(i, j int) bool { return reconciliations[i].UpdatedAt.After(reconciliations[j].UpdatedAt) })
	if len(reconciliations) > stateMaxReconciliationRequests {
		reconciliations = reconciliations[:stateMaxReconciliationRequests]
	}
	sort.Slice(integrationEvents, func(i, j int) bool { return integrationEvents[i].ID > integrationEvents[j].ID })
	if len(integrationEvents) > stateMaxIntegrationEvents {
		integrationEvents = integrationEvents[:stateMaxIntegrationEvents]
	}

	leases, err := model.ListLeases(ctx)
	if err != nil {
		return ServerState{}, fmt.Errorf("list leases: %w", err)
	}
	sort.Slice(leases, func(i, j int) bool { return leases[i].UpdatedAt.After(leases[j].UpdatedAt) })
	if len(leases) > stateMaxFeatureLeases {
		leases = leases[:stateMaxFeatureLeases]
	}

	return ServerState{
		Approver:               defaultApprover,
		ApproverRate:           rate,
		OpencodeCommand:        opencodeCmd,
		Approvals:              approvals,
		ServerTime:             time.Now().UTC(),
		Runs:                   runs,
		Lanes:                  lanes,
		Events:                 events,
		LaneProgress:           laneProgress,
		IntegrationEvents:      integrationEvents,
		IntegrationAttempts:    attempts,
		Features:               features,
		FeatureLeases:          leases,
		OverlapEvidence:        overlaps,
		ReconciliationRequests: reconciliations,
		SDDFlows:               sddFlows,
	}, nil
}

// resolveRunWindow returns the newest-first run ids to surface in
// /api/state, and the RunSummary for each one that has an actual runs-table
// row. It does not trust the runs table alone: lucind-ai run only recently
// started calling ledger.RegisterRun, and nothing backfills the runs table
// for a ledger dispatched before that, so a run id can be real (its lanes
// and events exist and carry it) without ever appearing in Model.ListRuns.
// Excluding those run ids would leave every lane, event, and lane_progress
// row they own permanently invisible to the console -- exactly the reported
// symptom on a live, historical ledger.
//
// The ordering favors runs.RunIDsByRecentEvent (events.id is a global
// autoincrement, so grouping by run_id and taking each run's own max id is
// a correct, cheap recency signal) over the runs table's own StartedAt
// ordering, then falls through to ledger.DistinctLaneRunIDs for the
// residual case of a run whose lanes exist but which has not (yet, or
// ever) produced a lifecycle event.
func resolveRunWindow(ctx context.Context, model *Model) (runIDs []string, realRuns map[string]RunSummary, err error) {
	l := model.Ledger()

	allRuns, err := model.ListRuns(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list runs: %w", err)
	}
	realRuns = make(map[string]RunSummary, len(allRuns))
	for _, run := range allRuns {
		realRuns[run.RunID] = run
	}

	byRecentEvent, err := l.RunIDsByRecentEvent(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list run ids by recent event: %w", err)
	}
	laneRunIDs, err := l.DistinctLaneRunIDs(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list distinct lane run ids: %w", err)
	}

	seen := make(map[string]bool, len(allRuns)+len(byRecentEvent)+len(laneRunIDs))
	ordered := make([]string, 0, len(allRuns)+len(byRecentEvent)+len(laneRunIDs))
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		ordered = append(ordered, id)
	}
	for _, id := range byRecentEvent {
		add(id)
	}
	for _, run := range allRuns {
		add(run.RunID)
	}
	for _, id := range laneRunIDs {
		add(id)
	}
	return ordered, realRuns, nil
}

// synthesizeRunSummary builds a best-effort RunSummary for a run id with no
// runs-table row, using only lanes, events, and approvals that are already
// known to carry it. Status is derived from the lanes' own statuses (the
// same rollup Model.ListSDDFlows/summarizeRun use elsewhere) since there is
// no persisted run-level status to report; FeatureID and TargetRef are
// taken from the first lane that carries them (metadata is optional per
// lane, so this is a best effort, not a guarantee); EndedAt is left nil
// since nothing records when an unregistered run actually finished.
func synthesizeRunSummary(ctx context.Context, l *ledger.Ledger, runID string, lanes []Lane, events []ledger.Event) (RunSummary, error) {
	summary := RunSummary{RunID: runID, LaneCount: len(lanes)}
	var counts LaneStatusCounts
	for _, ln := range lanes {
		addLaneStatus(&counts, lane.Status(ln.Status))
		if summary.FeatureID == "" && ln.Feature != "" {
			summary.FeatureID = ln.Feature
		}
		if ln.StartedAt != nil && (summary.StartedAt.IsZero() || ln.StartedAt.Before(summary.StartedAt)) {
			summary.StartedAt = *ln.StartedAt
		}
	}
	summary.LaneStatusCounts = counts
	summary.Status = statusFromCounts(counts)
	if summary.StartedAt.IsZero() {
		for _, e := range events {
			if summary.StartedAt.IsZero() || e.At.Before(summary.StartedAt) {
				summary.StartedAt = e.At
			}
		}
	}

	approvals, err := l.Approvals(ctx, runID)
	if err != nil {
		return RunSummary{}, fmt.Errorf("list run approvals: %w", err)
	}
	for _, approval := range approvals {
		if approval.Decision == ledger.DecisionPending {
			summary.PendingApprovals++
		}
	}
	return summary, nil
}

func handleDecide(w http.ResponseWriter, r *http.Request, l *ledger.Ledger, defaultApprover string, runID, laneID string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		http.Error(w, "empty request body", http.StatusBadRequest)
		return
	}

	// Threat matrix / spec: bulk approval request MUST be rejected with 400
	if trimmed[0] == '[' {
		http.Error(w, "bulk approval rejected; decisions must be made individually", http.StatusBadRequest)
		return
	}

	var req decideRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, fmt.Sprintf("invalid json: %v", err), http.StatusBadRequest)
		return
	}

	if req.Approvals != nil || req.Decisions != nil || req.Lanes != nil {
		http.Error(w, "bulk approval rejected; decisions must be made individually", http.StatusBadRequest)
		return
	}

	// Unselected or empty decision is rejected
	decisionStr := strings.TrimSpace(req.Decision)
	if decisionStr == "" {
		http.Error(w, "unselected decision: decision must be 'approved' or 'rejected'", http.StatusBadRequest)
		return
	}

	if decisionStr != string(ledger.DecisionApproved) && decisionStr != string(ledger.DecisionRejected) {
		http.Error(w, fmt.Sprintf("invalid decision %q: must be 'approved' or 'rejected'", decisionStr), http.StatusBadRequest)
		return
	}

	approver := strings.TrimSpace(req.Approver)
	if approver == "" {
		approver = defaultApprover
	}

	if err := l.Decide(r.Context(), runID, laneID, approver, ledger.Decision(decisionStr)); err != nil {
		if errors.Is(err, ledger.ErrAlreadyDecided) {
			http.Error(w, fmt.Sprintf("approval already decided: %v", err), http.StatusConflict)
			return
		}
		if errors.Is(err, ledger.ErrLaneUnknown) {
			http.Error(w, fmt.Sprintf("approval not found: %v", err), http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("failed to record decision: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func handleDefect(w http.ResponseWriter, r *http.Request, l *ledger.Ledger, runID, laneID string) {
	var req defectRequest
	req.Defect = true // default to true if not specified
	if r.Body != nil {
		body, _ := io.ReadAll(r.Body)
		if len(bytes.TrimSpace(body)) > 0 {
			_ = json.Unmarshal(body, &req)
		}
	}

	if err := l.MarkDefectSurfaced(r.Context(), runID, laneID, req.Defect); err != nil {
		http.Error(w, fmt.Sprintf("failed to mark defect: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}
