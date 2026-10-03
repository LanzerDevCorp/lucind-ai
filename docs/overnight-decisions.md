# Overnight decisions (herdr-agent-factory)

Autonomous session started 2026-10-03. Decision format: `D<n> | date-time | task | context | options | chosen | why | how to revert | commits`.

## Decisions

D-ROT-0 | 2026-10-03 | preflight | `agy-pool list`: 3 saved profiles (lanzerdev20, ponenofeik5, corp.systems.lanzer), each containing only `google_accounts.json` and `oauth_creds.json`; none has `antigravity-oauth-token`, so `agy-pool use` would not change the account `agy` uses | enable rotation / disable | ACCOUNT ROTATION DISABLED all night | rule R0.3 of the mission | n/a (rotation is simply not used) | none

D1 | 2026-10-03 | T1b | attestations were keyed by sha256(toplevel path), invisible from the primary root when made in a lane worktree | key by toplevel (keep) / key by git common dir / store the entry in the lane and copy | key by git common dir | all worktrees of one repo share a namespace; the tree hash (not the path) binds an attestation to content; smallest change | revert `RepoID` input in `internal/attest` and callers in `cmd/lucind-ai/attest.go` | 6b82c49, aa8a693

D2 | 2026-10-03 | T1b | one unexplained `FAIL` count (3) from `go test ./...` on the feature branch right after `make install`; 3 reruns clean | investigate with `-race -count=20` / record | recorded | identified later: `TestLeaseAcquisitionAndMonotonicFence` (internal/feature/feature_test.go:297) is timing-flaky when the whole suite runs in parallel; passes 3/3 in isolation; unrelated to attest/packet changes. FIXED later the same night: the tests used 200-250 ms lease TTLs that expire under load (load average 5), widened to 1.5 s TTL / 1.6 s sleeps; 6 consecutive -race runs green. No code under internal/feature changed | n/a | n/a

D3 | 2026-10-03 | T3 | legacy lanes carrying only `sdd_phase` (explore/spec/...) and no `lane_role`/`read_only` used to skip mechanical checks; the new predicate runs them | keep skipping on legacy SDD phases / fail closed | fail closed (checks run unless the lane is declared read-only or has a non-writing role) | the mission asks for a fail-closed role/read_only predicate; legacy SDD packets are being retired (T4/T5) | revert `RequiresMechanicalChecks` to the SDDPhase condition in accept.go and attempt.go | d5d07f2

D4 | 2026-10-03 | T4 | "make sdd-* derivation optional": lane roles apply/verify/archive always added sdd-apply/verify/archive | keep as-is / opt-in via explicit sdd_phase / new flag | opt-in via explicit sdd_phase (reuses the existing field, no new API) | consistent with how lens/synthesis already gate sdd-<phase>; ODD packets get no sdd-* skills | revert the three `if sddPhase != ""` guards in internal/skillset/skillset.go; NOTE required_skills (hence packet digest) of role-only apply/verify/archive packets change, so a replay of an old such packet gets a new digest | 85b4a24

D5 | 2026-10-03 | T8 | blind review item: the exit sentinel nonce is readable from run.sh by the agent (it could print a forged sentinel) | delete run.sh before running agy / accept | accepted | agy already runs with --dangerously-skip-permissions and can write any file, so exit code and output are not a security boundary; trust comes from the dispatcher (attestation, allowed_paths diff, judges). A forged sentinel without a matching exit.code fails closed (read error) | n/a | f721ce8
D6 | 2026-10-03 | T8 | review items: no hard-kill after C-c grace; state dirs of failed runs are never reaped | fix now / new task | new task T16 | closing panes/workspaces needs a policy for reused panes (rule: never close panes you did not create) that deserves its own task | n/a | f721ce8

D7 | 2026-10-03 | T9 | dispatcher commit and repository hooks | run hooks (repo policy) / --no-verify | --no-verify | attested verification already gates quality; hooks could be redirected by the worker (core.hooksPath) or add trailers, and would run with dispatcher authority; two blind reviewers recommended it | remove --no-verify in commit_step.go (defaultGitCommit) | bed8d65
D8 | 2026-10-03 | T9 | packet field design: commit_message requires verification; commit obligation value dispatcher; envelope.Commit must be empty | worker may also commit / dispatcher-only | dispatcher-only for packets that declare commit_message | one clear owner of the commit; legacy packets unchanged | drop CommitMessage handling in run.Execute and the dispatcher branch in accept | bed8d65

