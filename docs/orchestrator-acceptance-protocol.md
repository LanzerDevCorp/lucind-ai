# Orchestrator Acceptance Protocol

A field report from running `native-stability-campaign` end to end as Orchestrator: apply phase (5 planned waves + 6 unplanned remediation waves), SDD verify, and a cross-feature reconciliation incident. This document is not a proposal — it is a description of what I actually did, command by command, followed by a set of concrete recommendations for making this repeatable without relying on one Orchestrator's memory and discipline.

## 1. Scope

Over one long session I dispatched 13 packets against `feature/native-stability-campaign` (Waves 1–4b from the original `tasks.md`, plus Waves 6a–6e born from a gap I found during my own acceptance review, plus one remediation packet R1 born from an SDD-verify BLOCKED verdict), ran a dual-executor qualitative verify, and worked through a real cross-feature reconciliation race against a concurrently active peer session. Every one of those dispatches went through the same acceptance protocol before I trusted it. That protocol — not the feature code — is the subject of this document.

## 2. The protocol, as actually executed

This is the sequence I repeated, without exception, after every `lucind-ai run` completed:

1. **Read the raw dispatch output**, not a summary of it — lane `status`, `summary`, and the `integrate:` line (`attempted`/`passed`/`integrated`/`reverted`).
2. **Confirm the new tip**: `git rev-parse refs/heads/feature/<name>`.
3. **Diffstat against the packet's own `base_sha`**: `git diff --stat <base_sha>..<new_tip>` — this is the *only* reliable check that a lane actually stayed inside its declared `allowed_paths`. A lane's own summary saying "only touched X and Y" is a claim, not evidence.
4. **Read the full diff** of every changed file: `git diff <base_sha>..<new_tip> -- <file>`. Not a skim — every hunk, because the defects that mattered were never in the parts of the diff a summary would have called out.
5. **Read the result envelope** at `.lucind/results/<packet-id>.json` — `done_criteria`, `hard_stops` (all must be present and `fired: false` unless a real stop occurred), `files_changed`.
6. **Read the new/changed tests themselves**, not just their names or pass/fail status. A test named `TestReceiptNotWrittenOnFailure` is only evidence if it actually asserts `os.Stat` + `errors.Is(err, os.ErrNotExist)` — I confirmed this by hand every time rather than trusting the name.
7. **Stand up an isolated worktree** off the new tip, always detached or on a throwaway branch, never the primary checkout and never the lane's own worktree:
   ```
   git worktree add /path/to/verify-<id> <branch-or-sha> [--detach]
   ```
8. **Run the mechanical checks myself, directly, inside that worktree** — never via `lucind-ai check` from a worktree (see §5.1 for why):
   ```
   cd <worktree> && gofmt -l <touched files>
   cd <worktree> && go vet ./...
   cd <worktree> && go build ./...
   cd <worktree> && go test -v ./<package> -run <TestName>
   cd <worktree> && go test ./...          # full repo, not just the touched package
   ```
9. **Tear the worktree down**: `git worktree remove <path> --force && git worktree prune`.
10. **Persist a memory of what I actually verified** (`mem_save`, structured `**What**/**Why**/**Where**/**Learned**`) *before* reporting acceptance — this made it possible to reconstruct exact reasoning after later context compaction, and is what let this document exist at all.
11. **Only then** report acceptance to the user, and merge/promote.

Step 5 (full-repo `go test ./...`, not just the touched package) caught real regressions in *already-accepted* code more than once — a change in `internal/executor/agy.go` broke nothing in its own package's tests but needed the full suite to prove it hadn't broken `internal/run`.

## 3. What worked well

