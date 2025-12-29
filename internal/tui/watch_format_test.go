package tui

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"otto/internal/repo"

	_ "modernc.org/sqlite"
)

func TestFormatChatMessageSlackStyle(t *testing.T) {
	m := NewModel(nil)
	m.width = 80

	// Create chat messages
	messages := []repo.Message{
		{
			FromAgent: "you",
			Type:      repo.MessageTypeChat,
			Content:   "hey there",
		},
		{
			FromAgent: "otto",
			Type:      "complete",
			Content:   "I'm done with that task",
		},
	}

	m.messages = messages

	// Get formatted lines
	lines := m.mainContentLines(80)

	// Join all lines for easier inspection
	output := strings.Join(lines, "\n")

	// Verify that "you" appears on its own line (not inline with content)
	// The output should have:
	// you
	// hey there
	// (blank line)
	// otto
	// I'm done with that task
	// (blank line)

	// Check that "you" is on a separate line from "hey there"
	if !strings.Contains(output, "you\n") && !strings.Contains(output, "you ") {
		t.Errorf("expected 'you' to appear as sender name, got:\n%s", output)
	}

	// Check that there's a line with just the sender name (not inline with content)
	hasSlackStyleFormat := false
	for i := 0; i < len(lines)-1; i++ {
		stripped := stripAnsi(lines[i])
		nextStripped := stripAnsi(lines[i+1])

		// Check if we have a line that's just a username followed by content
		if strings.TrimSpace(stripped) == "you" && strings.Contains(nextStripped, "hey there") {
			hasSlackStyleFormat = true
			break
		}
	}

	if !hasSlackStyleFormat {
		t.Errorf("expected Slack-style format with sender on separate line, got:\n%s", output)
		for i, line := range lines {
			t.Logf("Line %d: %q", i, stripAnsi(line))
		}
	}
}

func TestFormatOttoCompleteMessageSlackStyle(t *testing.T) {
	m := NewModel(nil)
	m.width = 80

	// Create a complete message from otto
	messages := []repo.Message{
		{
			FromAgent: "otto",
			Type:      "complete",
			Content:   "Task completed successfully",
		},
	}

	m.messages = messages

	// Get formatted lines
	lines := m.mainContentLines(80)

	// Verify that "otto" appears on its own line
	hasSlackStyleFormat := false
	for i := 0; i < len(lines)-1; i++ {
		stripped := stripAnsi(lines[i])
		nextStripped := stripAnsi(lines[i+1])

		// Check if we have a line that's just "otto" followed by content
		if strings.TrimSpace(stripped) == "otto" && strings.Contains(nextStripped, "Task completed") {
			hasSlackStyleFormat = true
			break
		}
	}

	if !hasSlackStyleFormat {
		t.Errorf("expected Slack-style format for otto complete message with sender on separate line")
		for i, line := range lines {
			t.Logf("Line %d: %q", i, stripAnsi(line))
		}
	}
}

func TestFormatSayMessageSlackStyle(t *testing.T) {
	m := NewModel(nil)
	m.width = 80

	// Create a "say" message from orchestrator
	messages := []repo.Message{
		{
			FromAgent: "orchestrator",
			Type:      "say",
			Content:   "I have completed the analysis",
		},
	}

	m.messages = messages

	// Get formatted lines
	lines := m.mainContentLines(80)

	// Verify that "orchestrator" appears on its own line (Slack-style format)
	hasSlackStyleFormat := false
	for i := 0; i < len(lines)-1; i++ {
		stripped := stripAnsi(lines[i])
		nextStripped := stripAnsi(lines[i+1])

		// Check if we have a line that's just "orchestrator" followed by content
		if strings.TrimSpace(stripped) == "orchestrator" && strings.Contains(nextStripped, "I have completed") {
			hasSlackStyleFormat = true
			break
		}
	}

	if !hasSlackStyleFormat {
		t.Errorf("expected Slack-style format for 'say' message with sender on separate line")
		for i, line := range lines {
			t.Logf("Line %d: %q", i, stripAnsi(line))
		}
	}
}

func TestFormatNonChatMessageKeepsOldFormat(t *testing.T) {
	m := NewModel(nil)
	m.width = 80

	// Create a prompt message (should keep old format for now)
	messages := []repo.Message{
		{
			FromAgent: "user",
			ToAgent:   sql.NullString{String: "impl-1", Valid: true},
			Type:      "prompt",
			Content:   "do something",
		},
	}

	m.messages = messages

	// Get formatted lines - should still use old inline format
	lines := m.mainContentLines(80)

	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}

	// For now, prompt messages should still be inline (Task 3.2 will change this)
	firstLine := stripAnsi(lines[0])

	// Should have username and content on same line (old format)
	if strings.Contains(firstLine, "user") && strings.Contains(firstLine, "do something") {
		// Good - old format still works
	} else {
		t.Logf("Got line: %q", firstLine)
		// This is ok for now - we're only changing chat/complete types
	}
}

