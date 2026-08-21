# Proposal Lens B — Capabilities & Surface: sdd-fan-out-lens

## Assumed scope

Candidate 1 — Null option: convention and template hardening only (no Go binary changes), folding missing frontmatter keys, CLI invocation subcommands, and feature branch ownership into `SKILL.md` and `assets/` templates.

## Capabilities

### New Capabilities

- None. Candidate 1 adds no new Go binary capabilities or OpenSpec specs; fan-out uses orchestrator prompt conventions, templates, and existing generic `--packet` execution.

### Modified Capabilities

- None. Candidate 1 modifies no existing Go binary capabilities or OpenSpec specs; existing requirements are honored unchanged.

## Accepted specifications in scope

| Spec requirement | Disposition | Citation |
|---|---|---|
| `read-only-packet-schema`: Frontmatter Read-Only Field Parsing | Honored unchanged | `openspec/specs/read-only-packet-schema/spec.md:9-26` |
| `read-only-packet-schema`: Default Value and Backward Compatibility | Honored unchanged | `openspec/specs/read-only-packet-schema/spec.md:28-45` |
| `read-only-packet-schema`: Explicit Flag Only — No Inference | Honored unchanged | `openspec/specs/read-only-packet-schema/spec.md:47-59` |
| `read-only-packet-schema`: The Envelope Cannot Declare or Override Mode | Honored unchanged | `openspec/specs/read-only-packet-schema/spec.md:61-73` |
| `read-only-packet-schema`: Additive Rollback | Honored unchanged | `openspec/specs/read-only-packet-schema/spec.md:75-80` |
| `read-only-done-criterion`: Write Packets Keep Criterion 2 Unchanged | Honored unchanged | `openspec/specs/read-only-done-criterion/spec.md:9-16` |
| `read-only-done-criterion`: Read-Only Packets Replace Criterion 2 | Honored unchanged | `openspec/specs/read-only-done-criterion/spec.md:18-30` |
| `read-only-done-criterion`: The Protocol Envelope Is Not a Mutation | Honored unchanged | `openspec/specs/read-only-done-criterion/spec.md:32-39` |
| `read-only-done-criterion`: Authoring Assets Document the Exception | Honored unchanged | `openspec/specs/read-only-done-criterion/spec.md:41-47` |
| `allowed-paths-enforcement`: Packet AllowedPaths Field | Honored unchanged | `openspec/specs/allowed-paths-enforcement/spec.md:9-26` |
| `allowed-paths-enforcement`: Omitting AllowedPaths Preserves Today's Exact Path | Honored unchanged | `openspec/specs/allowed-paths-enforcement/spec.md:28-40` |
| `allowed-paths-enforcement`: Upfront Batch Disjointness Check | Honored unchanged | `openspec/specs/allowed-paths-enforcement/spec.md:42-59` |
| `allowed-paths-enforcement`: Base-SHA Four-Way Diff Union Defines "Actual Diff" | Honored unchanged | `openspec/specs/allowed-paths-enforcement/spec.md:61-93` |
| `allowed-paths-enforcement`: Post-Execution Scope Check Demotes Done to Deviated | Honored unchanged | `openspec/specs/allowed-paths-enforcement/spec.md:95-117` |
| `allowed-paths-enforcement`: Blocked and Failed Are Never Rewritten to Deviated | Honored unchanged | `openspec/specs/allowed-paths-enforcement/spec.md:119-131` |
| `allowed-paths-enforcement`: .lucind/ Is Always Excluded From Scope Comparison | Honored unchanged | `openspec/specs/allowed-paths-enforcement/spec.md:133-140` |
| `allowed-paths-enforcement`: Git Inspection Failure Blocks, Never Guesses | Honored unchanged | `openspec/specs/allowed-paths-enforcement/spec.md:142-151` |
| `completion-mode-enforcement`: Post-Status Git Verification, Not Envelope Trust | Honored unchanged | `openspec/specs/completion-mode-enforcement/spec.md:9-26` |
| `completion-mode-enforcement`: Write Packet Completion Matrix | Honored unchanged | `openspec/specs/completion-mode-enforcement/spec.md:28-45` |
| `completion-mode-enforcement`: Read-Only Packet Completion Matrix | Honored unchanged | `openspec/specs/completion-mode-enforcement/spec.md:47-64` |
| `completion-mode-enforcement`: Git Inspection Failure Resolves to Failed, Not Blocked | Honored unchanged | `openspec/specs/completion-mode-enforcement/spec.md:66-73` |
| `completion-mode-enforcement`: Combine Stays Unaware of Read-Only Lanes | Honored unchanged | `openspec/specs/completion-mode-enforcement/spec.md:75-81` |
| `apply-dag-dispatch`: Sidecar DAG Artifact | Honored unchanged | `openspec/specs/apply-dag-dispatch/spec.md:9-26` |
| `apply-dag-dispatch`: Split Is the Mechanical Consumer | Honored unchanged | `openspec/specs/apply-dag-dispatch/spec.md:28-40` |
| `apply-dag-dispatch`: Non-Empty Allowed Paths at Split | Honored unchanged | `openspec/specs/apply-dag-dispatch/spec.md:51-58` |
| `apply-dag-dispatch`: Sequential Run Per Wave | Honored unchanged | `openspec/specs/apply-dag-dispatch/spec.md:117-134` |
| `parent-feature-integration`: Explicit Feature Target | Honored unchanged | `openspec/specs/parent-feature-integration/spec.md:5-17` |
| `parent-feature-integration`: Managed Parent Lifecycle | Honored unchanged | `openspec/specs/parent-feature-integration/spec.md:19-31` |
| `parent-feature-integration`: Immutable Starts and Serialized Promotion | Honored unchanged | `openspec/specs/parent-feature-integration/spec.md:33-45` |
| `parent-feature-integration`: Recoverable Idempotent Attempts | Honored unchanged | `openspec/specs/parent-feature-integration/spec.md:47-58` |

