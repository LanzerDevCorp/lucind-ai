# Synthesis Notes: Control Room Telemetry

## Unresolved Contradictions

None. Lens A and lens C independently treat high-frequency SQLite ingestion as operationally unsafe. Lens B assumed that store for scenarios and success criteria; that is approach divergence (below), not two current-state claims the code cannot break. Archive path, SSE payload shape, and whether coarse milestones need schema v6 are open questions, not incompatible assertions.

## Coverage Gaps

None. All eight spine items were present in the drafts. After citation drops, scenarios 2 and 3 were rewritten to the verified types and call sites rather than left empty.

## Dropped Citations

Claims removed from `explore.md` because the cited lines do not say what the draft claimed. Where the same fact exists elsewhere, the canonical doc uses the verified location instead of the failed citation.

- **`AppendTelemetry` at `internal/ledger/ledger.go:366-381` (lens A).** Those lines are `AppendEvent` inserting into `events`. No `AppendTelemetry` method exists.
- **Worktree cleanup / log archive at `internal/run/run.go:641-646` and `:647-660` (lens A).** Those lines are `enforceAllowedPaths` (diff scan, `lane.Deviated`). `PersistEnvelope` is the Deps field at `internal/run/run.go:189-195` and the CLI implementation at `cmd/lucind-ai/cli.go:647-660` (writes `.lucind/results/<laneID>.json`). Worktree removal is `RemoveLaneWorktree` at `cmd/lucind-ai/cli.go:641-646` and `runWorktreeCleanup` at `cmd/lucind-ai/cli.go:1460-1474`.
- **`worktree cleanup` removes the lane directory at `cmd/lucind-ai/cli.go:118` (lens C).** Line 118 is `case "worktree":` in the top-level switch. Cleanup is `runWorktreeCleanup` at `:1460-1474`.
- **`executor.Result` with `inputTokens`/`outputTokens` at `internal/executor/cursor_agent.go:37-65` and `internal/executor/executor.go:24-30` (lens B).** `cursor_agent.go:37-65` is `KnownModels` plus the start of `Run` (including `--output-format json`). `executor.go:24-30` is `Request.Agent` / `SchemaPath`. `Outcome` (`executor.go:42-63`) has `ExitCode`, `TimedOut`, `Stderr`, `Stdout`, `OutputTruncated` — no token fields. No `inputTokens`/`outputTokens` symbols exist under `internal/executor`.
- **`serve.Model.ListRunEvents`, `GetLaneTelemetry`, `GetAttemptMetrics` (lens B, `internal/serve/model.go:14-25`).** `Model` exists as a feature/attempt/reconciliation DTO layer. Those three methods do not exist. Run history is `Ledger.Events` at `internal/ledger/ledger.go:488-526`.
- **Attempt phases `leased`/`combining`/`checking`/`cas_pending` at `internal/run/integrate_feature.go:34-118` (lens B).** That range is `FeatureTarget` field-copy plus `IntegrateFeature` calling `ExecuteAttempt`. The state machine comment and transitions are `internal/run/attempt.go:213-214` and `:408-443`.
- **`integrate.Check` / `lucind-checks.sh` at `internal/integrate/integrate.go:14-36` (lens B).** Those lines are imports and error vars (`ErrMergeConflict` … `ErrEmptySHA`). `Check` is `internal/integrate/integrate.go:90-109` and uses `CombinedOutput()`, so it is not an incremental stream today.
- **Live check UI before CAS at `internal/serve/model.go:87-125,370-384` (lens B).** Those ranges are reconciliation DTOs (`ReconciliationRecord`, `CASResult`, `AuditEvent`) and assembling a reconciliation payload. They are not integration-attempt check streaming.
- **Parallel worktrees at `internal/worktree/worktree.go:32-68` (lens B).** Those lines are error sentinels and `GitRunner`. Worktree creation is `:179-238`.
- **Events CHECK / v6 readers at `internal/ledger/ledger.go:84-91` (lens C).** Those lines parse `lanes.executor` admitted values from DDL. The events `type` CHECK is only `internal/ledger/schema.go:34-43`.
- **Non-zero exit diagnosis at `internal/run/run.go:73-86` (lens B).** Those lines are the `streamDetailCap` comment. Post-terminal diagnosis printing is `cmd/lucind-ai/cli.go:523-539`.
- **Early stall/deadlock detection at `internal/executor/agy.go:48-66` (lens B).** That range is `printTimeoutFor` (agy `--print-timeout` vs context deadline), not heartbeat or quota observation.
- **WaitDelay proof at `internal/executor/agy_test.go:40-100` (lens C spike 1).** Those tests are happy-path exit 0, non-zero exit, stderr capture, and working directory. Grandchild/`WaitDelay` coverage starts at `agy_test.go:158-174`.
- **Contention benchmark at `internal/ledger/ledger_test.go:200-260` (lens C spike 2).** That range is `RegisterLane` rejecting an unadmitted executor. No telemetry-vs-lease benchmark exists there.
- **SSE disconnect tests at `internal/serve/server_test.go:40-80` (lens C spike 3).** That range is `TestBulkRequestBodyReturns400`. No SSE tests exist.

## Approach Divergence

Lens A's problem (black-box buffered dispatch, closed `events` CHECK, polling-only serve) is the canonical problem. Lens C independently described the same three pressures (buffer RAM, WAL locks, no stream transport) and the same preferred storage/transport pair (worktree files + SSE, not SQLite ingest, not WebSockets/OTLP).

Lens B assumed Candidate 1: structured telemetry rows in the primary ledger (`lane_heartbeat`, `lane_progress`, `tool_call`, `token_usage`, `check_output` via `AppendEvent`), new `serve.Model` query methods, and success criteria that every progress event lands in SQLite. That cost B the live-tail design: its five scenarios and five success boxes were written as ledger appends and `/api/state` polls. Lens C's lock-starvation risk makes that store unviable for high-frequency chunks, so synthesis kept B's operator outcomes (live progress, stall warning, token/duration visibility, shell-free query, integration phase audit, loopback/approval invariants) and rewired the mechanism to Candidate 2. Coarse lifecycle rows and `integration_events` remain SQLite, matching A's "discrete status stays in the ledger."

Independent convergence: A and C both recommend files + SSE; all three agree serve is loopback-only, `events.type` is closed, diagnosis is capped, and executors buffer until exit. They disagree on where *progress* is stored, not on whether live visibility is missing.
