package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	t.Run("valid multi-section", func(t *testing.T) {
		src := []byte("<!-- lucind:rules audience=all -->\n# All\nSome content for all\n<!-- lucind:rules audience=orchestrator -->\n# Orchestrator\nOrchestrator instructions\n<!-- lucind:rules audience=worker -->\n# Worker\nWorker rules\n")
		sections, err := Parse(src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sections) != 3 {
			t.Fatalf("expected 3 sections, got %d", len(sections))
		}
		if len(sections[0].Audiences) != 1 || sections[0].Audiences[0] != "all" {
			t.Errorf("expected audience [all], got %v", sections[0].Audiences)
		}
		if !strings.Contains(sections[0].Content, "Some content for all") {
			t.Errorf("unexpected content in section 0: %q", sections[0].Content)
		}
		if len(sections[1].Audiences) != 1 || sections[1].Audiences[0] != "orchestrator" {
			t.Errorf("expected audience [orchestrator], got %v", sections[1].Audiences)
		}
		if len(sections[2].Audiences) != 1 || sections[2].Audiences[0] != "worker" {
			t.Errorf("expected audience [worker], got %v", sections[2].Audiences)
		}
	})

	t.Run("multi-audience marker", func(t *testing.T) {
		src := []byte("<!-- lucind:rules audience=worker,orchestrator -->\nShared content\n")
		sections, err := Parse(src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sections) != 1 {
			t.Fatalf("expected 1 section, got %d", len(sections))
		}
		if len(sections[0].Audiences) != 2 || sections[0].Audiences[0] != "worker" || sections[0].Audiences[1] != "orchestrator" {
			t.Errorf("expected audiences [worker, orchestrator], got %v", sections[0].Audiences)
		}
	})

	t.Run("text before first marker error with line", func(t *testing.T) {
		src := []byte("Leading text without marker\n<!-- lucind:rules audience=all -->\nContent\n")
		_, err := Parse(src)
		if err == nil {
			t.Fatal("expected error for text before first marker, got nil")
		}
		if !strings.Contains(err.Error(), "line 1") {
			t.Errorf("expected error to mention line 1, got %v", err)
		}
	})

	t.Run("unknown audience", func(t *testing.T) {
		src := []byte("<!-- lucind:rules audience=invalid_audience -->\nContent\n")
		_, err := Parse(src)
		if err == nil {
			t.Fatal("expected error for unknown audience, got nil")
		}
		if !strings.Contains(err.Error(), "line 1") {
			t.Errorf("expected error to mention line 1, got %v", err)
		}
	})

	t.Run("empty list", func(t *testing.T) {
		src := []byte("<!-- lucind:rules audience= -->\nContent\n")
		_, err := Parse(src)
		if err == nil {
			t.Fatal("expected error for empty audience list, got nil")
		}
		if !strings.Contains(err.Error(), "line 1") {
			t.Errorf("expected error to mention line 1, got %v", err)
		}
	})

	t.Run("duplicate audience", func(t *testing.T) {
		src := []byte("<!-- lucind:rules audience=worker,worker -->\nContent\n")
		_, err := Parse(src)
		if err == nil {
			t.Fatal("expected error for duplicate audience, got nil")
		}
		if !strings.Contains(err.Error(), "line 1") {
			t.Errorf("expected error to mention line 1, got %v", err)
		}
	})

	t.Run("audience list with spaces", func(t *testing.T) {
		src := []byte("<!-- lucind:rules audience=worker, orchestrator -->\nContent\n")
		_, err := Parse(src)
		if err == nil {
			t.Fatal("expected error for audience list with spaces, got nil")
		}
		if !strings.Contains(err.Error(), "line 1") {
			t.Errorf("expected error to mention line 1, got %v", err)
		}
	})

	t.Run("trailing comma in audience list", func(t *testing.T) {
		src := []byte("<!-- lucind:rules audience=worker, -->\nContent\n")
		_, err := Parse(src)
		if err == nil {
			t.Fatal("expected error for trailing comma in audience list, got nil")
		}
		if !strings.Contains(err.Error(), "line 1") {
			t.Errorf("expected error to mention line 1, got %v", err)
		}
	})

	t.Run("crlf line endings", func(t *testing.T) {
		src := []byte("<!-- lucind:rules audience=all -->\r\nLine 1\r\nLine 2\r\n")
		sections, err := Parse(src)
		if err != nil {
			t.Fatalf("unexpected error with CRLF: %v", err)
		}
		if len(sections) != 1 {
			t.Fatalf("expected 1 section, got %d", len(sections))
		}
		if !strings.Contains(sections[0].Content, "Line 1\nLine 2") {
			t.Errorf("expected content to contain Line 1 and Line 2, got %q", sections[0].Content)
		}
	})

	t.Run("no sections", func(t *testing.T) {
		src := []byte("")
		_, err := Parse(src)
		if err == nil {
			t.Fatal("expected error for empty source with no sections, got nil")
		}
		if !strings.Contains(err.Error(), "line 1") {
			t.Errorf("expected error to mention line 1, got %v", err)
		}
	})
}

