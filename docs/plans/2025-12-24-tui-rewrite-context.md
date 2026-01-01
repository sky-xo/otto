# TUI Rewrite Context (2025-12-24)

## Current Task

**Rewriting `internal/tui/watch.go`** with proper two-column layout that doesn't overflow the terminal.

## The Problem

Previous TUI implementation had **HEIGHT overflow issues** - content was expanding beyond terminal bounds, pushing layout off-screen. The issue was NOT width, but HEIGHT.

## The Solution (Researched)

Key patterns from Bubble Tea best practices:

1. **Use `viewport.Model`** for scrolling content - don't manually slice content
2. **Set explicit `.Height()` on all panels** - prevents auto-expansion
3. **Calculate heights from `tea.WindowSizeMsg`** - dynamic, not hardcoded
4. **Use `lipgloss.JoinHorizontal/JoinVertical`** for layout composition

### Critical Code Pattern

```go
const sidebarWidth = 20
const statusBarHeight = 1

type model struct {
    width, height int
    viewport      viewport.Model  // for content scrolling
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width, m.height = msg.Width, msg.Height

        // Main area height = terminal - status bar
        mainHeight := max(0, m.height - statusBarHeight)

        // Content viewport gets remaining width after sidebar
        m.viewport.Width = max(0, m.width - sidebarWidth)
        m.viewport.Height = mainHeight
    }
    return m, nil
}

func (m model) View() string {
    mainHeight := max(0, m.height - statusBarHeight)

    // Sidebar with EXPLICIT height
    sidebar := lipgloss.NewStyle().
        Width(sidebarWidth).
        Height(mainHeight).   // <-- CRITICAL!
        Render(m.renderChannelList())

    // Content uses viewport (handles scrolling)
    content := m.viewport.View()

    // Join horizontally
    mainArea := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

    // Status bar
    statusBar := lipgloss.NewStyle().
        Width(m.width).
        Render("j/k: scroll | q: quit")

    return lipgloss.JoinVertical(lipgloss.Top, mainArea, statusBar)
}
```

## Spec Requirements (from design doc)

From `docs/plans/2025-12-23-tui-channel-view-design.md`:

### Layout
- Left panel: channel list (Main + agents) - fixed width ~20 chars
- Right panel: content area - remaining width, uses viewport for scrolling
- Status bar: bjunem, 1 line

### Channel List
- Main channel always at top
- Agents sorted by status: busy -> blocked -> complete -> failed
- Status indicators:
  - `●` green = busy
  - `●` yellow = blocked
  - `○` grey = complete
  - `✗` red = failed

### Content Area
- Main channel: show messages from `repo.ListMessages()`
- Agent channel: show transcript from `repo.ListTranscriptEntries(agentID, "")`
- Scroll support via viewport

### Navigation
- j/k or arrows: move selection in channel list
- Enter: select channel
- Escape: return to Main
- g/G: top/bjunem of content
- q: quit

## Current File State

`internal/tui/watch.go` is currently a simple message viewer without the two-column layout. It needs to be completely rewritten.

## Existing Repo Functions to Use

```go
repo.ListAgents(db)                           // Get agents for channel list
repo.ListMessages(db, filter)                 // Get messages for Main channel
repo.ListTranscriptEntries(db, agentID, "")   // Get transcript for agent channel
```

## Dependencies Already in go.mod

- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/lipgloss` - Styling
- `github.com/charmbracelet/bubbles/viewport` - Scrollable content area

## Next Steps

1. **Dispatch implementer subagent** with this context to rewrite watch.go
2. After implementation, dispatch **spec reviewer** to verify layout matches spec
3. Then dispatch **code quality reviewer** for Go idioms, test coverage
4. Run `make build && make watch` to verify visually

## Background Tasks Running

Many June background agents are running (from previous session). Check with `./june status` before starting new work.

## Key Sources

- [Official Pager Example](https://github.com/charmbracelet/bubbletea/blob/main/examples/pager/main.go)
- [Layout Handling Discussion](https://github.com/charmbracelet/bubbletea/discussions/307)
- [Tips for Building BubbleTea Programs](https://leg100.github.io/en/posts/building-bubbletea-programs/)

## Skill Being Used

`superpowers:subagent-driven-development` - Fresh subagent per task with two-stage review (spec compliance first, then code quality).
