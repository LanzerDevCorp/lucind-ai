package packet_test

import (
	"errors"
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
	skillPath := filepath.Join("..", "..", "plugin", "claude-code", "skills", "lucind-ai", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", skillPath, err)
	}
	content := string(data)

	// Explore is documented as dispatchable via lucind-ai run.
	if !strings.Contains(content, "Dispatch via `lucind-ai run`") {
		t.Errorf("SKILL.md does not document explore dispatch via lucind-ai run")
	}

	// Mandatory criterion 2 states the read-only exception.
	if !strings.Contains(content, "*Mandatory criterion 2*") {
		t.Errorf("SKILL.md missing mandatory criterion 2")
	}
	if !strings.Contains(content, "read_only: true") || !strings.Contains(content, "git merge-base HEAD <primary HEAD>") {
		t.Errorf("SKILL.md mandatory criterion 2 missing read-only exception with merge-base check")
	}

	// Explore blocker row no longer says the exception is missing.
	if strings.Contains(content, "Needs an explicit read-only-packet exception") {
		t.Errorf("SKILL.md explore blocker row still states the exception is missing")
	}

	// Apply row now documents the built DAG-split loop (apply-dag-dispatch
	// Phase 6); verify row remains untouched until verify-dual-dispatch's
	// own SKILL.md documentation phase runs.
	if !strings.Contains(content, "lucind-ai split --dag") {
		t.Errorf("SKILL.md apply row does not document the built split loop")
	}
	if !strings.Contains(content, "Dual-dispatch `agy` + `cursor-agent` for the *qualitative* half of verification") {
		t.Errorf("SKILL.md verify row was modified or removed")
	}

	// Unrelated section present from earlier in the session is preserved.
	if !strings.Contains(content, "### Where to author packet files") {
		t.Errorf("SKILL.md missing 'Where to author packet files' section")
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

	// Feature-branch ownership (2.3-RED).
	if !strings.Contains(content, "feature create") {
		t.Errorf("SKILL.md missing feature create orchestration guidance")
	}
	if !strings.Contains(content, "Lanes do not create or move parent refs") &&
		!strings.Contains(content, "lanes do not create or move parent refs") &&
		!strings.Contains(content, "Lanes do not create or move parent references") {
		t.Errorf("SKILL.md missing feature-branch lane immutability rule")
	}

	// Shipped subcommands and run flags (2.5-RED).
	for _, cmd := range []string{"serve", "feature", "reconcile", "renew"} {
		if !strings.Contains(content, "lucind-ai "+cmd) && !strings.Contains(content, "`"+cmd+"`") && !strings.Contains(content, cmd) {
			t.Errorf("SKILL.md invocation/CLI section missing subcommand %q", cmd)
		}
	}
	for _, flag := range []string{"--approval-timeout", "--legacy-main", "--expected-parent-sha"} {
		if !strings.Contains(content, flag) {
			t.Errorf("SKILL.md invocation/CLI section missing run flag %q", flag)
		}
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

func TestSkillMDVerifyOperationalWorkflow(t *testing.T) {
	skillPath := filepath.Join("..", "..", "plugin", "claude-code", "skills", "lucind-ai", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", skillPath, err)
	}
	content := string(data)

	verifyRow, ok := skillMDTableRow(content, "`verify`")
	if !ok {
		t.Fatal("SKILL.md target-direction table missing `verify` row")
	}

	// Target-direction verify row: operational two-stage dispatch, not a blocker.
	if !strings.Contains(verifyRow, "lucind-ai check") {
		t.Errorf("verify row does not document Stage 1 mechanical check via lucind-ai check:\n%s", verifyRow)
	}
	if !strings.Contains(verifyRow, "agy") || !strings.Contains(verifyRow, "cursor-agent") {
		t.Errorf("verify row does not document Stage 2 qualitative judgment dual-dispatched to agy + cursor-agent:\n%s", verifyRow)
	}
	if !strings.Contains(verifyRow, "Built. See **Verify dispatch** below.") {
		t.Errorf("verify row is not marked built / does not point at Verify dispatch:\n%s", verifyRow)
	}
	for _, blocked := range []string{
		"once tooling exists",
		"not yet built",
		"Needs an explicit",
	} {
		if strings.Contains(verifyRow, blocked) {
			t.Errorf("verify row still contains blocked/unbuilt language %q:\n%s", blocked, verifyRow)
		}
	}

	// Stage 1: Mechanical Check
	if !strings.Contains(content, "Stage 1: Mechanical Check") {
		t.Error("SKILL.md missing Stage 1: Mechanical Check")
	}
	if !strings.Contains(content, "lucind-ai check --out openspec/changes/<change-id>/verify-mechanical.log") {
		t.Error("SKILL.md missing lucind-ai check --out openspec/changes/<change-id>/verify-mechanical.log")
	}
	if !strings.Contains(content, "Halts immediately") && !strings.Contains(content, "halt immediately") {
		t.Error("SKILL.md Stage 1 does not say verification halts immediately if checks fail")
	}
	if !strings.Contains(content, "candidate branch") || !strings.Contains(strings.ToLower(content), "commit") {
		t.Error("SKILL.md Stage 1 does not say the log is committed to the candidate branch HEAD on pass")
	}

	// Stage 2: Dual Qualitative Judgment Dispatch
	if !strings.Contains(content, "Stage 2: Dual Qualitative Judgment Dispatch") {
		t.Error("SKILL.md missing Stage 2: Dual Qualitative Judgment Dispatch")
	}
	if !strings.Contains(content, "packets/verify-<id>-agy.md") {
		t.Error("SKILL.md missing packets/verify-<id>-agy.md")
	}
	if !strings.Contains(content, "packets/verify-<id>-cursor-agent.md") {
		t.Error("SKILL.md missing packets/verify-<id>-cursor-agent.md")
	}
	if !strings.Contains(content, "verify-packet-template.md") {
		t.Error("SKILL.md missing verify-packet-template.md")
	}
	if !strings.Contains(content, "read_only: true") {
		t.Error("SKILL.md missing read_only: true")
	}
	if !strings.Contains(content, "## Context") {
		t.Error("SKILL.md missing frozen mechanical summary in ## Context")
	}
	if !strings.Contains(content, "lucind-ai run --packet") || !strings.Contains(content, "verify-<id>-agy.md") || !strings.Contains(content, "verify-<id>-cursor-agent.md") {
		t.Error("SKILL.md missing parallel lucind-ai run --packet dispatch of both verify lanes")
	}
	if !strings.Contains(strings.ToLower(content), "barrier") || !strings.Contains(strings.ToLower(content), "terminal") {
		t.Error("SKILL.md Stage 2 does not document barrier join when both lanes reach terminal status")
	}

	// Stage 3: Evidence Cross-Checking & Verdict Reconciliation
	if !strings.Contains(content, "Stage 3: Evidence Cross-Checking & Verdict Reconciliation") {
		t.Error("SKILL.md missing Stage 3: Evidence Cross-Checking & Verdict Reconciliation")
	}
	if !strings.Contains(content, ".lucind/result.json") {
		t.Error("SKILL.md Stage 3 does not read both .lucind/result.json envelopes")
	}
	if !strings.Contains(content, "file:line") {
		t.Error("SKILL.md Stage 3 does not independently verify cited file:line evidence")
	}

	for _, want := range []string{
		"Unanimous Pass",
		"`done`/`done`",
		"openspec/changes/<id>/verify.md",
		"PASSED",
		"verify: { status: done }",
		"Disagreement / Disputed Defects",
		"`blocked`/`deviated`",
		"BLOCKED",
		"Lane Failure",
		"`failed`",
		"Irreconcilable Ambiguity",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("SKILL.md Stage 3 missing %q", want)
		}
	}
}

func skillMDTableRow(content, phaseCell string) (string, bool) {
	needle := "| " + phaseCell + " |"
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, needle) {
			return line, true
		}
	}
	return "", false
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
		name                   string
		src                    string
		wantFeature            string
		wantParentRef          string
		wantBaseSHA            string
		wantExpectedParentSHA  string
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

func TestExplorePacketTemplatesContract(t *testing.T) {
	assetsDir := filepath.Join("..", "..", "plugin", "claude-code", "skills", "lucind-ai", "assets")
	templates := []struct {
		filename      string
		wantID        string
		wantExecutor  string
		wantPaths     []string
		wantStrings   []string
		forbidStrings []string
	}{
		{
			filename:     "explore-lens-a-packet-template.md",
			wantID:       "explore-<change-id>-lens-a",
			wantExecutor: "agy",
			wantPaths:    []string{"openspec/changes/<change-id>/explore-lens-a.md"},
			wantStrings: []string{
				"problem and candidates",
				"~/.claude/skills/sdd-explore/SKILL.md",
				"Explore Lens A — Problem & Candidates",
				"Lens B owns",
				"Lens C owns",
				"1000 words",
			},
		},
		{
			filename:     "explore-lens-b-packet-template.md",
			wantID:       "explore-<change-id>-lens-b",
			wantExecutor: "agy",
			wantPaths:    []string{"openspec/changes/<change-id>/explore-lens-b.md"},
			wantStrings: []string{
				"capabilities and scenarios",
				"~/.claude/skills/sdd-explore/SKILL.md",
				"Explore Lens B — Capabilities & Scenarios",
				"Lens A owns",
				"Lens C owns",
				"1000 words",
			},
		},
		{
			filename:     "explore-lens-c-packet-template.md",
			wantID:       "explore-<change-id>-lens-c",
			wantExecutor: "agy",
			wantPaths:    []string{"openspec/changes/<change-id>/explore-lens-c.md"},
			wantStrings: []string{
				"risks, trade-offs",
				"~/.claude/skills/sdd-explore/SKILL.md",
				"Explore Lens C — Risks, Trade-offs & Spikes",
				"Lens A owns",
				"Lens B owns",
				"1000 words",
			},
		},
		{
			filename:     "explore-synthesis-packet-template.md",
			wantID:       "explore-<change-id>-synthesis",
			wantExecutor: "cursor-agent",
			wantPaths: []string{
				"openspec/changes/<change-id>/explore.md",
				"openspec/changes/<change-id>/explore-synthesis-notes.md",
			},
			wantStrings: []string{
				"explore-lens-a.md",
				"explore-lens-b.md",
				"explore-lens-c.md",
				"explore.md",
				"explore-synthesis-notes.md",
				"## Unresolved Contradictions",
				"## Coverage Gaps",
				"## Dropped Citations",
				"## Approach Divergence",
				"1800 words",
			},
			forbidStrings: []string{
				"## Architecture Divergence",
				"architecture divergence",
			},
		},
	}

	var lensPackets []packet.Packet
	for _, tt := range templates {
		t.Run(tt.filename, func(t *testing.T) {
			path := filepath.Join(assetsDir, tt.filename)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%s) error = %v", path, err)
			}
			content := string(data)

			p, err := packet.Parse(strings.NewReader(content))
			if err != nil {
				t.Fatalf("packet.Parse(%s) error = %v", tt.filename, err)
			}

			if p.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", p.ID, tt.wantID)
			}
			if p.Executor != tt.wantExecutor {
				t.Errorf("Executor = %q, want %q", p.Executor, tt.wantExecutor)
			}
			if p.RoutedBy == "" {
				t.Errorf("RoutedBy is empty")
			}
			if !slices.Equal(p.AllowedPaths, tt.wantPaths) {
				t.Errorf("AllowedPaths = %v, want %v", p.AllowedPaths, tt.wantPaths)
			}
			for _, ws := range tt.wantStrings {
				if !strings.Contains(content, ws) {
					t.Errorf("template %s missing expected string %q", tt.filename, ws)
				}
			}
			for _, fs := range tt.forbidStrings {
				if strings.Contains(content, fs) {
					t.Errorf("template %s contains forbidden string %q", tt.filename, fs)
				}
			}

			// The amended Planning Fan-Out Template Assets requirement makes
			// "declares no dispatch target" the default for a reusable
			// template, and this is the assertion that defends it. A template
			// that bakes legacy_main: true silently targets main even when the
			// change runs against a named feature parent -- the exact coupling
			// the amendment removed. Without this check, reintroducing that
			// field would pass every other assertion in this table.
			if p.LegacyMain {
				t.Errorf("template %s declares legacy_main: true; a reusable template must declare no dispatch target", tt.filename)
			}
			if p.Feature != "" || p.ParentRef != "" || p.BaseSHA != "" || p.ExpectedParentSHA != "" {
				t.Errorf("template %s declares feature-target fields (feature=%q parent_ref=%q base_sha=%q expected_parent_sha=%q); a reusable template must declare no dispatch target",
					tt.filename, p.Feature, p.ParentRef, p.BaseSHA, p.ExpectedParentSHA)
			}

			if strings.Contains(tt.filename, "lens") {
				lensPackets = append(lensPackets, p)
			}
		})
	}

	if len(lensPackets) == 3 {
		if err := packet.DisjointAllowedPaths(lensPackets); err != nil {
			t.Errorf("DisjointAllowedPaths(explore lenses) error = %v", err)
		}
	}
}

