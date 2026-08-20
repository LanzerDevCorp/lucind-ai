# Tasks: Read-Only Packet Dispatch

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 500–700 lines |
| 2000-line budget risk | Low (well below the 2000-line review budget) |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | single-pr |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
2000-line budget risk: Low (estimated 500–700 lines vs 2000-line review budget)

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | `internal/packet`: `ReadOnly` field + YAML parsing | PR 1 | `go test ./internal/packet -race -count=1` | N/A (unit tests) | `internal/packet/` |
| 2 | `internal/worktree`: `HasUniqueCommits` + `PorcelainEmpty` helpers | PR 1 | `go test ./internal/worktree -race -count=1` | N/A (real-git unit tests) | `internal/worktree/` |
| 3 | `internal/run`: `Deps` additions + `enforceCompletionMode` call site | PR 1 | `go test ./internal/run -race -count=1` | N/A (stubbed deps tests) | `internal/run/` |
| 4 | `cmd/lucind-ai`: Wire real git helpers to runtime `Deps` | PR 1 | `go test ./cmd/lucind-ai -race -count=1` | CLI harness | `cmd/lucind-ai/cli.go` |
| 5 | Prompt assets, schemas & docs: templates, skill, result schema | PR 1 | `go test ./internal/result -race -count=1` | Schema validator | `plugin/claude-code/skills/`, `internal/result/` |
| 6 | End-to-end verification: Full test suite + real read-only dispatch | PR 1 | `go test ./... -race -count=1` | `lucind-ai run --packet <path>` | Full repository |

---

## Unit 1: `internal/packet` (ReadOnly Field & Parsing)

- [ ] 1.1 RED `internal/packet/packet_test.go`: Write unit tests for frontmatter `read_only` parsing:
  - Explicit `read_only: true` sets `Packet.ReadOnly = true` (cite: `specs/read-only-packet-schema/spec.md#Requirement: Frontmatter Read-Only Field Parsing`).
  - Explicit `read_only: false` sets `Packet.ReadOnly = false` (cite: `specs/read-only-packet-schema/spec.md#Requirement: Frontmatter Read-Only Field Parsing`).
  - Non-boolean values (strings like `"yes"`, numbers) return a parse error and reject the packet (cite: `specs/read-only-packet-schema/spec.md#Requirement: Frontmatter Read-Only Field Parsing`).
  - Omitted `read_only` key defaults `Packet.ReadOnly = false` and preserves existing required-key validations (`ErrMissingID`, `ErrMissingExecutor`, `ErrMissingRoutedBy`, `ErrEmptyBody`) and unknown-key drop (cite: `specs/read-only-packet-schema/spec.md#Requirement: Default Value and Backward Compatibility`, `specs/read-only-packet-schema/spec.md#Requirement: Additive Rollback`).
  - Verify `explore-` packet IDs and empty allowed paths lists do not infer `read_only` mode (cite: `specs/read-only-packet-schema/spec.md#Requirement: Explicit Flag Only — No Inference`).
- [ ] 1.2 GREEN `internal/packet/packet.go`: Add `ReadOnly bool` to `Packet` struct. In `Parse`, add `case "read_only":` in the frontmatter scanner switch to parse boolean values (`true`/`false`) into `p.ReadOnly`, returning an error on non-boolean values while keeping zero-value `false` when omitted (cite: `specs/read-only-packet-schema/spec.md#Requirement: Frontmatter Read-Only Field Parsing`, `specs/read-only-packet-schema/spec.md#Requirement: Default Value and Backward Compatibility`).

---

## Unit 2: `internal/worktree` (Git-Inspection Helpers)

