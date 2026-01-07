# June Config for MCP Servers - Design Doc

## Problem

Users want Gemini subagents to have access to MCP servers (like Chrome browser automation), but there's no way to configure this in June.

## Goal

Add a YAML config file where users define MCP servers. When spawning Gemini, June creates an isolated environment with those MCP servers configured.

## Non-Goals

- Other settings beyond MCP servers (YAGNI)
- Per-spawn MCP server filtering (`--mcp` flag)
- Reading MCP servers from Claude plugins automatically
- `june config` command for setup assistance

## Design

### Config File

Location: `~/.june/config.yaml`

```yaml
mcpServers:
  # Chrome DevTools Protocol - browser automation
  chrome:
    command: node
    args:
      - /path/to/chrome-mcp/index.js

  # Playwright - accessibility tree based
  playwright:
    command: npx
    args:
      - "@anthropic/mcp-playwright"
```

The structure mirrors Gemini CLI's native `settings.json` format for MCP servers.

### Isolated Gemini Environment

Same pattern as Codex - isolated home directory with copied auth and custom config.

**Directory structure:**

```
~/.june/
├── config.yaml              # User's MCP server config (new)
├── june.db                  # Agent database (existing)
├── codex/                   # Isolated Codex home (existing)
│   └── auth.json
└── gemini/                  # Isolated Gemini home (existing, enhance)
    ├── settings.json        # Generated from config.yaml (new)
    ├── oauth_creds.json     # Copied from ~/.gemini/
    ├── google_accounts.json # Copied from ~/.gemini/
    └── sessions/            # Session files (existing)
```

**Auth files to copy:**

| File | Purpose |
|------|---------|
| `oauth_creds.json` | OAuth tokens from browser login |
| `google_accounts.json` | Active account info |

Both copied with `0600` permissions, only if source exists and destination doesn't (atomic create, same pattern as Codex).

### Spawn Flow

1. Load `~/.june/config.yaml` (if exists)
2. Copy auth files from `~/.gemini/` (if not already present, warn if missing)
3. If config has MCP servers:
   - Write `~/.june/gemini/settings.json` with MCP servers (permissions `0600`)
   - Set `GEMINI_CONFIG_DIR=~/.june/gemini/` env var
4. Run `gemini -p "task" ...`

**Important:** If no config file exists or it has no MCP servers, Gemini uses its normal `~/.gemini/` config. This preserves any MCP servers the user configured directly in Gemini.

### Components

1. **Config loading** - New capability in `internal/config/` to read YAML config
2. **Gemini home setup** - Enhance `EnsureGeminiHome()` to copy auth and write settings
3. **Spawn integration** - Set `GEMINI_CONFIG_DIR` env var when launching

### Behavior

| Scenario | Behavior |
|----------|----------|
| No config file | Gemini spawns normally, no MCP servers |
| Config exists | MCP servers available to Gemini |
| Invalid YAML | Spawn fails with clear error |
| Auth files missing in `~/.gemini/` | Warning only (user may not have logged in) |

## Future Considerations

- `june config init` command to create starter config with examples
- `--mcp <name>` flag to filter which servers a specific spawn gets
- Reading MCP servers from Claude plugins automatically
- Codex MCP server support (if Codex adds MCP support)
