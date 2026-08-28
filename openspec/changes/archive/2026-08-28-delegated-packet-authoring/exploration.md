## Exploration: Delegated packet authoring

### Current State

The prior evidence is substantially correct, with several important qualifications:

- `packet.Parse` reconstructs the Markdown body after frontmatter, removes leading blank lines, and leaves a trailing newline; `run.Execute` then assigns that `Packet.Body` directly to `executor.Request.Prompt`. Executors receive no synthesized summary of frontmatter or DAG metadata. Runtime code consumes selected metadata separately.
- DAG validation is strong for graph and write-scope structure: required node fields, body-file existence, unique IDs, non-empty `allowed_paths`, dependency validity, cycles, and unordered overlap are checked before emit. Raw packet parsing is more permissive: it is line-oriented, ignores unknown keys, and only requires `id`, `executor`, `routed_by`, and a non-empty body.
- `read_only_paths` is validated and emitted by `internal/dag`, but `packet.Parse` does not parse it. It therefore disappears before dispatch and is not added to the executor prompt. The current agent must learn those read-only inputs from hand-authored body text, if they are mentioned at all.
- Body semantic validation is weak. Production parsing does not require Goal, criteria, hard stops, output instructions, or correspondence between body declarations and the result. Asset tests protect selected shipped templates, not arbitrary packets presented to `lucind-ai run`.
- The historical delivery failure is verified. `openspec/changes/archive/2026-08-20-approvals-web-ui/state.yaml` records two correctly completed and committed lanes that blocked because a stale template omitted the `.lucind/result.json` instruction. The current contract and troubleshooting docs explicitly describe this failure mode.
- Post-execution Git enforcement is now strong: declared write scope is checked against a recorded-base four-way diff, write packets require unique commits plus clean porcelain, and read-only packets require no unique commits plus clean porcelain. Earlier commit-cleanliness concerns are therefore closed at runtime, although authoring text can still contradict the enforced mode.
- Result schema validation is structural, not correspondent. `hard_stops` is required but may be empty; `done_criteria` and `files_changed` are optional. Neither `result.Read` nor `run.Execute` proves that every body criterion/stop was echoed, that `files_changed` matches the diff, or that the optional `commit` matches the candidate.
- Mechanical Acceptance closes part of that gap by comparing `files_changed` paths to the frozen candidate diff and rejecting fired stops, unmet criteria, external changes, or out-of-scope paths. It still cannot detect omitted criteria/stops because no normalized authoring contract is frozen. It also does not validate `files_changed.change` classifications or compare `Envelope.Commit` to the candidate commit.
- `files_changed` has conflicting semantics. The JSON schema says every path changed inside the worktree, and `.opencode/agent/lucind-dag.md` explicitly includes `.lucind/result.json`; Acceptance compares against the committed base-to-candidate Git diff, where ignored `.lucind/result.json` is absent. Following the specialist guidance can therefore make Acceptance reject an otherwise valid candidate.
- Specialist routing already exists: an opencode packet may name a primary agent through `agent`, and the executor fail-closes opencode's silent subagent-to-default fallback. The only repository agent today is the apply-specific `lucind-dag`; there is no general packet-author specialist or shadow comparison flow.
- Reusable templates support late binding only for legacy-main dispatch through CLI flags. Feature-targeted packets still need the four-field target tuple rendered into packet frontmatter; an authoring specialist must not invent those live values.

### Affected Areas

- `internal/packet/packet.go` — current parsed packet model, permissive frontmatter parser, and verbatim body seam.
- `internal/packet/packet_test.go` — parser and shipped-template contracts; currently tests assets rather than arbitrary body semantics.
- `internal/dag/{parse,validate,emit,split}.go` — typed DAG source, `read_only_paths`, deterministic frontmatter emit, and the existing rendering precedent.
- `internal/run/run.go` — dispatch admission, prompt construction, scope/commit enforcement, packet digest, and frozen Acceptance candidate.
- `cmd/lucind-ai/cli.go` — whole-batch preflight and the likely thin CLI adapter for compile/validate/shadow workflows.
- `internal/result/result.go` and `internal/result/result.schema.json` — result structure that must correspond to the authored criteria, hard stops, mode, commit, and changed paths.
- `internal/accept/accept.go` — frozen result/diff validation and exact Acceptance binding.
- `internal/ledger/acceptance.go` and lane-candidate persistence — possible storage for a canonical authoring-contract digest or bytes if Acceptance must independently re-check correspondence.
- `internal/executor/opencode.go` — already supports a named primary specialist and fail-closed fallback detection.
- `plugin/claude-code/skills/lucind-ai/references/contracts/packets-results.md` — public packet/result semantics and manual-author compatibility policy.
- `plugin/claude-code/skills/lucind-ai/assets/*packet-template.md` — current hand-authored render sources and migration inputs.
- `.opencode/agent/lucind-dag.md` — existing specialist precedent plus stale `files_changed` and leading-frontmatter guidance.
- `plugin/claude-code/skills/lucind-ai/SKILL.md` — orchestrator routing, shadow-mode operation, and explicit non-cutover policy.

### Approaches

1. **Deep typed compiler with manual and delegated adapters** — Introduce a pure packet-authoring module whose small interface compiles a target-free typed contract plus a late target binding into canonical packet Markdown and a normalized manifest. Existing manual packets use a compatibility adapter; a new opencode specialist emits the same typed contract and is compared in shadow mode.
   - Pros: Makes body/result invariants machine-readable; centralizes validation, deterministic ordering/rendering, target binding, and diagnostics; gives manual and delegated authoring one test surface; enables field-level shadow comparison without allowing an LLM to write live SHAs.
   - Cons: Requires an explicit version/migration policy and likely touches result/Acceptance persistence if independent correspondence checks are required.
   - Effort: High

