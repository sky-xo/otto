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

	err := CreateAgent(db, Agent{ID: "authbackend", Type: "claude", Task: "design", Status: "busy"})
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

func TestDeleteAgent(t *testing.T) {
	conn := openTestDB(t)
	defer conn.Close()

	agent := Agent{ID: "test", Type: "claude", Task: "test task", Status: "busy"}
	if err := CreateAgent(conn, agent); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := DeleteAgent(conn, "test"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := GetAgent(conn, "test")
	if err != sql.ErrNoRows {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
}

func TestAgentCompletionLifecycle(t *testing.T) {
	conn := openTestDB(t)
	defer conn.Close()

	agent := Agent{ID: "complete-me", Type: "claude", Task: "test task", Status: "busy"}
	if err := CreateAgent(conn, agent); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := SetAgentComplete(conn, agent.ID); err != nil {
		t.Fatalf("set complete: %v", err)
	}

	updated, err := GetAgent(conn, agent.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if updated.Status != "complete" {
		t.Fatalf("expected status complete, got %q", updated.Status)
	}
	if !updated.CompletedAt.Valid {
		t.Fatalf("expected completed_at to be set")
	}

	if err := ResumeAgent(conn, agent.ID); err != nil {
		t.Fatalf("resume: %v", err)
	}

	resumed, err := GetAgent(conn, agent.ID)
	if err != nil {
		t.Fatalf("get resumed: %v", err)
	}
	if resumed.Status != "busy" {
		t.Fatalf("expected status busy, got %q", resumed.Status)
	}
	if resumed.CompletedAt.Valid {
		t.Fatalf("expected completed_at to be cleared")
	}
}

func TestAgentFailure(t *testing.T) {
	conn := openTestDB(t)
	defer conn.Close()

	agent := Agent{ID: "fail-me", Type: "claude", Task: "test task", Status: "busy"}
	if err := CreateAgent(conn, agent); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := SetAgentFailed(conn, agent.ID); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	updated, err := GetAgent(conn, agent.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if updated.Status != "failed" {
		t.Fatalf("expected status failed, got %q", updated.Status)
	}
	if !updated.CompletedAt.Valid {
		t.Fatalf("expected completed_at to be set")
	}
}
