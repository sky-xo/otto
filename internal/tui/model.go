// internal/tui/model.go
package tui

import (
	"fmt"
	"strings"
	"time"

	"june/internal/claude"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

const sidebarWidth = 23

var (
	activeStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // green
	doneStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))  // gray
	selectedBgStyle = lipgloss.NewStyle().Background(lipgloss.Color("8"))  // highlighted background
	promptStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true).Background(lipgloss.Color("236")) // cyan on subtle dark gray
	toolStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	statusBarStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	focusedBorderColor   = lipgloss.Color("#C8FB9E") // Pale lime green
	unfocusedBorderColor = lipgloss.Color("8") // Dim
)

// Panel focus
const (
	panelLeft  = 0
	panelRight = 1
)

// Model is the TUI state.
type Model struct {
	projectDir  string                    // Claude project directory we're watching
	agents      []claude.Agent            // List of agents
	transcripts map[string][]claude.Entry // Agent ID -> transcript entries

	selectedIdx   int // Currently selected agent index
	sidebarOffset int // Scroll offset for the sidebar (index of first visible agent)
	focusedPanel  int // Which panel has focus (panelLeft or panelRight)
	width         int
	height        int
	viewport      viewport.Model
	err           error
}

// NewModel creates a new TUI model.
func NewModel(projectDir string) Model {
	return Model{
		projectDir:  projectDir,
		agents:      []claude.Agent{},
		transcripts: make(map[string][]claude.Entry),
		viewport:    viewport.New(0, 0),
	}
}

// SelectedAgent returns the currently selected agent, or nil if none.
func (m Model) SelectedAgent() *claude.Agent {
	if m.selectedIdx < 0 || m.selectedIdx >= len(m.agents) {
		return nil
	}
	return &m.agents[m.selectedIdx]
}

// sidebarVisibleLines returns how many agent lines can be displayed in the sidebar,
// accounting for scroll indicators if they would be shown.
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
	// Reserve space for bottom indicator if there are more agents below
	endIdx := m.sidebarOffset + lines
	if endIdx < len(m.agents) {
		lines--
	}
	if lines < 0 {
		lines = 0
	}
	return lines
}

