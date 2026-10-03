package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/accept"
	"github.com/LanzerDevCorp/lucind-ai/internal/attest"
	"github.com/LanzerDevCorp/lucind-ai/internal/check"
	"github.com/LanzerDevCorp/lucind-ai/internal/dispatch"
)

const usage = "usage: lucind-ai dispatch --cwd <dir> --allow <glob>... --brief <file|-> [--model M] [--timeout D] [--detach] [--lane <id>] [--min-quota F]\n" +
	"       lucind-ai wait <lane> [--cwd <dir>] [--timeout D]\n" +
	"       lucind-ai check [--out <path>]\n" +
	"       lucind-ai accept --lane <id>\n" +
	"       lucind-ai attest run -- <command> [args...]\n" +
	"       lucind-ai attest verify --command \"<exact command string>\"\n" +
	"       lucind-ai hook pre-tool-use|stop   (agy plugin handlers; stdin JSON)\n" +
	"       lucind-ai plugin install [--dir <staging root>]   (registers via agy plugin install)\n" +
	"       lucind-ai --version"

const (
	dispatchUsage = "usage: lucind-ai dispatch --cwd <dir> --allow <glob>... --brief <file|-> [--model M] [--timeout D] [--detach] [--lane <id>] [--min-quota F]"
	waitUsage     = "usage: lucind-ai wait <lane> [--cwd <dir>] [--timeout D]"
)

var (
	dispatchRun           = dispatch.Dispatch
	waitRun               = dispatch.Wait
	stdinReader io.Reader = os.Stdin
)

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
	case "dispatch":
		return runDispatch(ctx, args[1:], stdout, stderr)
	case "wait":
		return runWait(ctx, args[1:], stdout, stderr)
	case "check":
		return runCheck(ctx, args[1:], stdout, stderr)
	case "accept":
		return runAccept(ctx, args[1:], stdout, stderr)
	case "attest":
		return attestDispatch(ctx, args[1:], stdout, stderr)
	case "hook":
		return hookDispatch(ctx, args[1:], os.Stdin, stdout, stderr)
	case "plugin":
		return pluginDispatch(ctx, args[1:], stdout, stderr)
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

type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	if s == nil || len(*s) == 0 {
		return ""
	}
	return strings.Join(*s, ", ")
}

func (s *stringSliceFlag) Set(val string) error {
	parts := strings.Split(val, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			*s = append(*s, p)
		}
	}
	return nil
}

func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty duration")
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	if secs, err := strconv.Atoi(s); err == nil {
		return time.Duration(secs) * time.Second, nil
	}
	return 0, fmt.Errorf("invalid duration %q", s)
}

func runDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	var usageBuf bytes.Buffer
	fs := flag.NewFlagSet("dispatch", flag.ContinueOnError)
	fs.SetOutput(&usageBuf)
	fs.Usage = func() {
		fmt.Fprintln(&usageBuf, dispatchUsage)
		fs.PrintDefaults()
	}

	cwd := fs.String("cwd", "", "working directory (required)")
	var allow stringSliceFlag
	fs.Var(&allow, "allow", "allowed glob pattern (repeatable or comma-separated, required)")
	briefPath := fs.String("brief", "", "path to brief file or '-' for stdin (required)")
	model := fs.String("model", "", "model override")
	timeoutStr := fs.String("timeout", "60m", "timeout duration")
	detach := fs.Bool("detach", false, "detach and return immediately without waiting")
	laneID := fs.String("lane", "", "lane identifier for continuation")
	minQuota := fs.Float64("min-quota", 0, "minimum quota required")

	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help" || args[0] == "-help") {
		fs.Usage()
		fmt.Fprint(stdout, usageBuf.String())
		return 0
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usageBuf.String())
			return 0
		}
		fmt.Fprint(stderr, usageBuf.String())
		return 1
	}

	if len(fs.Args()) > 0 {
		fmt.Fprintf(stderr, "lucind-ai: dispatch: unexpected argument(s): %s\n", strings.Join(fs.Args(), " "))
		usageBuf.Reset()
		fs.Usage()
		fmt.Fprint(stderr, usageBuf.String())
		return 1
	}

	if strings.TrimSpace(*cwd) == "" {
		fmt.Fprintln(stderr, "lucind-ai: dispatch: --cwd is required")
		usageBuf.Reset()
		fs.Usage()
		fmt.Fprint(stderr, usageBuf.String())
		return 1
	}

	if len(allow) == 0 {
		fmt.Fprintln(stderr, "lucind-ai: dispatch: --allow is required")
		usageBuf.Reset()
		fs.Usage()
		fmt.Fprint(stderr, usageBuf.String())
		return 1
	}

	if strings.TrimSpace(*briefPath) == "" {
		fmt.Fprintln(stderr, "lucind-ai: dispatch: --brief is required")
		usageBuf.Reset()
		fs.Usage()
		fmt.Fprint(stderr, usageBuf.String())
		return 1
	}

	timeout, err := parseDuration(*timeoutStr)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: dispatch: invalid --timeout: %v\n", err)
		usageBuf.Reset()
		fs.Usage()
		fmt.Fprint(stderr, usageBuf.String())
		return 1
	}

	var briefContent string
	if *briefPath == "-" {
		data, err := io.ReadAll(stdinReader)
		if err != nil {
			fmt.Fprintf(stderr, "lucind-ai: dispatch: read stdin: %v\n", err)
			return 1
		}
		briefContent = string(data)
	} else {
		data, err := os.ReadFile(*briefPath)
		if err != nil {
			fmt.Fprintf(stderr, "lucind-ai: dispatch: read brief file %s: %v\n", *briefPath, err)
			return 1
		}
		briefContent = string(data)
	}

	opts := dispatch.Options{
		Cwd:      *cwd,
		LaneID:   *laneID,
		Allow:    allow,
		Model:    *model,
		Brief:    briefContent,
		MinQuota: *minQuota,
		Detach:   *detach,
		Timeout:  timeout,
	}

	out, exitCode, err := dispatchRun(ctx, opts, nil)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: dispatch: %v\n", err)
		if exitCode == 0 {
			return 1
		}
		return exitCode
	}

	data, jsonErr := json.MarshalIndent(out, "", "  ")
	if jsonErr != nil {
		fmt.Fprintf(stderr, "lucind-ai: dispatch: marshal output: %v\n", jsonErr)
		return 1
	}
	fmt.Fprintln(stdout, string(data))
	return exitCode
}

