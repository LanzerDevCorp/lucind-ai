# lane-checks-and-install

Branch: `feature/lane-checks-and-install` (from `dev`). Delivery strategy: `ask-on-risk`.

## Objective

1. Let the orchestrator choose, per lane, which verification commands must be attested (zero or N),
   and deprecate the hardcoded `sh lucind-checks.sh`.
2. Add one fixed, flagless `lucind-ai install` that installs everything lucind needs (agy plugin
   and the Claude skill) so the binary is portable to any machine.

## Problem and why

- `accept` only trusts an attestation of the exact command `sh lucind-checks.sh`
  (`internal/accept/accept.go:89`), so a project with several test types and e2e cannot scope
  verification to the change.
- Only the agy side is installed by the CLI; the Claude skill is a Makefile symlink
  (`Makefile:10-14`) and does not travel with the binary.

## Decisions (user-confirmed)

- `--check '<cmd>'` on `dispatch` is repeatable and optional. Zero checks means `accept` requires
  no attestation and validates only the result envelope and allowed globs.
- `lucind-checks.sh` is deprecated: no default, no fallback runner in the lane flow.
  Running the whole suite is the orchestrator's call, passed as a `--check`.
- `lane.json` `checks` is the single source of truth for the lane's checks. Today the orchestrator
  fills it with flags; a future Jev-driven selector only has to write that field. `attest` and
  `accept` read it and nothing else. No selector interface now.
- `lucind-ai install` takes no configuration flags (one internal flag for tests is allowed). It
  always installs the same assets. The Claude skill is embedded with `go:embed` and copied, not
  symlinked.

## Scope

In: `internal/lane`, `internal/dispatch`, `internal/accept`, `internal/check` (deprecate),
`cmd/lucind-ai` (dispatch flag, install command, help), agy plugin assets, Claude skill, docs,
Makefile.
Out: `internal/attest` internals (matches by exact command string already), result schema,
Jev integration itself.

## Constraints

- Test-first: Go tests exist, so observe RED, then GREEN, then refactor. `go test ./...` is the runner.
- Artifacts in English. About 400 authored changed lines per task is a planning heuristic only.
- `attest` records `strings.Join(argv, " ")`, so the command string given to `--check` must be the
  exact string agy runs; the footer must print it verbatim.
- After any change touching the binary: `make install` (project CLAUDE.md).

## Tasks

Route per task is recorded when started (inline or delegated, with trigger evidence).

- [x] T1 `lane.json` `checks` field and `dispatch --check` (repeatable, optional); footer lists
      `lucind-ai attest run -- <cmd>` per check and omits the attest step when there are none.
      Route: delegated to agy via lucind lane `20261004-053033-5b71` (writes 6 non-trivial files,
      mapped by an explorer first). Accepted; `lane.CheckCommand` is the shared helper for T2.
- [ ] T2 `accept`: require a valid attestation per lane check on the final tree, run only the
      missing ones, accept with none; receipt evidence records per-check attestation or log.
      Stop using `check.Check` and the hardcoded string.
- [ ] T3 Deprecate `lucind-checks.sh`: update agy rule and `lucind-result` skill, Claude skill,
      docs (`docs/attestation.md`, `docs/product.md`, `README.md`, `CONTEXT.md`), `cli.go:153`
      log label; mark the `check` subcommand deprecated in help.
- [ ] T4 `lucind-ai install`: embed the Claude skill, copy it to `~/.claude/skills/lucind`, then
      run the existing agy plugin setup; Makefile `install` calls it.

- [x] T5 Bug: `dispatch` passes a relative `--cwd` (for example `.`) unresolved to
      `herdr pane split --cwd`, so the pane opens in the herdr server's home and the agy
      PreToolUse hook denies every tool call (`git rev-parse --show-toplevel` fails, lane hangs).
      Fix 1: resolve `--cwd` to an absolute path at the start of `Dispatch` and report it in the
      output JSON. Fix 2: after creating the pane, compare its real cwd (`herdr pane get`) with the
      expected one; on mismatch close the pane and fail with both paths. Found when lane
      `20261004-052728-9ea1` died this way. Idea parked, not in scope: pass the repo root to the
      hook in an env var (`LUCIND_REPO`) so it stops depending on the pane cwd.

      Route: delegated to agy via lane `20261004-054348-0962`. Accepted, commit `09aab78`.
- [ ] T6 Pane lifecycle guidance in `plugin/claude-code/skills/lucind/SKILL.md` (skill only, no
      Go): after `accept`, the orchestrator decides per lane. Default: close the pane with
      `herdr pane close <pane_id>`. Alternative: leave it open to ask the implementer follow-up
      questions about the diff with its fresh context (`herdr agent prompt <pane_id> ...`), then
      close it. Also use `--cwd "$PWD"` in the dispatch example. No `accept --close-pane` flag
      for now (it would change the lucind-ai contract). Found when pane `w1:p13` stayed open.

## Acceptance criteria

- `dispatch` with no `--check` produces a lane that `accept` can accept without any attestation.
- `dispatch` with two `--check` values: `accept` rejects when either lacks a valid attestation for
  the final tree and runs only the missing one(s).
- No non-test code path references `lucind-checks.sh` as a default.
- `lucind-ai install` on a clean HOME produces the Claude skill copy and the registered agy plugin.
- `go test ./...` passes; `lucind-ai -v` reflects the installed commit after `make install`.

## Progress

- Explorer map done (attest/accept/lane/install); findings folded into the tasks above.
- T1 done and accepted. T5 found while dispatching T1 (relative `--cwd`); queued after T1 because
  both edit `internal/dispatch/dispatch.go` (one writer per tree).

## Verification evidence

- T1: agy reported RED then GREEN per criterion; parent spot check
  `go test ./internal/lane/... ./internal/dispatch/... ./cmd/lucind-ai/...`: 162 passed;
  `lucind-ai accept` accepted lane `20261004-053033-5b71`.

## Commits

- T1: `8c3b84d` feat(dispatch). Review assessment: risk medium (`executable_change`), 643 lines,
  `review_due: true` (`slice_budget_reached`), but native review preflight returned
  `stop: rdd_disabled`, so the tier outcome is unmanaged (RDD is off for this clone; not enabled
  on the user's behalf).

## Next step

Start T1 with one delegated writer.
