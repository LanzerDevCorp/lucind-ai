# control-room-serve

## Scope

Expose the existing rich `serve.Model` over REST, add the event/progress tailer and SSE hub, add worktree status data, preserve the existing approvals endpoints, and add guarded control endpoints. `serve.ListenAndServe` remains loopback-only (`internal/serve/server.go:16-22`).

## Non-scope

Do not add UI rendering, schema/ledger APIs, executor decoders, or automatic wave advancement. `POST /api/runs` must remain disabled by default and require the proposal's explicit security gate.

## Exact allowed paths

- `internal/serve/model_ext.go`, `internal/serve/model_ext_test.go`
- `internal/serve/hub.go`, `internal/serve/hub_test.go`
- `internal/serve/worktrees.go`, `internal/serve/worktrees_test.go`
- `internal/serve/handlers_api.go`, `internal/serve/handlers_api_test.go`, `internal/serve/handlers.go`

## Acceptance criteria

- REST snapshots expose the declared runs, lanes, progress, flows, features, leases, reconciliations, and worktrees without shell execution in the model.
- SSE uses durable cursors, reconnects via `Last-Event-ID`/`since`, and drops slow subscribers with a resync signal instead of blocking the tailer.
- Existing `/api/state` and individual approvals/defect behavior remain compatible, including bulk approval rejection.
- Non-loopback bind remains rejected and dispatch control is gated by explicit configuration, origin, and token checks.

## Definition of done

Both DAG waves exit 0, focused `internal/serve` tests and `lucind-ai check` pass, and API contract evidence is recorded.
