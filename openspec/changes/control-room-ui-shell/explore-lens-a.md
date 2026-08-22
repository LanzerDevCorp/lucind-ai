# Explore Lens A — Problem & Candidates: Control Room UI Shell

## Problem Space

The `lucind-ai` daemon currently provides a localhost web interface via `lucind-ai serve` (`cmd/lucind-ai/cli.go:674-725`, `internal/serve/server.go:19-53`). However, this interface is hardcoded as a single-purpose approvals queue (`internal/serve/static/index.html:143-158`, `internal/serve/static/app.js:22-70`). It renders pending approval cards and captures approver decisions (`internal/serve/handlers.go:87-115`, `internal/serve/handlers.go:148-211`), but offers no broader visibility or control over engine operations.

While the data layer in `internal/serve/model.go:14-343` and `internal/ledger/schema.go:18-180` already defines rich query models for features, integration attempts, lease status, overlap evidence, reconciliation requests/candidates, and audit events, these models are completely unexposed to the web interface. Handlers in `internal/serve/handlers.go:39-85` only serve a monolithic `/api/state` endpoint (`internal/serve/handlers.go:120-146`) returning pending approvals.

Furthermore, the frontend architecture in `internal/serve/static/` lacks any structural shell:
1. **Monolithic Page**: `index.html` has no persistent layout framework (navigation, header, main view outlet, status bar).
2. **Ad-hoc DOM Mutation**: `app.js:39-69` renders UI via raw HTML string interpolation without component modularity.
3. **No View Routing**: The UI cannot switch views or mount distinct dashboards (e.g., Approvals, Features, Reconciliations, Telemetry, DAG Runs).
4. **Hardcoded Polling**: The client blindly polls `/api/state` every 2000ms (`internal/serve/static/app.js:97`) regardless of active view or state changes.

To transform `lucind-ai serve` into a full-featured operator Control Room, a foundational **UI Shell** is required. The UI Shell must provide an extensible layout shell, client-side routing, shared state/event management, and view registry while strictly maintaining localhost loopback security (`internal/serve/server.go:20-22, 57-73`), zero-external-toolchain Go single-binary distribution (`internal/serve/static.go:8-18`, `Makefile:7-8`), and individual decision invariants (`internal/serve/handlers.go:161-176`).

## Candidate Approaches

### Candidate 1 — Modular Vanilla ES Modules SPA with Client Hash Routing

**Approach**: Retain the zero-dependency embedded static asset pipeline (`internal/serve/static.go:8-18`). Refactor the frontend into modular ES modules: a central `shell.js` orchestrator, a lightweight client-side hash router (`#/approvals`, `#/features`, `#/reconcile`, `#/runs`), shared design tokens and components, and isolated view modules. Backend handlers in `internal/serve/handlers.go:36-118` are expanded to expose RESTful endpoints backed by `internal/serve/model.go:127-343`.
**Pros**: Zero build toolchain or npm dependencies required; 100% native Go single-binary packaging (`//go:embed static/*` in `internal/serve/static.go:8-10`); instant developer workflow without compilation; minimal footprint (<50KB); preserves existing loopback security (`internal/serve/server.go:20-22`).
**Cons**: Requires manual DOM rendering and manual event cleanup across view transitions; complex reactive state synchronization across views must be managed with a custom lightweight pub/sub store.
**Feasibility**: High. Directly extends existing static asset embedding in `internal/serve/static.go:8-18` and integrates directly with query methods already implemented in `internal/serve/model.go:127-343`.

### Candidate 2 — Server-Side Rendered (SSR) Multi-Page App with Go Templates and HTMX

**Approach**: Replace the client-side JavaScript rendering with Go's standard `html/template` package embedded in the binary. Use HTMX to perform dynamic partial DOM swaps and navigation without full page reloads. Server handlers in `internal/serve/handlers.go` render template fragments for the shell layout and individual sub-views, directly querying `internal/serve/model.go:127-343`.
**Pros**: Zero JavaScript build steps; keeps view logic and data binding entirely in Go; simplifies state management by making the server authoritative; unit testable view rendering in standard Go test suites (`internal/serve/*_test.go`).
**Cons**: Tightly couples HTML UI generation into Go backend code; increases server round-trip latency for simple UI transitions; complicates future live streaming telemetry/log viewer requirements; departs from existing REST JSON API patterns (`internal/serve/handlers.go:120-146`).
**Feasibility**: Medium-High. Fully supported by standard library `net/http` and `html/template`, but requires rewriting existing handler routes and replacing `internal/serve/static/app.js` with template structures.

### Candidate 3 — Pre-compiled Lightweight Single-Page App (Preact/Solid.js via Vite)

**Approach**: Develop the UI shell as a modern TypeScript/JSX single-page application using Preact or Solid.js, compiled during development/release via Vite into static assets in `internal/serve/static/dist/` and embedded into the Go binary via `//go:embed` (`internal/serve/static.go:8-10`). Provides declarative components, router, state management, and type-safe API clients.
**Pros**: Declarative reactive components; robust UI ecosystem and accessible component primitives; strong type safety across frontend state; clean separation between frontend presentation and backend API.
**Cons**: Introduces Node.js and npm toolchains into a repository that is otherwise pure Go (`go.mod:1-17`, `Makefile:1-9`, `lucind-checks.sh:1-4`); complicates CI/CD and contributor setup; requires build artifact checking or complex multi-stage builds.
**Feasibility**: Medium. Embedding compiled assets into Go via `embed.FS` is straightforward (`internal/serve/static.go:8-18`), but adding Node.js build dependencies introduces tooling friction contrary to repository conventions.

## Initial Recommendations

Candidate 1 (Modular Vanilla ES Modules SPA with Client Hash Routing) is recommended. It delivers the necessary UI Shell modularity (layout shell, navigation, view lifecycle, router) while strictly honoring the repository's zero-external-build-dependency philosophy (`internal/serve/static.go:8-18`, `lucind-checks.sh:1-4`). It cleanly decouples UI views from backend handlers, exposes the already-implemented query capabilities in `internal/serve/model.go:127-343` through clean REST endpoints, and maintains the strict loopback security constraints (`internal/serve/server.go:20-22`).

## Open Questions

- [ ] Whether client-side routing should use URL hash fragments (`#/approvals`) or the HTML5 History API with a catch-all route handler in `internal/serve/handlers.go:39-77`.
- [ ] Whether periodic background updates should transition from polling (`internal/serve/static/app.js:97`) to Server-Sent Events (SSE) streaming in the serve daemon.
