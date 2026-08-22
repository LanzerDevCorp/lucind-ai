# Proposal Lens B — Capability Impact & Specs: Control Room Capture

## User & Capability Impact

| Capability | Impact (Added / Modified / Removed) | Description | Existing seam (file:line) |
|---|---|---|---|
| `lane-execution` | Modified | Spool subprocess stdio to primary-root logs during execution, preserving full transcripts across all statuses (`done`, `blocked`, `deviated`, `failed`) instead of discarding on exit 0. | `internal/run/run.go:402,416-435,501-508`, `internal/executor/executor.go:15-37,42-63`, `internal/executor/agy.go:165-176`, `internal/executor/cursor_agent.go:87-98`, `internal/executor/opencode.go:126-137` |
| `control-room-capture` | Added | Continuous stream capture for child executors to primary repository logs (`<primaryRoot>/.lucind/runs/<run_id>/lanes/<lane_id>.log`), isolated from worktree deletion and SQLite contention. | `internal/executor/agy.go:169-175`, `internal/executor/cursor_agent.go:91-97`, `internal/executor/opencode.go:130-136`, `internal/run/integrate.go:159-165`, `internal/ledgerpath/ledgerpath.go:23-58` |
| `approvals-web-ui` | Modified | Loopback server (`lucind-ai serve`) adds live stream tailing (SSE) and transcript downloads for finished lanes, without requiring a daemon or WebSockets. | `internal/serve/handlers.go:36-118`, `internal/serve/server.go:19-53`, `internal/serve/model.go:14-25,128-145`, `cmd/lucind-ai/cli.go:674-725` |
| `ledger-events` | Modified | SQLite `events.detail` diagnostic notes remain capped at `streamDetailCap` (4096 bytes/stream) on failure/timeout, while complete streams reside on disk. | `internal/run/run.go:89,125-144,422-435`, `internal/ledger/schema.go:34-43` |

## Delta Specifications

### Requirement: Continuous Primary-Root Stream Spooling

The executor and run pipeline MUST stream child stdout and stderr continuously to durable log files on the primary repository root (`<primaryRoot>/.lucind/...`) for all supported executors (`agy`, `cursor-agent`, `opencode`). Log files MUST be created at process spawn and MUST persist across all terminal statuses (`lane.Done`, `lane.Blocked`, `lane.Failed`, `lane.Deviated`), ensuring transcripts survive lane worktree deletion.

#### Scenario: Successful lane preserves complete stream transcript

- GIVEN an executor child process completing with exit code 0 and valid envelope (`lane.Done`)
- WHEN `completeIntegration` removes the lane worktree (`internal/run/integrate.go:159-165`)
- THEN the primary root log file MUST retain the full unclipped stdout and stderr output (`internal/run/run.go:501-508`)

#### Scenario: Uniform spooling across all executors

- GIVEN a lane dispatched to any executor (`agy`, `cursor-agent`, `opencode`)
- WHEN the child writes output to stdout or stderr (`internal/executor/agy.go:169-175`; `internal/executor/cursor_agent.go:91-97`; `internal/executor/opencode.go:130-136`)
- THEN output MUST be tee-streamed incrementally to the primary root log file via `io.MultiWriter` or file tee

#### Scenario: Primary root logs survive worktree destruction

- GIVEN lanes executing concurrently in isolated worktrees (`internal/run/batch.go:66-89`)
- WHEN `completeIntegration` promotes green lanes, deletes worktrees, and calls `PersistEnvelope` (`cmd/lucind-ai/cli.go:641-660`; `internal/run/integrate.go:159-165`)
- THEN log files under `<primaryRoot>/.lucind/` MUST remain intact

### Requirement: Non-Interfering WaitDelay Drain and Truncation Semantics

Stream spooling MUST NOT alter `exec.Cmd.WaitDelay` drain handling. When grandchild processes hold stdio pipes open past `WaitDelay`, the executor MUST mark `Outcome.OutputTruncated = true` and `Report.OutputCaptureIncomplete = true`, preserving the child's true exit code without failing or blocking the lane solely due to incomplete pipe draining.

#### Scenario: Grandchild holds pipes open past WaitDelay

