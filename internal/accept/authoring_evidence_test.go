package accept

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/candidatechange"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
)

func TestValidateTypedTargetBindingRequiresCompleteIdentity(t *testing.T) {
	metadata := ledger.LaneMetadata{
		Feature:           "feature-1",
		ParentRef:         "refs/heads/feature-1",
		BaseSHA:           "base-sha",
		ExpectedParentSHA: "expected-parent",
	}
	validFeature := json.RawMessage(`{"kind":"feature","feature":"feature-1","parent_ref":"refs/heads/feature-1","base_sha":"base-sha","expected_parent_sha":"expected-parent"}`)
	tests := []struct {
		name    string
		binding string
	}{
		{name: "valid feature", binding: string(validFeature)},
		{name: "feature identity", binding: strings.Replace(string(validFeature), "feature-1", "other-feature", 1)},
		{name: "parent ref", binding: strings.Replace(string(validFeature), "refs/heads/feature-1", "refs/heads/other", 1)},
		{name: "base sha", binding: strings.Replace(string(validFeature), "base-sha", "other-base", 1)},
		{name: "expected parent sha", binding: strings.Replace(string(validFeature), "expected-parent", "other-parent", 1)},
		{name: "missing feature field", binding: strings.Replace(string(validFeature), `"feature":"feature-1",`, "", 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTypedTargetBinding(json.RawMessage(tt.binding), metadata)
			if tt.name == "valid feature" {
				if err != nil {
					t.Fatalf("valid binding rejected: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("invalid typed binding accepted")
			}
		})
	}

	legacy := ledger.LaneMetadata{ParentRef: "main", ExpectedParentSHA: "expected-parent"}
	if err := validateTypedTargetBinding(json.RawMessage(`{"kind":"legacy-main","parent_ref":"refs/heads/main","expected_parent_sha":"expected-parent"}`), legacy); err != nil {
		t.Fatalf("valid legacy binding rejected: %v", err)
	}
	if err := validateTypedTargetBinding(json.RawMessage(`{"kind":"legacy-main","parent_ref":"refs/heads/main","expected_parent_sha":"other-parent"}`), legacy); err == nil {
		t.Fatal("legacy expected-parent mismatch accepted")
	}
}

func TestValidateVersionedResultRequiresExactFrozenCorrespondence(t *testing.T) {
	resultJSON := `{"packet_id":"lane-1","status":"done","summary":"done","hard_stops":[{"hard_stop":"stop","fired":false}],"files_changed":[{"change":"copied","source_path":"seed.txt","path":"copy.txt"}],"done_criteria":[{"criterion":"criterion","met":true}],"skills_loaded":["lucind-executor","lucind-apply"],"commit":"CANDIDATE"}`
	f := newVerifierFixture(t, resultJSON, "", map[string]string{"copy.txt": "seed\n"}, []string{"seed.txt", "copy.txt"})
	resultJSON = strings.Replace(resultJSON, "CANDIDATE", f.candidate, 1)
	f.candidateRow.ResultJSON, f.candidateRow.ResultHash = resultJSON, hashValues("result:v1", resultJSON)
	contract := `{"version":"packet-author/v1","mode":"write","lane_role":"apply","required_skills":["lucind-executor","lucind-apply"],"write_paths":["copy.txt","seed.txt"],"read_only_paths":null,"done_criteria":["criterion"],"hard_stops":["stop"],"result":{"path":".lucind/result.json","schema":".lucind/result.schema.json"}}`
	e := ledger.AuthoringEvidence{PacketDigest: f.candidateRow.PacketDigest, AuthoringMode: "versioned", ContractVersion: "packet-author/v1", Contract: json.RawMessage(contract), Binding: json.RawMessage(`{"kind":"feature","base_sha":"` + f.base + `"}`),
		Mode: "write", CommitObligation: "required", WritePaths: []string{"copy.txt", "seed.txt"}, DoneCriteria: []string{"criterion"}, HardStops: []string{"stop"}, ResultPath: ".lucind/result.json", ResultSchema: ".lucind/result.schema.json",
		BaseCommit: f.base, BaseTree: f.candidateRow.BaseTree, CandidateCommit: f.candidate, CandidateTree: f.candidateRow.CandidateTree, Changes: []candidatechange.Change{{Change: candidatechange.Copied, SourcePath: "seed.txt", Path: "copy.txt"}}, ResultHash: f.candidateRow.ResultHash}
	encoded, hash, err := ledger.FreezeAuthoringEvidence(e)
	if err != nil {
		t.Fatal(err)
	}
	f.candidateRow.AuthoringEvidenceVersion, f.candidateRow.AuthoringEvidenceJSON, f.candidateRow.AuthoringEvidenceHash = ledger.AuthoringEvidenceVersion, encoded, hash
	if err := validateResultAndScope(context.Background(), f.root, f.candidateRow); err != nil {
		t.Fatalf("exact correspondence rejected: %v", err)
	}

	mutations := []struct {
		name   string
		mutate func(*ledger.LaneCandidate)
	}{
		{"criterion", func(c *ledger.LaneCandidate) {
			c.ResultJSON = strings.Replace(c.ResultJSON, "criterion", "altered", 1)
			c.ResultHash = hashValues("result:v1", c.ResultJSON)
		}},
		{"hard stop", func(c *ledger.LaneCandidate) {
			c.ResultJSON = strings.Replace(c.ResultJSON, "stop", "altered stop", 1)
			c.ResultHash = hashValues("result:v1", c.ResultJSON)
		}},
		{"commit", func(c *ledger.LaneCandidate) {
			c.ResultJSON = strings.Replace(c.ResultJSON, f.candidate, f.base, 1)
			c.ResultHash = hashValues("result:v1", c.ResultJSON)
		}},
		{"copy source", func(c *ledger.LaneCandidate) {
			c.ResultJSON = strings.Replace(c.ResultJSON, "seed.txt", "other.txt", 1)
			c.ResultHash = hashValues("result:v1", c.ResultJSON)
		}},
		{"copy classification", func(c *ledger.LaneCandidate) {
			c.ResultJSON = strings.Replace(c.ResultJSON, `"change":"copied","source_path":"seed.txt",`, `"change":"created",`, 1)
			c.ResultHash = hashValues("result:v1", c.ResultJSON)
		}},
		{"missing required skill", func(c *ledger.LaneCandidate) {
			c.ResultJSON = strings.Replace(c.ResultJSON, `"skills_loaded":["lucind-executor","lucind-apply"]`, `"skills_loaded":["lucind-executor"]`, 1)
			c.ResultHash = hashValues("result:v1", c.ResultJSON)
		}},
		{"omitted skills_loaded", func(c *ledger.LaneCandidate) {
			c.ResultJSON = strings.Replace(c.ResultJSON, `,"skills_loaded":["lucind-executor","lucind-apply"]`, "", 1)
			c.ResultHash = hashValues("result:v1", c.ResultJSON)
		}},
		{"evidence hash", func(c *ledger.LaneCandidate) { c.AuthoringEvidenceHash = "sha256:tampered" }},
	}
	for _, tt := range mutations {
		t.Run(tt.name, func(t *testing.T) {
			c := f.candidateRow
			tt.mutate(&c)
			if tt.name != "evidence hash" {
				updated := e
				updated.ResultHash = c.ResultHash
				c.AuthoringEvidenceJSON, c.AuthoringEvidenceHash, err = ledger.FreezeAuthoringEvidence(updated)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := validateResultAndScope(context.Background(), f.root, c); err == nil {
				t.Fatal("mismatch accepted")
			}
		})
	}
	tamperedContract := e
	tamperedContract.Contract = json.RawMessage(strings.Replace(string(e.Contract), "criterion", "different", 1))
	c := f.candidateRow
	c.AuthoringEvidenceJSON, c.AuthoringEvidenceHash, err = ledger.FreezeAuthoringEvidence(tamperedContract)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateResultAndScope(context.Background(), f.root, c); err == nil {
		t.Fatal("contradictory normalized contract accepted")
	}

	tamperedLaneRole := e
	tamperedLaneRole.Contract = json.RawMessage(strings.Replace(string(e.Contract), `"lane_role":"apply"`, `"lane_role":"invalid-role"`, 1))
	cLaneRole := f.candidateRow
	cLaneRole.AuthoringEvidenceJSON, cLaneRole.AuthoringEvidenceHash, err = ledger.FreezeAuthoringEvidence(tamperedLaneRole)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateResultAndScope(context.Background(), f.root, cLaneRole); err == nil {
		t.Fatal("invalid lane role accepted")
	}
}

func TestValidateVersionedReadOnlyForbidsCommitAndChanges(t *testing.T) {
	resultJSON := `{"packet_id":"lane-1","status":"done","summary":"read","hard_stops":[{"hard_stop":"stop","fired":false}],"files_changed":[],"done_criteria":[{"criterion":"inspect","met":true}]}`
	f := newVerifierFixture(t, resultJSON, "", map[string]string{"temporary.txt": "candidate\n"}, []string{"temporary.txt"})
	f.candidateRow.BaseCommit, f.candidateRow.BaseTree = f.candidate, f.candidateRow.CandidateTree
	e := ledger.AuthoringEvidence{PacketDigest: f.candidateRow.PacketDigest, AuthoringMode: "versioned", ContractVersion: "packet-author/v1",
		Contract: json.RawMessage(`{"version":"packet-author/v1","mode":"read-only","write_paths":null,"read_only_paths":["temporary.txt"],"done_criteria":["inspect"],"hard_stops":["stop"],"result":{"path":".lucind/result.json","schema":".lucind/result.schema.json"}}`), Binding: json.RawMessage(`{"kind":"feature","base_sha":"` + f.candidate + `"}`),
		Mode: "read-only", CommitObligation: "forbidden", ReadOnlyPaths: []string{"temporary.txt"}, DoneCriteria: []string{"inspect"}, HardStops: []string{"stop"}, ResultPath: ".lucind/result.json", ResultSchema: ".lucind/result.schema.json",
		BaseCommit: f.candidate, BaseTree: f.candidateRow.CandidateTree, CandidateCommit: f.candidate, CandidateTree: f.candidateRow.CandidateTree, Changes: []candidatechange.Change{}, ResultHash: f.candidateRow.ResultHash}
	var err error
	f.candidateRow.AuthoringEvidenceJSON, f.candidateRow.AuthoringEvidenceHash, err = ledger.FreezeAuthoringEvidence(e)
	if err != nil {
		t.Fatal(err)
	}
	f.candidateRow.AuthoringEvidenceVersion = ledger.AuthoringEvidenceVersion
	if err := validateResultAndScope(context.Background(), f.root, f.candidateRow); err != nil {
		t.Fatal(err)
	}
	f.candidateRow.ResultJSON = strings.Replace(resultJSON, `"done_criteria"`, `"commit":"`+f.candidate+`","done_criteria"`, 1)
	f.candidateRow.ResultHash = hashValues("result:v1", f.candidateRow.ResultJSON)
	e.ResultHash = f.candidateRow.ResultHash
	f.candidateRow.AuthoringEvidenceJSON, f.candidateRow.AuthoringEvidenceHash, _ = ledger.FreezeAuthoringEvidence(e)
	if err := validateResultAndScope(context.Background(), f.root, f.candidateRow); err == nil {
		t.Fatal("read-only commit accepted")
	}
}

func TestValidateVersionedContractLaneRoleValidation(t *testing.T) {
	resultJSON := `{"packet_id":"lane-1","status":"done","summary":"done","hard_stops":[{"hard_stop":"stop","fired":false}],"files_changed":[{"change":"copied","source_path":"seed.txt","path":"copy.txt"}],"done_criteria":[{"criterion":"criterion","met":true}],"skills_loaded":["lucind-executor","lucind-apply"],"commit":"CANDIDATE"}`
	f := newVerifierFixture(t, resultJSON, "", map[string]string{"copy.txt": "seed\n"}, []string{"seed.txt", "copy.txt"})
	resultJSON = strings.Replace(resultJSON, "CANDIDATE", f.candidate, 1)
	f.candidateRow.ResultJSON, f.candidateRow.ResultHash = resultJSON, hashValues("result:v1", resultJSON)

	roles := []struct {
		role  string
		valid bool
	}{
		{"apply", true},
		{"verify", true},
		{"lens", true},
		{"synthesis", true},
		{"archive", true},
		{"ultrafixer", true},
		{"human", true},
		{"", true},
		{"invalid-role", false},
		{"executor", false},
		{"unknown", false},
	}

	for _, tt := range roles {
		t.Run("role_"+tt.role, func(t *testing.T) {
			contract := `{"version":"packet-author/v1","mode":"write","lane_role":"` + tt.role + `","required_skills":["lucind-executor","lucind-apply"],"write_paths":["copy.txt","seed.txt"],"read_only_paths":null,"done_criteria":["criterion"],"hard_stops":["stop"],"result":{"path":".lucind/result.json","schema":".lucind/result.schema.json"}}`
			if tt.role == "" {
				contract = `{"version":"packet-author/v1","mode":"write","required_skills":["lucind-executor","lucind-apply"],"write_paths":["copy.txt","seed.txt"],"read_only_paths":null,"done_criteria":["criterion"],"hard_stops":["stop"],"result":{"path":".lucind/result.json","schema":".lucind/result.schema.json"}}`
			}
			e := ledger.AuthoringEvidence{
				PacketDigest: f.candidateRow.PacketDigest, AuthoringMode: "versioned", ContractVersion: "packet-author/v1",
				Contract: json.RawMessage(contract), Binding: json.RawMessage(`{"kind":"feature","base_sha":"` + f.base + `"}`),
				Mode: "write", CommitObligation: "required", WritePaths: []string{"copy.txt", "seed.txt"}, DoneCriteria: []string{"criterion"}, HardStops: []string{"stop"},
				ResultPath: ".lucind/result.json", ResultSchema: ".lucind/result.schema.json",
				BaseCommit: f.base, BaseTree: f.candidateRow.BaseTree, CandidateCommit: f.candidate, CandidateTree: f.candidateRow.CandidateTree,
				Changes: []candidatechange.Change{{Change: candidatechange.Copied, SourcePath: "seed.txt", Path: "copy.txt"}}, ResultHash: f.candidateRow.ResultHash,
			}
			encoded, hash, err := ledger.FreezeAuthoringEvidence(e)
			if err != nil {
				t.Fatal(err)
			}
			c := f.candidateRow
			c.AuthoringEvidenceVersion, c.AuthoringEvidenceJSON, c.AuthoringEvidenceHash = ledger.AuthoringEvidenceVersion, encoded, hash
			err = validateResultAndScope(context.Background(), f.root, c)
			if tt.valid && err != nil {
				t.Fatalf("valid lane role %q rejected: %v", tt.role, err)
			}
			if !tt.valid && err == nil {
				t.Fatalf("invalid lane role %q accepted", tt.role)
			}
		})
	}
}
