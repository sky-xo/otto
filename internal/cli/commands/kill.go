package commands

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"syscall"

	"otto/internal/repo"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func NewKillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kill <agent-id>",
		Short: "Kill a running agent process",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Reject --id flag for orchestrator commands
			if cmd.Flags().Changed("id") {
				return errors.New("kill is an orchestrator command and does not accept --id flag")
			}

			agentID := args[0]

			conn, err := openDB()
			if err != nil {
				return err
			}
			defer conn.Close()

			return runKill(conn, agentID)
		},
	}
	return cmd
}

func runKill(db *sql.DB, agentID string) error {
	// Look up agent
	agent, err := repo.GetAgent(db, agentID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("agent %q not found", agentID)
	}
	if err != nil {
		return fmt.Errorf("get agent: %w", err)
	}

	// Check if agent has a PID
	if !agent.Pid.Valid {
		return fmt.Errorf("agent %q has no PID", agentID)
	}

	pid := int(agent.Pid.Int64)

	// Find and kill the process
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}

	// Send SIGTERM
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("kill process %d: %w", pid, err)
	}

	// Post kill message
	msg := repo.Message{
		ID:           uuid.New().String(),
		FromID:       agentID,
		Type:         "exit",
		Content:      "KILLED: by orchestrator",
		MentionsJSON: "[]",
		ReadByJSON:   "[]",
	}
	if err := repo.CreateMessage(db, msg); err != nil {
		return fmt.Errorf("create message: %w", err)
	}

	// Delete agent row
	if err := repo.DeleteAgent(db, agentID); err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}

	return nil
}
