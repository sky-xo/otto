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
3. Buffer first JSON event: `{"type":"init","session_id":"...","timestamp":"..."}`
4. Extract `session_id` (UUID format) as the agent's identifier
5. Create session file at `~/.june/gemini/sessions/{session_id}.jsonl`
6. Write buffered init event, then stream remaining output to session file
7. Insert agent record into SQLite with `type='gemini'`
8. Print agent name when process completes

**Note:** We buffer the first event before creating the file so we know the session_id for the filename, then write it as the first line.

### Gemini Stream-JSON Format (Verified)

Tested with gemini-cli v0.22.5. All events include ISO 8601 timestamps.

```jsonl
{"type":"init","timestamp":"2026-01-07T10:02:12.875Z","session_id":"8b6238bf-8332-4fc7-ba9a-2f3323119bb2","model":"auto-gemini-3"}
{"type":"message","timestamp":"...","role":"user","content":"Fix the parser bug"}
{"type":"message","timestamp":"...","role":"assistant","content":"I've","delta":true}
{"type":"message","timestamp":"...","role":"assistant","content":" fixed","delta":true}
{"type":"tool_use","timestamp":"...","tool_name":"read_file","tool_id":"read_file-123-abc","parameters":{"path":"parser.go"}}
{"type":"tool_result","timestamp":"...","tool_id":"read_file-123-abc","status":"success","output":"..."}
{"type":"result","timestamp":"...","status":"success","stats":{"total_tokens":7266,"duration_ms":3262,"tool_calls":1}}
```

**Key implementation detail:** Assistant messages arrive as multiple chunks with `"delta":true`. The transcript parser must accumulate these into a single message.

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
| Session ID source | `thread.started` event (`thread_id`) | `init` event (`session_id`, UUID format) |
| Session file | Written by Codex | Written by June |
| Session location | `~/.june/codex/sessions/` | `~/.june/gemini/sessions/` |
| Auto-approve | N/A | `--approval-mode auto_edit` default |
| Message streaming | Single events | Chunked with `delta:true` |

## Future Enhancements

When gemini-cli adds support:
- `--thinking-budget` flag for reasoning effort
- `--max-tokens` flag for output limits

## Implementation Plan

TBD - will create detailed plan after design approval.
