---
id: apply-agentic-phase-specialist
executor: agy
routed_by: single-packet sequential apply of an accepted three-unit tasks checklist under strict TDD; broad mechanical implementation favors agy per references/adapters/executors.md
model: gemini-3.7-flash-high
allowed_paths: ["plugin/claude-code/skills/lucind-ai/SKILL.md", "plugin/opencode/skills/lucind-ai/SKILL.md", "plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md", "plugin/opencode/skills/lucind-ai/references/strategies/fan-out.md", "plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md", "plugin/opencode/skills/lucind-ai/references/contracts/acceptance-promotion.md", "internal/accept", "internal/run", "openspec/changes/agentic-phase-specialist/tasks.md"]
---

# Packet apply-agentic-phase-specialist

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/apply-agentic-phase-specialist  ·  **Branch:** lucind/apply-agentic-phase-specialist

## Goal

Implement every task in `openspec/changes/agentic-phase-specialist/tasks.md` as three sequential
work-unit commits: (1) both skill trees' Hard Rule carve-out, fan-out synthesis-review reassignment,
and the decision-bearing Acceptance + `sdd_phase` checklist caveat in `acceptance-promotion.md`;
(2) `internal/accept` SDD-phase gating, RED then GREEN; (3) `internal/run` SDD-phase gating, RED
then GREEN. When finished, `go build ./...` and `go test ./... -race -count=1` are green on the
combined tree, both skill trees stay byte-identical, and every checkbox in `tasks.md` for units
1.1–1.4, 2.1–2.2, and 3.1–3.2 is ticked.

## Why this is safe to dispatch now

`proposal.md`, `design.md`, `specs/**/spec.md`, and `tasks.md` for `agentic-phase-specialist` are
all accepted and present in this worktree. `design.md` already names exact line numbers, exact
before/after text for the skill-tree Hard Rule, and the exact call sites (`accept.go:84-137`,
`attempt.go:431-448`) this packet touches. Do not re-derive any decision — copy the text and logic
below exactly.

## Preconditions

- Verify `openspec/changes/agentic-phase-specialist/{proposal,design,tasks}.md` and `specs/` exist
  and are accepted.
- Verify `./lucind-checks.sh` is green before you start (`go build ./...` then
  `go test ./... -race -count=1`). If it is not, return `blocked` — you did not break it.
- Verify `plugin/claude-code/skills/lucind-ai/SKILL.md` and its OpenCode mirror are still
  byte-identical before you start (`diff` them).

**A precondition satisfied by one of this packet's own later steps is a misordered packet.**
Return `blocked` and say so; do not work around it.

## Strict TDD is mandatory (`openspec/config.yaml: strict_tdd: true`)

Unit 2 and Unit 3 below each pair a RED test-writing step with a GREEN production step. For each,
write the failing test first, run the focused test command, actually observe it fail for the
intended reason, and only then write the production code that turns it green. Do not collapse a
RED+GREEN pair into work done before the RED test is observed failing.

## Three commits, not one

One commit per unit, in order 1 → 2 → 3. Each commit leaves the combined tree green — `Integrate`
reverts a red combined tree, and a unit that is red on its own cannot be reverted cleanly. Keep
both skill trees in Unit 1's single commit so `TestSkillTreesByteIdentical` is never a two-lane
race and never fails mid-packet. Before each commit, run the unit's focused test command below,
then `./lucind-checks.sh` on the full tree. Conventional commit messages. **No Co-Authored-By and
no AI attribution of any kind.**

Tick each task's checkbox (`1.1`–`1.4`, `2.1-RED`, `2.2`, `3.1-RED`, `3.2`) in `tasks.md` as you
complete it — do not edit any other content, ordering, or citation in that file.

---

## Unit 1 — Skill-tree operational contracts (commit 1)

Focused test: `go test ./internal/packet -run 'TestSkillTreesByteIdentical|TestSkillAssetContract' -count=1`

Edit **both** `plugin/claude-code/skills/lucind-ai/SKILL.md` and
`plugin/opencode/skills/lucind-ai/SKILL.md` identically for every change below (they must stay
byte-identical; diff them after every edit).

### 1.1 — Hard Rule carve-out (`SKILL.md` line 19)

Find this exact line under `## Hard Rules`:

```
- Keep one Orchestrator authoritative for the Change. Agents own Lanes, not scope, priorities, Dependencies, Acceptance, or Promotion.
```

Replace it with exactly:

```
- Keep one Orchestrator authoritative for the Change. Agents own Lanes, not scope, priorities, Dependencies, Acceptance, or Promotion — except that a named `sdd-*` phase-Specialist may independently Accept its own phase's Lanes; Promotion remains forbidden to every Agent, Specialist included.
```

Do not touch any other line of `SKILL.md`.

### 1.2 — Fan-out synthesis review moves to the Specialist (`references/strategies/fan-out.md`)

In both trees' `references/strategies/fan-out.md`, find this exact paragraph (under
`## Authority and compression`):

```
The Orchestrator reads synthesis notes: unresolved contradictions, coverage gaps, dropped citations, and phase divergence. A populated contradiction section requires human judgment. Verify every canonical citation; synthesis is the single point where hallucinated evidence can otherwise pass.
```

Replace it with exactly:

```
The Specialist reads synthesis notes: unresolved contradictions, coverage gaps, dropped citations, and phase divergence. It arbitrates persistent contradictions, resolving with `needs-revision` and exactly one bounded correction rather than a full re-fan-out. Verify every canonical citation; synthesis is the single point where hallucinated evidence can otherwise pass.
```

Do not change any other line of `fan-out.md`.

### 1.3 — Decision-bearing Specialist Acceptance + `sdd_phase` checklist caveat (`references/contracts/acceptance-promotion.md`)

In both trees' `references/contracts/acceptance-promotion.md`, make these two edits.

**(a) Checklist step 1** — find this exact step 1 line (it is one long line/paragraph):

```
1. **Mechanical acceptance automation**: Run `lucind-ai accept --run <run-id> --lane <lane-id>`, using the run and lane identifiers from the dispatch (`lucind-ai run` output / ledger). It loads the frozen done-candidate for that run and lane from the ledger — not the live branch — re-confirms the exact binding (packet digest, base and candidate commit/tree, `allowed_paths`), fails closed if any hard stop fired or a done criterion is unmet, then runs the repository checks (`lucind-checks.sh`) inside a verifier-owned detached worktree at the candidate commit and tears it down. On success it persists an immutable acceptance receipt and prints the receipt id, binding hash, and candidate commit; a missing candidate or failing checks exit nonzero with no receipt and no ref changes. The receipt is mechanical evidence only — never Promotion/CAS and never qualitative approval. Run `lucind-ai accept` with no flags for live usage rather than trusting cached syntax.
```

Replace it with exactly:

```
1. **Mechanical acceptance automation**: Run `lucind-ai accept --run <run-id> --lane <lane-id>`, using the run and lane identifiers from the dispatch (`lucind-ai run` output / ledger). It loads the frozen done-candidate for that run and lane from the ledger — not the live branch — re-confirms the exact binding (packet digest, base and candidate commit/tree, `allowed_paths`), fails closed if any hard stop fired or a done criterion is unmet, then — only when the lane's `sdd_phase` is `apply`, empty/missing, or carries an explicit exception — runs the repository checks (`lucind-checks.sh`) inside a verifier-owned detached worktree at the candidate commit and tears it down; a declared non-apply planning phase (e.g. `propose`, `design`) skips `lucind-checks.sh` and accepts on schema, hard stops, done criteria, and scope alone. On success it persists an immutable acceptance receipt and prints the receipt id, binding hash, and candidate commit; a missing candidate or failing checks exit nonzero with no receipt and no ref changes. The receipt is mechanical evidence only — never Promotion/CAS and never qualitative approval. Run `lucind-ai accept` with no flags for live usage rather than trusting cached syntax.
```

**(b) Checklist step 8** — find this exact line:

```
8. **Full-repo suite pass**: Ensure mechanical checks cover the whole repository, not only the touched package.
```

Replace it with exactly:

```
8. **Full-repo suite pass**: When the lane's `sdd_phase` is `apply`, empty/missing, or carries an explicit exception, ensure mechanical checks cover the whole repository, not only the touched package. A declared non-apply planning phase skips this step; schema, hard stops, done criteria, and scope still apply.
```

**(c) Acceptance subagent delegation section** — find this exact section (heading through its
three bullets):

```
## Acceptance subagent delegation

To protect the Orchestrator's context window across multi-wave sessions, the Orchestrator may delegate steps 1–9 to an ephemeral Acceptance Subagent:
- Prompt consists strictly of the acceptance checklist.
- Tools are restricted to `Read`, `Grep`, and `Bash` within a scoped worktree.
- Subagent returns structured evidence (diffstat, test semantics, envelope audit, check logs) without inflating the Orchestrator's transcript.
```

