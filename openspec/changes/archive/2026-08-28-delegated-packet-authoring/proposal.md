# Proposal: Delegated Packet Authoring

## Intent

Packet prose is a weak machine protocol: packets can omit result instructions, contradict metadata, and report criteria or stops that Acceptance cannot match to the assignment. Introduce a safe, measurable contract seam while preserving manual authoring. Contracts make specialist experimentation safe; canonical cutover requires a separate evidence-gated Change.

## Scope

### In Scope
- Validate metadata/body safety before worktree or quota allocation.
- Compile versioned, target-free contracts deterministically; bind live targets only at dispatch.
- Preserve manual prompt bytes except universal result path/schema and contradiction checks.
- Share criteria, stops, mode, commit, and changed-path semantics with result/Acceptance.
- Shadow a permission-bounded specialist that emits typed data only; record validity, semantic-equivalence, digest stability, latency/review cost, and failure/fallback classes.

### Out of Scope
- Automatic specialist cutover, manual-path removal, specialist-selected SHAs/side effects, or unrelated rewrites.

## Capabilities

### New Capabilities
- `packet-authoring-contract`: Typed compilation, late binding, diagnostics, and compatibility.
- `delegated-packet-author-shadow`: Bounded invocation, comparison evidence, metrics, and manual fallback.

### Modified Capabilities
- `lane-execution`: Admit in `cmd/lucind-ai.runDispatch` before `internal/run.ExecuteBatch`; freeze candidate evidence.
- `read-only-packet-schema`: Preserve and expose declared read-only input paths.
- `acceptance-verifier`: Enforce exact authored-result correspondence.
- `allowed-paths-enforcement`: Share diff and change classifications.

## Approach

Add a deep `internal/packetauthor` module exposing `Compile(Contract, TargetBinding) (Artifact, error)`. Manual and specialist adapters feed it; only the compiler renders. Timeout, invalid data, unavailable routing, and fallback-agent detection remain observations and never block canonical manual dispatch. This Change's 3000-line single-PR budget overrides stale 2000-line config; reconcile before tasks without changing unrelated defaults.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/packetauthor/` | New | Compiler and comparison seam |
| `internal/{packet,run,result,accept,ledger}/` | Modified | Admission, evidence, and correspondence |
| `cmd/lucind-ai/cli.go` | Modified | Thin compile/validate/shadow adapter |
| `.opencode/agent/`, `plugin/claude-code/skills/lucind-ai/` | Modified | Specialist and guidance |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Contract mirrors prose or breaks legacy packets | Med | Narrow invariants; versioning; fixtures |
| Ledger migration or scope exceeds 3000 lines | Med | Additive evidence schema; forecast before apply |

## Rollback Plan

Disable shadow invocation first; manual packets remain canonical. Revert compiler/admission and additive evidence persistence without converting stored manual packets.

## Dependencies

- Reconcile the budget; design additive persistence if normalized evidence requires migration.

## Success Criteria

- [ ] Unsafe packets fail before dispatch with actionable diagnostics.
- [ ] Repeated compilation yields byte-identical output and stable digests.
- [ ] Versioned results correspond exactly to frozen authored contracts.
- [ ] Manual packets remain dispatchable except universal safety violations.
- [ ] Shadow evidence reports all stated measures while manual dispatch remains authoritative.
