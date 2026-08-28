package packetauthor_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/packetauthor"
)

func TestCompareRecordsEquivalentFieldsDigestsAndReplayStability(t *testing.T) {
	binding := compareBinding()
	manual, err := packetauthor.Compile(compareContract(), binding)
	if err != nil {
		t.Fatal(err)
	}

	evidence := packetauthor.Compare(manual, compareContract(), binding)
	if !evidence.Valid || !evidence.Equivalent || !evidence.DigestEqual || !evidence.ReplayStable || !evidence.ManualSelected {
		t.Fatalf("comparison evidence = %+v", evidence)
	}
	if evidence.ManualDigest != manual.Digest || evidence.SpecialistDigest != manual.Digest {
		t.Fatalf("comparison digests = %+v, manual digest %q", evidence, manual.Digest)
	}
	if len(evidence.Differences) != 0 || evidence.FailureClass != packetauthor.ShadowFailureNone {
		t.Fatalf("comparison differences/failure = %+v", evidence)
	}
	manualDigestDrift := manual
	manualDigestDrift.Digest = "manual-digest-drift"
	drift := packetauthor.Compare(manualDigestDrift, compareContract(), binding)
	if !drift.Valid || !drift.Equivalent || drift.DigestEqual || !drift.ReplayStable {
		t.Fatalf("semantic equivalence must be distinct from digest equality: %+v", drift)
	}
}

func TestCompareSortsFieldDifferencesAndClassifiesInvalidShadowAttempts(t *testing.T) {
	binding := compareBinding()
	manual, err := packetauthor.Compile(compareContract(), binding)
	if err != nil {
		t.Fatal(err)
	}
	changed := compareContract()
	changed.Mode = packetauthor.ModeReadOnly
	changed.WritePaths = nil
	changed.DoneCriteria = []string{"a different criterion"}
	evidence := packetauthor.Compare(manual, changed, binding)
	if !evidence.Valid || evidence.Equivalent || evidence.DigestEqual || !evidence.ReplayStable {
		t.Fatalf("changed comparison = %+v", evidence)
	}
	if got := []string{evidence.Differences[0].Field, evidence.Differences[1].Field}; strings.Join(got, ",") != "done_criteria,mode" {
		t.Fatalf("difference order = %v", got)
	}

	runner := specialistRunnerFunc(func(ctx context.Context, _ packetauthor.SpecialistInvocation) (packetauthor.SpecialistResponse, error) {
		return packetauthor.SpecialistResponse{}, context.DeadlineExceeded
	})
	timeout := packetauthor.Observe(context.Background(), manual, compareContract(), runner, binding)
	if timeout.FailureClass != packetauthor.ShadowFailureTimeout || timeout.ManualSelected != true || timeout.Valid {
		t.Fatalf("timeout observation = %+v", timeout)
	}

	fallback := packetauthor.Observe(context.Background(), manual, compareContract(), specialistRunnerFunc(func(context.Context, packetauthor.SpecialistInvocation) (packetauthor.SpecialistResponse, error) {
		return packetauthor.SpecialistResponse{Identity: "default", Output: validSpecialistOutput()}, nil
	}), binding)
	if fallback.FailureClass != packetauthor.ShadowFailureFallbackAgent || fallback.ManualSelected != true {
		t.Fatalf("fallback observation = %+v", fallback)
	}

	invalid := packetauthor.Observe(context.Background(), manual, compareContract(), specialistRunnerFunc(func(context.Context, packetauthor.SpecialistInvocation) (packetauthor.SpecialistResponse, error) {
		return packetauthor.SpecialistResponse{Identity: packetauthor.SpecialistAgentName, Output: []byte(`{"version":`)}, nil
	}), binding)
	if invalid.FailureClass != packetauthor.ShadowFailureInvalidJSON || invalid.ManualSelected != true {
		t.Fatalf("invalid observation = %+v", invalid)
	}
	if errors.Is(invalid.Warning, context.DeadlineExceeded) {
		t.Fatal("shadow evidence must not expose raw runner errors")
	}
	schema := packetauthor.Observe(context.Background(), manual, compareContract(), specialistRunnerFunc(func(context.Context, packetauthor.SpecialistInvocation) (packetauthor.SpecialistResponse, error) {
		return packetauthor.SpecialistResponse{Identity: packetauthor.SpecialistAgentName, Output: []byte(`{"version":"packet-author-output/v1","contract":{}}`)}, nil
	}), binding)
	if schema.FailureClass != packetauthor.ShadowFailureSchemaInvalid || !schema.ManualSelected {
		t.Fatalf("schema observation = %+v", schema)
	}
	if unavailable := packetauthor.Observe(context.Background(), manual, compareContract(), nil, binding); unavailable.FailureClass != packetauthor.ShadowFailureUnavailableRoute || !unavailable.ManualSelected {
		t.Fatalf("unavailable observation = %+v", unavailable)
	}
	invalidContract := compareContract()
	invalidContract.Goal = ""
	compiler := packetauthor.Compare(manual, invalidContract, binding)
	if compiler.FailureClass != packetauthor.ShadowFailureCompilerRejected || compiler.Valid || !compiler.ManualSelected {
		t.Fatalf("compiler rejection = %+v", compiler)
	}
	disabled := packetauthor.Disabled(manual)
	if disabled.FailureClass != packetauthor.ShadowFailureDisabled || !disabled.ManualSelected || disabled.ManualDigest != manual.Digest {
		t.Fatalf("disabled observation = %+v", disabled)
	}
}

type specialistRunnerFunc func(context.Context, packetauthor.SpecialistInvocation) (packetauthor.SpecialistResponse, error)

func (f specialistRunnerFunc) Run(ctx context.Context, invocation packetauthor.SpecialistInvocation) (packetauthor.SpecialistResponse, error) {
	return f(ctx, invocation)
}

func compareContract() packetauthor.Contract {
	return packetauthor.Contract{
		Version: packetauthor.ContractVersion, RouteIntent: "apply", Mode: packetauthor.ModeWrite,
		WritePaths: []string{"internal/z.go", "internal/a.go"}, ReadOnlyPaths: []string{"docs/input.md"},
		Goal: "Compare typed packet authoring.", DoneCriteria: []string{"a criterion", "another criterion"},
		HardStops: []string{"do not dispatch"}, Result: packetauthor.ResultObligations{Path: ".lucind/result.json", Schema: ".lucind/result.schema.json"},
	}
}

func compareBinding() packetauthor.TargetBinding {
	return packetauthor.TargetBinding{LegacyMain: &packetauthor.LegacyMainTarget{
		ExpectedParentSHA: strings.Repeat("a", 40), LiveParentSHA: strings.Repeat("a", 40),
	}}
}
