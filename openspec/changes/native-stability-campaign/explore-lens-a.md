# Explore Lens A — Problem & Candidates: Native Stability Campaign

## Problem Space

Lucind AI coordinates independent units of repository work across isolated git worktrees (`internal/worktree/worktree.go:180-228`), dispatches headless AI worker agents via executors such as `agy` (`internal/executor/agy.go:12-40`, `internal/executor/agy.go:68-80`), and records lane states and lease acquisitions in a primary SQLite ledger at `<primaryRoot>/.lucind/lucind.db` (`internal/ledger/ledger.go:146-168`, `internal/ledgerpath/ledgerpath.go:34-38`). The CLI provides subcommands for running packet batches, DAG partitioning, baseline validation, server dispatch, feature registration, and merge reconciliation (`cmd/lucind-ai/cli.go:56-57`, `cmd/lucind-ai/cli.go:123-145`).

However, the repository currently lacks a native, repeatable release-certification capability. As established in ADR-0001 (`docs/adr/0001-native-stability-campaign.md:5-13`), validating candidate stability via external harnesses places release evidence, binary verification, and crash recovery semantics outside product authority. Furthermore, ordinary batch runs (`cmd/lucind-ai/cli.go:147-150`, `internal/run/run.go:48-60`) execute isolated tasks on individual feature branches (`internal/run/run.go:43-46`, `internal/feature/feature.go:98-113`, `internal/feature/feature.go:115-130`) without multi-trial lifecycle orchestration, lease expiration recovery, cross-Change defect remediation, or immutable receipt generation.

To provide release confidence within product boundaries, Lucind AI requires a native, Linux-only `lucind-ai stability` command group (`run`, `status [--json]`, `resume`, `abort`) (`lucind-ai-stability-run-sdd-master-plan.md:7-16`). This capability must validate an immutable candidate commit against three consecutive, deterministic Stability Trials using real `agy` dispatches (`docs/adr/0001-native-stability-campaign.md:15-24`, `lucind-ai-stability-run-sdd-master-plan.md:59-86`), isolate mutable lifecycle state in SQLite/WAL under `<git-common-dir>/lucind-ai/stability/v1/` (`docs/adr/0001-native-stability-campaign.md:15-24`, `lucind-ai-stability-run-sdd-master-plan.md:141-143`), and emit a content-addressed Stability Receipt.

## Candidate Approaches

### Candidate 1 — Monolithic Extension of Existing Run and Ledger Subsystems

**Approach**: Extend the existing `lucind-ai run` command (`cmd/lucind-ai/cli.go:147-150`, `internal/run/run.go:48-60`) and primary SQLite ledger (`internal/ledger/ledger.go:146-168`, `internal/ledgerpath/ledgerpath.go:34-38`) with Campaign/Trial tables, multi-trial loops, and receipt serialization within the primary repository's `.lucind/` directory.
**Pros**: Maximizes reuse of existing database migration mechanisms and execution helpers; avoids adding new top-level package roots.
**Cons**: Couples ephemeral feature runs with long-lived release validation authority; pollutes the primary ledger schema; fails ADR-0001 requirement (`docs/adr/0001-native-stability-campaign.md:15-24`) and Master Plan Decision 67 (`lucind-ai-stability-run-sdd-master-plan.md:141-143`) for dedicated Git common-dir storage.
**Feasibility**: Infeasible. The ledger path validator (`internal/ledgerpath/ledgerpath.go:40-58`) explicitly rejects paths outside `<primaryRoot>/.lucind/`, preventing authority co-location in `<git-common-dir>/lucind-ai/stability/v1/`.

### Candidate 2 — Modular Subpackage Decomposition under `internal/stability`

**Approach**: Implement `stability run|status|resume|abort` as a thin CLI projection (`cmd/lucind-ai/cli.go:123-145`) backed by a dedicated `internal/stability` engine partitioned into focused subpackages: `store` (Git common-dir SQLite/WAL authority), `fixture` (embedded deterministic templates/checks), `process` (Linux process groups and survivor proof), `evidence` (sanitization and receipts), and `reconcile` (resume/abort cleanup) (`lucind-ai-stability-run-sdd-master-plan.md:240-252`). Existing primitives (`internal/worktree/worktree.go:180-228`, `internal/executor/agy.go:68-80`, `internal/integrate/integrate.go:52-85`, `internal/integrate/candidate.go:48-60`, `internal/reconcile/reconcile.go:1-33`) are consumed directly.
**Pros**: Enforces clean separation of concerns; keeps `cmd/lucind-ai/cli.go` thin; isolates SQLite schema from ordinary feature runs; cleanly encapsulates Linux-specific process-group management; provides disjoint package Write Scopes for parallel SDD implementation waves.
**Cons**: Requires interface contracts between `store`, `process`, and `evidence` subpackages to prevent circular dependencies.
**Feasibility**: High. Integrates with existing primitives like `worktree.GitRunner` (`internal/worktree/worktree.go:47-69`), `executor.Agy` (`internal/executor/agy.go:68-80`), and in-memory join primitives (`internal/barrier/barrier.go:36-60`, `internal/overlap/overlap.go:21-33`).

### Candidate 3 — Consolidated Flat `internal/stability` Package

