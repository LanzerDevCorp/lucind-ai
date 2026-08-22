# Exploration: Control Room UI Views

**Recommendation:** Modular REST endpoints with lazy-loaded vanilla JS panels (Candidate 2). Keep the embedded zero-dependency SPA. Do not replace it with server-rendered HTML. Reconciliation mutation in the UI is unresolved (see Open Questions).

## Problem statement and background

`lucind-ai serve` binds a loopback HTTP server (`internal/serve/server.go:19-28`) whose handler serves static assets, `GET /api/state`, and `POST /approvals/{runID}/{laneID}` (`internal/serve/handlers.go:36-118`). `ServerState` carries only approver identity, wrong-approval rate, the opencode command, and pending approvals (`internal/serve/handlers.go:16-21, 120-146`). The embedded page is titled "lucind-ai approvals" (`internal/serve/static/index.html:6, 144`); `app.js` polls `/api/state` every 2s and renders approval cards (`internal/serve/static/app.js:22-70, 96-97`).

Feature-parent integration landed in schema v4 (`internal/ledger/schema.go:95-180`; current `schemaVersion` is 5 at line 10): features (`96-104`), integration attempts with statuses `recorded` through `stale` (`106-120`), expiring leases with monotonic fences (`122-129`), overlap evidence classified `required` / `warning` / `informational` (`131-139`), reconciliation requests and candidates, and `integration_events` (`141-179`). `serve.Model` is a shell-free JSON query surface over that state (`internal/serve/model.go:14-24, 26-125`) with `ListFeatures`/`GetFeature` (`128-164`), `ListAttempts` (`167-188`), `ListLeases` (`206-227`), `ListOverlapEvidence` (`245-266`), `ListReconciliationRequests` (`278-292`), and `ListAuditEvents` (`326-343`).

`NewHandler` takes `*ledger.Ledger`, not `Model` (`internal/serve/handlers.go:36`). Operators query the same Model from the CLI (`cmd/lucind-ai/cli.go:820-915`) and run `reconcile approve|decline|cancel|renew|resolve` from the terminal (`cmd/lucind-ai/cli.go:56, 1043-1065`). The UI cannot show feature lifecycles, leases, overlap evidence, or the reconciliation queue.

This change is the dashboard that puts those records next to lane approvals, without spawning git or shell from the browser.

## Candidate approaches

| Approach | Pros | Cons | Feasibility |
|---|---|---|---|
| **1. Monolithic `/api/state` + tabbed SPA** — extend `ServerState` (`internal/serve/handlers.go:16-21`) with Model aggregates; one poll loop (`internal/serve/static/app.js:96-97`) | Reuses the existing timer and `go:embed` pipeline (`internal/serve/static.go:8-19`) | Every 2s tick ships historical attempts and `evidence_json` (`internal/ledger/schema.go:137`, `internal/serve/model.go:68`) | High — queries already exist (`internal/serve/model.go:128-343`) |
| **2. Modular REST + lazy panels** — `/api/approvals`, `/api/features`, `/api/features/{id}/attempts`, `/api/overlap/{featureID}`, `/api/reconcile/requests`; details fetched on expand | Restricts high-frequency polls to pending approvals (`internal/serve/handlers.go:121`) and leases (`internal/serve/model.go:206-227`); heavy overlap payloads load on demand | More handler dispatch (`internal/serve/handlers.go:87-115` already splits `/approvals/` subpaths) and more JS state | High |
| **3. `html/template` views with form postbacks** — `/approvals`, `/features`, `/reconcile`; HTTP 303 + meta-refresh | Renders Model structs on the server (`internal/serve/model.go:26-125`); less client JSON | Full reloads break the current POST-then-`fetchState` approval flow (`internal/serve/static/app.js:72-89`) and the bulk-reject tests (`internal/serve/server_test.go:1-100`) | Medium |

Candidate 2 is the recommendation: it matches Model's already-granular methods, avoids payload bloat, and keeps the stdlib embedded SPA.

## User and capability impact

Single operator. Five view surfaces:

1. **Batch & wave inspector** — `BatchReport` (`internal/run/batch.go:19-27`), DAG waves (`internal/dag/waves.go:41-70`), lane statuses `pending`/`running`/`done`/`blocked`/`deviated`/`failed` (`internal/lane/status.go:10-16, 31-38`), executors `agy`/`cursor-agent`/`opencode` (`cmd/lucind-ai/cli.go:65-69`), per-lane deadlines (`internal/run/batch.go:40-43`), worktree paths (`internal/worktree/worktree.go:168-238`), barrier release (`internal/barrier/barrier.go:21-60`). `Model` does not query `lanes` today (`internal/serve/model.go:26-125`).
2. **Approvals** — extend the existing cards (`internal/serve/static/app.js:22-69`) with individual decisions and bulk rejection (`internal/serve/handlers.go:161-176`, `openspec/specs/approvals-web-ui/spec.md:26-48`), inline command output / `file:line` (`internal/serve/static/app.js:12-20`, spec `49-66`), personal wrong-approval rate (`internal/serve/handlers.go:120-146`, `docs/prd.md:229-241`), and the opencode command (`internal/serve/handlers.go:139`).
3. **Feature / lease monitor** — feature rows (`internal/ledger/schema.go:96-104`, `internal/feature/feature.go:48-93`, `internal/serve/model.go:27-35`), lease owner and fence (`internal/feature/feature.go:293-360`, `internal/serve/model.go:52-59`), attempt CAS promotion (`internal/run/integrate_feature.go:80-99`, `internal/serve/model.go:37-50`), overlap payloads (`internal/serve/model.go:62-70`).
4. **Reconciliation workspace** — requests with candidates and audit (`internal/reconcile/reconcile.go:35-120`, `internal/serve/model.go:74-92, 101-115, 117-125`). CLI verbs exist (`cmd/lucind-ai/cli.go:56`); HTTP action routes do not (`internal/serve/handlers.go:36-118`).
5. **Lane envelope inspector** — envelope contract (`internal/result/result.schema.json`, including `hard_stops` and `questions`), `allowed_paths` demotion Done→Deviated (`internal/run/run.go:576-654`), preserved worktrees (`internal/run/batch.go:50-52`).

## Scenarios and use cases

1. **Wave progress.** Operator opens the batch view after `ExecuteBatch` (`internal/run/batch.go:29-89`). UI shows lane status, worktree path, per-lane timeout, and barrier outcome (`internal/barrier/barrier.go:21-60`) until release.
2. **Approve with evidence.** Pending items start unselected; bulk POST is 400 (`internal/serve/handlers.go:161-176`, spec `26-48`). Evidence renders only when it looks like command output or `file:line` (`internal/serve/static/app.js:12-20`); rate and opencode command stay visible (`internal/serve/handlers.go:136-140`).
3. **Supervise a feature.** Selecting a feature shows Model status, lease fence, latest attempt, and `evidence_json` (`internal/serve/model.go:27-70`).
4. **Review a reconciliation.** Workspace shows direction, candidate `output`/`checks`, and `CASResult` (`internal/serve/model.go:74-115, 429-439`). Whether the UI mutates via POST or only offers copy-paste CLI is unresolved.
5. **Diagnose a demoted lane.** Envelope status `done` with an out-of-scope path becomes `deviated` (`internal/run/run.go:650-652`); UI names offending paths (`620-650`) and the preserved worktree (`internal/run/batch.go:50-52`).

## Technical risks and trade-offs

| Risk | Severity | Seam | Mitigation |
|---|---|---|---|
| Full DOM rebuild on each poll drops scroll and input | High | `innerHTML` wipe (`internal/serve/static/app.js:45-70, 96-97`) | Keyed card patching |
| XSS from agent output / evidence | High | evidence uses `escapeHtml` (`internal/serve/static/app.js:51-55, 91-94`); candidate `output` is raw (`internal/serve/model.go:109-110`) | `textContent` / `escapeHtml` on every payload |
| Bulk / "approve all" leaking into new views | High | backend already 400s arrays (`internal/serve/handlers.go:161-176`); UI is per-card (`app.js:63-66`) | Keep one-item POST paths |
| Large `evidence_json` / candidate output freezes the tab | Medium | `internal/serve/model.go:68, 109, 245-275` | Lazy-load on expand (favors Candidate 2) |
| Browser clock vs lease/reconcile expiry | Medium | Model emits `ExpiresAt` (`internal/serve/model.go:56, 84`); awaiting→expired uses `time.Now().UTC()` (`354-357`) | Server `remaining_seconds` or `server_time` |

