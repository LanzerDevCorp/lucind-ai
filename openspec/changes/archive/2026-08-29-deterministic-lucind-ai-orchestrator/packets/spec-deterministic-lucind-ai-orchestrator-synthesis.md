---
id: spec-deterministic-lucind-ai-orchestrator-synthesis
executor: cursor-agent
routed_by: synthesis of three parallel spec lenses into the canonical delta spec tree
model: cursor-grok-4.6-high
allowed_paths: ["openspec/changes/deterministic-lucind-ai-orchestrator/specs/", "openspec/changes/deterministic-lucind-ai-orchestrator/spec-synthesis-notes.md"]
---

# Packet spec-deterministic-lucind-ai-orchestrator-synthesis

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-deterministic-orchestrator-worktrees/spec-deterministic-lucind-ai-orchestrator-synthesis  ·  **Branch:** lucind/spec-deterministic-lucind-ai-orchestrator-synthesis

## Goal

Read the three spec lens drafts for `deterministic-lucind-ai-orchestrator`, verify their claims
against the real code and the real live specs, arbitrate where they disagree, and produce the
canonical delta spec tree under `openspec/changes/deterministic-lucind-ai-orchestrator/specs/` plus
a separate synthesis notes file recording everything that did not make it in and why.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you — the
orchestrator reads only your notes file. Anything you accept without checking ships into
`openspec/specs/` at archive time.

## Why this is safe to dispatch now

All three lens lanes have reached terminal status (`done`) and integrated at commit `55405b4`.
This worktree is branched from the integrated result, so `spec-lens-a.md`, `spec-lens-b.md`, and
`spec-lens-c.md` are all present here. Lens worktrees could not see each other; this one sees all
three.

## Preconditions

- `spec-lens-a.md`, `spec-lens-b.md`, and `spec-lens-c.md` all exist in this worktree.
- `openspec/changes/deterministic-lucind-ai-orchestrator/specs/` does not yet exist.
- The proposal for `deterministic-lucind-ai-orchestrator` is present and accepted.

## What each lens owns

| Draft | Owns |
|---|---|
| `spec-lens-a.md` | Capability map; requirement statements; ADDED / MODIFIED / REMOVED / RENAMED classification |
| `spec-lens-b.md` | Given/When/Then scenarios; coverage table; untestable assertions |
| `spec-lens-c.md` | Live-spec inventory and conflicts; verbatim MODIFIED full blocks; removals, renames, consumers, migration |

All three also emit `## Open Questions`. Merge and deduplicate them.

## Required procedure

Do these in order. Skipping step 2 or step 3 makes the output worthless regardless of how good it
reads.

### 1. Read all three drafts in full

Do not begin writing until you have read all three.

### 2. Citation verification pass

Every `file:line` citation in every draft is a claim about this repository. Open each one in this
worktree and confirm it says what the draft says it says. Lens C's live-spec citations matter
most: a MODIFIED block copied from a requirement that is not there is a silent deletion at archive
time.

- A citation that resolves and supports the claim: keep it.
- A citation that does not resolve, points at unrelated code, or does not support the claim:
  **drop the claim from the delta spec** and record it under `## Dropped Citations` in the notes
  with what you found instead.

A lens draft is evidence, not authority. You have the code and the live specs; use them.

**One specific thing to re-verify, because a prior dispatch of lens A and lens C got this wrong
once already:** confirm that `packet-authoring-contract` and `acceptance-verifier` genuinely do
NOT exist under `openspec/specs/` in this worktree (they should be **New**, not Modified), and that
only `sdd-apply` and `parent-feature-integration` exist (both **Modified**). If any lens draft
disagrees with that, or if this worktree somehow shows `packet-authoring-contract` or
`acceptance-verifier` as present, treat it as a hard stop — do not silently trust either the draft
or an unexpected directory. Those two names also exist as live specs on the unrelated, still
in-flight `feature/skill-provisioning-and-phase-specialist` branch; make sure nothing here was
contaminated from that branch.

### 3. Requirement arbitration

The three drafts each opened with `## Assumed requirements`. Compare them.

- **Lens A's requirement set is authoritative.** It is the lens that owned it.
- A scenario from lens B keyed to a requirement lens A did not name does not go into the delta.
  Record it under `## Requirement Divergence` in the notes with the name lens B used.
