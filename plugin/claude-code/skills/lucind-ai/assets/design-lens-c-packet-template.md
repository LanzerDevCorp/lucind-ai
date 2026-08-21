---
id: design-<change-id>-lens-c
executor: agy
routed_by: failure-test-rollback lens of the three-lens design fan-out
model: <model, e.g. gemini-3.7-flash-high>
allowed_paths: ["openspec/changes/<change-id>/design-lens-c.md"]
---

# Packet design-<change-id>-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../<repo>-worktrees/design-<change-id>-lens-c  ·  **Branch:** lucind/design-<change-id>-lens-c

## Goal

Produce `openspec/changes/<change-id>/design-lens-c.md`: how this change is tested, which seams already exist to test it through, the applicability-driven threat matrix, and the rollback/additivity decision.

This is one of three parallel design lenses. It is feedstock for a synthesis lane, not the final design document. Do not write a complete `design.md`.

## Why this is safe to dispatch now

The proposal and specs for `<change-id>` are accepted and frozen. Lens A and lens B run in parallel against the same frozen inputs and write to different files, so no lane races another.

Lens A owns the architecture decision and is running concurrently, so you do not have it. Declare the architecture you are assuming in `## Assumed architecture` and design against it consistently. The synthesizer arbitrates divergence; a silent second architecture does not survive that arbitration.

## Preconditions

- `openspec/changes/<change-id>/proposal.md` exists and is accepted.
- `openspec/changes/<change-id>/specs/` exists (or the packet `## Context` states it does not).
- `openspec/changes/<change-id>/design-lens-c.md` does not yet exist.
- The threat-matrix reference table is embedded verbatim in this packet's `## Context`.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to failure, proof, and reversal — not to rationale or signatures:

1. `~/.claude/skills/sdd-design/SKILL.md` — the real `gentle-ai` design skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/<change-id>/proposal.md` and `openspec/changes/<change-id>/specs/`.
3. The existing test files for the packages in scope. Read how this repository actually tests: what it asserts on, what it fakes, what it refuses to mock.
4. The injection seams that already exist — runner interfaces, `Deps`-style structs, function fields, existing fakes and stubs. Name them by `file:line`.
5. The threat-matrix table in `## Context` of this packet, and
   `~/.claude/skills/sdd-design/references/threat-matrix.md` behind it. The embedded copy is the
   frozen evidence; the reference is the authority. Report any drift between them.

Never guess at a test seam. A seam you cannot cite does not exist yet, and saying so is the useful answer.

## Output format

Write exactly this skeleton to `openspec/changes/<change-id>/design-lens-c.md`:

```markdown
# Design Lens C — Failure, Test & Rollback: <Change Title>

## Assumed architecture

<2–4 sentences naming the structural shape you are designing against: which
existing types or packages get extended, which are new. Lens A and lens B write
this same block independently; the synthesizer compares all three. Be specific
enough that a disagreement is visible.>

## Testing Strategy

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|

<Unit / Integration / E2E. The seam column cites the injection point that makes
the test possible, or states "new seam required" and what it would be.>

## Test Seams

<What is injectable or fakeable today, and what this change would have to add.
A change that needs a new seam is a design fact, not an implementation detail —
say it here so tasks can schedule it.>

## Threat Matrix

<The table from `## Context`, every row marked `Applicable` or `N/A: <reason>`.
For every applicable row: the expected safe behavior, the expected failure
behavior, and the concrete RED test that proves it. Invent no rows and no tests
for `N/A` rows. If no routing, shell, subprocess, VCS/PR automation,
executable-file classification, or process-integration boundary exists, record
`N/A — no such boundary` and stop.>

## Rollback and Additivity

**Choice**: <what reverting looks like>
**Alternatives considered**: <what other reversal strategy was rejected>
**Rationale**: <why, grounded in what the format deltas actually move>

<State explicitly whether any schema, ledger, or envelope version moves, and
what reverting the apply commits restores. "Purely additive" is a claim that
needs the evidence next to it.>

## Out of Scope

<Adjacent work this change explicitly does not do, and which sibling change or
deferral owns it.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`design-lens-c.md` MUST be under 1000 words. Tables over prose. The threat matrix rows count toward the budget — keep the reasons to one clause.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens A owns**: the technical approach and every architecture decision except rollback.
- **Lens B owns**: the file-changes table, data-flow diagrams, invariants, and every exact type/schema/CLI signature delta.

Rollback is yours even though it is shaped like an architecture decision. Everything else shaped like one is lens A's.

## Allowed paths

`openspec/changes/<change-id>/design-lens-c.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-design/` — the real `gentle-ai` design skill and its
`references/`. Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what a design document must contain*: its required sections, the
choice / alternatives / rationale shape of a decision, and the threat-matrix applicability rule.
Where this packet paraphrases any of that and drifts, the skill wins and the drift belongs in
`## Open Questions`.

This packet is authority on *how this phase is being executed here*: that the design is split
across three parallel lanes, which slice this lane owns, its word budget, its output path and
skeleton, its out-of-scope list, and its done criteria. The skill describes one sub-agent writing
a whole `design.md` by itself, so parts of it will read as instructing you to do what this packet
forbids — write the complete document, persist it to Engram, return the phase summary block, hold
an 800-word budget. Those are superseded here on purpose. Do not correct yourself toward them; note
the conflict in `## Open Questions` and follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Done criteria

- [ ] **Every named test seam carries a `file:line` citation that points at real code in this worktree**, or is explicitly marked "new seam required".
- [ ] **Every threat-matrix row is marked `Applicable` or `N/A` with a reason**, and every applicable row names a planned RED test.
- [ ] **`design-lens-c.md` exists, is under 1000 words, and carries every skeleton section including `## Assumed architecture`.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- A behavior the specs require cannot be tested through any existing or proposed seam.
- Whether a format delta is additive cannot be determined from the specs, so the rollback decision would be a guess.
- The threat matrix is missing from both `## Context` and the skill reference.
- Satisfying one instruction in this packet would require violating another.

## Context

### Threat-matrix reference table

<Embed the applicability-driven threat matrix verbatim here — boundary rows for
documentation-like paths, git repository selection, commit state, push state,
and PR commands, with the "Applicable / N/A: reason" column. The lane must not
read it only from the skill reference above.>

### Ground truth

<Facts with file:line references: the test files and seams in scope, the current
schema/ledger/envelope versions, and any decision the human has already made in
conversation and does not want re-litigated.>

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
