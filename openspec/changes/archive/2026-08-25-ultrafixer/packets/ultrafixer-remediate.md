---
id: ultrafixer-remediate
executor: agy
routed_by: SDD verify (Stage 3) confirmed two violating findings and four non-blocking findings; remediation dispatch before re-verify/archive, following BLOCKED verdict at feature/ultrafixer's verify.md (integrated at fc57d35).
allowed_paths: ["cmd/lucind-ai/cli.go", "cmd/lucind-ai/cli_test.go", "internal/ledger/ledger.go", "internal/ledger/ledger_test.go", "internal/packet/packet_test.go", "plugin/claude-code/skills/lucind-ai/assets/ultrafixer-packet-template.md", "openspec/changes/ultrafixer/verify.md", "openspec/changes/ultrafixer/state.yaml", ".claude-plugin/marketplace.json", "plugin/claude-code/.claude-plugin/plugin.json", "internal/packet/testdata/skill_content_hash.txt"]
feature: ultrafixer
parent_ref: refs/heads/feature/ultrafixer
base_sha: fc57d355947c8cc4be8f663d7a26b9c7d24bc9ba
expected_parent_sha: fc57d355947c8cc4be8f663d7a26b9c7d24bc9ba
---

# Packet ultrafixer-remediate

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/ultrafixer-remediate  ·  **Branch:** lucind/ultrafixer-remediate

## Goal

Fix the two confirmed verify violations and four non-blocking findings from
`openspec/changes/ultrafixer/verify.md`'s Disposition section, Strict-TDD (RED then GREEN, no
exceptions). Mark `openspec/changes/ultrafixer/verify.md`'s Disposition section updated to reflect
remediation done, and update `openspec/changes/ultrafixer/state.yaml`'s `phases.verify` to record
that remediation landed (do not change `status` yourself — leave it for the follow-up re-verify
dispatch to set, since this packet does not re-run Stage 2/3 judgment).

## Why this is safe to dispatch now

