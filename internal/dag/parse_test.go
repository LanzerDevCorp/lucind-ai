package dag_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/dag"
)

func TestParse_ValidSidecar(t *testing.T) {
	dir := t.TempDir()
	bodiesDir := filepath.Join(dir, "bodies")
	if err := os.MkdirAll(bodiesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(bodiesDir, "apply-ledger.md"), []byte("# Goal\nLedger work"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bodiesDir, "apply-serve.md"), []byte("# Goal\nServe work"), 0o644); err != nil {
		t.Fatal(err)
	}

	yamlContent := `change: apply-dag-dispatch
packets:
  - id: apply-ledger
    executor: agy
    routed_by: schema and CRUD isolated from HTTP
    model: gemini-3.7-flash-high
    allowed_paths:
      - internal/ledger/
    depends_on: []
    body_path: bodies/apply-ledger.md
  - id: apply-serve
    executor: agy
    routed_by: HTTP isolated after ledger exists
    allowed_paths:
      - internal/serve/
      - cmd/lucind-ai/cli.go
    depends_on:
      - apply-ledger
    body_path: bodies/apply-serve.md
`
	dagPath := filepath.Join(dir, "apply-dag.yaml")
	if err := os.WriteFile(dagPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := dag.Parse(dagPath)
	if err != nil {
		t.Fatalf("unexpected error parsing valid sidecar: %v", err)
	}

	if d.Change != "apply-dag-dispatch" {
		t.Errorf("expected change %q, got %q", "apply-dag-dispatch", d.Change)
	}
	if len(d.Packets) != 2 {
		t.Fatalf("expected 2 packets, got %d", len(d.Packets))
	}

	p0 := d.Packets[0]
	if p0.ID != "apply-ledger" || p0.Executor != "agy" || p0.RoutedBy != "schema and CRUD isolated from HTTP" ||
		p0.Model != "gemini-3.7-flash-high" || p0.BodyPath != "bodies/apply-ledger.md" {
		t.Errorf("packet 0 fields mismatch: %+v", p0)
	}
	if len(p0.AllowedPaths) != 1 || p0.AllowedPaths[0] != "internal/ledger/" {
		t.Errorf("packet 0 allowed_paths mismatch: %v", p0.AllowedPaths)
	}
	if len(p0.DependsOn) != 0 {
		t.Errorf("packet 0 depends_on mismatch: %v", p0.DependsOn)
	}

	p1 := d.Packets[1]
	if p1.ID != "apply-serve" || p1.Executor != "agy" || p1.RoutedBy != "HTTP isolated after ledger exists" ||
		p1.Model != "" || p1.BodyPath != "bodies/apply-serve.md" {
		t.Errorf("packet 1 fields mismatch: %+v", p1)
	}
	if len(p1.AllowedPaths) != 2 || p1.AllowedPaths[0] != "internal/serve/" || p1.AllowedPaths[1] != "cmd/lucind-ai/cli.go" {
		t.Errorf("packet 1 allowed_paths mismatch: %v", p1)
	}
	if len(p1.DependsOn) != 1 || p1.DependsOn[0] != "apply-ledger" {
		t.Errorf("packet 1 depends_on mismatch: %v", p1)
	}
}

func TestParse_AgentFieldRoundTrips(t *testing.T) {
	dir := t.TempDir()
	bodiesDir := filepath.Join(dir, "bodies")
	if err := os.MkdirAll(bodiesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bodiesDir, "author-dag.md"), []byte("# Goal\nAuthor the DAG"), 0o644); err != nil {
		t.Fatal(err)
	}

	yamlContent := `change: apply-dag-dispatch
packets:
  - id: author-dag
    executor: opencode
    routed_by: DAG authoring, specialist agent required
    agent: lucind-dag
    body_path: bodies/author-dag.md
`
	dagPath := filepath.Join(dir, "apply-dag.yaml")
	if err := os.WriteFile(dagPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := dag.Parse(dagPath)
	if err != nil {
		t.Fatalf("unexpected error parsing sidecar: %v", err)
	}
	if len(d.Packets) != 1 {
		t.Fatalf("expected 1 packet, got %d", len(d.Packets))
	}
	if got := d.Packets[0].Agent; got != "lucind-dag" {
		t.Errorf("Agent = %q, want %q", got, "lucind-dag")
	}
}

func TestParse_AgentFieldAbsentLeavesFieldEmpty(t *testing.T) {
	dir := t.TempDir()
	bodiesDir := filepath.Join(dir, "bodies")
	if err := os.MkdirAll(bodiesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bodiesDir, "apply-ledger.md"), []byte("# Goal\nLedger work"), 0o644); err != nil {
		t.Fatal(err)
	}

	yamlContent := `change: apply-dag-dispatch
packets:
  - id: apply-ledger
    executor: agy
    routed_by: schema and CRUD isolated from HTTP
    body_path: bodies/apply-ledger.md
`
	dagPath := filepath.Join(dir, "apply-dag.yaml")
	if err := os.WriteFile(dagPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := dag.Parse(dagPath)
	if err != nil {
		t.Fatalf("unexpected error parsing sidecar: %v", err)
	}
	if got := d.Packets[0].Agent; got != "" {
		t.Errorf("Agent = %q, want empty", got)
	}
}

func TestParse_ReadOnlyFieldRoundTrips(t *testing.T) {
	dir := t.TempDir()
	bodiesDir := filepath.Join(dir, "bodies")
	if err := os.MkdirAll(bodiesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bodiesDir, "apply-impl.md"), []byte("# Goal\nImpl work"), 0o644); err != nil {
		t.Fatal(err)
	}

	yamlContent := `change: apply-dag-dispatch
packets:
  - id: apply-impl
    executor: agy
    routed_by: implementation depends on failing test
    allowed_paths:
      - impl/foo.go
    read_only:
      - path/a.go
    depends_on: []
    body_path: bodies/apply-impl.md
`
	dagPath := filepath.Join(dir, "apply-dag.yaml")
	if err := os.WriteFile(dagPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := dag.Parse(dagPath)
	if err != nil {
		t.Fatalf("unexpected error parsing sidecar: %v", err)
	}
	if len(d.Packets) != 1 {
		t.Fatalf("expected 1 packet, got %d", len(d.Packets))
	}
	if got := d.Packets[0].ReadOnly; len(got) != 1 || got[0] != "path/a.go" {
		t.Errorf("ReadOnly = %v, want [path/a.go]", got)
	}
}

func TestParse_MissingBodyPathFile(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `change: test-change
packets:
  - id: p1
    executor: agy
    routed_by: test
    allowed_paths:
      - internal/test/
    depends_on: []
    body_path: bodies/nonexistent.md
`
	dagPath := filepath.Join(dir, "apply-dag.yaml")
	if err := os.WriteFile(dagPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := dag.Parse(dagPath)
	if err == nil {
		t.Fatal("expected error for missing body_path file, got nil")
	}
}

func TestParse_IgnoresSiblingTasksMd(t *testing.T) {
	dir := t.TempDir()
	bodiesDir := filepath.Join(dir, "bodies")
	if err := os.MkdirAll(bodiesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bodiesDir, "p1.md"), []byte("# Goal"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a fake tasks.md with contradictory wave prose
	tasksMd := `## Wave 1
- id: fake-task
  depends_on: [something-else]
`
	if err := os.WriteFile(filepath.Join(dir, "tasks.md"), []byte(tasksMd), 0o644); err != nil {
		t.Fatal(err)
	}

	yamlContent := `change: test-change
packets:
  - id: p1
    executor: agy
    routed_by: test
    allowed_paths:
      - internal/test/
    depends_on: []
    body_path: bodies/p1.md
`
	dagPath := filepath.Join(dir, "apply-dag.yaml")
	if err := os.WriteFile(dagPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := dag.Parse(dagPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.Packets) != 1 || d.Packets[0].ID != "p1" {
		t.Errorf("expected 1 packet p1 from YAML, got: %+v", d.Packets)
	}
}

func TestParse_MissingRequiredFields(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "missing change",
			yaml: `packets:
  - id: p1
    executor: agy
    routed_by: test
    allowed_paths: [a]
    depends_on: []
    body_path: bodies/p1.md`,
		},
		{
			name: "missing packets",
			yaml: `change: test`,
		},
		{
			name: "missing packet id",
			yaml: `change: test
packets:
  - executor: agy
    routed_by: test
    allowed_paths: [a]
    depends_on: []
    body_path: bodies/p1.md`,
		},
		{
			name: "missing executor",
			yaml: `change: test
packets:
  - id: p1
    routed_by: test
    allowed_paths: [a]
    depends_on: []
    body_path: bodies/p1.md`,
		},
		{
			name: "missing routed_by",
			yaml: `change: test
packets:
  - id: p1
    executor: agy
    allowed_paths: [a]
    depends_on: []
    body_path: bodies/p1.md`,
		},
		{
			name: "missing body_path",
			yaml: `change: test
packets:
  - id: p1
    executor: agy
    routed_by: test
    allowed_paths: [a]
    depends_on: []`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dagPath := filepath.Join(dir, "apply-dag.yaml")
			if err := os.WriteFile(dagPath, []byte(tt.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := dag.Parse(dagPath)
			if err == nil {
				t.Errorf("expected error for %s, got nil", tt.name)
			}
		})
	}
}

func TestParse_FeatureTargetFields(t *testing.T) {
	dir := t.TempDir()
	bodiesDir := filepath.Join(dir, "bodies")
	if err := os.MkdirAll(bodiesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(bodiesDir, "apply-feat.md"), []byte("# Goal\nFeat work"), 0o644); err != nil {
		t.Fatal(err)
	}

	yamlContent := `change: feature-parent-integration
packets:
  - id: apply-feat
    executor: agy
    routed_by: explicit feature target
    feature: user-auth
    parent_ref: refs/heads/feature/user-auth
    base_sha: 1111111111111111111111111111111111111111
    expected_parent_sha: 2222222222222222222222222222222222222222
    legacy_main: false
    allowed_paths:
      - internal/auth/
    depends_on: []
    body_path: bodies/apply-feat.md
`
	dagPath := filepath.Join(dir, "apply-dag.yaml")
	if err := os.WriteFile(dagPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := dag.Parse(dagPath)
	if err != nil {
		t.Fatalf("unexpected error parsing valid sidecar with target fields: %v", err)
	}

	if len(d.Packets) != 1 {
		t.Fatalf("expected 1 packet, got %d", len(d.Packets))
	}

	p := d.Packets[0]
	if p.Feature != "user-auth" {
		t.Errorf("Feature = %q, want %q", p.Feature, "user-auth")
	}
	if p.ParentRef != "refs/heads/feature/user-auth" {
		t.Errorf("ParentRef = %q, want %q", p.ParentRef, "refs/heads/feature/user-auth")
	}
	if p.BaseSHA != "1111111111111111111111111111111111111111" {
		t.Errorf("BaseSHA = %q, want %q", p.BaseSHA, "1111111111111111111111111111111111111111")
	}
	if p.ExpectedParentSHA != "2222222222222222222222222222222222222222" {
		t.Errorf("ExpectedParentSHA = %q, want %q", p.ExpectedParentSHA, "2222222222222222222222222222222222222222")
	}
	if p.LegacyMain != false {
		t.Errorf("LegacyMain = %v, want false", p.LegacyMain)
	}
}
