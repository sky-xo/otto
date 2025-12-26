package commands

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	ottoexec "otto/internal/exec"
	"otto/internal/repo"
	"otto/internal/scope"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func NewWorkerSpawnCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "worker-spawn <agent-id>",
		Short:  "Internal worker command for detached spawn",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentID := args[0]

			conn, err := openDB()
			if err != nil {
				return err
			}
			defer conn.Close()

			return runWorkerSpawn(conn, &ottoexec.DefaultRunner{}, agentID)
		},
	}
	return cmd
}

func runWorkerSpawn(db *sql.DB, runner ottoexec.Runner, agentID string) error {
	ctx := scope.CurrentContext()

	// Load agent
	agent, err := repo.GetAgent(db, ctx.Project, ctx.Branch, agentID)
	if err != nil {
		return fmt.Errorf("load agent: %w", err)
	}

	// Get latest prompt message
	promptMsg, err := repo.GetLatestPromptForAgent(db, ctx.Project, ctx.Branch, agentID)
	if err != nil {
		return fmt.Errorf("load prompt: %w", err)
	}
	promptContent := promptMsg.Content

	// Store prompt as input log entry
	entry := repo.LogEntry{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		AgentName: agentID,
		AgentType: agent.Type,
		EventType: "in",
		Content:   sql.NullString{String: promptContent, Valid: true},
	}
	if err := repo.CreateLogEntry(db, entry); err != nil {
		return fmt.Errorf("store prompt log: %w", err)
	}

	// Build command based on agent type
	cmdArgs := buildSpawnCommand(agent.Type, promptContent, agent.SessionID.String)

	// Run with transcript capture (different logic for codex vs claude)
	if agent.Type == "codex" {
		return runWorkerCodexSpawn(db, runner, ctx, agentID, cmdArgs)
	}

	// For Claude agents, use standard transcript capture
	pid, output, wait, err := runner.StartWithTranscriptCapture(cmdArgs[0], cmdArgs[1:]...)
	if err != nil {
		return fmt.Errorf("spawn %s: %w", agent.Type, err)
	}

	// Update agent with PID
	_ = repo.UpdateAgentPid(db, ctx.Project, ctx.Branch, agentID, pid)

	// Consume transcript entries
	transcriptDone := consumeTranscriptEntries(db, ctx, agentID, output, nil)

	// Wait for process
	if err := wait(); err != nil {
		if consumeErr := <-transcriptDone; consumeErr != nil {
			return fmt.Errorf("spawn %s: %w", agent.Type, consumeErr)
		}
		// Record launch error
		repoRoot := scope.RepoRoot()
		branch := scope.BranchName()
		if branch == "" {
			branch = "main"
		}
		scopePath := scope.Scope(repoRoot, branch)
		errorText := fmt.Sprintf("process failed: %v", err)
		_ = repo.RecordLaunchError(scopePath, agentID, errorText)

		// Post failure message and mark agent failed
		msg := repo.Message{
			ID:        uuid.New().String(),
			Project:   ctx.Project,
			Branch:    ctx.Branch,
			FromAgent: agentID,
			Type:      "exit",
			Content:   errorText,
			MentionsJSON: "[]",
			ReadByJSON:   "[]",
		}
		_ = repo.CreateMessage(db, msg)
		_ = repo.SetAgentFailed(db, ctx.Project, ctx.Branch, agentID)
		return fmt.Errorf("spawn %s: %w", agent.Type, err)
	}

	if consumeErr := <-transcriptDone; consumeErr != nil {
		return fmt.Errorf("spawn %s: %w", agent.Type, consumeErr)
	}

	// Success - post completion message
	msg := repo.Message{
		ID:        uuid.New().String(),
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		FromAgent: agentID,
		Type:      "exit",
		Content:   "process completed successfully",
		MentionsJSON: "[]",
		ReadByJSON:   "[]",
	}
	_ = repo.CreateMessage(db, msg)
	_ = repo.SetAgentComplete(db, ctx.Project, ctx.Branch, agentID)

	return nil
}

