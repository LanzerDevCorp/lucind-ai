# Explore: sdd-fan-out-lens

The three-lens design fan-out is a committed convention — prose plus four packet templates — with no Go support specific to it. Generic dispatch already runs the topology. This exploration asks what, if anything, the binary must grow. The null option is live.

## What exists today

**Convention** (`plugin/claude-code/skills/lucind-ai/SKILL.md:126-151`): a 4-lane pipeline for the `design` phase only. Three parallel `agy` lanes own disjoint drafts — A: technical approach and architecture decisions except rollback (`plugin/claude-code/skills/lucind-ai/SKILL.md:145`); B: flow, invariants, surface deltas, file changes (`plugin/claude-code/skills/lucind-ai/SKILL.md:146`); C: testing, threat matrix, rollback (`plugin/claude-code/skills/lucind-ai/SKILL.md:147`). One sequential `cursor-agent` lane writes canonical `design.md` plus `design-synthesis-notes.md` (`plugin/claude-code/skills/lucind-ai/SKILL.md:148`). Templates live under `plugin/claude-code/skills/lucind-ai/assets/`.

**Dispatch** is two `lucind-ai run --packet` invocations with no sidecar (`plugin/claude-code/skills/lucind-ai/SKILL.md:153-155,163-176`). Wave 1 writes three disjoint draft paths concurrently; after integration, wave 2 branches the synthesizer from the integrated tree (`plugin/claude-code/skills/lucind-ai/SKILL.md:184-186`). Templates carry `legacy_main: true` because every packet must name feature-target fields or declare legacy mode (`plugin/claude-code/skills/lucind-ai/SKILL.md:157-161`).

Lenses open with `## Assumed architecture`; synthesis treats A's as authoritative and records B/C deviations under `## Architecture Divergence` (`plugin/claude-code/skills/lucind-ai/SKILL.md:188-193`). Lens drafts stay under 1000 words; canonical `design.md` under 1800 (`plugin/claude-code/skills/lucind-ai/SKILL.md:199-200`). Compressing ~3000 words of feedstock to 1800 forces arbitration (`plugin/claude-code/skills/lucind-ai/SKILL.md:202-207`). Skill `~/.claude/skills/sdd-design/` wins on document contents; the packet wins on execution topology, budgets, and criteria (`plugin/claude-code/skills/lucind-ai/SKILL.md:218-227`). Citation verification and the eight-item design spine are synthesis-packet procedure (`plugin/claude-code/skills/lucind-ai/assets/design-synthesis-packet-template.md:48-55,70-84,147-148`). The orchestrator reads the notes file, not the three drafts (`plugin/claude-code/skills/lucind-ai/SKILL.md:130-131`; `plugin/claude-code/skills/lucind-ai/assets/design-synthesis-packet-template.md:94`).

**Machinery the convention rides:**

- Repeatable `--packet`, executor/agent/model checks, upfront `DisjointAllowedPaths`, `ExecuteBatch`, `Integrate` (`cmd/lucind-ai/cli.go:57-61,79-85,132-134,187-246,285-329`).
- Frontmatter: `id`, `executor`, `routed_by`, `model`, `agent`, `read_only`, feature-target fields, `legacy_main`, `allowed_paths` (`internal/packet/packet.go:33-74,94-165`). Unknown keys ignored.
- Concurrent lanes, independent deadlines, never-started lanes recorded `Failed` (`internal/run/batch.go:37-43,66-113,149-173`).
- CombineTree, checks, promote-or-bisect (`internal/run/integrate.go:31-81,186-253`).
- `enforceAllowedPaths` four-way diff vs birth `BaseSHA`; out-of-scope demotes Done→Deviated (`internal/run/run.go:379-381,547-626`). `enforceCompletionMode`: write needs commits and a clean tree; read-only needs zero commits and a clean tree (`internal/run/run.go:634-662`). Envelope schema via `result.Read` (`internal/run/run.go:515-543`).
- Worktrees at `../<repo>-worktrees/<id>` on `lucind/<id>` (`internal/worktree/worktree.go:79-81,150-171,184-237`).

