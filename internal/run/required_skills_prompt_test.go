package run

import (
	"strings"
	"testing"
)

func TestWithRequiredSkills(t *testing.T) {
	t.Run("appends section and declaration instruction", func(t *testing.T) {
		got := withRequiredSkills("# Goal\nwork\n", []string{"lucind-executor", "lucind-fan-out-lens"})
		for _, want := range []string{"## Required skills\n- lucind-executor\n- lucind-fan-out-lens\n", "`skills_loaded`"} {
			if !strings.Contains(got, want) {
				t.Fatalf("missing %q in %q", want, got)
			}
		}
		if !strings.HasPrefix(got, "# Goal\nwork\n") {
			t.Fatalf("original body not preserved: %q", got)
		}
	})
	t.Run("no skills leaves the body untouched", func(t *testing.T) {
		if got := withRequiredSkills("body", nil); got != "body" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("body that already has the section is untouched", func(t *testing.T) {
		body := "# Goal\n\n## Required skills\n- /x/SKILL.md\n"
		if got := withRequiredSkills(body, []string{"a"}); got != body {
			t.Fatalf("got %q", got)
		}
	})
}
