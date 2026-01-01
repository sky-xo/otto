# TUI Panel Focus System Design

**Status:** Completed

## Goal

Implement focus-based panel navigation for `june watch --ui` that:
1. Works like tmux/vim splits - focused panel receives scroll input
2. Supports future 3-panel layout (agents, messages, todos)

## Current State (Partial)

Already changed in `internal/tui/watch.go`:
- Added constants: `focusLeft`, `focusRight`
- Added `focusedPanel` field to model struct
- Initialized to `focusRight` in NewModel
- Added `tea.WithMouseCellMotion()` to enable mouse events
- Partially updated Update() for focus-based key routing (incomplete)

## Design

### Panel Names (Future-Proof)

Use numeric panel indices instead of left/right to support 3+ panels:
```go
const (
    panelAgents   = 0  // Left: channel/agent list
    panelMessages = 1  // Middle/Right: content/messages
    panelTodos    = 2  // Future: todo list
)
focusedPanel int  // Index of focused panel
```

### Focus Switching

1. **Tab**: Cycle focus to next panel (wraps around)
2. **Shift+Tab** (optional): Cycle backwards
3. **Mouse click**: Focus panel that was clicked
4. **Number keys** (optional): 1/2/3 to focus specific panel

### Input Routing

When panel has focus, it receives:
- j/k: Scroll up/down
- Arrows: Navigate items (or scroll)
- g/G: Go to top/bjunem
- Page up/down
- Mouse wheel scroll

### Visual Indicator

Focused panel gets highlighted border:
```go
focusedBorderStyle = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("6"))  // Cyan

unfocusedBorderStyle = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("8"))  // Dim
```

### Mouse Click Detection

Calculate panel boundaries from layout():
```go
case tea.MouseMsg:
    if msg.Action == tea.MouseActionPress {
        leftWidth, _, _, _ := m.layout()
        if msg.X < leftWidth + 2 {
            m.focusedPanel = panelAgents
        } else {
            m.focusedPanel = panelMessages
        }
    }
```

### Status Bar Update

Show which panel is focused and relevant keybindings:
```
Tab: switch panel | [Agents] j/k: scroll | Enter: select | q: quit
```

## Implementation Tasks

1. **Rename constants**: `focusLeft`/`focusRight` -> `panelAgents`/`panelMessages` (int)
2. **Fix Update()**: Complete focus-based input routing
3. **Add visual indicator**: Different border color for focused panel
4. **Update status bar**: Show focus state and relevant keys
5. **Test mouse click**: Verify click-to-focus works
6. **Test mouse scroll**: Verify scroll goes to focused panel

## Files

- `internal/tui/watch.go` - Main implementation

## Testing

```bash
make build && ./june watch --ui
```

Verify:
- Tab switches focus between panels
- Clicking panel focuses it
- j/k scrolls focused panel only
- Mouse wheel scrolls focused panel
- Visual indicator shows which panel is focused
