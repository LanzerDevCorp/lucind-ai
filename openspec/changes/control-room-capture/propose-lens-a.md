# Proposal Lens A — Candidate & Approach: Control Room Capture

## Selected Candidate & Approach

Selected Candidate: **Candidate 4 — Hybrid File-Backed Stream Spooling with Ledger Milestones & Model Query Routing**.

Today, `lucind-ai` executes agent CLIs (`agy`, `cursor-agent`, `opencode`) headlessly in isolated git worktrees (`internal/worktree/worktree.go:150-171`; `docs/prd.md:12-18,61-70`) using `exec.CommandContext` with in-memory `bytes.Buffer` capture (`internal/executor/agy.go:165-174`; `internal/executor/cursor_agent.go:87-95`; `internal/executor/opencode.go:126-135`). During long-running dispatches (`defaultTimeout = 20 * time.Minute` at `cmd/lucind-ai/cli.go:42`), there is zero live visibility into stdout/stderr streams. When a lane completes successfully (`lane.Done` via `internal/result/result.go:117-135`), stdout and stderr are completely discarded from memory without persistence (`internal/run/run.go:501-508`). On non-zero exit or timeout (`internal/run/run.go:549-555`; `internal/executor/status.go:14-21`), only an 8KiB capped tail (`streamDetailCap = 4096` bytes per stream, `internal/run/run.go:89,125-144`) is written as a diagnostic note to the SQLite `events` table (`internal/ledger/schema.go:34-43`). Furthermore, the loopback HTTP UI in `lucind-ai serve` (`internal/serve/server.go:19-53`; `cmd/lucind-ai/cli.go:674-725`) only exposes pending human approvals (`internal/serve/handlers.go:15-21,36-118`), leaving the rich query surface in `internal/serve/model.go:14-125,128-343` unrouted and accessible only via CLI commands (`cmd/lucind-ai/cli.go:852`).

The core approach solves this via a decoupled, stdlib-only architecture:
1. **Primary-Root File Spooling**: `executor.Request` (`internal/executor/executor.go:14-37`) accepts an output sink, and each executor uses standard `io.MultiWriter` at `cmd.Stdout` and `cmd.Stderr` (`internal/executor/agy.go:169-171`; `internal/executor/cursor_agent.go:91-93`; `internal/executor/opencode.go:130-132`) to tee stream output concurrently to dedicated log files on the **primary repository root** (`<primaryRoot>/.lucind/runs/<run_id>/...`). Writing to the primary root ensures full transcripts survive `completeIntegration`, which deletes lane worktrees (`internal/run/integrate.go:159-163`; `cmd/lucind-ai/cli.go:647-660`).
2. **Lean Ledger Milestones**: Heavy stream text is kept strictly on disk to prevent SQLite write amplification and database lock contention under `_pragma=busy_timeout(5000)` (`internal/ledger/ledger.go:126-128,162-163`) across the 4-connection pool (`internal/ledger/ledger.go:180-184`). SQLite `events` (`internal/ledger/schema.go:34-43`) records only structured lifecycle milestones (`run_started`, `lane_registered`, `lane_status_changed`, `lane_note`, `barrier_released`, `run_ended`).
3. **Loopback Streaming & Model Query Routing**: `lucind-ai serve` (`internal/serve/handlers.go:36-118`) adds endpoints for live log tailing via Server-Sent Events (`http.Flusher`) and post-mortem transcript downloads, and activates `serve.Model` (`internal/serve/model.go:14-125`) to serve feature, attempt, lease, reconciliation, and audit telemetry over JSON endpoints.
4. **Daemonless Decoupling**: `lucind-ai run` (`cmd/lucind-ai/cli.go:99-127,129-173`) and `lucind-ai serve` (`cmd/lucind-ai/cli.go:674-725`) operate independently without IPC or background daemons (`docs/prd.md:188-193`).
5. **Drain Preservation**: Preserves `exec.Cmd.WaitDelay` drain handling across all executors (`internal/executor/agy.go:15-39,182-197`; `internal/executor/cursor_agent.go:104-118`; `internal/executor/opencode.go:143-160`), keeping `Outcome.OutputTruncated` (`internal/executor/executor.go:52-63`) and `Report.OutputCaptureIncomplete` (`internal/run/run.go:231-242,506`) accurate when grandchild processes hold pipes open.

