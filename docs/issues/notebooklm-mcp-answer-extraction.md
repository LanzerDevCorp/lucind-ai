# notebooklm-mcp — `ask_question` returns the loading spinner instead of the answer

**Upstream:** `PleasePrompto/notebooklm-mcp` (npm `notebooklm-mcp`)
**Found:** 2026-08-17, while running PRD-derived research for `lucind-ai`
**Status:** not filed upstream yet. Not a `lucind-ai` defect — kept here because it blocks this repo's
research workflow.

## Summary

`ask_question` returns `success: true` with Google's rotating **loading message** in the `answer`
field instead of the generated answer. The answer is produced correctly and persists in the
notebook's chat history — the defect is purely in reading it back.

## Observed

Four calls, four different placeholders — all of them Google's loading strings, none of them content:

```
"Descubriendo la idea principal…"
"Leyendo tus fuentes…"
"Abriendo tus notas…"
"Escaneando el texto…"
```

Not a timing or complexity artifact: a trivial one-line question in a fresh session failed
identically after ~15s, and waiting 150s between calls does not help — each new call submits a new
question and reads that question's own fresh spinner.

## Root cause

The answer extraction reads the response container **immediately after submitting the question**,
without waiting for generation to finish.

## The readiness signal — already available

While generating, the query textarea is `disabled` with `placeholder="Respondiendo…"`. Captured from
a `page.fill` failure log on a call issued during generation:

```
locator resolved to <textarea rows="1" disabled matinput="" autocomplete="off"
  cdkautosizeminrows="1" placeholder="Respondiendo…" aria-label="Cuadro de consulta"
  class="cdk-textarea-autosize mat-mdc-autocomplete-trigger query-box-input ...">
```

Selector: `textarea.query-box-input`. Polling until it is enabled again is a sufficient and clean
completion condition.

**The package already hits this signal and misclassifies it.** A call made while a previous
generation is running fails with `page.fill: Timeout 30000ms exceeded` because the textarea is
disabled — so the wait condition is being treated as an error rather than as backpressure.

## Secondary findings

- `browser_options.timeout_ms` is **not** applied to `page.fill`, which uses the hardcoded 30s default.
- Related defect already known in the same package: the login-detection loop in
  `dist/auth/auth-manager.js` (`performLogin`) only matches
  `currentUrl.startsWith("https://notebooklm.google.com/")`, but Google's real OAuth continue URL
  redirects to a different host.

## Fork and patch checklist

Currently invoked with no local checkout — `~/.claude.json` has:

```json
"notebooklm": { "type": "stdio", "command": "npx", "args": ["notebooklm-mcp@latest"] }
```

1. Fork `PleasePrompto/notebooklm-mcp`, clone, build.
2. In the answer-extraction path: after submitting, wait for `textarea.query-box-input` to lose
   `disabled` (or for `placeholder` to stop being `Respondiendo…`) **before** reading the response
   container. Add a generous ceiling — complex questions take minutes.
3. Treat a disabled textarea on entry as *wait for the in-flight generation*, not as a fill error.
4. Honor `browser_options.timeout_ms` for `page.fill`.
5. Fix the login-host match while in there.
6. Repoint `~/.claude.json` at the local build; restart the MCP server.
7. File both defects upstream with this evidence.