- A conflict lens C found against a requirement lens A classified as `ADDED` means the
  classification is wrong: it is `MODIFIED`. Lens C's evidence wins on classification, because it
  is the lens that opened the live spec. Record the correction in the notes.
- If lens B or lens C converged independently on lens A's requirement set, say so. Independent
  convergence is corroboration and is worth recording.
- If lens A's requirement set is refuted by a live spec you verified in step 2, do not silently
  substitute your own. That is a hard stop.

### 4. Assemble, do not concatenate

Write one file per capability at
`openspec/changes/deterministic-lucind-ai-orchestrator/specs/<capability>/spec.md`, following the
delta format: `## ADDED Requirements`, `## MODIFIED Requirements`, `## REMOVED Requirements`,
`## RENAMED Requirements`. Omit any section with no entries.

- Each requirement gets its statement from lens A and its scenarios from lens B, joined on the
  requirement name.
- Each `MODIFIED` requirement starts as lens C's verbatim full block, edited to the new behavior,
  with `(Previously: <one line>)` under the requirement text. **Never ship a partial MODIFIED
  block.** Archive replaces the live requirement with exactly what you write; a scenario you drop
  here is deleted from the capability.
- Each `REMOVED` and `RENAMED` requirement carries lens C's Reason and Migration.

### 5. Budget

The authored content of the delta tree MUST stay under 1800 words, **excluding scenarios copied
verbatim from a live spec inside a MODIFIED block**. The three drafts total roughly 3000 authored
words; compressing them to 1800 is what forces arbitration rather than stapling.

The exclusion is deliberate. Copied blocks are evidence, not prose, and truncating one to hit a
word count is a silent capability deletion. Compress your own writing; never compress a copied
block.

### 6. Coverage check

The delta tree must satisfy this repository's spec spine:

1. Every capability the proposal names has a file, at the right path for new versus existing
2. Every requirement classified `ADDED` / `MODIFIED` / `REMOVED` / `RENAMED`
3. Every requirement text carries an RFC 2119 keyword — MUST, SHALL, SHOULD, MAY
4. Every requirement carries at least one scenario
5. Scenarios cover happy path and edge cases, in GIVEN / WHEN / THEN form
6. Every `MODIFIED` block is the complete live block, edited — never partial
7. Every `REMOVED` and `RENAMED` requirement carries a Reason, and a Migration where consumers exist
8. No implementation detail — the delta says WHAT, not HOW

Anything no draft covered goes under `## Coverage Gaps` in the notes. Do not invent content to
fill a gap; report it.

## Output

### `openspec/changes/deterministic-lucind-ai-orchestrator/specs/<capability>/spec.md`

One file per capability. Contains only requirements whose citations you verified in step 2 and
which survive lens A's requirement set.

### `openspec/changes/deterministic-lucind-ai-orchestrator/spec-synthesis-notes.md`

Exactly these four sections, in this order. This file is what the orchestrator reads:

```markdown
# Spec Synthesis Notes: Deterministic lucind-ai Orchestrator

## Unresolved Contradictions

<Where two drafts assert incompatible things and neither the code nor the live
specs settle it. State both positions and what evidence each has. Do NOT pick —
this section is the escalation. "None" if there are none.>

## Coverage Gaps

<Spine items no draft covered — a requirement with no scenario, a capability with
no file, a removal with no migration. "None" if there are none.>

## Dropped Citations

<Every claim removed in step 2, with the citation that failed and what the code or
live spec actually says. "None" if there are none.>

## Requirement Divergence

<What lens B or lens C assumed that differed from lens A's requirement set, what
content that cost them, which classifications lens C's live-spec evidence
corrected, and where they converged independently. "None — all three converged"
if that is the case.>
```

## Out of scope

- Do NOT modify the three lens drafts. They are the record of what each lens produced.
- Do NOT write into `openspec/specs/`. Archive merges the delta there; this phase never touches
  the live tree.
