---
id: archive-skill-provisioning-and-phase-specialist
executor: agy
routed_by: mechanical archival of a verified change, single lane, no fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/specs", "openspec/changes/skill-provisioning-and-phase-specialist", "openspec/changes/archive"]
---

# Packet archive-skill-provisioning-and-phase-specialist

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/archive-skill-provisioning-and-phase-specialist  ·  **Branch:** lucind/archive-skill-provisioning-and-phase-specialist

## Goal

Close the SDD cycle for `skill-provisioning-and-phase-specialist` mechanically: preserve every packet and result envelope the session produced, merge the delta specs into `openspec/specs/`, write the archive report, and move the change folder into `openspec/changes/archive/`.

## Why this is one lane and not a fan-out

Archival is a filesystem operation, not a judgment. Three lenses would produce three opinions about a `git mv`, and a synthesizer would compress an audit trail whose whole value is that nothing was compressed. There is no word budget in this packet for the same reason: every byte it moves must arrive unchanged.

The one judgment archive does own — whether the change is *allowed* to close — is a gate with fixed inputs, checked once. It is in `## Procedure` step 1, and it either passes or the lane blocks.

## Why this is safe to dispatch now

Verification for `skill-provisioning-and-phase-specialist` reached a terminal PASS verdict (third dual-judgment pass, `verify.md` and machine-validated `verify-report.md` both present) and the orchestrator accepted it. All 36 `tasks.md` checkboxes are checked. Nothing in this lane re-decides that; it either finds the verdict clean and archives, or blocks.

## Preconditions

- `openspec/changes/skill-provisioning-and-phase-specialist/` exists in this worktree.
- `openspec/changes/archive/<archive-date>-skill-provisioning-and-phase-specialist/` does not exist.
- `openspec/changes/skill-provisioning-and-phase-specialist/verify.md` exists and records a terminal PASS verdict.
- `openspec/changes/skill-provisioning-and-phase-specialist/tasks.md` exists with all 36 items checked.
- Shell access is available. Without it this packet cannot run — see `## Hard stops`.

## Required reading

1. The real `gentle-ai` archive skill (delivered under `## Required skills`). It is the phase
   contract this lane executes; read it rather than trusting this packet's paraphrase of it. Its
   **Mechanical Copy Contract**, **Task Completion Gate**, and **Final-State Authority** sections
   are the parts this packet leans on hardest.
2. `openspec/changes/skill-provisioning-and-phase-specialist/verify.md` — the verdict and any issues it raised (PASS on the third pass; the document also records the earlier two BLOCKED passes and their resolutions — this is the full audit trail, not a contradiction).
3. `openspec/changes/skill-provisioning-and-phase-specialist/verify-report.md` — the machine-validated `gentle-ai.verify-result/v1` envelope (verdict: pass, 8/8 requirements, 34/34 scenarios).
4. `openspec/changes/skill-provisioning-and-phase-specialist/tasks.md` — every checkbox (all 36 checked).
5. `openspec/changes/skill-provisioning-and-phase-specialist/specs/` — the delta specs about to be merged (8 capabilities: acceptance-verifier, lane-execution, packet-authoring-contract, phase-specialist-dispatch, read-only-packet-schema, skill-derivation, skill-load-correspondence, skill-root-resolution).
6. The live `openspec/specs/<capability>/spec.md` for every capability those deltas touch — note that `acceptance-verifier`, `lane-execution`, `packet-authoring-contract`, `phase-specialist-dispatch`, `read-only-packet-schema`, `skill-derivation`, `skill-load-correspondence`, and `skill-root-resolution` may not all have existing live specs; check each and follow the new-capability procedure for any that don't.

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

- **Task completion**: all 36 items in `tasks.md` are already `- [x]`. Confirm this with a shell grep before proceeding; if any is found `- [ ]`, STOP and block.
- **Verification**: `verify.md`'s final (third-pass) verdict is PASS with zero outstanding CRITICAL issues. Confirm this by reading the document's final "Result:" line, not an earlier superseded section.
- **Missing artifacts**: a missing proposal, spec, or design is reported, not silently skipped. All of proposal.md, design.md, tasks.md, and the 8 delta specs should be present; report anything unexpectedly missing.

