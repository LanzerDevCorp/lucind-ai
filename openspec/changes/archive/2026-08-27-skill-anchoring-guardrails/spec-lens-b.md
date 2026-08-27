# Spec Lens B — Scenarios & Coverage: Skill Anchoring & Worktree Cleanup Guardrails

## Assumed requirements

This lens assumes the six requirements from the accepted proposal (`openspec/changes/skill-anchoring-guardrails/proposal.md:69-128`): fail-closed worktree cleanup with `--force` (`internal/worktree/worktree.go:247-261`, `cmd/lucind-ai/cli.go:1934-1978`), non-done lane guidance banners (`cmd/lucind-ai/cli.go:698-726`), reverted integration retry banners (`cmd/lucind-ai/cli.go:730-740`), qualitative acceptance review reminders (`cmd/lucind-ai/cli.go:685-690`), multi-wave DAG split base SHA warnings (`cmd/lucind-ai/cli.go:485-516`), and prescriptive TDD WIP-rescue documentation (`.agents/skills/lucind-apply/SKILL.md:10-21`).

## Scenarios

### Requirement: Worktree cleanup dirty guardrail and force flag

#### Scenario: Refuse cleanup on dirty worktree without force

- GIVEN a linked worktree where `PorcelainEmpty` reports false (`internal/worktree/worktree.go:319-325`)
- WHEN running `lucind-ai worktree cleanup --lane <id>` (`cmd/lucind-ai/cli.go:1934-1978`) without `--force`
- THEN the command MUST exit 1, output porcelain status citing `troubleshooting.md`, and preserve files on disk

#### Scenario: Force cleanup removes dirty worktree

- GIVEN a dirty linked worktree
- WHEN running `lucind-ai worktree cleanup --lane <id> --force` (`cmd/lucind-ai/cli.go:1934-1978`)
- THEN the command MUST delete the worktree via `worktree.Remove` (`internal/worktree/worktree.go:256-261`) and exit 0

#### Scenario: Clean worktree cleanup succeeds idempotently

- GIVEN a clean linked worktree (`PorcelainEmpty` true) or nonexistent path
- WHEN running `lucind-ai worktree cleanup --lane <id>` without `--force`
- THEN `worktree.Cleanup` MUST remove the worktree if present (`internal/worktree/worktree_test.go:1034-1069`) and exit 0

### Requirement: Blocked and timeout lane report guidance banner

#### Scenario: Non-done lane displays troubleshooting banner

- GIVEN a lane ending with status `blocked`, `failed`, or timeout (`internal/run/run.go:452-465`)
- WHEN `printReport` formats output (`cmd/lucind-ai/cli.go:698-726`)
- THEN output MUST include a banner with worktree path, diff inspection steps (`git diff`), and reference to `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18`

#### Scenario: Done lane omits troubleshooting banner

- GIVEN a lane completing with `lane.Done`
- WHEN `printReport` renders output (`cmd/lucind-ai/cli.go:698-726`, `cmd/lucind-ai/cli_test.go:685-724`)
- THEN output MUST NOT display the non-done warning banner or `troubleshooting.md` reference

### Requirement: Integration report reverted IDs recovery banner

#### Scenario: Reverted integration outcome surfaces retry instructions

- GIVEN an integration batch with non-empty `reverted_ids` (`cmd/lucind-ai/cli.go:730-740`, `cmd/lucind-ai/cli_test.go:729-777`)
- WHEN `printIntegrateReport` renders summary
- THEN output MUST list `reverted_ids` and append retry instructions with `lucind-ai integrate retry --run <run-id>` referencing `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:33-35`

#### Scenario: Fully integrated batch omits recovery banner

- GIVEN an integration batch where all lanes integrated and `reverted_ids` is empty
- WHEN `printIntegrateReport` formats output (`cmd/lucind-ai/cli.go:730-740`)
- THEN `reverted_ids:` MUST be printed explicitly empty (`reverted_ids:\n`) without the retry guidance banner

### Requirement: Acceptance receipt qualitative review banner

#### Scenario: Mechanical acceptance output prints qualitative checklist reminder

