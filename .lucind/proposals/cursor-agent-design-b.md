# Cursor Agent Integration & Multi-Executor Dispatch Design

## Findings

This section records empirical findings from executing commands against the local environment and `cursor-agent` CLI (`2026.08.11-e8db854`). Each finding is substantiated by command output executed directly within this worktree.

### 1. Reachability, Version, and Authentication

`cursor-agent` is installed in PATH at `/home/lanzerdev/.local/bin/cursor-agent` and is authenticated.

**Command:**
```bash
which cursor-agent && cursor-agent --version && cursor-agent status
```
**Output:**
```
/home/lanzerdev/.local/bin/cursor-agent
2026.08.11-e8db854
✓ Logged in as corp.systems.luisangel@gmail.com
```

### 2. Headless and Non-Interactive Invocation Shape

`cursor-agent` provides a `-p` / `--print` flag for non-interactive execution, matching `agy`'s `-p` mode. However, executing `cursor-agent -p` in a new workspace without trust or permission flags triggers an interactive prompt and exits with code 1.

**Command (Unattended execution without trust flags):**
```bash
cursor-agent -p --output-format json "echo hello"
```
**Output:**
```
⚠ Workspace Trust Required

  Cursor Agent can execute code and access files in this directory.
  Do you trust the contents of this directory?

    /home/lanzerdev/.gemini/antigravity-cli/brain/d3220b63-54c8-4346-9625-fc23cbd572b3/scratch/test-cursor

  To proceed, you can either:
    • Run 'agent' interactively to decide
    • Pass --trust, --yolo, or -f if you trust this directory
```

To run completely unattended and non-interactively without stalling on permissions or workspace prompts, `cursor-agent` requires:
- `-p` / `--print`: Non-interactive output to console.
- `--trust`: Trust the workspace directory without interactive prompting.
- `-f` / `--force` (or `--yolo`): Auto-approve tool and command executions.
- `--approve-mcps`: Auto-approve MCP servers if configured.
- `--output-format json`: Output machine-readable JSON metadata on completion.

**Command (Headless execution with trust and force flags):**
```bash
cursor-agent -p --trust --force --output-format json "reply with exactly: test output"
```
**Output:**
```
{"type":"result","subtype":"success","is_error":false,"duration_ms":4444,"duration_api_ms":4444,"result":"test output","session_id":"287cdaed-484a-4125-8c72-4e05afdc975b","request_id":"7db4acf6-890c-4706-ba4b-1b18573d91d4","usage":{"inputTokens":7107,"outputTokens":2,"cacheReadTokens":15466,"cacheWriteTokens":0}}
```

### 3. Structured / Machine-Readable Output Mode

`cursor-agent --help` documents three output formats for `--output-format`: `text`, `json`, and `stream-json`.

**Command:**
```bash
cursor-agent --help | grep -A 2 -B 1 "output-format"
```
**Output:**
```
  -p, --print                  Print responses to console (for scripts or
                               non-interactive use). Has access to all tools,
                               including write and shell. (default: false)
  --output-format <format>     Output format (only works with --print): text |
                               json | stream-json (default: "text")
  --stream-partial-output      Stream partial output as individual text deltas
```

In `--output-format json`, `cursor-agent` emits a JSON object on stdout containing execution metadata (`type`, `subtype`, `is_error`, `duration_ms`, `result`, `session_id`, `request_id`, `usage`).

### 4. Source-Level Schema Enforcement (Non-Cooperative)

`docs/prd.md` section 6 step 4 notes that `cursor-agent` lacks a source-level `--json-schema` enforcement flag like `agy`. Inspecting `cursor-agent --help` confirms there are no flags for JSON schema enforcement.

**Command:**
```bash
cursor-agent --help | grep -i "schema" || echo "No schema flags found"
```
**Output:**
```
No schema flags found
```

`cursor-agent` is confirmed non-cooperative regarding schema enforcement at source. As architected in `docs/prd.md` §6 step 4, schema enforcement for `cursor-agent` is achieved by injecting the expected schema and envelope instructions into the prompt body, writing the schema to `.lucind/result.schema.json`, and validating the resulting `.lucind/result.json` on disk via `internal/result.Read`.

