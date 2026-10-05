# Skill Selection via Jev (Experimental)

`lucind-ai skills select` asks Jev (TypeSafe SystemOne) which skills a coding subagent must load before working on a task prompt.

When dispatching tasks to coding subagents, injecting only relevant skills keeps agent prompts focused, reduces context window overhead, and prevents subagents from getting confused by unrelated guidelines. Rather than forcing human or lead orchestrators to manually inspect a skill registry (often 40+ skills) on every task dispatch, `lucind-ai skills select` evaluates the task prompt and allowed edit surfaces against indexed skills using Jev's parallel Noul evaluations.

> [!NOTE]
> This command is currently **experimental**. Raw probabilities, selection decisions, and token usage are output as JSON so orchestrators can evaluate selection accuracy and threshold calibration before enabling automatic skill injection.

## Usage

```bash
lucind-ai skills select --prompt <file|-> [--allow <glob>]... [--cwd <dir>] [--registry <path>] [--threshold <float>]
```

### Flags

- `--prompt <file|->`: (Required) Path to the task prompt markdown file, or `-` to read the task prompt from standard input (`stdin`).
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
   The request state includes the prompt text, allowed edit surfaces, and derived file extensions. The executor instruction establishes the coding subagent boundary:
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

### Running with a Task Prompt File

Evaluate skill requirements for a task prompt on disk, specifying allowed edit surfaces and a probability threshold:

```bash
lucind-ai skills select \
  --prompt odd/tasks/feature-prompt.md \
  --allow "cmd/**/*.go" \
  --allow "internal/**/*.go" \
  --threshold 0.7
```

### Piping from Stdin

Pass prompt content directly via standard input using `--prompt -`:

```bash
cat << 'EOF' | lucind-ai skills select --prompt - --allow "cmd/**"
# Implement CLI Flag Validation

Add strict validation for CLI flags in cmd/lucind-ai/skills.go.
EOF
```

### Custom Working Directory and Registry

Specify a distinct working directory or explicit registry location:

```bash
lucind-ai skills select \
  --prompt task-prompt.md \
  --cwd /path/to/project \
  --registry /path/to/custom-skill-registry.md \
  --threshold 0.8
```

## Prompt Integration

Selected skills can be formatted into a task prompt under the `## Skills to load before work` section. When this section is present in a prompt, the coding subagent reads each listed skill path before making edits:

```markdown
## Skills to load before work
/home/user/.gemini/config/skills/go-testing/SKILL.md
/home/user/git_root/lucind-ai/.agents/skills/golang-cli/SKILL.md
```

## Automated Selection (`dispatch --auto-skills`)

Rather than running `lucind-ai skills select` manually and copying paths into the task prompt, the orchestrator can pass `--auto-skills` to `lucind-ai dispatch` to automate evaluation and injection:

```bash
lucind-ai dispatch \
  --prompt task-prompt.md \
  --allow "cmd/**/*.go" \
  --auto-skills
```

### How It Works

- **Opt-in**: Automated selection is active only when `--auto-skills` is explicitly supplied.
- **Registry and Jev evaluation**: Reads candidate skills from `<repo-root>/.atl/skill-registry.md` and calls Jev using `TYPESAFE_API_KEY`.
- **Prompt injection**: When skills are selected (passing threshold 0.7), `lucind-ai` moves the `## Skills to load before work` section right after the lane marker at the top of the prompt sent to agy, so the worker loads skills before reading the goal and scope.
- **Manual override takes precedence**: If the prompt already contains `## Skills to load before work`, Jev is not called and the prompt is kept unchanged.
- **Fail open**: On any error (such as a missing registry, unset `TYPESAFE_API_KEY`, network failure, or API error), `lucind-ai` warns on `stderr` and dispatches without the section. Dispatch never fails due to selector errors.
- **Always writes lane record**: Always writes `.lucind/lanes/<id>/skills-<turn>.json` recording selection decisions and telemetry for each turn.

## Lane Record Format (`skills-<turn>.json`)

When `--auto-skills` is enabled, `lucind-ai dispatch` always records `.lucind/lanes/<id>/skills-<turn>.json` (for example, `skills-1.json`) for the lane turn.

### Schema Fields

- `turn` (`int`): Turn number of the lane.
- `injected` (`bool`): Whether the `## Skills to load before work` section was injected into the prompt.
- `skipped_reason` (`string`, optional): Reason why skill injection was skipped. Omitted when skills are injected or when an error occurs. Known values:
  - `"brief_has_section"`: The prompt already contained a `## Skills to load before work` section; Jev was not called.
  - `"no_skill_selected"`: Jev evaluated candidate skills, but none met the selection threshold (`probability >= 0.7`).
- `error` (`string`, optional): Error message if the selector failed. Omitted on success.
- `result` (`object`, optional): The full `skillselect.Result` with decisions and token usage (`model`, `threshold`, `decisions`, `usage`). Omitted if Jev was not called (e.g. `brief_has_section`) or if the selector failed.

### Examples

#### 1. Skills Selected and Injected

Jev selected one or more skills with probability exceeding the threshold; the section was injected into the prompt:

```json
{
  "turn": 1,
  "injected": true,
  "result": {
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
}
```

#### 2. Prompt Already Contains Skills Section (`brief_has_section`)

The task prompt already contains `## Skills to load before work`. Jev is skipped and the prompt is preserved unchanged:

```json
{
  "turn": 1,
  "injected": false,
  "skipped_reason": "brief_has_section"
}
```

#### 3. No Skill Selected (`no_skill_selected`)

Jev evaluated candidate skills, but none reached the threshold. No skills section is injected, and the full evaluation result is recorded:

```json
{
  "turn": 1,
  "injected": false,
  "skipped_reason": "no_skill_selected",
  "result": {
    "model": "jev-latest",
    "threshold": 0.7,
    "decisions": [
      {
        "name": "golang-testing",
        "path": "/home/user/.gemini/config/skills/go-testing/SKILL.md",
        "probability": 0.35,
        "selected": false
      },
      {
        "name": "branch-pr",
        "path": "/home/user/.gemini/config/skills/branch-pr/SKILL.md",
        "probability": 0.08,
        "selected": false
      }
    ],
    "usage": {
      "input_tokens": 1280,
      "output_tokens": 62
    }
  }
}
```

#### 4. Selector Error (Fail Open)

An error occurred during selection (e.g. missing API key, network timeout, or invalid registry). `lucind-ai` warns on stderr, records the error, and dispatches the lane without the skills section:

```json
{
  "turn": 1,
  "injected": false,
  "error": "call jev: Post \"https://api.typesafe.ai/v1/noul\": dial tcp: lookup api.typesafe.ai: no such host"
}
```

