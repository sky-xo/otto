package tui

import (
	"strings"
	"testing"
	"time"

	"june/internal/claude"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFormatTimestamp(t *testing.T) {
	// Fix "now" to a known time for predictable tests
	// We'll test relative to actual time.Now() since the function uses it

	now := time.Now()
	// Use noon today to avoid midnight crossing issues
	todayNoon := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())

	tests := []struct {
		name     string
		input    time.Time
		contains string // We check contains since exact time formatting varies
	}{
		{
			name:     "today shows just time",
			input:    todayNoon,
			contains: "PM", // noon is always PM
		},
		{
			name:     "yesterday shows Yesterday @",
			input:    now.AddDate(0, 0, -1),
			contains: "Yesterday @",
		},
		{
			name:     "3 days ago shows weekday",
			input:    now.AddDate(0, 0, -3),
			contains: "@", // Should have weekday @ time
		},
		{
			name:     "2 weeks ago shows date",
			input:    now.AddDate(0, 0, -14),
			contains: "@", // Should have "14 Jan @" or similar
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTimestamp(tt.input)
			if !contains(result, tt.contains) {
				t.Errorf("formatTimestamp(%v) = %q, expected to contain %q", tt.input, result, tt.contains)
			}
		})
	}
}

func TestFormatTimestamp_Today(t *testing.T) {
	// Use a fixed time at noon today to avoid crossing midnight
	// regardless of when the test runs
	now := time.Now()
	todayNoon := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())

	result := formatTimestamp(todayNoon)

	// Today's time should NOT contain "Yesterday" or "@" prefix
	if contains(result, "Yesterday") {
		t.Errorf("Today's timestamp should not contain 'Yesterday': %q", result)
	}
	// Should be just a time like "12:00:00 PM"
	if contains(result, "@") {
		t.Errorf("Today's timestamp should not contain '@': %q", result)
	}
}

func TestFormatTimestamp_Yesterday(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)

	result := formatTimestamp(yesterday)

	if !contains(result, "Yesterday @") {
		t.Errorf("Yesterday's timestamp should contain 'Yesterday @': %q", result)
	}
}

func TestFormatTimestamp_ThisWeek(t *testing.T) {
	now := time.Now()
	threeDaysAgo := now.AddDate(0, 0, -3)

	result := formatTimestamp(threeDaysAgo)

	// Should contain a weekday abbreviation
	weekdays := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	hasWeekday := false
	for _, day := range weekdays {
		if contains(result, day) {
			hasWeekday = true
			break
		}
	}
	if !hasWeekday {
		t.Errorf("This week's timestamp should contain weekday: %q", result)
	}
	if !contains(result, "@") {
		t.Errorf("This week's timestamp should contain '@': %q", result)
	}
}

func TestFormatTimestamp_Older(t *testing.T) {
	now := time.Now()
	twoWeeksAgo := now.AddDate(0, 0, -14)

	result := formatTimestamp(twoWeeksAgo)

	// Should contain a month abbreviation
	months := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	hasMonth := false
	for _, month := range months {
		if contains(result, month) {
			hasMonth = true
			break
		}
	}
	if !hasMonth {
		t.Errorf("Older timestamp should contain month: %q", result)
	}
	if !contains(result, "@") {
		t.Errorf("Older timestamp should contain '@': %q", result)
	}
}

// helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && searchString(s, substr)))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// createTestAgents creates N test agents for testing
func createTestAgents(n int) []claude.Agent {
	agents := make([]claude.Agent, n)
	for i := 0; i < n; i++ {
		agents[i] = claude.Agent{
			ID:       "agent" + string(rune('A'+i)),
			FilePath: "/test/agent-" + string(rune('A'+i)) + ".jsonl",
			LastMod:  time.Now().Add(-time.Duration(i) * time.Hour),
		}
	}
	return agents
}

