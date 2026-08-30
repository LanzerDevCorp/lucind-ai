# Phase Verdict Reporting Specification

## Purpose

Define the compressed Phase Verdict a phase Specialist returns to its Orchestrator after accepting or rejecting its assigned phase.

## Requirements

### Requirement: Structured Phase Verdict Reporting

The phase Specialist MUST return a structured Phase Verdict reporting `outcome` (`accepted` or `needs-revision`), `canonical_artifact_path`, and `unresolved_divergence` to the Orchestrator. The Specialist MUST NOT include raw result envelopes, candidate diffs, or full synthesis notes unless the Orchestrator explicitly requests them. On a `needs-revision` verdict, the Orchestrator MUST dispatch at most one bounded correction rather than a full phase re-fan-out.

#### Scenario: Successful phase completion returns compressed verdict

- GIVEN an accepted phase synthesis producing a canonical planning artifact
- WHEN the Specialist reports phase completion to the Orchestrator
- THEN the Phase Verdict contains outcome `accepted`, the canonical artifact path, and empty unresolved divergence without raw transcripts or full synthesis notes

#### Scenario: Synthesis divergence triggers bounded correction

- GIVEN synthesis notes identifying an unresolvable contradiction between planning lens drafts
- WHEN the Specialist reports the phase outcome to the Orchestrator
- THEN the Phase Verdict contains outcome `needs-revision` with populated `unresolved_divergence`, and the Orchestrator dispatches at most one bounded correction instead of a full re-fan-out

#### Scenario: Unstructured evidence delivery rejected

- GIVEN a phase completion report containing raw multi-lane transcripts or unparsed logs instead of structured fields
- WHEN the report is received by the Orchestrator
- THEN the submission is rejected and structured phase verdict fields are required
