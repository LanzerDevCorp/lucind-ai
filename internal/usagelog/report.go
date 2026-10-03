package usagelog

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Target share constants for the three core providers.
const (
	TargetAgy    = 0.60
	TargetCursor = 0.15
	TargetClaude = 0.25
)

// ProviderTotals aggregates usage metrics for a single provider.
type ProviderTotals struct {
	Provider           string  `json:"provider"`
	Calls              int     `json:"calls"`
	TotalTokens        int64   `json:"total_tokens"`
	CostUSD            float64 `json:"cost_usd"`
	TokensKnownCalls   int     `json:"tokens_known_calls"`
	TokensUnknownCalls int     `json:"tokens_unknown_calls"`
}

// ModelTotals aggregates usage metrics for a model.
type ModelTotals struct {
	Model       string  `json:"model"`
	Calls       int     `json:"calls"`
	TotalTokens int64   `json:"total_tokens"`
	CostUSD     float64 `json:"cost_usd"`
}

// LaneRoleTotals aggregates usage metrics for a lane role.
type LaneRoleTotals struct {
	LaneRole    string  `json:"lane_role"`
	Calls       int     `json:"calls"`
	TotalTokens int64   `json:"total_tokens"`
	CostUSD     float64 `json:"cost_usd"`
}

// GrandTotals aggregates usage metrics across all providers.
type GrandTotals struct {
	Calls              int     `json:"calls"`
	TotalTokens        int64   `json:"total_tokens"`
	CostUSD            float64 `json:"cost_usd"`
	TokensKnownCalls   int     `json:"tokens_known_calls"`
	TokensUnknownCalls int     `json:"tokens_unknown_calls"`
}

// TargetShareComparison represents one provider's actual vs target share.
type TargetShareComparison struct {
	Provider    string  `json:"provider"`
	TargetShare float64 `json:"target_share"`
	ActualShare float64 `json:"actual_share"`
	DeltaPoints float64 `json:"delta_percentage_points"`
}

// TargetComparison holds the 60/15/25 target comparison across the target providers.
type TargetComparison struct {
	HasTokenData bool                    `json:"has_token_data"`
	TargetTokens int64                   `json:"target_tokens"`
	Providers    []TargetShareComparison `json:"providers"`
}

// Report holds aggregated usage data and target share comparisons.
type Report struct {
	Providers        []ProviderTotals `json:"providers"`
	Models           []ModelTotals    `json:"models"`
	LaneRoles        []LaneRoleTotals `json:"lane_roles"`
	GrandTotal       GrandTotals      `json:"grand_total"`
	SkippedLines     int              `json:"skipped_lines"`
	TargetComparison TargetComparison `json:"target_comparison"`
}

// BuildReport aggregates records filtered by since (zero since includes all)
// and computes provider split targets and grand totals.
func BuildReport(records []Record, since time.Time, skipped int) Report {
	rep := Report{
		SkippedLines: skipped,
	}

	providerMap := map[string]*ProviderTotals{
		"agy":    {Provider: "agy"},
		"cursor": {Provider: "cursor"},
		"claude": {Provider: "claude"},
	}
	modelMap := make(map[string]*ModelTotals)
	roleMap := make(map[string]*LaneRoleTotals)

	for _, r := range records {
		if !since.IsZero() && r.TS.Before(since) {
			continue
		}

		prov := r.Provider
		if prov == "" {
			if r.Executor != "" {
				prov = Provider(r.Executor)
			} else {
				prov = "unknown"
			}
		}

		pt, ok := providerMap[prov]
		if !ok {
			pt = &ProviderTotals{Provider: prov}
			providerMap[prov] = pt
		}
		pt.Calls++
		pt.TotalTokens += r.TotalTokens
		pt.CostUSD += r.CostUSD
		if r.TokensKnown {
			pt.TokensKnownCalls++
		} else {
			pt.TokensUnknownCalls++
		}

		rep.GrandTotal.Calls++
		rep.GrandTotal.TotalTokens += r.TotalTokens
		rep.GrandTotal.CostUSD += r.CostUSD
		if r.TokensKnown {
			rep.GrandTotal.TokensKnownCalls++
		} else {
			rep.GrandTotal.TokensUnknownCalls++
		}

		modelKey := r.Model
		if modelKey == "" {
			modelKey = "(default)"
		}
		mt, ok := modelMap[modelKey]
		if !ok {
			mt = &ModelTotals{Model: modelKey}
			modelMap[modelKey] = mt
		}
		mt.Calls++
		mt.TotalTokens += r.TotalTokens
		mt.CostUSD += r.CostUSD

		roleKey := r.LaneRole
		if roleKey == "" {
			roleKey = "(none)"
		}
		rt, ok := roleMap[roleKey]
		if !ok {
			rt = &LaneRoleTotals{LaneRole: roleKey}
			roleMap[roleKey] = rt
		}
		rt.Calls++
		rt.TotalTokens += r.TotalTokens
		rt.CostUSD += r.CostUSD
	}

	// Providers ordered: agy, cursor, claude, then others sorted alphabetically
	rep.Providers = append(rep.Providers, *providerMap["agy"], *providerMap["cursor"], *providerMap["claude"])
	var otherProviders []string
	for k := range providerMap {
		if k != "agy" && k != "cursor" && k != "claude" {
			otherProviders = append(otherProviders, k)
		}
	}
	sort.Strings(otherProviders)
	for _, p := range otherProviders {
		rep.Providers = append(rep.Providers, *providerMap[p])
	}

	// Target share calculation across agy, cursor, claude
	targetTotalTokens := providerMap["agy"].TotalTokens + providerMap["cursor"].TotalTokens + providerMap["claude"].TotalTokens
	targets := []struct {
		name   string
		target float64
	}{
		{"agy", TargetAgy},
		{"cursor", TargetCursor},
		{"claude", TargetClaude},
	}

	rep.TargetComparison.TargetTokens = targetTotalTokens
	if targetTotalTokens > 0 {
		rep.TargetComparison.HasTokenData = true
		for _, tgt := range targets {
			pt := providerMap[tgt.name]
			actual := float64(pt.TotalTokens) / float64(targetTotalTokens)
			delta := (actual - tgt.target) * 100.0
			rep.TargetComparison.Providers = append(rep.TargetComparison.Providers, TargetShareComparison{
				Provider:    tgt.name,
				TargetShare: tgt.target,
				ActualShare: actual,
				DeltaPoints: delta,
			})
		}
	} else {
		rep.TargetComparison.HasTokenData = false
		for _, tgt := range targets {
			rep.TargetComparison.Providers = append(rep.TargetComparison.Providers, TargetShareComparison{
				Provider:    tgt.name,
				TargetShare: tgt.target,
				ActualShare: 0.0,
				DeltaPoints: 0.0,
			})
		}
	}

	// Models sorted
	var modelNames []string
	for k := range modelMap {
		modelNames = append(modelNames, k)
	}
	sort.Strings(modelNames)
	for _, k := range modelNames {
		rep.Models = append(rep.Models, *modelMap[k])
	}

	// Lane roles sorted
	var roleNames []string
	for k := range roleMap {
		roleNames = append(roleNames, k)
	}
	sort.Strings(roleNames)
	for _, k := range roleNames {
		rep.LaneRoles = append(rep.LaneRoles, *roleMap[k])
	}

	return rep
}

