# Tasks Lens A — Decomposition & Ordering: Control Room Ledger

## Assumed decomposition

Assumed 3-phase decomposition: Phase 1 delivers transactional schema v6 migration and modular ledger domain store files (`schema.go`, `ledger.go`, `runs.go`, `lanes_meta.go`, `progress.go`, `events.go`); Phase 2 integrates CLI dispatch run lifecycle tracking, linked worktree refusal, and lane metadata persistence (`cli.go`, `run.go`, `batch.go`); Phase 3 adds shell-free SQLite query DTOs to `internal/serve/model.go`. The critical path runs Phase 1 → Phase 2 (schema and domain store methods unblock CLI and runtime execution writes), while Phase 3 (serve read model) can proceed in parallel with Phase 2 once Phase 1 lands.

## Phase 1: Schema v6 & Modular Ledger Stores

- [ ] 1.1 `internal/ledger/schema.go:10,240-308`: Bump `schemaVersion` to 6, define `migrateV5ToV6DDL` (`runs` and `lane_progress` STRICT tables, `lanes` copy-drop-rename adding nullable `model`, `agent`, `feature`, and `events.type` CHECK widening with `'run_status_changed'`), and add `currentVersion < 6` migration step.
- [ ] 1.2 `internal/ledger/ledger.go:230-330`: Update `Lane` struct with nullable `Model`, `Agent`, `Feature string` fields; retain `Open`, `Close`, connection pool, and WAL pragma configuration while delegating domain queries to modular files.
- [ ] 1.3 `internal/ledger/runs.go` (new file): Define `Run` struct and implement `RegisterRun`, `UpdateRunStatus`, `GetRun`, and `ListRuns` methods on `*Ledger`.
- [ ] 1.4 `internal/ledger/lanes_meta.go` (new file): Extract `RegisterLane`, `Lanes`, and `LaneStates` from `ledger.go`, updating `RegisterLane` INSERT to persist packet `model`, `agent`, and `feature` columns.
- [ ] 1.5 `internal/ledger/progress.go` (new file): Define `LaneProgress` struct and implement `AppendProgress`, `GetProgressAfter` (`seq > afterSeq ORDER BY seq ASC`), and `PruneProgress` (`DELETE FROM lane_progress WHERE at < ?`) on `*Ledger`.
- [ ] 1.6 `internal/ledger/events.go` (new file): Define `EventRunStatusChanged = "run_status_changed"` constant, and extract `AppendEvent`, `Events`, and event helper methods from `ledger.go`.

## Phase 2: CLI Lifecycle & Runtime Metadata Integration

- [ ] 2.1 `cmd/lucind-ai/cli.go:282-311`: Add `RegisterRun` call with status `running` and UTC `started_at` after run UUID minting and `ledger.Open`, and add `UpdateRunStatus` call to terminal status with non-null `ended_at` after `lucindrun.ExecuteBatch` returns.
- [ ] 2.2 `cmd/lucind-ai/cli.go:277-280,702-705`: Enforce linked worktree refusal exit code 1 with stderr diagnostic before `ledger.Open` in `run` and `serve` subcommands to preserve primary-root isolation.
- [ ] 2.3 `internal/run/run.go:327-335`: Update `RegisterLane` call in `Execute` to pass `Model: p.Model`, `Agent: p.Agent`, and `Feature: p.Feature` from `packet.Packet`.
- [ ] 2.4 `internal/run/batch.go:184-191`: Update `RegisterLane` call in never-started lane handling to pass `Model: p.Model`, `Agent: p.Agent`, and `Feature: p.Feature` from `packet.Packet`.

## Phase 3: Serve Read Model DTOs

- [ ] 3.1 `internal/serve/model.go:14-25`: Add typed DTO structs for run summaries and progress chunks, and implement shell-free query methods on `serve.Model` using `ledger.DB()` without subprocess or git invocations.

## Dependency Order

| Task | Depends on | Why |
|---|---|---|
| 1.1 | — | Foundational schema DDL; has no prerequisites. |
| 1.2 | 1.1 | `Lane` struct changes align with schema v6 nullable columns. |
| 1.3 | 1.1 | `Run` persistence methods require the `runs` table defined in schema v6. |
| 1.4 | 1.1, 1.2 | `RegisterLane` persists schema v6 columns and requires the updated `Lane` struct. |
| 1.5 | 1.1 | Progress append, cursor tail, and prune execute SQL against schema v6 `lane_progress` table. |
| 1.6 | 1.1 | `EventRunStatusChanged` constant and `AppendEvent` require the widened `events` CHECK constraint. |
| 2.1 | 1.3 | CLI run lifecycle registration and status update calls cannot compile without `RegisterRun` and `UpdateRunStatus` on `*Ledger`. |
| 2.2 | — | Validates worktree isolation before ledger initialization; operates independently of schema changes. |
| 2.3 | 1.2, 1.4 | Passing packet metadata fields to `RegisterLane` requires the updated `Lane` struct and `RegisterLane` implementation. |
| 2.4 | 1.2, 1.4 | Passing packet metadata fields to `RegisterLane` requires the updated `Lane` struct and `RegisterLane` implementation. |
| 3.1 | 1.3, 1.5 | `serve.Model` DTO methods query `runs` and `lane_progress` tables and depend on ledger store method contracts. |

## Requirement Traceability

| Requirement | Tasks |
|---|---|
| `First-Class Run Persistence` (`specs/run-lifecycle-ledger/spec.md:9-30`) | 1.1, 1.3, 2.1 |
| `Primary-Root Isolation Preservation` (`specs/run-lifecycle-ledger/spec.md:31-52`) | 2.2 |
| `Progress Ingest and Cursor Tail` (`specs/lane-progress-stream/spec.md:9-30`) | 1.1, 1.5 |
| `Isolated Progress Cutoff Pruning` (`specs/progress-stream-pruning/spec.md:9-31`) | 1.5 |
| `Lane Dispatch Metadata Persistence` (`specs/lane-execution/spec.md:6-20`) | 1.1, 1.2, 1.4, 2.3, 2.4 |
| `Admitted Run Status Event Types` (`specs/lane-execution/spec.md:21-30`) | 1.1, 1.6 |
| `Shell-Free Run and Progress Model DTOs` (`specs/approvals-web-ui/spec.md:6-26`) | 3.1 |

## Open Questions

- [ ] Should `lane_progress` pruning trigger via a background ticker in `lucind-ai serve` (`cmd/lucind-ai/cli.go:674-725`) or an on-demand CLI command?
- [ ] Should `lane_progress.message` be constrained to structured JSON or remain raw string data?
- [ ] Contract note: `~/.claude/skills/sdd-tasks/SKILL.md` specifies full single-agent `tasks.md` generation, which this 3-lens parallel decomposition workflow intentionally supersedes for this lane.
