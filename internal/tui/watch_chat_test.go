package tui

import (
	"database/sql"
	"testing"

	"june/internal/repo"

	_ "modernc.org/sqlite"
)

func TestChatInputAppearsForProjectHeader(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "june", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	// Select project header
	m.activeChannelID = "june/main"

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

func TestGetJuneAgentForProjectBranch(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "june", Branch: "main", Name: "june", Status: "complete"},
		{Project: "other", Branch: "feature", Name: "june", Status: "busy"},
		{Project: "june", Branch: "main", Name: "impl-1", Status: "busy"},
	}

	// Get june for june/main
	june := m.getJuneAgent("june", "main")
	if june == nil {
		t.Fatal("expected to find june agent for june/main")
	}
	if june.Name != "june" || june.Project != "june" || june.Branch != "main" {
		t.Errorf("got wrong agent: %+v", june)
	}

	// Get june for other/feature
	june = m.getJuneAgent("other", "feature")
	if june == nil {
		t.Fatal("expected to find june agent for other/feature")
	}
	if june.Status != "busy" {
		t.Errorf("expected busy june, got %q", june.Status)
	}

	// Get june for non-existent project/branch
	june = m.getJuneAgent("nonexistent", "branch")
	if june != nil {
		t.Error("expected nil for non-existent project/branch")
	}
}

func TestIsJuneBusy(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Project: "june", Branch: "main", Name: "june", Status: "busy"},
		{Project: "other", Branch: "feature", Name: "june", Status: "complete"},
	}

	// june/main has busy june
	if !m.isJuneBusy("june", "main") {
		t.Error("expected june to be busy for june/main")
	}

	// other/feature has complete june
	if m.isJuneBusy("other", "feature") {
		t.Error("expected june to NOT be busy for other/feature")
	}

	// No june for this project/branch
	if m.isJuneBusy("nonexistent", "branch") {
		t.Error("expected june to NOT be busy when it doesn't exist")
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
			name:           "june is busy - no action",
			dbAgents:       []repo.Agent{{Project: "p", Branch: "b", Name: "june", Type: "codex", Task: "test", Status: "busy"}},
			project:        "p",
			branch:         "b",
			expectedAction: "none",
		},
		{
			name:           "june complete - prompt",
			dbAgents:       []repo.Agent{{Project: "p", Branch: "b", Name: "june", Type: "codex", Task: "test", Status: "complete"}},
			project:        "p",
			branch:         "b",
			expectedAction: "prompt",
		},
		{
			name:           "june failed - prompt",
			dbAgents:       []repo.Agent{{Project: "p", Branch: "b", Name: "june", Type: "codex", Task: "test", Status: "failed"}},
			project:        "p",
			branch:         "b",
			expectedAction: "prompt",
		},
		{
			name:           "no june - spawn",
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
// where the TUI's cached m.agents is empty but june exists in the database.
// Before the fix, this would incorrectly return "spawn" and create "june-2".
// After the fix, it correctly queries the DB and returns "prompt".
func TestGetChatSubmitActionRaceCondition(t *testing.T) {
	// Create database with an existing june agent
	db := createTestDBWithAgents(t, []repo.Agent{
		{Project: "june", Branch: "main", Name: "june", Type: "codex", Task: "initial task", Status: "complete"},
	})
	defer db.Close()

	// Create model with the database but EMPTY m.agents slice
	// This simulates the race condition: user submits before fetchAgentsCmd completes
	m := NewModel(db)
	m.agents = []repo.Agent{} // Explicitly empty - simulates stale/uninitialized cache

	// Without the fix, this would check m.agents (empty), find no june, and return "spawn"
	// With the fix, this queries the DB directly, finds june, and returns "prompt"
	action := m.getChatSubmitAction("june", "main")

	if action != "prompt" {
		t.Errorf("Race condition bug! Expected 'prompt' (june exists in DB), got %q. "+
			"This would incorrectly spawn 'june-2' instead of prompting existing 'june'.", action)
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
		{Project: "june", Branch: "main", Name: "impl-1", Status: "busy"},
	}
	// Inject no-op command runner to prevent fork bomb during tests
	m.runCommand = func(name string, args ...string) error { return nil }

	// Set activeChannelID to project header
	m.activeChannelID = "june/main"

	// Set chat input value
	m.chatInput.SetValue("hello june")

	// Call handleChatSubmit
	_ = m.handleChatSubmit()

	// Query messages from db
	messages, err := repo.ListMessages(db, repo.MessageFilter{
		Project: "june",
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
	if msg.ToAgent.Valid && msg.ToAgent.String != "june" {
		t.Errorf("expected ToAgent to be 'june', got %q", msg.ToAgent.String)
	}
	if msg.Content != "hello june" {
		t.Errorf("expected Content to be 'hello june', got %q", msg.Content)
	}
	if msg.Project != "june" {
		t.Errorf("expected Project to be 'june', got %q", msg.Project)
	}
	if msg.Branch != "main" {
		t.Errorf("expected Branch to be 'main', got %q", msg.Branch)
	}
}

func TestHandleChatSubmitOptimisticUIUpdate(t *testing.T) {
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
		{Project: "june", Branch: "main", Name: "impl-1", Status: "busy"},
	}
	// Inject no-op command runner to prevent fork bomb during tests
	m.runCommand = func(name string, args ...string) error { return nil }

	// Set activeChannelID to project header
	m.activeChannelID = "june/main"

	// Verify initial state
	if len(m.messages) != 0 {
		t.Fatalf("expected 0 initial messages, got %d", len(m.messages))
	}
	if m.lastMessageID != "" {
		t.Fatalf("expected empty lastMessageID, got %q", m.lastMessageID)
	}

	// Set chat input value
	m.chatInput.SetValue("test message")

	// Call handleChatSubmit - now uses optimistic UI so returns nil
	cmd := m.handleChatSubmit()

	// Should return nil (no fetch command needed since message is added optimistically)
	if cmd != nil {
		t.Errorf("expected handleChatSubmit to return nil (optimistic UI), got a command")
	}

	// Verify optimistic UI update - message should be in local state
	if len(m.messages) != 1 {
		t.Fatalf("expected 1 message after optimistic update, got %d", len(m.messages))
	}

	msg := m.messages[0]
	if msg.FromAgent != "you" {
		t.Errorf("expected FromAgent to be 'you', got %q", msg.FromAgent)
	}
	if msg.Content != "test message" {
		t.Errorf("expected Content to be 'test message', got %q", msg.Content)
	}
	if msg.Project != "june" {
		t.Errorf("expected Project to be 'june', got %q", msg.Project)
	}
	if msg.Branch != "main" {
		t.Errorf("expected Branch to be 'main', got %q", msg.Branch)
	}

	// Verify lastMessageID was updated
	if m.lastMessageID != msg.ID {
		t.Errorf("expected lastMessageID to be %q, got %q", msg.ID, m.lastMessageID)
	}
}

