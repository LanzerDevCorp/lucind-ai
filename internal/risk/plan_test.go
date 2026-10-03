package risk_test

import (
	"reflect"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/risk"
)

func TestPlanFor(t *testing.T) {
	tests := []struct {
		name string
		tier risk.Tier
		want risk.Plan
	}{
		{
			name: "passive",
			tier: risk.TierPassive,
			want: risk.Plan{
				Tier:               risk.TierPassive,
				StructuralReadback: true,
				Attestation:        false,
				Judges:             0,
				ClaudeReview:       false,
			},
		},
		{
			name: "medium",
			tier: risk.TierMedium,
			want: risk.Plan{
				Tier:               risk.TierMedium,
				StructuralReadback: false,
				Attestation:        true,
				Judges:             1,
				ClaudeReview:       false,
			},
		},
		{
			name: "high",
			tier: risk.TierHigh,
			want: risk.Plan{
				Tier:               risk.TierHigh,
				StructuralReadback: false,
				Attestation:        true,
				Judges:             2,
				ClaudeReview:       true,
			},
		},
		{
			name: "empty string fails closed to high",
			tier: risk.Tier(""),
			want: risk.Plan{
				Tier:               risk.TierHigh,
				StructuralReadback: false,
				Attestation:        true,
				Judges:             2,
				ClaudeReview:       true,
			},
		},
		{
			name: "unknown tier fails closed to high",
			tier: risk.Tier("unknown_tier_value"),
			want: risk.Plan{
				Tier:               risk.TierHigh,
				StructuralReadback: false,
				Attestation:        true,
				Judges:             2,
				ClaudeReview:       true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := risk.PlanFor(tt.tier)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("PlanFor(%q) = %+v, want %+v", tt.tier, got, tt.want)
			}
		})
	}
}