- **Never trusting a summary.** Every real defect this session was found by opening code the lane's own envelope had marked `done`/`met: true`. None were found by reading a summary more carefully — they required opening the file.
- **Full-repo tests, every time**, not scoped to the touched package. Cheap insurance; caught cross-package regressions the touched package's own tests could not see.
- **Chaining `cd <dir> && <command>` in one shell invocation.** The tool's working directory does *not* persist between separate Bash calls, but *does* persist between commands joined by `&&`/`;`/newline within one call. Treating these as the same thing was the direct cause of the one real incident this session (§4).
- **Never merging into a branch another session has checked out.** Using a temporary worktree (`git worktree add --detach <path> <sha>`) for every reconciliation merge kept me from ever touching a peer session's live checkout.
- **Saving memory before reporting**, not after. This is what let me correctly answer "what did you actually do" days into a long session, and what let me self-correct when the same defect pattern (e.g. worktree-path mismatch) reappeared in a different form.
- **Asking before escalating scope**, every time a defect turned out to require more than a one-file fix (`AskUserQuestion`). Six of the eleven packets this session existed only because I found something the original plan hadn't anticipated and asked before dispatching a fix for it, rather than silently expanding scope or silently ignoring it.
- **The native dual-executor qualitative verify design** (`agy` + `cursor-agent` dispatched together, independently, on the same frozen candidate) is a genuinely load-bearing design decision, not decoration. In this session the two judges *disagreed*: `agy` reported the implementation fully compliant; `cursor-agent` found four real, independently-confirmed HIGH-severity spec gaps (a worktree-path convention mismatch that would have broken the very first live Trial, and a "Promoted" state transition that never actually promoted anything). If a blanket "always use the same executor" policy had been applied here, the second judge would never have run, and a structurally broken campaign would have shipped as PASSED.

## 4. What went wrong: the `cd` incident

Reconciling an overlap with a peer feature, I ran a compound worktree-setup command where the `git worktree add` step failed (the target branch was already checked out by another session's worktree). The subsequent `cd` into a path that was never created **failed silently and the script kept running in whatever directory the shell was already in** — which happened to be the primary checkout, on `dev`. The `git merge` that followed landed a merge commit directly on `dev`.

Caught it within the same turn by checking `git branch --show-current` and `git log --oneline -3` immediately after — the unexpected merge commit was obvious. Recovered with `git reflog show dev` to find the exact pre-mistake tip, confirmed nothing had been pushed, and used `git reset --hard <verified-sha>` — **only after explicit user confirmation**, since a hard reset is exactly the class of action that requires it.

The fix going forward, adopted for the rest of the session: never issue `cd X` as its own step and trust the next command runs there. Always chain `cd X && pwd && <verify> && <the actual risky command>` in one call, so a failed `cd` aborts the whole chain instead of silently executing downstream commands in the wrong directory. `pwd` as the second link is not decorative — it is the assertion that step one actually worked.

## 5. Other discoveries worth flagging on their own

### 5.1 `lucind-ai check`'s root resolution changed mid-session, silently

A peer feature's own remediation (`fix(ultrafixer): remediate verify violations`, upstream commit `3441c81`) deliberately changed `resolvePrimaryRoot()` in `cmd/lucind-ai/cli.go` from `git rev-parse --show-toplevel` (resolves to *whatever worktree you're standing in*) to `git rev-parse --git-common-dir` + `filepath.Dir()` (resolves to *the primary checkout, always*, regardless of cwd). This is a deliberate, correct hardening of the project's own "linked-worktree dispatch is refused" rule — not a bug.

The practical consequence: once that fix reached the installed binary, running `lucind-ai check` from *any* worktree silently tests the **primary checkout's currently-checked-out content**, not the worktree's own tree. I only caught this because a `lucind-ai check` run printed a commit SHA that didn't match the worktree I was standing in, and I did not accept that at face value — I cross-checked with `git rev-parse HEAD` directly in the worktree, found a mismatch, and did the git archaeology (`git log -S`, `git show <commit>:<path>`) to find *when* and *why* it changed, rather than assuming it was either "always been broken" or "a bug I should report."

**Practical rule going forward**: never trust `lucind-ai check`'s own printed `commit:` line as proof of what was tested from inside a worktree. Verify with `go test`/`go vet`/`go build`/`gofmt` directly, scoped to the worktree's own `cwd` — which is exactly what the acceptance protocol in §2 already does, and is now the *only* reliable path.

