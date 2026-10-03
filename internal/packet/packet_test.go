package packet_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
)

func TestParseSeparatesFrontmatterFromBody(t *testing.T) {
	src := "---\n" +
		"id: fix-auth\n" +
		"executor: agy\n" +
		"routed_by: touches auth, Tier A verification required\n" +
		"---\n" +
		"\n" +
		"## Goal\n" +
		"\n" +
		"Session cookies must survive a restart.\n"

	p, err := packet.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}

	if p.ID != "fix-auth" {
		t.Errorf("ID = %q, want %q", p.ID, "fix-auth")
	}
	if p.Executor != "agy" {
		t.Errorf("Executor = %q, want %q", p.Executor, "agy")
	}
	if p.RoutedBy != "touches auth, Tier A verification required" {
		t.Errorf("RoutedBy = %q, want %q", p.RoutedBy, "touches auth, Tier A verification required")
	}

	wantBody := "## Goal\n\nSession cookies must survive a restart.\n"
	if p.Body != wantBody {
		t.Errorf("Body = %q, want %q", p.Body, wantBody)
	}
	if len(p.AllowedPaths) != 0 {
		t.Errorf("AllowedPaths = %v, want empty", p.AllowedPaths)
	}
}

func TestParseModelPresentIsParsed(t *testing.T) {
	src := "---\n" +
		"id: fix-auth\n" +
		"executor: agy\n" +
		"routed_by: touches auth, Tier A verification required\n" +
		"model: gemini-3.7-flash-high\n" +
		"---\n" +
		"\n" +
		"## Goal\n"

	p, err := packet.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
	if p.Model != "gemini-3.7-flash-high" {
		t.Errorf("Model = %q, want %q", p.Model, "gemini-3.7-flash-high")
	}
}

// TestParseModelAbsentLeavesFieldEmpty documents the chosen split of
// responsibility: packet.Parse reflects frontmatter literally and applies
// no runtime policy, so an absent model key leaves Packet.Model at its
// zero value. Filling in the project default is internal/run's job, done
// once at the composition root — see run.DefaultModel.
func TestParseModelAbsentLeavesFieldEmpty(t *testing.T) {
	src := "---\n" +
		"id: fix-auth\n" +
		"executor: agy\n" +
		"routed_by: touches auth, Tier A verification required\n" +
		"---\n" +
		"\n" +
		"## Goal\n"

	p, err := packet.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
	if p.Model != "" {
		t.Errorf("Model = %q, want empty", p.Model)
	}
}

func TestParseAgentPresentIsParsed(t *testing.T) {
	src := "---\n" +
		"id: author-dag\n" +
		"executor: opencode\n" +
		"routed_by: DAG authoring, specialist agent required\n" +
		"agent: lucind-dag\n" +
		"---\n" +
		"\n" +
		"## Goal\n"

	p, err := packet.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
	if p.Agent != "lucind-dag" {
		t.Errorf("Agent = %q, want %q", p.Agent, "lucind-dag")
	}
}

// TestParseAgentAbsentLeavesFieldEmpty mirrors
// TestParseModelAbsentLeavesFieldEmpty: an absent agent key leaves
// Packet.Agent at its zero value, no default injected.
func TestParseAgentAbsentLeavesFieldEmpty(t *testing.T) {
	src := "---\n" +
		"id: fix-auth\n" +
		"executor: agy\n" +
		"routed_by: touches auth, Tier A verification required\n" +
		"---\n" +
		"\n" +
		"## Goal\n"

	p, err := packet.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
	if p.Agent != "" {
		t.Errorf("Agent = %q, want empty", p.Agent)
	}
}

func TestParseObservabilityFrontmatter(t *testing.T) {
	tests := []struct {
		name            string
		extra           string
		wantSDDPhase    string
		wantFanoutGroup string
		wantSkill       string
		wantLaneRole    string
		wantAdhocSkills []string
	}{
		{
			name: "all observability keys present with lane role and adhoc skills",
			extra: "lane_role: apply\n" +
				"sdd_phase: apply\n" +
				"fanout_group: ledger\n" +
				"skill: lucind-apply\n" +
				"adhoc_skills: [\"custom-1\", \"custom-2\"]\n",
			wantSDDPhase:    "apply",
			wantFanoutGroup: "ledger",
			wantSkill:       "lucind-apply",
			wantLaneRole:    "apply",
			wantAdhocSkills: []string{"custom-1", "custom-2"},
		},
		{
			name:            "omitted keys default to empty",
			extra:           "",
			wantSDDPhase:    "",
			wantFanoutGroup: "",
			wantSkill:       "",
			wantLaneRole:    "",
			wantAdhocSkills: nil,
		},
		{
			name: "explicit empty keys are empty strings",
			extra: "sdd_phase:\n" +
				"fanout_group:   \n" +
				"skill:\n",
			wantSDDPhase:    "",
			wantFanoutGroup: "",
			wantSkill:       "",
			wantLaneRole:    "",
			wantAdhocSkills: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := "---\n" +
				"id: fix-auth\n" +
				"executor: agy\n" +
				"routed_by: touches auth, Tier A verification required\n" +
				tt.extra +
				"---\n\n" +
				"## Goal\n"
			p, err := packet.Parse(strings.NewReader(src))
			if err != nil {
				t.Fatalf("Parse() error = %v, want nil", err)
			}
			if p.SDDPhase != tt.wantSDDPhase {
				t.Errorf("SDDPhase = %q, want %q", p.SDDPhase, tt.wantSDDPhase)
			}
			if p.FanoutGroup != tt.wantFanoutGroup {
				t.Errorf("FanoutGroup = %q, want %q", p.FanoutGroup, tt.wantFanoutGroup)
			}
			if p.Skill != tt.wantSkill {
				t.Errorf("Skill = %q, want %q", p.Skill, tt.wantSkill)
			}
			if p.LaneRole != tt.wantLaneRole {
				t.Errorf("LaneRole = %q, want %q", p.LaneRole, tt.wantLaneRole)
			}
			if !slices.Equal(p.AdhocSkills, tt.wantAdhocSkills) && !(len(p.AdhocSkills) == 0 && len(tt.wantAdhocSkills) == 0) {
				t.Errorf("AdhocSkills = %v, want %v", p.AdhocSkills, tt.wantAdhocSkills)
			}
			if p.Path != "" {
				t.Errorf("Path = %q, want empty (Parse does not invent a filesystem path)", p.Path)
			}
		})
	}
}

func TestParseLaneRoleAndPhaseValidation(t *testing.T) {
	validRoles := []string{"lens", "synthesis", "apply", "verify", "archive", "ultrafixer", "human"}
	validPhases := []string{"explore", "propose", "spec", "design", "tasks", "apply", "verify", "remediate", "archive"}

	for _, role := range validRoles {
		t.Run("valid role "+role, func(t *testing.T) {
			src := "---\n" +
				"id: test-role\n" +
				"executor: agy\n" +
				"routed_by: test\n" +
				"lane_role: " + role + "\n" +
				"---\n\n## Goal\nTest\n"
			p, err := packet.Parse(strings.NewReader(src))
			if err != nil {
				t.Fatalf("Parse() error = %v, want nil", err)
			}
			if p.LaneRole != role {
				t.Errorf("LaneRole = %q, want %q", p.LaneRole, role)
			}
		})
	}

	for _, phase := range validPhases {
		t.Run("valid role with valid phase "+phase, func(t *testing.T) {
			src := "---\n" +
				"id: test-role-phase\n" +
				"executor: agy\n" +
				"routed_by: test\n" +
				"lane_role: lens\n" +
				"sdd_phase: " + phase + "\n" +
				"---\n\n## Goal\nTest\n"
			p, err := packet.Parse(strings.NewReader(src))
			if err != nil {
				t.Fatalf("Parse() error = %v, want nil", err)
			}
			if p.LaneRole != "lens" || p.SDDPhase != phase {
				t.Errorf("LaneRole=%q SDDPhase=%q, want lens/%s", p.LaneRole, p.SDDPhase, phase)
			}
		})
	}

	t.Run("invalid lane_role rejected", func(t *testing.T) {
		src := "---\n" +
			"id: test-role\n" +
			"executor: agy\n" +
			"routed_by: test\n" +
			"lane_role: invalid_role\n" +
			"---\n\n## Goal\nTest\n"
		_, err := packet.Parse(strings.NewReader(src))
		if !errors.Is(err, packet.ErrInvalidLaneRole) {
			t.Fatalf("Parse() error = %v, want %v", err, packet.ErrInvalidLaneRole)
		}
	})

	t.Run("invalid sdd_phase rejected when lane_role present", func(t *testing.T) {
		src := "---\n" +
			"id: test-role\n" +
			"executor: agy\n" +
			"routed_by: test\n" +
			"lane_role: apply\n" +
			"sdd_phase: bogus_phase\n" +
			"---\n\n## Goal\nTest\n"
		_, err := packet.Parse(strings.NewReader(src))
		if !errors.Is(err, packet.ErrInvalidSDDPhase) {
			t.Fatalf("Parse() error = %v, want %v", err, packet.ErrInvalidSDDPhase)
		}
	})

	t.Run("legacy omission: arbitrary sdd_phase permitted when lane_role omitted", func(t *testing.T) {
		src := "---\n" +
			"id: test-legacy\n" +
			"executor: agy\n" +
			"routed_by: test\n" +
			"sdd_phase: unvalidated_custom_phase\n" +
			"---\n\n## Goal\nTest\n"
		p, err := packet.Parse(strings.NewReader(src))
		if err != nil {
			t.Fatalf("Parse() error = %v, want nil", err)
		}
		if p.LaneRole != "" {
			t.Errorf("LaneRole = %q, want empty", p.LaneRole)
		}
		if p.SDDPhase != "unvalidated_custom_phase" {
			t.Errorf("SDDPhase = %q, want unvalidated_custom_phase", p.SDDPhase)
		}
	})
}

