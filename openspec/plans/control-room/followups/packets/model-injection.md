---
id: model-injection
executor: agy
routed_by: two small serve-internal follow-ups from the control-room-ui-views archive report, disjoint from the reconcile mutation surface
model: gemini-3.7-flash-high
read_only: false
allowed_paths: ["internal/serve/handlers.go","internal/serve/model.go","internal/serve/server_test.go","internal/serve/model_test.go","internal/serve/static/app.js","internal/serve/static_test.go","cmd/lucind-ai/cli.go"]
feature: control-room-followups
parent_ref: refs/heads/control-room/followups
base_sha: d90f12078d006cbd2358ec1488f23984389b35ba
expected_parent_sha: d90f12078d006cbd2358ec1488f23984389b35ba
legacy_main: false
---

# Apply model-injection

Two independent follow-ups carried from `openspec/changes/archive/2026-08-22-control-room-ui-views/archive-report.md`. Do both.

## 1. `NewHandler` should accept a `*Model`

`NewHandler` and `NewHandlerWithConfig` (`internal/serve/handlers.go`) take a `*ledger.Ledger` and construct `NewModel(l)` internally. That makes it impossible to hand the handler a substitute model in a test without going through a real ledger.

Change both to accept a `*Model` instead of building one. Update every caller in this repository. Preserve the existing behaviour exactly — this is a constructor-shape change, not a behaviour change.

`serveDispatch` in `cmd/lucind-ai/cli.go` is a caller and is inside your allowed paths; update it. If every caller can pass a `*Model`, do not leave a ledger-taking compatibility wrapper behind.

**A nil `*Model` is a programming error, not a runtime case.** Do not add `if model == nil` guards to the route handlers. A handler that silently answers `[]` or 404 because its model is nil turns a wiring bug into an empty console with no diagnostic — strictly worse than the nil dereference, which at least names the line. The constructor's contract is that it receives a real model; let it.

Do not weaken or delete any existing test. `TestModelSourceDoesNotShellOut` in particular must keep passing unchanged.

## 2. Lease countdown must not drift with client clock skew

The archive report left open whether the model should return `remaining_seconds` or `expires_at` plus a server timestamp. The implementation already answered half of it: `Lease.ExpiresAt` is what ships, and the client differences against it locally. That is the right choice — a precomputed `remaining_seconds` goes stale the moment it is serialized.

What is missing is the other half. The client currently differences `expires_at` against the *browser's* clock, so a viewer whose machine is a few minutes off sees a wrong countdown — a lease that reads expired when it is live, or live when it is long gone. Both are misleading in a console whose whole job is to show real lease state.

Expose the server's current UTC timestamp alongside the lease data in the same payload the client already fetches, and have the client compute its countdown against a server-anchored clock rather than `Date.now()` directly. The correction is a one-time offset measured when a payload arrives; do not add a new endpoint and do not poll for time.

**The server field is only half the change, and shipping it alone is worse than shipping nothing.** A timestamp added to the payload that no client code reads is dead weight that looks like a feature. This exact pattern — a field or method added with no terminal consumer — shipped undetected in the immediately preceding change and had to be remediated. `internal/serve/static/app.js` is in your allowed paths precisely so you can finish it there.

Cover it with a test that proves the countdown is computed from the server-supplied anchor and not the local clock.

## Done criteria

- [ ] `NewHandler` and `NewHandlerWithConfig` accept a `*Model`; every caller updated; no existing test weakened or removed.
- [ ] The lease payload carries a server UTC timestamp, **and `app.js` actually reads it** and anchors its countdown to it rather than to the raw browser clock.
- [ ] A test proves the countdown uses the server anchor.
- [ ] No `if model == nil` guard was added to any route handler.
- [ ] The work is committed (`git status --porcelain` empty, `git log -1` shows the commit) **and** `.lucind/result.json` is written and schema-valid. A lane that edits files but commits nothing and writes no envelope has produced nothing.
- [ ] `go build ./...` and `go test ./...` pass.
- [ ] Only the declared allowed paths change and the commit is recorded.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- A caller of `NewHandler` lives outside the allowed paths and cannot be updated.
- Anchoring the countdown would require a new endpoint or a change to a file outside the allowed paths.
- Two reasonable shapes exist for the server timestamp field and the surrounding payload conventions do not settle which.
- Satisfying one instruction in this packet would require violating another.

## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
