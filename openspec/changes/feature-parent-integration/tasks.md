# Tasks: Feature Parent Integration

## Review Workload Forecast

Estimate: 2,800–4,000 lines; 35–50 files/11 packages. Risks: Git/SQLite, races, recovery, resolver safety.

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

`single-pr`; 2,000 budget retains the 400-line guard. Future apply needs maintainer decision; no exception.

### Recommended units

| Unit | Focused proof | Runtime harness | Rollback boundary |
|---|---|---|---|
| 1 Ledger/features | `go test ./internal/ledger ./internal/feature` | Temp Git+SQLite | Disable v3 commands; retain data |
| 2 Admission/Git | `go test ./internal/packet ./internal/dag ./internal/worktree` | Temp Git | Targets/plumbing |
| 3 Attempts/promote | `go test ./internal/run ./internal/integrate` | Git/SQLite race | Orchestration |
| 4 Reconciliation | `go test ./internal/overlap ./internal/reconcile ./internal/resolve` | Git conflict fixture | Gate/candidate flow |
| 5 CLI/DTOs/docs | `go test ./cmd/lucind-ai ./internal/serve` | Git/SQLite CLI | User surfaces/docs |

## Phase 1: Foundation

- [ ] 1.1 Target `internal/ledger`: **RED** v3 schema, atomic state+audit, rollback gate/data retention; **GREEN** durable repositories. **Proof:** ledger tests. **Deps:** none. **Rollback:** Unit 1.
- [ ] 1.2 Target new `internal/feature`: **RED** `created→active→disabled`, declared-base parent, no rewrite/main mutation; **GREEN** lifecycle/audit. **Proof:** temp-Git test. **Deps:** 1.1. **Rollback:** Unit 1.
- [ ] 1.3 Target `internal/{ledger,feature}`: **RED** same-feature serialization, expired fencing/recovery, cross-feature concurrency; **GREEN** short owner/fence/expiry transactions. **Proof:** multi-handle SQLite. **Deps:** 1.1. **Rollback:** Unit 1.

## Phase 2: Targets and Git

- [ ] 2.1 Target `internal/{packet,dag,run}`, `cmd/lucind-ai/cli.go`: **RED** four-field acceptance; missing/implicit legacy rejection pre-side-effect. **GREEN** compile-safe adapter; `--legacy-main --expected-parent-sha` dispatch-only. **Proof:** package tests. **Deps:** 1.1. **Rollback:** Unit 2.
- [ ] 2.2 Target `internal/{worktree,integrate}`: **RED** relative/foreign/linked roots; forbidden/checked-out/deleted/non-commit refs, starts, stale CAS. **GREEN** absolute `GitRunner`, validation, scoped worktree, `update-ref` CAS. **Proof:** temp Git. **Deps:** 2.1. **Rollback:** Unit 2.
- [ ] 2.3 Target `internal/{run,integrate}`: **RED** terminal replay, unchanged-ref resume, changed-ref block, parent bisection/promotion. **GREEN** attempt machine, faults, explicit parent—not `HEAD`. **Proof:** fault tests. **Deps:** 1.2–2.2. **Rollback:** Unit 3.

## Phase 3: Reconciliation

- [ ] 3.1 Target new `internal/overlap`: **RED** bases/tips, special-file labels, hunks/hotspots/classes, missing structural disclosure. **GREEN** normalized Git evidence, configurable thresholds. **Proof:** table+Git fixtures. **Deps:** 1.2. **Rollback:** Unit 4.
- [ ] 3.2 Target `internal/{run,integrate,ledger}`: **RED** required blocks both promotions while dispatch continues; warning/info visible and nonblocking. **GREEN** affected-parent gate. **Proof:** two-parent integration. **Deps:** 3.1,2.3. **Rollback:** Unit 4.
- [ ] 3.3 Target new `internal/reconcile`, `internal/ledger`: **RED** exact direction, actor/SHA/evidence binding, decline/cancel/expiry/fresh renewal. **GREEN** service, audit, one candidate/request. **Proof:** state tests. **Deps:** 3.1. **Rollback:** Unit 4.
- [ ] 3.4 Target `internal/{resolve,integrate}`: **RED** out-of-scope/staged/`commit -a` leakage, empty index, markers, ambiguity, checks/timeout/400-line/stale refs preserve evidence. **GREEN** direction-only Sonnet, approved staging, revalidation/fenced target CAS. **Proof:** Git faults. **Deps:** 2.3,3.3. **Rollback:** Unit 4.

## Phase 4: Operations

- [ ] 4.1 Target new `internal/serve/model.go`: **RED** list/get/decide DTOs expose status/audit without shell calls. **GREEN** reconciliation adapter only; no server. **Proof:** DTO tests. **Deps:** 3.3. **Rollback:** Unit 5.
- [ ] 4.2 Target `cmd/lucind-ai/cli.go`, `internal/{run,ledger}`: **RED** create/status/recover and approve/decline/cancel/renew outputs, idempotency, audit. **GREEN** command wiring. **Proof:** `t.TempDir()` Git/SQLite CLI. **Deps:** 1.1–4.1. **Rollback:** Unit 5.
- [ ] 4.3 Target `README.md` or `docs/feature-parent-integration.md`: **RED** migration/rollback/no-main/closure review checklist. **GREEN** operator notes and `testing.Short()`-skipped real-Sonnet probe instructions. **Proof:** docs review. **Deps:** 4.2. **Rollback:** Unit 5.
