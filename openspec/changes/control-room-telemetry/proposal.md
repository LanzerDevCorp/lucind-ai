# Proposal: Control Room Telemetry

## Intent

Give operators a live tail of lane stdout/stderr while `lucind-ai` dispatches parallel worktrees, without turning the SQLite ledger into a log store or weakening localhost-only serve.

Lanes run in isolated git worktrees (`internal/worktree/worktree.go:179-238`, `internal/run/batch.go:81-89`). Three constraints hide execution until the child exits:

1. **Buffered child I/O.** `agy`, `cursor-agent`, and `opencode` assign stdout/stderr to in-memory `bytes.Buffer` and call `cmd.Run()` (`internal/executor/agy.go:169-175`, `internal/executor/cursor_agent.go:91-97`, `internal/executor/opencode.go:130-136`). Capture exists only as `executor.Outcome` after exit (`internal/executor/executor.go:42-63`). Default per-lane timeout is 20 minutes (`cmd/lucind-ai/cli.go:42`).
2. **Closed, small ledger.** `events.type` admits only `run_started`, `lane_registered`, `lane_status_changed`, `lane_note`, `barrier_released`, `run_ended` (`internal/ledger/schema.go:34-43`). Diagnosis notes are capped at 4096 bytes per stream (`internal/run/run.go:71-89`). Concurrent batch writes (`internal/run/batch.go:66-113`) already share WAL SQLite (`busy_timeout=5000`, `internal/ledger/ledger.go:127-129,162-164`) with `SetStatus` (`internal/run/run.go:348-351`) and feature lease renewals (`internal/run/run.go:203-211`, `internal/feature/feature.go:354-385`).
3. **Polling-only serve.** `lucind-ai serve` binds loopback (`cmd/lucind-ai/cli.go:674-725`, `internal/serve/server.go:12-22`). `/api/state` returns approver identity, rate, and pending approvals (`internal/serve/handlers.go:79-85`). `NewHandler` registers `/`, `/api/state`, and `/approvals/` only (`internal/serve/handlers.go:36-85`). `serve.Model` is a shell-free feature/attempt DTO layer (`internal/serve/model.go:14-25`), not run telemetry.

## Selected Candidate and Approach

**Candidate 2: worktree-local log files plus an in-memory SSE hub** (`openspec/changes/control-room-telemetry/explore.md:23-30`).

Hook at `exec.Run` inside `run.Execute` (`internal/run/run.go:368-375`):

1. **Tee stdout/stderr.** Replace `bytes.Buffer` with `io.MultiWriter` to a worktree-local log under `wt.Path` (Execute already writes `.lucind/` there via `writeResultSchema`, `internal/run/run.go:311-316`) and an in-memory hub in `internal/serve`.
2. **Keep WaitDelay.** Preserve `cmd.WaitDelay` and map `exec.ErrWaitDelay` to `Outcome.OutputTruncated = true` with `cmd.ProcessState.ExitCode()` (`internal/executor/agy.go:160-168,182-197`, `internal/executor/cursor_agent.go:82-90,104-118`, `internal/executor/opencode.go:121-129,143-159`). Optional `io.Writer` sinks on `executor.Request` (`internal/executor/executor.go:14-37`); `Executor.Run` and `Outcome` keep their current signatures (`internal/executor/executor.go:42-63,65-80`).
3. **Loopback SSE.** Add `/api/telemetry/events` on the existing mux (`internal/serve/handlers.go:36-85`) using stdlib `net/http` and `http.Flusher` (`internal/serve/server.go:16-53`). Bind stays loopback via `serve.IsLoopback` (`internal/serve/server.go:12-22,55-73`). Unregister subscribers on client disconnect. Do not change per-item decide (`internal/serve/handlers.go:148-211`).
4. **Ledger stays coarse.** Lifecycle rows remain `AppendEvent` / `SetStatus` (`internal/ledger/ledger.go:366-381,448-485`). Failure notes stay capped by `streamDetailCap` (`internal/run/run.go:71-89,422-435`). Integration checks still audit through `WriteWithAudit` (`internal/ledger/schema.go:171-180`, `internal/run/attempt.go:408-443`). High-frequency chunks never insert into `events`.

### Rejected alternatives

- **SQLite `telemetry_events` ingest.** WAL serializes writes; parallel lanes and `RenewLease` would contend (`internal/run/batch.go:66-113`, `internal/feature/feature.go:354-385`, `internal/ledger/ledger.go:127-129`) against the small-ledger rule (`internal/run/run.go:71-78`). New `events.type` values fail the CHECK (`internal/ledger/schema.go:38-39`).
- **Per-CLI NDJSON + Unix-socket gateway.** All three CLIs already request JSON (`internal/executor/agy.go:143`, `internal/executor/cursor_agent.go:70`, `internal/executor/opencode.go:108`), but live parsing couples to upstream schema drift and adds IPC vs the single-user constraint (`docs/prd.md:48-57`).
- **WebSockets / OTLP sidecars.** Extra dependencies; `go.mod` has no websocket library; unidirectional SSE on stdlib HTTP is enough inside loopback (`internal/serve/server.go:12-22`).

