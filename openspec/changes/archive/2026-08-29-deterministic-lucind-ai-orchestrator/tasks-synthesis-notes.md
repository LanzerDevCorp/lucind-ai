# Tasks Synthesis Notes: Deterministic lucind-ai Orchestrator

## Unresolved Contradictions

None.

## Coverage Gaps

- **No waves merged for Integrate viability.** Lens B’s two-wave table was independently re-checked: exact-file lists for units 3 and 4 under `internal/run/` are PathInScope-disjoint (`disjoint.go:13-22`); Wave 1 combined (skills + packet/DAG + `run.go`/`batch.go` + attempts) would be green if each unit keeps RED+GREEN together. The dispatch shape is still one sequential packet because B recommended no sidecar and `Integrate` reverts a RED-only wave (`integrate.go:50-59`). That is a shape choice, not a merge of a failing wave.
- **Lens C forecast vs `sdd-tasks` field contract.** C marked 400-line risk Low, Chained PRs No, `size-exception`, Decision needed No, citing this session’s 5000-line budget. The skill names the field `400-line budget risk` and, when the estimate exceeds 400 lines, requires High, Chained PRs Yes, and `ask-on-risk` → Decision needed Yes with chain strategy pending. `tasks.md` follows the skill; C’s scoring is not used. Packet 1800-word budget (not skill 530) is used; Engram persist and the skill return block are superseded by the two files plus `.lucind/result.json`.
- **OpenCode line estimate.** C’s 3200–4500 assumed creating ~40 OpenCode files (2500–3500 lines). `diff -rq plugin/claude-code/skills/lucind-ai plugin/opencode/skills/lucind-ai` is empty on this 61aa0cc-merged base. Net-new is skill edits times two trees plus Go gaps. Forecast in `tasks.md` is 700–1400.
- **No lens named a tree-comparator test.** Design Testing Strategy calls for one. Unit 1’s focused command `TestSkillAssetContract` (`packet_test.go:777`) reads only the Claude tree. Not invented; apply should add a comparator when doing 1.2.
- **Phases 2 and 4 are mostly pin-existing.** Target-free parse, omitted `allowed_paths` skip, Split stdout, ExecuteBatch join, attempt replay, CAS fail-closed, `FeatureTarget`, `IntegrateFeature` revert, linked-worktree refusal, and `integrated_ids`/`reverted_ids` already exist. Real gaps: skill contract text, OpenCode re-sync after 1.1, `HardStop.Fired` demotion, CLI skill-parity/schema preflight, and the named threat-matrix tests that are still missing.

## Dropped Citations

Every `file:line` below was opened in this worktree. Claims that used them are out of `tasks.md`.

