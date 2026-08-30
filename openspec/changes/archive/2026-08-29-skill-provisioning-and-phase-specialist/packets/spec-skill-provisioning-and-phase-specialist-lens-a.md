---
id: spec-skill-provisioning-and-phase-specialist-lens-a
executor: agy
routed_by: capabilities and requirements lens of the three-lens spec fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/skill-provisioning-and-phase-specialist/spec-lens-a.md"]
---

# Packet spec-skill-provisioning-and-phase-specialist-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/spec-skill-provisioning-and-phase-specialist-lens-a  ·  **Branch:** lucind/spec-skill-provisioning-and-phase-specialist-lens-a

## Goal

Produce `openspec/changes/skill-provisioning-and-phase-specialist/spec-lens-a.md`: the
capability-to-file map for this change's eight capabilities, and every requirement statement it
introduces or changes, each classified `ADDED` / `MODIFIED` / `REMOVED` / `RENAMED`.

This is one of three parallel spec lenses. It is feedstock for a synthesis lane, not the final
delta spec. Do not write anything under `openspec/changes/skill-provisioning-and-phase-specialist/specs/`.

## Why this is safe to dispatch now

`proposal.md` is accepted, frozen, and already committed on this branch. Lens B and lens C run in
parallel against the same frozen input and write to different files, so no lane races another.
This lens owns the requirement set; the other two write scenarios for it and check it against live
specs.

## Preconditions

- `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md` exists and is accepted.
- `openspec/changes/skill-provisioning-and-phase-specialist/spec-lens-a.md` does not yet exist.
- `openspec/specs/` exists with the four live capabilities this change modifies.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-spec/SKILL.md` — the real `gentle-ai` spec skill.
2. `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md` (full), and its
   **Capabilities** and **Delta Specifications** sections in particular — the proposal already
   drafted six requirement stubs; treat them as a starting point to verify and complete, not as
   final text to copy blind.
3. The index of `openspec/specs/` — confirm which of the eight named capabilities (four New, four
   Modified) already have a live spec directory and which do not.
4. `openspec/changes/archive/` for a prior change that introduced a similarly multi-capability
   change, if one exists, to see this repository's requirement granularity.

Never guess at a capability name. A capability you cannot cite in the proposal or in
`openspec/specs/` does not exist yet, and saying so is the useful answer.

## Output format

Write exactly this skeleton to
`openspec/changes/skill-provisioning-and-phase-specialist/spec-lens-a.md`:

```markdown
# Spec Lens A — Capabilities & Requirements: Skill Provisioning and the SDD Phase Specialist

## Assumed requirements

<2-4 sentences naming the requirement set: eight capabilities (four New, four
Modified), how many requirements each gets. Lens B and lens C write this same
block independently; the synthesizer compares all three. Be specific enough
that a disagreement is visible.>

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|

<One row per capability the proposal names under New/Modified Capabilities.
"New" rows target openspec/specs/<capability>/spec.md and cite nothing.
"Existing" rows target
openspec/changes/skill-provisioning-and-phase-specialist/specs/<capability>/spec.md
and MUST cite the live spec they delta.>

## ADDED Requirements

### Requirement: <Name>

<Requirement text with an RFC 2119 keyword. Observable behavior, not
implementation. No scenarios here; lens B owns them.>

**Terminal consumer**: <file:line, or "new, introduced by this change">

## MODIFIED Requirements

### Requirement: <Existing Requirement Name>

<Full updated requirement text — replaces the live one entirely.>
(Previously: <one line>)

**Live block**: <file:line, and scenario count>

## Open Questions

- [ ] <unresolved question, or "None">
```

Omit any of the four classification sections that has no entries. This change has no REMOVED or
RENAMED requirements per the proposal — omit both sections unless your own reading of the proposal
finds otherwise.

## Size budget

`spec-lens-a.md` MUST be under 1000 words. Requirement text is terse — one or two sentences with
an RFC 2119 keyword.

## Out of scope

Owned by the sibling lenses. Do NOT write these:

- **Lens B owns**: every `#### Scenario:` block, in Given/When/Then form, and the
  happy-path/edge-case coverage argument.
