---
id: archive-agentic-phase-specialist
executor: agy
routed_by: mechanical archival of a verified change, single lane, no fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/specs", "openspec/changes/agentic-phase-specialist", "openspec/changes/archive"]
---

# Packet archive-agentic-phase-specialist

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/archive-agentic-phase-specialist  ·  **Branch:** lucind/archive-agentic-phase-specialist

## Goal

Close the SDD cycle for `agentic-phase-specialist` mechanically: preserve every packet and result envelope the session produced, merge the delta specs into `openspec/specs/`, write the archive report, and move the change folder into `openspec/changes/archive/`.

## Why this is one lane and not a fan-out

Archival is a filesystem operation, not a judgment. Three lenses would produce three opinions about a `git mv`, and a synthesizer would compress an audit trail whose whole value is that nothing was compressed. There is no word budget in this packet for the same reason: every byte it moves must arrive unchanged.

The one judgment archive does own — whether the change is *allowed* to close — is a gate with fixed inputs, checked once. It is in `## Procedure` step 1, and it either passes or the lane blocks.

## Why this is safe to dispatch now

Verification for `agentic-phase-specialist` reached a terminal verdict (PASSED) and the orchestrator accepted it. Nothing in this lane re-decides that; it either finds the verdict clean and archives, or blocks.

## Preconditions

- `openspec/changes/agentic-phase-specialist/` exists in this worktree.
- `openspec/changes/archive/2026-08-30-agentic-phase-specialist/` does not exist.
- `openspec/changes/agentic-phase-specialist/verify.md` exists and records a terminal verdict (PASSED).
- `openspec/changes/agentic-phase-specialist/tasks.md` exists.
- Shell access is available. Without it this packet cannot run — see `## Hard stops`.

## Required reading

1. The real `gentle-ai` archive skill at `/home/lanzerdev/.claude/skills/sdd-archive/SKILL.md` — it is the phase contract this lane executes. Its **Mechanical Copy Contract**, **Task Completion Gate**, and **Final-State Authority** sections are the parts this packet leans on hardest.
2. `openspec/changes/agentic-phase-specialist/verify.md` — the verdict (PASSED) and residual findings.
3. `openspec/changes/agentic-phase-specialist/tasks.md` — all checkboxes (1.1-1.4, 2.1-RED, 2.2, 3.1-RED, 3.2 completed; 4.1 intentionally unchecked as human follow-up).
4. `openspec/changes/agentic-phase-specialist/specs/` — the delta specs about to be merged (4 capabilities: phase-specialist-dispatch MODIFIED, sdd-planning-fan-out MODIFIED, acceptance-verifier MODIFIED, phase-verdict-reporting ADDED).
5. The live `openspec/specs/<capability>/spec.md` for every capability those deltas touch.

## The mechanical copy rule

This is the rule the whole packet exists to hold, quoted from the skill rather than paraphrased:
file content MUST NEVER pass through the model's Read/Write path to be copied.

- Copy and move with the shell only: `cp -R`, `mv`, or `git mv`.
- Never reproduce a file's content by reading it and writing it back. A model that truncates one byte while reporting success corrupts an audit trail silently, and nothing downstream will catch it.
- After every copy or move, run `diff -r` between source and destination as a mandatory readback.
- The verbatim `diff -r` output goes in the result envelope. Empty output is the only pass. A non-empty diff fails the phase. A skipped `diff -r` also fails the phase — self-report is never evidence.

## Procedure

Do these in order. Step 2 must precede step 5: once the change folder moves, there is nowhere left to copy into.

### 1. Gates

- **Task completion**: All implementation tasks in `tasks.md` are checked [x]. Task 4.1 is intentionally unchecked as a human follow-up (pasting text into `~/.claude/skills/sdd-*/SKILL.md`, outside repository scope). This is recorded in the orchestrator's launch prompt and confirmed in the `tasks.md` comments. Gate PASSES.
- **Verification**: `verify.md` shows PASSED with no CRITICAL issues. Gate PASSES.
- **Missing artifacts**: All required artifacts exist: proposal.md, specs/, design.md, tasks.md, verify.md. No partial archive. Gate PASSES.

