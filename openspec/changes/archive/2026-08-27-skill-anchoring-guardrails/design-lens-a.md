# Design Lens A — Decisions: Skill Anchoring & Worktree Cleanup Guardrails

## Assumed architecture

`worktree.Cleanup` and `worktree.Remove` (`internal/worktree/worktree.go:247-261`) gain an explicit `force bool` parameter and export the `ErrWorktreeDirty` sentinel error (`internal/worktree/worktree.go:26-45`). `cmd/lucind-ai/cli.go:1934-1978` adds the `--force`/`-f` flag to `lucind-ai worktree cleanup` and embeds four static guidance-banner call sites (`cmd/lucind-ai/cli.go:485-516,685-690,698-726,730-740`). No new packages are introduced, and no database or ledger schema changes are required (`internal/ledger/acceptance.go:41-47`, `internal/result/result.go:26-34`).

## Technical Approach

The design directly implements the six delta requirements defined in the accepted proposal (`openspec/changes/skill-anchoring-guardrails/proposal.md:40-46`):
1. **Worktree cleanup dirty guardrail and force flag**: `worktree.Cleanup` and `worktree.Remove` (`internal/worktree/worktree.go:247-261`) check `PorcelainEmpty` (`internal/worktree/worktree.go:319-325`) when `force` is false, failing closed with `ErrWorktreeDirty` without deleting uncommitted files, while `lucind-ai worktree cleanup` (`cmd/lucind-ai/cli.go:1934-1978`) accepts `--force`/`-f` and prints dirty diagnostics on refusal.
2. **Blocked and timeout lane report guidance banner**: `printReport` (`cmd/lucind-ai/cli.go:698-726`) prints diagnostic steps and links `troubleshooting.md:7-18` upon non-done lane execution (`internal/run/run.go:452-465`).
3. **Integration report reverted IDs recovery banner**: `printIntegrateReport` (`cmd/lucind-ai/cli.go:730-740`) surfaces `lucind-ai integrate retry --run <run-id>` instructions referencing `recovery-reconciliation.md:33-35` when `reverted_ids` is non-empty.
4. **Acceptance receipt qualitative review banner**: `renderAcceptanceReceipt` (`cmd/lucind-ai/cli.go:685-690`) prints a reminder for manual checklist steps 2–10 in `acceptance-promotion.md:18-30` following mechanical validation (`internal/accept/accept.go:120-130`).
5. **DAG split multi-wave base SHA warning banner**: `runSplit` (`cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43`) emits a warning banner to advance primary checkout and refresh `base_sha`/`expected_parent_sha` across sequential waves per `recovery-reconciliation.md:27-30`.
6. **Prescriptive TDD WIP-rescue protocol documentation**: Standardize the operator rescue workflow in `troubleshooting.md:7-18` and `.agents/skills/lucind-apply/SKILL.md:10-21` to inspect preserved worktrees (`internal/worktree/worktree.go:150-159`), commit partial work, and re-dispatch after lane timeouts.

## Decision 1 — Signature shape: `force bool` parameter vs. wrapper function

**Choice**: Extend `worktree.Cleanup(ctx context.Context, primaryRoot, laneID string, force bool) error` and `worktree.Remove(ctx context.Context, primaryRoot, path string, force bool) error` by adding a trailing `force bool` parameter directly.
**Alternatives considered**: Separate helper wrappers like `worktree.RemoveForced` / `worktree.CleanupForced`, or functional options (`worktree.WithForce()`), or a config struct (`worktree.RemoveOptions{Force: bool}`).
**Rationale**: There are only four internal deletion call sites across the codebase (`cmd/lucind-ai/cli.go:858-869`, `internal/integrate/integrate.go:118-124`, `internal/integrate/candidate.go:262-265`, `internal/run/integrate.go:159-165`) and one CLI call site (`cmd/lucind-ai/cli.go:1934-1978`). Adding `force bool` directly makes deletion semantics an explicit, compile-time verified decision at every call site. Internal callers managing transient scratch trees explicitly pass `force: true`, while the CLI passes the parsed flag value (`*forceFlag`), avoiding redundant wrapper functions and struct allocation overhead.
**Terminal consumer**: `cmd/lucind-ai/cli.go:1934-1978` (where `runWorktreeCleanup` passes `*forceFlag` to `worktree.Cleanup`), `internal/integrate/integrate.go:118-124` (where `Combine` abort passes `force: true` to `worktree.Remove`), `internal/integrate/candidate.go:262-265` (where `ResolveCandidate` teardown passes `force: true` to `worktree.Remove`), and `cmd/lucind-ai/cli.go:858-869` (where `DiscardCombined` and `RemoveLaneWorktree` pass `force: true` to `worktree.Remove`).

