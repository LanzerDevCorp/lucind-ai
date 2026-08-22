# Synthesis Notes: Control Room Serve

## Unresolved Contradictions

None

## Coverage Gaps

None

## Dropped Citations

- Lens A: `ledger.Ledger` at `internal/ledger/ledger.go:217-222`. Those lines are `Close()`. `type Ledger struct` is at `internal/ledger/ledger.go:132`. Claim kept with the corrected citation.
- Lens A: "approver accuracy rates" at `internal/ledger/ledger.go:797-815`. `ApproverRate` documents a **wrong-approval** rate (flagged defects / approved count) at `internal/ledger/ledger.go:795-814`. "Accuracy" dropped.
- Lens A Candidate 1: wiring to `ledger.Ledger` at `217-222` (same miss as above).
- Lens B: UI "polling disjoint endpoints" at `internal/serve/static/app.js:1-10, 97`. `fetchState` calls only `/api/state` (`1-5`); `setInterval` is `96-97`. "Disjoint" dropped; 2s poll kept.
- Lens B: `/api/v1/state` already returns unified state from `internal/serve/handlers.go:16-21` and `internal/serve/model.go:27-125`. `ServerState` is approver, rate, command, approvals only. `model.go:27-125` are JSON DTO types, not a unified query. `/api/v1/state` does not exist.
- Lens B Scenario 1 action: `GET /api/v1/state` with `Accept: application/json` at `internal/serve/handlers.go:63-66`. That branch is `/` JSON Accept. `/api/state` is `79-85`.
- Lens B Scenario 1: "approver defect rate" at `internal/ledger/ledger.go:578-605`. Those lines are the `Approval` struct. Rate is `795-814`.
- Lens B Scenario 1: current GET returns "registered feature summaries" via `internal/serve/model.go:128-149`. `serveStateJSON` (`120-146`) never calls `ListFeatures`. Kept only as a proposed addition.
- Lens B: runs/lanes HTTP "querying lane state transitions" at `internal/ledger/ledger.go:249-302`. `RegisterLane` insert starts at `255`; `Lanes` query is `285-330`.
- Lens B Scenario 2: SSE frames as events occur at `internal/ledger/ledger.go:304-332`. That is the `Lanes()` scan loop. Event types are `438-446`; `Events()` is `490-525`. No SSE handler exists.
- Lens B Scenario 3: pending approval at `internal/ledger/ledger.go:520-547`. Those lines finish `Events()` iteration and define helpers. `PendingApprovals` is `705-717`. `handlers.go:87-115` is the POST router, not "awaiting review".
- Lens B Scenario 3: `ledger.Decide` at `internal/ledger/ledger.go:429-460`. That is `Event` plus the start of `SetStatus`. `Decide` is `615-640`.
- Lens B Scenario 3: `handleDecide` "broadcasts transition". It records `Decide` and writes `{"ok":true}` (`internal/serve/handlers.go:195-211`). No broadcast.
- Lens B: `POST /api/v1/reconcile/requests/.../approve` citing `cmd/lucind-ai/cli.go:1068-1180` as the HTTP handler. That range is CLI `runReconcileApprove`. Kept only as a wrap seam at `1166-1176`.
- Lens B: "creates candidate record" at `internal/serve/model.go:101-115`. That is the `ReconciliationCandidate` DTO, not insert logic.
- Lens B: "awaiting" ledger status at `internal/serve/model.go:72-92`. That is the JSON struct; `'awaiting'` is the CHECK at `internal/ledger/schema.go:141-154`.
- Lens C: bulk-mutation rejection at `internal/serve/handlers.go:195-206`. Those lines map `ErrAlreadyDecided` / `ErrLaneUnknown`. Array rejection is `161-176`.
- Lens C: "configure connection pool (`db.SetMaxOpenConns(4)`)" as a future mitigation at `internal/ledger/ledger.go:162-185`. `SetMaxOpenConns(4)` already runs in `openAtPath` at `182`. SQLITE_BUSY risk kept; unimplemented-pool claim dropped.
- Lens C: WebSockets require `gorilla/websocket`. `go.mod` has no websocket module. Lens A named `nhooyr.io/websocket` instead. Library names dropped; "would add a third-party WS library" kept.

## Approach Divergence

Lens A (primary) named three candidates (REST+SSE, expanded `/api/state` polling, WebSockets) and recommended 1.

Lens B did not evaluate candidates 2 or 3. It assumed REST+SSE (`/api/v1/*` + `/api/v1/events/stream`) and spent its budget on personas, six scenarios, and success criteria. Several scenarios described proposed HTTP as if it already existed (unified `/api/v1/state`, SSE, reconcile POST). Those current-tense claims were dropped; the scenarios survive as proposed use cases with verified seams.

Lens C used the same three transport choices in a trade-off matrix (SSE / polling / WebSockets) without A's candidate numbers. It added: in-memory cache cannot sync across independent `run` and `serve` processes; DNS rebinding beyond bind-address checks; SSE vs `WriteTimeout`; SPA 404 masking. That cross-process fact makes A's in-process event-channel option insufficient as the **sole** notifier. Canonical explore.md records that as settled by `runDispatch` and `serveDispatch` each calling `ledger.Open` (`cmd/lucind-ai/cli.go:285,707`), and leaves WAL-tail vs IPC vs SQLite hooks as the open question.

Independent convergence: all three treat today's serve as a loopback approvals UI; all three want Model reads over HTTP plus live events; all three keep loopback, linked-worktree refusal, and individual (non-bulk) decisions; none recommend WebSockets as the primary design.

Naming: A/B use `/api/v1/events/stream`; C's spike says `/api/events`. Canonical uses A's path (A owns candidates).
