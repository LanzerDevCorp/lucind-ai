# lane-result-scope

Branch: `feature/lane-result-scope` (from `dev` @ 9ceca27). Mirror: Engram `odd/lane-result-scope/tasks`.

## Objective

Give the Lane module one home for two pieces of knowledge that are re-derived at each call site:
reading the current turn's Result envelope (with the "done" check), and deciding whether a path is
inside the lane's code scope (Allowed globs minus the `.lucind` state directory).

## Problem and why

Architecture review 2026-10-05, candidate 2. After `feature/lane-lifecycle`:

- Current-turn result read plus `Status != "done"` is written at `internal/lane/lifecycle.go:105,168`
  and `:224,227`, `internal/dispatch/wait.go:73-74` and `:80-82`, and `internal/accept/accept.go:41-50`.
  `accept` builds the lane path by hand (`filepath.Join(".lucind","lanes",laneID,...)`) instead of `lane.LaneDir`.
- The `.lucind` exclusion plus `MatchAny` is written at `internal/agyhook/agyhook.go:205` (PreToolUse
  deny) and `internal/accept/accept.go:76-82` (changed-files check). Same rule, two copies: the hook
  and accept can drift apart.
- `internal/result/result.go:2` doc comment still says `.lucind/result.json`.

## Design

- `lane` gets a result accessor for the current turn returning the envelope plus an outcome
  (missing, invalid, not done, done) and the read error, so callers keep their messages.
- `lane` gets `IsStatePath(rel)` (`.lucind` or under it) and `InScope(l, rel)` (not a state path and
  matches an Allowed glob). The hook denies when `!InScope`; accept skips state paths and rejects
  the rest when `!InScope`.

## Scope

In: `internal/lane/**`, `internal/dispatch/**`, `internal/agyhook/**`, `internal/accept/**`,
`internal/result/result.go` (doc comment only), and their tests.
Out: git seam (candidate 3), CLI, schema or file name changes, user-visible behavior or messages.

## Constraints

- Pure refactor: hook decisions, accept reasons text, exit codes and file names stay the same.
- Test-first with `go test` (RED then GREEN) for the new lane functions.
- Existing tests may not be edited to fit new behavior.
- English artifacts. About 400 authored changed lines per task is a planning heuristic only.

## Acceptance criteria

- No `result.Read(` outside `internal/lane` and `internal/result` (non-test code).
- No `".lucind"` scope check outside `internal/lane` (non-test code).
- Table tests for the result accessor (turn 0 legacy `result.json`, turn N, missing, invalid, not done,
  done) and for `IsStatePath`/`InScope`.
- `golangci-lint run`, `CGO_ENABLED=0 go build ./...`, `go test ./... -race -count=1` pass.

## Tasks

- [x] **T1 — Current result accessor and scope rule in `lane`, callers migrated.** Route: delegated
  lane (writer trigger: 2+ non-trivial files across four packages). RDD: off (clone-local).
  Evidence: lane `20261006-051018-ef26` (gemini-3.8-flash-high, 1 turn) accepted with attested
  `golangci-lint run`, `CGO_ENABLED=0 go build ./...`, `go test ./... -race -count=1` on the final
  tree; both rg checks printed nothing; no existing test assertion edited. Commit `c4ac84b`.

## Progress

- 2026-10-05: feature document created; T1 dispatched.
- Parent polish before accept (inline, mechanical): doc comments on `ResultOutcome`, `CurrentResult`,
  `IsStatePath`, `InScope`; corrected the legacy path in the `result` package doc to
  `.lucind/lanes/<id>/result.json`; `gofmt -w` on `lane_test.go` and `lifecycle_test.go` (the latter
  was left unformatted by `feature/lane-lifecycle`).

## Next step

Feature complete. Merge into `dev` is the user's decision. Follow-up: candidate 3 (git seam).
