# Spec Synthesis Notes: Control Room UI Views

## Unresolved Contradictions

None

## Coverage Gaps

- **sdd-spec new-domain path.** The skill writes new capabilities as full specs (`Purpose` + `Requirements`) at `openspec/specs/<capability>/spec.md`. Packet execution forbids writing the live tree; all five files are change-folder deltas (`ADDED` for the four new capabilities). Archive places them into `openspec/specs/`.
- **Batch-wave error state.** Lens B's cyclic-DAG scenario was dropped (failed citation; see Dropped Citations). Happy path and mixed-terminal edge remain. No remaining draft-backed error-state scenario for `batch-wave-view`.
- **Reconciliation unknown-id.** Lens B's HTTP 404 scenario was dropped. Failed CAS remains as the error/edge case. Proposal lists collection `GET /api/reconcile/requests` only; no by-id route was specified.
- **Envelope `hard_stops`.** Proposal displays them only if a DTO can supply them without `os`/`git`. No draft named a requirement for that; ledger has no envelope blob. Not invented.
- **Shared open questions** (reconcile POST vs copy-paste CLI, overlap JSON rendering, countdown source) appear in all three drafts and stay unspecified. Not invented.

## Dropped Citations

- **`internal/serve/model.go:26-125` as `ListBatchLanes` (lens A).** Those lines are feature-parent DTOs (`Feature` through `AuditEvent`). No `ListBatchLanes` or `BatchLane`. Design.md:80 states the same. Batch-wave requirement kept via `internal/lane/status.go:10-16`, `internal/barrier/barrier.go:21-60`, `internal/dag/waves.go:41-70`, `internal/run/batch.go:19-27,40-43,50-52`, `internal/worktree/worktree.go:212-238`.
- **`internal/serve/model.go:26-125` as `BatchLane.DemotionNote` (lens A).** No such field. Demotion text is `ledger.EventLaneNote` at `internal/run/run.go:423-430`; offending-path format at `internal/run/run.go:650-652`. Lane-demotion requirement kept on those lines.
- **`internal/dag/waves_test.go:41-70` as cycle/wave coverage (lens B).** Those lines are `TestWaves_OrderingAndYAMLOrderPreserved` fixtures. Cycle test is `TestWaves_CycleDetected` at `:11-40`; `ErrCycleDetected` is `internal/dag/waves.go:8,52`. Cyclic-DAG UI scenario dropped: it names an implementation error, and a cycle fails before ledger lanes exist for the dashboard to inspect.
- **`internal/run/run_test.go:643-653` as demotion coverage (lens B).** Those lines are `TestExecuteTruncatedOutcomeStillYieldsDoneAndReportsCapture`. Demotion production code is `internal/run/run.go:650-652`; demotion tests start at `internal/run/run_test.go:1593`.
- **`internal/serve/server_test.go:196-236` as reconciliation 404 (lens B).** Those lines are `TestDecideAlreadyDecidedReturns409Conflict`. `NewHandler` (`internal/serve/handlers.go:36-118`) has no reconcile routes. Unknown-id 404 scenario dropped.

## Requirement Divergence

Lens A's set is authoritative: four ADDED (`Batch and DAG Wave Inspection`, `Shell-Free Feature and Lease Monitoring`, `Reconciliation Candidate Inspection`, `Lane Demotion Diagnosis`) and one MODIFIED (`Individual Decisions Without Bulk Approval`). Live spec does not refute that set. Loopback Binding (`openspec/specs/approvals-web-ui/spec.md:10-25`) stays untouched.

**Lens B** keyed scenarios to proposal name `Anti-rubber-stamping in the multi-view shell`. Cost: `Unsupported claim withheld` and the evidence / wrong-rate / opencode clauses of `Single approval submission` did not enter the delta (they belong to live `Inline Evidence and Batch Review Command` and `Approver Wrong-Approval Rate`, which A did not name). Bulk-reject and a trimmed single-item success scenario joined A's MODIFIED block.

**Lens C** copied three live MODIFIED blocks and offered consolidating them under the proposal name. C's Conflicts section is None — those live guarantees stay true without replacement — so the extra two blocks are not shipped. No ADDED→MODIFIED correction: C found no live-spec conflict against A's four ADDED capabilities.

**Independent convergence.** All three named the same four new capabilities and the same four ADDED requirement names (casing aside). All three left Loopback Binding untouched. All three share the three open questions above.
