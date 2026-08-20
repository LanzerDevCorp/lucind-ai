# Proposal: Read-only packet dispatch

Let a packet declare itself read-only so `lucind-ai run` can finish `status: "done"` with an empty working tree and no commit. First consumer: the `explore` SDD phase, which today Claude Code still runs locally as a subagent because the packet template forbids a no-commit finish. Write packets stay the default. The working dual-executor propose / design / specs / tasks flow does not change.

## Intent

`explore` should dispatch through `lucind-ai run` (Claude Code orchestrates; `agy` / `cursor-agent` execute) instead of a local Claude Code subagent. That is already the project's identity (`plugin/claude-code/skills/lucind-ai/SKILL.md:78`). The only named blocker is that explore is read-only, while every packet is told it must commit.

This change removes that blocker with an **opt-in read-only packet class**. It stays general enough that a later sibling (`verify-dual-dispatch`) can reuse the same class for a judgment packet that must not mutate the tree. This proposal does not design that sibling.

The binary already accepts `status: "done"` with no `commit` and no `files_changed`. The gap is that a packet cannot *declare* that outcome as intended, so an explore packet dispatched today would either fabricate a commit or silently violate mandatory criterion 2.

## Current behavior (verified in this worktree)

| Claim | Evidence |
|-------|----------|
| `Packet` has no read-only vs write distinction. Fields are `ID`, `Executor`, `RoutedBy`, `Model`, `Body`. | `internal/packet/packet.go:29-47` |
| `Parse` only copies `id`, `executor`, `routed_by`, `model`. Unknown frontmatter keys are dropped. | `internal/packet/packet.go:65-75` |
| Envelope `commit` and `files_changed` are optional (`omitempty`). | `internal/result/result.go:107-110` |
| Schema required fields are `packet_id`, `status`, `summary`, `hard_stops`. `commit` is not required. | `internal/result/result.schema.json` (mirrored at `.lucind/result.schema.json`) |
| `Envelope.LaneStatus()` maps the self-reported `status` string to `lane.Status`. It does not read `Commit` or `FilesChanged`. | `internal/result/result.go:122-135` |
| After a schema-valid envelope, `decideStatus` returns `envelope.LaneStatus()` with no commit or dirty-tree check. | `internal/run/run.go:419-430` |
| Mandatory criterion 2 ("The work is committed") is prompt text only. | `plugin/claude-code/skills/lucind-ai/assets/packet-template.md:35-41` and `SKILL.md:95-96` |
| Dual-executor propose / design / specs / tasks is an orchestrator convention (`--packet` twice, distinct draft paths, human synthesis). Not binary-enforced. | `SKILL.md:42-63` |
| Explore's named blocker is this missing read-only exception. | `SKILL.md:74-78` |
| `combine` merges each lane branch with `git merge --no-ff` and does not require the branch to have unique commits. | `internal/integrate/integrate.go:44-46` |

Consequence: a read-only packet that returns `status: "done"` with omitted `commit` already flows `Execute` → `decideStatus` → `ExecuteBatch` → barrier → `combine`. The template, not the binary, is what currently makes that result look unfinished.

## Scope

### What changes

- Packets gain a way to **declare** themselves read-only (mechanism is a design-phase question; not chosen here).
- Read-only packets replace mandatory criterion 2 with a checkable "no working-tree mutation" criterion (clean `git status --porcelain`, no fabricated commit).
- Write packets remain the default. Omitting the declaration keeps today's commit convention.
- `SKILL.md`'s explore target (`SKILL.md:78`) becomes dispatchable once this lands. Prose that restates "every packet must commit" (`SKILL.md:95-96`, `packet-template.md:35-41`) must describe the exception.
- The declaration is reusable by any later read-only packet, including a future verify-judgment packet, without this change knowing that packet's contents.

### What stays untouched

- Dual-executor propose / design / specs / tasks: same `--packet` twice, same distinct draft paths, same orchestrator synthesis, same write-packet template.
- `Envelope.LaneStatus()`, `decideStatus`, `ExecuteBatch` (`internal/run/batch.go:66`), and `combine` — they already accept a no-commit `done`.
- `allowed_paths` remains a Markdown section in the packet body (`packet-template.md:48-54`). The binary still does not parse or enforce it (`packet.go:29-47`).
- `approvals-web-ui/`, `apply-dag-dispatch/`, Go implementation, packet templates, and `state.yaml` in this proposal packet. Those belong to other owners or later phases.

## Non-goals

- **No `allowed_paths` enforcement.** Parsing frontmatter into `Packet`, comparing declared paths to the lane diff, and failing a lane that escaped its list are `apply-dag-dispatch`.
- **No DAG splitting.** `ExecuteBatch` stays a flat concurrent batch (`internal/run/batch.go:66`, `84`). Turning `tasks.md` into dependency-ordered waves is `apply-dag-dispatch`.
- **No verify dual-dispatch design.** Splitting mechanical checks (`internal/integrate/integrate.go:79` `Check`) from qualitative dual-judge packets is `verify-dual-dispatch`. This change only keeps the read-only class general enough that that sibling can consume it later.

## Impact on the existing dual-executor propose / design / specs / tasks flow

**Backward compatible. Additive opt-in. Default remains write.**

That flow already works and is what dispatched this packet (`SKILL.md:49-63`). Propose / design / specs / tasks packets do not declare themselves read-only, so they keep mandatory criterion 2, they keep committing, and they keep the same envelope shape (`commit` present, `files_changed` listing the draft path).

No existing packet frontmatter key is renamed or required. `Parse` already ignores unknown keys (`packet.go:65-75`), so a later additive declaration cannot break today's packets unless design chooses a breaking parse — which this proposal forbids.

Exit code and barrier behavior stay as documented (`SKILL.md:133`): a read-only explore that correctly reports `done` is `lane.Done`, same as a write packet that correctly reports `done`.

## Open questions (design phase; not decided here)

Do not pick a field name or schema in this document. Design, running in parallel, owns the mechanism. Open questions to resolve there:

1. How a packet declares read-only (frontmatter key, template variant, body convention, or another signal).
2. Whether the binary enforces the declaration (reject a dirty tree / unexpected commit on a read-only packet) or only the prompt changes.
3. What status a read-only packet gets if it still commits or leaves a dirty tree (`blocked` vs `deviated` vs prompt-only).
4. Whether write packets gain a counterpart code-level commit gate. Today there is none (`result.go:122-135`, `run.go:419-430`); adding one would be a behavior change for every current packet and needs an explicit compatibility check.

## Rollback plan

This proposal packet is documentation only. Revert the commit that adds `openspec/changes/read-only-packet-dispatch/proposal-cursor-agent.md`. Nothing runtime changes.

After implementation: revert the declaration plus the template / `SKILL.md` exception. Write packets never call the read-only path, so they keep working without it. No schema migration is implied by this proposal because no schema is chosen here.
