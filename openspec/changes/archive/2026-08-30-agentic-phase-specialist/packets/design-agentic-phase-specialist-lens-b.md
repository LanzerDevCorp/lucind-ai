---
id: design-agentic-phase-specialist-lens-b
executor: agy
routed_by: surface-and-flow lens of the three-lens design fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/agentic-phase-specialist/design-lens-b.md"]
---

# Packet design-agentic-phase-specialist-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/design-agentic-phase-specialist-lens-b  ·  **Branch:** lucind/design-agentic-phase-specialist-lens-b

## Goal

Produce `openspec/changes/agentic-phase-specialist/design-lens-b.md`: how data moves through this change, invariants at each hop, exact signature/format deltas, and the file-change table with a terminal consumer per row.

This is one of three parallel design lenses. It is feedstock for a synthesis lane, not the final design document. Do not write a complete `design.md`.

## Why this is safe to dispatch now

The proposal and specs for `agentic-phase-specialist` are accepted and frozen. Lens A and lens C run in parallel against the same frozen inputs and write to different files, so no lane races another.

Lens A owns the architecture decision and is running concurrently, so you do not have it. Declare the architecture you are assuming in `## Assumed architecture` and design against it consistently.

## Preconditions

- `openspec/changes/agentic-phase-specialist/proposal.md` exists and is accepted.
- `openspec/changes/agentic-phase-specialist/specs/` exists.
- `openspec/changes/agentic-phase-specialist/design-lens-b.md` does not yet exist.

## Required reading (this lens only)

1. The real `gentle-ai` design skill (delivered under `## Required skills`).
2. `openspec/changes/agentic-phase-specialist/proposal.md` and `openspec/changes/agentic-phase-specialist/specs/`.
3. `internal/accept/accept.go` in full (not just the cited ranges) — the exact struct/function signatures around `Verifier.Verify`, `CheckPolicySnapshot`, and where `LaneMetadata` is loaded.
4. `internal/ledger/lanes_meta.go` in full — the `LaneMetadata` struct and its JSON tags, to state precisely how `SDDPhase` is read today and whether any format change is needed (the proposal says reuse, not add).
5. `internal/run/attempt.go` around line 431-435 and its surrounding function signature.
6. `plugin/claude-code/skills/lucind-ai/SKILL.md:19` and the OpenCode mirror, verbatim, for the exact Hard Rule text the carve-out edits.

Never guess at a signature. Every row carries a `file:line` citation to real code in this worktree.

## Output format

Write exactly this skeleton to `openspec/changes/agentic-phase-specialist/design-lens-b.md`:

```markdown
# Design Lens B — Surface & Flow: Agentic Phase Specialist

## Assumed architecture

<2-4 sentences. Lens A and lens C write this same block independently.>

## Flow and Invariants

<How a phase's fan-out+synthesis dispatch flows from Orchestrator authoring
packets, through lucind-ai run/accept, to the Specialist's Acceptance judgment,
to the Phase Verdict returned. An ASCII diagram when it clarifies.>

    Orchestrator (authors packets, runs lucind-ai) ──→ Lens Lanes ──→ Synthesis Lane ──→ Specialist (judges Acceptance) ──→ Phase Verdict ──→ Orchestrator

## Surface Deltas

| Symbol or format | Today (file:line) | Delta | Backward compatible? |
|---|---|---|---|

<Must include rows for: the Hard Rule text at SKILL.md:19 (both trees); the
check-gating condition in accept.go and attempt.go; any new field or none on
LaneMetadata (the proposal says reuse SDDPhase, verify no new field is needed);
the Phase Verdict's shape per lens A's decision.>

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|

<Include: plugin/claude-code/skills/lucind-ai/SKILL.md, plugin/opencode/skills/lucind-ai/SKILL.md,
plugin/*/skills/lucind-ai/references/strategies/fan-out.md, plugin/*/skills/lucind-ai/references/contracts/acceptance-promotion.md,
internal/accept/accept.go, internal/run/attempt.go, plus their _test.go files.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`design-lens-b.md` MUST be under 1000 words.

## Out of scope

Owned by the sibling lenses:

- **Lens A owns**: technical approach, architecture decisions, alternatives, rationale.
- **Lens C owns**: testing strategy, test seams, threat matrix, rollback/additivity.

Do not assess whether the change is additively revertible — that is lens C's from your surface deltas.

## Allowed paths

`openspec/changes/agentic-phase-specialist/design-lens-b.md` only.

## Allowed paths outside the repository

**Read-only**: The real `gentle-ai` design skill and its `references/` (delivered under `## Required skills`). Not symmetric: skill governs *what*; this packet governs *how this phase is executed here*, including this repository's actual 1800-word canonical budget.

Write nothing outside this repository.

## Citation manifest (REQUIRED — excluded from the word budget)

One row per unique citation, grouped by file, ascending line numbers, plainly stated claim.

## Mechanical self-check (REQUIRED — replaces narrating these facts)

**Before you commit**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/design-lens-b.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Assumed architecture" --require-section "Flow and Invariants" \
  --require-section "Surface Deltas" --require-section "File Changes" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/design-lens-b.md
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` reported no FAIL against this draft's own manifest.**
- [ ] **Every `Surface Deltas` and `File Changes` row carries a `file:line` citation to real code**, and every `File Changes` row names a terminal consumer.
- [ ] **`design-lens-b.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section.**
- [ ] **The work is committed with a conventional commit and no AI attribution.**

## Hard stops

- The specs do not determine whether a format delta is additive or breaking.
- A file change cannot name any terminal consumer.
- Two reasonable architectures produce incompatible surface tables and the proposal does not choose.
- Satisfying one instruction in this packet would require violating another.

## Context

**Change**: `agentic-phase-specialist`. Proposal's Rollback Plan and Additivity states: reuse `LaneMetadata.SDDPhase` (`lanes_meta.go:20-47,49-60`); evidence version stays `"lane-authoring-evidence/v1"`; `Contract` remains `json.RawMessage` (`authoring.go:14,26,44-75`); no DDL, no schema migration (`internal/ledger/schema.go:425-445,584-592`). Affected Areas per proposal: `plugin/*/skills/lucind-ai/SKILL.md:19` (Hard Rule carve-out); `fan-out.md:47-48` (synthesis-note review moves to Specialist); `acceptance-promotion.md:31-36` (Acceptance Subagent becomes decision-bearing); `internal/accept/accept.go:84-137` (unconditional metadata load, phase-gate `v.check`); `internal/run/attempt.go:431-435` (equivalent gate).

**Do not relitigate**: no new field on `LaneMetadata`; `internal/phasespec.Adapter` unchanged; `integrate.Check` itself unchanged (gate lives at callers only).

## Required skills

- sdd-design

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
