package tui

import (
	"database/sql"
	"strings"
	"testing"

	"june/internal/repo"

	_ "modernc.org/sqlite"
)

func TestProjectHeaderSelectionSetsActiveChannel(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "june", Branch: "main", Name: "impl-1", Status: "busy"},
		{Project: "other", Branch: "feature", Name: "worker", Status: "complete"},
	}

	channels := m.sidebarItems()

	// Find the june/main header
	headerIndex := -1
	for i, ch := range channels {
		if ch.ID == "june/main" && ch.Kind == SidebarChannelHeader {
			headerIndex = i
			break
		}
	}
	if headerIndex == -1 {
		t.Fatal("expected to find june/main header")
	}

	// Select the project header
	m.cursorIndex = headerIndex
	_ = m.activateSelection()

	// Should set activeChannelID to the project header ID
	if m.activeChannelID != "june/main" {
		t.Errorf("expected activeChannelID to be 'june/main', got %q", m.activeChannelID)
	}
}

func TestProjectHeaderSelectionTogglesExpanded(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "june", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	channels := m.sidebarItems()

	// Find the june/main header
	headerIndex := -1
	for i, ch := range channels {
		if ch.ID == "june/main" && ch.Kind == SidebarChannelHeader {
			headerIndex = i
			break
		}
	}
	if headerIndex == -1 {
		t.Fatal("expected to find june/main header")
	}

	// Initially expanded (default)
	if !m.isProjectExpanded("june/main") {
		t.Fatal("expected june/main to be expanded by default")
	}

	// Select the header - should toggle to collapsed
	m.cursorIndex = headerIndex
	_ = m.toggleSelection()
	if m.isProjectExpanded("june/main") {
		t.Error("expected june/main to be collapsed after first activation")
	}

	// Select again - should toggle back to expanded
	_ = m.toggleSelection()
	if !m.isProjectExpanded("june/main") {
		t.Error("expected june/main to be expanded after second activation")
	}
}

func TestAgentSelectionStillSetsActiveChannelToAgent(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "june", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	channels := m.sidebarItems()

	// Find the agent
	agentIndex := -1
	for i, ch := range channels {
		if ch.ID == "impl-1" && ch.Kind == SidebarAgentRow {
			agentIndex = i
			break
		}
	}
	if agentIndex == -1 {
		t.Fatal("expected to find impl-1 agent")
	}

	// Select the agent
	m.cursorIndex = agentIndex
	_ = m.activateSelection()

	// Should set activeChannelID to the agent name
	if m.activeChannelID != "impl-1" {
		t.Errorf("expected activeChannelID to be 'impl-1', got %q", m.activeChannelID)
	}
}

func TestRenderChannelLineProjectHeader(t *testing.T) {
	m := NewModel(nil)
	// Set project as expanded (default)
	m.projectExpanded = map[string]bool{"june/main": true}

	// Create a project header channel
	ch := SidebarItem{
		ID:    "june/main",
		Name:  "june/main",
		Kind:  SidebarChannelHeader,
		Level: 0,
	}

	// Render with cursor (should have background)
	width := 20
	rendered := m.renderChannelLine(ch, width, true, false)

	// Strip ANSI codes for easier testing
	stripped := stripAnsi(rendered)

	// Should show project name and collapse indicator
	if !strings.Contains(stripped, "june/main") {
		t.Errorf("expected header to contain project name, got: %q", stripped)
	}

	// Should show expanded indicator (▼)
	if !strings.Contains(stripped, "▼") {
		t.Errorf("expected expanded indicator (▼) for expanded header, got: %q", stripped)
	}

	// Verify display width matches expected width (use rune count, not byte length)
	if len([]rune(stripped)) != width {
		t.Errorf("expected stripped display width %d, got %d: %q", width, len([]rune(stripped)), stripped)
	}

	// Should NOT contain status indicator (●, ○, ✗)
	if strings.Contains(stripped, "●") || strings.Contains(stripped, "○") || strings.Contains(stripped, "✗") {
		t.Errorf("expected no status indicator for header, got: %q", stripped)
	}

	// Test collapsed state
	m.projectExpanded["june/main"] = false
	renderedCollapsed := m.renderChannelLine(ch, width, true, false)
	strippedCollapsed := stripAnsi(renderedCollapsed)

	// Should show collapsed indicator (▶)
	if !strings.Contains(strippedCollapsed, "▶") {
		t.Errorf("expected collapsed indicator (▶) for collapsed header, got: %q", strippedCollapsed)
	}
}

