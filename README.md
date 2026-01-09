# June

A subagent viewer for Claude Code.

<img width="1512" height="911" alt="june-screenshot" src="https://github.com/user-attachments/assets/f5502731-e81b-4fe4-8ca7-d8aa23746367" />

## What It Shows

- List of all subagents spawned in your project (grouped by branch)
- Real-time transcript of each agent's conversation

## Installation

```bash
# macOS
brew install sky-xo/tap/june --cask

# Go (any platform)
go install github.com/sky-xo/june@latest
```

## Usage

Run `june` from any git repository where you've used Claude Code:

```bash
june
```

The TUI will launch showing any subagents that have been spawned in that project.

## Spawning Agents

June can spawn and monitor both Codex and Gemini agents:

```bash
# Codex agents
june spawn codex "your task here" --name refactor   # Output: refactor-9c4f
june spawn codex "your task here"                   # Output: swift-falcon-7d1e

# Gemini agents
june spawn gemini "your task here" --name research  # Output: research-3b7a
june spawn gemini "your task here"                  # Output: quick-fox-8d2e

# Monitor agents
june peek refactor-9c4f                             # Show new output since last peek
june logs refactor-9c4f                             # Show full transcript
```

Names always include a unique 4-character suffix. The `--name` flag sets a prefix; if omitted, an adjective-noun prefix is auto-generated.

### Spawn Options

| Flag | Description |
|------|-------------|
| `--name` | Custom prefix for agent name |
| `--sandbox` | Enable sandbox. Codex: `--sandbox` (defaults to `workspace-write`) or `--sandbox=VALUE` where VALUE is `read-only`, `workspace-write`, or `danger-full-access`. Gemini: `--sandbox` only (no value accepted) |
| `--model` | Model to use (Codex: `o3`, `o4-mini`; Gemini: `gemini-2.5-pro`, etc.) |
| `--yolo` | Auto-approve all tool calls (Gemini only) |

Agent state is stored in `~/.june/june.db`.

## How It Works

June watches agent transcripts from multiple sources:

```
# Claude Code subagents
~/.claude/projects/{project-path}/agent-{id}.jsonl

# Gemini CLI sessions
~/.june/gemini/sessions/{session-id}.jsonl
```

The TUI displays these transcripts with real-time updates.

## Development

```bash
make build    # Build the binary
make test     # Run tests
./june        # Launch TUI
```

## License

MIT
