# Design Proposal: `executor.CursorAgent` and Per-Lane Executor Dispatch

**Author:** LanzerDev
**Date:** 2026-08-19
**Packet:** `cursor-agent-design-a`
**Status:** Proposed

---

## Findings

This section documents empirical findings from direct execution and inspection of the `cursor-agent` CLI binary (`2026.08.11-e8db854`) in this environment.

### 1. Preconditions Verification

All preconditions required by packet `cursor-agent-design-a` were verified:

1. `which cursor-agent` resolves to a valid executable, and `cursor-agent --version` runs cleanly:
```bash
$ which cursor-agent && cursor-agent --version
/home/lanzerdev/.local/bin/cursor-agent
2026.08.11-e8db854
```

2. `internal/executor` has exactly one exported `Executor` implementation (`Agy` in `internal/executor/agy.go`):
```bash
$ ls -la internal/executor/
total 52
drwxr-xr-x  2 lanzerdev lanzerdev  4096 Aug 19 14:18 .
drwxr-xr-x 14 lanzerdev lanzerdev  4096 Aug 19 14:18 ..
-rw-r--r--  1 lanzerdev lanzerdev  8212 Aug 19 14:18 agy.go
-rw-r--r--  1 lanzerdev lanzerdev 13531 Aug 19 14:18 agy_test.go
-rw-r--r--  1 lanzerdev lanzerdev  2787 Aug 19 14:18 executor.go
-rw-r--r--  1 lanzerdev lanzerdev   967 Aug 19 14:18 printtimeout_internal_test.go
-rw-r--r--  1 lanzerdev lanzerdev   749 Aug 19 14:18 status.go
-rw-r--r--  1 lanzerdev lanzerdev  1703 Aug 19 14:18 status_test.go
```

3. `cmd/lucind-ai/cli.go`'s `supportedExecutors` map names only `"agy"` (lines 44–46):
```go
var supportedExecutors = map[string]func() executor.Executor{
	"agy": func() executor.Executor { return executor.Agy{} },
}
```

---

### 2. Headless and Non-Interactive Invocation

`cursor-agent` supports headless one-shot dispatch using the `-p` / `--print` flag.

```bash
$ cursor-agent --help
Usage: agent [options] [command] [prompt...]

Start the Cursor Agent

Arguments:
  prompt                       Initial prompt for the agent

Options:
  -v, --version                Output the version number
  --api-key <key>              API key for authentication (can also use
                               CURSOR_API_KEY env var)
  -H, --header <header>        Add custom header to agent requests (format:
                               'Name: Value', can be used multiple times)
  -e, --endpoint <url>         Target API endpoint URL (can also use
                               CURSOR_API_ENDPOINT env var) (default:
                               "https://api2.cursor.sh", env:
                               CURSOR_API_ENDPOINT)
  -p, --print                  Print responses to console (for scripts or
                               non-interactive use). Has access to all tools,
                               including write and shell. (default: false)
  --output-format <format>     Output format (only works with --print): text |
                               json | stream-json (default: "text")
  --stream-partial-output      Stream partial output as individual text deltas
                               (only works with --print and stream-json format)
                               (default: false)
  --mode <mode>                Start in the given execution mode. plan:
                               read-only/planning (analyze, propose plans, no
                               edits). ask: Q&A style for explanations and
                               questions (read-only). (choices: "plan", "ask")
  --plan                       Start in plan mode (shorthand for --mode=plan).
                               (default: false)
  --resume [chatId]            Select a session to resume (default: false)
  --continue                   Continue previous session (default: false)
  --model <model>              Model to use (e.g., gpt-5, sonnet-4-thinking).
                               Parameterized models accept quoted bracket
                               overrides, e.g.
                               'claude-opus-4-8[context=1m,effort=high,fast=false]'
  --list-models                List available models and exit (default: false)
  -f, --force                  Force allow commands unless explicitly denied
                               (default: false)
  --yolo                       Alias for --force (Run Everything) (default:
                               false)
  --auto-review                Use Auto-review (Smart Auto): a server classifier
                               auto-runs safe tool calls and prompts for the
                               rest (default: false)
  --sandbox <mode>             Explicitly enable or disable sandbox mode
                               (overrides config) (choices: "enabled",
                               "disabled")
  --approve-mcps               Automatically approve all MCP servers (default:
                               false)
  --trust                      Trust the current workspace without prompting
                               (default: false)
  --workspace <path-or-name>   Workspace directory or saved workspace name to
                               use (defaults to current working directory)
  --add-dir <path>             Add an additional workspace root directory (can
                               be specified multiple times)
  --plugin-dir <path>          Load a local plugin directory (can be specified
                               multiple times)
  -w, --worktree [name]        Start in an isolated git worktree at
                               ~/.cursor/worktrees/<reponame>/<name>. If
                               omitted, a name is generated.
  --worktree-base <branch>     Branch or ref to base the new worktree on
                               (default: current HEAD)
  --skip-worktree-setup        Skip running worktree setup scripts from
                               .cursor/worktrees.json (default: false)
  -h, --help                   Display help for command
```