- GIVEN a child process exits 0 while an inherited grandchild holds pipes open past `WaitDelay` (`internal/executor/agy.go:15-39,182-197`; `internal/executor/cursor_agent.go:104-118`; `internal/executor/opencode.go:143-160`)
- WHEN `WaitDelay` expires returning `exec.ErrWaitDelay`
- THEN `Outcome.OutputTruncated` MUST be true, true exit code MUST be preserved from `ProcessState`, and lane status MUST evaluate to `lane.Done` on valid `result.json` (`internal/run/run.go:402,506`)

#### Scenario: Ledger records diagnostic truncation event

- GIVEN an executor outcome with `OutputTruncated == true`
- WHEN `Execute` persists terminal status (`internal/run/run.go:480-499`)
- THEN an `EventLaneNote` diagnostic event with `outputTruncatedDetail` MUST be appended to `events` without altering terminal status

### Requirement: Bounded SQLite Ledger Diagnostics

SQLite `events.detail` diagnostic notes MUST remain capped at `streamDetailCap` (4096 bytes per stream) for failed, timed-out, or unreadable-envelope dispatches (`internal/run/run.go:89,125-144,422-435`). The binary MUST NOT store unclipped stream blobs in SQLite, avoiding database write amplification.

#### Scenario: Large failure output clipped in ledger notes

- GIVEN a dispatch produces 50KiB output and exits with non-zero code or timeout (`internal/run/run.go:549-555`)
- WHEN `Execute` records the failure diagnostic in `events.detail` (`internal/run/run.go:422-435`)
- THEN the note MUST contain at most the 4096-byte tail of each stream with a truncation marker, while the complete 50KiB transcript is preserved on disk

#### Scenario: Clean Done lane writes no failure detail note

- GIVEN a dispatch completes with `status == lane.Done` and empty reason string (`internal/run/run.go:416-435`)
- WHEN `Execute` finishes
- THEN no diagnostic failure detail note MUST be appended to `events`, keeping the ledger lean

### Requirement: Loopback HTTP Stream Access and Post-Mortem Retrieval

`lucind-ai serve` MUST provide read-only HTTP endpoints bound strictly to loopback (`127.0.0.1`) for live tailing in-flight lane streams via SSE using stdlib `http.Flusher`, and downloading post-mortem log transcripts for finished lanes (`internal/serve/handlers.go:36-118`; `internal/serve/server.go:19-53`). HTTP consumer backpressure or client disconnection MUST NOT stall child execution.

#### Scenario: Live tailing in-flight lane via SSE

- GIVEN an in-flight lane executing under `lucind-ai run` (`cmd/lucind-ai/cli.go:129-173`)
- WHEN a client connects to the log stream endpoint on `lucind-ai serve` (`internal/serve/handlers.go:36-118`)
- THEN the server MUST stream new file appends via SSE without backpressuring the child process

#### Scenario: Post-mortem log download after run completion

- GIVEN a completed run whose CLI process has exited (`cmd/lucind-ai/cli.go:99-113`)
- WHEN an HTTP client requests the lane log transcript from `lucind-ai serve`
- THEN the server MUST return HTTP 200 with the complete stored log content

#### Scenario: Missing or unstarted lane returns 404

- GIVEN a request for a non-existent run ID or unstarted lane ID
- WHEN an HTTP client requests the log endpoint
- THEN the server MUST return HTTP 404 Not Found

#### Scenario: Client disconnect terminates SSE handler cleanly

- GIVEN an active SSE stream connection to an in-flight lane
- WHEN the client disconnects (`r.Context().Done()`)
- THEN the server stream goroutine MUST terminate cleanly without leaking resources or stalling child execution

## Open Questions

- [ ] Directory naming and stream layout: Should primary-root logs use `.lucind/runs/<run_id>/lanes/<lane_id>.log` or `.lucind/logs/<run_id>/<lane_id>.log`, and should stdout/stderr be single interleaved or split into `.stdout.log` and `.stderr.log`?
- [ ] Route registration boundary: Should HTTP log streaming and model query routing registration be implemented within `control-room-capture` in `internal/serve/handlers.go:36-118` or deferred to `control-room-serve`?
- [ ] Log archival and retention: Should run logs under gitignored `.lucind/` be archived during session archive (`plugin/claude-code/skills/lucind-ai/SKILL.md:280-285`) or pruned via a cleanup command (`cmd/lucind-ai/cli.go:56`)?
- [ ] Skill contract precedence: `~/.claude/skills/sdd-propose/SKILL.md` describes a monolithic `proposal.md` with PRD proposal question rounds and Engram persistence, intentionally superseded by this packet's parallel three-lens workflow writing `propose-lens-b.md`.