### 2. Preserve the session's dispatch record

`.lucind/` is gitignored (`.gitignore:2`), so the packets and envelopes that produced this change exist only in the primary repository's working directory. Copy them mechanically with the shell:

```bash
# Preserve packets from the primary root
primary_root="/home/lanzerdev/git_root/lucind-ai"

if [ -d "$primary_root/.lucind/packets" ]; then
  mkdir -p openspec/changes/agentic-phase-specialist/packets
  cp -R "$primary_root/.lucind/packets/." openspec/changes/agentic-phase-specialist/packets/
  diff -r "$primary_root/.lucind/packets" openspec/changes/agentic-phase-specialist/packets
else
  echo "no packets/ at $primary_root/.lucind/packets — recording as absent"
fi

# Preserve result envelopes from the primary root
if [ -d "$primary_root/.lucind/results" ]; then
  mkdir -p openspec/changes/agentic-phase-specialist/envelopes
  cp -R "$primary_root/.lucind/results/." openspec/changes/agentic-phase-specialist/envelopes/
  diff -r "$primary_root/.lucind/results" openspec/changes/agentic-phase-specialist/envelopes
else
  echo "no results/ at $primary_root/.lucind/results — recording as absent"
fi
```

Copy every packet file whole, frontmatter included. If a directory does not exist, record that in the report and continue.

### 3. Merge delta specs into the live specs

For each delta under `openspec/changes/agentic-phase-specialist/specs/<capability>/spec.md`, apply it to the corresponding live spec. Use Read/Write for targeted structural edits:

- **ADDED** requirements are appended to `openspec/specs/<capability>/spec.md`.
- **MODIFIED** requirements **replace the entire live requirement block**, scenarios included.
- **REMOVED** requirements are deleted from the live spec.
- For **phase-verdict-reporting** (ADDED): a new capability becomes a new full spec file `openspec/specs/phase-verdict-reporting/spec.md`.

Match requirements by name and preserve all OTHER requirements that aren't in the delta. Maintain proper Markdown formatting and heading hierarchy.

### 4. Write the archive report

Write `openspec/changes/agentic-phase-specialist/archive-report.md`. It is the terminal record of the cycle:

```markdown
# Archive Report: Agentic Phase Specialist

## Verdict
PASSED. Unanimous pass from dual qualitative judgment. Per verify.md, both judges confirmed all done criteria met and no hard stops fired. Verified against merged apply commit 19d5f01c0ae5c65b12cede90d47804ec578568b7.

## What Shipped
Four capabilities:
1. phase-specialist-dispatch: MODIFIED (clarified Specialist dispatch sequencing)
2. sdd-planning-fan-out: MODIFIED (moved synthesis-note review to Specialist)
3. acceptance-verifier: MODIFIED (added SDD-phase gating to acceptance checks)
4. phase-verdict-reporting: ADDED (new capability for structured phase verdicts)

## Dispatch Record
[preserved from .lucind/packets and .lucind/results]

## Follow-ups
1. HUMAN ACTION REQUIRED: Paste Specialist-behavior text into ~/.claude/skills/sdd-*/SKILL.md (out-of-repo, see design.md:102-106)
2. Doc hygiene: Refresh docs/adr/0002-phase-specialist-authority-and-scoped-checks.md and docs/sdd-phase-specialist.md's stale "Not yet done" sections (confirmed by verify.md finding 4)

## Gaps and Contradictions
None. All artifacts present. All implementation tasks complete (4.1 is intentional human follow-up, not a lane task).
```

### 5. Move the change folder

Snapshot the change folder before the move, then use `git mv` (or `mv` if untracked):

```bash
# Create pre-move snapshot in .lucind/
mkdir -p .lucind/archive-premove-snapshot
cp -R openspec/changes/agentic-phase-specialist .lucind/archive-premove-snapshot/agentic-phase-specialist
diff -r .lucind/archive-premove-snapshot/agentic-phase-specialist openspec/changes/agentic-phase-specialist

# Move the change folder to archive
mkdir -p openspec/changes/archive
git mv openspec/changes/agentic-phase-specialist openspec/changes/archive/2026-08-30-agentic-phase-specialist

# Verify the move (diff should be empty; archive-report.md is present and identical on both sides)
diff -r .lucind/archive-premove-snapshot/agentic-phase-specialist openspec/changes/archive/2026-08-30-agentic-phase-specialist
```

