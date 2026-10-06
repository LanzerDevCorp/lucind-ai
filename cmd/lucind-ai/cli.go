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
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/accept"
	"github.com/LanzerDevCorp/lucind-ai/internal/dispatch"
	"github.com/LanzerDevCorp/lucind-ai/internal/repo"
)

const usage = "usage: lucind-ai dispatch --cwd <dir> [--allow <glob>]... --prompt <file|-> [--auto-skills] [--check <cmd>]... [--model M] [--timeout D] [--detach] [--lane <id>] [--min-quota F]\n" +
	"       lucind-ai wait <lane> [--cwd <dir>] [--timeout D]\n" +
	"       lucind-ai accept --lane <id>\n" +
	"       lucind-ai attest run -- <command> [args...]\n" +
	"       lucind-ai attest verify --command \"<exact command string>\"\n" +
	"       lucind-ai hook pre-tool-use|stop   (agy plugin handlers; stdin JSON)\n" +
	"       lucind-ai skills select --prompt <file|-> [--allow <glob>]... [--cwd <dir>] [--registry <path>] [--threshold <float>]\n" +
	"       lucind-ai plugin install [--dir <staging root>]   (registers via agy plugin install)\n" +
	"       lucind-ai install\n" +
	"       lucind-ai --version"

const (
	dispatchUsage = "usage: lucind-ai dispatch --cwd <dir> [--allow <glob>]... --prompt <file|-> [--auto-skills] [--check <cmd>]... [--model M] [--timeout D] [--detach] [--lane <id>] [--min-quota F]"
	waitUsage     = "usage: lucind-ai wait <lane> [--cwd <dir>] [--timeout D]"
)

const (
	ExitDone                  = dispatch.ExitDone
	ExitError                 = dispatch.ExitError
	ExitFailed                = dispatch.ExitFailed
	ExitTimeout               = dispatch.ExitTimeout
	ExitAutoSkillsUnavailable = dispatch.ExitAutoSkillsUnavailable
)

var (
	dispatchRun           = dispatch.Dispatch
	waitRun               = dispatch.Wait
	stdinReader io.Reader = os.Stdin
)

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, usage)
		return 1
	}

	switch args[0] {
	case "--help", "-help", "-h", "help":
		_, _ = fmt.Fprintln(stdout, usage)
		return 0
	case "--version", "-version", "-v", "version":
		_, _ = fmt.Fprintf(stdout, "lucind-ai %s (%s, %s/%s)\n", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return 0
	case "dispatch":
		return runDispatch(ctx, args[1:], stdout, stderr)
	case "wait":
		return runWait(ctx, args[1:], stdout, stderr)
	case "accept":
		return runAccept(ctx, args[1:], stdout, stderr)
	case "attest":
		return attestDispatch(ctx, args[1:], stdout, stderr)
	case "hook":
		return hookDispatch(ctx, args[1:], os.Stdin, stdout, stderr)
	case "skills":
		return skillsDispatch(ctx, args[1:], stdout, stderr)
	case "plugin":
		return pluginDispatch(ctx, args[1:], stdout, stderr)
	case "install":
		return runInstall(ctx, args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "lucind-ai: unknown subcommand %q\n%s\n", args[0], usage)
		return 1
	}
}

