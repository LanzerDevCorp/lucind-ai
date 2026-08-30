package packetauthor_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/packetauthor"
)

type fakeSpecialistRunner struct {
	response   packetauthor.SpecialistResponse
	invocation packetauthor.SpecialistInvocation
}

func (f *fakeSpecialistRunner) Run(_ context.Context, invocation packetauthor.SpecialistInvocation) (packetauthor.SpecialistResponse, error) {
	f.invocation = invocation
	return f.response, nil
}

func TestSpecialistAdapterAcceptsTypedTargetFreeOutput(t *testing.T) {
	request, err := packetauthor.NewSpecialistRequest(validContract())
	if err != nil {
		t.Fatalf("NewSpecialistRequest() error = %v", err)
	}
	runner := &fakeSpecialistRunner{response: packetauthor.SpecialistResponse{
		Identity: packetauthor.SpecialistAgentName,
		Output:   validSpecialistOutput(),
	}}

	contract, err := (packetauthor.SpecialistAdapter{Runner: runner}).Author(context.Background(), request)
	if err != nil {
		t.Fatalf("Author() error = %v", err)
	}
	if contract.Goal != validContract().Goal || len(contract.DoneCriteria) != 2 {
		t.Fatalf("Author() contract = %#v", contract)
	}
	if runner.invocation.Agent != packetauthor.SpecialistAgentName {
		t.Fatalf("invocation.Agent = %q", runner.invocation.Agent)
	}
	var input map[string]any
	if err := json.Unmarshal(runner.invocation.Input, &input); err != nil {
		t.Fatalf("request JSON = %s: %v", runner.invocation.Input, err)
	}
	contractInput := input["contract"].(map[string]any)
	for _, forbidden := range []string{"target_sha", "expected_parent_sha", "dispatch", "worktree", "acceptance", "promotion"} {
		if _, exists := contractInput[forbidden]; exists {
			t.Fatalf("specialist request grants authority through %q: %s", forbidden, runner.invocation.Input)
		}
	}
	if strings.Contains(string(runner.response.Output), "# Goal") {
		t.Fatal("fixture must contain typed data, not compiler-rendered Markdown")
	}
}

func TestSpecialistOutputRejectsAuthorityAndUntrustedRendering(t *testing.T) {
	tests := []struct {
		name string
		body string
		code string
	}{
		{"target SHA", `{"version":"packet-author-output/v1","target_sha":"abc","contract":{}}`, packetauthor.CodeSpecialistAuthority},
		{"target binding", `{"version":"packet-author-output/v1","binding":{},"contract":{}}`, packetauthor.CodeSpecialistAuthority},
		{"dispatch", `{"version":"packet-author-output/v1","dispatch":true,"contract":{}}`, packetauthor.CodeSpecialistAuthority},
		{"worktree", `{"version":"packet-author-output/v1","worktree_path":"/tmp/x","contract":{}}`, packetauthor.CodeSpecialistAuthority},
		{"integration", `{"version":"packet-author-output/v1","integration":true,"contract":{}}`, packetauthor.CodeSpecialistAuthority},
		{"Acceptance", `{"version":"packet-author-output/v1","acceptance":"allow","contract":{}}`, packetauthor.CodeSpecialistAuthority},
		{"Promotion", `{"version":"packet-author-output/v1","promotion":true,"contract":{}}`, packetauthor.CodeSpecialistAuthority},
		{"rendered Markdown", `{"version":"packet-author-output/v1","markdown":"# Goal","contract":{}}`, packetauthor.CodeSpecialistRender},
		{"frontmatter", `{"version":"packet-author-output/v1","frontmatter":"---","contract":{}}`, packetauthor.CodeSpecialistRender},
		{"unknown", strings.Replace(string(validSpecialistOutput()), `,"contract":`, `,"surprise":true,"contract":`, 1), packetauthor.CodeSpecialistOutput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := packetauthor.DecodeSpecialistOutput([]byte(tt.body))
			assertSpecialistCode(t, err, tt.code)
		})
	}
}

func TestSpecialistOutputRejectsMalformedAndDuplicateData(t *testing.T) {
	tests := []struct {
		name string
		body string
		code string
	}{
		{"malformed", `{"version":`, packetauthor.CodeSpecialistOutput},
		{"duplicate field", `{"version":"packet-author-output/v1","version":"packet-author-output/v1","contract":{}}`, packetauthor.CodeSpecialistDuplicate},
		{"duplicate nested field", `{"version":"packet-author-output/v1","contract":{"goal":"a","goal":"b"}}`, packetauthor.CodeSpecialistDuplicate},
		{"multiple values", string(validSpecialistOutput()) + ` {}`, packetauthor.CodeSpecialistDuplicate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := packetauthor.DecodeSpecialistOutput([]byte(tt.body))
			assertSpecialistCode(t, err, tt.code)
		})
	}
}

func TestSpecialistAdapterRejectsFallbackIdentity(t *testing.T) {
	request, err := packetauthor.NewSpecialistRequest(validContract())
	if err != nil {
		t.Fatal(err)
	}
	for _, identity := range []string{"", "default", "build", "lucind-packet-author-fallback"} {
		t.Run(identity, func(t *testing.T) {
			runner := &fakeSpecialistRunner{response: packetauthor.SpecialistResponse{Identity: identity, Output: validSpecialistOutput()}}
			_, err := (packetauthor.SpecialistAdapter{Runner: runner}).Author(context.Background(), request)
			assertSpecialistCode(t, err, packetauthor.CodeSpecialistIdentity)
		})
	}
}

func TestSpecialistRequestRejectsTargetClaims(t *testing.T) {
	contract := validContract()
	contract.TargetClaims = map[string]string{"expected_parent_sha": sha('a')}
	_, err := packetauthor.NewSpecialistRequest(contract)
	assertSpecialistCode(t, err, packetauthor.CodeSpecialistAuthority)
}

func assertSpecialistCode(t *testing.T, err error, want string) {
	t.Helper()
	var specialistErr *packetauthor.SpecialistError
	if !errors.As(err, &specialistErr) || specialistErr.Code != want {
		t.Fatalf("error = %v, want SpecialistError code %q", err, want)
	}
}

func validSpecialistOutput() []byte {
	return []byte(`{"version":"packet-author-output/v1","contract":{"version":"packet-author/v1","route_intent":"apply","mode":"write","write_paths":["internal/z.go","internal/a.go"],"read_only_paths":["openspec/spec.md"],"goal":"Compile delegated packet contracts.","done_criteria":["Manual bodies remain byte-identical.","Replay is deterministic."],"hard_stops":["Do not dispatch work."],"result":{"path":".lucind/result.json","schema":".lucind/result.schema.json"}}}`)
}