func TestRender(t *testing.T) {
	sections := []Section{
		{
			Audiences: []string{"all"},
			Content:   "Shared content for everyone.\n",
		},
		{
			Audiences: []string{"orchestrator"},
			Content:   "Orchestrator specific rules.\n",
		},
		{
			Audiences: []string{"worker"},
			Content:   "Worker specific rules.\n",
		},
	}

	src := []byte("test-source")
	h := sha256.Sum256(src)
	srcHash := hex.EncodeToString(h[:])

	rendered := Render(sections, srcHash)
	if rendered == nil {
		t.Fatal("expected rendered map, got nil")
	}

	for _, name := range []string{"CLAUDE.md", "GEMINI.md", "AGENTS.md"} {
		content, ok := rendered[name]
		if !ok {
			t.Fatalf("missing output for %s", name)
		}
		str := string(content)
		// Check generated marker is first line with expected sha
		markerPrefix := "<!-- GENERATED by lucind-ai rules generate from lucind-rules.md (sha256:" + srcHash[:12] + "); do not edit, change the source and regenerate -->"
		if !strings.HasPrefix(str, markerPrefix) {
			t.Errorf("%s: expected generated marker at start, got:\n%s", name, str)
		}
	}

	// Check titles
	claudeStr := string(rendered["CLAUDE.md"])
	geminiStr := string(rendered["GEMINI.md"])
	agentsStr := string(rendered["AGENTS.md"])

	if !strings.Contains(claudeStr, "# Agent rules: orchestrator") {
		t.Errorf("CLAUDE.md missing title, got:\n%s", claudeStr)
	}
	if !strings.Contains(geminiStr, "# Agent rules: worker") {
		t.Errorf("GEMINI.md missing title, got:\n%s", geminiStr)
	}
	if !strings.Contains(agentsStr, "# Agent rules: implementers") {
		t.Errorf("AGENTS.md missing title, got:\n%s", agentsStr)
	}

	// Check audience routing
	if !strings.Contains(claudeStr, "Shared content for everyone.") {
		t.Errorf("CLAUDE.md missing 'all' section")
	}
	if !strings.Contains(claudeStr, "Orchestrator specific rules.") {
		t.Errorf("CLAUDE.md missing 'orchestrator' section")
	}
	if strings.Contains(claudeStr, "Worker specific rules.") {
		t.Errorf("CLAUDE.md should NOT contain worker section")
	}

	if !strings.Contains(geminiStr, "Shared content for everyone.") {
		t.Errorf("GEMINI.md missing 'all' section")
	}
	if !strings.Contains(geminiStr, "Worker specific rules.") {
		t.Errorf("GEMINI.md missing 'worker' section")
	}
	if strings.Contains(geminiStr, "Orchestrator specific rules.") {
		t.Errorf("GEMINI.md should NOT contain orchestrator section")
	}

	if !strings.Contains(agentsStr, "Shared content for everyone.") {
		t.Errorf("AGENTS.md missing 'all' section")
	}
	if !strings.Contains(agentsStr, "Worker specific rules.") {
		t.Errorf("AGENTS.md missing 'worker' section")
	}
	if strings.Contains(agentsStr, "Orchestrator specific rules.") {
		t.Errorf("AGENTS.md should NOT contain orchestrator section")
	}

	// Deterministic output
	rendered2 := Render(sections, srcHash)
	for k, v := range rendered {
		if string(v) != string(rendered2[k]) {
			t.Errorf("non-deterministic output for %s", k)
		}
	}

	// Different hash gives different marker
	renderedDiff := Render(sections, "000000000000")
	if string(rendered["CLAUDE.md"]) == string(renderedDiff["CLAUDE.md"]) {
		t.Errorf("expected different hash to produce different output")
	}
}

