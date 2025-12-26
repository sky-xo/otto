package tui

import "otto/internal/repo"

// Reasoning labels are added at render time based on EventType; DB stores only event_type + content.
// Render reasoning lines with a dim style in the TUI.
func FormatLogEntry(entry repo.LogEntry) string {
	content := ""
	if entry.Content.Valid {
		content = entry.Content.String
	}
	command := ""
	if entry.Command.Valid {
		command = entry.Command.String
	}
	switch entry.EventType {
	case "reasoning":
		return "[reasoning] " + content
	case "command_execution":
		if command == "" {
			return content
		}
		if content == "" {
			return command
		}
		return command + "\n" + content
	case "agent_message":
		return content
	default:
		return content
	}
}
