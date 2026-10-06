package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/LanzerDevCorp/lucind-ai/internal/agyplugin"
	"github.com/LanzerDevCorp/lucind-ai/internal/claudemd"
	"github.com/LanzerDevCorp/lucind-ai/internal/claudeplugin"
	claudecode "github.com/LanzerDevCorp/lucind-ai/plugin/claude-code"
)

var (
	claudeInstall   = claudeplugin.InstallVariant
	claudemdInstall = claudemd.Install
	userHomeDir     = os.UserHomeDir
)

const installUsage = "usage: lucind-ai install [--no-claude-md] [--reset-key]"

func runInstall(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	var noClaudeMD, resetKey bool
	if len(args) == 1 {
		switch args[0] {
		case "--help", "-help", "-h", "help":
			// Help is only valid on its own; next to other arguments it is a usage error.
			_, _ = fmt.Fprintln(stdout, installUsage)
			return 0
		}
	}
	for _, arg := range args {
		switch arg {
		case "--no-claude-md":
			noClaudeMD = true
		case "--reset-key":
			resetKey = true
		default:
			_, _ = fmt.Fprintln(stderr, installUsage)
			return 1
		}
	}

	// 0. API key and skill variant
	variant, err := determineVariantAndSetupKey(stdout, stderr, resetKey)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	// 1. Claude skill
	skillDir, err := claudeInstall(variant)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: install claude skill: %v\n", err)
		return 1
	}
	variantDesc := "manual variant"
	if variant == claudeplugin.VariantAuto {
		variantDesc = "auto-skills variant"
	}
	_, _ = fmt.Fprintf(stdout, "installed claude skill (%s) into %s\n", variantDesc, skillDir)

	// 2. lucind agy plugin
	root, err := agyplugin.StagingRoot()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: install lucind: %v\n", err)
		return 1
	}

	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: install lucind: resolve binary path: %v\n", err)
		return 1
	}

	obsolete, err := agyplugin.ObsoleteDir()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: install lucind: %v\n", err)
		return 1
	}

	pluginDir, err := agyplugin.Setup(ctx, agyplugin.Options{
		StagingRoot: root,
		Bin:         exe,
		Agy:         pluginAgy,
		ObsoleteDir: obsolete,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: install lucind: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "installed lucind via agy plugin install (staged at %s)\n", pluginDir)

	// 3. lucind-roles agy plugin
	rolesDir, err := agyplugin.SetupRoles(ctx, agyplugin.Options{
		StagingRoot: root,
		Agy:         pluginAgy,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: install lucind-roles: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "installed lucind-roles via agy plugin install (staged at %s)\n", rolesDir)

	// 4. Claude dispatch block
	if !noClaudeMD {
		home, err := userHomeDir()
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "lucind-ai: install claude md: resolve user home directory: %v\n", err)
			return 1
		}
		target := filepath.Join(home, ".claude", "CLAUDE.md")
		outcome, err := claudemdInstall(target, claudecode.Dispatch)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "lucind-ai: install claude md: %v\n", err)
			return 1
		}
		_, _ = fmt.Fprintf(stdout, "lucind dispatch block %s in %s\n", outcome, target)
	}

	return 0
}
