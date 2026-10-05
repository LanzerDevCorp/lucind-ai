# direct-prompt-dispatch

Branch: `feature/direct-prompt-dispatch`
Engram mirror: `odd/direct-prompt-dispatch/tasks`
Delivery strategy: `ask-on-risk` (forecast ~600 authored lines across 16 files; to be sliced by task).

## Objective

Dispatch sends the task content itself to agy as the prompt. The pointer prompt
`Read and follow <abs>/brief.md` is removed, and `--brief` is removed with it.

## Problem

ROADMAP "Next" item 4: agy does not follow the brief literally. With a pointer prompt it
never opened the `## Skills to load before work` files and did not pass them to its worker
subagents (lane `20261005-050638-3cba`: only `golang-cli` was read, yet the envelope's
`skills_loaded` listed every skill).

## Decisions

- D1 (owner): send the content directly as the prompt, with the skills section first.
- D2 (owner): remove `--brief` at once, no deprecated alias. `--prompt` replaces it in
  `dispatch` and in `skills select`.
- D3: the hook classified the main conversation by matching `Read and follow` + `brief.md`
  in transcript step 0. That marker disappears, so the prompt starts with a lucind-owned
  marker line the hook matches instead. This only decouples the hook from agy's wording; the
  ROADMAP item 3 owner review (find a supported signal) stays open.
- D4: the final prompt is still persisted in the lane directory as a record
  (`prompt.md`), never referenced from the prompt sent to agy.

## Scope

In: `internal/dispatch`, `internal/agyhook`, `cmd/lucind-ai` (dispatch + skills select),
agy plugin assets (`lucind-lane.md`, `lucind-result/SKILL.md`), Claude skill, docs, tests.
Out: ROADMAP item 3 replacement, skill-loading enforcement hook (plan step 2 of item 4),
transcript-based measurement (plan step 3).

## Tasks

- [x] T1 dispatch: send the full prompt (skills section first, marker line first of all),
      persist `prompt.md`, drop `Read and follow`. Route: delegated (lane).
- [x] T2 CLI: rename `--brief` to `--prompt` in `dispatch` and `skills select`; `--brief`
      must fail with a clear error; update usage strings. Route: delegated (lane).
- [x] T3 hook: classify the main conversation by the marker line; keep worker detection.
      Route: delegated (lane).
- [x] T4 docs and assets: README, product, skill-selection, ROADMAP, Claude skill,
      agy rule and skill, dispatch block in the CLAUDE.md asset. Route: delegated (lane).

## Acceptance criteria

- `rg 'Read and follow|brief\.md|--brief'` finds nothing outside history (`openspec/`,
  `odd/`, `.lucind/`).
- A dispatched lane's first transcript step contains the whole prompt and begins with the marker.
- The Stop hook still classifies main vs worker conversations (existing tests adapted).
- `--brief` exits non-zero with a message pointing to `--prompt`.

## Checks

- `golangci-lint run`
- `CGO_ENABLED=0 go build ./...`
- `go test ./... -race -count=1` (`TestLaneIDFormat` is a known flake, repeat before blaming)

## Progress

T1-T4 done in lane `20261005-171421-9331` (single writer, one tree). Route: delegated (lane).
Evidence: `golangci-lint run` clean, `CGO_ENABLED=0 go build ./...` OK, `go test ./... -race
-count=1` 576 passed, final `rg` leaves only the `--brief` rejection code and its tests.
The lane hit an agy 429 right after dispatch and the Stop hook marked it `failed` (retries spent
in 6 s); agy kept working, delivered a valid envelope, and `accept` succeeded.
Commits: `3925c93` (code and tests: T1-T3), `5549c6b` (docs and assets: T4).
RDD: off for this clone (clone-local), so no native review.

## Next step

Run `make install`, dispatch the first real lane with the new flow, and measure skill loading
in the main and worker transcripts (ROADMAP item 4 plan steps 2 and 3).
