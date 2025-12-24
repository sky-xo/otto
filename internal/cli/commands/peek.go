package commands

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"

	"otto/internal/repo"

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
	agent, err := repo.GetAgent(db, agentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("agent %q not found", agentID)
		}
		return err
	}

	sinceID := ""
	if agent.LastReadLogID.Valid {
		sinceID = agent.LastReadLogID.String
	}

	logs, err := repo.ListLogs(db, agentID, sinceID)
	if err != nil {
		return err
	}

	if len(logs) == 0 {
		fmt.Fprintf(w, "No new log entries for %s\n", agentID)
		return nil
	}

	for _, entry := range logs {
		stream := ""
		if entry.Stream.Valid {
			stream = fmt.Sprintf("[%s] ", entry.Stream.String)
		}
		fmt.Fprintf(w, "%s%s\n", stream, entry.Content)
	}

	lastID := logs[len(logs)-1].ID
	return repo.UpdateAgentLastReadLogID(db, agentID, lastID)
}
