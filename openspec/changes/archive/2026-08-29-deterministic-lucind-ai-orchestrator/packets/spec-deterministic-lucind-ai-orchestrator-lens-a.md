---
id: spec-deterministic-lucind-ai-orchestrator-lens-a
executor: agy
routed_by: capabilities and requirements lens of the three-lens spec fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/deterministic-lucind-ai-orchestrator/spec-lens-a.md"]
---

# Packet spec-deterministic-lucind-ai-orchestrator-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-deterministic-orchestrator-worktrees/spec-deterministic-lucind-ai-orchestrator-lens-a  ·  **Branch:** lucind/spec-deterministic-lucind-ai-orchestrator-lens-a

## Goal

Produce `openspec/changes/deterministic-lucind-ai-orchestrator/spec-lens-a.md`: the
capability-to-file map for this change's five capabilities, and every requirement statement it
introduces or changes, each classified `ADDED` / `MODIFIED` / `REMOVED` / `RENAMED`.

This is one of three parallel spec lenses. It is feedstock for a synthesis lane, not the final
delta spec. Do not write anything under `openspec/changes/deterministic-lucind-ai-orchestrator/specs/`.

## Why this is safe to dispatch now

`proposal.md` is accepted, frozen, and already committed on this branch. Lens B and lens C run in
parallel against the same frozen input and write to different files, so no lane races another.
This lens owns the requirement set; the other two write scenarios for it and check it against live
specs.

## Preconditions

- `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md` exists and is accepted.
- `openspec/changes/deterministic-lucind-ai-orchestrator/spec-lens-a.md` does not yet exist.
- `openspec/specs/` exists with the two live capabilities this change modifies
  (`sdd-apply`, `parent-feature-integration`).

## Required reading (this lens only)

1. `~/.claude/skills/sdd-spec/SKILL.md` — the real `gentle-ai` spec skill.
2. `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md` (full), and its
   **Capabilities** section in particular.
3. The index of `openspec/specs/` — confirm which of the five named capabilities (one New, four
   Modified) already have a live spec directory and which do not.
4. `openspec/changes/archive/` for a prior change that introduced a similarly multi-capability
   change, if one exists, to see this repository's requirement granularity.

Never guess at a capability name. A capability you cannot cite in the proposal or in
`openspec/specs/` does not exist yet, and saying so is the useful answer.

## Output format

Write exactly this skeleton to
`openspec/changes/deterministic-lucind-ai-orchestrator/spec-lens-a.md`:

```markdown
# Spec Lens A — Capabilities & Requirements: Deterministic lucind-ai Orchestrator

## Assumed requirements

<2-4 sentences naming the requirement set: five capabilities (one New, four
Modified), how many requirements each gets. Lens B and lens C write this same
block independently; the synthesizer compares all three. Be specific enough
that a disagreement is visible.>

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|

<One row per capability the proposal names under New/Modified Capabilities.
"New" rows target openspec/specs/<capability>/spec.md and cite nothing.
"Existing" rows target
openspec/changes/deterministic-lucind-ai-orchestrator/specs/<capability>/spec.md
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
`openspec/changes/deterministic-lucind-ai-orchestrator/specs/`. That tree belongs to the
synthesizer.

## Allowed paths

`openspec/changes/deterministic-lucind-ai-orchestrator/spec-lens-a.md` only. Create no other
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
| `openspec/specs/sdd-apply/spec.md:1` | sdd-apply has a live spec today, confirming it is a Modified, not New, capability |

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root twice.

**Before you commit**:

```
./lucind-lane-check.sh --file openspec/changes/deterministic-lucind-ai-orchestrator/spec-lens-a.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Assumed requirements" --require-section "Capability Map" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/deterministic-lucind-ai-orchestrator/spec-lens-a.md
```

Paste the report's PASS/FAIL lines into `done_criteria[].evidence` in your envelope instead of
narrating the same facts in prose.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL
  against this draft's own manifest.**
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

- The proposal's Capabilities section (`openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:21-28`,
  corrected in commit `55a1543` after a prior dispatch of this exact lens hard-stopped on a real
  baseline mismatch) names exactly these five capabilities:
  **New** — `deterministic-orchestrator-contract`, `packet-authoring-contract`, `acceptance-verifier`.
  **Modified** — `sdd-apply`, `parent-feature-integration`.
- Directory listing of `openspec/specs/` at this exact commit confirms: `parent-feature-integration`
  and `sdd-apply` already exist as live spec directories — genuinely **Modified**, matching the
  proposal. `deterministic-orchestrator-contract`, `packet-authoring-contract`, and
  `acceptance-verifier` do **not** exist anywhere under `openspec/specs/` on this Change's `main`
  base — all three are genuinely **New**. This was confirmed by direct listing, not by trusting the
  proposal's own claim.
- **Do not be misled by identically-named specs elsewhere in this repository.** `packet-authoring-contract`
  and `acceptance-verifier` DO exist as live specs on the unrelated, still in-flight
  `feature/skill-provisioning-and-phase-specialist` branch (639 commits ahead of this Change's
  `main` base) — that branch is a different, deliberately isolated Change. Citing its specs here
  would silently couple two Changes the human explicitly asked to keep separate. If your worktree
  somehow shows those two directories as already present under `openspec/specs/`, treat that as a
  worktree/target contamination hard stop, not as confirmation.
- The proposal's `## Approach` section (`proposal.md:30-34`) already frames the two-layer split:
  a prompt/reference layer authored only in `plugin/claude-code/skills/lucind-ai/` (with
  `plugin/opencode/skills/lucind-ai/` as a byte-identical, parity-verified copy — never a second
  contract), and a runtime layer of narrow enforcement/reporting at existing `cmd/lucind-ai`,
  `internal/packet`, `internal/run`, `internal/dag`, `internal/ledger`, `internal/accept`, and
  `internal/worktree` boundaries.

**Decided already — do not re-litigate:** no new lifecycle states, scheduler/wave engine, flags,
routing mechanism, or replacement for existing Combine/Resolve/Check/bisect/CAS primitives
(`proposal.md:14-15`); executor/model/provider/profile selection and semantic approval/promotion
remain human-owned (`proposal.md:16`); no cross-fork coordination, global state, automatic main
promotion, or redesign of unrelated SDD flows (`proposal.md:17`).

**Out of scope, and including any of it is wrong:** anything the proposal's `## Out of Scope`
section excludes (`proposal.md:14-17`).

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
