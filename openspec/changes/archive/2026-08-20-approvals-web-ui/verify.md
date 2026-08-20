# Verify: Approvals Web UI

**Overall status: PASSED (post-remediation)** — originally BLOCKED on the findings below.
Remediated via a scoped TDD dispatch (packet `remediate-approvals-web-ui-verify`, commit
`662b871`, integrated `6902555`): removed the over-permissive `trimmed.includes('\n')` disjunct
from `isValidEvidence` (closes the spec violation), clarified `serve --approval-timeout`'s help
text as informational-only, added a `decision='pending'` guard + `ErrAlreadyDecided`/409 to
`ledger.Decide`. Orchestrator independently re-verified the diff and a real
`go build/vet/test -race -count=1` run — all green. The findings below are preserved as the
original verify record; treat this file's header as the current status.

## Stage 1: Mechanical Check

`lucind-ai check` — PASSED. `go build ./...`, `go vet ./...`, `go test ./... -race -count=1` all
green at candidate `f7421fb0a88f1a8727547242f0dad7756ff2dbe3` (includes `internal/serve`). Full
transcript: `openspec/changes/approvals-web-ui/verify-mechanical.log`.

## Stage 2: Qualitative Judgment (dual dispatch)

Both lanes reported `status: done`. `agy` found no issues; `cursor-agent` surfaced 8 findings,
several of which the orchestrator independently verified against the real code and spec text
before accepting (per SKILL.md: green lane status is not proof of complete work).

## Confirmed findings (orchestrator-verified, not just self-reported)

### 1. BLOCKING — `isValidEvidence` violates the "Bare claim withheld" spec scenario

`openspec/changes/approvals-web-ui/specs/approvals-web-ui/spec.md:61-65` is a hard requirement:

> GIVEN an item with neither command output nor a `file:line` reference, WHEN the UI renders it,
> THEN the system MUST NOT present an unsupported claim as evidence.

`internal/serve/static/app.js:12-19`'s `isValidEvidence` treats **any string containing a
newline**, or starting with `ok `/`PASS`/`$ `, or containing `---`, as valid "command output" —
independent of whether a real `file:line` or command transcript is present. A multi-paragraph
bare claim with a blank line between paragraphs renders as evidence in the UI, exactly the failure
mode this whole project exists to prevent (PRD: "green criteria with a walked-past hard stop").
`static_test.go:50-52` only greps for the substring `opencode`, not this behavior — the gap has
no covering test.

**Remediation**: tighten `hasCommandOutput` to require an actual `file:line` pattern OR a real
transcript marker tied to an actual test/build run, not a bare newline; add a test asserting a
multi-line prose string with no `file:line` renders as "(no command output or file:line evidence
provided)".

### 2. Confirmed defect — `serve --approval-timeout` is decorative

`cmd/lucind-ai/cli.go:571` parses `--approval-timeout` on `serve` (default `30m`) but the only use
is `cli.go:603`'s `fmt.Fprintf` banner — it is never passed to anything that gates a lane. The
actual gate is `Deps.ApprovalTimeout`, wired only from `run --approval-timeout` (default `0` =
bypass, `cli.go:125,513`). An operator who runs `lucind-ai serve --approval-timeout 10m` expecting
that to configure the gate gets no such effect; `run` must separately pass the same flag. Not a
spec violation (no delta-spec scenario mandates this flag's behavior) but a real, confirmed
UX/documentation mismatch against `design.md:41-43` and `tasks.md:46`.

**Remediation**: either wire `serve`'s flag into a default used when a `run` doesn't specify its
own, or remove the flag from `serve` and document that the gate is `run`-only.

### 3. Confirmed defect — `Decide` has no `decision='pending'` guard (race)

`internal/ledger/ledger.go:539-544`'s `Decide` is an unconditional `UPDATE ... WHERE run_id=? AND
lane_id=?`, no `AND decision='pending'`. `WaitDecision`'s timeout path (`ledger.go:701-705`) also
calls `Decide` (recording `timed_out`) using `context.Background()`. A late UI click landing after
a timeout has already fired can overwrite `timed_out` with `approved`/`rejected` in the
`approvals` table, even though `Execute` has already demoted the lane to `lane.Blocked` and
persisted that terminal status (`run.go:406-412`) — leaving `approvals.decision` inconsistent with
`lanes.status`. Narrow blast radius (does not reopen the already-terminal lane), but a genuine
audit-trail correctness gap directly in the threat class `design.md`'s own Threat Matrix calls out
("Unauthenticated loopback HTTP that can release a waiting lane").

**Remediation**: add `AND decision = 'pending'` to `Decide`'s `WHERE` clause; treat 0 rows affected
as "already decided" rather than `ErrLaneUnknown` (needs a distinguishable error).

## Non-blocking findings (recorded, not remediation-required before archive)

- `ledger.Approvals(runID)` (`ledger.go:636-648`) has zero callers — dead exported API; drop or
  wire it.
- Merged-batch `opencode` command (`cli.go:600`) is a static template, not derived per-batch, and
  omits the `--prompt` fragment this repo's own RDD docs use
  (`plugin/claude-code/skills/lucind-ai/references/runtime.md:77`). `design.md:120-122`'s open
  question ("Merged-batch opencode argv vs integrate combined-tree path") is still genuinely open
  — this is not a new defect, it's the same acknowledged gap, still unresolved.
- UI/CLI test coverage is weaker than `tasks.md`'s all-`[x]` state implies: no `httptest` `GET /`
  or `GET /api/state` test exists; `TestBulkRequestBodyReturns400` (`server_test.go:79-92`) checks
  only the response code, not that the ledger row is unaffected; the barrier-stays-idle test
  (`batch_test.go:872-928`) never samples `barrier.Released` *during* the wait, only after;
  `TestServeFlagsAndSubcommandRecognized` (`cli_test.go:1825-1831`) does not assert the documented
  defaults. Treat `tasks.md`'s checkmarks as "code exists and builds," not as full scenario proof.

## Verdict

**BLOCKED** on finding 1 (confirmed spec violation). Findings 2 and 3 are real but not literal
spec violations — recorded as required remediation before archive per this project's own
precedent (state.yaml notes elsewhere: "the actual cause was ... not anything wrong with the apply
work itself," i.e. block on real defects, don't block on process noise). Non-blocking findings are
follow-up backlog, not archive blockers.