Replace it with exactly:

```
## Specialist and subagent acceptance delegation

A named `sdd-*` phase-Specialist independently judges and executes Acceptance (steps 1–9) for its own phase's Lanes, without requesting human confirmation — this is decision-bearing Acceptance, not evidence-only delegation. Ordinary delegated workers MUST NOT judge Acceptance. To protect the Orchestrator's context window across multi-wave sessions outside that Specialist path, the Orchestrator may instead delegate steps 1–9 to an ephemeral, non-deciding Acceptance Subagent that returns structured evidence only:
- Prompt consists strictly of the acceptance checklist.
- Tools are restricted to `Read`, `Grep`, and `Bash` within a scoped worktree.
- Subagent returns structured evidence (diffstat, test semantics, envelope audit, check logs) without inflating the Orchestrator's transcript; the Orchestrator, not the subagent, still judges Acceptance.
```

Do not change any other line of `acceptance-promotion.md`.

### 1.4 — Verify byte-identity and glossary lockstep

After 1.1–1.3 land in both trees, run:
`go test ./internal/packet -run 'TestSkillTreesByteIdentical|TestSkillAssetContract' -count=1`
Both must pass. If they do not, your two trees have diverged — fix the mismatch before committing
Unit 1. Do not touch `CONTEXT.md` or `domain.md`; they were already updated with the
Specialist/Phase Verdict glossary terms in a prior session and must not change here.

Commit Unit 1 now (all six files, one commit) before starting Unit 2.

---

## Unit 2 — `internal/accept` SDD-phase gating (commits 2a RED, then 2b GREEN — may be one commit
containing both the failing-then-passing test and the production fix, but you must actually run
the focused test and observe RED before writing the GREEN code)

Focused test: `go test ./internal/accept -race -count=1`

### 2.1-RED — Add failing tests to `internal/accept/accept_test.go`

Add these three test functions (they use the existing `newVerifierFixture` helper already in this
file). Run the focused test command and confirm
`TestVerifierSkipsChecksForDeclaredNonApplyPhase` fails today (checks always run, so a checks
script that exits non-zero currently rejects the candidate).

```go
func TestVerifierSkipsChecksForDeclaredNonApplyPhase(t *testing.T) {
	f := newVerifierFixture(t, validResult("allowed.txt"), "#!/bin/sh\nexit 7\n", map[string]string{"allowed.txt": "candidate\n"}, []string{"allowed.txt"})
	if err := f.ledger.UpdateLaneMetadata(context.Background(), ledger.LaneMetadata{RunID: "run-1", LaneID: "lane-1", SDDPhase: "propose"}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.verifier.Verify(context.Background(), AcceptanceRequest{"run-1", "lane-1"}); err != nil {
		t.Fatalf("Verify() with declared non-apply sdd_phase should skip the failing checks script: %v", err)
	}
}

func TestVerifierRunsChecksForApplyEmptyOrMissingSDDPhase(t *testing.T) {
	tests := []struct {
		name     string
		setPhase bool
		phase    string
	}{
		{name: "declared apply", setPhase: true, phase: "apply"},
		{name: "explicit empty sdd_phase", setPhase: true, phase: ""},
		{name: "missing lane metadata", setPhase: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newVerifierFixture(t, validResult("allowed.txt"), "#!/bin/sh\nexit 7\n", map[string]string{"allowed.txt": "candidate\n"}, []string{"allowed.txt"})
			if tt.setPhase {
				if err := f.ledger.UpdateLaneMetadata(context.Background(), ledger.LaneMetadata{RunID: "run-1", LaneID: "lane-1", SDDPhase: tt.phase}, time.Now().UTC()); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := f.verifier.Verify(context.Background(), AcceptanceRequest{"run-1", "lane-1"}); err == nil {
				t.Fatalf("Verify() should still run checks and reject a failing script for %s", tt.name)
			}
		})
	}
}

func TestVerifierNonApplyPhaseStillEnforcesScope(t *testing.T) {
	f := newVerifierFixture(t, validResult("allowed.txt"), "#!/bin/sh\necho checks-ok\n", map[string]string{"allowed.txt": "candidate\n"}, []string{"other.txt"})
	if err := f.ledger.UpdateLaneMetadata(context.Background(), ledger.LaneMetadata{RunID: "run-1", LaneID: "lane-1", SDDPhase: "propose"}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.verifier.Verify(context.Background(), AcceptanceRequest{"run-1", "lane-1"}); err == nil {
		t.Fatal("Verify() with a declared non-apply sdd_phase still accepted an out-of-scope change")
	}
}
```

