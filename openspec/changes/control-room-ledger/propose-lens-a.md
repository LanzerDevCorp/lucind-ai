# Proposal Lens A — Candidate & Approach: Control Room Ledger

## Selected Candidate & Approach

**Chosen Candidate:** Candidate 1 — Relational Schema Expansion with Modular Domain Files.

Advance schema version from 5 (`internal/ledger/schema.go:10`) to 6 through transactional migrations in `migrate` (`internal/ledger/schema.go:221-307`). The core approach resolves the six exploration gaps across persistence, concurrency, and modularity:

1. **First-Class `runs` Table**: Introduce a `runs` table (`run_id`, `feature_id`, `status`, `target_ref`, `lane_count`, `started_at`, `ended_at`) to replace ephemeral UUID minting in CLI dispatch (`cmd/lucind-ai/cli.go:282-290`) and eliminate table scans across `lanes` (`internal/ledger/ledger.go:285-330`) or `events` (`internal/ledger/ledger.go:490-525`) for run lifecycle rollups.
2. **Relational `lanes` Rebuild**: Rebuild `lanes` via copy-drop-rename (`internal/ledger/schema.go:191-219`) to persist dispatch metadata (`model`, `agent`, `feature`) currently defined in `packet.Packet` (`internal/packet/packet.go:33-75`) and `dag.Node` (`internal/dag/parse.go:22-37`) but discarded during `RegisterLane` (`internal/ledger/ledger.go:255-282`).
3. **Sequenced Streaming Telemetry (`lane_progress`)**: Create an indexed `lane_progress(run_id, lane_id, seq, message, at)` table to capture mid-flight streaming progress from executors (`internal/executor/executor.go:42-61`), replacing end-of-run-only diagnostic note logging (`internal/run/run.go:422-434,488-499`) while keeping high-frequency output isolated from operational audit logs.
4. **Widen `events.type` CHECK Constraint**: Rebuild `events` via copy-drop-rename (`internal/ledger/schema.go:59-78`) to expand `events.type` literals (`internal/ledger/schema.go:38-39`) with run lifecycle transitions (`run_status_changed`).
5. **Modularize Domain Files**: Split the monolithic `internal/ledger/ledger.go` (`internal/ledger/ledger.go:131-192`) into domain files (`runs.go`, `lanes_meta.go`, `progress.go`, `events.go`) sharing `*Ledger`, satisfying apply DAG path disjointness (`internal/packet/disjoint.go:29-48`) for parallel implementation lanes.
6. **Preserve Single-Ledger Primary Isolation**: Retain SQLite WAL mode, `busy_timeout=5000`, and `MaxOpenConns=4` (`internal/ledger/ledger.go:127-130,162-184`) targeting the single primary repository ledger `<primaryRoot>/.lucind/lucind.db` (`internal/ledgerpath/ledgerpath.go:36-38`; `cmd/lucind-ai/cli.go:277-280,702-705`).

This approach provides typed relational queries for the Control Room read model (`internal/serve/model.go:14-25`), isolates high-frequency append streams, and satisfies worktree concurrency boundaries.

## Conceptual Changes & Architecture Rationale

### Conceptual Changes

- **Run as a First-Class System Entity**: Elevates runs from ephemeral CLI runtime strings (`cmd/lucind-ai/cli.go:282-290`) to durable, queryable entities in SQLite, tracking lifecycle statuses (`pending`, `running`, `done`, `failed`, `released`).
- **Metadata-Preserving Lane Registration**: Retains executor configuration and dispatch target context (`model`, `agent`, `feature` from `internal/packet/packet.go:48-64` and `internal/dag/parse.go:26-28`) in SQLite rather than dropping them during `RegisterLane` (`internal/ledger/ledger.go:269-276`).
- **Dual-Stream Telemetry vs Audit Separation**: Formally splits append-heavy execution progress (`lane_progress`) from immutable lifecycle and governance audit trails (`events` at `internal/ledger/schema.go:34-43` and `approvals` at `internal/ledger/schema.go:45-56`).
- **Domain-Partitioned Ledger Source**: Restructures `internal/ledger` operations from a monolithic file (`internal/ledger/ledger.go:131-1435`) into distinct Go domain modules to enable concurrent apply lanes under `packet.DisjointAllowedPaths` (`internal/packet/disjoint.go:29-48`).

