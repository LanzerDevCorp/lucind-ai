---
id: agent-prompts-assets
executor: agy
routed_by: Template and executor-skill naming cleanup after the required-skills rendering contract is defined.
feature: skill-provisioning-and-phase-specialist
parent_ref: refs/heads/feature/skill-provisioning-and-phase-specialist
base_sha: aa59d8e2bf8d95687df33c9e939bd2b55b84166c
expected_parent_sha: aa59d8e2bf8d95687df33c9e939bd2b55b84166c
allowed_paths: [".agents/skills","plugin/claude-code/skills/lucind-ai/assets"]
read_only_paths: ["internal/packet","internal/packetauthor"]
---

# Packet agent-prompts-assets

## Goal
Remove executor-named skill files and hardcoded dispatched-skill paths from the allowed skill/template assets while aligning templates with required-skills delivery.

## Why this is safe to dispatch now
The rendering contract is integrated, and this unit is markdown-only with a disjoint asset scope.

## Preconditions
- Verify `packet-contract` is integrated and inspect only the declared asset trees.
- Read `tasks.md:80-81` and `design.md:55-60,108`.

## Allowed paths
- `.agents/skills/`
- `plugin/claude-code/skills/lucind-ai/assets/`

## Read-only inputs
- `internal/packet/`
- `internal/packetauthor/`

## Allowed paths outside the repository
None. Do not touch external paths.

## Out of scope
Do not create `lucind-archive` or `lucind-ultrafixer`, edit `.opencode/agent/lucind-packet-author.md`, or modify Go code.

## Done criteria
- [ ] `git grep -n 'lucind-archive\|lucind-ultrafixer' .agents/skills plugin/claude-code/skills/lucind-ai/assets` proves forbidden executor-named skills are absent.
- [ ] `git grep -n '~/.claude/skills/' plugin/claude-code/skills/lucind-ai/assets` returns no hardcoded dispatched-skill paths; templates remain structurally valid.
- [ ] Every changed template contract is consumed by the dispatch workflow; cite file/line evidence.
- [ ] Commit conventionally with no AI attribution; clean status and latest commit evidence are recorded.

## Hard stops
- [ ] Stop `blocked` if a stub skill, hardcoded path, or edit outside the whitelist is required.
- [ ] Stop `blocked` for impossible criteria, unresolved design choices, or contradictory instructions.

## Context
`design.md:55-59,108` and `tasks.md:80-81` explicitly prohibit the stubs and hardcoded paths.

## Return
Write the result envelope to **.lucind/result.json in this worktree**. That file is what the dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at .lucind/result.schema.json in this worktree. Validate against it before writing — an envelope that fails schema validation makes the lane `blocked` regardless of how well the work went.
