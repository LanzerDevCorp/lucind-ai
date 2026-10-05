# turn-bound-results

Branch: `feature/turn-bound-results` (from `dev`). Mirror: Engram topic `odd/turn-bound-results/tasks`.
Delivery: work-unit commits on this branch; the merge to `dev` is decided at the end. Roadmap items
4, 8 and 9. RDD is off for this clone, so there is no native review; ordinary checks apply.

## Objective

A lane is `done` only when the **current turn** delivered its own result file. A result written by
an older or earlier turn can never close, pass or be accepted as the current one.

## Problem and why

- Done means "any schema-valid `result.json` with `status: done` exists". Nothing ties it to the turn
  (`internal/agyhook/agyhook.go:240-292`, `internal/lane/lane.go:303-325`).
- `wait` returns done without revalidating (`internal/dispatch/wait.go:71-73`) and recovers a
  `failed` lane to `done` from any done envelope (`wait.go:74-80`).
- A continuation skips the idle wait (`internal/dispatch/dispatch.go:187-230`) and `sendPrompt`
  accepts `--until working`, so a still-running old turn counts as the new one.
- Seen twice while fixing lint (lane `20261004-224812-cff8`): the Stop hook marked the lane done about
  a minute after a continuation while agy kept working and rewrote `result.json` 90 s later.
- Hook payload `executionNum` is not a turn counter (it alternates 0/1 inside one lane), so it is
  not usable as a discriminator.
- `TestLaneIDFormat` flakes (16-bit suffix, 50 IDs in one second, about 1.9% collision).

## Decisions (user-confirmed)

- Option C: one result file per turn, `.lucind/lanes/<id>/result-<N>.json`. `lane.json` gains
  `turn`, which starts at 1 and increases on every `dispatch --lane` continuation. Only the file of
  the current turn is read by the Stop hook, `MarkStopped`, `wait` and `accept`.
- Log the raw Stop payload in `hook.log` so real lanes can show whether agy sends a turn or
  conversation identifier (a later fix for stale Stops depends on it).
- Known residual: a Stop from the old turn cannot be told apart from a new one; it can spend a retry
  and inject the "write the envelope" nudge. Documented, not fixed here.

## Decisions (mine, to flag)

- Lanes already on disk have no `turn`: `turn == 0` is a legacy lane and keeps using `result.json`.
- `result.prev.json` is no longer produced; previous turns stay as `result-<N>.json`.
- The lane ID format (`YYYYMMDD-HHMMSS-<4 hex>`) is not changed: `ValidateID` runs in the hooks.
  Only the test is fixed.
- During migration `lane.ResultPath(root, id)` stays as a legacy helper so every task compiles; it
  is removed in T4. It is not marked `Deprecated:` because staticcheck SA1019 would fail the lint
  at every caller.
- T2 added a transitional fallback in `MarkStopped`: at turn 1, if `result-1.json` is missing it
  reads `result.json`, because callers still write the legacy file until T3/T4. **T4 must remove
  this fallback**; keeping it would let a stale `result.json` close a turn-1 lane again.
- Every lane brief lists the Go skills from `.atl/skill-registry.md` under
  `## Skills to load before work` (missed for the lint lane and T2; reviewed T2 against them).

## Scope

In: `internal/lane`, `internal/dispatch`, `internal/agyhook`, `internal/accept`, the agy plugin
assets that name the result file, the Claude skill, docs.
Out: the result schema (`internal/result`), the ID format, a turn identifier from the hook payload,
the herdr idle wait on continuation.

## Constraints

- Test-first: Go tests exist, so observe RED, then GREEN, then refactor. Runner: `go test ./... -race -count=1`.
- Artifacts in English. About 400 authored changed lines per task is a planning heuristic only.
- Code-changing tasks go through an agy lane (`lucind-ai dispatch`), one lane per task, one writer.
  `--check` set: `golangci-lint run`, `CGO_ENABLED=0 go build ./...`, `go test ./... -race -count=1`.
- After any change touching the binary: `make install` (project `CLAUDE.md`).
- Forecast of authored changed lines (additions plus deletions): about 600 including tests.

## Tasks

- [x] T1 (commit `4b6aff9`) Item 9: `TestLaneIDFormat` uses distinct seconds through `GenerateIDAt` for its uniqueness
      loop. File: `internal/lane/lane_test.go`. Route: inline (one mechanical, understood test file).
      RED: 8 duplicates in 400 runs; GREEN: 2000 of 2000 pass.
