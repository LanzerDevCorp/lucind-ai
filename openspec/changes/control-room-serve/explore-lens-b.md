# Explore Lens B — Capabilities & Scenarios: Control Room Serve Subsystem

## User & Capability Impact

The `control-room-serve` change expands `lucind-ai serve` from a single-purpose approvals server into the unified backend HTTP daemon and API gateway for the entire Control Room observability and orchestration suite.

- **Impacted Personas**:
  - **Operators / Engineers**: Monitor multi-lane batch executions, inspect DAG state transitions, review inline evidence, and issue individual approvals or feature reconciliations from a single localhost interface.
  - **Control Room UI Frontend**: Consumes unified REST APIs and Server-Sent Events (SSE) streams instead of polling disjoint endpoints (`internal/serve/static/app.js:1-10, 97`).
  - **Automated Workflows / Sub-agents**: Access structured JSON endpoints for execution inspection and reconciliation status without invoking shell subcommands (`internal/serve/model.go:14-19`).

- **Current Capabilities**:
  - Single-purpose web UI and REST API (`/api/state`, `/approvals/{runID}/{laneID}`) focused strictly on pending lane approvals (`internal/serve/handlers.go:36-118`).
  - Read-only data model query methods in `serve.Model` (`internal/serve/model.go:17-343`) that query features, leases, attempts, overlap evidence, and reconciliation requests, but are currently only exposed via terminal CLI commands (`cmd/lucind-ai/cli.go:852-894`).
  - Frontend polling via `setInterval(fetchState, 2000)` (`internal/serve/static/app.js:97`).

- **New & Modified Capabilities**:
  - **Unified Control Room HTTP Daemon**: Extends `lucind-ai serve` (`cmd/lucind-ai/cli.go:674-725`) to host the complete Control Room UI shell and views while preserving strict loopback security (`internal/serve/server.go:12-22, 55-73`).
  - **Expanded REST API Surface**:
    - System State: `/api/v1/state` returning unified state (`internal/serve/handlers.go:16-21`, `internal/serve/model.go:27-125`).
    - Runs & Lanes: `/api/v1/runs` and `/api/v1/runs/{runID}/lanes` querying lane state transitions (`internal/ledger/schema.go:18-32`, `internal/ledger/ledger.go:249-302`).
    - Features & Reconciliation: `/api/v1/features`, `/api/v1/features/{id}`, `/api/v1/reconcile/requests` exposing `serve.Model` queries (`internal/serve/model.go:128-343`).
    - Action Dispatch: Endpoints for individual approval decisions (`internal/serve/handlers.go:148-211`), defect marking (`internal/serve/handlers.go:213-231`), and reconciliation decisions (`cmd/lucind-ai/cli.go:1068-1250`).
  - **Real-Time Live Event Streaming (SSE)**: Provides `/api/v1/events/stream` delivering ledger events (`run_started`, `lane_registered`, `lane_status_changed`, `barrier_released`, `run_ended` from `internal/ledger/schema.go:38-42`) and integration events (`internal/ledger/schema.go:171-180`) via chunked HTTP push.
  - **Preserved Safety Invariants**: Rejection of non-loopback addresses (`internal/serve/server.go:20-22`), rejection of bulk/unselected approvals (`internal/serve/handlers.go:161-183`), and linked worktree execution refusal (`cmd/lucind-ai/cli.go:702-705`).

## Scenarios & Use Cases

### Scenario 1 — Unified State and Approvals Query

- **Context**: Operator starts `lucind-ai serve` (`cmd/lucind-ai/cli.go:675`) with pending lane approvals (`internal/ledger/schema.go:45-56`) and registered features (`internal/ledger/schema.go:96-104`).
- **Action**: UI client performs `GET /api/v1/state` with `Accept: application/json` (`internal/serve/handlers.go:63-66`).
- **Outcome**: Server returns HTTP 200 JSON with approver identity, approver defect rate (`internal/ledger/ledger.go:578-605`), pending approvals with evidence (`internal/serve/handlers.go:120-146`), and registered feature summaries (`internal/serve/model.go:128-149`).

### Scenario 2 — Real-time Event Streaming via Server-Sent Events

- **Context**: A multi-lane run executes via `internal/run` while `lucind-ai serve` is active on loopback (`cmd/lucind-ai/cli.go:717-722`).
- **Action**: Browser opens `GET /api/v1/events/stream` with `Accept: text/event-stream`.
- **Outcome**: Server streams real-time SSE frames as ledger events occur in `internal/ledger/ledger.go:304-332` (`lane_status_changed`, `barrier_released`, `run_ended`) without requiring client-side interval polling (`internal/serve/static/app.js:97`).

