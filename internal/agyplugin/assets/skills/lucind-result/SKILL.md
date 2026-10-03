---
name: lucind-result
description: Write the lucind-ai result envelope (result.json) and attest the final tree when finishing work inside a lucind-ai lane. Use when LUCIND_LANE is set or the brief points at .lucind/lanes/<id>/brief.md.
---

# lucind-result

Inside a lane (`LUCIND_LANE=<id>`) you finish by writing
`.lucind/lanes/<id>/result.json`. It is validated against a strict JSON schema
(unknown top-level properties are rejected).

## Envelope

```json
{
  "lane_id": "<lane id>",
  "status": "done",
  "summary": "Two or three sentences on what was actually done.",
  "hard_stops": [],
  "files_changed": [
    {"path": "src/a.go", "change": "modified", "why": "optional"}
  ],
  "done_criteria": [
    {"criterion": "tests pass", "met": true, "evidence": "go test ./... ok"}
  ],
  "commit": "optional sha",
  "findings": [
    {"finding": "something unrelated but important", "evidence": "file:line"}
  ]
}
```

Required: `lane_id`, `status`, `summary`, `hard_stops` (one `{hard_stop, fired, note}` entry
per hard stop in the brief; `[]` when the brief lists none).

`status` is one of:

- `done`: every criterion met and no hard stop fired. Only this status passes the lane.
- `blocked`: a decision is needed that the brief does not authorize. Add `questions`
  (`question`, `why_blocking`, optional `options`, `recommendation`).
- `deviated`: you had to depart from the brief. Add `deviations` (`expected`, `actual`, `reason`).
- `failed`: technical failure.
- `interaction_required`: you need an answer to proceed. Add `interaction`
  (`question`, `reason`, `unblock_response`).

`files_changed[].change` is `created|modified|deleted|copied` (`copied` also needs `source_path`).
Paths outside the repository go in `external_changes` (`path`, `change`, `why`, `revert`).

Any status other than `done` ends the lane as failed without a retry; a missing or
schema-invalid file sends you back to fix it (at most twice).

## Attest the final tree

After your last edit, run exactly:

```bash
lucind-ai attest run -- sh lucind-checks.sh
```

Do not edit files afterwards: the attestation is bound to the exact tree, and `lucind-ai accept`
rejects a stale one. Never read the attest key or the attestations directory.