- [x] T2 (commit `28e4295`, lane `20261005-004502-3c2c`, agy) Lane model: `Turn` field in `lane.json`; `Create` sets 1; `ResultFile` helper
      (`result-<turn>.json`, `result.json` when `turn == 0`); `MarkStopped` reads the current turn file.
      Files: `internal/lane/lane.go`, `internal/lane/lane_test.go`. Route: delegated lane.
- [x] T3 (commit `c4f615c`, lane `20261005-005347-eeec`, agy; agy made the commit itself, kept after
      review: author and message are correct. Later briefs forbid git write commands) Dispatch and wait: a continuation increments and persists `turn`, no rename to
      `result.prev.json`; the brief footer and `Output.ResultPath` name the turn file; `wait` reads the
      turn file and revalidates it before reporting done, including the failed to done recovery.
      Files: `internal/dispatch/dispatch.go`, `wait.go`, their tests, `cmd/lucind-ai/cli_test.go` if
      fixtures need it. Route: delegated lane.
- [x] T4 (commit `497987c`, lane `20261005-010459-1a98`, agy; lint clean, 178 tests in 3 packages pass, fallback and `ResultPath` removed) Hook and accept: the Stop hook and `accept` read the current turn file; log the raw Stop
      payload (truncated) in `hook.log`; remove deprecated `lane.ResultPath`.
      Files: `internal/agyhook/agyhook.go`, `internal/accept/accept.go`, their tests,
      `internal/lane/lane.go`. Route: delegated lane.
- [x] T5a (inline, commit `1e1baf7`) The agy-facing rule `lucind-lane.md` and skill `lucind-result` told agy to write
      `result.json`, contradicting the footer and the new hook: fixed first and reinstalled.
- [x] T5b (commit `41e1202`, lane `20261005-011507-22a2`, agy) Docs and skills (lane): `CONTEXT.md`, `docs/attestation.md`, `docs/product.md`, `README.md`,
      `plugin/claude-code/skills/lucind/SKILL.md`, `internal/agyplugin/assets/rules/lucind-lane.md`,
      `internal/agyplugin/assets/skills/lucind-result/SKILL.md`, `docs/ROADMAP.md`; then `make install`.
      Route: delegated lane (passive docs, no RED).

## Acceptance criteria

- At turn 2, a valid `result-1.json` is never treated as the result: the Stop hook counts it as
  missing, `wait` does not report done, `accept` refuses.
- A legacy lane (no `turn`) still completes through `result.json`.
- A continuation persists `turn + 1`; the brief footer and `Output.ResultPath` name `result-<N>.json`.
- `wait` revalidates the current turn file before reporting done.
- `hook.log` contains the raw Stop payload.
- `TestLaneIDFormat` passes repeatedly (`-count=200`).
- `golangci-lint run`, the build and `go test ./... -race -count=1` pass.

## Progress and evidence

Created after exploration (explorer report plus verified reads of `dispatch.go`, `wait.go`,
`agyhook.go`). T1 and T2 done and verified (lint clean, lane package 89 tests pass with `-race`,
lane accepted with attested checks).

T3 verified: 77 dispatch tests pass with `-race`, lint clean, lane accepted. T3 also exposed that
the PreToolUse hook only allows writing `result.json` under `.lucind/` (`agyhook.go:190`), so T4
restricts it to the current turn file.

T4 verified (see above); `make install` run after T4 (binary `497987c`) and again after T5a (`1e1baf7`).

T5b verified: lint clean, lane accepted, docs diff reviewed. **Real-lane evidence** (lane
`20261005-011507-22a2`, first lane run with the new binary): it closed through `result-1.json`, and
`hook.log` holds 7 `stop: raw payload` lines. The payload keys are `artifactDirectoryPath`,
`conversationId`, `error`, `executionNum`, `fullyIdle`, `modelName`, `terminationReason`,
`transcriptPath`, `workspacePaths`: there is a conversation id but no turn id. The lane showed three
conversations, two with `fullyIdle=true`, and both retries were spent before the result existed.
Recorded in roadmap item 4.

Not covered by real-lane evidence: a continuation (turn 2). It is covered by unit tests only;
exercise it in the real-lane stability trials (roadmap item 1).

## Next step

Decide the merge of `feature/turn-bound-results` into `dev` (user).
