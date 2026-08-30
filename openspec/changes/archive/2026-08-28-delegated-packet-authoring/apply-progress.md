# Apply Progress: Delegated Packet Authoring

**Mode:** Strict TDD  
**Delivery:** `size:exception` approved; WU1–WU6 complete

## Completed Tasks
- [x] 1.1 WU1 — compile deterministic contracts
- [x] 2.1 WU2 — freeze canonical candidate evidence
- [x] 3.1 WU3 — admit every batch before allocation
- [x] 4.1 WU4 — add bounded typed specialist
- [x] 5.1 WU5 — persist non-authoritative comparisons
- [x] 6.1 WU6 — prove end-to-end contracts

## TDD Cycle Evidence
| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 1.1 | `internal/packetauthor/*_test.go` | Unit/interface | N/A (new module) | Exit 1: package absent; then behavioral assertions failed against API skeleton | Exit 0: `ok .../internal/packetauthor 1.024s` | 25 passing tests across replay, changed inputs, grammar, bindings, fixtures, and ordered diagnostics | Gofmt plus ASCII-only case folding; focused tests remained green |
| 2.1 | `internal/candidatechange/collect_test.go`, `internal/{result,accept,ledger}/*_test.go` | Integration/unit | Exit 0: 4 existing packages passed, 0 failed (`result`, `accept`, `ledger`, and preserved WU1 `packetauthor`); Go did not emit an individual-case count | Original exit 1 compile/interface RED remains historical; copy retained its original behavioral mutation RED. Authorized remediation later executed seven distinct behavioral mutation RED proofs and did not relabel them as original TDD chronology. | Exit 0: all four focused packages passed, 0 failed | 7 top-level test functions cover add/modify/delete/rename/copy, four-way Git state, canonical roots, schema, v9→v10, exact correspondence, read-only, and tampering; remediation added a focused `commit -a` assertion inside the existing Git scenario | Shared canonical collector/evidence seams, gofmt, focused build/vet/tests green; every temporary mutation was restored immediately |
| 3.1 | `cmd/lucind-ai/cli_test.go`, `internal/{packet,run}/*_test.go` | Integration/unit | Exit 0 baseline: 605 pass events, 0 failed/skipped, 4 packages passed | Initial interface RED plus later controlled behavioral mutations proved batch atomicity, diagnostic order, target freshness, input visibility, write-scope separation, frozen evidence, and remediation advice | Focused exit 0: 616 pass events, 0 failed/skipped, 4 packages passed | Mixed manual/typed inputs, malformed path/body, missing/stale target, valid/omitted/non-array inputs, unchanged prompt, out-of-scope writes, and frozen evidence | Removed impossible adapter error path; gofmt and full race suite remained green |
| 4.1 | `internal/packetauthor/specialist_test.go`, `cmd/lucind-ai/packet_authoring_test.go` | Unit/integration | Exit 0 baseline: 176 pass events, 0 failed/skipped, 2 packages passed | Exit 1 compile RED, then exit 1 behavioral RED against a permissive skeleton for every authority, renderer, identity, malformed, duplicate, and unknown-output case | Exit 0: 201 pass events, 0 failed/skipped, 2 packages passed | 7 top-level tests and 18 subtest pass events cover valid typed output, exact identity, target-free requests, trusted compilation, and all rejection classes | Extracted request validation, defensive contract cloning, recursive duplicate detection, and typed errors; gofmt/build/vet/full race remained green |
| 5.1 | `internal/packetauthor/compare_test.go`, `internal/ledger/shadow_test.go`, `cmd/lucind-ai/packet_authoring_test.go` | Unit/integration | Exit 0 baseline: 283 pass events, 0 failed/skipped, 3 packages passed | Exit 1 compile RED against missing `Compare`, `Observe`, and transactional shadow-store interfaces; later behavior mutation REDs failed for unsorted differences and discarded manual selection | Exit 0: 288 pass events, 0 failed/skipped, 3 packages passed | 5 top-level tests cover equivalent and mismatched fields, replay/digest metrics, disabled/unavailable/timeout/invalid-schema/fallback classes, manual canonicality, SQLite review persistence, isolated continuation, and begin/insert/commit failures | Extracted warning-only observation, deterministic field comparison, per-attempt transactions, sanitized stage warnings, and explicit disable behavior; gofmt and focused race suite remained green |
| 6.1 | `cmd/lucind-ai/cli_test.go` | End-to-end integration | Existing `cmd/lucind-ai/cli_test.go` baseline: no numeric pass count was recorded in the original session evidence | Tests were written first. The original session recorded correspondence, shadow-persistence, and disabled-shadow manual-selection mutations as focused exit 1 failures; the correction temporarily returned `packets,nil` from admission on error, and `TestWU6AdmissionRejectsInvalidReadOnlyInput` failed with exit 1 (`admission error = <nil> <nil>, want one PA_PATH_INVALID diagnostic`) | Exit 0: 3 WU6 tests passed, 0 failed/skipped | 3 top-level tests cover the real Git/SQLite typed path, admission rejection, fallback, frozen evidence, Acceptance correspondence, isolated shadow persistence, and manual canonicality | Test-only refactor: WU6 Go test file gofmt-clean; no production refactor retained; focused tests remained green |

