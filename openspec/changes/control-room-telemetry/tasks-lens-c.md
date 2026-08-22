# Tasks Lens C — Proof & Review Burden: Control Room Telemetry

## Assumed decomposition

The work breaks down into three functional units across nine files: (1) executor sink extensions and multi-writer stdio teeing across `Agy`, `CursorAgent`, and `Opencode`; (2) in-memory SSE telemetry `Hub`, `GET /api/telemetry/events` loopback endpoint on `serve.NewHandler`, and shell-free `ListRunEvents` on `serve.Model`; and (3) run engine integration (`<wt.Path>/.lucind/lane.log` writing, stream flushes bounded before terminal `SetStatus`, log archive in `PersistEnvelope`, and CLI dispatch wiring). Units 1 and 2 are independent foundation layers running concurrently in Wave 1, while Unit 3 integrates both and forms the critical path in Wave 2.

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 350–550 lines (250–350 production across 9 files, 200–300 test across 8 test files) |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | Single PR (Units 1–3) |
| Delivery strategy | single-pr |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Medium

Basis: Estimated additions and modifications across nine production files (`internal/executor/` ~95 lines, `internal/serve/` ~150 lines, `internal/run/` ~65 lines, `cmd/lucind-ai/` ~40 lines) and eight test suites (~250 lines), comparable to the additive changes in `openspec/changes/archive/2026-08-20-approvals-web-ui` (420 lines total diff).

## RED Tests from the Threat Matrix

| Threat row | Applicable | RED test | Asserts | Production task it precedes |
|---|---|---|---|---|
| Documentation-like paths | N/A: telemetry tees stdio; no path-classification boundary | N/A | N/A | N/A |
| Git repository selection | N/A: log path is under existing `wt.Path` | N/A | N/A | N/A |
| Commit state | N/A: no commit/index changes | N/A | N/A | N/A |
| Push state | N/A: no push | N/A | N/A | N/A |
| PR commands | N/A: no PR argv | N/A | N/A | N/A |

*Note: As recorded in `design.md:107-118`, all five boundary threat rows are non-applicable to stdio stream teeing and SSE hub routing. Existing loopback bind security control is preserved and tested via `TestNonLoopbackListenFails` (`internal/serve/server_test.go:17-40`).*

## Acceptance Evidence

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|
| 1. Optional writer sinks & stdio teeing on `Agy`, `CursorAgent`, `Opencode` (`internal/executor/`) | `go test -v -run 'Test(Run|CursorAgentRun|OpencodeRun)(ExitZero|NonZeroExitCode|GrandchildHoldingPipes)' ./internal/executor` (derived from `internal/executor/agy_test.go:28,166`, `cursor_agent_test.go:28`, `opencode_test.go:28`) | Concurrent stdio teeing to `StdoutWriter`/`StderrWriter` while preserving exit codes and `cmd.WaitDelay` timeout / `OutputTruncated` behavior | Real filesystem write permissions or multi-megabyte log buffering under OS memory pressure |
| 2. In-memory SSE telemetry `Hub` & loopback endpoint (`internal/serve/hub.go`, `handlers.go`) | `go test -v -run 'Test(Hub|NonLoopbackListenFails|SSE)' ./internal/serve` (derived from `internal/serve/server_test.go:17-40`) | `GET /api/telemetry/events` returns 200 with `text/event-stream`, broadcasts live chunks via `http.Flusher`, unsubscribes on context cancel, and rejects non-loopback addresses | Long-lived client reconnection resilience under network packet loss |
| 3. Shell-free lifecycle DTO queries on `serve.Model` (`internal/serve/model.go`) | `go test -v -run 'TestModel(ListRunEvents|SourceDoesNotShellOut)' ./internal/serve` (derived from `internal/serve/model_test.go:595-627`) | `ListRunEvents` returns ordered DTOs directly from `Ledger.Events` with zero `os/exec`, `os`, or `git` imports/calls | SQLite query performance under extreme database write concurrency |
| 4. Schema v5 CHECK constraint stream isolation (`internal/ledger/schema.go`) | `go test -v -run 'Test(LedgerSchemaRejectsStreamEventTypes|MigrateIsIdempotent)' ./internal/ledger` (derived from `internal/ledger/ledger_test.go:733`) | SQLite schema CHECK constraint rejects unadmitted streaming event types, keeping raw chunks out of `events` | SQLite physical file corruption recovery |
| 5. Worktree `.lucind/lane.log` writing & bounded stream flush (`internal/run/run.go`) | `go test -v -run 'TestExecute(WritesWorktreeLog|TruncatesDiagnosticNote|FlushesBeforeTerminalStatus)' ./internal/run` (derived from `internal/run/run_test.go:29,88`) | `Execute` writes full raw output to worktree `.lucind/lane.log`, bounds flush (<500ms), and truncates failure `lane_note` in ledger to ≤4096 bytes | Multi-lane concurrent filesystem I/O contention on networked mounts |
| 6. Batch barrier release after persisted terminal status (`internal/run/batch.go`) | `go test -v -run 'TestExecuteBatch(BarrierObservesPersistedTerminal|ConcurrentExecution)' ./internal/run` (derived from `internal/run/batch_test.go:61-120`, `internal/barrier/barrier_test.go:31-60`) | `barrier.Observe` runs only after terminal `SetStatus` commits, releasing barrier only when all lane terminal states are durable | OS thread scheduling jitter under CPU starvation |
| 7. CLI log archival in `PersistEnvelope` and `serveDispatch` wiring (`cmd/lucind-ai/cli.go`) | `go test -v -run 'Test(PersistEnvelopeArchivesLog|ServeDispatchWired)' ./cmd/lucind-ai` (derived from `cmd/lucind-ai/cli_test.go:37-60`) | `PersistEnvelope` copies `<wt.Path>/.lucind/lane.log` to `.lucind/results/<lane-id>.log` before cleanup, and `serveDispatch` constructs `serve.Hub` | Interactive terminal ANSI color formatting |

## Verification Gaps

None. All functional requirements and edge cases specified across the five spec deltas (`lane-execution`, `lane-telemetry-streaming`, `approvals-web-ui`, `parent-feature-integration`, and `shell-free-telemetry-query`) are verifiable through in-process unit tests, AST static analysis, and integration harnesses (`writeStub`, `fakeExecutor`, `batchFakeExecutor`, and loopback `httptest.Server`).

## Open Questions

- [ ] SSE payload format: raw stdout/stderr chunks vs multiplexed JSON envelope (`lane_id`, `stream`, `chunk`) as noted in `design.md:127`.
