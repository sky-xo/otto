# Spawn & Log Commands Design

**Status:** Draft
**Created:** 2025-12-24
**Related:** `docs/plans/2025-12-24-brainstorm-session-notes.md`

## Problem Statement

When using Codex as an orchestrator (instead of Claude Code), June's blocking spawn hits Codex's ~60 second tool timeout. Additionally, orchestrators need a way to check agent output (logs) without reading the full history every time (token efficiency).

### Root Causes

1. **Codex timeout**: Codex CLI has ~60s tool harness timeout, no built-in background execution
2. **Claude Code workaround**: Claude has `run_in_background: true` which works with blocking spawn
3. **No log access**: Raw agent output (stdout/stderr) not accessible via CLI, only in TUI

## Decisions

### 1. Spawn Command

**Keep blocking as default**, add `--detach` flag for Codex users.

```bash
june spawn codex "task"            # Blocking (works with Claude's run_in_background)
june spawn codex "task" --detach   # Returns immediately (for Codex orchestrators)
```

**Rationale:**
- Claude Code users get notifications via `run_in_background: true` on blocking spawn
- Only Codex needs `--detach` to avoid timeout
- Defer building notification system - Claude's built-in mechanism is sufficient

### 2. Table Rename

Rename `transcript_entries` → `logs` for simplicity.

| Before | After |
|--------|-------|
| `transcript_entries` table | `logs` table |
| `repo.TranscriptEntry` | `repo.LogEntry` |
| `ListTranscriptEntries()` | `ListLogs()` |

### 3. New Commands: `june peek` and `june log`

Add CLI access to agent logs (currently only visible in TUI). Follows codex-subagent's proven API design.

**Two commands with different purposes:**

```bash
june peek <agent-id>             # Unread entries, advances cursor
june log <agent-id>              # Full history, no cursor change
june log <agent-id> --tail 5     # Last 5 entries, no cursor change
```

#### `june peek` (incremental read)

- Shows only unread log entries since last peek
- Advances cursor to latest entry shown
- If no new entries: "No new log entries for <agent>"
- Token efficient for polling orchestrators

#### `june log` (full history)

- Shows all log entries (or last N with `--tail`)
- Does NOT advance cursor
- Use case: debugging, reviewing what happened

### 4. Cursor Tracking

Track read position per agent to support unread filtering.

```sql
ALTER TABLE agents ADD COLUMN last_read_log_id TEXT;
```

**Cursor behavior:**

| Command | Cursor Effect |
|---------|---------------|
| `june peek agent` | Advances to latest shown |
| `june log agent` | Unchanged |
| `june log agent --tail 5` | Unchanged |

**Why `log` doesn't advance cursor:**
`log` is for viewing history, not consuming a stream. If you want to mark entries as read, use `peek`.

## Command Reference

### Spawn

```bash
june spawn <type> "<task>"              # Blocking, captures output
june spawn <type> "<task>" --detach     # Returns immediately
june spawn <type> "<task>" --name foo   # Custom agent name
june spawn <type> "<task>" --files x,y  # Relevant files
june spawn <type> "<task>" --context z  # Additional context
```

### Peek

```bash
june peek <agent-id>             # Unread entries, advances cursor
```

### Log

```bash
june log <agent-id>              # Full history
june log <agent-id> --tail N     # Last N entries
```

### Comparison with codex-subagent

| codex-subagent | June | Notes |
|----------------|------|-------|
| `peek <thread>` | `june peek <agent>` | Unread entries, advances cursor |
| `log <thread>` | `june log <agent>` | Full history, no cursor change |
| `log --tail N` | `june log --tail N` | Last N entries |
| `status <thread>` | `june status` | Agent status info |

## Implementation Notes

### Detached Spawn

When `--detach` is used:
1. Fork the agent process
2. Print agent ID to stdout
3. Return immediately (exit 0)
4. Agent continues running, posts `[exit]` message when done

Orchestrator workflow:
```bash
june spawn --detach codex "task"   # Returns: agent-id
june peek agent-id                  # Check for new output
june status                         # See if complete
```

### Log Storage

Logs stored in `logs` table (renamed from `transcript_entries`):
- `id`: Entry ID
- `agent_id`: Foreign key to agents
- `stream`: "stdout" or "stderr"
- `content`: Log content
- `created_at`: Timestamp

### Schema Migration

```sql
-- Rename table
ALTER TABLE transcript_entries RENAME TO logs;

-- Add cursor tracking
ALTER TABLE agents ADD COLUMN last_read_log_id TEXT;
```

## Future Considerations

1. **`june wait` command**: Block until specific agent(s) complete
2. **Notification system**: For super-orchestrator V1+
3. **Log retention**: Currently 7-day cleanup on DB open

## Open Questions

1. **Detached transcript capture**: When spawn returns immediately, who captures stdout? Current thinking: spawn process stays alive in background, captures to DB.

2. **Log format**: Should `june log` show raw output or parse JSON events (for Codex)?
