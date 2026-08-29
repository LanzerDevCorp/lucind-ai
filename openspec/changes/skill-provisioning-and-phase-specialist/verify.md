# Verify: Skill Provisioning and the SDD Phase Specialist

## Result: BLOCKED

## Stage 1: Mechanical check

`lucind-ai check` at commit `07b359c6e9ace392613a9d2e5b8d24af28fdb95b` — PASSED (exit 0, full `go build ./...` + `go test ./... -race -count=1` green). See `verify-mechanical.log`. An earlier run at the same commit failed only on `TestRunLegacyModeDispatch`, an unrelated pre-existing test that shells out to a live `agy` OAuth session; reproduced twice at the identical commit with different outcomes, confirming it is environmental, not a regression.

## Stage 2: Dual qualitative judgment

Dispatched `agy` (gemini-3.7-flash-high) and `cursor-agent` (cursor-grok-4.6-high), both read-only, against the frozen mechanical evidence.

- `agy`: reported unconditional pass across all 8 delta specs. Its "terminal consumer" citations traced only the *read* side of each seam (e.g. that `enforceRequiredSkills` reads `Envelope.SkillsLoaded`), never the *write* side (whether the field is actually populated on the real dispatch path). This verdict is not reliable evidence on its own.
- `cursor-agent`: reported 8 findings with `file:line` citations, several disputing that production code paths actually do what the passing tests assume.

## Stage 3: Evidence cross-checking

Independently verified against the candidate (not just re-reading the envelopes):

1. **CONFIRMED** — `internal/run/run.go:445-453` builds `executor.Request{...}` and copies `ReadOnlyPaths` but never copies `RequiredSkills`/`AdhocSkills`. `LUCIND_REQUIRED_SKILLS` is therefore never injected on the real `run.Execute` path; it only appears in `internal/executor` unit tests because those tests set `Request.RequiredSkills` directly.
2. **CONFIRMED** — `cmd/lucind-ai/cli.go:2452-2460` (`phaseDispatch`) calls `adapter.Synthesize(ctx, phasespec.SynthesizeRequest{ChangeName, Phase, Force})` with no `LensStates` and no `Content`. The specialist never calls `admitDispatchBatch`/`runDispatch`, so `lucind-ai phase <name>` cannot actually dispatch a synthesis lane in production — only the already-complete no-op path is reachable from the CLI.
3. **CONFIRMED** — `plugin/opencode/skills/lucind-ai/assets/*.md` still contain hardcoded `~/.claude/skills/...` paths (verified via `git grep`); only the `plugin/claude-code` sibling tree was updated. Task 4.2 named only `plugin/claude-code/skills/lucind-ai/assets/*.md`; design.md's File Changes list did not scope out the OpenCode sibling.

Not independently re-verified line-by-line but corroborated by consistent, specific citations from the same lane whose first three claims all checked out on inspection:

4. Lens accepted/merged state from `gentle-ai sdd-status` is never passed into `Synthesize` (`phasespec.Status` carries no lens fields; only test code constructs `LensStates` directly) — same root cause as finding 2.
5. Canonical artifact filenames (`proposal.md`, `apply-progress.md`, `verify-report.md`, `archive-report.md`) diverge from the spec's named files (`propose.md` etc., `phase-specialist-dispatch/spec.md:7,13`).
6. `admitDispatchBatch` → `skillset.Derive` fail-closes on any `sdd_phase` outside the closed set, even for legacy packets that omit `lane_role` (which `packet.Parse` otherwise still accepts per the backward-compatibility scenario) — a regression against `read-only-packet-schema`'s own "omitted lane_role preserves backward compatibility" scenario.
7. Design Decision 8 required `LaneRole` in the accept decode struct in lockstep with `packetDigest`; only `RequiredSkills` was added (`internal/accept/accept.go:275-287` has no `LaneRole` field), so `packetDigest`'s inclusion of `LaneRole` is never cross-checked at acceptance.
8. `phasespec.isPhaseComplete` treats a `done` status-JSON token as sufficient without checking that the canonical artifact file actually exists on disk, contrary to the unchanged-phase scenario's stated precondition.

## Disposition

Per this repo's SDD strategy (dual-dispatch verify, evidence cross-checking): **confirmed violations produce BLOCKED and remediation tasks**, not PASSED. The mechanical gate and 21/21 `tasks.md` checkboxes are necessary but were not sufficient — three of the eight qualitative findings are independently confirmed real production-wiring gaps, not disputed by any contrary evidence, and the remaining five are corroborated by concrete citations from the same source.

`tasks.md` items 2.2/2.3 (dual delivery), 3.2/3.3 (phase specialist dispatch + canonical artifacts + backward-compat regression), 2.4 (accept lockstep), and 4.2 (OpenCode assets) are reopened — see amended checkboxes and the "Remediation" subsection appended to `tasks.md`. Re-run this verify sequence after remediation lands.
