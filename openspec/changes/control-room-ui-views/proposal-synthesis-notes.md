# Synthesis Notes: Control Room UI Views

## Unresolved Contradictions

None. The three propose drafts do not assert incompatible approaches. Reconciliation mutation (HTTP POST vs copy-paste CLI), overlap `evidence_json` formatting, and expiry countdown source are shared open questions, not competing claims. The code does not settle those three; they are carried in `proposal.md` Open Questions, matching `openspec/changes/control-room-ui-views/explore.md:82-86`.

## Coverage Gaps

None. Packet spine items 1–9 appear in at least one lens draft (A: summary, candidate, concepts, alternatives; B: impact table and delta specs; C: risks, rollback/additivity, tests, out of scope; all three: open questions). No propose lens emitted a dedicated Success Criteria or Dependencies section; those are skill-template extras, not packet spine items, so they are not treated as gaps. How envelope fields other than demotion `lane_note` (`internal/run/run.go:423-430`) would populate a DTO is unspecified because `internal/ledger` stores no envelope blob; that is recorded under Scope Divergence, not as a missing spine heading.

## Dropped Citations

Claims removed from `proposal.md` because the cited range did not support them. Neighboring claims were kept only with a different, verified citation.

### Lens A

- **`internal/ledger/schema.go:76-92` — Model lacks queries for `runs` / lanes / approvals.** Lines 76–92 are the `events` index close and `migrateV2ToV3DDL` creating `approvals`. There is no `CREATE TABLE runs` in this repository. Lanes live at `internal/ledger/schema.go:18-32`. The true gap (Model has no lanes/approvals methods) is kept via `internal/serve/model.go:26-125`. The `runs` table claim is dropped.

- **`internal/serve/static/app.js:1-98` as currently “organized into tabbed view panels.”** That file is the approvals-only poller (`fetchState` / `renderState` / per-card POST). Tabs are the proposed rewrite of this file, not present state. The approach still names this file as the client seam.

### Lens B

- **`internal/serve/model.go:109-110` — “Candidate output diffs MUST be constrained to 400 lines per resolution rules.”** Those lines are unbounded `Output` and `Checks` strings on `ReconciliationCandidate`. `MaxConflictLines = 400` is `internal/resolve/resolve.go:16-18` (git conflict-marker blocks), not UI truncation of candidate `output`. The 400-line MUST is dropped; candidate inspection without a line cap is kept via `internal/serve/model.go:101-115`.

- **“4-way overlap evidence” citing `internal/ledger/schema.go:96-139`, `internal/serve/model.go:27-70, 128-266`, `internal/feature/feature.go:48-93, 293-360`.** Overlap rows classify `required` / `warning` / `informational` (`internal/ledger/schema.go:136`). The four-way union is git path collection in `enforceAllowedPaths` (`internal/run/run.go:403-407, 576-582`). Feature/lease/overlap monitoring is kept without “4-way.”

- **“The UI MUST inspect `.lucind/result.json` envelopes against `internal/result/result.schema.json:1-98`.”** Ledger has no envelope column (no `result.json` / `Envelope` in `internal/ledger`). `TestModelSourceDoesNotShellOut` forbids `os` and `os/exec` in `model.go` (`internal/serve/model_test.go:610-618`). Schema lines 1–98 cover through `done_criteria`; `deviations` starts at line 125. Filesystem inspection of the worktree envelope is dropped. Demotion diagnosis via persisted `lane_note` plus `run.go:650-652` is kept.

### Lens C

- **`internal/result/schema_test.go:1-80` — existing tests that inspect envelope `hard_stops` / `deviations` / `done_criteria` for the UI.** The file is 33 lines: `TestSchemaJSONParsesAsJSON` and `TestSchemaJSONReturnsDefensiveCopy` only. Not a UI or field-inspection seam. Dropped from Test impact.

- **“Data races with concurrent execution” if batch views query the ledger without Model (`internal/serve/model.go:14-24`).** Those lines state Model does not run git or shell. Skipping Model does not introduce a SQLite race. The shell-free DTO requirement is kept; the data-race claim is dropped.

- **`internal/serve/static/index.html:1-140` as a “sidebar navigation bar.”** Lines 1–140 are CSS through `</head>`. There is no sidebar. The file is 162 lines; the body (approvals header) starts at 141. Out-of-scope ownership of chrome/CSS tokens is kept via `:root` tokens at lines 8–17, without claiming a sidebar exists.

## Scope Divergence

Lens A’s Candidate 2 (modular REST + lazy vanilla panels, `explore.md:3, 20-23`) is authoritative. Approaches were not irreconcilable.

**Lens B vs A.** B did not pick a competing candidate. Its five-view inventory matches A’s panels (Batch/Wave, Approvals, Feature/Lease, Reconciliation, Lane envelope). Cost: B added a 400-line candidate-diff MUST, “4-way overlap,” and filesystem `.lucind/result.json` inspection. Those contradict either the code or A’s Model-backed, no-`os` query surface, so they did not enter `proposal.md`. B’s delta specs otherwise assume Model reads and anti-bulk approvals, which is A’s approach.

**Lens C vs A.** C independently listed additive GET routes matching A (`/api/approvals`, `/api/features`, `/api/leases`, `/api/overlap/…`, `/api/reconcile/requests`, `/api/batch/lanes`), lazy-load of heavy payloads, keep `GET /api/state`, and `git revert` with no schema bump. C omitted A’s `/api/features/{id}/attempts` (that route is in `proposal.md` because A owns the approach). C named overlap `{id}` vs A’s `{feature_id}`; A’s name is used. C’s `TestModelSourceDoesNotShellOut` constraint is treated as corroboration of A’s shell-free Model, not a fork.

**Independent convergence.** Zero-npm `embed.FS` SPA; loopback; Candidate 2 over monolithic `/api/state` and `html/template`; 2s poll only on hot endpoints; `escapeHtml` on agent strings; HTTP 400 bulk; Model as the query boundary; three shared open questions.
