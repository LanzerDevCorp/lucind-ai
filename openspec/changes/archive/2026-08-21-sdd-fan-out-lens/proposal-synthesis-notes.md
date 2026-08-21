# Synthesis Notes: sdd-fan-out-lens proposal

## Unresolved Contradictions

None. Apparent collisions were settled against the code:

- Lens A treats path disjointness and post-diff scope as existing Go machine boundaries; lens B's "prose-only elements" table listed them as deliberately left in prose, citing `SKILL.md:143-148`. The code matches A and the exploration's built-versus-convention table: disjointness is `internal/packet/disjoint.go:24-48` and post-diff is `internal/run/run.go:590-626`. `SKILL.md:143-148` is the lane-ownership table (who writes which draft), not those checks. B's disposition for slice ownership stays; the "left in prose" claim for the machine checks does not.
- Lens A writes "required by `internal/packet/packet.go:33-74,94-165`" for the five feature-target keys; admission copy in `SKILL.md:157-161` is the requirement. `Parse` accepts the keys (`packet.go:63-72,114-130`) but does not require them (`packet.go:155-164` only demands `id`, `executor`, `routed_by`, and a non-empty body). Not a two-draft fight: A's wording overclaimed the parser. Recast to parser-accepts plus admission-requires.

## Coverage Gaps

- `~/.claude/skills/sdd-propose/SKILL.md` Step 0 asks for an interactive product question round (users, business rules, edge cases). No draft produced one. Lens C's four questions are design/mechanism, not product-shaping. This change is an internal orchestrator-convention decision; the gap is reported rather than filled with invented product questions.
- The same skill's size rule is "under 450 words." This packet sets a 1400-word budget and wins on execution. Canonical `proposal.md` follows the packet budget and the skill's required sections / proposal altitude. Not a missing spine item.
- No draft wrote the `openspec/config.yaml` `rules.proposal` sentence naming the lane-lifecycle call site. Filled from lens B's dependency list without inventing a hook: the change rides existing `lucind-ai run --packet` batch-and-integrate; it does not add a call site.

Spine items the drafts did cover: intent; scope and non-goals; capabilities new and modified (both None); approach; affected areas; risks; rollback; success criteria; dependencies; open questions left to design.

## Dropped Citations

- **Lens A `cmd/lucind-ai/cli.go:57-61` as "repeatable `--packet` flag parsing."** Those lines are `supportedExecutors` (agy / cursor-agent / opencode). Flag parsing is `cli.go:63-67` (`packetPaths`) and `cli.go:121-149` (`runDispatch`). Kept 57-61,187-197 only as executor registry and support check. `--packet` parsing in `proposal.md` uses `cli.go:121-149`.
- **Lens A `explore.md:3` plus `explore-synthesis-notes.md:1-60` as proof of "zero reverts, clean integration, and rigorous citation verification."** `explore.md:3` says generic dispatch already runs the topology and the null option is live. The notes file records explore-phase dropped citations and lens overlap; it never says zero reverts. Dropped the success-story wording. Kept `explore.md:3` for "topology already runs on `--packet`."
- **Lens A `internal/packet/packet.go:33-74,94-165` as keys "required by" Parse.** See Unresolved Contradictions. Recast to `packet.go:63-72,114-130` (fields and parse cases) plus `SKILL.md:157-161,178-182` (admission and silent failure).
- **Lens A "seven shipped subcommands" at `SKILL.md:288-293`.** The block omits `serve`, `feature`, and `reconcile` as claimed. The count "seven" does not match the binary (`cli.go:104-109,664-665,910-911`: `serve` + `feature create|status|recover` + `reconcile approve|decline|cancel|renew`). Dropped the number; kept the omission with the actual names.
- **Lens B `SKILL.md:143-148` as evidence that "Path disjointness + post-diff scope" is "deliberately left in prose."** Those lines are the executor/ownership table. Machine checks are Go (see Unresolved Contradictions). Dropped.
- **Lens B `cmd/lucind-ai/cli.go:121-149` as "multi-lane batch execution and barrier join."** That range is `runDispatch` flag parse and `--packet` required. Batch and barrier are `internal/run/batch.go:66-113`; the call is `cli.go:285-297`. Recast.
- **Lens B `cmd/lucind-ai/cli.go:664-890` as "parent feature branch admission and promotion."** That range is `feature create|status|recover`. Packet admission is `SKILL.md:157-161` / `internal/run` `ErrMissingFeatureTarget`. Promotion of a combined batch is `internal/run/integrate.go:62-79` (`PromoteTarget`). Kept the feature CLI only as a shipped subcommand to document.
- **Lens B `internal/packet/packet.go:71-78` as feature admission.** `71-72` is the `LegacyMain` field; `77-78` opens `Parse`. Not admission.
- **Lens C `integrate.go:62-79` as "Integrate fails on git merge conflicts."** Those lines handle `PromoteTarget` failure after checks pass. `CombineTree` failure is `integrate.go:45-47`. Dropped the merge-conflict wording. The overlapping-`allowed_paths` risk remains, backed by `cli.go:243` (`DisjointAllowedPaths`).
- **Lens C "omits paths without `read_only: true`" as a cause of `DisjointAllowedPaths` rejection.** Omitting `allowed_paths` skips disjointness and post-diff (`disjoint.go:24-48` comment; `allowed-paths-enforcement` "Omitting AllowedPaths Preserves Today's Exact Path"). Dropped that clause. Overlapping declared paths still reject.

