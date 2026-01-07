package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sky-xo/june/internal/codex"
	"github.com/sky-xo/june/internal/db"
	"github.com/sky-xo/june/internal/scope"
	"github.com/spf13/cobra"
)

func newSpawnCmd() *cobra.Command {
	var (
		name            string
		model           string
		reasoningEffort string
		maxTokens       int
		sandbox         string
	)

	cmd := &cobra.Command{
		Use:   "spawn <type> <task>",
		Short: "Spawn an agent",
		Long:  "Spawn a Codex agent to perform a task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentType := args[0]
			task := args[1]

			if agentType != "codex" {
				return fmt.Errorf("unsupported agent type: %s (only 'codex' is supported)", agentType)
			}

			if name == "" {
				return fmt.Errorf("--name is required")
			}

			return runSpawnCodex(name, task, model, reasoningEffort, sandbox, maxTokens)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name for the agent (required)")
	cmd.MarkFlagRequired("name")
	cmd.Flags().StringVar(&model, "model", "", "Codex model to use")
	cmd.Flags().StringVar(&reasoningEffort, "reasoning-effort", "", "Reasoning effort (minimal|low|medium|high|xhigh)")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 0, "Max output tokens")
	cmd.Flags().StringVar(&sandbox, "sandbox", "", "Sandbox mode (read-only|workspace-write|danger-full-access)")

	return cmd
}

func runSpawnCodex(name, task, model, reasoningEffort, sandbox string, maxTokens int) error {
	// Capture git context before spawning
	// Non-fatal if not in a git repo - we just won't have channel info
	repoPath := scope.RepoRoot()
	branch := scope.BranchName()

	// Open database
	home, err := juneHome()
	if err != nil {
		return fmt.Errorf("failed to get june home: %w", err)
	}
	dbPath := filepath.Join(home, "june.db")
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	// Check if agent already exists
	if _, err := database.GetAgent(name); err == nil {
		return fmt.Errorf("agent %q already exists", name)
	} else if err != db.ErrAgentNotFound {
		return fmt.Errorf("failed to check for existing agent: %w", err)
	}

	// Before creating the command, ensure isolated codex home
	isolatedCodexHome, err := codex.EnsureCodexHome()
	if err != nil {
		return fmt.Errorf("failed to setup isolated codex home: %w", err)
	}

	// Build codex command arguments dynamically
	args := buildCodexArgs(task, model, reasoningEffort, sandbox, maxTokens)

	// Start codex exec --json
	codexCmd := exec.Command("codex", args...)
	codexCmd.Stderr = os.Stderr
	codexCmd.Env = append(os.Environ(), fmt.Sprintf("CODEX_HOME=%s", isolatedCodexHome))

	stdout, err := codexCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := codexCmd.Start(); err != nil {
		return fmt.Errorf("failed to start codex: %w", err)
	}

	// Read first line to get thread_id
	scanner := bufio.NewScanner(stdout)
	var threadID string
	if scanner.Scan() {
		var event struct {
			Type     string `json:"type"`
			ThreadID string `json:"thread_id"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &event); err == nil {
			if event.Type == "thread.started" {
				threadID = event.ThreadID
			}
		}
	}

	if threadID == "" {
		codexCmd.Process.Kill()
		codexCmd.Wait() // Reap the killed process
		return fmt.Errorf("failed to get thread_id from codex output")
	}

	// Find the session file
	sessionFile, err := codex.FindSessionFile(threadID)
	if err != nil {
		// Session file might not exist yet, construct expected path
		// For now, we'll store it and look it up later
		sessionFile = "" // Will be populated later
	}

	// Create agent record
	agent := db.Agent{
		Name:        name,
		ULID:        threadID,
		SessionFile: sessionFile,
		PID:         codexCmd.Process.Pid,
		RepoPath:    repoPath,
		Branch:      branch,
	}
	if err := database.CreateAgent(agent); err != nil {
		return fmt.Errorf("failed to create agent record: %w", err)
	}

	// Drain remaining output (without printing)
	for scanner.Scan() {
		// Consume output silently
	}

	// Wait for process to finish
	if err := codexCmd.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "codex exited with error: %v\n", err)
	}

	// Update session file if we didn't have it
	if sessionFile == "" {
		if found, err := codex.FindSessionFile(threadID); err == nil {
			// Update the agent record with the session file
			if err := database.UpdateSessionFile(name, found); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to update session file: %v\n", err)
			}
		}
	}

	return nil
}

// buildCodexArgs constructs the argument slice for the codex exec command.
func buildCodexArgs(task, model, reasoningEffort, sandbox string, maxTokens int) []string {
	args := []string{"exec", "--json"}
	if model != "" {
		args = append(args, "--model", model)
	}
	if reasoningEffort != "" {
		args = append(args, "-c", "model_reasoning_effort="+reasoningEffort)
	}
	if maxTokens > 0 {
		args = append(args, "-c", fmt.Sprintf("model_max_output_tokens=%d", maxTokens))
	}
	if sandbox != "" {
		args = append(args, "--sandbox", sandbox)
	}
	args = append(args, task)
	return args
}

func juneHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".june"), nil
}
