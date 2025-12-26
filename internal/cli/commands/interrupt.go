package commands

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"syscall"

	"otto/internal/repo"
	"otto/internal/scope"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func NewInterruptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "interrupt <agent-id>",
		Short: "Interrupt an agent (can be resumed later)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Reject --id flag for orchestrator commands
			if cmd.Flags().Changed("id") {
				return errors.New("interrupt is an orchestrator command and does not accept --id flag")
			}

			agentID := args[0]

			conn, err := openDB()
			if err != nil {
				return err
			}
			defer conn.Close()

			return runInterrupt(conn, agentID)
		},
	}
	return cmd
}

func runInterrupt(db *sql.DB, agentID string) error {
	ctx := scope.CurrentContext()

	// Get agent
	agent, err := repo.GetAgent(db, ctx.Project, ctx.Branch, agentID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("agent %q not found", agentID)
	}
	if err != nil {
		return fmt.Errorf("get agent: %w", err)
	}

	// Check PID exists
	if !agent.Pid.Valid {
		return fmt.Errorf("agent %q has no PID", agentID)
	}

	pid := int(agent.Pid.Int64)

	// Send SIGINT
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}
	if err := proc.Signal(syscall.SIGINT); err != nil {
		return fmt.Errorf("send SIGINT to process %d: %w", pid, err)
	}

	// Update status to idle
	if err := repo.UpdateAgentStatus(db, ctx.Project, ctx.Branch, agentID, "idle"); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	// Post message
	msg := repo.Message{
		ID:        uuid.New().String(),
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		FromAgent: agentID,
		Type:      "system",
		Content:   "INTERRUPTED",
		MentionsJSON: "[]",
		ReadByJSON:   "[]",
	}
	if err := repo.CreateMessage(db, msg); err != nil {
		return fmt.Errorf("create message: %w", err)
	}

	fmt.Printf("Interrupted agent %s\n", agentID)
	return nil
}
