package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
)

func TestUpdateAndGetLaneMetadata(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()
	registerLaneForMetadataTest(t, l, "run-meta", "lane-a")

	want := LaneMetadata{
		RunID:        "run-meta",
		LaneID:       "lane-a",
		Model:        "openai/gpt-5.6-sol",
		Agent:        "build",
		SDDPhase:     "apply",
		FanoutGroup:  "ledger",
		Change:       "meta-ledger",
		Feature:      "feature/meta-ledger",
		Skill:        "lucind-apply",
		PacketPath:   ".lucind/packets/apply-lane-status-observability.md",
		AllowedPaths: []string{"internal/ledger/lanes_meta.go", "internal/ledger/lanes_meta_test.go"},
		Dependencies: []string{"schema-v6"},
		BodyDigest:   "sha256:0123456789abcdef",
	}
	at := time.Date(2026, time.August, 22, 12, 0, 0, 0, time.UTC)

	if err := l.UpdateLaneMetadata(ctx, want, at); err != nil {
		t.Fatalf("UpdateLaneMetadata() error = %v", err)
	}

	got, err := l.GetLaneMetadata(ctx, want.RunID, want.LaneID)
	if err != nil {
		t.Fatalf("GetLaneMetadata() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetLaneMetadata() = %+v, want %+v", got, want)
	}

	var (
		rowCount                                int
		packetID, executor, routing, statusText string
		model, agent, feature                   string
	)
	err = l.db.QueryRowContext(ctx, `
		SELECT COUNT(*), packet_id, executor, routing_condition, status,
		       COALESCE(model, ''), COALESCE(agent, ''), COALESCE(feature, '')
		FROM lanes WHERE run_id = ? AND lane_id = ?`, want.RunID, want.LaneID).
		Scan(&rowCount, &packetID, &executor, &routing, &statusText, &model, &agent, &feature)
	if err != nil {
		t.Fatalf("query lane identity: %v", err)
	}
	if rowCount != 1 || packetID != "packet-lane-a" || executor != "agy" ||
		routing != "metadata test" || statusText != string(lane.Pending) {
		t.Fatalf("lane identity changed: count=%d packet=%q executor=%q routing=%q status=%q",
			rowCount, packetID, executor, routing, statusText)
	}
	if model != want.Model || agent != want.Agent || feature != want.Feature {
		t.Fatalf("v6 lane columns = (%q, %q, %q), want (%q, %q, %q)",
			model, agent, feature, want.Model, want.Agent, want.Feature)
	}

	audits := laneMetadataAudits(t, l, want.RunID)
	if len(audits) != 1 {
		t.Fatalf("metadata audit count = %d, want 1", len(audits))
	}
	if audits[0].LaneID != want.LaneID || !audits[0].At.Equal(at) {
		t.Fatalf("metadata audit identity/time = (%q, %s), want (%q, %s)",
			audits[0].LaneID, audits[0].At, want.LaneID, at)
	}
	if !reflect.DeepEqual(decodeLaneMetadataAudit(t, audits[0].Detail), want) {
		t.Fatalf("metadata audit detail does not contain the written snapshot: %s", audits[0].Detail)
	}
}

