// internal/tui/model.go
package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"june/internal/claude"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"golang.design/x/clipboard"
)

const sidebarWidth = 23

// selectionHighlightColor is the background color for selected text (256-color palette gray)
var selectionHighlightColor = Color{Type: Color256, Value: 238}

var (
	// AdaptiveColor: Light = color on light bg, Dark = color on dark bg
	activeStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "2", Dark: "10"})  // green
	doneStyle       = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "243", Dark: "8"}) // gray
	selectedBgStyle = lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: "254", Dark: "8"}) // highlighted background
	promptStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "4", Dark: "6"}).Bold(true) // blue/cyan, bold
	promptBarStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "4", Dark: "6"})            // blue/cyan for half-block
	toolStyle       = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#C8FB9E"})        // lime green (matches focused border)
	toolBoldStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#C8FB9E"}).Bold(true) // lime green bold for tool names
	toolDimStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "243", Dark: "240"})                   // dim gray for command details
	diffAddStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#98FB98"}).
			Background(lipgloss.AdaptiveColor{Light: "#E8F5E9", Dark: "#1B3D1B"}) // green fg + subtle green bg
	diffDelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#C62828", Dark: "#FF6B6B"}).
			Background(lipgloss.AdaptiveColor{Light: "#FFEBEE", Dark: "#3D1B1B"}) // red fg + subtle red bg
	statusBarStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "243", Dark: "8"})          // gray

	focusedBorderColor   = lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#C8FB9E"} // green (darker on light, pale on dark)
	unfocusedBorderColor = lipgloss.AdaptiveColor{Light: "243", Dark: "8"}           // gray

	selectionIndicatorStyle = lipgloss.NewStyle().
		Background(lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#C8FB9E"}). // lime green (matches focused border)
		Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#000000"}). // contrasting text
		Bold(true).
		Padding(0, 1)
)

// Panel focus
const (
	panelLeft  = 0
	panelRight = 1
)

// Position represents a location in content (row = line number, col = character offset)
type Position struct {
	Row int // Line number in content (0-indexed)
	Col int // Character position in line (0-indexed)
}

// SelectionState tracks text selection in the content panel
type SelectionState struct {
	Active   bool     // Whether selection mode is active
	Anchor   Position // Where the drag started
	Current  Position // Current drag position
	Dragging bool     // Whether mouse button is currently held down
}

// IsEmpty returns true if there's no actual selection (same start and end, or not active)
func (s SelectionState) IsEmpty() bool {
	if !s.Active {
		return true
	}
	return s.Anchor == s.Current
}

// Normalize returns start and end positions where start is always before end
func (s SelectionState) Normalize() (start, end Position) {
	if s.Anchor.Row < s.Current.Row || (s.Anchor.Row == s.Current.Row && s.Anchor.Col <= s.Current.Col) {
		return s.Anchor, s.Current
	}
	return s.Current, s.Anchor
}

// screenToContentPosition converts screen coordinates to content position
// accounting for panel borders, sidebar width, and viewport scroll offset.
func (m *Model) screenToContentPosition(screenX, screenY int) Position {
	leftWidth, _, _, _ := m.layout()

	// Convert screen X to content column
	// Subtract: sidebar width + right panel left border (1)
	col := screenX - leftWidth - 1
	if col < 0 {
		col = 0
	}

	// Convert screen Y to content row
	// Subtract: top border (1), then add viewport scroll offset
	row := screenY - 1 + m.viewport.YOffset
	if row < 0 {
		row = 0
	}

	// Clamp to valid content range
	if len(m.contentLines) > 0 {
		if row >= len(m.contentLines) {
			row = len(m.contentLines) - 1
		}
		// Clamp column to line length
		lineLen := len(m.contentLines[row])
		if col > lineLen {
			col = lineLen
		}
	}

	return Position{Row: row, Col: col}
}

