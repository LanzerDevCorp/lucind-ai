# Spec Lens C — Live-Spec Conflicts & Migration: Skill Anchoring & Worktree Cleanup Guardrails

## Assumed requirements

This lens evaluates the six candidate requirements defined in the accepted change proposal (`openspec/changes/skill-anchoring-guardrails/proposal.md:69-128`): fail-closed worktree cleanup guardrails with the `--force` flag, blocked and timeout lane report guidance banners, integration report reverted IDs recovery banners, acceptance receipt qualitative review banners, DAG split multi-wave base SHA warning banners, and prescriptive TDD WIP-rescue protocol documentation. It investigates all 24 live specifications under `openspec/specs/` to determine whether worktree cleanup, CLI lifecycle guardrails, or guidance banners are specified under an existing capability name (such as `lane-execution`), or whether the proposal's "Modified Capabilities" classification (`openspec/changes/skill-anchoring-guardrails/proposal.md:29-39`) was a mislabeling of genuinely new capabilities.

## Live Spec Inventory

| Capability | Live spec (file:line) | Requirements | Scenarios | Touched by this change |
|---|---|---|---|---|
| `acceptance-verifier` | `openspec/specs/acceptance-verifier/spec.md:1-121` | 8 | 13 | No |
| `allowed-paths-enforcement` | `openspec/specs/allowed-paths-enforcement/spec.md:1-154` | 8 | 23 | No |
| `apply-dag-dispatch` | `openspec/specs/apply-dag-dispatch/spec.md:1-166` | 11 | 23 | No |
| `completion-mode-enforcement` | `openspec/specs/completion-mode-enforcement/spec.md:1-82` | 5 | 11 | No |
| `conflict-fixture` | `openspec/specs/conflict-fixture/spec.md:1-30` | 1 | 3 | No |
| `conflict-triage` | `openspec/specs/conflict-triage/spec.md:1-29` | 1 | 3 | No |
| `defect-records` | `openspec/specs/defect-records/spec.md:1-71` | 4 | 8 | No |
| `dependencies-defects` | `openspec/specs/dependencies-defects/spec.md:1-29` | 1 | 3 | No |
| `dispatched-packet-body` | `openspec/specs/dispatched-packet-body/spec.md:1-29` | 1 | 3 | No |
| `lane-approval-wait` | `openspec/specs/lane-approval-wait/spec.md:1-61` | 3 | 6 | No |
| `lane-execution` | `openspec/specs/lane-execution/spec.md:1-84` | 4 | 9 | No |
| `lane-progress-telemetry` | `openspec/specs/lane-progress-telemetry/spec.md:1-29` | 1 | 3 | No |
| `orphan-lane-reconciliation` | `openspec/specs/orphan-lane-reconciliation/spec.md:1-23` | 1 | 2 | No |
| `parent-feature-integration` | `openspec/specs/parent-feature-integration/spec.md:1-64` | 4 | 9 | No |
| `read-only-done-criterion` | `openspec/specs/read-only-done-criterion/spec.md:1-58` | 4 | 7 | No |
| `read-only-packet-schema` | `openspec/specs/read-only-packet-schema/spec.md:1-105` | 6 | 14 | No |
| `reconciliation-approval` | `openspec/specs/reconciliation-approval/spec.md:1-89` | 6 | 12 | No |
| `sdd-apply` | `openspec/specs/sdd-apply/spec.md:1-86` | 6 | 12 | No |
| `sdd-planning-fan-out` | `openspec/specs/sdd-planning-fan-out/spec.md:1-107` | 5 | 13 | No |
| `triage-evaluation-rubric` | `openspec/specs/triage-evaluation-rubric/spec.md:1-29` | 1 | 3 | No |
| `ultrafixer-dispatch` | `openspec/specs/ultrafixer-dispatch/spec.md:1-87` | 5 | 11 | No |
| `verify-dual-dispatch` | `openspec/specs/verify-dual-dispatch/spec.md:1-178` | 10 | 19 | No |
| `verify-judgment-packet` | `openspec/specs/verify-judgment-packet/spec.md:1-153` | 7 | 17 | No |
| `verify-mechanical-check` | `openspec/specs/verify-mechanical-check/spec.md:1-116` | 6 | 12 | No |

## Conflicts

None: no live spec covers this behavior, all six requirements are genuinely ADDED against new capability files. A comprehensive audit of all 24 live specifications in `openspec/specs/` confirms that neither worktree cleanup guardrails, CLI failure guidance banners, nor the TDD WIP-rescue protocol are specified in any existing capability. Although the proposal (`openspec/changes/skill-anchoring-guardrails/proposal.md:29-39`) labeled `lane-worktree-lifecycle` and `worktree-cleanup-cli` as Modified Capabilities, neither capability exists in `openspec/specs/`, and candidate capabilities like `lane-execution` (`openspec/specs/lane-execution/spec.md:1-84`) specify only approval-wait gating and metadata persistence without touching worktree cleanup or lifecycle management.