Parser artifact, not a draft claim: `~/.claude/skills/sdd-propose/SKILL.md:92-158` in lens A is the propose-skill template. It resolves. It was not carried: it is this packet's layout-precedence note, not a change requirement.

## Scope Divergence

None on the candidate. All three independently assumed **Candidate 1 — Null option** (lens A: "convention and template hardening only (no Go binary changes)"; lens B: the same, folding frontmatter keys, CLI subcommands, and feature-branch ownership; lens C: "convention and template hardening only (no Go)"). Independent convergence.

Lens A owns the choice and slightly expanded explore's Candidate 1 surface (`explore.md:48-53` "Would touch: `SKILL.md` and `assets/`") by adding `internal/packet/packet_test.go` contract tests, still with "no Go runtime / `cmd/` / `internal/run/` changes." Lenses B and C corroborate the tests (C in risks, rollback, and review burden; B omitted the test file from Affected Areas). Carried as Candidate 1, not as Candidate 2 or 3.

Lens B content that only made sense under a different candidate: none. Its spec-disposition table is "honored unchanged," which is Candidate 1. The "prose-only elements" table mixed Candidate 1 editorial protocol with machine checks that already exist; only the editorial half was kept.

Lens C's Candidate 2 / 3 text is rejection under Candidate 1, not an alternate assumed scope. Carried as rejected alternatives.

## Altitude Leaks

- **Lens A, deciding-question paragraph:** inventoried `ExecuteBatch`, `DisjointAllowedPaths`, `enforceAllowedPaths`, `decideStatus`, and related functions as the "machine boundaries." Compressed to verified `file:line` evidence that generic `--packet` already runs the topology; function names stripped.
- **Lens A, Approach / Success Criteria:** named `internal.packet.Parse` as the template gate. Replaced with "parse as packets" / "contract tests versus the parser."
- **Lens A, Open Questions:** parenthetical that this packet's three-lane layout precedes `sdd-propose` Step 4. Packet-local execution note; not a change-level open question. Dropped.
- **Lens B, Accepted specifications table:** every requirement row of six specs with line ranges. Compressed to spec names, all honored unchanged.
- **Lens B, Prose-only elements table:** per-element function and template line map (explore leftover). One sentence in Intent that editorial protocol stays in `SKILL.md` / templates; table not copied.
- **Lens B, Dependencies:** function:line list. Compressed to the existing `--packet` surfaces; no new hook.
- **Lens C, Risks "Mechanism" column:** `DisjointAllowedPaths`, `cli.go:243`, `integrate.go:62-79`, "hardcode word limits in lens packet headers." Risks kept at consequence + mitigation; argv, function names, and "hardcode" stripped.
- **Lens C, Rejected alternatives:** named `parse.go` / `validate.go` / `emit.go` / `internal/fanout` as the would-touch set. Kept why they were rejected; file-change table left to design.
- **Lens C, Open questions:** "what specific test assertions" is already a design question and was kept as such, not answered here.