## Affected Areas

| Area | Why it is touched |
|---|---|
| `plugin/claude-code/skills/lucind-ai/SKILL.md` | Document frontmatter keys (`feature`, `parent_ref`, `base_sha`, `expected_parent_sha`, `legacy_main`), CLI subcommands (`serve`, `feature`, `reconcile`), CLI flags (`--approval-timeout`, `--legacy-main`, `--expected-parent-sha`), feature branch ownership, and three-lens fan-out protocol. |
| `plugin/claude-code/skills/lucind-ai/assets/` | Update packet templates with required frontmatter keys, disjoint path bounds, word budgets, and done-criteria structures. |
| `openspec/changes/sdd-fan-out-lens/` | Record SDD artifacts for this change. |

## Dependencies

- Multi-lane batch execution and barrier join (`cmd/lucind-ai/cli.go:121-149`, `internal/run/batch.go:66-95`).
- Multi-lane worktree isolation and ledger status tracking (`internal/worktree/worktree.go:168-237`, `internal/run/run.go:451-454`).
- Read-only packet parsing and completion mode verification (`internal/packet/packet.go:105-113`, `internal/run/run.go:654-662`).
- Batch disjointness check and post-execution diff scope check (`internal/packet/disjoint.go:24-48`, `internal/run/run.go:590-626`).
- Result envelope validation against `.lucind/result.schema.json` (`internal/run/run.go:515-543`).
- Parent feature branch admission and promotion support (`internal/packet/packet.go:71-78,122-130`, `cmd/lucind-ai/cli.go:664-890`).

## Prose-only elements: moved or left

| Element | This change |
|---|---|
| Any known executor per packet | Deliberately left in prose (`plugin/claude-code/skills/lucind-ai/SKILL.md:143-148` executor assignment conventions and templates). |
| Concurrent batch + integrate | Deliberately left in prose (`plugin/claude-code/skills/lucind-ai/SKILL.md:153-176` two-invocation wave sequencing via CLI). |
| `run --packet` without a sidecar | Deliberately left in prose (`plugin/claude-code/skills/lucind-ai/SKILL.md:153-155` hand-authored packets under `.lucind/packets/`). |
| Path disjointness + post-diff scope | Deliberately left in prose (`plugin/claude-code/skills/lucind-ai/SKILL.md:143-148` slice ownership and reading lists). |
| Worktree isolation + ledger status | Deliberately left in prose (packet ID conventions in templates). |
| Envelope schema | Deliberately left in prose (findings format in templates; validated by schema). |
| Assumed architecture, word budgets, compression gap, skill/packet precedence, citation pass, 8-item spine, notes 4-section shape | Deliberately left in prose (`plugin/claude-code/skills/lucind-ai/SKILL.md:188-227` and `plugin/claude-code/skills/lucind-ai/assets/design-synthesis-packet-template.md:70-119,147-150`). |

## Open Questions

- [ ] None