### 5. Execution Time Bounding (Timeout)

`docs/prd.md` section 7 claims `cursor-agent` exposes no timeout flag. Verification against `cursor-agent --help` confirms this claim.

**Command:**
```bash
cursor-agent --help | grep -i -E "timeout|deadline|time-limit" || echo "No timeout flags found"
```
**Output:**
```
No timeout flags found
```

Unlike `agy` (which accepts `--print-timeout`), `cursor-agent` has no built-in timeout flag. Wall-clock bounding must be enforced strictly by the host process via `context.WithTimeout` and process cancellation in `os/exec.CommandContext`.

### 6. Model Selection

`cursor-agent` supports model selection via `--model <model>` and lists available models with `--list-models`.

**Command:**
```bash
cursor-agent --list-models | head -n 12
```
**Output:**
```
claude-opus-5-thinking-medium - Claude Opus 5 1M Medium Thinking
claude-opus-5-thinking-medium-fast - Claude Opus 5 1M Medium Thinking Fast
claude-opus-5-thinking-xhigh - Claude Opus 5 1M Extra High Thinking
claude-opus-5-thinking-xhigh-fast - Claude Opus 5 1M Extra High Thinking Fast
claude-opus-5-thinking-max - Claude Opus 5 1M Max Thinking
claude-opus-5-thinking-max-fast - Claude Opus 5 1M Max Thinking Fast
claude-opus-4-8-low - Claude Opus 4.8 1M Low
claude-opus-4-8-low-fast - Claude Opus 4.8 1M Low Fast
claude-opus-4-8-medium - Claude Opus 4.8 1M Medium
claude-opus-4-8-medium-fast - Claude Opus 4.8 1M Medium Fast
claude-opus-4-8-high - Claude Opus 4.8 1M
claude-opus-4-8-high-fast - Claude Opus 4.8 1M Fast
```

When `executor.Request.Model` is provided, `CursorAgent` appends `--model <model>` to the CLI arguments.

### 7. Working Directory Determination (`cwd` vs `--workspace`)

`cursor-agent` defaults to the process's current working directory (`cwd`), and also provides `--workspace <path-or-name>` and `--add-dir <path>`.

**Command (Testing process `cwd` / `cmd.Dir`):**
```bash
cd /home/lanzerdev/.gemini/antigravity-cli/brain/d3220b63-54c8-4346-9625-fc23cbd572b3/scratch/test-cursor && cursor-agent -p --trust --force "pwd"
```
**Output:**
```
The current working directory is `/home/lanzerdev/.gemini/antigravity-cli/brain/d3220b63-54c8-4346-9625-fc23cbd572b3/scratch/test-cursor`.
```

**Command (Testing explicit `--workspace` flag):**
```bash
cursor-agent -p --workspace /home/lanzerdev/.gemini/antigravity-cli/brain/d3220b63-54c8-4346-9625-fc23cbd572b3/scratch/test-cursor --trust --force "pwd"
```
**Output:**
```
The current working directory is `/home/lanzerdev/.gemini/antigravity-cli/brain/d3220b63-54c8-4346-9625-fc23cbd572b3/scratch/test-cursor`.
```

Setting `cmd.Dir = req.WorktreePath` as well as passing `--workspace req.WorktreePath` (or relying on `cmd.Dir`) guarantees that `cursor-agent` operates inside the designated lane worktree.

### 8. Worktrees and `.cursor/worktrees.json` Investigation

`docs/prd.md` section 12 lists a concern: *"cursor-agent keeps its own .cursor/worktrees.json, which can collide with ours."*

Investigation of `cursor-agent --help` reveals what this refers to:
- `cursor-agent` has an internal worktree creation feature: `-w, --worktree [name]`, which creates worktrees at `~/.cursor/worktrees/<reponame>/<name>`.
- `--skip-worktree-setup` skips running worktree setup scripts defined in `.cursor/worktrees.json`.