// createModelWithAgents creates a model with the given agents and dimensions
func createModelWithAgents(agents []claude.Agent, width, height int) Model {
	m := NewModel("/test/claude/projects", "/test/repo", "repo")
	// Create a single channel with the test agents
	m.channels = []claude.Channel{
		{Name: "repo:main", Dir: "/test/claude/projects/repo", Agents: agents},
	}
	// Set lastViewedAgent to first agent if available (for right panel display)
	if len(agents) > 0 {
		m.lastViewedAgent = &m.channels[0].Agents[0]
	}
	m.width = width
	m.height = height
	return m
}

func TestRenderSidebarContent_NoScroll(t *testing.T) {
	// Test that when all agents fit, no scroll indicators are shown
	agents := createTestAgents(3)
	m := createModelWithAgents(agents, 25, 20) // Height 20 = plenty of room

	content := m.renderSidebarContent(20, 10) // 10 lines, 3 agents

	// Should not have scroll indicators
	if strings.Contains(content, "more") {
		t.Errorf("Expected no scroll indicators, but got: %s", content)
	}

	// Should contain all agent IDs
	for _, agent := range agents {
		if !strings.Contains(content, agent.ID) {
			t.Errorf("Expected to find agent %s in content: %s", agent.ID, content)
		}
	}
}

func TestRenderSidebarContent_ShowsBottomIndicator(t *testing.T) {
	// Test that bottom indicator shows when more agents are below
	agents := createTestAgents(10)
	m := createModelWithAgents(agents, 25, 10)
	m.sidebarOffset = 0

	content := m.renderSidebarContent(20, 5) // Only 5 lines available

	// Should show bottom indicator
	if !strings.Contains(content, "more") {
		t.Errorf("Expected bottom scroll indicator, but got: %s", content)
	}

	// Should show downward arrow
	if !strings.Contains(content, "\u2193") {
		t.Errorf("Expected downward arrow indicator, but got: %s", content)
	}
}

func TestRenderSidebarContent_ShowsTopIndicator(t *testing.T) {
	// Test that top indicator shows when scrolled down
	agents := createTestAgents(10)
	m := createModelWithAgents(agents, 25, 10)
	m.sidebarOffset = 3 // Scrolled down by 3

	content := m.renderSidebarContent(20, 10)

	// Should show top indicator with "3 more"
	if !strings.Contains(content, "3 more") {
		t.Errorf("Expected '3 more' in top indicator, but got: %s", content)
	}

	// Should show upward arrow
	if !strings.Contains(content, "\u2191") {
		t.Errorf("Expected upward arrow indicator, but got: %s", content)
	}
}

func TestRenderSidebarContent_ShowsBothIndicators(t *testing.T) {
	// Test that both indicators show when in the middle of a long list
	agents := createTestAgents(20)
	m := createModelWithAgents(agents, 25, 10)
	m.sidebarOffset = 5 // Scrolled to middle

	content := m.renderSidebarContent(20, 5) // Small height to force both indicators

	// Should show top indicator
	if !strings.Contains(content, "\u2191") {
		t.Errorf("Expected top scroll indicator, but got: %s", content)
	}

	// Should show bottom indicator
	if !strings.Contains(content, "\u2193") {
		t.Errorf("Expected bottom scroll indicator, but got: %s", content)
	}
}

func TestEnsureSelectedVisible_ScrollsDown(t *testing.T) {
	// Test that selecting an item below the visible area scrolls down
	agents := createTestAgents(20)
	m := createModelWithAgents(agents, 25, 10) // Small height
	m.sidebarOffset = 0
	m.selectedIdx = 15 // Select item beyond visible range

	m.ensureSelectedVisible()

	// Offset should have increased to show selected item
	if m.sidebarOffset == 0 {
		t.Error("Expected sidebarOffset to increase when selecting item below visible range")
	}

	// Selected item should now be visible
	visibleStart := m.sidebarOffset
	visibleEnd := m.sidebarOffset + m.sidebarVisibleLines()
	if m.selectedIdx < visibleStart || m.selectedIdx >= visibleEnd {
		t.Errorf("Selected item %d should be visible in range [%d, %d)", m.selectedIdx, visibleStart, visibleEnd)
	}
}

