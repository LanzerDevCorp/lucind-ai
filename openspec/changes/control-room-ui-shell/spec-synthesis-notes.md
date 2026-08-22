# Spec Synthesis Notes: Control Room UI Shell

## Unresolved Contradictions

Does this change ship a read-only Features view (`#/features`), or only the shell, Approvals view, and Model GET routes, deferring Features UI to `control-room-ui-views`?

- **Lens B** writes routing scenarios against registered routes `#/approvals` and `#/features` (`spec-lens-b.md` Client-Side Routing scenarios).
- **Lens A** names no Features view requirement; its open question matches the proposal (`proposal.md:170`) and canonical design (`design.md` Open Questions).
- **Lens C** asks the same question (`proposal.md:170`) and does not treat a Features view as a live-spec conflict.
- **Code** has no `#/features`, `view-outlet`, or `hashchange` handler (grep over `internal/serve` is empty). `index.html:154-158` is still `#approvals-container`.

Not picked. The delta keeps Client-Side Routing but names the destination "another registered hash route" / "an unregistered hash", and does not add a Features view requirement.

## Coverage Gaps

1. **Store subscription granularity.** All three lenses ask whether views subscribe to store slices or full snapshots. No draft specified it. Not invented.

2. **New-capability file format vs ADDED headers.** `sdd-spec` says a domain with no live spec is a full spec (`Purpose` + `Requirements`), not a delta. The five new capabilities follow that. Spine item "classified ADDED / MODIFIED / REMOVED / RENAMED" is implicit: no live spec exists, so each requirement is ADDED. `approvals-web-ui` uses `## ADDED Requirements`. No `MODIFIED` / `REMOVED` / `RENAMED` sections (lens C Conflicts: None).

3. **sdd-spec 650-word budget vs packet 1800.** The skill's size budget is 650 words; this packet sets 1800 authored (verbatim MODIFIED copies excluded) and wins on execution. Skill still wins on ADDED/MODIFIED/REMOVED/RENAMED shape, RFC 2119, one-scenario minimum, and copy-full-then-edit for MODIFIED.

4. **Later History API / SSE.** All three lenses ask; proposal and design already keep hash routing and 2000ms polling, and list History catch-all and SSE as out of scope. Not a spec gap for this change.

## Dropped Citations

1. **Lens A: `internal/serve/static/app.js:96-98` as "browser hash change listener and view lifecycle dispatcher".** Those lines are `fetchState(); setInterval(fetchState, 2000);` — the poll timer with no teardown. `handlers.go:39-55` is static MIME + 404 after lookup, not a dispatcher. Requirement kept; hash routing is ADDED. The same `app.js:96-98` lines are used only as the timer unmount MUST cancel.

2. **Lens A: `internal/serve/static/app.js:1-10` as "client view subscriber callbacks".** Those lines `fetch('/api/state')` and call `renderState`; there is no subscriber API. `handlers.go:79-85` does implement `GET /api/state`. Requirement kept as ADDED shared store; the subscriber-callback label is dropped.

3. **Lens A: `internal/serve/static/index.html:142-158` as existing tabs and `#view-outlet`.** Those lines are the current inbox header (approver/rate) and `#approvals-container`. No tabs, no `#view-outlet`. Requirement kept as ADDED shell; the span is the replacement target, not existing chrome.

4. **Lens A: `internal/serve/handlers.go:36-118` as already serving Model GET queries.** `NewHandler` serves `/`, static files, `/api/state`, and `POST /approvals/...` only. `model.go:14-16` documents a shell-free query surface; `NewHandler` never constructs `NewModel`. Requirement kept as ADDED; the mux is the landing site, not an existing Model router.

5. **Lens B coverage: `internal/serve/static_test.go:41` as Layout Shell test seam.** `TestStaticAssetsContainOpencodeCommandAndInlineEvidence` asserts `opencode`, `#approvals-container`, and `isValidEvidence` — evidence/command invariants, not shell chrome. Not used as Layout Shell evidence.

6. **Lens C MODIFIED full blocks** for Loopback Binding, Individual Decisions Without Bulk Approval, Inline Evidence and Batch Review Command, and Approver Wrong-Approval Rate. Verified verbatim against `openspec/specs/approvals-web-ui/spec.md:10-83`. Conflicts: None, so they are not shipped as `MODIFIED` (archive would replace live blocks with identical text). Not a failed citation; omitted as classification.

## Requirement Divergence

All three lenses independently named the same six requirements on the same six capabilities (five new, `approvals-web-ui` existing). Independent convergence.

Lens A's set is authoritative and is not refuted by the live spec: the four live `approvals-web-ui` requirements remain, and "Approvals Inbox View Integration" is a new requirement on that capability.

**Lens B** used the same six names. Cost: the `#/features` destination in routing scenarios is not an A requirement (see Unresolved Contradictions). B's not-found-hash scenario is kept under A's routing requirement. No B-only requirement entered the delta.

**Lens C** used the same six names and inventoried `approvals-web-ui` as the only live spec. Conflicts: None, so no A `ADDED` was reclassified to `MODIFIED`. C's open question on whether wrong-approval rate moves under `control-room-ui-shell` is already split by A: Layout Shell shows the rate in header chrome; the live Approver Wrong-Approval Rate requirement is unchanged. C's consumer inventory (`server.go:12-22,55-73`, `handlers.go:36-118`, `server_test.go:17-236`, `static_test.go:11-102`, `docs/prd.md:217-241`, `cli.go:674-725`) verified; no REMOVED/RENAMED, no migration.
