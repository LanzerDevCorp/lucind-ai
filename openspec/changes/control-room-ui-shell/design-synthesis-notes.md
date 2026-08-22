# Synthesis Notes: Control Room UI Shell

## Unresolved Contradictions

Lens C lists an optional new seam of `node --test` or a headless browser harness for end-to-end DOM rendering, and in the same draft places Node.js/npm/bundlers out of scope (`docs/prd.md:219-222`, `internal/serve/static.go:8-18`). Lenses A and B never mention a Node test runner. Existing frontend tests are Go substring/AST checks over `StaticFS()` (`internal/serve/static_test.go:11-102`); the repo has no Node test harness (`Makefile:7-8` is `go install` only). Product-build no-Node is settled; whether a test-only Node runner is allowed is not. Not picked.

## Coverage Gaps

1. **Change-level delta specs are absent.** `openspec/changes/control-room-ui-shell/specs/` does not exist. Requirements used in the canonical design are the proposal's inline delta-spec names (Layout Shell, Client-Side Routing, Shared Store, Zero-Build Embed, Approvals Inbox Integration, Read-Only Model Inspection). No lens cited change-level spec files.

2. **sdd-design SKILL.md 800-word budget vs packet 1800.** The skill's size budget is 800 words; this packet sets 1800 and wins on execution. Canonical `design.md` follows the packet. Skill still wins on decision shape (Choice / Alternatives considered / Rationale) and on marking every threat-matrix row `Applicable` or `N/A: reason`.

3. **File-changes tables omit test files.** No lens listed `internal/serve/static_test.go` or `internal/serve/server_test.go` as Modify, even though `static_test.go:14,57,86` `ReadFile`s `app.js` (deleted in this change) and C's HTTP layer requires httptest of new Model GET routes. Testing Strategy records the retarget; those paths are not in File Changes.

4. **Archived extra threat row not in lens C.** `openspec/changes/archive/2026-08-20-approvals-web-ui/design.md` added an Applicable row for unauthenticated loopback HTTP that can release a waiting lane. Lens C (threat-matrix owner) marked only the skill's five rows, all N/A. Canonical design follows C and notes loopback/anti-bulk as preserved invariants under testing, not as a new Applicable row.

## Dropped Citations

1. **Lens A: `internal/serve/static/index.html:142-158` as existing "nav anchors".** Those lines are the current inbox (`<header>` with approver/rate, `#approvals-container`). Hash routing is kept via the 404-after-static-lookup seam (`internal/serve/handlers.go:39-55`). The inbox span is cited as the replacement target (`index.html:141-163`), not as nav that already exists.

2. **Lens B: `internal/serve/static/index.html:7` as the `style.css` load site.** Line 7 is `:root {` inside the inline `<style>` block. CSS delivery is kept via existing `.css` MIME (`internal/serve/handlers.go:47-48`). Create `style.css` kept; the `index.html:7` load-site claim dropped.

3. **Lens C: `internal/ledger/ledger.go:42-58` as `ledger.Open` / ephemeral schema v5.** Those lines are error sentinels (`ErrLaneUnknown`, `ErrAlreadyDecided`, `ErrPragmaNotApplied`). `Open` is `internal/ledger/ledger.go:146`. Seam kept via `model_test.go:22-30` and `ledger.go:146`.

4. **Lens C: `httptest.NewServer` as an existing seam at `internal/serve/server_test.go:42-93` and `136-236`.** Those tests use `httptest.NewRequest` + `httptest.NewRecorder` only. No `httptest.NewServer` in `*_test.go`. NewServer dropped; NewRecorder kept.

5. **Lens C: `internal/result/schema.go:1-63` as the envelope contract.** The file ends at line 62. Envelope-untouched claim kept via `schema.go:1-62` (embedded `result.schema.json`).

## Architecture Divergence

Lens A's assumed architecture is canonical: extend `NewHandler` with `NewModel` and read-only Model GET routes; modular ES modules (`shell.js`, `router.js`, `store.js`, `views/approvals.js`); hash router; shared poll store; process/CLI/loopback/schema v5/approval POSTs unchanged.

**Lens C** independently converged on that same paragraph (near-verbatim). Its testing, threat matrix, rollback, and out-of-scope content all survive. Cost: none from architecture mismatch. Residual: the optional Node test seam (see Unresolved Contradictions), which did not enter `design.md`.

**Lens B** assumed Candidate 1 independently (embed SPA, `NewHandler` wires `NewModel` without signature change, writes stay decide/defect). Divergence: B's assumed architecture and File Changes **Create** `views/features.js` and name it as a `handlers.go` terminal consumer. A (and C) leave whether a Features **view** ships as an open question while still exposing Model GET routes including `ListFeatures`/`GetFeature`. `views/features.js` did not enter `design.md`; Model GET routes did. B's REST path table (including `GET /api/features/{id}/events`) is the surface; A's `/api/audit`-style names were treated as groupings, not competing URLs. B's store-slice vs snapshot question kept as an open question.
