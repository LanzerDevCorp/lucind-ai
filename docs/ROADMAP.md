# lucind-ai: roadmap

Only unimplemented work. What exists today is in [`product.md`](product.md). Each item was re-checked against `dev` and is still pending.

## Verification

### `lucind-ai stability run` (ADR 0001, accepted, not implemented)

Product-owned stable-release validation instead of an external harness. No `stability` subcommand and no `internal/stability` package exist on `dev`; an August prototype (`feature/native-stability-campaign`, remote only) stopped at a blocked qualitative verdict and was never merged.

Essentials of the decision:

- **Scope.** Linux-only, interactive terminal, no non-interactive bypass. One **Stability Campaign** bound to a clean `HEAD` and the exact installed build; `agy` only, pinned model `gemini-3.7-flash-high`. Passing never tags or releases.
- **Preflight (read-only).** Clean worktree, running binary matches `HEAD`, `agy` available, `lucind-ai check` green, forecast of 15 model dispatches, explicit `yes/no` confirmation with `no` as default.
- **Trials.** Three strictly sequential Stability Trials; any failure stops the Campaign and resets the consecutive-success count. Each runs the full deterministic fixture journey from clean state with real `agy` edits: Change A, independent Change B, separate Fix Change, selective blocking, explicit test-actor approvals, distinct Integration Targets, ancestry isolation, resumption of A after the fix.
- **Canonical crash.** B's `agy` process is killed after its Lane result is persisted and before Acceptance; a 10 s Ownership Lease must expire before an explicitly recorded reclaim dispatches B's replacement. Surviving descendant processes fail the Trial.
- **Budgets.** 5 `agy` dispatches per Trial (A, B, replacement B, Fix, resumed A), no automatic retries; 10 min per dispatch, 45 min per Trial, 135 min per Campaign.
- **Storage.** Mutable state in SQLite/WAL plus immutable content-addressed JSON receipts under `<git-common-dir>/lucind-ai/stability/v1/`; ordinary Run IDs stay in `<primary-root>/.lucind/lucind.db` and are linked, not migrated. Records hold bounded sanitized logs and raw-payload hashes only (no credentials, env values, usernames or absolute paths).
- **Recovery.** `stability status [--json]` (read-only), `resume` and `abort` (interactive); ambiguous recovery fails closed, unresolved residue leaves the Campaign `blocked_cleanup`, `abort` retries cleanup without redispatching.
- **Receipt.** Binds candidate commit, build, fixture digest, the three Trial Records, `agy`/model versions, Linux environment and verdict; the latest terminal Campaign per commit decides certification. `lucind-ai check` runs before mutation and again after cleanup.
- **Testing.** Fake-executor tests cover the state machine; only a real three-Trial `agy` Campaign is acceptance evidence and it is excluded from `go test ./...`.
- **Known defects to fix from the blocked prototype:** fix worktree path derivation (`pathFor`, not `wt-<id>`), wire real CAS promotion and ancestry isolation, persist trial stages for resume, track process groups for orphan cleanup.

### Proofs still missing

- **`cursor-agent` as a write lane / simultaneous batch.** A read-only lane and the blind judges were run against the real CLI in a sandbox repo (2026-10-03, per the factory design record), but there is no recorded run of a `cursor-agent` write lane through dispatcher commit and integration, and the v1 criterion "one batch dispatching `agy` and `cursor-agent` simultaneously" is unproven. (`README.md` still says "never executed"; reconcile it when the proof lands.)
- **RDD over a batch this binary produced.** The `gentle-ai` boundary is designed, not exercised end to end.
- **Intermittent tests.** Two timing-sensitive ledger/feature tests still flake under full-suite load; reproduce each in isolation before calling it flaky.

## Orchestration refactors

From the 2026-08-23 architecture review of `internal/run` and `internal/integrate`. None is realized in code. Start with 1; 3 is the cheapest independent win; 2, 4, 5 get easier once 1 forces real seams.

