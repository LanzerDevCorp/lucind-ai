# Tasks Lens B — Partition & Dispatch Shape: Deterministic lucind-ai Orchestrator

## Assumed decomposition

The change decomposes into five standalone deliverables derived from the design's file-changes table (`design.md:68-78`): (1) Skill contract updates and OpenCode byte-identical replica; (2) Target-free packet parsing and DAG split emissions; (3) Runtime status demotion, scope enforcement, and completion mode invariants; (4) Idempotent attempt replay, CAS recovery, and batch target validation; and (5) CLI preflight checks and truthful dispatch reporting. Critical path flows from foundational packet/DAG parsing and runtime execution invariants to CLI preflight wiring (`cli.go:804-826`), while the prompt skill tree remains independent.

## Suggested Work Units

| Unit | Goal | allowed_paths | Executor | Rollback boundary |
|---|---|---|---|---|
| 1 | Author deterministic orchestrator prompt contracts in Claude skill/references, establish byte-identical OpenCode replica, and verify tree parity (`design.md:9-13`). | `plugin/claude-code/skills/lucind-ai/`<br>`plugin/opencode/skills/lucind-ai/` | `agy` | Reverting restores prior orchestrator instructions and deletes OpenCode copy without impacting Go binaries or test suites. |
| 2 | Implement target-free template parsing and omitted `allowed_paths` skip semantics in `packet.Parse` (`packet.go:118-239`), and emit target-preserving DAG split packets without plan files (`split.go:18-43`). | `internal/packet/packet.go`<br>`internal/packet/packet_test.go`<br>`internal/dag/split.go`<br>`internal/dag/split_test.go` | `cursor-agent` | Reverting restores strict target requirement in packet parsing and baseline DAG emission without affecting runtime execution. |
| 3 | Demote fired hard stops to `lane.Blocked` in `decideStatus` (`run.go:868-892`), enforce four-way diff scope against `Worktree.BaseSHA`, and enforce clean porcelain/unique commits for write vs read-only lanes (`run.go:486-503`). | `internal/run/run.go`<br>`internal/run/run_test.go`<br>`internal/run/scope_test.go`<br>`internal/run/admission_test.go` | `cursor-agent` | Reverting restores 1:1 envelope status mapping and prior scope/porcelain enforcement in `Execute`. |
| 4 | Implement idempotent attempt replay without redispatch (`attempt.go:217-256`), fail-closed CAS recovery on parent SHA mismatch (`attempt.go:596-682`), and homogeneous bound-target validation in `integrate_feature.go` (`integrate_feature.go:26-77`). | `internal/run/attempt.go`<br>`internal/run/attempt_test.go`<br>`internal/run/batch.go`<br>`internal/run/batch_test.go`<br>`internal/run/integrate_feature.go`<br>`internal/run/integrate_feature_test.go` | `cursor-agent` | Reverting restores previous attempt recovery and batch target handling without touching `run.go` scope checks or CLI commands. |
| 5 | Add fail-closed preflight at `resolvePrimaryRoot` and dispatch barriers before worktree allocation for linked worktrees (`worktree.go:299-313`), stale skills, and schema freshness; wire truthful `integrated_ids` and `reverted_ids` reporting (`cli.go:267-280`, `cli.go:804-826`). | `cmd/lucind-ai/cli.go`<br>`cmd/lucind-ai/cli_test.go` | `cursor-agent` | Reverting restores baseline CLI dispatch preflight and reporting without altering internal packet or run primitives. |

## Wave Plan

| Wave | Units | Runs in parallel | Green on its own |
|---|---|---|---|
| 1 | 1, 2, 3, 4 | Yes | Yes. Unit 1 is prompt/doc only; Units 2, 3, and 4 modify disjoint Go packages/files that compile and pass `lucind-checks.sh` independently (`integrate.go:50-60`). |
| 2 | 5 | No | Yes. Unit 5 wires CLI preflight and calls into the merged Wave 1 runtime and packet APIs; passes `lucind-checks.sh` on the combined tree (`integrate.go:50-60`). |

## Disjointness Check

- **Wave 1 (Pair 1, 2)**: `plugin/...` vs `internal/packet/...`, `internal/dag/...` — Disjoint (disjoint top-level directories `plugin` vs `internal` under `disjoint.go:13-22`).
- **Wave 1 (Pair 1, 3)**: `plugin/...` vs `internal/run/run.go`, `internal/run/run_test.go`, `internal/run/scope_test.go`, `internal/run/admission_test.go` — Disjoint (`plugin` vs `internal` under `disjoint.go:13-22`).
- **Wave 1 (Pair 1, 4)**: `plugin/...` vs `internal/run/attempt.go`, `internal/run/attempt_test.go`, `internal/run/batch.go`, `internal/run/batch_test.go`, `internal/run/integrate_feature.go`, `internal/run/integrate_feature_test.go` — Disjoint (`plugin` vs `internal` under `disjoint.go:13-22`).
- **Wave 1 (Pair 2, 3)**: `internal/packet/...`, `internal/dag/...` vs `internal/run/run.go`, `internal/run/run_test.go`, `internal/run/scope_test.go`, `internal/run/admission_test.go` — Disjoint (disjoint subpackages `packet`, `dag` vs `run` under `disjoint.go:13-22`).
- **Wave 1 (Pair 2, 4)**: `internal/packet/...`, `internal/dag/...` vs `internal/run/attempt.go`, `internal/run/attempt_test.go`, `internal/run/batch.go`, `internal/run/batch_test.go`, `internal/run/integrate_feature.go`, `internal/run/integrate_feature_test.go` — Disjoint (disjoint subpackages `packet`, `dag` vs `run` under `disjoint.go:13-22`).
- **Wave 1 (Pair 3, 4)**: `{run.go, run_test.go, scope_test.go, admission_test.go}` vs `{attempt.go, attempt_test.go, batch.go, batch_test.go, integrate_feature.go, integrate_feature_test.go}` — Disjoint (both inside `internal/run/` but naming exact concrete files with zero intersection and no directory prefix under `disjoint.go:13-22`, `disjoint.go:29-48`).
- **Wave 2**: Unit 5 only (`cmd/lucind-ai/cli.go`, `cmd/lucind-ai/cli_test.go`) — Single-unit wave requires no pairwise check.

