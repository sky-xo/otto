# Unified Chat Stream & Focus Model Design

**Status:** Ready
**Date:** 2025-12-27

## Overview

Redesign the TUI right panel to be a unified chat stream (Slack/Discord style) with three-focus keyboard handling. This replaces the original P4 plan (Activity Feed + Chat Split) with a simpler, chat-centric approach.

## Goals

1. Fix keyboard shortcuts capturing keys when chat input is focused (can't type g, j, k, etc.)
2. Simplify the right panel to a unified stream instead of separate activity/chat panels
3. Make messages look like a chat conversation, not a log of system events

## Design

### Layout & Focus Model

Three distinct focus targets, Tab cycles between them:

```
Tab →  Tab →  Tab →
  ↓      ↓      ↓
┌────────────┬─────────────────────────────────┐
│            │                                 │
│  Agents    │  Unified Stream                 │
│  [focus 1] │  [focus 2]                      │
│            │                                 │
│            ├─────────────────────────────────┤
│            │  Chat input  [focus 3]          │
└────────────┴─────────────────────────────────┘
```

**Focus behavior:**

| Focus | Area | Keys |
|-------|------|------|
| 1 | Agents panel | j/k navigate, Enter expand/collapse, g/G top/bottom |
| 2 | Stream viewport | j/k/g/G scroll, Enter on activity → select agent |
| 3 | Chat input | ALL keys go to textinput, only Esc/Tab escape |

**Key insight:** When chat input has focus, it captures everything. No shortcuts leak through.

**Visual indicators:**
- Focused area gets highlighted border
- Chat cursor only visible when focus 3 active

### Message Formatting

**Chat messages (Slack/Discord style):**

```
you
hey there, can you hear me?

otto
Confirmed I can hear you. Here's my analysis
of the keyboard handling problem...

you
What about the viewport?

otto
Good point. For the viewport we could...
```

- Name on its own line (bold/colored)
- Message body below, word-wrapped
- Blank line after every item (chat or activity) for consistent spacing
- Full horizontal width for message content

**Activity notifications (compact, single line):**

```
otto spawned brainstorm-keyboard — "Brainstorm solutions..."
brainstorm-keyboard completed
otto spawned impl-keyboard — "Implement three-focus..."
```

Format: `{agent} {action} {target} — "{truncated prompt}"`

Activity is dimmed/muted styling to distinguish from chat.

### What Shows in the Stream

| Type | Format | Example |
|------|--------|---------|
| User message | `you` + body | User's chat input |
| Otto response | `otto` + body | Otto's replies |
| Agent spawn | single line | `otto spawned foo — "prompt..."` |
| Agent complete | single line | `foo completed` |
| Agent message | `foo` + body | If sub-agents send messages back |

### What's Hidden (Noise)

- `exited - process completed successfully` — implementation detail
- `orchestrator spawned otto` — user message implies this
- Internal state changes that don't affect the user

### Color Scheme (Tentative)

| Element | Color |
|---------|-------|
| `you` | white/default |
| `otto` | blue |
| sub-agents | green |
| activity lines | dim/gray |
| timestamps | omit for now |

## Implementation Notes

### Current State Issues

The current display shows:
```
orchestrator @otto spawned otto - hey there, can you hear me?
otto completed - Confirmed I can hear you; no actions needed.
otto exited - process completed successfully
```

Problems:
1. User message shown as "orchestrator spawned" event
2. Otto's response shown as "completed" event
3. "exited" noise visible
4. "orchestrator" label is confusing

### Data Model Analysis

**Messages table schema:**
```sql
CREATE TABLE messages (
  id TEXT PRIMARY KEY,
  project TEXT NOT NULL,
  branch TEXT NOT NULL,
  from_agent TEXT NOT NULL,    -- who sent it
  to_agent TEXT,               -- who it's for
  type TEXT NOT NULL,          -- message type
  content TEXT NOT NULL,       -- the actual content
  mentions TEXT,
  ...
);
```

**Current message types:**
- `prompt` — message to an agent (spawn or follow-up)
- `say` — agent-to-agent message
- `complete` — agent completion with result
- `exit` — agent process exited
- `error` — error message

**Current problem flow:**

1. User types "hey there" in TUI chat
2. TUI runs `otto spawn codex "hey there" --name otto`
3. spawn.go line 111 creates summary: `"spawned otto - hey there"`
4. storePrompt() creates message:
   - `from_agent: "orchestrator"`
   - `to_agent: "otto"`
   - `type: "prompt"`
   - `content: "spawned otto - hey there"`
5. TUI renders: `orchestrator @otto spawned otto - hey there`

**Proposed changes:**

1. **Add new message type `chat`** for user-initiated messages
2. **TUI stores user message directly** before calling spawn/prompt:
   - `from_agent: "you"` (or "user")
   - `to_agent: "otto"`
   - `type: "chat"`
   - `content: "hey there"` (raw message, no "spawned" prefix)
3. **Spawn still creates its internal `prompt` message** but with different from_agent:
   - `from_agent: "otto"` (when otto spawns sub-agents)
   - or hide entirely when spawning otto itself
4. **TUI rendering logic:**
   - `type: "chat"` → Slack-style user message
   - `type: "prompt"` from otto → activity line "otto spawned foo"
   - `type: "prompt"` from orchestrator to otto → hide (redundant)
   - `type: "complete"` → depends on who:
     - otto → Slack-style response
     - sub-agents → activity line "foo completed"
   - `type: "exit"` → hide

**Decision:** Go with the proper fix (add `chat` type, TUI stores user message directly).

## Implementation Plan

### Phase 1: Three-Focus Keyboard Model

1. Add `focusedArea` enum: `areaAgents`, `areaContent`, `areaChat`
2. Update Tab handling to cycle through all three
3. When `focusedArea == areaChat`: route ALL keys to textinput except Esc/Tab
4. Update visual indicators (border highlighting)
5. Tests for focus cycling and key capture

### Phase 2: User Message Storage

1. Add `chat` message type constant
2. Before spawn/prompt, TUI stores message:
   ```go
   msg := repo.Message{
       FromAgent: "you",
       ToAgent:   "otto",
       Type:      "chat",
       Content:   userInput,  // raw message, no "spawned" prefix
   }
   repo.CreateMessage(db, msg)
   ```
3. Then trigger spawn/prompt in background (existing code)

### Phase 3: Message Rendering

1. Update `formatMessage()` and `mainContentLines()` for new display:
   - `type: "chat"` → Slack-style user message
   - `type: "complete"` from otto → Slack-style otto response
   - `type: "prompt"` from otto to sub-agent → activity line
   - `type: "exit"` → hide
   - `type: "prompt"` to otto → hide (redundant with chat message)
2. Add blank line after every item (chat or activity)
3. Name on own line, message body below

### Phase 4: Polish

1. Color scheme (you=white, otto=blue, sub-agents=green, activity=dim)
2. Word wrapping improvements
3. Scroll-to-bottom on new messages

## Review Decisions

Based on Codex review feedback and discussion:

### 1. Spawn Failure Visibility

**Problem:** If spawn fails after storing the `chat` message, user sees their message but no error.

**Decision:** Show an error line in the stream:
```
you
hey there

⚠ Failed to start otto: exec: "codex": executable file not found
```

### 2. Two-Phase Completion ("finishing" status)

**Problem:** Agent calls `otto complete` but process continues outputting for 10-30 more seconds. Status shows "complete" while still talking.

**Decision:** Add `finishing` status:
- `otto complete` sets status to `finishing` (not `complete`)
- Status changes to `complete` on process exit
- `finishing` blocks new input (like `busy`)
- Visual: ● gray filled (between green busy and hollow complete)

**Status indicators:**
| Status | Symbol | Color |
|--------|--------|-------|
| `busy` | ● | Green |
| `finishing` | ● | Gray |
| `complete` | ○ | Gray |
| `failed` | ✗ | Red |

### 3. `complete` Message Content

**Confirmed:** The `complete` message content IS the agent's response (whatever they pass to `otto complete "..."`). Render otto's completions as Slack-style chat messages, not activity lines.

## Alternatives Considered

1. **Modifier shortcuts (Ctrl+g, Ctrl+j)** — Rejected: less ergonomic, terminal conflicts
2. **Vim-style modes (i to type, Esc to exit)** — Rejected: confusing for non-Vim users
3. **Three panels on right (activity/chat/input)** — Rejected: too cluttered
4. **Input-first routing (2 focus targets)** — Considered: simpler but less explicit

## Not In Scope

- Full agent transcripts in stream (click agent in sidebar)
- Expandable activity lines
- Timestamps
- Message search/filtering
