# Tasks Lens C — Proof & Review Burden: Control Room UI Shell

## Assumed decomposition

The change decomposes into three sequential units along the frontend-backend boundary:
1. **Unit 1 (Model Query API)**: Wire `NewModel(l)` in `serve.NewHandler` (`internal/serve/handlers.go:36`) and expose the 13 read-only Model GET endpoints with JSON responses, 404 handling, and 405 rejection.
2. **Unit 2 (UI Shell & Core Modules)**: Replace monolithic `index.html` and `app.js` with zero-build static assets (`index.html`, `style.css`, `shell.js`, `router.js`, `store.js`) establishing persistent header chrome, hash routing (`#/approvals`), 2000ms polling, and view lifecycle.
3. **Unit 3 (Approvals View & Tests)**: Implement `internal/serve/static/views/approvals.js` with targeted DOM patching, inline evidence enforcement, single-item POST decisions, and retarget static AST/embed tests.

Critical path: Unit 1 → Unit 2 → Unit 3, establishing backend endpoints and shell lifecycle before mounting the approvals view.

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 650–850 lines (additions: ~600, deletions: ~200) |
| 400-line budget risk | Medium |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Model Query API) → PR 2 (UI Shell & Router) → PR 3 (Approvals View & Tests) |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Medium

Derived by counting estimated line changes across affected files and test suites, grounded against archived change `openspec/changes/archive/2026-08-20-approvals-web-ui/` (1200 lines total churn):
- `internal/serve/handlers.go`: +90 lines (Model GET routes and JSON helpers).
- `internal/serve/server_test.go`: +160 lines (tests for 13 GET routes, 404s, 405 on non-GET, zero mutations).
- `internal/serve/static/index.html`: ~60 lines (+60 added, -160 deleted = ~220 lines churn).
- `internal/serve/static/style.css`: +150 lines (stylesheet for shell, tabs, cards, evidence, disconnected banner).
- `internal/serve/static/shell.js`: +55 lines (entry point module, store/router bootstrap, header metrics).
- `internal/serve/static/router.js`: +65 lines (hash registry, mount/unmount lifecycle, 404 view).
- `internal/serve/static/store.js`: +70 lines (2s poll timer, state cache, subscriber pub/sub, error retention).
- `internal/serve/static/views/approvals.js`: +110 lines (targeted card patching, event listeners, decide POST).
- `internal/serve/static/app.js`: -98 lines deleted.
- `internal/serve/static_test.go`: +75 lines (+75 added, -30 deleted for modular JS/MIME assertions).

## RED Tests from the Threat Matrix

| Threat row | Applicable | RED test | Asserts | Production task it precedes |
|---|---|---|---|---|
| Documentation-like paths (`requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh`) | N/A: UI renders evidence as text; no file classification or execution | None | N/A | None |
| Git repository selection (`git -C`, relative/absolute paths) | N/A: `serveDispatch` resolves root once (`cli.go:696-705`); no git subprocesses | None | N/A | None |
| Commit state (staged, `commit -a`, empty index) | N/A: no git commits; web writes are SQLite approval rows (`handlers.go:87-115`) | None | N/A | None |
| Push state (tracking branch, first push, refspec) | N/A: no push | None | N/A | None |
| PR commands (`--head`, env prefix, composed commands) | N/A: no PR automation | None | N/A | None |

*Note*: Preserved invariant checks from `internal/serve/server_test.go:17-40` (`TestNonLoopbackListenFails`), `internal/serve/server_test.go:42-93` (`TestBulkRequestBodyReturns400`), and `internal/serve/model_test.go:595-627` (`TestModelSourceDoesNotShellOut`) continue to guard loopback bind, anti-bulk 400 rejection, and shell-free model execution.

## Acceptance Evidence

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|
| Model Query GET routes (`internal/serve/handlers.go`) | `go test -v -run 'TestModelEndpointsReturnJSON\|TestModelEndpointsRejectNonGET\|TestModelEndpointNotFound' ./internal/serve` (derived from `internal/serve/server_test.go:136` & `internal/serve/model_test.go:22`) | 13 Model GET routes return 200 with JSON DTOs, return 404 for unknown IDs, return 405 for non-GET, and make zero ledger writes. | Browser handling of malformed DTO payloads. |
| Zero-build ES module asset delivery (`internal/serve/handlers.go`, `internal/serve/static.go`) | `go test -v -run 'TestStaticAssetsMIMETypes\|TestEmbedFSHasNoApproveAllControl' ./internal/serve` (derived from `internal/serve/static_test.go:11,41`) | Static files serve with `application/javascript` or `text/css`, missing files return 404, and no embedded asset contains bulk approval strings. | Browser cross-origin script import resolution. |
| Layout shell HTML & Header Chrome (`internal/serve/static/index.html`) | `go test -v -run 'TestSPAShellContainsOutletAndHeader' ./internal/serve` (derived from `internal/serve/static_test.go:41,69`) | `index.html` embeds header chrome (`#approver-name`, `#approver-rate`), tabs, `#view-outlet`, and ES module entry script. | CSS responsive layout rendering across viewports. |
| Hash router & view lifecycle (`internal/serve/static/router.js`) | `go test -v -run 'TestRouterModuleStructureAndLifecycle' ./internal/serve` (derived from `internal/serve/static_test.go:83`) | `router.js` implements `mount`/`unmount` lifecycle, registers `#/approvals`, listens to `hashchange`, clears previous timers, and renders 404 on unknown hashes. | Browser history back/forward navigation in a live browser engine. |
| Shared reactive store (`internal/serve/static/store.js`) | `go test -v -run 'TestStoreModulePollingAndSubscriberAPI' ./internal/serve` (derived from `internal/serve/static_test.go:83`) | `store.js` polls `GET /api/state` every 2000ms, caches state, notifies subscribers, and retains cached state on HTTP 500 errors. | Timer throttling behavior in backgrounded browser tabs. |
| Approvals view & targeted DOM patching (`internal/serve/static/views/approvals.js`) | `go test -v -run 'TestApprovalsViewValidationAndSafety' ./internal/serve` (derived from `internal/serve/static_test.go:83`) | `views/approvals.js` mounts into `#view-outlet`, patches cards by ID without wiping `innerHTML`, validates inline evidence, and posts single decisions. | Visual scroll and focus preservation during DOM node replacement. |
| Preserved loopback and anti-bulk invariants (`internal/serve/server.go`, `internal/serve/handlers.go`) | `go test -v -run 'TestNonLoopbackListenFails\|TestBulkRequestBodyReturns400\|TestSingleApprovalAndDefectEndpoints' ./internal/serve` (derived from `internal/serve/server_test.go:17,42,136`) | Server rejects non-loopback bind (`ErrNonLoopback`), rejects bulk payloads with 400, and allows single-item decision/defect POSTs. | Network behavior over external reverse proxies. |

## Verification Gaps

1. **Browser DOM Execution & Lifecycle**: Tests run in Go without a headless browser harness (e.g. Playwright). JavaScript module resolution, live DOM patching, and timer cancellation are verified via static AST/substring inspection. Runtime UI validation requires manual verification with `lucind-ai serve --addr 127.0.0.1:7433` or a future browser test harness.
2. **Focus and Scroll Preservation**: Preserving active input focus and scroll positions during 2000ms poll ticks is an in-browser layout behavior that cannot be directly asserted by Go string/MIME unit tests.

## Open Questions

- [ ] Should the synthesizer adopt the 3-unit split (`stacked-to-main` chain) or request a single PR with a size exception?
