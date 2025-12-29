package tui

import (
	"database/sql"
	"testing"

	"otto/internal/repo"

	tea "github.com/charmbracelet/bubbletea"
	_ "modernc.org/sqlite"
)

func TestViewportHeightUpdatesWhenChatInputShows(t *testing.T) {
	// This test reproduces the bug where viewport dimensions are not recalculated
	// when activeChannelID changes to a project header (which shows chat input).
	// The viewport height should be 1 line smaller when chat input is visible.

	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	// Set window size
	m.width = 80
	m.height = 24

	// Initially select an agent (no chat input shown)
	m.activeChannelID = "impl-1"
	m.updateViewportDimensions() // Simulate what happens in activateSelection()

	// Calculate initial viewport dimensions
	_, _, _, contentHeight := m.layout()
	initialHeight := contentHeight

	// Verify chat input is NOT shown for agent
	if m.showChatInput() {
		t.Fatal("expected chat input to be hidden for agent selection")
	}

	// Now change to a project header (should show chat input)
	m.activeChannelID = "otto/main"
	m.updateViewportDimensions() // This is the fix - should be called when activeChannelID changes

	// BUG: viewport.Height is not updated when activeChannelID changes
	// It still has the old height from when chat input was hidden

	// Verify chat input IS shown for project header
	if !m.showChatInput() {
		t.Fatal("expected chat input to be shown for project header")
	}

	// Calculate new layout dimensions - contentHeight should be 1 less
	_, _, _, newContentHeight := m.layout()

	// When chat input is shown, contentHeight should be 1 line smaller
	expectedHeightDifference := 1
	actualHeightDifference := initialHeight - newContentHeight

	if actualHeightDifference != expectedHeightDifference {
		t.Errorf("expected content height to decrease by %d when chat input shows, got decrease of %d",
			expectedHeightDifference, actualHeightDifference)
	}

	// The BUG: m.viewport.Height was not updated when activeChannelID changed
	// It should match newContentHeight, but it still has the old value
	if m.viewport.Height != newContentHeight {
		t.Errorf("expected viewport.Height to be %d after showing chat input, got %d (viewport dimensions not updated)",
			newContentHeight, m.viewport.Height)
	}

	// Test the reverse: changing from project header back to agent
	m.activeChannelID = "impl-1"
	m.updateViewportDimensions() // Should be called when activeChannelID changes

	// Verify chat input is hidden again
	if m.showChatInput() {
		t.Fatal("expected chat input to be hidden again for agent")
	}

	// Calculate dimensions again
	_, _, _, backToAgentHeight := m.layout()

	// Should be back to original height
	if backToAgentHeight != initialHeight {
		t.Errorf("expected content height to return to %d when hiding chat input, got %d",
			initialHeight, backToAgentHeight)
	}

	// Viewport height should be updated to match
	if m.viewport.Height != backToAgentHeight {
		t.Errorf("expected viewport.Height to be %d after hiding chat input, got %d (viewport dimensions not updated)",
			backToAgentHeight, m.viewport.Height)
	}
}

// Bug 1: Tab key swallowed by textinput
func TestTabKeySwitchesPanels(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	channels := m.sidebarItems()
	// Find project header index
	headerIndex := -1
	for i, ch := range channels {
		if ch.ID == "otto/main" && ch.Kind == SidebarChannelHeader {
			headerIndex = i
			break
		}
	}
	if headerIndex == -1 {
		t.Fatal("expected to find otto/main header")
	}

	// Select project header (shows chat input)
	m.cursorIndex = headerIndex
	m.activeChannelID = "otto/main"
	m.focusedPanel = panelMessages
	m.chatInput.Focus()

	// Chat input is visible and focused
	if !m.showChatInput() {
		t.Fatal("expected chat input to be visible for project header")
	}

	// Send Tab key
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(model)

	// BUG: Tab is swallowed by textinput, focusedPanel stays at panelMessages
	// FIX: Tab should switch focus to panelAgents
	if m.focusedPanel != panelAgents {
		t.Errorf("expected focusedPanel to be panelAgents after Tab, got %d", m.focusedPanel)
	}
}

