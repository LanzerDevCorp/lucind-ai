---
id: design-agentic-phase-specialist-lens-a
executor: agy
routed_by: decisions lens of the three-lens design fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/agentic-phase-specialist/design-lens-a.md"]
---

# Packet design-agentic-phase-specialist-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/design-agentic-phase-specialist-lens-a  ·  **Branch:** lucind/design-agentic-phase-specialist-lens-a

## Goal

Produce `openspec/changes/agentic-phase-specialist/design-lens-a.md`: the technical approach and every architecture decision for the **Agentic Phase Specialist** change, each with choice, alternatives rejected, rationale, and terminal consumer.

This is one of three parallel design lenses. It is feedstock for a synthesis lane, not the final design document. Do not write a complete `design.md`.

## Why this is safe to dispatch now

The proposal and specs for `agentic-phase-specialist` are accepted and frozen. Lens B and lens C run in parallel against the same frozen inputs and write to different files, so no lane races another.

## Preconditions

- `openspec/changes/agentic-phase-specialist/proposal.md` exists and is accepted.
- `openspec/changes/agentic-phase-specialist/specs/` exists (four capability files).
- `openspec/changes/agentic-phase-specialist/design-lens-a.md` does not yet exist.

## Required reading (this lens only)

1. The real `gentle-ai` design skill (delivered under `## Required skills`).
2. `openspec/changes/agentic-phase-specialist/proposal.md` in full, and `openspec/changes/agentic-phase-specialist/specs/` (all four capability spec files).
3. `internal/phasespec/phasespec.go` (`Adapter`, `CLIStatusQuerier` at lines ~308-350) and `cmd/lucind-ai/cli.go:2517-2649` (`phaseDispatch`) — the existing deterministic tool this design must show as still-callable, not replaced.
4. `internal/accept/accept.go:84-137`, `internal/run/attempt.go:431-435`, `internal/ledger/lanes_meta.go:20-47` — the exact call sites the check-gating decision touches.
5. `openspec/changes/archive/2026-08-29-skill-provisioning-and-phase-specialist/design.md` for precedent on how this repository designed the original (deterministic) Specialist — a structurally similar problem, now being superseded.

## Decision this lens must resolve explicitly (from the accepted proposal's Open Questions and the propose-phase divergence notes)

The proposal's design-relevant Open Question: *"Should the Phase Verdict be a JSON schema under `internal/result/` or a structured markdown section returned to the Orchestrator?"* — this lens owns architecture decisions, so **decide this** (or state why it must stay open for tasks/apply) rather than leaving it unresolved, since it directly shapes the interface between Specialist and Orchestrator. Ground the choice in `internal/result/` conventions if such a package exists, or explain why a markdown convention is sufficient given the Specialist has no code to change (it is an existing Claude Code subagent's *behavior*, not new Go code, in the near term per the proposal's Out of Scope on tool grants).

## Output format

Write exactly this skeleton to `openspec/changes/agentic-phase-specialist/design-lens-a.md`:

```markdown
# Design Lens A — Decisions: Agentic Phase Specialist

## Assumed architecture

<2-4 sentences naming the structural shape: what Go code changes (accept.go,
attempt.go check-gating), what changes only outside Go code (SKILL.md, fan-out.md,
sdd-* subagent behavior contracts), and what stays exactly as-is (internal/phasespec.Adapter,
integrate.Check). Lens B and lens C write this same block independently.>

## Technical Approach

<Concise strategy. How it maps to the proposal and the four spec capabilities.>

## Decision 1 — <title>

**Choice**: <what we chose>
**Alternatives considered**: <what we rejected>
**Rationale**: <why, grounded in this repository's code>
**Terminal consumer**: <file:line, or spec requirement>

## Decision N — <title>

<same four fields — MUST include a decision resolving the Phase Verdict shape (JSON vs markdown), a decision on the check-gate's exact insertion point in accept.go/attempt.go, and a decision on the Hard Rule carve-out's exact wording scope (named sdd-* Specialists only, own-phase Lanes only)>

## Open Questions

- [ ] <unresolved technical question, or "None">
```

