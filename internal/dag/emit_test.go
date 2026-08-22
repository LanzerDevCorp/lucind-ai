package dag_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/dag"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
)

func TestEmit_SuccessfulSplitWritesPackets(t *testing.T) {
	tempDir := t.TempDir()
	bodiesDir := filepath.Join(tempDir, "bodies")
	if err := os.MkdirAll(bodiesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	body1 := "## Goal\n\nLedger CRUD implementation\n\n## Done criteria\n\n- [ ] Ledger works\n"
	body2 := "## Goal\n\nHTTP server implementation\n\n## Done criteria\n\n- [ ] Server works\n"

	if err := os.WriteFile(filepath.Join(bodiesDir, "apply-ledger.md"), []byte(body1), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bodiesDir, "apply-serve.md"), []byte(body2), 0o644); err != nil {
		t.Fatal(err)
	}

	d := dag.DAG{
		Change: "test-change",
		Packets: []dag.Node{
			{
				ID:           "apply-ledger",
				Executor:     "agy",
				RoutedBy:     "schema and CRUD isolated from HTTP",
				Model:        "gemini-3.7-flash-high",
				AllowedPaths: []string{"internal/ledger/"},
				DependsOn:    []string{},
				BodyPath:     "bodies/apply-ledger.md",
			},
			{
				ID:           "apply-serve",
				Executor:     "cursor-agent",
				RoutedBy:     "HTTP isolated after ledger exists",
				AllowedPaths: []string{"internal/serve/", "cmd/lucind-ai/cli.go"},
				DependsOn:    []string{"apply-ledger"},
				BodyPath:     "bodies/apply-serve.md",
			},
		},
	}

	outDir := filepath.Join(tempDir, "packets")
	if err := dag.Emit(d, tempDir, outDir); err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Verify apply-ledger.md
	ledgerPath := filepath.Join(outDir, "apply-ledger.md")
	ledgerBytes, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("failed to read emitted ledger packet: %v", err)
	}
	ledgerContent := string(ledgerBytes)

	// Check single-line JSON array in frontmatter
	if !strings.Contains(ledgerContent, `allowed_paths: ["internal/ledger/"]`) {
		t.Errorf("expected single-line JSON array for allowed_paths in ledger packet, got:\n%s", ledgerContent)
	}
	if !strings.Contains(ledgerContent, `model: gemini-3.7-flash-high`) {
		t.Errorf("expected model in ledger packet frontmatter, got:\n%s", ledgerContent)
	}

	// Round-trip proof via packet.Parse
	pLedger, err := packet.Parse(strings.NewReader(ledgerContent))
	if err != nil {
		t.Fatalf("packet.Parse failed for emitted ledger: %v", err)
	}
	if pLedger.ID != "apply-ledger" || pLedger.Executor != "agy" || pLedger.RoutedBy != "schema and CRUD isolated from HTTP" || pLedger.Model != "gemini-3.7-flash-high" {
		t.Errorf("pLedger field mismatch: %+v", pLedger)
	}
	if !slices.Equal(pLedger.AllowedPaths, []string{"internal/ledger/"}) {
		t.Errorf("pLedger allowed_paths mismatch: %v", pLedger.AllowedPaths)
	}
	if pLedger.Body != body1 {
		t.Errorf("pLedger body mismatch: got %q, want %q", pLedger.Body, body1)
	}

	// Verify apply-serve.md
	servePath := filepath.Join(outDir, "apply-serve.md")
	serveBytes, err := os.ReadFile(servePath)
	if err != nil {
		t.Fatalf("failed to read emitted serve packet: %v", err)
	}
	serveContent := string(serveBytes)

	if !strings.Contains(serveContent, `allowed_paths: ["internal/serve/","cmd/lucind-ai/cli.go"]`) &&
		!strings.Contains(serveContent, `allowed_paths: ["internal/serve/", "cmd/lucind-ai/cli.go"]`) {
		t.Errorf("expected single-line JSON array for allowed_paths in serve packet, got:\n%s", serveContent)
	}
	if strings.Contains(serveContent, "model:") {
		t.Errorf("expected no model in serve packet when omitted, got:\n%s", serveContent)
	}

	pServe, err := packet.Parse(strings.NewReader(serveContent))
	if err != nil {
		t.Fatalf("packet.Parse failed for emitted serve: %v", err)
	}
	if pServe.ID != "apply-serve" || pServe.Executor != "cursor-agent" || pServe.RoutedBy != "HTTP isolated after ledger exists" || pServe.Model != "" {
		t.Errorf("pServe field mismatch: %+v", pServe)
	}
	if !slices.Equal(pServe.AllowedPaths, []string{"internal/serve/", "cmd/lucind-ai/cli.go"}) {
		t.Errorf("pServe allowed_paths mismatch: %v", pServe.AllowedPaths)
	}
	if pServe.Body != body2 {
		t.Errorf("pServe body mismatch: got %q, want %q", pServe.Body, body2)
	}
}

