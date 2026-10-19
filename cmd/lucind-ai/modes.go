package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/LanzerDevCorp/lucind-ai/internal/modes"
)

const (
	modeUsage  = "usage: lucind-ai mode list|show|use|clear"
	routeUsage = "usage: lucind-ai route <role> [--cwd <dir>] [--json]"
)

const minimalModesJSON = `{
  "active": "default",
  "modes": {
    "default": {
      "explorer": {"executor": "native", "model": "haiku-5.5"},
      "writer": {"executor": "lane", "model": "sonnet-5.5"},
      "verifier": {"executor": "native", "model": "haiku-5.5"}
    }
  }
}`

type routeJSON struct {
	Mode     string `json:"mode"`
	Role     string `json:"role"`
	Executor string `json:"executor"`
	Model    string `json:"model"`
}

func isValidRole(role string) bool {
	for _, r := range modes.Roles() {
		if r == role {
			return true
		}
	}
	return false
}

func handleConfigError(stderr io.Writer, cmd, cfgPath string, err error) {
	_, _ = fmt.Fprintf(stderr, "lucind-ai: %s: %v\n", cmd, err)
	if errors.Is(err, modes.ErrNoConfig) {
		_, _ = fmt.Fprintf(stderr, "create %s with a minimal configuration, for example:\n%s\n", cfgPath, minimalModesJSON)
	}
}

func loadModesConfig(stderr io.Writer, cmd string) (*modes.Config, string, bool) {
	path, err := modes.Path()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: %s: %v\n", cmd, err)
		return nil, "", false
	}
	cfg, err := modes.Load(path)
	if err != nil {
		handleConfigError(stderr, cmd, path, err)
		return nil, path, false
	}
	return cfg, path, true
}

func parseCwdFlag(cmd string, args []string, stderr io.Writer) (string, int, bool) {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	cwd := fs.String("cwd", ".", "project directory")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return "", 0, false
		}
		return "", 1, false
	}
	if len(fs.Args()) > 0 {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: %s: unexpected argument(s): %s\n", cmd, strings.Join(fs.Args(), " "))
		return "", 1, false
	}
	return *cwd, 0, true
}

func parsePositionalAndFlags(fs *flag.FlagSet, args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("missing argument")
	}
	pos := ""
	parseArgs := args
	if !strings.HasPrefix(args[0], "-") {
		pos = args[0]
		parseArgs = args[1:]
	}
	if err := fs.Parse(parseArgs); err != nil {
		return "", err
	}
	if pos == "" {
		if len(fs.Args()) == 0 {
			return "", errors.New("missing argument")
		}
		pos = fs.Args()[0]
		if len(fs.Args()) > 1 {
			return "", fmt.Errorf("unexpected argument(s): %s", strings.Join(fs.Args()[1:], " "))
		}
	} else if len(fs.Args()) > 0 {
		return "", fmt.Errorf("unexpected argument(s): %s", strings.Join(fs.Args(), " "))
	}
	return pos, nil
}

func modeDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, modeUsage)
		return 1
	}
	switch args[0] {
	case "--help", "-help", "-h", "help":
		_, _ = fmt.Fprintln(stdout, modeUsage)
		return 0
	case "list":
		return runModeList(ctx, args[1:], stdout, stderr)
	case "show":
		return runModeShow(ctx, args[1:], stdout, stderr)
	case "use":
		return runModeUse(ctx, args[1:], stdout, stderr)
	case "clear":
		return runModeClear(ctx, args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "lucind-ai: unknown subcommand %q\n%s\n", args[0], modeUsage)
		return 1
	}
}

func runModeList(_ context.Context, args []string, stdout, stderr io.Writer) int {
	cwd, code, ok := parseCwdFlag("mode list", args, stderr)
	if !ok {
		return code
	}
	cfg, _, ok := loadModesConfig(stderr, "mode list")
	if !ok {
		return 1
	}
	active, _, err := cfg.ActiveName(cwd)
	if err != nil && err.Error() != "no active mode set" {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: mode list: %v\n", err)
		return 1
	}
	for _, m := range cfg.Modes() {
		prefix := "  "
		if m == active {
			prefix = "* "
		}
		_, _ = fmt.Fprintf(stdout, "%s%s\n", prefix, m)
	}
	return 0
}

