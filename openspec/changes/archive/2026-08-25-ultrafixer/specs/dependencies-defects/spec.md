# Delta for dependencies-defects

## MODIFIED Requirements

### Requirement: Structured ultrafixer defect triage coordination

The defect coordination contract (`plugin/claude-code/skills/lucind-ai/references/coordination/dependencies-defects.md:7-23`) MUST be updated from a manual assessment contract to specify automated ultrafixer packet dispatch for pre-existing defect triage and remediation. When a failing check is encountered during feature development, the Orchestrator MUST dispatch an on-demand ultrafixer packet (`plugin/claude-code/skills/lucind-ai/assets/ultrafixer-packet-template.md`) via `lucind-ai run --packet` instead of manually performing origin classification and fix Change generation. The Orchestrator MUST handle `blocked` result envelopes by reviewing the question and recommendation and presenting them to the human operator for CAS promotion (`lucind-ai integrate retry`).

#### Scenario: Orchestrator dispatches ultrafixer packet upon check failure

- GIVEN a test, linter, or build failure encountered during feature development
- WHEN the Orchestrator assesses the defect
- THEN the Orchestrator MUST generate and dispatch an ultrafixer packet carrying the failing command and error transcript in Context

#### Scenario: Human Orchestrator processes blocked result envelope

- GIVEN a `blocked` result envelope emitted by an ultrafixer lane with a repair commit and recommendation question
- WHEN the Orchestrator reviews the triage outcome
- THEN the Orchestrator MUST present the decision to the human operator for CAS promotion via `lucind-ai integrate retry` rather than self-integrating

#### Scenario: Feature-local regressions remain in feature lane

- GIVEN an ultrafixer lane that exits `done` indicating the defect was introduced by the feature branch
- WHEN the Orchestrator receives the result
- THEN the active feature lane MUST remain responsible for fixing its own regression without a separate fix Change
