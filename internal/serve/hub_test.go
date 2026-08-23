package serve

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
)

func TestHubSSEOrdersRowsAndResumesFromLastEventID(t *testing.T) {
	l := openHubLedger(t)
	ctx := context.Background()
	const runID = "run-resume"
	base := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)

	appendHubEvent(t, l, ledger.Event{RunID: runID, Type: ledger.EventRunStarted, Detail: "started", At: base})
	appendHubProgress(t, l, ledger.LaneProgress{RunID: runID, LaneID: "lane-b", Seq: 1, Message: "b1", At: base.Add(time.Second)})
	appendHubProgress(t, l, ledger.LaneProgress{RunID: runID, LaneID: "lane-a", Seq: 1, Message: "a1", At: base.Add(time.Second)})
	appendHubEvent(t, l, ledger.Event{RunID: runID, Type: ledger.EventRunEnded, Detail: "ended", At: base.Add(2 * time.Second)})

	hub := NewHub(l, runID, HubConfig{SubscriberBuffer: 8})
	server := httptest.NewServer(hub)
	t.Cleanup(server.Close)

	firstCtx, cancelFirst := context.WithCancel(ctx)
	firstReq, err := http.NewRequestWithContext(firstCtx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest(first): %v", err)
	}
	firstResp, err := server.Client().Do(firstReq)
	if err != nil {
		t.Fatalf("Do(first): %v", err)
	}
	if got := firstResp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}

	firstReader := bufio.NewReader(firstResp.Body)
	first := readSSEFrame(t, firstReader)
	second := readSSEFrame(t, firstReader)
	if first.ID == "" || second.ID == "" {
		t.Fatalf("SSE ids = %q, %q; both frames must contain id fields", first.ID, second.ID)
	}
	if got := recordKey(first.Record); got != "event:1" {
		t.Fatalf("first record = %q, want event:1", got)
	}
	if got := recordKey(second.Record); got != "progress:lane-a:1" {
		t.Fatalf("second record = %q, want progress:lane-a:1", got)
	}
	resumeID := second.ID
	cancelFirst()
	firstResp.Body.Close()

	secondCtx, cancelSecond := context.WithCancel(ctx)
	defer cancelSecond()
	resumeReq, err := http.NewRequestWithContext(secondCtx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest(resume): %v", err)
	}
	resumeReq.Header.Set("Last-Event-ID", resumeID)
	resumeResp, err := server.Client().Do(resumeReq)
	if err != nil {
		t.Fatalf("Do(resume): %v", err)
	}
	defer resumeResp.Body.Close()

	resumeReader := bufio.NewReader(resumeResp.Body)
	third := readSSEFrame(t, resumeReader)
	fourth := readSSEFrame(t, resumeReader)
	got := []string{recordKey(first.Record), recordKey(second.Record), recordKey(third.Record), recordKey(fourth.Record)}
	want := []string{"event:1", "progress:lane-a:1", "progress:lane-b:1", "event:2"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ordered resumed records = %v, want %v", got, want)
	}
	if third.ID == "" || fourth.ID == "" || third.ID == resumeID {
		t.Fatalf("resumed ids = %q, %q after %q; want advancing non-empty ids", third.ID, fourth.ID, resumeID)
	}
}

func TestHubCatchUpOverflowResyncCarriesLatestDurableCursor(t *testing.T) {
	l := openHubLedger(t)
	const runID = "run-catch-up-overflow"
	base := time.Date(2026, 8, 22, 12, 30, 0, 0, time.UTC)
	for i, detail := range []string{"one", "two", "three"} {
		appendHubEvent(t, l, ledger.Event{
			RunID:  runID,
			Type:   ledger.EventLaneNote,
			Detail: detail,
			At:     base.Add(time.Duration(i) * time.Second),
		})
	}

	hub := NewHub(l, runID, HubConfig{SubscriberBuffer: 1})
	overflowed, err := hub.Subscribe(context.Background(), Cursor{})
	if err != nil {
		t.Fatalf("Subscribe(overflowed): %v", err)
	}
	defer overflowed.Close()
	resync := receiveDelivery(t, overflowed)
	if !resync.Resync {
		t.Fatalf("catch-up delivery = %+v, want resync", resync)
	}

	var output strings.Builder
	if err := writeSSE(&output, resync); err != nil {
		t.Fatalf("writeSSE(resync): %v", err)
	}
	frame := readSSEFrame(t, bufio.NewReader(strings.NewReader(output.String())))
	if frame.Event != "resync" || frame.ID == "" {
		t.Fatalf("resync frame = event %q id %q, want resync with durable id", frame.Event, frame.ID)
	}
	resumeCursor, err := ParseCursor(frame.ID)
	if err != nil {
		t.Fatalf("ParseCursor(resync id): %v", err)
	}
	if resumeCursor.EventID != 3 {
		t.Fatalf("resync event cursor = %d, want latest durable event id 3", resumeCursor.EventID)
	}

	resumed, err := hub.Subscribe(context.Background(), resumeCursor)
	if err != nil {
		t.Fatalf("Subscribe(resumed): %v", err)
	}
	defer resumed.Close()
	assertNoImmediateDelivery(t, resumed)
}

