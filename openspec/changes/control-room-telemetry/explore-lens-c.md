# Explore Lens C — Risks, Trade-offs & Spikes: Control Room Telemetry

## Technical Risks & Unknowns

| Risk / Unknown | Severity | Mitigation / Investigation Strategy | Existing seam (file:line) |
|---|---|---|---|
| **SQLite Write Contention & Lock Starvation**: High-frequency streaming events written directly to SQLite can trigger `SQLITE_BUSY` errors and starve concurrent feature lease renewals and lane status transitions. | High | Direct high-throughput stdout/stderr logs to worktree-local log files and in-memory SSE channels; persist only coarse lifecycle events to SQLite. | `internal/ledger/ledger.go:127-129,162-164`, `internal/feature/feature.go:354-385`, `internal/run/run.go:348-350` |
| **Strict Event Table CHECK Constraint Violations**: The `events` table uses `STRICT` typing with an exhaustive `CHECK (type IN (...))` constraint that rejects new telemetry event types without schema migration. | High | Implement additive schema migration v6 adding a dedicated telemetry table or extending `type` CHECK without breaking v5 readers. | `internal/ledger/schema.go:34-43`, `internal/ledger/ledger.go:84-91` |
| **Executor Pipe Deadlocks with Grandchild Processes**: Asynchronous streaming of stdout/stderr could hang `cmd.Wait()` indefinitely if child processes or grandchild tool servers retain open inherited file handles past context cancellation. | High | Enforce `cmd.WaitDelay` on all `exec.Cmd` instances, close stream pipes on context cancellation, and rely on `ProcessState.ExitCode()` fallback. | `internal/executor/agy.go:160-168,182-197`, `internal/executor/cursor_agent.go:82-90,104-118`, `internal/executor/opencode.go:121-129,143-159` |
| **Memory Bloat from Process Output Buffering**: Current executors accumulate entire stdout/stderr streams in memory via `bytes.Buffer`, causing high RAM usage and hiding runtime progress during long dispatches. | Medium | Replace buffered `bytes.Buffer` with an `io.MultiWriter` teeing output to worktree log files and a bounded in-memory circular buffer while retaining capped diagnostic tails. | `internal/executor/agy.go:169-175`, `internal/executor/cursor_agent.go:91-97`, `internal/executor/opencode.go:130-136`, `internal/run/run.go:71-89` |
| **Unauthenticated Network Exposure of Telemetry Streams**: Introducing HTTP/SSE streaming endpoints could expose execution logs and operational controls if loopback enforcement is bypassed. | Medium | Enforce `serve.IsLoopback` on all new telemetry routes, reject non-loopback bindings, and keep telemetry strictly localhost-only. | `internal/serve/server.go:12-22,55-73`, `internal/serve/handlers.go:36-118` |
| **Batch Barrier Desynchronization from Blocking Telemetry Flushes**: Synchronous log flushing or lingering stream sinks during lane teardown could delay lane termination and block the batch join. | Low | Decouple stream cleanup from terminal status resolution with a bounded flush timeout (<500ms) prior to barrier observation. | `internal/run/batch.go:29-65`, `internal/run/run.go:437-450` |

## Trade-offs Matrix

| Dimension / Choice | Advantages | Disadvantages | Operational Cost |
|---|---|---|---|
| **Storage: Worktree-Local Log Files + SSE Hub** (vs. SQLite Ingestion) | Zero SQLite lock contention; fast sequential disk I/O; clean worktree isolation; natural cleanup via `worktree cleanup`. | Raw logs unavailable after worktree deletion without explicit archive step; requires filesystem reads for history. | Low (zero database maintenance; relies on OS file cache). |
| **Storage: SQLite Telemetry Table** (vs. Worktree Files) | Single consolidated query interface; automatic joins with lanes, events, and approvals. | High write lock contention in WAL mode; database file bloat; requires vacuuming and pruning policies. | Medium (database vacuuming and connection pool tuning). |
| **Transport: Server-Sent Events (SSE)** (vs. WebSockets) | Native Go stdlib `net/http` support; automatic browser reconnection; no third-party dependencies. | Unidirectional transport; client backpressure cannot be signaled over the same HTTP stream. | Low (embeds cleanly in existing stdlib HTTP server). |
| **Transport: WebSockets** (vs. SSE) | Bidirectional full-duplex communication; lower framing overhead. | Requires external non-stdlib dependency, violating the stdlib-only constraint in `internal/serve`. | High (adds external package footprint and complex connection lifecycle). |
| **Ingestion: Stdio Stream Interception** (vs. OTLP Receiver) | Works out of the box with existing agent CLIs (`agy`, `cursor-agent`, `opencode`) without wrapper instrumentation. | Requires regex/JSON parsing to extract structured events from raw text streams. | Low (zero changes needed in external agent binaries). |
| **Ingestion: OTLP / OpenTelemetry Endpoint** (vs. Stdio Interception) | Industry-standard schema for spans and metrics. | External agent CLIs do not emit OTLP natively; requires heavyweight dependencies and sidecar harnesses. | High (significant integration and protocol translation overhead). |

## Potential Spikes / Proof of Concepts

- **Spike 1: Executor Stream Piping & WaitDelay Handling**: Prototype replacing `bytes.Buffer` in `internal/executor/agy.go:169-175` and `internal/executor/opencode.go:130-136` with an `io.MultiWriter` that tees stdout/stderr to a worktree file and a streaming reader. Verify in `internal/executor/agy_test.go:40-100` that `cmd.WaitDelay` gracefully terminates open pipes without hanging on child process leaks.
- **Spike 2: SQLite Write Contention Benchmark under Parallel Batch Dispatch**: Implement a synthetic benchmark in `internal/ledger/ledger_test.go:200-260` simulating 10 concurrent lanes emitting 50 telemetry events/sec while `feature.RenewLease` (`internal/feature/feature.go:354-385`) runs periodically. Measure `SQLITE_BUSY` frequency to validate isolating raw logs from `internal/ledger/ledger.go:162-164`.
- **Spike 3: Stdlib SSE Stream Multiplexing in `internal/serve`**: Build a prototype SSE endpoint in `internal/serve/handlers.go:36-118` using `http.Flusher` that broadcasts per-lane log chunks to multiple subscribers. Test client disconnects and goroutine lifecycle cleanups in `internal/serve/server_test.go:40-80`.

## Out of Scope

- Modifying external agent CLI binaries (`agy`, `cursor-agent`, `opencode`) or forcing agent-side OTLP exporters.
- Introducing non-stdlib frontend frameworks, bundlers, npm dependencies, or external WebSocket libraries into `internal/serve`.
- Remote network binding, authentication tokens, or multi-tenant authorization (retaining loopback-only binding per `internal/serve/server.go:12-22`).
- Defining problem space scope, evaluating candidate architectures, or defining UI user scenarios (owned by parallel Lenses A and B).
- Modifying the core 6-value `lane.Status` enum in `internal/lane/status.go:10-17`.

## Open Questions

- [ ] Should worktree-local telemetry log files be automatically archived to `.lucind/logs/<run_id>/` upon batch completion before `worktree cleanup` (`cmd/lucind-ai/cli.go:118`) removes the lane directory?
- [ ] Should schema migration v6 introduce a dedicated `telemetry_events` table for structured milestone markers, or extend `events.type` CHECK in `internal/ledger/schema.go:34-43`?
