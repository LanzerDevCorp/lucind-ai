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
- [x] T2 (commit `d6ad882`, lane `20261004-055320-1006`, agy) `accept`: require a valid attestation per lane check on the final tree, run only the
      missing ones, accept with none; receipt evidence records per-check attestation or log.
      Stop using `check.Check` and the hardcoded string.
- [x] T3 (lane `20261004-060931-1de1`, agy, `retries: 0`) Deprecate `lucind-checks.sh`: update agy rule and `lucind-result` skill, Claude skill,
      docs (`docs/attestation.md`, `docs/product.md`, `README.md`, `CONTEXT.md`), `cli.go:153`
      log label; mark the `check` subcommand deprecated in help.
- [x] T4 (commit `113ae0d`, lane `20261004-063252-cfc0`, agy + a follow-up turn) `lucind-ai install`: embed the Claude skill, copy it to `~/.claude/skills/lucind`, then
      run the existing agy plugin setup; Makefile `install` calls it. Also embed and install the
      `lucind-roles` agy plugin (`plugin.json`, `agents/worker.md` with the `tools` frontmatter
      from T8), so a fresh machine gets a working worker role; its source is not in the repo today.

- [x] T5 Bug: `dispatch` passes a relative `--cwd` (for example `.`) unresolved to
      `herdr pane split --cwd`, so the pane opens in the herdr server's home and the agy
      PreToolUse hook denies every tool call (`git rev-parse --show-toplevel` fails, lane hangs).
      Fix 1: resolve `--cwd` to an absolute path at the start of `Dispatch` and report it in the
      output JSON. Fix 2: after creating the pane, compare its real cwd (`herdr pane get`) with the
      expected one; on mismatch close the pane and fail with both paths. Found when lane
      `20261004-052728-9ea1` died this way. Idea parked, not in scope: pass the repo root to the
      hook in an env var (`LUCIND_REPO`) so it stops depending on the pane cwd.

      Route: delegated to agy via lane `20261004-054348-0962`. Accepted, commit `09aab78`.
- [x] T6 (commit `c9f3aba`, inline) Pane lifecycle guidance in `plugin/claude-code/skills/lucind/SKILL.md` (skill only, no
      Go): after `accept`, the orchestrator decides per lane. Default: close the pane with
      `herdr pane close <pane_id>`. Alternative: leave it open to ask the implementer follow-up
      questions about the diff with its fresh context (`herdr agent prompt <pane_id> ...`), then
      close it. Also use `--cwd "$PWD"` in the dispatch example. No `accept --close-pane` flag
      for now (it would change the lucind-ai contract). Found when pane `w1:p13` stayed open.

- [x] T7 (commit `d35c7a6`, lane `20261004-064637-d5cb`) Polish the dispatch footer: it repeats "As the final verification run exactly..." once
      per check; print one intro line and a list of commands.
- [x] T8 Fix the `worker` role in the `lucind-roles` agy plugin. Root cause (per
      `docs/provider-docs/gemini/subagents.md`): custom agent frontmatter `tools` defaults to `[]`,
      and `worker.md` declared none, so the role had no tools. Fix: declared `tools`
      (`view_file`, `write_to_file`, `replace_file_content`, `multi_replace_file_content`,
      `list_dir`, `find_by_name`, `grep_search`, `run_command`), plus `subagent: true` and
      `mainAgent: false`, in the installed copy
      `~/.gemini/config/plugins/lucind-roles/agents/worker.md` (outside this repo, so done inline,
      not as a lane; backup kept in the session scratchpad). Probe with a free agy: the worker
      created a file with `write_to_file` and ran `go test ./internal/lane/...` and `go version`
      with `run_command`, all succeeded. The docs warn that a misspelled tool name can hang the
      subagent, so names were copied from the documented list. `commandExecutionPolicy` left at
      its default (`sandbox`); its semantics are undocumented and the probe did not need a change.
      Remaining: the role source is not in this repo, so a re-registration could overwrite the
      fix; T4 must ship `lucind-roles` as an embedded asset and install it.
- [x] T9 Find out why lane `20261004-054348-0962` (T5) needed `retries: 2` on the result envelope.
      Finding: agy's orchestrator goes idle while its worker subagent is still running, so the
      Stop hook sees no valid `result.json` and spends a retry (`internal/agyhook/agyhook.go:226`,
      `MaxRetries = 2`). Observed retries across lanes: 2, 1, 2, 1, 0, 2. A third early stop would
      mark the lane failed while agy is still working, so it is a real fragility, and the hook logs
      nothing about each retry (only the final `lane marked done (retries=N)`). Follow-ups T13/T14.