func TestEnsureSelectedVisible_ScrollsUp(t *testing.T) {
	// Test that selecting an item above the visible area scrolls up
	agents := createTestAgents(20)
	m := createModelWithAgents(agents, 25, 10)
	m.sidebarOffset = 10 // Scrolled down
	m.selectedIdx = 3    // Select item above visible range

	m.ensureSelectedVisible()

	// Offset should have decreased to show selected item
	if m.sidebarOffset > 3 {
		t.Errorf("Expected sidebarOffset to be at most 3, got %d", m.sidebarOffset)
	}
}

func TestEnsureSelectedVisible_NoChangeWhenVisible(t *testing.T) {
	// Test that offset doesn't change when item is already visible
	agents := createTestAgents(20)
	m := createModelWithAgents(agents, 25, 15)
	m.sidebarOffset = 5
	m.selectedIdx = 7 // Already in visible range

	originalOffset := m.sidebarOffset
	m.ensureSelectedVisible()

	if m.sidebarOffset != originalOffset {
		t.Errorf("Offset should not change when item is visible. Was %d, now %d", originalOffset, m.sidebarOffset)
	}
}

func TestSidebarVisibleLines_NoIndicators(t *testing.T) {
	// When offset is 0 and all items fit, no indicators are needed
	agents := createTestAgents(3)
	m := createModelWithAgents(agents, 25, 10)
	m.sidebarOffset = 0

	lines := m.sidebarVisibleLines()

	// With height 10, panelHeight = 10-1 = 9, contentHeight = 9-2 = 7
	// No indicators needed since 3 agents fit in 7 lines
	if lines < 3 {
		t.Errorf("Expected at least 3 visible lines for 3 agents, got %d", lines)
	}
}

func TestSidebarVisibleLines_WithTopIndicator(t *testing.T) {
	// When scrolled down, top indicator takes one line
	agents := createTestAgents(20)
	m := createModelWithAgents(agents, 25, 10)
	m.sidebarOffset = 5

	linesScrolled := m.sidebarVisibleLines()

	// Reset offset and compare
	m.sidebarOffset = 0
	linesNotScrolled := m.sidebarVisibleLines()

	// When scrolled, we lose one line to the top indicator
	// (and possibly another to bottom indicator)
	if linesScrolled >= linesNotScrolled {
		t.Errorf("Expected fewer visible lines when scrolled (has top indicator). Scrolled: %d, Not scrolled: %d",
			linesScrolled, linesNotScrolled)
	}
}

func TestRenderSidebarContent_EmptyAgents(t *testing.T) {
	// With no channels at all, should show "No agents found"
	m := NewModel("/test/claude/projects", "/test/repo", "repo")
	m.channels = []claude.Channel{} // No channels
	m.width = 25
	m.height = 10

	content := m.renderSidebarContent(20, 5)

	if content != "No agents found" {
		t.Errorf("Expected 'No agents found', got: %s", content)
	}
}

func TestRenderSidebarContent_ZeroHeight(t *testing.T) {
	agents := createTestAgents(5)
	m := createModelWithAgents(agents, 25, 10)

	content := m.renderSidebarContent(20, 0)

	if content != "No agents found" {
		t.Errorf("Expected 'No agents found' for zero height, got: %s", content)
	}
}

