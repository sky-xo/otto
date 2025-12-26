package tui

import (
	"database/sql"
	"testing"
	"time"

	"otto/internal/repo"
	"otto/internal/scope"

	_ "modernc.org/sqlite"
)

func TestNewModel(t *testing.T) {
	// Create in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create model
	m := NewModel(db)

	// Verify initial state
	if m.db == nil {
		t.Error("expected db to be set")
	}
	if len(m.messages) != 0 {
		t.Error("expected empty messages list")
	}
	if len(m.agents) != 0 {
		t.Error("expected empty agents list")
	}
	if m.activeChannelID != mainChannelID {
		t.Errorf("expected activeChannelID to be %q", mainChannelID)
	}
	if len(m.transcripts) != 0 {
		t.Error("expected empty transcripts map")
	}
}

func TestModelView(t *testing.T) {
	// Create in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create model with size
	m := NewModel(db)
	m.width = 80
	m.height = 24

	// Should render without panic
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestChannelOrdering(t *testing.T) {
	agents := []repo.Agent{
		{Name: "agent-3", Status: "failed"},
		{Name: "agent-2", Status: "blocked"},
		{Name: "agent-1", Status: "busy"},
		{Name: "agent-4", Status: "complete"},
	}

	ordered := sortAgentsByStatus(agents)
	if len(ordered) != 4 {
		t.Fatalf("expected 4 agents, got %d", len(ordered))
	}

	expected := []string{"agent-1", "agent-2", "agent-4", "agent-3"}
	for i, id := range expected {
		if ordered[i].Name != id {
			t.Fatalf("expected %q at index %d, got %q", id, i, ordered[i].Name)
		}
	}
}

func TestChannelsIncludeMainFirst(t *testing.T) {
	m := NewModel(nil)
	m.agents = []repo.Agent{
		{Name: "agent-2", Status: "complete"},
		{Name: "agent-1", Status: "busy"},
	}

	channels := m.channels()
	if len(channels) != 3 {
		t.Fatalf("expected 3 channels, got %d", len(channels))
	}
	if channels[0].ID != mainChannelID {
		t.Fatalf("expected main channel first, got %q", channels[0].ID)
	}
	if channels[1].ID != "agent-1" || channels[2].ID != "agent-2" {
		t.Fatalf("unexpected channel order: %q, %q", channels[1].ID, channels[2].ID)
	}
}

func TestArchivedAgentsHiddenByDefault(t *testing.T) {
	m := NewModel(nil)
	archivedAt := time.Now().Add(-time.Hour)
	m.agents = []repo.Agent{
		{Name: "agent-1", Status: "busy"},
		{
			Name:       "agent-2",
			Status:     "complete",
			ArchivedAt: sql.NullTime{Time: archivedAt, Valid: true},
		},
	}

	channels := m.channels()
	if len(channels) != 3 {
		t.Fatalf("expected 3 channels, got %d", len(channels))
	}
	if channels[0].ID != mainChannelID {
		t.Fatalf("expected main channel first, got %q", channels[0].ID)
	}
	if channels[1].ID != "agent-1" {
		t.Fatalf("expected active agent second, got %q", channels[1].ID)
	}
	if channels[2].ID != archivedChannelID {
		t.Fatalf("expected archived header last, got %q", channels[2].ID)
	}
}

func TestArchivedAgentsAppearWhenExpanded(t *testing.T) {
	m := NewModel(nil)
	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)
	m.agents = []repo.Agent{
		{Name: "agent-1", Status: "busy"},
		{
			Name:       "agent-2",
			Status:     "complete",
			ArchivedAt: sql.NullTime{Time: older, Valid: true},
		},
		{
			Name:       "agent-3",
			Status:     "failed",
			ArchivedAt: sql.NullTime{Time: newer, Valid: true},
		},
	}
	m.archivedExpanded = true

	channels := m.channels()
	if len(channels) != 5 {
		t.Fatalf("expected 5 channels, got %d", len(channels))
	}
	if channels[2].ID != archivedChannelID {
		t.Fatalf("expected archived header at index 2, got %q", channels[2].ID)
	}
	if channels[2].Name != "Archived (2)" {
		t.Fatalf("expected archived header label, got %q", channels[2].Name)
	}
	if channels[3].ID != "agent-3" || channels[4].ID != "agent-2" {
		t.Fatalf("unexpected archived order: %q, %q", channels[3].ID, channels[4].ID)
	}
}

