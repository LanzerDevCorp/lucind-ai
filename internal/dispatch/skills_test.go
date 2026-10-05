package dispatch_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/dispatch"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/skillselect"
)

func TestAutoSkills_FlagOff(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	runner := setupFakeRunnerForNewLane(t, "w1:pLane1")

	selectorCalled := false
	dispatch.SetSkillSelectorForTesting(func(ctx context.Context, repoRoot string, in skillselect.Input) (skillselect.Result, error) {
		selectorCalled = true
		return skillselect.Result{}, nil
	})
	t.Cleanup(dispatch.ResetSkillSelectorForTesting)

	opts := dispatch.Options{
		Cwd:        repoDir,
		AutoSkills: false,
		Model:      "gemini-3.8-flash-high",
		Brief:      "# Implement Feature A\nTask description",
		Detach:     true,
	}

	out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}

	if selectorCalled {
		t.Errorf("selector was called when AutoSkills was false")
	}

	skillsPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "skills-1.json")
	if _, err := os.Stat(skillsPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("skills-1.json should not exist when AutoSkills is false, stat err = %v", err)
	}
}

func TestAutoSkills_BriefAlreadyHasSection(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	runner := setupFakeRunnerForNewLane(t, "w1:pLane1")

	selectorCalled := false
	dispatch.SetSkillSelectorForTesting(func(ctx context.Context, repoRoot string, in skillselect.Input) (skillselect.Result, error) {
		selectorCalled = true
		return skillselect.Result{}, nil
	})
	t.Cleanup(dispatch.ResetSkillSelectorForTesting)

	userBrief := "# Implement Feature B\n\n## Skills to load before work\n/path/to/skill/SKILL.md\n"
	opts := dispatch.Options{
		Cwd:        repoDir,
		AutoSkills: true,
		Model:      "gemini-3.8-flash-high",
		Brief:      userBrief,
		Detach:     true,
	}

	out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}

	if selectorCalled {
		t.Errorf("selector should not be called when brief already has skills section")
	}

	// Verify brief.md contains original brief unchanged
	briefPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "brief.md")
	contentBytes, err := os.ReadFile(briefPath)
	if err != nil {
		t.Fatalf("read brief.md: %v", err)
	}
	content := string(contentBytes)
	if !strings.HasPrefix(content, strings.TrimRight(userBrief, "\n")) {
		t.Errorf("brief.md does not start with original brief; got:\n%s", content)
	}

	// Verify skills-1.json records brief_has_section
	skillsPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "skills-1.json")
	data, err := os.ReadFile(skillsPath)
	if err != nil {
		t.Fatalf("read skills-1.json: %v", err)
	}

	var rec dispatch.SkillsRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("unmarshal skills-1.json: %v", err)
	}

	if rec.Turn != 1 {
		t.Errorf("rec.Turn = %d, want 1", rec.Turn)
	}
	if rec.Injected {
		t.Errorf("rec.Injected = true, want false")
	}
	if rec.SkippedReason != "brief_has_section" {
		t.Errorf("rec.SkippedReason = %q, want %q", rec.SkippedReason, "brief_has_section")
	}
	if rec.Error != "" {
		t.Errorf("rec.Error = %q, want empty", rec.Error)
	}
	if rec.Result != nil {
		t.Errorf("rec.Result = %+v, want nil", rec.Result)
	}
}

