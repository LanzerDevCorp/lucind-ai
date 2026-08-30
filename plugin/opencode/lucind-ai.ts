import { type Plugin, tool } from "@opencode-ai/plugin"
import { runLucindAI } from "./process.mjs"

const LucindAIPlugin: Plugin = async () => ({
  tool: {
    lucind_ai: tool({
      description: "Invoke the installed lucind-ai binary in the active OpenCode directory.",
      args: {
        args: tool.schema.array(tool.schema.string()).describe("Arguments passed to lucind-ai, each as a separate argv item."),
      },
      async execute(input, context) {
        return runLucindAI(input.args, { directory: context.directory, signal: context.abort })
      },
    }),
  },
})

export default LucindAIPlugin