// getSelectedText extracts the selected text from content
func (m *Model) getSelectedText() string {
	if !m.selection.Active || m.selection.IsEmpty() {
		return ""
	}

	start, end := m.selection.Normalize()

	if len(m.contentLines) == 0 || start.Row >= len(m.contentLines) {
		return ""
	}
	if end.Row >= len(m.contentLines) {
		end.Row = len(m.contentLines) - 1
		end.Col = len(m.contentLines[end.Row])
	}

	var result strings.Builder

	for row := start.Row; row <= end.Row; row++ {
		line := m.contentLines[row]
		lineLen := len(line)

		startCol := 0
		endCol := lineLen

		if row == start.Row {
			startCol = start.Col
			if startCol > lineLen {
				startCol = lineLen
			}
		}
		if row == end.Row {
			endCol = end.Col
			if endCol > lineLen {
				endCol = lineLen
			}
		}

		if startCol < endCol {
			// Extract plain text from cells
			for i := startCol; i < endCol; i++ {
				result.WriteRune(line[i].Char)
			}
		}

		if row < end.Row {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// copySelection copies the selected text to the system clipboard
func (m *Model) copySelection() {
	text := m.getSelectedText()
	if text == "" {
		return
	}

	// Initialize clipboard (safe to call multiple times)
	if err := clipboard.Init(); err != nil {
		return // Silently fail if clipboard unavailable
	}

	clipboard.Write(clipboard.FmtText, []byte(text))
}

// updateSelectionHighlight applies or clears selection highlighting in the viewport.
// Call this when selection state changes (not on every scroll).
func (m *Model) updateSelectionHighlight() {
	m.renderViewportContent()
}

// applySelectionHighlight returns content lines with selection highlighting applied
func (m *Model) applySelectionHighlight() []StyledLine {
	if !m.selection.Active || m.selection.IsEmpty() || len(m.contentLines) == 0 {
		return m.contentLines
	}

	start, end := m.selection.Normalize()
	result := make([]StyledLine, len(m.contentLines))
	copy(result, m.contentLines)

	// Clamp to valid range
	if start.Row >= len(m.contentLines) {
		return result
	}
	if end.Row >= len(m.contentLines) {
		end.Row = len(m.contentLines) - 1
		end.Col = len(m.contentLines[end.Row])
	}

	for row := start.Row; row <= end.Row; row++ {
		startCol := 0
		endCol := len(m.contentLines[row])

		if row == start.Row {
			startCol = start.Col
		}
		if row == end.Row {
			endCol = end.Col
		}

		if startCol < endCol {
			result[row] = m.contentLines[row].WithSelection(startCol, endCol, selectionHighlightColor)
		}
	}

	return result
}

// sidebarItem represents either a channel header or an agent in the sidebar.
type sidebarItem struct {
	isHeader    bool
	channelName string        // Only set for headers
	channelIdx  int           // Index into m.channels
	agent       *claude.Agent // Only set for agents
	agentIdx    int           // Index within channel's agents slice
}

// Model is the TUI state.
type Model struct {
	claudeProjectsDir string                    // Base Claude projects directory (~/.claude/projects)
	basePath          string                    // Git repo base path
	repoName          string                    // Repository name (e.g., "june")
	channels          []claude.Channel          // Channels with their agents
	transcripts       map[string][]claude.Entry // Agent ID -> transcript entries

	selectedIdx        int            // Currently selected item index (across all channels + headers)
	lastViewedAgent    *claude.Agent  // Last agent shown in right panel (persists when header selected)
	sidebarOffset      int            // Scroll offset for the sidebar
	lastNavWasKeyboard bool           // Track if last sidebar interaction was keyboard (for auto-scroll behavior)
	focusedPanel       int            // Which panel has focus (panelLeft or panelRight)
	selection          SelectionState // Text selection state
	width              int
	height             int
	viewport           viewport.Model
	contentLines       []StyledLine // Lines of content for selection mapping
	err                error
}

// NewModel creates a new TUI model.
func NewModel(claudeProjectsDir, basePath, repoName string) Model {
	return Model{
		claudeProjectsDir: claudeProjectsDir,
		basePath:          basePath,
		repoName:          repoName,
		channels:          []claude.Channel{},
		transcripts:       make(map[string][]claude.Entry),
		viewport:          viewport.New(0, 0),
	}
}

// sidebarItems returns a flat list of all items to display in the sidebar.
func (m Model) sidebarItems() []sidebarItem {
	var items []sidebarItem
	for ci, ch := range m.channels {
		// Add channel header
		items = append(items, sidebarItem{
			isHeader:    true,
			channelName: ch.Name,
			channelIdx:  ci,
		})
		// Add agents
		for ai := range ch.Agents {
			items = append(items, sidebarItem{
				isHeader:   false,
				channelIdx: ci,
				agent:      &m.channels[ci].Agents[ai],
				agentIdx:   ai,
			})
		}
	}
	return items
}

// countSeparatorsBefore returns the number of blank separator lines that would appear
// before the given item index. Separators appear before each channel header except the first.
func (m Model) countSeparatorsBefore(itemIdx int) int {
	items := m.sidebarItems()
	count := 0
	for i := 0; i < itemIdx && i < len(items); i++ {
		if items[i].isHeader && i > 0 {
			count++
		}
	}
	return count
}

// countSeparatorsInRange returns the number of blank separator lines in the given item range [start, end).
func (m Model) countSeparatorsInRange(start, end int) int {
	items := m.sidebarItems()
	count := 0
	for i := start; i < end && i < len(items); i++ {
		if items[i].isHeader && i > 0 {
			count++
		}
	}
	return count
}

// totalSidebarItems returns the total count of items (headers + agents).
func (m Model) totalSidebarItems() int {
	count := 0
	for _, ch := range m.channels {
		count++ // header
		count += len(ch.Agents)
	}
	return count
}

// SelectedAgent returns the currently selected agent, or nil if a header is selected.
func (m Model) SelectedAgent() *claude.Agent {
	items := m.sidebarItems()
	if m.selectedIdx < 0 || m.selectedIdx >= len(items) {
		return nil
	}
	item := items[m.selectedIdx]
	if item.isHeader {
		return nil
	}
	return item.agent
}

// sidebarVisibleLines returns how many sidebar items can be displayed in the sidebar,
// accounting for scroll indicators and separator lines between channels.
// Note: This returns item count, not screen lines. Actual screen lines used may be higher
// due to separator lines between channels.
func (m Model) sidebarVisibleLines() int {
	_, _, _, contentHeight := m.layout()
	if contentHeight <= 0 {
		return 0
	}

	lines := contentHeight
	// Reserve space for top indicator if scrolled down
	if m.sidebarOffset > 0 {
		lines--
	}

	// We need to figure out how many items fit given that separators take space too.
	// Use iterative approach: start with assuming all lines are items, then adjust.
	items := m.sidebarItems()
	totalItems := len(items)

	// Start with naive estimate
	visibleItems := lines
	for {
		endIdx := m.sidebarOffset + visibleItems
		if endIdx > totalItems {
			endIdx = totalItems
		}

		// Count separators in visible range
		sepsInRange := m.countSeparatorsInRange(m.sidebarOffset, endIdx)

		// Total screen lines needed = items + separators
		screenLinesNeeded := visibleItems + sepsInRange

		// Reserve space for bottom indicator if there are more items below
		hasMoreBelow := endIdx < totalItems
		if hasMoreBelow {
			screenLinesNeeded++ // need room for bottom indicator
		}

		if screenLinesNeeded <= lines {
			// We fit, done
			break
		}

		// We don't fit, reduce visible items
		visibleItems--
		if visibleItems <= 0 {
			visibleItems = 0
			break
		}
	}

	return visibleItems
}

// ensureSelectedVisible adjusts sidebarOffset to keep selectedIdx visible.
func (m *Model) ensureSelectedVisible() {
	if m.totalSidebarItems() == 0 {
		return
	}

	_, _, _, contentHeight := m.layout()
	if contentHeight <= 0 {
		return
	}

	// If selection is above the visible area, scroll up
	if m.selectedIdx < m.sidebarOffset {
		m.sidebarOffset = m.selectedIdx
		return
	}

	// If selection is below the visible area, scroll down
	// We need to iterate because changing offset affects visible lines
	// (due to scroll indicators taking space)
	for {
		visibleLines := m.sidebarVisibleLines()
		if visibleLines <= 0 {
			break
		}

		if m.selectedIdx >= m.sidebarOffset+visibleLines {
			// Selected item is below visible area, scroll down
			m.sidebarOffset++
		} else {
			// Selected item is now visible
			break
		}
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		scanChannelsCmd(m.claudeProjectsDir, m.basePath, m.repoName),
	)
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle selection mode keys first
		if m.selection.Active {
			switch msg.String() {
			case "esc":
				m.selection = SelectionState{}
				m.updateSelectionHighlight()
				return m, nil
			case "c":
				m.copySelection()
				m.selection = SelectionState{}
				m.updateSelectionHighlight()
				return m, nil
			}
			// In selection mode, block other keys except quit
			if msg.String() != "q" && msg.String() != "ctrl+c" {
				return m, nil
			}
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			// Toggle focus between panels
			if m.focusedPanel == panelLeft {
				m.focusedPanel = panelRight
			} else {
				m.focusedPanel = panelLeft
			}
		case "up", "k":
			if m.focusedPanel == panelLeft {
				// Navigate agent list
				if m.selectedIdx > 0 {
					m.selectedIdx--
					m.lastNavWasKeyboard = true
					m.ensureSelectedVisible()
					if agent := m.SelectedAgent(); agent != nil {
						m.lastViewedAgent = agent
						cmds = append(cmds, loadTranscriptCmd(*agent))
					}
				}
				// Return early to prevent viewport from also handling this key
				return m, tea.Batch(cmds...)
			} else {
				// Scroll transcript
				m.viewport.LineUp(1)
			}
		case "down", "j":
			if m.focusedPanel == panelLeft {
				// Navigate sidebar list
				if m.selectedIdx < m.totalSidebarItems()-1 {
					m.selectedIdx++
					m.lastNavWasKeyboard = true
					m.ensureSelectedVisible()
					if agent := m.SelectedAgent(); agent != nil {
						m.lastViewedAgent = agent
						cmds = append(cmds, loadTranscriptCmd(*agent))
					}
				}
				// Return early to prevent viewport from also handling this key
				return m, tea.Batch(cmds...)
			} else {
				// Scroll transcript
				m.viewport.LineDown(1)
			}
		case "u":
			if m.focusedPanel == panelLeft {
				// Page up - vim-style until hitting top, then jump to first
				visualRow := m.selectedIdx - m.sidebarOffset
				oldOffset := m.sidebarOffset
				_, _, _, contentHeight := m.layout()
				pageSize := contentHeight / 2
				if pageSize < 1 {
					pageSize = 1
				}
				m.sidebarOffset -= pageSize
				if m.sidebarOffset < 0 {
					m.sidebarOffset = 0
				}
				if m.sidebarOffset == 0 && oldOffset == 0 {
					// Already at top, move selection to first item
					m.selectedIdx = 0
				} else {
					// Keep selection at same visual row
					m.selectedIdx = m.sidebarOffset + visualRow
					if m.selectedIdx < 0 {
						m.selectedIdx = 0
					}
				}
				m.lastNavWasKeyboard = true
				if agent := m.SelectedAgent(); agent != nil {
					m.lastViewedAgent = agent
					cmds = append(cmds, loadTranscriptCmd(*agent))
				}
				return m, tea.Batch(cmds...)
			} else {
				m.viewport.HalfViewUp()
			}
		case "d":
			if m.focusedPanel == panelLeft {
				// Page down - vim-style until hitting bottom, then jump to last
				visualRow := m.selectedIdx - m.sidebarOffset
				oldOffset := m.sidebarOffset
				_, _, _, contentHeight := m.layout()
				pageSize := contentHeight / 2
				if pageSize < 1 {
					pageSize = 1
				}
				maxOffset := m.totalSidebarItems() - m.sidebarVisibleLines()
				if maxOffset < 0 {
					maxOffset = 0
				}
				m.sidebarOffset += pageSize
				if m.sidebarOffset > maxOffset {
					m.sidebarOffset = maxOffset
				}
				if m.sidebarOffset == maxOffset && oldOffset == maxOffset {
					// Already at bottom, move selection to last item
					m.selectedIdx = m.totalSidebarItems() - 1
				} else {
					// Keep selection at same visual row
					m.selectedIdx = m.sidebarOffset + visualRow
					if m.selectedIdx >= m.totalSidebarItems() {
						m.selectedIdx = m.totalSidebarItems() - 1
					}
				}
				m.lastNavWasKeyboard = true
				if agent := m.SelectedAgent(); agent != nil {
					m.lastViewedAgent = agent
					cmds = append(cmds, loadTranscriptCmd(*agent))
				}
				return m, tea.Batch(cmds...)
			} else {
				m.viewport.HalfViewDown()
			}
		case "g":
			m.viewport.GotoTop()
		case "G":
			m.viewport.GotoBottom()
		}

	case tea.MouseMsg:
		leftWidth, _, _, _ := m.layout()
		inLeftPanel := msg.X < leftWidth

		// If selection is active and user clicks in left panel, cancel selection
		if m.selection.Active && inLeftPanel && msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionRelease {
			m.selection = SelectionState{}
			m.updateSelectionHighlight()
			// Don't return here - let the click also select an agent if applicable
		}

		// Handle scroll wheel (check button type directly, wheel events always work)
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if inLeftPanel {
				// Scroll sidebar up by 1 line
				if m.sidebarOffset > 0 {
					m.sidebarOffset--
				}
				m.lastNavWasKeyboard = false // Mouse scroll - don't auto-scroll to selection
			} else {
				m.viewport.LineUp(1)
			}
			return m, nil
		case tea.MouseButtonWheelDown:
			if inLeftPanel {
				// Scroll sidebar down by 1 line
				maxOffset := m.totalSidebarItems() - m.sidebarVisibleLines()
				if maxOffset < 0 {
					maxOffset = 0
				}
				if m.sidebarOffset < maxOffset {
					m.sidebarOffset++
				}
				m.lastNavWasKeyboard = false // Mouse scroll - don't auto-scroll to selection
			} else {
				m.viewport.LineDown(1)
			}
			return m, nil
		}

		// Handle clicks in left panel to select items
		if inLeftPanel && msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionRelease {
			// Calculate which item was clicked
			// Subtract 1 for top border
			clickY := msg.Y - 1

			// Account for top scroll indicator if present
			if m.sidebarOffset > 0 {
				clickY-- // First line is the "N more" indicator
			}

			// Convert to item index, but only within visible range
			visibleLines := m.sidebarVisibleLines()
			if clickY >= 0 && clickY < visibleLines {
				itemIdx := m.sidebarOffset + clickY
				if itemIdx >= 0 && itemIdx < m.totalSidebarItems() {
					m.selectedIdx = itemIdx
					if agent := m.SelectedAgent(); agent != nil {
						m.lastViewedAgent = agent
						cmds = append(cmds, loadTranscriptCmd(*agent))
					}
				}
			}
		}

		// Handle mouse events for text selection in right panel
		if !inLeftPanel {
			switch msg.Action {
			case tea.MouseActionPress:
				if msg.Button == tea.MouseButtonLeft {
					// Start new selection
					pos := m.screenToContentPosition(msg.X, msg.Y)
					m.selection = SelectionState{
						Active:   true,
						Dragging: true,
						Anchor:   pos,
						Current:  pos,
					}
					// Don't update highlighting yet - wait for drag or release
					return m, nil
				}
			case tea.MouseActionMotion:
				if m.selection.Dragging {
					// Update selection end point
					newPos := m.screenToContentPosition(msg.X, msg.Y)
					selectionChanged := newPos != m.selection.Current
					if selectionChanged {
						m.selection.Current = newPos
					}

					// Auto-scroll if near edges
					_, _, _, contentHeight := m.layout()
					edgeThreshold := 2

					// Y position relative to content area (subtract top border)
					relativeY := msg.Y - 1

					if relativeY <= edgeThreshold && m.viewport.YOffset > 0 {
						// Near top edge - scroll up
						m.viewport.LineUp(1)
						// Update selection to follow scroll
						newPos = m.screenToContentPosition(msg.X, msg.Y)
						if newPos != m.selection.Current {
							m.selection.Current = newPos
							selectionChanged = true
						}
					} else if relativeY >= contentHeight-edgeThreshold {
						// Near bottom edge - scroll down
						m.viewport.LineDown(1)
						// Update selection to follow scroll
						newPos = m.screenToContentPosition(msg.X, msg.Y)
						if newPos != m.selection.Current {
							m.selection.Current = newPos
							selectionChanged = true
						}
					}

					// Only update highlighting when selection actually changed
					if selectionChanged {
						m.updateSelectionHighlight()
					}

					return m, nil
				}
			case tea.MouseActionRelease:
				if msg.Button == tea.MouseButtonLeft && m.selection.Dragging {
					// Finish dragging, keep selection active
					m.selection.Current = m.screenToContentPosition(msg.X, msg.Y)
					m.selection.Dragging = false

					// If empty selection (click without drag), exit selection mode
					if m.selection.IsEmpty() {
						m.selection.Active = false
					}
					m.updateSelectionHighlight()
					return m, nil
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateViewportDimensions()
		m.updateViewport()

	case tickMsg:
		cmds = append(cmds, tickCmd(), scanChannelsCmd(m.claudeProjectsDir, m.basePath, m.repoName))

	case channelsMsg:
		m.channels = msg
		if agent := m.SelectedAgent(); agent != nil {
			m.lastViewedAgent = agent
			cmds = append(cmds, loadTranscriptCmd(*agent))
		}

	case transcriptMsg:
		// Check if we were at the bottom BEFORE updating content
		wasAtBottom := m.viewport.AtBottom()
		_, hadTranscript := m.transcripts[msg.agentID]
		m.transcripts[msg.agentID] = msg.entries
		m.updateViewport()
		if !hadTranscript || wasAtBottom {
			// First time loading OR was following at bottom - keep at bottom
			m.viewport.GotoBottom()
		}

	case errMsg:
		m.err = msg
	}

	// Update viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateViewportDimensions() {
	_, rightWidth, _, contentHeight := m.layout()
	m.viewport.Width = rightWidth - 2  // Subtract borders
	m.viewport.Height = contentHeight
}

func (m *Model) updateViewport() {
	// Use lastViewedAgent to persist content when a header is selected
	agent := m.lastViewedAgent
	if agent == nil {
		m.viewport.SetContent("")
		m.contentLines = nil
		return
	}
	entries := m.transcripts[agent.ID]
	content := formatTranscript(entries, m.viewport.Width)

	// Parse ANSI content into StyledLines
	lines := strings.Split(content, "\n")
	m.contentLines = make([]StyledLine, len(lines))
	for i, line := range lines {
		m.contentLines[i] = ParseStyledLine(line)
	}

	// Render for viewport (apply selection if active)
	m.renderViewportContent()
}

// renderViewportContent renders contentLines to viewport, applying selection if active
func (m *Model) renderViewportContent() {
	lines := m.contentLines
	if m.selection.Active && !m.selection.IsEmpty() {
		lines = m.applySelectionHighlight()
	}

	rendered := make([]string, len(lines))
	for i, line := range lines {
		rendered[i] = line.Render()
	}
	m.viewport.SetContent(strings.Join(rendered, "\n"))
}

// layout calculates panel dimensions
func (m Model) layout() (leftWidth, rightWidth, panelHeight, contentHeight int) {
	// Height: subtract status bar (1) + border top/bottom (2) = 3
	panelHeight = m.height - 1
	if panelHeight < 3 {
		panelHeight = 3
	}
	contentHeight = panelHeight - 2

	// Width: both panels have borders (2 chars each = 4 total)
	availableWidth := m.width
	leftWidth = sidebarWidth
	minRight := 20

	if availableWidth-leftWidth < minRight {
		leftWidth = availableWidth - minRight
		if leftWidth < 10 {
			leftWidth = 10
		}
	}
	rightWidth = availableWidth - leftWidth
	if rightWidth < minRight {
		rightWidth = minRight
	}

	return
}

// View renders the UI.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	leftWidth, rightWidth, panelHeight, contentHeight := m.layout()

	// Determine border colors based on focus
	leftBorderColor := unfocusedBorderColor
	rightBorderColor := unfocusedBorderColor
	if m.focusedPanel == panelLeft {
		leftBorderColor = focusedBorderColor
	} else {
		rightBorderColor = focusedBorderColor
	}

	// Left panel: agent list
	leftContent := m.renderSidebarContent(leftWidth-2, contentHeight)
	leftPanel := renderPanelWithTitle("Subagents", leftContent, leftWidth, panelHeight, leftBorderColor)

	// Right panel: transcript (uses lastViewedAgent to persist when header is selected)
	var rightTitle string
	if agent := m.lastViewedAgent; agent != nil {
		if agent.Description != "" {
			rightTitle = fmt.Sprintf("%s (%s) | %s", agent.Description, agent.ID, formatTimestamp(agent.LastMod))
		} else {
			rightTitle = fmt.Sprintf("%s | %s", agent.ID, formatTimestamp(agent.LastMod))
		}
	}

	// Add selection indicator to title
	if m.selection.Active && !m.selection.IsEmpty() {
		indicator := selectionIndicatorStyle.Render("SELECTING · C: copy · Esc: cancel")
		if rightTitle != "" {
			rightTitle = rightTitle + " " + indicator
		} else {
			rightTitle = indicator
		}
	}

	// Viewport content is already set in Update() with selection highlighting if active
	rightContent := m.viewport.View()
	rightPanel := renderPanelWithTitle(rightTitle, rightContent, rightWidth, panelHeight, rightBorderColor)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	// Status bar
	status := statusBarStyle.Render("Tab: switch | j/k: navigate | u/d: page | g/G: top/bottom | q: quit")

	return lipgloss.JoinVertical(lipgloss.Left, panels, status)
}

func (m Model) renderSidebarContent(width, height int) string {
	items := m.sidebarItems()
	if len(items) == 0 || height <= 0 {
		return "No agents found"
	}

	// Calculate scroll indicators
	hiddenAbove := m.sidebarOffset
	totalItems := len(items)

	availableLines := height
	showTopIndicator := hiddenAbove > 0
	if showTopIndicator {
		availableLines--
	}

	// Calculate how many items we can show, accounting for separator lines
	visibleItems := m.sidebarVisibleLines()
	visibleEnd := m.sidebarOffset + visibleItems
	if visibleEnd > totalItems {
		visibleEnd = totalItems
	}

	hiddenBelow := totalItems - visibleEnd
	showBottomIndicator := hiddenBelow > 0

	var lines []string

	// Top scroll indicator
	if showTopIndicator {
		indicator := fmt.Sprintf("\u2191 %d more", hiddenAbove)
		if len(indicator) > width {
			indicator = indicator[:width]
		}
		lines = append(lines, doneStyle.Render(indicator))
	}

	// Header style
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "4", Dark: "6"}) // cyan

	// Render visible items
	for i := m.sidebarOffset; i < visibleEnd; i++ {
		item := items[i]

		if item.isHeader {
			// Add blank separator line before channel headers (except the first visible item)
			if i > 0 && i > m.sidebarOffset {
				lines = append(lines, "")
			}

			// Render channel header
			header := item.channelName
			if len(header) > width {
				header = header[:width]
			}
			if i == m.selectedIdx {
				// Selected header
				selectedBg := lipgloss.AdaptiveColor{Light: "254", Dark: "8"}
				rest := header
				if len(rest) < width {
					rest = rest + strings.Repeat(" ", width-len(rest))
				}
				lines = append(lines, headerStyle.Background(selectedBg).Render(rest))
			} else {
				lines = append(lines, headerStyle.Render(header))
			}
		} else {
			// Render agent
			// Layout: "● Name" for active (dot + space + name)
			//         "  Name" for inactive (2 spaces + name)
			// Text position stays the same whether active or not
			agent := item.agent

			name := agent.Description
			if name == "" {
				name = agent.ID
			}
			maxNameLen := width - 2 // 2 chars for prefix (dot+space or 2 spaces)
			if len(name) > maxNameLen {
				name = name[:maxNameLen]
			}

			if i == m.selectedIdx {
				selectedBg := lipgloss.AdaptiveColor{Light: "254", Dark: "8"}
				var prefix string
				if agent.IsActive() {
					prefix = activeStyle.Background(selectedBg).Render("\u25cf") + selectedBgStyle.Render(" ")
				} else {
					prefix = selectedBgStyle.Render("  ")
				}
				rest := name
				if 2+len(rest) < width {
					rest = rest + strings.Repeat(" ", width-2-len(rest))
				}
				lines = append(lines, prefix+selectedBgStyle.Render(rest))
			} else {
				if agent.IsActive() {
					lines = append(lines, activeStyle.Render("\u25cf")+" "+name)
				} else {
					lines = append(lines, "  "+name)
				}
			}
		}
	}

	// Bottom scroll indicator
	if showBottomIndicator {
		indicator := fmt.Sprintf("\u2193 %d more", hiddenBelow)
		if len(indicator) > width {
			indicator = indicator[:width]
		}
		lines = append(lines, doneStyle.Render(indicator))
	}

	return strings.Join(lines, "\n")
}

// truncateToWidth truncates a string (possibly with ANSI codes) to fit within maxWidth.
// Uses ANSI-aware truncation to avoid cutting through escape sequences.
func truncateToWidth(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}

	// Use ANSI-aware truncation that won't cut through escape sequences
	result := ansi.Truncate(s, maxWidth, "")

	// Ensure styles are reset after truncation to prevent bleeding
	result = result + "\x1b[0m"

	// Pad if needed (reset has 0 visual width so this is still correct)
	resultWidth := lipgloss.Width(result)
	if resultWidth < maxWidth {
		result = result + strings.Repeat(" ", maxWidth-resultWidth)
	}
	return result
}