D9 | 2026-10-03 | T11 | what to do when a declared route contradicts computed signals | reject all mismatches / upgrade inline to worker / warn only | upgrade inline->worker (printed), reject only missing evidence or a malformed fanout; over-delegation (worker below threshold) accepted | a packet that reaches the dispatcher is delegated anyway, so upgrading is consistent and never unsafe; rejection is reserved for missing/incoherent declarations | remove validateDispatchThresholds from runDispatch in cmd/lucind-ai/cli.go | ebcd33c

D10 | 2026-10-03 | T12a | default lane concurrency: before this change ExecuteBatch started every lane at once | keep unlimited by default / default 3 / default 1 | default 3 with --max-parallel | the mission asks for a cap of 3 workers; behavior change for batches of more than 3 lanes (they now queue) | pass --max-parallel with a large value, or set DefaultMaxParallelLanes | be120f5

D11 | 2026-10-03 | T12b | mission requires two blind reviewers from different families; the Claude-family reviewer hit the quota wall (resets 14:06Z) | wait ~3h idle / integrate with one reviewer and finish the second later / skip | integrated T12b after applying every reproduced finding of reviewer A; second review kept as pending work, then DONE at 14:2xZ after the quota reset: reviewer B found no high-severity issue (see T12b entry); range 7f12d0e..f919168) | idle waiting would waste the Gemini window; the feature branch is not pushed, so a late finding costs one follow-up commit | revert the T12b commits on the feature branch | f919168
D12 | 2026-10-03 | T12b | loop semantics: loops require commit_message (dispatcher commits), retry only on verification failure, total cap 4, ladder exhaustion blocks, no-ladder exhaustion fails | allow loops without dispatcher commit | require commit_message | worker commits conflict with the shared-worktree loop (HEAD must stay at base); a single owner of the commit | relax ErrLoopNeedsCommitMessage and dispatcherVerify's HEAD check | f919168

D13 | 2026-10-03 | queue order | Gemini 5h quota at 28% (resets 13:55Z) with T12c, T13, T14, T15 left; Claude/GPT bucket at 0% (resets 14:06Z) | follow the mission order T12c,T13,T14,T15 / do the required list first | T13, T15, T14 first, T12c (explorer fan-out, the most optional slice) last | if quota runs out the items that complete the owner's list are already done; every task keeps its own dependencies (T12c depends on T12a and T6, both done) | none (order only) | n/a

D14 | 2026-10-03 | T15 | which sections go to which generated file; what to do with hand-written files | AGENTS.md = everything / worker+all; overwrite hand-written with --force / never | CLAUDE.md = orchestrator+all, GEMINI.md and AGENTS.md = worker+all; hand-written or symlinked files are never overwritten (no --force); this repo keeps its hand-written CLAUDE.md and no lucind-rules.md was added to it | AGENTS.md is read by implementer agents (opencode, cursor-agent lanes); overwriting a user's file is not reversible | change the audience mapping in internal/rules Render | 218053c

D15 | 2026-10-03 | T14 | Jev (TypeSafe) API read from the public docs (docs.typesafe.ai/api.md and legal.md, read-only fetch, no data sent): POST https://api.typesafe.ai/v1/systemone, Bearer key, body {state, model:"jev-latest", questions:{key:{type: noul|choice|score, instructions, criteria}}}, response {model, answers, usage}, errors 401/422/429/529. legal.md is only an index (Data Processing Agreement, Privacy Policy, Master Customer Agreement); it states a commitment not to train on user data and offers zero data retention for enterprise customers, but the exact retention periods were NOT readable | send richer context / only numbers and booleans | only numbers and booleans, opt-in via two env vars, shadow mode, no real API call tonight | retention terms unverified (open item for the owner: read the Data Processing Agreement before enabling) | remove the shadow wiring in cmd/lucind-ai/cli.go | n/a

