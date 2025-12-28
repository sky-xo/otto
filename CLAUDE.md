# Otto - Project Context for Claude Code

## What is Otto?

Otto is a multi-agent orchestrator CLI that enables Claude Code to spawn and coordinate multiple AI agents (Claude Code and Codex) working in parallel.

## Quick Commands

```bash
make build    # Build the binary
make test     # Run all tests
./otto        # Run TUI (or: make watch)
```

## Architecture

```
~/.otto/otto.db   # Global SQLite database (project/branch columns in tables)
```

**Packages:**
- `cmd/otto/` - Entry point
- `internal/cli/` - Cobra root command
- `internal/cli/commands/` - All CLI commands
- `internal/repo/` - Database operations (agents, messages, tasks, logs)
- `internal/db/` - SQLite schema and connection
- `internal/scope/` - Git project/branch detection
- `internal/tui/` - Bubbletea TUI for watch command
- `internal/exec/` - Process execution abstraction

## Key Commands

| Command | Purpose |
|---------|---------|
| `otto` | Launch TUI (same as `otto watch`) |
| `spawn <type> "<task>"` | Spawn claude/codex agent |
| `prompt <agent> "<msg>"` | Send message to agent |
| `say/ask/complete` | Agent messaging |
| `messages/status` | View messages and agent states |

## Coding Conventions

- Follow TDD: write failing test, implement, verify
- Use `repo` package for all database operations
- Commands use `run*` functions for testability
- Agents require `--id` flag, orchestrator commands reject it

## Database Schema

Four tables with project/branch scoping:
- `agents` (project, branch, name, type, task, status, session_id, pid, compacted_at, ...)
- `messages` (id, project, branch, from_agent, to_agent, type, content, mentions, ...)
- `tasks` (project, branch, id, parent_id, name, sort_index, assigned_agent, result)
- `logs` (id, project, branch, agent_name, agent_type, event_type, content, raw_json, ...)

## Documentation

- `TODO.md` - Feature status and next priorities
- `docs/ARCHITECTURE.md` - How Otto works
- `docs/SCENARIOS.md` - Usage scenarios / test cases
- `docs/plans/` - Design docs and implementation plans

## Documentation Conventions

When brainstorming new features or design ideas:
- Create `docs/plans/YYYY-MM-DD-<feature>-design.md`
- Mark status at top: `Draft` → `Ready for review` → `Approved`
- Keep TODO.md as quick overview, detailed design goes in plan files

## Using Otto for Subagent Development

See `.claude/skills/otto-orchestrate/SKILL.md` for full details.

**Key pattern:**
1. Spawn with `run_in_background: true`
2. Wait—you'll be notified automatically when the agent completes
3. Use `BashOutput` with `block: true` to retrieve results

You don't need to poll with `otto status` or `otto peek`—the notification comes automatically. But if things seem slow, feel free to check progress with `otto peek <agent>`.

**Progress:** Task 1.1 complete (commit ea21dd7, spec review passed). Next: Task 1.2.
