# Explore: Control Room UI Shell

**Recommendation:** a modular vanilla ES-module SPA with a layout shell, hash (or History) router, view registry, and shared store, still shipped by `//go:embed` with no npm. Expand HTTP handlers to expose the existing `serve.Model` query surface. Do not lock Server-Sent Events or a six-view catalog in this change — those remain open.

## Problem statement and background

`lucind-ai serve` is a localhost HTTP process (`cmd/lucind-ai/cli.go:674-725`, `internal/serve/server.go:19-53`). The page is a single-purpose approvals inbox (`internal/serve/static/index.html:143-158`): pending cards, approver name, wrong-approval rate, and a copyable `opencode` review command. `app.js` rebuilds that list by string-interpolated `innerHTML` (`internal/serve/static/app.js:22-70`) and polls `GET /api/state` every 2000ms (`internal/serve/static/app.js:96-98`).

The mux exposes only `/`, static files, `/api/state`, and `POST /approvals/{runID}/{laneID}` (`internal/serve/handlers.go:36-118`). `/api/state` returns `ServerState`: approver, `approver_rate`, `opencode_command`, and pending approvals (`internal/serve/handlers.go:79-85`, `internal/serve/handlers.go:120-146`). There is no `/api/stream`.

`serve.Model` already queries features, attempts, leases, overlap evidence, reconciliations, candidates, and audit events (`internal/serve/model.go:14-343`, `internal/serve/model.go:127-343`). The web handler never constructs it; those rows live in the ledger schema (`internal/ledger/schema.go:18-180`) but are invisible in the browser.

There is no persistent nav, main outlet, or view lifecycle. Hash or path routing cannot switch Approvals vs Features vs runs. Constraints that must survive: loopback-only bind (`internal/serve/server.go:14-22`, `internal/serve/server.go:57-73`, `cmd/lucind-ai/cli.go:691-694`), Go single-binary embed (`internal/serve/static.go:8-18`, `Makefile:7-8`), and individual decisions — JSON arrays and bulk fields return 400 (`internal/serve/handlers.go:161-176`, `docs/prd.md:232-234`).

## Candidate approaches

### 1. Modular vanilla ES modules + client routing (recommended)

Keep `//go:embed static/*` (`internal/serve/static.go:8-18`). Split the client into `shell.js`, a router (`#/approvals`, `#/features`, …), shared tokens/components, and isolated view modules. Add REST handlers over `Model` (`internal/serve/model.go:127-343`).

- **Pros:** no Node; native packaging; edit-JS-and-rebuild-Go workflow; tiny footprint; loopback unchanged (`internal/serve/server.go:20-22`). Matches `lucind-checks.sh:1-4` and `go.mod:1-17` (Go-only).
- **Cons:** manual DOM and listener cleanup; custom pub/sub for cross-view state.
- **Feasibility:** High. Extends the embed pipeline and query methods that already exist.

### 2. Go `html/template` + HTMX multi-page

Server renders shell and fragments; HTMX swaps partials. Views become Go-testable.

- **Pros:** no JS bundler; server-authoritative state; `httptest` already in `internal/serve/server_test.go`.
- **Cons:** HTML generation couples to Go; extra round-trips for tab switches; fights the existing JSON API (`internal/serve/handlers.go:120-146`); HTMX must be vendored (CDN is out of scope).
- **Feasibility:** Medium. Stdlib can do it; it rewrites `app.js` and the mux.

### 3. Preact/Solid + Vite → `static/dist/` embed

Declarative components, typed clients, ecosystem widgets.

- **Pros:** reactivity and routing for free; clear API/UI split; `embed.FS` can hold a dist folder (`internal/serve/static.go:8-10`).
- **Cons:** Node/npm in a repo whose Makefile and checks are `go install` / `go test` only (`Makefile:1-9`, `lucind-checks.sh:1-4`, `docs/prd.md:219-222`).
- **Feasibility:** Low for this change. Lens B requires zero npm; lens C forbids Node, bundlers, and frontend frameworks. Tooling friction is a product constraint, not a later cleanup.