func TestUpdateLaneMetadataKeepsAuditHistoryAndReturnsLatestSnapshot(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()
	registerLaneForMetadataTest(t, l, "run-updates", "lane-b")

	if err := l.AppendEvent(ctx, Event{
		RunID:  "run-updates",
		LaneID: "lane-b",
		Type:   EventLaneNote,
		Detail: "ordinary diagnostic note",
		At:     time.Date(2026, time.August, 22, 12, 29, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}

	updates := []struct {
		at   time.Time
		meta LaneMetadata
	}{
		{
			at: time.Date(2026, time.August, 22, 12, 30, 0, 0, time.UTC),
			meta: LaneMetadata{
				RunID: "run-updates", LaneID: "lane-b", Model: "model-a", Agent: "agent-a",
				SDDPhase: "design", FanoutGroup: "lens", Change: "change-a", Feature: "feature-a",
				AllowedPaths: []string{"a.go"}, Dependencies: []string{"packet-a"}, BodyDigest: "sha256:first",
			},
		},
		{
			at: time.Date(2026, time.August, 22, 12, 31, 0, 0, time.UTC),
			meta: LaneMetadata{
				RunID: "run-updates", LaneID: "lane-b", Model: "model-b", Agent: "agent-b",
				SDDPhase: "apply", FanoutGroup: "ledger", Change: "change-b", Feature: "feature-b",
				AllowedPaths: []string{"b.go", "b_test.go"}, Dependencies: []string{"packet-b", "packet-c"}, BodyDigest: "sha256:second",
			},
		},
	}

	for _, update := range updates {
		if err := l.UpdateLaneMetadata(ctx, update.meta, update.at); err != nil {
			t.Fatalf("UpdateLaneMetadata(%q) error = %v", update.meta.BodyDigest, err)
		}
	}

	got, err := l.GetLaneMetadata(ctx, "run-updates", "lane-b")
	if err != nil {
		t.Fatalf("GetLaneMetadata() error = %v", err)
	}
	if !reflect.DeepEqual(got, updates[1].meta) {
		t.Fatalf("GetLaneMetadata() = %+v, want latest snapshot %+v", got, updates[1].meta)
	}

	audits := laneMetadataAudits(t, l, "run-updates")
	if len(audits) != len(updates) {
		t.Fatalf("metadata audit count = %d, want %d", len(audits), len(updates))
	}
	for i, audit := range audits {
		gotSnapshot := decodeLaneMetadataAudit(t, audit.Detail)
		if !reflect.DeepEqual(gotSnapshot, updates[i].meta) {
			t.Errorf("audit %d snapshot = %+v, want %+v", i, gotSnapshot, updates[i].meta)
		}
	}

	var laneRows int
	if err := l.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM lanes WHERE run_id = ? AND lane_id = ?`, "run-updates", "lane-b").Scan(&laneRows); err != nil {
		t.Fatalf("count lane rows: %v", err)
	}
	if laneRows != 1 {
		t.Fatalf("lane row count after metadata updates = %d, want 1", laneRows)
	}
}

func TestLaneMetadataUnknownLane(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()
	meta := LaneMetadata{RunID: "missing-run", LaneID: "missing-lane", Model: "unused"}

	if err := l.UpdateLaneMetadata(ctx, meta, time.Now()); !errors.Is(err, ErrLaneUnknown) {
		t.Fatalf("UpdateLaneMetadata() error = %v, want ErrLaneUnknown", err)
	}
	if _, err := l.GetLaneMetadata(ctx, meta.RunID, meta.LaneID); !errors.Is(err, ErrLaneUnknown) {
		t.Fatalf("GetLaneMetadata() error = %v, want ErrLaneUnknown", err)
	}

	var eventCount int
	if err := l.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM events WHERE run_id = ? AND lane_id = ?`, meta.RunID, meta.LaneID).Scan(&eventCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventCount != 0 {
		t.Fatalf("unknown-lane update wrote %d audit events, want 0", eventCount)
	}
}

func TestGetLaneMetadataWithoutAuditReturnsV6Columns(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()
	registerLaneForMetadataTest(t, l, "run-legacy", "lane-c")

	if _, err := l.db.ExecContext(ctx, `
		UPDATE lanes SET model = ?, agent = ?, feature = ?
		WHERE run_id = ? AND lane_id = ?`,
		"legacy-model", "legacy-agent", "legacy-feature", "run-legacy", "lane-c"); err != nil {
		t.Fatalf("seed v6 metadata columns: %v", err)
	}

	got, err := l.GetLaneMetadata(ctx, "run-legacy", "lane-c")
	if err != nil {
		t.Fatalf("GetLaneMetadata() error = %v", err)
	}
	want := LaneMetadata{
		RunID:   "run-legacy",
		LaneID:  "lane-c",
		Model:   "legacy-model",
		Agent:   "legacy-agent",
		Feature: "legacy-feature",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetLaneMetadata() = %+v, want v6-column fallback %+v", got, want)
	}
	if got.Skill != "" || got.PacketPath != "" {
		t.Fatalf("v6 fallback Skill/PacketPath = (%q, %q), want empty", got.Skill, got.PacketPath)
	}
}

func registerLaneForMetadataTest(t *testing.T, l *Ledger, runID, laneID string) {
	t.Helper()
	if err := l.RegisterLane(context.Background(), Lane{
		RunID:            runID,
		LaneID:           laneID,
		PacketID:         "packet-" + laneID,
		Executor:         "agy",
		RoutingCondition: "metadata test",
		Status:           lane.Pending,
	}); err != nil {
		t.Fatalf("RegisterLane() error = %v", err)
	}
}

func laneMetadataAudits(t *testing.T, l *Ledger, runID string) []Event {
	t.Helper()
	events, err := l.Events(context.Background(), runID)
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	var audits []Event
	for _, event := range events {
		if event.Type == EventLaneNote && strings.HasPrefix(event.Detail, laneMetadataAuditPrefix) {
			audits = append(audits, event)
		}
	}
	return audits
}

func decodeLaneMetadataAudit(t *testing.T, detail string) LaneMetadata {
	t.Helper()
	var metadata LaneMetadata
	if err := json.Unmarshal([]byte(strings.TrimPrefix(detail, laneMetadataAuditPrefix)), &metadata); err != nil {
		t.Fatalf("decode metadata audit %q: %v", detail, err)
	}
	return metadata
}

func TestLaneMetadataRequiresMechanicalChecks(t *testing.T) {
	tests := []struct {
		name     string
		metadata LaneMetadata
		want     bool
	}{
		{
			name:     "apply role requires checks",
			metadata: LaneMetadata{LaneRole: "apply", ReadOnly: false},
			want:     true,
		},
		{
			name:     "ultrafixer role requires checks",
			metadata: LaneMetadata{LaneRole: "ultrafixer", ReadOnly: false},
			want:     true,
		},
		{
			name:     "empty role and not read-only requires checks",
			metadata: LaneMetadata{LaneRole: "", ReadOnly: false},
			want:     true,
		},
		{
			name:     "unknown role requires checks (fail closed)",
			metadata: LaneMetadata{LaneRole: "unknown_custom_role", ReadOnly: false},
			want:     true,
		},
		{
			name:     "lens role skips checks",
			metadata: LaneMetadata{LaneRole: "lens", ReadOnly: false},
			want:     false,
		},
		{
			name:     "synthesis role skips checks",
			metadata: LaneMetadata{LaneRole: "synthesis", ReadOnly: false},
			want:     false,
		},
		{
			name:     "verify role skips checks",
			metadata: LaneMetadata{LaneRole: "verify", ReadOnly: false},
			want:     false,
		},
		{
			name:     "archive role skips checks",
			metadata: LaneMetadata{LaneRole: "archive", ReadOnly: false},
			want:     false,
		},
		{
			name:     "human role skips checks",
			metadata: LaneMetadata{LaneRole: "human", ReadOnly: false},
			want:     false,
		},
		{
			name:     "read-only true with apply role skips checks",
			metadata: LaneMetadata{LaneRole: "apply", ReadOnly: true},
			want:     false,
		},
		{
			name:     "read-only true with empty role skips checks",
			metadata: LaneMetadata{LaneRole: "", ReadOnly: true},
			want:     false,
		},
		{
			name:     "read-only true with unknown role skips checks",
			metadata: LaneMetadata{LaneRole: "unknown_custom_role", ReadOnly: true},
			want:     false,
		},
		{
			name:     "legacy explore sdd_phase with empty role runs checks (fail closed)",
			metadata: LaneMetadata{SDDPhase: "explore", LaneRole: "", ReadOnly: false},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.metadata.RequiresMechanicalChecks()
			if got != tt.want {
				t.Errorf("RequiresMechanicalChecks() = %v, want %v for %+v", got, tt.want, tt.metadata)
			}
		})
	}
}

func TestLaneMetadataLegacyJSONDecodesZeroValues(t *testing.T) {
	legacyJSON := `{"run_id":"run-legacy","lane_id":"lane-l","model":"legacy-model","sdd_phase":"propose"}`
	var meta LaneMetadata
	if err := json.Unmarshal([]byte(legacyJSON), &meta); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if meta.LaneRole != "" {
		t.Fatalf("LaneRole = %q, want empty zero-value", meta.LaneRole)
	}
	if meta.ReadOnly {
		t.Fatalf("ReadOnly = %v, want false zero-value", meta.ReadOnly)
	}
	if meta.SDDPhase != "propose" {
		t.Fatalf("SDDPhase = %q, want propose", meta.SDDPhase)
	}
}

func TestLaneMetadataNewFieldsRoundTrip(t *testing.T) {
	l := openTestLedger(t)
	ctx := context.Background()
	registerLaneForMetadataTest(t, l, "run-new-fields", "lane-nf")

	want := LaneMetadata{
		RunID:    "run-new-fields",
		LaneID:   "lane-nf",
		Model:    "test-model",
		Agent:    "test-agent",
		LaneRole: "lens",
		ReadOnly: true,
	}
	at := time.Date(2026, time.October, 3, 8, 0, 0, 0, time.UTC)
	if err := l.UpdateLaneMetadata(ctx, want, at); err != nil {
		t.Fatalf("UpdateLaneMetadata() error = %v", err)
	}

	got, err := l.GetLaneMetadata(ctx, want.RunID, want.LaneID)
	if err != nil {
		t.Fatalf("GetLaneMetadata() error = %v", err)
	}
	if got.LaneRole != want.LaneRole {
		t.Fatalf("got LaneRole = %q, want %q", got.LaneRole, want.LaneRole)
	}
	if got.ReadOnly != want.ReadOnly {
		t.Fatalf("got ReadOnly = %v, want %v", got.ReadOnly, want.ReadOnly)
	}
}
