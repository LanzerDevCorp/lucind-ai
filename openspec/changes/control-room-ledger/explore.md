# Explore: Control Room Ledger

**Recommendation:** Candidate 1 — schema v6 relational expansion (`runs`, richer `lanes`, `lane_progress`) plus splitting `internal/ledger` methods across domain files. Candidate 3 (in-memory progress only) is not viable: `lucind-ai run` and `lucind-ai serve` are separate processes that share state only through SQLite. Candidate 2 (JSON blobs in `events.detail`) still needs an `events` rebuild and loses typed indexes.

**Ready for proposal:** Yes. Design must settle retention, whether chunks live in SQLite or files, and where a live pub/sub layer sits. Spike 1 should run before locking high-frequency ingest into the same database as leases.

## Problem statement and background

`lucind-ai` runs concurrent lanes in linked worktrees (`internal/run/batch.go:66-113`, `internal/worktree/worktree.go:150-171`). The localhost server is an approvals polling UI (`internal/serve/server.go:12-18`, `internal/serve/handlers.go:36-85`, `internal/serve/static/app.js:1-9,96-97`). Control Room needs the ledger to be a queryable run/lane/progress store. Six gaps:

1. **No `runs` table.** Dispatch mints an ephemeral UUID (`cmd/lucind-ai/cli.go:282-290`). Schema v5 (`internal/ledger/schema.go:10`) has `lanes`, `events`, `approvals` (`internal/ledger/schema.go:18-57`) but no `runs`. Run-level views scan `Lanes` or `Events` (`internal/ledger/ledger.go:285-330,490-525`).
2. **Lane metadata discarded.** `lanes` stores identity and status only (`internal/ledger/schema.go:18-32`; `internal/ledger/ledger.go:231-245`). `packet.Packet` and `dag.Node` carry `Model`, `Agent`, and `Feature` (`internal/packet/packet.go:33-75`, `internal/dag/parse.go:22-37`); `RegisterLane` does not persist them (`internal/ledger/ledger.go:255-282`).
3. **Closed `events.type` CHECK** — six literals only (`internal/ledger/schema.go:38-39`, `internal/ledger/ledger.go:440-445`). New types fail SQLite validation.
4. **No mid-flight progress store.** Executors capture stdout/stderr at process end (`internal/executor/executor.go:42-61`). Completion writes a diagnostic `lane_note` (`internal/run/run.go:422-434,488-499`).
5. **STRICT rebuilds.** Tables are `STRICT` (`internal/ledger/schema.go:32,42,56,129`). CHECK changes copy-drop-rename (`internal/ledger/schema.go:59-78,191-219`).
6. **Monolithic operations file.** Methods live in `internal/ledger/ledger.go` (`internal/ledger/ledger.go:131-192`). Overlapping `allowed_paths` fail `DisjointAllowedPaths` (`internal/packet/disjoint.go:29-48`), so parallel apply lanes cannot share that file.

`Open` enables WAL, `busy_timeout=5000`, and a pool of 4 (`internal/ledger/ledger.go:127-130,162-184`). Path is always `<primaryRoot>/.lucind/lucind.db` (`internal/ledgerpath/ledgerpath.go:36-38`; `internal/ledger/ledger.go:146-148`). CLI refuses linked worktrees (`cmd/lucind-ai/cli.go:277-280,702-705`). `ledgerpath.Validate` exists (`internal/ledgerpath/ledgerpath.go:44-58`) but `Open` does not call it (`internal/ledger/ledger.go:8-17`).

## Affected areas

- `internal/ledger/schema.go` — v6 migration
- `internal/ledger/ledger.go` — methods; split into domain files for disjoint apply
- `internal/serve/model.go` — shell-free DTOs today are feature-parent, not runs (`internal/serve/model.go:14-25,128-149`)
- `internal/serve/handlers.go` — `/api/state` reads pending approvals directly (`internal/serve/handlers.go:120-145`)
- `cmd/lucind-ai/cli.go`, `internal/run/run.go` — run identity and completion notes
- `internal/packet/disjoint.go` — file split is an apply-DAG constraint, not optional cleanup

## Candidate approaches

### 1. Relational expansion + domain files (recommended)

Advance schema through existing `migrate` (`internal/ledger/schema.go:221-307`): add `runs`; rebuild `lanes` with nullable `model`, `agent`, `feature` (and only those dispatch fields that actually exist today); add indexed `lane_progress(run_id, lane_id, seq, …)`; widen `events.type`; split methods into `runs.go` / `lanes_meta.go` / `progress.go` sharing `*Ledger` (`internal/ledger/ledger.go:131-192`).