func TestProposePacketTemplatesContract(t *testing.T) {
	assetsDir := filepath.Join("..", "..", "plugin", "claude-code", "skills", "lucind-ai", "assets")
	templates := []struct {
		filename      string
		wantID        string
		wantExecutor  string
		wantPaths     []string
		wantStrings   []string
		forbidStrings []string
	}{
		{
			filename:     "propose-lens-a-packet-template.md",
			wantID:       "propose-<change-id>-lens-a",
			wantExecutor: "agy",
			wantPaths:    []string{"openspec/changes/<change-id>/propose-lens-a.md"},
			wantStrings: []string{
				"candidate and approach",
				"~/.claude/skills/sdd-propose/SKILL.md",
				"Proposal Lens A — Candidate & Approach",
				"Lens B owns",
				"Lens C owns",
				"1000 words",
			},
		},
		{
			filename:     "propose-lens-b-packet-template.md",
			wantID:       "propose-<change-id>-lens-b",
			wantExecutor: "agy",
			wantPaths:    []string{"openspec/changes/<change-id>/propose-lens-b.md"},
			wantStrings: []string{
				"capability impact and delta specs",
				"~/.claude/skills/sdd-propose/SKILL.md",
				"Proposal Lens B — Capability Impact & Specs",
				"Lens A owns",
				"Lens C owns",
				"1000 words",
			},
		},
		{
			filename:     "propose-lens-c-packet-template.md",
			wantID:       "propose-<change-id>-lens-c",
			wantExecutor: "agy",
			wantPaths:    []string{"openspec/changes/<change-id>/propose-lens-c.md"},
			wantStrings: []string{
				"risks, rollback, and test impact",
				"~/.claude/skills/sdd-propose/SKILL.md",
				"Proposal Lens C — Risks, Rollback & Test Impact",
				"Lens A owns",
				"Lens B owns",
				"1000 words",
			},
		},
		{
			filename:     "propose-synthesis-packet-template.md",
			wantID:       "propose-<change-id>-synthesis",
			wantExecutor: "cursor-agent",
			wantPaths: []string{
				"openspec/changes/<change-id>/proposal.md",
				"openspec/changes/<change-id>/proposal-synthesis-notes.md",
			},
			wantStrings: []string{
				"propose-lens-a.md",
				"propose-lens-b.md",
				"propose-lens-c.md",
				"proposal.md",
				"proposal-synthesis-notes.md",
				"## Unresolved Contradictions",
				"## Coverage Gaps",
				"## Dropped Citations",
				"## Scope Divergence",
				"1800 words",
			},
			forbidStrings: []string{
				"## Architecture Divergence",
			},
		},
	}

	var lensPackets []packet.Packet
	for _, tt := range templates {
		t.Run(tt.filename, func(t *testing.T) {
			path := filepath.Join(assetsDir, tt.filename)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%s) error = %v", path, err)
			}
			content := string(data)

			p, err := packet.Parse(strings.NewReader(content))
			if err != nil {
				t.Fatalf("packet.Parse(%s) error = %v", tt.filename, err)
			}

			if p.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", p.ID, tt.wantID)
			}
			if p.Executor != tt.wantExecutor {
				t.Errorf("Executor = %q, want %q", p.Executor, tt.wantExecutor)
			}
			if p.RoutedBy == "" {
				t.Errorf("RoutedBy is empty")
			}
			if !slices.Equal(p.AllowedPaths, tt.wantPaths) {
				t.Errorf("AllowedPaths = %v, want %v", p.AllowedPaths, tt.wantPaths)
			}
			for _, ws := range tt.wantStrings {
				if !strings.Contains(content, ws) {
					t.Errorf("template %s missing expected string %q", tt.filename, ws)
				}
			}
			for _, fs := range tt.forbidStrings {
				if strings.Contains(content, fs) {
					t.Errorf("template %s contains forbidden string %q", tt.filename, fs)
				}
			}

			// The amended Planning Fan-Out Template Assets requirement makes
			// "declares no dispatch target" the default for a reusable
			// template, and this is the assertion that defends it. A template
			// that bakes legacy_main: true silently targets main even when the
			// change runs against a named feature parent -- the exact coupling
			// the amendment removed. Without this check, reintroducing that
			// field would pass every other assertion in this table.
			if p.LegacyMain {
				t.Errorf("template %s declares legacy_main: true; a reusable template must declare no dispatch target", tt.filename)
			}
			if p.Feature != "" || p.ParentRef != "" || p.BaseSHA != "" || p.ExpectedParentSHA != "" {
				t.Errorf("template %s declares feature-target fields (feature=%q parent_ref=%q base_sha=%q expected_parent_sha=%q); a reusable template must declare no dispatch target",
					tt.filename, p.Feature, p.ParentRef, p.BaseSHA, p.ExpectedParentSHA)
			}

			if strings.Contains(tt.filename, "lens") {
				lensPackets = append(lensPackets, p)
			}
		})
	}

	if len(lensPackets) == 3 {
		if err := packet.DisjointAllowedPaths(lensPackets); err != nil {
			t.Errorf("DisjointAllowedPaths(propose lenses) error = %v", err)
		}
	}
}

