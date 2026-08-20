# Tasks: Apply-Phase DAG Dispatch

Strict TDD. Runner: `go test ./...`. Every code item is RED then GREEN. Do not implement GREEN until the named RED test exists and fails for the stated reason.

Canonical sources (read in full before coding): `design.md`, `specs/apply-dag-dispatch/spec.md`, `specs/allowed-paths-enforcement/spec.md`, `specs/sdd-apply/spec.md`. Spec citations use `specs/<capability>/spec.md#<Requirement name>`.

## Review Workload Forecast

**This forecast exceeds `review_budget_lines: 2000`. That is a real over-budget risk, not a rounding footnote.** This is the largest of the three dual-executor-dispatch siblings: a new package (`internal/dag`), a new CLI subcommand, a greenfield git-diff scope check (temp-repo tests for 0-commit / 2-commit / git-failure / `.lucind/` exclusion), plus SKILL/template prose. Tests dominate the count because Strict TDD maps each spec scenario onto a failing test.

| Field | Value |
|-------|-------|
| Estimated changed lines | **2600–3800** (impl ~800–950, tests ~1600–2500, docs ~150–250) |
| `review_budget_lines` | 2000 (`state.yaml`; this packet must not edit it) |
| Over-budget? | **Yes — by ~600–1800 lines. Do not ship this as an unremarked single PR.** |
| 400-line work-unit risk | High (scope-check tests and `internal/dag` each exceed 400 on their own) |
| Chained PRs recommended | **Yes** — three slices below. `delivery_strategy` is currently `single-pr`; resolving that conflict is an orchestrator/human decision at apply time, not this packet. |
| Suggested split | PR1 packet+disjoint · PR2 `decideStatus` scope check · PR3 `internal/dag` + `split` + CLI wiring + docs + e2e |
| Delivery strategy | `single-pr` in `state.yaml` — **conflicts with this forecast** |
| Chain strategy | pending human decision |

Decision needed before apply: **Yes** — accept `size:exception` on a single PR, or re-cut `delivery_strategy` to chained PRs matching the suggested split. This tasks file does not edit `state.yaml`.

Corroboration: the independently-dispatched agy draft of this same task list arrived at a lower but still-elevated estimate (1700-2400 lines, "High/Elevated" risk) using a coarser 3-unit split. Both drafts agree independently that this change is real over-budget risk under `single-pr`; they differ only on exactly how far over. Treat 2600-3800 as the operative estimate since it comes from the more granular RED/GREEN breakdown below, but do not read the gap between the two estimates as disagreement on the underlying risk.

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | `Packet.AllowedPaths` parse + `DisjointAllowedPaths` | PR 1 | `go test ./internal/packet -race -count=1` | N/A: `strings.Reader` + table tests | `internal/packet/packet.go`, `internal/packet/disjoint.go`, their tests |
| 2 | `decideStatus` base-SHA diff-union scope check | PR 2 | `go test ./internal/run -race -count=1` | N/A: fakeExecutor + temp git repo (git *is* the spec here) | `internal/run/run.go` scope-check hunk only; existing envelope path untouched |
| 3 | `internal/dag` parse/Kahn/emit + `lucind-ai split` | PR 3 | `go test ./internal/dag ./cmd/lucind-ai -race -count=1` | `lucind-ai split --dag … --out …` on a 2-packet fixture | `internal/dag/`, `cmd/lucind-ai/cli.go` `split` case |
| 4 | CLI overlap-before-Create, `integrated_ids`/`reverted_ids`, SKILL/templates, e2e | PR 3 | `go test ./cmd/lucind-ai ./internal/dag -race -count=1` | `lucind-ai split` then dispatch printed wave lines against a fake executor | `cmd/lucind-ai/cli.go` overlap + `printReport` hunks; `plugin/claude-code/skills/lucind-ai/SKILL.md`; packet templates |

### Constraints (do not add tasks that violate these)

