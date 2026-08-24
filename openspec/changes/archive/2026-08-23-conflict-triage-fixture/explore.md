# Explore: Conflict Triage Fixture

Two halves, one change: implement the designed-but-absent `conflict-triage` agent under `internal/conflicttriage/`, and a reproducible fixture that forces a `required` overlap on demand. Reconciliation is production-wired and has never fired in this clone.

## Problem

`evaluateOverlapGate` (`internal/run/attempt.go:687`) compares an attempt's candidate against every other active feature via `overlap.Evaluate`, which calls `overlap.Classify` (`internal/overlap/overlap.go:622-660,1053`). `ClassRequired` fires on predicted merge-tree conflict, rename/delete collision, shared binary, intersecting hunks, nearby hunks, or hotspot weight ≥ 0.50 (`DefaultThresholds`, `internal/overlap/overlap.go:93-98`). The gate then creates a reconciliation request (`internal/reconcile/reconcile.go:213-336`), sets `AttemptStatusBlocked`, and releases the lease (`internal/run/attempt.go:848-854`).

Ledger (this clone): 36 `integration_attempts`, 0 `overlap_evidence`, 0 `reconciliation_requests`, 0 `reconciliation_candidates`. Two plain git branches that conflict never enter this path — the feature layer is mandatory. Batch admission (`internal/run/integrate_feature.go:26-78`) refuses mixed feature targets, mixed legacy/feature packets, divergent `expected_parent_sha`, and `parent_ref` that is `main`, inside `lucind/`, or empty (`internal/feature/feature.go:101-113`).

Clearing a required block is two steps, not one: `reconcile approve` authorizes a direction (`cmd/lucind-ai/cli.go:1090-1195`); a human resolves out of band and registers the SHA with `reconcile resolve --candidate <id> --sha <sha>` (`cmd/lucind-ai/cli.go:1398-1463`). Retry adopts that SHA only if the request is approved, the candidate is `integrated`, and the other tip is unchanged (`internal/run/attempt.go:821-828,870`). The existing resolver must not choose direction (`ErrSemanticAmbiguity`, `internal/resolve/candidate.go:26,303-305`). `internal/conflicttriage/` does not exist yet.

Worktrees live under `../<repo>-worktrees/<laneID>` on `lucind/<laneID>` (`internal/worktree/worktree.go:79-81,150-185`). A blocked lane's tree is kept until `lucind-ai worktree cleanup --lane <id>` (`cmd/lucind-ai/cli.go:56`).

## Candidates

| Approach | Pros | Cons | Feasibility |
|---|---|---|---|
| **1. Two disjoint features + dual-judge rubric** (recommended) | Matches the decided split (`conflict-triage-agent` owns `internal/conflicttriage/**`; `conflict-fixture` owns generator, judge packets, rubric); path-disjoint (`internal/packet/disjoint.go:29-48`); real ledger and git; A/B across pinned executors (`cmd/lucind-ai/cli.go:65-70`) | Two separate `lucind-ai run` invocations; offline rubric | High |
| **2. In-memory harness + sync LLM in the gate** | Single test package; no feature batches | Couples `evaluateOverlapGate` to LLM latency; empty shared `base_sha` → `ErrNoMergeBase` skip (`internal/run/attempt.go:743-747`); no real worktrees (`internal/worktree/worktree.go:1-14,150-185`) | Medium technically, **unviable** operationally |
| **3. Production CLI `fixture generate` / `reconcile triage`** | Matches existing verb UX (`cmd/lucind-ai/cli.go:56`) | Public CLI before prompts and the risk formula stabilize | High, premature |

Recommend **Candidate 1**. Candidate 2 is rejected: a missing merge base silently continues past classification, and a synchronous LLM call in the promotion loop is an unacceptable failure coupling. Candidate 3 can wrap an internal package later; do not grow `cli.go` until judges calibrate.

The intended collision shape holds: two features edit one toy file with three hunks — one BUSINESS (both versions compile and pass their own tests; no technical criterion exists to choose) plus two mechanical controls (slice-literal union; rename colliding with an edit to the old name). `Classify` already treats intersecting hunks and rename/delete as required (`internal/overlap/overlap.go:634-650`). A judge that scores all three hunks alike fails, in either direction. The two *build* features must not collide with each other.

