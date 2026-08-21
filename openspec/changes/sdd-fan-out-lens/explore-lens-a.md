# Explore Lens A — Current State: sdd-fan-out-lens

## What the convention specifies

The three-lens design fan-out (`plugin/claude-code/skills/lucind-ai/SKILL.md:126-233`) pilots a 4-lane pipeline across four templates in `plugin/claude-code/skills/lucind-ai/assets/`:

- **Topology & Slices**: Three parallel `agy` lanes author disjoint drafts: Lens A owns approach and decisions (`plugin/claude-code/skills/lucind-ai/SKILL.md:145`, `plugin/claude-code/skills/lucind-ai/assets/design-lens-a-packet-template.md:16`); Lens B owns flow/invariants, surface deltas, and file changes with terminal consumers (`plugin/claude-code/skills/lucind-ai/SKILL.md:146`, `plugin/claude-code/skills/lucind-ai/assets/design-lens-b-packet-template.md:16`); Lens C owns testing strategy, seams, threat matrix, and rollback (`plugin/claude-code/skills/lucind-ai/SKILL.md:147`, `plugin/claude-code/skills/lucind-ai/assets/design-lens-c-packet-template.md:16`). One sequential `cursor-agent` lane synthesizes canonical `design.md` and notes (`plugin/claude-code/skills/lucind-ai/SKILL.md:148`, `plugin/claude-code/skills/lucind-ai/assets/design-synthesis-packet-template.md:16`).
- **Two Invocations (No Sidecar)**: Wave 1 runs three write packets concurrently via `lucind-ai run --packet ...` without `apply-dag.yaml` (`plugin/claude-code/skills/lucind-ai/SKILL.md:153-159`). Following auto-integration, Wave 2 dispatches `design-synthesis.md` branched from the integrated tree (`plugin/claude-code/skills/lucind-ai/SKILL.md:160-165`).
- **Assumed Architecture & Divergence**: Lenses declare `## Assumed architecture`. Synthesis treats Lens A as authoritative and logs deviations under `## Architecture Divergence` (`plugin/claude-code/skills/lucind-ai/SKILL.md:167-173`).
- **Budgets & Compression Gap**: Lens drafts are capped <1000 words; canonical `design.md` <1800 words (`plugin/claude-code/skills/lucind-ai/SKILL.md:178-180`). Compressing ~3000 words of input to 1800 forces synthesis over concatenation (`plugin/claude-code/skills/lucind-ai/SKILL.md:181-186`).
- **Asymmetric Precedence**: Skill (`~/.claude/skills/sdd-design/`) governs document sections; packet governs execution topology, budgets, and criteria (`plugin/claude-code/skills/lucind-ai/SKILL.md:197-214`).
- **Synthesis Outputs & Review**: Synthesizer verifies citations (`plugin/claude-code/skills/lucind-ai/SKILL.md:221-225`), checks 8-item design spine coverage (`plugin/claude-code/skills/lucind-ai/SKILL.md:227-233`), and emits `design.md` plus `design-synthesis-notes.md` (4 sections: Unresolved Contradictions, Coverage Gaps, Dropped Citations, Architecture Divergence). Orchestrator reviews only notes (`plugin/claude-code/skills/lucind-ai/SKILL.md:216-219`).

## Dispatch machinery it rides on

| Element | Where | What it does |
|---|---|---|
| Executor mapping & checks | `cmd/lucind-ai/cli.go:57-61, 187-239` | Maps executors; validates executor, agent, and model before dispatch. |
| Repeatable `--packet` flag | `cmd/lucind-ai/cli.go:67-85, 132-134` | Accumulates `--packet` flags into a multi-lane batch. |
| Batch dispatch & exit code | `cmd/lucind-ai/cli.go:273-329` | Runs batch via `run.ExecuteBatch`, integrates via `run.Integrate`, exits 0 on success. |
| Frontmatter parser | `internal/packet/packet.go:33-167` | Parses YAML frontmatter (`id`, `executor`, `allowed_paths`, `read_only`) and body. |
| Scope prefix matching & disjointness | `internal/packet/disjoint.go:13-48` | `PathInScope` and `DisjointAllowedPaths` validate disjointness before worktree creation (`cmd/lucind-ai/cli.go:243-246`). |
| Diff overlap classification | `internal/overlap/overlap.go:26-99` | Defines patch metrics, change labels, and overlap threshold classification. |
| Concurrent batch loop & timeouts | `internal/run/batch.go:66-147` | Runs lanes concurrently via `sync.WaitGroup` with independent per-lane deadlines. |
| Failure persistence | `internal/run/batch.go:159-209` | `ensureLaneFailed` registers never-started lanes as `lane.Failed` in ledger/barrier. |
| Tree combination & bisection | `internal/run/integrate.go:31-98, 186-253` | Merges lane branches via `CombineTree`, runs `RunChecks`, promotes or bisects. |
| Diff & completion guards | `internal/run/run.go:379-385, 590-662` | `enforceAllowedPaths` demotes out-of-scope diffs; `enforceCompletionMode` checks commit state. |
| Worktree isolation & identity | `internal/worktree/worktree.go:79-81, 150-171, 263-300` | Creates worktrees at `../<repo>-worktrees/<id>` on `lucind/<id>`, checking `.git` and commits. |