func TestDefaultSource(t *testing.T) {
	src := DefaultSource()
	if len(src) == 0 {
		t.Fatal("expected non-empty default source")
	}

	sections, err := Parse(src)
	if err != nil {
		t.Fatalf("default source failed to parse: %v", err)
	}

	h := sha256.Sum256(src)
	rendered := Render(sections, hex.EncodeToString(h[:]))

	claude := string(rendered["CLAUDE.md"])
	gemini := string(rendered["GEMINI.md"])
	agents := string(rendered["AGENTS.md"])

	exactSentence := "Delegation goes through the dispatcher: run `lucind-ai run --packet <file>`; never start ad-hoc agent processes for implementation work."
	if !strings.Contains(claude, exactSentence) {
		t.Errorf("CLAUDE.md missing exact delegation sentence:\n%s", exactSentence)
	}

	if strings.Contains(gemini, exactSentence) {
		t.Errorf("GEMINI.md must not contain orchestrator delegation sentence")
	}
	if strings.Contains(agents, exactSentence) {
		t.Errorf("AGENTS.md must not contain orchestrator delegation sentence")
	}

	// Worker rules in GEMINI and AGENTS
	if !strings.Contains(gemini, "allowed edit surfaces") {
		t.Errorf("GEMINI.md missing worker rules")
	}
	if !strings.Contains(agents, "allowed edit surfaces") {
		t.Errorf("AGENTS.md missing worker rules")
	}
	if strings.Contains(claude, "## Key Learnings") {
		t.Errorf("CLAUDE.md should not contain worker section")
	}
}

