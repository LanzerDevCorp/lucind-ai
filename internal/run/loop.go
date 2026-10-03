package run

import (
	"fmt"
	"strings"

	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
)

// MaxTotalAttempts caps the total number of executor runs per lane across all rungs.
const MaxTotalAttempts = 4

// attemptPlan describes one planned execution attempt.
type attemptPlan struct {
	RungIndex    int
	ExecutorName string
	Model        string
}

// buildAttemptPlan constructs the ordered list of attempts for a packet.
// Rung 0 is the packet's own executor/model and gets max(1, MaxIterations) attempts;
// each escalation rung also gets max(1, MaxIterations) attempts.
// The total number of executor runs per lane is capped at MaxTotalAttempts.
func buildAttemptPlan(p packet.Packet) []attemptPlan {
	perRung := p.MaxIterations
	if perRung < 1 {
		perRung = 1
	}

	var plan []attemptPlan
	// Rung 0: packet's own executor and model
	for i := 0; i < perRung; i++ {
		plan = append(plan, attemptPlan{
			RungIndex:    0,
			ExecutorName: p.Executor,
			Model:        p.Model,
		})
	}

	// Escalation rungs
	for rungIdx, rung := range p.Escalation {
		for i := 0; i < perRung; i++ {
			plan = append(plan, attemptPlan{
				RungIndex:    rungIdx + 1,
				ExecutorName: rung.Executor,
				Model:        rung.Model,
			})
		}
	}

	if len(plan) > MaxTotalAttempts {
		plan = plan[:MaxTotalAttempts]
	}
	return plan
}

// formatFeedback constructs the prompt for the next attempt after verification failed.
func formatFeedback(body string, prevAttemptNum int, failedCommand string, exitCode int, output string) string {
	var b strings.Builder
	b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\n")
	}
	b.WriteString(fmt.Sprintf("\n## Previous attempt %d failed verification\n\n", prevAttemptNum))
	if failedCommand == "tree changed during verification" || (failedCommand == "" && exitCode == 0) {
		b.WriteString("tree changed during verification\n\n")
	} else {
		b.WriteString(fmt.Sprintf("%s, exit code: %d\n\n", failedCommand, exitCode))
	}
	b.WriteString("```\n")
	if output != "" {
		b.WriteString(strings.TrimRight(output, "\n"))
		b.WriteString("\n")
	}
	b.WriteString("```\n\n")
	b.WriteString("Fix the failures without weakening or deleting tests, and stay inside the allowed edit surfaces.\n")
	return b.String()
}
