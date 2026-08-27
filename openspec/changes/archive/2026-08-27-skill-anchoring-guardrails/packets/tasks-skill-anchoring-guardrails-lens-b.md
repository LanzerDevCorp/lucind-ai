---
id: tasks-skill-anchoring-guardrails-lens-b
executor: agy
routed_by: partition and dispatch-shape lens of the three-lens tasks fan-out for Change skill-anchoring-guardrails
allowed_paths: ["openspec/changes/skill-anchoring-guardrails/tasks-lens-b.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: 2ffb8b2b4e6b9a0b63d2d661efb372b36755fa6a
expected_parent_sha: 2ffb8b2b4e6b9a0b63d2d661efb372b36755fa6a
---

# Packet tasks-skill-anchoring-guardrails-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/tasks-skill-anchoring-guardrails-lens-b  ·  **Branch:** lucind/tasks-skill-anchoring-guardrails-lens-b

## Goal

Produce `openspec/changes/skill-anchoring-guardrails/tasks-lens-b.md`: how this change's work is partitioned into dispatchable units — the work-unit table, the wave plan, each unit's `allowed_paths`, its executor, and whether an `apply-dag.yaml` sidecar is warranted.

This is one of three parallel tasks lenses. It is feedstock for a synthesis lane, not the final checklist. Do not write a complete `tasks.md`.

## Why this is safe to dispatch now

`design.md` and specs are accepted and frozen. Lens A and lens C run in parallel and write to different files.

Lens A owns the task decomposition and is running concurrently, so you do not have it. Derive the work from the design's file-changes table, declare it in `## Assumed decomposition`, and partition that.

## Why this lens exists

- **The `allowed_paths` prefix trap.** Naming a directory covers everything beneath it (`internal/packet/disjoint.go`). This change touches `internal/worktree/worktree.go`, `cmd/lucind-ai/cli.go` (multiple functions), and `internal/integrate/*.go` — several units may need to touch the SAME file (`cmd/lucind-ai/cli.go`) for different concerns (flag parsing vs. banners vs. internal callers), which is a same-file collision, not a same-directory one; check carefully whether that forces those units into one wave or one unit.
- **The `Integrate` gate.** Every wave must pass `lucind-checks.sh` on its own. Strict-TDD RED and GREEN for one unit belong in one lane, never split across waves.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-tasks/SKILL.md` — the real `gentle-ai` tasks skill, its **Suggested Work Units** table in particular.
2. `openspec/changes/skill-anchoring-guardrails/design.md` — the file-changes table is the partition's input.
3. `internal/packet/disjoint.go` — the real matching rule.
4. `internal/run/integrate.go` — the gate and bisection behavior.
5. `internal/dag/` — the sidecar's node shape.
6. `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md` (if present) — a precedent that **declined** a DAG split for units too small to pay for sidecar orchestration. This change is likely in the same category: check `cmd/lucind-ai/cli.go` touch-count across all proposed units before recommending a sidecar.

Never claim two units are disjoint without checking their paths against the prefix rule by hand.

## Output format

Write exactly this skeleton to `openspec/changes/skill-anchoring-guardrails/tasks-lens-b.md`:

```markdown
# Tasks Lens B — Partition & Dispatch Shape: Skill Anchoring & Worktree Cleanup Guardrails

## Assumed decomposition

<Same shape lens A is expected to assume — state your own finding independently.>

## Suggested Work Units

| Unit | Goal | allowed_paths | Executor | Rollback boundary |
|---|---|---|---|---|

## Wave Plan

| Wave | Units | Runs in parallel | Green on its own |
|---|---|---|---|

## Disjointness Check

<For every pair of units in the same wave, by hand.>

## Sidecar Recommendation

**Recommendation**: <sidecar warranted / single packet, no sidecar>
**Rationale**: <why>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`tasks-lens-b.md` MUST be under 1000 words.

## Out of scope

- **Lens A owns**: the phased checklist, dependency-order table, requirement traceability.
- **Lens C owns**: the Review Workload Forecast table, per-task acceptance evidence, and RED-test tasks.

Do not write the task checklist. Do not estimate changed lines or name a PR split.

## Allowed paths

`openspec/changes/skill-anchoring-guardrails/tasks-lens-b.md` only. Create no other file.

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
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/tasks-lens-b.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Assumed decomposition" --require-section "Suggested Work Units" \
  --require-section "Wave Plan" --require-section "Disjointness Check" \
  --require-section "Sidecar Recommendation" --require-section "Open Questions" \
  --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/tasks-lens-b.md
```

Paste both PASS/FAIL reports into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every pair of units sharing a wave was checked by hand against the component-boundary prefix rule**, and the verdict is recorded.
- [ ] **Every wave's "green on its own" column is a yes with a reason**, or the wave was merged.
- [ ] **Every unit names concrete `allowed_paths` and an executor.**
- [ ] **`tasks-lens-b.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section plus `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Strip any injected `Co-authored-by:` trailer.

## Hard stops

- No partition exists in which every wave is green on its own. Say so; a single packet is a legitimate answer.
- Two units that must run in the same wave cannot have disjoint `allowed_paths` because they edit the same file.
- The design's file-changes table does not determine which unit owns a file.
- Satisfying one instruction in this packet would require violating another.

## Context

Change: **skill-anchoring-guardrails**. Design's file-changes table: `internal/worktree/worktree.go` (signature+sentinel), `cmd/lucind-ai/cli.go` (flag parsing, 4 internal force:true call sites, 4 guidance banners — ALL in one file), `internal/integrate/integrate.go`, `internal/integrate/candidate.go`, `internal/run/integrate.go` (force:true call sites), `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md`, `.agents/skills/lucind-apply/SKILL.md` (docs only, no Go code, no test dependency). Delivery strategy: single-pr, 2000-line budget. Execution: Isolated Mode, `agy`-only executor except verify's second qualitative judge (kept on `cursor-agent`) — already decided, do not re-litigate.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