func runWorkerCodexSpawn(db *sql.DB, runner ottoexec.Runner, ctx scope.Context, agentID string, cmdArgs []string) error {
	// Create temp directory for CODEX_HOME to bypass superpowers
	tempDir, err := os.MkdirTemp("", "otto-codex-*")
	if err != nil {
		return fmt.Errorf("create temp CODEX_HOME: %w", err)
	}
	defer os.RemoveAll(tempDir) // Cleanup after agent process exits

	// Copy auth.json from real CODEX_HOME to preserve credentials
	realCodexHome := os.Getenv("CODEX_HOME")
	if realCodexHome == "" {
		home, _ := os.UserHomeDir()
		realCodexHome = filepath.Join(home, ".codex")
	}
	authSrc := filepath.Join(realCodexHome, "auth.json")
	if authData, err := os.ReadFile(authSrc); err == nil {
		_ = os.WriteFile(filepath.Join(tempDir, "auth.json"), authData, 0600)
	}

	// Set CODEX_HOME to temp dir to bypass ~/.codex/AGENTS.md
	env := append(os.Environ(), fmt.Sprintf("CODEX_HOME=%s", tempDir))

	// Start with transcript capture to parse JSON output
	pid, output, wait, err := runner.StartWithTranscriptCaptureEnv(cmdArgs[0], env, cmdArgs[1:]...)
	if err != nil {
		os.RemoveAll(tempDir) // Cleanup temp dir on spawn failure
		return fmt.Errorf("spawn codex: %w", err)
	}

	// Update agent with PID
	_ = repo.UpdateAgentPid(db, ctx.Project, ctx.Branch, agentID, pid)

	// Parse output stream for Codex events
	onEvent := func(event CodexEvent) {
		if event.Type == "thread.started" && event.ThreadID != "" {
			_ = repo.UpdateAgentSessionID(db, ctx.Project, ctx.Branch, agentID, event.ThreadID)
		}
		if event.Type == "context_compacted" {
			_ = repo.MarkAgentCompacted(db, ctx.Project, ctx.Branch, agentID)
		}
		if event.Type == "turn.failed" {
			_ = repo.SetAgentFailed(db, ctx.Project, ctx.Branch, agentID)
		}
		if event.Type == "item.completed" && event.Item != nil {
			logEntry := repo.LogEntry{
				Project:   ctx.Project,
				Branch:    ctx.Branch,
				AgentName: agentID,
				AgentType: "codex",
				EventType: event.Item.Type,
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

	// Wait for process
	if err := wait(); err != nil {
		if consumeErr := <-transcriptDone; consumeErr != nil {
			return fmt.Errorf("spawn codex: %w", consumeErr)
		}
		// Record launch error
		repoRoot := scope.RepoRoot()
		branch := scope.BranchName()
		if branch == "" {
			branch = "main"
		}
		scopePath := scope.Scope(repoRoot, branch)
		errorText := fmt.Sprintf("process failed: %v", err)
		_ = repo.RecordLaunchError(scopePath, agentID, errorText)

		// Post failure message and mark agent failed
		msg := repo.Message{
			ID:        uuid.New().String(),
			Project:   ctx.Project,
			Branch:    ctx.Branch,
			FromAgent: agentID,
			Type:      "exit",
			Content:   errorText,
			MentionsJSON: "[]",
			ReadByJSON:   "[]",
		}
		_ = repo.CreateMessage(db, msg)
		_ = repo.SetAgentFailed(db, ctx.Project, ctx.Branch, agentID)
		return fmt.Errorf("spawn codex: %w", err)
	}

	if consumeErr := <-transcriptDone; consumeErr != nil {
		return fmt.Errorf("spawn codex: %w", consumeErr)
	}

	msg := repo.Message{
		ID:        uuid.New().String(),
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		FromAgent: agentID,
		Type:      "exit",
		Content:   "process completed successfully",
		MentionsJSON: "[]",
		ReadByJSON:   "[]",
	}
	_ = repo.CreateMessage(db, msg)
	_ = repo.SetAgentComplete(db, ctx.Project, ctx.Branch, agentID)

	return nil
}
