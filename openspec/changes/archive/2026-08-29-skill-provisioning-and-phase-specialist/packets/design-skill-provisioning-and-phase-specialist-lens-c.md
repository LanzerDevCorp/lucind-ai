---
id: design-skill-provisioning-and-phase-specialist-lens-c
executor: agy
routed_by: failure-test-rollback lens of the three-lens design fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/skill-provisioning-and-phase-specialist/design-lens-c.md"]
---

# Packet design-skill-provisioning-and-phase-specialist-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/design-skill-provisioning-and-phase-specialist-lens-c  ·  **Branch:** lucind/design-skill-provisioning-and-phase-specialist-lens-c

## Goal

Produce `openspec/changes/skill-provisioning-and-phase-specialist/design-lens-c.md`: how this
change is tested, which seams already exist to test through, the applicability-driven threat
matrix, and the rollback/additivity decision. This lens MUST make a final, concrete decision on
Open Question 2 (see `## Context`) — not another punt.

This is one of three parallel design lenses. It is feedstock for a synthesis lane, not the final
design document. Do not write a complete `design.md`.

## Why this is safe to dispatch now

`proposal.md` is accepted, frozen, and already committed on this branch. Lens A and lens B run in
parallel against the same frozen input and write to different files, so no lane races another.

Lens A owns the architecture decisions and is running concurrently, so you do not have its final
text. Declare the architecture you are assuming in `## Assumed architecture` and design against it
consistently. The synthesizer arbitrates divergence; a silent second architecture does not survive
that arbitration.

## Preconditions

- `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md` exists and is accepted.
- `openspec/changes/skill-provisioning-and-phase-specialist/design-lens-c.md` does not yet exist.
- The threat-matrix reference table is embedded verbatim in this packet's `## Context`.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-design/SKILL.md` — the real `gentle-ai` design skill, plus
   `references/threat-matrix.md` behind it. The embedded copy in `## Context` is the frozen
   evidence; the reference is the authority. Report any drift between them.
2. `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md` (full) — Selected
   Candidate items 5 and 6, the Rollback Plan section, the Test and Validation Impact table, and
   Open Question 2.
3. `internal/accept/accept_test.go` and `internal/packet/packet_test.go` (both, in full) — how
   this repository actually tests frontmatter parsing and acceptance today: what it asserts on,
   what it fakes.
4. `internal/accept/authoring_evidence_test.go:56-127` (the mutation-case pattern this change must
   extend with a `required_skills` case).
5. `internal/run/run.go:876-904` (`enforceAllowedPaths`) as the shape `enforceRequiredSkills` must
   follow — same demotion pattern, different check.
6. `internal/skillcontent/skillcontent.go` (full — `HashDir`, and its doc comment's incident
   writeup on why content hashing must never be a blocking gate).

Never guess at a test seam. A seam you cannot cite does not exist yet, and saying so is the useful
answer.

## Output format

Write exactly this skeleton to
`openspec/changes/skill-provisioning-and-phase-specialist/design-lens-c.md`:

```markdown
# Design Lens C — Failure, Test & Rollback: Skill Provisioning and the SDD Phase Specialist

## Assumed architecture

<2-4 sentences. Lens A and lens B write this same block independently against
their own slices; the synthesizer compares all three for contradiction.>

## Decision 1 — Missing archive/ultrafixer child skills (resolves Open
Question 2)

**Choice**: <final: create stub `sdd-archive`/ultrafixer-role skills, or declare
those roles derived-empty and let admission pass with zero required skills for
them>
**Alternatives considered**: <the other option, and its operational risk>
**Rationale**: <grounded in whether admission's fail-closed rule
(`admitDispatchBatch`) would reject a real archive/ultrafixer dispatch today if
you chose "stub required">
**Terminal consumer**: <file:line>

## Testing Strategy

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|

<Unit / Integration / E2E. The seam column cites the injection point, or states
"new seam required".>

## Test Seams

<What is injectable or fakeable today, and what this change would have to add.>

## Threat Matrix

<The table from `## Context`, every row marked `Applicable` or `N/A: <reason>`.
For every applicable row: expected safe behavior, expected failure behavior, and
the concrete RED test that proves it.>

## Rollback and Additivity

**Choice**: <what reverting looks like — stop rendering, stop deriving, revert
enforcement, revert envelope/schema, revert new packages, in what order>
**Alternatives considered**: <a different reversal strategy, if any is plausible>
**Rationale**: <grounded in what the format deltas actually move>

## Out of Scope

<Adjacent work this change explicitly does not do.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`design-lens-c.md` MUST be under 1000 words. Tables over prose. Threat-matrix rows count toward the
budget — keep reasons to one clause.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the
synthesizer and wastes the lane:

- **Lens A owns**: the three-tier derivation architecture, resolution order, budget shape, and
  specialist CLI shape (Open Questions 1, 3, 4).