### Architecture Rationale

- **Durable SQLite as the Single Cross-Process Seam**: The runtime architecture decouples dispatch (`lucind-ai run` at `cmd/lucind-ai/cli.go:99-371`) from operator serving (`lucind-ai serve` at `cmd/lucind-ai/cli.go:674-725`, `internal/serve/server.go:12-18`). Storing runs, metadata, and progress in SQLite ensures continuous observation across process restarts without external IPC brokers or daemon dependencies.
- **Strict Typing and Schema Migration Integrity**: Maintains SQLite `STRICT` table guarantees (`internal/ledger/schema.go:32,42,56,129`) via transactional schema migrations (`internal/ledger/schema.go:221-307`), adhering to proven create-copy-drop-rename patterns (`internal/ledger/schema.go:191-219`).
- **Shell-Free Read Surface**: Enables shell-free DTO querying for localhost UI handlers (`internal/serve/model.go:14-25,128-149`), keeping Control Room reads isolated from Git or shell operations.
- **Governance & Concurrency Invariant Preservation**: Preserves critical governance mechanisms unchanged, including approval decisions and bulk-rejections (`internal/serve/handlers.go:161-177`), approver accuracy metrics (`internal/ledger/ledger.go:797-814`), and lease fencing (`internal/ledger/schema.go:122-129`, `internal/run/attempt.go:482-488`).

## Alternatives Considered & Rejected

### Candidate 2 — JSON Metadata Extension & Progress in `events.detail`

- **Approach**: Add a `runs` table, add a `metadata_json TEXT` column to `lanes`, and append streaming progress logs as JSON strings in `events.detail` (`internal/ledger/schema.go:40`).
- **Why Rejected**:
  1. Lacks SQL-level schema validation and column indexing for lane metadata.
  2. Modifying `events.type` CHECK literals (`internal/ledger/schema.go:38-39`) still requires a table rebuild, yielding no migration savings.
  3. Mixing high-frequency progress logs into `events` severely degrades query performance and poll efficiency for operational audit history (`internal/ledger/ledger.go:490-525`).
  4. Forces runtime JSON serialization/deserialization and error handling onto Go consumers in `internal/serve` and `internal/run`.

### Candidate 3 — In-Memory Ring Buffer for Progress

- **Approach**: Migrate `runs` and `lanes` in SQLite, but buffer streaming logs exclusively in process memory inside `internal/serve`, persisting only terminal notes (`internal/run/run.go:422-434,488-499`) upon lane completion.
- **Why Rejected**:
  1. Breaks process isolation: `lucind-ai run` (`cmd/lucind-ai/cli.go:99-371`) and `lucind-ai serve` (`cmd/lucind-ai/cli.go:674-725`) run as separate OS processes and cannot share in-memory state without external IPC mechanisms.
  2. Telemetry is lost on server restart or when headless CLI runs execute without an active server.
  3. Violates the core architectural invariant establishing the SQLite database at `<primaryRoot>/.lucind/lucind.db` (`internal/ledgerpath/ledgerpath.go:36-38`) as the authoritative cross-command state seam (`internal/ledger/ledger.go:1-17`, `internal/serve/model.go:14-25`).

## Open Questions

- [ ] Should `lane_progress` implement an automated retention pruning mechanism modeled on `PruneIntegrationEvents` (`internal/ledger/ledger.go:877-890`) with a configurable cutoff?
- [ ] Should run lifecycle state transitions be emitted through `AppendEvent` (`internal/ledger/ledger.go:366-381`) with `run_status_changed`, or via dedicated transaction helpers?
- [ ] Execution-topology precedence: As authorized by this packet, proposal fan-out executes across three parallel lenses (Lens A Candidate & Approach, Lens B Capabilities & Scenarios, Lens C Risks & Rollback) rather than a single sub-agent writing a monolithic `proposal.md` (`~/.claude/skills/sdd-propose/SKILL.md:92-158`).