func TestArchivedEnterTogglesExpanded(t *testing.T) {
	m := NewModel(nil)
	archivedAt := time.Now().Add(-time.Hour)
	m.agents = []repo.Agent{
		{Name: "agent-1", Status: "busy"},
		{
			Name:       "agent-2",
			Status:     "complete",
			ArchivedAt: sql.NullTime{Time: archivedAt, Valid: true},
		},
	}

	channels := m.channels()
	headerIndex := -1
	for i, ch := range channels {
		if ch.ID == archivedChannelID {
			headerIndex = i
			break
		}
	}
	if headerIndex == -1 {
		t.Fatal("expected archived header to exist")
	}

	m.cursorIndex = headerIndex
	_ = m.activateSelection()
	if !m.archivedExpanded {
		t.Fatal("expected archived section to expand on enter")
	}
	if m.activeChannelID != mainChannelID {
		t.Fatalf("expected active channel to remain main, got %q", m.activeChannelID)
	}

	_ = m.activateSelection()
	if m.archivedExpanded {
		t.Fatal("expected archived section to collapse on enter")
	}
}

func TestFetchTranscriptsUsesCurrentScope(t *testing.T) {
	// Create in-memory database with schema
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Initialize schema - we need to use the db package's Open function
	// to ensure schema is created properly
	db.Close()
	db, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to reopen database: %v", err)
	}

	// Manually create schema since we can't use db.Open with :memory: cleanly
	schemaSQL := `
		CREATE TABLE IF NOT EXISTS agents (
			project TEXT NOT NULL,
			branch TEXT NOT NULL,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			task TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (project, branch, name)
		);
		CREATE TABLE IF NOT EXISTS logs (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			branch TEXT NOT NULL,
			agent_name TEXT NOT NULL,
			agent_type TEXT NOT NULL,
			event_type TEXT NOT NULL,
			tool_name TEXT,
			content TEXT,
			raw_json TEXT,
			command TEXT,
			exit_code INTEGER,
			status TEXT,
			tool_use_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := db.Exec(schemaSQL); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Insert an agent in the current project/branch scope
	// Use scope.CurrentContext() to get actual values for the test environment
	ctx := scope.CurrentContext()
	currentProject := ctx.Project
	currentBranch := ctx.Branch
	agentName := "agent-1"

	_, err = db.Exec(
		`INSERT INTO agents (project, branch, name, type, task, status) VALUES (?, ?, ?, ?, ?, ?)`,
		currentProject, currentBranch, agentName, "codex", "test task", "busy",
	)
	if err != nil {
		t.Fatalf("failed to insert agent: %v", err)
	}

	// Insert logs for this agent in the same scope
	_, err = db.Exec(
		`INSERT INTO logs (id, project, branch, agent_name, agent_type, event_type, content) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"log-1", currentProject, currentBranch, agentName, "codex", "message", "test log entry",
	)
	if err != nil {
		t.Fatalf("failed to insert log: %v", err)
	}

	// Insert logs for a different project/branch - should NOT be returned
	_, err = db.Exec(
		`INSERT INTO logs (id, project, branch, agent_name, agent_type, event_type, content) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"log-other", "other-project", "other-branch", agentName, "codex", "message", "other log entry",
	)
	if err != nil {
		t.Fatalf("failed to insert other log: %v", err)
	}

	// Call fetchTranscriptsCmd - it should use scope.CurrentContext()
	// and only return logs matching the current project/branch
	cmd := fetchTranscriptsCmd(db, agentName, "")
	msg := cmd()

	// Verify we got the correct logs
	transcriptsMsg, ok := msg.(transcriptsMsg)
	if !ok {
		if err, ok := msg.(error); ok {
			t.Fatalf("fetchTranscriptsCmd returned error: %v", err)
		}
		t.Fatalf("expected transcriptsMsg, got %T", msg)
	}

	if transcriptsMsg.agentID != agentName {
		t.Errorf("expected agentID %q, got %q", agentName, transcriptsMsg.agentID)
	}

	// Verify we got at least one log entry from current scope
	if len(transcriptsMsg.entries) == 0 {
		t.Fatalf("expected at least 1 log entry, got 0 - fetchTranscriptsCmd is not using current scope")
	}

	// Verify we got the right log (not the one from other project/branch)
	if len(transcriptsMsg.entries) != 1 {
		t.Errorf("expected 1 log entry, got %d", len(transcriptsMsg.entries))
	}

	if transcriptsMsg.entries[0].ID != "log-1" {
		t.Errorf("expected log ID %q, got %q", "log-1", transcriptsMsg.entries[0].ID)
	}
	if transcriptsMsg.entries[0].Content.String != "test log entry" {
		t.Errorf("expected content %q, got %q", "test log entry", transcriptsMsg.entries[0].Content.String)
	}
}

func TestFetchAgentsUsesCurrentScope(t *testing.T) {
	// Create in-memory database with schema
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Manually create schema
	schemaSQL := `
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
			PRIMARY KEY (project, branch, name)
		);
	`
	if _, err := db.Exec(schemaSQL); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Insert agents in the current project/branch scope
	// Use scope.CurrentContext() to get actual values for the test environment
	ctx := scope.CurrentContext()
	currentProject := ctx.Project
	currentBranch := ctx.Branch

	_, err = db.Exec(
		`INSERT INTO agents (project, branch, name, type, task, status) VALUES (?, ?, ?, ?, ?, ?)`,
		currentProject, currentBranch, "agent-1", "codex", "task 1", "busy",
	)
	if err != nil {
		t.Fatalf("failed to insert agent-1: %v", err)
	}

	// Insert agent in different scope - should NOT be returned
	_, err = db.Exec(
		`INSERT INTO agents (project, branch, name, type, task, status) VALUES (?, ?, ?, ?, ?, ?)`,
		"other-project", "other-branch", "agent-2", "codex", "task 2", "busy",
	)
	if err != nil {
		t.Fatalf("failed to insert agent-2: %v", err)
	}

	// Call fetchAgentsCmd - it should use scope.CurrentContext()
	cmd := fetchAgentsCmd(db)
	msg := cmd()

	// Verify we got the correct agents
	agentsMsg, ok := msg.(agentsMsg)
	if !ok {
		if err, ok := msg.(error); ok {
			t.Fatalf("fetchAgentsCmd returned error: %v", err)
		}
		t.Fatalf("expected agentsMsg, got %T", msg)
	}

	// Verify we got at least one agent from current scope
	if len(agentsMsg) == 0 {
		t.Fatalf("expected at least 1 agent, got 0 - fetchAgentsCmd is not using current scope")
	}

	// Verify we only got agents from the current scope
	if len(agentsMsg) != 1 {
		t.Errorf("expected 1 agent from current scope, got %d", len(agentsMsg))
	}

	if agentsMsg[0].Name != "agent-1" {
		t.Errorf("expected agent name %q, got %q", "agent-1", agentsMsg[0].Name)
	}
}

func TestFetchMessagesUsesCurrentScope(t *testing.T) {
	// Create in-memory database with schema
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Manually create schema
	schemaSQL := `
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			branch TEXT NOT NULL,
			from_agent TEXT,
			to_agent TEXT,
			type TEXT NOT NULL,
			content TEXT,
			mentions TEXT,
			requires_human INTEGER DEFAULT 0,
			read_by TEXT,
			from_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := db.Exec(schemaSQL); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Insert messages in the current project/branch scope
	// Use scope.CurrentContext() to get actual values for the test environment
	ctx := scope.CurrentContext()
	currentProject := ctx.Project
	currentBranch := ctx.Branch

	_, err = db.Exec(
		`INSERT INTO messages (id, project, branch, from_agent, type, content, mentions, read_by, from_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"msg-1", currentProject, currentBranch, "agent-1", "say", "message in current scope", "", "", "agent-1",
	)
	if err != nil {
		t.Fatalf("failed to insert msg-1: %v", err)
	}

	// Insert message in different scope - should NOT be returned
	_, err = db.Exec(
		`INSERT INTO messages (id, project, branch, from_agent, type, content, mentions, read_by, from_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"msg-2", "other-project", "other-branch", "agent-2", "say", "message in other scope", "", "", "agent-2",
	)
	if err != nil {
		t.Fatalf("failed to insert msg-2: %v", err)
	}

	// Call fetchMessagesCmd - it should use scope.CurrentContext()
	cmd := fetchMessagesCmd(db, "")
	msg := cmd()

	// Verify we got the correct messages
	messagesMsg, ok := msg.(messagesMsg)
	if !ok {
		if err, ok := msg.(error); ok {
			t.Fatalf("fetchMessagesCmd returned error: %v", err)
		}
		t.Fatalf("expected messagesMsg, got %T", msg)
	}

	// Verify we got at least one message from current scope
	if len(messagesMsg) == 0 {
		t.Fatalf("expected at least 1 message, got 0 - fetchMessagesCmd is not using current scope")
	}

	// Verify we only got messages from the current scope
	if len(messagesMsg) != 1 {
		t.Errorf("expected 1 message from current scope, got %d", len(messagesMsg))
	}

	if messagesMsg[0].ID != "msg-1" {
		t.Errorf("expected message ID %q, got %q", "msg-1", messagesMsg[0].ID)
	}
	if messagesMsg[0].Content != "message in current scope" {
		t.Errorf("expected content %q, got %q", "message in current scope", messagesMsg[0].Content)
	}
}