| Dimension | Choice A | Choice B |
|---|---|---|
| Navigation | Hash SPA; one `/` static handler (`internal/serve/handlers.go:39-77`) | Multi-page `/views/*`; reloads tear down the 2s poll |
| Fetching | One `/api/state` (`handlers.go:120-146`) | Per-view `/api/*` (Candidate 2) |
| Evidence | `<pre>` + `escapeHtml` (PRD §8.3, no npm — `docs/prd.md:217-221`) | Custom vanilla diff formatter (~150 lines) |
| Action feedback | Poll-after-POST (`app.js:72-89`) | Optimistic UI with 409 rollback |

## Potential spikes

- **Keyed DOM reconciliation** in `renderState` (`internal/serve/static/app.js:22-70`) proving scroll and open-card state survive a 2s tick.
- **On-demand overlap viewer** fetching `ListOverlapEvidence` (`internal/serve/model.go:245-275`) only on click.
- **Reconciliation card** rendering direction, candidate status, and `deriveCASResult` (`internal/serve/model.go:74-115, 429-439`) against individual CLI actions (`cmd/lucind-ai/cli.go:1067-1100`) without adding bulk controls (`internal/serve/handlers.go:161-176`). Mutation transport (POST vs copy-paste) is the spike's product question, not a given.

## Success criteria

- [ ] Approvals keep individual selection, reject bulk, and render inline evidence (`internal/serve/handlers.go:161-176`, `openspec/specs/approvals-web-ui/spec.md:26-66`).
- [ ] Feature/lease/overlap/reconcile/audit views read through `Model` with no git from the browser (`internal/serve/model.go:14-125`).
- [ ] Batch/wave view reflects ledger lane statuses and barrier release (`internal/lane/status.go:10-16`, `internal/barrier/barrier.go:21-60`); this needs a Model (or equivalent) query over `lanes` that does not exist yet.
- [ ] Envelope inspector surfaces schema fields including `hard_stops` (`internal/result/result.schema.json`, `docs/prd.md:181-185`) and `allowed_paths` demotion (`internal/run/run.go:576-654`).
- [ ] No npm / bundler (PRD §8.3, `docs/prd.md:217-221`).

## Out of scope

- Loopback bind, TLS, listen address — `control-room-serve` (`internal/serve/server.go:1-60`).
- Shell chrome and dark-theme tokens — `control-room-ui-shell` (`internal/serve/static/index.html:1-140`).
- Schema migrations and ledger writes — `control-room-ledger` (`internal/ledger/schema.go:18-180`).
- Live log streaming / executor capture — `control-room-capture`, `control-room-telemetry`.
- React/Vue/Svelte, TypeScript, npm.

## Open questions

- [ ] May the UI POST reconcile approve/decline/cancel/renew/resolve, or only display copy-paste CLI matching `cmd/lucind-ai/cli.go:1043-1065`? Lens B/C assume buttons; Lens A left this open; handlers have no reconcile routes (`internal/serve/handlers.go:36-118`). **Escalated — do not pick here.**
- [ ] Render overlap `evidence_json` (`internal/serve/model.go:68`) as raw JSON, `<pre>` text, or a custom vanilla diff?
- [ ] For lease/reconcile countdowns, emit `remaining_seconds` from Model or a `server_time` for client offset (`internal/serve/model.go:56, 84, 354-357`)?

## Ready for proposal

Yes on architecture (Candidate 2, vanilla SPA, lazy evidence). No on reconciliation mutation until the first open question is answered.
