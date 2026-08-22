# Proposal: Control Room Ledger

**Chosen candidate: Candidate 1 — Relational schema expansion with modular domain files.**

Advance schema version from 5 (`internal/ledger/schema.go:10`) to 6 through the existing transactional `migrate` (`internal/ledger/schema.go:221-307`). Persist runs, lane dispatch metadata, and sequenced progress in SQLite so `lucind-ai run` and `lucind-ai serve` — separate processes (`cmd/lucind-ai/cli.go:99-371`, `cmd/lucind-ai/cli.go:674-725`) — share one queryable store at `<primaryRoot>/.lucind/lucind.db`.

## Intent

Control Room needs a durable run/lane/progress read model. Today dispatch mints an ephemeral UUID (`cmd/lucind-ai/cli.go:282-290`) and never writes a `runs` row. Schema v5 has `lanes`, `events`, and `approvals` (`internal/ledger/schema.go:10-57`) but no `runs`; run rollups scan `Lanes` (`internal/ledger/ledger.go:285-330`) or unpaginated `Events` (`internal/ledger/ledger.go:490-525`). `RegisterLane` inserts identity and status only (`internal/ledger/ledger.go:269-276`) and drops `Model`, `Agent`, and `Feature` from `packet.Packet` / `dag.Node` (`internal/packet/packet.go:43-54,64`; `internal/dag/parse.go:26-28`). `events.type` admits six literals (`internal/ledger/schema.go:38-39`). Executors expose stdout/stderr on `Outcome` after the process ends (`internal/executor/executor.go:42-61`); completion writes diagnostic `lane_note` events (`internal/run/run.go:422-434,488-499`), not mid-flight chunks. `/api/state` returns pending approvals (`internal/serve/handlers.go:79-85`). The localhost UI polls every 2s (`internal/serve/static/app.js:96-97`).

## Scope

### In Scope
- Schema v6: `runs`; rebuild `lanes` with nullable `model`, `agent`, `feature`; indexed `lane_progress(run_id, lane_id, seq, message, at)`; widen `events.type` to admit `run_status_changed`.
- Hook persistence at existing call sites: persist a run after UUID mint (`cmd/lucind-ai/cli.go:282`); extend `RegisterLane` (`internal/ledger/ledger.go:255-282`); extend `migrate` (`internal/ledger/schema.go:221-307`).
- Split `internal/ledger/ledger.go` methods (`internal/ledger/ledger.go:131-192`) into `runs.go`, `lanes_meta.go`, `progress.go`, `events.go` sharing `*Ledger`, so apply-DAG `allowed_paths` can be disjoint (`internal/packet/disjoint.go:29-48`).
- Typed, shell-free DTOs on `serve.Model` (`internal/serve/model.go:14-25`) for run summaries and progress tails.
- Time-cutoff prune of `lane_progress` only, modeled on `PruneIntegrationEvents` (`internal/ledger/ledger.go:877-890`).

### Out of Scope
- Control Room HTML/CSS/JS (`control-room-ui-shell`, `control-room-ui-views`).
- HTTP/SSE/WebSocket listener lifecycle (`control-room-serve`).
- Child stdout/stderr piping (`control-room-capture`) — this change is the store those writers will use.
- External telemetry exporters (`control-room-telemetry`).
- gentle-ai review, RDD gates, packet/result schema changes (`internal/packet/packet.go:33-75`).
- Inventing packet/DAG fields that do not exist today.

## Capabilities

### New Capabilities
- `run-lifecycle-ledger`: durable `runs` rows with status and timestamps; run status is not derived solely by scanning `lanes`.
- `lane-progress-stream`: sequenced mid-flight chunks with cursor tail reads.
- `progress-stream-pruning`: delete expired `lane_progress` rows without touching `runs`, `lanes`, `approvals`, or `events`.

