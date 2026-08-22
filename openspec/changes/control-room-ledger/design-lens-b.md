# Design Lens B — Surface & Flow: Control Room Ledger

## Assumed architecture

We assume Candidate 1 (relational schema expansion with modular domain files). Schema version advances from 5 (`internal/ledger/schema.go:10`) to 6 via transactional `migrate` (`internal/ledger/schema.go:221-307`), creating `runs` and `lane_progress` tables, rebuilding `lanes` with nullable `model`, `agent`, `feature` columns, and widening `events.type` CHECK for `run_status_changed`. `internal/ledger/` methods split into domain files (`runs.go`, `lanes_meta.go`, `progress.go`, `events.go`) sharing `*Ledger` (`internal/ledger/ledger.go:131-135`) for disjoint apply paths (`internal/packet/disjoint.go:29-48`). `serve.Model` (`internal/serve/model.go:14-25`) adds shell-free DTOs querying SQLite directly, while `cmd/lucind-ai/cli.go:282-290` registers runs at dispatch preserving WAL mode, `busy_timeout=5000`, `MaxOpenConns=4` (`internal/ledger/ledger.go:162-185`), and primary-root isolation (`internal/ledgerpath/ledgerpath.go:36-38`).

## Flow and Invariants

```
[CLI / Dispatcher] ──(1. RegisterRun)──→ [SQLite: runs]
        │
        ├──(2. RegisterLane w/ Model,Agent,Feature)──→ [SQLite: lanes]
        │
        ├──(3. AppendProgress chunks)──→ [SQLite: lane_progress] ──(5. PruneProgress)──→ [Retention Prune]
        │                                       │
        └──(4. SetStatus / AppendEvent)         └──(6. GetProgressAfter cursor)
                     │                                         │
                     ▼                                         ▼
            [SQLite: events] ────────────────────────→ [serve.Model DTOs] ──→ [/api/state / UI]
```

- **Hop 1 (CLI -> `runs`)**: CLI minting `runID` (`cmd/lucind-ai/cli.go:282-290`) inserts a `runs` row (`status='running'`, UTC `started_at`) before lane execution begins. *Breaks if violated*: Control Room cannot discover active run; lanes lack parent run provenance.
- **Hop 2 (Packet Frontmatter -> `lanes`)**: `RegisterLane` (`internal/ledger/ledger.go:255-282`) persists `model`, `agent`, `feature` from `packet.Packet` (`internal/packet/packet.go:43-64`) / `dag.Node` (`internal/dag/parse.go:26-28`). *Breaks if violated*: Lane telemetry loses model/agent attribution and feature context.
- **Hop 3 (Stream Ingest -> `lane_progress`)**: Workers append chunks via `AppendProgress` with monotonic `seq` without blocking lease renewals (`internal/run/attempt.go:434-441`). *Breaks if violated*: `SQLITE_BUSY` contention causes lease renewal timeouts and attempt failures.
- **Hop 4 (Lane Completion -> `runs`)**: When lanes reach terminal status, `runs.status` transitions to terminal with non-null `ended_at` (`internal/run/run.go:480-483`). *Breaks if violated*: Run summaries display eternal `running` status.
- **Hop 5 (Prune -> Storage)**: `PruneProgress(cutoff)` deletes only `lane_progress` rows where `at < cutoff` (`internal/ledger/ledger.go:877-890`), keeping `runs`, `lanes`, `events`, `approvals` intact. *Breaks if violated*: Audit trails or run/lane records are deleted during cleanup.
- **Hop 6 (Cursor Tail -> `serve.Model`)**: `GetProgressAfter(runID, laneID, afterSeq)` returns ascending chunks (`seq > afterSeq`) directly from SQLite without `os/exec` / Git subprocesses (`internal/serve/model_test.go:595`). *Breaks if violated*: UI streaming duplicates/drops chunks during 2s polls (`internal/serve/static/app.js:96-97`) or fails AST checks.

## Surface Deltas

