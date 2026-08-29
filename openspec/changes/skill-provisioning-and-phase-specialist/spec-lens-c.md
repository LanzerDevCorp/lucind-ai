# Spec Lens C — Live-Spec Conflicts & Migration: Skill Provisioning and the SDD Phase Specialist

## Assumed requirements

The change introduces deterministic three-tier skill provisioning and an SDD phase specialist through six assumed delta requirements: (1) `Deterministic multi-tier derivation` computes required skills from `(sdd_phase, lane_role)` unioned with stack (`lucind.yaml`) and ad-hoc tiers under an admission budget default of 3; (2) `Root resolution and fail-closed admission` resolves skills to filesystem `SKILL.md` paths via `.lucind/skill-roots.yaml` with tilde expansion, failing admission if missing; (3) `Contract extension and rendered delivery` passes `lane_role` and `required_skills` inside `Contract json.RawMessage` and delivers them via `## Required skills` body rendering and `LUCIND_REQUIRED_SKILLS` environment variables while preserving `lane-authoring-evidence/v1`; (4) `Closed-set lane_role` validates `lane_role` against `{lens, synthesis, apply, verify, archive, ultrafixer, human}` and validates `sdd_phase` when `lane_role` is present; (5) `Demotion and acceptance correspondence` adds `skills_loaded` to result envelopes, demotes runtime shortfalls to `lane.Deviated`, and enforces mechanical acceptance rejection on missing skills; and (6) `Specialist sequencing` drives fan-out planning before synthesis using non-intercepting `gentle-ai sdd-status` orchestration.

## Live Spec Inventory

| Capability | Live spec (file:line) | Requirements | Scenarios | Touched by this change |
|---|---|---|---|---|
| `packet-authoring-contract` | `openspec/specs/packet-authoring-contract/spec.md:1` | 4 | 10 | Yes (3 MODIFIED) |
| `lane-execution` | `openspec/specs/lane-execution/spec.md:1` | 6 | 15 | Yes (2 MODIFIED) |
| `acceptance-verifier` | `openspec/specs/acceptance-verifier/spec.md:1` | 8 | 17 | Yes (2 MODIFIED) |
| `read-only-packet-schema` | `openspec/specs/read-only-packet-schema/spec.md:1` | 8 | 19 | Yes (1 MODIFIED) |

## Conflicts

- **`Extended packet frontmatter parsing` (`openspec/specs/read-only-packet-schema/spec.md:84`)**: The live spec notes that exact key names for phase and skill remain open design questions and treats keys as unvalidated strings. This change resolves the open question by defining `lane_role` with closed-set validation (`{lens, synthesis, apply, verify, archive, ultrafixer, human}`) and validating `sdd_phase` when `lane_role` is present. An unrecognized `lane_role` value fails parsing with a validation error rather than being accepted as an arbitrary string. Unrecognized other keys remain ignored per `openspec/specs/read-only-packet-schema/spec.md:28`.
- **`Universal Pre-Dispatch Packet Admission` (`openspec/specs/lane-execution/spec.md:85`) and `Universal Admission and Manual Compatibility` (`openspec/specs/packet-authoring-contract/spec.md:42`)**: Live specs validate targets, paths, routes, modes, and result obligations. This change extends pre-dispatch admission to fail-closed reject batches if any required skill cannot be resolved to a readable `SKILL.md` path or if the derived skill set exceeds the budget (default 3).
- **`Versioned Result Correspondence` (`openspec/specs/packet-authoring-contract/spec.md:61`) and `Fail-Closed Mechanical Criteria` (`openspec/specs/acceptance-verifier/spec.md:30`)**: Live specs verify criteria, hard stops, mode, commit obligations, and canonical file changes. This change extends correspondence and acceptance to enforce that every frozen `required_skills` entry is present in the envelope's `skills_loaded` declaration. Shortfalls trigger `lane.Deviated` during runtime enforcement and fail mechanical acceptance without a receipt.
- **Refutation of false collision candidates**: `Frozen Candidate Verification` (`openspec/specs/acceptance-verifier/spec.md:60`) was evaluated as a plausible collision candidate due to its name, but inspection proves it strictly governs evaluating the candidate commit/tree over live checkout changes; it is untouched.

