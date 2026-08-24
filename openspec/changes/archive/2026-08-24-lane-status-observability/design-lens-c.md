# Design Lens C — Orphan-Lane Reconciliation: Lane Status Observability

## Assumed architecture

Lane Status Observability introduces structured telemetry, persistent lane metadata, packet markdown inspection, and orphan-lane reconciliation to lucind-ai. Schema v7 widens `runs` with `pid` and `lane_progress` with numeric usage fields in a single STRICT create-copy-drop-rename transaction. `lucind-ai run` dispatches record runner PID and lane metadata, while `serve` exposes packet endpoints and reconciles dead-process `running` lanes to `failed`.

## Decision 1 — PID capture point

**Choice**: Capture runner PID via `os.Getpid()` in `cmd/lucind-ai/cli.go:314-324` within `runDispatch`, passing `PID: os.Getpid()` on the `ledger.Run` struct (`internal/ledger/runs.go:16-24`) to `ledg.RegisterRun`.
**Alternatives considered**: Capturing child executor PIDs per-lane or capturing `serve` PID on server start.
**Rationale**: `lucind-ai run` (`runDispatch`) is the driving process that orchestrates lane lifecycles after `RegisterLane` (`internal/run/run.go:334-344`); if `runDispatch` crashes, all its active lanes are orphaned.
**Terminal consumer**: `internal/ledger/runs.go:29-41` (persisted in SQLite `runs` table), `internal/ledger/runs.go:63-76` (retrieved via `GetRun`), and `internal/ledger/runs.go:165-188` (scanned into `Run.PID`).

## Decision 2 — Sweep architecture (where it hooks into serveDispatch)

**Choice**: Define `serve.Sweeper` in `internal/serve/sweeper.go`, instantiated via `serve.NewSweeper(ledg, serve.SweeperConfig{Interval: 10 * time.Second})`. Hook into `cmd/lucind-ai/cli.go:770-774` in `serveDispatch` alongside `Hub`, launched as `go func() { _ = sweeper.Run(ctx) }()`. `Sweeper.Run(ctx)` runs an initial immediate sweep then polls on a ticker matching `Hub.Run` (`internal/serve/hub.go:213-235`), marking orphaned lanes `lane.Failed` (`internal/lane/status.go:11-17`, validated by `internal/lane/status.go:31-38`) and appending `EventLaneNote` (`internal/ledger/ledger.go:440-446`).
**Alternatives considered**: Sweeping synchronously in `lucind-ai run` or on-demand via HTTP.
**Rationale**: `serve` is the long-lived daemon monitoring the shared ledger. Sweeping in `serve` reconciles lanes if `runDispatch` terminates abruptly. An independent goroutine mirrors `Hub.Run` without sharing sub-second polling (`internal/serve/hub.go:24`).
**Terminal consumer**: `internal/ledger/ledger.go:452-484` (`SetStatus` updating `lanes.status`) and `internal/ledger/ledger.go:366-378` (`AppendEvent` recording `EventLaneNote`).

## Decision 3 — Ticker interval (resolves Open Question 3)

**Choice**: Exactly `10 * time.Second`.
**Alternatives considered**: `100ms` (unnecessary SQLite polling overhead) and `60s` (excessive UI status latency).
**Rationale**: 10 seconds provides prompt detection of dead runners on the dashboard with negligible SQLite query overhead.
**Terminal consumer**: `internal/serve/hub.go:213-235` (ticker loop in `Sweeper.Run`).

## Decision 4 — PID-liveness mechanism (resolves Open Question 4)

**Choice**: `os.FindProcess(pid).Signal(syscall.Signal(0))` (POSIX `kill(pid, 0)` probe). Linux-only deployment scope: alive when `err == nil` or `errors.Is(err, syscall.EPERM)`; dead when `errors.Is(err, syscall.ESRCH)` or `errors.Is(err, os.ErrProcessDone)`.
**Alternatives considered**: Direct `/proc/<pid>` directory checks and cross-platform process abstraction libraries.
**Rationale**: `kill(pid, 0)` is a standard syscall checking process existence without sending signals. Cross-platform portability is out of scope for this Linux-only deployment.
**Terminal consumer**: `internal/serve/hub.go:213-235` (probe execution in `Sweeper.Run`).

## Decision 5 — Historical/zero-PID rows

