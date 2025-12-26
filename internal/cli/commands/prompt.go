package commands

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	ottoexec "otto/internal/exec"
	"otto/internal/repo"
	"otto/internal/scope"

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
		cmdArgs = []string{"codex", "exec", "--skip-git-repo-check", "-s", "danger-full-access", "resume", sessionID, message}
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

func runCodexPrompt(db *sql.DB, runner ottoexec.Runner, ctx scope.Context, agentID string, cmdArgs []string) error {
	// Create temp directory for CODEX_HOME to bypass superpowers
	tempDir, err := os.MkdirTemp("", "otto-codex-*")
	if err != nil {
		return fmt.Errorf("create temp CODEX_HOME: %w", err)
	}

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

	pid, output, wait, err := runner.StartWithTranscriptCaptureEnv(cmdArgs[0], env, cmdArgs[1:]...)
	if err != nil {
		os.RemoveAll(tempDir)
		return fmt.Errorf("prompt codex: %w", err)
	}

	_ = repo.UpdateAgentPid(db, ctx.Project, ctx.Branch, agentID, pid)

	transcriptDone := consumeTranscriptEntries(db, ctx, agentID, output, nil)

	err = wait()
	os.RemoveAll(tempDir)
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