**Tests pin** prior lifecycle docs, not this convention. `TestSkillAssetContract` and `TestSkillMDVerifyOperationalWorkflow` pin explore dispatch, apply `split --dag`, and verify dual-dispatch text (`internal/packet/packet_test.go:476-516,609-710`). Template tests pin `packet-template.md`, `verify-packet-template.md`, and `human-packet-template.md` (`internal/packet/packet_test.go:438-474,518-607,722-735`). Zero tests assert on `plugin/claude-code/skills/lucind-ai/SKILL.md:126-233` or the four design packet templates.

## Built versus convention

| Element | Binary | Prose only |
|---|---|---|
| Any known executor per packet | `cmd/lucind-ai/cli.go:57-61,187-197` | agy lenses + cursor-agent synthesis assignment (`plugin/claude-code/skills/lucind-ai/SKILL.md:143-148`) |
| Concurrent batch + integrate | `cmd/lucind-ai/cli.go:285-297`; `internal/run/integrate.go:31-81` | two-invocation wave sequencing (`plugin/claude-code/skills/lucind-ai/SKILL.md:153-176`) |
| `run --packet` without a sidecar | `cmd/lucind-ai/cli.go:121-149` | hand-author packets; do not `split` (`plugin/claude-code/skills/lucind-ai/SKILL.md:153-155`) |
| Path disjointness + post-diff scope | `internal/packet/disjoint.go:24-48`; `internal/run/run.go:590-626` | slice ownership and reading lists (`plugin/claude-code/skills/lucind-ai/SKILL.md:143-148`) |
| Worktree isolation + ledger status | `internal/worktree/worktree.go:168-237`; `internal/run/run.go:451-454` | packet ID convention (templates) |
| Envelope schema | `internal/run/run.go:515-543` | findings format (synthesis template) |
| Assumed architecture, word budgets, compression gap, skill/packet precedence, citation pass, 8-item spine, notes 4-section shape | none | `plugin/claude-code/skills/lucind-ai/SKILL.md:188-227`; `plugin/claude-code/skills/lucind-ai/assets/design-synthesis-packet-template.md:70-119,147-150` |

## Constraints and hard blockers

**Sidecar (`apply-dag.yaml` / `lucind-ai split`)** cannot express this fan-out today. `dag.Node` has no `read_only` (`internal/dag/parse.go:22-36`). `Validate` rejects empty `allowed_paths` (`internal/dag/validate.go:11,30-32`). `EmitPacketContent` never emits `read_only: true` (`internal/dag/emit.go:23-53`). The accepted apply spec binds the sidecar to `openspec/changes/<change-id>/apply-dag.yaml` (`openspec/specs/apply-dag-dispatch/spec.md:9-12`); `Parse` itself takes any path (`internal/dag/parse.go:44-46`). Overlap without a `depends_on` path is rejected (`internal/dag/overlap.go:52-75`; `internal/dag/waves.go:65-70`). Body files must exist on disk (`internal/dag/parse.go:75-86`).

**Hand-authored `lucind-ai run --packet`** is not blocked. Wave 1: disjoint write `allowed_paths`, write completion mode, integrate (`internal/packet/packet.go:131-137`; `cmd/lucind-ai/cli.go:243`; `internal/run/run.go:645-653`; `internal/run/integrate.go:62-79`). Wave 2: synthesizer reads promoted drafts (`internal/run/batch.go:66-89`; `plugin/claude-code/skills/lucind-ai/SKILL.md:184-186`). Read-only parallel packets can set `read_only: true` and omit `allowed_paths` (`internal/packet/packet.go:105-113`; `internal/run/run.go:654-662`); envelope aggregation remains orchestrator convention (`openspec/specs/verify-dual-dispatch/spec.md:145-151`). Omitted `AllowedPaths` skips disjointness and post-diff checks (`internal/packet/disjoint.go:30-37`; `internal/run/run.go:379-381`); sidecar empty lists stay forbidden (`internal/dag/validate.go:30-32`).