- **Lens A `internal/run/run.go:549-573` as `runOneLane` / `decideStatus`.** Range is approval `RequestApproval` / `WaitDecision`. `decideStatus` is `:868-893`. Task 3.1 kept with the latter range.
- **Lens A `cmd/lucind-ai/cli.go:784-815` as `runFeatureCreate`.** Range is `gitShowToplevel` and the start of `resolvePrimaryRoot`. `runFeatureCreate` is `:958-1025` and already refuses linked worktrees (`:1003-1007`). Task 5.1 kept with the real function.
- **Lens B `internal/run/attempt.go:596-682` as fail-closed parent SHA mismatch.** That range is `RecoverAttempt` plus the matching-ref resume path. Scenario 3 mismatch is `:695-701`. Task 4.1 kept with `:592-701`.
- **Lens B `internal/run/run.go:486-503` as work to implement required-skills enforcement.** The lines call `enforceRequiredSkills` (`:496-498`). That belongs to `feature/skill-provisioning-and-phase-specialist` (`design.md:119`). Citation describes live code; no task modifies it.
- **Lens C `internal/packet/packet.go:78` as `AllowedPaths`.** Line 78 is `ReadOnlyPaths`. `AllowedPaths` is `:74-77`.
- **Lens C `cmd/lucind-ai/cli.go:540-554` as feature recover / CAS reporting.** Range is the `runCheck` comment that it does *not* use `resolvePrimaryRoot`. `printIntegrateReport` is `:752-782`.
- **Lens C `cmd/lucind-ai/cli.go:267-280` as linked-worktree / skill-parity preflight.** Range is agent-then-model KnownModels checks. Linked-worktree refusal is `:353-361`.
- **Lens C `internal/run/run_test.go:962-985` as `decideStatus` terminal-status pattern.** Range is `TestExecuteTerminalStatusWriteFailureStillReturnsOriginalCause` (ledger write after status). No `TestDecideStatus_FiredHardStopDemotes` exists; that name is the 3.0-RED to write.
- **Lens C proving names `TestParse_OmittedAllowedPaths` / `TestParse_TargetFree` / `TestEnforceCompletionMode_*`.** Those funcs do not exist. Omitted `allowed_paths` is `TestParseAllowedPathsFrontmatter` (`packet_test.go:490-498`); completion mode is `TestExecuteWriteDoneWithoutUniqueCommitsFails`, `TestExecuteWriteDoneWithDirtyWorktreeFails`, `TestExecuteReadOnlyDoneWithUniqueCommitsFails` (`run_test.go:1597-1747`). `tasks.md` uses the live names.
- **Lens C `internal/overlap/overlap_test.go:37-60` / `internal/dag/overlap.go:54` as apply work.** `ValidateGlobalOverlap` exists; design File Changes does not modify `overlap.go`. Not in lens A’s checklist.
- **Design.md:11 / lens A 1.2 “Create OpenCode tree”.** The tree is present (40 files) and currently byte-identical. Task 1.2 is re-sync after 1.1, not create. `design.md:16-19` also cites `cli.go:267-280` and `:791-800` for `runDispatch` / `runFeatureCreate` barriers; those line numbers are stale after 61aa0cc (model check and `gitShowToplevel`). Policy (preflight at existing barriers) kept; line numbers in `tasks.md` are the live ones.

## Decomposition Divergence

Lens A’s five-phase split is the one in `tasks.md`. A was not refuted: every named production file exists; OpenCode existing changes 1.2’s verb from create to re-sync, not the phase.

**Independent convergence.** B and C also opened with five units covering (1) skill parity, (2) packet/DAG, (3) runtime evidence, (4) attempts/CAS, (5) CLI. That is corroboration of the area split.

**Lens B — `batch.go` in unit 4 with attempts.** A places `batch.go` in phase 3 with `run.go` (3.2 depends on 3.1). Cost: B’s unit-4 `allowed_paths` listed `batch.go` next to `attempt.go`. Remapped to unit 3. Exact-file lists remain disjoint under PathInScope; a directory `internal/run/` would not.

**Lens B — unit 3 implements four-way diff and completion mode.** Those already run after `decideStatus` (`run.go:492-502`, `:895-923`, `:969-980`) with tests. A’s production delta in `run.go` is hard-stop demotion. Cost: B’s “implement” wording; `tasks.md` pins existing tests (3.0b) and adds only the missing demotion RED/GREEN.

**Lens B — two-wave parallel 1–4 then CLI, plus no sidecar.** Compatible with A’s dependencies if sequential inside one packet. Sidecar declined as B recommended.

**Lens C — CLI preflight inside unit 1 with skills.** A keeps CLI as phase 5 (depends on 1.2). Cost: C’s unit-1 proving command spanning `plugin/` and `cmd/lucind-ai`. Preflight tests stay in 5.0-RED.

**Lens C — wave split + `batch.go` as one unit; attempts + CLI reporting as one unit.** A splits Split (2.2) from batch (3.2) and keeps CLI reporting as existing `printIntegrateReport`, not phase-4 work. Cost: C’s combined proving commands; live `TestSplit_*` / `TestWaves_*` pin 2.2, live attempt/FeatureTarget tests pin 4.x.
