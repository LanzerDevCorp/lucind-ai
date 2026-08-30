# Lucind AI Stability Run — SDD Master Plan

This document is the complete product and execution authority for building `lucind-ai stability run`. It consolidates the 91 decisions approved during the 2026-08-24 design session, maps them to their Engram history, derives the implementation requirements, and ends with a copy-pasteable prompt for a fresh Claude session to build the feature through Lucind AI, SDD, and controlled fan-out.

## Outcome

Build a first-class Linux-only product capability that validates one immutable Lucind AI candidate through a **Stability Campaign** containing three consecutive, deterministic, real-`agy` **Stability Trials**. The capability must own its lifecycle, crash recovery, evidence, cleanup, and content-addressed terminal receipt without modifying permanent branches or publishing a release.

The public V1 surface is:

```text
lucind-ai stability run
lucind-ai stability status [--json]
lucind-ai stability resume
lucind-ai stability abort
```

## Authorities and traceability

### Repository authorities

- [`CONTEXT.md`](CONTEXT.md) defines the canonical domain vocabulary.
- [`docs/adr/0001-native-stability-campaign.md`](docs/adr/0001-native-stability-campaign.md) records why stability validation is a native product capability rather than an external harness.
- This master plan is the implementation contract derived from the approved decision tree.

### Engram authorities

Engram observations use evolving topic keys. Their revision histories contain the decisions made during the interview; the current observation body summarizes the terminal state.

| Key | Engram reference | Topic | Purpose |
| --- | --- | --- | --- |
| E1 | `Engram #2458` | `release/stability-evidence` | Early Stability Campaign evidence and journey decisions; 26 revisions. |
| E2 | `Engram #2459` | `workflow/executor-preference` | `agy`-only AI dispatch rule and deterministic-native-operation exception; 2 revisions. |
| E3 | `Engram #2460` | `architecture/stability-command` | Native command architecture and final approval; 64 revisions. |
| E4 | `Engram #2432` | Repository Coordinator | Neutral repository-wide coordination authority. |
| E5 | `Engram #2433` | Coordination Scope | Initial stable scope is one local repository on one machine. |
| E6 | `Engram #2438` | Collaboration channels | Provider-specific channels are optional and non-authoritative. |
| E7 | `Engram #2439` | Cross-Change coordination | Cross-Change effects are coordinated by Orchestrators. |
| E8 | `Engram #2442` | Stable journey | Stable release is defined by the concurrent-Change journey. |
| E9 | `Engram #2443` | Agent Teams | Claude Agent Teams integration is deferred until after core stability. |
| E10 | `Engram #2445` | Remediation architecture | Local Fix Changes and Dependencies are authoritative; external trackers are optional. |
| E11 | `Engram #2448` | Shared Memory | Remediation stops if its durable Defect Record cannot be stored. |
| E12 | `Engram #2454` | Integration Target fix | Integration worktrees must branch from the declared feature parent, not primary `HEAD`. |
| E13 | `Engram #2456` | Skill modularization | The Lucind AI orchestration skill and canonical glossary were modularized and committed. |

## Canonical vocabulary

| Term | Meaning |
| --- | --- |
| **Stability Campaign** | The complete release-validation process bound to one immutable candidate and composed of three consecutive successful Stability Trials. |
| **Stability Trial** | One complete execution of the canonical concurrent-Change journey inside a Campaign. |
| **Run** | The existing ordinary execution concept. It is not a Campaign or Trial. |
| **Trial Record** | Durable authority for one Trial's configuration, events, evidence, outcome, and cleanup proof. |
| **Stability Receipt** | Immutable content-addressed terminal evidence produced only by an approved Campaign. |
| **Test Actor** | Deterministic actor that records required Remediation Proposal and Promotion decisions without bypassing their gates. |

## Decision ledger — all 91 accepted decisions

### A. Stability evidence and canonical journey

