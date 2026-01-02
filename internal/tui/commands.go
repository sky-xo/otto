// internal/tui/commands.go
package tui

import (
	"time"

	"june/internal/claude"

	tea "github.com/charmbracelet/bubbletea"
)

// Messages for the TUI
type (
	tickMsg       time.Time
	channelsMsg   []claude.Channel // Changed from agentsMsg
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
func scanChannelsCmd(claudeProjectsDir, basePath, repoName string) tea.Cmd {
	return func() tea.Msg {
		channels, err := claude.ScanChannels(claudeProjectsDir, basePath, repoName)
		if err != nil {
			return errMsg(err)
		}
		return channelsMsg(channels)
	}
}

// loadTranscriptCmd loads a transcript from a file.
func loadTranscriptCmd(agent claude.Agent) tea.Cmd {
	return func() tea.Msg {
		entries, err := claude.ParseTranscript(agent.FilePath)
		if err != nil {
			return errMsg(err)
		}
		return transcriptMsg{
			agentID: agent.ID,
			entries: entries,
		}
	}
}
