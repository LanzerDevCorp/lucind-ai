package serve

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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

// NewHandler creates a new HTTP handler that serves the approvals UI and API.
func NewHandler(l *ledger.Ledger, defaultApprover string, opencodeCmd string) http.Handler {
	mux := http.NewServeMux()

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
			handleDefect(w, r, l, runID, laneID)
			return
		}

		if len(parts) > 2 {
			http.Error(w, "invalid approval path", http.StatusBadRequest)
			return
		}

		handleDecide(w, r, l, defaultApprover, runID, laneID)
	})

	return mux
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