1. **Use repeated evidence.** Stable core requires multiple complete journey executions, not one. One run proves possibility; repetition demonstrates stability. `[E1, E8]`
2. **Require three successes.** Certification requires exactly three consecutive successful executions. `[E1]`
3. **Reset operational state.** Every execution starts after removing prior temporary worktrees, branches, locks, leases, and execution state while preserving Git history and configuration. `[E1]`
4. **Exercise the full journey every time.** Every execution includes concurrent Changes, a discovered defect, separate Fix Change, selective blocking, approved resumption, correct Promotion, and crashed-Orchestrator recovery. `[E1, E8]`
5. **Reset after failure.** Any failed execution resets the consecutive-success count to zero. `[E1]`
6. **Freeze one candidate.** All three executions use the same immutable commit, versioned configuration, and assets. Any candidate change requires a new Campaign. `[E1]`
7. **Persist durable evidence.** Every execution records commit, executor, timestamps, commands, journey events, stage outcomes, and cleanup proof. `[E1]`
8. **Keep pass/fail authority repository-native.** Stability evidence is authoritative in Git common-dir; Engram provides semantic continuity but never decides pass/fail. `[E1, E4]`
9. **Treat cleanup as behavior.** Leftover worktrees, temporary branches, locks, leases, execution records, or processes make the execution fail. `[E1]`
10. **Preserve human-gate semantics deterministically.** A recorded Test Actor issues Remediation Proposal and Promotion decisions; it exercises rather than bypasses the gates. `[E1]`
11. **Use one canonical crash point.** All three executions inject the same failure; a broader crash matrix is deferred. `[E1]`
12. **Crash after result persistence.** The canonical crash occurs after a Lane result is durably stored and before Acceptance. `[E1]`
13. **Recover only through native authority.** Recovery uses lease expiry and explicit reclaim; no manual authority edits, lock deletion, or state repair are allowed. `[E1]`
14. **Prove selective blocking.** Change A discovers the defect and depends on Fix; unaffected Change B continues. `[E1, E7, E10]`
15. **Make ordering observable.** B promotes before Fix completes; A promotes only after Fix reaches A's required target and the Dependency is satisfied. `[E1]`
16. **Use distinct Integration Targets.** A and B declare different targets; Fix promotes to the target required by A, never the active checkout or B's target. `[E1, E12]`
17. **Prove no target contamination.** Git ancestry must show A's target contains Fix+A and B's target contains B without Fix or A. `[E1, E12]`
18. **Keep external trackers out of the Trial.** Create the durable Defect Record but no GitHub or other external issue. `[E1, E10]`
19. **Scope Defect Records to the Trial.** Each execution creates a Run-ID-bound Defect Record, closes it at completion, preserves it historically, and never reuses it as active state. `[E1, E11]`
20. **Distinguish pre-start from post-start executor failure.** `agy` unavailable before authority creation is not a Trial; failure after start fails the Trial and resets the count. `[E1]`
21. **Define atomic start.** An execution starts only when the Repository Coordinator atomically creates its record bound to the candidate and `agy`. `[E1, E4]`
22. **Define atomic success.** The counter increments only after atomic close validates the complete journey, ancestry, isolation, evidence, and cleanup. `[E1]`
23. **Require real concurrency.** A and B must overlap temporally; sequential execution is invalid. `[E1]`
24. **Require concrete overlap evidence.** Both Orchestrators must hold active ownership and have dispatched at least one Lane before either starts Promotion. `[E1]`
25. **Defer cross-provider proof.** V1 demonstrates provider-neutral coordination through independent Orchestrators using `agy`; cross-provider validation remains future work. `[E1, E2, E6, E9]`
26. **Use one launcher.** One human action creates authority and immutable internal A/B packets. Users do not manually copy two prompts. `[E1]`

### B. Product command, safety, and operator flow

27. **Make it a product capability.** Stability validation belongs in Lucind AI rather than only in an external harness. `[E3]`
28. **Separate command semantics.** Use `lucind-ai stability run`, distinct from ordinary `lucind-ai run`. `[E3]`
29. **Reject embedded-password security.** A proposed NIP was classified only as an accidental-run barrier and was later superseded by Decisions 31–33. V1 implements no NIP or secret storage. `[E3]`
30. **Run read-only preflight first.** Show candidate, temporary targets, cleanup scope, three Trials, and estimated `agy` consumption before mutation. `[E3]`
31. **Require a clean primary checkout.** Tracked, staged, or untracked changes block execution. The preflight—not a NIP—is the safety barrier. `[E3]`
32. **Require explicit confirmation.** After preflight, ask `yes/no` with `no` as the default. `[E3]`
33. **Keep V1 human-interactive.** Reject non-interactive stdin and provide no `--yes` or CI bypass. `[E3]`
34. **Close the preflight race.** After confirmation, revalidate unchanged `HEAD` and a clean worktree immediately before authority creation. `[E3]`
35. **Expose explicit recovery commands.** If the launcher dies, another `run` refuses and points to `stability resume` or `stability abort`. `[E3]`
36. **Guard resume.** `resume` shows its own preflight, validates the original commit and `agy` binding, and requires human confirmation. `[E3]`
37. **Make abort durable.** `abort` fails the Trial, resets the count, attempts complete cleanup, preserves evidence, and blocks new Campaigns while residue remains. `[E3]`
38. **Expose read-only status.** `stability status` shows candidate, active Campaign, stage, consecutive count, outcomes, and blocking residue. `[E3]`
39. **Keep stability authority distinct from ordinary Run.** Campaign lifecycle cannot be overloaded into the existing Run entity. `[E3]`
40. **Use precise names.** The outer three-execution process is a Stability Campaign; one journey is a Stability Trial; Run retains its existing meaning. The command remains `stability run`. `[E3]`
41. **Validate only current HEAD in V1.** Do not accept commit, branch, ref, or historical SHA selectors. `[E3]`
42. **Bind the binary to the candidate.** Preflight blocks unless the running Lucind AI build was produced exactly from candidate `HEAD`. `[E3]`
43. **Never self-rebuild.** A stale binary stops with exact `make install` guidance; the command does not compile or replace itself. `[E3]`
44. **Validate `agy` before consuming quota.** Preflight checks that `agy` is installed, authenticated, and available, then forecasts simultaneous and total dispatches. `[E2, E3]`

