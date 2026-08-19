package result_test

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
)

func TestReadFullyPopulatedEnvelopeRoundTrips(t *testing.T) {
	src := `{
		"packet_id": "fix-auth",
		"status": "deviated",
		"summary": "Fixed the cookie expiry bug.",
		"hard_stops": [
			{"hard_stop": "do not touch internal/barrier", "fired": false},
			{"hard_stop": "do not commit", "fired": true, "note": "left uncommitted per instructions"}
		],
		"files_changed": [
			{"path": "internal/auth/session.go", "change": "modified", "why": "fix expiry"}
		],
		"done_criteria": [
			{"criterion": "cookies survive restart", "met": true, "evidence": "go test ./internal/auth/ -run TestSurvivesRestart: PASS"}
		],
		"questions": [
			{"question": "should the TTL be configurable?", "why_blocking": "no default is specified in the packet", "options": ["24h", "7d"], "recommendation": "24h"}
		],
		"deviations": [
			{"expected": "edit internal/auth/session.go only", "actual": "also edited internal/auth/cookie.go", "reason": "expiry logic lived there", "reversible": true}
		],
		"findings": [
			{"finding": "cookie.go has a second expiry bug", "evidence": "internal/auth/cookie.go:42", "affects": "packet fix-cookie-race"}
		],
		"commit": "abc1234",
		"session_id": "sess-789"
	}`
	fsys := fstest.MapFS{
		"result.json": {Data: []byte(src)},
	}

	e, err := result.Read(fsys, "result.json")
	if err != nil {
		t.Fatalf("Read() error = %v, want nil", err)
	}

	if e.PacketID != "fix-auth" {
		t.Errorf("PacketID = %q, want %q", e.PacketID, "fix-auth")
	}
	if e.Status != "deviated" {
		t.Errorf("Status = %q, want %q", e.Status, "deviated")
	}
	if e.Summary != "Fixed the cookie expiry bug." {
		t.Errorf("Summary = %q, want %q", e.Summary, "Fixed the cookie expiry bug.")
	}
	if e.Commit != "abc1234" {
		t.Errorf("Commit = %q, want %q", e.Commit, "abc1234")
	}
	if e.SessionID != "sess-789" {
		t.Errorf("SessionID = %q, want %q", e.SessionID, "sess-789")
	}

	if len(e.HardStops) != 2 {
		t.Fatalf("len(HardStops) = %d, want 2", len(e.HardStops))
	}
	if got, want := e.HardStops[1], (result.HardStop{HardStop: "do not commit", Fired: true, Note: "left uncommitted per instructions"}); got != want {
		t.Errorf("HardStops[1] = %+v, want %+v", got, want)
	}

	if len(e.FilesChanged) != 1 {
		t.Fatalf("len(FilesChanged) = %d, want 1", len(e.FilesChanged))
	}
	if got, want := e.FilesChanged[0], (result.FileChange{Path: "internal/auth/session.go", Change: "modified", Why: "fix expiry"}); got != want {
		t.Errorf("FilesChanged[0] = %+v, want %+v", got, want)
	}

	if len(e.DoneCriteria) != 1 {
		t.Fatalf("len(DoneCriteria) = %d, want 1", len(e.DoneCriteria))
	}
	if got, want := e.DoneCriteria[0], (result.DoneCriterion{Criterion: "cookies survive restart", Met: true, Evidence: "go test ./internal/auth/ -run TestSurvivesRestart: PASS"}); got != want {
		t.Errorf("DoneCriteria[0] = %+v, want %+v", got, want)
	}

	if len(e.Questions) != 1 {
		t.Fatalf("len(Questions) = %d, want 1", len(e.Questions))
	}
	wantQuestion := result.Question{
		Question:       "should the TTL be configurable?",
		WhyBlocking:    "no default is specified in the packet",
		Options:        []string{"24h", "7d"},
		Recommendation: "24h",
	}
	if got := e.Questions[0]; got.Question != wantQuestion.Question || got.WhyBlocking != wantQuestion.WhyBlocking ||
		got.Recommendation != wantQuestion.Recommendation || len(got.Options) != 2 || got.Options[0] != "24h" || got.Options[1] != "7d" {
		t.Errorf("Questions[0] = %+v, want %+v", got, wantQuestion)
	}

	if len(e.Deviations) != 1 {
		t.Fatalf("len(Deviations) = %d, want 1", len(e.Deviations))
	}
	wantDeviation := result.Deviation{
		Expected:   "edit internal/auth/session.go only",
		Actual:     "also edited internal/auth/cookie.go",
		Reason:     "expiry logic lived there",
		Reversible: true,
	}
	if got := e.Deviations[0]; got != wantDeviation {
		t.Errorf("Deviations[0] = %+v, want %+v", got, wantDeviation)
	}

	if len(e.Findings) != 1 {
		t.Fatalf("len(Findings) = %d, want 1", len(e.Findings))
	}
	wantFinding := result.Finding{
		Finding:  "cookie.go has a second expiry bug",
		Evidence: "internal/auth/cookie.go:42",
		Affects:  "packet fix-cookie-race",
	}
	if got := e.Findings[0]; got != wantFinding {
		t.Errorf("Findings[0] = %+v, want %+v", got, wantFinding)
	}

	if got, want := e.LaneStatus(), lane.Deviated; got != want {
		t.Errorf("LaneStatus() = %v, want %v", got, want)
	}
}

