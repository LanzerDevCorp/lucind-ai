---
id: propose-agentic-phase-specialist-synthesis
executor: cursor-agent
routed_by: synthesis of three parallel propose lenses into one canonical proposal document
model: cursor-grok-4.6-high
allowed_paths: ["openspec/changes/agentic-phase-specialist/proposal.md", "openspec/changes/agentic-phase-specialist/proposal-synthesis-notes.md"]
---

# Packet propose-agentic-phase-specialist-synthesis

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/propose-agentic-phase-specialist-synthesis  ·  **Branch:** lucind/propose-agentic-phase-specialist-synthesis

## Goal

Read the three propose lens drafts for `agentic-phase-specialist`, verify their claims against the real code, arbitrate where they disagree, and produce one canonical `openspec/changes/agentic-phase-specialist/proposal.md` plus a separate synthesis notes file recording everything that did not make it in and why.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you — the orchestrator reads only your notes file. Anything you accept without checking, ships.

## Why this is safe to dispatch now

All three lens lanes have reached terminal status and integrated. This worktree is branched from the integrated result, so `propose-lens-a.md`, `propose-lens-b.md`, and `propose-lens-c.md` are all present here. Lens worktrees could not see each other; this one sees all three.

## Preconditions

- `propose-lens-a.md`, `propose-lens-b.md`, and `propose-lens-c.md` all exist in this worktree.
- `openspec/changes/agentic-phase-specialist/proposal.md` does not yet exist.

## What each lens owns

| Draft | Owns |
|---|---|
| `propose-lens-a.md` | Candidate selection; technical approach; conceptual changes; alternatives considered |
| `propose-lens-b.md` | User and capability impact table; delta specification requirements and scenarios |
| `propose-lens-c.md` | Technical risks and failure modes; rollback plan and additivity; test and validation impact; out of scope |

All three also emit `## Open Questions`. Merge and deduplicate them.

## Required procedure

Do these in order. Skipping step 2 or step 3 makes the output worthless regardless of how good it reads.

### 1. Read all three drafts in full

Do not begin writing until you have read all three.

### 2. Citation verification pass

Every `file:line` citation in every draft is a claim about this repository. Open each one in this worktree and confirm it says what the draft says it says.

- A citation that resolves and supports the claim: keep it.
- A citation that does not resolve, points at unrelated code, or does not support the claim: **drop the claim from `proposal.md`** and record it under `## Dropped Citations` in the notes with what you found instead.

A lens draft is evidence, not authority. You have the code; use it.

### 3. Candidate and scope arbitration

Compare the technical approaches and delta specs across drafts.

- Lens A's candidate selection and approach is authoritative.
- Any content in lens B or lens C that contradicts lens A's chosen candidate does not go into `proposal.md`. Record it under `## Scope Divergence` in the notes.
- If lens B or lens C converged independently on lens A's approach, record that corroboration in the notes.

### 4. Compress — do not concatenate

`proposal.md` MUST be under 1800 words. Merge overlapping statements, drop restatement, keep the specific sentence over the general one. A concatenation of three drafts is a failed synthesis even if every word in it is true.

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

Anything no draft covered goes under `## Coverage Gaps` in the notes. Do not invent content to fill a gap; report it.

## Output

### `openspec/changes/agentic-phase-specialist/proposal.md`

The canonical proposal. Under 1800 words. Covers the proposal spine. Contains only claims whose citations you verified in step 2 and which survive lens A's approach.

### `openspec/changes/agentic-phase-specialist/proposal-synthesis-notes.md`

