# Fan-out strategy

Load this module only for approved multi-Agent delegation or multi-lens SDD planning.

## Planning topology

Three `agy` Lanes own disjoint lenses; one `cursor-agent` Lane synthesizes the canonical artifact. Repeated copies of the same prompt are not lenses: each lens needs unique required reading, an output skeleton, and explicit sibling ownership.

| Phase | Lens A | Lens B | Lens C | Synthesis |
|---|---|---|---|---|
| explore | Problem and candidates | Capabilities and scenarios | Risks, trade-offs, spikes | `explore.md` plus notes |
| propose | Candidate and approach | Capability impact and specs | Risks, rollback, tests | `proposal.md` plus notes |
| design | Technical approach and decisions | Flow, invariants, file changes | Testing, threat matrix, rollback | `design.md` plus notes |
| specs | Capabilities and requirements | Scenarios and coverage | Live-spec conflicts and migration | capability specs plus notes |
| tasks | Decomposition and ordering | Partition and dispatch shape | Proof and review burden | `tasks.md` plus notes |

Templates live at `../../assets/<phase>-lens-{a,b,c}-packet-template.md` and `../../assets/<phase>-synthesis-packet-template.md`; specs uses the singular `spec-` basename.

## Dispatch

- Dispatch Lens wave first and synthesis second. Lens worktrees cannot see siblings; synthesis must start from the accepted combined drafts.
- Design and specs are siblings consuming the accepted proposal and may run concurrently as six disjoint Lens packets, followed by both synthesizers. Tasks waits for both.
- Dispatch against an explicit target. For the runtime `legacy_main` path, recompute expected SHA after Lens Acceptance before synthesis because the target moved.
- If one Lens fails admission, repair target metadata. If execution fails, remediate and rerun only that Lens. Never start synthesis until all required Lens IDs are accepted.
- Synthesis Lanes need an explicit timeout of at least 30 minutes; measured runs exceeded the default.

The older dual-executor pattern remains valid when approved: dispatch `agy` and `cursor-agent` to distinct draft paths, then have the Orchestrator synthesize one canonical artifact rather than selecting a draft wholesale. Record each draft's contribution in Change state. Use it only when complementary specificity justifies the additional quota; convergence alone does not.

## Authority and compression

Lens A owns the phase's primary declaration. B and C state assumptions; synthesis records divergence rather than hiding it.

| Phase | Lens A owns | B/C assumption | Divergence section |
|---|---|---|---|
| explore | Problem, candidates, recommendation | Assumed problem and candidates | `## Approach Divergence` |
| propose | Candidate, approach, conceptual changes | Assumed candidate and approach | `## Scope Divergence` |
| design | Architecture decisions | `## Assumed architecture` | `## Architecture Divergence` |
| specs | Capability map and requirement statements | `## Assumed requirements` | `## Requirement Divergence` |
| tasks | Checklist and dependency order | `## Assumed decomposition` | `## Decomposition Divergence` |

In specs only, Lens C's live-spec evidence outranks Lens A for ADDED versus MODIFIED classification, while Lens A still owns requirement text and scope.

The real phase skill governs required document content. The packet governs topology, ownership, budgets, paths, and done criteria. Nothing outside the repository is written.

Each Lens stays under 1000 words and the canonical artifact under 1800 words. The canonical budget stays below the sum of the lens budgets; this compression gap forces arbitration. Copied live scenarios inside a MODIFIED requirement are excluded from specs evidence budgets because archive must preserve the complete live block.

The Orchestrator reads synthesis notes: unresolved contradictions, coverage gaps, dropped citations, and phase divergence. A populated contradiction section requires human judgment. Verify every canonical citation; synthesis is the single point where hallucinated evidence can otherwise pass.

## Coverage and archive gate

Use the phase-specific spine below and its matching synthesis asset.

| Phase | Required spine |
|---|---|
| explore | Current behavior; built versus convention; constraints and blockers; candidate scopes with costs; prior art; deciding question; open questions. |
| propose | Intent; in/out scope; new/modified capabilities; approach; affected areas; risks; rollback; dependencies; success criteria; review burden; rejected alternatives; design questions. |
| design | Technical approach; decisions with alternatives and rationale; flow and invariants; file changes with terminal consumers; test seams; threat matrix with every row applicable or reasoned N/A; rollback; open questions. |
| specs | Every proposed capability at the correct path; every requirement classified; RFC 2119 keyword; scenarios in GIVEN/WHEN/THEN; complete MODIFIED blocks; reasons and migration for removal/rename; no implementation detail. |
| tasks | Review workload forecast; reviewable work units with rollback boundaries; specific verifiable checklist; test-before-production tasks for applicable threats; dependency order; green waves; executor per DAG unit; requirement traceability. |

Every tasks wave must pass repository checks independently, trace requirements, declare dependency order, and keep same-wave scopes disjoint. Forecast template-heavy work from the existing roughly 150-line templates rather than treating each template as a tiny edit; a prior Lens forecast of 120-250 total lines missed an actual 1730-line template Change.

Before archive, audit every unchecked task against real symbols and callers; classify it as stale-complete, genuinely undelivered, or explicitly superseded. Never turn superseded work into a false completion checkmark. Open Questions are design follow-ups, not implementation checkboxes.
