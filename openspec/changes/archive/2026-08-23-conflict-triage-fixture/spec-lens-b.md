# Spec Lens B — Scenarios & Coverage: Conflict Triage Fixture

## Assumed requirements

The `conflict-triage-fixture` change introduces three new capabilities (`conflict-triage`, `conflict-fixture`, `triage-evaluation-rubric`) and modifies `reconciliation-approval`. The assumed requirement set comprises: `Deterministic three-hunk fixture`, asserting that the generator forces `ClassRequired` on a 3-hunk toy file between two leased features sharing a registered `base_sha`; `Semantic triage and risk ratchet`, asserting that the advisory agent explains cause, resolves mechanical controls, flags business conflicts as ARBITRARY, ratchets risk to `high`, and never fails closed; `Two-step close and retry CAS`, asserting that approval and out-of-band resolution unblock retry and promote via compare-and-swap; and `Dual-judge rubric isolation`, asserting offline A/B evaluation across pinned executors with complete provider isolation and rejection of uniform hunk scoring.

## Scenarios

### Requirement: Deterministic three-hunk fixture

#### Scenario: Fixture generation forces ClassRequired and creates awaiting request

- GIVEN two active leased features sharing a registered `base_sha` and conflicting across three hunks in the toy file
- WHEN `evaluateOverlapGate` calls `Evaluate` on the candidate commits
- THEN classification is `ClassRequired`, `CreateRequest` persists `overlap_evidence` and an awaiting `reconciliation_requests` row, and attempt status transitions to `AttemptStatusBlocked`
- AND the feature lease is released for external resolution

#### Scenario: Sibling build features validate disjoint allowed paths

- GIVEN two build packets targeting `internal/conflicttriage/` and `internal/conflicttriage/fixture/`
- WHEN `DisjointAllowedPaths` validates the batch
- THEN validation fails unless dispatched as separate `lucind-ai run` dispatches with disjoint scopes

#### Scenario: Missing shared base SHA bypasses classification

- GIVEN two features with missing or non-matching `base_sha` registrations
- WHEN `evaluateOverlapGate` evaluates overlap
- THEN `Evaluate` returns `overlap.ErrNoMergeBase` and the gate continues without creating an awaiting request or blocking promotion

### Requirement: Semantic triage and risk ratchet

#### Scenario: Business conflict ratchets risk to high with verify budget

- GIVEN an awaiting reconciliation request containing a business conflict hunk with no technical selection criterion
- WHEN `conflict-triage` evaluates the conflict
- THEN triage flags the business hunk as ARBITRARY, pins overall risk to `high`, records a wall-clock verify budget with concrete command, and writes structured JSON to `Candidate.Output`

#### Scenario: Mechanical control hunks resolve deterministically without ambiguity error

- GIVEN conflict hunks consisting solely of slice-literal union and rename-versus-edit controls
- WHEN `conflict-triage` processes the hunks
- THEN it emits deterministic resolution proposals and completes without raising `ErrSemanticAmbiguity`

#### Scenario: Triage output fails validation on invariant violations

- GIVEN a triage resolution candidate that introduces unresolved conflict markers or edits outside `allowed_paths`
- WHEN `ScanConflictMarkers` or `EnforceAllowedPaths` runs
- THEN candidate validation fails and marks the candidate failed without blocking ledger auditability

### Requirement: Two-step close and retry CAS

#### Scenario: Approved candidate unblocks retry and promotes via CAS

- GIVEN an approved reconciliation request with a registered candidate SHA and an unchanged target feature tip
- WHEN `evaluateOverlapGate` evaluates the retry dispatch
- THEN the gate adopts `CandidateSHA` and successfully promotes the ref via compare-and-swap update-ref

#### Scenario: Concurrent multi-feature resolutions block instead of picking arbitrary candidate

- GIVEN an attempt resolving multiple required conflicts simultaneously across two or more features
- WHEN `evaluateOverlapGate` processes the resolved candidates
- THEN the gate detects multiple resolutions, blocks promotion with attempt status `AttemptStatusBlocked`, and releases the lease for sequential processing

