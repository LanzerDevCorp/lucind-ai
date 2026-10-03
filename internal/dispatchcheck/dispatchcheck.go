// Package dispatchcheck validates a packet's declared route against signals
// that lucind-ai can compute before dispatch.
package dispatchcheck

import (
	"fmt"
	"strings"

	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/risk"
)

type Signals struct {
	AllowedPathCount int
	NewFile          bool      // true if any allowed path may create a file
	Tier             risk.Tier // from risk.ClassifyPaths(allowed paths)
	Reasons          []string  // why each hard signal fired, e.g. "allowed_paths:3", "new_file:docs/x.md", "risk_tier:high"
}

type Action string

const (
	ActionAccept  Action = "accept"
	ActionUpgrade Action = "upgrade"
	ActionReject  Action = "reject"
)

type Verdict struct {
	Action  Action
	Route   string   // route to use after the check ("" when the packet declared none)
	Reasons []string // human-readable, stable order
}

// PathExists reports whether a repository-relative path exists at the dispatch base.
type PathExists func(path string) (bool, error)

// ComputeSignals computes observable routing signals from a packet and base state.
func ComputeSignals(p packet.Packet, exists PathExists) Signals {
	tierResult := risk.ClassifyPaths(p.AllowedPaths)

	// A ReadOnly packet has NewFile=false and count 0 for threshold purposes (it writes nothing)
	// but keeps its computed tier.
	if p.ReadOnly {
		var reasons []string
		if tierResult.Tier == risk.TierHigh {
			reasons = append(reasons, "risk_tier:high")
		}
		return Signals{
			AllowedPathCount: 0,
			NewFile:          false,
			Tier:             tierResult.Tier,
			Reasons:          reasons,
		}
	}

	count := len(p.AllowedPaths)
	var newFiles []string
	for _, path := range p.AllowedPaths {
		if isNewFile(path, exists) {
			newFiles = append(newFiles, path)
		}
	}
	newFile := len(newFiles) > 0

	var reasons []string
	if count >= 2 {
		reasons = append(reasons, fmt.Sprintf("allowed_paths:%d", count))
	}
	for _, nf := range newFiles {
		reasons = append(reasons, fmt.Sprintf("new_file:%s", nf))
	}
	if tierResult.Tier == risk.TierHigh {
		reasons = append(reasons, "risk_tier:high")
	}

	return Signals{
		AllowedPathCount: count,
		NewFile:          newFile,
		Tier:             tierResult.Tier,
		Reasons:          reasons,
	}
}

func isNewFile(path string, exists PathExists) bool {
	if strings.HasSuffix(path, "/") || strings.ContainsAny(path, "*?[") {
		return true
	}
	if exists == nil {
		return true
	}
	ok, err := exists(path)
	if err != nil || !ok {
		return true
	}
	return false
}

// Check evaluates a packet against dispatch threshold rules.
func Check(p packet.Packet, exists PathExists) Verdict {
	if p.Route == "" {
		return Verdict{
			Action:  ActionAccept,
			Route:   "",
			Reasons: []string{"no route declared"},
		}
	}

	signals := ComputeSignals(p, exists)
	hasHardSignals := signals.AllowedPathCount >= 2 || signals.NewFile || signals.Tier == risk.TierHigh

	switch p.Route {
	case "inline":
		declared := declaredReasons(p)
		if hasHardSignals || len(declared) > 0 {
			return Verdict{
				Action:  ActionUpgrade,
				Route:   "worker",
				Reasons: append(append([]string(nil), signals.Reasons...), declared...),
			}
		}
		return Verdict{
			Action:  ActionAccept,
			Route:   "inline",
			Reasons: []string{"below dispatch threshold"},
		}

	case "worker":
		if strings.TrimSpace(p.RouteEvidence) == "" {
			return Verdict{
				Action:  ActionReject,
				Route:   "worker",
				Reasons: []string{"route_evidence is required when route is worker or fanout"},
			}
		}
		reasons := signals.Reasons
		if !hasHardSignals {
			reasons = []string{"below dispatch threshold, accepted"}
		}
		return Verdict{
			Action:  ActionAccept,
			Route:   "worker",
			Reasons: reasons,
		}

	case "fanout":
		if !p.ReadOnly && signals.AllowedPathCount < 2 {
			return Verdict{
				Action:  ActionReject,
				Route:   "fanout",
				Reasons: []string{"fanout requires a read-only lane or at least 2 allowed paths"},
			}
		}
		if strings.TrimSpace(p.RouteEvidence) == "" {
			return Verdict{
				Action:  ActionReject,
				Route:   "fanout",
				Reasons: []string{"route_evidence is required when route is worker or fanout"},
			}
		}
		reasons := signals.Reasons
		if !hasHardSignals {
			reasons = []string{"below dispatch threshold, accepted"}
		}
		return Verdict{
			Action:  ActionAccept,
			Route:   "fanout",
			Reasons: reasons,
		}

	default:
		return Verdict{
			Action:  ActionReject,
			Route:   p.Route,
			Reasons: []string{fmt.Sprintf("unknown route: %s", p.Route)},
		}
	}
}

// maxInlineLookups is the evidence budget for inline work: more than this many sequential
// lookups requires a delegated explorer or writer.
const maxInlineLookups = 5

// declaredReasons lists the orchestrator-declared signals that rule out the inline route.
func declaredReasons(p packet.Packet) []string {
	var reasons []string
	if p.Understood != nil && !*p.Understood {
		reasons = append(reasons, "understood:false")
	}
	if p.OpenDesign {
		reasons = append(reasons, "open_design:true")
	}
	if p.EstimatedLookups > maxInlineLookups {
		reasons = append(reasons, fmt.Sprintf("estimated_lookups:%d", p.EstimatedLookups))
	}
	return reasons
}
