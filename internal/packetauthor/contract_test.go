package packetauthor_test

import (
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/packetauthor"
)

func TestCompileBindsExactlyOneAuthoritativeTarget(t *testing.T) {
	contract := validContract()
	tests := []struct {
		name    string
		binding packetauthor.TargetBinding
		want    packetauthor.TargetKind
	}{
		{
			name: "feature",
			binding: packetauthor.TargetBinding{Feature: &packetauthor.FeatureTarget{
				Feature: "delegated-authoring", ParentRef: "refs/heads/feature/delegated-authoring",
				BaseSHA: sha('a'), ExpectedParentSHA: sha('b'), LiveParentSHA: sha('b'),
			}},
			want: packetauthor.TargetFeature,
		},
		{
			name: "legacy main",
			binding: packetauthor.TargetBinding{LegacyMain: &packetauthor.LegacyMainTarget{
				ExpectedParentSHA: sha('c'), LiveParentSHA: sha('c'),
			}},
			want: packetauthor.TargetLegacyMain,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifact, err := packetauthor.Compile(contract, tt.binding)
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			if artifact.Binding.Kind != tt.want {
				t.Fatalf("artifact.Binding.Kind = %q, want %q", artifact.Binding.Kind, tt.want)
			}
		})
	}
}

func TestCompileRejectsTargetAuthorityAndStaleBindings(t *testing.T) {
	tests := []struct {
		name     string
		contract packetauthor.Contract
		binding  packetauthor.TargetBinding
		wantCode string
	}{
		{
			name: "authored live target",
			contract: func() packetauthor.Contract {
				c := validContract()
				c.TargetClaims = map[string]string{"expected_parent_sha": sha('a')}
				return c
			}(),
			binding:  validFeatureBinding(),
			wantCode: packetauthor.CodeForbiddenTarget,
		},
		{
			name:     "stale feature",
			contract: validContract(),
			binding: func() packetauthor.TargetBinding {
				b := validFeatureBinding()
				b.Feature.LiveParentSHA = sha('c')
				return b
			}(),
			wantCode: packetauthor.CodeTargetStale,
		},
		{
			name:     "both target variants",
			contract: validContract(),
			binding: packetauthor.TargetBinding{
				Feature: validFeatureBinding().Feature,
				LegacyMain: &packetauthor.LegacyMainTarget{
					ExpectedParentSHA: sha('b'), LiveParentSHA: sha('b'),
				},
			},
			wantCode: packetauthor.CodeTargetIncomplete,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := packetauthor.Compile(tt.contract, tt.binding)
			assertDiagnosticCode(t, err, tt.wantCode)
		})
	}
}

func validContract() packetauthor.Contract {
	return packetauthor.Contract{
		Version:       packetauthor.ContractVersion,
		RouteIntent:   "apply",
		Mode:          packetauthor.ModeWrite,
		WritePaths:    []string{"internal/z.go", "internal/a.go"},
		ReadOnlyPaths: []string{"openspec/spec.md"},
		Goal:          "Compile delegated packet contracts.",
		DoneCriteria:  []string{"Manual bodies remain byte-identical.", "Replay is deterministic."},
		HardStops:     []string{"Do not dispatch work."},
		Result: packetauthor.ResultObligations{
			Path: ".lucind/result.json", Schema: ".lucind/result.schema.json",
		},
	}
}

func validFeatureBinding() packetauthor.TargetBinding {
	return packetauthor.TargetBinding{Feature: &packetauthor.FeatureTarget{
		Feature: "delegated-authoring", ParentRef: "refs/heads/feature/delegated-authoring",
		BaseSHA: sha('a'), ExpectedParentSHA: sha('b'), LiveParentSHA: sha('b'),
	}}
}

func sha(ch byte) string {
	buf := make([]byte, 40)
	for i := range buf {
		buf[i] = ch
	}
	return string(buf)
}
