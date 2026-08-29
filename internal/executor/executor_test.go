package executor

import (
	"testing"
	"time"
)

func TestRequestProgressDefaultsToNil(t *testing.T) {
	var req Request
	if req.Progress != nil {
		t.Fatal("zero-value Request.Progress must be nil")
	}
}

func TestRequestProgressCarriesCanonicalEvent(t *testing.T) {
	progress := make(chan ProgressEvent, 1)
	req := Request{Progress: progress}
	want := ProgressEvent{Message: "running checks", At: time.Unix(42, 0)}

	req.Progress <- want
	if got := <-progress; got != want {
		t.Fatalf("progress event = %#v, want %#v", got, want)
	}
}

func TestRequestEnvInjectsRequiredSkills(t *testing.T) {
	req := Request{
		RequiredSkills: []string{"lucind-executor", "lucind-apply"},
	}
	env := requestEnv(req)
	want := "LUCIND_REQUIRED_SKILLS=[\"lucind-executor\",\"lucind-apply\"]"

	found := false
	for _, entry := range env {
		if entry == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("requestEnv() missing expected %q in %v", want, env)
	}
}

func TestRequestEnvStripsInheritedRequiredSkills(t *testing.T) {
	t.Setenv("LUCIND_REQUIRED_SKILLS", `["leaked-skill"]`)

	t.Run("with new skills", func(t *testing.T) {
		req := Request{
			RequiredSkills: []string{"lucind-verify"},
		}
		env := requestEnv(req)
		want := "LUCIND_REQUIRED_SKILLS=[\"lucind-verify\"]"

		foundWant := false
		for _, entry := range env {
			if entry == want {
				foundWant = true
			}
			if entry == `LUCIND_REQUIRED_SKILLS=["leaked-skill"]` {
				t.Fatalf("requestEnv() leaked inherited LUCIND_REQUIRED_SKILLS")
			}
		}
		if !foundWant {
			t.Fatalf("requestEnv() missing expected %q", want)
		}
	})

	t.Run("with empty skills", func(t *testing.T) {
		req := Request{
			RequiredSkills: nil,
		}
		env := requestEnv(req)
		for _, entry := range env {
			if entry == `LUCIND_REQUIRED_SKILLS=["leaked-skill"]` {
				t.Fatalf("requestEnv() leaked inherited LUCIND_REQUIRED_SKILLS when skills empty")
			}
		}
	})
}

func TestRequestEnvEmptyRequiredSkillsOmitted(t *testing.T) {
	req := Request{
		RequiredSkills: []string{},
	}
	env := requestEnv(req)
	for _, entry := range env {
		if len(entry) >= 22 && entry[:22] == "LUCIND_REQUIRED_SKILLS" {
			t.Fatalf("requestEnv() should not contain LUCIND_REQUIRED_SKILLS when empty, got %q", entry)
		}
	}
}
