package router_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/risk"
	"github.com/LanzerDevCorp/lucind-ai/internal/router"
)

func TestSignals_OnlyIntAndBoolFields(t *testing.T) {
	sigType := reflect.TypeOf(router.Signals{})
	if sigType.NumField() == 0 {
		t.Fatal("Signals struct has no fields")
	}

	for i := 0; i < sigType.NumField(); i++ {
		field := sigType.Field(i)
		kind := field.Type.Kind()
		switch kind {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Bool:
			// Allowed numeric and boolean types
		default:
			t.Errorf("field %q has kind %v; only int and bool kinds are allowed by the data-protection rule", field.Name, kind)
		}
	}
}

func TestSignalsFromRiskTier(t *testing.T) {
	tests := []struct {
		name string
		tier risk.Tier
		want int
	}{
		{name: "passive", tier: risk.TierPassive, want: 0},
		{name: "medium", tier: risk.TierMedium, want: 1},
		{name: "high", tier: risk.TierHigh, want: 2},
		{name: "unknown empty", tier: "", want: 2},
		{name: "unknown other", tier: "critical", want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := router.SignalsFromRiskTier(tt.tier)
			if got != tt.want {
				t.Fatalf("SignalsFromRiskTier(%q) = %d; want %d", tt.tier, got, tt.want)
			}
		})
	}
}

func TestDeterministic_Route(t *testing.T) {
	tests := []struct {
		name        string
		signals     router.Signals
		wantRoute   string
		wantReasons []string
	}{
		{
			name:        "no hard signals",
			signals:     router.Signals{AllowedPathCount: 1, NewFile: false, RiskTierLevel: 1, ReadOnly: false},
			wantRoute:   "inline",
			wantReasons: nil,
		},
		{
			name:        "hard signal: path count 2",
			signals:     router.Signals{AllowedPathCount: 2, NewFile: false, RiskTierLevel: 0, ReadOnly: false},
			wantRoute:   "worker",
			wantReasons: []string{"allowed_paths:2"},
		},
		{
			name:        "hard signal: path count 5",
			signals:     router.Signals{AllowedPathCount: 5, NewFile: false, RiskTierLevel: 1, ReadOnly: false},
			wantRoute:   "worker",
			wantReasons: []string{"allowed_paths:5"},
		},
		{
			name:        "hard signal: new file",
			signals:     router.Signals{AllowedPathCount: 1, NewFile: true, RiskTierLevel: 0, ReadOnly: false},
			wantRoute:   "worker",
			wantReasons: []string{"new_file"},
		},
		{
			name:        "hard signal: risk tier high (level 2)",
			signals:     router.Signals{AllowedPathCount: 1, NewFile: false, RiskTierLevel: 2, ReadOnly: false},
			wantRoute:   "worker",
			wantReasons: []string{"risk_tier:high"},
		},
		{
			name:        "multiple hard signals",
			signals:     router.Signals{AllowedPathCount: 3, NewFile: true, RiskTierLevel: 2, ReadOnly: false},
			wantRoute:   "worker",
			wantReasons: []string{"allowed_paths:3", "new_file", "risk_tier:high"},
		},
	}

	det := router.Deterministic{}
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec, err := det.Route(ctx, tt.signals)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if dec.Route != tt.wantRoute {
				t.Errorf("Route = %q; want %q", dec.Route, tt.wantRoute)
			}
			if dec.Source != "deterministic" {
				t.Errorf("Source = %q; want %q", dec.Source, "deterministic")
			}
			if dec.Confidence != 1.0 {
				t.Errorf("Confidence = %v; want 1.0", dec.Confidence)
			}
			if len(dec.Reasons) != len(tt.wantReasons) {
				t.Fatalf("len(Reasons) = %d; want %d (%v)", len(dec.Reasons), len(tt.wantReasons), dec.Reasons)
			}
			for i, r := range tt.wantReasons {
				if dec.Reasons[i] != r {
					t.Errorf("Reasons[%d] = %q; want %q", i, dec.Reasons[i], r)
				}
			}
		})
	}
}