### Modified Capabilities
- `lane-execution`: persist `model`, `agent`, `feature` on `RegisterLane`; admit `run_status_changed` on `events`.
- `approvals-web-ui`: extend `serve.Model` so handlers can serve run summaries and progress tails without git/shell. Per-item decide, bulk-reject (`internal/serve/handlers.go:161-177`), and `ApproverRate` (`internal/ledger/ledger.go:797-814`) stay unchanged.

| Capability | Impact | Description | Existing seam |
|---|---|---|---|
| `run-lifecycle-ledger` | Added | Persist run rows at dispatch | `cmd/lucind-ai/cli.go:282-290`; `internal/ledger/schema.go:10-57` |
| `lane-progress-stream` | Added | Sequenced chunks, isolated from `events` | `internal/run/run.go:422-434,488-499`; `internal/ledger/ledger.go:162-185` |
| `progress-stream-pruning` | Added | Time-cutoff delete of progress only | `internal/ledger/ledger.go:877-890` |
| `lane-execution` | Modified | Lane metadata columns + wider event types | `internal/packet/packet.go:33-75`; `internal/ledger/ledger.go:255-282` |
| `approvals-web-ui` | Modified | Run/progress DTOs on `Model` | `internal/serve/model.go:14-25`; `internal/serve/handlers.go:79-85` |

## Approach

1. **`runs` table** (`run_id`, `feature_id`, `status`, `target_ref`, `lane_count`, `started_at`, `ended_at`) so lifecycle queries do not scan `lanes` or `events`.
2. **Rebuild `lanes`** via copy-drop-rename (`internal/ledger/schema.go:191-219`) adding nullable `model`, `agent`, `feature`.
3. **`lane_progress`** indexed by `(run_id, lane_id, seq)` so high-frequency appends stay out of `events` (`internal/ledger/schema.go:34-43`).
4. **Rebuild `events`** via the v1→v2 copy-drop-rename pattern (`internal/ledger/schema.go:59-78`) to widen the CHECK (`internal/ledger/schema.go:38-39`).
5. **Domain file split** for disjoint apply lanes.
6. **Keep** WAL, `busy_timeout=5000`, `MaxOpenConns=4` (`internal/ledger/ledger.go:127-130,162-184`); path via `ledgerpath.Resolve` (`internal/ledgerpath/ledgerpath.go:36-38`; `internal/ledger/ledger.go:146-148`); CLI refuse linked worktrees (`cmd/lucind-ai/cli.go:277-280,702-705`). `Open` still does not reject a worktree passed as `primaryRoot` (`internal/ledger/ledger.go:8-17`).

Tables stay `STRICT` (`internal/ledger/schema.go:32,42,56,129`). Lease fencing (`internal/ledger/schema.go:122-129`; `internal/run/attempt.go:482-488`) is untouched.

### Conceptual changes
- Run becomes a durable entity, not a CLI string.
- Lane registration keeps dispatch metadata.
- Progress stream is separate from audit `events` and `approvals` (`internal/ledger/schema.go:45-56`).
- Ledger operations are partitioned by domain file so parallel apply lanes do not share `ledger.go`.

SQLite is the cross-process seam (`internal/ledger/ledger.go:1-17`). `serve.Model` remains the shell-free DTO surface (`internal/serve/model.go:14-25`); `ListFeatures` (`internal/serve/model.go:128-149`) is the analog to extend, not a run API.

### Alternatives rejected
- **Candidate 2** (JSON `metadata_json` + progress in `events.detail` at `internal/ledger/schema.go:40`): still needs an `events` CHECK rebuild; no column indexes; `Events` (`internal/ledger/ledger.go:490-525`) becomes a stream dump.
- **Candidate 3** (in-memory progress in `serve`): `run` and `serve` do not share process memory; telemetry vanishes on restart or headless CLI; contradicts the durable-ledger contract (`internal/ledger/ledger.go:1-17`).

## Delta specifications

