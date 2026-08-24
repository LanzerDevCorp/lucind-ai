---
name: lucind-ai
description: "Author dispatch packets and drive the lucind-ai delegated execution binary."
license: Apache-2.0
metadata:
  author: "LanzerDevCorp"
  version: "2.0"
---

# lucind-ai

## Activation Contract

Use this skill to choose and execute a human-approved Execution Strategy for one Change, author dispatch packets, or operate the `lucind-ai` binary. `SKILL.md` is the only skill entry point; files under `references/` are disclosed modules, not independently invoked skills.

## Hard Rules

- Confirm the Mode and Execution Strategy before execution. A later strategy change also requires human confirmation.
- Keep one Orchestrator authoritative for the Change. Agents own Lanes, not scope, priorities, Dependencies, Acceptance, or Promotion.
- Run `lucind-ai -v` before dispatch. Run CLI usage from the environment rather than caching command syntax here.
- Run from the primary repository root. Linked-worktree dispatch is refused.
- Put packets under `.lucind/packets/`; tracked packet files dirty the primary checkout and can block integration.
- Treat `.lucind/` as ignored runtime state and `openspec/changes/<id>/` as tracked Change history.
- Never infer unbuilt capability from the domain vocabulary. Today, feature attempts, Ownership Leases, compare-and-swap Promotion, overlap gating, recovery, and reconciliation are production-wired; issue #4 defect automation and Claude Agent Teams launching are not.
- Feature-targeted (Isolated Mode) candidate construction resolves each candidate from its own declared `base_sha`/`parent_ref`, ancestry-checked against `parent_ref`; it does not depend on primary-checkout `HEAD` (`integration-target-isolation`, merged `77f6f00`). The `legacy_main`/no-target path still branches from primary `HEAD` by design — that is Exclusive Mode's contract, not a defect.
- `lucind-ai split` emits apply waves but does not schedule them. Unordered overlapping `allowed_paths` are rejected.
- Integration bisection is post-execution recovery, not a scheduler or pre-dispatch safety mechanism.

## Decision Gates

Read only the modules whose trigger branch fires. When multiple branches fire, read their union.

| Runtime situation | Read |
|---|---|
| Canonical term interpretation or packaged glossary use | `references/core/domain.md` |
| Any repository write, dispatch boundary, or safety decision | `references/core/safety.md` |
| Small Change completed by its Orchestrator without delegation | `references/strategies/direct.md` |
| Isolated Mode or feature-targeted dispatch | `references/modes/isolated.md` |
| Exclusive Mode or runtime `legacy_main` path | `references/modes/exclusive.md` |
| SDD lifecycle, apply DAG, verify, or archive | `references/strategies/sdd.md` |
| Multi-lens planning or multi-Agent delegation | `references/strategies/fan-out.md` |
| Dependency, defect, blocker, or issue #4 question | `references/coordination/dependencies-defects.md` |
| Failed attempt, overlap, reconciliation, lease, retry, or multi-wave recovery | `references/coordination/recovery-reconciliation.md` |
| Packet authoring, result envelope, or returned evidence | `references/contracts/packets-results.md` |
| Lane Acceptance, batch outcome, checks, or Change Promotion | `references/contracts/acceptance-promotion.md` |
| Executor, model, provider, agent, quota, or route selection | `references/adapters/executors.md` |
| Claude Agent Teams or provider collaboration question | `references/adapters/claude-agent-teams.md` |
| `serve`, approvals, ledger, status, or operator monitoring | `references/operations/control-room.md` |
| Dirty tree, stale worktree, reverted Lane, timeout, flaky check, or environment failure | `references/operations/troubleshooting.md` |

## Execution Steps

1. Identify the Change, Integration Target, Write Scope, Mode, and approved Execution Strategy. The entry state is complete when none is inferred from the checked-out branch.
2. Load the exact branch modules from the table. Loading is complete when every active branch has one authoritative module and unrelated branches remain unloaded.
3. Execute that strategy and preserve packet, result, check, and decision evidence required by the loaded contracts.
4. Judge evidence independently. Green criteria are not proof; verify cited files, terminal consumers, checks, and scope before Acceptance or Promotion.

## Output Contract

Return the Change status, Mode, Execution Strategy, Lanes and outcomes, evidence checked, accepted or reverted IDs, current blockers, preserved worktrees, and whether human-confirmed Promotion occurred. State every current limitation that affected the run and never report a planned capability as available.

## References

The Decision Gates table is the authoritative pointer index. Packet assets remain under `assets/`; load a template only when its named strategy phase requires it.