func TestParseAdhocSkills(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		wantSkills []string
		wantErr    error
	}{
		{
			name: "valid adhoc skills array",
			src: "---\n" +
				"id: test-adhoc\n" +
				"executor: agy\n" +
				"routed_by: test\n" +
				"adhoc_skills: [\"skill-a\", \"skill-b\"]\n" +
				"---\n\n## Goal\nTest\n",
			wantSkills: []string{"skill-a", "skill-b"},
		},
		{
			name: "valid empty array",
			src: "---\n" +
				"id: test-adhoc\n" +
				"executor: agy\n" +
				"routed_by: test\n" +
				"adhoc_skills: []\n" +
				"---\n\n## Goal\nTest\n",
			wantSkills: []string{},
		},
		{
			name: "invalid non-array string",
			src: "---\n" +
				"id: test-adhoc\n" +
				"executor: agy\n" +
				"routed_by: test\n" +
				"adhoc_skills: bare-string\n" +
				"---\n\n## Goal\nTest\n",
			wantErr: packet.ErrInvalidAdhocSkills,
		},
		{
			name: "invalid number array",
			src: "---\n" +
				"id: test-adhoc\n" +
				"executor: agy\n" +
				"routed_by: test\n" +
				"adhoc_skills: [1, 2, 3]\n" +
				"---\n\n## Goal\nTest\n",
			wantErr: packet.ErrInvalidAdhocSkills,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := packet.Parse(strings.NewReader(tt.src))
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Parse() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() error = %v, want nil", err)
			}
			if !slices.Equal(p.AdhocSkills, tt.wantSkills) && !(len(p.AdhocSkills) == 0 && len(tt.wantSkills) == 0) {
				t.Errorf("AdhocSkills = %v, want %v", p.AdhocSkills, tt.wantSkills)
			}
		})
	}
}

func TestParseIgnoresRequiredSkillsFrontmatter(t *testing.T) {
	src := "---\n" +
		"id: test-ignore-required\n" +
		"executor: agy\n" +
		"routed_by: test\n" +
		"required_skills: [\"malicious-skill\", \"authored-path\"]\n" +
		"---\n\n## Goal\nTest\n"
	p, err := packet.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
	if len(p.RequiredSkills) != 0 {
		t.Errorf("RequiredSkills = %v, want empty/nil (required_skills must never be parsed from frontmatter)", p.RequiredSkills)
	}
}

func TestParseReadOnlyFrontmatter(t *testing.T) {
	tests := []struct {
		name         string
		src          string
		wantReadOnly bool
	}{
		{
			name: "explicit read_only true",
			src: "---\n" +
				"id: fix-auth\n" +
				"executor: agy\n" +
				"routed_by: touches auth, Tier A verification required\n" +
				"read_only: true\n" +
				"---\n\n" +
				"## Goal\n",
			wantReadOnly: true,
		},
		{
			name: "explicit read_only false",
			src: "---\n" +
				"id: fix-auth\n" +
				"executor: agy\n" +
				"routed_by: touches auth, Tier A verification required\n" +
				"read_only: false\n" +
				"---\n\n" +
				"## Goal\n",
			wantReadOnly: false,
		},
		{
			name: "omitted key defaults to false",
			src: "---\n" +
				"id: fix-auth\n" +
				"executor: agy\n" +
				"routed_by: touches auth, Tier A verification required\n" +
				"---\n\n" +
				"## Goal\n",
			wantReadOnly: false,
		},
		{
			name: "id explore-foo with omitted key does not infer read_only",
			src: "---\n" +
				"id: explore-foo\n" +
				"executor: agy\n" +
				"routed_by: touches auth, Tier A verification required\n" +
				"---\n\n" +
				"## Goal\n",
			wantReadOnly: false,
		},
		{
			name: "unrecognized sibling keys ignored and read_only stays false",
			src: "---\n" +
				"id: fix-auth\n" +
				"executor: agy\n" +
				"routed_by: touches auth, Tier A verification required\n" +
				"unknown_key: something\n" +
				"another_key: 123\n" +
				"---\n\n" +
				"## Goal\n",
			wantReadOnly: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := packet.Parse(strings.NewReader(tt.src))
			if err != nil {
				t.Fatalf("Parse() error = %v, want nil", err)
			}
			if p.ReadOnly != tt.wantReadOnly {
				t.Errorf("ReadOnly = %v, want %v", p.ReadOnly, tt.wantReadOnly)
			}
		})
	}
}

func TestParseAllowedPathsFrontmatter(t *testing.T) {
	tests := []struct {
		name             string
		src              string
		wantAllowedPaths []string
		wantErr          error
	}{
		{
			name: "valid JSON array with multiple paths",
			src: "---\n" +
				"id: fix-auth\n" +
				"executor: agy\n" +
				"routed_by: touches auth, Tier A verification required\n" +
				"allowed_paths: [\"internal/ledger/\", \"cmd/lucind-ai/cli.go\"]\n" +
				"---\n\n" +
				"## Goal\n",
			wantAllowedPaths: []string{"internal/ledger/", "cmd/lucind-ai/cli.go"},
		},
		{
			name: "valid empty JSON array",
			src: "---\n" +
				"id: fix-auth\n" +
				"executor: agy\n" +
				"routed_by: touches auth, Tier A verification required\n" +
				"allowed_paths: []\n" +
				"---\n\n" +
				"## Goal\n",
			wantAllowedPaths: []string{},
		},
		{
			name: "omitted key leaves AllowedPaths empty",
			src: "---\n" +
				"id: fix-auth\n" +
				"executor: agy\n" +
				"routed_by: touches auth, Tier A verification required\n" +
				"---\n\n" +
				"## Goal\n",
			wantAllowedPaths: nil,
		},
		{
			name: "bare YAML list is rejected as invalid JSON",
			src: "---\n" +
				"id: fix-auth\n" +
				"executor: agy\n" +
				"routed_by: touches auth, Tier A verification required\n" +
				"allowed_paths:\n" +
				"  - internal/ledger/\n" +
				"---\n\n" +
				"## Goal\n",
			wantErr: packet.ErrInvalidAllowedPaths,
		},
		{
			name: "opening brace only is rejected as invalid JSON",
			src: "---\n" +
				"id: fix-auth\n" +
				"executor: agy\n" +
				"routed_by: touches auth, Tier A verification required\n" +
				"allowed_paths: {\n" +
				"---\n\n" +
				"## Goal\n",
			wantErr: packet.ErrInvalidAllowedPaths,
		},
		{
			name: "bare string is rejected as invalid JSON",
			src: "---\n" +
				"id: fix-auth\n" +
				"executor: agy\n" +
				"routed_by: touches auth, Tier A verification required\n" +
				"allowed_paths: internal/ledger/\n" +
				"---\n\n" +
				"## Goal\n",
			wantErr: packet.ErrInvalidAllowedPaths,
		},
		{
			name: "JSON object is rejected as invalid type",
			src: "---\n" +
				"id: fix-auth\n" +
				"executor: agy\n" +
				"routed_by: touches auth, Tier A verification required\n" +
				"allowed_paths: {\"path\": \"internal/ledger/\"}\n" +
				"---\n\n" +
				"## Goal\n",
			wantErr: packet.ErrInvalidAllowedPaths,
		},
		{
			name: "JSON number array is rejected as invalid type",
			src: "---\n" +
				"id: fix-auth\n" +
				"executor: agy\n" +
				"routed_by: touches auth, Tier A verification required\n" +
				"allowed_paths: [123, 456]\n" +
				"---\n\n" +
				"## Goal\n",
			wantErr: packet.ErrInvalidAllowedPaths,
		},
		{
			name: "empty value is rejected as invalid JSON",
			src: "---\n" +
				"id: fix-auth\n" +
				"executor: agy\n" +
				"routed_by: touches auth, Tier A verification required\n" +
				"allowed_paths:\n" +
				"---\n\n" +
				"## Goal\n",
			wantErr: packet.ErrInvalidAllowedPaths,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := packet.Parse(strings.NewReader(tt.src))
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Parse() error = nil, want %v", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Parse() error = %v, want %v", err, tt.wantErr)
				}
				if p.ID != "" || p.Executor != "" || len(p.AllowedPaths) != 0 {
					t.Errorf("Parse() returned non-empty packet %v on error", p)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() error = %v, want nil", err)
			}
			if !slices.Equal(p.AllowedPaths, tt.wantAllowedPaths) && !(len(p.AllowedPaths) == 0 && len(tt.wantAllowedPaths) == 0) {
				t.Errorf("AllowedPaths = %v, want %v", p.AllowedPaths, tt.wantAllowedPaths)
			}
		})
	}
}