#### Scenario: Target feature tip drift rejects stale resolution on retry

- GIVEN an approved request whose target feature tip has moved since the candidate SHA was registered
- WHEN retry integration dispatch executes `evaluateOverlapGate`
- THEN the gate rejects the stale candidate SHA and transitions attempt status to `AttemptStatusBlocked`

### Requirement: Dual-judge rubric isolation

#### Scenario: Offline rubric grades distinct classification of three hunks

- GIVEN identical 3-hunk fixture evidence evaluated by `claude-opus-5` and `openai/gpt-5.6-sol`
- WHEN the evaluation rubric grades executor outputs
- THEN the rubric awards a passing score only to evaluations distinguishing the business hunk from mechanical controls and flagging ARBITRARY on business

#### Scenario: Streaming Claude decoder recovers terminal output on partial degradation

- GIVEN an A/B evaluation dispatch on `claude` producing progress stream events with degraded non-terminal chunks
- WHEN `Claude.Run` processes the stream decoder
- THEN execution falls back gracefully to raw stdout while retaining complete terminal JSON output

#### Scenario: Uniform scoring across all hunks fails rubric evaluation

- GIVEN a judge evaluation that assigns identical risk or classification across all three distinct hunks
- WHEN the rubric computes the evaluation score
- THEN the rubric rejects the evaluation with a failing grade

## Coverage

| Requirement | Happy path | Edge case | Error state | Testable through (file:line) |
|---|---|---|---|---|
| Deterministic three-hunk fixture | covered | covered | covered | `internal/reconcile/reconcile_test.go:52-100`, `internal/overlap/overlap.go:623-659`, `internal/run/attempt.go:743-747,777-855` |
| Semantic triage and risk ratchet | covered | covered | covered | `internal/resolve/candidate_test.go:16-49,51-80`; new seam required |
| Two-step close and retry CAS | covered | covered | covered | `cmd/lucind-ai/cli_test.go:2944,3126`, `internal/run/attempt.go:821-828,870`, `internal/integrate/integrate.go:151-173` |
| Dual-judge rubric isolation | covered | covered | covered | `internal/executor/claude.go:35-49,106-122`, `internal/executor/opencode.go:53-59`; new seam required |

## Untestable Assertions

None. All scenario outcomes assert observable ledger rows, structured JSON payload fields, attempt state machine transitions, or offline rubric grading outputs.

## Open Questions