## Sidecar Recommendation

**Recommendation**: Single packet, no sidecar.
**Rationale**: While the 5 work units can be grouped into 2 valid waves, the total change is compact (~9 files, ~600–1000 lines across Go and prompt templates) with closely coupled runtime semantics (`design.md:68-78`). Applying the change sequentially in a single packet avoids sidecar orchestration overhead (`apply-dag.yaml`, packet body emission, and intermediate checkout advancement across waves), following the direct precedent in `2026-08-20-apply-dag-dispatch-hardening/tasks.md:26-27`. Furthermore, strict TDD RED/GREEN assertions belong in the same lane to ensure every batch passes the `Integrate` gate (`integrate.go:50-60`, `apply-dag.yaml:8-13`).

## Open Questions

- [ ] None. The precedence between `~/.claude/skills/sdd-tasks/SKILL.md` (content authority) and packet instructions (fan-out execution authority) is fully respected with out-of-scope sections deferred to sibling lenses.

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:267-280` | Preflight model validation verifies known executor models before dispatching worktrees |
| `cmd/lucind-ai/cli.go:804-826` | `resolvePrimaryRoot` resolves the primary repository root using `git rev-parse --git-common-dir` |
| `internal/dag/parse.go:22-37` | `Node` struct defines sidecar packet fields including `ID`, `Executor`, `AllowedPaths`, and `DependsOn` |
| `internal/dag/split.go:18-43` | `Split` parses `apply-dag.yaml`, groups nodes into waves, emits packet files, and prints `lucind-ai run` commands to stdout |
| `internal/dag/waves.go:16-72` | `Waves` computes execution waves using Kahn's algorithm and validates global path overlap across the DAG |
| `internal/packet/disjoint.go:13-22` | `PathInScope` implements component-boundary prefix matching over normalized repository-relative paths |
| `internal/packet/disjoint.go:29-48` | `DisjointAllowedPaths` verifies pairwise disjointness of declared `AllowedPaths` across packets, skipping undeclared ones |
| `internal/packet/packet.go:74-77` | `AllowedPaths` field restricts editable paths, where empty or omitted slices skip scope checks |
| `internal/packet/packet.go:118-239` | `Parse` parses packet frontmatter without requiring target fields in reusable templates |
| `internal/run/attempt.go:217-256` | `ExecuteAttempt` performs idempotent attempt lookup and replay without redispatching completed lanes |
| `internal/run/attempt.go:596-682` | `RecoverAttempt` verifies current parent SHA against expected parent SHA and fails closed on mismatch |
| `internal/run/integrate.go:50-60` | `Integrate` executes `RunChecks` against combined tree and triggers bisection on failure |
| `internal/run/integrate_feature.go:26-77` | `FeatureTarget` validates complete, homogeneous target fields across dispatched batch packets |
| `internal/run/run.go:486-503` | `Execute` enforces status decisions, allowed paths, required skills, and completion modes |
| `internal/run/run.go:868-892` | `decideStatus` reads `.lucind/result.json` to determine lane terminal status and demote unreadable envelopes |
| `internal/worktree/worktree.go:299-313` | `IsLinkedWorktree` checks whether a path is a linked worktree by examining the `.git` file content |
| `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md:26-27` | Apply DAG dispatch hardening declined `apply-dag.yaml` sidecar because small units do not pay for orchestration overhead |
| `openspec/changes/archive/2026-08-21-sdd-fan-out-lens/apply-dag.yaml:8-13` | Strict-TDD RED/GREEN wave splitting fails Integrate check gate and necessitates single-lane execution |
| `openspec/changes/deterministic-lucind-ai-orchestrator/design.md:9-13` | Architecture Decision 1 establishes two-layer split between canonical Claude skill, OpenCode byte copy, and enforcing Go runtime |
| `openspec/changes/deterministic-lucind-ai-orchestrator/design.md:15-19` | Architecture Decision 2 places fail-closed preflight checks at existing CLI barriers before worktree allocation |
| `openspec/changes/deterministic-lucind-ai-orchestrator/design.md:21-25` | Architecture Decision 3 establishes target-free templates with late binding at wave dispatch and fail-closed admission |
| `openspec/changes/deterministic-lucind-ai-orchestrator/design.md:27-32` | Architecture Decision 4 demotes envelopes with fired hard stops or undeclared diffs to blocked/deviated |
| `openspec/changes/deterministic-lucind-ai-orchestrator/design.md:68-78` | File Changes table lists all modified and created files with terminal consumers |
| `openspec/changes/deterministic-lucind-ai-orchestrator/design.md:86-98` | Testing Strategy and Test Seams table outlines test seams across unit, integration, and E2E layers |
| `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:14-18` | Out of scope items exclude new lifecycle states, schedulers, flags, or replacement of core Combine/CAS primitives |
| `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:51-53` | Rollback plan mandates independent reversibility of skill, parity, and additive runtime commits |
