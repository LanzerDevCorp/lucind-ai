# Explore Lens C — Risks, Trade-offs & Spikes: Control Room Ledger

## Technical Risks & Unknowns

| Risk / Unknown | Severity | Mitigation / Investigation Strategy | Existing seam (file:line) |
|---|---|---|---|
| SQLite write lock contention during high-throughput telemetry appends | High | Isolate high-frequency telemetry logs or configure dedicated write queues; verify `busy_timeout` (5000ms) and WAL mode handle concurrent agent log ingest without blocking critical lease renewals or status changes. | `internal/ledger/ledger.go:162-185`, `internal/ledger/ledger.go:366-381`, `internal/run/attempt.go:434-441` |
| Table rebuild locking during schema migrations in SQLite `STRICT` mode | Medium | Altering `CHECK` constraints on `lanes` or `events` requires full table copy-and-rename migrations (`CREATE TABLE ..._new AS SELECT`); ensure migrations run in atomic transactions without deadlocking concurrent readers. | `internal/ledger/schema.go:18-57`, `internal/ledger/schema.go:190-220`, `internal/ledger/schema.go:224-255` |
| Polling query overhead & database churn from multi-view Control Room UI | High | Replace repetitive SQLite polling loops (`WaitDecision` / `/api/state`) with an in-memory Go pub/sub broadcast mechanism for live push updates (SSE/WebSocket), using SQLite solely for durable state on initial connect. | `internal/ledger/ledger.go:772-793`, `internal/serve/handlers.go:36-85`, `internal/serve/model.go:17-49` |
| Unbounded event query degradation on large run histories | Medium | Unpaginated scans via `Events(ctx, runID)` become latency bottlenecks as event rows scale; introduce cursor-based pagination (`after_id`, `limit`) backed by composite indexes `(run_id, id)`. | `internal/ledger/schema.go:43`, `internal/ledger/schema.go:179`, `internal/ledger/ledger.go:490-520`, `internal/ledger/ledger.go:893-925` |
| Cross-worktree ledger access and worktree boundary violations | Critical | `ledgerpath.Validate` rejects ledger instances located inside worktrees; Control Room telemetry/agent probes running inside worktrees must communicate through host IPC or primary root path rather than opening local SQLite handles. | `internal/ledgerpath/ledgerpath.go:23-58`, `internal/ledger/ledger.go:146-148` |
| Fencing token and lease monotonicity invalidation during operator overrides | High | Manual operator interventions (force lease revocation, manual reconciliation overrides) must preserve monotonic `fence` counters in `feature_leases` to prevent stale CAS promotions from corrupting parent refs. | `internal/ledger/schema.go:106-129`, `internal/run/attempt.go:113-135`, `internal/run/attempt.go:474-480` |

## Trade-offs Matrix

| Dimension / Choice | Advantages | Disadvantages | Operational Cost |
|---|---|---|---|
| Unified SQLite Database vs. Split Telemetry Append Store | Single ACID transaction boundary, foreign key integrity across lanes/events, zero extra file handles. | High telemetry write volume contends for the single SQLite write lock against lane status updates and lease renewals. | Low operational overhead; requires automated event retention and vacuum pruning. |
| In-Memory Event Pub/Sub vs. Periodic SQLite Polling | Sub-millisecond UI update latency, zero database query load during active streaming, efficient Go channel fan-out. | Process-local; external out-of-process CLI observers require HTTP/IPC connection rather than direct file inspection. | Minimal in-memory footprint in host `lucind-ai serve` process. |
| Structured DDL Columns vs. JSON `detail` Payload | JSON payload provides schema flexibility for diverse telemetry events without table rebuilds or DDL migrations. | Runtime JSON marshal/unmarshal overhead; queries filtering on nested JSON attributes are slower without generated columns. | Low schema migration burden; minor CPU serialization cost. |
| Time-Based Retention Pruning vs. Run-Scoped Archival Export | Fixed database size ceiling; simple `DELETE ... WHERE at < cutoff` execution. | Permanently removes historical debug logs; SQLite `DELETE` causes fragmentation without periodic `VACUUM`. | Low maintenance complexity; requires scheduled background vacuum tasks. |

## Potential Spikes / Proof of Concepts

- **Spike 1: High-Frequency Concurrent Writer Load Benchmark on WAL-mode SQLite**
  - Measure write transaction throughput and lock wait latency under 50–100Hz concurrent telemetry event appends combined with periodic lease renewals (`internal/run/attempt.go:434-441`) and lane status updates (`internal/ledger/ledger.go:452-486`) across the connection pool (`internal/ledger/ledger.go:182-184`).
  - *Seams:* `internal/ledger/ledger.go:155-192`, `internal/ledger/ledger.go:366-381`, `internal/ledger/ledger_test.go:40-120`.

- **Spike 2: In-Memory Event Fan-Out Broadcaster Hook**
  - Prototype a non-blocking Go channel pub/sub broker wrapping `Ledger.AppendEvent` (`internal/ledger/ledger.go:366-381`) and `Ledger.WriteWithAudit` (`internal/ledger/ledger.go:835-873`) to stream live state directly to `internal/serve` HTTP handlers without polling (`internal/ledger/ledger.go:772-793`).
  - *Seams:* `internal/ledger/ledger.go:366-381`, `internal/serve/handlers.go:36-85`, `internal/serve/model.go:17-49`.

- **Spike 3: Cursor-Based Paginated Event Queries**
  - Implement and benchmark `EventsSince(ctx, runID, afterID, limit)` leveraging the `(run_id, id)` index (`internal/ledger/schema.go:43`) to ensure sub-10ms response times on datasets exceeding 50,000 event rows.
  - *Seams:* `internal/ledger/schema.go:34-44`, `internal/ledger/ledger.go:490-520`.

- **Spike 4: STRICT Table Migration Under Active Read Concurrency**
  - Validate migration robustness and verify zero reader disruption when executing table rebuild copy-and-rename DDL (`internal/ledger/schema.go:190-220`, `internal/ledger/schema.go:224-255`) while concurrent read queries are in flight.
  - *Seams:* `internal/ledger/schema.go:190-220`, `internal/ledger/schema.go:224-255`, `internal/ledger/ledger_test.go:300-350`.

## Out of Scope

- Frontend UI views, layout rendering, HTML templates, CSS, and client-side JavaScript components (owned by `control-room-ui-shell` and `control-room-ui-views`).
- HTTP / SSE / WebSocket server routing, endpoint definitions, and listener lifecycle (owned by `control-room-serve`).
- Child process stdout/stderr stream piping and process multiplexing (owned by `control-room-capture`).
- Observability metric aggregation algorithms and trace format definitions (owned by `control-room-telemetry`).
- Problem space definition, candidate architectural designs, user personas, and capability scenarios (owned by Lens A and Lens B).
- Modifying `gentle-ai` review, RDD delivery gates, or external CLI admission contracts.

## Open Questions

- [ ] Should execution stdout/stderr stream chunks be persisted as ledger event rows or stored as worktree log files referenced by URI in the ledger?
- [ ] Should the in-memory pub/sub broker reside directly within `internal/ledger` as an event emitter hook or in a service layer within `internal/serve`?
