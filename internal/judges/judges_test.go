package judges

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/risk"
)

func TestParseFindings(t *testing.T) {
	want := []Finding{{Severity: "high", Path: "a.go", Line: 12, Summary: "broken"}}
	tests := []struct {
		name    string
		raw     string
		want    []Finding
		wantErr bool
	}{
		{
			name: "bare JSON",
			raw:  `[{"severity":"high","path":"a.go","line":12,"summary":"broken"}]`,
			want: want,
		},
		{
			name: "fenced JSON",
			raw:  "Review result:\n```json\n[{\"severity\":\"high\",\"path\":\"a.go\",\"line\":12,\"summary\":\"broken\"}]\n```\n",
			want: want,
		},
		{
			name: "empty array",
			raw:  `[]`,
			want: []Finding{},
		},
		{
			name:    "garbage",
			raw:     `nothing useful`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFindings(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseFindings() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseFindings() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestConsensus(t *testing.T) {
	tests := []struct {
		name string
		a    []Finding
		b    []Finding
		want []Finding
	}{
		{
			name: "line tolerance matches and output is sorted",
			a: []Finding{
				{Severity: "medium", Path: "z.go", Line: 20, Summary: "z issue"},
				{Severity: "high", Path: "a.go", Line: 10, Summary: "a issue"},
			},
			b: []Finding{
				{Severity: "high", Path: "a.go", Line: 13, Summary: "same a issue"},
				{Severity: "medium", Path: "z.go", Line: 18, Summary: "same z issue"},
			},
			want: []Finding{
				{Severity: "high", Path: "a.go", Line: 10, Summary: "a issue"},
				{Severity: "medium", Path: "z.go", Line: 20, Summary: "z issue"},
			},
		},
		{
			name: "outside line tolerance",
			a:    []Finding{{Severity: "high", Path: "a.go", Line: 10, Summary: "issue"}},
			b:    []Finding{{Severity: "high", Path: "a.go", Line: 14, Summary: "issue"}},
			want: []Finding{},
		},
		{
			name: "different severity does not match",
			a:    []Finding{{Severity: "high", Path: "a.go", Line: 10, Summary: "issue"}},
			b:    []Finding{{Severity: "medium", Path: "a.go", Line: 10, Summary: "issue"}},
			want: []Finding{},
		},
		{
			name: "equivalent duplicates returned once",
			a: []Finding{
				{Severity: "high", Path: "a.go", Line: 12, Summary: "later report"},
				{Severity: "high", Path: "a.go", Line: 10, Summary: "first report"},
			},
			b:    []Finding{{Severity: "high", Path: "a.go", Line: 11, Summary: "confirmed"}},
			want: []Finding{{Severity: "high", Path: "a.go", Line: 10, Summary: "first report"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Consensus(tt.a, tt.b); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Consensus() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestGatePreCommit(t *testing.T) {
	finding := Finding{Severity: "high", Path: "internal/x.go", Line: 42, Summary: "nil dereference"}
	findingJSON := `[{"severity":"high","path":"internal/x.go","line":42,"summary":"nil dereference"}]`

	tests := []struct {
		name       string
		plan       risk.Plan
		models     []string
		outputs    []string
		runErr     error
		wantStatus lane.Status
		wantReason string
		wantCalls  int
	}{
		{
			name:       "passive skips runner",
			plan:       risk.Plan{Judges: 0},
			wantStatus: lane.Done,
		},
		{
			name:       "medium clean",
			plan:       risk.Plan{Judges: 1},
			models:     []string{"claude-sonnet"},
			outputs:    []string{"[]"},
			wantStatus: lane.Done,
			wantCalls:  1,
		},
		{
			name:       "medium high finding",
			plan:       risk.Plan{Judges: 1},
			models:     []string{"claude-sonnet"},
			outputs:    []string{findingJSON},
			wantStatus: lane.Failed,
			wantReason: "internal/x.go:42 [high] nil dereference",
			wantCalls:  1,
		},
		{
			name:       "high unconfirmed finding",
			plan:       risk.Plan{Judges: 2},
			models:     []string{"claude-sonnet", "gpt-5"},
			outputs:    []string{findingJSON, "[]"},
			wantStatus: lane.Done,
			wantCalls:  2,
		},
		{
			name:       "high confirmed finding",
			plan:       risk.Plan{Judges: 2},
			models:     []string{"claude-sonnet", "gpt-5"},
			outputs:    []string{findingJSON, `[{"severity":"high","path":"internal/x.go","line":44,"summary":"same defect"}]`},
			wantStatus: lane.Failed,
			wantReason: formatFinding(finding),
			wantCalls:  2,
		},
		{
			name:       "unparseable output",
			plan:       risk.Plan{Judges: 1},
			models:     []string{"claude-sonnet"},
			outputs:    []string{"not JSON"},
			wantStatus: lane.Blocked,
			wantReason: "parse judge 1 output",
			wantCalls:  1,
		},
		{
			name:       "same model twice",
			plan:       risk.Plan{Judges: 2},
			models:     []string{"claude-sonnet", "claude-sonnet"},
			wantStatus: lane.Blocked,
			wantReason: "different",
		},
		{
			name:       "same model family twice",
			plan:       risk.Plan{Judges: 2},
			models:     []string{"claude-sonnet", "claude-opus"},
			wantStatus: lane.Blocked,
			wantReason: "families",
		},
		{
			name:       "runner error",
			plan:       risk.Plan{Judges: 1},
			models:     []string{"claude-sonnet"},
			runErr:     errors.New("runner unavailable"),
			wantStatus: lane.Blocked,
			wantReason: "runner unavailable",
			wantCalls:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worktree := ""
			if tt.plan.Judges > 0 && tt.wantCalls > 0 {
				worktree = newGitRepo(t)
			}

			var calls int
			var prompts []string
			run := func(_ context.Context, gotWorktree, _ string, prompt string) (string, error) {
				calls++
				if gotWorktree != worktree {
					t.Errorf("runner worktree = %q, want %q", gotWorktree, worktree)
				}
				prompts = append(prompts, prompt)
				if tt.runErr != nil {
					return "", tt.runErr
				}
				return tt.outputs[calls-1], nil
			}

			gate := Gate{
				Plan:   tt.plan,
				Models: tt.models,
				Run:    run,
			}
			gotStatus, gotReason := gate.PreCommit(context.Background(), worktree, packet.Packet{Body: "implement the goal"})
			if gotStatus != tt.wantStatus {
				t.Errorf("PreCommit() status = %q, want %q (reason %q)", gotStatus, tt.wantStatus, gotReason)
			}
			if tt.wantReason != "" && !strings.Contains(gotReason, tt.wantReason) {
				t.Errorf("PreCommit() reason = %q, want it to contain %q", gotReason, tt.wantReason)
			}
			if calls != tt.wantCalls {
				t.Errorf("runner calls = %d, want %d", calls, tt.wantCalls)
			}
			for i := 1; i < len(prompts); i++ {
				if prompts[i] != prompts[0] {
					t.Errorf("judge prompts differ:\nfirst: %q\nlater: %q", prompts[0], prompts[i])
				}
			}
		})
	}
}

func TestGatePreCommitBlocksWorktreeMutation(t *testing.T) {
	worktree := newGitRepo(t)
	run := func(_ context.Context, worktreePath, _, _ string) (string, error) {
		if err := os.WriteFile(filepath.Join(worktreePath, "judge-write.txt"), []byte("forbidden"), 0600); err != nil {
			return "", err
		}
		return "[]", nil
	}

	status, reason := (Gate{
		Plan:   risk.Plan{Judges: 1},
		Models: []string{"claude-sonnet"},
		Run:    run,
	}).PreCommit(context.Background(), worktree, packet.Packet{Body: "goal"})

	if status != lane.Blocked {
		t.Errorf("PreCommit() status = %q, want %q", status, lane.Blocked)
	}
	if reason != "judge modified the worktree" {
		t.Errorf("PreCommit() reason = %q, want %q", reason, "judge modified the worktree")
	}
}

func newGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.name", "Judges Test")
	runGit(t, dir, "config", "user.email", "judges@example.invalid")
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0600); err != nil {
		t.Fatalf("write base file: %v", err)
	}
	runGit(t, dir, "add", "base.txt")
	runGit(t, dir, "commit", "-q", "-m", "base")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}

func TestCursorRunner(t *testing.T) {
	dir := t.TempDir()
	argvPath := filepath.Join(dir, "argv.txt")
	pwdPath := filepath.Join(dir, "pwd.txt")
	binary := filepath.Join(dir, "cursor-agent")
	script := fmt.Sprintf(`#!/bin/sh
pwd > %q
printf '%%s\n' "$@" > %q
printf '%%s\n' '{"result":"[]"}'
`, pwdPath, argvPath)
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatalf("write cursor stub: %v", err)
	}
	worktree := t.TempDir()

	got, err := CursorRunner(binary)(context.Background(), worktree, "gpt-5", "review this")
	if err != nil {
		t.Fatalf("CursorRunner() error = %v", err)
	}
	if got != "[]" {
		t.Errorf("CursorRunner() = %q, want %q", got, "[]")
	}
	rawArgs, err := os.ReadFile(argvPath)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	wantArgs := "--print\n--output-format\njson\n--mode\nplan\n--trust\n--model\ngpt-5\nreview this\n"
	if string(rawArgs) != wantArgs {
		t.Errorf("argv = %q, want %q", rawArgs, wantArgs)
	}
	rawPWD, err := os.ReadFile(pwdPath)
	if err != nil {
		t.Fatalf("read pwd: %v", err)
	}
	if strings.TrimSpace(string(rawPWD)) != worktree {
		t.Errorf("working directory = %q, want %q", strings.TrimSpace(string(rawPWD)), worktree)
	}
}
