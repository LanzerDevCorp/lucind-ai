# Spec Lens A — Capabilities & Requirements: Conflict Triage Fixture

## Assumed requirements

This change introduces four requirements across four capabilities: three new capabilities (`conflict-fixture`, `conflict-triage`, and `triage-evaluation-rubric`) and one existing capability (`reconciliation-approval`). `conflict-fixture` defines one requirement for deterministic three-hunk fixture generation targeting `openspec/specs/conflict-fixture/spec.md`. `conflict-triage` defines one requirement for fail-open advisory conflict triage and risk assessment targeting `openspec/specs/conflict-triage/spec.md`. `triage-evaluation-rubric` defines one requirement for isolated dual-judge A/B scoring targeting `openspec/specs/triage-evaluation-rubric/spec.md`. `reconciliation-approval` adds one requirement for two-step human resolution registration and retry CAS promotion targeting `openspec/changes/conflict-triage-fixture/specs/reconciliation-approval/spec.md` as a delta against `openspec/specs/reconciliation-approval/spec.md:1-74`.

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|
| `conflict-fixture` | New | `openspec/specs/conflict-fixture/spec.md` | — |
| `conflict-triage` | New | `openspec/specs/conflict-triage/spec.md` | — |
| `reconciliation-approval` | Existing | `openspec/changes/conflict-triage-fixture/specs/reconciliation-approval/spec.md` | `openspec/specs/reconciliation-approval/spec.md:1-74` |
| `triage-evaluation-rubric` | New | `openspec/specs/triage-evaluation-rubric/spec.md` | — |

## ADDED Requirements

### Requirement: Deterministic three-hunk fixture

The fixture generator at `internal/conflicttriage/fixture/` MUST generate two leased feature branches sharing a single registered base SHA (`internal/feature/feature.go:123-124`) and valid parent references (`internal/feature/feature.go:101-113`) that trigger `ClassRequired` (`internal/overlap/overlap.go:623-659`) during overlap evaluation (`internal/run/attempt.go:687`), where a missing or divergent base SHA would cause `ErrNoMergeBase` and bypass classification (`internal/run/attempt.go:743-747`). The synthetic conflict file MUST consist of exactly three hunks: one business conflict where both branches compile and pass independent tests, and two mechanical control conflicts (a slice-literal union and a rename-versus-edit collision). The generator's two build features MUST maintain prefix-disjoint allowed paths (`internal/packet/disjoint.go:29-48`) and MUST be dispatched as separate feature batches (`internal/run/integrate_feature.go:17`, `internal/run/integrate_feature.go:41`).

**Terminal consumer**: `internal/conflicttriage/fixture/` generator invoked by integration tests and test harnesses (new, introduced by this change; exercising `internal/run/attempt.go:687` and `internal/reconcile/reconcile.go:266`).

### Requirement: Semantic triage and risk ratchet

The `conflict-triage` advisory agent MUST analyze pending reconciliation requests and persist structured JSON to candidate output (`internal/reconcile/reconcile.go:105`, `internal/ledger/schema.go:163`) without failing closed on semantic ambiguity (`internal/resolve/candidate.go:26`, `internal/resolve/candidate.go:303-312`). For business conflicts lacking technical selection criteria, the agent MUST flag the hunk as ARBITRARY, document why the selected branch was chosen, pin the risk classification to `high` (which downstream tools cannot lower), propose a candidate resolution commit, and specify verification cost as wall-clock duration plus concrete command. Mechanical control hunks MUST receive deterministic candidate resolutions. Triage outputs MUST satisfy invariant checks for absence of conflict markers (`internal/resolve/candidate.go:48-95`) and adherence to declared allowed paths (`internal/resolve/candidate.go:100-145`).

**Terminal consumer**: `internal/conflicttriage/` advisory agent writing to `reconciliation_candidates.output` (`internal/reconcile/reconcile.go:105`, `internal/ledger/schema.go:163`), observable via `reconcile resolve` and inspection commands (`cmd/lucind-ai/cli.go:56`).

### Requirement: Two-step close and retry CAS

Clearing a `ClassRequired` promotion block MUST require explicit direction approval via `reconcile approve` (`internal/reconcile/reconcile.go:406-535`, `cmd/lucind-ai/cli.go:56`) followed by candidate resolution registration via `reconcile resolve --candidate --sha` executed from the primary checkout root (`internal/worktree/worktree.go:278-292`). Upon feature retry, `evaluateOverlapGate` MUST adopt `CandidateSHA` (`internal/run/attempt.go:870`) if and only if the conflicting target feature tip matches the resolved SHA (`internal/run/attempt.go:821-828`), re-blocking if the target tip has advanced, and promote the resolved feature via compare-and-swap (`internal/integrate/integrate.go:151-173`).

**Terminal consumer**: `internal/run/attempt.go:821-828`, `internal/run/attempt.go:870` in `evaluateOverlapGate`, promoting via `internal/integrate/integrate.go:151-173` and driven by CLI verbs in `cmd/lucind-ai/cli.go:56`.

