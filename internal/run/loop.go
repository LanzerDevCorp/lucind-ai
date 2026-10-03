package run

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
)

// maxFeedbackOutputBytes bounds the verification output fed back to the worker.
const maxFeedbackOutputBytes = 8 * 1024

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
	output = tailBytes(output, maxFeedbackOutputBytes)
	// The output comes from commands the worker can influence. Fence it with more
	// backticks than any run inside it so it cannot close the block and inject text.
	fence := strings.Repeat("`", max(3, longestBacktickRun(output)+1))
	b.WriteString(fence + "\n")
	if output != "" {
		b.WriteString(strings.TrimRight(output, "\n"))
		b.WriteString("\n")
	}
	b.WriteString(fence + "\n\n")
	b.WriteString("Fix the failures without weakening or deleting tests, and stay inside the allowed edit surfaces.\n")
	return b.String()
}

// longestBacktickRun returns the length of the longest run of consecutive backticks in s.
func longestBacktickRun(s string) int {
	longest, current := 0, 0
	for _, r := range s {
		if r == '`' {
			current++
			if current > longest {
				longest = current
			}
		} else {
			current = 0
		}
	}
	return longest
}

// tailBytes returns at most n trailing bytes of s without splitting a UTF-8 sequence.
func tailBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := len(s) - n
	for cut < len(s) && !utf8.RuneStart(s[cut]) {
		cut++
	}
	return s[cut:]
}