## MODIFIED Full Blocks

None. Neither `lane-worktree-lifecycle` nor `worktree-cleanup-cli` (nor any equivalent live spec covering worktree cleanup guardrails or guidance banners) exists in `openspec/specs/`. Consequently, there are no live specification blocks to reproduce or modify. All requirements for this change must be introduced as ADDED requirements in new capability specs.

## Removals and Renames

| Requirement | Removed or renamed | Reason | Consumers (file:line) | Migration |
|---|---|---|---|---|

None — the proposal names no removals or renames.

## Open Questions

- [ ] Classification correction: The proposal (`openspec/changes/skill-anchoring-guardrails/proposal.md:29-39`) classified `lane-worktree-lifecycle` and `worktree-cleanup-cli` as "Modified Capabilities", but neither capability exists in `openspec/specs/`. The synthesis lane should treat all six requirements as ADDED capabilities (e.g., creating `openspec/specs/worktree-dirty-guardrail/spec.md`, `openspec/specs/failure-guidance-banners/spec.md`, and `openspec/specs/tdd-wip-rescue-protocol/spec.md` or structuring them under clean new capability names) rather than delta specs against non-existent live specs.

## Citation Manifest

| citation | claim |
|---|---|
| `openspec/changes/skill-anchoring-guardrails/proposal.md:29-39` | Proposal Capabilities section naming New and Modified capabilities |
| `openspec/changes/skill-anchoring-guardrails/proposal.md:69-128` | Six delta specification requirements defined in the proposal |
| `openspec/specs/acceptance-verifier/spec.md:1-121` | Live spec defining mechanical acceptance criteria and isolation; untouched by this change |
| `openspec/specs/allowed-paths-enforcement/spec.md:1-154` | Live spec defining scope enforcement and four-way diff union; untouched by this change |
| `openspec/specs/apply-dag-dispatch/spec.md:1-166` | Live spec defining DAG sidecar parsing and wave splitting; untouched by this change |
| `openspec/specs/completion-mode-enforcement/spec.md:1-82` | Live spec defining completion-mode enforcement; untouched by this change |
| `openspec/specs/conflict-fixture/spec.md:1-30` | Live spec defining three-hunk conflict fixture; untouched by this change |
| `openspec/specs/conflict-triage/spec.md:1-29` | Live spec defining advisory conflict triage agent; untouched by this change |
| `openspec/specs/defect-records/spec.md:1-71` | Live spec defining ledger schema v8 defect records; untouched by this change |
| `openspec/specs/dependencies-defects/spec.md:1-29` | Live spec defining ultrafixer defect triage coordination; untouched by this change |
| `openspec/specs/dispatched-packet-body/spec.md:1-29` | Live spec defining HTTP dispatched packet body inspection; untouched by this change |
| `openspec/specs/lane-approval-wait/spec.md:1-61` | Live spec defining lane approval wait gates; untouched by this change |
| `openspec/specs/lane-execution/spec.md:1-84` | Live spec defining approval-wait lifecycle hooks and metadata persistence; untouched by this change |
| `openspec/specs/lane-progress-telemetry/spec.md:1-29` | Live spec defining structured progress telemetry; untouched by this change |
| `openspec/specs/orphan-lane-reconciliation/spec.md:1-23` | Live spec defining orphan lane reconciliation sweep; untouched by this change |
| `openspec/specs/parent-feature-integration/spec.md:1-64` | Live spec defining feature parent lifecycle and serialized CAS; untouched by this change |
| `openspec/specs/read-only-done-criterion/spec.md:1-58` | Live spec defining read-only done-criterion 2; untouched by this change |
| `openspec/specs/read-only-packet-schema/spec.md:1-105` | Live spec defining read_only packet frontmatter schema; untouched by this change |
| `openspec/specs/reconciliation-approval/spec.md:1-89` | Live spec defining reconciliation approval and CAS retry; untouched by this change |
| `openspec/specs/sdd-apply/spec.md:1-86` | Live spec defining packet-based SDD apply workflow; untouched by this change |
| `openspec/specs/sdd-planning-fan-out/spec.md:1-107` | Live spec defining two-wave three-lens planning fan-out; untouched by this change |
| `openspec/specs/triage-evaluation-rubric/spec.md:1-29` | Live spec defining offline dual-judge triage grading; untouched by this change |
| `openspec/specs/ultrafixer-dispatch/spec.md:1-87` | Live spec defining ultrafixer defect triage and repair; untouched by this change |
| `openspec/specs/verify-dual-dispatch/spec.md:1-178` | Live spec defining two-stage verify dual-dispatch workflow; untouched by this change |
| `openspec/specs/verify-judgment-packet/spec.md:1-153` | Live spec defining verify judgment packet schema; untouched by this change |
| `openspec/specs/verify-mechanical-check/spec.md:1-116` | Live spec defining deterministic mechanical checks and log artifacts; untouched by this change |
