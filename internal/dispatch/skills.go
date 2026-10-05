package dispatch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/skillselect"
)

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

func handleAutoSkills(ctx context.Context, repoRoot string, l lane.Lane, brief string, allow []string, stderr io.Writer) string {
	w := stderr
	if w == nil {
		w = io.Discard
	}

	skillsPath := lane.SkillsFilePath(repoRoot, l)

	if hasSkillsSection(brief) {
		record := SkillsRecord{
			Turn:          l.Turn,
			Injected:      false,
			SkippedReason: "brief_has_section",
		}
		if err := lane.AtomicWriteJSON(skillsPath, record); err != nil {
			_, _ = fmt.Fprintf(w, "lucind-ai: auto-skills: write skills record: %v\n", err)
		}
		return brief
	}

	in := skillselect.Input{
		Brief: brief,
		Allow: allow,
	}

	result, err := selectSkills(ctx, repoRoot, in)
	if err != nil {
		errMsg := redactAPIKey(err.Error())
		if errors.Is(err, skillselect.ErrMissingAPIKey) || errMsg == skillselect.ErrMissingAPIKey.Error() {
			_, _ = fmt.Fprintf(w, "lucind-ai: %s; dispatching without a skills section\n", errMsg)
		} else {
			_, _ = fmt.Fprintf(w, "lucind-ai: auto-skills: %s; dispatching without a skills section\n", errMsg)
		}

		record := SkillsRecord{
			Turn:     l.Turn,
			Injected: false,
			Error:    errMsg,
		}
		if writeErr := lane.AtomicWriteJSON(skillsPath, record); writeErr != nil {
			_, _ = fmt.Fprintf(w, "lucind-ai: auto-skills: write skills record: %v\n", writeErr)
		}
		return brief
	}

	sec := skillselect.Section(result.Decisions)
	if sec != "" {
		newBrief := insertSkillsSection(brief, sec)
		record := SkillsRecord{
			Turn:     l.Turn,
			Injected: true,
			Result:   &result,
		}
		if err := lane.AtomicWriteJSON(skillsPath, record); err != nil {
			_, _ = fmt.Fprintf(w, "lucind-ai: auto-skills: write skills record: %v\n", err)
		}
		return newBrief
	}

	record := SkillsRecord{
		Turn:          l.Turn,
		Injected:      false,
		SkippedReason: "no_skill_selected",
		Result:        &result,
	}
	if err := lane.AtomicWriteJSON(skillsPath, record); err != nil {
		_, _ = fmt.Fprintf(w, "lucind-ai: auto-skills: write skills record: %v\n", err)
	}
	return brief
}