func TestDesignPacketTemplatesContract(t *testing.T) {
	assetsDir := filepath.Join("..", "..", "plugin", "claude-code", "skills", "lucind-ai", "assets")
	templates := []struct {
		filename      string
		wantID        string
		wantExecutor  string
		wantPaths     []string
		wantStrings   []string
		forbidStrings []string
	}{
		{
			filename:     "design-lens-a-packet-template.md",
			wantID:       "design-<change-id>-lens-a",
			wantExecutor: "agy",
			wantPaths:    []string{"openspec/changes/<change-id>/design-lens-a.md"},
			wantStrings: []string{
				"decisions lens",
				"~/.claude/skills/sdd-design/SKILL.md",
				"Design Lens A — Decisions",
				"Lens B owns",
				"Lens C owns",
				"1000 words",
			},
		},
		{
			filename:     "design-lens-b-packet-template.md",
			wantID:       "design-<change-id>-lens-b",
			wantExecutor: "agy",
			wantPaths:    []string{"openspec/changes/<change-id>/design-lens-b.md"},
			wantStrings: []string{
				"surface-and-flow lens",
				"~/.claude/skills/sdd-design/SKILL.md",
				"Design Lens B — Surface & Flow",
				"Lens A owns",
				"Lens C owns",
				"1000 words",
			},
		},
		{
			filename:     "design-lens-c-packet-template.md",
			wantID:       "design-<change-id>-lens-c",
			wantExecutor: "agy",
			wantPaths:    []string{"openspec/changes/<change-id>/design-lens-c.md"},
			wantStrings: []string{
				"failure-test-rollback lens",
				"~/.claude/skills/sdd-design/SKILL.md",
				"Design Lens C — Failure, Test & Rollback",
				"Lens A owns",
				"Lens B owns",
				"1000 words",
			},
		},
		{
			filename:     "design-synthesis-packet-template.md",
			wantID:       "design-<change-id>-synthesis",
			wantExecutor: "cursor-agent",
			wantPaths: []string{
				"openspec/changes/<change-id>/design.md",
				"openspec/changes/<change-id>/design-synthesis-notes.md",
			},
			wantStrings: []string{
				"design-lens-a.md",
				"design-lens-b.md",
				"design-lens-c.md",
				"design.md",
				"design-synthesis-notes.md",
				"## Unresolved Contradictions",
				"## Coverage Gaps",
				"## Dropped Citations",
				"1800 words",
			},
		},
	}

	var lensPackets []packet.Packet
	for _, tt := range templates {
		t.Run(tt.filename, func(t *testing.T) {
			path := filepath.Join(assetsDir, tt.filename)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%s) error = %v", path, err)
			}
			content := string(data)

			p, err := packet.Parse(strings.NewReader(content))
			if err != nil {
				t.Fatalf("packet.Parse(%s) error = %v", tt.filename, err)
			}

			if p.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", p.ID, tt.wantID)
			}
			if p.Executor != tt.wantExecutor {
				t.Errorf("Executor = %q, want %q", p.Executor, tt.wantExecutor)
			}
			if p.RoutedBy == "" {
				t.Errorf("RoutedBy is empty")
			}
			if !slices.Equal(p.AllowedPaths, tt.wantPaths) {
				t.Errorf("AllowedPaths = %v, want %v", p.AllowedPaths, tt.wantPaths)
			}
			for _, ws := range tt.wantStrings {
				if !strings.Contains(content, ws) {
					t.Errorf("template %s missing expected string %q", tt.filename, ws)
				}
			}
			for _, fs := range tt.forbidStrings {
				if strings.Contains(content, fs) {
					t.Errorf("template %s contains forbidden string %q", tt.filename, fs)
				}
			}

			// The amended Planning Fan-Out Template Assets requirement makes
			// "declares no dispatch target" the default for a reusable
			// template, and this is the assertion that defends it. A template
			// that bakes legacy_main: true silently targets main even when the
			// change runs against a named feature parent -- the exact coupling
			// the amendment removed. Without this check, reintroducing that
			// field would pass every other assertion in this table.
			if p.LegacyMain {
				t.Errorf("template %s declares legacy_main: true; a reusable template must declare no dispatch target", tt.filename)
			}
			if p.Feature != "" || p.ParentRef != "" || p.BaseSHA != "" || p.ExpectedParentSHA != "" {
				t.Errorf("template %s declares feature-target fields (feature=%q parent_ref=%q base_sha=%q expected_parent_sha=%q); a reusable template must declare no dispatch target",
					tt.filename, p.Feature, p.ParentRef, p.BaseSHA, p.ExpectedParentSHA)
			}

			if strings.Contains(tt.filename, "lens") {
				lensPackets = append(lensPackets, p)
			}
		})
	}

	if len(lensPackets) == 3 {
		if err := packet.DisjointAllowedPaths(lensPackets); err != nil {
			t.Errorf("DisjointAllowedPaths(design lenses) error = %v", err)
		}
	}
}