- GIVEN a candidate commit passing mechanical checks (`internal/accept/accept.go:118-130`)
- WHEN `renderAcceptanceReceipt` renders output (`cmd/lucind-ai/cli.go:685-690`, `cmd/lucind-ai/cli_test.go:4503-4545`)
- THEN output MUST state mechanical evidence passed and append a reminder to complete qualitative checklist steps 2–10 from `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30`

#### Scenario: Failing mechanical checks exit non-zero without receipt

- GIVEN a candidate commit failing mechanical checks (`internal/accept/accept.go:118-120`)
- WHEN `lucind-ai accept` executes
- THEN the command MUST exit non-zero and MUST NOT output an acceptance receipt or qualitative checklist reminder

### Requirement: DAG split multi-wave base SHA warning banner

#### Scenario: Multi-wave DAG split emits base SHA warning

- GIVEN an `apply-dag.yaml` defining two or more sequential waves (`internal/dag/split.go:34-43`, `internal/dag/split_test.go:13-111`)
- WHEN `runSplit` executes `dag.Split` (`cmd/lucind-ai/cli.go:485-516`)
- THEN output MUST append a warning banner to advance checkout and refresh `base_sha`/`expected_parent_sha` per `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30`

#### Scenario: Single-wave DAG split omits base SHA warning

- GIVEN an `apply-dag.yaml` defining a single wave
- WHEN `runSplit` executes (`cmd/lucind-ai/cli.go:485-516`)
- THEN output MUST emit the single `lucind-ai run` command without the multi-wave warning banner

### Requirement: Prescriptive TDD WIP-rescue protocol documentation

#### Scenario: Operator executes TDD WIP-rescue after lane timeout

- GIVEN an apply lane timing out with uncommitted progress (`internal/worktree/worktree.go:150-159`, `internal/run/run.go:452-465`)
- WHEN following the rescue protocol in `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` and `.agents/skills/lucind-apply/SKILL.md:10-21`
- THEN the operator inspects diffs, commits WIP, updates packet timeout, and re-dispatches without data loss

## Coverage

| Requirement | Happy path | Edge case | Error state | Testable through (file:line) |
|---|---|---|---|---|
| Worktree cleanup dirty guardrail and force flag | `worktree cleanup -f` deletes dirty worktree; clean worktree succeeds without `-f` | Nonexistent path succeeds idempotently | Unforced cleanup on dirty worktree exits 1 and preserves files | `internal/worktree/worktree_test.go:255-266,536-595,1034-1069`, `cmd/lucind-ai/cli_test.go:2974-3012` |
| Blocked and timeout lane report guidance banner | Non-done lane outputs banner with worktree path and `troubleshooting.md` link | Blocked lane with empty diagnosis renders banner cleanly | Done lane omits non-done banner | `cmd/lucind-ai/cli_test.go:685-724` |
| Integration report reverted IDs recovery banner | Non-empty `reverted_ids` prints retry banner and `recovery-reconciliation.md` link | Mixed integrated and reverted IDs prints both lists and banner | Clean integration omits recovery banner | `cmd/lucind-ai/cli_test.go:729-777` |
| Acceptance receipt qualitative review banner | Passing candidate renders receipt and reminder for checklist steps 2–10 in `acceptance-promotion.md` | Non-promotion execution verifies mechanical evidence only | Failing mechanical checks exit non-zero without receipt or banner | `cmd/lucind-ai/cli_test.go:4503-4545`, `internal/accept/accept.go:118-130` |
| DAG split multi-wave base SHA warning banner | Multi-wave DAG split outputs wave commands and base SHA refresh warning banner | Single-wave DAG emits commands without warning banner | Invalid DAG structure fails before wave generation | `internal/dag/split_test.go:13-111`, `cmd/lucind-ai/cli.go:485-516` |
| Prescriptive TDD WIP-rescue protocol documentation | Operator rescues uncommitted files from preserved worktree after timeout | Preserved worktree with only RED tests committed as partial WIP | Nonexistent worktree cannot be rescued | Untestable directly (documentation artifact; verified via skill review) |

