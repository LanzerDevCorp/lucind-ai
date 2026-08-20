# Proposal: Read-Only Packet Dispatch

## Intent

Enable `lucind-ai` to dispatch read-only packets through `lucind-ai run`, starting with the `explore` SDD phase and establishing the foundation for qualitative `verify` reviews, instead of running `explore` locally as a Claude Code subagent.

Dispatching `explore` aligns with the project's core operating philosophy: Claude Code orchestrates while `agy` and `cursor-agent` execute in isolated git worktrees (`plugin/claude-code/skills/lucind-ai/SKILL.md:74-81`). Currently, a contract gap prevents this: mandatory done-criterion #2 in `plugin/claude-code/skills/lucind-ai/assets/packet-template.md:40-41` requires that "The work is committed" with `git status --porcelain` empty and `git log --oneline -1`. A read-only exploration lane produces reports or findings in the result envelope without creating repository commits. Because frontmatter in `internal/packet/packet.go:29-47` does not currently declare read-only execution, an `explore` lane today would either fail mandatory criterion verification or be forced to fabricate artificial commits.

Crucially, the binary runtime is already capable of handling uncommitted completion: `internal/result/result.go:122-135` (`Envelope.LaneStatus()`) maps `status: "done"` directly to `lane.Done` without requiring `Envelope.Commit` or `Envelope.FilesChanged`, both of which are optional in `.lucind/result.schema.json:6` (lines 44–57, 99–101) and unmarshaled as `omitempty` in `internal/result/result.go:107-110`. This proposal establishes the authoring, convention, and skill framework to make read-only packet dispatch first-class and safe.

## Scope

### In Scope
- Packet contract adjustments in `plugin/claude-code/skills/lucind-ai/assets/packet-template.md:35-42` allowing read-only packets to explicitly declare mandatory done-criterion #2 (commit) as N/A with finding/envelope deliverable evidence.
- Orchestrator documentation and workflow updates in `plugin/claude-code/skills/lucind-ai/SKILL.md:74-81` removing the blocker on `explore` phase dispatch via `lucind-ai run`.
- Optional frontmatter representation (e.g., in `internal/packet/packet.go:29-47` and `internal/packet/packet.go:65-76`) to allow packets or orchestrators to explicitly indicate read-only semantics if the design phase determines binary-level visibility is desired.
- Result reporting conventions for read-only lanes where deliverable evidence is captured in `.lucind/result.json` (`findings`, `summary`, or designated artifact paths) validated against `.lucind/result.schema.json:6`.

### Out of Scope
- Code-level automatic routing logic — routing decisions remain explicit human/orchestrator choices per `plugin/claude-code/skills/lucind-ai/SKILL.md:31-39` and `docs/prd.md` section 6 step 1.
- Modifying the 6-value `lane.Status` enum in `internal/lane/status.go:10-17` or changing terminal status evaluation in `internal/result/result.go:122-135` and `internal/run/run.go:407-432`.
- Altering worktree creation or isolation mechanisms in `internal/worktree/worktree.go:61-83` and `internal/run/run.go:222-250` (read-only lanes continue to run in isolated worktrees to prevent read contention and state corruption).
- Implementation artifacts (`design.md`, `spec.md`, `tasks.md`).

## Non-goals

- **No `allowed_paths` enforcement**: Path boundary enforcement is a separate concern/sibling change and is not addressed here.
- **No DAG splitting**: Splitting `tasks.md` into dependent packets is the responsibility of the `apply-dag-dispatch` sibling change.
- **No `verify` dual-dispatch design**: While read-only packet dispatch is a direct prerequisite for dual-dispatch qualitative verification (`plugin/claude-code/skills/lucind-ai/SKILL.md:80`), designing the `verify` mechanical vs. qualitative split is handled in the `verify-dual-dispatch` change.
- **No final schema / field name mandate**: This proposal does not dictate the final frontmatter schema or field naming (e.g., `read_only: true` vs. prompt-level convention); that determination belongs to the parallel `design` phase.

## Impact on Existing Dual-Executor Flow

The existing dual-executor propose/design/specs/tasks flow (`plugin/claude-code/skills/lucind-ai/SKILL.md:42-73`) is active, verified, and responsible for dispatching SDD lifecycle lanes today.

This proposal is strictly backward-compatible:
1. **Preservation of Write Packets**: Standard write-based packets (such as `propose`, `design`, `specs`, `tasks`, and `apply`) retain mandatory done-criterion #2 ("The work is committed") and continue to enforce conventional commits and clean `git status --porcelain`.
2. **Opt-in Read-Only Handling**: Read-only rules apply only to lanes explicitly authored as read-only (such as `explore` or qualitative review).
3. **Runtime Stability**: Because `internal/result/result.go:122-135` and `.lucind/result.schema.json:6` already accept envelopes without `commit` or `files_changed`, no breaking changes are introduced to the execution engine (`internal/run/run.go:219-367`).

