# Design Lens C — Failure, Test & Rollback: Agentic Phase Specialist

## Assumed architecture

Existing `sdd-*` subagents become phase-scoped Specialists driving fan-out and synthesis via `lucind-ai run`, independently accepting own-phase planning lanes via `lucind-ai accept` without human confirmation, and returning a compressed Phase Verdict to the Orchestrator while Promotion remains strictly human-confirmed. `Verifier.Verify` (`internal/accept/accept.go:120-137`) and `executeAttempt` (`internal/run/attempt.go:431-435`) load `LaneMetadata` unconditionally to gate `lucind-checks.sh` execution to apply phases, unlabeled lanes, or explicit exceptions. Deterministic scope validation (`allowed_paths`), hard-stop demotion, and Tier A Dual-Judge qualitative acceptance remain unconditionally active.

## Testing Strategy

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|
| Acceptance Verifier (`internal/accept`) | `Verifier.Verify` skips `lucind-checks.sh` for non-apply planning phases (`"explore"`, `"propose"`, `"design"`, `"spec"`, `"tasks"`), but runs checks when `SDDPhase == "apply"`, empty/missing (`""`), or on explicit exception; missing metadata runs checks; scope and qualitative gates remain unconditional. | Fixture tests in `internal/accept/accept_test.go:26-67` registering `LaneMetadata` with various `SDDPhase` values and asserting failing checks (`exit 1`) fail apply/unlabeled lanes but allow valid planning candidates. | `Verifier.check` (`internal/accept/accept.go:49,55`), `Verifier.loadCandidate` (`internal/accept/accept.go:48,56`), `ledger.UpdateLaneMetadata` (`internal/ledger/lanes_meta.go:49-60`), `newVerifierFixture` (`internal/accept/accept_test.go:26-67`) |
| Integration Attempt (`internal/run`) | `executeAttempt` evaluates phase gate before calling `checkFunc` in `AttemptStatusChecking`, skipping `RunChecks` for non-apply phases while preserving lease renewal, status transitions, and audit logging. | Unit tests in `internal/run/attempt_test.go:24-80` using `newAttemptTestDeps` and `attemptSpies` asserting `checkCalls` across `sdd_phase` values. | `Deps.RunChecks` (`internal/run/run.go:208`, `internal/run/attempt.go:431-435`), `attemptSpies.checkFunc` (`internal/run/attempt_test.go:41,83-92`), `Deps.Ledger` (`internal/run/run.go:165`) |
| Mechanical Verification (`internal/integrate`) | `integrate.Check` remains an ungated execution primitive running `lucind-checks.sh` at worktree root whenever invoked, returning output and status without inspecting metadata. | Regression tests in `internal/integrate/integrate_test.go:21-50` asserting script execution, timeout handling, and environment isolation. | `integrate.Check` (`internal/integrate/integrate.go:159-200`), `CheckPolicySnapshot` (`internal/integrate/integrate.go:120-148`), `initRepo` (`internal/integrate/integrate_test.go:21-36`) |
| Skill & Contract Parity (`internal/packet`) | Claude Code and OpenCode skill trees remain byte-identical after Hard Rule carve-out (`SKILL.md:19-20`) and fan-out updates (`fan-out.md:47-48`); domain glossary in `references/core/domain.md` accurately projects `CONTEXT.md:103-109`. | Regression tests in `internal/packet/packet_test.go:924-967` asserting zero tree diff and verbatim projection of `CONTEXT.md`. | `TestSkillDocumentsLanguageGlossary` (`internal/packet/packet_test.go:924-941`), `TestSkillTreesByteIdentical` (`internal/packet/packet_test.go:943-967`) |

## Test Seams

