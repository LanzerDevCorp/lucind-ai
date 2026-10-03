package packetauthor_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
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
	if !strings.Contains(bodyA, "Read each SKILL.md above before starting work and list each skill's directory name in `skills_loaded` of the result envelope.") {
		t.Errorf("artA.Body missing the instruction to load and declare required skills: %s", bodyA)
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

func TestCompileNamedSkillsOnly(t *testing.T) {
	c := validContract()
	c.LaneRole = "apply"
	c.AdhocSkills = []string{"custom-tool"}
	c.NamedSkillsOnly = true

	art, err := packetauthor.Compile(c, validFeatureBinding())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	// Should derive only lucind-executor and custom-tool, no lucind-apply and no sdd-apply
	wantReq := []byte(`"required_skills":["custom-tool","lucind-executor"]`)
	if !bytes.Contains(art.ContractJSON, wantReq) {
		t.Fatalf("ContractJSON = %s, want required_skills with only custom-tool and lucind-executor", art.ContractJSON)
	}
	if bytes.Contains(art.ContractJSON, []byte("lucind-apply")) || bytes.Contains(art.ContractJSON, []byte("sdd-apply")) {
		t.Fatalf("ContractJSON contains lane-role or sdd skills: %s", art.ContractJSON)
	}
}

func TestCompileNewFieldsPropagate(t *testing.T) {
	base, err := packetauthor.Compile(validContract(), validFeatureBinding())
	if err != nil {
		t.Fatalf("Compile(base) error = %v", err)
	}

	c := validContract()
	c.Route = "worker"
	c.RouteEvidence = "touches auth"
	c.NamedSkillsOnly = true
	c.Verification = []string{"go test ./..."}
	c.KnownEnvironmentalFailures = []string{"TestFlaky"}

	art, err := packetauthor.Compile(c, validFeatureBinding())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if art.Digest == base.Digest {
		t.Errorf("art.Digest = %q, want different from base", art.Digest)
	}
	if !bytes.Contains(art.ContractJSON, []byte(`"route":"worker"`)) {
		t.Errorf("ContractJSON missing route: %s", art.ContractJSON)
	}
	if !bytes.Contains(art.ContractJSON, []byte(`"route_evidence":"touches auth"`)) {
		t.Errorf("ContractJSON missing route_evidence: %s", art.ContractJSON)
	}
	if !bytes.Contains(art.ContractJSON, []byte(`"named_skills_only":true`)) {
		t.Errorf("ContractJSON missing named_skills_only: %s", art.ContractJSON)
	}
	if !bytes.Contains(art.ContractJSON, []byte(`"verification":["go test ./..."]`)) {
		t.Errorf("ContractJSON missing verification: %s", art.ContractJSON)
	}
	if !bytes.Contains(art.ContractJSON, []byte(`"known_environmental_failures":["TestFlaky"]`)) {
		t.Errorf("ContractJSON missing known_environmental_failures: %s", art.ContractJSON)
	}
}

func TestCompileRejectsInvalidRoute(t *testing.T) {
	c := validContract()
	c.Route = "turbo"
	_, err := packetauthor.Compile(c, validFeatureBinding())
	assertDiagnosticCode(t, err, packetauthor.CodeContractInvalid)
	for _, ok := range []string{"", "inline", "worker", "fanout"} {
		c.Route = ok
		if _, err := packetauthor.Compile(c, validFeatureBinding()); err != nil {
			t.Fatalf("route %q must be accepted: %v", ok, err)
		}
	}
}

func TestCompileCommitMessage(t *testing.T) {
	t.Run("valid commit_message yields dispatcher commit obligation", func(t *testing.T) {
		c := validContract()
		c.Verification = []string{"go test ./..."}
		c.CommitMessage = "feat(auth): add login flow"
		art, err := packetauthor.Compile(c, validFeatureBinding())
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		if !strings.Contains(string(art.Body), "commit: dispatcher") {
			t.Errorf("expected body to contain 'commit: dispatcher', got:\n%s", string(art.Body))
		}
		if !bytes.Contains(art.ContractJSON, []byte(`"commit_message":"feat(auth): add login flow"`)) {
			t.Errorf("ContractJSON missing commit_message: %s", art.ContractJSON)
		}
	})

	t.Run("empty commit_message write mode yields required commit obligation", func(t *testing.T) {
		c := validContract()
		art, err := packetauthor.Compile(c, validFeatureBinding())
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		if !strings.Contains(string(art.Body), "commit: required") {
			t.Errorf("expected body to contain 'commit: required', got:\n%s", string(art.Body))
		}
	})

	t.Run("read-only mode yields forbidden commit obligation even if commit_message set", func(t *testing.T) {
		c := validContract()
		c.Mode = packetauthor.ModeReadOnly
		c.WritePaths = nil
		c.Verification = []string{"go test ./..."}
		c.CommitMessage = "feat: add login"
		art, err := packetauthor.Compile(c, validFeatureBinding())
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		if !strings.Contains(string(art.Body), "commit: forbidden") {
			t.Errorf("expected body to contain 'commit: forbidden', got:\n%s", string(art.Body))
		}
	})

	t.Run("commit_message requires verification", func(t *testing.T) {
		c := validContract()
		c.CommitMessage = "feat: add login"
		_, err := packetauthor.Compile(c, validFeatureBinding())
		assertDiagnosticCode(t, err, packetauthor.CodeContractInvalid)
	})

	t.Run("invalid commit_message rejected", func(t *testing.T) {
		for _, bad := range []string{
			"Update stuff",
			"feat implement login",
			"feat: " + strings.Repeat("a", 95),
			"feat: login\nnewline",
			"feat: login Co-Authored-By: AI",
			"feat: login (generated with LLM)",
		} {
			c := validContract()
			c.Verification = []string{"go test ./..."}
			c.CommitMessage = bad
			_, err := packetauthor.Compile(c, validFeatureBinding())
			assertDiagnosticCode(t, err, packetauthor.CodeContractInvalid)
		}
	})

	t.Run("setting commit_message changes artifact digest", func(t *testing.T) {
		c1 := validContract()
		c1.Verification = []string{"go test ./..."}
		art1, err := packetauthor.Compile(c1, validFeatureBinding())
		if err != nil {
			t.Fatal(err)
		}
		c2 := c1
		c2.CommitMessage = "feat: add login"
		art2, err := packetauthor.Compile(c2, validFeatureBinding())
		if err != nil {
			t.Fatal(err)
		}
		if art1.Digest == art2.Digest {
			t.Errorf("setting commit_message did not change digest: %q", art1.Digest)
		}
	})
}

func TestCompileLoopAndEscalation(t *testing.T) {
	t.Run("max_iterations and escalation propagate and change digest", func(t *testing.T) {
		baseContract := validContract()
		baseContract.Verification = []string{"go test ./..."}
		baseContract.CommitMessage = "feat: x"
		baseArt, err := packetauthor.Compile(baseContract, validFeatureBinding())
		if err != nil {
			t.Fatal(err)
		}

		c := baseContract
		c.MaxIterations = 2
		c.Escalation = []packet.EscalationRung{{Executor: "herdr-agy", Model: "gemini-3.8-flash-high"}}
		art, err := packetauthor.Compile(c, validFeatureBinding())
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}

		if art.Digest == baseArt.Digest {
			t.Errorf("art.Digest = %q, want different from baseArt.Digest", art.Digest)
		}
		if !bytes.Contains(art.ContractJSON, []byte(`"max_iterations":2`)) {
			t.Errorf("ContractJSON missing max_iterations: %s", art.ContractJSON)
		}
		if !bytes.Contains(art.ContractJSON, []byte(`"escalation":[{"executor":"herdr-agy","model":"gemini-3.8-flash-high"}]`)) {
			t.Errorf("ContractJSON missing escalation: %s", art.ContractJSON)
		}
	})

	t.Run("max_iterations out of bounds rejected", func(t *testing.T) {
		for _, bad := range []int{-1, 5} {
			c := validContract()
			c.Verification = []string{"go test ./..."}
			c.MaxIterations = bad
			_, err := packetauthor.Compile(c, validFeatureBinding())
			assertDiagnosticCode(t, err, packetauthor.CodeContractInvalid)
		}
	})

	t.Run("max_iterations > 1 requires verification", func(t *testing.T) {
		c := validContract()
		c.MaxIterations = 2
		_, err := packetauthor.Compile(c, validFeatureBinding())
		assertDiagnosticCode(t, err, packetauthor.CodeContractInvalid)
	})

	t.Run("escalation requires verification", func(t *testing.T) {
		c := validContract()
		c.Escalation = []packet.EscalationRung{{Executor: "herdr-agy"}}
		_, err := packetauthor.Compile(c, validFeatureBinding())
		assertDiagnosticCode(t, err, packetauthor.CodeContractInvalid)
	})

	t.Run("escalation with empty executor rejected", func(t *testing.T) {
		c := validContract()
		c.Verification = []string{"go test ./..."}
		c.Escalation = []packet.EscalationRung{{Executor: ""}}
		_, err := packetauthor.Compile(c, validFeatureBinding())
		assertDiagnosticCode(t, err, packetauthor.CodeContractInvalid)
	})

	t.Run("escalation with more than 3 rungs rejected", func(t *testing.T) {
		c := validContract()
		c.Verification = []string{"go test ./..."}
		c.Escalation = []packet.EscalationRung{
			{Executor: "a"},
			{Executor: "b"},
			{Executor: "c"},
			{Executor: "d"},
		}
		_, err := packetauthor.Compile(c, validFeatureBinding())
		assertDiagnosticCode(t, err, packetauthor.CodeContractInvalid)
	})
}

func TestCompileDeclaredRouterSignalsPropagate(t *testing.T) {
	base, err := packetauthor.Compile(validContract(), validFeatureBinding())
	if err != nil {
		t.Fatalf("Compile(base) error = %v", err)
	}
	no := false
	c := validContract()
	c.Understood = &no
	c.OpenDesign = true
	c.EstimatedLookups = 7
	art, err := packetauthor.Compile(c, validFeatureBinding())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if art.Digest == base.Digest {
		t.Error("declared signals must change the digest")
	}
	for _, want := range []string{`"understood":false`, `"open_design":true`, `"estimated_lookups":7`} {
		if !bytes.Contains(art.ContractJSON, []byte(want)) {
			t.Errorf("ContractJSON missing %s: %s", want, art.ContractJSON)
		}
	}
}
