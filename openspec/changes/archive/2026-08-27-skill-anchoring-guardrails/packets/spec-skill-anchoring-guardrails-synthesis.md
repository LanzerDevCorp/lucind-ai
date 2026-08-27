---
id: spec-skill-anchoring-guardrails-synthesis
executor: agy
routed_by: synthesis of three parallel spec lenses into the canonical delta spec tree — executor overridden from the template default (cursor-agent) to agy per human-approved AGY-only Execution Strategy for Change skill-anchoring-guardrails; cursor-agent is reserved only for verify's second qualitative judge
allowed_paths: ["openspec/changes/skill-anchoring-guardrails/specs/", "openspec/changes/skill-anchoring-guardrails/spec-synthesis-notes.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: bfc7e7c7e89471fd2871b3f1d58410a3badba59a
expected_parent_sha: bfc7e7c7e89471fd2871b3f1d58410a3badba59a
---

# Packet spec-skill-anchoring-guardrails-synthesis

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/spec-skill-anchoring-guardrails-synthesis  ·  **Branch:** lucind/spec-skill-anchoring-guardrails-synthesis

## Goal

Read the three spec lens drafts for `skill-anchoring-guardrails`, verify their claims against the real code and the real live specs, arbitrate where they disagree, and produce the canonical delta spec tree under `openspec/changes/skill-anchoring-guardrails/specs/` plus a separate synthesis notes file.

You are the last judgment in this phase. Anything you accept without checking ships into `openspec/specs/` at archive time.

## Why this is safe to dispatch now

All three lens lanes reached terminal `done` status and integrated (integrated=6 alongside the design lenses, reverted=0). Lens C already audited all 24 live specs in `openspec/specs/` and found none cover worktree cleanup guardrails, failure guidance banners, or the TDD WIP-rescue protocol — treat this as strong evidence, not as license to skip your own citation verification pass.

## Preconditions

- `spec-lens-a.md`, `spec-lens-b.md`, and `spec-lens-c.md` all exist in this worktree.
- `openspec/changes/skill-anchoring-guardrails/specs/` does not yet exist.
- The proposal for `skill-anchoring-guardrails` is present and accepted.

## What each lens owns

| Draft | Owns |
|---|---|
| `spec-lens-a.md` | Capability map; requirement statements; ADDED / MODIFIED / REMOVED / RENAMED classification |
| `spec-lens-b.md` | Given/When/Then scenarios; coverage table; untestable assertions |
| `spec-lens-c.md` | Live-spec inventory and conflicts; verbatim MODIFIED full blocks (if any); removals, renames, consumers, migration |

All three also emit `## Open Questions`. Merge and deduplicate them.

## Required procedure

### 1. Read all three drafts in full

### 2. Citation verification pass

Every `file:line` citation is a claim about this repository. Lens C's live-spec citations matter most — verify its finding that no live spec covers these capabilities by independently checking `openspec/specs/lane-execution/spec.md` (and any other capability lens C names) yourself before accepting ADDED classification for all five capabilities.

### 3. Requirement arbitration

- **Lens A's requirement set is authoritative.**
- A scenario from lens B keyed to a requirement lens A did not name does not go into the delta. Record under `## Requirement Divergence`.
- If lens C's live-spec evidence contradicts lens A's ADDED/MODIFIED classification for any capability, lens C's evidence wins on classification (it is the lens that opened the live spec). Record the correction in the notes.
- If lens A's requirement set is refuted by a live spec you verified, do not silently substitute your own — that is a hard stop.

### 4. Assemble, do not concatenate

Write one file per capability at `openspec/changes/skill-anchoring-guardrails/specs/<capability>/spec.md`, using the delta format: `## ADDED Requirements`, `## MODIFIED Requirements`, `## REMOVED Requirements`, `## RENAMED Requirements`. Omit any section with no entries — if all three lenses confirm no live spec exists for any of the five capabilities, every file will carry only `## ADDED Requirements`.

### 5. Budget

The authored content MUST stay under 1800 words, excluding any scenarios copied verbatim from a live spec inside a genuine MODIFIED block.

### 6. Coverage check

1. Every capability the proposal names has a file, at the right path for new versus existing
2. Every requirement classified `ADDED` / `MODIFIED` / `REMOVED` / `RENAMED`
3. Every requirement text carries an RFC 2119 keyword
4. Every requirement carries at least one scenario
5. Scenarios cover happy path and edge cases, GIVEN / WHEN / THEN
6. Every genuine `MODIFIED` block is the complete live block, edited — never partial
7. Every `REMOVED`/`RENAMED` requirement carries a Reason and Migration where consumers exist
8. No implementation detail — WHAT, not HOW

Anything no draft covered goes under `## Coverage Gaps`.

## Output

### `openspec/changes/skill-anchoring-guardrails/specs/<capability>/spec.md`

One file per capability: `worktree-dirty-guardrail`, `failure-guidance-banners`, `tdd-wip-rescue-protocol`, and the two capabilities the proposal called "Modified" (`lane-worktree-lifecycle`, `worktree-cleanup-cli`) — reclassify these to ADDED unless your own verification finds a live spec lens C missed.

### `openspec/changes/skill-anchoring-guardrails/spec-synthesis-notes.md`

```markdown
# Spec Synthesis Notes: Skill Anchoring & Worktree Cleanup Guardrails

## Unresolved Contradictions

<"None" if there are none.>

## Coverage Gaps

<"None" if there are none.>

## Dropped Citations

<"None" if there are none.>

## Requirement Divergence

<Explicitly record whether "Modified Capabilities" from the proposal were reclassified to ADDED here, and why. "None — all three converged" only if no reclassification was needed.>
```

## Out of scope

- Do NOT modify the three lens drafts.
- Do NOT write into `openspec/specs/`. Archive merges the delta there.
- Do NOT write design, tasks, or any implementation code.
- Do NOT resolve an unresolved contradiction by choosing.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`.

## Allowed paths

`openspec/changes/skill-anchoring-guardrails/specs/` and `openspec/changes/skill-anchoring-guardrails/spec-synthesis-notes.md` only.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-spec/` — the real `gentle-ai` spec skill. This packet sets the 1800-word authored budget and its verbatim-block exclusion, the synthesis procedure, the notes file, and the done criteria.

Write nothing outside this repository.

## Using the lens citation manifests

Treat the union of the three manifests as your **verification worklist, never as evidence**. Open every cited range yourself and check it against the real code AND the real live specs in this worktree. Record each outcome in `## Dropped Citations`.

## Commit discipline (REQUIRED — two commits, not one)

**Commit the delta spec tree the moment it is written, before you begin the notes file.** Then write the notes and commit them as a second conventional commit. Both commits are conventional, with no AI attribution. Strip any injected `Co-authored-by:` trailer.

## Mechanical self-check (REQUIRED)

**Right after the first commit:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/specs/worktree-dirty-guardrail/spec.md --skip-result
```

**After the second commit and writing `.lucind/result.json`:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/spec-synthesis-notes.md \
  --require-section "Unresolved Contradictions" --require-section "Coverage Gaps" \
  --require-section "Dropped Citations" --require-section "Requirement Divergence"
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence`.

## Done criteria

- [ ] **The delta spec tree was committed as its own commit before `spec-synthesis-notes.md` was started**, confirmed by the mid-flow `lucind-lane-check.sh` run reporting a clean `git status --porcelain`.
- [ ] **Every `file:line` citation surviving into the delta tree was opened and confirmed in this worktree**, and every dropped claim is listed under `## Dropped Citations`.
- [ ] **Every requirement carries an RFC 2119 keyword and at least one GIVEN / WHEN / THEN scenario.**
- [ ] **The delta tree's authored content is under 1800 words excluding verbatim copied blocks**, and every spine item is satisfied or reported under `## Coverage Gaps`.
- [ ] **`spec-synthesis-notes.md` exists with exactly the four required sections**, each either populated or explicitly "None".
- [ ] **The work is committed with two conventional commits and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`.

## Hard stops

- The three `## Assumed requirements` blocks are mutually irreconcilable and the proposal does not choose between them. Write the notes file, leave `specs/` uncreated, and block.
- Lens A's requirement set is refuted by a live spec you verified. Do not substitute your own.
- A `MODIFIED` requirement's live block cannot be recovered, so the block would have to be partial. Never write a partial block.
- One or more lens drafts is missing from this worktree.
- Satisfying one instruction in this packet would require violating another.

## Context

Change: **skill-anchoring-guardrails**. Design synthesis (running concurrently as a sibling packet in this same wave, on the same base) is arbitrating the architecture in parallel and does not affect your requirement arbitration. Execution: Isolated Mode, `agy`-only executor except verify's second qualitative judge (kept on `cursor-agent`) — already decided, do not re-litigate.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, report the counts that matter: citations verified, citations dropped, MODIFIED blocks confirmed complete, contradictions escalated, coverage gaps. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
