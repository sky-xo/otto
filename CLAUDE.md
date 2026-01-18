# June - Project Context for Claude Code

## What is June?

June is a read-only TUI for viewing Claude Code subagent activity. It watches `~/.claude/projects/{project}/agent-*.jsonl` files and displays their transcripts in a terminal interface.

June is also a Claude Code plugin. Run `claude --plugin-dir .` to use june:* skills.

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

## Task Commands

Persist tasks across context compaction:

```bash
# Create root task
june task create "Implement auth feature"   # Output: t-a3f8b

# Create child tasks
june task create "Add middleware" --parent t-a3f8b

# Create multiple tasks at once
june task create "Task 1" "Task 2" "Task 3"

# List root tasks
june task list

# Show task details and children
june task list t-a3f8b

# Update task
june task update t-a3f8b --status in_progress
june task update t-a3f8b --note "Started work"
june task update t-a3f8b --status closed --note "Done"

# Delete task (soft delete, cascades to children)
june task delete t-a3f8b

# JSON output for machine consumption
june task list --json
june task create "New task" --json
```

Tasks are scoped to the current git repo and branch.

## Coding Conventions

- Follow TDD: write failing test, implement, verify
- Keep TUI logic in `internal/tui/`
- Keep file parsing in `internal/claude/`

## Documentation

- `docs/plans/` - Design docs and implementation plans
