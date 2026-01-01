package repo

import (
	"database/sql"
	"path/filepath"
	"testing"

	"june/internal/db"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "june.db")
	conn, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return conn
}

func TestAgentsCRUD(t *testing.T) {
	db := openTestDB(t)

	err := CreateAgent(db, Agent{Project: "june", Branch: "main", Name: "authbackend", Type: "claude", Task: "design", Status: "busy"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := UpdateAgentStatus(db, "june", "main", "authbackend", "done"); err != nil {
		t.Fatalf("update: %v", err)
	}

	agents, err := ListAgents(db, AgentFilter{Project: "june", Branch: "main"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(agents) != 1 || agents[0].Status != "done" {
		t.Fatalf("unexpected agents: %#v", agents)
	}

	if _, err := GetAgent(db, "june", "main", "authbackend"); err != nil {
		t.Fatalf("get: %v", err)
	}
}

func TestUpdateAgentSessionID(t *testing.T) {
	db := openTestDB(t)

	// Create agent with initial session ID
	initialSessionID := "uuid-1234"
	err := CreateAgent(db, Agent{
		Project:   "june",
		Branch:    "main",
		Name:      "codexagent",
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
	if err := UpdateAgentSessionID(db, "june", "main", "codexagent", realThreadID); err != nil {
		t.Fatalf("update session_id: %v", err)
	}

	// Verify the session ID was updated
	agent, err := GetAgent(db, "june", "main", "codexagent")
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

func TestAgentLastSeenMsgID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	agent := Agent{
		Project: "june",
		Branch:  "main",
		Name:    "test-agent",
		Type:    "claude",
		Task:    "test task",
		Status:  "busy",
	}
	if err := CreateAgent(db, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	// Update last seen message ID
	if err := UpdateAgentLastSeenMsgID(db, "june", "main", "test-agent", "msg-123"); err != nil {
		t.Fatalf("update last seen: %v", err)
	}

	// Verify it was saved
	got, err := GetAgent(db, "june", "main", "test-agent")
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if !got.LastSeenMsgID.Valid || got.LastSeenMsgID.String != "msg-123" {
		t.Errorf("expected LastSeenMsgID='msg-123', got %v", got.LastSeenMsgID)
	}
}

func TestDeleteAgent(t *testing.T) {
	conn := openTestDB(t)
	defer conn.Close()

	agent := Agent{Project: "june", Branch: "main", Name: "test", Type: "claude", Task: "test task", Status: "busy"}
	if err := CreateAgent(conn, agent); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := DeleteAgent(conn, "june", "main", "test"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := GetAgent(conn, "june", "main", "test")
	if err != sql.ErrNoRows {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
}

func TestAgentCompletionLifecycle(t *testing.T) {
	conn := openTestDB(t)
	defer conn.Close()

	agent := Agent{Project: "june", Branch: "main", Name: "complete-me", Type: "claude", Task: "test task", Status: "busy"}
	if err := CreateAgent(conn, agent); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := SetAgentComplete(conn, agent.Project, agent.Branch, agent.Name); err != nil {
		t.Fatalf("set complete: %v", err)
	}

	updated, err := GetAgent(conn, agent.Project, agent.Branch, agent.Name)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if updated.Status != "complete" {
		t.Fatalf("expected status complete, got %q", updated.Status)
	}
	if !updated.CompletedAt.Valid {
		t.Fatalf("expected completed_at to be set")
	}

	if err := ResumeAgent(conn, agent.Project, agent.Branch, agent.Name); err != nil {
		t.Fatalf("resume: %v", err)
	}

	resumed, err := GetAgent(conn, agent.Project, agent.Branch, agent.Name)
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

	agent := Agent{Project: "june", Branch: "main", Name: "fail-me", Type: "claude", Task: "test task", Status: "busy"}
	if err := CreateAgent(conn, agent); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := SetAgentFailed(conn, agent.Project, agent.Branch, agent.Name); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	updated, err := GetAgent(conn, agent.Project, agent.Branch, agent.Name)
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

func TestArchiveAgentSetsArchivedAt(t *testing.T) {
	conn := openTestDB(t)
	defer conn.Close()

	err := CreateAgent(conn, Agent{Project: "june", Branch: "main", Name: "arch-me", Type: "claude", Task: "task", Status: "complete"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := ArchiveAgent(conn, "june", "main", "arch-me"); err != nil {
		t.Fatalf("archive: %v", err)
	}

	got, err := GetAgent(conn, "june", "main", "arch-me")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.ArchivedAt.Valid {
		t.Fatalf("expected archived_at to be set")
	}
}

func TestListAgentsFilteredByProjectBranch(t *testing.T) {
	db := openTestDB(t)
	_ = CreateAgent(db, Agent{Project: "alpha", Branch: "main", Name: "a1", Type: "codex", Task: "t", Status: "busy"})
	_ = CreateAgent(db, Agent{Project: "beta", Branch: "main", Name: "a1", Type: "codex", Task: "t", Status: "busy"})
	_ = CreateAgent(db, Agent{Project: "alpha", Branch: "dev", Name: "a1", Type: "codex", Task: "t", Status: "busy"})

	agents, err := ListAgents(db, AgentFilter{Project: "alpha", Branch: "main"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Project != "alpha" || agents[0].Branch != "main" {
		t.Fatalf("unexpected agent: %#v", agents[0])
	}
}

func TestMarkAgentCompacted(t *testing.T) {
	conn := openTestDB(t)
	defer conn.Close()

	agent := Agent{Project: "june", Branch: "main", Name: "compactor", Type: "codex", Task: "test task", Status: "busy"}
	if err := CreateAgent(conn, agent); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Mark agent as compacted
	if err := MarkAgentCompacted(conn, agent.Project, agent.Branch, agent.Name); err != nil {
		t.Fatalf("mark compacted: %v", err)
	}

	// Verify compacted_at is set
	updated, err := GetAgent(conn, agent.Project, agent.Branch, agent.Name)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !updated.CompactedAt.Valid {
		t.Fatalf("expected compacted_at to be set")
	}
}
