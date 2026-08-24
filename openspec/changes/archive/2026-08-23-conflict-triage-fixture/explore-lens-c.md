# Explore Lens C — Risks, Trade-offs & Spikes: Conflict Triage Fixture

## Technical Risks & Unknowns

| Risk / Unknown | Severity | Mitigation / Investigation Strategy | Existing seam (file:line) |
|---|---|---|---|
| **Fail-Closed Prompt Coupling**: If `conflict-triage` reuses `resolve` prompt machinery, it inherits fail-closed behavior on semantic ambiguity instead of proposing an arbitrary resolution with ratcheted risk. | High | Isolate `internal/conflicttriage/` prompt templates and invoker from `internal/resolve/candidate.go`. Unit test that business ambiguity generates a proposed commit and High risk. | `internal/resolve/candidate.go:26`, `internal/resolve/candidate.go:303-312` |
| **Worktree Escape & Dirty State**: Triage agent generating resolution commits might leave residual conflict markers or mutate files outside declared `allowed_paths`. | High | Enforce post-triage invariant checks: execute `ScanConflictMarkers` and `EnforceAllowedPaths` (4-way diff union) before recording any resolution candidate SHA. | `internal/resolve/candidate.go:48-94`, `internal/resolve/candidate.go:100-145` |
| **Fixture Merge Base / Overlap Gate Bypass**: Synthetic fixture branches missing a valid ledger base commit cause `overlap.Evaluate` to return `ErrNoMergeBase`, silently skipping `ClassRequired` evaluation. | Critical | Initialize fixture branches from an explicit committed `base_sha` registered in `features` table; verify `overlap.Classify` triggers `ClassRequired` on intersecting/nearby hunks. | `internal/run/attempt.go:743-747`, `internal/overlap/overlap.go:622-660` |
| **Resolution Invalidation on Target Tip Drift (TOCTOU)**: Overlap gate retry clears blocks only if `matchedOtherSHA == otherSHA`. If target feature tip advances after triage, resolution is invalidated and attempt re-blocks. | Medium | Fixture test harness must freeze target feature commits during blocked attempt retries until CAS promotion completes via `PromoteCASWithRunner`. | `internal/run/attempt.go:821-828`, `internal/integrate/integrate.go:151-173` |
| **CLI Resolution Rejection in Linked Worktrees**: `lucind-ai reconcile resolve` rejects execution when invoked from inside linked worktrees via `IsLinkedWorktree`. | Medium | Automated fixture harnesses must target the primary repo root explicitly or invoke `reconcile.Service.UpdateCandidateStatus` directly in internal tests. | `cmd/lucind-ai/cli.go:1430-1433`, `internal/worktree/worktree.go:278-292` |
| **Multi-Feature Admission Rejections**: Fixture dispatch batches mixing feature targets, bad parent refs (`main`, `lucind/*`), or diverging parent SHAs will be rejected prior to dispatch. | Low | Keep `conflict-triage-agent` and `conflict-fixture` strictly path-disjoint and dispatch as independent single-feature batches satisfying `ValidateParentRef`. | `internal/run/integrate_feature.go:39-75`, `internal/feature/feature.go:101-113` |
| **Claude Progress Stream Telemetry Degradation**: Unexpected output structures from `claude --output-format stream-json --verbose` could trigger stream degradation during A/B judge evaluation. | Low | Rely on fallback to raw stdout capture in `Claude.Run`; verify JSON decoder coverage across stream event types in `claude_stream.go`. | `internal/executor/claude.go:106-122`, `internal/executor/claude_stream.go:10-16` |

## Trade-offs Matrix

| Dimension / Choice | Advantages | Disadvantages | Operational Cost |
|---|---|---|---|
| **Package Separation (`internal/conflicttriage/` vs `internal/resolve/`)** | Preserves fail-closed guarantee of autonomous resolver while allowing triage agent's fail-open / risk-ratchet semantics to evolve independently. | Small amount of boilerplate for worktree setup and diff checking. | Low: Prevents dangerous semantic leakage into autonomous promotion. |
| **Fixture Realism (Real Ledger & Branches vs In-Memory/Mocks)** | Validates end-to-end SQLite schema (`.lucind/lucind.db`), `evaluateOverlapGate`, and CAS state transitions under actual production conditions. | Requires branch ref setup and post-test worktree cleanup (`lucind-ai worktree cleanup`). | Low: Full fidelity with zero-row ledger tables (`overlap_evidence`, `reconciliation_requests`). |
| **Judge Pair Diversity (`opencode` + `claude` vs Single Provider)** | A/B testing GPT-5.6-Sol against Claude Opus 5 uncovers model bias on business vs mechanical hunks without cross-billing ambiguity. | Requires maintenance of two distinct CLI wrappers and streaming decoders; double token consumption. | Medium: Pinned models (`claude-opus-5`, `openai/gpt-5.6-sol`) bounded by manual triage invocation. |
| **Risk Ratchet Model (Discrete Categorical vs Continuous Formula)** | Forced `High` classification for business decisions ensures predictable human review trigger and eliminates false precision. | Coarser granularity for multi-file composite overlaps. | Low: Simplifies UI badge rendering and rubric assertion checks. |

## Potential Spikes / Proof of Concepts

1. **Three-Hunk Collision Calibration Spike**:
   - Construct synthetic branch pair modifying one toy file with 1 business rule conflict (differing logic, both compile/pass) and 2 mechanical controls (slice literal union, rename-vs-edit).
   - Verify `overlap.Evaluate` (`internal/run/attempt.go:743`) and `overlap.Classify` (`internal/overlap/overlap.go:622-660`) emit `ClassRequired` and populate `overlap_evidence` rows.

2. **Fail-Open Triage Invoker Prototype Spike**:
   - Implement prototype prompt in `internal/conflicttriage/` that extracts conflicting hunks, chooses an arbitrary branch, marks rationale as ARBITRARY, and outputs a formatted resolution proposal without returning `ErrSemanticAmbiguity` (`internal/resolve/candidate.go:26`).
   - Validate against `internal/resolve/candidate.go:303-326` to confirm contrasting behavior.

3. **End-to-End Reconcile Promotion Lifecycle Spike**:
   - Execute full lifecycle: trigger `evaluateOverlapGate` (`internal/run/attempt.go:687-856`), approve request with `reconcile.Service.Approve` (`internal/reconcile/reconcile.go:406-460`), register resolution commit with `lucind-ai reconcile resolve` (`cmd/lucind-ai/cli.go:1397-1463`), and confirm retry attempt CAS promotion via `PromoteCASWithRunner` (`internal/integrate/integrate.go:151-173`).

## Out of Scope

- Modifying default overlap classification thresholds (`DefaultThresholds` in `internal/overlap/overlap.go:93-99`).
- Wiring reconcile POST endpoints into Web UI / `internal/serve/` (surface remains read-only).
- Modifying core CAS promotion or batch execution mechanics (`internal/integrate/integrate.go`, `internal/run/run.go`).
- Relaxing fail-closed rules in autonomous resolver (`internal/resolve/candidate.go:26,305`).
- Supporting simultaneous N-way (>2) feature reconciliation (`internal/run/attempt.go:873-893`).

## Open Questions

- [ ] Should the conflict fixture generator exist as a CLI diagnostic subcommand (`lucind-ai fixture create`) or exclusively as an internal test package under `internal/conflicttriage/fixture/`?
- [ ] What is the exact JSON schema for the triage agent's explanation payload recorded in `reconciliation_candidates.output` (`internal/reconcile/reconcile.go:105`)?
- [ ] How should the verify budget (wall clock vs test execution weight) be calculated before displaying to the human approver?
