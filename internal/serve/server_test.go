package serve_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/serve"
)

func TestNonLoopbackListenFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	nonLoopbackAddrs := []string{
		"0.0.0.0:7433",
		"192.168.1.100:7433",
		"10.0.0.1:7433",
		"example.com:7433",
		":7433",
	}

	for _, addr := range nonLoopbackAddrs {
		err := serve.ListenAndServe(ctx, addr, dummyHandler)
		if err == nil {
			t.Errorf("ListenAndServe(%q) succeeded, want non-loopback error", addr)
		}
		if !errors.Is(err, serve.ErrNonLoopback) && !strings.Contains(strings.ToLower(err.Error()), "loopback") {
			t.Errorf("ListenAndServe(%q) error = %v, want ErrNonLoopback or error mentioning loopback", addr, err)
		}
	}
}

func TestBulkRequestBodyReturns400(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	l, err := ledger.Open(ctx, root)
	if err != nil {
		t.Fatalf("ledger.Open: %v", err)
	}
	defer l.Close()

	if err := l.RequestApproval(ctx, ledger.Approval{
		RunID:       "run-1",
		LaneID:      "lane-1",
		PacketID:    "pkt-1",
		Evidence:    "file.go:10",
		RequestedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("RequestApproval 1: %v", err)
	}
	if err := l.RequestApproval(ctx, ledger.Approval{
		RunID:       "run-1",
		LaneID:      "lane-2",
		PacketID:    "pkt-2",
		Evidence:    "file.go:20",
		RequestedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("RequestApproval 2: %v", err)
	}

	handler := newDispatchEnabledHandler(l)

	// Array body to single item route
	arrayBody := `[{"decision":"approved"}, {"decision":"approved"}]`
	req := httptest.NewRequest(http.MethodPost, "/approvals/run-1/lane-1", bytes.NewBufferString(arrayBody))
	req.Header.Set("Content-Type", "application/json")
	authorizeControlRequest(req)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("POST array body code = %d, want 400 Bad Request", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "bulk approval rejected; decisions must be made individually" {
		t.Errorf("POST array body = %q, want unchanged individual-only rejection", got)
	}

	// Bulk object body
	bulkBody := `{"approvals": [{"run_id":"run-1","lane_id":"lane-1","decision":"approved"},{"run_id":"run-1","lane_id":"lane-2","decision":"approved"}]}`
	req2 := httptest.NewRequest(http.MethodPost, "/approvals/run-1/lane-1", bytes.NewBufferString(bulkBody))
	req2.Header.Set("Content-Type", "application/json")
	authorizeControlRequest(req2)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusBadRequest {
		t.Errorf("POST bulk object body code = %d, want 400 Bad Request", rec2.Code)
	}
	if got := strings.TrimSpace(rec2.Body.String()); got != "bulk approval rejected; decisions must be made individually" {
		t.Errorf("POST bulk object body = %q, want unchanged individual-only rejection", got)
	}
}

func TestUnselectedDecisionReturns400(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	l, err := ledger.Open(ctx, root)
	if err != nil {
		t.Fatalf("ledger.Open: %v", err)
	}
	defer l.Close()

	if err := l.RequestApproval(ctx, ledger.Approval{
		RunID:       "run-1",
		LaneID:      "lane-1",
		PacketID:    "pkt-1",
		Evidence:    "file.go:10",
		RequestedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}

	handler := newDispatchEnabledHandler(l)

	emptyDecisionBody := `{"decision": ""}`
	req := httptest.NewRequest(http.MethodPost, "/approvals/run-1/lane-1", bytes.NewBufferString(emptyDecisionBody))
	req.Header.Set("Content-Type", "application/json")
	authorizeControlRequest(req)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("POST empty decision code = %d, want 400 Bad Request", rec.Code)
	}

	// Verify ledger row is still pending
	app, err := l.Approval(ctx, "run-1", "lane-1")
	if err != nil {
		t.Fatalf("Approval: %v", err)
	}
	if app.Decision != ledger.DecisionPending {
		t.Errorf("app.Decision = %v, want DecisionPending", app.Decision)
	}
}

func TestSingleApprovalAndDefectEndpoints(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	l, err := ledger.Open(ctx, root)
	if err != nil {
		t.Fatalf("ledger.Open: %v", err)
	}
	defer l.Close()

	if err := l.RequestApproval(ctx, ledger.Approval{
		RunID:       "run-1",
		LaneID:      "lane-1",
		PacketID:    "pkt-1",
		Evidence:    "file.go:10",
		RequestedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}

	handler := newDispatchEnabledHandler(l)

	// Approve lane-1
	approveBody := `{"decision": "approved"}`
	req := httptest.NewRequest(http.MethodPost, "/approvals/run-1/lane-1", bytes.NewBufferString(approveBody))
	req.Header.Set("Content-Type", "application/json")
	authorizeControlRequest(req)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST approve code = %d, want 200 OK (body: %s)", rec.Code, rec.Body.String())
	}

	app, err := l.Approval(ctx, "run-1", "lane-1")
	if err != nil {
		t.Fatalf("Approval: %v", err)
	}
	if app.Decision != ledger.DecisionApproved || app.Approver != "alice" {
		t.Errorf("app = %+v, want approved by alice", app)
	}

	// Mark defect
	defectBody := `{"defect": true}`
	req2 := httptest.NewRequest(http.MethodPost, "/approvals/run-1/lane-1/defect", bytes.NewBufferString(defectBody))
	req2.Header.Set("Content-Type", "application/json")
	authorizeControlRequest(req2)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("POST defect code = %d, want 200 OK (body: %s)", rec2.Code, rec2.Body.String())
	}

	appAfterDefect, err := l.Approval(ctx, "run-1", "lane-1")
	if err != nil {
		t.Fatalf("Approval after defect: %v", err)
	}
	if !appAfterDefect.DefectSurfacedLater {
		t.Errorf("appAfterDefect.DefectSurfacedLater = false, want true")
	}
}

func TestDecideAlreadyDecidedReturns409Conflict(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	l, err := ledger.Open(ctx, root)
	if err != nil {
		t.Fatalf("ledger.Open: %v", err)
	}
	defer l.Close()

	if err := l.RequestApproval(ctx, ledger.Approval{
		RunID:       "run-1",
		LaneID:      "lane-1",
		PacketID:    "pkt-1",
		Evidence:    "file.go:10",
		RequestedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}

	handler := newDispatchEnabledHandler(l)

	// First decision succeeds with 200 OK
	approveBody := `{"decision": "approved"}`
	req := httptest.NewRequest(http.MethodPost, "/approvals/run-1/lane-1", bytes.NewBufferString(approveBody))
	req.Header.Set("Content-Type", "application/json")
	authorizeControlRequest(req)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("First POST approve code = %d, want 200 OK (body: %s)", rec.Code, rec.Body.String())
	}

	// Second decision returns 409 Conflict
	rejectBody := `{"decision": "rejected"}`
	req2 := httptest.NewRequest(http.MethodPost, "/approvals/run-1/lane-1", bytes.NewBufferString(rejectBody))
	req2.Header.Set("Content-Type", "application/json")
	authorizeControlRequest(req2)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Errorf("Second POST approve code = %d, want 409 Conflict (body: %s)", rec2.Code, rec2.Body.String())
	}
}

func TestModelReadRoutes(t *testing.T) {
	l := openServeLedger(t)
	seedModelReadRows(t, l)
	handler := serve.NewHandler(serve.NewModel(l), "alice", "opencode run")

	tests := []struct {
		name string
		path string
		code int
	}{
		{name: "features", path: "/api/features", code: http.StatusOK},
		{name: "feature", path: "/api/features/feat-1", code: http.StatusOK},
		{name: "feature attempts", path: "/api/features/feat-1/attempts", code: http.StatusOK},
		{name: "attempt", path: "/api/attempts/attempt-1", code: http.StatusOK},
		{name: "leases", path: "/api/leases", code: http.StatusOK},
		{name: "feature lease", path: "/api/features/feat-1/lease", code: http.StatusOK},
		{name: "feature overlap", path: "/api/features/feat-1/overlap", code: http.StatusOK},
		{name: "overlap evidence", path: "/api/features/feat-1/overlap/overlap-1", code: http.StatusOK},
		{name: "feature reconciliations", path: "/api/features/feat-1/reconciliations", code: http.StatusOK},
		{name: "reconciliation", path: "/api/reconciliations/reconciliation-1", code: http.StatusOK},
		{name: "reconciliation candidates", path: "/api/reconciliations/reconciliation-1/candidates", code: http.StatusOK},
		{name: "candidate", path: "/api/candidates/candidate-1", code: http.StatusOK},
		{name: "feature events", path: "/api/features/feat-1/events", code: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.code {
				t.Fatalf("GET %s code = %d, want %d (body: %s)", tt.path, rec.Code, tt.code, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
				t.Errorf("GET %s Content-Type = %q, want application/json", tt.path, got)
			}
			if tt.code == http.StatusOK && (rec.Body.String() == "[]\n" || !json.Valid(rec.Body.Bytes())) {
				t.Errorf("GET %s body = %q, want non-empty valid fixture JSON", tt.path, rec.Body.String())
			}
		})
	}

	for _, path := range []string{
		"/api/features/missing", "/api/attempts/missing", "/api/features/missing/lease",
		"/api/features/feat-1/overlap/missing", "/api/reconciliations/missing", "/api/candidates/missing",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound || !json.Valid(rec.Body.Bytes()) {
			t.Errorf("GET missing %s = %d %q, want JSON 404", path, rec.Code, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/features", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/features code = %d, want 405", rec.Code)
	}
}

func TestGetRoutesReturnJSON(t *testing.T) {
	ctx := context.Background()
	l := openServeLedger(t)
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)

	// Seed run with lanes
	runWithLanes := "run-with-lanes"
	if err := l.RegisterRun(ctx, ledger.Run{RunID: runWithLanes, Status: "running", LaneCount: 1, StartedAt: now}); err != nil {
		t.Fatalf("RegisterRun: %v", err)
	}
	if err := l.RegisterLane(ctx, ledger.Lane{
		RunID:            runWithLanes,
		LaneID:           "lane-1",
		PacketID:         "pkt-1",
		Executor:         "agy",
		RoutingCondition: "primary",
		Status:           lane.Done,
	}); err != nil {
		t.Fatalf("RegisterLane: %v", err)
	}
	if err := l.UpdateLaneMetadata(ctx, ledger.LaneMetadata{RunID: runWithLanes, LaneID: "lane-1", Change: "test"}, now); err != nil {
		t.Fatalf("UpdateLaneMetadata: %v", err)
	}

	// Seed run without lanes
	runEmpty := "run-empty"
	if err := l.RegisterRun(ctx, ledger.Run{RunID: runEmpty, Status: "pending", LaneCount: 0, StartedAt: now}); err != nil {
		t.Fatalf("RegisterRun empty: %v", err)
	}

	// Seed approval
	if err := l.RequestApproval(ctx, ledger.Approval{
		RunID:       runWithLanes,
		LaneID:      "lane-1",
		PacketID:    "pkt-1",
		Evidence:    "file.go:10",
		RequestedAt: now,
	}); err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}

	handler := serve.NewHandler(serve.NewModel(l), "alice", "opencode run")

	tests := []struct {
		name      string
		method    string
		path      string
		wantCode  int
		wantJSON  bool
		wantEmpty bool
		checkBody func(t *testing.T, body string)
	}{
		{
			name:     "GET approvals with data returns 200 JSON",
			method:   http.MethodGet,
			path:     "/api/approvals",
			wantCode: http.StatusOK,
			wantJSON: true,
			checkBody: func(t *testing.T, body string) {
				var approvals []ledger.Approval
				if err := json.Unmarshal([]byte(body), &approvals); err != nil {
					t.Fatalf("unmarshal approvals: %v", err)
				}
				if len(approvals) != 1 || approvals[0].LaneID != "lane-1" {
					t.Errorf("approvals = %+v, want 1 approval for lane-1", approvals)
				}
			},
		},
		{
			name:     "GET batch lanes with data returns 200 JSON",
			method:   http.MethodGet,
			path:     "/api/batch/run-with-lanes/lanes",
			wantCode: http.StatusOK,
			wantJSON: true,
			checkBody: func(t *testing.T, body string) {
				var batch []serve.BatchLane
				if err := json.Unmarshal([]byte(body), &batch); err != nil {
					t.Fatalf("unmarshal batch lanes: %v", err)
				}
				if len(batch) != 1 || batch[0].LaneID != "lane-1" {
					t.Errorf("batch = %+v, want 1 batch lane for lane-1", batch)
				}
			},
		},
		{
			name:      "GET batch lanes empty run encodes as empty array",
			method:    http.MethodGet,
			path:      "/api/batch/run-empty/lanes",
			wantCode:  http.StatusOK,
			wantJSON:  true,
			wantEmpty: true,
		},
		{
			name:     "GET batch lanes unknown run returns 404",
			method:   http.MethodGet,
			path:     "/api/batch/unknown-run/lanes",
			wantCode: http.StatusNotFound,
			wantJSON: true,
		},
		{
			name:     "GET batch missing run id returns 404",
			method:   http.MethodGet,
			path:     "/api/batch/lanes",
			wantCode: http.StatusNotFound,
			wantJSON: true,
		},
		{
			name:     "GET batch root returns 404",
			method:   http.MethodGet,
			path:     "/api/batch/",
			wantCode: http.StatusNotFound,
			wantJSON: true,
		},
		{
			name:     "POST approvals returns 405",
			method:   http.MethodPost,
			path:     "/api/approvals",
			wantCode: http.StatusMethodNotAllowed,
		},
		{
			name:     "POST batch lanes returns 405",
			method:   http.MethodPost,
			path:     "/api/batch/run-with-lanes/lanes",
			wantCode: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("%s %s code = %d, want %d (body: %s)", tt.method, tt.path, rec.Code, tt.wantCode, rec.Body.String())
			}
			if tt.wantJSON {
				if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
					t.Errorf("%s %s Content-Type = %q, want application/json", tt.method, tt.path, got)
				}
				if !json.Valid(rec.Body.Bytes()) {
					t.Errorf("%s %s body is not valid JSON: %q", tt.method, tt.path, rec.Body.String())
				}
			}
			if tt.wantEmpty {
				if strings.TrimSpace(rec.Body.String()) != "[]" {
					t.Errorf("%s %s body = %q, want []", tt.method, tt.path, rec.Body.String())
				}
			}
			if tt.checkBody != nil {
				tt.checkBody(t, rec.Body.String())
			}
		})
	}

	t.Run("GET approvals empty ledger encodes as empty array", func(t *testing.T) {
		cleanLedger := openServeLedger(t)
		cleanHandler := serve.NewHandler(serve.NewModel(cleanLedger), "alice", "opencode run")
		req := httptest.NewRequest(http.MethodGet, "/api/approvals", nil)
		rec := httptest.NewRecorder()
		cleanHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET /api/approvals code = %d, want 200", rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		if strings.TrimSpace(rec.Body.String()) != "[]" {
			t.Errorf("GET /api/approvals body = %q, want []", rec.Body.String())
		}
	})
}

