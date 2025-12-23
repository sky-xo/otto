package commands

import (
	"database/sql"
	"strings"
	"testing"

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
	expected := []string{"codex", "exec", "--json", "test prompt"}

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
		ID:     "authbackend",
		Type:   "claude",
		Task:   "task",
		Status: "busy",
		SessionID: sql.NullString{String: "session-1", Valid: true},
	})

	// Generate ID for same task should get -2 suffix
	result := generateAgentID(db, "auth backend")
	if result != "authbackend-2" {
		t.Fatalf("expected authbackend-2, got %q", result)
	}

	// Create second agent
	_ = repo.CreateAgent(db, repo.Agent{
		ID:     "authbackend-2",
		Type:   "claude",
		Task:   "task",
		Status: "busy",
		SessionID: sql.NullString{String: "session-2", Valid: true},
	})

	// Generate ID for same task should get -3 suffix
	result = generateAgentID(db, "auth backend")
	if result != "authbackend-3" {
		t.Fatalf("expected authbackend-3, got %q", result)
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

// mockRunner for testing
type mockRunner struct {
	startWithCaptureFunc    func(name string, args ...string) (int, <-chan string, func() error, error)
	startWithCaptureEnvFunc func(name string, env []string, args ...string) (int, <-chan string, func() error, error)
	startFunc               func(name string, args ...string) (int, func() error, error)
}

// Ensure mockRunner implements ottoexec.Runner
var _ ottoexec.Runner = (*mockRunner)(nil)

func (m *mockRunner) Run(name string, args ...string) error {
	return nil
}

func (m *mockRunner) Start(name string, args ...string) (int, func() error, error) {
	if m.startFunc != nil {
		return m.startFunc(name, args...)
	}
	return 1234, func() error { return nil }, nil
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

func TestCodexSpawnCapturesThreadID(t *testing.T) {
	db := openTestDB(t)

	// Create mock runner that simulates Codex JSON output
	lines := make(chan string, 5)
	lines <- `{"type":"other_event","data":"something"}`
	lines <- `{"type":"thread.started","thread_id":"thread_abc123"}`
	lines <- `{"type":"message","content":"hello"}`
	close(lines)

	runner := &mockRunner{
		startWithCaptureEnvFunc: func(name string, env []string, args ...string) (int, <-chan string, func() error, error) {
			return 5678, lines, func() error { return nil }, nil
		},
	}

	// Run Codex spawn
	err := runSpawn(db, runner, "codex", "test task", "", "")
	if err != nil {
		t.Fatalf("runSpawn failed: %v", err)
	}

	// Agent should be deleted after process exits
	_, err = repo.GetAgent(db, "testtask")
	if err != sql.ErrNoRows {
		t.Fatalf("expected agent to be deleted, got err=%v", err)
	}
}

func TestCodexSpawnWithoutThreadID(t *testing.T) {
	db := openTestDB(t)

	// Create mock runner with no thread.started event
	lines := make(chan string, 3)
	lines <- `{"type":"message","content":"hello"}`
	lines <- `{"type":"other_event","data":"something"}`
	close(lines)

	runner := &mockRunner{
		startWithCaptureEnvFunc: func(name string, env []string, args ...string) (int, <-chan string, func() error, error) {
			return 5678, lines, func() error { return nil }, nil
		},
	}

	// Run Codex spawn
	err := runSpawn(db, runner, "codex", "test task", "", "")
	if err != nil {
		t.Fatalf("runSpawn failed: %v", err)
	}

	// Agent should be deleted after process exits
	_, err = repo.GetAgent(db, "testtask")
	if err != sql.ErrNoRows {
		t.Fatalf("expected agent to be deleted, got err=%v", err)
	}
}

func TestClaudeSpawnUsesNormalStart(t *testing.T) {
	db := openTestDB(t)

	called := false
	runner := &mockRunner{
		startFunc: func(name string, args ...string) (int, func() error, error) {
			called = true
			// Verify it's a Claude command
			if name != "claude" {
				t.Fatalf("expected 'claude', got %q", name)
			}
			return 1234, func() error { return nil }, nil
		},
	}

	// Run Claude spawn
	err := runSpawn(db, runner, "claude", "test task", "", "")
	if err != nil {
		t.Fatalf("runSpawn failed: %v", err)
	}

	if !called {
		t.Fatal("Start() should have been called for Claude")
	}
}

func TestCodexSpawnSetsCodexHome(t *testing.T) {
	db := openTestDB(t)

	var capturedEnv []string
	lines := make(chan string)
	close(lines)

	runner := &mockRunner{
		startWithCaptureEnvFunc: func(name string, env []string, args ...string) (int, <-chan string, func() error, error) {
			capturedEnv = env
			return 5678, lines, func() error { return nil }, nil
		},
	}

	// Run Codex spawn
	err := runSpawn(db, runner, "codex", "test task", "", "")
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
