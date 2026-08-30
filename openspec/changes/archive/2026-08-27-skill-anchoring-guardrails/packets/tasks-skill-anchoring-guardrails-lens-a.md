---
id: tasks-skill-anchoring-guardrails-lens-a
executor: agy
routed_by: decomposition and ordering lens of the three-lens tasks fan-out for Change skill-anchoring-guardrails
allowed_paths: ["openspec/changes/skill-anchoring-guardrails/tasks-lens-a.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: 2ffb8b2b4e6b9a0b63d2d661efb372b36755fa6a
expected_parent_sha: 2ffb8b2b4e6b9a0b63d2d661efb372b36755fa6a
---

# Packet tasks-skill-anchoring-guardrails-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/tasks-skill-anchoring-guardrails-lens-a  ·  **Branch:** lucind/tasks-skill-anchoring-guardrails-lens-a

## Goal

Produce `openspec/changes/skill-anchoring-guardrails/tasks-lens-a.md`: the phased implementation checklist for this change and the dependency order behind it.

This is one of three parallel tasks lenses. It is feedstock for a synthesis lane, not the final checklist. Do not write a complete `tasks.md`.

## Why this is safe to dispatch now

`design.md` and the delta spec tree (5 capabilities, all ADDED) are accepted and frozen, 58+81 citations independently verified with zero unresolved contradictions. Lens B and lens C run in parallel and write to different files.

## Preconditions

- `openspec/changes/skill-anchoring-guardrails/specs/` and `openspec/changes/skill-anchoring-guardrails/design.md` both exist.
- `openspec/changes/skill-anchoring-guardrails/tasks-lens-a.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-tasks/SKILL.md` — the real `gentle-ai` tasks skill.
2. `openspec/changes/skill-anchoring-guardrails/design.md` in full, its file-changes table in particular.
3. `openspec/changes/skill-anchoring-guardrails/specs/*/spec.md` — every requirement across the 5 capability files.
4. `internal/worktree/worktree.go`, `cmd/lucind-ai/cli.go`, `internal/integrate/integrate.go`, `internal/integrate/candidate.go`, `internal/run/integrate.go`, `.agents/skills/lucind-apply/SKILL.md`, `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md` — the actual current state of every file the design says changes.

Never guess at a file. Every task names a concrete path, and every path claim carries a `file:line` citation or is explicitly marked "new file".

## Output format

Write exactly this skeleton to `openspec/changes/skill-anchoring-guardrails/tasks-lens-a.md`:

```markdown
# Tasks Lens A — Decomposition & Ordering: Skill Anchoring & Worktree Cleanup Guardrails

## Assumed decomposition

<2–4 sentences: e.g. 3 phases — (1) worktree.go signature+sentinel+tests, (2) CLI flag+internal force:true callers+tests, (3) CLI guidance banners+troubleshooting.md/SKILL.md docs+tests. Critical path: phase 1 before phase 2 (CLI depends on the new signature); phase 3 is independent of 1/2 except sharing cli.go.>

## Phase 1: <Phase Name>

- [ ] 1.1 <Concrete action>
- [ ] 1.2 <Concrete action>

## Phase 2: <Phase Name>

- [ ] 2.1 <Concrete action>

## Phase 3: <Phase Name>

- [ ] 3.1 <Concrete action>

## Dependency Order

| Task | Depends on | Why |
|---|---|---|

## Requirement Traceability

| Requirement | Tasks |
|---|---|

## Open Questions

- [ ] <unresolved question, or "None">
```

Every task MUST be specific, actionable, verifiable, and small (one file or one logical unit).

## Size budget

`tasks-lens-a.md` MUST be under 1000 words.

## Out of scope

- **Lens B owns**: the Suggested Work Units table, wave partition, per-unit `allowed_paths`, executor assignment, sidecar recommendation.
- **Lens C owns**: the Review Workload Forecast table, per-task acceptance evidence, and the RED-test task for every applicable threat-matrix row.

Do not estimate changed lines, do not name PR boundaries, do not assign an executor to a task.

## Allowed paths

`openspec/changes/skill-anchoring-guardrails/tasks-lens-a.md` only. Create no other file.

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
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/tasks-lens-a.md --budget 1000 \
  --exclude-section "Citation Manifest" --require-section "Assumed decomposition" \
  --require-section "Dependency Order" --require-section "Requirement Traceability" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/tasks-lens-a.md
```

Paste both PASS/FAIL reports into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every task names a concrete path**, and every "modify" task's path resolves in this worktree.
- [ ] **Every requirement in `specs/` appears in the traceability table with at least one task**, and every gap in either direction is reported rather than hidden.
- [ ] **Every dependency row states why it is a real ordering constraint**, not a preference.
- [ ] **`tasks-lens-a.md` exists, is under 1000 words excluding the Citation Manifest, and carries `## Assumed decomposition`, `## Dependency Order`, `## Requirement Traceability`, and `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Strip any injected `Co-authored-by:` trailer.

## Hard stops

- A requirement in `specs/` cannot be decomposed into tasks because the design does not say how it is built.
- The design's file-changes table and the specs disagree about what this change does.
- Two tasks are mutually circular.
- Satisfying one instruction in this packet would require violating another.

## Context

Change: **skill-anchoring-guardrails**. Delivery strategy: **single-pr**, review budget **2000 changed lines** (human-confirmed session preflight) — this change is expected to fit comfortably within that budget (small, additive, no schema changes), but do not assume; base the decomposition on real file scope. Design's accepted architecture: `force bool` on `worktree.Cleanup`/`Remove` + `ErrWorktreeDirty` sentinel + `--force`/`-f` CLI flag + 4 internal callers passing `force: true` + 4 CLI guidance banners + TDD WIP-rescue protocol docs. Execution: Isolated Mode, `agy`-only executor except verify's second qualitative judge (kept on `cursor-agent`) — already decided, do not re-litigate.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
