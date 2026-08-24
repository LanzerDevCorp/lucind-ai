# Design Lens C — Failure, Test & Rollback: Conflict Triage Fixture

## Assumed architecture

This design introduces two packages: `internal/conflicttriage/` implementing an advisory, fail-open triage agent that populates structured JSON into `Candidate.Output` (`internal/reconcile/reconcile.go:105`), and `internal/conflicttriage/fixture/` providing a 3-hunk conflict generator and offline dual-judge A/B rubric. Existing gate evaluation (`internal/run/attempt.go:687-880`), CAS promotion (`internal/integrate/integrate.go:151-173`), and ledger schemas (`internal/ledger/schema.go:163`) remain unmodified. Crucially, `CandidateSHA` (`internal/reconcile/reconcile.go:107`) is written exclusively by the human operator via `reconcile resolve --candidate --sha` (`cmd/lucind-ai/cli.go:56`), never directly by the triage agent. Post-triage invariant checks reuse `ScanConflictMarkers` and `EnforceAllowedPaths` (`internal/resolve/candidate.go:48-95,100-145`) without inheriting `resolve`'s fail-closed `ErrSemanticAmbiguity` (`internal/resolve/candidate.go:26,303-312`).

## Testing Strategy

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|
| Unit: Triage Agent | Fail-open execution; ARBITRARY label + `high` risk ratchet on business hunk; deterministic resolution on mechanical hunks; wall-clock budget format (`~N min: <cmd>`); no `ErrSemanticAmbiguity` | Table-driven tests invoking triage logic with mocked agent responses | new seam required (`conflicttriage.TriageInvoker` func field) |
| Unit: Invariants | Post-triage verification rejecting residual conflict markers and out-of-scope diffs | Unit assertions with clean/dirty fixture worktrees and staged/unstaged diffs | `internal/resolve/candidate.go:48-95` (`ScanConflictMarkers`), `internal/resolve/candidate.go:100-145` (`EnforceAllowedPaths`) |
| Unit: Fixture Generator | Generator creates two features sharing one `base_sha` (`internal/feature/feature.go:118-133`); 3-hunk toy file triggers `overlap.ClassRequired` | Execute generator in temp repo; evaluate signals with `DefaultThresholds` | `internal/overlap/overlap.go:93-98` (`DefaultThresholds`), `internal/overlap/overlap.go:623-659` (`Classify`) |
| Integration: Overlap Gate | Shared `base_sha` creates awaiting request and blocks attempt (`AttemptStatusBlocked`); missing `base_sha` triggers `ErrNoMergeBase` and skips gate | Run `evaluateOverlapGate` against in-memory ledger with injected `EvaluateOverlap` | `internal/run/run.go:211` (`Deps.EvaluateOverlap`), `internal/run/attempt.go:687-880` (`evaluateOverlapGate`), `internal/reconcile/reconcile.go:157-168` (`NewService`), `internal/reconcile/reconcile.go:213-336` (`CreateRequest`) |
| Integration: CLI & CAS Retry | `reconcile approve` (`internal/reconcile/reconcile.go:406-535`) creates candidate; operator `reconcile resolve` sets `CandidateSHA`; retry unblocks and CAS-promotes (`internal/run/attempt.go:821-828,870`); tip movement re-blocks; linked worktree rejected | Subcommand execution via CLI test harness on temporary git checkouts | `cmd/lucind-ai/cli.go:60` (`depsFactory`), `internal/integrate/integrate.go:151-173` (`PromoteCASWithRunner`), `internal/worktree/worktree.go:278-292` (`IsLinkedWorktree`) |
| Qualitative: Dual-Judge A/B | Rubric evaluates identical fixture across `claude`/`claude-opus-5` and `opencode`/`openai/gpt-5.6-sol`; scores 3-hunk separation and ARBITRARY label; rejects uniform scores; stream fallback resilience | Offline evaluation harness dispatching registered executors on fixture packets | `cmd/lucind-ai/cli.go:65-70` (`supportedExecutors`), `internal/executor/claude.go:30-52` (`Claude`), `internal/executor/claude.go:106-122` (stream decode fallback), `internal/executor/opencode.go:50-65` (`Opencode`) |

## Test Seams