func TestParseReadOnlyPathsFrontmatter(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "preserves declared input paths",
			src: "---\n" +
				"id: inspect-auth\n" +
				"executor: agy\n" +
				"routed_by: inspect auth\n" +
				"read_only_paths: [\"docs/spec.md\", \"internal/auth/config.go\"]\n" +
				"---\n\n## Goal\nInspect auth.\n",
			want: []string{"docs/spec.md", "internal/auth/config.go"},
		},
		{
			name: "omitted declaration remains empty",
			src: "---\n" +
				"id: inspect-auth\n" +
				"executor: agy\n" +
				"routed_by: inspect auth\n" +
				"---\n\n## Goal\nInspect auth.\n",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := packet.Parse(strings.NewReader(tt.src))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if !slices.Equal(got.ReadOnlyPaths, tt.want) {
				t.Fatalf("ReadOnlyPaths = %v, want %v", got.ReadOnlyPaths, tt.want)
			}
		})
	}
}

func TestParseRejectsNonArrayReadOnlyPaths(t *testing.T) {
	src := "---\nid: inspect-auth\nexecutor: agy\nrouted_by: inspect auth\nread_only_paths: null\n---\nbody\n"
	_, err := packet.Parse(strings.NewReader(src))
	if !errors.Is(err, packet.ErrInvalidReadOnlyPaths) {
		t.Fatalf("Parse() error = %v, want %v", err, packet.ErrInvalidReadOnlyPaths)
	}
}

