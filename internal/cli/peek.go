package cli

import (
	"fmt"
	"path/filepath"

	"github.com/sky-xo/june/internal/codex"
	"github.com/sky-xo/june/internal/db"
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
		codexHome, err := codex.CodexHome()
		if err != nil {
			return fmt.Errorf("failed to get codex home: %w", err)
		}
		found, err := codex.FindSessionFile(codexHome, agent.ULID)
		if err != nil {
			return fmt.Errorf("session file not found for agent %q", name)
		}
		sessionFile = found
		// Update in database
		database.Exec("UPDATE agents SET session_file = ? WHERE name = ?", found, name)
	}

	// Read from cursor
	entries, newCursor, err := codex.ReadTranscript(sessionFile, agent.Cursor)
	if err != nil {
		return fmt.Errorf("failed to read transcript: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("(no new output)")
		return nil
	}

	// Update cursor
	if err := database.UpdateCursor(name, newCursor); err != nil {
		return fmt.Errorf("failed to update cursor: %w", err)
	}

	// Print entries
	fmt.Print(codex.FormatEntries(entries))
	return nil
}
