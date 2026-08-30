package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/packetauthor"
	"github.com/LanzerDevCorp/lucind-ai/internal/skillroots"
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

func helperCreateSkill(t *testing.T, root, skillName string) string {
	t.Helper()
	dir := filepath.Join(root, skillName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillFile := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("# "+skillName+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return skillFile
}

func TestAdmitDispatchBatch_MissingSkillFailsClosed(t *testing.T) {
	tempDir := t.TempDir()
	skillsDir := filepath.Join(tempDir, "skills")
	helperCreateSkill(t, skillsDir, "lucind-executor")
	helperCreateSkill(t, skillsDir, "lucind-fan-out-lens")
	// Note: sdd-propose is intentionally NOT created

	lucindDir := filepath.Join(tempDir, ".lucind")
	if err := os.MkdirAll(lucindDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rootsContent := "roots:\n  - " + skillsDir + "\n"
	if err := os.WriteFile(filepath.Join(lucindDir, "skill-roots.yaml"), []byte(rootsContent), 0o644); err != nil {
		t.Fatal(err)
	}

	origResolve := resolveAdmissionRefSHA
	defer func() { resolveAdmissionRefSHA = origResolve }()
	resolveAdmissionRefSHA = func(context.Context, string, string) (string, error) {
		return strings.Repeat("a", 40), nil
	}

	body := "## Done criteria\n- criterion 1\n\n## Return\nWrite the result envelope to .lucind/result.json in this worktree.\nValidate it against .lucind/result.schema.json before writing.\nAfter you commit, report success.\n"
	inputs := []dispatchAuthoringInput{
		{
			Packet: packet.Packet{
				ID:                "lens-lane",
				Executor:          "agy",
				RoutedBy:          "propose",
				SDDPhase:          "propose",
				LaneRole:          "lens",
				LegacyMain:        true,
				ExpectedParentSHA: strings.Repeat("a", 40),
				Body:              body,
			},
		},
	}

	_, err := admitDispatchBatch(context.Background(), tempDir, inputs)
	if err == nil {
		t.Fatal("expected admitDispatchBatch to fail when required skill is missing, got nil")
	}
	if !errors.Is(err, skillroots.ErrSkillNotFound) {
		t.Fatalf("expected ErrSkillNotFound, got %v", err)
	}
	if !strings.Contains(err.Error(), "sdd-propose") {
		t.Fatalf("expected error message to cite missing skill %q, got %q", "sdd-propose", err.Error())
	}
}

func TestAdmitDispatchBatch_OverBudgetFailsClosed(t *testing.T) {
	tempDir := t.TempDir()
	skillsDir := filepath.Join(tempDir, "skills")
	helperCreateSkill(t, skillsDir, "lucind-executor")
	helperCreateSkill(t, skillsDir, "lucind-fan-out-lens")
	helperCreateSkill(t, skillsDir, "sdd-propose")
	helperCreateSkill(t, skillsDir, "adhoc-tool")

	lucindDir := filepath.Join(tempDir, ".lucind")
	if err := os.MkdirAll(lucindDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rootsContent := "roots:\n  - " + skillsDir + "\n"
	if err := os.WriteFile(filepath.Join(lucindDir, "skill-roots.yaml"), []byte(rootsContent), 0o644); err != nil {
		t.Fatal(err)
	}

	origResolve := resolveAdmissionRefSHA
	defer func() { resolveAdmissionRefSHA = origResolve }()
	resolveAdmissionRefSHA = func(context.Context, string, string) (string, error) {
		return strings.Repeat("a", 40), nil
	}

	body := "## Done criteria\n- criterion 1\n\n## Return\nWrite the result envelope to .lucind/result.json in this worktree.\nValidate it against .lucind/result.schema.json before writing.\nAfter you commit, report success.\n"
	// propose lens derives 3 skills (lucind-executor, lucind-fan-out-lens, sdd-propose) + 1 adhoc = 4 skills > default budget 3
	inputs := []dispatchAuthoringInput{
		{
			Packet: packet.Packet{
				ID:                "lens-lane",
				Executor:          "agy",
				RoutedBy:          "propose",
				SDDPhase:          "propose",
				LaneRole:          "lens",
				AdhocSkills:       []string{"adhoc-tool"},
				LegacyMain:        true,
				ExpectedParentSHA: strings.Repeat("a", 40),
				Body:              body,
			},
		},
	}

	_, err := admitDispatchBatch(context.Background(), tempDir, inputs)
	if err == nil {
		t.Fatal("expected admitDispatchBatch to fail when skills exceed budget, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds budget 3") && !strings.Contains(err.Error(), "budget") {
		t.Fatalf("expected error mentioning budget, got %v", err)
	}
}

func TestAdmitDispatchBatch_CustomBudgetInLucindYAML(t *testing.T) {
	tempDir := t.TempDir()
	skillsDir := filepath.Join(tempDir, "skills")
	helperCreateSkill(t, skillsDir, "lucind-executor")
	helperCreateSkill(t, skillsDir, "lucind-fan-out-lens")
	helperCreateSkill(t, skillsDir, "sdd-propose")
	helperCreateSkill(t, skillsDir, "adhoc-tool")

	lucindDir := filepath.Join(tempDir, ".lucind")
	if err := os.MkdirAll(lucindDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rootsContent := "roots:\n  - " + skillsDir + "\n"
	if err := os.WriteFile(filepath.Join(lucindDir, "skill-roots.yaml"), []byte(rootsContent), 0o644); err != nil {
		t.Fatal(err)
	}
	// Configure skill_budget: 5 in lucind.yaml
	lucindYaml := "skill_budget: 5\n"
	if err := os.WriteFile(filepath.Join(tempDir, "lucind.yaml"), []byte(lucindYaml), 0o644); err != nil {
		t.Fatal(err)
	}

	origResolve := resolveAdmissionRefSHA
	defer func() { resolveAdmissionRefSHA = origResolve }()
	resolveAdmissionRefSHA = func(context.Context, string, string) (string, error) {
		return strings.Repeat("a", 40), nil
	}

	body := "## Done criteria\n- criterion 1\n\n## Return\nWrite the result envelope to .lucind/result.json in this worktree.\nValidate it against .lucind/result.schema.json before writing.\nAfter you commit, report success.\n"
	inputs := []dispatchAuthoringInput{
		{
			Packet: packet.Packet{
				ID:                "lens-lane",
				Executor:          "agy",
				RoutedBy:          "propose",
				SDDPhase:          "propose",
				LaneRole:          "lens",
				AdhocSkills:       []string{"adhoc-tool"},
				LegacyMain:        true,
				ExpectedParentSHA: strings.Repeat("a", 40),
				Body:              body,
			},
		},
	}

	admitted, err := admitDispatchBatch(context.Background(), tempDir, inputs)
	if err != nil {
		t.Fatalf("unexpected admission error with custom budget: %v", err)
	}
	if len(admitted) != 1 {
		t.Fatalf("expected 1 admitted packet, got %d", len(admitted))
	}
	wantSkills := []string{"adhoc-tool", "lucind-executor", "lucind-fan-out-lens", "sdd-propose"}
	if !reflect.DeepEqual(admitted[0].RequiredSkills, wantSkills) {
		t.Fatalf("admitted RequiredSkills = %v, want %v", admitted[0].RequiredSkills, wantSkills)
	}
}

func TestAdmitDispatchBatch_PopulatesRequiredSkillsOnContractAndManual(t *testing.T) {
	tempDir := t.TempDir()
	skillsDir := filepath.Join(tempDir, "skills")
	helperCreateSkill(t, skillsDir, "lucind-executor")
	helperCreateSkill(t, skillsDir, "lucind-apply")
	helperCreateSkill(t, skillsDir, "sdd-apply")

	lucindDir := filepath.Join(tempDir, ".lucind")
	if err := os.MkdirAll(lucindDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rootsContent := "roots:\n  - " + skillsDir + "\n"
	if err := os.WriteFile(filepath.Join(lucindDir, "skill-roots.yaml"), []byte(rootsContent), 0o644); err != nil {
		t.Fatal(err)
	}

	origResolve := resolveAdmissionRefSHA
	defer func() { resolveAdmissionRefSHA = origResolve }()
	resolveAdmissionRefSHA = func(context.Context, string, string) (string, error) {
		return strings.Repeat("a", 40), nil
	}

	manualBody := "## Done criteria\n- criterion 1\n\n## Return\nWrite the result envelope to .lucind/result.json in this worktree.\nValidate it against .lucind/result.schema.json before writing.\nAfter you commit, report success.\n"
	contract := &packetauthor.Contract{
		Version:      packetauthor.ContractVersion,
		RouteIntent:  "apply",
		Mode:         packetauthor.ModeWrite,
		LaneRole:     "apply",
		WritePaths:   []string{"internal/compiled"},
		Goal:         "Apply change safely",
		DoneCriteria: []string{"compiled criterion"},
		HardStops:    []string{"stop on ambiguity"},
		Result:       packetauthor.ResultObligations{Path: ".lucind/result.json", Schema: ".lucind/result.schema.json"},
	}

	inputs := []dispatchAuthoringInput{
		{
			Packet: packet.Packet{
				ID:                "manual-apply",
				Executor:          "agy",
				RoutedBy:          "apply",
				SDDPhase:          "apply",
				LaneRole:          "apply",
				LegacyMain:        true,
				ExpectedParentSHA: strings.Repeat("a", 40),
				Body:              manualBody,
			},
		},
		{
			Packet: packet.Packet{
				ID:                "typed-apply",
				Executor:          "agy",
				RoutedBy:          "apply",
				SDDPhase:          "apply",
				LaneRole:          "apply",
				LegacyMain:        true,
				ExpectedParentSHA: strings.Repeat("a", 40),
			},
			Contract: contract,
		},
	}

	admitted, err := admitDispatchBatch(context.Background(), tempDir, inputs)
	if err != nil {
		t.Fatalf("unexpected admission error: %v", err)
	}
	if len(admitted) != 2 {
		t.Fatalf("expected 2 admitted packets, got %d", len(admitted))
	}
	wantSkills := []string{"lucind-apply", "lucind-executor", "sdd-apply"}
	if !reflect.DeepEqual(admitted[0].RequiredSkills, wantSkills) {
		t.Fatalf("manual RequiredSkills = %v, want %v", admitted[0].RequiredSkills, wantSkills)
	}
	if !reflect.DeepEqual(admitted[1].RequiredSkills, wantSkills) {
		t.Fatalf("typed RequiredSkills = %v, want %v", admitted[1].RequiredSkills, wantSkills)
	}
	if !strings.Contains(admitted[1].Body, "## Required skills") {
		t.Fatalf("typed packet body lacks ## Required skills section: %s", admitted[1].Body)
	}
}

func TestAdmitDispatchBatch_LegacyPhaseOmission(t *testing.T) {
	tempDir := t.TempDir()
	skillsDir := filepath.Join(tempDir, "skills")
	helperCreateSkill(t, skillsDir, "lucind-executor")

	lucindDir := filepath.Join(tempDir, ".lucind")
	if err := os.MkdirAll(lucindDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rootsContent := "roots:\n  - " + skillsDir + "\n"
	if err := os.WriteFile(filepath.Join(lucindDir, "skill-roots.yaml"), []byte(rootsContent), 0o644); err != nil {
		t.Fatal(err)
	}

	origResolve := resolveAdmissionRefSHA
	defer func() { resolveAdmissionRefSHA = origResolve }()
	resolveAdmissionRefSHA = func(context.Context, string, string) (string, error) {
		return strings.Repeat("a", 40), nil
	}

	manualBody := "## Done criteria\n- criterion 1\n\n## Return\nWrite the result envelope to .lucind/result.json in this worktree.\nValidate it against .lucind/result.schema.json before writing.\nAfter you commit, report success.\n"
	inputs := []dispatchAuthoringInput{
		{
			Packet: packet.Packet{
				ID:                "legacy-manual",
				Executor:          "agy",
				RoutedBy:          "legacy",
				SDDPhase:          "unvalidated_custom_phase",
				LaneRole:          "",
				LegacyMain:        true,
				ExpectedParentSHA: strings.Repeat("a", 40),
				Body:              manualBody,
			},
		},
	}

	admitted, err := admitDispatchBatch(context.Background(), tempDir, inputs)
	if err != nil {
		t.Fatalf("unexpected admission error for legacy packet: %v", err)
	}
	if len(admitted) != 1 {
		t.Fatalf("expected 1 admitted packet, got %d", len(admitted))
	}
	wantSkills := []string{"lucind-executor"}
	if !reflect.DeepEqual(admitted[0].RequiredSkills, wantSkills) {
		t.Fatalf("admitted RequiredSkills = %v, want %v", admitted[0].RequiredSkills, wantSkills)
	}
}
