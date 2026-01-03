# June Spawn - Design Doc

## Problem

When orchestrating multi-model workflows, Claude needs to spawn Codex (and eventually Gemini) agents and get results back. Currently this capability was lost when otto-v0 was stripped down to the current June TUI.

## Goal

Add minimal spawn functionality to June:
- Spawn Codex agents from Claude
- See spawned agents in June TUI
- Provide peek/logs commands for Claude to check on backgrounded agents

## Non-Goals (v0)

- `june status` command (use Claude Code's background notifications)
- Fancy agent names (quoted strings, spaces, mixed case)
- Gemini support
- Agent-to-agent messaging

## Design

### CLI Commands

```bash
june spawn codex "task description" --name <name>
```
- Blocks and streams stdout
- Claude can background with `run_in_background: true` on Bash tool
- Captures thread_id from first JSON event
- Stores agent state in SQLite (`~/.june/june.db`)

```bash
june peek <name>
```
- Shows output since last peek (advances cursor)
- Reads from Codex session file at cursor position

```bash
june logs <name>
```
- Shows full transcript
- Does not advance cursor

### Database

SQLite database at `~/.june/june.db`:

```sql
CREATE TABLE agents (
  name TEXT PRIMARY KEY,
  ulid TEXT NOT NULL,
  session_file TEXT NOT NULL,
  cursor INTEGER DEFAULT 0,
  pid INTEGER,
  spawned_at TEXT NOT NULL
);
```

| Column | Purpose |
|--------|---------|
| name | Agent name (e.g., "impl-1") - primary key |
| ulid | Thread ID from Codex, maps to session file |
| session_file | Full path to Codex session JSONL |
| cursor | Last line read (for peek) |
| pid | Process ID (to check if still running) |
| spawned_at | ISO timestamp |

**Why SQLite:**
- Atomic writes (no corruption on concurrent spawns)
- Queryable (for future `june status`, filtering, etc.)
- Schema migrations if we add fields later
- Already proven in otto-v0

### How Spawn Works

1. Run `codex exec --json "task"`
2. Parse first line: `{"type":"thread.started","thread_id":"..."}`
3. Extract thread_id (ULID)
4. Find session file: `~/.codex/sessions/YYYY/MM/DD/rollout-*{ulid}*.jsonl`
5. Insert agent record into SQLite (name, ulid, session_file, pid)
6. Stream remaining stdout to terminal
7. On completion, agent record remains for peek/logs

### How Peek Works

1. Query agent by name from SQLite
2. Read session_file from cursor position
3. Parse JSONL, extract relevant content (messages, tool calls)
4. Update cursor in SQLite
5. Return formatted output

### How Logs Works

Same as peek, but:
- Reads from beginning (cursor 0)
- Does not update cursor in SQLite

### TUI Integration

June TUI currently watches `~/.claude/projects/{project}/agent-*.jsonl`.

For Codex agents:
- Query SQLite for list of spawned agents
- Watch their session_files
- Display alongside Claude subagents
- Distinguish with `[codex]` label (or similar)

### Codex Session File Format

Codex writes to `~/.codex/sessions/YYYY/MM/DD/rollout-{timestamp}-{ulid}.jsonl`

Event types include:
- `session_meta` - metadata (id, cwd, model, etc.)
- `response_item` - AI and user messages
- `function_call` / `function_call_output` - command execution
- `agent_reasoning` - AI reasoning

We parse these to extract displayable transcript.

## Open Questions

1. **Agent cleanup**: When/how do we remove old agents?
   - Option: Manual `june rm <name>`
   - Option: Auto-expire after N days
   - Option: Leave for v1

2. **Error handling**: What if Codex fails to start or crashes?
   - Add status column to track state?
   - Just let it fail and user sees in output?

3. **Concurrent spawns**: What if same name used twice?
   - Error? (SQLite will fail on duplicate primary key)
   - Overwrite?
   - Append suffix?

4. **Consolidate Claude agent cache?**: Currently June caches Claude subagent descriptions in a JSON file. Move to same SQLite DB?
   - Pro: Single source of truth, consistent patterns
   - Con: Scope creep for v0
   - Decision: TBD

## Implementation Plan

TBD - will create detailed plan after design approval.