func TestHubRejectsCursorFromAnotherRun(t *testing.T) {
	l := openHubLedger(t)
	base := time.Date(2026, 8, 22, 12, 45, 0, 0, time.UTC)
	appendHubProgress(t, l, ledger.LaneProgress{
		RunID: "run-a", LaneID: "lane-a", Seq: 1, Message: "run-a-progress", At: base,
	})
	appendHubProgress(t, l, ledger.LaneProgress{
		RunID: "run-b", LaneID: "lane-a", Seq: 1, Message: "run-b-progress", At: base,
	})

	hubA := NewHub(l, "run-a", HubConfig{SubscriberBuffer: 2})
	subA, err := hubA.Subscribe(context.Background(), Cursor{})
	if err != nil {
		t.Fatalf("Subscribe(run-a): %v", err)
	}
	runACursor := receiveDelivery(t, subA).Cursor
	subA.Close()

	hubB := NewHub(l, "run-b", HubConfig{SubscriberBuffer: 2})
	if _, err := hubB.Subscribe(context.Background(), runACursor); err == nil {
		t.Fatal("Subscribe(run-b, run-a cursor) error = nil, want run mismatch rejection")
	}

	fresh, err := hubB.Subscribe(context.Background(), Cursor{})
	if err != nil {
		t.Fatalf("Subscribe(run-b, empty cursor): %v", err)
	}
	defer fresh.Close()
	if got := receiveDelivery(t, fresh); recordKey(got.Record) != "progress:lane-a:1" {
		t.Fatalf("fresh run-b delivery = %+v, want run-b lane-a progress", got)
	}
}

func TestHubSSEDisconnectUnregistersSubscriber(t *testing.T) {
	l := openHubLedger(t)
	hub := NewHub(l, "run-disconnect", HubConfig{SubscriberBuffer: 2})
	server := httptest.NewServer(hub)
	t.Cleanup(server.Close)

	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	waitForSubscriberCount(t, hub, 1)
	cancel()
	resp.Body.Close()
	waitForSubscriberCount(t, hub, 0)
}

func TestHubSlowConsumerResyncDoesNotBlockOtherSubscribers(t *testing.T) {
	l := openHubLedger(t)
	const runID = "run-slow"
	hub := NewHub(l, runID, HubConfig{PollInterval: 5 * time.Millisecond, SubscriberBuffer: 1})

	slow, err := hub.Subscribe(context.Background(), Cursor{})
	if err != nil {
		t.Fatalf("Subscribe(slow): %v", err)
	}
	defer slow.Close()
	fast, err := hub.Subscribe(context.Background(), Cursor{})
	if err != nil {
		t.Fatalf("Subscribe(fast): %v", err)
	}
	defer fast.Close()

	runCtx, cancelRun := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- hub.Run(runCtx) }()
	t.Cleanup(func() {
		cancelRun()
		if err := <-runErr; err != nil {
			t.Errorf("Run() error = %v", err)
		}
	})

	base := time.Date(2026, 8, 22, 13, 0, 0, 0, time.UTC)
	appendHubEvent(t, l, ledger.Event{RunID: runID, Type: ledger.EventRunStarted, Detail: "one", At: base})
	if got := receiveDelivery(t, fast); recordKey(got.Record) != "event:1" || got.Resync {
		t.Fatalf("fast first delivery = %+v, want event:1", got)
	}

	appendHubEvent(t, l, ledger.Event{RunID: runID, Type: ledger.EventLaneNote, Detail: "two", At: base.Add(time.Second)})
	if got := receiveDelivery(t, fast); recordKey(got.Record) != "event:2" || got.Resync {
		t.Fatalf("fast second delivery = %+v, want event:2", got)
	}
	if got := receiveDelivery(t, slow); !got.Resync {
		t.Fatalf("slow delivery = %+v, want resync signal", got)
	}

	appendHubProgress(t, l, ledger.LaneProgress{RunID: runID, LaneID: "lane-a", Seq: 1, Message: "still-live", At: base.Add(2 * time.Second)})
	if got := receiveDelivery(t, fast); recordKey(got.Record) != "progress:lane-a:1" || got.Resync {
		t.Fatalf("fast delivery after slow resync = %+v, want progress:lane-a:1", got)
	}
}

