# Tasks Lens C — Orphan-Lane Reconciliation: Lane Status Observability

## Assumed decomposition

This slice decomposes orphan-lane reconciliation into two sequential phases: Phase C1 delivers runner PID persistence on `ledger.Run` and CLI PID registration during run dispatch; Phase C2 delivers the serve orphan sweeper, Linux PID-liveness probing, and lane failure reconciliation with audit notes. The critical path requires schema v7's `runs.pid` column (owned by lens B) to land first, followed by ledger PID support and CLI capture in Phase C1, enabling Phase C2's sweeper to probe runner PIDs.

## Phase C1: Runner Process ID Persistence & Capture

- [ ] C1.1 [RED]: Add test cases to `internal/ledger/runs_test.go:12-43` asserting `Run.PID` is persisted by `RegisterRun`, retrieved by `GetRun`, listed by `ListRuns`, and scanned by `scanRun`.
- [ ] C1.2 [GREEN]: Add `PID int` to `Run` in `internal/ledger/runs.go:16-24` and update `RegisterRun` (`internal/ledger/runs.go:29-41`), `GetRun` (`internal/ledger/runs.go:63-76`), `ListRuns` (`internal/ledger/runs.go:80-101`), and `scanRun` (`internal/ledger/runs.go:165-188`) to insert, select, and scan `pid`.
- [ ] C1.3 [GREEN]: Modify `cmd/lucind-ai/cli.go:314-324` in `runDispatch` to pass `PID: os.Getpid()` on `ledger.Run` to `ledg.RegisterRun`.

## Phase C2: Serve Orphan Sweeper & Liveness Reconciliation

- [ ] C2.1 [RED]: Create `internal/serve/sweeper_test.go` with `TestSweeper_LivePIDRetained` asserting a running lane with a live PID (`err == nil`) remains in `running` status (`internal/run/run.go:355-358`).
- [ ] C2.2 [RED]: Add `TestSweeper_DeadPIDReconciled` in `internal/serve/sweeper_test.go` asserting a running lane with a dead PID (`syscall.ESRCH`/`os.ErrProcessDone`) transitions to `failed` (`internal/lane/status.go:11-17`) via `SetStatus` (`internal/ledger/ledger.go:452-484`) and appends `EventLaneNote` ("orphaned: driving process no longer running") via `AppendEvent` (`internal/ledger/ledger.go:366-378`).
- [ ] C2.3 [RED]: Add `TestSweeper_ZeroPIDIgnored` in `internal/serve/sweeper_test.go` asserting `pid <= 0` skips liveness probing and leaves `running` lanes unchanged.
- [ ] C2.4 [RED]: Add `TestSweeper_RecycledPIDAndEPERM` in `internal/serve/sweeper_test.go` asserting `syscall.EPERM` treats the process as alive and active recycled PIDs are retained without failure transitions.
- [ ] C2.5 [GREEN]: Create `internal/serve/sweeper.go` implementing `Sweeper`, `NewSweeper`, `SweeperConfig` (10s interval), Linux liveness probing via `os.FindProcess(pid).Signal(syscall.Signal(0))`, dead-process transition to `lane.Failed` with `EventLaneNote` (`internal/ledger/ledger.go:440-446`), and loop ticker in `Sweeper.Run(ctx)` mirroring `Hub.Run` (`internal/serve/hub.go:213-235`).
- [ ] C2.6 [GREEN]: Modify `cmd/lucind-ai/cli.go:770-774` in `serveDispatch` to instantiate `Sweeper` and launch `go func() { _ = sweeper.Run(ctx) }()` beside `Hub`.

## Dependency Order (this slice)

| Task | Depends on | Why |
|---|---|---|
| C1.2 | Lens B: Schema v7 migration (`runs.pid` DDL) | SQLite STRICT `runs` table must have `pid INTEGER NOT NULL DEFAULT 0 CHECK (pid >= 0)` before Go queries compile and run. |
| C1.1 | C1.2 | RED test for `Run.PID` requires `Run.PID` field on the struct to compile. |
| C1.3 | C1.2 | CLI `runDispatch` cannot set `PID: os.Getpid()` until `ledger.Run` exposes `PID`. |
| C2.1 | C1.2 | Live PID test requires `runs` table to persist runner PID. |
| C2.2 | C1.2 | Dead PID test requires `runs` table to persist runner PID. |
| C2.3 | C1.2 | Zero PID test requires `runs` table to persist runner PID. |
| C2.4 | C1.2 | EPERM and recycled PID test requires `runs` table to persist runner PID. |
| C2.5 | C2.1, C2.2, C2.3, C2.4, C1.2 | Sweeper implementation satisfies RED tests and requires `ledger.Run` PID. |
| C2.6 | C2.5 | CLI `serveDispatch` cannot launch sweeper until `serve.Sweeper` is implemented. |

## Suggested Work Unit

