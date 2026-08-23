---
id: archive-control-room-ui-views
executor: agy
routed_by: mechanical archival of a verified change, single lane, no fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/specs/", "openspec/changes/control-room-ui-views/", "openspec/changes/archive/"]
feature: control-room-ui-views
parent_ref: refs/heads/control-room/control-room-ui-views
base_sha: 60621777b7846e9ab74c7f24190d9fa6503e08f4
expected_parent_sha: 60621777b7846e9ab74c7f24190d9fa6503e08f4
legacy_main: false
---

# Packet archive-control-room-ui-views

**Tier:** A (human merge)
**Worktree:** ../<repo>-worktrees/archive-control-room-ui-views  ·  **Branch:** lucind/archive-control-room-ui-views

## Goal

Close the SDD cycle for `control-room-ui-views` mechanically: preserve every packet and result envelope the session produced, merge the delta specs into `openspec/specs/`, write the archive report, and move the change folder into `openspec/changes/archive/`.

## Why this is one lane and not a fan-out

Archival is a filesystem operation, not a judgment. Three lenses would produce three opinions about a `git mv`, and a synthesizer would compress an audit trail whose whole value is that nothing was compressed. There is no word budget in this packet for the same reason: every byte it moves must arrive unchanged.

The one judgment archive does own — whether the change is *allowed* to close — is a gate with fixed inputs, checked once. It is in `## Procedure` step 1, and it either passes or the lane blocks.

## Why this is safe to dispatch now

Verification for `control-room-ui-views` reached a terminal verdict and the orchestrator accepted it. Nothing in this lane re-decides that; it either finds the verdict clean and archives, or blocks.

## Preconditions

- `openspec/changes/control-room-ui-views/` exists in this worktree.
- `openspec/changes/archive/2026-08-22-control-room-ui-views/` does not exist.
- `openspec/changes/control-room-ui-views/verify.md` exists and records a terminal verdict.
- `openspec/changes/control-room-ui-views/tasks.md` exists.
- Shell access is available. Without it this packet cannot run — see `## Hard stops`.

## Required reading

1. `~/.claude/skills/sdd-archive/SKILL.md` — the real `gentle-ai` archive skill. It is the phase
   contract this lane executes; read it rather than trusting this packet's paraphrase of it. Its
   **Mechanical Copy Contract**, **Task Completion Gate**, and **Final-State Authority** sections
   are the parts this packet leans on hardest.
2. `openspec/changes/control-room-ui-views/verify.md` — the verdict and any issues it raised.
3. `openspec/changes/control-room-ui-views/tasks.md` — every checkbox.
4. `openspec/changes/control-room-ui-views/specs/` — the delta specs about to be merged.
5. The live `openspec/specs/<capability>/spec.md` for every capability those deltas touch.

## The mechanical copy rule

This is the rule the whole packet exists to hold, quoted from the skill rather than paraphrased:
file content MUST NEVER pass through the model's Read/Write path to be copied.

- Copy and move with the shell only: `cp -R`, `mv`, or `git mv`.
- Never reproduce a file's content by reading it and writing it back. A model that truncates one
  byte while reporting success corrupts an audit trail silently, and nothing downstream will catch it.
- After every copy or move, run `diff -r` between source and destination as a mandatory readback.
- The verbatim `diff -r` output goes in the result envelope. Empty output is the only pass. A
  non-empty diff fails the phase. A skipped `diff -r` also fails the phase — self-report is never
  evidence.

## Procedure

Do these in order. Step 2 must precede step 5: once the change folder moves, there is nowhere left to copy into.

### 1. Gates

- **Task completion**: if any implementation task in `tasks.md` is still `- [ ]`, STOP and block.
  Do not sync specs and do not move anything. Reconciling stale checkboxes is permitted only when
  the orchestrator explicitly instructed it in `## Context` and the evidence proves completion; if
  you do it, record the exact reason in the archive report.
