package risk_test

import (
	"reflect"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/risk"
)

func TestClassifyPaths(t *testing.T) {
	tests := []struct {
		name        string
		paths       []string
		wantTier    risk.Tier
		wantReasons []string
	}{
		{
			name:        "nil paths fail closed to high",
			paths:       nil,
			wantTier:    risk.TierHigh,
			wantReasons: []string{"classifier_error:no_paths"},
		},
		{
			name:        "empty paths slice fail closed to high",
			paths:       []string{},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"classifier_error:no_paths"},
		},
		{
			name:        "path token auth positive",
			paths:       []string{"internal/auth/login.go"},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"path_token:auth:internal/auth/login.go"},
		},
		{
			name:        "path token update positive",
			paths:       []string{"cmd/update.go"},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"path_token:update:cmd/update.go"},
		},
		{
			name:        "path token security positive",
			paths:       []string{"pkg/security/check.go"},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"path_token:security:pkg/security/check.go"},
		},
		{
			name:        "path token webhook positive",
			paths:       []string{"services/webhook/send.go"},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"path_token:webhook:services/webhook/send.go"},
		},
		{
			name:        "path token payments positive",
			paths:       []string{"billing/payments/charge.go"},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"path_token:payments:billing/payments/charge.go"},
		},
		{
			name:        "sensitive location Makefile positive",
			paths:       []string{"Makefile"},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"sensitive_path:Makefile"},
		},
		{
			name:        "sensitive location go.mod positive",
			paths:       []string{"go.mod"},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"sensitive_path:go.mod"},
		},
		{
			name:        "sensitive location internal/ledger positive",
			paths:       []string{"internal/ledger/entry.go"},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"sensitive_path:internal/ledger/entry.go"},
		},
		{
			name:        "active content SKILL.md is medium",
			paths:       []string{"skills/test/SKILL.md"},
			wantTier:    risk.TierMedium,
			wantReasons: []string{"active_content:skills/test/SKILL.md"},
		},
		{
			name:        "active content CLAUDE.md is medium",
			paths:       []string{"CLAUDE.md"},
			wantTier:    risk.TierMedium,
			wantReasons: []string{"active_content:CLAUDE.md"},
		},
		{
			name:        "active content plugin directory is medium",
			paths:       []string{"plugin/something/guide.md"},
			wantTier:    risk.TierMedium,
			wantReasons: []string{"active_content:plugin/something/guide.md"},
		},
		{
			name:        "plain code is medium without reasons",
			paths:       []string{"pkg/server/handler.go"},
			wantTier:    risk.TierMedium,
			wantReasons: nil,
		},
		{
			name:        "docs is medium because content cannot be proven",
			paths:       []string{"docs/readme.md"},
			wantTier:    risk.TierMedium,
			wantReasons: nil,
		},
		{
			name:        "mixed paths with tokens, sensitive, active and plain code",
			paths:       []string{"docs/readme.md", "internal/auth/login.go", "plugin/tool/run.md", "Makefile"},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"active_content:plugin/tool/run.md", "path_token:auth:internal/auth/login.go", "sensitive_path:Makefile"},
		},
		{
			name:        "duplicate reasons are deduplicated and sorted",
			paths:       []string{"internal/auth/login.go", "internal/auth/login.go"},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"path_token:auth:internal/auth/login.go"},
		},
		{
			name:        "odd input empty string does not panic and stays medium",
			paths:       []string{""},
			wantTier:    risk.TierMedium,
			wantReasons: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := risk.ClassifyPaths(tt.paths)
			if got.Tier != tt.wantTier {
				t.Errorf("ClassifyPaths() Tier = %v, want %v", got.Tier, tt.wantTier)
			}
			if !reflect.DeepEqual(got.Reasons, tt.wantReasons) {
				t.Errorf("ClassifyPaths() Reasons = %#v, want %#v", got.Reasons, tt.wantReasons)
			}
		})
	}
}

func TestClassifyPathsDeterminism(t *testing.T) {
	paths := []string{"internal/auth/login.go", "Makefile", "CLAUDE.md"}
	first := risk.ClassifyPaths(paths)
	for i := 0; i < 50; i++ {
		got := risk.ClassifyPaths(paths)
		if !reflect.DeepEqual(got, first) {
			t.Fatalf("iteration %d: ClassifyPaths() = %#v, want %#v", i, got, first)
		}
	}
}
