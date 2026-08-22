# Proposal: Control Room Capture

File-backed stream spooling on the primary repository `.lucind/` directory, with compact ledger notes and loopback HTTP access, is the viable shape. SQLite chunk logs and a required streaming daemon are not.

## Intent

Operators cannot watch an in-flight dispatch and cannot retrieve a successful lane’s full stdio.

`lucind-ai` runs `agy`, `cursor-agent`, and `opencode` in isolated git worktrees (`internal/worktree/worktree.go:150-171`; `docs/prd.md:12-18,61-70`) via `exec.CommandContext` into unbounded in-memory `bytes.Buffer`s (`internal/executor/agy.go:165-174`; `internal/executor/cursor_agent.go:87-95`; `internal/executor/opencode.go:126-135`). The per-lane budget is `20 * time.Minute` (`cmd/lucind-ai/cli.go:42`). Nothing tails those buffers while the child runs.

On `lane.Done` (`internal/result/result.go:117-135`), `Execute` writes a ledger note only when `reason != ""` (`internal/run/run.go:416-435`). A clean envelope leaves `Diagnosis` empty; `Report` has no stream fields (`internal/run/run.go:501-508`). The buffers die with the stack.

On timeout or non-zero exit (`internal/run/run.go:549-555`; `internal/executor/status.go:14-21`), `formatStreamDetail` keeps a 4096-byte tail per stream (`internal/run/run.go:89,132-144`) as `EventLaneNote` in SQLite `events` (`internal/ledger/schema.go:34-43`). That is a diagnosis, not a transcript.

`lucind-ai serve` is loopback HTTP (`cmd/lucind-ai/cli.go:674-725`; `internal/serve/server.go:19-53`). `NewHandler` routes `/`, `/api/state`, and `/approvals/` (`internal/serve/handlers.go:15-21,36-118`). `serveDispatch` never calls `NewModel` (`cmd/lucind-ai/cli.go:715`). `serve.Model` already queries features, attempts, leases, reconciliation, and audit (`internal/serve/model.go:14-125,128-343`) for `lucind-ai feature status` (`cmd/lucind-ai/cli.go:852`), not the HTTP UI.

## Selected Candidate and Approach

**Candidate 4 — Hybrid File-Backed Stream Spooling with Ledger Milestones and Model Query Routing.**

Lane-lifecycle hook: `Execute` builds `executor.Request` at `internal/run/run.go:369-374`. Capture today is the three `cmd.Stdout` / `cmd.Stderr` assignments (`internal/executor/agy.go:169-171`; `internal/executor/cursor_agent.go:91-93`; `internal/executor/opencode.go:130-132`). This change extends `Request` (`internal/executor/executor.go:14-37`) with a dest and tees those writers with `io.MultiWriter`. `Outcome` (`internal/executor/executor.go:42-63`) stays diagnosis-only.

1. **Primary-root file spooling.** Open log files under `<primaryRoot>/.lucind/…` at spawn; tee stdout/stderr for every executor. Files persist for `Done`, `Blocked`, `Failed`, and `Deviated`. They must live on the primary root: `completeIntegration` persists the envelope then removes the worktree (`internal/run/integrate.go:159-163`; `cmd/lucind-ai/cli.go:641-660`). `.lucind/` is gitignored (`.gitignore:2`).
2. **Lean ledger notes.** Keep megabyte streams off SQLite. The ledger already uses `_pragma=busy_timeout(5000)` (`internal/ledger/ledger.go:126-128,162-163`) and a 4-connection pool (`internal/ledger/ledger.go:180-184`) while concurrent lanes write events and statuses (`internal/run/run.go:425-434`; `internal/run/batch.go:66-89,147-214`). `events.type` stays the six lifecycle values (`internal/ledger/schema.go:34-43`). Failure notes stay capped at 4096 bytes/stream.
3. **Loopback tail, download, and Model JSON.** Existing `lucind-ai serve` gains live SSE tailing (`http.Flusher`), post-mortem transcript download, and JSON routes over `serve.NewModel` (`internal/serve/model.go:21-24`). `run` and `serve` stay independent processes (`cmd/lucind-ai/cli.go:99-127,129-173,674-725`; `docs/prd.md:188-193`). Which change registers those routes is an open question.
4. **WaitDelay preserved.** Keep `exec.Cmd.WaitDelay` (5s default) and `exec.ErrWaitDelay` handling (`internal/executor/agy.go:15-39,182-197`; `internal/executor/cursor_agent.go:104-118`; `internal/executor/opencode.go:143-160`) so grandchild-held pipes still set `Outcome.OutputTruncated` and `Report.OutputCaptureIncomplete` (`internal/run/run.go:231-242,506`) without failing a valid `Done` envelope.