// ensureSelectedVisible adjusts sidebarOffset to keep selectedIdx visible.
func (m *Model) ensureSelectedVisible() {
	if len(m.agents) == 0 {
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
		scanAgentsCmd(m.projectDir),
	)
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
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
					m.ensureSelectedVisible()
					if agent := m.SelectedAgent(); agent != nil {
						cmds = append(cmds, loadTranscriptCmd(*agent))
					}
				}
			} else {
				// Scroll transcript
				m.viewport.LineUp(1)
			}
		case "down", "j":
			if m.focusedPanel == panelLeft {
				// Navigate agent list
				if m.selectedIdx < len(m.agents)-1 {
					m.selectedIdx++
					m.ensureSelectedVisible()
					if agent := m.SelectedAgent(); agent != nil {
						cmds = append(cmds, loadTranscriptCmd(*agent))
					}
				}
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
				if agent := m.SelectedAgent(); agent != nil {
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
				maxOffset := len(m.agents) - m.sidebarVisibleLines()
				if maxOffset < 0 {
					maxOffset = 0
				}
				m.sidebarOffset += pageSize
				if m.sidebarOffset > maxOffset {
					m.sidebarOffset = maxOffset
				}
				if m.sidebarOffset == maxOffset && oldOffset == maxOffset {
					// Already at bottom, move selection to last item
					m.selectedIdx = len(m.agents) - 1
				} else {
					// Keep selection at same visual row
					m.selectedIdx = m.sidebarOffset + visualRow
					if m.selectedIdx >= len(m.agents) {
						m.selectedIdx = len(m.agents) - 1
					}
				}
				if agent := m.SelectedAgent(); agent != nil {
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

		// Handle scroll wheel (check button type directly, wheel events always work)
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if inLeftPanel {
				// Scroll sidebar up by 1 line
				if m.sidebarOffset > 0 {
					m.sidebarOffset--
				}
			} else {
				m.viewport.LineUp(1)
			}
			return m, nil
		case tea.MouseButtonWheelDown:
			if inLeftPanel {
				// Scroll sidebar down by 1 line
				maxOffset := len(m.agents) - m.sidebarVisibleLines()
				if maxOffset < 0 {
					maxOffset = 0
				}
				if m.sidebarOffset < maxOffset {
					m.sidebarOffset++
				}
			} else {
				m.viewport.LineDown(1)
			}
			return m, nil
		}

		// Handle clicks in left panel to select agents
		if inLeftPanel && msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionRelease {
			// Calculate which agent was clicked
			// Subtract 1 for top border
			clickY := msg.Y - 1

			// Account for top scroll indicator if present
			if m.sidebarOffset > 0 {
				clickY-- // First line is the "N more" indicator
			}

			// Convert to agent index, but only within visible range
			visibleLines := m.sidebarVisibleLines()
			if clickY >= 0 && clickY < visibleLines {
				agentIdx := m.sidebarOffset + clickY
				if agentIdx >= 0 && agentIdx < len(m.agents) {
					m.selectedIdx = agentIdx
					if agent := m.SelectedAgent(); agent != nil {
						cmds = append(cmds, loadTranscriptCmd(*agent))
					}
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateViewportDimensions()
		m.updateViewport()

	case tickMsg:
		cmds = append(cmds, tickCmd(), scanAgentsCmd(m.projectDir))

	case agentsMsg:
		prevLen := len(m.agents)
		m.agents = msg
		if prevLen == 0 && len(m.agents) > 0 {
			m.selectedIdx = 0
		}
		if agent := m.SelectedAgent(); agent != nil {
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
	agent := m.SelectedAgent()
	if agent == nil {
		m.viewport.SetContent("")
		return
	}
	entries := m.transcripts[agent.ID]
	m.viewport.SetContent(formatTranscript(entries, m.viewport.Width))
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

	// Right panel: transcript
	var rightTitle string
	if agent := m.SelectedAgent(); agent != nil {
		rightTitle = fmt.Sprintf("%s | %s", agent.ID, formatTimestamp(agent.LastMod))
	}
	rightPanel := renderPanelWithTitle(rightTitle, m.viewport.View(), rightWidth, panelHeight, rightBorderColor)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	// Status bar
	status := statusBarStyle.Render("Tab: switch | j/k: navigate | u/d: page | g/G: top/bottom | q: quit")

	return lipgloss.JoinVertical(lipgloss.Left, panels, status)
}

func (m Model) renderSidebarContent(width, height int) string {
	if len(m.agents) == 0 || height <= 0 {
		return "No agents found"
	}

	// Calculate how many agents are hidden above and below
	hiddenAbove := m.sidebarOffset
	totalAgents := len(m.agents)

	// Calculate available lines for agents (reserve lines for indicators if needed)
	availableLines := height
	showTopIndicator := hiddenAbove > 0
	if showTopIndicator {
		availableLines--
	}

	// Calculate how many agents we can show
	visibleEnd := m.sidebarOffset + availableLines
	if visibleEnd > totalAgents {
		visibleEnd = totalAgents
	}

	hiddenBelow := totalAgents - visibleEnd
	showBottomIndicator := hiddenBelow > 0
	if showBottomIndicator && availableLines > 0 {
		availableLines--
		visibleEnd = m.sidebarOffset + availableLines
		if visibleEnd > totalAgents {
			visibleEnd = totalAgents
		}
		hiddenBelow = totalAgents - visibleEnd
	}

	var lines []string

	// Top scroll indicator
	if showTopIndicator {
		indicator := fmt.Sprintf("\u2191 %d more", hiddenAbove)
		if len(indicator) > width {
			indicator = indicator[:width]
		}
		lines = append(lines, doneStyle.Render(indicator))
	}

	// Render visible agents
	for i := m.sidebarOffset; i < visibleEnd; i++ {
		agent := m.agents[i]

		// Determine indicator (without styling for selected row)
		var indicator string
		indicatorChar := "\u2713"
		if agent.IsActive() {
			indicatorChar = "\u25cf"
		}

		name := agent.ID
		maxNameLen := width - 3 // indicator + space + name
		if len(name) > maxNameLen {
			name = name[:maxNameLen]
		}

		if i == m.selectedIdx {
			// For selected row: apply background to entire row but keep indicator color
			var styledIndicator string
			if agent.IsActive() {
				styledIndicator = activeStyle.Background(lipgloss.Color("8")).Render(indicatorChar)
			} else {
				styledIndicator = doneStyle.Background(lipgloss.Color("8")).Render(indicatorChar)
			}
			// Build the rest with background
			rest := fmt.Sprintf(" %s", name)
			if len(indicatorChar)+len(rest) < width {
				rest = rest + strings.Repeat(" ", width-len(indicatorChar)-len(rest))
			}
			lines = append(lines, styledIndicator+selectedBgStyle.Render(rest))
		} else {
			// For non-selected: apply color to indicator only
			if agent.IsActive() {
				indicator = activeStyle.Render(indicatorChar)
			} else {
				indicator = doneStyle.Render(indicatorChar)
			}
			lines = append(lines, fmt.Sprintf("%s %s", indicator, name))
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

// renderPanelWithTitle renders a panel with the title embedded in the top border
// like: ╭─ Title ────────╮
// This is copied from the old working TUI code.
func renderPanelWithTitle(title, content string, width, height int, borderColor lipgloss.Color) string {
	// Border characters (rounded)
	topLeft := "╭"
	topRight := "╮"
	bottomLeft := "╰"
	bottomRight := "╯"
	horizontal := "─"
	vertical := "│"

	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	titleStyle := lipgloss.NewStyle().Foreground(borderColor).Bold(true)

	// Content width is width - 2 (for left and right borders)
	contentWidth := width - 2
	if contentWidth < 0 {
		contentWidth = 0
	}

	// Build top border with embedded title
	var topBorder string
	if title == "" {
		topBorder = borderStyle.Render(topLeft + strings.Repeat(horizontal, contentWidth) + topRight)
	} else {
		titleText := " " + title + " "
		titleLen := len([]rune(titleText))

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

	// Render middle lines with side borders
	var middleLines []string
	for i := 0; i < contentLines; i++ {
		var line string
		if i < len(lines) {
			line = lines[i]
		}
		// Use lipgloss.Width for visual width (handles ANSI codes)
		visualWidth := lipgloss.Width(line)
		if visualWidth < contentWidth {
			line = line + strings.Repeat(" ", contentWidth-visualWidth)
		} else if visualWidth > contentWidth {
			// Truncate line to fit
			runes := []rune(line)
			for len(runes) > 0 && lipgloss.Width(string(runes)) > contentWidth {
				runes = runes[:len(runes)-1]
			}
			line = string(runes)
			// Pad if truncation left us short
			if lipgloss.Width(line) < contentWidth {
				line = line + strings.Repeat(" ", contentWidth-lipgloss.Width(line))
			}
		}
		middleLines = append(middleLines, borderStyle.Render(vertical)+line+borderStyle.Render(vertical))
	}

	// Join all parts
	allLines := []string{topBorder}
	allLines = append(allLines, middleLines...)
	allLines = append(allLines, bottomBorder)

	return strings.Join(allLines, "\n")
}

func formatTranscript(entries []claude.Entry, width int) string {
	var lines []string

	for _, e := range entries {
		switch e.Type {
		case "user":
			if e.IsToolResult() {
				continue
			}
			content := strings.TrimSpace(e.TextContent())
			if content != "" {
				// Style only the actual text, then add a reset to prevent background bleed
				styled := promptStyle.Render("> " + content)
				lines = append(lines, styled+"\033[0m")
				lines = append(lines, "")
			}
		case "assistant":
			if tool := e.ToolName(); tool != "" {
				lines = append(lines, toolStyle.Render("  "+tool))
			} else if text := e.TextContent(); text != "" {
				rendered := renderMarkdown(text, width)
				lines = append(lines, rendered)
			}
		}
	}
	return strings.Join(lines, "\n")
}

// renderMarkdown renders markdown text using glamour with a dark terminal style.
func renderMarkdown(text string, width int) string {
	if width <= 0 {
		width = 80 // default width
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
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