### 2.2-GREEN — Gate `internal/accept/accept.go`

In `Verifier.Verify` (`accept.go:62`), the existing code loads `LaneMetadata` only inside the
`AuthoringEvidenceVersion` branch (around current lines 84–96), then always calls
`integrate.CheckPolicySnapshot()` and `v.check` (around current lines 120–137). Change it so:

1. `v.ledger.GetLaneMetadata(ctx, candidate.RunID, candidate.LaneID)` is called **unconditionally**,
   immediately after `validateObjects` succeeds and before the `AuthoringEvidenceVersion` check —
   not only inside that branch. Keep returning
   `fmt.Errorf("accept: load frozen target metadata: %w", err)` on error, exactly as today.
2. The `AuthoringEvidenceVersion` branch keeps decoding evidence and calling
   `validateTypedTargetBinding(evidence.Binding, metadata)` exactly as today, just using the
   metadata loaded above instead of loading it again inline.
3. `validateResultAndScope`, `v.binding`, `bindingHash`, and the `FindAcceptanceReceipt` cache
   lookup stay exactly where they are today, unconditional.
4. `createOwnedIsolation` stays unconditional, exactly as today.
5. Only `integrate.CheckPolicySnapshot()` and `v.check(...)` (and their surrounding
   `checkCtx`/`cancel`/error-mapping) become conditional on
   `runSDDPhaseChecks := metadata.SDDPhase == "" || metadata.SDDPhase == "apply"`. When
   `runSDDPhaseChecks` is true, behavior is byte-identical to today (same errors, same
   `ChecksHash` derivation from `version` and `output`). When false, skip
   `CheckPolicySnapshot`/`v.check` entirely, still call `cleanupOwnedIsolation` (returning its
   error exactly as today if it fails), and build the receipt's `ChecksHash` from empty
   `version`/`output` strings (`hashValues("checks:v1", "", "")`).
6. Do not modify `integrate.Check` (`internal/integrate/integrate.go`) at all — it stays an
   ungated primitive. Do not touch `validateVersionedEvidence`, `validateResultAndScope`, or any
   function below `Verify` in this file.

After this change, run the focused test command again; all three new tests plus every pre-existing
test in `internal/accept` must pass.

Commit Unit 2 now (both `accept.go` and `accept_test.go`, one commit, containing the now-passing
RED tests and the GREEN fix) before starting Unit 3.

---

## Unit 3 — `internal/run` SDD-phase gating (RED then GREEN, one commit)

Focused test: `go test ./internal/run -run 'TestExecuteAttempt' -count=1`

### 3.1-RED — Add failing tests to `internal/run/attempt_test.go`

Add `"github.com/LanzerDevCorp/lucind-ai/internal/lane"` to this file's import block (it is not
currently imported there; `internal/ledger` already is). Then add these two test functions, which
use the existing `newAttemptTestDeps`/`attemptSpies` fixtures already in this file. Run the
focused test command and confirm `TestExecuteAttemptSkipsChecksForDeclaredNonApplyLanes` fails
today (`checkFunc` is always called, so `checkCalls` is 1, not the asserted 0).

