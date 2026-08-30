---
id: spec-agentic-phase-specialist-lens-c
executor: agy
routed_by: live-spec conflict and migration lens of the three-lens spec fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/agentic-phase-specialist/spec-lens-c.md"]
---

# Packet spec-agentic-phase-specialist-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/spec-agentic-phase-specialist-lens-c  ·  **Branch:** lucind/spec-agentic-phase-specialist-lens-c

## Goal

Produce `openspec/changes/agentic-phase-specialist/spec-lens-c.md`: what this change collides with in the live specs under `openspec/specs/phase-specialist-dispatch/`, `openspec/specs/acceptance-verifier/`, and `openspec/specs/sdd-planning-fan-out/`, the verbatim full block of every requirement it modifies, and migration guidance for anything removed or renamed.

This is one of three parallel spec lenses. It is feedstock for a synthesis lane, not the final delta spec. Do not write anything under `openspec/changes/agentic-phase-specialist/specs/`.

## Why this is safe to dispatch now

The proposal for `agentic-phase-specialist` is accepted and frozen. Lens A and lens B run in parallel against the same frozen inputs and write to different files, so no lane races another.

Lens A owns the requirement set and is running concurrently, so you do not have it. Derive the requirements you are checking from the proposal itself, declare them in `## Assumed requirements`, and key every finding to one of them by name.

## Why this lens exists

Archive replaces a live requirement with whatever the MODIFIED block says. A partial MODIFIED block silently deletes every scenario it failed to copy. This lens is the lane that opens the live spec and copies the whole block forward.

## Required reading (this lens only)

1. The real `gentle-ai` spec skill (delivered under `## Required skills`), and the **MODIFIED Requirements Workflow** section in particular.
2. `openspec/specs/phase-specialist-dispatch/spec.md`, `openspec/specs/acceptance-verifier/spec.md`, `openspec/specs/sdd-planning-fan-out/spec.md` **in full** — not a search result, the whole file each.
3. `openspec/changes/agentic-phase-specialist/proposal.md`, for what the change intends to do to each of them (it names these three as Modified Capabilities).
4. Consumers of anything being removed or renamed: none are named in the accepted proposal (it only proposes one ADDED capability and three MODIFIED capabilities, no REMOVED/RENAMED) — verify this is actually true by reading the proposal's `## Delta Specifications` and `## Capabilities` sections; if you find a requirement that should be classified REMOVED or RENAMED instead, say so.

Never claim a live requirement says something without opening it.

## Output format

Write exactly this skeleton to `openspec/changes/agentic-phase-specialist/spec-lens-c.md`:

```markdown
# Spec Lens C — Live-Spec Conflicts & Migration: Agentic Phase Specialist

## Assumed requirements

<2-4 sentences naming the requirement set you are checking against live specs.
Lens A and lens B write this same block independently; the synthesizer compares
all three.>

## Live Spec Inventory

| Capability | Live spec (file:line) | Requirements | Scenarios | Touched by this change |
|---|---|---|---|---|

## Conflicts

<Every place this change contradicts a live requirement rather than extending it.
"None" if there are none.>

## MODIFIED Full Blocks

### Requirement: <Live Requirement Name>

**Source**: `openspec/specs/<capability>/spec.md:<line>` — <N> scenarios

<The COMPLETE live block, copied verbatim.>

## Removals and Renames

| Requirement | Removed or renamed | Reason | Consumers (file:line) | Migration |
|---|---|---|---|---|

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`spec-lens-c.md` MUST be under 1000 words **excluding the verbatim blocks under `## MODIFIED Full Blocks`**.

## Out of scope

Owned by the sibling lenses:

- **Lens A owns**: the capability map, new requirement statements, classification.
- **Lens B owns**: every new `#### Scenario:` block and the coverage argument.

The scenarios inside a `## MODIFIED Full Blocks` entry are yours (copied evidence). Any new scenario is lens B's.

Do NOT create or write any file under `openspec/changes/agentic-phase-specialist/specs/`.

## Allowed paths

`openspec/changes/agentic-phase-specialist/spec-lens-c.md` only.

## Allowed paths outside the repository

**Read-only**: The real `gentle-ai` spec skill and its `references/` (delivered under `## Required skills`). Not symmetric: the skill governs *what* a delta spec must contain (MODIFIED copy-full-then-edit workflow, REMOVED Reason-and-Migration rule, RENAMED both-names rule); this packet governs *how this phase is executed here*. Note conflicts in `## Open Questions`.

Write nothing outside this repository.

## Citation manifest (REQUIRED — excluded from the word budget)

One row per unique citation, grouped by file, ascending line numbers, plainly stated claim. Worklist, not a certificate.

## Mechanical self-check (REQUIRED — replaces narrating these facts)

**Before you commit**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/spec-lens-c.md --budget 1000 \
  --exclude-section "MODIFIED Full Blocks" --exclude-section "Citation Manifest" \
  --require-section "Assumed requirements" --require-section "Live Spec Inventory" \
  --require-section "Conflicts" --require-section "MODIFIED Full Blocks" \
  --require-section "Removals and Renames" --require-section "Open Questions" \
  --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/spec-lens-c.md
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` reported no FAIL against this draft's own manifest.**
- [ ] **Every capability listed as modified was opened in full**, and its inventory row's counts came from the file.
- [ ] **Every `## MODIFIED Full Blocks` entry is the complete live block**, nothing summarized or elided.
- [ ] **`spec-lens-c.md` exists, is under 1000 words excluding verbatim blocks and the Citation Manifest, and carries `## Assumed requirements`, `## Live Spec Inventory`, and `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution.** Strip any injected `Co-authored-by:` trailer.

## Hard stops

- A capability the proposal lists as modified has no live spec to read.
- A requirement being removed has consumers the proposal never mentions.
- Copying a MODIFIED block whole would exceed what you can write — report which requirement forces it, never write a partial block.
- Satisfying one instruction in this packet would require violating another.

## Context

**Change**: `agentic-phase-specialist`, proposal accepted. Modified Capabilities per the proposal: `phase-specialist-dispatch` ("agentic Specialist is the decision-maker; the Go adapter stays the status/eligibility/dispatch tool", `openspec/specs/phase-specialist-dispatch/spec.md:9-11`); `acceptance-verifier` ("phase-gate `lucind-checks.sh`; Acceptance still must not mutate refs or invoke Promotion", `openspec/specs/acceptance-verifier/spec.md:30-33,124-127`); `sdd-planning-fan-out` ("synthesis review and contradiction arbitration move to the Specialist", `openspec/specs/sdd-planning-fan-out/spec.md:9-12`). New Capability: `phase-verdict-reporting` (no live spec — this is an ADDED capability, full new spec file, not a delta).

**Do not relitigate**: Promotion stays human-confirmed; `sdd-*` lack Bash and the Specialist's Acceptance requirement should be read as decision authority (the Orchestrator executes mechanically), not literal self-execution — if the live spec text at any of the three capabilities already implies otherwise, flag it as a Conflict, don't silently resolve it yourself.

## Required skills

- sdd-spec

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
