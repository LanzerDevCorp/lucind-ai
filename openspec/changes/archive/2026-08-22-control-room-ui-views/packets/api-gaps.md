---
id: api-gaps
executor: agy
routed_by: closing the reachability and coverage gaps the verify lane's task-completion gate surfaced, single lane, no fan-out
model: gemini-3.7-flash-high
read_only: false
allowed_paths: ["internal/serve/handlers.go","internal/serve/server_test.go","internal/serve/model_test.go","internal/serve/static_test.go"]
feature: control-room-ui-views
parent_ref: refs/heads/control-room/control-room-ui-views
base_sha: de5f5cbe939c5f1e2c9bd19e23a39059b73cd8d1
expected_parent_sha: de5f5cbe939c5f1e2c9bd19e23a39059b73cd8d1
legacy_main: false
---

# Apply api-gaps

## Goal

`(*serve.Model).ListBatchLanes` (`internal/serve/model.go`) has zero callers anywhere in the repository outside its own definition. It was built in this change and never given a terminal consumer, so it is dead code. Mount it over HTTP, mount the approvals read endpoint alongside it, and add the three test cases the change's `tasks.md` named but never delivered.

## Scope

### 1. Mount `GET /api/batch/lanes`

In `NewHandler` (`internal/serve/handlers.go`), mount a read-only `GET` route that calls `ListBatchLanes` and encodes its result as JSON. It takes a run id, so the route must carry one. Follow the route shape already established in this file rather than inventing a new one — the existing handlers use the `/api/features/{id}/attempts` path-segment style, and consistency with them wins over the literal path string `tasks.md` guessed at. A nil slice encodes as `[]`, never `null`. A missing or unknown run id is a 404, not a 500.

### 2. Mount `GET /api/approvals`

Mount a read-only `GET` route returning the pending approvals list. The composed `/api/state` handler already sources this data; reuse that source rather than opening a second query path to the ledger. Same encoding rules as above.

Do NOT touch the existing `POST /approvals/{...}` decision routes, the bulk-request 400 (`TestBulkRequestBodyReturns400`), or the already-decided 409 (`TestDecideAlreadyDecidedReturns409Conflict`).

### 3. `TestBatchLanesRoundTrip` in `internal/serve/model_test.go`

Use the existing `openModelLedger` helper. Assert status mapping, worktree preservation, the demotion note, and the barrier outcome. Do not weaken or remove `TestModelSourceDoesNotShellOut`.

### 4. `TestGetRoutesReturnJSON` in `internal/serve/server_test.go`

Cover both new routes: 200 with JSON content type, an empty result encoding as `[]`, and the not-found path. Follow the table-driven style the surrounding tests already use.

### 5. Static contract test in `internal/serve/static_test.go`

Assert, through `StaticFS()`, that every view container the shell actually renders is present in the embedded assets. Assert the real ids that exist today — do not rename anything in `index.html` or `app.js` to match a name from `tasks.md`. Those two files are outside your allowed paths.

## Out of scope

`tasks.md` also called for replacing the client's single `/api/state` poller with five per-panel fetches. That is deliberately NOT in this packet. The implementation converged on one composed state endpoint plus an SSE stream, which supersedes that task rather than failing it. Do not rewrite the client. Do not touch `internal/serve/static/app.js` or `internal/serve/static/index.html` — neither is in your allowed paths.

## Done criteria

- [ ] `ListBatchLanes` is reachable over HTTP and that route is covered by a test.
- [ ] `GET /api/approvals` is mounted and covered by a test.
- [ ] `TestBatchLanesRoundTrip`, `TestGetRoutesReturnJSON`, and the static contract test all exist and pass.
- [ ] `go build ./...` and `go test ./...` pass.
- [ ] Only the declared allowed paths change and the commit is recorded.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- Mounting either route would require editing a file outside `allowed_paths`.
- `ListBatchLanes`'s signature cannot serve an HTTP route without changing `model.go`, which is outside `allowed_paths`.
- Two reasonable route shapes exist and the existing handlers do not settle which.
- Satisfying one instruction in this packet would require violating another.

## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
