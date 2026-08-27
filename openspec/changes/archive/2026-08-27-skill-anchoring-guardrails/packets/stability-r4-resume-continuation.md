---
id: stability-r4-resume-continuation
executor: agy
routed_by: verify.md Remediation Task R4 (blocks Campaign) — remaining half explicitly scoped by the current BLOCKED verify disposition as the sole open blocker
allowed_paths: ["cmd/lucind-ai/stability.go", "cmd/lucind-ai/stability_test.go", "internal/stability/campaign.go", "internal/stability/campaign_test.go"]
feature: native-stability-campaign
parent_ref: refs/heads/feature/native-stability-campaign
base_sha: 4c865d99b779e54b9abca3d4556b5f15acb73127
expected_parent_sha: 4c865d99b779e54b9abca3d4556b5f15acb73127
---

# Packet stability-r4-resume-continuation

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/stability-r4-resume-continuation · **Branch:** lucind/stability-r4-resume-continuation

## Goal

`lucind-ai stability resume` must actually continue a crashed Stability Campaign from its
persisted stage, not merely inspect and print it. When `reconcile.Inspect` returns
`DecisionSafe`, `runStabilityResumeWithDir` (`cmd/lucind-ai/stability.go:780-847`) currently
prints the campaign/decision/persisted-trial/stage and returns 0 — it never resumes execution.
When this packet is done, that same `DecisionSafe` path reconstructs the in-memory
`StateMachine` from the persisted `TrialProgress` (`internal/stability/store/store.go:60-69`:
`CampaignID`, `TrialNumber`, `Stage`, `PGIDA/B/Fix`) and continues the campaign — calling into
the same journey/trial-execution machinery `RunCampaign` uses (`cmd/lucind-ai/stability.go:441`,
`internal/stability/campaign.go:664` `ExecuteTrialJourneyLive`, `campaign.go:389`
`StartNextTrial`) — until the campaign reaches a terminal state or fails again.

## Why this is safe to dispatch now

This is the sole remaining blocker in `openspec/changes/native-stability-campaign/verify.md`'s
BLOCKED disposition (candidate `51e31ed`, re-verified independently by the Orchestrator against
this exact commit range). R1 (worktree path convention), R2 (real CAS promotion wiring), R3
(trial-lifecycle persistence schema + PGID-B capture), and R7 (cross-Trial worktree cleanup) are
already integrated and independently re-verified green on this feature's current base
(`4c865d99b779e54b9abca3d4556b5f15acb73127`, itself the result of merging in
`dev`/acceptance-verifier plus two additional independently-verified fixes this session). R4's
`ProcessGroups`-threading half (`pgidsForActiveCampaign`, `stability.go:753`) is already done and
wired into both `resume` (`stability.go:816`) and `abort` (`stability.go:891`). Only the
resume-continuation half remains. R5/R6/R8 are explicitly non-blocking per verify.md and are
intentionally out of scope for this packet — do not touch them.

## Preconditions

- Working tree clean at dispatch time (enforced by `lucind-ai run`'s worktree creation from
  `base_sha`, not something this packet must check itself).
- `go build ./... && go vet ./...` pass on `base_sha` (independently confirmed via
  `lucind-ai check`, `status: passed`, immediately before this dispatch).

## Done criteria

- [ ] **Every indirection introduced is demonstrably consumed by a terminal consumer.** For any
      new field, method, or constructor added to `StateMachine` (or elsewhere) to support
      resuming from persisted state, name the exact call site in
      `runStabilityResumeWithDir` that consumes it, with the real command output proving the
      call compiles and runs.
- [ ] **The work is committed.** Evidence: `git status --porcelain` empty and
      `git log --oneline -1`. Conventional commit, no AI attribution.
- [ ] On `DecisionSafe`, `runStabilityResumeWithDir` reconstructs `StateMachine` state
      (current trial, consecutive-passes count, active trial stage) from the persisted
      `TrialProgress` row — not from a fresh `NewStateMachine()` — and resumes the same
      trial-execution loop `RunCampaign` uses, through to a terminal campaign state or a clean
      re-failure. `consecutivePasses` before the persisted trial is derivable as
      `TrialNumber - 1`, since the zero-retry-reset design (`campaign.go` `RecordTrialOutcome`)
      means any prior trial failure would have reset the counter and the persisted trial itself
      would not be in-flight.
