// Package judges implements the blind-judge pre-commit gate.
package judges

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/LanzerDevCorp/lucind-ai/internal/attest"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/risk"
)

const maxReasonBytes = 4 * 1024

// Finding is one issue reported by a judge.
type Finding struct {
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Summary  string `json:"summary"`
}

// Runner executes one judge and returns its raw response text.
type Runner func(ctx context.Context, worktreePath, model, prompt string) (string, error)

// CursorRunner returns a Runner backed by cursor-agent.
func CursorRunner(binary string) Runner {
	if binary == "" {
		binary = "cursor-agent"
	}
	return func(ctx context.Context, worktreePath, model, prompt string) (string, error) {
		cmd := exec.CommandContext(ctx, binary,
			"--print",
			"--output-format", "json",
			"--mode", "plan",
			"--trust",
			"--model", model,
			prompt,
		)
		cmd.Dir = worktreePath

		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		stdout, err := cmd.Output()
		if err != nil {
			detail := strings.TrimSpace(stderr.String())
			if detail == "" {
				return "", fmt.Errorf("cursor-agent: %w", err)
			}
			return "", fmt.Errorf("cursor-agent: %w: %s", err, detail)
		}

		var envelope struct {
			Result *string `json:"result"`
		}
		if err := json.Unmarshal(stdout, &envelope); err != nil {
			return "", fmt.Errorf("parse cursor-agent JSON envelope: %w", err)
		}
		if envelope.Result == nil {
			return "", errors.New("parse cursor-agent JSON envelope: missing result")
		}
		return *envelope.Result, nil
	}
}

// ParseFindings parses a bare or Markdown-fenced JSON findings array.
func ParseFindings(raw string) ([]Finding, error) {
	candidates := []string{strings.TrimSpace(raw)}
	candidates = append(candidates, fencedJSON(raw)...)

	var lastErr error
	for _, candidate := range candidates {
		findings, err := decodeFindings(candidate)
		if err == nil {
			return findings, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("no JSON findings array")
	}
	return nil, fmt.Errorf("parse findings: %w", lastErr)
}

func fencedJSON(raw string) []string {
	var candidates []string
	remaining := raw
	for {
		start := strings.Index(remaining, "```")
		if start < 0 {
			return candidates
		}
		remaining = remaining[start+3:]
		newline := strings.IndexByte(remaining, '\n')
		if newline < 0 {
			return candidates
		}
		language := strings.TrimSpace(remaining[:newline])
		remaining = remaining[newline+1:]
		end := strings.Index(remaining, "```")
		if end < 0 {
			return candidates
		}
		if language == "" || strings.EqualFold(language, "json") {
			candidates = append(candidates, strings.TrimSpace(remaining[:end]))
		}
		remaining = remaining[end+3:]
	}
}

func decodeFindings(raw string) ([]Finding, error) {
	if raw == "" {
		return nil, errors.New("empty output")
	}

	decoder := json.NewDecoder(strings.NewReader(raw))
	var findings []Finding
	if err := decoder.Decode(&findings); err != nil {
		return nil, err
	}
	if findings == nil {
		return nil, errors.New("findings must be a JSON array")
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return nil, errors.New("unexpected content after findings array")
	} else if !errors.Is(err, io.EOF) {
		return nil, err
	}

	for i, finding := range findings {
		switch finding.Severity {
		case "high", "medium", "low":
		default:
			return nil, fmt.Errorf("finding %d has invalid severity %q", i+1, finding.Severity)
		}
	}
	return findings, nil
}

// Consensus returns findings confirmed by both judges.
func Consensus(a, b []Finding) []Finding {
	leftFindings := append([]Finding(nil), a...)
	sortFindings(leftFindings)
	confirmed := make([]Finding, 0)
	for _, left := range leftFindings {
		for _, right := range b {
			if left.Path == right.Path &&
				left.Severity == right.Severity &&
				abs(left.Line-right.Line) <= 3 {
				if !containsEquivalent(confirmed, left) {
					confirmed = append(confirmed, left)
				}
				break
			}
		}
	}
	sortFindings(confirmed)
	return confirmed
}

func containsEquivalent(findings []Finding, candidate Finding) bool {
	for _, finding := range findings {
		if finding.Path == candidate.Path &&
			finding.Severity == candidate.Severity &&
			abs(finding.Line-candidate.Line) <= 3 {
			return true
		}
	}
	return false
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func sortFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		if findings[i].Summary != findings[j].Summary {
			return findings[i].Summary < findings[j].Summary
		}
		return findings[i].Severity < findings[j].Severity
	})
}

