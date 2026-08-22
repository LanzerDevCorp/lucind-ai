# Spec Lens A — Capabilities & Requirements: Control Room Capture

## Assumed requirements

This change touches three capabilities: the new capability `control-room-capture` (full spec), and existing capabilities `lane-execution` and `approvals-web-ui` (delta specs). Four total requirements are introduced across these capabilities: two added to `control-room-capture` (continuous primary-root stream spooling and bounded SQLite diagnostics), one added to `lane-execution` (non-interfering WaitDelay drain), and one added to `approvals-web-ui` (loopback HTTP stream access). No existing requirements in live specs are modified, removed, or renamed.

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|
| `control-room-capture` | New | `openspec/specs/control-room-capture/spec.md` | |
| `lane-execution` | Existing | `openspec/changes/control-room-capture/specs/lane-execution/spec.md` | `openspec/specs/lane-execution/spec.md:1` |
| `approvals-web-ui` | Existing | `openspec/changes/control-room-capture/specs/approvals-web-ui/spec.md` | `openspec/specs/approvals-web-ui/spec.md:1` |

## ADDED Requirements

### Requirement: Continuous primary-root stream spooling

Executors MUST stream child stdout and stderr to durable files under `<primaryRoot>/.lucind/…` for `agy`, `cursor-agent`, and `opencode`. Spool files MUST be created at process spawn and MUST persist across `Done`, `Blocked`, `Failed`, and `Deviated` lane outcomes, remaining accessible after worktree deletion.

**Terminal consumer**: `Execute` building `executor.Request` at `internal/run/run.go:368-374` and executor stdio assignments at `internal/executor/agy.go:169-173`, `internal/executor/cursor_agent.go:91-95`, and `internal/executor/opencode.go:130-134`.

### Requirement: Non-interfering WaitDelay drain

Stream spooling MUST NOT alter `exec.Cmd.WaitDelay` timeout or process exit handling. When grandchild processes hold output pipes past `WaitDelay`, the executor MUST set `Outcome.OutputTruncated` and `Report.OutputCaptureIncomplete`, preserve the child process exit code, and MUST NOT fail an otherwise valid `Done` lane.

**Terminal consumer**: `Execute` evaluating `Outcome.OutputTruncated` and recording `Report.OutputCaptureIncomplete` at `internal/run/run.go:488-499,506`.

### Requirement: Bounded SQLite diagnostics

`events.detail` in the ledger MUST stay capped at 4096 bytes per stream for failed, timed-out, or unreadable-envelope dispatches, and the binary MUST NOT store unclipped stream blobs in SQLite. Completed lanes with `lane.Done` and an empty failure reason MUST NOT write a failure-detail `EventLaneNote`.

**Terminal consumer**: `diagnosisDetail` at `internal/run/run.go:125-144` and `Execute` calling `Ledger.AppendEvent` at `internal/run/run.go:422-435`.

### Requirement: Loopback HTTP stream access

`lucind-ai serve` MUST expose read-only loopback endpoints for live SSE log tailing and finished transcript download. Client disconnects or HTTP backpressure MUST NOT stall, leak, or backpressure running child executor processes.

**Terminal consumer**: `NewHandler` at `internal/serve/handlers.go:36-118` dispatched from `serveDispatch` in `cmd/lucind-ai/cli.go:715-719`.

## Open Questions

- [ ] Directory layout: `.lucind/runs/<run_id>/lanes/<lane_id>.log` vs `.lucind/logs/<run_id>/<lane_id>.log`? One interleaved file vs `.stdout.log` / `.stderr.log`?
- [ ] Route ownership: register log SSE, download, and Model JSON in this change (`internal/serve/handlers.go:36-118`) or in `control-room-serve`?
- [ ] Retention: copy run logs at archive (today only `.lucind/packets/` and `.lucind/results/` — `plugin/claude-code/skills/lucind-ai/SKILL.md:280-285`), prune via `lucind-ai worktree cleanup` / a dedicated command (`cmd/lucind-ai/cli.go:56`), or leave gitignored?