## Strict-TDD Test Summary
- **Total tests written:** 56 recorded top-level tests/functions plus one strengthened existing Git scope test (WU1: 25; WU2: 7; WU3: 9; WU4: 7; WU5: 5; WU6: 3).
- **Total tests passing:** Corrective WU6 focused run reported 3 pass events, 0 failed/skipped, and 1 passing package. The original required full race suite and harness both exited 0 across all packages; exact full-suite numeric totals were not recorded and were not invented.
- **Layers used:** Unit/interface (WU1, WU4–WU5); mixed integration/unit (WU2–WU4); E2E (WU6: 1 real Git/SQLite scenario plus 1 admission and 1 fallback scenario).
- **Approval tests:** None — no refactoring task was assigned.
- **Pure functions created:** Not separately counted in the recorded run evidence; no count is invented here.

## Work Unit Evidence
| Evidence | Result |
|---|---|
| Focused test | `go test ./internal/packetauthor -race -count=1` → exit 0, 25 passed, 0 failed |
| Runtime harness | N/A — WU1 is a pure compiler and fixture module with no runtime boundary |
| Rollback boundary | Remove `internal/packetauthor/` and revert only task 1.1/progress records; no existing runtime wiring changes |

### WU2
| Evidence | Result |
|---|---|
| Focused test | `go test ./internal/candidatechange ./internal/result ./internal/accept ./internal/ledger -race -count=1` → exit 0; 4 packages passed, 0 failed |
| Runtime harness | `go test ./internal/candidatechange ./internal/accept ./internal/ledger -run 'TestCollectCanonicalCommittedChangesAndCopyScope|TestCollectFourWayUnionAndCanonicalRootSelectors|TestValidateVersionedResultRequiresExactFrozenCorrespondence|TestValidateVersionedReadOnlyForbidsCommitAndChanges|TestSchemaV10AddsAuthoringEvidenceAndPreservesLegacyReads' -race -count=1 -v` → exit 0; 5 top-level real Git/SQLite scenarios and 6 correspondence subtests passed, 0 failed |
| Rollback boundary | Remove `internal/candidatechange/`, `internal/{result/filechange_test.go,accept/authoring_evidence_test.go,ledger/authoring.go,ledger/authoring_evidence_test.go}`; revert only WU2 edits in `internal/{result,accept,ledger}` and task/progress records. `internal/packetauthor/` and WU1 evidence remain intact. |
| Commit | None — WU2 remains uncommitted; the native attempt is held by the orchestrator, and this corrective rerun created no commit. |

### WU3
| Evidence | Result |
|---|---|
| Focused test | `go test ./cmd/lucind-ai ./internal/packet ./internal/run ./internal/executor -race -count=1 -json` → exit 0; 616 pass events, 0 failed/skipped; 4 packages passed |
| Runtime harness | `go test ./cmd/lucind-ai ./internal/run -run 'TestRunDispatchRejectsWholeBatchBeforeQuotaAllocationAndExecutor|TestRunDispatchRejectsStaleCompiledBodyBeforeAllocation|TestExecuteBatchOutOfScopeUntrackedFileDeviatedExcludedFromIntegrate' -race -count=1 -v` → exit 0; 3 top-level scenarios passed, 0 failed; 2 packages passed, 0 failed (`cmd/lucind-ai`: 2 scenarios; `internal/run`: 1 scenario) |
| Full verification | `go test ./... -race -count=1` and `./lucind-checks.sh` → exit 0; all packages passed |
| Rollback boundary | Revert `cmd/lucind-ai/{packet_authoring.go,cli.go}` WU3 edits, WU3 tests/fixture updates, and `ReadOnlyPaths`/authoring propagation in `internal/{packet,executor,run}`; WU1–WU2 modules/evidence remain intact |
| Commit | None — WU3 remains uncommitted under the orchestrator-held native attempt. |

