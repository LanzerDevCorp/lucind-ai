# Spec Lens B — Edges & Modifications: sdd-fan-out-lens

## Assumed capability

none — no delta (convention and template hardening only; no runtime binary or spec modifications)

## Edge and failure scenarios

#### Scenario: Documentation frontmatter table drifts from parser
- **GIVEN** `plugin/claude-code/skills/lucind-ai/SKILL.md` frontmatter table is modified and omits a parser-supported key (e.g. `feature`, `parent_ref`, `base_sha`, `expected_parent_sha`, `legacy_main` accepted by `internal/packet/packet.go:63-72,114-130`)
- **WHEN** `go test ./internal/packet` executes contract tests in `internal/packet/packet_test.go`
- **THEN** the test MUST fail, reporting the undocumented frontmatter key before changes are integrated

#### Scenario: Malformed template fails packet parsing
- **GIVEN** an authoring template under `plugin/claude-code/skills/lucind-ai/assets/` missing a mandatory frontmatter key (`id`, `executor`, `routed_by`), having an empty body, or containing a non-JSON `allowed_paths` array
- **WHEN** `packet.Parse` (`internal/packet/packet.go:77-165`) or template contract tests validate the template
- **THEN** parsing MUST fail with an explicit error (`ErrMissingID`, `ErrMissingExecutor`, `ErrMissingRoutedBy`, `ErrEmptyBody`, or JSON syntax error) and reject the packet

#### Scenario: Overlapping lens templates fail upfront batch disjointness
- **GIVEN** two wave-1 lens packets declare overlapping directory or file scopes in `allowed_paths` (e.g. both claiming `openspec/changes/<id>/draft.md`)
- **WHEN** `lucind-ai run` evaluates `packet.DisjointAllowedPaths` (`internal/packet/disjoint.go:24-48`, called from `cmd/lucind-ai/cli.go:243`) prior to lane execution
- **THEN** dispatch MUST fail closed and reject the batch before any worktree is created via `worktree.Create` (`openspec/specs/allowed-paths-enforcement/spec.md:42-60`)

#### Scenario: Copied packet lacking feature-target fields fails admission
- **GIVEN** a packet document copied from documentation omits all four feature-target fields (`feature`, `parent_ref`, `base_sha`, `expected_parent_sha`) and omits `legacy_main: true`
- **WHEN** `lucind-ai run` evaluates lane admission (`internal/packet/packet.go:63-72`; `plugin/claude-code/skills/lucind-ai/SKILL.md:157-161,178-182`)
- **THEN** admission MUST fail closed with `status: failed` and an empty worktree path, preventing worktree creation and executor execution (`openspec/specs/parent-feature-integration/spec.md:7-18`)

#### Scenario: Wave-2 synthesizer dispatched before wave 1 integrates
- **GIVEN** wave-1 lens execution completed in isolated worktrees (`../<repo>-worktrees/<id>`) but has not yet integrated into the primary branch `HEAD` (`internal/run/integrate.go:31-81`)
- **WHEN** wave 2 dispatches the synthesizer in a worktree branched from unintegrated `HEAD`
- **THEN** the synthesizer worktree MUST NOT contain the wave-1 draft files, causing precondition failure (`explore.md:9,42`; `plugin/claude-code/skills/lucind-ai/SKILL.md:184-186`)

#### Scenario: Over-budget lens draft constrained by synthesis compression
- **GIVEN** a wave-1 lens produces a draft exceeding its declared word ceiling (e.g. >1000 words for design/explore drafts)
- **WHEN** the wave-2 synthesizer processes the draft against the canonical document ceiling (<1800 words, `plugin/claude-code/skills/lucind-ai/SKILL.md:199-207`)
- **THEN** the synthesizer MUST arbitrate and compress the feedstock below the canonical ceiling rather than concatenating over-budget text, recording unresolved issues in synthesis notes (`assets/design-synthesis-packet-template.md:70-119`)

#### Scenario: Out-of-scope file mutation during lens execution demotes status to Deviated
- **GIVEN** a lens packet with `allowed_paths: ["openspec/changes/<id>/specs-lens-b/"]` modifies a file outside that directory
- **WHEN** `decideStatus` evaluates the four-way diff union against `Worktree.BaseSHA` via `enforceAllowedPaths` (`internal/run/run.go:590-626`)
- **THEN** lane status MUST be demoted from `Done` to `Deviated`, the offending path recorded in the ledger note, and the lane excluded from integration (`openspec/specs/allowed-paths-enforcement/spec.md:95-118`)

#### Scenario: Read-only lens lane creating a commit fails completion mode
- **GIVEN** an inspection or explore packet with `read_only: true` produces a git commit in its worktree
- **WHEN** `Execute` evaluates `enforceCompletionMode` (`internal/run/run.go:634-662`) after the agent reports `status: done`
- **THEN** the lane MUST be marked `lane.Failed` with a ledger note, regardless of self-reported envelope success (`openspec/specs/completion-mode-enforcement/spec.md:47-65`)

## Existing specs: does anything move?

| Spec | Verdict | Evidence |
|---|---|---|
| `read-only-packet-schema` | unchanged | `openspec/specs/read-only-packet-schema/spec.md:9-83`; `proposal.md:35-37`. Parsing of `read_only` boolean and write default are untouched. |
| `read-only-done-criterion` | unchanged | `openspec/specs/read-only-done-criterion/spec.md:9-59`; `proposal.md:35-37`. Commit criterion for write packets and unchanged-tree criterion for read-only packets are preserved. |
| `allowed-paths-enforcement` | unchanged | `openspec/specs/allowed-paths-enforcement/spec.md:9-155`; `proposal.md:9,35-37`; `explore.md:15,19,31`. Batch disjointness and post-execution 4-way diff union checks operate unmodified. |
| `completion-mode-enforcement` | unchanged | `openspec/specs/completion-mode-enforcement/spec.md:9-83`; `proposal.md:9,35-37`; `explore.md:19`. Independent git verification for write (>=1 commit, clean tree) and read-only (0 commits, clean tree) remains intact. |
| `apply-dag-dispatch` | unchanged | `openspec/specs/apply-dag-dispatch/spec.md:9-167`; `proposal.md:13,23-28,35-37,81-82`; `explore.md:38,44`. Sidecar DAG parsing, validation, and split remain apply-phase only; planning fan-out uses hand-authored `--packet` waves. |
| `parent-feature-integration` | unchanged | `openspec/specs/parent-feature-integration/spec.md:5-65`; `proposal.md:11,35-37,54`; `explore.md:9-10,16`. Strict admission requiring feature target fields or explicit legacy mode is honored; documentation is updated to match. |

## Open Questions

- [ ] Whether contract tests in `internal/packet/packet_test.go` should assert against exact string matches in `SKILL.md` or parse the Markdown table AST (`proposal.md:89`).
- [ ] Whether synthesis-notes files across all planning phases require a structured machine-parseable frontmatter or stay sectioned Markdown (`proposal.md:90`).
