# TUI Channel View & Agent Transcripts

**Status:** Draft
**Date:** 2025-12-23

## Overview

Redesign the Otto TUI to provide a channel-based interface where users can view the main coordination stream and drill into individual agent transcripts. Includes capturing full agent session logs (prompts sent + stdout output) and keeping completed agents resumable.

## Key Changes

1. **Rename `otto watch` to `otto`** - TUI becomes the default command
2. **Channel-based layout** - Left panel lists channels (Main + agents), right panel shows content
3. **Capture agent transcripts** - Store prompts sent to agents and their stdout output
4. **Keep completed agents** - Don't delete on completion, mark with timestamp, cleanup after 7 days
5. **Resumable agents** - Completed agents can be resumed via their stored session_id

## TUI Layout

```
┌─────────────────────────────────────────────────────────────────┐
│ Otto                                                            │
├──────────────┬──────────────────────────────────────────────────┤
│ Channels     │ Main                                             │
│ ───────────  │                                                  │
│   Main       │ agent-1       Starting work on auth feature...   │
│ ● build-auth │ orchestrator  @agent-1 implement login endpoint  │
│ ● fix-tests  │ agent-2       Done. Created 3 files.             │
│ ○ schema-db  │ agent-1       Question: should I use JWT?        │
│ ✗ broken-one │ orchestrator  @agent-1 use JWT, here's the spec  │
│              │                                                  │
├──────────────┴──────────────────────────────────────────────────┤
│ q: quit | j/k: navigate | Enter: select | Esc: back to Main     │
└─────────────────────────────────────────────────────────────────┘
```

### Channel List (Left Panel)

- **Main** always at top
- Agents sorted by status: active → blocked → complete → failed
- Status indicators:
  - `●` green = active/busy
  - `●` yellow = blocked (waiting on `ask`)
  - `○` grey = complete (resumable)
  - `✗` red/grey = failed
- Unread count badge: `agent-1 (3)` if new activity

### Content Area (Right Panel)

**Main channel:**
- Shows coordination messages (`say`, `ask`, `complete`, `exit`)
- Orchestrator prompts shown grey/muted: `orchestrator @agent-1 message...`
- Agent messages normal styling with username color

**Agent channel:**
- Full transcript: prompts (direction: in) + stdout (direction: out) interleaved chronologically
- Shows complete session history

### Navigation

- **Mouse click** on channel to select
- **j/k or arrows** to move selection
- **Enter** to view selected channel
- **Escape** returns to Main
- **g/G** top/bottom of content

## Database Schema

### Option Selected: Hybrid Pointer (No Duplication)

Based on analysis from both Claude and Codex agents, we use separate tables with pointer references to avoid content duplication.

### Schema Changes

```sql
-- Agents table: add completed_at, standardize statuses
ALTER TABLE agents ADD COLUMN completed_at DATETIME;
-- Status values: 'busy', 'blocked', 'complete', 'failed' (deprecate legacy values)

-- Messages table: add to_id for prompt routing
ALTER TABLE messages ADD COLUMN to_id TEXT;
-- to_id = NULL for broadcasts, agent-id for direct prompts
-- type 'prompt' for orchestrator → agent messages

-- New: transcript_entries table for raw agent I/O
CREATE TABLE transcript_entries (
  id TEXT PRIMARY KEY,
  agent_id TEXT NOT NULL,
  direction TEXT NOT NULL,     -- 'in' (prompt) or 'out' (stdout/stderr)
  stream TEXT,                 -- NULL for prompts, 'stdout' or 'stderr' for output
  content TEXT NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
-- Note: Capture both stdout and stderr from the start

CREATE INDEX idx_transcript_agent ON transcript_entries(agent_id, created_at);
CREATE INDEX idx_agents_cleanup ON agents(completed_at) WHERE completed_at IS NOT NULL;
CREATE INDEX idx_messages_to ON messages(to_id, created_at);
```

### Data Flow

**When spawning an agent:**
1. Create agent record (status: 'busy')
2. Store prompt in `messages` (type: 'prompt', to_id: agent-id, from_id: 'orchestrator')
3. Store same prompt in `transcript_entries` (direction: 'in')
4. Start process, tee stdout to terminal AND `transcript_entries` (direction: 'out')

**When prompting an existing agent:**
1. Store prompt in `messages` (type: 'prompt', to_id: agent-id)
2. Store in `transcript_entries` (direction: 'in')
3. Resume agent session, continue capturing stdout

**When agent completes:**
1. Agent calls `otto complete` → creates completion message
2. Update agent: `status = 'complete'`, `completed_at = NOW()`
3. Agent row persists (not deleted)

**When agent fails:**
1. Process exit detected → creates exit message
2. Update agent: `status = 'failed'`, `completed_at = NOW()`

### Queries