### Requirement: Dual-judge rubric isolation

The offline triage evaluation rubric MUST execute the identical three-hunk fixture across registered model executors (`cmd/lucind-ai/cli.go:65-70`), specifically evaluating `claude`/`claude-opus-5` (`internal/executor/claude.go:35-52`, `internal/executor/claude.go:106-122`) against `opencode`/`openai/gpt-5.6-sol` (`internal/executor/opencode.go:52-65`) without cross-provider configuration or billing leaks. The rubric SHALL grade each evaluation on distinguishing the business conflict from the mechanical controls and explicitly declaring ARBITRARY on the business hunk; any evaluation that scores all three hunks identically SHALL fail.

**Terminal consumer**: Offline evaluation test runner and judge harness in `internal/conflicttriage/fixture/` (new, introduced by this change).

## Open Questions

- [ ] Exact non-decreasing risk formula and thresholds: What is the precise numerical or tiered formula and thresholds for calculating composite risk across mixed business and mechanical hunks? (Pending empirical calibration from fixture runs).
- [ ] Production triage executor and model selection: Which registered executor and model will be assigned to run production conflict triage runs (as distinct from the offline A/B evaluation judges `claude-opus-5` and `gpt-5.6-sol`)?
- [ ] CandidateSHA write authority and lifecycle: Does the triage agent write candidate resolution commits directly into `Candidate.CandidateSHA` (`internal/reconcile/reconcile.go:107`), or does it stage proposed changes for a human operator to inspect and register out-of-band via `lucind-ai reconcile resolve --candidate --sha`? (Delegated to the design phase).
- [ ] SDD spec phase fan-out structure: The `sdd-spec` skill specifies a single subagent writing the complete delta spec tree and persisting to Engram, whereas this packet executes a three-lens parallel fan-out where Lens A owns capability mapping and requirement text while Lens B and C own scenarios and live spec deltas. (Process note acknowledging packet precedence).

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:56` | usage string defining reconcile CLI verbs including approve and resolve |
| `cmd/lucind-ai/cli.go:65-70` | supportedExecutors registry mapping executor identifiers to constructors |
| `internal/executor/claude.go:35-52` | Claude executor DefaultModel and KnownModels pinned to claude-opus-5 |
| `internal/executor/claude.go:106-122` | Claude.Run stream-json progress handling and terminal record decoding |
| `internal/executor/opencode.go:52-65` | Opencode executor DefaultModel and KnownModels pinned to openai/gpt-5.6-sol |
| `internal/feature/feature.go:101-113` | ValidateParentRef rejects empty, main, and lucind/ branch references |
| `internal/feature/feature.go:123-124` | Service.Create requires non-empty baseSHA |
| `internal/integrate/integrate.go:151-173` | PromoteCASWithRunner performs compare-and-swap feature branch promotion |
| `internal/ledger/schema.go:163` | reconciliation_candidates table schema defines output column |
| `internal/overlap/overlap.go:623-659` | Classify maps detected conflict signals to ClassRequired |
| `internal/packet/disjoint.go:29-48` | DisjointAllowedPaths verifies pairwise disjoint path scopes across packets |
| `internal/reconcile/reconcile.go:105` | Candidate struct defines Output field for triage JSON payload |
| `internal/reconcile/reconcile.go:107` | Candidate struct defines CandidateSHA field for resolution commit |
| `internal/reconcile/reconcile.go:266` | CreateRequest persists overlap evidence on ClassRequired |
| `internal/reconcile/reconcile.go:406-535` | Service.Approve validates and transitions reconciliation request to approved |
| `internal/resolve/candidate.go:26` | ErrSemanticAmbiguity sentinel error returned on unresolved semantic conflict |
| `internal/resolve/candidate.go:48-95` | ScanConflictMarkers detects git merge conflict markers in worktree files |
| `internal/resolve/candidate.go:100-145` | EnforceAllowedPaths checks modified paths against declared allowed paths |
| `internal/resolve/candidate.go:303-312` | resolve prompt instructs resolver to fail closed on semantic ambiguity |
| `internal/run/attempt.go:687` | evaluateOverlapGate evaluates overlap for active feature attempts |
| `internal/run/attempt.go:743-747` | ErrNoMergeBase from overlap evaluation causes gate to continue |
| `internal/run/attempt.go:821-828` | evaluateOverlapGate validates target tip unchanged before adopting candidate |
| `internal/run/attempt.go:870` | evaluateOverlapGate overrides CandidateSHA with resolved candidate SHA |
| `internal/run/integrate_feature.go:17` | ErrMixedFeatureTargets sentinel returned when batch packets mix feature targets |
| `internal/run/integrate_feature.go:41` | FeatureTarget rejects batches with mixed legacy or feature targets |
| `internal/worktree/worktree.go:278-292` | IsLinkedWorktree checks if directory is a linked git worktree |
| `openspec/specs/reconciliation-approval/spec.md:1-74` | Live specification for reconciliation-approval capability |