D16 | 2026-10-03 | verification | repeated full `go test ./... -race` runs: (a) `TestHerdrAgyTimeoutSendsCtrlCAndReturnsTimedOut` (mine, T8) was timing-flaky because the fake `pane run` was tied to a 20 ms context: FIXED (the fake no longer uses the caller context; 6 consecutive -race runs green); (b) two internal/ledger concurrency tests hit SQLITE_BUSY once under full-suite load: NOT changed, tracked as T20 | ignore / fix now / track | fixed (a), tracked (b) | (b) is pre-existing, passes in isolation, and the ledger code is untouched by this work | n/a | n/a

D17 | 2026-10-03 | T12c | `executor.Agy.KnownModels()` only allowed `gemini-3.7-flash-high`, so `lucind-ai run` would have rejected every model of the factory design (found while wiring explore; the nightly manual dispatches bypassed `run`) | keep the list and add per-command wrappers / extend the real list | extended `Agy.KnownModels()` to the design set (3.7-flash-high, 3.8-flash-high, 3.8-flash-medium, 3.1-pro-high, claude-opus-4-6-thinking); `herdr-agy` shares it; default model unchanged (3.7-flash-high) | the doc comment says extending the set is a deliberate code change for models verified against the real CLI: all five were used successfully tonight; wrappers would hide the problem and recorded a smaller list | restore the single-model list in internal/executor/agy.go | 52c9549

Note for tomorrow: the T1 worktree `~/git_root/lucind-ai-worktrees/lane-t1-hmac-attestation` and branch `lane/t1-hmac-attestation` are kept (deletion is forbidden overnight).

## Registro de rotaciones

- 2026-10-03 | lanzerdev20@gmail.com | preflight | `list`/`current` read-only; active account lanzerdev20, usage cache 98%; no profile has `antigravity-oauth-token` -> rotation disabled (D-ROT-0)
- 2026-10-03 04:02 | lanzerdev20@gmail.com | quota | blind reviewer B for T12b (`claude-opus-4-6-thinking`) failed with 429 RESOURCE_EXHAUSTED ("Resets in 3h4m"); `agy --print /usage`: Claude and GPT models 5h = 0% (resets 14:06Z), weekly 48%; Gemini models 5h = 28% (resets 13:55Z), weekly 58%. Rotation is disabled (D-ROT-0), so no account switch. Plan (R4 adapted): keep working with Gemini models, spend Gemini quota only on writers (no pro reviewers), and run the pending review B after 14:06Z.

## Reporte matutino

Cierre de la sesión sin supervisión, 2026-10-03 (hora local ~07:30). Rama `feature/herdr-agent-factory`; nada empujado, ningún PR, RDD apagado, ningún `gentle-ai review ...`, rotación de cuentas desactivada (D-ROT-0), un solo `agy` a la vez.

### Tareas

| Tarea | Estado | Commits en la rama de feature |
|---|---|---|
| T1b atestación (clave atómica, 0444, accept la reutiliza, RepoID) | hecha | 6b82c49, aa8a693 |
| T2 campos del packet + `interaction_required` | hecha | 73179d2 |
| T3 predicado fail-closed de checks mecánicos | hecha | d5d07f2 |
| T4 SDD fuera (docs/plantillas, `sdd-*` opt-in) | hecha | 85b4a24 |
| T5 comando `phase` y `internal/phasespec` fuera | hecha | 10ef695 |
| T6 skills worker y explorador | hecha | 3fe1e8c |
| T7 spike herdr/agy | hecha | `docs/herdr-spike-findings.md` |
| T8 executor `herdr-agy` | hecha (revisión ciega x2) | cc6df27, f721ce8 |
| T9 commit del dispatcher | hecha (revisión ciega x2) | 82cae7d, bed8d65 |
| T10 clasificador de riesgo y plan por tier (sin cursor-agent) | hecha | 5563143 |
| T11 validador del umbral de despacho | hecha | ebcd33c |
| T12a tope de paralelismo (3) | hecha | be120f5 |
| T12b loop write/test/fix + escalera | hecha (revisión ciega x2) | ac33b61, f919168 |
| T12c fan-out de exploradores (`lucind-ai explore`) | hecha | 52c9549, 7aa93ac |
| T13 log de uso y `usage report` | hecha | 47bd901 |
| T14 Router + Jev en shadow (sin llamadas reales) | hecha | 2cd9e78 |
| T15 fuente única de reglas (`rules init/generate`) | hecha | 218053c |
| T16-T20 seguimientos descubiertos por las revisiones y los e2e | creados, sin empezar | n/a |

