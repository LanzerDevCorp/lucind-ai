```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:d55fe21ab4c1687652ada108d6a68c28e5a3173d9316e1acffc683e309e8a957
verdict: pass_with_warnings
blockers: 0
critical_findings: 0
requirements: 8/8
scenarios: 13/13
test_command: go test ./... -race -count=1
test_exit_code: 0
test_output_hash: sha256:ae20e8808a93619430bb02164357626482b9382af4e7ad5c435a7107418baad6
build_command: CGO_ENABLED=0 go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: acceptance-verifier
**Version**: N/A (new capability spec)
**Mode**: Strict TDD
**Branch/commit**: feature/acceptance-verifier @ 5bb7a15
**Artifact store**: hybrid (OpenSpec + Engram)

### Completeness
| Metric | Value |
|--------|-------|
| Tasks total | 16 |
| Tasks complete | 16 |
| Tasks incomplete | 0 |

All 16 tasks in `tasks.md` are checked. Task/code cross-check: every phase has corresponding
implementation plus genuine tests (see matrix). Task 4.2's read-only claims were independently
re-verified against source (schema triggers, atomic insert, exact cache, frozen-scope validation).

### Build & Tests Execution

**Build**: PASS — `CGO_ENABLED=0 go build ./...` exit 0, no output.

**Tests**: PASS — `go test ./... -race -count=1` exit 0.
- 24 packages discovered; 23 compiled+passed with tests; `cmd/plugincontent` `[no test files]`.
- Race detector enabled. Slowest: `internal/run` 68.9s, `cmd/lucind-ai` 70.2s, `internal/ledger` 30.5s.
- Change-relevant packages: `internal/accept` ok 10.4s, `internal/ledger` ok 30.5s,
  `internal/integrate` ok 11.6s, `internal/run` ok 68.9s, `cmd/lucind-ai` ok 70.2s, `internal/feature` ok 6.3s.
- Known-flaky `TestExecuteBatchAppliesPerLaneDeadlineIndependently` did NOT fail this run.

**Coverage**: Not enforced (`coverage_threshold: 0`). Coverage report not run separately; informational only.

### Spec Compliance Matrix
| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| Exact Acceptance Binding | Record the complete binding | `internal/accept/accept_test.go > TestVerifierPersistsCompleteReceiptAndReusesExactBinding`; `internal/ledger/acceptance_test.go > TestAcceptanceRowsRejectUpdateAndDelete` | ✅ COMPLIANT |
| Exact Acceptance Binding | Reject an identity mismatch | `internal/accept/accept_test.go > TestVerifierRejectsRootOrObjectIdentityMismatch`, `TestVerifierRejectsInvalidEvidenceWithoutReceipt`; `internal/ledger/acceptance_test.go > TestAcceptanceReceiptInsertAndExactReuse` | ✅ COMPLIANT |
| Fail-Closed Mechanical Criteria | Reject invalid result evidence | `internal/accept/accept_test.go > TestVerifierRejectsInvalidEvidenceWithoutReceipt` (schema/packet/hard-stop/criterion subtests + no-receipt assert) | ✅ COMPLIANT |
| Fail-Closed Mechanical Criteria | Reject scope or check failure | `internal/accept/accept_test.go > TestVerifierRejectsInvalidEvidenceWithoutReceipt` (undeclared/out-of-scope), `TestVerifierCheckFailureAndForeignIsolationPersistNoReceipt` (exit 7) | ✅ COMPLIANT |
| Frozen Candidate Verification | Primary checkout changes concurrently | `internal/accept/accept_test.go > TestVerifierUsesFrozenDetachedCandidateDespitePrimaryState` (staged / commit-a / empty-index) | ✅ COMPLIANT |
| Owned Isolation and Cleanup | Clean owned isolation | `internal/accept/accept_test.go > TestVerifierPersistsCompleteReceiptAndReusesExactBinding` (Cleanup=="removed"), `TestVerifierCleanupMarkerMismatchRejectsAndPreservesIsolation` | ✅ COMPLIANT |
| Owned Isolation and Cleanup | Preserve foreign worktrees | `internal/accept/accept_test.go > TestVerifierCheckFailureAndForeignIsolationPersistNoReceipt`, `TestVerifierCleanupMarkerMismatchRejectsAndPreservesIsolation` | ✅ COMPLIANT |
| Durable Receipt and Exact Cache Reuse | Persist successful acceptance | `internal/ledger/acceptance_test.go > TestAcceptanceReceiptInsertAndExactReuse`; `internal/accept/accept_test.go > TestVerifierPersistsCompleteReceiptAndReusesExactBinding` | ✅ COMPLIANT |
| Durable Receipt and Exact Cache Reuse | Reuse only an exact receipt | `internal/accept/accept_test.go > TestVerifierBindingDifferencePreventsCacheReuse` (env change -> checks==2, new receipt), `TestVerifierPersistsCompleteReceiptAndReusesExactBinding` (idempotent reuse) | ✅ COMPLIANT |
| Receipt-Gated CLI Success | Successful command | `cmd/lucind-ai/cli_test.go > TestAcceptRequiresExactReceiptAndRendersMechanicalEvidenceOnly` (exit 0, prints receipt id) | ✅ COMPLIANT |
| Receipt-Gated CLI Success | Receipt absent | `cmd/lucind-ai/cli_test.go > TestAcceptRequiresExactReceiptAndRendersMechanicalEvidenceOnly` (verifier error -> nonzero), `TestAcceptNoFlags` | ✅ COMPLIANT |
| No Promotion Authority | Accepted candidate remains unpromoted | `internal/accept/accept_test.go > TestVerifierPersistsCompleteReceiptAndReusesExactBinding` (show-ref before==after); `cmd/lucind-ai/cli_test.go > TestAcceptRequiresExactReceiptAndRendersMechanicalEvidenceOnly` (refs unchanged) | ✅ COMPLIANT |
| Mechanical Evidence Is Not Semantic Approval | Present an acceptance receipt | `cmd/lucind-ai/cli_test.go > TestAcceptRequiresExactReceiptAndRendersMechanicalEvidenceOnly` (asserts "mechanical evidence" / "qualitative approval remains separate", rejects "semantically approved") | ✅ COMPLIANT |

**Compliance summary**: 13/13 scenarios compliant; 8/8 requirements implemented and covered by a passing test.

### Correctness (Static Evidence)
| Requirement | Status | Notes |
|------------|--------|-------|
| Exact Acceptance Binding | ✅ Implemented | `internal/accept/accept.go:205-226` binds run/lane/packet/digest, base+candidate commit+tree, allowed-paths hash, check-policy hash (version+timeout+script blob), environment hash (ordered allowlist). `bindingHash` covers every field with versioned length-prefixed SHA-256. Rows immutable via schema v9 triggers (`internal/ledger/schema.go:415-422`). |
| Fail-Closed Mechanical Criteria | ✅ Implemented | `validateResultAndScope` (`accept.go:159-203`): result hash match, `result.Read` schema, status=="done", packet id match, fired hard stop -> reject, unmet done criterion -> reject, ExternalChanges -> reject, NUL-safe git diff must equal declared files, every path `packet.PathInScope`. No receipt written on any rejection (early return before `InsertAcceptanceReceipt`). |
| Frozen Candidate Verification | ✅ Implemented | `validateObjects` rev-parses `^{commit}`/`^{tree}` for base+candidate against the persisted `LaneCandidate`; isolation is `git worktree add --detach <candidate>`; `createOwnedIsolation` asserts detached HEAD == candidate and clean `status --porcelain`. Never reads primary index/worktree. |
| Owned Isolation and Cleanup | ✅ Implemented | Sibling `<root>-worktrees/accept-<lane>-<uuid>`, JSON owner marker binding root/path/candidate/token. `cleanupOwnedIsolation` validates marker equality and HEAD before `worktree remove --force`; marker missing/mismatch -> error + foreign worktree preserved. Cleanup error is surfaced as `accept: cleanup failed` and blocks the receipt. `receipt.Cleanup` records outcome. |
| Durable Receipt and Exact Cache Reuse | ✅ Implemented | `InsertAcceptanceReceipt` (`internal/ledger/acceptance.go:147-175`): single tx, `INSERT OR IGNORE`, re-read by unique `binding_hash`, full struct equality required before commit, else `ErrAcceptanceBindingMismatch`. Cache path in `Verify` (`accept.go:88-95`) reuses only when `cached.Binding == binding && cached.ResultHash == candidate.ResultHash`. |
| Receipt-Gated CLI Success | ✅ Implemented | `runAccept` (`cmd/lucind-ai/cli.go:641-688`): thin adapter, parses only `--run`/`--lane`, opens ledger, calls `accept.Verifier.Verify`, exit 1 on any error, exit 0 + `renderAcceptanceReceipt` only on a returned receipt. |
| No Promotion Authority | ✅ Implemented | `AcceptanceRequest` carries only run/lane IDs. `Verify` performs no ref write; only `git worktree add/remove/prune` on an ephemeral detached path. No Promotion/CAS symbol referenced in `internal/accept`. Tests assert `show-ref` unchanged. |
| Mechanical Evidence Is Not Semantic Approval | ✅ Implemented | `renderAcceptanceReceipt` prints `meaning: mechanical evidence only; qualitative approval remains separate`. Package doc comment states it "never ... represents its receipt as semantic approval". No qualitative-review dependency in `internal/accept`. |

### Coherence (Design)
| Decision | Followed? | Notes |
|----------|-----------|-------|
| Request only run/lane; persisted identity, not caller refs | ✅ Yes | `AcceptanceRequest{RunID,LaneID}`; identity from `ledger.GetLaneCandidate`. |
| `run.Execute` atomically freezes packet/base/candidate identity before barrier | ✅ Yes | `internal/run/run.go` `setDoneCandidate` on `status == lane.Done`, inside `persistCtx`; failure -> `freeze done candidate` error and lane failure (`TestExecuteDoneRejectsAbsentCandidateIdentity`). |
| One `Verifier.Verify(ctx, request)` interface hiding Git/scope/check/lifecycle/cleanup/storage | ✅ Yes | Single exported method; seams (`loadCandidate`, `check`, `now`, `newID`) are unexported test injection points. |
| Schema v9 immutable `lane_candidates` + `acceptance_receipts` with abort triggers | ✅ Yes | `migrateV8ToV9DDL`; `schemaVersion = 9`; `TestSchemaV9AddsImmutableAcceptanceEvidence` asserts version and columns. |
| `INSERT OR IGNORE` + exact comparison for idempotency | ✅ Yes | Both `SetDoneCandidate` and `InsertAcceptanceReceipt`. |
| Isolation marker binds root/path/candidate; cleanup validates marker + Git path/HEAD | ✅ Yes | `ownerMarker`, `createOwnedIsolation`, `cleanupOwnedIsolation`. |
| `integrate.Check` remains the `lucind-checks.sh` seam; preserve interface | ✅ Yes | `Check(ctx, worktreePath)` signature preserved; hardened with env allowlist, process group, owned deadline, escalation, bounded output (`internal/integrate/integrate.go`). New `CheckPolicySnapshot` exported for policy/env hashing. |
| Paths reject empty/absolute/traversal before normalize/dedupe/sort | ✅ Yes | `normalizeAllowedPaths` in `run.go`. |
| Rollback removes callers/adapter, retains audit rows; no ref change | ✅ Yes (design intent) | Additive v9 migration; immutable rows/triggers persist independent of adapter. See WARNING 1 re: extra unrelated callers added. |
| Out of scope: lease-recovery redesign, promotion/CAS changes, broad docs, plugin metadata | ⚠️ Partially violated | See WARNING 1 and WARNING 2. Plugin manifests + version 2.0.9 unchanged (verified). |

### TDD Compliance
| Check | Result | Details |
|-------|--------|---------|
| TDD Evidence reported | ✅ | `apply-progress.md` contains the TDD Cycle Evidence table (16 rows). |
| All tasks have tests | ✅ | 16/16 tasks map to test files that exist and pass. |
| RED confirmed (tests exist) | ⚠️ | Test files verified present for every behavioral task. Historical RED transcripts for tasks 1.1–3.3 are unavailable (implementation predates the verification-only batch); apply-progress states this honestly rather than fabricating. Tasks 4.1–4.2 are verification-only (no RED/GREEN cycle). |
| GREEN confirmed (tests pass) | ✅ | Full `-race` suite exit 0; every change-relevant package green. |
| Triangulation adequate | ✅ | Rejection tests are table-driven with 3–6 distinct cases each (schema/packet/hard-stop/criterion/undeclared/out-of-scope; relative/foreign root + tree mismatch; staged/commit-a/empty-index; exit 7 + foreign isolation). Cache: hit vs env-difference-forces-re-verify. |
| Safety Net for modified files | ⚠️ | `internal/integrate`, `internal/run`, `cmd/lucind-ai`, `internal/ledger` are modified files; apply-progress marks safety net "Inherited" rather than showing a captured pre-edit baseline. Current full suite passing is the compensating evidence. |

**TDD Compliance**: 4/6 checks fully green, 2 partial (historical-evidence gaps, not defects).

### Test Layer Distribution
| Layer | Tests | Files | Tools |
|-------|-------|-------|-------|
| Unit | ~a few (hash/path helpers exercised indirectly) | — | Go `testing` |
| Integration | ~20 change-relevant test funcs | `internal/accept/accept_test.go`, `internal/ledger/acceptance_test.go`, `internal/ledger/schema_test.go`, `internal/run/run_test.go`, `cmd/lucind-ai/cli_test.go`, `internal/integrate/integrate_test.go` | Go `testing` + real Git + real on-disk SQLite (`t.TempDir`) |
| E2E | 0 | — | No E2E runner exists (documented in design). |

Real-dependency integration tests (spawn `git`, create worktrees, run `sh` scripts) dominate — appropriate for this fail-closed boundary.

### Changed File Coverage
Coverage analysis skipped — no coverage gate configured (`coverage_threshold: 0`) and per-file coverage not separately collected. Not a failure.

### Assertion Quality
Audited `internal/accept/accept_test.go`, `internal/ledger/acceptance_test.go`, `internal/ledger/schema_test.go`, `internal/run/run_test.go`, `cmd/lucind-ai/cli_test.go` (accept + lease + wait-stable cases).

| File | Line | Assertion | Issue | Severity |
|------|------|-----------|-------|----------|
| — | — | — | No tautologies, no orphan-empty checks, no ghost loops, no type-only assertions, no CSS/impl-detail coupling. | — |

Representative real assertions: `os.Stat(SHOULD_NOT_EXIST)` must be `IsNotExist` (proves doc-like paths never executed); `show-ref` byte-equality before/after (proves no ref mutation); `errors.Is(err, ledger.ErrAcceptanceReceiptNotFound)` after every rejection (proves fail-closed); `checks == 2 && first.ReceiptID != second.ReceiptID` (proves binding difference re-verifies); `reflect.DeepEqual(got.AllowedPaths, []string{"cmd/lucind-ai","internal/run"})` (proves normalize/dedupe/sort). 

**Assertion quality**: ✅ All assertions verify real behavior. 0 CRITICAL, 0 WARNING.

### Quality Metrics
**Linter**: ➖ Not configured (`linter: "not detected"`).
**Type Checker**: ✅ `go build ./...` and `CGO_ENABLED=0 go build ./...` both exit 0.
**Formatter**: gofmt — not separately run in this phase; full build/test green.

### Issues Found

**CRITICAL**: None.

**WARNING**:
1. **Out-of-scope feature code shipped in this change.** The diff (`3a00fc5..HEAD`, commit `68736fd`
   "checkpoint isolated apply baseline") adds functionality the proposal and design explicitly place
   OUT of scope and that task 4.2 claims to have excluded:
   - `internal/feature/feature.go` `Service.ForceReleaseLease` + `internal/feature/feature_test.go` coverage.
   - `cmd/lucind-ai/cli.go` new command surface: `feature lease release` / `feature lease status`
     (`featureLeaseDispatch`, `runFeatureLeaseRelease`, `runFeatureLeaseStatus`, `processAlive`) and
     the top-level `usage` string entries for them.
   - `cmd/lucind-ai/cli.go` `reconcile ... --wait-stable <duration>` flag on `renew`/`resolve` +
     `TestReconcileWaitStableCLI`.
   - `cmd/lucind-ai` `check` now prints `resolved root:` (+ `TestRunCheckScriptPasses` assertion).
   These map 1:1 to recommendations 5–6 in the new `docs/orchestrator-acceptance-protocol.md`.
   proposal Scope explicitly excludes "Lease-recovery redesign, feature promotion/CAS changes";
   design Migration/Rollout says rollback "removes callers but retains audit rows" and lists
   "lease recovery" as excluded; `apply-progress.md` states "Design deviations: none" and task 4.2
   records "Lease recovery exclusion ... Confirmed" — all contradicted by the actual diff.
   Impact: not spec-breaking and fully test-covered (suite green), but it inflates the PR beyond the
   approved acceptance-verifier scope and makes the rollback boundary larger than documented.
2. **Documentation/metadata coupling beyond scope.** New `docs/orchestrator-acceptance-protocol.md`
   (106 lines) and unrelated `CONTEXT.md` glossary edits about ultrafixer ("Defect Assessment /
   Record / Remediation Proposal / External Work Item") are outside the acceptance-verifier
   spec/design. proposal Scope excludes "broad documentation rewrites". `openspec/config.yaml`
   changes (artifact_store, execution_mode, testing capabilities, extra rule) are legitimate SDD
   setup and are fine. `plugin/.../references/contracts/acceptance-promotion.md` (+28/-2) documents
   the real new `accept` command and is acceptable; plugin manifests and version `2.0.9` are
   byte-unchanged from `dev` (verified).

**SUGGESTION**:
1. apply-progress "Deviations and Issues" should be corrected to disclose WARNING 1/2 rather than
   asserting "Design deviations: none", so the archive decision has accurate provenance.
2. Consider splitting the lease-release / reconcile `--wait-stable` / `docs` work into its own
   change before merge, keeping this PR to the acceptance-verifier surface the spec describes.
3. `integrate.Check` timeout on context cancellation returns `(false, output, nil)`; acceptance
   then reports "required mechanical checks failed" rather than a distinct timeout error. Behavior
   is fail-closed and correct; a dedicated sentinel would improve diagnostics.

### Verdict
**PASS WITH WARNINGS**

Every spec requirement (8) and scenario (13) is implemented and covered by a test that passed under
`go test ./... -race -count=1` (exit 0); `CGO_ENABLED=0 go build ./...` exits 0; all 16 tasks are
complete with genuine, behavior-asserting tests; plugin manifests and version are untouched. The
change also carries out-of-scope lease-release / reconcile / documentation additions that the
proposal and design exclude and that apply-progress incorrectly reports as "no deviations" — not
archive-blocking, but the orchestrator/user should decide whether to trim the PR or accept the
expanded scope before archive.

## Key Learnings

1. The acceptance verifier is fail-closed by construction: every rejection path returns before InsertAcceptanceReceipt, and tests assert ErrAcceptanceReceiptNotFound after each failure.
2. Schema v9 enforces receipt immutability at the database layer with BEFORE UPDATE and BEFORE DELETE triggers on lane_candidates and acceptance_receipts, not just in Go code.
3. Exact-cache reuse is gated on full Binding struct equality plus ResultHash, so any environment or policy difference forces a fresh verification run rather than returning a stale receipt.
4. Commit 68736fd folded pre-existing working-tree scaffolding (feature lease release, reconcile --wait-stable, orchestrator acceptance protocol doc) into the acceptance-verifier branch, producing out-of-scope changes that apply-progress still reported as "Design deviations: none".
5. The full race suite passed on the first run with no flake from TestExecuteBatchAppliesPerLaneDeadlineIndependently.