func TestSpecPacketTemplatesContract(t *testing.T) {
	assetsDir := filepath.Join("..", "..", "plugin", "claude-code", "skills", "lucind-ai", "assets")
	templates := []struct {
		filename      string
		wantID        string
		wantExecutor  string
		wantPaths     []string
		wantStrings   []string
		forbidStrings []string
	}{
		{
			filename:     "spec-lens-a-packet-template.md",
			wantID:       "spec-<change-id>-lens-a",
			wantExecutor: "agy",
			wantPaths:    []string{"openspec/changes/<change-id>/spec-lens-a.md"},
			wantStrings: []string{
				"capabilities and requirements",
				"~/.claude/skills/sdd-spec/SKILL.md",
				"Spec Lens A — Capabilities & Requirements",
				"Lens B owns",
				"Lens C owns",
				"1000 words",
			},
		},
		{
			filename:     "spec-lens-b-packet-template.md",
			wantID:       "spec-<change-id>-lens-b",
			wantExecutor: "agy",
			wantPaths:    []string{"openspec/changes/<change-id>/spec-lens-b.md"},
			wantStrings: []string{
				"scenarios and coverage",
				"~/.claude/skills/sdd-spec/SKILL.md",
				"Spec Lens B — Scenarios & Coverage",
				"Lens A owns",
				"Lens C owns",
				"1000 words",
			},
		},
		{
			filename:     "spec-lens-c-packet-template.md",
			wantID:       "spec-<change-id>-lens-c",
			wantExecutor: "agy",
			wantPaths:    []string{"openspec/changes/<change-id>/spec-lens-c.md"},
			wantStrings: []string{
				"live-spec conflict and migration",
				"~/.claude/skills/sdd-spec/SKILL.md",
				"Spec Lens C — Live-Spec Conflicts & Migration",
				"Lens A owns",
				"Lens B owns",
				"1000 words",
				// Lens C is the only lane that opens the live specs in full, so
				// the verbatim full-block section is the property that keeps
				// archive from silently deleting scenarios a partial MODIFIED
				// block failed to copy. Losing this heading loses the lens.
				"## MODIFIED Full Blocks",
			},
		},
		{
			filename:     "spec-synthesis-packet-template.md",
			wantID:       "spec-<change-id>-synthesis",
			wantExecutor: "cursor-agent",
			wantPaths: []string{
				"openspec/changes/<change-id>/specs/",
				"openspec/changes/<change-id>/spec-synthesis-notes.md",
			},
			wantStrings: []string{
				"spec-lens-a.md",
				"spec-lens-b.md",
				"spec-lens-c.md",
				"spec-synthesis-notes.md",
				"## Unresolved Contradictions",
				"## Coverage Gaps",
				"## Dropped Citations",
				"## Requirement Divergence",
				"1800 words",
			},
			forbidStrings: []string{
				"## Architecture Divergence",
			},
		},
	}

	var lensPackets []packet.Packet
	for _, tt := range templates {
		t.Run(tt.filename, func(t *testing.T) {
			path := filepath.Join(assetsDir, tt.filename)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%s) error = %v", path, err)
			}
			content := string(data)

			p, err := packet.Parse(strings.NewReader(content))
			if err != nil {
				t.Fatalf("packet.Parse(%s) error = %v", tt.filename, err)
			}

			if p.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", p.ID, tt.wantID)
			}
			if p.Executor != tt.wantExecutor {
				t.Errorf("Executor = %q, want %q", p.Executor, tt.wantExecutor)
			}
			if p.RoutedBy == "" {
				t.Errorf("RoutedBy is empty")
			}
			if !slices.Equal(p.AllowedPaths, tt.wantPaths) {
				t.Errorf("AllowedPaths = %v, want %v", p.AllowedPaths, tt.wantPaths)
			}
			for _, ws := range tt.wantStrings {
				if !strings.Contains(content, ws) {
					t.Errorf("template %s missing expected string %q", tt.filename, ws)
				}
			}
			for _, fs := range tt.forbidStrings {
				if strings.Contains(content, fs) {
					t.Errorf("template %s contains forbidden string %q", tt.filename, fs)
				}
			}

			// The amended Planning Fan-Out Template Assets requirement makes
			// "declares no dispatch target" the default for a reusable
			// template, and this is the assertion that defends it. A template
			// that bakes legacy_main: true silently targets main even when the
			// change runs against a named feature parent -- the exact coupling
			// the amendment removed. Without this check, reintroducing that
			// field would pass every other assertion in this table.
			if p.LegacyMain {
				t.Errorf("template %s declares legacy_main: true; a reusable template must declare no dispatch target", tt.filename)
			}
			if p.Feature != "" || p.ParentRef != "" || p.BaseSHA != "" || p.ExpectedParentSHA != "" {
				t.Errorf("template %s declares feature-target fields (feature=%q parent_ref=%q base_sha=%q expected_parent_sha=%q); a reusable template must declare no dispatch target",
					tt.filename, p.Feature, p.ParentRef, p.BaseSHA, p.ExpectedParentSHA)
			}

			if strings.Contains(tt.filename, "lens") {
				lensPackets = append(lensPackets, p)
			}
		})
	}

	if len(lensPackets) == 3 {
		if err := packet.DisjointAllowedPaths(lensPackets); err != nil {
			t.Errorf("DisjointAllowedPaths(spec lenses) error = %v", err)
		}
	}
}

