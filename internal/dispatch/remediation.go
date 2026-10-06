package dispatch

import (
	"errors"
	"fmt"
)

// AutoSkillsRemediationLines formats helpful remediation messages when auto-skills selection fails.
// Returns nil if the error is not related to auto-skills unavailability.
func AutoSkillsRemediationLines(err error) []string {
	if err == nil {
		return nil
	}

	var typedErr *AutoSkillsUnavailableError
	isTyped := errors.As(err, &typedErr)
	if !isTyped && !errors.Is(err, ErrAutoSkillsUnavailable) {
		return nil
	}

	reason := ""
	isMissingKey := false
	isKeyRejected := false
	if typedErr != nil {
		reason = typedErr.RedactedReason()
		isMissingKey = typedErr.IsMissingKey()
		isKeyRejected = typedErr.IsKeyRejected()
	} else {
		reason = err.Error()
	}

	lines := []string{
		fmt.Sprintf("lucind-ai: auto-skills unavailable: %s", reason),
		"lucind-ai: no lane was created. Fallback: add a \"## Skills to load before work\" section with absolute SKILL.md paths to the prompt and dispatch again (a hand-written section skips Jev).",
	}

	if isMissingKey {
		lines = append(lines, "lucind-ai: to store the key run lucind-ai install, or put TYPESAFE_API_KEY=... in ~/.config/lucind/env")
	}
	if isKeyRejected {
		lines = append(lines, "lucind-ai: the server rejected the API key; correct TYPESAFE_API_KEY (environment variable or ~/.config/lucind/env) or run lucind-ai install --reset-key (plain lucind-ai install never replaces an existing key)")
	}

	return lines
}