### C. Orchestrator topology and crash recovery

45. **Give Fix independent authority.** After recorded approval, the Repository Coordinator creates Fix and dispatches a third independent `agy` Orchestrator. `[E3, E7]`
46. **Crash B, not A.** The canonical failure affects unaffected Change B after its Lane result and before Acceptance, isolating recovery from remediation. `[E3]`
47. **Resume A under the same identity.** After Fix Promotion, a new `agy` dispatch continues A under its existing durable Orchestrator identity without reclaim. `[E3]`
48. **Replace B through explicit reclaim.** Wait for lease expiry, record previous/new identities and reason, then dispatch B's replacement through `agy`. `[E3]`
49. **Accelerate time without bypassing state.** Stability profile uses a short lease but traverses the unchanged native expiry/reclaim machine. `[E3]`
50. **Fix the lease at ten seconds.** V1 exposes no lease-duration override. `[E3]`
51. **Inject a real abrupt failure.** Kill only B's `agy` child process without cooperative shutdown while the Repository Coordinator remains alive. `[E3]`
52. **Detect process leaks.** Any surviving descendant process, including inherited MCP children, fails the Trial. `[E3]`
53. **Limit mutating execution to Linux.** Unsupported operating systems stop read-only before authority mutation. `[E3, E5]`

### D. Verdicts, receipts, and current certification

54. **Separate certification from delivery.** Passing persists verdict and evidence only; it does not tag, change versions, publish, or release. `[E3]`
55. **Produce an immutable receipt.** A passed Campaign creates content-addressed evidence linking candidate, build, three Trial Records, `agy`, and verdict. `[E3]`
56. **Bind execution environment.** Receipt includes exact `agy` version, Linux distribution/kernel, and architecture. `[E3]`
57. **Let the latest terminal Campaign decide current certification.** Earlier receipts remain historical, but a later failed Campaign for the same commit becomes current truth. `[E3]`
58. **Support machine-readable observation.** `stability status --json` exists in V1; mutating commands remain interactive. `[E3]`

### E. Deterministic fixture and dispatch budget

59. **Use ephemeral synthetic work.** A, B, and Fix modify generated fixture files only on temporary refs/worktrees, never permanent branches. `[E3]`
60. **Make tasks deterministic.** Every Agent receives a small exact modification with expected content and tree hash. `[E3]`
61. **Require real Agent edits.** `agy` Agents edit fixtures and produce normal result envelopes; the Coordinator prepares and verifies but does not perform their work. `[E2, E3]`
62. **Seed a real deterministic defect.** A's check fails on a known fixture precondition while B's check passes, forcing actual Defect Assessment and Remediation Proposal. `[E3]`
63. **Enforce the separate Fix path.** A's Write Scope excludes the defective file, so a separate Fix Change is the only valid remediation. `[E3]`
64. **Forbid hidden retries.** One dispatch is allowed per slot. Each Trial uses five dispatches—initial A, initial B, B replacement, Fix, resumed A—for fifteen per Campaign. `[E3]`
65. **Bound time at every level.** Use ten minutes per dispatch, 45 minutes per Trial, and 135 minutes per Campaign with no V1 overrides. Post-start timeout fails the Trial. `[E3]`
66. **Run Trials sequentially.** Trial N+1 starts only after verified cleanup of Trial N. Concurrency exists inside each Trial, not between Trials. `[E3]`

### F. Native storage and lifecycle

