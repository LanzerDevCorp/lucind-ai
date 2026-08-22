# Spec Lens C — Live-Spec Conflicts & Migration: Control Room UI Views

## Assumed requirements

This change modifies 1 existing capability (`approvals-web-ui`) and introduces 4 new capabilities (`batch-wave-view`, `feature-lease-monitor`, `reconciliation-workspace`, `lane-envelope-inspector`). The assumed requirement set asserted by `proposal.md` covers: (1) `Anti-rubber-stamping in the multi-view shell` (`proposal.md:61-74`), asserting individual selection, one-item POST, HTTP 400 rejection on bulk payloads, inline command output or `file:line` evidence display with fallback notes, approver wrong-approval rate, and the opencode review command; (2) `Batch and DAG wave inspection` (`proposal.md:75-88`), asserting inspection of batch status, DAG waves, lane statuses, executors, worktrees, deadlines, and barrier release/preservation; (3) `Shell-free feature and lease monitoring` (`proposal.md:89-97`), asserting `serve.Model` reads for feature state, lease owner/fence, attempts, and overlap evidence; (4) `Reconciliation candidate inspection` (`proposal.md:98-106`), asserting read-only inspection of reconciliation requests, candidate diffs, checks, and CAS promotion results; and (5) `Lane demotion diagnosis` (`proposal.md:107-115`), asserting display of `deviated` status, offending-path notes, and preserved worktrees.

## Live Spec Inventory

| Capability | Live spec (file:line) | Requirements | Scenarios | Touched by this change |
|---|---|---|---|---|
| `approvals-web-ui` | `openspec/specs/approvals-web-ui/spec.md:1-83` | 4 | 9 | 3 requirements (7 scenarios) modified/preserved in multi-panel shell (`:26-83`); 1 requirement (2 scenarios) untouched (`Loopback Binding`, `:10-25`) |

## Conflicts

None. The change preserves all live guarantees from `openspec/specs/approvals-web-ui/spec.md:10-83` (loopback binding, individual decisions without bulk controls, HTTP 400 bulk rejection, inline command output/`file:line` validation with bare-claim withholding, and personal wrong-approval rate calculation). The change modifies the presentation context from a single-purpose approvals page to a multi-panel dashboard shell with lazy panel loading and restricted hot polling (`GET /api/approvals`, `/api/leases`, `/api/batch/lanes`), but makes no existing behavioral guarantee untrue.

## MODIFIED Full Blocks

### Requirement: Individual Decisions Without Bulk Approval

**Source**: `openspec/specs/approvals-web-ui/spec.md:26` — 3 scenarios

Items MUST start unselected. Every decision MUST be made individually; the UI MUST NOT provide a
bulk/"approve all" control, and the server MUST reject a multi-item request.

#### Scenario: Fresh load starts unselected

- GIVEN pending items
- WHEN the page loads
- THEN every item MUST be unselected.

#### Scenario: Unselected item cannot be decided

- GIVEN an unselected item
- WHEN a decision is submitted for it
- THEN the system MUST NOT record `approved` or `rejected`.

#### Scenario: Bulk request rejected

- GIVEN multiple pending items
- WHEN a multi-item approval request is posted
- THEN the server MUST reject it; the UI MUST NOT expose a control that could produce one.

### Requirement: Inline Evidence and Batch Review Command

**Source**: `openspec/specs/approvals-web-ui/spec.md:49` — 2 scenarios

Evidence MUST be command output or `file:line`, never a bare claim. The merged-batch view MUST
show the exact `opencode` RDD command to run.

#### Scenario: Evidence and command visible

- GIVEN pending items with command output or `file:line` evidence, and a merged batch ready for
  review
- WHEN the user opens the UI
- THEN evidence MUST render inline and the exact `opencode` command MUST be shown.

#### Scenario: Bare claim withheld

- GIVEN an item with neither command output nor a `file:line` reference
- WHEN the UI renders it
- THEN the system MUST NOT present an unsupported claim as evidence.

### Requirement: Approver Wrong-Approval Rate

**Source**: `openspec/specs/approvals-web-ui/spec.md:67` — 2 scenarios

The UI MUST show the signed-in user's own rate of approvals that later surfaced a defect in the
same packet — never another approver's rate.

#### Scenario: Zero defect history

- GIVEN an approver with zero flagged defects
- WHEN viewing their rate
- THEN the UI MUST display 0%.

#### Scenario: Own rate only

- GIVEN another approver has marked defects
- WHEN the current user opens the UI
- THEN those rows MUST NOT count toward the current user's displayed rate.

## Removals and Renames

| Requirement | Removed or renamed | Reason | Consumers (file:line) | Migration |
|---|---|---|---|---|
| `Individual Decisions Without Bulk Approval`, `Inline Evidence and Batch Review Command`, `Approver Wrong-Approval Rate` | Potential Consolidation / Rename to `Anti-rubber-stamping in the multi-view shell` | Proposal §Delta specifications (`proposal.md:61-74`) unifies the approvals panel rules into a single requirement statement. | `internal/serve/handlers.go:16-21, 120-211`; `internal/serve/server_test.go:42-194`; `internal/serve/static_test.go:11-102`; `internal/serve/static/app.js:12-70`; `docs/prd.md:217-241` | If synthesizer consolidates the 3 requirements into `Anti-rubber-stamping in the multi-view shell`, keep all 7 underlying scenarios and existing test assertions intact; no behavioral migration required. |

## Open Questions

- [ ] Spec requirement consolidation: Proposal §Delta specifications defines a single requirement `Anti-rubber-stamping in the multi-view shell` (`proposal.md:61-74`) covering individual selection, inline evidence, opencode command, and wrong-approval rate. The synthesizer must arbitrate whether to replace the three live requirements (`Individual Decisions Without Bulk Approval`, `Inline Evidence and Batch Review Command`, `Approver Wrong-Approval Rate` at `openspec/specs/approvals-web-ui/spec.md:26-83`) with this single consolidated requirement or retain them as three distinct MODIFIED requirements in the delta spec.
- [ ] Reconciliation mutation in UI: `proposal.md:152` leaves open whether the UI may POST reconcile `approve`/`decline`/`cancel`/`renew`/`resolve` or only display copy-paste CLI commands matching `cmd/lucind-ai/cli.go:1044-1065`. Live spec `openspec/specs/reconciliation-approval/spec.md:33-46` defines direction approvals, but HTTP handlers currently lack reconciliation mutation routes (`internal/serve/handlers.go:36-118`).
- [ ] Overlap evidence rendering: `proposal.md:153` leaves open whether `evidence_json` (`internal/serve/model.go:68`) renders as `<pre>` + `escapeHtml` (`internal/serve/static/app.js:53, 91-94`) or via a zero-dependency inline diff tokenizer.
- [ ] Expiry countdown source: `proposal.md:154` leaves open whether lease/reconcile countdowns use `remaining_seconds` calculated server-side or `expires_at` with client-side comparison against server time.
