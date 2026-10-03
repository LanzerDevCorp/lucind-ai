package risk_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/candidatechange"
	"github.com/LanzerDevCorp/lucind-ai/internal/risk"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name        string
		input       risk.Input
		wantTier    risk.Tier
		wantReasons []string
	}{
		// Rule 1: Empty changes
		{
			name: "empty changes fail closed to high",
			input: risk.Input{
				Changes: nil,
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"classifier_error:no_changes"},
		},
		{
			name: "empty changes slice fail closed to high",
			input: risk.Input{
				Changes: []candidatechange.Change{},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"classifier_error:no_changes"},
		},

		// Rule 2: HIGH path tokens
		{
			name: "path token auth positive",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "internal/auth/login.go"},
				},
				Read: func(path string) ([]byte, error) { return []byte("package auth"), nil },
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"path_token:auth:internal/auth/login.go"},
		},
		{
			name: "path token oauth near miss negative",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "internal/oauth/x.go"},
				},
				Read: func(path string) ([]byte, error) { return []byte("package oauth"), nil },
			},
			wantTier:    risk.TierMedium,
			wantReasons: nil,
		},
		{
			name: "path token authentication in doc near miss negative",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "docs/authentication.md"},
				},
				Read: func(path string) ([]byte, error) { return []byte("# Authentication Guide"), nil },
			},
			wantTier:    risk.TierPassive,
			wantReasons: nil,
		},
		{
			name: "path token update positive",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Created, Path: "update.go"},
				},
				Read: func(path string) ([]byte, error) { return []byte("package main"), nil },
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"path_token:update:update.go"},
		},
		{
			name: "path token updated near miss negative",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Created, Path: "updated.go"},
				},
				Read: func(path string) ([]byte, error) { return []byte("package main"), nil },
			},
			wantTier:    risk.TierMedium,
			wantReasons: nil,
		},
		{
			name: "path token security positive",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "pkg/security/check.go"},
				},
				Read: func(path string) ([]byte, error) { return []byte("package security"), nil },
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"path_token:security:pkg/security/check.go"},
		},
		{
			name: "path token insecurity near miss negative",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "pkg/insecurity/check.go"},
				},
				Read: func(path string) ([]byte, error) { return []byte("package insecurity"), nil },
			},
			wantTier:    risk.TierMedium,
			wantReasons: nil,
		},
		{
			name: "path token webhook positive",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "services/webhook/send.go"},
				},
				Read: func(path string) ([]byte, error) { return []byte("package webhook"), nil },
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"path_token:webhook:services/webhook/send.go"},
		},
		{
			name: "path token webhooks near miss negative",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "services/webhooks/send.go"},
				},
				Read: func(path string) ([]byte, error) { return []byte("package webhooks"), nil },
			},
			wantTier:    risk.TierMedium,
			wantReasons: nil,
		},
		{
			name: "path token payments positive",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "billing/payments/charge.go"},
				},
				Read: func(path string) ([]byte, error) { return []byte("package payments"), nil },
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"path_token:payments:billing/payments/charge.go"},
		},
		{
			name: "path token payment near miss negative",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "billing/payment/charge.go"},
				},
				Read: func(path string) ([]byte, error) { return []byte("package payment"), nil },
			},
			wantTier:    risk.TierMedium,
			wantReasons: nil,
		},
		{
			name: "path token in SourcePath on copy",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Copied, SourcePath: "auth/old.go", Path: "clean/new.go"},
				},
				Read: func(path string) ([]byte, error) { return []byte("package clean"), nil },
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"path_token:auth:auth/old.go"},
		},

		// Rule 3: HIGH sensitive locations
		{
			name: "sensitive path internal/ledger/ positive",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "internal/ledger/entry.go"},
				},
				Read: func(path string) ([]byte, error) { return []byte("package ledger"), nil },
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"sensitive_path:internal/ledger/entry.go"},
		},
		{
			name: "sensitive path internal/attest/ positive",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "internal/attest/attest.go"},
				},
				Read: func(path string) ([]byte, error) { return []byte("package attest"), nil },
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"sensitive_path:internal/attest/attest.go"},
		},
		{
			name: "sensitive path internal/accept/ positive",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "internal/accept/accept.go"},
				},
				Read: func(path string) ([]byte, error) { return []byte("package accept"), nil },
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"sensitive_path:internal/accept/accept.go"},
		},
		{
			name: "sensitive path .github/workflows/ positive",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: ".github/workflows/ci.yml"},
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"sensitive_path:.github/workflows/ci.yml"},
		},
		{
			name: "sensitive path Makefile exact name positive",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "Makefile"},
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"sensitive_path:Makefile"},
		},
		{
			name: "sensitive path go.mod exact name positive",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "go.mod"},
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"sensitive_path:go.mod"},
		},
		{
			name: "sensitive path go.sum exact name positive",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "go.sum"},
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"sensitive_path:go.sum"},
		},
		{
			name: "sensitive path lucind-checks.sh exact name positive",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "lucind-checks.sh"},
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"sensitive_path:lucind-checks.sh"},
		},
		{
			name: "sensitive path .claude-plugin/ positive",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: ".claude-plugin/plugin.json"},
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"sensitive_path:.claude-plugin/plugin.json"},
		},
		{
			name: "sensitive path plugin/claude-code/.claude-plugin/ positive",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "plugin/claude-code/.claude-plugin/manifest.json"},
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"active_content:plugin/claude-code/.claude-plugin/manifest.json", "sensitive_path:plugin/claude-code/.claude-plugin/manifest.json"},
		},
		{
			name: "sensitive path near-miss negative for Makefile",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "docs/Makefile.txt"},
				},
				Read: func(path string) ([]byte, error) { return []byte("how to use make"), nil },
			},
			wantTier:    risk.TierPassive,
			wantReasons: nil,
		},
		{
			name: "sensitive path near-miss negative for internal/ledger_helper",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "internal/ledger_helper/x.go"},
				},
				Read: func(path string) ([]byte, error) { return []byte("package ledger_helper"), nil },
			},
			wantTier:    risk.TierMedium,
			wantReasons: nil,
		},
		{
			name: "sensitive path in SourcePath on copy",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Copied, SourcePath: "Makefile", Path: "scripts/build.mk"},
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"sensitive_path:Makefile"},
		},

		// Rule 4: HIGH process/network/permission signals in Go sources
		{
			name: "signal os/exec in go source",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "pkg/runner.go"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("package pkg\nimport \"os/exec\"\n"), nil
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"process_boundary:pkg/runner.go"},
		},
		{
			name: "signal exec.Command in go source",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "pkg/runner.go"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("package pkg\nfunc run() { exec.Command(\"ls\") }\n"), nil
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"process_boundary:pkg/runner.go"},
		},
		{
			name: "signal syscall in go source",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "pkg/sys.go"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("package pkg\nimport \"syscall\"\n"), nil
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"process_boundary:pkg/sys.go"},
		},
		{
			name: "signal net/http in go source",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "pkg/web.go"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("package pkg\nimport \"net/http\"\n"), nil
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"network:pkg/web.go"},
		},
		{
			name: "signal net in go source",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "pkg/conn.go"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("package pkg\nimport \"net\"\n"), nil
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"network:pkg/conn.go"},
		},
		{
			name: "words containing net or syscall in comments and strings are not signals",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "pkg/words.go"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("package pkg\n// internet, network, planet and a syscall-free design\nvar s = \"cabinet\"\n"), nil
				},
			},
			wantTier:    risk.TierMedium,
			wantReasons: nil,
		},
		{
			name: "signal os.Chmod in go source",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "pkg/fs.go"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("package pkg\nfunc f() { os.Chmod(\"a\", 0777) }\n"), nil
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"permissions:pkg/fs.go"},
		},
		{
			name: "signal os.Chown in go source",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "pkg/fs.go"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("package pkg\nfunc f() { os.Chown(\"a\", 1, 1) }\n"), nil
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"permissions:pkg/fs.go"},
		},
		{
			name: "signal os.Remove in go source",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "pkg/clean.go"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("package pkg\nfunc f() { os.Remove(\"a\") }\n"), nil
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"deletion_api:pkg/clean.go"},
		},
		{
			name: "signal os.RemoveAll in go source",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "pkg/clean.go"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("package pkg\nfunc f() { os.RemoveAll(\"a\") }\n"), nil
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"deletion_api:pkg/clean.go"},
		},
		{
			name: "safe go source with no signals near-miss negative",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "pkg/safe.go"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("package pkg\nfunc Safe() string { return \"safe\" }\n"), nil
				},
			},
			wantTier:    risk.TierMedium,
			wantReasons: nil,
		},
		{
			name: "signals in _test.go ignored",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "pkg/clean_test.go"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("package pkg\nimport \"os/exec\"\nfunc TestX(t *testing.T) { exec.Command(\"echo\"); os.RemoveAll(\"tmp\") }\n"), nil
				},
			},
			wantTier:    risk.TierMedium,
			wantReasons: nil,
		},
		{
			name: "go source read error triggers classifier_error:read",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "pkg/bad.go"},
				},
				Read: func(path string) ([]byte, error) {
					return nil, errors.New("cannot read file")
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"classifier_error:read:pkg/bad.go"},
		},
		{
			name: "go source nil Read triggers classifier_error:read",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "pkg/bad.go"},
				},
				Read: nil,
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"classifier_error:read:pkg/bad.go"},
		},

		// Rule 5: MEDIUM floor
		{
			name: "deleted file medium",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Deleted, Path: "docs/old.md"},
				},
			},
			wantTier:    risk.TierMedium,
			wantReasons: []string{"deleted:docs/old.md"},
		},
		{
			name: "deleted go file medium",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Deleted, Path: "pkg/old.go"},
				},
			},
			wantTier:    risk.TierMedium,
			wantReasons: []string{"deleted:pkg/old.go"},
		},
		{
			name: "SKILL.md never passive",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "skills/test/SKILL.md"},
				},
				Read: func(path string) ([]byte, error) { return []byte("# Skill"), nil },
			},
			wantTier:    risk.TierMedium,
			wantReasons: []string{"active_content:skills/test/SKILL.md"},
		},
		{
			name: "CLAUDE.md never passive",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "CLAUDE.md"},
				},
				Read: func(path string) ([]byte, error) { return []byte("# Instructions"), nil },
			},
			wantTier:    risk.TierMedium,
			wantReasons: []string{"active_content:CLAUDE.md"},
		},
		{
			name: "AGENTS.md never passive",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "AGENTS.md"},
				},
				Read: func(path string) ([]byte, error) { return []byte("# Agents"), nil },
			},
			wantTier:    risk.TierMedium,
			wantReasons: []string{"active_content:AGENTS.md"},
		},
		{
			name: "GEMINI.md never passive",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "GEMINI.md"},
				},
				Read: func(path string) ([]byte, error) { return []byte("# Gemini"), nil },
			},
			wantTier:    risk.TierMedium,
			wantReasons: []string{"active_content:GEMINI.md"},
		},
		{
			name: "change under plugin/ is active content",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "plugin/something/guide.md"},
				},
				Read: func(path string) ([]byte, error) { return []byte("# Guide"), nil },
			},
			wantTier:    risk.TierMedium,
			wantReasons: []string{"active_content:plugin/something/guide.md"},
		},
		{
			name: "change under .agents/ is active content",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: ".agents/config.txt"},
				},
				Read: func(path string) ([]byte, error) { return []byte("agent config"), nil },
			},
			wantTier:    risk.TierMedium,
			wantReasons: []string{"active_content:.agents/config.txt"},
		},
		{
			name: "change under .claude/ is active content",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: ".claude/settings.txt"},
				},
				Read: func(path string) ([]byte, error) { return []byte("settings"), nil },
			},
			wantTier:    risk.TierMedium,
			wantReasons: []string{"active_content:.claude/settings.txt"},
		},
		{
			name: "non-passive config file is medium",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "configs/app.json"},
				},
			},
			wantTier:    risk.TierMedium,
			wantReasons: nil,
		},

		// Rule 6: PASSIVE byte-proven content
		{
			name: "README passive with valid UTF-8",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "README.md"},
				},
				Read: func(path string) ([]byte, error) { return []byte("# Hello World\nThis is valid UTF-8.\n"), nil },
			},
			wantTier:    risk.TierPassive,
			wantReasons: nil,
		},
		{
			name: "README with a NUL byte => HIGH",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "README.md"},
				},
				Read: func(path string) ([]byte, error) { return []byte("Hello\x00World"), nil },
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"classifier_error:unproven:README.md"},
		},
		{
			name: "doc.txt with invalid UTF-8 => HIGH",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "notes.txt"},
				},
				Read: func(path string) ([]byte, error) { return []byte{0xff, 0xfe, 0xfd}, nil },
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"classifier_error:unproven:notes.txt"},
		},
		{
			name: "PNG with correct magic",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Created, Path: "images/logo.png"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("\x89PNG\r\n\x1a\nrest_of_png_bytes"), nil
				},
			},
			wantTier:    risk.TierPassive,
			wantReasons: nil,
		},
		{
			name: "PNG with wrong magic",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Created, Path: "images/logo.png"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("NOT_A_PNG_FILE"), nil
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"classifier_error:unproven:images/logo.png"},
		},
		{
			name: "JPG with correct magic",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Created, Path: "images/photo.jpg"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte{0xff, 0xd8, 0xff, 0xe0, 0x00}, nil
				},
			},
			wantTier:    risk.TierPassive,
			wantReasons: nil,
		},
		{
			name: "JPEG with correct magic",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Created, Path: "images/photo.jpeg"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte{0xff, 0xd8, 0xff, 0x01}, nil
				},
			},
			wantTier:    risk.TierPassive,
			wantReasons: nil,
		},
		{
			name: "JPEG with wrong magic",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Created, Path: "images/photo.jpeg"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte{0x01, 0x02, 0x03}, nil
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"classifier_error:unproven:images/photo.jpeg"},
		},
		{
			name: "JPG with wrong magic",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Created, Path: "images/photo.jpg"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte{0x00, 0x01, 0x02}, nil
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"classifier_error:unproven:images/photo.jpg"},
		},
		{
			name: "GIF87a with correct magic",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Created, Path: "images/anim.gif"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("GIF87a_data"), nil
				},
			},
			wantTier:    risk.TierPassive,
			wantReasons: nil,
		},
		{
			name: "GIF89a with correct magic",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Created, Path: "images/anim.gif"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("GIF89a_data"), nil
				},
			},
			wantTier:    risk.TierPassive,
			wantReasons: nil,
		},
		{
			name: "GIF with wrong magic",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Created, Path: "images/anim.gif"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("GIF99a_data"), nil
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"classifier_error:unproven:images/anim.gif"},
		},
		{
			name: "case-insensitive extension for doc",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Created, Path: "DOCS.MD"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("valid content"), nil
				},
			},
			wantTier:    risk.TierPassive,
			wantReasons: nil,
		},
		{
			name: "case-insensitive extension for png",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Created, Path: "IMAGE.PNG"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("\x89PNG\r\n\x1a\nbytes"), nil
				},
			},
			wantTier:    risk.TierPassive,
			wantReasons: nil,
		},
		{
			name: "nil Read for a doc => HIGH",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "docs/readme.md"},
				},
				Read: nil,
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"classifier_error:nil_read"},
		},
		{
			name: "Read error for a doc => HIGH",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "docs/readme.md"},
				},
				Read: func(path string) ([]byte, error) {
					return nil, errors.New("read failed")
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"classifier_error:read:docs/readme.md"},
		},
		{
			name: "Copied doc with passive SourcePath",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Copied, SourcePath: "docs/old.md", Path: "docs/new.md"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("copied doc content"), nil
				},
			},
			wantTier:    risk.TierPassive,
			wantReasons: nil,
		},
		{
			name: "Copied doc with non-passive SourcePath is medium",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Copied, SourcePath: "pkg/old.go", Path: "docs/new.md"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("copied doc content"), nil
				},
			},
			wantTier:    risk.TierMedium,
			wantReasons: nil,
		},

		// Mixed sets
		{
			name: "mixed doc + code => at least medium",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "docs/readme.md"},
					{Change: candidatechange.Modified, Path: "pkg/safe.go"},
				},
				Read: func(path string) ([]byte, error) {
					if path == "docs/readme.md" {
						return []byte("# Readme"), nil
					}
					return []byte("package pkg\nfunc Safe() {}\n"), nil
				},
			},
			wantTier:    risk.TierMedium,
			wantReasons: nil,
		},
		{
			name: "doc + sensitive => HIGH",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "docs/readme.md"},
					{Change: candidatechange.Modified, Path: "Makefile"},
				},
				Read: func(path string) ([]byte, error) {
					return []byte("# Readme"), nil
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"sensitive_path:Makefile"},
		},
		{
			name: "doc + high token => HIGH",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "docs/readme.md"},
					{Change: candidatechange.Modified, Path: "pkg/auth/login.go"},
				},
				Read: func(path string) ([]byte, error) {
					if path == "docs/readme.md" {
						return []byte("# Readme"), nil
					}
					return []byte("package auth"), nil
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"path_token:auth:pkg/auth/login.go"},
		},
		{
			name: "unproven doc in mixed set => HIGH",
			input: risk.Input{
				Changes: []candidatechange.Change{
					{Change: candidatechange.Modified, Path: "docs/readme.md"},
					{Change: candidatechange.Modified, Path: "pkg/safe.go"},
				},
				Read: func(path string) ([]byte, error) {
					if path == "docs/readme.md" {
						return []byte("bad\x00byte"), nil
					}
					return []byte("package pkg\nfunc Safe() {}\n"), nil
				},
			},
			wantTier:    risk.TierHigh,
			wantReasons: []string{"classifier_error:unproven:docs/readme.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := risk.Classify(tt.input)
			if got.Tier != tt.wantTier {
				t.Errorf("Classify().Tier = %q, want %q", got.Tier, tt.wantTier)
			}
			if !reflect.DeepEqual(got.Reasons, tt.wantReasons) {
				t.Errorf("Classify().Reasons = %v, want %v", got.Reasons, tt.wantReasons)
			}
		})
	}
}

