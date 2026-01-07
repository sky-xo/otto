package cli

import (
	"fmt"
	"path/filepath"

	"github.com/sky-xo/june/internal/codex"
	"github.com/sky-xo/june/internal/db"
	"github.com/sky-xo/june/internal/gemini"
	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs <name>",
		Short: "Show full transcript from an agent",
		Long:  "Show full transcript without advancing the cursor",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return runLogs(name)
		},
	}
}

func runLogs(name string) error {
	// Open database
	home, err := juneHome()
	if err != nil {
		return fmt.Errorf("failed to get june home: %w", err)
	}
	dbPath := filepath.Join(home, "june.db")
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	// Get agent
	agent, err := database.GetAgent(name)
	if err == db.ErrAgentNotFound {
		return fmt.Errorf("agent %q not found", name)
	}
	if err != nil {
		return err
	}

	// Find session file if not set
	sessionFile := agent.SessionFile
	if sessionFile == "" {
		var findErr error
		if agent.Type == "gemini" {
			sessionFile, findErr = gemini.FindSessionFile(agent.ULID)
		} else {
			sessionFile, findErr = codex.FindSessionFile(agent.ULID)
		}
		if findErr != nil {
			return fmt.Errorf("session file not found for agent %q", name)
		}
	}

	// Read transcript based on agent type
	var output string

	if agent.Type == "gemini" {
		entries, _, err := gemini.ReadTranscript(sessionFile, 0)
		if err != nil {
			return fmt.Errorf("failed to read transcript: %w", err)
		}
		output = gemini.FormatEntries(entries)
	} else {
		entries, _, err := codex.ReadTranscript(sessionFile, 0)
		if err != nil {
			return fmt.Errorf("failed to read transcript: %w", err)
		}
		output = codex.FormatEntries(entries)
	}

	if output == "" {
		fmt.Println("(no output)")
		return nil
	}

	fmt.Print(output)
	return nil
}
