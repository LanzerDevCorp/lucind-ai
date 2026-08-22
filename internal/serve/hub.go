package serve

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
)

const (
	// RecordEvent identifies a lifecycle row from the events table.
	RecordEvent = "event"
	// RecordProgress identifies a sequenced row from the lane_progress table.
	RecordProgress = "progress"

	defaultPollInterval     = 100 * time.Millisecond
	defaultSubscriberBuffer = 64
	cursorVersion           = 1
)

// HubConfig controls polling and subscriber backpressure.
type HubConfig struct {
	PollInterval     time.Duration
	SubscriberBuffer int
}

// Cursor is the durable position of one SSE client across both ledger streams.
// EventID advances globally, while Progress tracks the last sequence per lane.
type Cursor struct {
	RunID    string
	EventID  int64
	Progress map[string]int64
}

type cursorPayload struct {
	Version  int              `json:"version"`
	RunID    string           `json:"run_id,omitempty"`
	EventID  int64            `json:"event_id,omitempty"`
	Progress map[string]int64 `json:"progress,omitempty"`
}

// String encodes a cursor for an SSE id field or Last-Event-ID header.
func (c Cursor) String() string {
	payload, err := json.Marshal(cursorPayload{
		Version:  cursorVersion,
		RunID:    c.RunID,
		EventID:  c.EventID,
		Progress: c.Progress,
	})
	if err != nil {
		panic(fmt.Sprintf("serve: encode cursor: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(payload)
}

// ParseCursor decodes an SSE id. An empty value means the beginning of the run.
func ParseCursor(encoded string) (Cursor, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return Cursor{}, nil
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return Cursor{}, fmt.Errorf("serve: decode cursor: %w", err)
	}
	var payload cursorPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return Cursor{}, fmt.Errorf("serve: decode cursor payload: %w", err)
	}
	if payload.Version != cursorVersion {
		return Cursor{}, fmt.Errorf("serve: unsupported cursor version %d", payload.Version)
	}
	cursor := Cursor{RunID: payload.RunID, EventID: payload.EventID, Progress: payload.Progress}
	if err := cursor.validate(); err != nil {
		return Cursor{}, err
	}
	return cursor.clone(), nil
}

func (c Cursor) validate() error {
	if c.RunID != "" && strings.TrimSpace(c.RunID) == "" {
		return fmt.Errorf("serve: cursor run id must not be blank")
	}
	if c.EventID < 0 {
		return fmt.Errorf("serve: cursor event id must not be negative")
	}
	for laneID, seq := range c.Progress {
		if strings.TrimSpace(laneID) == "" {
			return fmt.Errorf("serve: cursor lane id must not be empty")
		}
		if seq < 0 {
			return fmt.Errorf("serve: cursor progress sequence must not be negative")
		}
	}
	return nil
}

func (c Cursor) clone() Cursor {
	copyCursor := Cursor{RunID: c.RunID, EventID: c.EventID}
	if len(c.Progress) > 0 {
		copyCursor.Progress = make(map[string]int64, len(c.Progress))
		for laneID, seq := range c.Progress {
			copyCursor.Progress[laneID] = seq
		}
	}
	return copyCursor
}

// Record is one ordered lifecycle or lane-progress row sent to subscribers.
type Record struct {
	Kind    string    `json:"kind"`
	RunID   string    `json:"run_id"`
	LaneID  string    `json:"lane_id,omitempty"`
	EventID int64     `json:"event_id,omitempty"`
	Seq     int64     `json:"seq,omitempty"`
	Type    string    `json:"type,omitempty"`
	Detail  string    `json:"detail,omitempty"`
	Message string    `json:"message,omitempty"`
	At      time.Time `json:"at"`
}

// Delivery is one buffered subscriber notification. Resync is set when the
// subscriber fell behind and must reconnect using its last received SSE id.
type Delivery struct {
	Cursor Cursor
	Record Record
	Resync bool
}

type subscriber struct {
	events        chan Delivery
	resyncPending bool
}

// Subscription owns one buffered Hub subscriber.
type Subscription struct {
	hub    *Hub
	id     uint64
	once   sync.Once
	events <-chan Delivery
}

// Events returns the subscription's delivery channel.
func (s *Subscription) Events() <-chan Delivery {
	return s.events
}

// Close unregisters the subscription and closes its delivery channel.
func (s *Subscription) Close() {
	if s == nil || s.hub == nil {
		return
	}
	s.once.Do(func() { s.hub.unsubscribe(s.id) })
}

// Hub tails indexed SQLite event and progress cursors and fans records out to
// independent buffered subscribers.
type Hub struct {
	ledger           *ledger.Ledger
	runID            string
	pollInterval     time.Duration
	subscriberBuffer int

	scanMu sync.Mutex
	cursor Cursor

	mu          sync.Mutex
	nextID      uint64
	subscribers map[uint64]*subscriber
}

// NewHub constructs a tailer for one run. Run starts periodic polling; a new
// subscription also performs a synchronized catch-up query immediately.
func NewHub(l *ledger.Ledger, runID string, config HubConfig) *Hub {
	pollInterval := config.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	buffer := config.SubscriberBuffer
	if buffer <= 0 {
		buffer = defaultSubscriberBuffer
	}
	return &Hub{
		ledger:           l,
		runID:            runID,
		pollInterval:     pollInterval,
		subscriberBuffer: buffer,
		subscribers:      make(map[uint64]*subscriber),
	}
}

// Run polls until ctx is canceled. A slow subscriber never blocks this loop.
func (h *Hub) Run(ctx context.Context) error {
	if err := h.poll(ctx); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	ticker := time.NewTicker(h.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := h.poll(ctx); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}
		}
	}
}

