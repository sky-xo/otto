package tui

import (
	"database/sql"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"otto/internal/process"
	"otto/internal/repo"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styling
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12"))

	panelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("6")).
			Background(lipgloss.Color("0")).
			Padding(0, 1)

	messageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))

	agentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10"))

	statusActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("10")).
				Bold(true)

	statusPendingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("11"))

	statusCompleteStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("8"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true)
)

// usernameColor returns a consistent ANSI color for a given username
// using hash-based selection from a palette
func usernameColor(name string) lipgloss.Color {
	// Palette: cyan (6), green (2), yellow (3), magenta (5), blue (4), red (1)
	palette := []string{"6", "2", "3", "5", "4", "1"}

	// Hash the name
	h := fnv.New32a()
	h.Write([]byte(name))
	hash := h.Sum32()

	// Pick a color from the palette
	return lipgloss.Color(palette[hash%uint32(len(palette))])
}

type tickMsg time.Time
type messagesMsg []repo.Message
type agentsMsg []repo.Agent

type model struct {
	db           *sql.DB
	messages     []repo.Message
	agents       []repo.Agent
	lastSeenID   string
	scrollOffset int
	width        int
	height       int
	err          error
}

func NewModel(db *sql.DB) model {
	return model{
		db:       db,
		messages: []repo.Message{},
		agents:   []repo.Agent{},
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		fetchMessagesCmd(m.db, ""),
		fetchAgentsCmd(m.db),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
		case "down", "j":
			maxScroll := len(m.messages) - (m.height - 10) // Leave room for UI chrome
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.scrollOffset < maxScroll {
				m.scrollOffset++
			}
		case "g":
			// Go to top
			m.scrollOffset = 0
		case "G":
			// Go to bottom
			maxScroll := len(m.messages) - (m.height - 10)
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.scrollOffset = maxScroll
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		return m, tea.Batch(
			tickCmd(),
			cleanupStaleAgentsCmd(m.db),
			fetchMessagesCmd(m.db, m.lastSeenID),
			fetchAgentsCmd(m.db),
		)

	case messagesMsg:
		// Append new messages
		if len(msg) > 0 {
			m.messages = append(m.messages, msg...)
			// Update lastSeenID to the last message
			m.lastSeenID = msg[len(msg)-1].ID
			// Auto-scroll to bottom on new messages
			maxScroll := len(m.messages) - (m.height - 10)
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.scrollOffset = maxScroll
		}

	case agentsMsg:
		m.agents = msg

	case error:
		m.err = msg
	}

	return m, nil
}

func (m model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	// Title
	title := titleStyle.Render("Otto Watch (TUI Mode)")

	// Calculate dimensions for panels
	leftWidth := m.width*2/3 - 2
	rightWidth := m.width - leftWidth - 4

	// Left panel: Messages
	messagesTitle := panelTitleStyle.Width(leftWidth).Render("Messages")
	messagesContent := m.renderMessages(leftWidth, m.height-6)

	// Right panel: Agents
	agentsTitle := panelTitleStyle.Width(rightWidth).Render("Agents")
	agentsContent := m.renderAgents(rightWidth, m.height-6)

	// Create panel borders
	leftPanel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(leftWidth).
		Height(m.height - 4).
		Render(messagesTitle + "\n" + messagesContent)

	rightPanel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(rightWidth).
		Height(m.height - 4).
		Render(agentsTitle + "\n" + agentsContent)

	// Combine panels side by side
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	// Help text
	help := helpStyle.Render("q: quit | ↑/k: scroll up | ↓/j: scroll down | g: top | G: bottom")

	// Combine everything
	return lipgloss.JoinVertical(lipgloss.Left, title, panels, help)
}

