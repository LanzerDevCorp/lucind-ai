# Design Lens A — Decisions: sdd-fan-out-lens

## Assumed architecture

This change implements Candidate 1 (null option: authoring contract and template hardening without Go runtime binary changes). We update `plugin/claude-code/skills/lucind-ai/SKILL.md` to establish multi-lens fan-out as the standard planning convention, document all five feature-target keys, document shipped CLI subcommands/flags, and define feature-branch ownership. We add phase-specific lens and synthesizer packet templates under `plugin/claude-code/skills/lucind-ai/assets/` for `explore` and `propose` (alongside existing `design` templates), and extend contract tests in `internal/packet/packet_test.go` to prevent parser and documentation drift.

## Technical Approach

We harden the authoring contract across documentation, template assets, and contract tests without modifying Go dispatch binaries (`cmd/lucind-ai/*` or `internal/run/*`). The orchestrator executes planning phases as two sequential `lucind-ai run --packet` invocations: Wave 1 dispatches three parallel `agy` lanes to mutually disjoint draft paths, and Wave 2 dispatches one `cursor-agent` synthesizer from the integrated tree (`specs/sdd-planning-fan-out/spec.md:5-20`). Packet templates encode asymmetric precedence (`~/.claude/skills/sdd-*/` governs document schema/content; the packet governs execution topology, slice ownership, word ceilings, and done criteria) and enforce compression ceilings where the canonical word budget stays strictly below the sum of lens budgets (`specs/sdd-planning-fan-out/spec.md:21-36`). Frontmatter reference tables and invocation blocks in `SKILL.md` document all parser-accepted feature-target keys and shipped CLI subcommands (`specs/sdd-planning-fan-out/spec.md:37-52`), backed by regression assertions in `internal/packet/packet_test.go` (`specs/sdd-planning-fan-out/spec.md:75-90`).

## Decision 1 — Phase-Specific Template Files for Planning Phases

**Choice**: Author dedicated phase-specific template files under `plugin/claude-code/skills/lucind-ai/assets/` for `explore`, `propose`, and `design` (`explore-lens-{a,b,c}-packet-template.md`, `explore-synthesis-packet-template.md`, `propose-lens-{a,b,c}-packet-template.md`, `propose-synthesis-packet-template.md`, alongside existing `design-lens-{a,b,c}-packet-template.md` and `design-synthesis-packet-template.md`).
**Alternatives considered**: A single generic parameterized template family (`planning-lens-packet-template.md` / `planning-synthesis-packet-template.md`) requiring per-phase prompt interpolation; generating speculative templates for unpiloted phases (`specs`, `tasks`) before their slice partitions stabilize.
**Rationale**: Each planning phase requires distinct slice ownership, exclusive reading lists, output skeletons, and budget compression targets (`plugin/claude-code/skills/lucind-ai/SKILL.md:143-148,199-207,218-227`). Generic templates force orchestrators to reconstruct prompt architecture manually, risking prompt drift. `explore`, `propose`, and `design` partitions are empirically proven across completed fan-out cycles (`openspec/changes/sdd-fan-out-lens/proposal.md:18-19`).
**Terminal consumer**: `packet.Parse` in `internal/packet/packet_test.go:476` (and `TestPlanningFanOutTemplateAssets`) parsing files in `plugin/claude-code/skills/lucind-ai/assets/`, satisfying `openspec/changes/sdd-fan-out-lens/specs/sdd-planning-fan-out/spec.md:53-74`.

## Decision 2 — Two-Tier Operator Remediation for Wave-1 Failures

**Choice**: Document a two-tier operator remediation protocol in `SKILL.md`: (1) On admission failure (`status: failed` with an empty worktree path), the operator inspects and repairs missing frontmatter fields (`feature`, `parent_ref`, `base_sha`, `expected_parent_sha`, or `legacy_main: true`) per `internal/packet/packet.go:63-72,114-130`; (2) On execution failure (`blocked`, `failed`, or `deviated` in wave 1), the operator remediates the blockage and re-dispatches only the failing lane(s) independently. The wave-2 synthesizer is dispatched only after all three lens drafts are confirmed integrated (`integrated_ids` contains all three lens IDs).
**Alternatives considered**: Dispatching wave 2 with partial feedstock (2 of 3 drafts) and letting the synthesizer extrapolate; re-running the entire 3-lane batch from scratch on single-lane failure; fallback to monolithic single-agent execution.
**Rationale**: The synthesis worktree branches from `HEAD` after wave-1 integration (`plugin/claude-code/skills/lucind-ai/SKILL.md:184-186`). Synthesizing partial drafts destroys the 3-slice partition and leaks coverage gaps. Re-dispatching only failing lanes leverages worktree isolation and ledger durability (`internal/run/batch.go:66-113`) without burning quota on completed lanes.
**Terminal consumer**: `plugin/claude-code/skills/lucind-ai/SKILL.md:178-186` and `internal/run/batch.go:66-113`, satisfying `openspec/changes/sdd-fan-out-lens/specs/sdd-planning-fan-out/spec.md:5-20`.