## Decision 2 — Sentinel error placement and shape (`ErrWorktreeDirty`)

**Choice**: Export `var ErrWorktreeDirty = errors.New("worktree: worktree has uncommitted changes")` in `internal/worktree/worktree.go:26-45`.
**Alternatives considered**: A structured custom error type (`type DirtyWorktreeError struct { Path, Status string }`) or unexported error strings matched via substring search in the CLI.
**Rationale**: Matches the established sentinel error convention throughout `internal/worktree` (`ErrEmptyLaneID`, `ErrWorktreeExists`, `ErrEmptyBaseSHA`, `ErrInvalidParentRef`, `ErrInvalidBaseSHA` in `internal/worktree/worktree.go:26-45`). It allows `cmd/lucind-ai` to branch cleanly using `errors.Is(err, worktree.ErrWorktreeDirty)` to format user-facing diagnostics, print porcelain status, and exit 1 without allocating heavy error structs or relying on brittle error string formatting.
**Terminal consumer**: `cmd/lucind-ai/cli.go:1934-1978` (where `runWorktreeCleanup` checks `errors.Is(err, worktree.ErrWorktreeDirty)` to output dirty status, diagnostic guidance, and exit 1).

## Decision 3 — Where dirty-check logic lives (inline vs. reusing `PorcelainEmpty`)

**Choice**: Invoke the existing `worktree.PorcelainEmpty(ctx, path)` helper (`internal/worktree/worktree.go:319-325`) directly within `worktree.Remove` whenever `force == false`.
**Alternatives considered**: Inlining raw `git status --porcelain` command execution inside `worktree.Remove`, or enforcing the dirty check exclusively in `cmd/lucind-ai` before invoking `worktree.Cleanup`.
**Rationale**: `PorcelainEmpty` already encapsulates gitignore-respecting status inspection and has proven test coverage (`internal/worktree/worktree.go:319-325`, `internal/worktree/worktree_test.go:536-595`). Placing the guardrail inside `worktree.Remove` enforces fail-closed safety at the core package boundary, ensuring no caller (CLI, integration runner, or test utility) can unintentionally delete dirty worktrees without explicitly opting in via `force: true`. Nonexistent worktrees continue to be handled idempotently in `worktree.Cleanup` via `os.Stat` before reaching `Remove` (`internal/worktree/worktree.go:247-261`, `internal/worktree/worktree_test.go:1034-1069`).
**Terminal consumer**: `internal/worktree/worktree.go:247-261` (in `worktree.Remove`, which calls `PorcelainEmpty` before issuing `git worktree remove --force`).

## Decision 4 — Banner call-site strategy (helper function vs. inline prints)

**Choice**: Implement targeted static prints directly at the four designated command and reporting call sites in `cmd/lucind-ai/cli.go` and `internal/dag/split.go`.
**Alternatives considered**: A dynamic banner registry or template-rendering subsystem with Markdown parsers, or dynamically reading Markdown excerpts from `plugin/claude-code/skills/lucind-ai/references/` at CLI runtime.
**Rationale**: The four banner contexts are compile-time static and directly bound to exact exit/report conditions: `printReport` (`cmd/lucind-ai/cli.go:698-726`), `printIntegrateReport` (`cmd/lucind-ai/cli.go:730-740`), `renderAcceptanceReceipt` (`cmd/lucind-ai/cli.go:685-690`), and `runSplit`/`dag.Split` (`cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43`). Direct static formatting eliminates runtime filesystem dependencies, prevents parsing failures, avoids latency, and preserves strict separation of stdout (records/commands) and stderr (warnings/diagnostics).
**Terminal consumer**: `cmd/lucind-ai/cli.go:698-726` (`printReport` emitting `troubleshooting.md:7-18` banner on non-done status), `cmd/lucind-ai/cli.go:730-740` (`printIntegrateReport` emitting `recovery-reconciliation.md:33-35` retry instructions on `reverted_ids`), `cmd/lucind-ai/cli.go:685-690` (`renderAcceptanceReceipt` emitting `acceptance-promotion.md:18-30` qualitative checklist reminder), and `cmd/lucind-ai/cli.go:485-516` / `internal/dag/split.go:34-43` (`runSplit` emitting `recovery-reconciliation.md:27-30` multi-wave base SHA warning).