func TestLegacyHandlerKeepsStateFallbackWithoutStream(t *testing.T) {
	l := openServeLedger(t)
	handler := serve.NewHandler(serve.NewModel(l), "alice", "opencode run")

	stateReq := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	stateRec := httptest.NewRecorder()
	handler.ServeHTTP(stateRec, stateReq)
	if stateRec.Code != http.StatusOK || !json.Valid(stateRec.Body.Bytes()) {
		t.Fatalf("GET /api/state = %d %q, want JSON 200", stateRec.Code, stateRec.Body.String())
	}

	streamReq := httptest.NewRequest(http.MethodGet, "/api/stream", nil)
	streamRec := httptest.NewRecorder()
	handler.ServeHTTP(streamRec, streamReq)
	if streamRec.Code != http.StatusServiceUnavailable || !json.Valid(streamRec.Body.Bytes()) {
		t.Fatalf("GET /api/stream without Hub = %d %q, want JSON 503", streamRec.Code, streamRec.Body.String())
	}
}

func TestStreamResumesFromLastEventID(t *testing.T) {
	l := openServeLedger(t)
	const runID = "run-resume"
	appendServeEvent(t, l, runID, "one")
	appendServeEvent(t, l, runID, "two")

	hub := serve.NewHub(l, runID, serve.HubConfig{SubscriberBuffer: 8})
	handler := serve.NewHandlerWithConfig(serve.NewModel(l), "alice", "opencode run", serve.HandlerConfig{Hub: hub})
	server := httptest.NewServer(handler)
	defer server.Close()

	firstResp, err := http.Get(server.URL + "/api/stream")
	if err != nil {
		t.Fatalf("GET first stream: %v", err)
	}
	if got := firstResp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("first stream Content-Type = %q, want text/event-stream", got)
	}
	first := readServerEvent(t, firstResp.Body)
	_ = firstResp.Body.Close()
	if first.Event != serve.RecordEvent || first.ID == "" || !strings.Contains(first.Data, `"detail":"one"`) {
		t.Fatalf("first event = %+v, want first durable ledger event", first)
	}

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/stream", nil)
	if err != nil {
		t.Fatalf("NewRequest resume: %v", err)
	}
	req.Header.Set("Last-Event-ID", first.ID)
	resumeResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET resumed stream: %v", err)
	}
	defer resumeResp.Body.Close()
	resumed := readServerEvent(t, resumeResp.Body)
	if resumed.Event != serve.RecordEvent || resumed.ID == first.ID || !strings.Contains(resumed.Data, `"detail":"two"`) {
		t.Fatalf("resumed event = %+v after %q, want second durable ledger event", resumed, first.ID)
	}
}

