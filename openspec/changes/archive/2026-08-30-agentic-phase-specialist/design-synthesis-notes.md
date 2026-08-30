# Synthesis Notes: Agentic Phase Specialist

## Unresolved Contradictions

None

## Coverage Gaps

None of the eight spine items were missing from the drafts.

Two items no lens specified, required by the packet Context (not invented here), and included in `design.md`:

1. Out-of-repository strategy for `~/.claude/skills/sdd-*/SKILL.md`. Lenses listed only in-repo plugin trees. Packet Context: the Change documents the required text; a human applies it outside any Lane (`fan-out.md:43`).
2. Labeling empty/missing `sdd_phase` as a conservative extension of ADR-0002 / docs decision 9 (which named only `apply` + explicit exception). All three lenses included the fail-safe; none called it an extension.

## Dropped Citations

1. **Retargeted — `internal/integrate/integrate.go:120-148` (Lens C).** Claim: `CheckPolicySnapshot` captures check version, timeout, and environment. Lines 120–148 are merge-abort cleanup and `branchCommitDate`. `CheckPolicySnapshot` is `integrate.go:232-236`. Claim kept; `design.md` does not cite the wrong range.

2. **Retargeted — `TestSkillDocumentsLanguageGlossary` (Lens C testing strategy).** Claim: glossary projection test plus `TestSkillTreesByteIdentical`. No function of that name exists. Projection lives at the end of `TestSkillAssetContract` (`internal/packet/packet_test.go:924-941`). `design.md` names `TestSkillAssetContract`.

3. **Retargeted — `internal/ledger/authoring.go:26,44-75` for `BindingVersion` (Lens C rollback).** Line 26 is `Contract json.RawMessage`; 44–75 is freeze/decode. `BindingVersion` is set in `internal/accept/accept.go:364`. Both facts kept with corrected cites.

4. **Retargeted — `internal/integrate/integrate_test.go:21-50` as Check regression (Lens C).** Those lines are `initRepo` / `runGit` helpers. Check tests start at `TestCheckAbsentScript` (`integrate_test.go:471`). `design.md` cites `:471`.

5. **Dropped — `internal/accept/accept.go:80-100` (Lens C).** Claim: Verifier generates immutable receipts without mutating refs. Lines 80–100 load authoring evidence and call `validateResultAndScope`. Receipt persist is 138–146; no-ref mutation is the package comment (`:1-3`) and `accept_test.go:80-99`. Not used as a receipt-site cite.

6. **Dropped — `plugin/claude-code/skills/lucind-ai/SKILL.md:21-22` for byte-identity (Lens C).** Line 21 is `lucind-ai -v`. Byte-identity before worktree allocation is line 22. `design.md` cites `packet_test.go:943-967` instead.

7. **Dropped as current-behavior — `internal/accept/accept_test.go:80-100` and `:102-125` (Lens B file changes).** Claim: those lines assert check skip vs apply/empty/exception. They currently test receipt persistence and invalid-evidence rejection. New tests belong at the fixture seam (`:26-67`), which `design.md` cites.

8. **Dropped as current-behavior — `internal/run/attempt_test.go:80-100` (Lens B file changes).** Claim: those lines assert non-apply skip. They finish `newAttemptTestDeps` and start replay tests. `design.md` cites spy seams `:24-44,83-92`.

All other unique ranges opened in this worktree supported their claims, including `accept.go:84-96` (conditional metadata load), `accept.go:120-137` (unconditional checks today), `attempt.go:431-435` (default `integrate.Check`), `lanes_meta.go:20-47` (`SDDPhase`), `result.go:1-12` (worktree envelopes), `SKILL.md:19` in both trees, and `fan-out.md:47-48`.

## Architecture Divergence

All three `## Assumed architecture` blocks independently converged on Lens A: agentic `sdd-*` Specialist, own-phase Acceptance, compressed Phase Verdict, caller-side check gating with empty/missing fail-safe, Adapter retained as a tool, human-only Promotion, Dual-Judge for Tier A.

**Phase Verdict shape:** Lens A Decision 1 resolved it (structured markdown in the Specialist chat response, not JSON under `internal/result/`). Synthesis confirmed the grounding — `internal/result/` models worktree `.lucind/result.json` (`result.go:1-12`); no conversation-level result package exists — and did not substitute a different call.

What B or C assumed that differed from A:

- **Lens B flow diagram** shows the Specialist invoking `lucind-ai run` / `lucind-ai accept`. Lens A Decision 4 keeps Orchestrator-mediated `lucind-ai run` this Change (no Bash/Agent tools; `proposal.md:18-19`). `design.md` follows A: Specialist authors packets and owns the judgment; Orchestrator dispatches. B’s hops survive as logical ownership, not a tool grant.
- **Lens C open question** on an extra skip-guard when `files_changed` contains `.go`: rejected by Lens A Decision 2. Not an open question in `design.md`.
- **Lens C assumed architecture** cited `accept.go:120-137` and `attempt.go:431-435` as the metadata-load sites. Those ranges are today’s unconditional check calls; current metadata load is `accept.go:84-96` (conditional). Target architecture matches A (lift the load, then gate those calls). No content lost.