67. **Separate stability and ordinary-run stores.** Stability authority lives in Git common-dir; ordinary Run ledger remains at `<primary-root>/.lucind/lucind.db`; Trial Records link ordinary Run IDs. `[E3, E4]`
68. **Use a versioned native root.** Store stability authority beneath `<git-common-dir>/lucind-ai/stability/v1/`. `[E3]`
69. **Use transactional mutable state and inspectable immutable artifacts.** SQLite/WAL stores Campaign and Trial lifecycle; receipts are content-addressed JSON. `[E3]`
70. **Allow one active Campaign per repository.** Enforce this in the authority-creation transaction before refs, worktrees, or dispatches. `[E3]`
71. **Stop after the first failed Trial.** Clean up, close the Campaign as failed, and require a new Campaign for another attempt. `[E3]`
72. **Represent unresolved cleanup honestly.** Cleanup failure leaves non-terminal `blocked_cleanup`; successful later cleanup closes the Campaign as failed. `[E3]`
73. **Make abort an idempotent cleanup retry.** From `blocked_cleanup`, `abort` retries only pending cleanup and never dispatches work. `[E3]`
74. **Preserve evidence before deleting infrastructure.** Archive logs, envelopes, refs/hashes, and process evidence before removing worktrees/refs; preservation failure blocks cleanup. `[E3]`
75. **Retain history indefinitely in V1.** Do not automatically prune Campaigns, Trial Records, or receipts. `[E3]`
76. **Persist privacy-safe evidence.** Store bounded sanitized logs without credentials, environment values, usernames, or absolute paths, plus hashes of validated raw payloads; discard raw output. `[E3]`
77. **Keep V1 CLI-only.** Control Room UI support is deferred. `[E3]`

### G. Verification, executor boundary, and final approval

78. **Use two verification layers.** Fake-executor tests cover deterministic state/failure behavior; only one real three-Trial `agy` Campaign proves acceptance, outside `go test ./...`. `[E3]`
79. **Fail on unexpected interaction.** Any unplanned question, ambiguity, or extra human decision inside a Trial fails it. `[E3]`
80. **Apply `agy` only to AI work.** Every AI Orchestrator/Agent dispatch uses `agy`; deterministic Git, check, SQLite, and process operations execute natively. `[E2]`
81. **Require a healthy candidate before mutation.** Preflight runs the repository's complete native baseline and blocks on failure. `[E3]`
82. **Reuse the canonical check.** Invoke `lucind-ai check`; do not hardcode `go test ./...` into Stability Campaign logic. `[E3]`
83. **Check again after cleanup.** Run `lucind-ai check` after Trial 3 cleanup before issuing an approved receipt. `[E3]`
84. **Use a non-personal fixture identity.** Temporary commits use per-command `Lucind Stability <stability@local.invalid>` without changing Git configuration. `[E3]`
85. **Embed and digest the journey.** Templates, checks, and packets are versioned with the binary; Campaign records their digest. `[E3]`
86. **Pin the model.** Packets explicitly select `gemini-3.7-flash-high`; preflight blocks if unavailable; receipt records it. `[E2, E3]`
87. **Detect environment drift before every Trial.** Revalidate commit, build, `agy` version, model, and environment; drift fails the Campaign. `[E3]`
88. **Treat normal interruption as abort.** Ctrl-C stops dispatches, fails the Trial, cleans state, and closes the Campaign. `[E3]`
89. **Fail closed during resume.** Reconcile processes, leases, refs, and worktrees first; ambiguous effects prohibit resume and leave abort as the only safe path. `[E3]`
90. **Record the architecture decision.** ADR 0001 is required repository context for implementation and review. `[E3]`
91. **Design approved.** Shared understanding was confirmed; implementation may enter specification/planning, but no real Campaign runs before the command is implemented, verified, reviewed, and frozen. `[E3]`

## Product requirements

### R1 — Campaign admission

`stability run` must perform a side-effect-free preflight that verifies Linux, primary checkout, clean status, exact HEAD/build identity, canonical baseline, `agy` and pinned model availability, no active Campaign, and forecasted cost. It must require an interactive `yes` after displaying the complete plan, then revalidate the candidate before atomically creating authority.

### R2 — Deterministic three-Trial scheduler

One Campaign must execute up to three Trials sequentially. A Trial begins only after the prior Trial's cleanup is proven. A failed Trial terminates the Campaign. Success increments the consecutive count atomically; count three advances to terminal verification.

### R3 — Real concurrent Changes

Each Trial must create distinct ephemeral Integration Targets and dispatch A and B concurrently through `agy`. Both must hold ownership and dispatch a Lane before Promotion begins. All AI packets pin `gemini-3.7-flash-high` and use deterministic embedded fixture work.

### R4 — Defect and remediation flow

A's real fixture check must expose a pre-seeded defect outside A's Write Scope. A must persist a Defect Record and produce a Remediation Proposal. The Test Actor approves it. The Repository Coordinator creates a separate Fix Change with its own `agy` Orchestrator and Dependency to A. B remains eligible to continue.

### R5 — Crash and ownership recovery

B must be killed abruptly after result persistence and before Acceptance. Its ten-second lease must expire naturally. A replacement Orchestrator must acquire ownership through explicit reclaim, preserve the existing result, avoid duplicate work, accept it, and promote B before Fix completes. Process-tree survivors fail the Trial.

### R6 — Fix, resumption, and Promotion

Fix modifies only its authorized fixture scope and promotes to A's declared target. A resumes through a new `agy` dispatch under the same Orchestrator identity, re-runs its check, and promotes only after the Dependency is satisfied. The Test Actor records all Promotion decisions.

