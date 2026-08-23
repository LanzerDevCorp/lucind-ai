# Tasks Synthesis Notes: Conflict Triage Fixture

## Unresolved Contradictions

- No draft contradiction refutes Lens A's authoritative four-phase decomposition. A, B, and C independently converge on the same four requirements.
- The exact non-decreasing risk formula/thresholds and the executor/model for production triage remain open exactly as required; neither is settled in the canonical checklist.
- Lens C's “Low” nominal forecast is reconciled as High for the skill-required 400-line field, while the human-approved 2000-line single-PR budget supplies the accepted `size-exception`; no chain decision is needed.

## Coverage Gaps

- The 101 distinct citation tokens across the three drafts/manifests were audited against source and claim support: 93 are retained as supported; 8 are dropped or retargeted below.
- Cloud execution of `claude-opus-5` and `openai/gpt-5.6-sol` is not proven: existing executor tests use temporary subprocess stubs (`internal/executor/claude_test.go:15-25`, `internal/executor/opencode_test.go:16-26`). The rubric must remain offline and must not choose the production triage runtime.
- The packet forbids Go test/build execution here. Integrate viability is therefore a reading judgment: one sequential packet keeps RED and GREEN together and makes the combined-tree gate the apply responsibility; no split wave is shipped independently.
- Push state and PR commands are explicitly N/A, so no RED tests were created for them. Candidate output is existing TEXT storage, not a schema migration (`internal/ledger/schema.go:156-166`).

## Dropped Citations

1. `internal/resolve/candidate.go:26` was cited as returning semantic ambiguity; it only declares the sentinel. The canonical text cites the declaration/prompt and requires triage not to return it.
2. `internal/resolve/candidate_test.go:16-49` was cited as proving documentation/script scanning, NUL-binary skipping, and no execution; it only tests clean/text marker files. The new RED test owns those cases.
3. `internal/resolve/candidate.go:100-145` was cited as the complete four-way check and error return; the function's union completion and `ErrOutOfScopeEdits` return are at `:154-168`. Canonical references use `:46-100,107-168`.
4. `internal/resolve/candidate_test.go:51-93` was cited for staged-and-untracked disjointness; it proves only ordinary in-scope and out-of-scope edits. The staged/untracked case remains a new RED test.
5. `internal/run/gate_test.go:122-160` was cited as proving the blocking result, but assertions begin after that range. The evidence is retargeted to the test's result assertions (`:122-169`).
6. `cmd/lucind-ai/cli_test.go:3126-3150` was cited as proving SHA registration, but it is test setup. The successful registration assertions are `:3199-3232`.
7. `internal/reconcile/reconcile_test.go:56-100` was cited as the complete exact-field proof, but status/evidence assertions continue through `:109`; the range is retargeted.
8. The shorthand `integrate.go:151-173` was not a repository-root path. It is retargeted to `internal/integrate/integrate.go:150-173`.

## Decomposition Divergence

- Lens B's Unit 1/3 then Unit 2/4 parallel waves are not canonical: they do not preserve A's sequential phase story, and strict TDD requires each RED and GREEN to stay in one unit. Their useful boundaries corroborate the four phases; their unpaired wave claims are excluded.
- Lens C's four capability units corroborate A, but its output seam is merged into Unit 1 and its acceptance rows are redistributed. The canonical checklist keeps A's task ordering while retaining C's threat rows, explicit focused verification, and 2000-line forecast.
- B's N-way simultaneous-resolution scenario and stream-decoder recovery scenario map to A's out-of-scope/implementation-detail boundary, so they are omitted. B's no-sidecar recommendation is retained as the single sequential packet shape.
- Every specification requirement maps to canonical tasks: fixture (3.1–3.3), triage (1.1–1.4 and 2.1–2.4), approval/CAS (1.3–1.4 and 3.4), and rubric isolation (4.1–4.2).