- [x] T12 (commit `2f99eb5`, inline, `gofmt -l .` now empty) Format with `gofmt`: lanes left unformatted files (`gofmt -l` lists several; `status.go`
      was already unformatted on `dev`). One `style:` commit, then use `test -z "$(gofmt -l ...)"`
      as a `--check` in later lanes.
- [x] T13 (commit `d35c7a6`) Bug: on a continuation (`dispatch --lane <id>`) the previous `result.json` stays on disk,
      so the Stop hook accepts the stale envelope and marks the lane `done` while agy is still
      working (seen in T4's follow-up turn). Move the old file aside (for example
      `result.prev.json`) when continuing so a fresh envelope is required.
- [x] T14 (commit `d35c7a6`; logging only) Make the Stop hook log every retry with its reason in
      `hook.log`; decide separately whether stops while a subagent is still running should consume
      retries (see T15).
- [x] T15 DONE (branch `feature/lane-stop-retries`; T15c commit `03900dc`, lane
      `20261004-075930-8771`; follow-up fix: a continuation also resets `continues` and
      `last_stop_at`, so the total cap of 10 means re-entries within one turn). T15a done (commit `7ff6a75`, lane
      `20261004-074526-c8f9`; `wait` revalidates `result.json` with a 10 min grace, hook logs every
      Stop payload). T15b done with two probe lanes (not accepted, panes closed). Data:
      probe 1, agy told to stop without writing: 3 Stops, all `fullyIdle=true`, `executionNum`
      0,1,2, gaps of 3 s and 7 s, lane `failed` after the 2 retries. Probe 2, orchestrator ends
      its turn while the worker runs `sleep 90`: Stops during the 90 s had `fullyIdle=false` and
      spent no retry (so the existing `fullyIdle` check already covers a running subagent); the
      first Stop after the worker finished had `fullyIdle=true` with no `result.json` yet (race:
      the orchestrator had not processed the worker's report) and spent 1 retry, then agy wrote
      the envelope and the lane ended `done (retries=1)`. Lane `c8f9` earlier burned its retries
      in 1m48s and 3 s. Conclusion: `fullyIdle=false` is reliable; the real problem is
      `fullyIdle=true` stops without a result between agent actions. T15c design (progress-aware
      budget): the retry budget resets when at least `RetryQuietWindow` (60 s) passed since the
      last counted retry, so only consecutive quick stops exhaust it (probe 1 still fails in
      seconds), plus a hard cap on total continues per lane (10) so a stuck agent cannot loop
      forever. `lane.json` gets `last_stop_at` and `continues`.
      Original plan (kept for reference), three steps:
      T15a lane: `wait` revalidates `result.json` before reporting `failed` (option c, grace
      period after retry exhaustion) and the Stop hook logs the full payload of every Stop
      (`executionNum`, `terminationReason`, `fullyIdle`, retry counter), all with unit tests that
      simulate Stop payloads. T15b: two real probe lanes to observe agy's real payloads (forced
      early stop without `result.json`; orchestrator ends its turn while a subagent runs
      `sleep 90`). T15c: option (a), do not spend retries while a subagent is running, using
      the signal the probes show. Note: the hook already skips retries when `fullyIdle` is false,
      yet retries were spent, so either agy reports `fullyIdle: true` during subagent work or
      those stops were legitimate; the payload logging decides it.
      Original description: Decide and fix retry exhaustion. Lane `20261004-064637-d5cb` ended with `lane.json`
      status `failed` (`retries: 2`) although agy kept working and later wrote a valid `done`
      envelope; `accept` only reads the envelope, so it was accepted, but `wait` and `dispatch`
      reported failure. Same root cause as T9. Wait for data first: the binary built from
      `d35c7a6` is the first whose Stop hook logs each retry reason in `hook.log`, so read those
      logs from the next lanes before choosing between (a) not spending retries while a subagent
      is running, (b) a higher `MaxRetries`, (c) letting `wait` re-read `result.json` before
      reporting `failed`.
- [x] T10 DONE (inline design + native writer, file outside the repo): appended a user-owned block
      `<!-- lucind:dispatch -->` (52 lines) after `@RTK.md` at the end of `~/.claude/CLAUDE.md`
      instead of editing the gentle-ai managed blocks, because `gentle-ai sync` rewrites those and
      never touches content outside its markers (`InjectMarkdownSection`, verified in
      `gentle-ai/internal/components/filemerge/section.go`). The block says the delegation
      triggers still decide WHEN and lucind-ai decides WHO: free agy for read-only work, lanes for
      code, a native writer only outside the repo, verification through lane `--check`
      attestations (no native verifier agent by default). Native exceptions kept: review actors
      (`review-*`), judgment-day (`jd-*`, only on request), `Explore`, and `sdd-*` only when the
      user invokes `gentle-sdd-*`. No silent fallback when `HERDR_ENV` or agy is unavailable.
      Verified: the old 71395 bytes are an exact prefix (same sha256), 2 new markers, the 6
      gentle-ai markers unchanged. Takes effect in new sessions. To undo, delete the lines between
      the two `lucind:dispatch` markers.
      Original task: Rewrite the global `~/.claude/CLAUDE.md` orchestration rules so code-changing work is
      dispatched through lucind-ai, and native Claude subagents become the exception (user request).
