package commands

import (
	"bytes"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"otto/internal/db"
	ottoexec "otto/internal/exec"
	"otto/internal/repo"
)

func TestSpawnBuildsCommand(t *testing.T) {
	cmd := buildSpawnCommand("claude", "task", "sess-123")
	if got := cmd[0]; got != "claude" {
		t.Fatalf("expected claude, got %q", got)
	}
}

func TestSpawnBuildsClaudeCommand(t *testing.T) {
	cmd := buildSpawnCommand("claude", "test prompt", "session-123")
	expected := []string{"claude", "-p", "test prompt", "--session-id", "session-123"}

	if len(cmd) != len(expected) {
		t.Fatalf("expected %d args, got %d", len(expected), len(cmd))
	}

	for i, arg := range expected {
		if cmd[i] != arg {
			t.Fatalf("arg %d: expected %q, got %q", i, arg, cmd[i])
		}
	}
}

func TestSpawnBuildsCodexCommand(t *testing.T) {
	cmd := buildSpawnCommand("codex", "test prompt", "session-123")
	expected := []string{"codex", "exec", "--json", "--skip-git-repo-check", "-s", "danger-full-access", "test prompt"}

	if len(cmd) != len(expected) {
		t.Fatalf("expected %d args, got %d", len(expected), len(cmd))
	}

	for i, arg := range expected {
		if cmd[i] != arg {
			t.Fatalf("arg %d: expected %q, got %q", i, arg, cmd[i])
		}
	}
}