- **Verification**: a CRITICAL issue in `verify.md` blocks archive with no override.
- **Missing artifacts**: a missing proposal, spec, or design is reported, not silently skipped.
  Continue only if `## Context` records the human's explicit choice to archive partially, and name
  what was missing in the report.

### 2. Preserve the session's dispatch record

`.lucind/` is gitignored (`.gitignore:2`), so the packets and envelopes that produced this change
exist only in the primary repository's working directory. This worktree's own `.lucind/` holds just
this lane's schema and result. Nothing preserves them but this step, and after the change folder is
archived there is no longer a natural home for them.

Read them from the primary root named in `## Context` and copy them in with the shell. Check each
source directory exists before copying — an absent directory is a fact about the run, not a
failure, and `cp -R <src>/. <dst>/` on a missing source fails hard with no graceful path, so guard
it rather than let it abort the lane:

```
if [ -d <primary-root>/.lucind/packets ]; then
  mkdir -p openspec/changes/control-room-ui-views/packets
  cp -R <primary-root>/.lucind/packets/.  openspec/changes/control-room-ui-views/packets/
  diff -r <primary-root>/.lucind/packets openspec/changes/control-room-ui-views/packets
else
  echo "no packets/ at <primary-root>/.lucind/packets — recording as absent"
fi

if [ -d <primary-root>/.lucind/results ]; then
  mkdir -p openspec/changes/control-room-ui-views/envelopes
  cp -R <primary-root>/.lucind/results/.  openspec/changes/control-room-ui-views/envelopes/
  diff -r <primary-root>/.lucind/results openspec/changes/control-room-ui-views/envelopes
else
  echo "no results/ at <primary-root>/.lucind/results — recording as absent"
fi
```

Copy every packet file whole, frontmatter included — the frontmatter is the record of which
executor, which model, and which target each lane actually ran with.

If the primary root holds packets from other changes, copy only this change's. If a directory does
not exist, record that in the report and continue; an absent `results/` is a fact about the run,
not a failure of this lane. Do not create an empty `packets/` or `envelopes/` folder for a source
that never existed — an empty folder misrepresents "preserved nothing" as "preserved an empty set."

This supersedes the narrower `apply-bodies/` precedent
(`openspec/changes/archive/2026-08-21-sdd-fan-out-lens/apply-bodies/`), which preserved apply
packet bodies only. Every phase's packets are preserved here, and the envelopes with them.

### 3. Merge delta specs into the live specs

For each delta under `openspec/changes/control-room-ui-views/specs/<capability>/spec.md`:

- **ADDED** requirements are appended to `openspec/specs/<capability>/spec.md`.
- **MODIFIED** requirements **replace the entire live requirement block**, scenarios included. This
  is why the delta had to carry the whole block: what you write here is what the capability becomes.
- **REMOVED** requirements are deleted from the live spec.
- **RENAMED** requirements keep their content under the new name.
- A capability with no live spec becomes a new full spec file. Do NOT `cp` the delta straight into
  place: a delta is titled `# Delta for <capability>` and sections its requirements under
  `## ADDED Requirements`, but a live spec is titled `# <Capability> Specification`, carries a
  `## Purpose` paragraph, and sections its requirements under `## Requirements` — copying the delta
  verbatim produces a spec file inconsistent with the rest of `openspec/specs/`. Write the title,
  Purpose, and `## Requirements` heading, then carry the requirement and scenario bodies over
  exactly as written (do not reword them). The one exception: if the delta is already authored as
  a complete spec (title, `## Purpose`, `## Requirements`), a plain `cp`/`diff -r` is correct and
  preferred over re-typing it.

Editing the live spec — ADDED, MODIFIED, REMOVED, RENAMED, and the new-capability case above — is a
targeted structural edit, not a copy, so it is the one place Read/Write is correct. The mechanical
copy rule governs whole-file copies and moves — never confuse the two.

### 4. Write the archive report

Write `openspec/changes/control-room-ui-views/archive-report.md`. It is the terminal record of the cycle, so
it describes the state **at close**, not at any earlier snapshot:

```markdown
# Archive Report: <Change Title>

## Verdict
<the terminal verdict, and where it came from>

## What Shipped
<capabilities added or modified, with requirement and scenario counts>

## Dispatch Record
<lane count by phase and executor, read from the preserved packet frontmatter>

## Follow-ups
<every open item, or "None". A follow-up recorded here is the only thing that survives the move.>

## Gaps and Contradictions
<missing artifacts, reconciled checkboxes with their reason, and any claim that could not be
corroborated — both statements and their sources. Never resolved silently.>
```

Do not restate a `verify.md` "pending" or "blocked" claim as current fact. Work continues after a
snapshot is written; attribute snapshot claims to their source and time.

### 5. Move the change folder

Take the pre-move copy after step 4, once `archive-report.md` has been written. Snapshot it to
`.lucind/archive-premove-snapshot/`, not `/tmp` or anywhere outside the repository: `.lucind/` is
already this lane's scratch space (`## Return` writes `.lucind/result.json` there) and is
unconditionally exempt from the `allowed_paths` scope check, so it is the one safe in-repo place
for a throwaway copy that must not itself land under `openspec/`:

```
mkdir -p .lucind/archive-premove-snapshot
cp -R openspec/changes/control-room-ui-views .lucind/archive-premove-snapshot/control-room-ui-views
git mv openspec/changes/control-room-ui-views openspec/changes/archive/2026-08-22-control-room-ui-views
diff -r .lucind/archive-premove-snapshot/control-room-ui-views openspec/changes/archive/2026-08-22-control-room-ui-views
```

`2026-08-22` is `YYYY-MM-DD`. Because the snapshot is taken after step 4, `archive-report.md`
is present and identical on both sides of this diff — there is nothing to exclude. (This differs
from the underlying skill's own ordering, where the report is written after the move and so is
never part of that comparison at all.)

### 6. Commit

One conventional commit, no AI attribution.

## Out of scope

- Do NOT re-run verification, re-read the code for defects, or revisit the verdict. A clean verdict
  is an input here.
- Do NOT fix code, tests, or documentation. A defect found now is a follow-up in the report.
- Do NOT edit any artifact's content while moving it. Content changes and archival never share a lane.
- Do NOT touch another change's folder under `openspec/changes/`.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`.

## Allowed paths

`openspec/specs/`, `openspec/changes/control-room-ui-views/`, and `openspec/changes/archive/` only.

The change folder and the archive destination are named separately, rather than granting
`openspec/changes/`, so this lane cannot reach another in-flight change — which matters when two
changes are open at once.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-archive/` — the real `gentle-ai` archive skill and its
`references/`.

**Read-only**: `<primary-root>/.lucind/packets/` and `<primary-root>/.lucind/results/`, named in
`## Context`. This is the only source for the dispatch record in step 2, and it is read, never
written.

Precedence between skill and packet is **not symmetric**, and here it leans further toward the
skill than in any other template.

The skill is authority on *what archival must do and must never do*: the Mechanical Copy Contract,
the Task Completion Gate, the CRITICAL-blocks rule, the Final-State Authority ranking, and the
delta merge semantics. Where this packet paraphrases any of that and drifts, the skill wins and the
drift belongs in the archive report's `## Gaps and Contradictions`.

This packet is authority on *how this phase is being executed here*: that archival is one `agy`
lane rather than a fan-out, that the session's packets and envelopes are preserved before the move,
its allowed paths, and its done criteria. The skill's Engram persistence step and its return block
are superseded: your output is the repository changes above plus `.lucind/result.json`.

Write nothing outside this repository.

## Done criteria

