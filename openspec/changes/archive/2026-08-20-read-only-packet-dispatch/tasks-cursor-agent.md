# Tasks: Read-Only Packet Dispatch (cursor-agent)

Strict TDD. Runner: `go test ./...`. Every code item is one red test, then the smallest green that makes it pass. Do not bulk-write tests, then bulk-write code.

`Packet.ReadOnly` has one terminal consumer: `run.enforceCompletionMode`, called from `Execute` after `decideStatus` returns `lane.Done` and before `Ledger.SetStatus`. Git state is the authority. Do not trust `Envelope.Commit` or `Envelope.FilesChanged`.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 900–1300 |
| `review_budget_lines` | 2000 (`state.yaml`) |
| Review-budget risk | **Low** — well under 2000 |
| 400-line chained-PR pressure | Medium if a later slice prefers smaller PRs; this change's delivery strategy is `single-pr` |
| Suggested split | Single apply PR |
| Delivery strategy | `single-pr` |

The list would not plausibly exceed 2000 changed lines. Largest slices are `internal/run/run_test.go` (Done-outcome matrix plus test-helper stubs) and `internal/worktree/worktree_test.go` (real-git helpers). Docs and CLI wiring are small.

### Suggested Work Units

| Unit | Goal | Focused test command | Runtime harness | Rollback boundary |
|------|------|----------------------|-----------------|-------------------|
| 1 | `ReadOnly` field + `read_only:` parse | `go test ./internal/packet -count=1` | N/A: `strings.NewReader` | `internal/packet/packet.go`, `internal/packet/packet_test.go` |
| 2 | `HasUniqueCommits` / `PorcelainEmpty` | `go test ./internal/worktree -count=1` | Real git via `initRepo` (skip in `-short`) | `internal/worktree/worktree.go`, `internal/worktree/worktree_test.go` |
| 3 | `enforceCompletionMode` + Execute call site | `go test ./internal/run -count=1` | N/A: `fakeExecutor` + stubbed Deps | `internal/run/run.go`, `internal/run/run_test.go`, stubs in `batch_test.go` |
| 4 | CLI wires git-backed Deps funcs | `go test ./cmd/lucind-ai -count=1` | Production assignment consumed by `run()` → `ExecuteBatch` | `cmd/lucind-ai/cli.go` |
| 5 | Template, SKILL, envelope `commit` description | `go test ./internal/result -count=1` | String assertions on assets | `packet-template.md`, `SKILL.md`, `internal/result/result.schema.json` |
| 6 | End-to-end `lucind-ai run` | (manual) | `lucind-ai run --packet <read_only packet>` reaches `done` with no unique commits | whole change |

## Sequencing notes (not blockers)

`design.md` already chose the architecture. These are implementation details to keep, not open product questions:

1. **Deps stubs before the call site.** `newTestDeps` (`internal/run/run_test.go:74`) and `newBatchTestDeps` (`internal/run/batch_test.go:144`) must stub `HasUniqueLaneCommits=true` and `PorcelainEmpty=true` in the same green that introduces the Execute call. Design testing strategy: the existing happy-path write test keeps passing *unmodified* because the helpers stub a compliant write tree. Without those stubs, every current Done path starts failing the disclosed write-packet commit gate.
2. **Helper vs Deps signatures.** Package helpers may take `primaryRoot` so merge-base has an authority (`worktree.HasUniqueCommits(ctx, worktreePath, primaryRoot)`). `Deps.HasUniqueLaneCommits` stays `(ctx, worktreePath)` per Decision 3. `cli.go` closes over `primaryRoot`. No third git-root selector (threat matrix: `git -C worktreePath` and `git -C primaryRoot` only).
3. **Parse stays line-based.** Accept trimmed `true` / `false` in the existing `strings.Cut` switch. Do not add a YAML library.
4. **`decideStatus` stays a pure mapping.** Git inspection is a separate function. `internal/run/integrate.go` is not modified; a passed read-only lane is still combined.

## Constraints (do not)

- Do not modify `internal/run/integrate.go` or filter read-only lanes out of `CombineTree`.
- Do not add `read_only` to `result.schema.json`. `additionalProperties: false` stays.
- Do not put `commit` into `required`. Do not bump ledger or envelope schema versions. No feature flag. No SQLite migration.
- Do not change `plugin/claude-code/skills/lucind-ai/assets/human-packet-template.md`.
- Do not infer mode from packet `id`, lane name, or any path list (`apply-dag-dispatch` owns path lists and is out of scope).
- Do not inspect git inside `decideStatus`. Do not treat `Envelope.Commit` / `Envelope.FilesChanged` as evidence.

---

## 1. `internal/packet` — ReadOnly field + parsing

Spec: `specs/read-only-packet-schema/spec.md`.

