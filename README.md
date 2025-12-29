# Otto

A multi-agent orchestrator for Claude Code and Codex.

Otto lets you spawn multiple AI agents that work in parallel, communicate through a shared message stream, and coordinate handoffs—all from a single Claude Code session.

## Features

- **Spawn agents**: Launch Claude Code or Codex agents for parallel tasks
- **Shared messaging**: Agents communicate via @mentions in a shared channel
- **Persistent state**: SQLite-backed, survives session restarts
- **Real-time monitoring**: Watch message stream with TUI or simple polling
- **Orchestrator control**: Route questions, coordinate handoffs, escalate to human

## Installation

```bash
go install github.com/sky-xo/otto@latest
```

Or build from source:

```bash
git clone https://github.com/sky-xo/otto
cd otto
make build
```

## Quick Start

```bash
# Spawn an agent
otto spawn claude "implement user authentication"

# Check agent status
otto status

# View messages
otto messages

# Launch TUI (watch messages, manage agents)
otto
```

## Commands

| Command | Description |
|---------|-------------|
| `otto` | Launch TUI (same as `otto watch`) |
| `otto spawn <type> "<task>"` | Spawn a new claude/codex agent |
| `otto status [--all] [--archive]` | List agent status |
| `otto messages` | View message stream |
| `otto prompt <agent> "<msg>"` | Send prompt to an agent |
| `otto attach <agent>` | Get command to attach to agent session |
| `otto archive <agent>` | Archive a completed/failed agent |
| `otto dm --from <agent> --to <recipient> "<msg>"` | Post direct message |
| `otto ask --id <agent> "<q>"` | Agent asks a question |
| `otto complete --id <agent> "<summary>"` | Agent marks task done |

## How It Works

```
You ←→ Claude Code (orchestrator)
         │
         │ calls otto CLI
         ▼
    ┌─────────────────────────────────────┐
    │  otto CLI                           │
    │  - spawn agents                     │
    │  - check status                     │
    │  - send/receive messages            │
    └─────────────────────────────────────┘
         │
         ├──────────────┬──────────────┐
         ▼              ▼              ▼
    Claude Code     Codex         Claude Code
    (design)      (implement)     (review)
```

State is stored in `~/.otto/otto.db` (global database with project/branch scoping).

## Development

```bash
make build    # Build binary
make test     # Run tests
make watch    # Build and run TUI
```

## License

MIT
