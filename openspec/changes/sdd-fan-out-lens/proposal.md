# Proposal: sdd-fan-out-lens

**Chosen candidate: Candidate 1 — Null option** (convention and template hardening only; no Go binary or spec changes).

## Intent

The exploration's deciding question (`openspec/changes/sdd-fan-out-lens/explore.md:75`) is answered: multi-lens fan-out is an orchestrator prompt and synthesis convention. It adds no deterministic machine invariants that the Go binary must grow to enforce.

Generic `lucind-ai run --packet` already runs the topology (`explore.md:3`): isolated worktrees, parallel lanes, barrier join, integrate, declared-path disjointness, post-diff scope, and envelope validation (`cmd/lucind-ai/cli.go:121-149,187-197,285-297`; `internal/run/batch.go:66-113`; `internal/run/integrate.go:31-81`; `internal/packet/disjoint.go:24-48`; `internal/run/run.go:451-454,515-543,590-626`; `internal/worktree/worktree.go:168-237`). Slice ownership, two-wave sequencing, budget compression, skill/packet precedence, and citation verification stay editorial (`plugin/claude-code/skills/lucind-ai/SKILL.md:143-176,188-227`).

`SKILL.md` still labels the topology a design-only unexercised pilot (`SKILL.md:126-135`). Its frontmatter table omits the five feature-target keys the parser accepts (`SKILL.md:22-30`; `internal/packet/packet.go:63-72,114-130`). Admission requires those four keys or `legacy_main: true` and fails closed with `status: failed`, an empty worktree path, and no reason on stdout or stderr (`SKILL.md:157-161,178-182`). The invocation block documents only `run`, `split`, `check`, and `--version` (`SKILL.md:288-293`) and only `--packet` / `--timeout` (`SKILL.md:295-298`). The binary also ships `serve`, `feature create|status|recover`, `reconcile approve|decline|cancel|renew`, and `--approval-timeout`, `--legacy-main`, `--expected-parent-sha` (`cmd/lucind-ai/cli.go:104-109,135-137,664-665,910-911`). Nothing says who creates or owns a feature branch. A packet copied from the table fails admission silently.

Candidate 2 would stretch the apply-only sidecar — `dag.Node` has no `read_only` (`internal/dag/parse.go:22-36`); empty `allowed_paths` is rejected at split (`internal/dag/validate.go:30-32`) — across a two-invocation schedule already expressible as hand-authored packets (`SKILL.md:153-176`). Candidate 3 would compile topology and budget ratios into the binary (`explore.md:62-67`) without adding a safety that dispatch lacks.

## Scope

### In Scope
- Promote multi-lens fan-out in `plugin/claude-code/skills/lucind-ai/SKILL.md` from a design-only pilot to the orchestrator convention for planning phases (explore, propose, design, specs, tasks).
- Add lens and synthesizer packet templates under `plugin/claude-code/skills/lucind-ai/assets/` for phases that lack them (at least explore and propose; design templates already exist).
- Document the five feature-target keys, shipped subcommands and `run` flags, and who creates or owns a feature branch.
- Add contract tests in `internal/packet/packet_test.go` so `SKILL.md` and `assets/` cannot rot against the parser.

### Out of Scope
- Go runtime, CLI flags, or subcommand routing (`cmd/lucind-ai/*`).
- Sidecar DAG parse, validate, or emit (`internal/dag/*`).
- Verify dual-dispatch or mechanical-check flows.
- A dedicated `lucind-ai fanout` (or equivalent) generator.
- Enforcing word counts or prompt schema inside Go.

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- None. Existing specs stay honored unchanged: `read-only-packet-schema`, `read-only-done-criterion`, `allowed-paths-enforcement`, `completion-mode-enforcement`, `apply-dag-dispatch`, `parent-feature-integration`.

## Approach

Harden the authoring contract, not the binary. Update `SKILL.md` so a copied packet admits. Ship per-lane templates with disjoint output paths, required reading, word ceilings, and a synthesis citation pass. Extend the existing asset-contract tests. Dispatch remains two `lucind-ai run --packet` waves on today's batch-and-integrate path. This change does not hook a new lane-lifecycle call site.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `plugin/claude-code/skills/lucind-ai/SKILL.md` | Modified | Fan-out convention, feature-target keys, CLI surface, feature-branch ownership, wave protocol. |
| `plugin/claude-code/skills/lucind-ai/assets/` | Modified | Lens and synthesizer templates with required frontmatter and disjoint draft paths. |
| `internal/packet/packet_test.go` | Modified | Contract tests for skill text and templates versus the parser. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Silent admission from a stale frontmatter table | High | Document the keys in `SKILL.md` and every template; contract-test them. |
| Overlapping `allowed_paths` in wave 1 | Med | Templates name disjoint draft files; `SKILL.md` restates the existing batch disjointness check. |
| Lens drafts over budget, starving synthesis | Med | Packet headers state word ceilings; the synthesis template keeps the compression gap. |
| Wave 2 starts before wave 1 integrates | Med | `SKILL.md` requires confirming the wave-1 integrated commit before the synthesizer run. |

## Rollback Plan

`git revert` of the commits that touch `SKILL.md`, `assets/`, and `internal/packet/packet_test.go`. No binary rebuild, schema version, ledger migration, or spec delta. Restoring prior revisions does not affect in-flight packets.

## Dependencies

- Existing `lucind-ai run --packet` dispatch, batch barrier, integrate, worktree isolation, path enforcement, envelope validation, and feature-target admission.
- Design-phase templates already under `assets/`, as the pattern to copy.

## Success Criteria

- [ ] `SKILL.md` documents the multi-lens convention, the five feature-target keys, shipped subcommands and `run` flags, and feature-branch ownership.
- [ ] Fan-out packet templates exist under `assets/` and parse as packets.
- [ ] Asset and skill contract tests fail if those tables or templates drift.
- [ ] No changes to Go dispatch logic under `cmd/` or `internal/run/`.
- [ ] A hand-authored two-wave fan-out still runs on generic `lucind-ai run --packet`.

## Review burden

About 120–250 lines across `SKILL.md`, templates, and `packet_test.go`. Documentation and test assertions only.

## Rejected alternatives

**Candidate 2 — sidecar DAG extension.** Planning-phase fan-out needs disjoint drafts and a two-wave barrier, not apply-style topological split. Closing `read_only` / empty-`allowed_paths` / apply-only location would churn `internal/dag` and `apply-dag-dispatch` without removing hand-authored packets. Right only if fan-out grows interdependent waves that prose cannot schedule.

**Candidate 3 — `lucind-ai fanout` scaffolding.** Compiling topology, templates, and budget ratios into the binary freezes prompt iteration behind a release. Right only if the four-lane shape is frozen and hand-authored packets error often.

## Open questions left to design (not decided here)

- [ ] One shared template family for every fan-out phase, or phase-specific templates beyond design / explore / propose?
- [ ] Exact assertions in `packet_test.go` (which `SKILL.md` strings, which template keys).
- [ ] Whether synthesis-notes stay sectioned Markdown or gain machine-parseable frontmatter.
- [ ] What `SKILL.md` tells the operator if a wave-1 lens fails admission or execution.