func TestAutoSkills_SectionPlacement(t *testing.T) {
	section := "## Skills to load before work\n/skills/golang-cli/SKILL.md"
	tests := []struct {
		name       string
		brief      string
		wantPrefix string
	}{
		{
			name:       "no title puts the section first",
			brief:      "## Goal\nDo X.\n",
			wantPrefix: section + "\n\n## Goal\nDo X.",
		},
		{
			name:       "leading blank lines before the title",
			brief:      "\n\n# Task\n\n## Goal\nDo X.\n",
			wantPrefix: "# Task\n\n" + section + "\n\n## Goal\nDo X.",
		},
		{
			name:       "title only",
			brief:      "# Task\n",
			wantPrefix: "# Task\n\n" + section + "\n\n---\n## Lane Contract",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HERDR_ENV", "1")
			repoDir := t.TempDir()
			initGitRepo(t, repoDir)
			runner := setupFakeRunnerForNewLane(t, "w1:pLane1")

			dispatch.SetSkillSelectorForTesting(func(ctx context.Context, repoRoot string, in skillselect.Input) (skillselect.Result, error) {
				return skillselect.Result{Decisions: []skillselect.Decision{
					{Name: "golang-cli", Path: "/skills/golang-cli/SKILL.md", Probability: 0.9, Selected: true},
				}}, nil
			})
			t.Cleanup(dispatch.ResetSkillSelectorForTesting)

			opts := dispatch.Options{
				Cwd:        repoDir,
				AutoSkills: true,
				Model:      "gemini-3.8-flash-high",
				Allow:      []string{"internal/**"},
				Brief:      tt.brief,
				Detach:     true,
			}
			out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
			if err != nil || exitCode != 0 {
				t.Fatalf("Dispatch() = exit %d, err %v; want 0, nil", exitCode, err)
			}

			content, err := os.ReadFile(filepath.Join(lane.LaneDir(repoDir, out.Lane), "brief.md"))
			if err != nil {
				t.Fatalf("read brief.md: %v", err)
			}
			if !strings.HasPrefix(string(content), tt.wantPrefix) {
				t.Errorf("brief.md prefix mismatch:\ngot:\n%s\nwant prefix:\n%s", content, tt.wantPrefix)
			}
		})
	}
}

func TestAutoSkills_SelectorSuccess_WithSelectedSkills(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	runner := setupFakeRunnerForNewLane(t, "w1:pLane1")

	expectedResult := skillselect.Result{
		Model:     "jev-latest",
		Threshold: 0.7,
		Decisions: []skillselect.Decision{
			{
				Name:        "golang-cli",
				Path:        "/skills/golang-cli/SKILL.md",
				Probability: 0.95,
				Selected:    true,
			},
			{
				Name:        "other-skill",
				Path:        "/skills/other/SKILL.md",
				Probability: 0.20,
				Selected:    false,
			},
		},
	}

	var capturedInput skillselect.Input
	dispatch.SetSkillSelectorForTesting(func(ctx context.Context, repoRoot string, in skillselect.Input) (skillselect.Result, error) {
		capturedInput = in
		return expectedResult, nil
	})
	t.Cleanup(dispatch.ResetSkillSelectorForTesting)

	userBrief := "# Implement Feature C\nPlease do it well."
	opts := dispatch.Options{
		Cwd:        repoDir,
		AutoSkills: true,
		Model:      "gemini-3.8-flash-high",
		Allow:      []string{"internal/**", "cmd/**"},
		Brief:      userBrief,
		Detach:     true,
	}

	out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}

	if capturedInput.Brief != userBrief {
		t.Errorf("capturedInput.Brief = %q, want %q", capturedInput.Brief, userBrief)
	}

	// Verify brief.md contains section before ## Lane Contract footer
	briefPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "brief.md")
	contentBytes, err := os.ReadFile(briefPath)
	if err != nil {
		t.Fatalf("read brief.md: %v", err)
	}
	content := string(contentBytes)

	sectionIdx := strings.Index(content, "## Skills to load before work\n/skills/golang-cli/SKILL.md")
	footerIdx := strings.Index(content, "## Lane Contract")
	if sectionIdx == -1 {
		t.Errorf("brief.md does not contain skills section:\n%s", content)
	}
	if footerIdx == -1 {
		t.Errorf("brief.md does not contain ## Lane Contract footer:\n%s", content)
	}
	if sectionIdx > footerIdx {
		t.Errorf("skills section appears after Lane Contract footer: section at %d, footer at %d", sectionIdx, footerIdx)
	}
	wantPrefix := "# Implement Feature C\n\n## Skills to load before work\n/skills/golang-cli/SKILL.md\n\nPlease do it well."
	if !strings.HasPrefix(content, wantPrefix) {
		t.Errorf("skills section is not right after the title:\ngot:\n%s\nwant prefix:\n%s", content, wantPrefix)
	}

	// Verify skills-1.json records injected: true and the decisions
	skillsPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "skills-1.json")
	data, err := os.ReadFile(skillsPath)
	if err != nil {
		t.Fatalf("read skills-1.json: %v", err)
	}

	var rec dispatch.SkillsRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("unmarshal skills-1.json: %v", err)
	}

	if rec.Turn != 1 {
		t.Errorf("rec.Turn = %d, want 1", rec.Turn)
	}
	if !rec.Injected {
		t.Errorf("rec.Injected = false, want true")
	}
	if rec.SkippedReason != "" {
		t.Errorf("rec.SkippedReason = %q, want empty", rec.SkippedReason)
	}
	if rec.Error != "" {
		t.Errorf("rec.Error = %q, want empty", rec.Error)
	}
	if rec.Result == nil {
		t.Fatal("rec.Result is nil, want non-nil")
	}
	if len(rec.Result.Decisions) != 2 {
		t.Fatalf("got %d decisions, want 2", len(rec.Result.Decisions))
	}
	if rec.Result.Decisions[0].Name != "golang-cli" || !rec.Result.Decisions[0].Selected {
		t.Errorf("decision[0] = %+v, want golang-cli selected", rec.Result.Decisions[0])
	}
}

