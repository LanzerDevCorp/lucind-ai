# Design Lens A — Decisions: Control Room Ledger

## Assumed architecture

We assume Candidate 1 (relational schema expansion with modular domain files). Schema version advances from 5 (`internal/ledger/schema.go:10`) to 6 via transactional `migrate` (`internal/ledger/schema.go:221-307`), adding `runs` and `lane_progress` tables, rebuilding `lanes` with nullable `model`, `agent`, `feature` columns, and rebuilding `events` to admit `run_status_changed`. `internal/ledger/ledger.go` methods split into domain files (`runs.go`, `lanes_meta.go`, `progress.go`, `events.go`) sharing `*Ledger` for disjoint apply paths. `serve.Model` (`internal/serve/model.go:14-25`) adds shell-free DTOs querying SQLite directly, while `cmd/lucind-ai/cli.go:282-290` registers runs at dispatch while preserving WAL mode, `busy_timeout=5000`, `MaxOpenConns=4`, and primary-root isolation.

## Technical Approach

Advance SQLite schema to v6 for Control Room's durable read model across CLI dispatch (`cmd/lucind-ai/cli.go:282-290`) and serving (`cmd/lucind-ai/cli.go:674-725`). Strategy maps to proposal delta specs:

1. **First-class run persistence**: Store durable `runs` rows at dispatch; eliminate dynamic rollups across `lanes` (`internal/ledger/ledger.go:285-330`) or `events` (`internal/ledger/ledger.go:490-525`).
2. **Lane dispatch metadata**: Rebuild `lanes` to retain executor model, agent, and feature parameters from `packet.Packet` (`internal/packet/packet.go:43-64`) and `dag.Node` (`internal/dag/parse.go:26-28`) during `RegisterLane` (`internal/ledger/ledger.go:255-282`).
3. **Progress ingest and cursor tail**: Add `lane_progress` indexed on `(run_id, lane_id, seq)` for executor streaming, isolated from audit `events` (`internal/ledger/schema.go:34-43`).
4. **Isolated progress pruning**: Provide timestamp-cutoff progress pruning without altering governance `approvals` (`internal/ledger/schema.go:45-56`) or audit logs.
5. **Shell-free run DTOs**: Extend `serve.Model` (`internal/serve/model.go:14-25`) with direct SQLite DTO queries for UI state polling (`internal/serve/static/app.js:96-97`).
6. **Primary-root isolation**: Retain `ledgerpath.Resolve` (`internal/ledgerpath/ledgerpath.go:36-38`) and worktree refusal gates (`cmd/lucind-ai/cli.go:277-280,702-705`).

## Decision 1 — Transactional Schema Migration via Copy-Drop-Rename

**Choice**: Advance `schemaVersion` from 5 to 6 in `internal/ledger/schema.go:10` via transactional `migrate` (`internal/ledger/schema.go:221-307`), applying copy-drop-rename DDL for `lanes` and `events` and creating `runs` and `lane_progress` with `STRICT` enforcement.
**Alternatives considered**: `ALTER TABLE ADD COLUMN` for `lanes` was rejected because SQLite cannot alter `CHECK` constraints on `events.type` (`internal/ledger/schema.go:38-39`) or enforce `STRICT` mode; external migration tools were rejected because embedded Go migrations guarantee idempotent upgrades inside `ledger.Open` (`internal/ledger/ledger.go:146-189`).
**Rationale**: Copy-drop-rename matches repository conventions in `migrateV1ToV2DDL` (`internal/ledger/schema.go:59-78`) and `migrateV4ToV5DDL` (`internal/ledger/schema.go:190-219`), preserving existing rows in one atomic transaction.
**Terminal consumer**: `ledger.Open` at `internal/ledger/ledger.go:186` executing `migrate` at `internal/ledger/schema.go:224`, invoked by `lucind-ai run` (`cmd/lucind-ai/cli.go:285`) and `lucind-ai serve` (`cmd/lucind-ai/cli.go:708`).

## Decision 2 — Durable `runs` Table for Lifecycle Tracking

**Choice**: Create a `runs` table (`run_id`, `feature_id`, `status`, `target_ref`, `lane_count`, `started_at`, `ended_at`) and insert a `running` record at UUID generation in `cmd/lucind-ai/cli.go:282-290`, updating to terminal status upon batch completion (`cmd/lucind-ai/cli.go:304-320`).
**Alternatives considered**: Dynamic rollup scans across `lanes` (`internal/ledger/ledger.go:285-330`) or `events` (`internal/ledger/ledger.go:490-525`) were rejected because $O(N)$ scans degrade under polling and miss run timestamps or pre-lane failures; filesystem sidecars were rejected for violating SQLite transaction boundaries.
**Rationale**: Elevates runs into first-class relational entities, enabling indexed lookup of lifecycle states across decoupled processes without scanning lane tables.
**Terminal consumer**: `lucind-ai run` dispatch in `cmd/lucind-ai/cli.go:282-292` registering runs, and `serve.Model.ListRuns` / `serve.Model.GetRun` in `internal/serve/model.go:14-25` queried by `/api/state` (`internal/serve/handlers.go:79-85,120-146`).

