# Proposal Lens C — Risks, Rollback & Test Impact: Control Room Telemetry

## Technical Risks & Failure Modes

| Risk / Failure Mode | Impact | Mitigation | Existing seam (file:line) |
|---|---|---|---|
| **SQLite Lock Contention**: High-frequency stream chunks written to SQLite cause `SQLITE_BUSY` (busy timeout 5000ms) and starve concurrent lease renewals. | High | Stream stdout/stderr to worktree files and in-memory SSE; restrict SQLite to coarse lifecycle events. | `internal/ledger/ledger.go:127-129,162-164`, `internal/feature/feature.go:354-385`, `internal/run/run.go:348-350` |
| **Event CHECK Constraint Violation**: Adding event types to `events` table fails the `CHECK (type IN (...))` constraint. | High | Keep stream events off `events`; if milestone events are needed, add migration v6 or use `integration_events`. | `internal/ledger/schema.go:34-43`, `internal/ledger/schema.go:171-180`, `internal/ledger/ledger.go:366-381` |
| **Grandchild Pipe Deadlock**: Subprocesses (e.g., MCP servers in `agy`) inherit pipes, causing `cmd.Wait()` to hang past deadline. | High | Enforce `cmd.WaitDelay`, close pipes on cancel, and use `ProcessState.ExitCode()` fallback on `exec.ErrWaitDelay`. | `internal/executor/agy.go:160-168,182-197`, `internal/executor/cursor_agent.go:82-90,104-118`, `internal/executor/opencode.go:121-129,143-159` |
| **Memory Bloat from Buffering**: Accumulating entire stdout/stderr streams in `bytes.Buffer` risks OOM during long parallel dispatches. | Medium | Use `io.MultiWriter` teeing to worktree files and broadcast channel, keeping capped 4096-byte diagnostic tails. | `internal/executor/agy.go:169-175`, `internal/executor/cursor_agent.go:91-97`, `internal/executor/opencode.go:130-136`, `internal/run/run.go:71-89` |
| **Unauthenticated Stream Exposure**: Exposing telemetry over non-loopback binds risks leaking execution data. | Medium | Enforce `serve.IsLoopback` on all telemetry routes, rejecting non-loopback binds. | `internal/serve/server.go:12-22,55-73`, `internal/serve/handlers.go:36-85` |
| **Barrier Teardown Desynchronization**: Blocking stream flushes during lane cleanup could delay terminal status and stall batch joins. | Low | Decouple stream cleanup from status resolution with a bounded flush (<500ms) prior to barrier observation. | `internal/run/batch.go:29-65,81-90`, `internal/barrier/barrier.go:36-59`, `internal/run/run.go:437-450` |
| **Log Loss on Worktree Cleanup**: Worktree logs are deleted when `worktree cleanup` or `RemoveLaneWorktree` runs. | Low | Archive worktree logs to `.lucind/results/<lane-id>.log` during `PersistEnvelope` before worktree removal. | `cmd/lucind-ai/cli.go:641-660`, `cmd/lucind-ai/cli.go:1460-1474` |

## Rollback & Additivity

**Rollback Plan**: Standard `git revert` of the telemetry commits.
- **Go binary**: Restores `bytes.Buffer` in `internal/executor/agy.go:169-175`, `internal/executor/cursor_agent.go:91-97`, `internal/executor/opencode.go:130-136`, and removes SSE routes in `internal/serve/handlers.go:36-85`.
- **Schema & Ledger**: Schema migration v6 in `internal/ledger/schema.go:224-306` is additive; reverting binary to v5 ignores additive tables without error. If no migration is used, schema version remains 5 (`internal/ledger/schema.go:9-10`).
- **Worktree & State**: Worktree logs (`.lucind/*.log`) and archived results (`cmd/lucind-ai/cli.go:647-660`) do not alter git history; existing worktrees remain functional upon revert.

