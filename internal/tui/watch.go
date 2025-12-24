package tui

import (
	"database/sql"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"time"

	"otto/internal/process"
	"otto/internal/repo"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	mainChannelID    = "main"
	channelListWidth = 16
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
	ID     string
	Name   string
	Kind   string
	Status string
}

type model struct {
	db                *sql.DB
	messages          []repo.Message
	agents            []repo.Agent
	transcripts       map[string][]repo.LogEntry
	lastMessageID     string
	lastTranscriptIDs map[string]string
	scrollOffsets     map[string]int
	width             int
	height            int
	cursorIndex       int
	activeChannelID   string
	err               error
}

func NewModel(db *sql.DB) model {
	return model{
		db:                db,
		messages:          []repo.Message{},
		agents:            []repo.Agent{},
		transcripts:       map[string][]repo.LogEntry{},
		lastTranscriptIDs: map[string]string{},
		scrollOffsets:     map[string]int{},
		activeChannelID:   mainChannelID,
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
			m.moveCursor(-1)
		case "down", "j":
			m.moveCursor(1)
		case "enter":
			return m, m.activateSelection()
		case "esc":
			m.activeChannelID = mainChannelID
			m.cursorIndex = 0
		case "g":
			m.scrollOffsets[m.activeChannelID] = 0
		case "G":
			m.scrollOffsets[m.activeChannelID] = m.maxScroll(m.activeChannelID)
		case "pgup", "ctrl+u":
			m.scrollContent(-5)
		case "pgdown", "ctrl+d":
			m.scrollContent(5)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		cmds := []tea.Cmd{
			tickCmd(),
			cleanupStaleAgentsCmd(m.db),
			fetchMessagesCmd(m.db, m.lastMessageID),
			fetchAgentsCmd(m.db),
		}
		if m.activeChannelID != mainChannelID {
			cmds = append(cmds, fetchTranscriptsCmd(m.db, m.activeChannelID, m.lastTranscriptIDs[m.activeChannelID]))
		}
		return m, tea.Batch(cmds...)

	case messagesMsg:
		if len(msg) > 0 {
			atBottom := m.scrollOffsets[mainChannelID] >= m.maxScroll(mainChannelID)
			m.messages = append(m.messages, msg...)
			m.lastMessageID = msg[len(msg)-1].ID
			if atBottom {
				m.scrollOffsets[mainChannelID] = m.maxScroll(mainChannelID)
			}
		}

	case transcriptsMsg:
		if len(msg.entries) > 0 {
			current := m.transcripts[msg.agentID]
			atBottom := m.scrollOffsets[msg.agentID] >= m.maxScroll(msg.agentID)
			m.transcripts[msg.agentID] = append(current, msg.entries...)
			m.lastTranscriptIDs[msg.agentID] = msg.entries[len(msg.entries)-1].ID
			if atBottom {
				m.scrollOffsets[msg.agentID] = m.maxScroll(msg.agentID)
			}
		}

	case agentsMsg:
		m.agents = msg
		m.ensureSelection()

	case error:
		m.err = msg
	}

	return m, nil
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	leftWidth, rightWidth, panelHeight, contentHeight := m.layout()

	// Title
	title := titleStyle.Render("Otto")

	// Left panel: Channels
	channelsTitle := panelTitleStyle.Width(leftWidth - 2).Render("Channels")
	channelsContent := m.renderChannels(leftWidth-2, contentHeight)

	leftPanel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(leftWidth).
		Height(panelHeight).
		Render(channelsTitle + "\n" + channelsContent)

	// Right panel: Content
	activeLabel := m.activeChannelLabel()
	rightTitle := panelTitleStyle.Width(rightWidth - 2).Render(activeLabel)
	content := m.renderContent(rightWidth-2, contentHeight)

	rightPanel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(rightWidth).
		Height(panelHeight).
		Render(rightTitle + "\n" + content)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	status := statusBarStyle.Render("q: quit | j/k: navigate | Enter: select | Esc: back to Main | g/G: top/bottom")

	return lipgloss.JoinVertical(lipgloss.Left, title, panels, status)
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

func (m model) renderContent(width, height int) string {
	if m.activeChannelID == mainChannelID {
		return m.renderMainContent(width, height)
	}
	return m.renderTranscriptContent(m.activeChannelID, width, height)
}

func (m model) renderMainContent(width, height int) string {
	lines := m.mainContentLines(width)
	return m.renderScrollable(lines, width, height, m.scrollOffsets[mainChannelID])
}

func (m model) renderTranscriptContent(agentID string, width, height int) string {
	lines := m.transcriptContentLines(agentID, width)
	return m.renderScrollable(lines, width, height, m.scrollOffsets[agentID])
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
		fromStyle := lipgloss.NewStyle().Foreground(usernameColor(msg.FromID)).Bold(true)
		fromText := padRight(fromStyle.Render(msg.FromID), fromWidth)

		content, style := formatMessage(msg)
		content = truncateString(content, contentWidth)
		contentText := style.Render(content)

		lines = append(lines, fromText+" "+contentText)
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
		prefix, style := transcriptPrefix(entry)
		prefix = padRight(style.Render(prefix), prefixWidth)
		content := truncateString(entry.Content, contentWidth)
		lines = append(lines, prefix+" "+content)
	}
	return lines
}

