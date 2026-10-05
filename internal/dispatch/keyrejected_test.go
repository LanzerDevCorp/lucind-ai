package dispatch_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/dispatch"
	"github.com/LanzerDevCorp/lucind-ai/internal/skillselect"
)

func TestAutoSkillsUnavailableError_IsKeyRejected(t *testing.T) {
	tests := []struct {
		name  string
		cause error
		want  bool
	}{
		{"401", &skillselect.HTTPError{StatusCode: 401, Body: "authentication_error"}, true},
		{"403", &skillselect.HTTPError{StatusCode: 403}, true},
		{"wrapped 401", fmt.Errorf("select skills: %w", &skillselect.HTTPError{StatusCode: 401}), true},
		{"server error is not a rejected key", &skillselect.HTTPError{StatusCode: 500}, false},
		{"rate limit is not a rejected key", &skillselect.HTTPError{StatusCode: 429}, false},
		{"missing key is not a rejected key", skillselect.ErrMissingAPIKey, false},
		{"network error", errors.New("dial tcp: connection refused"), false},
		{"nil cause", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &dispatch.AutoSkillsUnavailableError{Cause: tt.cause}
			if got := err.IsKeyRejected(); got != tt.want {
				t.Errorf("IsKeyRejected() = %v, want %v", got, tt.want)
			}
		})
	}
}
