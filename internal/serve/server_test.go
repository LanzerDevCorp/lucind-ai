package serve_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

	handler := serve.NewHandler(l, "alice", "opencode run --agent build -m openai/gpt-5.6-sol")

	// Array body to single item route
	arrayBody := `[{"decision":"approved"}, {"decision":"approved"}]`
	req := httptest.NewRequest(http.MethodPost, "/approvals/run-1/lane-1", bytes.NewBufferString(arrayBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("POST array body code = %d, want 400 Bad Request", rec.Code)
	}

	// Bulk object body
	bulkBody := `{"approvals": [{"run_id":"run-1","lane_id":"lane-1","decision":"approved"},{"run_id":"run-1","lane_id":"lane-2","decision":"approved"}]}`
	req2 := httptest.NewRequest(http.MethodPost, "/approvals/run-1/lane-1", bytes.NewBufferString(bulkBody))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusBadRequest {
		t.Errorf("POST bulk object body code = %d, want 400 Bad Request", rec2.Code)
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

	handler := serve.NewHandler(l, "alice", "opencode run --agent build -m openai/gpt-5.6-sol")

	emptyDecisionBody := `{"decision": ""}`
	req := httptest.NewRequest(http.MethodPost, "/approvals/run-1/lane-1", bytes.NewBufferString(emptyDecisionBody))
	req.Header.Set("Content-Type", "application/json")
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

	handler := serve.NewHandler(l, "alice", "opencode run --agent build -m openai/gpt-5.6-sol")

	// Approve lane-1
	approveBody := `{"decision": "approved"}`
	req := httptest.NewRequest(http.MethodPost, "/approvals/run-1/lane-1", bytes.NewBufferString(approveBody))
	req.Header.Set("Content-Type", "application/json")
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

	handler := serve.NewHandler(l, "alice", "opencode run --agent build -m openai/gpt-5.6-sol")

	// First decision succeeds with 200 OK
	approveBody := `{"decision": "approved"}`
	req := httptest.NewRequest(http.MethodPost, "/approvals/run-1/lane-1", bytes.NewBufferString(approveBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("First POST approve code = %d, want 200 OK (body: %s)", rec.Code, rec.Body.String())
	}

	// Second decision returns 409 Conflict
	rejectBody := `{"decision": "rejected"}`
	req2 := httptest.NewRequest(http.MethodPost, "/approvals/run-1/lane-1", bytes.NewBufferString(rejectBody))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Errorf("Second POST approve code = %d, want 409 Conflict (body: %s)", rec2.Code, rec2.Body.String())
	}
}