// Subscribe catches up from cursor and then joins live fan-out atomically with
// respect to polling.
func (h *Hub) Subscribe(ctx context.Context, cursor Cursor) (*Subscription, error) {
	if err := cursor.validate(); err != nil {
		return nil, err
	}
	if cursor.RunID != "" && cursor.RunID != h.runID {
		return nil, fmt.Errorf("serve: cursor run %q does not match hub run %q", cursor.RunID, h.runID)
	}
	if cursor.RunID == "" && (cursor.EventID != 0 || len(cursor.Progress) != 0) {
		return nil, fmt.Errorf("serve: non-empty cursor is not bound to a run")
	}
	cursor = cursor.clone()

	h.scanMu.Lock()
	defer h.scanMu.Unlock()
	if err := h.pollLocked(ctx); err != nil {
		return nil, err
	}
	records, err := h.queryAfter(ctx, cursor)
	if err != nil {
		return nil, err
	}

	buffer := make(chan Delivery, h.subscriberBuffer)
	sub := &subscriber{events: buffer}
	for _, record := range records {
		cursor.advance(record)
		h.enqueue(sub, Delivery{Cursor: cursor.clone(), Record: record})
	}

	h.mu.Lock()
	h.nextID++
	id := h.nextID
	h.subscribers[id] = sub
	h.mu.Unlock()
	return &Subscription{hub: h, id: id, events: buffer}, nil
}

func (h *Hub) unsubscribe(id uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	sub, ok := h.subscribers[id]
	if !ok {
		return
	}
	delete(h.subscribers, id)
	close(sub.events)
}

func (h *Hub) poll(ctx context.Context) error {
	h.scanMu.Lock()
	defer h.scanMu.Unlock()
	return h.pollLocked(ctx)
}

func (h *Hub) pollLocked(ctx context.Context) error {
	records, err := h.queryAfter(ctx, h.cursor)
	if err != nil {
		return err
	}
	for _, record := range records {
		h.cursor.advance(record)
		h.broadcast(Delivery{Cursor: h.cursor.clone(), Record: record})
	}
	return nil
}

func (h *Hub) broadcast(delivery Delivery) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, sub := range h.subscribers {
		h.enqueue(sub, delivery)
	}
}

func (h *Hub) enqueue(sub *subscriber, delivery Delivery) {
	if sub.resyncPending {
		h.replaceResync(sub, delivery.Cursor)
		return
	}
	select {
	case sub.events <- delivery:
		return
	default:
	}

	for {
		select {
		case <-sub.events:
		default:
			sub.resyncPending = true
			h.queueResync(sub, delivery.Cursor)
			return
		}
	}
}

func (h *Hub) replaceResync(sub *subscriber, cursor Cursor) {
	for {
		select {
		case <-sub.events:
		default:
			h.queueResync(sub, cursor)
			return
		}
	}
}

func (h *Hub) queueResync(sub *subscriber, cursor Cursor) {
	select {
	case sub.events <- Delivery{Cursor: cursor.clone(), Resync: true}:
	default:
	}
}

func (c *Cursor) advance(record Record) {
	if c.RunID == "" {
		c.RunID = record.RunID
	}
	switch record.Kind {
	case RecordEvent:
		if record.EventID > c.EventID {
			c.EventID = record.EventID
		}
	case RecordProgress:
		if c.Progress == nil {
			c.Progress = make(map[string]int64)
		}
		if record.Seq > c.Progress[record.LaneID] {
			c.Progress[record.LaneID] = record.Seq
		}
	}
}

func (h *Hub) queryAfter(ctx context.Context, cursor Cursor) ([]Record, error) {
	sources := make([][]Record, 0)
	events, err := h.queryEventsAfter(ctx, cursor.EventID)
	if err != nil {
		return nil, err
	}
	if len(events) > 0 {
		sources = append(sources, events)
	}

	laneIDs, err := h.progressLaneIDs(ctx)
	if err != nil {
		return nil, err
	}
	for _, laneID := range laneIDs {
		progress, err := h.queryProgressAfter(ctx, laneID, cursor.Progress[laneID])
		if err != nil {
			return nil, err
		}
		if len(progress) > 0 {
			sources = append(sources, progress)
		}
	}
	return mergeRecordSources(sources), nil
}

