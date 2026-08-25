---
id: archive-ultrafixer
executor: agy
routed_by: mechanical archival of a verified change, single lane, no fan-out
allowed_paths: ["openspec/specs/", "openspec/changes/ultrafixer/", "openspec/changes/archive/"]
feature: ultrafixer
parent_ref: refs/heads/feature/ultrafixer
base_sha: c1b0a9b8501dcb5a3da8c1ca597f59b4ffa48751
expected_parent_sha: c1b0a9b8501dcb5a3da8c1ca597f59b4ffa48751
---

# Packet archive-ultrafixer

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/archive-ultrafixer  ·  **Branch:** lucind/archive-ultrafixer

## Goal

Close the SDD cycle for `ultrafixer` mechanically: preserve every packet and result envelope the
session produced, merge the three delta specs into `openspec/specs/`, write the archive report,
and move the change folder into `openspec/changes/archive/`.

## Why this is one lane and not a fan-out

Archival is a filesystem operation, not a judgment. Three lenses would produce three opinions
about a `git mv`, and a synthesizer would compress an audit trail whose whole value is that
nothing was compressed.

## Why this is safe to dispatch now

Verification for `ultrafixer` reached a terminal verdict (`PASSED`, round 2, after a remediation
cycle) and the orchestrator accepted it. See `openspec/changes/ultrafixer/verify.md`. All 16
implementation tasks in `tasks.md` are checked `[x]`.

## Preconditions

- `openspec/changes/ultrafixer/` exists in this worktree.
- `openspec/changes/archive/2026-08-25-ultrafixer/` does not exist.
- `openspec/changes/ultrafixer/verify.md` exists and records a terminal verdict (`PASSED`).
- `openspec/changes/ultrafixer/tasks.md` exists, all tasks checked.
- Shell access is available.

## Required reading

1. `~/.claude/skills/sdd-archive/SKILL.md` — the real `gentle-ai` archive skill. It is the phase
   contract this lane executes; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/ultrafixer/verify.md` — the verdict and its history (round 1 BLOCKED with two
   confirmed violations, remediated, round 2 PASSED).
3. `openspec/changes/ultrafixer/tasks.md` — every checkbox (all 16 are `[x]`).
4. `openspec/changes/ultrafixer/specs/` — the three delta specs about to be merged:
   `defect-records`, `dependencies-defects`, `ultrafixer-dispatch`.
5. `openspec/specs/defect-records/`, `openspec/specs/dependencies-defects/`,
   `openspec/specs/ultrafixer-dispatch/` — none of these exist yet (verified: `ls openspec/specs/`
   has no matching directories). All three deltas are genuinely new capabilities — use the
   new-capability case in `## Procedure` step 3 (write title + `## Purpose` + `## Requirements`,
   carrying requirement/scenario bodies over exactly as written) for each.

## The mechanical copy rule

file content MUST NEVER pass through the model's Read/Write path to be copied.

- Copy and move with the shell only: `cp -R`, `mv`, or `git mv`.
- Never reproduce a file's content by reading it and writing it back.
- After every copy or move, run `diff -r` between source and destination as a mandatory readback.
- The verbatim `diff -r` output goes in the result envelope. Empty output is the only pass.

## Procedure

Do these in order.

### 1. Gates

- **Task completion**: all 16 tasks in `tasks.md` are already `[x]` — verify this yourself, don't
  trust this claim blindly.
- **Verification**: `verify.md`'s terminal verdict is `PASSED` (round 2) — confirm no CRITICAL
  issue remains open anywhere in the file (the round 1 BLOCKED section is historical record, both
  its confirmed violations are marked "Remediated:" with evidence, and round 2 independently
  re-confirmed both fixes — this is not a live CRITICAL blocker).
- **Missing artifacts**: proposal, spec, design, tasks, and verify all exist. Nothing missing.

### 2. Preserve the session's dispatch record

Primary root: `/home/lanzerdev/git_root/lucind-ai`. Copy `.lucind/packets/` and `.lucind/results/`
from there — but this primary root has packets/results from OTHER in-flight changes too (this
session dispatched packets for other work). Copy **only** files whose name starts with
`ultrafixer` or `verify-ultrafixer` or `archive-ultrafixer` (i.e. everything this Change's own
dispatch produced) — do not copy packets/results belonging to unrelated changes.

```
mkdir -p openspec/changes/ultrafixer/packets openspec/changes/ultrafixer/envelopes
for f in /home/lanzerdev/git_root/lucind-ai/.lucind/packets/ultrafixer*.md /home/lanzerdev/git_root/lucind-ai/.lucind/packets/verify-ultrafixer*.md /home/lanzerdev/git_root/lucind-ai/.lucind/packets/archive-ultrafixer.md; do
  [ -f "$f" ] && cp "$f" openspec/changes/ultrafixer/packets/
done
for f in /home/lanzerdev/git_root/lucind-ai/.lucind/results/ultrafixer*.json /home/lanzerdev/git_root/lucind-ai/.lucind/results/verify-ultrafixer*.json; do
  [ -f "$f" ] && cp "$f" openspec/changes/ultrafixer/envelopes/
done
```

Then verify each copied file's content matches its source exactly with `diff` per-file (not
`diff -r` on the whole `.lucind/packets`/`.lucind/results` directories, since those directories
also hold other changes' files this lane must not touch or claim to have preserved).

### 3. Merge delta specs into the live specs