func TestAutoSkills_SelectorSuccess_NoSelectedSkill(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	runner := setupFakeRunnerForNewLane(t, "w1:pLane1")

	expectedResult := skillselect.Result{
		Model:     "jev-latest",
		Threshold: 0.7,
		Decisions: []skillselect.Decision{
			{
				Name:        "unrelated-skill",
				Path:        "/skills/unrelated/SKILL.md",
				Probability: 0.10,
				Selected:    false,
			},
		},
	}

	dispatch.SetSkillSelectorForTesting(func(ctx context.Context, repoRoot string, in skillselect.Input) (skillselect.Result, error) {
		return expectedResult, nil
	})
	t.Cleanup(dispatch.ResetSkillSelectorForTesting)

	userBrief := "# Implement Feature D\nDo something simple."
	opts := dispatch.Options{
		Cwd:        repoDir,
		AutoSkills: true,
		Model:      "gemini-3.8-flash-high",
		Brief:      userBrief,
		Detach:     true,
	}

	out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}

	// Verify brief.md has userBrief unchanged before ## Lane Contract
	briefPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "brief.md")
	contentBytes, err := os.ReadFile(briefPath)
	if err != nil {
		t.Fatalf("read brief.md: %v", err)
	}
	content := string(contentBytes)
	if strings.Contains(content, "## Skills to load before work") {
		t.Errorf("brief.md should not contain skills section when none selected; got:\n%s", content)
	}
	expectedPrefix := strings.TrimRight(userBrief, "\n") + "\n\n---\n## Lane Contract"
	if !strings.HasPrefix(content, expectedPrefix) {
		t.Errorf("brief.md does not match expected prefix; got:\n%s", content)
	}

	// Verify skills-1.json records no_skill_selected and result
	skillsPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "skills-1.json")
	data, err := os.ReadFile(skillsPath)
	if err != nil {
		t.Fatalf("read skills-1.json: %v", err)
	}

	var rec dispatch.SkillsRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("unmarshal skills-1.json: %v", err)
	}

	if rec.Turn != 1 {
		t.Errorf("rec.Turn = %d, want 1", rec.Turn)
	}
	if rec.Injected {
		t.Errorf("rec.Injected = true, want false")
	}
	if rec.SkippedReason != "no_skill_selected" {
		t.Errorf("rec.SkippedReason = %q, want %q", rec.SkippedReason, "no_skill_selected")
	}
	if rec.Error != "" {
		t.Errorf("rec.Error = %q, want empty", rec.Error)
	}
	if rec.Result == nil {
		t.Fatal("rec.Result is nil, want non-nil")
	}
	if len(rec.Result.Decisions) != 1 {
		t.Fatalf("got %d decisions, want 1", len(rec.Result.Decisions))
	}
}

