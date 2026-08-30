---
id: spec-agentic-phase-specialist-lens-a
executor: agy
routed_by: capabilities and requirements lens of the three-lens spec fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/agentic-phase-specialist/spec-lens-a.md"]
---

# Packet spec-agentic-phase-specialist-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/spec-agentic-phase-specialist-lens-a  ·  **Branch:** lucind/spec-agentic-phase-specialist-lens-a

## Goal

Produce `openspec/changes/agentic-phase-specialist/spec-lens-a.md`: the capability-to-file map for the **Agentic Phase Specialist** change, and every requirement statement it introduces or changes, each classified `ADDED` / `MODIFIED` / `REMOVED` / `RENAMED`.

This is one of three parallel spec lenses. It is feedstock for a synthesis lane, not the final delta spec. Do not write anything under `openspec/changes/agentic-phase-specialist/specs/`.

## Why this is safe to dispatch now

The proposal for `agentic-phase-specialist` is accepted and frozen. Lens B and lens C run in parallel against the same frozen inputs and write to different files, so no lane races another. This lens owns the requirement set; the other two write scenarios for it and check it against live specs.

## Preconditions

- `openspec/changes/agentic-phase-specialist/proposal.md` exists and is accepted.
- `openspec/changes/agentic-phase-specialist/spec-lens-a.md` does not yet exist.
- `openspec/specs/` exists with `phase-specialist-dispatch/`, `acceptance-verifier/`, and `sdd-planning-fan-out/` capabilities already present.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to *which* requirements exist and *what they say* — not to scenarios and not to migration:

1. The real `gentle-ai` spec skill (delivered under `## Required skills`). It is the phase contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/agentic-phase-specialist/proposal.md` in full — its `## Capabilities`, `## Delta Specifications`, and `## User and Capability Impact` sections are the primary contract. It already names one new capability (`phase-verdict-reporting`) and three modified capabilities (`phase-specialist-dispatch`, `acceptance-verifier`, `sdd-planning-fan-out`) with 4 draft requirements — treat the proposal's `## Delta Specifications` as a strong starting draft to verify and refine, not something to re-derive from zero.
3. `openspec/specs/phase-specialist-dispatch/spec.md`, `openspec/specs/acceptance-verifier/spec.md`, `openspec/specs/sdd-planning-fan-out/spec.md` — the live specs the three modified capabilities delta against.
4. `openspec/changes/archive/2026-08-29-skill-provisioning-and-phase-specialist/specs/` for precedent delta spec structure in this repository.

Never guess at a capability name. A capability you cannot cite in the proposal or in `openspec/specs/` does not exist yet, and saying so is the useful answer.

## Known correction to make (do not silently repeat the proposal's ambiguity)

The proposal's first delta requirement text says: *"The Specialist MUST execute `lucind-ai accept` for Lanes in its assigned phase without human confirmation."* This conflates decision *authority* with command *execution*: per the accepted proposal's own Out of Scope, `sdd-*` subagents have no Bash/Agent tool and cannot execute any CLI command — the Orchestrator performs the mechanical `lucind-ai accept` invocation. Rewrite this requirement as an **authority** requirement: the Specialist MUST independently *decide* Acceptance for Lanes in its assigned phase without human confirmation, and MUST direct the Orchestrator to execute the corresponding `lucind-ai accept`/`lucind-ai run` invocation on its decision. Do not phrase it as the Specialist itself running a command it structurally cannot run.

## Output format

Write exactly this skeleton to `openspec/changes/agentic-phase-specialist/spec-lens-a.md`:

```markdown
# Spec Lens A — Capabilities & Requirements: Agentic Phase Specialist

## Assumed requirements

<2-4 sentences naming the requirement set you are specifying: which capabilities
this change touches, how many requirements each gets, and whether each capability
is new (full spec) or existing (delta). Lens B and lens C write this same block
independently; the synthesizer compares all three. Be specific enough that a
disagreement is visible.>

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|

## ADDED Requirements

### Requirement: <Name>

<Requirement text with RFC 2119 keyword. State observable behavior, not implementation.>

**Terminal consumer**: <file:line, or "new, introduced by this change">

## MODIFIED Requirements

### Requirement: <Existing Requirement Name>

<The full updated requirement text — it replaces the live one entirely.>
(Previously: <one line on what changed>)

**Live block**: <file:line of the requirement in `openspec/specs/`, and scenario count>

## REMOVED Requirements

### Requirement: <Name>

(Reason: <why>)
(Migration: <what replaces it, or "None">)

## RENAMED Requirements

### Requirement: <Old Name> → <New Name>

(Reason: <why>)

## Open Questions

- [ ] <unresolved question, or "None">
```

Omit any of the four classification sections that has no entries. Do not write an empty heading.

## Size budget