Three new capabilities, all ADDED requirements, no existing live spec for any — write each as a
new full spec file (title, `## Purpose` paragraph, `## Requirements` heading, then carry the
requirement/scenario bodies from the delta over exactly as written, unreworded):

- `openspec/changes/ultrafixer/specs/defect-records/spec.md` → `openspec/specs/defect-records/spec.md`
- `openspec/changes/ultrafixer/specs/dependencies-defects/spec.md` → `openspec/specs/dependencies-defects/spec.md`
  (this one is a MODIFIED-capability delta in its heading, but since no live
  `openspec/specs/dependencies-defects/spec.md` exists, treat its content as the new full live
  spec — do not search for a non-existent prior version to merge into.)
- `openspec/changes/ultrafixer/specs/ultrafixer-dispatch/spec.md` → `openspec/specs/ultrafixer-dispatch/spec.md`

### 4. Write the archive report

`openspec/changes/ultrafixer/archive-report.md`, per the template's exact section structure. In
`## Verdict`, cover the full arc: round 1 BLOCKED (two confirmed violations, four non-blocking),
remediation, round 2 PASSED — don't just state the final verdict without the history, since that
history is the actual value of this verify cycle. In `## Follow-ups`, name the operator-facing
residual gap fixed directly by the orchestrator (dependencies-defects.md `defect decline` mention,
commit `0bb86af`) plus anything else `verify.md` or `tasks.md` still flags as open (check both).
In `## Gaps and Contradictions`, name the two real lucind-ai infrastructure bugs found and fixed
during this change's verify phase (reconcile `Renew` stale-approved supersession bug, fixed at
`77fca29`; reconcile stale-candidate-reuse-without-ancestry-check bug, fixed at `c0827fc`) — these
are infrastructure fixes to lucind-ai itself, not to ultrafixer's own design/code, but they're
directly relevant history for anyone reading this change's record later.

### 5. Move the change folder

`<archive-date>` = `2026-08-25`.

```
mkdir -p .lucind/archive-premove-snapshot
cp -R openspec/changes/ultrafixer .lucind/archive-premove-snapshot/ultrafixer
git mv openspec/changes/ultrafixer openspec/changes/archive/2026-08-25-ultrafixer
diff -r .lucind/archive-premove-snapshot/ultrafixer openspec/changes/archive/2026-08-25-ultrafixer
```

### 6. Commit

One conventional commit, no AI attribution.

## Out of scope

- Do NOT re-run verification, re-read the code for defects, or revisit the verdict.
- Do NOT fix code, tests, or documentation.
- Do NOT edit any artifact's content while moving it.
- Do NOT touch another change's folder under `openspec/changes/`.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`.
- Do NOT touch `feature/native-stability-campaign` or any packets/results belonging to it or to
  any other in-flight change.

## Allowed paths

`openspec/specs/`, `openspec/changes/ultrafixer/`, and `openspec/changes/archive/` only.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-archive/`.

**Read-only**: `/home/lanzerdev/git_root/lucind-ai/.lucind/packets/` and
`/home/lanzerdev/git_root/lucind-ai/.lucind/results/`, filtered to this change's own files as
described in step 2.

Write nothing outside this repository.

## Done criteria

- [ ] Every whole-file copy and every folder move ran through the shell, with the verbatim `diff`
      output for each in the result envelope, empty.
- [ ] Every packet and result envelope from this change's own dispatch is preserved under
      `openspec/changes/ultrafixer/packets/` and `.../envelopes/` (or its absence is recorded).
- [ ] Every delta requirement reached the live spec with its classification honored.
- [ ] `archive-report.md` exists with every section populated.
- [ ] The change folder is at `openspec/changes/archive/2026-08-25-ultrafixer/` and no longer at
      its original path.
- [ ] The work is committed with a conventional commit and no AI attribution.

## Hard stops

- An implementation task in `tasks.md` is unchecked and `## Context` grants no explicit
  reconciliation. (Not expected — verify yourself.)
- `verify.md` records a CRITICAL issue that is not marked remediated/re-confirmed.
- A `diff`/`diff -r` readback is non-empty.
- Shell access for the mechanical copy is unavailable.
- A MODIFIED delta block cannot be matched to a live requirement. (Not expected — all three
  capabilities are new, no live spec exists to conflict with.)
- The archive destination already exists.
- Satisfying one instruction in this packet would require violating another.

## Context

- **Change title**: Ultrafixer — an agy subagent that triages and repairs pre-existing defects
  across any lucind-ai-orchestrated project.
- **Primary repository root**: `/home/lanzerdev/git_root/lucind-ai`.
- **Archive date**: `2026-08-25`.
- **Terminal verify verdict**: `PASSED` (round 2, at candidate `95c426e`, after a remediation cycle
  that fixed two round-1 confirmed violations plus four non-blocking findings). Full history in
  `verify.md`.
- **Capability ids**: `defect-records`, `dependencies-defects`, `ultrafixer-dispatch` — all new,
  all-ADDED, no existing live spec for any.
- No partial-archive or checkbox-reconciliation authorization was given or needed — all 16 tasks
  are genuinely complete and verify genuinely passed.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. In `findings`, carry the verbatim `diff`/`diff -r`
output for every copy and move, the count of packets and envelopes preserved, and every follow-up
recorded in the report. Report `done` only when every done-criterion carries evidence and every
hard stop is declared.
