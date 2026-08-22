# Design Lens A — Decisions: Control Room Capture

## Assumed architecture

This change implements Candidate 4 (hybrid file-backed stream spooling with bounded ledger notes and Model query routing) without background daemons or non-stdlib dependencies. `internal/executor.Request` (`internal/executor/executor.go:15-37`) is extended with stream destinations, executors tee stdio to primary-root disk files (`internal/executor/agy.go:169-175`, `cursor_agent.go:91-95`, `opencode.go:130-135`), and `internal/run/run.go:368-374` provisions log paths surviving worktree teardown (`internal/run/integrate.go:159-163`). `internal/serve/handlers.go:36-118` adds stdlib SSE tailing, transcript downloads, and `serve.NewModel` (`internal/serve/model.go:21-24`) JSON endpoints under schema version 5 (`internal/ledger/schema.go:9-10`).

## Technical Approach

We implement continuous primary-root stream capture across all three executors while keeping SQLite coordination bounded and loopback HTTP access decoupled. In batch dispatch (`internal/run/batch.go:66-89`), `Execute` (`internal/run/run.go:368-374`) opens durable log writers under `<primaryRoot>/.lucind/` and passes them via `executor.Request`, teeing child stdio concurrently (`Requirement: Continuous primary-root stream spooling`). Process exits retain `exec.Cmd.WaitDelay` handling (`internal/executor/agy.go:160-197`, `cursor_agent.go:82-118`, `opencode.go:121-160`), recording grandchild pipe drain timeouts as `OutputTruncated` without invalidating `lane.Done` envelopes (`Requirement: Non-interfering WaitDelay drain`). Ledger diagnostics enforce independent 4096-byte caps in SQLite `events` (`internal/run/run.go:89,125-144`; `Requirement: Bounded SQLite diagnostics`), while full transcripts are served via loopback SSE tailing and HTTP downloads (`internal/serve/handlers.go:36-118`; `Requirement: Loopback HTTP stream access`).

## Decision 1 — Primary-Root File-Backed Spooling vs SQLite Chunk Blobs

**Choice**: Spool child stdout/stderr continuously to append-only files under `<primaryRoot>/.lucind/runs/<run_id>/lanes/<lane_id>.log` via `io.MultiWriter`, avoiding SQLite stream blobs.
**Alternatives considered**: SQLite chunk log table (`lane_stream_chunks`) polled via HTTP; spooling logs to per-lane worktree directories; buffering streams in memory until exit.
**Rationale**: Multi-megabyte stream writes contend with batch status writes under SQLite's 4-connection pool and 5000ms busy timeout (`internal/ledger/ledger.go:126-128,180-184`; `internal/run/batch.go:147-152,157-214`). Storing logs in worktrees loses transcripts because `completeIntegration` removes worktrees on merge (`internal/run/integrate.go:159-163`; `cmd/lucind-ai/cli.go:641-646`). Primary-root file append is crash-resilient and gitignored (`.gitignore:2`).
**Terminal consumer**: `exec.Run` call in `internal/run/run.go:368-374` constructing `executor.Request`, and `internal/executor/agy.go:169-175` (and siblings `cursor_agent.go:91-95`, `opencode.go:130-135`) teeing stdio, satisfying `Requirement: Continuous primary-root stream spooling` (`openspec/changes/control-room-capture/proposal.md:57-75`).

## Decision 2 — Request Dest Parameterization with Diagnosis-Only Outcome

**Choice**: Extend `executor.Request` (`internal/executor/executor.go:15-37`) with stream destinations while keeping `executor.Outcome` (`internal/executor/executor.go:42-63`) strictly scoped to execution metadata (`ExitCode`, `TimedOut`, `Stderr`, `Stdout`, `OutputTruncated`).
**Alternatives considered**: Holding full megabyte transcripts in memory inside `Outcome`; removing string captures from `Outcome` completely; passing stream channels into `Executor.Run`.
**Rationale**: Bounded diagnostic strings in `Outcome` keep `decideStatus` (`internal/run/run.go:402,549-572`) and `diagnosisDetail` (`internal/run/run.go:125-144`) synchronous and disk-independent during failure analysis, while preventing unbounded memory growth during concurrent batches (`internal/run/batch.go:80-89`).
**Terminal consumer**: `executor.Request` fields in `internal/executor/executor.go:15-37` instantiated by `Execute` in `internal/run/run.go:368-374`, satisfying `Requirement: Uniform tee across executors` (`openspec/changes/control-room-capture/proposal.md:66-69`).

## Decision 3 — WaitDelay Grandchild Pipe Drain Preservation