- **Lens C owns**: conflicts against live specs, the full-block copy of each MODIFIED requirement,
  and any Migration note.

Do NOT create or write any file under
`openspec/changes/skill-provisioning-and-phase-specialist/specs/`. That tree belongs to the
synthesizer.

## Allowed paths

`openspec/changes/skill-provisioning-and-phase-specialist/spec-lens-a.md` only. Create no other
file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-spec/` — the real `gentle-ai` spec skill and its
`references/`. Precedence is **not symmetric**: the skill wins on *what a delta spec must contain*
(ADDED/MODIFIED/REMOVED/RENAMED format, RFC 2119 rule, one-scenario-minimum rule, MODIFIED
copy-full-then-edit workflow). This packet wins on *how this phase is executed here* (three-lane
split, this lens's slice, word budget, skeleton, done criteria). Note any conflict in
`## Open Questions`, follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row
per unique citation, grouped by file, files alphabetical, ascending within each file.

| citation | claim |
|---|---|
| `openspec/specs/lane-execution/spec.md:1` | lane-execution has a live spec today, confirming it is a Modified, not New, capability |

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file.**
- [ ] **Every capability-map row is classified new or existing**, and every "existing" row cites
  the live spec with `file:line` that resolves in this worktree.
- [ ] **Every requirement carries an RFC 2119 keyword** and states observable behavior rather than
  implementation.
- [ ] **`spec-lens-a.md` exists, is under 1000 words, and carries `## Assumed requirements`,
  `## Capability Map`, and `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**
  (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- The proposal's Capabilities section and the actual `openspec/specs/` index disagree about
  whether a capability is New or Modified.
- A requirement cannot be stated without deciding an implementation detail the design phase has
  not decided.
- Satisfying one instruction in this packet would require violating another.

## Context

**Ground truth — cite it, do not re-derive it. Verified directly in this worktree before this
packet was authored:**

- The proposal's Capabilities section names exactly these eight capabilities:
  **New** — `skill-derivation`, `skill-root-resolution`, `skill-load-correspondence`,
  `phase-specialist-dispatch`. **Modified** — `packet-authoring-contract`, `lane-execution`,
  `acceptance-verifier`, `read-only-packet-schema`.
- Directory listing of `openspec/specs/` at this exact commit confirms: `acceptance-verifier`,
  `lane-execution`, `packet-authoring-contract`, and `read-only-packet-schema` all already exist
  as live spec directories — these four are genuinely **Modified**, not New, matching the
  proposal. `skill-derivation`, `skill-root-resolution`, `skill-load-correspondence`, and
  `phase-specialist-dispatch` do **not** exist anywhere under `openspec/specs/` — these four are
  genuinely **New**, matching the proposal. This was confirmed by direct listing, not by trusting
  the proposal's own claim.
- The proposal's own `## Delta Specifications` section already drafts five requirement stubs with
  RFC 2119 language (`Deterministic multi-tier derivation`, `Root resolution and fail-closed
  admission`, `Contract extension and rendered delivery`, `Closed-set lane_role`, `Demotion and
  acceptance correspondence`, `Specialist sequencing`) — six total. Verify each maps to exactly one
  of the eight capabilities above; the proposal's own capability table (`## User and Capability
  Impact`) already pairs five of these with a capability and an existing seam citation — reuse
  those pairings only after confirming each cited seam still resolves in this worktree.
- `internal/skillcontent/skillcontent.go:1-28` and `:73-100` (confirmed by direct read) already
  exist as production code today implementing content hashing — but its role in this change is
  **observation only, never a requirement-enforcing gate**, per the proposal's own explicit
  framing. Do not write a requirement that makes `HashDir`'s output authoritative for admission.

**Decided already — do not re-litigate:** six-item scope ships as one PR (`size:exception`
pre-accepted, `openspec/config.yaml:6-7`); no SQLite migration; `AuthoringEvidence` version never
changes; specialist-side skill selection is out of scope.

**Out of scope, and including any of it is wrong:** any capability named `internal/ledger/`
migration, specialist tool access, wrapping or replacing gentle-ai, authoring skill content itself.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