### Existing Seams
- `reconcile.NewService` options (`internal/reconcile/reconcile.go:128-145,157-168`): Injects custom clock, ID generator, and `OverlapEvaluator` without database or git mocks.
- `run.Deps.EvaluateOverlap` (`internal/run/run.go:211`): Injects deterministic overlap evidence into `evaluateOverlapGate` (`internal/run/attempt.go:687-880`), verified in `internal/run/gate_test.go:122-140`.
- `integrate.PromoteCASWithRunner` (`internal/integrate/integrate.go:151-173`): Accepts injected `worktree.GitRunner` to test CAS atomic reference updates and stale-ref failures.
- `worktree.IsLinkedWorktree` (`internal/worktree/worktree.go:278-292`): Pure filesystem predicate checking `gitdir:` pointers to validate primary vs linked worktree execution.
- `resolve.ScanConflictMarkers` (`internal/resolve/candidate.go:48-95`) & `resolve.EnforceAllowedPaths` (`internal/resolve/candidate.go:100-145`): Standalone invariant validators tested in `internal/resolve/candidate_test.go:16-49,51-93`.
- `cmd/lucind-ai` test hooks (`cmd/lucind-ai/cli.go:60,65-70`): `depsFactory` and `supportedExecutors` allow integration tests (`cmd/lucind-ai/cli_test.go:2944,3126`) to stub runtime dependencies and route to registered executors.

### New Seams Required
- `conflicttriage.TriageInvoker`: Functional injection point `type TriageInvoker func(ctx context.Context, req TriageRequest) (TriageResponse, error)` in `internal/conflicttriage/` allowing unit tests to stub LLM responses without spawning subprocesses.
- `fixture.GeneratorOptions`: Parameterized builder struct in `internal/conflicttriage/fixture/` to configure target repository root, branch names, and base commit in tests.

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | Applicable | Scanned as plain text for conflict markers and diff scopes without execution; binary null-byte check (`internal/resolve/candidate.go:80`) ignores true binaries | `TestScanConflictMarkers_DocumentationAndScriptPaths` in `internal/resolve/candidate_test.go` |
| Git repository selection | `git -C`, relative paths, absolute paths | Applicable | Git commands use explicit worktree cwd (`git -C` or `cmd.Dir`); mutate CLI verbs (`reconcile resolve`) reject linked worktrees via `IsLinkedWorktree` (`internal/worktree/worktree.go:278-292`) | `TestReconcileResolve_RejectsLinkedWorktree` in `cmd/lucind-ai/cli_test.go`; `TestEnforceAllowedPaths_ExplicitWorktreeCwd` in `internal/resolve/candidate_test.go` |
| Commit state | staged, `commit -a`, empty index | Applicable | 4-way diff union in `EnforceAllowedPaths` (`internal/resolve/candidate.go:100-145`) inspects committed, unstaged, staged, and untracked files against `base_sha` before staging | `TestEnforceAllowedPaths_StagedAndUntrackedDisjoint` in `internal/resolve/candidate_test.go` |
| Push state | tracking branch, first push, explicit refspec | N/A: no remote git push or network transport exists in local feature reconciliation or fixture generation; refs are promoted locally via `git update-ref` (`internal/integrate/integrate.go:151-173`) | N/A | None |
| PR commands | explicit `--head`, environment prefix, composed commands | N/A: no VCS/PR hosting automation (GitHub/GitLab PRs) or command composition exists in this local feature-branch loop | N/A | None |

## Rollback and Additivity

**Choice**: Revert commits introducing `internal/conflicttriage/` (including `fixture/`) and judge packets via `git revert`.
**Alternatives considered**: Ledger schema teardown/migration rollback; feature flag gating.
**Rationale**: The change is strictly additive. Triage JSON is stored in the pre-existing, nullable `output` TEXT column of `reconciliation_candidates` (`internal/ledger/schema.go:163`, `internal/reconcile/reconcile.go:105`). No ledger schema, table definitions, or database versions move. Overlap gate logic (`internal/run/attempt.go:687-880`), CAS promotion (`internal/integrate/integrate.go:151-173`), CLI verbs (`cmd/lucind-ai/cli.go:56`), and read-only GET routes remain unmodified. Reverting application commits completely removes the advisory triage agent and fixture generator with zero ledger un-migration.

## Out of Scope

- Modifying overlap classification thresholds (`DefaultThresholds` in `internal/overlap/overlap.go:93-98`): owned by `internal/overlap`.
- Web UI reconcile POST endpoints (read-only GET routes remain): owned by `internal/serve`.
- Relaxing fail-closed resolver policy (`internal/resolve/candidate.go:26,303-312`): owned by `internal/resolve`.
- N-way (>2) reconciliation merge-of-merges (`internal/run/attempt.go:873-891`): owned by `internal/run`.
- Public CLI commands (`fixture generate`, `reconcile triage` in `cmd/lucind-ai/cli.go:56`): deferred to subsequent CLI features.
- Automated resolution grading and human timing as A/B win criteria: rejected product criteria.

## Open Questions

