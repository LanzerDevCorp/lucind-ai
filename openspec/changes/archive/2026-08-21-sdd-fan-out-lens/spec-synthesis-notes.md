# Synthesis Notes: sdd-fan-out-lens spec

## Delta Verdict

Delta, under new capability `sdd-planning-fan-out`. Lens A is authoritative.

A weighed both precedents. Against a delta: the proposal recorded `New Capabilities: None` and `Modified Capabilities: None` (`openspec/changes/sdd-fan-out-lens/proposal.md:32-36`), chose Candidate 1 (null option: convention and template hardening, no Go runtime change), and the dual-executor planning pattern (`plugin/claude-code/skills/lucind-ai/SKILL.md:64-69`) already ran on generic `lucind-ai run --packet` with no `openspec/specs/` entry. For a delta: orchestrator protocols backed by contract tests are capabilities here — `verify-dual-dispatch` and `sdd-apply` specify dispatch workflows without new integrator hooks — and this change adds tests in `internal/packet/packet_test.go` that pin `SKILL.md` and `assets/` (`proposal.md:21,48,72`). A requirement a test enforces is what a specification records. A coined `sdd-planning-fan-out` because no existing capability covers planning-phase fan-out, template assets, or admission documentation. The name does not collide: `openspec/specs/` has 14 capabilities, none named `sdd-planning-fan-out`.

On the evidence I agree with the direction and do not overrule A. The strongest delta case is the contract tests and the documentation they pin. The two-wave protocol itself is the same kind of unenforced orchestrator convention as dual-executor, which A cited as precedent for None; that tension stays inside A's argument rather than reversing it. B and C followed the proposal's "None" wording; that is Capability Divergence, not a licence to drop the spec.

## Unresolved Contradictions

None

## Coverage Gaps

Lens C's matrix listed no landing gaps for SC-1..SC-5 or AA-1..AA-3. Mapping onto A's requirements:

- SC-1 / AA-1 (`SKILL.md` convention, five keys, CLI, feature-branch ownership) → Frontmatter Admission and CLI Documentation, plus Two-Wave Planning Fan-Out Protocol
- SC-2 / AA-2 (templates parse) → Planning Fan-Out Template Assets
- SC-3 / AA-3 (contract tests fail on drift) → Skill and Asset Contract Tests
- SC-4 (no Go dispatch change) and SC-5 (hand-authored two-wave still runs) → Two-Wave requirement plus proposal out-of-scope; not new runtime behavior

Q1 (shared vs phase-specific templates) and Q4 (operator copy if wave-1 fails) stay design-deferred, as C classified and as `proposal.md:88,91` marked them. Q2 (exact `packet_test.go` strings vs Markdown AST) is a requirement in disguise per C; A's Skill and Asset Contract Tests names the keys, protocol, CLI, and template parse rules, leaving exact matching vs AST to design (`proposal.md:89`). Q3 (synthesis-notes frontmatter vs sectioned Markdown) is a requirement in disguise per C; A specified no notes schema; `proposal.md:90` explicitly deferred it — not an in-scope proposal commitment left uncovered.

Format drift versus `~/.claude/skills/sdd-spec/SKILL.md`: that skill's "For NEW Specs" template uses `# {Domain} Specification` / `## Purpose` / `## Requirements`. This packet required the skill's delta format, and Lens A wrote `## ADDED Requirements` for a new capability. The canonical file follows the packet and Lens A. Archive-time copy-full-then-edit does not apply (no MODIFIED block; Lens B found none).

## Dropped Citations