- [ ] **Every whole-file copy and every folder move ran through the shell**, with the verbatim `diff -r` output for each one in the result envelope, empty. Spec merges — ADDED/MODIFIED/REMOVED/RENAMED, and a new-capability spec authored from an ADDED-style delta — are the named exception in `## Procedure` step 3 and go through Read/Write instead.
- [ ] **Every packet and result envelope from this change's dispatch is preserved under the change folder**, frontmatter included, or its absence is recorded in the report.
- [ ] **Every delta requirement reached the live spec with its classification honored**, and every MODIFIED block replaced the whole live block rather than part of it.
- [ ] **`archive-report.md` exists with every section populated**, and every follow-up is named there.
- [ ] **The change folder is at `openspec/changes/archive/2026-08-22-control-room-ui-views/`** and no longer at its original path.
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- An implementation task in `tasks.md` is unchecked and `## Context` grants no explicit reconciliation.
- `verify.md` records a CRITICAL issue.
- A `diff -r` readback is non-empty. Report the difference verbatim; never retry the copy over it.
- Shell access for the mechanical copy is unavailable. Never fall back to Read/Write copying.
- A MODIFIED delta block cannot be matched to a live requirement, so merging it would either duplicate a requirement or delete the wrong one.
- The archive destination already exists.
- Satisfying one instruction in this packet would require violating another.

## Context

**Change title**: Control Room UI Views

**Primary repository root**: `/home/lanzerdev/git_root/lucind-ai` — the source for `.lucind/packets/` and `.lucind/results/` in step 2. Note that `.lucind/packets/` does not exist in this project; packets live under `openspec/plans/control-room/<feature>/packets/`. Record that absence per step 2 rather than creating an empty folder, and preserve this change's packets from `openspec/plans/control-room/control-room-ui-views/packets/` instead. `.lucind/results/` does exist and holds 131 envelopes spanning all six control-room features; copy only this change's — `ui-features.json`, `ui-timeline.json`, and `api-gaps.json`. The other three apply lanes for this change (`ui-approvals`, `cli-wiring`, `verify-control-room-ui-views`) produced no envelope in that directory because their integration attempts did not promote; that is a fact about the run and belongs in the report, not a failure of this lane.

**Archive date**: 2026-08-22

**Terminal verify verdict**: PASSED, two rounds, recorded in `openspec/changes/control-room-ui-views/verify.md` with the frozen mechanical transcript in `verify-mechanical.log` (final run `fea7459f7ab222243aa0ba51b7d39a430056fdef`, exit 0). No CRITICAL issue.

**Capability ids** under `openspec/changes/control-room-ui-views/specs/`: `approvals-web-ui`, `batch-wave-view`, `feature-lease-monitor`, `lane-envelope-inspector`, `reconciliation-workspace`.

**Checkbox reconciliation — explicitly authorized.** The human reviewed `tasks.md`'s eleven unchecked boxes and directed that the genuine gaps be closed by a remediation lane rather than reconciled away. That lane (`api-gaps`, `fea7459`) shipped, and tasks 1.1, 1.2, 2.1, 2.2, 3.1, and 5.1 are now checked with the code to back them.

Two categories remain unchecked, both deliberately, and neither blocks this archive:

1. **Task 4.1 is marked `[~] SUPERSEDED`, not `[x]`.** It called for five per-panel client fetches; the implementation converged on one composed `/api/state` endpoint plus an SSE stream, which supersedes it. Do not treat `[~]` as an unchecked implementation task. Name it in the report's Gaps and Contradictions with this reason.

2. **The four Open Questions** at the end of `tasks.md` are design questions (reconcile UI POST surface, lease countdown source, overlap evidence rendering, `NewHandler` model injection), not implementation tasks. Carry them into the report's Follow-ups verbatim.

**Dispatch record for the report.** Every lane in this change ran on `agy` / `gemini-3.7-flash-high`. Planning ran 18 agy lens lanes and 6 cursor-agent synthesis lanes across five phases. Apply ran four lanes: `ui-features` and `ui-timeline` promoted on first attempt; `ui-approvals` and `cli-wiring` were marked `deviated` because their packets' `allowed_paths` named files that do not exist in this repository, and were hand-integrated after their commits were inspected and found valid. Verify ran one read-only lane. Remediation ran one lane, `api-gaps`, promoted. Read the preserved packet frontmatter for the authoritative per-lane record rather than restating this paragraph.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, carry the verbatim `diff -r` output for every copy and move, the count of packets and envelopes preserved, and every follow-up recorded in the report. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
