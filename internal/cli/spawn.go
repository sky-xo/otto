package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sky-xo/june/internal/codex"
	"github.com/sky-xo/june/internal/db"
	"github.com/sky-xo/june/internal/gemini"
	"github.com/sky-xo/june/internal/scope"
	"github.com/spf13/cobra"
)

func newSpawnCmd() *cobra.Command {
	var (
		name            string
		model           string
		yolo            bool
		sandbox         string
		reasoningEffort string
		maxTokens       int
	)

	cmd := &cobra.Command{
		Use:   "spawn <type> <task>",
		Short: "Spawn an agent",
		Long: `Spawn a Codex or Gemini agent to perform a task.

On success, prints the agent name to stdout (e.g., "swift-falcon-7d1e").
Use this name with peek, logs, and other commands.

Naming: --name sets a prefix; if omitted, an adjective-noun is auto-generated.
A 4-char suffix is always appended to ensure uniqueness.

Examples:
  june spawn codex "fix the tests" --name refactor  # Output: refactor-9c4f
  june spawn codex "add feature"                    # Output: swift-falcon-7d1e
  june peek swift-falcon-7d1e                       # Show new output`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentType := args[0]
			task := args[1]

			switch agentType {
			case "codex":
				// For Codex, if --sandbox was passed without value, default to workspace-write
				codexSandbox := sandbox
				if sandbox == "true" {
					codexSandbox = "workspace-write"
				}
				return runSpawnCodex(name, task, model, reasoningEffort, codexSandbox, maxTokens)
			case "gemini":
				// Gemini sandbox is boolean-only, reject explicit values
				if sandbox != "" && sandbox != "true" {
					return fmt.Errorf("Gemini --sandbox does not accept values, use --sandbox without a value")
				}
				geminiSandbox := cmd.Flags().Changed("sandbox")
				return runSpawnGemini(name, task, model, yolo, geminiSandbox)
			default:
				return fmt.Errorf("unsupported agent type: %s (supported: codex, gemini)", agentType)
			}
		},
	}

	// Shared flags
	cmd.Flags().StringVar(&name, "name", "", "Name prefix for the agent (auto-generated if omitted)")
	cmd.Flags().StringVar(&model, "model", "", "Model to use")
	cmd.Flags().StringVar(&sandbox, "sandbox", "", "Enable sandbox (Codex: optional value read-only|workspace-write|danger-full-access, defaults to workspace-write; Gemini: boolean)")
	cmd.Flags().Lookup("sandbox").NoOptDefVal = "true" // Allow --sandbox without value

	// Codex-specific flags
	cmd.Flags().StringVar(&reasoningEffort, "reasoning-effort", "", "Reasoning effort (codex only)")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 0, "Max output tokens (codex only)")

	// Gemini-specific flags
	cmd.Flags().BoolVar(&yolo, "yolo", false, "Auto-approve all actions (gemini only, default is auto_edit)")

	return cmd
}