**Additivity**: Formats, schemas, and ledgers change strictly additively:
- **Ledger Schema**: Schema version 5 (`internal/ledger/schema.go:9-10`) is backward-compatible; optional migration v6 (`internal/ledger/schema.go:224-306`) adds additive tables without mutating existing constraints in `lanes` (`internal/ledger/schema.go:18-32`), `events` (`internal/ledger/schema.go:34-43`), or `approvals` (`internal/ledger/schema.go:45-56`).
- **Result Envelope**: Schema `.lucind/result.schema.json:1-160` and envelope structs (`internal/result/result.go:10-40`) remain unchanged.
- **HTTP API**: Adds new `/api/telemetry` SSE route (`internal/serve/handlers.go:36-85`) without changing `/api/state` (`internal/serve/handlers.go:79-85`) or `/approvals/` (`internal/serve/handlers.go:87-146`).
- **Executor Interface**: `executor.Executor.Run` (`internal/executor/executor.go:65-80`) and `executor.Outcome` (`internal/executor/executor.go:42-63`) remain backward-compatible.

## Test & Validation Impact

| Test Layer | Impact / Required Coverage | Existing seam (file:line) |
|---|---|---|
| **Unit / Executor** | Verify `io.MultiWriter` teeing to disk and stream sink; verify `WaitDelay` and `OutputTruncated` on pipe retention. | `internal/executor/agy_test.go:158-191`, `internal/executor/cursor_agent_test.go:70-110`, `internal/executor/opencode_test.go:60-105` |
| **Unit / Server & Handlers** | Test SSE endpoint, `http.Flusher` streaming, client disconnect cleanup, and loopback enforcement. | `internal/serve/server_test.go:17-40`, `internal/serve/handlers.go:36-85` |
| **Unit / Ledger & Schema** | Validate migration idempotency (if v6 added) and verify no `SQLITE_BUSY` contention during concurrent lease renewals. | `internal/ledger/schema_test.go:15-80`, `internal/ledger/ledger_test.go:120-180`, `internal/feature/feature_test.go:210-250` |
| **Unit / Model** | Verify `serve.Model` queries for lane telemetry, run events, duration, exit code, and token metadata without shell execution. | `internal/serve/model_test.go:595-627` |
| **Integration / Run & Batch** | Test parallel execution with live log streaming, verifying barrier observation and diagnosis generation remain intact. | `internal/run/run_test.go:45-120`, `internal/run/batch_test.go:30-95`, `internal/barrier/barrier_test.go:20-60` |
| **CLI E2E** | Test `lucind-ai run` and `lucind-ai serve` integration, verifying log streaming, worktree archiving, and report formatting. | `cmd/lucind-ai/cli_test.go:80-160`, `cmd/lucind-ai/cli.go:512-540,647-660` |

## Out of Scope

- Modifying external agent CLI binaries (`agy`, `cursor-agent`, `opencode`) or requiring agent-side OTLP exporters (`internal/executor/agy.go:140-158`, `internal/executor/cursor_agent.go:65-80`, `internal/executor/opencode.go:100-120`).
- Introducing non-stdlib frontend dependencies, bundlers, or external WebSocket libraries into `internal/serve` (`internal/serve/server.go:1-53`, `internal/serve/handlers.go:1-85`).
- Multi-tenant authentication, tokens, or non-loopback network listening (`internal/serve/server.go:12-22,55-73`).
- Modifying core lane status definitions in `internal/lane/status.go:10-17`.
- Candidate selection, technical approach, and conceptual changes (owned by Proposal Lens A).
- Capability impact table, delta specification requirements, and user scenarios (owned by Proposal Lens B).

## Open Questions

- [ ] Should worktree-local telemetry log files be archived to `.lucind/results/<lane-id>.log` or `.lucind/logs/<run-id>/` during `PersistEnvelope` (`cmd/lucind-ai/cli.go:647-660`) before `worktree cleanup` (`cmd/lucind-ai/cli.go:1460-1474`) deletes the worktree?
- [ ] Should coarse milestone events be recorded via an additive migration v6 in `internal/ledger/schema.go:224-306`, or routed through `integration_events` (`internal/ledger/schema.go:171-180`)?
