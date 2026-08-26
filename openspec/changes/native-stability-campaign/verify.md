# Verify: Native Stability Campaign

**Status:** BLOCKED
**Candidate SHA:** `51e31edab361bf3b0fb278b75e8727cbe1360a25`
**Verified by:** Orchestrator synthesis of dual qualitative judgment (`agy`/`gemini-3.7-flash-high` + `cursor-agent`/`cursor-grok-4.6-high`), with every cited `file:line` independently re-verified against the candidate before this disposition was written.

## Stage 1: Mechanical Check

`lucind-ai check` — `status: passed`, `duration: 1m37.487143698s`, full `go test ./...` green across every package. Transcript: `openspec/changes/native-stability-campaign/verify-mechanical.log` (committed at `51e31ed`).

## Stage 2: Dual Qualitative Judgment

Both lanes reported `status: done`. Their qualitative conclusions **disagreed**: `agy` found the implementation fully compliant; `cursor-agent` found four HIGH-severity spec gaps in the production dispatch path. Per this project's verify disposition rule, disagreement with confirmed violations is `BLOCKED`, not `PASSED`, regardless of either lane's self-reported `status`.

## Stage 3: Evidence Cross-Checking

Every finding below was independently re-verified by the Orchestrator by reading the cited code directly — not accepted on either lane's word.

### CONFIRMED — Blocking (must fix before any real Stability Campaign)

1. **Worktree path convention mismatch (HIGH).** `internal/stability/campaign.go`'s `ExecuteTrialJourney`/`ExecuteTrialJourneyLive`/`continueTrialAfterDispatch` compute Change A/B worktree paths as `filepath.Join(deps.PrimaryRoot, "wt-"+packetID)` (`campaign.go:644,790`; `cmd/lucind-ai/stability.go:543-546` mirrors this for `RunCampaign`). Real production worktree creation (`productionDeps.CreateWorktree` in `cmd/lucind-ai/cli.go`, calling `worktree.Create`/`worktree.CreateWithParent`) places a lane's worktree at `internal/worktree/worktree.go:155-159`'s `pathFor`: `<parent-of-primaryRoot>/<basename-of-primaryRoot>-worktrees/<laneID>` — a sibling directory, not `<primaryRoot>/wt-<laneID>`.
   **Orchestrator-independent confirmation:** read `internal/worktree/worktree.go:155-159` directly; the doc comment states this is "a hard project rule, never a temp directory." The mismatch is real.
   **Impact:** In a real live Campaign, Change A/B worktrees are created at the real `pathFor` location, but the journey's envelope watch (`ExecuteTrialJourneyLive`'s poll loop), defect-record read (`ReadDefectRecord(deps.WorktreeFS(wtPathA), wtPathA)`), and `RecoverCrashedChangeB`'s envelope adoption all look at `wt-<laneID>` under `primaryRoot` — a path nothing ever writes to. Every real Trial would fail immediately on defect assessment / envelope read.
   **Why every test passed anyway:** both `campaign_test.go`'s `newJourneyTestDeps` and `cmd/lucind-ai/stability_test.go`'s `newSimulatedTestDeps` supply a fake `CreateWorktree` that deliberately writes to the same wrong `wt-<laneID>` convention the journey code reads from — internally consistent, never checked against the real `productionDeps.CreateWorktree`.
   **Reference:** `internal/stability/reconcile/reconcile.go:48-52,149`'s `WorktreePathFor` independently already uses the *correct* `pathFor`-based convention — the fixture journey code should be using the same helper, not reinventing its own.

