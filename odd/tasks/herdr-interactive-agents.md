# herdr-interactive-agents

Feature branch: `feature/herdr-interactive-agents` (worktree `~/git_root/lucind-ai-worktrees/herdr-interactive-agents`, from `feature/herdr-agent-factory`). Mirror: Engram topic `odd/herdr-interactive-agents/tasks`.
Follows `odd/tasks/herdr-agent-factory.md`. Spike facts: `docs/herdr-spike-findings.md` section 8. Runs in parallel with another agent's `herdr-direct-dispatch` (same file `internal/executor/herdr.go`; expect a merge there).

## Objective

Every agent that lucind-ai dispatches through herdr runs interactively in a visible pane (the owner always watches it work), not as a headless `run.sh`. Completion is signalled by an Antigravity `Stop` hook, not by process exit, and Antigravity Rules carry the lane's standing constraints.

## Problem

- `herdr-agy` types `sh run.sh` (`agy --print ...`) into a pane: output is redirected to files, the pane shows nothing useful.
- Interactive `agy -i` never exits, so the exit-code sentinel cannot mark the end.
- Interactive `agy` blocks on a trust prompt for every new folder (trust is exact-path, not inherited).
- Lane constraints travel only in the prompt, which grows with every packet.

## Decisions

- Completion signal = `Stop` hook with `fullyIdle:true` (spike: fires at the end of every turn). The hook writes a sentinel file; the result envelope file stays the contract. A hard timeout still applies (hook failure must not hang a lane).
- The `Stop` hook may return `{"decision":"continue","reason":...}` when the result envelope is missing or invalid, bounded by a counter (max 2), so the agent repairs its own envelope. After the cap the hook lets it stop and the dispatcher marks the lane as today.
- Hooks and rules are written per lane into the worktree (`.agents/hooks.json`, `.agents/rules/lucind-*.md`) and excluded from the lane diff and the dispatcher commit. Never global config.
- Trust: lucind-ai adds each lane worktree path to `trustedWorkspaces` in `~/.gemini/antigravity-cli/settings.json` (owner-authorized 2026-10-03), parse-preserving, atomic, never rewriting a file it cannot parse, and removes the path when the lane worktree is removed.
- Rules: detailed guides use `trigger: model_decision` (progressive disclosure), the lane's write scope uses `always_on`; one file at most 24 KB, all rules under the 20k-token budget; generated from `lucind-rules.md` sections tagged for antigravity lanes.
- Interactive runs give no usage JSON and no exit code; the usage log records `tokens_known:false` for them (transcript parsing is out of scope).
- Scope: `agy` first. Lanes with executor `agy` run through `herdr-agy` interactive when `HERDR_ENV=1`; `cursor-agent` and `claude` interactive are follow-ups.

## Scope

In: T1-T6 below. Out: removing headless `agy` for non-herdr environments (CI, no herdr), cursor/claude interactive, transcript-based token accounting.

## Constraints

- Worker never commits; the dispatcher commits. Conventional Commits, no AI attribution, English artifacts.
- Test-first where a deterministic runner exists; RED observed first.
- `make install` after binary changes. Plugin skill edits need `make bump-plugin-version` and a byte-identical OpenCode copy.
- About 400 authored changed lines per task is a planning heuristic only.
- No push, PR or merge. Do not touch the other agent's branch or worktrees.

## Delivery

Forecast about 900-1,100 authored changed lines (T1-T6). Strategy `ask-on-risk`; each task is a work-unit commit with tests and docs.

## Tasks

- [x] **T1. Workspace trust registry.** `internal/agytrust`: `Add(path)` / `Remove(path)` on `trustedWorkspaces`; atomic write (temp + rename, mode kept), refuses unparseable JSON, preserves unknown keys, file lock. Route: inline. ~120 lines.
  - Route: inline (small, security-sensitive; ~230 lines with tests). RED: package did not compile before the implementation; GREEN: 8 tests, `-race -count=5`. Behavior: parse-preserving (unknown keys kept, indentation normalized), atomic temp+rename keeping the file mode, `flock` on `settings.json.lucind.lock`, never rewrites JSON it cannot parse or a wrongly typed `trustedWorkspaces`, no write when already trusted, `Remove` is a no-op for a missing file or entry, relative paths rejected. Tests only use temp dirs; the real settings file was not touched. Commit `62ce490`.
- [x] **T2. Lane hooks.** `lucind-ai hook stop` subcommand (reads the Stop payload on stdin, writes the sentinel, validates the envelope with `result.Read`, returns continue/stop with a bounded counter) and `internal/agyhooks` that writes the worktree's `.agents/hooks.json` plus the info/exclude entry. Route: delegated writer. ~300 lines.
  - Route: delegated writer = `agy` (`gemini-3.8-flash-high`, print mode) in lane `lane/ia-t2-hooks`, packet = spec file in the worktree; parent verified scope (only the 5 allowed files), gofmt, vet, `go test -race`, read both implementation files, patched one defect (1 KiB error truncation could split a UTF-8 character), cherry-picked `ceec200`. Delivered: `lucind-ai hook stop` (always exit 0, `fullyIdle:false` writes nothing, valid envelope writes `done.json`, invalid one returns `continue` up to `--max-continues`, then `done.json` invalid), `internal/agyhooks` (`Install` writes `.agents/hooks.json` with a single `lucind-lane` Stop handler, refuses an existing file, adds `/.agents/hooks.json` to the repo `info/exclude`; `Done`, `ReadDone`, `WriteDone`). Notes for T3: the dispatcher must delete a stale `done.json` and `continues` before each launch, and remove `.agents/hooks.json` after the lane. Observed once: `TestExploreAllThreeLensesSucceed` failed in a full package run (3 lane executions instead of 4); 5 later runs passed, base too; not caused by this change, likely a timing flake.