### R7 — Content-bound verification

Each Trial must validate expected fixture tree hashes, event ordering, real temporal overlap, Run IDs, Defect Record lifecycle, ownership lineage, target ancestry, no target contamination, process cleanup, worktree/ref cleanup, and absence of active leases or execution records.

### R8 — Durable authority and recovery

SQLite/WAL under `<git-common-dir>/lucind-ai/stability/v1/` must persist recoverable state. Only one active Campaign may exist. `status` is read-only; `resume` reconciles exact effects before continuing; `abort` is idempotent and never redispatches cleanup work. Ambiguity fails closed.

### R9 — Evidence and receipt

Evidence must be preserved before cleanup, sanitized, bounded, and content-bound. A passed Campaign requires three Trial Records plus a passing final `lucind-ai check`. Its receipt must bind candidate, binary build, fixture digest, executor/model versions, OS/kernel/architecture, Trial records, and verdict.

### R10 — Delivery boundary

A passed Campaign must not commit to permanent branches, tag, change versions, publish, create issues, push, or release. Delivery remains a separate human action.

## Observable acceptance scenarios

1. **Preflight rejects dirt:** Given any staged, tracked, or untracked change, `stability run` reports the exact blocker and creates no stability database row, ref, worktree, or dispatch.
2. **Preflight rejects stale binary:** Given a build not matching HEAD, the command stops with `make install` guidance and no mutation.
3. **Declined confirmation is inert:** Given valid preflight and response other than explicit `yes`, no authority or fixture state is created.
4. **Concurrent start is proven:** A and B have overlapping ownership and dispatched Lanes before either Promotion.
5. **Selective dependency works:** A blocks on Fix while B continues.
6. **Crash preserves result:** Killing B after result persistence never repeats B's original task.
7. **Early takeover is rejected:** B replacement cannot acquire ownership during the ten-second lease.
8. **Explicit reclaim succeeds:** After expiry, replacement ownership records old/new identities and reason.
9. **Independent Promotion wins first:** B promotes before Fix completes.
10. **Fix targets A correctly:** Fix lands on A's required target, never B's target or current checkout.
11. **A resumes without reclaim:** A continues under its original durable Orchestrator identity.
12. **Ancestry proves isolation:** A target contains Fix+A; B target contains only B's intended fixture changes.
13. **Process leaks fail:** Any surviving child process prevents successful Trial close.
14. **Cleanup residue blocks:** Any residual ref, worktree, lease, process, or active record moves Campaign to `blocked_cleanup`.
15. **Abort retries cleanup only:** Repeated abort from `blocked_cleanup` is idempotent and performs zero AI dispatches.
16. **No retries hide instability:** Empty, malformed, timed-out, or failed slot result immediately fails the Trial.
17. **Trial failure stops Campaign:** Trial 2 failure prevents Trial 3 from starting and resets certification progress.
18. **Resume is content-aware:** Known committed effects continue without replay; ambiguous effects block resume.
19. **Receipt requires final baseline:** Three successful Trials without a passing final `lucind-ai check` cannot produce approval.
20. **Latest result governs:** A later failed Campaign supersedes an older pass as current certification while preserving both histories.

## Architecture plan

### Suggested packages

The SDD design phase must validate names against the current codebase, but the intended seams are:

```text
cmd/lucind-ai/                 CLI routing and human interaction
internal/stability/           Campaign/Trial domain state machine
internal/stability/store/     Git-common-dir SQLite/WAL authority
internal/stability/fixture/   embedded deterministic templates and packets
internal/stability/process/   Linux process-group launch, kill, and survivor proof
internal/stability/evidence/  sanitization, hashing, Trial Records, receipts
internal/stability/reconcile/ resume/abort inspection and cleanup planning
```

Avoid turning `cmd/lucind-ai/cli.go` into the domain owner. CLI parses and projects; `internal/stability` owns invariants.

### State model

Candidate Campaign states:

```text
preflight (not persisted)
    │ explicit yes + atomic admission
    ▼
running
    ├── Trial failed + cleanup complete ──► failed
    ├── cleanup/evidence unresolved ─────► blocked_cleanup
    ├── launcher lost ───────────────────► running (requires resume or abort)
    └── three Trials + final check ──────► passed

blocked_cleanup
    └── idempotent abort cleanup ────────► failed
```

Candidate Trial states should represent admission, concurrent dispatch, defect assessment, remediation approval, Fix creation, B crash, lease wait, B reclaim, B Promotion, Fix Promotion, A resumption, A Promotion, evidence capture, cleanup, and terminal outcome. Each transition must have an idempotency key and persisted preconditions.

### Transaction boundaries

