# Design Lens B — Surface & Flow: sdd-fan-out-lens

## Assumed architecture

Candidate 1 (Null option): harden orchestrator prompt conventions, asset templates, and Go contract tests with no changes to Go execution binaries (`cmd/lucind-ai/*`), sidecar DAG parsers (`internal/dag/*`), or batch/integration dispatchers (`internal/run/*`). All five requirements in `openspec/changes/sdd-fan-out-lens/specs/sdd-planning-fan-out/spec.md:5-90` are realized by updating `plugin/claude-code/skills/lucind-ai/SKILL.md`, adding templates under `plugin/claude-code/skills/lucind-ai/assets/`, and adding tests to `internal/packet/packet_test.go`.

## Flow and Invariants

```
              Wave 1 (Parallel Lenses)
         ┌──→ [agy] Explore/Propose/Design Lens A ──┐
[Author] ┼──→ [agy] Explore/Propose/Design Lens B ──┼──→ [Integrate Barrier] ──→ [Wave 2 Synthesizer] ──→ [Canonical Artifact]
         └──→ [agy] Explore/Propose/Design Lens C ──┘                            (cursor-agent)
```

1. **Admission** (`plugin/claude-code/skills/lucind-ai/SKILL.md:22-31,157-161`; `internal/packet/packet.go:63-72,114-130`; `internal/run/run.go:250-265`):
   - **Invariant**: Frontmatter contains valid target fields (`legacy_main: true` or 4 feature keys) and non-empty prompt body.
   - **Breaks**: Admission fails closed with `status: failed` and empty worktree path.

2. **Disjointness Barrier** (`internal/packet/disjoint.go:29-48`; `cmd/lucind-ai/cli.go:243`):
   - **Invariant**: Wave-1 lens packets declare mutually disjoint paths in `allowed_paths`.
   - **Breaks**: Batch dispatch aborts before worktree creation with `ErrOverlappingAllowedPaths` (`internal/packet/disjoint.go:41`).

3. **Isolated Execution** (`internal/worktree/worktree.go:168-237`; `internal/run/batch.go:66-113`; `internal/run/run.go:590-626`):
   - **Invariant**: Each lens runs isolated, mutates only declared paths, commits with conventional commit under word ceiling.
   - **Breaks**: Out-of-scope diffs demote lane to `status: deviated` (`internal/run/run.go:621-623`); uncommitted edits fail completion mode (`internal/run/run.go:634-662`).

4. **Integration Barrier** (`internal/run/integrate.go:31-81`; `cmd/lucind-ai/cli.go:315-317`):
   - **Invariant**: All wave-1 lenses reach `done` and cleanly merge into target branch `HEAD`.
   - **Breaks**: Non-zero exit code; unmerged lanes listed under `reverted_ids:`.

5. **Synthesis Branching** (`plugin/claude-code/skills/lucind-ai/SKILL.md:171-176,184-186`):
   - **Invariant**: Wave-2 synthesizer worktree branches from integrated `HEAD` containing all 3 lens drafts.
   - **Breaks**: Synthesizer runs against stale tree missing draft files.

6. **Precedence and Compression** (`plugin/claude-code/skills/lucind-ai/SKILL.md:199-235`; `openspec/changes/sdd-fan-out-lens/specs/sdd-planning-fan-out/spec.md:21-36`):
   - **Invariant**: Phase skill (`~/.claude/skills/sdd-*/SKILL.md`) governs document schema; packet governs topology, ceilings, and criteria. Canonical artifact word count strictly < sum of lens draft ceilings.
   - **Breaks**: Synthesizer concatenates drafts or executes single-agent workflows violating packet bounds.

7. **Citation Verification** (`plugin/claude-code/skills/lucind-ai/SKILL.md:237-253`):
   - **Invariant**: Synthesizer verifies all `file:line` citations against real code and logs dropped citations in notes.
   - **Breaks**: Hallucinated or stale citations leak into canonical artifact.

8. **Contract Tests** (`internal/packet/packet_test.go:476-516`):
   - **Invariant**: Contract tests in `packet_test.go` validate `SKILL.md` tables, CLI commands/flags, and template assets.
   - **Breaks**: `go test ./internal/packet` fails in CI/build check (`lucind-checks.sh:2-10`).

## Surface Deltas