func TestParseRejectsIncompletePackets(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want error
	}{
		{
			name: "no frontmatter block at all",
			src:  "## Goal\n\nDo the thing.\n",
			want: packet.ErrNoFrontmatter,
		},
		{
			name: "frontmatter is never closed",
			src:  "---\nid: fix-auth\nexecutor: agy\n\n## Goal\n",
			want: packet.ErrNoFrontmatter,
		},
		{
			name: "id is missing",
			src:  "---\nexecutor: agy\n---\n\n## Goal\n",
			want: packet.ErrMissingID,
		},
		{
			name: "id is present but blank",
			src:  "---\nid:\nexecutor: agy\n---\n\n## Goal\n",
			want: packet.ErrMissingID,
		},
		{
			name: "executor is missing",
			src:  "---\nid: fix-auth\n---\n\n## Goal\n",
			want: packet.ErrMissingExecutor,
		},
		{
			name: "routed_by is missing",
			src:  "---\nid: fix-auth\nexecutor: agy\n---\n\n## Goal\n",
			want: packet.ErrMissingRoutedBy,
		},
		{
			name: "routed_by is present but blank",
			src:  "---\nid: fix-auth\nexecutor: agy\nrouted_by:\n---\n\n## Goal\n",
			want: packet.ErrMissingRoutedBy,
		},
		{
			name: "body is empty",
			src:  "---\nid: fix-auth\nexecutor: agy\nrouted_by: touches auth, Tier A verification required\n---\n\n\n",
			want: packet.ErrEmptyBody,
		},
		{
			name: "read_only is yes",
			src:  "---\nid: fix-auth\nexecutor: agy\nrouted_by: touches auth, Tier A verification required\nread_only: yes\n---\n\n## Goal\n",
			want: packet.ErrInvalidReadOnly,
		},
		{
			name: "read_only is 1",
			src:  "---\nid: fix-auth\nexecutor: agy\nrouted_by: touches auth, Tier A verification required\nread_only: 1\n---\n\n## Goal\n",
			want: packet.ErrInvalidReadOnly,
		},
		{
			name: "read_only is quoted string",
			src:  "---\nid: fix-auth\nexecutor: agy\nrouted_by: touches auth, Tier A verification required\nread_only: \"true\"\n---\n\n## Goal\n",
			want: packet.ErrInvalidReadOnly,
		},
		{
			name: "read_only is empty",
			src:  "---\nid: fix-auth\nexecutor: agy\nrouted_by: touches auth, Tier A verification required\nread_only:\n---\n\n## Goal\n",
			want: packet.ErrInvalidReadOnly,
		},
		{
			name: "id is missing with read_only true",
			src:  "---\nexecutor: agy\nrouted_by: touches auth, Tier A verification required\nread_only: true\n---\n\n## Goal\n",
			want: packet.ErrMissingID,
		},
		{
			name: "executor is missing with read_only true",
			src:  "---\nid: fix-auth\nrouted_by: touches auth, Tier A verification required\nread_only: true\n---\n\n## Goal\n",
			want: packet.ErrMissingExecutor,
		},
		{
			name: "routed_by is missing with read_only true",
			src:  "---\nid: fix-auth\nexecutor: agy\nread_only: true\n---\n\n## Goal\n",
			want: packet.ErrMissingRoutedBy,
		},
		{
			name: "body is empty with read_only true",
			src:  "---\nid: fix-auth\nexecutor: agy\nrouted_by: touches auth, Tier A verification required\nread_only: true\n---\n\n\n",
			want: packet.ErrEmptyBody,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := packet.Parse(strings.NewReader(tt.src))
			if !errors.Is(err, tt.want) {
				t.Errorf("Parse() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestPacketTemplateAssetContract(t *testing.T) {
	templatePath := filepath.Join("..", "..", "plugin", "claude-code", "skills", "lucind-ai", "assets", "packet-template.md")
	data, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", templatePath, err)
	}
	content := string(data)

	// Example frontmatter must omit read_only (skeleton stays write-default).
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		t.Fatalf("packet-template.md missing frontmatter delimiters")
	}
	frontmatter := parts[1]
	if strings.Contains(frontmatter, "read_only") {
		t.Errorf("example frontmatter contains read_only, want skeleton to stay write-default")
	}

	// Criterion 2 still requires a commit for write packets.
	if !strings.Contains(content, "**The work is committed.**") {
		t.Errorf("packet-template.md missing mandatory criterion 2 'The work is committed.'")
	}
	if !strings.Contains(content, "`git status --porcelain` empty") || !strings.Contains(content, "`git log --oneline -1`") {
		t.Errorf("packet-template.md missing write-packet commit evidence requirements")
	}

	// Read-only note must document swapping criterion 2 for unchanged tree evidence.
	if !strings.Contains(content, "Read-only packets") {
		t.Errorf("packet-template.md missing 'Read-only packets' note")
	}
	if !strings.Contains(content, "read_only: true") {
		t.Errorf("packet-template.md note missing 'read_only: true' instruction")
	}
	if !strings.Contains(content, "git merge-base HEAD <primary HEAD>") {
		t.Errorf("packet-template.md note missing merge-base check evidence")
	}
}

func TestSkillAssetContract(t *testing.T) {
	skillDir := filepath.Join("..", "..", "plugin", "claude-code", "skills", "lucind-ai")
	skillPath := filepath.Join(skillDir, "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", skillPath, err)
	}
	router := string(data)
	content := router
	err = filepath.WalkDir(filepath.Join(skillDir, "references"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		content += "\n" + string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("read skill references: %v", err)
	}

	references := []string{
		"references/core/domain.md",
		"references/core/safety.md",
		"references/modes/isolated.md",
		"references/modes/exclusive.md",
		"references/strategies/direct.md",
		"references/strategies/odd.md",
		"references/strategies/fan-out.md",
		"references/coordination/dependencies-defects.md",
		"references/coordination/recovery-reconciliation.md",
		"references/contracts/packets-results.md",
		"references/contracts/acceptance-promotion.md",
		"references/adapters/executors.md",
		"references/adapters/claude-agent-teams.md",
		"references/operations/troubleshooting.md",
	}
	for _, reference := range references {
		if !strings.Contains(router, "`"+reference+"`") {
			t.Errorf("SKILL.md missing pointer to %s", reference)
		}
		if _, statErr := os.Stat(filepath.Join(skillDir, filepath.FromSlash(reference))); statErr != nil {
			t.Errorf("SKILL.md pointer %s does not resolve: %v", reference, statErr)
		}
	}

	// Mandatory criterion 2 states the read-only exception.
	if !strings.Contains(content, "*Mandatory criterion 2*") {
		t.Errorf("SKILL.md missing mandatory criterion 2")
	}
	if !strings.Contains(content, "read_only: true") || !strings.Contains(content, "git merge-base HEAD <primary HEAD>") {
		t.Errorf("SKILL.md mandatory criterion 2 missing read-only exception with merge-base check")
	}

	// ODD strategy is documented.
	if !strings.Contains(content, "odd/tasks/<feature>.md") {
		t.Errorf("skill package missing ODD feature document checklist guidance")
	}

	// The dirty-primary-root hazard remains explicit after modularization.
	if !strings.Contains(content, ".lucind/packets/") || !strings.Contains(content, "primary root") {
		t.Errorf("skill package missing packet location and dirty-primary-root hazard")
	}

	// Frontmatter table documents the five feature-target keys (2.1-RED).
	for _, key := range []string{"`feature`", "`parent_ref`", "`base_sha`", "`expected_parent_sha`", "`legacy_main`"} {
		if !strings.Contains(content, key) {
			t.Errorf("SKILL.md frontmatter table missing key %s", key)
		}
	}

	// Planning fan-out convention (2.2-RED).
	for _, phase := range []string{"explore", "propose", "design", "specs", "tasks"} {
		if !strings.Contains(content, phase) {
			t.Errorf("SKILL.md planning fan-out convention missing phase %q", phase)
		}
	}
	if !strings.Contains(content, "canonical budget stays below the sum of the lens budgets") &&
		!strings.Contains(content, "canonical ceiling stays strictly below the sum") &&
		!strings.Contains(content, "canonical artifact word budget MUST stay strictly below") &&
		!strings.Contains(content, "canonical budget stays strictly below the sum") {
		t.Errorf("SKILL.md missing strict compression ceiling relation")
	}

	// Deterministic orchestrator contract: cross-runtime preflight, late
	// target bind, and wave-N+1-only-after-exit-0.
	if !strings.Contains(content, "byte-identical") {
		t.Errorf("skill package missing Claude/OpenCode skill-tree byte-identity preflight")
	}
	if !strings.Contains(content, "embedded result schema") && !strings.Contains(content, "embedded-schema freshness") {
		t.Errorf("skill package missing embedded-schema freshness preflight")
	}
	if !strings.Contains(content, "reusable packet templates") && !strings.Contains(content, "reusable templates omit") {
		t.Errorf("skill package missing late target bind / target-free template rule")
	}
	if !strings.Contains(content, "wave N+1") || !strings.Contains(content, "exits 0") {
		t.Errorf("skill package missing wave-N+1-only-after-exit-0 rule")
	}

	// Feature-branch ownership (2.3-RED).
	if !strings.Contains(content, "feature create") {
		t.Errorf("SKILL.md missing feature create orchestration guidance")
	}
	if !strings.Contains(content, "Lanes do not create or move parent refs") &&
		!strings.Contains(content, "lanes do not create or move parent refs") &&
		!strings.Contains(content, "Lanes do not create or move parent references") {
		t.Errorf("SKILL.md missing feature-branch lane immutability rule")
	}

	// Shipped subcommands and run flags (2.5-RED). "serve" is deliberately
	// absent: the control room was decommissioned in 751c6b1, and asserting
	// SKILL.md still names it would pin a subcommand the binary no longer
	// has. (It never really asserted anything anyway -- the third Contains
	// clause below matches the substring inside "preserve".)
	for _, cmd := range []string{"feature", "reconcile", "renew"} {
		if !strings.Contains(content, "lucind-ai "+cmd) && !strings.Contains(content, "`"+cmd+"`") && !strings.Contains(content, cmd) {
			t.Errorf("SKILL.md invocation/CLI section missing subcommand %q", cmd)
		}
	}
	for _, flag := range []string{"--legacy-main", "--expected-parent-sha"} {
		if !strings.Contains(content, flag) {
			t.Errorf("SKILL.md invocation/CLI section missing run flag %q", flag)
		}
	}

	contextPath := filepath.Join("..", "..", "CONTEXT.md")
	contextData, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", contextPath, err)
	}
	parts := strings.SplitN(string(contextData), "## Language\n\n", 2)
	if len(parts) != 2 {
		t.Fatalf("CONTEXT.md missing Language glossary")
	}
	domainPath := filepath.Join(skillDir, "references", "core", "domain.md")
	domainData, err := os.ReadFile(domainPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", domainPath, err)
	}
	if !strings.Contains(string(domainData), strings.TrimSpace(parts[1])) {
		t.Errorf("references/core/domain.md canonical projection drifted from CONTEXT.md")
	}
}

func TestSkillTreesByteIdentical(t *testing.T) {
	claude := filepath.Join("..", "..", "plugin", "claude-code", "skills", "lucind-ai")
	opencode := filepath.Join("..", "..", "plugin", "opencode", "skills", "lucind-ai")
	claudeFiles := skillTreeFiles(t, claude)
	opencodeFiles := skillTreeFiles(t, opencode)
	if len(claudeFiles) != len(opencodeFiles) {
		t.Fatalf("skill tree file counts differ: claude=%d opencode=%d", len(claudeFiles), len(opencodeFiles))
	}
	for rel, want := range claudeFiles {
		got, ok := opencodeFiles[rel]
		if !ok {
			t.Errorf("OpenCode tree missing %s (present in Claude tree)", rel)
			continue
		}
		if !bytes.Equal(want, got) {
			t.Errorf("skill file %s differs between Claude and OpenCode trees", rel)
		}
	}
	for rel := range opencodeFiles {
		if _, ok := claudeFiles[rel]; !ok {
			t.Errorf("OpenCode tree has extra file %s (absent from Claude tree)", rel)
		}
	}
}

func skillTreeFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = data
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if len(files) == 0 {
		t.Fatalf("skill tree %s is empty", root)
	}
	return files
}

// pluginManifest mirrors the fields this test reads from
// plugin/claude-code/.claude-plugin/plugin.json.
type pluginManifest struct {
	Version string `json:"version"`
}

