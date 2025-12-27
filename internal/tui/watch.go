package tui

import (
	"database/sql"
	"fmt"
	"hash/fnv"
	"os/exec"
	"sort"
	"strings"
	"time"

	"otto/internal/process"
	"otto/internal/repo"
	"otto/internal/scope"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	mainChannelID    = "main"
	archivedChannelID = "__archived__"
	channelListWidth = 20

	// Panel focus indices (future-proof for 3-panel layout)
	panelAgents   = 0 // Left: channel/agent list
	panelMessages = 1 // Right: content/messages
	// panelTodos = 2  // Future: todo list
)

// Styling
var (
	panelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("6")).
			Background(lipgloss.Color("0")).
			Padding(0, 1)

	messageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))

	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	channelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))

	channelActiveStyle = lipgloss.NewStyle().
				Bold(true)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	statusBusyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10"))

	statusBlockedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("11"))

	statusCompleteStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("8"))

	statusFailedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("1"))

	inputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("7")).
				Background(lipgloss.Color("235"))

	focusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("6")) // Cyan

	unfocusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("8")) // Dim
)

// usernameColor returns a consistent ANSI color for a given username
// using hash-based selection from a palette
func usernameColor(name string) lipgloss.Color {
	// Palette: cyan (6), green (2), yellow (3), magenta (5), blue (4), red (1)
	palette := []string{"6", "2", "3", "5", "4", "1"}

	// Hash the name
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	hash := h.Sum32()

	// Pick a color from the palette
	return lipgloss.Color(palette[hash%uint32(len(palette))])
}

type tickMsg time.Time
type messagesMsg []repo.Message
type agentsMsg []repo.Agent

type transcriptsMsg struct {
	agentID string
	entries []repo.LogEntry
}

type channel struct {
	ID      string
	Name    string
	Kind    string
	Status  string
	Level   int
	Project string
	Branch  string
}

type model struct {
	db                *sql.DB
	messages          []repo.Message
	agents            []repo.Agent
	transcripts       map[string][]repo.LogEntry
	lastMessageID     string
	lastTranscriptIDs map[string]string
	width             int
	height            int
	cursorIndex       int
	activeChannelID   string
	archivedExpanded  bool
	projectExpanded   map[string]bool // Tracks expanded/collapsed state for project/branch headers
	focusedPanel      int             // Panel index (panelAgents, panelMessages, etc.)
	mouseX            int             // Mouse X position for hover detection
	mouseY            int             // Mouse Y position for hover detection
	err               error
	viewport          viewport.Model
	chatInput         textinput.Model // Text input for sending messages to @otto
}

func NewModel(db *sql.DB) model {
	vp := viewport.New(0, 0)
	ti := textinput.New()
	ti.Placeholder = "Message @otto..."
	ti.CharLimit = 500
	ti.Width = 50

	return model{
		db:                db,
		messages:          []repo.Message{},
		agents:            []repo.Agent{},
		transcripts:       map[string][]repo.LogEntry{},
		lastTranscriptIDs: map[string]string{},
		activeChannelID:   mainChannelID,
		archivedExpanded:  false,
		projectExpanded:   map[string]bool{}, // Default: all projects expanded
		focusedPanel:      panelMessages,      // Default to content panel
		viewport:          vp,
		chatInput:         ti,
	}
}