1. **`Deps` god-interface** (`internal/run/run.go`, `type Deps`, ~25 function-valued fields). Collapse into deep seams (Workspace: worktree/combine/checks; Promotion: CAS/target; Gate: overlap/reconcile) with real git adapters and test fakes. Today the real `integrate.Combine/Check/PromoteTarget/PromoteCAS` wiring (`cmd/lucind-ai`, `productionDeps`) is covered by no composed test; every orchestration test fakes it.
2. **Two failure protocols.** Legacy `Integrate` (ff-merge + recursive bisection + `revertLanes`) and feature `IntegrateFeature`/`attempt.go` (lease, overlap gate, CAS, no bisection) each own their revert/report/cleanup semantics. Extract one combine -> check -> report -> revert/promote pipeline with bisection as an explicit swappable strategy.
3. **`ledger.DB()` leak.** `internal/run/attempt.go` hand-writes SELECT/INSERT/UPDATE on `integration_attempts` through the raw `*sql.DB`. Move attempt CRUD (`GetAttempt`, `GetAttemptByIdempotencyKey`, `InsertAttempt`, `UpdateAttempt`) behind `ledger.Ledger` and drop raw access.
4. **Unexported overlap gate.** `evaluateOverlapGate` (`internal/run/attempt.go`, ~235 lines) is testable only by driving the whole attempt state machine. Export a directly testable seam (`gate.Evaluate(ctx, deps, params) Result`).
5. **Duplicated combine/promote protocol.** `integrate.ResolveAndPromoteCandidate` and `run` `driveAttemptFromLeased` + `performCASPromotion` are two hand-written copies of preflight -> combine -> check -> re-validate (TOCTOU) -> CAS. Unify into one fail-closed protocol so a staleness-recheck fix lands once.
6. **Bisection tested only against fakes.** `TestBisect*` fake `CombineTree`/`RunChecks`; no test drives recursive bisection through real multi-branch merge conflicts in real git. Add a composed test (closed for free by item 1).

## Herdr

`herdr-direct-dispatch` (branch `feature/herdr-direct-dispatch`, not merged: it branched before the interactive-agents merge and lacks that code; only T1 `result validate` exists there, and `LUCIND_RESULT` / `result validate` are absent from `dev`). Rebase or merge onto `dev` first; T3 and T5 both edit `internal/executor/herdr.go`.

- **T2. Packetless `dispatch` command** (no packet file; direct prompt dispatch).
- **T3. `LUCIND_RESULT` env and sentinel** in `herdr*.go` so the agent is told where to write and completion is signaled by sentinel.
- **T4. Self-validation prompt preamble** (agent validates its own envelope via `result validate`).
- **T5. Bounded repair pane** (max 2).
- **T6. Docs and proof.**

Overlap to decide before building: the interactive-agy Stop hook (`hook stop`) already validates the envelope and forces up to 2 agent repairs, which makes T4 and T5 largely redundant for interactive `agy`. They would still be the only repair path for headless `agy`, `cursor-agent`, `claude` and `opencode`, whose native Stop-style hooks are unverified (`cursor-agent`, `opencode`).

## Known gaps

- **Conflict triage past 400 lines is unwired.** `internal/conflicttriage` (advisory, fail-open) is implemented and tested, but its `TriageInvoker` has no production wiring (`internal/conflicttriage/invoker.go`), so CI never places a live LLM call. Conflicts over 400 lines escalate to the human today.
- **Usage log is blind for interactive agy.** Interactive runs have no usage JSON, so the log records `tokens_known:false`; token/cost telemetry (e.g. transcript parsing) is not implemented.
- **Headless executors have no result-repair loop** (see Herdr above).
- **herdr `agent` idle detection is unreliable with agy**; completion relies on the exit sentinel or `done.json`.
- **Stale agy trust entries after a crash** in `~/.gemini/antigravity-cli/settings.json`.
- **Approvals habit unaddressed.** The approvals UI is gone; whether `accept` plus the human diff read beats the "always say yes" habit is untested.