// renderPanelWithTitle renders a panel with the title embedded in the top border
// like: ╭─ Title ────────╮
// This is copied from the old working TUI code.
func renderPanelWithTitle(title, content string, width, height int, borderColor lipgloss.TerminalColor) string {
	// Border characters (rounded)
	const (
		topLeft     = "╭"
		topRight    = "╮"
		bottomLeft  = "╰"
		bottomRight = "╯"
		horizontal  = "─"
		vertical    = "│"
	)

	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	titleStyle := lipgloss.NewStyle().Foreground(borderColor).Bold(true)

	// Content width is width - 2 (for left and right borders)
	contentWidth := width - 2
	if contentWidth < 0 {
		contentWidth = 0
	}

	// Pre-render border characters once
	leftBorder := borderStyle.Render(vertical)
	rightBorder := borderStyle.Render(vertical)

	// Build top border with embedded title
	var topBorder string
	if title == "" {
		topBorder = borderStyle.Render(topLeft + strings.Repeat(horizontal, contentWidth) + topRight)
	} else {
		titleText := " " + title + " "
		titleLen := lipgloss.Width(titleText) // Use visual width for ANSI-aware measurement

		remainingWidth := contentWidth - titleLen
		leftDashes := 1
		rightDashes := remainingWidth - leftDashes
		if rightDashes < 0 {
			rightDashes = 0
		}

		topBorder = borderStyle.Render(topLeft+strings.Repeat(horizontal, leftDashes)) +
			titleStyle.Render(titleText) +
			borderStyle.Render(strings.Repeat(horizontal, rightDashes)+topRight)
	}

	// Build bottom border
	bottomBorder := borderStyle.Render(bottomLeft + strings.Repeat(horizontal, contentWidth) + bottomRight)

	// Split content into lines and pad/truncate to fit
	lines := strings.Split(content, "\n")
	contentLines := height - 2 // Subtract top and bottom borders
	if contentLines < 0 {
		contentLines = 0
	}

	// Pre-allocate with exact capacity
	middleLines := make([]string, 0, contentLines)

	// Render middle lines with side borders
	for i := 0; i < contentLines; i++ {
		var line string
		if i < len(lines) {
			line = lines[i]
		}
		// Use lipgloss.Width for visual width (handles ANSI codes)
		visualWidth := lipgloss.Width(line)
		if visualWidth < contentWidth {
			// Reset any active styles before padding to prevent background color bleeding
			line = line + "\x1b[0m" + strings.Repeat(" ", contentWidth-visualWidth)
		} else if visualWidth > contentWidth {
			// Truncate line (includes reset after truncation)
			line = truncateToWidth(line, contentWidth)
		} else {
			// Exact width - still reset styles to prevent bleeding into borders
			line = line + "\x1b[0m"
		}
		middleLines = append(middleLines, leftBorder+line+rightBorder)
	}

	// Join all parts
	allLines := []string{topBorder}
	allLines = append(allLines, middleLines...)
	allLines = append(allLines, bottomBorder)

	return strings.Join(allLines, "\n")
}