### 2. Preserve the session's dispatch record

`.lucind/` is gitignored (`.gitignore:2`), so the packets and envelopes that produced this change
exist only in the primary repository's working directory. This worktree's own `.lucind/` holds just
this lane's schema and result. Nothing preserves them but this step, and after the change folder is
archived there is no longer a natural home for them.

Read them from the primary root named in `## Context` and copy them in with the shell. Check each
source directory exists before copying:

```
if [ -d <primary-root>/.lucind/packets ]; then
  mkdir -p openspec/changes/skill-provisioning-and-phase-specialist/packets
  cp -R <primary-root>/.lucind/packets/.  openspec/changes/skill-provisioning-and-phase-specialist/packets/
  diff -r <primary-root>/.lucind/packets openspec/changes/skill-provisioning-and-phase-specialist/packets
else
  echo "no packets/ at <primary-root>/.lucind/packets — recording as absent"
fi

if [ -d <primary-root>/.lucind/results ]; then
  mkdir -p openspec/changes/skill-provisioning-and-phase-specialist/envelopes
  cp -R <primary-root>/.lucind/results/.  openspec/changes/skill-provisioning-and-phase-specialist/envelopes/
  diff -r <primary-root>/.lucind/results openspec/changes/skill-provisioning-and-phase-specialist/envelopes
else
  echo "no results/ at <primary-root>/.lucind/results — recording as absent"
fi
```

Copy every packet file whole, frontmatter included. If the primary root holds packets from other
changes, copy only this change's (this change's packet/result IDs are named across `proposal.md`,
`design.md`, `tasks.md`, `verify.md`, and this very packet — cross-reference by ID prefix; do not
guess). If a directory does not exist, record that in the report and continue.

### 3. Merge delta specs into the live specs

For each delta under `openspec/changes/skill-provisioning-and-phase-specialist/specs/<capability>/spec.md`:

- **ADDED** requirements are appended to `openspec/specs/<capability>/spec.md`.
- **MODIFIED** requirements **replace the entire live requirement block**, scenarios included.
- **REMOVED** requirements are deleted from the live spec.
- **RENAMED** requirements keep their content under the new name.
- A capability with no live spec becomes a new full spec file. Do NOT `cp` the delta straight into
  place: write the title, `## Purpose`, and `## Requirements` heading, then carry the requirement
  and scenario bodies over exactly as written (do not reword them). The one exception: if the delta
  is already authored as a complete spec (title, `## Purpose`, `## Requirements`), a plain
  `cp`/`diff -r` is correct and preferred over re-typing it.

Editing the live spec is the one place Read/Write is correct. The mechanical copy rule governs
whole-file copies and moves only.

### 4. Write the archive report

