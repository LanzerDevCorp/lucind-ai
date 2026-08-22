# Explore Lens B — Capabilities & Scenarios: Control Room Ledger

## User & Capability Impact

The Control Room Ledger provides the persistent storage foundation for the Lucind-AI Control Room, evolving the existing SQLite ledger into a high-throughput, queryable data store for live monitoring, execution telemetry, log stream indexing, and run history.

### Affected Users & Operators
- **Human Operators**: Gain real-time visibility into parallel lane execution, DAG wave status, approval queues, and historical run performance from a single centralized dashboard.
- **Reviewers & Approvers**: Retain strict per-lane decision capabilities ([internal/serve/handlers.go:161-177](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/serve/handlers.go#L161-L177)) with durable tracking of individual wrong-approval rates ([internal/ledger/ledger.go:797-814](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/ledger/ledger.go#L797-L814)).
- **Automated Executors (`agy`, `cursor-agent`, `opencode`)**: Stream logs, lifecycle events, and telemetry spans concurrently into the primary ledger without contention.

### New & Modified Capabilities
1. **Durable Telemetry & Log Stream Indexing**: Extends schema version 5 ([internal/ledger/schema.go:10](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/ledger/schema.go#L10)) to version 6+, adding structured storage for execution telemetry (spans, token usage, durations) and sequential log stream chunks indexed by `(run_id, lane_id, offset)`.
2. **High-Throughput Concurrent Ingestion**: Leverages SQLite WAL mode and 5000ms busy timeout ([internal/ledger/ledger.go:163](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/ledger/ledger.go#L163)) with a sized connection pool ([internal/ledger/ledger.go:182-184](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/ledger/ledger.go#L182-L184)), enabling parallel worker worktrees to write logs while the Control Room UI reads without blocking.
3. **Control Room Read Model & Aggregations**: Expands `internal/serve/model.go` ([internal/serve/model.go:14-25](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/serve/model.go#L14-L25)) to serve unified run summaries, active DAG wave states, approval queues, and failure diagnostics as shell-free DTOs.
4. **Telemetry & Log Pruning**: Adds retention cleanup mechanisms modeled after `PruneIntegrationEvents` ([internal/ledger/ledger.go:877-890](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/ledger/ledger.go#L877-L890)) to purge granular stream chunks while preserving immutable run history, lane states, and audit trails.
5. **Strict Repository-Root Ledger Isolation**: Preserves the single-ledger invariant at `<primaryRoot>/.lucind/lucind.db` ([internal/ledgerpath/ledgerpath.go:36-38](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/ledgerpath/ledgerpath.go#L36-L38)) and enforces refusal if invoked from linked worktrees ([cmd/lucind-ai/cli.go:702-705](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/cmd/lucind-ai/cli.go#L702-L705)).

## Scenarios & Use Cases

### Scenario 1 — Live Log Stream Chunk Ingestion During Parallel Execution

- **Context**: A multi-lane run executes parallel tasks using `agy` and `cursor-agent` in isolated worktrees.
- **Action**: Lane workers capture process stdout/stderr and write structured log chunks with monotonically increasing sequence offsets to the ledger via `AppendLogChunk(ctx, chunk)`.
- **Outcome**: Log chunks are committed to SQLite; WAL mode ([internal/ledger/ledger.go:163](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/ledger/ledger.go#L163)) ensures concurrent ingestion across workers completes without lock contention.

### Scenario 2 — Incremental Log Streaming to Control Room UI

- **Context**: The Control Room web server (`internal/serve`) streams real-time lane output to an operator observing active work.
- **Action**: The server polls or tails the ledger using `GetLogsAfter(ctx, runID, laneID, lastOffset)`.
- **Outcome**: The ledger queries and returns only log chunks newer than `lastOffset`, enabling low-latency, zero-duplication UI streaming without reading raw disk files.

### Scenario 3 — Dashboard Aggregation of Active and Historical Runs

- **Context**: An operator loads the Control Room overview dashboard to check system status.
- **Action**: The UI requests run summaries via `Model.ListRunSummaries(ctx)` ([internal/serve/model.go:128-149](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/serve/model.go#L128-L149)).
- **Outcome**: The ledger aggregates total runs, lane terminal status breakdowns ([internal/ledger/schema.go:24-25](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/ledger/schema.go#L24-L25)), pending approvals ([internal/ledger/ledger.go:705-717](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/ledger/ledger.go#L705-L717)), and active feature leases ([internal/serve/model.go:207-227](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/serve/model.go#L207-L227)) in a single efficient query.

### Scenario 4 — Individual Decision Recording & Approver Metric Update

- **Context**: An approval request is recorded with inline evidence in the approvals table ([internal/ledger/schema.go:45-56](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/ledger/schema.go#L45-L56)).
- **Action**: An operator submits a decision via `Decide` ([internal/ledger/ledger.go:614-640](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/ledger/ledger.go#L614-L640)), and later a defect is flagged via `MarkDefectSurfaced` ([internal/ledger/ledger.go:643-661](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/ledger/ledger.go#L643-L661)).
- **Outcome**: The decision is recorded, bulk approval is rejected ([internal/serve/handlers.go:161-177](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/serve/handlers.go#L161-L177)), and `ApproverRate` ([internal/ledger/ledger.go:797-814](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/ledger/ledger.go#L797-L814)) updates the personal wrong-approval rate for display in the Control Room.

### Scenario 5 — Pruning Expired Telemetry and Stream Logs

- **Context**: The ledger contains thousands of fine-grained log chunks and telemetry records from past runs.
- **Action**: Maintenance routines call `PruneLogs(ctx, cutoff)` and `PruneTelemetry(ctx, cutoff)` ([internal/ledger/ledger.go:877-890](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/ledger/ledger.go#L877-L890)).
- **Outcome**: Granular stream chunks and telemetry rows older than `cutoff` are deleted, while run summaries, lane statuses ([internal/ledger/ledger.go:285-330](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/ledger/ledger.go#L285-L330)), and audit events ([internal/ledger/ledger.go:488-525](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/ledger/ledger.go#L488-L525)) remain intact.

## Success Criteria

- [ ] Schema migration v6+ applies cleanly and idempotently via `migrate` ([internal/ledger/schema.go:224-307](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/ledger/schema.go#L224-L307)) without corrupting existing lane, event, or approval data.
- [ ] Concurrent log chunk and telemetry insertions from multiple worker worktrees succeed without `SQLITE_BUSY` errors under connection pool limits ([internal/ledger/ledger.go:182-184](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/ledger/ledger.go#L182-L184)).
- [ ] Control Room read queries in `internal/serve/model.go` return structured Go structs and JSON payloads without executing shell or git commands ([internal/serve/model.go:14-25](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/serve/model.go#L14-L25)).
- [ ] Log stream queries support offset-based pagination (`GetLogsAfter`) for smooth UI streaming.
- [ ] Pruning methods remove expired log chunks and telemetry rows without deleting run records, lane terminal states, or approval history.
- [ ] All database connections strictly resolve to `<primaryRoot>/.lucind/lucind.db` ([internal/ledgerpath/ledgerpath.go:36-38](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/internal/ledgerpath/ledgerpath.go#L36-L38)) and refuse execution from linked worktrees ([cmd/lucind-ai/cli.go:702-705](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ledger-lens-b/cmd/lucind-ai/cli.go#L702-L705)).

## Open Questions

- [ ] Should fine-grained stream log chunks be stored directly inside SQLite tables or on disk with index offsets recorded in the ledger?
- [ ] What default retention period (e.g. 7 days vs 30 days) should apply to high-volume telemetry spans and stream logs before automated pruning?