func formatTranscript(entries []claude.Entry, width int) string {
	var lines []string
	lines = append(lines, "") // top padding

	lastWasText := false // track if previous entry was text (for spacing before tools)

	for _, e := range entries {
		switch e.Type {
		case "user":
			if e.IsToolResult() {
				continue
			}
			content := strings.TrimSpace(e.TextContent())
			if content != "" {
				// Add blank line before user prompts (when following assistant content)
				if len(lines) > 1 {
					lines = append(lines, "")
				}
				// Use a colored half-block on the right as visual indicator
				bar := promptBarStyle.Render("▐")
				contentLines := strings.Split(content, "\n")
				for _, line := range contentLines {
					lines = append(lines, bar+promptStyle.Render(" "+strings.TrimRight(line, " ")))
				}
				lastWasText = true
			}
		case "assistant":
			if tool := e.ToolName(); tool != "" {
				// Add blank line before tools if previous was text
				if lastWasText {
					lines = append(lines, "")
				}
				toolLines := formatToolUse(e, tool, width)
				lines = append(lines, toolLines...)
				lastWasText = false
			} else if text := e.TextContent(); text != "" {
				rendered := renderMarkdown(text, width)
				lines = append(lines, rendered)
				lastWasText = true
			}
		}
	}
	return strings.Join(lines, "\n")
}

