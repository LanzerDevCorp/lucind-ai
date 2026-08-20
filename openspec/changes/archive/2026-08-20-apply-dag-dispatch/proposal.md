# Proposal: Apply-Phase DAG Dispatch

Replace `sdd-apply`'s in-process Read/Edit/Write with real `lucind-ai run` dispatch: split an SDD change's `tasks.md` into packets whose `allowed_paths` do not overlap, run independent packets in parallel, sequence dependent ones into waves, and reuse the existing combine / 400-line conflict resolver / bisection path unmodified.

**This is the highest-uncertainty piece of the three sibling dual-executor-dispatch changes: there is zero code today that checks a packet's declared scope against its actual diff.** Design must treat that gap as novel engineering, not a small add-on to bisection.

## Intent

`lucind-ai run` can already execute a flat concurrent batch of packets and integrate the `done` lanes (`docs/prd.md` §6 steps 6-8, confirmed working). Apply does not use that path yet. `plugin/claude-code/skills/lucind-ai/SKILL.md:79` states the target — split `tasks.md` into a DAG of packets, dispatch via `lucind-ai run` — and names the blocker: the orchestrator step that produces non-overlapping `allowed_paths` and dependency order is **not built**. Until it exists, `apply` stays a Claude Code sub-agent (`sdd-apply`) writing files directly in its own session.

This change builds that step and the binary contracts it needs, so apply packets become real lanes with real worktrees, real envelopes, and real integration — the same execution model `explore`/`propose`/`design` already use or are gaining via the sibling `read-only-packet-dispatch` change.

## Scope

### What changes

- An orchestrator convention (plus whatever small in-repo helpers design chooses) that turns `tasks.md` into packets with non-overlapping `allowed_paths`, grouped into dependency waves $W_1, W_2, \dots, W_k$: within one wave, every packet is independent and has mutually exclusive `allowed_paths`; wave $N$ must integrate before wave $N+1$ dispatches.
- Binary support sufficient for that convention: `allowed_paths` must become something the program can see, not only prompt prose. Today `packet.Packet` (`internal/packet/packet.go:29-47`) has `ID`, `Executor`, `RoutedBy`, `Model`, `Body` — no `AllowedPaths`. `Parse` (`internal/packet/packet.go:66-75`) only copies `id`/`executor`/`routed_by`/`model`; any other frontmatter key is silently dropped.
- A design-phase decision on **declared scope vs. actual diff**: today the binary never makes that comparison. `allowed_paths` lives as Markdown prose (`packet-template.md:48-54`) and a self-policed hard stop (`packet-template.md:79`). `decideStatus` (`internal/run/run.go:407-431`) maps `result.json` through `Envelope.LaneStatus` (`internal/result/result.go:122-135`) and never inspects the worktree diff; `FilesChanged` is optional (`result.go:107`) and, today, unverified.
- Wave-based dispatch orchestration: dispatch a wave via `lucind-ai run --packet ...` (repeatable flag), let the binary run it concurrently in isolated worktrees, join at the barrier, and run `Integrate` (`internal/run/integrate.go:30-60`).
- Surfacing a wave's partial failure back to the orchestrator. Prefer the existing `IntegrateReport` (`integrate.go:14-21`: `Attempted`, `Passed`, `Integrated`, `Reverted`, `Reason`) plus `barrier.Outcome` (`internal/barrier/barrier.go:21-28`: `Integrate` vs. `Preserve`) and ledger notes (`EventLaneNote` written by `revertLanes`, `integrate.go:274-297`). Design must verify this is sufficient *before* adding any new ledger event type — the six existing types (`internal/ledger/ledger.go:358-365`) may already cover it.
- `plugin/claude-code/skills/lucind-ai/SKILL.md:79`'s apply row moves from "not built" to the implemented dispatch path.

### What stays untouched

- Bisection (`internal/run/integrate.go:183`, `bisect`), conflict resolution (`internal/resolve/resolve.go`: `MaxConflictLines = 400` at line 18, `RealInvoker` is `claude -p --model sonnet` at line 28), and combine (`internal/integrate/integrate.go:34-70`: `git merge --no-ff` then `resolve.Resolve`; `Check` at line 79). Confirmed current and working against PRD §6 steps 6-8. **This change calls them; it does not replace or re-architect them.**
- Flat `ExecuteBatch` (`internal/run/batch.go:66-89`: one goroutine per packet, `sync.WaitGroup`, no wave/DAG concept). Design may wrap it with sequential `lucind-ai run` calls per wave rather than rewriting it — see Approach.
- The 6-value `lane.Status` enum (`internal/lane/status.go:10-18`) and the rule that a barrier releases only once every lane in its batch reaches a terminal state (`batch.go:25-27`, `internal/barrier/barrier.go:22-31`).
- The append-only SQLite ledger and its existing event types (`internal/ledger/ledger.go:348-365`).
- Worktree isolation guarantees (`internal/worktree/worktree.go:20-45`).
- The sibling changes `read-only-packet-dispatch` and `verify-dual-dispatch`. Apply-DAG lanes always write files — there is no read-only overlap and no dependency on either sibling.

