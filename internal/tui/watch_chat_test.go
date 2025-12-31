package tui

import (
	"database/sql"
	"testing"

	"otto/internal/repo"

	_ "modernc.org/sqlite"
)

func TestChatInputAppearsForProjectHeader(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	// Select project header
	m.activeChannelID = "otto/main"

	// Should show chat input
	if !m.showChatInput() {
		t.Error("expected chat input to show when project header is selected")
	}

	// Select an agent - should NOT show chat input
	m.activeChannelID = "impl-1"
	if m.showChatInput() {
		t.Error("expected chat input to be hidden when agent is selected")
	}

	// Select Main - should NOT show chat input
	m.activeChannelID = mainChannelID
	if m.showChatInput() {
		t.Error("expected chat input to be hidden when Main is selected")
	}
}

func TestGetOttoAgentForProjectBranch(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "otto", Status: "complete"},
		{Project: "other", Branch: "feature", Name: "otto", Status: "busy"},
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	// Get otto for otto/main
	otto := m.getOttoAgent("otto", "main")
	if otto == nil {
		t.Fatal("expected to find otto agent for otto/main")
	}
	if otto.Name != "otto" || otto.Project != "otto" || otto.Branch != "main" {
		t.Errorf("got wrong agent: %+v", otto)
	}

	// Get otto for other/feature
	otto = m.getOttoAgent("other", "feature")
	if otto == nil {
		t.Fatal("expected to find otto agent for other/feature")
	}
	if otto.Status != "busy" {
		t.Errorf("expected busy otto, got %q", otto.Status)
	}

	// Get otto for non-existent project/branch
	otto = m.getOttoAgent("nonexistent", "branch")
	if otto != nil {
		t.Error("expected nil for non-existent project/branch")
	}
}

func TestIsOttoBusy(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "otto", Status: "busy"},
		{Project: "other", Branch: "feature", Name: "otto", Status: "complete"},
	}

	// otto/main has busy otto
	if !m.isOttoBusy("otto", "main") {
		t.Error("expected otto to be busy for otto/main")
	}

	// other/feature has complete otto
	if m.isOttoBusy("other", "feature") {
		t.Error("expected otto to NOT be busy for other/feature")
	}

	// No otto for this project/branch
	if m.isOttoBusy("nonexistent", "branch") {
		t.Error("expected otto to NOT be busy when it doesn't exist")
	}
}

// createTestDBWithAgents creates an in-memory database with agents table and optional agents
func createTestDBWithAgents(t *testing.T, agents []repo.Agent) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Create agents table schema
	agentsSchemaSQL := `
		CREATE TABLE IF NOT EXISTS agents (
			project TEXT NOT NULL,
			branch TEXT NOT NULL,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			task TEXT NOT NULL,
			status TEXT NOT NULL,
			session_id TEXT,
			pid INTEGER,
			compacted_at DATETIME,
			last_seen_message_id TEXT,
			peek_cursor TEXT,
			completed_at DATETIME,
			archived_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			id TEXT,
			PRIMARY KEY (project, branch, name)
		);
	`
	if _, err := db.Exec(agentsSchemaSQL); err != nil {
		t.Fatalf("failed to create agents schema: %v", err)
	}

	// Insert agents
	for _, agent := range agents {
		if err := repo.CreateAgent(db, agent); err != nil {
			t.Fatalf("failed to create agent %s: %v", agent.Name, err)
		}
	}

	return db
}

func TestGetChatSubmitAction(t *testing.T) {
	tests := []struct {
		name           string
		dbAgents       []repo.Agent // Agents in the database
		project        string
		branch         string
		expectedAction string // "none", "spawn", "prompt"
	}{
		{
			name:           "otto is busy - no action",
			dbAgents:       []repo.Agent{{Project: "p", Branch: "b", Name: "otto", Type: "codex", Task: "test", Status: "busy"}},
			project:        "p",
			branch:         "b",
			expectedAction: "none",
		},
		{
			name:           "otto complete - prompt",
			dbAgents:       []repo.Agent{{Project: "p", Branch: "b", Name: "otto", Type: "codex", Task: "test", Status: "complete"}},
			project:        "p",
			branch:         "b",
			expectedAction: "prompt",
		},
		{
			name:           "otto failed - prompt",
			dbAgents:       []repo.Agent{{Project: "p", Branch: "b", Name: "otto", Type: "codex", Task: "test", Status: "failed"}},
			project:        "p",
			branch:         "b",
			expectedAction: "prompt",
		},
		{
			name:           "no otto - spawn",
			dbAgents:       []repo.Agent{{Project: "p", Branch: "b", Name: "impl-1", Type: "codex", Task: "test", Status: "busy"}},
			project:        "p",
			branch:         "b",
			expectedAction: "spawn",
		},
		{
			name:           "no agents - spawn",
			dbAgents:       []repo.Agent{},
			project:        "p",
			branch:         "b",
			expectedAction: "spawn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := createTestDBWithAgents(t, tt.dbAgents)
			defer db.Close()

			m := NewModel(db)
			// Note: m.agents is intentionally left empty to simulate the race condition
			// where the TUI hasn't fetched agents yet. getChatSubmitAction should
			// query the DB directly, so this should still work correctly.

			action := m.getChatSubmitAction(tt.project, tt.branch)
			if action != tt.expectedAction {
				t.Errorf("expected action %q, got %q", tt.expectedAction, action)
			}
		})
	}
}

