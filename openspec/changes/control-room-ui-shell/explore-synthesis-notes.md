# Synthesis Notes: Control Room UI Shell

## Unresolved Contradictions

1. **View inventory for this change.** Lens A names example dashboards as Approvals, Features, Reconciliations, Telemetry, DAG Runs. Lens B locks six primary views as a success criterion: Fleet, DAG Canvas, SDD Flows, Features, Timeline, Approvals. Lens C’s spike list is Approvals, Features & Leases, Reconciliation, DAG Runs, Audit Stream. The code only has the approvals inbox (`internal/serve/static/index.html:143-158`). Canonical `explore.md` treats the shell as a view registry and does not pick a frozen catalog.

2. **Whether this change commits to SSE.** Lens A leaves polling vs SSE as an open question (`app.js:97` vs a new stream). Lens B writes success criteria and scenarios as if a reactive store must ingest `/api/stream` with EventSource reconnect and `/api/state` fallback. Lens C puts Polling vs SSE in the trade-off matrix and asks the same question, using `/api/events/stream`. No stream handler exists (`internal/serve/handlers.go:36-118`). Canonical `explore.md` keeps both transports in trade-offs/open questions and does not make SSE a success criterion.

## Coverage Gaps

None. All eight exploration-spine items appear in at least one draft. No draft listed a dedicated affected-files inventory; the canonical doc derives that from verified seams rather than inventing a new file set. No draft used the sdd-explore “Ready for Proposal” heading; the one-line verdict in `explore.md` is synthesis judgment, not feedstock.

## Dropped Citations

1. **Lens A: `internal/serve/handlers.go:39-85` “only serve a monolithic `/api/state`.”** Lines 39–77 are `GET /` (index.html + static files, 404 otherwise). `/api/state` is 79–85. Claim dropped; `/api/state` payload claim kept via `handlers.go:79-85` and `120-146`.

2. **Lens B: SSE from `/api/stream` cited as `handlers.go:79-85` and `app.js:1-10`.** Those lines are `GET /api/state` and `fetch('/api/state')`. No `EventSource`, no `/api/stream`. Dropped any claim that dual-transport SSE exists. Polling kept.

3. **Lens B: `lane_progress` “matching event schema” at `internal/ledger/schema.go:34-42`.** CHECK allows `run_started`, `lane_registered`, `lane_status_changed`, `lane_note`, `barrier_released`, `run_ended`. There is no `lane_progress`. Dropped that event type; `lane_status_changed` kept.

4. **Lens B: global chrome (run/lane metrics, session token/cost rollups, connection indicators) cited as `handlers.go:130-134`.** That block is `l.ApproverRate`. `ServerState` is approver, rate, opencode command, approvals (`handlers.go:15-21`, `136-141`). No token or cost fields in serve/ledger. Dropped token/cost and run/lane counters as if this endpoint already serves them. Approver rate kept.

5. **Lens C: CLI lease guards at `internal/feature/feature.go:89-135`.** That span is `Service`, `NewService`, `ValidateParentRef`, and the start of `Create`. `AcquireLease` / `RenewLease` / `ReleaseLease` start at line 283. Citation dropped. Read-only mutation risk kept via `handlers.go:87-115` (the only web write).

6. **Lens C: reconcile mutation/lease guards at `internal/reconcile/reconcile.go:116-168`.** That span is `Service` fields and `NewService`. Mutations are later (`CreateRequest` at 213, `Approve` at 406). Citation dropped.

## Approach Divergence

Lens A is primary: three packaging candidates, recommend vanilla ES modules, expand REST over `Model`, keep loopback and individual decide, leave hash-vs-History and poll-vs-SSE open.

Lens B assumed the vanilla SPA and then treated SSE dual-transport plus a six-view catalog (default Fleet) as given. That cost a Candidate 2/3 analysis and turned open transport/inventory questions into success criteria. Its process question (parallel lenses vs monolithic `sdd-explore`) is orchestration meta and was not copied into `explore.md`. View-module subscription (slices vs snapshots) was kept as an open question.

Lens C assumed zero-build vanilla and a read-only web surface except individual approvals. It never named A’s three candidates; its out-of-scope Node/frameworks line is what makes Candidate 3 unviable. Its spike view list and `/api/events/stream` naming differ from A/B.

**Arbitration (not an unresolved contradiction):** Candidate 3’s feasibility is Low for this change, not Medium as lens A estimated, because B’s embed-only success criterion and C’s Node/framework ban independently forbid it. Candidate 2 stays documented and not recommended. Candidate 1 remains the recommendation.

**Independent convergence:** all three describe today’s UI as an approvals-only inbox; `Model` as an unused (by HTTP) query surface; `embed.FS` + no npm as non-negotiable; loopback and per-item decide as invariants; a modular tabbed/SPA shell as the viable UI shape.