func TestStreamOverflowRequestsDurableResync(t *testing.T) {
	l := openServeLedger(t)
	const runID = "run-resync"
	for _, detail := range []string{"one", "two", "three"} {
		appendServeEvent(t, l, runID, detail)
	}

	hub := serve.NewHub(l, runID, serve.HubConfig{SubscriberBuffer: 1})
	handler := serve.NewHandlerWithConfig(serve.NewModel(l), "alice", "opencode run", serve.HandlerConfig{Hub: hub})
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/stream")
	if err != nil {
		t.Fatalf("GET overflow stream: %v", err)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("overflow stream Content-Type = %q, want text/event-stream", got)
	}
	frame := readServerEvent(t, resp.Body)
	_ = resp.Body.Close()
	if frame.Event != "resync" || frame.ID == "" || frame.Data != `{"reason":"slow_consumer"}` {
		t.Fatalf("overflow frame = %+v, want durable resync frame", frame)
	}
	cursor, err := serve.ParseCursor(frame.ID)
	if err != nil {
		t.Fatalf("ParseCursor(resync): %v", err)
	}
	if cursor.RunID != runID || cursor.EventID != 3 {
		t.Fatalf("resync cursor = %+v, want run %q event 3", cursor, runID)
	}

	appendServeEvent(t, l, runID, "four")
	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/stream", nil)
	if err != nil {
		t.Fatalf("NewRequest after resync: %v", err)
	}
	req.Header.Set("Last-Event-ID", frame.ID)
	resumed, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET after resync: %v", err)
	}
	defer resumed.Body.Close()
	next := readServerEvent(t, resumed.Body)
	if next.Event != serve.RecordEvent || !strings.Contains(next.Data, `"detail":"four"`) {
		t.Fatalf("event after resync = %+v, want fourth durable event", next)
	}
}