| Symbol or format | Today (file:line) | Delta | Backward compatible? |
|---|---|---|---|
| Frontmatter table (`feature`, `parent_ref`, `base_sha`, `expected_parent_sha`, `legacy_main`) | `plugin/claude-code/skills/lucind-ai/SKILL.md:22-31` | Document 5 target keys accepted by parser (`internal/packet/packet.go:63-72,114-130`) | Yes; documents existing parser support |
| Planning fan-out protocol | `plugin/claude-code/skills/lucind-ai/SKILL.md:126-176` | Promote fan-out from design pilot to convention across planning phases | Yes; orchestrator authoring convention |
| Asymmetric precedence rule | `plugin/claude-code/skills/lucind-ai/SKILL.md:218-235` | Generalize rule from `sdd-design` to all planning skills (`~/.claude/skills/sdd-*/`) | Yes; authoring precedence rule |
| Shipped CLI subcommands | `plugin/claude-code/skills/lucind-ai/SKILL.md:288-294` | Document `serve`, `feature`, `reconcile`, `renew` (`cmd/lucind-ai/cli.go:98-111,665,911`) | Yes; documents existing CLI |
| Shipped CLI `run` flags | `plugin/claude-code/skills/lucind-ai/SKILL.md:295-303` | Document `--approval-timeout`, `--legacy-main`, `--expected-parent-sha` (`cmd/lucind-ai/cli.go:133-137`) | Yes; documents existing flags |
| Feature branch ownership | `plugin/claude-code/skills/lucind-ai/SKILL.md:34-63` | Document human/orchestrator ownership of feature branches | Yes; operational documentation |
| Failure recovery guidance | `plugin/claude-code/skills/lucind-ai/SKILL.md:178-186` | Document re-dispatching failing wave-1 lanes individually before wave 2 | Yes; operational guidance |
| Explore templates | `plugin/claude-code/skills/lucind-ai/assets/` (missing) | Add 3 wave-1 lens templates + 1 wave-2 synthesis template | Yes; additive asset templates |
| Propose templates | `plugin/claude-code/skills/lucind-ai/assets/` (missing) | Add 3 wave-1 lens templates + 1 wave-2 synthesis template | Yes; additive asset templates |
| Design templates | `plugin/claude-code/skills/lucind-ai/assets/design-lens-a-packet-template.md:1-157` | Align 3 lens templates + 1 synthesis template with target frontmatter | Yes; updates existing templates |
| `TestSkillAssetContract` | `internal/packet/packet_test.go:476-516` | Assert `SKILL.md` frontmatter keys, CLI commands/flags, and protocol | Yes; test assertions |
| `TestPlanningPacketTemplates` | `internal/packet/packet_test.go:518-594` | Assert all planning templates parse under `packet.Parse` with disjoint paths | Yes; test assertions |

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `plugin/claude-code/skills/lucind-ai/SKILL.md` | Modify | Document 5 frontmatter keys, planning protocol, CLI commands/flags, branch ownership | Human orchestrator; `internal/packet/packet_test.go:TestSkillAssetContract` (`internal/packet/packet_test.go:476`) |
| `plugin/claude-code/skills/lucind-ai/assets/explore-lens-a-packet-template.md` | Create | Explore Lens A template (problem & candidates) | Orchestrator, `agy`, `internal/packet/packet_test.go:TestPlanningPacketTemplates` (`internal/packet/packet_test.go:518`) |
| `plugin/claude-code/skills/lucind-ai/assets/explore-lens-b-packet-template.md` | Create | Explore Lens B template (capabilities & scenarios) | Orchestrator, `agy`, `internal/packet/packet_test.go:TestPlanningPacketTemplates` (`internal/packet/packet_test.go:518`) |
| `plugin/claude-code/skills/lucind-ai/assets/explore-lens-c-packet-template.md` | Create | Explore Lens C template (risks, trade-offs & spikes) | Orchestrator, `agy`, `internal/packet/packet_test.go:TestPlanningPacketTemplates` (`internal/packet/packet_test.go:518`) |
| `plugin/claude-code/skills/lucind-ai/assets/explore-synthesis-packet-template.md` | Create | Explore synthesis template (canonical `explore.md` + notes) | Orchestrator, `cursor-agent`, `internal/packet/packet_test.go:TestPlanningPacketTemplates` (`internal/packet/packet_test.go:518`) |
| `plugin/claude-code/skills/lucind-ai/assets/propose-lens-a-packet-template.md` | Create | Propose Lens A template (candidate selection & approach) | Orchestrator, `agy`, `internal/packet/packet_test.go:TestPlanningPacketTemplates` (`internal/packet/packet_test.go:518`) |
| `plugin/claude-code/skills/lucind-ai/assets/propose-lens-b-packet-template.md` | Create | Propose Lens B template (capability impact & specs) | Orchestrator, `agy`, `internal/packet/packet_test.go:TestPlanningPacketTemplates` (`internal/packet/packet_test.go:518`) |
| `plugin/claude-code/skills/lucind-ai/assets/propose-lens-c-packet-template.md` | Create | Propose Lens C template (risks, rollback & test impact) | Orchestrator, `agy`, `internal/packet/packet_test.go:TestPlanningPacketTemplates` (`internal/packet/packet_test.go:518`) |
| `plugin/claude-code/skills/lucind-ai/assets/propose-synthesis-packet-template.md` | Create | Propose synthesis template (canonical `proposal.md` + notes) | Orchestrator, `cursor-agent`, `internal/packet/packet_test.go:TestPlanningPacketTemplates` (`internal/packet/packet_test.go:518`) |
| `plugin/claude-code/skills/lucind-ai/assets/design-lens-a-packet-template.md` | Modify | Align Design Lens A template with frontmatter target fields | Orchestrator, `agy`, `internal/packet/packet_test.go:TestPlanningPacketTemplates` (`internal/packet/packet_test.go:518`) |
| `plugin/claude-code/skills/lucind-ai/assets/design-lens-b-packet-template.md` | Modify | Align Design Lens B template with frontmatter target fields | Orchestrator, `agy`, `internal/packet/packet_test.go:TestPlanningPacketTemplates` (`internal/packet/packet_test.go:518`) |
| `plugin/claude-code/skills/lucind-ai/assets/design-lens-c-packet-template.md` | Modify | Align Design Lens C template with frontmatter target fields | Orchestrator, `agy`, `internal/packet/packet_test.go:TestPlanningPacketTemplates` (`internal/packet/packet_test.go:518`) |
| `plugin/claude-code/skills/lucind-ai/assets/design-synthesis-packet-template.md` | Modify | Align Design synthesis template with frontmatter target fields | Orchestrator, `cursor-agent`, `internal/packet/packet_test.go:TestPlanningPacketTemplates` (`internal/packet/packet_test.go:518`) |
| `internal/packet/packet_test.go` | Modify | Add contract tests for `SKILL.md` tables and `assets/` templates | `go test ./internal/packet` in `lucind-checks.sh:2-10` |

## Open Questions

- [ ] None