**Approach**: Implement the stability command family as a thin CLI router (`cmd/lucind-ai/cli.go:123-145`) backed by a single flat `internal/stability` package where state machine transitions (`lucind-ai-stability-run-sdd-master-plan.md:254-273`), SQLite persistence, embedded fixtures, process control, and receipt generation reside together across file boundaries in a single package namespace.
**Pros**: Avoids cross-subpackage boilerplate interfaces and adapters; simplifies atomic transactions across SQLite store and state machine transitions; eliminates package import cycle risks.
**Cons**: Yields large, tightly coupled source files; mixes SQLite storage primitives with low-level Linux process-group handling; hampers parallel SDD implementation waves by creating file-level merge conflicts across concurrent implementation lanes.
**Feasibility**: Medium. Compiles cleanly in Go, but hinders modular unit testing and contradicts repository conventions of decomposing distinct concerns into domain packages (`internal/barrier/`, `internal/integrate/`, `internal/ledger/`, `internal/reconcile/`).

## Initial Recommendations

Recommend **Candidate 2 (Modular Subpackage Decomposition under `internal/stability`)**.

**Technical Rationale**:
1. **ADR-0001 and Storage Isolation**: Directly fulfills the requirement for dedicated SQLite/WAL authority under `<git-common-dir>/lucind-ai/stability/v1/` (`docs/adr/0001-native-stability-campaign.md:15-24`, `lucind-ai-stability-run-sdd-master-plan.md:141-143`) without modifying `internal/ledger/` or breaking `internal/ledgerpath/ledgerpath.go:40-58`.
2. **OS Process Boundary**: Linux process-group management, abrupt SIGKILL termination, and grandchild survivor detection are cleanly isolated in `internal/stability/process`, avoiding pollution of domain state machine logic.
3. **SDD Fan-Out Alignment**: Distinct subpackage directories provide disjoint Write Scopes, enabling parallel implementation waves (I1–I3) without file write collisions.
4. **Architectural Consistency**: Preserves `cmd/lucind-ai/cli.go` as a thin dispatch layer (`cmd/lucind-ai/cli.go:123-145`), placing all lifecycle invariants in the domain engine (`lucind-ai-stability-run-sdd-master-plan.md:240-252`).

## Open Questions

- [ ] Should `internal/stability/reconcile` share abstractions with `internal/reconcile/reconcile.go:1-33` or remain separate given that stability reconciliation handles crashed-trial worktree/ref cleanup rather than feature merge overlap?
- [ ] How will `internal/stability/store` resolve `<git-common-dir>` portably across primary checkouts and linked worktrees using `worktree.GitRunner` (`internal/worktree/worktree.go:47-69`)?

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:56-57` | usage string defines existing CLI subcommand routing |
| `cmd/lucind-ai/cli.go:123-145` | run dispatcher routes subcommands to package handlers |
| `cmd/lucind-ai/cli.go:147-150` | runDispatch executes ordinary packet batches via run.ExecuteBatch |
| `docs/adr/0001-native-stability-campaign.md:5-13` | ADR 0001 establishes native stability campaign rationale over external harnesses |
| `docs/adr/0001-native-stability-campaign.md:15-24` | ADR 0001 defines 3-trial execution model, Linux constraint, and storage path |
| `internal/barrier/barrier.go:36-60` | Evaluate performs in-memory join over lane terminal states |
| `internal/executor/agy.go:12-40` | Agy executor defines binary invocation and stdio pipe wait delay |
| `internal/executor/agy.go:68-80` | Agy struct implements headless executor dispatch |
| `internal/feature/feature.go:98-113` | ValidateParentRef rejects empty, main, and lucind namespace refs |
| `internal/feature/feature.go:115-130` | Create registers features with immutable base SHAs in ledger |
| `internal/integrate/candidate.go:48-60` | ResolveAndPromoteCandidate performs bounded conflict resolution and checks |
| `internal/integrate/integrate.go:52-85` | Combine merges branches into temporary integration worktree |
| `internal/ledger/ledger.go:146-168` | Open initializes primary SQLite ledger with WAL mode and foreign keys |
| `internal/ledgerpath/ledgerpath.go:34-38` | Resolve determines SQLite ledger path under primary root |
| `internal/ledgerpath/ledgerpath.go:40-58` | Validate rejects database paths outside primary root .lucind dir |
| `internal/overlap/overlap.go:21-33` | overlap package classifies merge base divergence classes |
| `internal/reconcile/reconcile.go:1-33` | reconcile package manages direction approval and candidate lifecycles |
| `internal/run/run.go:43-46` | ErrMissingFeatureTarget enforces required feature target parameters |
| `internal/run/run.go:48-60` | lucindDir and resultEnvelopePath define worktree-relative envelope path |
| `internal/worktree/worktree.go:47-69` | GitRunner and DefaultGitRunner provide git command execution interface |
| `internal/worktree/worktree.go:180-228` | createWithRunner creates linked git worktrees with parent ancestry |
| `lucind-ai-stability-run-sdd-master-plan.md:7-16` | Master Plan defines public stability command surface |
| `lucind-ai-stability-run-sdd-master-plan.md:59-86` | Master Plan decision ledger specifies 3-trial journey rules |
| `lucind-ai-stability-run-sdd-master-plan.md:141-143` | Decision 67 establishes separate stability storage under git-common-dir |
| `lucind-ai-stability-run-sdd-master-plan.md:240-252` | Architecture plan defines suggested internal/stability package seams |
| `lucind-ai-stability-run-sdd-master-plan.md:254-273` | State model defines Campaign and Trial state transitions |
