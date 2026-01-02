package tui

import (
	"strings"
	"testing"

	"june/internal/claude"
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
