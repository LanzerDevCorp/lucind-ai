# Dependencies and defects

Load this module when a defect, blocker, cross-Change effect, or Dependency appears.

## Structured ultrafixer defect triage coordination

When a test, linter, or build failure is encountered during feature development, the Orchestrator generates and dispatches an on-demand ultrafixer packet (`plugin/claude-code/skills/lucind-ai/assets/ultrafixer-packet-template.md`) via `lucind-ai run --packet` rather than performing manual triage or creating ad-hoc fix Changes.

The ultrafixer lifecycle follows a structured protocol:

1. **Origin classification via `base_sha` diffing**:
   - Ultrafixer diffs the failure context against the target feature's immutable `base_sha` (`internal/packet/packet.go:68`; `internal/ledger/ledger.go`).
   - If the defect was introduced on the current feature branch between `base_sha` and `HEAD`, ultrafixer exits with status `done` and an explanatory summary. The feature lane remains responsible for fixing its own regression without a separate fix Change.
   - If the defect pre-existed `base_sha`, ultrafixer proceeds to two-axis evaluation.

2. **Independent two-axis evaluation**:
   - **Global critical severity**: Security vulnerability, data loss or corruption hazard, or total CI/build failure.
   - **Blocking impact**: Evaluated separately for the originating branch and every active feature branch discovered via `lucind-ai feature status`.
   - If non-critical and non-blocking for all active features, ultrafixer records a Defect Record with disposition `recorded` in the ledger via `lucind-ai defect record` and exits `done` without code changes.

3. **Signal reproduction for cross-branch impact**:
   - CodeGraph (`codegraph impact`/`codegraph affected`) acts strictly as a candidate filter.
   - Cross-branch impact MUST be verified by reproducing the failing check command in the candidate branch worktree before marking a branch as affected or blocked. Syntactic overlap without failure reproduction is never marked as blocked.

4. **Isolated repair delivery and human-gated CAS promotion**:
   - Ultrafixer repairs pre-existing critical or blocking defects in an isolated git worktree (`../<repo>-worktrees/<lane-id>`).
   - Repairs are committed using conventional commit formatting with zero AI attribution or `Co-Authored-By` trailers.
   - Ultrafixer emits a `blocked` result envelope containing the repair commit, affected files, and distinct `Question` and `Finding` entries encoding per-branch recommendations and evidence.
   - Ultrafixer MUST NOT auto-integrate or push fixes. The Orchestrator presents the `blocked` result envelope to the human operator for decision and CAS promotion via `lucind-ai integrate retry`.
   - If the operator accepts the fix, the Orchestrator runs `lucind-ai integrate retry` to promote it. If the operator declines, the Orchestrator records the decision with `lucind-ai defect decline --id <id>`, which transitions the Defect Record's disposition to `declined` in the ledger.
   - Repair branches and worktrees remain preserved on disk upon `blocked` emission or operator decline.

## Evidence boundary

Runtime evidence includes packet scope, Lane status and reason, result envelopes, check output, ledger events, defect records (`lucind-ai defect list --feature <id>`), integration attempts, overlap evidence, reconciliation records, and git objects.

A `blocked` result envelope must include the decision question and recommendation. The Orchestrator reviews the evidence and recommendation and presents them to the human operator for CAS promotion via `lucind-ai integrate retry` rather than guessing or self-integrating.

