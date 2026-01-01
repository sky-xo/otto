package commands

import (
	"database/sql"
	"errors"
	"fmt"
	"os"

	juneexec "june/internal/exec"
	"june/internal/repo"
	"june/internal/scope"

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

			return runPrompt(conn, &juneexec.DefaultRunner{}, agentID, message)
		},
	}
	return cmd
}

func runPrompt(db *sql.DB, runner juneexec.Runner, agentID, message string) error {
	ctx := scope.CurrentContext()

	// Look up agent
	agent, err := repo.GetAgent(db, ctx.Project, ctx.Branch, agentID)
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

	if agent.ArchivedAt.Valid {
		if err := repo.UnarchiveAgent(db, ctx.Project, ctx.Branch, agentID); err != nil {
			return fmt.Errorf("unarchive agent: %w", err)
		}
	}

	sessionID := agent.SessionID.String

	// Build command based on agent type
	var cmdArgs []string
	if agent.Type == "claude" {
		cmdArgs = []string{"claude", "--resume", sessionID, "-p", message}
	} else if agent.Type == "codex" {
		// Attempt Codex resume (support may be limited)
		cmdArgs = []string{"codex", "exec", "--json", "--skip-git-repo-check", "-s", "danger-full-access", "resume", sessionID, message}
	} else {
		return fmt.Errorf("unsupported agent type %q", agent.Type)
	}

	if err := storePrompt(db, ctx, agentID, message, message); err != nil {
		return fmt.Errorf("store prompt: %w", err)
	}

	if err := repo.ResumeAgent(db, ctx.Project, ctx.Branch, agentID); err != nil {
		return fmt.Errorf("resume agent: %w", err)
	}

	if agent.Type == "codex" {
		return runCodexPrompt(db, runner, ctx, agentID, cmdArgs)
	}

	// Run command for Claude agents
	pid, output, wait, err := runner.StartWithTranscriptCapture(cmdArgs[0], cmdArgs[1:]...)
	if err != nil {
		return fmt.Errorf("prompt %s: %w", agent.Type, err)
	}

	_ = repo.UpdateAgentPid(db, ctx.Project, ctx.Branch, agentID, pid)

	transcriptDone := consumeTranscriptEntries(db, ctx, agentID, output, nil)

	if err := wait(); err != nil {
		if consumeErr := <-transcriptDone; consumeErr != nil {
			return fmt.Errorf("prompt %s: %w", agent.Type, consumeErr)
		}
		_ = repo.SetAgentFailed(db, ctx.Project, ctx.Branch, agentID)
		return fmt.Errorf("prompt %s: %w", agent.Type, err)
	}

	if consumeErr := <-transcriptDone; consumeErr != nil {
		return fmt.Errorf("prompt %s: %w", agent.Type, consumeErr)
	}
	_ = repo.SetAgentComplete(db, ctx.Project, ctx.Branch, agentID)

	return nil
}

func runCodexPrompt(db *sql.DB, runner juneexec.Runner, ctx scope.Context, agentID string, cmdArgs []string) error {
	codexHome, err := ensureCodexHome()
	if err != nil {
		return err
	}

	// Set CODEX_HOME to dedicated dir to bypass ~/.codex/AGENTS.md
	env := append(os.Environ(), fmt.Sprintf("CODEX_HOME=%s", codexHome))

	pid, output, wait, err := runner.StartWithTranscriptCaptureEnv(cmdArgs[0], env, cmdArgs[1:]...)
	if err != nil {
		return fmt.Errorf("prompt codex: %w", err)
	}

	_ = repo.UpdateAgentPid(db, ctx.Project, ctx.Branch, agentID, pid)

	// Parse output stream for Codex events (same as spawn)
	onEvent := func(event CodexEvent) {
		if event.Type == "turn.started" || event.Type == "turn.completed" {
			logEntry := repo.LogEntry{
				Project:   ctx.Project,
				Branch:    ctx.Branch,
				AgentName: agentID,
				AgentType: "codex",
				EventType: event.Type,
			}
			_ = repo.CreateLogEntry(db, logEntry)
		}
		if event.Type == "item.started" && event.Item != nil {
			// Use Command as Content fallback to avoid blank transcript lines
			content := event.Item.Text
			if content == "" && event.Item.Command != "" {
				content = event.Item.Command
			}
			logEntry := repo.LogEntry{
				Project:   ctx.Project,
				Branch:    ctx.Branch,
				AgentName: agentID,
				AgentType: "codex",
				EventType: "item.started",
				Command:   sql.NullString{String: event.Item.Command, Valid: event.Item.Command != ""},
				Content:   sql.NullString{String: content, Valid: content != ""},
			}
			_ = repo.CreateLogEntry(db, logEntry)
		}
		if event.Type == "item.completed" && event.Item != nil {
			logEntry := repo.LogEntry{
				Project:   ctx.Project,
				Branch:    ctx.Branch,
				AgentName: agentID,
				AgentType: "codex",
				EventType: NormalizeCodexItemType(event.Item.Type),
				Content:   sql.NullString{String: event.Item.Text, Valid: event.Item.Text != ""},
				RawJSON:   sql.NullString{String: event.Raw, Valid: true},
				Command:   sql.NullString{String: event.Item.Command, Valid: event.Item.Command != ""},
			}
			if event.Item.Type == "command_execution" {
				logEntry.Content = sql.NullString{String: event.Item.AggregatedOutput, Valid: true}
				if event.Item.ExitCode != nil {
					logEntry.ExitCode = sql.NullInt64{Int64: int64(*event.Item.ExitCode), Valid: true}
				}
			}
			if event.Item.Status != "" {
				logEntry.Status = sql.NullString{String: event.Item.Status, Valid: true}
			}
			_ = repo.CreateLogEntry(db, logEntry)
		}
	}

	transcriptDone := consumeTranscriptEntries(db, ctx, agentID, output, onEvent)

	err = wait()
	if err != nil {
		if consumeErr := <-transcriptDone; consumeErr != nil {
			return fmt.Errorf("prompt codex: %w", consumeErr)
		}
		_ = repo.SetAgentFailed(db, ctx.Project, ctx.Branch, agentID)
		return fmt.Errorf("prompt codex: %w", err)
	}

	if consumeErr := <-transcriptDone; consumeErr != nil {
		return fmt.Errorf("prompt codex: %w", consumeErr)
	}
	_ = repo.SetAgentComplete(db, ctx.Project, ctx.Branch, agentID)

	return nil
}
