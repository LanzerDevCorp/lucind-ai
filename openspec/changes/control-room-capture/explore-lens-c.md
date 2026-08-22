# Explore Lens C — Risks, Trade-offs & Spikes: Control Room Capture

## Technical Risks & Unknowns

| Risk / Unknown | Severity | Mitigation / Investigation Strategy | Existing seam (file:line) |
|---|---|---|---|
| Grandchild process pipe inheritance causing stream reader deadlocks and WaitDelay truncation | High | Implement non-blocking asynchronous pipe drains with timeout channels; isolate child process groups (`Setpgid`) to propagate termination signals (`SIGTERM`/`SIGKILL`) on exit while retaining `cmd.WaitDelay` fallback. | `internal/executor/agy.go:15-39`, `internal/executor/agy.go:182-197`, `internal/executor/cursor_agent.go:104-118`, `internal/executor/opencode.go:143-160` |
| Unbounded memory consumption from in-memory stream buffering across concurrent dispatches | High | Replace unbounded in-memory `bytes.Buffer` accumulation with direct disk-backed stream spooling via `io.MultiWriter` and bounded circular ring buffers (~1MB per active lane) for in-flight UI tailing. | `internal/executor/agy.go:169-175`, `internal/executor/cursor_agent.go:91-97`, `internal/executor/opencode.go:130-136`, `internal/run/run.go:71-89`, `internal/run/batch.go:80-89` |
| Ephemeral worktree log destruction upon batch integration and worktree removal | Critical | Persist execution logs directly to a centralized run-scoped directory in the primary root (`<primaryRoot>/.lucind/logs/<run_id>/<lane_id>.log`) before worktree deletion, mirroring `PersistEnvelope` path validation. | `internal/run/integrate.go:159-165`, `internal/run/run.go:189-196`, `cmd/lucind-ai/cli.go:647-660`, `internal/ledgerpath/ledgerpath.go:23-58` |
| Slow or disconnected SSE/stream consumers exerting backpressure on running child CLIs | High | Decouple child stdio collection pumps from HTTP SSE broadcasting using buffered channels and non-blocking subscriber fanout that monitors client disconnects (`r.Context().Done()`). | `internal/run/run.go:368-375`, `internal/run/batch.go:80-89`, `internal/serve/server.go:19-53`, `internal/serve/handlers.go:36-60` |
| Standard OS pipe vs pseudo-terminal (PTY) buffering mismatch across agent CLIs | Medium | Benchmark token delivery granularity over OS pipes with JSON streaming flags (`--output-format json`, `--format json`) to verify unbuffered output without requiring platform-dependent PTY allocations. | `internal/executor/agy.go:141-155`, `internal/executor/cursor_agent.go:68-80`, `internal/executor/opencode.go:105-120` |
| ANSI escape sequences and terminal spinner codes corrupting plaintext diagnostics and ledger notes | Medium | Persist raw byte streams to disk logs for full terminal fidelity while applying an ANSI-stripping sanitizer before formatting diagnostic notes (`formatStreamDetail`) or ledger note events. | `internal/run/run.go:103-144`, `internal/run/run.go:422-435`, `internal/ledger/schema.go:34-43` |

## Trade-offs Matrix

| Dimension / Choice | Advantages | Disadvantages | Operational Cost |
|---|---|---|---|
| Storage: Disk-Backed Append Logs (`.lucind/logs/`) vs. SQLite Stream Chunks | Zero SQLite lock contention (`_busy_timeout=5000`); bounded database size; simple OS-level tailing and crash resilience. | Requires file path lifecycle management, disk cleanup, and directory creation. | Low disk I/O; avoids write amplification and database bloat. |
| Transport: Direct OS Pipes (`os.Pipe`) vs. Pseudo-Terminal Emulation (`creack/pty`) | Pure Go standard library; zero CGO or descriptor leak risks; reliable cross-platform process isolation. | Some CLIs may enable block buffering when stdout is not attached to a TTY. | Minimal maintenance overhead; zero external dependencies. |
| Log Location: Centralized Primary Root (`.lucind/logs/`) vs. Ephemeral Worktree Migration | Immune to worktree deletion upon lane integration or bisect; simple query access for `lucind-ai serve`. | Requires directory structure under `.lucind/` and path validation against primary repository root. | Low; single directory creation under `<primaryRoot>/.lucind/logs/`. |
| Delivery: Server-Sent Events (SSE) vs. Periodic Short-Polling (`/api/logs`) | Push latency <50ms; single long-lived connection per stream; stdlib `http.Flusher` implementation. | Requires subscriber channel lifecycle and disconnect management in Go handlers. | Low memory overhead for Go channel fanout vs recurring disk read churn. |
| Stream Organization: Split Files (`.stdout.log` / `.stderr.log`) vs. Multiplexed Interleaved Stream | Direct isolation of stderr diagnostics from stdout progress; trivial stream-specific tailing. | Does not preserve exact relative ordering between stdout progress and stderr lines. | Low; simple separate file handles without custom binary framing. |

