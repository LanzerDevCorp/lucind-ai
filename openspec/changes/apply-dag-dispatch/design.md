# Design: Apply-Phase DAG Dispatch

Split an SDD change's `tasks.md` into independent packets with non-overlapping `allowed_paths`, dispatch each dependency wave as one `lucind-ai run --packet ... --packet ...`, and let the existing combine / 400-line resolver / bisection / promote path integrate that wave before the next one starts. The novel piece is binary enforcement of declared scope against a lane's actual git diff — not a new integrator, and not a new orchestration engine.

Both independently-drafted designs (agy, cursor-agent) agree strongly on decisions 3 and 5 below (sequential dispatch, rollback), which is a good signal those are right. They disagree sharply on decisions 1 and 4, and on one implementation detail inside decision 2. Each disagreement is resolved explicitly below, with the rejected side kept as a recorded alternative, not silently dropped.

## Recommendations at a glance

| # | Question | Recommendation | Source |
|---|---|---|---|
| 1 | How are paths and order declared? | New sidecar `openspec/changes/<id>/apply-dag.yaml`, not `tasks.md` prose. New `lucind-ai split` subcommand parses it. | cursor-agent (agy's tasks.md-embedded-YAML rejected — see below) |
| 2 | `Packet.AllowedPaths`? | Yes, `[]string`. Two terminal consumers: an upfront batch-disjointness check, and a post-execution git-diff scope check in `decideStatus` that demotes `Done` → `Blocked`/`Deviated` on a violation. | Both agree on the field and on two consumers; canonical adopts cursor-agent's base-SHA diff computation over agy's `HEAD~1` (see below — the latter is a real bug). |
| 3 | Who drives waves? | The orchestrator issues one `lucind-ai run` per wave. No wave/DAG logic added inside `ExecuteBatch`. | Both agree, independently, in detail. |
| 4 | Partial failure surfacing? | `IntegrateReport` + the ledger already record everything needed. The one real gap: `printReport` never prints the `Integrated`/`Reverted` lane **IDs**, only counts. Fix: print the ID lists on stdout. | cursor-agent (agy's `--json` flag / `.lucind/runs/<id>.json` rejected — see below) |
| 5 | Rollback? | Field omitted = today's behavior, exactly. No ledger migration. Revert the apply commits. | Both agree. |

## Decision 1 — DAG artifact and exact format

**Choice**: a new YAML sidecar file, `openspec/changes/<change-id>/apply-dag.yaml`. `tasks.md` stays exactly what `sdd-tasks` already produces today — an unstructured human checklist — and is **not** the parse source. The orchestrator authors the YAML (task-to-wave split remains orchestrator judgment per PRD §6 step 1); a new `lucind-ai split` subcommand is the mechanical consumer.

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

The splitter enforces: unique `id`; non-empty `allowed_paths` (an empty list is the sibling `read-only-packet-dispatch` change's concern, not this one's); acyclic `depends_on` (Kahn's algorithm); same-wave paths pairwise disjoint (component-boundary prefix match, no globs in v1 — `internal/ledger` matches `internal/ledger/foo.go`, `internal/led` does not); cross-wave overlap is allowed, that's what the ordering is for; an overlap with no `depends_on` edge between the two packets is rejected — the author must add a dependency or shrink scope.

`lucind-ai split --dag openspec/changes/<id>/apply-dag.yaml --out openspec/changes/<id>/packets/` writes one packet file per node by concatenating generated frontmatter (including the JSON-array `allowed_paths:` line — see Decision 2) with `body_path`'s Markdown, and prints copy-pasteable wave commands to stdout in dependency order:

```
lucind-ai run --packet packets/apply-ledger.md
lucind-ai run --packet packets/apply-serve.md --packet packets/apply-run.md
```

No separate `waves.json` — stdout **is** the wave plan; a file nothing reads back is an unused indirection. `body_path` keeps Goal/Context/criteria as Markdown the orchestrator already knows how to write; `split` does not invent prompt text.

### Rejected: embedding wave/task structure directly in `tasks.md`

One draft (agy) proposed the opposite: no sidecar at all — `tasks.md` itself grows `## Wave <N>` H2 headings and per-task H3 sections, each carrying a fenced YAML metadata block (`id`, `executor`, `allowed_paths`, `depends_on`) that a mechanical splitter parses directly out of the checklist document.

**Rejected**, for a scope reason, not a taste reason: `tasks.md` as `sdd-tasks` produces it today (verified against the real example at `openspec/changes/approvals-web-ui/tasks.md`) is informal checkbox prose with no such structure. Making it parseable the way agy's draft requires is not a small addition to *this* change — it is a redesign of the `sdd-tasks` phase's output contract, project-wide, for every future SDD change whether or not it ever uses DAG dispatch. That is a materially larger and different change than "give apply a DAG dispatch path." A sidecar file is purely additive: `sdd-tasks` keeps producing exactly what it produces today, and `apply-dag.yaml` is optional, authored only when DAG dispatch is actually wanted for that change. Agy's own stated rejection reason for the sidecar option — "disconnects task documentation from task metadata, requiring dual updates and risking drift" — is a real cost, but a smaller one than rewriting an existing, working phase contract; it can be revisited if drift turns out to be a practical problem.

## Decision 2 — `Packet.AllowedPaths` and its terminal consumers

**Choice**: `internal/packet.Packet` gains `AllowedPaths []string`. Omitted or empty means "not declared" — today's packets (including every propose/design/specs/tasks packet dispatched so far) keep working unmodified. `split` always emits a non-empty list. This is not a recorded-and-ignored field; both drafts independently arrived at the same two consumers:

1. **Upfront batch-disjointness check**, before any worktree is created: reject a batch whose declared `AllowedPaths` overlap, using the same component-boundary prefix rule as Decision 1's split-time check. Runs at the CLI/dispatch layer (`packet.DisjointAllowedPaths`, called from `runDispatch` before `ExecuteBatch`) — not inside `ExecuteBatch` itself, keeping that function's existing contract ("lanes never cancel each other," worktree creation is the first side effect) unchanged.
2. **Post-execution git-diff scope check**, inside `decideStatus`, after a schema-valid envelope maps to `lane.Done`: if the lane's actual worktree diff touches anything outside `AllowedPaths`, the lane is demoted — `lane.Deviated`, matching both the envelope schema's existing meaning for that word (`result.schema.json:15`: "touching a path outside allowed_paths") and the packet template's own label ("touching anything else is a **deviation**," `packet-template.md:50`) — with a `lane_note` naming the offending paths. A `blocked`/`failed` envelope is never rewritten into `deviated`; only a `done` verdict is subject to override.

Frontmatter shape (smallest extension that preserves `Parse`'s "reflect literally" contract, `packet.go:39-43`): a single-line JSON array, since `Parse` is a line-oriented `strings.Cut`-on-`:` loop, not a YAML parser, and a nested YAML list would not parse as-is.

```
---
id: apply-ledger
executor: agy
routed_by: schema and CRUD isolated from HTTP
allowed_paths: ["internal/ledger/"]
---
```

### What "actual diff" means, precisely — and a real bug this comparison caught

Both drafts agreed the check belongs in `decideStatus`. They disagreed on *how* to compute "what the lane actually touched," and one version is wrong.

**Rejected: `git diff --name-only HEAD~1`** (agy's draft). This assumes the lane's branch carries exactly one commit. It silently produces the wrong answer — or a git error — for a lane with zero commits (nothing to diff; `HEAD~1` may not resolve at all) or with two or more commits (only the last commit's diff is inspected; earlier commits' changes are invisible to the check, so a genuinely out-of-scope file touched in an earlier commit would pass undetected). Packets are not guaranteed to produce exactly one commit — the packet template asks for "a conventional commit," not "exactly one."

**Chosen: base-SHA plus a three-way union** (cursor-agent's draft). Capture the primary repository's `HEAD` at check time (`git -C primaryRoot rev-parse HEAD`) — this is valid because `worktree.Create` births the lane branch from that exact commit (`worktree.go:74-79`), and `Integrate` only runs after `ExecuteBatch` returns, so primary's `HEAD` has not moved while the batch was in flight. Changed paths are the union of:

1. `git -C worktree diff --name-only --diff-filter=ACDMRT <base> HEAD` — everything committed on the lane, regardless of commit count.
2. `git -C worktree diff --name-only --diff-filter=ACDMRT` — unstaged changes.
3. `git -C worktree ls-files -o --exclude-standard` — untracked files, respecting `.gitignore`.

A path is in-scope iff some `AllowedPaths` entry equals it or is a component-boundary prefix of it. `.lucind/` is always excluded from the comparison (it's gitignored, and `ls-files --exclude-standard` already drops it, but the check also filters the prefix explicitly so a forced add cannot slip a `.lucind/` file through as evidence of a scope violation, or absence of one). A git-command failure resolves to `lane.Blocked` with a diagnosis, never a guessed `done`.

One residual gap, accepted by both drafts: `external_changes` (paths outside the repository, per the packet template) stay self-reported — git cannot see outside its own worktree, so this check cannot verify them. That gap is out of scope for this change.

## Decision 3 — sequential `lucind-ai run` per wave (strong agreement)

**Choice**: the orchestrator runs the wave commands `split` printed, one at a time, stopping on any non-zero exit. `ExecuteBatch` stays exactly the flat concurrent batch it is today (`internal/run/batch.go:66-89`: one goroutine per packet, one `WaitGroup`, no DAG concept added). A wave *is* a flat batch: independent packets, disjoint declared paths, one barrier, then combine/resolve/bisect/promote — nothing new is required to make that true.

Both drafts reached this conclusion independently and for the same reasons: `worktree.Create` branches from primary's current `HEAD` (`worktree.go:74`), and `Integrate`'s promotion fast-forwards primary (`internal/integrate/integrate.go:118`) — so a second `lucind-ai run` issued after the first exits 0 automatically creates its worktrees on the already-promoted tree. No new binary sequencer is required for wave $N{+}1$ to see wave $N$'s integrated code. Teaching `ExecuteBatch` itself about waves was considered and rejected by both: it would put sequencing (`Create` after a prior `Promote`) inside a function explicitly documented as doing all its validation before any side effect, for no reuse benefit — the orchestrator loop already gets that ordering for free.

| | Sequential `lucind-ai run` per wave (chosen) | Wave loop inside `internal/run` |
|---|---|---|
| `ExecuteBatch` | Unchanged | Unchanged internally, wrapped in a new loop |
| Combine/resolve/bisect | Unchanged, once per wave | Same, plus a new stop/resume policy to invent |
| Failed wave | Process exits 1; orchestrator stops (already how every dispatch in this session has been read) | Binary must invent its own resume/nested-run-ID semantics |
| Ledger | One `run_id` per wave, individually inspectable | One `run_id` for the whole DAG, or nested IDs — new concept |
| Alignment | PRD §6 step 1 (orchestrator owns the split) and `SKILL.md:79` (this was always named as an orchestrator step, not a binary one) | New orchestration policy moved into the binary |

## Decision 4 — partial-failure surfacing: one small CLI gap, not a new format

**Choice**: verified by reading the code — do not invent a parallel reporting channel. `IntegrateReport` (`integrate.go:14-21`: `Attempted`, `Passed`, `Integrated`, `Reverted`, `Reason`), `barrier.Outcome`'s `Integrate`/`Preserve` split, `EventLaneNote` written by `revertLanes` (`integrate.go:274-296`), and the CLI's existing per-lane report plus exit code already carry everything the orchestrator needs — this session's own dispatches this far were read exactly this way, successfully. The six existing ledger event types are sufficient; nothing new is required there.

**The one real gap**: `printReport` prints each lane's *dispatch* status (`cli.go:226-228`), but `revertLanes` can later change a lane's true fate during integration (`integrate.go:279`) — so a bisected-out lane prints `status: done` even though it will never land on primary. The exit code is still correct (non-zero), but `IntegrateReport.Integrated`/`.Reverted` — which *do* hold the right lane IDs — are never printed; only their counts are (`cli.go:231-237`, confirmed by this session's own dispatch output: `integrate: attempted=true passed=false integrated=0 reverted=7 reason=...` with no ID list). `SKILL.md:133` also currently undersells the exit-code contract by omitting the revert check `cli.go:248` already performs.

**Fix** (apply phase, not this design document): print the ID lists alongside the existing counts —

```
integrate: attempted=true passed=true integrated=1 reverted=1
integrated_ids: apply-ledger
reverted_ids: apply-serve
reason: bisected out of batch
```

### Rejected: a new `--json` flag or `.lucind/runs/<run_id>.json`

One draft (agy) proposed a structured machine-readable output mode as the fix. Rejected as larger than the identified gap requires: the actual missing information is two lists of strings, already computed and held in memory (`IntegrateReport.Integrated`/`.Reverted`) — printing them closes the gap completely. A new output format is a bigger, separately-justified surface (parsing mode, versioning, a second code path to keep in sync with the text report) that this change does not need in order to make apply's partial failures fully inspectable. It can be proposed later on its own merits if plain-text stdout parsing turns out to be insufficient in practice — nothing in this design forecloses that.

## Decision 5 — rollback (agreement)

| Layer | If shipped | How to undo |
|---|---|---|
| `Packet.AllowedPaths` omitted | Skips both the overlap check and the diff check | Existing packets unchanged, during rollout and after revert |
| `lucind-ai split` unused | Nobody has to run it | Delete the subcommand |
| `apply-dag.yaml` absent | Apply stays "one big packet" or hand-split, as today | Delete the file |
| `decideStatus` scope check | Only runs when `AllowedPaths` is non-empty | Revert the `run.go` hunk |
| Ledger schema | **No migration.** `allowed_paths` is not stored on `lanes` (nothing reads it back from SQLite) | Nothing to roll back |
| Combine/resolve/bisect | Untouched | N/A |

Rollback boundary for the eventual apply PR: `internal/packet`, `internal/run/run.go` (the new check only — `decideStatus`'s existing envelope logic is untouched), a new `internal/dag` (or `internal/split`) package, `cmd/lucind-ai/cli.go` (`split` subcommand, overlap check, ID printing), their tests, plus later `SKILL.md`/template prose. Revert that commit set. Primary history already contains whatever waves were promoted before a rollback decision; reverting the binary does not rewrite those merges.

## Data flow

```
orchestrator authors tasks.md (human, unchanged) + apply-dag.yaml (declared DAG)
        |
        v
lucind-ai split --dag ... --out packets/
        |  validates: unique id, cycle-free, per-wave disjoint paths
        |  writes packets/<id>.md (frontmatter allowed_paths JSON array + body)
        |  prints one `lucind-ai run --packet ...` line per wave
        v
for each printed line, stopping on exit 1:
        lucind-ai run --packet p1 [--packet p2 ...]
              Parse (AllowedPaths)
              DisjointAllowedPaths (CLI, before ExecuteBatch)
              ExecuteBatch                 <- unchanged
                Create from primary HEAD
                Execute -> decideStatus
                  envelope done? + AllowedPaths set?
                    base-SHA diff union vs declared paths -> else Deviated
                Observe -> barrier
              Integrate                    <- unchanged
                Combine + resolve(<=400) + Check + bisect + Promote
              stdout: per-lane report + integrated_ids/reverted_ids
              exit 0 only if every lane is done and none reverted
        v
next wave's Create sees the promoted primary HEAD
```

## File changes (apply phase — not this design document)

| File | Action |
|---|---|
| `internal/packet/packet.go` | Add `AllowedPaths []string`; parse the single-line JSON array. |
| `internal/packet/packet_test.go` | Cases: JSON array fills the field; omitted stays nil/empty; invalid JSON is a parse error. |
| `internal/packet/disjoint.go` (or similar) | New `DisjointAllowedPaths` prefix-overlap check, called from the CLI layer. |
| `internal/run/run.go` | `decideStatus` gains the base-SHA diff-union scope check, gated on `len(AllowedPaths) > 0`; envelope-interpretation logic otherwise unchanged. |
| `internal/run/run_test.go` | In-scope-only → `Done`; out-of-scope tracked file → `Deviated`; git failure → `Blocked`; existing no-`AllowedPaths` packets still reach `Done` unmodified (regression coverage). |
| `internal/dag/` (new) | Parse `apply-dag.yaml`; Kahn's-algorithm wave grouping; emit packet files. |
| `cmd/lucind-ai/cli.go` | New `split` subcommand; overlap check before `ExecuteBatch`; print `integrated_ids`/`reverted_ids`. |
| `plugin/claude-code/skills/lucind-ai/SKILL.md`, packet templates | Document the sidecar format, `split`, the wave-dispatch loop, and the `allowed_paths` frontmatter key (implementation-phase prose, not this file). |

`internal/run/batch.go` (beyond the pre-`ExecuteBatch` overlap check's call site), `internal/run/integrate.go` (`bisect`), `internal/integrate/integrate.go`, and `internal/resolve/resolve.go` all get **no edits** — reused exactly as they are, per both drafts' explicit agreement.

## Testing strategy

| Layer | RED | GREEN |
|---|---|---|
| `packet.Parse` | Invalid JSON in `allowed_paths` | JSON array fills the field; omitted stays empty |
| Disjoint check | `internal/foo/` vs. `internal/foo/bar.go` overlap; overlap with no `depends_on` edge | `internal/foo/` vs. `internal/bar/` pass; overlap *with* an edge splits into two waves |
| DAG split | Cycle rejected (Kahn) | Valid DAG produces correctly-ordered wave commands |
| Scope check (`decideStatus`) | Envelope `done` + an untracked/out-of-scope file → `Deviated`, excluded from `Outcome.Integrate`; a lane with 0 or 2+ commits touching only in-scope paths still evaluates correctly (regression for the `HEAD~1` bug) | In-scope-only diff → `Done`; git-inspection failure → `Blocked` |
| CLI | Overlapping `--packet` pair fails before any worktree is created | Integrate stdout contains the reverted lane's ID, not just a count |
| Regression | — | An existing packet that omits `allowed_paths` still reaches `Done` through the unmodified path |

Stdlib `testing` only; a fake `WorktreeFS`/`Deps` plus a real temporary git repo for the scope-check tests, since git behavior is the actual specification there.

## Threat matrix

| Boundary | Applicability | Reason |
|---|---|---|
| Cross-lane write contention | Applicable | Upfront disjointness check rejects overlapping batches before any worktree exists. |
| Silent scope creep (a lane modifying an unlisted file) | Applicable | The base-SHA diff-union check in `decideStatus` catches it regardless of commit count, unlike a naive last-commit-only diff. |
| Stale base commits across waves | Applicable | Waves run strictly sequentially; wave $N{+}1$'s worktrees only ever branch from primary after wave $N$ has promoted. |
| Cascading merge conflicts on partial failure | Applicable | Unmodified bisection isolates the failing lane; the run exits non-zero and halts before the next wave dispatches. |
| Git repository selection | Applicable | New git calls are `git -C worktreePath`/`git -C primaryRoot` only — the same two roots `Execute` already owns. |
| Ledger/schema corruption | N/A | Zero SQLite schema changes; existing event types and columns are sufficient. |

## Out of scope (owned by sibling changes, or explicitly deferred)

Read-only / empty-`AllowedPaths` handling — `read-only-packet-dispatch`. Verify-phase dual dispatch — `verify-dual-dispatch`. Redesign of bisection, conflict resolution, or combine. Glob support in `allowed_paths`, enforcing `external_changes`, requiring `Envelope.Commit`. Any new ledger column or event type. An in-process `lucind-ai run --dag` wave loop. Inferring the DAG from `tasks.md` prose without `apply-dag.yaml`.
