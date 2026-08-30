# Spec Lens B — Scenarios & Coverage: Deterministic lucind-ai Orchestrator

## Assumed requirements

This specification establishes deterministic orchestration across Claude Code and OpenCode by introducing `deterministic-orchestrator-contract` and refining four existing capabilities: `packet-authoring-contract`, `sdd-apply`, `parent-feature-integration`, and `acceptance-verifier`. Together, these requirements enforce fail-closed preflight and cross-runtime skill parity, preserve target-free packet template authoring with late target binding, mandate strict wave barrier advancement with DAG target handling, guarantee immutable CAS promotion with no-redispatch retry, and bind acceptance to frozen artifact evidence.

## Scenarios

### Requirement: deterministic-orchestrator-contract

#### Scenario: Preflight verification succeeds across runtimes

- GIVEN identical canonical skill trees in Claude Code and OpenCode and a clean repository root
- WHEN orchestrator preflight executes before dispatch
- THEN preflight exits 0 and allows phase routing and wave planning to proceed

#### Scenario: Concurrent execution in sibling worktree preserves workspace isolation

- GIVEN an active lane execution in a sibling worktree
- WHEN preflight runs in the primary workspace
- THEN fork-local roots, ledgers, and worktrees remain isolated without cross-talk

#### Scenario: Stale skill copy or schema mismatch halts before allocation

- GIVEN OpenCode skill copy differs from canonical Claude skill or binary schema is outdated
- WHEN orchestrator preflight executes
- THEN preflight exits non-zero and halts execution before any worktree allocation

### Requirement: packet-authoring-contract

#### Scenario: Target-free packet template binds feature target at dispatch

- GIVEN a packet template authored without feature target fields
- WHEN the orchestrator supplies feature, parent ref, and base SHA at wave dispatch
- THEN packet parsing and admission bind the target and admit the packet into the wave

#### Scenario: Packet omitting allowed paths defaults to open scope safely

- GIVEN a packet template that omits allowed_paths
- WHEN the packet is parsed and admitted
- THEN AllowedPaths remains empty and diff-boundary scope checks are skipped

#### Scenario: Malformed frontmatter or invalid JSON array fails admission

- GIVEN a packet with invalid frontmatter or a non-array allowed_paths value
- WHEN packet parsing validates the document
- THEN parsing returns a schema error and rejects admission before worktree creation

### Requirement: sdd-apply

#### Scenario: Wave barrier advances to next wave on clean completion

- GIVEN wave N exits 0 with all lanes completed and integrated
- WHEN the orchestrator evaluates the wave barrier
- THEN wave N+1 dispatches using the updated parent state

#### Scenario: Optional sidecar absent falls back to single-wave execution

- GIVEN an SDD change lacking apply-dag.yaml
- WHEN apply is dispatched
- THEN the orchestrator executes packets as a single wave without running split

#### Scenario: Lane failure or unordered path overlap halts subsequent waves

- GIVEN a lane in wave N deviates, fails, or declares unordered overlapping paths
- WHEN wave N completes with a non-zero exit
- THEN the orchestrator halts remaining waves without dispatching wave N+1

### Requirement: parent-feature-integration

#### Scenario: Atomic CAS promotes combined tree to parent ref

- GIVEN a passing wave batch targeting a valid feature parent ref and matching expected SHA
- WHEN integration promotion executes
- THEN CAS updates the feature parent ref without mutator checkout on primary

#### Scenario: Completed terminal attempt returns cached result on retry

- GIVEN an integration attempt that previously reached a terminal status
- WHEN ExecuteAttempt is invoked with the same idempotency key
- THEN the recorded terminal attempt is returned without re-dispatching lanes

#### Scenario: Stale parent SHA or conflicting lease fails CAS safely

- GIVEN the target parent ref SHA advanced concurrently or lease expired
- WHEN CAS promotion executes
- THEN promotion fails with a ref mismatch error and preserves worktrees and ledger evidence

### Requirement: acceptance-verifier

#### Scenario: Valid frozen evidence satisfies acceptance verification

- GIVEN a clean working tree, valid commit, and schema-compliant result envelope
- WHEN acceptance verification evaluates the lane
- THEN the lane status is recorded as done in the ledger

#### Scenario: Read-only packet with clean working tree and no commits passes verification

- GIVEN a packet declared read-only with no unique commits and an empty diff
- WHEN completion mode verification runs
- THEN verification passes and marks the lane done

#### Scenario: Violated hard stop or unapproved scope deviation demotes verdict

- GIVEN a result envelope where a declared hard stop fired or undeclared paths were touched
- WHEN acceptance verification executes
- THEN the lane is demoted to blocked or deviated regardless of green criteria claims

## Coverage

| Requirement | Happy path | Edge case | Error state | Testable through (file:line) |
|---|---|---|---|---|
| `deterministic-orchestrator-contract` | covered | covered | covered | `cmd/lucind-ai/cli.go:791-815` |
| `packet-authoring-contract` | covered | covered | covered | `internal/packet/packet.go:78-120` |
| `sdd-apply` | covered | covered | covered | `internal/dag/waves.go:43-66` |
| `parent-feature-integration` | covered | covered | covered | `internal/run/integrate_feature.go:100-140` |
| `acceptance-verifier` | covered | covered | covered | `internal/run/run.go:603-655` |

## Untestable Assertions

None

## Open Questions

- [ ] None

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:791-815` | CLI validates primary repository root, refuses execution inside linked worktrees, and checks base SHA before feature creation |
| `cmd/lucind-ai/cli.go:946-973` | CLI recovers attempt state idempotently and handles terminal status without duplicate execution |
| `cmd/lucind-ai/cli.go:1466-1500` | Worktree cleanup verifies primary root and idempotently removes lane worktree directories |
| `internal/dag/overlap.go:10-15` | Sentinel error and dependency reachability helpers enforce acyclic path overlap constraints |
| `internal/dag/overlap.go:52-67` | Global overlap validator rejects unordered intersecting `allowed_paths` across DAG packets |
| `internal/dag/waves.go:11-18` | Waves computation groups independent packets into topological execution tiers via Kahn's algorithm |
| `internal/dag/waves.go:43-66` | Iterative wave builder ensures dependency satisfaction and enforces global path disjointness |
| `internal/ledger/ledger.go:1-60` | SQLite ledger initializes WAL mode, busy timeout pragmas, and fail-closed routing constraints |
| `internal/packet/packet.go:22-30` | Sentinel errors reject missing frontmatter, blank IDs, invalid booleans, or malformed JSON path arrays |
| `internal/packet/packet.go:78-120` | Parser extracts frontmatter fields, feature target identities, and completion modes from packet text |
| `internal/result/result.go:20-34` | Result envelope unmarshaler validates schema compliance and mandatory hard stop declarations |
| `internal/run/attempt.go:245-255` | Attempt execution detects existing idempotency keys and returns terminal records without redispatch |
| `internal/run/integrate_feature.go:13-52` | Batch target extractor verifies uniform feature target identity across all wave packets |
| `internal/run/integrate_feature.go:100-140` | Feature integrator executes attempt CAS, promotes parent ref, and demotes/reverts on failure |
| `internal/run/run.go:603-655` | Post-execution diff checker compares staged, unstaged, and untracked changes against declared `allowed_paths` |
| `internal/run/run.go:657-670` | Completion mode enforcer verifies clean git status and commit presence matching packet type |
