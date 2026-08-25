# Verify: Ultrafixer

**Round 1 verdict: BLOCKED** (see below). **Round 2 (re-verify) verdict: PASSED.**

## Round 2 — Re-verify after remediation

Mechanical check re-run clean at `f5e3a7f` (21/21 packages, no flake). Dual qualitative
re-verification dispatched (`verify-ultrafixer-r2-agy`, `verify-ultrafixer-r2-cursor-agent`)
against candidate `95c426e`, both independently instructed to *confirm or refute* — not
trust — the remediation Lane's own claims.

Both judges now agree: **both round-1 confirmed violations are genuinely fixed**, independently
traced to terminal consumers:

- Violation 1 (linked-worktree refusal): `resolvePrimaryRoot` now resolves via `git rev-parse
  --git-common-dir` + `filepath.Dir` instead of `--show-toplevel`, so it correctly lands on the
  primary repository's ledger even when invoked from inside a linked worktree.
  `runFeatureStatus`/`runDefectRecord`/`runDefectList`/`runDefectDecline` no longer call
  `IsLinkedWorktree`; git-mutating verbs (`feature create`, `worktree cleanup`, `reconcile *`,
  `integrate retry`) still do, via the separate `gitShowToplevel` check — cursor-agent verified
  this distinction is deliberate and correct, not an accidental blanket relaxation.
  `TestLinkedWorktreeCommands` (`cmd/lucind-ai/cli_test.go:4532-4649`) proves it with a **real**
  `git worktree`, not just an in-process stub — closing the exact gap that let the original defect
  ship undetected.
- Violation 2 (no declined-disposition path): `Ledger.UpdateDefectDisposition` exists, validates
  via `DefectDisposition.Valid`, is wired to `lucind-ai defect decline --id <id>`, and the packet
  template instructs it. `TestDefectDeclineCLI` proves a decline actually flips the ledger row.

All four non-blocking findings confirmed addressed. One small residual, non-blocking gap
(`dependencies-defects.md` named the accept path but not `defect decline`) was fixed directly by
the orchestrator after the re-verify (commit `0bb86af`, plugin version bumped to 2.0.7) — trivial
doc-only addition, not worth a third dispatch round.

No regressions found in the original apply-phase work (schema v8, `RecordDefect`/`GetDefect`/
`ListDefects`, `defect record`/`defect list` CLI).

**Verdict: PASSED.** Ready for archive.

## Round 1

**Verdict: BLOCKED**

### Stage 1 — Mechanical Check

`lucind-ai check` at commit `13b6295` (log frozen and committed at `6ca101b`):
`status: passed`, all 21 packages green. One transient failure on an earlier run
(`TestConcurrentLeaseAcquisition`, `internal/feature`) was reproduced 3/3 clean in isolation and
matches a documented known full-suite timing/concurrency flake in
`plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md` — not a regression.
See `openspec/changes/ultrafixer/verify-mechanical.log` for the frozen transcript.

## Stage 2 — Dual Qualitative Judgment

Dispatched `verify-ultrafixer-agy` (executor `agy`) and `verify-ultrafixer-cursor-agent` (executor
`cursor-agent`) against candidate `6ca101b`. Both lanes reported envelope `status: done`, but their
findings substantively disagree — `agy` reported full compliance with no gaps; `cursor-agent`
surfaced concrete, confirmed violations `agy` missed entirely.

## Stage 3 — Evidence Cross-Checking

Per this repo's own Stage 3 disposition table, a `done`/`done` envelope pair alone is not proof —
every `cursor-agent` finding was independently re-verified against the actual source before
disposition. Two findings are **confirmed violations**, not disputed or false-positive:

### Confirmed violation 1 — origin/discovery/persistence CLI paths are unreachable from an ultrafixer Lane's own worktree

`runDefectRecord` (`cmd/lucind-ai/cli.go:1908-1911`), `runDefectList` (`cli.go:1967-1970`), and
`runFeatureStatus` (`cli.go:974-977`) all call `resolvePrimaryRoot` (git-toplevel of the *current*
working directory) and then `worktree.IsLinkedWorktree(primaryRoot)` — if true, they print
`"refusing to run from inside a linked worktree"` and return exit code 1. Independently verified
by reading all three call sites directly.

