# Triage Evaluation Rubric Specification

## Purpose

Offline dual-judge grading of the same fixture, scoring hunk classification rather than a prepared resolution.

## Requirements

### Requirement: Dual-judge rubric isolation

The offline rubric MUST run the same three-hunk fixture on the registered `claude`/`claude-opus-5` and `opencode`/`openai/gpt-5.6-sol` judges without cross-provider configuration or billing leaks. The win criterion MUST be correct classification of the three hunks: the business hunk separated from the two mechanical controls, with ARBITRARY declared on the business hunk. An evaluation that scores all three hunks alike MUST fail. The rubric MUST NOT grade the prepared resolution and MUST NOT time a human. This requirement does not name a production triage runtime.

#### Scenario: Rubric grades distinct three-hunk classification

- GIVEN identical three-hunk fixture evidence evaluated by `claude-opus-5` and `openai/gpt-5.6-sol`
- WHEN the rubric grades the outputs
- THEN it awards a passing score only when the business hunk is distinguished from the mechanical controls and flagged ARBITRARY

#### Scenario: Uniform hunk scoring fails

- GIVEN a judge evaluation that assigns the same class or risk to all three hunks
- WHEN the rubric scores the evaluation
- THEN it MUST reject the evaluation

#### Scenario: Judges do not share provider configuration

- GIVEN both pinned judges running the same fixture
- WHEN the rubric executes each evaluation
- THEN each run MUST use only its registered executor and model, with no cross-provider configuration or billing data