- [ ] A new test in `cmd/lucind-ai/stability_test.go` (or `internal/stability/campaign_test.go`
      if the reconstruction logic lives there) proves resume actually re-enters trial execution
      after a simulated crash mid-Trial — not merely that it prints the persisted values. Use
      the same real-subprocess/real-git-worktree fixture style already established in
      `campaign_test.go`'s `newJourneyTestDeps` (recently updated this session to give lanes
      real git worktree identity for the acceptance-verifier freeze-done-candidate invariant —
      reuse that pattern, do not reinvent a non-git fixture).
- [ ] Full `go build ./...`, `go vet ./...`, and `go test ./...` green. Run the target package's
      tests with `-count=5` at minimum for anything touching the campaign/resume path, since this
      codebase has known timing-sensitive tests in this exact area (see Context below) — a
      single green run is not sufficient evidence for this packet.
- [ ] `gofmt -l` reports nothing for changed files.

## Allowed paths

- `cmd/lucind-ai/stability.go`
- `cmd/lucind-ai/stability_test.go`
- `internal/stability/campaign.go`
- `internal/stability/campaign_test.go`

## Allowed paths outside the repository

None.

## Out of scope

- R5 (enforcing `TrialTimeout`/`CampaignTimeout` as real timers) — non-blocking, not this packet.
- R6 (fixture test presence-assertion for the `sleep 5` instruction) — non-blocking, not this packet.
- R8 (`GetActiveCampaign` only matching `running` status, breaking retried `abort` against
  `blocked_cleanup`) — non-blocking, not this packet, and touches `store.go` which is outside
  `allowed_paths` here.
- Do not touch `internal/run/run.go`'s freeze-done-candidate logic (acceptance-verifier's
  fail-closed invariant) or `internal/stability/store/store.go`'s schema.
- Do not attempt to run a real, full 3-Trial `lucind-ai stability run` live campaign as part of
  this packet's own verification — that is a separate, much longer-running activity the
  Orchestrator will decide on after this packet lands and re-verify passes.

## Hard stops

- Any credential value would need to be chosen, generated, or written.
- A done-criterion turns out to be impossible, or already true for a reason this packet did not
  anticipate.
- The change would break something outside `allowed_paths`.
- Two reasonable implementations exist for how `StateMachine` should expose/accept resumed state
  (e.g., a new constructor vs. exported setters vs. a resume-specific type) and nothing here
  says which — if you hit this, stop and ask rather than guessing; do not spend budget on two
  parallel implementations.
- Satisfying one instruction in this packet would require violating another.

## Context

- `openspec/changes/native-stability-campaign/verify.md` — full BLOCKED disposition, findings
  1-4 confirmed with independent re-verification, R1-R3/R7 already integrated, R4 half-done.
  Read this file first; it is the authoritative scope statement for this packet.
- `cmd/lucind-ai/stability.go:780-847` — `runStabilityResumeWithDir`, the function to change.
- `cmd/lucind-ai/stability.go:441` — `RunCampaign`, the fresh-campaign path whose journey loop
  this packet must resume into rather than duplicate.
- `internal/stability/campaign.go:304-434` — `StateMachine` struct and its current
  constructor/transition methods (`NewStateMachine`, `Start`, `StartNextTrial`,
  `AdvanceTrial`, `RecordTrialOutcome`); all fields are unexported today.
- `internal/stability/store/store.go:60-69` — `TrialProgress` struct: `TrialNumber`, `Stage`
  (string), `PGIDA/B/Fix` (`*int`), no `ConsecutivePasses` field — must be derived, not stored.
- `cmd/lucind-ai/stability.go:753` — `pgidsForActiveCampaign`, already wired into both resume
  and abort; do not change its signature unless strictly necessary.
- Known timing-sensitive area: `TestStabilityRunProductionDefaultWiresLiveJourney`
  (`cmd/lucind-ai` package) previously failed ~70-80% of the time due to a lease-reclaim clock
  race, fixed this session in `internal/stability/campaign.go`'s `ReclaimTargetLease`
  (bounded 100ms/5s retry on transient `ErrLeaseHeld`) — already on this packet's `base_sha`.
  If new tests in this same area show similar transient timing failures, apply the same
  retry-on-transient-rejection pattern rather than a longer fixed sleep.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate against
`.lucind/result.schema.json` in this worktree before writing. Report `done` only when every
done-criterion carries evidence and every hard stop is declared.
