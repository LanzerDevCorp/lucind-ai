package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/LanzerDevCorp/lucind-ai/internal/agyplugin"
)

const pluginUsage = "usage: lucind-ai plugin install [--dir <plugins root>]"

// pluginDispatch handles `lucind-ai plugin`.
func pluginDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "install" {
		fmt.Fprintln(stderr, pluginUsage)
		return 1
	}

	fs := flag.NewFlagSet("plugin install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "", "agy plugins root (default ~/.gemini/antigravity-cli/plugins)")
	if err := fs.Parse(args[1:]); err != nil {
		return 1
	}
	if len(fs.Args()) > 0 {
		fmt.Fprintln(stderr, pluginUsage)
		return 1
	}

	root := *dir
	if root == "" {
		var err error
		if root, err = agyplugin.DefaultRoot(); err != nil {
			fmt.Fprintf(stderr, "lucind-ai: plugin install: %v\n", err)
			return 1
		}
	}

	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: plugin install: resolve binary path: %v\n", err)
		return 1
	}

	pluginDir, err := agyplugin.Install(root, exe)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: plugin install: %v\n", err)
		return 1
	}
	validated, err := agyplugin.Validate(ctx, pluginDir)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: plugin install: %v\n", err)
		return 1
	}
	if validated {
		fmt.Fprintf(stdout, "installed and validated %s\n", pluginDir)
	} else {
		fmt.Fprintf(stdout, "installed %s (agy not on PATH; not validated)\n", pluginDir)
	}
	return 0
}
