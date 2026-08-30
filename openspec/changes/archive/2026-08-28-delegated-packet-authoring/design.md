# Design: Delegated Packet Authoring

## Technical Approach

`internal/packetauthor` exposes `Compile`, `AdmitManual`, `AdmitBatch`, and `Compare`. `runDispatch` gates batches before side effects. Manual Markdown remains canonical/byte-identical; specialist JSON observational. One 3000-line PR; apply requires GPT-5.6 Sol Fast.

## Architecture Decisions

| Decision | Choice and rationale |
|---|---|
| Rendering | `packet-author/v1`/`packet-manifest/v1`: struct-ordered UTF-8 JSON, LF, no HTML escaping, one terminal LF; criteria/stops reject duplicates, paths byte-sort, and domain-separated length-prefixed SHA-256 is deterministic. |
| Authority | Target-free contracts bind feature or `--legacy-main` immediately before admission; stale `ExpectedParentSHA` fails. Specialist targets/side effects are forbidden. |
| Changes | `candidatechange` owns NUL-safe `git diff --name-status -z -M -C --find-copies-harder`; `.lucind/**` is excluded. One canonical copy is exactly `{"change":"copied","source_path":"<normalized source>","path":"<normalized destination>"}` (this struct key order). Sorting is `(path bytes, change rank created<modified<deleted<copied, source_path bytes)`. |
| Rollout | Shadowing is opt-in, additive, manual-canonical, and never auto-cuts over. |

## Data Flow

`manual/Compile(binding) → AdmitBatch → ExecuteBatch → evidence → Acceptance`; shadow runs `validate → Compile → Compare → isolated store` and is never selected. `ReadOnlyPaths` reaches assignment/`Request` without write authority or `Body` changes.

## Manual Admission Grammar

Scan a copy after CRLF/lone-CR→LF; never assign it to `Packet.Body`. `trim` removes ASCII SP/HT. `OPEN="\x60\x60\x60lucind-result-contract"`, `CLOSE="\x60\x60\x60"`, and `trigger(l)=Contains(trim(l),"lucind-result-contract")`; exact markers are unindented whole lines.

```text
if any trigger:
  require exactly one OPEN; require line OPEN+6 == CLOSE
  require lines OPEN+1..OPEN+5, exactly in this order:
    version: 1
    path: .lucind/result.json
    schema: .lucind/result.schema.json
    mode: write|read-only
    commit: required|forbidden
  require no other/blank/duplicate/unknown field or trigger; mode/commit must match metadata
else: compatibility(body)
```

Markers are mandatory only for structured bodies; absence selects compatibility. Any malformed/duplicate/extra/unclosed marker fails closed. Compatibility ignores fences of 0–3 spaces then ≥3 identical U+0060 or `~` bytes, closed by at least that count. It requires exactly one raw `## Done criteria` followed later by one `## Return`; each ends at the next raw `^#{1,2}[ \t]+` heading or EOF. For matching only, delete `*`, `_`, and backticks outside fences, collapse ASCII SP/HT/LF to one SP, and trim; punctuation, case, and Unicode remain unchanged. Return recognizes exactly one occurrence of each sequence and ignores other text:

- `Write the result envelope to .lucind/result.json in this worktree.`
- either `Validate it against .lucind/result.schema.json before writing.` or `The schema is at .lucind/result.schema.json in this worktree. Validate against it before writing`

No NLP is used. ASCII-case-fold non-fenced text only for commit checks: required regex `(^|[.!?] )(commit |create a commit|after you commit)`; forbidden substrings `do not commit` or `no unique commit`. Both classes, read-only+required, or write+forbidden conflict. Success returns original bytes.

| Input | Diagnostic |
|---|---|
| exact structured block matching metadata | none; admit |
| duplicate/unknown/unclosed structured block | `PA_MANUAL_MARKER_INVALID` |
| missing fixed write/schema phrase | `PA_RESULT_PATH_MISSING` / `PA_RESULT_SCHEMA_MISSING` |
| `read_only:true` plus “After you commit” | `PA_MODE_COMMIT_CONFLICT` |

## Diagnostic Order

| Rank | Validator/code family |
|---:|---|
| 10 | manual structure/`PA_MANUAL_MARKER_INVALID` |
| 20 | result path/`PA_RESULT_PATH_MISSING` |
| 30 | result schema/`PA_RESULT_SCHEMA_MISSING` |
| 40 | route/`PA_ROUTE_INVALID` |
| 50 | mode/`PA_MODE_COMMIT_CONFLICT` |
| 60 | forbidden target/`PA_FORBIDDEN_TARGET` |
| 70 | target completeness/`PA_TARGET_INCOMPLETE` |
| 80 | target freshness/`PA_TARGET_STALE` |
| 90 | path validity/`PA_PATH_INVALID` |

All independent validators run; freshness is skipped only without a complete typed target. Whole-batch diagnostics sort by `(packet argv index, numeric rank, field UTF-8 bytes, item index with scalar=-1, code bytes, message bytes)`; exact duplicate tuples collapse. Any diagnostic rejects the batch without side effects.

## Evidence, Copies, and Shadow Transactions

Frozen `lane-authoring-evidence/v1` JSON records packet digest, legacy/versioned contract, binding, obligations, base/candidate commits+trees, canonical changes, and result hash; its hash omits itself. `binding:v2` binds these. Per shadow attempt: compare; begin one SQLite transaction; insert attempt/review; commit. Failure's only effect is rollback of that transaction, attaching `{attempt_index,code:"SHADOW_EVIDENCE_PERSIST_FAILED",stage:begin|insert|commit}` without DB text, then continuation. Warnings sort by attempt then stage.

| Verification | Scenario |
|---|---|
| Copy correspondence | Freeze the exact entry; separately scope-check both endpoints; require it in the result; Acceptance independently recomputes it. RED altered/omitted endpoints reject; both in-scope accept. |

## File Changes and Testing

Create `internal/packetauthor/*`, `internal/candidatechange/*`; modify `cmd/lucind-ai/{cli.go,packet_authoring.go}`, `internal/{packet,executor,run,result,accept}/*`, ledger schema/acceptance/shadow, and specialist/templates. Strict TDD covers grammar bytes/diagnostics, replay, feature/legacy staleness, copies, read-only visibility, Acceptance, isolated shadows, and v9→v10.

## Threat Matrix

| Boundary | Applicability; behavior/RED test |
|---|---|
| Documentation-like paths | N/A—no executable classification change. |
| Git repository selection | Applicable—canonical absolute roots and argv-only `git -C`; reject mismatches; RED relative/absolute/symlink selectors. |
| Commit state | Applicable—four-way union then clean frozen candidate; RED staged, `commit -a`, empty-index, rename, copy. |
| Push state | N/A—no push. |
| PR commands | N/A—no PR automation. |

## Migration / Rollout

`v9→v10` adds `lane_candidates(authoring_evidence_version,authoring_evidence_json,authoring_evidence_hash)`, `acceptance_receipts(binding_version,contract_version,authoring_evidence_version,authoring_evidence_hash)`, `packet_author_shadow_attempts(id,run_id,lane_id,input_hash,specialist_identity,failure_class,valid,equivalent,diff_json,manual_digest,specialist_digest,replay_stable,latency_ms,created_at)`, and `packet_author_shadow_reviews(attempt_id,reviewer,review_ms,created_at)`. Existing rows read `legacy/v1`; new rows require recomputable hashes. Disable shadow first; storage may remain inert, without packet conversion.

## Open Questions

None.
