# Subagent Viewer MVP

**Status:** Ready for Implementation
**Date:** 2025-01-01
**Branch:** `june`

## Goal

A read-only viewer for Claude Code's subagent sessions. See what subagents are doing without losing track of them.

## The Problem

> "I spawn subagents and lose track of them"

When Claude Code spawns subagents via the Task tool, they run in the background. There's no easy way to see:
- Which subagents exist
- What they're doing
- What they've done

## Discovery

Claude Code logs everything we need:

```
~/.claude/projects/{project-path}/
├── {session-id}.jsonl          # Main session transcript
├── agent-{agent-id}.jsonl      # Subagent transcript (full conversation)
└── ...
```

Each `agent-*.jsonl` file contains the full subagent conversation:
- User/assistant messages
- Tool calls (Read, Edit, Bash, etc.)
- Tool results
- Model info, timestamps

**Verified structure:**
```json
{
  "type": "user" | "assistant",
  "message": {
    "role": "...",
    "content": [...],
    "stop_reason": "tool_use" | null
  },
  "agentId": "abc123",
  "sessionId": "parent-session-uuid",
  "timestamp": "2025-01-01T12:00:00.000Z"
}
```

## MVP Scope

**In scope:**
- Watch `~/.claude/projects/{current-project}/agent-*.jsonl` for changes
- List agents in left panel, sorted by most recently modified
- Show selected agent's transcript in right panel
- Auto-refresh as new content appears
- Active indicator based on file modification time

**Out of scope:**
- Multi-project view (single project only, detected from cwd)
- Prompting/interacting with agents
- Task list / flow tracking
- Spawning agents
- State persistence (future feature)

## UI

```
┌──────────────┬──────────────────────────────────────────┐
│ Subagents    │ agent-abc123                             │
│              │                                          │
│  ● abc123    │ > Implement user validation              │
│  ● def456    │                                          │
│  ✓ ghi789    │ Reading user.go...                       │
│              │ Found 3 validation points                │
│              │                                          │
│              │ Editing user.go:45-67                    │
│              │ + func validateEmail(s string) bool {    │
│              │                                          │
│              │ $ go test ./...                          │
│              │ ✓ PASS                                   │
│              │                                          │
│              │ Done.                                    │
└──────────────┴──────────────────────────────────────────┘
```

**Indicators:**
- `●` green - file modified in last 10 seconds (active)
- `✓` gray - file not modified recently (done)

**Sorting:** Most recently modified at top.

## Active Detection

**Approach:** File modification time heuristic.

```
mtime within 10s → active
mtime older      → done
```

**Limitation:** During long "thinking" periods (10-30s), file doesn't update. Agent may briefly show as "done" then flip back to "active" when response completes.

**Why this is okay for MVP:** Simple to implement, correct most of the time, can refine later.

**Alternative considered:** Parse last entry for completion patterns. More complex, may not be more accurate.

## Data Model

```go
type Subagent struct {
    ID        string    // From filename: agent-{id}.jsonl
    FilePath  string    // Full path to jsonl file
    LastMod   time.Time // File modification time
    IsActive  bool      // Modified in last 10 seconds
}

type TranscriptEntry struct {
    Type      string    // "user", "assistant"
    Message   Message   // Contains role, content, stop_reason
    AgentID   string
    Timestamp time.Time
}
```

## Implementation Approach

**Bootstrap from existing June TUI:**
- Keep: Bubbletea structure, panel layout, basic rendering
- Remove: SQLite, messaging, spawning, project grouping, most CLI commands
- Change: Data source from DB → file watching

**New package:** `internal/claude/`
- `projects.go` - Find Claude projects directory, map git root to project path
- `agents.go` - Scan for agent files, watch for changes
- `transcript.go` - Parse JSONL format

**Key changes to TUI:**
1. Replace `fetchAgentsCmd` → scan for `agent-*.jsonl` files
2. Replace `fetchTranscriptsCmd` → parse JSONL file
3. Remove chat input (read-only for MVP)
4. Simplify left panel (just agent list, no project headers)

## Project Path Mapping

Claude Code stores projects at:
```
~/.claude/projects/{path-with-dashes}/
```

Where `{path-with-dashes}` is the absolute path with `/` replaced by `-`.

Example:
```
/Users/glowy/code/otto → -Users-glowy-code-otto
```

## Open Questions (Deferred)

- How to handle very long transcripts? (Pagination? Lazy loading?)
- Should we show agent task/prompt in sidebar? (Need to parse first user message)

## Future: State Persistence

After MVP, June will add CLI commands for persisting task state:
- `june task add/complete` - Track progress across context clears
- `june checkpoint` - Save freeform state

The viewer would then show this persisted state alongside agent activity.

## Future: Multi-Model Spawning

Eventually, June will support spawning non-Claude agents:
- `june spawn codex "review this PR"`
- `june spawn gemini "implement this UI"`

But that's post-MVP.
