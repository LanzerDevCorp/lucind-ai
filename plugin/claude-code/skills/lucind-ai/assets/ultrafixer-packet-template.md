---
id: <id>
executor: agy
routed_by: pre-existing defect triage and repair
model: gemini-3.7-flash-high
base_sha: <base_sha>
parent_ref: <parent_ref>
allowed_paths: ["<path1>", "<path2>"]
---

# Packet <id>

**Tier:** B (auto-merge after audit)
**Worktree:** ../<repo>-worktrees/<id>  ·  **Branch:** lucind/<id>

## Goal

Triage pre-existing defect `<error-signature>`, assess critical and blocking impact across active features, and either deliver an isolated repair commit via a `blocked` result envelope or persist a Defect Record in the ledger.

## Preconditions

- Target feature `<feature-id>` is active with recorded `base_sha` `<base_sha>`.
- Failing check command is runnable in the current worktree environment.

## Done criteria

- [ ] **Every indirection introduced is demonstrably consumed by a terminal consumer.**
- [ ] **The work is committed (if critical/blocking) OR recorded in the ledger (if non-critical/non-blocking).**
- [ ] Origin classification executed against `base_sha`. If feature-introduced, exit `done` with explanatory summary.
- [ ] Two-axis evaluation completed across active features discovered via `lucind-ai feature status`.
- [ ] Cross-branch impact verified via failure signal reproduction in candidate worktrees.
- [ ] Schema-valid `.lucind/result.json` emitted with appropriate `status`, `questions`, and `findings`.

## Allowed paths

- `<path1>`
- `<path2>`

## Hard stops

- Origin classification reveals the defect was introduced on the current feature branch between `base_sha` and `HEAD` (exit `status: done`, touch no files).
- The defect cannot be reproduced locally with the provided check command.
- Any credential value would need to be chosen, generated, or written.
- Auto-integrating or merging the repair commit directly into any branch.

## Context

### Failing check command
`<exact-failing-command, e.g. go test ./... / npm test / cargo test / pytest>`

### Error transcript and signature
`<error-output-and-stack-trace>`

### Feature metadata
- Target feature: `<feature-id>`
- Base SHA: `<base_sha>`
- Parent ref: `<parent_ref>`

## Return

Write the result envelope to `.lucind/result.json` in this worktree.
