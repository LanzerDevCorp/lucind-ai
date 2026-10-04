package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/LanzerDevCorp/lucind-ai/internal/agyplugin"
	"github.com/LanzerDevCorp/lucind-ai/internal/claudeplugin"
)

var claudeInstall = claudeplugin.Install

const installUsage = "usage: lucind-ai install"

func runInstall(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		if len(args) == 1 {
			switch args[0] {
			case "--help", "-help", "-h", "help":
				fmt.Fprintln(stdout, installUsage)
				return 0
			}
		}
		fmt.Fprintln(stderr, installUsage)
		return 1
	}

	// 1. Claude skill
	skillDir, err := claudeInstall()
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: install claude skill: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "installed claude skill into %s\n", skillDir)

	// 2. lucind agy plugin
	root, err := agyplugin.StagingRoot()
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: install lucind: %v\n", err)
		return 1
	}

	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: install lucind: resolve binary path: %v\n", err)
		return 1
	}

	obsolete, err := agyplugin.ObsoleteDir()
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: install lucind: %v\n", err)
		return 1
	}

	pluginDir, err := agyplugin.Setup(ctx, agyplugin.Options{
		StagingRoot: root,
		Bin:         exe,
		Agy:         pluginAgy,
		ObsoleteDir: obsolete,
	})
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: install lucind: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "installed lucind via agy plugin install (staged at %s)\n", pluginDir)

	// 3. lucind-roles agy plugin
	rolesDir, err := agyplugin.SetupRoles(ctx, agyplugin.Options{
		StagingRoot: root,
		Agy:         pluginAgy,
	})
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: install lucind-roles: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "installed lucind-roles via agy plugin install (staged at %s)\n", rolesDir)

	return 0
}