- [x] **T3. Interactive HerdrAgy.** Run `agy -i` in the lane pane, wait for the sentinel file (poll, hard timeout), then end the session cleanly; trust the worktree first (T1), install hooks (T2); keep the headless path behind `Interactive=false`. Route: delegated writer. ~300 lines. Conflicts with `herdr-direct-dispatch` T3 on `internal/executor/herdr.go`.
  - Route: inline (owner asked for no agy for now). RED: 7 tests did not compile before; GREEN: `go test ./internal/executor -race` 185 passed. Design for a small merge surface with `herdr-direct-dispatch`: all logic in new `internal/executor/herdr_interactive.go`; `herdr.go` only gained 3 config fields (`Interactive`, `HookBinary`, `TrustSettingsPath`) and a 3-line dispatch at the top of `Run`. Flow: trust the worktree, install the Stop hook, open the pane, type `sh run.sh` (`agy -i`, no output redirection), wait for whichever comes first of `done.json` (hook), the pane sentinel (agy exited by itself, reports its exit code) or the deadline; then two C-c, and the T16 hard stop (second C-c, close only a workspace this executor opened). `valid` and `invalid` envelopes both exit 0 (the dispatcher demotes an unreadable envelope), a hook error is exit 1; trust and `hooks.json` are removed at the end; state dir removed on success. Live proof (`TestHerdrAgyInteractiveLive`, opt-in `LUCIND_HERDR_LIVE=1`, 72 s): real `agy -i` in a real herdr pane, the worktree was trusted automatically (no prompt), the agent wrote a deliberately incomplete envelope, the Stop hook answered `continue`, the agent repaired it, the final envelope validated; the real settings file and state dir were left clean (a harmless `settings.json.lucind.lock` file remains beside the settings). Commit `03bea9a`.
- [x] **T4. Antigravity rules for lanes.** Extend `internal/rules` to emit `.agents/rules/lucind-*.md` with valid frontmatter (`always_on` write scope, `model_decision` guides), size and budget checks, excluded from the lane diff. Route: delegated writer. ~200 lines. Depends on T2 (exclude mechanism).
  - Route: inline. RED: lane rule tests did not compile before. GREEN: rules, agyhooks, executor and run packages `-race` 524 passed. `rules.LaneFiles` renders three Antigravity rule files from the existing `lucind-rules.md` sections (workspace file if present, else the default): `lucind-worker.md` (`always_on`, `worker` + `all` sections), `lucind-lane-scope.md` (`always_on`, allowed and read-only paths; a hostile path cannot break out of its code span or start a heading) and `lucind-result-envelope.md` (`model_decision`, loaded on demand, schema inlined with `@[Result schema](../../.lucind/result.schema.json)`). `agyhooks.InstallRules/RemoveRules` write them under `.agents/rules`, only `lucind-*.md` names, never overwriting, excluded from git (`/.agents/rules/lucind-*.md`), removed with `.agents` when empty. `executor.Request` gained `AllowedPaths` (the dispatcher passes the packet's), a rules failure is a warning in `Outcome.Stderr`, not a lane failure. Live proof: the live test put a worker rule that the prompt never mentions ("every file must contain BANANA") in the temp repo's `lucind-rules.md`; real agy created `hello.txt` with BANANA, so the lane rules do reach the agent (137 s, trust and hook flow as in T3). Commit `ff3f6ad`.
- [ ] **T5. Route agy lanes through interactive herdr.** With `HERDR_ENV=1`, executor `agy` lanes (including explore lenses) use `herdr-agy` interactive; opt-out env `LUCIND_HERDR_VISIBLE=off`. Route: inline or delegated. ~100 lines. Depends on T3.
- [ ] **T6. Docs, install, real end-to-end proof.** Docs and skill text; `make install`; sandbox repo proof: a write lane with dispatcher commit and a forced bad envelope that the `Stop` hook repairs, watched in a pane. Depends on T1-T5.

## Acceptance criteria

- An `agy` lane dispatched through herdr is visible and interactive in its pane from start to finish.
- A lane finishes when the `Stop` hook reports `fullyIdle`, with the envelope validated; a missing or invalid envelope is repaired by the agent (max 2 `continue`s) or the lane is blocked as today.
- A new lane worktree never shows the trust prompt; the trust entry is removed with the worktree.
- Hooks and rules never appear in the lane diff or the dispatcher commit.
- `go test ./...` passes; headless `agy` still works.

## Progress

- 2026-10-03: feature document created from the spike (hooks schema, trust inheritance, `Stop` continue verified).
- 2026-10-03: T4 closed (`ff3f6ad`).
- 2026-10-03: T3 closed (`03bea9a`).
- 2026-10-03: T2 closed (`ceec200`).
- 2026-10-03: T1 closed (`62ce490`).

## Next step

T5: route `agy` lanes through interactive herdr.
