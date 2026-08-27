---
id: tasks-skill-anchoring-guardrails-synthesis
executor: agy
routed_by: synthesis of three parallel tasks lenses into one canonical tasks.md — executor overridden from the template default (cursor-agent) to agy per human-approved AGY-only Execution Strategy for Change skill-anchoring-guardrails; cursor-agent is reserved only for verify's second qualitative judge
allowed_paths: ["openspec/changes/skill-anchoring-guardrails/tasks.md", "openspec/changes/skill-anchoring-guardrails/tasks-synthesis-notes.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: 330c602517cb9cda29bdf52290efb58791771bf4
expected_parent_sha: 330c602517cb9cda29bdf52290efb58791771bf4
---

# Packet tasks-skill-anchoring-guardrails-synthesis

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/tasks-skill-anchoring-guardrails-synthesis  ·  **Branch:** lucind/tasks-skill-anchoring-guardrails-synthesis

## Goal

Read the three tasks lens drafts for `skill-anchoring-guardrails`, verify their claims against the real code, arbitrate where they disagree, and produce one canonical `openspec/changes/skill-anchoring-guardrails/tasks.md` plus a separate synthesis notes file.

You are the last judgment in this phase. The apply phase executes what you accept without checking.

## Why this is safe to dispatch now

All three lens lanes reached terminal `done` status and integrated (integrated=3, reverted=0). Lens B recommended a single packet with no `apply-dag.yaml` sidecar — verify that recommendation independently in step 5 rather than taking it on faith.

## Preconditions

- `tasks-lens-a.md`, `tasks-lens-b.md`, and `tasks-lens-c.md` all exist in this worktree.
- `openspec/changes/skill-anchoring-guardrails/tasks.md` does not yet exist.
- The spec and design for `skill-anchoring-guardrails` are present and accepted.

## What each lens owns

| Draft | Owns |
|---|---|
| `tasks-lens-a.md` | Phased checklist; dependency order; requirement traceability |
| `tasks-lens-b.md` | Suggested Work Units; wave plan; `allowed_paths`; executor assignment; disjointness check; sidecar recommendation |
| `tasks-lens-c.md` | Review Workload Forecast; RED tests from the threat matrix; acceptance evidence; verification gaps |

All three also emit `## Open Questions`. Merge and deduplicate them.

## Required procedure

### 1. Read all three drafts in full

### 2. Citation verification pass

Every `file:line` citation in every draft is a claim about this repository. Open each one in this worktree and confirm it. Lens C's proving commands and lens B's path claims matter most. Drop and record under `## Dropped Citations` anything that does not resolve or support the claim.

### 3. Decomposition arbitration

- **Lens A's decomposition is authoritative.**
- A work unit from lens B or an acceptance row from lens C that maps to no task in lens A's checklist does not go into `tasks.md`. Record under `## Decomposition Divergence`.
- If lens B or lens C converged independently on lens A's decomposition, say so.
- If lens A's decomposition is refuted by code you verified, do not silently substitute your own — that is a hard stop.

### 4. Assemble, do not concatenate

`tasks.md` MUST be under 1800 words.

### 5. Wave viability re-check

Do not take lens B's "green on its own" column or its single-packet recommendation on faith. Independently verify: does the recommended shape (single packet vs. multi-unit) actually hold given `internal/packet/disjoint.go`'s component-boundary prefix rule and the fact that `cmd/lucind-ai/cli.go` is touched by multiple concerns in this change (flag parsing, banners, internal callers)? Record your own independent verdict.

### 6. Coverage check

`tasks.md` must satisfy:

1. Review Workload Forecast table, every field populated
2. Suggested Work Units table, each unit a standalone deliverable with a rollback boundary
3. Phased checklist, every task specific, actionable, verifiable, small
4. A RED-test task before its production task for every threat-matrix row the design marked `Applicable`, none for `N/A`
5. Dependency order stated explicitly
6. Every wave green on its own under `Integrate`, every same-wave unit pair path-disjoint
7. Executor named per unit wherever a DAG is intended
8. Every requirement in `specs/` traced to at least one task

Anything no draft covered goes under `## Coverage Gaps`.

## Output

### `openspec/changes/skill-anchoring-guardrails/tasks.md`

The canonical checklist. Under 1800 words. Covers all eight spine items.

### `openspec/changes/skill-anchoring-guardrails/tasks-synthesis-notes.md`

```markdown
# Tasks Synthesis Notes: Skill Anchoring & Worktree Cleanup Guardrails

## Unresolved Contradictions

<"None" if there are none.>

## Coverage Gaps

<"None" if there are none.>

## Dropped Citations

<"None" if there are none.>

## Decomposition Divergence

<"None — all three converged" if that is the case.>
```

## Out of scope

- Do NOT modify the three lens drafts.
- Do NOT write `apply-dag.yaml`.
- Do NOT write specs, design, or any implementation code.
- Do NOT resolve an unresolved contradiction by choosing.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`.

## Allowed paths

`openspec/changes/skill-anchoring-guardrails/tasks.md` and `openspec/changes/skill-anchoring-guardrails/tasks-synthesis-notes.md` only.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-tasks/` — the real `gentle-ai` tasks skill. This packet sets the 1800-word budget, the synthesis procedure, the wave viability re-check, the notes file, and the done criteria.

Write nothing outside this repository.

## Using the lens citation manifests

Treat the union of the three manifests as your **verification worklist, never as evidence**. Open every cited range yourself. Record each outcome in `## Dropped Citations`.

## Commit discipline (REQUIRED — two commits, not one)

**Commit `tasks.md` the moment it is written, before you begin the notes file.** Then write the notes and commit them as a second conventional commit. Both commits are conventional, with no AI attribution. Strip any injected `Co-authored-by:` trailer.

## Mechanical self-check (REQUIRED)

**Right after the first commit:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/tasks.md --budget 1800 --skip-result
```

**After the second commit and writing `.lucind/result.json`:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/tasks-synthesis-notes.md \
  --require-section "Unresolved Contradictions" --require-section "Coverage Gaps" \
  --require-section "Dropped Citations" --require-section "Decomposition Divergence"
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence`.

## Done criteria

- [ ] **`tasks.md` was committed as its own commit before `tasks-synthesis-notes.md` was started**, confirmed by the mid-flow `lucind-lane-check.sh` run reporting a clean `git status --porcelain`.
- [ ] **Every `file:line` citation surviving into `tasks.md` was opened and confirmed in this worktree**, and every dropped claim is listed under `## Dropped Citations`.
- [ ] **Every wave was independently judged green-on-its-own**, and every wave that was not is merged and reported under `## Coverage Gaps`.
- [ ] **Every same-wave unit pair was re-checked against the component-boundary prefix rule** rather than taken from lens B's table.
- [ ] **`tasks.md` exists, is under 1800 words, and substantively covers all eight spine items**, with anything missing reported under `## Coverage Gaps`.
- [ ] **`tasks-synthesis-notes.md` exists with exactly the four required sections**, each either populated or explicitly "None".
- [ ] **The work is committed with two conventional commits and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`.

## Hard stops

- The three `## Assumed decomposition` blocks are mutually irreconcilable and the spec and design do not choose between them. Write the notes file, leave `tasks.md` uncreated, and block.
- Lens A's decomposition is refuted by code you verified. Do not substitute your own.
- No partition exists in which every wave is green on its own. Report it; a single packet may be the correct answer.
- One or more lens drafts is missing from this worktree.
- Covering all eight spine items honestly would require exceeding 1800 words.
- Satisfying one instruction in this packet would require violating another.

## Context

Change: **skill-anchoring-guardrails**. Delivery strategy: **single-pr**, review budget **2000 changed lines** (human-confirmed). Execution: Isolated Mode, `agy`-only executor except verify's second qualitative judge (kept on `cursor-agent`) — already decided, do not re-litigate. After this phase, apply dispatches directly (single packet if lens B/your own re-check confirms no sidecar is warranted).

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, report the counts that matter: citations verified, citations dropped, waves merged for viability, contradictions escalated, coverage gaps. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
