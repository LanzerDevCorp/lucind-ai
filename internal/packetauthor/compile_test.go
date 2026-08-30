package packetauthor_test

import (
	"bytes"
	"strings"
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

func TestCompileDigestExcludesResolvedPaths(t *testing.T) {
	contractA := validContract()
	contractA.LaneRole = "apply"
	contractA.RequiredSkills = []string{
		"/var/tmp/root-a/lucind-executor/SKILL.md",
		"/var/tmp/root-a/lucind-apply/SKILL.md",
	}

	contractB := validContract()
	contractB.LaneRole = "apply"
	contractB.RequiredSkills = []string{
		"/home/user/root-b/lucind-executor/SKILL.md",
		"/home/user/root-b/lucind-apply/SKILL.md",
	}

	contractC := validContract()
	contractC.LaneRole = "apply"
	contractC.RequiredSkills = []string{
		"~/skills/lucind-executor/SKILL.md",
		"~/skills/lucind-apply/SKILL.md",
	}

	artA, err := packetauthor.Compile(contractA, validFeatureBinding())
	if err != nil {
		t.Fatalf("Compile(contractA) error = %v", err)
	}
	artB, err := packetauthor.Compile(contractB, validFeatureBinding())
	if err != nil {
		t.Fatalf("Compile(contractB) error = %v", err)
	}
	artC, err := packetauthor.Compile(contractC, validFeatureBinding())
	if err != nil {
		t.Fatalf("Compile(contractC) error = %v", err)
	}

	// Digests and normalized contract JSON must be identical across root prefixes.
	if artA.Digest == "" || artA.Digest != artB.Digest || artA.Digest != artC.Digest {
		t.Fatalf("digests differ across root prefixes: A=%q B=%q C=%q", artA.Digest, artB.Digest, artC.Digest)
	}
	if !bytes.Equal(artA.ContractJSON, artB.ContractJSON) || !bytes.Equal(artA.ContractJSON, artC.ContractJSON) {
		t.Fatalf("contractJSON differs across root prefixes: A=%s B=%s", artA.ContractJSON, artB.ContractJSON)
	}

	// Rendered bodies must differ because they carry the resolved filesystem paths.
	if bytes.Equal(artA.Body, artB.Body) {
		t.Fatal("rendered bodies should differ with differing resolved paths")
	}

	// Verify ## Required skills is rendered between ## Hard stops and ## Return.
	bodyA := string(artA.Body)
	if !strings.Contains(bodyA, "## Required skills\n- /var/tmp/root-a/lucind-executor/SKILL.md\n- /var/tmp/root-a/lucind-apply/SKILL.md") {
		t.Errorf("artA.Body missing expected ## Required skills section: %s", bodyA)
	}
	hardStopsIdx := strings.Index(bodyA, "## Hard stops")
	reqSkillsIdx := strings.Index(bodyA, "## Required skills")
	returnIdx := strings.Index(bodyA, "## Return")
	if hardStopsIdx < 0 || reqSkillsIdx < 0 || returnIdx < 0 || !(hardStopsIdx < reqSkillsIdx && reqSkillsIdx < returnIdx) {
		t.Errorf("section ordering incorrect in body: hardStops=%d, reqSkills=%d, return=%d", hardStopsIdx, reqSkillsIdx, returnIdx)
	}

	// Empty RequiredSkills omits ## Required skills section.
	contractEmpty := validContract()
	contractEmpty.RequiredSkills = nil
	artEmpty, err := packetauthor.Compile(contractEmpty, validFeatureBinding())
	if err != nil {
		t.Fatalf("Compile(contractEmpty) error = %v", err)
	}
	if strings.Contains(string(artEmpty.Body), "## Required skills") {
		t.Errorf("artEmpty.Body should omit ## Required skills: %s", string(artEmpty.Body))
	}
}

func TestCompileDigestChangesOnLaneRoleAndSkills(t *testing.T) {
	base, err := packetauthor.Compile(validContract(), validFeatureBinding())
	if err != nil {
		t.Fatalf("Compile(base) error = %v", err)
	}

	tests := []struct {
		name     string
		contract packetauthor.Contract
	}{
		{
			name: "lane_role changed",
			contract: mutateContract(func(c *packetauthor.Contract) {
				c.LaneRole = "verify"
			}),
		},
		{
			name: "adhoc_skills added",
			contract: mutateContract(func(c *packetauthor.Contract) {
				c.AdhocSkills = []string{"custom-lint"}
			}),
		},
		{
			name: "required_skills changed canonical name",
			contract: mutateContract(func(c *packetauthor.Contract) {
				c.RequiredSkills = []string{"/root/custom-different-skill/SKILL.md"}
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := packetauthor.Compile(tt.contract, validFeatureBinding())
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			if got.Digest == base.Digest || bytes.Equal(got.ContractJSON, base.ContractJSON) {
				t.Fatalf("%s did not change contractJSON and digest", tt.name)
			}
		})
	}
}

func mutateContract(change func(*packetauthor.Contract)) packetauthor.Contract {
	c := validContract()
	c.WritePaths = append([]string(nil), c.WritePaths...)
	c.ReadOnlyPaths = append([]string(nil), c.ReadOnlyPaths...)
	c.AdhocSkills = append([]string(nil), c.AdhocSkills...)
	c.RequiredSkills = append([]string(nil), c.RequiredSkills...)
	c.DoneCriteria = append([]string(nil), c.DoneCriteria...)
	c.HardStops = append([]string(nil), c.HardStops...)
	change(&c)
	return c
}
