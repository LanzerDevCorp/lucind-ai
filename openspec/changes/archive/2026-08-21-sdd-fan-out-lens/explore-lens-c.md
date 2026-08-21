# Explore Lens C — Options & Prior Art: sdd-fan-out-lens

## Prior art: what got formalized and what did not

| Convention | Path taken | What distinguished it | Citation |
|---|---|---|---|
| Dual-executor SDD phases (`propose`, `design`, `specs`, `tasks`) | Orchestrator prose in `SKILL.md`; no Go code or specs. | Pure LLM prompt coordination and subjective artifact synthesis; no deterministic machine checks, git invariant gaps, or binary sequencing required. | `plugin/claude-code/skills/lucind-ai/SKILL.md:71-95`, `openspec/changes/archive/2026-08-20-verify-dual-dispatch/proposal.md:34-40` |
| Verify dual dispatch (`verify`) | Formal Go CLI command (`lucind-ai check`) + specifications in `openspec/specs/verify-dual-dispatch/` and `openspec/specs/verify-mechanical-check/` + orchestrator protocol in `SKILL.md`. | Hybrid deterministic/qualitative boundary: mechanical checks had to execute once host-side and freeze an immutable log (`verify-mechanical.log`) to prevent duplicate LLM test suite runs. | `openspec/changes/archive/2026-08-20-verify-dual-dispatch/proposal.md:20-29`, `openspec/changes/archive/2026-08-20-verify-dual-dispatch/tasks.md:16-24`, `openspec/specs/verify-dual-dispatch/spec.md:10-18` |
| Read-only packet execution (`explore`, `verify` judgment) | Go schema addition (`Packet.ReadOnly`) + runtime git invariant check (`enforceCompletionMode`) + specifications in `openspec/specs/read-only-packet-schema/` and `openspec/specs/read-only-done-criterion/`. | Binary invariant mismatch: default runtime required commits for `status: done`, blocking read-only lanes unless an explicit schema key and non-self-reported git inspection were built. | `openspec/changes/archive/2026-08-20-read-only-packet-dispatch/design.md:7-25`, `openspec/changes/archive/2026-08-20-read-only-packet-dispatch/design.md:41-58`, `openspec/specs/read-only-packet-schema/spec.md:9-18` |
| Apply DAG dependency execution (`apply`) | Sidecar YAML parser (`apply-dag.yaml`, `internal/dag/parse.go`) + splitting command (`lucind-ai split`) + diff-scope enforcement (`Packet.AllowedPaths`). | Multi-wave topological ordering, cycle validation, static path disjointness, and post-execution diff containment that cannot be reliably maintained in unstructured prose. | `openspec/changes/archive/2026-08-20-apply-dag-dispatch/proposal.md:1-20`, `openspec/changes/archive/2026-08-20-apply-dag-dispatch/design.md:17-58`, `internal/dag/parse.go:21-36` |

## Candidate scopes

### Candidate 1 — Null Option: Convention & Template Hardening Only (No Go Changes)

**Buys**: Zero binary churn, zero schema additions, and zero runtime maintenance overhead. Preserves complete flexibility for orchestrator prompts, lens definitions, word budgets, and synthesis models without recompiling Go tooling or migrating schema formats.
**Costs**: Orchestrator must continue manually authoring four separate packet files and managing the two-wave barrier (`run` lens A+B+C, then `run` synthesis). No static validation of path disjointness, acyclicity, or word budgets prior to dispatch.
**Forecloses**: Expressing phase fan-out via `apply-dag.yaml` or running `lucind-ai split` for design/explore phases. Machine-enforced word budget validation at packet parse time.
**Would touch**: `plugin/claude-code/skills/lucind-ai/SKILL.md` and template files in `plugin/claude-code/skills/lucind-ai/assets/`.

### Candidate 2 — Additive Sidecar DAG Extension (`read_only` & Fan-Out in `internal/dag`)

**Buys**: Closes the schema gap where `internal/dag/parse.go` omitted `read_only`. Allows any SDD phase fan-out (design lenses, verify judgment, explore) to be authored as an optional sidecar DAG (`<phase>-dag.yaml`) and mechanically split into copy-pasteable wave commands via `lucind-ai split`, with automated path disjointness and dependency validation.
**Costs**: Moderate addition to `internal/dag` (~100-200 lines Go + tests) and parser schema expansion. Requires authoring both sidecar YAML and markdown packet bodies for fan-out phases instead of direct packets.
**Forecloses**: Keeping `internal/dag` strictly isolated to the `apply` phase (`apply-dag.yaml` becomes generic phase DAG support).
**Would touch**: `internal/dag/parse.go`, `internal/dag/dag.go`, `cmd/lucind-ai/split.go`, `internal/dag/*_test.go`, `cmd/lucind-ai/split_test.go`, `openspec/specs/`.

### Candidate 3 — Dedicated Fan-Out Scaffolding Command (`lucind-ai fanout`)

**Buys**: Eliminates manual orchestrator authoring of boilerplate lens and synthesis packets. Deterministically generates all four packets from templates for a given change ID, validates lens output paths and budget ratios at generation time, and emits the exact two-wave CLI execution commands.
**Costs**: Significant CLI surface expansion and new Go package (`internal/fanout`). Hardcodes the three-lens plus synthesizer topology into compiled Go code, requiring binary updates if lens slices, models, or phase structures change.
**Forecloses**: Dynamic or ad-hoc lens reconfiguration by prompt alone without CLI flag/subcommand changes.
**Would touch**: `cmd/lucind-ai/cli.go`, new `internal/fanout/` package, template assets, CLI tests, `openspec/specs/`.

## The deciding question

Does multi-lens fan-out introduce deterministic machine invariants that require Go binary enforcement, or is it an orchestrator prompt and synthesis convention?

## Open Questions

- [ ] None