**Choice**: Maintain `exec.Cmd.WaitDelay` (default 5s) across all executors (`internal/executor/agy.go:160-168`, `cursor_agent.go:82-90`, `opencode.go:121-129`), preserving `errors.Is(err, exec.ErrWaitDelay)` handling so grandchild processes holding pipes set `Outcome.OutputTruncated = true` and `Report.OutputCaptureIncomplete = true` without failing valid `lane.Done` dispatches.
**Alternatives considered**: Indefinite blocking (`WaitDelay = 0`); killing process groups with `Setpgid` on child exit; treating `exec.ErrWaitDelay` as fatal `lane.Failed`.
**Rationale**: Headless agent CLIs spawn MCP subprocesses that inherit stdio descriptors. Indefinite wait hangs batch runs (`internal/run/batch.go:80-89`), while treating pipe leaks as execution errors invalidates legitimate `lane.Done` results (`internal/run/run.go:402,488-499,506`; `internal/result/result.go:117-135`).
**Terminal consumer**: `exec.ErrWaitDelay` check in `internal/executor/agy.go:182-197`, `cursor_agent.go:104-118`, and `opencode.go:143-160`, consumed by `runOneLane` in `internal/run/run.go:488-499,506`, satisfying `Requirement: Non-interfering WaitDelay drain` (`openspec/changes/control-room-capture/proposal.md:76-89`).

## Decision 4 — Bounded SQLite Diagnostic Milestones

**Choice**: Enforce an independent 4096-byte per-stream cap (`streamDetailCap = 4096`, `internal/run/run.go:89,132-144`) on `EventLaneNote` events (`internal/ledger/schema.go:34-43`) for non-zero exits, timeouts, or unreadable envelopes, while recording zero stream bytes in SQLite for clean `lane.Done` completions (`internal/run/run.go:416-435`).
**Alternatives considered**: Storing unclipped stream payloads in SQLite; raising the cap to 64KB+; removing `EventLaneNote` diagnostics in favor of disk-only inspection.
**Rationale**: SQLite is the lightweight coordination ledger for status transitions, barriers, and approval gates (`internal/ledger/ledger.go:452-475`; `internal/run/run.go:425-434`; `internal/barrier/barrier.go:75-80`). Bounding notes prevents WAL bloat while maintaining instant CLI diagnostic visibility (`lucind-ai status`).
**Terminal consumer**: `formatStreamDetail` and `diagnosisDetail` in `internal/run/run.go:89,125-144` calling `deps.Ledger.AppendEvent` (`internal/run/run.go:425-434`), satisfying `Requirement: Bounded SQLite diagnostics` (`openspec/changes/control-room-capture/proposal.md:90-103`).

## Decision 5 — Loopback Server-Sent Events (SSE) and Direct File Download

**Choice**: Implement real-time log streaming via HTTP SSE (`http.Flusher`) reading directly from disk log files, and static finished-log download via `http.ServeContent` on `lucind-ai serve` (`internal/serve/server.go:19-53`; `internal/serve/handlers.go:36-118`), terminating streams on client disconnect (`r.Context().Done()`).
**Alternatives considered**: Requiring a background daemon; implementing WebSockets; polling `/api/state`.
**Rationale**: `lucind-ai` enforces a daemonless CLI architecture where `run` and `serve` operate independently (`cmd/lucind-ai/cli.go:99-127,674-725`; `docs/prd.md:188-193`). `lucind-ai serve` is loopback-only and stdlib-only (`internal/serve/server.go:19-53`; `ErrNonLoopback`). SSE requires no external dependencies and prevents slow clients from backpressuring child dispatches.
**Terminal consumer**: `NewHandler` in `internal/serve/handlers.go:36-118` executed by `serveDispatch` in `cmd/lucind-ai/cli.go:674-725`, satisfying `Requirement: Loopback HTTP stream access` (`openspec/changes/control-room-capture/proposal.md:104-122`).

## Decision 6 — Model Query Integration in HTTP Serve Handler

**Choice**: Wire `serve.NewModel` (`internal/serve/model.go:21-24`) query methods directly into `serve.NewHandler` (`internal/serve/handlers.go:36-118`) under read-only `/api/model/...` endpoints for features, attempts, leases, and audit logs.
**Alternatives considered**: Keeping `serve.NewHandler` approvals-only (`internal/serve/handlers.go:79-115`) and deferring Model routes to `control-room-serve`; creating a separate HTTP subcommand.
**Rationale**: `serve.Model` already implements complete SQL query mapping (`internal/serve/model.go:14-125,128-343`), currently used only by CLI feature status (`cmd/lucind-ai/cli.go:852`). Exposing read-only routes in `serve.NewHandler` consolidates the Control Room read surface without changing schema or ledger transactions.
**Terminal consumer**: `serve.NewModel` in `internal/serve/model.go:21-24` wired into `serve.NewHandler` in `internal/serve/handlers.go:36-118` during `serveDispatch` (`cmd/lucind-ai/cli.go:715-716`).

## Open Questions

- [ ] Directory layout naming convention: whether to standardize on `.lucind/runs/<run_id>/lanes/<lane_id>.log` vs `.lucind/logs/<run_id>/<lane_id>.log`, and whether stdout/stderr should be interleaved in a single `.log` or split into `.stdout.log` / `.stderr.log`.
- [ ] Log retention policy: whether to extend archive procedures (`plugin/claude-code/skills/lucind-ai/SKILL.md:280-285`) to archive run logs alongside `.lucind/results/`, or prune them during worktree cleanup commands (`cmd/lucind-ai/cli.go:56`).
- [ ] Precedence conflict: asymmetric precedence governs between `~/.claude/skills/sdd-design/SKILL.md` (monolithic single-agent schema) and packet `design-control-room-capture-lens-a` (parallel three-lens fan-out with a 1000-word ceiling).
