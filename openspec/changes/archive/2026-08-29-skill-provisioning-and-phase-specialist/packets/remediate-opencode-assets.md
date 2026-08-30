---
id: remediate-opencode-assets
executor: agy
routed_by: fix confirmed verify.md finding 7 — OpenCode assets sibling tree not updated
model: gemini-3.7-flash-high
---

# Packet remediate-opencode-assets

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/remediate-opencode-assets  ·  **Branch:** lucind/remediate-opencode-assets

## Goal

Fix verify.md finding 7: `plugin/opencode/skills/lucind-ai/assets/*.md` still contains hardcoded `~/.claude/skills/...` dispatched-skill paths, confirmed via `git grep -n '~/.claude/skills/' plugin/opencode/skills/lucind-ai/assets/`. The sibling `plugin/claude-code/skills/lucind-ai/assets/` tree was already fixed by an earlier lane. Apply the equivalent fix to the OpenCode tree: drop hardcoded paths, align with `## Required skills` delivery, same as the already-fixed `plugin/claude-code` tree.

## Preconditions

- Read `plugin/claude-code/skills/lucind-ai/assets/*.md` (the already-fixed reference) and diff its approach against the current `plugin/opencode/skills/lucind-ai/assets/*.md` state to see exactly what changed there.
- Read `internal/packetauthor/compile.go:220-238` (`renderBody`) to confirm the `## Required skills` contract these templates should align with.

## Allowed paths

- `plugin/opencode/skills/lucind-ai/assets/`

## Read-only inputs

- `plugin/claude-code/skills/lucind-ai/assets/`
- `internal/packetauthor/`

## Out of scope

Do not modify `plugin/claude-code/skills/lucind-ai/assets/` (already correct) or `.opencode/agent/lucind-packet-author.md` (no hardcoded skill paths). Do not create `lucind-archive` or `lucind-ultrafixer` stubs. Do not touch any Go source.

## Done criteria

- [ ] `git grep -n '~/.claude/skills/' plugin/opencode/skills/lucind-ai/assets` returns no matches.
- [ ] Every changed OpenCode template stays structurally valid (parses the same way the `plugin/claude-code` templates do — check for any `internal/packet` test that also covers OpenCode templates, if one exists; if none exists, verify by hand that structure matches the `claude-code` sibling).
- [ ] Commit conventionally with no AI attribution; clean status and latest commit evidence are recorded.

## Hard stops

- Stop `blocked` if the OpenCode templates have a materially different structure/contract from `plugin/claude-code`'s such that a like-for-like fix doesn't apply cleanly — describe the difference instead of guessing.

## Return

Write the result envelope to **.lucind/result.json in this worktree**. Validate it against `.lucind/result.schema.json` before writing.