### WU4
| Evidence | Result |
|---|---|
| Focused test | `go test ./internal/packetauthor ./cmd/lucind-ai -race -count=1` with JSON counter → exit 0; 201 pass events, 0 failed/skipped; 2 packages passed, 0 failed |
| Runtime harness | `go test ./internal/packetauthor ./cmd/lucind-ai -run 'TestSpecialist|TestCompileSpecialist' -race -count=1 -json` with JSON counter → exit 0; 25 pass events, 0 failed/skipped; 2 packages passed, 0 failed. The in-process fake runner proved exact specialist identity, target-free request bytes, typed rejection/acceptance, and trusted compiler rendering without account or network access. |
| Agent structural validation | `opencode agent list --pure` piped to a read-only JSON parser → exit 0; `agent=lucind-packet-author mode=primary wildcard_permission=deny parsed=true`. No global configuration was changed. |
| Full verification | `CGO_ENABLED=0 go build ./...`, `go vet ./...`, `go test ./... -race -count=1`, and `./lucind-checks.sh` → exit 0; all packages passed and touched Go files were gofmt-clean |
| Rollback boundary | Remove `internal/packetauthor/{specialist.go,specialist_test.go}`, `cmd/lucind-ai/packet_authoring_test.go`, `.opencode/agent/lucind-packet-author.md`; revert only WU4's `Contract` JSON tags, `compileSpecialistPacket` seam, and task/progress records. WU1–WU3 behavior and evidence remain intact. |
| Commit | None — WU4 remains uncommitted under the orchestrator-held native attempt. |

### WU5
| Evidence | Result |
|---|---|
| Focused test | `go test ./internal/packetauthor ./internal/ledger ./cmd/lucind-ai -race -count=1` → exit 0; 288 pass events, 0 failed/skipped; 3 packages passed |
| Runtime harness | `go test ./internal/packetauthor ./internal/ledger ./cmd/lucind-ai -run 'TestCompare|TestPersistShadow|TestShadowObservation' -race -count=1 -v` → exit 0; 5 top-level scenarios passed, 0 failed; fake specialist timeout/fallback/invalid-schema and real SQLite review/rollback paths passed |
| Metrics | Validity, semantic equivalence, digest equality, replay stability, latency, review-cost storage, field-level differences, and typed failure classes are represented in comparison/persistence evidence |
| Cleanup/process proof | Temporary sort/manual-selection mutations each produced exit 1 on their mapped behavioral assertion and were restored; `git diff --check` passed; no temporary files, processes, or external account calls were created |
| Rollback boundary | Disable shadow invocation by using `packetauthor.Disabled`; otherwise remove only `internal/packetauthor/{compare.go,compare_test.go}`, `internal/ledger/{shadow.go,shadow_test.go}`, the WU5 CLI test, WU5 docs/profile additions, and WU5 task/progress records. Preserve all WU1–WU4 compiler, evidence, admission, dispatch, and specialist behavior. |
| Commit | None — WU5 remains uncommitted under the orchestrator-held native attempt. |

