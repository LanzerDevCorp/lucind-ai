# Design: Control Room Ledger

Candidate 1: relational schema expansion with modular domain files. Advance `schemaVersion` from 5 (`internal/ledger/schema.go:10`) to 6 inside transactional `migrate` (`internal/ledger/schema.go:221-307`) so `lucind-ai run` (`cmd/lucind-ai/cli.go:282-290`) and `lucind-ai serve` (`cmd/lucind-ai/cli.go:674-725`) share `<primaryRoot>/.lucind/lucind.db`. Maps to proposal deltas: first-class runs, lane dispatch metadata, progress ingest and cursor tail, isolated prune, shell-free DTOs, primary-root isolation.

| # | Question | Recommendation |
|---|---|---|
| 1 | Where do runs live? | `runs` table, not rollups of `Lanes` (`internal/ledger/ledger.go:285-330`) or `Events` (`:490-525`). |
| 2 | Lane model/agent/feature? | Nullable columns via copy-drop-rename; persist at `RegisterLane` (`internal/run/run.go:327`). |
| 3 | Mid-flight progress? | `lane_progress` + cursor `seq > ?`, not `events.detail` (`internal/ledger/schema.go:40`). |
| 4 | Prune? | `DELETE FROM lane_progress WHERE at < ?` only; analog `PruneIntegrationEvents` (`internal/ledger/ledger.go:877-890`). |
| 5 | Control Room reads? | Typed methods on `serve.Model` (`internal/serve/model.go:14-25`); analog `ListFeatures` (`:128-149`). |
| 6 | Concurrency? | Keep WAL, `busy_timeout=5000`, `MaxOpenConns=4` (`internal/ledger/ledger.go:162-185`). Do not treat that as proof of zero `SQLITE_BUSY` under ingest. |

## Architecture Decisions

### Decision: Transactional schema v6 via copy-drop-rename

**Choice**: Bump `schemaVersion` to 6. Add `migrateV5ToV6DDL`: `CREATE` `runs` and `lane_progress` `STRICT`; rebuild `lanes` adding nullable `model`, `agent`, `feature`; rebuild `events` widening `CHECK(type IN (...))` (`internal/ledger/schema.go:38-39`) with `'run_status_changed'`. Gate with `currentVersion < 6` like the v5 step (`:240-304`).
**Alternatives considered**: `ALTER TABLE ADD COLUMN` on `lanes` (cannot change the `events` CHECK; STRICT rebuild is the repo pattern — `migrateV1ToV2DDL` `:59-78`, `migrateV4ToV5DDL` `:190-219`). External migrators (rejected: `Open` already runs `migrate` at `internal/ledger/ledger.go:186`).
**Rationale**: One transaction (`:221-229`); existing rows copied; second `Open` is a no-op.
**Terminal consumer**: `ledger.Open` (`internal/ledger/ledger.go:146-189`) from `lucind-ai run` (`cmd/lucind-ai/cli.go:285`) and `serve` (`:708`).

### Decision: Durable `runs` row at dispatch

**Choice**: Table `runs(run_id, feature_id, status, target_ref, lane_count, started_at, ended_at)`. After `runID := uuid.NewString()` (`cmd/lucind-ai/cli.go:282-290`) and `ledger.Open`, insert `status='running'` with UTC `started_at`. After `ExecuteBatch` returns (`:304-311`), `UpdateRunStatus` to a terminal value and non-null `ended_at`.
**Alternatives considered**: Derive status by scanning `Lanes` or `Events` (O(N), no run timestamps, misses pre-lane failure). Filesystem sidecars (outside the SQLite transaction).
**Rationale**: `run` and `serve` are separate subcommands each calling `ledger.Open` (`cmd/lucind-ai/cli.go:285`, `:708`); the DB is the shared read model.
**Terminal consumer**: `runDispatch`; `serve.Model` run-summary queries consumed from `/api/state` (`internal/serve/handlers.go:79-85`) which today is approvals-only (`serveStateJSON` `:120-146`).

### Decision: Discrete `lane_progress` with cursor tail

