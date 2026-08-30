# Tasks Lens A — Decomposition & Ordering: Deterministic lucind-ai Orchestrator

## Assumed decomposition

We decompose the deterministic orchestrator into five sequential phases covering cross-runtime skill parity, contract parsing and DAG splitting, runtime status demotion, idempotent attempt integration, and CLI preflight enforcement. Phase 1 establishes the canonical Claude skill tree and its identical OpenCode replica; Phase 2 updates packet parsing for target-free templates and stdout wave splitting; Phase 3 implements fail-closed status demotion for fired hard stops and batch isolation; Phase 4 hardens idempotent attempts and compare-and-swap promotion; and Phase 5 binds preflight checks across CLI entrypoints. The critical path runs through Phase 2 (packet authoring) -> Phase 3 (mechanical acceptance demotion) -> Phase 4 (idempotent attempt integration) -> Phase 5 (CLI preflight gates).

## Phase 1: Orchestrator Skill Parity & Baseline

- [ ] 1.1 Update `plugin/claude-code/skills/lucind-ai/SKILL.md:14-28` and references to specify cross-runtime preflight, late target binding, and multi-wave execution barriers.
- [ ] 1.2 Create `plugin/opencode/skills/lucind-ai/` as a byte-identical copy of `plugin/claude-code/skills/lucind-ai/` for OpenCode orchestrator parity (new file).

## Phase 2: Packet Authoring & DAG Splitting

- [ ] 2.1 Update `internal/packet/packet.go:73-92,187-194` to support target-free templates and allow omitted `allowed_paths` to default to open scope without validation failures.
- [ ] 2.2 Update `internal/dag/split.go:18-49` in `Split` to format and print copy-pasteable `lucind-ai run` wave commands to stdout in dependency order without writing on-disk plan files.

## Phase 3: Runtime Execution & Acceptance Enforcement

- [ ] 3.1 Update `internal/run/run.go:549-573,868-893` in `decideStatus` to demote lane status to `lane.Blocked` when any declared hard stop fires, regardless of top-level envelope status.
- [ ] 3.2 Update `internal/run/batch.go:29-89` in `ExecuteBatch` to preserve lane independence and barrier outcome reporting during multi-lane wave execution.

## Phase 4: Feature Integration & Idempotent Attempts

- [ ] 4.1 Update `internal/run/attempt.go:217-256,576-610` in `ExecuteAttempt` and `RecoverAttempt` to return stored terminal results on replay and verify expected parent references on recovery.
- [ ] 4.2 Update `internal/run/integrate_feature.go:26-78,100-140` in `FeatureTarget` and `IntegrateFeature` to require homogeneous bound targets and preserve worktrees and ledger rows on failed promotion.

## Phase 5: CLI Preflight & Entrypoint Wiring

- [ ] 5.1 Update `cmd/lucind-ai/cli.go:260-326,784-815` in `runDispatch` and `runFeatureCreate` to verify skill parity, embedded schema freshness, and primary root isolation before worktree allocation.

## Dependency Order

| Task | Depends on | Why |
|---|---|---|
| 1.1 | — | Establishes orchestrator contract; independent of runtime code. |
| 1.2 | 1.1 | OpenCode skill tree must replicate the finalized canonical Claude skill tree byte-for-byte. |
| 2.1 | — | Target-free packet parsing and open scope rules are independent of skills and runtime execution. |
| 2.2 | 2.1 | DAG wave splitting consumes packet frontmatter parsing rules. |
| 3.1 | 2.1 | Result evaluation and hard-stop demotion validate packet authoring outputs. |
| 3.2 | 3.1 | Batch barrier and lane outcome aggregation require per-lane `decideStatus` demotion logic. |
| 4.1 | — | Attempt idempotency and recovery state machine can be built independently against ledger storage. |
| 4.2 | 2.1, 4.1 | Feature integration consumes bound packet target structures (2.1) and invokes `ExecuteAttempt` (4.1). |
| 5.1 | 1.2, 2.1, 3.2, 4.2 | CLI preflight verifies skill parity (1.2), validates bound targets (2.1), and orchestrates batch execution (3.2) and feature integration (4.2). |

## Requirement Traceability

