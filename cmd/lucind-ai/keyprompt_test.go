package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/claudeplugin"
	"github.com/LanzerDevCorp/lucind-ai/internal/userconfig"
)

func TestInstall_KeyResolves_UsesAutoVariantWithoutPrompt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tempXDG := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempXDG)
	t.Setenv("TYPESAFE_API_KEY", "env-resolved-key")

	fakeAgy := &fakeInstallAgy{}
	origAgy := pluginAgy
	pluginAgy = fakeAgy
	defer func() { pluginAgy = origAgy }()

	var installedVariant claudeplugin.Variant
	origClaudeInstall := claudeInstall
	claudeInstall = func(v claudeplugin.Variant) (string, error) {
		installedVariant = v
		return filepath.Join(home, ".claude", "skills", "lucind"), nil
	}
	defer func() { claudeInstall = origClaudeInstall }()

	promptCalled := false
	origReadSecret := readSecretKey
	readSecretKey = func(stderr io.Writer) (string, error) {
		promptCalled = true
		return "", nil
	}
	defer func() { readSecretKey = origReadSecret }()

	var stdout, stderr bytes.Buffer
	code := runInstall(context.Background(), nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runInstall exit code = %d, want 0; stderr: %s", code, stderr.String())
	}

	if promptCalled {
		t.Errorf("readSecretKey was called when key already resolved")
	}
	if installedVariant != claudeplugin.VariantAuto {
		t.Errorf("installedVariant = %q, want %q", installedVariant, claudeplugin.VariantAuto)
	}
	if !strings.Contains(stdout.String(), "installed claude skill (auto-skills variant) into ") {
		t.Errorf("stdout missing auto-skills variant line: %s", stdout.String())
	}
}

func TestInstall_NoKey_WithTerminal_StoresKeyAndUsesAutoVariant(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tempXDG := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempXDG)
	t.Setenv("TYPESAFE_API_KEY", "")

	fakeAgy := &fakeInstallAgy{}
	origAgy := pluginAgy
	pluginAgy = fakeAgy
	defer func() { pluginAgy = origAgy }()

	origIsTerm := isStdinTerminal
	isStdinTerminal = func() bool { return true }
	defer func() { isStdinTerminal = origIsTerm }()

	const userEnteredKey = "user-prompted-secret-key"
	promptPrompted := false
	origReadSecret := readSecretKey
	readSecretKey = func(w io.Writer) (string, error) {
		promptPrompted = true
		return userEnteredKey, nil
	}
	defer func() { readSecretKey = origReadSecret }()

	var installedVariant claudeplugin.Variant
	origClaudeInstall := claudeInstall
	claudeInstall = func(v claudeplugin.Variant) (string, error) {
		installedVariant = v
		return filepath.Join(home, ".claude", "skills", "lucind"), nil
	}
	defer func() { claudeInstall = origClaudeInstall }()

	var stdout, stderr bytes.Buffer
	code := runInstall(context.Background(), nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runInstall exit code = %d, want 0; stderr: %s", code, stderr.String())
	}

	if !promptPrompted {
		t.Errorf("prompt was not called when terminal was present and key unset")
	}
	if installedVariant != claudeplugin.VariantAuto {
		t.Errorf("installedVariant = %q, want %q", installedVariant, claudeplugin.VariantAuto)
	}
	if !strings.Contains(stdout.String(), "installed claude skill (auto-skills variant) into ") {
		t.Errorf("stdout missing auto-skills variant line: %s", stdout.String())
	}

	storedKey, err := userconfig.ReadKey("TYPESAFE_API_KEY")
	if err != nil {
		t.Fatalf("ReadKey error: %v", err)
	}
	if storedKey != userEnteredKey {
		t.Errorf("storedKey = %q, want %q", storedKey, userEnteredKey)
	}
}

func TestInstall_NoKey_WithTerminal_EmptyAnswerSkipsKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tempXDG := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempXDG)
	t.Setenv("TYPESAFE_API_KEY", "")

	fakeAgy := &fakeInstallAgy{}
	origAgy := pluginAgy
	pluginAgy = fakeAgy
	defer func() { pluginAgy = origAgy }()

	origIsTerm := isStdinTerminal
	isStdinTerminal = func() bool { return true }
	defer func() { isStdinTerminal = origIsTerm }()

	origReadSecret := readSecretKey
	readSecretKey = func(w io.Writer) (string, error) {
		return "", nil // user pressed Enter
	}
	defer func() { readSecretKey = origReadSecret }()

	var installedVariant claudeplugin.Variant
	origClaudeInstall := claudeInstall
	claudeInstall = func(v claudeplugin.Variant) (string, error) {
		installedVariant = v
		return filepath.Join(home, ".claude", "skills", "lucind"), nil
	}
	defer func() { claudeInstall = origClaudeInstall }()

	var stdout, stderr bytes.Buffer
	code := runInstall(context.Background(), nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runInstall exit code = %d, want 0; stderr: %s", code, stderr.String())
	}

	if installedVariant != claudeplugin.VariantManual {
		t.Errorf("installedVariant = %q, want %q", installedVariant, claudeplugin.VariantManual)
	}
	if !strings.Contains(stdout.String(), "installed claude skill (manual variant) into ") {
		t.Errorf("stdout missing manual variant line: %s", stdout.String())
	}

	storedKey, err := userconfig.ReadKey("TYPESAFE_API_KEY")
	if err != nil {
		t.Fatalf("ReadKey error: %v", err)
	}
	if storedKey != "" {
		t.Errorf("storedKey = %q, want empty", storedKey)
	}
}