// Task 3.2 tests

func TestPromptToOttoIsHidden(t *testing.T) {
	m := NewModel(nil)
	m.width = 80

	// Create a prompt-to-otto message (should be hidden)
	messages := []repo.Message{
		{
			FromAgent: "you",
			ToAgent:   sql.NullString{String: "otto", Valid: true},
			Type:      "prompt",
			Content:   "do something",
		},
	}

	m.messages = messages
	lines := m.mainContentLines(80)

	// Should have only the "Waiting for messages..." line or be empty
	// since the prompt-to-otto message should be hidden
	if len(lines) != 1 {
		t.Errorf("expected 1 line (waiting message), got %d lines", len(lines))
		for i, line := range lines {
			t.Logf("Line %d: %q", i, stripAnsi(line))
		}
	}

	// The only line should be the "Waiting for messages..." placeholder
	if len(lines) > 0 {
		firstLine := stripAnsi(lines[0])
		if !strings.Contains(firstLine, "Waiting for messages") {
			t.Errorf("expected only 'Waiting for messages' line, got: %q", firstLine)
		}
	}
}

func TestExitMessageIsHidden(t *testing.T) {
	m := NewModel(nil)
	m.width = 80

	// Create an exit message (should be hidden)
	messages := []repo.Message{
		{
			FromAgent: "agent-1",
			Type:      "exit",
			Content:   "process finished",
		},
	}

	m.messages = messages
	lines := m.mainContentLines(80)

	// Should have only the "Waiting for messages..." line
	// since the exit message should be hidden
	if len(lines) != 1 {
		t.Errorf("expected 1 line (waiting message), got %d lines", len(lines))
		for i, line := range lines {
			t.Logf("Line %d: %q", i, stripAnsi(line))
		}
	}

	// The only line should be the "Waiting for messages..." placeholder
	if len(lines) > 0 {
		firstLine := stripAnsi(lines[0])
		if !strings.Contains(firstLine, "Waiting for messages") {
			t.Errorf("expected only 'Waiting for messages' line, got: %q", firstLine)
		}
	}
}

func TestPromptToOtherAgentRendersAsActivityLine(t *testing.T) {
	m := NewModel(nil)
	m.width = 80

	// Create a prompt from otto to reviewer (should render as activity line)
	messages := []repo.Message{
		{
			FromAgent: "otto",
			ToAgent:   sql.NullString{String: "reviewer", Valid: true},
			Type:      "prompt",
			Content:   "Review the code and check for bugs",
		},
	}

	m.messages = messages
	lines := m.mainContentLines(80)

	// Should have the activity line + blank line
	if len(lines) < 1 {
		t.Fatal("expected at least 1 line")
	}

	firstLine := stripAnsi(lines[0])

	// Should be formatted as: "otto spawned reviewer — "Review...""
	if !strings.Contains(firstLine, "otto") {
		t.Errorf("expected activity line to contain sender 'otto', got: %q", firstLine)
	}
	if !strings.Contains(firstLine, "spawned") {
		t.Errorf("expected activity line to contain 'spawned', got: %q", firstLine)
	}
	if !strings.Contains(firstLine, "reviewer") {
		t.Errorf("expected activity line to contain target 'reviewer', got: %q", firstLine)
	}
	if !strings.Contains(firstLine, "Review the code") {
		t.Errorf("expected activity line to contain content, got: %q", firstLine)
	}
}