| Requirement | Tasks |
|---|---|
| `specs/deterministic-orchestrator-contract/spec.md:5-25` | 1.1, 1.2, 5.1 |
| `specs/packet-authoring-contract/spec.md:5-29` | 2.1, 4.2, 5.1 |
| `specs/sdd-apply/spec.md:5-18` | 1.1, 2.2, 3.2 |
| `specs/acceptance-verifier/spec.md:5-39` | 3.1, 3.2 |
| `specs/parent-feature-integration/spec.md:5-23` | 4.1, 4.2 |

## Open Questions

- [ ] None

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:260-326` | `runDispatch` input validation, model check, disjointness, and late target admission |
| `cmd/lucind-ai/cli.go:784-815` | `gitShowToplevel` and `resolvePrimaryRoot` implementations for repository root resolution |
| `internal/dag/split.go:18-49` | `Split` emits wave commands to stdout in dependency order with multi-wave guidance |
| `internal/packet/packet.go:73-92` | `Packet` struct defines target fields (`Feature`, `ParentRef`, `BaseSHA`, `ExpectedParentSHA`) and `AllowedPaths` |
| `internal/packet/packet.go:187-194` | `Parse` unmarshals `allowed_paths` JSON array |
| `internal/run/attempt.go:217-256` | `ExecuteAttempt` performs terminal replay and idempotent lookup |
| `internal/run/attempt.go:576-610` | `RecoverAttempt` verifies parent ref and recovers interrupted attempt state |
| `internal/run/batch.go:29-89` | `ExecuteBatch` runs concurrent lanes via `sync.WaitGroup` with non-cancelling barrier |
| `internal/run/integrate_feature.go:26-78` | `FeatureTarget` checks homogeneous target fields across batch packets |
| `internal/run/integrate_feature.go:100-140` | `IntegrateFeature` executes attempt and reverts lanes on promotion failure |
| `internal/run/run.go:549-573` | `runOneLane` evaluates status and freezes done candidate or records ledger status |
| `internal/run/run.go:868-893` | `decideStatus` validates result envelope and handles exit codes |
| `openspec/changes/deterministic-lucind-ai-orchestrator/design.md:16-19` | Architecture Decision 2 specifies preflight checks at CLI barriers before worktree allocation |
| `openspec/changes/deterministic-lucind-ai-orchestrator/design.md:22-26` | Architecture Decision 3 specifies target-free templates with late target binding and open scope for omitted `allowed_paths` |
| `openspec/changes/deterministic-lucind-ai-orchestrator/design.md:28-32` | Architecture Decision 4 specifies fail-closed hard-stop demotion in `decideStatus` |
| `openspec/changes/deterministic-lucind-ai-orchestrator/design.md:34-38` | Architecture Decision 5 specifies wave advancement only on zero exit |
| `openspec/changes/deterministic-lucind-ai-orchestrator/design.md:40-44` | Architecture Decision 6 specifies idempotent attempts and CAS promotion |
| `openspec/changes/deterministic-lucind-ai-orchestrator/design.md:53-65` | Flow and Invariants sequence (Preflight, Bind+Admit, Split/Waves, Barrier, Evidence, Integrate) |
| `openspec/changes/deterministic-lucind-ai-orchestrator/design.md:68-78` | File Changes table defining actions and terminal consumers for all 9 modified/created artifacts |
| `openspec/changes/deterministic-lucind-ai-orchestrator/specs/acceptance-verifier/spec.md:5-39` | Fail-Closed Mechanical Criteria requirement and scenarios including fired hard-stop demotion |
| `openspec/changes/deterministic-lucind-ai-orchestrator/specs/deterministic-orchestrator-contract/spec.md:5-25` | Cross-Runtime Orchestrator Preflight and Sequencing requirement and scenarios |
| `openspec/changes/deterministic-lucind-ai-orchestrator/specs/packet-authoring-contract/spec.md:5-29` | Versioned Contract and Late Target Binding requirement and scenarios |
| `openspec/changes/deterministic-lucind-ai-orchestrator/specs/parent-feature-integration/spec.md:5-23` | Recoverable Idempotent Attempts requirement and scenarios |
| `openspec/changes/deterministic-lucind-ai-orchestrator/specs/sdd-apply/spec.md:5-18` | Orchestrator Advances Only on a Passing Wave requirement and scenarios |
| `plugin/claude-code/skills/lucind-ai/SKILL.md:14-28` | Hard rules defining orchestrator authority, primary root execution, and packet location |
