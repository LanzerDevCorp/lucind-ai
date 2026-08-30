package packetauthor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

// ShadowFailureClass is an observational failure category. None of these
// outcomes changes the manual packet selected for dispatch.
type ShadowFailureClass string

const (
	ShadowFailureNone                  ShadowFailureClass = "none"
	ShadowFailureTimeout               ShadowFailureClass = "timeout"
	ShadowFailureInvalidJSON           ShadowFailureClass = "invalid_json"
	ShadowFailureSchemaInvalid         ShadowFailureClass = "schema_invalid"
	ShadowFailureUnavailableRoute      ShadowFailureClass = "unavailable_route"
	ShadowFailureFallbackAgent         ShadowFailureClass = "fallback_agent"
	ShadowFailureCompilerRejected      ShadowFailureClass = "compiler_rejected"
	ShadowFailureDeterministicUnstable ShadowFailureClass = "deterministic_instability"
	ShadowFailureDisabled              ShadowFailureClass = "disabled"
)

// FieldDifference is a deterministic, normalized field-level comparison.
type FieldDifference struct {
	Field      string `json:"field"`
	Manual     string `json:"manual"`
	Specialist string `json:"specialist"`
}

// Disabled records the explicit no-invocation path while keeping manual
// selection visible to callers and reports.
func Disabled(manual Artifact) ShadowComparison {
	return ShadowComparison{ManualSelected: true, ManualDigest: manual.Digest, FailureClass: ShadowFailureDisabled}
}

// ShadowWarning deliberately contains no runner or SQLite text. It is safe to
// print as an observation while preserving the canonical manual path.
type ShadowWarning struct {
	Code  string             `json:"code"`
	Class ShadowFailureClass `json:"class"`
}

func (w ShadowWarning) Error() string { return fmt.Sprintf("shadow warning: %s (%s)", w.Code, w.Class) }

// ShadowComparison is additive evidence for one specialist attempt.
type ShadowComparison struct {
	Valid            bool               `json:"valid"`
	Equivalent       bool               `json:"equivalent"`
	DigestEqual      bool               `json:"digest_equal"`
	ReplayStable     bool               `json:"replay_stable"`
	ManualSelected   bool               `json:"manual_selected"`
	ManualDigest     string             `json:"manual_digest"`
	SpecialistDigest string             `json:"specialist_digest"`
	Differences      []FieldDifference  `json:"differences"`
	FailureClass     ShadowFailureClass `json:"failure_class"`
	LatencyMS        int64              `json:"latency_ms"`
	ReviewCostMS     int64              `json:"review_cost_ms"`
	Warning          error              `json:"-"`
}

// Compare compiles the typed specialist contract twice and compares it with
// the already-admitted manual artifact. It never returns the specialist
// artifact, so callers cannot accidentally dispatch it.
func Compare(manual Artifact, specialist Contract, binding TargetBinding) ShadowComparison {
	return compareWithCompiler(manual, specialist, binding, Compile)
}

func compareWithCompiler(manual Artifact, specialist Contract, binding TargetBinding, compile func(Contract, TargetBinding) (Artifact, error)) ShadowComparison {
	evidence := ShadowComparison{ManualSelected: true, ManualDigest: manual.Digest, FailureClass: ShadowFailureNone}
	first, err := compile(specialist, binding)
	if err != nil {
		return shadowFailure(evidence, ShadowFailureCompilerRejected, "PA_SHADOW_COMPILER_REJECTED")
	}
	second, err := compile(specialist, binding)
	if err != nil {
		return shadowFailure(evidence, ShadowFailureDeterministicUnstable, "PA_SHADOW_REPLAY_UNSTABLE")
	}
	evidence.Valid = true
	evidence.SpecialistDigest = first.Digest
	evidence.DigestEqual = manual.Digest == first.Digest
	evidence.ReplayStable = string(first.Body) == string(second.Body) && first.Digest == second.Digest && string(first.ContractJSON) == string(second.ContractJSON)
	evidence.Differences = contractDifferences(manual.ContractJSON, first.ContractJSON)
	// Semantic equivalence is intentionally independent from rendered digest
	// equality; the evidence reports both so a byte-level drift is visible.
	evidence.Equivalent = len(evidence.Differences) == 0
	if !evidence.ReplayStable {
		evidence.FailureClass = ShadowFailureDeterministicUnstable
		evidence.Warning = ShadowWarning{Code: "PA_SHADOW_REPLAY_UNSTABLE", Class: evidence.FailureClass}
	}
	return evidence
}