- [ ] Prepared SHA writer authority: Proposal requirement says the agent leaves a prepared SHA (`internal/reconcile/reconcile.go:107`), while Approach says a human resolves out of band and registers the SHA via `reconcile resolve --candidate --sha`. Design must decide explicitly whether the agent or human operator sets `CandidateSHA`.
- [ ] Risk formula calibration: Exact non-decreasing risk formula and threshold boundaries for mixed business and mechanical hunks remain open until fixture calibration data is gathered.
- [ ] Production triage executor: Selection of the production executor and model runtime for live conflict triage remains open; offline rubric pins `opencode`/`openai/gpt-5.6-sol` and `claude`/`claude-opus-5`.
- [ ] Specification process drift: `~/.claude/skills/sdd-spec/SKILL.md` prescribes a single agent authoring delta specs under `specs/` with an Engram summary, superseded by this three-lens parallel workflow.

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:56` | Reconcile CLI commands usage for approve and resolve |
| `cmd/lucind-ai/cli.go:65-70` | Registered executor map for claude, opencode, agy, cursor-agent |
| `cmd/lucind-ai/cli_test.go:2944` | TestReconcileApproveCLI tests approval of awaiting reconciliation requests |
| `cmd/lucind-ai/cli_test.go:3126` | TestReconcileResolveCLI tests candidate registration and unblocking |
| `internal/executor/claude.go:35` | DefaultModel for Claude returns claude-opus-5 |
| `internal/executor/claude.go:35-49` | Claude model registration pins claude-opus-5 to prevent cross-provider escalation |
| `internal/executor/claude.go:106-122` | Claude.Run stream-json decoder fallback to stdout |
| `internal/executor/claude_stream.go:10-16` | Claude progress stream degradation handling |
| `internal/executor/opencode.go:53-54` | DefaultModel for Opencode returns openai/gpt-5.6-sol |
| `internal/executor/opencode.go:53-59` | Opencode model registration pins openai/gpt-5.6-sol |
| `internal/feature/feature.go:101-113` | ValidateParentRef rejects empty, main, and lucind/* parent refs |
| `internal/feature/feature.go:123-124` | Create enforces non-empty baseSHA requirement |
| `internal/integrate/integrate.go:151-173` | PromoteCASWithRunner executes compare-and-swap git update-ref |
| `internal/ledger/schema.go:156-169` | Schema definition for reconciliation_candidates table |
| `internal/ledger/schema.go:163` | Column output in reconciliation_candidates stores triage JSON |
| `internal/ledger/schema.go:165` | Column candidate_sha in reconciliation_candidates stores registered commit SHA |
| `internal/overlap/overlap.go:93-98` | DefaultThresholds defines default conflict classification thresholds |
| `internal/overlap/overlap.go:623` | Classify signature and classification logic |
| `internal/overlap/overlap.go:634-650` | Classify triggers ClassRequired on rename/delete collisions, shared binary, and intersecting hunks |
| `internal/overlap/overlap.go:658-659` | Classify returns ClassRequired when required rationales are present |
| `internal/packet/disjoint.go:29-48` | DisjointAllowedPaths validates pairwise disjointness of allowed paths across packets |
| `internal/reconcile/reconcile.go:105` | Candidate.Output struct field for storing triage JSON payload |
| `internal/reconcile/reconcile.go:107` | Candidate.CandidateSHA struct field for storing resolved commit SHA |
| `internal/reconcile/reconcile.go:266` | CreateRequest inserts overlap_evidence record into ledger |
| `internal/reconcile/reconcile.go:280-300` | CreateRequest inserts reconciliation_requests row with status awaiting |
| `internal/reconcile/reconcile.go:406-535` | Service.Approve validates and transitions request from awaiting to approved |
| `internal/reconcile/reconcile_test.go:52-100` | TestCreateRequestFromRequiredOverlapDisplaysExactFields verifies request creation fields |
| `internal/resolve/candidate.go:26` | ErrSemanticAmbiguity sentinel error in fail-closed resolver |
| `internal/resolve/candidate.go:48-95` | ScanConflictMarkers checks worktree files for unresolved git conflict markers |
| `internal/resolve/candidate.go:100-145` | EnforceAllowedPaths validates modified paths against declared allowed paths scope |
| `internal/resolve/candidate.go:303-312` | Prompt template for fail-closed candidate resolver |
| `internal/resolve/candidate_test.go:16-49` | TestScanConflictMarkers tests detection of git conflict markers |
| `internal/resolve/candidate_test.go:51-80` | TestEnforceAllowedPaths tests enforcement of allowed_paths bounds |
| `internal/run/attempt.go:687` | evaluateOverlapGate function entry point |
| `internal/run/attempt.go:743-747` | evaluateOverlapGate continues when Evaluate returns ErrNoMergeBase |
| `internal/run/attempt.go:777-855` | evaluateOverlapGate handles ClassRequired by creating request, blocking attempt, and releasing lease |
| `internal/run/attempt.go:821-828` | evaluateOverlapGate matches approved candidate against unchanged target feature tip |
| `internal/run/attempt.go:848-855` | evaluateOverlapGate transitions attempt to AttemptStatusBlocked and releases lease |
| `internal/run/attempt.go:870` | evaluateOverlapGate adopts resolved CandidateSHA for CAS promotion |
| `internal/run/integrate_feature.go:17` | ErrMixedFeatureTargets error definition for mixed batch targets |
| `internal/run/integrate_feature.go:41` | FeatureTarget rejects batches mixing legacy and feature targets |
| `internal/run/integrate_feature.go:73` | FeatureTarget validates parent_ref against main and lucind/* refs |
| `internal/worktree/worktree.go:278-292` | IsLinkedWorktree checks whether worktree path is a linked git worktree |