Write `openspec/changes/skill-provisioning-and-phase-specialist/archive-report.md` per the
`## Archive Report` template in the required skill. It must accurately reflect:
- The PASS verdict and its full history (two BLOCKED rounds with 10 total confirmed findings, all remediated and re-confirmed; a third round of 3 cosmetic non-blocking follow-ups also fixed).
- Capabilities added/modified with their requirement/scenario counts (8 requirements, 34 scenarios total across 8 capabilities — see `verify-report.md`'s compliance matrix).
- The dispatch record (lane count by phase, read from preserved packet frontmatter after step 2).
- Follow-ups: state explicitly there are none outstanding (all cosmetic items from `tasks.md` 7.1-7.3 were fixed, not deferred).
- Gaps and contradictions: none known; if the packet inventory in step 2 reveals anything unaccounted for, name it here rather than silently omitting it.

### 5. Move the change folder

Take the pre-move copy after step 4:

```
mkdir -p .lucind/archive-premove-snapshot
cp -R openspec/changes/skill-provisioning-and-phase-specialist .lucind/archive-premove-snapshot/skill-provisioning-and-phase-specialist
git mv openspec/changes/skill-provisioning-and-phase-specialist openspec/changes/archive/<archive-date>-skill-provisioning-and-phase-specialist
diff -r .lucind/archive-premove-snapshot/skill-provisioning-and-phase-specialist openspec/changes/archive/<archive-date>-skill-provisioning-and-phase-specialist
```

`<archive-date>` is today's date in `YYYY-MM-DD` — read the current date from the shell (`date +%F`), do not guess or hardcode.

### 6. Commit

One conventional commit, no AI attribution.

## Out of scope

- Do NOT re-run verification, re-read the code for defects, or revisit the verdict. A clean verdict is an input here.
- Do NOT fix code, tests, or documentation. A defect found now is a follow-up in the report (there should be none, per the PASS verdict).
- Do NOT edit any artifact's content while moving it. Content changes and archival never share a lane.
- Do NOT touch another change's folder under `openspec/changes/`.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`.

## Allowed paths

`openspec/specs/`, `openspec/changes/skill-provisioning-and-phase-specialist/`, and `openspec/changes/archive/` only.

## Allowed paths outside the repository

**Read-only**: The real `gentle-ai` archive skill and its `references/` (delivered under `## Required skills`).

**Read-only**: `<primary-root>/.lucind/packets/` and `<primary-root>/.lucind/results/`, named in `## Context`. This is the only source for the dispatch record in step 2, and it is read, never written.

Write nothing outside this repository.

## Done criteria

- [ ] **Every whole-file copy and every folder move ran through the shell**, with the verbatim `diff -r` output for each one in the result envelope, empty.
- [ ] **Every packet and result envelope from this change's dispatch is preserved under the change folder**, frontmatter included, or its absence is recorded in the report.
- [ ] **Every delta requirement reached the live spec with its classification honored**, and every MODIFIED block replaced the whole live block rather than part of it.
- [ ] **`archive-report.md` exists with every section populated**, and every follow-up is named there (should be "None").
- [ ] **The change folder is at `openspec/changes/archive/<archive-date>-skill-provisioning-and-phase-specialist/`** and no longer at its original path.
- [ ] **The work is committed with a conventional commit and no AI attribution**.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- An implementation task in `tasks.md` is unchecked.
- `verify.md`'s final verdict is not PASS, or records a CRITICAL issue.
- A `diff -r` readback is non-empty. Report the difference verbatim; never retry the copy over it.
- Shell access for the mechanical copy is unavailable. Never fall back to Read/Write copying.
- A MODIFIED delta block cannot be matched to a live requirement.
- The archive destination already exists.
- Satisfying one instruction in this packet would require violating another.

## Context

**Change title**: Skill Provisioning and the SDD Phase Specialist
**Primary repository root**: `/home/lanzerdev/git_root/lucind-ai`
**Archive date**: use `date +%F` at execution time
**Terminal verify verdict**: PASS (third dual-judgment pass, commit `225229b4fa078638f2fc5f1a1745898e3b3d36f9` for the mechanical check, `verify-report.md` machine-validated via `gentle-ai sdd-verify-validate` with evidence_revision `sha256:81f583d3a661be91ccfd065b877ead85217edadedcb8249ecdb75291a8cdb127`)
**Capability ids**: acceptance-verifier, lane-execution, packet-authoring-contract, phase-specialist-dispatch, read-only-packet-schema, skill-derivation, skill-load-correspondence, skill-root-resolution
**Human decision**: the orchestrator (and user) explicitly reviewed and approved archiving this change now, after the third-pass PASS verdict and the checkbox-reconciliation of tasks 2.3/2.4/3.2/3.3 (originally reopened by the first BLOCKED verify, confirmed fixed by remediation and re-verify, re-checked in commit `316e769`). No partial-archive authorization was given or needed — this is a full, clean archive.

## Required skills

- <sdd-archive>

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, carry the verbatim `diff -r` output for every copy and move, the count of packets and envelopes preserved, and every follow-up recorded in the report. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