## User and capability impact

- **Operators** run `lucind-ai serve --addr 127.0.0.1:7433` (`cmd/lucind-ai/cli.go:683`) and today see only the approvals inbox. Dispatch remains `lucind-ai run` (`cmd/lucind-ai/cli.go:95-98`). The shell is how that same loopback process becomes a multi-view console.
- **Approvers** still decide one card at a time (`internal/serve/static/app.js:72-89`, `internal/serve/handlers.go:87-115`) with evidence checks (`internal/serve/static/app.js:12-20`).
- **Developers/auditors** would finally see `Model` data in the browser (`internal/serve/model.go:128-343`).

Capabilities this change should introduce: a layout shell (header, nav, outlet, status); client routing with mount/unmount; a shared store that can keep serving `/api/state` (`internal/serve/handlers.go:79-85`); chrome that at least shows connection health plus the existing approver rate (`internal/serve/handlers.go:130-134`); REST (or tab-scoped) reads of `Model`. Mutation surface on the web stays individual approvals unless a later change says otherwise.

Named views differ across drafts (see notes). Treat the shell as a **registry**; do not freeze Fleet / DAG Canvas / SDD Flows / Timeline as the set.

## Scenarios and use cases

1. **Shell load.** Operator opens `http://127.0.0.1:7433/`. `GET /` returns embedded `index.html` (`internal/serve/handlers.go:39-77`, `internal/serve/static.go:8-18`). Shell paints chrome and mounts a default registered view without a full reload.
2. **Live-enough updates.** While `lucind-ai run` is in flight (`cmd/lucind-ai/cli.go:95-98`), the store refreshes. Today that is 2s polling of `/api/state`. Ledger `events.type` currently allows `run_started`, `lane_registered`, `lane_status_changed`, `lane_note`, `barrier_released`, `run_ended` (`internal/ledger/schema.go:34-42`). Push vs poll is not decided.
3. **Transport loss.** If a future SSE socket drops, the client must still be able to poll `/api/state` (`internal/serve/static/app.js:1-10`, `internal/serve/handlers.go:79-85`) and show reconnecting vs connected. That fallback is the only transport that exists today.
4. **View navigation.** Hash or nav click unmounts the previous view (timers/listeners), mounts the next, and binds it to `Model` queries (`internal/serve/model.go:128-343`).
5. **Individual approval.** Valid evidence (`internal/serve/static/app.js:12-20`, `internal/ledger/schema.go:45-56`) → `POST /approvals/{runID}/{laneID}` (`internal/serve/handlers.go:87-115`) → bulk body rejected (`internal/serve/handlers.go:161-176`) → card leaves pending; rate refresh uses `ApproverRate` (`internal/serve/handlers.go:130-134`).
6. **Non-loopback refused.** `lucind-ai serve --addr 0.0.0.0:7433` hits `IsLoopback` (`cmd/lucind-ai/cli.go:691-694`) and `ErrNonLoopback` (`internal/serve/server.go:14-22`); process exits without serving.

## Technical risks and trade-offs

| Risk | Severity | Seam |
|---|---|---|
| SQLite read load if every tab dumps the ledger on a 2s tick while batches write | Medium | WAL + `busy_timeout(5000)` (`internal/ledger/ledger.go:162-185`); unbounded `Model` SELECTs (`internal/serve/model.go:128-250`); concurrent lanes (`internal/run/batch.go:110-180`) |
| Full-list `innerHTML` rebuilds lose scroll/focus | Medium | `app.js:22-70`, `app.js:96-98` |
| UI mutations that skip CLI lease/reconcile state machines | High | Only web write today is decide (`internal/serve/handlers.go:87-115`) |
| Loopback erosion / DNS rebinding | High | Listen-address `IsLoopback` only (`internal/serve/server.go:16-22`, `internal/serve/server.go:57-73`); no HTTP `Host` check |
| XSS from agent evidence and candidate `Output` via `innerHTML` | Medium | `escapeHtml` exists (`internal/serve/static/app.js:91-94`); IDs are interpolated into `onclick` unescaped (`app.js:56-65`); candidate output is a string field (`internal/serve/model.go:102-115`) |
| HTTP tests cover approvals, not `Model` JSON routes | Low | `internal/serve/server_test.go`; `internal/serve/model_test.go` tests the model off-HTTP |