func runWait(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	var usageBuf bytes.Buffer
	fs := flag.NewFlagSet("wait", flag.ContinueOnError)
	fs.SetOutput(&usageBuf)
	fs.Usage = func() {
		fmt.Fprintln(&usageBuf, waitUsage)
		fs.PrintDefaults()
	}

	cwd := fs.String("cwd", ".", "working directory")
	timeoutStr := fs.String("timeout", "60m", "timeout duration")

	if len(args) == 0 {
		fmt.Fprintln(stderr, "lucind-ai: wait: lane is required")
		fs.Usage()
		fmt.Fprint(stderr, usageBuf.String())
		return 1
	}

	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" || args[0] == "-help" {
		fs.Usage()
		fmt.Fprint(stdout, usageBuf.String())
		return 0
	}

	var laneID string
	if !strings.HasPrefix(args[0], "-") {
		laneID = args[0]
		if err := fs.Parse(args[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				fmt.Fprint(stdout, usageBuf.String())
				return 0
			}
			fmt.Fprint(stderr, usageBuf.String())
			return 1
		}
		if len(fs.Args()) > 0 {
			fmt.Fprintf(stderr, "lucind-ai: wait: unexpected argument(s): %s\n", strings.Join(fs.Args(), " "))
			usageBuf.Reset()
			fs.Usage()
			fmt.Fprint(stderr, usageBuf.String())
			return 1
		}
	} else {
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				fmt.Fprint(stdout, usageBuf.String())
				return 0
			}
			fmt.Fprint(stderr, usageBuf.String())
			return 1
		}
		if len(fs.Args()) == 0 {
			fmt.Fprintln(stderr, "lucind-ai: wait: lane is required")
			usageBuf.Reset()
			fs.Usage()
			fmt.Fprint(stderr, usageBuf.String())
			return 1
		}
		laneID = fs.Args()[0]
		if len(fs.Args()) > 1 {
			fmt.Fprintf(stderr, "lucind-ai: wait: unexpected argument(s): %s\n", strings.Join(fs.Args()[1:], " "))
			usageBuf.Reset()
			fs.Usage()
			fmt.Fprint(stderr, usageBuf.String())
			return 1
		}
	}

	if strings.TrimSpace(laneID) == "" {
		fmt.Fprintln(stderr, "lucind-ai: wait: lane is required")
		usageBuf.Reset()
		fs.Usage()
		fmt.Fprint(stderr, usageBuf.String())
		return 1
	}

	timeout, err := parseDuration(*timeoutStr)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: wait: invalid --timeout: %v\n", err)
		usageBuf.Reset()
		fs.Usage()
		fmt.Fprint(stderr, usageBuf.String())
		return 1
	}

	repoRoot, err := attest.RepoToplevel(ctx, *cwd)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: wait: resolve repo root: %v\n", err)
		return 1
	}

	out, exitCode, err := waitRun(ctx, repoRoot, laneID, timeout, nil)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: wait: %v\n", err)
		if exitCode == 0 {
			return 1
		}
		return exitCode
	}

	data, jsonErr := json.MarshalIndent(out, "", "  ")
	if jsonErr != nil {
		fmt.Fprintf(stderr, "lucind-ai: wait: marshal output: %v\n", jsonErr)
		return 1
	}
	fmt.Fprintln(stdout, string(data))
	return exitCode
}