**Permission and Trust Requirements:**
When running in a new directory without `--trust`, `cursor-agent` fails immediately with exit code 1 and prompts for interactive trust:

```bash
$ cursor-agent -p --output-format json --model gemini-3.6-flash-minimal "Respond with the word PONG"
[exit code: 1]
⚠ Workspace Trust Required

  Cursor Agent can execute code and access files in this directory.
  Do you trust the contents of this directory?

    /home/lanzerdev/git_root/lucind-ai-worktrees/cursor-agent-design-a

  To proceed, you can either:
    • Run 'agent' interactively to decide
    • Pass --trust, --yolo, or -f if you trust this directory
```

When `--trust`, `--force` (`-f`), and `--approve-mcps` are passed, headless execution succeeds without any interactive blockage:

```bash
$ cursor-agent -p --trust --force --approve-mcps --output-format json --model gemini-3.6-flash-minimal "Respond with the word PONG"
{"type":"result","subtype":"success","is_error":false,"duration_ms":4523,"duration_api_ms":4523,"result":"PONG","session_id":"7aa5ab18-641c-41af-ba60-4aa2a72a9679","request_id":"5aebfa9e-fcf5-4721-963d-745783359b77","usage":{"inputTokens":22566,"outputTokens":2,"cacheReadTokens":0,"cacheWriteTokens":0}}
```

---

### 3. Structured / Machine-Readable Output Mode

`cursor-agent` supports `--output-format json` (only active when `--print` is specified).

- When successful, stdout contains a single structured JSON object describing the run result:
  `{"type":"result","subtype":"success","is_error":false,"duration_ms":...,"result":"...","session_id":"...","usage":{...}}`
- When an execution or argument error occurs (such as an invalid model name), `cursor-agent` writes the error message to stderr/stdout and exits with code 1:

```bash
$ cursor-agent -p --output-format json --model nonexistent-model-xyz "hello"
[exit code: 1]
Cannot use this model: nonexistent-model-xyz. Available models: auto, gpt-5.3-codex-low, ...
```

**Comparison with `agy`:**
`agy` outputs an event stream or structured JSON wrapper containing assistant responses. In `lucind-ai`, neither `agy`'s wrapper nor `cursor-agent`'s JSON wrapper is treated as the task result envelope. Instead, as designed in `docs/prd.md` section 6 step 4, the agent writes its result envelope directly to `.lucind/result.json` in its worktree, which the Go binary reads and validates against `internal/result/result.schema.json`.

---

### 4. Execution Time Bounding (Timeout Flag)

Direct empirical check against `cursor-agent --help`:
`cursor-agent` does **not** provide any timeout flag (e.g., `--timeout`, `--print-timeout`, `--max-time`).

This confirms the claim in `docs/prd.md` section 7:
> "The only one actually available: `cursor-agent` exposes no timeout flag at all, and `agy`'s `--print-timeout` defaults to 5m"

Consequently, `executor.CursorAgent` must rely exclusively on Go-side context deadlines (`exec.CommandContext(ctx, ...)`), killing the child process upon `context.DeadlineExceeded`, exactly as `executor.Agy` does.

---

### 5. Model Selection

Model selection is supported via `--model <model>`.