## MODIFIED Full Blocks

### Requirement: Versioned Contract and Late Target Binding

**Source**: `openspec/specs/packet-authoring-contract/spec.md:9` — 3 scenarios

An authored contract MUST declare its contract version, route intent, execution mode, write paths, read-only input paths, goal, ordered done criteria, ordered hard stops, and result obligations. It MUST NOT contain live feature, parent, base, expected-parent, or commit values. Compilation MUST accept exactly one validated typed binding: feature target or legacy-main target.

#### Scenario: Compile with a feature binding
- GIVEN a valid target-free contract and a valid feature-target binding
- WHEN compilation runs
- THEN the artifact MUST contain the bound target values and identify the contract version

#### Scenario: Reject authored target authority
- GIVEN specialist or versioned manual contract data containing a live target SHA
- WHEN contract validation runs
- THEN validation MUST fail with a diagnostic identifying the forbidden field

#### Scenario: Reject a stale binding
- GIVEN a binding whose expected parent no longer matches the live parent
- WHEN dispatch admission validates the binding
- THEN admission MUST fail before worktree or quota allocation

### Requirement: Universal Admission and Manual Compatibility

**Source**: `openspec/specs/packet-authoring-contract/spec.md:42` — 3 scenarios

Every compiled or manual packet MUST pass admission before dispatch. Admission MUST reject missing or contradictory result-path, result-schema, route, mode, target, or path obligations with actionable diagnostics. An admitted unversioned manual packet MUST remain in legacy compatibility mode, MUST retain its dispatch body bytes unchanged, and MUST NOT acquire strict versioned correspondence retroactively.

#### Scenario: Safe legacy manual packet
- GIVEN an unversioned manual packet satisfying universal safety checks
- WHEN it is admitted
- THEN its body MUST remain byte-identical and it MUST dispatch in compatibility mode

#### Scenario: Unsafe legacy manual packet
- GIVEN a manual packet omitting `.lucind/result.json` delivery or schema validation
- WHEN admission runs
- THEN it MUST fail before dispatch with the missing obligation identified

#### Scenario: Contradictory mode
- GIVEN packet metadata declares read-only while its body requires a commit
- WHEN admission runs
- THEN it MUST reject the packet with both contradictory declarations identified

### Requirement: Versioned Result Correspondence

**Source**: `openspec/specs/packet-authoring-contract/spec.md:61` — 2 scenarios

For a versioned artifact, the system MUST freeze the normalized contract and require the result to correspond exactly to its packet identity, ordered criteria, ordered hard stops, mode, commit obligation, and canonical changed paths. Missing, extra, duplicate, or altered criteria or stops MUST fail correspondence. Write results MUST name the frozen candidate commit; read-only results MUST omit commit and report no canonical changes.

#### Scenario: Exact versioned result
- GIVEN a versioned write result matching all frozen declarations and candidate facts
- WHEN correspondence is checked
- THEN the result MUST be eligible for mechanical acceptance

#### Scenario: Omitted or extra declaration
- GIVEN a versioned result omits, duplicates, alters, or adds a criterion or hard stop
- WHEN correspondence is checked
- THEN correspondence MUST fail and no acceptance receipt may be created

### Requirement: Universal Pre-Dispatch Packet Admission

**Source**: `openspec/specs/lane-execution/spec.md:85` — 3 scenarios

Dispatch MUST apply universal packet admission to manual and compiled packets before `ExecuteBatch`, worktree creation, or quota allocation. Admission MUST validate result delivery and schema obligations, route and mode consistency, path declarations, target completeness, and live target freshness. A rejected packet MUST produce field-specific diagnostics and MUST NOT partially dispatch its batch.

