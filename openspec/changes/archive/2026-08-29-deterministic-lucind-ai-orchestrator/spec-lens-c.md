# Spec Lens C — Live-Spec Conflicts & Migration: Deterministic lucind-ai Orchestrator

## Assumed requirements

This change establishes deterministic cross-runtime orchestration across Claude Code and OpenCode by introducing three new capabilities (`deterministic-orchestrator-contract`, `packet-authoring-contract`, and `acceptance-verifier`) and modifying two live capabilities (`sdd-apply` and `parent-feature-integration`) as defined in `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:21-28`. For `sdd-apply`, the requirements assert that DAG wave advancement enforces fail-closed wave barriers, per-wave late target binding, and consumer-test ownership while retaining sidecar-free fallback, stdout reporting, and untouched combine/resolve/bisect primitives (`openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:10-13`, `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:14-18`). For `parent-feature-integration`, the requirements assert that late target binding supplies explicit feature targets at wave dispatch before admission, and that recovery remains isolated, immutable, and idempotent without redispatching completed lanes or rewriting parent history (`openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:32-35`, `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:51-53`).

## Live Spec Inventory

| Capability | Live spec (file:line) | Requirements | Scenarios | Touched by this change |
|---|---|---|---|---|
| `parent-feature-integration` | `openspec/specs/parent-feature-integration/spec.md:1-64` | 4 | 9 | `Explicit Feature Target`, `Recoverable Idempotent Attempts` |
| `sdd-apply` | `openspec/specs/sdd-apply/spec.md:1-86` | 6 | 11 | `Apply Authors Packets, Not Primary Diffs`, `Orchestrator Advances Only on a Passing Wave` |

## Conflicts

- **`sdd-apply` — `Orchestrator Advances Only on a Passing Wave` (`openspec/specs/sdd-apply/spec.md:37-50`)**: The live requirement guarantees advancement to wave N+1 based solely on exit code 0 and non-reverted lane status. The change modifies this to require explicit DAG target handling, per-wave late target binding validation, and consumer-test ownership at the wave barrier before advancing. This is a MODIFIED requirement.
- **`sdd-apply` — `Apply Authors Packets, Not Primary Diffs` (`openspec/specs/sdd-apply/spec.md:9-22`)**: The live requirement guarantees packet-based authoring and dispatch via `lucind-ai run`. The change modifies this to require target-free template authoring with late target binding and DAG target handling. This is a MODIFIED requirement.
- **`parent-feature-integration` — `Explicit Feature Target` (`openspec/specs/parent-feature-integration/spec.md:5-18`)**: The live requirement guarantees that every dispatchable unit identifies all four target fields at admission and fails closed on implicit primary state. The change modifies this to clarify that authoring produces target-free templates and that late target binding supplies explicit target parameters per wave at dispatch before admission. This is a MODIFIED requirement.
- **`parent-feature-integration` — `Recoverable Idempotent Attempts` (`openspec/specs/parent-feature-integration/spec.md:47-64`)**: The live requirement guarantees idempotent retry without secondary promotion and verification of recorded refs. The change modifies this to mandate isolated per-fork recovery boundaries and truthful ledger projections while strictly prohibiting redispatch of completed lanes. This is a MODIFIED requirement.

No other live requirements are conflicted or contradicted:
- `Managed Parent Lifecycle` (`openspec/specs/parent-feature-integration/spec.md:19-32`) and `Immutable Starts and Serialized Promotion` (`openspec/specs/parent-feature-integration/spec.md:33-46`) are preserved unchanged, retaining CAS promotion and external feature closure (`openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:14-18`, `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:21-28`).
- `An Absent Sidecar Preserves Hand-Split Apply` (`openspec/specs/sdd-apply/spec.md:23-36`), `Orchestrator Reads Stdout, Not a New Report Format` (`openspec/specs/sdd-apply/spec.md:51-64`), `Combine, Resolve, and Bisect Stay Untouched` (`openspec/specs/sdd-apply/spec.md:65-73`), and `Additive Rollback, No Ledger Migration` (`openspec/specs/sdd-apply/spec.md:74-86`) remain untouched and preserved (`openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:14-18`, `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:32-35`, `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:51-53`).

## MODIFIED Full Blocks

### Requirement: Explicit Feature Target

**Source**: `openspec/specs/parent-feature-integration/spec.md:5` — 2 scenarios

Every dispatchable unit SHALL identify its feature, parent ref, immutable base SHA, and expected parent SHA. Lucind MUST NOT infer any target from the primary checkout. Legacy packets and current single-`main` flows MUST fail closed unless an explicit legacy mode declares `main` and its expected SHA.

#### Scenario: Explicit target accepted
- GIVEN a unit with all four target fields
- WHEN it is admitted
- THEN its recorded parent ref and SHAs SHALL be used unchanged

#### Scenario: Missing or implicit target rejected
- GIVEN a unit omits target data or relies on checkout state
- WHEN it is admitted without explicit legacy mode
- THEN admission MUST fail before worktree creation or ref mutation

### Requirement: Recoverable Idempotent Attempts

**Source**: `openspec/specs/parent-feature-integration/spec.md:47` — 3 scenarios

Each attempt SHALL have a durable identity and recorded inputs. A retry MUST return its terminal result or resume from those inputs without a second promotion. After interruption or lease expiry, recovery MUST verify recorded expected and current refs before resuming; unsafe recovery SHALL fail closed while preserving evidence and worktrees.

#### Scenario: Completed attempt is retried
- GIVEN an attempt already reached a terminal result
- WHEN its identity is replayed
- THEN the same result SHALL be returned without another ref update