// Observe bounds the specialist adapter and turns all expected failures into
// warning-only evidence. The manual artifact remains selected in every case.
func Observe(ctx context.Context, manual Artifact, source Contract, runner SpecialistRunner, binding TargetBinding) ShadowComparison {
	started := time.Now()
	evidence := ShadowComparison{ManualSelected: true, ManualDigest: manual.Digest, FailureClass: ShadowFailureNone}
	if runner == nil {
		return finishShadowFailure(evidence, ShadowFailureUnavailableRoute, "PA_SHADOW_ROUTE_UNAVAILABLE", started)
	}
	request, err := NewSpecialistRequest(source)
	if err != nil {
		return finishShadowFailure(evidence, ShadowFailureSchemaInvalid, "PA_SHADOW_SCHEMA_INVALID", started)
	}
	response, err := runner.Run(ctx, SpecialistInvocation{Agent: SpecialistAgentName, Input: encodeJSON(request)})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return finishShadowFailure(evidence, ShadowFailureTimeout, "PA_SHADOW_TIMEOUT", started)
		}
		return finishShadowFailure(evidence, ShadowFailureUnavailableRoute, "PA_SHADOW_ROUTE_UNAVAILABLE", started)
	}
	if response.Identity != SpecialistAgentName {
		return finishShadowFailure(evidence, ShadowFailureFallbackAgent, "PA_SHADOW_FALLBACK_AGENT", started)
	}
	contract, err := DecodeSpecialistOutput(response.Output)
	if err != nil {
		class := ShadowFailureSchemaInvalid
		if !json.Valid(response.Output) {
			class = ShadowFailureInvalidJSON
		}
		return finishShadowFailure(evidence, class, "PA_SHADOW_OUTPUT_INVALID", started)
	}
	evidence = Compare(manual, contract, binding)
	evidence.ManualSelected = true
	evidence.LatencyMS = time.Since(started).Milliseconds()
	return evidence
}

func finishShadowFailure(evidence ShadowComparison, class ShadowFailureClass, code string, started time.Time) ShadowComparison {
	evidence = shadowFailure(evidence, class, code)
	evidence.LatencyMS = time.Since(started).Milliseconds()
	return evidence
}

func shadowFailure(evidence ShadowComparison, class ShadowFailureClass, code string) ShadowComparison {
	evidence.Valid = false
	evidence.Equivalent = false
	evidence.DigestEqual = false
	evidence.ReplayStable = false
	evidence.FailureClass = class
	evidence.Warning = ShadowWarning{Code: code, Class: class}
	return evidence
}

func contractDifferences(manualJSON, specialistJSON []byte) []FieldDifference {
	var manual, specialist normalizedContract
	if json.Unmarshal(manualJSON, &manual) != nil || json.Unmarshal(specialistJSON, &specialist) != nil {
		return []FieldDifference{{Field: "contract", Manual: string(manualJSON), Specialist: string(specialistJSON)}}
	}
	fields := []struct {
		name       string
		manual     any
		specialist any
	}{
		{"version", manual.Version, specialist.Version}, {"route_intent", manual.RouteIntent, specialist.RouteIntent},
		{"mode", manual.Mode, specialist.Mode}, {"lane_role", manual.LaneRole, specialist.LaneRole},
		{"adhoc_skills", manual.AdhocSkills, specialist.AdhocSkills}, {"required_skills", manual.RequiredSkills, specialist.RequiredSkills},
		{"write_paths", manual.WritePaths, specialist.WritePaths},
		{"read_only_paths", manual.ReadOnlyPaths, specialist.ReadOnlyPaths}, {"goal", manual.Goal, specialist.Goal},
		{"done_criteria", manual.DoneCriteria, specialist.DoneCriteria}, {"hard_stops", manual.HardStops, specialist.HardStops},
		{"result", manual.Result, specialist.Result},
	}
	differences := make([]FieldDifference, 0)
	for _, field := range fields {
		left, _ := json.Marshal(field.manual)
		right, _ := json.Marshal(field.specialist)
		if string(left) != string(right) {
			differences = append(differences, FieldDifference{Field: field.name, Manual: string(left), Specialist: string(right)})
		}
	}
	sort.Slice(differences, func(i, j int) bool { return differences[i].Field < differences[j].Field })
	return differences
}
