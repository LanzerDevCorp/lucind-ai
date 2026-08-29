# Spec Synthesis Notes: Deterministic lucind-ai Orchestrator

## Unresolved Contradictions

None. Lens B's "four existing capabilities" claim is settled by this worktree's `openspec/specs/` listing: `packet-authoring-contract` and `acceptance-verifier` are absent, so they are New. Lens C's extra MODIFIED conflicts are settled by lens A's requirement set plus the live admission contract, which still requires all four target fields after late binding.

## Coverage Gaps

- Proposal `sdd-apply` names consumer-test ownership; no draft named a requirement or scenario for it. Not invented.
- `deterministic-orchestrator-contract` capability text covers evidence, recovery, and terminal reporting; lens A specified only Cross-Runtime Orchestrator Preflight and Sequencing.
- Frozen Evidence names tree hashes; no draft gave a tree-hash scenario, and this worktree has no tree-hash consumer.
- Late-binding statement names feature, parent ref, and base SHA. Live `Explicit Feature Target` and `FeatureTarget()` still require `expected_parent_sha` at admission. Not added to lens A's text.
- `sdd-spec` wants Purpose + Requirements full specs for new domains and a 650-word per-artifact budget. This packet requires change-folder ADDED/MODIFIED deltas and a tree-wide 1800-word authored budget (905 authored words). Packet spine followed; skill 650 not applied per file.
- REMOVED/RENAMED: none (lens C). No missing migration.

## Dropped Citations

Union of the three manifests: 40 unique `file:line` citations, each opened in this worktree. Live-spec and proposal ranges all support their claims. Code citations that do not support the stated claim (dropped from the delta; requirements kept only where the proposal/live spec still warrants them):

- `cmd/lucind-ai/cli.go:791-815` (A, B): claimed primary-root, linked-worktree refusal, and base-SHA check as the preflight consumer. Range is `feature create` after the empty `--base-sha` check (785-789). No skill-parity or schema preflight.
- `internal/dag/waves.go:43-66` (A, B): claimed global path disjointness. Loop is dependency placement; `ValidateGlobalOverlap` is 68-70.
- `internal/packet/packet.go:78-120` (A, B): claimed completion-mode and late-binding parse. Range extracts targets and `read_only`; `allowed_paths` is 131-137; late bind is `integrate_feature.go:58-65`.
- `internal/run/run.go:603-670` (A): claimed tree hashes, schema envelopes, and hard-stop demotion. Range is path diff plus the start of completion-mode checks. No tree hashes.
- `internal/run/run.go:657-670` (B): claimed completion-mode matching packet type. Read-only vs write checks are 674-690.
- `internal/result/result.go:20-34` (B): claimed the unmarshaler. Range is `ErrSchemaInvalid` and `HardStop`; `Read` starts at 137.
- `internal/dag/overlap.go:10-15` (B): claimed enforcement of acyclic overlap. Range is the sentinel and the start of `reaches`; it does not enforce.
- `internal/dag/overlap.go:52-67` (B): claimed rejection of unordered overlap. Reject is 71-74.
- `cmd/lucind-ai/cli.go:946-973` (B): claimed no-duplicate recover. Range prints `RecoverAttempt` status; no-redispatch is `attempt.go:245-255`.
- `internal/ledger/ledger.go:1-60` (B): claimed WAL/busy-timeout init. Range is package comment and sentinels; `Open` is later.
- `internal/run/integrate_feature.go:13-52` (B): uniform target check holds; empty-template late-bind is 58-65 (retargeted, not used as late-bind proof).

Verified and kept as support (not listed above): `waves.go:11-18`, `packet.go:22-30`, `attempt.go:245-255`, `integrate_feature.go:100-140`, `run.go:603-655`, `cli.go:1466-1500`, and every lens C live-spec/proposal range. Two MODIFIED blocks match live `sdd-apply` 37-50 and `parent-feature-integration` 47-64 scenario-for-scenario.

## Requirement Divergence

Lens A's set (authoritative): three ADDED (Cross-Runtime Orchestrator Preflight and Sequencing, Target-Free Packet Authoring and Late Binding, Frozen Evidence Acceptance Verification) and two MODIFIED (Orchestrator Advances Only on a Passing Wave, Recoverable Idempotent Attempts). No ADDED→MODIFIED correction: those three names have no live spec here.

Lens B independently covered the same five capabilities and supplied happy/edge/error scenarios for each, but (1) treated `packet-authoring-contract` and `acceptance-verifier` as existing, and (2) keyed scenarios by capability name. Joined on the 1:1 capability map. Dropped from the delta, under the names B used:

- `sdd-apply` / Optional sidecar absent — live `An Absent Sidecar Preserves Hand-Split Apply`, untouched.
- `parent-feature-integration` / Atomic CAS promotes combined tree — live `Immutable Starts and Serialized Promotion`, untouched.

Lens C independently converged with A and the proposal on 3 New + 2 Modified capabilities and on the two MODIFIED names A listed. C also marked `Apply Authors Packets, Not Primary Diffs` and `Explicit Feature Target` as MODIFIED. Omitted: target-free authoring is the new packet-authoring-contract; admission still requires all four fields. Copy-full-then-edit kept live lease-recovery text that A's shorter statement would have deleted.

All three lenses reported no open questions.