#### Scenario: Expired lease is recovered
- GIVEN an interrupted attempt whose lease expired
- WHEN recovery verifies unchanged expected and current refs
- THEN it MAY reacquire the lease and resume from recorded inputs

#### Scenario: Recovery finds changed state
- GIVEN an interrupted attempt whose recorded refs no longer match
- WHEN recovery runs
- THEN it MUST remain blocked and preserve diagnostic artifacts

### Requirement: Apply Authors Packets, Not Primary Diffs

**Source**: `openspec/specs/sdd-apply/spec.md:9` — 2 scenarios

After this change, `sdd-apply` MUST implement an SDD apply by authoring packet files and dispatching them through `lucind-ai run`. It MUST NOT write the apply diff itself via in-session Read/Edit/Write against the primary repository. (Design Decision 1, Decision 3; proposal: Impact on the existing `sdd-apply` flow.)

#### Scenario: Apply is a DAG of lucind-ai run packets
- GIVEN an SDD change whose tasks have been split into packets
- WHEN apply runs
- THEN each packet MUST execute as a real lane (worktree, envelope, barrier) via `lucind-ai run`, not as an in-process edit on primary

#### Scenario: Primary is not the apply session's write target
- GIVEN `sdd-apply` is performing this change's apply path
- WHEN a task's code is written
- THEN the write MUST occur in the lane worktree `lucind-ai run` created, not in the orchestrator's primary checkout

### Requirement: Orchestrator Advances Only on a Passing Wave

**Source**: `openspec/specs/sdd-apply/spec.md:37` — 2 scenarios

The orchestrator MUST advance to wave N+1 only when wave N's `lucind-ai run` exits 0 — meaning every lane is `done` and none were reverted. On a non-zero exit the orchestrator MUST halt the remaining DAG for human review or replanning, not attempt to skip ahead. (Design Decision 3, Decision 4.)

#### Scenario: Passed wave advances
- GIVEN wave N's stdout reports `passed=true` and the process exits 0
- WHEN the orchestrator considers wave N+1
- THEN it MUST dispatch the next printed `lucind-ai run` command

#### Scenario: Reverted or blocked wave stops the DAG
- GIVEN wave N exits non-zero because a lane is `blocked`, `deviated`, `failed`, or listed in `reverted_ids`
- WHEN the orchestrator considers further waves
- THEN it MUST NOT dispatch any of them

## Removals and Renames

| Requirement | Removed or renamed | Reason | Consumers (file:line) | Migration |
|---|---|---|---|---|
| None | None | No live requirements are removed or renamed by this change; existing capabilities and requirements are retained or extended. | None | None |

## Open Questions

- [ ] None

## Citation Manifest

| citation | claim |
|---|---|
| `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:10-13` | Scope definition for deterministic preflight, late target binding, consumer tests, and recovery |
| `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:14-18` | Out of scope commitments preserving existing states, schedulers, and Combine/Resolve/CAS primitives |
| `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:21-28` | Capabilities classification defining new capabilities and listing sdd-apply and parent-feature-integration as modified |
| `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:32-35` | Approach defining prompt/reference layer and runtime enforcement boundaries |
| `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:51-53` | Rollback plan establishing independent reversibility without evidence or ledger rewrites |
| `openspec/specs/parent-feature-integration/spec.md:1` | Opening line of parent-feature-integration spec establishing file existence and inventory base |
| `openspec/specs/parent-feature-integration/spec.md:1-64` | Full file range of parent-feature-integration spec confirming requirement and scenario counts |
| `openspec/specs/parent-feature-integration/spec.md:5` | Header for Explicit Feature Target requirement |
| `openspec/specs/parent-feature-integration/spec.md:5-18` | Full block for Explicit Feature Target requirement and 2 scenarios |
| `openspec/specs/parent-feature-integration/spec.md:19-32` | Full block for Managed Parent Lifecycle requirement and 2 scenarios |
| `openspec/specs/parent-feature-integration/spec.md:33-46` | Full block for Immutable Starts and Serialized Promotion requirement and 2 scenarios |
| `openspec/specs/parent-feature-integration/spec.md:47` | Header for Recoverable Idempotent Attempts requirement |
| `openspec/specs/parent-feature-integration/spec.md:47-64` | Full block for Recoverable Idempotent Attempts requirement and 3 scenarios |
| `openspec/specs/sdd-apply/spec.md:1` | Opening line of sdd-apply spec establishing file existence and inventory base |
| `openspec/specs/sdd-apply/spec.md:1-86` | Full file range of sdd-apply spec confirming requirement and scenario counts |
| `openspec/specs/sdd-apply/spec.md:9` | Header for Apply Authors Packets, Not Primary Diffs requirement |
| `openspec/specs/sdd-apply/spec.md:9-22` | Full block for Apply Authors Packets, Not Primary Diffs requirement and 2 scenarios |
| `openspec/specs/sdd-apply/spec.md:23-36` | Full block for An Absent Sidecar Preserves Hand-Split Apply requirement and 2 scenarios |
| `openspec/specs/sdd-apply/spec.md:37` | Header for Orchestrator Advances Only on a Passing Wave requirement |
| `openspec/specs/sdd-apply/spec.md:37-50` | Full block for Orchestrator Advances Only on a Passing Wave requirement and 2 scenarios |
| `openspec/specs/sdd-apply/spec.md:51-64` | Full block for Orchestrator Reads Stdout, Not a New Report Format requirement and 2 scenarios |
| `openspec/specs/sdd-apply/spec.md:65-73` | Full block for Combine, Resolve, and Bisect Stay Untouched requirement and 1 scenario |
| `openspec/specs/sdd-apply/spec.md:74-86` | Full block for Additive Rollback, No Ledger Migration requirement and 2 scenarios |