2. **Trial promotion is a state-machine label, not a real promotion (HIGH).** `continueTrialAfterDispatch`'s "9. Promote stage" (`internal/stability/campaign.go:872-876`, directly re-read by the Orchestrator) is exactly `sm.AdvanceTrial(TrialPromoted)` — no call to `PromoteTargetCAS`, `PromoteChangeACAS`, `PromoteTargetBCAS`, or `fixture.VerifyAncestryIsolation` anywhere in `ExecuteTrialJourney`, `ExecuteTrialJourneyLive`, `continueTrialAfterDispatch`, or `RunCampaign`. Those functions exist and are exercised only by direct unit tests (`campaign_test.go`, `fixture_test.go`), never by the integrated journey.
   **Impact:** A real Campaign's mechanical-green Trials never actually perform the git-ref CAS promotion or ancestry-isolation verification the design describes — "Promoted" is reached as a bookkeeping tick with no corresponding side effect.

3. **Trial lifecycle is never persisted; `resume` cannot restore an interrupted stage (HIGH).** `internal/stability/store/store.go`'s schema holds only a `campaigns` row (id/candidate_sha/status/timestamps) — no trials table, no trial-stage column, no PGID column. Trial-stage state lives exclusively in the in-memory `StateMachine`, which is explicitly documented as "no clock, no timer, and no I/O." `specs/stability-resume-abort/spec.md` requires resuming "the exact interrupted stage" after a crash, which is structurally impossible with the current schema.

4. **`stability resume` inspects but never resumes (HIGH).** `runStabilityResumeWithDir` (`cmd/lucind-ai/stability.go`) calls `reconcile.Inspect`, prints the decision, and returns 0 on `DecisionSafe` — it never calls `StateMachine.StartNextTrial`/`ExecuteJourney` to actually continue campaign execution. Compounding this, `InspectParams`/`AbortParams` passed from the CLI omit `ProcessGroups` entirely, so `reconcile.Inspect`'s and `reconcile.Abort`'s orphaned-process auditing/termination loops (`internal/stability/reconcile/reconcile.go:200-216,381-394`) are unreachable dead code from the CLI today — there is no mechanism to recall which PGIDs belonged to a crashed campaign, since (per finding 3) nothing persists them.

### CONFIRMED — Non-blocking (should fix, does not itself block a Campaign attempt)

5. **Trial/Campaign timeout budgets are never enforced as real timers (MEDIUM).** Only `DispatchTimeout` (10m) is actually wired, as `run.Deps.LaneTimeout` via `productionDeps`. `TrialTimeout` (45m) and `CampaignTimeout` (135m) are pure constants; `CheckTrialBudget`/`CheckCampaignBudget` have no production caller, only a self-referential test (`campaign_test.go:670-680,799-818`).

6. **Fixture test doesn't assert the `sleep 5` instruction's presence (LOW/test-hygiene).** `fixture_test.go`'s `TestFixtureDeterministicSyntheticPackets` checks ID/Executor/Model/AllowedPaths but never asserts `Body` contains the Wave 6c `sleep 5` instruction. This was a known, explicitly documented limitation of Wave 6c (an automated test cannot prove a live `agy` CLI will obey a prompt instruction) — worth a presence-only assertion (does the string literal exist in `Body`) as a cheap regression guard, though it still can't prove real-world compliance.

### Independently re-confirmed COMPLIANT areas (both lanes agreed, spot-checked by the Orchestrator)

- Linux process-group isolation and abrupt kill (`Setpgid`, `SysProcAttr`, `Cancel` → `syscall.Kill(-pgid, ...)`, real-subprocess-proven).
- CLI preflight admission, interactive confirmation with no bypass flags, forecast copy.
- SQLite/WAL single-active campaign gate (partial unique index + conditional insert).
- Evidence sanitization (4096B cap, path stripping, hashing) and RFC 8785 canonical receipt shape.
- 3-Trial state machine's zero-retry reset, 10s lease TTL/fence/reclaim mechanics, PID-capture plumbing (Waves 6a/6b) — sound in isolation; the defect is that finding #1's wrong path means the real journey never reaches the state where these matter in a live run.

## Disposition

**BLOCKED.** Findings 1-4 are confirmed, high-severity, and collectively mean a real `lucind-ai stability run` today would not survive Trial 1 (finding 1) and, even if the paths were fixed, would not actually promote targets (finding 2) or be resumable after a crash (findings 3-4). None of this was caught by mechanical checks because every test's fake dependencies were internally self-consistent but never validated against the real `productionDeps` wiring path.