#### Scenario: Safe mixed batch
- GIVEN a batch of safe manual and compiled packets
- WHEN dispatch admission completes
- THEN the batch MAY proceed to execution without rewriting admitted manual bodies

#### Scenario: Unsafe packet blocks allocation
- GIVEN one packet lacks a result obligation or contradicts its declared mode
- WHEN the batch is admitted
- THEN the batch MUST fail before any worktree or quota allocation and identify the violation

#### Scenario: Target becomes stale
- GIVEN a compiled packet binding was valid when authored but its expected parent is stale at admission
- WHEN dispatch starts
- THEN the batch MUST fail before `ExecuteBatch` and report the stale target

### Requirement: Frozen Authored Candidate Evidence

**Source**: `openspec/specs/lane-execution/spec.md:104` — 3 scenarios

Before executor work can become a lane candidate, lane execution MUST freeze the exact admitted packet identity and digest, contract version or explicit legacy mode, normalized versioned contract evidence when present, typed target binding, execution mode, write paths, read-only paths, and result obligations. Later packet, target, or checkout changes MUST NOT alter this evidence.

#### Scenario: Versioned candidate freezes correspondence evidence
- GIVEN a compiled packet passes admission
- WHEN its candidate evidence is recorded
- THEN Acceptance MUST be able to recover all declarations needed for independent correspondence checks

#### Scenario: Source packet changes later
- GIVEN frozen candidate evidence and a packet file edited after dispatch
- WHEN result or Acceptance verification runs
- THEN verification MUST use the frozen evidence rather than the edited file

#### Scenario: Legacy packet is explicit
- GIVEN an admitted unversioned manual packet
- WHEN candidate evidence is frozen
- THEN it MUST be marked legacy and MUST NOT be mistaken for a versioned contract

### Requirement: Exact Acceptance Binding

**Source**: `openspec/specs/acceptance-verifier/spec.md:9` — 3 scenarios

Every decision and receipt MUST immutably bind the lane, packet, base commit and tree, candidate commit and tree, allowed paths, check policy, relevant environment identity, authoring mode, and authored evidence identity. For a versioned contract, the binding MUST include the contract version and immutable normalized evidence sufficient to verify criteria, hard stops, execution mode, commit obligation, read-only paths, and canonical changed-path semantics.
(Previously: The binding covered packet and candidate identity, allowed paths, policy, and environment but not normalized authored-contract correspondence.)

#### Scenario: Record the complete binding
- GIVEN a candidate with every required identity and authoring value
- WHEN mechanical acceptance succeeds
- THEN the receipt MUST contain the exact complete binding
- AND none of its bound values can be changed

#### Scenario: Reject an identity mismatch
- GIVEN the packet, contract evidence, commit, tree, policy, or environment differs from the requested binding
- WHEN acceptance is attempted
- THEN acceptance MUST fail and no receipt exists

#### Scenario: Stale authored evidence cannot be substituted
- GIVEN a frozen candidate whose source contract or target changes later
- WHEN acceptance is attempted
- THEN Acceptance MUST use the frozen evidence and MUST reject any substituted digest or normalized contract

### Requirement: Fail-Closed Mechanical Criteria

**Source**: `openspec/specs/acceptance-verifier/spec.md:30` — 5 scenarios

The verifier MUST reject a missing or invalid result schema, packet or candidate-commit mismatch, fired hard stop, unmet done criterion, undeclared or out-of-scope change, or failed required check. For versioned contracts it MUST also reject any missing, extra, duplicate, reordered, or altered authored criterion or hard stop; mode or commit disagreement; and any path or change-classification mismatch against the canonical frozen candidate change set. A rejected attempt MUST NOT create or reuse a receipt.
(Previously: Result validity checked reported criteria, stops, and changed paths but could not prove exact correspondence to frozen authored declarations or commit and classification semantics.)

