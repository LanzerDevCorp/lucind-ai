---
id: spec-skill-provisioning-and-phase-specialist-synthesis
executor: cursor-agent
routed_by: synthesis of three parallel spec lenses into the canonical delta spec tree
model: cursor-grok-4.6-high
allowed_paths: ["openspec/changes/skill-provisioning-and-phase-specialist/specs", "openspec/changes/skill-provisioning-and-phase-specialist/spec-synthesis-notes.md"]
---

# Packet spec-skill-provisioning-and-phase-specialist-synthesis

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/spec-skill-provisioning-and-phase-specialist-synthesis  ·
**Branch:** lucind/spec-skill-provisioning-and-phase-specialist-synthesis

## Goal

Read the three spec lens drafts for `skill-provisioning-and-phase-specialist`, verify their claims
against the real code and the real live specs, arbitrate where they disagree, and produce the
canonical delta spec tree under
`openspec/changes/skill-provisioning-and-phase-specialist/specs/` plus a separate synthesis notes
file recording everything that did not make it in and why.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you — the
orchestrator reads only your notes file. Anything you accept without checking ships into
`openspec/specs/` at archive time.

## Why this is safe to dispatch now

All three lens lanes have reached terminal status and integrated. This worktree is branched from
the integrated result, so `spec-lens-a.md`, `spec-lens-b.md`, and `spec-lens-c.md` are all present
here. Lens worktrees could not see each other; this one sees all three.

## Preconditions

- `spec-lens-a.md`, `spec-lens-b.md`, and `spec-lens-c.md` all exist in this worktree.
- `openspec/changes/skill-provisioning-and-phase-specialist/specs/` does not yet exist.
- The proposal for `skill-provisioning-and-phase-specialist` is present and accepted.

## Correct target path — read before step 4 (Assemble)

**Every capability file — new and modified alike — lands under
`openspec/changes/skill-provisioning-and-phase-specialist/specs/<capability>/spec.md`. None land
directly under `openspec/specs/`.** This phase never touches the live tree; archive merges the
delta there later.

`spec-lens-a.md`'s `## Capability Map` "Target file" column is wrong for the four new capabilities
— it points `skill-derivation`, `skill-root-resolution`, `skill-load-correspondence`, and
`phase-specialist-dispatch` at `openspec/specs/<capability>/spec.md` (the live tree). Confirmed
against a real archived example in this repository:
`openspec/changes/archive/2026-08-24-lane-status-observability/specs/dispatched-packet-body/spec.md`
and `.../orphan-lane-reconciliation/spec.md` are new capabilities from that change, and both were
authored under the **change** directory's `specs/`, not `openspec/specs/`, during the spec phase —
`openspec/specs/dispatched-packet-body/` did not exist until archive. Do not follow lens A's Target
file column for the four new capabilities; write all eight files under this change's own `specs/`
tree. This correction does not affect lens A's requirement text or classification, only where you
write the file.

## What each lens owns

This fan-out used the standard aspect split, not a capability split.

| Draft | Owns |
|---|---|
| `spec-lens-a.md` | Capability map; requirement statements; ADDED / MODIFIED classification (four new capabilities, four modified) |
| `spec-lens-b.md` | Given/When/Then scenarios; coverage table; untestable assertions |
| `spec-lens-c.md` | Live-spec inventory and conflicts; verbatim MODIFIED full blocks; no removals or renames in this change |

All three also emit `## Open Questions`. Merge and deduplicate them.

## Required procedure

Do these in order. Skipping step 2 or step 3 makes the output worthless regardless of how good it
reads.

### 1. Read all three drafts in full

Do not begin writing until you have read all three.

### 2. Citation verification pass

Every `file:line` citation in every draft is a claim about this repository. Open each one in this
worktree and confirm it says what the draft says it says. Lens C's live-spec citations matter most:
a MODIFIED block copied from a requirement that is not there is a silent deletion at archive time.

- A citation that resolves and supports the claim: keep it.
- A citation that does not resolve, points at unrelated code, or does not support the claim: **drop
  the claim from the delta spec** and record it under `## Dropped Citations` in the notes with what
  you found instead.

A lens draft is evidence, not authority. You have the code and the live specs; use them.

**Known-wrong from the propose phase — confirm the drafts avoided them, and if any resurfaced, drop
without re-verifying (already confirmed wrong in `proposal-synthesis-notes.md`):**
`internal/ledger/authoring.go:23` (not the `Contract json.RawMessage` line — that is line 26);
`internal/packetauthor/compile.go:49-65` as an existing budget-enforcement citation (no budget
arithmetic exists there today — this range only checks version, goal/criteria/stops, and result
path/schema); `internal/accept/authoring_evidence_test.go:56-127` as a schema/struct reflection pin
(it asserts correspondence, not a `result.Envelope`↔`result.schema.json` reflection pin — that seam
is `internal/result/schema_test.go`); `internal/skillcontent/skillcontent.go:90-100` as the full
`HashDir` range (the walk starts at line 75; the correct full range is `73-100`).