- Campaign creation and one-active-Campaign enforcement are one transaction.
- Trial start binds all immutable inputs before dispatch.
- Every externally visible effect is recorded before the next effect becomes eligible.
- Evidence capture precedes destructive cleanup.
- Trial successful close and Campaign counter increment are one transaction.
- Receipt write uses canonical JSON, content hashing, atomic materialization, and readback verification before Campaign becomes passed.

## Suggested SDD capability decomposition

1. **stability-command-contract** — CLI grammar, preflight, interaction, status JSON, platform gating.
2. **stability-authority-store** — common-dir resolution, SQLite schema, transactions, one-active constraint, migrations.
3. **stability-campaign-state-machine** — Campaign/Trial transitions, budgets, counters, failure and blocked cleanup.
4. **stability-fixture-journey** — embedded templates, packets, deterministic checks, Write Scopes, expected trees.
5. **stability-process-recovery** — Linux process groups, abrupt kill, ten-second lease, reclaim, survivor detection.
6. **stability-remediation-flow** — Defect Record, Test Actor gates, Fix Change, Dependency, A resumption.
7. **stability-evidence-receipt** — sanitization, hashes, Trial Records, canonical content-addressed receipt.
8. **stability-resume-abort** — reconciliation, idempotent cleanup, ambiguity handling.
9. **stability-driven-acceptance** — real `agy` Campaign gate and release-facing evidence report.

## SDD fan-out implementation topology

Fan-out is for context compression and disjoint work only. One SDD Change owns the feature: `native-stability-campaign`.

### Planning waves

| Wave | Parallel lanes | Completion gate |
| --- | --- | --- |
| P1 | CLI/current-run mapper; ledger/worktree/feature mapper; executor/process mapper; testing/fixture mapper | One synthesized exploration with every cited symbol verified. |
| P2 | Spec lenses grouped by capability; design risk lenses for storage, process lifecycle, and state machine | Canonical spec/design with no contradictory authority. |
| P3 | Task decomposition by package seam and dependency | Review Workload Forecast and disjoint Write Scopes approved. |

### Implementation waves

The exact paths come from design, but the dependency shape should be:

| Wave | Work units | Parallelism rule |
| --- | --- | --- |
| I1 | Domain types/state machine; storage schema; fixture schemas/templates | Parallel only when generated/schema files and Go package paths are disjoint. |
| I2 | Store implementation; evidence canonicalization; fake-executor harness | Parallel behind I1 contracts. |
| I3 | Linux process control; Campaign scheduler; resume/abort reconciliation | Separate Write Scopes; synthesize before CLI wiring. |
| I4 | CLI run/status/resume/abort; status JSON; end-to-end fake-executor tests | One writer owns CLI wiring to avoid conflicts. |
| I5 | Full integration, docs, install/version validation, native review | Sequential candidate normalization and freeze. |
| I6 | Real Campaign acceptance | Exactly one real Campaign after all prior gates pass. |

No implementation lane may modify overlapping files concurrently. Every AI lane uses `agy` with `gemini-3.7-flash-high`. Deterministic Git, SQLite, checks, builds, and process commands remain native.

## Verification plan

### Strict TDD surfaces

- State transition tables, invalid transitions, idempotency, and counter reset.
- SQLite migration, WAL behavior, unique-active-Campaign constraint, crash reopening, and content-bound reads.
- Clean-tree, HEAD/build, platform, TTY, `agy`, model, baseline, and post-confirmation drift preflight.
- Exact CLI exit behavior and stable JSON schemas.
- Deterministic fixture digests, Write Scope enforcement, expected tree hashes, and ancestry assertions.
- Failure classification before and after authoritative start.
- Evidence sanitization and secret/path redaction.
- Receipt canonicalization, hashing, atomic write/readback, and latest-certification selection.
- Resume reconciliation matrices and abort cleanup idempotency.
- Linux process-group kill, child survivor detection, and ten-second lease behavior with controllable clocks where possible.

### Required command evidence

- Focused package tests during each work unit.
- `go test ./...` or repository-prescribed focused suites during implementation.
- `lucind-ai check` as the canonical pre/post Campaign baseline.
- `make install` after binary changes.
- `lucind-ai -v` proving the installed binary matches candidate HEAD.
- Native review receipt when review mode is enabled.
- One real `lucind-ai stability run` using 15 `agy` dispatches only after the immutable candidate is ready.

### Real Campaign acceptance

The final real Campaign is long, quota-consuming work. Forecast it before launch. It must not begin unless:

- SDD apply and verify are complete.
- All fake-executor and Linux process tests pass.
- The source is normalized and frozen.
- The installed binary matches HEAD.
- Native review and applicable delivery gates allow the exact candidate.
- The operator reviews preflight and explicitly enters `yes`.

## Non-goals for V1

