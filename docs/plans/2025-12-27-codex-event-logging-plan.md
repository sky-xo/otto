# Codex Event Logging Enhancement

**Status:** Ready
**Date:** 2025-12-27

## Problem

When Claude monitors Codex agents spawned via Otto, it lacks visibility into what the agent is currently doing. The only logged events are `item.completed`, meaning Claude doesn't know an action has started until it finishes.

Additionally, repeated polling of `BashOutput` burns context because:
1. BashOutput appears to return cumulative output (not incremental)
2. Raw Codex JSON is verbose

## Solution

Log `item.started` and turn lifecycle events to provide real-time visibility.

> **Note:** The otto-orchestrate skill already warns against BashOutput polling and recommends `otto status`/`otto peek`. No skill updates needed.

## Implementation

### Task 1: Add `item.started` event logging

**Files:** `internal/cli/commands/spawn.go`, `internal/cli/commands/prompt.go`, `internal/cli/commands/worker_spawn.go`

**Change:** In each `onEvent` handler, add logging for `item.started`:

```go
if event.Type == "item.started" && event.Item != nil {
    // Use Command as Content fallback to avoid blank transcript lines
    content := event.Item.Text
    if content == "" && event.Item.Command != "" {
        content = event.Item.Command
    }
    logEntry := repo.LogEntry{
        Project:   ctx.Project,
        Branch:    ctx.Branch,
        AgentName: agentID,
        AgentType: "codex",
        EventType: "item.started",
        Command:   sql.NullString{String: event.Item.Command, Valid: event.Item.Command != ""},
        Content:   sql.NullString{String: content, Valid: content != ""},
    }
    _ = repo.CreateLogEntry(db, logEntry)
}
```

**Tests:**
- `spawn_test.go` - verify `item.started` events are logged
- `prompt_test.go` - verify logging in `runCodexPrompt`
- `worker_spawn_test.go` - verify logging in `runWorkerCodexSpawn`

**Complexity:** Small

### Task 2: Add turn lifecycle event logging

**Files:** Same as Task 1

**Change:** Add logging for `turn.started` and `turn.completed`:

```go
if event.Type == "turn.started" || event.Type == "turn.completed" {
    logEntry := repo.LogEntry{
        Project:   ctx.Project,
        Branch:    ctx.Branch,
        AgentName: agentID,
        AgentType: "codex",
        EventType: event.Type,
    }
    _ = repo.CreateLogEntry(db, logEntry)
}
```

**Tests:** Add test cases in spawn_test.go, prompt_test.go, worker_spawn_test.go to verify turn events are logged.

**Complexity:** Small

### Task 3: Update `otto peek` output format

**Files:** `internal/cli/commands/peek.go`

**Change:** Format `item.started` and turn events distinctly:

```go
if entry.EventType == "item.started" {
    if entry.Command.Valid && entry.Command.String != "" {
        fmt.Fprintf(w, "[running] %s\n", entry.Command.String)
    } else if entry.Content.Valid && entry.Content.String != "" {
        fmt.Fprintf(w, "[starting] %s\n", entry.Content.String)
    }
    // Skip if both empty (edge case - shouldn't happen with Task 1 fix)
    continue
}
if entry.EventType == "turn.started" {
    fmt.Fprintf(w, "--- turn started ---\n")
    continue
}
if entry.EventType == "turn.completed" {
    fmt.Fprintf(w, "--- turn completed ---\n")
    continue
}
```

**Tests:** Add `peek_test.go` tests for new event type formatting.

**Complexity:** Small

## Expected Outcome

After implementation, `otto peek` output will show:

```
--- turn started ---
[running] rg -n "foo" .
<search results>
[running] cat src/file.go
<file contents>
**Thinking about the problem**
Here's what I found...
--- turn completed ---
```

This gives Claude visibility into what the agent is doing without the context burn of raw JSON via BashOutput.

## Testing

```bash
go test ./internal/cli/commands -run "TestCodex.*Started|TestCodex.*Turn|TestPeek"
```

## Future Work (TUI)

The TUI (`internal/tui/watch.go`) reads `entry.Content` for display. The new event types will currently render with their content (if set) or be blank. If we want special rendering (e.g., `[running]` prefix, turn separators) in the TUI, that's a separate enhancement to coordinate with the unified chat stream work.

## Dependencies

None - this is a standalone enhancement.
