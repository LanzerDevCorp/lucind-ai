package risk

import (
	"fmt"
	"strings"
)

// ClassifyPaths performs a path-only risk classification before any candidate content exists.
// It applies only the path rules of Classify: HIGH path tokens, HIGH sensitive locations,
// and MEDIUM for active content locations. Otherwise, the floor is MEDIUM because content
// cannot be proven passive without bytes. Empty paths fails closed to HIGH.
func ClassifyPaths(paths []string) (result Result) {
	defer func() {
		if r := recover(); r != nil {
			result = Result{
				Tier:    TierHigh,
				Reasons: []string{"classifier_error:panic"},
			}
		}
	}()

	if len(paths) == 0 {
		return Result{
			Tier:    TierHigh,
			Reasons: []string{"classifier_error:no_paths"},
		}
	}

	var reasons []string
	currentTier := TierMedium

	for _, p := range paths {
		for _, tok := range splitTokens(p) {
			lower := strings.ToLower(tok)
			if highTokens[lower] {
				reasons = append(reasons, fmt.Sprintf("path_token:%s:%s", lower, p))
				elevateTier(&currentTier, TierHigh)
			}
		}

		if isSensitivePath(p) {
			reasons = append(reasons, fmt.Sprintf("sensitive_path:%s", p))
			elevateTier(&currentTier, TierHigh)
		}

		if isActiveContentPath(p) {
			reasons = append(reasons, fmt.Sprintf("active_content:%s", p))
			elevateTier(&currentTier, TierMedium)
		}
	}

	return Result{
		Tier:    currentTier,
		Reasons: dedupeAndSort(reasons),
	}
}
