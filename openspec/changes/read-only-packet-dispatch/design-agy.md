# Design: Read-Only Packet Dispatch

## Technical Approach

`lucind-ai` adds first-class support for read-only packets (such as SDD `explore`, codebase audits, and research lanes) by introducing an optional boolean frontmatter field `read_only: true` (default `false`) in `internal/packet/packet.go:29-47`. 

At dispatch time, `internal/run/run.go:Execute` (`:219-367`) passes the packet's `ReadOnly` flag into `decideStatus` (`:407-432`). In `decideStatus`, the Go runtime enforces the commit invariant: write packets (`read_only: false`, the default) must supply a non-empty `commit` in `.lucind/result.json` before claiming `lane.Done`, while read-only packets (`read_only: true`) are permitted to complete `done` without a commit. 

In `internal/run/integrate.go:Integrate` (`:30-80`), lanes marked `read_only` are bypassed during `CombineTree` branch merging (`:41`) while still having their worktrees cleanly removed in `completeIntegration` (`:150-180`). 

At the prompt level, `plugin/claude-code/skills/lucind-ai/assets/packet-template.md:29-37` updates mandatory criterion 2 for read-only packets from "The work is committed" to "No repository files modified", verified via `git status --porcelain` empty.

## Architecture Decisions

### Decision 1: Add `read_only: bool` to packet frontmatter with `false` default

**Choice**: Add `ReadOnly bool` to `packet.Packet` (`internal/packet/packet.go:29-47`). In `packet.Parse` (`internal/packet/packet.go:50-104`), parse the frontmatter key `read_only:` into a boolean (`true`/`false`). When omitted, Go's zero-value defaults to `false`.

**Alternatives considered**:
- A string enum `mode: read_only | write`: More verbose than needed when the distinction is strictly binary (mutates repository vs reads only).
- Inferring read-only status from packet ID or lane name (e.g. prefix `explore-`): Violates the explicit routing principle of `lucind-ai` (`packet.go:34-38`), introducing fragile implicit coupling.
- No schema addition (prompt-only convention): Leaves the binary unable to distinguish read-only lanes during status decision and batch integration.

**Rationale**:
- **Backward Compatibility**: Every existing packet (and all write packets) omits `read_only:`, automatically evaluating to `false`. `packet.Parse` (`packet.go:92-101`) only requires `id`, `executor`, `routed_by`, and `body`; `read_only` is fully optional.
- **Explicit Invariant**: Explicitly declares the lane's execution contract in frontmatter alongside `executor`, `routed_by`, and `model`.

### Decision 2: Split prompt mandatory criterion 2; leave `human-packet-template.md` and `result.schema.json` intact

**Choice**: 
- In `plugin/claude-code/skills/lucind-ai/assets/packet-template.md:29-37`, split mandatory criterion 2 based on `read_only`:
  - For write packets (`read_only: false` or omitted):
    `- [ ] **The work is committed.** Evidence: git status --porcelain empty and git log --oneline -1. Conventional commit, no AI attribution.`
  - For read-only packets (`read_only: true`):
    `- [ ] **No repository files modified.** Evidence: git status --porcelain empty.`
- In `plugin/claude-code/skills/lucind-ai/assets/human-packet-template.md:1-65`: Make **no changes**. Verified in on-disk source: human packets have no frontmatter block, no automated git branch workflow, no mandatory "The work is committed" criterion, and exist solely for manual secret execution.
- In `internal/result/result.schema.json:6,99-102`: Make **no schema change**. Verified in on-disk source: `"commit"` is already optional (not in `"required": ["packet_id", "status", "summary", "hard_stops"]`). Read-only envelopes can omit `"commit"` or pass an empty string while remaining 100% schema-valid.

**Alternatives considered**:
- Forcing read-only agents to make empty commits (`git commit --allow-empty`): Pollutes git history with synthetic empty commits and hides real changes.
- Making `commit` required in `result.schema.json` with a sentinel string like `"none"`: Breaks JSON schema validation for existing envelopes and introduces string sentinel ambiguity.