func (m model) Init() tea.Cmd {
	ctx := scope.CurrentContext()
	return tea.Batch(
		tickCmd(),
		fetchMessagesCmd(m.db, ctx.Project, ctx.Branch, ""),
		fetchAgentsCmd(m.db),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If chat input is visible and focused, handle input
		if m.showChatInput() && m.focusedPanel == panelMessages {
			// Check if otto is busy first
			project, branch := parseProjectBranch(m.activeChannelID)
			ottoBusy := m.isOttoBusy(project, branch)

			switch msg.String() {
			case "enter":
				// Submit message (blocked if otto is busy)
				if !ottoBusy {
					cmd := m.handleChatSubmit()
					return m, cmd
				}
			case "esc":
				// Clear input and blur
				m.chatInput.SetValue("")
				m.chatInput.Blur()
			default:
				// Pass to textinput only if otto is not busy
				if !ottoBusy {
					m.chatInput, cmd = m.chatInput.Update(msg)
					return m, cmd
				}
			}
		}

		// Global key handling
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			// Toggle focus between panels
			if m.focusedPanel == panelAgents {
				m.focusedPanel = panelMessages
				// Focus chat input if visible
				if m.showChatInput() {
					m.chatInput.Focus()
				}
			} else {
				m.focusedPanel = panelAgents
				m.chatInput.Blur()
			}
		case "up", "k":
			if m.focusedPanel == panelAgents {
				cmd = m.moveCursor(-1)
				return m, cmd
			} else {
				m.viewport.LineUp(1)
			}
		case "down", "j":
			if m.focusedPanel == panelAgents {
				cmd = m.moveCursor(1)
				return m, cmd
			} else {
				m.viewport.LineDown(1)
			}
		case "enter":
			// Enter works for toggling archived section in agent panel
			if m.focusedPanel == panelAgents {
				return m, m.activateSelection()
			}
		case "esc":
			m.activeChannelID = mainChannelID
			m.cursorIndex = 0
			m.focusedPanel = panelMessages
			m.chatInput.Blur()
			m.updateViewportContent()
		case "g":
			if m.focusedPanel == panelMessages {
				m.viewport.GotoTop()
			}
		case "G":
			if m.focusedPanel == panelMessages {
				m.viewport.GotoBottom()
			}
		default:
			// Pass other keys to viewport for scrolling (pgup, pgdn, etc)
			if m.focusedPanel == panelMessages {
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}
		}

	case tea.MouseMsg:
		// Track mouse position for hover detection
		m.mouseX, m.mouseY = msg.X, msg.Y

		// Get layout to determine panel boundaries
		leftWidth, _, _, _ := m.layout()
		inContentPanel := msg.X > leftWidth

		// Handle scroll wheel events
		if inContentPanel && msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.viewport.LineUp(3)
			case tea.MouseButtonWheelDown:
				m.viewport.LineDown(3)
			}
		}

		// Click - select agent if clicking in agent list
		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionRelease {
			if !inContentPanel {
				// Calculate which agent was clicked based on msg.Y
				// Subtract 2 for border + title row
				clickedIndex := msg.Y - 2
				channels := m.channels()
				if clickedIndex >= 0 && clickedIndex < len(channels) {
					m.cursorIndex = clickedIndex
					return m, m.activateSelection()
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update viewport dimensions
		_, rightWidth, _, contentHeight := m.layout()
		m.viewport.Width = rightWidth - 2 // Account for border
		m.viewport.Height = contentHeight

		// Update chat input width
		m.chatInput.Width = rightWidth - 4 // Account for border + some padding

		// Update viewport content with new dimensions
		m.updateViewportContent()

	case tickMsg:
		// Determine which project/branch to fetch messages for
		var project, branch string
		if isProjectHeader(m.activeChannelID) {
			// If a project header is selected, use its project/branch
			project, branch = parseProjectBranch(m.activeChannelID)
		} else {
			// Otherwise, use the current git context
			ctx := scope.CurrentContext()
			project = ctx.Project
			branch = ctx.Branch
		}

		cmds := []tea.Cmd{
			tickCmd(),
			cleanupStaleAgentsCmd(m.db),
			fetchMessagesCmd(m.db, project, branch, m.lastMessageID),
			fetchAgentsCmd(m.db),
		}
		if m.activeChannelID != mainChannelID && !isProjectHeader(m.activeChannelID) {
			cmds = append(cmds, fetchTranscriptsCmd(m.db, m.activeChannelID, m.lastTranscriptIDs[m.activeChannelID]))
		}
		return m, tea.Batch(cmds...)

	case messagesMsg:
		if len(msg) > 0 {
			atBottom := m.viewport.AtBottom()
			m.messages = append(m.messages, msg...)
			m.lastMessageID = msg[len(msg)-1].ID
			// Update viewport if we're viewing orchestrator chat (Main or a project header)
			showingOrchestratorChat := m.activeChannelID == mainChannelID || isProjectHeader(m.activeChannelID)
			if atBottom && showingOrchestratorChat {
				m.updateViewportContent()
				m.viewport.GotoBottom()
			} else if showingOrchestratorChat {
				m.updateViewportContent()
			}
		}

	case transcriptsMsg:
		if len(msg.entries) > 0 {
			current := m.transcripts[msg.agentID]
			atBottom := m.viewport.AtBottom()
			m.transcripts[msg.agentID] = append(current, msg.entries...)
			m.lastTranscriptIDs[msg.agentID] = msg.entries[len(msg.entries)-1].ID
			if atBottom && m.activeChannelID == msg.agentID {
				m.updateViewportContent()
				m.viewport.GotoBottom()
			} else if m.activeChannelID == msg.agentID {
				m.updateViewportContent()
			}
		}

	case agentsMsg:
		m.agents = msg
		m.ensureSelection()

	case error:
		m.err = msg
	}

	return m, cmd
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	leftWidth, rightWidth, panelHeight, contentHeight := m.layout()

	// Left panel: Channels
	channelsTitle := panelTitleStyle.Width(leftWidth - 2).Render("Channels")
	channelsContent := m.renderChannels(leftWidth-2, contentHeight)

	leftBorderStyle := unfocusedBorderStyle
	if m.focusedPanel == panelAgents {
		leftBorderStyle = focusedBorderStyle
	}
	leftPanel := leftBorderStyle.
		Width(leftWidth).
		Height(panelHeight).
		Render(channelsTitle + "\n" + channelsContent)

	// Right panel: Content using viewport
	activeLabel := m.activeChannelLabel()
	rightTitle := panelTitleStyle.Width(rightWidth - 2).Render(activeLabel)

	// Build right panel content
	var rightContent string
	if m.showChatInput() {
		// Chat input is visible - render viewport + input
		project, branch := parseProjectBranch(m.activeChannelID)

		// Render chat input with status hint
		var inputLine string
		if m.isOttoBusy(project, branch) {
			// Otto is busy - show grayed input with hint
			disabledInputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
			disabledInput := m.chatInput.View()
			hint := " (Otto is working...)"
			inputLine = disabledInputStyle.Render(disabledInput + hint)
		} else {
			// Normal input
			inputLine = m.chatInput.View()
		}

		rightContent = m.viewport.View() + "\n" + inputLine
	} else {
		// No chat input - just viewport
		rightContent = m.viewport.View()
	}

	rightBorderStyle := unfocusedBorderStyle
	if m.focusedPanel == panelMessages {
		rightBorderStyle = focusedBorderStyle
	}
	rightPanel := rightBorderStyle.
		Width(rightWidth).
		Height(panelHeight).
		Render(rightTitle + "\n" + rightContent)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	// Status bar with error display
	statusText := "Tab: switch panel | j/k: navigate/scroll | Enter: select | Esc: Main | g/G: top/bottom | q: quit"
	if m.err != nil {
		statusText = statusFailedStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}
	status := statusBarStyle.Render(statusText)

	return lipgloss.JoinVertical(lipgloss.Left, panels, status)
}

func (m model) renderChannels(width, height int) string {
	channels := m.channels()
	if len(channels) == 0 || height <= 0 {
		return ""
	}

	lines := make([]string, 0, len(channels))
	for i, ch := range channels {
		line := m.renderChannelLine(ch, width, i == m.cursorIndex, ch.ID == m.activeChannelID)
		lines = append(lines, line)
		if len(lines) >= height {
			break
		}
	}
	return strings.Join(lines, "\n")
}

func (m model) mainContentLines(width int) []string {
	if len(m.messages) == 0 {
		return []string{messageStyle.Render("Waiting for messages...")}
	}

	fromWidth := 12
	if width < fromWidth+6 {
		fromWidth = clamp(width/3, 6, fromWidth)
	}
	contentWidth := width - fromWidth - 1
	if contentWidth < 1 {
		contentWidth = 1
	}

	lines := make([]string, 0, len(m.messages))
	for _, msg := range m.messages {
		fromStyle := lipgloss.NewStyle().Foreground(usernameColor(msg.FromAgent)).Bold(true)
		fromText := padRight(fromStyle.Render(msg.FromAgent), fromWidth)

		content, style := formatMessage(msg)

		// For prompts with a target agent, prepend styled @mention
		var mentionPrefix string
		if msg.Type == "prompt" && msg.ToAgent.Valid {
			mentionStyle := lipgloss.NewStyle().Foreground(usernameColor(msg.ToAgent.String)).Bold(true)
			mentionPrefix = mentionStyle.Render("@"+msg.ToAgent.String) + " "
		}

		// Wrap long lines to prevent overflow
		contentLines := wrapText(content, contentWidth)
		for i, line := range contentLines {
			var displayLine string
			if i == 0 {
				// First line: show "from" prefix and optional @mention
				displayLine = fromText + " " + mentionPrefix + style.Render(line)
			} else {
				// Continuation lines: indent to align with content
				displayLine = strings.Repeat(" ", fromWidth+1) + style.Render(line)
			}
			lines = append(lines, displayLine)
		}
	}
	return lines
}

func (m model) transcriptContentLines(agentID string, width int) []string {
	entries := m.transcripts[agentID]
	if len(entries) == 0 {
		return []string{messageStyle.Render("No transcript entries yet.")}
	}

	prefixWidth := 3
	contentWidth := width - prefixWidth - 1
	if contentWidth < 1 {
		contentWidth = 1
	}

	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		prefix, prefixStyle := transcriptPrefix(entry)

		// Determine content to display
		content := entry.Content.String
		if entry.EventType == "command_execution" {
			// Show command only, not output
			content = entry.Command.String
		}

		// Determine if content should be dimmed (reasoning and commands fade into background)
		isDimmed := entry.EventType == "reasoning" || entry.EventType == "thinking" ||
			entry.EventType == "command_execution" || entry.EventType == "tool_call" ||
			entry.EventType == "tool_result"

		// Special handling for input - full background, preserve newlines
		if entry.EventType == "input" {
			inputWidth := width - 4 // leave some margin
			if inputWidth < 1 {
				inputWidth = 1
			}
			// Split on newlines first to preserve paragraph structure
			paragraphs := strings.Split(content, "\n")
			firstLine := true
			for _, para := range paragraphs {
				if para == "" {
					// Empty line - render as blank with background
					padded := strings.Repeat(" ", width)
					lines = append(lines, inputStyle.Render(padded))
					continue
				}
				wrappedLines := wrapText(para, inputWidth)
				for _, line := range wrappedLines {
					var displayLine string
					if firstLine {
						displayLine = prefix + " " + line
						firstLine = false
					} else {
						// Continuation: small indent, flows naturally
						displayLine = "  " + line
					}
					// Apply inputStyle to entire line, pad to width
					padded := displayLine + strings.Repeat(" ", width-lipgloss.Width(displayLine))
					lines = append(lines, inputStyle.Render(padded))
				}
			}
			lines = append(lines, "") // blank line after entry
			continue
		}

		// Wrap long lines to prevent overflow
		contentLines := wrapText(content, contentWidth)
		for i, line := range contentLines {
			var displayLine string
			if i == 0 {
				if isDimmed {
					// For dimmed entries, build line plain then style all at once
					// (can't nest styles - ANSI codes from first render persist)
					plainPrefix := padRight(prefix, prefixWidth)
					displayLine = mutedStyle.Render(plainPrefix + " " + line)
				} else {
					// For bright entries, style prefix separately
					prefixText := padRight(prefixStyle.Render(prefix), prefixWidth)
					displayLine = prefixText + " " + line
				}
			} else {
				// Continuation lines: indent to align with content
				contLine := strings.Repeat(" ", prefixWidth+1) + line
				if isDimmed {
					displayLine = mutedStyle.Render(contLine)
				} else {
					displayLine = contLine
				}
			}
			lines = append(lines, displayLine)
		}
		lines = append(lines, "") // blank line after entry
	}
	return lines
}

