# Proposal Lens C — Risks, Rollback & Test Impact: Control Room UI Shell

## Technical Risks & Failure Modes

| Risk / Failure Mode | Impact | Mitigation | Existing seam (file:line) |
|---|---|---|---|
| SQLite read lock contention during concurrent CLI batch execution | Engine writes fail with `SQLITE_BUSY` or UI polling experiences high latency under heavy write transactions. | Keep UI queries scoped by active view tab rather than full ledger dumps; enforce query timeouts; rely on WAL mode and 5000ms busy timeout. | `internal/ledger/ledger.go:162-185`, `internal/serve/model.go:128-250`, `internal/run/batch.go:110-180` |
| XSS and script injection from untrusted agent output or diffs | Arbitrary script execution in operator browser session via malicious candidate stdout/stderr, diff snippets, or event details. | Strictly use `textContent` and pre-escaped DOM nodes; escape HTML entities (`escapeHtml`); avoid raw HTML interpolation for candidate `Output` or unescaped IDs in click handlers. | `internal/serve/static/app.js:56-65`, `internal/serve/static/app.js:91-94`, `internal/serve/model.go:102-115` |
| Loopback security boundary erosion or DNS rebinding | Unauthenticated control room access or telemetry exfiltration across external networks or cross-origin browser requests. | Strictly enforce loopback binding (`127.0.0.1`/`localhost`) at startup in `serve.ListenAndServe` and CLI dispatch; reject non-loopback addresses; validate Host header. | `internal/serve/server.go:14-22`, `internal/serve/server.go:57-73`, `cmd/lucind-ai/cli.go:691-694` |
| Premature write/mutation actions bypassing CLI concurrency and lease invariants | Concurrent Git or ledger state corruption if browser triggers branch rebase, lease acquire/renew, or reconciliation directly. | Restrict Control Room UI Shell strictly to read-only views for features, leases, reconciliations, and runs; restrict web writes solely to individual approval decisions. | `internal/serve/handlers.go:87-115`, `internal/feature/feature.go:89-135`, `internal/reconcile/reconcile.go:116-168` |
| Violation of anti-bulk approval and individual decision invariants | Accidental inclusion of "Approve All" or bulk approval APIs violates Rule 4 accountability and PRD safety requirements. | Retain strict 400 rejection for array or multi-item decision payloads in `handleDecide`; verify static assets contain no bulk controls; keep items unselected by default. | `internal/serve/handlers.go:161-176`, `internal/serve/static/app.js:22-70`, `internal/serve/static_test.go:11-39` |
| DOM state thrashing and focus/scroll loss during polling updates | Full-container `innerHTML` replacement resets operator scroll position, input focus, and selection state on every polling interval. | Isolate DOM updates to registered view outlets; apply targeted DOM diffing/patching per card/row instead of wiping container innerHTML; preserve active focus. | `internal/serve/static/app.js:22-70`, `internal/serve/static/app.js:96-98` |

## Rollback & Additivity

**Rollback Plan**: Reversal is accomplished via a standard `git revert` of the commit(s) carrying the UI shell changes. Because the UI shell consists solely of embedded static assets (`internal/serve/static.go:8-18`) and read-only HTTP handler routes (`internal/serve/handlers.go:36-118`) backed by existing `serve.Model` queries (`internal/serve/model.go:127-343`), no database migrations, schema rollbacks, or ledger data cleanup are required. Reverting the Git commit immediately restores the single-purpose approvals inbox UI with zero runtime side effects.
**Additivity**: All changes are strictly additive. The SQLite schema version remains at 5 (`internal/ledger/schema.go:10`) with no table modifications or DDL migrations. Ledger wire formats, CLI result envelopes (`internal/result/schema.go:1-68`, `.lucind/result.schema.json:1-160`), and existing HTTP routes (`/api/state` in `internal/serve/handlers.go:79-85` and `POST /approvals/{runID}/{laneID}` in `internal/serve/handlers.go:87-115`) remain backward-compatible while new read-only endpoints expose `serve.Model` structures (`internal/serve/model.go:26-125`).

## Test & Validation Impact

| Test Layer | Impact / Required Coverage | Existing seam (file:line) |
|---|---|---|
| **DTO Serialization & Query Isolation** | Verify `serve.Model` methods correctly read features, leases, attempts, overlap evidence, and reconciliation requests from the ledger as plain JSON DTOs without invoking shell or git subprocesses. | `internal/serve/model_test.go:74-347`, `internal/serve/model_test.go:595-627` |
| **HTTP Handlers & Routing** | Test that `serve.NewHandler` routes new view read endpoints and static assets; confirm 200 OK on valid routes; confirm 400 Bad Request on bulk approval payloads or unselected decisions; confirm 409 on already-decided items. | `internal/serve/server_test.go:42-93`, `internal/serve/server_test.go:95-134`, `internal/serve/server_test.go:136-194`, `internal/serve/server_test.go:196-236` |
| **Security & Boundary Enforcement** | Ensure `serve.ListenAndServe` and CLI dispatch strictly reject non-loopback addresses (`0.0.0.0`, external IPs, public hostnames) with `serve.ErrNonLoopback`. | `internal/serve/server_test.go:17-40`, `internal/serve/server.go:20-22`, `cmd/lucind-ai/cli.go:691-694` |
| **Static Asset Integrity & Invariants** | Validate that embedded static files contain no "Approve All" or bulk selection controls, that items start unselected, and that evidence validation enforces command output or `file:line` syntax. | `internal/serve/static_test.go:11-39`, `internal/serve/static_test.go:41-67`, `internal/serve/static_test.go:69-81`, `internal/serve/static_test.go:83-102` |
| **CLI Dispatch & Lifecycle** | Test `lucind-ai serve` flag parsing (`--addr`, `--approver`, `--approval-timeout`), refusal when executed inside a linked worktree, and graceful shutdown on context cancellation. | `cmd/lucind-ai/cli.go:674-725`, `internal/serve/server.go:41-52`, `internal/worktree/worktree.go:1-35` |

## Out of Scope

- SQLite schema version bump or DDL migrations (remains at schemaVersion 5 in `internal/ledger/schema.go:10`).
- Node.js, npm, bundlers, frontend frameworks (React, Vue, Svelte), or external CDN assets (`internal/serve/static.go:8-18`, `docs/prd.md:219-222`).
- Remote network binding, multi-tenant authentication, or TLS termination (`internal/serve/server.go:14-22`, `cmd/lucind-ai/cli.go:691-694`).
- Bulk approval actions or bypassing per-item individual decision enforcement (`internal/serve/handlers.go:161-176`, `docs/prd.md:232-234`).
- Browser-initiated Git mutations, branch creation, rebase triggers, lease renewals, or reconciliation writes.
- Terminal emulation or interactive shell replacement in the browser.
- Candidate selection, technical approach, and conceptual changes (owned by Lens A).
- Capability impact table, delta specification requirements, and user scenarios (owned by Lens B).

## Open Questions

- [ ] Should live telemetry in the UI shell rely on granular tab-scoped HTTP polling (e.g. 2s ticks) or introduce standard library Server-Sent Events (SSE) for push updates without adding external dependencies?
- [ ] Should client-side navigation use URL hash routing (`#/approvals`, `#/features`) or HTML5 History API with a wildcard route fallback in `internal/serve/handlers.go:39-77`?