**Choice**: `lane_progress(run_id, lane_id, seq, message, at)` `STRICT`, index `(run_id, lane_id, seq)`. `AppendProgress` assigns monotonic `seq`. `GetProgressAfter` returns `seq > afterSeq ORDER BY seq ASC`.
**Alternatives considered**: Append chunks to `events.detail` (pollutes the six-literal audit log; `Events` is unpaginated `:490-525`). In-memory buffer in `serve` (processes do not share memory; dies with the process).
**Rationale**: High-frequency appends stay off lifecycle `events` (`internal/ledger/schema.go:34-43`). This change is the store; `control-room-capture` writes it. Today's `lane_note` at `internal/run/run.go:422-434,488-499` stays a completion diagnostic, not a stream.
**Terminal consumer**: New `*Ledger` progress API; `serve.Model` progress-tail methods; UI poll `internal/serve/static/app.js:96-97`.

### Decision: Isolated time-cutoff prune

**Choice**: `PruneProgress(ctx, cutoff) (int64, error)` → `DELETE FROM lane_progress WHERE at < ?`. No `ON DELETE CASCADE` onto `runs`/`lanes`/`events`/`approvals` (`internal/ledger/schema.go:45-56`).
**Alternatives considered**: Cascade from `runs`. Truncate the database (would erase `ApproverRate` history `:797-814`).
**Rationale**: Same shape as `PruneIntegrationEvents` (`:877-890`). Trigger (serve ticker vs CLI) is an open question.
**Terminal consumer**: `internal/ledger/progress.go`.

### Decision: Domain files sharing `*Ledger`

**Choice**: Split methods out of `internal/ledger/ledger.go` (1436 lines) into `runs.go`, `lanes_meta.go`, `progress.go`, `events.go` on the same `Ledger` (`:131-134`). Keep `Open`/`Close`/pragmas/pool in `ledger.go`.
**Alternatives considered**: Grow `ledger.go` (apply-DAG `allowed_paths` cannot be disjoint — `internal/packet/disjoint.go:29-48`). Subpackages (circular imports with `cmd/lucind-ai` and `internal/serve`).
**Rationale**: One public type; parallel apply lanes can each own one file.
**Terminal consumer**: `packet.DisjointAllowedPaths` (`:29-48`); DAG nodes (`internal/dag/parse.go:22-37`).

### Decision: Direct SQLite DTOs on `serve.Model`

**Choice**: Add typed run-summary and progress-tail methods on `serve.Model` (`internal/serve/model.go:14-25`) using `ledger.DB()` (`internal/ledger/ledger.go:816-818`). No `os/exec`, no git.
**Alternatives considered**: Shell out from handlers (fails `TestModelSourceDoesNotShellOut` at `internal/serve/model_test.go:595`). Handlers taking raw `*sql.DB` (breaks JSON mapping in `Model`).
**Rationale**: Same pattern as `ListFeatures` (`internal/serve/model.go:128-149`). Method identifiers on `Model` are not locked (A/C vs B names; synthesis notes).
**Terminal consumer**: `serveStateJSON` (`internal/serve/handlers.go:120-146`); `ServerState` today has only approvals (`:16-21`).

## Flow and Invariants

```
CLI uuid+Open ──RegisterRun──→ runs
       │
       ├── RegisterLane (model,agent,feature) ──→ lanes
       ├── AppendProgress ──→ lane_progress ──PruneProgress──→ cutoff delete
       └── ExecuteBatch return ──UpdateRunStatus──→ runs
                                      │
         GetProgressAfter / Model DTOs ┴──→ /api/state (2s poll)
```

