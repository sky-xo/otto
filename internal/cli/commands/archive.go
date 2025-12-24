package commands

import (
	"database/sql"
	"errors"
	"fmt"

	"otto/internal/repo"

	"github.com/spf13/cobra"
)

func NewArchiveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive <agent-id>",
		Short: "Archive a completed/failed agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Reject --id flag for orchestrator commands
			if cmd.Flags().Changed("id") {
				return errors.New("archive is an orchestrator command and does not accept --id flag")
			}

			agentID := args[0]

			conn, err := openDB()
			if err != nil {
				return err
			}
			defer conn.Close()

			return runArchive(conn, agentID)
		},
	}
	return cmd
}

func runArchive(db *sql.DB, agentID string) error {
	agent, err := repo.GetAgent(db, agentID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("agent %q not found", agentID)
	}
	if err != nil {
		return fmt.Errorf("get agent: %w", err)
	}

	if agent.Status != "complete" && agent.Status != "failed" {
		return fmt.Errorf("agent %q is %s and cannot be archived", agentID, agent.Status)
	}

	if agent.ArchivedAt.Valid {
		return nil
	}

	if err := repo.ArchiveAgent(db, agentID); err != nil {
		return fmt.Errorf("archive agent: %w", err)
	}

	return nil
}
