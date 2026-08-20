# Verify: verify-dual-dispatch

**Overall verdict: PASSED**

This is the mechanism verifying itself: the two-stage verify protocol this change built is used
here, for the first time, to check the change that built it — closing the circularity that
blocked self-verification until this change's own apply phase merged.

## Stage 1 — Mechanical Check

- Command: `lucind-ai check --out openspec/changes/verify-dual-dispatch/verify-mechanical.log`
- Result: **passed**, exit code 0, duration 12.595374208s.
- Candidate git SHA: `cfee550a326975bf27b43a968ca1d54396e10046`.
- Full transcript: `openspec/changes/verify-dual-dispatch/verify-mechanical.log` (committed, commit `922df57`).

## Stage 2 — Dual Qualitative Judgment Dispatch

Two `read_only: true` packets dispatched in parallel via `lucind-ai run`:

- `verify-verify-dual-dispatch-agy` (executor: `agy`) — status `done`.
- `verify-verify-dual-dispatch-cursor-agent` (executor: `cursor-agent`) — status `done`.

Both lanes satisfied the read-only completion contract and integrated cleanly. Full structured
envelopes persisted to `.lucind/results/verify-verify-dual-dispatch-{agy,cursor-agent}.json` via
the `fix-persist-lane-envelope` fix.

## Stage 3 — Evidence Cross-Checking & Reconciliation

**Case: Unanimous Pass** — both lanes returned `done`, no blocking findings.

cursor-agent's envelope was the more detailed of the two; every citation load-bearing for a
non-blocking finding was independently re-verified by the orchestrator:

1. **This change's own `design.md`/`tasks.md` did not anticipate the envelope-persistence gap.**
   Both lanes flagged this independently (agy: "design documents must trace artifact persistence
   across worktree cleanup boundaries"; cursor-agent: names the exact mechanism —
   `Integrate` always calls `RemoveLaneWorktree`, `printReport` only emits `Envelope.Summary`,
   the ledger stores only status transitions). This is the same gap discovered live during this
   change's own apply phase (task 5.3) and already fixed by the separate `fix-persist-lane-
   envelope` change — this self-verify is itself running on the *fixed* binary, which is why
   these very findings survived to be read at all. Non-blocking: already remediated, outside this
   change's own diff.
2. **No pre-dispatch gate rejects a verify packet missing `read_only: true`.** **Confirmed
   accurate** by reading `internal/packet/packet.go` directly: `ReadOnly bool` (line 51) defaults
   to Go's zero value `false` when the `read_only` key is simply absent from frontmatter — parsing
   only validates the value *if present* (`ErrInvalidReadOnly` on a malformed boolean, line 27),
   never that the key exists at all. Protection today is template discipline only
   (`verify-packet-template.md` always sets `read_only: true`) plus
   `TestVerifyPacketTemplateAssetStructure` asserting the *template* has it — nothing asserts
   that an arbitrary hand-authored verify packet does. A mis-authored verify packet omitting the
   key would silently dispatch as a normal write lane and could commit. Non-blocking (the named
   spec scenario's `WHEN` clause is about orchestrator validation, which `design.md` explicitly
   chose not to implement as new parser code), but a real, worth-tracking gap.
3. **`check --out` on a failing script is implemented but untested.** `runCheck` writes the
   `--out` log before returning exit 1 on a failing script (confirmed: the write happens ahead of
   the failure return in `cli.go`), but no existing test exercises `check --out` against a
   failing `lucind-checks.sh` to lock in the `Exit Code: 1` header. Non-blocking: production
   behavior is correct by inspection, this is a coverage gap, not a defect.

None of the three findings contradicts a tested spec scenario or blocks this verdict.

## Result

`openspec/changes/verify-dual-dispatch/state.yaml` updated: `verify: { status: done }`.

With this, all three dual-executor-dispatch sibling changes (`read-only-packet-dispatch`,
`apply-dag-dispatch`, `verify-dual-dispatch`) have both `apply` and `verify` at `done`, each with
a real, evidence-backed `verify.md` — ready for `archive`.
