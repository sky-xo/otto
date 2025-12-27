# Unified Chat Stream & Focus Model Design

**Status:** Ready
**Date:** 2025-12-27

## Overview

Redesign the TUI right panel to be a unified chat stream (Slack/Discord style) with two-focus keyboard handling. This replaces the original P4 plan (Activity Feed + Chat Split) with a simpler, chat-centric approach.

## Goals

1. Fix keyboard shortcuts capturing keys when chat input is focused (can't type g, j, k, etc.)
2. Simplify the right panel to a unified stream instead of separate activity/chat panels
3. Make messages look like a chat conversation, not a log of system events

## Design

### Layout & Focus Model

Two focus targets, Tab toggles between them:

```
┌────────────┬─────────────────────────────────┐
│            │                                 │
│  Sidebar   │  Content (mouse scroll)         │
│  [focus 1] │                                 │
│            ├─────────────────────────────────┤
│            │  Chat input [focus 2]           │
└────────────┴─────────────────────────────────┘
```

**Focus behavior:**

| Focus | Keys |
|-------|------|
| Sidebar | j/k/↑/↓ navigate, Enter select/expand, Tab → right panel |
| Right panel | All keys → chat input, Esc → sidebar, Tab → sidebar |

**Key insight:** When right panel is focused, chat input captures everything. Content scrolling is mouse-only (no keyboard shortcuts needed).

**Visual indicators:**
- Focused panel gets highlighted border
- Chat cursor visible when right panel focused

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

### Phase 1: Two-Focus Keyboard Model

#### Task 1.1: Right panel sends all keys to chat input

Files: Modify `internal/tui/watch.go`; Test `internal/tui/watch_test.go`

Step 1: Write failing test (with code)
```go
func TestRightPanelRoutesKeysToInput(t *testing.T) {
	db := newTestDB(t)
	m := NewModel(db)
	m.focusedPanel = panelMessages

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	next, _ := m.Update(msg)
	model := next.(Model)

	if model.chatInput.Value() != "j" {
		t.Fatalf("expected chat input to capture key, got %q", model.chatInput.Value())
	}
}
```

Step 2: Run test, expect FAIL
```sh
go test ./internal/tui -run TestRightPanelRoutesKeysToInput
```

Step 3: Write implementation (with code)
```go
case panelMessages:
	switch key := msg.(type) {
	case tea.KeyMsg:
		if key.Type != tea.KeyEsc && key.Type != tea.KeyTab {
			m.chatInput, cmd = m.chatInput.Update(key)
			return m, cmd
		}
	}
```

Step 4: Run test, expect PASS
```sh
go test ./internal/tui -run TestRightPanelRoutesKeysToInput
```

Step 5: Commit
```sh
git commit -am "tui: route right-panel keys to chat input"
```

#### Task 1.2: Esc/Tab from right panel returns to sidebar

Files: Modify `internal/tui/watch.go`; Test `internal/tui/watch_test.go`

Step 1: Write failing test (with code)
```go
func TestRightPanelEscReturnsToSidebar(t *testing.T) {
	db := newTestDB(t)
	m := NewModel(db)
	m.focusedPanel = panelMessages

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := next.(Model)

	if model.focusedPanel != panelAgents {
		t.Fatalf("expected sidebar focus, got %v", model.focusedPanel)
	}
}
```

Step 2: Run test, expect FAIL
```sh
go test ./internal/tui -run TestRightPanelEscReturnsToSidebar
```

Step 3: Write implementation (with code)
```go
if key.Type == tea.KeyEsc || key.Type == tea.KeyTab {
	m.focusedPanel = panelAgents
	return m, nil
}
```

Step 4: Run test, expect PASS
```sh
go test ./internal/tui -run TestRightPanelEscReturnsToSidebar
```

Step 5: Commit
```sh
git commit -am "tui: allow esc/tab to return focus to sidebar"
```

#### Task 1.3: Remove keyboard scrolling from right panel

Files: Modify `internal/tui/watch.go`; Test `internal/tui/watch_test.go`

Step 1: Write failing test (with code)
```go
func TestRightPanelIgnoresScrollKeys(t *testing.T) {
	db := newTestDB(t)
	m := NewModel(db)
	m.focusedPanel = panelMessages
	m.viewport.YPosition = 5

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model := next.(Model)

	if model.viewport.YPosition != 5 {
		t.Fatalf("expected no keyboard scroll in right panel")
	}
}
```

Step 2: Run test, expect FAIL
```sh
go test ./internal/tui -run TestRightPanelIgnoresScrollKeys
```

Step 3: Write implementation (with code)
```go
// Remove j/k/g/G handlers when focusedPanel == panelMessages.
// Keep viewport scrolling only in mouse handlers.
```

Step 4: Run test, expect PASS
```sh
go test ./internal/tui -run TestRightPanelIgnoresScrollKeys
```

Step 5: Commit
```sh
git commit -am "tui: disable keyboard scrolling in right panel"
```

### Phase 2: User Message Storage

#### Task 2.1: Add chat message type constant

Files: Modify `internal/repo/messages.go`; Test `internal/repo/messages_test.go`

Step 1: Write failing test (with code)
```go
func TestMessageTypeChatConstant(t *testing.T) {
	if MessageTypeChat == "" {
		t.Fatal("expected MessageTypeChat to be defined")
	}
}
```

Step 2: Run test, expect FAIL
```sh
go test ./internal/repo -run TestMessageTypeChatConstant
```

Step 3: Write implementation (with code)
```go
const MessageTypeChat = "chat"
```

Step 4: Run test, expect PASS
```sh
go test ./internal/repo -run TestMessageTypeChatConstant
```

Step 5: Commit
```sh
git commit -am "repo: add chat message type"
```

#### Task 2.2: Store user chat message before spawn

Files: Modify `internal/cli/commands/watch.go`; Test `internal/cli/commands/watch_test.go`

Step 1: Write failing test (with code)
```go
func TestWatchStoresChatMessageBeforeSpawn(t *testing.T) {
	db := newTestDB(t)
	err := runWatchOnce(db, "hello")
	if err != nil {
		t.Fatalf("runWatchOnce: %v", err)
	}
	msgs := mustListMessages(t, db)
	if msgs[0].Type != repo.MessageTypeChat || msgs[0].Content != "hello" {
		t.Fatalf("expected chat message first, got %#v", msgs[0])
	}
}
```

Step 2: Run test, expect FAIL
```sh
go test ./internal/cli/commands -run TestWatchStoresChatMessageBeforeSpawn
```

Step 3: Write implementation (with code)
```go
msg := repo.Message{
	FromAgent: "you",
	ToAgent:   "otto",
	Type:      repo.MessageTypeChat,
	Content:   userInput,
}
if err := repo.CreateMessage(db, msg); err != nil {
	return err
}
```

Step 4: Run test, expect PASS
```sh
go test ./internal/cli/commands -run TestWatchStoresChatMessageBeforeSpawn
```

Step 5: Commit
```sh
git commit -am "watch: store chat message before spawn"
```

### Phase 3: Message Rendering

#### Task 3.1: Render chat and otto completions as Slack-style blocks

Files: Modify `internal/tui/watch.go`; Test `internal/tui/watch_test.go`

Step 1: Write failing test (with code)
```go
func TestFormatMessageChatBlock(t *testing.T) {
	msg := repo.Message{
		FromAgent: "you",
		Type:      repo.MessageTypeChat,
		Content:   "hey there",
	}
	lines := formatMessage(msg)
	got := strings.Join(lines, "\n")
	want := "you\nhey there\n"
	if !strings.Contains(got, want) {
		t.Fatalf("expected chat block, got:\n%s", got)
	}
}
```

Step 2: Run test, expect FAIL
```sh
go test ./internal/tui -run TestFormatMessageChatBlock
```

Step 3: Write implementation (with code)
```go
case repo.MessageTypeChat:
	return []string{msg.FromAgent, msg.Content, ""}
case repo.MessageTypeComplete:
	if msg.FromAgent == "otto" {
		return []string{"otto", msg.Content, ""}
	}
```

Step 4: Run test, expect PASS
```sh
go test ./internal/tui -run TestFormatMessageChatBlock
```

Step 5: Commit
```sh
git commit -am "tui: render chat and otto completion blocks"
```

#### Task 3.2: Render activity lines and hide noise

Files: Modify `internal/tui/watch.go`; Test `internal/tui/watch_test.go`

Step 1: Write failing test (with code)
```go
func TestFormatMessageHidesPromptToOtto(t *testing.T) {
	msg := repo.Message{FromAgent: "orchestrator", ToAgent: "otto", Type: repo.MessageTypePrompt}
	lines := formatMessage(msg)
	if len(lines) != 0 {
		t.Fatalf("expected prompt-to-otto to be hidden")
	}
}
```

Step 2: Run test, expect FAIL
```sh
go test ./internal/tui -run TestFormatMessageHidesPromptToOtto
```

Step 3: Write implementation (with code)
```go
case repo.MessageTypePrompt:
	if msg.ToAgent == "otto" {
		return nil
	}
	return []string{fmt.Sprintf("%s spawned %s — %q", msg.FromAgent, msg.ToAgent, msg.Content), ""}
case repo.MessageTypeExit:
	return nil
```

Step 4: Run test, expect PASS
```sh
go test ./internal/tui -run TestFormatMessageHidesPromptToOtto
```

Step 5: Commit
```sh
git commit -am "tui: render activity lines and hide noise"
```

### Phase 4: Polish

#### Task 4.1: Apply color styles for chat and activity lines

Files: Modify `internal/tui/watch.go`; Test `internal/tui/watch_test.go`

Step 1: Write failing test (with code)
```go
func TestMessageStyles(t *testing.T) {
	if messageStyle("otto") == "" {
		t.Fatal("expected non-empty style for otto")
	}
}
```

Step 2: Run test, expect FAIL
```sh
go test ./internal/tui -run TestMessageStyles
```

Step 3: Write implementation (with code)
```go
var (
	styleYou  = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	styleOtto = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	styleDim  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)
```

Step 4: Run test, expect PASS
```sh
go test ./internal/tui -run TestMessageStyles
```

Step 5: Commit
```sh
git commit -am "tui: add message color styles"
```

#### Task 4.2: Improve word wrapping for chat blocks

Files: Modify `internal/tui/watch.go`; Test `internal/tui/watch_test.go`

Step 1: Write failing test (with code)
```go
func TestChatWrapsToViewportWidth(t *testing.T) {
	m := NewModel(nil)
	m.width = 40
	lines := wrapChat("otto", strings.Repeat("a", 120), m.width)
	for _, line := range lines {
		if len(line) > 40 {
			t.Fatalf("expected wrapped line, got %q", line)
		}
	}
}
```

Step 2: Run test, expect FAIL
```sh
go test ./internal/tui -run TestChatWrapsToViewportWidth
```

Step 3: Write implementation (with code)
```go
func wrapChat(name, body string, width int) []string {
	return append([]string{name}, wordwrap.String(body, width))
}
```

Step 4: Run test, expect PASS
```sh
go test ./internal/tui -run TestChatWrapsToViewportWidth
```

Step 5: Commit
```sh
git commit -am "tui: wrap chat blocks to viewport width"
```

#### Task 4.3: Scroll to bottom on new messages

Files: Modify `internal/tui/watch.go`; Test `internal/tui/watch_test.go`

Step 1: Write failing test (with code)
```go
func TestScrollToBottomOnNewMessages(t *testing.T) {
	m := NewModel(nil)
	m.viewport.Height = 10
	m.messages = []repo.Message{{Content: "a"}, {Content: "b"}}
	m.viewport.YPosition = 0

	m = m.refreshViewport()
	if m.viewport.YPosition == 0 {
		t.Fatal("expected viewport to scroll to bottom")
	}
}
```

Step 2: Run test, expect FAIL
```sh
go test ./internal/tui -run TestScrollToBottomOnNewMessages
```

Step 3: Write implementation (with code)
```go
if m.viewport.YPosition < m.viewport.ScrollPercent()*float64(m.viewport.TotalLineCount()) {
	m.viewport.GotoBottom()
}
```

Step 4: Run test, expect PASS
```sh
go test ./internal/tui -run TestScrollToBottomOnNewMessages
```

Step 5: Commit
```sh
git commit -am "tui: scroll to bottom on new messages"
```

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

1. **Three-focus model (agents/viewport/chat)** — Rejected: two Tabs to reach chat from sidebar, overengineered
2. **Modifier shortcuts (Ctrl+j/k for viewport scroll)** — Rejected: not needed, mouse scroll is fine
3. **Vim-style modes (i to type, Esc to exit)** — Rejected: confusing for non-Vim users
4. **Three panels on right (activity/chat/input)** — Rejected: too cluttered

## Not In Scope

- Keyboard scrolling for content viewport (mouse is sufficient)
- Full agent transcripts in stream (click agent in sidebar)
- Expandable activity lines
- Timestamps
- Message search/filtering