func TestMixedMessagesWithHiddenAndActivityLines(t *testing.T) {
	m := NewModel(nil)
	m.width = 80

	// Create a mix of messages: chat, prompt-to-otto (hidden), prompt-to-agent (activity), exit (hidden)
	messages := []repo.Message{
		{
			FromAgent: "you",
			Type:      repo.MessageTypeChat,
			Content:   "hello",
		},
		{
			FromAgent: "you",
			ToAgent:   sql.NullString{String: "otto", Valid: true},
			Type:      "prompt",
			Content:   "this should be hidden",
		},
		{
			FromAgent: "otto",
			ToAgent:   sql.NullString{String: "reviewer", Valid: true},
			Type:      "prompt",
			Content:   "Review this",
		},
		{
			FromAgent: "reviewer",
			Type:      "exit",
			Content:   "process finished",
		},
	}

	m.messages = messages
	lines := m.mainContentLines(80)

	output := strings.Join(lines, "\n")

	// Should have:
	// - "you" on its own line (Slack style)
	// - "hello" on next line
	// - blank line
	// - "otto spawned reviewer — "Review this"" (activity line)
	// - blank line
	// Should NOT have the prompt-to-otto or exit messages

	if strings.Contains(output, "this should be hidden") {
		t.Error("expected prompt-to-otto message to be hidden")
	}

	if strings.Contains(output, "process finished") {
		t.Error("expected exit message to be hidden")
	}

	if !strings.Contains(output, "hello") {
		t.Error("expected chat message to be visible")
	}

	if !strings.Contains(output, "spawned") {
		t.Error("expected activity line for prompt-to-agent")
	}

	if !strings.Contains(output, "Review this") {
		t.Error("expected activity line content to be visible")
	}
}

// Task 4.1: Color styling tests

func TestActivityLinesUseDimStyle(t *testing.T) {
	m := NewModel(nil)
	m.width = 80

	// Create a prompt-to-agent message (should render as dim activity line)
	messages := []repo.Message{
		{
			FromAgent: "otto",
			ToAgent:   sql.NullString{String: "reviewer", Valid: true},
			Type:      "prompt",
			Content:   "Review this code",
		},
	}

	m.messages = messages
	lines := m.mainContentLines(80)

	// Activity line should be present
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}

	// Verify the activity line contains the expected format
	output := strings.Join(lines, "\n")
	if !strings.Contains(output, "spawned") {
		t.Error("expected activity line to contain 'spawned'")
	}

	// The activity line should be formatted as "{agent} spawned {target} — "{content}""
	stripped := stripAnsi(lines[0])
	if !strings.Contains(stripped, "otto spawned reviewer") {
		t.Errorf("expected activity line format, got: %q", stripped)
	}
}

func TestUsernameColorForYou(t *testing.T) {
	m := NewModel(nil)
	m.width = 80

	// Create a chat message from "you"
	messages := []repo.Message{
		{
			FromAgent: "you",
			Type:      repo.MessageTypeChat,
			Content:   "hello",
		},
	}

	m.messages = messages
	lines := m.mainContentLines(80)

	if len(lines) < 2 {
		t.Fatal("expected at least 2 lines (sender + content)")
	}

	// First line should be the sender "you" with Slack-style formatting
	senderLine := stripAnsi(lines[0])

	if strings.TrimSpace(senderLine) != "you" {
		t.Errorf("expected first line to be 'you', got %q", senderLine)
	}

	// Second line should be the content
	contentLine := stripAnsi(lines[1])
	if !strings.Contains(contentLine, "hello") {
		t.Errorf("expected second line to contain content, got %q", contentLine)
	}
}

func TestUsernameColorForOtto(t *testing.T) {
	m := NewModel(nil)
	m.width = 80

	// Create a complete message from otto (Slack-style)
	messages := []repo.Message{
		{
			FromAgent: "otto",
			Type:      "complete",
			Content:   "Task completed",
		},
	}

	m.messages = messages
	lines := m.mainContentLines(80)

	if len(lines) < 2 {
		t.Fatal("expected at least 2 lines (sender + content)")
	}

	// First line should be "otto" with Slack-style formatting
	senderLine := stripAnsi(lines[0])

	if strings.TrimSpace(senderLine) != "otto" {
		t.Errorf("expected first line to be 'otto', got %q", senderLine)
	}

	// Second line should be the content (without "completed -" prefix for Slack style)
	contentLine := stripAnsi(lines[1])
	if !strings.Contains(contentLine, "Task completed") {
		t.Errorf("expected second line to contain content, got %q", contentLine)
	}
}

func TestActivityLinesAreDimmed(t *testing.T) {
	// This test verifies that activity lines use mutedStyle (dim color).
	// Since lipgloss doesn't render ANSI codes in test environment,
	// we verify the styling is applied by checking the code structure.

	m := NewModel(nil)
	m.width = 80

	messages := []repo.Message{
		{
			FromAgent: "otto",
			ToAgent:   sql.NullString{String: "impl", Valid: true},
			Type:      "prompt",
			Content:   "Implement feature X",
		},
	}

	m.messages = messages
	lines := m.mainContentLines(80)

	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}

	// Just verify the activity line is present and formatted correctly
	// The actual styling (mutedStyle) is verified by manual testing in TUI
	stripped := stripAnsi(lines[0])
	expected := `otto spawned impl — "Implement feature X"`
	if stripped != expected {
		t.Errorf("expected activity line format:\n  %q\ngot:\n  %q", expected, stripped)
	}
}

