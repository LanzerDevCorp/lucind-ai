# Verify: Skill Anchoring & Worktree Cleanup Guardrails

**Verdict: PASSED**

Candidate: `231640d72332e9c3ac2e30f2b7b792a5290d032f` (implementation `ad00d71b5ee549760cf316db7cb385c10a0517e8`, cherry-picked/landed as `f621d68` via `apply-skill-anchoring-guardrails-2`, plus the frozen mechanical-check-log commit).

## Stage 1: Mechanical check

`lucind-ai check --out openspec/changes/skill-anchoring-guardrails/verify-mechanical.log` — **passed**, exit 0, duration ~1m33s, all packages `ok`. One transient `internal/feature` failure (`TestConcurrentLeaseAcquisition`, `SQLITE_BUSY`) on the first run was independently reproduced in isolation (3/3 pass) and confirmed as the pre-existing, repository-documented full-suite concurrency flake named in `troubleshooting.md` — unrelated to this change (it does not touch `internal/feature`). Re-run came back fully green. Transcript committed at `openspec/changes/skill-anchoring-guardrails/verify-mechanical.log`.

## Stage 2: Dual qualitative judgment

Dispatched `agy` and `cursor-agent` in one barrier (`lucind-ai run`), both `read_only: true`, both reaching `status: done` on the first attempt (a subsequent `reverted`/renew cycle was purely a reconciliation-registration staleness issue, not a re-judgment — see below).

- **agy**: all 5 capabilities satisfy their requirements; fail-closed guardrail across all 3 dirty categories; all 4 banners fire correctly with stderr/stdout separation; TDD rescue protocol documented. 5 findings, all supporting PASS.
- **cursor-agent**: independently confirmed the same 5 capabilities, plus 10 findings including 4 non-blocking gaps: (1) no dedicated test that unforced cleanup succeeds when only `.lucind/result.json` (ignored) is present — compositional on `PorcelainEmpty`, not directly tested; (2) threat-matrix "Git repository selection" only partially covered — no test distinguishing relative/absolute paths, though `worktree cleanup` never takes an operator-supplied path so this may be moot; (3) `dag.Split`'s new `stderr ...io.Writer` parameter is fail-open by design — a caller that omits it gets no warning; the pre-existing e2e two-wave test still omits it and so cannot catch a missing banner (this repo's own `runSplit` always passes stderr, so the CLI spec is met; the gap is only in that one pre-existing test's coverage); (4) the accept-error path doesn't assert the receipt/reminder is absent on failure (implementation early-returns before `renderAcceptanceReceipt`, so the spec scenario holds in code, just under-tested).

**Orchestrator cross-check**: independently re-opened and confirmed the load-bearing citations from both judges directly against the candidate branch — `internal/worktree/worktree.go` (`Cleanup`/`Remove`/`ErrWorktreeDirty` — force-gated `PorcelainEmpty` check, unconditional path unchanged when `force: true`), `cmd/lucind-ai/cli.go` (`runWorktreeCleanup` — `--force`/`-f` flag, dirty-diagnostic banner citing `troubleshooting.md`, linked-worktree refusal), and `internal/dag/split.go` (`Split`'s variadic `stderr` parameter and multi-wave warning body). All three match both judges' descriptions exactly.

**Disposition**: Unanimous Pass (`done`/`done`). No disputed defects, no unresolved contradictions between judges. The four gaps cursor-agent found are genuine but non-blocking — they narrow test coverage at the margins (ignored-file-only dirty state, relative/absolute path distinction that doesn't apply to this CLI's actual argument shape, one pre-existing e2e test not exercising the new optional stderr parameter, and one error-path assertion). None of them contradicts a requirement or leaves a requirement unimplemented. Recorded as follow-up rather than blocking.

## Reconciliation note (process, not code)

Promotion was blocked twice by the cross-feature overlap gate against `native-stability-campaign` (a live worktree sharing `cmd/lucind-ai/cli.go`'s churn hotspot; that feature has zero committed content of its own, so there was no real textual conflict to resolve out of band — only a declared-overlap registration to clear). Both times resolved via `lucind-ai reconcile approve`/`resolve` (the second via `reconcile renew` after the candidate advanced past the first resolution's registered SHA), then `lucind-ai integrate retry` — no AI redispatch involved, consistent with this Change's own recovery protocol. Evidence: reconciliation requests `4289a950-...` (superseded) and `4d64bf91-...` (integrated), both direction `skill-anchoring-guardrails -> native-stability-campaign`.

A separate apply-stage deviation (packet `allowed_paths` omitted two pre-existing test files, `internal/integrate/integrate_test.go` and `internal/run/isolation_test.go`, that had to change for the new 4-argument `worktree.Remove` signature to compile) was corrected by re-dispatching a packet with corrected `allowed_paths` that cherry-picked the already-completed, already-correct commit rather than re-implementing it — no work was lost, no unnecessary re-authorship occurred.

## Next

Archive.
