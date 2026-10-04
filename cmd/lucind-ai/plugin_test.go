package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginInstall_WritesToDir(t *testing.T) {
	t.Setenv("PATH", "") // no agy on PATH: validation is skipped
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := pluginDispatch(context.Background(), []string{"install", "--dir", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	exe, _ := os.Executable()
	data, err := os.ReadFile(filepath.Join(root, "lucind", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), exe) {
		t.Errorf("hooks.json does not reference %s:\n%s", exe, data)
	}
	if !strings.Contains(stdout.String(), filepath.Join(root, "lucind")) {
		t.Errorf("stdout %q does not name the plugin dir", stdout.String())
	}
}

func TestPluginDispatch_Usage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := pluginDispatch(context.Background(), []string{"bogus"}, &stdout, &stderr); code != 1 {
		t.Fatalf("code=%d, want 1", code)
	}
	if code := pluginDispatch(context.Background(), nil, &stdout, &stderr); code != 1 {
		t.Fatalf("code=%d, want 1", code)
	}
}
