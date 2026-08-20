# Proposal: Read-Only Packet Dispatch

Let a packet declare itself read-only so `lucind-ai run` can finish `status: "done"` with an empty working tree and no commit. First consumer: the `explore` SDD phase, which today Claude Code still runs locally as a subagent because the packet template forbids a no-commit finish. Write packets stay the default and keep committing exactly as they do now; the working dual-executor propose/design/specs/tasks flow does not change.

## Intent

`explore` should dispatch through `lucind-ai run` (Claude Code orchestrates; `agy`/`cursor-agent` execute) instead of running as a local Claude Code subagent — that is already the project's own identity (`plugin/claude-code/skills/lucind-ai/SKILL.md:74-78`). The only named blocker: explore is read-only, while mandatory done-criterion #2 in `plugin/claude-code/skills/lucind-ai/assets/packet-template.md:40-41` requires "the work is committed."

This proposal removes that blocker with an **opt-in read-only packet class**, general enough that the sibling change `verify-dual-dispatch` can reuse it for a judgment packet that must not mutate the tree, without this change knowing that packet's contents. It establishes only the authoring/contract layer — the exact schema mechanism is left to the parallel `design` phase.

## Current behavior (verified against on-disk source)

| Claim | Evidence |
|---|---|
| `Packet` has no read-only vs. write distinction; fields are `ID`, `Executor`, `RoutedBy`, `Model`, `Body`. | `internal/packet/packet.go:29-47` |
| `Parse` only copies `id`, `executor`, `routed_by`, `model`; unknown frontmatter keys are dropped. | `internal/packet/packet.go:65-75` |
| Envelope `commit` and `files_changed` are optional (`omitempty`). | `internal/result/result.go:107-110` |
| Schema required fields are `packet_id`, `status`, `summary`, `hard_stops` — `commit` is not required. | `internal/result/result.schema.json` (mirrored at `.lucind/result.schema.json`) |
| `Envelope.LaneStatus()` maps the self-reported `status` string to `lane.Status` directly; it never reads `Commit` or `FilesChanged`. | `internal/result/result.go:122-135` |
| `decideStatus` returns `envelope.LaneStatus()` with no commit or dirty-tree check, for any packet. | `internal/run/run.go:419-430` |
| `combine` merges each lane branch with `git merge --no-ff` and does not require the branch to carry a unique commit. | `internal/integrate/integrate.go:44-46` |
| Mandatory criterion 2 ("the work is committed") is **prompt text only** — no code-level gate exists. | `packet-template.md:35-41`, `SKILL.md:95-96` |
| Dual-executor propose/design/specs/tasks is an orchestrator convention (`--packet` twice, distinct draft paths, human synthesis) — not binary-enforced. | `SKILL.md:42-63` |
| Explore's named blocker is exactly this missing read-only exception. | `SKILL.md:74-78` |

**Consequence**: a read-only packet that returns `status: "done"` with `commit` omitted already flows cleanly through `Execute` → `decideStatus` → `ExecuteBatch` → barrier → `combine`. The packet template's prose, not the binary, is what currently makes that result look unfinished. Worktree isolation (`internal/worktree/worktree.go:61-83`) is preserved either way — even read-only lanes run in an isolated worktree, to keep CodeGraph index isolation and avoid interfering with concurrent write lanes.

## Scope

### What changes

- Packets gain a way to **declare** themselves read-only (mechanism is a design-phase question, not chosen here).
- Read-only packets replace mandatory criterion 2 with a checkable "no working-tree mutation" criterion (clean `git status --porcelain`, no fabricated commit).
- Write packets remain the default: omitting the declaration keeps today's commit convention unchanged.
- `SKILL.md`'s explore target (`SKILL.md:74-78`) moves from "target direction, not yet built" to the standard dispatch workflow; the mandatory-criterion prose (`SKILL.md:95-96`, `packet-template.md:35-41`) gains the read-only exception.
- The declaration is reusable by any later read-only packet — including a future `verify-dual-dispatch` judgment packet — without this change needing to know that packet's contents.

### What stays untouched

- Dual-executor propose/design/specs/tasks: same `--packet` twice, same distinct draft paths, same orchestrator synthesis, same write-packet template.
- `Envelope.LaneStatus()`, `decideStatus`, `ExecuteBatch` (`internal/run/batch.go:66`), and `combine` — they already accept a no-commit `done`; no runtime behavior change is required for the read-only path itself.
- `allowed_paths` remains a Markdown section in the packet body (`packet-template.md:48-54`); the binary still does not parse or enforce it — that is `apply-dag-dispatch`'s job, not this change's.
- `approvals-web-ui/` and `apply-dag-dispatch/` — separate, unrelated in-flight changes.