- [ ] 2.1 RED `internal/worktree/worktree_test.go`: Write real-git tests for worktree git inspection helpers:
  - `HasUniqueCommits`:
    - Returns `false, nil` on a freshly created linked worktree whose `HEAD` matches the primary repo `HEAD` / merge-base (cite: `specs/read-only-done-criterion/spec.md#Requirement: Read-Only Packets Replace Criterion 2`, `specs/completion-mode-enforcement/spec.md#Requirement: Post-Status Git Verification, Not Envelope Trust`).
    - Returns `true, nil` after committing a new commit on the worktree's branch (cite: `specs/read-only-done-criterion/spec.md#Requirement: Read-Only Packets Replace Criterion 2`, `specs/completion-mode-enforcement/spec.md#Requirement: Write Packet Completion Matrix`).
    - Returns error wrapping git stderr when executed against a non-git or inaccessible directory (cite: `specs/completion-mode-enforcement/spec.md#Requirement: Git Inspection Failure Resolves to Failed, Not Blocked`).
  - `PorcelainEmpty`:
    - Returns `true, nil` on a clean working tree (cite: `specs/completion-mode-enforcement/spec.md#Requirement: Post-Status Git Verification, Not Envelope Trust`).
    - Returns `true, nil` when `.lucind/result.json` exists in the worktree, confirming gitignored files do not dirty porcelain (cite: `specs/read-only-done-criterion/spec.md#Requirement: The Protocol Envelope Is Not a Mutation`).
    - Returns `false, nil` when an untracked or modified non-ignored file is present (cite: `specs/completion-mode-enforcement/spec.md#Requirement: Write Packet Completion Matrix#Scenario: Write lane with a dirty tree fails`, `specs/completion-mode-enforcement/spec.md#Requirement: Read-Only Packet Completion Matrix#Scenario: Read-only lane with a dirty tree fails`).
    - Returns error wrapping git stderr on git failure (cite: `specs/completion-mode-enforcement/spec.md#Requirement: Git Inspection Failure Resolves to Failed, Not Blocked`).
- [ ] 2.2 GREEN `internal/worktree/worktree.go`: Implement git inspection functions:
  - `HasUniqueCommits(ctx context.Context, worktreePath string) (bool, error)`: executes git command to check whether worktree `HEAD` equals `git merge-base HEAD <primary-HEAD>` (or has unique commits relative to primary base).
  - `PorcelainEmpty(ctx context.Context, worktreePath string) (bool, error)`: executes `git status --porcelain` in `worktreePath` and reports whether output is empty. (cite: `specs/completion-mode-enforcement/spec.md#Requirement: Post-Status Git Verification, Not Envelope Trust`, `specs/read-only-done-criterion/spec.md#Requirement: Read-Only Packets Replace Criterion 2`, `specs/read-only-done-criterion/spec.md#Requirement: The Protocol Envelope Is Not a Mutation`).

---

## Unit 3: `internal/run` (Completion Mode Enforcement)

- [ ] 3.1 RED `internal/run/run_test.go`: Add tests for completion mode enforcement and update test harness:
  - Update `newTestDeps` to stub `HasUniqueLaneCommits` (defaulting `true`) and `PorcelainEmpty` (defaulting `true`) so existing write tests pass unmodified (cite: `specs/completion-mode-enforcement/spec.md#Requirement: Write Packet Completion Matrix`).
  - Write packet completion matrix tests (`ReadOnly: false`):
    - Write packet + `Done` envelope + unique commits + porcelain clean -> remains `lane.Done` (cite: `specs/completion-mode-enforcement/spec.md#Requirement: Write Packet Completion Matrix#Scenario: Compliant write lane`).
    - Write packet + `Done` envelope + zero unique commits -> transitions to `lane.Failed` with ledger note naming missing commit, ignoring any self-reported `commit` field in envelope (cite: `specs/completion-mode-enforcement/spec.md#Requirement: Write Packet Completion Matrix#Scenario: Write lane without a commit fails (disclosed behavior change)`, `specs/completion-mode-enforcement/spec.md#Requirement: Post-Status Git Verification, Not Envelope Trust#Scenario: A self-reported commit hash is not evidence`).
    - Write packet + `Done` envelope + unique commits + dirty porcelain -> transitions to `lane.Failed` with ledger note (cite: `specs/completion-mode-enforcement/spec.md#Requirement: Write Packet Completion Matrix#Scenario: Write lane with a dirty tree fails`).
  - Read-only packet completion matrix tests (`ReadOnly: true`):
    - Read-only packet + `Done` envelope + zero unique commits + porcelain clean -> remains `lane.Done` (cite: `specs/completion-mode-enforcement/spec.md#Requirement: Read-Only Packet Completion Matrix#Scenario: Compliant read-only lane`).
    - Read-only packet + `Done` envelope + unique commits -> transitions to `lane.Failed` with ledger note (cite: `specs/completion-mode-enforcement/spec.md#Requirement: Read-Only Packet Completion Matrix#Scenario: Read-only lane that committed fails`).
    - Read-only packet + `Done` envelope + dirty porcelain -> transitions to `lane.Failed` with ledger note (cite: `specs/completion-mode-enforcement/spec.md#Requirement: Read-Only Packet Completion Matrix#Scenario: Read-only lane with a dirty tree fails`).
  - Bypass test: Non-`Done` envelope statuses (`lane.Blocked`, `lane.Deviated`, `lane.Failed`) bypass completion mode checks and persist directly (cite: `specs/completion-mode-enforcement/spec.md#Requirement: Post-Status Git Verification, Not Envelope Trust#Scenario: Non-Done statuses bypass the check entirely`).
  - Error resolution test: Error from `HasUniqueLaneCommits` or `PorcelainEmpty` results in `lane.Failed` (not `Blocked`) with error details recorded in ledger note (cite: `specs/completion-mode-enforcement/spec.md#Requirement: Git Inspection Failure Resolves to Failed, Not Blocked`).
