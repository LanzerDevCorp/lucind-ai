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
