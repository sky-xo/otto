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
	agentsMsg     []claude.Agent
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

// scanAgentsCmd scans for agent files.
func scanAgentsCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		agents, err := claude.ScanAgents(dir)
		if err != nil {
			return errMsg(err)
		}
		return agentsMsg(agents)
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
