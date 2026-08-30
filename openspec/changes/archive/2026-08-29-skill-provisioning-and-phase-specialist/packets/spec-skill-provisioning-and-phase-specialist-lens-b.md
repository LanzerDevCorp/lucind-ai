---
id: spec-skill-provisioning-and-phase-specialist-lens-b
executor: agy
routed_by: scenarios and coverage lens of the three-lens spec fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/skill-provisioning-and-phase-specialist/spec-lens-b.md"]
---

# Packet spec-skill-provisioning-and-phase-specialist-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/spec-skill-provisioning-and-phase-specialist-lens-b  ·  **Branch:** lucind/spec-skill-provisioning-and-phase-specialist-lens-b

## Goal

Produce `openspec/changes/skill-provisioning-and-phase-specialist/spec-lens-b.md`: a
Given/When/Then scenario set for every requirement this change introduces or changes, plus the
coverage argument that says which happy paths, edge cases, and error states are proven and which
are not.

This is one of three parallel spec lenses. It is feedstock for a synthesis lane, not the final
delta spec. Do not write anything under `openspec/changes/skill-provisioning-and-phase-specialist/specs/`.

## Why this is safe to dispatch now

`proposal.md` is accepted, frozen, and already committed on this branch. Lens A and lens C run in
parallel against the same frozen input and write to different files, so no lane races another.

Lens A owns the requirement set and is running concurrently, so you do not have it. Derive the
requirements you are writing scenarios for from the proposal itself, declare them in
`## Assumed requirements`, and key every scenario to one of them by name. The synthesizer
arbitrates divergence; scenarios keyed to a requirement nobody else named are dropped, so name
them the way the proposal does.

## Preconditions

- `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md` exists and is accepted.
- `openspec/changes/skill-provisioning-and-phase-specialist/spec-lens-b.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-spec/SKILL.md` — the real `gentle-ai` spec skill.
2. `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md` (full), and its
   `## Delta Specifications` section in particular — each requirement stub already carries a
   `#### Scenario:` sketch; expand each to full coverage rather than copying it verbatim.
3. Two or three archived delta specs under `openspec/changes/archive/*/specs/` — read how this
   repository actually writes a scenario: GIVEN granularity, whether THEN names a concrete value.
4. `internal/packet/packet_test.go` and `internal/accept/authoring_evidence_test.go` (both, in
   full) — the code paths a scenario's THEN must be observable through: parse errors, admission
   rejection, acceptance rejection.

Never invent a state the system cannot be in. A precondition you cannot reach is a scenario nobody
can write a test from.

## Output format

Write exactly this skeleton to
`openspec/changes/skill-provisioning-and-phase-specialist/spec-lens-b.md`:

```markdown
# Spec Lens B — Scenarios & Coverage: Skill Provisioning and the SDD Phase Specialist

## Assumed requirements

<2-4 sentences naming the requirement set. Lens A and lens C write this same
block independently; the synthesizer compares all three.>

## Scenarios

### Requirement: <Name, as the proposal names it>

#### Scenario: <Happy path>

- GIVEN <precondition>
- WHEN <action>
- THEN <observable outcome>

#### Scenario: <Edge case>

- GIVEN <precondition>
- WHEN <action>
- THEN <observable outcome>

#### Scenario: <Error state>

- GIVEN <precondition>
- WHEN <action>
- THEN <observable failure>

### Requirement: <Next name>

<same shape>

## Coverage

| Requirement | Happy path | Edge case | Error state | Testable through (file:line) |
|---|---|---|---|---|

## Untestable Assertions

<Every scenario you wanted to write but could not. "None" if there are none.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`spec-lens-b.md` MUST be under 1000 words. If the scenario set does not fit, cover every
requirement's happy path first, then edge cases, then error states.

## Out of scope

Owned by the sibling lenses. Do NOT write these:

- **Lens A owns**: the capability map, the requirement statements themselves, and their
  ADDED/MODIFIED classification.
- **Lens C owns**: conflicts against live specs, the full-block copy of each MODIFIED requirement,
  and any Migration note.

Do NOT create or write any file under
`openspec/changes/skill-provisioning-and-phase-specialist/specs/`. That tree belongs to the
synthesizer.

## Allowed paths

`openspec/changes/skill-provisioning-and-phase-specialist/spec-lens-b.md` only. Create no other
file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-spec/` — the real `gentle-ai` spec skill and its
`references/`. Precedence is **not symmetric**: the skill wins on *what a delta spec must contain*
(Given/When/Then format, one-scenario-minimum rule, "WHAT not HOW" rule). This packet wins on *how
this phase is executed here*. Note any conflict in `## Open Questions`, follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row
per unique citation, grouped by file, files alphabetical, ascending within each file.

