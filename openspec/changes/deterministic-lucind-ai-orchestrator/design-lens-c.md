# Design Lens C — Failure, Test & Rollback: Deterministic lucind-ai Orchestrator

## Assumed architecture

This design assumes a two-layer architecture: a canonical orchestrator skill in `plugin/claude-code/skills/lucind-ai/` mirrored byte-for-byte in `plugin/opencode/skills/lucind-ai/`, paired with narrow, fail-closed Go runtime enforcement. The runtime extends `cmd/lucind-ai` (preflight and reporting), `internal/packet` (target-free templates and open-scope defaulting), and `internal/run` (`Deps` extensions, `FeatureTarget` late binding, and frozen evidence verification), while reusing existing `internal/dag`, `internal/ledger`, `internal/integrate`, and `internal/worktree` CAS and isolation primitives without new lifecycle states or schedulers.

## Testing Strategy

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|
| Unit | Target-free packet parsing | Table tests parsing omitted target fields and open-scope `allowed_paths` defaulting | `internal/packet/packet.go:78` (`packet.Parse`) |
| Unit | Skill parity across runtimes | Byte-for-byte tree comparison between Claude Code canonical skill and OpenCode mirror | New seam required: parity check helper |
| Unit | Wave barriers & path overlap | Kahn grouping and acyclic path overlap verification | `internal/dag/waves.go:16` (`dag.Waves`), `internal/dag/overlap.go:54` (`dag.ValidateGlobalOverlap`) |
| Unit | Frozen evidence & hard stops | Demote lane on fired hard stop or unapproved paths despite green criteria | `internal/run/run.go:549` (`decideStatus`), `internal/run/run.go:582` (`enforceAllowedPaths`), `internal/run/run.go:163` (`Deps.WorktreeFS`) |
| Unit | Completion mode verification | Verify unique commits and porcelain status for write vs read-only modes | `internal/run/run.go:663` (`enforceCompletionMode`), `internal/run/run.go:181` (`Deps.HasUniqueLaneCommits`), `internal/run/run.go:182` (`Deps.PorcelainEmpty`) |
| Unit | Idempotent retry & lease recovery | Replay completed attempt without redispatch; fail closed on ref mismatch | `internal/run/attempt.go:245` (`recoverAttemptInternal`), `internal/run/run.go:152` (`Deps.Ledger`) |
| Integration | CLI dispatch late binding | Drive batch dispatch with runtime-supplied target ref and base SHA | `cmd/lucind-ai/cli.go:132` (`runDispatch`), `cmd/lucind-ai/cli.go:60` (`depsFactory`) |
| Integration | Concurrent ledger persistence | Multi-connection SQLite WAL execution under `-race` asserting truthful status | `internal/ledger/ledger.go:146` (`ledger.Open`), `internal/ledger/ledger_test.go:24` (`openTestLedger`) |
| Integration | Feature CAS promotion | Merge combined tree, run checks, and CAS promote parent ref | `internal/run/integrate_feature.go:100` (`IntegrateFeature`), `internal/run/run.go:199` (`Deps.PromoteCAS`) |
| E2E | Preflight & CLI reporting | Subcommand execution verifying fail-closed preflight and terminal summaries | `cmd/lucind-ai/cli.go:571` (`resolvePrimaryRoot`), `cmd/lucind-ai/cli_test.go:37` (`TestRunNoArgsPrintsUsageAndFails`) |

## Test Seams

