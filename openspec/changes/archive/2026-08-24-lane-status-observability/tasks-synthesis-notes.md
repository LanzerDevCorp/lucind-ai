# Tasks Synthesis Notes: Lane Status Observability

## Unresolved Contradictions

None.

The three `## Assumed decomposition` blocks partition the six capabilities the way `design.md` already did. They do not assert incompatible architectures. Lens B’s leftover “notice to C” that `Run.PID` needs v7 `runs.pid` matches Lens C’s own assumed order. Lens A’s `Lane`/`laneDTO` fields (`skill`, `packet_path`) do not collide with Lens B’s `LaneProgress` numerics/`tool_rate`. Differences that looked like fights were ownership/order (see Decomposition Divergence), not two product claims the code cannot settle.

## Coverage Gaps

- **Swept-orphan UI.** Spec `batch-wave-view` requires the dashboard to show a swept lane as `failed` plus the explanatory note. No lens tasked an `app.js` change for that. Design File Changes for `app.js` only lists skill + packet link. Existing `BatchLane.Note` (`model.go:202-205,:348-374`) already maps `EventLaneNote`, and `app.js:526` already falls back to `note`/`Note`. Not invented as a task; apply should confirm that existing path, not add a second renderer.
- **Skill vs packet on execution, not spine.** `sdd-tasks` Step 3 lists a 530-word budget, Engram persist (Step 4), and a return-summary block (Step 5). This packet’s 1800-word budget (DDL excluded), two-file output, and `.lucind/result.json` supersede those. Forecast field *names*, Suggested Work Units *columns*, specific/actionable/verifiable/small, and the threat-matrix RED rule follow the skill. Skill rule “High 400-line risk → Chained PRs recommended: Yes” is overridden by the already-accepted `exception-ok` / `size:exception` values this packet required.
- **C1.3 / C2.6 had no RED line.** Lens C listed GREEN-only CLI wiring and named `TestRunDispatch` / `TestServeDispatch` only in acceptance evidence. Canonical tasks add RED 2.3 and 3.14 from those proving commands (extend `TestRunDispatchRegistersRunRowInLedger`; new serve-dispatch sweeper assert). Not invented coverage.
- **DAG spine items 6–7.** Wave green-on-its-own, same-wave path-disjoint, and per-unit executor apply only if a DAG is recommended. Step 4 declined a DAG, so those items are N/A rather than missing.
- **`tool_rate` start instant.** Design-synthesis coverage gap is closed in `design.md`: elapsed is lane `StartedAt` → progress `At` with a 1s floor. Task 3.3/3.4 states that. Not still open.

## Dropped Citations

Manifest union was the worklist. Unique ranges were opened once, batched by file. Verified rows shipped (or informed a shipped sentence) and are not repeated. Counts: 88 manifest rows (A 30, B 38, C 20); ~70 unique ranges; 68 verified; 1 dropped; 1 retargeted; 1 overclaim dropped. Known-wrong citations from earlier synthesis notes did not resurface as authority.

1. **Lens A — `cmd/lucind-ai/cli_test.go:40-60` as “CLI test suite structure for command flag and execution testing.”** Those lines are `TestRunNoArgsPrintsUsageAndFails` and `TestRunUnknownSubcommandPrintsUsageAndFails` (usage text, not flags). Packet-flag seams are `TestRunMissingPacketFlagIsUsageError` (`:98`) and `TestRunRepeatablePacketFlagPreservesOrderAndProcessesEachOne` (`:393`). The Path-assignment task is kept; the `:40-60` claim is dropped. **Retargeted** to `:98` and `:393`.

2. **Lens A — `design.md:179-189` as “Process Integration owned by lens C.”** The table marks Process integration Applicable and the other five rows N/A. It does not assign a lens. N/A omission is kept; the ownership sentence is dropped.

3. **Lens B threat-matrix row — Process integration with `TestMigrateV6ToV7PreservesRowsAndAddsSchema`.** Not a `file:line` miss: those schema tests exist as v6 seams (`schema_test.go:14-97,:99-144,:146-201`) and are kept as schema TDD (task 1.1). They do not exercise PID liveness. Process-integration RED tests are Lens C’s sweeper cases only. Classification dropped, citation kept for schema.

`server_test.go:47-100` is `TestBulkRequestBodyReturns400`. Kept only as the httptest status/error pattern, explicitly not packet-GET coverage (same as design-synthesis). `hub.go:213-235` is `Hub.Run`; kept as the sweeper loop *pattern*, not as an existing sweeper (same as design-synthesis). `claude_stream_test.go:42-67` is argv (`--output-format stream-json`, `--verbose`); Lens B claimed flag setup, which holds. Usage fixtures remain new work after that test.

## Decomposition Divergence

All three slices converged on the same system. Divergence was order, TDD polarity, and dispatch shape.

**Named cross-lens dependency 1 (resolved):** Lens C’s `Run.PID` insert/select/scan (`runs.go:16-24,:29-41,:63-76,:80-101,:165-188`) is after Lens B’s schema-v7 task. Canonical: 1.10 depends on 1.2. Strict tables cannot grow `pid` in place; Go queries against a v6 `runs` table would fail at runtime even if they compiled.

**Named cross-lens dependency 2 (resolved):** Shared-file touches do **not** need a DAG `allowed_paths` sequence. This change ships as one accepted PR (`size:exception`), not parallel apply lanes, so there is no `allowed_paths` conflict to block. Sequencing inside one packet is enough:

- `cli.go`: Unit 2 Path at `:160-174`, then Unit 3 PID at `:314-324`, then Sweeper at `:770-774`.
- `model.go`: Unit 2 Lane/`laneDTO` (`:163-184,:322-333`) then Unit 1 LaneProgress/`tool_rate` (`:186-193,:336-346`) — different structs, still one file, so sequential commits.
- `app.js`: Unit 2 only (skill + packet link at `:534-536,:575-593`). Lens B cited `:542-544` as an *existing* consumer of `total_tokens`/`cost_usd`/`tool_rate`; B’s work unit does not modify `app.js`. The A/B “overlap” Open Question overstated the edit conflict.

**Sidecar recommendation (step 4):** **No `apply-dag.yaml`. Single packet.** Weighed against `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md` and `conflict-triage-fixture/tasks.md`, which declined a DAG because Strict-TDD RED/GREEN for one unit belongs in one lane and `Integrate` bisects a failing combined tree (`internal/run/integrate.go:50-59`). The two real cross-slice deps (v7 column before `Run.PID`; three shared files) are ordering constraints, not independently green waves. A DAG that split A/B/C would put `cli.go` and `model.go` in two units of the same wave (path-not-disjoint) or serialize them anyway, paying sidecar cost for no parallelism. User already accepted one PR.

**Lens C TDD inversion:** C’s table made C1.1 depend on C1.2 because the PID field must exist for the test to compile. Canonical restores RED-before-GREEN (1.9 then 1.10). Uncompiling tests are normal Go TDD here.

**Lens C extra sweeper test:** Design Planned RED tests list three names. Adversarial cases still include PID recycling and `EPERM`. Canonical keeps C’s `TestSweeper_RecycledPIDAndEPERM` (3.12).

**Work-unit order:** Unit 1 = Lens B (schema first), Unit 2 = Lens A, Unit 3 = Lens C. Alternative A-then-B also works for types; B-first is chosen so `runs.pid` exists before any PID Go code.

**Open Questions from the three drafts:** all were synthesizer flags (shared files, v7-before-runs.go, skill word budget). Closed here. None remain in `tasks.md`.
