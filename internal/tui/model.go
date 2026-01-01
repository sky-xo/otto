// internal/tui/model.go
package tui

import (
	"fmt"
	"strings"

	"june/internal/claude"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const sidebarWidth = 23

var (
	activeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // green
	doneStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))  // gray
	selectedBgStyle = lipgloss.NewStyle().Background(lipgloss.Color("8")) // highlighted background
	promptStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	toolStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	statusBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	focusedBorderColor   = lipgloss.Color("6") // Cyan
	unfocusedBorderColor = lipgloss.Color("8") // Dim
)

// Model is the TUI state.
type Model struct {
	projectDir  string                    // Claude project directory we're watching
	agents      []claude.Agent            // List of agents
	transcripts map[string][]claude.Entry // Agent ID -> transcript entries

	selectedIdx int // Currently selected agent index
	width       int
	height      int
	viewport    viewport.Model
	err         error
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
		case "up", "k":
			if m.selectedIdx > 0 {
				m.selectedIdx--
				if agent := m.SelectedAgent(); agent != nil {
					cmds = append(cmds, loadTranscriptCmd(*agent))
				}
			}
		case "down", "j":
			if m.selectedIdx < len(m.agents)-1 {
				m.selectedIdx++
				if agent := m.SelectedAgent(); agent != nil {
					cmds = append(cmds, loadTranscriptCmd(*agent))
				}
			}
		case "g":
			m.viewport.GotoTop()
		case "G":
			m.viewport.GotoBottom()
		}

	case tea.MouseMsg:
		leftWidth, _, _, _ := m.layout()
		inLeftPanel := msg.X < leftWidth

		// Handle scroll wheel in right panel
		if !inLeftPanel && msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.viewport.LineUp(3)
				return m, nil
			case tea.MouseButtonWheelDown:
				m.viewport.LineDown(3)
				return m, nil
			}
		}

		// Handle clicks in left panel to select agents
		if inLeftPanel && msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionRelease {
			// Calculate which agent was clicked
			// Subtract 1 for top border
			clickedIndex := msg.Y - 1
			if clickedIndex >= 0 && clickedIndex < len(m.agents) {
				m.selectedIdx = clickedIndex
				if agent := m.SelectedAgent(); agent != nil {
					cmds = append(cmds, loadTranscriptCmd(*agent))
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
		m.transcripts[msg.agentID] = msg.entries
		m.updateViewport()
		// Scroll to bottom when transcript loads
		m.viewport.GotoBottom()

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
	m.viewport.SetContent(formatTranscript(entries))
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

	// Left panel: agent list
	leftContent := m.renderSidebarContent(leftWidth-2, contentHeight)
	leftPanel := renderPanelWithTitle("Subagents", leftContent, leftWidth, panelHeight, unfocusedBorderColor)

	// Right panel: transcript
	var rightTitle string
	if agent := m.SelectedAgent(); agent != nil {
		rightTitle = agent.ID
	}
	rightPanel := renderPanelWithTitle(rightTitle, m.viewport.View(), rightWidth, panelHeight, focusedBorderColor)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	// Status bar
	status := statusBarStyle.Render("j/k: navigate | g/G: top/bottom | q: quit")

	return lipgloss.JoinVertical(lipgloss.Left, panels, status)
}

func (m Model) renderSidebarContent(width, height int) string {
	if len(m.agents) == 0 || height <= 0 {
		return "No agents found"
	}

	var lines []string
	for i, agent := range m.agents {
		if len(lines) >= height {
			break
		}

		var indicator string
		if agent.IsActive() {
			indicator = activeStyle.Render("●")
		} else {
			indicator = doneStyle.Render("✓")
		}

		name := agent.ID
		maxNameLen := width - 3 // indicator + space + name
		if len(name) > maxNameLen {
			name = name[:maxNameLen]
		}

		// Build the line content
		lineContent := fmt.Sprintf("%s %s", indicator, name)

		// For selected item, pad to full width and apply background
		if i == m.selectedIdx {
			// Pad to full width so background spans entire row
			visualWidth := lipgloss.Width(lineContent)
			if visualWidth < width {
				lineContent = lineContent + strings.Repeat(" ", width-visualWidth)
			}
			lineContent = selectedBgStyle.Render(lineContent)
		}

		lines = append(lines, lineContent)
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

func formatTranscript(entries []claude.Entry) string {
	var lines []string

	for _, e := range entries {
		switch e.Type {
		case "user":
			if e.IsToolResult() {
				continue
			}
			content := e.TextContent()
			if content != "" {
				if len(content) > 200 {
					content = content[:200] + "..."
				}
				lines = append(lines, promptStyle.Render("> "+content))
				lines = append(lines, "")
			}
		case "assistant":
			if tool := e.ToolName(); tool != "" {
				lines = append(lines, toolStyle.Render("  "+tool))
			} else if text := e.TextContent(); text != "" {
				if len(text) > 500 {
					text = text[:500] + "..."
				}
				lines = append(lines, text)
				lines = append(lines, "")
			}
		}
	}
	return strings.Join(lines, "\n")
}
