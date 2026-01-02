# June

A subagent viewer for Claude Code.

## What It Shows

- List of all subagents spawned in your current project
- Real-time transcript of each agent's conversation

## Installation

Install with Go:

```bash
go install github.com/sky-xo/june@latest
```

Or build from source:

```bash
git clone https://github.com/sky-xo/june
cd june
make build
```

## Usage

Run `june` from any git repository where you've used Claude Code:

```bash
june
```

The TUI will launch showing any subagents that have been spawned in that project.

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate up/down in agent list |
| `u` / `d` | Page up/down in transcript |
| `Tab` | Switch focus between panels |
| `q` | Quit |

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
