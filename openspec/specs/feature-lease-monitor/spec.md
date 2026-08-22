# Feature Lease Monitor Specification

## Purpose

Serve a shell-free dashboard interface for monitoring feature lifecycle states, active lease holders, fence counters, integration attempt candidate SHAs, and classified overlap evidence payloads.

## Requirements

### Requirement: Shell-Free Feature and Lease Monitoring

The UI MUST display feature lifecycle status, active lease owner, monotonic
lease fence counter, latest integration attempt status with candidate commit
SHA, and classified overlap evidence payloads from ledger-backed queries. It
MUST NOT execute shell or git subprocesses to obtain that data.

#### Scenario: Active lease and attempt inspection

- GIVEN a feature with an active lease, fence counter, and integration attempts
- WHEN the operator opens the feature-lease monitor
- THEN the UI MUST display feature status, lease owner, fence counter, attempt
  status, and candidate SHA

#### Scenario: On-demand overlap evidence display

- GIVEN a feature with classified overlap evidence rows
- WHEN the operator expands the overlap evidence panel
- THEN the UI MUST render the evidence class, hash, and escaped JSON payload
  without polling that payload during hot refresh

#### Scenario: Expired lease queried without shell subprocess

- GIVEN a feature whose lease expiration timestamp is in the past
- WHEN the operator inspects leases
- THEN the response MUST return the recorded expiration timestamp and owner
  without spawning git or shell subprocesses
