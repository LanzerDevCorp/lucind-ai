---
id: remediate-run-skills-delivery
executor: agy
routed_by: fix confirmed verify.md finding 1 — dual delivery not wired on the real dispatch path
model: gemini-3.7-flash-high
---

# Packet remediate-run-skills-delivery

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/remediate-run-skills-delivery  ·  **Branch:** lucind/remediate-run-skills-delivery

## Goal

Fix verify.md finding 1 (CONFIRMED): the `executor.Request{...}` construction in `run.go` (around lines 445-453) copies `ReadOnlyPaths` from the packet but never copies the packet's required-skill fields, so `LUCIND_REQUIRED_SKILLS` is never actually injected into dispatched children on the real `run.Execute` path — it only works in `internal/executor` unit tests that set `Request.RequiredSkills` directly.

## Why this is safe to dispatch now

The candidate implementation (through commit `a363096d1612f78763528a0e5a2d2a69785612eb`) already has `RequiredSkills` fields on `packet.Packet` and `executor.Request`; this unit only wires the existing packet field through to the existing request field at the one call site. No new types or contracts are introduced.

## Preconditions

- **This lane already has WIP committed** (commit `2b08d27` on this branch): `internal/run/run.go`'s `executor.Request{...}` construction now copies `p.RequiredSkills` with a defensive copy, and `internal/run/skills_enforcement_test.go` gained `TestExecutePassesRequiredSkillsToExecutorRequest` covering non-empty/nil/empty-slice cases plus defensive-copy isolation. A prior dispatch of this packet timed out mid-verification after finishing this work; `go build ./...` and `go test ./internal/run/... -race -count=1` were independently confirmed green before this redispatch. Start by reviewing `git log -1` and `git diff HEAD~1` in this worktree to see exactly what's already done.
- Read `internal/run/run.go:435-460` (the `executor.Request{...}` construction) and `internal/executor/executor.go:20-45` (`requestEnv`, which already correctly reads `Request.RequiredSkills`).
- Read `internal/packet/packet.go` to find the exact resolved field name(s) that should flow here — the packet's own `RequiredSkills`/`AdhocSkills` fields (post-admission resolution), not a placeholder.
- Read `internal/executor/required_skills_test.go:40-44` to see how the field is exercised in isolation today.

## Allowed paths

- `internal/run/` (only the request-construction call site and any directly-adjacent code needed to pass the right field through; do not touch unrelated logic)

## Read-only inputs

- `internal/executor/`
- `internal/packet/`
- `internal/packetauthor/`

## Out of scope

Do not change `enforceRequiredSkills`'s existing demotion logic (it already works correctly per the passing `TestEnforceRequiredSkills`). Do not touch `internal/accept`, `internal/phasespec`, `cmd/lucind-ai`, or any plugin/asset files — those are separate remediation units dispatching in the same wave. Do not modify `internal/executor/executor.go`.

## Done criteria

- [ ] A new or extended test in `internal/run` proves that `run.Execute` (or whichever function builds the `executor.Request`), given a packet with required skills, produces a `Request` whose skills field is populated — not just that `enforceRequiredSkills` demotes on shortfall after the fact. The test must exercise the real construction path, not construct `Request` directly. (Already satisfied by the committed WIP — verify it, don't redo it.)
- [ ] `go test ./internal/run/... -run RequiredSkills` (and any renamed/added test names covering this) passes.
- [ ] `go build ./...` and `go test ./... -race -count=1` remain green.
- [ ] If the committed WIP already satisfies every criterion on inspection, do not add unnecessary changes — just verify, then write the result envelope reporting `done` with evidence citing the existing commit.
- [ ] Commit conventionally with no AI attribution; clean status and latest commit evidence are recorded (this may already be satisfied by the existing WIP commit if no further changes are needed).

## Hard stops

- Stop `blocked` if fixing this requires changing the `executor.Request` struct shape, `packet.Packet` struct shape, or any other package's public contract — flag it instead of guessing.
- Stop `blocked` if two different packet fields plausibly hold "the skills to deliver" and it's ambiguous which one `run.go` should read.

## Return

Write the result envelope to **.lucind/result.json in this worktree**. Validate it against `.lucind/result.schema.json` before writing.