// formatToolUse formats a tool use entry, with special handling for Bash commands.
func formatToolUse(e claude.Entry, toolName string, width int) []string {
	var result []string
	maxLen := width - 4 // leave room for "  " prefix and some padding

	// Special handling for Bash: show description + dimmed command
	if toolName == "Bash" {
		input := e.ToolInput()
		desc, _ := input["description"].(string)
		cmd, _ := input["command"].(string)

		if desc != "" {
			// Truncate description if needed
			if maxLen > 0 && len(desc) > maxLen-7 { // "Bash(" + ")" = 6 chars + some padding
				desc = desc[:maxLen-10] + "..."
			}
			// Format: Bash(description) with Bash in bold lime green, parens in regular lime green
			line := "  " + toolBoldStyle.Render("Bash") + toolStyle.Render("(") + desc + toolStyle.Render(")")
			result = append(result, line)

			// Show command dimmed below if present
			if cmd != "" {
				// Take first line only
				if idx := strings.Index(cmd, "\n"); idx != -1 {
					cmd = cmd[:idx] + "..."
				}
				if maxLen > 0 && len(cmd) > maxLen {
					cmd = cmd[:maxLen-3] + "..."
				}
				result = append(result, toolDimStyle.Render("    "+cmd))
			}
			return result
		}

		// No description, just show command
		if cmd != "" {
			if idx := strings.Index(cmd, "\n"); idx != -1 {
				cmd = cmd[:idx] + "..."
			}
			if maxLen > 0 && len(cmd) > maxLen-7 {
				cmd = cmd[:maxLen-10] + "..."
			}
			// Format: Bash(command) with Bash in bold lime green, parens in regular lime green
			line := "  " + toolBoldStyle.Render("Bash") + toolStyle.Render("(") + cmd + toolStyle.Render(")")
			result = append(result, line)
			return result
		}
	}

	// Special handling for Edit: show file path + diff
	if toolName == "Edit" {
		input := e.ToolInput()
		filePath, _ := input["file_path"].(string)
		oldStr, _ := input["old_string"].(string)
		newStr, _ := input["new_string"].(string)

		// Show file path in Edit(path) format with split styling
		shortPath := shortenPath(filePath)
		if maxLen > 0 && len(shortPath) > maxLen-7 { // "Edit(" + ")" = 6 chars + some padding
			shortPath = shortPath[:maxLen-10] + "..."
		}
		// Format: Edit(path) with Edit in bold lime green, parens in regular lime green
		line := "  " + toolBoldStyle.Render("Edit") + toolStyle.Render("(") + shortPath + toolStyle.Render(")")
		result = append(result, line)

		// Show summary line
		summaryLine := formatDiffSummary(oldStr, newStr)
		result = append(result, toolDimStyle.Render("    "+summaryLine))

		// Show diff with syntax highlighting based on file type
		diffLines := formatDiff(oldStr, newStr, maxLen, filePath)
		result = append(result, diffLines...)
		return result
	}

	// Special handling for Write: show file path + content with syntax highlighting
	if toolName == "Write" {
		input := e.ToolInput()
		filePath, _ := input["file_path"].(string)
		content, _ := input["content"].(string)

		// Show file path in Write(path) format with split styling
		shortPath := shortenPath(filePath)
		if maxLen > 0 && len(shortPath) > maxLen-8 { // "Write(" + ")" = 7 chars + some padding
			shortPath = shortPath[:maxLen-11] + "..."
		}
		// Format: Write(path) with Write in bold lime green, parens in regular lime green
		line := "  " + toolBoldStyle.Render("Write") + toolStyle.Render("(") + shortPath + toolStyle.Render(")")
		result = append(result, line)

		// Show summary line with line count
		summaryLine := formatWriteSummary(content)
		result = append(result, toolDimStyle.Render("    "+summaryLine))

		// Show content with syntax highlighting based on file type
		contentLines := formatWriteContent(content, maxLen, filePath)
		result = append(result, contentLines...)
		return result
	}

	// Special handling for TodoWrite: show todo list with status indicators
	if toolName == "TodoWrite" {
		input := e.ToolInput()
		todos, _ := input["todos"].([]interface{})

		// Show TodoWrite header
		line := "  " + toolBoldStyle.Render("TodoWrite") + toolStyle.Render("()")
		result = append(result, line)

		// Show each todo with status indicator
		for _, todo := range todos {
			if todoMap, ok := todo.(map[string]interface{}); ok {
				content, _ := todoMap["content"].(string)
				status, _ := todoMap["status"].(string)

				// Choose indicator based on status
				var indicator string
				var style lipgloss.Style
				switch status {
				case "completed":
					indicator = "\u2713" // checkmark
					style = doneStyle
				case "in_progress":
					indicator = "\u25d0" // half circle
					style = activeStyle
				default: // pending
					indicator = "\u2610" // empty box
					style = lipgloss.NewStyle() // normal text color (white-ish)
				}

				// Truncate content if needed
				displayContent := content
				if maxLen > 0 && len(displayContent) > maxLen-6 { // indicator + spaces
					displayContent = displayContent[:maxLen-9] + "..."
				}

				todoLine := "    " + style.Render(indicator+" "+displayContent)
				result = append(result, todoLine)
			}
		}
		return result
	}

	// Default: use ToolSummary for other tools
	// ToolSummary returns "Tool: detail" format, convert to "Tool(detail)" with split styling
	summary := e.ToolSummary()

	// Parse "Tool: detail" format
	if idx := strings.Index(summary, ": "); idx != -1 {
		name := summary[:idx]
		detail := summary[idx+2:]
		if maxLen > 0 && len(detail) > maxLen-len(name)-3 { // name + "(" + ")" + some padding
			detail = detail[:maxLen-len(name)-6] + "..."
		}
		// Format: Tool(detail) with Tool in bold lime green, parens in regular lime green
		line := "  " + toolBoldStyle.Render(name) + toolStyle.Render("(") + detail + toolStyle.Render(")")
		result = append(result, line)
	} else {
		// No detail, just the tool name: show as Tool()
		line := "  " + toolBoldStyle.Render(summary) + toolStyle.Render("()")
		result = append(result, line)
	}
	return result
}

