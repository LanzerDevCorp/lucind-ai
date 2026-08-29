# Exclusive Mode

Load this module when one Change exclusively owns the primary workspace.

## Current mapping

Exclusive Mode is the canonical product term. Current packet and CLI syntax call this runtime path `legacy_main`; preserve that spelling only at the implementation boundary.

Use `legacy_main: true` in packet frontmatter or `--legacy-main` at dispatch, and supply `--expected-parent-sha`. Neither value alone is admissible. Despite the legacy name, Promotion uses the current checked-out branch; verify that checkout is the intended Integration Target before dispatch.

## Constraints

- The primary working tree must remain clean and receives the fast-forward merge.
- No Ownership Lease or cross-feature overlap gate protects this path.
- Lane worktrees start from primary-checkout `HEAD`.
- Post-execution integration may bisect a failing batch to the clean subset. This recovery occurs only after all Lanes execute.
- Exclusive Mode is fixed for the Change lifetime. Do not switch a running Change between feature-targeted and `legacy_main` dispatch.
