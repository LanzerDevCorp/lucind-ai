---
id: explore-skill-anchoring-guardrails-synthesis
executor: agy
routed_by: synthesis of three parallel explore lenses into one canonical explore document — executor overridden from the template default (cursor-agent) to agy per human-approved AGY-only Execution Strategy for Change skill-anchoring-guardrails; cursor-agent is reserved only for verify's second qualitative judge
allowed_paths: ["openspec/changes/skill-anchoring-guardrails/explore.md", "openspec/changes/skill-anchoring-guardrails/explore-synthesis-notes.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: e54bf2af3327cbb2a2002562d0b21259802db2f3
expected_parent_sha: e54bf2af3327cbb2a2002562d0b21259802db2f3
---

# Packet explore-skill-anchoring-guardrails-synthesis

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/explore-skill-anchoring-guardrails-synthesis  ·  **Branch:** lucind/explore-skill-anchoring-guardrails-synthesis

## Goal

Read the three explore lens drafts for `skill-anchoring-guardrails`, verify their claims against the real code, arbitrate where they disagree, and produce one canonical `openspec/changes/skill-anchoring-guardrails/explore.md` plus a separate synthesis notes file recording everything that did not make it in and why.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you — the orchestrator reads only your notes file. Anything you accept without checking, ships.

## Why this is safe to dispatch now

All three lens lanes (`explore-skill-anchoring-guardrails-lens-a/b/c`) reached terminal `done` status and integrated onto `refs/heads/feature/skill-anchoring-guardrails` (integrated=3, reverted=0). This packet's `base_sha` is refreshed to that integrated tip, so `explore-lens-a.md`, `explore-lens-b.md`, and `explore-lens-c.md` are all present in this worktree. Lens worktrees could not see each other; this one sees all three.

## Preconditions

- `explore-lens-a.md`, `explore-lens-b.md`, and `explore-lens-c.md` all exist in this worktree.
- `openspec/changes/skill-anchoring-guardrails/explore.md` does not yet exist.

## What each lens owns

| Draft | Owns |
|---|---|
| `explore-lens-a.md` | Problem space; candidate approaches; initial recommendations |
| `explore-lens-b.md` | User and capability impact; scenarios and use cases; success criteria |
| `explore-lens-c.md` | Technical risks and unknowns; trade-offs matrix; potential spikes; out of scope |

All three also emit `## Open Questions`. Merge and deduplicate them.

## Required procedure

Do these in order. Skipping step 2 or step 3 makes the output worthless regardless of how good it reads.

### 1. Read all three drafts in full

Do not begin writing until you have read all three.

### 2. Citation verification pass

Every `file:line` citation in every draft is a claim about this repository. Open each one in this worktree and confirm it says what the draft says it says.

- A citation that resolves and supports the claim: keep it.
- A citation that does not resolve, points at unrelated code, or does not support the claim: **drop the claim from `explore.md`** and record it under `## Dropped Citations` in the notes with what you found instead.

A lens draft is evidence, not authority. You have the code; use it.

### 3. Approach and problem arbitration

Compare the problem statements and candidate feasibility across drafts.

- Lens A's problem and candidate analysis is primary.
- If lens B scenarios or lens C risks reveal unviable candidates, document the arbitration in synthesis notes.
- If all three converged independently on problem boundaries and approach viability, record that corroboration under `## Approach Divergence` in the notes.

### 4. Compress — do not concatenate

`explore.md` MUST be under 1800 words. The three drafts total roughly 3000. Cutting is the job: merge overlapping statements, drop restatement, keep the specific sentence over the general one. A concatenation of three drafts is a failed synthesis even if every word in it is true.

### 5. Coverage check

`explore.md` must cover this repository's exploration spine:

1. Problem statement and background
2. Candidate approaches (pros, cons, feasibility)
3. User & capability impact
4. Scenarios and use cases
5. Technical risks and trade-offs matrix
6. Potential spikes or proof-of-concepts
7. Success criteria
8. Out of scope and open questions

Anything no draft covered goes under `## Coverage Gaps` in the notes. Do not invent content to fill a gap; report it.

## Output

### `openspec/changes/skill-anchoring-guardrails/explore.md`

The canonical exploration document. Under 1800 words. Covers the exploration spine. Contains only claims whose citations you verified in step 2.

### `openspec/changes/skill-anchoring-guardrails/explore-synthesis-notes.md`

Exactly these four sections, in this order. This file is what the orchestrator reads:

```markdown
# Synthesis Notes: Skill Anchoring & Worktree Cleanup Guardrails

## Unresolved Contradictions

<Where two drafts assert incompatible things and the code does not settle it.
State both positions and what evidence each has. Do NOT pick — this section is
the escalation. "None" if there are none.>

## Coverage Gaps

<Spine items no draft covered. "None" if there are none.>

## Dropped Citations

<Every claim removed in step 2, with the citation that failed and what the code
actually says. "None" if there are none.>

## Approach Divergence

<What lens B or lens C assumed that differed from lens A, what content that cost
them, and where they converged independently. "None — all three converged" if
that is the case.>
```

## Out of scope

- Do NOT modify the three lens drafts. They are the record of what each lens produced.
- Do NOT write proposal, specs, design, tasks, or any implementation code.
- Do NOT resolve an unresolved contradiction by choosing. Escalating it is the correct output.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`. This is a document synthesis lane.

## Allowed paths

`openspec/changes/skill-anchoring-guardrails/explore.md` and `openspec/changes/skill-anchoring-guardrails/explore-synthesis-notes.md` only.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-explore/` — the real `gentle-ai` explore skill. Check the canonical document against the contract as written.

This packet sets the 1800-word budget along with the synthesis procedure, the notes file, and the done criteria.

Write nothing outside this repository.

## Using the lens citation manifests

Each lens draft ends with a `## Citation Manifest` table. Treat the union of the three manifests as your **verification worklist, never as evidence**. A manifest row was written by the same lane that made the claim, so a wrong citation arrives with a confident row beside it. The property that makes this fan-out trustworthy is that you open every cited range yourself and check it against the real code in this worktree. That property is not negotiable and the manifests do not relax it.

What the manifests are for is speed without loss:

- **Deduplicate across the three.** Verify each unique citation exactly once, not once per prose mention.
- **Batch by file.** Open each cited file once and check every citation into it, instead of jumping between files in prose order.
- **Verify the claim, not the line's existence.** A row states what the lens asserts that range shows. A range that exists but does not support the claim is a dropped citation.
- **A citation in a lens's prose but missing from its manifest is still yours to verify.** An incomplete manifest does not shrink your obligation.

Record each entry's outcome — verified, dropped, or retargeted — in `## Dropped Citations`.

## Commit discipline (REQUIRED — two commits, not one)

**Commit `explore.md` the moment it is written, before you begin the notes file.** Then write the notes and commit them as a second conventional commit.

This is not bookkeeping. A synthesis lane that dies on the wall clock after its first commit leaves finished work on its branch, recoverable by whoever re-dispatches it. A lane that dies before a single end-of-run commit leaves an untracked file that `lucind-ai worktree cleanup` deletes without warning. Two commits convert a timeout from lost work into resumable work.

Both commits are conventional, with no AI attribution. Check `git log -1 --format=%B` after each one and strip any injected `Co-authored-by:` trailer.

## Mechanical self-check (REQUIRED)

Run `./lucind-lane-check.sh` from the repo root, bracketing each commit.

**Right after the first commit** (`explore.md`, before you start the notes file):

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/explore.md --budget 1800 --skip-result
```

**After the second commit and writing `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/explore-synthesis-notes.md \
  --require-section "Unresolved Contradictions" --require-section "Coverage Gaps" \
  --require-section "Dropped Citations" --require-section "Approach Divergence"
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence`.

## Done criteria

- [ ] **`explore.md` was committed as its own commit before `explore-synthesis-notes.md` was started**, confirmed by the mid-flow `lucind-lane-check.sh` run reporting a clean `git status --porcelain`.
- [ ] **Every `file:line` citation surviving into `explore.md` was opened and confirmed in this worktree**, and every dropped claim is listed under `## Dropped Citations`.
- [ ] **`explore.md` exists, is under 1800 words, and substantively covers the exploration spine**, with anything missing reported under `## Coverage Gaps`.
- [ ] **`explore-synthesis-notes.md` exists with exactly the four required sections**, each either populated or explicitly "None", confirmed by the final `lucind-lane-check.sh` run.
- [ ] **The work is committed with two conventional commits and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The problem space or candidate approaches across drafts are mutually irreconcilable. Write the notes file, leave `explore.md` uncreated, and block.
- One or more lens drafts is missing from this worktree.
- Covering the exploration spine honestly would require exceeding 1800 words. Report which item forces it rather than silently overrunning or silently cutting.
- Satisfying one instruction in this packet would require violating another.

## Context

Change: **skill-anchoring-guardrails** — worktree dirty guardrails (`--force` gate on `lucind-ai worktree cleanup`), CLI banner anchoring of agent/operator terminal output to the existing skill reference docs at the exact moment a Lane blocks/times out/reverts/needs qualitative review, and a prescriptive TDD WIP-rescue protocol. Source lead: `docs/plan_1_audit_and_skill_anchoring.md` (human draft, unverified). Execution: Isolated Mode, `agy`-only executor except verify's second qualitative judge (kept on `cursor-agent`) — already decided, do not re-litigate.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, report the counts that matter: citations verified, citations dropped, contradictions escalated, coverage gaps. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
