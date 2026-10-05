# Skill Selection via Jev (Experimental)

`lucind-ai skills select` asks Jev (TypeSafe SystemOne) which skills a coding subagent must load before working on a task brief.

When dispatching tasks to coding subagents, injecting only relevant skills keeps agent prompts focused, reduces context window overhead, and prevents subagents from getting confused by unrelated guidelines. Rather than forcing human or lead orchestrators to manually inspect a skill registry (often 40+ skills) on every task dispatch, `lucind-ai skills select` evaluates the task brief and allowed edit surfaces against indexed skills using Jev's parallel Noul evaluations.

> [!NOTE]
> This command is currently **experimental**. Raw probabilities, selection decisions, and token usage are output as JSON so orchestrators can evaluate selection accuracy and threshold calibration before enabling automatic skill injection.

## Usage

```bash
lucind-ai skills select --brief <file|-> [--allow <glob>]... [--cwd <dir>] [--registry <path>] [--threshold <float>]
```

### Flags

- `--brief <file|->`: (Required) Path to the task brief markdown file, or `-` to read the task brief from standard input (`stdin`).
- `--allow <glob>`: (Repeatable) Allowed edit surface glob patterns. Can be specified multiple times or as comma-separated values. The selector parses these globs to derive relevant file extensions (such as `.go`, `.ts`, `.md`) included in Jev's state payload.
- `--cwd <dir>`: Working directory (defaults to `.`). Used to resolve the repository root via Git (`git rev-parse --show-toplevel`).
- `--registry <path>`: Path to the skill registry markdown file (defaults to `<repo-root>/.atl/skill-registry.md`).
- `--threshold <float>`: Minimum probability threshold for a skill to be marked selected (defaults to `0.7`, valid range is `(0, 1]`).

### Environment

The API key must be supplied via the environment:

- `TYPESAFE_API_KEY`: API key for the TypeSafe SystemOne Jev API. Sent as a Bearer token in the `Authorization` HTTP header. Required when invoking `lucind-ai skills select`.

## Registry Source

The skill candidates are read from `<repo-root>/.atl/skill-registry.md`. This registry file is generated and updated by:

```bash
gentle-ai skill-registry refresh
```

The registry is a Markdown table indexing available skills, their trigger descriptions, and absolute paths to their `SKILL.md` files. Jev only evaluates existing skills listed in this registry—it never invents skill paths or searches for unindexed skills.

## Evaluation Model

Skill selection evaluates every skill in the registry using TypeSafe SystemOne's **Noul** primitive:

1. **Parallel Noul Evaluations**:
   All skills from the registry are submitted to Jev in a single batch request containing one parallel Noul question per indexed skill (`skill_000`, `skill_001`, ...).
2. **Context & Executor Instructions**:
   The request state includes the brief text, allowed edit surfaces, and derived file extensions. The executor instruction establishes the coding subagent boundary:
   > *"A coding subagent will implement `task_brief`, editing only files matching `allowed_edit_surfaces`. It writes code, tests and docs. It cannot open pull requests, create issues, orchestrate or delegate to other agents, review other work, or talk to the user."*
3. **Question & Evaluation Criteria**:
   - **Question**: *"Must this subagent read the skill below before it starts changing files?"*
   - **True Criteria** (task relevance): *"The skill's guidance directly shapes code, tests or documentation the subagent will write for this task."*
   - **False Criteria** (filtering): *"The skill is unrelated to the task, only covers orchestration, delegation, pull requests, issues, reviews or user interaction, or merely shares the task's topic without guiding the subagent's edits."*
   This boundary filters out orchestrator-only skills (such as PR creation, issue triage, multi-agent orchestration, or dual reviews) and ignores general topic overlaps that do not directly guide the subagent's file edits.
4. **Sorting and Decisions**:
   - Each skill receives a probability score between `0.0` and `1.0`.
   - Skills with `probability >= threshold` receive `"selected": true`; all others receive `"selected": false`.
   - Output decisions are sorted by **probability descending** (highest probability first).
   - Any ties in probability are broken deterministically by **skill name ascending** (alphabetical order).

## Output Format

The command outputs formatted JSON to standard output:

```json
{
  "model": "jev-latest",
  "threshold": 0.7,
  "decisions": [
    {
      "name": "golang-testing",
      "path": "/home/user/.gemini/config/skills/go-testing/SKILL.md",
      "probability": 0.94,
      "selected": true
    },
    {
      "name": "golang-cli",
      "path": "/home/user/git_root/lucind-ai/.agents/skills/golang-cli/SKILL.md",
      "probability": 0.88,
      "selected": true
    },
    {
      "name": "branch-pr",
      "path": "/home/user/.gemini/config/skills/branch-pr/SKILL.md",
      "probability": 0.12,
      "selected": false
    }
  ],
  "usage": {
    "input_tokens": 1420,
    "output_tokens": 84
  }
}
```

### Output Fields

- `model`: Model identifier returned by the Jev API (defaults to `jev-latest`).
- `threshold`: The probability threshold used to determine selection.
- `decisions`: Array of evaluated skills, sorted by probability descending, then by name ascending:
  - `name`: Identifier of the skill.
  - `path`: Absolute filesystem path to `SKILL.md`.
  - `probability`: Float probability in `[0, 1]` indicating Jev's assessment of whether the subagent must read the skill.
  - `selected`: Boolean indicating if `probability >= threshold`.
- `usage`: Token consumption statistics (`input_tokens`, `output_tokens`).

## Examples

### Running with a Task Brief File

Evaluate skill requirements for a task brief on disk, specifying allowed edit surfaces and a probability threshold:

```bash
lucind-ai skills select \
  --brief odd/tasks/feature-brief.md \
  --allow "cmd/**/*.go" \
  --allow "internal/**/*.go" \
  --threshold 0.7
```

### Piping from Stdin

Pass brief content directly via standard input using `--brief -`:

```bash
cat << 'EOF' | lucind-ai skills select --brief - --allow "cmd/**"
# Implement CLI Flag Validation

Add strict validation for CLI flags in cmd/lucind-ai/skills.go.
EOF
```

### Custom Working Directory and Registry

Specify a distinct working directory or explicit registry location:

```bash
lucind-ai skills select \
  --brief task-brief.md \
  --cwd /path/to/project \
  --registry /path/to/custom-skill-registry.md \
  --threshold 0.8
```

## Brief Integration

Selected skills can be formatted into a task brief under the `## Skills to load before work` section. When this section is present in a brief, the coding subagent reads each listed skill path before making edits:

```markdown
## Skills to load before work
/home/user/.gemini/config/skills/go-testing/SKILL.md
/home/user/git_root/lucind-ai/.agents/skills/golang-cli/SKILL.md
```
