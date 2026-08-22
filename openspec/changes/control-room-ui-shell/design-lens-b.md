# Design Lens B — Surface & Flow: Control Room UI Shell

## Assumed architecture

Candidate 1 is assumed: a modular vanilla ES-module SPA embedded in `internal/serve` via `//go:embed static/*` with zero Node/npm dependencies. Backend `serve.NewHandler` (`internal/serve/handlers.go:36`) wires `serve.Model` (`internal/serve/model.go:22-24`) to expose read-only GET JSON endpoints for existing model methods (`internal/serve/model.go:128-343`) without altering its signature. Frontend decomposes `internal/serve/static/app.js` into `shell.js`, `router.js`, `store.js`, and views (`views/approvals.js`, `views/features.js`). Web writes stay strictly restricted to existing approval and defect POST endpoints (`internal/serve/handlers.go:87-115`).

## Flow and Invariants

```
Browser ──(1) GET / ──→ Mux Static Handler ──→ StaticFS (index.html, shell.js)
   │
   ├──(2) Hash Change ──→ Hash Router ──(unmount/mount)──→ Active View
   │                                                           │
   ├──(3) 2s Poll ──────→ Store ──(GET /api/state)─────────────┤
   │                                                           ▼
   ├──(4) Model Query ──→ View ───(GET /api/features)────→ Model GET Handlers
   │                                                           │
   └──(5) Single Post ──→ ApprovalsView ──(POST /approvals)────┴──→ SQLite DB
```

1. **Browser ↔ Static Mux (`internal/serve/server.go:19-22`, `57-73`):** Server binds strictly to loopback; assets serve from `embed.FS` (`internal/serve/static.go:8-18`). *Breaks:* Non-loopback addresses return `ErrNonLoopback` (`internal/serve/server.go:14`, `cmd/lucind-ai/cli.go:691-694`).
2. **Router ↔ View Lifecycle (`internal/serve/static/router.js`):** Router unmounts active view before mounting target into `#view-outlet`. *Breaks:* Leaked timers corrupt DOM across route switches (`internal/serve/static/app.js:96-98`).
3. **Store ↔ DOM Patching (`internal/serve/static/store.js`):** Store polls `/api/state` every 2000ms (`internal/serve/handlers.go:79-85`). Views patch DOM without wiping `#view-outlet.innerHTML` (`internal/serve/static/app.js:45-70`). Untrusted fields use `textContent`/`escapeHtml` (`internal/serve/static/app.js:91-94`, `internal/serve/model.go:109`). *Breaks:* Full `innerHTML` wipes lose scroll/focus; unescaped text allows XSS.
4. **View ↔ Model Handlers (`internal/serve/handlers.go:39`):** Model routes are read-only HTTP GET queries backed by `serve.Model` (`internal/serve/model.go:128-343`). *Breaks:* No shell commands or database mutations run.
5. **Approvals ↔ Write Gate (`internal/serve/handlers.go:87-115`):** Writes are strictly `POST /approvals/{runID}/{laneID}` and `/defect`. Bulk payloads (`req.Approvals != nil` or array bodies) return 400 (`internal/serve/handlers.go:161-176`). *Breaks:* Bulk approvals and empty decisions fail.

## Surface Deltas

