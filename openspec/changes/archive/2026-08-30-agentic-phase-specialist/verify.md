# Verify: Agentic Phase Specialist

## Status

**PASSED**

## Stage 1: Mechanical Check

- Command: `lucind-ai check --out openspec/changes/agentic-phase-specialist/verify-mechanical.log`
- Candidate: `19d5f01c0ae5c65b12cede90d47804ec578568b7` (branch `dev`, apply already merged)
- Result: exit 0, `CGO_ENABLED=0 go build ./...` then `go test ./... -race -count=1`, all packages `ok` (duration 2m36.385271238s)
- Log committed at `c271f2e9d4382121abc1f38091e551c00c04a348`

## Stage 2: Dual Qualitative Judgment Dispatch

- Run id: `f70557a2-d4da-4249-9d57-2dccc2bcec49`
- `lucind-ai run --packet .lucind/packets/verify-agentic-phase-specialist-agy.md --packet .lucind/packets/verify-agentic-phase-specialist-cursor-agent.md --legacy-main --expected-parent-sha c271f2e9d4382121abc1f38091e551c00c04a348 --timeout 30m`
- `verify-agentic-phase-specialist-agy` (agy / gemini-3.7-flash-high): `status: done`, all done criteria met, no hard stops fired.
- `verify-agentic-phase-specialist-cursor-agent` (cursor-agent / cursor-grok-4.6-high): `status: done`, all done criteria met, no hard stops fired.
- Both lanes integrated cleanly (`integrated_ids`: both; `reverted_ids`: none).

## Stage 3: Evidence Cross-Checking & Verdict Reconciliation

Both judges independently confirmed unanimous pass (`done`/`done`). Every cited `file:line` was independently re-verified against the actual merged code at `19d5f01c0ae5c65b12cede90d47804ec578568b7`:

- `internal/accept/accept.go:84` — `GetLaneMetadata` loads unconditionally, outside the `AuthoringEvidenceVersion` branch (`:88-96`). Confirmed.
- `internal/accept/accept.go:97-99` — `validateResultAndScope` (schema/hard-stops/done-criteria/`allowed_paths`) runs unconditionally regardless of the gate. Confirmed.
- `internal/accept/accept.go:120` — `runSDDPhaseChecks := metadata.SDDPhase == "" || metadata.SDDPhase == "apply"` gates `integrate.CheckPolicySnapshot` and `v.check` (`:122-149`). Confirmed.
- `internal/run/attempt.go:384-396` (`shouldRunAttemptChecks`) — resolves `SDDPhase` per combined lane via `deps.Ledger.GetLaneMetadata`; fails closed (returns `true`) on nil ledger, empty branches, lookup error, empty `SDDPhase`, or `"apply"`; returns `false` only when every combined lane is a declared non-apply phase. Confirmed.
- `internal/run/attempt.go:469-477` — `checkFunc` invocation gated by `runSDDPhaseChecks`; lease renewal (`:447-449`) and `integrate.Check` itself (`internal/integrate/integrate.go:159`) stay ungated. Confirmed.
- Both skill trees (`plugin/claude-code/skills/lucind-ai/` and `plugin/opencode/skills/lucind-ai/`): `SKILL.md:19` Hard Rule carve-out, `references/strategies/fan-out.md:47` Specialist-reads-synthesis-notes move, and `references/contracts/acceptance-promotion.md` decision-bearing Specialist Acceptance + `sdd_phase`-conditional checklist steps 1 and 8 — byte-identical (`diff` empty on all three pairs), matching `design.md:39-47,95-96` verbatim. Confirmed.
- `internal/accept/accept_test.go:362-405` and `internal/run/attempt_test.go:1252-1348` — assert real terminal behavior (a failing `lucind-checks.sh` script causing/not causing rejection; `checkFunc` call counts of 0 vs 1), not tautologies or mocks of the gate itself. Confirmed.
- `openspec/changes/agentic-phase-specialist/tasks.md:52` — task 4.1 correctly left unchecked as out-of-lane-authority (human/Orchestrator paste to `~/.claude/skills/sdd-*/SKILL.md`; Lanes cannot write outside the repository per `fan-out.md:43`). Confirmed.

### Non-blocking residual findings (cursor-agent)

All independently re-checked; none are confirmed spec violations of this Change:

1. **Exclusive-mode `--legacy-main` `Integrate` path stays ungated** (`internal/run/integrate.go:50,117,276`; `cmd/lucind-ai/cli.go:451-457`). This is `design.md` Decision 2's explicit, named choice ("`integrate.Check` stays ungated"), not an omission — confirmed by direct reading of Decision 2's text and call-site list. No action required.
2. **The "explicit check exception" clause is currently unconfigurable** (`internal/ledger/lanes_meta.go:20-47` has `SDDPhase` and no exception field). Confirmed. `tasks.md`'s own Open Questions (`:80-82`) and `design.md:140` explicitly defer this to a later Change and explicitly say "do not add schema" for this one. Not a defect of this Change.
3. **`RecoverAttempt`'s non-CASPending replay path rebuilds `Branches` as `[]string{worktree.BranchFor(att.ID)}`** (`internal/run/attempt.go:718`) rather than the original combined lane IDs, so `shouldRunAttemptChecks`'s lane lookup will typically miss and fail closed to running checks on recovery. Confirmed by direct read. This fails closed (safe, conservative — matches the designed default), so it is a minor optimization gap for a future change, not a correctness or spec violation.
4. **`docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:19` and `docs/sdd-phase-specialist.md:36-41`** still list the Hard Rule rewrite, `fan-out.md` rewrite, and `sdd_phase` gate as "Not yet done," even though commits `6723b77`/`2080528`/`8dc595b` landed all three. Confirmed by direct read — these pre-SDD planning notes are now stale. They are not part of this Change's own artifact set (`design.md`'s File Changes table does not list them, and `tasks.md` has no task for them), so updating them is out of this Change's scope. Recommend a follow-up doc-hygiene task, not a remediation of this Change.

## Unresolved Divergence

None.

## Disposition

Unanimous Pass (`done`/`done`) per `references/strategies/sdd.md`'s Stage 3 table. `verify: { status: done }`.
