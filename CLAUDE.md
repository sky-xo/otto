# June - Project Context for Claude Code

## What is June?

June is a read-only TUI for viewing Claude Code subagent activity. It watches `~/.claude/projects/{project}/agent-*.jsonl` files and displays their transcripts in a terminal interface.

## Quick Commands

```bash
make build    # Build the binary
make test     # Run all tests
./june        # Run TUI
```

## Architecture

```
~/.claude/projects/{project-path}/
  agent-{id}.jsonl   # Subagent transcripts (read by June)
```

**Packages:**
- `main.go` - Entry point
- `internal/cli/` - Cobra root command, launches TUI
- `internal/claude/` - Agent file scanning and JSONL parsing
- `internal/scope/` - Git project/branch detection
- `internal/tui/` - Bubbletea TUI

## Usage

Run `june` in a git repository where Claude Code has been used:

```bash
june
```

The TUI shows:
- Left panel: List of subagents (sorted by modification time)
- Right panel: Selected agent's transcript
- Activity indicators based on file modification time

## Spawn Commands

Spawn and monitor Codex agents:

```bash
june spawn codex "task" --name refactor   # Output: refactor-9c4f
june spawn codex "task"                   # Output: swift-falcon-7d1e

june peek refactor-9c4f                   # Show new output since last peek
june logs refactor-9c4f                   # Show full transcript
```

Names always include a unique 4-char ULID suffix. `--name` sets a prefix; if omitted, an adjective-noun prefix is auto-generated.

Agent state is stored in `~/.june/june.db` (SQLite).

## Coding Conventions

- Follow TDD: write failing test, implement, verify
- Keep TUI logic in `internal/tui/`
- Keep file parsing in `internal/claude/`

## Documentation

- `docs/plans/` - Design docs and implementation plans