### 3. Requirement arbitration

The three drafts each opened with `## Assumed requirements`. Compare them.

- **Lens A's requirement set is authoritative.** It is the lens that owned it.
- A scenario from lens B keyed to a requirement lens A did not name does not go into the delta.
  Record it under `## Requirement Divergence` in the notes with the name lens B used.
- A conflict lens C found against a requirement lens A classified as `ADDED` means the classification
  is wrong: it is `MODIFIED`. Lens C's evidence wins on classification, because it is the lens that
  opened the live spec. Record the correction in the notes.
- If lens B or lens C converged independently on lens A's requirement set, say so. Independent
  convergence is corroboration and is worth recording.
- If lens A's requirement set is refuted by a live spec you verified in step 2, do not silently
  substitute your own — only the classification may move; the requirement wording stays lens A's
  unless it is factually wrong about a citation, in which case fix the citation and record it under
  `## Dropped Citations`.

### 4. Assemble, do not concatenate

Write one file per capability at
`openspec/changes/skill-provisioning-and-phase-specialist/specs/<capability>/spec.md` — see the
"Correct target path" section above; do not follow lens A's Target-file column for new capabilities.

- New capabilities (`skill-derivation`, `skill-root-resolution`, `skill-load-correspondence`,
  `phase-specialist-dispatch`): a full spec (`## Purpose` + `## Requirements`), not a delta.
- Existing/modified capabilities (`packet-authoring-contract`, `lane-execution`,
  `acceptance-verifier`, `read-only-packet-schema`): a delta (`## ADDED Requirements`,
  `## MODIFIED Requirements`, `## REMOVED Requirements`, `## RENAMED Requirements`). Omit any section
  with no entries — spec-lens-c found no removals or renames in this change, so those two sections
  should be absent, not present-and-empty.

Each requirement gets its statement from lens A and its scenarios from lens B, joined on the
requirement name. Each `MODIFIED` requirement starts as lens C's verbatim full block, edited to the
new behavior, with `(Previously: <one line>)` under the requirement text. **Never ship a partial
MODIFIED block.** Archive replaces the live requirement with exactly what you write; a scenario you
drop here is deleted from the capability.

### 5. Budget

The authored content of the delta tree MUST stay under 1800 words, **excluding scenarios copied
verbatim from a live spec inside a MODIFIED block**. The three drafts total roughly 2400 authored
words; compressing them to 1800 is what forces arbitration rather than stapling.

The exclusion is deliberate. Copied blocks are evidence, not prose, and truncating one to hit a word
count is a silent capability deletion. Compress your own writing; never compress a copied block.

### 6. Coverage check

The delta tree must satisfy this repository's spec spine:

1. Every capability the proposal names has a file, at the right path (all eight under this change's
   `specs/`, per the correction above — four new full specs, four modified deltas)
2. Every requirement classified `ADDED` / `MODIFIED` / `REMOVED` / `RENAMED`
3. Every requirement text carries an RFC 2119 keyword — MUST, SHALL, SHOULD, MAY
4. Every requirement carries at least one scenario
5. Scenarios cover happy path and edge cases, in GIVEN / WHEN / THEN form
6. Every `MODIFIED` block is the complete live block, edited — never partial
7. No implementation detail — the delta says WHAT, not HOW
8. None of the proposal's five still-open questions (ad-hoc authoring surface shape; archive/
   ultrafixer child skills; budget-default override; specialist CLI/profile granularity;
   `lucind.yaml` filename collision) is answered by a requirement or scenario as if decided. If any
   lens draft quietly picked one, correct it back to open, mechanism-agnostic phrasing and record the
   correction under `## Coverage Gaps`.

Anything no draft covered goes under `## Coverage Gaps` in the notes. Do not invent content to fill
a gap; report it.

## Output

### `openspec/changes/skill-provisioning-and-phase-specialist/specs/<capability>/spec.md`

Eight files, one per capability. Contains only requirements whose citations you verified in step 2
and which survive lens A's requirement set.

### `openspec/changes/skill-provisioning-and-phase-specialist/spec-synthesis-notes.md`

Exactly these four sections, in this order. This file is what the orchestrator reads:

```markdown
# Spec Synthesis Notes: Skill Provisioning and the SDD Phase Specialist

## Unresolved Contradictions

<Where two drafts assert incompatible things and neither the code nor the live specs settle it.
State both positions and what evidence each has. Do NOT pick — this section is the escalation.
"None" if there are none.>

## Coverage Gaps

<Spine items no draft covered — a requirement with no scenario, a capability with no file, a
still-open proposal question a draft answered as if decided. "None" if there are none.>

## Dropped Citations

<Every claim removed in step 2, with the citation that failed and what the code or live spec
actually says. Include the target-path correction from spec-lens-a.md even though it is a path
error rather than a factual citation error — record it here for the record. "None" if there are
none besides that.>

## Requirement Divergence

<What lens B or lens C assumed that differed from lens A's requirement set, what content that cost
them, which classifications lens C's live-spec evidence corrected, and where they converged
independently. "None — all three converged" if that is the case.>
```

## Out of scope

- Do NOT modify the three lens drafts. They are the record of what each lens produced.
- Do NOT write into `openspec/specs/`. Archive merges the delta there; this phase never touches the
  live tree.
- Do NOT write design, tasks, or any implementation code.
- Do NOT resolve an unresolved contradiction by choosing. Escalating it is the correct output.
- Do NOT answer any of the proposal's still-open questions. A requirement that hardcodes one is a
  defect to fix, not content to preserve.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`. This is a document synthesis
  lane.

## Allowed paths

`openspec/changes/skill-provisioning-and-phase-specialist/specs/` and
`openspec/changes/skill-provisioning-and-phase-specialist/spec-synthesis-notes.md` only.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-spec/` — the real `gentle-ai` spec skill. Check the delta tree
against the contract as written: the ADDED/MODIFIED/REMOVED/RENAMED format, the RFC 2119 rule, the
one-scenario-minimum rule, and the MODIFIED copy-full-then-edit workflow. On those, the skill wins
over this packet's paraphrase, and the drift goes in `## Coverage Gaps`.

It does not win on execution. This packet sets the 1800-word authored budget and its verbatim-block
exclusion, the synthesis procedure, the notes file, and the done criteria. The skill's own Engram
persistence and return-block steps are superseded: your output is the delta tree, the notes file,
and `.lucind/result.json`.

Write nothing outside this repository.

## Using the lens citation manifests

Each lens draft ends with a `## Citation Manifest` table. Treat the union of the three manifests as
your **verification worklist, never as evidence**. A manifest row was written by the same lane that
made the claim, so a wrong citation arrives with a confident row beside it. The property that makes
this fan-out trustworthy is that you open every cited range yourself and check it against the real
code in this worktree. That property is not negotiable and the manifests do not relax it.

What the manifests are for is speed without loss:

- **Deduplicate across the three.** Verify each unique citation exactly once, not once per prose
  mention.
- **Batch by file.** Open each cited file once and check every citation into it, instead of jumping
  between files in prose order.
- **Verify the claim, not the line's existence.** A row states what the lens asserts that range
  shows. A range that exists but does not support the claim is a dropped citation.
- **A citation in a lens's prose but missing from its manifest is still yours to verify.** An
  incomplete manifest does not shrink your obligation.

Record each entry's outcome — verified, dropped, or retargeted — in `## Dropped Citations`.

## Commit discipline (REQUIRED — two commits, not one)

**Commit the delta spec tree the moment it is written, before you begin the notes file.** Then write
the notes and commit them as a second conventional commit.

This is not bookkeeping. A synthesis lane that dies on the wall clock after its first commit leaves
finished work on its branch, recoverable by whoever re-dispatches it. A lane that dies before a
single end-of-run commit leaves an untracked file that `lucind-ai worktree cleanup` deletes without
warning — which has already cost this project one full synthesis run. Two commits convert a timeout
from lost work into resumable work.

Both commits are conventional, with no AI attribution. Check `git log -1 --format=%B` after each
one: some executors' commit wrappers append a `Co-authored-by:` trailer the message never contained.
Strip it if present.

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root, bracketing each commit. It is a deterministic
script, not a judge: it reports whether these facts hold; it does not decide whether your synthesis
is good, and it does not replace your own judgment against `## Done criteria` below.

**Right after the first commit** (the delta tree, before you start the notes file):

```
./lucind-lane-check.sh --file openspec/changes/skill-provisioning-and-phase-specialist/specs/skill-derivation/spec.md --skip-result
```

Point `--file` at any one delta spec. What this run is for is the `git status --porcelain` check
(the default, not skipped): a FAIL means the first commit did not actually land the whole tree —
catch that before you start the notes file, not after the second commit buries it. Deliberately
**no `--budget`**: the 1800-word cap is tree-wide and this script counts one file at a time, so the
budget stays your own judgment rather than something a per-file count could mislead you about.