### WU6
| Evidence | Result |
|---|---|
| Focused test | `set -o pipefail; go test ./cmd/lucind-ai -run '^TestWU6' -race -count=1 -json | python3 -c 'import sys,json; from collections import Counter; events=[json.loads(line) for line in sys.stdin if line.strip()]; tests=Counter(e["Action"] for e in events if e.get("Test") and e.get("Action") in ("pass","fail","skip")); packages=Counter(e["Action"] for e in events if not e.get("Test") and e.get("Package") and e.get("Action") in ("pass","fail")); print("tests_passed={} tests_failed={} tests_skipped={} packages_passed={} packages_failed={}".format(tests["pass"], tests["fail"], tests["skip"], packages["pass"], packages["fail"]))'` → exit 0; `tests_passed=3 tests_failed=0 tests_skipped=0 packages_passed=1 packages_failed=0` |
| Runtime harness | Same focused command exercises the real runtime boundary in its typed scenario: temporary Git repository/worktree, SQLite ledger, typed admission, executor request, result candidate, Acceptance verifier, and shadow persistence; it also runs the admission and fallback scenarios → exit 0; 3 tests passed, 0 failed/skipped |
| Full verification | Original `go test ./... -race -count=1`, `./lucind-checks.sh`, `go vet ./...`, and `git diff --check` records all exited 0. Exact full-suite numeric totals were not recorded; no full-suite rerun was performed in this correction. |
| Golden evidence | N/A — WU6 adds no generated goldens or `-update` path; deterministic compiled bytes, digests, frozen evidence, and receipt bindings are asserted directly. |
| Mutation evidence | Temporarily bypassing Acceptance correspondence rejected the tampered candidate with `Acceptance accepted a result whose declaration diverges from frozen evidence`; bypassing shadow persistence produced `shadow:0 receipt:1`; disabling manual selection produced `manual body changed`; bypassing admission produced the exact `PA_PATH_INVALID` test failure recorded above. All mutations were restored immediately and WU6 returned green with 3 passing tests. |
| Rollback boundary | Remove only the WU6 test block/helpers and WU6 task/progress entries in `cmd/lucind-ai/cli_test.go`, `openspec/changes/delegated-packet-authoring/tasks.md`, and this progress artifact. No production behavior or WU1–WU5 tests are required for rollback. |
| Changed-line budget / commit | Correction delta: 34 authored additions/deletions (16 permanent WU6 test additions plus 18 progress-line replacements); temporary production mutation restored immediately and contributes zero net lines. Below the 180-line correction limit. No commit created; all prior uncommitted work preserved. |

### WU4 Task-Mapped RED Threat Accounting
| Threat | Behavior-level RED | Restored GREEN |
|---|---|---|
| Target SHAs/binding | Permissive decoding accepted `target_sha` and `binding`; target claims also entered requests | All target/binding keys return `PA_SPECIALIST_AUTHORITY_FORBIDDEN`; typed requests omit target facts |
| Dispatch/worktree | Permissive decoding accepted `dispatch` and `worktree_path` | Both authority classes reject before compilation |
| Integration/Acceptance/Promotion | Permissive decoding accepted each authority field | Each field returns `PA_SPECIALIST_AUTHORITY_FORBIDDEN` |
| Unknown fields | Plain `json.Unmarshal` ignored a valid contract's `surprise` field | Strict decoding returns `PA_SPECIALIST_OUTPUT_INVALID` |
| Malformed/duplicate output | Malformed data returned an untyped JSON error; duplicate keys were accepted; a second value returned only a generic syntax error | All failures are typed; duplicate keys and multiple values return `PA_SPECIALIST_OUTPUT_DUPLICATE` |
| Fallback/default identity | Empty, `default`, `build`, and lookalike identities were accepted | Exact `lucind-packet-author` identity is required before output decoding |
| Untrusted renderer output | `markdown` and `frontmatter` were accepted by the permissive adapter | Renderer fields return `PA_SPECIALIST_RENDER_FORBIDDEN`; only `Compile` produces `Artifact.Body` |
| Permission/side effects | The initial interface had no enforced profile proof | Runner receives only agent name plus target-free JSON; the parsed primary profile denies wildcard permissions and uses no explicit model override |

### WU3 Task-Mapped RED Threat Accounting
| Threat | Controlled RED | Restored GREEN |
|---|---|---|
| Whole-batch atomicity | Bypassing admission produced quota:1, allocation:2, executor:2 and race failure | Rejection harness passed with all counters zero |
| Diagnostic order/targets | Reordering inputs changed packet indices; missing ref returned a generic resolver error | Ordered codes include deterministic packet-indexed target diagnostics |
| Stale target | Treating expected SHA as live caused one allocation | Stale packet rejected before allocation |
| Input parsing/visibility | Dropping parsed/request paths returned empty slices | Valid paths preserved; non-array metadata rejected; executor sees inputs |
| Write authority | Unioning read-only paths into allowed paths integrated lane-a | Read-only input remains out-of-scope and lane-a deviates |
| Frozen evidence/advice | Skipping evidence caused incomplete evidence; removing advice omitted feature/legacy remediation | Versioned evidence and explicit remediation assertions pass |

