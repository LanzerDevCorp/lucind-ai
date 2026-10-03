# HMAC Test Attestation

`lucind-ai attest` provides deterministic, tamper-evident test attestation. It allows an orchestrator or automated agent to verify that a required test command has already run and passed against the current state of a repository's working tree, avoiding redundant test execution when nothing has changed.

## Commands

### `lucind-ai attest run -- <command> [args...]`

Executes the specified command in the current working directory:
- Passes standard input, output, and error streams through directly.
- Records timestamps (`started_at` and `finished_at`).
- After the command terminates, computes a git tree hash of the working tree (including uncommitted edits and untracked, non-ignored files) without modifying the repository's real git index.
- Computes an HMAC-SHA256 signature (`mac`) across canonical fields of the attestation entry.
- Atomically writes the attestation record to the state directory and marks it read-only (`0444`).
- Exits with the target command's exact exit code. Even if the command fails (`exit_code != 0`), an attestation entry is recorded; failing entries never count as a passing attestation.

Example:
```bash
lucind-ai attest run -- go test ./...
```

### `lucind-ai attest verify --command "<exact command string>"`

Verifies that a valid attestation exists for the specified command in the current repository:
- Exits `0` only if an attestation entry exists where:
  1. `command` matches the exact string.
  2. `exit_code == 0`.
  3. `tree_hash` matches the current working tree hash.
  4. The HMAC-SHA256 signature (`mac`) verifies with the local secret key.
- Otherwise, exits `1` and prints a single short reason to stderr:
  - `no entry`: No attestation entry found for this command in this repository.
  - `tree changed`: The working tree has uncommitted edits, untracked files, or deletions since the last test run.
  - `tests failed`: The recorded test execution failed (`exit_code != 0`).
  - `bad mac`: The attestation entry was forged, corrupted, or tampered with.

Example:
```bash
lucind-ai attest verify --command "go test ./..."
```

## Threat Model

The attestation mechanism's threat model is **accidental, not adversarial**:
- **Protected against**: Accidental repeats, out-of-order execution, stale cached results, forgotten uncommitted edits, newly added untracked files, deleted files, and inadvertent corruption.
- **Out of scope**: Adversarial circumvention by the local user. The secret signing key is owned by and readable by the local user account (`mode 0600`). A user or process running under the same UID with access to the key file could technically compute valid HMAC signatures. The purpose is coordination integrity and reliable orchestration, not defense against a hostile local user.

## Secret Key and Storage

- **Key location**: `$XDG_CONFIG_HOME/lucind-ai/attest.key` (fallback: `~/.config/lucind-ai/attest.key`).
  - 32 cryptographically secure random bytes generated on first use.
  - File permissions are restricted to `0600`.
  - Keys are never written inside any repository or printed to logs/terminals.
- **Log location**: `$XDG_STATE_HOME/lucind-ai/attestations/<repo_id>/` (fallback: `~/.local/state/lucind-ai/attestations/<repo_id>/`).
  - `<repo_id>` is the SHA-256 hex string of the absolute path to the repository top-level.
  - Individual entries are stored as `<finished_at unix nanos>-<tree_hash[:12]>.json`.
  - Written atomically via temporary file and rename, then set to `0444` (read-only).

## Working Tree Hashing

To ensure any uncommitted change or untracked file invalidates attestation:
1. A temporary index file path is created outside the repository.
2. The index is seeded using `git read-tree HEAD` with `GIT_INDEX_FILE` pointing to the temporary index.
3. Uncommitted and untracked (non-ignored) files are added using `git add -A`.
4. The tree object is written using `git write-tree`.
5. The temporary index file is deleted.

The repository's actual index (`.git/index`) is never read, locked, or modified.