- **Lens B owns**: `lane_role` closed-set values, `## Required skills` rendered format, envelope
  `skills_loaded` schema delta, and Open Question 5.

Rollback is yours even though it is shaped like an architecture decision. Everything else shaped
like one is lens A's or lens B's.

## Allowed paths

`openspec/changes/skill-provisioning-and-phase-specialist/design-lens-c.md` only. Create no other
file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-design/` — the real `gentle-ai` design skill and its
`references/`, including `references/threat-matrix.md`. Precedence is **not symmetric**: the skill
wins on *what a design document must contain*. This packet wins on *how this phase is executed
here* (capability split, this lens's slice, word budget, skeleton, done criteria). The skill's
800-word budget, Engram persistence step, and phase-summary return block are superseded here; note
the conflict in `## Open Questions`, follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row
per unique citation, grouped by file, files alphabetical, ascending within each file. The claim is
what YOU assert that range shows. This is the synthesizer's worklist, not a certificate.

| citation | claim |
|---|---|
| `internal/run/run.go:876-904` | enforceAllowedPaths is the exact demotion shape (real git diff inspection, lane.Deviated on shortfall) enforceRequiredSkills must follow |

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file.**
- [ ] **Decision 1 states a FINAL choice** (resolving Open Question 2) — no "either could work."
- [ ] **Every named test seam carries a `file:line` citation** that points at real code in this
  worktree, or is explicitly marked "new seam required".
- [ ] **Every threat-matrix row is marked `Applicable` or `N/A` with a reason**, and every
  applicable row names a planned RED test.
- [ ] **`design-lens-c.md` exists, is under 1000 words, and carries every skeleton section.**
- [ ] **The work is committed with a conventional commit and no AI attribution**
  (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- A behavior the proposal requires cannot be tested through any existing or proposed seam.
- Whether a format delta is additive cannot be determined from the proposal, so the rollback
  decision would be a guess.
- The threat matrix is missing from both `## Context` and the skill reference.
- Satisfying one instruction in this packet would require violating another.

## Context

### Threat-matrix reference table

<Embed the applicability-driven threat matrix verbatim here — boundary rows for
documentation-like paths, git repository selection, commit state, push state, and
PR commands, with the "Applicable / N/A: reason" column. Source it from
`~/.claude/skills/sdd-design/references/threat-matrix.md`; the lane must not read
it only from the skill reference above. This change's own most relevant boundary
is fail-closed admission (an unresolvable required skill rejecting a batch
before worktree allocation) and demotion-on-shortfall (a declared but undelivered
skill demoting a lane to `deviated`) — both are process-integration-adjacent
boundaries the matrix's generic rows may map onto as "Applicable" once you read
the actual reference file.>

### Ground truth

**Verified directly in this worktree before this packet was authored:**

- `internal/accept/authoring_evidence_test.go:56-127` already contains the mutation-case pattern
  (mutate one field of frozen evidence, assert `accept` rejects it) this change must extend with a
  `required_skills` shortfall case — confirmed present at this range by direct read.
- `enforceAllowedPaths` (`internal/run/run.go:876-904`, confirmed by direct read) demotes to
  `lane.Deviated` by inspecting the **actual git diff** of the worktree (via
  `candidatechange.Collect`), not by trusting the envelope. `enforceRequiredSkills` cannot use the
  same git-diff mechanism — a declared skill is not a file path — so it must instead compare the
  packet's derived required-skill set against the envelope's `skills_loaded` field. State this
  distinction explicitly in Testing Strategy.
- `internal/skillcontent/skillcontent.go:1-28`'s package doc (confirmed by direct read) is a
  first-person incident writeup: an earlier content-hash check that WAS wired into `go test`/`make
  install` forced a shared-file version-bump race across concurrently active isolated feature
  branches, which in turn triggered overlap-required reconciliation gates with no support for 3+
  simultaneous overlaps. This is the concrete, already-lived failure mode that makes the
  proposal's "external skill hash is observation only, never a gate"
  (`skillcontent.go:1-28`, `HashDir` at `:73-100`) a load-bearing constraint, not a style choice —
  the threat matrix's rollback-relevant row should cite this precedent directly.
- No `openspec/changes/skill-provisioning-and-phase-specialist/specs/` tree exists yet in this
  worktree (specs is a concurrent sibling phase) — do not cite scenario numbers from it.

**Decided already — do not re-litigate:** six-item scope ships as one PR (`size:exception`
pre-accepted); no SQLite migration, no historical-row backfill; `AuthoringEvidence` version never
changes.

**Out of scope, and including any of it is wrong:** the derivation algorithm (lens A's),
`lane_role`/envelope surface deltas (lens B's), authoring skill content itself, CLI
failure-guidance banners.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
