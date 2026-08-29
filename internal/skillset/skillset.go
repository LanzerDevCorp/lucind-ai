// Package skillset implements deterministic three-tier skill derivation,
// default skill budgets, and root-independent packet body digesting.
package skillset

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// DefaultSkillBudget is the default maximum number of skills permitted in a lane.
const DefaultSkillBudget = 3

var (
	// ErrInvalidLaneRole is returned when an unrecognized lane_role is specified.
	ErrInvalidLaneRole = errors.New("skillset: invalid lane role")
	// ErrInvalidSDDPhase is returned when an unrecognized sdd_phase is specified.
	ErrInvalidSDDPhase = errors.New("skillset: invalid sdd phase")
)

var validLaneRoles = map[string]struct{}{
	"lens":       {},
	"synthesis":  {},
	"apply":      {},
	"verify":     {},
	"archive":    {},
	"ultrafixer": {},
	"human":      {},
}

var validSDDPhases = map[string]struct{}{
	"explore":   {},
	"propose":   {},
	"spec":      {},
	"design":    {},
	"tasks":     {},
	"apply":     {},
	"verify":    {},
	"remediate": {},
	"archive":   {},
}

// IsValidLaneRole reports whether role is a recognized lane role.
func IsValidLaneRole(role string) bool {
	_, ok := validLaneRoles[role]
	return ok
}

// IsValidSDDPhase reports whether phase is a recognized SDD phase.
func IsValidSDDPhase(phase string) bool {
	_, ok := validSDDPhases[phase]
	return ok
}

// Derive deterministically derives the union of derived, stack, and ad-hoc skills.
// It returns a deduplicated, lexicographically sorted slice of skill names.
// Derived skills are mandatory and guaranteed in the returned set.
func Derive(sddPhase, laneRole string, stackSkills, adhocSkills []string) ([]string, error) {
	if laneRole != "" && !IsValidLaneRole(laneRole) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidLaneRole, laneRole)
	}
	if sddPhase != "" && !IsValidSDDPhase(sddPhase) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidSDDPhase, sddPhase)
	}

	var derived []string
	// Every lane derives lucind-executor.
	derived = append(derived, "lucind-executor")

	switch laneRole {
	case "lens", "synthesis":
		derived = append(derived, "lucind-fan-out-lens")
		if sddPhase != "" && sddPhase != "remediate" {
			derived = append(derived, "sdd-"+sddPhase)
		}
	case "apply":
		derived = append(derived, "lucind-apply", "sdd-apply")
	case "verify":
		derived = append(derived, "lucind-verify", "sdd-verify")
	case "archive":
		derived = append(derived, "sdd-archive")
	case "ultrafixer", "human":
		// No child or phase skill derived.
	case "":
		if sddPhase != "" && sddPhase != "remediate" {
			switch sddPhase {
			case "apply":
				derived = append(derived, "lucind-apply", "sdd-apply")
			case "verify":
				derived = append(derived, "lucind-verify", "sdd-verify")
			case "archive":
				derived = append(derived, "sdd-archive")
			default:
				derived = append(derived, "sdd-"+sddPhase)
			}
		}
	}

	seen := make(map[string]bool)
	var result []string

	add := func(skills []string) {
		for _, s := range skills {
			s = strings.TrimSpace(s)
			if s == "" || seen[s] {
				continue
			}
			seen[s] = true
			result = append(result, s)
		}
	}

	add(derived)
	add(stackSkills)
	add(adhocSkills)

	sort.Strings(result)
	return result, nil
}

// DigestBody elides the ## Required skills section — the heading line through
// the line before the next ## heading or EOF — leaving the rest byte-identical.
// A body without that heading returns unchanged.
func DigestBody(body string) string {
	if !strings.Contains(body, "## Required skills") {
		return body
	}

	lines := strings.Split(body, "\n")
	var result []string
	inRequiredSkills := false

	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		if !inRequiredSkills {
			if trimmed == "## Required skills" || strings.HasPrefix(trimmed, "## Required skills") {
				inRequiredSkills = true
				continue
			}
			result = append(result, line)
		} else {
			if strings.HasPrefix(trimmed, "## ") {
				inRequiredSkills = false
				result = append(result, line)
			}
		}
	}

	return strings.Join(result, "\n")
}
