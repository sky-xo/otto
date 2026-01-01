---
name: june-orchestrate
description: Use when spawning Codex agents via June to delegate implementation work.
---

# June Orchestration

Spawn and coordinate Codex agents for implementation work.

## Orchestrator Role

You coordinate, you don't implement. Dispatch agents to do the work.

## CLI Commands

### Spawning Agents

```bash
june spawn codex "task description" --name <name> --detach
```

Options:
- `--name <name>` - Readable ID (e.g., `planner`, `impl-1`)
- `--detach` - Return immediately, agent runs in background
- `--files <paths>` - Attach relevant files (keep minimal)
- `--context <text>` - Extra context

### Monitoring

```bash
june status                  # List all agents and their status
june peek <agent>            # New output since last peek (advances cursor)
                             # For completed/failed agents, shows full log (capped at 100 lines)
june log <agent>             # Full history
june log <agent> --tail 20   # Last 20 entries
june messages                # Shared message channel
```

### Communication

```bash
june prompt <agent> "message"                           # Send followup to agent
june dm --from <sender> --to <recipient> "message"      # Send direct message between agents
                                                        # Supports cross-branch: --to feature/login:frontend
```

### Lifecycle

```bash
june kill <agent>            # Terminate agent process
june interrupt <agent>       # Pause agent (can resume later)
june archive <agent>         # Archive completed/failed agent
```

## Agent Communication

Spawned agents use these to communicate back:
- `june dm --from <self> --to <recipient> "message"` - Send direct message to another agent or orchestrator
- `june ask "question?"` - Set status to WAITING, block for answer
- `june complete` - Signal task is done

**Important:** Subagents don't have skills. Include all instructions they need in the spawn prompt.

## Typical Flow

```bash
# 1. Spawn with task-scoped prompt
june spawn codex "Implement Task 1: Add user model.

Rules:
- Do not read or act on other tasks
- Stop after Task 1 and report
- Use 'june ask' if you need clarification
- Use 'june complete' when done" --name task-1 --detach

# 2. Poll for completion
june status
june peek task-1

# 3. Answer questions if waiting
june prompt task-1 "Use UUID for the ID field"

# 4. Clean up when done
june archive task-1
```

## Scope Control

**Do NOT attach full plan files.** Paste only the specific task text.

**Always include guardrails:**
- "Do not read or act on other tasks"
- "Stop after this task and report"

## Quick Reference

| Action | Command |
|--------|---------|
| Spawn | `june spawn codex "task" --name x --detach` |
| Status | `june status` |
| New output | `june peek <agent>` |
| Full log | `june log <agent>` |
| Send message | `june prompt <agent> "msg"` |
| Agent-to-agent msg | `june dm --from <sender> --to <recipient> "msg"` |
| Check channel | `june messages` |
| Kill | `june kill <agent>` |
| Archive | `june archive <agent>` |