## Conceptual Changes

- **Primary-root log domain.** Transcripts outlive worktrees deleted after integrate (`internal/run/integrate.go:159-163`). Prefix (`runs/` vs `logs/`) and split vs interleaved files are unset.
- **Executor dest, unchanged Outcome.** Incremental tee; Outcome contract unchanged.
- **Serve as Control Room read surface.** Today approvals-only (`internal/serve/handlers.go:15-21,36-118`); Candidate 4 activates Model plus file tail/download.
- **Store split.** SQLite stays the small coordination ledger (`docs/prd.md:194-205`; `internal/ledger/schema.go:10-57`). Raw process bytes stay on disk.

## Capabilities

### New Capabilities
- `control-room-capture`: Continuous primary-root stdio spooling for all executors, bounded SQLite notes, loopback live-tail and post-mortem retrieval.

### Modified Capabilities
- `lane-execution`: `Execute` / executor capture path spools to disk for every terminal status; WaitDelay truncation semantics unchanged.
- `approvals-web-ui`: Loopback server adds log SSE/download (and Model JSON if this change registers those routes). Existing approval rules stay.

No `ledger-events` spec exists. Capped `events.detail` belongs under `lane-execution` / `control-room-capture`.

## User and Capability Impact

| Capability | Impact | Description | Seam |
|---|---|---|---|
| `lane-execution` | Modified | Tee child stdio to primary-root files for `done`/`blocked`/`deviated`/`failed`; stop discarding on exit 0. | `internal/run/run.go:369-374,416-435,501-508`; executor `Request`/`Outcome` and the three `cmd.Stdout` assignments |
| `control-room-capture` | Added | Durable logs under `<primaryRoot>/.lucind/…`, isolated from worktree deletion. | `internal/run/integrate.go:159-165`; `internal/ledgerpath/ledgerpath.go:23-58` |
| `approvals-web-ui` | Modified | Loopback SSE tail + transcript download; no daemon, no WebSockets. | `internal/serve/handlers.go:36-118`; `internal/serve/server.go:19-53`; `cmd/lucind-ai/cli.go:674-725` |
| Ledger notes (no standalone spec) | Unchanged bound | `streamDetailCap` 4096 bytes/stream on failure/timeout/unreadable envelope; full bytes on disk. | `internal/run/run.go:89,132-144,422-435`; `internal/ledger/schema.go:34-43` |

## Delta Specifications

### Requirement: Continuous primary-root stream spooling

Executors MUST stream child stdout and stderr to durable files on `<primaryRoot>/.lucind/…` for `agy`, `cursor-agent`, and `opencode`. Files MUST be created at spawn and MUST persist across `Done`, `Blocked`, `Failed`, and `Deviated`, including after worktree deletion.

#### Scenario: Successful lane keeps the transcript
- GIVEN exit 0 and a valid envelope (`lane.Done`)
- WHEN `completeIntegration` removes the worktree (`internal/run/integrate.go:159-165`)
- THEN the primary-root log MUST retain full unclipped stdout and stderr

#### Scenario: Uniform tee across executors
- GIVEN any of the three executors
- WHEN the child writes stdout or stderr
- THEN output MUST be tee’d incrementally to the primary-root log

#### Scenario: Logs survive worktree destruction
- GIVEN concurrent worktree lanes (`internal/run/batch.go:66-89`)
- WHEN integrate persists envelopes and deletes worktrees (`cmd/lucind-ai/cli.go:641-660`; `internal/run/integrate.go:159-165`)
- THEN files under `<primaryRoot>/.lucind/` MUST remain

### Requirement: Non-interfering WaitDelay drain

Spooling MUST NOT change WaitDelay handling. A grandchild holding pipes past WaitDelay MUST set `Outcome.OutputTruncated` and `Report.OutputCaptureIncomplete`, keep the real exit code, and MUST NOT fail the lane solely for incomplete drain.

#### Scenario: Grandchild holds pipes
- GIVEN exit 0 while a grandchild holds pipes past WaitDelay
- WHEN `WaitDelay` returns `exec.ErrWaitDelay`
- THEN `OutputTruncated` is true, `ProcessState` exit is preserved, and a valid envelope still yields `lane.Done` (`internal/run/run.go:402,506`)

