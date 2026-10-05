# Install the lucind dispatch block into CLAUDE.md

## Objective

`lucind-ai install` writes the `lucind:dispatch` block into the global `~/.claude/CLAUDE.md`, so a
new machine is ready after cloning the repository and running `make install` (ROADMAP item 6).

## Problem and why

The block that maps delegation to lucind-ai lanes lives only in the hand-maintained
`~/.claude/CLAUDE.md` of one machine. The owner moves to another machine tomorrow and wants
clone + install to bring the whole ecosystem up (herdr and rtk are already there).

## Decisions

- D1. The repository is the source of truth: `plugin/claude-code/claudemd/lucind-dispatch.md`,
  copied verbatim from the current global file, including its `<!-- lucind:dispatch -->` and
  `<!-- /lucind:dispatch -->` markers. It is embedded in the binary.
- D2. Idempotent: replace the marked span when present, append when absent, create the file when
  missing, no write when already identical. Malformed markers fail without writing.
- D3. Never replace a symlinked `CLAUDE.md`; write through to its target. Keep one backup of the
  previous content when it changes.
- D4. `--no-claude-md` skips the step.

## Delivery strategy

Local `feature-branch-chain`, branch `feature/install-claude-md` from
`feature/lane-stop-main-conversation`. Forecast about 300-400 authored changed lines.

## Tasks

- [ ] **T1. `lucind-ai install` writes the dispatch block.** Route: lane (new package, CLI wiring,
  tests, docs). Dispatched with `--auto-skills`.

## Acceptance criteria

- Running install twice leaves `CLAUDE.md` byte-identical after the first run.
- Content outside the markers (gentle-ai blocks, `@RTK.md`) is never changed.

## Checks

`golangci-lint run`, `CGO_ENABLED=0 go build ./...`, `go test ./... -race -count=1`.

## Progress

- RDD: off (clone-local).
- Next: dispatch T1.