| Symbol or format | Today (file:line) | Delta | Backward compatible? |
|---|---|---|---|
| `serve.NewHandler` | `internal/serve/handlers.go:36` | Unchanged signature; wires `serve.Model` (`internal/serve/model.go:22`) for GET routes | Yes, Go signature unchanged |
| `serve.ServerState` | `internal/serve/handlers.go:16-21` | Unchanged struct (`Approver`, `ApproverRate`, `OpencodeCommand`, `Approvals`) | Yes, wire schema preserved |
| `GET /api/features` | Absent (`internal/serve/handlers.go:39`) | Added GET route for `[]serve.Feature` JSON (`internal/serve/model.go:27,128`) | Yes, additive read route |
| `GET /api/features/{id}` | Absent (`internal/serve/handlers.go:39`) | Added GET route for `serve.Feature` JSON (`internal/serve/model.go:27,152`) | Yes, additive read route |
| `GET /api/features/{id}/attempts` | Absent (`internal/serve/handlers.go:39`) | Added GET route for `[]serve.Attempt` JSON (`internal/serve/model.go:38,167`) | Yes, additive read route |
| `GET /api/attempts/{id}` | Absent (`internal/serve/handlers.go:39`) | Added GET route for `serve.Attempt` JSON (`internal/serve/model.go:38,191`) | Yes, additive read route |
| `GET /api/leases` | Absent (`internal/serve/handlers.go:39`) | Added GET route for `[]serve.Lease` JSON (`internal/serve/model.go:52,206`) | Yes, additive read route |
| `GET /api/features/{id}/lease` | Absent (`internal/serve/handlers.go:39`) | Added GET route for `serve.Lease` JSON (`internal/serve/model.go:52,230`) | Yes, additive read route |
| `GET /api/features/{id}/overlap` | Absent (`internal/serve/handlers.go:39`) | Added GET route for `[]serve.OverlapEvidence` JSON (`internal/serve/model.go:62,245`) | Yes, additive read route |
| `GET /api/features/{id}/overlap/{hash}` | Absent (`internal/serve/handlers.go:39`) | Added GET route for `serve.OverlapEvidence` JSON (`internal/serve/model.go:62,269`) | Yes, additive read route |
| `GET /api/features/{id}/reconciliations` | Absent (`internal/serve/handlers.go:39`) | Added GET route for `[]serve.ReconciliationRequest` JSON (`internal/serve/model.go:74,278`) | Yes, additive read route |
| `GET /api/reconciliations/{id}` | Absent (`internal/serve/handlers.go:39`) | Added GET route for `serve.ReconciliationRequest` JSON (`internal/serve/model.go:74,295`) | Yes, additive read route |
| `GET /api/reconciliations/{id}/candidates` | Absent (`internal/serve/handlers.go:39`) | Added GET route for `[]serve.ReconciliationCandidate` JSON (`internal/serve/model.go:102,304`) | Yes, additive read route |
| `GET /api/candidates/{id}` | Absent (`internal/serve/handlers.go:39`) | Added GET route for `serve.ReconciliationCandidate` JSON (`internal/serve/model.go:102,316`) | Yes, additive read route |
| `GET /api/features/{id}/events` | Absent (`internal/serve/handlers.go:39`) | Added GET route for `[]serve.AuditEvent` JSON (`internal/serve/model.go:118,326`) | Yes, additive read route |
| `POST /approvals/{runID}/{laneID}` | `internal/serve/handlers.go:87-115` | Unchanged path & payload (`decideRequest` at `internal/serve/handlers.go:23`); anti-bulk 400 kept (`internal/serve/handlers.go:161-176`) | Yes, write contract unchanged |
| `POST /approvals/{runID}/{laneID}/defect` | `internal/serve/handlers.go:104-107` | Unchanged path & payload (`defectRequest` at `internal/serve/handlers.go:31`) | Yes, defect contract unchanged |
| `serve.StaticFS` | `internal/serve/static.go:12-18` | Unchanged signature; embeds static files under `static/*` (`internal/serve/static.go:8`) | Yes, embed API unchanged |
| `lucind-ai serve` CLI flags | `cmd/lucind-ai/cli.go:683-686` | Unchanged flags (`--addr`, `--approver`, `--approval-timeout`) and loopback check (`cmd/lucind-ai/cli.go:691-694`) | Yes, CLI invocation unchanged |

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `internal/serve/handlers.go` | Modify | Registers Model GET routes (`/api/features`, etc.) via `serve.NewModel(l)` (`internal/serve/model.go:22`); sets JS/CSS MIME types | `cmd/lucind-ai/cli.go:715` (`serveDispatch`) and `internal/serve/static/views/features.js` (fetch client) |
| `internal/serve/static/index.html` | Modify | Replaces inbox (`internal/serve/static/index.html:141-163`) with layout shell: header chrome, tabs, `#view-outlet`, and module script import | Browser GET `/` served by `internal/serve/handlers.go:69-76` |
| `internal/serve/static/style.css` | Create | Stylesheet for shell layout, header metrics, nav tabs, cards, and evidence blocks | `internal/serve/static/index.html:7` loaded via `internal/serve/handlers.go:43-51` |
| `internal/serve/static/shell.js` | Create | Modular ES shell bootstrap: initializes store/router, updates header metrics, mounts default route | `internal/serve/static/index.html:160` loaded via `internal/serve/handlers.go:43-51` |
| `internal/serve/static/router.js` | Create | Client hash router & view registry managing `hashchange` events and unmount/mount lifecycle | `internal/serve/static/shell.js` (shell router coordinator) |
| `internal/serve/static/store.js` | Create | Reactive store polling `GET /api/state` (`internal/serve/handlers.go:79-85`) every 2000ms; notifies subscribers | `internal/serve/static/shell.js` and `internal/serve/static/views/approvals.js` (subscribers) |
| `internal/serve/static/views/approvals.js` | Create | Approvals view: in-place card DOM patching, evidence checks (`internal/serve/static/app.js:12-20`), POST decide (`internal/serve/handlers.go:87-115`) | `internal/serve/static/router.js` (mounts on `#/approvals`) |
| `internal/serve/static/views/features.js` | Create | Read-only view querying and rendering feature/reconciliation status from `/api/features` (`internal/serve/handlers.go:39`) | `internal/serve/static/router.js` (mounts on `#/features`) |
| `internal/serve/static/app.js` | Delete | Deprecated monolithic script (`internal/serve/static/app.js:1-98`) replaced by modular ES modules | Superseded by `internal/serve/static/shell.js` in `internal/serve/static/index.html:160` |

## Open Questions

- [ ] Does this change ship `views/features.js` immediately, or shell + Approvals view + Model GET routes, deferring Features UI to `control-room-ui-views`?
- [ ] Should later History routing introduce a wildcard fallback in `NewHandler` (`internal/serve/handlers.go:39-77`), or keep strict hash routing?
- [ ] Should views subscribe to granular store slices or to full store snapshot broadcasts?
- [ ] Should SSE streaming (`/api/stream` via `http.Flusher`) be introduced in a follow-up after measuring multi-tab SQLite load?
