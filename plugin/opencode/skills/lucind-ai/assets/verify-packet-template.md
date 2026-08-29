---
id: <id>
executor: agy
routed_by: qualitative verification of spec compliance, edge cases, and test quality
model: <model, e.g. gemini-3.7-flash-high>
read_only: true
---

# Packet <id>

**Tier:** A (human merge) | B (auto-merge after audit)
**Worktree:** ../<repo>-worktrees/<id>  ·  **Branch:** lucind/<id>

## Goal

Perform qualitative verification of the candidate implementation for change `<change-id>` against its specifications, edge cases, and test quality, evaluating the frozen mechanical check results.

## Why this is safe to dispatch now

The candidate implementation is complete, and mechanical checks (`lucind-checks.sh`) have already run once and passed deterministically. This judgment lane is read-only and does not mutate repository state or race with other lanes.

## Preconditions

- Mechanical checks have already executed deterministically and passed.
- Frozen mechanical check log is committed to the candidate branch and embedded in `## Context`.
- Worktree is created from the candidate branch `HEAD`.

## Done criteria

- [ ] **Every indirection introduced is demonstrably consumed by a terminal consumer.** Verification citations trace to concrete symbols, spec requirements, or tests.
- [ ] **The worktree carries no unique commits and no working-tree changes relative to the lane's birth point (`git status --porcelain` empty AND `HEAD` equals `git merge-base HEAD <primary HEAD>`).**
- [ ] **Qualitative evaluation completed** (`.lucind/result.json` populated with `status`, `summary`, and structured `findings`).

## Allowed paths

None. This is a read-only judgment lane. Do NOT create, modify, or delete any tracked or untracked files in the worktree, other than `.lucind/result.json`.

## Allowed paths outside the repository

None.

## Out of scope

Do NOT execute `go test`, `go build`, `go vet`, `lucind-checks.sh`, or any shell test/build suite. Deterministic mechanical checks have already run once; their frozen output is in `## Context`. Re-running them wastes quota and adds no new signal. Do NOT modify any source files or commit any changes.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired. An undeclared hard stop invalidates the result.

- Executing mechanical test/build commands when mechanical results are already provided.
- Any credential value would need to be chosen, generated, or written.
- A done-criterion turns out to be impossible, or already true for a reason the packet did not anticipate.
- Two reasonable interpretations exist for a spec requirement and the specification does not say which.
- Satisfying one instruction in this packet would require violating another.

## Tool selection guidance

Perform your qualitative evaluation using read/navigation tools (`Read`, `Glob`, `Grep`, `codegraph`) and read-only git queries (`git diff`, `git log`, `git show`). Do NOT use shell execution for build or test runners.

## Evaluation areas

1. **Spec compliance**: Verify that the implementation satisfies all requirements and scenarios in `specs/<spec-id>/spec.md`.
2. **Edge cases**: Identify any missing edge-case handling, negative scenarios, boundary conditions, or concurrency concerns.
3. **Test quality**: Evaluate whether test cases assert on real terminal behavior rather than tautologies, mocks, or internal implementation details.

## Context

### Mechanical check summary
<Embedded summary of lucind-ai check / lucind-checks.sh execution: command line, exit code, duration, candidate git SHA>

### Mechanical check transcript
<Embedded frozen mechanical log transcript or path to openspec/changes/<id>/verify-mechanical.log>

### Relevant specifications and design documents
- `openspec/changes/<id>/design.md`
- `openspec/changes/<id>/specs/`

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. That file is what the dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at `.lucind/result.schema.json` in this worktree. Validate against it before writing — an envelope that fails schema validation makes the lane `blocked` regardless of how well the work went.

Omit the `commit` field (or leave it empty) per read-only envelope convention. Report all qualitative observations in `findings` with `finding`, `evidence` (`file:line` or command output), and `affects`.

Report `done` only when every done-criterion carries evidence and every hard stop is declared.
