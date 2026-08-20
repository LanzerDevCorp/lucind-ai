# Proposal: Apply-phase DAG dispatch

Replace `sdd-apply`'s in-process Read/Edit/Write with real `lucind-ai run` dispatch: split an SDD change's `tasks.md` into packets whose `allowed_paths` do not overlap, run independent packets in parallel, and sequence dependent ones. Reuse the existing combine / 400-line conflict resolver / bisection path. Do not redesign it.

This is the highest-uncertainty piece of the three-change dual-executor effort: **there is zero code that checks declared scope against the actual diff.** Design must treat that gap as novel engineering, not a small add-on to bisection.

## Intent

Today `lucind-ai run` can already execute a flat concurrent batch of packets and integrate the `done` lanes (`docs/prd.md` §6 steps 6–8). Apply still does not use that path. `plugin/claude-code/skills/lucind-ai/SKILL.md:79` states the target — split `tasks.md` into a DAG of packets and dispatch via `lucind-ai run` — and names the blocker: the orchestrator step that produces non-overlapping `allowed_paths` and dependency order is **not built**. Until it is, apply stays a Claude Code sub-agent writing files itself.

This change builds that step and the binary contracts it needs, so apply packets are real lanes with real worktrees, real envelopes, and real integration.

## Scope

### What changes

- An orchestrator convention (and any small in-repo helpers the design chooses) that turns `tasks.md` into packets with non-overlapping `allowed_paths`, grouped so packets that share no file scope run together and dependents wait.
- Binary support sufficient for that convention: `allowed_paths` must become something the program can see, not only prompt prose. Today `packet.Packet` (`internal/packet/packet.go:29–47`) has `ID`, `Executor`, `RoutedBy`, `Model`, `Body` — no `AllowedPaths`. `Parse` (`internal/packet/packet.go:66–75`) only copies `id` / `executor` / `routed_by` / `model`; any other frontmatter key is dropped.
- A decision, implemented in design, on **declared scope vs actual diff**. Today the binary never makes that comparison. `allowed_paths` lives as a Markdown section (`plugin/claude-code/skills/lucind-ai/assets/packet-template.md:48–54`) and a self-policed hard stop (`packet-template.md:79`). `decideStatus` (`internal/run/run.go:407–431`) maps `result.json` through `Envelope.LaneStatus` (`internal/result/result.go:122–135`) and does not inspect the worktree diff. `FilesChanged` is optional (`internal/result/result.go:107`).
- `plugin/claude-code/skills/lucind-ai/SKILL.md:79` apply row: move from "Not built" to the implemented dispatch path. Packet-template / SKILL packet-structure text (`SKILL.md:87`) only if design makes `allowed_paths` machine-readable.
- Surfacing a wave's partial failure to the orchestrator. Prefer existing `IntegrateReport` (`internal/run/integrate.go:14–21`: `Attempted`, `Passed`, `Integrated`, `Reverted`, `Reason`) plus `barrier.Outcome` (`internal/barrier/barrier.go:21–28`: `Integrate` vs `Preserve`) and ledger notes (`revertLanes` writes `EventLaneNote` at `internal/run/integrate.go:274–297`; event types are the six constants at `internal/ledger/ledger.go:358–365`). Design must verify this is enough before adding types.

### What stays untouched

- Bisection (`internal/run/integrate.go:183` `bisect`), conflict resolution (`internal/resolve/resolve.go`: `MaxConflictLines = 400` at line 18; `RealInvoker` is `claude -p --model sonnet` at line 28), combine (`internal/integrate/integrate.go:34–70` `git merge --no-ff` then `resolve.Resolve`; `Check` at line 79). Confirmed current against PRD §6 steps 6–8 (`docs/prd.md:126–137`). This change **calls** them; it does not replace them.
- Flat `ExecuteBatch` (`internal/run/batch.go:66–89`: one goroutine per packet, `sync.WaitGroup`, no wave/DAG). Design may wrap it with sequential `lucind-ai run` calls per wave instead of rewriting it.
- Sibling changes `read-only-packet-dispatch` and `verify-dual-dispatch`. Apply DAG lanes always write files; there is no read-only overlap and no dependency on either sibling.

## Non-goals

- **No read-only-packet design.** Empty/`read_only` `allowed_paths`, swapping mandatory criterion 2 ("the work is committed"), and explore-phase dispatch belong to `read-only-packet-dispatch`.
- **No verify dual-dispatch design.** Mechanical `lucind-checks.sh` vs qualitative dual `agy`/`cursor-agent` judgment belongs to `verify-dual-dispatch`.
- No redesign of bisection, conflict resolution, or combine.
- No `design.md` / `spec.md` / `tasks.md` in this phase. No edits to `openspec/changes/apply-dag-dispatch/state.yaml`.
- No change to `openspec/changes/approvals-web-ui/` or `openspec/changes/read-only-packet-dispatch/`.

## Open design questions

Left to `design`. This proposal does not pick.