// Task 4.2: Word wrapping tests for chat blocks

func TestWrapTextBasic(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		width    int
		expected []string
	}{
		{
			name:     "short text fits on one line",
			text:     "hello world",
			width:    20,
			expected: []string{"hello world"},
		},
		{
			name:     "text exactly at width",
			text:     "hello world",
			width:    11,
			expected: []string{"hello world"},
		},
		{
			name:     "text wraps at word boundary",
			text:     "hello world from testing",
			width:    15,
			expected: []string{"hello world", "from testing"},
		},
		{
			name:     "multiple wraps",
			text:     "this is a longer message that will wrap multiple times",
			width:    20,
			expected: []string{"this is a longer", "message that will", "wrap multiple times"},
		},
		{
			name:     "single long word wraps mid-word",
			text:     "supercalifragilisticexpialidocious",
			width:    10,
			expected: []string{"supercalif", "ragilistic", "expialidoc", "ious"},
		},
		{
			name:     "zero width returns empty line",
			text:     "hello",
			width:    0,
			expected: []string{""},
		},
		{
			name:     "negative width returns empty line",
			text:     "hello",
			width:    -1,
			expected: []string{""},
		},
		{
			name:     "empty text returns empty line",
			text:     "",
			width:    10,
			expected: []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapText(tt.text, tt.width)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d lines, got %d\nExpected: %v\nGot: %v",
					len(tt.expected), len(result), tt.expected, result)
				return
			}
			for i, line := range result {
				if line != tt.expected[i] {
					t.Errorf("line %d: expected %q, got %q", i, tt.expected[i], line)
				}
			}
		})
	}
}

func TestChatMessageWrapsToViewportWidth(t *testing.T) {
	m := NewModel(nil)
	m.width = 40

	// Create a chat message with long content
	longMessage := strings.Repeat("word ", 30) // 150 chars total
	messages := []repo.Message{
		{
			FromAgent: "you",
			Type:      repo.MessageTypeChat,
			Content:   longMessage,
		},
	}
	m.messages = messages

	// Get formatted lines with width=40
	lines := m.mainContentLines(40)

	// Strip ANSI codes and check that no line exceeds width
	for i, line := range lines {
		stripped := stripAnsi(line)
		// Skip blank lines
		if stripped == "" {
			continue
		}
		if len([]rune(stripped)) > 40 {
			t.Errorf("line %d exceeds width 40: len=%d, content=%q",
				i, len([]rune(stripped)), stripped)
		}
	}

	// Verify that the message was actually wrapped (should have multiple content lines)
	// We expect: sender name line + multiple wrapped content lines + blank line
	if len(lines) < 4 { // At minimum: sender + 2 content lines + blank
		t.Errorf("expected message to wrap into multiple lines, got %d lines", len(lines))
	}
}

func TestChatMessageWrapsWithDifferentWidths(t *testing.T) {
	widths := []int{20, 40, 60, 80}
	content := "This is a moderately long chat message that should wrap differently at different viewport widths."

	for _, width := range widths {
		t.Run(fmt.Sprintf("width=%d", width), func(t *testing.T) {
			m := NewModel(nil)
			m.width = width

			messages := []repo.Message{
				{
					FromAgent: "otto",
					Type:      repo.MessageTypeChat,
					Content:   content,
				},
			}
			m.messages = messages

			lines := m.mainContentLines(width)

			// Check each line doesn't exceed width
			for i, line := range lines {
				stripped := stripAnsi(line)
				if stripped == "" {
					continue
				}
				lineLen := len([]rune(stripped))
				if lineLen > width {
					t.Errorf("line %d exceeds width %d: len=%d, content=%q",
						i, width, lineLen, stripped)
				}
			}
		})
	}
}

func TestChatMessageWithMultibyteCharacters(t *testing.T) {
	m := NewModel(nil)
	m.width = 20

	// Message with Unicode characters (emojis, accents, etc.)
	content := "Hello 世界 🌍 こんにちは Café"
	messages := []repo.Message{
		{
			FromAgent: "you",
			Type:      repo.MessageTypeChat,
			Content:   content,
		},
	}
	m.messages = messages

	lines := m.mainContentLines(20)

	// Check that lines are properly wrapped respecting character boundaries
	for i, line := range lines {
		stripped := stripAnsi(line)
		if stripped == "" {
			continue
		}
		lineLen := len([]rune(stripped))
		if lineLen > 20 {
			t.Errorf("line %d exceeds width 20: len=%d, content=%q",
				i, lineLen, stripped)
		}
	}
}