Exactly these four sections, in this order. This file is what the orchestrator (and the phase-Specialist judging this phase's Acceptance) reads:

```markdown
# Synthesis Notes: Agentic Phase Specialist

## Unresolved Contradictions

<Where two drafts assert incompatible things and the code does not settle it.
State both positions and what evidence each has. Do NOT pick — this section is
the escalation. "None" if there are none.>

## Coverage Gaps

<Spine items no draft covered. "None" if there are none.>

## Dropped Citations

<Every claim removed in step 2, with the citation that failed and what the code
actually says. "None" if there are none.>

## Scope Divergence

<What lens B or lens C assumed that differed from lens A, what content that cost
them, and where they converged independently. "None — all three converged" if
that is the case.>
```

## Out of scope

- Do NOT modify the three lens drafts. They are the record of what each lens produced.
- Do NOT write specs, design, tasks, or any implementation code.
- Do NOT resolve an unresolved contradiction by choosing. Escalating it is the correct output.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`. This is a document synthesis lane.

## Allowed paths

`openspec/changes/agentic-phase-specialist/proposal.md` and `openspec/changes/agentic-phase-specialist/proposal-synthesis-notes.md` only.

## Allowed paths outside the repository

**Read-only**: The real `gentle-ai` propose skill (delivered under `## Required skills`). Check the canonical document against the contract as written.

This packet sets the 1800-word budget along with the synthesis procedure, the notes file, and the done criteria.

Write nothing outside this repository.

## Using the lens citation manifests

Each lens draft ends with a `## Citation Manifest` table. Treat the union of the three manifests as your **verification worklist, never as evidence**. Open every cited range yourself and check it against the real code in this worktree.

- **Deduplicate across the three.** Verify each unique citation exactly once, not once per prose mention.
- **Batch by file.** Open each cited file once and check every citation into it, instead of jumping between files in prose order.
- **Verify the claim, not the line's existence.** A range that exists but does not support the claim is a dropped citation.
- **A citation in a lens's prose but missing from its manifest is still yours to verify.**

Record each entry's outcome — verified, dropped, or retargeted — in `## Dropped Citations`.

## Commit discipline (REQUIRED — two commits, not one)

**Commit `proposal.md` the moment it is written, before you begin the notes file.** Then write the notes and commit them as a second conventional commit.

Both commits are conventional, with no AI attribution. Check `git log -1 --format=%B` after each one and strip any injected `Co-authored-by:` trailer.

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root, bracketing each commit.

**Right after the first commit** (`proposal.md`, before you start the notes file):

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/proposal.md --budget 1800 --skip-result
```

**After the second commit and writing `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/proposal-synthesis-notes.md \
  --require-section "Unresolved Contradictions" --require-section "Coverage Gaps" \
  --require-section "Dropped Citations" --require-section "Scope Divergence"
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence` in your envelope instead of narrating the same facts in prose.

## Done criteria

- [ ] **`proposal.md` was committed as its own commit before `proposal-synthesis-notes.md` was started**, confirmed by the mid-flow `lucind-lane-check.sh` run reporting a clean `git status --porcelain`.
- [ ] **Every `file:line` citation surviving into `proposal.md` was opened and confirmed in this worktree**, and every dropped claim is listed under `## Dropped Citations`.
- [ ] **`proposal.md` exists, is under 1800 words, and substantively covers the proposal spine**, with anything missing reported under `## Coverage Gaps`.
- [ ] **`proposal-synthesis-notes.md` exists with exactly the four required sections**, each either populated or explicitly "None", confirmed by the final `lucind-lane-check.sh` run.
- [ ] **The work is committed with two conventional commits and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The proposed approaches across drafts are mutually irreconcilable. Write the notes file, leave `proposal.md` uncreated, and block.
- One or more lens drafts is missing from this worktree.
- Covering the proposal spine honestly would require exceeding 1800 words. Report which item forces it rather than silently overrunning or silently cutting.
- Satisfying one instruction in this packet would require violating another.

## Context

**Change**: `agentic-phase-specialist` — insert a phase-scoped **Specialist** (existing `sdd-*` Claude Code subagent) that administers its own SDD phase's fan-out+synthesis dispatch via lucind-ai, independently accepts its own phase's Lanes without human confirmation, and reports only a compressed **Phase Verdict** to the Orchestrator. Promotion of the whole Change stays human-confirmed, unchanged. Supersedes the archived `2026-08-29-skill-provisioning-and-phase-specialist`'s deterministic, non-agentic `internal/phasespec.Adapter` definition of "Specialist" (that adapter's status/eligibility/dispatch mechanics remain a callable tool).

**Known hard constraint, already surfaced in exploration and all three lenses**: `sdd-propose`/`sdd-spec`/`sdd-design`/`sdd-tasks` have no Bash/Agent tool access and cannot themselves dispatch `lucind-ai run`. For now the Orchestrator performs the mechanical dispatch while the Specialist subagent authors packets, reads synthesis, and judges Acceptance. If the three lenses disagree on how to phrase or scope this constraint, it is not a real contradiction to escalate — treat the exploration's own framing (in `openspec/changes/agentic-phase-specialist/explore.md`) as authoritative on this specific point.

**Human decision already made** (do not treat as an open question even if a lens drafted it as one): planning phases for THIS change itself run via lucind-ai fan-out+synthesis (as you are running right now), administered by the corresponding `sdd-*` phase-Specialist subagent judging Acceptance — this was an explicit human choice resolving the scoping question explore.md raised, not something for this synthesis to re-open.

## Required skills

- sdd-propose

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, report the counts that matter: citations verified, citations dropped, contradictions escalated, coverage gaps. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
