# Proposal: Apply DAG Dispatch

## Intent
Replace the `sdd-apply` sub-agent's direct local Read/Edit/Write file modifications with structured, parallel binary dispatch via `lucind-ai run`. By decomposing an SDD change's `tasks.md` into independent task packets with non-overlapping `allowed_paths`, work is dispatched in dependency order (waves/DAG) across isolated git worktrees and integrated using the existing bisection, conflict-resolution, and combine machinery.

## Scope

### In Scope
- **Task decomposition into packet waves**: Transforming `tasks.md` items into independent dispatch packets (`packet.Packet`) grouped into dependency waves where concurrent tasks share no file scope.
- **Wave-based dispatch orchestration**: Dispatching waves of independent packets via `lucind-ai run`, ensuring wave $N$ completes and integrates before wave $N+1$ begins.
- **Integration of wave outcomes**: Merging completed lane branches into the primary tree after each wave using existing integration routines (`internal/run/integrate.go:30-60`).
- **Orchestration evolution**: Updating the `apply` phase in `plugin/claude-code/skills/lucind-ai/SKILL.md:79` to drive `lucind-ai run` instead of performing in-tree direct edits via `sdd-apply`.

### What Stays Untouched
- **Integration, bisection, and conflict resolution**: Reusing without modification `bisect` (`internal/run/integrate.go:183-220`), LLM conflict resolution bounded to 400 lines (`internal/resolve/resolve.go:18`, `internal/resolve/resolve.go:27-43`), and `Combine` (`internal/integrate/integrate.go:34-70`), which are verified and working per PRD §6 steps 6-8 (`docs/prd.md:126-137`).
- **Status vocabulary and barrier rules**: Retaining the 6-value `lane.Status` enum (`internal/lane/status.go:10-18`) and the requirement that barriers release only when all lanes reach terminal state (`internal/run/batch.go:25-27`, `internal/barrier/barrier.go:22-31`).
- **Ledger architecture**: Keeping the append-only SQLite event ledger and existing event types (`internal/ledger/ledger.go:348-365`).
- **Worktree isolation guarantees**: Preserving linked worktree creation and per-lane isolation semantics (`internal/worktree/worktree.go:20-45`).

## Non-goals
- **No read-only-packet design**: Read-only packet dispatch is a separate, in-flight sibling change (`read-only-packet-dispatch`). Apply-DAG lanes are purely mutating code tasks that commit file changes on dedicated branches.
- **No verify dual-dispatch design**: Dual-executor verification and qualitative comparisons are scoped to the sibling change `verify-dual-dispatch`.
- **No redesign of integration or bisection**: We do not replace or re-architect `internal/run/integrate.go`, `internal/resolve/resolve.go`, or `internal/integrate/integrate.go`.
- **No changes to approvals web UI**: The localhost approval UI and schema v3 (`openspec/changes/approvals-web-ui/`) remain separate and untouched.

## Capabilities

### New Capabilities
- `apply-dag-dispatch`: Orchestrator capability to decompose `tasks.md` into dependency waves of packets with non-overlapping `allowed_paths`, invoking `lucind-ai run` per wave and handling wave progression.

### Modified Capabilities
- `sdd-apply`: Shifts from executing direct filesystem edits in the primary repository to authoring packet files, driving batch executions, and handling returned integration reports.

## Approach
Today, `ExecuteBatch` (`internal/run/batch.go:66-113`) runs a single flat batch of lanes concurrently via `sync.WaitGroup` (`internal/run/batch.go:81-89`) and evaluates a single shared barrier. The CLI (`cmd/lucind-ai/cli.go:39`, `cmd/lucind-ai/cli.go:103-133`) accepts multiple `--packet` flags and runs them as one concurrent batch followed by `lucindrun.Integrate` (`cmd/lucind-ai/cli.go:220`).