`spec-lens-a.md` MUST be under 1000 words. Requirement text is terse by nature. If the requirement set does not fit, say so in `## Open Questions` rather than truncating silently.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens B owns**: every `#### Scenario:` block, in Given/When/Then form, and the happy-path/edge-case coverage argument.
- **Lens C owns**: conflicts against live specs, the full-block copy of each MODIFIED requirement, and every Migration note.

Do NOT create or write any file under `openspec/changes/agentic-phase-specialist/specs/`. That tree belongs to the synthesizer.

## Allowed paths

`openspec/changes/agentic-phase-specialist/spec-lens-a.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: The real `gentle-ai` spec skill and its `references/` (delivered under `## Required skills`). Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**. The skill is authority on *what a delta spec must contain*: the ADDED/MODIFIED/REMOVED/RENAMED format, the RFC 2119 rule, the "every requirement has at least one scenario" rule, and the MODIFIED copy-full-then-edit workflow. Where this packet paraphrases and drifts, the skill wins; note the drift in `## Open Questions`.

This packet is authority on *how this phase is executed here*: three parallel lanes, this lane's slice, word budget, output path, out-of-scope list, done criteria. The skill describes one sub-agent writing the whole delta spec tree itself — that is superseded here. Do not correct yourself toward it; note the conflict in `## Open Questions` and follow this packet.

Write nothing outside this repository.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` cited, one row per unique citation, grouped by file, files alphabetical, line numbers ascending. The claim is what YOU assert that range shows, stated plainly. This does not count against the word budget. The manifest is a worklist for the synthesizer, not a certificate of correctness.

## Mechanical self-check (REQUIRED — replaces narrating these facts)

**Before you commit**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/spec-lens-a.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Assumed requirements" --require-section "Capability Map" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/spec-lens-a.md
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` reported no FAIL against this draft's own manifest.**
- [ ] **Every capability-map row is classified new or existing**, and every "existing" row cites the live spec with `file:line` that resolves in this worktree.
- [ ] **Every requirement carries an RFC 2119 keyword** and states observable behavior.
- [ ] **The Specialist Acceptance requirement is phrased as decision authority, not literal command execution** (see "Known correction" above).
- [ ] **`spec-lens-a.md` exists, is under 1000 words excluding the Citation Manifest, and carries `## Assumed requirements`, `## Capability Map`, and `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution.** Strip any injected `Co-authored-by:` trailer.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The proposal has no Capabilities section and no Affected Areas section to infer one from.
- A capability the proposal names as Modified has no live spec in `openspec/specs/`.
- A requirement cannot be stated without deciding an implementation detail the design phase has not decided.
- Satisfying one instruction in this packet would require violating another.

## Context

**Change**: `agentic-phase-specialist`, proposal accepted (`openspec/changes/agentic-phase-specialist/proposal.md`). Chosen candidate: Phase-Scoped Agentic Specialist — existing `sdd-*` subagents administer their phase's lucind-ai fan-out+synthesis, independently accept that phase's Lanes, return only a Phase Verdict. Promotion stays human-confirmed.

**Proposal's Capabilities section verbatim**:
- New: `phase-verdict-reporting` — structured Phase Verdict from Specialist to Orchestrator.
- Modified: `phase-specialist-dispatch` (agentic Specialist is decision-maker; Go adapter stays status/eligibility/dispatch tool, `openspec/specs/phase-specialist-dispatch/spec.md:9-11`); `acceptance-verifier` (phase-gate `lucind-checks.sh`; Acceptance still must not mutate refs or invoke Promotion, `openspec/specs/acceptance-verifier/spec.md:30-33,124-127`); `sdd-planning-fan-out` (synthesis review and contradiction arbitration move to the Specialist, `openspec/specs/sdd-planning-fan-out/spec.md:9-12`).

**Proposal's own draft delta requirements** (verify, correct per "Known correction" above, and refine — do not blindly copy): "Specialist Phase Acceptance and Authority Carve-Out", "Structured Phase Verdict Reporting", "SDD Phase-Gated Verification Check Execution", "Specialist-Owned Synthesis Arbitration". Full text is in `proposal.md` under `## Delta Specifications`.

**Ground-truth code citations already verified by prior lanes**: `internal/accept/accept.go:84-96,120-137` (LaneMetadata load inside `AuthoringEvidenceVersion` branch, unconditional check calls); `internal/run/attempt.go:431-435`; `internal/ledger/lanes_meta.go:20-47` (SDDPhase at :25); `internal/integrate/integrate.go:159-200` (`Check` stays an ungated primitive per proposal's Out of Scope); `plugin/claude-code/skills/lucind-ai/SKILL.md:19` and OpenCode mirror; `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48`; `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:31-36,38-43,44-50`; `CONTEXT.md:51-53,91-93,103-109`.

**Do not relitigate**: Promotion stays human-confirmed and forbidden to every Specialist; `sdd-*` have no Bash/Agent tool (Orchestrator dispatches mechanically); the deterministic `internal/phasespec.Adapter` remains a callable tool, not replaced.

## Required skills

- sdd-spec

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
