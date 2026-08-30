---
id: spec-skill-provisioning-and-phase-specialist-lens-c
executor: agy
routed_by: live-spec conflict and migration lens of the three-lens spec fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/skill-provisioning-and-phase-specialist/spec-lens-c.md"]
---

# Packet spec-skill-provisioning-and-phase-specialist-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/spec-skill-provisioning-and-phase-specialist-lens-c  ·  **Branch:** lucind/spec-skill-provisioning-and-phase-specialist-lens-c

## Goal

Produce `openspec/changes/skill-provisioning-and-phase-specialist/spec-lens-c.md`: what this
change collides with in the four live capabilities it modifies
(`packet-authoring-contract`, `lane-execution`, `acceptance-verifier`, `read-only-packet-schema`),
the verbatim full block of every requirement it modifies, and migration guidance for anything it
removes or renames.

This is one of three parallel spec lenses. It is feedstock for a synthesis lane, not the final
delta spec. Do not write anything under `openspec/changes/skill-provisioning-and-phase-specialist/specs/`.

## Why this is safe to dispatch now

`proposal.md` is accepted, frozen, and already committed on this branch. Lens A and lens B run in
parallel against the same frozen input and write to different files, so no lane races another.

Lens A owns the requirement set and is running concurrently, so you do not have it. Derive the
requirements you are checking from the proposal itself, declare them in `## Assumed requirements`,
and key every finding to one of them by name. The synthesizer arbitrates divergence — **but your
live-spec evidence outranks lens A's classification when they conflict**: if you find a real
collision with a live requirement lens A called `ADDED`, the true classification is `MODIFIED`,
and your evidence wins.

## Why this lens exists

Archive replaces a live requirement with whatever the MODIFIED block says. A partial MODIFIED
block silently deletes every scenario it failed to copy, and nothing catches it until the
capability is already wrong in `openspec/specs/`. This lens is the lane that opens the live spec
and copies the whole block forward.

## Preconditions

- `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md` exists and is accepted.
- `openspec/changes/skill-provisioning-and-phase-specialist/spec-lens-c.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-spec/SKILL.md` — the real `gentle-ai` spec skill, and the **MODIFIED
   Requirements Workflow** section in particular.
2. `openspec/specs/packet-authoring-contract/spec.md`, `openspec/specs/lane-execution/spec.md`,
   `openspec/specs/acceptance-verifier/spec.md`, and `openspec/specs/read-only-packet-schema/spec.md`
   **each in full** — not the index, not a grep. You cannot report what a change collides with from
   a search result.
3. `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md` (full), for what the
   change intends to do to each of the four.
4. Consumers of anything you find no longer accurate after this change: tests, docs, other specs
   that reference the same requirement by name. Cite each with `file:line`.

Never claim a live requirement says something without opening it. This lens is the only lane that
reads the live specs in full; a wrong claim here is not caught downstream.

## Output format

Write exactly this skeleton to
`openspec/changes/skill-provisioning-and-phase-specialist/spec-lens-c.md`:

```markdown
# Spec Lens C — Live-Spec Conflicts & Migration: Skill Provisioning and the SDD Phase Specialist

## Assumed requirements

<2-4 sentences naming the requirement set you are checking against live specs.
Lens A and lens B write this same block independently; the synthesizer compares
all three.>

## Live Spec Inventory

| Capability | Live spec (file:line) | Requirements | Scenarios | Touched by this change |
|---|---|---|---|---|

<One row per one of the four modified capabilities. Counts come from opening the
file, not from estimating.>

## Conflicts

<Every place this change contradicts a live requirement rather than extending
it. "None" if there are none.>

## MODIFIED Full Blocks

### Requirement: <Live Requirement Name>

**Source**: `openspec/specs/<capability>/spec.md:<line>` — <N> scenarios

<The COMPLETE live block, copied verbatim. Do not summarize, do not elide a
scenario.>

## Removals and Renames

| Requirement | Removed or renamed | Reason | Consumers (file:line) | Migration |
|---|---|---|---|---|

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`spec-lens-c.md` MUST be under 1000 words **excluding the verbatim blocks under
`## MODIFIED Full Blocks`**. Those blocks are copied evidence, not authored prose.

## Out of scope

Owned by the sibling lenses. Do NOT write these:

- **Lens A owns**: the capability map, new requirement statements, and ADDED classification for
  the four New capabilities.
- **Lens B owns**: every new `#### Scenario:` block and the coverage argument.

The scenarios inside a `## MODIFIED Full Blocks` entry are yours (copied evidence from the live
spec). Any scenario that does not already exist in `openspec/specs/` is lens B's.

Do NOT create or write any file under
`openspec/changes/skill-provisioning-and-phase-specialist/specs/`. That tree belongs to the
synthesizer.

## Allowed paths

`openspec/changes/skill-provisioning-and-phase-specialist/spec-lens-c.md` only. Create no other
file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-spec/` — the real `gentle-ai` spec skill and its
`references/`. Precedence is **not symmetric**: the skill wins on *what a delta spec must contain*
(MODIFIED copy-full-then-edit workflow, REMOVED Reason-and-Migration rule). This packet wins on
*how this phase is executed here*. Note any conflict in `## Open Questions`, follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row
per unique citation, grouped by file, files alphabetical, ascending within each file.