## Decision 3 — Discrete `lane_progress` Table with Cursor Tail Queries

**Choice**: Store execution progress chunks in a dedicated `lane_progress(run_id, lane_id, seq, message, at)` table indexed by `(run_id, lane_id, seq)`, queried via ascending cursor filters (`WHERE run_id = ? AND lane_id = ? AND seq > ? ORDER BY seq ASC`).
**Alternatives considered**: Appending streaming logs into `events.detail` (`internal/ledger/schema.go:40`) was rejected because high-volume log writes pollute audit trails and degrade UI polling (`internal/serve/static/app.js:96-97`); an in-memory buffer was rejected because `lucind-ai run` and `lucind-ai serve` run in separate OS processes and cannot share memory.
**Rationale**: Separates high-frequency append telemetry from lifecycle transitions (`internal/ledger/schema.go:38-39`), enabling fast cursor pagination without full table scans.
**Terminal consumer**: Progress writer in `internal/run/run.go:422-435` and `serve.Model.GetLaneProgress` in `internal/serve/model.go:14-25`.

## Decision 4 — Isolated Time-Cutoff Retention Pruning

**Choice**: Implement `PruneProgress(ctx context.Context, cutoff time.Time) (int64, error)` executing `DELETE FROM lane_progress WHERE at < ?`, isolated to progress records.
**Alternatives considered**: Cascading deletes (`ON DELETE CASCADE`) from `runs`/`lanes` were rejected because progress pruning must never delete governance artifacts (`approvals` at `internal/ledger/schema.go:45-56`); database truncation was rejected because historical approval records (`internal/ledger/ledger.go:797-814`) are permanent compliance requirements.
**Rationale**: Matches the retention pattern in `PruneIntegrationEvents` (`internal/ledger/ledger.go:877-890`), maintaining strict separation between disposable log telemetry and durable governance records.
**Terminal consumer**: `ledger.PruneProgress` method in `internal/ledger/progress.go` modeled on `internal/ledger/ledger.go:877-890`.

## Decision 5 — Modular Domain File Decomposition for Apply Path Disjointness

**Choice**: Decompose `internal/ledger/ledger.go` (1436 lines) into domain files (`runs.go`, `lanes_meta.go`, `progress.go`, `events.go`) sharing `*Ledger` and `db *sql.DB` (`internal/ledger/ledger.go:131-134`).
**Alternatives considered**: Keeping all methods in `internal/ledger/ledger.go` was rejected because parallel apply DAG lanes cannot declare disjoint `allowed_paths` (`internal/packet/disjoint.go:29-48`); subpackages were rejected because they introduce circular dependencies and break public package imports across `cmd/lucind-ai` and `internal/serve`.
**Rationale**: Preserves the single cohesive `ledger.Ledger` type and public API while allowing parallel apply lanes to declare non-overlapping file scopes under `packet.DisjointAllowedPaths` (`internal/packet/disjoint.go:29-48`).
**Terminal consumer**: `packet.DisjointAllowedPaths` validation at `internal/packet/disjoint.go:29-48` and DAG execution in `internal/dag/parse.go:22-37`.

## Decision 6 — Direct SQLite Read Model on `serve.Model`

**Choice**: Implement run and progress DTO queries (`ListRuns`, `GetRun`, `GetLaneProgress`) directly on `serve.Model` (`internal/serve/model.go:14-25`) using SQL queries against `ledger.DB()`, returning JSON-safe structs without invoking git or shell subprocesses.
**Alternatives considered**: Shelling out to `git` or CLI tools from HTTP handlers was rejected because subprocesses introduce latency, shell injection risks, and violate the AST check in `TestModelSourceDoesNotShellOut` (`internal/serve/model_test.go:595`); exposing raw `*sql.DB` to HTTP handlers was rejected because `serve.Model` encapsulates JSON mapping.
**Rationale**: Extends the established pattern of `ListFeatures` (`internal/serve/model.go:128-149`), guaranteeing fast, shell-free read models for localhost UI consumers.
**Terminal consumer**: `serveStateJSON` and HTTP state handlers in `internal/serve/handlers.go:79-85,120-146`.

## Open Questions

- [ ] Should `lane_progress` retention pruning be triggered periodically by `lucind-ai serve` background workers or exposed strictly as an on-demand CLI maintenance command?
- [ ] Should `lane_progress.message` store raw text chunks or structured JSON distinguishing stdout, stderr, and control events?
- [ ] Execution-topology note: Proposal and design phases fan out across three parallel lenses (Lens A, Lens B, Lens C) feeding a synthesis lane, per packet authorization, superseding single-subagent monolithic `design.md` generation in `~/.claude/skills/sdd-design/SKILL.md`.