- Do NOT write design, tasks, or any implementation code.
- Do NOT resolve an unresolved contradiction by choosing. Escalating it is the correct output.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`. This is a document synthesis
  lane.

## Allowed paths

`openspec/changes/deterministic-lucind-ai-orchestrator/specs/` and
`openspec/changes/deterministic-lucind-ai-orchestrator/spec-synthesis-notes.md` only.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-spec/` — the real `gentle-ai` spec skill. Check the delta tree
against the contract as written: the ADDED / MODIFIED / REMOVED / RENAMED format, the RFC 2119
rule, the one-scenario-minimum rule, and the MODIFIED copy-full-then-edit workflow. On those, the
skill wins over this packet's paraphrase, and the drift goes in `## Coverage Gaps`.

It does not win on execution. This packet sets the 1800-word authored budget and its
verbatim-block exclusion, the synthesis procedure, the notes file, and the done criteria. The
skill's Step 5 Engram persistence and Step 6 return block are superseded: your output is the delta
tree, the notes file, and `.lucind/result.json`.

Write nothing outside this repository.

## Using the lens citation manifests

Each lens draft ends with a `## Citation Manifest` table. Treat the union of the three manifests as
your **verification worklist, never as evidence**. A manifest row was written by the same lane that
made the claim, so a wrong citation arrives with a confident row beside it. The property that makes
this fan-out trustworthy is that you open every cited range yourself and check it against the real
code in this worktree. That property is not negotiable and the manifests do not relax it.

Each lens also ran a cheap pre-commit existence check over its own manifest (does the file exist,
is the line within range) before it committed. That check catches a citation that cannot possibly
be right; it says nothing about whether the range actually supports the claim. Do not treat a lens
having run that check as a reason to verify its citations any less thoroughly — it changes what
kind of wrong citation you are likely to find, not how many you must open.

What the manifests are for is speed without loss:

- **Deduplicate across the three.** Verify each unique citation exactly once, not once per prose
  mention.
- **Batch by file.** Open each cited file once and check every citation into it, instead of
  jumping between files in prose order.
- **Verify the claim, not the line's existence.** A row states what the lens asserts that range
  shows. A range that exists but does not support the claim is a dropped citation.
- **A citation in a lens's prose but missing from its manifest is still yours to verify.** An
  incomplete manifest does not shrink your obligation.

Record each entry's outcome — verified, dropped, or retargeted — in `## Dropped Citations`.

## Commit discipline (REQUIRED — two commits, not one)

**Commit the delta spec tree the moment it is written, before you begin the notes file.** Then
write the notes and commit them as a second conventional commit.

This is not bookkeeping. A synthesis lane that dies on the wall clock after its first commit leaves
finished work on its branch, recoverable by whoever re-dispatches it. A lane that dies before a
single end-of-run commit leaves an untracked file that `lucind-ai worktree cleanup` deletes without
warning. Two commits convert a timeout from lost work into resumable work.

Both commits are conventional, with no AI attribution. Check `git log -1 --format=%B` after each
one: some executors' commit wrappers append a `Co-authored-by:` trailer the message never
contained. Strip it if present.

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root, bracketing each commit. It is a deterministic
script, not a judge: it reports whether these facts hold; it does not decide whether your synthesis
is good, and it does not replace your own judgment against `## Done criteria` below.

**Right after the first commit** (the delta tree, before you start the notes file):

```
./lucind-lane-check.sh --file openspec/changes/deterministic-lucind-ai-orchestrator/specs/<capability>/spec.md --skip-result
```

Point `--file` at any one delta spec. What this run is for is the `git status --porcelain` check
(the default, not skipped): a FAIL means the first commit did not actually land the whole tree —
catch that before you start the notes file, not after the second commit buries it. Deliberately
**no `--budget`**: the 1800-word cap is tree-wide and this script counts one file at a time, so the
budget stays your own judgment rather than something a per-file count could mislead you about.

**After the second commit and writing `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/deterministic-lucind-ai-orchestrator/spec-synthesis-notes.md \
  --require-section "Unresolved Contradictions" --require-section "Coverage Gaps" \
  --require-section "Dropped Citations" --require-section "Requirement Divergence"
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence` in your envelope instead of
narrating the same facts in prose. The tree-wide word budget and the spine coverage are substantive
judgments the script cannot make — they stay yours.

## Done criteria

- [ ] **The delta spec tree was committed as its own commit before `spec-synthesis-notes.md` was
  started**, confirmed by the mid-flow `lucind-lane-check.sh` run reporting a clean
  `git status --porcelain`.
