package dispatch_test

import (
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/dispatch"
)

func TestResolveModel_FlagTakesPrecedence(t *testing.T) {
	t.Setenv("LUCIND_AGY_MODEL", "gemini-3.8-flash-medium")
	got, err := dispatch.ResolveModel("gemini-3.8-flash-high")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "gemini-3.8-flash-high"; got != want {
		t.Errorf("ResolveModel() = %q, want %q", got, want)
	}
}

func TestResolveModel_EnvTakesPrecedenceOverDefault(t *testing.T) {
	t.Setenv("LUCIND_AGY_MODEL", "claude-opus-5-5-high")
	got, err := dispatch.ResolveModel("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "claude-opus-5-5-high"; got != want {
		t.Errorf("ResolveModel() = %q, want %q", got, want)
	}
}

func TestResolveModel_Default(t *testing.T) {
	t.Setenv("LUCIND_AGY_MODEL", "")
	got, err := dispatch.ResolveModel("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "gemini-3.8-flash-high"; got != want {
		t.Errorf("ResolveModel() = %q, want %q", got, want)
	}
}

func TestResolveModel_Unrecognized(t *testing.T) {
	_, err := dispatch.ResolveModel("unrecognized-model-foo")
	if err == nil {
		t.Fatalf("expected error for unrecognized model, got nil")
	}
}

func TestResolveModel_AllKnownModels(t *testing.T) {
	known := []string{
		"gemini-3.8-flash-high",
		"gemini-3.8-flash-medium",
		"gemini-3.8-flash-low",
		"gemini-3.7-flash-high",
		"gemini-3.7-flash-medium",
		"gemini-3.7-flash-low",
		"gemini-3.6-flash-high",
		"gemini-3.6-flash-medium",
		"gemini-3.6-flash-low",
		"gemini-3.1-pro-high",
		"gemini-3.1-pro-low",
		"claude-opus-5-5-low",
		"claude-opus-5-5-medium",
		"claude-opus-5-5-high",
		"claude-sonnet-5-5-low",
		"claude-sonnet-5-5-medium",
		"claude-sonnet-5-5-high",
		"gpt-oss-120b-medium",
	}
	for _, m := range known {
		got, err := dispatch.ResolveModel(m)
		if err != nil {
			t.Errorf("ResolveModel(%q) unexpected error: %v", m, err)
		}
		if got != m {
			t.Errorf("ResolveModel(%q) = %q, want %q", m, got, m)
		}
	}
}

func TestResolveModel_InvalidEnv(t *testing.T) {
	t.Setenv("LUCIND_AGY_MODEL", "invalid-model-from-env")
	_, err := dispatch.ResolveModel("")
	if err == nil {
		t.Fatalf("expected error for invalid env model, got nil")
	}
}
