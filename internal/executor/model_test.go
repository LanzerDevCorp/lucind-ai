package executor_test

import (
	"reflect"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
)

func TestDefaultModel(t *testing.T) {
	want := "gemini-3.7-flash-high"
	if got := executor.DefaultModel(); got != want {
		t.Errorf("DefaultModel() = %q, want %q", got, want)
	}
}

func TestKnownModels(t *testing.T) {
	want := []string{
		"gemini-3.7-flash-high",
		"gemini-3.8-flash-high",
		"gemini-3.8-flash-medium",
		"gemini-3.1-pro-high",
		"claude-opus-4-6-thinking",
	}
	got := executor.KnownModels()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("KnownModels() = %v, want %v", got, want)
	}
}

func TestIsValidModel(t *testing.T) {
	for _, m := range executor.KnownModels() {
		if !executor.IsValidModel(m) {
			t.Errorf("IsValidModel(%q) = false, want true", m)
		}
	}
	for _, m := range []string{"unknown-model", "", "gpt-4", "gemini-1.5-pro"} {
		if executor.IsValidModel(m) {
			t.Errorf("IsValidModel(%q) = true, want false", m)
		}
	}
}

func TestResolveModel(t *testing.T) {
	t.Run("default when flag and env empty", func(t *testing.T) {
		t.Setenv("LUCIND_AGY_MODEL", "")
		got, err := executor.ResolveModel("")
		if err != nil {
			t.Fatalf("ResolveModel(\"\") error = %v, want nil", err)
		}
		if got != executor.DefaultModel() {
			t.Errorf("ResolveModel(\"\") = %q, want default %q", got, executor.DefaultModel())
		}
	})

	t.Run("env var used when flag empty", func(t *testing.T) {
		t.Setenv("LUCIND_AGY_MODEL", "gemini-3.8-flash-high")
		got, err := executor.ResolveModel("")
		if err != nil {
			t.Fatalf("ResolveModel(\"\") error = %v, want nil", err)
		}
		if got != "gemini-3.8-flash-high" {
			t.Errorf("ResolveModel(\"\") = %q, want %q", got, "gemini-3.8-flash-high")
		}
	})

	t.Run("flag overrides env var", func(t *testing.T) {
		t.Setenv("LUCIND_AGY_MODEL", "gemini-3.8-flash-high")
		got, err := executor.ResolveModel("claude-opus-4-6-thinking")
		if err != nil {
			t.Fatalf("ResolveModel flag error = %v, want nil", err)
		}
		if got != "claude-opus-4-6-thinking" {
			t.Errorf("ResolveModel = %q, want %q", got, "claude-opus-4-6-thinking")
		}
	})

	t.Run("invalid flag returns error", func(t *testing.T) {
		t.Setenv("LUCIND_AGY_MODEL", "")
		_, err := executor.ResolveModel("unsupported-model")
		if err == nil {
			t.Fatal("ResolveModel(\"unsupported-model\") error = nil, want error")
		}
	})

	t.Run("invalid env returns error when flag empty", func(t *testing.T) {
		t.Setenv("LUCIND_AGY_MODEL", "invalid-model")
		_, err := executor.ResolveModel("")
		if err == nil {
			t.Fatal("ResolveModel(\"\") with invalid env error = nil, want error")
		}
	})
}
