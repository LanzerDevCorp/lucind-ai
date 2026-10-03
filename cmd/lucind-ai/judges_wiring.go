package main

import (
	"context"
	"os"
	"strings"

	"github.com/LanzerDevCorp/lucind-ai/internal/judges"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/risk"
)

// defaultJudgeModels are two blind judges from different model families (Cursor model ids).
var defaultJudgeModels = []string{"claude-sonnet-5-thinking-high", "gpt-5.6-sol-high"}

// judgeGateFromEnv returns the dispatcher-commit PreCommitGate that runs blind judges, or nil
// when LUCIND_JUDGES is not "on". Judges cost Cursor quota and need cursor-agent, so they are
// opt-in. LUCIND_JUDGE_MODELS overrides the default models (comma separated).
func judgeGateFromEnv() func(context.Context, string, packet.Packet) (lane.Status, string) {
	if os.Getenv("LUCIND_JUDGES") != "on" {
		return nil
	}
	models := defaultJudgeModels
	if raw := strings.TrimSpace(os.Getenv("LUCIND_JUDGE_MODELS")); raw != "" {
		models = nil
		for _, m := range strings.Split(raw, ",") {
			if m = strings.TrimSpace(m); m != "" {
				models = append(models, m)
			}
		}
	}
	return newJudgeGate(judges.CursorRunner(""), models)
}

// newJudgeGate builds the gate. The verification plan follows the risk tier of the packet's
// declared write paths; an unclassifiable tier fails closed to the high plan (see risk.PlanFor).
func newJudgeGate(run judges.Runner, models []string) func(context.Context, string, packet.Packet) (lane.Status, string) {
	return func(ctx context.Context, worktreePath string, p packet.Packet) (lane.Status, string) {
		plan := risk.PlanFor(risk.ClassifyPaths(p.AllowedPaths).Tier)
		return judges.Gate{Plan: plan, Models: models, Run: run}.PreCommit(ctx, worktreePath, p)
	}
}
