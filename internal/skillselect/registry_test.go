package skillselect_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/skillselect"
)

func TestParseRegistry_Fixture(t *testing.T) {
	fixturePath := filepath.Join("testdata", "registry.md")
	f, err := os.Open(fixturePath)
	if err != nil {
		t.Fatalf("failed to open test fixture %s: %v", fixturePath, err)
	}
	defer func() { _ = f.Close() }()

	skills, err := skillselect.ParseRegistry(f)
	if err != nil {
		t.Fatalf("unexpected error parsing registry: %v", err)
	}

	if len(skills) != 4 {
		t.Fatalf("got %d skills, want 4", len(skills))
	}

	// 1st skill: branch-pr
	if skills[0].Name != "branch-pr" {
		t.Errorf("skill[0].Name = %q, want %q", skills[0].Name, "branch-pr")
	}
	if skills[0].Scope != "user" {
		t.Errorf("skill[0].Scope = %q, want %q", skills[0].Scope, "user")
	}
	if skills[0].Path != "/home/lanzerdev/.agents/skills/branch-pr/SKILL.md" {
		t.Errorf("skill[0].Path = %q, want %q", skills[0].Path, "/home/lanzerdev/.agents/skills/branch-pr/SKILL.md")
	}
	if !strings.HasPrefix(skills[0].Description, "Create Gentle AI pull requests") {
		t.Errorf("skill[0].Description = %q, want prefix 'Create Gentle AI pull requests'", skills[0].Description)
	}

	// 3rd skill: golang-cli (contains `cobra` and → in description)
	if skills[2].Name != "golang-cli" {
		t.Errorf("skill[2].Name = %q, want %q", skills[2].Name, "golang-cli")
	}
	if skills[2].Scope != "repo" {
		t.Errorf("skill[2].Scope = %q, want %q", skills[2].Scope, "repo")
	}
	if !strings.Contains(skills[2].Description, "`cobra`") {
		t.Errorf("skill[2].Description should preserve backticks in description: %q", skills[2].Description)
	}
	if !strings.Contains(skills[2].Description, "→") {
		t.Errorf("skill[2].Description should preserve '→' in description: %q", skills[2].Description)
	}
}

func TestParseRegistry_MissingTable(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "empty document",
			content: "",
		},
		{
			name: "no skills heading",
			content: `# Skill Registry
## Sources scanned
- .agents/skills
`,
		},
		{
			name: "skills heading without table",
			content: `# Skill Registry
## Skills
No table here! Just text.
## Next Section
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.content)
			_, err := skillselect.ParseRegistry(r)
			if err == nil {
				t.Fatalf("expected error for missing table, got nil")
			}
		})
	}
}

func TestParseRegistry_EmptyTable(t *testing.T) {
	content := `# Registry
## Skills

| Skill | Trigger / description | Scope | Path |
| --- | --- | --- | --- |

## Next Section
`
	r := strings.NewReader(content)
	skills, err := skillselect.ParseRegistry(r)
	if err != nil {
		t.Fatalf("unexpected error for empty table: %v", err)
	}
	if skills == nil {
		t.Fatalf("expected non-nil empty slice, got nil")
	}
	if len(skills) != 0 {
		t.Fatalf("got %d skills, want 0", len(skills))
	}
}

func TestParseRegistry_MalformedRows(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "too few columns",
			content: `## Skills

| Skill | Trigger / description | Scope | Path |
| --- | --- | --- | --- |
| ` + "`skill-1`" + ` | description only | missing-path |
`,
		},
		{
			name: "too many columns",
			content: `## Skills

| Skill | Trigger / description | Scope | Path |
| --- | --- | --- | --- |
| ` + "`skill-1`" + ` | desc | user | ` + "`/path`" + ` | extra |
`,
		},
		{
			name: "empty skill name",
			content: `## Skills

| Skill | Trigger / description | Scope | Path |
| --- | --- | --- | --- |
| ` + "``" + ` | desc | user | ` + "`/path`" + ` |
`,
		},
		{
			name: "empty path",
			content: `## Skills

| Skill | Trigger / description | Scope | Path |
| --- | --- | --- | --- |
| ` + "`skill-1`" + ` | desc | user | ` + "``" + ` |
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.content)
			_, err := skillselect.ParseRegistry(r)
			if err == nil {
				t.Fatalf("expected error for malformed row, got nil")
			}
		})
	}
}

func TestParseRegistry_BacktickStripping(t *testing.T) {
	content := `## Skills

| Skill | Trigger / description | Scope | Path |
| --- | --- | --- | --- |
| ` + "`my-skill`" + ` | Using ` + "`code`" + ` inside description | repo | ` + "`/abs/path/SKILL.md`" + ` |
`
	r := strings.NewReader(content)
	skills, err := skillselect.ParseRegistry(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].Name != "my-skill" {
		t.Errorf("Name = %q, want %q", skills[0].Name, "my-skill")
	}
	if skills[0].Path != "/abs/path/SKILL.md" {
		t.Errorf("Path = %q, want %q", skills[0].Path, "/abs/path/SKILL.md")
	}
	if !strings.Contains(skills[0].Description, "`code`") {
		t.Errorf("Description should preserve inline backticks: %q", skills[0].Description)
	}
}

func TestResolvePaths(t *testing.T) {
	tmpDir := t.TempDir()

	realFile := filepath.Join(tmpDir, "real_skill.md")
	if err := os.WriteFile(realFile, []byte("# Test Skill"), 0o644); err != nil {
		t.Fatalf("failed to create real file: %v", err)
	}

	canonicalRealFile, err := filepath.EvalSymlinks(realFile)
	if err != nil {
		t.Fatalf("failed to eval symlinks on real file: %v", err)
	}

	symlinkFile := filepath.Join(tmpDir, "symlink_skill.md")
	if err := os.Symlink(realFile, symlinkFile); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	nonexistentFile := filepath.Join(tmpDir, "does_not_exist.md")

	input := []skillselect.Skill{
		{
			Name:        "symlinked-skill",
			Description: "desc 1",
			Scope:       "repo",
			Path:        symlinkFile,
		},
		{
			Name:        "nonexistent-skill",
			Description: "desc 2",
			Scope:       "user",
			Path:        nonexistentFile,
		},
	}

	got := skillselect.ResolvePaths(input)

	if len(got) != 2 {
		t.Fatalf("got %d skills, want 2", len(got))
	}

	// 1. Symlink resolved to real target path
	if got[0].Path != canonicalRealFile {
		t.Errorf("got[0].Path = %q, want canonical %q", got[0].Path, canonicalRealFile)
	}
	if got[0].Name != "symlinked-skill" || got[0].Description != "desc 1" || got[0].Scope != "repo" {
		t.Errorf("got[0] fields altered: %+v", got[0])
	}

	// 2. Nonexistent path unchanged
	if got[1].Path != nonexistentFile {
		t.Errorf("got[1].Path = %q, want %q", got[1].Path, nonexistentFile)
	}
	if got[1].Name != "nonexistent-skill" || got[1].Description != "desc 2" || got[1].Scope != "user" {
		t.Errorf("got[1] fields altered: %+v", got[1])
	}

	// 3. Empty skills slice returns empty slice
	emptyResult := skillselect.ResolvePaths([]skillselect.Skill{})
	if emptyResult == nil {
		t.Errorf("ResolvePaths(empty) = nil, want non-nil empty slice")
	}
	if len(emptyResult) != 0 {
		t.Errorf("len(ResolvePaths(empty)) = %d, want 0", len(emptyResult))
	}
}
