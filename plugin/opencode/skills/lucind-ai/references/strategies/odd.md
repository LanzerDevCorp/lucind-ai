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
6. **Iteration budget**: Write/test/fix loops are capped at 4 iterations before escalating.

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
