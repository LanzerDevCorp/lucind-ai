# Tasks: Feature Parent Integration

## Review Workload Forecast

Estimate: 2,800–4,000 lines. Review budget: 5,000 changed lines.

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

`single-pr`: 5,000 leaves independent 400-line guard. Apply unauthorized without `size:exception` (none recorded) unless strategy changes.

### Work units

| Unit | Focused proof | Runtime harness | Rollback |
|---|---|---|---|
| 1 Ledger/features | `go test ./internal/ledger ./internal/feature` | Temp Git+SQLite | Disable v3 commands; retain data |
| 2 Admission/Git | `go test ./internal/packet ./internal/dag ./internal/worktree` | Temp Git | Targets/plumbing |
| 3 Attempts/promote | `go test ./internal/run ./internal/integrate` | Git/SQLite race | Orchestration |
| 4 Reconciliation | `go test ./internal/overlap ./internal/reconcile ./internal/resolve` | Git conflict fixture | Gate/candidate flow |
| 5 CLI/DTOs/docs | `go test ./cmd/lucind-ai ./internal/serve` | Git/SQLite CLI | User surfaces/docs |

## Apply-time DAG contract

`tasks.md` is deterministic seed, not executable DAG/runtime NLP input. Stable IDs/minimum semantic dependencies start here. `sdd-apply` re-reads code, groups IDs into packets, authors bodies/exact `allowed_paths`, projects edges, and may add safety edges. `lucind-ai split` validates/authors waves—not dependencies/prose. Sidecars bind `tasks_path`, `tasks_digest`, `task_ids`; mismatch regenerates. Paths, executor/model, bodies: apply-time facts.

### Prerequisite hardening

Proposed separate `apply-dag-dispatch-hardening` lands before Phase 2: global transitive-reachability ordering for every overlapping packet pair; NUL staged diff (including index-only paths), both rename endpoints, immutable start SHA. Old unannotated tasks use hand-authored sidecar fallback. Prerequisite, not feature scope.

### Candidate wave sketch

Non-executable; paths add safety edges: W1 `1.1`; W2 `1.2 + 2.1`; W3 `1.3 + 2.2 + 3.1`; W4 `2.3 + 3.3`; W5 `3.2 + 4.1`; W6 `3.4`; W7 `4.2`; W8 `4.3`.

## Phase 1: Foundation

- [x] 1.1 `internal/ledger`: **RED** v4 atomic state/audit, retention; **GREEN** repositories. Ledger. **Deps:** none. U1.
- [x] 1.2 `internal/feature`: **RED** lifecycle/base/no rewrite or `main`; **GREEN** audit. Temp-Git. **Deps:** 1.1. U1.
- [x] 1.3 `internal/{ledger,feature}`: **RED** serialization, expired fence/recovery, cross-feature concurrency; **GREEN** fences. Multi-handle SQLite. **Deps:** 1.1, 1.2. U1.

## Phase 2: Targets and Git

- [x] 2.1 `internal/{packet,dag,run}`, `cmd/lucind-ai/cli.go`: **RED** four fields/legacy rejection; **GREEN** adapter/dispatch-only flag. Package. **Deps:** 1.1. U2.
- [ ] 2.2 `internal/{worktree,integrate}`: **RED** roots, refs/starts, stale CAS; **GREEN** `GitRunner`, worktree, CAS. Temp Git. **Deps:** 2.1. U2.
- [ ] 2.3 `internal/{run,integrate}`: **RED** replay, resume/block, bisection/promotion; **GREEN** attempt machine/parent. Faults. **Deps:** 1.1, 1.2, 1.3, 2.1, 2.2. U3.

## Phase 3: Reconciliation

- [x] 3.1 `internal/overlap`: **RED** evidence/labels/thresholds/structural omission; **GREEN** normalize. Table/Git fixtures. **Deps:** 1.2. U4.
- [ ] 3.2 `internal/{run,integrate,ledger}`: **RED** required blocks both promotions; warnings dispatch; **GREEN** gate. Two-parent integration. **Deps:** 2.3, 3.1. U4.
- [ ] 3.3 `internal/reconcile`, `internal/ledger`: **RED** direction binding, decline/cancel/expiry/renewal; **GREEN** audit/candidate. State. **Deps:** 3.1. U4.
- [ ] 3.4 `internal/{resolve,integrate}`: **RED** leakage/index/markers/ambiguity/checks/timeout/limit/stale refs preserve evidence; **GREEN** resolver/CAS. Git faults. **Deps:** 2.3, 3.2, 3.3. U4.

## Phase 4: Operations

- [ ] 4.1 `internal/serve/model.go`: **RED** shell-free status/audit DTOs; **GREEN** adapter. DTO. **Deps:** 3.3. U5.
- [ ] 4.2 `cmd/lucind-ai/cli.go`, `internal/{run,ledger}`: **RED** feature/reconciliation outputs, idempotency/audit; **GREEN** wiring. Git/SQLite CLI. **Deps:** 1.1, 1.2, 1.3, 2.1, 2.2, 2.3, 3.1, 3.2, 3.3, 3.4, 4.1. U5.
- [ ] 4.3 `README.md` or `docs/feature-parent-integration.md`: **RED** migration/rollback/no-main/closure; **GREEN** operator/probe notes. Docs review. **Deps:** 4.2. U5.
