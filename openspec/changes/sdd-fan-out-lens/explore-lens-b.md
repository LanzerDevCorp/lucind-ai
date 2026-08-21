# Explore Lens B — Constraints: sdd-fan-out-lens

## Sidecar schema: what it can and cannot express

| Capability | Status | Citation |
|---|---|---|
| Directed acyclic dependency declaration (`depends_on`) | Supported | `internal/dag/parse.go:34`, `internal/dag/waves.go:25-63` |
| Execution wave grouping (Kahn's algorithm) | Supported | `internal/dag/waves.go:41-63` |
| Disjoint same-wave execution | Enforced | `internal/dag/overlap.go:52-78`, `internal/dag/waves.go:68-70` |
| Transitive cross-wave overlap authorization | Supported | `internal/dag/overlap.go:15-35,71-75` |
| Target ref specification (`feature`, `parent_ref`, `base_sha`, `expected_parent_sha`, `legacy_main`) | Supported | `internal/dag/parse.go:28-32`, `internal/dag/emit.go:33-47` |
| Path scoping (`allowed_paths`) | Required non-empty | `internal/dag/parse.go:33`, `internal/dag/validate.go:30-37` |
| Read-only lane declaration (`read_only`) | Expressly missing | `internal/dag/parse.go:22-36`, `internal/dag/emit.go:23-53` |
| Empty or omitted `allowed_paths` | Forbidden at split time | `internal/dag/validate.go:11,30-32` |
| Non-apply phase DAG artifacts | Forbidden (hardcoded apply sidecar) | `internal/dag/parse.go:40-42`, `openspec/specs/apply-dag-dispatch/spec.md:9-12` |
| Body file existence validation | Required on disk | `internal/dag/parse.go:75-86` |

## Packet frontmatter surface

- **Accepted keys**: `id`, `executor`, `routed_by`, `model`, `agent`, `read_only`, `feature`, `parent_ref`, `base_sha`, `expected_parent_sha`, `legacy_main`, `allowed_paths` (`internal/packet/packet.go:94-138`).
- **Required keys**: `id` (`ErrMissingID`), `executor` (`ErrMissingExecutor`), `routed_by` (`ErrMissingRoutedBy`), non-empty Markdown `Body` (`ErrEmptyBody`) (`packet.go:23-26,155-165`).
- **Typing and defaults**: `read_only` (bool, default `false`; `packet.go:57-58,105-113`), `legacy_main` (bool, default `false`; `packet.go:71-72,122-130`), `allowed_paths` (JSON array, default `nil`; `packet.go:59-61,131-137`).
- **Unknown keys**: Silently ignored by parser (`packet.go:94-139`, `openspec/specs/read-only-packet-schema/spec.md:42-46`).

## Accepted specifications in scope

| Spec requirement | Would be | Citation |
|---|---|---|
| Frontmatter read-only field parsing | Honored unchanged | `openspec/specs/read-only-packet-schema/spec.md:9-27` |
| Default value and backward compatibility | Honored unchanged | `openspec/specs/read-only-packet-schema/spec.md:28-46` |
| Explicit flag only — no inference | Honored unchanged | `openspec/specs/read-only-packet-schema/spec.md:47-60` |
| Envelope cannot declare or override mode | Honored unchanged | `openspec/specs/read-only-packet-schema/spec.md:61-74` |
| Write packets keep criterion 2 unchanged | Honored unchanged | `openspec/specs/read-only-done-criterion/spec.md:9-17` |
| Read-only packets replace criterion 2 | Honored unchanged | `openspec/specs/read-only-done-criterion/spec.md:18-31` |
| Protocol envelope is not a mutation | Honored unchanged | `openspec/specs/read-only-done-criterion/spec.md:32-40` |
| Packet `AllowedPaths` field | Honored unchanged | `openspec/specs/allowed-paths-enforcement/spec.md:9-27` |
| Omitting `AllowedPaths` preserves today's exact path | Honored unchanged | `openspec/specs/allowed-paths-enforcement/spec.md:28-41` |
| Upfront batch disjointness check | Honored unchanged | `openspec/specs/allowed-paths-enforcement/spec.md:42-60` |
| Base-SHA 4-way diff union defines actual diff | Honored unchanged | `openspec/specs/allowed-paths-enforcement/spec.md:61-94` |
| Post-execution scope check demotes Done to Deviated | Honored unchanged | `openspec/specs/allowed-paths-enforcement/spec.md:95-118` |
| Post-status git verification, not envelope trust | Honored unchanged | `openspec/specs/completion-mode-enforcement/spec.md:9-27` |
| Write packet completion matrix (>=1 commit, clean tree) | Honored unchanged | `openspec/specs/completion-mode-enforcement/spec.md:28-46` |
| Read-only packet completion matrix (0 commits, clean tree) | Honored unchanged | `openspec/specs/completion-mode-enforcement/spec.md:47-65` |
| Combine stays unaware of read-only lanes | Honored unchanged | `openspec/specs/completion-mode-enforcement/spec.md:75-83` |
| Sequential run per wave | Honored unchanged | `openspec/specs/apply-dag-dispatch/spec.md:117-135` |
| Sidecar DAG artifact (`apply-dag.yaml`) | Modified | `openspec/specs/apply-dag-dispatch/spec.md:9-27` |
| Non-empty `allowed_paths` at split | Modified | `openspec/specs/apply-dag-dispatch/spec.md:51-59` |
| Dual parallel judgment dispatch and barrier join | Honored unchanged | `openspec/specs/verify-dual-dispatch/spec.md:29-51` |
| Canonical report synthesis from parallel findings | Honored unchanged | `openspec/specs/verify-dual-dispatch/spec.md:145-167` |

## Runtime invariants a fan-out depends on

- **Worktree isolation**: Lanes run in separate git worktrees (`internal/worktree/worktree.go:168-238`, `internal/run/batch.go:81-89`) without visibility into sibling uncommitted or unpromoted changes (`internal/run/batch.go:37-43`).
- **Data visibility and exchange**: `.lucind/result.json` is gitignored and worktree-local (`internal/run/run.go:50-60,599-601`). Inter-lane artifact consumption across waves requires Wave 1 promotion into parent/primary branch before Wave 2 worktrees branch from updated `HEAD` (`internal/run/integrate.go:62-79`, `openspec/specs/apply-dag-dispatch/spec.md:121-125`).
- **Integration timing**: Batch barrier waits for all lanes in a wave to reach terminal status (`internal/run/batch.go:88-95`). Integration runs post-barrier via `deps.CombineTree` and CAS target `HEAD` update (`internal/run/integrate.go:31-81`, `openspec/specs/parent-feature-integration/spec.md:33-46`).
- **Post-execution `allowed_paths` enforcement**: `enforceAllowedPaths` evaluates 4-way diff union against recorded birth `BaseSHA` (`internal/run/run.go:547-626`). Out-of-scope diffs demote `Done` to `Deviated` (`internal/run/run.go:621-623`), excluding the lane from integration (`internal/barrier/barrier.go:49-57`, `openspec/specs/allowed-paths-enforcement/spec.md:109-113`).
- **Empty or absent path list semantics**: Omitted/empty `AllowedPaths` skips CLI disjointness and post-execution diff checks (`internal/packet/disjoint.go:30-37`, `internal/run/run.go:379-381`, `openspec/specs/allowed-paths-enforcement/spec.md:28-41`). In sidecar DAGs, empty `allowed_paths` is unconditionally rejected (`internal/dag/validate.go:30-32`, `openspec/specs/apply-dag-dispatch/spec.md:51-59`).

## Hard blockers

- **Sidecar DAG path (`apply-dag.yaml` / `lucind-ai split`)**:
  - **Read-only fan-out is blocked today**: `dag.Node` lacks `read_only` (`internal/dag/parse.go:22-36`), `dag.Validate` rejects empty `allowed_paths` (`internal/dag/validate.go:11,30-32`), and `dag.EmitPacketContent` cannot emit `read_only: true` (`internal/dag/emit.go:23-53`).
  - **Phase-location coupling**: Sidecar is bound to `apply-dag.yaml` (`openspec/specs/apply-dag-dispatch/spec.md:9-12`, `internal/dag/parse.go:38-42`). Write fan-out is expressible only via `apply-dag.yaml` declaring non-empty disjoint paths.
- **Hand-authored packet path (`lucind-ai run --packet ...`)**:
  - **Fully expressible today (Not Blocked)**:
    - **Write fan-out**: Wave 1 dispatches parallel write packets with disjoint `allowed_paths` (`internal/packet/packet.go:131-137`, `cmd/lucind-ai/cli.go:243`), verified by write completion mode (`internal/run/run.go:645-653`), integrated at barrier (`internal/run/integrate.go:62-79`); Wave 2 dispatches synthesis reading merged drafts from `HEAD` (`internal/run/batch.go:66-113`).
    - **Read-only fan-out**: Parallel packets declare `read_only: true` and omit `allowed_paths` (`internal/packet/packet.go:105-113`), verified by 0-commit mode (`internal/run/run.go:654-662`); orchestrator aggregates `.lucind/result.json` envelopes across lanes (`openspec/specs/verify-dual-dispatch/spec.md:29-51`).

## Open Questions

- [ ] None
