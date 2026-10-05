# Lane completion from the main agy conversation

## Objective

Mark a lane `done`/`failed` only when agy's main conversation finishes with all of its workers,
and stop nudging worker conversations. Also let `dispatch --lane` reuse the lane's stored
`--allow` globs. First real use of `dispatch --auto-skills` (Jev skill selection).

## Problem and why

agy orchestrates: the main conversation spawns worker subagents, and the Stop hook fires for
every conversation. Evidence from lane `20261005-035100-7fce` `hook.log`:

- main conversation `0c13b5df` sent `fullyIdle=false` on every Stop while workers ran;
- worker conversations `ed047531` and `ba81bfb2` sent `fullyIdle=true` when each finished;
- the hook treated all Stops alike: it injected "write the envelope" into workers
  (`retry 1/2`, `retry 2/2`, budget reset, repeated) and marked the lane `done` at 04:02:45 while
  the main conversation still had workers running.

Consequences seen in `jev-skill-selector`: T1 turn 2 got a `result-2.json` byte-identical to
`result-1.json` 12 s after the brief (a nudged worker copied it), and both T1 and T2 reported
`done` while agy was still working.

## Decisions

- D1. Use the structured Stop payload, not the pane text (owner decision): only a Stop from the
  lane's main conversation with `fullyIdle=true` may decide done/retry/failed.
- D2. Stops from worker conversations, and main-conversation Stops with `fullyIdle=false`, are
  ignored (allowed to stop, no nudge, no retry counted, no status change), and logged.
- D3. The main conversation is identified from evidence in its transcript (to be confirmed by the
  lane); `conversationId` alone is not enough after a continuation (docs/ROADMAP.md item 3).
- D4. `dispatch --lane` without `--allow` reuses the lane's stored globs.

## Delivery strategy

Local `feature-branch-chain` (owner preference), branch `feature/lane-stop-main-conversation`
from `feature/jev-skill-selector-t2`. Forecast about 400-600 authored changed lines.

## Tasks

- [ ] **T1. Stop hook decides only from the main conversation.** Route: lane (agyhook logic,
  tests, docs; 2+ non-trivial files). Dispatched with `--auto-skills`.
- [ ] **T2. `dispatch --lane` reuses stored `--allow`.** Route: same lane (small, disjoint files),
  separate commit.

## Acceptance criteria

- A worker Stop never nudges and never changes lane status.
- A main Stop with `fullyIdle=false` never marks the lane done.
- The lane becomes `done` only after a main `fullyIdle=true` Stop with a valid current-turn
  envelope; the retry nudge goes only to the main conversation.
- `dispatch --lane <id>` works without `--allow`; `--allow` given still replaces the globs.

## Checks

`golangci-lint run`, `CGO_ENABLED=0 go build ./...`, `go test ./... -race -count=1`.

## Progress

- RDD: off (clone-local).
- Jev (first real use, `--auto-skills`): `jev-1.13.0`, 12,986 input / 844 output tokens for 42
  skills. Selected (>= 0.7): golang-cli 0.86, golang-testing 0.84, go-testing 0.80,
  golang-code-style 0.78, golang-error-handling 0.70. Every orchestrator-only skill stayed below
  0.35. Probabilities are compressed (relevant rejections near 0.5); go-testing (Bubbletea) is
  noise next to golang-testing. Paths resolved to `.agents/skills` and the section landed after
  the title.
- Gap: the result envelope's `skills_loaded` was `null`, so Jev's choice cannot yet be compared
  with what agy loaded. The worker role or envelope contract must require it.
- The fixed hook is not yet proven on a real lane (this lane ran on the old binary). Next: a real
  lane on the new binary, then check `hook.log` for `ignored worker conversation`.