// Gate runs the blind judges required by Plan.
type Gate struct {
	Plan   risk.Plan
	Models []string
	Run    Runner
	Goal   func(packet.Packet) string
}

// PreCommit implements run.Deps.PreCommitGate.
func (g Gate) PreCommit(ctx context.Context, worktreePath string, p packet.Packet) (lane.Status, string) {
	if g.Plan.Judges == 0 {
		return lane.Done, ""
	}
	if reason := g.validate(); reason != "" {
		return blocked(reason)
	}

	treeBefore, err := attest.TreeHash(ctx, worktreePath)
	if err != nil {
		return blocked(fmt.Sprintf("compute tree hash before judges: %v", err))
	}

	goal := p.Body
	if g.Goal != nil {
		goal = g.Goal(p)
	}
	prompt := judgePrompt(goal)

	allFindings := make([][]Finding, 0, g.Plan.Judges)
	var judgeErr error
	for i := 0; i < g.Plan.Judges; i++ {
		raw, err := g.Run(ctx, worktreePath, strings.TrimSpace(g.Models[i]), prompt)
		if err != nil {
			judgeErr = fmt.Errorf("run judge %d: %w", i+1, err)
			break
		}
		findings, err := ParseFindings(raw)
		if err != nil {
			judgeErr = fmt.Errorf("parse judge %d output: %w", i+1, err)
			break
		}
		allFindings = append(allFindings, findings)
	}

	treeAfter, err := attest.TreeHash(ctx, worktreePath)
	if err != nil {
		return blocked(fmt.Sprintf("compute tree hash after judges: %v", err))
	}
	if treeAfter != treeBefore {
		return lane.Blocked, "judge modified the worktree"
	}
	if judgeErr != nil {
		return blocked(judgeErr.Error())
	}

	verdict := allFindings[0]
	for i := 1; i < len(allFindings); i++ {
		verdict = Consensus(verdict, allFindings[i])
	}
	sortFindings(verdict)

	var counted []Finding
	for _, finding := range verdict {
		if finding.Severity == "high" || finding.Severity == "medium" {
			counted = append(counted, finding)
		}
	}
	if len(counted) == 0 {
		return lane.Done, ""
	}

	lines := make([]string, 0, len(counted))
	for _, finding := range counted {
		lines = append(lines, formatFinding(finding))
	}
	return lane.Failed, capReason(strings.Join(lines, "\n"))
}

func (g Gate) validate() string {
	if g.Plan.Judges < 0 {
		return "judge count must not be negative"
	}
	if len(g.Models) < g.Plan.Judges {
		return fmt.Sprintf("judge plan requires %d different models, got %d", g.Plan.Judges, len(g.Models))
	}
	if g.Run == nil {
		return "judge runner is not configured"
	}

	models := make(map[string]struct{}, g.Plan.Judges)
	families := make(map[string]struct{}, g.Plan.Judges)
	for i := 0; i < g.Plan.Judges; i++ {
		model := strings.TrimSpace(g.Models[i])
		if model == "" {
			return fmt.Sprintf("judge model %d is empty", i+1)
		}
		if _, duplicate := models[model]; duplicate {
			return "judge models must be pairwise different"
		}
		models[model] = struct{}{}

		family := strings.SplitN(model, "-", 2)[0]
		if g.Plan.Judges >= 2 {
			if _, duplicate := families[family]; duplicate {
				return "judge models must come from different model families"
			}
			families[family] = struct{}{}
		}
	}
	return ""
}

func judgePrompt(goal string) string {
	return fmt.Sprintf(`Goal:
%s

Review the uncommitted diff in this worktree: inspect git diff HEAD plus all untracked files.
This is a read-only review. Do not modify, create, delete, stage, or commit any file.
Return only a JSON array with exactly this contract:
[{"severity":"high|medium|low","path":"repository/relative/path","line":1,"summary":"concise finding"}]
Use [] when there are no findings.`, goal)
}

func formatFinding(finding Finding) string {
	return fmt.Sprintf("%s:%d [%s] %s", finding.Path, finding.Line, finding.Severity, finding.Summary)
}

func blocked(reason string) (lane.Status, string) {
	return lane.Blocked, capReason(reason)
}

func capReason(reason string) string {
	if len(reason) <= maxReasonBytes {
		return reason
	}
	reason = reason[:maxReasonBytes]
	for !utf8.ValidString(reason) {
		reason = reason[:len(reason)-1]
	}
	return reason
}
