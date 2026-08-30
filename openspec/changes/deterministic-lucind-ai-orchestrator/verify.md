# Verify: deterministic-lucind-ai-orchestrator

**Date:** 2026-08-29
**Overall verdict: PASSED**

## Stage 1 -- Mechanical Check

`lucind-ai check` at commit `e6daee3`, exit 0, 1m55.04s. Full transcript:
`openspec/changes/deterministic-lucind-ai-orchestrator/verify-mechanical.log` (frozen and
committed at `13eb3b3`). Every package `ok`, including `cmd/lucind-ai`, `internal/run`,
`internal/packet`, `internal/packetauthor`, `internal/dag`, `internal/worktree`,
`internal/accept`, `internal/ledger`.

## Stage 2 -- Dual Qualitative Judgment

Dispatched via real `lucind-ai run` (two `read_only: true` packets, `agy` and `cursor-agent`,
dispatched in one barrier-joined invocation). Both lanes reached `status: done`, both integrated
cleanly with 0 reverted. Envelopes: `.lucind/results/verify-deterministic-lucind-ai-orchestrator-{agy,cursor-agent}.json`.

**Unanimous Pass (done/done).** Both lanes independently confirmed: `HardStop.Fired` demotion in
`decideStatus` overrides `envelope.Status` (Phase 3), cross-runtime skill-parity and
embedded-schema-freshness preflight gates both `runDispatch` and `runFeatureCreate` before
worktree/ledger allocation (Phase 5), the Claude and OpenCode skill trees are byte-identical
(Phase 1), and the pinned target-free-parse/DAG-split/attempt-replay/CAS behavior (Phases 2 and
4) was left untouched, matching `tasks-synthesis-notes.md`'s "mostly pin-existing" scoping.

## Stage 3 -- Evidence Cross-Checking

The two highest-leverage citation clusters — the ones that decide PASS vs. BLOCKED — were
independently re-verified against the real candidate at `e6daee3`, not accepted on the lanes'
word alone.

### Confirmed spec compliance (independently re-checked)

- **Hard-stop demotion is real, not just asserted.** `internal/run/run.go:868-896` implements
  `decideStatus`: after a schema-valid `result.Read`, it loops `envelope.HardStops` and returns
  `lane.Blocked` on the first `Fired` entry regardless of `envelope.LaneStatus()`. Directly
  re-read, byte for byte — matches both lanes' citations. `internal/run/decide_status_test.go:12-39`
  is a real RED-turned-GREEN test: it builds a schema-valid `status: "done"` envelope with one
  fired hard stop via `fstest.MapFS`, calls the real `decideStatus`, and asserts the returned
  status is `lane.Blocked` while `env.Status` stays `"done"` — a terminal, non-tautological
  assertion of exactly the scenario `tasks.md:45` (`3.0-RED`) specified.
- **CLI preflight genuinely gates both barriers before allocation, not after.** Directly re-read
  `cmd/lucind-ai/cli.go:353-378`: `preflightOrchestratorContract(primaryRoot)` (`:363`) runs after
  the linked-worktree refusal but strictly before `ledger.Open` (`:371`) and `depsFactory`
  (`:378`) in `runDispatch`. `cli.go:1104-1123`: the same call (`:1110`) runs before `ledger.Open`
  (`:1115`) and `featSvc.Create` (`:1123`) in `runFeatureCreate`. `preflightOrchestratorContract`
  itself (`cli.go:854-868`) calls `skillTreesByteIdentical` then compares the embedded schema
  against the on-disk file, failing closed on either mismatch — confirmed at `cli.go:870-928`.
- **cursor-agent's own honest caveat is accurate, not a defect.** Re-read `cli.go:326-366`: the
  `agy` quota gate (`ensureAgyQuota`, guarded by `usesAgy`) runs before
  `preflightOrchestratorContract`. A stale-schema or skill-mismatch dispatch that also uses `agy`
  can rotate a pooled credential before failing closed. This does not violate the spec's actual
  requirement — "halt before any worktree allocation or ledger mutation" — since no worktree or
  ledger write happens either way, but it is a real ordering fact worth recording as a follow-up.

### Non-blocking findings (confirmed real, not spec violations, not production defects)

Both lanes independently flagged the same class of residual gap; re-read and confirmed accurate,
none dispute a spec requirement:

1. **`decideStatus`'s hard-stop demotion is tested in isolation, not through a full `Execute()` →
   ledger-write path.** `TestDecideStatus_FiredHardStopDemotes` calls `decideStatus` directly via
   a fake `fs.FS`; it does not drive `Execute` (`run.go:486-502`) end-to-end to assert a `Blocked`
   row actually lands in the ledger. The production wiring (`Execute` calls `decideStatus` and
   returns early on `Blocked`, skipping `enforceAllowedPaths`/`enforceRequiredSkills`/
   `enforceCompletionMode`) is unchanged and correct on direct read. **Follow-up, not a
   blocker**: an `Execute()`-level integration test asserting the ledger row for a fired-hard-stop
   lane.
2. **CLI preflight failure messages are asserted by exit code and worktree-creation suppression
   (`spyCreateWorktree` false), not by stderr substring.** `TestRunPreflight_SkillParity` and
   siblings (`cli_test.go:5956-6135`) prove the halt is real and terminal but do not assert the
   stderr text names "skill parity" or "stale schema." **Follow-up, not a blocker**: add stderr
   substring assertions for operator-facing diagnosability.
3. **Missing-tree and missing-schema negative cases are covered in production code
   (`readSkillTree`/`os.ReadFile` both return real errors and fail closed) but lack dedicated CLI
   negative tests** for a wholly absent OpenCode tree or absent on-disk schema file, as opposed to
   a byte-mismatch. **Follow-up, not a blocker.**
4. **Preflight sits after the `agy` quota gate**, per the caveat above. **Follow-up, not a
   blocker**: consider moving `preflightOrchestratorContract` before the quota check if avoiding
   any pre-failure network/credential activity becomes a requirement later.

None of the above findings dispute a spec requirement, and none were rated as blocking by either
judgment lane or by this independent re-check.

## Verdict

**PASSED.** All five delta specs are implemented and tested against real production paths, not
merely checked off in `tasks.md`. The two MODIFIED specs (`packet-authoring-contract`,
`acceptance-verifier`) extend, rather than duplicate or contradict, the live requirements already
in `openspec/specs/` — confirmed by both lanes and independently re-read. Four non-blocking
findings are recorded above as follow-ups; none block this change.

## Follow-ups (not blockers, not tracked as separate SDD changes unless requested)

- Add an `Execute()`-level integration test asserting the ledger row for a fired-hard-stop lane.
- Add stderr substring assertions to the CLI preflight negative tests for operator diagnosability.
- Add dedicated CLI negative tests for a wholly missing OpenCode tree and a wholly missing
  on-disk schema file.
- Consider whether `preflightOrchestratorContract` should run before the `agy` quota gate if
  "no side effects on any failure path" becomes an explicit requirement.
