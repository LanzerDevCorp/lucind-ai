---
id: propose-<change-id>-lens-a
executor: agy
routed_by: candidate and approach lens of the three-lens propose fan-out
model: <model, e.g. gemini-3.7-flash-high>
allowed_paths: ["openspec/changes/<change-id>/propose-lens-a.md"]
---

# Packet propose-<change-id>-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../<repo>-worktrees/propose-<change-id>-lens-a  ·  **Branch:** lucind/propose-<change-id>-lens-a

## Goal

Produce `openspec/changes/<change-id>/propose-lens-a.md`: the candidate selection, proposed technical approach, changes to system concepts, and architecture rationale for this change.

This is one of three parallel propose lenses. It is feedstock for a synthesis lane, not the final proposal document. Do not write a complete `proposal.md`.

## Why this is safe to dispatch now

The proposal phase for `<change-id>` is initiating. Lens B and lens C run in parallel against the same codebase and write to different files, so no lane races another. This lens owns candidate selection and core approach; the other two explore capability specs and risk/rollback.

## Preconditions

- `openspec/changes/<change-id>/` exists (and `explore.md` exists if explore was run).
- `openspec/changes/<change-id>/propose-lens-a.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to candidate selection and approach, not to spec delta authoring or test matrices:

1. `~/.claude/skills/sdd-propose/SKILL.md` — the real `gentle-ai` propose skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/<change-id>/explore.md` (if present).
3. The entry points and module structure of the packages relevant to the proposal.
4. The existing patterns and conventions those packages already follow — how comparable problems were already solved in this repository.
5. `openspec/changes/archive/` for prior proposals that addressed similar changes.

Never guess at code. Every claim about existing structure carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/<change-id>/propose-lens-a.md`:

```markdown
# Proposal Lens A — Candidate & Approach: <Change Title>

## Selected Candidate & Approach

<State the chosen candidate from exploration, summarize the core approach, and explain why this approach solves the problem. Cite file:line for existing code behavior.>

## Conceptual Changes & Architecture Rationale

<Describe additions, modifications, or deprecations to system concepts, interfaces, or architectural patterns. Cite file:line for existing concepts.>

## Alternatives Considered & Rejected

<What alternative approaches were considered during candidate selection and why they were rejected.>

## Open Questions

- [ ] <unresolved technical question, or "None">
```

## Size budget

`propose-lens-a.md` MUST be under 1000 words. Approach descriptions as compact blocks, not essays. Code snippets only for a non-obvious pattern, never to restate code the reader can open.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens B owns**: capability impact table, delta specification requirements, and scenarios.
- **Lens C owns**: technical risks, failure modes, rollback plan, additivity assessment, and test impact.

Do not write delta spec requirements or a rollback plan here. They belong to lenses B and C.

## Allowed paths

`openspec/changes/<change-id>/propose-lens-a.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-propose/` — the real `gentle-ai` propose skill and its
`references/`. Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what a proposal document must contain*: its required sections, the
candidate selection, approach, and conceptual change shape of a proposal.
Where this packet paraphrases any of that and drifts, the skill wins and the drift belongs in
`## Open Questions`.

This packet is authority on *how this phase is being executed here*: that the proposal is split
across three parallel lanes, which slice this lane owns, its word budget, its output path and
skeleton, its out-of-scope list, and its done criteria. The skill describes one sub-agent writing
a whole `proposal.md` by itself, so parts of it will read as instructing you to do what this packet
forbids — write the complete document, persist it to Engram, return the phase summary block.
Those are superseded here on purpose. Do not correct yourself toward them; note the conflict in
`## Open Questions` and follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Done criteria

- [ ] **Every candidate and approach claim carries `file:line` citations to real code in this worktree.**
- [ ] **`propose-lens-a.md` exists, is under 1000 words, and carries every skeleton section.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The technical approach cannot be grounded in existing codebase patterns or proposal context.
- Candidate selection contradicts frozen exploration conclusions without justification.
- Satisfying one instruction in this packet would require violating another.

## Context

<Ground-truth facts with file:line references: the packages involved, the
relevant existing types, the change goals, and any decision the
human has already made in conversation and does not want re-litigated.>

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
