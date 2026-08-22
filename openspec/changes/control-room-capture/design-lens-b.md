# Design Lens B — Surface & Flow: Control Room Capture

## Assumed architecture

This design assumes hybrid file-backed stream spooling with lean SQLite ledger notes and loopback HTTP Model/log query routing (Candidate 4). `executor.Request` gains destination writers teeing child stdio via `io.MultiWriter`, keeping `executor.Outcome` diagnosis-only. `internal/run` manages per-lane log files under `<primaryRoot>/.lucind/runs/<run_id>/lanes/<lane_id>.log` to survive worktree destruction, while SQLite `events.detail` remains capped at 4096 bytes per stream. `internal/serve` adds loopback SSE live-tailing (`http.Flusher`), transcript downloads, and `serve.Model` query endpoints without running a background daemon.

## Flow and Invariants

```
CLI (run) ──→ run.Execute ──→ executor.Run (io.MultiWriter) ──→ .lucind/runs/.../<lane_id>.log
                   │                        │                                 │
                   ▼                        ▼                                 ▼
           SQLite Ledger Event    Outcome (diagnosis-only)            HTTP serve (SSE/Download)
```

- **Hop 1: Run Dispatch → File Spool Creation**:
  - *Invariant*: The log file MUST be opened under the primary repository root (`<primaryRoot>/.lucind/runs/<run_id>/lanes/<lane_id>.log`) before subprocess spawn (`internal/run/run.go:368-375`; `internal/ledgerpath/ledgerpath.go:44-59`).
  - *Failure mode*: If created inside `wt.Path`, `completeIntegration` deletes the log upon lane integration (`internal/run/integrate.go:159-165`).
- **Hop 2: Subprocess Execution → Stream Teeing**:
  - *Invariant*: Child stdout/stderr MUST tee concurrently to internal buffers and destination log writers via `io.MultiWriter` (`internal/executor/agy.go:169-173`; `internal/executor/cursor_agent.go:91-95`; `internal/executor/opencode.go:130-134`).
  - *Failure mode*: If teeing is omitted or blocking, stdout/stderr is lost on exit 0 or unbuffered pipe writes stall execution.
- **Hop 3: Process Exit & Pipe Drain → WaitDelay Isolation**:
  - *Invariant*: Pipe drainage timeout returning `exec.ErrWaitDelay` MUST set `Outcome.OutputTruncated = true` and preserve real exit code from `ProcessState` (`internal/executor/agy.go:182-197`; `internal/executor/cursor_agent.go:104-118`; `internal/executor/opencode.go:143-160`).
  - *Failure mode*: Treating `ErrWaitDelay` as fatal causes valid `Done` runs with surviving background processes to fail.
- **Hop 4: Lane Terminalization → Bounded Ledger Diagnostics**:
  - *Invariant*: SQLite `events.detail` MUST be capped at `streamDetailCap` (4096 bytes/stream) on non-zero exit/timeout, and clean `lane.Done` runs MUST NOT write failure notes (`internal/run/run.go:89,125-144,416-435`).
  - *Failure mode*: Writing full streams into SQLite causes database write amplification and lock contention across batch lanes (`internal/ledger/schema.go:34-43`; `internal/run/batch.go:147-214`).
- **Hop 5: HTTP Serve → SSE Tail & Post-Mortem Download**:
  - *Invariant*: SSE log tailing MUST use `http.Flusher` bounded to loopback (`IsLoopback`), closing on `r.Context().Done()` without backpressuring the child (`internal/serve/handlers.go:36-118`; `internal/serve/server.go:19-53`).
  - *Failure mode*: Slow HTTP consumers or network drops stall child dispatches or leak streaming goroutines.

## Surface Deltas