## Size budget

`design-lens-a.md` MUST be under 1000 words.

## Out of scope

Owned by the sibling lenses:

- **Lens B owns**: the file-changes table, data-flow diagrams, invariants, exact type/schema/CLI signature deltas.
- **Lens C owns**: testing strategy, test seams, threat matrix, rollback/additivity.

Do not write a rollback decision here.

## Allowed paths

`openspec/changes/agentic-phase-specialist/design-lens-a.md` only.

## Allowed paths outside the repository

**Read-only**: The real `gentle-ai` design skill and its `references/` (delivered under `## Required skills`). Not symmetric: the skill governs *what* a design document must contain (required sections, choice/alternatives/rationale shape, threat-matrix applicability rule); this packet governs *how this phase is executed here* (three lanes, this slice, budget, path, done criteria, and this repository's actual 1800-word canonical budget rather than the skill's nominal 800). Note conflicts in `## Open Questions`.

Write nothing outside this repository.

## Citation manifest (REQUIRED — excluded from the word budget)

One row per unique citation, grouped by file, ascending line numbers, plainly stated claim.

## Mechanical self-check (REQUIRED — replaces narrating these facts)

**Before you commit**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/design-lens-a.md --budget 1000 \
  --exclude-section "Citation Manifest" --require-section "Assumed architecture" \
  --require-section "Technical Approach" --require-section "Open Questions" \
  --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/design-lens-a.md
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` reported no FAIL against this draft's own manifest.**
- [ ] **Every decision names a terminal consumer with a `file:line` citation.**
- [ ] **The Phase Verdict shape decision (JSON vs markdown) is explicitly resolved or explicitly deferred with a reason.**
- [ ] **`design-lens-a.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section.**
- [ ] **The work is committed with a conventional commit and no AI attribution.**

## Hard stops

- The proposal or specs do not determine an architectural choice, and two reasonable shapes are equally supported.
- A decision cannot name any terminal consumer.
- Satisfying one instruction in this packet would require violating another.

## Context

**Change**: `agentic-phase-specialist` — Phase-Scoped Agentic Specialist. Proposal's own Selected Candidate & Approach (verbatim from `proposal.md`): (1) Phase Verdict — outcome/artifact-path/unresolved-divergence, raw evidence stays with Specialist; needs-revision triggers one bounded correction. (2) Tool-constrained dispatch — Specialist authors packets and judges Acceptance; Orchestrator runs `lucind-ai run`/`accept`; `internal/phasespec.Adapter` and `CLIStatusQuerier` remain the tool. (3) Scoped checks — load `LaneMetadata` unconditionally in `accept.go`, skip `CheckPolicySnapshot`/`v.check` unless `SDDPhase == "apply"`, empty/missing, or explicit exception; same gate in `attempt.go`. (4) Fan-out dogfooding — synthesis starts only after all lens receipts exist.

**Ground-truth citations**: `internal/accept/accept.go:84-96` (LaneMetadata load inside `AuthoringEvidenceVersion` branch), `accept.go:120-137` (unconditional check calls), `accept.go:97-98,214-261` (scope validation, unaffected); `internal/run/attempt.go:431-435`; `internal/ledger/lanes_meta.go:20-47`; `internal/integrate/integrate.go:159-200` (`Check` stays an ungated primitive); `internal/phasespec/phasespec.go:308-350`; `cmd/lucind-ai/cli.go:2517-2649`; `plugin/claude-code/skills/lucind-ai/SKILL.md:19` (Hard Rule, both trees); `fan-out.md:47-48`; `acceptance-promotion.md:31-50`.

**Do not relitigate**: Promotion stays human-confirmed and forbidden to Specialists; `internal/ledger/authoring.go:14,26` (`AuthoringEvidenceVersion`) and SQLite schema are explicitly out of scope — no migration.

## Required skills

- sdd-design

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