func TestAutoSkills_SelectorError(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("TYPESAFE_API_KEY", "secret-key-12345")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	runner := setupFakeRunnerForNewLane(t, "w1:pLane1")

	dispatch.SetSkillSelectorForTesting(func(ctx context.Context, repoRoot string, in skillselect.Input) (skillselect.Result, error) {
		return skillselect.Result{}, errors.New("auth failed with secret-key-12345: bad token")
	})
	t.Cleanup(dispatch.ResetSkillSelectorForTesting)

	var stderrBuf bytes.Buffer
	userBrief := "# Implement Feature E\nFix issue."
	opts := dispatch.Options{
		Cwd:        repoDir,
		AutoSkills: true,
		Stderr:     &stderrBuf,
		Model:      "gemini-3.8-flash-high",
		Brief:      userBrief,
		Detach:     true,
	}

	out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("dispatch should succeed despite selector error, got err: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}

	// Verify warning on stderr: lucind-ai: auto-skills: <error>; dispatching without a skills section\n
	stderrStr := stderrBuf.String()
	if !strings.HasPrefix(stderrStr, "lucind-ai: auto-skills: ") {
		t.Errorf("stderr does not have expected prefix; got: %q", stderrStr)
	}
	if !strings.Contains(stderrStr, "; dispatching without a skills section\n") {
		t.Errorf("stderr does not have expected suffix; got: %q", stderrStr)
	}
	// Verify API key is redacted in warning
	if strings.Contains(stderrStr, "secret-key-12345") {
		t.Errorf("stderr contains unredacted API key: %q", stderrStr)
	}
	if !strings.Contains(stderrStr, "[REDACTED]") {
		t.Errorf("stderr does not contain [REDACTED]: %q", stderrStr)
	}

	// Verify brief.md has userBrief unchanged before ## Lane Contract
	briefPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "brief.md")
	contentBytes, err := os.ReadFile(briefPath)
	if err != nil {
		t.Fatalf("read brief.md: %v", err)
	}
	content := string(contentBytes)
	if strings.Contains(content, "## Skills to load before work") {
		t.Errorf("brief.md should not contain skills section on error; got:\n%s", content)
	}

	// Verify skills-1.json records the error and redacts API key
	skillsPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "skills-1.json")
	data, err := os.ReadFile(skillsPath)
	if err != nil {
		t.Fatalf("read skills-1.json: %v", err)
	}

	var rec dispatch.SkillsRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("unmarshal skills-1.json: %v", err)
	}

	if rec.Turn != 1 {
		t.Errorf("rec.Turn = %d, want 1", rec.Turn)
	}
	if rec.Injected {
		t.Errorf("rec.Injected = true, want false")
	}
	if strings.Contains(rec.Error, "secret-key-12345") {
		t.Errorf("rec.Error contains unredacted API key: %q", rec.Error)
	}
	if !strings.Contains(rec.Error, "[REDACTED]") {
		t.Errorf("rec.Error does not contain [REDACTED]: %q", rec.Error)
	}
	if rec.Result != nil {
		t.Errorf("rec.Result = %+v, want nil", rec.Result)
	}
}

