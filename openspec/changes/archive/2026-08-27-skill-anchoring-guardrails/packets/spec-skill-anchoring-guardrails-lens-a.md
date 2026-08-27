---
id: spec-skill-anchoring-guardrails-lens-a
executor: agy
routed_by: capabilities and requirements lens of the three-lens spec fan-out for Change skill-anchoring-guardrails
allowed_paths: ["openspec/changes/skill-anchoring-guardrails/spec-lens-a.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: f5a531183361804ed95c797e16a70dbbcca27763
expected_parent_sha: f5a531183361804ed95c797e16a70dbbcca27763
---

# Packet spec-skill-anchoring-guardrails-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/spec-skill-anchoring-guardrails-lens-a  ·  **Branch:** lucind/spec-skill-anchoring-guardrails-lens-a

## Goal

Produce `openspec/changes/skill-anchoring-guardrails/spec-lens-a.md`: the capability-to-file map for this change, and every requirement statement it introduces or changes, each classified `ADDED` / `MODIFIED` / `REMOVED` / `RENAMED`.

This is one of three parallel spec lenses. It is feedstock for a synthesis lane, not the final delta spec. Do not write anything under `openspec/changes/skill-anchoring-guardrails/specs/`.

## Why this is safe to dispatch now

`proposal.md` is accepted and frozen, and already contains a **Capabilities** section (3 New, 2 Modified) plus a fully drafted **Delta Specifications** section with 6 requirements. Lens B and lens C run in parallel against the same frozen inputs and write to different files. This lens owns the requirement set.

## Preconditions

- `openspec/changes/skill-anchoring-guardrails/proposal.md` exists and is accepted.
- `openspec/changes/skill-anchoring-guardrails/spec-lens-a.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-spec/SKILL.md` — the real `gentle-ai` spec skill.
2. `openspec/changes/skill-anchoring-guardrails/proposal.md`, its **Capabilities** section and its **Delta Specifications** section in full — the proposal already drafted six requirements; your job is to independently re-verify and formally classify them, not invent new ones, unless the proposal's own content demands a different split.
3. `ls openspec/specs/` — the proposal claims two **Modified Capabilities**: `lane-worktree-lifecycle` and `worktree-cleanup-cli`. **Neither name appears in the current `openspec/specs/` directory listing** (`acceptance-verifier`, `allowed-paths-enforcement`, `apply-dag-dispatch`, `completion-mode-enforcement`, `conflict-fixture`, `conflict-triage`, `defect-records`, `dependencies-defects`, `dispatched-packet-body`, `lane-approval-wait`, `lane-execution`, `lane-progress-telemetry`, `orphan-lane-reconciliation`, `parent-feature-integration`, `read-only-done-criterion`, `read-only-packet-schema`, `reconciliation-approval`, `sdd-apply`, `sdd-planning-fan-out`, `triage-evaluation-rubric`, `ultrafixer-dispatch`, `verify-dual-dispatch`, `verify-judgment-packet`, `verify-mechanical-check`). Check whether worktree/CLI-cleanup behavior is covered under one of these existing names (e.g. `lane-execution`) instead. If no live spec covers it under any name, these two capabilities are actually **New**, not **Modified** — say so; lens C independently verifies this same question from the live-spec side.
4. `openspec/changes/archive/` for prior delta spec precedent in this repository.

Never guess at a capability name. A capability you cannot cite in the proposal or in `openspec/specs/` does not exist yet, and saying so is the useful answer.

## Output format

Write exactly this skeleton to `openspec/changes/skill-anchoring-guardrails/spec-lens-a.md`:

```markdown
# Spec Lens A — Capabilities & Requirements: Skill Anchoring & Worktree Cleanup Guardrails

## Assumed requirements

<2–4 sentences naming the requirement set: 3 candidate New capabilities (worktree-dirty-guardrail, failure-guidance-banners, tdd-wip-rescue-protocol) and 2 candidate Modified capabilities (lane-worktree-lifecycle, worktree-cleanup-cli) per the proposal — state your own finding on whether the two "Modified" ones actually have a live spec to modify.>

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|

## ADDED Requirements

### Requirement: <Name>

<text with RFC 2119 keyword>

**Terminal consumer**: <file:line or "new, introduced by this change">

## MODIFIED Requirements

### Requirement: <Existing Requirement Name>

<full updated text>
(Previously: <one line>)

**Live block**: <file:line, scenario count — ONLY if a live spec genuinely exists>

## Open Questions

- [ ] <unresolved question, or "None">
```

Omit REMOVED/RENAMED sections — the proposal names no removals or renames. Omit MODIFIED Requirements entirely if your capability-map finding is that both candidate "Modified" capabilities are actually New.

## Size budget

`spec-lens-a.md` MUST be under 1000 words.

## Out of scope

- **Lens B owns**: every `#### Scenario:` block and the coverage argument.
- **Lens C owns**: conflicts against live specs, the full-block copy of each MODIFIED requirement, and every Migration note.

Do NOT create or write any file under `openspec/changes/skill-anchoring-guardrails/specs/`. That tree belongs to the synthesizer.

## Allowed paths

`openspec/changes/skill-anchoring-guardrails/spec-lens-a.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-spec/` — the real `gentle-ai` spec skill and its `references/`. The skill is authority on *what* a delta spec must contain; this packet is authority on *how this phase is executed here* — superseded on purpose where they conflict; note conflicts in `## Open Questions`.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

| citation | claim |
|---|---|
| `path/to/example_file.ext:12-34` | <YOUR OWN real citation and claim from THIS draft> |

## Mechanical self-check (REQUIRED)

**Before you commit:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/spec-lens-a.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Assumed requirements" --require-section "Capability Map" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/spec-lens-a.md
```

Paste both PASS/FAIL reports into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every capability-map row is classified new or existing**, and every "existing" row cites the live spec with `file:line` that resolves in this worktree.
- [ ] **Every requirement carries an RFC 2119 keyword** and states observable behavior rather than implementation.
- [ ] **`spec-lens-a.md` exists, is under 1000 words excluding the Citation Manifest, and carries `## Assumed requirements`, `## Capability Map`, and `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Strip any injected `Co-authored-by:` trailer.

## Hard stops

- The proposal has no Capabilities section and no Affected Areas section to infer one from.
- A requirement cannot be stated without deciding an implementation detail the design phase has not decided.
- Satisfying one instruction in this packet would require violating another.

## Context

Change: **skill-anchoring-guardrails**. Accepted proposal's Capabilities section: New = `worktree-dirty-guardrail`, `failure-guidance-banners`, `tdd-wip-rescue-protocol`; Modified = `lane-worktree-lifecycle`, `worktree-cleanup-cli` (verify these two actually have a live spec; `openspec/specs/` listing above shows neither name present). The proposal's own Delta Specifications section already drafted 6 requirements: worktree cleanup dirty guardrail + force flag; blocked/timeout report banner; integration report reverted-IDs banner; acceptance receipt qualitative-review banner; DAG split multi-wave base-SHA banner; TDD WIP-rescue protocol documentation. Execution: Isolated Mode, `agy`-only executor except verify's second qualitative judge (kept on `cursor-agent`) — already decided, do not re-litigate.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
