package tui

import (
	"database/sql"
	"testing"

	"june/internal/repo"
	"june/internal/scope"

	_ "modernc.org/sqlite"
)

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

func TestFetchAgentsFetchesAllProjects(t *testing.T) {
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

	// Insert agent in different scope - should ALSO be returned (global view)
	_, err = db.Exec(
		`INSERT INTO agents (project, branch, name, type, task, status) VALUES (?, ?, ?, ?, ?, ?)`,
		"other-project", "other-branch", "agent-2", "codex", "task 2", "busy",
	)
	if err != nil {
		t.Fatalf("failed to insert agent-2: %v", err)
	}

	// Call fetchAgentsCmd - it should return ALL agents across all projects
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

	// Verify we got agents from ALL projects (global view)
	if len(agentsMsg) != 2 {
		t.Errorf("expected 2 agents from all projects, got %d", len(agentsMsg))
	}

	// Verify both agents are present
	names := map[string]bool{}
	for _, a := range agentsMsg {
		names[a.Name] = true
	}
	if !names["agent-1"] || !names["agent-2"] {
		t.Errorf("expected both agent-1 and agent-2, got %v", names)
	}
}

func TestFetchAgentsIncludesArchivedAgents(t *testing.T) {
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

	// Insert active agent
	ctx := scope.CurrentContext()
	currentProject := ctx.Project
	currentBranch := ctx.Branch

	_, err = db.Exec(
		`INSERT INTO agents (project, branch, name, type, task, status) VALUES (?, ?, ?, ?, ?, ?)`,
		currentProject, currentBranch, "active-agent", "codex", "task 1", "busy",
	)
	if err != nil {
		t.Fatalf("failed to insert active agent: %v", err)
	}

	// Insert archived agent
	_, err = db.Exec(
		`INSERT INTO agents (project, branch, name, type, task, status, archived_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		currentProject, currentBranch, "archived-agent", "codex", "task 2", "completed", "2024-01-01 00:00:00",
	)
	if err != nil {
		t.Fatalf("failed to insert archived agent: %v", err)
	}

	// Call fetchAgentsCmd - it should return BOTH active and archived agents
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

	// Verify we got both active and archived agents
	if len(agentsMsg) != 2 {
		t.Errorf("expected 2 agents (1 active + 1 archived), got %d", len(agentsMsg))
	}

	// Verify both agents are present
	names := map[string]bool{}
	for _, a := range agentsMsg {
		names[a.Name] = true
	}
	if !names["active-agent"] || !names["archived-agent"] {
		t.Errorf("expected both active-agent and archived-agent, got %v", names)
	}

	// Verify the archived agent has archived_at set
	var archivedAgent repo.Agent
	for _, a := range agentsMsg {
		if a.Name == "archived-agent" {
			archivedAgent = a
			break
		}
	}
	if !archivedAgent.ArchivedAt.Valid {
		t.Errorf("expected archived-agent to have ArchivedAt set, but it was not set")
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

	// Call fetchMessagesCmd with explicit project/branch from current context
	cmd := fetchMessagesCmd(db, currentProject, currentBranch, "")
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