```bash
$ cursor-agent --list-models
claude-opus-5-thinking-medium - Claude Opus 5 1M Medium Thinking
claude-opus-4-8-high - Claude Opus 4.8 1M
gpt-5.6-sol-medium - GPT-5.6 Sol 1M
gemini-3.7-flash-high - Gemini 3.7 Flash
...
Tip: use --model <id> (or /model <id> in interactive mode) to switch. Parameterized models also accept quoted overrides, e.g. --model 'claude-opus-4-8[context=1m,effort=high,fast=false]'.
```

If `req.Model` is non-empty, `executor.CursorAgent` appends `"--model", req.Model`.

---

### 6. Working Directory and Workspace Determination

`cursor-agent` determines its operational workspace via:
1. `--workspace <path-or-name>`: "Workspace directory or saved workspace name to use (defaults to current working directory)"
2. Process working directory (`cmd.Dir`).

Empirical test showing that setting `cmd.Dir` and `--workspace` executes directly inside the target directory:

```bash
$ cursor-agent -p --trust --force --approve-mcps --output-format json --model gemini-3.6-flash-minimal --workspace /home/lanzerdev/.gemini/antigravity-cli/brain/c8804d61-41de-4e54-b204-e55fecc1d248/scratch/test-worktree "Create a file named result.json in .lucind/ with content {\"status\":\"done\"}"
[exit code: 0]
{"type":"result","subtype":"success","is_error":false,"duration_ms":12308,"duration_api_ms":12308,"result":"I have created the file `.lucind/result.json` with the content `{\"status\":\"done\"}`.","session_id":"6c8c54bc-e177-463d-821a-752c90cf7741","request_id":"5d5fbb2c-518b-4018-adf1-d5fb53389d09","usage":{"inputTokens":10100,"outputTokens":146,"cacheReadTokens":57951,"cacheWriteTokens":0}}

$ cat /home/lanzerdev/.gemini/antigravity-cli/brain/c8804d61-41de-4e54-b204-e55fecc1d248/scratch/test-worktree/.lucind/result.json
{"status":"done"}
```

---

### 7. The `.cursor/worktrees.json` Concern

`docs/prd.md` section 12 states:
> "`cursor-agent` keeps its own `.cursor/worktrees.json`, which can collide with ours."

**Empirical Analysis:**
1. In `cursor-agent --help`:
   - `-w, --worktree [name]`: "Start in an isolated git worktree at `~/.cursor/worktrees/<reponame>/<name>`. If omitted, a name is generated."
   - `--skip-worktree-setup`: "Skip running worktree setup scripts from `.cursor/worktrees.json` (default: false)"
   This mechanism is Cursor's internal worktree management feature.
2. In `lucind-ai`, git worktrees are created and managed by `internal/worktree` using `git worktree add ../<repo>-worktrees/<lane-id> -b lucind/<lane-id>`. `lucind-ai` **never** passes `-w` or `--worktree` to `cursor-agent`.
3. Inspection of `~/.cursor/` shows that `cursor-agent` isolates per-project state under `~/.cursor/projects/<sanitized-absolute-path>/`:
```bash
$ ls -d ~/.cursor/projects/home-lanzerdev-git-root-lucind-ai-worktrees-*
~/.cursor/projects/home-lanzerdev-git-root-lucind-ai-worktrees-cursor-agent-design-a
~/.cursor/projects/home-lanzerdev-git-root-lucind-ai-worktrees-cursor-agent-design-b
```
Each lane's worktree has a distinct path and therefore receives an isolated directory under `~/.cursor/projects/`. Running `cursor-agent` without `-w` does not touch or create `.cursor/worktrees.json`. There is zero collision risk between concurrent lanes.

---

### 8. Schema Enforcement at Source

`cursor-agent --help` contains no `--json-schema` or `--schema` option. `cursor-agent` cannot constrain its output at the process source.

This confirms the non-cooperative classification in `docs/prd.md` section 6 step 4:
> "Writing to a known path normalizes the asymmetry — validation happens once, in one place, identically for every lane, and adding a third executor later changes nothing. For the lane that cannot be constrained at the source, the packet injects the schema into the prompt itself... and the binary validates what lands on disk."

---

## Proposed executor.CursorAgent

`executor.CursorAgent` will be implemented in `internal/executor/cursor_agent.go`, implementing `executor.Executor`.

### Struct Definition

```go
package executor

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

