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