func (h *Hub) queryEventsAfter(ctx context.Context, afterID int64) ([]Record, error) {
	rows, err := h.ledger.DB().QueryContext(ctx, `
		SELECT id, run_id, lane_id, type, detail, at
		FROM events
		WHERE run_id = ? AND id > ?
		ORDER BY id`, h.runID, afterID)
	if err != nil {
		return nil, fmt.Errorf("serve: query events after cursor: %w", err)
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var record Record
		var laneID sql.NullString
		var at string
		if err := rows.Scan(&record.EventID, &record.RunID, &laneID, &record.Type, &record.Detail, &at); err != nil {
			return nil, fmt.Errorf("serve: scan event after cursor: %w", err)
		}
		record.Kind = RecordEvent
		record.LaneID = laneID.String
		record.At, err = time.Parse(time.RFC3339, at)
		if err != nil {
			return nil, fmt.Errorf("serve: parse event timestamp %q: %w", at, err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("serve: iterate events after cursor: %w", err)
	}
	return records, nil
}

func (h *Hub) progressLaneIDs(ctx context.Context) ([]string, error) {
	rows, err := h.ledger.DB().QueryContext(ctx, `
		SELECT DISTINCT lane_id
		FROM lane_progress
		WHERE run_id = ?
		ORDER BY lane_id`, h.runID)
	if err != nil {
		return nil, fmt.Errorf("serve: query progress lanes: %w", err)
	}
	defer rows.Close()

	var laneIDs []string
	for rows.Next() {
		var laneID string
		if err := rows.Scan(&laneID); err != nil {
			return nil, fmt.Errorf("serve: scan progress lane: %w", err)
		}
		laneIDs = append(laneIDs, laneID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("serve: iterate progress lanes: %w", err)
	}
	return laneIDs, nil
}

func (h *Hub) queryProgressAfter(ctx context.Context, laneID string, afterSeq int64) ([]Record, error) {
	rows, err := h.ledger.DB().QueryContext(ctx, `
		SELECT run_id, lane_id, seq, message, at
		FROM lane_progress
		WHERE run_id = ? AND lane_id = ? AND seq > ?
		ORDER BY seq`, h.runID, laneID, afterSeq)
	if err != nil {
		return nil, fmt.Errorf("serve: query progress after cursor: %w", err)
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var record Record
		var at string
		if err := rows.Scan(&record.RunID, &record.LaneID, &record.Seq, &record.Message, &at); err != nil {
			return nil, fmt.Errorf("serve: scan progress after cursor: %w", err)
		}
		record.Kind = RecordProgress
		record.At, err = time.Parse(time.RFC3339, at)
		if err != nil {
			return nil, fmt.Errorf("serve: parse progress timestamp %q: %w", at, err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("serve: iterate progress after cursor: %w", err)
	}
	return records, nil
}

func mergeRecordSources(sources [][]Record) []Record {
	positions := make([]int, len(sources))
	total := 0
	for _, source := range sources {
		total += len(source)
	}
	merged := make([]Record, 0, total)
	for len(merged) < total {
		selected := -1
		for i, source := range sources {
			if positions[i] >= len(source) {
				continue
			}
			if selected == -1 || recordBefore(source[positions[i]], sources[selected][positions[selected]]) {
				selected = i
			}
		}
		merged = append(merged, sources[selected][positions[selected]])
		positions[selected]++
	}
	return merged
}

func recordBefore(left, right Record) bool {
	if !left.At.Equal(right.At) {
		return left.At.Before(right.At)
	}
	if left.Kind != right.Kind {
		return left.Kind < right.Kind
	}
	if left.LaneID != right.LaneID {
		return left.LaneID < right.LaneID
	}
	if left.Kind == RecordEvent {
		return left.EventID < right.EventID
	}
	return left.Seq < right.Seq
}

// ServeHTTP streams records as SSE and accepts Last-Event-ID or a cursor query
// parameter for reconnects.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	encodedCursor := r.URL.Query().Get("cursor")
	if encodedCursor == "" {
		encodedCursor = r.Header.Get("Last-Event-ID")
	}
	cursor, err := ParseCursor(encodedCursor)
	if err != nil {
		http.Error(w, "invalid event cursor", http.StatusBadRequest)
		return
	}
	subscription, err := h.Subscribe(r.Context(), cursor)
	if err != nil {
		http.Error(w, "telemetry stream unavailable", http.StatusInternalServerError)
		return
	}
	defer subscription.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case delivery, ok := <-subscription.Events():
			if !ok {
				return
			}
			if err := writeSSE(w, delivery); err != nil {
				return
			}
			flusher.Flush()
			if delivery.Resync {
				return
			}
		}
	}
}

func writeSSE(w io.Writer, delivery Delivery) error {
	if delivery.Resync {
		_, err := fmt.Fprintf(w, "id: %s\nevent: resync\ndata: {\"reason\":\"slow_consumer\"}\n\n", delivery.Cursor.String())
		return err
	}
	data, err := json.Marshal(delivery.Record)
	if err != nil {
		return fmt.Errorf("serve: encode SSE record: %w", err)
	}
	if _, err := fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", delivery.Cursor.String(), delivery.Record.Kind, data); err != nil {
		return fmt.Errorf("serve: write SSE record: %w", err)
	}
	return nil
}
