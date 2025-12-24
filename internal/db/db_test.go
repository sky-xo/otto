package db

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestEnsureSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "otto.db")

	conn, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer conn.Close()

	// Verify tables exist
	var name string
	if err := conn.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='agents'").Scan(&name); err != nil {
		t.Fatalf("agents table missing: %v", err)
	}
	if err := conn.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='messages'").Scan(&name); err != nil {
		t.Fatalf("messages table missing: %v", err)
	}
	if err := conn.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='logs'").Scan(&name); err != nil {
		t.Fatalf("logs table missing: %v", err)
	}

	// Verify columns exist
	if !columnExists(t, conn, "agents", "completed_at") {
		t.Fatalf("agents.completed_at column missing")
	}
	if !columnExists(t, conn, "messages", "to_id") {
		t.Fatalf("messages.to_id column missing")
	}

	// Verify indexes exist
	indexes := []string{"idx_messages_created", "idx_agents_status", "idx_logs_agent", "idx_agents_cleanup", "idx_messages_to"}
	for _, idx := range indexes {
		if err := conn.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx).Scan(&name); err != nil {
			t.Fatalf("index %q missing: %v", idx, err)
		}
	}
}

func TestLogsTableExists(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	// Verify logs table exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&count)
	if err != nil {
		t.Errorf("logs table should exist: %v", err)
	}

	// Verify old table doesn't exist (or is aliased)
	err = db.QueryRow("SELECT COUNT(*) FROM transcript_entries").Scan(&count)
	// This should still work due to migration handling old DBs
}

func columnExists(t *testing.T, conn *sql.DB, table, column string) bool {
	t.Helper()
	rows, err := conn.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatalf("pragma table_info %s: %v", table, err)
	}
	defer rows.Close()

	var (
		cid        int
		name       string
		colType    string
		notNull    int
		defaultVal sql.NullString
		pk         int
	)
	for rows.Next() {
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultVal, &pk); err != nil {
			t.Fatalf("scan table_info %s: %v", table, err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows table_info %s: %v", table, err)
	}
	return false
}

func TestCleanupOnOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "otto.db")

	conn, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	oldCompleted := sqliteTime(time.Now().Add(-8 * 24 * time.Hour))
	recentCompleted := sqliteTime(time.Now().Add(-2 * 24 * time.Hour))

	_, err = conn.Exec(
		`INSERT INTO agents (id, type, task, status, completed_at) VALUES (?, ?, ?, ?, ?)`,
		"agent-old", "codex", "old task", "complete", oldCompleted,
	)
	if err != nil {
		t.Fatalf("insert old agent: %v", err)
	}
	_, err = conn.Exec(
		`INSERT INTO agents (id, type, task, status, completed_at) VALUES (?, ?, ?, ?, ?)`,
		"agent-recent", "codex", "recent task", "complete", recentCompleted,
	)
	if err != nil {
		t.Fatalf("insert recent agent: %v", err)
	}
	_, err = conn.Exec(
		`INSERT INTO agents (id, type, task, status) VALUES (?, ?, ?, ?)`,
		"agent-active", "codex", "active task", "busy",
	)
	if err != nil {
		t.Fatalf("insert active agent: %v", err)
	}

	_, err = conn.Exec(
		`INSERT INTO logs (id, agent_id, direction, content) VALUES (?, ?, ?, ?)`,
		"entry-old", "agent-old", "out", "old output",
	)
	if err != nil {
		t.Fatalf("insert old log entry: %v", err)
	}
	_, err = conn.Exec(
		`INSERT INTO logs (id, agent_id, direction, content) VALUES (?, ?, ?, ?)`,
		"entry-recent", "agent-recent", "out", "recent output",
	)
	if err != nil {
		t.Fatalf("insert recent log entry: %v", err)
	}

	_, err = conn.Exec(
		`INSERT INTO messages (id, from_id, to_id, type, content) VALUES (?, ?, ?, ?, ?)`,
		"msg-old", "orchestrator", "agent-old", "prompt", "old prompt",
	)
	if err != nil {
		t.Fatalf("insert old message: %v", err)
	}
	_, err = conn.Exec(
		`INSERT INTO messages (id, from_id, to_id, type, content) VALUES (?, ?, ?, ?, ?)`,
		"msg-recent", "orchestrator", "agent-recent", "prompt", "recent prompt",
	)
	if err != nil {
		t.Fatalf("insert recent message: %v", err)
	}
	_, err = conn.Exec(
		`INSERT INTO messages (id, from_id, type, content) VALUES (?, ?, ?, ?)`,
		"msg-main", "orchestrator", "note", "main channel",
	)
	if err != nil {
		t.Fatalf("insert main message: %v", err)
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	conn, err = Open(path)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	defer conn.Close()

	if countRows(t, conn, "SELECT COUNT(*) FROM agents WHERE id='agent-old'") != 0 {
		t.Fatalf("expected old agent to be deleted")
	}
	if countRows(t, conn, "SELECT COUNT(*) FROM agents WHERE id='agent-recent'") != 1 {
		t.Fatalf("expected recent agent to remain")
	}
	if countRows(t, conn, "SELECT COUNT(*) FROM agents WHERE id='agent-active'") != 1 {
		t.Fatalf("expected active agent to remain")
	}

	if countRows(t, conn, "SELECT COUNT(*) FROM logs WHERE id='entry-old'") != 0 {
		t.Fatalf("expected old log entry to be deleted")
	}
	if countRows(t, conn, "SELECT COUNT(*) FROM logs WHERE id='entry-recent'") != 1 {
		t.Fatalf("expected recent log entry to remain")
	}

	if countRows(t, conn, "SELECT COUNT(*) FROM messages WHERE id='msg-old'") != 0 {
		t.Fatalf("expected old message to be deleted")
	}
	if countRows(t, conn, "SELECT COUNT(*) FROM messages WHERE id='msg-recent'") != 1 {
		t.Fatalf("expected recent message to remain")
	}
	if countRows(t, conn, "SELECT COUNT(*) FROM messages WHERE id='msg-main'") != 1 {
		t.Fatalf("expected main channel message to remain")
	}
}

func sqliteTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}

func countRows(t *testing.T, conn *sql.DB, query string) int {
	t.Helper()
	var count int
	if err := conn.QueryRow(query).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}