**Rationale**:
- Working tree purity is the true invariant of a read-only lane: `git status --porcelain` empty guarantees the agent did not leave dirty edits or unstaged artifacts behind.
- Omitting `"commit"` in `.lucind/result.json` is already supported by `result.schema.json:99-102` and Go unmarshaling in `internal/result/result.go:102-115` (`Commit string json:"commit,omitempty"`).

### Decision 3: Enforce terminal consumption in `run.go` (`decideStatus`) and `integrate.go` (`Integrate`) (Option A)

**Choice**: Adopt **Option (a)**. Name concrete terminal consumers in the Go runtime for `p.ReadOnly`:
1. **Terminal Consumer 1 (`internal/run/run.go:407-432` `decideStatus`)**:
   Pass `p.ReadOnly` into `decideStatus`. When `envelope.LaneStatus() == lane.Done` (`internal/result/result.go:122-135`):
   - If `!p.ReadOnly` (write packet): `decideStatus` enforces `envelope.Commit != ""`. If `envelope.Commit` is empty, `decideStatus` rejects `lane.Done` and returns `lane.Blocked` with diagnosis `"write packet reported done without a commit hash"`.
   - If `p.ReadOnly` (read-only packet): `decideStatus` permits `envelope.Commit == ""`. Additionally, if `len(envelope.FilesChanged) > 0`, it rejects `done` and returns `lane.Blocked` with diagnosis `"read-only packet reported file modifications"`.
2. **Terminal Consumer 2 (`internal/run/integrate.go:30-80` `Integrate`)**:
   `IntegrateReport` / `Integrate` identifies read-only lanes from batch packet metadata. Read-only lanes are omitted from `branches` passed to `CombineTree` (`integrate.go:39-44`) because they carry no branch commits against `main`. In `completeIntegration` (`integrate.go:150-180`), read-only lane worktrees are cleanly removed (`:158`) and ledger preservation is cleared (`:159`) alongside write lanes.

**Alternatives considered**:
- Option (b) (Prompt-asset-only change, zero Go code changes): Keeps the status-quo gap where `Envelope.LaneStatus()` (`result.go:122`) blindly accepts `status: "done"` without verifying commits for write packets, allowing write packets that produce no commits to pass silently.

**Rationale**:
- **Satisfies Indirection Consumption**: `p.ReadOnly` is directly read and acted upon by runtime control logic in `decideStatus` and `Integrate`.
- **Strengthens Write Packet Invariants**: By enforcing `commit != ""` for `read_only: false`, the binary eliminates a known vulnerability where write lanes could hallucinate `done` without committing.
- **Optimizes Integration**: Avoids redundant no-op git branch merges in `CombineTree` for read-only lanes.

### Decision 4: Safe Rollback and Zero Database Migration

**Choice**: Maintain zero-state migration requirement. `read_only` is a stateless packet frontmatter property and does not require changes to the SQLite ledger schema (`internal/ledger/schema.go:1-120`).

**Alternatives considered**:
- Adding a `read_only` column to the `lanes` table in SQLite schema v3: Unnecessary database migration overhead for a property already preserved in the immutable packet body.

**Rationale**:
- If rolled back, reverting Go source and template assets restores the prior behavior with zero database cleanup or migration rollback needed.

## Data Flow

