package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/LanzerDevCorp/lucind-ai/internal/agyhook"
)

// hookDispatch handles `lucind-ai hook <event>` invocations from the agy
// plugin: the hook payload arrives as JSON on stdin and exactly one JSON
// object is printed on stdout. A hook never breaks the agent: it always exits
// 0, and the lane (LUCIND_LANE) decides whether it enforces anything.
func hookDispatch(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "lucind-ai: hook: subcommand required (pre-tool-use|stop)")
		fmt.Fprintln(stdout, "{}")
		return 0
	}

	var handler func(context.Context, string, []byte) []byte
	switch args[0] {
	case "pre-tool-use":
		handler = agyhook.PreToolUse
	case "stop":
		handler = agyhook.Stop
	default:
		fmt.Fprintf(stderr, "lucind-ai: hook: unknown subcommand %q\n", args[0])
		fmt.Fprintln(stdout, "{}")
		return 0
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: hook %s: read stdin: %v\n", args[0], err)
		data = nil
	}
	fmt.Fprintln(stdout, string(handler(ctx, os.Getenv("LUCIND_LANE"), data)))
	return 0
}
