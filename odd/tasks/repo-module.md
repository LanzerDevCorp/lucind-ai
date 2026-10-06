# repo-module

Branch: `feature/repo-module`, stacked on `feature/lane-result-scope` @ cf8bf4d (not yet merged into
`dev`). Mirror: Engram `odd/repo-module/tasks`.

## Objective

Concentrate every git call about the repository and its trees in one deep `internal/repo` module, so
`attest` only owns attestations, `accept` stops running git itself, and the CLI drops its duplicate
git helpers. Add a small seam in `accept` so its verdict logic can be tested without a git repo.

## Problem and why

Architecture review 2026-10-05, candidate 3.

- `internal/attest/attest.go:175,195` (`RepoToplevel`, `RepoCommonDir`) and `:281-318` (`TreeHash`,
  temp `GIT_INDEX_FILE` + `read-tree HEAD` + `add -A` + `write-tree`) put repository and tree
  knowledge inside the Attestation module. `lane`, `dispatch`, `wait`, `agyhook`, `accept` and three
  CLI files depend on `attest` only to find the repo or hash the tree.
- `internal/accept/accept.go:57` runs its own `git diff --name-only base final`.
- `cmd/lucind-ai/cli.go:176-200` duplicates `rev-parse HEAD` / `--show-toplevel`.

## Design

- Part 1: `internal/repo` with `Toplevel`, `CommonDir`, `TreeHash`, `ChangedFiles(base, final)`,
  `HeadSHA`, exec-git only, tested against real temp git repos. `attest` keeps `RepoID`, keys, entries,
  `Verify`/`FindValidAttestation`, and calls `repo` where it needs a tree hash.
- Part 2: `accept` depends on a two-method interface (tree hash, changed files) with the exec-git
  `repo` adapter in production and a fake in verdict tests. Exported `accept` entry points keep their
  signatures. Decision 2026-10-05: user asked for both parts in one lane.
- Real-git tests stay for `TreeHash` and `ChangedFiles`: a fake would hide untracked, ignored and
  no-HEAD behavior, which is exactly what must be verified.

## Scope

In: `internal/repo/**` (new), `internal/attest/**`, `internal/lane/**`, `internal/dispatch/**`,
`internal/agyhook/**`, `internal/accept/**`, `cmd/lucind-ai/**`, and their tests.
Out: accept's check runner (`sh -c`), attestation format, CLI flags or output, user-visible behavior.

## Constraints

- Pure refactor: outputs, messages, exit codes, receipts and tree hashes stay identical.
- Test-first with `go test`. Existing tests may not be edited to fit new behavior; mechanical edits
  (moved function names) are allowed and must be listed in the envelope.
- English artifacts. About 400 authored changed lines per task is a planning heuristic only.

## Acceptance criteria

- No `exec.CommandContext(ctx, "git"` (or equivalent) outside `internal/repo` (non-test code).
- `attest` exports no `RepoToplevel`, `RepoCommonDir` or `TreeHash`.
- `internal/repo` tests cover tracked, untracked, ignored files and a repo without HEAD.
- `accept` verdict tests run on a fake trees adapter.
- `golangci-lint run`, `CGO_ENABLED=0 go build ./...`, `go test ./... -race -count=1` pass; `gofmt -l` clean.

## Tasks

- [ ] **T1 — `internal/repo` module, callers migrated, accept seam.** Route: delegated lane (writer
  trigger: 2+ non-trivial files across seven packages). Model: claude-sonnet-5-5-medium. RDD: off (clone-local).

## Progress

- 2026-10-05: feature document created; T1 dispatched.

## Next step

Review the lane diff against the acceptance criteria, accept, commit.
