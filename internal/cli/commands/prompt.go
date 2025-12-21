package commands

import (
	"database/sql"
	"errors"
	"fmt"

	ottoexec "otto/internal/exec"
	"otto/internal/repo"

	"github.com/spf13/cobra"
)

func NewPromptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prompt <agent-id> <message>",
		Short: "Send a prompt to an agent",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Reject --id flag for orchestrator commands
			if cmd.Flags().Changed("id") {
				return errors.New("prompt is an orchestrator command and does not accept --id flag")
			}

			agentID := args[0]
			message := args[1]

			conn, err := openDB()
			if err != nil {
				return err
			}
			defer conn.Close()

			return runPrompt(conn, &ottoexec.DefaultRunner{}, agentID, message)
		},
	}
	return cmd
}

func runPrompt(db *sql.DB, runner ottoexec.Runner, agentID, message string) error {
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

	// Build command based on agent type
	var cmdArgs []string
	if agent.Type == "claude" {
		cmdArgs = []string{"claude", "--resume", sessionID, "-p", message}
	} else if agent.Type == "codex" {
		// Codex doesn't fully support resume, just run new
		cmdArgs = []string{"codex", "exec", message}
	} else {
		return fmt.Errorf("unsupported agent type %q", agent.Type)
	}

	// Run command
	if err := runner.Run(cmdArgs[0], cmdArgs[1:]...); err != nil {
		return fmt.Errorf("prompt %s: %w", agent.Type, err)
	}

	return nil
}
