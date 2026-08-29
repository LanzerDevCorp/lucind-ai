package skillset_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/skillset"
)

func TestDefaultSkillBudget(t *testing.T) {
	if skillset.DefaultSkillBudget != 3 {
		t.Fatalf("DefaultSkillBudget = %d, want 3", skillset.DefaultSkillBudget)
	}
}

func TestDerive(t *testing.T) {
	tests := []struct {
		name        string
		sddPhase    string
		laneRole    string
		stackSkills []string
		adhocSkills []string
		wantSkills  []string
		wantErr     error
	}{
		{
			name:        "planning lens propose",
			sddPhase:    "propose",
			laneRole:    "lens",
			stackSkills: nil,
			adhocSkills: nil,
			wantSkills:  []string{"lucind-executor", "lucind-fan-out-lens", "sdd-propose"},
		},
		{
			name:        "planning lens explore",
			sddPhase:    "explore",
			laneRole:    "lens",
			stackSkills: nil,
			adhocSkills: nil,
			wantSkills:  []string{"lucind-executor", "lucind-fan-out-lens", "sdd-explore"},
		},
		{
			name:        "planning lens spec",
			sddPhase:    "spec",
			laneRole:    "lens",
			stackSkills: nil,
			adhocSkills: nil,
			wantSkills:  []string{"lucind-executor", "lucind-fan-out-lens", "sdd-spec"},
		},
		{
			name:        "planning lens design",
			sddPhase:    "design",
			laneRole:    "lens",
			stackSkills: nil,
			adhocSkills: nil,
			wantSkills:  []string{"lucind-executor", "lucind-fan-out-lens", "sdd-design"},
		},
		{
			name:        "planning lens tasks",
			sddPhase:    "tasks",
			laneRole:    "lens",
			stackSkills: nil,
			adhocSkills: nil,
			wantSkills:  []string{"lucind-executor", "lucind-fan-out-lens", "sdd-tasks"},
		},
		{
			name:        "planning synthesis propose",
			sddPhase:    "propose",
			laneRole:    "synthesis",
			stackSkills: nil,
			adhocSkills: nil,
			wantSkills:  []string{"lucind-executor", "lucind-fan-out-lens", "sdd-propose"},
		},
		{
			name:        "apply lane",
			sddPhase:    "apply",
			laneRole:    "apply",
			stackSkills: nil,
			adhocSkills: nil,
			wantSkills:  []string{"lucind-apply", "lucind-executor", "sdd-apply"},
		},
		{
			name:        "apply lane with empty sddPhase",
			sddPhase:    "",
			laneRole:    "apply",
			stackSkills: nil,
			adhocSkills: nil,
			wantSkills:  []string{"lucind-apply", "lucind-executor", "sdd-apply"},
		},
		{
			name:        "verify lane",
			sddPhase:    "verify",
			laneRole:    "verify",
			stackSkills: nil,
			adhocSkills: nil,
			wantSkills:  []string{"lucind-executor", "lucind-verify", "sdd-verify"},
		},
		{
			name:        "verify lane with empty sddPhase",
			sddPhase:    "",
			laneRole:    "verify",
			stackSkills: nil,
			adhocSkills: nil,
			wantSkills:  []string{"lucind-executor", "lucind-verify", "sdd-verify"},
		},
		{
			name:        "archive lane",
			sddPhase:    "archive",
			laneRole:    "archive",
			stackSkills: nil,
			adhocSkills: nil,
			wantSkills:  []string{"lucind-executor", "sdd-archive"},
		},
		{
			name:        "archive lane with empty sddPhase",
			sddPhase:    "",
			laneRole:    "archive",
			stackSkills: nil,
			adhocSkills: nil,
			wantSkills:  []string{"lucind-executor", "sdd-archive"},
		},
		{
			name:        "ultrafixer role",
			sddPhase:    "",
			laneRole:    "ultrafixer",
			stackSkills: nil,
			adhocSkills: nil,
			wantSkills:  []string{"lucind-executor"},
		},
		{
			name:        "human role",
			sddPhase:    "",
			laneRole:    "human",
			stackSkills: nil,
			adhocSkills: nil,
			wantSkills:  []string{"lucind-executor"},
		},
		{
			name:        "remediate phase does not derive phase skill",
			sddPhase:    "remediate",
			laneRole:    "lens",
			stackSkills: nil,
			adhocSkills: nil,
			wantSkills:  []string{"lucind-executor", "lucind-fan-out-lens"},
		},
		{
			name:        "legacy omitted role and empty phase",
			sddPhase:    "",
			laneRole:    "",
			stackSkills: nil,
			adhocSkills: nil,
			wantSkills:  []string{"lucind-executor"},
		},
		{
			name:        "legacy omitted role with sdd phase",
			sddPhase:    "propose",
			laneRole:    "",
			stackSkills: nil,
			adhocSkills: nil,
			wantSkills:  []string{"lucind-executor", "sdd-propose"},
		},
		{
			name:        "stack deduplication with derived skill",
			sddPhase:    "propose",
			laneRole:    "lens",
			stackSkills: []string{"lucind-executor"},
			adhocSkills: nil,
			wantSkills:  []string{"lucind-executor", "lucind-fan-out-lens", "sdd-propose"},
		},
		{
			name:        "union and sorting across derived, stack, and adhoc",
			sddPhase:    "apply",
			laneRole:    "apply",
			stackSkills: []string{"go-testing", "lucind-executor"},
			adhocSkills: []string{"custom-lint", "go-testing"},
			wantSkills:  []string{"custom-lint", "go-testing", "lucind-apply", "lucind-executor", "sdd-apply"},
		},
		{
			name:        "whitespace and empty items in stack and adhoc are ignored",
			sddPhase:    "apply",
			laneRole:    "apply",
			stackSkills: []string{"", "  ", "go-testing "},
			adhocSkills: []string{" \t ", "custom-lint"},
			wantSkills:  []string{"custom-lint", "go-testing", "lucind-apply", "lucind-executor", "sdd-apply"},
		},
		{
			name:        "invalid lane role returns error",
			sddPhase:    "propose",
			laneRole:    "invalid-role",
			stackSkills: nil,
			adhocSkills: nil,
			wantErr:     skillset.ErrInvalidLaneRole,
		},
		{
			name:        "invalid sdd phase returns error",
			sddPhase:    "invalid-phase",
			laneRole:    "lens",
			stackSkills: nil,
			adhocSkills: nil,
			wantErr:     skillset.ErrInvalidSDDPhase,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := skillset.Derive(tt.sddPhase, tt.laneRole, tt.stackSkills, tt.adhocSkills)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Derive() error = nil, wantErr %v", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Derive() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Derive() unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.wantSkills) {
				t.Errorf("Derive()\ngot:  %#v\nwant: %#v", got, tt.wantSkills)
			}
		})
	}
}