```sql
-- Main channel (coordination messages including prompts)
SELECT * FROM messages
ORDER BY created_at;

-- Agent transcript view
SELECT * FROM transcript_entries
WHERE agent_id = ?
ORDER BY created_at;

-- Cleanup (run periodically)
DELETE FROM transcript_entries
WHERE agent_id IN (
  SELECT id FROM agents
  WHERE completed_at < datetime('now', '-7 days')
);
DELETE FROM messages
WHERE to_id IN (
  SELECT id FROM agents
  WHERE completed_at < datetime('now', '-7 days')
);
DELETE FROM agents
WHERE completed_at < datetime('now', '-7 days');
```

## Agent Lifecycle

```
                    ┌─────────┐
                    │ spawned │
                    └────┬────┘
                         │
                    ┌────▼────┐
              ┌─────│  busy   │─────┐
              │     └────┬────┘     │
              │          │          │
         otto ask    otto complete  process dies
              │          │          │
         ┌────▼────┐ ┌───▼────┐ ┌───▼────┐
         │ blocked │ │complete│ │ failed │
         └────┬────┘ └───┬────┘ └───┬────┘
              │          │          │
         otto prompt     └────┬─────┘
              │               │
         ┌────▼────┐    7 days pass
         │  busy   │          │
         └─────────┘    ┌─────▼─────┐
                        │  deleted  │
                        └───────────┘
```

### Resumable Agents

Completed/failed agents remain in the database with their `session_id`. They can be resumed:

```bash
# Orchestrator prompts a completed agent
otto prompt schema-db "What about the multi-agent prompt edge case?"

# Otto uses stored session_id to resume
claude --resume <session-id> -p "..."
# or
codex exec resume <session-id> "..."
```

Agent status returns to 'busy', transcript capture continues.

## Stdout Capture Implementation

Modify `internal/exec/runner.go` to tee stdout:

```go
// Pseudocode
func (r *Runner) StartWithCapture(cmd string, agentID string, db *sql.DB) {
    process := exec.Command(...)

    stdout, _ := process.StdoutPipe()

    go func() {
        reader := bufio.NewReader(stdout)
        buffer := strings.Builder{}

        for {
            line, err := reader.ReadString('\n')
            if err != nil {
                break
            }

            // Write to terminal (existing behavior)
            os.Stdout.Write([]byte(line))

            // Buffer for DB
            buffer.WriteString(line)

            // Flush to DB periodically (every N lines or K bytes)
            if buffer.Len() > 4096 {
                repo.CreateTranscriptEntry(db, agentID, "out", "stdout", buffer.String())
                buffer.Reset()
            }
        }

        // Flush remaining
        if buffer.Len() > 0 {
            repo.CreateTranscriptEntry(db, agentID, "out", "stdout", buffer.String())
        }
    }()
}
```

## Command Changes

### `otto` (no args)
- Launches TUI (previously `otto watch`)
- Detects terminal vs pipe (TUI vs plain text)

### `otto watch`
- Removed (no alias)

### `otto spawn`
- Now stores prompt in `messages` AND `transcript_entries`
- Returns immediately, stdout captured in background

### `otto prompt`
- Stores prompt in both tables
- Resumes agent session

### `otto complete`
- Sets `status = 'complete'`, `completed_at = NOW()`
- No longer deletes agent row

## Design Decisions

Finalized through discussion with Codex agents:

1. **Stderr** - Capture now as `stream='stderr'`. Low incremental cost while building stdout capture, valuable for debugging agent failures.

2. **Status enum** - Standardize to `{busy, blocked, complete, failed}`. Deprecate legacy values (`idle`, `working`, `active`, `pending`). One-time migration, cleaner codebase.

3. **Cleanup location** - Run opportunistically on every DB open. The DELETE is idempotent and fast with indexed `completed_at`. Handle errors gracefully (log and continue, don't fail the command). Skip on SQLITE_BUSY. TUI tick can also run it for long sessions.

4. **On resume** - Clear `completed_at` (set to NULL) and set status back to `busy`. Agent is active again; the 7-day clock restarts on next completion.

## Future Work

1. **Chunk size tuning** - Adjust 4KB buffer based on real usage
2. **Timestamp precision** - May need microseconds if events interleave rapidly
3. **Multi-agent prompts** - One prompt to multiple agents (shared prompt_id)
4. **Third column** - Right-side panel for todos/status

## Migration Path

1. Add `completed_at` column to agents
2. Create `transcript_entries` table
3. Add `to_id` column to messages
4. Update spawn/prompt commands to capture transcripts
5. Update complete command to set timestamp instead of delete
6. Add cleanup job (in TUI tick or separate command)
7. Redesign TUI with channel layout

## References

- Claude Code `--resume`: https://docs.claude.com/en/docs/claude-code/cli-usage
- Codex `resume`: https://developers.openai.com/codex/cli/features