- [ ] 3.2 GREEN `internal/run/run.go`: Add `HasUniqueLaneCommits` and `PorcelainEmpty` to `Deps`. Implement `enforceCompletionMode(ctx context.Context, deps Deps, worktreePath string, p packet.Packet) (lane.Status, string)` called in `Execute` strictly after `decideStatus` returns `lane.Done` and before `SetStatus`. On verification failure or git error, return `lane.Failed` with diagnostic note and append `EventLaneNote` event (cite: `specs/completion-mode-enforcement/spec.md#Requirement: Post-Status Git Verification, Not Envelope Trust`, `specs/completion-mode-enforcement/spec.md#Requirement: Write Packet Completion Matrix`, `specs/completion-mode-enforcement/spec.md#Requirement: Read-Only Packet Completion Matrix`, `specs/completion-mode-enforcement/spec.md#Requirement: Git Inspection Failure Resolves to Failed, Not Blocked`).

---

## Unit 4: `cmd/lucind-ai/cli.go` (CLI Wiring)

- [ ] 4.1 RED `cmd/lucind-ai/cli_test.go` (or integration tests): Write test verifying CLI dependencies wire git-backed `HasUniqueLaneCommits` and `PorcelainEmpty` to real worktree helpers (cite: `specs/completion-mode-enforcement/spec.md#Requirement: Post-Status Git Verification, Not Envelope Trust`).
- [ ] 4.2 GREEN `cmd/lucind-ai/cli.go`: In `runDispatch`, populate `lucindrun.Deps` with `HasUniqueLaneCommits: worktree.HasUniqueCommits` and `PorcelainEmpty: worktree.PorcelainEmpty` (cite: `specs/completion-mode-enforcement/spec.md#Requirement: Post-Status Git Verification, Not Envelope Trust`).

---

## Unit 5: Prompt Assets, Schemas & Documentation Updates

- [ ] 5.1 Update `internal/result/result.schema.json`: Update the `commit` property description to state that `commit` is omitted for read-only packets and that the binary independently validates git state without trusting self-reported values. Ensure `commit` remains optional and `additionalProperties: false` is maintained (cite: `specs/read-only-packet-schema/spec.md#Requirement: The Envelope Cannot Declare or Override Mode`).
- [ ] 5.2 Update `plugin/claude-code/skills/lucind-ai/assets/packet-template.md`: Retain default write skeleton (omitting `read_only` key). Add a note in the template instructing authors that setting `read_only: true` replaces mandatory criterion 2 with the unchanged worktree check (`git status --porcelain` empty and `HEAD` equals `git merge-base HEAD <primary HEAD>`) (cite: `specs/read-only-done-criterion/spec.md#Requirement: Authoring Assets Document the Exception`, `specs/read-only-done-criterion/spec.md#Requirement: Read-Only Packets Replace Criterion 2`).
- [ ] 5.3 Update `plugin/claude-code/skills/lucind-ai/SKILL.md`: Update the `explore` blocker row (line 78) to document `explore` as dispatchable via `lucind-ai run` with `read_only: true`, and update the mandatory criterion 2 description (line 96) to document the read-only exception (cite: `specs/read-only-done-criterion/spec.md#Requirement: Authoring Assets Document the Exception`).

---

## Unit 6: End-to-End Verification & Integration Check

- [ ] 6.1 Run full project test suite via `go test -race -count=1 ./...` and ensure all packages pass cleanly (cite: `specs/completion-mode-enforcement/spec.md#Requirement: Combine Stays Unaware of Read-Only Lanes`).
- [ ] 6.2 End-to-end runtime check: Dispatch a real `read_only: true` packet via `lucind-ai run --packet <path>` in a real repository worktree and confirm it reaches `done` status with zero unique commits and a clean working tree without making a git commit (cite: `specs/completion-mode-enforcement/spec.md#Requirement: Read-Only Packet Completion Matrix#Scenario: Compliant read-only lane`, `specs/read-only-done-criterion/spec.md#Requirement: Read-Only Packets Replace Criterion 2`).