- [x] T11 Verified: agy does persist Key Learnings. Engram holds observations that the orchestrator
      did not write and that match the lanes (for example "Herdr pane cwd fail-fast validation",
      "Embedding asset trees across Go packages with go:embed", "Declarative lane checks
      verification"). Caveat: they can encode mistakes (one note about symlink handling was vague
      around the overreach fixed in T4), so they are not reviewed truth.
      Original task: Verify in a real lane that agy actually persists Key Learnings with Engram `mem_save`
      (no hook captures them; only the instruction in the agy plugin asks for it).

## Follow-up details

- RESOLVED by T7 (`d35c7a6`): the dispatch footer used to repeat "As the final verification run
  exactly..." once per check. It now prints one intro line followed by one list item per check.
  Verified in the `brief.md` of a real lane with three checks.
- RESOLVED by T8 and T4: agy reported that the `worker` role of the `lucind-roles` plugin had no
  write or command tools. Root cause was the missing frontmatter `tools`; fixed, and the plugin
  source is now vendored in `internal/agyplugin/roles/` and installed by `lucind-ai install`.
  Engram: `odd/lane-checks-and-install/pending-lucind-roles-worker`.
- RESOLVED by T9, T14 and T15: lane `20261004-054348-0962` (T5) needed `retries: 2`. Cause: the
  Stop hook spent retries on `fullyIdle=true` stops without a result while agy kept working. Fixed
  with a retry budget that resets after a quiet window plus a total cap, and `wait` revalidation.

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

- T2: first real use of `dispatch --check` (two checks). Footer rendered correctly. Accepted with
  the old binary, then re-accepted with the new one (`d6ad882`): both receipt evidence entries
  used the agy attestation (`attestation` set, no `check_log`), so nothing was re-run.

- T3: commit `af2d2e7`. Parent spot check `go test ./...` 328 passed, `go vet ./...` clean; receipt
  evidence used both agy attestations. `rg lucind-checks README.md docs plugin internal/agyplugin`
  leaves only the deprecated `check` subcommand mentions. Agy also removed stale
  `Verifier.Verify`/`integrate.Check`/`ChecksHash` text from `docs/attestation.md`; verified those
  symbols no longer exist in the Go code.

- T4: first turn reported `done` with only the `lucind-roles` part; the orchestrator rejected it
  as incomplete and sent a follow-up turn to the same lane. Review also found agy's
  `claudeplugin` removed every symlink component from `$HOME` down to the skill directory (it
  would delete a dotfiles-managed `~/.claude`) and had a test requiring it; fixed inline with RED
  then GREEN (only the destination symlink is replaced; parents are written through). Parent
  checks: `go test ./...` 357 passed, `go vet ./...` clean; stale attestations made `accept` run
  both checks itself. First real `make install`: all three components installed, skill copy equals
  the repo source, installed `worker` equals the embedded one.

- T7/T13/T14: first lane with three checks including `gofmt`; all three attestations reused by
  `accept` (no re-run). Parent spot check: `go test ./...` clean, `go vet ./...` clean,
  `gofmt -l .` empty. `lane.json` showed `failed` with `retries: 2` before accept (see T15).

- T15 verification with the installed binary `ba8e0e1` (two real probe lanes, not accepted):
  stuck agent (replies OK and stops, gaps of 5 s and 3 s) still fails fast: 3 Stops, retries 1/2
  and 2/2, then `lane marked failed (retries, retries=2)`. Slow agent (three turns, each with
  `sleep 70`, Stops 1m21s and 1m23s apart) survives: `retry budget reset after 1m21s` and
  `after 1m23s`, each Stop spent retry 1/2, and the lane ended `done (retries=1)` with
  `continues: 3`. With the previous hook the third Stop would have failed the lane. Panes were
  laid out per the new skill section (Claude left, lanes stacked right, resized to 84/36).

## Next step

T10 (global CLAUDE.md rewrite) is excluded by the user's goal and stays pending. T15 waits for
retry data from `hook.log` in upcoming lanes.
