package packet_test

import (
	"errors"
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
