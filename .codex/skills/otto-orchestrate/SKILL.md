---
name: otto-orchestrate
description: Use when triggering subagents
---

# Using Otto

Otto spawns and monitors AI agents. Use it when you need parallel workers or want to delegate tasks.

## Spawn an Agent

```bash
otto spawn codex "your task description" --detach
# Returns: agent-id
```

Options:
- `--name <name>` - Custom ID (e.g., `--name reviewer`)
- `--files <paths>` - Attach relevant files
- `--context <text>` - Extra context

## Check Status

```bash
otto status
```

Shows all agents: `busy`, `complete`, `failed`, or `waiting`.

## Read Output

```bash
otto peek <agent-id>    # New output since last peek (advances cursor)
otto log <agent-id>     # Full history
otto log <agent-id> --tail 20   # Last 20 entries
```

Use `peek` for polling. Use `log` to review history.

## Send Follow-up

```bash
otto prompt <agent-id> "your message"
```

Use when an agent finishes and you need more work, or to answer a `waiting` agent.

## Typical Flow

```bash
# 1. Spawn
otto spawn codex "implement feature X" --name feature-x --detach

# 2. Poll until done
otto status                    # Check if still busy
otto peek feature-x            # Read new output

# 3. Follow up if needed
otto prompt feature-x "also add tests"
```

## Tips

- Use `--name` for readable IDs instead of auto-generated slugs
- Check `otto status` before `peek` to avoid polling completed agents
- Agents see each other's messages - use `@agent-id` to mention specific agents