func TestTasksPacketTemplatesContract(t *testing.T) {
	assetsDir := filepath.Join("..", "..", "plugin", "claude-code", "skills", "lucind-ai", "assets")
	templates := []struct {
		filename      string
		wantID        string
		wantExecutor  string
		wantPaths     []string
		wantStrings   []string
		forbidStrings []string
	}{
		{
			filename:     "tasks-lens-a-packet-template.md",
			wantID:       "tasks-<change-id>-lens-a",
			wantExecutor: "agy",
			wantPaths:    []string{"openspec/changes/<change-id>/tasks-lens-a.md"},
			wantStrings: []string{
				"decomposition and ordering",
				"~/.claude/skills/sdd-tasks/SKILL.md",
				"Tasks Lens A — Decomposition & Ordering",
				"Lens B owns",
				"Lens C owns",
				"1000 words",
			},
		},
		{
			filename:     "tasks-lens-b-packet-template.md",
			wantID:       "tasks-<change-id>-lens-b",
			wantExecutor: "agy",
			wantPaths:    []string{"openspec/changes/<change-id>/tasks-lens-b.md"},
			wantStrings: []string{
				"partition and dispatch-shape",
				"~/.claude/skills/sdd-tasks/SKILL.md",
				"Tasks Lens B — Partition & Dispatch Shape",
				"Lens A owns",
				"Lens C owns",
				"1000 words",
				// The two repository-specific traps this lens exists to catch.
				// Without both citations the lane has no reason to hand-check a
				// partition, and an unviable wave plan reaches apply, where
				// Integrate reverts it.
				"internal/run/integrate.go:50-59",
				"internal/packet/disjoint.go",
			},
		},
		{
			filename:     "tasks-lens-c-packet-template.md",
			wantID:       "tasks-<change-id>-lens-c",
			wantExecutor: "agy",
			wantPaths:    []string{"openspec/changes/<change-id>/tasks-lens-c.md"},
			wantStrings: []string{
				"proof and review-burden",
				"~/.claude/skills/sdd-tasks/SKILL.md",
				"Tasks Lens C — Proof & Review Burden",
				"Lens A owns",
				"Lens B owns",
				"1000 words",
				"## Review Workload Forecast",
			},
		},
		{
			filename:     "tasks-synthesis-packet-template.md",
			wantID:       "tasks-<change-id>-synthesis",
			wantExecutor: "cursor-agent",
			wantPaths: []string{
				"openspec/changes/<change-id>/tasks.md",
				"openspec/changes/<change-id>/tasks-synthesis-notes.md",
			},
			wantStrings: []string{
				"tasks-lens-a.md",
				"tasks-lens-b.md",
				"tasks-lens-c.md",
				"tasks-synthesis-notes.md",
				"## Unresolved Contradictions",
				"## Coverage Gaps",
				"## Dropped Citations",
				"## Decomposition Divergence",
				"1800 words",
				// The synthesizer must re-derive wave viability instead of
				// trusting lens B's column; these are the citations that make
				// that check performable rather than rhetorical.
				"internal/run/integrate.go:50-59",
				"internal/packet/disjoint.go",
			},
			forbidStrings: []string{
				"## Architecture Divergence",
			},
		},
	}

	var lensPackets []packet.Packet
	for _, tt := range templates {
		t.Run(tt.filename, func(t *testing.T) {
			path := filepath.Join(assetsDir, tt.filename)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%s) error = %v", path, err)
			}
			content := string(data)

			p, err := packet.Parse(strings.NewReader(content))
			if err != nil {
				t.Fatalf("packet.Parse(%s) error = %v", tt.filename, err)
			}

			if p.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", p.ID, tt.wantID)
			}
			if p.Executor != tt.wantExecutor {
				t.Errorf("Executor = %q, want %q", p.Executor, tt.wantExecutor)
			}
			if p.RoutedBy == "" {
				t.Errorf("RoutedBy is empty")
			}
			if !slices.Equal(p.AllowedPaths, tt.wantPaths) {
				t.Errorf("AllowedPaths = %v, want %v", p.AllowedPaths, tt.wantPaths)
			}
			for _, ws := range tt.wantStrings {
				if !strings.Contains(content, ws) {
					t.Errorf("template %s missing expected string %q", tt.filename, ws)
				}
			}
			for _, fs := range tt.forbidStrings {
				if strings.Contains(content, fs) {
					t.Errorf("template %s contains forbidden string %q", tt.filename, fs)
				}
			}

			// The amended Planning Fan-Out Template Assets requirement makes
			// "declares no dispatch target" the default for a reusable
			// template, and this is the assertion that defends it. A template
			// that bakes legacy_main: true silently targets main even when the
			// change runs against a named feature parent -- the exact coupling
			// the amendment removed. Without this check, reintroducing that
			// field would pass every other assertion in this table.
			if p.LegacyMain {
				t.Errorf("template %s declares legacy_main: true; a reusable template must declare no dispatch target", tt.filename)
			}
			if p.Feature != "" || p.ParentRef != "" || p.BaseSHA != "" || p.ExpectedParentSHA != "" {
				t.Errorf("template %s declares feature-target fields (feature=%q parent_ref=%q base_sha=%q expected_parent_sha=%q); a reusable template must declare no dispatch target",
					tt.filename, p.Feature, p.ParentRef, p.BaseSHA, p.ExpectedParentSHA)
			}

			if strings.Contains(tt.filename, "lens") {
				lensPackets = append(lensPackets, p)
			}
		})
	}

	if len(lensPackets) == 3 {
		if err := packet.DisjointAllowedPaths(lensPackets); err != nil {
			t.Errorf("DisjointAllowedPaths(tasks lenses) error = %v", err)
		}
	}
}
