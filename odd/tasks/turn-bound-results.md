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
- During migration `lane.ResultPath(root, id)` stays (marked deprecated) so every task compiles; it
  is removed in T4.

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

- [ ] T1 Item 9: `TestLaneIDFormat` uses distinct seconds through `GenerateIDAt` for its uniqueness
      loop. File: `internal/lane/lane_test.go`. Route: inline (one mechanical, understood test file).
- [ ] T2 Lane model: `Turn` field in `lane.json`; `Create` sets 1; `ResultFile` helper
      (`result-<turn>.json`, `result.json` when `turn == 0`); `MarkStopped` reads the current turn file.
      Files: `internal/lane/lane.go`, `internal/lane/lane_test.go`. Route: delegated lane.
- [ ] T3 Dispatch and wait: a continuation increments and persists `turn`, no rename to
      `result.prev.json`; the brief footer and `Output.ResultPath` name the turn file; `wait` reads the
      turn file and revalidates it before reporting done, including the failed to done recovery.
      Files: `internal/dispatch/dispatch.go`, `wait.go`, their tests, `cmd/lucind-ai/cli_test.go` if
      fixtures need it. Route: delegated lane.
- [ ] T4 Hook and accept: the Stop hook and `accept` read the current turn file; log the raw Stop
      payload (truncated) in `hook.log`; remove deprecated `lane.ResultPath`.
      Files: `internal/agyhook/agyhook.go`, `internal/accept/accept.go`, their tests,
      `internal/lane/lane.go`. Route: delegated lane.
- [ ] T5 Docs and skills: `CONTEXT.md`, `docs/attestation.md`, `docs/product.md`, `README.md`,
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
`agyhook.go`). No task started.

## Next step

T1 inline, then T2 lane.
