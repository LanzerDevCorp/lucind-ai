# Control Room SDD Master Plan

This plan decomposes the approved Control Room proposal into six independently schedulable SDD flows. All six planning flows may start together; implementation is staged because the ledger, capture, server, and UI layers have real compile-time dependencies. The plan is documentation only and does not change production code.

## Verified execution contract

- Current branch: `dev`; verified planning anchor: `705cf492348202e3106bd80c12961ad4ea45aafd`.
- Assets: 24 total: 20 planning templates plus `packet-template.md`, `human-packet-template.md`, `verify-packet-template.md`, and `archive-packet-template.md`.
- Supported dispatch executors are exactly `agy`, `cursor-agent`, and `opencode` (`cmd/lucind-ai/cli.go:65-69`). `human` is schema-supported by the ledger but is not dispatchable by the CLI.
- Verified models: `agy=gemini-3.7-flash-high`, `cursor-agent=cursor-grok-4.6-high`, `opencode=openai/gpt-5.6-sol` (`internal/executor/agy.go:84-95`, `internal/executor/cursor_agent.go:32-46`, `internal/executor/opencode.go:50-64`).
- `routed_by` is the routing condition, never the executor name (`internal/packet/packet.go:38-42`, `internal/run/run.go:318-325`). `agent` appears only on opencode packets.
- Packet `read_only` is a boolean (`internal/packet/packet.go:55-58`, `105-113`). DAG nodes use `read_only_paths`, which is `[]string` (`internal/dag/parse.go:21-36`); `dag.Emit` intentionally does not emit that list as packet `read_only` (`internal/dag/emit.go:17-23`).
- `dag.Waves` preserves declaration order within Kahn waves and validates global transitive overlap (`internal/dag/waves.go:11-18`, `65-72`). `dag.Split` prints one run command per wave; it does not run waves automatically (`internal/dag/split.go:10-18`, `34-43`).
- Empty `allowed_paths` is tolerated by direct packet dispatch but rejected by DAG validation (`internal/packet/packet.go:59-62`, `internal/dag/validate.go:17-37`). Every plan DAG therefore declares non-empty file scopes.

## Feature decomposition

| Feature | Goal | Proposal layers | Apply paths | Blocking dependencies | SDD start |
|---|---|---|---|---|---|
| `control-room-telemetry` | Add schema v6: runs, lane metadata, progress, retention, and open event types. | L1 | `internal/ledger/schema.go`, `internal/ledger/schema_v6_test.go` | None for planning; implementation is the root. | Immediately |
| `control-room-ledger` | Add new-file ledger APIs for runs, progress, lane metadata, and pruning without editing `internal/ledger/ledger.go` in parallel. | L1 | `internal/ledger/runs.go`, `progress.go`, `lanes_meta.go` and their tests | `control-room-telemetry` integrated first. | Immediately |
| `control-room-capture` | Add executor stream contracts/decoders and batched per-lane progress persistence. | L2 | `internal/executor/{executor.go,agy.go,agy_stream.go,cursor_agent.go,cursor_stream.go,opencode.go,opencode_stream.go}`, `internal/run/{batch.go,progress.go}` and tests | `control-room-ledger` integrated first. | Immediately |
| `control-room-serve` | Expose model/API, SSE hub, guarded controls, and worktree status. | L3-L4 | `internal/serve/model_ext.go`, `hub.go`, `handlers_api.go`, `handlers.go`, `worktrees.go` and tests | `control-room-capture` integrated first. | Immediately |
| `control-room-ui-shell` | Replace the static shell with the embedded live-store shell and first three views. | L5 | `internal/serve/static/index.html`, `app.css`, `store.js`, `views/{fleet,dag,flows}.js` and tests | `control-room-serve` integrated first. | Immediately |
| `control-room-ui-views` | Add Features, Timeline, Approvals, and CLI wiring/feature flag integration. | L5 | `internal/serve/static/views/{features,timeline,approvals}.js`, `internal/serve/static/app.js`, `cmd/lucind-ai/cli.go` and tests | `control-room-ui-shell` integrated first. | Immediately |

The proposal's W2/W3/W4/W5/W6 names are retained where they identify the intended new files. Existing shared files that must be changed are assigned to exactly one node; the plan never assigns `internal/ledger/ledger.go` to a new feature node.

## Pairwise parallelism matrix

Every SDD flow can run concurrently with every other SDD flow: each phase writes under its own `openspec/changes/<feature-id>/` directory, and each synthesis lane writes only that feature's canonical artifacts. Apply work is intentionally ordered as shown in the `Apply ordering` column because compilation and embedded-consumer dependencies are real.