// TestStreamTailsSeparatelyStartedRunWhenHubHasNoRunID mirrors the actual
// `lucind-ai serve` wiring in cmd/lucind-ai/cli.go, which constructs its Hub
// with an empty runID because serve has no flag to learn the run id of a
// `lucind-ai run` process started separately (and that process only prints
// its run id after serve is already listening). Before the fix, the console
// showed "Live event stream connected" but the stream matched no rows and
// never emitted anything. A Control Room left open must show that
// independently-dispatched work without being told its run id in advance.
func TestStreamTailsSeparatelyStartedRunWhenHubHasNoRunID(t *testing.T) {
	l := openServeLedger(t)
	appendServeEvent(t, l, "run-dispatched-later", "lane dispatched")

	hub := serve.NewHub(l, "", serve.HubConfig{SubscriberBuffer: 8})
	handler := serve.NewHandlerWithConfig(serve.NewModel(l), "alice", "opencode run", serve.HandlerConfig{Hub: hub})
	server := httptest.NewServer(handler)
	defer server.Close()

	reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, server.URL+"/api/stream", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/stream: %v", err)
	}
	defer resp.Body.Close()

	frame := readServerEvent(t, resp.Body)
	if frame.Event != serve.RecordEvent || !strings.Contains(frame.Data, `"detail":"lane dispatched"`) {
		t.Fatalf("frame = %+v, want the separately-started run's event, not silence", frame)
	}
}