**Choice**: When `pid <= 0`, the sweeper skips liveness probing and leaves lane status unchanged in `running`.
**Rationale**: Pre-migration rows default to `pid = 0`. Zero means no PID recorded; skipping avoids false-positive failure transitions.
**Terminal consumer**: `internal/serve/hub.go:213-235` (guard check in `Sweeper.Run`).

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `cmd/lucind-ai/cli.go` | Modify | Pass `os.Getpid()` in `RegisterRun` in `runDispatch`; launch `Sweeper` in `serveDispatch`. | `internal/ledger/runs.go:29-41`, `cmd/lucind-ai/cli.go:770-774` |
| `internal/ledger/runs.go` | Modify | Add `PID int` to `Run`; update `RegisterRun`, `GetRun`, `ListRuns`, and `scanRun` for `pid`. | `internal/ledger/runs.go:29-41`, `internal/ledger/runs.go:165-188` |
| `internal/serve/sweeper.go` | Create | Implement `Sweeper`, `NewSweeper`, `Run(ctx)`, liveness probe, and orphan transition with `EventLaneNote`. | `internal/ledger/ledger.go:452-484`, `internal/ledger/ledger.go:366-378` |

## Testing Strategy (this slice)

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|
| Unit (`ledger`) | `RegisterRun` / `GetRun` persist `PID` | Insert `Run` with `PID`; assert `GetRun` and `ListRuns` scan matching `PID`. | `internal/ledger/runs.go:29-41` |
| Unit (`serve`) | Liveness probe logic | Probe self (alive), dead PID, and PID 0; assert alive/dead/skip outcomes. | `internal/serve/hub.go:213-235` |
| Integration (`serve`) | Orphan lane swept to `failed` | Seed `running` lane with dead PID; invoke sweeper; assert status `failed` and note appended. | `internal/ledger/ledger.go:452-484` |
| Integration (`serve`) | Active and zero-PID lanes untouched | Seed `running` lanes with current PID and PID 0; invoke sweeper; assert status stays `running`. | `internal/run/run.go:355-358` |

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | N/A: no path classification in sweeper | N/A | N/A |
| Git repository selection | `git -C`, relative paths, absolute paths | N/A: no git CLI repository selection | N/A | N/A |
| Commit state | staged, `commit -a`, empty index | N/A: no git commit creation or index interaction | N/A | N/A |
| Push state | tracking branch, first push, explicit refspec | N/A: no git push operations | N/A | N/A |
| PR commands | explicit `--head`, environment prefix, composed commands | N/A: no PR command synthesis | N/A | N/A |
| Process integration | Live PID, dead PID, PID 0, PID recycling, signal permission error (`EPERM`) | Applicable | Safe: `kill(pid, 0)` returns alive on `nil`/`EPERM`, dead on `ESRCH`/`ErrProcessDone`, skips `PID <= 0`. Failure: dead PID transitions lane to `failed` and appends note; unknown errors log without crashing. | `TestSweeper_LivePIDRetained`, `TestSweeper_DeadPIDReconciled`, `TestSweeper_ZeroPIDIgnored` |

## Rollback and Additivity (this slice)

**Choice**: Revert Go code changes; v7 schema remains backward compatible since queries explicitly select named columns.
**Rationale**: `runs.pid` defaults to 0 and is strictly additive. The sweeper uses standard `SetStatus` and `AppendEvent` interfaces.

## Open Questions

- [ ] None (skill 800-word budget and Engram persistence superseded by packet contract).

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:314-324` | runDispatch registers run with started_at and status, where PID will be added |
| `cmd/lucind-ai/cli.go:770-774` | serveDispatch initializes and launches Hub in background goroutine |
| `internal/lane/status.go:11-17` | Lane status constants defining Running and Failed |
| `internal/lane/status.go:31-38` | Valid method verifying permissible lane statuses |
| `internal/ledger/ledger.go:366-378` | AppendEvent inserts standalone events such as EventLaneNote into events table |
| `internal/ledger/ledger.go:440-446` | Event type constants including EventLaneStatusChanged and EventLaneNote |
| `internal/ledger/ledger.go:452-484` | SetStatus updates lane status and inserts lane_status_changed event in one transaction |
| `internal/ledger/runs.go:16-24` | Run struct defining run attributes |
| `internal/ledger/runs.go:29-41` | RegisterRun executes SQL insert into runs table |
| `internal/ledger/runs.go:63-76` | GetRun queries run by run_id from runs table |
| `internal/ledger/runs.go:165-188` | scanRun scans SQL row into Run struct |
| `internal/run/run.go:334-344` | Execute calls RegisterLane to register lane in ledger |
| `internal/run/run.go:355-358` | Execute sets lane status to Running before execution starts |
| `internal/serve/hub.go:24` | defaultPollInterval defines standard polling duration |
| `internal/serve/hub.go:213-235` | Hub.Run executes immediate poll pass followed by ticker loop on context |
