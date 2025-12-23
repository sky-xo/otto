package commands

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	spawnName    string
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

			return runSpawn(conn, &ottoexec.DefaultRunner{}, agentType, task, spawnFiles, spawnContext, spawnName)
		},
	}
	cmd.Flags().StringVar(&spawnFiles, "files", "", "Relevant files for the agent")
	cmd.Flags().StringVar(&spawnContext, "context", "", "Additional context for the agent")
	cmd.Flags().StringVar(&spawnName, "name", "", "Custom name for the agent (defaults to auto-generated from task)")
	return cmd
}

func runSpawn(db *sql.DB, runner ottoexec.Runner, agentType, task, files, context, name string) error {
	// Generate agent ID: use provided name or auto-generate from task
	var agentID string
	if name != "" {
		agentID = resolveAgentName(db, name)
	} else {
		agentID = generateAgentID(db, task)
	}

	// Generate session ID (for Claude, or as placeholder for Codex until we capture thread_id)
	sessionID := uuid.New().String()

	// Create agent row (status: busy)
	agent := repo.Agent{
		ID:        agentID,
		Type:      agentType,
		Task:      task,
		Status:    "busy",
		SessionID: sql.NullString{String: sessionID, Valid: true},
	}

	if err := repo.CreateAgent(db, agent); err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	// Get current executable path so agents can find otto
	ottoBin, err := os.Executable()
	if err != nil {
		ottoBin = "otto" // fallback to PATH
	}

	// Build spawn prompt
	prompt := buildSpawnPrompt(agentID, task, files, context, ottoBin)

	if err := storePrompt(db, agentID, prompt); err != nil {
		return fmt.Errorf("store prompt: %w", err)
	}

	// Build and run command
	cmdArgs := buildSpawnCommand(agentType, prompt, sessionID)

	// For Codex agents, we need to capture the thread_id from JSON output
	if agentType == "codex" {
		return runCodexSpawn(db, runner, agentID, cmdArgs)
	}

	// For Claude agents, use transcript capture
	pid, output, wait, err := runner.StartWithTranscriptCapture(cmdArgs[0], cmdArgs[1:]...)
	if err != nil {
		return fmt.Errorf("spawn %s: %w", agentType, err)
	}

	// Update agent with PID
	_ = repo.UpdateAgentPid(db, agentID, pid)

	transcriptDone := consumeTranscriptEntries(db, agentID, output, nil)

	// Wait for process
	if err := wait(); err != nil {
		if consumeErr := <-transcriptDone; consumeErr != nil {
			return fmt.Errorf("spawn %s: %w", agentType, consumeErr)
		}
		// Post failure message and mark agent failed
		msg := repo.Message{
			ID:           uuid.New().String(),
			FromID:       agentID,
			Type:         "exit",
			Content:      fmt.Sprintf("process failed: %v", err),
			MentionsJSON: "[]",
			ReadByJSON:   "[]",
		}
		_ = repo.CreateMessage(db, msg)
		_ = repo.SetAgentFailed(db, agentID)
		return fmt.Errorf("spawn %s: %w", agentType, err)
	}

	if consumeErr := <-transcriptDone; consumeErr != nil {
		return fmt.Errorf("spawn %s: %w", agentType, consumeErr)
	}

	msg := repo.Message{
		ID:           uuid.New().String(),
		FromID:       agentID,
		Type:         "exit",
		Content:      "process completed successfully",
		MentionsJSON: "[]",
		ReadByJSON:   "[]",
	}
	_ = repo.CreateMessage(db, msg)
	_ = repo.SetAgentComplete(db, agentID)

	return nil
}

func runCodexSpawn(db *sql.DB, runner ottoexec.Runner, agentID string, cmdArgs []string) error {
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
	_ = repo.UpdateAgentPid(db, agentID, pid)

	// Parse output stream for thread_id in background
	threadIDCaptured := false
	threadIDParser := func(line string) {
		if threadIDCaptured {
			return
		}
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return
		}
		if eventType, ok := event["type"].(string); ok && eventType == "thread.started" {
			if threadID, ok := event["thread_id"].(string); ok && threadID != "" {
				_ = repo.UpdateAgentSessionID(db, agentID, threadID)
				threadIDCaptured = true
			}
		}
	}

	transcriptDone := consumeTranscriptEntries(db, agentID, output, threadIDParser)

	// Wait for process
	if err := wait(); err != nil {
		if consumeErr := <-transcriptDone; consumeErr != nil {
			return fmt.Errorf("spawn codex: %w", consumeErr)
		}
		// Post failure message and mark agent failed
		msg := repo.Message{
			ID:           uuid.New().String(),
			FromID:       agentID,
			Type:         "exit",
			Content:      fmt.Sprintf("process failed: %v", err),
			MentionsJSON: "[]",
			ReadByJSON:   "[]",
		}
		_ = repo.CreateMessage(db, msg)
		_ = repo.SetAgentFailed(db, agentID)
		return fmt.Errorf("spawn codex: %w", err)
	}

	if consumeErr := <-transcriptDone; consumeErr != nil {
		return fmt.Errorf("spawn codex: %w", consumeErr)
	}

	// If we didn't capture thread_id, log a warning (but don't fail)
	if !threadIDCaptured {
		fmt.Fprintf(os.Stderr, "Warning: Could not capture thread_id for Codex agent %s\n", agentID)
	}

	msg := repo.Message{
		ID:           uuid.New().String(),
		FromID:       agentID,
		Type:         "exit",
		Content:      "process completed successfully",
		MentionsJSON: "[]",
		ReadByJSON:   "[]",
	}
	_ = repo.CreateMessage(db, msg)
	_ = repo.SetAgentComplete(db, agentID)

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

func resolveAgentName(db *sql.DB, name string) string {
	// Clean up provided name: lowercase, alphanumeric and hyphens only
	slug := strings.ToLower(name)
	slug = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(slug, "")
	// Collapse multiple hyphens and trim
	slug = regexp.MustCompile(`-+`).ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "agent"
	}

	// Check if name exists, append -2, -3, etc.
	baseName := slug
	counter := 2
	for {
		_, err := repo.GetAgent(db, slug)
		if err == sql.ErrNoRows {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", baseName, counter)
		counter++
	}
}

func buildSpawnPrompt(agentID, task, files, context, ottoBin string) string {
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
` + ottoBin + ` messages --id ` + agentID + `              # unread messages
` + ottoBin + ` messages --mentions ` + agentID + `        # just messages that @mention you

### Post a message
` + ottoBin + ` say --id ` + agentID + ` "message here"

### Ask a question (sets you to WAITING)
` + ottoBin + ` ask --id ` + agentID + ` "your question"

### Mark task as complete
` + ottoBin + ` complete --id ` + agentID + ` "summary of what was done"

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
	// codex flags:
	// --json: capture thread_id from output
	// --skip-git-repo-check: allow non-repo dirs
	// -s danger-full-access: full filesystem access (needed for otto db writes)
	return []string{"codex", "exec", "--json", "--skip-git-repo-check", "-s", "danger-full-access", prompt}
}
