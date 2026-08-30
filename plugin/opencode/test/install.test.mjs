import assert from "node:assert/strict"
import { cp, mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { execFile } from "node:child_process"
import { promisify } from "node:util"
import test from "node:test"

const execFileAsync = promisify(execFile)
const distribution = new URL("..", import.meta.url)

async function fixture() {
  const root = await mkdtemp(join(tmpdir(), "lucind-opencode-installer-"))
  await cp(new URL("../install.sh", import.meta.url), join(root, "install.sh"))
  await cp(new URL("../lucind-ai.ts", import.meta.url), join(root, "lucind-ai.ts"))
  await cp(new URL("../process.mjs", import.meta.url), join(root, "process.mjs"))
  await cp(new URL("../skills/lucind-ai", import.meta.url), join(root, "skills/lucind-ai"), { recursive: true })
  const repo = join(root, "repo", "plugin", "opencode")
  await mkdir(repo, { recursive: true })
  await cp(join(root, "install.sh"), join(repo, "install.sh"))
  await cp(join(root, "lucind-ai.ts"), join(repo, "lucind-ai.ts"))
  await cp(join(root, "process.mjs"), join(repo, "process.mjs"))
  await cp(join(root, "skills"), join(repo, "skills"), { recursive: true })
  return { root, repo, config: join(root, "config") }
}

async function install(repo, config) {
  return execFileAsync("sh", [join(repo, "install.sh")], { cwd: repo, env: { ...process.env, XDG_CONFIG_HOME: config } })
}

test("installs, reruns identically, upgrades owned files, and rejects unowned conflicts", async () => {
  const { root, repo, config } = await fixture()
  const target = join(config, "opencode")
  try {
    await install(repo, config)
    const first = await readFile(join(target, "plugins/lucind-ai.ts"), "utf8")
    await install(repo, config)
    assert.equal(await readFile(join(target, "plugins/lucind-ai.ts"), "utf8"), first)

    await writeFile(join(repo, "lucind-ai.ts"), "// owned upgrade\n")
    await writeFile(join(repo, "skills/lucind-ai/SKILL.md"), "owned skill upgrade\n")
    await install(repo, config)
    assert.equal(await readFile(join(target, "plugins/lucind-ai.ts"), "utf8"), "// owned upgrade\n")
    assert.equal(await readFile(join(target, "skills/lucind-ai/SKILL.md"), "utf8"), "owned skill upgrade\n")

    const conflict = await fixture()
    try {
      await execFileAsync("mkdir", ["-p", join(conflict.config, "opencode/plugins")])
      await writeFile(join(conflict.config, "opencode/plugins/lucind-ai.ts"), "unowned\n")
      await assert.rejects(install(conflict.repo, conflict.config), /unrelated plugin/)
    } finally { await rm(conflict.root, { recursive: true, force: true }) }
  } finally { await rm(root, { recursive: true, force: true }) }
})