func TestGenerateAgentID(t *testing.T) {
	db := openTestDB(t)

	tests := []struct {
		name     string
		task     string
		expected string
	}{
		{"simple", "auth backend", "authbackend"},
		{"with dashes", "auth-backend-api", "authbackendapi"},
		{"long task", "this is a very long task name that exceeds sixteen chars", "thisisaverylongt"},
		{"special chars", "task#1: fix @bugs!", "task1fixbugs"},
		{"empty after filter", "!!!", "agent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateAgentID(db, tt.task)
			if result != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGenerateAgentIDUnique(t *testing.T) {
	db := openTestDB(t)

	// Create first agent
	_ = repo.CreateAgent(db, repo.Agent{
		ID:        "authbackend",
		Type:      "claude",
		Task:      "task",
		Status:    "busy",
		SessionID: sql.NullString{String: "session-1", Valid: true},
	})

	// Generate ID for same task should get -2 suffix
	result := generateAgentID(db, "auth backend")
	if result != "authbackend-2" {
		t.Fatalf("expected authbackend-2, got %q", result)
	}

	// Create second agent
	_ = repo.CreateAgent(db, repo.Agent{
		ID:        "authbackend-2",
		Type:      "claude",
		Task:      "task",
		Status:    "busy",
		SessionID: sql.NullString{String: "session-2", Valid: true},
	})

	// Generate ID for same task should get -3 suffix
	result = generateAgentID(db, "auth backend")
	if result != "authbackend-3" {
		t.Fatalf("expected authbackend-3, got %q", result)
	}
}

func TestResolveAgentName(t *testing.T) {
	db := openTestDB(t)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "researcher", "researcher"},
		{"with hyphens", "auth-backend-api", "auth-backend-api"},
		{"uppercase", "MyAgent", "myagent"},
		{"special chars", "agent#1: test@!", "agent1test"},
		{"multiple hyphens", "agent---name", "agent-name"},
		{"leading/trailing hyphens", "-agent-", "agent"},
		{"empty after filter", "!!!", "agent"},
		{"spaces", "my agent name", "myagentname"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveAgentName(db, tt.input)
			if result != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestResolveAgentNameUnique(t *testing.T) {
	db := openTestDB(t)

	// Create first agent with name "researcher"
	_ = repo.CreateAgent(db, repo.Agent{
		ID:        "researcher",
		Type:      "claude",
		Task:      "task",
		Status:    "busy",
		SessionID: sql.NullString{String: "session-1", Valid: true},
	})

	// Resolve same name should get -2 suffix
	result := resolveAgentName(db, "researcher")
	if result != "researcher-2" {
		t.Fatalf("expected researcher-2, got %q", result)
	}

	// Create second agent
	_ = repo.CreateAgent(db, repo.Agent{
		ID:        "researcher-2",
		Type:      "claude",
		Task:      "task",
		Status:    "busy",
		SessionID: sql.NullString{String: "session-2", Valid: true},
	})

	// Resolve same name should get -3 suffix
	result = resolveAgentName(db, "researcher")
	if result != "researcher-3" {
		t.Fatalf("expected researcher-3, got %q", result)
	}
}

func TestBuildSpawnPrompt(t *testing.T) {
	prompt := buildSpawnPrompt("test-agent", "build auth", "", "", "/usr/local/bin/otto")

	if !strings.Contains(prompt, "test-agent") {
		t.Fatal("prompt should contain agent ID")
	}
	if !strings.Contains(prompt, "build auth") {
		t.Fatal("prompt should contain task")
	}
	if !strings.Contains(prompt, "/usr/local/bin/otto messages --id test-agent") {
		t.Fatal("prompt should contain communication template with otto path")
	}
}

func TestBuildSpawnPromptWithFilesAndContext(t *testing.T) {
	prompt := buildSpawnPrompt("test-agent", "task", "auth.go,user.go", "use JWT tokens", "otto")

	if !strings.Contains(prompt, "auth.go,user.go") {
		t.Fatal("prompt should contain files")
	}
	if !strings.Contains(prompt, "use JWT tokens") {
		t.Fatal("prompt should contain context")
	}
}

func TestSpawnStoresPromptAndTranscript(t *testing.T) {
	db := openTestDB(t)

	chunks := make(chan ottoexec.TranscriptChunk, 2)
	chunks <- ottoexec.TranscriptChunk{Stream: "stdout", Data: "hello\n"}
	chunks <- ottoexec.TranscriptChunk{Stream: "stderr", Data: "oops\n"}
	close(chunks)

	runner := &mockRunner{
		startWithTranscriptCaptureFunc: func(name string, args ...string) (int, <-chan ottoexec.TranscriptChunk, func() error, error) {
			return 1234, chunks, func() error { return nil }, nil
		},
	}

	err := runSpawn(db, runner, "claude", "test task", "", "", "")
	if err != nil {
		t.Fatalf("runSpawn failed: %v", err)
	}

	msgs, err := repo.ListMessages(db, repo.MessageFilter{Type: "prompt", ToID: "testtask"})
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 prompt message, got %d", len(msgs))
	}
	// Message should contain short summary
	if !strings.Contains(msgs[0].Content, "spawned") || !strings.Contains(msgs[0].Content, "test task") {
		t.Fatalf("expected message to be short summary, got %q", msgs[0].Content)
	}

	entries, err := repo.ListLogs(db, "testtask", "")
	if err != nil {
		t.Fatalf("list transcript entries: %v", err)
	}

	var inCount, outCount int
	var streams []string
	for _, entry := range entries {
		switch entry.Direction {
		case "in":
			inCount++
			// Log should contain full prompt (different from summary message)
			if !strings.Contains(entry.Content, "test task") || !strings.Contains(entry.Content, "CRITICAL RULES") {
				t.Fatalf("expected full prompt in transcript, got %q", entry.Content)
			}
		case "out":
			outCount++
			if entry.Stream.Valid {
				streams = append(streams, entry.Stream.String)
			}
		}
	}

	if inCount != 1 {
		t.Fatalf("expected 1 input transcript entry, got %d", inCount)
	}
	if outCount != 2 {
		t.Fatalf("expected 2 output transcript entries, got %d", outCount)
	}
	if len(streams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(streams))
	}
}

func TestSpawnDetach(t *testing.T) {
	conn, _ := db.Open(":memory:")
	defer conn.Close()

	runner := &mockRunner{
		startDetachedFunc: func(name string, args ...string) (int, error) {
			return 12345, nil
		},
	}

	var buf bytes.Buffer
	err := runSpawnWithOptions(conn, runner, "claude", "test task", "", "", "", true, &buf)
	if err != nil {
		t.Fatalf("spawn detach failed: %v", err)
	}

	// Should print agent ID
	output := buf.String()
	if !strings.Contains(output, "test") {
		t.Errorf("expected agent ID in output, got: %s", output)
	}

	// Agent should exist and be busy
	agents, _ := repo.ListAgents(conn)
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Status != "busy" {
		t.Errorf("expected status busy, got %s", agents[0].Status)
	}
	if !agents[0].Pid.Valid || agents[0].Pid.Int64 != 12345 {
		t.Errorf("expected PID 12345, got %v", agents[0].Pid)
	}
}

// mockRunner for testing
type mockRunner struct {
	startWithCaptureFunc           func(name string, args ...string) (int, <-chan string, func() error, error)
	startWithCaptureEnvFunc        func(name string, env []string, args ...string) (int, <-chan string, func() error, error)
	startWithTranscriptCaptureFunc func(name string, args ...string) (int, <-chan ottoexec.TranscriptChunk, func() error, error)
	startWithTranscriptCaptureEnv  func(name string, env []string, args ...string) (int, <-chan ottoexec.TranscriptChunk, func() error, error)
	startFunc                      func(name string, args ...string) (int, func() error, error)
	startDetachedFunc              func(name string, args ...string) (int, error)
}

// Ensure mockRunner implements ottoexec.Runner
var _ ottoexec.Runner = (*mockRunner)(nil)

func (m *mockRunner) Run(name string, args ...string) error {
	return nil
}

func (m *mockRunner) RunWithEnv(name string, env []string, args ...string) error {
	return nil
}

func (m *mockRunner) Start(name string, args ...string) (int, func() error, error) {
	if m.startFunc != nil {
		return m.startFunc(name, args...)
	}
	return 1234, func() error { return nil }, nil
}

func (m *mockRunner) StartDetached(name string, args ...string) (int, error) {
	if m.startDetachedFunc != nil {
		return m.startDetachedFunc(name, args...)
	}
	return 1234, nil
}

func (m *mockRunner) StartWithCapture(name string, args ...string) (int, <-chan string, func() error, error) {
	if m.startWithCaptureFunc != nil {
		return m.startWithCaptureFunc(name, args...)
	}
	lines := make(chan string)
	close(lines)
	return 1234, lines, func() error { return nil }, nil
}

func (m *mockRunner) StartWithCaptureEnv(name string, env []string, args ...string) (int, <-chan string, func() error, error) {
	if m.startWithCaptureEnvFunc != nil {
		return m.startWithCaptureEnvFunc(name, env, args...)
	}
	lines := make(chan string)
	close(lines)
	return 1234, lines, func() error { return nil }, nil
}

func (m *mockRunner) StartWithTranscriptCapture(name string, args ...string) (int, <-chan ottoexec.TranscriptChunk, func() error, error) {
	if m.startWithTranscriptCaptureFunc != nil {
		return m.startWithTranscriptCaptureFunc(name, args...)
	}
	chunks := make(chan ottoexec.TranscriptChunk)
	close(chunks)
	return 1234, chunks, func() error { return nil }, nil
}

func (m *mockRunner) StartWithTranscriptCaptureEnv(name string, env []string, args ...string) (int, <-chan ottoexec.TranscriptChunk, func() error, error) {
	if m.startWithTranscriptCaptureEnv != nil {
		return m.startWithTranscriptCaptureEnv(name, env, args...)
	}
	chunks := make(chan ottoexec.TranscriptChunk)
	close(chunks)
	return 1234, chunks, func() error { return nil }, nil
}

func TestCodexSpawnCapturesThreadID(t *testing.T) {
	db := openTestDB(t)

	// Create mock runner that simulates Codex JSON output
	chunks := make(chan ottoexec.TranscriptChunk, 5)
	chunks <- ottoexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"other_event","data":"something"}` + "\n"}
	chunks <- ottoexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"thread.started","thread_id":"thread_abc123"}` + "\n"}
	chunks <- ottoexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"message","content":"hello"}` + "\n"}
	close(chunks)

	runner := &mockRunner{
		startWithTranscriptCaptureEnv: func(name string, env []string, args ...string) (int, <-chan ottoexec.TranscriptChunk, func() error, error) {
			return 5678, chunks, func() error { return nil }, nil
		},
	}

	// Run Codex spawn
	err := runSpawn(db, runner, "codex", "test task", "", "", "")
	if err != nil {
		t.Fatalf("runSpawn failed: %v", err)
	}

	agent, err := repo.GetAgent(db, "testtask")
	if err != nil {
		t.Fatalf("expected agent to exist, got err=%v", err)
	}
	if agent.SessionID.String != "thread_abc123" {
		t.Fatalf("expected session_id to be thread_abc123, got %q", agent.SessionID.String)
	}
	if agent.Status != "complete" {
		t.Fatalf("expected status complete, got %q", agent.Status)
	}
	if !agent.CompletedAt.Valid {
		t.Fatal("expected completed_at to be set")
	}
}

func TestCodexSpawnWithoutThreadID(t *testing.T) {
	db := openTestDB(t)

	// Create mock runner with no thread.started event
	chunks := make(chan ottoexec.TranscriptChunk, 3)
	chunks <- ottoexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"message","content":"hello"}` + "\n"}
	chunks <- ottoexec.TranscriptChunk{Stream: "stdout", Data: `{"type":"other_event","data":"something"}` + "\n"}
	close(chunks)

	runner := &mockRunner{
		startWithTranscriptCaptureEnv: func(name string, env []string, args ...string) (int, <-chan ottoexec.TranscriptChunk, func() error, error) {
			return 5678, chunks, func() error { return nil }, nil
		},
	}

	// Run Codex spawn
	err := runSpawn(db, runner, "codex", "test task", "", "", "")
	if err != nil {
		t.Fatalf("runSpawn failed: %v", err)
	}

	agent, err := repo.GetAgent(db, "testtask")
	if err != nil {
		t.Fatalf("expected agent to exist, got err=%v", err)
	}
	if agent.Status != "complete" {
		t.Fatalf("expected status complete, got %q", agent.Status)
	}
	if !agent.CompletedAt.Valid {
		t.Fatal("expected completed_at to be set")
	}
}

