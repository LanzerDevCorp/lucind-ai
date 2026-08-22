# Tasks Lens B — Partition & Dispatch Shape: Control Room Ledger

## Assumed decomposition

Assumed 3 work units derived from design file changes: Unit 1 delivers transactional schema v6 migration and the domain method split on `*Ledger` (`internal/ledger/`); Unit 2 wires CLI run lifecycle tracking and lane metadata recording (`cmd/lucind-ai/cli.go`, `internal/run/`); Unit 3 implements shell-free SQLite query DTOs on `serve.Model` (`internal/serve/`). The critical path runs through Unit 1 (schema and domain store methods), whose completion unblocks parallel downstream implementation of Unit 2 (write pipeline) and Unit 3 (read model).

## Suggested Work Units

| Unit | Goal | allowed_paths | Executor | Rollback boundary |
|---|---|---|---|---|
| 1 | Transactional schema v6 migration and domain method split on `*Ledger` | `internal/ledger/schema.go`<br>`internal/ledger/ledger.go`<br>`internal/ledger/runs.go` (new file)<br>`internal/ledger/lanes_meta.go` (new file)<br>`internal/ledger/progress.go` (new file)<br>`internal/ledger/events.go` (new file)<br>`internal/ledger/ledger_test.go` | `agy` | Reverting restores ledger schema v5 and removes new domain methods from `*Ledger`; existing v5 callers remain unaffected. |
| 2 | CLI run lifecycle dispatch wiring and lane metadata persistence | `cmd/lucind-ai/cli.go`<br>`cmd/lucind-ai/cli_test.go`<br>`internal/run/run.go`<br>`internal/run/batch.go`<br>`internal/run/run_test.go` | `cursor-agent` | Reverting restores CLI and run orchestration to untracked batch execution without run lifecycle or lane metadata persistence. |
| 3 | Shell-free Control Room read model DTO queries on `serve.Model` | `internal/serve/model.go`<br>`internal/serve/handlers.go`<br>`internal/serve/model_test.go` | `cursor-agent` | Reverting restores `serve.Model` and `/api/state` to approvals-only without Control Room run or progress queries. |

## Wave Plan

| Wave | Units | Runs in parallel | Green on its own |
|---|---|---|---|
| 1 | Unit 1 | No (1 unit) | Yes. Transactional schema v6 migration and domain methods on `*Ledger` compile cleanly and pass all unit tests (`go test ./internal/ledger`); additive schema and method additions leave existing callers fully functional. |
| 2 | Unit 2, Unit 3 | Yes (2 units) | Yes. Both units consume the completed `*Ledger` API from Wave 1; their path sets are disjoint (`cmd/lucind-ai/` + `internal/run/` vs `internal/serve/`), and each compiles and passes `lucind-checks.sh` independently and combined. |

## Disjointness Check

- **Wave 1 (Unit 1)**: Single unit — no intra-wave disjointness check required.
- **Wave 2 (Unit 2 & Unit 3)**:
  - Unit 2 paths: `cmd/lucind-ai/cli.go`, `cmd/lucind-ai/cli_test.go`, `internal/run/run.go`, `internal/run/batch.go`, `internal/run/run_test.go`
  - Unit 3 paths: `internal/serve/model.go`, `internal/serve/handlers.go`, `internal/serve/model_test.go`
  - Under `packet.PathInScope` (`internal/packet/disjoint.go`), neither path set shares any file or component directory prefix (`cmd/lucind-ai/` vs `internal/serve/`, `internal/run/` vs `internal/serve/`). Verdict: Disjoint (no overlap).

## Sidecar Recommendation

**Recommendation**: single packet, no sidecar
**Rationale**: While Units 2 and 3 are pairwise disjoint and could execute concurrently in Wave 2 following Unit 1, their combined diff (~360 lines across CLI wiring and Serve DTOs) is too small to justify the orchestration overhead of authoring `apply-dag.yaml`, running `lucind-ai split`, and managing multi-wave Integrate gates. As established in `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md`, changes under 1,500 lines dominated by a foundation unit (Unit 1 represents ~65% of the total diff) are more safely and quickly executed sequentially within a single packet using work-unit commits.

## Open Questions

- [ ] Should `lane_progress` pruning trigger via a background ticker in `lucind-ai serve` (`cmd/lucind-ai/cli.go:674-725`) or via an on-demand CLI command?
- [ ] Should `lane_progress.message` be constrained to structured JSON (stdout/stderr/control) or remain unstructured text?
- [ ] Packet precedence notice: `~/.claude/skills/sdd-tasks/SKILL.md` describes single-author end-to-end `tasks.md` generation, which is explicitly superseded by this 3-lens parallel synthesis workflow.
