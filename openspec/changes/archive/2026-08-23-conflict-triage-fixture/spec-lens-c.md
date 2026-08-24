# Spec Lens C — Live-Spec Conflicts & Migration: Conflict Triage Fixture

## Assumed requirements

This change modifies 1 existing capability (`reconciliation-approval`) and introduces 3 new capabilities (`conflict-triage`, `conflict-fixture`, `triage-evaluation-rubric`). The assumed requirement set asserted in `openspec/changes/conflict-triage-fixture/proposal.md:79-134` covers: (1) `Deterministic three-hunk fixture` (`openspec/changes/conflict-triage-fixture/proposal.md:79-92`), asserting `internal/conflicttriage/fixture/` generates two leased features sharing a registered `base_sha` that forces `ClassRequired` during `evaluateOverlapGate` on a 3-hunk file (1 business conflict, 2 mechanical controls) without build-feature collisions; (2) `Semantic triage and risk ratchet` (`openspec/changes/conflict-triage-fixture/proposal.md:93-106`), asserting advisory `conflict-triage` explains causes without failing closed on semantic ambiguity, flags ARBITRARY on business hunks, ratchets risk to `high`, leaves a prepared SHA, states verify cost in wall clock plus command, and writes JSON into `Candidate.Output` (`internal/reconcile/reconcile.go:105`); (3) `Two-step close and retry CAS` (`openspec/changes/conflict-triage-fixture/proposal.md:107-120`), asserting `ClassRequired` clears via `reconcile approve` followed by out-of-band resolution registering the candidate SHA via `reconcile resolve --candidate --sha` from the primary root, promoting on retry via CAS if target tip is unchanged; and (4) `Dual-judge rubric isolation` (`openspec/changes/conflict-triage-fixture/proposal.md:121-134`), asserting offline rubric evaluation across pinned judges (`claude`/`claude-opus-5` and `opencode`/`openai/gpt-5.6-sol`) without cross-provider leaks, scoring cause quality and hunk separation.

## Live Spec Inventory

| Capability | Live spec (file:line) | Requirements | Scenarios | Touched by this change |
|---|---|---|---|---|
| `reconciliation-approval` | `openspec/specs/reconciliation-approval/spec.md:1-74` | 5 | 10 | 1 requirement (2 scenarios) modified (`One Bounded Candidate`, `:47-60`); 4 requirements (8 scenarios) exercised/preserved (`Visible Balanced Evidence`, `:5-18`; `Required Reconciliation Gate`, `:19-32`; `Exact Expiring Direction Approval`, `:33-46`; `Resolver Authority and Observable Audit`, `:61-74`) |

## Conflicts

None. The change preserves all live guarantees in `openspec/specs/reconciliation-approval/spec.md:1-74` (deterministic overlap classification, gating on required overlap, single-record expiring direction approval, ref-matching CAS retry promotion, fail-closed resolver authority, and observable audit history).

Specifically:
- In `Visible Balanced Evidence` (`:5-18`): The fixture generator creates two active features sharing a registered `base_sha` and editing a 3-hunk file, evaluated to `ClassRequired` by `overlap.Classify` (`internal/overlap/overlap.go:623-660`) under existing `DefaultThresholds` (`internal/overlap/overlap.go:93-98`). No classification signals or guarantees are invalidated.
- In `Required Reconciliation Gate` (`:19-32`): `evaluateOverlapGate` (`internal/run/attempt.go:687-856`) creates an awaiting request, inserts `overlap_evidence` inside `CreateRequest` (`internal/reconcile/reconcile.go:266,280-305`), blocks promotion (`internal/run/attempt.go:848-855`), and releases the lease (`internal/run/attempt.go:854-855`). Gating behaviors are exercised as specified.
- In `Exact Expiring Direction Approval` (`:33-46`): Direction approval via `reconcile approve` (`cmd/lucind-ai/cli.go:56`; `internal/reconcile/reconcile.go:406-435`) continues to bind exact direction, evidence snapshot, and expected SHAs.
- In `One Bounded Candidate` (`:47-60`): Live spec guarantees CAS retry promotion only when mandatory checks pass, limits are honored, no markers remain, and expected refs are unchanged (`internal/run/attempt.go:821-828,870`; `internal/integrate/integrate.go:151-173`). This change modifies candidate payload handling to store structured triage JSON in `Candidate.Output` (`internal/reconcile/reconcile.go:105`; `internal/ledger/schema.go:163`) and candidate SHA registration via `reconcile resolve --candidate --sha` (`internal/worktree/worktree.go:278-292`), without altering retry CAS safety checks.
- In `Resolver Authority and Observable Audit` (`:61-74`): The resolver's fail-closed contract on semantic ambiguity (`internal/resolve/candidate.go:26,303-312`) is strictly preserved and isolated from advisory `conflict-triage` (`internal/conflicttriage/`). Triage operates fail-open with ARBITRARY flags and a high risk ratchet, but never relaxes or replaces the resolver's fail-closed mandate. Ledger audit trails (`internal/ledger/schema.go:156-169`) remain observable.

