# Synthesis Notes: Control Room Serve

## Unresolved Contradictions

None

## Coverage Gaps

None

## Dropped Citations

- Lens A: `/api/v1/runs` “wired to existing query routines” at `internal/ledger/ledger.go:285-358`. Those lines are `Lanes` and `LaneStates`, both requiring `runID`. No `Ledger.Runs()` exists. Endpoint kept as proposed; the “existing Runs query” claim dropped.
- Lens B: `GET /api/v1/features` with `Accept: application/json` at `internal/serve/handlers.go:63-66`. Those lines are the `/` JSON Accept branch calling `serveStateJSON`, not a features route.
- Lens B: unmatched `/api/v1/nonexistent` returns JSON 404 at `internal/serve/handlers.go:53`. That call is stdlib `http.NotFound` (HTML). JSON 404 kept as a proposed requirement without this citation.
- Lens B: non-file UI navigation falls back to `index.html` at `internal/serve/handlers.go:68-77`. Those lines serve `index.html` only for exact `/`. No history fallback exists. SPA fallback dropped (also out of Lens A’s approach; UI-owned).
- Lens B: client disconnect ends the SSE loop via `internal/serve/server.go:41-52`. Those lines are `ListenAndServe` process shutdown (3s `Shutdown` on the listen context), not `r.Context()` on client disconnect. Disconnect requirement kept; this citation dropped. `41-52` retained only for server shutdown.
- Lens C: SSE already tails by primary-key cursor `id > lastID` at `internal/ledger/ledger.go:490-525,892-925`. `Events` filters `WHERE run_id = ?`; `IntegrationEvents` filters `WHERE feature_id = ?`. Neither is an `id > lastID` tail. Cursor query kept as proposed work.
- Lens C: `cmd/lucind-ai/cli_test.go:1908-1917` verifies linked-worktree refusal. That test is `TestServeNonLoopbackAddrRejectedAtCLI` (`0.0.0.0`). Loopback coverage kept; worktree-test claim dropped. Worktree refusal itself remains at `cmd/lucind-ai/cli.go:702-707`.
- Lens C: `internal/serve/server_test.go:136-194` verifies granular REST `/api/v1/*` queries. Those lines are `TestSingleApprovalAndDefectEndpoints` (POST approve/defect). REST tests kept as required new coverage without this citation.
- Lens C: `internal/serve/static_test.go:11-103` verifies MIME types, UI `index.html` fallback, and JSON API 404. Those tests cover absent bulk-approve controls, opencode/evidence copy, unselected items, and evidence validation. MIME/JSON-404 tests kept as required new coverage; this citation dropped.
- Lens C: result envelope unchanged at `.lucind/result.schema.json:1-160`. That path does not exist (`.lucind/` is gitignored). Schema is `internal/result/result.schema.json:1-160`. Claim restated with the real path.

## Scope Divergence

Lens A (authoritative) selected Candidate 1: granular REST + SSE; routes `/api/v1/runs`, `/api/v1/lanes`, `/api/v1/features`, `/api/v1/reconciliations`, `/api/v1/approvals`, `/api/v1/events/stream`; preserve `GET /api/state` and `POST /approvals/{runID}/{laneID}`; leave extra HTTP dispatch as an open question.

Lens B assumed REST+SSE independently (corroboration) but diverged on wire shape and mutation scope:

- Nested `GET /api/v1/runs/{runID}/lanes` instead of A’s `/api/v1/lanes`. `Lanes(ctx, runID)` supports either; canonical proposal uses A’s path and requires a run ID on the request.
- `/api/v1/state` as a new unified snapshot, citing current `ServerState` (`internal/serve/handlers.go:16-21,120-146`). A did not list this route and keeps `/api/state`. Dropped from `proposal.md`.
- `/api/v1/reconcile/requests` instead of A’s `/api/v1/reconciliations`. Canonical uses A.
- In-scope `control-room-reconcile-dispatch` (`POST /api/v1/reconcile/requests/{id}/approve`) citing CLI `reconcile.Service.Approve` (`cmd/lucind-ai/cli.go:1160-1180`). A and C leave HTTP dispatch beyond single-lane decide as an open question. Capability and the anti-bulk requirement’s extra POST dropped from `proposal.md`; the CLI seam remains in Open Questions.
- Required SPA history fallback. Not in A; UI-owned. Dropped.

Lens C converged on Candidate 1 independently (REST GETs + SSE, loopback, anti-bulk, no WebSockets, no in-memory cache across `run`/`serve`). Naming followed B’s `/api/v1/reconcile/requests` rather than A’s `/api/v1/reconciliations` (path only; recorded above). C’s rollback, additivity, risks, tests, and out-of-scope list match A and are the source of those spine sections. C also left reconcile HTTP as an open question (corroboration with A, against B).