The check-gating change leverages existing seams:
- **`Verifier.check`** (`internal/accept/accept.go:49,55`): Injected function field `func(context.Context, string) (bool, string, error)` on `Verifier`, defaulting to `integrate.Check`. Asserts whether checks ran or were skipped.
- **`Verifier.loadCandidate`** (`internal/accept/accept.go:48,56`): Injected candidate loader for supplying frozen candidate records.
- **`Verifier.newID` and `Verifier.now`** (`internal/accept/accept.go:50-51,55`): Injected UUID generator and clock for deterministic receipt ID and timestamps.
- **`Deps.RunChecks`** (`internal/run/run.go:208`, `internal/run/attempt.go:431-435`): Injected check execution function field in `run.Deps`, defaulting to `integrate.Check` and spied by `attemptSpies.checkFunc` (`internal/run/attempt_test.go:41,83-92`).
- **`Deps.Ledger`** (`internal/run/run.go:165`): Injected ledger instance providing access to `GetLaneMetadata` (`internal/ledger/lanes_meta.go:104-115`).
- **`newVerifierFixture`** (`internal/accept/accept_test.go:26-67`): Test fixture setting up isolated Git repository, `lucind-checks.sh`, ledger DB, and candidate registration.
- **`attemptSpies`** (`internal/run/attempt_test.go:24-44`): Spy recording lifecycle calls (`checkCalls`, `combineCalls`, `promoteCASCalls`).

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | N/A: No new file classification or execution mechanisms. `internal/accept` treats documentation-like files as scope-only via `TestVerifierTreatsDocumentationLikeFilesAsScopeOnly` (`internal/accept/accept_test.go:127-140`). Check-gating relies on `LaneMetadata.SDDPhase` (`internal/ledger/lanes_meta.go:20-47`) while `validateResultAndScope` (`internal/accept/accept.go:97-98,214-261`) is unchanged. | Scope boundaries unchanged; non-apply planning lanes skip checks via metadata rather than heuristics. | None (existing coverage in `internal/accept/accept_test.go:127-140`). |
| Git repository selection | `git -C`, relative paths, absolute paths | N/A: Repository root resolution and Git execution are unchanged. `canonicalRoot` (`internal/accept/accept.go:149-158`) enforces absolute evaluated symlink paths and rejects relative roots (`internal/accept/accept_test.go:297-318`). | Root validation and detached worktree isolation in `createOwnedIsolation` (`internal/accept/accept.go:406-428`) are preserved. | None (existing coverage in `internal/accept/accept_test.go:297-318`). |
| Commit state | staged, `commit -a`, empty index | N/A: Verifier does not interact with primary index or commit flags. `internal/accept` operates on frozen detached commits in isolated worktrees (`internal/accept/accept.go:406-428`), ignoring dirty primary state (`internal/accept/accept_test.go:142-166`). | Candidate validation in `validateObjects` (`internal/accept/accept.go:198-212`) and isolated detached worktrees remain unchanged. | None (existing coverage in `internal/accept/accept_test.go:142-166`). |
| Push state | tracking branch, first push, explicit refspec | N/A: Verifier performs no remote Git operations or ref pushes. `internal/accept` operates purely on local repository objects and ledger state without mutating refs (`internal/accept/accept.go:1-3,80-100`). | Local immutable receipt generation and ledger recording; remote push boundaries are out of scope. | None (existing coverage in `internal/accept/accept_test.go:80-100`). |
| PR commands | explicit `--head`, environment prefix, composed commands | N/A: No VCS, PR creation, or GitHub CLI integration exists in this repository (`CONTEXT.md:23-26`). All lane execution and acceptance is local via CLI. | VCS and PR automation boundaries are untouched; no shell composition for PRs exists. | None (PR automation is outside repository scope). |

## Rollback and Additivity

**Choice**: Standard `git revert` of code and documentation commits.
**Alternatives considered**: Database migration down-scripts or ledger rollback transactions (rejected: no DDL or database schema changes exist); runtime feature flags (rejected: Git revert provides clean reversal without runtime complexity).
**Rationale**: Zero database schema changes, zero DDL migrations, and zero ledger column modifications are introduced (`internal/ledger/schema.go:425-445,584-592`). `AuthoringEvidenceVersion` remains `"lane-authoring-evidence/v1"` (`internal/ledger/authoring.go:14`), `BindingVersion` remains `"binding:v2"`, and `Contract` remains `json.RawMessage` (`internal/ledger/authoring.go:26,44-75`). Existing frozen candidate rows decode byte-identically. Reverting code restores unconditional `integrate.Check` in `internal/accept/accept.go:120-137` and `internal/run/attempt.go:415-460`, and Orchestrator-owned synthesis review in `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48`.