| Unit | Goal | allowed_paths | Executor | Rollback boundary |
|---|---|---|---|---|
| Unit C (Orphan-Lane Reconciliation) | Record runner PID on registration and sweep dead-PID running lanes to failed with notes in serve | `cmd/lucind-ai/cli.go`, `internal/ledger/runs.go`, `internal/ledger/runs_test.go`, `internal/serve/sweeper.go`, `internal/serve/sweeper_test.go` | lucind-ai | Revert Go changes in `cmd/lucind-ai/cli.go`, `internal/ledger/runs.go`, `internal/serve/sweeper.go`; schema v7 columns remain additive with safe defaults |

## RED Tests from the Threat Matrix (this slice)

| Threat row | Applicable | RED test | Asserts | Production task it precedes |
|---|---|---|---|---|
| Process integration | Applicable | `TestSweeper_LivePIDRetained` | Probing living process PID (`err == nil`) leaves associated `running` lanes untouched. | C2.5 |
| Process integration | Applicable | `TestSweeper_DeadPIDReconciled` | Probing dead process PID (`syscall.ESRCH` or `os.ErrProcessDone`) transitions `running` lanes to `failed` and appends `EventLaneNote`. | C2.5 |
| Process integration | Applicable | `TestSweeper_ZeroPIDIgnored` | Probing `pid <= 0` skips liveness probe and leaves `running` lanes untouched. | C2.5 |
| Process integration | Applicable | `TestSweeper_RecycledPIDAndEPERM` | Probing PID returning `syscall.EPERM` treats process as alive; active recycled PID retains lane until death. | C2.5 |

## Acceptance Evidence

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|
| C1.1, C1.2 | `go test -run 'TestRegisterAndGetRun' ./internal/ledger` | `Run.PID` is persisted and retrieved via `RegisterRun`, `GetRun`, `ListRuns`, and `scanRun`. | Does not prove CLI captures active OS process PID. |
| C1.3 | `go test -run 'TestRunDispatch' ./cmd/lucind-ai` | `runDispatch` records `os.Getpid()` upon run registration. | Does not prove background sweeper runs or cleans lanes. |
| C2.1, C2.2, C2.3, C2.4, C2.5 | `go test -run 'TestSweeper_' ./internal/serve` | Sweeper retains live/zero PIDs, handles EPERM/recycled PIDs, marks dead-PID lanes `failed`, and appends note. | Does not prove serve CLI background goroutine startup. |
| C2.6 | `go test -run 'TestServeDispatch' ./cmd/lucind-ai` | `serveDispatch` starts background sweeper goroutine alongside Hub. | Does not prove cross-process resilience across host restarts. |

## Requirement Traceability

| Requirement | Tasks |
|---|---|
| Orphaned lane reconciliation | C1.1, C1.2, C1.3, C2.1, C2.2, C2.3, C2.4, C2.5, C2.6 |
| Scenario: Dead-process lane swept to failed | C2.2, C2.5 |
| Scenario: Active process lanes unchanged | C2.1, C2.3, C2.4, C2.5 |

## Open Questions

- [ ] Cross-lens dependency: Phase C1 (`internal/ledger/runs.go:16-24,29-41`) depends on Lens B's schema v7 migration DDL landing first so SQLite table `runs` carries `pid`.
- [ ] Shared-file overlap: `cmd/lucind-ai/cli.go` is modified by both Lens C (`runDispatch` PID at `cli.go:314-324`, `serveDispatch` sweeper at `cli.go:770-774`) and Lens A (`Packet.Path` at `cli.go:160-174`); synthesizer should sequence or merge cleanly.

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:160-174` | CLI packet flag parsing where Lens A adds Packet.Path |
| `cmd/lucind-ai/cli.go:314-324` | runDispatch registers run with started_at and status where PID will be added |
| `cmd/lucind-ai/cli.go:770-774` | serveDispatch initializes and launches Hub in background goroutine where Sweeper will be launched |
| `internal/lane/status.go:11-17` | Lane status constants defining Running and Failed |
| `internal/lane/status.go:31-38` | Valid method verifying permissible lane statuses |
| `internal/ledger/ledger.go:366-378` | AppendEvent inserts standalone events such as EventLaneNote into events table |
| `internal/ledger/ledger.go:440-446` | Event type constants including EventLaneStatusChanged and EventLaneNote |
| `internal/ledger/ledger.go:452-484` | SetStatus updates lane status and inserts lane_status_changed event in one transaction |
| `internal/ledger/runs.go:16-24` | Run struct defining run attributes where PID field is added |
| `internal/ledger/runs.go:29-41` | RegisterRun executes SQL insert into runs table |
| `internal/ledger/runs.go:63-76` | GetRun queries run by run_id from runs table |
| `internal/ledger/runs.go:80-101` | ListRuns queries all runs ordered newest first from runs table |
| `internal/ledger/runs.go:165-188` | scanRun scans SQL row into Run struct |
| `internal/ledger/runs_test.go:12-43` | TestRegisterAndGetRun tests run insertion and retrieval |
| `internal/run/run.go:334-344` | Execute calls RegisterLane to register lane in ledger |
| `internal/run/run.go:355-358` | Execute sets lane status to Running before execution starts |
| `internal/serve/hub.go:24` | defaultPollInterval defines standard polling duration |
| `internal/serve/hub.go:213-235` | Hub.Run executes immediate poll pass followed by ticker loop on context |
