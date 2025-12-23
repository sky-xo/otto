package repo

import (
	"database/sql"
	"path/filepath"
	"testing"

	"otto/internal/db"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "otto.db")
	conn, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return conn
}

func TestAgentsCRUD(t *testing.T) {
	db := openTestDB(t)

	err := CreateAgent(db, Agent{ID: "authbackend", Type: "claude", Task: "design", Status: "working"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := UpdateAgentStatus(db, "authbackend", "done"); err != nil {
		t.Fatalf("update: %v", err)
	}

	agents, err := ListAgents(db)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(agents) != 1 || agents[0].Status != "done" {
		t.Fatalf("unexpected agents: %#v", agents)
	}

	if _, err := GetAgent(db, "authbackend"); err != nil {
		t.Fatalf("get: %v", err)
	}
}

func TestUpdateAgentSessionID(t *testing.T) {
	db := openTestDB(t)

	// Create agent with initial session ID
	initialSessionID := "uuid-1234"
	err := CreateAgent(db, Agent{
		ID:        "codexagent",
		Type:      "codex",
		Task:      "test task",
		Status:    "working",
		SessionID: sql.NullString{String: initialSessionID, Valid: true},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Update session ID to the real thread_id
	realThreadID := "thread_abc123xyz"
	if err := UpdateAgentSessionID(db, "codexagent", realThreadID); err != nil {
		t.Fatalf("update session_id: %v", err)
	}

	// Verify the session ID was updated
	agent, err := GetAgent(db, "codexagent")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if !agent.SessionID.Valid {
		t.Fatal("session_id should be valid")
	}

	if agent.SessionID.String != realThreadID {
		t.Fatalf("expected session_id %q, got %q", realThreadID, agent.SessionID.String)
	}
}
