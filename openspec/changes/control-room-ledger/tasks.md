# Tasks: Control Room Ledger

Single packet, no `apply-dag.yaml`. Sequential work-unit commits Unit 1 → 2 → 3. Units 2 and 3 are path-disjoint (`cmd/lucind-ai/` + `internal/run/` vs `internal/serve/`) under `packet.PathInScope` (`internal/packet/disjoint.go:8-22`) and would each pass `lucind-checks.sh` after Unit 1, but the combined CLI+DTO slice is too small to pay for sidecar Integrate.

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 1,200–1,600 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (schema v6 & ledger domain) → PR 2 (CLI lifecycle & lane metadata) → PR 3 (serve Model DTOs) |
| Delivery strategy | ask-on-risk |
| Chain strategy | feature-branch-chain |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

PR 1 base = feature/tracker; PR 2 base = PR 1; PR 3 base = PR 2.

## Suggested Work Units

| Unit | Goal | Likely PR | allowed_paths | Executor | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|---|---|
| 1 | Schema v6 + domain methods on `*Ledger` | PR 1 | `internal/ledger/schema.go`, `internal/ledger/ledger.go`, `internal/ledger/runs.go`, `internal/ledger/lanes_meta.go`, `internal/ledger/progress.go`, `internal/ledger/events.go`, `internal/ledger/ledger_test.go` | `agy` | `go test ./internal/ledger -count=1` | N/A: `Open` on temp dirs in `ledger_test.go` | Revert restores schema v5 and removes new `*Ledger` methods |
| 2 | CLI run lifecycle + `RegisterLane` metadata | PR 2 | `cmd/lucind-ai/cli.go`, `cmd/lucind-ai/cli_test.go`, `internal/run/run.go`, `internal/run/batch.go`, `internal/run/run_test.go` | `cursor-agent` | `go test ./cmd/lucind-ai ./internal/run -count=1` | `lucind-ai run --packet …` then a `runs` row exists | Revert restores untracked batch dispatch |
| 3 | Shell-free run/progress methods on `serve.Model` | PR 3 | `internal/serve/model.go`, `internal/serve/model_test.go` | `cursor-agent` | `go test ./internal/serve -count=1` | N/A: HTTP/UI is `control-room-serve` / `control-room-ui-views` | Revert restores feature-parent DTOs only |

Wave 1 = Unit 1 (green alone: additive DDL and methods; existing `RegisterLane` callers compile with zero-value `Model`/`Agent`/`Feature`). Wave 2 = Unit 2 then Unit 3 (each green alone after Wave 1; `NewHandler` signature at `cmd/lucind-ai/cli.go:715` unchanged). Same-wave pair only arises if Unit 2∥3; prefixes do not overlap.

## Phase 1: Schema v6 & modular ledger stores

- [ ] 1.1-RED Write `TestMigrateV5ToV6Database` in `internal/ledger/ledger_test.go` (shape of `TestMigrateV4DatabaseAllowsOpencodeExecutorAndPreservesRows` `:934`): `runs` and `lane_progress` exist; `lanes`/`events` rebuilt; v5 rows preserved; second `Open` is a no-op. Must fail: `schemaVersion` is 5 (`internal/ledger/schema.go:10`).
- [ ] 1.1 GREEN Bump `schemaVersion` to 6. Add `migrateV5ToV6DDL` (`runs` and `lane_progress` STRICT; `lanes` copy-drop-rename adding nullable `model`,`agent`,`feature`; `events.type` CHECK adds `'run_status_changed'`). Gate with `currentVersion < 6` after the v5 step (`schema.go:293-307`). Analog: `migrateV4ToV5DDL` `:190-219`.
- [ ] 1.2-RED Write `TestRegisterLanePersistsMetadata`: `RegisterLane` stores packet `model`,`agent`,`feature`; v5 rows migrate null/empty. Must fail: `Lane` has no those fields (`internal/ledger/ledger.go:233-245`); INSERT omits them (`:269-276`).
- [ ] 1.2 GREEN Add `Model`,`Agent`,`Feature string` on `Lane`. Keep `Open`/`Close`/WAL/`busy_timeout=5000`/`MaxOpenConns=4` in `ledger.go` (`:127-130,:146-191,:162-184,:217-218`).
- [ ] 1.3-RED Write `TestRegisterAndGetRun` and `TestUpdateRunStatus`: insert `running` + UTC `started_at`; duplicate `run_id` rejected; terminal status + non-null `ended_at`. Must fail: no `runs` API.
- [ ] 1.3 GREEN Create `internal/ledger/runs.go`: `Run` plus `RegisterRun`,`UpdateRunStatus`,`GetRun`,`ListRuns` on `*Ledger`.
- [ ] 1.4 GREEN Create `internal/ledger/lanes_meta.go`: move `RegisterLane`,`Lanes`,`LaneStates` from `ledger.go`; INSERT persists `model`,`agent`,`feature`.
- [ ] 1.5-RED Write `TestAppendProgressAndCursorTail` (`seq > afterSeq` ascending; duplicate seq rejected; empty on exact cursor; isolated from `events`) and `TestPruneProgressIsolated` (`DELETE FROM lane_progress WHERE at < ?`; analog `TestPruneIntegrationEventsRetention` `:1584`). Write `TestConcurrentProgressAndSetStatus` (pool of 4; analog `TestConcurrentRegisterAndSetStatusAcrossDistinctLanes` `:367`); no unhandled `SQLITE_BUSY`.
- [ ] 1.5 GREEN Create `internal/ledger/progress.go`: `LaneProgress`,`AppendProgress`,`GetProgressAfter`,`PruneProgress`.
- [ ] 1.6-RED Write `TestEventsAdmitRunStatusChanged`: CHECK admits `run_status_changed` and rejects unknown literals. Must fail: six literals at `schema.go:38-39` and `Event*` consts at `ledger.go:439-446`.
- [ ] 1.6 GREEN Create `internal/ledger/events.go`: `EventRunStatusChanged = "run_status_changed"`; move `AppendEvent`,`Events`, event helpers from `ledger.go` (`:366-381,:488-525`).