// stripANSI removes ANSI escape codes from a string for easier test assertions
func stripANSI(s string) string {
	// Simple regex-free approach: remove escape sequences
	var result strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

func TestRenderMarkdown_BasicFormatting(t *testing.T) {
	// Test that markdown is processed (asterisks removed)
	input := "This is **bold** text"
	result := renderMarkdown(input, 80)

	// The rendered output should NOT contain the literal asterisks
	if strings.Contains(result, "**bold**") {
		t.Errorf("Expected markdown to be rendered, but found literal asterisks: %s", result)
	}

	// Strip ANSI and verify the word "bold" is present
	stripped := stripANSI(result)
	if !strings.Contains(stripped, "bold") {
		t.Errorf("Expected 'bold' to be in output: %s", stripped)
	}
}

func TestRenderMarkdown_PreservesContent(t *testing.T) {
	// Test that plain text content is preserved (after stripping ANSI)
	input := "Just plain text here"
	result := renderMarkdown(input, 80)

	stripped := stripANSI(result)
	if !strings.Contains(stripped, "Just plain text here") {
		t.Errorf("Expected plain text to be preserved: %s (stripped: %s)", result, stripped)
	}
}

func TestRenderMarkdown_ZeroWidth(t *testing.T) {
	// Test that zero width defaults to 80
	input := "Some text"
	result := renderMarkdown(input, 0)

	// Should not panic and should contain the text (after stripping ANSI)
	stripped := stripANSI(result)
	if !strings.Contains(stripped, "Some text") {
		t.Errorf("Expected text to be present: %s (stripped: %s)", result, stripped)
	}
}

func TestFormatTranscript_UserPromptStyle(t *testing.T) {
	// Test that user prompts contain the content
	entries := []claude.Entry{
		{Type: "user", Message: claude.Message{
			Content: "Hello there",
		}},
	}

	result := formatTranscript(entries, 80)

	// User prompts should contain the content
	if !strings.Contains(result, "Hello there") {
		t.Errorf("Expected user prompt content to be present: %s", result)
	}
}

func TestFormatTranscript_AssistantMarkdownRendered(t *testing.T) {
	// Test that assistant markdown content is processed
	entries := []claude.Entry{
		{Type: "assistant", Message: claude.Message{
			Content: []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "Here is **bold** text",
				},
			},
		}},
	}

	result := formatTranscript(entries, 80)

	// Should not contain literal asterisks
	if strings.Contains(result, "**bold**") {
		t.Errorf("Expected markdown to be rendered, but found literal asterisks: %s", result)
	}

	// Strip ANSI and verify the word "bold" is present
	stripped := stripANSI(result)
	if !strings.Contains(stripped, "bold") {
		t.Errorf("Expected 'bold' to be present: %s", stripped)
	}
}

func TestFormatDiffSummary_AddedOnly(t *testing.T) {
	// When old_string is empty, should show "Added N lines"
	result := formatDiffSummary("", "line1\nline2\nline3")
	stripped := stripANSI(result)

	if !strings.Contains(stripped, "Added 3 lines") {
		t.Errorf("Expected 'Added 3 lines', got: %s", stripped)
	}
	// Should contain the corner character
	if !strings.Contains(result, "\u2514") {
		t.Errorf("Expected corner character, got: %s", result)
	}
}

func TestFormatDiffSummary_AddedOneLine(t *testing.T) {
	// Single line should use singular "line"
	result := formatDiffSummary("", "single line")
	stripped := stripANSI(result)

	if !strings.Contains(stripped, "Added 1 line") {
		t.Errorf("Expected 'Added 1 line', got: %s", stripped)
	}
}

func TestFormatDiffSummary_RemovedOnly(t *testing.T) {
	// When new_string is empty, should show "Removed N lines"
	result := formatDiffSummary("line1\nline2", "")
	stripped := stripANSI(result)

	if !strings.Contains(stripped, "Removed 2 lines") {
		t.Errorf("Expected 'Removed 2 lines', got: %s", stripped)
	}
}

func TestFormatDiffSummary_RemovedOneLine(t *testing.T) {
	// Single line should use singular "line"
	result := formatDiffSummary("single line", "")
	stripped := stripANSI(result)

	if !strings.Contains(stripped, "Removed 1 line") {
		t.Errorf("Expected 'Removed 1 line', got: %s", stripped)
	}
}

