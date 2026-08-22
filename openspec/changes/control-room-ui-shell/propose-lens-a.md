# Proposal Lens A — Candidate & Approach: Control Room UI Shell

## Selected Candidate & Approach

**Selected Candidate:** Candidate 1 — Modular Vanilla ES-Module Single-Page Application (SPA) with embedded static assets (`internal/serve/static.go:8-18`), client-side routing, a centralized reactive store, and read-only REST endpoints exposing the existing `serve.Model` (`internal/serve/model.go:14-343`) query surface.

**Core Approach:**
1. **SPA Shell Layout & Global Chrome**: Replace the single-purpose approvals inbox (`internal/serve/static/index.html:141-163`) with a persistent SPA layout shell. The shell introduces a persistent header displaying connection health, active approver identity, and wrong-approval rate (`internal/serve/handlers.go:130-141`), a navigation tab bar, and a main dynamic outlet container (`#view-outlet`).
2. **Client-Side Routing & View Registry Lifecycle**: Implement a lightweight hash-based router (`#/approvals`, `#/features`, etc.) with a view registry contract (`mount(container, store)`, `unmount()`). When transitioning routes, the router cleanly tears down active view listeners and polling timers (`internal/serve/static/app.js:96-98`), mounts the target view module into the outlet, and performs targeted DOM updates to prevent focus and scroll loss (`internal/serve/static/app.js:22-70`).
3. **Centralized Client Store**: Introduce `store.js` as a shared client-side cache and pub/sub bus. The store persists across view transitions, ingests `/api/state` (`internal/serve/handlers.go:79-85`) via HTTP polling (`internal/serve/static/app.js:1-10`), and notifies registered view subscribers.
4. **REST Read-Only Model Exposure**: Extend `serve.NewHandler` (`internal/serve/handlers.go:36-118`) to mount read-only JSON endpoints exposing `serve.Model` methods (`internal/serve/model.go:128-343`) for features (`ListFeatures`, `GetFeature` at `128-164`), integration attempts (`ListAttempts` at `167-188`), leases (`ListLeases` at `206-227`), overlap evidence (`ListOverlapEvidence` at `245-266`), reconciliation requests/candidates (`ListReconciliationRequests` at `278-292`), and audit events (`ListAuditEvents` at `326-343`).
5. **Preservation of System Invariants**:
   - *Zero-Build Binary Embedding*: Deliver all assets (HTML, CSS, vanilla ES modules) via `//go:embed static/*` (`internal/serve/static.go:8-18`). No Node.js, npm, or bundler toolchains are introduced (`Makefile:7-8`, `lucind-checks.sh:1-4`, `docs/prd.md:219-222`).
   - *Strict Loopback Security*: Enforce localhost-only binding in `serve.ListenAndServe` (`internal/serve/server.go:14-22,57-73`) and CLI flags (`cmd/lucind-ai/cli.go:683-694`).
   - *Individual Decisions & Accountability*: Preserve individual item approvals (`POST /approvals/{runID}/{laneID}` at `internal/serve/handlers.go:87-115`), mandatory evidence inspection (`internal/serve/static/app.js:12-20`), and strict HTTP 400 rejection of bulk arrays or multi-item decision payloads (`internal/serve/handlers.go:161-176`, `docs/prd.md:229-240`). All new UI views remain strictly read-only; browser-initiated write operations outside individual approvals remain prohibited.

**Why this solves the problem:**
Currently, `lucind-ai serve` only renders an approvals inbox (`internal/serve/static/index.html:143-158`) while SQLite ledger data for features, attempts, leases, and reconciliations (`internal/ledger/schema.go:18-180`) remains trapped in `serve.Model` (`internal/serve/model.go:14-24`), inaccessible to browser operators. Candidate 1 unlocks this telemetry as a modular, extensible multi-view console without introducing external dependencies or compromising loopback safety.

## Conceptual Changes & Architecture Rationale

1. **From Single-Purpose Inbox to Extensible View Registry**:
   The existing client hardcodes a single approvals list directly into the DOM via string-interpolated `innerHTML` (`internal/serve/static/app.js:22-70`, `internal/serve/static/index.html:154-158`). The proposal introduces a modular SPA architecture where views (Approvals, Features, etc.) implement a standard lifecycle contract (`mount`/`unmount`) managed by a central registry, allowing seamless extension without bloating `index.html`.