#### Scenario: Truncation diagnostic
- GIVEN `OutputTruncated == true`
- WHEN `Execute` persists terminal status (`internal/run/run.go:480-499`)
- THEN an `EventLaneNote` with `outputTruncatedDetail` (`internal/run/run.go:62-70`) is appended without changing status

### Requirement: Bounded SQLite diagnostics

`events.detail` MUST stay capped at 4096 bytes per stream for failed, timed-out, or unreadable-envelope dispatches. The binary MUST NOT store unclipped stream blobs in SQLite.

#### Scenario: Large failure clipped in the ledger, complete on disk
- GIVEN 50KiB output and non-zero exit or timeout
- WHEN `Execute` records the note (`internal/run/run.go:422-435`)
- THEN the note contains at most the 4096-byte tail plus truncation marker; the file has all 50KiB

#### Scenario: Clean Done writes no failure note
- GIVEN `lane.Done` and empty reason (`internal/run/run.go:416-435`)
- WHEN `Execute` finishes
- THEN no failure-detail `EventLaneNote` is appended

### Requirement: Loopback HTTP stream access

`lucind-ai serve` MUST expose read-only loopback endpoints for live SSE tail and finished-transcript download. HTTP backpressure or disconnect MUST NOT stall the child. Route registration may land here or in `control-room-serve`.

#### Scenario: Live tail
- GIVEN an in-flight lane under `lucind-ai run` (`cmd/lucind-ai/cli.go:129-173`)
- WHEN a client connects to the log stream on `serve`
- THEN the server streams file appends via SSE without backpressuring the child

#### Scenario: Post-mortem download
- GIVEN `run` has exited (`cmd/lucind-ai/cli.go:99-113`)
- WHEN a client requests the lane transcript
- THEN HTTP 200 with stored content, or 404 if missing/unstarted

#### Scenario: Client disconnect
- GIVEN an active SSE connection
- WHEN the client disconnects (`r.Context().Done()`)
- THEN the handler exits without leaking or stalling the child

## Alternatives Considered

- **Candidate 1 — File spool + SSE, no Model routing.** Rejected: leaves `serve.Model` (`internal/serve/model.go:14-125,128-343`) unused by HTTP; UI stays approvals-only (`internal/serve/handlers.go:15-21`).
- **Candidate 2 — SQLite `lane_stream_chunks` + long-poll.** Rejected: multi-megabyte streams contend with barrier/status writes under busy_timeout 5000 and a 4-conn pool (`internal/run/run.go:425-434`; `internal/run/batch.go:147-214`; `internal/ledger/ledger.go:126-128,162-163,180-184`).
- **Candidate 3 — Required serve daemon, Unix sockets, WebSockets.** Rejected: `run` and `serve` are independent subcommands (`cmd/lucind-ai/cli.go:99-127`; `docs/prd.md:188-193`). Current serve is stdlib `net/http` (`internal/serve/server.go:19-53`).

## Technical Risks and Failure Modes

| Risk | Impact | Mitigation | Seam |
|---|---|---|---|
| Grandchild inherits stdio; WaitDelay truncates capture | Incomplete `Outcome` streams; must not fail the lane | Keep non-zero WaitDelay; tee must still set `OutputCaptureIncomplete` | `internal/executor/agy.go:15-39,165-197` and sibling WaitDelay paths; `internal/run/run.go:62-70,488-499,506` |
| Unbounded `bytes.Buffer` × concurrent lanes | Heap growth on multi-megabyte dispatches | Disk tee; bound any UI replay buffer (~1MB/lane) | executor Stdout/Stderr assignments; `internal/run/batch.go:80-89` |
| Worktree-local logs vanish on integrate | Lost transcripts after merge | Write only on primary `.lucind/`; `ledgerpath.Validate` exists (unwired: `internal/ledgerpath/ledgerpath.go:7-14,23-58`) | `internal/run/integrate.go:159-165`; `cmd/lucind-ai/cli.go:641-660` |
| Slow SSE client backpressures the child | Hung dispatch | Tail the file (or non-blocking subscribers); watch `r.Context().Done()` | `internal/serve/handlers.go:36-118`; `internal/serve/server.go:19-53` |
| ANSI in ledger notes | `formatStreamDetail` truncates only (`internal/run/run.go:132-144`) | Raw bytes on disk; strip ANSI before the 4096-byte note | `internal/run/run.go:89,132-144,422-435` |
| Done path discards streams | Empty reason skips notes; buffers dropped | Unconditional spool from spawn | `internal/result/result.go:117-135`; `internal/run/run.go:416-435,501-508` |

