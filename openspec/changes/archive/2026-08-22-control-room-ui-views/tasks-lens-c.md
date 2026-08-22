# Tasks Lens C — Proof & Review Burden: Control Room UI Views

## Assumed decomposition

Three sequential work units deliver this change across internal/serve: Unit 1 adds the `BatchLane` DTO and `ListBatchLanes` SQLite query method to `model.go` with barrier evaluation and shell-free guarantees; Unit 2 mounts seven additive read-only GET endpoints on `serve.NewHandler` in `handlers.go` while preserving single-item approval POST invariants; Unit 3 upgrades `index.html` and `app.js` to a five-panel dashboard with tiered polling, keyed DOM patching, and HTML escaping. The critical path is Unit 1 (Model queries) → Unit 2 (HTTP GET routing) → Unit 3 (Static multi-panel UI).

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 690–1000 lines (Model: ~220, Handlers: ~260, Static UI: ~350) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Model DTO & Queries) → PR 2 (HTTP GET Endpoints) → PR 3 (Static Multi-Panel UI) |
| Delivery strategy | ask-on-risk |
| Chain strategy | feature-branch-chain |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

Basis for estimate: Derived from existing file sizes in `internal/serve/`: `model.go` (563 lines) adding `BatchLane` DTO and queries (~100 lines product + ~120 lines in `model_test.go`); `handlers.go` (231 lines) mounting 7 GET routes (~140 lines product + ~120 lines in `server_test.go`); `app.js` (97 lines) and `index.html` (162 lines) refactored for five panels with tiered polling, keyed DOM patching, and escaping (~290 lines product + ~50 lines in `static_test.go`). Total across 7 files is 690–1000 lines, exceeding the 400-line single-PR budget.

## RED Tests from the Threat Matrix

| Threat row | Applicable | RED test | Asserts | Production task it precedes |
|---|---|---|---|---|
| Documentation-like paths | N/A | None | Handlers and Model do not classify or execute filesystem paths | None |
| Git repository selection | N/A | None | Model reads SQLite only without subprocesses (`internal/serve/model_test.go:595-627`) | None |
| Commit state | N/A | None | No commit construction performed by UI view endpoints | None |
| Push state | N/A | None | No git push performed by UI view endpoints | None |
| PR commands | N/A | None | No PR argv composed or executed by UI view endpoints | None |

## Acceptance Evidence

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|
| Unit 1: Model `BatchLane` & `ListBatchLanes` (`internal/serve/model.go:85-115`) | `go test -run 'TestBatchLanesRoundTrip|TestModelSourceDoesNotShellOut' ./internal/serve` (`internal/serve/model_test.go:22,595`) | `ListBatchLanes` scans `lanes`/`lane_note` events, maps `barrier.Evaluate` outcome, and imports zero `os`/`exec`/`git` packages | Does not prove JSON wire serialization or HTTP transport |
| Unit 2: HTTP GET API Routes (`internal/serve/handlers.go:80-145`) | `go test -run 'TestGetRoutesReturnJSON|TestBulkRequestBodyReturns400|TestDecideAlreadyDecidedReturns409Conflict' ./internal/serve` (`internal/serve/server_test.go:42,70,196`) | `NewHandler` returns 200 with valid JSON for all 7 GET endpoints and preserves 400 on bulk POST and 409 on conflicts | Does not prove client poller timing or UI rendering |
| Unit 3: Five-Panel HTML Structure (`internal/serve/static/index.html:140-220`) | `go test -run 'TestStaticAssetsContainFivePanels|TestEmbedFSHasNoApproveAllControl|TestItemsStartUnselectedInUI' ./internal/serve` (`internal/serve/static_test.go:11,41,69`) | Embedded HTML defines containers for all 5 panels, preserves metric IDs, avoids pre-selected controls, and lacks bulk terms | Does not prove CSS stylesheet layout or visual alignment |
| Unit 3: Vanilla JS Multi-Panel Poller (`internal/serve/static/app.js:20-130`) | `go test -run 'TestStaticEvidenceValidationRejectsBareMultilineProse|TestEmbedFSHasNoApproveAllControl' ./internal/serve` (`internal/serve/static_test.go:11,83`) | Embedded JS enforces strict evidence validation, escapes HTML diagnostics, and omits bulk approval loops | Does not prove live browser DOM event loops or network latency handling |

## Verification Gaps

- Dynamic browser runtime and DOM state: Repository verifies static embedded assets via AST/string checks (`internal/serve/static_test.go:11-102`) and server endpoints via `net/http/httptest` (`internal/serve/server_test.go:42-236`). Headless browser integration (e.g., Chromedp/Playwright) would be required to prove live 2s timer cycles, scroll preservation across keyed DOM updates, and card expansion fetch triggers.
- Visual styling and contrast: Visual rendering and responsive layout across viewports require manual browser inspection since no visual regression harness is configured in the repository.

## Open Questions

- [ ] Reconcile mutation transport: Will future reconciliation lifecycle transitions (`approve`, `decline`, `cancel`, `renew`, `resolve`) support UI POST endpoints or remain copy-paste CLI invocations matching `cmd/lucind-ai/cli.go:1044-1065`? (Escalated from `design.md:133-134`; surface is read-only).
- [ ] Lease countdown synchronization: Should `Model` provide computed `remaining_seconds` or raw `expires_at` with client clock skew handling? (`design.md:134-135`).
- [ ] Overlap payload formatting: Should `evidence_json` render via `<pre>` with `escapeHtml` or a zero-dependency JSON syntax highlighter? (`design.md:135-136`).
- [ ] Process precedence drift: `sdd-tasks/SKILL.md` directs generating a monolithic `tasks.md` with checklist, work units, and Engram persistence, while packet `tasks-control-room-ui-views-lens-c` scopes this lane to `tasks-lens-c.md` (proof and review workload forecast only); noted per asymmetric precedence contract.