2. **Exposing `serve.Model` as Read-Only REST Endpoints**:
   `serve.Model` was designed as a shell-free query abstraction over the SQLite ledger (`internal/serve/model.go:14-24`), returning structured DTOs (`Feature`, `Attempt`, `Lease`, `ReconciliationRequest`, `AuditEvent` at `internal/serve/model.go:26-125`). However, `serve.NewHandler` (`internal/serve/handlers.go:36-118`) only mounted `GET /api/state` and `POST /approvals/`. Exposing read-only REST endpoints over `Model` methods connects the browser to existing ledger state without executing shell or git subprocesses.

3. **Client State Persistence & DOM Patching**:
   Replacing full-container `innerHTML` replacements (`internal/serve/static/app.js:45-70`) with structured DOM patching and a persistent client store (`store.js`) preserves user focus, selection, and scroll position during 2000ms polling intervals (`internal/serve/static/app.js:96-98`), while sanitizing untrusted candidate output via `textContent` and `escapeHtml` (`internal/serve/static/app.js:91-94`, `internal/serve/model.go:102-115`).

4. **Architecture Rationale**:
   - *Zero-Dependency Go Toolchain*: Retaining vanilla ES modules served directly from Go's `embed.FS` (`internal/serve/static.go:8-18`) keeps `go install ./cmd/lucind-ai` (`Makefile:7-8`) and `go test ./...` (`lucind-checks.sh:1-4`) as the sole build and check commands, avoiding npm supply-chain bloat.
   - *Read-Only Control Plane*: Keeping browser views read-only and restricting web writes strictly to individual approvals (`internal/serve/handlers.go:87-115`) ensures that stateful git and lease operations (branch rebases, lease renewals, reconciliations) remain guarded by CLI concurrency controls and fence counters (`internal/ledger/schema.go:106-170`, `internal/serve/model.go:44,55,95-99`).

## Alternatives Considered & Rejected

- **Candidate 2 — Go `html/template` + HTMX Multi-Page Application**:
  - *Description*: Server renders shell layout and HTML fragments via Go `html/template`; HTMX attributes swap DOM partials during navigation and polling.
  - *Rejection Reason*: Couples UI view markup tightly to Go backend code, requires vendoring HTMX into the binary, departs from the established JSON DTO API pattern (`internal/serve/handlers.go:120-146`, `internal/serve/model.go:26-125`), adds round-trip latency on tab navigation, and complicates client-side state caching.

- **Candidate 3 — Preact / Solid.js / React + Vite SPA Build**:
  - *Description*: Build a modern component-driven SPA using Preact/Solid and Vite, emitting compiled bundles to `internal/serve/static/dist/` for embedding (`internal/serve/static.go:8-18`).
  - *Rejection Reason*: Violates repository-wide product and build constraints (`docs/prd.md:219-222`, `Makefile:7-8`, `lucind-checks.sh:1-4`). Adding Node.js, npm dependencies, build scripts, and transpilers creates unnecessary operational overhead and supply-chain surface for a lightweight loopback console.

## Open Questions

- [ ] **Hash vs. History Routing**: Hash routing (`#/approvals`, `#/features`) works natively with embedded static file handlers (`internal/serve/handlers.go:39-77`). If HTML5 History API navigation is desired in the future, should a wildcard route fallback (`/*` -> `index.html`) be added to `serve.NewHandler`?
- [ ] **Live Telemetry Transport Evolution**: Immediate implementation relies on HTTP polling of `/api/state` (`internal/serve/static/app.js:96-98`, `internal/serve/handlers.go:79-85`). Should a standard library Server-Sent Events endpoint (`/api/stream`) using `http.Flusher` be introduced in a follow-up iteration to reduce SQLite read polling load?
- [ ] **Initial View Scope**: Confirm whether this initial change delivers the shell foundation with the Approvals view and a read-only Features view, deferring specialized views (Reconciliations, Leases, Timeline) to a follow-up `control-room-ui-views` packet.
- [ ] **Process Drift Note**: The canonical `sdd-propose` skill (`~/.claude/skills/sdd-propose/SKILL.md:90-158`) defines a single monolithic `proposal.md` authoring phase. In accordance with packet instructions, this work executes as a 3-lane parallel proposal decomposition where Lens A authors candidate selection and technical architecture.