// JSON returns a stable, formatted JSON representation of the Report.
func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// Text formats the Report as an aligned plain-text table.
func (r Report) Text() string {
	var sb strings.Builder

	sb.WriteString("=== Lucind-ai Usage Report ===\n\n")

	sb.WriteString("Providers:\n")
	sb.WriteString(fmt.Sprintf("  %-14s %6s %12s %14s %10s\n", "Provider", "Calls", "Known/Unk", "Total Tokens", "Cost USD"))
	for _, p := range r.Providers {
		knownStr := fmt.Sprintf("%d/%d", p.TokensKnownCalls, p.TokensUnknownCalls)
		sb.WriteString(fmt.Sprintf("  %-14s %6d %12s %14d %10.4f\n", p.Provider, p.Calls, knownStr, p.TotalTokens, p.CostUSD))
	}
	sb.WriteString("  " + strings.Repeat("-", 60) + "\n")
	grandKnownStr := fmt.Sprintf("%d/%d", r.GrandTotal.TokensKnownCalls, r.GrandTotal.TokensUnknownCalls)
	sb.WriteString(fmt.Sprintf("  %-14s %6d %12s %14d %10.4f\n\n", "Grand Total", r.GrandTotal.Calls, grandKnownStr, r.GrandTotal.TotalTokens, r.GrandTotal.CostUSD))

	sb.WriteString("Target Split (agy 60% / cursor 15% / claude 25%):\n")
	if !r.TargetComparison.HasTokenData {
		sb.WriteString("  no token data\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("  %-10s %8s %8s %10s\n", "Provider", "Target", "Actual", "Delta"))
		for _, comp := range r.TargetComparison.Providers {
			deltaStr := fmt.Sprintf("%+.1f%%", comp.DeltaPoints)
			sb.WriteString(fmt.Sprintf("  %-10s %7.1f%% %7.1f%% %10s\n",
				comp.Provider, comp.TargetShare*100, comp.ActualShare*100, deltaStr))
		}
		sb.WriteString("\n")
	}

	if len(r.Models) > 0 {
		sb.WriteString("Models:\n")
		sb.WriteString(fmt.Sprintf("  %-32s %6s %14s %10s\n", "Model", "Calls", "Total Tokens", "Cost USD"))
		for _, m := range r.Models {
			sb.WriteString(fmt.Sprintf("  %-32s %6d %14d %10.4f\n", m.Model, m.Calls, m.TotalTokens, m.CostUSD))
		}
		sb.WriteString("\n")
	}

	if len(r.LaneRoles) > 0 {
		sb.WriteString("Lane Roles:\n")
		sb.WriteString(fmt.Sprintf("  %-20s %6s %14s %10s\n", "Role", "Calls", "Total Tokens", "Cost USD"))
		for _, lr := range r.LaneRoles {
			sb.WriteString(fmt.Sprintf("  %-20s %6d %14d %10.4f\n", lr.LaneRole, lr.Calls, lr.TotalTokens, lr.CostUSD))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Skipped lines: %d\n\n", r.SkippedLines))
	sb.WriteString("Note: Only lane calls made through lucind-ai are logged, so orchestrator Claude usage outside claude lanes is not visible.\n")

	return sb.String()
}