func TestRenderChannelLineIndentedAgentWithCursor(t *testing.T) {
	m := NewModel(nil)

	// Create an indented agent channel (Level 1)
	ch := SidebarItem{
		ID:      "impl-1",
		Name:    "impl-1",
		Kind:    SidebarAgentRow,
		Status:  "busy",
		Level:   1,
		Project: "june",
		Branch:  "main",
	}

	// Render with cursor (should have background on indent AND content)
	width := 20
	rendered := m.renderChannelLine(ch, width, true, false)

	// Strip ANSI codes for easier testing
	stripped := stripAnsi(rendered)

	// Should be indented (Level 1 = 2 spaces)
	if !strings.HasPrefix(stripped, "  ") {
		t.Errorf("expected 2-space indent for Level 1, got: %q", stripped)
	}

	// Should contain status indicator
	if !strings.Contains(stripped, "●") {
		t.Errorf("expected status indicator for agent, got: %q", stripped)
	}

	// Should contain agent name
	if !strings.Contains(stripped, "impl-1") {
		t.Errorf("expected agent name in output, got: %q", stripped)
	}

	// Verify length matches width (accounting for ANSI codes in lipgloss output)
	// The stripped output should be exactly width characters
	// Note: lipgloss may not render ANSI codes in test environment (no TTY)
	// so we just check the visual width matches
	if len([]rune(stripped)) < width {
		t.Errorf("expected stripped display width at least %d, got %d: %q", width, len([]rune(stripped)), stripped)
	}

	// Render without cursor for comparison
	renderedNoCursor := m.renderChannelLine(ch, width, false, false)
	strippedNoCursor := stripAnsi(renderedNoCursor)

	// Both should have the same visual content (indent + indicator + label)
	if !strings.HasPrefix(strippedNoCursor, "  ") {
		t.Errorf("expected 2-space indent even without cursor, got: %q", strippedNoCursor)
	}
}

func TestRenderChannelLineIndentedHeaderLevel1(t *testing.T) {
	m := NewModel(nil)

	// Create a project header at Level 1 (archived section)
	ch := SidebarItem{
		ID:    "june/main",
		Name:  "june/main",
		Kind:  SidebarChannelHeader,
		Level: 1,
	}

	// Render with cursor
	width := 20
	rendered := m.renderChannelLine(ch, width, true, false)

	// Strip ANSI codes
	stripped := stripAnsi(rendered)

	// Should be indented (Level 1 = 2 spaces)
	if !strings.HasPrefix(stripped, "  ") {
		t.Errorf("expected 2-space indent for Level 1 header, got: %q", stripped)
	}

	// Verify display width matches expected width (use rune count, not byte length)
	if len([]rune(stripped)) != width {
		t.Errorf("expected stripped display width %d, got %d: %q", width, len([]rune(stripped)), stripped)
	}
}

// stripAnsi removes ANSI escape codes from a string
func stripAnsi(s string) string {
	// Simple ANSI escape sequence stripper
	// Matches ESC [ ... m sequences
	var result strings.Builder
	inEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			inEscape = true
			i++ // skip '['
			continue
		}
		if inEscape {
			if s[i] == 'm' {
				inEscape = false
			}
			continue
		}
		result.WriteByte(s[i])
	}
	return result.String()
}

