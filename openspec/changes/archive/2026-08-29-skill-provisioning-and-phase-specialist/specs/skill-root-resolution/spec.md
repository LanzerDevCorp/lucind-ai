# Delta for skill-root-resolution

## ADDED Requirements

### Requirement: Root resolution and fail-closed admission

The system MUST resolve skill names to `SKILL.md` paths through `.lucind/skill-roots.yaml` with tilde expansion, and MUST fail batch admission with field-specific diagnostics identifying the missing skill and searched roots if any required skill cannot be resolved.

#### Scenario: Tilde-expanded skill root resolution

- GIVEN `.lucind/skill-roots.yaml` with root `~/.claude/skills` and skill `sdd-propose` at `~/.claude/skills/sdd-propose/SKILL.md`
- WHEN batch admission resolves the skill path
- THEN resolution MUST expand `~` to the home directory and locate `SKILL.md`.

#### Scenario: Multi-root ordered resolution

- GIVEN `.lucind/skill-roots.yaml` with two roots where a skill exists only under the second root
- WHEN root resolution searches configured roots in order
- THEN resolution MUST locate the skill under the second root.

#### Scenario: Unresolvable required skill fails admission

- GIVEN a required skill missing from all configured roots in `.lucind/skill-roots.yaml`
- WHEN batch admission validates the batch
- THEN admission MUST return a non-nil error naming the unresolvable skill and searched roots before creating worktrees.