| Symbol or format | Today (file:line) | Delta | Backward compatible? |
|---|---|---|---|
| `const schemaVersion` | `internal/ledger/schema.go:10` | Advance `5` to `6` | Yes; `migrate` skips lower versions on existing DBs. |
| Table `runs` | None (`internal/ledger/schema.go:17-57`) | Add `runs(run_id, feature_id, status, target_ref, lane_count, started_at, ended_at)` STRICT | Yes; additive table. |
| Table `lane_progress` | None (`internal/ledger/schema.go:17-57`) | Add `lane_progress(run_id, lane_id, seq, message, at)` STRICT; index on `(run_id, lane_id, seq)` | Yes; additive table. |
| Table `lanes` | `internal/ledger/schema.go:18-32` | Rebuild via copy-drop-rename adding nullable `model`, `agent`, `feature` columns | Yes; named SELECTs (`internal/ledger/ledger.go:287-289`) ignore new columns. |
| Table `events` | `internal/ledger/schema.go:34-43` | Rebuild via copy-drop-rename widening `CHECK(type IN (...))` to include `'run_status_changed'` | Yes; superset enum preserves existing event rows. |
| `type Lane` | `internal/ledger/ledger.go:233-245` | Add fields `Model string`, `Agent string`, `Feature string` | Yes; optional fields. |
| `type Run` | None (`internal/ledger/ledger.go:233`) | Add struct `Run` (`RunID`, `FeatureID`, `Status`, `TargetRef`, `LaneCount`, `StartedAt`, `EndedAt`) | Yes; additive Go struct. |
| `type LaneProgress` | None (`internal/ledger/ledger.go:429`) | Add struct `LaneProgress` (`RunID`, `LaneID`, `Seq`, `Message`, `At`) | Yes; additive Go struct. |
| `const EventRunStatusChanged` | None (`internal/ledger/ledger.go:429`) | Add `EventRunStatusChanged = "run_status_changed"` | Yes; additive constant. |
| `*Ledger` run methods | None (`internal/ledger/ledger.go:255`) | Add `RegisterRun`, `UpdateRunStatus`, `GetRun`, `ListRuns` | Yes; additive methods on `*Ledger`. |
| `*Ledger` progress methods | None (`internal/ledger/ledger.go:420`) | Add `AppendProgress`, `GetProgressAfter`, `PruneProgress` | Yes; additive methods on `*Ledger`. |
| `serve.Model` DTOs | None (`internal/serve/model.go:27-125`) | Add `RunSummary` and `ProgressChunk` structs | Yes; additive DTO structs. |
| `serve.Model` query methods | None (`internal/serve/model.go:128-344`) | Add `GetRunSummary`, `ListRunSummaries`, `GetLaneProgress` | Yes; additive query methods on `*Model`. |
| CLI `lucind-ai run` dispatch | `cmd/lucind-ai/cli.go:282-311` | Register run at dispatch after UUID mint and update status on batch finish | Yes; CLI flags/syntax unchanged. |

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `internal/ledger/schema.go` | Modify | Advance `schemaVersion` to 6, add `migrateV5ToV6DDL` for `runs`, `lane_progress`, and STRICT rebuilds for `lanes`/`events`. | `ledger.Open` (`internal/ledger/ledger.go:146-148`) / `migrate` (`internal/ledger/schema.go:224-307`) |
| `internal/ledger/ledger.go` | Modify | Update `Lane` struct with `Model`, `Agent`, `Feature`; retain connection lifecycle (`Open`, `Close`, pragmas, pool). | `lucindrun.ExecuteBatch` (`cmd/lucind-ai/cli.go:304`) |
| `internal/ledger/runs.go` | Create | Define `Run` struct and implement `RegisterRun`, `UpdateRunStatus`, `GetRun`, `ListRuns` on `*Ledger`. | CLI run lifecycle (`cmd/lucind-ai/cli.go:282-290`) and `serve.Model.GetRunSummary` (`internal/serve/model.go:128`) |
| `internal/ledger/lanes_meta.go` | Create | Extract lane operations (`RegisterLane`, `Lanes`, `LaneStates`) into modular domain file for disjoint apply paths. | DAG lane executor (`internal/run/run.go:390-483`) and `DisjointAllowedPaths` (`internal/packet/disjoint.go:29-48`) |
| `internal/ledger/progress.go` | Create | Define `LaneProgress` struct and implement `AppendProgress`, `GetProgressAfter`, `PruneProgress` on `*Ledger`. | `serve.Model.GetLaneProgress` (`internal/serve/model.go:14-25`) and output capture store (`internal/run/run.go:420-500`) |
| `internal/ledger/events.go` | Create | Extract event logging methods (`AppendEvent`, `Events`) and define `EventRunStatusChanged` constant. | Lane diagnostic logging (`internal/run/run.go:425-435`, `internal/run/run.go:488-499`) |
| `internal/serve/model.go` | Modify | Define `RunSummary` and `ProgressChunk` DTOs; implement `GetRunSummary`, `ListRunSummaries`, `GetLaneProgress`. | HTTP state handler (`internal/serve/handlers.go:79-85`) and Control Room UI pollers (`internal/serve/static/app.js:96-97`) |
| `cmd/lucind-ai/cli.go` | Modify | Invoke `RegisterRun` at dispatch after minting `runID` and record terminal status upon `ExecuteBatch` completion. | CLI operator command `lucind-ai run` (`cmd/lucind-ai/cli.go:282-311`) |

## Open Questions

- [ ] Should `lane_progress` retention pruning be triggered periodically by a background ticker in `lucind-ai serve` (`cmd/lucind-ai/cli.go:674-725`) or exposed strictly as an on-demand CLI maintenance command?
- [ ] Should `lane_progress.message` store raw text chunks or structured JSON distinguishing stdout, stderr, and system event markers?
- [ ] Execution-topology note: Proposal and design phases fan out across three parallel lenses (Lens A, Lens B, Lens C) feeding a synthesis lane, per packet authorization, superseding single-subagent monolithic `design.md` generation in `~/.claude/skills/sdd-design/SKILL.md`.
