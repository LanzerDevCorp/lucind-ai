package packetauthor_test

import (
	"bytes"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/packetauthor"
)

func TestCompileDeterministicReplayAndCanonicalOrdering(t *testing.T) {
	first, err := packetauthor.Compile(validContract(), validFeatureBinding())
	if err != nil {
		t.Fatalf("first Compile() error = %v", err)
	}
	second, err := packetauthor.Compile(validContract(), validFeatureBinding())
	if err != nil {
		t.Fatalf("second Compile() error = %v", err)
	}
	if !bytes.Equal(first.Body, second.Body) || !bytes.Equal(first.ContractJSON, second.ContractJSON) || !bytes.Equal(first.ManifestJSON, second.ManifestJSON) {
		t.Fatal("replayed compilation produced different bytes")
	}
	if first.Digest == "" || first.Digest != second.Digest {
		t.Fatalf("replayed digest = %q and %q, want one stable non-empty digest", first.Digest, second.Digest)
	}
	wantPaths := []byte(`"write_paths":["internal/a.go","internal/z.go"]`)
	if !bytes.Contains(first.ContractJSON, wantPaths) {
		t.Fatalf("ContractJSON = %s, want byte-sorted write_paths", first.ContractJSON)
	}
	for name, data := range map[string][]byte{"body": first.Body, "contract": first.ContractJSON, "manifest": first.ManifestJSON} {
		if len(data) == 0 || data[len(data)-1] != '\n' || bytes.HasSuffix(data, []byte("\n\n")) {
			t.Errorf("%s must have exactly one terminal LF: %q", name, data)
		}
	}
}

func TestCompileDigestChangesForEveryRelevantInputClass(t *testing.T) {
	base, err := packetauthor.Compile(validContract(), validFeatureBinding())
	if err != nil {
		t.Fatalf("Compile(base) error = %v", err)
	}
	tests := []struct {
		name     string
		contract packetauthor.Contract
		binding  packetauthor.TargetBinding
	}{
		{name: "criterion", contract: mutateContract(func(c *packetauthor.Contract) { c.DoneCriteria[0] = "Changed criterion." }), binding: validFeatureBinding()},
		{name: "stop", contract: mutateContract(func(c *packetauthor.Contract) { c.HardStops[0] = "Changed stop." }), binding: validFeatureBinding()},
		{name: "mode", contract: mutateContract(func(c *packetauthor.Contract) { c.Mode = packetauthor.ModeReadOnly; c.WritePaths = nil }), binding: validFeatureBinding()},
		{name: "path", contract: mutateContract(func(c *packetauthor.Contract) { c.WritePaths[0] = "internal/changed.go" }), binding: validFeatureBinding()},
		{name: "target", contract: validContract(), binding: func() packetauthor.TargetBinding { b := validFeatureBinding(); b.Feature.BaseSHA = sha('d'); return b }()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := packetauthor.Compile(tt.contract, tt.binding)
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			if got.Digest == base.Digest || bytes.Equal(got.ManifestJSON, base.ManifestJSON) {
				t.Fatalf("relevant %s change did not change manifest and digest", tt.name)
			}
		})
	}
}

func TestCompileRejectsDuplicateDeclarations(t *testing.T) {
	contract := validContract()
	contract.DoneCriteria = append(contract.DoneCriteria, contract.DoneCriteria[0])
	_, err := packetauthor.Compile(contract, validFeatureBinding())
	assertDiagnosticCode(t, err, packetauthor.CodeContractInvalid)
}

func mutateContract(change func(*packetauthor.Contract)) packetauthor.Contract {
	c := validContract()
	c.WritePaths = append([]string(nil), c.WritePaths...)
	c.ReadOnlyPaths = append([]string(nil), c.ReadOnlyPaths...)
	c.DoneCriteria = append([]string(nil), c.DoneCriteria...)
	c.HardStops = append([]string(nil), c.HardStops...)
	change(&c)
	return c
}
