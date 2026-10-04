---
name: lucind
description: Orchestrate Antigravity (agy) from Claude through herdr, with lucind-ai as the deterministic contract for code-changing lanes. Use when delegating implementation, e2e tests, visual validation, exploration or research to agy, or when the user mentions lucind, lanes, dispatch, accept or attest. Requires HERDR_ENV=1.
---

# lucind — Claude orchestrates, agy executes, lucind-ai enforces

Split of work: Claude owns criteria (plan, architecture, when to parallelize, merges, conflicts,
review, model choice). agy does the heavy lifting (implementation, exploration, e2e, visual
checks). `lucind-ai` only enforces what needs no judgment: the result contract, allowed paths,
and test attestation.

Load the `herdr` skill too; this skill assumes its pane/agent commands.

## Choose the path

| Work | Path |
|---|---|
| Read-only: explore, research, fan-out of explorers | **Free agy via herdr** — no lucind-ai |
| Changes code | **Lane**: `lucind-ai dispatch` → `wait` → `accept` |

### Free agy (read-only)

1. `herdr pane split --current --direction right|down --cwd "$PWD" --no-focus` → pane id.
2. `herdr agent start <name> --kind agy --pane <id> -- --dangerously-skip-permissions`.
3. `herdr agent prompt <pane-id> "<task>. Do not edit any file. ..."` — always say *do not edit*;
   nothing enforces it outside a lane.
4. Short answers: `herdr pane read <pane-id> --source recent-unwrapped --lines 120`.
   Long research: ask agy to write Markdown into your scratchpad and reply with the path.
5. Fan-out = repeat per explorer. Close panes you created when done.

### Lane (code changes)

```bash
lucind-ai dispatch --cwd <dir> --allow '<glob>' [--allow ...] --brief <file|-> \
  [--model M] [--timeout 60m] [--detach] [--min-quota 0.1]
lucind-ai wait <lane>            # only after --detach
lucind-ai accept --lane <id>     # run from the lane's repo
```

- `dispatch` creates `.lucind/lanes/<id>/`, records the base tree, opens a new pane with
  `LUCIND_LANE=<id>`, starts agy and sends it the brief plus a contract footer.
- Blocks until agy's Stop hook marks the lane `done`/`failed`; prints one JSON object
  `{lane, pane_id, cwd, status, result_path}`. Exit codes: 0 done, 1 error, 3 failed, 4 timeout.
- The pane is **never** closed or killed by lucind-ai. Talk to agy freely with
  `herdr agent prompt <pane_id> ...`; the contract is checked once, at `accept`, on the final tree.
  For a formal follow-up turn with wait + validation: `dispatch --lane <id> --brief ...`.
- On `timeout`: inspect the pane, nudge, or close it yourself. lucind-ai does nothing more.
- `accept` writes `receipt.json` and accepts only when: the result envelope is valid; every file
  changed since the base tree matches an `--allow` glob; and an attestation of
  `sh lucind-checks.sh` matches the final tree (otherwise it runs the checks itself).
  **Never re-run tests yourself after an accepted receipt** — the attestation is the proof.

## Writing a brief

Free Markdown, not validated by lucind-ai. Include: goal, scope, acceptance criteria, constraints
(TDD, style), and what to report in the result envelope. The footer added by `dispatch` already
covers lane id, allowed globs, result path and the final `lucind-ai attest run -- sh lucind-checks.sh`.
Keep `--allow` as narrow as the task: it is enforced by agy's PreToolUse hook and again by `accept`.

When the task needs project skills, add this section to the brief, one exact `SKILL.md` path per
line, and nothing else under it:

```markdown
## Skills to load before work
/abs/path/to/skill-a/SKILL.md
/abs/path/to/skill-b/SKILL.md
```

Resolve the paths yourself (skill registry or `~/.claude/skills`); agy reads those files before
touching code. Omit the section when no skill applies.

## Parallelism and worktrees

- Default: one lane in the main checkout, on a branch you create. lucind-ai never creates
  branches or worktrees.
- Parallel writers: one writer per tree. Create worktrees yourself
  (`git worktree add <repo-parent>/<repo>-worktrees/<name> -b <branch>`), dispatch one lane per
  worktree (`--cwd` = worktree), then merge with plain git and resolve conflicts yourself.
- Read-only agents can share any tree.

## Models

`--model` > `LUCIND_AGY_MODEL` > agy default. Fast/cheap model for exploration and validation;
strong model for implementation and e2e.

## Quota

`--min-quota F` refuses to dispatch below a remaining 5h fraction. When agy runs out, switch
accounts with `agy-pool use <email>` (`agy-pool usage --refresh`, `agy-pool best`) and start a
fresh agy session; an open session does not pick up new credentials reliably.

## Setup

`make install` installs the binary and the embedded agy plugin
(`lucind-ai plugin install`, which registers it via `agy plugin install` into `~/.gemini/config/plugins/lucind/`). Check the build with
`lucind-ai -v` before dispatching.
