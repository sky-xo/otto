package tui

import (
	"fmt"
	"strings"
	"testing"

	"june/internal/claude"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSelectionState_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		state    SelectionState
		expected bool
	}{
		{
			name:     "inactive selection is empty",
			state:    SelectionState{Active: false},
			expected: true,
		},
		{
			name:     "same anchor and current is empty",
			state:    SelectionState{Active: true, Anchor: Position{Row: 5, Col: 10}, Current: Position{Row: 5, Col: 10}},
			expected: true,
		},
		{
			name:     "different positions is not empty",
			state:    SelectionState{Active: true, Anchor: Position{Row: 5, Col: 10}, Current: Position{Row: 5, Col: 15}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.IsEmpty(); got != tt.expected {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSelectionState_Normalize(t *testing.T) {
	tests := []struct {
		name          string
		anchor        Position
		current       Position
		expectedStart Position
		expectedEnd   Position
	}{
		{
			name:          "anchor before current",
			anchor:        Position{Row: 2, Col: 5},
			current:       Position{Row: 4, Col: 10},
			expectedStart: Position{Row: 2, Col: 5},
			expectedEnd:   Position{Row: 4, Col: 10},
		},
		{
			name:          "anchor after current (different rows)",
			anchor:        Position{Row: 4, Col: 10},
			current:       Position{Row: 2, Col: 5},
			expectedStart: Position{Row: 2, Col: 5},
			expectedEnd:   Position{Row: 4, Col: 10},
		},
		{
			name:          "same row anchor after current",
			anchor:        Position{Row: 3, Col: 15},
			current:       Position{Row: 3, Col: 5},
			expectedStart: Position{Row: 3, Col: 5},
			expectedEnd:   Position{Row: 3, Col: 15},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := SelectionState{Active: true, Anchor: tt.anchor, Current: tt.current}
			start, end := s.Normalize()
			if start != tt.expectedStart || end != tt.expectedEnd {
				t.Errorf("Normalize() = (%v, %v), want (%v, %v)", start, end, tt.expectedStart, tt.expectedEnd)
			}
		})
	}
}

func TestModel_ContentLines(t *testing.T) {
	m := NewModel("/claude", "/test", "test-repo")
	m.width = 80
	m.height = 24

	// Set up channels and transcripts like the real application
	testAgent := claude.Agent{ID: "test-agent", FilePath: "/test/agent.jsonl"}
	m.channels = []claude.Channel{
		{Name: "test-channel", Agents: []claude.Agent{testAgent}},
	}
	m.selectedIdx = 1 // First agent (index 0 is header)
	m.lastViewedAgent = &m.channels[0].Agents[0]

	// Add transcript entries for the selected agent
	m.transcripts["test-agent"] = []claude.Entry{
		{
			Type:    "user",
			Message: claude.Message{Content: "Hello world"},
		},
	}

	// Call updateViewport to populate contentLines
	m.updateViewport()

	// Verify contentLines was populated
	if len(m.contentLines) == 0 {
		t.Errorf("Expected contentLines to be populated, got empty slice")
	}

	// Verify the content contains the user message
	// Convert StyledLines back to strings for checking
	var lines []string
	for _, line := range m.contentLines {
		lines = append(lines, line.String())
	}
	content := strings.Join(lines, "\n")
	if !strings.Contains(content, "Hello world") {
		t.Errorf("Expected contentLines to contain 'Hello world', got: %s", content)
	}
}

func TestModel_ScreenToContentPosition(t *testing.T) {
	m := NewModel("/claude", "/test", "test-repo")
	m.width = 80
	m.height = 24
	m.focusedPanel = panelRight

	// Set up viewport dimensions (simulate layout)
	m.viewport.Width = 50
	m.viewport.Height = 10
	m.viewport.YOffset = 0

	// Set content using StyledLines (plain text, no ANSI)
	lines := []string{"Short", "A longer line here", "Third"}
	m.contentLines = make([]StyledLine, len(lines))
	for i, line := range lines {
		m.contentLines[i] = ParseStyledLine(line)
	}
	m.viewport.SetContent(strings.Join(lines, "\n"))

	tests := []struct {
		name        string
		screenX     int
		screenY     int
		expectedRow int
		expectedCol int
	}{
		{
			name:        "first character of first line",
			screenX:     sidebarWidth + 1, // After sidebar + left border
			screenY:     1,                // After top border
			expectedRow: 0,
			expectedCol: 0,
		},
		{
			name:        "middle of second line",
			screenX:     sidebarWidth + 6, // 5 chars into content
			screenY:     2,                // Second content line
			expectedRow: 1,
			expectedCol: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := m.screenToContentPosition(tt.screenX, tt.screenY)
			if pos.Row != tt.expectedRow || pos.Col != tt.expectedCol {
				t.Errorf("screenToContentPosition(%d, %d) = {Row:%d, Col:%d}, want {Row:%d, Col:%d}",
					tt.screenX, tt.screenY, pos.Row, pos.Col, tt.expectedRow, tt.expectedCol)
			}
		})
	}
}

func TestUpdate_MouseDragStartsSelection(t *testing.T) {
	m := NewModel("/claude", "/test", "test-repo")
	m.width = 80
	m.height = 24
	m.focusedPanel = panelRight
	m.viewport.Width = 50
	m.viewport.Height = 10

	lines := []string{"Line one", "Line two", "Line three"}
	m.contentLines = make([]StyledLine, len(lines))
	for i, line := range lines {
		m.contentLines[i] = ParseStyledLine(line)
	}
	m.viewport.SetContent(strings.Join(lines, "\n"))

	// Simulate mouse press in content area
	pressMsg := tea.MouseMsg{
		X:      sidebarWidth + 5,
		Y:      2,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}

	newModel, _ := m.Update(pressMsg)
	updated := newModel.(Model)

	if !updated.selection.Active {
		t.Error("Expected selection to be active after mouse press")
	}
	if !updated.selection.Dragging {
		t.Error("Expected dragging to be true after mouse press")
	}

	// Verify anchor and current positions are set correctly
	// X = sidebarWidth + 5, so col = 5 - 1 = 4 (subtracting panel border)
	// Y = 2, so row = 2 - 1 + 0 (viewport offset) = 1
	expectedPos := Position{Row: 1, Col: 4}
	if updated.selection.Anchor != expectedPos {
		t.Errorf("Expected Anchor %+v, got %+v", expectedPos, updated.selection.Anchor)
	}
	if updated.selection.Current != expectedPos {
		t.Errorf("Expected Current %+v, got %+v", expectedPos, updated.selection.Current)
	}
}

func TestUpdate_MouseReleaseStopsDragging(t *testing.T) {
	m := NewModel("/claude", "/test", "test-repo")
	m.width = 80
	m.height = 24
	m.focusedPanel = panelRight
	m.viewport.Width = 50
	m.viewport.Height = 10
	m.selection = SelectionState{
		Active:   true,
		Dragging: true,
		Anchor:   Position{Row: 1, Col: 5},
		Current:  Position{Row: 1, Col: 10},
	}

	lines := []string{"Line one", "Line two", "Line three"}
	m.contentLines = make([]StyledLine, len(lines))
	for i, line := range lines {
		m.contentLines[i] = ParseStyledLine(line)
	}
	m.viewport.SetContent(strings.Join(lines, "\n"))

	// Simulate mouse release at a new position
	releaseMsg := tea.MouseMsg{
		X:      sidebarWidth + 15,
		Y:      2,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
	}

	newModel, _ := m.Update(releaseMsg)
	updated := newModel.(Model)

	if !updated.selection.Active {
		t.Error("Selection should remain active after release")
	}
	if updated.selection.Dragging {
		t.Error("Dragging should be false after release")
	}

	// Verify anchor is preserved from the original selection
	expectedAnchor := Position{Row: 1, Col: 5}
	if updated.selection.Anchor != expectedAnchor {
		t.Errorf("Expected Anchor to be preserved as %+v, got %+v", expectedAnchor, updated.selection.Anchor)
	}

	// Verify current position is updated to the release position
	// X = sidebarWidth + 15, so col = 15 - 1 = 14 (subtracting panel border)
	// Y = 2, so row = 2 - 1 + 0 (viewport offset) = 1
	// However, col is clamped to line length: "Line two" has length 8
	expectedCurrent := Position{Row: 1, Col: 8}
	if updated.selection.Current != expectedCurrent {
		t.Errorf("Expected Current %+v, got %+v", expectedCurrent, updated.selection.Current)
	}
}

func TestUpdate_MouseMotionUpdatesSelection(t *testing.T) {
	m := NewModel("/claude", "/test", "test-repo")
	m.width = 80
	m.height = 24
	m.focusedPanel = panelRight
	m.viewport.Width = 50
	m.viewport.Height = 10

	lines := []string{"Line one", "Line two", "Line three"}
	m.contentLines = make([]StyledLine, len(lines))
	for i, line := range lines {
		m.contentLines[i] = ParseStyledLine(line)
	}
	m.viewport.SetContent(strings.Join(lines, "\n"))

	// Set up an active dragging selection
	m.selection = SelectionState{
		Active:   true,
		Dragging: true,
		Anchor:   Position{Row: 0, Col: 2},
		Current:  Position{Row: 0, Col: 2},
	}

	// Simulate mouse motion to a new position
	motionMsg := tea.MouseMsg{
		X:      sidebarWidth + 10,
		Y:      3,
		Button: tea.MouseButtonNone,
		Action: tea.MouseActionMotion,
	}

	newModel, _ := m.Update(motionMsg)
	updated := newModel.(Model)

	// Verify selection remains active and dragging
	if !updated.selection.Active {
		t.Error("Selection should remain active during motion")
	}
	if !updated.selection.Dragging {
		t.Error("Dragging should remain true during motion")
	}

	// Verify anchor is preserved
	expectedAnchor := Position{Row: 0, Col: 2}
	if updated.selection.Anchor != expectedAnchor {
		t.Errorf("Expected Anchor to be preserved as %+v, got %+v", expectedAnchor, updated.selection.Anchor)
	}

	// Verify current position is updated to the motion position
	// X = sidebarWidth + 10, so col = 10 - 1 = 9 (subtracting panel border)
	// Y = 3, so row = 3 - 1 + 0 (viewport offset) = 2
	expectedCurrent := Position{Row: 2, Col: 9}
	if updated.selection.Current != expectedCurrent {
		t.Errorf("Expected Current %+v, got %+v", expectedCurrent, updated.selection.Current)
	}
}

func TestUpdate_EscapeCancelsSelection(t *testing.T) {
	m := NewModel("/claude", "/test", "test-repo")
	m.selection = SelectionState{
		Active:  true,
		Anchor:  Position{Row: 1, Col: 5},
		Current: Position{Row: 2, Col: 10},
	}

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	newModel, _ := m.Update(msg)
	updated := newModel.(Model)

	if updated.selection.Active {
		t.Error("Expected selection to be inactive after Escape")
	}
}

func TestUpdate_CKeyInSelectionModeCopies(t *testing.T) {
	m := NewModel("/claude", "/test", "test-repo")
	m.width = 80
	m.height = 24
	m.selection = SelectionState{
		Active:  true,
		Anchor:  Position{Row: 0, Col: 0},
		Current: Position{Row: 0, Col: 5},
	}
	lines := []string{"Hello World", "Line two"}
	m.contentLines = make([]StyledLine, len(lines))
	for i, line := range lines {
		m.contentLines[i] = ParseStyledLine(line)
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	newModel, _ := m.Update(msg)
	updated := newModel.(Model)

	// Selection should be cleared after copy
	if updated.selection.Active {
		t.Error("Expected selection to be inactive after copy")
	}
}

func TestUpdate_NavigationKeysBlockedInSelectionMode(t *testing.T) {
	m := NewModel("/claude", "/test", "test-repo")
	m.selection = SelectionState{Active: true, Anchor: Position{Row: 1, Col: 5}, Current: Position{Row: 2, Col: 10}}
	m.selectedIdx = 1
	m.channels = []claude.Channel{{Name: "ch", Agents: []claude.Agent{{ID: "a1", FilePath: "/tmp/a1.jsonl"}, {ID: "a2", FilePath: "/tmp/a2.jsonl"}}}}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	newModel, _ := m.Update(msg)
	updated := newModel.(Model)

	// Selection should still be active (key was blocked)
	if !updated.selection.Active {
		t.Error("Selection should remain active when navigation keys are pressed")
	}
	// selectedIdx should not change
	if updated.selectedIdx != 1 {
		t.Error("Navigation should be blocked in selection mode")
	}
}

func TestModel_GetSelectedText(t *testing.T) {
	m := NewModel("/claude", "/test", "test-repo")
	lines := []string{
		"First line of text",
		"Second line here",
		"Third line content",
	}
	m.contentLines = make([]StyledLine, len(lines))
	for i, line := range lines {
		m.contentLines[i] = ParseStyledLine(line)
	}

	tests := []struct {
		name     string
		anchor   Position
		current  Position
		expected string
	}{
		{
			name:     "single line partial",
			anchor:   Position{Row: 0, Col: 6},
			current:  Position{Row: 0, Col: 10},
			expected: "line",
		},
		{
			name:     "multiple lines",
			anchor:   Position{Row: 0, Col: 6},
			current:  Position{Row: 1, Col: 6},
			expected: "line of text\nSecond",
		},
		{
			name:     "reversed selection",
			anchor:   Position{Row: 1, Col: 6},
			current:  Position{Row: 0, Col: 6},
			expected: "line of text\nSecond",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.selection = SelectionState{
				Active:  true,
				Anchor:  tt.anchor,
				Current: tt.current,
			}
			got := m.getSelectedText()
			if got != tt.expected {
				t.Errorf("getSelectedText() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestModel_GetSelectedText_StripsANSI(t *testing.T) {
	m := NewModel("/claude", "/test", "test-repo")
	// Content with ANSI escape codes - ParseStyledLine will parse this
	m.contentLines = []StyledLine{
		ParseStyledLine("\x1b[32mGreen text\x1b[0m normal"),
	}
	m.selection = SelectionState{
		Active:  true,
		Anchor:  Position{Row: 0, Col: 0},
		Current: Position{Row: 0, Col: 17}, // "Green text normal" = 17 chars
	}

	got := m.getSelectedText()

	// Should not contain ANSI codes (StyledLine extracts plain text)
	if strings.Contains(got, "\x1b[") {
		t.Errorf("getSelectedText() should strip ANSI codes, got: %q", got)
	}
	// Should contain the text
	if !strings.Contains(got, "Green text") {
		t.Errorf("getSelectedText() should contain text, got: %q", got)
	}
}

func TestModel_ApplySelectionHighlight(t *testing.T) {
	m := NewModel("/claude", "/test", "test-repo")
	m.contentLines = []StyledLine{ParseStyledLine("Hello World")}
	m.selection = SelectionState{
		Active:  true,
		Anchor:  Position{Row: 0, Col: 0},
		Current: Position{Row: 0, Col: 5},
	}

	highlighted := m.applySelectionHighlight()

	// Should have exactly one line
	if len(highlighted) != 1 {
		t.Fatalf("Expected 1 line, got %d", len(highlighted))
	}

	// The highlighted version should contain the text
	plainText := highlighted[0].String()
	if !strings.Contains(plainText, "Hello") {
		t.Errorf("Expected 'Hello' in output, got: %s", plainText)
	}

	// The plain text content should still have the full text
	if plainText != "Hello World" {
		t.Errorf("Expected plain text to be 'Hello World', got: %s", plainText)
	}
}

func TestModel_ApplySelectionHighlight_MultipleLines(t *testing.T) {
	m := NewModel("/claude", "/test", "test-repo")
	lines := []string{"Line one", "Line two", "Line three"}
	m.contentLines = make([]StyledLine, len(lines))
	for i, line := range lines {
		m.contentLines[i] = ParseStyledLine(line)
	}
	m.selection = SelectionState{
		Active:  true,
		Anchor:  Position{Row: 0, Col: 5},
		Current: Position{Row: 2, Col: 4},
	}

	highlighted := m.applySelectionHighlight()

	// Should have all three lines
	if len(highlighted) != 3 {
		t.Fatalf("Expected 3 lines, got %d", len(highlighted))
	}

	// All lines should still contain their original text
	for i, line := range highlighted {
		plainText := line.String()
		original := m.contentLines[i].String()
		if plainText != original {
			t.Errorf("Line %d: expected %q, got %q", i, original, plainText)
		}
	}
}

func TestModel_ApplySelectionHighlight_EmptySelection(t *testing.T) {
	m := NewModel("/claude", "/test", "test-repo")
	m.contentLines = []StyledLine{ParseStyledLine("Hello World")}
	m.selection = SelectionState{
		Active:  true,
		Anchor:  Position{Row: 0, Col: 5},
		Current: Position{Row: 0, Col: 5}, // Same position = empty
	}

	highlighted := m.applySelectionHighlight()

	// Should return original content unchanged when selection is empty
	if len(highlighted) != len(m.contentLines) {
		t.Fatalf("Expected %d lines, got %d", len(m.contentLines), len(highlighted))
	}
	if highlighted[0].String() != m.contentLines[0].String() {
		t.Error("Empty selection should return original content unchanged")
	}
}

func TestModel_ApplySelectionHighlight_InactiveSelection(t *testing.T) {
	m := NewModel("/claude", "/test", "test-repo")
	m.contentLines = []StyledLine{ParseStyledLine("Hello World")}
	m.selection = SelectionState{
		Active:  false,
		Anchor:  Position{Row: 0, Col: 0},
		Current: Position{Row: 0, Col: 5},
	}

	highlighted := m.applySelectionHighlight()

	// Should return original content unchanged when selection is inactive
	if highlighted[0].String() != m.contentLines[0].String() {
		t.Error("Inactive selection should return original content unchanged")
	}
}

func TestView_ShowsSelectionIndicator(t *testing.T) {
	m := NewModel("/claude", "/test", "test-repo")
	m.width = 80
	m.height = 24
	testAgent := claude.Agent{ID: "test123", FilePath: "/tmp/test.jsonl"}
	m.channels = []claude.Channel{{Name: "ch", Agents: []claude.Agent{testAgent}}}
	m.selectedIdx = 1
	m.lastViewedAgent = &m.channels[0].Agents[0]
	m.selection = SelectionState{
		Active:  true,
		Anchor:  Position{Row: 0, Col: 0},
		Current: Position{Row: 0, Col: 5},
	}
	m.contentLines = []StyledLine{ParseStyledLine("Hello World")}

	view := m.View()

	// Should contain the selection indicator
	if !strings.Contains(view, "SELECTING") {
		t.Errorf("Expected 'SELECTING' in view when selection active, got: %s", view)
	}
	if !strings.Contains(view, "C: copy") {
		t.Errorf("Expected 'C: copy' hint in view, got: %s", view)
	}
}

func TestView_NoSelectionIndicatorWhenInactive(t *testing.T) {
	m := NewModel("/claude", "/test", "test-repo")
	m.width = 80
	m.height = 24
	testAgent := claude.Agent{ID: "test123", FilePath: "/tmp/test.jsonl"}
	m.channels = []claude.Channel{{Name: "ch", Agents: []claude.Agent{testAgent}}}
	m.selectedIdx = 1
	m.lastViewedAgent = &m.channels[0].Agents[0]
	m.selection = SelectionState{Active: false}
	m.contentLines = []StyledLine{ParseStyledLine("Hello World")}

	view := m.View()

	if strings.Contains(view, "SELECTING") {
		t.Errorf("Should not show 'SELECTING' when selection inactive")
	}
}

func TestUpdate_DragNearTopEdgeScrollsUp(t *testing.T) {
	m := NewModel("/claude", "/test", "test-repo")
	m.width = 80
	m.height = 24
	m.focusedPanel = panelRight
	m.viewport.Width = 50
	m.viewport.Height = 10
	m.viewport.YOffset = 5 // Scrolled down

	// Create content with enough lines
	rawLines := make([]string, 20)
	for i := range rawLines {
		rawLines[i] = fmt.Sprintf("Line %d content", i)
	}
	m.contentLines = make([]StyledLine, len(rawLines))
	for i, line := range rawLines {
		m.contentLines[i] = ParseStyledLine(line)
	}
	m.viewport.SetContent(strings.Join(rawLines, "\n"))

	m.selection = SelectionState{
		Active:   true,
		Dragging: true,
		Anchor:   Position{Row: 7, Col: 0},
		Current:  Position{Row: 7, Col: 5},
	}

	// Simulate drag near top edge (Y=1 is the border, Y=2 is first content line)
	msg := tea.MouseMsg{
		X:      sidebarWidth + 5,
		Y:      2, // Near top edge
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionMotion,
	}

	newModel, _ := m.Update(msg)
	updated := newModel.(Model)

	// Should have scrolled up
	if updated.viewport.YOffset >= 5 {
		t.Errorf("Expected viewport to scroll up from offset 5, got %d", updated.viewport.YOffset)
	}
}

func TestUpdate_ClickOutsideContentExitsSelection(t *testing.T) {
	m := NewModel("/claude", "/test", "test-repo")
	m.width = 80
	m.height = 24
	m.selection = SelectionState{
		Active:  true,
		Anchor:  Position{Row: 1, Col: 5},
		Current: Position{Row: 2, Col: 10},
	}

	// Click in left panel (sidebar)
	msg := tea.MouseMsg{
		X:      5, // In sidebar
		Y:      5,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
	}

	newModel, _ := m.Update(msg)
	updated := newModel.(Model)

	if updated.selection.Active {
		t.Error("Expected selection to be cancelled when clicking outside content area")
	}
}

func TestUpdate_ScrollPreservesSelection(t *testing.T) {
	m := NewModel("/claude", "/test", "test-repo")
	m.width = 80
	m.height = 24
	m.focusedPanel = panelRight
	m.viewport.Width = 50
	m.viewport.Height = 10

	// Create content with enough lines to scroll
	rawLines := make([]string, 30)
	for i := range rawLines {
		rawLines[i] = fmt.Sprintf("Line %d content here", i)
	}
	m.contentLines = make([]StyledLine, len(rawLines))
	for i, line := range rawLines {
		m.contentLines[i] = ParseStyledLine(line)
	}
	m.viewport.SetContent(strings.Join(rawLines, "\n"))

	// Set up a selection
	originalSelection := SelectionState{
		Active:  true,
		Anchor:  Position{Row: 5, Col: 3},
		Current: Position{Row: 7, Col: 10},
	}
	m.selection = originalSelection

	// Scroll down with wheel
	msg := tea.MouseMsg{
		X:      sidebarWidth + 10,
		Y:      5,
		Button: tea.MouseButtonWheelDown,
	}

	newModel, _ := m.Update(msg)
	updated := newModel.(Model)

	// Selection should be preserved
	if !updated.selection.Active {
		t.Error("Selection should remain active after scrolling")
	}
	if updated.selection.Anchor != originalSelection.Anchor {
		t.Errorf("Selection anchor changed: got %v, want %v", updated.selection.Anchor, originalSelection.Anchor)
	}
	if updated.selection.Current != originalSelection.Current {
		t.Errorf("Selection current changed: got %v, want %v", updated.selection.Current, originalSelection.Current)
	}
}
