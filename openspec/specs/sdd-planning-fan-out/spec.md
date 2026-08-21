# SDD Planning Fan-Out Specification

## Purpose

Define the two-wave, three-lens planning fan-out: how planning phases split across parallel lens lanes, how a synthesizer arbitrates and compresses their drafts, what the packet templates under `plugin/claude-code/skills/lucind-ai/assets/` must guarantee, and what the skill must document so a copied packet is admissible.

## Requirements

### Requirement: Two-Wave Planning Fan-Out Protocol

The orchestrator MUST execute planning phases (explore, propose, design, specs, tasks) as a two-wave fan-out on generic `lucind-ai run --packet` (`cmd/lucind-ai/cli.go:121-149`; `openspec/changes/sdd-fan-out-lens/proposal.md:18,74`; `openspec/changes/sdd-fan-out-lens/explore.md:9`). Wave 1 MUST dispatch three parallel `agy` lens lanes to mutually disjoint draft paths. Wave 2 MUST dispatch one sequential `cursor-agent` synthesis lane branched from the integrated tree, producing the canonical artifact and synthesis notes (`plugin/claude-code/skills/lucind-ai/SKILL.md:153-176,184-186`). Sidecar `apply-dag.yaml` / `lucind-ai split` MUST NOT be required (`explore.md:38-40`).

#### Scenario: Planning phase dual-wave dispatch

- GIVEN an SDD change in a planning phase
- WHEN wave 1 completes and integrates all three lens drafts
- THEN the orchestrator MUST dispatch wave 2 to produce the canonical artifact and synthesis notes

#### Scenario: Wave-2 synthesizer dispatched before wave 1 integrates

- GIVEN wave-1 lanes finished in isolated worktrees but have not integrated into primary `HEAD` (`internal/run/integrate.go:31-81`; `explore.md:42`)
- WHEN wave 2 dispatches the synthesizer from that unintegrated `HEAD`
- THEN the synthesizer worktree MUST NOT contain the wave-1 draft files (`SKILL.md:184-186`)

### Requirement: Asymmetric Precedence and Compression Ceiling

In planning fan-out packets, precedence MUST be asymmetric: the phase skill (`~/.claude/skills/sdd-*/`) SHALL govern document content and schema; the packet SHALL govern execution topology, slice ownership, word ceilings, and done criteria. Current `SKILL.md` states this for `sdd-design` (`plugin/claude-code/skills/lucind-ai/SKILL.md:218-227`); the same rule MUST apply to every planning-phase fan-out packet. The canonical artifact word budget MUST stay strictly below the sum of the three lens-draft budgets (`SKILL.md:199-207`; `proposal.md:74`). Go MUST NOT parse or enforce those budgets (`proposal.md:28`).

#### Scenario: Synthesizer enforces compression ceiling

- GIVEN three integrated lens drafts with combined word ceiling N
- WHEN the synthesizer generates the canonical phase artifact
- THEN the canonical artifact word count MUST remain strictly below N

#### Scenario: Over-budget lens draft constrained by synthesis compression

- GIVEN a wave-1 lens draft that exceeds its declared word ceiling
- WHEN the wave-2 synthesizer processes the drafts against the canonical ceiling
- THEN the synthesizer MUST compress below that ceiling rather than concatenate, and MUST record unresolved issues in synthesis notes

### Requirement: Frontmatter Admission and CLI Documentation

`plugin/claude-code/skills/lucind-ai/SKILL.md` MUST document all five feature-target keys (`feature`, `parent_ref`, `base_sha`, `expected_parent_sha`, `legacy_main`) accepted by `packet.Parse` (`internal/packet/packet.go:63-72,114-130`), the shipped CLI surface (`serve`, `feature`, `reconcile`, `renew`, `check`, `split`, and `run` flags `--timeout`, `--approval-timeout`, `--legacy-main`, `--expected-parent-sha`; `cmd/lucind-ai/cli.go:100-111,135-137,664-665,910-911`), and who creates or owns a feature branch (`proposal.md:20,70`). The current frontmatter table (`SKILL.md:22-30`) and invocation block (`SKILL.md:288-298`) omit these. Copied packets that omit the four keys and `legacy_main: true` already fail admission closed (`SKILL.md:157-161,178-182`; `openspec/specs/parent-feature-integration/spec.md:7-18`).

#### Scenario: SKILL.md frontmatter table contains feature target keys

- GIVEN `plugin/claude-code/skills/lucind-ai/SKILL.md`
- WHEN the frontmatter reference table is inspected
- THEN it MUST document `feature`, `parent_ref`, `base_sha`, `expected_parent_sha`, and `legacy_main`

#### Scenario: Copied packet lacking feature-target fields fails admission

