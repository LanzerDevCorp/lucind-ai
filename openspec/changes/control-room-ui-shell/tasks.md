# Tasks: Control Room UI Shell

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 650–850 (additions ~600, deletions ~200) |
| 400-line budget risk | Medium |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Model GET API) → PR 2 (SPA shell, approvals view, embed tests) |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Medium

**Dispatch:** No `apply-dag.yaml` sidecar. Single packet, two sequential work-unit commits. Units 1 and 2 are path-disjoint and each Integrate-green, but two nodes do not justify split overhead.

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary | allowed_paths | Executor |
|------|------|-----------|----------------------|-----------------|-------------------|---------------|----------|
| 1 | Wire `NewModel` in `NewHandler` and register 13 read-only Model GET routes with HTTP coverage | PR 1 | `go test ./internal/serve` | N/A — httptest against `NewHandler`; no new listen path | `internal/serve/handlers.go` GET registrations and Model HTTP tests in `internal/serve/server_test.go` | `internal/serve/handlers.go`, `internal/serve/server_test.go` | `cursor-agent` |
| 2 | Replace monolithic inbox with modular ES-module SPA (store, router, CSS, approvals view, shell), delete `app.js`, retarget embed tests | PR 2 | `go test ./internal/serve` | `lucind-ai serve --addr 127.0.0.1:7433` then open `/` and `#/approvals` (Go tests are substring/AST only) | `internal/serve/static/` assets and `internal/serve/static_test.go`; restores `app.js` and inbox `index.html` | `internal/serve/static/index.html`, `internal/serve/static/style.css`, `internal/serve/static/shell.js`, `internal/serve/static/router.js`, `internal/serve/static/store.js`, `internal/serve/static/views/approvals.js`, `internal/serve/static/app.js`, `internal/serve/static_test.go` | `agy` |

Concrete file paths keep Unit 1 vs Unit 2 disjoint under `PathInScope` (`internal/packet/disjoint.go`). A directory prefix `internal/serve/` would collide. Do not split Unit 2 asset edits from `static_test.go`: `TestEmbedFSHasNoApproveAllControl`, `TestStaticAssetsContainOpencodeCommandAndInlineEvidence`, and `TestStaticEvidenceValidationRejectsBareMultilineProse` `ReadFile` `app.js` today (`internal/serve/static_test.go:14,57,86`).

## RED tests from the threat matrix

Design marked every threat-matrix row `N/A`. No new RED-test tasks. Keep `TestNonLoopbackListenFails` (`internal/serve/server_test.go:17-40`), `TestBulkRequestBodyReturns400` (`internal/serve/server_test.go:42-93`), and `TestModelSourceDoesNotShellOut` (`internal/serve/model_test.go:595-627`).

## Phase 1: Backend Model Telemetry Endpoints (Unit 1)

- [ ] 1.1 In `serve.NewHandler` (`internal/serve/handlers.go:36-38`, which does not construct `NewModel` today) call `serve.NewModel(l)` (`internal/serve/model.go:21-24`) and register 13 read-only GET routes wrapping `internal/serve/model.go:128-343`: `/api/features`, `/api/features/{id}`, `/api/features/{id}/attempts`, `/api/attempts/{id}`, `/api/leases`, `/api/features/{id}/lease`, `/api/features/{id}/overlap`, `/api/features/{id}/overlap/{hash}`, `/api/features/{id}/reconciliations`, `/api/reconciliations/{id}`, `/api/reconciliations/{id}/candidates`, `/api/candidates/{id}`, `/api/features/{id}/events`. JSON encode; 404 unknown IDs; 405 non-GET; no ledger writes. `NewHandler` signature stays (`handlers.go:36`).
- [ ] 1.2 Add HTTP tests in `internal/serve/server_test.go` (existing seam `TestSingleApprovalAndDefectEndpoints` at `:136`): 13 GETs return 200 JSON, missing IDs 404, non-GET 405, zero ledger mutations. Prove with `go test ./internal/serve`. Existing decide/defect/bulk tests must stay green.

## Phase 2: Foundational Client Modules (Unit 2)

- [ ] 2.1 Create `internal/serve/static/store.js`: cache `GET /api/state` (`internal/serve/handlers.go:79-85`), poll every 2000ms (today `app.js:96-97`), notify subscribers, retain cache on HTTP 500.
- [ ] 2.2 Create `internal/serve/static/router.js`: hash routing (`#/route`), `mount`/`unmount`, `hashchange`, timer/listener teardown, 404 view. Do not replace `#view-outlet` `innerHTML` on poll ticks.
- [ ] 2.3 Create `internal/serve/static/style.css`: layout shell, header (`#approver-name`, `#approver-rate`), disconnected indicator, tabs, card outlet, evidence. `.css` MIME already set (`handlers.go:46-47`).

## Phase 3: Approvals View & Shell Integration (Unit 2)

- [ ] 3.1 Create `internal/serve/static/views/approvals.js`: `mount(container, store)` / `unmount()`, patch cards by id (not wipe outlet `innerHTML`; today `app.js:45-69`), inline `file:line` / command-output evidence (`app.js:12-19`), single-item `POST /approvals/{runID}/{laneID}` (`handlers.go:87-115`). No `onclick` id interpolation (`app.js:64-65`).
- [ ] 3.2 Modify `internal/serve/static/index.html` (162 lines): replace inbox (`:141-157`) and `<script src="/app.js">` (`:160`) with persistent header chrome, nav tabs, `#view-outlet`, `<script type="module" src="/shell.js">`.
- [ ] 3.3 Create `internal/serve/static/shell.js`: init store, register `#/approvals`, bind header metrics, default route.
- [ ] 3.4 Delete `internal/serve/static/app.js` (97 lines) only after 3.3.

## Phase 4: Static Embed & Invariant Verification (Unit 2)

- [ ] 4.1 Update `internal/serve/static_test.go:11-102`: scan new modules (`shell.js`, `router.js`, `store.js`, `views/approvals.js`) for bulk-approval terms; retarget `isValidEvidence` / no `trimmed.includes('\\n')` checks off `app.js`; keep unselected-controls check; add httptest MIME assertions for `.js` / `.css` (`handlers.go:44-47`). Prove with `go test ./internal/serve`. Also keep `TestSingleApprovalAndDefectEndpoints` (`server_test.go:136-194`).

## Dependency Order

Phase 1 is independent of Phases 2–4 (`GET /api/state` already exists). Inside Unit 2: 2.1/2.2/2.3 have no mutual deps; 3.1 needs 2.1; 3.2 needs 2.3; 3.3 needs 2.1, 2.2, 3.1, 3.2; 3.4 needs 3.3; 4.1 needs 3.2–3.4. Apply Unit 1 then Unit 2 (or reverse); do not ship 3.4 without 4.1.

## Requirement Traceability

| Requirement | Tasks |
|---|---|
| Layout Shell and Global Chrome | 2.3, 3.2, 3.3 |
| Client-Side Routing and View Lifecycle | 2.2, 3.3 |
| Centralized Reactive Client Store | 2.1, 3.3 |
| Zero-Build Embedded ES Module Delivery | 2.1, 2.2, 2.3, 3.1, 3.2, 3.3, 3.4, 4.1 |
| Read-Only Model Inspection Endpoints | 1.1, 1.2 |
| Approvals Inbox View Integration | 3.1, 3.3, 4.1 |

## Open Questions

None for implementers. Orchestrator still asks chained-PR vs `size:exception` (`ask-on-risk`). Features view stays out of scope (`control-room-ui-views`). Browser DOM lifecycle and focus/scroll are not covered by Go embed tests; runtime harness above is manual.