// defaultCursorBinary is the name CursorAgent execs when Binary is left empty.
const defaultCursorBinary = "cursor-agent"

// CursorAgent dispatches the cursor-agent CLI headlessly.
type CursorAgent struct {
	// Binary is the executable to run. Defaults to "cursor-agent" when empty;
	// tests override it with a stub script path.
	Binary string

	// WaitDelay bounds how long Run waits for stdio pipes to drain once the
	// child process exits. Zero is replaced with defaultWaitDelay (5s).
	WaitDelay time.Duration
}
```

### Proposed `Run` Implementation

```go
// Run execs cursor-agent with req.Prompt, in req.WorktreePath, bounded by ctx.
func (c CursorAgent) Run(ctx context.Context, req Request) (Outcome, error) {
	binary := c.Binary
	if binary == "" {
		binary = defaultCursorBinary
	}

	args := []string{
		"--print", req.Prompt,
		"--output-format", "json",
		"--trust",
		"--force",
		"--approve-mcps",
	}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	if req.WorktreePath != "" {
		args = append(args, "--workspace", req.WorktreePath)
	}

	waitDelay := c.WaitDelay
	if waitDelay == 0 {
		waitDelay = defaultWaitDelay
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = req.WorktreePath
	cmd.WaitDelay = waitDelay

	var stderr, stdout bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	err := cmd.Run()

	outcome := Outcome{Stderr: stderr.String(), Stdout: stdout.String()}

	if ctx.Err() == context.DeadlineExceeded {
		outcome.TimedOut = true
		return outcome, nil
	}

	if errors.Is(err, exec.ErrWaitDelay) {
		if cmd.ProcessState != nil {
			outcome.ExitCode = cmd.ProcessState.ExitCode()
		}
		outcome.OutputTruncated = true
		return outcome, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		outcome.ExitCode = exitErr.ExitCode()
		return outcome, nil
	}
	if err != nil {
		// Binary missing, permission denied, etc.
		return Outcome{}, err
	}

	outcome.ExitCode = 0
	return outcome, nil
}
```

### Outcome Mapping Rationale
- `ExitCode`: Populated from `cmd.ProcessState.ExitCode()` or `exec.ExitError.ExitCode()`.
- `TimedOut`: Set to `true` when `ctx.Err() == context.DeadlineExceeded`.
- `Stderr` / `Stdout`: Captured via `bytes.Buffer`.
- `OutputTruncated`: Set to `true` if child background workers keep pipes open past `WaitDelay` (`exec.ErrWaitDelay`).

---

## Proposed Deps fix

### The Problem
`internal/run.Deps` currently defines:
```go
Executor executor.Executor
```
In `cmd/lucind-ai/cli.go` (lines 144, 175), the executor is instantiated as `newExecutor := supportedExecutors[ps[0].Executor]` and assigned to `deps.Executor`. If a batch contains packets for different executors (e.g., packet 0 for `"agy"` and packet 1 for `"cursor-agent"`), every lane in the batch is dispatched through `ps[0].Executor`'s instance.

### The Proposed Type Change
Change `run.Deps.Executor` from a single `executor.Executor` instance to a factory/lookup function:
```go
Executor func(name string) (executor.Executor, error)
```

**Rationale:**
1. **Consistency:** Matches the existing functional dependency injection pattern throughout `Deps` (`CreateWorktree`, `WorktreeFS`, `Now`, `CombineTree`, `RunChecks`, `PromoteTarget`, `DiscardCombined`, `RemoveLaneWorktree`).
2. **Safety:** Resolves executor instances per packet / per lane at dispatch time.
3. **Testability:** Tests can return mock executors or inspect the requested executor name without mutating shared global state.

### File:Line Inventory of Required Changes

#### 1. `internal/run/run.go`
- **Line 157**: Change `Deps` struct definition:
  ```go
  // Current (line 157):
  Executor       executor.Executor

  // Proposed:
  Executor       func(name string) (executor.Executor, error)
  ```
- **Line 218**: Update doc comment to reflect executor lookup.
- **Lines 285–290**: Resolve the executor by packet name before `Run`:
  ```go
  // Current (lines 285-290):
  outcome, err := deps.Executor.Run(ctx, executor.Request{
  	Prompt:       p.Body,
  	WorktreePath: wt.Path,
  	Model:        model,
  	SchemaPath:   schemaPath,
  })

  // Proposed:
  exec, err := deps.Executor(p.Executor)
  if err != nil {
  	cause := fmt.Errorf("run: resolve executor %q for lane %q: %w", p.Executor, p.ID, err)
  	return Report{}, recordLaneFailure(persistCtx, deps, p.ID, now, cause)
  }

  outcome, err := exec.Run(ctx, executor.Request{
  	Prompt:       p.Body,
  	WorktreePath: wt.Path,
  	Model:        model,
  	SchemaPath:   schemaPath,
  })
  ```

#### 2. `cmd/lucind-ai/cli.go`
- **Lines 44–46**: Register `"cursor-agent"` in `supportedExecutors`:
  ```go
  // Current (lines 44-46):
  var supportedExecutors = map[string]func() executor.Executor{
  	"agy": func() executor.Executor { return executor.Agy{} },
  }

  // Proposed:
  var supportedExecutors = map[string]func() executor.Executor{
  	"agy":          func() executor.Executor { return executor.Agy{} },
  	"cursor-agent": func() executor.Executor { return executor.CursorAgent{} },
  }
  ```
- **Line 140**: Update unsupported executor error message to list `cursor-agent`:
  ```go
  // Current (line 140):
  fmt.Fprintf(stderr, "lucind-ai: unsupported executor %q in packet %q (supported: agy)\n", p.Executor, packetFlags[i])

  // Proposed:
  fmt.Fprintf(stderr, "lucind-ai: unsupported executor %q in packet %q (supported: agy, cursor-agent)\n", p.Executor, packetFlags[i])
  ```
- **Lines 144–145**: Remove single-executor selection:
  ```go
  // Current (line 144):
  newExecutor := supportedExecutors[ps[0].Executor]
  // Proposed: Delete line 144
  ```
- **Line 175**: Wire `Deps.Executor` lookup function in `runDispatch`:
  ```go
  // Current (line 175):
  Executor:       newExecutor(),

  // Proposed:
  Executor: func(name string) (executor.Executor, error) {
  	factory, ok := supportedExecutors[name]
  	if !ok {
  		return nil, fmt.Errorf("unsupported executor %q", name)
  	}
  	return factory(), nil
  },
  ```

#### 3. `internal/run/run_test.go`
- **Line 84**: Update test helper `newTestDeps`:
  ```go
  // Current (line 84):
  Executor:    exec,

  // Proposed:
  Executor: func(name string) (executor.Executor, error) {
  	return exec, nil
  },
  ```

#### 4. `internal/run/batch_test.go`
- **Line 154**: Update test helper `newBatchTestDeps`:
  ```go
  // Current (line 154):
  Executor: exec,

  // Proposed:
  Executor: func(name string) (executor.Executor, error) {
  	return exec, nil
  },
  ```
- **Lines 478 and 518**: Update inline `Deps` constructions:
  ```go
  // Current (lines 478, 518):
  Executor: newBatchFakeExecutor(),

  // Proposed:
  Executor: func(name string) (executor.Executor, error) {
  	return newBatchFakeExecutor(), nil
  },
  ```

#### 5. `cmd/lucind-ai/cli_test.go`
- **Lines 107 and 186**: In `TestRunUnsupportedExecutorNamesIt` (line 107) and `TestRunMultiplePacketsSecondUnsupportedExecutorIsCaught` (line 186), the test fixtures used `executor: cursor-agent` as the example of an unsupported executor. When `cursor-agent` becomes supported, update those test fixtures to use an actual unsupported executor name (e.g. `executor: codex` or `executor: unsupported-agent`).

---

## Test strategy

This project follows the strict convention of using real subprocesses only where execution is cheap, local, and non-networked (e.g. git commands, `go test`, shell stubs), and injected fakes everywhere a real CLI call would consume subscription quota or require network.

### 1. `internal/executor/cursor_agent_test.go`
Unit tests for `CursorAgent` using lightweight shell script stubs created via `writeStub(t, script)` (mirroring `agy_test.go`):
- `TestCursorAgentRunExitZero`: Verifies successful exit (code 0) and default outcome.
- `TestCursorAgentRunNonZeroExitCode`: Verifies non-zero exit code propagation.
- `TestCursorAgentRunCapturesStderrAndStdout`: Verifies stderr and stdout string capture.
- `TestCursorAgentRunFlagsSent`: Verifies that `--print`, `--output-format json`, `--trust`, `--force`, and `--approve-mcps` are always passed.
- `TestCursorAgentRunModelFlag`: Verifies that `--model` is passed when `req.Model` is specified.
- `TestCursorAgentRunWorkspaceFlagAndDir`: Verifies that `--workspace` and `cmd.Dir` are set to `req.WorktreePath`.
- `TestCursorAgentRunContextDeadlineKillsChild`: Verifies that an expired context kills the subprocess and sets `outcome.TimedOut = true`.
- `TestCursorAgentRunWaitDelayTruncation`: Verifies that a grandchild holding open pipes triggers `exec.ErrWaitDelay` and sets `outcome.OutputTruncated = true`.

### 2. `internal/run/` Tests
- `internal/run/run_test.go`:
  - Verify that `Execute` calls `deps.Executor(p.Executor)` with the packet's executor name.
  - Verify that an error returned by `deps.Executor` transitions the lane to `lane.Failed` with a diagnostic ledger note.
- `internal/run/batch_test.go`:
  - `TestExecuteBatchHeterogeneousExecutors`: Create a batch with two packets (`packet-1` with `executor: agy` and `packet-2` with `executor: cursor-agent`). Verify that `deps.Executor` is called for both names and each lane dispatches to its respective executor without cross-lane pollution.

### 3. `cmd/lucind-ai/cli_test.go` Tests
- `TestRunSupportsAgyAndCursorAgent`: Verify that a multi-packet run with both `agy` and `cursor-agent` passes CLI argument validation.
- `TestRunUnsupportedExecutorRejection`: Verify that an unknown executor name (e.g. `codex`) is rejected with a clear error listing `(supported: agy, cursor-agent)`.

---

## Open risks and deferrals

### Open Risks
1. **Default Model Selection:**
   `internal/run/run.go` (line 64) defines `DefaultModel = "gemini-3.7-flash-high"`. While `cursor-agent` currently supports `gemini-3.7-flash-high` in its model catalog, Gemini is an Antigravity default rather than a natural Cursor default (where Sonnet or GPT models are typically expected for precision tasks). If a packet omits `model:`, it will default to `gemini-3.7-flash-high` on both executors.
   *Recommendation:* Retain `DefaultModel` for initial parity; consider adding per-executor default models in a subsequent iteration if needed.
2. **Background Process Tree Cleanup:**
   `cursor-agent` may spawn background daemons (such as language servers or node processes). Setting `cmd.WaitDelay = 5 * time.Second` ensures that `lucind-ai` does not hang if grandchild processes keep stdout/stderr open, but host resource usage should be monitored during long runs.
3. **Quota Depletion Signatures:**
   When Cursor subscription quota is exhausted, `cursor-agent` returns a non-zero exit code and logs the error to stderr/stdout. `lucind-ai` safely absorbs this by marking the lane `lane.Blocked` with the captured diagnosis, preserving the worktree for human inspection.

### Deferrals
1. **Live Stream Parsing (`--output-format stream-json`):**
   Deferred to a future release. Batch execution currently waits for process exit and reads the on-disk envelope from `.lucind/result.json`.
2. **Per-Executor Default Model Configuration:**
   Deferred until multi-model routing policy is expanded.
3. **Cursor Native Worktrees (`-w` flag):**
   Deferred permanently. `lucind-ai` owns git worktrees via `internal/worktree` to guarantee consistent layout (`<repo>-worktrees/<lane-id>`) and independent `.codegraph` indexes across all executors.