- [ ] 1.1 **RED** `internal/packet/packet_test.go`: table-driven `TestParseReadOnlyFrontmatter`. Cases: `read_only: true` → `Packet.ReadOnly == true`; `read_only: false` → `false`; omitted key → `false`; `id: explore-foo` with key omitted → `false` (no inference from name); unrecognized sibling keys still ignored and `ReadOnly` stays `false`. Spec: [Frontmatter Read-Only Field Parsing](specs/read-only-packet-schema/spec.md#requirement-frontmatter-read-only-field-parsing), [Default Value and Backward Compatibility](specs/read-only-packet-schema/spec.md#requirement-default-value-and-backward-compatibility), [Explicit Flag Only — No Inference](specs/read-only-packet-schema/spec.md#requirement-explicit-flag-only--no-inference).
- [ ] 1.2 **GREEN** `internal/packet/packet.go`: add `ReadOnly bool` to `Packet` (zero value `false`). Parse `read_only:` in the existing frontmatter switch. `go test ./internal/packet -count=1` green for 1.1.
- [ ] 1.3 **RED** `internal/packet/packet_test.go`: non-boolean values (`yes`, `1`, `"true"`, empty) return a new sentinel (e.g. `ErrInvalidReadOnly`) and reject the packet. Completeness gates unchanged: missing `id` / `executor` / `routed_by` / empty body still return `ErrMissingID` / `ErrMissingExecutor` / `ErrMissingRoutedBy` / `ErrEmptyBody` even when `read_only: true` is present. Extend `TestParseRejectsIncompletePackets` rather than inventing a second gate. Spec: [Frontmatter Read-Only Field Parsing](specs/read-only-packet-schema/spec.md#requirement-frontmatter-read-only-field-parsing), [Default Value and Backward Compatibility](specs/read-only-packet-schema/spec.md#requirement-default-value-and-backward-compatibility).
- [ ] 1.4 **GREEN** `internal/packet/packet.go`: reject non-boolean `read_only` with that sentinel. Do not add `read_only` to the completeness switch. `go test ./internal/packet -count=1` green.

Rollback constraint (no extra code): after these commits are reverted, `read_only:` becomes an unknown key again and is dropped. Do not add a schema/ledger version bump while implementing this unit. Spec: [Additive Rollback](specs/read-only-packet-schema/spec.md#requirement-additive-rollback).

---

## 2. `internal/worktree` — HasUniqueCommits / PorcelainEmpty

Spec: `specs/read-only-done-criterion/spec.md`. Real git, same `initRepo` / `runGit` / `testing.Short` pattern as `TestCreateAddsLinkedWorktree`.

Unique commits means worktree `HEAD` ≠ `git merge-base HEAD <primary HEAD>`. Porcelain-empty means `git status --porcelain` produces no output. Use git's default ignore rules.

- [ ] 2.1 **RED** `internal/worktree/worktree_test.go`: after `Create`, a fresh worktree reports no unique commits (`HEAD` equals `merge-base HEAD <primary HEAD>`). After one commit on the lane branch, unique commits are present. Skip in `-short`. Spec: [Read-Only Packets Replace Criterion 2](specs/read-only-done-criterion/spec.md#requirement-read-only-packets-replace-criterion-2).
- [ ] 2.2 **GREEN** `internal/worktree/worktree.go`: add `HasUniqueCommits(ctx, worktreePath, primaryRoot)` using `git -C` on those two roots only. Honour a cancelled `ctx` the way `Create` does. `go test ./internal/worktree -count=1` green for 2.1.
- [ ] 2.3 **RED** `internal/worktree/worktree_test.go`: an untracked non-ignored file makes `PorcelainEmpty` false. Writing only `.lucind/result.json` in a repo whose `.gitignore` contains `.lucind/` makes `PorcelainEmpty` true. Spec: [The Protocol Envelope Is Not a Mutation](specs/read-only-done-criterion/spec.md#requirement-the-protocol-envelope-is-not-a-mutation), [Write Packets Keep Criterion 2 Unchanged](specs/read-only-done-criterion/spec.md#requirement-write-packets-keep-criterion-2-unchanged).
- [ ] 2.4 **GREEN** `internal/worktree/worktree.go`: add `PorcelainEmpty(ctx, worktreePath)` via `git -C worktreePath status --porcelain`. `go test ./internal/worktree -count=1` green.

---

## 3. `internal/run` — enforceCompletionMode call site

Spec: `specs/completion-mode-enforcement/spec.md`. Stub the two new `Deps` funcs; do not shell out to git in this package. `decideStatus` (`internal/run/run.go:407`) stays a pure timeout / exit / envelope map.

Call site: immediately after `decideStatus` (`run.go:315`), before `SetStatus` (`run.go:338`). On a miss: `status = lane.Failed` and a descriptive ledger note. Git-inspection errors are `Failed`, never `Blocked`.

A packet with `ReadOnly == false` and no allowed-path list is still write. Spec: [Explicit Flag Only — No Inference](specs/read-only-packet-schema/spec.md#requirement-explicit-flag-only--no-inference).

- [ ] 3.1 **RED** `internal/run/run.go`, `internal/run/run_test.go`: add `HasUniqueLaneCommits` and `PorcelainEmpty` to `Deps`. Stub both in `newTestDeps` and `newBatchTestDeps` as `true` (compliant write tree). Write `TestExecuteWriteDoneWithoutUniqueCommitsFails`: write packet, envelope `done` with a non-empty `commit` string, stubs report no unique commits → `lane.Failed`, ledger note names the missing commit, self-reported hash is ignored. This test is red until the call site exists. Spec: [Post-Status Git Verification, Not Envelope Trust](specs/completion-mode-enforcement/spec.md#requirement-post-status-git-verification-not-envelope-trust), [Write Packet Completion Matrix](specs/completion-mode-enforcement/spec.md#requirement-write-packet-completion-matrix).
- [ ] 3.2 **GREEN** `internal/run/run.go`: add `enforceCompletionMode` (or equivalent). Call it only when `decideStatus` returned `lane.Done`, before `SetStatus`. Write packet: unique commits **and** porcelain empty, else `Failed` + note. Existing `TestExecuteHappyPathEnvelopeDoneReachesLaneDone` stays green unmodified via the helper stubs. `go test ./internal/run -count=1` green for 3.1 plus existing tests.
- [ ] 3.3 **RED** `internal/run/run_test.go`: write packet, unique commits true, porcelain false → `Failed` + ledger note. Spec: [Write Packet Completion Matrix](specs/completion-mode-enforcement/spec.md#requirement-write-packet-completion-matrix).
- [ ] 3.4 **GREEN** `internal/run/run.go`: dirty write tree fails the check. `go test ./internal/run -count=1` green.
- [ ] 3.5 **RED** `internal/run/run_test.go`: `ReadOnly: true`, no unique commits, porcelain empty → stays `Done`. `ReadOnly: true`, unique commits true (even with porcelain empty) → `Failed` + note. `ReadOnly: true`, porcelain dirty → `Failed` + note. Use `testPacket()` plus `ReadOnly: true`; do not infer mode from `id`. Spec: [Read-Only Packet Completion Matrix](specs/completion-mode-enforcement/spec.md#requirement-read-only-packet-completion-matrix), [Read-Only Packets Replace Criterion 2](specs/read-only-done-criterion/spec.md#requirement-read-only-packets-replace-criterion-2).
- [ ] 3.6 **GREEN** `internal/run/run.go`: read-only matrix. `go test ./internal/run -count=1` green.
- [ ] 3.7 **RED** `internal/run/run_test.go`: `decideStatus` returned `Blocked` / `Deviated` / `Failed` → persist that status, do not call the git stubs (spy: call count stays 0). When `HasUniqueLaneCommits` or `PorcelainEmpty` returns an error on a Done path → `Failed`, error text in the ledger note, not `Blocked`. Spec: [Post-Status Git Verification, Not Envelope Trust](specs/completion-mode-enforcement/spec.md#requirement-post-status-git-verification-not-envelope-trust), [Git Inspection Failure Resolves to Failed, Not Blocked](specs/completion-mode-enforcement/spec.md#requirement-git-inspection-failure-resolves-to-failed-not-blocked).
- [ ] 3.8 **GREEN** `internal/run/run.go`: skip the check off the Done path; map inspection errors to `Failed`. `go test ./internal/run -count=1` green.
- [ ] 3.9 **RED** `internal/run/integrate_test.go`: a Done lane's branch is still passed to `CombineTree` (recorder already captures `Branches`). Do not add a `ReadOnly` filter. Spec: [Combine Stays Unaware of Read-Only Lanes](specs/completion-mode-enforcement/spec.md#requirement-combine-stays-unaware-of-read-only-lanes).
- [ ] 3.10 **GREEN** none in `integrate.go`. If 3.9 is already true of the current recorder tests, keep a named regression case so a later filter cannot land quietly. `go test ./internal/run -count=1` green.

---

## 4. `cmd/lucind-ai/cli.go` — wiring

Spec: [Post-Status Git Verification, Not Envelope Trust](specs/completion-mode-enforcement/spec.md#requirement-post-status-git-verification-not-envelope-trust). Terminal consumer of this assignment is `Execute` → `enforceCompletionMode`.

- [ ] 4.1 **RED** `cmd/lucind-ai/cli.go`, `cmd/lucind-ai/cli_test.go`: failing proof that production `run()` Deps sets `HasUniqueLaneCommits` and `PorcelainEmpty` to non-nil git-backed funcs. If the inlined `lucindrun.Deps{...}` literal (`cli.go:169`) has no observable seam, extract the smallest package-level constructor whose only consumer is `run()`'s `ExecuteBatch` call. Do not add an unused wrapper.
- [ ] 4.2 **GREEN** `cmd/lucind-ai/cli.go`: `HasUniqueLaneCommits` closes over `primaryRoot` and calls `worktree.HasUniqueCommits`; `PorcelainEmpty` is `worktree.PorcelainEmpty`. `go test ./cmd/lucind-ai -count=1` green.

---

## 5. Prompt assets and envelope schema description

Specs: `specs/read-only-done-criterion/spec.md`, `specs/read-only-packet-schema/spec.md`.

- [ ] 5.1 **RED** `internal/result/result_test.go`: add `TestReadSchemaViolations` case `"read_only top-level property"` (sibling of `"unknown top-level property"` at `result_test.go:228`) — a valid envelope plus `"read_only": true` returns `ErrSchemaInvalid`. Assert a minimal envelope that omits `commit` still `Read`s (already true of `TestReadMinimalEnvelopeMapsLaneStatus`; keep an explicit name). Unmarshal `result.SchemaJSON()` and assert `commit` is not in `required`. Spec: [The Envelope Cannot Declare or Override Mode](specs/read-only-packet-schema/spec.md#requirement-the-envelope-cannot-declare-or-override-mode).
- [ ] 5.2 **GREEN** `internal/result/result.schema.json`: change only the `commit` description — omitted on a read-only packet; the binary does not trust this field for enforcement. Do not add a `read_only` property. Do not change `required` or `additionalProperties`. `go test ./internal/result -count=1` green.
- [ ] 5.3 **RED** a Go test that reads `plugin/claude-code/skills/lucind-ai/assets/packet-template.md` from the module root: example frontmatter omits `read_only`; criterion 2 still requires a commit (`git status --porcelain` / `git log --oneline -1`); a "Read-only packets" note tells an author who sets `read_only: true` to replace criterion 2 with porcelain-empty **and** `HEAD` equals `git merge-base HEAD <primary HEAD>`. Put the test next to existing packet tests (`internal/packet/packet_test.go`) so it has a home without a new package. Spec: [Authoring Assets Document the Exception](specs/read-only-done-criterion/spec.md#requirement-authoring-assets-document-the-exception), [Write Packets Keep Criterion 2 Unchanged](specs/read-only-done-criterion/spec.md#requirement-write-packets-keep-criterion-2-unchanged).
- [ ] 5.4 **GREEN** `plugin/claude-code/skills/lucind-ai/assets/packet-template.md`: add that note. Skeleton frontmatter stays write-default. `go test ./internal/packet -count=1` green.
- [ ] 5.5 **RED** same style of file-content test for `plugin/claude-code/skills/lucind-ai/SKILL.md`: explore is documented as dispatchable via `lucind-ai run`; criterion-2 bullet states the read-only exception; the explore-blocker row no longer says the exception is missing. Spec: [Authoring Assets Document the Exception](specs/read-only-done-criterion/spec.md#requirement-authoring-assets-document-the-exception).
- [ ] 5.6 **GREEN** `plugin/claude-code/skills/lucind-ai/SKILL.md`: update the explore row (`SKILL.md:78`) and mandatory-criterion-2 bullet (`SKILL.md:96`). Leave the `apply` and `verify` blocker rows alone. `go test ./internal/packet -count=1` green.
- [ ] 5.7 **Verify (no edit)** `plugin/claude-code/skills/lucind-ai/assets/human-packet-template.md` is unmodified (`git diff --exit-code --` that path). Spec: [Authoring Assets Document the Exception](specs/read-only-done-criterion/spec.md#requirement-authoring-assets-document-the-exception) scenario "Human packet template is untouched".

---

## 6. End-to-end check

- [ ] 6.1 Dispatch a real packet with `read_only: true` via `lucind-ai run`. Confirm the lane reaches `done` without a commit: `git status --porcelain` empty in the lane worktree, and worktree `HEAD` equals `git merge-base HEAD <primary HEAD>`. The envelope may omit `commit`. This is the terminal proof that units 1–5 compose. Spec: [Read-Only Packet Completion Matrix](specs/completion-mode-enforcement/spec.md#requirement-read-only-packet-completion-matrix) scenario "Compliant read-only lane", [Read-Only Packets Replace Criterion 2](specs/read-only-done-criterion/spec.md#requirement-read-only-packets-replace-criterion-2).

After 6.1: `go test ./... -count=1` still green.
