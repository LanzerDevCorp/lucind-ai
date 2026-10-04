# lucind-cleanup

## Objective

Close the non-blocking leftovers of `agy-only-contract` (see `odd/tasks/agy-only-contract.md`,
Leftovers) on branch `feature/agy-only-contract`.

## Tasks

Route: delegated to one Claude writer subagent (multi-file; agy out of quota).

- [x] **C1 — Static agy model list.** `internal/executor/model.go` lists exactly the models
  `agy models` reports on 2026-10-03 (18: gemini 3.8/3.7/3.6 flash high|medium|low, gemini-3.1-pro
  high|low, claude-opus-5-5 / claude-sonnet-5-5 low|medium|high, gpt-oss-120b-medium). Default
  model → `gemini-3.8-flash-high` (D-C1). Tests updated.
- [x] **C2 — Stale docs.** Delete `docs/agent-rules.md`, `docs/explore.md`, `docs/router.md`,
  `docs/usage-log.md`, `docs/feature-parent-integration.md`; rewrite `CONTEXT.md` as the glossary
  of the new domain (lane, brief, base/final tree, result envelope, receipt, attestation, allowed
  globs, free agy vs lane, plugin). Fix any links to deleted docs.
- [x] **C3 — Stray files.** Delete `lucind-lane-check.sh`, `templates/project-routing.md`
  (and `templates/` if empty), and the committed SQLite file `lucind.db.backup`.
- [x] **C4 — Result schema naming.** Rename `packet_id` → `lane_id` and "packet" wording in
  `internal/result/result.schema.json` and all producers/consumers (Go, agy plugin skill/rule,
  dispatch footer, Claude skill, docs). Breaking change; nothing external depends on it (D-C2).

## Decisions

- D-C1: default agy model becomes `gemini-3.8-flash-high` (newest flash high).
- D-C2: `packet_id` renamed to `lane_id` without a compatibility alias.

## Progress / evidence

- Route: one Claude Sonnet writer. Commits `e8fa769` (C1), `be8c9ab` (C2), `cace3db` (C3),
  `b38c230` (C4). RED/GREEN observed for C1 and C4. Orchestrator re-run: `sh lucind-checks.sh`
  EXIT 0; schema requires `lane_id`; plugin re-registered with renamed assets (`lucind-ai b38c230`).
  Remaining `packet` mentions are intentional ("removed on purpose", glossary "_Avoid_").
- Kept `docs/attestation.md` (linked from product.md). Pre-existing gofmt drift in 7 files left as is.

## Next step

Done. Optional: `gofmt -w` the 7 pre-existing unformatted files.