- NIP, password, secret management, or authentication.
- `--yes`, CI execution, or unattended mutation.
- macOS or Windows mutating execution.
- Cross-provider Campaigns.
- Configurable model, lease, timeout, or Trial count.
- Historical commit/ref selectors.
- Automatic retry of any AI dispatch.
- Control Room UI.
- Automatic evidence pruning.
- External issue creation.
- Tagging, version bumping, pushing, PR creation, publishing, or releasing.
- Migration of the existing ordinary Run ledger.

## Delivery and rollback boundaries

- Implementation commits must be reviewable work units with tests beside behavior.
- If workload exceeds the configured review budget, use the SDD delivery strategy and chained PR skill; never silently create an oversized PR.
- Storage schema work must include forward-only migration and corruption/fail-closed tests.
- Before the first public use, rollback means removing the new CLI route and stability packages while leaving historical common-dir data untouched and ignored by older binaries.
- After a passed real Campaign exists, receipt compatibility becomes part of the product contract; schema evolution must preserve validation of V1 history.
- No commit, push, PR, tag, or release occurs without explicit human instruction.

## Completion checklist

- [ ] Proposal, specs, design, and tasks cover every decision Q1–Q91.
- [ ] Every requirement has observable scenarios and tests.
- [ ] CLI remains a projection over a deep `internal/stability` module.
- [ ] Every AI dispatch uses `agy` and `gemini-3.7-flash-high`.
- [ ] Campaign authority lives under Git common-dir; ordinary ledger is not migrated.
- [ ] Fake-executor tests cover all failure and recovery paths.
- [ ] Linux process-group behavior is proven.
- [ ] `make install` and exact version check pass.
- [ ] Native review allows the frozen candidate when enabled.
- [ ] A real three-Trial Campaign passes and emits a verifiable receipt.
- [ ] The real Campaign leaves no process, worktree, ref, lease, or active-record residue.
- [ ] SDD verification passes against the canonical specs.
- [ ] Archive records final evidence without stale intermediate claims.

## Follow-up prompt for Claude

Copy the complete prompt below into a fresh Claude session rooted at this repository.

