---
id: agent-prompts-assets-retry
executor: agy
routed_by: Retry of agent-prompts-assets after preserving its completed asset changes and refreshing the integration base.
feature: skill-provisioning-and-phase-specialist
parent_ref: refs/heads/feature/skill-provisioning-and-phase-specialist
base_sha: ed28806c7745bccbac01de86296ac79522b68ebe
expected_parent_sha: ed28806c7745bccbac01de86296ac79522b68ebe
allowed_paths: [".agents/skills","plugin/claude-code/skills/lucind-ai/assets","internal/packet/packet_test.go"]
read_only_paths: ["internal/packetauthor"]
---

# Packet agent-prompts-assets-retry

## Goal
Remove executor-named skill files and hardcoded dispatched-skill paths from the allowed skill/template assets while aligning templates with required-skills delivery, and update the existing packet-contract tests that assert those exact hardcoded paths so they match the new template content.

## Preconditions
- Verify `packet-contract` is integrated and inspect only the declared asset trees.
- Read `tasks.md:80-81` and `design.md:55-60,108`.
- A prior dispatch of this same packet failed integration because it removed `~/.claude/skills/...` strings from templates without updating `internal/packet/packet_test.go`'s `TestExplorePacketTemplatesContract`, `TestProposePacketTemplatesContract`, `TestDesignPacketTemplatesContract`, `TestSpecPacketTemplatesContract`, `TestTasksPacketTemplatesContract`, and `TestArchivePacketTemplateContract`, which assert those exact strings are present. This dispatch MUST update those assertions in the same commit — do not remove the assertions' intent (that templates carry the required-skills contract), just the literal hardcoded string they check for.

## Allowed paths
- `.agents/skills/`
- `plugin/claude-code/skills/lucind-ai/assets/`
- `internal/packet/packet_test.go` (test-assertion updates only; do not touch `internal/packet/packet.go` or any other non-test file in that package)

## Read-only inputs
- `internal/packetauthor/`

## Out of scope
Do not create `lucind-archive` or `lucind-ultrafixer`, edit `.opencode/agent/lucind-packet-author.md`, or modify any Go source file (only the one named test file, and only its assertions).

## Done criteria
- `git grep -n 'lucind-archive\|lucind-ultrafixer' .agents/skills plugin/claude-code/skills/lucind-ai/assets` proves forbidden executor-named skills are absent.
- `git grep -n '~/.claude/skills/' plugin/claude-code/skills/lucind-ai/assets` returns no hardcoded dispatched-skill paths; templates remain structurally valid.
- `go test ./internal/packet/... -run PacketTemplatesContract` passes against the updated templates.
- Every changed template contract is consumed by the dispatch workflow; cite file/line evidence.
- Commit conventionally with no AI attribution; clean status and latest commit evidence are recorded.

## Hard stops
- Stop `blocked` if a stub skill, hardcoded path, or edit outside the whitelist is required.
- Stop `blocked` for impossible criteria, unresolved design choices, or contradictory instructions.

## Return
Write the result envelope to **.lucind/result.json in this worktree**. Validate it against `.lucind/result.schema.json` before writing.