## Remediation Tasks

- [x] **R1 (blocks Campaign):** Replace `internal/stability/campaign.go`'s ad-hoc `filepath.Join(deps.PrimaryRoot, "wt-"+id)` worktree-path derivation (all call sites: `ExecuteTrialJourney`, `ExecuteTrialJourneyLive`, `continueTrialAfterDispatch`, `cmd/lucind-ai/stability.go`'s `RunCampaign`) with the real, correct convention — reuse `internal/stability/reconcile.WorktreePathFor` (or the underlying `worktree.pathFor`-equivalent) rather than reinventing it. Every test fake that currently stubs `CreateWorktree` at the wrong `wt-<laneID>` path must be updated to match, or the test's own internal consistency will mask the fix. **Done:** integrated at `7407950` (manual merge, native gate blocked by ledger/lease issues at the time; independently re-verified gofmt/vet/build/full test suite green).
- [x] **R2 (blocks Campaign):** Wire `PromoteTargetCAS`/`PromoteChangeACAS`/`PromoteTargetBCAS` and `fixture.VerifyAncestryIsolation` into the actual journey flow at the "Promote stage" transition, instead of a bare `AdvanceTrial(TrialPromoted)` state tick. **Done:** integrated at `963bbd5` via native `lucind-ai run`/CAS promotion (feature tip `b675a750`); four new tests assert real Git ref-state movement, fix-dependency ordering, CAS staleness rejection, and ancestry-violation rejection; independently re-verified gofmt/vet/build/full test suite green in an isolated worktree.
- [x] **R3 (blocks Campaign):** Extend `internal/stability/store`'s schema with real trial-lifecycle persistence (trial number, trial stage, relevant PGIDs) so a crash mid-Trial leaves enough on disk for `resume` to reconstruct "the exact interrupted stage," per spec. **Done:** integrated at `9f1265c` (feature tip `aaf4c25`) — new `trial_progress` table; `StateMachine` gains an injectable `SetOnTransition` observer (still no direct I/O itself); `ExecuteTrialJourneyLive` now chains the caller's `OnProcessStart` instead of discarding it; `RunCampaign` persists real stage/PGID-B on every transition; `stability resume` reads and prints persisted trial/stage/pgid_b (does not yet act on it — that remains R4). Scope note: only Change B's PGID is captured/persisted (Change A and the Fix Change's PGIDs are not) — a deliberate, orchestrator-approved scope narrowing, not an oversight. Independently re-verified: full diff read against the exact design given to the dispatch packet, all new tests read and confirmed to assert real state (DB rows, stdout, real subprocess kill+chained-callback), gofmt/vet/build/full test suite green in an isolated worktree.
- [ ] **R4 (blocks Campaign):** Make `stability resume` actually continue execution from the persisted stage (R3) on `DecisionSafe`, instead of only inspecting and exiting; thread real `ProcessGroups` through `InspectParams`/`AbortParams` from persisted PGIDs (R3) so orphan detection/termination is reachable.
- [ ] **R5 (non-blocking, should fix):** Wire `TrialTimeout`/`CampaignTimeout` as real enforced budgets (not just `DispatchTimeout`/`LaneTimeout`), or explicitly narrow the spec requirement if dispatch-level timeout alone is judged sufficient.
- [ ] **R6 (non-blocking, cheap):** Add a presence assertion for the `sleep 5` instruction to `TestFixtureDeterministicSyntheticPackets`.

## Next Step

R1-R4 are a materially larger remediation than any single follow-up packet dispatched so far this Change — they touch the core journey/persistence/resume architecture, not an isolated seam. This warrants explicit human scoping before any further dispatch: whether to tackle R1-R4 as one bounded follow-up arc (mirroring the Wave 6a-6e pattern), whether R3/R4 in particular need a short design note given they add real schema, or whether some finding changes the Change's own scope. Do not dispatch remediation packets until that scoping decision is made.