// TestHubEmptyRunIDTailsEveryRun reproduces the actual `lucind-ai serve`
// wiring (cmd/lucind-ai/cli.go constructs its hub with an empty runID because
// it has no way to learn the run id of a separately-started `lucind-ai run`
// process). Before the fix, queryEventsAfter filters `WHERE run_id = ?` with
// that empty string, which never matches a real row, so the stream connects
// but silently emits nothing forever. An empty runID must mean "tail every
// run, unfiltered" instead.
func TestHubEmptyRunIDTailsEveryRun(t *testing.T) {
	l := openHubLedger(t)
	base := time.Date(2026, 8, 22, 14, 0, 0, 0, time.UTC)
	appendHubEvent(t, l, ledger.Event{RunID: "run-a", Type: ledger.EventRunStarted, Detail: "a-started", At: base})
	appendHubEvent(t, l, ledger.Event{RunID: "run-b", Type: ledger.EventRunStarted, Detail: "b-started", At: base.Add(time.Second)})
	appendHubProgress(t, l, ledger.LaneProgress{RunID: "run-b", LaneID: "lane-1", Seq: 1, Message: "b-progress", At: base.Add(2 * time.Second)})

	hub := NewHub(l, "", HubConfig{SubscriberBuffer: 8})
	sub, err := hub.Subscribe(context.Background(), Cursor{})
	if err != nil {
		t.Fatalf("Subscribe(empty hub runID): %v", err)
	}
	defer sub.Close()

	first := receiveDelivery(t, sub)
	second := receiveDelivery(t, sub)
	third := receiveDelivery(t, sub)
	if first.Resync || second.Resync || third.Resync {
		t.Fatalf("unexpected resync in tail-all catch-up: %+v %+v %+v", first, second, third)
	}
	if first.Record.RunID != "run-a" || first.Record.Detail != "a-started" {
		t.Fatalf("first record = %+v, want run-a's event", first.Record)
	}
	if second.Record.RunID != "run-b" || second.Record.Detail != "b-started" {
		t.Fatalf("second record = %+v, want run-b's event", second.Record)
	}
	if third.Record.RunID != "run-b" || third.Record.Kind != RecordProgress {
		t.Fatalf("third record = %+v, want run-b's progress", third.Record)
	}
}

// TestHubEmptyRunIDPreservesProgressAcrossCollidingLaneIDs guards the sharp
// edge in "progress is already keyed by lane id": lane ids are packet ids
// (see internal/run), not globally unique, so two concurrent runs commonly
// reuse the same lane id (TestHubRejectsCursorFromAnotherRun above seeds
// exactly this: "lane-a" under both run-a and run-b). A cursor that tracked
// progress by bare lane id would let run-b's lane-a rows get silently
// swallowed as "already delivered" once run-a's lane-a advanced the shared
// key past them. The fix must key progress by (run id, lane id).
func TestHubEmptyRunIDPreservesProgressAcrossCollidingLaneIDs(t *testing.T) {
	l := openHubLedger(t)
	base := time.Date(2026, 8, 22, 14, 30, 0, 0, time.UTC)
	appendHubProgress(t, l, ledger.LaneProgress{RunID: "run-a", LaneID: "lane-a", Seq: 5, Message: "a5", At: base})

	hub := NewHub(l, "", HubConfig{SubscriberBuffer: 8})
	sub, err := hub.Subscribe(context.Background(), Cursor{})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	first := receiveDelivery(t, sub)
	if first.Record.RunID != "run-a" || first.Record.Seq != 5 {
		t.Fatalf("first delivery = %+v, want run-a lane-a seq 5", first.Record)
	}
	resumeCursor := first.Cursor
	sub.Close()

	// A different run reuses the same lane id at a lower sequence number.
	appendHubProgress(t, l, ledger.LaneProgress{RunID: "run-b", LaneID: "lane-a", Seq: 1, Message: "b1", At: base.Add(time.Second)})

	resumed, err := hub.Subscribe(context.Background(), resumeCursor)
	if err != nil {
		t.Fatalf("Subscribe(resumed): %v", err)
	}
	defer resumed.Close()
	second := receiveDelivery(t, resumed)
	if second.Resync {
		t.Fatalf("resumed delivery = %+v, want run-b lane-a progress, not a resync", second)
	}
	if second.Record.RunID != "run-b" || second.Record.LaneID != "lane-a" || second.Record.Seq != 1 {
		t.Fatalf("resumed delivery = %+v, want run-b's lane-a seq 1 (must not be swallowed by run-a's cursor entry)", second.Record)
	}
}

