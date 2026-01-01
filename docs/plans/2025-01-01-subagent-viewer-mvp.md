# Subagent Viewer MVP

**Status:** In Progress
**Date:** 2025-01-01
**Branch:** `june`

## Progress

- [x] Decided on single app approach (viewer + future spawner in one binary)
- [x] Renamed project: otto → june (for voice dictation compatibility)
- [x] Preserved old orchestration experiment in `otto-v0` branch
- [ ] Gut TUI to remove unneeded features
- [ ] Write Claude Code file parser (`internal/claude/`)
- [ ] Wire up new data source
- [ ] Test with real subagent sessions

## Goal

A read-only viewer for Claude Code's subagent sessions. See what subagents are doing without losing track of them.

## The Problem

> "I spawn subagents and lose track of them"

When Claude Code spawns subagents via the Task tool, they run in the background. There's no easy way to see:
- Which subagents exist
- What they're doing
- What they've done

## Discovery

Claude Code already logs everything we need:

```
~/.claude/projects/{project-path}/
├── {session-id}.jsonl          # Main session transcript
├── agent-{agent-id}.jsonl      # Subagent transcript (full conversation)
├── agent-{agent-id}.jsonl      # Another subagent
└── ...
```

Each `agent-*.jsonl` file contains the full subagent conversation in the same format as main sessions:
- User/assistant messages
- Tool calls (Read, Edit, Bash, etc.)
- Tool results
- Thinking blocks

## MVP Scope

**In scope:**
- Watch `~/.claude/projects/{current-project}/` for agent files
- List all subagents in left panel
- Show selected subagent's transcript in right panel
- Auto-refresh as new content appears

**Out of scope (for now):**
- Prompting/interacting with agents
- Task list / flow tracking
- Cross-project visibility
- Spawning agents
- Any orchestration

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

**Status indicators:**
- `●` - agent file still being written to (active)
- `✓` - agent file not modified recently (done)

## Data Model

```go
type Subagent struct {
    ID        string    // From filename: agent-{id}.jsonl
    FilePath  string    // Full path to jsonl file
    LastMod   time.Time // File modification time
    IsActive  bool      // Modified in last N seconds
}

type TranscriptEntry struct {
    Type      string    // "user", "assistant"
    Content   []ContentBlock
    Timestamp time.Time
    // ... other fields from jsonl
}
```

## Implementation Approach

**Bootstrap from June's existing TUI:**
- Keep: Bubbletea structure, panel layout, transcript rendering
- Remove: SQLite, messaging, spawning, most CLI commands
- Change: Data source from June DB → Claude Code jsonl files

**Key changes:**
1. Replace `fetchAgentsCmd` → scan for `agent-*.jsonl` files
2. Replace `fetchTranscriptsCmd` → parse jsonl file
3. Remove chat input (read-only for MVP)
4. Simplify left panel (just agent list, no projects/channels)

## File Structure

```
cmd/june/main.go           # Entry point (simplified)
internal/
  claude/                  # NEW: Claude Code file parsing
    projects.go            # Find project directory
    agents.go              # Scan for agent files
    transcript.go          # Parse jsonl format
  tui/
    watch.go               # Simplified TUI (keep structure)
    ...
```

## Key Decisions

1. **Single app, not two** - June will be both viewer and (later) multi-model spawner. Internally separated but one binary.

2. **Future: Codex/Gemini spawning** - June will later add `june spawn codex` for multi-model orchestration. Viewer-only for MVP.

3. **Superpowers plugin separate** - Problems 1-3 (skill amnesia, state amnesia, context cliff) will be addressed by a separate superpowers plugin, not June.

## Open Questions

- How to detect "active" vs "done"? File modification time? Parse for completion message?
- Should we watch all projects or just current git project?
- How to handle very long transcripts? (Pagination? Lazy loading?)

## Next Steps

1. Gut June's TUI to remove unneeded features (SQLite, messaging, spawning)
2. Write the Claude Code file parser (`internal/claude/`)
3. Wire up new data source
4. Test with real subagent sessions