## Conceptual Changes & Architecture Rationale

- **Primary-Root Log Storage Domain**: Establishes a durable log storage directory under the primary repository root (`<primaryRoot>/.lucind/runs/<run_id>/lanes/` or `.lucind/logs/`). This decouples run observability from the ephemeral lifecycle of lane git worktrees, which are discarded upon successful batch integration (`internal/run/integrate.go:159-163`).
- **Executor Output Sink Interface**: Generalizes `executor.Request` (`internal/executor/executor.go:14-37`) to receive destination writers or log directory paths, allowing `internal/executor` implementations to stream subprocess output incrementally via `io.MultiWriter` without changing the `executor.Outcome` struct contract (`internal/executor/executor.go:42-63`).
- **Control Room Handler Expansion**: Transforms `internal/serve/handlers.go:36-118` from an approvals-only endpoint collection (`ServerState` at `internal/serve/handlers.go:15-21`) into a comprehensive Control Room routing layer by integrating `serve.NewModel` (`internal/serve/model.go:21-24`) and streaming log handlers.
- **Strict Store Separation Rationale**: Preserves SQLite's role as an atomic metadata and barrier coordination ledger (`docs/prd.md:194-205`; `internal/ledger/schema.go:10-57`), explicitly rejecting database storage for raw process streams to avoid SQLite WAL bottlenecks during concurrent batch execution (`internal/run/batch.go:66-89`).

## Alternatives Considered & Rejected

- **Pure File Spooling without Model Routing (Candidate 1)**:
  - *Approach*: Spool stdout/stderr to disk and add SSE log streaming to `internal/serve/handlers.go:36-118`, but omit `serve.Model` query routing.
  - *Rejection Rationale*: Fails to expose the status and audit query capabilities already implemented in `internal/serve/model.go:14-125,128-343`. It leaves the web server restricted solely to pending approvals (`internal/serve/handlers.go:15-21`), missing feature, attempt, lease, and reconciliation visibility in the Control Room.
- **SQLite Chunked Stream Storage with Long-Polling (Candidate 2)**:
  - *Approach*: Store stream chunks in a new `lane_stream_chunks` SQLite table (`internal/ledger/schema.go:10-57`) with long-polling endpoints.
  - *Rejection Rationale*: Multi-megabyte agent CLI output causes extreme database write amplification and lock contention with batch barrier updates (`internal/run/run.go:425-434`; `internal/run/batch.go:147-214`) under SQLite's 4-connection pool (`internal/ledger/ledger.go:180-184`) and busy timeout limits (`internal/ledger/ledger.go:126-128,162-163`).
- **Persistent Background Daemon with WebSockets (Candidate 3)**:
  - *Approach*: Require `lucind-ai serve` to run as a persistent background daemon, receiving streams over Unix domain sockets and broadcasting via WebSockets.
  - *Rejection Rationale*: Violates the standalone CLI architecture (`cmd/lucind-ai/cli.go:99-127`; `docs/prd.md:188-193`), introduces IPC daemon failure modes, and requires non-stdlib WebSocket dependencies that violate project constraints (`openspec/changes/archive/2026-08-20-apply-dag-dispatch/proposal.md`).

## Open Questions

- [ ] Directory naming and stream layout: Should primary-root log files use `.lucind/runs/<run_id>/lanes/<lane_id>.log` or `.lucind/logs/<run_id>/<lane_id>.log`, and should stdout/stderr be captured into a single interleaved log or split into `.stdout.log` and `.stderr.log`?
- [ ] Route ownership boundary: Should HTTP log streaming and model query routing registration be implemented within `control-room-capture` in `internal/serve/handlers.go:36-118` or deferred to the sibling `control-room-serve` change?
- [ ] Log archival lifecycle: Should run logs under gitignored `.lucind/` be archived into `openspec/changes/<change-id>/` during session archive (`plugin/claude-code/skills/lucind-ai/SKILL.md:280-285`) or managed via a dedicated prune/cleanup command (`cmd/lucind-ai/cli.go:56`)?