func runModeShow(_ context.Context, args []string, stdout, stderr io.Writer) int {
	cwd, code, ok := parseCwdFlag("mode show", args, stderr)
	if !ok {
		return code
	}
	cfg, _, ok := loadModesConfig(stderr, "mode show")
	if !ok {
		return 1
	}
	active, isOverride, err := cfg.ActiveName(cwd)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: mode show: %v\n", err)
		return 1
	}
	origin := "global"
	if isOverride {
		origin = "project override"
	}
	_, _ = fmt.Fprintf(stdout, "mode: %s (%s)\n", active, origin)
	roleMap, _ := cfg.Mode(active)
	for _, role := range modes.Roles() {
		if rc, exists := roleMap[role]; exists {
			_, _ = fmt.Fprintf(stdout, "%s: %s %s\n", role, rc.Executor, rc.Model)
		}
	}
	return 0
}

func runModeUse(_ context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mode use", flag.ContinueOnError)
	project := fs.Bool("project", false, "persist as project override in <cwd>/.lucind/mode")
	cwd := fs.String("cwd", ".", "project directory")

	name, err := parsePositionalAndFlags(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		_, _ = fmt.Fprintf(stderr, "lucind-ai: mode use: %v\n", err)
		return 1
	}

	cfg, cfgPath, ok := loadModesConfig(stderr, "mode use")
	if !ok {
		return 1
	}
	if _, exists := cfg.Mode(name); !exists {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: mode use: mode %q does not exist\n", name)
		return 1
	}

	if !*project {
		if err := modes.SetActive(cfgPath, name); err != nil {
			handleConfigError(stderr, "mode use", cfgPath, err)
			return 1
		}
		_, _ = fmt.Fprintf(stdout, "active mode: %s (global)\n", name)
		return 0
	}

	lucindDir := filepath.Join(*cwd, ".lucind")
	if err := os.MkdirAll(lucindDir, 0o755); err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: mode use: create %s: %v\n", lucindDir, err)
		return 1
	}
	targetPath := filepath.Join(lucindDir, "mode")
	if err := os.WriteFile(targetPath, []byte(name+"\n"), 0o644); err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: mode use: write %s: %v\n", targetPath, err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "active mode: %s (project: %s)\n", name, targetPath)
	return 0
}

func runModeClear(_ context.Context, args []string, _, stderr io.Writer) int {
	cwd, code, ok := parseCwdFlag("mode clear", args, stderr)
	if !ok {
		return code
	}
	targetPath := filepath.Join(cwd, ".lucind", "mode")
	if err := os.Remove(targetPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: mode clear: %v\n", err)
		return 1
	}
	return 0
}

func routeDispatch(_ context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("route", flag.ContinueOnError)
	cwd := fs.String("cwd", ".", "project directory")
	asJSON := fs.Bool("json", false, "print route as JSON")

	role, err := parsePositionalAndFlags(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = fmt.Fprintln(stdout, routeUsage)
			return 0
		}
		_, _ = fmt.Fprintln(stderr, routeUsage)
		return 1
	}
	if !isValidRole(role) {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: unknown role %q\n%s\n", role, routeUsage)
		return 1
	}

	cfg, _, ok := loadModesConfig(stderr, "route")
	if !ok {
		return 1
	}
	route, err := cfg.Resolve(*cwd, role)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lucind-ai: route: %v\n", err)
		return 1
	}

	if *asJSON {
		data, err := json.Marshal(routeJSON{
			Mode:     route.Mode,
			Role:     route.Role,
			Executor: route.Executor,
			Model:    route.Model,
		})
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "lucind-ai: route: marshal json: %v\n", err)
			return 1
		}
		_, _ = fmt.Fprintln(stdout, string(data))
		return 0
	}

	_, _ = fmt.Fprintf(stdout, "%s %s\n", route.Executor, route.Model)
	return 0
}