## Conceptual Changes

- **Dual-tier telemetry.** Tier 1: unbounded stdout/stderr through worktree files and the in-memory SSE hub. Tier 2: `lane.Status` (`internal/lane/status.go:8-28`), approvals (`internal/ledger/schema.go:45-56`), and capped `lane_note` rows stay in SQLite.
- **Shell-free run history.** Extend `serve.Model` with read-only DTOs over `Ledger.Events` (`internal/ledger/ledger.go:488-526`) without `os/exec` (`internal/serve/model_test.go:595-627`). Duration is observed at the `exec.Run` seam (`internal/run/run.go:368-375`), not a column on `events`.
- **Barrier decoupling.** Bounded stream flush (<500ms) before completion; `Evaluate` releases only when every lane is terminal (`internal/barrier/barrier.go:36-52`, `internal/run/batch.go:29-65`). Approval wait already persists terminal status before Observe (`internal/run/run.go:437-450`).

## Capabilities

### New Capabilities

- `lane-telemetry-streaming`: live tee of child stdout/stderr to a worktree log and SSE hub.
- `shell-free-telemetry-query`: read-only `serve.Model` DTOs over ledger lifecycle events.

### Modified Capabilities

- `approvals-web-ui`: additive SSE route; loopback and individual decide unchanged (`openspec/specs/approvals-web-ui/spec.md:10-25,26-48`).
- `lane-execution`: tee inside `Execute`; six-value `lane.Status` unchanged; barrier still waits for persisted terminal status (`openspec/specs/lane-execution/spec.md:10-48`).
- `parent-feature-integration`: keep `WriteWithAudit` phase rows; do not stream check output through `events`.

| Capability | Impact | Description | Seam |
|---|---|---|---|
| `lane-telemetry-streaming` | Added | Tee stdout/stderr to worktree log + in-memory hub instead of unbounded buffers. | `internal/run/run.go:368-375`, `internal/executor/agy.go:169-175`, `internal/executor/cursor_agent.go:91-97`, `internal/executor/opencode.go:130-136` |
| `approvals-web-ui` | Modified | Loopback SSE for live output; no change to bind or per-item decide. | `internal/serve/server.go:16-23,55-73`, `internal/serve/handlers.go:36-85,148-211` |
| `shell-free-telemetry-query` | Added | Model DTOs from `Ledger.Events`; no shell/git. | `internal/serve/model.go:14-25`, `internal/ledger/ledger.go:488-526`, `internal/serve/model_test.go:595-627` |
| `lane-execution` | Modified | Live flush during dispatch; status enum and barrier Observe unchanged. | `internal/lane/status.go:8-28`, `internal/run/run.go:348-351`, `internal/barrier/barrier.go:36-47` |
| `parent-feature-integration` | Modified | Attempt phases and check results still go through `WriteWithAudit`; no high-frequency SQLite chunks. | `internal/run/attempt.go:213-214,408-443`, `internal/integrate/integrate.go:90-109`, `internal/ledger/ledger.go:832-873` |

## Delta Specifications

### Requirement: Worktree-local log teeing

The executor dispatch loop MUST stream child stdout and stderr concurrently to a worktree-local log under `wt.Path` and an in-memory hub via `io.MultiWriter` (`internal/executor/agy.go:169-175`, `internal/executor/cursor_agent.go:91-97`, `internal/executor/opencode.go:130-136`, `internal/run/run.go:368-375`). It MUST honor `cmd.WaitDelay` and set `Outcome.OutputTruncated` when pipes stay open, preserving the observed exit code (`internal/executor/agy.go:160-197`, `internal/executor/cursor_agent.go:82-118`, `internal/executor/opencode.go:121-159`).

#### Scenario: Live output during execution

- GIVEN a running lane in an isolated worktree (`internal/run/run.go:368-375`)
- WHEN the child writes stdout or stderr (`internal/executor/agy.go:169-175`)
- THEN the bytes MUST be written to the worktree log and broadcast to subscribers without blocking the child.

#### Scenario: Grandchild holds stdio open

- GIVEN the child exited 0 while a grandchild keeps pipes open (`internal/executor/agy.go:182-197`)
- WHEN `cmd.WaitDelay` elapses (`internal/executor/agy.go:160-168`)
- THEN the executor MUST return `OutputTruncated = true` with exit code 0 and MUST NOT hang.

### Requirement: Loopback SSE stream