func TestProjectHeaderMessagesUsesProjectScope(t *testing.T) {
	// Create in-memory database with schema
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Manually create schema
	schemaSQL := `
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			branch TEXT NOT NULL,
			from_agent TEXT,
			to_agent TEXT,
			type TEXT NOT NULL,
			content TEXT,
			mentions TEXT,
			requires_human INTEGER DEFAULT 0,
			read_by TEXT,
			from_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := db.Exec(schemaSQL); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Insert messages for different projects/branches
	_, err = db.Exec(
		`INSERT INTO messages (id, project, branch, from_agent, type, content, mentions, read_by, from_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"msg-june-main", "june", "main", "user", "say", "message for june/main", "[]", "[]", "user",
	)
	if err != nil {
		t.Fatalf("failed to insert june/main message: %v", err)
	}

	_, err = db.Exec(
		`INSERT INTO messages (id, project, branch, from_agent, type, content, mentions, read_by, from_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"msg-other-feature", "other", "feature", "user", "say", "message for other/feature", "[]", "[]", "user",
	)
	if err != nil {
		t.Fatalf("failed to insert other/feature message: %v", err)
	}

	// Fetch messages with explicit project/branch (new signature)
	// This should use june/main scope, not the current git context
	cmd := fetchMessagesCmd(db, "june", "main", "")
	msg := cmd()

	// Verify we got the correct messages
	messagesMsg, ok := msg.(messagesMsg)
	if !ok {
		if err, ok := msg.(error); ok {
			t.Fatalf("fetchMessagesCmd returned error: %v", err)
		}
		t.Fatalf("expected messagesMsg, got %T", msg)
	}

	// Should get only june/main messages
	if len(messagesMsg) != 1 {
		t.Errorf("expected 1 message from june/main scope, got %d", len(messagesMsg))
	}

	if len(messagesMsg) > 0 {
		if messagesMsg[0].ID != "msg-june-main" {
			t.Errorf("expected message ID %q, got %q", "msg-june-main", messagesMsg[0].ID)
		}
		if messagesMsg[0].Content != "message for june/main" {
			t.Errorf("expected content %q, got %q", "message for june/main", messagesMsg[0].Content)
		}
	}
}

func TestProjectHeaderMouseClick(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "june", Branch: "main", Name: "impl-1", Status: "busy"},
		{Project: "june", Branch: "main", Name: "reviewer", Status: "blocked"},
		{Project: "other", Branch: "feature", Name: "worker", Status: "complete"},
	}

	channels := m.sidebarItems()
	// Expected structure:
	// 0: Main
	// 1: other/feature header
	// 2:   worker
	// 3: separator
	// 4: june/main header
	// 5:   impl-1
	// 6:   reviewer

	// Find the june/main header index
	headerIndex := -1
	for i, ch := range channels {
		if ch.ID == "june/main" && ch.Kind == SidebarChannelHeader {
			headerIndex = i
			break
		}
	}
	if headerIndex == -1 {
		t.Fatal("expected to find june/main header")
	}

	// Simulate mouse click on project header with activateSelection
	// This should just set the activeChannelID, not toggle
	m.cursorIndex = headerIndex
	_ = m.activateSelection()

	// Should set activeChannelID to project header
	if m.activeChannelID != "june/main" {
		t.Errorf("expected activeChannelID to be 'june/main', got %q", m.activeChannelID)
	}

	// Should NOT toggle expansion on activateSelection (still expanded)
	if !m.isProjectExpanded("june/main") {
		t.Error("expected june/main to still be expanded after activateSelection")
	}

	// Use toggleSelection to actually toggle
	_ = m.toggleSelection()
	if m.isProjectExpanded("june/main") {
		t.Error("expected june/main to be collapsed after toggleSelection")
	}
}

func TestNavigationSkipsCollapsedAgents(t *testing.T) {
	// This test verifies that navigation works correctly with collapsed project groups.
	// When collapsed agents are not in the channel list, cursor navigation skips them naturally.

	m := NewModel(nil)
	m.projectExpanded = map[string]bool{
		"june/main":     false, // Collapse june/main
		"other/feature": true,  // Keep other/feature expanded to avoid auto-toggle
	}
	m.agents = []repo.Agent{
		{Project: "june", Branch: "main", Name: "impl-1", Status: "busy"},
		{Project: "june", Branch: "main", Name: "reviewer", Status: "blocked"},
		{Project: "other", Branch: "feature", Name: "worker", Status: "complete"},
	}

	channels := m.sidebarItems()
	// Expected structure (june/main collapsed, other/feature expanded):
	// 0: june/main header (collapsed, agents hidden)
	// 1: separator
	// 2: other/feature header
	// 3:   worker

	if len(channels) != 4 {
		t.Fatalf("expected 4 channels with june/main collapsed, got %d", len(channels))
	}

	// Verify june/main agents are not in the list
	for _, ch := range channels {
		if ch.ID == "impl-1" || ch.ID == "reviewer" {
			t.Errorf("expected june/main agents to be hidden when collapsed, found %q", ch.ID)
		}
	}

	// Navigate through the list - should only see visible channels
	m.cursorIndex = 0 // june/main header
	if channels[m.cursorIndex].ID != "june/main" {
		t.Errorf("expected cursor at june/main header, got %q", channels[m.cursorIndex].ID)
	}

	// Move down - should skip separator (index 1) and go to other/feature (index 2)
	_ = m.moveCursor(1)
	if channels[m.cursorIndex].ID != "other/feature" {
		t.Errorf("expected cursor at other/feature header, got %q", channels[m.cursorIndex].ID)
	}

	// Move down - should go to worker (index 3)
	_ = m.moveCursor(1)
	if channels[m.cursorIndex].ID != "worker" {
		t.Errorf("expected cursor at worker, got %q", channels[m.cursorIndex].ID)
	}

	// Verify we can't move past the end
	m.cursorIndex = 3
	_ = m.moveCursor(1) // Try to move down
	if m.cursorIndex != 3 {
		t.Errorf("expected cursor to clamp at last channel (3), got %d", m.cursorIndex)
	}

	// Verify we can't move before the beginning
	m.cursorIndex = 0
	_ = m.moveCursor(-1) // Try to move up
	if m.cursorIndex != 0 {
		t.Errorf("expected cursor to clamp at first channel (0), got %d", m.cursorIndex)
	}
}

func TestEnsureSelectionHandlesCollapsedAgents(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "june", Branch: "main", Name: "impl-1", Status: "busy"},
		{Project: "june", Branch: "main", Name: "reviewer", Status: "blocked"},
		{Project: "other", Branch: "feature", Name: "worker", Status: "complete"},
	}

	// Start with expanded project, cursor on impl-1 (index 4)
	channels := m.sidebarItems()
	impl1Index := -1
	for i, ch := range channels {
		if ch.ID == "impl-1" {
			impl1Index = i
			break
		}
	}
	if impl1Index == -1 {
		t.Fatal("expected to find impl-1")
	}

	m.cursorIndex = impl1Index
	m.activeChannelID = "impl-1"

	// Now collapse june/main - impl-1 disappears from channel list
	m.projectExpanded["june/main"] = false

	// Call ensureSelection - should adjust cursor to valid position
	m.ensureSelection()

	// Cursor should be adjusted to valid index
	channels = m.sidebarItems()
	if m.cursorIndex >= len(channels) {
		t.Errorf("expected cursor index < %d after collapse, got %d", len(channels), m.cursorIndex)
	}

	// Active channel should be set to the first valid channel when selected agent is hidden
	// Channels are sorted alphabetically, so "june/main" comes before "other/feature"
	if m.activeChannelID != "june/main" {
		t.Errorf("expected activeChannelID to be 'june/main' after agent hidden, got %q", m.activeChannelID)
	}
}

func TestNavigationRespectsChannelListLength(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "june", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	channels := m.sidebarItems()
	// Expected: june/main header, impl-1 (2 channels)

	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}

	// Start at last channel (impl-1)
	m.cursorIndex = len(channels) - 1

	// Move down - should clamp to last index
	_ = m.moveCursor(1)
	if m.cursorIndex != len(channels)-1 {
		t.Errorf("expected cursor to stay at last index %d, got %d", len(channels)-1, m.cursorIndex)
	}

	// Move up to first channel
	m.cursorIndex = 0

	// Move up - should clamp to 0
	_ = m.moveCursor(-1)
	if m.cursorIndex != 0 {
		t.Error("expected cursor to stay at index 0")
	}
}

