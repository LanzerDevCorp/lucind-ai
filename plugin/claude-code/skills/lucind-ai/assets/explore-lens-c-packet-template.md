---
id: explore-<change-id>-lens-c
executor: agy
routed_by: risks, trade-offs, and spikes lens of the three-lens explore fan-out
model: <model, e.g. gemini-3.7-flash-high>
allowed_paths: ["openspec/changes/<change-id>/explore-lens-c.md"]
---

# Packet explore-<change-id>-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../<repo>-worktrees/explore-<change-id>-lens-c  ·  **Branch:** lucind/explore-<change-id>-lens-c

## Goal

Produce `openspec/changes/<change-id>/explore-lens-c.md`: technical risks, trade-offs matrix, spike proposals, and out-of-scope boundaries for this change.

This is one of three parallel explore lenses. It is feedstock for a synthesis lane, not the final explore document. Do not write a complete `explore.md`.

## Why this is safe to dispatch now

The exploration for `<change-id>` is initiating. Lens A and lens B run in parallel against the same codebase and write to different files, so no lane races another.

## Preconditions

- `openspec/changes/<change-id>/` exists (or the packet `## Context` states the change goal).
- `openspec/changes/<change-id>/explore-lens-c.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to risks, trade-offs, and spikes:

1. `~/.claude/skills/sdd-explore/SKILL.md` — the real `gentle-ai` explore skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. The complex, performance-sensitive, or stateful subsystems that this change interacts with.
3. Existing test suites, failure modes, error handling, and reliability boundaries.
4. `openspec/changes/archive/` for historical trade-offs and edge-case postmortems.

Never guess at code or risks. Every claim about existing code mechanisms carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/<change-id>/explore-lens-c.md`:

```markdown
# Explore Lens C — Risks, Trade-offs & Spikes: <Change Title>

## Technical Risks & Unknowns

| Risk / Unknown | Severity | Mitigation / Investigation Strategy | Existing seam (file:line) |
|---|---|---|---|

## Trade-offs Matrix

| Dimension / Choice | Advantages | Disadvantages | Operational Cost |
|---|---|---|---|

## Potential Spikes / Proof of Concepts

<Targeted experiments or prototype spikes needed to de-risk technical unknowns, citing existing code seams with file:line.>

## Out of Scope

<Adjacent problems, features, or refactors explicitly excluded from this change.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`explore-lens-c.md` MUST be under 1000 words. Tables over prose.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens A owns**: problem space definition, candidate approaches, and initial recommendations.
- **Lens B owns**: capabilities, user scenarios, and success criteria.

Do not define user personas or author candidate architectural designs here. They belong to lenses A and B.

## Allowed paths

`openspec/changes/<change-id>/explore-lens-c.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-explore/` — the real `gentle-ai` explore skill and its
`references/`. Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what an explore document must contain*: its required sections, the
risk / trade-off / spike shape of exploration.
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

- [ ] **Every risk and spike proposal carries `file:line` citations to real code in this worktree.**
- [ ] **`explore-lens-c.md` exists, is under 1000 words, and carries every skeleton section.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- Critical unknown cannot be framed as a risk or spike with available codebase knowledge.
- The change requires touching fundamental invariant boundaries without possible mitigation.
- Satisfying one instruction in this packet would require violating another.

## Context

<Ground-truth facts with file:line references: the critical code paths,
concurrency/data invariants, and any decision the human has already made in
conversation and does not want re-litigated.>

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
