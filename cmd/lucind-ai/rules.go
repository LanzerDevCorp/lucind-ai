package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/LanzerDevCorp/lucind-ai/internal/rules"
)

func rulesDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 1
	}

	switch args[0] {
	case "init":
		return rulesInitDispatch(ctx, args[1:], stdout, stderr)
	case "generate":
		return rulesGenerateDispatch(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "lucind-ai: unknown rules subcommand %q\n%s\n", args[0], usage)
		return 1
	}
}

func rulesInitDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("rules init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai rules init [--root <dir>]")
		fs.PrintDefaults()
	}

	rootFlag := fs.String("root", "", "repository root directory (default: git rev-parse --show-toplevel)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if len(fs.Args()) > 0 {
		fmt.Fprintf(stderr, "lucind-ai: unexpected argument(s): %s\n", strings.Join(fs.Args(), " "))
		fs.Usage()
		return 1
	}

	root := *rootFlag
	if root == "" {
		r, err := gitShowToplevel(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "lucind-ai: not in a git repository; specify --root\n")
			return 1
		}
		root = r
	}

	written, err := rules.InitSource(root)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}
	if written {
		fmt.Fprintf(stdout, "%s: created\n", rules.SourceName)
	} else {
		fmt.Fprintf(stdout, "%s: already exists\n", rules.SourceName)
	}
	return 0
}

func rulesGenerateDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("rules generate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai rules generate [--root <dir>] [--check]")
		fs.PrintDefaults()
	}

	rootFlag := fs.String("root", "", "repository root directory (default: git rev-parse --show-toplevel)")
	checkFlag := fs.Bool("check", false, "check if generated files are up to date without writing")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if len(fs.Args()) > 0 {
		fmt.Fprintf(stderr, "lucind-ai: unexpected argument(s): %s\n", strings.Join(fs.Args(), " "))
		fs.Usage()
		return 1
	}

	root := *rootFlag
	if root == "" {
		r, err := gitShowToplevel(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "lucind-ai: not in a git repository; specify --root\n")
			return 1
		}
		root = r
	}

	res, err := rules.Generate(root, rules.Options{Check: *checkFlag})
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	hasStaleOrMissing := false
	for _, t := range res.Targets {
		fmt.Fprintf(stdout, "%s: %s\n", t.File, t.Status)
		if t.Status == rules.StatusStale || t.Status == rules.StatusMissing {
			hasStaleOrMissing = true
		}
	}

	if *checkFlag && hasStaleOrMissing {
		return 1
	}
	return 0
}