```go
func TestExecuteAttemptSkipsChecksForDeclaredNonApplyLanes(t *testing.T) {
	spies := &attemptSpies{}
	deps, l, featSvc := newAttemptTestDeps(t, spies)

	featID := "feat-non-apply-1"
	if _, err := featSvc.Create(context.Background(), featID, "refs/heads/feature-non-apply", "base-sha-non-apply", "expected-parent-sha-1"); err != nil {
		t.Fatalf("featSvc.Create() error = %v", err)
	}
	if err := l.RegisterLane(context.Background(), ledger.Lane{RunID: deps.RunID, LaneID: "lane-1", PacketID: "lane-1", Executor: "agy", RoutingCondition: "test", Status: lane.Running}); err != nil {
		t.Fatalf("RegisterLane() error = %v", err)
	}
	if err := l.UpdateLaneMetadata(context.Background(), ledger.LaneMetadata{RunID: deps.RunID, LaneID: "lane-1", SDDPhase: "propose"}, time.Now().UTC()); err != nil {
		t.Fatalf("UpdateLaneMetadata() error = %v", err)
	}

	req := run.AttemptRequest{
		ID:                "att-non-apply-1",
		FeatureID:         featID,
		ParentRef:         "refs/heads/feature-non-apply",
		BaseSHA:           "base-sha-non-apply",
		ExpectedParentSHA: "expected-parent-sha-1",
		IdempotencyKey:    "key-non-apply-1",
		Owner:             "owner-non-apply",
		Branches:          []string{"lucind/lane-1"},
	}

	res, err := run.ExecuteAttempt(context.Background(), deps, req)
	if err != nil {
		t.Fatalf("ExecuteAttempt() error = %v", err)
	}
	if res.Status != run.AttemptStatusPromoted {
		t.Fatalf("res.Status = %v, want %v", res.Status, run.AttemptStatusPromoted)
	}

	spies.mu.Lock()
	checkCalls := len(spies.checkCalls)
	spies.mu.Unlock()
	if checkCalls != 0 {
		t.Fatalf("checkFunc called %d times, want 0 for a declared non-apply combined lane", checkCalls)
	}
}

func TestExecuteAttemptRunsChecksForApplyEmptyOrMissingCombinedLane(t *testing.T) {
	tests := []struct {
		slug         string
		registerLane bool
		setMetadata  bool
		sddPhase     string
	}{
		{slug: "apply", registerLane: true, setMetadata: true, sddPhase: "apply"},
		{slug: "empty", registerLane: true, setMetadata: true, sddPhase: ""},
		{slug: "missing", registerLane: false, setMetadata: false},
	}
	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			spies := &attemptSpies{}
			deps, l, featSvc := newAttemptTestDeps(t, spies)
			featID := "feat-run-checks-" + tt.slug
			if _, err := featSvc.Create(context.Background(), featID, "refs/heads/feature-run-checks-"+tt.slug, "base-sha-run-checks-"+tt.slug, "expected-parent-sha-1"); err != nil {
				t.Fatalf("featSvc.Create() error = %v", err)
			}
			if tt.registerLane {
				if err := l.RegisterLane(context.Background(), ledger.Lane{RunID: deps.RunID, LaneID: "lane-1", PacketID: "lane-1", Executor: "agy", RoutingCondition: "test", Status: lane.Running}); err != nil {
					t.Fatalf("RegisterLane() error = %v", err)
				}
			}
			if tt.setMetadata {
				if err := l.UpdateLaneMetadata(context.Background(), ledger.LaneMetadata{RunID: deps.RunID, LaneID: "lane-1", SDDPhase: tt.sddPhase}, time.Now().UTC()); err != nil {
					t.Fatalf("UpdateLaneMetadata() error = %v", err)
				}
			}
			req := run.AttemptRequest{
				ID:                "att-run-checks-" + tt.slug,
				FeatureID:         featID,
				ParentRef:         "refs/heads/feature-run-checks-" + tt.slug,
				BaseSHA:           "base-sha-run-checks-" + tt.slug,
				ExpectedParentSHA: "expected-parent-sha-1",
				IdempotencyKey:    "key-run-checks-" + tt.slug,
				Owner:             "owner-run-checks",
				Branches:          []string{"lucind/lane-1"},
			}
			res, err := run.ExecuteAttempt(context.Background(), deps, req)
			if err != nil {
				t.Fatalf("ExecuteAttempt() error = %v", err)
			}
			if res.Status != run.AttemptStatusPromoted {
				t.Fatalf("res.Status = %v, want %v", res.Status, run.AttemptStatusPromoted)
			}
			spies.mu.Lock()
			checkCalls := len(spies.checkCalls)
			spies.mu.Unlock()
			if checkCalls != 1 {
				t.Fatalf("checkFunc called %d times, want 1 for %s", checkCalls, tt.slug)
			}
		})
	}
}
```

### 3.2-GREEN — Gate `internal/run/attempt.go`

In `driveAttemptFromLeased`'s CHECKING phase (current lines ~414–449), add an unexported helper:

```go
// shouldRunAttemptChecks resolves whether this attempt's CHECKING phase must
// execute checkFunc, from the SDDPhase each combined lane declared at
// dispatch time (ledger.LaneMetadata.SDDPhase, written via UpdateLaneMetadata
// in internal/run/run.go). Checks run unless every combined lane is a
// declared non-apply phase: an "apply" phase, an empty/missing sdd_phase, or
// metadata that cannot be resolved all fail closed to running checks — a
// conservative extension of design.md's Decision 2, matching
// internal/accept's gate.
func shouldRunAttemptChecks(ctx context.Context, deps Deps, branches []string) bool {
	if deps.Ledger == nil || len(branches) == 0 {
		return true
	}
	for _, branch := range branches {
		laneID := strings.TrimPrefix(branch, "lucind/")
		metadata, err := deps.Ledger.GetLaneMetadata(ctx, deps.RunID, laneID)
		if err != nil || metadata.SDDPhase == "" || metadata.SDDPhase == "apply" {
			return true
		}
	}
	return false
}
```

