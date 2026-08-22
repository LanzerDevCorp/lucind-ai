# Explore Lens B — Capabilities & Scenarios: Control Room UI Shell

## User & Capability Impact

The Control Room UI Shell transforms the existing single-purpose approvals inbox in [internal/serve/static/index.html:142-158](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/static/index.html#L142-L158) and [internal/serve/static/app.js:1-98](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/static/app.js#L1-L98) into an extensible single-page application (SPA) shell.

### Affected Users & Roles
- **Local Operators**: Driving `lucind-ai serve` ([cmd/lucind-ai/cli.go:675](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/cmd/lucind-ai/cli.go#L675)) to monitor multi-agent runs, dispatch waves, and manage worktrees.
- **Approvers**: Reviewing lane outputs and submitting individual approval/rejection decisions ([internal/serve/handlers.go:36](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/handlers.go#L36), [internal/serve/static/app.js:72-89](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/static/app.js#L72-L89)).
- **Developers & Auditors**: Inspecting feature integration attempts, leases, and reconciliation states queryable through `Model` ([internal/serve/model.go:128-343](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/model.go#L128-L343)).

### Introduced & Modified Capabilities
1. **Core SPA Navigation Shell**: Provides client-side routing, navigation headers, and container mounting for 6 primary views (Fleet, DAG Canvas, SDD Flows, Features, Timeline, Approvals), replacing static markup ([internal/serve/static/index.html:142-158](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/static/index.html#L142-L158)).
2. **Reactive Dual-Transport Store (`store.js`)**: Implements an in-memory reactive state manager ingesting real-time Server-Sent Events from `/api/stream` while retaining automatic fallback polling to `/api/state` ([internal/serve/handlers.go:79-85](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/handlers.go#L79-L85), [internal/serve/static/app.js:1-10](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/static/app.js#L1-L10)) during transport disconnects.
3. **Global Status & Telemetry Header**: Renders a persistent top bar with active run/lane status indicators, session token/cost rollups, connection indicators, and approver accuracy ([internal/serve/handlers.go:130-134](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/handlers.go#L130-L134)).
4. **Zero-Build Vanilla Asset Pipeline**: Ships vanilla ES modules and modern CSS via Go embedding ([internal/serve/static.go:8-18](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/static.go#L8-L18)), ensuring `make install` remains the single build step.
5. **Invariant Preservation**: Preserves strict loopback binding ([internal/serve/server.go:19-22](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/server.go#L19-L22), [cmd/lucind-ai/cli.go:691-694](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/cmd/lucind-ai/cli.go#L691-L694)), evidence validation ([internal/serve/static/app.js:12-20](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/static/app.js#L12-L20)), and rejection of bulk approval requests ([internal/serve/handlers.go:161-176](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/handlers.go#L161-L176)).

## Scenarios & Use Cases

### Scenario 1 — Initial SPA Shell Load and SSE Connection

- **Context**: Operator starts server using `lucind-ai serve --addr 127.0.0.1:7433` ([cmd/lucind-ai/cli.go:683](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/cmd/lucind-ai/cli.go#L683)) and opens `http://127.0.0.1:7433/` in a browser.
- **Action**: Browser requests `/` ([internal/serve/handlers.go:39-77](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/handlers.go#L39-L77)), loads embedded `index.html` and scripts via `StaticFS()` ([internal/serve/static.go:8-18](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/static.go#L8-L18)), and initializes `store.js` connecting to `/api/stream`.
- **Outcome**: UI shell mounts the top bar, sets connection status to "Connected (SSE)", fetches initial state snapshot, and mounts the default Fleet view without full-page reloads.

### Scenario 2 — Live Telemetry Push Ingestion

- **Context**: UI shell is connected via SSE while `lucind-ai run` executes active lane dispatches ([cmd/lucind-ai/cli.go:95-98](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/cmd/lucind-ai/cli.go#L95-L98)).
- **Action**: Server pushes ledger event deltas (`lane_status_changed`, `lane_progress`) matching event schema ([internal/ledger/schema.go:34-42](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/ledger/schema.go#L34-L42)).
- **Outcome**: `store.js` updates in-memory state, top-bar lane counters refresh reactively, and active view subscribers update DOM elements without resetting operator scroll or selections.

### Scenario 3 — Stream Disconnection with Fallback Polling

- **Context**: SSE connection drops due to network hiccup or server restart.
- **Action**: Client EventSource detects connection loss.
- **Outcome**: UI shell switches connection indicator to "Reconnecting", begins exponential backoff reconnection, and activates fallback 2s polling to `/api/state` ([internal/serve/static/app.js:1-10](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/static/app.js#L1-L10), [internal/serve/handlers.go:79-85](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/handlers.go#L79-L85)) until the stream recovers.

### Scenario 4 — Client-Side View Navigation Lifecycle

- **Context**: Operator navigates between different console tools (e.g. from Fleet view to Features or Approvals).
- **Action**: Operator clicks a navigation tab or changes URL hash (e.g. `#/features`).
- **Outcome**: Shell router unmounts the previous view, cleans up view-specific timers/listeners, mounts the target view container, and binds it to domain queries backed by `Model` ([internal/serve/model.go:128-343](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/model.go#L128-L343)).

### Scenario 5 — Submitting Individual Approval from Shell

- **Context**: A pending approval is displayed with valid command output or `file:line` evidence ([internal/serve/static/app.js:12-20](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/static/app.js#L12-L20), [internal/ledger/schema.go:45-56](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/ledger/schema.go#L45-L56)).
- **Action**: Approver clicks "Approve" for an individual card.
- **Outcome**: Shell dispatches POST to `/approvals/{runID}/{laneID}` ([internal/serve/handlers.go:87-115](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/handlers.go#L87-L115)), individual decision constraint succeeds ([internal/serve/handlers.go:161-176](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/handlers.go#L161-L176)), ledger records approval, card removes from pending list, and approver rate updates ([internal/serve/handlers.go:130-134](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/handlers.go#L130-L134)).

### Scenario 6 — Rejection of Non-Loopback Binding

- **Context**: User attempts to start server bound to external interface `lucind-ai serve --addr 0.0.0.0:7433`.
- **Action**: `serveDispatch` executes and invokes `serve.IsLoopback` ([cmd/lucind-ai/cli.go:691-694](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/cmd/lucind-ai/cli.go#L691-L694)).
- **Outcome**: Server returns `ErrNonLoopback` ([internal/serve/server.go:14-22](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/server.go#L14-L22)) and exits with code 1 without exposing the unauthenticated UI shell to the network.

## Success Criteria

- [ ] UI shell assets embed into Go binary via `embed.FS` ([internal/serve/static.go:8-18](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/static.go#L8-L18)) requiring zero npm or Node.js build dependencies.
- [ ] Reactive store (`store.js`) maintains state across views and ingests SSE push deltas from `/api/stream` with automatic reconnection and fallback polling to `/api/state` ([internal/serve/handlers.go:79-85](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/handlers.go#L79-L85)).
- [ ] Top bar displays real-time connection status, aggregate active run/lane metrics, and approver wrong-approval rate ([internal/serve/handlers.go:130-134](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/handlers.go#L130-L134)).
- [ ] Client router supports view mounting and unmounting for 6 primary views (Fleet, DAG Canvas, SDD Flows, Features, Timeline, Approvals).
- [ ] Individual approval invariants ([internal/serve/handlers.go:161-176](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/handlers.go#L161-L176)) and evidence checks ([internal/serve/static/app.js:12-20](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/static/app.js#L12-L20)) remain strictly enforced.
- [ ] Strict loopback-only binding invariant ([internal/serve/server.go:19-22](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/internal/serve/server.go#L19-L22), [cmd/lucind-ai/cli.go:691-694](file:///home/lanzerdev/git_root/lucind-ai-worktrees/explore-control-room-ui-shell-lens-b/cmd/lucind-ai/cli.go#L691-L694)) is preserved for all serve executions.

## Open Questions

- [ ] Parallel Lens Execution vs Standard Explore Contract: The canonical `sdd-explore` skill describes a monolithic exploration resulting in `exploration.md`, whereas this workflow executes parallel lenses (A/B/C) feeding a synthesis lane.
- [ ] View Ingestion Interface: Should view modules in `control-room-ui-views` subscribe to discrete slice selectors on `store.js` or observe global state snapshot updates?
