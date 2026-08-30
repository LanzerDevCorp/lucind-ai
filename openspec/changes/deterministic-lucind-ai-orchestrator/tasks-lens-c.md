# Tasks Lens C — Proof & Review Burden: Deterministic lucind-ai Orchestrator

## Assumed decomposition

The implementation decomposes into five units across the orchestration lifecycle:
- Unit 1: Canonical Claude skill updates, OpenCode skill tree creation, and cross-runtime preflight checks (`plugin/claude-code/skills/lucind-ai/`, `plugin/opencode/skills/lucind-ai/`, `cmd/lucind-ai/cli.go:267-280,303-361`).
- Unit 2: Target-free packet parsing and open-scope handling for omitted `allowed_paths` (`internal/packet/packet.go:78,119`, `internal/packet/disjoint.go:24-48`).
- Unit 3: Wave splitting, Kahn wave sequencing, and zero-exit wave advancement barriers (`internal/dag/split.go:18-46`, `internal/dag/waves.go:16-72`, `internal/run/batch.go:1-60`).
- Unit 4: Mechanical acceptance verification with fired hard-stop demotion to blocked, four-way diff boundary checks, and completion mode enforcement (`internal/run/run.go:868-893,901-923,969-980`).
- Unit 5: Idempotent attempt replay without redispatch, CAS promotion, and fail-closed parent feature recovery (`internal/run/attempt.go:217-256,576-682`, `internal/run/integrate_feature.go:26-77,80-140`, `cmd/lucind-ai/cli.go:540-554,802-805`).
Critical path runs Unit 1 and Unit 2 → Unit 3 and Unit 4 → Unit 5.

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 3200–4500 lines (2500–3500 lines replicated OpenCode skill tree and templates; 700–1000 lines Go runtime logic and tests) |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | single PR |
| Delivery strategy | ask-on-risk |
| Chain strategy | size-exception |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: Low

Basis for estimate: Replicating the canonical Claude skill tree (`plugin/claude-code/skills/lucind-ai/`) into `plugin/opencode/skills/lucind-ai/` comprises ~40 template and reference files totaling ~2500–3500 lines (`design.md:68-78`). The Go runtime changes span 9 production files adding ~250–350 lines of logic, plus ~450–650 lines of focused unit and integration tests across package suites (`cli_test.go:45-88`, `packet_test.go:16-49`, `run_test.go:962-985`, `attempt_test.go:128-150`, `split_test.go:13-57`). Under the active session's 5000-line review budget for `ask-on-risk`, the total change is well within budget and does not require PR chaining.

## RED Tests from the Threat Matrix

| Threat row | Applicable | RED test | Asserts | Production task it precedes |
|---|---|---|---|---|
| Documentation-like paths | N/A | — | — | — |
| Git repository selection | Applicable | `TestRunDispatch_RejectsLinkedWorktree`, `TestRunDispatch_SiblingWorktreeRejected`, `TestResolvePrimaryRoot_RelativeCwdResolvesToPrimary` | Asserts `cli.go` preflight fails before `worktree.Create` on linked or sibling worktrees and relative cwd resolves to primary root (`cli.go:267-280,303-361`, `worktree.go:292-313`) | CLI preflight at `runDispatch` and `runFeatureCreate` (`cmd/lucind-ai/cli.go:267-280,802-805`) |
| Commit state | Applicable | `TestEnforceCompletionMode_StagedLeftoverFails`, `TestEnforceCompletionMode_UntrackedLeftoverFails`, `TestEnforceCompletionMode_WriteWithZeroCommitsFails`, `TestEnforceCompletionMode_ReadOnlyWithCommitsFails` | Asserts uncommitted staged/untracked changes fail `PorcelainEmpty`, write packets without unique commits fail, and read-only packets with commits fail (`run.go:969-980`, `worktree.go:315-336,338-346`) | Completion mode verification in `decideStatus` flow (`internal/run/run.go:868-893,969-980`) |
| Push state | N/A | — | — | — |
| PR commands | N/A | — | — | — |

