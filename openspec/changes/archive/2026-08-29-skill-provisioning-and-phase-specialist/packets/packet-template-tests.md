---
id: packet-template-tests
executor: agy
routed_by: Update packet template contract tests to match required-skills delivery after asset path decoupling.
feature: skill-provisioning-and-phase-specialist
parent_ref: refs/heads/feature/skill-provisioning-and-phase-specialist
base_sha: ed28806c7745bccbac01de86296ac79522b68ebe
expected_parent_sha: ed28806c7745bccbac01de86296ac79522b68ebe
allowed_paths: ["internal/packet/packet_test.go"]
read_only_paths: ["plugin/claude-code/skills/lucind-ai/assets"]
---

# Packet packet-template-tests

## Goal
Update packet template contract tests so they verify each template names its phase skill as delivered under `## Required skills`, without asserting machine-local `~/.claude/skills/` paths that the templates must no longer contain.

## Preconditions
- Confirm the asset templates use the required-skills delivery wording and contain no hardcoded dispatched-skill paths.
- Keep the test coverage equivalent: every affected template must still be checked for its phase skill and required-skills delivery wording.

## Allowed paths
- `internal/packet/packet_test.go`

## Read-only inputs
- `plugin/claude-code/skills/lucind-ai/assets/`

## Out of scope
Do not modify packet parser behavior, asset templates, or any other test or production file.

## Done criteria
- Replace every affected hardcoded path expectation with an exact assertion for the corresponding phase skill delivered under `## Required skills`.
- Focused packet tests pass, and the full repository check passes.
- Commit conventionally with no AI attribution; clean status and latest commit evidence are recorded.

## Hard stops
- Stop `blocked` if the asset wording is not present, hardcoded paths remain in the assets, or any edit outside the whitelist is required.
- Stop `blocked` for impossible criteria, unresolved design choices, or contradictory instructions.

## Return
Write the result envelope to **.lucind/result.json in this worktree**. Validate it against `.lucind/result.schema.json` before writing.