`internal/serve` MUST expose an SSE endpoint (e.g. `/api/telemetry/events`) via stdlib `net/http` and `http.Flusher` on the existing mux (`internal/serve/handlers.go:36-85`). It MUST keep loopback bind through `serve.IsLoopback` (`internal/serve/server.go:16-23,55-73`, `openspec/specs/approvals-web-ui/spec.md:10-25`). Streaming MUST NOT bypass per-item approval controls (`internal/serve/handlers.go:148-211`, `openspec/specs/approvals-web-ui/spec.md:26-48`). Client disconnect MUST unregister the subscriber.

#### Scenario: Loopback client subscribes

- GIVEN `lucind-ai serve` on `127.0.0.1:7433` (`cmd/lucind-ai/cli.go:674-725`, `internal/serve/server.go:16-23`)
- WHEN a loopback client requests the SSE endpoint
- THEN the server MUST return 200 with `Content-Type: text/event-stream` and flush lane output as it arrives.

### Requirement: Ledger isolation

High-frequency chunks MUST NOT insert into `events` (`internal/ledger/schema.go:34-43`, `internal/ledger/ledger.go:366-381`). SQLite MUST record only the six admitted lifecycle types via `SetStatus` and `AppendEvent` (`internal/ledger/ledger.go:360-381,448-485`). `lane_note` detail MUST remain bounded by `streamDetailCap` (4096 bytes per stream) (`internal/run/run.go:71-89,422-435`).

#### Scenario: High-volume output stays off the ledger

- GIVEN a lane emitting multi-megabyte stdout (`internal/run/run.go:368-375`)
- WHEN the child runs and completes (`internal/executor/executor.go:42-63`)
- THEN raw stdout MUST live in the worktree log while `events` holds only coarse lifecycle rows (`internal/ledger/ledger.go:366-381,448-485`).

#### Scenario: Terminal failure records a capped note

- GIVEN non-zero exit and stderr exceeding 4096 bytes (`internal/run/run.go:422-435`)
- WHEN `Execute` records the failure note
- THEN `lane_note` detail MUST truncate to 4096 bytes per stream with `streamTruncatedMarker` (`internal/run/run.go:71-100`).

### Requirement: Shell-free queries and status invariants

`serve.Model` MUST query run lifecycle events from SQLite without `os/exec` (`internal/serve/model.go:14-25`, `internal/serve/model_test.go:595-627`). Streaming MUST NOT add a seventh `lane.Status` (`internal/lane/status.go:8-28`) and MUST NOT delay barrier release beyond bounded flush (`internal/barrier/barrier.go:36-47`, `internal/run/batch.go:29-65`).

#### Scenario: Shell-free event query

- GIVEN completed lanes recorded in the ledger (`internal/ledger/ledger.go:488-526`)
- WHEN `serve.Model` runs a telemetry query
- THEN it MUST return DTOs from SQLite with no external process or git (`internal/serve/model_test.go:595-627`).

#### Scenario: Barrier waits for terminal persist

- GIVEN parallel lanes with active streams (`internal/run/batch.go:19-68`)
- WHEN a lane finishes and flushes (`internal/run/run.go:368-375`)
- THEN the batch barrier MUST NOT release until every lane's terminal status is persisted (`internal/barrier/barrier.go:36-47`, `internal/run/run.go:348-351`).

## Risks

| Risk | Likelihood | Mitigation | Seam |
|---|---|---|---|
| High-frequency SQLite writes cause `SQLITE_BUSY` and starve `RenewLease` | High | Files + SSE; SQLite for coarse events only | `internal/ledger/ledger.go:127-129,162-164`, `internal/feature/feature.go:354-385` |
| New `events.type` values fail CHECK | High | Keep stream events off `events`; v6 only if coarse milestones are chosen later | `internal/ledger/schema.go:34-43` |
| Grandchild MCP pipes hang `Wait` | High | Keep `WaitDelay` and `ProcessState.ExitCode` on `ErrWaitDelay` | `internal/executor/agy.go:160-168,182-197`, `internal/executor/cursor_agent.go:82-90,104-118`, `internal/executor/opencode.go:121-129,143-159` |
| Unbounded `bytes.Buffer` RAM | Medium | `MultiWriter` to file + hub; keep 4096-byte diagnosis tails | `internal/executor/agy.go:169-175`, `internal/run/run.go:71-89` |
| SSE on a non-loopback bind | Medium | `IsLoopback` on listen; no new auth | `internal/serve/server.go:12-22,55-73` |
| Stream flush delays barrier Observe | Low | Bounded flush (<500ms); Observe already waits for terminal status | `internal/run/batch.go:29-65`, `internal/run/run.go:437-450` |
| Logs vanish on worktree removal | Low | Open question: archive before `RemoveLaneWorktree` / `worktree cleanup` | `cmd/lucind-ai/cli.go:641-646,1460-1474` |