2. **Deepen `internal/packet` with Markdown semantic linting** — Keep raw Markdown canonical and add heading/token parsers plus stricter pre-dispatch validation directly around `packet.Parse`.
   - Pros: Smaller format change; manual packets remain the primary artifact; catches the historical missing-result instruction quickly.
   - Cons: Markdown becomes an unstable machine protocol; phase-specific templates make exact correspondence brittle; deterministic rendering, shadow comparison, and target late binding remain scattered.
   - Effort: Medium

3. **Agent-and-template hardening only** — Add a general packet-author agent, improve templates, and compare raw generated Markdown in shadow mode without a Go contract module.
   - Pros: Lowest production-code cost and fastest experiment.
   - Cons: Repeats the already-demonstrated failure class: prose and tests can drift, arbitrary packets bypass asset checks, and raw-text diffs provide weak evidence for cutover.
   - Effort: Low

### Recommendation

Choose the deep typed compiler. The proposed seam is a new `internal/packetauthor` module with one compact external operation:

```go
type Compiler interface {
    Compile(Contract, TargetBinding) (Artifact, error)
}
```

`Contract` should be target-free and versioned. It should type route intent, execution mode, `allowed_paths`, `read_only_paths`, Goal, preconditions, out-of-scope statements, context citations, ordered done criteria, ordered hard stops, and the fixed result-delivery contract. `TargetBinding` should be a validated sum type: feature target or legacy-main target. `Artifact` should contain canonical Markdown, the parsed `packet.Packet`, a normalized manifest, and a stable digest. Map iteration must never affect output; field and section order, newline policy, JSON encoding, and path normalization must be fixed and golden-tested.

The module earns depth by owning all of these invariants behind `Compile`:

- reject incomplete route/mode/target combinations before worktree creation or quota use;
- always render explicit `.lucind/result.json` write and `.lucind/result.schema.json` validation instructions;
- render `read_only_paths` into a visible body section and preserve them in the normalized contract;
- generate mode-correct commit/cleanliness criteria that match runtime enforcement;
- derive body criteria and hard stops from the same ordered values later expected in the envelope;
- define `files_changed` as the canonical candidate Git diff, excluding `.lucind/**`, and share that definition with run and Acceptance;
- reject duplicate, missing, extra, or reordered-as-significant result declarations according to an explicit set/list policy;
- bind live target values only at compile/dispatch time, never in specialist-authored content.

Migration should be additive and fail closed where history proves it must:

1. Keep `packet.Parse` and successful manual packet prompt bytes unchanged. Add universal pre-dispatch checks for the fixed result path/schema instructions and metadata contradictions; unsafe legacy packets should fail before dispatch with actionable diagnostics.
2. Mark compiler-produced packets with a versioned machine contract. Strict body/result correspondence applies to these packets; legacy manual packets remain compatibility-mode until deliberately regenerated.
3. Freeze enough normalized contract evidence with the lane candidate for Acceptance to independently verify expected criteria/stops, mode, commit semantics, and changed-path semantics. A digest alone binds bytes but cannot reveal an omitted declaration.
4. Add a primary, permission-bounded packet-author specialist that outputs typed contract data, not Markdown and not target SHAs. The deterministic compiler remains the sole renderer.
5. Run that specialist only in shadow mode: feed it the same authoring inputs as the manual path, compile both with the same late target binding, and record validation outcome, normalized field diff, rendered digest diff, and failure class. The manual artifact remains canonical and solely dispatchable.
6. Treat specialist timeout, invalid JSON, schema failure, unavailable route, or fallback-agent detection as shadow observations only. They must never block or replace canonical manual dispatch in this Change.

Test through the module interface with table-driven contract cases, golden deterministic renders, mutation tests for every mandatory result instruction, property/fuzz tests for path and ordering normalization, manual compatibility fixtures, exact body/result correspondence cases, Acceptance fixtures, and a fake specialist adapter for shadow comparisons. Do not test internal formatting helpers directly when an interface-level compile assertion proves the same behavior.

Canonical automatic cutover is intentionally excluded. A later Change may consider it only after shadow evidence defines thresholds for validity rate, semantic-equivalence rate, deterministic stability, failure recovery, and operator review cost.

### Risks

- A normalized contract that is too broad becomes a shallow mirror of every Markdown phrase; keep free-form prose behind a small typed invariant set.
- Requiring strong correspondence for all historical manual packets immediately could create avoidable migration breakage; version strictness while enforcing the indispensable result-delivery baseline universally.
- Persisting full normalized contract evidence may require a ledger schema migration and can push the single-PR change toward the 3000-line review budget.
- Sharing changed-path semantics across run, result, and Acceptance can expose rename/deletion edge cases currently hidden by `git diff --name-only`.
- Shadow comparisons are meaningless if manual and specialist paths receive different source facts or target timing; both must use the same normalized input and late binding.
- `openspec/config.yaml` still records a 2000-line review budget, while this launch specifies 3000. The orchestrator should reconcile that before task planning; this exploration did not modify project configuration.

### Ready for Proposal

Yes. The proposal should commit to the typed compiler seam, versioned manual compatibility, shared result/Acceptance semantics, and a non-authoritative shadow specialist. It must state that automatic canonical cutover is deferred to a separate evidence-gated Change.