## Potential Spikes / Proof of Concepts

- **Spike 1: Multi-Writer Non-Blocking Disk Streamer & Ring-Buffer Benchmark**
  - Prototype an `io.MultiWriter` tap in `internal/executor/` that streams subprocess stdout/stderr concurrently to disk (`.lucind/logs/<run_id>/<lane_id>.{stdout,stderr}.log`), a 1MB circular ring buffer for instant UI replay, and an asynchronous broadcast channel, verifying that slow consumers never block child process execution throughput across parallel dispatches (`internal/run/batch.go:80-89`).
  - *Seams:* `internal/executor/agy.go:165-175`, `internal/executor/cursor_agent.go:87-97`, `internal/executor/opencode.go:126-136`, `internal/run/batch.go:80-89`.

- **Spike 2: Grandchild Process Leaks, Process Group Isolation, and WaitDelay Drain Hardening**
  - Spawn mock child CLI processes that fork background grandchild processes (simulating MCP servers) holding stdout/stderr open. Verify that `Setpgid: true` with signal propagation and `cmd.WaitDelay` cleanly detects parent exit, prevents hangs, and sets `Outcome.OutputTruncated` / `Report.OutputCaptureIncomplete` without data loss.
  - *Seams:* `internal/executor/agy.go:15-39`, `internal/executor/agy.go:182-197`, `internal/executor/cursor_agent.go:104-118`, `internal/executor/opencode.go:143-160`, `internal/run/run.go:220-242`.

- **Spike 3: CLI Output Buffering Granularity (TTY vs Pipe)**
  - Measure token and chunk delivery intervals across `agy` (`--output-format json`), `cursor-agent` (`--output-format json`), and `opencode` (`--format json`) over standard pipes to ensure JSON streaming events are emitted incrementally without requiring a full PTY.
  - *Seams:* `internal/executor/agy.go:141-155`, `internal/executor/cursor_agent.go:68-80`, `internal/executor/opencode.go:105-120`.

- **Spike 4: Primary Root Log Lifecycle & Worktree Teardown Safety**
  - Verify that writing logs directly to `<primaryRoot>/.lucind/logs/<run_id>/` persists across `completeIntegration` worktree removal (`internal/run/integrate.go:159-165`), conforms to `ledgerpath.Validate` rules (`internal/ledgerpath/ledgerpath.go:23-58`), and provides shell-free reader access via `serve.Model` (`internal/serve/model.go:14-25`).
  - *Seams:* `internal/run/integrate.go:159-165`, `cmd/lucind-ai/cli.go:647-660`, `internal/ledgerpath/ledgerpath.go:23-58`, `internal/serve/model.go:14-25`.

## Out of Scope

- Web UI frontend rendering, terminal components, xterm.js DOM views, and CSS layout panes (owned by `control-room-ui-shell` and `control-room-ui-views`).
- HTTP server multiplexing, routing registration, and loopback listener lifecycle (owned by `control-room-serve`).
- SQLite schema migrations, ledger table definitions, and transactional event stores (owned by `control-room-ledger`).
- Real-time telemetry calculations, token counters, and status timeline aggregations (owned by `control-room-telemetry`).
- Authoring user scenarios, personas, problem space definitions, or candidate architecture designs (owned by Lens A and Lens B).
- Modifying `gentle-ai` delivery gates, receipt contracts, or external CLI admission invariants.

## Open Questions

- [ ] Should execution stdout and stderr be logged to separate files (`.stdout.log` and `.stderr.log`) or merged into a single line-tagged stream with stream identifiers?
- [ ] What retention/pruning policy should govern `.lucind/logs/` during `lucind-ai archive` or after old runs age out?
- [ ] Contract Precedence Note: `~/.claude/skills/sdd-explore/SKILL.md` specifies writing a single monolithic `exploration.md` and phase summary, which is intentionally superseded by this packet's parallel partitioned lens contract (`openspec/changes/control-room-capture/explore-lens-c.md`).