## Capabilities

### New Capabilities
- `read-only-packet-dispatch`: Support for dispatching non-mutating SDD phases (starting with `explore`) via `lucind-ai run` with clean validation, explicit N/A commit criteria declarations, and structured finding delivery.

### Modified Capabilities
- `packet-authoring`: Packet templates (`plugin/claude-code/skills/lucind-ai/assets/packet-template.md:30-47`) and skill documentation (`plugin/claude-code/skills/lucind-ai/SKILL.md:92-97`) define clear rules for read-only done criteria without ambiguity.
- `sdd-explore-dispatch`: Orchestrator guidelines in `plugin/claude-code/skills/lucind-ai/SKILL.md:74-81` unblock running `explore` in parallel worktrees via `lucind-ai run`.

## Approach

The execution binary already supports result envelopes from non-mutating executions:
- `internal/result/result.go:102-115` marks `FilesChanged` and `Commit` as `omitempty`.
- `internal/result/result.go:122-135` computes `lane.Status` directly from the validated status string without checking for git commits.
- `.lucind/result.schema.json:6` defines `required: ["packet_id", "status", "summary", "hard_stops"]`, leaving `commit` and `files_changed` optional.

The gap is entirely at the authoring and orchestrator contract layer:
1. Update `plugin/claude-code/skills/lucind-ai/assets/packet-template.md:40-41` to document that read-only packets mark mandatory criterion #2 as N/A with an explicit note in the envelope, delivering findings via `summary`, `findings`, or designated exploration docs.
2. Update `plugin/claude-code/skills/lucind-ai/SKILL.md:74-81` to transition `explore` from "Target direction, not yet built" to the standard dispatch workflow.
3. Keep worktree isolation intact: even read-only lanes receive an isolated worktree created via `internal/worktree/worktree.go:61-83` to maintain CodeGraph index isolation and prevent interference with active workspaces.

### Open Questions for Design Phase
- Should read-only status be declared explicitly in YAML frontmatter (e.g., `read_only: true` parsed in `internal/packet/packet.go:65-76`), or should it remain a prompt/template convention evaluated during orchestrator synthesis?
- Should read-only lanes generate an ephemeral exploration artifact (e.g., `openspec/changes/<change>/explore-<executor>.md`) or deliver strictly through `.lucind/result.json` `findings` and `summary` fields?

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `plugin/claude-code/skills/lucind-ai/assets/packet-template.md` | Modified | Clarifies mandatory done criteria exceptions for read-only packets. |
| `plugin/claude-code/skills/lucind-ai/SKILL.md` | Modified | Unblocks `explore` dispatch in target direction table and updates packet authoring rules. |
| `internal/packet/packet.go` | Potential Modification | Optional additive frontmatter field parsing if selected by the design phase. |
| `internal/result/result.go` | Unchanged | Already supports uncommitted envelopes and maps terminal statuses cleanly. |
| `internal/run/run.go` | Unchanged | Worktree lifecycle and status decision logic already handle read-only executions. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Executor hallucinates dummy commits to satisfy perceived git requirement | Medium | Clear prompt-level instruction in packet template stating commit criterion is N/A for read-only lanes. |
| Write packet accidentally authored as read-only bypassing commit requirement | Low | Orchestrator review verifies write phases (`apply`, `propose`, `design`) enforce mandatory commit criteria. |
| Worktree accumulation from read-only exploration runs | Low | Worktree preservation policy (`docs/prd.md:164-166`) remains uniform; worktrees can be pruned post-synthesis. |

## Rollback Plan

Changes are additive to skill documentation, packet templates, and optionally frontmatter parsing. Reverting the relevant git commits restores previous Claude Code subagent exploration behavior immediately with zero database migration, zero schema breaks, and no impact on active worktrees.

## Dependencies

- Existing `internal/result/result.go` schema validation and envelope mapping.
- Existing `internal/worktree/worktree.go` worktree creation.
- Existing `plugin/claude-code/skills/lucind-ai/SKILL.md` dual-executor orchestration workflow.

## Success Criteria

- [ ] `plugin/claude-code/skills/lucind-ai/assets/packet-template.md` explicitly specifies how read-only packets satisfy or declare N/A on mandatory done-criterion #2.
- [ ] `plugin/claude-code/skills/lucind-ai/SKILL.md` reflects `explore` as a dispatchable phase via `lucind-ai run`.
- [ ] Existing dual-executor dispatch for propose/design/specs/tasks continues to operate without breaking changes or regression.
- [ ] Dispatched read-only explore lanes successfully return `status: "done"` with valid `.lucind/result.json` envelopes without requiring git commits.
