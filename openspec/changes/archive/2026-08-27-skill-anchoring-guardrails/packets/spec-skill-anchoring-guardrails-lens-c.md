---
id: spec-skill-anchoring-guardrails-lens-c
executor: agy
routed_by: live-spec conflict and migration lens of the three-lens spec fan-out for Change skill-anchoring-guardrails
allowed_paths: ["openspec/changes/skill-anchoring-guardrails/spec-lens-c.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: f5a531183361804ed95c797e16a70dbbcca27763
expected_parent_sha: f5a531183361804ed95c797e16a70dbbcca27763
---

# Packet spec-skill-anchoring-guardrails-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/spec-skill-anchoring-guardrails-lens-c  ·  **Branch:** lucind/spec-skill-anchoring-guardrails-lens-c

## Goal

Produce `openspec/changes/skill-anchoring-guardrails/spec-lens-c.md`: what this change collides with in the live specs under `openspec/specs/`, the verbatim full block of every requirement it genuinely modifies, and migration guidance for anything removed or renamed.

This is one of three parallel spec lenses. It is feedstock for a synthesis lane, not the final delta spec. Do not write anything under `openspec/changes/skill-anchoring-guardrails/specs/`.

## Why this is safe to dispatch now

`proposal.md` is accepted and frozen. Lens A and lens B run in parallel and write to different files.

## Why this lens exists

The proposal's own Capabilities section names `lane-worktree-lifecycle` and `worktree-cleanup-cli` as **Modified Capabilities** — but neither name appears in the current `openspec/specs/` directory listing (`acceptance-verifier`, `allowed-paths-enforcement`, `apply-dag-dispatch`, `completion-mode-enforcement`, `conflict-fixture`, `conflict-triage`, `defect-records`, `dependencies-defects`, `dispatched-packet-body`, `lane-approval-wait`, `lane-execution`, `lane-progress-telemetry`, `orphan-lane-reconciliation`, `parent-feature-integration`, `read-only-done-criterion`, `read-only-packet-schema`, `reconciliation-approval`, `sdd-apply`, `sdd-planning-fan-out`, `triage-evaluation-rubric`, `ultrafixer-dispatch`, `verify-dual-dispatch`, `verify-judgment-packet`, `verify-mechanical-check`). This lens exists specifically to resolve that: open every plausibly-related live spec in full and determine authoritatively whether worktree cleanup / CLI-lane-lifecycle behavior is already covered under a different capability name (`lane-execution` is the most likely candidate — check it first), or whether these are genuinely new capabilities mislabeled "Modified" in the proposal.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-spec/SKILL.md` — the real `gentle-ai` spec skill, and the **MODIFIED Requirements Workflow** section in particular.
2. `openspec/specs/lane-execution/spec.md` **in full** — the most likely home for existing worktree-lifecycle requirements. Also check `openspec/specs/apply-dag-dispatch/spec.md` and `openspec/specs/dispatched-packet-body/spec.md` if `lane-execution` does not cover it.
3. `openspec/changes/skill-anchoring-guardrails/proposal.md`, for what the change intends to do to each capability.
4. Consumers of any requirement being removed or renamed: tests, docs, CLI help text, other specs referencing it by name.

Never claim a live requirement says something without opening it.

## Output format

Write exactly this skeleton to `openspec/changes/skill-anchoring-guardrails/spec-lens-c.md`:

```markdown
# Spec Lens C — Live-Spec Conflicts & Migration: Skill Anchoring & Worktree Cleanup Guardrails

## Assumed requirements

<2–4 sentences naming the requirement set you are checking: the six requirements from proposal.md's Delta Specifications section, and specifically whether worktree/CLI-cleanup behavior already exists in any live spec under a different capability name.>

## Live Spec Inventory

| Capability | Live spec (file:line) | Requirements | Scenarios | Touched by this change |
|---|---|---|---|---|

<Row for every live spec you opened, even ones that turn out unrelated — that is evidence the classification is correct, not an omission.>

## Conflicts

<Every place this change contradicts a live requirement rather than extending it. "None" if there are none — including "None: no live spec covers this behavior, all six requirements are genuinely ADDED against new capability files" if that is your finding.>

## MODIFIED Full Blocks

<Only if a genuine live spec exists for one of the two "Modified Capabilities" the proposal named. If your Live Spec Inventory finds neither `lane-worktree-lifecycle` nor `worktree-cleanup-cli` (nor an equivalent) actually exists, state that here explicitly and leave this section empty — do not fabricate a block to satisfy the skeleton.>

## Removals and Renames

| Requirement | Removed or renamed | Reason | Consumers (file:line) | Migration |
|---|---|---|---|---|

<"None" — the proposal names no removals or renames.>

## Open Questions

- [ ] <unresolved question — flag explicitly if the proposal's "Modified Capabilities" classification needs correcting to "New", or "None">
```

## Size budget

`spec-lens-c.md` MUST be under 1000 words **excluding any verbatim blocks under `## MODIFIED Full Blocks`**.

## Out of scope

- **Lens A owns**: the capability map, the new requirement statements, and their classification.
- **Lens B owns**: every new `#### Scenario:` block and the coverage argument.

Do NOT create or write any file under `openspec/changes/skill-anchoring-guardrails/specs/`. That tree belongs to the synthesizer.

## Allowed paths

`openspec/changes/skill-anchoring-guardrails/spec-lens-c.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-spec/` — the real `gentle-ai` spec skill and its `references/`. The skill is authority on *what* a delta spec must contain; this packet is authority on *how this phase is executed here* — superseded on purpose where they conflict; note conflicts in `## Open Questions`.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

| citation | claim |
|---|---|
| `path/to/example_file.ext:12-34` | <YOUR OWN real citation and claim from THIS draft> |

## Mechanical self-check (REQUIRED)

**Before you commit:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/spec-lens-c.md --budget 1000 \
  --exclude-section "MODIFIED Full Blocks" --exclude-section "Citation Manifest" \
  --require-section "Assumed requirements" --require-section "Live Spec Inventory" \
  --require-section "Conflicts" --require-section "Removals and Renames" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/spec-lens-c.md
```

Paste both PASS/FAIL reports into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every capability plausibly related to this change was opened in full**, and its inventory row's requirement/scenario counts came from the file rather than an estimate.
- [ ] **If a `MODIFIED Full Blocks` entry exists, it is the complete live block**, scenario for scenario, with nothing summarized or elided; if none exists, the section explicitly states why (no live spec found).
- [ ] **`spec-lens-c.md` exists, is under 1000 words excluding verbatim blocks and the Citation Manifest, and carries `## Assumed requirements`, `## Live Spec Inventory`, and `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Strip any injected `Co-authored-by:` trailer.

## Hard stops

- Copying a MODIFIED block whole would exceed what you can write, so the copy would have to be partial. Report which requirement forces it; never write a partial block.
- Satisfying one instruction in this packet would require violating another.

## Context

Change: **skill-anchoring-guardrails**. Accepted proposal's Capabilities section names New = `worktree-dirty-guardrail`, `failure-guidance-banners`, `tdd-wip-rescue-protocol`; claims Modified = `lane-worktree-lifecycle`, `worktree-cleanup-cli` — but current `openspec/specs/` has no file by either name. `openspec/specs/lane-execution/spec.md` is the most likely place worktree-lifecycle behavior would already be specified, if at all. Execution: Isolated Mode, `agy`-only executor except verify's second qualitative judge (kept on `cursor-agent`) — already decided, do not re-litigate.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
