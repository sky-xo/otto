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
- `cmd/june/` - Entry point
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

## Keyboard Shortcuts

- `j`/`k` - Navigate agent list
- `u`/`d` - Page up/down in transcript
- `Tab` - Switch panel focus
- `q` - Quit

### Selection Mode (mouse-initiated)
- Click+drag in transcript - Start text selection
- `C` - Copy selection to clipboard and exit
- `Esc` - Exit selection mode without copying

## Coding Conventions

- Follow TDD: write failing test, implement, verify
- Keep TUI logic in `internal/tui/`
- Keep file parsing in `internal/claude/`

## Documentation

- `docs/plans/` - Design docs and implementation plans