To execute `tasks.md` as a DAG:
1. The orchestrator partitions `tasks.md` into sequential waves ($W_1, W_2, \dots, W_k$). Within any single wave $W_i$, all task packets are strictly independent and have mutually exclusive `allowed_paths`.
2. Each wave $W_i$ is dispatched via `lucind-ai run --packet ...`.
3. The binary executes the wave concurrently in isolated worktrees, joins at the barrier, and executes `Integrate` (`internal/run/integrate.go:30-60`).
4. The existing `IntegrateReport` (`internal/run/integrate.go:14-21`) and ledger events (`internal/ledger/ledger.go:358-365`) communicate which lanes integrated and which were bisected or reverted.
5. If all lanes in $W_i$ integrate cleanly (`IntegrateReport.Passed == true`), the orchestrator advances to $W_{i+1}$. If any lane fails or is reverted to `blocked`, the run halts for human review or replanning before subsequent waves are dispatched.

### Risk Framing: Declared Scope vs. Actual Diff
This change carries a major structural uncertainty flagged in prior exploration: there is currently zero code precedent in `lucind-ai` for enforcing declared `allowed_paths` against actual git diffs.
- Today, `Packet` (`internal/packet/packet.go:29-47`) has no `AllowedPaths` field, and `packet.Parse` (`internal/packet/packet.go:66-76`) does not parse allowed paths from frontmatter.
- `allowed_paths` exists purely as Markdown prose in the packet prompt body (`plugin/claude-code/skills/lucind-ai/assets/packet-template.md:48-55`) and relies entirely on executor self-policing via hard-stop conventions.
- `result.Envelope` (`internal/result/result.go:102-115`) records `files_changed` (`internal/result/result.go:40-44`), but the binary does not validate this against the git diff or against the declared packet scope before integration.

The design phase must explicitly resolve whether and how scope enforcement is verified.

## Open Design Questions for the Design Phase
1. **Exact DAG / wave representation for `tasks.md`**:
   - How should dependency ordering and wave boundaries be represented in `tasks.md`? Options include:
     - Explicit Markdown wave sections (e.g. `### Wave 1`, `### Wave 2`).
     - Dependency annotations on task headers (e.g. `depends_on: [task-1]`).
     - Purely orchestrator-inferred scheduling from declared file scopes.
2. **Binary enforcement vs. prompt convention for `allowed_paths`**:
   - Should `allowed_paths` remain a prompt-level contract self-reported in `result.json`'s `files_changed` (`internal/result/result.go:40-44`, `internal/result/result.schema.json:44-57`), or should `packet.Packet` (`internal/packet/packet.go:29-47`) gain a structured `AllowedPaths []string` field verified by `internal/run` against `git diff --name-only` before integration?
3. **Partial wave failures and downstream pruning**:
   - When a task in Wave $N$ fails, reverts, or returns `blocked`, how should the orchestrator handle dependent tasks in Wave $N+1$? Should independent branches in Wave $N+1$ continue, or should execution stop immediately at the wave boundary?

## Impact on Existing `sdd-apply` Flow
- **Elimination of context pollution and workspace drift**: Rather than running multiple editing turns in the working tree where uncommitted edits can break adjacent files (`docs/prd.md:167-171`), tasks execute in pristine, isolated linked worktrees.
- **Deterministic fallback**: Merge conflicts and build regressions are isolated automatically via `bisect` (`internal/run/integrate.go:183-220`) and resolved via bounded Claude invokers (`internal/resolve/resolve.go:27-43`), preventing whole-batch pollution.
- **Structured observability**: All task progress, lane notes, and barrier outcomes are durably recorded in SQLite (`internal/ledger/ledger.go:371-405`) instead of ephemeral sub-agent chat context.

## Rollback Plan
If DAG dispatch encounters issues during apply:
1. The orchestrator skill (`plugin/claude-code/skills/lucind-ai/SKILL.md`) can be immediately reverted to invoke direct `sdd-apply` file modifications.
2. The binary changes (`lucind-ai run`) are additive and backwards-compatible with single-packet and flat-batch dispatches, introducing no breaking changes to existing repositories or ledger schemas.