- `run.Deps` (`internal/run/run.go:149-212`): Full injection struct for `Ledger`, `LookupExecutor`, `CreateWorktree`, `WorktreeFS`, `Now`, `LaneTimeout`, `ApprovalTimeout`, `HasUniqueLaneCommits`, `PorcelainEmpty`, `CombineTree`, `RunChecks`, `PromoteTarget`, `PromoteCAS`, `ResolveRefSHA`, `ResolveCandidateSHA`, and `FeatureLeaseTTL`.
- `worktree.GitRunner` (`internal/worktree/worktree.go:47-49`): Mockable git command execution interface.
- `depsFactory` (`cmd/lucind-ai/cli.go:60`): Package hook enabling CLI tests to inject dependency doubles.
- `resolve.Invoker` (`internal/resolve/resolve.go:21`): Injectable conflict resolution double.
- `io.Reader` packet stream (`internal/packet/packet.go:78`): In-memory frontmatter parsing seam.
- Temporary SQLite fixture (`internal/ledger/ledger_test.go:24`): Real on-disk SQLite ledger tested under `-race`.
- New seams required: (1) preflight hook in `cmd/lucind-ai/cli.go` to inject custom skill roots/schemas before worktree allocation; (2) cross-runtime file tree comparator for Claude/OpenCode skill parity tests.

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | N/A: packet markdown parsed as prompt text without execution; path scoping uses prefix containment | Classification and execution boundary | N/A — no executable file classification boundary |
| Git repository selection | `git -C`, relative paths, absolute paths | Applicable | Authority resolves to primary root; linked/sibling worktrees fail closed before allocation | (1) Test dispatch in linked worktree fails closed (`git -C`); (2) test relative subdirectory resolves to primary root; (3) test absolute path targeting sibling worktree is rejected |
| Commit state | staged, `commit -a`, empty index | Applicable | `enforceCompletionMode` and `enforceAllowedPaths` require clean porcelain; write packets require unique commits, read-only packets require zero unique commits | (1) Test write packet with uncommitted staged index fails completion; (2) test write packet with `commit -a` leaving untracked files fails porcelain check; (3) test write packet with empty commit index fails unique commits check; (4) test read-only packet with commits/staged changes fails verification |
| Push state | tracking branch, first push, explicit refspec | N/A: operations are local branches, local worktrees, and local CAS ref updates; no remote push | Destination/ref resolution | N/A — no remote push boundary |
| PR commands | explicit `--head`, environment prefix, composed commands | N/A: PR creation and upstream review are human/RDD-owned; lucind-ai automates local wave CAS promotion only | Argument composition and ownership | N/A — no PR automation boundary |

## Rollback and Additivity

**Choice**: Revert skill/reference, parity-test, and additive runtime commits independently; preserve existing packet, ledger, lifecycle, and CAS formats without rewriting prior evidence (`proposal.md:51-53`).
**Alternatives considered**: Schema migration script with ledger rewrite (rejected: breaks immutable audit history and risks locks); monolithic all-or-nothing revert (rejected: two-layer separation permits independent skill vs runtime rollbacks).
**Rationale**: Format changes are strictly additive. Target-free packet parsing maintains backward compatibility with legacy and explicit feature targets. SQLite ledger tables (`integration_attempts`, `lanes`, `events`) and `.lucind/result.schema.json` are unchanged. Reverting runtime commits restores earlier preflight and enforcement behavior cleanly with zero database migration or evidence loss.

## Out of Scope

- Introducing new lifecycle states, wave execution engines, CLI flags, or replacing `Combine`/`Resolve`/`Check`/`PromoteCAS` primitives (`proposal.md:14-15`).
- Automatic PR creation, model selection, or semantic qualitative review approvals (human/RDD-owned).
- Cross-fork orchestration or distributed multi-repository coordination.
- Touching or depending on unmerged capabilities from the in-flight `feature/skill-provisioning-and-phase-specialist` branch.

## Open Questions

- [ ] Drift with `sdd-design` SKILL.md: Skill prescribes an 800-word monolithic `design.md` with Engram persistence; this packet scopes Lens C as a parallel <=1000-word draft in `design-lens-c.md` without Engram persistence.
- [ ] Preflight CLI surface: Confirm whether preflight parity checks run inside `runDispatch` / `lucind-ai check` or expose a dedicated subcommand.
