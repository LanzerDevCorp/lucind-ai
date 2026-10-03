package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/usagelog"
)

func usageDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 1
	}

	switch args[0] {
	case "report":
		return usageReportDispatch(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "lucind-ai: unknown subcommand %q\n%s\n", args[0], usage)
		return 1
	}
}

func usageReportDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("usage report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai usage report [--since <duration or YYYY-MM-DD>] [--file <path>] [--json]")
		fs.PrintDefaults()
	}

	sinceFlag := fs.String("since", "", "filter records since duration (e.g. 24h, 7d) or date (YYYY-MM-DD)")
	fileFlag := fs.String("file", "", "path to usage log file (default: $XDG_STATE_HOME/lucind-ai/usage.jsonl)")
	jsonFlag := fs.Bool("json", false, "output report as JSON")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if len(fs.Args()) > 0 {
		fmt.Fprintf(stderr, "lucind-ai: unexpected argument(s): %s\n", strings.Join(fs.Args(), " "))
		fs.Usage()
		return 1
	}

	var since time.Time
	if *sinceFlag != "" {
		s, err := parseSince(*sinceFlag)
		if err != nil {
			fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
			return 1
		}
		since = s
	}

	filePath := *fileFlag
	if filePath == "" {
		def, err := usagelog.DefaultPath()
		if err != nil {
			fmt.Fprintf(stderr, "lucind-ai: resolve default usage log path: %v\n", err)
			return 1
		}
		filePath = def
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Fprintln(stdout, "no usage recorded")
		return 0
	}

	records, skipped, err := usagelog.ReadAll(filePath)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: read usage log: %v\n", err)
		return 1
	}

	if len(records) == 0 && skipped == 0 {
		fmt.Fprintln(stdout, "no usage recorded")
		return 0
	}

	report := usagelog.BuildReport(records, since, skipped)

	if *jsonFlag {
		data, err := report.JSON()
		if err != nil {
			fmt.Fprintf(stderr, "lucind-ai: render json report: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(data))
		return 0
	}

	fmt.Fprint(stdout, report.Text())
	return 0
}

func parseSince(sinceStr string) (time.Time, error) {
	trimmed := strings.TrimSpace(sinceStr)
	if trimmed == "" {
		return time.Time{}, nil
	}

	// 1. Try date format YYYY-MM-DD in UTC
	if t, err := time.Parse("2006-01-02", trimmed); err == nil {
		return t.UTC(), nil
	}

	// 2. Try explicit <n>d day suffix (e.g. 7d, 1d)
	if strings.HasSuffix(trimmed, "d") || strings.HasSuffix(trimmed, "D") {
		numStr := trimmed[:len(trimmed)-1]
		if n, err := strconv.Atoi(numStr); err == nil && n >= 0 {
			duration := time.Duration(n) * 24 * time.Hour
			return time.Now().UTC().Add(-duration), nil
		}
	}

	// 3. Try standard Go duration (e.g. 24h, 30m)
	if d, err := time.ParseDuration(trimmed); err == nil {
		return time.Now().UTC().Add(-d), nil
	}

	return time.Time{}, fmt.Errorf("invalid --since %q: must be a duration (e.g. 24h, 7d) or date (YYYY-MM-DD)", sinceStr)
}
