package db

import (
	"database/sql"
	"errors"
	"log"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// schemaTablesSQL creates tables only - indexes created after migrations
const schemaTablesSQL = `-- agents table
CREATE TABLE IF NOT EXISTS agents (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  task TEXT NOT NULL,
  status TEXT NOT NULL,
  session_id TEXT,
  pid INTEGER,
  worktree_path TEXT,
  branch_name TEXT,
  completed_at DATETIME,
  archived_at DATETIME,
  last_read_log_id TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- messages table
CREATE TABLE IF NOT EXISTS messages (
  id TEXT PRIMARY KEY,
  from_id TEXT NOT NULL,
  to_id TEXT,
  type TEXT NOT NULL,
  content TEXT NOT NULL,
  mentions TEXT,
  requires_human BOOLEAN DEFAULT FALSE,
  read_by TEXT DEFAULT '[]',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- logs table
CREATE TABLE IF NOT EXISTS logs (
  id TEXT PRIMARY KEY,
  agent_id TEXT NOT NULL,
  direction TEXT NOT NULL,
  stream TEXT,
  content TEXT NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

// schemaIndexesSQL creates indexes - must run AFTER migrations add columns
const schemaIndexesSQL = `
CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_logs_agent ON logs(agent_id, created_at);
CREATE INDEX IF NOT EXISTS idx_agents_cleanup ON agents(completed_at) WHERE completed_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_agents_archived ON agents(archived_at) WHERE archived_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_messages_to ON messages(to_id, created_at);
`

func Open(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := ensureSchema(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	cleanupOldData(conn)
	return conn, nil
}

func ensureSchema(conn *sql.DB) error {
	// Step 1: Create tables (for new databases)
	if _, err := conn.Exec(schemaTablesSQL); err != nil {
		return err
	}

	// Step 2: Run migrations (for existing databases missing columns)
	// These use _, _ to ignore "duplicate column" errors
	_, _ = conn.Exec(`ALTER TABLE agents ADD COLUMN pid INTEGER`)
	_, _ = conn.Exec(`ALTER TABLE agents ADD COLUMN completed_at DATETIME`)
	_, _ = conn.Exec(`ALTER TABLE agents ADD COLUMN archived_at DATETIME`)
	_, _ = conn.Exec(`ALTER TABLE agents ADD COLUMN last_read_log_id TEXT`)
	_, _ = conn.Exec(`ALTER TABLE messages ADD COLUMN to_id TEXT`)
	_, _ = conn.Exec(`ALTER TABLE transcript_entries RENAME TO logs`)

	// Step 3: Create indexes (after migrations ensure columns exist)
	if _, err := conn.Exec(schemaIndexesSQL); err != nil {
		return err
	}
	return nil
}

func cleanupOldData(conn *sql.DB) {
	statements := []string{
		`DELETE FROM logs
		WHERE agent_id IN (
			SELECT id FROM agents
			WHERE archived_at < datetime('now', '-7 days')
		);`,
		`DELETE FROM messages
		WHERE to_id IN (
			SELECT id FROM agents
			WHERE archived_at < datetime('now', '-7 days')
		);`,
		`DELETE FROM agents
		WHERE archived_at < datetime('now', '-7 days');`,
	}

	for _, stmt := range statements {
		if _, err := conn.Exec(stmt); err != nil {
			if isSQLiteBusy(err) {
				return
			}
			log.Printf("db cleanup: %v", err)
			return
		}
	}
}

func isSQLiteBusy(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	return sqliteErr.Code() == sqlite3.SQLITE_BUSY
}