// cwdToplevel returns the repository top-level of the process working directory.
// A git failure keeps git's stderr in the message.
func cwdToplevel(ctx context.Context) (string, error) {
	root, err := repo.Toplevel(ctx, ".")
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("git rev-parse --show-toplevel: %w: %s", exitErr, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return root, nil
}

func runAccept(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("accept", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "usage: lucind-ai accept --lane <id>")
		fs.PrintDefaults()
	}

	laneID := fs.String("lane", "", "lane identifier to accept")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if len(fs.Args()) > 0 {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: unexpected argument(s): %s\n", strings.Join(fs.Args(), " "))
		fs.Usage()
		return 1
	}

	if strings.TrimSpace(*laneID) == "" {
		_, _ = fmt.Fprintln(stderr, "lucind-ai: --lane is required")
		fs.Usage()
		return 1
	}

	root, err := cwdToplevel(ctx)
	if err != nil {
		if wd, wdErr := os.Getwd(); wdErr == nil {
			root = wd
		} else {
			_, _ = fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
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

type checkSliceFlag []string

func (s *checkSliceFlag) String() string {
	if s == nil || len(*s) == 0 {
		return ""
	}
	return strings.Join(*s, ", ")
}

func (s *checkSliceFlag) Set(val string) error {
	if strings.TrimSpace(val) == "" {
		return errors.New("check command cannot be empty")
	}
	*s = append(*s, val)
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

func usageError(w io.Writer, usageBuf *bytes.Buffer, fs *flag.FlagSet, msg string) int {
	_, _ = fmt.Fprintln(w, msg)
	usageBuf.Reset()
	fs.Usage()
	_, _ = fmt.Fprint(w, usageBuf.String())
	return 1
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
	fs.Var(&allow, "allow", "allowed glob pattern (repeatable or comma-separated, required for new lane)")
	var checks checkSliceFlag
	fs.Var(&checks, "check", "verification command to run and attest (repeatable)")
	promptPath := fs.String("prompt", "", "path to prompt file or '-' for stdin (required)")
	model := fs.String("model", "", "model override")
	timeoutStr := fs.String("timeout", "60m", "timeout duration")
	detach := fs.Bool("detach", false, "detach and return immediately without waiting")
	laneID := fs.String("lane", "", "lane identifier for continuation")
	minQuota := fs.Float64("min-quota", 0, "minimum quota required")
	autoSkills := fs.Bool("auto-skills", false, "automatically select and inject relevant skills using Jev")

	for _, arg := range args {
		if arg == "--brief" || strings.HasPrefix(arg, "--brief=") || arg == "-brief" || strings.HasPrefix(arg, "-brief=") {
			_, _ = fmt.Fprintln(stderr, "lucind-ai: --brief was removed; use --prompt instead")
			return 1
		}
	}

	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help" || args[0] == "-help") {
		fs.Usage()
		_, _ = fmt.Fprint(stdout, usageBuf.String())
		return 0
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = fmt.Fprint(stdout, usageBuf.String())
			return 0
		}
		_, _ = fmt.Fprint(stderr, usageBuf.String())
		return 1
	}

	if len(fs.Args()) > 0 {
		return usageError(stderr, &usageBuf, fs, fmt.Sprintf("lucind-ai: dispatch: unexpected argument(s): %s", strings.Join(fs.Args(), " ")))
	}

	if strings.TrimSpace(*cwd) == "" {
		return usageError(stderr, &usageBuf, fs, "lucind-ai: dispatch: --cwd is required")
	}

	if len(allow) == 0 && strings.TrimSpace(*laneID) == "" {
		return usageError(stderr, &usageBuf, fs, "lucind-ai: dispatch: --allow is required")
	}

	if strings.TrimSpace(*promptPath) == "" {
		return usageError(stderr, &usageBuf, fs, "lucind-ai: dispatch: --prompt is required")
	}

	timeout, err := parseDuration(*timeoutStr)
	if err != nil {
		return usageError(stderr, &usageBuf, fs, fmt.Sprintf("lucind-ai: dispatch: invalid --timeout: %v", err))
	}

	var promptContent string
	if *promptPath == "-" {
		data, err := io.ReadAll(stdinReader)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "lucind-ai: dispatch: read stdin: %v\n", err)
			return 1
		}
		promptContent = string(data)
	} else {
		data, err := os.ReadFile(*promptPath)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "lucind-ai: dispatch: read prompt file %s: %v\n", *promptPath, err)
			return 1
		}
		promptContent = string(data)
	}

	opts := dispatch.Options{
		Cwd:        *cwd,
		LaneID:     *laneID,
		Allow:      allow,
		Checks:     checks,
		Model:      *model,
		Prompt:     promptContent,
		MinQuota:   *minQuota,
		Detach:     *detach,
		Timeout:    timeout,
		AutoSkills: *autoSkills,
		Stderr:     stderr,
	}

	out, exitCode, err := dispatchRun(ctx, opts, nil)
	if err != nil {
		if lines := dispatch.AutoSkillsRemediationLines(err); lines != nil {
			for _, line := range lines {
				_, _ = fmt.Fprintln(stderr, line)
			}
			return ExitAutoSkillsUnavailable
		}
		_, _ = fmt.Fprintf(stderr, "lucind-ai: dispatch: %v\n", err)
		if exitCode == 0 {
			return ExitError
		}
		return exitCode
	}

	data, jsonErr := json.MarshalIndent(out, "", "  ")
	if jsonErr != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: dispatch: marshal output: %v\n", jsonErr)
		return 1
	}
	_, _ = fmt.Fprintln(stdout, string(data))
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
		return usageError(stderr, &usageBuf, fs, "lucind-ai: wait: lane is required")
	}

	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" || args[0] == "-help" {
		fs.Usage()
		_, _ = fmt.Fprint(stdout, usageBuf.String())
		return 0
	}

	var laneID string
	var parseArgs []string
	if !strings.HasPrefix(args[0], "-") {
		laneID = args[0]
		parseArgs = args[1:]
	} else {
		parseArgs = args
	}

	if err := fs.Parse(parseArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = fmt.Fprint(stdout, usageBuf.String())
			return 0
		}
		_, _ = fmt.Fprint(stderr, usageBuf.String())
		return 1
	}

	extra := fs.Args()
	if laneID == "" {
		if len(extra) == 0 {
			return usageError(stderr, &usageBuf, fs, "lucind-ai: wait: lane is required")
		}
		laneID = extra[0]
		extra = extra[1:]
	}

	if len(extra) > 0 {
		return usageError(stderr, &usageBuf, fs, fmt.Sprintf("lucind-ai: wait: unexpected argument(s): %s", strings.Join(extra, " ")))
	}

	if strings.TrimSpace(laneID) == "" {
		return usageError(stderr, &usageBuf, fs, "lucind-ai: wait: lane is required")
	}

	timeout, err := parseDuration(*timeoutStr)
	if err != nil {
		return usageError(stderr, &usageBuf, fs, fmt.Sprintf("lucind-ai: wait: invalid --timeout: %v", err))
	}

	repoRoot, err := repo.Toplevel(ctx, *cwd)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: wait: resolve repo root: %v\n", err)
		return 1
	}

	out, exitCode, err := waitRun(ctx, repoRoot, laneID, timeout)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: wait: %v\n", err)
		if exitCode == 0 {
			return 1
		}
		return exitCode
	}

	data, jsonErr := json.MarshalIndent(out, "", "  ")
	if jsonErr != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: wait: marshal output: %v\n", jsonErr)
		return 1
	}
	_, _ = fmt.Fprintln(stdout, string(data))
	return exitCode
}