// Bug 2: Chat cursor not showing when clicking project header
func TestProjectHeaderClickFocusesChatInput(t *testing.T) {
	m := NewModel(nil)
	m.width = 80
	m.height = 24
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	channels := m.sidebarItems()
	headerIndex := -1
	for i, ch := range channels {
		if ch.ID == "otto/main" && ch.Kind == SidebarChannelHeader {
			headerIndex = i
			break
		}
	}
	if headerIndex == -1 {
		t.Fatal("expected to find otto/main header")
	}

	// Simulate mouse click on project header (not on caret)
	// X=10 is past the caret area (which is X=1-2 for Level 0)
	mouseMsg := tea.MouseMsg{
		X:      10,
		Y:      headerIndex + 2, // +2 for border + title
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
	}
	updated, _ := m.Update(mouseMsg)
	m = updated.(model)

	// Clicking project header should focus the messages panel and chat input
	if m.focusedPanel != panelMessages {
		t.Errorf("expected focusedPanel to be panelMessages after clicking project header, got %d", m.focusedPanel)
	}

	if !m.chatInput.Focused() {
		t.Error("expected chatInput to be focused after clicking project header")
	}
}

// Bug 3: Clicking caret doesn't toggle expand/collapse
func TestCaretClickTogglesExpand(t *testing.T) {
	m := NewModel(nil)
	m.width = 80
	m.height = 24
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	channels := m.sidebarItems()
	headerIndex := -1
	for i, ch := range channels {
		if ch.ID == "otto/main" && ch.Kind == SidebarChannelHeader {
			headerIndex = i
			break
		}
	}
	if headerIndex == -1 {
		t.Fatal("expected to find otto/main header")
	}

	// Project is expanded by default
	if !m.isProjectExpanded("otto/main") {
		t.Fatal("expected otto/main to be expanded by default")
	}

	// Simulate clicking on the caret area (X position 1-2 for Level 0 header)
	// The caret is rendered at the start of the line after the border
	// Border is at X=0, so caret is at X=1-2 (▼ takes 1 char + 1 space)
	m.cursorIndex = headerIndex

	// BUG: Currently, clicking anywhere on the header calls activateSelection()
	// which just sets activeChannelID, doesn't toggle expand/collapse
	// FIX: When clicking on caret area, should call toggleSelection() instead

	// Simulate mouse click at caret position (X=1, Y is headerIndex+2 for border+title)
	mouseMsg := tea.MouseMsg{
		X:      1,
		Y:      headerIndex + 2,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
	}
	updated, _ := m.Update(mouseMsg)
	m = updated.(model)

	// Should toggle to collapsed
	if m.isProjectExpanded("otto/main") {
		t.Error("expected otto/main to be collapsed after clicking caret")
	}

	// Click again should toggle back to expanded
	updated, _ = m.Update(mouseMsg)
	m = updated.(model)

	if !m.isProjectExpanded("otto/main") {
		t.Error("expected otto/main to be expanded after clicking caret again")
	}
}

// Clicking empty space in left panel should focus the left panel
func TestClickEmptySpaceInLeftPanelFocusesPanel(t *testing.T) {
	m := NewModel(nil)
	m.width = 80
	m.height = 24
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	// Start with focus on messages panel
	m.focusedPanel = panelMessages

	// Click on empty space in left panel (Y position beyond any channels)
	// With 1 agent, channels are: header (Y=2), agent (Y=3), so Y=10 is empty
	mouseMsg := tea.MouseMsg{
		X:      5, // In left panel (left panel is ~20 chars wide)
		Y:      10,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
	}
	updated, _ := m.Update(mouseMsg)
	m = updated.(model)

	if m.focusedPanel != panelAgents {
		t.Errorf("expected focusedPanel to be panelAgents after clicking empty space in left panel, got %d", m.focusedPanel)
	}
}

