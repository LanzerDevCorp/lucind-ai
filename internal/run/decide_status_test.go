package run

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
)

func TestDecideStatus_FiredHardStopDemotes(t *testing.T) {
	envelopeJSON := `{
		"packet_id": "lane-a",
		"status": "done",
		"summary": "claimed done with a fired hard stop",
		"hard_stops": [{"hard_stop": "do not guess", "fired": true, "note": "the stop fired"}]
	}`
	fsys := fstest.MapFS{
		resultEnvelopePath: {Data: []byte(envelopeJSON)},
	}
	deps := Deps{
		WorktreeFS: func(string) fs.FS { return fsys },
	}

	st, env, _ := decideStatus(deps, "/wt", executor.Outcome{ExitCode: 0})
	if env == nil {
		t.Fatal("decideStatus returned a nil envelope, want the schema-valid envelope")
	}
	if !env.HardStops[0].Fired {
		t.Fatal("fixture HardStop.Fired is false, want true")
	}
	if env.Status != "done" {
		t.Fatalf("envelope.Status = %q, want done", env.Status)
	}
	if st != lane.Blocked {
		t.Fatalf("decideStatus status = %v, want %v (HardStop.Fired must demote regardless of envelope.Status=%q)", st, lane.Blocked, env.Status)
	}
}

func TestDecideStatus_InteractionRequired(t *testing.T) {
	envelopeJSON := `{
		"packet_id": "lane-a",
		"status": "interaction_required",
		"summary": "needs user choice",
		"hard_stops": [{"hard_stop": "do not guess", "fired": false}],
		"interaction": {
			"question": "Which database engine?",
			"reason": "missing DB option",
			"unblock_response": "postgres"
		}
	}`
	fsys := fstest.MapFS{
		resultEnvelopePath: {Data: []byte(envelopeJSON)},
	}
	deps := Deps{
		WorktreeFS: func(string) fs.FS { return fsys },
	}

	st, env, reason := decideStatus(deps, "/wt", executor.Outcome{ExitCode: 0})
	if env == nil {
		t.Fatal("decideStatus returned nil envelope")
	}
	if st != lane.Blocked {
		t.Errorf("decideStatus status = %v, want %v", st, lane.Blocked)
	}
	wantReason := "interaction required: Which database engine?"
	if reason != wantReason {
		t.Errorf("decideStatus reason = %q, want %q", reason, wantReason)
	}
}

func TestDecideStatus_InteractionRequiredWithFiredHardStopDemotes(t *testing.T) {
	envelopeJSON := `{
		"packet_id": "lane-a",
		"status": "interaction_required",
		"summary": "needs user choice but hard stop fired",
		"hard_stops": [{"hard_stop": "do not delete db", "fired": true, "note": "stopped"}],
		"interaction": {
			"question": "Which database engine?",
			"reason": "missing DB option",
			"unblock_response": "postgres"
		}
	}`
	fsys := fstest.MapFS{
		resultEnvelopePath: {Data: []byte(envelopeJSON)},
	}
	deps := Deps{
		WorktreeFS: func(string) fs.FS { return fsys },
	}

	st, env, reason := decideStatus(deps, "/wt", executor.Outcome{ExitCode: 0})
	if env == nil {
		t.Fatal("decideStatus returned nil envelope")
	}
	if st != lane.Blocked {
		t.Errorf("decideStatus status = %v, want %v", st, lane.Blocked)
	}
	wantReason := "hard stop fired: do not delete db"
	if reason != wantReason {
		t.Errorf("decideStatus reason = %q, want %q (hard stop takes precedence)", reason, wantReason)
	}
}