`verify.md` (committed at this packet's `base_sha`) already names the exact confirmed violations
with `file:line` evidence and a stated remediation direction for each. This packet's job is to
implement those fixes — not re-diagnose or re-litigate the verdict.

## Preconditions

- `openspec/changes/ultrafixer/verify.md` exists at this packet's `base_sha`. Read it in full —
  its Disposition section (numbered items 1-3, where item 3 bundles the four non-blocking
  findings) is your authoritative task list.
- This repo runs Strict TDD Mode. Test runner: `go test ./...`.

## Done criteria

- [ ] **Every indirection introduced is demonstrably consumed by a terminal consumer.**
- [ ] **The work is committed.** Evidence: `git status --porcelain` empty and `git log --oneline
      -1`. Conventional commit(s), no AI attribution.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l` (on touched files) all clean.
- [ ] `go test ./... -race -count=1` passes in full.
- [ ] **Confirmed violation 1 resolved and proven with a REAL linked-worktree invocation, not just
      an in-process Go test.** After your fix, actually create a throwaway linked worktree (e.g.
      `git worktree add /tmp/<scratch> HEAD` from this Lane's own worktree — clean it up with
      `git worktree remove` before finishing) and run the actual `lucind-ai` binary's
      `feature status`/`defect list` (or whatever verb you fixed) from inside it, showing it no
      longer refuses. This is the specific gap that let the original defect ship undetected — an
      in-process test alone does not count as evidence for this criterion.
- [ ] **Confirmed violation 2 resolved**: a disposition-transition method exists, is covered by a
      RED/GREEN test pair, and the packet template / coordination doc now actually instructs
      writing `disposition=declined` when a human declines a proposed fix (per
      `ultrafixer-dispatch/spec.md`'s "Human declines fix" scenario).
- [ ] All four non-blocking findings from `verify.md` are addressed.
- [ ] `openspec/changes/ultrafixer/verify.md`'s Disposition section has each of the 3 items marked
      with a short "Remediated:" note and evidence (commit/file:line), without deleting the
      original findings (they're the historical record of what was found).

## Allowed paths

- `cmd/lucind-ai/cli.go`
- `cmd/lucind-ai/cli_test.go`
- `internal/ledger/ledger.go`
- `internal/ledger/ledger_test.go`
- `internal/packet/packet_test.go`
- `plugin/claude-code/skills/lucind-ai/assets/ultrafixer-packet-template.md`
- `openspec/changes/ultrafixer/verify.md`
- `openspec/changes/ultrafixer/state.yaml`
- `.claude-plugin/marketplace.json`
- `plugin/claude-code/.claude-plugin/plugin.json`
- `internal/packet/testdata/skill_content_hash.txt`

(The last three are this repo's plugin-version guard, which tests will require bumping because
this packet touches `plugin/claude-code/skills/lucind-ai/**` content — declared proactively this
time, per the lesson from the earlier `ultrafixer-apply` packet's `deviated` result.)

## Allowed paths outside the repository

- `/tmp/<scratch-worktree-for-violation-1-proof>` — create and remove it yourself
  (`git worktree add` / `git worktree remove`) entirely within this Lane's own execution; it must
  not exist when you finish. Revert: `git worktree remove --force <path>` if still present.

## Out of scope

- Any file not listed in Allowed paths above.
- Re-running Stage 2/3 verify judgment yourself — a fresh dual-judge dispatch happens separately
  after this packet lands.
- Modifying `explore.md`/`proposal.md`/`design.md`/`specs/`/`tasks.md` (historical record, do not
  rewrite them even if remediation reveals they were slightly imprecise) or
  `/home/lanzerdev/.claude/agents/lucind-ai-fixer.md` (never touch it).
- Touching `plugin/claude-code/skills/lucind-ai/references/coordination/dependencies-defects.md`
  unless a non-blocking finding specifically requires it (it currently doesn't — the four
  non-blocking findings are all `ultrafixer-packet-template.md`/CLI-scoped).

## Hard stops

- Any credential value would need to be chosen, generated, or written.
- A done-criterion turns out to be impossible, or already true for a reason the packet did not
  anticipate.
- The change would break something outside `allowed_paths`.
- Two reasonable implementations exist for the `IsLinkedWorktree` fix (relax the guard vs. add an
  explicit `--repo` override flag) and neither `verify.md` nor precedent elsewhere in this codebase
  clearly favors one — stop and name the fork with your recommendation rather than picking silently.
- Satisfying one instruction in this packet would require violating another.

## Context

### verify.md's exact Disposition items (authoritative — see the full file for evidence/citations)

1. **Confirmed violation 1**: `runDefectRecord`/`runDefectList`/`runFeatureStatus`
   (`cmd/lucind-ai/cli.go:1908-1911,1967-1970,974-977`) all refuse via `worktree.IsLinkedWorktree`
   when run from inside a linked worktree — but ultrafixer's own Lane always runs inside one.
   `verify.md` suggests: either relax the guard for these three specific read/ledger-only verbs
   (they don't mutate git refs or worktrees, unlike `feature create`/`worktree cleanup` — check
   `internal/worktree/worktree.go` and any comment near `IsLinkedWorktree`'s definition/other call
   sites for why the guard exists there, to judge whether relaxing is actually safe), or add an
   explicit `--repo <path>` override on these three verbs and have
   `ultrafixer-packet-template.md` instruct passing the primary root path (you'd need a way for the
   Lane to actually know that path — check what packet/environment context is already available to
   an agy subprocess, e.g. is the primary root ever passed via an env var or packet field today?).
2. **Confirmed violation 2**: add `Ledger.UpdateDefectDisposition(ctx, id, disposition)` (or
   equivalent name matching this file's existing method-naming convention) and wire the "human
   declines fix" path — likely a new CLI verb or flag, e.g. `lucind-ai defect decline --id <id>`,
   matching the existing `lucind-ai defect record`/`lucind-ai defect list` pattern — plus update
   `ultrafixer-packet-template.md`/have the human-facing recommendation actually name this command.
3. **Four non-blocking findings** (Tier label contradiction, missing conventional-commit
   instruction in the template, weak packet-template contract test, missing CLI-level
   `--disposition` validation test) — fix each as described in `verify.md`.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. That file is what the
dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at `.lucind/result.schema.json` in this worktree. Validate against it before
writing — an envelope that fails schema validation makes the lane `blocked` regardless of how well
the work went.

Report `done` only when every done-criterion carries evidence and every hard stop is declared.
