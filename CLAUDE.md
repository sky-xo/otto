# Otto - Project Context for Claude Code

## What is Otto?

Otto is a multi-agent orchestrator CLI that enables Claude Code to spawn and coordinate multiple AI agents (Claude Code and Codex) working in parallel.

## Quick Commands

```bash
make build    # Build the binary
make test     # Run all tests
make watch    # Build and run TUI
```

## Architecture

```
~/.otto/orchestrators/<project>/<branch>/otto.db   # SQLite per orchestrator
```

**Packages:**
- `cmd/otto/` - Entry point
- `internal/cli/` - Cobra root command
- `internal/cli/commands/` - All CLI commands
- `internal/repo/` - Database operations (agents, messages)
- `internal/db/` - SQLite schema and connection
- `internal/scope/` - Git project/branch detection
- `internal/tui/` - Bubbletea TUI for watch --ui
- `internal/exec/` - Process execution abstraction

## Key Commands

| Command | Purpose |
|---------|---------|
| `spawn <type> "<task>"` | Spawn claude/codex agent |
| `prompt <agent> "<msg>"` | Send message to agent |
| `say/ask/complete` | Agent messaging |
| `messages/status` | View messages and agent states |
| `watch` | Monitor message stream (TUI in terminal, plain text when piped) |

## Coding Conventions

- Follow TDD: write failing test, implement, verify
- Use `repo` package for all database operations
- Commands use `run*` functions for testability
- Agents require `--id` flag, orchestrator commands reject it

## Database Schema

Two tables: `agents` (id, type, task, status, session_id) and `messages` (id, from_id, type, content, mentions, read_by).

## Documentation

- `docs/ROADMAP.md` - Feature status by version
- `docs/ARCHITECTURE.md` - How Otto works
- `docs/SCENARIOS.md` - Usage scenarios / test cases
- `docs/plans/` - Design docs and implementation plans

## Documentation Conventions

When brainstorming new features or design ideas:
- Create `docs/plans/YYYY-MM-DD-<feature>-design.md`
- Mark status at top: `Draft` → `Ready for review` → `Approved`
- Keep ROADMAP.md as quick overview, detailed design goes in plan files
