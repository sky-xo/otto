package db

import (
	"database/sql"
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
	if err := db.CreateAgent(agent); err != nil {
		t.Fatalf("CreateAgent failed: %v", err)
	}

	err := db.UpdateCursor("impl-1", 42)
	if err != nil {
		t.Fatalf("UpdateCursor failed: %v", err)
	}

	got, err := db.GetAgent("impl-1")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if got.Cursor != 42 {
		t.Errorf("Cursor = %d, want 42", got.Cursor)
	}
}

func TestListAgents(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := db.CreateAgent(Agent{Name: "a", ULID: "ulid-a", SessionFile: "/a.jsonl", PID: 1}); err != nil {
		t.Fatalf("CreateAgent failed: %v", err)
	}
	if err := db.CreateAgent(Agent{Name: "b", ULID: "ulid-b", SessionFile: "/b.jsonl", PID: 2}); err != nil {
		t.Fatalf("CreateAgent failed: %v", err)
	}

	agents, err := db.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	if len(agents) != 2 {
		t.Errorf("len(agents) = %d, want 2", len(agents))
	}
}

func TestUpdateSessionFile(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	agent := Agent{
		Name:        "impl-1",
		ULID:        "test-ulid",
		SessionFile: "",
		PID:         12345,
	}
	if err := db.CreateAgent(agent); err != nil {
		t.Fatalf("CreateAgent failed: %v", err)
	}

	err := db.UpdateSessionFile("impl-1", "/path/to/session.jsonl")
	if err != nil {
		t.Fatalf("UpdateSessionFile failed: %v", err)
	}

	got, err := db.GetAgent("impl-1")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if got.SessionFile != "/path/to/session.jsonl" {
		t.Errorf("SessionFile = %q, want %q", got.SessionFile, "/path/to/session.jsonl")
	}
}

func TestUpdateSessionFileNotFound(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	err := db.UpdateSessionFile("nonexistent", "/path/to/session.jsonl")
	if err != ErrAgentNotFound {
		t.Errorf("err = %v, want ErrAgentNotFound", err)
	}
}

func TestCreateAgent_WithGitContext(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	agent := Agent{
		Name:        "test-agent",
		ULID:        "01234567890",
		SessionFile: "/tmp/session.jsonl",
		PID:         1234,
		RepoPath:    "/Users/test/code/myproject",
		Branch:      "main",
	}

	if err := db.CreateAgent(agent); err != nil {
		t.Fatalf("CreateAgent failed: %v", err)
	}

	got, err := db.GetAgent("test-agent")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}

	if got.RepoPath != agent.RepoPath {
		t.Errorf("RepoPath = %q, want %q", got.RepoPath, agent.RepoPath)
	}
	if got.Branch != agent.Branch {
		t.Errorf("Branch = %q, want %q", got.Branch, agent.Branch)
	}
}

func TestMigration_AddsNewColumns(t *testing.T) {
	// Create a DB with the OLD schema (no repo_path, branch)
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Manually create old schema
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = rawDB.Exec(`
		CREATE TABLE agents (
			name TEXT PRIMARY KEY,
			ulid TEXT NOT NULL,
			session_file TEXT NOT NULL,
			cursor INTEGER DEFAULT 0,
			pid INTEGER,
			spawned_at TEXT NOT NULL
		);
		INSERT INTO agents (name, ulid, session_file, pid, spawned_at)
		VALUES ('old-agent', 'ulid123', '/tmp/session.jsonl', 0, '2025-01-01T00:00:00Z');
	`)
	if err != nil {
		t.Fatal(err)
	}
	rawDB.Close()

	// Now open with our Open() which should migrate
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Old agent should still be readable with empty repo_path/branch
	agent, err := db.GetAgent("old-agent")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if agent.RepoPath != "" {
		t.Errorf("expected empty RepoPath for migrated agent, got %q", agent.RepoPath)
	}
	if agent.Branch != "" {
		t.Errorf("expected empty Branch for migrated agent, got %q", agent.Branch)
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
