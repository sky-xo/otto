# June

A multi-agent orchestrator for Claude Code and Codex.

June lets you spawn multiple AI agents that work in parallel, communicate through a shared message stream, and coordinate handoffs—all from a single Claude Code session.

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
cd june
make build
```

## Quick Start

```bash
# Spawn an agent
june spawn claude "implement user authentication"

# Check agent status
june status

# View messages
june messages

# Launch TUI (watch messages, manage agents)
june
```

## Commands

| Command | Description |
|---------|-------------|
| `june` | Launch TUI (same as `june watch`) |
| `june spawn <type> "<task>"` | Spawn a new claude/codex agent |
| `june status [--all] [--archive]` | List agent status |
| `june messages` | View message stream |
| `june prompt <agent> "<msg>"` | Send prompt to an agent |
| `june attach <agent>` | Get command to attach to agent session |
| `june archive <agent>` | Archive a completed/failed agent |
| `june dm --from <agent> --to <recipient> "<msg>"` | Post direct message |
| `june ask --id <agent> "<q>"` | Agent asks a question |
| `june complete --id <agent> "<summary>"` | Agent marks task done |

## How It Works

```
You ←→ Claude Code (orchestrator)
         │
         │ calls june CLI
         ▼
    ┌─────────────────────────────────────┐
    │  june CLI                           │
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

State is stored in `~/.june/june.db` (global database with project/branch scoping).

## Development

```bash
make build    # Build binary
make test     # Run tests
make watch    # Build and run TUI
```

## License

MIT