**Command:**
```bash
cursor-agent --help | grep -A 4 -B 1 "worktree"
```
**Output:**
```
  -w, --worktree [name]        Start in an isolated git worktree at
                               ~/.cursor/worktrees/<reponame>/<name>. If
                               omitted, a name is generated.
  --worktree-base <branch>     Branch or ref to base the new worktree on
                               (default: current HEAD)
  --skip-worktree-setup        Skip running worktree setup scripts from
                               .cursor/worktrees.json (default: false)
```

In `lucind-ai`, worktrees are created and managed directly by the host binary via `git worktree add` at `<repo-parent>/<repo-name>-worktrees/<laneID>` (`internal/worktree`). `lucind-ai` **never** passes `-w` to `cursor-agent`.

When `cursor-agent` is executed inside a linked worktree created by `lucind-ai` without `-w`, `cursor-agent` does not touch or require `.cursor/worktrees.json`, and does not interact with `~/.cursor/worktrees/`.

**Command (Verifying git status after running inside a linked worktree):**
```bash
cursor-agent -p --trust --force "echo 'verifying worktree'" && git status --porcelain
```
**Output:**
```
verifying worktree
```
(Exit code 0, and `git status --porcelain` produces no output).

There is zero collision risk with `lucind-ai`'s worktrees as long as `lucind-ai` does not pass the `-w` flag.

---

## Proposed executor.CursorAgent

### Architecture and Type Definition

