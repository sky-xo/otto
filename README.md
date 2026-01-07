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

## Spawning Codex Agents

```bash
june spawn codex "your task here" --name refactor   # Output: refactor-9c4f
june spawn codex "your task here"                   # Output: swift-falcon-7d1e

june peek refactor-9c4f                             # Show new output since last peek
june logs refactor-9c4f                             # Show full transcript
```

Names always include a unique 4-character suffix. The `--name` flag sets a prefix; if omitted, an adjective-noun prefix is auto-generated.

Agent state is stored in `~/.june/june.db`.

## How It Works

Claude Code stores subagent transcripts at:

```
~/.claude/projects/{project-path}/agent-{id}.jsonl
```

June watches these files and displays their contents.

## Development

```bash
make build    # Build the binary
make test     # Run tests
./june        # Launch TUI
```

## License

MIT