### WU2 Task-Mapped RED Threat Accounting
| Threat | RED evidence | GREEN evidence |
|---|---|---|
| Relative selector | Authorized remediation mutation rejected relative selectors; `go test ./internal/candidatechange -run '^TestCollectFourWayUnionAndCanonicalRootSelectors$' -race -count=1 -v` → exit 1 at `selector "../../.../repo;touch argv-injection"`, `relative selectors are unsupported` | Same exact command after restoration → exit 0, PASS |
| Absolute selector | Authorized remediation mutation rejected absolute selectors; the same selector command → exit 1 at `commit -a candidate`, `absolute selectors are unsupported` | Same exact command after restoration → exit 0, PASS |
| Symlink selector | Authorized remediation mutation skipped symlink resolution; the same selector command → exit 1 at `selector "/tmp/.../repo-link"`, `selector is not the canonical repository root` | Same exact command after restoration → exit 0, PASS |
| Staged change | Authorized remediation mutation removed cached-diff collection; the same selector command → exit 1, `four-way union ... missing {created staged.txt}` | Same exact command after restoration → exit 0, PASS |
| `commit -a` / committed path | A focused assertion was first added immediately after `git commit -am`. Authorized remediation mutation diffed candidate against itself; the same selector command → exit 1, `commit -a candidate = [], <nil>` | Same exact command after restoration → exit 0, PASS |
| Empty index | Authorized remediation mutation removed deletion parsing; the same selector command → exit 1 after `git read-tree --empty`, `candidatechange: unsupported status "D"` | Same exact command after restoration → exit 0, PASS |
| Rename | Authorized remediation mutation omitted the deleted source endpoint; `go test ./internal/candidatechange -run '^TestCollectCanonicalCommittedChangesAndCopyScope$' -race -count=1 -v` → exit 1 with `rename-source.txt` absent from the actual canonical set | Same exact command after restoration → exit 0, PASS |
| Copy | Compile/interface RED first; recorded behavioral mutation changed `copied` to `created` and failed both candidate and Acceptance suites | Copy classification/source/path and exact frozen correspondence passed after restoration |

### WU2 Authorized Remediation Execution
- **Chronology:** Original compile/interface RED and copy mutation evidence are preserved. The seven rows above are explicitly later, user-authorized behavioral mutation proofs; they are not claimed as historical RED.
- **Safety-net baseline:** `go test ./internal/candidatechange ./internal/result ./internal/accept ./internal/ledger -race -count=1` → exit 0; 4 packages passed, 0 failed before permanent remediation edits.
- **Final focused suite:** `go test ./internal/candidatechange ./internal/result ./internal/accept ./internal/ledger -race -count=1` → exit 0; 4 packages passed, 0 failed.
- **Numeric test summary:** `set -o pipefail && go test ./internal/candidatechange ./internal/result ./internal/accept ./internal/ledger -race -count=1 -json | python3 -c 'import sys,json; from collections import Counter; events=[json.loads(line) for line in sys.stdin if line.strip()]; counts=Counter(event["Action"] for event in events if event.get("Test") and event.get("Action") in ("pass","fail","skip")); packages=Counter(event["Action"] for event in events if not event.get("Test") and event.get("Package") and event.get("Action") in ("pass","fail")); print("tests_passed={} tests_failed={} tests_skipped={} packages_passed={} packages_failed={}".format(counts["pass"], counts["fail"], counts["skip"], packages["pass"], packages["fail"]))'` → exit 0; 136 test pass events, 0 failed, 0 skipped; 4 packages passed, 0 failed. One prior auxiliary counter invocation had a Python quoting syntax error before tests were counted; the corrected counter above succeeded.
- **Runtime Git/SQLite harness:** `go test ./internal/candidatechange ./internal/accept ./internal/ledger -run 'TestCollectCanonicalCommittedChangesAndCopyScope|TestCollectFourWayUnionAndCanonicalRootSelectors|TestValidateVersionedResultRequiresExactFrozenCorrespondence|TestValidateVersionedReadOnlyForbidsCommitAndChanges|TestSchemaV10AddsAuthoringEvidenceAndPreservesLegacyReads' -race -count=1 -v` → exit 0; 5 top-level scenarios plus 6 correspondence subtests passed, 0 failed.
- **Cleanup/process proof:** No unsupported-selector mutation text remained; canonical base→candidate diff, cached diff, symlink resolution, deletion mapping, and both rename endpoints were present. `/tmp` contained no matching Go-test repository directories, and `ps -C go -o pid=,args=` returned no Go processes.
- **Changed-line count / no commit:** 42 authored additions+deletions for this remediation: 4 permanent test additions and 38 merged apply-progress evidence additions+deletions; temporary production mutations net to zero. This is below the 240-line cap. No commit was created; the orchestrator continues to hold the native attempt.

