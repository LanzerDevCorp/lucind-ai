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
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/agyhooks"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
)

type stopPayload struct {
	TerminationReason string   `json:"terminationReason"`
	FullyIdle         bool     `json:"fullyIdle"`
	Error             string   `json:"error"`
	ConversationID    string   `json:"conversationId"`
	WorkspacePaths    []string `json:"workspacePaths"`
}

// hookDispatch handles `lucind-ai hook` invocations.
// A hook must never break the agent: it always exits code 0 and prints
// exactly one JSON object on stdout.
func hookDispatch(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "lucind-ai: hook: subcommand required")
		fmt.Fprintln(stdout, "{}")
		return 0
	}

	switch args[0] {
	case "stop":
		return hookStopDispatch(ctx, args[1:], stdin, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "lucind-ai: hook: unknown subcommand %q\n", args[0])
		fmt.Fprintln(stdout, "{}")
		return 0
	}
}

func hookStopDispatch(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("stop", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var stateDir string
	var resultPath string
	var maxContinues int

	fs.StringVar(&stateDir, "state-dir", "", "path to state directory (required)")
	fs.StringVar(&resultPath, "result", "", "path to result envelope (required)")
	fs.IntVar(&maxContinues, "max-continues", 2, "maximum continue retries")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stdout, "{}")
		return 0
	}

	if stateDir == "" {
		fmt.Fprintln(stderr, "lucind-ai: hook stop: --state-dir is required")
		fmt.Fprintln(stdout, "{}")
		return 0
	}

	nowUTC := func() string {
		return time.Now().UTC().Format(time.RFC3339)
	}

	// 1. Decode the payload from stdin; undecodable => write done.json with status hook_error and print {}
	var payload stopPayload
	data, readErr := io.ReadAll(stdin)
	decodeErr := func() error {
		if readErr != nil {
			return readErr
		}
		trimmed := bytes.TrimSpace(data)
		if len(trimmed) == 0 {
			return errors.New("empty stdin")
		}
		return json.Unmarshal(trimmed, &payload)
	}()

	if decodeErr != nil {
		_ = agyhooks.WriteDone(stateDir, agyhooks.Done{
			Status: "hook_error",
			Error:  decodeErr.Error(),
			At:     nowUTC(),
		})
		fmt.Fprintln(stdout, "{}")
		return 0
	}

	// 2. If fullyIdle is false: print {} and write nothing (the agent is not finished)
	if !payload.FullyIdle {
		fmt.Fprintln(stdout, "{}")
		return 0
	}

	// 3. Read envelope with result.Read(os.DirFS("/"), strings.TrimPrefix(<absolute result path>, "/"))
	// require an absolute --result, otherwise hook_error
	if !filepath.IsAbs(resultPath) {
		_ = agyhooks.WriteDone(stateDir, agyhooks.Done{
			Status:            "hook_error",
			TerminationReason: payload.TerminationReason,
			Error:             fmt.Sprintf("result path must be absolute: %s", resultPath),
			At:                nowUTC(),
		})
		fmt.Fprintln(stdout, "{}")
		return 0
	}

	fsysPath := strings.TrimPrefix(filepath.Clean(resultPath), "/")
	_, envErr := result.Read(os.DirFS("/"), fsysPath)

	// 4. Valid envelope => atomically write <state-dir>/done.json = {"status":"valid","terminationReason":"...","at":"<RFC3339 UTC>"} and print {}
	if envErr == nil {
		_ = agyhooks.WriteDone(stateDir, agyhooks.Done{
			Status:            "valid",
			TerminationReason: payload.TerminationReason,
			At:                nowUTC(),
		})
		fmt.Fprintln(stdout, "{}")
		return 0
	}

	// 5. Missing or invalid envelope => keep a counter in <state-dir>/continues (integer text).
	// If counter < max-continues: increment it and print {"decision":"continue","reason":"<message>"}
	// Otherwise atomically write done.json with status invalid and an error field, and print {}
	continuesPath := filepath.Join(stateDir, "continues")
	count := 0
	if cdata, err := os.ReadFile(continuesPath); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(string(cdata))); err == nil {
			count = n
		}
	}

	if count < maxContinues {
		count++
		_ = agyhooks.WriteAtomic(continuesPath, []byte(strconv.Itoa(count)+"\n"))

		errStr := envErr.Error()
		if len(errStr) > 1024 {
			errStr = strings.ToValidUTF8(errStr[:1024], "")
		}
		reason := fmt.Sprintf("The result envelope at %s is missing or invalid: %s. The schema is at .lucind/result.schema.json in the worktree. Please write a valid envelope before stopping.", resultPath, errStr)
		respBytes, _ := json.Marshal(map[string]string{
			"decision": "continue",
			"reason":   reason,
		})
		fmt.Fprintln(stdout, string(respBytes))
		return 0
	}

	_ = agyhooks.WriteDone(stateDir, agyhooks.Done{
		Status:            "invalid",
		TerminationReason: payload.TerminationReason,
		Error:             envErr.Error(),
		At:                nowUTC(),
	})
	fmt.Fprintln(stdout, "{}")
	return 0
}
