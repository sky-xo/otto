package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sky-xo/june/internal/codex"
	"github.com/sky-xo/june/internal/db"
	"github.com/sky-xo/june/internal/gemini"
	"github.com/spf13/cobra"
)

func newPeekCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "peek <name>",
		Short: "Show new output from an agent",
		Long:  "Show output since last peek and advance the cursor",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return runPeek(name)
		},
	}
}

func runPeek(name string) error {
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
		if err := database.UpdateSessionFile(name, sessionFile); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to update session file in database: %v\n", err)
		}
	}

	// Read transcript based on agent type
	var output string
	var newCursor int

	if agent.Type == "gemini" {
		entries, cursor, err := gemini.ReadTranscript(sessionFile, agent.Cursor)
		if err != nil {
			return fmt.Errorf("failed to read transcript: %w", err)
		}
		newCursor = cursor
		output = gemini.FormatEntries(entries)
	} else {
		entries, cursor, err := codex.ReadTranscript(sessionFile, agent.Cursor)
		if err != nil {
			return fmt.Errorf("failed to read transcript: %w", err)
		}
		newCursor = cursor
		output = codex.FormatEntries(entries)
	}

	if output == "" {
		fmt.Println("(no new output)")
		return nil
	}

	// Update cursor
	if err := database.UpdateCursor(name, newCursor); err != nil {
		return fmt.Errorf("failed to update cursor: %w", err)
	}

	fmt.Print(output)
	return nil
}