## Rollback and Additivity

`git revert` of the telemetry commits. The binary restores `bytes.Buffer` in the three executors and drops the SSE route. Schema version stays 5 (`internal/ledger/schema.go:9-10`) unless an optional later v6 lands in `migrate` (`internal/ledger/schema.go:224-306`); v5 readers ignore unused additive tables. Worktree logs do not rewrite git history. `PersistEnvelope` today writes `.lucind/results/<laneID>.json` only (`cmd/lucind-ai/cli.go:647-660`).

Additive: no mutation of `lanes`, `events`, or `approvals` CHECKs (`internal/ledger/schema.go:18-32,34-43,45-56`). Packet envelope types stay as-is (`internal/result/result.go:10-40`, `.lucind/result.schema.json:1-159`). New SSE route does not change `/api/state` or `/approvals/` (`internal/serve/handlers.go:79-85,87-146`). `Executor.Run` and `Outcome` stay backward-compatible; Request may gain optional writers.

## Test and Validation Impact

- **Executor:** Preserve grandchild/`WaitDelay` coverage (`internal/executor/agy_test.go:158-191`, `internal/executor/cursor_agent_test.go:178-205`, `internal/executor/opencode_test.go:178-205`); add MultiWriter-to-file-and-sink cases beside existing capture tests.
- **Serve:** Extend loopback tests (`internal/serve/server_test.go:17-40`) with SSE 200/`text/event-stream`, flush, disconnect cleanup, and `IsLoopback`.
- **Ledger:** If v6 is chosen later, extend `TestMigrateIsIdempotent` (`internal/ledger/ledger_test.go:733`); do not put stream chunks in `events`.
- **Model:** Keep `TestModelSourceDoesNotShellOut` (`internal/serve/model_test.go:595-627`) over any new event DTOs.
- **Run/barrier:** Extend `Execute`/`ExecuteBatch` and `TestEvaluate` (`internal/barrier/barrier.go:36-59`) so flush cannot release the barrier before terminal persist.
- **CLI:** `printReport` still prints diagnosis after the lane ends (`cmd/lucind-ai/cli.go:512-540`); add archive coverage only if archive is chosen.

## Out of Scope

- Patching `agy` / `cursor-agent` / `opencode` or requiring OTLP (`internal/executor/agy.go:140-158`, `internal/executor/cursor_agent.go:65-80`, `internal/executor/opencode.go:100-120`).
- Non-stdlib bundlers or WebSocket libraries in `internal/serve` (`internal/serve/server.go:1-53`, `internal/serve/handlers.go:1-85`).
- Remote bind, tokens, multi-tenant auth (`internal/serve/server.go:12-22,55-73`).
- Changing the six-value `lane.Status` (`internal/lane/status.go:10-17`).
- Parsing token usage from JSON stdout into Go result types (not present on `Outcome`, `internal/executor/executor.go:42-63`).

## Open Questions

- Archive worktree logs before removal? Beside `PersistEnvelope` as `.lucind/results/<lane-id>.log`, or under `.lucind/logs/<run-id>/`, before `RemoveLaneWorktree` (`cmd/lucind-ai/cli.go:641-646`) / `worktree cleanup` (`cmd/lucind-ai/cli.go:1460-1474`)? `PersistEnvelope` currently writes JSON only (`cmd/lucind-ai/cli.go:647-660`).
- SSE payload: raw stdout/stderr chunks, or a multiplexed JSON envelope (lane ID, stream, timestamp, payload)?
- Coarse milestones (turn index, elapsed): in-memory hub only, additive migration v6 in `migrate` (`internal/ledger/schema.go:224-306`), or (not recommended for lane streams) `integration_events` (`internal/ledger/schema.go:171-180`)?

## Success Criteria

- [ ] Every dispatched lane tees live stdout/stderr to a worktree log, and to SSE while serve is up, from `Execute` / `Executor.Run`.
- [ ] Exit code, timeout, truncation, and run events are queryable from `serve.Model` without `os/exec`.
- [ ] New routes stay loopback-only and do not change approval decide rules.
- [ ] Feature attempts still record phase transitions into `integration_events`.
- [ ] High-frequency chunks never depend on extending `events.type` without an explicit migration.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/executor/*` | Modified | MultiWriter sinks; keep WaitDelay. |
| `internal/run/run.go` | Modified | Hook at `exec.Run` (`:368-375`); optional log archive call. |
| `internal/serve/handlers.go`, `server.go` | Modified | SSE route + hub; loopback unchanged. |
| `internal/serve/model.go` | Modified | Event DTOs over `Ledger.Events`. |
| `internal/ledger/schema.go` | Unchanged unless later v6 | Open question only. |
| `cmd/lucind-ai/cli.go` | Modified | Optional archive next to `PersistEnvelope`. |