**After the second commit and writing `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/skill-provisioning-and-phase-specialist/spec-synthesis-notes.md \
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
- [ ] **All eight capability files exist under
      `openspec/changes/skill-provisioning-and-phase-specialist/specs/`, none under
      `openspec/specs/` directly** — confirmed by `git status --porcelain` / the commit diff, not by
      trusting lens A's Target-file column.
- [ ] **Every `MODIFIED` block matches the live requirement scenario for scenario**, with only the
      intended edits applied — verified by opening the live spec, not by trusting lens C.
- [ ] **Every requirement carries an RFC 2119 keyword and at least one GIVEN / WHEN / THEN
      scenario.**
- [ ] **None of the proposal's five still-open questions is answered as if decided** anywhere in the
      delta tree.
- [ ] **The delta tree's authored content is under 1800 words excluding verbatim copied blocks**,
      and every spine item is satisfied or reported under `## Coverage Gaps`.
- [ ] **`spec-synthesis-notes.md` exists with exactly the four required sections**, each either
      populated or explicitly "None", confirmed by the final `lucind-lane-check.sh` run.
- [ ] **The work is committed with two conventional commits and no AI attribution**, confirmed by
      the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid
      `.lucind/result.json`.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- The three `## Assumed requirements` blocks are mutually irreconcilable and the proposal does not
  choose between them. Write the notes file, leave `specs/` uncreated, and block.
- Lens A's requirement set is refuted by a live spec you verified. Do not substitute your own.
- A `MODIFIED` requirement's live block cannot be recovered from lens C or from `openspec/specs/`,
  so the block would have to be partial. Never write a partial block.
- One or more lens drafts is missing from this worktree.
- Satisfying the spine honestly would require exceeding the authored budget. Report which
  requirement forces it rather than silently overrunning or silently cutting.
- Satisfying one instruction in this packet would require violating another.

## Context

**Change**: Skill Provisioning and the SDD Phase Specialist. Chosen candidate: Candidate 1
(`openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:3`). Read the full proposal
first — it is committed in this worktree, along with `proposal-synthesis-notes.md`.

**Four new capabilities, four modified** (`proposal.md:30-34`): New — `skill-derivation`,
`skill-root-resolution`, `skill-load-correspondence`, `phase-specialist-dispatch`. Modified —
`packet-authoring-contract`, `lane-execution`, `acceptance-verifier`, `read-only-packet-schema`.

**Verified by the orchestrator before dispatch** (you should still re-verify independently per step
2, but these are not expected to surprise you):

- The four live spec files modified capabilities target exist and their cited requirement blocks are
  real: `openspec/specs/packet-authoring-contract/spec.md:9` is `### Requirement: Versioned Contract
  and Late Target Binding`; `openspec/specs/lane-execution/spec.md:104` is `### Requirement: Frozen
  Authored Candidate Evidence`; `openspec/specs/acceptance-verifier/spec.md:30` is `### Requirement:
  Fail-Closed Mechanical Criteria`; `openspec/specs/read-only-packet-schema/spec.md:84` is
  `### Requirement: Extended packet frontmatter parsing`.
- `internal/accept/accept.go:275-286` today has no `LaneRole` or `RequiredSkills` fields in its
  decode struct, consistent with all three lenses' MODIFIED framing.
- `internal/result/result.go:103-116` (`Envelope`) has no `SkillsLoaded` field yet;
  `internal/result/result.schema.json:5` sets `"additionalProperties": false` — consistent with the
  ADDED `skill-load-correspondence` requirement.
- The real archived example confirming the target-path convention:
  `openspec/changes/archive/2026-08-24-lane-status-observability/specs/dispatched-packet-body/spec.md`
  (new capability, full `## Purpose`/`## Requirements` spec, written under the **change's own**
  `specs/` during that change's spec phase, not under `openspec/specs/`).

**Decided already — do not re-open, re-offer, or widen:**

1. No `AuthoringEvidence` struct field addition and no `AuthoringEvidenceVersion` bump.
2. No SQLite migration; no removal or rename of any existing requirement.
3. The specialist never intercepts or wraps `gentle-ai`.
4. External skill content hashing (`HashDir`) is observation-only, never a blocking gate.
5. Single PR, `single-pr` delivery strategy.

**The proposal's five open questions remain OPEN — do not let any survive into the delta tree as a
decided requirement:** ad-hoc authoring surface shape; missing archive/ultrafixer child skills;
skill-budget default-of-3 override; specialist CLI/profile granularity; `lucind.yaml` filename
collision (`proposal.md:169-174`).

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. In `findings`, report the counts that matter: citations
verified, citations dropped, classification corrections made, MODIFIED blocks confirmed complete,
contradictions escalated, coverage gaps, and confirmation that all eight files landed under the
change's own `specs/` tree. Report `done` only when every done-criterion carries evidence and every
hard stop is declared.