func TestClaudeSpawnUsesNormalStart(t *testing.T) {
	db := openTestDB(t)

	called := false
	runner := &mockRunner{
		startWithTranscriptCaptureFunc: func(name string, args ...string) (int, <-chan ottoexec.TranscriptChunk, func() error, error) {
			called = true
			// Verify it's a Claude command
			if name != "claude" {
				t.Fatalf("expected 'claude', got %q", name)
			}
			chunks := make(chan ottoexec.TranscriptChunk)
			close(chunks)
			return 1234, chunks, func() error { return nil }, nil
		},
	}

	// Run Claude spawn
	err := runSpawn(db, runner, "claude", "test task", "", "", "")
	if err != nil {
		t.Fatalf("runSpawn failed: %v", err)
	}

	if !called {
		t.Fatal("StartWithTranscriptCapture() should have been called for Claude")
	}
}

func TestCodexSpawnSetsCodexHome(t *testing.T) {
	db := openTestDB(t)

	var capturedEnv []string
	chunks := make(chan ottoexec.TranscriptChunk)
	close(chunks)

	runner := &mockRunner{
		startWithTranscriptCaptureEnv: func(name string, env []string, args ...string) (int, <-chan ottoexec.TranscriptChunk, func() error, error) {
			capturedEnv = env
			return 5678, chunks, func() error { return nil }, nil
		},
	}

	// Run Codex spawn
	err := runSpawn(db, runner, "codex", "test task", "", "", "")
	if err != nil {
		t.Fatalf("runSpawn failed: %v", err)
	}

	// Verify CODEX_HOME was set
	found := false
	for _, envVar := range capturedEnv {
		if strings.HasPrefix(envVar, "CODEX_HOME=") {
			found = true
			// Verify it's a temp directory
			codexHome := strings.TrimPrefix(envVar, "CODEX_HOME=")
			if !strings.Contains(codexHome, "otto-codex-") {
				t.Fatalf("CODEX_HOME should be temp dir with 'otto-codex-' prefix, got %q", codexHome)
			}
			break
		}
	}

	if !found {
		t.Fatal("CODEX_HOME environment variable should be set for Codex agents")
	}
}

