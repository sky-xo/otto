package commands

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"

	"otto/internal/repo"
	"otto/internal/scope"

	"github.com/spf13/cobra"
)

func NewPeekCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "peek <agent-id>",
		Short: "Show unread agent logs",
		Long:  "Show unread log entries for an agent and advance read cursor.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("id") {
				return errors.New("peek is an orchestrator command and does not accept --id flag")
			}

			conn, err := openDB()
			if err != nil {
				return err
			}
			defer conn.Close()

			return runPeek(conn, args[0], os.Stdout)
		},
	}
	return cmd
}

func runPeek(db *sql.DB, agentID string, w io.Writer) error {
	ctx := scope.CurrentContext()

	agent, err := repo.GetAgent(db, ctx.Project, ctx.Branch, agentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("agent %q not found", agentID)
		}
		return err
	}

	// Get logs since the last peek cursor
	sinceID := ""
	if agent.PeekCursor.Valid {
		sinceID = agent.PeekCursor.String
	}
	logs, err := repo.ListLogs(db, ctx.Project, ctx.Branch, agentID, sinceID)
	if err != nil {
		return err
	}

	if len(logs) == 0 {
		fmt.Fprintf(w, "No new log entries for %s\n", agentID)
		return nil
	}

	for _, entry := range logs {
		// Handle item.started events
		if entry.EventType == "item.started" {
			if entry.Command.Valid && entry.Command.String != "" {
				fmt.Fprintf(w, "[running] %s\n", entry.Command.String)
			} else if entry.Content.Valid && entry.Content.String != "" {
				fmt.Fprintf(w, "[starting] %s\n", entry.Content.String)
			}
			// Skip if both empty (edge case - shouldn't happen with Task 1 fix)
			continue
		}

		// Handle turn events
		if entry.EventType == "turn.started" {
			fmt.Fprintf(w, "--- turn started ---\n")
			continue
		}
		if entry.EventType == "turn.completed" {
			fmt.Fprintf(w, "--- turn completed ---\n")
			continue
		}

		// Handle all other events (existing behavior)
		stream := ""
		if entry.ToolName.Valid {
			stream = fmt.Sprintf("[%s] ", entry.ToolName.String)
		}
		if entry.Content.Valid {
			fmt.Fprintf(w, "%s%s\n", stream, entry.Content.String)
		}
	}

	// Update the peek cursor to the last log entry ID
	lastLogID := logs[len(logs)-1].ID
	if err := repo.UpdateAgentPeekCursor(db, ctx.Project, ctx.Branch, agentID, lastLogID); err != nil {
		return err
	}

	return nil
}
