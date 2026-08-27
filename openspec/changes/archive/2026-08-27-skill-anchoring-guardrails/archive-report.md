# Archive Report: Skill Anchoring & Worktree Cleanup Guardrails

## Verdict

**PASSED** — terminal verification passed with zero CRITICAL or blocking defects.
- **Stage 1 (Mechanical check)**: `lucind-ai check --out openspec/changes/skill-anchoring-guardrails/verify-mechanical.log` passed clean (exit code 0, duration ~1m33s, all packages `ok`). One transient `internal/feature` test failure (`TestConcurrentLeaseAcquisition`, `SQLITE_BUSY`) reproduced 3/3 clean in isolation, matching the known pre-existing concurrency flake in `troubleshooting.md`. Transcript committed at `verify-mechanical.log`.
- **Stage 2 (Dual qualitative judgment)**: Unanimous Pass (`done`/`done`) from `agy` (`verify-skill-anchoring-guardrails-agy`) and `cursor-agent` (`verify-skill-anchoring-guardrails-cursor-agent`). All 5 capabilities verified across all requirements and scenarios. Orchestrator independently cross-checked load-bearing citations in `internal/worktree/worktree.go`, `cmd/lucind-ai/cli.go`, and `internal/dag/split.go`.

## What Shipped

Five new capability specifications were created and merged into `openspec/specs/`:

- `worktree-dirty-guardrail` (Added capability): 1 requirement (`Worktree dirty guardrail check`), 3 scenarios (`Refuse cleanup on dirty worktree without force`, `Force cleanup removes dirty worktree`, `Clean worktree cleanup succeeds idempotently`). `worktree.Cleanup` and `worktree.Remove` fail closed, checking `PorcelainEmpty` before deleting any linked worktree directory and returning `worktree.ErrWorktreeDirty` unless `force: true` is explicitly passed.
- `lane-worktree-lifecycle` (Added capability): 1 requirement (`Lane worktree lifecycle force parameter and automated teardown`), 2 scenarios (`Internal automated callers pass force true for teardown`, `Nonexistent worktree path teardown is idempotent`). `worktree.Cleanup` and `worktree.Remove` accept trailing `force bool`; internal automated callers (`DiscardCombined`, `RemoveLaneWorktree`, merge conflict abort in `Combine`, `ResolveCandidate`, `integrateAttempt`) pass `force: true` to bypass dirty checks for scratch/temporary worktrees; cleanup on nonexistent lane paths is idempotent.
- `worktree-cleanup-cli` (Added capability): 1 requirement (`Worktree cleanup CLI force flag and diagnostic status reporting`), 3 scenarios (`Refuse CLI cleanup on dirty worktree without force`, `Force CLI cleanup removes dirty worktree`, `Clean or nonexistent worktree CLI cleanup succeeds`). `lucind-ai worktree cleanup` adds `--force`/`-f` flag support; dirty worktree cleanup without `--force` exits 1, outputs porcelain status diff diagnostic steps referencing `troubleshooting.md`, and preserves files on disk.
- `failure-guidance-banners` (Added capability): 4 requirements (`Blocked and timeout lane report guidance banner`, `Integration report reverted IDs recovery banner`, `Acceptance receipt qualitative review banner`, `DAG split multi-wave base SHA warning banner`), 8 scenarios. Emits visual diagnostic and recovery banners for non-done/timeout lane reports (`troubleshooting.md`), reverted integration batches (`recovery-reconciliation.md`), mechanical acceptance receipts (qualitative review checklist steps 2–10 in `acceptance-promotion.md`), and multi-wave DAG splits (stderr warning to advance checkout and refresh base/parent SHAs).
- `tdd-wip-rescue-protocol` (Added capability): 1 requirement (`Prescriptive TDD WIP-rescue protocol documentation`), 1 scenario (`Operator executes TDD WIP-rescue after lane timeout`). Documents prescriptive procedures in `troubleshooting.md` and `lucind-apply` (`SKILL.md`) for rescuing uncommitted test/implementation progress from timed-out or blocked lanes.

Total: 5 new capability specifications, 8 requirements, 17 scenarios synced to live specs.

## Dispatch Record

The SDD cycle for `skill-anchoring-guardrails` executed across the following phases:

