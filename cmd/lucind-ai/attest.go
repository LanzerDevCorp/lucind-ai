package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/LanzerDevCorp/lucind-ai/internal/attest"
)

const attestUsage = "usage: lucind-ai attest run -- <command> [args...]\n       lucind-ai attest verify --command \"<exact command string>\""

func attestDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, attestUsage)
		return 1
	}

	switch args[0] {
	case "run":
		return attestRunDispatch(ctx, args[1:], stdout, stderr)
	case "verify":
		return attestVerifyDispatch(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "lucind-ai: unknown subcommand %q\n%s\n", args[0], attestUsage)
		return 1
	}
}

func attestRunDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "lucind-ai: command is required")
		fmt.Fprintln(stderr, attestUsage)
		return 1
	}

	if args[0] == "--" {
		args = args[1:]
	}

	if len(args) == 0 {
		fmt.Fprintln(stderr, "lucind-ai: command is required")
		fmt.Fprintln(stderr, attestUsage)
		return 1
	}

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: getwd: %v\n", err)
		return 1
	}

	entry, err := attest.RunAndRecord(ctx, wd, args, strings.Join(args, " "), os.Stdin, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	return entry.ExitCode
}

func attestVerifyDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, attestUsage)
	}

	command := fs.String("command", "", "exact command string")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if strings.TrimSpace(*command) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --command is required")
		fmt.Fprintln(stderr, attestUsage)
		return 1
	}

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: getwd: %v\n", err)
		return 1
	}

	toplevel, err := attest.RepoToplevel(ctx, wd)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve repository toplevel: %v\n", err)
		return 1
	}
	commonDir, err := attest.RepoCommonDir(ctx, wd)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve repository common dir: %v\n", err)
		return 1
	}
	repoID := attest.RepoID(commonDir)

	key, err := attest.LoadOrCreateKey("")
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: load attestation key: %v\n", err)
		return 1
	}

	logDir, err := attest.ResolveStateDir(repoID)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve attestation log dir: %v\n", err)
		return 1
	}

	reason, err := attest.Verify(ctx, toplevel, *command, key, logDir)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	if reason != "" {
		fmt.Fprintln(stderr, reason)
		return 1
	}

	return 0
}
