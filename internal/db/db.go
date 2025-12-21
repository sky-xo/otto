package db

import (
	"database/sql"
	_ "modernc.org/sqlite"
)

const schemaSQL = `-- agents table
CREATE TABLE IF NOT EXISTS agents (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  task TEXT NOT NULL,
  status TEXT NOT NULL,
  session_id TEXT,
  worktree_path TEXT,
  branch_name TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- messages table
CREATE TABLE IF NOT EXISTS messages (
  id TEXT PRIMARY KEY,
  from_id TEXT NOT NULL,
  type TEXT NOT NULL,
  content TEXT NOT NULL,
  mentions TEXT,
  requires_human BOOLEAN DEFAULT FALSE,
  read_by TEXT DEFAULT '[]',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
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
	return conn, nil
}

func ensureSchema(conn *sql.DB) error {
	_, err := conn.Exec(schemaSQL)
	return err
}