func TestGenerate(t *testing.T) {
	validSource := []byte("<!-- lucind:rules audience=all -->\n# All\nSome content\n<!-- lucind:rules audience=orchestrator -->\n# Orch\nDelegation goes through the dispatcher: run `lucind-ai run --packet <file>`; never start ad-hoc agent processes for implementation work.\n<!-- lucind:rules audience=worker -->\n# Work\nStay inside allowed edit surfaces.\n")

	t.Run("generate writes files, second run unchanged, edit source rewrites", func(t *testing.T) {
		root := t.TempDir()
		srcPath := filepath.Join(root, SourceName)
		if err := os.WriteFile(srcPath, validSource, 0644); err != nil {
			t.Fatal(err)
		}

		res, err := Generate(root, Options{})
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}

		for _, name := range []string{"CLAUDE.md", "GEMINI.md", "AGENTS.md"} {
			if res.Files[name] != StatusWritten {
				t.Errorf("expected %s to be written, got %s", name, res.Files[name])
			}
			p := filepath.Join(root, name)
			if _, err := os.Stat(p); err != nil {
				t.Errorf("file %s not created: %v", p, err)
			}
		}

		// Second run => unchanged
		res2, err := Generate(root, Options{})
		if err != nil {
			t.Fatalf("Generate second run failed: %v", err)
		}
		for _, name := range []string{"CLAUDE.md", "GEMINI.md", "AGENTS.md"} {
			if res2.Files[name] != StatusUnchanged {
				t.Errorf("expected %s to be unchanged, got %s", name, res2.Files[name])
			}
		}

		// Edit source => rewritten
		updatedSource := append(validSource, []byte("\n# Added line\n")...)
		if err := os.WriteFile(srcPath, updatedSource, 0644); err != nil {
			t.Fatal(err)
		}
		res3, err := Generate(root, Options{})
		if err != nil {
			t.Fatalf("Generate after edit failed: %v", err)
		}
		for _, name := range []string{"CLAUDE.md", "GEMINI.md", "AGENTS.md"} {
			if res3.Files[name] != StatusWritten {
				t.Errorf("expected %s to be written after edit, got %s", name, res3.Files[name])
			}
		}
	})

	t.Run("hand-written CLAUDE.md is skipped and left byte-identical", func(t *testing.T) {
		root := t.TempDir()
		srcPath := filepath.Join(root, SourceName)
		if err := os.WriteFile(srcPath, validSource, 0644); err != nil {
			t.Fatal(err)
		}

		claudePath := filepath.Join(root, "CLAUDE.md")
		handWrittenContent := []byte("# Hand-written Claude rules\nDo not touch this.\n")
		if err := os.WriteFile(claudePath, handWrittenContent, 0644); err != nil {
			t.Fatal(err)
		}

		res, err := Generate(root, Options{})
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}

		if res.Files["CLAUDE.md"] != StatusSkippedHandWritten {
			t.Errorf("expected CLAUDE.md to be skipped_hand_written, got %s", res.Files["CLAUDE.md"])
		}
		got, err := os.ReadFile(claudePath)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(handWrittenContent) {
			t.Errorf("hand-written CLAUDE.md was modified!")
		}
		// GEMINI.md and AGENTS.md should still be written
		if res.Files["GEMINI.md"] != StatusWritten {
			t.Errorf("expected GEMINI.md to be written, got %s", res.Files["GEMINI.md"])
		}
		if res.Files["AGENTS.md"] != StatusWritten {
			t.Errorf("expected AGENTS.md to be written, got %s", res.Files["AGENTS.md"])
		}
	})

	t.Run("destination symlink is not followed and treated as skipped", func(t *testing.T) {
		root := t.TempDir()
		srcPath := filepath.Join(root, SourceName)
		if err := os.WriteFile(srcPath, validSource, 0644); err != nil {
			t.Fatal(err)
		}

		externalFile := filepath.Join(t.TempDir(), "external.md")
		if err := os.WriteFile(externalFile, []byte("external content"), 0644); err != nil {
			t.Fatal(err)
		}

		claudePath := filepath.Join(root, "CLAUDE.md")
		if err := os.Symlink(externalFile, claudePath); err != nil {
			t.Fatal(err)
		}

		res, err := Generate(root, Options{})
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}
		if res.Files["CLAUDE.md"] != StatusSkippedHandWritten {
			t.Errorf("expected symlink target to be skipped_hand_written, got %s", res.Files["CLAUDE.md"])
		}
		// Check external file was not modified
		extGot, err := os.ReadFile(externalFile)
		if err != nil {
			t.Fatal(err)
		}
		if string(extGot) != "external content" {
			t.Errorf("external file through symlink was modified!")
		}
	})

	t.Run("check mode reports stale and missing and writes nothing", func(t *testing.T) {
		root := t.TempDir()
		srcPath := filepath.Join(root, SourceName)
		if err := os.WriteFile(srcPath, validSource, 0644); err != nil {
			t.Fatal(err)
		}

		// Initial check: all missing
		res, err := Generate(root, Options{Check: true})
		if err != nil {
			t.Fatalf("Generate check failed: %v", err)
		}
		for _, name := range []string{"CLAUDE.md", "GEMINI.md", "AGENTS.md"} {
			if res.Files[name] != StatusMissing {
				t.Errorf("expected %s to be missing in check mode, got %s", name, res.Files[name])
			}
			if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
				t.Errorf("file %s should not have been created in check mode", name)
			}
		}

		// Generate normally
		if _, err := Generate(root, Options{}); err != nil {
			t.Fatal(err)
		}

		// Edit source
		if err := os.WriteFile(srcPath, append(validSource, []byte("\n# change\n")...), 0644); err != nil {
			t.Fatal(err)
		}

		// Check mode: should report stale
		res2, err := Generate(root, Options{Check: true})
		if err != nil {
			t.Fatalf("Generate check failed: %v", err)
		}
		for _, name := range []string{"CLAUDE.md", "GEMINI.md", "AGENTS.md"} {
			if res2.Files[name] != StatusStale {
				t.Errorf("expected %s to be stale in check mode, got %s", name, res2.Files[name])
			}
		}
	})

	t.Run("missing source errors and suggests init", func(t *testing.T) {
		root := t.TempDir()
		_, err := Generate(root, Options{})
		if err == nil {
			t.Fatal("expected error for missing source, got nil")
		}
		if !strings.Contains(err.Error(), SourceName) || !strings.Contains(err.Error(), "lucind-ai rules init") {
			t.Errorf("expected error mentioning %s and 'lucind-ai rules init', got %v", SourceName, err)
		}
	})

	t.Run("forbidden roots refused", func(t *testing.T) {
		tempHome := t.TempDir()
		t.Setenv("HOME", tempHome)

		claudeDir := filepath.Join(tempHome, ".claude")
		geminiDir := filepath.Join(tempHome, ".gemini")
		if err := os.MkdirAll(claudeDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(geminiDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Inside .claude
		insideClaude := filepath.Join(claudeDir, "subdir")
		if err := os.MkdirAll(insideClaude, 0755); err != nil {
			t.Fatal(err)
		}
		if _, err := Generate(insideClaude, Options{}); err == nil {
			t.Errorf("expected error for root inside .claude, got nil")
		}

		// Inside .gemini
		insideGemini := filepath.Join(geminiDir, "subdir")
		if err := os.MkdirAll(insideGemini, 0755); err != nil {
			t.Fatal(err)
		}
		if _, err := Generate(insideGemini, Options{}); err == nil {
			t.Errorf("expected error for root inside .gemini, got nil")
		}

		// Symlink pointing into forbidden dir
		linkDir := filepath.Join(t.TempDir(), "link_to_claude")
		if err := os.Symlink(claudeDir, linkDir); err != nil {
			t.Fatal(err)
		}
		if _, err := Generate(linkDir, Options{}); err == nil {
			t.Errorf("expected error for symlink pointing to forbidden dir, got nil")
		}
	})

	t.Run("atomicity no leftover temp files", func(t *testing.T) {
		root := t.TempDir()
		srcPath := filepath.Join(root, SourceName)
		if err := os.WriteFile(srcPath, validSource, 0644); err != nil {
			t.Fatal(err)
		}

		if _, err := Generate(root, Options{}); err != nil {
			t.Fatal(err)
		}

		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if strings.Contains(e.Name(), ".tmp") {
				t.Errorf("found leftover temp file: %s", e.Name())
			}
		}
	})
}

