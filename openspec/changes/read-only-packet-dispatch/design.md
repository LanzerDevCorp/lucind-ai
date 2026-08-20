# Design: Read-Only Packet Dispatch

`lucind-ai run` gains an optional packet frontmatter flag `read_only: true` (default `false`). The flag's terminal consumer is a new post-`decideStatus` check in `internal/run.Execute`: on an envelope that mapped to `lane.Done`, a write packet must show unique commits on its lane branch **and** a clean working tree; a read-only packet must show **no** unique commits **and** a clean working tree. Git state is the authority — not the envelope's self-reported `commit`/`files_changed` fields, which an executor could misreport. Explore (and later `verify-dual-dispatch`'s judgment packets) become dispatchable without faking a commit.

Both independently-drafted designs (agy, cursor-agent) converged on the same core architecture — add the bool, enforce it in the runtime, strengthen the write-packet invariant as a deliberate side effect. Where they diverged, this document picks one and records why; the losing alternative is kept as a rejected option, not silently dropped.

## Decision 1 — packet schema addition

**Choice**: `read_only` (YAML key) / `Packet.ReadOnly bool` (Go), default `false` when the key is absent or omitted.

```go
type Packet struct {
    ID, Executor, RoutedBy, Model, Body string
    ReadOnly bool // false when the key is omitted
}
```

**Backward compatibility**: `Parse` already ignores unknown frontmatter keys (`internal/packet/packet.go:65-75` — no `default` case in the switch). Every existing packet — including the eight that dispatched the packets producing this very design — omits the key, so the zero value `false` applies and they remain write packets. `ErrMissingID` / `ErrMissingExecutor` / `ErrMissingRoutedBy` / `ErrEmptyBody` stay the only completeness gates; `read_only` adds no new required key.

**Rejected alternatives**:
- A string enum `mode: read_only | write` — more verbose than needed for a strictly binary distinction.
- Inferring read-only status from packet ID or lane name (e.g. an `explore-` prefix) — fragile implicit coupling, and violates the project's explicit-routing principle (`packet.go:34-38`: `routed_by` exists precisely so routing decisions are never inferred from a name).
- Reusing an empty `AllowedPaths` as the read-only marker — `Packet` has no `AllowedPaths` field yet (that belongs to the sibling `apply-dag-dispatch` change), and even if it did, an empty list would ambiguously collapse "not declared" with "no paths allowed." `read_only` must be orthogonal to whatever `apply-dag-dispatch` designs.
- No schema addition, prompt-convention only — leaves the binary unable to distinguish read-only lanes at status-decision or integration time; also the option this design explicitly rejects in Decision 3.

## Decision 2 — done-criterion 2 for a read-only packet

**Choice**: write packets (the default) keep mandatory criterion 2 exactly as-is — "the work is committed," evidence `git status --porcelain` empty and `git log --oneline -1`. A `read_only: true` packet replaces it with: **the worktree carries no unique commits and no working-tree changes relative to the lane's birth point.** Evidence: `git status --porcelain` empty **and** the worktree's `HEAD` equals `git merge-base HEAD <primary HEAD>`.

The merge-base check is required, not optional — porcelain-empty alone is insufficient: a lane could commit and leave a clean tree afterward, satisfying "no dirty files" while still mutating history. Checking for the absence of unique commits closes that gap.

Why porcelain-empty is achievable at all: `.lucind/` is gitignored, so `.lucind/result.json` (the envelope every packet, read-only or not, must write) never appears in `git status --porcelain`. The protocol file is not deliverable work and does not count as a mutation.

| Asset | Change |
|---|---|
| `plugin/claude-code/skills/lucind-ai/assets/packet-template.md` | Skeleton stays write-default (no `read_only` in the example frontmatter). Add a "Read-only packets" note: set `read_only: true`, swap criterion 2 for the unchanged-tree text above. Current write text: `packet-template.md:40-41`. |
| `plugin/claude-code/skills/lucind-ai/assets/human-packet-template.md` | **No change.** No frontmatter, not parsed by `packet.Parse`, no "work is committed" criterion — human packets are the one-secret-command class, unrelated to `explore`. |
| `internal/result/result.schema.json` | `commit` stays optional (not in `required`, line 6). Update its description: omitted on a read-only packet; the binary does not trust this field for enforcement (see Decision 3). Do **not** add a `read_only` property to the envelope — the packet declares the mode up front; the agent must not be able to self-declare it after the fact. `additionalProperties: false` stays as-is. |
| `plugin/claude-code/skills/lucind-ai/SKILL.md` | Apply-phase only, out of this design packet's own allowed paths: the explore-blocker row (`SKILL.md:78`) and mandatory-criterion-2 bullet (`SKILL.md:96`) pick up the same exception text. |

## Decision 3 — consumption: independent git inspection, not the self-reported envelope

**Choice**: the terminal consumer of `Packet.ReadOnly` is a new function (`run.enforceCompletionMode` or equivalent), called from `Execute` immediately after `decideStatus` returns `lane.Done`, and *before* the final status is persisted. `decideStatus` itself stays unchanged — a pure "timeout / non-zero exit / unreadable envelope / else trust `envelope.LaneStatus()`" function, exactly as it is today. The new check inspects real git state through two new `Deps` functions, following the project's existing dependency-injection pattern (`deps.WorktreeFS` is the precedent):

```go
HasUniqueLaneCommits func(ctx context.Context, worktreePath string) (bool, error)
PorcelainEmpty       func(ctx context.Context, worktreePath string) (bool, error)
```

| Packet | Envelope mapped to `Done` | Required git fact | On miss |
|---|---|---|---|
| `ReadOnly == false` (default, every current packet) | unique commits **and** porcelain empty | `lane.Failed`, ledger note |
| `ReadOnly == true` | **no** unique commits **and** porcelain empty | `lane.Failed`, ledger note |

Other envelope statuses (`blocked`/`deviated`/`failed`) are not overridden. A git-inspection error (git itself cannot run) also resolves to `Failed`, not `blocked` — no human decision is needed for a binary-wiring problem.

**Why git, not `Envelope.Commit`/`Envelope.FilesChanged`**: those fields are self-reported by the executor inside its own worktree. Today `Envelope.LaneStatus()` (`internal/result/result.go:122-135`) never reads them at all — nothing currently verifies a packet's claim against reality. An enforcement mechanism that only checks `envelope.Commit != ""` (a string the executor writes itself) can be satisfied by a hallucinated or copy-pasted value; it does not actually prove a commit happened. Inspecting the worktree's real git state (`git merge-base`, `git status --porcelain`) cannot be fooled this way, and mirrors the packet template's own standing principle for every done-criterion: "prefer a command whose output proves it over a claim that it works" (`packet-template.md:33-34`). This was the deciding factor between the two independent drafts, which otherwise agreed on adding the same field.

**Rejected: enforcing inline inside `decideStatus` using `envelope.Commit`/`envelope.FilesChanged`.** One draft proposed exactly this. Rejected because it trusts the self-reported envelope rather than verifying it, and because it would make `decideStatus` do two unrelated jobs (interpret the envelope vs. verify git truth) instead of keeping it a pure mapping function with verification as a separate, independently testable step.

**Rejected: also filtering read-only lanes out of `CombineTree`'s branch list in `internal/run/integrate.go`.** One draft proposed skipping read-only lanes explicitly during `combine` as an optimization. Rejected as unnecessary complexity: a read-only lane that passes `enforceCompletionMode` has, by construction, zero unique commits on its branch, so `git merge --no-ff` against it is already a correct, harmless no-op — `combine` needs no read-only-awareness at all.

**This is an intended, disclosed behavior change for every existing write packet**, not a side effect to bury: a write packet that today reaches `Done` without actually committing (nothing currently stops this) will, after this change, reach `Failed` instead. This is the change's explicit goal per the accepted proposal's open question 4, but it means every current packet author benefits from — and is now held to — a stronger guarantee than before.

## Decision 4 — rollback

No ledger or schema version bump. The change is purely additive: one bool field, two new `Deps` functions, one new guard call, template text. Rollback is a straight revert of the apply commit(s) touching `internal/packet/`, `internal/run/`, and the `plugin/claude-code/skills/lucind-ai/assets/` templates.

After revert: `read_only:` in a packet document is silently ignored again (unknown-key drop in `Parse`). Any in-flight explore-style packet reverts to being treated as a write packet under the template and would need a real commit to satisfy criterion 2 — i.e., exactly today's world. No migration, no dual-read window, no feature flag, zero database impact (`internal/ledger`'s SQLite schema is untouched).