// shortenPath removes common prefixes to show a shorter path.
func shortenPath(path string) string {
	// Remove /Users/xxx/code/ or /home/xxx/code/ prefix
	if idx := strings.Index(path, "/code/"); idx != -1 {
		return path[idx+6:] // Skip "/code/"
	}
	// Fallback: just return last 3 components
	parts := strings.Split(path, "/")
	if len(parts) > 3 {
		return strings.Join(parts[len(parts)-3:], "/")
	}
	return path
}

// formatDiffSummary returns a summary line for the diff like "Added N lines" or "-N +M lines".
func formatDiffSummary(oldStr, newStr string) string {
	// Count non-empty lines
	countLines := func(s string) int {
		if strings.TrimSpace(s) == "" {
			return 0
		}
		count := 0
		for _, line := range strings.Split(s, "\n") {
			if strings.TrimSpace(line) != "" {
				count++
			}
		}
		return count
	}

	oldCount := countLines(oldStr)
	newCount := countLines(newStr)

	if oldCount == 0 && newCount > 0 {
		if newCount == 1 {
			return "\u2514 Added 1 line"
		}
		return fmt.Sprintf("\u2514 Added %d lines", newCount)
	}
	if newCount == 0 && oldCount > 0 {
		if oldCount == 1 {
			return "\u2514 Removed 1 line"
		}
		return fmt.Sprintf("\u2514 Removed %d lines", oldCount)
	}
	// Both have content
	return fmt.Sprintf("\u2514 -%d +%d lines", oldCount, newCount)
}