## Non-goals

- **No read-only-packet design.** The `read_only` flag, its criterion-2 exception, and explore-phase dispatch belong to `read-only-packet-dispatch`.
- **No verify dual-dispatch design.** Mechanical `lucind-checks.sh` vs. qualitative dual `agy`/`cursor-agent` judgment belongs to `verify-dual-dispatch`.
- **No redesign of bisection, conflict resolution, or combine** — reused exactly as they are.
- No `design.md`/`spec.md`/`tasks.md` in this phase; no edits to this change's own `state.yaml`.
- No changes to `openspec/changes/approvals-web-ui/` or `openspec/changes/read-only-packet-dispatch/`.

## Approach

Keep `ExecuteBatch` as the per-wave execution engine; put DAG ordering *outside* it via sequential `lucind-ai run` invocations, one per wave, unless the design phase proves an in-binary scheduler is genuinely smaller than that wrapper. Parse `allowed_paths` into a real `Packet` field so any enforcement design chooses is not a Markdown scrape. Call `Combine` → `resolve.Resolve` → `bisect` unchanged, per wave. Treat scope-vs-diff as the risky slice of this change and TDD it first, with an explicit declare-only (no binary enforcement) fallback if design rejects full enforcement as out of scope for the first cut.

Concretely, per wave $W_i$: dispatch → binary runs the wave concurrently in isolated worktrees → barrier join → `Integrate`. If `IntegrateReport.Passed == true`, the orchestrator advances to $W_{i+1}$; if any lane is `blocked`/reverted, the run halts for human review or replanning before further waves dispatch — design must decide (open question 4) whether that halt is whole-DAG or only the failed lane's dependents.

## Open design questions (left to `design`; this proposal does not pick)

1. **Exact DAG/wave representation for `tasks.md`.** Explicit Markdown wave sections (`### Wave 1`)? Per-task `depends_on:`/`allowed_paths:` annotations? A sidecar file next to `tasks.md`? Whatever it is must be writable by `sdd-tasks` and readable by the splitter without the binary parsing Markdown heuristics it does not already own.
2. **Whether/how `allowed_paths` is enforced by the binary vs. only declared.** Options range from (a) prose-only, still self-policed — fails the non-overlap *guarantee*; (b) parse into `Packet` and compare `git diff --name-only`/porcelain against the declared set before accepting `lane.Done`; (c) additionally reject overlapping declarations at split time, before dispatch. **(b)/(c) have no precedent in this codebase.** Enforcing inside `decideStatus` or `result.Read` would touch every existing packet path, including the working propose/design dual-dispatch flow — backward compatibility is a first-class design constraint here, not an afterthought.
3. Sequential `lucind-ai run` per wave (reuses `ExecuteBatch` as-is) vs. in-binary wave scheduling inside `ExecuteBatch` itself.
4. After a wave produces a `blocked`/`deviated`/`failed` lane: skip only that lane's downstream dependents, or halt the remaining DAG entirely? `barrier.Evaluate` (`internal/barrier/barrier.go:36-59`) already splits `Integrate` vs. `Preserve` *within* one batch; cross-wave policy is new and undecided.
5. New ledger event types vs. reusing `IntegrateReport` + existing `EventLaneNote`/`EventBarrierReleased` (`batch.go:99-104`). Do not add machinery the current report already carries — verify the gap first.

## Impact on the existing `sdd-apply` flow

`sdd-apply` today is a Claude Code sub-agent that receives a slice of `tasks.md` and implements it by reading spec/design and writing code directly in its own session. After this change, apply becomes:

1. Split `tasks.md` into packets (orchestrator judgment, still prose per `docs/prd.md` §6 step 1, unless design adds a mechanical helper).
2. `lucind-ai run` one wave at a time (or one DAG run, if design puts wave sequencing inside the binary).
3. Each packet is a real lane: worktree, envelope, barrier, then combine/resolve/bisect for that wave's `done` lanes.
4. The orchestrator reads `IntegrateReport`/preserved lanes and either dispatches the next wave or stops with questions.

