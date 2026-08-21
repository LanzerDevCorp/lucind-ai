---
id: explore-<change-id>-lens-a
executor: agy
routed_by: problem and candidates lens of the three-lens explore fan-out
model: <model, e.g. gemini-3.7-flash-high>
allowed_paths: ["openspec/changes/<change-id>/explore-lens-a.md"]
---

# Packet explore-<change-id>-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../<repo>-worktrees/explore-<change-id>-lens-a  ·  **Branch:** lucind/explore-<change-id>-lens-a

## Goal

Produce `openspec/changes/<change-id>/explore-lens-a.md`: the problem space definition, motivation, background, and candidate approaches for this change, each with its description, pros, cons, and feasibility assessment.

This is one of three parallel explore lenses. It is feedstock for a synthesis lane, not the final explore document. Do not write a complete `explore.md`.

## Why this is safe to dispatch now

The exploration for `<change-id>` is initiating. Lens B and lens C run in parallel against the same codebase and write to different files, so no lane races another. This lens owns the problem definition and candidate solutions; the other two explore capabilities and risks.

## Preconditions

- `openspec/changes/<change-id>/` exists (or the packet `## Context` states the change goal).
- `openspec/changes/<change-id>/explore-lens-a.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to problem space and candidates, not to scenarios or risk matrices:

1. `~/.claude/skills/sdd-explore/SKILL.md` — the real `gentle-ai` explore skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. The entry points and module structure of the packages relevant to the problem space.
3. The existing patterns and conventions those packages already follow — how comparable problems were already solved in this repository.
4. `openspec/changes/archive/` for prior explorations or changes that addressed similar problem spaces, if one exists.

Never guess at code. Every claim about existing structure carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/<change-id>/explore-lens-a.md`:

```markdown
# Explore Lens A — Problem & Candidates: <Change Title>

## Problem Space

<Concise description of the problem, background, current limitations, and motivation for the change. Cite file:line for existing code behavior.>

## Candidate Approaches

### Candidate 1 — <title>

**Approach**: <summary of candidate approach>
**Pros**: <advantages>
**Cons**: <disadvantages and costs>
**Feasibility**: <assessment grounded in this codebase with file:line citations>

### Candidate N — <title>

<same four fields>

## Initial Recommendations

<Preliminary recommendation among candidates, with technical rationale.>

## Open Questions

- [ ] <unresolved technical question, or "None">
```

## Size budget

`explore-lens-a.md` MUST be under 1000 words. Candidate descriptions as compact blocks, not essays. Code snippets only for a non-obvious pattern, never to restate code the reader can open.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens B owns**: capabilities, user scenarios, and success criteria.
- **Lens C owns**: technical risks, unknowns, trade-offs matrix, and potential spikes/proof-of-concepts.

Do not write a risks matrix or detailed user scenarios here. They belong to lenses B and C.

## Allowed paths

`openspec/changes/<change-id>/explore-lens-a.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-explore/` — the real `gentle-ai` explore skill and its
`references/`. Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what an explore document must contain*: its required sections, the
problem / candidates / feasibility shape of exploration, and the exploration taxonomy.
Where this packet paraphrases any of that and drifts, the skill wins and the drift belongs in
`## Open Questions`.

This packet is authority on *how this phase is being executed here*: that the exploration is split
across three parallel lanes, which slice this lane owns, its word budget, its output path and
skeleton, its out-of-scope list, and its done criteria. The skill describes one sub-agent writing
a whole `explore.md` by itself, so parts of it will read as instructing you to do what this packet
forbids — write the complete document, persist it to Engram, return the phase summary block.
Those are superseded here on purpose. Do not correct yourself toward them; note the conflict in
`## Open Questions` and follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Done criteria

- [ ] **Every candidate and problem claim carries `file:line` citations to real code in this worktree.**
- [ ] **`explore-lens-a.md` exists, is under 1000 words, and carries every skeleton section.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The problem scope cannot be identified from codebase inspection or packet context.
- Candidate approaches cannot be formulated without designing complete implementation details (which belongs to design phase).
- Satisfying one instruction in this packet would require violating another.

## Context

<Ground-truth facts with file:line references: the packages involved, the
relevant existing types, the problem statement, and any decision the
human has already made in conversation and does not want re-litigated.>

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