#### Scenario: Reject invalid result evidence
- GIVEN result evidence is missing, schema-invalid, mismatched, has a fired hard stop, or has an unmet done criterion
- WHEN acceptance is attempted
- THEN acceptance MUST fail and no receipt exists

#### Scenario: Reject scope or check failure
- GIVEN the candidate contains an undeclared or disallowed change, or a required check fails
- WHEN acceptance is attempted
- THEN acceptance MUST fail and no receipt exists

#### Scenario: Reject authored-result mismatch
- GIVEN a versioned result omits or changes an authored criterion or stop
- WHEN acceptance is attempted
- THEN acceptance MUST fail even when every reported entry is green

#### Scenario: Reject commit or path-class mismatch
- GIVEN a write result names another commit or misclassifies a deletion or rename endpoint
- WHEN acceptance compares it with the frozen candidate
- THEN acceptance MUST fail and no receipt exists

#### Scenario: Preserve explicit legacy behavior
- GIVEN an admitted manual candidate is explicitly marked legacy
- WHEN acceptance runs
- THEN universal schema, scope, commit-state, and check rules MUST apply without inventing versioned declaration correspondence

### Requirement: Extended packet frontmatter parsing

**Source**: `openspec/specs/read-only-packet-schema/spec.md:84` — 3 scenarios

Packet parsing MUST accept optional SDD-phase, fanout-group, and skill frontmatter keys (exact key names remain an open design question) and MUST map present values onto the corresponding packet fields. Omitted keys MUST default to empty strings. Absence of these keys MUST NOT fail parsing. Live executor runtime skill telemetry MUST NOT be decoded from packet frontmatter.

#### Scenario: Parse frontmatter keys

- GIVEN a packet markdown document that declares SDD-phase, fanout-group, and skill values
- WHEN the packet is parsed
- THEN the returned packet MUST carry those declared values

#### Scenario: Optional keys omitted

- GIVEN a packet markdown document that omits SDD-phase, fanout-group, and skill keys
- WHEN the packet is parsed
- THEN parsing MUST succeed with empty values for those fields

#### Scenario: Empty frontmatter values handled

- GIVEN a packet document that includes those keys with empty values
- WHEN the packet is parsed
- THEN parsing MUST succeed and assign empty strings to the corresponding fields

## Removals and Renames

| Requirement | Removed or renamed | Reason | Consumers (file:line) | Migration |
|---|---|---|---|---|
| None | None | None | None | None |

No live requirements are removed or renamed. All modifications are additive schema extensions and in-place requirement updates that maintain complete backwards compatibility for legacy and unversioned packets.

## Open Questions

