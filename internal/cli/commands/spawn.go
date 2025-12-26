package commands

import (
	"bytes"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	ottoexec "otto/internal/exec"
	"otto/internal/repo"
	"otto/internal/scope"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

//go:embed prompts/agent_instructions.md
var agentInstructionsTemplate string

var (
	spawnFiles   string
	spawnContext string
	spawnName    string
	spawnDetach  bool
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

			return runSpawnWithOptions(conn, &ottoexec.DefaultRunner{}, agentType, task, spawnFiles, spawnContext, spawnName, spawnDetach, os.Stdout)
		},
	}
	cmd.Flags().StringVar(&spawnFiles, "files", "", "Relevant files for the agent")
	cmd.Flags().StringVar(&spawnContext, "context", "", "Additional context for the agent")
	cmd.Flags().StringVar(&spawnName, "name", "", "Custom name for the agent (defaults to auto-generated from task)")
	cmd.Flags().BoolVar(&spawnDetach, "detach", false, "Return immediately (no output capture)")
	return cmd
}

func runSpawn(db *sql.DB, runner ottoexec.Runner, agentType, task, files, context, name string) error {
	return runSpawnWithOptions(db, runner, agentType, task, files, context, name, false, io.Discard)
}

func runSpawnWithOptions(db *sql.DB, runner ottoexec.Runner, agentType, task, files, context, name string, detach bool, w io.Writer) error {
	// Get current context
	ctx := scope.CurrentContext()

	// Generate agent ID: use provided name or auto-generate from task
	var agentID string
	if name != "" {
		agentID = resolveAgentName(db, ctx, name)
	} else {
		agentID = generateAgentID(db, ctx, task)
	}

	// Generate session ID (for Claude, or as placeholder for Codex until we capture thread_id)
	sessionID := uuid.New().String()

	// Create agent row (status: busy)
	agent := repo.Agent{
		Project:   ctx.Project,
		Branch:    ctx.Branch,
		Name:      agentID,
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

	// Store short summary in messages, full prompt in logs
	summary := fmt.Sprintf("spawned %s - %s", agentID, task)
	if err := storePrompt(db, ctx, agentID, summary, prompt); err != nil {
		return fmt.Errorf("store prompt: %w", err)
	}

	// Build and run command
	if detach {
		// Launch otto worker-spawn instead of agent directly
		if w == nil {
			w = io.Discard
		}

		// Launch worker-spawn in detached mode
		pid, err := runner.StartDetached(ottoBin, "worker-spawn", agentID)
		if err != nil {
			// On failure: record launch error, mark agent as failed, and create exit message
			repoRoot := scope.RepoRoot()
			branch := scope.BranchName()
			if branch == "" {
				branch = "main"
			}
			scopePath := scope.Scope(repoRoot, branch)
			errorText := fmt.Sprintf("failed to start worker: %v", err)
			_ = repo.RecordLaunchError(scopePath, agentID, errorText)

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
			return fmt.Errorf("spawn worker: %w", err)
		}

		_ = repo.UpdateAgentPid(db, ctx.Project, ctx.Branch, agentID, pid)
		fmt.Fprintln(w, agentID)
		return nil
	}

	cmdArgs := buildSpawnCommand(agentType, prompt, sessionID)

	// For Codex agents, we need to capture the thread_id from JSON output
	if agentType == "codex" {
		return runCodexSpawn(db, runner, ctx, agentID, cmdArgs)
	}

	// For Claude agents, use transcript capture
	pid, output, wait, err := runner.StartWithTranscriptCapture(cmdArgs[0], cmdArgs[1:]...)
	if err != nil {
		return fmt.Errorf("spawn %s: %w", agentType, err)
	}

	// Update agent with PID
	_ = repo.UpdateAgentPid(db, ctx.Project, ctx.Branch, agentID, pid)

	transcriptDone := consumeTranscriptEntries(db, ctx, agentID, output, nil)

	// Wait for process
	if err := wait(); err != nil {
		if consumeErr := <-transcriptDone; consumeErr != nil {
			return fmt.Errorf("spawn %s: %w", agentType, consumeErr)
		}
		// Post failure message and mark agent failed
		msg := repo.Message{
			ID:        uuid.New().String(),
			Project:   ctx.Project,
			Branch:    ctx.Branch,
			FromAgent: agentID,
			Type:      "exit",
			Content:   fmt.Sprintf("process failed: %v", err),
			MentionsJSON: "[]",
			ReadByJSON:   "[]",
		}
		_ = repo.CreateMessage(db, msg)
		_ = repo.SetAgentFailed(db, ctx.Project, ctx.Branch, agentID)
		return fmt.Errorf("spawn %s: %w", agentType, err)
	}

	if consumeErr := <-transcriptDone; consumeErr != nil {
		return fmt.Errorf("spawn %s: %w", agentType, consumeErr)
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

func runCodexSpawn(db *sql.DB, runner ottoexec.Runner, ctx scope.Context, agentID string, cmdArgs []string) error {
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
				_ = repo.UpdateAgentSessionID(db, ctx.Project, ctx.Branch, agentID, threadID)
				threadIDCaptured = true
			}
		}
	}

	transcriptDone := consumeTranscriptEntries(db, ctx, agentID, output, threadIDParser)

	// Wait for process
	if err := wait(); err != nil {
		if consumeErr := <-transcriptDone; consumeErr != nil {
			return fmt.Errorf("spawn codex: %w", consumeErr)
		}
		// Post failure message and mark agent failed
		msg := repo.Message{
			ID:        uuid.New().String(),
			Project:   ctx.Project,
			Branch:    ctx.Branch,
			FromAgent: agentID,
			Type:      "exit",
			Content:   fmt.Sprintf("process failed: %v", err),
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

	// If we didn't capture thread_id, log a warning (but don't fail)
	if !threadIDCaptured {
		fmt.Fprintf(os.Stderr, "Warning: Could not capture thread_id for Codex agent %s\n", agentID)
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

func generateAgentID(db *sql.DB, ctx scope.Context, task string) string {
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
		_, err := repo.GetAgent(db, ctx.Project, ctx.Branch, slug)
		if err == sql.ErrNoRows {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", baseSlug, counter)
		counter++
	}
}

func resolveAgentName(db *sql.DB, ctx scope.Context, name string) string {
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
		_, err := repo.GetAgent(db, ctx.Project, ctx.Branch, slug)
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

	// Parse and execute the embedded template
	tmpl, err := template.New("instructions").Parse(agentInstructionsTemplate)
	if err != nil {
		// Fallback to basic instructions if template fails
		prompt += fmt.Sprintf("\n\nUse %s complete --id %s when done.", ottoBin, agentID)
		return prompt
	}

	data := struct {
		AgentID string
		OttoBin string
	}{
		AgentID: agentID,
		OttoBin: ottoBin,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		prompt += fmt.Sprintf("\n\nUse %s complete --id %s when done.", ottoBin, agentID)
		return prompt
	}

	prompt += "\n\n" + buf.String()
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