Ultrafixer's own design requires it to run as an ephemeral Lane inside its own isolated linked
worktree (`design.md`'s "Zero new Go dispatch plumbing" decision, `ultrafixer-packet-template.md`).
From inside that worktree, `git rev-parse --show-toplevel` resolves to the *linked* worktree path,
so `lucind-ai feature status` (Step 2's active-branch discovery) and `lucind-ai defect record`/
`lucind-ai defect list` (the entire non-critical/non-blocking Defect Record path) both refuse to
run. This is not a hypothetical edge case — it is the ordinary, every-dispatch code path, and it
was never exercised end-to-end: the new Go tests all call these functions in-process (not through
a real linked worktree), so the guard's interaction with ultrafixer's actual runtime shape was
never caught.

**Affects**: `ultrafixer-dispatch` spec Requirement "Independent two-axis evaluation and
multi-branch triage" (feature discovery), `defect-records` spec (the entire non-critical/
non-blocking persistence path).

### Confirmed violation 2 — no disposition-transition path; "declined" is never actually recorded

`internal/ledger/ledger.go` defines `DefectDispositionDeclined` (`:1446`) but has no
`UpdateDefect` or any other method that transitions an existing record's `disposition`.
`RecordDefect` (`:1464`) only inserts. Independently verified: no `UpdateDefect` symbol exists
anywhere in `ledger.go`, no CLI verb, no `serve` handler wires this either.

`ultrafixer-dispatch/spec.md`'s "Human declines fix" scenario is explicit: *"a declined disposition
MUST be recorded in the ledger"* — there is no code path that can satisfy this MUST today.

**Affects**: `ultrafixer-dispatch` spec Requirement "Isolated repair delivery and human-gated CAS
integration", Scenario "Human declines fix and worktree is preserved".

### Non-blocking findings (real, worth fixing in the same remediation pass, not independently blocking)

- Packet template's `**Tier:** B (auto-merge after audit)` label contradicts its own hard stop
  forbidding auto-integration — prose inconsistency, not mechanically enforced either way, but
  confusing for a human reading the template. (`ultrafixer-packet-template.md:13`)
- Packet template never states the conventional-commit / no-`Co-Authored-By` requirement that
  `ultrafixer-dispatch/spec.md:55` MUSTs. (`ultrafixer-packet-template.md`)
- `TestUltrafixerPacketTemplateContract` only asserts frontmatter + section-heading strings, not
  the actual protocol MUSTs (failing-command scope, isolated-worktree language, conventional
  commit, no-auto-integrate) that `tasks.md`'s own threat-matrix claimed it covers.
  (`internal/packet/packet_test.go:2043-2094`)
- `lucind-ai defect record --disposition <invalid>` is only rejected by the SQLite CHECK
  constraint, not by CLI-level flag validation — functionally correct (spec's own scenario only
  requires the database to reject it) but no test asserts the CLI's stderr/exit-code behavior for
  this case specifically. (`cmd/lucind-ai/cli.go:1897-1927`)

## Refuted / not confirmed

None of `cursor-agent`'s findings were false positives on independent re-check; all were either
confirmed (the two violations above) or genuinely non-blocking observations (the four items above).
`agy`'s findings were not wrong, just incomplete — it verified spec-text alignment and test
presence but did not trace the actual runtime call path an ultrafixer Lane would take from inside
its own worktree.

## Disposition

**BLOCKED.** Remediation required before archive:

1. Resolve the linked-worktree refusal for `lucind-ai feature status`, `lucind-ai defect record`,
   and `lucind-ai defect list` — either relax `IsLinkedWorktree` for these specific read/ledger-only
   verbs (they mutate only the shared primary-root ledger, not git refs or worktrees — unlike
   `feature create`/`worktree cleanup`, which is presumably why the guard exists there), or add an
   explicit `--repo <path>` override these three verbs accept and have
   `ultrafixer-packet-template.md` instruct passing the primary root path. Whichever direction is
   taken must be verified with a **real** linked-worktree invocation (not just an in-process
   Go test), since that's exactly the gap that let this ship undetected.
   - **Remediated:** Updated `resolvePrimaryRoot` in `cmd/lucind-ai/cli.go` to resolve the primary repository root via `git rev-parse --git-common-dir`, relaxed `IsLinkedWorktree` checks on `runFeatureStatus`, `runDefectRecord`, `runDefectList`, `runDefectDecline`, added in-process test `TestLinkedWorktreeCommands` (`cmd/lucind-ai/cli_test.go`), and verified end-to-end via execution from within a real linked worktree.
2. Add `Ledger.UpdateDefectDisposition` (or equivalent) and wire the "human declines fix" path to
   call it with `disposition=declined`, per the spec's scenario.
   - **Remediated:** Added `Valid()` and `UpdateDefectDisposition(ctx, id, disposition)` to `internal/ledger/ledger.go`, added CLI subcommand `lucind-ai defect decline --id <id>` in `cmd/lucind-ai/cli.go`, covered by unit tests in `internal/ledger/ledger_test.go` and `cmd/lucind-ai/cli_test.go`, and updated `plugin/claude-code/skills/lucind-ai/assets/ultrafixer-packet-template.md` to instruct `defect decline` / `disposition=declined`.
3. Fix the four non-blocking findings above in the same pass.
   - **Remediated:** Fixed Tier label contradiction in `ultrafixer-packet-template.md` (updated to human-gated CAS integration), added conventional-commit and zero AI attribution / `Co-Authored-By` instruction to `ultrafixer-packet-template.md`, expanded `TestUltrafixerPacketTemplateContract` in `internal/packet/packet_test.go` to assert all protocol MUSTs, added CLI-level `--disposition` validation in `cmd/lucind-ai/cli.go` with coverage in `cmd/lucind-ai/cli_test.go`, and bumped plugin manifest versions to 2.0.6 in `.claude-plugin/marketplace.json`, `plugin/claude-code/.claude-plugin/plugin.json`, and `internal/packet/testdata/skill_content_hash.txt`.

Remediation completed via packet `ultrafixer-remediate`. A fresh dual-judge verify dispatch will follow against `feature/ultrafixer`.