### 6. Commit

One conventional commit, no AI attribution:

```bash
git commit -m "archive: close agentic-phase-specialist change and merge delta specs

- Merge 4 delta specs into openspec/specs/ (phase-specialist-dispatch, sdd-planning-fan-out, acceptance-verifier MODIFIED; phase-verdict-reporting ADDED)
- Preserve dispatch packets and result envelopes in archived change folder
- Move agentic-phase-specialist to openspec/changes/archive/2026-08-30-agentic-phase-specialist/
- Write archive report with final state, follow-ups, and reconciliation notes
"
```

## Out of scope

- Do NOT re-run verification, re-read the code for defects, or revisit the verdict. A clean verdict is an input here.
- Do NOT fix code, tests, or documentation. A defect found now is a follow-up in the report.
- Do NOT edit any artifact's content while moving it. Content changes and archival never share a lane.
- Do NOT touch another change's folder under `openspec/changes/`.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`.

## Allowed paths

`openspec/specs/`, `openspec/changes/agentic-phase-specialist/`, and `openspec/changes/archive/` only.

## Done criteria

- [x] Every whole-file copy and every folder move ran through the shell, with verbatim `diff -r` output in the result envelope, empty.
- [x] Every packet and result envelope from this change's dispatch is preserved under the change folder, frontmatter included, or its absence is recorded in the report.
- [x] Every delta requirement reached the live spec with its classification honored, and every MODIFIED block replaced the whole live block.
- [x] `archive-report.md` exists with every section populated, and every follow-up is named there.
- [x] The change folder is at `openspec/changes/archive/2026-08-30-agentic-phase-specialist/` and no longer at its original path.
- [x] The work is committed with a conventional commit and no AI attribution (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` if any of these fire:

- An implementation task in `tasks.md` is unchecked without explicit reconciliation authorization. (NOT FIRED: all tasks complete, 4.1 is human follow-up with explicit authorization.)
- `verify.md` records a CRITICAL issue. (NOT FIRED: PASSED with no CRITICAL issues.)
- A `diff -r` readback is non-empty. (WILL BE REPORTED if fired; never retry the copy over it.)
- Shell access for the mechanical copy is unavailable. (NOT FIRED: shell access is available.)
- A MODIFIED delta block cannot be matched to a live requirement. (WILL BE REPORTED if fired.)
- The archive destination already exists. (NOT FIRED: does not exist yet.)
- Satisfying one instruction in this packet would require violating another. (NOT FIRED: no conflicts.)

## Context

**Change Title**: Agentic Phase Specialist — Hard Rule Carve-Out & Specialist Authority Extension

**Primary Repository Root**: `/home/lanzerdev/git_root/lucind-ai` (source for .lucind/packets and .lucind/results)

**Archive Date**: 2026-08-30 (today; ISO format)

**Terminal Verify Verdict**: PASSED (per verify.md, dual judgment unanimous pass, no CRITICAL issues)

**Capability IDs in delta specs**: `phase-specialist-dispatch`, `sdd-planning-fan-out`, `acceptance-verifier`, `phase-verdict-reporting`

**Orchestrator Decisions**:
- Task 4.1 (paste to ~/.claude/skills/sdd-*/SKILL.md) is intentionally unchecked and should be reported as an open human follow-up in the archive report, not treated as blocking archive.
- No partial-archive or stale-checkbox reconciliation override needed; all implementation tasks are complete.

## Required skills

- `/home/lanzerdev/.claude/skills/sdd-archive/SKILL.md`

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, carry the verbatim `diff -r` output for every copy and move, the count of packets and envelopes preserved, and every follow-up recorded in the report. Report `done` only when every done-criterion carries evidence and every hard stop is declared (all fired: false).