// TestHubEmptyRunIDRejectsCursorBoundToOneRun keeps cursor/resync semantics
// coherent: a cursor minted by (or for) a single-run hub carries a specific
// RunID and cannot be meaningfully resumed against an unfiltered multi-run
// tail, so it must be rejected rather than silently reinterpreted.
func TestHubEmptyRunIDRejectsCursorBoundToOneRun(t *testing.T) {
	l := openHubLedger(t)
	hub := NewHub(l, "", HubConfig{SubscriberBuffer: 2})
	if _, err := hub.Subscribe(context.Background(), Cursor{RunID: "run-a", EventID: 1}); err == nil {
		t.Fatal("Subscribe(run-bound cursor on tail-all hub) error = nil, want rejection")
	}
}

func TestHubSSERejectsMalformedLastEventID(t *testing.T) {
	l := openHubLedger(t)
	hub := NewHub(l, "run-bad-cursor", HubConfig{SubscriberBuffer: 2})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Last-Event-ID", "not-a-cursor")
	recorder := httptest.NewRecorder()

	hub.ServeHTTP(recorder, req)

	if got := recorder.Code; got != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", got, http.StatusBadRequest)
	}
}

type sseFrame struct {
	ID     string
	Event  string
	Record Record
}

func readSSEFrame(t *testing.T, reader *bufio.Reader) sseFrame {
	t.Helper()
	frame := sseFrame{}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				t.Fatalf("SSE stream ended before a complete frame: %+v", frame)
			}
			t.Fatalf("read SSE frame: %v", err)
		}
		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			return frame
		}
		switch {
		case strings.HasPrefix(line, "id: "):
			frame.ID = strings.TrimPrefix(line, "id: ")
		case strings.HasPrefix(line, "event: "):
			frame.Event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &frame.Record); err != nil {
				t.Fatalf("decode SSE data %q: %v", line, err)
			}
		}
	}
}

func openHubLedger(t *testing.T) *ledger.Ledger {
	t.Helper()
	l, err := ledger.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("ledger.Open: %v", err)
	}
	t.Cleanup(func() {
		if err := l.Close(); err != nil {
			t.Errorf("Ledger.Close: %v", err)
		}
	})
	return l
}

func appendHubEvent(t *testing.T, l *ledger.Ledger, event ledger.Event) {
	t.Helper()
	if err := l.AppendEvent(context.Background(), event); err != nil {
		t.Fatalf("AppendEvent(%q): %v", event.Detail, err)
	}
}

func appendHubProgress(t *testing.T, l *ledger.Ledger, progress ledger.LaneProgress) {
	t.Helper()
	if err := l.AppendProgress(context.Background(), progress); err != nil {
		t.Fatalf("AppendProgress(%q): %v", progress.Message, err)
	}
}

func recordKey(record Record) string {
	if record.Kind == RecordProgress {
		return record.Kind + ":" + record.LaneID + ":" + strconv.FormatInt(record.Seq, 10)
	}
	return record.Kind + ":" + strconv.FormatInt(record.EventID, 10)
}

func receiveDelivery(t *testing.T, subscription *Subscription) Delivery {
	t.Helper()
	select {
	case delivery, ok := <-subscription.Events():
		if !ok {
			t.Fatal("subscription closed before delivery")
		}
		return delivery
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for hub delivery")
		return Delivery{}
	}
}

func assertNoImmediateDelivery(t *testing.T, subscription *Subscription) {
	t.Helper()
	select {
	case delivery, ok := <-subscription.Events():
		if !ok {
			t.Fatal("subscription closed unexpectedly")
		}
		t.Fatalf("unexpected immediate delivery after durable resync cursor: %+v", delivery)
	default:
	}
}

func waitForSubscriberCount(t *testing.T, hub *Hub, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		hub.mu.Lock()
		got := len(hub.subscribers)
		hub.mu.Unlock()
		if got == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	hub.mu.Lock()
	got := len(hub.subscribers)
	hub.mu.Unlock()
	t.Fatalf("subscriber count = %d, want %d", got, want)
}