func TestAutoSkills_ContinuationTurn(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	runner := setupFakeRunnerForNewLane(t, "w1:pLane1")

	resultTurn1 := skillselect.Result{
		Model:     "jev-latest",
		Threshold: 0.7,
		Decisions: []skillselect.Decision{
			{Name: "skill-1", Path: "/skills/skill-1/SKILL.md", Probability: 0.9, Selected: true},
		},
	}
	resultTurn2 := skillselect.Result{
		Model:     "jev-latest",
		Threshold: 0.7,
		Decisions: []skillselect.Decision{
			{Name: "skill-2", Path: "/skills/skill-2/SKILL.md", Probability: 0.85, Selected: true},
		},
	}

	currentTurnCall := 0
	dispatch.SetSkillSelectorForTesting(func(ctx context.Context, repoRoot string, in skillselect.Input) (skillselect.Result, error) {
		currentTurnCall++
		if currentTurnCall == 1 {
			return resultTurn1, nil
		}
		return resultTurn2, nil
	})
	t.Cleanup(dispatch.ResetSkillSelectorForTesting)

	// Turn 1
	opts1 := dispatch.Options{
		Cwd:        repoDir,
		AutoSkills: true,
		Model:      "gemini-3.8-flash-high",
		Brief:      "# Turn 1 brief",
		Detach:     true,
	}

	out1, exitCode, err := dispatch.Dispatch(context.Background(), opts1, runner)
	if err != nil {
		t.Fatalf("dispatch turn 1 failed: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("turn 1 exitCode = %d, want 0", exitCode)
	}

	skills1Path := filepath.Join(lane.LaneDir(repoDir, out1.Lane), "skills-1.json")
	data1, err := os.ReadFile(skills1Path)
	if err != nil {
		t.Fatalf("read skills-1.json: %v", err)
	}
	var rec1 dispatch.SkillsRecord
	if err := json.Unmarshal(data1, &rec1); err != nil {
		t.Fatalf("unmarshal skills-1.json: %v", err)
	}
	if rec1.Turn != 1 {
		t.Errorf("turn 1 record turn = %d, want 1", rec1.Turn)
	}
	if !rec1.Injected {
		t.Errorf("turn 1 injected = false, want true")
	}

	// Turn 2: continuation
	runner2 := newFakeHerdrRunner()
	runner2.handlers["agent prompt"] = func(args []string) ([]byte, error) {
		return []byte(`{"result": {"submitted": true}}`), nil
	}

	opts2 := dispatch.Options{
		Cwd:        repoDir,
		LaneID:     out1.Lane,
		AutoSkills: true,
		Brief:      "# Turn 2 brief",
		Detach:     true,
	}

	out2, exitCode, err := dispatch.Dispatch(context.Background(), opts2, runner2)
	if err != nil {
		t.Fatalf("dispatch turn 2 failed: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("turn 2 exitCode = %d, want 0", exitCode)
	}

	skills2Path := filepath.Join(lane.LaneDir(repoDir, out2.Lane), "skills-2.json")
	data2, err := os.ReadFile(skills2Path)
	if err != nil {
		t.Fatalf("read skills-2.json: %v", err)
	}
	var rec2 dispatch.SkillsRecord
	if err := json.Unmarshal(data2, &rec2); err != nil {
		t.Fatalf("unmarshal skills-2.json: %v", err)
	}
	if rec2.Turn != 2 {
		t.Errorf("turn 2 record turn = %d, want 2", rec2.Turn)
	}
	if !rec2.Injected {
		t.Errorf("turn 2 injected = false, want true")
	}
	if len(rec2.Result.Decisions) != 1 || rec2.Result.Decisions[0].Name != "skill-2" {
		t.Errorf("turn 2 decisions unexpected: %+v", rec2.Result)
	}
}

func TestDefaultSkillSelector_MissingRegistry(t *testing.T) {
	tempDir := t.TempDir()
	initGitRepo(t, tempDir)
	// No .atl/skill-registry.md in tempDir
	dispatch.ResetSkillSelectorForTesting()

	opts := dispatch.Options{
		Cwd:        tempDir,
		AutoSkills: true,
		Model:      "gemini-3.8-flash-high",
		Brief:      "test brief",
		Detach:     true,
	}

	runner := setupFakeRunnerForNewLane(t, "w1:pLane1")
	out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("dispatch should succeed even if default selector fails, got: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}

	skillsPath := filepath.Join(lane.LaneDir(tempDir, out.Lane), "skills-1.json")
	data, err := os.ReadFile(skillsPath)
	if err != nil {
		t.Fatalf("read skills-1.json: %v", err)
	}
	var rec dispatch.SkillsRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("unmarshal skills-1.json: %v", err)
	}
	if rec.Injected {
		t.Errorf("rec.Injected = true, want false")
	}
	if !strings.Contains(rec.Error, "no such file or directory") {
		t.Errorf("rec.Error expected missing file error, got: %q", rec.Error)
	}
}

func TestDefaultSkillSelector_MissingAPIKey(t *testing.T) {
	tempDir := t.TempDir()
	initGitRepo(t, tempDir)

	// Create valid .atl/skill-registry.md
	atlDir := filepath.Join(tempDir, ".atl")
	if err := os.MkdirAll(atlDir, 0755); err != nil {
		t.Fatalf("mkdir .atl: %v", err)
	}
	regContent := `# Registry

## Skills

| Skill | Trigger / description | Scope | Path |
| --- | --- | --- | --- |
| my-skill | Do something | internal | skills/my-skill/SKILL.md |
`
	if err := os.WriteFile(filepath.Join(atlDir, "skill-registry.md"), []byte(regContent), 0644); err != nil {
		t.Fatalf("write skill-registry.md: %v", err)
	}

	t.Setenv("TYPESAFE_API_KEY", "")
	dispatch.ResetSkillSelectorForTesting()

	opts := dispatch.Options{
		Cwd:        tempDir,
		AutoSkills: true,
		Model:      "gemini-3.8-flash-high",
		Brief:      "test brief",
		Detach:     true,
	}

	runner := setupFakeRunnerForNewLane(t, "w1:pLane1")
	out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("dispatch should succeed even if missing api key, got: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}

	skillsPath := filepath.Join(lane.LaneDir(tempDir, out.Lane), "skills-1.json")
	data, err := os.ReadFile(skillsPath)
	if err != nil {
		t.Fatalf("read skills-1.json: %v", err)
	}
	var rec dispatch.SkillsRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("unmarshal skills-1.json: %v", err)
	}
	if rec.Injected {
		t.Errorf("rec.Injected = true, want false")
	}
	if !strings.Contains(rec.Error, "typesafe api key is required") {
		t.Errorf("rec.Error expected typesafe api key is required, got: %q", rec.Error)
	}
}

func TestAutoSkills_SelectorError_DefaultStderr(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	runner := setupFakeRunnerForNewLane(t, "w1:pLane1")

	dispatch.SetSkillSelectorForTesting(func(ctx context.Context, repoRoot string, in skillselect.Input) (skillselect.Result, error) {
		return skillselect.Result{}, errors.New("network failure")
	})
	t.Cleanup(dispatch.ResetSkillSelectorForTesting)

	// Stderr left nil (defaulting to io.Discard)
	opts := dispatch.Options{
		Cwd:        repoDir,
		AutoSkills: true,
		Stderr:     nil,
		Model:      "gemini-3.8-flash-high",
		Brief:      "test brief",
		Detach:     true,
	}

	out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("dispatch should succeed with nil Stderr, got: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}

	skillsPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "skills-1.json")
	data, err := os.ReadFile(skillsPath)
	if err != nil {
		t.Fatalf("read skills-1.json: %v", err)
	}
	var rec dispatch.SkillsRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("unmarshal skills-1.json: %v", err)
	}
	if rec.Error != "network failure" {
		t.Errorf("rec.Error = %q, want \"network failure\"", rec.Error)
	}
}

func TestAutoSkills_EmptyBrief_WithSelectedSkills(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	runner := setupFakeRunnerForNewLane(t, "w1:pLane1")

	expectedResult := skillselect.Result{
		Model:     "jev-latest",
		Threshold: 0.7,
		Decisions: []skillselect.Decision{
			{
				Name:        "golang-cli",
				Path:        "/skills/golang-cli/SKILL.md",
				Probability: 0.95,
				Selected:    true,
			},
		},
	}

	dispatch.SetSkillSelectorForTesting(func(ctx context.Context, repoRoot string, in skillselect.Input) (skillselect.Result, error) {
		return expectedResult, nil
	})
	t.Cleanup(dispatch.ResetSkillSelectorForTesting)

	opts := dispatch.Options{
		Cwd:        repoDir,
		AutoSkills: true,
		Model:      "gemini-3.8-flash-high",
		Brief:      "",
		Detach:     true,
	}

	out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}

	briefPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "brief.md")
	contentBytes, err := os.ReadFile(briefPath)
	if err != nil {
		t.Fatalf("read brief.md: %v", err)
	}
	content := string(contentBytes)

	// Should start directly with skills section
	expectedPrefix := "## Skills to load before work\n/skills/golang-cli/SKILL.md\n\n---\n## Lane Contract"
	if !strings.HasPrefix(content, expectedPrefix) {
		t.Errorf("brief.md content with empty brief does not match expected prefix; got:\n%s", content)
	}
}