## Additional Evidence
- Mutation: a constant digest caused all five relevant-input subtests to fail; restoration returned green.
- Local quality: `CGO_ENABLED=0 go build ./internal/packetauthor`, `go vet ./internal/packetauthor`, and `gofmt -l` passed.
- Changed-line count: 998 authored additions/deletions for WU1 evidence, including task/progress persistence.
- WU2 original RED: candidate package had no implementation; result failed on missing `SourcePath`; ledger/Acceptance failed on missing canonical evidence interfaces. These remain compile/interface evidence and are not represented as behavioral RED.
- WU2 mutation: changing copy classification from `copied` to `created` failed both candidate and Acceptance suites; restoration returned green.
- WU2 remediation: seven separately restored behavioral mutations proved relative, absolute, and symlink selector handling; staged, committed, and empty-index state; and both rename endpoints. A permanent focused `commit -a` assertion was added because the prior overlapping worktree state did not independently prove the committed path.
- WU2 local quality: focused `CGO_ENABLED=0 go build`, `go vet`, and `gofmt -l` passed previously; this remediation ran `gofmt` on the touched Go test and the required focused/runtime suites. No full repository suite was requested or run.
- WU2 changed-line count: 866 authored additions/deletions, including task/progress persistence.
- WU3 local quality: final full race suite passed; the project binary was refreshed from `d2e75c5` to `d2e75c5-dirty` with `make install`.
- WU3 changed-line count: 686 authored additions/deletions including task/progress persistence; below the approved 750-line cap.
- WU4 local quality: focused and runtime suites, agent parser validation, build, vet, full race suite, and `./lucind-checks.sh` passed. `make install` refreshed `lucind-ai d2e75c5-dirty (go1.25.0, linux/amd64)` after final checks.
- WU4 changed-line count: 569 authored additions/deletions including task/progress persistence; below the 650-line cap. No temporary files, external account calls, network calls, or commits were created.
- WU5 changed-line count: 594 authored additions/deletions including WU5-owned code, tests, docs, and merged task/progress hunks; below the 800-line cap. Pre-existing WU1–WU4 changes are excluded. No temporary files, external account calls, network calls, or commits were created.
- WU5 mutation evidence: removing deterministic field sorting caused `TestCompareSortsFieldDifferencesAndClassifiesInvalidShadowAttempts` to exit 1 with `[mode write_paths]`; discarding manual selection caused `TestCompareRecordsEquivalentFieldsDigestsAndReplayStability` to exit 1 with `ManualSelected:false`; both were restored and the focused suite returned green.
- Deviations: none. WU6 is test-only and intentionally does not alter production behavior or remove manual authoring.

## Apply Result Contract
- **Status:** `success` — WU6 proves the typed authoring path reaches Acceptance while preserving frozen evidence, manual canonicality, and isolated shadow persistence.
- **Completed tasks:** 6 — `1.1`, `2.1`, `3.1`, `4.1`, `5.1`, `6.1`.
- **Remaining tasks:** 0.
- **Issues:** No implementation or test failures remain. Repository-wide `gofmt -l .` still reports pre-existing formatting in unrelated files (`internal/dag/parse.go`, `internal/dag/waves_test.go`, `internal/executor/*_test.go`, `internal/overlap/overlap_test.go`, `internal/packet/disjoint_test.go`, `internal/result/result_test.go`); the WU6-touched Go file is gofmt-clean. The pre-check installed binary was stale and lacked `sdd-status`; source-based fallback established `applyState: ready` from the existing artifacts.
- **OpenSpec artifact:** This file contains merged WU1–WU6 progress; `tasks.md` marks `1.1`–`6.1` complete.
- **Engram mirror:** Topic `sdd/delegated-packet-authoring/apply-progress` must mirror this full merged artifact and tasks observation.
- **Hybrid persistence:** OpenSpec file persistence and Engram topic persistence are both required and declared complete for these identical merged progress contents.
- **Risks:** Accepted `size:exception` remains in force; automatic cutover and manual-path removal remain disabled/out of scope. No WU6 production cutover was introduced.
- **Skill resolution:** `paths-injected` — all six requested skills were read (the injected writing skill path used its installed `.claude` location), with required shared OpenSpec/Strict-TDD modules.
- **Next recommended:** Return to the parent orchestrator; implementation is complete and ready for `sdd-verify`.

