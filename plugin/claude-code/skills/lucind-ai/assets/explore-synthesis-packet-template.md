---
id: explore-<change-id>-synthesis
executor: cursor-agent
routed_by: synthesis of three parallel explore lenses into one canonical explore document
model: cursor-grok-4.6-high
allowed_paths: ["openspec/changes/<change-id>/explore.md", "openspec/changes/<change-id>/explore-synthesis-notes.md"]
---

# Packet explore-<change-id>-synthesis

**Tier:** A (human merge)
**Worktree:** ../<repo>-worktrees/explore-<change-id>-synthesis  ·  **Branch:** lucind/explore-<change-id>-synthesis

## Goal

Read the three explore lens drafts for `<change-id>`, verify their claims against the real code, arbitrate where they disagree, and produce one canonical `openspec/changes/<change-id>/explore.md` plus a separate synthesis notes file recording everything that did not make it in and why.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you — the orchestrator reads only your notes file. Anything you accept without checking, ships.

## Why this is safe to dispatch now

All three lens lanes have reached terminal status and integrated. This worktree is branched from the integrated result, so `explore-lens-a.md`, `explore-lens-b.md`, and `explore-lens-c.md` are all present here. Lens worktrees could not see each other; this one sees all three.

## Preconditions

- `explore-lens-a.md`, `explore-lens-b.md`, and `explore-lens-c.md` all exist in this worktree.
- `openspec/changes/<change-id>/explore.md` does not yet exist.

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

### `openspec/changes/<change-id>/explore.md`

The canonical exploration document. Under 1800 words. Covers the exploration spine. Contains only claims whose citations you verified in step 2.

### `openspec/changes/<change-id>/explore-synthesis-notes.md`

Exactly these four sections, in this order. This file is what the orchestrator reads:

```markdown
# Synthesis Notes: <Change Title>

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

`openspec/changes/<change-id>/explore.md` and `openspec/changes/<change-id>/explore-synthesis-notes.md` only.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-explore/` — the real `gentle-ai` explore skill. Check the
canonical document against the contract as written.

This packet sets the 1800-word budget along with the synthesis procedure, the notes file, and the done criteria.

Write nothing outside this repository.

## Done criteria

- [ ] **Every `file:line` citation surviving into `explore.md` was opened and confirmed in this worktree**, and every dropped claim is listed under `## Dropped Citations`.
- [ ] **`explore.md` exists, is under 1800 words, and substantively covers the exploration spine**, with anything missing reported under `## Coverage Gaps`.
- [ ] **`explore-synthesis-notes.md` exists with exactly the four required sections**, each either populated or explicitly "None".
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The problem space or candidate approaches across drafts are mutually irreconcilable. Write the notes file, leave `explore.md` uncreated, and block.
- One or more lens drafts is missing from this worktree.
- Covering the exploration spine honestly would require exceeding 1800 words. Report which item forces it rather than silently overrunning or silently cutting.
- Satisfying one instruction in this packet would require violating another.

## Context

<The change title, problem summary, and any decision the human has already
made in conversation and does not want re-litigated.>

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, report the counts that matter: citations verified, citations dropped, contradictions escalated, coverage gaps. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