### First-class run persistence
The ledger MUST store a `runs` row at dispatch and update it when the batch finishes. Status MUST NOT be derived solely from `lanes`.

- GIVEN CLI minting `runID` (`cmd/lucind-ai/cli.go:282-290`), WHEN the run is registered, THEN a `runs` row exists with status `running` and UTC `started_at`.
- GIVEN a `running` row, WHEN all lanes are terminal, THEN the row has a terminal status and non-null `ended_at`.

### Lane dispatch metadata
Schema v6 MUST add nullable `model`, `agent`, `feature` on `lanes`. `RegisterLane` MUST persist them when present on `packet.Packet`.

- GIVEN a packet with `model` and `agent` (`internal/packet/packet.go:43-54`), WHEN `RegisterLane` runs (`internal/ledger/ledger.go:255-282`), THEN those values are stored.
- GIVEN a v5 database (`internal/ledger/schema.go:10,191-219`), WHEN `migrate` runs (`internal/ledger/schema.go:221-307`), THEN existing lane rows are preserved with null/empty new columns.

### Progress ingest and cursor tail
The ledger MUST append to `lane_progress` and return chunks with `seq > afterSeq` in ascending order. WAL and the pool (`internal/ledger/ledger.go:162-185`) are the concurrency seam; they do not by themselves prove zero writer wait.

- GIVEN chunks 1–10, WHEN the reader asks for sequences after 5, THEN it receives 6–10 ascending.

### Isolated progress pruning
A cutoff prune MUST delete only `lane_progress` rows older than `T`. Analog: `PruneIntegrationEvents` (`internal/ledger/ledger.go:877-890`).

- GIVEN old progress rows plus terminal lanes and approvals, WHEN prune runs at `T`, THEN only expired progress rows are gone.

### Shell-free run DTOs
`serve.Model` MUST grow typed run-summary and progress-tail methods that query SQLite only (`internal/serve/model.go:14-25`). `/api/state` today is approvals-only (`internal/serve/handlers.go:79-85`).

### Primary-root isolation (preserve)
`Open` MUST resolve via `ledgerpath.Resolve` (`internal/ledger/ledger.go:146-148`; `internal/ledgerpath/ledgerpath.go:36-38`). `run` and `serve` MUST still exit 1 inside a linked worktree (`cmd/lucind-ai/cli.go:277-280,702-705`).

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Progress appends contend with lease renew (`internal/run/attempt.go:434-441`) and `SetStatus` on single-writer SQLite | High | Keep WAL/`busy_timeout=5000`/pool of 4 (`internal/ledger/ledger.go:162-185`); isolate progress transactions from lease loops. Spike 1 must measure before locking high-Hz ingest into this writer. |
| STRICT rebuild of `lanes`/`events` corrupts rows | Med | One transaction (`internal/ledger/schema.go:221-229`); copy-drop-rename as in v4→v5 (`internal/ledger/schema.go:191-219`). |
| Unbounded scans of progress or `Events` (`internal/ledger/ledger.go:490-525`) under 2s UI polls (`internal/serve/static/app.js:96-97`) | High | Index `(run_id, lane_id, seq)`; cursor tail. Keep `idx_events_run` (`internal/ledger/schema.go:43`). |
| Ledger opened under a worktree | High | Keep CLI gates; `Open` still will not detect a worktree `primaryRoot` (`internal/ledger/ledger.go:8-17`). |
| Progress prune deletes governance rows | Med | `DELETE FROM lane_progress WHERE at < ?` only; no cascading FKs onto `approvals` (`internal/ledger/schema.go:45-56`). |
| Parallel apply lanes collide on `ledger.go` | Med | Split domain files (`internal/packet/disjoint.go:29-48`). |

## Rollback plan and additivity