`CursorAgent` implements `executor.Executor` for the `cursor-agent` CLI. It will live in `internal/executor/cursor_agent.go` alongside `agy.go`.

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
	// tests override it to point at a stub script.
	Binary string

	// WaitDelay bounds how long Run waits for the child's stdio pipes to
	// drain once the direct child process has exited. Defaults to
	// defaultWaitDelay (5s).
	WaitDelay time.Duration
}
```

### Flag Construction and Execution Logic

In `CursorAgent.Run(ctx context.Context, req Request) (Outcome, error)`:

1. **Binary resolution:** If `c.Binary == ""`, use `defaultCursorBinary` ("cursor-agent").
2. **Arguments:**
   ```go
   args := []string{
       "-p", req.Prompt,
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
   ```
   - Note on `req.SchemaPath`: `cursor-agent` has no `--json-schema` flag; `req.SchemaPath` is omitted from CLI flags.
   - Note on timeout: `cursor-agent` has no timeout flag; no `--print-timeout` is appended.
3. **Working Directory & Subprocess Execution:**
   ```go
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
   ```

### Populating `executor.Outcome`

`Outcome` fields are mapped identically to `Agy.Run`:
- `Stderr`: `stderr.String()`
- `Stdout`: `stdout.String()`
- `TimedOut`: Checked via `ctx.Err() == context.DeadlineExceeded`. Returns `Outcome{TimedOut: true, Stderr: ..., Stdout: ...}, nil`.
- `OutputTruncated`: Checked via `errors.Is(err, exec.ErrWaitDelay)`. Returns `Outcome{ExitCode: cmd.ProcessState.ExitCode(), OutputTruncated: true, Stderr: ..., Stdout: ...}, nil`.
- `ExitCode`: Populated from `exitErr.ExitCode()` when `errors.As(err, &exitErr)`. Returns `Outcome{ExitCode: code, ...}, nil`.
- Infrastructure errors (e.g. executable not found, exec failure): Returns `Outcome{}, err`.
- Clean exit 0: Returns `Outcome{ExitCode: 0, Stderr: ..., Stdout: ...}, nil`.

---

## Proposed Deps fix

### The Root Problem

In the current codebase:
1. `internal/run.Deps` ([`internal/run/run.go:157`](file:///home/lanzerdev/git_root/lucind-ai-worktrees/cursor-agent-design-b/internal/run/run.go#L157)) defines a single `Executor executor.Executor` field.
2. In `cmd/lucind-ai/cli.go` ([`cmd/lucind-ai/cli.go:144`](file:///home/lanzerdev/git_root/lucind-ai-worktrees/cursor-agent-design-b/cmd/lucind-ai/cli.go#L144)), the binary resolves `newExecutor := supportedExecutors[ps[0].Executor]` from the *first* packet in the batch, and passes this single instance to `deps.Executor` ([`cmd/lucind-ai/cli.go:175`](file:///home/lanzerdev/git_root/lucind-ai-worktrees/cursor-agent-design-b/cmd/lucind-ai/cli.go#L175)).
3. During execution ([`internal/run/run.go:285`](file:///home/lanzerdev/git_root/lucind-ai-worktrees/cursor-agent-design-b/internal/run/run.go#L285)), `Execute` dispatches every packet through `deps.Executor`, completely ignoring the packet's own `p.Executor` field for packets subsequent to the first.

### Proposed Type Change in `internal/run.Deps`

To support heterogeneous batches where each packet selects its own executor dynamically, `Deps.Executor` should be changed to a lookup function:

```go
// LookupExecutor resolves an executor by its packet identifier (e.g. "agy", "cursor-agent").
// Injected into Deps so the resolution is decoupled and testable.
LookupExecutor func(name string) (executor.Executor, error)
```

**Recommendation:** Replacing `Executor executor.Executor` with `LookupExecutor func(name string) (executor.Executor, error)` in `Deps` directly mirrors the other injected function seams (`CreateWorktree`, `WorktreeFS`, `Now`, `CombineTree`, `RunChecks`, `PromoteTarget`, `DiscardCombined`, `RemoveLaneWorktree`). It prevents silent fallback and guarantees explicit error handling if an unknown executor name reaches execution.

### Complete List of Call Sites to Modify

Every call site needing modification in the current codebase is listed below with exact file and line numbers:

1. **[`internal/run/run.go:157`](file:///home/lanzerdev/git_root/lucind-ai-worktrees/cursor-agent-design-b/internal/run/run.go#L157)**:
   - **Current:** `Executor executor.Executor`
   - **Change:** Change field definition to `LookupExecutor func(name string) (executor.Executor, error)`

2. **[`internal/run/run.go:285-290`](file:///home/lanzerdev/git_root/lucind-ai-worktrees/cursor-agent-design-b/internal/run/run.go#L285-L290)**:
   - **Current:**
     ```go
     outcome, err := deps.Executor.Run(ctx, executor.Request{
         Prompt:       p.Body,
         WorktreePath: wt.Path,
         Model:        model,
         SchemaPath:   schemaPath,
     })
     ```
   - **Change:** Resolve executor via `deps.LookupExecutor(p.Executor)` before calling `Run`:
     ```go
     exec, err := deps.LookupExecutor(p.Executor)
     if err != nil {
         cause := fmt.Errorf("run: lookup executor %q for lane %q: %w", p.Executor, p.ID, err)
         return Report{}, recordLaneFailure(persistCtx, deps, p.ID, now, cause)
     }
     outcome, err := exec.Run(ctx, executor.Request{
         Prompt:       p.Body,
         WorktreePath: wt.Path,
         Model:        model,
         SchemaPath:   schemaPath,
     })
     ```

3. **[`cmd/lucind-ai/cli.go:43`](file:///home/lanzerdev/git_root/lucind-ai-worktrees/cursor-agent-design-b/cmd/lucind-ai/cli.go#L43)**:
   - **Current:** `// fallback to agy — see internal/run's Deps.Executor field.`
   - **Change:** Update comment to reference `Deps.LookupExecutor`.

4. **[`cmd/lucind-ai/cli.go:44-46`](file:///home/lanzerdev/git_root/lucind-ai-worktrees/cursor-agent-design-b/cmd/lucind-ai/cli.go#L44-L46)**:
   - **Current:**
     ```go
     var supportedExecutors = map[string]func() executor.Executor{
         "agy": func() executor.Executor { return executor.Agy{} },
     }
     ```
   - **Change:** Register `"cursor-agent"`:
     ```go
     var supportedExecutors = map[string]func() executor.Executor{
         "agy":          func() executor.Executor { return executor.Agy{} },
         "cursor-agent": func() executor.Executor { return executor.CursorAgent{} },
     }
     ```

5. **[`cmd/lucind-ai/cli.go:138-145`](file:///home/lanzerdev/git_root/lucind-ai-worktrees/cursor-agent-design-b/cmd/lucind-ai/cli.go#L138-L145)**:
   - **Current:**
     ```go
     for i, p := range ps {
         if _, ok := supportedExecutors[p.Executor]; !ok {
             fmt.Fprintf(stderr, "lucind-ai: unsupported executor %q in packet %q (supported: agy)\n", p.Executor, packetFlags[i])
             return 1
         }
     }
     newExecutor := supportedExecutors[ps[0].Executor]
     ```
   - **Change:** Update supported executors list in error message; remove single `newExecutor` resolution:
     ```go
     for i, p := range ps {
         if _, ok := supportedExecutors[p.Executor]; !ok {
             fmt.Fprintf(stderr, "lucind-ai: unsupported executor %q in packet %q (supported: agy, cursor-agent)\n", p.Executor, packetFlags[i])
             return 1
         }
     }
     ```

6. **[`cmd/lucind-ai/cli.go:175`](file:///home/lanzerdev/git_root/lucind-ai-worktrees/cursor-agent-design-b/cmd/lucind-ai/cli.go#L175)**:
   - **Current:** `Executor: newExecutor(),`
   - **Change:** Supply `LookupExecutor` closure:
     ```go
     LookupExecutor: func(name string) (executor.Executor, error) {
         factory, ok := supportedExecutors[name]
         if !ok {
             return nil, fmt.Errorf("unsupported executor %q", name)
         }
         return factory(), nil
     },
     ```

7. **[`internal/run/run_test.go:69-85`](file:///home/lanzerdev/git_root/lucind-ai-worktrees/cursor-agent-design-b/internal/run/run_test.go#L69-L85)**:
   - **Current:** `newTestDeps` sets `Executor: exec`
   - **Change:** Change `newTestDeps` to set `LookupExecutor: func(string) (executor.Executor, error) { return exec, nil }`

8. **[`internal/run/batch_test.go:140-155`](file:///home/lanzerdev/git_root/lucind-ai-worktrees/cursor-agent-design-b/internal/run/batch_test.go#L140-L155)**:
   - **Current:** `newBatchTestDeps` sets `Executor: exec`
   - **Change:** Change `newBatchTestDeps` to set `LookupExecutor: func(string) (executor.Executor, error) { return exec, nil }`

9. **[`internal/run/batch_test.go:478`](file:///home/lanzerdev/git_root/lucind-ai-worktrees/cursor-agent-design-b/internal/run/batch_test.go#L478) and [`internal/run/batch_test.go:518`](file:///home/lanzerdev/git_root/lucind-ai-worktrees/cursor-agent-design-b/internal/run/batch_test.go#L518)**:
   - **Current:** `Executor: newBatchFakeExecutor(),`
   - **Change:** Change to `LookupExecutor: func(string) (executor.Executor, error) { return newBatchFakeExecutor(), nil },`

10. **[`cmd/lucind-ai/cli_test.go:100-123`](file:///home/lanzerdev/git_root/lucind-ai-worktrees/cursor-agent-design-b/cmd/lucind-ai/cli_test.go#L100-L123) and [`cmd/lucind-ai/cli_test.go:169-205`](file:///home/lanzerdev/git_root/lucind-ai-worktrees/cursor-agent-design-b/cmd/lucind-ai/cli_test.go#L169-L205)**:
    - **Current:** Tests use `executor: cursor-agent` to test rejection of an unsupported executor.
    - **Change:** Update fixture in `TestRunUnsupportedExecutorNamesIt` and `TestRunMultiplePacketsSecondUnsupportedExecutorIsCaught` to an actual unsupported executor name (e.g. `unsupported-executor` or `bogus-agent`), and add a test verifying `cursor-agent` is accepted as supported.

---

## Test strategy

Following the project's established convention:
- Real subprocesses are used only where cheap, fast, and local (git operations, local filesystem).
- Coding agent CLI dispatches are faked in unit/integration tests to avoid burning real subscription quota, incurring network latency, or relying on external API availability.

### 1. Testing `executor.CursorAgent` (`internal/executor/cursor_agent_test.go`)

Mirroring `internal/executor/agy_test.go`, tests will use local executable bash stubs:
- **Stub mechanism:** A temporary helper script (written dynamically in `t.TempDir()`) records arguments passed to it and outputs simulated stdout/stderr.
- **Test Scenarios:**
  - `TestCursorAgentRunSuccess`: Stub exits 0 and writes JSON output; verify `Outcome.ExitCode == 0`, `Outcome.Stdout`, `Outcome.Stderr`.
  - `TestCursorAgentRunNonZeroExit`: Stub exits 1; verify `Outcome.ExitCode == 1`.
  - `TestCursorAgentRunTimeout`: Context deadline expires; verify `Outcome.TimedOut == true` and `err == nil`.
  - `TestCursorAgentRunWaitDelay`: Stub spawns a background grandchild process holding stdout open; verify `Outcome.OutputTruncated == true` and `Outcome.ExitCode` is preserved.
  - `TestCursorAgentRunBinaryMissing`: Invocation of a nonexistent binary returns an `error` (not an `Outcome`).
  - `TestCursorAgentArgsConstruction`: Verify all flags (`-p`, `--output-format json`, `--trust`, `--force`, `--approve-mcps`, `--model`, `--workspace`) are passed correctly based on `Request` fields.

### 2. Testing `internal/run.Deps` Multi-Executor Dispatch

- **Heterogeneous Batch Test (`internal/run/batch_test.go`):**
  Add `TestExecuteBatchDispatchesDifferentExecutorsPerPacket`:
  - Define a test batch with two packets: `Packet{ID: "lane-agy", Executor: "agy"}` and `Packet{ID: "lane-cursor", Executor: "cursor-agent"}`.
  - Wire `LookupExecutor` to return two distinct recording fake executors (`fakeAgy` and `fakeCursor`).
  - Run `ExecuteBatch(ctx, deps, ps)`.
  - Assert that `fakeAgy` was invoked with `lane-agy`'s prompt/worktree, and `fakeCursor` was invoked with `lane-cursor`'s prompt/worktree.
- **Lookup Error Test (`internal/run/run_test.go`):**
  Add `TestExecuteUnsupportedExecutorFailsLane`:
  - Wire `LookupExecutor` to return `errors.New("unsupported executor")`.
  - Assert `Execute` records `lane.Failed` in the ledger and returns a non-nil error.
- **CLI Acceptance Test (`cmd/lucind-ai/cli_test.go`):**
  - Verify that a batch naming both `agy` and `cursor-agent` passes CLI validation without error.

---

## Open risks and deferrals

### Open Risks

1. **Streaming JSON Output Mode vs Full Capture:**
   `cursor-agent` supports `--output-format stream-json` in addition to `json`. For headless non-interactive execution, `--output-format json` provides complete metadata at completion and is simplest to capture. If live progress reporting is required in future versions, streaming JSON parsing will be needed.
2. **Flag Drift in Upstream `cursor-agent` Releases:**
   `cursor-agent` is evolving rapidly. The flags verified here (`--trust`, `--force`, `--approve-mcps`, `--output-format json`) are valid in `2026.08.11-e8db854`. Future upgrades should be tested against `cursor-agent --help`.
3. **Prompt Injection Schema Reliability:**
   Because `cursor-agent` lacks source-level schema enforcement (`--json-schema`), a malformed prompt or model confusion could result in `.lucind/result.json` being omitted or malformed. `lucind-ai`'s `result.Read` validation correctly classifies unreadable envelopes as `lane.Blocked` with preserved worktrees, containing this risk.

### Explicit Deferrals (Out of v1)

1. **Internal `cursor-agent -w` Worktree Management:**
   Deferred completely. `lucind-ai` continues to manage git worktrees directly via git commands. `cursor-agent` is never invoked with `-w`.
2. **Interactive / Session Resume Mode (`--resume` / `--continue`):**
   Deferred out of v1. Headless one-shot dispatch (`-p`) is sufficient for packet execution.
3. **Live Streaming Progress Output:**
   Streaming progress deltas (`stream-json`) are deferred; standard `json` output capture is used for v1.
