package db

import (
	"database/sql"
	"errors"
	"log"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const schemaSQL = `-- agents table
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

-- transcript entries table
CREATE TABLE IF NOT EXISTS transcript_entries (
  id TEXT PRIMARY KEY,
  agent_id TEXT NOT NULL,
  direction TEXT NOT NULL,
  stream TEXT,
  content TEXT NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_transcript_agent ON transcript_entries(agent_id, created_at);
CREATE INDEX IF NOT EXISTS idx_agents_cleanup ON agents(completed_at) WHERE completed_at IS NOT NULL;
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
	if _, err := conn.Exec(schemaSQL); err != nil {
		return err
	}
	// Migration: add pid column if it doesn't exist
	_, _ = conn.Exec(`ALTER TABLE agents ADD COLUMN pid INTEGER`)
	// Migration: add completed_at column if it doesn't exist
	_, _ = conn.Exec(`ALTER TABLE agents ADD COLUMN completed_at DATETIME`)
	// Migration: add to_id column if it doesn't exist
	_, _ = conn.Exec(`ALTER TABLE messages ADD COLUMN to_id TEXT`)
	return nil
}

func cleanupOldData(conn *sql.DB) {
	statements := []string{
		`DELETE FROM transcript_entries
		WHERE agent_id IN (
			SELECT id FROM agents
			WHERE completed_at < datetime('now', '-7 days')
		);`,
		`DELETE FROM messages
		WHERE to_id IN (
			SELECT id FROM agents
			WHERE completed_at < datetime('now', '-7 days')
		);`,
		`DELETE FROM agents
		WHERE completed_at < datetime('now', '-7 days');`,
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