- [ ] What is the exact non-decreasing risk formula and threshold curve for mixed business and mechanical hunks? (Preserved open question from proposal; requires fixture calibration data).
- [ ] Which executor and model runs production triage? (Judges are pinned to `opencode`/`openai/gpt-5.6-sol` and `claude`/`claude-opus-5`; production execution remains deferred).

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:56` | Subcommand usage for reconcile approve and resolve CLI commands |
| `cmd/lucind-ai/cli.go:60` | Test injection seam for CLI dispatch dependencies |
| `cmd/lucind-ai/cli.go:65-70` | Registered executor map supporting agy, claude, cursor-agent, and opencode |
| `cmd/lucind-ai/cli_test.go:2944` | Integration test verifying reconcile approve CLI execution |
| `cmd/lucind-ai/cli_test.go:3126` | Integration test verifying reconcile resolve CLI execution |
| `internal/executor/claude.go:30-52` | Claude executor definition with default model claude-opus-5 |
| `internal/executor/claude.go:106-122` | Claude stream-json execution with raw stream terminal fallback |
| `internal/executor/claude_stream.go:10-16` | Sentinel degradation message and fallback retainment for claude stream decoder |
| `internal/executor/opencode.go:50-65` | Opencode executor definition with default model openai/gpt-5.6-sol |
| `internal/feature/feature.go:101-113` | Parent ref validation rejecting empty, main, and lucind prefixes |
| `internal/feature/feature.go:118-133` | Feature creation requiring non-empty base SHA and valid parent ref |
| `internal/integrate/integrate.go:151-173` | Atomic compare-and-swap ref promotion via git update-ref |
| `internal/ledger/schema.go:163` | Pre-existing output column definition in reconciliation_candidates table |
| `internal/overlap/overlap.go:93-98` | Default classification thresholds for hotspot weight and nearby hunks |
| `internal/overlap/overlap.go:623-659` | Overlap signal classification rules triggering ClassRequired |
| `internal/packet/disjoint.go:13-21` | Component-boundary prefix matching rule for allowed paths |
| `internal/packet/disjoint.go:29-48` | Pairwise disjoint path validation for batch packets |
| `internal/reconcile/reconcile.go:105` | Candidate Output field carrying advisory triage JSON payload |
| `internal/reconcile/reconcile.go:107` | Candidate SHA field populated upon human resolution registration |
| `internal/reconcile/reconcile.go:128-145` | Functional service options for injecting clock, ID source, and evaluator |
| `internal/reconcile/reconcile.go:157-168` | Reconcile Service constructor with default evaluator |
| `internal/reconcile/reconcile.go:213-336` | CreateRequest persisting overlap evidence and awaiting reconciliation request |
| `internal/reconcile/reconcile.go:266-276` | Durable insertion of ClassRequired overlap evidence into ledger |
| `internal/reconcile/reconcile.go:406-535` | Approve creating running candidate record with allowed paths |
| `internal/reconcile/reconcile_test.go:52-120` | Unit test verifying exact request fields and evidence snapshot persistence |
| `internal/resolve/candidate.go:26` | Fail-closed ErrSemanticAmbiguity sentinel error definition |
| `internal/resolve/candidate.go:48-95` | Conflict marker scanner checking non-ignored regular files |
| `internal/resolve/candidate.go:80` | Null-byte check skipping binary files during marker scanning |
| `internal/resolve/candidate.go:100-145` | 4-way diff union enforcing changes stay within allowed paths |
| `internal/resolve/candidate.go:303-312` | Fail-closed resolver prompt forbidding business decisions |
| `internal/resolve/candidate_test.go:16-49` | Unit test verifying conflict marker scanning on clean and dirty files |
| `internal/resolve/candidate_test.go:51-93` | Unit test verifying 4-way diff union allowed paths enforcement |
| `internal/resolve/resolve.go:20` | Functional Invoker type definition for merge resolution |
| `internal/run/attempt.go:687-880` | Overlap gate evaluation and ClassRequired attempt blocking |
| `internal/run/attempt.go:743-747` | ErrNoMergeBase bypass allowing attempt to continue without blocking |
| `internal/run/attempt.go:777-855` | ClassRequired request creation and attempt blocking path |
| `internal/run/attempt.go:821-828` | Target tip matching check unblocking approved candidate on retry |
| `internal/run/attempt.go:870` | Adoption of resolved candidate SHA on retry unblock |
| `internal/run/attempt.go:873-891` | Simultaneous multi-conflict resolution block |
| `internal/run/gate_test.go:122-140` | Test verifying required overlap gate blocks attempt promotion |
| `internal/run/integrate_feature.go:17` | ErrMixedFeatureTargets sentinel error definition |
| `internal/run/integrate_feature.go:41` | Rejection of mixed legacy and feature targets in one batch |
| `internal/run/integrate_feature.go:73` | Validation of feature parent ref during batch admission |
| `internal/run/run.go:155-221` | Deps struct defining test injection seams for run package |
| `internal/run/run.go:211` | EvaluateOverlap seam field on run Deps struct |
| `internal/worktree/worktree.go:278-292` | IsLinkedWorktree verifying gitdir pointer in worktree root |
