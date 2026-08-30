---
id: spec-agentic-phase-specialist-synthesis
executor: cursor-agent
routed_by: synthesis of three parallel spec lenses into the canonical delta spec tree
model: cursor-grok-4.6-high
allowed_paths: ["openspec/changes/agentic-phase-specialist/specs", "openspec/changes/agentic-phase-specialist/spec-synthesis-notes.md"]
---

# Packet spec-agentic-phase-specialist-synthesis

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/spec-agentic-phase-specialist-synthesis  ·  **Branch:** lucind/spec-agentic-phase-specialist-synthesis

## Goal

Read the three spec lens drafts for `agentic-phase-specialist`, verify their claims against the real code and the real live specs, arbitrate where they disagree, and produce the canonical delta spec tree under `openspec/changes/agentic-phase-specialist/specs/` plus a separate synthesis notes file.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you — the phase-Specialist (`sdd-spec`) and the Orchestrator read only your notes file.

## Why this is safe to dispatch now

All three lens lanes have reached terminal status and integrated. This worktree is branched from the integrated result, so `spec-lens-a.md`, `spec-lens-b.md`, and `spec-lens-c.md` are all present here.

## Preconditions

- `spec-lens-a.md`, `spec-lens-b.md`, and `spec-lens-c.md` all exist in this worktree.
- `openspec/changes/agentic-phase-specialist/specs/` does not yet exist.
- `openspec/changes/agentic-phase-specialist/proposal.md` is present and accepted.

## What each lens owns

| Draft | Owns |
|---|---|
| `spec-lens-a.md` | Capability map; requirement statements; ADDED/MODIFIED/REMOVED/RENAMED classification |
| `spec-lens-b.md` | Given/When/Then scenarios; coverage table; untestable assertions |
| `spec-lens-c.md` | Live-spec inventory and conflicts; verbatim MODIFIED full blocks; removals, renames, consumers, migration |

All three also emit `## Open Questions`. Merge and deduplicate them.

## Required procedure

### 1. Read all three drafts in full

### 2. Citation verification pass

Every `file:line` citation is a claim about this repository. Open each one in this worktree. A citation that does not resolve or does not support the claim: drop the claim, record under `## Dropped Citations`.

### 3. Requirement arbitration

- **Lens A's requirement set is authoritative.**
- A scenario keyed to a requirement lens A did not name does not go into the delta — record under `## Requirement Divergence`.
- A conflict lens C found against a requirement lens A classified `ADDED` means the classification is wrong: it is `MODIFIED`. Lens C's live-spec evidence wins on classification.
- If lens A's own requirement set is refuted by a live spec you verified, do not silently substitute your own — that is a hard stop.
- **Specifically resolve the Specialist Acceptance authority-vs-execution phrasing**: all three lenses were instructed to correct the proposal's ambiguous "Specialist MUST execute `lucind-ai accept`" into an authority requirement (Specialist decides; Orchestrator executes mechanically, since `sdd-*` has no Bash). Confirm every surviving lens phrased it this way; if any reverted to literal execution language, correct it in the canonical spec and note the correction under `## Requirement Divergence`.

### 4. Assemble, do not concatenate