func (m model) renderScrollable(lines []string, width, height, offset int) string {
	if height <= 0 {
		return ""
	}
	if len(lines) == 0 {
		return ""
	}

	maxScroll := len(lines) - height
	if maxScroll < 0 {
		maxScroll = 0
	}
	if offset > maxScroll {
		offset = maxScroll
	}
	if offset < 0 {
		offset = 0
	}

	start := offset
	end := offset + height
	if end > len(lines) {
		end = len(lines)
	}

	return strings.Join(lines[start:end], "\n")
}

func (m model) channels() []channel {
	channels := []channel{{ID: mainChannelID, Name: "Main", Kind: "main"}}
	if len(m.agents) == 0 {
		return channels
	}

	ordered := sortAgentsByStatus(m.agents)
	for _, agent := range ordered {
		channels = append(channels, channel{
			ID:     agent.ID,
			Name:   agent.ID,
			Kind:   "agent",
			Status: agent.Status,
		})
	}
	return channels
}

func (m model) activeChannelLabel() string {
	if m.activeChannelID == mainChannelID {
		return "Main"
	}
	return m.activeChannelID
}

func (m model) renderChannelLine(ch channel, width int, cursor, active bool) string {
	label := ch.Name
	labelWidth := width
	if ch.Kind != "main" {
		labelWidth = width - 2
		if labelWidth < 1 {
			labelWidth = 1
		}
	}
	label = truncateString(label, labelWidth)

	labelStyle := channelStyle
	if active {
		labelStyle = channelActiveStyle
	}

	line := labelStyle.Render(label)
	if ch.Kind != "main" {
		indicator, indicatorStyle := channelIndicator(ch)
		line = fmt.Sprintf("%s %s", indicatorStyle.Render(indicator), line)
	}
	line = padRight(line, width)

	if cursor {
		return lipgloss.NewStyle().Background(lipgloss.Color("8")).Render(line)
	}

	return line
}

func (m model) moveCursor(delta int) {
	channels := m.channels()
	if len(channels) == 0 {
		m.cursorIndex = 0
		return
	}
	m.cursorIndex += delta
	if m.cursorIndex < 0 {
		m.cursorIndex = 0
	}
	if m.cursorIndex >= len(channels) {
		m.cursorIndex = len(channels) - 1
	}
}

func (m *model) activateSelection() tea.Cmd {
	channels := m.channels()
	if len(channels) == 0 || m.cursorIndex >= len(channels) {
		return nil
	}
	selected := channels[m.cursorIndex]
	m.activeChannelID = selected.ID
	if selected.Kind == "agent" {
		return fetchTranscriptsCmd(m.db, selected.ID, m.lastTranscriptIDs[selected.ID])
	}
	return nil
}

func (m model) maxScroll(channelID string) int {
	_, _, _, contentHeight := m.layout()
	count := 0
	switch channelID {
	case mainChannelID:
		count = len(m.mainContentLines(80))
	default:
		count = len(m.transcriptContentLines(channelID, 80))
	}
	maxScroll := count - contentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	return maxScroll
}

func (m *model) scrollContent(delta int) {
	current := m.scrollOffsets[m.activeChannelID]
	current += delta
	maxScroll := m.maxScroll(m.activeChannelID)
	if current < 0 {
		current = 0
	}
	if current > maxScroll {
		current = maxScroll
	}
	m.scrollOffsets[m.activeChannelID] = current
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

func (m model) layout() (leftWidth, rightWidth, panelHeight, contentHeight int) {
	panelHeight = m.height - 2
	if panelHeight < 3 {
		panelHeight = 3
	}
	leftWidth = channelListWidth
	minRight := 20
	if m.width-leftWidth < minRight {
		leftWidth = clamp(m.width-minRight, 12, leftWidth)
	}
	if leftWidth < 12 {
		leftWidth = 12
	}
	rightWidth = m.width - leftWidth
	if rightWidth < minRight {
		rightWidth = minRight
		leftWidth = m.width - rightWidth
	}
	contentHeight = panelHeight - 3
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
		return ordered[i].ID < ordered[j].ID
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
		if msg.ToID.Valid {
			return fmt.Sprintf("@%s %s", msg.ToID.String, msg.Content), mutedStyle
		}
		return msg.Content, mutedStyle
	default:
		return msg.Content, messageStyle
	}
}

func transcriptPrefix(entry repo.LogEntry) (string, lipgloss.Style) {
	switch entry.Direction {
	case "in":
		return "→", mutedStyle
	case "out":
		if entry.Stream.Valid && entry.Stream.String == "stderr" {
			return "!", statusFailedStyle
		}
		return "←", messageStyle
	default:
		return "·", mutedStyle
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

func fetchTranscriptsCmd(db *sql.DB, agentID, sinceID string) tea.Cmd {
	return func() tea.Msg {
		entries, err := repo.ListLogs(db, agentID, sinceID)
		if err != nil {
			return err
		}
		return transcriptsMsg{agentID: agentID, entries: entries}
	}
}

func cleanupStaleAgentsCmd(db *sql.DB) tea.Cmd {
	return func() tea.Msg {
		agents, err := repo.ListAgents(db)
		if err != nil {
			return nil
		}
		for _, a := range agents {
			if a.Status == "busy" && a.Pid.Valid {
				if !process.IsProcessAlive(int(a.Pid.Int64)) {
					msg := repo.Message{
						ID:           fmt.Sprintf("%s-exit-%d", a.ID, time.Now().Unix()),
						FromID:       a.ID,
						Type:         "exit",
						Content:      "process died unexpectedly",
						MentionsJSON: "[]",
						ReadByJSON:   "[]",
					}
					_ = repo.CreateMessage(db, msg)
					_ = repo.SetAgentFailed(db, a.ID)
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
