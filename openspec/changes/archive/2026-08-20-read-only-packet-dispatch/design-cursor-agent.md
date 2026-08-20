# Design: Read-only packet dispatch

`lucind-ai run` gains an optional packet frontmatter flag `read_only: true`. Omitted or `false` keeps today's write semantics. The flag's terminal consumer is a new `run.enforceCompletionMode` call in `Execute`, after `decideStatus` and only on an envelope that mapped to `lane.Done`: a read-only lane may not have unique commits or a non-ignored dirty tree; a write lane must have at least one unique commit and a clean porcelain. That check is the consumption of the field. Explore (and later verify-judgment) packets become dispatchable without faking a commit.

## Quick path

1. Author sets `read_only: true` in packet frontmatter and uses the read-only criterion 2 text from the template.
2. `packet.Parse` records `Packet.ReadOnly`; `Execute` still creates a worktree and reads `.lucind/result.json` as today.
3. If the envelope says `done`, `enforceCompletionMode` inspects git. Pass → `lane.Done`. Fail → `lane.Failed` with a ledger note. `git merge --no-ff` of a branch with no unique commits stays a no-op at integrate.

## Decision 1 — packet schema addition

**Choice**: `read_only` (YAML key) / `Packet.ReadOnly bool` (Go). Default `false` when the key is absent. Parse accepts only `true` or `false` (after trim). Any other value is `packet.ErrInvalidReadOnly` and the document is not a packet.

**Backward compatibility**: `Parse` already ignores unknown frontmatter keys (`packet.go:65-75` — the switch has no `default`). Every existing packet, including the ones that dispatched this design packet, omits the key. After the field is added, omission leaves the bool at its zero value `false`, so they remain write packets. No required-key error is added; `ErrMissingID` / `ErrMissingExecutor` / `ErrMissingRoutedBy` / `ErrEmptyBody` (`packet.go:20-26`) stay the only completeness gates.

**Rejected: empty `AllowedPaths` as the read-only marker.** `Packet` today is `ID, Executor, RoutedBy, Model, Body` only (`packet.go:29-47`). `allowed_paths` is still a Markdown section, unparsed. Introducing `AllowedPaths` here would design `apply-dag-dispatch`'s field under a different name, and empty-list would collapse "not declared" with "no paths allowed." `read_only` is orthogonal: apply-dag-dispatch must read this bool if it wants to skip scope-vs-diff on a read-only lane, not reuse emptiness of a list.

**Parse shape** (mirrors `model`, which is optional and literally reflected — `packet.go:39-44`, `packet.go:73-74`):

```go
type Packet struct {
    ID, Executor, RoutedBy, Model, Body string
    ReadOnly bool // false when the key is omitted
}
```

## Decision 2 — done-criterion 2 for a read-only packet

**Choice**: keep mandatory criterion 2 as "the work is committed" for write packets (the default). When `read_only: true`, replace it with: the worktree is unchanged relative to the lane's birth commit. Evidence: `git status --porcelain` empty **and** `git rev-parse HEAD` equals `git merge-base HEAD <primary HEAD>` (no unique commits on `lucind/<id>`). Do not commit.

Why porcelain-empty is achievable: `.lucind/` is gitignored (`.gitignore:2`), so the envelope the executor must write (`.lucind/result.json`) and the schema `Execute` already writes (`run.go:434-447`, `run.go:40-52`) never appear in porcelain. The protocol files are not deliverable work.

| Asset | Change |
|-------|--------|
| `plugin/claude-code/skills/lucind-ai/assets/packet-template.md` | Skeleton stays write-default (no `read_only` in the example frontmatter). Add a short "Read-only packets" note: set `read_only: true`; swap criterion 2 for the unchanged-tree text above. Current write text is `packet-template.md:40-41`. |
| `plugin/claude-code/skills/lucind-ai/assets/human-packet-template.md` | **No change.** It has no frontmatter, is not parsed by `packet.Parse`, and has no "work is committed" criterion (`human-packet-template.md:52-57`). Human packets are the one-secret-command class, not explore. |
| `internal/result/result.schema.json` | `commit` stays optional (not in `required` at line 6). Update its description (`result.schema.json:99-101`): omit on a read-only packet; the binary does not trust this field. Do **not** add a `read_only` property to the envelope — the packet declared the mode; the agent must not self-declare it after the fact. `additionalProperties: false` stays. `.lucind/result.schema.json` is a dispatch-time copy of this embed (`internal/result/schema.go`), not a second source of truth. |
| `plugin/claude-code/skills/lucind-ai/SKILL.md` | Apply-phase only: the explore blocker row (`SKILL.md:78`) and mandatory criterion 2 bullet (`SKILL.md:96`) pick up the same exception. Out of this design packet's allowed paths. |

## Decision 3 — consumption: (a), not (b)

**Choice: (a).** The terminal consumer of `Packet.ReadOnly` is `run.enforceCompletionMode`, called from `Execute` after `decideStatus` (`run.go:315`) and before `SetStatus` (`run.go:338`), and only when `decideStatus` returned `lane.Done`.

Today nothing in the binary enforces "must commit" for any packet. `Envelope.LaneStatus()` (`result.go:122-134`) maps the self-reported `status` string to `lane.Status` and never inspects `Envelope.Commit` (`result.go:110`, `omitempty`) or `Envelope.FilesChanged` (`result.go:107`). `decideStatus` (`run.go:407-431`) returns that mapping once the envelope is schema-valid. `combine` then runs `git merge --no-ff` (`integrate.go:45`); a lane branch with no unique commits is already-up-to-date — write work that forgot to commit is marked `Done` and silently dropped. Option (b) (template-only, zero Go) would leave that hole and would add a schema field with no runtime reader, which is the indirection rule this change exists to honor.

