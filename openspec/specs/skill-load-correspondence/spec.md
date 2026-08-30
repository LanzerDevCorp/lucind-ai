# Skill Load Correspondence Specification

## Purpose

Specify result envelope schema declaration and validation for loaded agent skills.

## Requirements

### Requirement: Result envelope skills loaded declaration

The result envelope schema MUST accept an optional `skills_loaded` property declaring the skills loaded by the executing agent, and MUST reject unexpected properties under strict schema validation.

#### Scenario: Complete skills loaded accepted

- GIVEN required skills `["lucind-executor", "lucind-fan-out-lens", "sdd-propose"]` and an envelope declaring matching `skills_loaded`
- WHEN the envelope is validated against the result schema
- THEN validation MUST succeed.

#### Scenario: Envelope without skills_loaded remains valid

- GIVEN a result envelope that omits `skills_loaded` and carries every required field
- WHEN the envelope is validated against the result schema
- THEN validation MUST succeed.
