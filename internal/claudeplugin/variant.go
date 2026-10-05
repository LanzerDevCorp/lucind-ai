package claudeplugin

import (
	"bytes"
	"fmt"
	"strings"
)

// Variant represents a variant of the installed Claude skill.
type Variant string

const (
	// VariantManual represents the manual skills resolution variant (no API key).
	VariantManual Variant = "manual"
	// VariantAuto represents the automatic skills resolution variant (API key configured).
	VariantAuto Variant = "auto"
)

// RenderSkill renders SKILL.md content for the given variant.
// Blocks demarcated by:
// <!-- lucind:variant manual --> ... <!-- /lucind:variant -->
// and
// <!-- lucind:variant auto --> ... <!-- /lucind:variant -->
// are filtered: blocks matching v are kept (without marker lines),
// blocks for other variants are dropped. Unmarked lines are common to both.
// An unbalanced or unknown marker is returned as an error.
func RenderSkill(content []byte, v Variant) ([]byte, error) {
	if v != VariantManual && v != VariantAuto {
		return nil, fmt.Errorf("unknown variant %q; must be %q or %q", v, VariantManual, VariantAuto)
	}

	var lines [][]byte
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			lines = append(lines, content[start:i+1])
			start = i + 1
		}
	}
	if start < len(content) {
		lines = append(lines, content[start:])
	}

	var out [][]byte
	var currentVariant string

	for _, line := range lines {
		trimmed := strings.TrimSpace(string(line))

		if strings.Contains(trimmed, "lucind:variant") {
			if trimmed == "<!-- /lucind:variant -->" {
				if currentVariant == "" {
					return nil, fmt.Errorf("unopened closing marker: %s", trimmed)
				}
				currentVariant = ""
				continue
			}

			if strings.HasPrefix(trimmed, "<!-- lucind:variant ") && strings.HasSuffix(trimmed, "-->") {
				inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "<!-- lucind:variant "), "-->"))
				if inner != string(VariantManual) && inner != string(VariantAuto) {
					return nil, fmt.Errorf("unknown variant marker %q", inner)
				}
				if currentVariant != "" {
					return nil, fmt.Errorf("nested variant marker %q inside %q", inner, currentVariant)
				}
				currentVariant = inner
				continue
			}

			return nil, fmt.Errorf("malformed variant marker: %s", trimmed)
		}

		if currentVariant == "" || Variant(currentVariant) == v {
			out = append(out, line)
		}
	}

	if currentVariant != "" {
		return nil, fmt.Errorf("unclosed variant marker <!-- lucind:variant %s -->", currentVariant)
	}

	return bytes.Join(out, nil), nil
}
