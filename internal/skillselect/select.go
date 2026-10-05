package skillselect

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultThreshold is the default probability threshold for skill selection.
const DefaultThreshold = 0.7

const (
	noulExecutorInstruction = "A coding subagent will implement `task_brief`, editing only files matching `allowed_edit_surfaces`. It writes code, tests and docs. It cannot open pull requests, create issues, orchestrate or delegate to other agents, review other work, or talk to the user."
	noulQuestionInstruction = "Must this subagent read the skill below before it starts changing files?"
	noulCriteriaTrue        = "The skill's guidance directly shapes code, tests or documentation the subagent will write for this task."
	noulCriteriaFalse       = "The skill is unrelated to the task, only covers orchestration, delegation, pull requests, issues, reviews or user interaction, or merely shares the task's topic without guiding the subagent's edits."
)

// Input contains the task brief and allowed edit surfaces for skill selection.
type Input struct {
	Brief string   `json:"brief"`
	Allow []string `json:"allow"`
}

// Decision represents the selection decision for a single skill.
type Decision struct {
	Name        string  `json:"name"`
	Path        string  `json:"path"`
	Probability float64 `json:"probability"`
	Selected    bool    `json:"selected"`
}

// Usage holds token consumption statistics from the API.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Result holds the overall outcome of skill selection.
type Result struct {
	Model     string     `json:"model"`
	Threshold float64    `json:"threshold"`
	Decisions []Decision `json:"decisions"`
	Usage     Usage      `json:"usage"`
}

type statePayload struct {
	TaskBrief           string   `json:"task_brief"`
	AllowedEditSurfaces []string `json:"allowed_edit_surfaces"`
	FileExtensions      []string `json:"file_extensions"`
}

type noulSkillPayload struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type noulInstructionsPayload struct {
	Executor string           `json:"executor"`
	Question string           `json:"question"`
	Skill    noulSkillPayload `json:"skill"`
}

type noulQuestionPayload struct {
	Type         string                  `json:"type"`
	Instructions noulInstructionsPayload `json:"instructions"`
	Criteria     map[string]string       `json:"criteria"`
}

type requestPayload struct {
	State     statePayload                   `json:"state"`
	Model     string                         `json:"model"`
	Questions map[string]noulQuestionPayload `json:"questions"`
}

type rawAnswer struct {
	Type string   `json:"type"`
	Noul *float64 `json:"noul"`
}

// InvalidAnswerError represents an answer that is not a Noul probability in [0, 1].
type InvalidAnswerError struct {
	QuestionID string
	Reason     string
}

func (e *InvalidAnswerError) Error() string {
	return fmt.Sprintf("invalid answer for question %q: %s", e.QuestionID, e.Reason)
}

func noulProbability(qid string, ans rawAnswer) (float64, error) {
	if ans.Type != "noul" {
		return 0, &InvalidAnswerError{QuestionID: qid, Reason: fmt.Sprintf("answer type %q, want \"noul\"", ans.Type)}
	}
	if ans.Noul == nil {
		return 0, &InvalidAnswerError{QuestionID: qid, Reason: "missing noul field"}
	}
	p := *ans.Noul
	if math.IsNaN(p) || p < 0 || p > 1 {
		return 0, &InvalidAnswerError{QuestionID: qid, Reason: fmt.Sprintf("noul %v outside [0, 1]", p)}
	}
	return p, nil
}

type rawResponse struct {
	Model   string               `json:"model"`
	Answers map[string]rawAnswer `json:"answers"`
	Usage   Usage                `json:"usage"`
}

// Select runs skill selection via the TypeSafe SystemOne Jev API.
func Select(ctx context.Context, client *Client, skills []Skill, in Input, threshold float64) (Result, error) {
	if len(skills) == 0 {
		return Result{
			Threshold: threshold,
			Decisions: []Decision{},
		}, nil
	}

	allow := in.Allow
	if allow == nil {
		allow = []string{}
	}

	state := statePayload{
		TaskBrief:           in.Brief,
		AllowedEditSurfaces: allow,
		FileExtensions:      deriveExtensions(allow),
	}

	questions := make(map[string]noulQuestionPayload, len(skills))
	for i, s := range skills {
		qid := fmt.Sprintf("skill_%03d", i)
		questions[qid] = noulQuestionPayload{
			Type: "noul",
			Instructions: noulInstructionsPayload{
				Executor: noulExecutorInstruction,
				Question: noulQuestionInstruction,
				Skill: noulSkillPayload{
					Name:        s.Name,
					Description: s.Description,
				},
			},
			Criteria: map[string]string{
				"true":  noulCriteriaTrue,
				"false": noulCriteriaFalse,
			},
		}
	}

	model := client.model
	if model == "" {
		model = DefaultModel
	}

	reqBody := requestPayload{
		State:     state,
		Model:     model,
		Questions: questions,
	}

	respBytes, err := client.Post(ctx, reqBody)
	if err != nil {
		return Result{}, err
	}

	var resp rawResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return Result{}, &MalformedJSONError{Err: err}
	}

	decisions := make([]Decision, 0, len(skills))
	for i, s := range skills {
		qid := fmt.Sprintf("skill_%03d", i)
		ans, ok := resp.Answers[qid]
		if !ok {
			return Result{}, &MissingAnswerError{QuestionID: qid}
		}

		prob, err := noulProbability(qid, ans)
		if err != nil {
			return Result{}, err
		}

		selected := prob >= threshold
		decisions = append(decisions, Decision{
			Name:        s.Name,
			Path:        s.Path,
			Probability: prob,
			Selected:    selected,
		})
	}

	sort.Slice(decisions, func(i, j int) bool {
		if decisions[i].Probability != decisions[j].Probability {
			return decisions[i].Probability > decisions[j].Probability
		}
		return decisions[i].Name < decisions[j].Name
	})

	respModel := resp.Model
	if respModel == "" {
		respModel = model
	}

	return Result{
		Model:     respModel,
		Threshold: threshold,
		Decisions: decisions,
		Usage:     resp.Usage,
	}, nil
}

func deriveExtensions(allow []string) []string {
	extMap := make(map[string]struct{})
	for _, p := range allow {
		base := filepath.Base(filepath.ToSlash(p))
		ext := filepath.Ext(base)
		if ext != "" && ext != "." && !strings.ContainsAny(ext, "*?[]/\\") {
			extMap[ext] = struct{}{}
		}
	}
	exts := make([]string, 0, len(extMap))
	for ext := range extMap {
		exts = append(exts, ext)
	}
	sort.Strings(exts)
	return exts
}

// Section renders the selected skills as a Markdown section for inclusion in a task brief.
func Section(decisions []Decision) string {
	var selected []string
	for _, d := range decisions {
		if d.Selected {
			selected = append(selected, d.Path)
		}
	}
	if len(selected) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Skills to load before work\n")
	for _, p := range selected {
		b.WriteString(p)
		b.WriteByte('\n')
	}
	return b.String()
}