Because no live requirement guarantee is made untrue or broken, there are no unmanaged live-spec conflicts. The modification to candidate payload and resolution is handled through the MODIFIED block below.

## MODIFIED Full Blocks

### Requirement: One Bounded Candidate

**Source**: `openspec/specs/reconciliation-approval/spec.md:47` — 2 scenarios

One valid approval SHALL authorize exactly one bounded Sonnet candidate in the approved target context. Automatic CAS promotion SHALL occur only when mandatory checks pass, limits are honored, no conflict markers remain, and both expected refs are unchanged. No second approval is required.

#### Scenario: Authorized candidate passes
- GIVEN one direction-bound candidate satisfies every gate
- WHEN promotion validates both expected refs
- THEN the target parent SHALL advance by CAS and the source SHALL remain unchanged

#### Scenario: Candidate is unsafe
- GIVEN checks fail, time expires, limits are exceeded, refs are stale, or markers remain
- WHEN promotion is evaluated
- THEN promotion MUST fail closed and preserve candidate and evidence

## Removals and Renames

| Requirement | Removed or renamed | Reason | Consumers (file:line) | Migration |
|---|---|---|---|---|
| `One Bounded Candidate` | Potential Rename / Modification to `Two-step close and retry CAS` | Proposal §Delta Specifications (`openspec/changes/conflict-triage-fixture/proposal.md:107-120`) names the candidate resolution and retry lifecycle `Two-step close and retry CAS` while extending the single-step Sonnet model to human-registered candidate SHAs and JSON advisory payloads. | `cmd/lucind-ai/cli.go:56, 1138-1195, 1445-1485`; `cmd/lucind-ai/cli_test.go:2944, 3126`; `internal/reconcile/reconcile.go:98-111, 406-435`; `internal/run/attempt.go:821-828, 870`; `internal/integrate/integrate.go:151-173`; `internal/ledger/schema.go:156-169` | If synthesizer renames `One Bounded Candidate` to `Two-step close and retry CAS`, retain both existing scenarios (`Authorized candidate passes`, `Candidate is unsafe`) while adding the two-step CLI registration and target tip drift scenarios; no code migration is required as CLI and retry CAS endpoints already exist. |

No requirements are removed. All live requirement contracts in `openspec/specs/reconciliation-approval/spec.md` remain active.

## Open Questions

