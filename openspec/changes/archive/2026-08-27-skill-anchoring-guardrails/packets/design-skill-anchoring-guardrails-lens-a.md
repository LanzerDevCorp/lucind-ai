---
id: design-skill-anchoring-guardrails-lens-a
executor: agy
routed_by: decisions lens of the three-lens design fan-out for Change skill-anchoring-guardrails
allowed_paths: ["openspec/changes/skill-anchoring-guardrails/design-lens-a.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: f5a531183361804ed95c797e16a70dbbcca27763
expected_parent_sha: f5a531183361804ed95c797e16a70dbbcca27763
---

# Packet design-skill-anchoring-guardrails-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/design-skill-anchoring-guardrails-lens-a  ·  **Branch:** lucind/design-skill-anchoring-guardrails-lens-a

## Goal

Produce `openspec/changes/skill-anchoring-guardrails/design-lens-a.md`: the technical approach and every architecture decision for this change, each with its choice, the alternatives rejected, the rationale, and the terminal consumer that makes the decision observable.

This is one of three parallel design lenses. It is feedstock for a synthesis lane, not the final design document. Do not write a complete `design.md`.

## Why this is safe to dispatch now

`proposal.md` is accepted and frozen (42 citations verified, full convergence). Lens B and lens C run in parallel against the same frozen inputs and write to different files. This lens owns the architectural choice; the other two consume it.

## Preconditions

- `openspec/changes/skill-anchoring-guardrails/proposal.md` exists and is accepted.
- `openspec/changes/skill-anchoring-guardrails/design-lens-a.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-design/SKILL.md` — the real `gentle-ai` design skill.
2. `openspec/changes/skill-anchoring-guardrails/proposal.md` in full — it already contains a detailed Approach section, Affected Areas table, and Delta Specifications; your job is to turn that into formal architecture decisions with alternatives and rationale, not restate it.
3. `internal/worktree/worktree.go` (`Cleanup`, `Remove`, `PorcelainEmpty`, `pathFor`) in full.
4. `openspec/changes/archive/` for a prior change that added a similar fail-closed guardrail + sentinel error, if one exists.

Never guess at code. Every claim about existing structure carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/skill-anchoring-guardrails/design-lens-a.md`:

```markdown
# Design Lens A — Decisions: Skill Anchoring & Worktree Cleanup Guardrails

## Assumed architecture

<2–4 sentences: worktree.Cleanup/Remove gain a force bool parameter and export
ErrWorktreeDirty; cmd/lucind-ai/cli.go gains a --force/-f flag and four static
guidance-banner call sites; no new packages, no schema change.>

## Technical Approach

<Concise strategy mapping to the proposal's Approach section. Reference the six delta requirements by name.>

## Decision 1 — Signature shape: `force bool` parameter vs. wrapper function

**Choice**: <what we chose>
**Alternatives considered**: <e.g. separate `RemoveForced`/`CleanupForced` functions, a functional option>
**Rationale**: <why, grounded in the existing call sites>
**Terminal consumer**: <file:line>

## Decision 2 — Sentinel error placement and shape (`ErrWorktreeDirty`)

<same four fields>

## Decision 3 — Where dirty-check logic lives (inline vs. reusing `PorcelainEmpty`)

<same four fields>

## Decision 4 — Banner call-site strategy (helper function vs. inline prints)

<same four fields>

## Open Questions

- [ ] <unresolved technical question — include the proposal's own two Open Questions (stderr vs stdout routing for banners; `force: true` inline vs. `RemoveForced` helper for internal callers) if still unresolved, or "None">
```

## Size budget

`design-lens-a.md` MUST be under 1000 words.

## Out of scope

- **Lens B owns**: the file-changes table, data-flow diagrams, invariants, and every exact type/schema/CLI signature delta.
- **Lens C owns**: testing strategy, test seams, the threat matrix, and the rollback/additivity decision.

Do not write a rollback decision here even though it is shaped like a decision.

## Allowed paths

`openspec/changes/skill-anchoring-guardrails/design-lens-a.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-design/` — the real `gentle-ai` design skill and its `references/`. The skill is authority on *what* a design document must contain; this packet is authority on *how this phase is executed here* (including the 1800-word synthesis budget, not the skill's nominal 800) — superseded on purpose where they conflict; note conflicts in `## Open Questions`.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

| citation | claim |
|---|---|
| `path/to/example_file.ext:12-34` | <YOUR OWN real citation and claim from THIS draft> |

## Mechanical self-check (REQUIRED)

**Before you commit:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/design-lens-a.md --budget 1000 \
  --exclude-section "Citation Manifest" --require-section "Assumed architecture" \
  --require-section "Technical Approach" --require-section "Open Questions" \
  --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/design-lens-a.md
```

Paste both PASS/FAIL reports into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every decision names a terminal consumer with a `file:line` citation**, and that citation points at real code in this worktree.
- [ ] **`design-lens-a.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section including `## Assumed architecture`, plus `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Strip any injected `Co-authored-by:` trailer.

## Hard stops

- The proposal does not determine an architectural choice, and two reasonable shapes are equally supported.
- A decision cannot name any terminal consumer.
- Satisfying one instruction in this packet would require violating another.

## Context

Change: **skill-anchoring-guardrails**. Accepted proposal already specifies: export `worktree.ErrWorktreeDirty`; `worktree.Cleanup`/`Remove` gain `force bool`, check `PorcelainEmpty` when `force` is false; `lucind-ai worktree cleanup` gains `--force`/`-f`; internal callers (`DiscardCombined`, `RemoveLaneWorktree`, `Combine` conflict abort, `ResolveCandidate` teardown) pass `force: true`; four CLI banners added at `printReport`, `printIntegrateReport`, `renderAcceptanceReceipt`, `runSplit`; TDD WIP-rescue protocol documented in `troubleshooting.md` and `.agents/skills/lucind-apply/SKILL.md`. Purely additive, single `git revert` rollback. Execution: Isolated Mode, `agy`-only executor except verify's second qualitative judge (kept on `cursor-agent`) — already decided, do not re-litigate.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
