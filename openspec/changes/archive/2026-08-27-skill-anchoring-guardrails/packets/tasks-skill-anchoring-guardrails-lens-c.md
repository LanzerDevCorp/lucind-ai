---
id: tasks-skill-anchoring-guardrails-lens-c
executor: agy
routed_by: proof and review-burden lens of the three-lens tasks fan-out for Change skill-anchoring-guardrails
allowed_paths: ["openspec/changes/skill-anchoring-guardrails/tasks-lens-c.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: 2ffb8b2b4e6b9a0b63d2d661efb372b36755fa6a
expected_parent_sha: 2ffb8b2b4e6b9a0b63d2d661efb372b36755fa6a
---

# Packet tasks-skill-anchoring-guardrails-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/tasks-skill-anchoring-guardrails-lens-c  ·  **Branch:** lucind/tasks-skill-anchoring-guardrails-lens-c

## Goal

Produce `openspec/changes/skill-anchoring-guardrails/tasks-lens-c.md`: what proves each piece of this change works — the RED test task for every applicable threat-matrix row, acceptance evidence per task, and the Review Workload Forecast.

This is one of three parallel tasks lenses. It is feedstock for a synthesis lane, not the final checklist. Do not write a complete `tasks.md`.

## Why this is safe to dispatch now

`design.md` (with threat matrix and testing strategy) and specs are accepted and frozen. Lens A and lens B run in parallel and write to different files.

## Preconditions

- `openspec/changes/skill-anchoring-guardrails/design.md` exists and carries a threat matrix and testing strategy.
- `openspec/changes/skill-anchoring-guardrails/specs/` exists.
- `openspec/changes/skill-anchoring-guardrails/tasks-lens-c.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-tasks/SKILL.md` — the real `gentle-ai` tasks skill, its **Review Workload Forecast** table and threat-matrix rule.
2. `openspec/changes/skill-anchoring-guardrails/design.md` — the threat matrix and testing strategy verbatim. Every `Applicable` row becomes an explicit RED-test task; `N/A` rows stay omitted.
3. `openspec/changes/skill-anchoring-guardrails/specs/*/spec.md` — the scenarios, what the tests assert.
4. `internal/worktree/worktree_test.go`, `cmd/lucind-ai/cli_test.go` — real existing test files (`TestCleanupRemovesExistingWorktree`, `TestCleanupOnLaneWithNoWorktreeIsNoOp`, `TestRemove`, `TestWorktreeCleanupCLI`, `TestPrintReportOmitsDiagnosisBlockWhenNoneCaptured`, `TestPrintIntegrateReportIncludesIntegratedAndRevertedIDs`, `TestAcceptRequiresExactReceiptAndRendersMechanicalEvidenceOnly`), so proposed proving commands are ones this repository can actually run.

Never propose a test command you did not derive from a real test file.

## Output format

Write exactly this skeleton to `openspec/changes/skill-anchoring-guardrails/tasks-lens-c.md`:

```markdown
# Tasks Lens C — Proof & Review Burden: Skill Anchoring & Worktree Cleanup Guardrails

## Assumed decomposition

<Same shape lens A is expected to assume — state your own finding independently.>

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | <basis: count comparable file diffs, e.g. worktree.go signature change ~X lines, cli.go flag+4 banners+4 force:true call sites ~Y lines, troubleshooting.md/SKILL.md prose ~Z lines, plus test updates> |
| 400-line budget risk | Low / Medium / High |
| Chained PRs recommended | Yes / No |
| Suggested split | <single PR, or PR 1 → PR 2> |
| Delivery strategy | single-pr (human-confirmed) |
| Chain strategy | pending (only if chaining needed) |

## RED Tests from the Threat Matrix

| Threat row | Applicable | RED test | Asserts | Production task it precedes |
|---|---|---|---|---|

<Copy the design's own Applicable/N/A verdicts verbatim; do not re-decide them.>

## Acceptance Evidence

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|

## Verification Gaps

<"None" if there are none.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`tasks-lens-c.md` MUST be under 1000 words.

## Out of scope

- **Lens A owns**: the phased checklist, dependency-order table, requirement traceability.
- **Lens B owns**: the Suggested Work Units table, wave partition, `allowed_paths`, executor assignment, sidecar recommendation.

Do not write the task checklist and do not partition units into waves.

## Allowed paths

`openspec/changes/skill-anchoring-guardrails/tasks-lens-c.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-tasks/` — the real `gentle-ai` tasks skill and its `references/`. The skill is authority on *what* a task file must contain; this packet is authority on *how this phase is executed here* — superseded on purpose where they conflict; note conflicts in `## Open Questions`.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

| citation | claim |
|---|---|
| `path/to/example_file.ext:12-34` | <YOUR OWN real citation and claim from THIS draft> |

## Mechanical self-check (REQUIRED)

**Before you commit:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/tasks-lens-c.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Assumed decomposition" --require-section "Review Workload Forecast" \
  --require-section "RED Tests from the Threat Matrix" --require-section "Acceptance Evidence" \
  --require-section "Verification Gaps" --require-section "Open Questions" \
  --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/tasks-lens-c.md
```

Paste both PASS/FAIL reports into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every threat-matrix row in the design appears with the design's own verdict**, and no row was invented.
- [ ] **Every applicable row names a RED test and the assertion that fires.**
- [ ] **The changed-line estimate states the basis it was derived from.**
- [ ] **Every proving command is derived from a real test file in this worktree**, cited with `file:line`.
- [ ] **`tasks-lens-c.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section plus `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Strip any injected `Co-authored-by:` trailer.

## Hard stops

- The design has no threat matrix, so there is nothing to derive RED tests from.
- A behavior the specs require cannot be proven through any test this repository can run, and no new seam is identified.
- The changed-line estimate cannot be grounded in anything.
- Satisfying one instruction in this packet would require violating another.

## Context

Change: **skill-anchoring-guardrails**. Delivery strategy: **single-pr** (human-confirmed session preflight), review budget **2000 changed lines** — ground your estimate honestly; do not assume it fits just because the budget is generous. Design's testing strategy already names: `TestCleanupRemovesExistingWorktree`, `TestCleanupOnLaneWithNoWorktreeIsNoOp`, `TestRemove` (`internal/worktree/worktree_test.go`), `TestWorktreeCleanupCLI` (`cmd/lucind-ai/cli_test.go:2974-3010`), `TestAcceptRequiresExactReceiptAndRendersMechanicalEvidenceOnly` (`:4503-4545`), `TestPrintReportOmitsDiagnosisBlockWhenNoneCaptured` (`:685-724`), `TestPrintIntegrateReportIncludesIntegratedAndRevertedIDs` (`:729-777`), `TestSplit_TwoWaveDAGSuccess` (`internal/dag/split_test.go:13-111`). Execution: Isolated Mode, `agy`-only executor except verify's second qualitative judge (kept on `cursor-agent`) — already decided, do not re-litigate.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
