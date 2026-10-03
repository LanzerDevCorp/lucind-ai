# Router and Jev Shadow Mode

This document describes the routing architecture in `lucind-ai`, the strict data-protection rule for routing signals, shadow runner semantics, usage log integration, and the open policy items regarding third-party model terms.

## Architecture

The routing subsystem (`internal/router`) decides how an implementation packet is routed (`inline`, `worker`, or `fanout`).

```
                    +---------------------------+
                    |  Packet & Base Repository |
                    +-------------+-------------+
                                  |
                                  v
                    +---------------------------+
                    | dispatchcheck / Signals   |
                    | (numbers & booleans only) |
                    +-------------+-------------+
                                  |
                                  v
                    +---------------------------+
                    |    router.Shadow Runner   |
                    +-------------+-------------+
                                  |
               +------------------+------------------+
               |                                     |
               v                                     v
+-------------------------------+   +---------------------------------+
|      router.Deterministic     |   |         router.Jev              |
|  (Primary / Permanent Basis)  |   |    (Candidate / Shadow Mode)    |
|   Decides route with 100%     |   |    HTTP to TypeSafe Jev API     |
|   authority; never fails      |   |    NO decision authority        |
+--------------+----------------+   +----------------+----------------+
               |                                     |
               |                                     v
               |                             Logs disagreements
               |                             and errors only
               v                                     |
+-------------------------------+                    v
|      Dispatch Execution       |   +---------------------------------+
|  (inline or worker lane)      |   |       internal/usagelog         |
+-------------------------------+   |    usage.jsonl router events    |
                                    +---------------------------------+
```

### Core Components

1. **`Router` Interface**:
   ```go
   type Router interface {
       Route(ctx context.Context, s Signals) (Decision, error)
   }
   ```
2. **`Deterministic` Router**:
   The baseline and permanent fallback. It inspects hard computable signals:
   - `AllowedPathCount >= 2`
   - `NewFile == true`
   - `RiskTierLevel >= 2` (high risk)
   If any hard signal holds, it routes to `worker`; otherwise `inline`. It never returns `fanout` (which requires declared intent, see T19), has confidence 1.0, and never errors.
3. **`Jev` Adapter**:
   An HTTP client adapter for the TypeSafe Jev SystemOne API (`POST https://api.typesafe.ai/v1/systemone`). It runs strictly as a candidate router without decision authority.
4. **`Shadow` Runner**:
   Executes the Primary router and returns its decision and error unchanged. It invokes the Candidate router solely to observe agreements, disagreements, and errors.

---

## The Data Protection Rule

**Jev must never receive text, file paths, repository identifiers, code diffs, or secrets.**

The signals sent to Jev are encapsulated in `router.Signals`:
```go
type Signals struct {
    AllowedPathCount int  `json:"allowed_path_count"`
    NewFile          bool `json:"new_file"`
    RiskTierLevel    int  `json:"risk_tier_level"` // 0 passive, 1 medium, 2 high
    ReadOnly         bool `json:"read_only"`
}
```

- **Invariant**: Every field in `Signals` is strictly numeric (`int`) or boolean (`bool`). This is enforced by reflection in `internal/router/router_test.go`.
- **Static Request Context**: The question instructions and choice criteria sent to Jev are fixed, static constants compiled into the binary. They describe abstract route characteristics (single agent vs delegated lane vs multi-lens exploration) and never contain repository-specific strings.
- **Credential Hygiene**: The API key is passed only in the HTTP `Authorization` header. It is never logged, printed to `stderr`/`stdout`, or included in error messages.

---

## Opt-In Configuration and Safety

Jev shadow mode is completely opt-in and disabled by default. No network calls are made unless explicitly enabled by the repository owner.

### Environment Variables

| Variable | Description |
|---|---|
| `LUCIND_JEV_API_KEY` | TypeSafe API key. Must be non-empty. |
| `LUCIND_JEV_SHADOW` | Must be explicitly set to `on`. |
| `LUCIND_JEV_URL` | Optional override for the API URL (defaults to `https://api.typesafe.ai/v1/systemone`). |
| `LUCIND_USAGE_LOG` | Set to `off` to suppress appending router shadow events to the usage log. |

If either `LUCIND_JEV_API_KEY` is empty or `LUCIND_JEV_SHADOW` is not `on`, routing runs purely deterministically with zero network calls.

### URL Safety Rules

- `BaseURL` must use `https://`.
- Plain `http://` is allowed **ONLY** when the target host is a loopback address (`127.0.0.1`, `::1`, `localhost`) to enable mock testing via `httptest.Server`.
- Any non-loopback HTTP address is rejected at instantiation before any connection is attempted.

---

## Shadow Semantics

The shadow runner guarantees:
1. **Zero Authority**: The Candidate decision is ignored. The dispatch verdict is governed solely by `Deterministic` and the dispatch-threshold validator.
2. **Fail-Safe Isolation**:
   - Timeouts are bounded by `DefaultJevTimeout` (2 seconds).
   - Network errors, HTTP error statuses, malformed JSON, and unexpected choices never disrupt dispatch.
   - Candidate panics are recovered and converted to a logged error event.
3. **No Retries**: Shadow requests are fire-and-forget; no retries are attempted to avoid adding latency to dispatch.
4. **Bounded Memory**: Jev response bodies are bounded to 1 MiB (`MaxJevResponseBodyBytes`).

---

## Usage Log Events

When shadow mode is active, events are appended as JSON lines to the usage log (`$XDG_STATE_HOME/lucind-ai/usage.jsonl`):

### 1. Disagreement Event (`router_disagreement`)
Logged when the Candidate route differs from the Primary route:
```json
{
  "ts": "2026-10-03T11:00:00Z",
  "kind": "router_disagreement",
  "primary_route": "inline",
  "candidate_route": "worker",
  "confidence": 0.85,
  "signals": {
    "allowed_path_count": 2,
    "new_file": false,
    "risk_tier_level": 1,
    "read_only": false
  }
}
```

### 2. Error Event (`router_error`)
Logged when Candidate execution fails:
```json
{
  "ts": "2026-10-03T11:01:00Z",
  "kind": "router_error",
  "error_kind": "rate_limited"
}
```
Error kinds include: `unauthorized` (401), `invalid_request` (422), `rate_limited` (429), `overloaded` (529), `http` (other status), `transport` (network/timeout), `malformed` (bad JSON, missing answer, or unknown choice), and `panic`.

### Reporting
Router shadow events do not increment executor call counts or token/cost usage totals in `lucind-ai usage report`. If any router events exist in the log, the report appends a shadow summary line:
```text
Router shadow: 5 disagreements, 1 errors
```

---

## Open Policy Item: Data Retention Terms

As documented in Decision D15 (`docs/overnight-decisions.md`):
- The TypeSafe Jev public documentation outlines API endpoints and models, and states a commitment not to train on customer data.
- However, the specific legal Data Processing Agreement (DPA) and data-retention schedules for the TypeSafe API have not yet been formally reviewed and confirmed.
- Therefore, **enabling `LUCIND_JEV_SHADOW=on` remains an explicit decision for the repository owner**. Until the DPA is reviewed, shadow mode remains opt-in and off by default.
