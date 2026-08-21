# Synthesis Notes: sdd-fan-out-lens explore

## Unresolved Contradictions

None. Apparent collisions resolved against the code:

- Lens A calls two-invocation wave sequencing prose-only; lens B calls the hand-authored `--packet` path fully expressible. Both true: batch + integrate exist (`cmd/lucind-ai/cli.go:285-297`, `internal/run/integrate.go:31-81`); the convention's two-wave schedule is orchestrator protocol (`SKILL.md:153-176`).
- Lens B's sidecar `read_only` / empty-`allowed_paths` / apply-only location gaps are the constraints candidate 2 would close. Candidate 2 does not rest on a gap lens B missed.
- Lens A "built" vs lens B "gap" never named the same mechanism: A describes generic dispatch; B describes what `internal/dag` cannot emit.

## Coverage Gaps

- Lens C's option space omits the three-invocation scheduling variant already in `SKILL.md:195-197` (lens A alone in wave 1, B and C together in wave 2). It is a dispatch schedule on the existing convention, not a Go-vs-prose fork, so it sits beside candidate 1 rather than against 2 or 3. Left out of `explore.md` to keep the candidate list as lens C formed it.
- No lens asked whether the topology should stay design-phase-only (`SKILL.md:128`) or generalize to explore/propose/specs/tasks. Candidate 2's `<phase>-dag.yaml` implies the latter; candidate 1 can stay design-only. The deciding question covers the fork; phase breadth is proposal work.

## Dropped Citations

Grouped. Claims recast in `explore.md` with verified lines are marked recast; claims dropped entirely are marked dropped.

**Lens A `SKILL.md` line numbers after the topology table are stale** (legacy-mode / silent-admission paragraphs shifted later cites). Cited range → what those lines actually contain → where the claim lives:

- `SKILL.md:153-159` for the wave 1 command: 153-155 is two-invocations/no-sidecar (kept); 157-161 is `legacy_main` admission, not the wave 1 argv. Wave 1 command is `163-170`. Recast.
- `SKILL.md:160-165` for wave 2: those lines are `legacy_main: true` and SHA supply. Wave 2 is `171-176`; integrated-tree branching is `184-186`. Recast.
- `SKILL.md:167-173` for assumed architecture: those lines are the wave 1 `--packet` argv. Assumed architecture is `188-193`. Recast.
- `SKILL.md:178-180` for word budgets: those lines are silent admission failure. Budgets are `199-200`. Recast.
- `SKILL.md:181-186` for the compression gap: those lines diagnose an empty worktree path. Compression gap is `202-207`. Recast.
- `SKILL.md:197-214` for asymmetric skill/packet precedence: those lines are the three-invocation alternative. Precedence is `218-227`. Recast.
- `SKILL.md:216-219` for "orchestrator reviews only notes" and the four notes sections: those lines are archived-design word-count calibration and `sdd-design` read access. Orchestrator-reads-notes is `130-131`; four sections are `design-synthesis-packet-template.md:92-119`. Recast.
- `SKILL.md:221-225` for the citation verification pass: those lines are "skill wins on contents." Citation verification is `design-synthesis-packet-template.md:48-55,147-148`. Recast.
- `SKILL.md:227-233` for the eight-item design spine: those lines are "packet wins on execution." Spine is `design-synthesis-packet-template.md:70-84`. Recast.

**Template `:16` cites are blank lines** after `## Goal`. Ownership text is Goal body `:17-18` (lens A/B/C templates) and `SKILL.md:145-148`. Recast; `:16` not reused.

**Lens B shorthand `packet.go:N` does not resolve as a repo-relative path** (nine ranges on explore-lens-b.md:21-23). The same lines in `internal/packet/packet.go` support the frontmatter claims (`ErrMissingID` etc. at `23-26,155-165`; `read_only` at `57-58,105-113`; `legacy_main` at `71-72,122-130`; `allowed_paths` at `59-61,131-137`; unknown keys ignored at `94-138`). Recast with the qualified path.

**`internal/dag/parse.go:40-42` does not hardcode `apply-dag.yaml`.** Those lines are `DAG.Change` and `DAG.Packets` struct tags. Filename binding is the spec (`apply-dag-dispatch/spec.md:9-12`); `Parse` takes any path (`parse.go:44-46`); `split --dag` help text names `apply-dag.yaml` but does not check the basename (`cmd/lucind-ai/cli.go:343`). Dropped the "hardcoded in parse.go:40-42" wording; kept the spec-bound location.

**`internal/run/run.go:50-60,599-601` do not say `.lucind/result.json` is gitignored.** `50-60` define `resultEnvelopePath`; `599-601` exclude `.lucind/` from scope comparison. Gitignore is `.gitignore:2` (`.lucind/`). Recast.

**`openspec/specs/verify-dual-dispatch/spec.md:29-51` does not describe envelope aggregation.** It is dual parallel judgment dispatch and barrier join. Aggregation is `145-151`. Recast.

**`openspec/changes/archive/2026-08-20-verify-dual-dispatch/proposal.md:34-40` does not support "dual-executor propose/design/specs/tasks has no Go code."** Those lines are verify's current-vs-target table. Dual-executor protocol is `SKILL.md:71-95`. Dropped the archive cite; qualified "no Go" to "no dedicated command or specs — rides generic `--packet`."

**Lens A `cli.go:124-150` does not enforce omitting `apply-dag.yaml`.** That range is `runDispatch` flag parse and `--packet` required. Absence of a sidecar requirement is not omission enforcement. Recast as "`run --packet` does not require a sidecar" (`cli.go:121-149`).

**Lens A `TestPacketTemplateAssetStructure`:** the function at `packet_test.go:438-474` is `TestPacketTemplateAssetContract`. Recast with the real name.

**Lens A `internal/overlap/overlap.go:26-99`:** types for Class, ChangeLabel, Metrics, Thresholds live there; the classify algorithm is later in the file. Not used in `explore.md` (overlap classification is not load-bearing for this fan-out). Dropped from the canonical doc as surplus machinery.

## Lens Overlap

The partition leaked on the sidecar-versus-hand-authored seam, which all three lenses covered:

- **A and B** both catalogued batch dispatch, integrate, `allowed_paths`, worktrees, and completion mode. A's job was current state; B's was constraints. The same files answered both.
- **B and C** both treated `dag.Node` lacking `read_only`, empty-`allowed_paths` rejection, and apply-only sidecar location. B as hard blocker; C as the gap candidate 2 would close.
- **A and C** both framed convention-versus-binary (A's built-versus-convention table; C's prior-art "what got formalized").
- **All three** restated two-wave `--packet` dispatch without `apply-dag.yaml`.

Harmless factually — the code agreed — but it is the signal that "current state / constraints / options" still share the same object (the sidecar gap). A tighter partition would give A the generic dispatch inventory, B only schema/spec invariants the fan-out would have to honor or amend, and C only candidates plus prior-art contrast, forbidding B from narrating the two-wave happy path and forbidding A from listing "prose only" items that are really B's gaps.