func TestDeriveDeterminismAndImmutability(t *testing.T) {
	stack := []string{"stack-b", "stack-a"}
	adhoc := []string{"adhoc-2", "adhoc-1"}

	res1, err := skillset.Derive("propose", "lens", stack, adhoc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify original input slices were not mutated in place
	if stack[0] != "stack-b" || stack[1] != "stack-a" {
		t.Errorf("stackSkills was mutated: %#v", stack)
	}
	if adhoc[0] != "adhoc-2" || adhoc[1] != "adhoc-1" {
		t.Errorf("adhocSkills was mutated: %#v", adhoc)
	}

	res2, err := skillset.Derive("propose", "lens", stack, adhoc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(res1, res2) {
		t.Errorf("Derive is non-deterministic:\nres1: %#v\nres2: %#v", res1, res2)
	}
}

func TestValidationHelpers(t *testing.T) {
	validRoles := []string{"lens", "synthesis", "apply", "verify", "archive", "ultrafixer", "human"}
	for _, r := range validRoles {
		if !skillset.IsValidLaneRole(r) {
			t.Errorf("IsValidLaneRole(%q) = false, want true", r)
		}
	}
	if skillset.IsValidLaneRole("unknown") {
		t.Errorf("IsValidLaneRole(\"unknown\") = true, want false")
	}

	validPhases := []string{"explore", "propose", "spec", "design", "tasks", "apply", "verify", "remediate", "archive"}
	for _, p := range validPhases {
		if !skillset.IsValidSDDPhase(p) {
			t.Errorf("IsValidSDDPhase(%q) = false, want true", p)
		}
	}
	if skillset.IsValidSDDPhase("unknown") {
		t.Errorf("IsValidSDDPhase(\"unknown\") = true, want false")
	}
}

func TestDigestBodyElidesRequiredSkills(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "body without required skills section is unchanged",
			body: "# Goal\nDo work\n\n## Hard stops\n- stop 1\n\n## Return\n```lucind-result-contract\nversion: 1\n```\n",
			want: "# Goal\nDo work\n\n## Hard stops\n- stop 1\n\n## Return\n```lucind-result-contract\nversion: 1\n```\n",
		},
		{
			name: "body with required skills between hard stops and return",
			body: "# Goal\nDo work\n\n## Hard stops\n- stop 1\n\n## Required skills\n- /home/user/.claude/skills/lucind-executor/SKILL.md\n- /home/user/.claude/skills/sdd-apply/SKILL.md\n\n## Return\n```lucind-result-contract\nversion: 1\n```\n",
			want: "# Goal\nDo work\n\n## Hard stops\n- stop 1\n\n## Return\n```lucind-result-contract\nversion: 1\n```\n",
		},
		{
			name: "body with required skills at EOF",
			body: "# Goal\nDo work\n\n## Required skills\n- /path/to/skill/SKILL.md\n",
			want: "# Goal\nDo work\n",
		},
		{
			name: "body with CRLF line endings",
			body: "# Goal\r\nDo work\r\n\r\n## Required skills\r\n- /path/to/skill/SKILL.md\r\n\r\n## Return\r\nresult\r\n",
			want: "# Goal\r\nDo work\r\n\r\n## Return\r\nresult\r\n",
		},
		{
			name: "body with arbitrary following heading",
			body: "# Goal\nDo work\n\n## Required skills\n- /path/to/skill/SKILL.md\n\n## Notes\nSome notes here\n",
			want: "# Goal\nDo work\n\n## Notes\nSome notes here\n",
		},
		{
			name: "empty body returns empty",
			body: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := skillset.DigestBody(tt.body)
			if got != tt.want {
				t.Errorf("DigestBody()\ngot:\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}
