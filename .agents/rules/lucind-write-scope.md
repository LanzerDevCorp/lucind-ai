# Write Scope Enforcement

The packet's `## Allowed paths` section (and frontmatter `allowed_paths`) defines the ONLY files you may create, modify, or delete in this lane.

## Rules
1. **Scope Check**: Before writing any file, confirm it is in `## Allowed paths`.
2. **Deviation Protocol**: If a file outside `## Allowed paths` must be touched, STOP immediately. Do NOT edit it. Return `status: deviated` with the exact path and rationale in `.lucind/result.json`.
3. **Always Forbidden**:
   - `~/.claude.json`, `~/.claude/` (Claude configuration).
   - `.git/` internal contents (use git CLI commands, never direct file edits).
   - Any files in sibling worktrees or outside the declared workspace.
4. **Always Writable in Your Worktree**:
   - `.lucind/result.json` (the result envelope).
   - `.lucind/result.schema.json` (read-only reference).