## Decision 3 — Contract Test Assertion Strategy in packet_test.go

**Choice**: Use targeted substring and table row extraction helpers for `SKILL.md` (extending `TestSkillAssetContract`) and direct `packet.Parse` execution with frontmatter field validation for template assets in `plugin/claude-code/skills/lucind-ai/assets/`.
**Alternatives considered**: Importing a third-party Markdown AST parser (e.g. `goldmark`) into `internal/packet`; whole-file exact string diffing; testing templates exclusively via shell scripts.
**Rationale**: `internal/packet` has zero non-standard-library dependencies (`internal/packet/packet.go:6-12`). Markdown AST libraries introduce unnecessary dependencies for table and string checks. Testing templates with `packet.Parse` directly exercises the production parser (`internal/packet/packet.go:78-165`) and guarantees template assets remain valid (`internal/packet/packet_test.go:476-594`).
**Terminal consumer**: `internal/packet/packet_test.go:476-516` (`TestSkillAssetContract`) and `internal/packet/packet.go:78-165` (`packet.Parse`), satisfying `openspec/changes/sdd-fan-out-lens/specs/sdd-planning-fan-out/spec.md:75-90`.

## Decision 4 — Sectioned Markdown Schema for Synthesis Notes

**Choice**: Retain pure sectioned Markdown with standard fixed spine headings (`## Unresolved Contradictions`, `## Coverage Gaps`, `## Dropped Citations`, `## Architecture Divergence` / `## Delta Verdict` / `## Scope Divergence`) without adding YAML frontmatter or JSON schemas.
**Alternatives considered**: Adding YAML frontmatter or JSON schemas to notes files; embedding synthesis notes into the canonical artifact; discarding synthesis notes after merge.
**Rationale**: Synthesis notes are consumed exclusively by the human orchestrator to audit arbitration decisions and verify dropped citations before merge approval (`plugin/claude-code/skills/lucind-ai/SKILL.md:237-241`). The Go CLI does not parse notes (`cmd/lucind-ai/cli.go:121-149`). Machine parsing adds schema overhead without operational benefit.
**Terminal consumer**: Human orchestrator review per `plugin/claude-code/skills/lucind-ai/SKILL.md:237-241`, satisfying `openspec/changes/sdd-fan-out-lens/specs/sdd-planning-fan-out/spec.md:5-20`.

## Decision 5 — Asymmetric Precedence and Editorial Compression Ceilings

**Choice**: Enforce asymmetric precedence and compression ceilings via prompt contracts in packet templates and `SKILL.md`. The phase skill (`~/.claude/skills/sdd-*/`) governs document schema and required sections; the packet governs execution topology, slice ownership, word ceilings, and done criteria. Canonical word ceilings stay strictly below the sum of lens draft ceilings (e.g. 1800 vs 3x1000 words), enforced during synthesis arbitration without Go runtime word-count checks.
**Alternatives considered**: Symmetric precedence (skill always wins or packet always wins); compiling word-count validation into Go (`internal/run/run.go:515-543`).
**Rationale**: Symmetric precedence breaks fan-out: phase skills describe monolithic single-agent execution and would collapse multi-lane partitioning (`plugin/claude-code/skills/lucind-ai/SKILL.md:218-235`). Enforcing word counts in Go would require binary re-compilation whenever prompt budget allocations adjust (`openspec/changes/sdd-fan-out-lens/proposal.md:28,82-85`).
**Terminal consumer**: `plugin/claude-code/skills/lucind-ai/SKILL.md:218-235` and template assets under `plugin/claude-code/skills/lucind-ai/assets/`, satisfying `openspec/changes/sdd-fan-out-lens/specs/sdd-planning-fan-out/spec.md:21-36`.

## Decision 6 — Orchestrator Ownership of Feature Branch Lifecycle

**Choice**: Feature branches (`refs/heads/feature/<id>`) are created and owned by the orchestrator/human via `lucind-ai feature create` (`cmd/lucind-ai/cli.go:686-700`) prior to wave dispatch. Wave-1 lens packets and wave-2 synthesis packets reference this feature target via frontmatter (`feature`, `parent_ref`, `base_sha`, `expected_parent_sha`) or run in legacy mode (`legacy_main: true`, `--expected-parent-sha`).
**Alternatives considered**: Dynamic feature branch creation by individual lens lanes at runtime; implicit default to `main` without frontmatter or CLI flags.
**Rationale**: Parallel lens lanes run concurrently in isolated worktrees (`internal/run/batch.go:66-113`). Concurrent parent ref manipulation by lanes causes git race conditions. Feature lifecycle management belongs to the orchestrator driving the session (`cmd/lucind-ai/cli.go:664-684`).
**Terminal consumer**: `cmd/lucind-ai/cli.go:686-700` (`runFeatureCreate`) and `internal/packet/packet.go:63-72,114-130`, satisfying `openspec/changes/sdd-fan-out-lens/specs/sdd-planning-fan-out/spec.md:37-52`.

## Open Questions

- [ ] None.