### 5.2 Reconciliation against an actively-moving peer branch does not converge on its own

Reconciling one single-line conflict (a `usage` string constant in `cmd/lucind-ai/cli.go`) against a concurrently active peer session took **five** full renew → approve → merge → verify → resolve → retry cycles, because the peer's branch tip moved during the ~1–2 minutes each cycle took. Each cycle was individually correct and individually verified (build clean, no conflict markers, `gofmt` clean) — the problem was purely a TOCTOU race between "I resolved against tip X" and "the tip is now X+1" with no way to reserve or pause the target.

Separately, and more seriously: one of those attempts left concrete evidence of **ledger corruption** — a `reconciliation_requests` row whose `target_sha` column held the exact value of its *own* candidate's `candidate_sha` instead of the actual peer branch tip. This was not explainable by timing; it corroborated a defect the peer session had already reported independently. I stopped attempting further automatic reconciliation at that point rather than risk compounding corruption on a ledger someone else's fixer was already investigating, and eventually integrated the one remaining lane with a manual `git merge --no-ff` in a throwaway worktree once a peer-side fix (disabling the now-merged-and-archived feature, which removes it from the overlap gate's comparison set) made the native path viable again — and finally, when even that stalled on an orphaned lease with no live process behind it, by explicit human instruction to stop waiting and finish it manually.

### 5.3 An orphaned lease can outlive the process that held it, with no clean release path

`lucind-ai feature recover --attempt <id>` is the documented recovery path for a stuck attempt, but it performs its own CAS check against the *feature's* static `expected_parent_sha` (frozen at feature-creation time) — which, on a feature that has been promoted through 20+ waves, will essentially never match the branch's real current tip. `recover` correctly refuses in that case (`recovery ref mismatch`) rather than promoting on stale data, but it does **not** release the lease it was invoked on. The only path I found within the CLI's documented surface was to wait out the lease's own TTL. I did not find (and did not go looking for, given time pressure) a `feature lease release`-equivalent command backed by a liveness check.

## 6. Real defects the protocol actually caught

| # | What it was | How it was found | Would a "trust the summary" flow have caught it? |
|---|---|---|---|
| 1 | Orphan sweeper never set `worktree_preserved`, silently excluding done-but-unintegrated lanes from `integrate retry` forever | Investigating a lane stuck in an unexpected state, not from any single dispatch's own report | No |
| 2 | `integrate retry` used the feature's immutable creation-time `base_sha` instead of each lane's own recorded dispatch-time target, breaking retry on any multi-wave feature past wave 1 | Reading `internal/run/integrate_retry.go` directly after a retry behaved unexpectedly | No |
| 3 | `RunCampaign`'s production default silently fed a fixed fake PGID (`99999999`) into the live crash-verification path, making "verify the process is really dead" a permanent no-op for the one real Campaign this feature exists to run | Reading the accepted diff line by line after the lane reported `done_criteria: met: true` on exactly this behavior | **No — this is the strongest example.** The envelope said the criterion was met. |
| 4 | The journey code computed worktree paths as `<primaryRoot>/wt-<id>`, but real production worktree creation uses a completely different, already-existing convention (`<parent>/<repo>-worktrees/<id>`) — a real live Trial would have failed immediately | SDD verify's dual-judgment dispatch: one judge (`cursor-agent`) flagged it, the other (`agy`) did not; independently re-verified the flagged claim against the real `worktree.pathFor` source before accepting it | Only half the time — depends entirely on which single judge you happened to run |
| 5 | The journey's "Promote" stage was a bare state-machine transition (`AdvanceTrial(TrialPromoted)`) with no call to any of the actual CAS-promotion or ancestry-verification functions it names | Same dual-judgment verify pass as #4 | Only half the time, same reason |
| 6 | Reconciliation ledger row with `target_sha` corrupted to equal its own `candidate_sha` | Direct SQLite inspection (`sqlite3 .lucind/lucind.db "SELECT ..."`) after a resolved-and-verified reconciliation still failed to unblock promotion | No — this required distrusting the CLI's own reported state and going to the ledger directly |

Three of six defects were found specifically *because* two independent AI judges were run and disagreed. That is not incidental to this protocol — it is close to its most valuable single property.

## 7. Proposal: making this repeatable without relying on one Orchestrator's memory

The protocol in §2 worked, but it worked because I executed the same ~10 manual steps by hand, every time, for thirteen dispatches in a row, without skipping any. That is not a scalable assumption for every future Orchestrator session, especially a tired or less careful one. Concrete recommendations, roughly in priority order:

1. **Codify the checklist in the skill itself, as literal numbered steps, not prose.** `references/contracts/acceptance-promotion.md` already exists as the designated module for "Lane Acceptance, batch outcome, checks, or Change Promotion" per the skill's own Decision Gates table — but it should contain the exact sequence in §2 as copy-pasteable steps, including the exact commands, not a description of the *idea* of verification. A prose description invites a future Orchestrator to approximate the spirit and skip the substance.

2. **Add a mechanical `lucind-ai accept` convenience command** that automates the *setup/teardown* half of the protocol — resolve the new tip, diff against `base_sha` to enumerate touched paths, spin up and tear down a throwaway worktree, run `gofmt`/`go vet`/`go build`/`go test ./...` inside it, and print one structured report. This does **not** replace steps 4 and 6 (reading the diff and the tests with human/AI judgment) — those stay manual and irreducible — but it removes the tedious, error-prone, purely mechanical scaffolding around them, which is exactly the kind of step a tired Orchestrator is most likely to shortcut.

3. **Delegate Acceptance itself to a dedicated, narrow-scoped sub-agent** whose entire prompt *is* the checklist from §2, with tool access limited to `Read`/`Grep`/`Bash` (scoped to a worktree it creates and destroys itself). This gets two things at once: it enforces the checklist mechanically (the sub-agent has no other instructions to drift toward), and it keeps the Orchestrator's own context from ballooning with every diff and test-run transcript across a long multi-wave session — a real, measured cost this session (context usage climbed from single digits to over 50% well before the work was done, driven almost entirely by inline verification output).

4. **Keep — do not "simplify away" — the native dual-executor qualitative verify design**, and consider extending the same principle to Acceptance itself for any packet whose `Tier` is `A` (human-merge-worthy): run two independent read-only judgment passes over the same frozen diff rather than one. §6 is a concrete, session-real case for why: single-judge review missed two HIGH-severity defects that a second, differently-modeled judge caught.

5. **Add a documented, liveness-checked lease-release path** for the orphaned-lease case in §5.3 — something equivalent to `feature lease release --id <feat> --owner <o> --fence <f>` that verifies (via a `/proc`-equivalent check, matching the pattern this feature's own `internal/stability/process` package already implements) that no live process actually holds the lease before releasing it, rather than requiring a human to either wait out a TTL or hand-edit the ledger.

6. **Add a `--wait-stable <duration>` (or equivalent) mode to `lucind-ai reconcile`** that blocks until the target feature's tip has been unchanged for the given window before returning, instead of requiring a human to manually detect and react to a moving target across repeated renew/approve/merge/resolve cycles (§5.2). This would have collapsed five manual cycles into one wait.

7. **Make root-resolution behavior changes in `lucind-ai` itself loud, not silent** (§5.1) — e.g., have `lucind-ai check`'s own output explicitly print *which* directory it resolved and tested (`resolved primary root: <path>`, not just a bare `commit:` line), so a change in resolution semantics between binary versions is visible in the tool's own output rather than requiring git archaeology to notice after the fact.

None of the above proposals remove the Orchestrator's judgment from the loop — they remove the *mechanical* cost of exercising it, so that judgment gets spent on steps 4 and 6 of §2 (reading the actual diff, reading the actual tests) every single time, instead of being the first casualty of a long session.