func TestFormatDiffSummary_Changed(t *testing.T) {
	// When both have content, should show "-N +M lines"
	result := formatDiffSummary("old1\nold2", "new1\nnew2\nnew3")
	stripped := stripANSI(result)

	if !strings.Contains(stripped, "-2 +3 lines") {
		t.Errorf("Expected '-2 +3 lines', got: %s", stripped)
	}
}

func TestFormatDiffSummary_IgnoresEmptyLines(t *testing.T) {
	// Empty lines should not be counted
	result := formatDiffSummary("", "line1\n\n\nline2")
	stripped := stripANSI(result)

	if !strings.Contains(stripped, "Added 2 lines") {
		t.Errorf("Expected 'Added 2 lines' (ignoring empty lines), got: %s", stripped)
	}
}

func TestFormatDiff_HasLineNumbers(t *testing.T) {
	// Test that diff lines have line numbers
	result := formatDiff("old line", "new line", 80, "")
	output := strings.Join(result, "\n")
	stripped := stripANSI(output)

	// Should contain line numbers with diff markers
	if !strings.Contains(stripped, "1 -") {
		t.Errorf("Expected '1 -' for deletion line number, got: %s", stripped)
	}
	if !strings.Contains(stripped, "1 +") {
		t.Errorf("Expected '1 +' for addition line number, got: %s", stripped)
	}
}

func TestFormatDiff_MultipleLineNumbers(t *testing.T) {
	// Test line numbers increment correctly
	result := formatDiff("line1\nline2", "newA\nnewB\nnewC", 80, "")
	output := strings.Join(result, "\n")
	stripped := stripANSI(output)

	// Check deletion line numbers
	if !strings.Contains(stripped, "1 - line1") {
		t.Errorf("Expected '1 - line1', got: %s", stripped)
	}
	if !strings.Contains(stripped, "2 - line2") {
		t.Errorf("Expected '2 - line2', got: %s", stripped)
	}

	// Check addition line numbers
	if !strings.Contains(stripped, "1 + newA") {
		t.Errorf("Expected '1 + newA', got: %s", stripped)
	}
	if !strings.Contains(stripped, "2 + newB") {
		t.Errorf("Expected '2 + newB', got: %s", stripped)
	}
	if !strings.Contains(stripped, "3 + newC") {
		t.Errorf("Expected '3 + newC', got: %s", stripped)
	}
}

func TestFormatDiff_SkipsEmptyLinesButCountsThem(t *testing.T) {
	// Empty lines should be skipped in output but counted for line numbers
	result := formatDiff("line1\n\nline3", "", 80, "")
	output := strings.Join(result, "\n")
	stripped := stripANSI(output)

	// Line 1 and line 3 should be present, line 2 (empty) skipped
	if !strings.Contains(stripped, "1 - line1") {
		t.Errorf("Expected '1 - line1', got: %s", stripped)
	}
	if !strings.Contains(stripped, "3 - line3") {
		t.Errorf("Expected '3 - line3' (line 2 was empty), got: %s", stripped)
	}
}

func TestFormatToolUse_TodoWrite(t *testing.T) {
	// Create a mock entry with TodoWrite tool use
	entry := claude.Entry{
		Type: "assistant",
		Message: claude.Message{
			Content: []interface{}{
				map[string]interface{}{
					"type": "tool_use",
					"name": "TodoWrite",
					"input": map[string]interface{}{
						"todos": []interface{}{
							map[string]interface{}{
								"content":    "First task",
								"status":     "completed",
								"activeForm": "Doing first task",
							},
							map[string]interface{}{
								"content":    "Second task",
								"status":     "in_progress",
								"activeForm": "Doing second task",
							},
							map[string]interface{}{
								"content":    "Third task",
								"status":     "pending",
								"activeForm": "Doing third task",
							},
						},
					},
				},
			},
		},
	}

	result := formatToolUse(entry, "TodoWrite", 80)
	output := strings.Join(result, "\n")
	stripped := stripANSI(output)

	// Should contain TodoWrite header
	if !strings.Contains(stripped, "TodoWrite") {
		t.Errorf("Expected 'TodoWrite' in output, got: %s", stripped)
	}

	// Should contain each todo content
	if !strings.Contains(stripped, "First task") {
		t.Errorf("Expected 'First task' in output, got: %s", stripped)
	}
	if !strings.Contains(stripped, "Second task") {
		t.Errorf("Expected 'Second task' in output, got: %s", stripped)
	}
	if !strings.Contains(stripped, "Third task") {
		t.Errorf("Expected 'Third task' in output, got: %s", stripped)
	}

	// Should contain status indicators (checkmark, half circle, empty box)
	if !strings.Contains(output, "\u2713") { // checkmark for completed
		t.Errorf("Expected checkmark for completed task, got: %s", output)
	}
	if !strings.Contains(output, "\u25d0") { // half circle for in_progress
		t.Errorf("Expected half circle for in_progress task, got: %s", output)
	}
	if !strings.Contains(output, "\u2610") { // empty box for pending
		t.Errorf("Expected empty box for pending task, got: %s", output)
	}
}

