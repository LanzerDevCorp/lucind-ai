package main

import (
	"context"
	"os/exec"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
)

func TestJudgeGateFromEnvIsOptIn(t *testing.T) {
	t.Setenv("LUCIND_JUDGES", "")
	if judgeGateFromEnv() != nil {
		t.Fatal("judges must be off unless LUCIND_JUDGES=on")
	}
	t.Setenv("LUCIND_JUDGES", "on")
	if judgeGateFromEnv() == nil {
		t.Fatal("LUCIND_JUDGES=on must enable the gate")
	}
}

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return dir
}

// Path-only classification cannot prove content passive, so the floor is the medium plan:
// exactly one judge, even for a docs path.
func TestNewJudgeGateFollowsRiskTier(t *testing.T) {
	calls := 0
	run := func(context.Context, string, string, string) (string, error) { calls++; return "[]", nil }
	gate := newJudgeGate(run, defaultJudgeModels)

	st, reason := gate(context.Background(), gitRepo(t), packet.Packet{AllowedPaths: []string{"docs/readme.md"}})
	if st != lane.Done || calls != 1 {
		t.Fatalf("medium packet: status=%v reason=%q judge calls=%d, want done with 1 call", st, reason, calls)
	}

	calls = 0
	st, reason = gate(context.Background(), gitRepo(t), packet.Packet{AllowedPaths: []string{"internal/auth/token.go"}})
	if st != lane.Done || calls != 2 {
		t.Fatalf("high packet: status=%v reason=%q judge calls=%d, want done with 2 calls", st, reason, calls)
	}
}

func TestNewJudgeGateFailsClosedWithoutDistinctModels(t *testing.T) {
	run := func(context.Context, string, string, string) (string, error) { return "[]", nil }
	gate := newJudgeGate(run, []string{"gpt-5.6-sol-high", "gpt-5.3-codex"})
	st, reason := gate(context.Background(), t.TempDir(), packet.Packet{AllowedPaths: []string{"internal/auth/token.go"}})
	if st != lane.Blocked || reason == "" {
		t.Fatalf("high-risk packet with same-family judges: status=%v reason=%q, want blocked", st, reason)
	}
}