```text
You are the owning Orchestrator for building the Lucind AI native Stability Campaign feature.

OUTCOME
Build `lucind-ai stability run`, `status [--json]`, `resume`, and `abort` through one complete SDD workflow using Lucind AI fan-out. Continue automatically after the mandatory SDD Session Preflight until implementation, verification, native review, real Campaign acceptance, and archive are complete. Do not merely explore or propose: BUILD the approved feature.

PRODUCT AUTHORITIES — READ FIRST
1. `lucind-ai-stability-run-sdd-master-plan.md` — complete Q1–Q91 implementation contract.
2. `CONTEXT.md` — canonical domain vocabulary.
3. `docs/adr/0001-native-stability-campaign.md` — architectural decision and V1 boundaries.
4. `plugin/claude-code/skills/lucind-ai/SKILL.md` and its progressive references — orchestration contract.

If any generated artifact contradicts those authorities, stop the dependent phase and correct the artifact. Never silently reinterpret a decision.

SDD SESSION PREFLIGHT — HARD GATE
Before any SDD command, collect the four grouped choices exactly once: execution pace, artifact store, delivery strategy, and changed-line review budget. Recommend Automatic pace because this is a one-shot build, but never choose missing values for the human. After the preflight is complete, cache it and execute phases automatically with gatekeeping. If the user selects Interactive, honor it instead of one-shot continuation.

SDD ROUTE
1. Detect the current project and query Engram for `sdd-init/lucind-ai`.
2. If initialization is absent, run SDD init first.
3. Resolve artifact store before using any native dispatcher. Never invoke the OpenSpec-only dispatcher for an Engram-only Change.
4. Reuse an equivalent active Change if native status proves one exists; otherwise create one Change named `native-stability-campaign`.
5. Execute explore → propose → spec → design → tasks → apply → verify → archive in dependency order.
6. Run the automatic gatekeeper after every phase. A failed phase gets one corrective retry only; never advance with a failed artifact.
7. Before apply, enforce the Review Workload Guard and the selected delivery strategy.

EXECUTION ROUTE
- Use the repository's installed `lucind-ai` binary to coordinate SDD fan-out.
- Every AI Orchestrator and Agent dispatch MUST use executor `agy` and model `gemini-3.7-flash-high`.
- Deterministic Git, SQLite, tests, checks, builds, and process operations run natively without an AI executor.
- Check `lucind-ai -v` before dispatch. After any binary change run `make install`; never dispatch with a stale binary or ad-hoc temporary build.
- One Orchestrator owns this Change. Fan-out only independent read/planning lenses or implementation lanes with disjoint declared Write Scopes and explicit dependency waves.
- Never run overlapping writers. Synthesize fan-out results before dependent phases.

SKILLS TO LOAD BEFORE MATCHING WORK
- `/home/lanzerdev/.agents/skills/tdd/SKILL.md`
- `/home/lanzerdev/.config/opencode/skills/go-testing/SKILL.md`
- `/home/lanzerdev/.config/opencode/skills/work-unit-commits/SKILL.md`
- `/home/lanzerdev/.config/opencode/skills/chained-pr/SKILL.md` when the workload guard triggers
- The exact phase skill under `/home/lanzerdev/.config/opencode/skills/sdd-<phase>/SKILL.md`
- Resolve any additional project skill from the Engram `skill-registry` observation or `.atl/skill-registry.md`.

PLANNING FAN-OUT
- Exploration lanes: current CLI/run flow; ledger/feature/worktree authority; agy/process lifecycle; fixture/testing seams.
- Spec lanes: command/preflight; Campaign authority/state machine; fixture/remediation; crash/recovery; evidence/receipt.
- Design lenses: SQLite/common-dir storage, Linux process groups, idempotent recovery, deep-module API, and test seams.
- Synthesize each wave into one canonical artifact. Cite current source symbols and tests; do not trust stale paths.

IMPLEMENTATION REQUIREMENTS
- Implement every Q1–Q91 decision from the master plan.
- Keep CLI parsing and projection thin; place lifecycle invariants behind a deep `internal/stability` module.
- Use SQLite/WAL under `<git-common-dir>/lucind-ai/stability/v1/` for mutable authority and content-addressed canonical JSON for terminal receipts.
- Preserve the existing ordinary ledger at `<primary-root>/.lucind/lucind.db` and link Run IDs from Trial Records.
- Embed and digest deterministic fixture templates/checks/packets.
- Implement exactly five agy dispatch slots per Trial, no automatic retries, three sequential Trials, ten-second lease, ten-minute dispatch budget, 45-minute Trial budget, and 135-minute Campaign budget.
- Implement Linux process-group isolation, abrupt B crash after result persistence/before Acceptance, descendant-survivor detection, explicit lease reclaim, and exact ownership evidence.
- Implement deterministic Test Actor decisions without bypassing Remediation Proposal or Promotion gates.
- Implement preflight, post-confirmation revalidation, status text/JSON, resume reconciliation, idempotent abort, `blocked_cleanup`, evidence-before-cleanup, sanitization, receipt hashing, and latest-certification selection.

STRICT TDD AND VERIFICATION
- If SDD init says `strict_tdd: true`, follow strict TDD without fallback.
- Build fake-executor tests for every state transition, failure class, timeout, retry prohibition, counter reset, cleanup residue, resume ambiguity, store crash/reopen, canonical receipt, and JSON contract.
- Add Linux process-group tests that prove abrupt child death and descendant detection.
- Add CLI tests for clean-tree, TTY, confirmation default, stale binary, unsupported platform, agy/model preflight, and no-side-effect failures.
- Run focused tests per work unit, then repository verification.
- Run `make install` after binary changes and prove installed `lucind-ai -v` matches candidate HEAD.

NATIVE REVIEW AND REAL ACCEPTANCE
- Normalize all source-mutating formatting before candidate freeze.
- Follow native review status and exact next transitions when review mode is enabled; never select lenses or synthesize PASS yourself.
- Do NOT execute the real Stability Campaign until apply, SDD verify, all deterministic tests, install/version validation, candidate freeze, and native review allow the exact candidate.
- Before the real Campaign, report the long-work forecast: 15 `agy` dispatches, three sequential Trials, up to 135 minutes, temporary refs/worktrees/processes, and final cleanup.
- The real Campaign must run through the new public command and its human preflight confirmation. Do not simulate its receipt.
- A valid final result requires three clean Trials, a passing final `lucind-ai check`, no residue, and a locally verifiable content-addressed receipt.

NON-GOALS — DO NOT ADD
- NIP/password/authentication, `--yes`, CI bypass, Control Room UI, non-Linux mutation, cross-provider mode, configurable Trial/model/lease/timeouts, historical selectors, AI retries, evidence pruning, external issues, automatic tags/versioning/releases, or ordinary-ledger migration.

DELIVERY BOUNDARY
- Do not commit, push, open a PR, tag, publish, or release unless the human explicitly asks for that delivery action.
- Keep work-unit commits and chained-PR boundaries planned in tasks even if delivery is not requested.

COMPLETION CONTRACT
Finish only when SDD archive succeeds after implementation and independent verification. Report:
1. SDD artifact references and archive identity.
2. Changed files grouped by work unit.
3. Focused and full test commands with exact outcomes.
4. Installed binary version evidence.
5. Native review receipt/gate result when applicable.
6. Real Stability Campaign ID, three Trial IDs, terminal receipt digest, and zero-residue proof — or an explicit statement that the real Campaign did not run and why.
7. Remaining risks and deferred V1 non-goals.
8. Git status and an explicit statement that no delivery action occurred unless the human requested it.
```
