# Design: Apply DAG dispatch

Split `tasks.md` into independent packets with non-overlapping `allowed_paths`, dispatch each dependency wave as one `lucind-ai run`, and let the existing combine / 400-line resolve / bisection / promote path integrate that wave before the next one starts. The novel piece is binary enforcement of declared scope against the lane's actual git diff — not a new integrator.

## Recommendations

| # | Question | Recommendation |
|---|----------|----------------|
| 1 | How are paths and order declared? | New `openspec/changes/<id>/apply-dag.yaml` (not `tasks.md` prose). `lucind-ai split` is the parser. |
| 2 | `Packet.AllowedPaths`? | Yes, `[]string`. Terminal consumer: `decideStatus` rejects `done` when the worktree diff escapes the list. |
| 3 | Who drives waves? | Orchestrator issues one `lucind-ai run` per wave. Do not add wave logic inside `ExecuteBatch`. |
| 4 | Partial failure surfacing? | `IntegrateReport` + ledger already record it. CLI must print integrated/reverted **lane IDs** (today it prints counts only). |
| 5 | Rollback? | Field omitted = today's behavior. No ledger migration. Revert the apply commits. |

## Decision 1 — DAG artifact and exact format

**Choice**: A new YAML file `openspec/changes/<change-id>/apply-dag.yaml`. `tasks.md` stays the human SDD checklist; it is not the parse source. The orchestrator authors the YAML (PRD §6 step 1: lane split is judgment). `lucind-ai split` is the mechanical consumer.

`tasks.md` as it exists today (see `openspec/changes/approvals-web-ui/tasks.md`) is checkbox prose with no `allowed_paths` and no `depends_on`. Parsing that would be NLP, not a splitter.

### `apply-dag.yaml`

```yaml
change: apply-dag-dispatch
packets:
  - id: apply-ledger
    executor: agy
    routed_by: schema and CRUD isolated from HTTP
    model: gemini-3.7-flash-high   # optional
    allowed_paths:
      - internal/ledger/
    depends_on: []
    body_path: bodies/apply-ledger.md
  - id: apply-serve
    executor: agy
    routed_by: HTTP isolated after ledger exists
    allowed_paths:
      - internal/serve/
      - cmd/lucind-ai/cli.go
    depends_on: [apply-ledger]
    body_path: bodies/apply-serve.md
```

Rules the splitter enforces:

| Rule | Reject if |
|------|-----------|
| Unique `id` | Duplicate ids |
| Non-empty `allowed_paths` | Empty list (read-only empty-list is a different change) |
| Acyclic `depends_on` | Cycle (Kahn's algorithm) |
| Same-wave disjoint paths | Two packets in one wave where one path is equal to, or a path-component prefix of, the other |
| Cross-wave overlap | Allowed — that is why they are ordered |
| Overlap + no edge | Two overlapping packets with no `depends_on` path would share a wave → reject; author must add a dependency or shrink paths |

Path match (also used by Decision 2): repo-relative POSIX paths, no globs in v1. `internal/ledger` and `internal/ledger/` both match `internal/ledger/foo.go`. `internal/led` does **not** match `internal/ledger/foo.go` (component boundary).

### `lucind-ai split`

```
lucind-ai split --dag openspec/changes/<id>/apply-dag.yaml --out openspec/changes/<id>/packets/
```

Writes one packet file per node (`packets/<id>.md`) by concatenating generated frontmatter with `body_path`. Prints copy-pasteable wave commands to stdout, in dependency order:

```
lucind-ai run --packet packets/apply-ledger.md
lucind-ai run --packet packets/apply-serve.md --packet packets/apply-run.md
```

No `waves.json`. A file with no binary reader is an unused indirection. Stdout **is** the wave plan; the orchestrator (or a human) runs those lines in order and stops on exit 1.

`body_path` keeps Goal/Context/criteria as Markdown the orchestrator already knows how to write. Split does not invent prompt text.

### Packet frontmatter `allowed_paths`

`packet.Parse` is not a YAML parser. It is a line loop that `strings.Cut`s on the first `:` and switches on `id` / `executor` / `routed_by` / `model`; unknown keys are dropped (`internal/packet/packet.go:59-76`). A nested YAML list would not parse.

Add one new key, JSON array on a single line — the smallest extension that preserves Parse's contract ("reflect frontmatter literally", `packet.go:39-43`):

```
---
id: apply-ledger
executor: agy
routed_by: schema and CRUD isolated from HTTP
allowed_paths: ["internal/ledger/"]
---
```

Split emits this line from the YAML list. The body still contains `## Allowed paths` as prompt prose for the executor (`packet-template.md:48-54`). The binary trusts only the frontmatter field.

## Decision 2 — `Packet.AllowedPaths` and its terminal consumer

**Choice**: `internal/packet.Packet` gains `AllowedPaths []string`. Omitted / empty means "not declared" → skip enforcement (today's packets keep working). Split always emits a non-empty list.

This is not a recorded-and-ignored field. Consumers:

| Consumer | What it does |
|----------|----------------|
| `packet.Parse` | Fills the field from the JSON array |
| `lucind-ai split` | Writes the field |
| `runDispatch` overlap check | Before `ExecuteBatch`, reject a batch whose declared lists overlap (same prefix rule as Decision 1). Lives in CLI / `packet.DisjointAllowedPaths`, **not** inside `ExecuteBatch` |
| **`decideStatus` (terminal)** | After a readable envelope, if `len(AllowedPaths) > 0` and the envelope mapped to `lane.Done`, list the worktree's actual changed paths; if any path is outside the list, return `lane.Deviated` instead of `Done` and a `lane_note` naming the offenders |

Do not trust `Envelope.FilesChanged` (`internal/result/result.go:107`, `omitempty`). `LaneStatus()` maps the self-reported `status` string 1:1 (`result.go:122-134`). That is the founding defect this project exists to catch (PRD §2): green criteria with a walked-past hard stop. Scope-vs-diff is the same class of check, done by the binary against git.

### Where the check runs

`Execute` today: dispatch → `decideStatus(deps, wt.Path, outcome)` (`internal/run/run.go:315`) → `SetStatus` (`:338`) → `runOneLane` `Observe` (`internal/run/batch.go:144`). Insert the check **inside `decideStatus`** (pass the packet and `deps.PrimaryRoot`) so a scope violation is never observed as `done` and never enters `Outcome.Integrate` (`internal/barrier/barrier.go:51-53`). PRD §6 step 7: non-`done` lanes never enter integration.

Do not put it in schema validation (`result.Read`). Schema changes would touch every existing envelope path. The check is git, not JSON.

### What "actual diff" means (no precedent — specified exactly)

`worktree.Create` runs `git worktree add -b lucind/<id> <path>` from `primaryRoot` (`internal/worktree/worktree.go:74-79`). The new branch's `HEAD` is primary's `HEAD` at Create. `runDispatch` calls `Integrate` only after `ExecuteBatch` returns (`cli.go:211,220`), so primary `HEAD` is still that same commit when `decideStatus` runs.

Base SHA: `git -C primaryRoot rev-parse HEAD` at check time. Changed paths = union of:

1. `git -C worktree diff --name-only --diff-filter=ACDMRT <base> HEAD` (committed on the lane)
2. `git -C worktree diff --name-only --diff-filter=ACDMRT` (unstaged)
3. `git -C worktree ls-files -o --exclude-standard` (untracked, respects gitignore)

A path is in-scope iff some `AllowedPaths` entry equals it or is a component-boundary prefix of it.

Always exclude `.lucind/` (binary writes `.lucind/result.schema.json` before dispatch, `run.go:434-449`; `.gitignore:2` ignores `.lucind/`). `ls-files --exclude-standard` already drops it; still filter the prefix so a force-add cannot sneak `done` through.

Git command failure → `lane.Blocked` with diagnosis, never `done`. Do not guess.

Override only `done` → `deviated`. Do not rewrite a `blocked`/`failed` envelope into `deviated`. Do not mutate `.lucind/result.json` on disk; the ledger status is the binary's verdict. `deviated` matches the schema's own meaning (`result.schema.json:15`: "touching a path outside allowed_paths") and the packet-template label (`packet-template.md:50`: touching anything else is a **deviation**).

Uncommitted in-scope files do not fail this check (criterion 2 "work is committed" stays a prompt convention, `result.go:122` still does not inspect `Commit`). Out-of-repo `external_changes` stay self-reported; git cannot see them. Residual, accepted.

## Decision 3 — Sequential `lucind-ai run` per wave

**Choice**: The orchestrator runs the commands `split` printed, one wave at a time. `ExecuteBatch` stays a flat concurrent batch (`internal/run/batch.go:66-88`: one goroutine per packet, `WaitGroup`, no DAG).

A wave **is** a flat batch: independent packets, disjoint paths, one barrier, then combine / resolve / bisect / promote.

Dependents must see promoted code. `Create` branches from current primary `HEAD` (`worktree.go:74`). `Promote` fast-forwards primary (`internal/integrate/integrate.go:118`). That is already the `lucind-ai run` loop: `ExecuteBatch` then `Integrate` (`cli.go:211-220`). A second `lucind-ai run` after exit 0 creates worktrees on the promoted tree. No new binary sequencer is required.

| | Sequential `lucind-ai run` per wave | Wave loop inside `internal/run` / `run --dag` |
|--|--------------------------------------|-----------------------------------------------|
| `ExecuteBatch` | Unchanged | Unchanged internally, wrapped in new loop |
| Combine / resolve / bisect | Unchanged, once per wave | Same, plus new stop/resume policy |
| Failed wave | Process exit 1 (`cli.go:244-251`); orchestrator stops | Binary must invent resume, nested run IDs |
| Ledger | One `run_id` per wave (inspectable) | One `run_id` for the whole DAG, or nested IDs |
| Aligns with | PRD §6 step 1 (orchestrator owns the split); SKILL.md:79 (orchestrator step, not built) | New orchestration inside the binary |

Rejected: teaching `ExecuteBatch` about waves. That would sequence `Create` after `Promote` inside the function that is documented as "lanes never cancel each other" and "validation happens before any side effect" (`batch.go:34-58`). The cost lands on the highest-uncertainty piece (scope-vs-diff) for no reuse gain.

Bisection (`internal/run/integrate.go:183`), resolve (`internal/resolve/resolve.go`, `MaxConflictLines = 400` at `:18`, `claude -p --model sonnet` at `:28`), combine (`internal/integrate/integrate.go:38-69`, `resolve.Resolve` at `:49`) stay unmodified. A same-wave path bug that escapes the overlap check still falls into merge + 400-line resolve + bisect, which already works.

## Decision 4 — Partial failure is mostly already there

Verified by reading the code. Do not invent a parallel reporting channel.

### Already sufficient

| Signal | Where | What the orchestrator learns |
|--------|-------|------------------------------|
| Per-lane status, worktree, summary | `printReport` (`cli.go:262-289`) | Dispatch outcome for each packet |
| Non-`done` banner | `cli.go:273-275` | Unmissable preserve-and-inspect |
| `released:` | `cli.go:229` | Barrier released (`batch.go:93-104` also appends `EventBarrierReleased`) |
| `integrate: attempted=… passed=… integrated=N reverted=N [reason=…]` | `cli.go:231-237` | Whether promote happened and why not |
| Exit 1 unless every lane is `done` **and** not in `Reverted` | `cli.go:244-251` | Stop; do not start the next wave |
| `Outcome.Integrate` vs `Preserve` | `barrier.go:22-28,49-57` | `done` vs every other terminal; `Preserve` never combines |
| `IntegrateReport.Integrated` / `Reverted` | `internal/run/integrate.go:14-20` | Who promoted, who bisect demoted |
| `revertLanes` → `lane.Blocked` + preserved worktree + `EventLaneNote` | `integrate.go:274-296` | Per-lane revert reason in the ledger |
| `EventLaneNote` "batch integrated: N" / "integration failed: …" | `integrate.go:166-170,291-296` | Run-scoped integrate summary |
| `Ledger.Events(runID)` | `internal/ledger/ledger.go:409` | Full append-only log for that wave |
| Event types | `ledger.go:358-365` | `run_started`, `lane_registered`, `lane_status_changed`, `lane_note`, `barrier_released`, `run_ended` — **no new type** |

A scope violation (Decision 2) becomes `deviated`, lands in `Preserve`, is printed by `printReport`, and exits 1. No new event type.

### What is missing (exactly one CLI gap)

`printReport` prints `batch.Lanes` **dispatch** status (`cli.go:226-228`). `revertLanes` later sets those rows to `blocked` (`integrate.go:279`). A bisected-out lane therefore prints `status: done` even though it will not be in primary. Exit 1 is correct (`cli.go:248-250`); the **lane IDs** in `IntegrateReport.Integrated` and `.Reverted` are never printed — only counts (`cli.go:231-237`).

`SKILL.md:133` also undersells this: it says exit 0 iff every lane is `done`, omitting the revert check that `cli.go:248` already performs.

**Fix (this change, apply phase):** print the ID lists:

```
integrate: attempted=true passed=true integrated=1 reverted=1
integrated_ids: apply-ledger
reverted_ids: apply-serve
reason: bisected out of batch
```

Do not add `lucind-ai ledger` or a JSON report. The orchestrator already reads stdout + exit code. Filling the ID list makes `IntegrateReport`'s existing fields visible. `Ledger.Events` remains the durable copy; there is still no CLI to dump it, and this change does not add one — stdout is the orchestrator-facing surface.

## Decision 5 — Rollback

| Layer | If we ship it | How to undo |
|-------|----------------|-------------|
| `Packet.AllowedPaths` omitted | Skip overlap check and diff check | Existing packets unchanged during rollout and after revert |
| `lucind-ai split` unused | Nobody has to run it | Delete the subcommand |
| `apply-dag.yaml` absent | Apply stays "one big packet" or hand-split as today | Delete the file |
| `decideStatus` check | Only runs when the field is non-empty | Revert the `run.go` hunk |
| Ledger schema | **No migration.** Do not store `allowed_paths` on `lanes` (no reader) | Nothing to roll back in SQLite |
| Combine / resolve / bisect | Untouched | N/A |

Rollback boundary for the apply PR: `internal/packet`, `internal/run/run.go` (check only), new `internal/dag` (or `internal/split`), `cmd/lucind-ai/cli.go` (`split` + overlap + ID print), tests next to them, plus later SKILL/template prose. Revert that commit. Primary history already contains whatever waves promoted; reverting the binary does not rewrite those merges.

## Data flow

```
orchestrator authors tasks.md (human) + apply-dag.yaml (declared DAG)
        │
        ▼
lucind-ai split --dag … --out packets/
        │  validates cycle + per-wave disjoint paths
        │  writes packets/<id>.md (frontmatter allowed_paths JSON array + body)
        │  prints one `lucind-ai run --packet …` line per wave
        ▼
for each printed line, stopping on exit 1:
        lucind-ai run --packet p1 [--packet p2 …]
              Parse (AllowedPaths)
              DisjointAllowedPaths (batch)
              ExecuteBatch          ← unchanged
                Create from primary HEAD
                Execute → decideStatus
                  envelope done? + AllowedPaths set?
                    git diff vs primary HEAD  → else Deviated
                Observe → barrier
              Integrate             ← unchanged
                Combine + resolve(≤400) + Check + bisect + Promote
              stdout: per-lane + integrated_ids/reverted_ids
              exit 0 only if all done and none reverted
        ▼
next wave's Create sees promoted primary HEAD
```

## File changes (apply phase, not this packet)

| File | Action | Why |
|------|--------|-----|
| `internal/packet/packet.go` | Modify | `AllowedPaths []string`; parse JSON array |
| `internal/packet/disjoint.go` (or same pkg) | Create | Prefix-overlap check; called from CLI |
| `internal/run/run.go` | Modify | `decideStatus` takes packet + primary root; git scope check |
| `internal/dag/` | Create | Parse `apply-dag.yaml`, Kahn waves, emit packets |
| `cmd/lucind-ai/cli.go` | Modify | `split` subcommand; overlap before `ExecuteBatch`; print integrate IDs |
| `plugin/…/SKILL.md`, packet templates | Modify | Document frontmatter key + split + wave loop (later; not this file) |

`internal/run/batch.go`, `internal/run/integrate.go` (`bisect`), `internal/integrate/integrate.go`, `internal/resolve/resolve.go`: **no edits**.

## Indirections and terminal consumers

| New name | Terminal consumer (reads it, not another mention) |
|----------|-----------------------------------------------------|
| `apply-dag.yaml` | `lucind-ai split` |
| `body_path` | `lucind-ai split` (packet body bytes) |
| `depends_on` | `lucind-ai split` (Kahn / wave grouping) |
| `allowed_paths` YAML list | `lucind-ai split` → frontmatter JSON array |
| `Packet.AllowedPaths` | `decideStatus` git check; `DisjointAllowedPaths` in `runDispatch` |
| `lucind-ai split` stdout lines | Orchestrator / human as `lucind-ai run` argv |
| `integrated_ids` / `reverted_ids` on stdout | Orchestrator: who to inspect, whether to start the next wave |

## Testing strategy (apply phase)

| Layer | RED proof |
|-------|-----------|
| Parse | JSON array fills `AllowedPaths`; omitted stays nil; invalid JSON is a parse error |
| Disjoint | `internal/foo/` vs `internal/foo/bar.go` overlap; `internal/foo/` vs `internal/bar/` pass |
| DAG | Cycle rejected; overlap without edge rejected; overlap with `depends_on` → two waves |
| Scope check | Envelope `done` + extra tracked file → `deviated`, not in `Outcome.Integrate`; in-scope only → `done`; git failure → `blocked` |
| CLI | Overlapping `--packet` pair fails before worktrees; integrate stdout contains reverted id |
| Regression | Existing packet without the key still `done` through `decideStatus` |

Stdlib `testing` only. Fake `WorktreeFS` + a temp git repo for the scope check (the one place git is the spec).

## Out of scope

- Read-only / empty-`AllowedPaths` exception (`read-only-packet-dispatch`).
- Verify-phase dual dispatch (`verify-dual-dispatch`).
- Redesign of bisection, conflict resolution, or combine.
- Glob `allowed_paths`, enforcing `external_changes`, requiring `Envelope.Commit`.
- Ledger column or new event type.
- `lucind-ai run --dag` / in-process wave loop.
- Inferring the DAG from `tasks.md` without `apply-dag.yaml`.
