---
name: otto-orchestrate
description: Use when spawning Codex agents via Otto to delegate implementation work.
---

# Otto Orchestration

Spawn and coordinate Codex agents for implementation work.

## Orchestrator Role

You coordinate, you don't implement. Dispatch agents to do the work.

## CLI Commands

### Spawning Agents

```bash
otto spawn codex "task description" --name <name> --detach
```

Options:
- `--name <name>` - Readable ID (e.g., `planner`, `impl-1`)
- `--detach` - Return immediately, agent runs in background
- `--files <paths>` - Attach relevant files (keep minimal)
- `--context <text>` - Extra context

### Monitoring

```bash
otto status                  # List all agents and their status
otto peek <agent>            # New output since last peek (advances cursor)
                             # For completed/failed agents, shows full log (capped at 100 lines)
otto log <agent>             # Full history
otto log <agent> --tail 20   # Last 20 entries
otto messages                # Shared message channel
```

### Communication

```bash
otto prompt <agent> "message"                           # Send followup to agent
otto dm --from <sender> --to <recipient> "message"      # Send direct message between agents
                                                        # Supports cross-branch: --to feature/login:frontend
```

### Lifecycle

```bash
otto kill <agent>            # Terminate agent process
otto interrupt <agent>       # Pause agent (can resume later)
otto archive <agent>         # Archive completed/failed agent
```

## Agent Communication

Spawned agents use these to communicate back:
- `otto dm --from <self> --to <recipient> "message"` - Send direct message to another agent or orchestrator
- `otto ask "question?"` - Set status to WAITING, block for answer
- `otto complete` - Signal task is done

**Important:** Subagents don't have skills. Include all instructions they need in the spawn prompt.

## Typical Flow

```bash
# 1. Spawn with task-scoped prompt
otto spawn codex "Implement Task 1: Add user model.

Rules:
- Do not read or act on other tasks
- Stop after Task 1 and report
- Use 'otto ask' if you need clarification
- Use 'otto complete' when done" --name task-1 --detach

# 2. Poll for completion
otto status
otto peek task-1

# 3. Answer questions if waiting
otto prompt task-1 "Use UUID for the ID field"

# 4. Clean up when done
otto archive task-1
```

## Scope Control

**Do NOT attach full plan files.** Paste only the specific task text.

**Always include guardrails:**
- "Do not read or act on other tasks"
- "Stop after this task and report"

## Quick Reference

| Action | Command |
|--------|---------|
| Spawn | `otto spawn codex "task" --name x --detach` |
| Status | `otto status` |
| New output | `otto peek <agent>` |
| Full log | `otto log <agent>` |
| Send message | `otto prompt <agent> "msg"` |
| Agent-to-agent msg | `otto dm --from <sender> --to <recipient> "msg"` |
| Check channel | `otto messages` |
| Kill | `otto kill <agent>` |
| Archive | `otto archive <agent>` |