// TestGetChatSubmitActionRaceCondition verifies the fix for the race condition bug
// where the TUI's cached m.agents is empty but otto exists in the database.
// Before the fix, this would incorrectly return "spawn" and create "otto-2".
// After the fix, it correctly queries the DB and returns "prompt".
func TestGetChatSubmitActionRaceCondition(t *testing.T) {
	// Create database with an existing otto agent
	db := createTestDBWithAgents(t, []repo.Agent{
		{Project: "otto", Branch: "main", Name: "otto", Type: "codex", Task: "initial task", Status: "complete"},
	})
	defer db.Close()

	// Create model with the database but EMPTY m.agents slice
	// This simulates the race condition: user submits before fetchAgentsCmd completes
	m := NewModel(db)
	m.agents = []repo.Agent{} // Explicitly empty - simulates stale/uninitialized cache

	// Without the fix, this would check m.agents (empty), find no otto, and return "spawn"
	// With the fix, this queries the DB directly, finds otto, and returns "prompt"
	action := m.getChatSubmitAction("otto", "main")

	if action != "prompt" {
		t.Errorf("Race condition bug! Expected 'prompt' (otto exists in DB), got %q. "+
			"This would incorrectly spawn 'otto-2' instead of prompting existing 'otto'.", action)
	}
}

func TestHandleChatSubmitStoresChatMessage(t *testing.T) {
	// Create in-memory database with schema
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Manually create messages table schema
	schemaSQL := `
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			branch TEXT NOT NULL,
			from_agent TEXT NOT NULL,
			to_agent TEXT,
			type TEXT NOT NULL,
			content TEXT NOT NULL,
			mentions TEXT,
			requires_human BOOLEAN DEFAULT FALSE,
			read_by TEXT DEFAULT '[]',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			from_id TEXT
		);
	`
	if _, err := db.Exec(schemaSQL); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Create model with test db
	m := NewModel(db)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}
	// Inject no-op command runner to prevent fork bomb during tests
	m.runCommand = func(name string, args ...string) error { return nil }

	// Set activeChannelID to project header
	m.activeChannelID = "otto/main"

	// Set chat input value
	m.chatInput.SetValue("hello otto")

	// Call handleChatSubmit
	_ = m.handleChatSubmit()

	// Query messages from db
	messages, err := repo.ListMessages(db, repo.MessageFilter{
		Project: "otto",
		Branch:  "main",
	})
	if err != nil {
		t.Fatalf("failed to list messages: %v", err)
	}

	// Verify a chat message was stored
	if len(messages) != 1 {
		t.Fatalf("expected 1 message to be stored, got %d", len(messages))
	}

	msg := messages[0]
	if msg.Type != repo.MessageTypeChat {
		t.Errorf("expected message type %q, got %q", repo.MessageTypeChat, msg.Type)
	}
	if msg.FromAgent != "you" {
		t.Errorf("expected FromAgent to be 'you', got %q", msg.FromAgent)
	}
	if msg.ToAgent.Valid && msg.ToAgent.String != "otto" {
		t.Errorf("expected ToAgent to be 'otto', got %q", msg.ToAgent.String)
	}
	if msg.Content != "hello otto" {
		t.Errorf("expected Content to be 'hello otto', got %q", msg.Content)
	}
	if msg.Project != "otto" {
		t.Errorf("expected Project to be 'otto', got %q", msg.Project)
	}
	if msg.Branch != "main" {
		t.Errorf("expected Branch to be 'main', got %q", msg.Branch)
	}
}

func TestHandleChatSubmitReturnsImmediateFetchCommand(t *testing.T) {
	// Create in-memory database with schema
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Manually create messages table schema
	schemaSQL := `
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			branch TEXT NOT NULL,
			from_agent TEXT NOT NULL,
			to_agent TEXT,
			type TEXT NOT NULL,
			content TEXT NOT NULL,
			mentions TEXT,
			requires_human BOOLEAN DEFAULT FALSE,
			read_by TEXT DEFAULT '[]',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			from_id TEXT
		);
	`
	if _, err := db.Exec(schemaSQL); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Create model with test db
	m := NewModel(db)
	m.agents = []repo.Agent{
		{Project: "otto", Branch: "main", Name: "impl-1", Status: "busy"},
	}
	// Inject no-op command runner to prevent fork bomb during tests
	m.runCommand = func(name string, args ...string) error { return nil }

	// Set activeChannelID to project header
	m.activeChannelID = "otto/main"

	// Set chat input value
	m.chatInput.SetValue("test message")

	// Call handleChatSubmit - should return a command
	cmd := m.handleChatSubmit()

	// Verify that a command was returned (not nil)
	if cmd == nil {
		t.Fatal("expected handleChatSubmit to return a command for immediate message fetch, got nil")
	}

	// Execute the command to verify it's a fetchMessagesCmd
	// We can't inspect the command type directly, but we can execute it
	// and verify it returns messagesMsg
	msg := cmd()

	// Should return a messagesMsg (even if empty)
	switch msg.(type) {
	case messagesMsg:
		// Success - this is what we expect
	case error:
		// Also acceptable - might be an error from the fetch
	default:
		t.Errorf("expected command to return messagesMsg or error, got %T", msg)
	}
}