- Lens A: "none of the 16 existing capabilities in `openspec/specs/`". This worktree has 14 directories under `openspec/specs/`. The no-collision claim for `sdd-planning-fan-out` still holds.
- Lens A: `internal/packet/packet_test.go:476-516,518-594` as tests that already verify the five feature-target keys, planning fan-out protocol, CLI surface, and planning templates. Lines 476-516 are `TestSkillAssetContract`: explore dispatch via `lucind-ai run`, mandatory criterion 2 / read-only exception, `lucind-ai split --dag` on the apply row, and the verify dual-dispatch row. Lines 518-594 are `TestVerifyPacketTemplateAssetStructure` for `assets/verify-packet-template.md` only. Canonical recast: extend `TestSkillAssetContract` at line 476; do not cite 518-594 as planning-fan-out coverage.
- Lens A: `plugin/claude-code/skills/lucind-ai/SKILL.md:143-148` as the two-wave protocol for all planning phases. Those lines are the design-phase-only ownership table (`design-<id>-lens-a/b/c` and `design-<id>-synthesis`). Two-wave branching is `SKILL.md:153-176,184-186`; promoting it off the design-only pilot heading (`SKILL.md:126-134`) is the change.
- Lens A: `SKILL.md:218-228` as already applying to `~/.claude/skills/sdd-*/`. Lines 218-227 name `sdd-design` only. Canonical recast: extend that asymmetric rule to every planning-phase fan-out packet.
- Lens B: `assets/design-synthesis-packet-template.md:70-119` as the compression/arbitration mechanism. Those lines are the eight-item design spine and the notes-section skeleton. Compression is `SKILL.md:199-207`. The notes skeleton still supports recording unresolved issues.
- Lens C: `SKILL.md` lines 22-30 as the landing site of the five feature-target keys. Those lines are the general frontmatter table (`id`, `executor`, `routed_by`, `model`, `agent`, `read_only`, `allowed_paths`). The four keys plus `legacy_main: true` appear in narrative at `SKILL.md:157-161`. Documenting them *in the table* is the requirement.
- Lens C: `TestPacketTemplateAssetStructure`. No such name. Existing names are `TestPacketTemplateAssetContract` (generic `packet-template.md`) and `TestVerifyPacketTemplateAssetStructure` (verify template).
- Lens C: `cmd/lucind-ai/cli.go:102-103` as verify dual-dispatch. Those lines are `case "check": return runCheck`. Verify dual-dispatch remains a spec/convention, not a CLI subcommand.
- Lens C: `internal/run/run.go:515-543` as evidence that Go does not enforce word counts. That span is `decideStatus` envelope reads. Absence of a word-count field is `internal/packet/packet.go:33-74`; proposal out-of-scope is `proposal.md:28`.

## Capability Divergence

Lens A assumed `sdd-planning-fan-out` (delta). Lens B assumed `none — no delta (convention and template hardening only; no runtime binary or spec modifications)`. Lens C assumed `none — no delta`. B and C converged independently with each other and with the proposal's Capabilities section; they did not converge with A. A's verdict is the one assembled.

Lens B's modification table found all six named specs unchanged (`read-only-packet-schema`, `read-only-done-criterion`, `allowed-paths-enforcement`, `completion-mode-enforcement`, `apply-dag-dispatch`, `parent-feature-integration`). No `## MODIFIED` / `## REMOVED` / `## RENAMED` section was written. That matches A's "new capability, existing specs untouched."

## Scenario Attachment

Attached to A's requirements:

- Two-Wave Planning Fan-Out Protocol ← B "Wave-2 synthesizer dispatched before wave 1 integrates"
- Asymmetric Precedence and Compression Ceiling ← B "Over-budget lens draft constrained by synthesis compression"
- Frontmatter Admission and CLI Documentation ← B "Copied packet lacking feature-target fields fails admission"
- Planning Fan-Out Template Assets ← B "Malformed template fails packet parsing"; B "Overlapping lens templates fail upfront batch disjointness"
- Skill and Asset Contract Tests ← B "Documentation frontmatter table drifts from parser"

Orphans (B scenarios that attached to no ADDED requirement):

- "Out-of-scope file mutation during lens execution demotes status to Deviated" — restates unchanged `allowed-paths-enforcement` (`internal/run/run.go:590-626`; `openspec/specs/allowed-paths-enforcement/spec.md:95-118`). Lens B correctly found that spec does not move.
- "Read-only lens lane creating a commit fails completion mode" — restates unchanged `completion-mode-enforcement` (`internal/run/run.go:634-662`; `openspec/specs/completion-mode-enforcement/spec.md:47-65`). Lens B correctly found that spec does not move.

These are not a missed ADDED requirement; they are non-regression already specified elsewhere.
