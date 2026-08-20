package serve_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/serve"
)

func TestEmbedFSHasNoApproveAllControl(t *testing.T) {
	staticFS := serve.StaticFS()

	files := []string{"index.html", "app.js"}
	forbiddenTerms := []string{
		"approve all",
		"approve-all",
		"approve_all",
		"approveall",
		"select all",
		"select-all",
		"select_all",
		"bulk-approve",
		"bulk_approve",
	}

	for _, filename := range files {
		data, err := fs.ReadFile(staticFS, filename)
		if err != nil {
			t.Fatalf("fs.ReadFile(%q): %v", filename, err)
		}
		lower := strings.ToLower(string(data))
		for _, term := range forbiddenTerms {
			if strings.Contains(lower, term) {
				t.Errorf("file %q contains forbidden bulk approval term %q", filename, term)
			}
		}
	}
}

func TestStaticAssetsContainOpencodeCommandAndInlineEvidence(t *testing.T) {
	staticFS := serve.StaticFS()

	htmlData, err := fs.ReadFile(staticFS, "index.html")
	if err != nil {
		t.Fatalf("fs.ReadFile(index.html): %v", err)
	}
	htmlStr := string(htmlData)

	if !strings.Contains(htmlStr, "opencode") {
		t.Errorf("index.html does not contain opencode command section")
	}
	if !strings.Contains(htmlStr, "approvals-container") {
		t.Errorf("index.html does not contain approvals container")
	}

	jsData, err := fs.ReadFile(staticFS, "app.js")
	if err != nil {
		t.Fatalf("fs.ReadFile(app.js): %v", err)
	}
	jsStr := string(jsData)

	// Verify evidence validation logic is present
	if !strings.Contains(jsStr, "isValidEvidence") {
		t.Errorf("app.js does not contain isValidEvidence logic for inline evidence")
	}
}

func TestItemsStartUnselectedInUI(t *testing.T) {
	staticFS := serve.StaticFS()

	htmlData, err := fs.ReadFile(staticFS, "index.html")
	if err != nil {
		t.Fatalf("fs.ReadFile(index.html): %v", err)
	}
	htmlStr := string(htmlData)

	if strings.Contains(htmlStr, "checked") || strings.Contains(htmlStr, "selected") {
		t.Errorf("index.html should not have pre-selected or pre-checked controls")
	}
}
