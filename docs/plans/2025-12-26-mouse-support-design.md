# Mouse Support Design

**Status:** Approved
**Date:** 2025-12-26

## Overview

Add mouse support to the June TUI for intuitive navigation and text selection.

## Phases

### Phase 1: Scroll + Click

**Mouse wheel scrolling:**
- Scroll affects the panel the mouse is hovering over
- Hovering over content panel + scroll → scrolls content
- Hovering over agent list + scroll → nothing (for now)

**Click to select agent:**
- Clicking on an agent in the left panel selects it
- Transcript updates automatically (existing auto-select behavior)

### Phase 2: Text Selection

**Entering selection mode:**
- Click-and-drag in content panel auto-enters selection mode
- No hotkey required - drag detection triggers it

**Visual indicators:**
- Content panel header shows: `Selection Mode - C: Copy | Esc: Cancel`
- Selected text is highlighted (inverted colors or background)

**Exiting selection mode:**
- `C` → copy selected text to clipboard, exit mode
- `Esc` → exit mode without copying

## Implementation Details

### Phase 1

**Re-enable mouse in Run():**
```go
p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
```

**Mouse state in model:**
```go
type model struct {
    // existing fields...
    mouseX, mouseY int  // track position for hover detection
}
```

**Mouse event handling:**
```go
case tea.MouseMsg:
    m.mouseX, m.mouseY = msg.X, msg.Y
    leftWidth, _, _, _ := m.layout()
    inContentPanel := msg.X > leftWidth

    // Scroll - only in content panel
    if inContentPanel {
        switch msg.Button {
        case tea.MouseButtonWheelUp:
            m.viewport.LineUp(3)
        case tea.MouseButtonWheelDown:
            m.viewport.LineDown(3)
        }
    }

    // Click - select agent
    if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionRelease {
        if !inContentPanel {
            // Calculate clicked agent from Y position
            clickedIndex := msg.Y - 2  // subtract border + title
            if clickedIndex >= 0 && clickedIndex < len(channels) {
                m.cursorIndex = clickedIndex
                m.activateSelection()
            }
        }
    }
```

### Phase 2

**Additional state:**
```go
type model struct {
    // existing fields...
    selectionMode   bool
    dragStartX      int
    dragStartY      int
    selectionStartY int
    selectionEndY   int
}
```

**Drag detection:**
- On `MouseActionPress` in content panel → save start position
- On `MouseMotionMsg` while button held → enter selection mode, update selection range
- On `MouseActionRelease` → selection complete

**Key handling in selection mode:**
- `C` → copy selected lines to clipboard, exit mode
- `Esc` → exit mode without copying

**Research before implementing:**
- Review mprocs source for selection highlighting patterns
- Review process-control for UI inspiration

## Files to Modify

- `internal/tui/watch.go` - mouse handling, selection state, rendering