## Built versus convention

| Fan-out element | Enforced by | Or prose only |
|---|---|---|
| Executor mapping (`agy` lenses, `cursor-agent` synthesis) | `cmd/lucind-ai/cli.go:57-61, 187-197` | Prose only (`plugin/claude-code/skills/lucind-ai/SKILL.md:143-148`); binary allows any known executor per packet. |
| Two-invocation wave dispatch | `cmd/lucind-ai/cli.go:132-134`, `internal/run/integrate.go:31-81` | Prose only (`plugin/claude-code/skills/lucind-ai/SKILL.md:153-165`); orchestrator sequences wave 1 and wave 2. |
| Omission of `apply-dag.yaml` sidecar | `cmd/lucind-ai/cli.go:124-150` | Prose only (`plugin/claude-code/skills/lucind-ai/SKILL.md:153-155`); convention hand-authors packets without `split`. |
| Slice ownership & reading lists | `internal/packet/disjoint.go:29-48`, `internal/run/run.go:590-626` | Prose only (`plugin/claude-code/skills/lucind-ai/SKILL.md:143-148`, `plugin/claude-code/skills/lucind-ai/assets/design-lens-a-packet-template.md:30-41, 83-91`). |
| Assumed architecture declaration | None | Prose only (`plugin/claude-code/skills/lucind-ai/SKILL.md:167-173`, `plugin/claude-code/skills/lucind-ai/assets/design-lens-a-packet-template.md:50-55`). |
| Word budgets (<1000 lens, <1800 synthesis) | None | Prose only (`plugin/claude-code/skills/lucind-ai/SKILL.md:178-180`, `plugin/claude-code/skills/lucind-ai/assets/design-lens-a-packet-template.md:79-82`). |
| Compression gap constraint | None | Prose only (`plugin/claude-code/skills/lucind-ai/SKILL.md:181-186`). |
| Asymmetric skill precedence | None | Prose only (`plugin/claude-code/skills/lucind-ai/SKILL.md:197-214`, `plugin/claude-code/skills/lucind-ai/assets/design-lens-a-packet-template.md:96-115`). |
| Synthesis notes file & 4 sections | `internal/run/run.go:590-626` (path scope) | Prose only (`plugin/claude-code/skills/lucind-ai/SKILL.md:216-219`, `plugin/claude-code/skills/lucind-ai/assets/design-synthesis-packet-template.md:85-119`). |
| Citation verification pass | None | Prose only (`plugin/claude-code/skills/lucind-ai/SKILL.md:221-225`, `plugin/claude-code/skills/lucind-ai/assets/design-synthesis-packet-template.md:48-55, 147`). |
| Design spine 8-item coverage | None | Prose only (`plugin/claude-code/skills/lucind-ai/SKILL.md:227-233`, `plugin/claude-code/skills/lucind-ai/assets/design-synthesis-packet-template.md:70-84`). |
| Worktree isolation & ledger state | `internal/worktree/worktree.go:168-171`, `internal/run/run.go:451-454` | Prose only (`plugin/claude-code/skills/lucind-ai/assets/design-lens-a-packet-template.md:1-7` for ID convention). |
| Result envelope schema validation | `internal/run/run.go:373, 515-530` | Prose only (`plugin/claude-code/skills/lucind-ai/assets/design-synthesis-packet-template.md:169-171` for findings format). |

## What the existing tests pin

Tests in `internal/packet/packet_test.go` pin prior lifecycle docs and general templates:

- `TestSkillAssetContract` (`internal/packet/packet_test.go:476-516`): Pins `SKILL.md` text for `explore` dispatch, read-only criterion 2 exception, `apply` DAG split (`lucind-ai split --dag`), `verify` dual dispatch, and packet authoring path (`.lucind/packets/`).
- `TestSkillMDVerifyOperationalWorkflow` (`internal/packet/packet_test.go:609-710`): Pins `SKILL.md` verify workflow (mechanical check, qualitative dual dispatch, 4 reconciliation cases).
- `TestVerifyPacketTemplateAssetStructure` (`internal/packet/packet_test.go:518-594`): Pins `plugin/claude-code/skills/lucind-ai/assets/verify-packet-template.md` (`read_only: true`, done criteria, forbidden commands, hard stops, mechanical context).
- `TestPacketTemplateAssetStructure` (`internal/packet/packet_test.go:438-474`): Pins `plugin/claude-code/skills/lucind-ai/assets/packet-template.md` (frontmatter, commit criterion 2, read-only note).
- `TestPacketTemplateVerifyPointerNote` (`internal/packet/packet_test.go:596-607`): Pins pointer note referencing `verify-packet-template.md`.
- `TestHumanPacketTemplateUntouched` (`internal/packet/packet_test.go:722-735`): Pins `plugin/claude-code/skills/lucind-ai/assets/human-packet-template.md` to git `HEAD`.

**Unpinned Surface**:
Zero tests assert on the `Three-lens design fan-out` section (`plugin/claude-code/skills/lucind-ai/SKILL.md:126-233`) or any of the four design packet templates (`plugin/claude-code/skills/lucind-ai/assets/design-lens-a-packet-template.md`, `plugin/claude-code/skills/lucind-ai/assets/design-synthesis-packet-template.md`, etc.).

## Open Questions

- [ ] None
