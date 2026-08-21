---
id: design-<change-id>-lens-a
executor: agy
routed_by: decisions lens of the three-lens design fan-out
model: <model, e.g. gemini-3.7-flash-high>
allowed_paths: ["openspec/changes/<change-id>/design-lens-a.md"]
---

# Packet design-<change-id>-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../<repo>-worktrees/design-<change-id>-lens-a  ·  **Branch:** lucind/design-<change-id>-lens-a

## Goal

Produce `openspec/changes/<change-id>/design-lens-a.md`: the technical approach and every architecture decision for this change, each with its choice, the alternatives rejected, the rationale, and the terminal consumer that makes the decision observable.

This is one of three parallel design lenses. It is feedstock for a synthesis lane, not the final design document. Do not write a complete `design.md`.

## Why this is safe to dispatch now

The proposal and specs for `<change-id>` are accepted and frozen. Lens B and lens C run in parallel against the same frozen inputs and write to different files, so no lane races another. This lens owns the architectural choice; the other two consume it.

## Preconditions

- `openspec/changes/<change-id>/proposal.md` exists and is accepted.
- `openspec/changes/<change-id>/specs/` exists (or the packet `## Context` states it does not).
- `openspec/changes/<change-id>/design-lens-a.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to structure, not to signatures or tests:

1. `openspec/changes/<change-id>/proposal.md` and `openspec/changes/<change-id>/specs/`.
2. The entry points and module structure of the packages the change lands in.
3. The existing patterns and conventions those packages already follow — how comparable problems were already solved in this repository.
4. `openspec/changes/archive/` for a prior change that solved a structurally similar problem, if one exists.

Never guess at code. Every claim about existing structure carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/<change-id>/design-lens-a.md`:

```markdown
# Design Lens A — Decisions: <Change Title>

## Assumed architecture

<2–4 sentences naming the structural shape you are designing against: which
existing types or packages get extended, which are new. Lens B and lens C write
this same block independently; the synthesizer compares all three. Be specific
enough that a disagreement is visible.>

## Technical Approach

<Concise strategy. How it maps to the proposal. Reference spec requirements by name.>

## Decision 1 — <title>

**Choice**: <what we chose>
**Alternatives considered**: <what we rejected>
**Rationale**: <why this over the alternatives, grounded in this repository's code>
**Terminal consumer**: <the concrete symbol, command, or spec requirement that
consumes this decision — with file:line. A decision no terminal consumer reaches
is not a decision, it is speculation.>

## Decision N — <title>

<same four fields>

## Open Questions

- [ ] <unresolved technical question, or "None">
```

## Size budget

`design-lens-a.md` MUST be under 700 words. Decisions as compact blocks, not essays. Code snippets only for a non-obvious pattern, never to restate code the reader can open.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens B owns**: the file-changes table, data-flow diagrams, invariants, and every exact type/schema/CLI signature delta.
- **Lens C owns**: testing strategy, test seams, the threat matrix, and the rollback/additivity decision.

Do not write a rollback decision here even though it is shaped like a decision. It belongs to lens C.

## Allowed paths

`openspec/changes/<change-id>/design-lens-a.md` only. Create no other file.

## Allowed paths outside the repository

None.

## Done criteria

- [ ] **Every decision names a terminal consumer with a `file:line` citation**, and that citation points at real code in this worktree.
- [ ] **`design-lens-a.md` exists, is under 700 words, and carries every skeleton section including `## Assumed architecture`.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The proposal or specs do not determine an architectural choice, and two reasonable shapes are equally supported.
- A decision cannot name any terminal consumer.
- Satisfying one instruction in this packet would require violating another.
- Making this decision would require designing the file-change surface or the test strategy, which this packet forbids.

## Context

<Ground-truth facts with file:line references: the packages involved, the
relevant existing types, the accepted proposal summary, and any decision the
human has already made in conversation and does not want re-litigated.>

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