| citation | claim |
|---|---|
| `openspec/specs/read-only-packet-schema/spec.md:84` | "Extended packet frontmatter parsing" requirement exists at this line, a plausible collision point for the new lane_role key |

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file.**
- [ ] **Every one of the four modified capabilities was opened in full**, and its inventory row's
  requirement and scenario counts came from the file rather than an estimate.
- [ ] **Every `## MODIFIED Full Blocks` entry is the complete live block**, scenario for scenario,
  nothing summarized or elided.
- [ ] **Every removal or rename (if any) names its consumers with `file:line`** and carries a
  Reason.
- [ ] **`spec-lens-c.md` exists, is under 1000 words excluding verbatim blocks, and carries
  `## Assumed requirements`, `## Live Spec Inventory`, and `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**
  (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- One of the four capabilities has no live spec to read (contradicts what this packet's `## Context`
  already confirmed — treat as a real anomaly, not a reason to skip verifying).
- A requirement being removed has consumers the proposal never mentions.
- Copying a MODIFIED block whole would exceed what you can write, so the copy would have to be
  partial. Report which requirement forces it; never write a partial block.
- Satisfying one instruction in this packet would require violating another.

## Context

**Ground truth — cite it, do not re-derive it. Verified directly in this worktree before this
packet was authored:**

- All four live spec files exist and were confirmed present at this exact commit by direct
  listing: `openspec/specs/acceptance-verifier/spec.md` (139 lines, 8 requirements —
  `Exact Acceptance Binding` `:9`, `Fail-Closed Mechanical Criteria` `:30`, `Frozen Candidate
  Verification` `:60`, `Owned Isolation and Cleanup` `:70`, `Durable Receipt and Exact Cache Reuse`
  `:86`, `Receipt-Gated CLI Success` `:103`, `No Promotion Authority` `:119`, `Mechanical Evidence
  Is Not Semantic Approval` `:130`); `openspec/specs/lane-execution/spec.md` (121 lines, 6
  requirements — `Gate Placement in the Lifecycle` `:10`, `Resolve Before Barrier Observation`
  `:27`, `Additive Schema, Unchanged Enum` `:44`, `Lane metadata dispatch persistence` `:63`,
  `Universal Pre-Dispatch Packet Admission` `:85`, `Frozen Authored Candidate Evidence` `:104`);
  `openspec/specs/packet-authoring-contract/spec.md` (73 lines, 4 requirements —
  `Versioned Contract and Late Target Binding` `:9`, `Deterministic Rendering and Digest` `:28`,
  `Universal Admission and Manual Compatibility` `:42`, `Versioned Result Correspondence` `:61`);
  `openspec/specs/read-only-packet-schema/spec.md` (137 lines, 8 requirements —
  `Frontmatter Read-Only Field Parsing` `:9`, `Default Value and Backward Compatibility` `:28`,
  `Explicit Flag Only — No Inference` `:47`, `The Envelope Cannot Declare or Override Mode` `:61`,
  `Additive Rollback` `:75`, `Extended packet frontmatter parsing` `:84`, `Read-Only Input Path
  Preservation and Visibility` `:106`, `Read-Only Path Validation` `:125`). These heading positions
  were confirmed by direct grep of `### Requirement:` lines in each file; treat them as a map to
  navigate from, not as a substitute for opening each file in full.
- These exact heading names are plausible collision candidates, based on their names alone — this
  is a hint, not a finding; only opening and reading each one settles it: `Universal Admission and
  Manual Compatibility` and `Versioned Result Correspondence` (both in
  `packet-authoring-contract`, since this change adds `LaneRole`/`RequiredSkills` to the contract
  and `skills_loaded` to the result correspondence); `Universal Pre-Dispatch Packet Admission` and
  `Frozen Authored Candidate Evidence` (both in `lane-execution`, since `admitDispatchBatch` gains
  a new fail-closed check and lane metadata gains dispatch persistence for the derived set);
  `Fail-Closed Mechanical Criteria` and `Frozen Candidate Verification` (both in
  `acceptance-verifier`, since acceptance must reject a `required_skills` shortfall); `Extended
  packet frontmatter parsing` (in `read-only-packet-schema`, the most likely home for a new
  `lane_role` closed-set frontmatter key, given its name already covers frontmatter extension).
  Confirm or refute every one of these five by opening the file — do not assume the name predicts
  the content.

**Decided already — do not re-litigate:** six-item scope ships as one PR (`size:exception`
pre-accepted); `AuthoringEvidence` shape and version never change; no SQLite migration; the
proposal names no capability removal or rename anywhere in its `## Delta Specifications` or
`## User and Capability Impact` sections — if your own reading of the four live specs finds a real
removal or rename this change forces, that is new information the proposal missed; report it, do
not suppress it.

**Out of scope, and including any of it is wrong:** new requirement text for the four New
capabilities (lens A's), new scenarios (lens B's), any capability outside the eight the proposal
names.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