- **No edits** to `internal/run/integrate.go`, `internal/resolve/resolve.go`, `internal/integrate/integrate.go`. Combine → resolve (400-line cap) → Check → bisect → Promote is reused unmodified. Spec: `specs/apply-dag-dispatch/spec.md#Per-Wave Integrate Reuses Combine, Resolve, and Bisect Unmodified`, `specs/sdd-apply/spec.md#Combine, Resolve, and Bisect Stay Untouched`.
- **No edits** to `internal/run/batch.go` (the overlap check's call site is `runDispatch` in `cli.go`, *before* `ExecuteBatch`). `ExecuteBatch` stays a flat concurrent batch: one goroutine per packet, one `WaitGroup`, no DAG type. Spec: `specs/apply-dag-dispatch/spec.md#Sequential Run Per Wave`.
- **No** new ledger column, **no** new ledger event type. Deviation notes reuse existing `ledger.EventLaneNote`. Spec: `specs/sdd-apply/spec.md#Additive Rollback, No Ledger Migration`.
- **No** `waves.json`, **no** `--json` flag, **no** `.lucind/runs/<id>.json`. Stdout **is** the wave plan; `integrated_ids`/`reverted_ids` are printed next to existing counts. Spec: `specs/apply-dag-dispatch/spec.md#Split Is the Mechanical Consumer`, `specs/apply-dag-dispatch/spec.md#Integrated and Reverted Lane IDs on Stdout`.
- **No** glob support in `allowed_paths`. Component-boundary prefix match only.
- **No** empty `allowed_paths` at split time (that is `read-only-packet-dispatch`). At *run* time, omitted/empty still means "not declared."
- `tasks.md` is **not** a parse source. Sidecar is `openspec/changes/<id>/apply-dag.yaml`.

### Sequencing notes (not blockers)

- Shared prefix-match lives in `internal/packet/disjoint.go` (`DisjointAllowedPaths` plus a `PathInScope` helper). `internal/dag` and `internal/run` consume it — do not duplicate the rule.
- `apply-dag.yaml` is YAML; `go.mod` currently has no YAML library. GREEN for 4.1 adds `gopkg.in/yaml.v3` — implied by Decision 1's sidecar format, not an open architecture fork.
- Within-wave packet order on stdout is unspecified. Preserve YAML declaration order so the printed plan is deterministic.
- `decideStatus` today is `func decideStatus(deps Deps, worktreePath string, outcome executor.Outcome)` (`internal/run/run.go:407`) and does not receive the packet. GREEN for 3.2 passes `packet.Packet` in (Execute already has it at `:315`) so the check can read `AllowedPaths`. `Deps.PrimaryRoot` already exists for the base SHA.
- Existing `Execute` tests omit `allowed_paths`, so they must keep skipping the git inspect and stay git-free. Only new scope-check tests create a temp git repo.

---

## Phase 1: `internal/packet` — `AllowedPaths` field + parsing

Matches File Changes row: `internal/packet/packet.go`, `internal/packet/packet_test.go`.

- [x] 1.1 RED `internal/packet/packet_test.go`: table-driven `packet.Parse` — (a) `allowed_paths: ["internal/ledger/", "cmd/lucind-ai/cli.go"]` fills `AllowedPaths` with those two strings; (b) omitted key leaves `AllowedPaths` empty (undeclared); (c) a non-JSON value (e.g. a bare YAML list or `{`) returns a parse error and no `Packet`. Must fail because `Packet` has no `AllowedPaths` field yet. Spec: `specs/allowed-paths-enforcement/spec.md#Packet AllowedPaths Field`.
- [x] 1.2 GREEN `internal/packet/packet.go`: add `AllowedPaths []string` to `Packet`. In the existing line-oriented `strings.Cut`-on-`:` loop (`packet.go:65-75`), handle `allowed_paths` as `json.Unmarshal` of the trimmed value into `[]string`. Invalid JSON is a parse error (new sentinel or wrapped `json` error). Omitted key leaves the slice nil/empty. Nested YAML lists are **not** parsed — `Parse` stays line-oriented. Spec: `specs/allowed-paths-enforcement/spec.md#Packet AllowedPaths Field`.
- [x] 1.3 RED `internal/packet/packet_test.go`: existing packets in this file (no `allowed_paths` key) still parse and keep `AllowedPaths` empty — regression that omitted remains undeclared. Spec: `specs/allowed-paths-enforcement/spec.md#Omitting AllowedPaths Preserves Today's Exact Path`.
- [x] 1.4 GREEN `internal/packet/packet.go`: no extra work if 1.2 already leaves omitted empty; 1.3 is the regression proof. Spec: `specs/allowed-paths-enforcement/spec.md#Omitting AllowedPaths Preserves Today's Exact Path`.

---

## Phase 2: `internal/packet/disjoint.go` — overlap check

Matches File Changes row: `internal/packet/disjoint.go` (new). Same component-boundary prefix rule the splitter and the scope check will reuse.

Rule (Decision 1 / Decision 2): repo-relative POSIX paths, no globs. `internal/ledger` and `internal/ledger/` both match `internal/ledger/foo.go`. `internal/led` does **not** match `internal/ledger/foo.go` (component boundary). Two path lists overlap iff some path in A equals, or is a component-boundary prefix of, some path in B, or vice versa.

- [x] 2.1 RED `internal/packet/disjoint_test.go`: `PathInScope("internal/ledger/foo.go", []string{"internal/ledger"})` and the slash-terminated variant are true; `PathInScope("internal/ledger/foo.go", []string{"internal/led"})` is false; exact file match is true. Must fail because the helper does not exist. Spec: `specs/apply-dag-dispatch/spec.md#Same-Wave Paths Pairwise Disjoint` (the match rule), `specs/allowed-paths-enforcement/spec.md#Post-Execution Scope Check Demotes Done to Deviated`.
- [x] 2.2 GREEN `internal/packet/disjoint.go`: implement `PathInScope(path string, allowed []string) bool` with component-boundary prefix match. Normalize trailing slashes so `internal/ledger` and `internal/ledger/` are equivalent prefixes. Spec: same as 2.1.
- [x] 2.3 RED `internal/packet/disjoint_test.go`: `DisjointAllowedPaths([]Packet{…})` — (a) `internal/foo/` vs `internal/foo/bar.go` returns an error naming both packet IDs; (b) `internal/foo/` vs `internal/bar/` returns nil; (c) `internal/led` vs `internal/ledger/foo.go` returns nil; (d) a packet with empty `AllowedPaths` is skipped (undeclared, not an overlap); (e) a single packet, or all undeclared, returns nil. Spec: `specs/allowed-paths-enforcement/spec.md#Upfront Batch Disjointness Check`, `specs/apply-dag-dispatch/spec.md#Same-Wave Paths Pairwise Disjoint`.
- [x] 2.4 GREEN `internal/packet/disjoint.go`: implement `DisjointAllowedPaths(ps []Packet) error` using `PathInScope` pairwise over declared (non-empty) lists. Called later from CLI, **not** from `ExecuteBatch`. Spec: `specs/allowed-paths-enforcement/spec.md#Upfront Batch Disjointness Check`.

---

## Phase 3: `internal/run/run.go` — scope-check call site, base-SHA diff union

Matches File Changes row: `internal/run/run.go`, `internal/run/run_test.go`. Gated on `len(AllowedPaths) > 0`. Envelope interpretation otherwise unchanged. Git behavior **is** the spec: new tests use a real `t.TempDir()` git repo; existing tests omit the field and must not start shelling out to git.

Capture primary `HEAD` at check time (`git -C primaryRoot rev-parse HEAD`). Union of:

1. `git -C worktree diff --name-only --diff-filter=ACDMRT <base> HEAD` — every committed path since base, regardless of commit count.
2. `git -C worktree diff --name-only --diff-filter=ACDMRT` — unstaged.
3. `git -C worktree ls-files -o --exclude-standard` — untracked, respecting `.gitignore`.

Never `git diff --name-only HEAD~1`. `.lucind/` is always dropped from the union (explicit prefix filter, even if force-added). A git-command failure → `lane.Blocked` with a diagnosis, never a guessed `Done`/`Deviated`. Only a `done` envelope is eligible for demotion to `lane.Deviated`; `blocked`/`failed` stay as-is. Offending paths go in the existing `reason` string so Execute's current `EventLaneNote` path (`run.go:324-329`) records them — no new event type.

- [x] 3.1 RED `internal/run/run_test.go`: packet **omits** `allowed_paths`, schema-valid `status: done` envelope, no git repo required → `Execute` still reaches `lane.Done` (existing happy-path fixture extended with an assertion that `AllowedPaths` empty skips git). Must keep passing after GREEN; write it first so GREEN cannot regress dual-dispatch. Spec: `specs/allowed-paths-enforcement/spec.md#Omitting AllowedPaths Preserves Today's Exact Path`, `specs/sdd-apply/spec.md#Additive Rollback, No Ledger Migration`.
- [x] 3.2 RED `internal/run/run_test.go`: temp git repo, packet `AllowedPaths: ["internal/ledger/"]`, `done` envelope, only in-scope committed/unstaged/untracked paths → `Execute` returns `lane.Done`. Must fail because `decideStatus` (`run.go:407`) returns the envelope status with no git inspect. Spec: `specs/allowed-paths-enforcement/spec.md#Post-Execution Scope Check Demotes Done to Deviated` (in-scope-only), `specs/allowed-paths-enforcement/spec.md#Base-SHA Three-Way Diff Union Defines "Actual Diff"`.
- [x] 3.3 GREEN `internal/run/run.go`: pass `p` into `decideStatus`. After a schema-valid envelope maps to `lane.Done`, if `len(p.AllowedPaths) > 0`, compute the three-way union against `deps.PrimaryRoot` + `worktreePath` and `packet.PathInScope`. In-scope-only keeps `Done`. Spec: same as 3.2.
- [x] 3.4 RED `internal/run/run_test.go`: `done` envelope, `AllowedPaths: ["internal/ledger/"]`, modified tracked file `internal/serve/server.go` → status `lane.Deviated`; ledger `EventLaneNote` names `internal/serve/server.go`. Spec: `specs/allowed-paths-enforcement/spec.md#Post-Execution Scope Check Demotes Done to Deviated`.
- [x] 3.5 GREEN `internal/run/run.go`: out-of-scope path demotes `Done` → `Deviated` and returns a reason listing the paths (Execute already writes `EventLaneNote` when `reason != ""`). Do **not** mutate `.lucind/result.json` on disk. Spec: same as 3.4.
- [x] 3.6 RED `internal/run/run_test.go` (and `internal/run/batch_test.go` if the barrier outcome is only visible there — **do not edit** `batch.go`): `done` envelope + untracked file outside `AllowedPaths` → `Deviated`, and that lane ID is **not** on `barrier.Outcome.Integrate` (Observe already preserves non-`Done`; this test is the terminal consumer of the new status). Spec: `specs/allowed-paths-enforcement/spec.md#Post-Execution Scope Check Demotes Done to Deviated`.
- [x] 3.7 GREEN `internal/run/run.go`: same demotion path as 3.5 covers untracked-via-`ls-files`; 3.6 is the proof `Integrate` never sees it. Spec: same as 3.6.
- [x] 3.8 RED `internal/run/run_test.go`: lane with **zero** commits, only untracked in-scope files, `done` envelope → `Done`; must **not** fail because `HEAD~1` does not resolve. Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Three-Way Diff Union Defines "Actual Diff"`.
- [x] 3.9 RED `internal/run/run_test.go`: lane with **two** commits, earlier commit touches `internal/serve/server.go`, last commit is in-scope only, `AllowedPaths: ["internal/ledger/"]`, `done` envelope → `Deviated` (the `HEAD~1` bug would miss this). Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Three-Way Diff Union Defines "Actual Diff"`.
- [x] 3.10 RED `internal/run/run_test.go`: two or more commits that together touch only in-scope paths + `done` envelope → still `Done`. Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Three-Way Diff Union Defines "Actual Diff"`.
- [x] 3.11 GREEN `internal/run/run.go`: the union in 3.3 already inspects `<base> HEAD` rather than `HEAD~1`; 3.8–3.10 are the proof. Spec: same as 3.8.
- [x] 3.12 RED `internal/run/run_test.go`: `blocked` envelope + out-of-scope touch → still `Blocked`, not `Deviated`. Parallel case: `failed` envelope + out-of-scope touch → still `Failed`. Spec: `specs/allowed-paths-enforcement/spec.md#Blocked and Failed Are Never Rewritten to Deviated`.
- [x] 3.13 GREEN `internal/run/run.go`: gate the union on envelope status `Done` only (`envelope.LaneStatus() == lane.Done` / equivalent). Spec: same as 3.12.
- [x] 3.14 RED `internal/run/run_test.go`: `done` envelope, non-empty `AllowedPaths`, force-added `.lucind/result.json` as the only extra path → stays `Done`; `.lucind/` neither demotes nor counts as satisfying scope on its own. Spec: `specs/allowed-paths-enforcement/spec.md#.lucind/ Is Always Excluded From Scope Comparison`.
- [x] 3.15 GREEN `internal/run/run.go`: drop any path with a `.lucind/` prefix from the union after the three git calls. Spec: same as 3.14.
- [x] 3.16 RED `internal/run/run_test.go`: `done` envelope, non-empty `AllowedPaths`, `worktreePath` is not a git repo (or primary `rev-parse` fails) → `Blocked` with a diagnosis, never `Done` or `Deviated`. Spec: `specs/allowed-paths-enforcement/spec.md#Git Inspection Failure Blocks, Never Guesses`.
- [x] 3.17 GREEN `internal/run/run.go`: any non-zero git exit in the union returns `lane.Blocked` and a reason; do not guess. Spec: same as 3.16.
- [x] 3.18 RED `internal/run/run_test.go`: parsed `AllowedPaths` empty (including explicitly empty JSON array `[]`) skips both git inspect and disjointness at this layer — `done` envelope → `Done` with no git. Spec: `specs/allowed-paths-enforcement/spec.md#Omitting AllowedPaths Preserves Today's Exact Path`.
- [x] 3.19 GREEN: 3.3's `len(AllowedPaths) > 0` gate; 3.18 is the proof. Spec: same as 3.18.

---

## Phase 4: `internal/dag/` — parse `apply-dag.yaml`, Kahn waves, emit packets

Matches File Changes row: `internal/dag/` (new). Package name is `dag`, not `split` — File Changes table and this packet both lock `internal/dag/`. `tasks.md` is never opened.

Sidecar schema (Decision 1):

```yaml
change: <change-id>
packets:
  - id: …
    executor: …
    routed_by: …
    model: …          # optional
    allowed_paths: [… ]  # YAML list here; split emits JSON-array frontmatter
    depends_on: [… ]
    body_path: bodies/<id>.md
```

- [x] 4.1 RED `internal/dag/parse_test.go`: valid sidecar (two packets, required fields + optional `model`, `body_path` files exist on disk via `t.TempDir`) → `Parse` returns both nodes. Missing `body_path` file → error. Must fail because the package does not exist. Spec: `specs/apply-dag-dispatch/spec.md#Sidecar DAG Artifact`.
- [x] 4.2 GREEN `internal/dag/parse.go` (+ `go.mod`/`go.sum`: add `gopkg.in/yaml.v3`): parse the sidecar; require `change`, `packets[]` with `id`, `executor`, `routed_by`, `allowed_paths`, `depends_on`, `body_path`; `model` optional. Stat `body_path` (relative to the sidecar's directory). Spec: same as 4.1.
- [x] 4.3 RED `internal/dag/parse_test.go`: a sibling `tasks.md` with fake `## Wave` / `depends_on` prose is ignored — `Parse` reads only the YAML path it was given. Spec: `specs/apply-dag-dispatch/spec.md#Sidecar DAG Artifact`.
- [x] 4.4 GREEN: `Parse` takes a filesystem path to `apply-dag.yaml` only; no `tasks.md` reader. Spec: same as 4.3.
- [x] 4.5 RED `internal/dag/validate_test.go`: two packets with the same `id` → validate error, and a subsequent emit (if called) writes nothing. Spec: `specs/apply-dag-dispatch/spec.md#Unique Packet IDs`.
- [x] 4.6 GREEN `internal/dag/validate.go`: unique `id` check. Spec: same as 4.5.
- [x] 4.7 RED `internal/dag/validate_test.go`: `allowed_paths: []` or omitted on a node → validate error. Spec: `specs/apply-dag-dispatch/spec.md#Non-Empty Allowed Paths at Split`.
- [x] 4.8 GREEN `internal/dag/validate.go`: reject omitted/empty `allowed_paths` at split (run-time undeclared is a different consumer). Spec: same as 4.7.
- [x] 4.9 RED `internal/dag/waves_test.go`: A depends on B and B depends on A → error that reports a cycle. Spec: `specs/apply-dag-dispatch/spec.md#Acyclic Depends-On, Grouped by Kahn's Algorithm`.
- [x] 4.10 GREEN `internal/dag/waves.go`: Kahn's algorithm; a remaining node with in-degree > 0 after the queue drains is a cycle. Spec: same as 4.9.
- [x] 4.11 RED `internal/dag/waves_test.go`: B depends on A, C depends on neither, A's paths disjoint from C's → wave 1 contains A and C, wave 2 contains B (B later than A). Spec: `specs/apply-dag-dispatch/spec.md#Acyclic Depends-On, Grouped by Kahn's Algorithm`.
- [x] 4.12 GREEN `internal/dag/waves.go`: each Kahn "ready set" is one wave, YAML order preserved inside a wave. Spec: same as 4.11.
- [x] 4.13 RED `internal/dag/waves_test.go`: no `depends_on` edge, `internal/foo/` vs `internal/foo/bar.go` → error (same-wave overlap). Spec: `specs/apply-dag-dispatch/spec.md#Same-Wave Paths Pairwise Disjoint`.
- [x] 4.14 RED `internal/dag/waves_test.go`: no edge, `internal/foo/` vs `internal/bar/` → same wave. Spec: `specs/apply-dag-dispatch/spec.md#Same-Wave Paths Pairwise Disjoint`.
- [x] 4.15 RED `internal/dag/waves_test.go`: no edge, `internal/led` vs `internal/ledger/foo.go` → treated disjoint, same wave. Spec: `specs/apply-dag-dispatch/spec.md#Same-Wave Paths Pairwise Disjoint`.
- [x] 4.16 GREEN `internal/dag/waves.go`: after Kahn grouping, run `packet.DisjointAllowedPaths` (or the same `PathInScope` pairwise) **per wave**. Spec: same as 4.13.
- [x] 4.17 RED `internal/dag/waves_test.go`: B `depends_on: [A]`, overlapping `allowed_paths` → two waves, A before B (cross-wave overlap allowed). Spec: `specs/apply-dag-dispatch/spec.md#Cross-Wave Overlap Requires a Dependency Edge`.
- [x] 4.18 RED `internal/dag/waves_test.go`: overlapping paths, no `depends_on` in either direction → error (author must add an edge or shrink scope). Spec: `specs/apply-dag-dispatch/spec.md#Cross-Wave Overlap Requires a Dependency Edge`.
- [x] 4.19 GREEN `internal/dag/waves.go`: overlap without an edge cannot share a wave, so 4.16's per-wave disjoint check rejects it; overlap *with* an edge is two waves and is accepted. Spec: same as 4.17.
- [x] 4.20 RED `internal/dag/emit_test.go`: successful split writes `<out>/<id>.md` whose body is the `body_path` Markdown **verbatim** (Goal/criteria unchanged) and whose frontmatter includes a **single-line JSON-array** `allowed_paths: […]` that `packet.Parse` accepts, plus `id`/`executor`/`routed_by`/`model` when set. Split invents no Goal/Context/criteria text. Spec: `specs/apply-dag-dispatch/spec.md#Split Is the Mechanical Consumer`, `specs/allowed-paths-enforcement/spec.md#Packet AllowedPaths Field`.
- [x] 4.21 GREEN `internal/dag/emit.go`: concatenate generated frontmatter + body; `packet.Parse` of the result is the round-trip proof. Spec: same as 4.20.
- [x] 4.22 RED `internal/dag/split_test.go` (or `emit_test.go`): DAG where `apply-serve` and `apply-run` both `depends_on: [apply-ledger]` and are same-wave-disjoint → stdout first line is `lucind-ai run --packet <out>/apply-ledger.md` and a later line is one `lucind-ai run` passing both remaining packets as `--packet` flags. **No** `waves.json` (or any other plan file) is written. Duplicate-id / cycle / overlap failures write **no** packet files. Spec: `specs/apply-dag-dispatch/spec.md#Split Is the Mechanical Consumer`.
- [x] 4.23 GREEN `internal/dag/split.go`: public `Split(dagPath, outDir string, stdout io.Writer) error` is the mechanical consumer: validate → waves → emit → print one command per wave. Spec: same as 4.22.

---

## Phase 5: `cmd/lucind-ai/cli.go` — `split` subcommand, overlap-check wiring, ID lists

Matches File Changes row: `cmd/lucind-ai/cli.go`. Three independent hunks; keep them as separate RED/GREEN pairs so a reviewer can revert one without the others.

### 5a. `integrated_ids` / `reverted_ids` on stdout

- [x] 5.1 RED `cmd/lucind-ai/cli_test.go`: given an `IntegrateReport` with `Integrated=["apply-ledger"]` and `Reverted=["apply-serve"]`, the integrate block of stdout contains the existing `integrate: attempted=… passed=… integrated=1 reverted=1` line **and** `integrated_ids: apply-ledger` and `reverted_ids: apply-serve`. Today `cli.go:231-237` prints counts only — this test fails until GREEN. Spec: `specs/apply-dag-dispatch/spec.md#Integrated and Reverted Lane IDs on Stdout`.
- [x] 5.2 RED `cmd/lucind-ai/cli_test.go`: every lane integrated, nothing reverted → stdout includes the integrated IDs and an **explicitly empty** `reverted_ids:` list (not omitted). Spec: `specs/apply-dag-dispatch/spec.md#Integrated and Reverted Lane IDs on Stdout`.
- [x] 5.3 GREEN `cmd/lucind-ai/cli.go`: in `runDispatch` after the existing counts (`cli.go:231-237`), print `integrated_ids:` and `reverted_ids:` from `integrateReport.Integrated` / `.Reverted`. Do **not** add `--json` or a side file. Spec: `specs/apply-dag-dispatch/spec.md#Integrated and Reverted Lane IDs on Stdout`, `specs/sdd-apply/spec.md#Orchestrator Reads Stdout, Not a New Report Format`.

### 5b. Overlap check before `ExecuteBatch`

Call site is `runDispatch` after packets are parsed (`cli.go:119-133`) and **before** `worktree.Create` / `ExecuteBatch` (`cli.go:211`). `ExecuteBatch` itself is not modified.

- [x] 5.4 RED `cmd/lucind-ai/cli_test.go`: two `--packet` files whose declared `AllowedPaths` overlap → `run` returns 1 **and** a test double for `CreateWorktree` is never invoked (inject via the same `runDispatch` wiring, or a seam that records Create calls). Spec: `specs/allowed-paths-enforcement/spec.md#Upfront Batch Disjointness Check`.
- [x] 5.5 RED `cmd/lucind-ai/cli_test.go`: two packets declaring `internal/foo/` and `internal/bar/` pass the check and proceed far enough that Create *would* be called (existing unsupported-executor / missing-primary tests show the pattern — stop before a real dispatch). Spec: `specs/allowed-paths-enforcement/spec.md#Upfront Batch Disjointness Check`.
- [x] 5.6 GREEN `cmd/lucind-ai/cli.go`: after the supported-executor loop (`cli.go:137-142`), if `packet.DisjointAllowedPaths(ps) != nil`, print the error and return 1. Packets with empty `AllowedPaths` are skipped inside that helper (Phase 2). Spec: same as 5.4.
- [x] 5.7 Confirm (no `batch.go` edit): existing `internal/run/batch_test.go` still proves first side effect is worktree creation and lanes never cancel each other. Spec: `specs/allowed-paths-enforcement/spec.md#Upfront Batch Disjointness Check` (ExecuteBatch contract unchanged), `specs/apply-dag-dispatch/spec.md#Sequential Run Per Wave`.

### 5c. `lucind-ai split` subcommand

- [x] 5.8 RED `cmd/lucind-ai/cli_test.go`: `lucind-ai split --dag <temp>/apply-dag.yaml --out <temp>/packets` on the two-wave fixture from 4.22 writes the packet files, prints the wave commands, exit 0. Unknown subcommand path currently rejects `split` (`cli.go:86-88`) — this fails until GREEN. Spec: `specs/apply-dag-dispatch/spec.md#Split Is the Mechanical Consumer`.
- [x] 5.9 RED `cmd/lucind-ai/cli_test.go`: `split` on a cyclic / duplicate-id / empty-`allowed_paths` sidecar exits 1, writes no files under `--out`. Spec: `specs/apply-dag-dispatch/spec.md#Unique Packet IDs`, `Non-Empty Allowed Paths at Split`, `Acyclic Depends-On, Grouped by Kahn's Algorithm`.
- [x] 5.10 GREEN `cmd/lucind-ai/cli.go`: add `case "split":` next to `case "run":` (`cli.go:83-88`). Flags `--dag` and `--out`. Call `dag.Split`. Update `usage` (`cli.go:39`) to mention `split`. Spec: `specs/apply-dag-dispatch/spec.md#Split Is the Mechanical Consumer`.
- [x] 5.11 RED `cmd/lucind-ai/cli_test.go`: two sequential `run` invocations (same process, stubbed deps) produce two distinct `run id:` lines — one ledger `run_id` per wave, no nested/whole-DAG identifier. Spec: `specs/apply-dag-dispatch/spec.md#One Run Per Wave on the Ledger`.
- [x] 5.12 GREEN: existing `runDispatch` already allocates `uuid.NewString()` per invocation (`cli.go:159-160`); 5.11 is the proof. Do not add a DAG-scoped run id. Spec: same as 5.11.

---

## Phase 6: `SKILL.md` and packet templates

Matches File Changes row: `plugin/claude-code/skills/lucind-ai/SKILL.md`, packet templates. Docs, not Go — no RED test file. Apply-phase prose only; do not invent a `--json` channel or an in-process `--dag` wave loop.

- [x] 6.1 `plugin/claude-code/skills/lucind-ai/SKILL.md`: Frontmatter table (`SKILL.md:23-27`) documents optional `allowed_paths` as a single-line JSON array; omitted = today's path (no overlap check, no diff check). Spec: `specs/allowed-paths-enforcement/spec.md#Packet AllowedPaths Field`, `specs/allowed-paths-enforcement/spec.md#Omitting AllowedPaths Preserves Today's Exact Path`.
- [x] 6.2 `plugin/claude-code/skills/lucind-ai/SKILL.md`: replace the apply row of "Target direction, not yet built" (`SKILL.md:79`) with the built loop: author `openspec/changes/<id>/apply-dag.yaml` (sidecar; `tasks.md` stays the human checklist) → `lucind-ai split --dag … --out …` → run each printed `lucind-ai run` line **sequentially**, stop on exit 1. Spec: `specs/sdd-apply/spec.md#Apply Authors Packets, Not Primary Diffs`, `specs/sdd-apply/spec.md#An Absent Sidecar Preserves Hand-Split Apply`, `specs/apply-dag-dispatch/spec.md#Sequential Run Per Wave`.
- [x] 6.3 `plugin/claude-code/skills/lucind-ai/SKILL.md`: document that apply authors packet files (and the sidecar when a DAG is wanted) and dispatches via `lucind-ai run`; it does **not** Write the apply diff in the orchestrator's primary checkout. An **absent** sidecar is still valid — one packet or a hand-split set, no `split` required. Spec: `specs/sdd-apply/spec.md#Apply Authors Packets, Not Primary Diffs`, `specs/sdd-apply/spec.md#An Absent Sidecar Preserves Hand-Split Apply`.
- [x] 6.4 `plugin/claude-code/skills/lucind-ai/SKILL.md`: wave N+1 is dispatched only when wave N exits 0 (every lane `done`, none reverted). On non-zero, halt; read `integrated_ids` / `reverted_ids` from stdout (not a new report format). Confirm every wave-N id is under `integrated_ids` before the next line. Spec: `specs/sdd-apply/spec.md#Orchestrator Advances Only on a Passing Wave`, `specs/sdd-apply/spec.md#Orchestrator Reads Stdout, Not a New Report Format`.
- [x] 6.5 `plugin/claude-code/skills/lucind-ai/SKILL.md`: Exit-code section (`SKILL.md:133`) currently says "Returns `0` only when every lane in the batch achieves `done`." Align it with `cli.go:244-251`: exit 0 also requires none listed in `reverted_ids` (bisection can print `status: done` then revert). Spec: `specs/apply-dag-dispatch/spec.md#Integrated and Reverted Lane IDs on Stdout`.
- [x] 6.6 `plugin/claude-code/skills/lucind-ai/assets/packet-template.md`: frontmatter example (`packet-template.md:1-6`) includes `allowed_paths: ["path/one", "path/two"]`. Allowed-paths body section (`packet-template.md:48-54`) states the binary will demote a `done` envelope to `deviated` if the worktree diff leaves that list. Spec: `specs/allowed-paths-enforcement/spec.md#Packet AllowedPaths Field`, `specs/allowed-paths-enforcement/spec.md#Post-Execution Scope Check Demotes Done to Deviated`.
- [x] 6.7 `plugin/claude-code/skills/lucind-ai/assets/human-packet-template.md`: no `allowed_paths` frontmatter (human packets are not `lucind-ai split` output). Leave the template's structure intact unless a sentence currently implies the binary will not enforce repo paths — in that case, one clarifying sentence, no schema change.

---

## Phase 7: End-to-end — 2 packets, one dependency edge

Explicit packet requirement: author a small real `apply-dag.yaml`, run `lucind-ai split`, dispatch the printed wave commands.

- [x] 7.1 RED `internal/dag/e2e_test.go` (or `cmd/lucind-ai/split_e2e_test.go`): `t.TempDir()` tree containing:
  - `apply-dag.yaml` with **two** packets and **one** `depends_on` edge (e.g. `apply-leaf` depends on `apply-root`; overlapping paths are fine *because* of the edge; or disjoint paths — pick one and keep the fixture tiny).
  - `bodies/apply-root.md` and `bodies/apply-leaf.md` with a non-empty Goal.
  Call `dag.Split` (or `run([]string{"split", …})`). Assert: two packet files; stdout is exactly two `lucind-ai run --packet …` lines in dependency order; each emitted file `packet.Parse`s with non-empty `AllowedPaths`. Must fail until Phases 4–5 exist. Spec: `specs/apply-dag-dispatch/spec.md#Split Is the Mechanical Consumer`, `specs/apply-dag-dispatch/spec.md#Sequential Run Per Wave`, `specs/sdd-apply/spec.md#An Absent Sidecar Preserves Hand-Split Apply` (the present-sidecar half: split then waves).
- [x] 7.2 GREEN: Phases 4–5 make 7.1 pass. No extra indirection (`waves.json` still must not appear in the temp out dir).
- [x] 7.3 RED `cmd/lucind-ai/cli_test.go`: parse the two wave command lines from 7.1 and invoke `runDispatch` for wave 1, then wave 2, against a **fake** executor that writes a schema-valid `done` envelope and an in-scope commit in a temp git repo (do not burn real `agy`/`cursor-agent` quota). Assert: two distinct `run id:` values; wave 1's `integrated_ids` lists the root packet; wave 2 is only dispatched in the test after wave 1's stubbed exit code is 0; a variant where wave 1 exits 1 never calls Create for wave 2. Spec: `specs/apply-dag-dispatch/spec.md#Sequential Run Per Wave`, `specs/apply-dag-dispatch/spec.md#One Run Per Wave on the Ledger`, `specs/sdd-apply/spec.md#Orchestrator Advances Only on a Passing Wave`.
- [x] 7.4 GREEN: wire 7.3 using existing `Execute`/`runDispatch` test doubles; no production wave loop inside the binary. The orchestrator (this test, later SKILL.md) is the sequencer. Spec: `specs/apply-dag-dispatch/spec.md#Sequential Run Per Wave`.

---

## Phase 8: Testing sweep

- [x] 8.1 `go test ./... -race -count=1` covers every remaining spec scenario not named above, including: `ExecuteBatch` still has no DAG/wave parameter (`specs/apply-dag-dispatch/spec.md#Sequential Run Per Wave`); Integrate still calls the unmodified combine/resolve/bisect path (`specs/apply-dag-dispatch/spec.md#Per-Wave Integrate Reuses Combine, Resolve, and Bisect Unmodified`); ledger schema version unchanged (`specs/sdd-apply/spec.md#Additive Rollback, No Ledger Migration`).
- [x] 8.2 Confirm `git grep` on the apply diff: **zero** hunks in `internal/run/integrate.go`, `internal/resolve/resolve.go`, `internal/integrate/integrate.go`, `internal/run/batch.go` (except tests in `batch_test.go` if 3.6 needed them).

---

## Phase 9: Cleanup

- [x] 9.1 `internal/run/run.go`: short comment at the `len(AllowedPaths) > 0` gate stating why `HEAD~1` is forbidden (0-commit / multi-commit).
- [x] 9.2 `internal/dag/`: comment on `Split` that stdout is the wave plan and a `waves.json` must not be added.
- [x] 9.3 `cmd/lucind-ai/cli.go`: comment on the `DisjointAllowedPaths` call that it must stay before `ExecuteBatch` so Create is not the first overlap-failure side effect.
- [x] 9.4 Drop unused helpers introduced during RED.

---

## Out of scope (do not add tasks)

- Read-only / empty-`AllowedPaths` handling — `read-only-packet-dispatch`.
- Verify-phase dual dispatch — `verify-dual-dispatch`.
- Glob support, enforcing `external_changes`, requiring `Envelope.Commit`.
- In-process `lucind-ai run --dag` sequencer.
- Inferring a DAG from `tasks.md` prose.
- New ledger columns or event types.
- `--json` / `.lucind/runs/<id>.json`.
- Any edit under `openspec/changes/approvals-web-ui/` or `openspec/changes/read-only-packet-dispatch/`.
