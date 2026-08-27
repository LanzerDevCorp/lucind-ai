---
id: propose-skill-anchoring-guardrails-lens-c
executor: agy
routed_by: risks, rollback, and test impact lens of the three-lens propose fan-out for Change skill-anchoring-guardrails
allowed_paths: ["openspec/changes/skill-anchoring-guardrails/propose-lens-c.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: 1bcc9ff596b7a0fd9bf8e84ea4f7b5f8e755d5d7
expected_parent_sha: 1bcc9ff596b7a0fd9bf8e84ea4f7b5f8e755d5d7
---

# Packet propose-skill-anchoring-guardrails-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/propose-skill-anchoring-guardrails-lens-c  ·  **Branch:** lucind/propose-skill-anchoring-guardrails-lens-c

## Goal

Produce `openspec/changes/skill-anchoring-guardrails/propose-lens-c.md`: risk assessment, rollback strategy, additivity, and test impact for this change.

This is one of three parallel propose lenses. It is feedstock for a synthesis lane, not the final proposal document. Do not write a complete `proposal.md`.

## Why this is safe to dispatch now

`explore.md` is integrated and accepted, including Lens C's original risk findings (backward-compat breakage across internal callers, stdout contamination, dirty-detection false positives/negatives, multi-wave `base_sha` staleness). Lens A and lens B run in parallel against the same codebase and write to different files, so no lane races another.

## Preconditions

- `openspec/changes/skill-anchoring-guardrails/explore.md` exists.
- `openspec/changes/skill-anchoring-guardrails/propose-lens-c.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-propose/SKILL.md` — the real `gentle-ai` propose skill.
2. `openspec/changes/skill-anchoring-guardrails/explore.md` — the accepted exploration's Technical Risks & Trade-offs section is your starting worklist, not your final answer; verify and extend it.
3. `internal/worktree/worktree_test.go` and `cmd/lucind-ai/cli_test.go` — existing tests that call `Cleanup`/`Remove`/`worktree cleanup` and assume unconditional deletion; these are the backward-compat break surface.
4. `cmd/lucind-ai/cli_test.go` output-parsing tests (structured stdout: `integrated_ids:`, `reverted_ids:`, receipt lines) — the stream-contamination risk surface.
5. `lucind-checks.sh` — whether it parses `lucind-ai` CLI output in a way new banners could break.

Never guess at seams or failure modes. Every row in your tables carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/skill-anchoring-guardrails/propose-lens-c.md`:

```markdown
# Proposal Lens C — Risks, Rollback & Test Impact: Skill Anchoring & Worktree Cleanup Guardrails

## Technical Risks & Failure Modes

| Risk / Failure Mode | Impact | Mitigation | Existing seam (file:line) |
|---|---|---|---|

## Rollback & Additivity

**Rollback Plan**: <exact mechanism for reversal, git revert vs schema rollback>
**Additivity**: <state explicitly whether formats, schemas, or ledgers change additively or destructively, citing file:line — this change should be purely additive: new `force` parameter/flag, new banners, no schema change>

## Test & Validation Impact

| Test Layer | Impact / Required Coverage | Existing seam (file:line) |
|---|---|---|

## Out of Scope

<Work explicitly excluded from this proposal.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`propose-lens-c.md` MUST be under 1000 words. Tables over prose.

## Out of scope

- **Lens A owns**: candidate selection, technical approach, and conceptual changes.
- **Lens B owns**: capability impact table, delta specification requirements, and scenarios.

Rollback and test impact are yours.

## Allowed paths

`openspec/changes/skill-anchoring-guardrails/propose-lens-c.md` only. Create no other file.

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
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/propose-lens-c.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Technical Risks & Failure Modes" --require-section "Rollback & Additivity" \
  --require-section "Test & Validation Impact" --require-section "Out of Scope" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/propose-lens-c.md
```

Paste both PASS/FAIL reports into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every risk, test seam, and rollback claim carries `file:line` citations to real code in this worktree.**
- [ ] **`propose-lens-c.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section plus `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Strip any injected `Co-authored-by:` trailer.

## Hard stops

- Whether a schema or format change is additive cannot be determined, making rollback a guess.
- A critical failure mode has no identifiable mitigation or test seam.
- Satisfying one instruction in this packet would require violating another.

## Context

Change: **skill-anchoring-guardrails**. Accepted exploration selected Candidate 1 (fail-closed worktree guardrail + banner anchoring + prescriptive TDD rescue), already flagging backward-compat breakage across internal teardown callers, stdout/stderr contamination risk, and multi-wave `base_sha` staleness as risks to mitigate rather than reasons to reject. Execution for this Change: Isolated Mode, SDD with fan-out planning, `agy`-only executor throughout except verify's second qualitative judge (kept on `cursor-agent`) — already decided, do not re-litigate.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