Saltadas o bloqueadas: ninguna. Fuera de alcance esta noche: `cursor-agent` (T18).

### Estado de verificación

- `go build ./...` y `go vet ./...` limpios sobre la rama de feature; `make verify-plugin-content verify-opencode-plugin` pasan (plugin 2.0.20).
- `go test ./...` pasa. Dos corridas completas finales con `-race`: una limpia y otra con la flake ya conocida de `internal/ledger` (`TestConcurrentProgressAndSetStatus`, SQLITE_BUSY bajo carga, T20). Dos flakes propias de esta noche se corrigieron (leases de `internal/feature`, fake de `pane run` de T8, D2 y D16).
- Cada tarea se verificó con build, vet, tests y una prueba real de comportamiento en repos temporales; los e2e encontraron bugs que los unit tests no veían (p. ej. `routed_by: explore` derivaba `sdd-explore`, T12c; tests de `cmd` escribiendo en el estado real, T13).

### Decisiones que esperan tu revisión (las más riesgosas primero)

1. D7: el commit del dispatcher usa `--no-verify` (no corre hooks del repo).
2. D3: los checks mecánicos corren salvo lane read-only o rol no escritor; lanes legacy con solo `sdd_phase` ahora ejecutan checks.
3. D4: `sdd-*` es opt-in; cambia el digest de packets con rol `apply`/`verify`/`archive` sin `sdd_phase`.
4. D12: los loops exigen `verification` y `commit_message`.
5. D17: `Agy.KnownModels()` ampliado a los 5 modelos del diseño (antes `lucind-ai run` solo aceptaba `gemini-3.7-flash-high`).
6. D15: Jev. Se leyó la documentación pública (solo lectura, sin enviar datos); los términos de retención exactos NO se pudieron leer. El router está apagado por defecto y solo envía 4 números/booleanos; leer el Data Processing Agreement antes de habilitarlo.
7. D10: `ExecuteBatch` limita a 3 lanes concurrentes por defecto (`--max-parallel`).
8. D1, D5, D6, D8, D9, D11, D13, D14, D16: ver arriba.

`BLOCKED-DECISION`: ninguna.

### Cuota y rotación

Rotación DESACTIVADA toda la noche (D-ROT-0): ningún perfil guardado tiene `antigravity-oauth-token`. Hubo un 429 (revisor Claude de T12b, ~11:00Z); la ventana de Gemini quedó al 6 % y se esperó al reinicio (13:55Z) con trabajo local mientras tanto; la segunda revisión se hizo tras el reinicio de la cuota Claude/GPT (14:06Z). Al cierre: Gemini 5 h ~90 %, Claude/GPT 5 h ~100 % (semanales 52 % / 48 %).

### No verificado

- Los términos de retención de datos de Jev (D15).
- Que `lucind-ai explore` y `herdr-agy` funcionen con el `agy` y `herdr` reales: solo se probaron con fakes y un `agy` falso en repos temporales (el `herdr` real sí se usó en el e2e de T8 con un `agy` falso).
- Dos tests de `internal/ledger` siguen fallando de forma intermitente bajo carga (T20).

### Siguiente tarea recomendada

T17 (accept re-verifica los candidatos con commit del dispatcher), luego T16 (parada dura y limpieza de `herdr-agy`), T19 (señales declaradas del router), T20 (flake del ledger) y T18 (jueces con `cursor-agent`, necesita Cursor). Antes de usar `lucind-ai explore` o `herdr-agy` en serio, una corrida real con `agy` en un repo descartable.

### Lo que dejé en disco

Worktrees `~/git_root/lucind-ai-worktrees/lane-*` y ramas `lane/*` se conservan todos (borrar está prohibido sin supervisión): hay más de 20; los commits ya están integrados en la rama de feature por cherry-pick, así que `git branch --no-merged` los mostrará hasta que decidas borrarlos. Estado de `herdr`: los workspaces temporales de los spikes se cerraron. El binario instalado en `$GOPATH/bin` corresponde a la rama de feature (`lucind-ai -v`).
