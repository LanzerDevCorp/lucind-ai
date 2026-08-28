package main

import (
	"context"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/packetauthor"
)

type fakePacketAuthorRunner struct {
	invocation packetauthor.SpecialistInvocation
}

func (f *fakePacketAuthorRunner) Run(_ context.Context, invocation packetauthor.SpecialistInvocation) (packetauthor.SpecialistResponse, error) {
	f.invocation = invocation
	return packetauthor.SpecialistResponse{
		Identity: packetauthor.SpecialistAgentName,
		Output:   []byte(`{"version":"packet-author-output/v1","contract":{"version":"packet-author/v1","route_intent":"apply","mode":"write","write_paths":["internal/packetauthor"],"read_only_paths":["openspec/changes/delegated-packet-authoring"],"goal":"Author typed contract data.","done_criteria":["Typed output validates."],"hard_stops":["Do not choose targets."],"result":{"path":".lucind/result.json","schema":".lucind/result.schema.json"}}}`),
	}, nil
}

func TestCompileSpecialistPacketKeepsTargetAuthorityInTrustedCompiler(t *testing.T) {
	runner := &fakePacketAuthorRunner{}
	source := validPacketAuthorSource()
	binding := packetauthor.TargetBinding{Feature: &packetauthor.FeatureTarget{
		Feature: "delegated-packet-authoring", ParentRef: "refs/heads/feature/delegated-packet-authoring",
		BaseSHA: strings.Repeat("a", 40), ExpectedParentSHA: strings.Repeat("b", 40), LiveParentSHA: strings.Repeat("b", 40),
	}}

	artifact, err := compileSpecialistPacket(context.Background(), runner, source, binding)
	if err != nil {
		t.Fatalf("compileSpecialistPacket() error = %v", err)
	}
	if artifact.Binding.ExpectedParentSHA != strings.Repeat("b", 40) || !strings.HasPrefix(string(artifact.Body), "# Goal\n") {
		t.Fatalf("trusted compiler artifact = %#v", artifact)
	}
	request := string(runner.invocation.Input)
	for _, secret := range []string{strings.Repeat("a", 40), strings.Repeat("b", 40), "parent_ref", "feature"} {
		if strings.Contains(request, secret) {
			t.Fatalf("specialist received target authority %q in %s", secret, request)
		}
	}
}

func validPacketAuthorSource() packetauthor.Contract {
	return packetauthor.Contract{
		Version: packetauthor.ContractVersion, RouteIntent: "apply", Mode: packetauthor.ModeWrite,
		WritePaths: []string{"internal/packetauthor"}, ReadOnlyPaths: []string{"openspec/changes/delegated-packet-authoring"},
		Goal: "Author typed contract data.", DoneCriteria: []string{"Typed output validates."}, HardStops: []string{"Do not choose targets."},
		Result: packetauthor.ResultObligations{Path: ".lucind/result.json", Schema: ".lucind/result.schema.json"},
	}
}

func TestCompileSpecialistPacketRejectsRendererOutput(t *testing.T) {
	runner := specialistRunnerFunc(func(context.Context, packetauthor.SpecialistInvocation) (packetauthor.SpecialistResponse, error) {
		return packetauthor.SpecialistResponse{Identity: packetauthor.SpecialistAgentName, Output: []byte(`{"version":"packet-author-output/v1","markdown":"# forged","contract":{}}`)}, nil
	})
	_, err := compileSpecialistPacket(context.Background(), runner, validPacketAuthorSource(), packetauthor.TargetBinding{})
	if err == nil || !strings.Contains(err.Error(), packetauthor.CodeSpecialistRender) {
		t.Fatalf("compileSpecialistPacket() error = %v", err)
	}
}

func TestShadowObservationKeepsManualArtifactCanonical(t *testing.T) {
	binding := packetauthor.TargetBinding{LegacyMain: &packetauthor.LegacyMainTarget{
		ExpectedParentSHA: strings.Repeat("a", 40), LiveParentSHA: strings.Repeat("a", 40),
	}}
	manual, err := packetauthor.Compile(validPacketAuthorSource(), binding)
	if err != nil {
		t.Fatal(err)
	}
	body := string(manual.Body)
	evidence := packetauthor.Observe(context.Background(), manual, validPacketAuthorSource(), &fakePacketAuthorRunner{}, binding)
	if !evidence.ManualSelected {
		t.Fatalf("shadow evidence selected a non-manual artifact: %+v", evidence)
	}
	if string(manual.Body) != body || manual.Digest == "" {
		t.Fatal("shadow observation changed or discarded the manual artifact")
	}
}

type specialistRunnerFunc func(context.Context, packetauthor.SpecialistInvocation) (packetauthor.SpecialistResponse, error)

func (f specialistRunnerFunc) Run(ctx context.Context, invocation packetauthor.SpecialistInvocation) (packetauthor.SpecialistResponse, error) {
	return f(ctx, invocation)
}