| Symbol or format | Today (file:line) | Delta | Backward compatible? |
|---|---|---|---|
| `executor.Request` | `internal/executor/executor.go:15-37` | Add `StdoutDest io.Writer` and `StderrDest io.Writer` fields for stream spooling. | Yes; struct field addition in Go defaults to nil (no external tee). |
| `executor.Agy.Run` | `internal/executor/agy.go:165-176` | Wrap `cmd.Stdout`/`cmd.Stderr` in `io.MultiWriter` with `req.StdoutDest`/`req.StderrDest` when non-nil. | Yes; preserves existing `Outcome` return values and `WaitDelay` semantics. |
| `executor.CursorAgent.Run` | `internal/executor/cursor_agent.go:87-98` | Wrap `cmd.Stdout`/`cmd.Stderr` in `io.MultiWriter` with `req.StdoutDest`/`req.StderrDest` when non-nil. | Yes; preserves existing `Outcome` return values and `WaitDelay` semantics. |
| `executor.Opencode.Run` | `internal/executor/opencode.go:126-137` | Wrap `cmd.Stdout`/`cmd.Stderr` in `io.MultiWriter` with `req.StdoutDest`/`req.StderrDest` when non-nil. | Yes; preserves existing `Outcome` return values and fallback checks. |
| `ledgerpath.ResolveLog` | `internal/ledgerpath/ledgerpath.go:36-38` | Add `ResolveLog(primaryRoot, runID, laneID string) string` returning `<primaryRoot>/.lucind/runs/<runID>/lanes/<laneID>.log`. | Yes; new additive helper function. |
| `run.Deps` | `internal/run/run.go:149-212` | Add optional log file opener factory or use `PrimaryRoot` with `ledgerpath.ResolveLog`. | Yes; additive field in dependency injection struct. |
| `run.Report` | `internal/run/run.go:220-250` | Add `LogPath string` field recording destination log path. | Yes; additive field on report struct. |
| `run.Execute` | `internal/run/run.go:368-375` | Create log destination file under `.lucind/runs/` and populate `executor.Request.StdoutDest`/`StderrDest`. | Yes; internal function logic change with identical public signature. |
| `serve.NewHandler` | `internal/serve/handlers.go:36-118` | Accept `primaryRoot string` and register `/api/runs/{runID}/lanes/{laneID}/tail`, `/api/runs/{runID}/lanes/{laneID}/log`, and Model routes. | Yes; existing `/`, `/api/state`, and `/approvals/` endpoints remain intact. |
| `cmd/lucind-ai/cli.go:serveDispatch` | `cmd/lucind-ai/cli.go:674-725` | Pass `primaryRoot` to `serve.NewHandler`. | Yes; CLI invocation and flags remain unchanged. |
| File layout `.lucind/runs/<run_id>/lanes/<lane_id>.log` | `internal/ledgerpath/ledgerpath.go:30-38` | New file path format on disk for durable raw process stdio logs. | Yes; additive path inside gitignored `.lucind/` directory (`.gitignore:2`). |

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `internal/executor/executor.go` | Modify | Add `StdoutDest` and `StderrDest` (`io.Writer`) fields to `Request` struct (`internal/executor/executor.go:15-37`). | `internal/run/run.go:368-374` (`run.Execute` constructing `executor.Request`) |
| `internal/executor/agy.go` | Modify | Tee `cmd.Stdout` and `cmd.Stderr` via `io.MultiWriter` when destination writers are present (`internal/executor/agy.go:169-173`). | `internal/executor/agy.go:173` (`cmd.Run()` invoked during `lucind-ai run` in `cmd/lucind-ai/cli.go:129-173`) |
| `internal/executor/cursor_agent.go` | Modify | Tee `cmd.Stdout` and `cmd.Stderr` via `io.MultiWriter` when destination writers are present (`internal/executor/cursor_agent.go:91-95`). | `internal/executor/cursor_agent.go:95` (`cmd.Run()` invoked during `lucind-ai run` in `cmd/lucind-ai/cli.go:129-173`) |
| `internal/executor/opencode.go` | Modify | Tee `cmd.Stdout` and `cmd.Stderr` via `io.MultiWriter` when destination writers are present (`internal/executor/opencode.go:130-134`). | `internal/executor/opencode.go:134` (`cmd.Run()` invoked during `lucind-ai run` in `cmd/lucind-ai/cli.go:129-173`) |
| `internal/ledgerpath/ledgerpath.go` | Modify | Add `ResolveLog` path resolver for primary-root log file locations (`internal/ledgerpath/ledgerpath.go:34-38`). | `internal/run/run.go:368-375` (spool creation) and `internal/serve/handlers.go:36-118` (log retrieval) |
| `internal/run/run.go` | Modify | Open primary log destination in `Execute`, pass writers to `executor.Request`, populate `Report.LogPath` (`internal/run/run.go:149-250,368-375`). | `cmd/lucind-ai/cli.go:129-173` (`runDispatch` handling batch execution and report output) |
| `internal/serve/handlers.go` | Modify | Add HTTP routes for SSE live tailing, raw log downloads, and Model query routing (`internal/serve/handlers.go:36-118`). | `cmd/lucind-ai/cli.go:674-725` (`serveDispatch`) and `openspec/changes/control-room-capture/proposal.md:104-122` |
| `cmd/lucind-ai/cli.go` | Modify | Wire `primaryRoot` into `serve.NewHandler` call within `serveDispatch` (`cmd/lucind-ai/cli.go:674-725`). | `cmd/lucind-ai/cli.go:113` (`run` entrypoint executing `serve` subcommand) |

## Open Questions

- [ ] Directory hierarchy: Should primary-root logs follow `.lucind/runs/<run_id>/lanes/<lane_id>.log` or `.lucind/logs/<run_id>/<lane_id>.log`?
- [ ] Stream splitting: Should stdout and stderr be interleaved into a single file or split into `.stdout.log` and `.stderr.log`?
- [ ] Route registration scope: Should `serve.Model` JSON endpoints (`/api/features`, `/api/attempts`) be registered alongside capture logs in `internal/serve/handlers.go:36-118` or deferred to `control-room-serve`?
- [ ] Skill contract precedence: `~/.claude/skills/sdd-design/SKILL.md` mandates a single monolithic `design.md` with Engram persistence and 800-word limit, superseded by this packet's parallel three-lens workflow writing `design-lens-b.md` under 1000 words.