// marketplaceManifest mirrors the fields this test reads from
// .claude-plugin/marketplace.json.
type marketplaceManifest struct {
	Plugins []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"plugins"`
}

// TestPluginVersionGuardsSkillContent asserts plugin.json's version and
// marketplace.json's version for the "lucind-ai" plugin stay in lockstep.
// This is a cheap, always-true invariant regardless of what content
// changed, so it stays a blocking go test / lucind-ai check gate.
//
// This test used to ALSO recompute a content hash of the shipped skill tree
// (plugin/claude-code/skills/lucind-ai/**) and require plugin.json's
// version to be bumped, and internal/packet/testdata/skill_content_hash.txt
// regenerated, in the same commit as any change under that tree. That
// forced every isolated feature branch touching the skill tree to bump the
// same shared version field to stay green: with 2+ features doing so
// concurrently, each bump tripped lucind-ai's own overlap-required
// reconciliation gate against every other one, and that gate has no support
// for resolving 3+ simultaneous overlaps in one retry pass -- an unfixable
// deadlock with no valid resolve/promote sequence. A version bump must be a
// deliberate, human-run action, never an automatic side effect of ordinary
// content edits, so that check now lives in `make verify-plugin-content`
// (internal/skillcontent.Verify) instead of here; `make bump-plugin-version`
// is the only place a bump should originate from.
func TestPluginVersionGuardsSkillContent(t *testing.T) {
	repoRoot := filepath.Join("..", "..")

	pluginPath := filepath.Join(repoRoot, "plugin", "claude-code", ".claude-plugin", "plugin.json")
	pluginData, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", pluginPath, err)
	}
	var pm pluginManifest
	if err := json.Unmarshal(pluginData, &pm); err != nil {
		t.Fatalf("parse %s: %v", pluginPath, err)
	}
	if pm.Version == "" {
		t.Fatalf("%s: version is empty", pluginPath)
	}

	marketplacePath := filepath.Join(repoRoot, ".claude-plugin", "marketplace.json")
	marketplaceData, err := os.ReadFile(marketplacePath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", marketplacePath, err)
	}
	var mm marketplaceManifest
	if err := json.Unmarshal(marketplaceData, &mm); err != nil {
		t.Fatalf("parse %s: %v", marketplacePath, err)
	}
	var marketplaceVersion string
	found := false
	for _, p := range mm.Plugins {
		if p.Name == "lucind-ai" {
			marketplaceVersion = p.Version
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("%s: no plugin named \"lucind-ai\" in the plugins array", marketplacePath)
	}

	if pm.Version != marketplaceVersion {
		t.Fatalf("plugin.json version %q does not match marketplace.json version %q for plugin \"lucind-ai\" -- they must stay in lockstep", pm.Version, marketplaceVersion)
	}
}

func TestVerifyPacketTemplateAssetStructure(t *testing.T) {
	templatePath := filepath.Join("..", "..", "plugin", "claude-code", "skills", "lucind-ai", "assets", "verify-packet-template.md")
	data, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", templatePath, err)
	}
	content := string(data)

	p, err := packet.Parse(strings.NewReader(content))
	if err != nil {
		t.Fatalf("packet.Parse() error = %v", err)
	}

	if !p.ReadOnly {
		t.Errorf("Packet.ReadOnly = %v, want true", p.ReadOnly)
	}

	// Assert body contains exact frontmatter skeleton with read_only: true.
	if !strings.Contains(content, "read_only: true") {
		t.Errorf("template asset missing 'read_only: true' in frontmatter skeleton")
	}

	// Assert body contains the three read-only done criteria:
	// 1. "Every indirection introduced is demonstrably consumed by a terminal consumer."
	if !strings.Contains(p.Body, "Every indirection introduced is demonstrably consumed by a terminal consumer.") {
		t.Errorf("template body missing done criterion 1: 'Every indirection introduced is demonstrably consumed by a terminal consumer.'")
	}
	// 2. "The worktree carries no unique commits and no working-tree changes relative to the lane's birth point (`git status --porcelain` empty AND `HEAD` equals `git merge-base HEAD <primary HEAD>`)."
	if !strings.Contains(p.Body, "The worktree carries no unique commits and no working-tree changes relative to the lane's birth point (`git status --porcelain` empty AND `HEAD` equals `git merge-base HEAD <primary HEAD>`).") {
		t.Errorf("template body missing done criterion 2: 'The worktree carries no unique commits and no working-tree changes relative to the lane's birth point (`git status --porcelain` empty AND `HEAD` equals `git merge-base HEAD <primary HEAD>`).'")
	}
	// 3. "Qualitative evaluation completed" (.lucind/result.json populated with status, summary, and structured findings).
	if !strings.Contains(p.Body, "Qualitative evaluation completed") ||
		!strings.Contains(p.Body, ".lucind/result.json") ||
		!strings.Contains(p.Body, "status") ||
		!strings.Contains(p.Body, "summary") ||
		!strings.Contains(p.Body, "findings") {
		t.Errorf("template body missing done criterion 3 for qualitative evaluation completion")
	}

	// Assert body does NOT contain a write-packet commit done criterion ("The work is committed").
	if strings.Contains(p.Body, "The work is committed") {
		t.Errorf("template body contains write-packet commit criterion 'The work is committed', want it omitted")
	}

	// Assert ## Out of scope explicitly forbids executing go test, go build, go vet, lucind-checks.sh, or any shell test/build suite.
	if !strings.Contains(p.Body, "## Out of scope") {
		t.Errorf("template body missing '## Out of scope' section")
	}
	for _, forbidden := range []string{"go test", "go build", "go vet", "lucind-checks.sh"} {
		if !strings.Contains(p.Body, forbidden) {
			t.Errorf("template body out of scope section does not mention forbidden command %q", forbidden)
		}
	}

	// Assert ## Hard stops contains exact hard stop string:
	// "Executing mechanical test/build commands when mechanical results are already provided."
	wantHardStop := "Executing mechanical test/build commands when mechanical results are already provided."
	if !strings.Contains(p.Body, wantHardStop) {
		t.Errorf("template body missing hard stop %q", wantHardStop)
	}

	// Assert ## Context contains sections for embedding frozen mechanical log transcript and summary.
	if !strings.Contains(p.Body, "## Context") {
		t.Errorf("template body missing '## Context' section")
	}
	if !strings.Contains(p.Body, "transcript") || !strings.Contains(p.Body, "summary") {
		t.Errorf("template body ## Context missing mechanical log transcript or summary placeholders")
	}

	// Assert tool-selection guidance instructs using read/navigation tools (Read, Glob, Grep, codegraph) and read-only git queries (git diff, git log, git show).
	for _, tool := range []string{"Read", "Glob", "Grep", "codegraph", "git diff", "git log", "git show"} {
		if !strings.Contains(p.Body, tool) {
			t.Errorf("template body tool-selection guidance missing recommended tool/query %q", tool)
		}
	}
}

func TestPacketTemplateVerifyPointerNote(t *testing.T) {
	templatePath := filepath.Join("..", "..", "plugin", "claude-code", "skills", "lucind-ai", "assets", "packet-template.md")
	data, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", templatePath, err)
	}
	content := string(data)

	if !strings.Contains(content, "verify-packet-template.md") {
		t.Errorf("packet-template.md missing pointer note referencing 'verify-packet-template.md' for qualitative verification lanes")
	}
}

func TestHumanPacketTemplateUntouched(t *testing.T) {
	const rel = "plugin/claude-code/skills/lucind-ai/assets/human-packet-template.md"
	repoRoot := filepath.Join("..", "..")
	path := filepath.Join(repoRoot, rel)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	cmd := exec.Command("git", "diff", "--exit-code", "HEAD", "--", rel)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("human-packet-template.md must match git HEAD (this packet does not touch it):\n%s", out)
	}
}

func TestParseFeatureTargetFrontmatter(t *testing.T) {
	tests := []struct {
		name                  string
		src                   string
		wantFeature           string
		wantParentRef         string
		wantBaseSHA           string
		wantExpectedParentSHA string
	}{
		{
			name: "all four target fields present",
			src: "---\n" +
				"id: feat-auth-1\n" +
				"executor: agy\n" +
				"routed_by: touches auth, Tier A verification required\n" +
				"feature: user-auth\n" +
				"parent_ref: refs/heads/feature/user-auth\n" +
				"base_sha: 1111111111111111111111111111111111111111\n" +
				"expected_parent_sha: 2222222222222222222222222222222222222222\n" +
				"---\n\n" +
				"## Goal\n",
			wantFeature:           "user-auth",
			wantParentRef:         "refs/heads/feature/user-auth",
			wantBaseSHA:           "1111111111111111111111111111111111111111",
			wantExpectedParentSHA: "2222222222222222222222222222222222222222",
		},
		{
			name: "all four target fields omitted default to empty string",
			src: "---\n" +
				"id: feat-auth-1\n" +
				"executor: agy\n" +
				"routed_by: touches auth, Tier A verification required\n" +
				"---\n\n" +
				"## Goal\n",
			wantFeature:           "",
			wantParentRef:         "",
			wantBaseSHA:           "",
			wantExpectedParentSHA: "",
		},
		{
			name: "partial target fields with whitespace trimming",
			src: "---\n" +
				"id: feat-auth-1\n" +
				"executor: agy\n" +
				"routed_by: touches auth, Tier A verification required\n" +
				"feature:   user-auth   \n" +
				"parent_ref: refs/heads/main \n" +
				"---\n\n" +
				"## Goal\n",
			wantFeature:           "user-auth",
			wantParentRef:         "refs/heads/main",
			wantBaseSHA:           "",
			wantExpectedParentSHA: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := packet.Parse(strings.NewReader(tt.src))
			if err != nil {
				t.Fatalf("Parse() error = %v, want nil", err)
			}
			if p.Feature != tt.wantFeature {
				t.Errorf("Feature = %q, want %q", p.Feature, tt.wantFeature)
			}
			if p.ParentRef != tt.wantParentRef {
				t.Errorf("ParentRef = %q, want %q", p.ParentRef, tt.wantParentRef)
			}
			if p.BaseSHA != tt.wantBaseSHA {
				t.Errorf("BaseSHA = %q, want %q", p.BaseSHA, tt.wantBaseSHA)
			}
			if p.ExpectedParentSHA != tt.wantExpectedParentSHA {
				t.Errorf("ExpectedParentSHA = %q, want %q", p.ExpectedParentSHA, tt.wantExpectedParentSHA)
			}
		})
	}
}

func TestParseLegacyMainFrontmatter(t *testing.T) {
	tests := []struct {
		name           string
		src            string
		wantLegacyMain bool
		wantErr        error
	}{
		{
			name: "explicit legacy_main true",
			src: "---\n" +
				"id: fix-auth\n" +
				"executor: agy\n" +
				"routed_by: touches auth\n" +
				"legacy_main: true\n" +
				"---\n\n" +
				"## Goal\n",
			wantLegacyMain: true,
		},
		{
			name: "explicit legacy_main false",
			src: "---\n" +
				"id: fix-auth\n" +
				"executor: agy\n" +
				"routed_by: touches auth\n" +
				"legacy_main: false\n" +
				"---\n\n" +
				"## Goal\n",
			wantLegacyMain: false,
		},
		{
			name: "omitted legacy_main defaults to false",
			src: "---\n" +
				"id: fix-auth\n" +
				"executor: agy\n" +
				"routed_by: touches auth\n" +
				"---\n\n" +
				"## Goal\n",
			wantLegacyMain: false,
		},
		{
			name: "invalid legacy_main is rejected",
			src: "---\n" +
				"id: fix-auth\n" +
				"executor: agy\n" +
				"routed_by: touches auth\n" +
				"legacy_main: invalid\n" +
				"---\n\n" +
				"## Goal\n",
			wantErr: packet.ErrInvalidLegacyMain,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := packet.Parse(strings.NewReader(tt.src))
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Parse() error = nil, want %v", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Parse() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() error = %v, want nil", err)
			}
			if p.LegacyMain != tt.wantLegacyMain {
				t.Errorf("LegacyMain = %v, want %v", p.LegacyMain, tt.wantLegacyMain)
			}
		})
	}
}

// TestRemainingPacketTemplatesContract asserts that only the authorized non-SDD
// templates remain in both plugin asset directories and that they parse cleanly.
func TestRemainingPacketTemplatesContract(t *testing.T) {
	wantTemplates := []string{
		"human-packet-template.md",
		"packet-template.md",
		"ultrafixer-packet-template.md",
		"verify-packet-template.md",
	}

	roots := []string{
		filepath.Join("..", "..", "plugin", "claude-code", "skills", "lucind-ai", "assets"),
		filepath.Join("..", "..", "plugin", "opencode", "skills", "lucind-ai", "assets"),
	}

	for _, assetsDir := range roots {
		entries, err := os.ReadDir(assetsDir)
		if err != nil {
			t.Fatalf("ReadDir(%s) error = %v", assetsDir, err)
		}

		var gotFiles []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				gotFiles = append(gotFiles, e.Name())
			}
		}
		slices.Sort(gotFiles)

		if !slices.Equal(gotFiles, wantTemplates) {
			t.Errorf("assets in %s: got %v, want %v", assetsDir, gotFiles, wantTemplates)
		}

		for _, name := range wantTemplates {
			path := filepath.Join(assetsDir, name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("ReadFile(%s) error = %v", path, err)
				continue
			}
			if name == "human-packet-template.md" {
				if !strings.HasPrefix(string(data), "# Human packet") {
					t.Errorf("template %s missing expected heading", name)
				}
				continue
			}
			p, err := packet.Parse(strings.NewReader(string(data)))
			if err != nil {
				t.Errorf("packet.Parse(%s) error = %v", path, err)
				continue
			}
			if p.ID == "" {
				t.Errorf("template %s has empty ID", name)
			}
		}
	}
}

// TestUltrafixerPacketTemplateContract asserts that ultrafixer-packet-template.md
// conforms to the packet parser contract and contains the required frontmatter and sections.
func TestUltrafixerPacketTemplateContract(t *testing.T) {
	path := filepath.Join("..", "..", "plugin", "claude-code", "skills", "lucind-ai", "assets", "ultrafixer-packet-template.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	content := string(data)

	p, err := packet.Parse(strings.NewReader(content))
	if err != nil {
		t.Fatalf("packet.Parse() error = %v", err)
	}

	if p.ID != "<id>" {
		t.Errorf("ID = %q, want %q", p.ID, "<id>")
	}
	if p.Executor != "agy" {
		t.Errorf("Executor = %q, want %q", p.Executor, "agy")
	}
	if p.RoutedBy != "pre-existing defect triage and repair" {
		t.Errorf("RoutedBy = %q, want %q", p.RoutedBy, "pre-existing defect triage and repair")
	}
	if p.Model != "gemini-3.7-flash-high" {
		t.Errorf("Model = %q, want %q", p.Model, "gemini-3.7-flash-high")
	}
	if p.BaseSHA != "<base_sha>" {
		t.Errorf("BaseSHA = %q, want %q", p.BaseSHA, "<base_sha>")
	}
	if p.ParentRef != "<parent_ref>" {
		t.Errorf("ParentRef = %q, want %q", p.ParentRef, "<parent_ref>")
	}

	wantSections := []string{
		"## Goal",
		"## Preconditions",
		"## Done criteria",
		"## Allowed paths",
		"## Hard stops",
		"## Context",
		"### Failing check command",
		"### Error transcript and signature",
		"### Feature metadata",
		"## Return",
	}
	for _, sec := range wantSections {
		if !strings.Contains(content, sec) {
			t.Errorf("template missing expected section %q", sec)
		}
	}

	// Protocol assertions from ultrafixer-dispatch and defect-records specs
	if strings.Contains(content, "auto-merge") {
		t.Errorf("template contains contradictory 'auto-merge' label")
	}
	if !strings.Contains(content, "conventional commit") {
		t.Errorf("template missing conventional commit instruction")
	}
	if !strings.Contains(content, "Co-Authored-By") {
		t.Errorf("template missing Co-Authored-By prohibition")
	}
	if !strings.Contains(content, "Auto-integrating or merging the repair commit directly into any branch") {
		t.Errorf("template missing hard stop forbidding auto-integration")
	}
	if !strings.Contains(content, "lucind-ai defect decline") && !strings.Contains(content, "disposition=declined") {
		t.Errorf("template missing declined disposition instruction")
	}
}

func TestParseNewFrontmatterFields(t *testing.T) {
	src := "---\n" +
		"id: test-new-fields\n" +
		"executor: agy\n" +
		"routed_by: test route\n" +
		"route: worker\n" +
		"route_evidence: touches auth, 2+ non-trivial files\n" +
		"named_skills_only: true\n" +
		"verification: [\"go build ./...\", \"go test ./...\"]\n" +
		"known_environmental_failures: [\"TestFlakyNetwork\"]\n" +
		"---\n\n## Goal\nTest\n"

	p, err := packet.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}

	if p.Route != "worker" {
		t.Errorf("Route = %q, want %q", p.Route, "worker")
	}
	if p.RouteEvidence != "touches auth, 2+ non-trivial files" {
		t.Errorf("RouteEvidence = %q, want %q", p.RouteEvidence, "touches auth, 2+ non-trivial files")
	}
	if !p.NamedSkillsOnly {
		t.Errorf("NamedSkillsOnly = %v, want true", p.NamedSkillsOnly)
	}
	wantVerification := []string{"go build ./...", "go test ./..."}
	if !slices.Equal(p.Verification, wantVerification) {
		t.Errorf("Verification = %v, want %v", p.Verification, wantVerification)
	}
	wantKnownFailures := []string{"TestFlakyNetwork"}
	if !slices.Equal(p.KnownEnvironmentalFailures, wantKnownFailures) {
		t.Errorf("KnownEnvironmentalFailures = %v, want %v", p.KnownEnvironmentalFailures, wantKnownFailures)
	}
}

func TestParseNewFrontmatterValidation(t *testing.T) {
	validRoutes := []string{"", "inline", "worker", "fanout"}
	for _, r := range validRoutes {
		t.Run("valid route "+r, func(t *testing.T) {
			src := "---\n" +
				"id: test-route\n" +
				"executor: agy\n" +
				"routed_by: test\n" +
				"route: " + r + "\n" +
				"---\n\n## Goal\nTest\n"
			p, err := packet.Parse(strings.NewReader(src))
			if err != nil {
				t.Fatalf("Parse() error = %v, want nil", err)
			}
			if p.Route != r {
				t.Errorf("Route = %q, want %q", p.Route, r)
			}
		})
	}

	t.Run("invalid route rejected", func(t *testing.T) {
		src := "---\n" +
			"id: test-route\n" +
			"executor: agy\n" +
			"routed_by: test\n" +
			"route: invalid_route\n" +
			"---\n\n## Goal\nTest\n"
		_, err := packet.Parse(strings.NewReader(src))
		if !errors.Is(err, packet.ErrInvalidRoute) {
			t.Fatalf("Parse() error = %v, want %v", err, packet.ErrInvalidRoute)
		}
	})

	t.Run("invalid named_skills_only non-boolean rejected", func(t *testing.T) {
		src := "---\n" +
			"id: test-named\n" +
			"executor: agy\n" +
			"routed_by: test\n" +
			"named_skills_only: yes\n" +
			"---\n\n## Goal\nTest\n"
		_, err := packet.Parse(strings.NewReader(src))
		if !errors.Is(err, packet.ErrInvalidNamedSkillsOnly) {
			t.Fatalf("Parse() error = %v, want %v", err, packet.ErrInvalidNamedSkillsOnly)
		}
	})

	t.Run("valid named_skills_only false", func(t *testing.T) {
		src := "---\n" +
			"id: test-named\n" +
			"executor: agy\n" +
			"routed_by: test\n" +
			"named_skills_only: false\n" +
			"---\n\n## Goal\nTest\n"
		p, err := packet.Parse(strings.NewReader(src))
		if err != nil {
			t.Fatalf("Parse() error = %v, want nil", err)
		}
		if p.NamedSkillsOnly {
			t.Errorf("NamedSkillsOnly = true, want false")
		}
	})

	t.Run("invalid verification non-array rejected", func(t *testing.T) {
		src := "---\n" +
			"id: test-verif\n" +
			"executor: agy\n" +
			"routed_by: test\n" +
			"verification: bare-string\n" +
			"---\n\n## Goal\nTest\n"
		_, err := packet.Parse(strings.NewReader(src))
		if !errors.Is(err, packet.ErrInvalidVerification) {
			t.Fatalf("Parse() error = %v, want %v", err, packet.ErrInvalidVerification)
		}
	})

	t.Run("invalid verification non-string array rejected", func(t *testing.T) {
		src := "---\n" +
			"id: test-verif\n" +
			"executor: agy\n" +
			"routed_by: test\n" +
			"verification: [123]\n" +
			"---\n\n## Goal\nTest\n"
		_, err := packet.Parse(strings.NewReader(src))
		if !errors.Is(err, packet.ErrInvalidVerification) {
			t.Fatalf("Parse() error = %v, want %v", err, packet.ErrInvalidVerification)
		}
	})

	t.Run("invalid known_environmental_failures non-array rejected", func(t *testing.T) {
		src := "---\n" +
			"id: test-kef\n" +
			"executor: agy\n" +
			"routed_by: test\n" +
			"known_environmental_failures: bare-string\n" +
			"---\n\n## Goal\nTest\n"
		_, err := packet.Parse(strings.NewReader(src))
		if !errors.Is(err, packet.ErrInvalidKnownEnvironmentalFailures) {
			t.Fatalf("Parse() error = %v, want %v", err, packet.ErrInvalidKnownEnvironmentalFailures)
		}
	})

	t.Run("invalid known_environmental_failures non-string array rejected", func(t *testing.T) {
		src := "---\n" +
			"id: test-kef\n" +
			"executor: agy\n" +
			"routed_by: test\n" +
			"known_environmental_failures: [123]\n" +
			"---\n\n## Goal\nTest\n"
		_, err := packet.Parse(strings.NewReader(src))
		if !errors.Is(err, packet.ErrInvalidKnownEnvironmentalFailures) {
			t.Fatalf("Parse() error = %v, want %v", err, packet.ErrInvalidKnownEnvironmentalFailures)
		}
	})

	t.Run("legacy packet with sdd_phase parses cleanly", func(t *testing.T) {
		src := "---\n" +
			"id: test-legacy\n" +
			"executor: agy\n" +
			"routed_by: test\n" +
			"sdd_phase: apply\n" +
			"---\n\n## Goal\nTest\n"
		p, err := packet.Parse(strings.NewReader(src))
		if err != nil {
			t.Fatalf("Parse() error = %v, want nil", err)
		}
		if p.SDDPhase != "apply" {
			t.Errorf("SDDPhase = %q, want apply", p.SDDPhase)
		}
		if p.Route != "" || p.RouteEvidence != "" || p.NamedSkillsOnly != false || len(p.Verification) != 0 || len(p.KnownEnvironmentalFailures) != 0 {
			t.Errorf("unexpected non-zero fields in legacy packet: %+v", p)
		}
	})
}

func TestParseCommitMessage(t *testing.T) {
	validCases := []struct {
		name string
		msg  string
	}{
		{"standard feat", "feat: implement user auth"},
		{"with scope", "fix(auth): handle expired token"},
		{"with scope having slash and dash", "refactor(core/run-engine): simplify lifecycle"},
		{"breaking with exclamation", "feat!: breaking api change"},
		{"scope and breaking exclamation", "fix(db)!: change schema primary key"},
		{"docs", "docs: update readme"},
		{"chore", "chore(deps): bump go version"},
		{"style", "style: format imports"},
		{"perf", "perf(cache): optimize lookup"},
		{"test", "test: add unit tests"},
		{"build", "build(ci): update build step"},
		{"ci", "ci: fix linting pipeline"},
		{"revert", "revert: rollback commit 12345"},
		{"max 100 characters", "feat: " + strings.Repeat("a", 94)}, // 6 + 94 = 100
	}

	for _, tc := range validCases {
		t.Run("valid: "+tc.name, func(t *testing.T) {
			src := "---\n" +
				"id: test-lane\n" +
				"executor: agy\n" +
				"routed_by: test\n" +
				"verification: [\"go test ./...\"]\n" +
				"commit_message: " + tc.msg + "\n" +
				"---\n\n## Goal\nTest\n"
			p, err := packet.Parse(strings.NewReader(src))
			if err != nil {
				t.Fatalf("Parse() error = %v, want nil", err)
			}
			if p.CommitMessage != tc.msg {
				t.Errorf("CommitMessage = %q, want %q", p.CommitMessage, tc.msg)
			}
		})
	}

	invalidCases := []struct {
		name    string
		msg     string
		wantErr error
	}{
		{"invalid header Update stuff", "Update stuff", packet.ErrInvalidCommitMessage},
		{"missing colon", "feat implement login", packet.ErrInvalidCommitMessage},
		{"missing description", "feat:", packet.ErrInvalidCommitMessage},
		{"empty description after space", "feat: ", packet.ErrInvalidCommitMessage},
		{"unknown type", "unknown: implement login", packet.ErrInvalidCommitMessage},
		{"101 characters", "feat: " + strings.Repeat("a", 95), packet.ErrInvalidCommitMessage}, // 6 + 95 = 101
		{"co-authored-by case insensitive", "feat: add login Co-Authored-By: AI", packet.ErrInvalidCommitMessage},
		{"co-authored-by lower", "feat: add login co-authored-by: helper", packet.ErrInvalidCommitMessage},
		{"generated with case insensitive", "feat: add login (generated with LLM)", packet.ErrInvalidCommitMessage},
		{"generated with capital", "feat: add login Generated With Claude", packet.ErrInvalidCommitMessage},
	}

	for _, tc := range invalidCases {
		t.Run("invalid: "+tc.name, func(t *testing.T) {
			src := "---\n" +
				"id: test-lane\n" +
				"executor: agy\n" +
				"routed_by: test\n" +
				"verification: [\"go test ./...\"]\n" +
				"commit_message: " + tc.msg + "\n" +
				"---\n\n## Goal\nTest\n"
			_, err := packet.Parse(strings.NewReader(src))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Parse() error = %v, want %v", err, tc.wantErr)
			}
		})
	}

	t.Run("commit_message without verification rejected", func(t *testing.T) {
		src := "---\n" +
			"id: test-lane\n" +
			"executor: agy\n" +
			"routed_by: test\n" +
			"commit_message: feat: implement user auth\n" +
			"---\n\n## Goal\nTest\n"
		_, err := packet.Parse(strings.NewReader(src))
		if !errors.Is(err, packet.ErrCommitMessageNeedsVerification) {
			t.Fatalf("Parse() error = %v, want %v", err, packet.ErrCommitMessageNeedsVerification)
		}
	})

	t.Run("commit_message with empty verification array rejected", func(t *testing.T) {
		src := "---\n" +
			"id: test-lane\n" +
			"executor: agy\n" +
			"routed_by: test\n" +
			"verification: []\n" +
			"commit_message: feat: implement user auth\n" +
			"---\n\n## Goal\nTest\n"
		_, err := packet.Parse(strings.NewReader(src))
		if !errors.Is(err, packet.ErrCommitMessageNeedsVerification) {
			t.Fatalf("Parse() error = %v, want %v", err, packet.ErrCommitMessageNeedsVerification)
		}
	})

	t.Run("ValidateCommitMessage with newline or carriage return", func(t *testing.T) {
		if err := packet.ValidateCommitMessage("feat: line1\nline2"); !errors.Is(err, packet.ErrInvalidCommitMessage) {
			t.Fatalf("expected ErrInvalidCommitMessage, got %v", err)
		}
		if err := packet.ValidateCommitMessage("feat: line1\rline2"); !errors.Is(err, packet.ErrInvalidCommitMessage) {
			t.Fatalf("expected ErrInvalidCommitMessage, got %v", err)
		}
	})
}

func TestParseMaxIterations(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4} {
		t.Run(fmt.Sprintf("valid: %d", n), func(t *testing.T) {
			src := fmt.Sprintf("---\nid: test-lane\nexecutor: agy\nrouted_by: test\nmax_iterations: %d\nverification: [\"go test ./...\"]\n---\n\n## Goal\nTest\n", n)
			p, err := packet.Parse(strings.NewReader(src))
			if err != nil {
				t.Fatalf("Parse() error = %v, want nil", err)
			}
			if p.MaxIterations != n {
				t.Errorf("MaxIterations = %d, want %d", p.MaxIterations, n)
			}
		})
	}

	t.Run("absent max_iterations defaults to 0", func(t *testing.T) {
		src := "---\nid: test-lane\nexecutor: agy\nrouted_by: test\n---\n\n## Goal\nTest\n"
		p, err := packet.Parse(strings.NewReader(src))
		if err != nil {
			t.Fatalf("Parse() error = %v, want nil", err)
		}
		if p.MaxIterations != 0 {
			t.Errorf("MaxIterations = %d, want 0", p.MaxIterations)
		}
	})

	invalid := []string{"0", "-1", "5", "abc", "1.5", ""}
	for _, val := range invalid {
		t.Run("invalid: "+val, func(t *testing.T) {
			src := fmt.Sprintf("---\nid: test-lane\nexecutor: agy\nrouted_by: test\nmax_iterations: %s\nverification: [\"go test ./...\"]\n---\n\n## Goal\nTest\n", val)
			_, err := packet.Parse(strings.NewReader(src))
			if !errors.Is(err, packet.ErrInvalidMaxIterations) {
				t.Fatalf("Parse() error = %v, want %v", err, packet.ErrInvalidMaxIterations)
			}
		})
	}
}

func TestParseEscalation(t *testing.T) {
	t.Run("valid single rung", func(t *testing.T) {
		src := "---\nid: test-lane\nexecutor: agy\nrouted_by: test\nverification: [\"go test ./...\"]\nescalation: [{\"executor\":\"herdr-agy\",\"model\":\"gemini-3.8-flash-high\"}]\n---\n\n## Goal\nTest\n"
		p, err := packet.Parse(strings.NewReader(src))
		if err != nil {
			t.Fatalf("Parse() error = %v, want nil", err)
		}
		if len(p.Escalation) != 1 {
			t.Fatalf("len(Escalation) = %d, want 1", len(p.Escalation))
		}
		if p.Escalation[0].Executor != "herdr-agy" || p.Escalation[0].Model != "gemini-3.8-flash-high" {
			t.Errorf("Escalation[0] = %+v, want {herdr-agy, gemini-3.8-flash-high}", p.Escalation[0])
		}
	})

	t.Run("valid 3 rungs with empty models", func(t *testing.T) {
		src := "---\nid: test-lane\nexecutor: agy\nrouted_by: test\nverification: [\"go test ./...\"]\nescalation: [{\"executor\":\"herdr-agy\"},{\"executor\":\"cursor-agent\",\"model\":\"claude-3.7-sonnet\"},{\"executor\":\"opencode\"}]\n---\n\n## Goal\nTest\n"
		p, err := packet.Parse(strings.NewReader(src))
		if err != nil {
			t.Fatalf("Parse() error = %v, want nil", err)
		}
		if len(p.Escalation) != 3 {
			t.Fatalf("len(Escalation) = %d, want 3", len(p.Escalation))
		}
		if p.Escalation[0].Executor != "herdr-agy" || p.Escalation[0].Model != "" {
			t.Errorf("Escalation[0] = %+v, want {herdr-agy, empty}", p.Escalation[0])
		}
		if p.Escalation[2].Executor != "opencode" || p.Escalation[2].Model != "" {
			t.Errorf("Escalation[2] = %+v, want {opencode, empty}", p.Escalation[2])
		}
	})

	invalid := []struct {
		name string
		val  string
	}{
		{"empty string", ""},
		{"not an array", "{\"executor\":\"herdr-agy\"}"},
		{"array of strings", "[\"herdr-agy\"]"},
		{"empty executor", "[{\"executor\":\"\"}]"},
		{"missing executor", "[{\"model\":\"gemini-3.8-flash-high\"}]"},
		{"unknown key", "[{\"executor\":\"herdr-agy\",\"unknown_field\":\"bogus\"}]"},
		{"more than 3 rungs", "[{\"executor\":\"a\"},{\"executor\":\"b\"},{\"executor\":\"c\"},{\"executor\":\"d\"}]"},
		{"trailing garbage", "[{\"executor\":\"herdr-agy\"}] trailing"},
	}
	for _, tc := range invalid {
		t.Run("invalid: "+tc.name, func(t *testing.T) {
			src := fmt.Sprintf("---\nid: test-lane\nexecutor: agy\nrouted_by: test\nverification: [\"go test ./...\"]\nescalation: %s\n---\n\n## Goal\nTest\n", tc.val)
			_, err := packet.Parse(strings.NewReader(src))
			if !errors.Is(err, packet.ErrInvalidEscalation) {
				t.Fatalf("Parse() error = %v, want %v", err, packet.ErrInvalidEscalation)
			}
		})
	}
}

func TestLoopNeedsVerification(t *testing.T) {
	t.Run("max_iterations > 1 without verification rejected", func(t *testing.T) {
		src := "---\nid: test-lane\nexecutor: agy\nrouted_by: test\nmax_iterations: 2\n---\n\n## Goal\nTest\n"
		_, err := packet.Parse(strings.NewReader(src))
		if !errors.Is(err, packet.ErrLoopNeedsVerification) {
			t.Fatalf("Parse() error = %v, want %v", err, packet.ErrLoopNeedsVerification)
		}
	})

	t.Run("max_iterations > 1 with empty verification array rejected", func(t *testing.T) {
		src := "---\nid: test-lane\nexecutor: agy\nrouted_by: test\nmax_iterations: 2\nverification: []\n---\n\n## Goal\nTest\n"
		_, err := packet.Parse(strings.NewReader(src))
		if !errors.Is(err, packet.ErrLoopNeedsVerification) {
			t.Fatalf("Parse() error = %v, want %v", err, packet.ErrLoopNeedsVerification)
		}
	})

	t.Run("escalation without verification rejected", func(t *testing.T) {
		src := "---\nid: test-lane\nexecutor: agy\nrouted_by: test\nescalation: [{\"executor\":\"herdr-agy\"}]\n---\n\n## Goal\nTest\n"
		_, err := packet.Parse(strings.NewReader(src))
		if !errors.Is(err, packet.ErrLoopNeedsVerification) {
			t.Fatalf("Parse() error = %v, want %v", err, packet.ErrLoopNeedsVerification)
		}
	})

	t.Run("max_iterations 1 without verification allowed", func(t *testing.T) {
		src := "---\nid: test-lane\nexecutor: agy\nrouted_by: test\nmax_iterations: 1\n---\n\n## Goal\nTest\n"
		p, err := packet.Parse(strings.NewReader(src))
		if err != nil {
			t.Fatalf("Parse() error = %v, want nil", err)
		}
		if p.MaxIterations != 1 {
			t.Errorf("MaxIterations = %d, want 1", p.MaxIterations)
		}
	})
}
