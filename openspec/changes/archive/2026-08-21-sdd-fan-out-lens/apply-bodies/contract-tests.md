# Packet sdd-fan-out-lens-contract-tests

## Goal

Add strict RED contract tests that pin packet parser errors, planning-template validity and per-phase path disjointness, and the complete `SKILL.md` planning-fan-out contract without implementing or weakening any production behavior.

## Why this is safe to dispatch now

The accepted checklist fixes the assertions and the only writable file, while the design fixes substring/table-driven tests rather than a Markdown AST. This root packet establishes tests only; the three GREEN packets own every implementation surface and cannot invalidate the required assertions.

## Preconditions

- `openspec/changes/sdd-fan-out-lens/tasks.md:23-47` remains the accepted task list.
- `internal/packet/packet_test.go:207`, `internal/packet/packet_test.go:340`, `internal/packet/packet_test.go:476`, `internal/packet/packet_test.go:518`, `internal/packet/packet_test.go:737`, and `internal/packet/packet_test.go:815` still expose the cited test seams.
- The eight explore/propose template files remain absent and `SKILL.md` still lacks the new contract, so the new assertions can demonstrate RED before GREEN work begins.

## Allowed paths

Only `internal/packet/packet_test.go` may be modified.

## Allowed paths outside the repository

None.

## Out of scope

- Do not create or edit any template, `SKILL.md`, parser/runtime code, DAG code, CLI code, or build script.
- Do not implement the behavior demanded by the tests or weaken an assertion to make current files pass.
- Do not rewrite existing design templates; they are fixtures to pin.
- Do not perform unrelated test cleanup, broad formatting, dependency changes, or compatibility refactors.

## Context

- Owned tasks, quoted from `tasks.md`: **1.0-RED, 1.1-RED, 1.3-RED, 2.1-RED, 2.2-RED, 2.3-RED, 2.5-RED, 3.1-RED, 3.1, 3.2** (`openspec/changes/sdd-fan-out-lens/tasks.md:23-47,55`). Strict TDD means this packet writes failing tests only.
- `internal/packet/packet_test.go:207-338` is the existing allowed-paths parse table; malformed non-JSON values must stay tied to `packet.ErrInvalidAllowedPaths`.
- `internal/packet/packet_test.go:340-474` pins incomplete-packet errors and the packet-template contract; extend the established error style rather than inventing a second harness.
- `internal/packet/packet_test.go:476-516` is `TestSkillAssetContract`, which already reads `SKILL.md` and uses targeted `strings.Contains` checks.
- `internal/packet/packet_test.go:518-539` demonstrates reading a template from disk and passing it to `packet.Parse`; it is only a pattern, not existing fan-out coverage.
- `internal/packet/packet_test.go:737-813` and `:815-887` cover feature-target and `legacy_main` parsing. Pin typed malformed-frontmatter behavior without changing parser code.
- Required template behavior is specified at `openspec/changes/sdd-fan-out-lens/specs/sdd-planning-fan-out/spec.md:53-73`; required skill drift checks are at `:75-89`.
- The design chooses table-driven `packet.Parse`, phase-local pairwise `DisjointAllowedPaths`, and targeted substring/table-row assertions at `openspec/changes/sdd-fan-out-lens/design.md:25-30,90-98`.
- The exact eight new asset paths and the three-wave-2 ownership boundaries are listed at `openspec/changes/sdd-fan-out-lens/tasks.md:56-58`. Structure the tests so each GREEN packet has a focused, separately runnable assertion even though the complete package becomes green only after all wave-2 commits integrate. This conservatively resolves the shared-suite verification ambiguity without changing the fixed parallel partition.

## Done Criteria & Hard Stops

### Done criteria

- [ ] `git diff -- internal/packet/packet_test.go` shows only test changes implementing every owned task above; no production file changed.
- [ ] `go test ./internal/packet` compiles and fails only on the newly introduced, expected missing-template or stale-`SKILL.md` contract assertions. Attach the failing test/subtest names and output; an unrelated failure is not acceptable RED evidence.
- [ ] The tests parse every explore, propose, and existing design template with `packet.Parse`, assert valid feature targeting, and call `DisjointAllowedPaths` pairwise within each phase. Evidence names the separately runnable explore, propose, and skill test/subtest seams.
- [ ] Malformed frontmatter is shown to map to the typed errors named by tasks 1.0-RED, including `ErrInvalidAllowedPaths`, the required-field/body errors, and `ErrInvalidLegacyMain`.
- [ ] Every assertion indirection has a terminal consumer: evidence traces fixture path tables into `os.ReadFile`, parsed bytes into `packet.Parse`, path sets into `DisjointAllowedPaths`, and skill substrings into the final `t.Errorf`/`t.Fatalf` failure sites; defining a table without those consumers is insufficient.
- [ ] `git status --porcelain` confirms all changed files are within the exact allowed path.
- [ ] Commit the work with a conventional commit message. Evidence: `git status --porcelain` is empty and `git log --oneline -1` shows the commit, with no AI attribution in the message, trailers, source comments, or generated documentation.

### Hard stops

Return `status: blocked` instead of guessing if any of these fires:

- A required precondition is false, including any implementation asset already satisfying the intended RED assertions.
- The work requires a path outside `internal/packet/packet_test.go`.
- A test requires an undocumented contract choice or contradicts `tasks.md`, `design.md`, or the delta spec.
- RED verification fails for an unrelated pre-existing reason or cannot compile.
- The tests cannot expose focused seams that let each parallel GREEN owner verify its own assertions without sibling files.
- Any test indirection has no identifiable terminal consumer.
- Completing the work would implement behavior owned by a GREEN packet or require weakening an assertion.