### Scenario 3 — Individual Approval Decision Dispatch

- **Context**: Lane approval is awaiting operator review (`internal/serve/handlers.go:87-115`, `internal/ledger/ledger.go:520-547`).
- **Action**: Operator clicks approve, sending `POST /approvals/run-1/lane-1` with `{"decision":"approved","approver":"alice"}` (`internal/serve/handlers.go:148-195`).
- **Outcome**: Server validates single-item payload, records decision via `ledger.Decide` (`internal/ledger/ledger.go:429-460`), broadcasts transition, and returns HTTP 200 `{"ok":true}`.

### Scenario 4 — Anti-Bulk Approval Rejection

- **Context**: Client sends a bulk approval array or composite multi-item payload (`internal/serve/handlers.go:161-176`, `internal/serve/server_test.go:42-93`).
- **Action**: Client issues `POST /approvals/run-1/lane-1` with `[{"decision":"approved"}]`.
- **Outcome**: Server returns HTTP 400 Bad Request with message `bulk approval rejected; decisions must be made individually` (`internal/serve/handlers.go:163`), leaving all approval states unchanged.

### Scenario 5 — Web-Initiated Feature Reconciliation Action

- **Context**: Overlap reconciliation request is in `awaiting` status in the ledger (`internal/serve/model.go:72-92`, `internal/ledger/schema.go:141-154`).
- **Action**: UI client posts `POST /api/v1/reconcile/requests/req-42/approve` with `{ "source": "feat-a", "target": "feat-b" }` (`cmd/lucind-ai/cli.go:1068-1180`).
- **Outcome**: Server verifies request state, delegates to `reconcile.Service.Approve` (`cmd/lucind-ai/cli.go:1166-1176`), creates candidate record (`internal/serve/model.go:101-115`), and returns HTTP 200 with candidate details.

### Scenario 6 — Strict Non-Loopback Binding Rejection

- **Context**: Operator attempts to start server with public address `lucind-ai serve --addr 0.0.0.0:7433` (`cmd/lucind-ai/cli.go:683-694`).
- **Action**: CLI executes `serve.ListenAndServe(ctx, *addr, handler)` (`internal/serve/server.go:19-22`).
- **Outcome**: `serve.IsLoopback` evaluates to `false` (`internal/serve/server.go:57-73`), server returns `serve.ErrNonLoopback` (`internal/serve/server.go:14`), and process terminates with status 1 without opening unauthenticated network ports.

## Success Criteria

- [ ] `lucind-ai serve` binds exclusively to loopback addresses (`127.0.0.1`, `localhost`, `::1`) and returns `ErrNonLoopback` on non-loopback addresses (`internal/serve/server.go:14-22, 57-73`).
- [ ] Server exposes JSON REST endpoints for state (`/api/v1/state`), runs (`/api/v1/runs`), features (`/api/v1/features`), and reconciliation (`/api/v1/reconcile/requests`) using `serve.Model` (`internal/serve/model.go:17-343`).
- [ ] Server exposes an SSE endpoint (`/api/v1/events/stream`) streaming live ledger events (`internal/ledger/schema.go:38-42`) and integration events (`internal/ledger/schema.go:171-180`) with `text/event-stream` headers.
- [ ] Decision endpoints reject bulk approval payloads and unselected decisions with HTTP 400 Bad Request (`internal/serve/handlers.go:161-183`, `internal/serve/server_test.go:42-93`).
- [ ] Server embeds and serves Control Room static assets with appropriate MIME types (`internal/serve/static.go:8-18`, `internal/serve/handlers.go:39-55`).
- [ ] Server shuts down cleanly within 3 seconds upon context cancellation (`internal/serve/server.go:41-52`).
- [ ] Server refuses to start when executed from within a linked worktree (`cmd/lucind-ai/cli.go:702-705`).

## Open Questions

- [ ] Broadcast fan-out architecture: Should SSE event streaming rely on an in-memory pub/sub broker inside `internal/serve` or a lightweight SQLite ledger polling loop on `events` (`internal/ledger/schema.go:34-44`)?
- [ ] Contract Precedence Note: `~/.claude/skills/sdd-explore/SKILL.md` specifies writing a single monolithic `exploration.md` and phase summary, which is intentionally superseded by this packet's parallel partitioned lens contract (`openspec/changes/control-room-serve/explore-lens-b.md`).