func TestDispatchControlsRequireExplicitEnableSameOriginAndToken(t *testing.T) {
	l := openServeLedger(t)
	requestApproval(t, l, "run-control", "lane-control")
	body := `{"decision":"approved"}`

	defaultHandler := serve.NewHandler(serve.NewModel(l), "alice", "opencode run")
	for _, path := range []string{"/approvals/run-control/lane-control", "/approvals/run-control/lane-control/defect"} {
		defaultReq := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		authorizeControlRequest(defaultReq)
		defaultRec := httptest.NewRecorder()
		defaultHandler.ServeHTTP(defaultRec, defaultReq)
		if defaultRec.Code != http.StatusForbidden {
			t.Fatalf("default control %s code = %d, want 403", path, defaultRec.Code)
		}
	}

	enabled := newDispatchEnabledHandler(l)
	tests := []struct {
		name   string
		origin string
		token  string
		code   int
	}{
		{name: "missing origin", token: "control-secret", code: http.StatusForbidden},
		{name: "cross origin", origin: "http://attacker.example", token: "control-secret", code: http.StatusForbidden},
		{name: "missing token", origin: "http://example.com", code: http.StatusUnauthorized},
		{name: "wrong token", origin: "http://example.com", token: "wrong", code: http.StatusUnauthorized},
		{name: "authorized", origin: "http://example.com", token: "control-secret", code: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/approvals/run-control/lane-control", strings.NewReader(body))
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			rec := httptest.NewRecorder()
			enabled.ServeHTTP(rec, req)
			if rec.Code != tt.code {
				t.Fatalf("control code = %d, want %d (body: %s)", rec.Code, tt.code, rec.Body.String())
			}
		})
	}

	approval, err := l.Approval(context.Background(), "run-control", "lane-control")
	if err != nil {
		t.Fatalf("Approval: %v", err)
	}
	if approval.Decision != ledger.DecisionApproved {
		t.Fatalf("decision = %q, want approved only after authorized request", approval.Decision)
	}
}