| citation | claim |
|---|---|
| `internal/packet/packet.go:196-205` | Parse's final validation switch is the observable seam for a missing-required-field parse error |

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file.**
- [ ] **Every requirement named in `## Assumed requirements` has at least one scenario**, and every
  scenario is in GIVEN/WHEN/THEN form.
- [ ] **Every scenario's THEN names an observable outcome**, and the coverage table cites the seam
  it is observable through or marks it "new seam required".
- [ ] **`spec-lens-b.md` exists, is under 1000 words, and carries `## Assumed requirements`,
  `## Coverage`, `## Untestable Assertions`, and `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**
  (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- The proposal does not determine what the system should do in a case, so the scenario would
  assert an outcome nobody chose.
- Every scenario for a requirement would be untestable, meaning the requirement as proposed is
  unobservable.
- Satisfying one instruction in this packet would require violating another.

## Context

**Ground truth — cite it, do not re-derive it. Verified directly in this worktree before this
packet was authored:**

- The proposal's `## Delta Specifications` section already names six requirements with sketch
  scenarios: `Deterministic multi-tier derivation` (2 scenarios: "Planning lens set", "Over
  budget"), `Root resolution and fail-closed admission` (1: "Tilde expansion"), `Contract extension
  and rendered delivery` (1: "Hash stability"), `Closed-set lane_role` (1: "Unknown role
  rejected"), `Demotion and acceptance correspondence` (1: "Shortfall"), `Specialist sequencing`
  (1: "Fan-out then synthesis"). Each of these six already has at least a happy-path or error-state
  sketch — your job is to reach full happy/edge/error coverage for each, not merely restate the
  proposal's single sketch per requirement.
- `packet.Parse`'s final validation switch (`internal/packet/packet.go:196-205`, confirmed by
  direct read) is exactly: `ID == ""`, `Executor == ""`, `RoutedBy == ""`, empty body — each
  returns a named sentinel error. A `lane_role` unknown-value scenario's THEN is observable as one
  more case in this same shape, once design adds it (do not assume the exact error variable name;
  cite the switch's existence and shape, not an error name design has not yet chosen).
- `admitDispatchBatch` (`cmd/lucind-ai/packet_authoring.go:32-54`, confirmed by direct read) is a
  pre-worktree, pre-quota admission function — a "batch rejected before worktree allocation"
  scenario's THEN is observable as this function returning a non-nil error with no worktree ever
  created, not as a lane status.
- `internal/accept/authoring_evidence_test.go` (153 lines total, confirmed by direct read) already
  contains the mutation-case test pattern this change's "Shortfall" scenario extends.

**Decided already — do not re-litigate:** six-item scope ships as one PR (`size:exception`
pre-accepted); budget default is 3 (proposal item 1); the closed `lane_role` set is `{lens,
synthesis, apply, verify, archive, ultrafixer, human}` (proposal item 8).

**Out of scope, and including any of it is wrong:** requirement text itself (lens A's), live-spec
migration scenarios (lens C's), implementation-detail scenarios (e.g. asserting a specific Go
function name).

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
