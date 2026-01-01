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

const sidebarWidth = 20

var (
	activeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))           // green
	doneStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))            // gray
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")) // cyan
	promptStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	toolStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
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
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Viewport takes remaining width minus sidebar, separator, and padding
		m.viewport.Width = msg.Width - sidebarWidth - 3
		m.viewport.Height = msg.Height - 3 // Account for header and separator
		m.updateViewport()

	case tickMsg:
		cmds = append(cmds, tickCmd(), scanAgentsCmd(m.projectDir))

	case agentsMsg:
		prevLen := len(m.agents)
		m.agents = msg
		// On first load, ensure selectedIdx is 0 and load transcript
		if prevLen == 0 && len(m.agents) > 0 {
			m.selectedIdx = 0
		}
		// Load transcript for selected agent
		if agent := m.SelectedAgent(); agent != nil {
			cmds = append(cmds, loadTranscriptCmd(*agent))
		}

	case transcriptMsg:
		m.transcripts[msg.agentID] = msg.entries
		m.updateViewport()

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

func (m *Model) updateViewport() {
	agent := m.SelectedAgent()
	if agent == nil {
		m.viewport.SetContent("")
		return
	}
	entries := m.transcripts[agent.ID]
	m.viewport.SetContent(formatTranscript(entries))
}

// View renders the UI.
func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}

	// Handle zero dimensions (before first WindowSizeMsg)
	if m.width < 40 || m.height < 10 {
		return "Loading..."
	}

	// Calculate panel dimensions
	contentWidth := m.width - sidebarWidth - 1
	if contentWidth < 10 {
		contentWidth = 10
	}

	// Left panel: agent list
	sidebar := m.renderSidebar()

	// Separator
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	separator := strings.Repeat(sepStyle.Render("│")+"\n", m.height)

	// Right panel: transcript
	var title string
	if agent := m.SelectedAgent(); agent != nil {
		title = agent.ID
	}

	// Build content panel
	content := m.renderContentPanel(title, contentWidth)

	// Combine horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, separator, content)
}

func (m Model) renderSidebar() string {
	var lines []string

	// Header
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	lines = append(lines, headerStyle.Render("Subagents"))
	lines = append(lines, strings.Repeat("─", sidebarWidth-2))

	// Agent list
	for i, agent := range m.agents {
		var indicator string
		if agent.IsActive() {
			indicator = activeStyle.Render("●")
		} else {
			indicator = doneStyle.Render("✓")
		}

		name := agent.ID
		if len(name) > sidebarWidth-4 {
			name = name[:sidebarWidth-4]
		}

		line := fmt.Sprintf("%s %s", indicator, name)
		if i == m.selectedIdx {
			line = selectedStyle.Render(line)
		}
		lines = append(lines, line)
	}

	// Pad to height
	for len(lines) < m.height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderContentPanel(title string, width int) string {
	var lines []string

	// Header with title
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	if title != "" {
		lines = append(lines, headerStyle.Render(title))
	} else {
		lines = append(lines, headerStyle.Render("Transcript"))
	}
	lines = append(lines, strings.Repeat("─", width-2))

	// Viewport content
	viewportLines := strings.Split(m.viewport.View(), "\n")
	lines = append(lines, viewportLines...)

	// Pad to height
	for len(lines) < m.height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func formatTranscript(entries []claude.Entry) string {
	var lines []string

	for _, e := range entries {
		switch e.Type {
		case "user":
			if e.IsToolResult() {
				// Skip tool results in display (too verbose)
				continue
			}
			content := e.TextContent()
			if content != "" {
				// Show first 200 chars of prompt
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
				// Show first 500 chars of response
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