Then, in the CHECKING block, keep `checkLeaseTTL`, `renewInterval`, and the
`startLeaseRenewal`/`stopLeaseRenewal` pair exactly as they are today (do not gate lease renewal
itself — leave it wrapping unconditionally, per `design.md`/`tasks.md`). Only gate whether
`checkFunc` is actually invoked inside that renewal window:

```go
	runSDDPhaseChecks := shouldRunAttemptChecks(ctx, deps, req.Branches)
	stopLeaseRenewal := startLeaseRenewal(ctx, featSvc, att, checkLeaseTTL, renewInterval)
	var passed bool
	var output string
	if runSDDPhaseChecks {
		passed, output, err = checkFunc(ctx, wtPath)
	} else {
		passed, output, err = true, "", nil
	}
	stopLeaseRenewal()
```

(This replaces today's `passed, output, err := checkFunc(ctx, wtPath)` line between
`startLeaseRenewal` and `stopLeaseRenewal`, using `=` not `:=` for `passed`/`output`/`err` since
they are declared just above.) Everything from `if err != nil || !passed {` onward stays exactly
as it is today. Do not modify `integrate.Check` or anything in `internal/run/integrate.go`.

After this change, run the focused test command; both new tests plus every pre-existing test in
`internal/run` (`go test ./internal/run -race -count=1`) must pass.

Commit Unit 3 now (both `attempt.go` and `attempt_test.go`, one commit).

---

## Out of scope

- Do not touch `internal/ledger/schema.go`, `internal/ledger/authoring.go`, or any migration.
- Do not touch `internal/integrate/integrate.go`.
- Do not touch `CONTEXT.md`, `domain.md`, or any file outside this packet's `allowed_paths`.
- Do not attempt to write `~/.claude/skills/sdd-*/SKILL.md` (outside the repository; a human pastes
  the `design.md:102-106` prompt there separately — this is explicitly not this packet's job).
- Do not add any `--force-checks` flag or new `LaneMetadata` field; the exception path named in
  `design.md`/`tasks.md` has no concrete schema this Change — only `apply`/empty/missing are gated.

## Done criteria

- [ ] Both skill trees (`plugin/claude-code/skills/lucind-ai/` and `plugin/opencode/skills/lucind-ai/`)
      carry identical Hard Rule, fan-out, and acceptance-promotion edits; `diff` them to confirm.
- [ ] `go test ./internal/packet -run 'TestSkillTreesByteIdentical|TestSkillAssetContract' -count=1`
      passes.
- [ ] `go test ./internal/accept -race -count=1` passes, including the three new tests.
- [ ] `go test ./internal/run -race -count=1` passes, including the two new tests.
- [ ] `go build ./...` and `go test ./... -race -count=1` (the full suite) both pass on the final
      combined tree.
- [ ] Three conventional commits exist, one per unit, no AI attribution. Evidence:
      `git log --oneline -3` and `git status --porcelain` empty.
- [ ] Tasks `1.1`, `1.2`, `1.3`, `1.4`, `2.1-RED`, `2.2`, `3.1-RED`, `3.2` are ticked `[x]` in
      `openspec/changes/agentic-phase-specialist/tasks.md`; no other content in that file changed.
- [ ] For each RED task, name the focused test command and the exact failure you observed before
      writing the GREEN code.

## Hard stops

- [ ] Stop `blocked` if the two skill trees cannot be kept byte-identical.
- [ ] Stop `blocked` if gating `v.check`/`checkFunc` cannot be done without touching
      `integrate.Check` itself.
- [ ] Stop `blocked` for out-of-scope edits, impossible criteria, unresolved alternatives, or
      conflicting instructions between this packet and `design.md`/`tasks.md` (cite the conflict).

## Return

Write the result envelope to **.lucind/result.json in this worktree**. That file is what the
dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at .lucind/result.schema.json in this worktree. Validate against it before writing —
an envelope that fails schema validation makes the lane `blocked` regardless of how well the work
went.