func TestAutoSkills_WriteFailure_FailOpen(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	createdLane, err := lane.Create(context.Background(), repoDir, []string{"internal/**"}, "gemini-3.8-flash-high")
	if err != nil {
		t.Fatalf("lane.Create failed: %v", err)
	}
	createdLane.PaneID = "w1:pCont1"
	createdLane.Status = lane.StatusFailed
	if err := createdLane.Save(repoDir); err != nil {
		t.Fatalf("lane.Save failed: %v", err)
	}

	// Make skills-2.json a directory so writing the skills record fails
	skills2Dir := filepath.Join(lane.LaneDir(repoDir, createdLane.ID), "skills-2.json")
	if err := os.Mkdir(skills2Dir, 0755); err != nil {
		t.Fatalf("mkdir skills-2.json: %v", err)
	}

	runner := newFakeHerdrRunner()
	runner.handlers["agent prompt"] = func(args []string) ([]byte, error) {
		return []byte(`{"result": {"submitted": true}}`), nil
	}

	expectedResult := skillselect.Result{
		Model:     "jev-latest",
		Threshold: 0.7,
		Decisions: []skillselect.Decision{
			{Name: "golang-cli", Path: "/skills/golang-cli/SKILL.md", Probability: 0.95, Selected: true},
		},
	}
	dispatch.SetSkillSelectorForTesting(func(ctx context.Context, repoRoot string, in skillselect.Input) (skillselect.Result, error) {
		return expectedResult, nil
	})
	t.Cleanup(dispatch.ResetSkillSelectorForTesting)

	var stderrBuf bytes.Buffer
	userBrief := "# Continuation task\nWork on feature."
	opts := dispatch.Options{
		Cwd:        repoDir,
		LaneID:     createdLane.ID,
		AutoSkills: true,
		Brief:      userBrief,
		Stderr:     &stderrBuf,
		Detach:     true,
	}

	out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("dispatch should succeed despite skills write error, got: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}
	if out.Lane != createdLane.ID {
		t.Errorf("out.Lane = %q, want %q", out.Lane, createdLane.ID)
	}

	stderrStr := stderrBuf.String()
	if !strings.Contains(stderrStr, "lucind-ai: auto-skills: write skills record: ") {
		t.Errorf("stderr does not contain expected warning; got: %q", stderrStr)
	}

	briefPath := filepath.Join(lane.LaneDir(repoDir, createdLane.ID), "brief.md")
	contentBytes, err := os.ReadFile(briefPath)
	if err != nil {
		t.Fatalf("read brief.md failed: %v", err)
	}
	content := string(contentBytes)
	if !strings.Contains(content, "## Skills to load before work\n/skills/golang-cli/SKILL.md") {
		t.Errorf("brief.md missing computed skills section; got:\n%s", content)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestDefaultSkillSelector_ResolvesSymlinks(t *testing.T) {
	t.Setenv("HERDR_ENV", "1")
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	skillsDir := t.TempDir()
	realSkillFile := filepath.Join(skillsDir, "SKILL.md")
	if err := os.WriteFile(realSkillFile, []byte("# My Skill\nDescription"), 0644); err != nil {
		t.Fatalf("write real skill file: %v", err)
	}
	canonicalPath, err := filepath.EvalSymlinks(realSkillFile)
	if err != nil {
		t.Fatalf("eval symlinks: %v", err)
	}

	symlinkDir := t.TempDir()
	symlinkSkillFile := filepath.Join(symlinkDir, "symlink-SKILL.md")
	if err := os.Symlink(realSkillFile, symlinkSkillFile); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	atlDir := filepath.Join(repoDir, ".atl")
	if err := os.MkdirAll(atlDir, 0755); err != nil {
		t.Fatalf("mkdir .atl: %v", err)
	}
	regContent := fmt.Sprintf("# Registry\n\n## Skills\n\n| Skill | Trigger / description | Scope | Path |\n| --- | --- | --- | --- |\n| `my-skill` | desc | repo | `%s` |\n", symlinkSkillFile)
	if err := os.WriteFile(filepath.Join(atlDir, "skill-registry.md"), []byte(regContent), 0644); err != nil {
		t.Fatalf("write skill-registry.md: %v", err)
	}

	t.Setenv("TYPESAFE_API_KEY", "mock-typesafe-key")

	origTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = origTransport })
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		respBody := `{"model":"jev-latest","answers":{"skill_000":{"type":"noul","noul":0.95}},"usage":{"input_tokens":10,"output_tokens":5}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(respBody)),
		}, nil
	})

	dispatch.ResetSkillSelectorForTesting()
	t.Cleanup(dispatch.ResetSkillSelectorForTesting)

	runner := setupFakeRunnerForNewLane(t, "w1:pLane1")

	opts := dispatch.Options{
		Cwd:        repoDir,
		AutoSkills: true,
		Model:      "gemini-3.8-flash-high",
		Brief:      "test brief",
		Detach:     true,
	}

	out, exitCode, err := dispatch.Dispatch(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}

	// Verify brief.md contains canonicalPath and not symlinkSkillFile
	briefPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "brief.md")
	contentBytes, err := os.ReadFile(briefPath)
	if err != nil {
		t.Fatalf("read brief.md: %v", err)
	}
	content := string(contentBytes)
	if !strings.Contains(content, canonicalPath) {
		t.Errorf("brief.md should contain canonical skill path %q; got:\n%s", canonicalPath, content)
	}
	if strings.Contains(content, symlinkSkillFile) {
		t.Errorf("brief.md should not contain unresolved symlink path %q; got:\n%s", symlinkSkillFile, content)
	}

	// Verify skills-1.json has canonicalPath in decisions
	skillsPath := filepath.Join(lane.LaneDir(repoDir, out.Lane), "skills-1.json")
	data, err := os.ReadFile(skillsPath)
	if err != nil {
		t.Fatalf("read skills-1.json: %v", err)
	}
	var rec dispatch.SkillsRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("unmarshal skills-1.json: %v", err)
	}
	if rec.Result == nil || len(rec.Result.Decisions) != 1 {
		t.Fatalf("unexpected result in skills record: %+v", rec.Result)
	}
	if rec.Result.Decisions[0].Path != canonicalPath {
		t.Errorf("decision path = %q, want canonical path %q", rec.Result.Decisions[0].Path, canonicalPath)
	}
}
