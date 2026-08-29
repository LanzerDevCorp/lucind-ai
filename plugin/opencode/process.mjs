import { spawn } from "node:child_process"

/** Run lucind-ai without involving a shell: every user value remains one argv item. */
export function runLucindAI(args, options) {
  const binary = options.binary ?? "lucind-ai"
  return new Promise((resolve, reject) => {
    const child = spawn(binary, args, { cwd: options.directory, shell: false, signal: options.signal, stdio: ["ignore", "pipe", "pipe"] })
    let stdout = ""
    let stderr = ""
    child.stdout.on("data", (chunk) => { stdout += chunk.toString() })
    child.stderr.on("data", (chunk) => { stderr += chunk.toString() })
    child.on("error", (error) => {
      if (options.signal?.aborted || error.name === "AbortError") {
        reject(new Error("lucind-ai execution cancelled"))
        return
      }
      if (error.code === "ENOENT") {
        reject(new Error("lucind-ai binary was not found on PATH; run `make install` and ensure GOPATH/bin is on PATH"))
        return
      }
      reject(new Error(`failed to start lucind-ai: ${error.message}`))
    })
    child.on("close", (code, signal) => {
      if (signal) return reject(new Error(`lucind-ai was cancelled by ${signal}`))
      if (code !== 0) return reject(new Error(`lucind-ai exited with status ${code}: ${stderr.trim() || stdout.trim() || "no diagnostic output"}`))
      resolve(stdout.trim())
    })
  })
}
