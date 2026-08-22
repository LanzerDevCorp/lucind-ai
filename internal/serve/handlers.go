package serve

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
)

// ServerState represents the JSON payload returned to the UI or API clients.
type ServerState struct {
	Approver        string            `json:"approver"`
	ApproverRate    float64           `json:"approver_rate"`
	OpencodeCommand string            `json:"opencode_command"`
	Approvals       []ledger.Approval `json:"approvals"`
}

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
func NewHandler(l *ledger.Ledger, defaultApprover string, opencodeCmd string) http.Handler {
	return NewHandlerWithConfig(l, defaultApprover, opencodeCmd, HandlerConfig{})
}

// NewHandlerWithConfig creates a handler with optional SSE and dispatch
// control. Dispatch remains disabled unless explicitly enabled with a token.
func NewHandlerWithConfig(l *ledger.Ledger, defaultApprover string, opencodeCmd string, config HandlerConfig) http.Handler {
	mux := http.NewServeMux()
	model := NewModel(l)

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
			serveStateJSON(w, r, l, defaultApprover, opencodeCmd)
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
		serveStateJSON(w, r, l, defaultApprover, opencodeCmd)
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

func serveStateJSON(w http.ResponseWriter, r *http.Request, l *ledger.Ledger, defaultApprover string, opencodeCmd string) {
	approvals, err := l.PendingApprovals(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to query approvals: %v", err), http.StatusInternalServerError)
		return
	}
	if approvals == nil {
		approvals = []ledger.Approval{}
	}

	rate, err := l.ApproverRate(r.Context(), defaultApprover)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to query approver rate: %v", err), http.StatusInternalServerError)
		return
	}

	state := ServerState{
		Approver:        defaultApprover,
		ApproverRate:    rate,
		OpencodeCommand: opencodeCmd,
		Approvals:       approvals,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(state)
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
