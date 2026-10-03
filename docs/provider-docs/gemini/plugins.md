# Plugins

Plugins package reusable skills, background subagents, linting rules, Model Context Protocol (MCP) servers, and lifecycle hooks into a single deployable asset.

## Directory structure

A plugin is a directory containing a required manifest file (`plugin.json`) and optional subdirectories for different customization types:

```
plugins/<plugin-name>/
├── plugin.json       # Required marker and manifest file
├── mcp_config.json   # Optional MCP server definitions
├── hooks.json        # Optional hooks definition
├── skills/           # Optional skills directory
│   └── <skill-name>/
│       └── SKILL.md
├── agents/           # Optional subagent definition templates
│   └── <agent-name>.md
└── rules/            # Optional rules directory
    └── <rule-name>.md
```

### Manifest file (`plugin.json`)

Every plugin requires a `plugin.json` file at its root to identify the directory as a plugin and define its metadata:

```
{
  "$schema": "https://antigravity.google/schemas/v1/plugin.json",
  "name": "my-custom-plugin",
  "description": "A brief description of what my plugin does."
}
```

#### Field reference

| Field | Type | Required | Description |
| :-- | :-- | :-- | :-- |
| `name` | String | **Yes** (CLI) / Optional (2.0 and IDE) | The unique, machine-readable name of the plugin (matches `^[a-zA-Z0-9-_]+$`). Required when managing plugins using Antigravity CLI commands; defaults to the folder name if omitted in Antigravity 2.0 or Antigravity IDE. |
| `description` | String | No | A brief human-readable description of the plugin’s purpose, displayed in plugin listings. |

#### Automatic validation

To enable automatic autocomplete and validation in editors like VS Code or JetBrains IDEs, include the `$schema` key pointing to the official schema URL:

```
"$schema": "https://antigravity.google/schemas/v1/plugin.json"
```

#### Full JSON schema

```
{
  "$schema": "https://antigravity.google/schemas/v1/plugin.json",
  "title": "Antigravity Plugin Manifest",
  "description": "Schema for Antigravity plugin manifest files (plugin.json)",
  "type": "object",
  "properties": {
    "name": {
      "type": "string",
      "description": "The unique, machine-readable name of the plugin. Must contain only alphanumeric characters, hyphens, and underscores.",
      "pattern": "^[a-zA-Z0-9-_]+$"
    },
    "description": {
      "type": "string",
      "description": "A brief human-readable description of the plugin's purpose and capabilities."
    }
  },
  "required": ["name"],
  "additionalProperties": false
}
```

### Supported components

A plugin can contain any of the following components:

*   `skills/`: Subdirectories containing a `SKILL.md` file with instructions for the agent.
*   `agents/`: Markdown files defining custom subagents and persona configurations.
*   `rules/`: Markdown files defining behavioral constraints or style guidelines.
*   `mcp_config.json`: Declarations connecting Antigravity to external tool servers.
*   `hooks.json`: Event handlers executing shell commands before or after tool calls.

## Managing plugins by surface

Follow the instructions below to install and manage plugins on your preferred surface:

*   [Antigravity 2.0](#tab-panel-21)
*   [Antigravity CLI](#tab-panel-22)
*   [Antigravity IDE](#tab-panel-23)

### Marketplace and bundled plugins

Antigravity 2.0 provides curated plugins that you can browse and install directly from the application interface:

1.  Open the **Customizations** tab, located below the **Scheduled Tasks** tab in the left sidebar.
2.  Switch between the **Marketplace** view (which displays all plugins available to you) and the **Installed** tab (which displays only your installed plugins).
3.  In the **Marketplace** tab, click the **+** button to install a plugin, or click a plugin’s name to view what is included in the bundle (such as [skills](/docs/skills), [MCP servers](/docs/mcp), [rules](/docs/rules), and [agents](/docs/subagents)).

For a complete walkthrough of plugin discovery and cross-surface synchronization, refer to the [Marketplace guide](/docs/marketplace).

Note

**Cross-surface synchronization**: Plugins installed in Antigravity 2.0 are automatically updated and shown in the Antigravity CLI’s **Installed** tab.

### Manual plugin installation

You can install custom plugins by placing their directories in either of the following locations:

*   **Workspace level**: Place your plugin folder in `.agents/plugins/` at the root of your workspace. The plugin activates only when working in that project.
*   **Global level**: Place your plugin folder in `~/.gemini/config/plugins/`. The plugin activates across all workspaces on your workstation.

### Interactive plugins manager (`/plugin`)

In an interactive TUI session, run `/plugin` (or its alias `/plugins`) to open the **Plugins Manager**, where you can browse the marketplace in the **Discover** tab, install plugins from a local directory, or enable, disable, and uninstall plugins in the **Installed** tab.

You can also manage and install plugins directly from the prompt using inline subcommands (`install`, `uninstall`, `enable`, `disable`, and `list`), including installing a custom plugin from a local path:

```
/plugin install <local-path>
/plugin install <plugin-name>@<marketplace-name>
/plugin list
```

For the full interactive walkthrough of the **Discover** and **Installed** tabs, keyboard shortcuts, and inline marketplace commands, refer to the [Marketplace guide](/docs/marketplace?tab=cli).

Note

**Cross-surface synchronization**: Plugins installed in Antigravity 2.0 are automatically updated and shown in the CLI’s **Installed** tab.

### CLI shell subcommands (`agy plugin`)

Outside an interactive TUI session, the Antigravity CLI also exposes the `agy plugin` subcommand pipeline to manage extensions from your shell:

*   **List installed plugins**: List all active packages and their loaded components:
    
    ```
    agy plugin list
    ```
    
*   **Install a plugin**: Stage a local package directory into your profile:
    
    ```
    agy plugin install </path/to/local/plugin>
    ```
    
*   **Enable or disable a plugin**: Toggle a plugin without removing its files:
    
    ```
    agy plugin disable <plugin_name>
    agy plugin enable <plugin_name>
    ```
    
*   **Uninstall a plugin**: Remove the plugin files and clean up configuration registries:
    
    ```
    agy plugin uninstall <plugin_name>
    ```
    

### CLI filesystem location

When installed, the CLI stages plugin assets in your global configuration directory:

```
~/.gemini/antigravity-cli/plugins/<plugin_name>/
```

### Next steps

Explore related documentation and guides:

*   [Marketplace](/docs/marketplace?tab=cli)
*   [Migration from Gemini CLI](/docs/cli/gcli-migration)
*   [Troubleshooting](/docs/cli/troubleshooting)
*   [Permissions and sandbox](/docs/sandbox?tab=cli)

### Standalone IDE plugin installation

In the standalone Antigravity IDE, plugins can be loaded locally or globally:

*   **Workspace level**: Save plugin folders to `.agents/plugins/` in your project root.
*   **Global level**: Save plugin folders to `~/.gemini/config/plugins/` to activate them across all IDE windows.
*   **Editor settings**: Access the **Customizations** dropdown from the agent side panel to review active plugin components and inspect loaded skills.