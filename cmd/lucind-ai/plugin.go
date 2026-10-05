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

// pluginAgy is the agy runner; tests replace it with a fake.
var pluginAgy agyplugin.Agy = agyplugin.ExecAgy{}

const pluginUsage = "usage: lucind-ai plugin install [--dir <staging root>]"

// pluginDispatch handles `lucind-ai plugin`.
func pluginDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "install" {
		_, _ = fmt.Fprintln(stderr, pluginUsage)
		return 1
	}

	fs := flag.NewFlagSet("plugin install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "", "staging root for the rendered plugin (default $XDG_DATA_HOME/lucind-ai/agy-plugin)")
	if err := fs.Parse(args[1:]); err != nil {
		return 1
	}
	if len(fs.Args()) > 0 {
		_, _ = fmt.Fprintln(stderr, pluginUsage)
		return 1
	}

	root := *dir
	if root == "" {
		var err error
		if root, err = agyplugin.StagingRoot(); err != nil {
			_, _ = fmt.Fprintf(stderr, "lucind-ai: plugin install: %v\n", err)
			return 1
		}
	}

	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: plugin install: resolve binary path: %v\n", err)
		return 1
	}

	obsolete, err := agyplugin.ObsoleteDir()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: plugin install: %v\n", err)
		return 1
	}
	pluginDir, err := agyplugin.Setup(ctx, agyplugin.Options{
		StagingRoot: root,
		Bin:         exe,
		Agy:         pluginAgy,
		ObsoleteDir: obsolete,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: plugin install: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "installed lucind via agy plugin install (staged at %s)\n", pluginDir)
	return 0
}
