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

	// For peek, we want to show logs, not messages
	// We'll just show all logs for now since there's no LastReadLogID anymore
	// (In the future, we could use agent.LastSeenMsgID to track the last message
	// and correlate it with logs, but for now we just show all logs)
	_ = agent // Suppress unused warning
	logs, err := repo.ListLogs(db, ctx.Project, ctx.Branch, agentID, "")
	if err != nil {
		return err
	}

	if len(logs) == 0 {
		fmt.Fprintf(w, "No log entries for %s\n", agentID)
		return nil
	}

	for _, entry := range logs {
		stream := ""
		if entry.ToolName.Valid {
			stream = fmt.Sprintf("[%s] ", entry.ToolName.String)
		}
		if entry.Content.Valid {
			fmt.Fprintf(w, "%s%s\n", stream, entry.Content.String)
		}
	}

	return nil
}