- GIVEN a packet copied from documentation omits all four feature-target fields and omits `legacy_main: true`
- WHEN `lucind-ai run` evaluates admission (`internal/packet/packet.go:63-72`)
- THEN admission MUST fail closed with `status: failed` and an empty worktree path

### Requirement: Planning Fan-Out Template Assets

Planning-phase packet templates under `plugin/claude-code/skills/lucind-ai/assets/` MUST exist for explore, propose, and design (`proposal.md:19,71`). Each MUST parse under `packet.Parse` (`internal/packet/packet.go:77-165`) and MUST assign mutually disjoint draft paths for parallel lens lanes (`internal/packet/disjoint.go:24-48`). Design templates already exist; explore and propose templates are in scope.

A template MUST NOT declare a dispatch target it cannot know. `legacy_main` and the four feature-target fields describe where one dispatch lands, not what a reusable template is; baking `legacy_main: true` into a template silently targets `main` even when the change runs against a named feature parent. A template therefore satisfies admission by any one of three paths: it declares `legacy_main: true`, it declares the four feature-target fields, or it declares neither and the orchestrator supplies the target at dispatch with **both** `--legacy-main` and `--expected-parent-sha` (`plugin/claude-code/skills/lucind-ai/SKILL.md:150`; `docs/feature-parent-integration.md:188-202`). Both are required, not either: `validatePacketAdmission` rejects `LegacyMain` without an expected SHA (`internal/run/run.go:251-253`), and an expected SHA without legacy mode falls through to the four-field branch and fails there (`internal/run/run.go:261`). The third path is the default for a template, because a template is copied across changes whose targets differ.

#### Scenario: Fan-out templates parse as valid packets

- GIVEN the fan-out packet templates in `plugin/claude-code/skills/lucind-ai/assets/`
- WHEN `packet.Parse` parses each template
- THEN parsing MUST succeed and declared draft paths MUST be pairwise disjoint

#### Scenario: A template declaring no dispatch target is admitted from the command line

- GIVEN a packet copied from a template that declares neither `legacy_main` nor the four feature-target fields
- WHEN the orchestrator dispatches it with `lucind-ai run --legacy-main --expected-parent-sha <sha>`
- THEN admission MUST succeed, because `--legacy-main` sets `LegacyMain` on the batch and `--expected-parent-sha` fills the field the packet left empty (`docs/feature-parent-integration.md:200-202`)

#### Scenario: The same template is admitted against a named feature parent

- GIVEN the same target-less template, used by a change that runs against a named feature parent rather than `main`
- WHEN the copied packet declares `feature`, `parent_ref`, `base_sha`, and `expected_parent_sha`
- THEN admission MUST succeed with no edit to the template itself, which is the property a baked-in `legacy_main: true` would destroy

#### Scenario: Malformed template fails packet parsing

- GIVEN a template missing `id`, `executor`, or `routed_by`, having an empty body, or containing a non-JSON `allowed_paths` value
- WHEN `packet.Parse` or template contract tests validate it
- THEN parsing MUST fail with `ErrMissingID`, `ErrMissingExecutor`, `ErrMissingRoutedBy`, `ErrEmptyBody`, or `ErrInvalidAllowedPaths`

#### Scenario: Overlapping lens templates fail upfront batch disjointness

- GIVEN two wave-1 lens packets declare overlapping `allowed_paths`
- WHEN `lucind-ai run` evaluates `packet.DisjointAllowedPaths` (`internal/packet/disjoint.go:24-48`; `cmd/lucind-ai/cli.go:243`)
- THEN dispatch MUST reject the batch before `worktree.Create` (`openspec/specs/allowed-paths-enforcement/spec.md:42-60`)

### Requirement: Skill and Asset Contract Tests

`internal/packet/packet_test.go` MUST extend the existing asset-contract suite so tests fail if `SKILL.md` omits the five feature-target keys, the planning fan-out protocol, or shipped CLI subcommands/flags, or if planning templates under `assets/` fail to parse or violate frontmatter and disjoint-path rules (`proposal.md:21,48,72`). `TestSkillAssetContract` (`internal/packet/packet_test.go:476`) today asserts explore dispatch, read-only criterion 2, apply split, and the verify row — not those keys.

#### Scenario: Contract test validates skill and template drift

- GIVEN the test suite in `internal/packet/packet_test.go`
- WHEN `go test ./internal/packet` runs
- THEN the contract tests MUST fail if `SKILL.md` or `assets/` templates drift from parser and protocol requirements

#### Scenario: Documentation frontmatter table drifts from parser

- GIVEN `SKILL.md`'s frontmatter table is edited to omit a parser-supported key among `feature`, `parent_ref`, `base_sha`, `expected_parent_sha`, `legacy_main`
- WHEN `go test ./internal/packet` runs
- THEN the test MUST fail, reporting the undocumented key
