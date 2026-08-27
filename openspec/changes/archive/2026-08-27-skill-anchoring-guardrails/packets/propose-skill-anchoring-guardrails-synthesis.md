---
id: propose-skill-anchoring-guardrails-synthesis
executor: agy
routed_by: synthesis of three parallel propose lenses into one canonical proposal document — executor overridden from the template default (cursor-agent) to agy per human-approved AGY-only Execution Strategy for Change skill-anchoring-guardrails; cursor-agent is reserved only for verify's second qualitative judge
allowed_paths: ["openspec/changes/skill-anchoring-guardrails/proposal.md", "openspec/changes/skill-anchoring-guardrails/proposal-synthesis-notes.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: 29bb0f4794922387bcee5738484e960b6fdfa1cb
expected_parent_sha: 29bb0f4794922387bcee5738484e960b6fdfa1cb
---

# Packet propose-skill-anchoring-guardrails-synthesis

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/propose-skill-anchoring-guardrails-synthesis  ·  **Branch:** lucind/propose-skill-anchoring-guardrails-synthesis

## Goal

Read the three propose lens drafts for `skill-anchoring-guardrails`, verify their claims against the real code, arbitrate where they disagree, and produce one canonical `openspec/changes/skill-anchoring-guardrails/proposal.md` plus a separate synthesis notes file recording everything that did not make it in and why.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you — the orchestrator reads only your notes file. Anything you accept without checking, ships.

## Why this is safe to dispatch now

All three lens lanes (`propose-skill-anchoring-guardrails-lens-a/b/c`) reached terminal `done` status and integrated onto `refs/heads/feature/skill-anchoring-guardrails` (integrated=3, reverted=0). This packet's `base_sha` is refreshed to that integrated tip, so all three drafts are present in this worktree.

## Preconditions

- `propose-lens-a.md`, `propose-lens-b.md`, and `propose-lens-c.md` all exist in this worktree.
- `openspec/changes/skill-anchoring-guardrails/proposal.md` does not yet exist.

## What each lens owns

| Draft | Owns |
|---|---|
| `propose-lens-a.md` | Candidate selection; technical approach; conceptual changes; alternatives considered |
| `propose-lens-b.md` | User and capability impact table; delta specification requirements and scenarios |
| `propose-lens-c.md` | Technical risks and failure modes; rollback plan and additivity; test and validation impact; out of scope |

All three also emit `## Open Questions`. Merge and deduplicate them.

## Required procedure

### 1. Read all three drafts in full

Do not begin writing until you have read all three.

### 2. Citation verification pass

Every `file:line` citation in every draft is a claim about this repository. Open each one in this worktree and confirm it says what the draft says it says.

- A citation that resolves and supports the claim: keep it.
- A citation that does not resolve, points at unrelated code, or does not support the claim: **drop the claim from `proposal.md`** and record it under `## Dropped Citations`.

### 3. Candidate and scope arbitration

- Lens A's candidate selection and approach is authoritative.
- Any content in lens B or lens C that contradicts lens A's chosen candidate does not go into `proposal.md`. Record it under `## Scope Divergence`.
- If lens B or lens C converged independently on lens A's approach, record that corroboration in the notes.

### 4. Compress — do not concatenate

`proposal.md` MUST be under 1800 words. Merge overlapping statements, drop restatement, keep the specific sentence over the general one.

### 5. Coverage check

`proposal.md` must cover this repository's proposal spine:

1. Executive summary and problem statement
2. Selected candidate and proposed technical approach
3. Changes to system concepts and architecture rationale
4. User and capability impact table
5. Delta specifications (requirements and scenarios)
6. Technical risks and failure modes
7. Rollback plan and additivity
8. Test and validation impact
9. Out of scope and open questions

Anything no draft covered goes under `## Coverage Gaps`. Do not invent content to fill a gap; report it.

## Output

### `openspec/changes/skill-anchoring-guardrails/proposal.md`

The canonical proposal. Under 1800 words. Covers the proposal spine. Contains only claims whose citations you verified in step 2 and which survive lens A's approach.

### `openspec/changes/skill-anchoring-guardrails/proposal-synthesis-notes.md`

Exactly these four sections, in this order:

```markdown
# Synthesis Notes: Skill Anchoring & Worktree Cleanup Guardrails

## Unresolved Contradictions

<"None" if there are none.>

## Coverage Gaps

<"None" if there are none.>

## Dropped Citations

<"None" if there are none.>

## Scope Divergence

<"None — all three converged" if that is the case.>
```

## Out of scope

- Do NOT modify the three lens drafts.
- Do NOT write specs, design, tasks, or any implementation code.
- Do NOT resolve an unresolved contradiction by choosing. Escalating it is the correct output.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`.

## Allowed paths

`openspec/changes/skill-anchoring-guardrails/proposal.md` and `openspec/changes/skill-anchoring-guardrails/proposal-synthesis-notes.md` only.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-propose/` — the real `gentle-ai` propose skill. Check the canonical document against the contract as written.

Write nothing outside this repository.

## Using the lens citation manifests

Treat the union of the three manifests as your **verification worklist, never as evidence**. Open every cited range yourself and check it against the real code in this worktree. Deduplicate across the three, batch by file, verify the claim not just the line's existence. Record each entry's outcome — verified, dropped, or retargeted — in `## Dropped Citations`.

## Commit discipline (REQUIRED — two commits, not one)

**Commit `proposal.md` the moment it is written, before you begin the notes file.** Then write the notes and commit them as a second conventional commit. Both commits are conventional, with no AI attribution. Check `git log -1 --format=%B` after each one and strip any injected `Co-authored-by:` trailer.

## Mechanical self-check (REQUIRED)

**Right after the first commit:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/proposal.md --budget 1800 --skip-result
```

**After the second commit and writing `.lucind/result.json`:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/proposal-synthesis-notes.md \
  --require-section "Unresolved Contradictions" --require-section "Coverage Gaps" \
  --require-section "Dropped Citations" --require-section "Scope Divergence"
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence`.

## Done criteria

- [ ] **`proposal.md` was committed as its own commit before `proposal-synthesis-notes.md` was started**, confirmed by the mid-flow `lucind-lane-check.sh` run reporting a clean `git status --porcelain`.
- [ ] **Every `file:line` citation surviving into `proposal.md` was opened and confirmed in this worktree**, and every dropped claim is listed under `## Dropped Citations`.
- [ ] **`proposal.md` exists, is under 1800 words, and substantively covers the proposal spine**, with anything missing reported under `## Coverage Gaps`.
- [ ] **`proposal-synthesis-notes.md` exists with exactly the four required sections**, each either populated or explicitly "None", confirmed by the final `lucind-lane-check.sh` run.
- [ ] **The work is committed with two conventional commits and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`.

## Hard stops

- The proposed approaches across drafts are mutually irreconcilable. Write the notes file, leave `proposal.md` uncreated, and block.
- One or more lens drafts is missing from this worktree.
- Covering the proposal spine honestly would require exceeding 1800 words. Report which item forces it.
- Satisfying one instruction in this packet would require violating another.

## Context

Change: **skill-anchoring-guardrails** — worktree dirty guardrails, CLI banner anchoring to skill reference docs, and a prescriptive TDD WIP-rescue protocol. Execution: Isolated Mode, `agy`-only executor except verify's second qualitative judge (kept on `cursor-agent`) — already decided, do not re-litigate.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, report the counts that matter: citations verified, citations dropped, contradictions escalated, coverage gaps. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
