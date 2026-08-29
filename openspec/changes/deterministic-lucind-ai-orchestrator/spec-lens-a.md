# Spec Lens A — Capabilities & Requirements: Deterministic lucind-ai Orchestrator

## Assumed requirements

Per the accepted proposal (`openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:21-28`), this specification defines five capabilities across the deterministic orchestration workflow: three New capabilities (`deterministic-orchestrator-contract`, `packet-authoring-contract`, and `acceptance-verifier`) and two Modified capabilities (`sdd-apply` and `parent-feature-integration`). Each capability introduces or updates exactly one requirement statement (totaling three ADDED and two MODIFIED requirements). Together, they establish cross-runtime preflight parity, dynamic wave target binding, explicit DAG barriers with consumer-test ownership, immutable CAS promotion with no-redispatch idempotency, and frozen evidence acceptance verification.

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|
| `deterministic-orchestrator-contract` | New | `openspec/specs/deterministic-orchestrator-contract/spec.md` | — |
| `packet-authoring-contract` | New | `openspec/specs/packet-authoring-contract/spec.md` | — |
| `acceptance-verifier` | New | `openspec/specs/acceptance-verifier/spec.md` | — |
| `sdd-apply` | Existing | `openspec/changes/deterministic-lucind-ai-orchestrator/specs/sdd-apply/spec.md` | `openspec/specs/sdd-apply/spec.md:1-86` |
| `parent-feature-integration` | Existing | `openspec/changes/deterministic-lucind-ai-orchestrator/specs/parent-feature-integration/spec.md` | `openspec/specs/parent-feature-integration/spec.md:1-64` |

## ADDED Requirements

### Requirement: Cross-Runtime Orchestrator Preflight and Sequencing

The orchestrator MUST execute deterministic preflight verification across Claude Code and OpenCode, enforce fail-closed phase routing and wave planning, preserve workspace isolation for concurrent sibling worktrees, and halt execution before allocating worktrees if skill parity or schema checks fail.

**Terminal consumer**: `cmd/lucind-ai/cli.go:791-815`

### Requirement: Target-Free Packet Authoring and Late Binding

Packet templates MUST be authorable without hardcoded feature targets and SHALL bind feature identity, parent ref, and base SHA dynamically at wave dispatch, while packets omitting `allowed_paths` MUST default to open scope without triggering diff-boundary validation failures.

**Terminal consumer**: `internal/packet/packet.go:78-120`

### Requirement: Frozen Evidence Acceptance Verification

Acceptance verification MUST evaluate immutable candidate commits, tree hashes, schema-compliant result envelopes, and clean worktree status, demoting any lane that violated hard stops or undeclared path boundaries regardless of claimed criteria.

**Terminal consumer**: `internal/run/run.go:603-670`

## MODIFIED Requirements

### Requirement: Orchestrator Advances Only on a Passing Wave

The orchestrator MUST advance to wave N+1 only when wave N's `lucind-ai run` exits 0 with all lanes completed and integrated without path overlap conflicts, binding target parent state deterministically per wave and halting remaining waves on any lane failure or reversion.
(Previously: The orchestrator MUST advance to wave N+1 only when wave N's `lucind-ai run` exits 0 — meaning every lane is `done` and none were reverted.)

**Live block**: `openspec/specs/sdd-apply/spec.md:37-50`, 2 scenarios
**Terminal consumer**: `internal/dag/waves.go:43-66`

### Requirement: Recoverable Idempotent Attempts

Each integration attempt SHALL maintain an immutable idempotency key and recorded inputs such that retries MUST return the recorded terminal result without re-dispatching lanes, and CAS promotion failure due to stale parent SHAs MUST preserve all worktrees and ledger evidence.
(Previously: Each attempt SHALL have a durable identity and recorded inputs, returning terminal results on retry or resuming without second promotion.)

**Live block**: `openspec/specs/parent-feature-integration/spec.md:47-64`, 3 scenarios
**Terminal consumer**: `internal/run/integrate_feature.go:100-140`

## Open Questions

- [ ] None

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:791-815` | CLI validates primary repository root, refuses execution inside linked worktrees, and checks base SHA before feature creation |
| `internal/dag/waves.go:43-66` | Iterative wave builder ensures dependency satisfaction and enforces global path disjointness |
| `internal/packet/packet.go:78-120` | Parser extracts frontmatter fields, feature target identities, and completion modes from packet text |
| `internal/run/integrate_feature.go:100-140` | Feature integrator executes attempt CAS, promotes parent ref, and demotes/reverts on failure |
| `internal/run/run.go:603-670` | Post-execution diff checker and completion mode enforcer verify allowed paths and clean git status |
| `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:21-28` | Proposal Capabilities section defines three New capabilities and two Modified capabilities |
| `openspec/specs/parent-feature-integration/spec.md:1-64` | parent-feature-integration live spec confirms Existing/Modified capability status |
| `openspec/specs/parent-feature-integration/spec.md:47-64` | Recoverable Idempotent Attempts live requirement block containing 3 scenarios |
| `openspec/specs/sdd-apply/spec.md:1-86` | sdd-apply live spec confirms Existing/Modified capability status |
| `openspec/specs/sdd-apply/spec.md:37-50` | Orchestrator Advances Only on a Passing Wave live requirement block containing 2 scenarios |