No schema, ledger, or result envelope versions move. Check-gating logic is additive and fail-safe: lanes with empty or missing `sdd_phase` continue running full mechanical checks.

## Out of Scope

- Modifying `AuthoringEvidence` struct, `AuthoringEvidenceVersion`, or SQLite schema migrations (`internal/ledger/authoring.go:14-26`, `internal/ledger/schema.go:425-445`).
- Delegating Change-level Promotion authority to any AI agent or Specialist (`CONTEXT.md:91-94`, `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:44-50`).
- Altering unconditional `allowed_paths` boundary enforcement or hard stop demotion (`internal/run/run.go:841-878`, `internal/accept/accept.go:97-98,214-261`).
- Direct Bash or Agent dispatch tool provisioning for `sdd-*` subagents (`cmd/lucind-ai/cli.go:2516-2517`, `docs/sdd-phase-specialist.md:21-24`).
- Modifying `integrate.Check` internal execution mechanics (`internal/integrate/integrate.go:159-200`).
- Multi-repository coordination or distributed execution (`CONTEXT.md:23-26`).

## Open Questions

- [ ] Should `internal/accept/accept.go` add a safety fallback checking if any modified file in `files_changed` ends in `.go` before skipping `lucind-checks.sh`?
- [ ] Should `lucind-ai accept` support a CLI flag (e.g. `--force-checks`) to override phase-based check skipping during manual auditing?

## Citation Manifest

