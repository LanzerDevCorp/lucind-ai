# auto-skills-fail-closed

Branch: `feature/auto-skills-fail-closed`
Engram mirror: `odd/auto-skills-fail-closed/tasks`
Delivery strategy: `ask-on-risk` (forecast ~400 authored lines; one lane, then split commits by task).

## Objective

When `--auto-skills` is requested and the skills cannot be selected, dispatch fails closed instead
of launching a lane without skills, and gives the orchestrator a clean fallback: write the
`## Skills to load before work` section by hand and dispatch again.

## Problem

Today any selector error (missing key, invalid key, Jev down, no network, no registry) only prints
one stderr line and launches the lane without a skills section (`handleAutoSkills`,
`internal/dispatch/skills.go`). With the `auto` skill variant the orchestrator no longer writes the
section (decision D3 of `global-api-key-and-skill-variants`), so the manual safety net is gone and
the lane silently runs skill-less. That is the failure ROADMAP item 4 set out to remove. Observed
on lane `20261005-191422-80ef`: the stored key was invalid (HTTP 401) and the lane started anyway.

## Decisions

- D1 (owner): fail closed when `--auto-skills` cannot select skills.
- D2 (owner): with a condition, the orchestrator must have a fallback: it sends the prompt with a
  hand-written skills section. A hand-written section already skips Jev, so that dispatch works
  without a key and with or without the `--auto-skills` flag.
- D3: the selection runs BEFORE anything is mutated. Today the lane is created (or, for a
  continuation, saved with its turn incremented) before `handleAutoSkills` runs, so failing there
  would leave an orphan `running` lane or a corrupted continuation. No lane state, no pane, no
  `prompt.md` and no `skills-<turn>.json` may exist after a failed selection.
- D4: new exit code 5, "auto-skills unavailable, no lane was created", next to 0 done, 1 error,
  3 failed, 4 timeout. The stderr text names the reason (redacted, never the key) and the fallback.
- D5: not a failure, dispatch continues as today: the prompt already has the section (record
  `brief_has_section`), or Jev ran fine and selected nothing (`no_skill_selected`).
- D6: every selector error counts as unavailable: missing key, HTTP errors, network errors, missing
  or unreadable registry. `lucind-ai skills select` keeps its own behavior.
- D7: the `auto` variant of the skill gains a fallback paragraph (on exit 5, resolve the skills by
  hand and dispatch again, and tell the user when it is a key problem). The how-to for resolving
  skills by hand becomes common text, shown as the primary path in `manual` and as the fallback in
  `auto`.

## Scope

In: `internal/dispatch` (selection before mutation, typed error), `cmd/lucind-ai` dispatch exit
code and messages, the skill source, docs, tests.
Out: retries or timeouts for Jev, changing `skills select`, validating the key at install time.

## Tasks

- [ ] T1 dispatch: run the selection before creating or mutating a lane; typed error when it fails;
      record the skills file only after the lane exists; nothing left behind on failure.
- [ ] T2 CLI: map the typed error to exit code 5 with the stderr guidance; update usage and docs of
      the exit codes.
- [ ] T3 skill: fallback paragraph in the `auto` variant and the by-hand procedure as common text.
- [ ] T4 docs: README, product, skill-selection, ROADMAP (Done entry).

## Acceptance criteria

- `dispatch --auto-skills` with a failing selector exits 5 and leaves no lane directory, no pane
  split, no `prompt.md`; for a continuation `lane.json` is byte-identical to before.
- The same dispatch with a hand-written section in the prompt does not call the selector and
  launches the lane, with or without `--auto-skills`, even with no key configured.
- A selector that returns no skills still dispatches, with `skills-<turn>.json` recording
  `no_skill_selected`.
- Missing key and a 401 both exit 5; the key never appears in any output.
- The installed `auto` skill says what to do on exit 5.

## Checks

- `golangci-lint run`
- `CGO_ENABLED=0 go build ./...`
- `go test ./... -race -count=1`
- `test -z "$(gofmt -l cmd internal)"`

## Progress

Branch created. No task started.

## Next step

Dispatch one lane (single writer, one tree), review the diff against this document, then `accept`.