func TestSpawnWithCustomName(t *testing.T) {
	db := openTestDB(t)

	// Channel to signal when we've verified the agent
	agentVerified := make(chan bool)

	runner := &mockRunner{
		startWithTranscriptCaptureFunc: func(name string, args ...string) (int, <-chan ottoexec.TranscriptChunk, func() error, error) {
			// Check agent exists with custom name while process is "running"
			_, err := repo.GetAgent(db, "researcher")
			if err != nil {
				t.Errorf("expected agent with ID 'researcher', got error: %v", err)
			}

			// Verify auto-generated ID was NOT used
			_, err = repo.GetAgent(db, "buildtheauthback") // auto-generated would be first 16 chars
			if err != sql.ErrNoRows {
				t.Error("expected no agent with auto-generated ID, but found one")
			}

			// Signal verification complete
			close(agentVerified)

			chunks := make(chan ottoexec.TranscriptChunk)
			close(chunks)
			return 1234, chunks, func() error {
				<-agentVerified // Wait for verification
				return nil
			}, nil
		},
	}

	// Spawn with custom name
	err := runSpawn(db, runner, "claude", "build the auth backend", "", "", "researcher")
	if err != nil {
		t.Fatalf("runSpawn failed: %v", err)
	}

	agent, err := repo.GetAgent(db, "researcher")
	if err != nil {
		t.Fatalf("expected agent to exist after completion, got err=%v", err)
	}
	if agent.Status != "complete" {
		t.Fatalf("expected status complete, got %q", agent.Status)
	}
}

