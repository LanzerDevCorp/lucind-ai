package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/LanzerDevCorp/lucind-ai/internal/attest"
	"github.com/LanzerDevCorp/lucind-ai/internal/skillselect"
)

const skillsUsage = "usage: lucind-ai skills select --prompt <file|-> [--allow <glob>]... [--cwd <dir>] [--registry <path>] [--threshold <float>]"

var selectSkills = skillselect.Select

func skillsDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, skillsUsage)
		return 1
	}

	switch args[0] {
	case "--help", "-help", "-h", "help":
		_, _ = fmt.Fprintln(stdout, skillsUsage)
		return 0
	case "select":
		return runSkillsSelect(ctx, args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "lucind-ai: unknown subcommand %q\n%s\n", args[0], skillsUsage)
		return 1
	}
}

func runSkillsSelect(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("skills select", flag.ContinueOnError)
	fs.SetOutput(stderr)

	prompt := fs.String("prompt", "", "task prompt file path or '-' for stdin")
	var allow stringSliceFlag
	fs.Var(&allow, "allow", "allowed glob pattern (repeatable or comma-separated)")
	cwd := fs.String("cwd", ".", "working directory")
	registry := fs.String("registry", "", "path to skill registry markdown")
	threshold := fs.Float64("threshold", skillselect.DefaultThreshold, "selection probability threshold (0, 1]")

	for _, arg := range args {
		if arg == "--brief" || strings.HasPrefix(arg, "--brief=") || arg == "-brief" || strings.HasPrefix(arg, "-brief=") {
			_, _ = fmt.Fprintln(stderr, "lucind-ai: --brief was removed; use --prompt instead")
			return 1
		}
	}

	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help" || args[0] == "-help") {
		_, _ = fmt.Fprintln(stdout, skillsUsage)
		fs.SetOutput(stdout)
		fs.PrintDefaults()
		return 0
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	if len(fs.Args()) > 0 {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: unexpected argument(s): %s\n", strings.Join(fs.Args(), " "))
		return 1
	}

	if math.IsNaN(*threshold) || *threshold <= 0 || *threshold > 1.0 {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: threshold must be in (0, 1], got %v\n", *threshold)
		return 1
	}

	if *prompt == "" {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: --prompt is required\n")
		return 1
	}

	var promptData []byte
	var err error
	if *prompt == "-" {
		promptData, err = io.ReadAll(stdinReader)
	} else {
		promptData, err = os.ReadFile(*prompt)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	var regPath string
	if *registry == "" {
		targetCwd := *cwd
		if targetCwd == "" {
			targetCwd = "."
		}
		repoRoot, err := attest.RepoToplevel(ctx, targetCwd)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
			return 1
		}
		regPath = filepath.Join(repoRoot, ".atl", "skill-registry.md")
	} else {
		regPath = *registry
	}

	f, err := os.Open(regPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}
	defer func() { _ = f.Close() }()

	skills, err := skillselect.ParseRegistry(f)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}
	skills = skillselect.ResolvePaths(skills)

	apiKey := skillselect.ResolveKey()
	if apiKey == "" {
		_, _ = fmt.Fprintln(stderr, "lucind-ai: TYPESAFE_API_KEY is not set; export it or run lucind-ai install to store it in ~/.config/lucind/env")
		return 1
	}

	client, err := skillselect.NewClient(apiKey)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	res, err := selectSkills(ctx, client, skills, skillselect.Input{Brief: string(promptData), Allow: []string(allow)}, *threshold)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	out, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "%s\n", out)
	return 0
}