- **Pros:** Typed queries for a Control Room read model; streaming writes isolated from lifecycle rows; new files can be disjoint (`internal/packet/disjoint.go:29-48`).
- **Cons:** Two STRICT rebuilds (`lanes`, `events`); larger schema.
- **Feasibility:** High for the migration shape (v4→v5 already rebuilt `lanes` at `internal/ledger/schema.go:191-219`). Ingest rate under lease renewals is unproven — see Spike 1 and Unresolved Contradictions in the notes.

### 2. JSON metadata + progress in `events.detail`

Add `runs` and a `metadata_json` column; stuff progress into `events.detail` (`internal/ledger/schema.go:40`).

- **Pros:** Fewer new tables; flexible keys.
- **Cons:** `events.type` still needs a rebuild; no column indexes; audit log becomes a stream dump (`internal/ledger/ledger.go:490-525`).
- **Feasibility:** Medium. Shifts validation to Go consumers.

### 3. In-memory ring buffer for progress

Migrate `runs`/`lanes` but keep live output in `internal/serve` memory; SQLite gets terminal notes only (`internal/run/run.go:422-434`).

- **Pros:** Avoids SQLite writer load during agent output.
- **Cons:** Lost on serve restart. `run` (`cmd/lucind-ai/cli.go:99-371`) and `serve` (`cmd/lucind-ai/cli.go:674-725`) do not share process memory. Package contract is a durable primary-root ledger (`internal/ledger/ledger.go:1-17`, `internal/serve/model.go:14-25`).
- **Feasibility:** Low as the progress store. An in-process pub/sub *in addition to* SQLite is a different idea (Spike 2).

## User and capability impact

Operators need live lanes, DAG/wave context, approvals, and history in one console. Reviewers keep per-lane decide and bulk-reject (`internal/serve/handlers.go:161-177`) plus `ApproverRate` (`internal/ledger/ledger.go:797-814`). Executors (`agy`, `cursor-agent`, `opencode`) need concurrent ingest without stealing the lease/status writer.

Capabilities this change must grow:

1. Durable progress chunks (seq/offset per run+lane), not only completion `lane_note`s.
2. First-class run summaries the UI can poll or tail without git/shell (`internal/serve/model.go:14-25`).
3. Pruning modeled on `PruneIntegrationEvents` (`internal/ledger/ledger.go:877-890`) that does not delete runs, lane terminal rows, approvals, or audit events.
4. Keep the single primary-root database (`internal/ledgerpath/ledgerpath.go:36-38`; `cmd/lucind-ai/cli.go:702-705`).

## Scenarios and use cases

1. **Parallel ingest.** Workers append sequenced progress rows while other lanes `SetStatus` (`internal/ledger/ledger.go:452-486`). WAL and the pool (`internal/ledger/ledger.go:162-184`) are the current concurrency seam; they do not by themselves prove zero writer wait.
2. **UI tail.** Serve reads rows after a sequence cursor (new `GetLogsAfter`-style API — not present today). `/api/state` today returns pending approvals only (`internal/serve/handlers.go:79-85,120-145`); `app.js` polls every 2s (`internal/serve/static/app.js:96-97`).
3. **Dashboard.** Compose run rollups from a `runs` table plus `Lanes` statuses (`internal/ledger/schema.go:24-25`), `PendingApprovals` (`internal/ledger/ledger.go:705-717`), and `ListLeases` (`internal/serve/model.go:207-227`). Those are separate queries today; `ListFeatures` at `internal/serve/model.go:128-149` is not a run summary.
4. **Decide + defect.** Evidence-bearing approvals (`internal/ledger/schema.go:45-56`), `Decide` (`internal/ledger/ledger.go:614-640`), `MarkDefectSurfaced` (`internal/ledger/ledger.go:643-661`), bulk reject (`internal/serve/handlers.go:161-177`), `ApproverRate` (`internal/ledger/ledger.go:797-814`).
5. **Prune streams.** Delete old progress/telemetry; leave `Lanes` (`internal/ledger/ledger.go:285-330`) and `Events` (`internal/ledger/ledger.go:490-525`). Analog: `PruneIntegrationEvents` (`internal/ledger/ledger.go:877-890`).

## Technical risks and trade-offs

