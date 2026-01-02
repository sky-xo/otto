# Selection Mode Design

## Overview

Add text selection capability to the transcript/content panel, allowing users to select and copy text using mouse drag.

## User Interaction

### Entering Selection Mode

- **Click and drag** in the content panel (right side) enters selection mode
- Mouse down sets the anchor point
- Dragging highlights text character-by-character, flowing across lines naturally

### While in Selection Mode

| Action | Result |
|--------|--------|
| Drag to viewport edge | Auto-scroll while extending selection |
| Release mouse | Selection is set, stays in selection mode |
| Scroll (wheel) | Viewport scrolls, selection persists |
| Click (no drag) in content | No-op, keeps current selection |
| Click+drag again in content | Starts new selection |
| `C` | Copy plain text to clipboard, exit selection mode |
| `Esc` | Exit selection mode without copying |
| Click outside content area | Exit selection mode without copying |

### Selection Persistence

Selection is tied to content positions (line + character offset), not screen positions. Scrolling away and back preserves the selection. Selection persists until explicit exit via `C`, `Esc`, click outside, or starting a new drag.

## Visual Design

### Selection Highlight

- Inverted colors (swap foreground/background)
- Applied during rendering, doesn't modify underlying content

### Header Indicator

When selection mode is active, display in the header (after timestamp):

```
SELECTING · C: copy · Esc: cancel
```

- Background: Limegreen
- Text: Inverted (black/dark)

## Implementation Architecture

### New State

```go
type SelectionState struct {
    Active    bool       // Whether we're in selection mode
    Anchor    Position   // Where the drag started (row, col in content)
    Current   Position   // Current drag position
}

type Position struct {
    Row int  // Line number in content
    Col int  // Character position in line
}
```

### Components

1. **Coordinate mapping** - Convert mouse screen coordinates to content position (accounting for viewport offset, borders, and scroll position)

2. **Content line tracking** - Parse the formatted transcript into a slice of lines so we can map positions to actual text

3. **Selection rendering** - Before displaying, apply inverted styling to characters within the selection range

4. **ANSI stripping** - When copying, extract the raw text from selected range and strip escape codes (library: `github.com/acarl005/stripansi`)

5. **Clipboard access** - Use `golang.design/x/clipboard` to copy to system clipboard

6. **Auto-scroll during drag** - When mouse Y is at viewport edge during drag, scroll and extend selection

### Auto-scroll Behavior

- Trigger: Mouse Y within ~2 rows of viewport top/bottom edge during drag
- Action: Scroll 1 line in that direction at regular interval (50-100ms)
- Selection extends as content scrolls

## Edge Cases

| Case | Behavior |
|------|----------|
| Empty content | Nothing to select, drag is no-op |
| Selection spans more than viewport | Works correctly (content positions, not screen) |
| Selecting across styled text | Highlight overlays styling, copy extracts plain text |
| Drag starts outside content area | Ignored, no selection mode entered |
| Click without drag (empty selection) | Exits selection mode, no-op |