func (m model) channels() []channel {
	channels := []channel{{ID: mainChannelID, Name: "Main", Kind: "main"}}
	if len(m.agents) == 0 {
		return channels
	}

	activeAgents := make([]repo.Agent, 0, len(m.agents))
	archivedAgents := make([]repo.Agent, 0, len(m.agents))
	for _, agent := range m.agents {
		if agent.ArchivedAt.Valid {
			archivedAgents = append(archivedAgents, agent)
		} else {
			activeAgents = append(activeAgents, agent)
		}
	}

	// Group active agents by project/branch
	groupedAgents := make(map[string][]repo.Agent)
	for _, agent := range activeAgents {
		key := projectBranchKey(agent.Project, agent.Branch)
		groupedAgents[key] = append(groupedAgents[key], agent)
	}

	// Sort group keys alphabetically
	var groupKeys []string
	for key := range groupedAgents {
		groupKeys = append(groupKeys, key)
	}
	sort.Strings(groupKeys)

	// Build channels with headers and grouped agents
	for _, key := range groupKeys {
		// Add project/branch header
		channels = append(channels, channel{
			ID:    key,
			Name:  key,
			Kind:  "project_header",
			Level: 0,
		})

		// Add agents under this header only if expanded
		if m.isProjectExpanded(key) {
			agents := groupedAgents[key]
			ordered := sortAgentsByStatus(agents)
			for _, agent := range ordered {
				channels = append(channels, channel{
					ID:      agent.Name,
					Name:    agent.Name,
					Kind:    "agent",
					Status:  agent.Status,
					Level:   1,
					Project: agent.Project,
					Branch:  agent.Branch,
				})
			}
		}
	}

	// Archived section at the bottom, grouped by project/branch when expanded
	if len(archivedAgents) > 0 {
		channels = append(channels, channel{
			ID:   archivedChannelID,
			Name: fmt.Sprintf("Archived (%d)", len(archivedAgents)),
			Kind: "archived_header",
		})
		if m.archivedExpanded {
			// Group archived agents by project/branch
			groupedArchived := make(map[string][]repo.Agent)
			for _, agent := range archivedAgents {
				key := projectBranchKey(agent.Project, agent.Branch)
				groupedArchived[key] = append(groupedArchived[key], agent)
			}

			// Sort group keys alphabetically
			var archivedGroupKeys []string
			for key := range groupedArchived {
				archivedGroupKeys = append(archivedGroupKeys, key)
			}
			sort.Strings(archivedGroupKeys)

			// Build channels with headers and grouped archived agents
			for _, key := range archivedGroupKeys {
				// Add project/branch header for archived section
				channels = append(channels, channel{
					ID:    key,
					Name:  key,
					Kind:  "project_header",
					Level: 1,
				})

				// Add archived agents under this header only if expanded
				if m.isProjectExpanded(key) {
					agents := groupedArchived[key]
					ordered := sortArchivedAgents(agents)
					for _, agent := range ordered {
						channels = append(channels, channel{
							ID:      agent.Name,
							Name:    agent.Name,
							Kind:    "agent",
							Status:  agent.Status,
							Level:   2,
							Project: agent.Project,
							Branch:  agent.Branch,
						})
					}
				}
			}
		}
	}
	return channels
}

