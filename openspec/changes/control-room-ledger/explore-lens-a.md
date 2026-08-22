# Explore Lens A — Problem & Candidates: Control Room Ledger

## Problem Space

The `lucind-ai` runtime orchestrates concurrent lane execution across isolated worktrees (`internal/run/batch.go:66-113`, `internal/worktree/worktree.go:79-115`). The Control Room initiative expands the existing localhost server (`internal/serve/server.go:1-68`, `internal/serve/handlers.go:36-118`, `internal/serve/static/app.js:1-97`) from an approvals-only polling UI into a comprehensive operational console. Supporting this requires resolving six core data model and persistence limitations in `internal/ledger`:

1. **No First-Class `runs` Entity**: Runs are created as ephemeral UUIDs at CLI invocation (`cmd/lucind-ai/cli.go:282-290`). In `internal/ledger/schema.go:18-57` (currently at version 5, `internal/ledger/schema.go:10`), there is no `runs` table. Querying historical or active run lifecycles, aggregate lane counts, feature associations, or overall run status (`pending`, `running`, `done`, `failed`, `released`) requires scanning `lanes` (`internal/ledger/ledger.go:285-330`) or `events` (`internal/ledger/ledger.go:490-525`).
2. **Missing Lane Execution & SDD Metadata**: The `lanes` table (`internal/ledger/schema.go:18-32`) only records basic identity and status (`run_id`, `lane_id`, `packet_id`, `executor`, `routing_condition`, `status`, `worktree_path`, `worktree_preserved`, `attempt`, `started_at`, `ended_at`). Dispatch metadata present in `dag.Node` (`internal/dag/parse.go:22-37`) and `packet.Packet` (`internal/packet/packet.go:33-75`)—including `model`, `agent`, `sdd_phase`, `fanout_group`, `change_id`, `feature_id`, and DAG `wave`—is discarded before `RegisterLane` (`internal/ledger/ledger.go:255-282`).
3. **Closed Event Type Constraint**: The `events` table restricts event classification via `CHECK (type IN ('run_started','lane_registered','lane_status_changed','lane_note','barrier_released','run_ended'))` (`internal/ledger/schema.go:38-39`). New telemetry and progress events fail SQLite validation.
4. **No Mid-Flight Streaming Telemetry Storage**: `internal/executor/executor.go` and `internal/run/run.go:488-499` only capture final stdout/diagnostics on lane completion. The ledger lacks structured storage for streaming agent updates (`progress_events`), preventing real-time UI rendering.
5. **STRICT Table Migration Mechanics**: The ledger uses SQLite `STRICT` tables (`internal/ledger/schema.go:32, 42, 56, 129`). Because SQLite cannot alter `CHECK` constraints in place, modifying `lanes` or `events` requires table-rebuilding migrations (`internal/ledger/schema.go:59-78, 191-219`).
6. **Monolithic Source File Coupling**: All ledger operations reside in `internal/ledger/ledger.go:1-1436`. Under apply DAG disjointness rules (`internal/packet/disjoint.go:29-48`), parallel implementation lanes cannot modify `ledger.go` concurrently.

## Candidate Approaches

### Candidate 1 — Relational Schema Expansion with Modular Domain Files

