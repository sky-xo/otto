package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sky-xo/june/internal/agent"
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

func TestTasksTableExists(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Query pragma to verify table structure
	// Note: DB embeds *sql.DB, so methods are promoted directly
	rows, err := db.Query("PRAGMA table_info(tasks)")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	columns := make(map[string]string)
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dfltValue any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		columns[name] = typ
	}

	expected := map[string]string{
		"id":         "TEXT",
		"parent_id":  "TEXT",
		"title":      "TEXT",
		"status":     "TEXT",
		"notes":      "TEXT",
		"created_at": "TEXT",
		"updated_at": "TEXT",
		"deleted_at": "TEXT",
		"repo_path":  "TEXT",
		"branch":     "TEXT",
	}

	for col, typ := range expected {
		if columns[col] != typ {
			t.Errorf("Column %s: got type %q, want %q", col, columns[col], typ)
		}
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

func TestListAgentsByRepo(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Create agents in different repos
	if err := db.CreateAgent(Agent{Name: "a1", ULID: "1", SessionFile: "/tmp/1.jsonl", RepoPath: "/code/project", Branch: "main"}); err != nil {
		t.Fatalf("CreateAgent failed: %v", err)
	}
	if err := db.CreateAgent(Agent{Name: "a2", ULID: "2", SessionFile: "/tmp/2.jsonl", RepoPath: "/code/project", Branch: "feature"}); err != nil {
		t.Fatalf("CreateAgent failed: %v", err)
	}
	if err := db.CreateAgent(Agent{Name: "a3", ULID: "3", SessionFile: "/tmp/3.jsonl", RepoPath: "/code/other", Branch: "main"}); err != nil {
		t.Fatalf("CreateAgent failed: %v", err)
	}

	agents, err := db.ListAgentsByRepo("/code/project")
	if err != nil {
		t.Fatalf("ListAgentsByRepo failed: %v", err)
	}

	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}

	// Verify the returned agents are the right ones
	names := make(map[string]bool)
	for _, a := range agents {
		names[a.Name] = true
	}
	if !names["a1"] || !names["a2"] {
		t.Errorf("expected agents a1 and a2, got %v", agents)
	}
}

func TestAgent_ToUnified(t *testing.T) {
	dbAgent := Agent{
		Name:        "my-agent",
		ULID:        "ulid123",
		SessionFile: "/path/to/session.jsonl",
		Cursor:      100,
		PID:         1234,
		SpawnedAt:   time.Now(),
		RepoPath:    "/Users/test/code/project",
		Branch:      "feature",
	}

	unified := dbAgent.ToUnified()

	if unified.ID != dbAgent.ULID {
		t.Errorf("ID = %q, want %q", unified.ID, dbAgent.ULID)
	}
	if unified.Name != dbAgent.Name {
		t.Errorf("Name = %q, want %q", unified.Name, dbAgent.Name)
	}
	if unified.Source != agent.SourceCodex {
		t.Errorf("Source = %q, want %q", unified.Source, agent.SourceCodex)
	}
	if unified.TranscriptPath != dbAgent.SessionFile {
		t.Errorf("TranscriptPath = %q, want %q", unified.TranscriptPath, dbAgent.SessionFile)
	}
	if unified.RepoPath != dbAgent.RepoPath {
		t.Errorf("RepoPath = %q, want %q", unified.RepoPath, dbAgent.RepoPath)
	}
	if unified.Branch != dbAgent.Branch {
		t.Errorf("Branch = %q, want %q", unified.Branch, dbAgent.Branch)
	}
	if unified.PID != dbAgent.PID {
		t.Errorf("PID = %d, want %d", unified.PID, dbAgent.PID)
	}
}