## Untestable Assertions

Prescriptive TDD WIP-rescue protocol documentation is a documentation-only requirement in `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` and `.agents/skills/lucind-apply/SKILL.md:10-21`. While the underlying fail-closed worktree preservation is tested via unit and CLI tests (`cmd/lucind-ai/cli_test.go:2974-3012`), the human operator workflow cannot be exercised through automated Go tests.

## Open Questions

- [ ] Should CLI warning banners for `lucind-ai accept` and `lucind-ai split` route to `stderr` exclusively or append to `stdout` after records? (`cmd/lucind-ai/cli.go:485-516,685-690`)
- [ ] Should internal teardown callers (`internal/integrate/integrate.go:118-124`, `internal/integrate/candidate.go:262-265`, `cmd/lucind-ai/cli.go:858-869`) pass `force: true` to `worktree.Remove` or use a dedicated helper?

## Citation Manifest

| citation | claim |
|---|---|
| `.agents/skills/lucind-apply/SKILL.md:10-21` | TDD Lifecycle definition in apply lane guide |
| `cmd/lucind-ai/cli.go:485-516` | `runSplit` CLI implementation and wave command emission |
| `cmd/lucind-ai/cli.go:685-690` | `renderAcceptanceReceipt` rendering mechanical acceptance details |
| `cmd/lucind-ai/cli.go:698-726` | `printReport` formatting lane dispatch reports and non-done banner |
| `cmd/lucind-ai/cli.go:730-740` | `printIntegrateReport` printing integrate counts and IDs |
| `cmd/lucind-ai/cli.go:858-869` | `DiscardCombined` and `RemoveLaneWorktree` internal teardown callers |
| `cmd/lucind-ai/cli.go:1934-1978` | `runWorktreeCleanup` command implementation and flag parsing |
| `cmd/lucind-ai/cli_test.go:685-724` | Unit tests for `printReport` diagnosis and non-done banners |
| `cmd/lucind-ai/cli_test.go:729-777` | Unit tests for `printIntegrateReport` ID output |
| `cmd/lucind-ai/cli_test.go:2974-3012` | Unit tests for `worktree cleanup` CLI behavior |
| `cmd/lucind-ai/cli_test.go:4503-4545` | Unit test for mechanical acceptance receipt formatting |
| `internal/accept/accept.go:118-130` | `Accept` validation and acceptance receipt creation |
| `internal/dag/split.go:34-43` | `dag.Split` wave command generation |
| `internal/dag/split_test.go:13-111` | Unit tests for two-wave DAG splitting |
| `internal/integrate/candidate.go:262-265` | `ResolveCandidate` teardown worktree removal |
| `internal/integrate/integrate.go:118-124` | Merge conflict abort and worktree removal in `Combine` |
| `internal/run/integrate.go:159-165` | `RemoveLaneWorktree` caller after lane integration |
| `internal/run/run.go:452-465` | `Report` terminal statuses and diagnosis preservation on timeout |
| `internal/run/run.go:466-475` | Infrastructure error handling for lane dispatches |
| `internal/worktree/worktree.go:150-159` | `pathFor` worktree directory location resolver |
| `internal/worktree/worktree.go:247-261` | `Cleanup` and `Remove` worktree lifecycle functions |
| `internal/worktree/worktree.go:319-325` | `PorcelainEmpty` status check implementation |
| `internal/worktree/worktree_test.go:255-266` | Unit test for `worktree.Remove` |
| `internal/worktree/worktree_test.go:536-595` | Unit test for `worktree.PorcelainEmpty` |
| `internal/worktree/worktree_test.go:1034-1069` | Unit tests for `worktree.Cleanup` behavior |
| `openspec/changes/skill-anchoring-guardrails/proposal.md:69-128` | Proposal Delta Specifications defining the six requirements |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30` | Canonical 10-step acceptance sequence |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30` | Multi-wave sequencing discipline and base SHA refresh |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:33-35` | Bisection recovery and `integrate retry` instructions |
| `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` | Dispatch and integration troubleshooting table |
