package commands

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	ottoexec "otto/internal/exec"
	"otto/internal/repo"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var (
	spawnFiles   string
	spawnContext string
)

func NewSpawnCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spawn <type> <task>",
		Short: "Spawn a new AI agent",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Reject --id flag for orchestrator commands
			if cmd.Flags().Changed("id") {
				return errors.New("spawn is an orchestrator command and does not accept --id flag")
			}

			agentType := args[0]
			task := args[1]

			if agentType != "claude" && agentType != "codex" {
				return fmt.Errorf("unsupported agent type %q (must be 'claude' or 'codex')", agentType)
			}

			conn, err := openDB()
			if err != nil {
				return err
			}
			defer conn.Close()

			return runSpawn(conn, &ottoexec.DefaultRunner{}, agentType, task, spawnFiles, spawnContext)
		},
	}
	cmd.Flags().StringVar(&spawnFiles, "files", "", "Relevant files for the agent")
	cmd.Flags().StringVar(&spawnContext, "context", "", "Additional context for the agent")
	return cmd
}

func runSpawn(db *sql.DB, runner ottoexec.Runner, agentType, task, files, context string) error {
	// Generate agent ID from task slug
	agentID := generateAgentID(db, task)

	// Generate session ID
	sessionID := uuid.New().String()

	// Create agent row (status: working)
	agent := repo.Agent{
		ID:        agentID,
		Type:      agentType,
		Task:      task,
		Status:    "working",
		SessionID: sql.NullString{String: sessionID, Valid: true},
	}

	if err := repo.CreateAgent(db, agent); err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	// Build spawn prompt
	prompt := buildSpawnPrompt(agentID, task, files, context)

	// Build and run command
	cmdArgs := buildSpawnCommand(agentType, prompt, sessionID)

	// Run the command (keep agent record even if it fails)
	if err := runner.Run(cmdArgs[0], cmdArgs[1:]...); err != nil {
		return fmt.Errorf("spawn %s: %w", agentType, err)
	}

	return nil
}

func generateAgentID(db *sql.DB, task string) string {
	// Generate slug: lowercase alphanumeric only, max 16 chars
	slug := strings.ToLower(task)
	slug = regexp.MustCompile(`[^a-z0-9]`).ReplaceAllString(slug, "")
	if len(slug) > 16 {
		slug = slug[:16]
	}
	if slug == "" {
		slug = "agent"
	}

	// Check if slug exists, append -2, -3, etc.
	baseSlug := slug
	counter := 2
	for {
		_, err := repo.GetAgent(db, slug)
		if err == sql.ErrNoRows {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", baseSlug, counter)
		counter++
	}
}

func buildSpawnPrompt(agentID, task, files, context string) string {
	prompt := fmt.Sprintf(`You are an agent working on: %s

Your agent ID: %s`, task, agentID)

	if files != "" {
		prompt += fmt.Sprintf("\nRelevant files: %s", files)
	}
	if context != "" {
		prompt += fmt.Sprintf("\nAdditional context: %s", context)
	}

	prompt += `

## Communication

You're part of a team. All agents share a message stream where everyone can
see everything. Use @mentions to direct attention to specific agents.

IMPORTANT: Always include your ID (--id ` + agentID + `) in every command.

### Check for messages
otto messages --id ` + agentID + `              # unread messages
otto messages --mentions ` + agentID + `        # just messages that @mention you

### Post a message
otto say --id ` + agentID + ` "message here"

### Ask a question (sets you to WAITING)
otto ask --id ` + agentID + ` "your question"

### Mark task as complete
otto complete --id ` + agentID + ` "summary of what was done"

## Guidelines

**Check messages regularly** - other agents may have questions or updates.
**Use @mentions** - when you need a specific agent's attention.
`

	return prompt
}

func buildSpawnCommand(agentType, prompt, sessionID string) []string {
	if agentType == "claude" {
		return []string{"claude", "-p", prompt, "--session-id", sessionID}
	}
	// codex
	return []string{"codex", "exec", prompt}
}