// formatDiff renders old/new strings as a unified diff with context lines and hunks.
// Shows unchanged context lines around changes, with "..." separators between distant hunks.
// If filePath is provided, syntax highlighting is applied to added/deleted lines.
func formatDiff(oldStr, newStr string, maxLen int, filePath string) []string {
	var result []string

	// Configuration
	const (
		maxDiffLines  = 15 // Limit total output lines
		contextLines  = 3  // Lines of context before/after changes
		gapThreshold  = 3  // Unchanged lines between changes before new hunk
	)

	oldLines := strings.Split(oldStr, "\n")
	newLines := strings.Split(newStr, "\n")

	// Compute the diff
	diff := computeDiff(oldLines, newLines)

	// Extract hunks with context
	hunks := extractHunks(diff, contextLines, gapThreshold)

	if len(hunks) == 0 {
		return result
	}

	// Calculate max line number width for consistent alignment
	maxLineNum := 0
	for _, hunk := range hunks {
		for _, d := range hunk.Lines {
			if d.OldLineNum > maxLineNum {
				maxLineNum = d.OldLineNum
			}
			if d.NewLineNum > maxLineNum {
				maxLineNum = d.NewLineNum
			}
		}
	}
	lineNumWidth := len(fmt.Sprintf("%d", maxLineNum))

	lineCount := 0
	truncated := false

	for hunkIdx, hunk := range hunks {
		// Add separator between hunks
		if hunkIdx > 0 {
			if lineCount >= maxDiffLines {
				truncated = true
				break
			}
			result = append(result, toolDimStyle.Render("    ..."))
			lineCount++
		}

		for _, d := range hunk.Lines {
			if lineCount >= maxDiffLines {
				truncated = true
				break
			}

			content := strings.TrimRight(d.Content, " \t")

			// Determine line number to display
			lineNum := d.OldLineNum
			if lineNum == 0 {
				lineNum = d.NewLineNum
			}

			var display string
			var styled string

			switch d.Op {
			case DiffEqual:
				// Context line: show line number, dim style (no syntax highlighting)
				display = fmt.Sprintf("%*d   %s", lineNumWidth, lineNum, content)
				if maxLen > 0 && len(display) > maxLen {
					display = display[:maxLen-3] + "..."
				}
				styled = toolDimStyle.Render("    " + display)
			case DiffDelete:
				// Deletion: show with "-" marker, apply syntax highlighting
				prefix := fmt.Sprintf("%*d - ", lineNumWidth, lineNum)
				if filePath != "" {
					var bgANSI string
					if lipgloss.HasDarkBackground() {
						bgANSI = ANSIBgDeleteDark
					} else {
						bgANSI = ANSIBgDeleteLight
					}
					// Apply syntax highlighting with background (no padding to avoid wrapping issues)
					highlightedWithBg := highlightWithBackground(content, filePath, bgANSI)
					if highlightedWithBg != content {
						// Highlighting was applied - prefix gets same background, flows into content
						styled = "    " + bgANSI + prefix + highlightedWithBg
					} else {
						// Fallback: no syntax highlighting available, use ANSI background directly
						styled = "    " + bgANSI + prefix + content + "\x1b[0m"
					}
				} else {
					// No filepath, use normal styling
					display = prefix + content
					if maxLen > 0 && len(display) > maxLen {
						display = display[:maxLen-3] + "..."
					}
					styled = diffDelStyle.Render("    " + display)
				}
			case DiffInsert:
				// Addition: show with "+" marker, apply syntax highlighting
				prefix := fmt.Sprintf("%*d + ", lineNumWidth, lineNum)
				if filePath != "" {
					var bgANSI string
					if lipgloss.HasDarkBackground() {
						bgANSI = ANSIBgInsertDark
					} else {
						bgANSI = ANSIBgInsertLight
					}
					// Apply syntax highlighting with background (no padding to avoid wrapping issues)
					highlightedWithBg := highlightWithBackground(content, filePath, bgANSI)
					if highlightedWithBg != content {
						// Highlighting was applied - prefix gets same background, flows into content
						styled = "    " + bgANSI + prefix + highlightedWithBg
					} else {
						// Fallback: no syntax highlighting available, use ANSI background directly
						styled = "    " + bgANSI + prefix + content + "\x1b[0m"
					}
				} else {
					// No filepath, use normal styling
					display = prefix + content
					if maxLen > 0 && len(display) > maxLen {
						display = display[:maxLen-3] + "..."
					}
					styled = diffAddStyle.Render("    " + display)
				}
			}

			result = append(result, styled)
			lineCount++
		}

		if truncated {
			break
		}
	}

	if truncated {
		result = append(result, toolDimStyle.Render("    ... (more lines)"))
	}

	return result
}