func (m model) renderMessages(width, height int) string {
	if len(m.messages) == 0 {
		return messageStyle.Render("Waiting for messages...")
	}

	var lines []string
	visibleHeight := height - 2 // Account for title and padding

	// Determine which messages to show based on scroll
	start := m.scrollOffset
	end := m.scrollOffset + visibleHeight
	if end > len(m.messages) {
		end = len(m.messages)
	}
	if start >= len(m.messages) {
		start = len(m.messages) - 1
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		msg := m.messages[i]

		// Create username style with hash-based color and bold
		usernameStyle := lipgloss.NewStyle().
			Foreground(usernameColor(msg.FromID)).
			Bold(true)

		var content string
		var contentStyle lipgloss.Style

		// Format and style based on message type
		switch msg.Type {
		case "exit":
			content = fmt.Sprintf("exited – %s", msg.Content)
			contentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		case "complete":
			content = fmt.Sprintf("completed – %s", msg.Content)
			contentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		case "error":
			content = msg.Content
			contentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
		case "say", "ask":
			content = msg.Content
			contentStyle = messageStyle
		default:
			// For other types (task, etc.), show them normally
			content = msg.Content
			contentStyle = messageStyle
		}

		// Build the line: username + space + content
		line := usernameStyle.Render(msg.FromID) + " " + contentStyle.Render(content)

		// Truncate if too long (account for ANSI codes by using visual length approximation)
		// We'll use a simple heuristic: username + content should fit
		maxContentLen := width - len(msg.FromID) - 6 // Account for spacing and border
		if len(content) > maxContentLen && maxContentLen > 3 {
			content = content[:maxContentLen-3] + "..."
			line = usernameStyle.Render(msg.FromID) + " " + contentStyle.Render(content)
		}

		lines = append(lines, line)
	}

	// Add scroll indicator if needed
	if len(m.messages) > visibleHeight {
		scrollInfo := fmt.Sprintf("(%d-%d of %d)", start+1, end, len(m.messages))
		lines = append([]string{helpStyle.Render(scrollInfo)}, lines...)
	}

	return strings.Join(lines, "\n")
}

func (m model) renderAgents(width, height int) string {
	if len(m.agents) == 0 {
		return agentStyle.Render("No agents")
	}

	var lines []string

	// Add summary counts
	active, pending, complete := 0, 0, 0
	for _, a := range m.agents {
		switch a.Status {
		case "active":
			active++
		case "pending":
			pending++
		case "complete":
			complete++
		}
	}

	summary := fmt.Sprintf("Total: %d | Active: %s | Pending: %s | Complete: %s",
		len(m.agents),
		statusActiveStyle.Render(fmt.Sprintf("%d", active)),
		statusPendingStyle.Render(fmt.Sprintf("%d", pending)),
		statusCompleteStyle.Render(fmt.Sprintf("%d", complete)),
	)
	lines = append(lines, summary, "")

	// List agents
	for _, agent := range m.agents {
		// Format agent info
		task := agent.Task
		maxLen := width - 15 // Leave room for ID and status
		if len(task) > maxLen {
			task = task[:maxLen-3] + "..."
		}

		var statusStyled string
		switch agent.Status {
		case "active":
			statusStyled = statusActiveStyle.Render(agent.Status)
		case "pending":
			statusStyled = statusPendingStyle.Render(agent.Status)
		case "complete":
			statusStyled = statusCompleteStyle.Render(agent.Status)
		default:
			statusStyled = agent.Status
		}

		line := fmt.Sprintf("%s [%s]", agentStyle.Render(agent.ID), statusStyled)
		if agent.Task != "" {
			line += "\n  " + task
		}

		lines = append(lines, line)

		// Stop if we're running out of space
		if len(lines) >= height-4 {
			break
		}
	}

	return strings.Join(lines, "\n")
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchMessagesCmd(db *sql.DB, sinceID string) tea.Cmd {
	return func() tea.Msg {
		filter := repo.MessageFilter{}
		if sinceID != "" {
			filter.SinceID = sinceID
		}
		messages, err := repo.ListMessages(db, filter)
		if err != nil {
			return err
		}
		return messagesMsg(messages)
	}
}

func fetchAgentsCmd(db *sql.DB) tea.Cmd {
	return func() tea.Msg {
		agents, err := repo.ListAgents(db)
		if err != nil {
			return err
		}
		return agentsMsg(agents)
	}
}

func cleanupStaleAgentsCmd(db *sql.DB) tea.Cmd {
	return func() tea.Msg {
		agents, err := repo.ListAgents(db)
		if err != nil {
			return nil
		}
		for _, a := range agents {
			if a.Status == "working" && a.Pid.Valid {
				if !process.IsProcessAlive(int(a.Pid.Int64)) {
					// Post exit message and delete agent
					msg := repo.Message{
						ID:           fmt.Sprintf("%s-exit-%d", a.ID, time.Now().Unix()),
						FromID:       a.ID,
						Type:         "exit",
						Content:      "EXITED: process died unexpectedly",
						MentionsJSON: "[]",
						ReadByJSON:   "[]",
					}
					_ = repo.CreateMessage(db, msg)
					_ = repo.DeleteAgent(db, a.ID)
				}
			}
		}
		return nil
	}
}

// Run starts the TUI
func Run(db *sql.DB) error {
	m := NewModel(db)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