## Open Questions

- [ ] Stderr vs. stdout stream routing for banners: Proposal Open Question 1 asks whether warning banners for `lucind-ai accept` and `lucind-ai split` should route exclusively to `stderr` or append to `stdout` after records.
- [ ] Internal caller cleanup signature convention: Proposal Open Question 2 asks whether internal teardowns (`cmd/lucind-ai/cli.go:858-869`, `internal/integrate/integrate.go:118-124`, `internal/integrate/candidate.go:262-265`) should pass `force: true` inline or use an explicit `RemoveForced` helper.
- [ ] Design document scope convention: `~/.claude/skills/sdd-design/SKILL.md` defines a full single-agent `design.md` under 800 words, whereas this packet specifies authoring `design-lens-a.md` (under 1000 words) as an architectural decision feedstock for a subsequent synthesis phase. The packet authority supersedes the skill here.

## Citation Manifest

| citation | claim |
|---|---|
| `.agents/skills/lucind-apply/SKILL.md:10-21` | Apply lane guide defining strict TDD RED/GREEN/SWEEP phases |
| `cmd/lucind-ai/cli.go:485-516` | runSplit executing DAG split and outputting wave commands |
| `cmd/lucind-ai/cli.go:685-690` | renderAcceptanceReceipt formatting acceptance receipt metadata |
| `cmd/lucind-ai/cli.go:698-726` | printReport printing lane summary and non-done banner |
| `cmd/lucind-ai/cli.go:730-740` | printIntegrateReport formatting integration results and reverted IDs |
| `cmd/lucind-ai/cli.go:858-869` | DiscardCombined and RemoveLaneWorktree teardowns calling worktree.Remove |
| `cmd/lucind-ai/cli.go:1934-1978` | runWorktreeCleanup executing worktree cleanup CLI command |
| `cmd/lucind-ai/cli_test.go:685-777` | Tests for printReport and printIntegrateReport output formatting |
| `cmd/lucind-ai/cli_test.go:2974-3010` | Tests verifying worktree cleanup CLI behavior |
| `cmd/lucind-ai/cli_test.go:4503-4545` | Tests verifying accept receipt stdout and mechanical evidence |
| `internal/accept/accept.go:120-130` | Verify creating and persisting AcceptanceReceipt in ledger |
| `internal/dag/split.go:34-43` | Split formatting wave commands to stdout |
| `internal/dag/split_test.go:13-111` | Tests verifying DAG wave split execution and stdout commands |
| `internal/integrate/candidate.go:262-265` | ResolveCandidate teardown calling worktree.Remove and DeleteBranch |
| `internal/integrate/integrate.go:118-124` | Combine merge conflict abort calling worktree.Remove |
| `internal/ledger/acceptance.go:41-47` | AcceptanceReceipt struct definition in ledger package |
| `internal/result/result.go:26-34` | HardStop struct definition in result package |
| `internal/run/integrate.go:159-165` | completeIntegration calling RemoveLaneWorktree |
| `internal/run/run.go:452-465` | Execute persisting non-done lane status and diagnosis |
| `internal/worktree/worktree.go:26-45` | Sentinel error definitions in worktree package |
| `internal/worktree/worktree.go:150-159` | pathFor returning canonical linked worktree filesystem path |
| `internal/worktree/worktree.go:247-261` | Cleanup and Remove function implementations |
| `internal/worktree/worktree.go:319-325` | PorcelainEmpty reporting whether git status --porcelain is empty |
| `internal/worktree/worktree_test.go:255-266` | TestRemove verifying worktree removal behavior |
| `internal/worktree/worktree_test.go:536-595` | TestPorcelainEmpty verifying status checks on fresh, ignored, and untracked files |
| `internal/worktree/worktree_test.go:1034-1069` | TestCleanupRemovesExistingWorktree and TestCleanupOnLaneWithNoWorktreeIsNoOp |
| `openspec/changes/skill-anchoring-guardrails/proposal.md:40-46` | Accepted proposal approach section defining core strategy |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30` | Acceptance checklist steps 1 through 10 |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30` | Multi-wave sequencing guidance for advancing base SHA |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:33-35` | Bisection boundary and integrate retry recovery guidance |
| `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` | Troubleshooting table for dispatch, worktree cleanup, and recovery |