func TestFormatToolUse_TodoWriteEmpty(t *testing.T) {
	// Test TodoWrite with no todos
	entry := claude.Entry{
		Type: "assistant",
		Message: claude.Message{
			Content: []interface{}{
				map[string]interface{}{
					"type": "tool_use",
					"name": "TodoWrite",
					"input": map[string]interface{}{
						"todos": []interface{}{},
					},
				},
			},
		},
	}

	result := formatToolUse(entry, "TodoWrite", 80)
	output := strings.Join(result, "\n")
	stripped := stripANSI(output)

	// Should still contain TodoWrite header even with no todos
	if !strings.Contains(stripped, "TodoWrite") {
		t.Errorf("Expected 'TodoWrite' in output, got: %s", stripped)
	}

	// Should only have one line (the header)
	if len(result) != 1 {
		t.Errorf("Expected 1 line for empty todo list, got: %d", len(result))
	}
}

func TestUpdate_KKeyAtTopBoundary_DoesNotScrollContent(t *testing.T) {
	// Test that when K is pressed at top of sidebar, it doesn't scroll content panel
	agents := createTestAgents(5)
	m := createModelWithAgents(agents, 80, 40)
	m.focusedPanel = panelLeft
	m.selectedIdx = 0 // Already at top

	// Set up viewport with some content that can be scrolled
	m.viewport.SetContent("Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7\nLine 8\nLine 9\nLine 10")
	m.viewport.Height = 5
	m.viewport.GotoBottom() // Scroll to bottom
	initialYOffset := m.viewport.YOffset

	// Press K while at top of sidebar
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	newModel, _ := m.Update(msg)
	updatedModel := newModel.(Model)

	// Viewport offset should NOT have changed
	if updatedModel.viewport.YOffset != initialYOffset {
		t.Errorf("Viewport should not scroll when K pressed at sidebar top. YOffset was %d, now %d",
			initialYOffset, updatedModel.viewport.YOffset)
	}

	// Selection should still be at 0
	if updatedModel.selectedIdx != 0 {
		t.Errorf("Selection should remain at 0 when K pressed at top, got %d", updatedModel.selectedIdx)
	}
}

func TestUpdate_JKeyAtBottomBoundary_DoesNotScrollContent(t *testing.T) {
	// Test that when J is pressed at bottom of sidebar, it doesn't scroll content panel
	agents := createTestAgents(5)
	m := createModelWithAgents(agents, 80, 40)
	m.focusedPanel = panelLeft
	// With channel headers, total items = 1 header + 5 agents = 6 items
	// Bottom is index 5 (last agent)
	bottomIdx := m.totalSidebarItems() - 1
	m.selectedIdx = bottomIdx

	// Set up viewport with some content that can be scrolled
	m.viewport.SetContent("Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7\nLine 8\nLine 9\nLine 10")
	m.viewport.Height = 5
	m.viewport.GotoTop() // Scroll to top
	initialYOffset := m.viewport.YOffset

	// Press J while at bottom of sidebar
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	newModel, _ := m.Update(msg)
	updatedModel := newModel.(Model)

	// Viewport offset should NOT have changed
	if updatedModel.viewport.YOffset != initialYOffset {
		t.Errorf("Viewport should not scroll when J pressed at sidebar bottom. YOffset was %d, now %d",
			initialYOffset, updatedModel.viewport.YOffset)
	}

	// Selection should still be at bottom
	if updatedModel.selectedIdx != bottomIdx {
		t.Errorf("Selection should remain at bottom when J pressed at bottom, got %d", updatedModel.selectedIdx)
	}
}

