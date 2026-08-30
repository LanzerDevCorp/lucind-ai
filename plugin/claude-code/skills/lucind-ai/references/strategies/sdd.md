# SDD strategy

Load this module for an approved structured development flow, apply DAG, qualitative verification, or archive.

## Phase routes

| Phase | Current route |
|---|---|
| Explore | Dispatch via `lucind-ai run` with `read_only: true` when delegated. |
| Propose, design, specs, tasks | Use one bounded packet, dual-executor drafts, or the separate multi-lens strategy only when approved. |
| Apply | Author packets directly, or author `openspec/changes/<id>/apply-dag.yaml` and split it. The Orchestrator writes no apply diff in the primary checkout. |
| `verify` | Run `lucind-ai check`, then dual-dispatch `agy` and `cursor-agent` for qualitative judgment. Built. See **Verify dispatch** below. |
| Archive | Use one mechanical `agy` Lane from `../../assets/archive-packet-template.md`; no fan-out or compression. |

## Apply DAG

Keep `tasks.md` as the human checklist; it is not the DAG parser input. `lucind-ai split --dag ... --out .lucind/packets` emits packet files and prints one run command per wave. It does not write `waves.json`, schedule waves, or expose a JSON wave channel.

Run emitted waves sequentially. Advance to wave N+1 only after wave N's `lucind-ai run` exits 0 and every expected Lane appears in `integrated_ids` with none in `reverted_ids`. Halt remaining waves on a non-zero exit, a blocked/failed/deviated lane, a name in `reverted_ids`, or unordered overlapping paths; do not skip failed nodes. The binary does not schedule waves. A Strict-TDD RED-only wave cannot survive the integration check gate; RED and GREEN for one unit belong in one Lane unless every wave is independently green.

## Verify dispatch

Dual-dispatch `agy` + `cursor-agent` for the *qualitative* half of verification after the one mechanical gate.

### Stage 1: Mechanical Check

Run `lucind-ai check --out openspec/changes/<change-id>/verify-mechanical.log` on the candidate branch. Halts immediately when checks fail; remediation precedes judgment. On pass, commit the log to candidate branch `HEAD` so linked judgment worktrees inherit the frozen transcript.

### Stage 2: Dual Qualitative Judgment Dispatch

Create `.lucind/packets/verify-<id>-agy.md` and `.lucind/packets/verify-<id>-cursor-agent.md` from `../../assets/verify-packet-template.md`. Both use `read_only: true` and carry the frozen mechanical summary in `## Context`.

Dispatch both in one `lucind-ai run --packet .lucind/packets/verify-<id>-agy.md --packet .lucind/packets/verify-<id>-cursor-agent.md` invocation. The barrier joins when both Lanes reach terminal status. Judgment Lanes do not rerun `go test`, `go build`, `go vet`, or `lucind-checks.sh`.

### Stage 3: Evidence Cross-Checking & Verdict Reconciliation

Read both `.lucind/result.json` envelopes and independently verify every cited `file:line` against the candidate. Green criteria are not complete proof.

| Case | Required disposition |
|---|---|
| Unanimous Pass, `done`/`done` | Write `openspec/changes/<id>/verify.md` with `PASSED`; update `verify: { status: done }`. |
| Disagreement / Disputed Defects, `blocked`/`deviated` | Confirm violations or refute false positives with concrete evidence. Confirmed violations produce `BLOCKED` and remediation tasks. |
| Lane Failure, `failed` from timeout or infrastructure | Inspect partial branch work, then re-dispatch only the failed Lane before synthesis. |
| Irreconcilable Ambiguity | Record `BLOCKED` and escalate requirement interpretations to the human. |

## Archive

Gate on completed tasks, no CRITICAL verification issue, and all required artifacts. Preserve `.lucind/packets/` and `.lucind/results/` under the Change, merge complete delta requirements into live specs, write `archive-report.md`, then `git mv` the Change into the dated archive path.

Use filesystem copy/move commands for mechanical preservation and record verbatim empty `diff -r` output as evidence. Targeted live-spec editing is the exception. Keep allowed paths specific to this Change and the archive destination.