```
+-------------------------------------------------------------------------------+
|                             Packet File (.md)                                 |
|  ---                                                                          |
|  id: explore-feature                                                          |
|  executor: agy                                                                |
|  routed_by: exploration phase                                                 |
|  read_only: true                                                              |
|  ---                                                                          |
+---------------------------------------+---------------------------------------+
                                        |
                                        v  packet.Parse (packet.go:50)
+-------------------------------------------------------------------------------+
|                       packet.Packet { ReadOnly: true }                        |
+---------------------------------------+---------------------------------------+
                                        |
                                        v  Execute (run.go:219)
+-------------------------------------------------------------------------------+
|                        exec.Run -> .lucind/result.json                        |
+---------------------------------------+---------------------------------------+
                                        |
                                        v  decideStatus (run.go:407)
+-------------------------------------------------------------------------------+
|                     decideStatus(deps, wt.Path, outcome, p)                   |
|                                                                               |
|  Is ReadOnly == true?                                                         |
|  - YES: Envelope.Commit can be empty; reject if FilesChanged > 0              |
|  - NO:  Envelope.Commit MUST be non-empty string; else lane.Blocked           |
+---------------------------------------+---------------------------------------+
                                        |
                                        v  status: lane.Done
+-------------------------------------------------------------------------------+
|                     ExecuteBatch / Barrier (batch.go:66)                      |
|                  Barrier.Observe(lane.Done) -> Outcome.Integrate              |
+---------------------------------------+---------------------------------------+
                                        |
                                        v  Integrate (integrate.go:30)
+-------------------------------------------------------------------------------+
|                           Integrate completed batch                           |
|                                                                               |
|  - Read-only lanes: Skip CombineTree branch merge (no new commits)            |
|  - Write lanes: Pass branches to CombineTree, RunChecks, PromoteTarget        |
|  - completeIntegration: Remove worktree & clear ledger preservation for all   |
+-------------------------------------------------------------------------------+
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/packet/packet.go` | Modify | Add `ReadOnly bool` to `Packet` struct (`:29-47`); parse `read_only:` frontmatter key in `Parse` (`:58-76`). |
| `internal/packet/packet_test.go` | Modify | Add unit tests for `read_only: true`, `read_only: false`, and omitted `read_only` backward compatibility. |
| `internal/run/run.go` | Modify | Pass `p.ReadOnly` to `decideStatus` (`:315, :407`); enforce commit presence for write packets and clean working tree for read-only packets (`:407-432`). |
| `internal/run/run_test.go` | Modify | Add tests for write packets failing without commit, read-only packets succeeding without commit, and read-only packets failing with dirty changes. |
| `internal/run/integrate.go` | Modify | Filter out read-only lanes when populating `branches` for `deps.CombineTree` (`:39-44`). |
| `internal/run/integrate_test.go` | Modify | Add integration tests verifying batches with mixed read-only and write lanes. |
| `plugin/claude-code/skills/lucind-ai/assets/packet-template.md` | Modify | Document `read_only: true|false` frontmatter; split mandatory criterion 2 for read-only vs write packets (`:29-37`). |

## Interfaces / Contracts

### `internal/packet/packet.go`

```go
// Packet is one unit of delegated work.
type Packet struct {
	// ID names the lane, its branch, and its worktree directory.
	ID string
	// Executor selects the runtime that carries out the work.
	Executor string
	// RoutedBy is the condition that caused this packet to be routed to Executor.
	RoutedBy string
	// Model selects which model the executor dispatches with (optional).
	Model string
	// ReadOnly indicates whether this packet is prohibited from committing
	// repository changes (e.g. explore/audit phases). Default is false.
	ReadOnly bool
	// Body is the Markdown prompt, passed to the executor unchanged.
	Body string
}
```

### `internal/run/run.go` Status Decision

```go
func decideStatus(deps Deps, worktreePath string, outcome executor.Outcome, readOnly bool) (lane.Status, *result.Envelope, string) {
	switch executor.Status(outcome) {
	case lane.Blocked:
		if outcome.TimedOut {
			return lane.Blocked, nil, "dispatch timed out"
		}
		return lane.Blocked, nil, fmt.Sprintf("dispatch exited %d", outcome.ExitCode)
	default:
		fsys := deps.WorktreeFS(worktreePath)
		envelope, err := result.Read(fsys, resultEnvelopePath)
		if err != nil {
			return lane.Blocked, nil, fmt.Errorf("%w: %v", ErrEnvelopeUnreadable, err).Error()
		}
		st := envelope.LaneStatus()
		if st == "" {
			return lane.Blocked, nil, ErrEnvelopeUnreadable.Error()
		}
		
		// Terminal consumer validation:
		if st == lane.Done {
			if !readOnly && strings.TrimSpace(envelope.Commit) == "" {
				return lane.Blocked, &envelope, "write packet reported done without a commit receipt"
			}
			if readOnly && len(envelope.FilesChanged) > 0 {
				return lane.Blocked, &envelope, "read-only packet reported file modifications"
			}
		}
		
		return st, &envelope, ""
	}
}
```

