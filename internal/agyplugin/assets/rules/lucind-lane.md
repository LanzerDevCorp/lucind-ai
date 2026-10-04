---
trigger: model_decision
description: Only when working inside a lucind-ai lane (the LUCIND_LANE environment variable is set and the task brief lives in .lucind/lanes/<id>/brief.md).
---

# lucind-ai lane contract

You are running inside a lucind-ai lane. Read `.lucind/lanes/$LUCIND_LANE/brief.md` first.

- Edit only files matching the lane's allowed paths (`allow` in `.lucind/lanes/$LUCIND_LANE/lane.json`).
  A hook denies file writes outside them; shell writes are caught later by `lucind-ai accept`.
- Never read the attest key or touch the attestations directory; the hook denies it.
- Before stopping, write your result envelope to `.lucind/lanes/$LUCIND_LANE/result.json`
  (see the `lucind-result` skill for the exact shape). Stopping without a valid envelope
  sends you back to fix it, at most twice, after which the lane is marked failed.
- Finish by running exactly the `lucind-ai attest run -- sh -c '<check>'` commands listed
  in the brief's `## Lane Contract` on the final tree so `accept` can reuse the attestation
  (run nothing when the contract lists none).
- If the brief has a `## Key Learnings` instruction, follow the `lucind-result` skill: persist the
  learnings with Engram `mem_save` and close your final response with the `## Key Learnings` block.
  They never go in `result.json`.
