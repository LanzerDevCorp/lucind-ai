---
id: propose-skill-anchoring-guardrails-lens-a
executor: agy
routed_by: candidate and approach lens of the three-lens propose fan-out for Change skill-anchoring-guardrails
allowed_paths: ["openspec/changes/skill-anchoring-guardrails/propose-lens-a.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: 1bcc9ff596b7a0fd9bf8e84ea4f7b5f8e755d5d7
expected_parent_sha: 1bcc9ff596b7a0fd9bf8e84ea4f7b5f8e755d5d7
---

# Packet propose-skill-anchoring-guardrails-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/propose-skill-anchoring-guardrails-lens-a  ·  **Branch:** lucind/propose-skill-anchoring-guardrails-lens-a

## Goal

Produce `openspec/changes/skill-anchoring-guardrails/propose-lens-a.md`: the candidate selection, proposed technical approach, changes to system concepts, and architecture rationale for this change.

This is one of three parallel propose lenses. It is feedstock for a synthesis lane, not the final proposal document. Do not write a complete `proposal.md`.

## Why this is safe to dispatch now

`explore.md` and `explore-synthesis-notes.md` are integrated (zero unresolved contradictions, zero coverage gaps, zero dropped citations — all three explore lenses converged on Candidate 1: fail-closed worktree cleanup + CLI banner anchoring + prescriptive TDD rescue protocol). Lens B and lens C run in parallel against the same codebase and write to different files, so no lane races another. This lens owns candidate selection and core approach; the other two own capability specs and risk/rollback.

## Preconditions

- `openspec/changes/skill-anchoring-guardrails/explore.md` exists.
- `openspec/changes/skill-anchoring-guardrails/propose-lens-a.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-propose/SKILL.md` — the real `gentle-ai` propose skill. It is the phase contract this draft feeds.
2. `openspec/changes/skill-anchoring-guardrails/explore.md` and `explore-synthesis-notes.md` — the accepted, citation-verified exploration. Its Candidate 1 recommendation is the starting point; do not re-litigate the candidate choice, only detail the approach.
3. `internal/worktree/worktree.go` (`Cleanup`, `Remove`, `PorcelainEmpty`) and its callers in `cmd/lucind-ai/cli.go`, `internal/integrate/integrate.go`, `internal/integrate/candidate.go`, `internal/run/integrate.go` — every automated internal teardown caller must keep working unforced.
4. `openspec/changes/archive/` for prior proposals with similar shape (CLI flag addition + guardrail).

Never guess at code. Every claim about existing structure carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/skill-anchoring-guardrails/propose-lens-a.md`:

```markdown
# Proposal Lens A — Candidate & Approach: Skill Anchoring & Worktree Cleanup Guardrails

## Selected Candidate & Approach

<State the chosen candidate from exploration, summarize the core approach, and explain why this approach solves the problem. Cite file:line for existing code behavior.>

## Conceptual Changes & Architecture Rationale

<Describe additions, modifications, or deprecations to system concepts, interfaces, or architectural patterns. Cite file:line for existing concepts. Include: the new `force bool` parameter shape for `worktree.Cleanup`/`Remove`, the new exported `ErrWorktreeDirty` sentinel, the `--force`/`-f` CLI flag, and which internal (non-operator) callers of `Cleanup`/`Remove` must pass `force: true` to preserve current unconditional-teardown behavior for automated paths.>

## Alternatives Considered & Rejected

<What alternative approaches were considered during candidate selection and why they were rejected — summarize Candidates 2 and 3 from explore.md.>

## Open Questions

- [ ] <unresolved technical question, or "None">
```

## Size budget

`propose-lens-a.md` MUST be under 1000 words.

## Out of scope

- **Lens B owns**: capability impact table, delta specification requirements, and scenarios.
- **Lens C owns**: technical risks, failure modes, rollback plan, additivity assessment, and test impact.

Do not write delta spec requirements or a rollback plan here.

## Allowed paths

`openspec/changes/skill-anchoring-guardrails/propose-lens-a.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-propose/` — the real `gentle-ai` propose skill and its `references/`. The skill is authority on *what* a proposal document must contain; this packet is authority on *how this phase is executed here* — superseded on purpose where they conflict; note conflicts in `## Open Questions`.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row per unique citation, grouped by file, files alphabetical, line numbers ascending.

| citation | claim |
|---|---|
| `path/to/example_file.ext:12-34` | <YOUR OWN real citation and claim from THIS draft> |

## Mechanical self-check (REQUIRED)

**Before you commit:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/propose-lens-a.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Selected Candidate & Approach" \
  --require-section "Conceptual Changes & Architecture Rationale" \
  --require-section "Alternatives Considered & Rejected" --require-section "Open Questions" \
  --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/propose-lens-a.md
```

Paste both PASS/FAIL reports into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every candidate and approach claim carries `file:line` citations to real code in this worktree.**
- [ ] **`propose-lens-a.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section plus `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Strip any injected `Co-authored-by:` trailer.

## Hard stops

- The technical approach cannot be grounded in existing codebase patterns or proposal context.
- Candidate selection contradicts frozen exploration conclusions without justification.
- Satisfying one instruction in this packet would require violating another.

## Context

Change: **skill-anchoring-guardrails**. Accepted exploration selected Candidate 1: fail-closed worktree cleanup (require `--force` unless the worktree is clean) + CLI banners anchoring failure/milestone output to the existing skill reference docs + a prescriptive TDD WIP-rescue protocol. Rejected: automatic stash/commit-on-teardown (ref pollution, clean-tree guarantee violations) and interactive TTY prompting (headless batch incompatibility). Execution for this Change: Isolated Mode (feature `skill-anchoring-guardrails`), SDD with fan-out planning, `agy`-only executor throughout except verify's second qualitative judge (kept on `cursor-agent`) — already decided, do not re-litigate.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