func TestEmit_AgentFieldEmittedWhenSet(t *testing.T) {
	tempDir := t.TempDir()
	bodiesDir := filepath.Join(tempDir, "bodies")
	if err := os.MkdirAll(bodiesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "## Goal\n\nAuthor the DAG\n"
	if err := os.WriteFile(filepath.Join(bodiesDir, "author-dag.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	d := dag.DAG{
		Change: "test-change",
		Packets: []dag.Node{
			{
				ID:       "author-dag",
				Executor: "opencode",
				RoutedBy: "DAG authoring, specialist agent required",
				Agent:    "lucind-dag",
				BodyPath: "bodies/author-dag.md",
			},
		},
	}

	outDir := filepath.Join(tempDir, "packets")
	if err := dag.Emit(d, tempDir, outDir); err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(outDir, "author-dag.md"))
	if err != nil {
		t.Fatalf("failed to read emitted packet: %v", err)
	}
	if !strings.Contains(string(content), "agent: lucind-dag") {
		t.Errorf("expected agent in emitted packet frontmatter, got:\n%s", content)
	}

	p, err := packet.Parse(strings.NewReader(string(content)))
	if err != nil {
		t.Fatalf("packet.Parse failed for emitted packet: %v", err)
	}
	if p.Agent != "lucind-dag" {
		t.Errorf("Agent = %q, want %q", p.Agent, "lucind-dag")
	}
}

func TestEmit_AgentFieldOmittedWhenEmpty(t *testing.T) {
	tempDir := t.TempDir()
	bodiesDir := filepath.Join(tempDir, "bodies")
	if err := os.MkdirAll(bodiesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "## Goal\n\nLedger CRUD implementation\n"
	if err := os.WriteFile(filepath.Join(bodiesDir, "apply-ledger.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	d := dag.DAG{
		Change: "test-change",
		Packets: []dag.Node{
			{
				ID:       "apply-ledger",
				Executor: "agy",
				RoutedBy: "schema and CRUD isolated from HTTP",
				BodyPath: "bodies/apply-ledger.md",
			},
		},
	}

	outDir := filepath.Join(tempDir, "packets")
	if err := dag.Emit(d, tempDir, outDir); err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(outDir, "apply-ledger.md"))
	if err != nil {
		t.Fatalf("failed to read emitted packet: %v", err)
	}
	if strings.Contains(string(content), "agent:") {
		t.Errorf("expected no agent line in emitted packet when omitted, got:\n%s", content)
	}
}

func TestEmit_ReadOnlyEmittedWhenSet(t *testing.T) {
	tempDir := t.TempDir()
	bodiesDir := filepath.Join(tempDir, "bodies")
	if err := os.MkdirAll(bodiesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "## Goal\n\nImplement to make test pass\n"
	if err := os.WriteFile(filepath.Join(bodiesDir, "apply-green.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	node := dag.Node{
		ID:           "apply-green",
		Executor:     "agy",
		RoutedBy:     "implementation depends on failing test",
		AllowedPaths: []string{"impl/foo.go"},
		ReadOnly:     []string{"test/foo_test.go"},
		BodyPath:     "bodies/apply-green.md",
	}

	content, err := dag.EmitPacketContent(node, tempDir)
	if err != nil {
		t.Fatalf("EmitPacketContent failed: %v", err)
	}

	if !strings.Contains(content, `read_only_paths: ["test/foo_test.go"]`) {
		t.Errorf("expected single-line JSON array for read_only_paths in frontmatter, got:\n%s", content)
	}
	if strings.Contains(content, "\nread_only:") {
		t.Errorf("must never emit under the pre-existing packet-level boolean key \"read_only\", got:\n%s", content)
	}
}

func TestEmit_ReadOnlyOmittedWhenEmpty(t *testing.T) {
	tempDir := t.TempDir()
	bodiesDir := filepath.Join(tempDir, "bodies")
	if err := os.MkdirAll(bodiesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "## Goal\n\nLedger CRUD implementation\n"
	if err := os.WriteFile(filepath.Join(bodiesDir, "apply-ledger.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	node := dag.Node{
		ID:           "apply-ledger",
		Executor:     "agy",
		RoutedBy:     "schema and CRUD isolated from HTTP",
		AllowedPaths: []string{"internal/ledger/"},
		BodyPath:     "bodies/apply-ledger.md",
	}

	content, err := dag.EmitPacketContent(node, tempDir)
	if err != nil {
		t.Fatalf("EmitPacketContent failed: %v", err)
	}

	if strings.Contains(content, "read_only:") {
		t.Errorf("expected no read_only line in emitted packet when omitted, got:\n%s", content)
	}
}

func TestEmit_FeatureTargetFieldsRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	bodiesDir := filepath.Join(tempDir, "bodies")
	if err := os.MkdirAll(bodiesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	body := "## Goal\n\nFeature auth implementation\n"
	if err := os.WriteFile(filepath.Join(bodiesDir, "apply-auth.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	node := dag.Node{
		ID:                "apply-auth",
		Executor:          "agy",
		RoutedBy:          "touches auth, Tier A verification required",
		Model:             "gemini-3.7-flash-high",
		Feature:           "user-auth",
		ParentRef:         "refs/heads/feature/user-auth",
		BaseSHA:           "1111111111111111111111111111111111111111",
		ExpectedParentSHA: "2222222222222222222222222222222222222222",
		LegacyMain:        false,
		AllowedPaths:      []string{"internal/auth/"},
		DependsOn:         []string{},
		BodyPath:          "bodies/apply-auth.md",
	}

	content, err := dag.EmitPacketContent(node, tempDir)
	if err != nil {
		t.Fatalf("EmitPacketContent failed: %v", err)
	}

	// Verify frontmatter strings
	if !strings.Contains(content, "feature: user-auth\n") {
		t.Errorf("content missing 'feature: user-auth', got:\n%s", content)
	}
	if !strings.Contains(content, "parent_ref: refs/heads/feature/user-auth\n") {
		t.Errorf("content missing 'parent_ref: refs/heads/feature/user-auth', got:\n%s", content)
	}
	if !strings.Contains(content, "base_sha: 1111111111111111111111111111111111111111\n") {
		t.Errorf("content missing base_sha, got:\n%s", content)
	}
	if !strings.Contains(content, "expected_parent_sha: 2222222222222222222222222222222222222222\n") {
		t.Errorf("content missing expected_parent_sha, got:\n%s", content)
	}

	// Round-trip proof through packet.Parse
	p, err := packet.Parse(strings.NewReader(content))
	if err != nil {
		t.Fatalf("packet.Parse failed on emitted content: %v", err)
	}
	if p.ID != node.ID || p.Feature != node.Feature || p.ParentRef != node.ParentRef ||
		p.BaseSHA != node.BaseSHA || p.ExpectedParentSHA != node.ExpectedParentSHA || p.LegacyMain != node.LegacyMain {
		t.Errorf("round-trip packet mismatch: got %+v, want %+v", p, node)
	}
}
