---
id: spec-agentic-phase-specialist-lens-b
executor: agy
routed_by: scenarios and coverage lens of the three-lens spec fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/agentic-phase-specialist/spec-lens-b.md"]
---

# Packet spec-agentic-phase-specialist-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/spec-agentic-phase-specialist-lens-b  ·  **Branch:** lucind/spec-agentic-phase-specialist-lens-b

## Goal

Produce `openspec/changes/agentic-phase-specialist/spec-lens-b.md`: a Given/When/Then scenario set for every requirement this change introduces or changes, plus the coverage argument.

This is one of three parallel spec lenses. It is feedstock for a synthesis lane, not the final delta spec. Do not write anything under `openspec/changes/agentic-phase-specialist/specs/`.

## Why this is safe to dispatch now

The proposal for `agentic-phase-specialist` is accepted and frozen. Lens A and lens C run in parallel against the same frozen inputs and write to different files, so no lane races another.

Lens A owns the requirement set and is running concurrently, so you do not have it. Derive the requirements you are writing scenarios for from the proposal itself, declare them in `## Assumed requirements`, and key every scenario to one of them by name.

## Preconditions

- `openspec/changes/agentic-phase-specialist/proposal.md` exists and is accepted.
- `openspec/changes/agentic-phase-specialist/spec-lens-b.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to *proof of behavior*:

1. The real `gentle-ai` spec skill (delivered under `## Required skills`). Read it rather than trusting this packet's paraphrase.
2. `openspec/changes/agentic-phase-specialist/proposal.md` in full — its `## Delta Specifications` section already has 4 requirements with draft scenarios; use those as your starting point, but write your own `### Requirement:` headings matching the names as the proposal states them, and extend coverage (edge cases, error states) beyond the proposal's happy-path scenarios where the coverage table would otherwise show a gap.
3. Two or three archived delta specs under `openspec/changes/archive/*/specs/` for scenario granularity precedent.
4. `internal/accept/accept.go:84-137`, `internal/run/attempt.go:431-435`, `internal/ledger/lanes_meta.go:20-47` — the code paths for the check-gating requirement, so your scenario THENs name observable outcomes (e.g., "the check function is invoked" vs "the check function is skipped"), not vague behavior.

Never invent a state the system cannot be in.

## Output format

Write exactly this skeleton to `openspec/changes/agentic-phase-specialist/spec-lens-b.md`:

```markdown
# Spec Lens B — Scenarios & Coverage: Agentic Phase Specialist

## Assumed requirements

<2-4 sentences naming the requirement set: Specialist Phase Acceptance and Authority
Carve-Out; Structured Phase Verdict Reporting; SDD Phase-Gated Verification Check
Execution; Specialist-Owned Synthesis Arbitration. Lens A and lens C write this same
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

`spec-lens-b.md` MUST be under 1000 words.

## Out of scope

Owned by the sibling lenses. Do NOT write these:

- **Lens A owns**: the capability map, requirement statements, and classification.
- **Lens C owns**: conflicts against live specs, full-block copy of MODIFIED requirements, Migration notes.

Do NOT create or write any file under `openspec/changes/agentic-phase-specialist/specs/`.

## Allowed paths

`openspec/changes/agentic-phase-specialist/spec-lens-b.md` only.

## Allowed paths outside the repository

**Read-only**: The real `gentle-ai` spec skill and its `references/` (delivered under `## Required skills`). Not symmetric with this packet: the skill governs *what* a delta spec must contain (Given/When/Then format, one-scenario-minimum, "specs describe WHAT not HOW"); this packet governs *how this phase is executed here* (three lanes, this slice, budget, path, done criteria). Where the skill describes one sub-agent writing the whole tree, that is superseded; note conflicts in `## Open Questions`.

Write nothing outside this repository.

## Citation manifest (REQUIRED — excluded from the word budget)

Same rules as every lens in this fan-out: one row per unique citation, grouped by file, ascending line numbers, the claim you assert plainly. Worklist for the synthesizer, not a certificate.

## Mechanical self-check (REQUIRED — replaces narrating these facts)

**Before you commit**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/spec-lens-b.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Assumed requirements" --require-section "Scenarios" \
  --require-section "Coverage" --require-section "Untestable Assertions" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/spec-lens-b.md
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` reported no FAIL against this draft's own manifest.**
- [ ] **Every requirement named in `## Assumed requirements` has at least one scenario**, in GIVEN/WHEN/THEN form.
- [ ] **Every scenario's THEN names an observable outcome**, and the coverage table cites the seam or marks "new seam required".
- [ ] **`spec-lens-b.md` exists, is under 1000 words excluding the Citation Manifest, and carries `## Assumed requirements`, `## Coverage`, `## Untestable Assertions`, and `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution.** Strip any injected `Co-authored-by:` trailer.

## Hard stops

- The proposal does not determine what the system should do in a case.
- Every scenario for a requirement would be untestable.
- Satisfying one instruction in this packet would require violating another.

## Context

**Change**: `agentic-phase-specialist`, proposal accepted. Same four requirements as lens A/C: Specialist Phase Acceptance and Authority Carve-Out (phrase as decision *authority*, not literal command execution — `sdd-*` have no Bash and cannot run `lucind-ai accept` themselves; the Orchestrator executes on the Specialist's decision); Structured Phase Verdict Reporting (outcome/artifact-path/unresolved-divergence, no raw evidence); SDD Phase-Gated Verification Check Execution (run `lucind-checks.sh` for `sdd_phase == "apply"`, empty/missing `sdd_phase`, or explicit exception; skip otherwise); Specialist-Owned Synthesis Arbitration (Specialist reads synthesis notes and arbitrates, not the Orchestrator).

**Ground-truth seams for observable THENs**: `internal/accept/accept.go:84-137` (metadata load and check invocation call sites); `internal/run/attempt.go:431-435` (`checkFunc` invocation); `internal/ledger/lanes_meta.go:20-47` (`SDDPhase` field); `internal/integrate/integrate.go:159-200` (`Check` — stays ungated, called by the gated sites); `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48` (synthesis-note review responsibility).

**Do not relitigate**: Promotion stays human-confirmed; `internal/phasespec.Adapter` remains the callable dispatch/status tool.

## Required skills

- sdd-spec

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
