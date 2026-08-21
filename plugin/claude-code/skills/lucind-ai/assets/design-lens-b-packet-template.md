---
id: design-<change-id>-lens-b
executor: agy
routed_by: surface-and-flow lens of the three-lens design fan-out
model: <model, e.g. gemini-3.7-flash-high>
allowed_paths: ["openspec/changes/<change-id>/design-lens-b.md"]
---

# Packet design-<change-id>-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../<repo>-worktrees/design-<change-id>-lens-b  ·  **Branch:** lucind/design-<change-id>-lens-b

## Goal

Produce `openspec/changes/<change-id>/design-lens-b.md`: how data moves through the change, the invariants that must hold at each hop, the exact signature and format deltas the change introduces, and the file-change table with a terminal consumer per row.

This is one of three parallel design lenses. It is feedstock for a synthesis lane, not the final design document. Do not write a complete `design.md`.

## Why this is safe to dispatch now

The proposal and specs for `<change-id>` are accepted and frozen. Lens A and lens C run in parallel against the same frozen inputs and write to different files, so no lane races another.

Lens A owns the architecture decision and is running concurrently, so you do not have it. Declare the architecture you are assuming in `## Assumed architecture` and design against it consistently. The synthesizer arbitrates divergence; a silent second architecture does not survive that arbitration.

## Preconditions

- `openspec/changes/<change-id>/proposal.md` exists and is accepted.
- `openspec/changes/<change-id>/specs/` exists (or the packet `## Context` states it does not).
- `openspec/changes/<change-id>/design-lens-b.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to surfaces and formats, not to rationale or tests:

1. `~/.claude/skills/sdd-design/SKILL.md` — the real `gentle-ai` design skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/<change-id>/proposal.md` and `openspec/changes/<change-id>/specs/`.
3. The exact type, struct, and interface declarations the change touches — read the declarations, not summaries of them.
4. Every persisted or wire format in scope: JSON schemas, YAML sidecar structs, packet frontmatter parsing, ledger rows, result envelopes.
5. The CLI flag and argument surface in `cmd/` for anything the change exposes to an operator.

Never guess at a signature. Every row in your tables carries a `file:line` citation to real code in this worktree.

## Output format

Write exactly this skeleton to `openspec/changes/<change-id>/design-lens-b.md`:

```markdown
# Design Lens B — Surface & Flow: <Change Title>

## Assumed architecture

<2–4 sentences naming the structural shape you are designing against: which
existing types or packages get extended, which are new. Lens A and lens C write
this same block independently; the synthesizer compares all three. Be specific
enough that a disagreement is visible.>

## Flow and Invariants

<How data moves for this change. A simple ASCII diagram when it clarifies —
clarity over beauty. Then the invariant that must hold at each hop, and what
observably breaks if it does not.>

    Component A ──→ Component B ──→ Component C

## Surface Deltas

| Symbol or format | Today (file:line) | Delta | Backward compatible? |
|---|---|---|---|

<One row per type, field, schema property, frontmatter key, or CLI flag the
change adds, changes, or removes. "Backward compatible?" must be yes/no with a
one-clause reason, never blank.>

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|

<Create / Modify / Delete. The terminal consumer column names the symbol,
command, or spec requirement that reaches this file's change — with file:line
where it already exists.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`design-lens-b.md` MUST be under 1000 words. Tables over prose. Do not restate code the reader can open; cite it.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens A owns**: the technical approach, every architecture decision, the alternatives rejected, and the rationale.
- **Lens C owns**: testing strategy, test seams, the threat matrix, and the rollback/additivity decision.

You will be tempted to argue for an architecture while tabulating its surface. Do not. If you believe the architecture you assumed is the wrong one, say so under `## Open Questions` with the evidence — do not quietly design a different one.

Do not assess whether the change is additively revertible. You supply the format deltas; lens C decides rollback from them.

## Allowed paths

`openspec/changes/<change-id>/design-lens-b.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-design/` — the real `gentle-ai` design skill and its
`references/`. Read the contract as written, not as this packet paraphrases it; where the two
disagree, the skill wins and the disagreement belongs in `## Open Questions`. Write nothing
outside this repository, so there is nothing to revert.

## Done criteria

- [ ] **Every `Surface Deltas` and `File Changes` row carries a `file:line` citation that points at real code in this worktree**, and every `File Changes` row names a terminal consumer.
- [ ] **`design-lens-b.md` exists, is under 1000 words, and carries every skeleton section including `## Assumed architecture`.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The specs do not determine whether a format delta is additive or breaking.
- A file change cannot name any terminal consumer.
- Two reasonable architectures produce incompatible surface tables and the proposal does not choose between them.
- Satisfying one instruction in this packet would require violating another.

## Context

<Ground-truth facts with file:line references: the exact declarations in scope,
the schema files, the persisted formats and their current versions, and any
decision the human has already made in conversation and does not want
re-litigated.>

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