const controlToken = "control-secret"

func openServeLedger(t *testing.T) *ledger.Ledger {
	t.Helper()
	l, err := ledger.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("ledger.Open: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l
}

func newDispatchEnabledHandler(l *ledger.Ledger) http.Handler {
	return serve.NewHandlerWithConfig(serve.NewModel(l), "alice", "opencode run --agent build -m openai/gpt-5.6-sol", serve.HandlerConfig{
		EnableDispatch: true,
		DispatchToken:  controlToken,
	})
}

func authorizeControlRequest(req *http.Request) {
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Authorization", "Bearer "+controlToken)
}

func requestApproval(t *testing.T, l *ledger.Ledger, runID, laneID string) {
	t.Helper()
	if err := l.RequestApproval(context.Background(), ledger.Approval{
		RunID: runID, LaneID: laneID, PacketID: "packet-" + laneID,
		Evidence: "file.go:10", RequestedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}
}

func seedModelReadRows(t *testing.T, l *ledger.Ledger) {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
	expires := time.Date(2026, 8, 22, 13, 0, 0, 0, time.UTC).Format(time.RFC3339)
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO features (id, parent_ref, base_sha, expected_parent_sha, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, []any{"feat-1", "feature-parent", "base-sha", "parent-sha", "active", now, now}},
		{`INSERT INTO integration_attempts (id, feature_id, idempotency_key, status, owner, fence, candidate_sha, failure_reason, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, []any{"attempt-1", "feat-1", "key-1", "recorded", "alice", 1, "candidate-sha", "", now, now}},
		{`INSERT INTO feature_leases (feature_id, owner, fence, expires_at, acquired_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, []any{"feat-1", "alice", 1, expires, now, now}},
		{`INSERT INTO overlap_evidence (feature_id, version, evidence_hash, evidence_class, evidence_json, created_at) VALUES (?, ?, ?, ?, ?, ?)`, []any{"feat-1", "v1", "overlap-1", "warning", `{}`, now}},
		{`INSERT INTO reconciliation_requests (id, feature_id, direction, status, actor, evidence_version, evidence_hash, source_sha, target_sha, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, []any{"reconciliation-1", "feat-1", "forward", "awaiting", "alice", "v1", "overlap-1", "source-sha", "target-sha", now, now}},
		{`INSERT INTO reconciliation_candidates (id, request_id, status, allowed_paths, model, config, output, checks, candidate_sha, failure_reason, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, []any{"candidate-1", "reconciliation-1", "candidate_running", "internal/serve", "model", "config", "output", "checks", "candidate-sha", "", now, now}},
		{`INSERT INTO integration_events (feature_id, attempt_id, type, detail, at) VALUES (?, ?, ?, ?, ?)`, []any{"feat-1", "attempt-1", "attempt_recorded", "fixture", now}},
	}
	for _, statement := range statements {
		if _, err := l.DB().ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("seed model rows: %v", err)
		}
	}
}

func appendServeEvent(t *testing.T, l *ledger.Ledger, runID, detail string) {
	t.Helper()
	if err := l.AppendEvent(context.Background(), ledger.Event{
		RunID: runID, Type: ledger.EventLaneNote, Detail: detail, At: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("AppendEvent(%q): %v", detail, err)
	}
}

type serverEvent struct {
	ID    string
	Event string
	Data  string
}

func readServerEvent(t *testing.T, body io.Reader) serverEvent {
	t.Helper()
	reader := bufio.NewReader(body)
	var frame serverEvent
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read SSE frame: %v", err)
		}
		line = strings.TrimSuffix(line, "\n")
		if line == "" {
			break
		}
		switch {
		case strings.HasPrefix(line, "id: "):
			frame.ID = strings.TrimPrefix(line, "id: ")
		case strings.HasPrefix(line, "event: "):
			frame.Event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			frame.Data = strings.TrimPrefix(line, "data: ")
		}
	}
	if frame.ID == "" || frame.Event == "" || !json.Valid([]byte(frame.Data)) {
		t.Fatalf("invalid SSE frame: %+v", frame)
	}
	return frame
}

func TestStateResponseCarriesServerTime(t *testing.T) {
	l := openServeLedger(t)
	handler := serve.NewHandler(serve.NewModel(l), "alice", "opencode run")

	before := time.Now().UTC().Add(-time.Second)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/state", nil))
	after := time.Now().UTC().Add(time.Second)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var state serve.ServerState
	if err := json.NewDecoder(rec.Body).Decode(&state); err != nil {
		t.Fatalf("decode /api/state: %v", err)
	}

	// The client measures its clock offset from this field; a zero value would
	// silently anchor every lease countdown to the viewer's own clock instead,
	// which is exactly the bug the field exists to prevent.
	if state.ServerTime.IsZero() {
		t.Fatal("ServerTime is zero -- the client would fall back to the browser clock with nothing on screen to say so")
	}
	if state.ServerTime.Before(before) || state.ServerTime.After(after) {
		t.Errorf("ServerTime = %v, want a timestamp within [%v, %v]", state.ServerTime, before, after)
	}
}

// TestStateResponseCarriesFleetFeatureAndTimelineData reproduces the Control
// Room symptom directly: a ledger with runs, lanes, progress, lifecycle
// events, features, and their attempts/leases/overlap/reconciliation rows
// must come back from /api/state under the exact top-level keys app.js reads
// (see refreshState/renderState in static/app.js), not just the five
// approval-only fields ServerState used to carry. Before the fix, every one
// of these keys is absent from the JSON object entirely, which is why the
// console renders "No lanes are reporting yet", "No SDD flows reported",
// "No features reported", and "Showing 0 of 0 events" even though the
// ledger is full.
func TestStateResponseCarriesFleetFeatureAndTimelineData(t *testing.T) {
	l := openServeLedger(t)
	ctx := context.Background()
	started := time.Date(2026, 8, 22, 9, 0, 0, 0, time.UTC)

	if err := l.RegisterRun(ctx, ledger.Run{
		RunID: "run-1", FeatureID: "feat-1", Status: "running",
		TargetRef: "refs/heads/feat-1", LaneCount: 1, StartedAt: started,
	}); err != nil {
		t.Fatalf("RegisterRun: %v", err)
	}
	if err := l.RegisterLane(ctx, ledger.Lane{
		RunID: "run-1", LaneID: "lane-a", PacketID: "packet-a",
		Executor: "opencode", RoutingCondition: "primary", Status: lane.Running,
	}); err != nil {
		t.Fatalf("RegisterLane: %v", err)
	}
	if err := l.AppendProgress(ctx, ledger.LaneProgress{
		RunID: "run-1", LaneID: "lane-a", Seq: 1, Message: "chunk-1", At: started.Add(time.Second),
	}); err != nil {
		t.Fatalf("AppendProgress: %v", err)
	}
	appendServeEvent(t, l, "run-1", "lane dispatched")
	seedModelReadRows(t, l)

	handler := serve.NewHandler(serve.NewModel(l), "alice", "opencode run")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/state", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/state code = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var state map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
		t.Fatalf("decode /api/state: %v", err)
	}

	for _, key := range []string{
		"runs", "lanes", "events", "lane_progress", "sdd_flows",
		"features", "feature_leases", "integration_attempts",
		"overlap_evidence", "reconciliation_requests", "integration_events",
	} {
		value, ok := state[key]
		if !ok {
			t.Errorf("state[%q] missing from /api/state", key)
			continue
		}
		arr, ok := value.([]any)
		if !ok {
			t.Errorf("state[%q] = %T, want a JSON array", key, value)
			continue
		}
		if len(arr) == 0 {
			t.Errorf("state[%q] is empty, want at least one row seeded by the fixture", key)
		}
	}
}

