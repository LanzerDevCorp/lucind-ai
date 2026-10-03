package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/accept"
	"github.com/LanzerDevCorp/lucind-ai/internal/check"
)

const usage = "usage: lucind-ai check [--out <path>]\n       lucind-ai accept --lane <id>\n       lucind-ai attest run -- <command> [args...]\n       lucind-ai attest verify --command \"<exact command string>\"\n       lucind-ai hook stop --state-dir <dir> --result <path> [--max-continues <n>]\n       lucind-ai --version"

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 1
	}

	switch args[0] {
	case "--help", "-help", "-h", "help":
		fmt.Fprintln(stdout, usage)
		return 0
	case "--version", "-version", "-v", "version":
		fmt.Fprintf(stdout, "lucind-ai %s (%s, %s/%s)\n", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return 0
	case "check":
		return runCheck(ctx, args[1:], stdout, stderr)
	case "accept":
		return runAccept(ctx, args[1:], stdout, stderr)
	case "attest":
		return attestDispatch(ctx, args[1:], stdout, stderr)
	case "hook":
		return hookDispatch(ctx, args[1:], os.Stdin, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "lucind-ai: unknown subcommand %q\n%s\n", args[0], usage)
		return 1
	}
}

func runCheck(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai check [--out <path>]")
		fs.PrintDefaults()
	}

	outPath := fs.String("out", "", "path to write execution record")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if len(fs.Args()) > 0 {
		fmt.Fprintf(stderr, "lucind-ai: unexpected argument(s): %s\n", strings.Join(fs.Args(), " "))
		fs.Usage()
		return 1
	}

	root, err := gitShowToplevel(ctx)
	if err != nil {
		if wd, wdErr := os.Getwd(); wdErr == nil {
			root = wd
		} else {
			fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
			return 1
		}
	}

	start := time.Now()
	passed, checkOutput, checkErr := check.Check(ctx, root)
	duration := time.Since(start)
	if checkErr != nil {
		fmt.Fprintf(stderr, "lucind-ai: check: %v\n", checkErr)
		return 1
	}

	commitSHA := resolveCommitSHA(ctx, root)

	exitCode := 0
	if !passed {
		exitCode = 1
	}

	if *outPath != "" {
		content := formatMechanicalLog(commitSHA, exitCode, duration, checkOutput)
		if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
			fmt.Fprintf(stderr, "lucind-ai: create log directory: %v\n", err)
			return 1
		}
		if err := os.WriteFile(*outPath, []byte(content), 0o644); err != nil {
			fmt.Fprintf(stderr, "lucind-ai: write log file: %v\n", err)
			return 1
		}
	}

	if !passed {
		fmt.Fprintln(stderr, strings.TrimRight(checkOutput, "\n"))
		return 1
	}

	fmt.Fprint(stdout, checkOutput)
	if !strings.HasSuffix(checkOutput, "\n") {
		fmt.Fprintln(stdout)
	}
	fmt.Fprintf(stdout, "status:        passed\nduration:      %v\ncommit:        %s\nresolved root: %s\n", duration, commitSHA, root)

	return 0
}

func formatMechanicalLog(commitSHA string, exitCode int, duration time.Duration, output string) string {
	var sb strings.Builder
	sb.WriteString("=== lucind-ai mechanical check ===\n")
	sb.WriteString(fmt.Sprintf("Git Commit SHA: %s\n", commitSHA))
	sb.WriteString("Command: lucind-checks.sh\n")
	sb.WriteString(fmt.Sprintf("Duration: %v\n", duration))
	sb.WriteString(fmt.Sprintf("Exit Code: %d\n", exitCode))
	sb.WriteString("==================================\n")
	sb.WriteString(output)
	return sb.String()
}

func resolveCommitSHA(ctx context.Context, dir string) string {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func gitShowToplevel(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	root := strings.TrimRight(stdout.String(), "\r\n")
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("git rev-parse --show-toplevel returned a non-absolute path: %q", root)
	}
	return root, nil
}

func runAccept(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("accept", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai accept --lane <id>")
		fs.PrintDefaults()
	}

	laneID := fs.String("lane", "", "lane identifier to accept")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if len(fs.Args()) > 0 {
		fmt.Fprintf(stderr, "lucind-ai: unexpected argument(s): %s\n", strings.Join(fs.Args(), " "))
		fs.Usage()
		return 1
	}

	if strings.TrimSpace(*laneID) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --lane is required")
		fs.Usage()
		return 1
	}

	root, err := gitShowToplevel(ctx)
	if err != nil {
		if wd, wdErr := os.Getwd(); wdErr == nil {
			root = wd
		} else {
			fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
			return 1
		}
	}

	return accept.Run(ctx, root, *laneID, stdout, stderr)
}