- [ ] Should the ad-hoc authoring surface support a dedicated frontmatter key (`adhoc_skills`), contract field only, or both? (`openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:170`)
- [ ] For roles without dedicated child skills (`archive`, `ultrafixer`), should stub skills be provisioned or should child derivation yield an empty set? (`openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:171`)
- [ ] Should `lucind.yaml` support a per-repository override for the default skill budget of 3? (`openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:172`)

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/packet_authoring.go:32-54` | `admitDispatchBatch` executes pre-dispatch admission over batch items |
| `internal/accept/accept.go:275-286` | Internal decode struct for authored contract in acceptance verification |
| `internal/accept/authoring_evidence_test.go:56-127` | Unit tests for versioned result exact correspondence and evidence mutations |
| `internal/ledger/authoring.go:20-42` | `AuthoringEvidence` struct schema carrying versioned contract and binding JSON blobs |
| `internal/ledger/authoring.go:44-75` | `FreezeAuthoringEvidence` and `DecodeAuthoringEvidence` hashing and verification functions |
| `internal/packet/packet.go:122-179` | `packet.Parse` frontmatter parser loop handling known frontmatter keys |
| `internal/packetauthor/compile.go:15-25` | `normalizedContract` struct definition for compiled packet authoring |
| `internal/packetauthor/compile.go:171-183` | `renderBody` compiling markdown body from normalized contract |
| `internal/result/result.go:103-116` | `Envelope` struct representing result envelope schema |
| `internal/result/result.schema.json:1-165` | JSON schema definition for result envelope validation |
| `internal/run/run.go:876-904` | `enforceAllowedPaths` demoting out-of-scope changes to `lane.Deviated` |
| `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:38` | Proposal section specifying deterministic three-tier skill derivation |
| `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:40` | Proposal section specifying skill root resolution and fail-closed admission |
| `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:43` | Proposal section specifying contract blob preservation without ledger migration |
| `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:44` | Proposal section specifying dual delivery via body rendering and environment variables |
| `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:46` | Proposal section specifying two-site enforcement in run demotion and accept verification |
| `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:52` | Proposal section specifying closed-set `lane_role` frontmatter key |
| `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:80-118` | Proposal Delta Specifications defining the six assumed requirements |
| `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:170` | Open question regarding ad-hoc skill authoring surface |
| `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:171` | Open question regarding missing child skills for archive and ultrafixer |
| `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:172` | Open question regarding skill budget repository overrides |
| `openspec/specs/acceptance-verifier/spec.md:1` | Capability spec for `acceptance-verifier` |
| `openspec/specs/acceptance-verifier/spec.md:9` | Live requirement `Exact Acceptance Binding` |
| `openspec/specs/acceptance-verifier/spec.md:14` | Scenario `Record the complete binding` |
| `openspec/specs/acceptance-verifier/spec.md:20` | Scenario `Reject an identity mismatch` |
| `openspec/specs/acceptance-verifier/spec.md:25` | Scenario `Stale authored evidence cannot be substituted` |
| `openspec/specs/acceptance-verifier/spec.md:30` | Live requirement `Fail-Closed Mechanical Criteria` |
| `openspec/specs/acceptance-verifier/spec.md:35` | Scenario `Reject invalid result evidence` |
| `openspec/specs/acceptance-verifier/spec.md:40` | Scenario `Reject scope or check failure` |
| `openspec/specs/acceptance-verifier/spec.md:45` | Scenario `Reject authored-result mismatch` |
| `openspec/specs/acceptance-verifier/spec.md:50` | Scenario `Reject commit or path-class mismatch` |
| `openspec/specs/acceptance-verifier/spec.md:55` | Scenario `Preserve explicit legacy behavior` |
| `openspec/specs/acceptance-verifier/spec.md:60` | Live requirement `Frozen Candidate Verification` |
| `openspec/specs/acceptance-verifier/spec.md:64` | Scenario `Primary checkout changes concurrently` |
| `openspec/specs/acceptance-verifier/spec.md:70` | Live requirement `Owned Isolation and Cleanup` |
| `openspec/specs/acceptance-verifier/spec.md:74` | Scenario `Clean owned isolation` |
| `openspec/specs/acceptance-verifier/spec.md:80` | Scenario `Preserve foreign worktrees` |
| `openspec/specs/acceptance-verifier/spec.md:86` | Live requirement `Durable Receipt and Exact Cache Reuse` |
| `openspec/specs/acceptance-verifier/spec.md:90` | Scenario `Persist successful acceptance` |
| `openspec/specs/acceptance-verifier/spec.md:96` | Scenario `Reuse only an exact receipt` |
| `openspec/specs/acceptance-verifier/spec.md:103` | Live requirement `Receipt-Gated CLI Success` |
| `openspec/specs/acceptance-verifier/spec.md:107` | Scenario `Successful command` |
| `openspec/specs/acceptance-verifier/spec.md:113` | Scenario `Receipt absent` |
| `openspec/specs/acceptance-verifier/spec.md:119` | Live requirement `No Promotion Authority` |
| `openspec/specs/acceptance-verifier/spec.md:123` | Scenario `Accepted candidate remains unpromoted` |
| `openspec/specs/acceptance-verifier/spec.md:130` | Live requirement `Mechanical Evidence Is Not Semantic Approval` |
| `openspec/specs/acceptance-verifier/spec.md:134` | Scenario `Present an acceptance receipt` |
| `openspec/specs/lane-execution/spec.md:1` | Capability spec for `lane-execution` |
| `openspec/specs/lane-execution/spec.md:10` | Live requirement `Gate Placement in the Lifecycle` |
| `openspec/specs/lane-execution/spec.md:15` | Scenario `Approve then persist done` |
| `openspec/specs/lane-execution/spec.md:21` | Scenario `Timeout persists blocked, never done` |
| `openspec/specs/lane-execution/spec.md:27` | Live requirement `Resolve Before Barrier Observation` |
| `openspec/specs/lane-execution/spec.md:33` | Scenario `Barrier waits for terminal persist` |
| `openspec/specs/lane-execution/spec.md:38` | Scenario `Barrier stays idle while one lane waits` |
| `openspec/specs/lane-execution/spec.md:44` | Live requirement `Additive Schema, Unchanged Enum` |
| `openspec/specs/lane-execution/spec.md:49` | Scenario `Persist approval record` |
| `openspec/specs/lane-execution/spec.md:56` | Scenario `Mark a defect surfaced later` |
| `openspec/specs/lane-execution/spec.md:63` | Live requirement `Lane metadata dispatch persistence` |
| `openspec/specs/lane-execution/spec.md:67` | Scenario `Dispatch persists metadata` |
| `openspec/specs/lane-execution/spec.md:73` | Scenario `Historical rows preserved` |
| `openspec/specs/lane-execution/spec.md:79` | Scenario `Pre-dispatch failure persists metadata` |
| `openspec/specs/lane-execution/spec.md:85` | Live requirement `Universal Pre-Dispatch Packet Admission` |
| `openspec/specs/lane-execution/spec.md:89` | Scenario `Safe mixed batch` |
| `openspec/specs/lane-execution/spec.md:94` | Scenario `Unsafe packet blocks allocation` |
| `openspec/specs/lane-execution/spec.md:99` | Scenario `Target becomes stale` |
| `openspec/specs/lane-execution/spec.md:104` | Live requirement `Frozen Authored Candidate Evidence` |
| `openspec/specs/lane-execution/spec.md:108` | Scenario `Versioned candidate freezes correspondence evidence` |
| `openspec/specs/lane-execution/spec.md:113` | Scenario `Source packet changes later` |
| `openspec/specs/lane-execution/spec.md:118` | Scenario `Legacy packet is explicit` |
| `openspec/specs/packet-authoring-contract/spec.md:1` | Capability spec for `packet-authoring-contract` |
| `openspec/specs/packet-authoring-contract/spec.md:9` | Live requirement `Versioned Contract and Late Target Binding` |
| `openspec/specs/packet-authoring-contract/spec.md:13` | Scenario `Compile with a feature binding` |
| `openspec/specs/packet-authoring-contract/spec.md:18` | Scenario `Reject authored target authority` |
| `openspec/specs/packet-authoring-contract/spec.md:23` | Scenario `Reject a stale binding` |
| `openspec/specs/packet-authoring-contract/spec.md:28` | Live requirement `Deterministic Rendering and Digest` |
| `openspec/specs/packet-authoring-contract/spec.md:32` | Scenario `Deterministic replay` |
| `openspec/specs/packet-authoring-contract/spec.md:37` | Scenario `Relevant input changes` |
| `openspec/specs/packet-authoring-contract/spec.md:42` | Live requirement `Universal Admission and Manual Compatibility` |
| `openspec/specs/packet-authoring-contract/spec.md:46` | Scenario `Safe legacy manual packet` |
| `openspec/specs/packet-authoring-contract/spec.md:51` | Scenario `Unsafe legacy manual packet` |
| `openspec/specs/packet-authoring-contract/spec.md:56` | Scenario `Contradictory mode` |
| `openspec/specs/packet-authoring-contract/spec.md:61` | Live requirement `Versioned Result Correspondence` |
| `openspec/specs/packet-authoring-contract/spec.md:65` | Scenario `Exact versioned result` |
| `openspec/specs/packet-authoring-contract/spec.md:70` | Scenario `Omitted or extra declaration` |
| `openspec/specs/read-only-packet-schema/spec.md:1` | Capability spec for `read-only-packet-schema` |
| `openspec/specs/read-only-packet-schema/spec.md:9` | Live requirement `Frontmatter Read-Only Field Parsing` |
| `openspec/specs/read-only-packet-schema/spec.md:13` | Scenario `Explicit read_only true` |
| `openspec/specs/read-only-packet-schema/spec.md:18` | Scenario `Explicit read_only false` |
| `openspec/specs/read-only-packet-schema/spec.md:23` | Scenario `Non-boolean value rejected` |
| `openspec/specs/read-only-packet-schema/spec.md:28` | Live requirement `Default Value and Backward Compatibility` |
| `openspec/specs/read-only-packet-schema/spec.md:32` | Scenario `Omitted read_only defaults to write packet` |
| `openspec/specs/read-only-packet-schema/spec.md:37` | Scenario `Existing validation gates preserved` |
| `openspec/specs/read-only-packet-schema/spec.md:42` | Scenario `Unknown frontmatter keys still ignored` |
| `openspec/specs/read-only-packet-schema/spec.md:47` | Live requirement `Explicit Flag Only — No Inference` |
| `openspec/specs/read-only-packet-schema/spec.md:51` | Scenario `Explore-prefixed packet is still write by default` |
| `openspec/specs/read-only-packet-schema/spec.md:56` | Scenario `An empty or absent path list is not a read-only signal` |
| `openspec/specs/read-only-packet-schema/spec.md:61` | Live requirement `The Envelope Cannot Declare or Override Mode` |
| `openspec/specs/read-only-packet-schema/spec.md:65` | Scenario `Envelope without commit stays valid` |
| `openspec/specs/read-only-packet-schema/spec.md:70` | Scenario `Envelope cannot inject a read_only property` |
| `openspec/specs/read-only-packet-schema/spec.md:75` | Live requirement `Additive Rollback` |
| `openspec/specs/read-only-packet-schema/spec.md:79` | Scenario `Revert restores the unknown-key drop` |
| `openspec/specs/read-only-packet-schema/spec.md:84` | Live requirement `Extended packet frontmatter parsing` |
| `openspec/specs/read-only-packet-schema/spec.md:88` | Scenario `Parse frontmatter keys` |
| `openspec/specs/read-only-packet-schema/spec.md:94` | Scenario `Optional keys omitted` |
| `openspec/specs/read-only-packet-schema/spec.md:100` | Scenario `Empty frontmatter values handled` |
| `openspec/specs/read-only-packet-schema/spec.md:106` | Live requirement `Read-Only Input Path Preservation and Visibility` |
| `openspec/specs/read-only-packet-schema/spec.md:110` | Scenario `Declared inputs reach the executor` |
| `openspec/specs/read-only-packet-schema/spec.md:115` | Scenario `Omitted inputs preserve compatibility` |
| `openspec/specs/read-only-packet-schema/spec.md:120` | Scenario `Read-only input does not grant writes` |
| `openspec/specs/read-only-packet-schema/spec.md:125` | Live requirement `Read-Only Path Validation` |
| `openspec/specs/read-only-packet-schema/spec.md:128` | Scenario `Traversal path rejected` |
| `openspec/specs/read-only-packet-schema/spec.md:134` | Scenario `Rename crosses read-only input scope` |
