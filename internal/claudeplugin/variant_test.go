package claudeplugin_test

import (
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/claudeplugin"
)

func TestRenderSkill_Variants(t *testing.T) {
	input := `# Title

Common intro.

<!-- lucind:variant auto -->
Auto section:
lucind-ai dispatch --auto-skills
<!-- /lucind:variant -->
<!-- lucind:variant manual -->
Manual section:
lucind-ai dispatch --manual-skills
<!-- /lucind:variant -->

Common footer.
`

	t.Run("auto variant", func(t *testing.T) {
		out, err := claudeplugin.RenderSkill([]byte(input), claudeplugin.VariantAuto)
		if err != nil {
			t.Fatalf("RenderSkill failed: %v", err)
		}
		got := string(out)

		if !strings.Contains(got, "Auto section:") {
			t.Errorf("expected auto section to be kept, got:\n%s", got)
		}
		if !strings.Contains(got, "lucind-ai dispatch --auto-skills") {
			t.Errorf("expected auto content, got:\n%s", got)
		}
		if strings.Contains(got, "Manual section:") {
			t.Errorf("expected manual section to be dropped, got:\n%s", got)
		}
		if strings.Contains(got, "lucind:variant") {
			t.Errorf("expected marker lines to be removed, got:\n%s", got)
		}
		if !strings.Contains(got, "Common intro.") || !strings.Contains(got, "Common footer.") {
			t.Errorf("common text missing, got:\n%s", got)
		}
	})

	t.Run("manual variant", func(t *testing.T) {
		out, err := claudeplugin.RenderSkill([]byte(input), claudeplugin.VariantManual)
		if err != nil {
			t.Fatalf("RenderSkill failed: %v", err)
		}
		got := string(out)

		if !strings.Contains(got, "Manual section:") {
			t.Errorf("expected manual section to be kept, got:\n%s", got)
		}
		if !strings.Contains(got, "lucind-ai dispatch --manual-skills") {
			t.Errorf("expected manual content, got:\n%s", got)
		}
		if strings.Contains(got, "Auto section:") {
			t.Errorf("expected auto section to be dropped, got:\n%s", got)
		}
		if strings.Contains(got, "lucind:variant") {
			t.Errorf("expected marker lines to be removed, got:\n%s", got)
		}
		if !strings.Contains(got, "Common intro.") || !strings.Contains(got, "Common footer.") {
			t.Errorf("common text missing, got:\n%s", got)
		}
	})
}

func TestRenderSkill_Errors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		variant claudeplugin.Variant
		wantErr string
	}{
		{
			name:    "unknown variant argument",
			input:   "some content",
			variant: claudeplugin.Variant("unsupported"),
			wantErr: "unknown variant",
		},
		{
			name: "unknown variant in marker",
			input: `Common
<!-- lucind:variant other -->
Other
<!-- /lucind:variant -->
`,
			variant: claudeplugin.VariantAuto,
			wantErr: "unknown variant marker",
		},
		{
			name: "unclosed variant marker",
			input: `Common
<!-- lucind:variant auto -->
Auto content
`,
			variant: claudeplugin.VariantAuto,
			wantErr: "unclosed",
		},
		{
			name: "unopened closing marker",
			input: `Common
<!-- /lucind:variant -->
`,
			variant: claudeplugin.VariantAuto,
			wantErr: "unopened",
		},
		{
			name: "nested variant marker",
			input: `Common
<!-- lucind:variant auto -->
<!-- lucind:variant manual -->
Nested
<!-- /lucind:variant -->
<!-- /lucind:variant -->
`,
			variant: claudeplugin.VariantAuto,
			wantErr: "nested",
		},
		{
			name: "malformed variant marker",
			input: `Common
<!-- lucind:variant
`,
			variant: claudeplugin.VariantAuto,
			wantErr: "malformed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := claudeplugin.RenderSkill([]byte(tt.input), tt.variant)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.wantErr)) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
