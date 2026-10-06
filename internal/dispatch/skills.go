package dispatch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/skillselect"
)

// ErrAutoSkillsUnavailable is the sentinel error returned when automatic skill selection fails.
var ErrAutoSkillsUnavailable = errors.New("auto-skills unavailable")

// AutoSkillsUnavailableError is the typed error returned when automatic skill selection fails.
type AutoSkillsUnavailableError struct {
	Cause error
}

func (e *AutoSkillsUnavailableError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("auto-skills unavailable: %s", redactAPIKey(e.Cause.Error()))
	}
	return "auto-skills unavailable"
}

func (e *AutoSkillsUnavailableError) Unwrap() error {
	return e.Cause
}

func (e *AutoSkillsUnavailableError) Is(target error) bool {
	if target == ErrAutoSkillsUnavailable {
		return true
	}
	if target == skillselect.ErrMissingAPIKey && e.IsMissingKey() {
		return true
	}
	return false
}

func (e *AutoSkillsUnavailableError) IsMissingKey() bool {
	return errors.Is(e.Cause, skillselect.ErrMissingAPIKey) ||
		(e.Cause != nil && (e.Cause.Error() == skillselect.ErrMissingAPIKey.Error() ||
			strings.Contains(e.Cause.Error(), "TYPESAFE_API_KEY is not set")))
}

// IsKeyRejected reports whether the server answered 401 or 403, which means a key is configured
// but it is not valid. That is different from a missing key: installing again does not replace it.
func (e *AutoSkillsUnavailableError) IsKeyRejected() bool {
	var httpErr *skillselect.HTTPError
	if !errors.As(e.Cause, &httpErr) {
		return false
	}
	return httpErr.StatusCode == http.StatusUnauthorized || httpErr.StatusCode == http.StatusForbidden
}

func (e *AutoSkillsUnavailableError) RedactedReason() string {
	if e.Cause != nil {
		return redactAPIKey(e.Cause.Error())
	}
	return ""
}

func newAutoSkillsUnavailableError(err error) *AutoSkillsUnavailableError {
	if err == nil {
		return &AutoSkillsUnavailableError{}
	}
	if errors.Is(err, skillselect.ErrMissingAPIKey) {
		return &AutoSkillsUnavailableError{Cause: skillselect.ErrMissingAPIKey}
	}
	return &AutoSkillsUnavailableError{Cause: errors.New(redactAPIKey(err.Error()))}
}

// SkillsRecord records the auto-skills selection decisions and outcome for a turn.
type SkillsRecord struct {
	Turn          int                 `json:"turn"`
	Injected      bool                `json:"injected"`
	SkippedReason string              `json:"skipped_reason,omitempty"`
	Error         string              `json:"error,omitempty"`
	Result        *skillselect.Result `json:"result,omitempty"`
}

type skillSelector func(ctx context.Context, repoRoot string, in skillselect.Input) (skillselect.Result, error)

var defaultSkillSelector skillSelector = func(ctx context.Context, repoRoot string, in skillselect.Input) (skillselect.Result, error) {
	regPath := filepath.Join(repoRoot, ".atl", "skill-registry.md")
	f, err := os.Open(regPath)
	if err != nil {
		return skillselect.Result{}, err
	}
	defer func() { _ = f.Close() }()

	skills, err := skillselect.ParseRegistry(f)
	if err != nil {
		return skillselect.Result{}, err
	}

	skills = skillselect.ResolvePaths(skills)

	key := skillselect.ResolveKey()
	if key == "" {
		return skillselect.Result{}, skillselect.ErrMissingAPIKey
	}

	client, err := skillselect.NewClient(key)
	if err != nil {
		return skillselect.Result{}, err
	}

	return skillselect.Select(ctx, client, skills, in, skillselect.DefaultThreshold)
}

var selectSkills = defaultSkillSelector

// SetSkillSelectorForTesting overrides the skill selector function for testing.
func SetSkillSelectorForTesting(fn func(ctx context.Context, repoRoot string, in skillselect.Input) (skillselect.Result, error)) {
	selectSkills = fn
}

// ResetSkillSelectorForTesting restores the default skill selector function.
func ResetSkillSelectorForTesting() {
	selectSkills = defaultSkillSelector
}

func hasSkillsSection(brief string) bool {
	for _, line := range strings.Split(brief, "\n") {
		if strings.TrimRight(line, " \t\r") == "## Skills to load before work" {
			return true
		}
	}
	return false
}

func redactAPIKey(msg string) string {
	key := skillselect.ResolveKey()
	if trimmed := strings.TrimSpace(key); trimmed != "" {
		msg = strings.ReplaceAll(msg, trimmed, "[REDACTED]")
	}
	if key != "" && key != strings.TrimSpace(key) {
		msg = strings.ReplaceAll(msg, key, "[REDACTED]")
	}
	return msg
}

// insertSkillsSection places the skills section right after the brief's leading "# " title, or at
// the top when the first non-empty line is not a title. Loading skills is a precondition, so the
// worker should meet it before the goal and scope instead of after them.
func insertSkillsSection(brief, section string) string {
	sec := strings.TrimRight(section, "\n")
	body := strings.TrimLeft(brief, "\n")
	if strings.TrimSpace(body) == "" {
		return sec
	}

	title, rest, _ := strings.Cut(body, "\n")
	if !strings.HasPrefix(title, "# ") {
		return sec + "\n\n" + body
	}
	rest = strings.TrimLeft(rest, "\n")
	if rest == "" {
		return title + "\n\n" + sec
	}
	return title + "\n\n" + sec + "\n\n" + rest
}

type autoSkillsOutcome struct {
	sectionToInsert string
	skippedReason   string
	injected        bool
	result          *skillselect.Result
}

// selectAutoSkills evaluates the prompt and allow globs without modifying any files.
func selectAutoSkills(ctx context.Context, repoRoot string, brief string, allow []string) (autoSkillsOutcome, error) {
	if hasSkillsSection(brief) {
		return autoSkillsOutcome{
			skippedReason: "brief_has_section",
			injected:      false,
		}, nil
	}

	in := skillselect.Input{
		Brief: brief,
		Allow: allow,
	}

	result, err := selectSkills(ctx, repoRoot, in)
	if err != nil {
		return autoSkillsOutcome{}, newAutoSkillsUnavailableError(err)
	}

	sec := skillselect.Section(result.Decisions)
	if sec != "" {
		return autoSkillsOutcome{
			sectionToInsert: sec,
			injected:        true,
			result:          &result,
		}, nil
	}

	return autoSkillsOutcome{
		skippedReason: "no_skill_selected",
		injected:      false,
		result:        &result,
	}, nil
}

// recordAutoSkills writes skills-<turn>.json once the lane exists on disk.
func recordAutoSkills(repoRoot string, l lane.Lane, outcome autoSkillsOutcome, stderr io.Writer) {
	w := stderr
	if w == nil {
		w = io.Discard
	}

	skillsPath := lane.SkillsFilePath(repoRoot, l)
	record := SkillsRecord{
		Turn:          l.Turn,
		Injected:      outcome.injected,
		SkippedReason: outcome.skippedReason,
		Result:        outcome.result,
	}
	if err := lane.AtomicWriteJSON(skillsPath, record); err != nil {
		_, _ = fmt.Fprintf(w, "lucind-ai: auto-skills: write skills record: %v\n", err)
	}
}