**Approach**: Advance the schema to version 6 via `migrateV5ToV6DDL` (`internal/ledger/schema.go:10, 224-306`). Introduce a `runs` table (`run_id`, `feature_id`, `status`, `target_ref`, `lane_count`, `started_at`, `ended_at`). Rebuild `lanes` with nullable metadata columns (`model`, `agent`, `sdd_phase`, `fanout_group`, `change_id`, `feature_id`, `wave`). Create an indexed `lane_progress` table (`run_id`, `lane_id`, `seq`, `event_type`, `message`, `at`) for streaming logs. Expand `events.type` CHECK literals. Modularize `internal/ledger/ledger.go` into domain files (`runs.go`, `lanes_meta.go`, `progress.go`, `events.go`) sharing `*Ledger` (`internal/ledger/ledger.go:131-192`).
**Pros**: Provides typed relational querying for `internal/serve/model.go:128-344`; isolates high-frequency streaming writes from primary audit logs (`internal/ledger/ledger.go:490-525`); separates files to satisfy `packet.DisjointAllowedPaths` (`internal/packet/disjoint.go:29-48`).
**Cons**: Requires table rebuild migrations for `lanes` and `events`; increases schema surface area.
**Feasibility**: High. Reuses existing WAL connection pooling (`internal/ledger/ledger.go:127, 163-185`) and proven migration patterns (`internal/ledger/schema.go:191-219`).

### Candidate 2 — JSON Blob Metadata Extension

**Approach**: Add a `runs` table, but avoid adding individual columns to `lanes` or creating a `lane_progress` table. Instead, add a `metadata_json TEXT` column to `lanes` and append streaming progress as JSON strings in `events.detail` (`internal/ledger/schema.go:40`).
**Pros**: Smaller migration footprint; flexible schema without column migrations for future metadata.
**Cons**: Lacks SQL-level column type safety and indexability; requires JSON serialization/deserialization on every read in `internal/serve/model.go:446-516`; mixing high-frequency streaming events into `events` table bloats operational audit history (`internal/ledger/ledger.go:490-525`); `events.type` CHECK still requires a table rebuild.
**Feasibility**: Medium. Shifts indexing and validation burden onto Go consumers in `internal/serve` and `internal/run`.

### Candidate 3 — In-Memory Ring Buffer for Progress with Core Schema Migration

**Approach**: Migrate SQLite to version 6 adding `runs` and expanding `lanes` columns, but exclude streaming progress from SQLite entirely. Retain real-time agent output in an in-memory ring buffer inside `internal/serve`, writing only final terminal logs to SQLite at lane completion (`internal/run/run.go:480-500`).
**Pros**: Eliminates SQLite disk I/O and WAL write contention during aggressive agent output streaming.
**Cons**: Streaming history is lost on server restart; headless CLI execution (`cmd/lucind-ai/cli.go:99-371`) cannot feed the UI server (`cmd/lucind-ai/cli.go:674-725`) without external IPC, violating the architecture where the ledger SQLite database is the sole cross-process coordination seam (`internal/ledger/ledger.go:1-17`, `internal/serve/model.go:14-25`).
**Feasibility**: Low. Breaks process isolation and cross-command visibility between `lucind-ai run` and `lucind-ai serve`.

## Initial Recommendations

Candidate 1 (Relational Schema Expansion with Modular Domain Files) is recommended:

1. **Architectural Alignment**: Preserves `internal/ledger` as the durable, SQLite-backed single source of truth across CLI dispatchers (`cmd/lucind-ai/cli.go:285-290`) and UI servers (`cmd/lucind-ai/cli.go:707-712`, `internal/serve/model.go:14-25`).
2. **Performance & Concurrency**: Leverages SQLite's active WAL mode and busy timeout configuration (`internal/ledger/ledger.go:127-130, 163`) while segregating append-heavy streaming progress (`lane_progress`) from relational lifecycle state (`runs`, `lanes`).
3. **Apply DAG Compatibility**: Splitting new domain methods across separate Go files (`runs.go`, `lanes_meta.go`, `progress.go`) enables parallel implementation lanes without violating path disjointness (`internal/packet/disjoint.go:29-48`).

## Open Questions

- [ ] Should `lane_progress` implement an automated retention/pruning mechanism analogous to `PruneIntegrationEvents` (`internal/ledger/ledger.go:877-890`) to manage long-term database size?
- [ ] Should `runs` lifecycle state transitions be recorded through `Ledger.WriteWithAudit` (`internal/ledger/ledger.go:835-873`) to emit synchronized `run_status_changed` events?
