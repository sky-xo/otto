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

var logTail int

func NewLogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log <agent-id>",
		Short: "Show agent log history",
		Long:  "Show full log history for an agent. Does not advance read cursor.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("id") {
				return errors.New("log is an orchestrator command and does not accept --id flag")
			}

			conn, err := openDB()
			if err != nil {
				return err
			}
			defer conn.Close()

			return runLog(conn, args[0], logTail, os.Stdout)
		},
	}
	cmd.Flags().IntVar(&logTail, "tail", 0, "Show only last N entries")
	return cmd
}

func runLog(db *sql.DB, agentID string, tail int, w io.Writer) error {
	ctx := scope.CurrentContext()

	if _, err := repo.GetAgent(db, ctx.Project, ctx.Branch, agentID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("agent %q not found", agentID)
		}
		return err
	}

	var logs []repo.LogEntry
	var err error

	if tail > 0 {
		logs, err = repo.ListLogsWithTail(db, ctx.Project, ctx.Branch, agentID, tail)
	} else {
		logs, err = repo.ListLogs(db, ctx.Project, ctx.Branch, agentID, "")
	}
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