func TestUpdate_KKeyInMiddleOfSidebar_DoesNotScrollContent(t *testing.T) {
	// Test that when K is pressed in middle of sidebar, it navigates sidebar but doesn't scroll content
	agents := createTestAgents(5)
	m := createModelWithAgents(agents, 80, 40)
	m.focusedPanel = panelLeft
	m.selectedIdx = 2 // In the middle

	// Set up viewport with some content that can be scrolled
	m.viewport.SetContent("Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7\nLine 8\nLine 9\nLine 10")
	m.viewport.Height = 5
	m.viewport.GotoBottom() // Scroll to bottom
	initialYOffset := m.viewport.YOffset

	// Press K in middle of sidebar
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	newModel, _ := m.Update(msg)
	updatedModel := newModel.(Model)

	// Viewport offset should NOT have changed
	if updatedModel.viewport.YOffset != initialYOffset {
		t.Errorf("Viewport should not scroll when K pressed in sidebar. YOffset was %d, now %d",
			initialYOffset, updatedModel.viewport.YOffset)
	}

	// Selection should have moved up
	if updatedModel.selectedIdx != 1 {
		t.Errorf("Selection should have moved up from 2 to 1, got %d", updatedModel.selectedIdx)
	}
}

func TestRenderSidebarShowsDescription(t *testing.T) {
	agents := []claude.Agent{
		{ID: "abc123", Description: "Fix login bug"},
		{ID: "def456", Description: ""},
	}
	m := createModelWithAgents(agents, 80, 24)

	content := m.renderSidebarContent(20, 10)

	// Should show description when available
	if !strings.Contains(content, "Fix login bug") {
		t.Errorf("expected sidebar to contain description, got: %s", content)
	}

	// Should fall back to ID when no description
	if !strings.Contains(content, "def456") {
		t.Errorf("expected sidebar to contain agent ID when no description, got: %s", content)
	}
}

func TestViewShowsDescriptionAndIDInRightPanel(t *testing.T) {
	agents := []claude.Agent{
		{ID: "abc12345", Description: "Fix login bug", FilePath: "/tmp/test.jsonl"},
	}
	m := createModelWithAgents(agents, 80, 24)
	// With channel headers, index 0 is the header, index 1 is the first agent
	m.selectedIdx = 1

	view := m.View()

	// Should show "Description (ID) | timestamp" format
	if !strings.Contains(view, "Fix login bug") {
		t.Errorf("expected description in right panel, got: %s", view)
	}
	if !strings.Contains(view, "(abc12345)") {
		t.Errorf("expected ID in parentheses in right panel, got: %s", view)
	}
}

func TestViewShowsOnlyIDWhenNoDescription(t *testing.T) {
	agents := []claude.Agent{
		{ID: "abc12345", Description: "", FilePath: "/tmp/test.jsonl"},
	}
	m := createModelWithAgents(agents, 80, 24)
	// With channel headers, index 0 is the header, index 1 is the first agent
	m.selectedIdx = 1

	view := m.View()

	// Should fall back to just ID when no description
	if !strings.Contains(view, "abc12345") {
		t.Errorf("expected agent ID in right panel, got: %s", view)
	}
	// Should NOT have empty parentheses
	if strings.Contains(view, "()") {
		t.Errorf("should not show empty parentheses when no description, got: %s", view)
	}
}
