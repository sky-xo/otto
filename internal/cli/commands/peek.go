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

const peekCapLines = 100

func NewPeekCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "peek <agent-id>",
		Short: "Show unread agent logs",
		Long:  "Show unread log entries for an agent and advance read cursor. For completed/failed agents, shows full log (capped at 100 lines).",
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

	// For completed/failed agents, show full log (capped)
	if agent.Status == "complete" || agent.Status == "failed" {
		return runPeekFullLog(db, ctx, agent, w)
	}

	// For active agents, show incremental logs
	return runPeekIncremental(db, ctx, agent, w)
}

func runPeekFullLog(db *sql.DB, ctx scope.Context, agent repo.Agent, w io.Writer) error {
	// Count total logs
	totalCount, err := repo.CountLogs(db, ctx.Project, ctx.Branch, agent.Name)
	if err != nil {
		return err
	}

	if totalCount == 0 {
		fmt.Fprintf(w, "No log entries for %s\n", agent.Name)
		return nil
	}

	// Get logs (capped)
	var logs []repo.LogEntry
	if totalCount > peekCapLines {
		logs, err = repo.ListLogsWithTail(db, ctx.Project, ctx.Branch, agent.Name, peekCapLines)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "[agent %s - showing last %d lines]\n\n", agent.Status, peekCapLines)
	} else {
		logs, err = repo.ListLogs(db, ctx.Project, ctx.Branch, agent.Name, "")
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "[agent %s - showing all %d lines]\n\n", agent.Status, totalCount)
	}

	// Render logs
	for _, entry := range logs {
		renderLogEntry(w, entry)
	}

	// Show footer if capped
	if totalCount > peekCapLines {
		fmt.Fprintf(w, "\n[full log: %d lines - run 'otto log %s' for complete history]\n", totalCount, agent.Name)
	}

	return nil
}

func runPeekIncremental(db *sql.DB, ctx scope.Context, agent repo.Agent, w io.Writer) error {
	// Get logs since the last peek cursor
	sinceID := ""
	if agent.PeekCursor.Valid {
		sinceID = agent.PeekCursor.String
	}
	logs, err := repo.ListLogs(db, ctx.Project, ctx.Branch, agent.Name, sinceID)
	if err != nil {
		return err
	}

	if len(logs) == 0 {
		fmt.Fprintf(w, "No new log entries for %s\n", agent.Name)
		return nil
	}

	for _, entry := range logs {
		renderLogEntry(w, entry)
	}

	// Update the peek cursor to the last log entry ID
	lastLogID := logs[len(logs)-1].ID
	if err := repo.UpdateAgentPeekCursor(db, ctx.Project, ctx.Branch, agent.Name, lastLogID); err != nil {
		return err
	}

	return nil
}

func renderLogEntry(w io.Writer, entry repo.LogEntry) {
	// Handle item.started events
	if entry.EventType == "item.started" {
		if entry.Command.Valid && entry.Command.String != "" {
			fmt.Fprintf(w, "[running] %s\n", entry.Command.String)
		} else if entry.Content.Valid && entry.Content.String != "" {
			fmt.Fprintf(w, "[starting] %s\n", entry.Content.String)
		}
		return
	}

	// Handle turn events
	if entry.EventType == "turn.started" {
		fmt.Fprintf(w, "--- turn started ---\n")
		return
	}
	if entry.EventType == "turn.completed" {
		fmt.Fprintf(w, "--- turn completed ---\n")
		return
	}

	// Handle all other events
	stream := ""
	if entry.ToolName.Valid {
		stream = fmt.Sprintf("[%s] ", entry.ToolName.String)
	}
	if entry.Content.Valid {
		fmt.Fprintf(w, "%s%s\n", stream, entry.Content.String)
	}
}