## Authorized Successor Remediation: `runtime-scope-canonicalization`

**Change:** `delegated-packet-authoring`  
**Binding:** failed verification evidence revision `sha256:a470dc14c501d51ff876146fb176dcc38d453a2f2c1aee39c782e62206968cce`  
**Attempt authority:** Native attempt was acquired by the orchestrator; this lane did not acquire or settle an attempt.  
**Objective limit:** Maximum two total attempts and 500 changed lines.  
**Scope:** Finding #1 only. Findings #2, #3, and #4 were not changed or claimed resolved. All WU1–WU6 evidence and completed task checkboxes remain preserved.

### Remediation TDD Cycle Evidence

| Cycle | Evidence |
|---|---|
| RED | Added `internal/run/scope_test.go` first. `go test ./internal/run -run '^TestEnforceAllowedPathsUsesCanonicalFourWayCopyAwareChanges$' -race -count=1 -v` → exit 1: copy source, rename source, and whitespace-path cases initially returned `done` instead of `deviated`. |
| GREEN | Rewired terminal scope to `candidatechange.Collect` with `IncludeWorktree: true` and `candidatechange.OutOfScope`; the focused remediation scenario then passed all five cases. |
| REFACTOR | Removed the obsolete `internal/run` duplicate NUL parser and its test-only exports/tests, preserved the existing git-failure diagnostic wording, and ran `gofmt` on every touched Go file. |

### Remediation Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test | `go test ./internal/run ./internal/candidatechange -race -count=1` → exit 0; both packages passed, 0 failures. |
| Runtime harness | `go test ./internal/run -run '^TestEnforceAllowedPathsUsesCanonicalFourWayCopyAwareChanges$' -race -count=1 -v` → exit 0; 5 real temporary-Git scenarios passed: copy source endpoint, rename source endpoint, leading/trailing whitespace, staged path, and two committed changes from the recorded base. |
| Mutation/differential proof | Temporarily set `IncludeWorktree: false`; the same runtime scenario exited 1 at `includes staged path` (`status = done, want deviated`). Restored `true`; the focused test and full race suite passed. |
| Full race suite | `go test ./... -race -count=1` → exit 0; all test-bearing packages passed; `cmd/plugincontent` reported no test files. |
| Build/vet/format | `CGO_ENABLED=0 go build ./...`, `go vet ./...`, `gofmt -l internal/run/run.go internal/run/scope_test.go internal/run/export_test.go internal/run/run_test.go`, and `git diff --check` → exit 0 / no output. |
| Rollback boundary | Remove `internal/run/scope_test.go`, restore `internal/run/gitpaths.go`, restore its test-only exports/tests, and revert only the `enforceAllowedPaths` implementation to the pre-remediation local parser. Preserve all WU1–WU6 production behavior and evidence. |
| Changed-line budget / cleanup | 392 remediation-owned authored additions/deletions (348 code/test lines plus 44 evidence lines), below the 500-line limit. Temporary mutation was restored, no commit was created, and no attempt was acquired or settled. |

### Remediation Result Contract

```yaml
schema: gentle-ai.remediation-result/v1
change: delegated-packet-authoring
successor: runtime-scope-canonicalization
failed_evidence_revision: sha256:a470dc14c501d51ff876146fb176dcc38d453a2f2c1aee39c782e62206968cce
finding_scope: 1
status: success
other_findings_claimed_resolved: 0
next_recommended: sdd-verify
```

```json
{"schema":"gentle-ai.remediation-evidence/v1","failed_evidence_revision":"sha256:a470dc14c501d51ff876146fb176dcc38d453a2f2c1aee39c782e62206968cce","focused_test":"go test ./internal/run ./internal/candidatechange -race -count=1","focused_exit_code":0,"full_race_test":"go test ./... -race -count=1","full_race_exit_code":0,"mutation":"IncludeWorktree=false failed staged-path assertion; restored","changed_lines":392,"rollback_boundary":"runtime scope collector wiring, duplicate parser removal, focused tests, and remediation evidence only"}
```

## Authorized Successor Remediation: `generation-14-six-blocker-closure`

**Change:** `delegated-packet-authoring`  
**Binding:** failed verification evidence revision `sha256:e038dcdaad85a2ab156ac3d4d154dd833d984e077b436d8b3b62a17981f08b11`  
**Objective:** close the six critical findings from the generation-14 verification report without changing manual canonicality or cutover behavior.  
**Delivery:** approved `size:exception`, single PR; all existing WU1–WU6 changes remain uncommitted and preserved.

