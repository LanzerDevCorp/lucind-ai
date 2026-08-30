---
id: design-skill-anchoring-guardrails-synthesis
executor: agy
routed_by: synthesis of three parallel design lenses into one canonical design — executor overridden from the template default (cursor-agent) to agy per human-approved AGY-only Execution Strategy for Change skill-anchoring-guardrails; cursor-agent is reserved only for verify's second qualitative judge
allowed_paths: ["openspec/changes/skill-anchoring-guardrails/design.md", "openspec/changes/skill-anchoring-guardrails/design-synthesis-notes.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: bfc7e7c7e89471fd2871b3f1d58410a3badba59a
expected_parent_sha: bfc7e7c7e89471fd2871b3f1d58410a3badba59a
---

# Packet design-skill-anchoring-guardrails-synthesis

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/design-skill-anchoring-guardrails-synthesis  ·  **Branch:** lucind/design-skill-anchoring-guardrails-synthesis

## Goal

Read the three design lens drafts for `skill-anchoring-guardrails`, verify their claims against the real code, arbitrate where they disagree, and produce one canonical `openspec/changes/skill-anchoring-guardrails/design.md` plus a separate synthesis notes file recording everything that did not make it in and why.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you.

## Why this is safe to dispatch now

All three lens lanes (`design-skill-anchoring-guardrails-lens-a/b/c`) reached terminal `done` status and integrated (integrated=6 alongside the spec lenses, reverted=0). This packet's `base_sha` is refreshed to that integrated tip.

## Preconditions

- `design-lens-a.md`, `design-lens-b.md`, and `design-lens-c.md` all exist in this worktree.
- `openspec/changes/skill-anchoring-guardrails/design.md` does not yet exist.
- The proposal and specs for `skill-anchoring-guardrails` are present and accepted.

## What each lens owns

| Draft | Owns |
|---|---|
| `design-lens-a.md` | Technical approach; every architecture decision except rollback, with alternatives and rationale |
| `design-lens-b.md` | Flow and invariants; surface deltas (types, schemas, frontmatter, CLI); file changes |
| `design-lens-c.md` | Testing strategy and test seams; threat matrix; rollback and additivity; out of scope |

All three also emit `## Open Questions`. Merge and deduplicate them.

## Required procedure

### 1. Read all three drafts in full

### 2. Citation verification pass

Every `file:line` citation in every draft is a claim about this repository. Open each one in this worktree and confirm it. Drop and record under `## Dropped Citations` anything that does not resolve or support the claim.

### 3. Architecture arbitration

- **Lens A's assumed architecture is authoritative.**
- Any content in lens B or lens C that does not survive lens A's architecture does not go into `design.md`. Record under `## Architecture Divergence`.
- If lens B or lens C converged independently on lens A's architecture, say so — independent convergence is corroboration.
- If lens A's own architecture is refuted by code you verified in step 2, do not silently substitute your own — that is a hard stop.

### 4. Compress — do not concatenate

`design.md` MUST be under 1800 words.

### 5. Coverage check

`design.md` must substantively cover:

1. Technical approach or recommendations at a glance
2. Architecture decisions, each with choice / alternatives considered / rationale
3. Flow and invariants
4. File changes, with terminal consumers
5. Testing strategy and test seams
6. Threat matrix — every row `Applicable` or `N/A: reason`
7. Rollback and additivity
8. Open questions and out of scope

Anything no draft covered goes under `## Coverage Gaps`.

## Output

### `openspec/changes/skill-anchoring-guardrails/design.md`

The canonical design. Under 1800 words. Covers all eight spine items.

### `openspec/changes/skill-anchoring-guardrails/design-synthesis-notes.md`

```markdown
# Synthesis Notes: Skill Anchoring & Worktree Cleanup Guardrails

## Unresolved Contradictions

<"None" if there are none.>

## Coverage Gaps

<"None" if there are none.>

## Dropped Citations

<"None" if there are none.>

## Architecture Divergence

<"None — all three converged" if that is the case.>
```

## Out of scope

- Do NOT modify the three lens drafts.
- Do NOT write specs, tasks, or any implementation code.
- Do NOT resolve an unresolved contradiction by choosing.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`.

## Allowed paths

`openspec/changes/skill-anchoring-guardrails/design.md` and `openspec/changes/skill-anchoring-guardrails/design-synthesis-notes.md` only.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-design/` — the real `gentle-ai` design skill. This packet sets the 1800-word budget (not the skill's nominal 800), the synthesis procedure, the notes file, and the done criteria.

Write nothing outside this repository.

## Using the lens citation manifests

Treat the union of the three manifests as your **verification worklist, never as evidence**. Open every cited range yourself. Deduplicate across the three, batch by file, verify the claim not just the line's existence. Record each outcome in `## Dropped Citations`.

## Commit discipline (REQUIRED — two commits, not one)

**Commit `design.md` the moment it is written, before you begin the notes file.** Then write the notes and commit them as a second conventional commit. Both commits are conventional, with no AI attribution. Strip any injected `Co-authored-by:` trailer.

## Mechanical self-check (REQUIRED)

**Right after the first commit:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/design.md --budget 1800 --skip-result
```

**After the second commit and writing `.lucind/result.json`:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/design-synthesis-notes.md \
  --require-section "Unresolved Contradictions" --require-section "Coverage Gaps" \
  --require-section "Dropped Citations" --require-section "Architecture Divergence"
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence`.

## Done criteria

- [ ] **`design.md` was committed as its own commit before `design-synthesis-notes.md` was started**, confirmed by the mid-flow `lucind-lane-check.sh` run reporting a clean `git status --porcelain`.
- [ ] **Every `file:line` citation surviving into `design.md` was opened and confirmed in this worktree**, and every dropped claim is listed under `## Dropped Citations`.
- [ ] **`design.md` exists, is under 1800 words, and substantively covers all eight spine items**, with anything missing reported under `## Coverage Gaps`.
- [ ] **`design-synthesis-notes.md` exists with exactly the four required sections**, each either populated or explicitly "None".
- [ ] **The work is committed with two conventional commits and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`.

## Hard stops

- The three `## Assumed architecture` blocks are mutually irreconcilable and the proposal/specs do not choose between them. Write the notes file, leave `design.md` uncreated, and block.
- Lens A's architecture is refuted by code you verified. Do not substitute your own.
- One or more lens drafts is missing from this worktree.
- Covering all eight spine items honestly would require exceeding 1800 words.
- Satisfying one instruction in this packet would require violating another.

## Context

Change: **skill-anchoring-guardrails**. The specs synthesis (running concurrently as a sibling packet in this same wave, on the same base) determined all five capabilities are genuinely New (no live spec exists for any of them) — this does not affect your architecture arbitration, only informs that no MODIFIED-block copy risk exists for this change at the design level. Execution: Isolated Mode, `agy`-only executor except verify's second qualitative judge (kept on `cursor-agent`) — already decided, do not re-litigate.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, report the counts that matter: citations verified, citations dropped, contradictions escalated, coverage gaps. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
