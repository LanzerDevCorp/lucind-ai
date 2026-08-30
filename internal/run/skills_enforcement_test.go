package run_test

import (
	"context"
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/run"
)

func TestPacketDigestExcludesResolvedPaths(t *testing.T) {
	p1 := testPacket()
	p1.LaneRole = "apply"
	p1.RequiredSkills = []string{"lucind-apply", "lucind-executor"}
	p1.AdhocSkills = []string{"custom-adhoc"}
	p1.Body = "# Goal\ndo work\n\n## Hard stops\n- stop\n\n## Required skills\n- /home/user/.gemini/skills/lucind-apply/SKILL.md\n- /home/user/.gemini/skills/lucind-executor/SKILL.md\n\n## Return\ncontract\n"

	p2 := testPacket()
	p2.LaneRole = "apply"
	p2.RequiredSkills = []string{"lucind-apply", "lucind-executor"}
	p2.AdhocSkills = []string{"custom-adhoc"}
	p2.Body = "# Goal\ndo work\n\n## Hard stops\n- stop\n\n## Required skills\n- /opt/custom/roots/lucind-apply/SKILL.md\n- /opt/custom/roots/lucind-executor/SKILL.md\n\n## Return\ncontract\n"

	d1 := run.PacketDigest(p1, []string{"internal/run"})
	d2 := run.PacketDigest(p2, []string{"internal/run"})
	if d1 != d2 {
		t.Fatalf("packetDigest differed across root prefixes: %q != %q", d1, d2)
	}

	// Changing LaneRole changes digest
	pRole := p1
	pRole.LaneRole = "verify"
	if dRole := run.PacketDigest(pRole, []string{"internal/run"}); dRole == d1 {
		t.Fatalf("packetDigest failed to reflect changed LaneRole: %q == %q", dRole, d1)
	}

	// Changing RequiredSkills changes digest
	pReq := p1
	pReq.RequiredSkills = []string{"lucind-executor", "lucind-verify"}
	if dReq := run.PacketDigest(pReq, []string{"internal/run"}); dReq == d1 {
		t.Fatalf("packetDigest failed to reflect changed RequiredSkills: %q == %q", dReq, d1)
	}

	// Changing AdhocSkills changes digest
	pAdhoc := p1
	pAdhoc.AdhocSkills = []string{"other-adhoc"}
	if dAdhoc := run.PacketDigest(pAdhoc, []string{"internal/run"}); dAdhoc == d1 {
		t.Fatalf("packetDigest failed to reflect changed AdhocSkills: %q == %q", dAdhoc, d1)
	}
}

func TestEnforceRequiredSkills(t *testing.T) {
	tests := []struct {
		name           string
		requiredSkills []string
		envelopeJSON   string
		wantStatus     lane.Status
	}{
		{
			name:           "matching skills",
			requiredSkills: []string{"lucind-executor", "lucind-apply"},
			envelopeJSON: `{
				"packet_id": "lane-a",
				"status": "done",
				"summary": "did the thing",
				"hard_stops": [{"hard_stop": "stop", "fired": false}],
				"skills_loaded": ["lucind-executor", "lucind-apply"]
			}`,
			wantStatus: lane.Done,
		},
		{
			name:           "matching skills with extra tolerated",
			requiredSkills: []string{"lucind-executor", "lucind-apply"},
			envelopeJSON: `{
				"packet_id": "lane-a",
				"status": "done",
				"summary": "did the thing",
				"hard_stops": [{"hard_stop": "stop", "fired": false}],
				"skills_loaded": ["lucind-executor", "lucind-apply", "sdd-apply"]
			}`,
			wantStatus: lane.Done,
		},
		{
			name:           "shortfall demotes to deviated",
			requiredSkills: []string{"lucind-executor", "lucind-apply"},
			envelopeJSON: `{
				"packet_id": "lane-a",
				"status": "done",
				"summary": "did the thing",
				"hard_stops": [{"hard_stop": "stop", "fired": false}],
				"skills_loaded": ["lucind-executor"]
			}`,
			wantStatus: lane.Deviated,
		},
		{
			name:           "omitted skills_loaded demotes to deviated when required",
			requiredSkills: []string{"lucind-executor", "lucind-apply"},
			envelopeJSON: `{
				"packet_id": "lane-a",
				"status": "done",
				"summary": "did the thing",
				"hard_stops": [{"hard_stop": "stop", "fired": false}]
			}`,
			wantStatus: lane.Deviated,
		},
		{
			name:           "empty required skills remains done without skills_loaded",
			requiredSkills: nil,
			envelopeJSON: `{
				"packet_id": "lane-a",
				"status": "done",
				"summary": "did the thing",
				"hard_stops": [{"hard_stop": "stop", "fired": false}]
			}`,
			wantStatus: lane.Done,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wtPath := t.TempDir()
			exec := &fakeExecutor{}
			deps := newTestDeps(t, wtPath, func(string) fs.FS {
				return fstest.MapFS{resultEnvelopePathForTest(): {Data: []byte(tt.envelopeJSON)}}
			}, exec)

			p := testPacket()
			p.AllowedPaths = nil
			p.RequiredSkills = tt.requiredSkills

			report, err := run.Execute(context.Background(), deps, p)
			if err != nil {
				t.Fatalf("Execute() error = %v, want nil", err)
			}
			if report.Status != tt.wantStatus {
				t.Fatalf("report.Status = %v, want %v (diagnosis: %q)", report.Status, tt.wantStatus, report.Diagnosis)
			}

			lanes, err := deps.Ledger.Lanes(context.Background(), deps.RunID)
			if err != nil {
				t.Fatalf("Lanes() error = %v", err)
			}
			if len(lanes) != 1 || lanes[0].Status != tt.wantStatus {
				t.Fatalf("persisted status = %v, want %v", lanes[0].Status, tt.wantStatus)
			}
		})
	}
}

func TestExecutePassesRequiredSkillsToExecutorRequest(t *testing.T) {
	tests := []struct {
		name       string
		skills     []string
		wantSkills []string
	}{
		{
			name:       "non-empty required skills",
			skills:     []string{"lucind-apply", "lucind-executor"},
			wantSkills: []string{"lucind-apply", "lucind-executor"},
		},
		{
			name:       "nil required skills",
			skills:     nil,
			wantSkills: nil,
		},
		{
			name:       "empty required skills slice",
			skills:     []string{},
			wantSkills: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wtPath := t.TempDir()
			exec := &fakeExecutor{}
			deps := newTestDeps(t, wtPath, func(string) fs.FS {
				return fstest.MapFS{resultEnvelopePathForTest(): {Data: []byte(doneEnvelopeJSON)}}
			}, exec)

			p := testPacket()
			p.RequiredSkills = tt.skills

			if _, err := run.Execute(context.Background(), deps, p); err != nil {
				t.Fatalf("Execute() error = %v, want nil", err)
			}

			if !reflect.DeepEqual(exec.gotReq.RequiredSkills, tt.wantSkills) {
				t.Fatalf("Request.RequiredSkills = %v, want %v", exec.gotReq.RequiredSkills, tt.wantSkills)
			}

			if len(tt.skills) > 0 {
				// Verify defensive copy / slice isolation.
				p.RequiredSkills[0] = "mutated"
				if exec.gotReq.RequiredSkills[0] == "mutated" {
					t.Fatalf("Request.RequiredSkills was not copied defensively: saw mutation %v", exec.gotReq.RequiredSkills)
				}
			}
		})
	}
}