## User and capability impact

- **Operators** get an on-demand `ClassRequired` so approve → resolve → retry can be practiced against the real CLI (`cmd/lucind-ai/cli.go:56,1090-1195,1398-1463`).
- **`conflict-triage`** turns a merge conflict into a ~30-second human decision: explain the *cause*, leave a prepared resolution commit, and state what accepting it unverified risks versus what verifying costs. On a business choice it must say the proposal is ARBITRARY, say why that side was picked, and ratchet risk to high. It does **not** fail closed — keep that discipline in `internal/resolve`.
- **Judges** score the same 3-hunk fixture across executors. Code registers four (`agy`, `claude`, `cursor-agent`, `opencode` at `cmd/lucind-ai/cli.go:65-70`); the skill frontmatter still lists three and omits `claude` — the code is authoritative. Pinned models: `gemini-3.7-flash-high` (`internal/executor/agy.go:86-88`), `claude-opus-5` (`internal/executor/claude.go:35`), `cursor-grok-4.6-high` (`internal/executor/cursor_agent.go:37-38`), `openai/gpt-5.6-sol` (`internal/executor/opencode.go:53-54`). Intended A/B pair: `opencode` vs `claude`.

## Scenarios

1. **Fixture fires `ClassRequired`.** Two leased features share one `base_sha`. The fixture injects the 3-hunk toy file. The gate classifies required (`internal/overlap/overlap.go:659`), `CreateRequest` persists overlap evidence and an awaiting request (`internal/reconcile/reconcile.go:266,213-336`), and the attempt blocks (`internal/run/attempt.go:848`). Shared `base_sha` is mandatory: without it, `Evaluate` returns `ErrNoMergeBase` and the gate continues (`internal/run/attempt.go:743-747`).
2. **Triage splits business vs mechanical.** Mechanical hunks get a deterministic proposal. The business hunk is labeled ARBITRARY, risk forced high, commit left prepared. Fail-closed `ErrSemanticAmbiguity` is the wrong behavior here (`internal/resolve/candidate.go:26`).
3. **Operator closes the loop.** `reconcile approve` (`internal/reconcile/reconcile.go:406-535`) then `reconcile resolve`, then `worktree cleanup --lane <id>`, then re-dispatch. Retry promotes `cand.CandidateSHA` (`internal/run/attempt.go:870`) via CAS (`internal/integrate/integrate.go:151-173`). Freeze the other feature's tip or the matching-SHA check fails (`internal/run/attempt.go:821-828`). Resolve must run from the primary root (`cmd/lucind-ai/cli.go:1430-1433`; `internal/worktree/worktree.go:278-292`).
4. **Admission.** Mixing the two *build* features in one batch, or `parent_ref` `main` / `lucind/*` / empty, is rejected before dispatch (`internal/run/integrate_feature.go:17,41,73`).
5. **Judge A/B.** Same fixture, `claude`/`claude-opus-5` vs `opencode`/`openai/gpt-5.6-sol`. Rubric scores cause quality, high-risk ratchet on the business hunk, and prepared-commit validity. No cross-provider billing (`cmd/lucind-ai/cli.go:65-70`).

## Risks and trade-offs

| Risk | Severity | Seam |
|---|---|---|
| Reusing `resolve` prompts inherits fail-closed | High | Isolate `internal/conflicttriage/`; `internal/resolve/candidate.go:26,303-312` |
| Residual conflict markers or edits outside `allowed_paths` | High | Reuse `ScanConflictMarkers` and `EnforceAllowedPaths` (`internal/resolve/candidate.go:48-94,100-145`) |
| Missing shared `base_sha` skips required | Critical | Register one `base_sha` on both fixture features (`internal/run/attempt.go:743-747`) |
| Other tip moves after triage (TOCTOU) | Medium | Freeze the target until CAS (`internal/run/attempt.go:821-828`) |
| `reconcile resolve` from a linked worktree | Medium | Invoke from the primary root (`cmd/lucind-ai/cli.go:1430-1433`) |
| Mixed-feature fixture dispatch | Low | Separate `lucind-ai run` invocations (`internal/run/integrate_feature.go:39-75`) |
| Claude `stream-json` decode during A/B | Low | Fallback already in `Claude.Run` (`internal/executor/claude.go:106-122`; `internal/executor/claude_stream.go:10-16`) |

