# Synthesis Notes: Conflict Triage Fixture

## Unresolved Contradictions

None. The proposal's prepared-SHA split is a design decision, not a leftover draft fight: all three lenses already put JSON in `Candidate.Output` (`internal/reconcile/reconcile.go:105`) and left `Candidate.CandidateSHA` (`:107`) to `reconcile resolve` (`cmd/lucind-ai/cli.go:1445-1511`). The two questions the proposal forbade guessing (risk formula; production executor/model) stay open in `design.md`; every lens left them open.

## Coverage Gaps

None of the eight packet spine items are missing.

Skill-template drift, same class as the proposal notes (packet wins; not a missing spine item):

- Skill size budget is 800 words; this packet's 1800-word cap is what `openspec/changes/archive/` actually uses. Canonical `design.md` is 1228 words.
- Skill Step 4 Engram persist of the design artifact and Step 5 return block are superseded: output is `design.md`, this file, and `.lucind/result.json`.
- Skill heading "Migration / Rollout" is covered as **Rollback and Additivity**. Skill "Data Flow" is covered as **Flow and Invariants**.
- Skill "Dependencies" is not a separate heading; seams and consumers sit in decisions, file changes, and testing.

No lens specified an output-only Service persist API. `UpdateCandidateStatus` (`internal/reconcile/reconcile.go:848-908`) writes status, SHA, and failure reason, not `output`. `design.md` records an output-only update using existing `ledger.UpdateReconciliationCandidate` (`internal/ledger/ledger.go:1314-1338`) so Decision 1 can actually land JSON without flipping SHA. That is completing lens A's persist story from verified code, not a substituted architecture.

## Dropped Citations

Union of the three manifests: 80 unique `path:line` citations. Each range was opened in this worktree. None of the twelve proposal-synthesis drops were revived.

**Dropped claim (citation kept for the column, qualifier removed):**

1. **Lens C — `internal/ledger/schema.go:163` as a nullable `output` column.** Line 163 is `output TEXT NOT NULL DEFAULT ''`. Pre-existing TEXT column: true. Nullable: false. Rollback in `design.md` uses the column as additive storage, not as nullable.

**Retargeted (claim kept, range tightened to the lines that actually hold it):**

2. **Lens B — `internal/overlap/overlap.go:623-660` as `Classify` returning class.** `ClassRequired` returns at `:658-659`; the warning branch starts after. `design.md` cites `:623-659` (proposal ground truth).
3. **Lens C — `internal/reconcile/reconcile.go:213-336` as `CreateRequest`.** The function ends at `:335`. `design.md` cites `:213-335`.

Wide-but-true ranges (`attempt.go:687-880`, `claude.go:30-52`, `feature.go:118-133`) still contain the claimed symbols and were kept.

**Not retargets of dropped proposal citations** (correct ranges, different claims):

- `cli.go:1445-1511` / `:1501-1506` is `runReconcileResolve` (the dropped `:1398-1463` was still renew).
- `reconcile.go:157-168` is `NewService` as a test seam (the dropped `:163-165` was "writes triage JSON").
- Linked-worktree refusal on resolve is `cli.go:1478-1481` (the dropped `:1430-1433` prints renew success). Newly verified; not a retarget of the failed range.

## Architecture Divergence

None — all three converged on lens A's architecture, independently:

- New packages `internal/conflicttriage/` and `internal/conflicttriage/fixture/`; no public CLI.
- Triage JSON in existing `Candidate.Output`; humans exclusive on `CandidateSHA` via `reconcile resolve`.
- Fail-open invoker; reuse `ScanConflictMarkers` / `EnforceAllowedPaths` as invariants only; do not share `internal/resolve` prompts or `ErrSemanticAmbiguity`.
- Two sequential `lucind-ai run` dispatches; prefix-disjoint `allowed_paths`; shared `base_sha`.
- Offline A/B on registered `claude` / `opencode`; win = 3-hunk classification, not grading the prepared commit.
- Gate, CAS, overlap thresholds, CLI verbs, and web GET left unchanged; revert is additive.

What did not enter `design.md` because it is not product architecture:

- Lens A's third open question (sdd-design skill vs fan-out topology) — process; packet already set 1800 words and this notes file.
- Lens B listing `design-lens-b.md` as a change deliverable — meta.
- Lens C "nullable" on `output` — false (see Dropped Citations).
- Exact `allowed_paths` encoding so `internal/conflicttriage/` does not PathInScope-include `fixture/` — deferred by proposal notes as a design detail; `design.md` Decision 4 states it (enumerate agent package-root files).