1. **RegisterRun before lanes.** Break: Control Room cannot see an in-flight run.
2. **`RegisterLane` stores packet `Model`/`Agent`/`Feature`** (`internal/packet/packet.go:43-64`; `internal/dag/parse.go:26-28`) at `internal/run/run.go:327` and the never-started path `internal/run/batch.go:184`. Today's INSERT omits them (`internal/ledger/ledger.go:269-276`). Break: no attribution.
3. **Progress appends are their own transactions**, isolated from lease renew (`internal/run/attempt.go:434-441`) and `ValidateLease` (`:482-488`). Break: `SQLITE_BUSY` during checks. WAL/`busy_timeout` convert busy into wait (`internal/ledger/ledger_test.go:360-366`); they do not prove ingest is cheap.
4. **Run row goes terminal after `ExecuteBatch` returns** (`cmd/lucind-ai/cli.go:304-311`). Lane `SetStatus` (`internal/run/run.go:480-483`) stays the lane write. Break: UI shows eternal `running`.
5. **Prune deletes only expired `lane_progress`.** Break: audit or approvals disappear.
6. **Cursor tail is `seq > afterSeq` ascending, shell-free.** Break: 2s polls (`internal/serve/static/app.js:96-97`) duplicate or skip chunks, or the AST test fails.

## Interfaces / Contracts

```go
type Run struct{ RunID, FeatureID, Status, TargetRef string; LaneCount int; StartedAt time.Time; EndedAt *time.Time }
type LaneProgress struct{ RunID, LaneID, Message string; Seq int64; At time.Time }
func (l *Ledger) RegisterRun(ctx context.Context, r Run) error
func (l *Ledger) UpdateRunStatus(ctx context.Context, runID, status string, endedAt time.Time) error
func (l *Ledger) GetRun(ctx context.Context, runID string) (Run, error)
func (l *Ledger) ListRuns(ctx context.Context) ([]Run, error)
func (l *Ledger) AppendProgress(ctx context.Context, p LaneProgress) error
func (l *Ledger) GetProgressAfter(ctx context.Context, runID, laneID string, afterSeq int64) ([]LaneProgress, error)
func (l *Ledger) PruneProgress(ctx context.Context, cutoff time.Time) (int64, error)
```

`Lane` gains `Model`, `Agent`, `Feature string`. `EventRunStatusChanged = "run_status_changed"`. Packet parsing (`internal/packet/packet.go:33-75`) and `internal/result/result.schema.json` unchanged. CLI flags unchanged.

| Surface | Today | Delta | Compatible? |
|---|---|---|---|
| `schemaVersion` | `5` at `schema.go:10` | `6` | Yes; `currentVersion < N`. |
| `runs`, `lane_progress` | absent (`schema.go:17-57`) | new STRICT tables | Yes; additive. |
| `lanes` | `:18-32` | nullable `model`,`agent`,`feature` | Yes; named SELECT `:287-289`. |
| `events.type` | six literals `:38-39` | plus `run_status_changed` | Yes; superset. |

## File Changes

| File | Action | Terminal consumer |
|---|---|---|
| `internal/ledger/schema.go` | Modify: v6 DDL + `currentVersion < 6` | `Open` `:146-189` / `migrate` `:224` |
| `internal/ledger/ledger.go` | Modify: `Lane` fields; keep Open/Close/pragmas/pool | `ExecuteBatch` (`cli.go:304`) |
| `internal/ledger/runs.go` | Create: `Run` + register/update/get/list | `cli.go:282-311` |
| `internal/ledger/lanes_meta.go` | Create: `RegisterLane`/`Lanes`/`LaneStates` | `run.go:327`, `batch.go:184` |
| `internal/ledger/progress.go` | Create: append/tail/prune | Model progress-tail; capture sibling |
| `internal/ledger/events.go` | Create: `AppendEvent`/`Events` + new const | `run.go:338-347,422-434,488-499` |
| `internal/serve/model.go` | Modify: run/progress DTOs + query methods | `handlers.go:79-85,120-146` |
| `cmd/lucind-ai/cli.go` | Modify: `RegisterRun` after mint; `UpdateRunStatus` after batch | `lucind-ai run` |

## Testing Strategy and Test Seams

