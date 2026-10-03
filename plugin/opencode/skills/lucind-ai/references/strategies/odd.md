# ODD strategy

Load this module when executing under the Organic Driven Development strategy for `lucind-ai`.

## Execution

1. **Feature document and checklist**: A feature document (`odd/tasks/<feature>.md`) holds the authoritative task checklist and design records.
2. **Task packets**: Each task is a packet declaring frontmatter:
   - `route`: `inline`, `worker`, or `fanout`
   - `route_evidence`: rationale justifying the route
   - `allowed_paths`: exact file boundary for changes
   - `verification`: commands to execute
   - `known_environmental_failures`: expected baseline failures
   - `named_skills_only`: optional boolean (`true` loads only explicitly named skills plus `lucind-executor`)
3. **Dispatch thresholds**:
   - `inline`: reserved for small, understood fixes.
   - Dispatch (`worker` / `fanout`): required when touching 2+ non-trivial files, creating a new file, performing more than 5 lookups, or reading that prepares a write.
4. **Worker boundary**: The worker never runs `git add`, `git commit`, or `git push`. It operates strictly within `allowed_paths` and returns the `.lucind/result.json` result envelope. If an instruction, path, or requirement is ambiguous, report `interaction_required` with concrete candidate choices instead of guessing.
5. **Dispatcher verification and commit**: The dispatcher verifies candidate work (including HMAC test attestation) and commits approved results on the candidate branch.

## Loop and escalation

When a packet declares `max_iterations` (> 1) or an `escalation` ladder (up to 3 rungs), the dispatcher executes a write/test/fix loop:

- **Requirements**: a looped packet must declare `verification` and a `commit_message`: the loop reuses one worktree across attempts and the dispatcher, not the worker, commits once verification passes.
- **Between attempts**: the previous result envelope is deleted before the next attempt (a failure to delete it fails the lane), verification output is fed back inside a fence longer than any backtick run it contains and capped at 8 KiB, and if the context ends between attempts the lane ends `blocked` with the last failure in its reason.
- **Retry condition**: The loop retries only when the worker reported done but dispatcher verification failed (a declared `verification` command exited non-zero or the tree changed during verification). Any other non-done outcome (timeout, envelope blocked/deviated/failed/interaction_required, allowed_paths violation, missing skills, nothing to commit, HEAD moved) stops the loop immediately without retry.
- **Same worktree**: Worker retries execute in the same worktree without reset so uncommitted changes persist. The subsequent attempt's prompt receives the failed command, exit code, and captured tail output.
- **Attempt plan and total cap**: Rung 0 (the packet's declared executor/model) and each escalation rung receive `max(1, max_iterations)` attempts, subject to a hard cap of `MaxTotalAttempts = 4` executor runs across the entire lane.
- **Exhaustion**: When all attempts on rung 0 fail with no ladder declared, the lane ends `failed` (`write/test/fix loop exhausted after N attempts: <reason>`). When all declared escalation rungs are exhausted (or the total cap of 4 attempts is reached on or after escalation), the lane ends `blocked` (`escalation ladder exhausted after N attempts: <reason>`) requiring human review.

## Route validation

Before dispatch, `lucind-ai run` computes signals from the packet and repository base (`BaseSHA` or `HEAD`):
- `allowed_paths` count (0 for read-only lanes)
- `new_file` flag: true if any allowed path does not exist at the base, ends with `/`, or contains glob characters (`*`, `?`, `[`)
- `risk_tier`: computed from path rules (`high`, `medium`)

These signals enforce dispatch thresholds against declared routes:
- **No route declared**: accepted as-is (legacy behavior unchanged).
- **`inline`**: if any hard signal fires (`allowed_paths >= 2`, `new_file`, or `risk_tier == high`), the route is upgraded to `worker`. Packets below the threshold are accepted.
- **`worker`**: requires non-empty `route_evidence`. Accepted whether below or above threshold (over-delegation is safe).
- **`fanout`**: requires non-empty `route_evidence`, and must either be `read_only` or declare at least 2 allowed paths; otherwise rejected.
- **Unknown routes**: rejected.
