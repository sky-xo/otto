package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Verify file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestOpenCreatesAgentsTable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Verify agents table exists by querying it
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM agents").Scan(&count)
	if err != nil {
		t.Errorf("agents table does not exist: %v", err)
	}
}

func TestCreateAndGetAgent(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	agent := Agent{
		Name:        "impl-1",
		ULID:        "019b825b-b138-7981-898d-2830d3610fc9",
		SessionFile: "/path/to/session.jsonl",
		PID:         12345,
	}

	err := db.CreateAgent(agent)
	if err != nil {
		t.Fatalf("CreateAgent failed: %v", err)
	}

	got, err := db.GetAgent("impl-1")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}

	if got.Name != agent.Name {
		t.Errorf("Name = %q, want %q", got.Name, agent.Name)
	}
	if got.ULID != agent.ULID {
		t.Errorf("ULID = %q, want %q", got.ULID, agent.ULID)
	}
	if got.SessionFile != agent.SessionFile {
		t.Errorf("SessionFile = %q, want %q", got.SessionFile, agent.SessionFile)
	}
	if got.PID != agent.PID {
		t.Errorf("PID = %d, want %d", got.PID, agent.PID)
	}
	if got.Cursor != 0 {
		t.Errorf("Cursor = %d, want 0", got.Cursor)
	}
}

func TestGetAgentNotFound(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	_, err := db.GetAgent("nonexistent")
	if err != ErrAgentNotFound {
		t.Errorf("err = %v, want ErrAgentNotFound", err)
	}
}

func TestUpdateCursor(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	agent := Agent{
		Name:        "impl-1",
		ULID:        "test-ulid",
		SessionFile: "/path/to/session.jsonl",
		PID:         12345,
	}
	db.CreateAgent(agent)

	err := db.UpdateCursor("impl-1", 42)
	if err != nil {
		t.Fatalf("UpdateCursor failed: %v", err)
	}

	got, _ := db.GetAgent("impl-1")
	if got.Cursor != 42 {
		t.Errorf("Cursor = %d, want 42", got.Cursor)
	}
}

func TestListAgents(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	db.CreateAgent(Agent{Name: "a", ULID: "ulid-a", SessionFile: "/a.jsonl", PID: 1})
	db.CreateAgent(Agent{Name: "b", ULID: "ulid-b", SessionFile: "/b.jsonl", PID: 2})

	agents, err := db.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	if len(agents) != 2 {
		t.Errorf("len(agents) = %d, want 2", len(agents))
	}
}

func openTestDB(t *testing.T) *DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	return db
}
