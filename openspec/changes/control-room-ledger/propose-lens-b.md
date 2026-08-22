# Proposal Lens B — Capability Impact & Specs: Control Room Ledger

## User & Capability Impact

| Capability | Impact (Added / Modified / Removed) | Description | Existing seam (file:line) |
|---|---|---|---|
| `run-lifecycle-ledger` | Added | Persists top-level run records (`runs` table) with timestamps and status to support multi-lane dashboard queries. | [internal/ledger/schema.go:10-57](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/internal/ledger/schema.go#L10-L57), [cmd/lucind-ai/cli.go:282-290](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/cmd/lucind-ai/cli.go#L282-L290) |
| `lane-progress-stream` | Added | Records incremental stdout/stderr progress chunks indexed by `(run_id, lane_id, seq)` for live UI streaming during execution. | [internal/run/run.go:422-435](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/internal/run/run.go#L422-L435), [internal/ledger/ledger.go:162-185](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/internal/ledger/ledger.go#L162-L185) |
| `progress-stream-pruning` | Added | Deletes expired progress stream chunks older than a retention cutoff without dropping run summaries, lane rows, approvals, or audit events. | [internal/ledger/ledger.go:877-890](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/internal/ledger/ledger.go#L877-L890), [internal/ledger/schema.go:171-180](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/internal/ledger/schema.go#L171-L180) |
| `lane-execution` | Modified | Persists dispatch metadata (`model`, `agent`, `feature`) in `lanes` and admits widened event types in `events` via schema v6 migration. | [internal/packet/packet.go:33-75](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/internal/packet/packet.go#L33-L75), [internal/ledger/schema.go:18-44](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/internal/ledger/schema.go#L18-L44), [internal/ledger/ledger.go:255-282](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/internal/ledger/ledger.go#L255-L282) |
| `approvals-web-ui` | Modified | Expands read model DTOs to serve unified run summaries and lane progress tail responses without spawning git or shell sub-processes. | [internal/serve/model.go:14-25](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/internal/serve/model.go#L14-L25), [internal/serve/handlers.go:79-85](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/internal/serve/handlers.go#L79-L85) |

## Delta Specifications

### Requirement: First-Class Run Lifecycle Persistence

The ledger MUST maintain a dedicated `runs` table populated at run initialization and updated upon batch completion. The table MUST record `run_id`, `status`, `started_at`, and `ended_at`. Run status MUST NOT be derived solely by scanning lane rows.

#### Scenario: Run registration at dispatch

- GIVEN a new batch dispatch initiated via the CLI ([cmd/lucind-ai/cli.go:282-290](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/cmd/lucind-ai/cli.go#L282-L290))
- WHEN `RegisterRun` is executed
- THEN the ledger MUST persist a new row in `runs` with status `running` and a valid UTC `started_at` timestamp.

#### Scenario: Run completion update

- GIVEN an active run in `runs` with status `running`
- WHEN all lanes reach a terminal status and the batch execution finishes
- THEN the ledger MUST update the run row with its terminal status and non-null `ended_at` timestamp.

### Requirement: Lane Dispatch Metadata Persistence

The `lanes` table MUST be migrated to schema version 6 to include nullable columns for dispatch metadata present on `packet.Packet` (`model`, `agent`, `feature`). `RegisterLane` MUST persist these fields when present.

#### Scenario: Lane registered with model and agent metadata

- GIVEN a packet with `model` and `agent` fields defined ([internal/packet/packet.go:43-54](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/internal/packet/packet.go#L43-L54))
- WHEN `RegisterLane` is invoked ([internal/ledger/ledger.go:255-282](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/internal/ledger/ledger.go#L255-L282))
- THEN the ledger MUST store the `model` and `agent` values in the migrated `lanes` table row.

#### Scenario: Backward-compatible schema migration

- GIVEN an existing schema v5 database with populated `lanes` rows ([internal/ledger/schema.go:10,191-219](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/internal/ledger/schema.go#L10))
- WHEN `migrate` executes ([internal/ledger/schema.go:224-307](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/internal/ledger/schema.go#L224-L307))
- THEN all existing lane records MUST be preserved with empty or null defaults for the new metadata columns.

### Requirement: Incremental Progress Chunk Ingestion and Cursor Tailing

The ledger MUST provide a `lane_progress` table indexed by `(run_id, lane_id, seq)` for appending mid-flight chunk payloads. The query API MUST support cursor-based tailing (`GetProgressAfter`) returning only chunks strictly greater than a provided sequence offset.

#### Scenario: Concurrent progress chunk insertion

- GIVEN multiple active worker lanes writing output concurrently under WAL mode ([internal/ledger/ledger.go:162-185](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/internal/ledger/ledger.go#L162-L185))
- WHEN workers invoke `AppendProgress` with incremental sequence numbers
- THEN chunks MUST be durably recorded without writer starvation or deadlocks.

#### Scenario: Cursor-based chunk retrieval

- GIVEN a lane with stored progress chunks at sequences `1` through `10`
- WHEN the reader queries `GetProgressAfter(ctx, runID, laneID, 5)`
- THEN the ledger MUST return only chunks with sequence numbers `6` through `10` in ascending order.

### Requirement: Isolated Progress and Telemetry Retention Pruning

The ledger MUST support time-cutoff pruning of high-frequency `lane_progress` records. Pruning operations MUST NOT delete or modify rows in `runs`, `lanes`, `approvals`, or `events`.

#### Scenario: Pruning expired progress rows

- GIVEN `lane_progress` entries created before cutoff timestamp `T` alongside terminal lane and approval records
- WHEN `PruneProgress(ctx, T)` is invoked ([internal/ledger/ledger.go:877-890](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/internal/ledger/ledger.go#L877-L890))
- THEN progress rows older than `T` MUST be deleted, while all `runs`, `lanes`, and `approvals` records remain intact.

### Requirement: Shell-Free Dashboard Aggregation DTOs

The `serve.Model` query layer MUST provide typed methods to retrieve unified run summaries, active lane statuses, pending approvals, and lease states directly from SQLite without invoking external shell or git processes.

#### Scenario: Fetching run summary aggregation

- GIVEN an active ledger containing run and lane state rows ([internal/ledger/schema.go:18-32](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/internal/ledger/schema.go#L18-L32))
- WHEN `Model.GetRunSummary(ctx, runID)` is requested ([internal/serve/model.go:14-25](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/internal/serve/model.go#L14-L25))
- THEN the model MUST return a structured DTO combining run details, lane status counts, and pending approval counts without executing external commands.

### Requirement: Single Primary-Root Enforcement and Linked Worktree Rejection

All ledger connections MUST resolve exclusively to `<primaryRoot>/.lucind/lucind.db`. CLI entry points MUST reject invocations originating within linked worktrees.

#### Scenario: Primary root path derivation

- GIVEN a valid primary repository root path
- WHEN `ledger.Open` is called ([internal/ledger/ledger.go:146-148](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/internal/ledger/ledger.go#L146-L148))
- THEN the database connection MUST resolve via `ledgerpath.Resolve` to `<primaryRoot>/.lucind/lucind.db` ([internal/ledgerpath/ledgerpath.go:36-38](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/internal/ledgerpath/ledgerpath.go#L36-L38)).

#### Scenario: Refusal inside linked worktree

- GIVEN an invocation where `primaryRoot` is a linked worktree
- WHEN `lucind-ai run` or `lucind-ai serve` is executed ([cmd/lucind-ai/cli.go:277-280,702-705](file:///home/lanzerdev/git_root/lucind-ai-worktrees/propose-control-room-ledger-lens-b/cmd/lucind-ai/cli.go#L277-L280))
- THEN the command MUST exit with code 1 and refuse to open a worktree-local ledger.

## Open Questions

- [ ] Should `lane_progress` store raw string chunks or structured JSON records distinguishing stdout, stderr, and system events?
- [ ] Should automatic retention pruning be triggered periodically by `lucind-ai serve` or exposed strictly as a maintenance command?
