# Proposal Lens A — Intent & Decision: sdd-fan-out-lens

## Assumed scope

Candidate 1 — Null option: convention and template hardening only (no Go binary changes).

## The deciding question, answered

Multi-lens fan-out is an orchestrator prompt and synthesis convention; it introduces no new deterministic machine invariants requiring Go binary enforcement.

The existing `lucind-ai` binary already enforces all necessary machine boundaries: repeatable `--packet` flag parsing and executor dispatch validation (`cmd/lucind-ai/cli.go:57-61,187-246`), static path disjointness checks (`internal/packet/disjoint.go:24-48`), isolated worktree execution (`internal/worktree/worktree.go:168-237`), parallel lane batching and barrier synchronization (`internal/run/batch.go:66-113`), automatic branch integration (`internal/run/integrate.go:31-81`), post-diff scope confinement (`internal/run/run.go:547-626`), and envelope schema validation (`internal/run/run.go:515-543`). The fan-out mechanics — 3-slice partitioning, word budget compression ratios (`plugin/claude-code/skills/lucind-ai/SKILL.md:199-207`), asymmetric skill precedence (`plugin/claude-code/skills/lucind-ai/SKILL.md:218-227`), and citation verification passes — are semantic editorial protocols suited for orchestrator prompts and templates, not rigid compiled Go types.

**Defense against alternatives:** Candidate 1 is chosen over Candidate 2 (additive sidecar DAG extension) and Candidate 3 (dedicated `lucind-ai fanout` CLI scaffolding). Candidate 2 forces planning phases into rigid sidecar YAML files and alters the apply-specific DAG schema (`internal/dag/parse.go:22-36`; `internal/dag/validate.go:30-32`) for workflows that are naturally 2-wave sequential dispatches (`plugin/claude-code/skills/lucind-ai/SKILL.md:153-176`). Candidate 3 bakes fluid prompt structures, word budget ratios, and agent topology into the compiled binary (`explore.md:62-67`), foreclosing rapid template iteration without adding safety. Crucially, empirical evidence from this change's own explore phase proved that the 3-lens fan-out runs cleanly on hand-authored packets on the current binary with zero reverts, clean integration, and rigorous citation verification (`explore.md:3`; `openspec/changes/sdd-fan-out-lens/explore-synthesis-notes.md:1-60`).

## Intent

Standardize and harden multi-lens SDD fan-out as a verified prompt-and-template convention across planning phases (`explore`, `propose`, `design`, `specs`, `tasks`), while closing critical frontmatter and subcommand documentation gaps in the skill definition.

Currently, `plugin/claude-code/skills/lucind-ai/SKILL.md:126-135` labels multi-lens fan-out as an unexercised pilot restricted to `design`. Furthermore, `SKILL.md:22-30` omits the five feature-target keys (`feature`, `parent_ref`, `base_sha`, `expected_parent_sha`, `legacy_main`) required by `internal/packet/packet.go:33-74,94-165`, and `SKILL.md:288-293` omits seven shipped subcommands, causing silent packet admission failures (`SKILL.md:178-182`). This change hardens the prompt contracts, adds missing phase templates, and documents required frontmatter fields without adding binary maintenance overhead.

## Scope

### In scope
- Promoting multi-lens fan-out in `plugin/claude-code/skills/lucind-ai/SKILL.md` from an unexercised `design` pilot to an established orchestrator convention for planning phases.
- Adding packet template assets under `plugin/claude-code/skills/lucind-ai/assets/` for multi-lens lanes and synthesizers across planning phases (`explore`, `propose`).
- Updating `plugin/claude-code/skills/lucind-ai/SKILL.md` frontmatter reference tables to document all feature-target keys (`feature`, `parent_ref`, `base_sha`, `expected_parent_sha`, `legacy_main`) and missing CLI subcommands (`serve`, `feature`, `reconcile`).
- Adding regression test coverage in `internal/packet/packet_test.go` asserting template validity and skill asset contract integrity.

### Out of scope
- Modifying Go runtime binaries, CLI flags, or subcommand routing (`cmd/lucind-ai/*`).
- Extending or modifying sidecar DAG parsing or validation in `internal/dag/*`.
- Modifying verify dual-dispatch or mechanical check flows (`openspec/specs/verify-dual-dispatch/spec.md`).

### Non-goals
- Building automated packet-generation CLI commands or DAG generators for multi-lens planning phases.
- Enforcing natural language word counts or prompt schema checks inside Go execution barriers.

## Approach

Deliver Candidate 1 by updating documentation, prompt contracts, packet templates, and asset tests:

1. **Skill Contract Hardening**: Update `plugin/claude-code/skills/lucind-ai/SKILL.md` to document the 4-lane fan-out convention (3 disjoint lenses + 1 sequential synthesizer) across applicable planning phases, document the mandatory feature-target frontmatter keys, and reflect complete CLI subcommands.
2. **Template Assets**: Provide reusable markdown packet templates under `plugin/claude-code/skills/lucind-ai/assets/` encoding slice ownership, required reading sets, word budget compression ratios, and synthesis citation verification rules.
3. **Asset Contract Testing**: Extend `internal/packet/packet_test.go` to validate new template assets against `internal/packet/packet.go` parsing rules and ensure frontmatter compliance.

## Success Criteria

- [ ] `plugin/claude-code/skills/lucind-ai/SKILL.md` documents multi-lens fan-out conventions, all five feature-target frontmatter keys (`internal/packet/packet.go:63-72`), and shipped CLI subcommands.
- [ ] Packet templates for multi-lens fan-out phases exist under `plugin/claude-code/skills/lucind-ai/assets/` and parse cleanly via `internal/packet.Parse`.
- [ ] Go test suite passes completely (`go test ./...`) with zero modifications to Go runtime dispatch logic in `cmd/` or `internal/run/`.
- [ ] Hand-authored multi-lens wave dispatches execute and integrate cleanly using standard `lucind-ai run --packet` invocations.

## Open Questions

- [ ] None. (Execution-topology precedence: As authorized by this packet, the three-lane proposal fan-out and skeleton take precedence over `~/.claude/skills/sdd-propose/SKILL.md:92-158` monolithic single-agent proposal layout.)