func TestReadMissingFileReturnsClearError(t *testing.T) {
	fsys := fstest.MapFS{}

	_, err := result.Read(fsys, "result.json")
	if err == nil {
		t.Fatal("Read() error = nil, want a not-found error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Read() error = %v, want it to wrap fs.ErrNotExist", err)
	}
}

func TestReadMalformedJSONReturnsClearError(t *testing.T) {
	fsys := fstest.MapFS{
		"result.json": {Data: []byte(`{"packet_id": "fix-auth", "status":`)},
	}

	_, err := result.Read(fsys, "result.json")
	if err == nil {
		t.Fatal("Read() error = nil, want a JSON parse error")
	}
	if errors.Is(err, result.ErrSchemaInvalid) {
		t.Errorf("Read() error = %v, want a parse error, not ErrSchemaInvalid", err)
	}
}

func TestReadSchemaViolations(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "missing mandatory hard_stops",
			src: `{
				"packet_id": "fix-auth",
				"status": "done",
				"summary": "Did the thing."
			}`,
		},
		{
			name: "unknown top-level property",
			src: `{
				"packet_id": "fix-auth",
				"status": "done",
				"summary": "Did the thing.",
				"hard_stops": [],
				"unexpected_field": "should not be here"
			}`,
		},
		{
			name: "status outside the enum",
			src: `{
				"packet_id": "fix-auth",
				"status": "in_progress",
				"summary": "Still working.",
				"hard_stops": []
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := fstest.MapFS{
				"result.json": {Data: []byte(tt.src)},
			}

			_, err := result.Read(fsys, "result.json")
			if err == nil {
				t.Fatal("Read() error = nil, want ErrSchemaInvalid")
			}
			if !errors.Is(err, result.ErrSchemaInvalid) {
				t.Errorf("Read() error = %v, want it to wrap ErrSchemaInvalid", err)
			}
		})
	}
}

func TestReadMinimalEnvelopeMapsLaneStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   lane.Status
	}{
		{"done", "done", lane.Done},
		{"blocked", "blocked", lane.Blocked},
		{"deviated", "deviated", lane.Deviated},
		{"failed", "failed", lane.Failed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := `{
				"packet_id": "fix-auth",
				"status": "` + tt.status + `",
				"summary": "Did the thing.",
				"hard_stops": []
			}`
			fsys := fstest.MapFS{
				"result.json": {Data: []byte(src)},
			}

			e, err := result.Read(fsys, "result.json")
			if err != nil {
				t.Fatalf("Read() error = %v, want nil", err)
			}

			if e.PacketID != "fix-auth" {
				t.Errorf("PacketID = %q, want %q", e.PacketID, "fix-auth")
			}
			if e.Status != tt.status {
				t.Errorf("Status = %q, want %q", e.Status, tt.status)
			}
			if e.Summary != "Did the thing." {
				t.Errorf("Summary = %q, want %q", e.Summary, "Did the thing.")
			}
			if len(e.HardStops) != 0 {
				t.Errorf("HardStops = %v, want empty", e.HardStops)
			}

			if got := e.LaneStatus(); got != tt.want {
				t.Errorf("LaneStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}
