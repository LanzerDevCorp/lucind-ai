# dispatch-outcome

Branch: `feature/dispatch-outcome` (from `dev` @ 89822c4). Mirror: Engram `odd/dispatch-outcome/tasks`.

## Objective

Keep `cmd/lucind-ai` to parsing flags and printing: move dispatch policy (auto-skills fallback and
API-key remediation text) into `internal/dispatch`, remove shallow pass-throughs in `dispatch`, and
delete the deprecated `check` subcommand with `internal/check`.

## Problem and why

Architecture review 2026-10-05, candidate 4.

- `cmd/lucind-ai/cli.go:403-427` decides the auto-skills fallback and missing/rejected key remediation
  lines: domain policy, testable only end to end through `cli_test.go`.
- `runDispatch` repeats the `usageBuf.Reset(); fs.Usage(); Fprint` block five times (`:334-367`),
  `runWait` repeats it and its positional parsing (`:482` onward).
- Pass-throughs: `dispatch.ResolveModel` (`internal/dispatch/model.go`) only wraps
  `executor.ResolveModel`; `dispatch.Wait` takes a `HerdrRunner` it never uses (`wait.go:130`);
  `wait.go` returns literal `3`/`4` while `ExitFailed`/`ExitTimeout` live in `skills.go:19-21`;
  `Options.Brief` (`dispatch.go:26,214`) survives the removal of `--brief`.
- `check` subcommand (`cli.go:73,93-174`) is deprecated and the only user of `internal/check`.
  Deletion test: removing both concentrates nothing; it only deletes.

## Scope

In: `cmd/lucind-ai/**`, `internal/dispatch/**`, `internal/check/**` (deleted), `README.md`,
`docs/attestation.md`, `CLAUDE.md` (references to the `check` subcommand only), and tests.
Out: `lucind-checks.sh` stays (humans run it; `CLAUDE.md` keeps it in sync with the check table),
other subcommands' behavior, `openspec/**` archives.

## Constraints

- Every stdout/stderr line and exit code of `dispatch`, `wait`, `accept` stays byte-identical.
- The only user-visible change is removing `lucind-ai check` (and its usage line).
- Test-first with `go test`. Existing assertions may not change to fit new behavior; tests of deleted
  code (`check`, `dispatch.ResolveModel`) are deleted with it, after confirming equivalent coverage
  exists in `internal/executor`.
- English artifacts. About 400 authored changed lines per task is a planning heuristic only.

## Acceptance criteria

- `internal/dispatch` exposes the remediation lines for an auto-skills failure; the CLI only prints them.
- Unit tests in `internal/dispatch` cover missing key, rejected key and generic auto-skills failure lines.
- One usage-error helper in the CLI; no repeated usage blocks.
- No `dispatch.ResolveModel`, no unused `HerdrRunner` in `Wait`, no literal exit codes in `wait.go`,
  no `Options.Brief`; exit code constants in their own file.
- `internal/check` and the `check` subcommand are gone; docs updated.
- `golangci-lint run`, `CGO_ENABLED=0 go build ./...`, `go test ./... -race -count=1`, gofmt clean.

## Tasks

- [ ] **T1 — Dispatch outcome policy, CLI cleanup, delete `check`.** Route: delegated lane (writer
  trigger: 2+ non-trivial files). Model: gemini-3.8-flash-high. RDD: off (clone-local).

## Progress

- 2026-10-05: feature document created; T1 dispatched.

## Next step

Review the lane diff against the acceptance criteria, accept, commit.