| Choice | Why | Cost |
|---|---|---|
| Separate `conflicttriage` vs `resolve` | Fail-open + risk ratchet must not leak into autonomous merge | Small duplicated invariant checks |
| Real ledger/branches vs mocks | Only way to populate the 0-row tables and exercise CAS | Worktree cleanup |
| `opencode` + `claude` judges vs one provider | Heterogeneous bias on business vs mechanical hunks; anti-cross-billing | Two wrappers, double tokens |
| Discrete High vs continuous risk | Predictable human trigger until a formula exists | Coarser multi-file cases |

## Spikes

1. **Three-hunk calibration.** Confirm `Evaluate`/`Classify` emit `ClassRequired` and `CreateRequest` inserts evidence (`internal/run/attempt.go:743`; `internal/overlap/overlap.go:622-660`; `internal/reconcile/reconcile.go:266`).
2. **Fail-open invoker.** Business hunk → ARBITRARY + high risk + prepared commit; must not return `ErrSemanticAmbiguity` (`internal/resolve/candidate.go:26,303-326`).
3. **Full lifecycle.** Gate → `Approve` → `reconcile resolve` → retry CAS (`internal/run/attempt.go:687-856`; `internal/reconcile/reconcile.go:406-460`; `cmd/lucind-ai/cli.go:1398-1463`; `internal/integrate/integrate.go:151-173`).

## Success criteria

- Fixture deterministically yields `ClassRequired` (`internal/overlap/overlap.go:622-660`) and an `overlap_evidence` row via `CreateRequest` (`internal/reconcile/reconcile.go:266`; table `internal/ledger/schema.go:131-139`).
- Gate creates an awaiting `reconciliation_requests` row (`internal/ledger/schema.go:141-154`) and blocks (`internal/run/attempt.go:848-856`).
- Agent resolves mechanical hunks, surfaces the business hunk as ARBITRARY with mandatory high risk and a prepared SHA, without fail-closed (`internal/resolve/candidate.go:26,305`).
- `reconcile approve` + `reconcile resolve` then retry promotes `CandidateSHA` (`cmd/lucind-ai/cli.go:1090,1398`; `internal/run/attempt.go:821-871`).
- Rubric runs on supported executors (`cmd/lucind-ai/cli.go:65-70`) without cross-executor config leaks.
- A judge that scores all three hunks the same way fails.

## Out of scope

- Changing `DefaultThresholds` (`internal/overlap/overlap.go:93-99`).
- Wiring reconcile POST into the web UI (`internal/serve/handlers.go:95-109` stay GET).
- Changing CAS or batch dispatch (`internal/integrate/integrate.go`, `internal/run/run.go`).
- Relaxing fail-closed `internal/resolve` (`internal/resolve/candidate.go:26,305`).
- N-way (>2) reconciliation (`internal/run/attempt.go:873-893`).
- Colliding the two *build* features with each other.
- Unifying `conflict-triage` with `resolve`.

## Open questions

- Verify-budget units: tokens, wall clock, test weight, or pricing?
- Exact non-decreasing risk formula and thresholds, including mixed business+mechanical hunks. The fixture exists to produce that data.
- Fixture packaging: `test/fixture/`, `internal/fixture/`, `internal/conflicttriage/fixture/`, and/or a later CLI diagnostic?
- JSON shape stored in `reconciliation_candidates.output` (`internal/reconcile/reconcile.go:105`).
- Which executor/model runs production triage (judges are `opencode` vs `claude`; runtime still open).

## Ready for proposal

Yes — Candidate 1, with the open questions above deferred to proposal/design rather than re-litigated as problem scope.