Revert the Go commits under `internal/ledger/`, `internal/serve/`, and `cmd/lucind-ai/`. `migrate` only applies `currentVersion < N` steps (`internal/ledger/schema.go:240-304`); it does not reject a recorded version above the binary's `schemaVersion`. v5 readers that `SELECT` named `lanes` columns (`internal/ledger/ledger.go:287-289`) ignore extra nullable columns and unused tables. No downgrade script.

Changes are additive: new `runs` and `lane_progress`; nullable lane metadata via rebuild; wider `events.type` CHECK via rebuild. `serve.Model` adds DTOs without changing feature/lease structs (`internal/serve/model.go:14-25`). Packet parsing is unchanged (`internal/packet/packet.go:33-75`).

## Test and validation impact

| Layer | Coverage | Existing seam |
|---|---|---|
| v6 migrate + idempotency | Clean and v1–v5 fixtures; rows preserved; second `migrate` is a no-op | `internal/ledger/ledger_test.go:579-620,733-745,934-970`; `internal/ledger/schema.go:221-307` |
| Writer contention | Concurrent progress appends with `SetStatus` and lease renew, no unhandled `SQLITE_BUSY` | Analog: `internal/ledger/ledger_test.go:367`; `internal/run/attempt.go:434-441,482-488` |
| Cursor tail | `seq > afterSeq` ascending; empty on exact cursor | New tests; unpaginated analog `internal/ledger/ledger.go:490-525` |
| Prune | Deletes old `lane_progress` only | Analog: `internal/ledger/ledger.go:877-890`; `internal/ledger/ledger_test.go:1584` |
| Shell-free Model | No `os/exec` or git | `internal/serve/model.go:14-25`; `internal/serve/model_test.go:595` |
| Primary root | `Resolve` path; CLI exit 1 in linked worktree | `internal/ledgerpath/ledgerpath_test.go:9`; `cmd/lucind-ai/cli.go:277-280,702-705` (CLI worktree tests are not at `cli_test.go:210-250`) |
| Governance | `Decide`, bulk-reject, `ApproverRate` unchanged | `internal/ledger/ledger.go:614-640,643-661,797-814`; `internal/serve/handlers.go:161-177`; `internal/serve/server_test.go:42` |

## Affected areas

| Area | Impact | Description |
|---|---|---|
| `internal/ledger/schema.go` | Modified | v6 migration |
| `internal/ledger/ledger.go` | Modified | Methods; split into domain files |
| `internal/serve/model.go` | Modified | Run/progress DTOs |
| `cmd/lucind-ai/cli.go`, `internal/run/run.go` | Modified | Register run; keep notes; progress ingest is the store for capture |

## Dependencies

- Sibling Control Room changes consume this store; they are not implementation prerequisites.
- Spike 1 (WAL ingest vs lease renew) should run before locking high-Hz progress into this writer.

## Success criteria

- [ ] v6 `migrate` is idempotent and preserves `lanes`/`events`/`approvals`.
- [ ] Concurrent progress writes complete without unhandled `SQLITE_BUSY` under `MaxOpenConns=4` (`internal/ledger/ledger.go:182-184`).
- [ ] Run/progress reads are typed DTOs with no git/shell (`internal/serve/model.go:14-25`).
- [ ] Tail queries return only `seq > afterSeq`.
- [ ] Prune deletes expired progress only.
- [ ] Path remains `<primaryRoot>/.lucind/lucind.db`; CLI still refuses linked worktrees.
- [ ] Decide, bulk-reject, and `ApproverRate` unchanged.

## Open questions

- [ ] Auto-prune `lane_progress` like `PruneIntegrationEvents` (`internal/ledger/ledger.go:877-890`)? Cutoff? Trigger: periodic `lucind-ai serve` vs on-demand CLI?
- [ ] Emit run status via `AppendEvent` (`internal/ledger/ledger.go:366-381`) with `run_status_changed`, or dedicated transaction helpers?
- [ ] Store `message` as a raw string (Candidate 1 sketch) or structured JSON distinguishing stdout, stderr, and system events?
