import assert from "node:assert/strict"
import { chmod, mkdtemp, rm, writeFile } from "node:fs/promises"
import { tmpdir } from "node:os"
import { join } from "node:path"
import test from "node:test"
import { runLucindAI } from "../process.mjs"

test("passes arguments as distinct argv values and uses the requested directory", async () => {
  const dir = await mkdtemp(join(tmpdir(), "lucind-opencode-"))
  const binary = join(dir, "fake-lucind-ai")
  await writeFile(binary, "#!/bin/sh\nprintf '%s|%s|%s' \"$PWD\" \"$1\" \"$2\"\n")
  await chmod(binary, 0o755)
  try {
    assert.equal(await runLucindAI(["run", "value with spaces;$(touch unsafe)"], { binary, directory: dir }), `${dir}|run|value with spaces;$(touch unsafe)`)
  } finally { await rm(dir, { recursive: true, force: true }) }
})

test("reports a missing binary and nonzero exit clearly", async () => {
  await assert.rejects(runLucindAI([], { binary: "/not/a/lucind-ai", directory: process.cwd() }), /binary was not found/)
  const dir = await mkdtemp(join(tmpdir(), "lucind-opencode-"))
  const binary = join(dir, "fake-lucind-ai")
  await writeFile(binary, "#!/bin/sh\nprintf 'bad news' >&2\nexit 7\n")
  await chmod(binary, 0o755)
  try { await assert.rejects(runLucindAI([], { binary, directory: dir }), /status 7: bad news/) }
  finally { await rm(dir, { recursive: true, force: true }) }
})

test("classifies AbortSignal cancellation with a stable diagnostic", async () => {
  const dir = await mkdtemp(join(tmpdir(), "lucind-opencode-"))
  const binary = join(dir, "fake-lucind-ai")
  await writeFile(binary, "#!/bin/sh\nsleep 10\n")
  await chmod(binary, 0o755)
  const controller = new AbortController()
  const run = runLucindAI([], { binary, directory: dir, signal: controller.signal })
  setTimeout(() => controller.abort(), 20)
  try { await assert.rejects(run, /lucind-ai execution cancelled/) }
  finally { await rm(dir, { recursive: true, force: true }) }
})