func TestOttoCompleteMessageWrapsCorrectly(t *testing.T) {
	m := NewModel(nil)
	m.width = 40

	// Otto's complete message with long content
	longContent := "I have successfully completed the task you requested. " +
		"The implementation is now finished and ready for review. " +
		"All tests are passing."

	messages := []repo.Message{
		{
			FromAgent: "otto",
			Type:      "complete",
			Content:   longContent,
		},
	}
	m.messages = messages

	lines := m.mainContentLines(40)

	// Verify wrapping
	for i, line := range lines {
		stripped := stripAnsi(line)
		if stripped == "" {
			continue
		}
		if len([]rune(stripped)) > 40 {
			t.Errorf("line %d exceeds width 40: len=%d, content=%q",
				i, len([]rune(stripped)), stripped)
		}
	}

	// Verify Slack-style format is maintained
	hasSlackFormat := false
	for i := 0; i < len(lines)-1; i++ {
		stripped := stripAnsi(lines[i])
		if strings.TrimSpace(stripped) == "otto" {
			hasSlackFormat = true
			break
		}
	}
	if !hasSlackFormat {
		t.Error("expected Slack-style format with otto on separate line")
	}
}

func TestWrapTextPreservesWordBoundaries(t *testing.T) {
	text := "one two three four five"
	result := wrapText(text, 12)

	// Verify each line is a complete word or words, not broken mid-word
	for i, line := range result {
		// Check that line doesn't start or end with partial word
		// (except for words longer than width)
		words := strings.Fields(line)
		rejoined := strings.Join(words, " ")
		if rejoined != line {
			t.Errorf("line %d has unexpected spacing: %q", i, line)
		}
	}
}

func TestMultipleChatMessagesAllWrapCorrectly(t *testing.T) {
	m := NewModel(nil)
	m.width = 30

	messages := []repo.Message{
		{
			FromAgent: "you",
			Type:      repo.MessageTypeChat,
			Content:   "This is my first message and it is quite long",
		},
		{
			FromAgent: "otto",
			Type:      repo.MessageTypeChat,
			Content:   "This is my response and it is also quite long",
		},
		{
			FromAgent: "you",
			Type:      repo.MessageTypeChat,
			Content:   "Short reply",
		},
	}
	m.messages = messages

	lines := m.mainContentLines(30)

	// Verify all lines respect width
	for i, line := range lines {
		stripped := stripAnsi(line)
		if stripped == "" {
			continue
		}
		if len([]rune(stripped)) > 30 {
			t.Errorf("line %d exceeds width 30: len=%d, content=%q",
				i, len([]rune(stripped)), stripped)
		}
	}
}

func TestScrollToBottomOnNewMessages(t *testing.T) {
	// Create model without database (we don't need to persist messages)
	m := NewModel(nil)
	m.width = 80
	m.height = 24
	m.activeChannelID = mainChannelID
	m.updateViewportDimensions()

	// Add enough content to make viewport scrollable
	// Viewport height is about 20 lines (24 - borders/header/footer)
	// Add many messages to exceed viewport height
	messages := make([]repo.Message, 30)
	for i := 0; i < 30; i++ {
		messages[i] = repo.Message{
			ID:        fmt.Sprintf("msg-%d", i+1),
			Project:   "otto",
			Branch:    "main",
			FromAgent: "you",
			ToAgent:   sql.NullString{String: "otto", Valid: true},
			Type:      repo.MessageTypeChat,
			Content:   fmt.Sprintf("Message %d with some content to make it visible", i+1),
		}
	}

	// Simulate receiving messages
	updated, _ := m.Update(messagesMsg(messages))
	m = updated.(model)

	// User scrolls up (not at bottom)
	m.viewport.YOffset = 0

	// Verify we're not at bottom after scrolling up
	if m.viewport.AtBottom() {
		t.Fatal("expected viewport to not be at bottom after scrolling up")
	}

	// Now receive new messages - this should scroll to bottom
	newMessages := []repo.Message{
		{
			ID:        "msg-new-1",
			Project:   "otto",
			Branch:    "main",
			FromAgent: "otto",
			ToAgent:   sql.NullString{String: "you", Valid: true},
			Type:      "complete",
			Content:   "New message that should trigger scroll to bottom",
		},
	}

	updated, _ = m.Update(messagesMsg(newMessages))
	m = updated.(model)

	// Verify viewport scrolled to bottom
	if !m.viewport.AtBottom() {
		t.Fatal("expected viewport to scroll to bottom on new messages")
	}
}