`sdd-apply` stops being the writer of apply diffs for this project. Whatever workload-forecast / chained-PR logic currently lives inside that sub-agent must be restated on the packets or the splitter — design decides where; this proposal only notes it cannot silently vanish. Practically, this also removes a category of failure mode: context pollution from uncommitted mid-session edits breaking adjacent files (`docs/prd.md:167-171`) is structurally impossible when each task runs in its own pristine worktree, and merge conflicts/build regressions are isolated automatically via bisection instead of corrupting a whole batch.

## Affected areas

| Area | Impact | Description |
|---|---|---|
| `internal/packet` | Modified | Add `AllowedPaths` (or equivalent); `Parse` currently drops unknown frontmatter keys. |
| `internal/run/run.go` (`decideStatus`) | Modified only if binary enforcement is chosen | The only place a lane currently becomes `Done`. |
| `internal/result` | Maybe | Whether the schema/envelope changes, or `files_changed` stays trusted-but-unverified. |
| `internal/run/batch.go` | Untouched, or wrapped | Flat batch stays as-is; waves likely sit above it, not inside it. |
| `internal/run/integrate.go`, `internal/integrate`, `internal/resolve` | Reused, not modified | No redesign — see What stays untouched. |
| `internal/ledger` | Verify first | Six existing event types; add only if `IntegrateReport` proves insufficient. |
| `plugin/claude-code/skills/lucind-ai/SKILL.md` | Modified | Apply row moves from target to built. |
| Packet templates | Maybe | Only if `allowed_paths` moves into frontmatter. |

## Capabilities

### New

- `apply-dag-dispatch`: orchestrator capability to decompose `tasks.md` into dependency waves of packets with non-overlapping `allowed_paths`, invoking `lucind-ai run` per wave and handling wave progression.

### Modified

- `sdd-apply`: shifts from executing direct filesystem edits in the primary repository to authoring packet files, driving batch executions, and handling returned integration reports.

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Scope-vs-diff enforcement is genuinely greenfield engineering. | High | Dedicated design-phase risk analysis; do not assume bisection machinery covers it — it doesn't. |
| Enforcement added to `result`/`decideStatus` regresses the working propose/design dual-dispatch flow. | High if schema-strict | Must stay backward-compatible: packets that omit the new field keep today's exact path. |
| DAG-encoding bikeshedding vs. what `sdd-tasks` can realistically emit. | Medium | Pick the smallest representation `sdd-tasks` can mechanically produce. |
| Overlapping `allowed_paths` declared but never caught (declare-only path). | High if declare-only is chosen | A split-time overlap check is cheaper than a post-hoc diff check; still not equivalent to real enforcement. |
| Partial-wave-failure handling needs new ledger machinery. | Low | Read `IntegrateReport` + `Preserve` + `EventLaneNote` before adding any new event type. |
| Field-shape drift if the sibling `read-only-packet-dispatch` change lands a `read_only`-adjacent field with different conventions. | Medium | This change does not wait on that sibling; design should note the reuse-if-present shape rather than diverge silently. |

## Rollback plan

This proposal file is scoping only — reverting it changes nothing at runtime. For the eventual implementation: revert the packet-field addition, its parsing, the optional `decideStatus` enforcement hook, the `SKILL.md` apply-row text, and any template edits. **Explicitly leave `internal/run/integrate.go`, `internal/resolve/resolve.go`, and `internal/integrate/integrate.go` out of that revert** — they are reused, not changed, by this work. A declare-only milestone (no binary enforcement) can ship first and later gain enforcement without unwinding dispatch itself.

## Dependencies

- Working combine/resolve/bisect (PRD §6 steps 6-8) — already on `main`.
- **None** on `read-only-packet-dispatch` or `verify-dual-dispatch`.
- `docs/prd.md` §6 step 1 remains orchestrator judgment for the task-to-packet split itself, unless design adds mechanical help.

## Success criteria

- [ ] An SDD apply is a DAG of `lucind-ai run` packets, not `sdd-apply` writing the diff itself.
- [ ] Packets within one wave have non-overlapping declared `allowed_paths`.
- [ ] Dependents run only after their upstream wave integrates (or the orchestrator can see exactly why not).
- [ ] Combine, the 400-line `claude -p` resolver, and bisection are called, not copied or reimplemented.
- [ ] Design records an explicit yes/no on binary scope-vs-diff enforcement and on the `tasks.md` DAG encoding.
- [ ] Existing propose/design dual-dispatch still reaches `lane.Done` without requiring any new field.