// projectBranchKey creates a consistent key for project/branch combinations
func projectBranchKey(project, branch string) string {
	return project + "/" + branch
}

// parseProjectBranch parses a channel ID in the format "project/branch" and returns the components
// Returns empty strings if the ID is not in the expected format
func parseProjectBranch(channelID string) (project, branch string) {
	parts := strings.SplitN(channelID, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

// isProjectHeader checks if a channel ID is a project/branch header (format: "project/branch")
func isProjectHeader(channelID string) bool {
	project, branch := parseProjectBranch(channelID)
	return project != "" && branch != ""
}

// isProjectExpanded checks if a project/branch group is expanded
// Default is expanded (true) if not explicitly set
func (m model) isProjectExpanded(key string) bool {
	expanded, exists := m.projectExpanded[key]
	if !exists {
		return true // Default to expanded
	}
	return expanded
}

func (m model) activeChannelLabel() string {
	if m.activeChannelID == mainChannelID {
		return "Main"
	}
	return m.activeChannelID
}

func (m model) renderChannelLine(ch channel, width int, cursor, active bool) string {
	// Background style for cursor highlight
	bgStyle := lipgloss.NewStyle()
	if cursor {
		bgStyle = bgStyle.Background(lipgloss.Color("8"))
	}

	// Calculate indentation based on Level
	indentWidth := ch.Level * 2
	indent := strings.Repeat(" ", indentWidth)
	// Style indent with background if cursor is active
	styledIndent := bgStyle.Render(indent)

	availableWidth := width - indentWidth
	if availableWidth < 1 {
		availableWidth = 1
	}

	// For project headers, add collapse/expand indicator
	var headerIndicator string
	if ch.Kind == "project_header" || ch.Kind == "archived_header" {
		if ch.Kind == "project_header" {
			// Check if this project is expanded
			if m.isProjectExpanded(ch.ID) {
				headerIndicator = "▼ "
			} else {
				headerIndicator = "▶ "
			}
		}
	}

	label := ch.Name
	labelWidth := availableWidth
	if ch.Kind == "agent" {
		labelWidth = availableWidth - 2 // Account for "● " prefix
		if labelWidth < 1 {
			labelWidth = 1
		}
	} else if ch.Kind == "project_header" {
		labelWidth = availableWidth - len(headerIndicator)
		if labelWidth < 1 {
			labelWidth = 1
		}
	}
	label = truncateString(label, labelWidth)

	// Label style - bold if active
	labelStyle := channelStyle
	if active {
		labelStyle = channelActiveStyle
	}
	if ch.Kind == "archived_header" || ch.Kind == "project_header" {
		labelStyle = mutedStyle
	}

	// For agents, render indicator separately to preserve its color
	if ch.Kind == "agent" {
		indicator, indicatorStyle := channelIndicator(ch)
		// Indicator keeps its foreground color, but gets background if cursor
		styledIndicator := bgStyle.Inherit(indicatorStyle).Render(indicator)
		// Label gets background if cursor, keeps its style
		styledLabel := bgStyle.Inherit(labelStyle).Render(label)
		// Pad the remaining space with background
		usedWidth := indentWidth + 2 + len(label) // indent + "● " + label
		padding := ""
		if usedWidth < width {
			padding = bgStyle.Render(strings.Repeat(" ", width-usedWidth))
		}
		return styledIndent + styledIndicator + " " + styledLabel + padding
	}

	// For project headers with collapse/expand indicator
	if ch.Kind == "project_header" {
		styledHeaderIndicator := bgStyle.Inherit(labelStyle).Render(headerIndicator)
		styledLabel := bgStyle.Inherit(labelStyle).Render(label)
		usedWidth := indentWidth + len(headerIndicator) + len(label)
		padding := ""
		if usedWidth < width {
			padding = bgStyle.Render(strings.Repeat(" ", width-usedWidth))
		}
		return styledIndent + styledHeaderIndicator + styledLabel + padding
	}

	// For main and archived_header, apply background to entire line
	styledLabel := bgStyle.Inherit(labelStyle).Render(label)
	usedWidth := indentWidth + len(label)
	padding := ""
	if usedWidth < width {
		padding = bgStyle.Render(strings.Repeat(" ", width-usedWidth))
	}
	return styledIndent + styledLabel + padding
}

func (m *model) moveCursor(delta int) tea.Cmd {
	channels := m.channels()
	if len(channels) == 0 {
		m.cursorIndex = 0
		return nil
	}
	m.cursorIndex += delta
	if m.cursorIndex < 0 {
		m.cursorIndex = 0
	}
	if m.cursorIndex >= len(channels) {
		m.cursorIndex = len(channels) - 1
	}
	// Auto-select on cursor move
	return m.activateSelection()
}

func (m *model) activateSelection() tea.Cmd {
	channels := m.channels()
	if len(channels) == 0 || m.cursorIndex >= len(channels) {
		return nil
	}
	selected := channels[m.cursorIndex]
	if selected.Kind == "archived_header" {
		m.archivedExpanded = !m.archivedExpanded
		return nil
	}
	if selected.Kind == "project_header" {
		// Toggle project header expand/collapse
		m.projectExpanded[selected.ID] = !m.isProjectExpanded(selected.ID)
		// Also set activeChannelID to the project header ID
		// This allows the right panel to show orchestrator chat for this project/branch
		m.activeChannelID = selected.ID
		m.updateViewportContent()
		// Focus chat input when selecting project header
		m.chatInput.Focus()
		return nil
	}
	m.activeChannelID = selected.ID

	// Blur chat input when selecting non-project items
	m.chatInput.Blur()

	// Update viewport content when switching channels
	m.updateViewportContent()

	if selected.Kind == "agent" {
		return fetchTranscriptsCmd(m.db, selected.ID, m.lastTranscriptIDs[selected.ID])
	}
	return nil
}

func (m *model) ensureSelection() {
	channels := m.channels()
	if len(channels) == 0 {
		m.cursorIndex = 0
		m.activeChannelID = mainChannelID
		return
	}

	if !channelExists(channels, m.activeChannelID) {
		m.activeChannelID = mainChannelID
		m.cursorIndex = 0
	}
	if m.cursorIndex >= len(channels) {
		m.cursorIndex = len(channels) - 1
	}
}

func (m *model) updateViewportContent() {
	// Get the actual content width for the viewport
	_, rightWidth, _, _ := m.layout()
	contentWidth := rightWidth - 2 // Account for border

	var content string
	if m.activeChannelID == mainChannelID || isProjectHeader(m.activeChannelID) {
		// Show orchestrator chat (messages) for Main or project headers
		lines := m.mainContentLines(contentWidth)
		content = strings.Join(lines, "\n")
	} else {
		// Show agent transcript for individual agents
		lines := m.transcriptContentLines(m.activeChannelID, contentWidth)
		content = strings.Join(lines, "\n")
	}

	m.viewport.SetContent(content)
}

func (m model) layout() (leftWidth, rightWidth, panelHeight, contentHeight int) {
	// Height: subtract status bar (1) + border top/bottom (2) = 3
	panelHeight = m.height - 3
	if panelHeight < 3 {
		panelHeight = 3
	}

	// Width: both panels have borders (2 chars each = 4 total)
	availableWidth := m.width - 4
	leftWidth = channelListWidth
	minRight := 20

	if availableWidth-leftWidth < minRight {
		leftWidth = clamp(availableWidth-minRight, 10, leftWidth)
	}
	if leftWidth < 10 {
		leftWidth = 10
	}
	rightWidth = availableWidth - leftWidth
	if rightWidth < minRight {
		rightWidth = minRight
		leftWidth = availableWidth - rightWidth
	}

	// Content height inside panel (1 for title row)
	contentHeight = panelHeight - 1
	// If chat input is shown, reserve 1 line for it
	if m.showChatInput() {
		contentHeight = contentHeight - 1
	}
	if contentHeight < 1 {
		contentHeight = 1
	}
	return leftWidth, rightWidth, panelHeight, contentHeight
}

func channelIndicator(ch channel) (string, lipgloss.Style) {
	if ch.Kind == "main" {
		return "●", statusCompleteStyle
	}
	status := strings.ToLower(ch.Status)
	switch status {
	case "busy":
		return "●", statusBusyStyle
	case "blocked", "idle":
		return "●", statusBlockedStyle
	case "complete":
		return "○", statusCompleteStyle
	case "failed":
		return "✗", statusFailedStyle
	default:
		return "○", statusCompleteStyle
	}
}

func sortAgentsByStatus(agents []repo.Agent) []repo.Agent {
	ordered := make([]repo.Agent, len(agents))
	copy(ordered, agents)
	order := map[string]int{
		"busy":     0,
		"blocked":  1,
		"complete": 2,
		"failed":   3,
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		iOrder, ok := order[strings.ToLower(ordered[i].Status)]
		if !ok {
			iOrder = 4
		}
		jOrder, ok := order[strings.ToLower(ordered[j].Status)]
		if !ok {
			jOrder = 4
		}
		if iOrder != jOrder {
			return iOrder < jOrder
		}
		return ordered[i].Name < ordered[j].Name
	})
	return ordered
}

func sortArchivedAgents(agents []repo.Agent) []repo.Agent {
	ordered := make([]repo.Agent, len(agents))
	copy(ordered, agents)
	sort.SliceStable(ordered, func(i, j int) bool {
		iTime := ordered[i].ArchivedAt.Time
		jTime := ordered[j].ArchivedAt.Time
		if !ordered[i].ArchivedAt.Valid {
			iTime = time.Time{}
		}
		if !ordered[j].ArchivedAt.Valid {
			jTime = time.Time{}
		}
		if !iTime.Equal(jTime) {
			return iTime.After(jTime)
		}
		return ordered[i].Name < ordered[j].Name
	})
	return ordered
}

func channelExists(channels []channel, id string) bool {
	for _, ch := range channels {
		if ch.ID == id {
			return true
		}
	}
	return false
}

func formatMessage(msg repo.Message) (string, lipgloss.Style) {
	switch msg.Type {
	case "exit":
		return fmt.Sprintf("exited - %s", msg.Content), mutedStyle
	case "complete":
		return fmt.Sprintf("completed - %s", msg.Content), mutedStyle
	case "error":
		return msg.Content, statusFailedStyle
	case "prompt":
		// Content only - @mention is styled separately in mainContentLines
		return msg.Content, messageStyle
	default:
		return msg.Content, messageStyle
	}
}

func transcriptPrefix(entry repo.LogEntry) (string, lipgloss.Style) {
	switch entry.EventType {
	case "input":
		return ">", inputStyle
	case "reasoning", "thinking":
		return "∴", mutedStyle
	case "command_execution":
		return "$", messageStyle
	case "tool_call":
		return "ƒ", messageStyle
	case "tool_result":
		if entry.Status.Valid && entry.Status.String == "failed" {
			return "!", statusFailedStyle
		}
		return "", messageStyle
	case "agent_message", "message":
		return "⏺", messageStyle
	default:
		return "∴", mutedStyle
	}
}

func padRight(s string, width int) string {
	pad := width - lipgloss.Width(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

func truncateString(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

// wrapText wraps text to fit within the specified width, breaking on word boundaries
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{""}
	}

	// Convert to runes to handle multi-byte characters correctly
	runes := []rune(text)
	if len(runes) <= width {
		return []string{text}
	}

	var lines []string
	var currentLine []rune

	words := strings.Fields(text)
	for _, word := range words {
		wordRunes := []rune(word)

		// If adding this word would exceed width, start a new line
		if len(currentLine) > 0 && len(currentLine)+1+len(wordRunes) > width {
			lines = append(lines, string(currentLine))
			currentLine = wordRunes
		} else if len(currentLine) == 0 {
			// First word on line
			if len(wordRunes) > width {
				// Word is too long, hard break it
				lines = append(lines, string(wordRunes[:width]))
				currentLine = wordRunes[width:]
			} else {
				currentLine = wordRunes
			}
		} else {
			// Add word to current line with space
			currentLine = append(currentLine, ' ')
			currentLine = append(currentLine, wordRunes...)
		}
	}

	// Add final line if any
	if len(currentLine) > 0 {
		lines = append(lines, string(currentLine))
	}

	// If no lines were created, return empty line
	if len(lines) == 0 {
		return []string{""}
	}

	return lines
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchMessagesCmd(db *sql.DB, project, branch, sinceID string) tea.Cmd {
	return func() tea.Msg {
		filter := repo.MessageFilter{
			Project: project,
			Branch:  branch,
		}
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
		ctx := scope.CurrentContext()
		agents, err := repo.ListAgents(db, repo.AgentFilter{
			Project: ctx.Project,
			Branch:  ctx.Branch,
		})
		if err != nil {
			return err
		}
		return agentsMsg(agents)
	}
}

func fetchTranscriptsCmd(db *sql.DB, agentID, sinceID string) tea.Cmd {
	return func() tea.Msg {
		ctx := scope.CurrentContext()
		entries, err := repo.ListLogs(db, ctx.Project, ctx.Branch, agentID, sinceID)
		if err != nil {
			return err
		}
		return transcriptsMsg{agentID: agentID, entries: entries}
	}
}

func cleanupStaleAgentsCmd(db *sql.DB) tea.Cmd {
	return func() tea.Msg {
		ctx := scope.CurrentContext()
		agents, err := repo.ListAgents(db, repo.AgentFilter{
			Project: ctx.Project,
			Branch:  ctx.Branch,
		})
		if err != nil {
			return nil
		}
		for _, a := range agents {
			if a.Status == "busy" && a.Pid.Valid {
				if !process.IsProcessAlive(int(a.Pid.Int64)) {
					msg := repo.Message{
						ID:           fmt.Sprintf("%s-exit-%d", a.Name, time.Now().Unix()),
						Project:      a.Project,
						Branch:       a.Branch,
						FromAgent:    a.Name,
						Type:         "exit",
						Content:      "process died unexpectedly",
						MentionsJSON: "[]",
						ReadByJSON:   "[]",
					}
					_ = repo.CreateMessage(db, msg)
					_ = repo.SetAgentFailed(db, a.Project, a.Branch, a.Name)
				}
			}
		}
		return nil
	}
}

// showChatInput returns true if the chat input should be visible
// Only show when a project header is selected (not Main, not individual agents)
func (m model) showChatInput() bool {
	return isProjectHeader(m.activeChannelID)
}

// getOttoAgent finds the @otto agent for the given project/branch
// Returns nil if not found
func (m model) getOttoAgent(project, branch string) *repo.Agent {
	for i := range m.agents {
		agent := &m.agents[i]
		if agent.Project == project && agent.Branch == branch && agent.Name == "otto" {
			return agent
		}
	}
	return nil
}

// isOttoBusy checks if @otto exists and is busy for the given project/branch
func (m model) isOttoBusy(project, branch string) bool {
	otto := m.getOttoAgent(project, branch)
	if otto == nil {
		return false
	}
	return otto.Status == "busy"
}

// getChatSubmitAction determines what action to take when user submits a message
// Returns: "none" (otto is busy), "spawn" (no otto exists), "prompt" (otto exists and is finished)
func (m model) getChatSubmitAction(project, branch string) string {
	otto := m.getOttoAgent(project, branch)
	if otto == nil {
		return "spawn"
	}
	if otto.Status == "busy" {
		return "none"
	}
	// Otto exists and is complete or failed - resume session
	return "prompt"
}

// handleChatSubmit processes chat input submission
// Spawns or prompts @otto based on current state
func (m *model) handleChatSubmit() tea.Cmd {
	if !m.showChatInput() {
		return nil
	}

	message := strings.TrimSpace(m.chatInput.Value())
	if message == "" {
		return nil
	}

	// Parse project/branch from active channel ID
	project, branch := parseProjectBranch(m.activeChannelID)
	if project == "" || branch == "" {
		return nil
	}

	// Determine action
	action := m.getChatSubmitAction(project, branch)
	if action == "none" {
		// Otto is busy - ignore submit
		return nil
	}

	// Clear input immediately
	m.chatInput.SetValue("")

	// Execute command in background
	var cmd *exec.Cmd
	if action == "spawn" {
		cmd = exec.Command("otto", "spawn", "codex", message, "--name", "otto")
	} else {
		// action == "prompt"
		cmd = exec.Command("otto", "prompt", "otto", message)
	}

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return err
		}
		return nil
	})
}

// Run starts the TUI
func Run(db *sql.DB) error {
	m := NewModel(db)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