// TestStateResponseBoundsRunsAndLanes pins the payload-size safeguard: with
// far more runs than the documented bound, /api/state must still return only
// the newest window rather than serializing the whole run history on every
// poll. Update the expected count here if the documented bound in
// serveStateJSON's helpers changes.
func TestStateResponseBoundsRunsAndLanes(t *testing.T) {
	const (
		seedRuns   = 60
		wantAtMost = 50 // must match the documented run-window bound
	)
	l := openServeLedger(t)
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < seedRuns; i++ {
		runID := fmt.Sprintf("run-%03d", i)
		if err := l.RegisterRun(ctx, ledger.Run{
			RunID: runID, Status: "done", StartedAt: base.Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatalf("RegisterRun(%s): %v", runID, err)
		}
	}
	newestRunID := fmt.Sprintf("run-%03d", seedRuns-1)

	handler := serve.NewHandler(serve.NewModel(l), "alice", "opencode run")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/state", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/state code = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var state struct {
		Runs []map[string]any `json:"runs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
		t.Fatalf("decode /api/state: %v", err)
	}
	if len(state.Runs) == 0 {
		t.Fatal("state.runs is empty, want the bounded newest-first window")
	}
	if len(state.Runs) > wantAtMost {
		t.Errorf("state.runs has %d entries, want at most %d", len(state.Runs), wantAtMost)
	}
	if got := state.Runs[0]["run_id"]; got != newestRunID {
		t.Errorf("state.runs[0].run_id = %v, want newest run %q (newest-first order)", got, newestRunID)
	}
}