func TestClassifyPanicRecovery(t *testing.T) {
	in := risk.Input{
		Changes: []candidatechange.Change{
			{Change: candidatechange.Modified, Path: "docs/readme.md"},
		},
		Read: func(path string) ([]byte, error) {
			panic("unexpected read panic")
		},
	}

	got := risk.Classify(in)
	if got.Tier != risk.TierHigh {
		t.Fatalf("panic recovery tier = %q, want %q", got.Tier, risk.TierHigh)
	}
	wantReasons := []string{"classifier_error:panic"}
	if !reflect.DeepEqual(got.Reasons, wantReasons) {
		t.Fatalf("panic recovery reasons = %v, want %v", got.Reasons, wantReasons)
	}
}

func TestClassifyDeterminismAndDeduplication(t *testing.T) {
	in := risk.Input{
		Changes: []candidatechange.Change{
			{Change: candidatechange.Modified, Path: "internal/ledger/auth/update.go"},
			{Change: candidatechange.Deleted, Path: "internal/attest/old.go"},
		},
		Read: func(path string) ([]byte, error) {
			return []byte("package test\nimport \"os/exec\"\nfunc Run() { exec.Command(\"sh\") }\n"), nil
		},
	}

	first := risk.Classify(in)
	for i := 0; i < 5; i++ {
		next := risk.Classify(in)
		if !reflect.DeepEqual(first, next) {
			t.Fatalf("iteration %d produced different result: %+v vs %+v", i, first, next)
		}
	}

	// Verify reasons are sorted and unique
	for i := 1; i < len(first.Reasons); i++ {
		if first.Reasons[i] <= first.Reasons[i-1] {
			t.Fatalf("reasons not sorted or not unique: %v", first.Reasons)
		}
	}
}
