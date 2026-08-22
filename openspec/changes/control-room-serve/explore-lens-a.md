# Explore Lens A — Problem & Candidates: Control Room Serve Subsystem

## Problem Space

The current `lucind-ai serve` command ([`cmd/lucind-ai/cli.go:674-725`](cmd/lucind-ai/cli.go#L674-L725)) operates strictly as a single-purpose approvals web server. It starts a localhost-only HTTP server using [`serve.ListenAndServe`](internal/serve/server.go#L19-L53) that serves static assets from embedded storage ([`internal/serve/static.go:8-18`](internal/serve/static.go#L8-L18)) and exposes a single JSON state endpoint `GET /api/state` ([`internal/serve/handlers.go:79-85,120-146`](internal/serve/handlers.go#L79-L85)).

The frontend client continuously polls `/api/state` on a 2-second interval ([`internal/serve/static/app.js:96-97`](internal/serve/static/app.js#L96-L97)), querying pending approvals ([`internal/ledger/ledger.go:705-718`](internal/ledger/ledger.go#L705-L718)) and approver accuracy rates ([`internal/ledger/ledger.go:797-815`](internal/ledger/ledger.go#L797-L815)). Decisions are posted individually via `POST /approvals/{runID}/{laneID}` ([`internal/serve/handlers.go:87-115,148-211`](internal/serve/handlers.go#L87-L115)).

As `lucind-ai` orchestrates multi-agent DAGs, parallel fan-out lanes, and feature-parent lifecycles, this architecture presents three major limitations:

1. **Unexposed Model Queries**: [`serve.Model`](internal/serve/model.go#L14-L25) already implements comprehensive query methods for features ([`internal/serve/model.go:128-149`](internal/serve/model.go#L128-L149)), integration attempts ([`internal/serve/model.go:167-188`](internal/serve/model.go#L167-L188)), leases ([`internal/serve/model.go:206-227`](internal/serve/model.go#L206-L227)), overlap evidence ([`internal/serve/model.go:245-266`](internal/serve/model.go#L245-L266)), and reconciliation requests ([`internal/serve/model.go:278-292`](internal/serve/model.go#L278-L292)). None of these are exposed via HTTP in [`internal/serve/handlers.go:36-118`](internal/serve/handlers.go#L36-L118); they are only consumed by CLI commands like `runFeatureStatus` ([`cmd/lucind-ai/cli.go:852-879`](cmd/lucind-ai/cli.go#L852-L879)).
2. **Missing Operational Telemetry**: The server exposes no endpoints for run execution progress, lane statuses ([`internal/ledger/ledger.go:285-335`](internal/ledger/ledger.go#L285-L335)), lane state snapshots ([`internal/ledger/ledger.go:337-364`](internal/ledger/ledger.go#L337-L364)), or structured event logs ([`internal/ledger/ledger.go:490-530`](internal/ledger/ledger.go#L490-L530)).
3. **Inefficient State Sync**: Polling `/api/state` repeatedly queries SQLite ledger tables ([`internal/ledger/schema.go:18-57`](internal/ledger/schema.go#L18-L57)), creating database overhead and latency unsuitable for real-time executor streaming.

Motivation: Evolve the serve daemon into a high-performance Control Room API that exposes granular read models, real-time event streaming, and secure operational controls while enforcing loopback isolation ([`internal/serve/server.go:19-22`](internal/serve/server.go#L19-L22)).

## Candidate Approaches

### Candidate 1 — Granular REST API with Server-Sent Events (SSE) Stream

**Approach**: Extend [`internal/serve/handlers.go`](internal/serve/handlers.go#L36-L118) with RESTful resource endpoints (`/api/v1/runs`, `/api/v1/lanes`, `/api/v1/features`, `/api/v1/reconciliations`, `/api/v1/approvals`) wired to [`serve.Model`](internal/serve/model.go#L17-L24) and [`ledger.Ledger`](internal/ledger/ledger.go#L217-L222), alongside an SSE stream endpoint (`/api/v1/events/stream`) using `http.Flusher` to push live event log entries ([`internal/ledger/schema.go:34-43`](internal/ledger/schema.go#L34-L43)).
**Pros**:
- Zero external dependencies: uses standard Go `net/http` and `http.Flusher`.
- Resource-oriented endpoints allow granular UI caching without payload bloat.
- Native browser support for SSE (`EventSource`) with automatic reconnection and low overhead.
- Direct reuse of existing `serve.Model` query methods ([`internal/serve/model.go:128-343`](internal/serve/model.go#L128-L343)).
**Cons**:
- Unidirectional SSE requires standard HTTP POST for mutations (like approvals in [`internal/serve/handlers.go:148-211`](internal/serve/handlers.go#L148-L211)).
- Server must manage subscriber channel lifecycles and clean up disconnected clients.
**Feasibility**: High. stdlib `http.ServeMux` and `http.Flusher` fit cleanly inside [`serve.ListenAndServe`](internal/serve/server.go#L19-L53), and `serve.Model` already implements the data layer.

### Candidate 2 — Monolithic Hierarchical Snapshot Polling (Extended `/api/state`)

**Approach**: Expand [`ServerState`](internal/serve/handlers.go#L16-L21) and [`serveStateJSON`](internal/serve/handlers.go#L120-L146) to aggregate all runs, active lanes, feature leases, and pending approvals into a single composite JSON response polled periodically (1-2s) by the frontend ([`internal/serve/static/app.js:96-97`](internal/serve/static/app.js#L96-L97)).
**Pros**:
- Minimal route changes and simple, stateless request/response lifecycle.
- Straightforward client-side state replacement on each tick.
**Cons**:
- Heavy SQLite read load and JSON serialization on every poll tick as history grows in [`internal/ledger/schema.go:18-57`](internal/ledger/schema.go#L18-L57).
- Polling interval latency prevents responsive agent telemetry or stdout streaming.
- Increased risk of SQLite WAL read contention against concurrent lane writes ([`internal/ledger/ledger.go:366,452`](internal/ledger/ledger.go#L366-L384)).
**Feasibility**: Medium. Easy to implement using existing [`serve.Model`](internal/serve/model.go#L128-L343) and [`ledger.Ledger`](internal/ledger/ledger.go#L285,490), but scales poorly under parallel multi-agent workloads.

### Candidate 3 — Full-Duplex WebSocket Server Gateway

**Approach**: Add a bidirectional WebSocket endpoint (`/ws`) using an external package (such as `nhooyr.io/websocket`) to handle real-time event streaming, state subscriptions, and client commands over a persistent TCP socket.
**Pros**:
- Bidirectional channel combines event streaming and action execution on a single socket.
- Low per-message framing overhead for high-frequency progress updates.
**Cons**:
- Adds external third-party dependencies to `go.mod`, breaking the zero-dependency standard library pattern.
- High connection management complexity (heartbeats, reconnections, multiplexed message frames).
- Difficult to debug with standard tools (`curl`) compared to HTTP REST endpoints.
**Feasibility**: Low to Medium. Viable, but introduces unnecessary complexity and dependencies over stdlib HTTP + SSE.

## Initial Recommendations

Candidate 1 (Granular REST API with Server-Sent Events) is strongly recommended. It leverages standard Go library primitives (`net/http`, `http.Flusher`) with zero new dependencies, directly surfaces the existing query methods in [`serve.Model`](internal/serve/model.go#L128-L343) and [`ledger.Ledger`](internal/ledger/ledger.go#L285-L530), and provides efficient, low-latency updates via SSE while maintaining loopback safety ([`internal/serve/server.go:19-22`](internal/serve/server.go#L19-L22)).

## Open Questions

- [ ] Should the SSE stream (`/api/v1/events/stream`) tail the SQLite `events` table ([`internal/ledger/schema.go:34-43`](internal/ledger/schema.go#L34-L43)) by sequence ID with poll backoff or subscribe to an in-process event channel fed by [`ledger.AppendEvent`](internal/ledger/ledger.go#L366-L384)?
- [ ] Should mutation or run-dispatch endpoints be added to `serve`, or remain gated behind explicit flags (e.g. `--enable-dispatch`) to protect the unauthenticated loopback interface ([`cmd/lucind-ai/cli.go:683-694`](cmd/lucind-ai/cli.go#L683-L694))?
- [ ] SDD exploration fan-out: the SDD skill contract (`~/.claude/skills/sdd-explore/SKILL.md`) defines a single `explore.md`, whereas this execution utilizes a three-lens parallel exploration producing `explore-lens-a.md` (superseded by packet contract).