- [ ] **Every `file:line` citation surviving into the delta tree was opened and confirmed in this
  worktree**, and every dropped claim is listed under `## Dropped Citations`.
- [ ] **Every `MODIFIED` block matches the live requirement scenario for scenario**, with only the
  intended edits applied — verified by opening the live spec, not by trusting lens C.
- [ ] **Every requirement carries an RFC 2119 keyword and at least one GIVEN / WHEN / THEN
  scenario.**
- [ ] **The delta tree's authored content is under 1800 words excluding verbatim copied blocks**,
  and every spine item is satisfied or reported under `## Coverage Gaps`.
- [ ] **`spec-synthesis-notes.md` exists with exactly the four required sections**, each either
  populated or explicitly "None", confirmed by the final `lucind-lane-check.sh` run.
- [ ] **The work is committed with two conventional commits and no AI attribution**, confirmed by
  the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid
  `.lucind/result.json`.
- [ ] **`packet-authoring-contract` and `acceptance-verifier` are re-confirmed absent from
  `openspec/specs/` in this worktree** (New, not Modified), and neither delta spec file for them
  contains a MODIFIED block or cites the unrelated `skill-provisioning-and-phase-specialist`
  branch.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- The three `## Assumed requirements` blocks are mutually irreconcilable and the proposal does not
  choose between them. Write the notes file, leave `specs/` uncreated, and block.
- Lens A's requirement set is refuted by a live spec you verified. Do not substitute your own.
- A `MODIFIED` requirement's live block cannot be recovered from lens C or from
  `openspec/specs/`, so the block would have to be partial. Never write a partial block.
- One or more lens drafts is missing from this worktree.
- `packet-authoring-contract` or `acceptance-verifier` is present under `openspec/specs/` in this
  worktree (would indicate cross-branch contamination — see `## Context`).
- Satisfying the spine honestly would require exceeding the authored budget. Report which
  requirement forces it rather than silently overrunning or silently cutting.
- Satisfying one instruction in this packet would require violating another.

## Context

**Ground truth — verified directly before this packet was authored:**

- This Change (`deterministic-lucind-ai-orchestrator`) is anchored at `base_sha` `705cf49`
  (`main` tip) via `lucind-ai feature create`, deliberately isolated from the unrelated, still
  in-flight `feature/skill-provisioning-and-phase-specialist` branch (639 commits ahead of this
  base). The human explicitly asked for the two Changes to stay on separate isolated feature
  branches.
- The proposal's Capabilities section
  (`openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:21-28`) — corrected in commit
  `55a1543` after a prior dispatch of lens A and lens C hard-stopped on a real mismatch — names:
  **New** — `deterministic-orchestrator-contract`, `packet-authoring-contract`,
  `acceptance-verifier`. **Modified** — `sdd-apply`, `parent-feature-integration`.
- All three lens lanes ran against this corrected proposal and integrated clean at `55405b4`
  (`integrated_ids: spec-deterministic-lucind-ai-orchestrator-lens-a spec-deterministic-lucind-ai-orchestrator-lens-c`,
  plus `spec-deterministic-lucind-ai-orchestrator-lens-b` recovered separately via cherry-pick at
  `7926935` after an unrelated `internal/feature.TestConcurrentLeaseAcquisition` SQLITE_BUSY flake
  reverted its otherwise-clean lane).

**Decided already — do not re-litigate:** no new lifecycle states, scheduler/wave engine, flags,
routing mechanism, or replacement for existing Combine/Resolve/Check/bisect/CAS primitives
(`proposal.md:14-15`); executor/model/provider/profile selection and semantic approval/promotion
remain human-owned (`proposal.md:16`); no cross-fork coordination, global state, automatic main
promotion, or redesign of unrelated SDD flows (`proposal.md:17`); Rollback Plan
(`proposal.md:51-53`) commits to reverting skill/parity/runtime commits independently while
retaining existing packet, ledger, lifecycle, and CAS behavior, never migrating or rewriting prior
evidence.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. In `findings`, report the counts that matter:
citations verified, citations dropped, MODIFIED blocks confirmed complete, contradictions
escalated, coverage gaps. Report `done` only when every done-criterion carries evidence and every
hard stop is declared.
