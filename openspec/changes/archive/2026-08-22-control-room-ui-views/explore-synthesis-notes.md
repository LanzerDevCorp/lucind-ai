# Synthesis Notes: Control Room UI Views

## Unresolved Contradictions

**Reconciliation mutation in the UI.** Lens A leaves open whether approve/decline/cancel/renew/resolve should be HTTP POST on `lucind-ai serve` or copy-paste CLI (`explore-lens-a.md` Open Questions, citing `cmd/lucind-ai/cli.go:1043-1441`). Lens B Scenario 4 and capability 4 assert action buttons that approve, decline, cancel, or resolve (`explore-lens-b.md`, citing `cmd/lucind-ai/cli.go:56` and `internal/reconcile/reconcile.go:122-260`). Lens C's CAS spike assumes individual action endpoints (`explore-lens-c.md`, citing `cmd/lucind-ai/cli.go:1067-1100`). Code: CLI dispatch exists (`cmd/lucind-ai/cli.go:1043-1065`); `NewHandler` registers `/`, `/api/state`, and `/approvals/` only (`internal/serve/handlers.go:36-118`); `reconcile.Service` has `Approve`/`Decline`/`Cancel`/`Renew` but no `Resolve` method (Resolve is CLI-only at `cmd/lucind-ai/cli.go:1375`). Not settled. Not picked.

## Coverage Gaps

None. All eight exploration-spine items appear in at least one draft. Skill `Ready for Proposal` is answered in `explore.md`; packet spine does not list it as a gap.

## Dropped Citations

Citations that did not resolve, pointed at unrelated code, or did not support the claim. Claims below are omitted from `explore.md`. Neighboring claims kept only when a *different, verified* citation supported them.

### Lens A

- **`internal/ledger/ledger.go:131-148` — "Ledger already exposes all required aggregation queries."** Those lines are the `Ledger` struct and `Open()`. Aggregation for the UI is on `serve.Model` (`internal/serve/model.go:128-343`), which was kept via that citation.
- **`internal/reconcile/reconcile.go:89-96` — "POST endpoints invoking `reconcile.Service`."** Those lines are `Request` JSON fields (`TargetSHA` … `UpdatedAt`). `Service` is at `116-122`. Candidate 1 still described as extending handlers; the Service cite was dropped.

### Lens B