func TestInstall_NoKey_NoTerminal_NonInteractiveNotice(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tempXDG := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempXDG)
	t.Setenv("TYPESAFE_API_KEY", "")

	fakeAgy := &fakeInstallAgy{}
	origAgy := pluginAgy
	pluginAgy = fakeAgy
	defer func() { pluginAgy = origAgy }()

	origIsTerm := isStdinTerminal
	isStdinTerminal = func() bool { return false }
	defer func() { isStdinTerminal = origIsTerm }()

	promptCalled := false
	origReadSecret := readSecretKey
	readSecretKey = func(w io.Writer) (string, error) {
		promptCalled = true
		return "should-not-be-called", nil
	}
	defer func() { readSecretKey = origReadSecret }()

	var installedVariant claudeplugin.Variant
	origClaudeInstall := claudeInstall
	claudeInstall = func(v claudeplugin.Variant) (string, error) {
		installedVariant = v
		return filepath.Join(home, ".claude", "skills", "lucind"), nil
	}
	defer func() { claudeInstall = origClaudeInstall }()

	var stdout, stderr bytes.Buffer
	code := runInstall(context.Background(), nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runInstall exit code = %d, want 0; stderr: %s", code, stderr.String())
	}

	if promptCalled {
		t.Errorf("prompt was called without a terminal")
	}
	if installedVariant != claudeplugin.VariantManual {
		t.Errorf("installedVariant = %q, want %q", installedVariant, claudeplugin.VariantManual)
	}

	out := stdout.String()
	if !strings.Contains(out, "installed claude skill (manual variant) into ") {
		t.Errorf("stdout missing manual variant line: %s", out)
	}
	// Check the non-interactive notice line
	if !strings.Contains(out, "lucind-ai install in a terminal") || !strings.Contains(out, "~/.config/lucind/env") {
		t.Errorf("stdout missing non-interactive key configuration instructions: %s", out)
	}
}

func TestInstall_ExistingKeyNeverOverwritten(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tempXDG := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempXDG)

	// Pre-create existing key in userconfig file
	lucindDir := filepath.Join(tempXDG, "lucind")
	if err := os.MkdirAll(lucindDir, 0700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(lucindDir, "env"), []byte("TYPESAFE_API_KEY=existing-file-key\n"), 0600); err != nil {
		t.Fatalf("write env file failed: %v", err)
	}
	t.Setenv("TYPESAFE_API_KEY", "")

	fakeAgy := &fakeInstallAgy{}
	origAgy := pluginAgy
	pluginAgy = fakeAgy
	defer func() { pluginAgy = origAgy }()

	origIsTerm := isStdinTerminal
	isStdinTerminal = func() bool { return true }
	defer func() { isStdinTerminal = origIsTerm }()

	promptCalled := false
	origReadSecret := readSecretKey
	readSecretKey = func(w io.Writer) (string, error) {
		promptCalled = true
		return "new-key-should-not-overwrite", nil
	}
	defer func() { readSecretKey = origReadSecret }()

	var installedVariant claudeplugin.Variant
	origClaudeInstall := claudeInstall
	claudeInstall = func(v claudeplugin.Variant) (string, error) {
		installedVariant = v
		return filepath.Join(home, ".claude", "skills", "lucind"), nil
	}
	defer func() { claudeInstall = origClaudeInstall }()

	var stdout, stderr bytes.Buffer
	code := runInstall(context.Background(), nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runInstall exit code = %d, want 0", code)
	}

	if promptCalled {
		t.Errorf("prompt was called when key already existed in config file")
	}
	if installedVariant != claudeplugin.VariantAuto {
		t.Errorf("installedVariant = %q, want %q", installedVariant, claudeplugin.VariantAuto)
	}

	key, err := userconfig.ReadKey("TYPESAFE_API_KEY")
	if err != nil {
		t.Fatalf("ReadKey error: %v", err)
	}
	if key != "existing-file-key" {
		t.Errorf("key was overwritten: got %q, want %q", key, "existing-file-key")
	}
}