func TestAgent_ToUnified_UsesFileModTime(t *testing.T) {
	// Create a temporary session file
	tmpDir := t.TempDir()
	sessionFile := filepath.Join(tmpDir, "session.jsonl")
	if err := os.WriteFile(sessionFile, []byte(`{"test": "data"}`), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Set SpawnedAt to a time in the past (1 hour ago)
	spawnedAt := time.Now().Add(-1 * time.Hour)

	dbAgent := Agent{
		Name:        "my-agent",
		ULID:        "ulid123",
		SessionFile: sessionFile,
		SpawnedAt:   spawnedAt,
	}

	unified := dbAgent.ToUnified()

	// LastActivity should use file mod time, not SpawnedAt
	// The file was just created, so its mod time should be close to now
	// SpawnedAt is 1 hour ago, so if we're using file mod time,
	// LastActivity should be much more recent than SpawnedAt
	timeDiff := unified.LastActivity.Sub(spawnedAt)
	if timeDiff < 50*time.Minute {
		t.Errorf("LastActivity should use file mod time, not SpawnedAt. "+
			"Expected LastActivity to be ~1 hour after SpawnedAt, but diff was %v", timeDiff)
	}
}

func TestAgent_ToUnified_FallsBackToSpawnedAt(t *testing.T) {
	// Use a non-existent file path
	spawnedAt := time.Now().Add(-1 * time.Hour)

	dbAgent := Agent{
		Name:        "my-agent",
		ULID:        "ulid123",
		SessionFile: "/nonexistent/path/session.jsonl",
		SpawnedAt:   spawnedAt,
	}

	unified := dbAgent.ToUnified()

	// When file doesn't exist, should fall back to SpawnedAt
	if !unified.LastActivity.Equal(spawnedAt) {
		t.Errorf("LastActivity = %v, want %v (should fall back to SpawnedAt when file doesn't exist)",
			unified.LastActivity, spawnedAt)
	}
}

func TestAgentTypeField(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer database.Close()

	// Create agent with type
	agent := Agent{
		Name:        "test-agent",
		ULID:        "test-ulid",
		SessionFile: "/tmp/session.jsonl",
		Type:        "gemini",
	}
	if err := database.CreateAgent(agent); err != nil {
		t.Fatalf("CreateAgent failed: %v", err)
	}

	// Retrieve and verify type
	got, err := database.GetAgent("test-agent")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if got.Type != "gemini" {
		t.Errorf("Type = %q, want %q", got.Type, "gemini")
	}
}

func TestAgentTypeDefaultsToCodex(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer database.Close()

	// Create agent without type (should default to codex)
	agent := Agent{
		Name:        "test-agent",
		ULID:        "test-ulid",
		SessionFile: "/tmp/session.jsonl",
	}
	if err := database.CreateAgent(agent); err != nil {
		t.Fatalf("CreateAgent failed: %v", err)
	}

	got, err := database.GetAgent("test-agent")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if got.Type != "codex" {
		t.Errorf("Type = %q, want %q", got.Type, "codex")
	}
}

func TestAgent_ToUnified_GeminiType(t *testing.T) {
	dbAgent := Agent{
		Name:        "gemini-agent",
		ULID:        "ulid-gemini",
		SessionFile: "/path/to/session.jsonl",
		Type:        "gemini",
		SpawnedAt:   time.Now(),
	}

	unified := dbAgent.ToUnified()

	if unified.Source != agent.SourceGemini {
		t.Errorf("Source = %q, want %q", unified.Source, agent.SourceGemini)
	}
}

func TestMigration_AddsTypeColumn(t *testing.T) {
	// Create a DB with old schema (no type column)
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Manually create schema without type column
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
			spawned_at TEXT NOT NULL,
			repo_path TEXT DEFAULT '',
			branch TEXT DEFAULT ''
		);
		INSERT INTO agents (name, ulid, session_file, pid, spawned_at, repo_path, branch)
		VALUES ('old-agent', 'ulid123', '/tmp/session.jsonl', 0, '2025-01-01T00:00:00Z', '/code/project', 'main');
	`)
	if err != nil {
		t.Fatal(err)
	}
	rawDB.Close()

	// Now open with our Open() which should migrate
	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer database.Close()

	// Old agent should still be readable with default type 'codex'
	got, err := database.GetAgent("old-agent")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if got.Type != "codex" {
		t.Errorf("expected type 'codex' for migrated agent, got %q", got.Type)
	}
}

func TestMigrationAddsTasksTable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create DB with only agents table (simulating old schema)
	oldDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	_, err = oldDB.Exec(`
		CREATE TABLE agents (
			name TEXT PRIMARY KEY,
			ulid TEXT,
			session_file TEXT,
			cursor INTEGER DEFAULT 0,
			pid INTEGER,
			spawned_at TEXT,
			repo_path TEXT DEFAULT '',
			branch TEXT DEFAULT '',
			type TEXT DEFAULT 'codex'
		)
	`)
	if err != nil {
		t.Fatalf("Create agents table failed: %v", err)
	}
	oldDB.Close()

	// Open with our DB package (should migrate)
	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open with migration failed: %v", err)
	}
	defer database.Close()

	// Verify tasks table exists
	rows, err := database.Query("PRAGMA table_info(tasks)")
	if err != nil {
		t.Fatalf("Query pragma failed: %v", err)
	}
	defer rows.Close()

	hasTasksTable := false
	for rows.Next() {
		hasTasksTable = true
		break
	}

	if !hasTasksTable {
		t.Error("tasks table not created during migration")
	}
}

func TestMigration_PartialMigration_AddsMissingBranchColumn(t *testing.T) {
	// Create a DB with repo_path but NOT branch (partial migration scenario)
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Manually create schema with repo_path but not branch
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
			spawned_at TEXT NOT NULL,
			repo_path TEXT DEFAULT ''
		);
		INSERT INTO agents (name, ulid, session_file, pid, spawned_at, repo_path)
		VALUES ('partial-agent', 'ulid456', '/tmp/session.jsonl', 0, '2025-01-01T00:00:00Z', '/code/project');
	`)
	if err != nil {
		t.Fatal(err)
	}
	rawDB.Close()

	// Now open with our Open() which should migrate and add the missing branch column
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Agent should be readable with empty branch
	agent, err := db.GetAgent("partial-agent")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if agent.RepoPath != "/code/project" {
		t.Errorf("expected RepoPath '/code/project', got %q", agent.RepoPath)
	}
	if agent.Branch != "" {
		t.Errorf("expected empty Branch for partially migrated agent, got %q", agent.Branch)
	}
}