### Remediation TDD Cycle Evidence

| Finding | RED | GREEN | REFACTOR |
|---|---|---|---|
| Concrete executor read-only visibility | Concrete adapter coverage was added against the missing child-process environment contract | Four concrete adapters expose exact `LUCIND_READ_ONLY_PATHS` JSON; focused subprocess test passes | Centralized environment construction and removed inherited-value leakage |
| Complete Acceptance target binding | Binding-field mutation cases were added for feature identity, parent ref, base SHA, expected-parent SHA, and missing feature | Acceptance validates feature and legacy-main bindings before result correspondence; focused test passes | Strict decoding rejects unknown/trailing/incomplete binding data |
| Multiple in-scope commits | Positive multi-commit scope scenario was added | Lane remains `Done`; focused scope test passes | Reused canonical four-way collector |
| Deterministic replay mismatch | Injectable compiler replay test exercises a changed second compilation | Comparison records `deterministic_instability` and remains manual-canonical | Kept compiler seam private and warning-only |
| Rename across read-only input scope | Rename source/destination scenario was added | Source endpoint is rejected as out of write scope; focused scope test passes | Reused canonical endpoint classification |
| Frozen evidence after source mutation | WU6 now mutates `.lucind/packets/typed-lane.md` after dispatch | Candidate evidence remains byte/hash-identical and Acceptance succeeds from frozen evidence | Added explicit source mutation and frozen-row assertions |

### Remediation Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused tests | `go test ./internal/executor -run '^TestConcreteExecutorsExposeReadOnlyPathsToChild$' -race -count=1 -v`; `go test ./internal/packetauthor -run '^TestCompareClassifiesNondeterministicSecondCompilation$' -race -count=1 -v`; `go test ./internal/accept -run '^TestValidateTypedTargetBindingRequiresCompleteIdentity$' -race -count=1 -v`; `go test ./internal/run -run '^TestEnforceAllowedPaths(KeepsMultipleInScopeCommitsDone|RejectsRenameAcrossReadOnlyInputScope)$' -race -count=1 -v`; and `go test ./cmd/lucind-ai -run '^TestWU6TypedAuthoringReachesAcceptanceWithShadowIsolation$' -race -count=1 -v` → all exit 0. |
| Runtime harness | WU6 focused integration passed with real temporary Git worktree, SQLite ledger, dispatch, candidate freezing, source-packet mutation, Acceptance, and shadow persistence. Exit 0. |
| Full verification | `go test ./... -race -count=1` → exit 0; `CGO_ENABLED=0 go build ./...` → exit 0; `go vet ./...` → exit 0; `./lucind-checks.sh` → exit 0; touched files are gofmt-clean; `git diff --check` → exit 0. |
| Rollback boundary | Revert only executor environment propagation/tests, typed binding validation/tests, the two scope scenarios, replay seam/test, and WU6 source-mutation assertions. Preserve WU1–WU6 and the earlier runtime-scope remediation. |

### Remediation Result Contract

```yaml
schema: gentle-ai.remediation-result/v1
change: delegated-packet-authoring
successor: generation-14-six-blocker-closure
failed_evidence_revision: sha256:e038dcdaad85a2ab156ac3d4d154dd833d984e077b436d8b3b62a17981f08b11
status: success
blockers_closed: 6
other_findings_claimed_resolved: 0
next_recommended: sdd-verify
```

```json
{"schema":"gentle-ai.remediation-evidence/v1","failed_evidence_revision":"sha256:e038dcdaad85a2ab156ac3d4d154dd833d984e077b436d8b3b62a17981f08b11","focused_test":"go test ./internal/executor ./internal/packetauthor ./internal/accept ./internal/run ./cmd/lucind-ai -race -count=1","focused_exit_code":0,"full_race_test":"go test ./... -race -count=1","full_race_exit_code":0,"build":"CGO_ENABLED=0 go build ./...","build_exit_code":0,"vet":"go vet ./...","vet_exit_code":0,"harness":"./lucind-checks.sh","harness_exit_code":0,"changed_lines":"below 500; remediation is bounded within the approved size:exception and excludes pre-existing WU1-WU6 worktree changes","rollback_boundary":"executor read-only environment propagation, complete typed binding validation, positive scope/read-only rename/replay/frozen-evidence tests only"}
```
