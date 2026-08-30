---
id: assets-and-template-tests
executor: agy
routed_by: Complete asset skill-path decoupling with its repository contract-test update so the integrated candidate remains green.
feature: skill-provisioning-and-phase-specialist
parent_ref: refs/heads/feature/skill-provisioning-and-phase-specialist
base_sha: ed28806c7745bccbac01de86296ac79522b68ebe
expected_parent_sha: ed28806c7745bccbac01de86296ac79522b68ebe
allowed_paths: [".agents/skills","plugin/claude-code/skills/lucind-ai/assets","internal/packet/packet_test.go"]
read_only_paths: ["internal/packet","internal/packetauthor"]
---

# Packet assets-and-template-tests

## Goal
Remove executor-named skill files and hardcoded dispatched-skill paths from the allowed skill/template assets, and update the packet template contract tests to verify required-skills delivery rather than machine-local paths.

## Preconditions
- Verify the runtime and packet contract dependencies are integrated.
- Read `tasks.md:80-81` and `design.md:55-60,108`.

## Allowed paths
- `.agents/skills/`
- `plugin/claude-code/skills/lucind-ai/assets/`
- `internal/packet/packet_test.go`

## Read-only inputs
- `internal/packet/`
- `internal/packetauthor/`

## Out of scope
Do not create `lucind-archive` or `lucind-ultrafixer`, edit `.opencode/agent/lucind-packet-author.md`, or modify production Go code outside the named contract test.

## Done criteria
- `git grep -n 'lucind-archive\|lucind-ultrafixer' .agents/skills plugin/claude-code/skills/lucind-ai/assets` proves forbidden executor-named skills are absent.
- `git grep -n '~/.claude/skills/' plugin/claude-code/skills/lucind-ai/assets` returns no hardcoded dispatched-skill paths; templates remain structurally valid.
- Contract tests assert each affected template names its phase skill delivered under `## Required skills`, with no hardcoded path expectations.
- Focused packet tests and the full repository check pass.
- Commit conventionally with no AI attribution; clean status and latest commit evidence are recorded.

## Hard stops
- Stop `blocked` if a stub skill, hardcoded path, edit outside the whitelist, or production behavior change is required.
- Stop `blocked` for impossible criteria, unresolved design choices, or contradictory instructions.

## Return
Write the result envelope to **.lucind/result.json in this worktree**. Validate it against `.lucind/result.schema.json` before writing.