| Risk | Severity | Notes |
|---|---|---|
| SQLite single-writer vs high-Hz progress + lease renew | High | WAL/`busy_timeout` (`internal/ledger/ledger.go:162-185`); renewals at `internal/run/attempt.go:434-441`; `AppendEvent` at `internal/ledger/ledger.go:366-381`. |
| STRICT rebuild locking | Medium | Pattern at `internal/ledger/schema.go:191-219`; `migrate` is one transaction (`internal/ledger/schema.go:221-229`). |
| Polling churn | High | `WaitDecision` ticks 250ms (`internal/ledger/ledger.go:772-793`); UI polls `/api/state` (`internal/serve/handlers.go:79-85`). Pub/sub would cut read load; it is not a substitute for durable rows. |
| Unpaginated `Events(runID)` | Medium | Full scan (`internal/ledger/ledger.go:490-520`); index `(run_id, id)` exists (`internal/ledger/schema.go:43`). |
| Worktree-opened DB | Critical | CLI refuses linked worktrees (`cmd/lucind-ai/cli.go:277-280,702-705`); `Open` still will not reject a worktree passed as `primaryRoot` (`internal/ledger/ledger.go:8-17`). |
| Fence/CAS | High | `feature_leases.fence` (`internal/ledger/schema.go:122-129`); CAS path calls `ValidateLease` (`internal/run/attempt.go:482-488`). Operator overrides must not reset fence. |

| Choice | Advantage | Cost |
|---|---|---|
| One SQLite file vs split telemetry store | One ACID boundary, FKs, one handle | Progress appends share the writer with status and leases |
| Pub/sub vs SQLite polling | Cheap live UI | Process-local; CLI observers still need HTTP or the DB |
| Typed columns vs JSON `detail` | Indexable, STRICT | Rebuilds when the CHECK/column set grows |
| Time-based prune vs export-then-delete | Simple size cap | Debug history gone; `DELETE` fragments without `VACUUM` |

## Potential spikes

1. **WAL writer load.** 50–100 Hz progress appends plus lease renew (`internal/run/attempt.go:434-441`) and `SetStatus` (`internal/ledger/ledger.go:452-486`) on the pool (`internal/ledger/ledger.go:155-192`). Decide unified DB vs split store from numbers.
2. **Pub/sub hook.** Fan-out around `AppendEvent` (`internal/ledger/ledger.go:366-381`) to serve handlers (`internal/serve/handlers.go:36-85`) so `/api/state` is not the live path. `WriteWithAudit` (`internal/ledger/ledger.go:835-873`) is feature-scoped (`FeatureID` required) — not the run-event seam.
3. **`EventsSince(runID, afterID, limit)`** on `idx_events_run` (`internal/ledger/schema.go:34-44,43`; `internal/ledger/ledger.go:490-520`).
4. **STRICT rebuild under readers.** Copy-drop-rename (`internal/ledger/schema.go:191-219`) inside `migrate` (`internal/ledger/schema.go:221-307`).

## Success criteria

- [ ] v6 applies idempotently through `migrate` (`internal/ledger/schema.go:221-307`) without corrupting `lanes`/`events`/`approvals`.
- [ ] Concurrent progress writes from multiple worktrees complete without unhandled `SQLITE_BUSY` under `MaxOpenConns=4` (`internal/ledger/ledger.go:182-184`).
- [ ] Control Room reads are typed structs/JSON with no git/shell (`internal/serve/model.go:14-25`).
- [ ] Offset/seq tail queries return only new chunks.
- [ ] Prune deletes expired progress only; lane terminal state and approval history remain.
- [ ] Connections resolve to `<primaryRoot>/.lucind/lucind.db` (`internal/ledgerpath/ledgerpath.go:36-38`) and CLI still refuses linked worktrees (`cmd/lucind-ai/cli.go:702-705`).
- [ ] Per-lane decide, bulk reject, and `ApproverRate` unchanged (`internal/serve/handlers.go:161-177`, `internal/ledger/ledger.go:797-814`).

## Out of scope

- Control Room HTML/CSS/JS views (`control-room-ui-shell`, `control-room-ui-views`)
- HTTP/SSE/WebSocket routing and listener lifecycle (`control-room-serve`)
- Child stdout piping (`control-room-capture`)
- Telemetry aggregation algorithms and trace formats (`control-room-telemetry`)
- gentle-ai review, RDD gates, CLI admission contracts
- Inventing packet/DAG fields (`sdd_phase`, `fanout_group`, `wave`) that do not exist on `Packet`/`Node` today

## Open questions

- [ ] Auto-prune `lane_progress` like `PruneIntegrationEvents` (`internal/ledger/ledger.go:877-890`)? Default cutoff (7 vs 30 days)?
- [ ] Run status changes: new `events` types via `AppendEvent` (`internal/ledger/ledger.go:366-381`), not `WriteWithAudit` as it stands (`internal/ledger/ledger.go:835-838`)?
- [ ] Progress bytes in SQLite vs worktree files with ledger offsets/URIs?
- [ ] Pub/sub inside `internal/ledger` or `internal/serve`?