## Data flow

```
Parse(frontmatter) -> Packet.ReadOnly (false if omitted)
Execute -> worktree.Create -> writeResultSchema -> Executor.Run
        -> decideStatus (unchanged: timeout / exit / envelope.LaneStatus)
        -> if status == Done: enforceCompletionMode(ctx, deps, worktreePath, packet)
             checks: HasUniqueLaneCommits, PorcelainEmpty
             write packet:      commits required, clean tree required
             read-only packet:  no commits allowed, clean tree required
             miss -> status = Failed, ledger note set
        -> SetStatus
combine: git merge --no-ff (already a correct no-op for a passed read-only lane)
```

## File changes (apply phase — not this design packet)

| File | Action |
|---|---|
| `internal/packet/packet.go` | Add `ReadOnly bool`; parse `read_only:` frontmatter key (defaulting `false`); reject a non-boolean value. |
| `internal/packet/packet_test.go` | Cases: omitted (→ `false`), `true`, `false`, invalid value. |
| `internal/worktree/worktree.go` | Add `HasUniqueCommits`, `PorcelainEmpty` git-inspection helpers. |
| `internal/worktree/worktree_test.go` | Real-git tests, consistent with the rest of the package. |
| `internal/run/run.go` | New `Deps` funcs; new `enforceCompletionMode` call site right after `decideStatus`, before `SetStatus`; `decideStatus` itself unchanged. |
| `internal/run/run_test.go` | Stub the two new `Deps` funcs; matrix of the four `Done`-outcome cells (write×clean, write×dirty, read-only×clean, read-only×committed). |
| `cmd/lucind-ai/cli.go` | Wire the two new `Deps` funcs to real git-backed implementations. |
| `plugin/claude-code/skills/lucind-ai/assets/packet-template.md` | Read-only criterion-2 note. |
| `plugin/claude-code/skills/lucind-ai/SKILL.md` | Explore blocker row + mandatory-criterion-2 bullet pick up the exception. |
| `internal/result/result.schema.json` | `commit` field description only — no shape change. |

