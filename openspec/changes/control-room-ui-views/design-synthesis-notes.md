# Synthesis Notes: Control Room UI Views

## Unresolved Contradictions

**Reconciliation mutation transport.** Lens A and lens B leave HTTP POST for `approve`/`decline`/`cancel`/`renew`/`resolve` versus copy-paste CLI as an open question (`design-lens-a.md` Open Questions; `design-lens-b.md` Open Questions; same question in `proposal.md:152` and `explore.md:84`). Lens C states it as decided out of scope: “UI renders copy-paste CLI commands” citing `cmd/lucind-ai/cli.go:1044-1065`. The code does not settle it: `NewHandler` has no reconcile routes (`internal/serve/handlers.go:36-118`); CLI dispatch exists at `cmd/lucind-ai/cli.go:1044-1065`. Not picked. Canonical `design.md` keeps the open question and lists mutation as out of scope for this change’s requirement set.

## Coverage Gaps

- **Kahn waves and per-lane deadlines are not in schema v5.** All three drafts (and the proposal) say `GET /api/batch/lanes` exposes wave grouping (`internal/dag/waves.go:41-70`) and per-lane deadlines (`internal/run/batch.go:40-43`). `lanes` has `started_at`/`ended_at` and `worktree_*` (`internal/ledger/schema.go:18-32`), not `wave`, `depends_on`, or `deadline`. `Waves` is in-memory Kahn over a packet DAG; `batch.go:40-43` is a comment on `Deps.LaneTimeout` at `ExecuteBatch` time. No draft named a ledger-backed reconstruction. `design.md` keeps status, worktree, preserved flag, `lane_note`, and in-process `barrier.Evaluate`; it does not claim SQL-backed waves or deadlines.
- **Envelope `hard_stops` have no Model field.** Proposal requires displaying them only if a DTO can supply them without `os`/`git`. Ledger has no envelope blob. Drafts point at `lane_note` (`internal/run/run.go:423-430`) for demotion text, not at `internal/result/result.schema.json:21-43`. No invented source.
- **Delta spec files.** Packet precondition named `openspec/changes/control-room-ui-views/specs/`. That directory is absent. Deltas live in `proposal.md`; the modified capability is `openspec/specs/approvals-web-ui/spec.md`.
- **sdd-design 800-word budget.** Skill `~/.claude/skills/sdd-design/SKILL.md` sizes designs at 800 words; this packet’s 1800-word ceiling and `openspec/changes/archive/` practice win. Not treated as a missing spine item. Skill-required Interfaces/Contracts is present (from lens B surface deltas). Threat-matrix applicability rule is present (every canonical row `N/A` with reason).

## Dropped Citations

Claims removed from `design.md` because the cited range did not support them. Neighboring true facts kept only with a verified line.

### Lens A

- **`internal/serve/static/app.js:1-98` as the whole client file.** File ends at line 97 (`setInterval(fetchState, 2000)`). Kept as `:1-97`.
- **`internal/serve/static/index.html:1-140` as the embedded UI to tab.** Lines 1–140 are CSS through `</head>`. Body/markup is `:141-162`. Five-panel work still targets this file; the 1–140 range is not the panel DOM.
- **`internal/serve/static.go:8-19` embed pipeline.** File ends at line 18. Kept as `:8-18`.
- **`GET /api/batch/lanes` exposes Kahn wave grouping from `internal/dag/waves.go:41-70` via `lanes`/`events`.** Those lines are in-memory Kahn plus overlap validation. No wave column. Wave-from-SQLite dropped.
- **`GET /api/batch/lanes` exposes deadlines from `internal/run/batch.go:40-43`.** Those lines document per-lane `LaneTimeout` on `ExecuteBatch`, not a persisted deadline. Deadline-from-SQLite dropped.

### Lens B

- **`internal/serve/model.go:561-563` — “timestamps format as RFC3339Nano.”** `parseTime` parses ledger TEXT with `time.RFC3339Nano`. It does not set JSON serialization. Serialization claim dropped. Empty-slice `[]` kept via `out := []Feature{}` (`internal/serve/model.go:137`).
- **`cmd/lucind-ai/cli.go:1044-1065` vs lens A `1043-1065`.** Both resolve: 1043 is the `reconcileDispatch` comment; function and switch are 1044–1065. Kept as `1044-1065` (proposal’s range). Not a dropped claim.
- Same `app.js:1-98`, `index.html:1-140`, wave, and deadline citations as lens A. Dropped the same way.

### Lens C

- **`internal/feature/feature.go:48` — `feature.NewService(l)`.** Line 48 is `StatusCreated`. `NewService` is `internal/feature/feature.go:94`. Seam kept with `:94`.
- **`internal/reconcile/service.go:26` — `reconcile.NewService(l, WithClock)`.** Path does not exist. Actual: `internal/reconcile/reconcile.go` (`WithClock` `:128`, `NewService` `:157`). Seam kept with those lines.
- **`internal/serve/model.go:26` as the `ListBatchLanes` insertion line.** Line 26 is the `Feature` DTO comment. Treated as a proposed neighborhood, not an existing method. New method is still required; the `:26` pin is dropped.
- Wave/deadline test mapping that assumed those values already live on ledger rows: dropped as data source; `barrier.Evaluate` (`internal/barrier/barrier.go:36-60`) over lane statuses is kept.

## Architecture Divergence

**Not irreconcilable.** All three assumed modular GET routes on `serve.NewHandler`, `*serve.Model` as the query surface, keep `GET /api/state` and `POST /approvals/{runID}/{laneID}`, add `ListBatchLanes`, no schema bump, five vanilla panels, 2s hot poll plus lazy heavy payloads, zero npm.

**Independent convergence.** B’s assumed architecture matches A’s Candidate 2, including `schemaVersion = 5` and keyed DOM. C lists the same route set (including `/api/features/{id}/attempts`), the same Model extension, and the same hot/lazy split, without naming Candidate 2. C’s rollback independently states version 5 and additive `/api/*`. Treat as corroboration of A.

**B vs A.** No competing architecture. B did not own decisions; its flow, surface deltas, and file table survive A. Unsupported wave/deadline-from-SQLite clauses (shared with A) were dropped in citation verification, not because B forked.

**C vs A.** C’s assumed architecture omits keyed in-place DOM (A Decision 4; B hop 5) and omits `schemaVersion = 5` from the opening block (it appears in C rollback). Neither is a fork; keyed DOM and version 5 enter `design.md` from A. C proposed `NewHandlerWithModel` as a new seam (`design-lens-c.md` Test Seams); A keeps `NewHandler` as the multiplexer without changing the constructor. That signature is an open question, not an A decision. C’s copy-paste-CLI close of reconciliation mutation does not survive: A (and the proposal) left it open — see Unresolved Contradictions.

**Content C lost.** Treating reconcile POST as already “deferred to CLI” as a design fact. **Content B lost.** RFC3339Nano response-format invariant; wave/deadline SQL backing (shared drop).