`LaneStatus` stays a pure map. Enforcement lives in `run` because `Execute`'s job is already "decide its terminal status from what actually happened" (`run.go:207-209`), and `decideStatus` today does not receive the `Packet` (`run.go:407`). Do not stuff git into `decideStatus`; keep that function as "timeout / non-zero / unreadable envelope / else trust `LaneStatus`."

**Invariant** (git is the authority, not `Envelope.Commit` / `files_changed`):

| Packet | Envelope mapped to `Done` | Required git fact | On miss |
|--------|---------------------------|-------------------|---------|
| `ReadOnly == false` (default, all current packets) | unique commits **and** porcelain empty | `lane.Failed`, ledger note | |
| `ReadOnly == true` | **no** unique commits **and** porcelain empty | `lane.Failed`, ledger note | |

Other envelope statuses (`blocked` / `deviated` / `failed`) are not overridden. Inspector errors (git cannot be run) are also `Failed` — not `blocked`; no human decision is needed.

**Unique commits**: `HEAD` of the worktree ≠ `merge-base(HEAD, primaryRoot HEAD)`. `worktree.Create` births the lane branch at primary HEAD via `git worktree add -b` (`worktree.go:74-82`). This comparison stays correct if primary moves during dispatch (a clean read-only worktree's HEAD remains the merge-base).

**Deps** (run stays testable without git — `run.go:1-8`):

```go
HasUniqueLaneCommits func(ctx context.Context, worktreePath string) (bool, error)
PorcelainEmpty       func(ctx context.Context, worktreePath string) (bool, error)
```

Production wires them in `cmd/lucind-ai/cli.go:169-199` to new `worktree` helpers that `exec` git. Call them with `persistCtx` (`run.go:301`), same as other post-dispatch bookkeeping. `newTestDeps` (`run_test.go:74-97`) must stub write-success (`has unique commits == true`, `porcelain empty == true`) so `TestExecuteHappyPathEnvelopeDoneReachesLaneDone` (`run_test.go:100`) keeps passing — `testPacket()` omits `ReadOnly`. Nil funcs at `Done` time are a binary wiring bug → `Failed`, not a silent skip.

This write-packet strengthening is an intended behavior change, not an accident: a write packet that previously reached `Done` with nothing to merge will now reach `Failed`.

## Decision 4 — rollback

No ledger/schema version. Apply is additive (one bool, two Deps funcs, one guard, template copy). Rollback is `git revert` of the apply commit.

After revert, `read_only:` in a packet document is ignored again (`packet.go:65-75`). In-flight explore packets become write packets under the template and would need a commit to satisfy criterion 2 — the pre-change world. No migration, no dual-read window, no feature flag.

## Data flow

```
Parse(frontmatter) → Packet.ReadOnly (false if omitted)
Execute → worktree.Create → writeResultSchema → Executor.Run
       → decideStatus (unchanged: timeout / exit / envelope.LaneStatus)
       → if status == Done: enforceCompletionMode(persistCtx, deps, path, p)
            miss → status = Failed, reason set (existing ledger-note path)
       → SetStatus
combine: git merge --no-ff (already-up-to-date when read-only passed)
```

## File changes (apply phase — not this packet)

| File | Action |
|------|--------|
| `internal/packet/packet.go` | `ReadOnly bool`; parse `read_only`; `ErrInvalidReadOnly` |
| `internal/packet/packet_test.go` | omitted / `true` / `false` / invalid |
| `internal/worktree/worktree.go` | `HasUniqueCommits`, `PorcelainEmpty` |
| `internal/worktree/worktree_test.go` | real git, as the rest of this package |
| `internal/run/run.go` | Deps funcs; `enforceCompletionMode`; call site between `:315` and `:338` |
| `internal/run/run_test.go` | stub `newTestDeps`; matrix of the four `Done` cells |
| `cmd/lucind-ai/cli.go` | wire the two funcs |
| `plugin/claude-code/skills/lucind-ai/assets/packet-template.md` | read-only criterion 2 note |
| `plugin/claude-code/skills/lucind-ai/SKILL.md` | explore blocker + criterion 2 |
| `internal/result/result.schema.json` | `commit` description only |
| `human-packet-template.md` | none |

## Testing strategy

| Layer | RED | GREEN |
|-------|-----|--------|
| Parse | `read_only: yes` → `ErrInvalidReadOnly`; omitted → `ReadOnly==false` | `true`/`false` round-trip |
| `enforceCompletionMode` | write+done+no unique commits → `Failed`; read-only+done+a commit → `Failed` | write+unique+clean → `Done`; read-only+no unique+clean → `Done` |
| Execute wiring | `newTestDeps` happy path still `Done` via write stubs | read-only test packet with opposite stubs |
| worktree git | dirty untracked non-ignored file fails porcelain; ignored `.lucind/` does not | merge-base equality on a fresh `worktree.Create` |

## Threat matrix

| Boundary | Applicability | Reason |
|----------|---------------|--------|
| Git repository selection | Applicable | New git is `git -C worktreePath` / `git -C primaryRoot` only — the same two roots `Execute` already owns. No new selector. |
| Commit state | Applicable | Inspect only; never `commit -a`. Porcelain uses default ignore (`.lucind/` stays out). |
| Documentation-like paths / push / PR argv | N/A | No new Markdown execution, push, or PR commands. |

## Out of scope (unchanged)

`allowed_paths` parsing and DAG splitting (`apply-dag-dispatch`). Verify-phase dual-dispatch (`verify-dual-dispatch`), which depends on this flag existing first. `Envelope.LaneStatus` remains a string map.

## Open questions

None. The four required decisions are the recommendations above.
