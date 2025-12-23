package db

import (
	"database/sql"
	"path/filepath"
	"testing"
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
	if err := conn.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='transcript_entries'").Scan(&name); err != nil {
		t.Fatalf("transcript_entries table missing: %v", err)
	}

	// Verify columns exist
	if !columnExists(t, conn, "agents", "completed_at") {
		t.Fatalf("agents.completed_at column missing")
	}
	if !columnExists(t, conn, "messages", "to_id") {
		t.Fatalf("messages.to_id column missing")
	}

	// Verify indexes exist
	indexes := []string{"idx_messages_created", "idx_agents_status", "idx_transcript_agent", "idx_agents_cleanup", "idx_messages_to"}
	for _, idx := range indexes {
		if err := conn.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx).Scan(&name); err != nil {
			t.Fatalf("index %q missing: %v", idx, err)
		}
	}
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
