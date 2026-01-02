package tui

import (
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
	m := NewModel("/test")
	m.width = 80
	m.height = 24

	// Set up agents and transcripts like the real application
	m.agents = []claude.Agent{
		{ID: "test-agent", FilePath: "/test/agent.jsonl"},
	}
	m.selectedIdx = 0

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
	content := strings.Join(m.contentLines, "\n")
	if !strings.Contains(content, "Hello world") {
		t.Errorf("Expected contentLines to contain 'Hello world', got: %s", content)
	}
}

func TestModel_ScreenToContentPosition(t *testing.T) {
	m := NewModel("/test")
	m.width = 80
	m.height = 24
	m.focusedPanel = panelRight

	// Set up viewport dimensions (simulate layout)
	m.viewport.Width = 50
	m.viewport.Height = 10
	m.viewport.YOffset = 0

	// Set content
	content := "Short\nA longer line here\nThird"
	m.viewport.SetContent(content)
	m.contentLines = strings.Split(content, "\n")

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
	m := NewModel("/test")
	m.width = 80
	m.height = 24
	m.focusedPanel = panelRight
	m.viewport.Width = 50
	m.viewport.Height = 10

	content := "Line one\nLine two\nLine three"
	m.viewport.SetContent(content)
	m.contentLines = strings.Split(content, "\n")

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
	m := NewModel("/test")
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

	content := "Line one\nLine two\nLine three"
	m.viewport.SetContent(content)
	m.contentLines = strings.Split(content, "\n")

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
	m := NewModel("/test")
	m.width = 80
	m.height = 24
	m.focusedPanel = panelRight
	m.viewport.Width = 50
	m.viewport.Height = 10

	content := "Line one\nLine two\nLine three"
	m.viewport.SetContent(content)
	m.contentLines = strings.Split(content, "\n")

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
	m := NewModel("/test")
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
	m := NewModel("/test")
	m.width = 80
	m.height = 24
	m.selection = SelectionState{
		Active:  true,
		Anchor:  Position{Row: 0, Col: 0},
		Current: Position{Row: 0, Col: 5},
	}
	m.contentLines = []string{"Hello World", "Line two"}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	newModel, _ := m.Update(msg)
	updated := newModel.(Model)

	// Selection should be cleared after copy
	if updated.selection.Active {
		t.Error("Expected selection to be inactive after copy")
	}
}

func TestUpdate_NavigationKeysBlockedInSelectionMode(t *testing.T) {
	m := NewModel("/test")
	m.selection = SelectionState{Active: true, Anchor: Position{Row: 1, Col: 5}, Current: Position{Row: 2, Col: 10}}
	m.selectedIdx = 1
	m.agents = []claude.Agent{{ID: "a1", FilePath: "/tmp/a1.jsonl"}, {ID: "a2", FilePath: "/tmp/a2.jsonl"}}

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
