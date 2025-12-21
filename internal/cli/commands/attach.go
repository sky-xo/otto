package commands

import (
	"database/sql"
	"errors"
	"fmt"

	"otto/internal/repo"

	"github.com/spf13/cobra"
)

func NewAttachCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attach <agent-id>",
		Short: "Show command to attach to an agent's session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Reject --id flag for orchestrator commands
			if cmd.Flags().Changed("id") {
				return errors.New("attach is an orchestrator command and does not accept --id flag")
			}

			agentID := args[0]

			conn, err := openDB()
			if err != nil {
				return err
			}
			defer conn.Close()

			return runAttach(conn, agentID)
		},
	}
	return cmd
}

func runAttach(db *sql.DB, agentID string) error {
	// Look up agent
	agent, err := repo.GetAgent(db, agentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("agent %q not found", agentID)
		}
		return fmt.Errorf("get agent: %w", err)
	}

	// Check session ID
	if !agent.SessionID.Valid {
		return fmt.Errorf("agent %q has no session ID", agentID)
	}

	sessionID := agent.SessionID.String

	// Print resume command (don't execute it)
	if agent.Type == "claude" {
		fmt.Printf("claude --resume %s\n", sessionID)
	} else if agent.Type == "codex" {
		// Note if not supported
		fmt.Printf("codex resume %s\n", sessionID)
		fmt.Println("(Note: Codex resume support may be limited)")
	} else {
		return fmt.Errorf("unsupported agent type %q", agent.Type)
	}

	return nil
}
