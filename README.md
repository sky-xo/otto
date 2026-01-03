# June

A subagent viewer for Claude Code.

<img width="1512" height="911" alt="june-screenshot" src="https://github.com/user-attachments/assets/f5502731-e81b-4fe4-8ca7-d8aa23746367" />

## What It Shows

- List of all subagents spawned in your project (grouped by branch)
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
