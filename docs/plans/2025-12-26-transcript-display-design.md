# Transcript Display Design

**Status:** Approved
**Date:** 2025-12-26

## Problem

When viewing Codex agent transcripts in `june watch`, raw JSON is displayed instead of nicely formatted content. This happens because:

1. `consumeTranscriptEntries()` logs every raw stdout chunk as `EventType: "output"` containing raw JSON
2. `onEvent()` callback also creates properly parsed event entries
3. TUI displays both, showing the JSON noise

## Solution

1. Stop double-logging for Codex agents
2. Update TUI prefix symbols to match Claude Code style
3. Add distinct styling for different event types

## Event Type Display Mapping

| EventType | Prefix | Style |
|-----------|--------|-------|
| `input` | `>` | Background color (muted) |
| `reasoning`, `thinking` | `∴` | Dim/muted text |
| `command_execution` | `$` | Normal text |
| `tool_call` | `ƒ` | Normal text |
| `tool_result` | (none) | Indented, aligned with content above |
| `agent_message`, `message` | `⏺` | Normal text |
| Failed results | `!` | Red/error style |

## Example Render

```
> Build the authentication module
∴ Let me start by examining the codebase...
ƒ Read file src/auth/auth.ts
  [file contents...]
$ npm test
  All tests passed
⏺ I've added the authentication handler.
```

## Files to Modify

### 1. `internal/tui/watch.go`

Update `transcriptPrefix()` function:

```go
func transcriptPrefix(entry repo.LogEntry) (string, lipgloss.Style) {
    switch entry.EventType {
    case "input":
        return ">", inputStyle  // new style with background
    case "reasoning", "thinking":
        return "∴", mutedStyle
    case "command_execution":
        return "$", messageStyle
    case "tool_call":
        return "ƒ", messageStyle
    case "tool_result":
        if entry.Status.Valid && entry.Status.String == "failed" {
            return "!", statusFailedStyle
        }
        return "", messageStyle  // no prefix, content aligns
    case "agent_message", "message":
        return "⏺", messageStyle
    default:
        return "∴", mutedStyle
    }
}
```

Add new `inputStyle` with background color.

### 2. `internal/cli/commands/transcript_capture.go`

Modify `consumeTranscriptEntries()` to skip raw logging when `onEvent` is provided:

```go
func consumeTranscriptEntries(db *sql.DB, ctx scope.Context, agentID string, output <-chan juneexec.TranscriptChunk, onEvent func(CodexEvent)) <-chan error {
    // ...
    for chunk := range output {
        // Only log raw output if no event parser is provided (Claude agents)
        if onEvent == nil {
            entry := repo.LogEntry{
                // ... create raw output entry
            }
            if err := repo.CreateLogEntry(db, entry); err != nil {
                done <- err
                return
            }
        }

        // Parse events for Codex
        if onEvent != nil && chunk.Stream == "stdout" {
            // ... existing event parsing logic
        }
    }
    // ...
}
```

### 3. `internal/tui/formatting.go` (optional)

Either integrate `FormatLogEntry()` or remove it - currently unused.

## Future Extensions

- When humans can interact with subagents directly, use `>` for human prompts
- Add names (like chat) when multi-person interaction is added: `alice:`, `orchestrator:`