// Keyboard navigation to project header should NOT change focus
func TestKeyboardNavToProjectHeaderKeepsFocus(t *testing.T) {
	m := NewModel(nil)
	m.width = 80
	m.height = 24
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	// Start with focus on agents panel
	m.focusedPanel = panelAgents
	m.cursorIndex = 0

	// Navigate with j/k - this calls moveCursor which calls activateSelection
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	updated, _ := m.Update(keyMsg)
	m = updated.(model)

	// Focus should still be on agents panel (not switched to messages)
	if m.focusedPanel != panelAgents {
		t.Errorf("expected focusedPanel to remain panelAgents after keyboard nav, got %d", m.focusedPanel)
	}
}

// Clicking in right panel should focus the right panel
func TestClickRightPanelFocusesPanel(t *testing.T) {
	m := NewModel(nil)
	m.width = 80
	m.height = 24
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	// Start with focus on agents panel
	m.focusedPanel = panelAgents

	// Click in right panel (X > left panel width ~20)
	mouseMsg := tea.MouseMsg{
		X:      40, // Well into right panel
		Y:      10,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
	}
	updated, _ := m.Update(mouseMsg)
	m = updated.(model)

	if m.focusedPanel != panelMessages {
		t.Errorf("expected focusedPanel to be panelMessages after clicking in right panel, got %d", m.focusedPanel)
	}
}

func TestRightPanelRoutesKeysToInput(t *testing.T) {
	// Create in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	m := NewModel(db)
	m.focusedPanel = panelMessages
	m.chatInput.Focus() // Focus the input first

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	next, _ := m.Update(msg)
	model := next.(model)

	if model.chatInput.Value() != "j" {
		t.Fatalf("expected chat input to capture key, got %q (focused panel: %d, focused: %v)", model.chatInput.Value(), model.focusedPanel, model.chatInput.Focused())
	}
}

func TestRightPanelEscReturnsToSidebar(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	m := NewModel(db)
	m.focusedPanel = panelMessages
	m.chatInput.Focus() // Focus the input first

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := next.(model)

	if model.focusedPanel != panelAgents {
		t.Fatalf("expected sidebar focus, got %v", model.focusedPanel)
	}
	if model.chatInput.Focused() {
		t.Fatalf("expected chat input to be unfocused, but it was focused")
	}
}

func TestRightPanelTabReturnsToSidebar(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	m := NewModel(db)
	m.focusedPanel = panelMessages
	m.chatInput.Focus() // Focus the input first

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := next.(model)

	if model.focusedPanel != panelAgents {
		t.Fatalf("expected sidebar focus, got %v", model.focusedPanel)
	}
	if model.chatInput.Focused() {
		t.Fatalf("expected chat input to be unfocused, but it was focused")
	}
}

func TestRightPanelIgnoresScrollKeys(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	m := NewModel(db)
	m.width = 80
	m.height = 24
	m.focusedPanel = panelMessages
	m.chatInput.Focus()

	// Set up viewport with some initial position
	m.updateViewportDimensions()
	m.viewport.YOffset = 5

	// Try j key (should NOT scroll viewport)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	updated := next.(model)

	if updated.viewport.YOffset != 5 {
		t.Fatalf("expected viewport YOffset to remain 5 after 'j' key, got %d", updated.viewport.YOffset)
	}

	// Try k key (should NOT scroll viewport)
	next, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	updated = next.(model)

	if updated.viewport.YOffset != 5 {
		t.Fatalf("expected viewport YOffset to remain 5 after 'k' key, got %d", updated.viewport.YOffset)
	}

	// Try g key (should NOT scroll viewport to top)
	next, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	updated = next.(model)

	if updated.viewport.YOffset != 5 {
		t.Fatalf("expected viewport YOffset to remain 5 after 'g' key, got %d", updated.viewport.YOffset)
	}

	// Try G key (should NOT scroll viewport to bottom)
	next, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	updated = next.(model)

	if updated.viewport.YOffset != 5 {
		t.Fatalf("expected viewport YOffset to remain 5 after 'G' key, got %d", updated.viewport.YOffset)
	}
}