## Phase 2: CLI lifecycle & lane metadata

- [ ] 2.2-RED Write `TestRunRefusesLinkedWorktree` and `TestServeRefusesLinkedWorktree` in `cmd/lucind-ai/cli_test.go`: exit 1; stderr contains `refusing to run from inside a linked worktree`; return before `ledger.Open`. Threat-matrix row Git repository selection. Production already exists (`cli.go:277-280,:702-705`); tests pin it. Do not rewrite the guard. `TestResolve` already covers path math (`internal/ledgerpath/ledgerpath_test.go:9`).
- [ ] 2.2 GREEN Leave the linked-worktree returns at `cli.go:277-280,:702-705` intact.
- [ ] 2.1-RED Write `TestRunLifecycleRegistration`: after mint (`cli.go:282-290`) a `runs` row is `running` with UTC `started_at`; after `ExecuteBatch` returns (`:304-311`) status is terminal with non-null `ended_at`. Inject via `depsFactory` (`cli.go:58-60`; tests at `cli_test.go:1074`).
- [ ] 2.1 GREEN Call `RegisterRun` after UUID+`Open`; call `UpdateRunStatus` on both `ExecuteBatch` error and success returns.
- [ ] 2.3 GREEN In `internal/run/run.go:327-335`, pass `Model: p.Model`, `Agent: p.Agent`, `Feature: p.Feature` (`internal/packet/packet.go:43-64`).
- [ ] 2.4 GREEN Same fields on the never-started `RegisterLane` in `internal/run/batch.go:184-191`.

## Phase 3: Serve read-model DTOs

- [ ] 3.1-RED Write `TestModelRunAndProgressQueries`: typed run-summary and progress-tail results from SQLite; unknown `run_id` is not-found or empty; DB errors do not shell out. Existing `TestModelSourceDoesNotShellOut` (`internal/serve/model_test.go:595`) must still pass.
- [ ] 3.1 GREEN Add typed run-summary and progress-tail structs and query methods on `serve.Model` (`internal/serve/model.go:14-25`) via `ledger.DB()` (`internal/ledger/ledger.go:816-818`). Analog `ListFeatures` `:128-149`. No `os/exec`, no git. Do not change `NewHandler` or `serveStateJSON` (`internal/serve/handlers.go:79-85,:120-146`). Method identifiers are unset in `design.md`.

## Dependency order

| Task | Depends on | Why |
|---|---|---|
| 1.1 | — | DDL has no prerequisites |
| 1.2 | 1.1 | `Lane` fields match v6 columns |
| 1.3 | 1.1 | `runs` table |
| 1.4 | 1.1, 1.2 | INSERT uses v6 columns and `Lane` |
| 1.5 | 1.1 | SQL against `lane_progress` |
| 1.6 | 1.1 | widened `events` CHECK |
| 2.2 | — | already in `cli.go`; independent of schema |
| 2.1 | 1.3 | needs `RegisterRun`/`UpdateRunStatus` |
| 2.3, 2.4 | 1.2, 1.4 | needs `Lane` fields and `RegisterLane` |
| 3.1 | 1.3, 1.5 | queries `runs` and `lane_progress` |

Phase 1 before 2 and 3. 2 and 3 may proceed after 1; this packet runs them sequentially. RED and GREEN for one unit stay in that unit so Integrate is not left red.

## Requirement traceability

| Requirement | Tasks |
|---|---|
| First-Class Run Persistence (`specs/run-lifecycle-ledger/spec.md`) | 1.1, 1.3, 2.1 |
| Primary-Root Isolation Preservation (`specs/run-lifecycle-ledger/spec.md`) | 2.2 |
| Progress Ingest and Cursor Tail (`specs/lane-progress-stream/spec.md`) | 1.1, 1.5 |
| Isolated Progress Cutoff Pruning (`specs/progress-stream-pruning/spec.md`) | 1.5 |
| Lane Dispatch Metadata Persistence (`specs/lane-execution/spec.md`) | 1.1, 1.2, 1.4, 2.3, 2.4 |
| Admitted Run Status Event Types (`specs/lane-execution/spec.md`) | 1.1, 1.6 |
| Shell-Free Run and Progress Model DTOs (`specs/approvals-web-ui/spec.md`) | 3.1 |

## Open questions

- [ ] Should `lane_progress` pruning run as a `lucind-ai serve` ticker (`cli.go:674-725`) or an on-demand CLI command? Cutoff?
- [ ] Should `lane_progress.message` be raw text or structured JSON (stdout / stderr / control)?