// formatWriteSummary returns a summary line for write content like "N lines".
func formatWriteSummary(content string) string {
	if strings.TrimSpace(content) == "" {
		return "\u2514 Empty file"
	}
	lines := strings.Split(content, "\n")
	count := len(lines)
	if count == 1 {
		return "\u2514 1 line"
	}
	return fmt.Sprintf("\u2514 %d lines", count)
}

// formatWriteContent renders file content with syntax highlighting and line numbers.
// Shows content with line numbers, no +/- markers since it's all new content.
func formatWriteContent(content string, maxLen int, filePath string) []string {
	var result []string

	if strings.TrimSpace(content) == "" {
		return result
	}

	const maxContentLines = 15 // Limit total output lines (same as diff)

	lines := strings.Split(content, "\n")

	// Calculate max line number width for consistent alignment
	maxLineNum := len(lines)
	lineNumWidth := len(fmt.Sprintf("%d", maxLineNum))

	lineCount := 0
	truncated := false

	for i, line := range lines {
		if lineCount >= maxContentLines {
			truncated = true
			break
		}

		lineNum := i + 1
		lineContent := strings.TrimRight(line, " \t")

		// Format line with line number
		prefix := fmt.Sprintf("%*d   ", lineNumWidth, lineNum)

		if filePath != "" {
			// Calculate available width for content (after "    " indent and prefix)
			indentWidth := 4
			prefixLen := len(prefix)
			contentWidth := maxLen - indentWidth - prefixLen
			if contentWidth < 0 {
				contentWidth = 0
			}

			// Apply syntax highlighting (no special background for write - it's not a diff)
			highlighted := syntaxHighlight(lineContent, filePath)
			if highlighted != lineContent {
				// Highlighting was applied
				styled := "    " + toolDimStyle.Render(prefix) + highlighted
				result = append(result, styled)
			} else {
				// No highlighting, use dim style for line number, regular for content
				display := prefix + lineContent
				if maxLen > 0 && len(display) > maxLen {
					display = display[:maxLen-3] + "..."
				}
				result = append(result, toolDimStyle.Render("    "+prefix)+lineContent)
			}
		} else {
			// No filepath, use dim style for everything
			display := prefix + lineContent
			if maxLen > 0 && len(display) > maxLen {
				display = display[:maxLen-3] + "..."
			}
			result = append(result, toolDimStyle.Render("    "+display))
		}

		lineCount++
	}

	if truncated {
		result = append(result, toolDimStyle.Render("    ... (more lines)"))
	}

	return result
}

// getGlamourStyle returns the appropriate glamour style for markdown rendering.
// It checks GLAMOUR_STYLE env var first, then auto-detects based on terminal background.
// Falls back to "dark" if no TTY is detected (e.g., in tests) to ensure markdown is processed.
func getGlamourStyle() glamour.TermRendererOption {
	// First check if user has set GLAMOUR_STYLE env var
	if style := os.Getenv("GLAMOUR_STYLE"); style != "" {
		return glamour.WithStylePath(style)
	}

	// Use lipgloss to detect terminal background color.
	// lipgloss.HasDarkBackground() returns true for dark terminals, false for light,
	// and defaults to true when no TTY is detected (which is what we want for tests).
	if lipgloss.HasDarkBackground() {
		return glamour.WithStylePath("dark")
	}
	return glamour.WithStylePath("light")
}

// renderMarkdown renders markdown text using glamour with auto-detected terminal style.
func renderMarkdown(text string, width int) string {
	if width <= 0 {
		width = 80 // default width
	}

	r, err := glamour.NewTermRenderer(
		getGlamourStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		// If renderer creation fails, return plain text
		return text
	}

	rendered, err := r.Render(text)
	if err != nil {
		// If rendering fails, return plain text
		return text
	}

	// glamour adds trailing newlines and leading whitespace, clean them up
	rendered = strings.TrimRight(rendered, "\n")

	// Strip leading whitespace from each line
	lines := strings.Split(rendered, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimLeft(line, " ")
	}
	return strings.Join(lines, "\n")
}

// formatTimestamp formats a timestamp intelligently based on how recent it is:
// - Today: "5:15:24 PM"
// - Yesterday: "Yesterday @ 11:45:23 PM"
// - This week: "Tue @ 5:30:21 PM"
// - Older: "5 Oct @ 11:30:25 AM"
func formatTimestamp(t time.Time) string {
	now := time.Now()
	timeStr := t.Format("3:04:05 PM")

	// Check if same day
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return timeStr
	}

	// Check if yesterday
	yesterday := now.AddDate(0, 0, -1)
	if t.Year() == yesterday.Year() && t.YearDay() == yesterday.YearDay() {
		return "Yesterday @ " + timeStr
	}

	// Check if within the last 7 days
	weekAgo := now.AddDate(0, 0, -7)
	if t.After(weekAgo) {
		return t.Format("Mon") + " @ " + timeStr
	}

	// Older - show date without year
	return t.Format("2 Jan") + " @ " + timeStr
}
