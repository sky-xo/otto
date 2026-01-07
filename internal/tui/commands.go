// internal/tui/commands.go
package tui

import (
	"time"

	"github.com/sky-xo/june/internal/agent"
	"github.com/sky-xo/june/internal/claude"
	"github.com/sky-xo/june/internal/codex"
	"github.com/sky-xo/june/internal/db"
	"github.com/sky-xo/june/internal/gemini"

	tea "github.com/charmbracelet/bubbletea"
)

// Messages for the TUI
type (
	tickMsg       time.Time
	channelsMsg   []agent.Channel
	transcriptMsg struct {
		agentID string
		entries []claude.Entry
	}
	errMsg error
)

// tickCmd returns a command that ticks every second.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// scanChannelsCmd scans for channels and their agents.
// The codexDB parameter is reused across ticks for performance.
func scanChannelsCmd(claudeProjectsDir, basePath, repoName string, codexDB *db.DB) tea.Cmd {
	return func() tea.Msg {
		channels, err := claude.ScanChannels(claudeProjectsDir, basePath, repoName, codexDB)
		if err != nil {
			return errMsg(err)
		}
		return channelsMsg(channels)
	}
}

// loadTranscriptCmd loads a transcript from a file.
func loadTranscriptCmd(a agent.Agent) tea.Cmd {
	return func() tea.Msg {
		var entries []claude.Entry
		var err error

		switch a.Source {
		case agent.SourceGemini:
			// Parse Gemini format and convert to claude.Entry for display
			var geminiEntries []gemini.TranscriptEntry
			geminiEntries, _, err = gemini.ReadTranscript(a.TranscriptPath, 0)
			if err != nil {
				return errMsg(err)
			}
			entries = convertGeminiEntries(geminiEntries)
		case agent.SourceCodex:
			// Parse Codex format and convert to claude.Entry for display
			var codexEntries []codex.TranscriptEntry
			codexEntries, _, err = codex.ReadTranscript(a.TranscriptPath, 0)
			if err != nil {
				return errMsg(err)
			}
			entries = convertCodexEntries(codexEntries)
		default:
			// Default to Claude format
			entries, err = claude.ParseTranscript(a.TranscriptPath)
			if err != nil {
				return errMsg(err)
			}
		}

		return transcriptMsg{
			agentID: a.ID,
			entries: entries,
		}
	}
}

// convertCodexEntries converts Codex transcript entries to Claude entry format for TUI display.
func convertCodexEntries(codexEntries []codex.TranscriptEntry) []claude.Entry {
	entries := make([]claude.Entry, 0, len(codexEntries))
	for _, ce := range codexEntries {
		var entry claude.Entry
		switch ce.Type {
		case "message":
			// Codex message -> Claude assistant with text content
			entry = claude.Entry{
				Type: "assistant",
				Message: claude.Message{
					Role: "assistant",
					Content: []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": ce.Content,
						},
					},
				},
			}
		case "reasoning":
			// Codex reasoning -> Claude assistant with text (prefixed with [thinking])
			entry = claude.Entry{
				Type: "assistant",
				Message: claude.Message{
					Role: "assistant",
					Content: []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "[thinking] " + ce.Content,
						},
					},
				},
			}
		case "tool":
			// Codex tool call -> Claude assistant with tool_use (displayed as summary)
			entry = claude.Entry{
				Type: "assistant",
				Message: claude.Message{
					Role: "assistant",
					Content: []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": ce.Content, // Already formatted as "[tool: name]"
						},
					},
				},
			}
		case "tool_output":
			// Codex tool output -> Claude user with tool_result style content
			entry = claude.Entry{
				Type: "user",
				Message: claude.Message{
					Role: "user",
					Content: []interface{}{
						map[string]interface{}{
							"type": "tool_result",
							"text": "  -> " + ce.Content,
						},
					},
				},
			}
		default:
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// convertGeminiEntries converts Gemini transcript entries to Claude entry format for TUI display.
func convertGeminiEntries(geminiEntries []gemini.TranscriptEntry) []claude.Entry {
	entries := make([]claude.Entry, 0, len(geminiEntries))
	for _, ge := range geminiEntries {
		var entry claude.Entry
		switch ge.Type {
		case "user":
			// Gemini user message -> Claude user
			entry = claude.Entry{
				Type: "user",
				Message: claude.Message{
					Role: "user",
					Content: []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": ge.Content,
						},
					},
				},
			}
		case "message":
			// Gemini assistant message -> Claude assistant
			entry = claude.Entry{
				Type: "assistant",
				Message: claude.Message{
					Role: "assistant",
					Content: []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": ge.Content,
						},
					},
				},
			}
		case "tool":
			// Gemini tool call -> Claude assistant with tool info
			entry = claude.Entry{
				Type: "assistant",
				Message: claude.Message{
					Role: "assistant",
					Content: []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": ge.Content, // Already formatted as "[tool: name]"
						},
					},
				},
			}
		case "tool_output":
			// Gemini tool output -> Claude user with tool_result
			entry = claude.Entry{
				Type: "user",
				Message: claude.Message{
					Role: "user",
					Content: []interface{}{
						map[string]interface{}{
							"type": "tool_result",
							"text": "  -> " + ge.Content,
						},
					},
				},
			}
		default:
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}
