# TUI Transcript: Replace-on-Complete Design

**Status:** Draft
**Date:** 2025-12-28

## Overview

Redesign the transcript view (individual agent view) to show ephemeral work status that replaces in-place, then collapses when the turn completes - leaving only durable output visible.

Currently, the transcript shows every intermediate step as permanent entries (thinking, commands, more thinking, more commands). This creates noise and makes it hard to see what actually matters: what the agent said.

## Goals

1. Reduce visual noise in transcript view
2. Show live "working" status with animation
3. Preserve important output while hiding ephemeral work details
4. Keep failed commands visible for debugging

## Design

### Mental Model: Ephemeral vs Durable

**Ephemeral (replaces in-place, then vanishes):**
- Thinking/reasoning events
- Command execution (unless failed)

**Durable (persists in transcript):**
- Input (`>` prefix)
- `june say` messages - agent status updates
- Final output (`agent_message`)
- Failed commands (debugging breadcrumb)

### Turn Lifecycle

A "turn" is the agent responding to input:
- **Turn starts:** new `input` event
- **Turn ends:** `agent_message`/`message` event (the output)
- **Everything between:** ephemeral status that replaces in-place

### Visual States

**During a turn (agent working):**
```
> Help me understand the codebase       <- durable input

[durable] Starting to explore...        <- june say (preserved)

[spinner] Checking project structure... <- ephemeral (thinking + shimmer)
$ find cmd -maxdepth 2 -type f          <- ephemeral (command)
```

**After turn completes:**
```
> Help me understand the codebase       <- preserved

[durable] Starting to explore...        <- preserved (june say)

[durable] Here's what I found...        <- preserved (final output)
```

The thinking and command lines disappear. Only durable content remains.

### Animation: Spinner + Shimmer

**Spinner:**
- Bubbletea's spinner component (braille dots: `[chars]`)
- Ticks every ~80ms for smooth animation
- Positioned before the thinking text

**Shimmer effect:**
- Wave of brightness moving left-to-right through thinking text
- Cycle between dim gray -> white -> dim gray
- Wave width: ~5-8 characters
- **Duration:** ~1.5 seconds to traverse full text
- **Frequency:** Every ~4 seconds (intermittent, not continuous)
- Pattern: `wave (1.5s) -> static (4s) -> wave (1.5s) -> ...`

The spinner provides constant "working" feedback. The shimmer adds periodic visual interest without being distracting.

**Command line (no animation):**
- `$` prefix in cyan
- Command text in normal brightness, static
- Truncate long commands with `...`

### Ephemeral Status Block

Always exactly 2 lines maximum:
```
[spinner] [thinking text with shimmer]
$ [current command]
```

- Thinking replaces previous thinking
- Command replaces previous command
- Block appears below last durable entry
- Block vanishes when turn completes

### Failed Commands

If a command fails (`tool_result` with `status: failed`), it becomes durable instead of ephemeral:
```
! find nonexistent -type f              <- persists (red styling)
```

This gives debugging breadcrumbs when something goes wrong.

### Event Type Classification

| Event Type | Category | Styling |
|------------|----------|---------|
| `input` | Durable | Full background highlight |
| `reasoning`/`thinking` | Ephemeral | Spinner + shimmer |
| `command_execution` | Ephemeral | `$` prefix, static |
| `tool_call` | Ephemeral | Same as command |
| `tool_result` (success) | Ephemeral | Hidden |
| `tool_result` (failed) | Durable | `!` prefix, red |
| `agent_message`/`message` | Durable | Normal text |
| `say` (june say) | Durable | Normal text |

## Implementation Notes

### State to Track

In `model` struct:
- `currentTurnStart`: timestamp or entry ID of current turn start
- `currentThinking`: latest thinking text (for ephemeral display)
- `currentCommand`: latest command text (for ephemeral display)
- `shimmerOffset`: current position in shimmer wave
- `shimmerActive`: whether shimmer wave is currently animating

### Rendering Logic

`transcriptContentLines()` changes:
1. Iterate through entries
2. For entries before `currentTurnStart`: render normally (all durable)
3. For entries in current turn:
   - `input`: render (durable)
   - `say`/`agent_message`: render (durable, also ends turn)
   - `thinking`/`command`: update ephemeral state, don't render
   - Failed `tool_result`: render (durable exception)
4. After all entries: if turn is active, render ephemeral status block

### Animation Tick

Add tick message for animation updates:
- Spinner: ~80ms tick
- Shimmer: track time since last wave, trigger wave every 4s
- During wave: update shimmerOffset each tick for 1.5s

### Shimmer Implementation

```go
func shimmerText(text string, offset int, waveWidth int) string {
    // For each character, calculate brightness based on distance from offset
    // Characters within waveWidth of offset get progressively brighter
    // Peak brightness at offset position
}
```

Use ANSI 256-color: dim gray (240) -> white (255) -> dim gray (240)

## Scope

**In scope:**
- Transcript view only (individual agent view)
- Replace-on-complete behavior
- Spinner animation
- Shimmer animation
- Failed command persistence

**Out of scope:**
- Main message stream (separate design: unified-chat-stream)
- Expandable history (click to see what happened)
- Timestamps

## Alternatives Considered

1. **Keep all entries, just dim old ones** - Still cluttered
2. **Collapsible sections per turn** - More complex, requires interaction
3. **Continuous shimmer** - Too distracting, settled on intermittent

## Open Questions

1. Should there be a way to expand/see the collapsed history?
2. What happens if agent produces multiple `june say` messages rapidly?
3. Should command output (stdout) be shown anywhere?
