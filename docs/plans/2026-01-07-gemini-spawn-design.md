# Gemini Spawn - Design Doc

## Problem

June currently only supports spawning Codex agents. Users want to spawn Gemini agents with the same workflow: spawn, monitor with peek/logs, view in TUI.

## Goal

Add Gemini spawning with feature parity to Codex:
- `june spawn gemini "task"` works like `june spawn codex "task"`
- Same peek/logs commands work for Gemini agents
- Gemini agents appear in June TUI

## Non-Goals

- Gemini-specific flags not yet supported by gemini-cli (thinking budget, max tokens)
- Different approval modes exposed as flags (we default to `auto_edit`)
- Gemini agent-to-agent messaging

## Design

### CLI Commands

```bash
june spawn gemini "task description" --name <name>
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--name` | auto-generated | Name prefix (ULID suffix added automatically) |
| `--model` | gemini default | Model to use (e.g., `gemini-2.5-pro`) |
| `--sandbox` | off | Run in Docker sandbox |
| `--yolo` | off | Full auto-approve (default is `auto_edit`) |

**Underlying command:**

```bash
gemini -p "task" --approval-mode auto_edit --output-format stream-json
```

With `--yolo` flag:
```bash
gemini -p "task" --yolo --output-format stream-json
```

### Architecture

New package `internal/gemini/` mirroring `internal/codex/`:

```
internal/gemini/
  home.go       # EnsureGeminiHome() - creates ~/.june/gemini/
  sessions.go   # FindSessionFile() - locates session JSONL files
  transcript.go # Parse stream-json events for peek/logs
```

### Key Difference from Codex

Codex automatically writes session files to `~/.codex/sessions/`. Gemini does not - it outputs `stream-json` to stdout.

**Solution:** We capture the stream-json output ourselves and write it to `~/.june/gemini/sessions/{session_id}.jsonl`.

### Database Changes

Add `type` column to agents table:

```sql
ALTER TABLE agents ADD COLUMN type TEXT DEFAULT 'codex';
```

This distinguishes Codex vs Gemini agents for transcript parsing.

### Spawn Flow

1. Create isolated home at `~/.june/gemini/` (for future config isolation)
2. Run `gemini -p "task" --approval-mode auto_edit --output-format stream-json`
3. Read first JSON event: `{"type":"init","session_id":"..."}`
4. Extract `session_id` as the agent's ULID
5. Create session file at `~/.june/gemini/sessions/{session_id}.jsonl`
6. Pipe remaining output to session file (while also draining stdout)
7. Insert agent record into SQLite with `type='gemini'`
8. Print agent name when process completes

### Gemini Stream-JSON Format

```jsonl
{"type":"init","session_id":"abc123","model":"gemini-2.5-pro"}
{"type":"message","role":"user","content":"Fix the parser bug"}
{"type":"tool_use","tool_name":"read_file","tool_id":"t1","parameters":{"path":"parser.go"}}
{"type":"tool_result","tool_id":"t1","status":"success","output":"..."}
{"type":"message","role":"assistant","content":"I've fixed the bug..."}
{"type":"result","status":"success","stats":{...}}
```

### Transcript Parsing for peek/logs

| Gemini Event | Display As |
|--------------|------------|
| `message` (role=user) | User prompt |
| `message` (role=assistant) | Agent response |
| `tool_use` | Tool call (file read, write, shell) |
| `tool_result` | Tool output |
| `result` | Final summary |

The peek/logs commands check agent type and use the appropriate parser (Codex or Gemini).

### Error Handling

| Scenario | Behavior |
|----------|----------|
| `gemini` not installed | Error: "gemini CLI not found - install with `npm install -g @google/gemini-cli`" |
| No `session_id` in first event | Kill process, return error |
| `--sandbox` without Docker | Let gemini fail with its own error message |
| Process exits non-zero | Log warning, still save transcript |

### TUI Integration

Gemini agents appear alongside Codex agents in June TUI:
- Query SQLite for agents where `type='gemini'`
- Watch their session files at `~/.june/gemini/sessions/`
- Distinguish with `[gemini]` label

## Comparison: Codex vs Gemini

| Aspect | Codex | Gemini |
|--------|-------|--------|
| Command | `codex exec --json` | `gemini -p --output-format stream-json` |
| Session ID source | `thread.started` event | `init` event |
| Session file | Written by Codex | Written by June |
| Session location | `~/.june/codex/sessions/` | `~/.june/gemini/sessions/` |
| Auto-approve | N/A | `--approval-mode auto_edit` default |

## Future Enhancements

When gemini-cli adds support:
- `--thinking-budget` flag for reasoning effort
- `--max-tokens` flag for output limits

## Implementation Plan

TBD - will create detailed plan after design approval.