1. **Exact DAG/wave representation for `tasks.md`.** Frontmatter on each task? A `depends_on:` / `allowed_paths:` block per work unit? A sidecar next to `tasks.md`? Whatever it is must be writable by `sdd-tasks` and readable by the splitter without the binary parsing Markdown heuristics it does not already own.
2. **Whether / how `allowed_paths` is enforced by the binary vs only declared.** Options range from (a) prose-only, still self-policed — fails the non-overlap *guarantee*; (b) parse into `Packet` and compare `git diff` / porcelain to the declared set before `lane.Done`; (c) also reject overlapping declarations at split time, before dispatch. **(b)/(c) have no precedent in this tree.** Putting the check in `decideStatus` or `result.Read` would touch every existing packet path, including working propose/design dual-dispatch — backward compatibility is a first-class design constraint, not an afterthought.
3. Sequential `lucind-ai run` per wave (reuses `ExecuteBatch` as-is) versus in-binary wave scheduling inside `ExecuteBatch`.
4. After a wave, a `blocked`/`deviated`/`failed` lane: skip only its dependents, or halt the remaining DAG? `Evaluate` (`internal/barrier/barrier.go:36–59`) already splits `Integrate` vs `Preserve` inside one batch; cross-wave policy is new.
5. New ledger event types versus `IntegrateReport` + existing `EventLaneNote` / `EventBarrierReleased` (`internal/run/batch.go:99–104`). Do not add machinery the current report already carries.

## Impact on the existing `sdd-apply` flow

`sdd-apply` is a Claude Code sub-agent that receives a slice of `tasks.md` and implements it by reading specs/design and writing code in its own session (`SKILL.md:79` names that as "sdd-apply's own Read/Edit/Write"). After this change, lucind-ai apply is:

1. Split `tasks.md` into packets (orchestrator judgment — still prose per `docs/prd.md:107–108` step 1, unless design adds a helper).
2. `lucind-ai run` one wave at a time (or one DAG run, if design puts waves in the binary).
3. Each packet is a lane: worktree, envelope, barrier, then combine / resolve / bisect for that wave's `done` lanes.
4. The orchestrator reads `IntegrateReport` / preserved lanes and either dispatches the next wave or stops with questions.

`sdd-apply` stops being the writer of apply diffs for this project. Workload-forecast / chained-PR rules that live in that sub-agent must be restated on the packets or the splitter — design says where, this proposal only notes they cannot vanish.

## Approach

Keep `ExecuteBatch` as the per-wave engine. Put DAG order outside it (sequential runs) unless design proves an in-binary scheduler is smaller than the wrapper. Parse `allowed_paths` into a real field so enforcement, if chosen, is not a Markdown scrape. Call `Combine` → `resolve.Resolve` → `bisect` unchanged. Treat scope-vs-diff as the risky slice: TDD it first, with an explicit "declare-only" fallback only if design rejects binary enforcement.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/packet` | Modified | Likely `AllowedPaths`; `Parse` currently ignores unknown keys |
| `internal/run/run.go` `decideStatus` | Modified if enforcing | Only place a lane becomes `Done` today |
| `internal/result` | Maybe | Schema/envelope vs trusting `files_changed` |
| `internal/run/batch.go` | Untouched or wrapped | Flat batch; waves may sit above it |
| `internal/run/integrate.go`, `internal/integrate`, `internal/resolve` | Reused | No redesign |
| `internal/ledger` | Verify first | Six event types; add only if `IntegrateReport` is insufficient |
| `plugin/claude-code/skills/lucind-ai/SKILL.md` | Modified | Apply row becomes the built path |
| Packet templates | Maybe | If `allowed_paths` moves into frontmatter |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Scope-vs-diff enforcement is greenfield | **High** | Own design-phase analysis; do not assume bisection covers it |
| Enforcement in `result`/`decideStatus` regresses working dual-dispatch | High if schema-strict | Backward-compatible: packets without the field keep today's path |
| DAG encoding bikeshed vs `sdd-tasks` | Med | Pick the smallest thing `sdd-tasks` can emit |
| Overlapping `allowed_paths` declared, not caught | High if declare-only | Split-time overlap check is cheaper than post-hoc; still not a diff check |
| Partial failure needs new events | Low | Read `IntegrateReport` + `Preserve` + `EventLaneNote` before adding types |
| Sibling field shape drift if `read-only-packet-dispatch` lands a different `AllowedPaths` | Med | This change does not wait on that sibling; design notes the reuse-if-present shape |

## Rollback Plan

This file is scoping only: delete `openspec/changes/apply-dag-dispatch/proposal-cursor-agent.md`.

Later implementation: revert packet-field / parse / optional `decideStatus` check / SKILL.md apply-row / template edits. Leave `internal/run/integrate.go`, `internal/resolve/resolve.go`, and `internal/integrate/integrate.go` out of that revert — they were not the change. A declare-only milestone can ship without the enforcement hook; removing the hook then does not unwind dispatch.

## Dependencies

- Working combine / resolve / bisect (PRD §6 steps 6–8) — already on `main`.
- **None** on `read-only-packet-dispatch` or `verify-dual-dispatch`.
- `docs/prd.md` §6 step 1 remains orchestrator judgment for the split itself.

## Success Criteria

- [ ] An SDD apply is a DAG of `lucind-ai run` packets, not `sdd-apply` writing the diff
- [ ] Packets in one wave have non-overlapping declared `allowed_paths`
- [ ] Dependents run only after their upstream wave integrates (or the orchestrator sees why not)
- [ ] Combine, 400-line `claude -p` resolve, and bisection are called, not copied
- [ ] Design records a yes/no on binary scope-vs-diff enforcement and the `tasks.md` DAG encoding
- [ ] Existing propose/design dual-dispatch still reaches `lane.Done` without a new required field