- [ ] **Exact non-decreasing risk formula and thresholds**: What exact formula and numeric thresholds govern risk escalation for mixed business and mechanical hunks (e.g. 1 business hunk + 2 mechanical controls)? (`openspec/changes/conflict-triage-fixture/proposal.md:150`). Decided product decisions pin discrete risk bands (low/medium/high) with business conflicts pinned to `high` and agent unable to lower it, but multi-hunk composition remains open pending fixture calibration.
- [ ] **Production triage runtime executor**: Which executor/model should run production triage once calibrated? (`openspec/changes/conflict-triage-fixture/proposal.md:151`). Benchmarking judges are pinned to `claude`/`claude-opus-5` (`internal/executor/claude.go:35`) and `opencode`/`openai/gpt-5.6-sol` (`internal/executor/opencode.go:53-54`), but production dispatch remains a separate open decision.
- [ ] **CandidateSHA writer authority (Design Ambiguity)**: Proposal §Delta Specifications states the agent must "leave a prepared SHA (`internal/reconcile/reconcile.go:107`)" while Proposal §Approach states "a human resolves out of band and registers the SHA" via `reconcile resolve --candidate --sha` (`cmd/lucind-ai/cli.go:56`; `internal/worktree/worktree.go:278-292`). Design must decide explicitly whether the triage agent writes `CandidateSHA` directly into the ledger or if `CandidateSHA` is strictly populated by human CLI registration, preserving the human-in-the-loop safety invariant without spec pre-emption.
- [ ] **Parallel spec lane synthesis vs single-agent spec skill**: `~/.claude/skills/sdd-spec/SKILL.md` describes a single sub-agent writing full delta specs directly under `openspec/changes/<change-name>/specs/` with Engram persistence, whereas this packet executes a 3-lens parallel fan-out where Lens C produces live-spec conflict analysis and verbatim MODIFIED blocks as feedstock for the synthesizer lane.

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:56` | CLI usage string defines `reconcile approve` and `reconcile resolve` subcommands |
| `cmd/lucind-ai/cli.go:65-70` | Supported executors map registers `agy`, `claude`, `cursor-agent`, and `opencode` |
| `cmd/lucind-ai/cli.go:1138-1195` | `runReconcileApprove` implements CLI direction approval dispatch |
| `cmd/lucind-ai/cli.go:1445-1485` | `runReconcileResolve` implements CLI candidate SHA registration |
| `cmd/lucind-ai/cli_test.go:2944` | `TestReconcileApproveCLI` validates reconcile approve CLI command |
| `cmd/lucind-ai/cli_test.go:3126` | `TestReconcileResolveCLI` validates reconcile resolve CLI candidate SHA registration |
| `internal/executor/claude.go:35` | Claude executor defines `claude-opus-5` as its default model |
| `internal/executor/claude.go:106-122` | `Claude.Run` implements streaming decode fallback to raw stdout |
| `internal/executor/claude_stream.go:10-16` | Degraded message constant for Claude stream decoding |
| `internal/executor/opencode.go:53-54` | Opencode executor defines `openai/gpt-5.6-sol` as its default model |
| `internal/feature/feature.go:101-113` | `ValidateParentRef` rejects empty, `main`, and `lucind/` refs |
| `internal/feature/feature.go:123-124` | `Service.Create` validates non-empty `baseSHA` |
| `internal/integrate/integrate.go:151-173` | `PromoteCASWithRunner` performs atomic compare-and-swap update-ref |
| `internal/ledger/schema.go:156-169` | Schema definition for `reconciliation_candidates` table |
| `internal/ledger/schema.go:163` | Schema defines `output` TEXT column in `reconciliation_candidates` |
| `internal/overlap/overlap.go:93-98` | `DefaultThresholds` defines hotspot and nearby hunk defaults |
| `internal/overlap/overlap.go:623-660` | `Classify` evaluates conflict signals into `ClassRequired` |
| `internal/packet/disjoint.go:29-48` | `DisjointAllowedPaths` verifies pairwise disjoint path scopes |
| `internal/reconcile/reconcile.go:98-111` | `Candidate` struct defines fields for candidate resolution records |
| `internal/reconcile/reconcile.go:105` | `Candidate.Output` holds unstructured output or triage JSON |
| `internal/reconcile/reconcile.go:107` | `Candidate.CandidateSHA` holds candidate resolution commit SHA |
| `internal/reconcile/reconcile.go:266` | `CreateRequest` inserts overlap evidence into ledger |
| `internal/reconcile/reconcile.go:280-305` | `CreateRequest` persists awaiting request in `reconciliation_requests` |
| `internal/reconcile/reconcile.go:406-435` | `Service.Approve` validates direction, expiry, and expected SHAs |
| `internal/reconcile/reconcile_test.go:52-100` | `TestCreateRequestFromRequiredOverlapDisplaysExactFields` verifies exact request field persistence |
| `internal/resolve/candidate.go:26` | `ErrSemanticAmbiguity` sentinel error definition for resolver |
| `internal/resolve/candidate.go:48-95` | `ScanConflictMarkers` detects residual merge conflict markers |
| `internal/resolve/candidate.go:100-145` | `EnforceAllowedPaths` checks that resolution edits stay within allowed paths |
| `internal/resolve/candidate.go:303-312` | Prompt enforces fail-closed policy on semantic ambiguity for resolver |
| `internal/resolve/candidate_test.go:16-49` | `TestScanConflictMarkers` verifies conflict marker scanning |
| `internal/resolve/candidate_test.go:51-80` | `TestEnforceAllowedPaths` verifies allowed paths scope enforcement |
| `internal/run/attempt.go:687-856` | `evaluateOverlapGate` manages overlap evaluation, evidence recording, and blocking |
| `internal/run/attempt.go:743-747` | Gate handles `ErrNoMergeBase` by continuing without `ClassRequired` |
| `internal/run/attempt.go:821-828` | Gate matches approved candidate with unchanged target SHA |
| `internal/run/attempt.go:848-855` | Gate marks attempt blocked and releases feature lease |
| `internal/run/attempt.go:854-855` | Gate releases feature lease on blocked attempt |
| `internal/run/attempt.go:870` | Gate adopts `resolvedOverrideSHA` onto `Attempt.CandidateSHA` |
| `internal/run/integrate_feature.go:17` | `ErrMixedFeatureTargets` error definition for mixed batch targets |
| `internal/run/integrate_feature.go:41` | Batch rejects mixed feature targets across packets |
| `internal/run/integrate_feature.go:73` | `FeatureTarget` validates parent ref against `feature.ValidateParentRef` |
| `internal/worktree/worktree.go:278-292` | `IsLinkedWorktree` detects linked worktree via `.git` file with `gitdir:` |
| `openspec/changes/conflict-triage-fixture/proposal.md:79-92` | Proposal delta specification for deterministic three-hunk fixture |
| `openspec/changes/conflict-triage-fixture/proposal.md:79-134` | Proposal delta specifications for conflict triage fixture change |
| `openspec/changes/conflict-triage-fixture/proposal.md:93-106` | Proposal delta specification for semantic triage and risk ratchet |
| `openspec/changes/conflict-triage-fixture/proposal.md:107-120` | Proposal delta specification for two-step close and retry CAS |
| `openspec/changes/conflict-triage-fixture/proposal.md:121-134` | Proposal delta specification for dual-judge rubric isolation |
| `openspec/changes/conflict-triage-fixture/proposal.md:150` | Proposal open question on exact non-decreasing risk formula |
| `openspec/changes/conflict-triage-fixture/proposal.md:151` | Proposal open question on production triage runtime executor |
| `openspec/specs/reconciliation-approval/spec.md:1-74` | Complete live specification for reconciliation-approval capability |
| `openspec/specs/reconciliation-approval/spec.md:5-18` | Live requirement `Visible Balanced Evidence` with 2 scenarios |
| `openspec/specs/reconciliation-approval/spec.md:19-32` | Live requirement `Required Reconciliation Gate` with 2 scenarios |
| `openspec/specs/reconciliation-approval/spec.md:33-46` | Live requirement `Exact Expiring Direction Approval` with 2 scenarios |
| `openspec/specs/reconciliation-approval/spec.md:47` | Live requirement `One Bounded Candidate` header |
| `openspec/specs/reconciliation-approval/spec.md:47-60` | Live requirement `One Bounded Candidate` with 2 scenarios |
| `openspec/specs/reconciliation-approval/spec.md:61-74` | Live requirement `Resolver Authority and Observable Audit` with 2 scenarios |
