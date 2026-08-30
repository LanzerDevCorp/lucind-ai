package packetauthor

import (
	"strings"
	"testing"
)

func TestCompareClassifiesNondeterministicSecondCompilation(t *testing.T) {
	contract := Contract{
		Version: ContractVersion, RouteIntent: "apply", Mode: ModeWrite,
		WritePaths: []string{"internal/file.go"}, ReadOnlyPaths: []string{"docs/input.md"},
		Goal: "Compare typed packet authoring.", DoneCriteria: []string{"a criterion"}, HardStops: []string{"do not dispatch"},
		Result: ResultObligations{Path: ".lucind/result.json", Schema: ".lucind/result.schema.json"},
	}
	binding := TargetBinding{LegacyMain: &LegacyMainTarget{ExpectedParentSHA: strings.Repeat("a", 40), LiveParentSHA: strings.Repeat("a", 40)}}
	manual, err := Compile(contract, binding)
	if err != nil {
		t.Fatal(err)
	}
	compileCalls := 0
	compile := func(contract Contract, binding TargetBinding) (Artifact, error) {
		compileCalls++
		artifact, err := Compile(contract, binding)
		if err == nil && compileCalls == 2 {
			artifact.Body = append(artifact.Body, []byte("nondeterministic replay\n")...)
		}
		return artifact, err
	}

	evidence := compareWithCompiler(manual, contract, binding, compile)
	if compileCalls != 2 {
		t.Fatalf("compiler calls = %d, want 2", compileCalls)
	}
	if evidence.ReplayStable || evidence.FailureClass != ShadowFailureDeterministicUnstable || !evidence.ManualSelected {
		t.Fatalf("replay evidence = %+v", evidence)
	}
}
