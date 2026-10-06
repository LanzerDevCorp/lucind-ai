# lane-lifecycle

Branch: `feature/lane-lifecycle` (from `dev`). Mirror: Engram `odd/lane-lifecycle/tasks`.

## Objective

Deepen the Lane module so it owns the lane lifecycle: every status change and the retry budget go
through `internal/lane` methods that check the transition is legal and save atomically. Callers
(`dispatch`, `wait`, `agyhook`, `accept`) stop mutating `lane.Lane` fields directly.

## Problem and why

Architecture review 2026-10-05, candidate 1. `lane.Lane` is a plain record with `Load`/`Save`; the
transition rules leak into four packages:

- `internal/dispatch/dispatch.go:197-256,303`: continuation turn setup, `result.json` →
  `result.prev.json` rename, resetting `Retries`/`Continues`/`LastStopAt`, `Status = running`.
  "A terminal lane can't be continued" lives only here.
- `internal/dispatch/wait.go:72-126`: sets `StatusTimeout` and saves in four places, calls `MarkStopped`.
- `internal/agyhook/agyhook.go:333-391`: retry budget (`Retries`, `Continues`, `LastStopAt`, quiet window).
- `internal/accept/accept.go:130-136,156`: sets `Accepted`/`Rejected`.
- `internal/lane/status.go`: deprecated aliases (`Running`, `Done`, `Failed`, `Pending`, `Blocked`,
  `Deviated`) with no users.

Deletion test: removing the scattered field writes concentrates the rules in one module.

## Scope

In: `internal/lane/**`, `internal/dispatch/**`, `internal/agyhook/**`, `internal/accept/**`, and
their tests.
Out: candidate 2 (shared `CurrentResult`/`InScope` helpers, beyond what the transitions need),
candidate 3 (git seam), CLI changes, `lane.json` on-disk schema changes, behavior changes visible to users.

## Constraints

- Pure refactor: observable behavior, exit codes, file names and `lane.json` contents stay the same.
- Test-first with `go test`: table tests for the transitions in `internal/lane` (RED then GREEN);
  existing tests in the caller packages must stay green.
- English artifacts. About 400 authored changed lines per task is a planning heuristic only.

## Acceptance criteria

- `internal/lane` exposes lifecycle methods (names may differ: begin a turn, record a Stop with the
  retry/continue budget, mark timeout, record the review verdict) that reject illegal transitions
  and persist the change.
- No package outside `internal/lane` assigns `Status`, `Retries`, `Continues`, `LastStopAt` or `Turn`
  on a `lane.Lane` (outside tests).
- Deprecated status aliases removed.
- Table-driven transition tests in `internal/lane` cover legal and illegal moves.
- `golangci-lint run`, `CGO_ENABLED=0 go build ./...`, `go test ./... -race -count=1` pass.

## Tasks

- [ ] **T1 — Lane lifecycle transitions and caller migration.** Route: delegated lane (writer
  trigger: 2+ non-trivial files across four packages). RDD: off (clone-local), no native review.

## Progress

- 2026-10-05: feature document created; T1 dispatched.

## Next step

Dispatch T1, review the diff against the acceptance criteria, accept, commit.