- **`internal/ledger/schema.go:156-172` — `feature_leases`.** Those lines are `reconciliation_candidates`. Leases are `122-129`. Lease capability kept via `internal/feature/feature.go:293-360` and `internal/serve/model.go:52-59`.
- **`internal/ledger/schema.go:174-190` — `overlap_evidence`.** Those lines are `integration_events` plus the v5 migration comment. Overlap table is `131-139`. Overlap payload kept via `internal/serve/model.go:62-70`.
- **`internal/dag/overlap.go:52-78` — "4-way diff overlap evidence."** That function is `ValidateGlobalOverlap` over DAG packet `allowed_paths`, not feature-parent overlap evidence (`internal/overlap/overlap.go`). Four-way git union lives on lanes (`internal/run/run.go:576-582`), not this view.
- **`internal/ledger/schema.go:206-230` — `reconciliation_requests`.** Those lines are `migrateV4ToV5DDL` `INSERT INTO lanes_new`. Requests are `141-154`. Reconciliation kept via `internal/reconcile/reconcile.go:35-120` and `internal/serve/model.go:74-92`.
- **`internal/ledger/schema.go:232-260` — `reconciliation_candidates`.** Those lines are `migrate()` startup. Candidates table is `156-169`.
- **`internal/ledger/schema.go:262-280` — `integration_events`.** Those lines are `migrate()`. Events table is `171-179`. Audit kept via `internal/serve/model.go:117-125`.
- **`internal/resolve/candidate.go:25-90` — "AI resolution candidate diffs bounded to 400 lines."** That range is `CandidateOptions` / `ScanConflictMarkers`. `MaxConflictLines = 400` is `internal/resolve/resolve.go:16-18`. The 400-line bound is not restated in `explore.md`.
- **`internal/reconcile/reconcile.go:122-260` — operator action triggers approve/decline/cancel/renew/resolve.** That range is `ServiceOption`, direction helpers, and `CreateRequest`. `Approve` is at `406`; `Decline` `538`; `Cancel` `596`; `Renew` `666`; there is no `Service.Resolve`.
- **`internal/ledger/ledger.go:307-353` — personal wrong-approval defect rate.** Those lines scan `lanes` / `LaneStates`. `ApproverRate` is `797-814`. Rate kept via `internal/serve/handlers.go:120-146`.
- **`internal/ledger/ledger.go:230-264` — approvals with attached test output.** Those lines are `Lane` and `RegisterLane`. Not used.
- **`internal/lane/status.go:22-26` — "running" status.** Those lines are `Terminal()` cases `Done, Blocked, Deviated, Failed`. `Running` is line 12. Status list kept via `10-16` and `31-38`.
- **`cmd/lucind-ai/cli.go:350-380` — verbatim question relays for blocked lanes.** Those lines print the attempt id after a feature-targeted run and start `runSplit`. Envelope `questions` exist in `internal/result/result.schema.json` (`questions` property); the CLI-relay claim is dropped.
- **`internal/result/schema.go:20-65` — deep inspection of `.lucind/result.json` envelopes.** Those lines compile the embedded schema (`schemaResourceURL`, `mustCompileSchema`). Envelope shape kept via `internal/result/result.schema.json`.
- **`internal/run/run.go:549-574` — mandatory hard-stop declaration checks.** `decideStatus` reads the envelope and maps status; it does not inspect `hard_stops`. Hard-stop *requirement* kept via `docs/prd.md:181-185` and the schema file.
- **Success criterion "every view maps to `internal/serve/model.go:27-125`."** Those structs are Feature/Attempt/Lease/Overlap/Reconciliation/Audit only. Batch/wave has no Model type; lanes live in `internal/ledger/schema.go:18-32`. Criterion recast in `explore.md`.

### Lens C

- **`internal/serve/static/index.html:1-140` as proof of "sidebar navigation layout."** That range is dark-theme CSS and the approvals header. There is no sidebar. Out-of-scope ownership of shell chrome is kept as a planning boundary, not as a claim that a sidebar exists.
- **Trade-off "monolithic `/api/state` already aggregates features, approvals, and reconciliations" (`handlers.go:120-146`).** That handler returns approvals + rate + opencode command only. Cited in `explore.md` as the *existing* single state handler (a place Candidate 1 would grow), not as a current unified snapshot.

## Approach Divergence

Lens A's problem (approvals-only HTTP vs unread Model) and three candidates are primary. Lens C independently drew the same three axes: hash SPA vs multi-page HTML (Candidates 1/2 vs 3), monolithic `/api/state` vs granular `/api/*` (1 vs 2), `<pre>` vs custom formatter. C's payload-bloat and DOM-rebuild risks make Candidate 1 worse and Candidate 3 worse (reloads); they do not make Candidate 2 unviable. C's open question on aggregated vs lazy fetch is treated as corroboration of Candidate 2, not a leftover product fork.

Lens B assumed a five-view inventory (batch/wave + envelope inspector) where A named four tabs (approvals, features/leases, overlap, reconcile). Not irreconcilable: B is a superset. Canonical `explore.md` keeps B's extra views and records that `Model` has no lanes query (`internal/serve/model.go:26-125`). Cost of B's wrong schema line numbers: several table cites were dropped; the same entities were re-anchored on A's v4 DDL and on Model structs.

Lens B Scenario 4 and Lens C's CAS spike assumed UI mutation. Lens A did not. Escalated under Unresolved Contradictions.

Independent convergence: vanilla `go:embed` SPA, no npm (PRD §8.3), preserve anti-bulk (`handlers.go:161-176`), lazy-load `evidence_json`, poll as source of truth after POST (`app.js:72-89`).
