package dispatch_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/dispatch"
	"github.com/LanzerDevCorp/lucind-ai/internal/skillselect"
)

func TestAutoSkillsRemediationLines(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want []string
	}{
		{
			name: "missing key",
			err:  &dispatch.AutoSkillsUnavailableError{Cause: skillselect.ErrMissingAPIKey},
			want: []string{
				"lucind-ai: auto-skills unavailable: " + skillselect.ErrMissingAPIKey.Error(),
				"lucind-ai: no lane was created. Fallback: add a \"## Skills to load before work\" section with absolute SKILL.md paths to the prompt and dispatch again (a hand-written section skips Jev).",
				"lucind-ai: to store the key run lucind-ai install, or put TYPESAFE_API_KEY=... in ~/.config/lucind/env",
			},
		},
		{
			name: "rejected key",
			err:  &dispatch.AutoSkillsUnavailableError{Cause: &skillselect.HTTPError{StatusCode: 401}},
			want: []string{
				"lucind-ai: auto-skills unavailable: " + (&skillselect.HTTPError{StatusCode: 401}).Error(),
				"lucind-ai: no lane was created. Fallback: add a \"## Skills to load before work\" section with absolute SKILL.md paths to the prompt and dispatch again (a hand-written section skips Jev).",
				"lucind-ai: the server rejected the API key; correct TYPESAFE_API_KEY (environment variable or ~/.config/lucind/env) or run lucind-ai install --reset-key (plain lucind-ai install never replaces an existing key)",
			},
		},
		{
			name: "generic AutoSkillsUnavailableError",
			err:  &dispatch.AutoSkillsUnavailableError{Cause: errors.New("network timeout")},
			want: []string{
				"lucind-ai: auto-skills unavailable: network timeout",
				"lucind-ai: no lane was created. Fallback: add a \"## Skills to load before work\" section with absolute SKILL.md paths to the prompt and dispatch again (a hand-written section skips Jev).",
			},
		},
		{
			name: "bare ErrAutoSkillsUnavailable",
			err:  dispatch.ErrAutoSkillsUnavailable,
			want: []string{
				"lucind-ai: auto-skills unavailable: auto-skills unavailable",
				"lucind-ai: no lane was created. Fallback: add a \"## Skills to load before work\" section with absolute SKILL.md paths to the prompt and dispatch again (a hand-written section skips Jev).",
			},
		},
		{
			name: "unrelated error returns nil",
			err:  errors.New("unrelated disk error"),
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dispatch.AutoSkillsRemediationLines(tt.err)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AutoSkillsRemediationLines() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