func TestSpawnWithCustomNameCollision(t *testing.T) {
	db := openTestDB(t)

	// Create first agent with name "researcher"
	_ = repo.CreateAgent(db, repo.Agent{
		ID:        "researcher",
		Type:      "claude",
		Task:      "task 1",
		Status:    "busy",
		SessionID: sql.NullString{String: "session-1", Valid: true},
	})

	// Channel to signal when we've verified the agent
	agentVerified := make(chan bool)

	runner := &mockRunner{
		startWithTranscriptCaptureFunc: func(name string, args ...string) (int, <-chan ottoexec.TranscriptChunk, func() error, error) {
			// Verify second agent got -2 suffix while process is "running"
			_, err := repo.GetAgent(db, "researcher-2")
			if err != nil {
				t.Errorf("expected agent with ID 'researcher-2', got error: %v", err)
			}

			// Signal verification complete
			close(agentVerified)

			chunks := make(chan ottoexec.TranscriptChunk)
			close(chunks)
			return 1234, chunks, func() error {
				<-agentVerified // Wait for verification
				return nil
			}, nil
		},
	}

	// Spawn with same custom name
	err := runSpawn(db, runner, "claude", "task 2", "", "", "researcher")
	if err != nil {
		t.Fatalf("runSpawn failed: %v", err)
	}

	agent, err := repo.GetAgent(db, "researcher-2")
	if err != nil {
		t.Fatalf("expected agent to exist after completion, got err=%v", err)
	}
	if agent.Status != "complete" {
		t.Fatalf("expected status complete, got %q", agent.Status)
	}

	// First agent should still exist
	_, err = repo.GetAgent(db, "researcher")
	if err != nil {
		t.Fatalf("expected first agent to still exist, got error: %v", err)
	}
}

func TestSpawnDetachLaunchesWorker(t *testing.T) {
	db := openTestDB(t)

	var capturedCmd string
	var capturedArgs []string

	runner := &mockRunner{
		startDetachedFunc: func(name string, args ...string) (int, error) {
			capturedCmd = name
			capturedArgs = args
			return 12345, nil
		},
	}

	var buf bytes.Buffer
	err := runSpawnWithOptions(db, runner, "claude", "test task", "", "", "", true, &buf)
	if err != nil {
		t.Fatalf("spawn detach failed: %v", err)
	}

	// Should launch otto worker-spawn <agent-id>
	// capturedCmd should be the path to otto binary (or test binary during tests)
	// Just verify it's not launching claude/codex directly
	if capturedCmd == "claude" || capturedCmd == "codex" {
		t.Errorf("expected command to be otto binary, not %q", capturedCmd)
	}

	// Args should be: worker-spawn <agent-id>
	if len(capturedArgs) != 2 {
		t.Fatalf("expected 2 args (worker-spawn <agent-id>), got %d: %v", len(capturedArgs), capturedArgs)
	}
	if capturedArgs[0] != "worker-spawn" {
		t.Errorf("expected first arg to be 'worker-spawn', got %q", capturedArgs[0])
	}
	// Second arg should be the agent ID (generated from task "test task")
	if !strings.HasPrefix(capturedArgs[1], "test") {
		t.Errorf("expected agent ID to start with 'test', got %q", capturedArgs[1])
	}

	// Verify agent was created
	agentID := capturedArgs[1]
	agent, err := repo.GetAgent(db, agentID)
	if err != nil {
		t.Fatalf("expected agent to exist, got error: %v", err)
	}
	if agent.Status != "busy" {
		t.Errorf("expected status busy, got %q", agent.Status)
	}
	if !agent.Pid.Valid || agent.Pid.Int64 != 12345 {
		t.Errorf("expected PID 12345, got %v", agent.Pid)
	}
}

func TestSpawnDetachHandlesWorkerLaunchFailure(t *testing.T) {
	db := openTestDB(t)

	runner := &mockRunner{
		startDetachedFunc: func(name string, args ...string) (int, error) {
			return 0, fmt.Errorf("failed to start worker")
		},
	}

	var buf bytes.Buffer
	err := runSpawnWithOptions(db, runner, "claude", "test task", "", "", "", true, &buf)
	if err == nil {
		t.Fatal("expected error when worker launch fails")
	}

	if !strings.Contains(err.Error(), "failed to start worker") {
		t.Errorf("expected error to mention worker launch failure, got: %v", err)
	}

	// Agent should exist but marked as failed
	agents, _ := repo.ListAgents(db)
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Status != "failed" {
		t.Errorf("expected status failed, got %q", agents[0].Status)
	}

	// Should have exit message
	msgs, _ := repo.ListMessages(db, repo.MessageFilter{Type: "exit"})
	if len(msgs) != 1 {
		t.Fatalf("expected 1 exit message, got %d", len(msgs))
	}
	if !strings.Contains(msgs[0].Content, "failed to start worker") {
		t.Errorf("expected exit message to mention failure, got: %q", msgs[0].Content)
	}
}