| Phase | Lanes / Artifacts | Executor | Outcome |
|---|---|---|---|
| Explore | 3 lenses + 1 synthesis (`explore-skill-anchoring-guardrails-*`) | `agy` (`gemini-3.7-flash-high`) | Grounded `explore.md` |
| Propose | 3 lenses + 1 synthesis (`propose-skill-anchoring-guardrails-*`) | `agy` (`gemini-3.7-flash-high`) | Authored `proposal.md` |
| Design | 3 lenses + 1 synthesis (`design-skill-anchoring-guardrails-*`) | `agy` (`gemini-3.7-flash-high`) | Authored `design.md` |
| Spec | 3 lenses + 1 synthesis (`spec-skill-anchoring-guardrails-*`) | `agy` (`gemini-3.7-flash-high`) | Authored delta specs under `specs/` |
| Tasks | 3 lenses + 1 synthesis (`tasks-skill-anchoring-guardrails-*`) | `agy` (`gemini-3.7-flash-high`) | Authored `tasks.md` (16 tasks) |
| Apply | 2 attempts (`apply-skill-anchoring-guardrails`, `apply-skill-anchoring-guardrails-2`) | `agy` (`gemini-3.7-flash-high`) | Attempt 1 deviated (`allowed_paths` missing 2 test files); superseded by attempt 2 (done, all 16 tasks checked) |
| Verify | 2 judges (`verify-skill-anchoring-guardrails-agy`, `verify-skill-anchoring-guardrails-cursor-agent`) | `agy` + `cursor-agent` | `done`/`done`, zero CRITICAL issues, verdict PASSED |
| Archive | 1 mechanical archival lane (`archive-skill-anchoring-guardrails`) | `agy` (`gemini-3.7-flash-high`) | Preserved session dispatch record, synced delta specs, moved to archive |

Total lanes: 22 dispatched lanes (21 `agy`, 1 `cursor-agent`, per approved AGY-only executor exception with dual-verification).
Preserved session dispatch artifacts under `openspec/changes/skill-anchoring-guardrails/`:
- `packets/`: 25 packet files preserved from `.lucind/packets/`
- `envelopes/`: 21 result envelope files preserved from `.lucind/results/`

## Follow-ups

Four non-blocking gaps identified by `cursor-agent` during qualitative verification (see `verify.md`):
1. **Ignored-file-only dirty state**: No dedicated unit test that unforced cleanup succeeds when only `.lucind/result.json` (ignored) is present — behavior is compositional on `PorcelainEmpty`, not directly tested.
2. **Path distinction scope**: Threat-matrix "Git repository selection" only partially covered — no test distinguishing relative/absolute paths, though `worktree cleanup` never takes an operator-supplied path so this may be moot.
3. **DAG split stderr optional parameter**: `dag.Split`'s new `stderr ...io.Writer` parameter is fail-open by design — a caller that omits it gets no warning; the pre-existing e2e two-wave test still omits it and so cannot catch a missing banner (the CLI's `runSplit` always passes stderr, satisfying the CLI spec; the gap is only in that one pre-existing test's coverage).
4. **Accept error path receipt absence assertion**: The accept-error path doesn't assert the receipt/reminder is absent on failure (implementation early-returns before `renderAcceptanceReceipt`, so the spec scenario holds in code, just under-tested).

## Gaps and Contradictions

- **Capability Classification**: The proposal originally categorized `lane-worktree-lifecycle` and `worktree-cleanup-cli` as Modified Capabilities. During spec fan-out, an exhaustive audit confirmed that no live specs existed for either capability in `openspec/specs/`. The classification was independently corrected to ADDED during spec synthesis (see `spec-synthesis-notes.md`), and all 5 capabilities have been merged into `openspec/specs/` as new full specifications.
- **Cross-Feature Overlap Reconciliation**: Promotion was temporarily held twice by the cross-feature overlap gate against `native-stability-campaign` due to churn on `cmd/lucind-ai/cli.go`. Both occurrences were cleanly resolved via `lucind-ai reconcile approve`/`resolve` (and `reconcile renew` after candidate advance) and `lucind-ai integrate retry`, adhering to the recovery protocol with no AI re-dispatch.
- **Apply Allowed Paths Deviation & Re-dispatch**: The initial apply lane deviated due to omitting two pre-existing test files (`internal/integrate/integrate_test.go` and `internal/run/isolation_test.go`) from packet `allowed_paths` required for updating `worktree.Remove` call sites. A corrected packet (`apply-skill-anchoring-guardrails-2`) was re-dispatched with expanded `allowed_paths`, cherry-picking the completed commit without loss of work.
