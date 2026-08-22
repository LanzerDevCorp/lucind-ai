# Tasks Lens B — Partition & Dispatch Shape: Control Room UI Shell

## Assumed decomposition

The implementation decomposes into two standalone deliverables: Unit 1 (Backend Model GET REST API) and Unit 2 (Frontend Vanilla ES SPA Shell & Approvals View). Unit 1 wires `serve.NewModel` in `serve.NewHandler` and registers read-only telemetry query routes, while Unit 2 replaces the monolithic `app.js` with modular vanilla ES modules (`shell.js`, `router.js`, `store.js`, `views/approvals.js`, `style.css`), updating layout and static embed tests. The critical path is the frontend shell migration (Unit 2), which encapsulates the approvals view and preserves existing loopback and anti-bulk invariants while decoupling telemetry views.

## Suggested Work Units

| Unit | Goal | allowed_paths | Executor | Rollback boundary |
|---|---|---|---|---|
| 1 | Expose read-only Model telemetry GET routes on `NewHandler` with HTTP test coverage | `internal/serve/handlers.go`<br>`internal/serve/server_test.go` | `cursor-agent` | `internal/serve/handlers.go` GET route registrations and `internal/serve/server_test.go` HTTP tests; reverts API without affecting static embed or `/approvals` routes |
| 2 | Migrate monolithic UI to modular ES-module SPA shell, router, store, approvals view, and CSS styling | `internal/serve/static/index.html`<br>`internal/serve/static/style.css` (new file)<br>`internal/serve/static/shell.js` (new file)<br>`internal/serve/static/router.js` (new file)<br>`internal/serve/static/store.js` (new file)<br>`internal/serve/static/views/approvals.js` (new file)<br>`internal/serve/static/app.js`<br>`internal/serve/static_test.go` | `agy` | Frontend assets in `internal/serve/static/` and `internal/serve/static_test.go`; restores monolithic `app.js` and original `index.html` inbox layout |

## Wave Plan

| Wave | Units | Runs in parallel | Green on its own |
|---|---|---|---|
| 1 | Unit 1, Unit 2 | Yes | Yes: Unit 1 only adds handlers to existing `serve.Model` methods and passes `lucind-checks.sh`; Unit 2 fully encapsulates frontend modules with updated `static_test.go` assertions, compiling under `go:embed` and passing `lucind-checks.sh` independently and combined. |

## Disjointness Check

- **Unit 1 vs Unit 2**:
  - Unit 1: `internal/serve/handlers.go`, `internal/serve/server_test.go`
  - Unit 2: `internal/serve/static/index.html`, `internal/serve/static/style.css` (new file), `internal/serve/static/shell.js` (new file), `internal/serve/static/router.js` (new file), `internal/serve/static/store.js` (new file), `internal/serve/static/views/approvals.js` (new file), `internal/serve/static/app.js`, `internal/serve/static_test.go`
  - *Verdict*: Disjoint under `internal/packet/disjoint.go` component-boundary prefix matching; naming concrete file paths prevents directory-level `internal/serve/` overlap.

## Sidecar Recommendation

**Recommendation**: single packet, no sidecar
**Rationale**: The change totals ~400–600 lines across 10 files split into two compact units. While Units 1 and 2 have disjoint file paths and can theoretically execute in parallel, a two-node partition does not justify the operational overhead of `apply-dag.yaml` generation, multi-worktree packet splitting (`lucind-ai split`), and per-wave bisection coordination. Following the precedent in `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md` (which declined a DAG sidecar for independent, small work units), this change should be applied sequentially in a single packet containing two reviewable work-unit commits.

## Open Questions

- [ ] None. Precedence note: `~/.claude/skills/sdd-tasks/SKILL.md` describes a monolithic task breakdown generating `tasks.md`, which is superseded here by the three-lens parallel task partitioning contract.
