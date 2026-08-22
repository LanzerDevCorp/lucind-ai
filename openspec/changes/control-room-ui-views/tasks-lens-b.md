# Tasks Lens B — Partition & Dispatch Shape: Control Room UI Views

## Assumed decomposition

The change partitions into three discrete units: Unit 1 adds `BatchLane` and `ListBatchLanes` to `*serve.Model` with SQLite ledger unit tests and AST shell-out enforcement (`internal/serve/model.go`, `internal/serve/model_test.go`); Unit 2 mounts the modular REST GET endpoints on `serve.NewHandler` with HTTP integration tests (`internal/serve/handlers.go`, `internal/serve/server_test.go`); Unit 3 implements the five-panel vanilla JS controller, tab layout, and embedded static asset assertions (`internal/serve/static/app.js`, `internal/serve/static/index.html`, `internal/serve/static_test.go`). The critical path is Unit 1 → Unit 2 for Go compilation (`handlers.go` calls `model.ListBatchLanes`), while Unit 3 is decoupled and builds independently against embedded static assets.

## Suggested Work Units

| Unit | Goal | allowed_paths | Executor | Rollback boundary |
|---|---|---|---|---|
| 1 | Add `BatchLane` DTO and `ListBatchLanes` query method to `*serve.Model` with AST and query unit tests | `internal/serve/model.go`<br>`internal/serve/model_test.go` | `cursor-agent` | Reverting restores `serve.Model` to existing feature-only list queries without `BatchLane` or `ListBatchLanes`. |
| 2 | Mount modular GET `/api/*` endpoints on `serve.NewHandler` and verify with HTTP route tests | `internal/serve/handlers.go`<br>`internal/serve/server_test.go` | `cursor-agent` | Reverting restores `serve.NewHandler` to legacy `/api/state` and `/approvals/` endpoints only. |
| 3 | Refactor static UI into five tabbed panels with tiered polling, keyed DOM patching, string escaping, and embedded asset tests | `internal/serve/static/app.js`<br>`internal/serve/static/index.html`<br>`internal/serve/static_test.go` | `agy` | Reverting restores `internal/serve/static/` to the single-panel approvals poller and template. |

## Wave Plan

| Wave | Units | Runs in parallel | Green on its own |
|---|---|---|---|
| 1 | Unit 1, Unit 3 | Yes | Yes: Unit 1 adds independent Model methods and unit tests; Unit 3 updates static files and `static_test.go` without Go symbol dependencies; the combined tree compiles (`go build ./...`) and passes all checks (`lucind-checks.sh`). |
| 2 | Unit 2 | No | Yes: Unit 2 mounts HTTP handlers consuming `ListBatchLanes` and existing Model methods; compiles and passes `lucind-checks.sh` once Wave 1 is integrated. |

## Disjointness Check

- **Wave 1 (Unit 1, Unit 3)**:
  - Unit 1 `allowed_paths`: `internal/serve/model.go`, `internal/serve/model_test.go`
  - Unit 3 `allowed_paths`: `internal/serve/static/app.js`, `internal/serve/static/index.html`, `internal/serve/static_test.go`
  - Component-boundary prefix evaluation (`internal/packet/disjoint.go:13-21`): All paths are concrete file paths; no path in Unit 1 is a prefix of any path in Unit 3 and vice versa. Verdict: **DISJOINT (PASS)**.
- **Wave 2 (Unit 2)**:
  - Single-unit wave (`internal/serve/handlers.go`, `internal/serve/server_test.go`). Single-unit wave requires no pair check. Verdict: **DISJOINT (PASS)**.

## Sidecar Recommendation

**Recommendation**: single packet, no sidecar
**Rationale**: Although the work partitions cleanly into a two-wave plan (Wave 1: Units 1 & 3; Wave 2: Unit 2), a DAG sidecar is not warranted. The change modifies 7 files in a single package (`internal/serve`), totaling ~400–600 lines including tests. Authoring `apply-dag.yaml`, creating packet bodies in `apply-bodies/`, running `lucind-ai split`, and managing multi-wave bisection overhead exceeds the complexity of the change itself. Following precedent from `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md` and `openspec/changes/archive/2026-08-21-sdd-fan-out-lens/apply-dag.yaml`, a single packet executed sequentially with three work-unit commits is the recommended dispatch shape.

## Open Questions

- [ ] Contract divergence: `~/.claude/skills/sdd-tasks/SKILL.md` prescribes a monolithic `tasks.md` with checklist, forecast, and Engram persistence, whereas this change uses the three-lens parallel task fan-out contract.
- [ ] Handler Model binding: `design.md:136` leaves open whether `serve.NewHandler` instantiates `NewModel(l)` internally or receives `*serve.Model` as an explicit parameter during Unit 2 implementation.
