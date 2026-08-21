# Spec Lens A — Requirements: sdd-fan-out-lens

## Assumed capability

sdd-planning-fan-out

## Delta verdict

Delta. We specify under a new capability name, `sdd-planning-fan-out`.

Weighing the precedent both ways:
1. **Against a delta (precedent for "None")**: The proposal recorded `New Capabilities: None` and `Modified Capabilities: None` (`openspec/changes/sdd-fan-out-lens/proposal.md:32-36`), choosing Candidate 1 (the null option) because multi-lens fan-out is an orchestrator convention requiring no Go runtime changes (`proposal.md:3-9`). Furthermore, the dual-executor planning pattern (`plugin/claude-code/skills/lucind-ai/SKILL.md:64-95`) ran across multiple changes on generic `lucind-ai run --packet` without an `openspec/specs/` entry, and the six existing specs (`openspec/specs/read-only-packet-schema/`, `read-only-done-criterion/`, `allowed-paths-enforcement/`, `completion-mode-enforcement/`, `apply-dag-dispatch/`, `parent-feature-integration/`) remain untouched.
2. **For a delta (precedent for "Delta")**: Orchestrator protocols backed by automated contract tests constitute formal capabilities in this repository. `verify-dual-dispatch` (`openspec/specs/verify-dual-dispatch/spec.md:1-179`) and `sdd-apply` (`openspec/specs/sdd-apply/spec.md:1-87`) specify orchestrator dispatch workflows, packet shapes, and stdout parsing without Go integrator changes. Crucially, `sdd-fan-out-lens` introduces contract tests in `internal/packet/packet_test.go` (`proposal.md:21,48,72`) that assert against `SKILL.md` content and `assets/` templates. A requirement enforced by tests is what a specification exists to record; leaving these contracts un-specified would create untraceable test assertions.

We coin `sdd-planning-fan-out` because none of the 16 existing capabilities in `openspec/specs/` covers planning phase fan-out, template assets, or admission documentation.

## ADDED Requirements

### Requirement: Two-Wave Planning Fan-Out Protocol

The orchestrator MUST execute planning phases (explore, propose, design, specs, tasks) as a two-wave fan-out protocol using generic `lucind-ai run --packet` (`cmd/lucind-ai/cli.go:121-149`). Wave 1 MUST dispatch three parallel `agy` lens lanes writing to mutually disjoint draft paths. Wave 2 MUST dispatch a single sequential `cursor-agent` synthesis lane branched from the integrated tree to produce the canonical artifact and synthesis notes (`plugin/claude-code/skills/lucind-ai/SKILL.md:143-148,184-186`).

#### Scenario: Planning phase dual-wave dispatch
- **GIVEN** an SDD change in a planning phase
- **WHEN** wave 1 completes and integrates all three lens drafts
- **THEN** the orchestrator MUST dispatch wave 2 with the synthesizer packet to produce the canonical artifact and synthesis notes

### Requirement: Asymmetric Precedence and Compression Ceiling

In all planning fan-out packets, precedence between skill and packet MUST be asymmetric: the phase skill (`~/.claude/skills/sdd-*/`) SHALL govern document content and schema, while the packet SHALL govern execution topology, slice ownership, word ceilings, and done criteria (`plugin/claude-code/skills/lucind-ai/SKILL.md:218-228`). The canonical artifact word budget MUST be strictly less than the sum of the three lens draft budgets (`plugin/claude-code/skills/lucind-ai/SKILL.md:199-207`).

#### Scenario: Synthesizer enforces compression ceiling
- **GIVEN** three integrated lens drafts with a combined word ceiling of N words
- **WHEN** the synthesizer lane generates the canonical phase artifact
- **THEN** the canonical artifact word count MUST remain strictly below N

### Requirement: Frontmatter Admission and CLI Documentation

`plugin/claude-code/skills/lucind-ai/SKILL.md` MUST document all five feature-target frontmatter keys (`feature`, `parent_ref`, `base_sha`, `expected_parent_sha`, `legacy_main`) accepted by `packet.Parse` (`internal/packet/packet.go:114-130`), the shipped CLI surface including `serve`, `feature`, `reconcile`, `renew`, `check`, `split`, and `run` flags (`cmd/lucind-ai/cli.go:104-109,135-137,664-665,910-911`), and feature branch creation and ownership rules (`proposal.md:20,70`).

#### Scenario: SKILL.md frontmatter table contains feature target keys
- **GIVEN** `plugin/claude-code/skills/lucind-ai/SKILL.md`
- **WHEN** the frontmatter reference table is inspected
- **THEN** it MUST document `feature`, `parent_ref`, `base_sha`, `expected_parent_sha`, and `legacy_main` so that copied packet frontmatter passes admission

### Requirement: Planning Fan-Out Template Assets

Planning phase packet templates under `plugin/claude-code/skills/lucind-ai/assets/` MUST exist for explore, propose, and design phases, MUST parse validly under `packet.Parse` (`internal/packet/packet.go:78-143`), MUST specify `legacy_main: true` or feature-target fields, and MUST assign mutually disjoint draft file paths for parallel lens lanes (`internal/packet/disjoint.go:24-48`).

#### Scenario: Fan-out templates parse as valid packets
- **GIVEN** the fan-out packet templates in `plugin/claude-code/skills/lucind-ai/assets/`
- **WHEN** `packet.Parse` parses each template file
- **THEN** parsing MUST succeed without error, `Packet.LegacyMain` MUST be true, and declared draft paths MUST be pairwise disjoint

### Requirement: Skill and Asset Contract Tests

`internal/packet/packet_test.go` MUST provide automated contract tests that verify `SKILL.md` documents the five feature-target keys, the planning fan-out protocol, and CLI subcommands/flags, and that all planning templates under `assets/` parse validly and adhere to frontmatter and done-criteria structures (`internal/packet/packet_test.go:476-516,518-594`).

#### Scenario: Contract test validates skill and template drift
- **GIVEN** the test suite in `internal/packet/packet_test.go`
- **WHEN** `go test ./internal/packet/...` is executed
- **THEN** the contract tests MUST assert that `SKILL.md` and `assets/` templates conform to parser and protocol requirements

## Traces

| Requirement | Proposal success criterion or affected area |
|---|---|
| Requirement: Two-Wave Planning Fan-Out Protocol | Success criterion: `proposal.md:70`; Affected area: `SKILL.md` (`proposal.md:46`) |
| Requirement: Asymmetric Precedence and Compression Ceiling | Success criterion: `proposal.md:74`; Affected area: `SKILL.md` (`proposal.md:46`) |
| Requirement: Frontmatter Admission and CLI Documentation | Success criterion: `proposal.md:70`; Affected area: `SKILL.md` (`proposal.md:46`) |
| Requirement: Planning Fan-Out Template Assets | Success criterion: `proposal.md:71`; Affected area: `assets/` (`proposal.md:47`) |
| Requirement: Skill and Asset Contract Tests | Success criterion: `proposal.md:72`; Affected area: `internal/packet/packet_test.go` (`proposal.md:48`) |

## Open Questions

- [ ] For Lens B: How should admission failures (e.g., omitted feature-target fields resulting in silent empty-worktree failure) be handled or reported if an operator attempts a wave-1 fan-out?
- [ ] For Lens B: How should the orchestrator recover or re-dispatch if one of the three wave-1 lens lanes fails, blocks, or deviates during execution?
- [ ] For Lens B: How should the synthesizer behave when all three wave-1 lens drafts diverge completely on assumed architecture or scope?