| Pair | SDD flows | Apply ordering | Concrete reason |
|---|---|---|---|
| telemetry / ledger | Yes | telemetry → ledger | Ledger APIs consume schema v6 tables and columns. |
| telemetry / capture | Yes | telemetry → capture | Capture persistence requires `runs`, `lane_progress`, and lane metadata. |
| telemetry / serve | Yes | telemetry → serve | Serve queries the v6 records. |
| telemetry / ui-shell | Yes | telemetry → ui-shell | UI consumes server payloads backed by v6. |
| telemetry / ui-views | Yes | telemetry → ui-views | UI views consume the same server contract. |
| ledger / capture | Yes | ledger → capture | `internal/run/progress.go` calls new ledger APIs. |
| ledger / serve | Yes | ledger → serve | `serve/model_ext.go` consumes ledger APIs. |
| ledger / ui-shell | Yes | ledger → ui-shell | UI is downstream of the ledger-backed server. |
| ledger / ui-views | Yes | ledger → ui-views | Views are downstream of the ledger-backed server. |
| capture / serve | Yes | capture → serve | SSE/API expose captured progress. |
| capture / ui-shell | Yes | capture → ui-shell | Fleet/store need progress events in API payloads. |
| capture / ui-views | Yes | capture → ui-views | Timeline and fleet details need progress data. |
| serve / ui-shell | Yes | serve → ui-shell | `go:embed` UI imports the new HTTP/API surface. |
| serve / ui-views | Yes | serve → ui-views | Views require the completed routes and SSE contract. |
| ui-shell / ui-views | Yes | ui-shell → ui-views | Later views import the shell's `store.js` and view registration contract. |

## Execution stages and width

| Stage | Work | Concurrent lanes | Executor budget | Barrier |
|---|---|---:|---|---|
| P1 | Explore for all six features: 3 lenses + synthesis each. | 24 | 18 agy, 6 cursor-agent | All 24 packets terminal and six `explore.md` artifacts integrated. |
| P2 | Propose for all six features. | 24 | 18 agy, 6 cursor-agent | Six proposals accepted; no synthesis contradiction remains unresolved. |
| P3 | Design for all six features. | 24 | 18 agy, 6 cursor-agent | Six designs accepted; every threat row is Applicable or N/A with evidence. |
| P4 | Spec for all six features. | 24 | 18 agy, 6 cursor-agent | Six canonical spec trees exist and are accepted. |
| P5 | Tasks for all six features. | 24 | 18 agy, 6 cursor-agent | Six task plans and review forecasts exist. |
| A1 | `control-room-telemetry` schema node. | 1 | 1 opencode (`openai/gpt-5.6-sol`, agent `build`) | `lucind-ai run` exits 0; `lucind-ai check` passes; feature parent is promoted. |
| A2 | `control-room-ledger`: runs, progress, lanes metadata. | 3 | 3 opencode | One run command with three disjoint packets exits 0; checks pass. |
| A3 | `control-room-capture`: contract, then three decoders, then writer. | 1 then 3 then 1 | 5 opencode | Each wave exits 0 before the next; decoder empirical gaps are resolved. |
| A4 | `control-room-serve`: model/hub/worktrees, then handlers. | 3 then 1 | 4 opencode | API/SSE tests and checks pass. |
| A5 | `control-room-ui-shell`: shell, then Fleet/DAG/Flows. | 1 then 3 | 4 opencode | Embedded assets and view tests/checks pass. |
| A6 | `control-room-ui-views`: Features/Timeline/Approvals, then wiring. | 3 then 1 | 4 opencode | Feature-flag and embedded UI checks pass. |

`agy` and `cursor-agent` are spent only on planning judgment and synthesis. `opencode` is reserved for apply DAG work, as requested. No unattended loop is assumed: the runbook explicitly waits for every split wave and checks its exit status.

## Integration order and SHA derivation

Feature-targeted dispatch requires `feature`, `parent_ref`, `base_sha`, and `expected_parent_sha` (`internal/run/run.go:267-285`). Static DAGs carry the verified planning anchor SHA above so they parse and are reviewable now. Planning uses one staging feature (`control-room-planning`) so all six SDD flows can dispatch safely in parallel without six concurrent feature leases. After planning, the staging parent is merged explicitly into `dev`; implementation feature records are then created one at a time immediately before their apply stage. Before each feature is split, the runbook derives the live SHA and rewrites the two SHA fields in that feature's DAG; it aborts on a stale parent rather than guessing. This is necessary because future implementation commits cannot be known in a static document.

Planning staging setup:

```sh
PLAN_SHA="$(git rev-parse refs/heads/dev)"
git branch --force control-room/planning "$PLAN_SHA"
lucind-ai feature create --id control-room-planning --parent refs/heads/control-room/planning --base-sha "$PLAN_SHA" --expected-parent-sha "$PLAN_SHA"
```

For feature `F` with parent branch `P`:

```sh
BASE_SHA="$(git rev-parse refs/heads/dev)"
EXPECTED_PARENT_SHA="$BASE_SHA"
git branch --force "control-room/$F" "$BASE_SHA"
```

The exact `feature create` flags are verified at `cmd/lucind-ai/cli.go:751-817`. The branch is an explicit integration target, not `main`; `feature.ValidateParentRef` rejects `main` and `lucind/*` (`internal/feature/feature.go:98-112`). After each feature's apply waves, a human or sequential orchestrator runs `git merge --ff-only refs/heads/control-room/$F` into `dev` (or resolves a normal merge explicitly), runs `lucind-ai check`, and only then proceeds. `lucind-ai` does not automatically merge feature branches into `main` or `dev`.

## Critical path

The longest implementation chain is `telemetry → ledger → capture(contract → decoders → writer) → serve(model/hub/worktrees → handlers) → ui-shell(shell → views) → ui-views(views → wiring)`: six feature stages plus seven internal apply barriers. Planning has a separate five-stage chain because each feature's phase synthesis gates its next phase.

## Risk register

| Risk | Mitigation |
|---|---|
| R1 — SQLite write contention. | Batch progress writes at 250 ms or 32 events, configure WAL/`busy_timeout=5000`/`synchronous=NORMAL`, make progress inserts best-effort, and retain the per-run file fallback from the proposal. |
| R2 — Stream format drift. | Empirically capture each executor's stream before implementing its decoder; tolerate unknown records, degrade to blocking JSON, and never fail a lane because telemetry parsing failed. Cursor-agent and opencode formats remain unverified evidence gaps. |
| R3 — Localhost control security. | Keep loopback-only binding, keep read-only endpoints safe, and require explicit `--enable-dispatch`, origin checking, and a per-session token before any dispatch control endpoint is enabled. |
| R4 — Six-wave scope. | Land the telemetry + capture + Fleet spine first; keep each feature rollback boundary independent and record deferred UI/control work rather than widening a packet. |
| D1 — Feature SHA drift during a long unattended run. | Derive and verify `base_sha`/`expected_parent_sha` immediately before `feature create` and `split`; stop on mismatch rather than dispatching stale packets. |
| D2 — Cross-feature dependency hidden by parallel SDD planning. | Keep SDD fan-out parallel but serialize apply/integration exactly in the order above; run `lucind-ai check` after every wave and feature promotion. |
| D3 — Existing-file scope collisions. | Use new ledger files (`runs.go`, `progress.go`, `lanes_meta.go`) and assign each existing shared file to one node; prove every same-wave pair below. |
| D4 — Telemetry schema/API mismatch. | Require schema and ledger API tests before capture, then API contract tests before UI; do not treat a green compile as proof of the stream wire format. |

## Same-wave allowed-path proof

All paths below are repository-relative and are compared by the component-boundary prefix rule (`internal/packet/disjoint.go:8-21`). A pair is legal only when neither path is equal to or a parent of the other.

- `control-room-ledger` wave 1: `runs.go` vs `progress.go` vs `lanes_meta.go` (and their distinct test files): all disjoint; union is `internal/ledger/{runs.go,runs_test.go,progress.go,progress_test.go,lanes_meta.go,lanes_meta_test.go}`.
- `control-room-capture` decoder wave: `agy_stream.go`/`agy.go` vs `cursor_stream.go`/`cursor_agent.go` vs `opencode_stream.go`/`opencode.go`: all disjoint; `run-writer` is in a later wave.
- `control-room-serve` wave 1: `model_ext.go`, `hub.go`, `worktrees.go` (and distinct tests): all disjoint; handlers are later because it registers their consumers.
- `control-room-ui-shell` view wave: `views/fleet.js`, `views/dag.js`, `views/flows.js` (and distinct tests): all disjoint; shell files are in the prior wave.
- `control-room-ui-views` view wave: `views/features.js`, `views/timeline.js`, `views/approvals.js` (and distinct tests): all disjoint; `app.js` and `cli.go` are in the later wiring wave.

No apply DAG uses an empty `allowed_paths` set or a directory scope that would hide a sibling file. Every DAG and every body path is validated before delivery.

## Evidence gaps and explicit skips

- The proposal's exact NDJSON field shape is not verified for cursor-agent or opencode; this plan does not invent one. Empirical validation is a required capture task and R2.
- The proposal names `internal/serve/static/app.css` and `views/*`, but only `index.html` and `app.js` exist today; the plan marks the new CSS/view files and Go static tests as new files and retains the verified `internal/serve/static/` prefix.
- `human-packet-template.md` is schema documentation, but `cmd/lucind-ai/cli.go:65-69` proves it is not dispatchable; no human lane is scheduled.
- No phase is skipped: all six features run explore, propose, design, spec, and tasks because each introduces new architecture, persisted formats, or independently reviewed implementation boundaries.