## Acceptance Evidence

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|
| Preflight & Skill Parity (`cmd/lucind-ai`, `plugin/`) | `go test -run TestRunPreflight_SkillParity ./cmd/lucind-ai` (`cmd/lucind-ai/cli_test.go:45-88`) | Preflight halts execution on skill mismatch between Claude and OpenCode or stale schema before worktree allocation. | Does not prove external OpenCode TypeScript process lifecycle under OS interrupts. |
| Linked Worktree Preflight (`cmd/lucind-ai/cli.go`) | `go test -run TestRunDispatch_RejectsLinkedWorktree ./cmd/lucind-ai` (`cmd/lucind-ai/cli_test.go:5033-5060`, `internal/worktree/worktree_test.go:16-50`) | `runDispatch` refuses invocation from linked worktrees and rejects sibling worktrees before allocating resources. | Does not prove git daemon health or file system permissions. |
| Target-Free Packet & Omitted Scope (`internal/packet`) | `go test -run 'TestParse_OmittedAllowedPaths\|TestParse_TargetFree' ./internal/packet` (`internal/packet/packet_test.go:16-49`, `internal/packet/disjoint_test.go:10-40`) | `packet.Parse` parses target-free templates without error, leaves `AllowedPaths` empty when omitted, and skips boundary checks safely. | Does not prove that the orchestrator fills bound target values at dispatch time. |
| Wave Split & Exit 0 Advance Barrier (`internal/dag`, `internal/run`) | `go test -run 'TestSplit_TwoWaveDAGSuccess\|TestWaves_OrderingAndYAMLOrderPreserved' ./internal/dag` (`internal/dag/split_test.go:13-57`, `internal/dag/waves_test.go:43-60`) | `dag.Split` generates per-wave commands preserving Kahn ordering and global reachability overlap constraints. | Does not prove downstream wave process execution when parent commits advance. |
| Hard-Stop Demotion & Completion Mode (`internal/run`) | `go test -run 'TestDecideStatus_FiredHardStopDemotes\|TestEnforceCompletionMode' ./internal/run` (`internal/run/run_test.go:962-985`, `internal/run/scope_test.go:15-50`) | `decideStatus` demotes `status=done` to `lane.Blocked` when `HardStop.Fired` is true, and validates clean porcelain / unique commits. | Does not prove executor agent honestly declared all fired hard stops in `result.json`. |
| Batch Admission & Target Validation (`internal/run`) | `go test -run 'TestFeatureTargetHomogeneousBatchNamesTheFeature\|TestFeatureTargetRejectsPacketWithNoDeclaredTarget' ./internal/run` (`internal/run/integrate_feature_test.go:55-80`, `internal/run/admission_test.go:16-50`) | `validatePacketAdmission` and `FeatureTarget` require complete target fields across all batch packets and fail before worktrees if mixed. | Does not prove remote branch refs exist on origin. |
| Attempt Replay & CAS Recovery (`internal/run`) | `go test -run 'TestAttemptReplayTerminalReturnsStoredResultWithoutSpies\|TestAttemptInterruptionAndRecoveryRefMismatchFailsClosed' ./internal/run` (`internal/run/attempt_test.go:128-150`, `internal/run/attempt_test.go:626-650`) | Terminal attempts return stored results without re-dispatching lanes, and recovery on parent ref mismatch fails closed preserving worktrees. | Does not prove automatic bisection resolution for non-mergeable code conflicts. |

## Verification Gaps

The following behaviors cannot be proven via hermetic in-repo tests alone:
1. Live cross-runtime multi-turn model interaction: Verifying prompt interpretations in live Claude Code and OpenCode sessions requires live external model provider APIs, which hermetic tests avoid (`specs/deterministic-orchestrator-contract/spec.md:5-25`).
2. Kernel-level OS crash recovery during SQLite WAL commit: Proving recovery from hard process kill during atomic transaction promotion requires fault-injection harnesses (`specs/parent-feature-integration/spec.md:5-23`).

## Open Questions