| Layer | What | Approach | Seam |
|---|---|---|---|
| Unit | v6 migrate + idempotency; preserved rows; new cols null/empty | Clean and v1–v5 fixtures; second `migrate` no-op | `schema.go:221-307`; `ledger_test.go:579-620,733-745,934-970` |
| Unit | Cursor tail | Append; `seq > afterSeq` ascending; empty on exact cursor | New: `AppendProgress`/`GetProgressAfter` |
| Unit | Prune | Seed progress + approvals; only old progress gone | New: `PruneProgress`; analog `:877-890`, `ledger_test.go:1584` |
| Integration | Contention | Concurrent append + `SetStatus` + `ValidateLease` under pool of 4; no unhandled `SQLITE_BUSY` | `:162-185`; analog `ledger_test.go:367`; `attempt.go:434-441,482-488` |
| Unit | Shell-free Model | New DTO methods query SQLite; AST forbids exec/git | `model.go:14-25`; `model_test.go:595` |
| Integration | Primary root | `Resolve` path; CLI exit 1 in linked worktree | `ledgerpath.go:36-38`; `ledgerpath_test.go:9,37`; `cli.go:277-280,702-705` |
| Unit | Governance unchanged | `Decide`, bulk-reject, `ApproverRate` after v6 | `ledger.go:614-640,643-661,797-814`; `handlers.go:161-177`; `ledger_test.go:1047,1153` |

Existing seams: `openTestLedger` (`ledger_test.go:24`), `openAtPath` (`ledger.go:155`), `migrate(ctx, db)`, `ledgerpath.Resolve`/`Validate` (`ledgerpath.go:36-44`), `depsFactory` (`cli.go:58-60,292`; inject at `cli_test.go:1074`). New seams: the `*Ledger` and `Model` methods above.

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | N/A: change does not classify or execute documentation/script paths | N/A | N/A |
| Git repository selection | `git -C`, relative paths, absolute paths | Applicable | DB path is `<primaryRoot>/.lucind/lucind.db` via `ledgerpath.Resolve` (`internal/ledgerpath/ledgerpath.go:36-38`). `run`/`serve` refuse linked worktrees with exit 1 before `Open` (`cmd/lucind-ai/cli.go:277-280,702-705`). Safe: primary DB. Failure: worktree dispatch refused. `Open` still does not detect a worktree passed as `primaryRoot` (`internal/ledger/ledger.go:8-17`). | `TestResolve` relative/absolute (`ledgerpath_test.go:9`); CLI exit 1 on linked worktree (new; not at `cli_test.go:210-250`). |
| Commit state | staged, `commit -a`, empty index | N/A: change does not manipulate git index or commit state | N/A | N/A |
| Push state | tracking branch, first push, explicit refspec | N/A: change does not manage git push or remote refs | N/A | N/A |
| PR commands | explicit `--head`, environment prefix, composed commands | N/A: change does not compose or run PR automation commands | N/A | N/A |

## Rollback and Additivity

**Choice**: `git revert` of Go commits under `internal/ledger/`, `internal/serve/`, and `cmd/lucind-ai/`.
**Alternatives considered**: Destructive downgrade (`DROP TABLE runs`/`lane_progress`, rebuild `lanes`/`events` to v5). Rejected: SQLite ignores unused tables; v5 named SELECTs (`internal/ledger/ledger.go:287-289`) ignore extra nullable columns. `migrate` applies only `currentVersion < N` (`internal/ledger/schema.go:240-304`) and does not reject a recorded version above the binary's `schemaVersion`.
**Rationale**: Reverting the Go restores v5 behavior with no data loss. No downgrade script.

Additive: new tables; nullable lane columns; wider `events.type`; new `Model` methods without changing Feature/Lease structs (`internal/serve/model.go:14-25`). Packet parse unchanged (`internal/packet/packet.go:33-75`).

## Open Questions and Out of Scope

- [ ] Should `lane_progress` prune run as a `lucind-ai serve` ticker (`cmd/lucind-ai/cli.go:674-725`) or an on-demand CLI command? Cutoff?
- [ ] Should `message` be raw text or structured JSON (stdout / stderr / control)?

Out of scope: Control Room HTML/CSS/JS (`control-room-ui-shell`, `control-room-ui-views`); HTTP/SSE/WebSocket listener lifecycle (`control-room-serve`); child stdout/stderr piping (`control-room-capture` — this change is the store those writers use); external telemetry (`control-room-telemetry`); gentle-ai review, RDD gates, packet/result schema changes; inventing packet/DAG fields that do not exist.