| Choice | Advantage | Cost |
|---|---|---|
| Poll `/api/state` vs stdlib SSE | Poll is what `app.js:97` already does; SSE avoids query spam | Poll hammers SQLite; SSE needs fan-out and disconnect hygiene |
| Monolithic `/api/state` vs granular `/api/features` etc. | One round-trip today | Payload grows with logs/audit; granular matches tab scope |
| Monolithic HTML vs tabbed shell | One file now (`index.html:141-163`) | Shell needs structured CSS/JS still embeddable (`static.go:8-18`) |
| Read-only dashboard vs interactive control | CLI keeps git/lease invariants | Read-only means copyable CLI snippets, not buttons for renew/reconcile |

## Potential spikes

1. **Vanilla tab shell + escaping** — hash router, five-ish panes, `textContent`/escaped nodes, no libraries (`internal/serve/static/app.js:22-70`, `internal/serve/static/index.html:141-163`).
2. **Granular REST vs SQLite busy** — concurrent `Model` reads vs batch writes (`internal/serve/model.go:128-250`, `internal/serve/handlers.go:120-146`, `internal/ledger/ledger.go:162-185`, `internal/run/batch.go:110-180`).
3. **Copyable CLI snippets** in read-only feature/reconcile views, next to the existing command box (`internal/serve/static/index.html:151-153`, `internal/serve/model.go:72-100`, `cmd/lucind-ai/cli.go:727-750`).

## Success criteria

- [ ] UI assets stay inside `embed.FS` (`internal/serve/static.go:8-18`); `make install` remains the only build (`Makefile:7-8`); no npm.
- [ ] Shell router mounts and unmounts registered views; chrome persists across navigations.
- [ ] Store survives view switches and can refresh from `/api/state` (`internal/serve/handlers.go:79-85`). SSE is not required to pass this change.
- [ ] Chrome shows at least connection/freshness and approver wrong-approval rate (`internal/serve/handlers.go:130-134`).
- [ ] Individual-decision and evidence rules still hold (`internal/serve/handlers.go:161-176`, `internal/serve/static/app.js:12-20`).
- [ ] Loopback-only bind still holds (`internal/serve/server.go:19-22`, `cmd/lucind-ai/cli.go:691-694`).

## Out of scope

- Schema version bump (stays 5, `internal/ledger/schema.go:10`).
- Node, npm, bundlers, React/Vue/Svelte, CDN styles (`internal/serve/static.go:8-18`, `docs/prd.md:219-222`).
- Non-loopback bind, multi-tenant auth, TLS (`internal/serve/server.go:16-22`).
- Bulk approve or skipping per-item decide (`internal/serve/handlers.go:161-176`, `docs/prd.md:232-234`).
- Browser-triggered git mutation, rebase, lease acquire/renew, or reconcile approve/decline.
- Terminal emulation.

## Open questions

- Hash routes (`#/approvals`) vs HTML5 History? Today unknown paths 404 after a static lookup (`internal/serve/handlers.go:39-77`); History needs a catch-all to `index.html`.
- Keep 2s (or tab-scoped) polling, or add stdlib SSE (`/api/stream` vs `/api/events/stream` were both proposed; neither exists)?
- Exact view registry for *this* change vs later `control-room-ui-views` (lists disagree).
- Should views subscribe to store slices or to full snapshots?
- Historical DAG/wave tree vs active runs and pending approvals only?

## Ready for proposal

Yes. Problem, recommended packaging, and invariants are stable. Proposal must leave view inventory and live-transport as decisions (or defer them), not inherit lens B’s locked SSE + six views.