Lanes do not see sibling uncommitted or unpromoted work: separate worktrees (`internal/worktree/worktree.go:168-237`; `internal/run/batch.go:81-89`). `.lucind/` is gitignored (`.gitignore:2`) and worktree-local (`internal/run/run.go:50-60`); `.lucind/` paths are excluded from scope comparison (`internal/run/run.go:599-601`). Barrier waits for every lane (`internal/run/batch.go:88-95`); only `lane.Done` integrates (`internal/barrier/barrier.go:49-57`).

Existing specs for read-only schema and done-criterion, allowed-paths enforcement, completion-mode, verify dual-dispatch, and sequential-run-per-wave would be honored unchanged by a fan-out that stays on hand-authored packets. Candidate 2 would modify apply-dag-dispatch's sidecar location and non-empty-`allowed_paths`-at-split requirements (`openspec/specs/apply-dag-dispatch/spec.md:9-27,51-59`).

## Candidate scopes

### Candidate 1 — Null option: convention and template hardening only (no Go)

**Buys**: no binary churn; orchestrator keeps prompt, budget, and model flexibility.
**Costs**: four packets hand-authored; two-wave barrier is operator protocol; no parse-time check of disjointness, acyclicity, or word budgets.
**Forecloses**: phase fan-out via sidecar / `lucind-ai split`; machine-enforced budgets at parse time.
**Would touch**: `plugin/claude-code/skills/lucind-ai/SKILL.md` and `assets/` templates.

### Candidate 2 — Additive sidecar DAG extension (`read_only` and non-apply fan-out in `internal/dag`)

**Buys**: close the `read_only` / empty-`allowed_paths` / apply-only location gaps; optional `<phase>-dag.yaml` split into wave commands with disjointness and dependency checks.
**Costs**: moderate `internal/dag` growth plus tests; authors keep sidecar YAML plus body files.
**Forecloses**: keeping `internal/dag` strictly apply-phase.
**Would touch**: `internal/dag/*`, `cmd/lucind-ai/split.go`, specs. (`internal/dag/dag.go` does not exist today.)

### Candidate 3 — Dedicated `lucind-ai fanout` scaffolding

**Buys**: generate the four packets from templates; validate output paths and budget ratios at generation; emit the two-wave commands.
**Costs**: new CLI surface and `internal/fanout`; topology compiled into the binary.
**Forecloses**: ad-hoc lens reconfiguration by prompt alone.
**Would touch**: `cmd/lucind-ai/cli.go`, new `internal/fanout/`, templates, specs.

## Prior art (why some conventions became Go)

Dual-executor propose/design/specs/tasks stays orchestrator protocol on generic `--packet` (`plugin/claude-code/skills/lucind-ai/SKILL.md:71-95`). Verify grew `lucind-ai check` plus specs because mechanical checks had to run once host-side (`openspec/specs/verify-dual-dispatch/spec.md:10-16`; `openspec/changes/archive/2026-08-20-verify-dual-dispatch/tasks.md:16-24`). Read-only grew `Packet.ReadOnly` plus `enforceCompletionMode` because default write completion blocked commitless lanes (`openspec/changes/archive/2026-08-20-read-only-packet-dispatch/design.md:7-12,41-53`). Apply grew sidecar + `split` + `AllowedPaths` because multi-wave order, cycles, and diff containment are not reliable as unstructured prose (`openspec/changes/archive/2026-08-20-apply-dag-dispatch/design.md:17-51`; `internal/dag/parse.go:21-36`).

## The deciding question

Does multi-lens fan-out introduce deterministic machine invariants that require Go binary enforcement, or is it an orchestrator prompt and synthesis convention?

## Open questions

None from the three lenses.
