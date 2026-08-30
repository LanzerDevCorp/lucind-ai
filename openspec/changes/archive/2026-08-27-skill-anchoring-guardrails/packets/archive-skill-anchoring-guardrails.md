---
id: archive-skill-anchoring-guardrails
executor: agy
routed_by: mechanical archival of a verified change, single lane, no fan-out
allowed_paths: ["openspec/specs/", "openspec/changes/skill-anchoring-guardrails/", "openspec/changes/archive/"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: c7c3e3a13809a545ffe33e4fcc441da3003706e0
expected_parent_sha: c7c3e3a13809a545ffe33e4fcc441da3003706e0
---

# Packet archive-skill-anchoring-guardrails

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/archive-skill-anchoring-guardrails · **Branch:** lucind/archive-skill-anchoring-guardrails

## Goal

Close the SDD cycle for `skill-anchoring-guardrails` mechanically: preserve every packet and result envelope the session produced, merge the delta specs into `openspec/specs/`, write the archive report, and move the change folder into `openspec/changes/archive/`.

## Why this is one lane and not a fan-out

Archival is a filesystem operation, not a judgment.

## Why this is safe to dispatch now

Verification reached a terminal verdict (`openspec/changes/skill-anchoring-guardrails/verify.md`, PASSED, unanimous `done`/`done` from `agy` and `cursor-agent`, zero blocking defects, Orchestrator independently cross-checked the load-bearing citations) and the Orchestrator accepted it. All 16 implementation tasks (1.1–3.7 plus the two DOC tasks) in `tasks.md` are checked `[x]`.

## Preconditions

- `openspec/changes/skill-anchoring-guardrails/` exists in this worktree.
- `openspec/changes/archive/2026-08-27-skill-anchoring-guardrails/` does not exist.
- `openspec/changes/skill-anchoring-guardrails/verify.md` exists and records the terminal PASSED verdict.
- `openspec/changes/skill-anchoring-guardrails/tasks.md` exists, every task checked.

## Required reading

1. `~/.claude/skills/sdd-archive/SKILL.md` — the real `gentle-ai` archive skill. Read its **Mechanical Copy Contract**, **Task Completion Gate**, and **Final-State Authority** sections; this packet leans on them hardest.
2. `openspec/changes/skill-anchoring-guardrails/verify.md` — the verdict and its notes.
3. `openspec/changes/skill-anchoring-guardrails/tasks.md` — every checkbox.
4. `openspec/changes/skill-anchoring-guardrails/specs/*/spec.md` — the 5 delta specs about to be merged (all classified ADDED — the proposal's original "Modified Capabilities" claim for `lane-worktree-lifecycle` and `worktree-cleanup-cli` was independently corrected to ADDED during the spec fan-out after an exhaustive live-spec audit found no existing spec for either; see `spec-synthesis-notes.md`).
5. `openspec/specs/` — confirm no live spec exists for any of the 5 capability names (`worktree-dirty-guardrail`, `lane-worktree-lifecycle`, `worktree-cleanup-cli`, `failure-guidance-banners`, `tdd-wip-rescue-protocol`) before treating all five as new-capability spec files.

## The mechanical copy rule

File content MUST NEVER pass through the model's Read/Write path to be copied.

- Copy and move with the shell only: `cp -R`, `mv`, or `git mv`.
- Never reproduce a file's content by reading it and writing it back.
- After every copy or move, run `diff -r` between source and destination as a mandatory readback.
- The verbatim `diff -r` output goes in the result envelope. Empty output is the only pass.

## Procedure

Do these in order.

### 1. Gates

- **Task completion**: every task in `tasks.md` is already `[x]` (confirmed by the Orchestrator against the apply commit diff before dispatch). If you find any unchecked, STOP and block — do not silently reconcile.
- **Verification**: `verify.md` records PASSED with zero CRITICAL issues. Confirm this yourself before proceeding.
- **Missing artifacts**: proposal, spec, and design all exist for this change — none are missing.

### 2. Preserve the session's dispatch record

`.lucind/` is gitignored, so the packets and envelopes this session produced exist only in the primary repository's working directory at `/home/lanzerdev/git_root/lucind-ai/.lucind/`.

```
if [ -d /home/lanzerdev/git_root/lucind-ai/.lucind/packets ]; then
  mkdir -p openspec/changes/skill-anchoring-guardrails/packets
  cp -R /home/lanzerdev/git_root/lucind-ai/.lucind/packets/. openspec/changes/skill-anchoring-guardrails/packets/
  diff -r /home/lanzerdev/git_root/lucind-ai/.lucind/packets openspec/changes/skill-anchoring-guardrails/packets
else
  echo "no packets/ — recording as absent"
fi

if [ -d /home/lanzerdev/git_root/lucind-ai/.lucind/results ]; then
  mkdir -p openspec/changes/skill-anchoring-guardrails/envelopes
  cp -R /home/lanzerdev/git_root/lucind-ai/.lucind/results/. openspec/changes/skill-anchoring-guardrails/envelopes/
  diff -r /home/lanzerdev/git_root/lucind-ai/.lucind/results openspec/changes/skill-anchoring-guardrails/envelopes
else
  echo "no results/ — recording as absent"
fi
```

The primary root's `.lucind/packets/` and `.lucind/results/` directories contain ONLY this change's packets and results (no other change was in flight concurrently in this primary root during this session) — copy everything found there.

### 3. Merge delta specs into the live specs

All 5 capabilities under `openspec/changes/skill-anchoring-guardrails/specs/` are classified ADDED with no live spec existing under any of their names. Each becomes a new full spec file at `openspec/specs/<capability>/spec.md`: title `# <Capability> Specification`, a `## Purpose` paragraph (write one sentence grounded in the capability's own requirement text — do not invent scope beyond it), a `## Requirements` heading, then carry the requirement and scenario bodies over exactly as written from the delta (do not reword them). If any delta file is already authored as a complete spec (title, `## Purpose`, `## Requirements`), a plain `cp`/`diff -r` is correct and preferred instead of re-typing.

### 4. Write the archive report

Write `openspec/changes/skill-anchoring-guardrails/archive-report.md` per the skill's required shape (`## Verdict`, `## What Shipped`, `## Dispatch Record`, `## Follow-ups`, `## Gaps and Contradictions`). Under `## Follow-ups`, include verbatim the 4 non-blocking gaps `cursor-agent`'s verify judgment found (see `verify.md`): the untested ignored-file-only dirty state, the moot relative/absolute path distinction, the one pre-existing e2e test not exercising `dag.Split`'s new optional stderr parameter, and the untested accept-error-path receipt absence. Under `## Dispatch Record`, count lanes by phase and executor from the preserved packet frontmatter you just copied in step 2 (expect: 3 explore lenses + 1 synthesis, 3 propose lenses + 1 synthesis, 3 design lenses + 3 spec lenses + 2 synthesis, 3 tasks lenses + 1 synthesis, 2 apply attempts (one deviated, superseded by its corrected re-dispatch), 2 verify judges — all `agy` except the verify `cursor-agent` judge, per the human-approved AGY-only exception).

### 5. Move the change folder

```
mkdir -p .lucind/archive-premove-snapshot
cp -R openspec/changes/skill-anchoring-guardrails .lucind/archive-premove-snapshot/skill-anchoring-guardrails
git mv openspec/changes/skill-anchoring-guardrails openspec/changes/archive/2026-08-27-skill-anchoring-guardrails
diff -r .lucind/archive-premove-snapshot/skill-anchoring-guardrails openspec/changes/archive/2026-08-27-skill-anchoring-guardrails
```

### 6. Commit

One conventional commit, no AI attribution.

## Out of scope

- Do NOT re-run verification, re-read the code for defects, or revisit the verdict.
- Do NOT fix code, tests, or documentation. A defect found now is a follow-up in the report.
- Do NOT edit any artifact's content while moving it.
- Do NOT touch another change's folder under `openspec/changes/`.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`.

## Allowed paths

`openspec/specs/`, `openspec/changes/skill-anchoring-guardrails/`, and `openspec/changes/archive/` only.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-archive/` — the real `gentle-ai` archive skill and its `references/`.

**Read-only**: `/home/lanzerdev/git_root/lucind-ai/.lucind/packets/` and `/home/lanzerdev/git_root/lucind-ai/.lucind/results/` — the only source for the dispatch record in step 2, read never written.

The skill is authority on *what* archival must do; this packet is authority on *how this phase is executed here* — superseded on purpose where they conflict; note conflicts in the archive report's `## Gaps and Contradictions`.

Write nothing outside this repository.

## Done criteria

- [ ] **Every whole-file copy and every folder move ran through the shell**, with the verbatim `diff -r` output for each one in the result envelope, empty.
- [ ] **Every packet and result envelope from this change's dispatch is preserved under the change folder**, frontmatter included, or its absence is recorded in the report.
- [ ] **Every delta requirement reached the live spec with its classification honored** (all ADDED for this change).
- [ ] **`archive-report.md` exists with every section populated**, and every follow-up is named there.
- [ ] **The change folder is at `openspec/changes/archive/2026-08-27-skill-anchoring-guardrails/`** and no longer at its original path.
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- An implementation task in `tasks.md` is unchecked.
- `verify.md` records a CRITICAL issue.
- A `diff -r` readback is non-empty.
- Shell access for the mechanical copy is unavailable.
- A MODIFIED delta block cannot be matched to a live requirement (not applicable — all 5 are ADDED, but verify this yourself rather than trusting this note).
- The archive destination already exists.
- Satisfying one instruction in this packet would require violating another.

## Context

Change: **skill-anchoring-guardrails**. Primary repository root: `/home/lanzerdev/git_root/lucind-ai`. Archive date: `2026-08-27`. Terminal verify verdict: PASSED (unanimous `done`/`done`, zero CRITICAL). Capability ids under `specs/`: `worktree-dirty-guardrail`, `lane-worktree-lifecycle`, `worktree-cleanup-cli`, `failure-guidance-banners`, `tdd-wip-rescue-protocol` — all ADDED, no live spec exists for any. No partial-archive or checkbox-reconciliation authorization is needed or given; every task is already checked in the current `tasks.md`.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, carry the verbatim `diff -r` output for every copy and move, the count of packets and envelopes preserved, and every follow-up recorded in the report. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