One file per capability at `openspec/changes/agentic-phase-specialist/specs/<capability>/spec.md`: `phase-verdict-reporting/spec.md` (new, full spec — no ADDED/MODIFIED framing needed since the whole capability is new, but still use the delta section headers per the skill's format for a new capability), `phase-specialist-dispatch/spec.md` (delta), `acceptance-verifier/spec.md` (delta), `sdd-planning-fan-out/spec.md` (delta). Follow the delta format: `## ADDED Requirements`, `## MODIFIED Requirements`, `## REMOVED Requirements`, `## RENAMED Requirements`, omitting empty sections.

Never ship a partial MODIFIED block — each starts as lens C's verbatim full block, edited to the new behavior, with `(Previously: <one line>)`.

### 5. Budget

Authored content of the delta tree MUST stay under 1800 words, excluding scenarios copied verbatim from a live spec inside a MODIFIED block.

### 6. Coverage check

1. Every capability the proposal names has a file, at the right path for new versus existing.
2. Every requirement classified ADDED/MODIFIED/REMOVED/RENAMED.
3. Every requirement text carries an RFC 2119 keyword.
4. Every requirement carries at least one scenario.
5. Scenarios cover happy path and edge cases.
6. Every MODIFIED block is the complete live block, edited.
7. No implementation detail — WHAT, not HOW.

Anything no draft covered goes under `## Coverage Gaps`.

## Output

### `openspec/changes/agentic-phase-specialist/specs/<capability>/spec.md` (four files, one per capability)

### `openspec/changes/agentic-phase-specialist/spec-synthesis-notes.md`

Exactly these four sections, in this order:

```markdown
# Spec Synthesis Notes: Agentic Phase Specialist

## Unresolved Contradictions

<"None" if there are none.>

## Coverage Gaps

<"None" if there are none.>

## Dropped Citations

<"None" if there are none.>

## Requirement Divergence

<Include, explicitly, whether the Specialist-Acceptance authority-vs-execution
phrasing needed correction in any lens, and what the canonical spec says now.
"None — all three converged" if that is the case.>
```

## Out of scope

- Do NOT modify the three lens drafts.
- Do NOT write into `openspec/specs/` (the live tree) — archive merges the delta there later.
- Do NOT write design, tasks, or implementation code.
- Do NOT resolve an unresolved contradiction by choosing.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`.

## Allowed paths

`openspec/changes/agentic-phase-specialist/specs/` and `openspec/changes/agentic-phase-specialist/spec-synthesis-notes.md` only.

## Allowed paths outside the repository

**Read-only**: The real `gentle-ai` spec skill (delivered under `## Required skills`). It wins on ADDED/MODIFIED/REMOVED/RENAMED format, RFC 2119, one-scenario-minimum, and the MODIFIED copy-full-then-edit workflow. This packet sets the 1800-word authored budget, the verbatim-block exclusion, the synthesis procedure, the notes file, and done criteria.

Write nothing outside this repository.

## Using the lens citation manifests

Treat the union of the three manifests as your verification worklist, never as evidence. Open every cited range yourself. Deduplicate across the three; batch by file; verify the claim, not just the line's existence. Record outcomes in `## Dropped Citations`.

## Commit discipline (REQUIRED — two commits, not one)

Commit the delta spec tree the moment it is written, before starting the notes file. Then commit the notes as a second conventional commit. No AI attribution; strip any injected `Co-authored-by:` trailer.

## Mechanical self-check (REQUIRED — replaces narrating these facts)

**Right after the first commit**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/specs/phase-verdict-reporting/spec.md --skip-result
```

**After the second commit and writing `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/spec-synthesis-notes.md \
  --require-section "Unresolved Contradictions" --require-section "Coverage Gaps" \
  --require-section "Dropped Citations" --require-section "Requirement Divergence"
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence`.

## Done criteria

- [ ] **The delta spec tree was committed as its own commit before `spec-synthesis-notes.md` was started.**
- [ ] **Every `file:line` citation surviving into the delta tree was opened and confirmed**, dropped claims listed.
- [ ] **Every MODIFIED block matches the live requirement scenario for scenario**, with only intended edits.
- [ ] **Every requirement carries an RFC 2119 keyword and at least one scenario.**
- [ ] **The Specialist Acceptance requirement is phrased as decision authority, not literal command execution, in the canonical spec.**
- [ ] **The delta tree's authored content is under 1800 words excluding verbatim copied blocks.**
- [ ] **`spec-synthesis-notes.md` exists with exactly the four required sections.**
- [ ] **The work is committed with two conventional commits and no AI attribution.**

## Hard stops

- The three `## Assumed requirements` blocks are mutually irreconcilable and the proposal does not choose.
- Lens A's requirement set is refuted by a live spec you verified.
- A MODIFIED requirement's live block cannot be recovered, so the block would have to be partial.
- One or more lens drafts is missing.
- Satisfying the spine honestly would require exceeding the authored budget.
- Satisfying one instruction in this packet would require violating another.

## Context

**Change**: `agentic-phase-specialist` — Phase-Scoped Agentic Specialist. Four capabilities: `phase-verdict-reporting` (new), `phase-specialist-dispatch`, `acceptance-verifier`, `sdd-planning-fan-out` (all modified). Live specs at `openspec/specs/phase-specialist-dispatch/spec.md`, `openspec/specs/acceptance-verifier/spec.md`, `openspec/specs/sdd-planning-fan-out/spec.md`.

**Human decision already made (do not treat as open)**: the Specialist Acceptance requirement is authority, not literal command execution — `sdd-*` subagents have no Bash/Agent tool; the Orchestrator performs the mechanical `lucind-ai accept`/`run` invocation on the Specialist's decision. All three lenses were instructed on this; verify it survived.

## Required skills

- sdd-spec

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, report the counts that matter. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