`internal/run/integrate.go` is explicitly **not** on this list (see Decision 3's rejected alternative).

## Testing strategy

| Layer | RED | GREEN |
|---|---|---|
| `packet.Parse` | Invalid `read_only` value rejected | Omitted → `false`; `true`/`false` round-trip |
| `enforceCompletionMode` | write+done+no unique commits → `Failed`; read-only+done+a commit → `Failed` | write+unique commits+clean → `Done`; read-only+no unique commits+clean → `Done` |
| `Execute` wiring | — | Existing happy-path write test keeps passing unmodified (it stubs unique-commits=true, porcelain=true) |
| Worktree git helpers | A dirty untracked non-ignored file fails porcelain | Gitignored `.lucind/` does not count as dirty; merge-base equality holds on a freshly created worktree |

## Threat matrix

| Boundary | Applicability | Reason |
|---|---|---|
| Git repository selection | Applicable | New git calls are `git -C worktreePath` / `git -C primaryRoot` only — the same two roots `Execute` already owns; no new selector surface. |
| Commit state | Applicable | Inspection only, never a mutating `git commit`/`git commit -a`. Porcelain check uses git's default ignore rules, so `.lucind/` never counts. |
| Documentation-like paths, push, PR argv | N/A | No new Markdown execution, push, or PR commands introduced. |

## Out of scope (owned by sibling changes)

`allowed_paths` parsing and DAG splitting — `apply-dag-dispatch`. Verify-phase dual dispatch — `verify-dual-dispatch`, which depends on `read_only` existing first. `Envelope.LaneStatus()` remains a pure string-to-status map; this change does not touch it.