| citation | claim |
|---|---|
| `CONTEXT.md:23-26` | defines Coordination Scope as one local repository on one machine |
| `CONTEXT.md:51-54` | defines Acceptance as verified inclusion of a Lane result into its owning Change without human confirmation |
| `CONTEXT.md:91-94` | defines Promotion as the human-confirmed integration of a completed Change into its target |
| `CONTEXT.md:103-106` | defines Specialist as a phase-scoped Agent holding autonomous Acceptance authority for its own phase |
| `CONTEXT.md:107-109` | defines Phase Verdict as the compressed report returned by a Specialist to the Orchestrator |
| `cmd/lucind-ai/cli.go:2516-2517` | implements the deterministic phase subcommand dispatch entry point |
| `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:5-8` | records architectural decision granting phase-scoped Acceptance authority and scoping checks to apply phase |
| `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:16-19` | specifies consequences on SKILL.md Hard Rule carve-out, fan-out strategy, and Dual-Judge for Tier A |
| `docs/sdd-phase-specialist.md:7-10` | defines problem of Orchestrator context bloat and goal of compressed Phase Verdict |
| `docs/sdd-phase-specialist.md:21-24` | records resolved decisions on runtime substrate and tool-constrained dispatch |
| `docs/sdd-phase-specialist.md:21-30` | records resolved decisions on runtime substrate, Acceptance authority, Promotion exclusion, and checks gating |
| `internal/accept/accept.go:1-3` | establishes package accept as non-promoting evidence generator that never mutates refs |
| `internal/accept/accept.go:48-56` | declares Verifier struct fields with injected candidate loader, check function, clock, and UUID generator |
| `internal/accept/accept.go:49` | declares injected check function field on Verifier struct |
| `internal/accept/accept.go:50-51` | declares injected now and newID function fields on Verifier struct |
| `internal/accept/accept.go:55` | initializes Verifier with integrate.Check, time.Now, and uuid.NewString |
| `internal/accept/accept.go:80-100` | demonstrates Verifier generates immutable receipts without mutating git refs |
| `internal/accept/accept.go:84-96` | loads LaneMetadata conditionally only when AuthoringEvidenceVersion matches |
| `internal/accept/accept.go:97-98` | invokes validateResultAndScope to enforce result schema and path boundaries before checks |
| `internal/accept/accept.go:120-137` | executes checks unconditionally in owned isolation and fails acceptance if checks do not pass |
| `internal/accept/accept.go:149-158` | canonicalRoot enforces absolute evaluated symlink paths and rejects relative roots |
| `internal/accept/accept.go:198-212` | validateObjects verifies immutable commit and tree hashes against git object database |
| `internal/accept/accept.go:214-261` | validateResultAndScope validates result envelope schema, hard stops, done criteria, and allowed_paths containment |
| `internal/accept/accept.go:406-428` | createOwnedIsolation creates detached worktree with ownership marker at frozen candidate commit |
| `internal/accept/accept_test.go:26-67` | sets up verifierFixture with isolated git repo, lucind-checks.sh, and candidate registration |
| `internal/accept/accept_test.go:80-100` | tests verifier persistence of complete receipt and exact binding reuse |
| `internal/accept/accept_test.go:127-140` | tests verifier treats documentation-like files as scope-only without executing them |
| `internal/accept/accept_test.go:142-166` | tests verifier uses frozen detached candidate despite dirty primary working tree state |
| `internal/accept/accept_test.go:297-318` | tests verifier rejects relative roots, foreign roots, or candidate tree mismatches |
| `internal/integrate/integrate.go:120-148` | CheckPolicySnapshot captures check version, timeout, and environment variables |
| `internal/integrate/integrate.go:159-200` | executes lucind-checks.sh at worktree root with timeout and captures combined output |
| `internal/integrate/integrate_test.go:21-36` | provides initRepo test helper creating throwaway git repository |
| `internal/integrate/integrate_test.go:21-50` | provides test helpers for git repo initialization and execution in integrate tests |
| `internal/ledger/authoring.go:14` | declares AuthoringEvidenceVersion constant frozen at v1 |
| `internal/ledger/authoring.go:14-26` | defines AuthoringEvidenceVersion and AuthoringEvidence struct fields |
| `internal/ledger/authoring.go:26` | defines Contract as json.RawMessage allowing additive contract extensions without struct migration |
| `internal/ledger/authoring.go:44-75` | computes domain hash over serialized AuthoringEvidence and decodes frozen candidates |
| `internal/ledger/lanes_meta.go:20-47` | defines LaneMetadata struct carrying existing SDDPhase string field |
| `internal/ledger/lanes_meta.go:49-60` | updates lane metadata and appends snapshot to append-only event log transactionally |
| `internal/ledger/lanes_meta.go:104-115` | queries audited lane metadata snapshot from ledger database |
| `internal/ledger/schema.go:425-445` | defines DDL migration v9 to v10 adding authoring evidence columns and shadow tables |
| `internal/ledger/schema.go:584-592` | applies schema migration transactionally when database version is below 10 |
| `internal/packet/packet_test.go:924-941` | validates CONTEXT.md glossary projections match references/core/domain.md |
| `internal/packet/packet_test.go:943-967` | asserts Claude Code and OpenCode skill trees are byte-identical |
| `internal/run/attempt.go:415-460` | transitions attempt to checking status and executes checkFunc unconditionally |
| `internal/run/attempt.go:431-435` | defaults checkFunc to integrate.Check when deps.RunChecks is nil |
| `internal/run/attempt_test.go:24-44` | defines attemptSpies struct tracking createWorktree, combine, check, and promoteCAS calls |
| `internal/run/attempt_test.go:24-80` | constructs test dependencies, ledger, and feature service for attempt execution tests |
| `internal/run/attempt_test.go:41` | defines checkFunc field on attemptSpies struct |
| `internal/run/attempt_test.go:83-92` | records worktree path in checkCalls and invokes spies.checkFunc |
| `internal/run/run.go:165` | declares Ledger field on Deps struct |
| `internal/run/run.go:208` | declares RunChecks function field on Deps struct |
| `internal/run/run.go:841-845` | demotes candidate lane to Blocked if any declared hard stop fired |
| `internal/run/run.go:841-878` | demotes candidate lane on fired hard stops, out-of-scope diffs, or skill mismatches |
| `internal/run/run.go:856-878` | demotes lane to Deviated if git diff touched paths outside declared allowed_paths |
| `plugin/claude-code/skills/lucind-ai/SKILL.md:19-20` | defines Hard Rule restricting Acceptance and Promotion authority to Orchestrator |
| `plugin/claude-code/skills/lucind-ai/SKILL.md:21-22` | enforces byte-identity between Claude Code and OpenCode skill trees before worktree allocation |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:44-50` | defines Promotion gate requiring explicit human confirmation |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48` | assigns synthesis note review and contradiction arbitration directly to Orchestrator |
| `plugin/opencode/skills/lucind-ai/SKILL.md:19-20` | mirrors Hard Rule restricting Acceptance and Promotion authority to Orchestrator |