- [ ] Generic `~/.claude/skills/sdd-tasks/SKILL.md` specifies writing a single monolithic `tasks.md` with full checklist and Suggested Work Units; this packet explicitly executes parallel Lens C scoped to proof and workload forecast (`tasks-lens-c.md`), leaving checklist decomposition to Lens A and work-unit partitioning to Lens B.
- [ ] Generic `sdd-tasks` assumes a default 400-line budget; this session was explicitly configured with a 5000-line review budget under the `ask-on-risk` delivery strategy (`proposal.md:14-15,51-53`).

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:267-280` | preflight model validation across supported executors |
| `cmd/lucind-ai/cli.go:303-361` | primary root resolution, batch admission, feature target validation, and linked worktree barrier |
| `cmd/lucind-ai/cli.go:540-554` | runCheck bypasses resolvePrimaryRoot to test standing worktree code |
| `cmd/lucind-ai/cli.go:802-805` | resolvePrimaryRoot implementation using git rev-parse --git-common-dir |
| `cmd/lucind-ai/cli_test.go:45-88` | CLI flag, argument, and version handling test patterns |
| `cmd/lucind-ai/cli_test.go:5033-5060` | linked worktree check isolation test pattern |
| `internal/dag/overlap.go:54` | ValidateGlobalOverlap enforcement function signature |
| `internal/dag/split.go:18-46` | Split parses DAG, partitions waves, emits packets, and prints run commands |
| `internal/dag/split_test.go:13-57` | Split multi-wave command emission test pattern |
| `internal/dag/waves.go:16-72` | Waves Kahn algorithm wave grouping and global overlap validation |
| `internal/dag/waves_test.go:11-41` | Waves dependency cycle detection test pattern |
| `internal/dag/waves_test.go:43-60` | Waves Kahn ordering and YAML order preservation test pattern |
| `internal/overlap/overlap_test.go:37-60` | raw git diff capture test fixture helper |
| `internal/packet/disjoint.go:24-48` | DisjointAllowedPaths component-boundary prefix check |
| `internal/packet/disjoint_test.go:10-40` | PathInScope prefix and path matching test cases |
| `internal/packet/packet.go:78` | AllowedPaths packet struct field definition |
| `internal/packet/packet.go:119` | Parse packet scanner and frontmatter parser |
| `internal/packet/packet_test.go:16-49` | Parse frontmatter and body separation test pattern |
| `internal/run/admission_test.go:16-50` | admission target validation test pattern |
| `internal/run/attempt.go:217-256` | ExecuteAttempt idempotent replay and deduplication |
| `internal/run/attempt.go:576-682` | RecoverAttempt ref verification, post-crash finalization, and blocked recovery |
| `internal/run/attempt_test.go:128-150` | terminal attempt replay without side-effects test pattern |
| `internal/run/attempt_test.go:626-650` | attempt recovery on ref mismatch fails closed test pattern |
| `internal/run/batch.go:1-60` | ExecuteBatch concurrent lane execution, barrier join, and deadline rules |
| `internal/run/batch_test.go:219-250` | ExecuteBatch barrier release and integration test pattern |
| `internal/run/integrate_feature.go:26-77` | FeatureTarget homogeneous target validation and template rejection |
| `internal/run/integrate_feature.go:80-140` | IntegrateFeature CAS promotion and lane revert |
| `internal/run/integrate_feature_test.go:55-80` | FeatureTarget homogeneous batch target name test pattern |
| `internal/run/integrate_feature_test.go:142-160` | FeatureTarget rejection of packet without declared target test pattern |
| `internal/run/run.go:868-893` | decideStatus outcome evaluation, envelope parsing, and status mapping |
| `internal/run/run.go:901-923` | enforceAllowedPaths 4-way diff inspection and out-of-scope demotion |
| `internal/run/run.go:969-980` | enforceCompletionMode unique commits and clean porcelain verification |
| `internal/run/run_test.go:962-985` | decideStatus terminal status transition test pattern |
| `internal/run/scope_test.go:15-50` | enforceAllowedPaths 4-way copy-aware diff test pattern |
| `internal/worktree/worktree.go:292-313` | IsLinkedWorktree dot-git file gitdir detection |
| `internal/worktree/worktree.go:315-336` | HasUniqueCommits merge-base comparison |
| `internal/worktree/worktree.go:338-346` | PorcelainEmpty git status check |
| `internal/worktree/worktree_test.go:16-50` | IsLinkedWorktree directory vs gitdir file test pattern |
| `internal/worktree/worktree_test.go:525-550` | HasUniqueCommits unique commits vs base test pattern |
| `internal/worktree/worktree_test.go:666-700` | PorcelainEmpty clean vs dirty status test pattern |
| `openspec/changes/deterministic-lucind-ai-orchestrator/design.md:68-78` | design File Changes table specifying terminal consumers |
| `openspec/changes/deterministic-lucind-ai-orchestrator/design.md:86-97` | design Testing Strategy and Test Seams across unit, integration, and E2E layers |
| `openspec/changes/deterministic-lucind-ai-orchestrator/design.md:104-109` | design Threat Matrix defining boundary verdicts and planned RED tests |
| `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:14-15` | proposal out-of-scope boundaries prohibiting new lifecycle states or schedulers |
| `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:51-53` | proposal rollback plan and non-destructive invariant requirements |
| `openspec/changes/deterministic-lucind-ai-orchestrator/specs/acceptance-verifier/spec.md:5-39` | acceptance-verifier modified requirement and hard-stop demotion scenarios |
| `openspec/changes/deterministic-lucind-ai-orchestrator/specs/deterministic-orchestrator-contract/spec.md:5-25` | deterministic-orchestrator-contract cross-runtime preflight requirement and scenarios |
| `openspec/changes/deterministic-lucind-ai-orchestrator/specs/packet-authoring-contract/spec.md:5-29` | packet-authoring-contract versioned contract and omitted allowed_paths scenarios |
| `openspec/changes/deterministic-lucind-ai-orchestrator/specs/parent-feature-integration/spec.md:5-23` | parent-feature-integration recoverable idempotent attempts requirement and scenarios |
| `openspec/changes/deterministic-lucind-ai-orchestrator/specs/sdd-apply/spec.md:5-18` | sdd-apply orchestrator wave zero-exit advancement requirement and scenarios |