## Rollback Plan and Additivity

Revert with `git revert <sha>`. That restores `bytes.Buffer` capture in `internal/executor/` and the diagnosis-only note path in `internal/run/run.go:422-435`. Gitignored `.lucind/` files (`.gitignore:2`) are working state; leave or delete them — no DB rollback.

Additive: logs sit beside `.lucind/results/` (`cmd/lucind-ai/cli.go:655-660`) and `.lucind/lucind.db` (`internal/ledgerpath/ledgerpath.go:30-38`). Schema stays `schemaVersion = 5` (`internal/ledger/schema.go:9-10`). Optional new `events.type` values would follow the events-table rebuild pattern (`internal/ledger/schema.go:38-39,59-78`). Embedded `result.schema.json` and envelope types (`internal/result/schema.go:10-28`; `internal/result/result.go:43-135`) stay unchanged. Existing serve routes remain.

## Test and Validation Impact

| Layer | Coverage | Existing seam |
|---|---|---|
| Executor spooling | Tee to dest; ExitCode / TimedOut / OutputTruncated unchanged; opencode fallback warning preserved | `internal/executor/agy_test.go:28-60,158-218`; `cursor_agent_test.go:28-60`; `opencode_test.go:28-80` |
| Path boundary | Log dir under primary `.lucind/`; `Validate` rejects worktree-shaped paths | `internal/ledgerpath/ledgerpath_test.go:9-35,37-60` |
| Run lifecycle | Files persist for Done/Blocked/Failed/Deviated; 4096-byte notes continue; `TestExecuteTruncatedOutcomeStillYieldsDoneAndReportsCapture` | `internal/run/run_test.go:645-730,820-950`; `internal/run/run.go:89,132-144,488-499` |
| Batch / integrate | Concurrent isolated files; worktree removal leaves primary logs | Extend `TestExecuteBatchRunsLanesConcurrentlyNotSequentially` (`internal/run/batch_test.go:530`); `TestCompleteIntegrationPersistsEnvelopeForEveryIntegratedLane` (`internal/run/integrate_test.go:392`) |
| Serve | SSE tail, 404 missing, disconnect cleanup, `ErrNonLoopback` | `internal/serve/server_test.go:17-40` (loopback today; SSE tests are new) |
| CLI | `lucind-ai run` with a stub child creates primary `.lucind/` logs | New tests in `cmd/lucind-ai`; current `cli_test.go:37-48` is usage-only |

## Out of Scope

- UI chrome, xterm views, layout (`control-room-ui-shell`, `control-room-ui-views`)
- Listener lifecycle and mux architecture (`control-room-serve`); log/Model route registration may still land here — see Open Questions
- SQLite schema migrations (`control-room-ledger`); token/timeline telemetry (`control-room-telemetry`)
- `gentle-ai` gates; PTY / `Setpgid` (not present in executors today)

## Open Questions

- [ ] Directory layout: `.lucind/runs/<run_id>/lanes/<lane_id>.log` vs `.lucind/logs/<run_id>/<lane_id>.log`? One interleaved file vs `.stdout.log` / `.stderr.log`?
- [ ] Route ownership: register log SSE, download, and Model JSON in this change (`internal/serve/handlers.go:36-118`) or in `control-room-serve`?
- [ ] Retention: copy run logs at archive (today only `.lucind/packets/` and `.lucind/results/` — `plugin/claude-code/skills/lucind-ai/SKILL.md:280-285`), prune via `lucind-ai worktree cleanup` / a dedicated command (`cmd/lucind-ai/cli.go:56`), or leave gitignored?

## Success Criteria

- [ ] All three executors spool stdout and stderr to primary-root files for Done, Blocked, Failed, and Deviated.
- [ ] `events.detail` stays bounded (`internal/run/run.go:89,132-144`); no unbounded blobs in SQLite.
- [ ] Loopback-only access (`internal/serve/server.go:19-53`) can download a finished transcript and tail an in-flight file, including after `run` has exited (`cmd/lucind-ai/cli.go:99-113`).
- [ ] WaitDelay / deadline behavior is unchanged under spooling.
- [ ] Archive/retention rule for run logs is explicit relative to today’s packets/envelopes copy.