func runSpawnCodex(prefix, task string, model, reasoningEffort, sandbox string, maxTokens int) error {
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

	// Resolve agent name using ULID (now that we have it)
	name, err := resolveAgentNameWithULID(database, prefix, threadID)
	if err != nil {
		codexCmd.Process.Kill()
		codexCmd.Wait()
		return fmt.Errorf("failed to resolve agent name: %w", err)
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
		Type:        "codex",
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

	// Print the agent name to confirm what was created
	fmt.Println(name)

	return nil
}

// buildGeminiArgs constructs the argument slice for the gemini command.
// sandbox is a boolean - for Gemini we just pass --sandbox if true.
func buildGeminiArgs(task, model string, yolo, sandbox bool) []string {
	args := []string{"-p", task, "--output-format", "stream-json"}

	if yolo {
		args = append(args, "--yolo")
	} else {
		args = append(args, "--approval-mode", "auto_edit")
	}

	if model != "" {
		args = append(args, "-m", model)
	}

	if sandbox {
		args = append(args, "--sandbox")
	}

	return args
}

// geminiInstalled checks if the gemini CLI is available in PATH.
func geminiInstalled() bool {
	_, err := exec.LookPath("gemini")
	return err == nil
}

func runSpawnGemini(prefix, task string, model string, yolo, sandbox bool) error {
	// Check if gemini is installed
	if !geminiInstalled() {
		return fmt.Errorf("gemini CLI not found - install with: npm install -g @google/gemini-cli")
	}

	// Capture git context before spawning
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

	// Ensure gemini home exists (copies auth files, creates sessions directory)
	_, err = gemini.EnsureGeminiHome()
	if err != nil {
		return fmt.Errorf("failed to setup gemini home: %w", err)
	}

	// Build gemini command arguments
	args := buildGeminiArgs(task, model, yolo, sandbox)

	// Start gemini -p ...
	geminiCmd := exec.Command("gemini", args...)
	geminiCmd.Stderr = os.Stderr

	stdout, err := geminiCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := geminiCmd.Start(); err != nil {
		return fmt.Errorf("failed to start gemini: %w", err)
	}

	// Read first line to get session_id
	// Use bufio.Reader instead of Scanner to handle arbitrarily large lines
	reader := bufio.NewReader(stdout)
	firstLine, err := reader.ReadBytes('\n')
	if err != nil && err != io.EOF {
		geminiCmd.Process.Kill()
		geminiCmd.Wait()
		return fmt.Errorf("failed to read first line from gemini: %w", err)
	}

	// Trim newline if present
	if len(firstLine) > 0 && firstLine[len(firstLine)-1] == '\n' {
		firstLine = firstLine[:len(firstLine)-1]
	}

	var sessionID string
	var event struct {
		Type      string `json:"type"`
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(firstLine, &event); err == nil {
		if event.Type == "init" {
			sessionID = event.SessionID
		}
	}

	if sessionID == "" {
		geminiCmd.Process.Kill()
		geminiCmd.Wait()
		return fmt.Errorf("failed to get session_id from gemini output")
	}

	// Get session file path and create it
	sessionFile, err := gemini.SessionFilePath(sessionID)
	if err != nil {
		geminiCmd.Process.Kill()
		geminiCmd.Wait()
		return fmt.Errorf("failed to get session file path: %w", err)
	}

	// Create session file and write first line
	f, err := os.Create(sessionFile)
	if err != nil {
		geminiCmd.Process.Kill()
		geminiCmd.Wait()
		return fmt.Errorf("failed to create session file: %w", err)
	}

	// Write the buffered first line
	if _, err := f.Write(firstLine); err != nil {
		f.Close()
		geminiCmd.Process.Kill()
		geminiCmd.Wait()
		return fmt.Errorf("failed to write to session file: %w", err)
	}
	if _, err := f.Write([]byte("\n")); err != nil {
		f.Close()
		geminiCmd.Process.Kill()
		geminiCmd.Wait()
		return fmt.Errorf("failed to write to session file: %w", err)
	}

	// Resolve agent name using session ID
	name, err := resolveAgentNameWithULID(database, prefix, sessionID)
	if err != nil {
		f.Close()
		geminiCmd.Process.Kill()
		geminiCmd.Wait()
		return fmt.Errorf("failed to resolve agent name: %w", err)
	}

	// Create agent record
	agent := db.Agent{
		Name:        name,
		ULID:        sessionID,
		SessionFile: sessionFile,
		PID:         geminiCmd.Process.Pid,
		RepoPath:    repoPath,
		Branch:      branch,
		Type:        "gemini",
	}
	if err := database.CreateAgent(agent); err != nil {
		f.Close()
		geminiCmd.Process.Kill()
		geminiCmd.Wait()
		return fmt.Errorf("failed to create agent record: %w", err)
	}

	// Stream remaining output to session file using streamLines (handles large lines)
	var writeErr error
	streamErr := streamLines(reader, func(line []byte) error {
		if _, err := f.Write(line); err != nil {
			return err
		}
		if _, err := f.Write([]byte("\n")); err != nil {
			return err
		}
		return nil
	})
	if streamErr != nil {
		writeErr = fmt.Errorf("error streaming gemini output: %w", streamErr)
	}

	// Check f.Close() error
	if err := f.Close(); err != nil && writeErr == nil {
		writeErr = fmt.Errorf("failed to close session file: %w", err)
	}

	if writeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: error writing session file: %v\n", writeErr)
	}

	// Wait for process to finish
	if err := geminiCmd.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "gemini exited with error: %v\n", err)
	}

	// Print the agent name
	fmt.Println(name)

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

// streamLines reads lines from r and calls fn for each line.
// Unlike bufio.Scanner, this handles arbitrarily large lines.
func streamLines(r io.Reader, fn func(line []byte) error) error {
	reader := bufio.NewReader(r)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			// Trim the newline if present
			if line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
			}
			if err := fn(line); err != nil {
				return err
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func juneHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".june"), nil
}
