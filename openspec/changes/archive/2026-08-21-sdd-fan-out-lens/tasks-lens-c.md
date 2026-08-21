# Tasks Lens C — Test-First Plan & Forecast: sdd-fan-out-lens

## Assumed work surface

Touches `plugin/claude-code/skills/lucind-ai/SKILL.md` (promotes fan-out convention across five planning phases, documents five feature keys, shipped CLI surface, feature-branch lifecycle, and failure remediation), `plugin/claude-code/skills/lucind-ai/assets/` (adds 8 explore/propose lens and synthesis templates), and `internal/packet/packet_test.go` (contract tests pinning documentation and templates). Confined strictly to documentation, template assets, and Go test assertions with no production runtime Go changes.

## Review Workload Forecast

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

- **Estimated changed lines**: 120–250 lines across `SKILL.md`, template assets, and `packet_test.go`.
- **Budget justification**: The change is purely additive documentation, markdown templates, and contract test assertions. No binary logic under `cmd/lucind-ai/` or `internal/run/` is modified. Well below the 400-line review budget and the session's 5000-line preflight limit (`delivery_strategy: single-pr`).

## RED test plan

| Production work | RED test asserts | Existing seam (file:line) | Spec requirement |
|---|---|---|---|
| `SKILL.md`: Frontmatter reference table | Table contains all five feature-target keys (`feature`, `parent_ref`, `base_sha`, `expected_parent_sha`, `legacy_main`) accepted by `packet.Parse`. | `internal/packet/packet_test.go:476` | Frontmatter Admission and CLI Documentation |
| `SKILL.md`: Planning fan-out protocol | Documents multi-lens fan-out as standard planning convention across explore, propose, design, specs, tasks; specifies wave-1 parallel dispatch and wave-2 synthesizer branching strictly from integrated `HEAD`. | `internal/packet/packet_test.go:476` | Two-Wave Planning Fan-Out Protocol |
| `SKILL.md`: Asymmetric precedence & compression | Documents asymmetric precedence (skill governs document schema; packet governs topology, slices, word ceilings, criteria) across all planning packets, and canonical ceiling strictly below sum of lens ceilings. | `internal/packet/packet_test.go:476` | Asymmetric Precedence and Compression Ceiling |
| `SKILL.md`: Shipped CLI surface documentation | Documents subcommands (`serve`, `feature`, `reconcile`, `renew`, `split`, `check`) and `run` flags (`--timeout`, `--approval-timeout`, `--legacy-main`, `--expected-parent-sha`). | `internal/packet/packet_test.go:476` | Frontmatter Admission and CLI Documentation |
| `SKILL.md`: Feature lifecycle & wave-1 failure copy | Documents orchestrator feature ownership and two-tier wave-1 remediation (admission failure repair vs execution blockage re-dispatch). | `internal/packet/packet_test.go:476` | Frontmatter Admission and CLI Documentation |
| `assets/explore-lens-{a,b,c}-packet-template.md` & `explore-synthesis-packet-template.md` | Explore templates parse via `packet.Parse`, have non-empty bodies, set `legacy_main: true` or feature keys, and wave-1 lens draft paths are pairwise disjoint via `packet.DisjointAllowedPaths`. | `internal/packet/packet_test.go:518` (using `internal/packet/packet.go:78`, `internal/packet/disjoint.go:29`) | Planning Fan-Out Template Assets |
| `assets/propose-lens-{a,b,c}-packet-template.md` & `propose-synthesis-packet-template.md` | Propose templates parse via `packet.Parse`, have non-empty bodies, set `legacy_main: true` or feature keys, and wave-1 lens draft paths are pairwise disjoint via `packet.DisjointAllowedPaths`. | `internal/packet/packet_test.go:518` (using `internal/packet/packet.go:78`, `internal/packet/disjoint.go:29`) | Planning Fan-Out Template Assets |
| `assets/design-*-packet-template.md` pinning | Existing design lens (a, b, c) and synthesis templates parse with valid frontmatter, non-empty body, and pairwise disjoint `allowed_paths`. | `internal/packet/packet_test.go:518` (using `internal/packet/packet.go:78`, `internal/packet/disjoint.go:29`) | Planning Fan-Out Template Assets |

## Threat-matrix obligations

- **Boundary: Documentation-like paths**
  - **Applicability**: Applicable
  - **Adversarial case**: Malformed template assets (invalid YAML delimiters, malformed JSON in `allowed_paths`, non-boolean `legacy_main`, missing mandatory keys `id`/`executor`/`routed_by`, or empty body) or passive markdown text treated as executable logic.
  - **Expected safe behavior**: `packet.Parse` parses frontmatter exclusively without evaluating markdown body content. All valid planning templates in `assets/` parse cleanly with `LegacyMain: true` or feature-target fields. Malformed templates fail fast with typed errors (`ErrInvalidAllowedPaths`, `ErrInvalidLegacyMain`, `ErrMissingID`, `ErrMissingExecutor`, `ErrMissingRoutedBy`, `ErrEmptyBody`), returning an empty worktree path and failing admission closed before dispatch.
  - **Expected failure behavior**: Frontmatter parser executing unparsed body text, throwing untyped errors, or admitting malformed packets with invalid path scopes or missing targets.
  - **Test Seam**: `internal/packet/packet_test.go:207`, `internal/packet/packet_test.go:340`, `internal/packet/packet_test.go:815`.

*(Rows for Git repository selection, Commit state, Push state, and PR commands were marked `N/A: reason` in design and generate no tasks.)*

## Open Questions

- [ ] None