func TestInitSource(t *testing.T) {
	t.Run("writes once and never overwrites", func(t *testing.T) {
		root := t.TempDir()
		written, err := InitSource(root)
		if err != nil {
			t.Fatalf("InitSource failed: %v", err)
		}
		if !written {
			t.Fatal("expected written = true on first init")
		}

		srcPath := filepath.Join(root, SourceName)
		content, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatal(err)
		}
		if len(content) == 0 {
			t.Fatal("expected non-empty source file")
		}

		// Overwrite test: custom content
		custom := []byte("# Custom content\n")
		if err := os.WriteFile(srcPath, custom, 0644); err != nil {
			t.Fatal(err)
		}

		written2, err := InitSource(root)
		if err != nil {
			t.Fatalf("second InitSource failed: %v", err)
		}
		if written2 {
			t.Error("expected written = false on second init")
		}

		content2, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(content2) != string(custom) {
			t.Errorf("InitSource overwrote existing source file!")
		}
	})

	t.Run("forbidden roots refused", func(t *testing.T) {
		tempHome := t.TempDir()
		t.Setenv("HOME", tempHome)

		claudeDir := filepath.Join(tempHome, ".claude")
		if err := os.MkdirAll(claudeDir, 0755); err != nil {
			t.Fatal(err)
		}

		if _, err := InitSource(claudeDir); err == nil {
			t.Errorf("expected error for InitSource in .claude, got nil")
		}
	})
}

func TestIsInsideOrEqualHandlesDotDotPrefixedNames(t *testing.T) {
	cases := []struct {
		path, target string
		want         bool
	}{
		{"/h/.claude", "/h/.claude", true},
		{"/h/.claude/sub", "/h/.claude", true},
		{"/h/.claude/..foo", "/h/.claude", true},
		{"/h/.claude/..foo/bar", "/h/.claude", true},
		{"/h/.claudex", "/h/.claude", false},
		{"/h", "/h/.claude", false},
		{"/other", "/h/.claude", false},
	}
	for _, c := range cases {
		if got := isInsideOrEqual(c.path, c.target); got != c.want {
			t.Errorf("isInsideOrEqual(%q, %q) = %v, want %v", c.path, c.target, got, c.want)
		}
	}
}
