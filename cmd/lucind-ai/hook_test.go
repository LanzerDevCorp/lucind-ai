package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runHook(t *testing.T, args []string, stdin string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := hookDispatch(context.Background(), args, strings.NewReader(stdin), &stdout, &stderr)
	return code, strings.TrimSpace(stdout.String()), stderr.String()
}

func TestHookPreToolUse_NoLaneAllows(t *testing.T) {
	t.Setenv("LUCIND_LANE", "")
	if err := os.Unsetenv("LUCIND_LANE"); err != nil {
		t.Fatalf("unsetenv LUCIND_LANE: %v", err)
	}
	code, out, _ := runHook(t, []string{"pre-tool-use"}, `{"workspacePaths":["/tmp"],"toolCall":{"name":"write_to_file","args":{}}}`)
	if code != 0 || out != `{"decision":"allow"}` {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestHookStop_NoLaneEnds(t *testing.T) {
	if err := os.Unsetenv("LUCIND_LANE"); err != nil {
		t.Fatalf("unsetenv LUCIND_LANE: %v", err)
	}
	code, out, _ := runHook(t, []string{"stop"}, `{}`)
	if code != 0 || out != `{}` {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestHook_UnknownSubcommandAndMissingFailOpen(t *testing.T) {
	if err := os.Unsetenv("LUCIND_LANE"); err != nil {
		t.Fatalf("unsetenv LUCIND_LANE: %v", err)
	}
	for _, args := range [][]string{nil, {"bogus"}} {
		code, out, stderr := runHook(t, args, `{}`)
		if code != 0 || out != `{}` || stderr == "" {
			t.Fatalf("args=%v code=%d out=%q stderr=%q", args, code, out, stderr)
		}
	}
}

func TestHookPreToolUse_InLaneDeniesOutsideAllow(t *testing.T) {
	root := t.TempDir()
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	t.Setenv("LUCIND_LANE", "20260101-120000-abcd")
	code, out, _ := runHook(t, []string{"pre-tool-use"},
		`{"workspacePaths":["`+root+`"],"toolCall":{"name":"write_to_file","args":{"TargetFile":"`+root+`/x.go"}}}`)
	// root is not a git repo, so the lane root cannot be resolved: fail closed.
	if code != 0 || !strings.Contains(out, `"decision":"deny"`) {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestHookPreInvocation_NoLaneReturnsEmpty(t *testing.T) {
	if err := os.Unsetenv("LUCIND_LANE"); err != nil {
		t.Fatalf("unsetenv LUCIND_LANE: %v", err)
	}
	code, out, stderr := runHook(t, []string{"pre-invocation"}, `{"conversationId":"main-1","workspacePaths":["/tmp"]}`)
	if code != 0 || out != `{}` || stderr != "" {
		t.Fatalf("code=%d out=%q stderr=%q", code, out, stderr)
	}
}

func TestHookPreInvocation_InvalidInputFailsOpen(t *testing.T) {
	t.Setenv("LUCIND_LANE", "20260101-120000-abcd")
	code, out, _ := runHook(t, []string{"pre-invocation"}, `{not json`)
	if code != 0 || out != `{}` {
		t.Fatalf("code=%d out=%q", code, out)
	}
}