## Non-goals

- **No `allowed_paths` enforcement.** Parsing frontmatter into `Packet`, comparing declared paths to the lane diff, and failing a lane that escaped its list belong to `apply-dag-dispatch`.
- **No DAG splitting.** `ExecuteBatch` stays a flat concurrent batch (`internal/run/batch.go:66,84`). Turning `tasks.md` into dependency-ordered waves belongs to `apply-dag-dispatch`.
- **No verify dual-dispatch design.** Splitting the mechanical check (`internal/integrate/integrate.go:79` `Check`) from qualitative dual-judge packets belongs to `verify-dual-dispatch`. This change only keeps the read-only class general enough for that sibling to consume later.
- **No final schema or field name mandate.** This proposal does not dictate frontmatter shape (e.g. `read_only: true` vs. a template variant vs. another convention) — that determination belongs to the parallel `design` phase.

## Impact on the existing dual-executor propose/design/specs/tasks flow

**Backward compatible. Additive opt-in. Default remains write.**

That flow already works and is what dispatched the packets that produced this very proposal (`SKILL.md:42-63`). Propose/design/specs/tasks packets do not declare themselves read-only, so they keep mandatory criterion 2, keep committing, and keep the same envelope shape (`commit` present, `files_changed` listing the draft path). No existing packet frontmatter key is renamed or required — `Parse` already ignores unknown keys (`packet.go:65-75`), so an additive declaration cannot break today's packets unless design deliberately chooses a breaking parse, which this proposal forbids. Exit-code and barrier behavior stay exactly as documented (`SKILL.md:133`): a read-only explore that correctly reports `done` is `lane.Done`, same as a write packet that correctly reports `done`.

## Open questions for the design phase (not decided here)

1. How a packet declares read-only — frontmatter key, template variant, body convention, or another signal.
2. Whether the binary *enforces* the declaration (reject a dirty tree or an unexpected commit on a read-only packet) or the change is prompt-only.
3. What status a read-only packet gets if it still commits or leaves a dirty tree (`blocked` vs. `deviated` vs. a prompt-only convention).
4. **Whether write packets gain a counterpart code-level commit gate.** Today there is none for *any* packet (`result.go:122-135`, `run.go:419-430`) — adding one would be a genuine behavior change for every current packet and needs an explicit compatibility check. This is also the question that determines whether the new field needs a real Go-code consumer to satisfy the packet template's own "every indirection must be demonstrably consumed" rule.
5. Whether a read-only lane should also produce an ephemeral exploration artifact (e.g. `openspec/changes/<change>/explore-<executor>.md`), or deliver strictly through `.lucind/result.json`'s `findings`/`summary` fields.

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Executor hallucinates a dummy commit to satisfy a perceived git requirement. | Medium | Explicit prompt-level instruction in the packet template stating the commit criterion is N/A for read-only lanes. |
| A write packet is accidentally authored as read-only, bypassing the commit requirement. | Low | Orchestrator review verifies write phases (`apply`, `propose`, `design`, `specs`, `tasks`) keep mandatory criterion 2. |
| Worktree accumulation from repeated read-only exploration runs. | Low | Existing worktree preservation policy (`docs/prd.md:164-166`) applies uniformly; worktrees can be pruned post-synthesis same as today. |

## Rollback plan

This proposal itself is documentation only — reverting its commit changes nothing at runtime. After implementation: revert the declaration mechanism plus the `packet-template.md`/`SKILL.md` exception text. Write packets never exercise the read-only path, so they keep working unmodified whether or not this change is present. No schema migration is implied by this proposal, since no schema is chosen here — that risk, if any, belongs to the design phase's answer to open question 4.

## Success criteria

- [ ] `packet-template.md` explicitly specifies how read-only packets satisfy or declare N/A on mandatory done-criterion #2.
- [ ] `SKILL.md` reflects `explore` as a dispatchable phase via `lucind-ai run`, not a "target, not yet built" entry.
- [ ] Existing dual-executor dispatch for propose/design/specs/tasks continues to operate without breaking changes or regression.
- [ ] A dispatched read-only explore lane returns `status: "done"` with a valid `.lucind/result.json` envelope and no commit, and the binary accepts it as `lane.Done`.

## Dependencies

- Existing `internal/result` schema validation and envelope-to-status mapping.
- Existing `internal/worktree` worktree creation/isolation.
- Existing `SKILL.md` dual-executor orchestration workflow.
