# Spec Lens C — Live-Spec Conflicts & Migration: Control Room UI Shell

## Assumed requirements

This change introduces five new capabilities (`control-room-ui-shell`, `control-room-client-routing`, `control-room-shared-store`, `control-room-asset-embed`, `control-room-model-queries`) to transform `lucind-ai serve` into a zero-build, embed-only, read-only multi-view shell with hash routing, persistent header metrics, and a shared client store. It modifies one existing capability (`approvals-web-ui`), converting the approvals inbox from a monolithic, full-page polling container into a mountable client-routed view inside `#view-outlet`. The change asserts that existing security and workflow invariants—strict loopback-only binding (`127.0.0.1`), individual decision recording starting unselected, rejection of bulk/multi-item requests (HTTP 400), inline `file:line` or command-output evidence, and per-approver wrong-approval rate tracking—remain fully preserved without semantic regression.

## Live Spec Inventory

| Capability | Live spec (file:line) | Requirements | Scenarios | Touched by this change |
|---|---|---|---|---|
| `approvals-web-ui` | `openspec/specs/approvals-web-ui/spec.md:1` | 4 | 9 | Yes. Inbox transitions from standalone page to mountable view in `#view-outlet`; approver wrong-approval metric moves to persistent global header chrome; DOM patching replaces full `innerHTML` wipe on poll ticks; loopback listen, individual unselected decisions, inline evidence, and anti-bulk rules remain unchanged. |

## Conflicts

None.

This change does not contradict or invalidate any live requirement in `openspec/specs/approvals-web-ui/spec.md`:
- `Loopback Binding` (`openspec/specs/approvals-web-ui/spec.md:10-25`): Unchanged. Server continues to bind exclusively to `127.0.0.1` and rejects non-loopback addresses with `ErrNonLoopback`.
- `Individual Decisions Without Bulk Approval` (`openspec/specs/approvals-web-ui/spec.md:26-48`): Preserved. Items in the approvals view start unselected upon mounting, decisions are submitted individually via `POST /approvals/{runID}/{laneID}`, and multi-item/bulk payloads continue to return HTTP 400.
- `Inline Evidence and Batch Review Command` (`openspec/specs/approvals-web-ui/spec.md:49-66`): Preserved. The approvals view renders inline `file:line` or command-output evidence and displays the `opencode` review command for merged batches.
- `Approver Wrong-Approval Rate` (`openspec/specs/approvals-web-ui/spec.md:67-83`): Preserved. The approver's personal defect rate continues to be calculated from `ServerState` and displayed for the signed-in user only, transitioning presentationally from the local inbox layout to the persistent layout shell header.

## MODIFIED Full Blocks

### Requirement: Loopback Binding

**Source**: `openspec/specs/approvals-web-ui/spec.md:10` — 2 scenarios

The server MUST bind only to `127.0.0.1` and MUST reject a non-loopback `--addr`.

#### Scenario: Loopback listen

- GIVEN loopback address `127.0.0.1:7433`
- WHEN starting `lucind-ai serve`
- THEN the server MUST listen on loopback.

#### Scenario: Non-loopback rejected

- GIVEN non-loopback address `0.0.0.0:7433`
- WHEN starting `lucind-ai serve --addr 0.0.0.0:7433`
- THEN the server MUST reject binding and exit with an error.

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
| None | None | No live requirements are removed or renamed. All 4 live requirements in `approvals-web-ui` retain their full functional and security contracts. | N/A | None |

### Reference Inventory of Live Consumers

For synthesis and migration verification, existing consumers of the live `approvals-web-ui` capability across the codebase include:
- `internal/serve/server.go:12-22,55-73` (`ErrNonLoopback`, `ListenAndServe`, `IsLoopback` implementation)
- `internal/serve/handlers.go:36-118,130-230` (`NewHandler`, static file serving, `/api/state`, `/approvals/{runID}/{laneID}`, bulk payload rejection)
- `internal/serve/server_test.go:17-40` (`TestNonLoopbackListenFails` validating Loopback Binding)
- `internal/serve/server_test.go:42-93` (`TestBulkRequestBodyReturns400` validating anti-bulk enforcement)
- `internal/serve/server_test.go:95-134` (`TestUnselectedDecisionReturns400` validating individual unselected state)
- `internal/serve/server_test.go:136-194` (`TestSingleApprovalAndDefectEndpoints` validating decide and defect endpoints)
- `internal/serve/server_test.go:196-236` (`TestDecideAlreadyDecidedReturns409Conflict` validating conflict handling)
- `internal/serve/static_test.go:11-39` (`TestEmbedFSHasNoApproveAllControl` validating absence of bulk approval terms)
- `internal/serve/static_test.go:41-67` (`TestStaticAssetsContainOpencodeCommandAndInlineEvidence` validating command and evidence rendering)
- `internal/serve/static_test.go:69-81` (`TestItemsStartUnselectedInUI` validating unselected initial state)
- `internal/serve/static_test.go:83-102` (`TestStaticEvidenceValidationRejectsBareMultilineProse` validating strict evidence checks)
- `docs/prd.md:217-241` (Section 8.3 defining the four immutable approvals rules)
- `cmd/lucind-ai/cli.go:674-725` (`serveDispatch` CLI entry point with `--addr`, `--approver`, `--approval-timeout`)

## Open Questions

- [ ] Scope boundary for Features view: Does `control-room-ui-shell` ship a placeholder/stub read-only Features view (`#/features`) in the shell view registry, or only the shell + Approvals view + Model GET endpoints, deferring the complete Features UI to `control-room-ui-views` (`proposal.md:170`)?
- [ ] Router fallback contract: Should client-side routing remain strictly hash-based (`#/approvals`) without server-side catch-all handling on `serve.NewHandler` (`proposal.md:171`)?
- [ ] Metric location in delta spec synthesis: Should `Approver Wrong-Approval Rate` remain defined under `approvals-web-ui` with a note on its header placement, or be co-specified under `control-room-ui-shell` (`proposal.md:15`)?
