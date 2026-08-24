# Dependencies and defects

Load this module when a defect, blocker, cross-Change effect, or Dependency appears.

## Current manual contract

Issue #4 automation is absent today. `lucind-ai` does not automatically perform Defect Assessment, create a Defect Record, prepare a fix Change, create an External Work Item, or launch remediation. Do not promise automatic fix Changes.

The Orchestrator must currently:

1. Classify whether the defect originates inside this Change, is pre-existing, or is shared.
2. Record evidence, affected Lanes, affected Changes, and disposition in durable Shared Memory.
3. Block only affected Lanes. Let unaffected Lanes complete.
4. Keep a defect caused by this Change inside it. Create a separate proposed fix Change for pre-existing or shared defects.
5. Record a Dependency when another Change must integrate before this one can safely continue.
6. Ask for one human confirmation before external tracking or remediation activation. Integration of the prerequisite only makes the blocked Change eligible to resume; its Orchestrator approves resumption.

## Evidence boundary

Current runtime evidence includes packet scope, Lane status and reason, result envelopes, check output, ledger events, integration attempts, overlap evidence, reconciliation records, and git objects. It does not establish product impact, defect origin, or whether an External Work Item is warranted; those remain Orchestrator judgments.

A `blocked` result must include the decision question and recommendation. Read it, verify the evidence, answer the decision point, and resume through the approved strategy rather than guessing inside the Lane.
