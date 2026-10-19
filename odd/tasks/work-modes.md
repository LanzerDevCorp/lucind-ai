# work-modes
Objective: lucind-ai becomes an extension of gentle-ai; a central config of "work modes" maps roles to executor+model. Branch: feature/work-modes. Delivery: ask-on-risk. Runner: `go test ./... -race -count=1`.

## Specs
- S1. lucind-ai no longer replaces gentle-ai: gentle-ai owns "cuándo lanzar un subagente o cómo trabajar" (delegation triggers, `Risk`, verifier). lucind-ai is only an alternative way to run subagents in herdr panes, plus attestations.
- S2. "un lugar centralizado para definir que modelo usar y cuando" — own file (user decision: "Archivo propio"), under the existing user config dir `$XDG_CONFIG_HOME/lucind` or `~/.config/lucind` (`userconfig.Dir`). Format JSON `modes.json` (assumption: no new dependency; go.mod has no TOML lib).
- S3. "modos de trabajo": a mode maps each role (`explorer`, `writer`, `verifier`, same names as gentle-ai) to `{executor, model}`. Executors: `native` (Claude Code subagent, orchestrator-run), `lane` (agy). `cursor` is reserved, not implemented (user: "aun me falta meter a cursor").
- S4. "el comando persistente y override por proyecto": `lucind-ai mode use <name>` persists the active mode in the config file; `.lucind/mode` in a project overrides it; `lucind-ai mode show|list` inspect.
- S5. `lucind-ai route <role>` prints the resolved executor and model for the active mode so the orchestrator obeys it (native -> Agent with that model; lane -> dispatch). Exit non-zero with a clear message for an unknown role/mode.
- S6. `dispatch --role <role>` resolves the model from the active mode when `--model` is absent; precedence `--model` > `--role` mode > `LUCIND_AGY_MODEL` > default. A role whose executor is not `lane` is rejected.
- S7. "el agente verificador puede seguir existiendo, incluso el podria ejecutar el attestation de ser necesario": verifier role may be native or lane; attestations stay available in every mode.
- S8. The dispatch block (`plugin/claude-code/claudemd/lucind-dispatch.md`) shrinks to: consult `lucind-ai route <role>`, obey it; no redefinition of gentle-ai verification rules; headings aligned with gentle-ai 4.2.0 ("Delegation Triggers", `Risk:` item N named -> one independent verifier).
- S9. "no necesariamente vamos a forzar el uso de lucind-ai": every role may be `native` in a mode.

## Tasks
- T1 (S2,S3,S4,S5) `internal/modes`: parse/validate/resolve, project override, persist active. Route: lane. Status: done (lane 20261019-192242-2124, accepted, 83.9% coverage).
- T2 (S4,S5) CLI `mode use|show|list` and `route <role>` in cmd/lucind-ai. Route: lane. Status: pending.
- T3 (S6) `dispatch --role`. Route: lane. Status: pending.
- T4 (S1,S7,S8,S9) dispatch block, SKILL.md, docs/product.md, retire "replace gentle-ai" narrative. Route: lane. Status: pending.

## Log
- L1 (2026-10-19) Mira, hay que replantarnos como funciona lucind-ai, anteriormente queria reemplazar a gentle-ai con lucind-ai, pero ya no mas, quiero que lucind-ai sea una extension de gentle-ai que use el orchestrator para despachar lanes mediante lucind-ai, tambien deberiamos de tener un lugar centralizado para definir que modelo usar y cuando (asi como gentle-ai) y lucind-ai solo deberia de ser una forma alternativa de despachar subagentes con panes de herdr, tipo, cuando quiera hacer fanout con agy o implementar con el. las fortalecas de lucind-ai son los attestations, pero el contrato sobre cuando lanzar un subagente o como trabajar lo deberia de tener gentle-ai, si me entiendes?
- L2 Archivo propio, y sobre el verificador, no se reemplaza por --check, el agente verificador puede seguir existiendo, incluso el podria ejecutar el attestation de ser necesario
- L3 el verificador ... puede ser un haiku 5.5 nativo de claude, y para el implementador, tampoco seria forzosamente una lane de agy, podria ser un sonnet 5.5 nativo tambien ... vamos mas como por "modos de trabajo" ... y aun me falta meter a cursor jaja
- L4 va, con el comando persistente y override por proyecto
- Evidence: HERDR_ENV=1; no cursor executor in code today; no TOML dependency in go.mod.