### `internal/run/integrate.go` Branch Filtering

```go
// In Integrate:
var mergeBranches []string
for _, l := range batch.Lanes {
	if l.Status == lane.Done && !l.ReadOnly {
		mergeBranches = append(mergeBranches, worktree.BranchFor(l.LaneID))
	}
}

if len(mergeBranches) > 0 {
	worktreePath, branchName, err := deps.CombineTree(ctx, deps.PrimaryRoot, deps.RunID, mergeBranches)
	// run checks & promote target
}
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| `internal/packet` | Parse `read_only: true`, `read_only: false`, absent key (defaults `false`), invalid boolean handling | Unit tests in `packet_test.go` using string readers. |
| `internal/run` (`decideStatus`) | Write packet with `Commit: ""` transitions to `lane.Blocked`. Read-only packet with `Commit: ""` remains `lane.Done`. Read-only packet with `FilesChanged` transitions to `lane.Blocked`. | Unit tests in `run_test.go` with mock `deps.WorktreeFS` and fstest. |
| `internal/run` (`Integrate`) | Batch with all read-only lanes succeeds without invoking `CombineTree` with empty branches; mixed batch combines only write branches; worktrees for all completed lanes are removed in `completeIntegration`. | Unit tests in `integrate_test.go` with injected fake `Deps`. |
| Prompt Template | `packet-template.md` syntax, frontmatter keys, and criterion 2 formatting. | Verify template formatting in skill tests / linting. |

## Threat Matrix

| Boundary | Applicability | Reason |
|---|---|---|
| Documentation-like paths (executable Markdown, README.sh, etc.) | N/A | Frontmatter parsing and runtime validation only. |
| Git repository selection (`git -C`, relative/absolute paths) | Applicable | Read-only lanes still create worktrees; verified worktree removal in `completeIntegration` cleans up cleanly without modifying primary branch. |
| Commit state (staged, `commit -a`, empty index) | Applicable | Write lanes must provide commit receipts (`envelope.Commit != ""`); read-only lanes must leave working tree clean (`git status --porcelain` empty). |
| Push state (tracking branch, first push, explicit refspec) | N/A | No push operations performed by `lucind-ai run`. |
| PR commands (explicit `--head`, env prefix, composed commands) | N/A | No PR operations in this slice. |
| Unintended state mutation by read-only lanes | Applicable | `decideStatus` enforces `len(FilesChanged) == 0` for read-only lanes claiming `done`. |

## Migration / Rollout

1. **Phase 1 (Apply)**: Update `internal/packet/packet.go`, `internal/run/run.go`, `internal/run/integrate.go`, and `packet-template.md`.
2. **Backward Compatibility**: Fully non-breaking. Every existing packet omits `read_only:`, which defaults to `false`. Write packets already populate `commit` in `.lucind/result.json`.
3. **Rollout Verification**: Run unit tests (`go test ./...`) and dispatch an `explore` phase packet with `read_only: true` to confirm successful execution without commit generation.

## Rollback Plan

1. **Reversion**: Revert code commits touching `internal/packet/`, `internal/run/`, and `plugin/claude-code/skills/lucind-ai/assets/`.
2. **Database Impact**: Zero. No SQLite schema migrations or table modifications are made.
3. **Runtime Invariants**: Reverting restores the prior behavior where `Envelope.LaneStatus()` maps envelope status directly without commit checks. Existing worktree and ledger states remain completely uncorrupted.
